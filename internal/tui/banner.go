package tui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/common-nighthawk/go-figure"
)

// The startup banner reveals two things in sequence, like a title card:
// the full Optimus mascot (assets/optimus_full.txt — see mascot.go),
// scaled down to fit the terminal (braille.go), wipes in first, then the
// "OPTIMUS" wordmark rendered via github.com/common-nighthawk/go-figure
// wipes in beneath it. bannerFont is "larry3d", chosen for its slanted
// drop-shadow strokes — the closest a monospace font gets to an actual 3D
// logo. Once the banner dismisses, the compact mascot takes over as the
// idle empty-conversation placeholder (see mascot.go's renderIdleMascot).
const bannerFont = "speed"

// wordmarkLines renders the wordmark and pads every line to the same width
// so the reveal animation and column-based color gradient can index into it
// uniformly (go-figure right-trims each line to its own length, which would
// otherwise be ragged).
func wordmarkLines() []string {
	fig := figure.NewFigure("OPTIMUS", bannerFont, true)
	rows := fig.Slicify()

	width := 0
	for _, r := range rows {
		if len(r) > width {
			width = len(r)
		}
	}
	for i, r := range rows {
		if len(r) < width {
			rows[i] = r + strings.Repeat(" ", width-len(r))
		}
	}
	return rows
}

// bannerGradient is the left-to-right color sweep used across both the
// mascot and the wordmark — Autobot blue fading through chrome into red.
var bannerGradient = []string{"27", "33", "39", "45", "252", "250", "247", "203", "196", "160"}

const (
	bannerFrameInterval = 20 * time.Millisecond
	bannerRevealStep    = 2 // columns revealed per tick
	bannerHoldTicks     = 20
)

// bannerTickMsg drives the reveal animation; see Model.bannerTick.
type bannerTickMsg struct{}

func bannerTick() tea.Cmd {
	return tea.Tick(bannerFrameInterval, func(time.Time) tea.Msg {
		return bannerTickMsg{}
	})
}

func ticksFor(cols int) int {
	return (cols + bannerRevealStep - 1) / bannerRevealStep
}

// introReservedRows is how much vertical room the intro sets aside below
// the mascot for the wordmark (7 rows), the blank lines around it, and the
// tagline, when deciding how much space the mascot itself gets to scale
// into. introMargin is a little extra breathing room on top of that.
const (
	introReservedRows = 10
	introMargin       = 2
	introMinRows      = 6
	introMinCols      = 20
)

// introMascotLines is the full mascot, scaled down (braille.go) to fit
// whatever room the terminal actually has — never a different, smaller
// asset, so the intro always shows the real full mascot, just resized.
func introMascotLines(termWidth, termHeight int) []string {
	maxRows := termHeight - introReservedRows - introMargin
	if maxRows < introMinRows {
		maxRows = introMinRows
	}
	maxCols := termWidth - introMargin*2
	if maxCols < introMinCols {
		maxCols = introMinCols
	}
	return scaleMascotToFit(mascotFullLines(), maxRows, maxCols)
}

func bannerMascotTicks(termWidth, termHeight int) int {
	return ticksFor(mascotWidth(introMascotLines(termWidth, termHeight)))
}
func bannerWordmarkTicks() int { return ticksFor(mascotWidth(wordmarkLines())) }

// bannerTotalTicks derives its sizing from the rendered mascot and wordmark
// so timing stays correct if either asset, the phrase/font, or the
// terminal's size (and therefore the mascot's scale) ever changes.
func bannerTotalTicks(termWidth, termHeight int) int {
	return bannerMascotTicks(termWidth, termHeight) + bannerWordmarkTicks() + bannerHoldTicks
}

var bannerTaglineStyle = lipgloss.NewStyle().Foreground(colorMuted).Italic(true)

// renderBanner is the full splash view: the mascot wiping in, then (once
// it's fully revealed) the wordmark wiping in beneath it, then a tagline —
// all centered in the terminal.
func renderBanner(m *Model) string {
	mascot := introMascotLines(m.width, m.height)
	mascotW := mascotWidth(mascot)
	mascotTicks := ticksFor(mascotW)

	mascotReveal := m.bannerTick * bannerRevealStep
	if mascotReveal > mascotW {
		mascotReveal = mascotW
	}
	content := renderArt(mascot, mascotReveal, bannerGradient)

	if m.bannerTick > mascotTicks {
		wordmark := wordmarkLines()
		wordmarkW := mascotWidth(wordmark)
		wordmarkTicks := ticksFor(wordmarkW)

		wTick := m.bannerTick - mascotTicks
		wordmarkReveal := wTick * bannerRevealStep
		if wordmarkReveal > wordmarkW {
			wordmarkReveal = wordmarkW
		}
		content += "\n\n" + renderArt(wordmark, wordmarkReveal, bannerGradient)

		if wTick >= wordmarkTicks {
			content += "\n" + bannerTaglineStyle.Render("local-first terminal coding agent")
		}
	}

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}
