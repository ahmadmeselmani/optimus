package repository

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// maxSearchResults bounds how many matches Search returns, so a broad query
// can't blow past the model's context budget (PLAN.md §13).
const maxSearchResults = 200

// search runs a regular-expression search for query over files (relative to
// root), preferring ripgrep (PLAN.md §3.4/§10) when it's on PATH and falling
// back to an in-process scan otherwise.
func search(ctx context.Context, root string, files []FileInfo, query string) ([]SearchResult, error) {
	if query == "" {
		return nil, fmt.Errorf("repository: empty search query")
	}

	if _, err := exec.LookPath("rg"); err == nil {
		results, err := searchRipgrep(ctx, root, query)
		if err == nil {
			return results, nil
		}
		// Fall through to the manual scan if rg itself errors out (e.g. an
		// invalid regex) rather than failing the whole tool call.
	}

	return searchManual(files, root, query)
}

func searchRipgrep(ctx context.Context, root, query string) ([]SearchResult, error) {
	cmd := exec.CommandContext(ctx, "rg",
		"--line-number", "--no-heading", "--color=never",
		"--max-count", "50", // per-file cap
		"--", query, ".",
	)
	cmd.Dir = root

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
			// Exit code 1 from rg just means "no matches"; anything else
			// (2 = usage/regex error, or a non-exec error) is a real failure.
			return nil, fmt.Errorf("rg: %v: %s", err, stderr.String())
		}
	}

	var results []SearchResult
	scanner := bufio.NewScanner(&stdout)
	for scanner.Scan() && len(results) < maxSearchResults {
		// Format: <path>:<line>:<text>
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 3)
		if len(parts) != 3 {
			continue
		}
		lineNo, err := strconv.Atoi(parts[1])
		if err != nil {
			continue
		}
		results = append(results, SearchResult{
			File: filepath.ToSlash(strings.TrimPrefix(parts[0], "./")),
			Line: lineNo,
			Text: strings.TrimSpace(parts[2]),
		})
	}
	return results, nil
}

func searchManual(files []FileInfo, root, query string) ([]SearchResult, error) {
	re, err := regexp.Compile(query)
	if err != nil {
		return nil, fmt.Errorf("repository: invalid search query: %w", err)
	}

	var results []SearchResult
	for _, f := range files {
		if len(results) >= maxSearchResults {
			break
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(f.Path)))
		if err != nil {
			continue // unreadable/binary/gone — skip rather than fail the search
		}
		if isBinary(data) {
			continue
		}

		scanner := bufio.NewScanner(bytes.NewReader(data))
		lineNo := 0
		for scanner.Scan() {
			lineNo++
			line := scanner.Text()
			if re.MatchString(line) {
				results = append(results, SearchResult{File: f.Path, Line: lineNo, Text: strings.TrimSpace(line)})
				if len(results) >= maxSearchResults {
					break
				}
			}
		}
	}
	return results, nil
}

func isBinary(data []byte) bool {
	if len(data) > 8000 {
		data = data[:8000]
	}
	return bytes.ContainsRune(data, 0)
}
