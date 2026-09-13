// Package agent implements the Milestone 2 agent loop: a bounded
// read-tool-use loop that lets a model inspect a real repository through
// internal/tools before answering. Planning, task queues, editing, and
// validation are later milestones (PLAN.md §35 describes the full loop this
// will grow into).
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"optimus/internal/model"
	"optimus/internal/tools"
)

// EventType identifies what an Event reports.
type EventType string

const (
	// EventToolCall fires just before a tool is executed.
	EventToolCall EventType = "tool_call"
	// EventToolResult fires after a tool finishes (successfully or not).
	EventToolResult EventType = "tool_result"
)

// Event reports agent-loop progress to the caller (e.g. the CLI, to print
// what the agent is doing). The full internal/events bus is a later
// milestone (PLAN.md §25); this is a minimal callback sufficient for M2.
type Event struct {
	Type    EventType
	Tool    string
	Input   json.RawMessage
	Result  string
	Err     error
	Attempt int
}

// Agent runs the bounded tool-use loop against a Model.
type Agent struct {
	Model        model.Model
	Tools        *tools.Registry
	MaxToolCalls int
	// RepoOverview, if set, is folded into the system prompt as a
	// deterministic, cheap summary of the repository (e.g. its top-level
	// layout) so the model doesn't burn tool calls guessing paths blindly.
	RepoOverview string
	// OnEvent, if set, is called synchronously as the loop progresses.
	OnEvent func(Event)
}

// action is the strict, deterministic protocol the model must reply with:
// either it wants to call a tool, or it's ready to answer. Parsing a fixed
// JSON shape — rather than relying on a provider's native function-calling,
// which small local models support inconsistently — keeps the runtime, not
// the prompt, in control (PLAN.md §3.1/§3.2).
type action struct {
	Type    string          `json:"type"`
	Tool    string          `json:"tool"`
	Input   json.RawMessage `json:"input"`
	Content string          `json:"content"`
}

const (
	actionToolCall    = "tool_call"
	actionFinalAnswer = "final_answer"
)

// maxParseRetries bounds how many times Run will nudge the model back onto
// the JSON protocol before giving up and treating its raw reply as the
// final answer. Without this, a model that narrates its intent instead of
// emitting a tool_call (e.g. "Let me list the directory...", with no JSON
// at all) would have that narration treated as a completed answer after a
// single turn, even though it never actually looked at anything.
const maxParseRetries = 2

const protocolReminder = `Reminder: respond with exactly one JSON object and nothing else — ` +
	`{"type":"tool_call","tool":"...","input":{...}} or {"type":"final_answer","content":"..."}. ` +
	`If you need information about the repository (its layout, or a file's contents) to answer, ` +
	`call a tool yourself — do not ask the user or describe what you're about to do in plain text.`

func (a *Agent) maxToolCalls() int {
	if a.MaxToolCalls > 0 {
		return a.MaxToolCalls
	}
	return 15
}

func (a *Agent) emit(ev Event) {
	if a.OnEvent != nil {
		a.OnEvent(ev)
	}
}

// Run answers prompt, letting the model call read-only repository tools
// along the way. It never mutates the repository — every tool available in
// M2 is read-only.
func (a *Agent) Run(ctx context.Context, prompt string) (string, error) {
	messages := []model.Message{
		{Role: model.RoleSystem, Content: a.systemPrompt()},
		{Role: model.RoleUser, Content: prompt},
	}

	limit := a.maxToolCalls()
	parseFailures := 0
	for attempt := 1; attempt <= limit; attempt++ {
		resp, err := a.Model.Generate(ctx, model.ModelRequest{Messages: messages})
		if err != nil {
			return "", fmt.Errorf("agent: model generation failed: %w", err)
		}

		act, ok := parseAction(resp.Content)
		if !ok {
			parseFailures++
			if parseFailures > maxParseRetries {
				// Still ignoring the protocol after being reminded.
				// Treat its raw reply as the final answer rather than
				// failing outright — small local models don't always
				// follow formatting instructions exactly.
				return strings.TrimSpace(resp.Content), nil
			}
			// Give it a chance to self-correct instead of immediately
			// treating narration ("let me list the directory...") as a
			// finished answer when it never actually called anything.
			messages = append(messages, model.Message{Role: model.RoleAssistant, Content: resp.Content})
			messages = append(messages, model.Message{Role: model.RoleUser, Content: protocolReminder})
			continue
		}
		parseFailures = 0

		switch act.Type {
		case actionFinalAnswer, "":
			return strings.TrimSpace(act.Content), nil

		case actionToolCall:
			messages = append(messages, model.Message{Role: model.RoleAssistant, Content: resp.Content})
			result := a.callTool(ctx, act, attempt)
			messages = append(messages, model.Message{Role: model.RoleTool, Content: result})

		default:
			return "", fmt.Errorf("agent: model returned unknown action type %q", act.Type)
		}
	}

	return "", fmt.Errorf("agent: exceeded max tool calls (%d) without a final answer", limit)
}

func (a *Agent) callTool(ctx context.Context, act action, attempt int) string {
	tool, found := a.Tools.Get(act.Tool)
	if !found {
		err := fmt.Errorf("unknown tool %q", act.Tool)
		a.emit(Event{Type: EventToolResult, Tool: act.Tool, Input: act.Input, Err: err, Attempt: attempt})
		return "error: " + err.Error()
	}

	a.emit(Event{Type: EventToolCall, Tool: act.Tool, Input: act.Input, Attempt: attempt})
	result, err := tool.Execute(ctx, act.Input)
	if err != nil {
		a.emit(Event{Type: EventToolResult, Tool: act.Tool, Input: act.Input, Err: err, Attempt: attempt})
		return "error: " + err.Error()
	}

	a.emit(Event{Type: EventToolResult, Tool: act.Tool, Input: act.Input, Result: result.Content, Attempt: attempt})
	return result.Content
}

// parseAction extracts the first balanced top-level JSON object from raw and
// decodes it as an action. Models sometimes wrap JSON in prose or markdown
// fences despite instructions not to; scanning for the first balanced
// object is more robust than requiring the whole reply to be valid JSON.
func parseAction(raw string) (action, bool) {
	start := strings.IndexByte(raw, '{')
	if start == -1 {
		return action{}, false
	}

	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(raw); i++ {
		c := raw[i]
		switch {
		case escaped:
			escaped = false
		case c == '\\' && inString:
			escaped = true
		case c == '"':
			inString = !inString
		case inString:
			// inside a string literal, braces don't count
		case c == '{':
			depth++
		case c == '}':
			depth--
			if depth == 0 {
				var act action
				if err := json.Unmarshal([]byte(raw[start:i+1]), &act); err != nil {
					return action{}, false
				}
				return act, true
			}
		}
	}
	return action{}, false
}
