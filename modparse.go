package main

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/mod/modfile"
)

// Module represents a parsed go.mod dependency.
type Module struct {
	Path    string // full module path, e.g. "github.com/foo/bar/v2"
	Version string
	Direct  bool
	Owner   string // GitHub owner (empty if non-GitHub)
	Repo    string // GitHub repo name (empty if non-GitHub)
}

// ParseGoMod reads and parses a go.mod file, returning all required modules.
func ParseGoMod(path string) ([]Module, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading go.mod: %w", err)
	}

	f, err := modfile.Parse(path, data, nil)
	if err != nil {
		return nil, fmt.Errorf("parsing go.mod: %w", err)
	}

	var modules []Module
	for _, req := range f.Require {
		m := Module{
			Path:    req.Mod.Path,
			Version: req.Mod.Version,
			Direct:  !req.Indirect,
		}
		m.Owner, m.Repo = extractGitHub(req.Mod.Path)
		modules = append(modules, m)
	}
	return modules, nil
}

// extractGitHub extracts the GitHub owner and repo from a module path.
// Returns ("", "") for non-GitHub modules.
// Handles paths like:
//   - github.com/foo/bar           → (foo, bar)
//   - github.com/foo/bar/v2        → (foo, bar)
//   - github.com/foo/bar/sdk/v2    → (foo, bar)
func extractGitHub(path string) (owner, repo string) {
	if !strings.HasPrefix(path, "github.com/") {
		return "", ""
	}
	parts := strings.SplitN(path, "/", 4) // ["github.com", owner, repo, ...]
	if len(parts) < 3 {
		return "", ""
	}
	return parts[1], parts[2]
}

// ModuleName reads the module path (the "module" directive) from a go.mod file.
func ModuleName(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	f, err := modfile.Parse(path, data, nil)
	if err != nil {
		return "", err
	}
	if f.Module == nil {
		return "", fmt.Errorf("no module directive in %s", path)
	}
	return f.Module.Mod.Path, nil
}

// FilterGitHub separates modules into GitHub and non-GitHub.
// GitHub modules are deduplicated by owner/repo.
func FilterGitHub(modules []Module, directOnly bool) (github []Module, nonGitHubCount int) {
	seen := make(map[string]bool)
	for _, m := range modules {
		if directOnly && !m.Direct {
			continue
		}
		if m.Owner == "" {
			nonGitHubCount++
			continue
		}
		key := m.Owner + "/" + m.Repo
		if seen[key] {
			continue
		}
		seen[key] = true
		github = append(github, m)
	}
	return github, nonGitHubCount
}
