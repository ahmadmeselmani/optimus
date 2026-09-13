package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/x/ansi"
)

func TestAssistantReplySpacing(t *testing.T) {
	for _, prefix := range []string{"", "\n\n", " \t\r\n\r\n"} {
		content := prefix + "Hello!\n\nNext paragraph.\n    code"
		want := "Optimus\nHello!\n\nNext paragraph.\n    code"
		got := ansi.Strip(renderMessage(chatMessage{role: "assistant", content: content}, 0))
		if got != want {
			t.Fatalf("completed reply: got %q, want %q", got, want)
		}
		m := &Model{ready: true, streaming: true, viewport: viewport.New(80, 12)}
		m.streamContent.WriteString(content)
		m.syncViewport()
		got = ansi.Strip(m.viewport.View())
		lines := strings.Split(got, "\n")
		if len(lines) < 2 || strings.TrimRight(lines[0], " ") != "Optimus" || strings.TrimRight(lines[1], " ") != "Hello!" {
			t.Fatalf("streaming reply has a gap: %q", got)
		}
	}
}

func TestAssistantReplyPreservesIndentation(t *testing.T) {
	content := "\n\n    indented code\n\n    more code"
	want := "Optimus\n    indented code\n\n    more code"
	if got := ansi.Strip(renderMessage(chatMessage{role: "assistant", content: content}, 0)); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	m := &Model{ready: true, streaming: true, viewport: viewport.New(80, 12)}
	m.streamContent.WriteString("\n \t\n")
	m.syncViewport()
	if got := ansi.Strip(m.viewport.View()); !strings.Contains(got, "thinking…") {
		t.Fatalf("blank tokens hid the spinner: %q", got)
	}
}
