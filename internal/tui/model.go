// Package tui implements Optimus's interactive terminal UI (PLAN.md §26-31):
// a persistent Bubble Tea session with a header, scrolling conversation,
// streaming model output, and slash commands, as an alternative to the
// one-shot `optimus ask` command.
package tui

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"optimus/internal/config"
	"optimus/internal/model"
)

const (
	headerHeight   = 3 // title line + meta line + divider
	footerHeight   = 2 // divider + hint line
	inputHeight    = 1
	maxSuggestions = 6 // cap on how many slash-command suggestions to show at once
)

// chatMessage is one rendered line of the conversation: a user prompt, an
// assistant reply, a system notice (slash command output), or an error.
type chatMessage struct {
	role    string // "user", "assistant", "system", "error"
	content string
}

// modelPicker holds the state of the interactive list /models opens when
// the active provider supports both listing and switching models: arrow
// keys move the highlighted row, enter switches to it, esc cancels. Nil
// when no picker is open.
type modelPicker struct {
	options []model.InstalledModel
	index   int
}

// Model is the root Bubble Tea model for the interactive session.
type Model struct {
	cfg     config.Config
	cwd     string
	project string
	branch  string
	model   model.Model
	logger  *slog.Logger

	viewport viewport.Model
	input    textinput.Model
	spinner  spinner.Model

	messages []chatMessage
	history  []model.Message

	streaming     bool
	streamContent strings.Builder
	cancel        context.CancelFunc
	gen           int // bumped on each new request/cancel; see stream.go

	totalPromptTokens, totalCompletionTokens int
	lastPromptTokens, lastCompletionTokens   int

	suggestIndex int          // which slash-command suggestion is highlighted
	picker       *modelPicker // non-nil while /models's interactive picker is open

	width, height int
	ready         bool
	quitting      bool

	showBanner bool // true until the startup banner finishes or is skipped
	bannerTick int  // advances once per bannerTick(); see banner.go
}

// New builds the interactive session model. m is the active ("fast") model
// the session talks to.
func New(cfg config.Config, cwd string, m model.Model, logger *slog.Logger) *Model {
	ti := textinput.New()
	ti.Prompt = ""
	ti.Placeholder = "Ask Optimus anything, or /help for commands"
	ti.Focus()
	ti.CharLimit = 4000

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	return &Model{
		cfg:        cfg,
		cwd:        cwd,
		project:    projectName(cwd),
		branch:     gitBranch(cwd),
		model:      m,
		logger:     logger,
		input:      ti,
		spinner:    sp,
		showBanner: true,
	}
}

func projectName(cwd string) string {
	name := cwd
	if i := strings.LastIndexByte(cwd, '/'); i >= 0 && i+1 < len(cwd) {
		name = cwd[i+1:]
	}
	if name == "" {
		name = cwd
	}
	return name
}

func gitBranch(cwd string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func (m *Model) Init() tea.Cmd {
	if m.showBanner {
		return tea.Batch(textinput.Blink, m.spinner.Tick, bannerTick())
	}
	return tea.Batch(textinput.Blink, m.spinner.Tick)
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		if !m.ready {
			m.viewport = viewport.New(msg.Width, 3)
			m.ready = true
		}
		m.resizeViewport()
		m.input.Width = msg.Width - 4
		m.syncViewport()
		return m, nil

	case tea.KeyMsg:
		if m.showBanner {
			if msg.Type == tea.KeyCtrlC {
				m.quitting = true
				return m, tea.Quit
			}
			m.showBanner = false
			return m, nil
		}
		if m.picker != nil {
			return m.handlePickerKey(msg)
		}
		return m.handleKey(msg)

	case streamStartedMsg:
		if msg.gen != m.gen {
			return m, nil
		}
		m.streaming = true
		return m, listenForEvent(msg.events, msg.gen)

	case streamEventMsg:
		if msg.gen != m.gen {
			return m, nil
		}
		return m.handleStreamEvent(msg)

	case streamErrMsg:
		if msg.gen != m.gen {
			return m, nil
		}
		m.streaming = false
		m.messages = append(m.messages, chatMessage{role: "error", content: msg.err.Error()})
		m.syncViewport()
		return m, nil

	case streamClosedMsg:
		if msg.gen != m.gen {
			return m, nil
		}
		m.streaming = false
		m.syncViewport()
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		if m.streaming {
			m.syncViewport()
		}
		return m, cmd

	case bannerTickMsg:
		if !m.showBanner {
			return m, nil
		}
		m.bannerTick++
		if m.bannerTick >= bannerTotalTicks(m.width, m.height) {
			m.showBanner = false
			return m, nil
		}
		return m, bannerTick()
	}

	return m, nil
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		if m.streaming && m.cancel != nil {
			m.cancel()
			m.gen++
		}
		m.quitting = true
		return m, tea.Quit

	case tea.KeyEsc:
		if m.streaming && m.cancel != nil {
			m.cancel()
			m.gen++
			m.streaming = false
			m.finalizePartial("Generation stopped.")
		}
		return m, nil

	case tea.KeyEnter:
		if m.streaming {
			return m, nil
		}
		if sugs := m.suggestions(); len(sugs) > 0 {
			m.input.SetValue("/" + sugs[m.suggestIndex].name)
			m.input.CursorEnd()
		}
		return m.handleSubmit()

	case tea.KeyTab:
		if m.streaming {
			return m, nil
		}
		if sugs := m.suggestions(); len(sugs) > 0 {
			m.input.SetValue("/" + sugs[m.suggestIndex].name + " ")
			m.input.CursorEnd()
			m.suggestIndex = 0
			m.resizeViewport()
		}
		return m, nil

	case tea.KeyUp:
		if sugs := m.suggestions(); len(sugs) > 0 {
			m.suggestIndex = clampIndex(m.suggestIndex-1, len(sugs))
			return m, nil
		}

	case tea.KeyDown:
		if sugs := m.suggestions(); len(sugs) > 0 {
			m.suggestIndex = clampIndex(m.suggestIndex+1, len(sugs))
			return m, nil
		}

	case tea.KeyPgUp, tea.KeyPgDown:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}

	if m.streaming {
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.suggestIndex = 0
	m.resizeViewport()
	return m, cmd
}

// handlePickerKey handles all key input while the /models picker is open,
// taking over from handleKey entirely — the picker is modal, so nothing
// else (typing, other slash commands) happens until it's closed.
func (m *Model) handlePickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		m.quitting = true
		return m, tea.Quit

	case tea.KeyEsc:
		m.picker = nil
		m.resizeViewport()
		return m, nil

	case tea.KeyUp:
		m.picker.index = clampIndex(m.picker.index-1, len(m.picker.options))
		return m, nil

	case tea.KeyDown:
		m.picker.index = clampIndex(m.picker.index+1, len(m.picker.options))
		return m, nil

	case tea.KeyEnter:
		name := m.picker.options[m.picker.index].Name
		m.picker = nil
		m.resizeViewport()
		m.messages = append(m.messages, chatMessage{role: "system", content: switchModel(m, name)})
		m.syncViewport()
		return m, nil
	}
	return m, nil
}

// suggestions returns the slash commands (capped at maxSuggestions) whose
// name has the currently-typed "/prefix" as a prefix. It's empty once the
// input isn't a bare "/word" anymore (e.g. a space was typed, meaning the
// user has moved on to arguments) or doesn't start with "/" at all.
func (m *Model) suggestions() []slashCommand {
	val := m.input.Value()
	if !strings.HasPrefix(val, "/") {
		return nil
	}
	rest := val[1:]
	if strings.ContainsAny(rest, " \t") {
		return nil
	}

	var out []slashCommand
	for _, cmd := range slashCommandTable() {
		if strings.HasPrefix(cmd.name, rest) {
			out = append(out, cmd)
			if len(out) == maxSuggestions {
				break
			}
		}
	}
	return out
}

func clampIndex(i, n int) int {
	if n == 0 {
		return 0
	}
	i %= n
	if i < 0 {
		i += n
	}
	return i
}

// resizeViewport recomputes the conversation viewport's height from the
// current terminal size and whatever's showing below the input — the
// suggestions dropdown (changes on every keystroke while typing a slash
// command) or the /models picker.
func (m *Model) resizeViewport() {
	if !m.ready {
		return
	}
	extraHeight := 0
	switch {
	case m.picker != nil:
		extraHeight = len(m.picker.options) + 1 // +1 for the divider above the list
	case len(m.suggestions()) > 0:
		extraHeight = len(m.suggestions()) + 1
	}
	vpHeight := m.height - headerHeight - footerHeight - inputHeight - extraHeight
	if vpHeight < 3 {
		vpHeight = 3
	}
	m.viewport.Width = m.width
	m.viewport.Height = vpHeight
}

func (m *Model) handleSubmit() (tea.Model, tea.Cmd) {
	line := strings.TrimSpace(m.input.Value())
	if line == "" {
		return m, nil
	}
	m.input.Reset()
	m.suggestIndex = 0
	m.resizeViewport()

	if strings.HasPrefix(line, "/") {
		m.messages = append(m.messages, chatMessage{role: "user", content: line})
		output, quit, ok := dispatchSlash(m, line)
		if ok {
			m.messages = append(m.messages, chatMessage{role: "system", content: output})
		}
		m.syncViewport()
		if quit {
			m.quitting = true
			return m, tea.Quit
		}
		return m, nil
	}

	m.messages = append(m.messages, chatMessage{role: "user", content: line})
	m.history = append(m.history, model.Message{Role: model.RoleUser, Content: line})
	m.streaming = true
	m.streamContent.Reset()
	m.syncViewport()

	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.gen++
	req := model.ModelRequest{Messages: append([]model.Message(nil), m.history...)}
	return m, tea.Batch(startStream(m.model, ctx, req, m.gen), m.spinner.Tick)
}

func (m *Model) handleStreamEvent(msg streamEventMsg) (tea.Model, tea.Cmd) {
	switch msg.event.Type {
	case model.EventToken:
		m.streamContent.WriteString(msg.event.Token)
		m.syncViewport()
		return m, listenForEvent(msg.events, msg.gen)

	case model.EventDone:
		resp := msg.event.Response
		m.messages = append(m.messages, chatMessage{role: "assistant", content: resp.Content})
		m.history = append(m.history, model.Message{Role: model.RoleAssistant, Content: resp.Content})
		m.lastPromptTokens, m.lastCompletionTokens = resp.PromptTokens, resp.CompletionTokens
		m.totalPromptTokens += resp.PromptTokens
		m.totalCompletionTokens += resp.CompletionTokens
		m.streaming = false
		m.streamContent.Reset()
		m.syncViewport()
		return m, nil

	case model.EventError:
		m.messages = append(m.messages, chatMessage{role: "error", content: msg.event.Err.Error()})
		m.streaming = false
		m.streamContent.Reset()
		m.syncViewport()
		return m, nil
	}

	return m, listenForEvent(msg.events, msg.gen)
}

// finalizePartial commits whatever was streamed so far as an assistant
// message (labeled with note, e.g. "Generation stopped.") instead of
// discarding it.
func (m *Model) finalizePartial(note string) {
	if m.streamContent.Len() > 0 {
		m.messages = append(m.messages, chatMessage{role: "assistant", content: m.streamContent.String()})
		m.history = append(m.history, model.Message{Role: model.RoleAssistant, Content: m.streamContent.String()})
	}
	m.streamContent.Reset()
	m.messages = append(m.messages, chatMessage{role: "system", content: note})
	m.syncViewport()
}

func (m *Model) syncViewport() {
	if !m.ready {
		return
	}
	if len(m.messages) == 0 && !m.streaming {
		m.viewport.SetContent(renderIdleMascot(m.viewport.Width, m.viewport.Height))
		return
	}
	var b strings.Builder
	for i, cm := range m.messages {
		if i > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(renderMessage(cm, m.viewport.Width))
	}
	if m.streaming {
		if len(m.messages) > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(assistantLabelStyle.Render("Optimus"))
		b.WriteString("\n")
		content := trimLeadingBlankLines(m.streamContent.String())
		if content == "" {
			b.WriteString(m.spinner.View())
			b.WriteString(" thinking…")
		} else {
			b.WriteString(wrapText(content, m.viewport.Width))
		}
	}
	atBottom := m.viewport.AtBottom()
	m.viewport.SetContent(b.String())
	if atBottom {
		m.viewport.GotoBottom()
	}
}

// renderMessage renders one conversation entry, word-wrapped to width.
// User and assistant content is free-form prose from the person or the
// model, so it's wrapped to guarantee no line ever overflows the terminal.
// System and error text is left unwrapped — it's slash-command output
// (aligned tables, /logo's ASCII art) that wrapping would visually break
// rather than fix, and is short enough in practice not to need it.
func renderMessage(cm chatMessage, width int) string {
	switch cm.role {
	case "user":
		return userLabelStyle.Render("You") + "\n" + wrapText(cm.content, width)
	case "assistant":
		return assistantLabelStyle.Render("Optimus") + "\n" + wrapText(trimLeadingBlankLines(cm.content), width)
	case "error":
		return errorStyle.Render(cm.content)
	default:
		return systemStyle.Render(cm.content)
	}
}

func (m *Model) View() string {
	if !m.ready {
		return "Initializing…"
	}

	if m.showBanner {
		return renderBanner(m)
	}

	divider := dividerStyle.Render(strings.Repeat("─", m.width))

	mc := m.cfg.Models["fast"]
	caps := m.model.Capabilities()
	branch := m.branch
	if branch == "" {
		branch = "-"
	}
	used := m.totalPromptTokens + m.totalCompletionTokens

	line1 := fmt.Sprintf("%s   Project: %s   Branch: %s",
		headerStyle.Render("Optimus"), m.project, branch)
	line2 := headerMetaStyle.Render(fmt.Sprintf(
		"Model: %s   Context: %d / %d", mc.Model, used, caps.MaxContextTokens))

	header := line1 + "\n" + line2 + "\n" + divider

	inputLine := inputPromptStyle.Render("> ") + m.input.View()

	hint := "enter send · esc stop · ctrl+c quit · /help commands"
	switch {
	case m.streaming:
		hint = m.spinner.View() + " generating… (esc to stop)"
	case m.picker != nil:
		hint = "↑/↓ select · enter switch · esc cancel"
	case len(m.suggestions()) > 0:
		hint = "↑/↓ select · tab complete · enter run"
	}
	footer := divider + "\n" + footerStyle.Render(hint)

	body := header + "\n" + m.viewport.View() + "\n" + inputLine
	switch {
	case m.picker != nil:
		body += "\n" + m.renderPicker()
	default:
		if suggestions := m.renderSuggestions(); suggestions != "" {
			body += "\n" + suggestions
		}
	}
	return body + "\n" + footer
}

// renderPicker renders the /models interactive list: one row per installed
// model, "*" marking the currently active one and "›" (highlighted) marking
// the row arrow keys currently point to — two independent things, since the
// active model and the highlighted-for-switching model aren't always the
// same row.
func (m *Model) renderPicker() string {
	active := m.cfg.Models["fast"].Model

	var b strings.Builder
	for i, im := range m.picker.options {
		marker := " "
		if im.Name == active {
			marker = "*"
		}
		line := fmt.Sprintf("%s %-20s %s %s", marker, im.Name, im.ParameterSize, im.Quantization)
		if i > 0 {
			b.WriteString("\n")
		}
		if i == m.picker.index {
			b.WriteString(suggestionActiveStyle.Render("› " + line))
		} else {
			b.WriteString(suggestionStyle.Render("  " + line))
		}
	}
	return b.String()
}

// renderSuggestions renders the slash-command dropdown shown below the
// input while typing "/word", empty when there's nothing to suggest.
func (m *Model) renderSuggestions() string {
	sugs := m.suggestions()
	if len(sugs) == 0 {
		return ""
	}
	idx := clampIndex(m.suggestIndex, len(sugs))

	var b strings.Builder
	for i, cmd := range sugs {
		line := fmt.Sprintf("/%-10s %s", cmd.name, cmd.summary)
		if i > 0 {
			b.WriteString("\n")
		}
		if i == idx {
			b.WriteString(suggestionActiveStyle.Render("› " + line))
		} else {
			b.WriteString(suggestionStyle.Render("  " + line))
		}
	}
	return b.String()
}
