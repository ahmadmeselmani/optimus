package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// runGit executes git with args rooted at root, using argument-based exec
// (never a shell string) per PLAN.md §17/§38.
func runGit(ctx context.Context, root string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// GitStatusTool reports the working tree's status.
type GitStatusTool struct {
	Root string
}

func (t *GitStatusTool) Name() string { return "git_status" }

func (t *GitStatusTool) Description() string {
	return "Show the repository's Git status (modified, added, deleted, untracked files)."
}

func (t *GitStatusTool) Schema() ToolSchema {
	return ToolSchema{Type: "object"}
}

func (t *GitStatusTool) Execute(ctx context.Context, input json.RawMessage) (ToolResult, error) {
	out, err := runGit(ctx, t.Root, "status", "--porcelain=v1", "--branch")
	if err != nil {
		return ToolResult{}, fmt.Errorf("git_status: %w", err)
	}

	// --branch always emits a leading "## <branch info>" line even with a
	// clean working tree, so "no output" isn't the right clean-check.
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	branch, rest := lines[0], lines[1:]
	if len(rest) == 0 {
		return ToolResult{Content: branch + "\n(clean working tree)"}, nil
	}
	return ToolResult{Content: out}, nil
}

// GitDiffTool shows unstaged (or, if requested, staged) changes.
type GitDiffTool struct {
	Root string
}

func (t *GitDiffTool) Name() string { return "git_diff" }

func (t *GitDiffTool) Description() string {
	return "Show the repository's Git diff. By default shows unstaged changes; set staged=true for the index (--cached) diff."
}

func (t *GitDiffTool) Schema() ToolSchema {
	return ToolSchema{
		Type: "object",
		Properties: map[string]PropertySchema{
			"staged": {Type: "boolean", Description: "If true, show the staged (--cached) diff instead of the working tree diff."},
		},
	}
}

type gitDiffInput struct {
	Staged bool `json:"staged"`
}

func (t *GitDiffTool) Execute(ctx context.Context, input json.RawMessage) (ToolResult, error) {
	var in gitDiffInput
	if len(input) > 0 {
		if err := json.Unmarshal(input, &in); err != nil {
			return ToolResult{}, fmt.Errorf("git_diff: invalid input: %w", err)
		}
	}

	args := []string{"diff"}
	if in.Staged {
		args = append(args, "--cached")
	}

	out, err := runGit(ctx, t.Root, args...)
	if err != nil {
		return ToolResult{}, fmt.Errorf("git_diff: %w", err)
	}
	if strings.TrimSpace(out) == "" {
		return ToolResult{Content: "(no changes)"}, nil
	}
	return ToolResult{Content: out}, nil
}
