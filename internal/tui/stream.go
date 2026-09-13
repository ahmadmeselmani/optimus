package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"optimus/internal/model"
)

// Every message below carries the generation it belongs to. The Model bumps
// its generation counter whenever a request starts or is canceled, so a
// message arriving from a request that's no longer current (e.g. the
// "context canceled" error produced by an esc-cancel) is recognized as
// stale and dropped instead of being shown as a second, confusing message.

// streamStartedMsg carries the event channel returned by Model.Stream once
// the request has been accepted.
type streamStartedMsg struct {
	gen    int
	events <-chan model.ModelEvent
}

// streamEventMsg wraps one event read from the channel, plus the channel
// itself so the listener loop can keep reading it.
type streamEventMsg struct {
	gen    int
	event  model.ModelEvent
	events <-chan model.ModelEvent
}

// streamErrMsg reports a failure starting the request (before any event
// could be read, e.g. the model is unreachable).
type streamErrMsg struct {
	gen int
	err error
}

// streamClosedMsg reports that the event channel closed without an
// EventDone/EventError (e.g. the request was canceled). It is a safety net;
// Ollama's adapter always sends a terminal event before closing.
type streamClosedMsg struct {
	gen int
}

// startStream sends req to m and, once the request is accepted, reports the
// event channel back to Update via streamStartedMsg.
func startStream(m model.Model, ctx context.Context, req model.ModelRequest, gen int) tea.Cmd {
	return func() tea.Msg {
		events, err := m.Stream(ctx, req)
		if err != nil {
			return streamErrMsg{gen: gen, err: err}
		}
		return streamStartedMsg{gen: gen, events: events}
	}
}

// listenForEvent reads exactly one event from events and reports it. The
// caller must call listenForEvent again (with the same channel) after
// handling the result to keep draining the stream.
func listenForEvent(events <-chan model.ModelEvent, gen int) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-events
		if !ok {
			return streamClosedMsg{gen: gen}
		}
		return streamEventMsg{gen: gen, event: ev, events: events}
	}
}
