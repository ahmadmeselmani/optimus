package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestListDirectoryTool(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "a.go", "package a\n")
	writeFile(t, root, "sub/b.go", "package b\n")

	tool := &ListDirectoryTool{Root: root}
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"path":"."}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result.Content, "a.go") || !strings.Contains(result.Content, "sub/") {
		t.Fatalf("unexpected listing: %q", result.Content)
	}
}

func TestReadFileTool(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "hello.txt", "hello world\n")

	tool := &ReadFileTool{Root: root}
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"path":"hello.txt"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Content != "hello world\n" {
		t.Fatalf("got %q", result.Content)
	}
}

func TestReadFileToolRejectsPathTraversal(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "safe.txt", "ok\n")

	tool := &ReadFileTool{Root: root}
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"path":"../../etc/passwd"}`))
	if err == nil {
		t.Fatal("expected path traversal to be rejected")
	}
}

func TestReadFileToolRejectsAbsolutePath(t *testing.T) {
	root := t.TempDir()
	tool := &ReadFileTool{Root: root}
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"path":"/etc/passwd"}`))
	if err == nil {
		t.Fatal("expected absolute path to be rejected")
	}
}

func TestReadFileToolRequiresPath(t *testing.T) {
	tool := &ReadFileTool{Root: t.TempDir()}
	_, err := tool.Execute(context.Background(), json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for missing path")
	}
}
