package repository

import (
	"context"
	"testing"
)

func TestSearchManualFallback(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.go", "package a\n\nfunc Hello() {}\n")
	writeFile(t, dir, "b.go", "package b\n\nfunc World() {}\n")

	files := []FileInfo{{Path: "a.go"}, {Path: "b.go"}}
	results, err := searchManual(files, dir, "func (Hello|World)")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2: %+v", len(results), results)
	}
	if results[0].File != "a.go" || results[0].Line != 3 {
		t.Errorf("unexpected first result: %+v", results[0])
	}
}

func TestSearchInvalidRegex(t *testing.T) {
	if _, err := searchManual(nil, t.TempDir(), "("); err == nil {
		t.Fatal("expected error for invalid regex")
	}
}

func TestFileRepositorySearch(t *testing.T) {
	dir := newGitRepo(t)
	writeFile(t, dir, "main.go", "package main\n\nfunc main() {}\n")

	repo := New(dir)
	if err := repo.Scan(context.Background()); err != nil {
		t.Fatalf("Scan: %v", err)
	}

	results, err := repo.Search(context.Background(), "func main")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 || results[0].File != "main.go" {
		t.Fatalf("got %+v", results)
	}
}
