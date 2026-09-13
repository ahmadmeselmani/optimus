package repository

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func mustRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v: %v\n%s", args, err, out)
	}
}

func newGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	mustRun(t, dir, "git", "init", "-q")
	mustRun(t, dir, "git", "config", "user.email", "test@example.com")
	mustRun(t, dir, "git", "config", "user.name", "test")
	return dir
}

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

func TestScannerRespectsGitignore(t *testing.T) {
	dir := newGitRepo(t)
	writeFile(t, dir, ".gitignore", "ignored.txt\nbuild/\n")
	writeFile(t, dir, "main.go", "package main\n")
	writeFile(t, dir, "ignored.txt", "should not appear\n")
	writeFile(t, dir, "build/output.bin", "junk\n")
	writeFile(t, dir, "untracked.go", "package main\n") // untracked but not ignored

	files, err := NewScanner(dir).Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	got := map[string]bool{}
	for _, f := range files {
		got[f.Path] = true
	}

	if !got["main.go"] {
		t.Error("expected main.go to be scanned")
	}
	if !got["untracked.go"] {
		t.Error("expected untracked.go to be scanned")
	}
	if got["ignored.txt"] {
		t.Error("expected ignored.txt to be excluded")
	}
	if got["build/output.bin"] {
		t.Error("expected build/output.bin to be excluded")
	}
	if got[".gitignore"] == false {
		t.Error("expected .gitignore itself to be tracked/listed")
	}
}

func TestScannerFallbackWithoutGit(t *testing.T) {
	dir := t.TempDir() // not a git repo
	writeFile(t, dir, "main.go", "package main\n")
	writeFile(t, dir, "node_modules/pkg/index.js", "module.exports = {}\n")

	files, err := NewScanner(dir).Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	got := map[string]bool{}
	for _, f := range files {
		got[f.Path] = true
	}
	if !got["main.go"] {
		t.Error("expected main.go to be scanned")
	}
	if got["node_modules/pkg/index.js"] {
		t.Error("expected node_modules to be skipped in fallback walk")
	}
}

func TestRootEntries(t *testing.T) {
	files := []FileInfo{
		{Path: "go.mod"},
		{Path: "cmd/optimus/main.go"},
		{Path: "internal/agent/agent.go"},
		{Path: "internal/model/model.go"},
	}
	entries := RootEntries(files)
	want := []string{"cmd/", "go.mod", "internal/"}
	if len(entries) != len(want) {
		t.Fatalf("got %v, want %v", entries, want)
	}
	for i := range want {
		if entries[i] != want[i] {
			t.Fatalf("got %v, want %v", entries, want)
		}
	}
}

func TestDetectProjectTypes(t *testing.T) {
	files := []FileInfo{{Path: "go.mod"}, {Path: "src/main.go"}, {Path: "package.json"}}
	types := DetectProjectTypes(files)
	if len(types) != 2 || types[0] != "go" || types[1] != "node" {
		t.Fatalf("got %v, want [go node]", types)
	}
}
