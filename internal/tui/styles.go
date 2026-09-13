package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Color palette. Kept small and terminal-friendly (works on dark and light
// backgrounds) rather than pulling in a full theming system.
var (
	colorAccent = lipgloss.Color("39")  // blue
	colorMuted  = lipgloss.Color("244") // gray
	colorGood   = lipgloss.Color("42")  // green
	colorError  = lipgloss.Color("203") // red
)

var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorAccent)

	headerMetaStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	dividerStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	userLabelStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorAccent)

	assistantLabelStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorGood)

	systemStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			Italic(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(colorError)

	footerStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	inputPromptStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorAccent)

	suggestionActiveStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorAccent)

	suggestionStyle = lipgloss.NewStyle().
			Foreground(colorMuted)
)

// wrapText word-wraps s to width columns (ANSI-aware via lipgloss), so
// free-form conversational text never produces a single line wider than
// the viewport. An unwrapped long line would make the terminal itself
// wrap it, which desyncs Bubble Tea's redraw from the terminal's actual
// row count and corrupts the display on the next frame. Deliberately not
// used on preformatted content (slash-command output, ASCII art), where
// wrapping would break intentional layout instead of fixing anything.
func wrapText(s string, width int) string {
	if width <= 0 {
		return s
	}
	return lipgloss.NewStyle().Width(width).Render(s)
}

// trimLeadingBlankLines keeps model-supplied blank lines from separating
// the assistant label from its reply, without changing code indentation.
func trimLeadingBlankLines(s string) string {
	for {
		line, rest, found := strings.Cut(s, "\n")
		if strings.TrimSpace(line) != "" {
			return s
		}
		if !found {
			return ""
		}
		s = rest
	}
}
