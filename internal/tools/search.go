package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"optimus/internal/repository"
)

// SearchCodeTool runs a deterministic text search over the repository.
type SearchCodeTool struct {
	Repo repository.Repository
}

func (t *SearchCodeTool) Name() string { return "search_code" }

func (t *SearchCodeTool) Description() string {
	return "Search the repository's file contents for a regular expression, returning matching file:line:text."
}

func (t *SearchCodeTool) Schema() ToolSchema {
	return ToolSchema{
		Type: "object",
		Properties: map[string]PropertySchema{
			"query": {Type: "string", Description: "Regular expression to search for."},
		},
		Required: []string{"query"},
	}
}

type searchCodeInput struct {
	Query string `json:"query"`
}

func (t *SearchCodeTool) Execute(ctx context.Context, input json.RawMessage) (ToolResult, error) {
	var in searchCodeInput
	if err := json.Unmarshal(input, &in); err != nil {
		return ToolResult{}, fmt.Errorf("search_code: invalid input: %w", err)
	}
	if in.Query == "" {
		return ToolResult{}, fmt.Errorf("search_code: %q is required", "query")
	}

	results, err := t.Repo.Search(ctx, in.Query)
	if err != nil {
		return ToolResult{}, fmt.Errorf("search_code: %w", err)
	}
	if len(results) == 0 {
		return ToolResult{Content: "(no matches)"}, nil
	}

	var b strings.Builder
	for _, r := range results {
		fmt.Fprintf(&b, "%s:%d: %s\n", r.File, r.Line, r.Text)
	}
	return ToolResult{Content: b.String()}, nil
}
