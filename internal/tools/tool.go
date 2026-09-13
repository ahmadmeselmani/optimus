// Package tools implements the Tool interface (PLAN.md §6/§15) the agent
// uses to inspect a repository: listing directories, reading files,
// searching code, and reading Git status/diff. Editing (apply_patch) and
// shell execution belong to later milestones.
package tools

import (
	"context"
	"encoding/json"
)

// PropertySchema describes one field of a Tool's JSON input.
type PropertySchema struct {
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}

// ToolSchema is a minimal JSON-Schema-shaped description of a Tool's input,
// enough for a model to know what to send without pulling in a full JSON
// Schema library (PLAN.md §48: no unjustified dependencies).
type ToolSchema struct {
	Type       string                    `json:"type"`
	Properties map[string]PropertySchema `json:"properties,omitempty"`
	Required   []string                  `json:"required,omitempty"`
}

// ToolResult is what a Tool returns on success. Content is plain text meant
// to be fed back to the model.
type ToolResult struct {
	Content string
}

// Tool is one capability the agent runtime can offer the model. See
// PLAN.md §6.
type Tool interface {
	Name() string
	Description() string
	Schema() ToolSchema
	Execute(ctx context.Context, input json.RawMessage) (ToolResult, error)
}
