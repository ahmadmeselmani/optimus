package tools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"optimus/internal/repository"
)

type fakeRepo struct {
	results []repository.SearchResult
	err     error
}

func (f *fakeRepo) Scan(ctx context.Context) error { return nil }
func (f *fakeRepo) Search(ctx context.Context, query string) ([]repository.SearchResult, error) {
	return f.results, f.err
}
func (f *fakeRepo) FindSymbol(ctx context.Context, name string) ([]repository.Symbol, error) {
	return nil, repository.ErrNotImplemented
}
func (f *fakeRepo) FindReferences(ctx context.Context, symbol repository.Symbol) ([]repository.Reference, error) {
	return nil, repository.ErrNotImplemented
}

func TestSearchCodeTool(t *testing.T) {
	repo := &fakeRepo{results: []repository.SearchResult{
		{File: "main.go", Line: 3, Text: "func main() {}"},
	}}
	tool := &SearchCodeTool{Repo: repo}

	result, err := tool.Execute(context.Background(), json.RawMessage(`{"query":"func main"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	want := "main.go:3: func main() {}\n"
	if result.Content != want {
		t.Fatalf("got %q, want %q", result.Content, want)
	}
}

func TestSearchCodeToolNoMatches(t *testing.T) {
	tool := &SearchCodeTool{Repo: &fakeRepo{}}
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"query":"nope"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Content != "(no matches)" {
		t.Fatalf("got %q", result.Content)
	}
}

func TestSearchCodeToolPropagatesError(t *testing.T) {
	tool := &SearchCodeTool{Repo: &fakeRepo{err: errors.New("boom")}}
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"query":"x"}`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSearchCodeToolRequiresQuery(t *testing.T) {
	tool := &SearchCodeTool{Repo: &fakeRepo{}}
	_, err := tool.Execute(context.Background(), json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for missing query")
	}
}
