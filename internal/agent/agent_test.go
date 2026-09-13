package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"optimus/internal/model"
	"optimus/internal/tools"
)

// scriptedModel replays a fixed sequence of responses, one per Generate
// call, and records the messages it was sent.
type scriptedModel struct {
	replies []string
	calls   [][]model.Message
}

func (m *scriptedModel) Generate(ctx context.Context, req model.ModelRequest) (model.ModelResponse, error) {
	m.calls = append(m.calls, req.Messages)
	if len(m.replies) == 0 {
		return model.ModelResponse{}, fmt.Errorf("scriptedModel: no more replies")
	}
	reply := m.replies[0]
	m.replies = m.replies[1:]
	return model.ModelResponse{Content: reply}, nil
}

func (m *scriptedModel) Stream(ctx context.Context, req model.ModelRequest) (<-chan model.ModelEvent, error) {
	return nil, errors.New("not implemented")
}

func (m *scriptedModel) Capabilities() model.Capabilities { return model.Capabilities{} }

type echoTool struct{ name string }

func (t *echoTool) Name() string             { return t.name }
func (t *echoTool) Description() string      { return "echoes its input" }
func (t *echoTool) Schema() tools.ToolSchema { return tools.ToolSchema{Type: "object"} }
func (t *echoTool) Execute(ctx context.Context, input json.RawMessage) (tools.ToolResult, error) {
	return tools.ToolResult{Content: "echo:" + string(input)}, nil
}

func TestAgentRunCallsToolThenAnswers(t *testing.T) {
	registry := tools.NewRegistry()
	registry.Register(&echoTool{name: "echo"})

	m := &scriptedModel{replies: []string{
		`{"type":"tool_call","tool":"echo","input":{"x":1}}`,
		`{"type":"final_answer","content":"done"}`,
	}}

	var events []Event
	a := &Agent{Model: m, Tools: registry, MaxToolCalls: 5, OnEvent: func(e Event) { events = append(events, e) }}

	answer, err := a.Run(context.Background(), "do the thing")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if answer != "done" {
		t.Fatalf("got %q, want %q", answer, "done")
	}
	if len(m.calls) != 2 {
		t.Fatalf("expected 2 model calls, got %d", len(m.calls))
	}
	// Second call's message history must include the tool's result.
	last := m.calls[1]
	found := false
	for _, msg := range last {
		if msg.Role == model.RoleTool && msg.Content == `echo:{"x":1}` {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected tool result in message history, got %+v", last)
	}

	if len(events) != 2 || events[0].Type != EventToolCall || events[1].Type != EventToolResult {
		t.Fatalf("unexpected events: %+v", events)
	}
}

func TestAgentRunUnknownTool(t *testing.T) {
	registry := tools.NewRegistry()
	m := &scriptedModel{replies: []string{
		`{"type":"tool_call","tool":"nope","input":{}}`,
		`{"type":"final_answer","content":"recovered"}`,
	}}

	a := &Agent{Model: m, Tools: registry, MaxToolCalls: 5}
	answer, err := a.Run(context.Background(), "task")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if answer != "recovered" {
		t.Fatalf("got %q", answer)
	}
}

func TestAgentRunFallsBackToRawReplyWhenUnparsable(t *testing.T) {
	registry := tools.NewRegistry()
	// One more than maxParseRetries allows, all unparsable: Run should nudge
	// the model with a protocolReminder each time before finally giving up
	// and returning the last raw reply.
	m := &scriptedModel{replies: []string{
		"let me think about that",
		"still thinking",
		"just a plain text answer, no JSON here",
	}}

	a := &Agent{Model: m, Tools: registry, MaxToolCalls: 5}
	answer, err := a.Run(context.Background(), "task")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if answer != "just a plain text answer, no JSON here" {
		t.Fatalf("got %q", answer)
	}
	if len(m.calls) != 3 {
		t.Fatalf("expected 3 model calls, got %d", len(m.calls))
	}
}

func TestAgentRunRecoversFromNarrationWithoutJSON(t *testing.T) {
	// Reproduces a real observed failure mode: asked to inspect the repo,
	// the model narrates its intent ("let me list the directory...")
	// instead of emitting a tool_call. Run must nudge it back onto the
	// protocol rather than treating the narration as a finished answer.
	registry := tools.NewRegistry()
	registry.Register(&echoTool{name: "list_directory"})

	m := &scriptedModel{replies: []string{
		"Sure, let me list the current directory's files first.",
		`{"type":"tool_call","tool":"list_directory","input":{"path":"."}}`,
		`{"type":"final_answer","content":"here's the summary"}`,
	}}

	a := &Agent{Model: m, Tools: registry, MaxToolCalls: 5}
	answer, err := a.Run(context.Background(), "inspect current dir and give a summary")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if answer != "here's the summary" {
		t.Fatalf("got %q", answer)
	}

	// The reminder must actually have been sent after the narration.
	secondCallMessages := m.calls[1]
	last := secondCallMessages[len(secondCallMessages)-1]
	if last.Role != model.RoleUser || last.Content != protocolReminder {
		t.Fatalf("expected protocolReminder as the last message before the retry, got %+v", last)
	}
}

func TestAgentRunExtractsJSONFromProse(t *testing.T) {
	registry := tools.NewRegistry()
	m := &scriptedModel{replies: []string{
		"Sure, here you go:\n```json\n{\"type\":\"final_answer\",\"content\":\"42\"}\n```",
	}}

	a := &Agent{Model: m, Tools: registry, MaxToolCalls: 5}
	answer, err := a.Run(context.Background(), "task")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if answer != "42" {
		t.Fatalf("got %q", answer)
	}
}

func TestAgentRunHitsMaxToolCalls(t *testing.T) {
	registry := tools.NewRegistry()
	registry.Register(&echoTool{name: "echo"})

	replies := make([]string, 0)
	for i := 0; i < 3; i++ {
		replies = append(replies, `{"type":"tool_call","tool":"echo","input":{}}`)
	}
	m := &scriptedModel{replies: replies}

	a := &Agent{Model: m, Tools: registry, MaxToolCalls: 3}
	_, err := a.Run(context.Background(), "task")
	if err == nil {
		t.Fatal("expected error when max tool calls is exceeded")
	}
}

func TestParseAction(t *testing.T) {
	act, ok := parseAction(`{"type":"final_answer","content":"hi"}`)
	if !ok || act.Type != "final_answer" || act.Content != "hi" {
		t.Fatalf("got %+v, %v", act, ok)
	}

	_, ok = parseAction("no json here")
	if ok {
		t.Fatal("expected ok=false for text with no JSON")
	}
}
