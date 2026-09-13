// Command optimus is the entrypoint for the Optimus CLI.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"optimus/internal/agent"
	"optimus/internal/config"
	"optimus/internal/logging"
	"optimus/internal/model"
	"optimus/internal/repository"
	"optimus/internal/tools"
	"optimus/internal/tui"
)

// version is set by the release build; local builds identify themselves as dev.
var version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "optimus:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return runTUI()
	}

	switch args[0] {
	case "version", "--version", "-v":
		fmt.Println("Optimus", version)
		return nil
	case "ask":
		return runAsk(args[1:])
	case "config":
		return runConfig(args[1:])
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		return runAgent(args)
	}
}

func printUsage() {
	fmt.Println(`Optimus - a local-first terminal coding agent

Usage:
  optimus                   Start an interactive session
  optimus "<task>"          Inspect the repository and answer a question or task
  optimus ask "<prompt>"    Send a one-off prompt to the configured model, with no tools
  optimus config model      Show the effective default model
  optimus config model <name> [--project]  Save a default model
  optimus --version         Show the installed version
  optimus help              Show this help message`)
}

func runConfig(args []string) error {
	const usage = "usage: optimus config model [<name>] [--project]"
	if len(args) == 0 || args[0] != "model" {
		return fmt.Errorf("%s", usage)
	}
	project := false
	var name string
	for _, arg := range args[1:] {
		if arg == "--project" && !project {
			project = true
		} else if name == "" && !strings.HasPrefix(arg, "-") {
			name = arg
		} else {
			return fmt.Errorf("%s", usage)
		}
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	if name != "" {
		path, err := config.SaveDefaultModel(cwd, name, project)
		if err != nil {
			return fmt.Errorf("save default model: %w", err)
		}
		fmt.Printf("Saved default model %q to %s.\n", name, path)
	} else if project {
		return fmt.Errorf("%s", usage)
	}
	cfg, err := config.Load(cwd)
	if err != nil {
		return err
	}
	fmt.Printf("Default model for this directory: %s\n", cfg.Models["fast"].Model)
	return nil
}

// loadModel resolves config and builds the "fast" model class the same way
// for every entrypoint (ask, agent, TUI).
func loadModel(cwd string) (config.Config, model.Model, *slog.Logger, error) {
	cfg, err := config.Load(cwd)
	if err != nil {
		return config.Config{}, nil, nil, fmt.Errorf("load config: %w", err)
	}

	logger := logging.New(cfg.Logging.Level)

	mc, ok := cfg.Models["fast"]
	if !ok {
		return config.Config{}, nil, nil, fmt.Errorf("no %q model configured", "fast")
	}
	if mc.Provider != "ollama" {
		return config.Config{}, nil, nil, fmt.Errorf("unsupported model provider %q", mc.Provider)
	}

	m := model.NewOllama(model.OllamaConfig{
		BaseURL: mc.BaseURL,
		Model:   mc.Model,
	})
	return cfg, m, logger, nil
}

// runTUI loads config/model the same way runAsk does and hands off to the
// interactive session, which owns the terminal until the user quits.
func runTUI() error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}

	cfg, m, logger, err := loadModel(cwd)
	if err != nil {
		return err
	}

	return tui.Run(cfg, cwd, m, logger)
}

// runAsk sends prompt straight to the model with no tools and no repository
// context — the Milestone 1 acceptance path, kept as a raw escape hatch.
func runAsk(args []string) error {
	prompt := strings.TrimSpace(strings.Join(args, " "))
	if prompt == "" {
		return fmt.Errorf("usage: optimus ask \"<prompt>\"")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}

	_, m, logger, err := loadModel(cwd)
	if err != nil {
		return err
	}

	logger.Debug("sending request", "prompt", prompt)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	events, err := m.Stream(ctx, model.ModelRequest{
		Messages: []model.Message{
			{Role: model.RoleUser, Content: prompt},
		},
	})
	if err != nil {
		return fmt.Errorf("start generation: %w", err)
	}

	for ev := range events {
		switch ev.Type {
		case model.EventToken:
			fmt.Print(ev.Token)
		case model.EventError:
			fmt.Println()
			return fmt.Errorf("generation failed: %w", ev.Err)
		case model.EventDone:
			fmt.Println()
			logger.Debug("generation complete",
				"prompt_tokens", ev.Response.PromptTokens,
				"completion_tokens", ev.Response.CompletionTokens,
			)
		}
	}

	return nil
}

// runAgent is Milestone 2's acceptance path: `optimus "<task>"` lets the
// model inspect the real repository (list_directory, read_file,
// search_code, git_status, git_diff) before answering. It cannot modify
// anything yet — that's Milestone 3.
func runAgent(args []string) error {
	prompt := strings.TrimSpace(strings.Join(args, " "))
	if prompt == "" {
		return fmt.Errorf("usage: optimus \"<task>\"")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}

	cfg, m, logger, err := loadModel(cwd)
	if err != nil {
		return err
	}

	repo := repository.New(cwd)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := repo.Scan(ctx); err != nil {
		return fmt.Errorf("scan repository: %w", err)
	}
	logger.Debug("scanned repository", "files", len(repo.Files()), "project_types", repo.ProjectTypes())

	registry := tools.NewRegistry()
	registry.Register(&tools.ListDirectoryTool{Root: cwd})
	registry.Register(&tools.ReadFileTool{Root: cwd})
	registry.Register(&tools.SearchCodeTool{Repo: repo})
	registry.Register(&tools.GitStatusTool{Root: cwd})
	registry.Register(&tools.GitDiffTool{Root: cwd})

	overview := fmt.Sprintf("Repository root contents: %s", strings.Join(repository.RootEntries(repo.Files()), ", "))
	if types := repo.ProjectTypes(); len(types) > 0 {
		overview += fmt.Sprintf("\nDetected project type(s): %s", strings.Join(types, ", "))
	}

	a := &agent.Agent{
		Model:        m,
		Tools:        registry,
		MaxToolCalls: cfg.Agent.MaxToolCalls,
		RepoOverview: overview,
		OnEvent:      printAgentEvent,
	}

	answer, err := a.Run(ctx, prompt)
	if err != nil {
		return err
	}

	fmt.Println(answer)
	return nil
}

func printAgentEvent(ev agent.Event) {
	switch ev.Type {
	case agent.EventToolCall:
		fmt.Printf("→ %s(%s)\n", ev.Tool, string(ev.Input))
	case agent.EventToolResult:
		if ev.Err != nil {
			fmt.Printf("  error: %v\n", ev.Err)
			return
		}
		fmt.Printf("  %s\n", truncate(ev.Result, 200))
	}
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
