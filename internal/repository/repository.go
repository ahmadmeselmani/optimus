// Package repository gives the agent deterministic ways to inspect a real
// codebase: enumerating its files, searching their contents, and (in later
// milestones) resolving symbols and references. See PLAN.md §10/§11 for the
// full design; this package currently implements only the file scanner and
// search described for Milestone 2.
package repository

import (
	"context"
	"errors"
)

// ErrNotImplemented is returned by Repository methods that belong to a
// later milestone (symbol/reference resolution needs the tree-sitter-backed
// index built in Milestone 6). Returning this instead of a fake or empty
// result keeps callers honest about what Optimus can actually do today.
var ErrNotImplemented = errors.New("repository: not implemented until Milestone 6 (Repository Intelligence)")

// FileInfo describes one file found by Scan, relative to the repository
// root, using forward slashes regardless of OS.
type FileInfo struct {
	Path string
}

// SearchResult is one line matched by Search.
type SearchResult struct {
	File string
	Line int
	Text string
}

// Symbol identifies a named declaration in the repository. Populated by
// Milestone 6.
type Symbol struct {
	Name string
	File string
	Line int
}

// Reference is a use site of a Symbol. Populated by Milestone 6.
type Reference struct {
	File string
	Line int
}

// Repository is the interface the agent runtime depends on for inspecting a
// codebase. See PLAN.md §6.
type Repository interface {
	Scan(ctx context.Context) error
	Search(ctx context.Context, query string) ([]SearchResult, error)
	FindSymbol(ctx context.Context, name string) ([]Symbol, error)
	FindReferences(ctx context.Context, symbol Symbol) ([]Reference, error)
}

// FileRepository is the Milestone 2 Repository implementation: a scanner
// plus a deterministic text search over the files it finds.
type FileRepository struct {
	root    string
	scanner *Scanner

	files []FileInfo
}

// New builds a FileRepository rooted at root (an absolute path to a project
// directory).
func New(root string) *FileRepository {
	return &FileRepository{
		root:    root,
		scanner: NewScanner(root),
	}
}

// Root returns the absolute path this repository is rooted at.
func (r *FileRepository) Root() string {
	return r.root
}

// Scan enumerates the repository's files, respecting .gitignore, and caches
// the result for Files/ProjectTypes.
func (r *FileRepository) Scan(ctx context.Context) error {
	files, err := r.scanner.Scan(ctx)
	if err != nil {
		return err
	}
	r.files = files
	return nil
}

// Files returns the file list produced by the most recent Scan. Callers
// must call Scan first.
func (r *FileRepository) Files() []FileInfo {
	return r.files
}

// ProjectTypes reports the project types detected from the most recent
// Scan's file list (e.g. "go", "node"). Callers must call Scan first.
func (r *FileRepository) ProjectTypes() []string {
	return DetectProjectTypes(r.files)
}

// Search runs a deterministic text search for query across the repository,
// preferring ripgrep when it's available on PATH. See search.go.
func (r *FileRepository) Search(ctx context.Context, query string) ([]SearchResult, error) {
	return search(ctx, r.root, r.files, query)
}

// FindSymbol is not implemented until Milestone 6.
func (r *FileRepository) FindSymbol(ctx context.Context, name string) ([]Symbol, error) {
	return nil, ErrNotImplemented
}

// FindReferences is not implemented until Milestone 6.
func (r *FileRepository) FindReferences(ctx context.Context, symbol Symbol) ([]Reference, error) {
	return nil, ErrNotImplemented
}
