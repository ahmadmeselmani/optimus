package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// maxReadBytes bounds how much of a file read_file returns, so one huge
// file can't blow the model's context budget (PLAN.md §13). Callers that
// need more should search within the file instead of dumping it whole.
const maxReadBytes = 64 * 1024

// resolveInRoot cleans and joins userPath against root, refusing to escape
// it. This is the one thing every filesystem tool must do before touching
// disk (PLAN.md §38: prevent path traversal, prevent writes/reads outside
// the project).
func resolveInRoot(root, userPath string) (string, error) {
	if userPath == "" {
		userPath = "."
	}
	cleaned := filepath.Clean(filepath.FromSlash(userPath))
	if filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("path must be relative to the repository root, got %q", userPath)
	}

	full := filepath.Join(root, cleaned)
	rel, err := filepath.Rel(root, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes the repository root", userPath)
	}
	return full, nil
}

// ListDirectoryTool lists the entries of a directory within the repository.
type ListDirectoryTool struct {
	Root string
}

func (t *ListDirectoryTool) Name() string { return "list_directory" }

func (t *ListDirectoryTool) Description() string {
	return "List files and subdirectories at a path relative to the repository root."
}

func (t *ListDirectoryTool) Schema() ToolSchema {
	return ToolSchema{
		Type: "object",
		Properties: map[string]PropertySchema{
			"path": {Type: "string", Description: `Directory path relative to the repository root, e.g. "." or "internal/agent".`},
		},
	}
}

type listDirectoryInput struct {
	Path string `json:"path"`
}

func (t *ListDirectoryTool) Execute(ctx context.Context, input json.RawMessage) (ToolResult, error) {
	var in listDirectoryInput
	if len(input) > 0 {
		if err := json.Unmarshal(input, &in); err != nil {
			return ToolResult{}, fmt.Errorf("list_directory: invalid input: %w", err)
		}
	}

	dir, err := resolveInRoot(t.Root, in.Path)
	if err != nil {
		return ToolResult{}, fmt.Errorf("list_directory: %w", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return ToolResult{}, fmt.Errorf("list_directory: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	var b strings.Builder
	for _, e := range entries {
		if e.IsDir() {
			fmt.Fprintf(&b, "%s/\n", e.Name())
		} else {
			fmt.Fprintf(&b, "%s\n", e.Name())
		}
	}
	if b.Len() == 0 {
		return ToolResult{Content: "(empty directory)"}, nil
	}
	return ToolResult{Content: b.String()}, nil
}

// ReadFileTool reads a file's contents within the repository.
type ReadFileTool struct {
	Root string
}

func (t *ReadFileTool) Name() string { return "read_file" }

func (t *ReadFileTool) Description() string {
	return fmt.Sprintf("Read a file's contents (up to %d bytes) at a path relative to the repository root.", maxReadBytes)
}

func (t *ReadFileTool) Schema() ToolSchema {
	return ToolSchema{
		Type: "object",
		Properties: map[string]PropertySchema{
			"path": {Type: "string", Description: `File path relative to the repository root, e.g. "internal/agent/agent.go".`},
		},
		Required: []string{"path"},
	}
}

type readFileInput struct {
	Path string `json:"path"`
}

func (t *ReadFileTool) Execute(ctx context.Context, input json.RawMessage) (ToolResult, error) {
	var in readFileInput
	if err := json.Unmarshal(input, &in); err != nil {
		return ToolResult{}, fmt.Errorf("read_file: invalid input: %w", err)
	}
	if in.Path == "" {
		return ToolResult{}, fmt.Errorf("read_file: %q is required", "path")
	}

	path, err := resolveInRoot(t.Root, in.Path)
	if err != nil {
		return ToolResult{}, fmt.Errorf("read_file: %w", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		return ToolResult{}, fmt.Errorf("read_file: %w", err)
	}
	if info.IsDir() {
		return ToolResult{}, fmt.Errorf("read_file: %q is a directory, use list_directory", in.Path)
	}

	f, err := os.Open(path)
	if err != nil {
		return ToolResult{}, fmt.Errorf("read_file: %w", err)
	}
	defer f.Close()

	buf := make([]byte, maxReadBytes)
	n, err := f.Read(buf)
	if err != nil && n == 0 {
		return ToolResult{}, fmt.Errorf("read_file: %w", err)
	}

	content := string(buf[:n])
	if info.Size() > int64(n) {
		content += fmt.Sprintf("\n... (truncated, file is %d bytes)", info.Size())
	}
	return ToolResult{Content: content}, nil
}
