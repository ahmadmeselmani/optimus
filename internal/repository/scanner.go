package repository

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// defaultSkipDirs are pruned during the non-git fallback walk. They're never
// useful repository content and can be enormous.
var defaultSkipDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"vendor":       true,
	".optimus":     true,
}

// Scanner enumerates the files in a project rooted at root, respecting
// .gitignore. See PLAN.md §10.
type Scanner struct {
	root string
}

// NewScanner builds a Scanner rooted at root (an absolute path).
func NewScanner(root string) *Scanner {
	return &Scanner{root: root}
}

// Scan lists the repository's files. Inside a Git work tree it shells out
// to `git ls-files`, which is a correct, dependency-free way to honor
// .gitignore (including nested and global ignore rules) without writing a
// gitignore parser ourselves (PLAN.md §48: no unjustified dependencies).
// Outside a Git work tree it falls back to a plain filesystem walk that
// skips a small set of well-known noise directories.
func (s *Scanner) Scan(ctx context.Context) ([]FileInfo, error) {
	if files, ok, err := s.scanGit(ctx); ok {
		return files, err
	}
	return s.scanWalk()
}

func (s *Scanner) scanGit(ctx context.Context) ([]FileInfo, bool, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return nil, false, nil
	}

	// Confirm root is inside a work tree before trusting `git ls-files`;
	// otherwise git would silently walk upward to an unrelated repo.
	check := exec.CommandContext(ctx, "git", "-C", s.root, "rev-parse", "--is-inside-work-tree")
	if err := check.Run(); err != nil {
		return nil, false, nil
	}

	// Cached (tracked) + others (untracked), excluding anything .gitignore
	// (or .git/info/exclude, or the user's global excludes) would exclude.
	cmd := exec.CommandContext(ctx, "git", "-C", s.root, "ls-files", "--cached", "--others", "--exclude-standard")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return nil, false, nil
	}

	var files []FileInfo
	for _, line := range strings.Split(stdout.String(), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		files = append(files, FileInfo{Path: filepath.ToSlash(line)})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, true, nil
}

func (s *Scanner) scanWalk() ([]FileInfo, error) {
	var files []FileInfo
	err := filepath.WalkDir(s.root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(s.root, path)
		if relErr != nil {
			return relErr
		}
		if rel == "." {
			return nil
		}
		if d.IsDir() {
			if defaultSkipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		files = append(files, FileInfo{Path: filepath.ToSlash(rel)})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

// projectMarkers maps a marker file (relative to the repo root) to the
// project type it indicates. Approximate detection is sufficient (PLAN.md
// §10) — this is not meant to be exhaustive.
var projectMarkers = map[string]string{
	"go.mod":           "go",
	"package.json":     "node",
	"pyproject.toml":   "python",
	"requirements.txt": "python",
	"setup.py":         "python",
	"Cargo.toml":       "rust",
	"pom.xml":          "java",
	"build.gradle":     "java",
}

// RootEntries reports the repository's top-level file and directory names
// (directories suffixed with "/"), sorted. It's meant to ground a model with
// a cheap, deterministic overview of the repository layout up front, so it
// doesn't have to spend tool calls guessing top-level paths (PLAN.md §3.1:
// prefer deterministic retrieval over letting the model flounder).
func RootEntries(files []FileInfo) []string {
	seen := map[string]bool{}
	for _, f := range files {
		root := f.Path
		if i := strings.IndexByte(root, '/'); i != -1 {
			root = root[:i] + "/"
		}
		seen[root] = true
	}

	entries := make([]string, 0, len(seen))
	for e := range seen {
		entries = append(entries, e)
	}
	sort.Strings(entries)
	return entries
}

// DetectProjectTypes reports which project types a file list looks like, by
// checking for well-known marker files at the repository root.
func DetectProjectTypes(files []FileInfo) []string {
	present := map[string]bool{}
	for _, f := range files {
		if strings.Contains(f.Path, "/") {
			continue // marker files are only checked at the root
		}
		if t, ok := projectMarkers[f.Path]; ok {
			present[t] = true
		}
	}

	var types []string
	for t := range present {
		types = append(types, t)
	}
	sort.Strings(types)
	return types
}
