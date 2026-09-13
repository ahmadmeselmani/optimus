package tools

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
)

func newGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "test"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir
}

func TestGitStatusToolClean(t *testing.T) {
	dir := newGitRepo(t)
	tool := &GitStatusTool{Root: dir}
	result, err := tool.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result.Content, "clean") {
		t.Fatalf("expected clean status, got %q", result.Content)
	}
}

func TestGitStatusToolReportsUntracked(t *testing.T) {
	dir := newGitRepo(t)
	writeFile(t, dir, "new.txt", "hi\n")

	tool := &GitStatusTool{Root: dir}
	result, err := tool.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result.Content, "new.txt") {
		t.Fatalf("expected new.txt in status, got %q", result.Content)
	}
}

func TestGitDiffToolNoChanges(t *testing.T) {
	dir := newGitRepo(t)
	tool := &GitDiffTool{Root: dir}
	result, err := tool.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result.Content, "no changes") {
		t.Fatalf("expected no changes, got %q", result.Content)
	}
}

func TestGitDiffToolShowsChange(t *testing.T) {
	dir := newGitRepo(t)
	writeFile(t, dir, "tracked.txt", "v1\n")
	cmd := exec.Command("git", "add", "tracked.txt")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	cmd = exec.Command("git", "commit", "-q", "-m", "init")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}
	writeFile(t, dir, "tracked.txt", "v2\n")

	tool := &GitDiffTool{Root: dir}
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"staged":false}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result.Content, "-v1") || !strings.Contains(result.Content, "+v2") {
		t.Fatalf("expected diff to show the change, got %q", result.Content)
	}
}
