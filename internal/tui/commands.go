package tui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"optimus/internal/config"
	"optimus/internal/model"
)

// listModelsTimeout bounds ListModels calls made from slash-command
// dispatch, which runs synchronously on the TUI's single event-loop
// goroutine (there's no async command machinery for slash commands, unlike
// generation). Without a short timeout here, an unreachable or hung Ollama
// server would freeze the entire TUI — including ctrl+c — for the model's
// full request timeout (2 minutes by default) instead of a few seconds.
const listModelsTimeout = 3 * time.Second

func listInstalledModels(lister model.ModelLister) ([]model.InstalledModel, error) {
	ctx, cancel := context.WithTimeout(context.Background(), listModelsTimeout)
	defer cancel()
	return lister.ListModels(ctx)
}

// slashCommand is one entry in the / command table.
type slashCommand struct {
	name    string
	summary string
	// run executes the command against m and returns the system-message
	// text to display in the conversation. quit signals the TUI should
	// exit after showing it.
	run func(m *Model, args []string) (output string, quit bool)
}

// slashCommandTable is the single source of truth for available commands.
// It's a function rather than a package-level var because cmdHelp needs to
// read the same list, which would otherwise be an initialization cycle
// (the var's initializer would reference cmdHelp, which reads the var).
//
// Only implemented commands belong here so /help and suggestions stay useful.
func slashCommandTable() []slashCommand {
	return []slashCommand{
		{name: "help", summary: "List available commands", run: cmdHelp},
		{name: "logo", summary: "Show the Optimus mascot", run: cmdLogo},
		{name: "model", summary: "Show/switch model; /model default [<name>] [--project] saves a default", run: cmdModel},
		{name: "models", summary: "List installed Ollama models", run: cmdModels},
		{name: "status", summary: "Show project, branch, and permissions", run: cmdStatus},
		{name: "context", summary: "Show this session's token usage", run: cmdContext},
		{name: "clear", summary: "Clear the conversation", run: cmdClear},
		{name: "bye", summary: "Exit Optimus", run: cmdQuit},
		{name: "quit", summary: "Exit Optimus", run: cmdQuit},
		{name: "exit", summary: "Exit Optimus", run: cmdQuit},
	}
}

// dispatchSlash parses a leading "/" line and runs the matching command.
// ok is false if line wasn't a recognized slash command at all.
func dispatchSlash(m *Model, line string) (output string, quit bool, ok bool) {
	line = strings.TrimSpace(strings.TrimPrefix(line, "/"))
	if line == "" {
		return "", false, false
	}
	fields := strings.Fields(line)
	name, args := fields[0], fields[1:]

	for _, cmd := range slashCommandTable() {
		if cmd.name != name {
			continue
		}
		output, quit = cmd.run(m, args)
		return output, quit, true
	}
	return fmt.Sprintf("Unknown command /%s — try /help.", name), false, true
}

func cmdHelp(m *Model, args []string) (string, bool) {
	var b strings.Builder
	b.WriteString("Commands:\n")
	for _, cmd := range slashCommandTable() {
		fmt.Fprintf(&b, "  /%-10s %s\n", cmd.name, cmd.summary)
	}
	b.WriteString("\nKeys: enter send · esc stop generation · ctrl+c quit")
	return strings.TrimRight(b.String(), "\n"), false
}

func cmdLogo(m *Model, args []string) (string, bool) {
	lines := mascotCompactLines()
	return renderArt(lines, mascotWidth(lines), bannerGradient), false
}

// switchModel points m.model at name and updates the "fast" config entry to
// match, so the header and /model/status commands stay consistent. Callers
// must have already confirmed m.model implements model.ModelSwitcher.
func switchModel(m *Model, name string) string {
	m.model.(model.ModelSwitcher).SwitchModel(name)
	mc := m.cfg.Models["fast"]
	mc.Model = name
	m.cfg.Models["fast"] = mc
	return fmt.Sprintf("Switched active model to %q.", name)
}

func containsModel(installed []model.InstalledModel, name string) bool {
	for _, im := range installed {
		if im.Name == name {
			return true
		}
	}
	return false
}

// configuredModelClasses renders the static models: config classes — the
// fallback view for a provider that can't list/switch models live.
func configuredModelClasses(m *Model) string {
	if len(m.cfg.Models) == 0 {
		return "No models configured."
	}
	classes := make([]string, 0, len(m.cfg.Models))
	for name := range m.cfg.Models {
		classes = append(classes, name)
	}
	sort.Strings(classes)

	var b strings.Builder
	b.WriteString("Configured model classes:\n")
	for _, class := range classes {
		mc := m.cfg.Models[class]
		fmt.Fprintf(&b, "  %-10s %s (%s @ %s)\n", class, mc.Model, mc.Provider, mc.BaseURL)
	}
	return strings.TrimRight(b.String(), "\n")
}

func cmdModel(m *Model, args []string) (string, bool) {
	mc, ok := m.cfg.Models["fast"]
	if !ok {
		return "No \"fast\" model configured.", false
	}

	if len(args) == 0 {
		caps := m.model.Capabilities()
		return fmt.Sprintf("model:       %s\nprovider:    %s\nbase_url:    %s\ncontext:     %d tokens",
			mc.Model, mc.Provider, mc.BaseURL, caps.MaxContextTokens), false
	}

	saveDefault := args[0] == "default"
	project := false
	if saveDefault {
		args = args[1:]
		if len(args) > 0 && args[len(args)-1] == "--project" {
			project = true
			args = args[:len(args)-1]
		}
		if len(args) == 0 {
			args = []string{mc.Model}
		}
	}
	if len(args) != 1 || strings.HasPrefix(args[0], "-") {
		return "Usage: /model <name> or /model default [<name>] [--project]", false
	}
	name := args[0]
	if _, ok := m.model.(model.ModelSwitcher); !ok {
		return fmt.Sprintf("The configured provider (%s) doesn't support switching models at runtime.", mc.Provider), false
	}

	// If the provider can also list what's installed, validate the name
	// against it — a typo should say so now, not surface as an opaque
	// "model not found" from Ollama on the next prompt.
	if lister, ok := m.model.(model.ModelLister); ok {
		installed, err := listInstalledModels(lister)
		if err == nil && !containsModel(installed, name) {
			return fmt.Sprintf("Model %q isn't installed. Run `ollama pull %s` first, or check /models for what's available.", name, name), false
		}
	}

	if saveDefault {
		path, err := config.SaveDefaultModel(m.cwd, name, project)
		if err != nil {
			return fmt.Sprintf("Could not save default model: %v", err), false
		}
		output := switchModel(m, name) + fmt.Sprintf("\nSaved default to %s.", path)
		if !project {
			if cfg, err := config.Load(m.cwd); err == nil && cfg.Models["fast"].Model != name {
				output += "\nThis project overrides the global default. Use /model default " + name + " --project to update it."
			}
		}
		return output, false
	}
	return switchModel(m, name), false
}

// cmdModels lists installed models and, when the provider supports both
// listing and switching, opens an interactive picker (m.picker) so the user
// can move with the arrow keys and press enter to switch instead of typing
// /model <name> by hand. Providers that can only list, or can't list at
// all, fall back to the static text views they always had.
func cmdModels(m *Model, args []string) (string, bool) {
	lister, listOK := m.model.(model.ModelLister)
	if !listOK {
		return configuredModelClasses(m), false
	}

	installed, err := listInstalledModels(lister)
	if err != nil {
		return fmt.Sprintf("Could not list installed models: %v\n\n%s", err, configuredModelClasses(m)), false
	}
	if len(installed) == 0 {
		return "No models installed.\n\n" + configuredModelClasses(m), false
	}
	sort.Slice(installed, func(i, j int) bool { return installed[i].Name < installed[j].Name })

	if _, switchOK := m.model.(model.ModelSwitcher); !switchOK {
		active := m.cfg.Models["fast"].Model
		var b strings.Builder
		b.WriteString("Installed models:\n")
		for _, im := range installed {
			marker := " "
			if im.Name == active {
				marker = "*"
			}
			fmt.Fprintf(&b, "%s %-20s %s %s\n", marker, im.Name, im.ParameterSize, im.Quantization)
		}
		return strings.TrimRight(b.String(), "\n") + "\n\n" + configuredModelClasses(m), false
	}

	active := m.cfg.Models["fast"].Model
	index := 0
	for i, im := range installed {
		if im.Name == active {
			index = i
			break
		}
	}
	m.picker = &modelPicker{options: installed, index: index}
	m.resizeViewport()
	return "Select a model — ↑/↓ to move, enter to switch, esc to cancel.", false
}

func cmdStatus(m *Model, args []string) (string, bool) {
	branch := m.branch
	if branch == "" {
		branch = "-"
	}
	p := m.cfg.Permissions
	return fmt.Sprintf(
		"project:     %s\ndirectory:   %s\nbranch:      %s\npermissions: read=%s write=%s shell=%s git_write=%s network=%s",
		m.project, m.cwd, branch, p.Read, p.Write, p.Shell, p.GitWrite, p.Network,
	), false
}

func cmdContext(m *Model, args []string) (string, bool) {
	caps := m.model.Capabilities()
	return fmt.Sprintf(
		"session tokens: %d prompt + %d completion = %d total\ncontext window: %d\nlast turn:      %d prompt + %d completion",
		m.totalPromptTokens, m.totalCompletionTokens, m.totalPromptTokens+m.totalCompletionTokens,
		caps.MaxContextTokens, m.lastPromptTokens, m.lastCompletionTokens,
	), false
}

func cmdClear(m *Model, args []string) (string, bool) {
	m.messages = nil
	m.history = nil
	m.totalPromptTokens, m.totalCompletionTokens = 0, 0
	m.lastPromptTokens, m.lastCompletionTokens = 0, 0
	m.syncViewport()
	return "Conversation cleared.", false
}

func cmdQuit(m *Model, args []string) (string, bool) {
	return "Bye.", true
}
