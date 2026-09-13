package tui

import (
	"fmt"
	"log/slog"

	tea "github.com/charmbracelet/bubbletea"

	"optimus/internal/config"
	"optimus/internal/model"
)

// Run starts the interactive session and blocks until the user quits
// (ctrl+c or /bye, /quit, /exit).
func Run(cfg config.Config, cwd string, m model.Model, logger *slog.Logger) error {
	// Deliberately no tea.WithMouseCellMotion(): claiming the mouse would let
	// the viewport scroll on wheel events, but it also stops the terminal's
	// own click-drag text selection from working. PgUp/PgDown already
	// scroll the viewport without the mouse, so the trade-off isn't worth
	// it — selection should just work like any other terminal program.
	program := tea.NewProgram(
		New(cfg, cwd, m, logger),
		tea.WithAltScreen(),
	)
	if _, err := program.Run(); err != nil {
		return fmt.Errorf("tui: %w", err)
	}
	return nil
}
