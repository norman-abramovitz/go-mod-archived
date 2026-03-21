package main

import (
	"bufio"
	"os"
	"strings"
)

// IgnoreList holds module paths that should be excluded from results,
// with optional reasons for each entry.
type IgnoreList struct {
	paths map[string]string // module path → reason (empty string if no reason)
}

// NewIgnoreList creates an empty IgnoreList.
func NewIgnoreList() *IgnoreList {
	return &IgnoreList{paths: make(map[string]string)}
}

// Add adds one or more module paths to the ignore list with no reason.
func (il *IgnoreList) Add(paths ...string) {
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p != "" {
			il.paths[p] = ""
		}
	}
}

// AddWithReason adds a module path with an optional reason.
func (il *IgnoreList) AddWithReason(path, reason string) {
	path = strings.TrimSpace(path)
	if path != "" {
		il.paths[path] = reason
	}
}

// IsIgnored returns true if the module path is in the ignore list.
func (il *IgnoreList) IsIgnored(modulePath string) bool {
	_, ok := il.paths[modulePath]
	return ok
}

// Reason returns the reason for ignoring a module path, or empty string.
func (il *IgnoreList) Reason(modulePath string) string {
	return il.paths[modulePath]
}

// Len returns the number of ignored paths.
func (il *IgnoreList) Len() int {
	return len(il.paths)
}

// FilterResults splits results into filtered (kept) and ignored.
func (il *IgnoreList) FilterResults(results []RepoStatus) (filtered, ignored []RepoStatus) {
	for _, r := range results {
		if il.IsIgnored(r.Module.Path) {
			ignored = append(ignored, r)
		} else {
			filtered = append(filtered, r)
		}
	}
	return filtered, ignored
}

// LoadIgnoreFile reads a .modrotignore file and returns an IgnoreList.
// Returns an empty list (not an error) if the file doesn't exist.
// Format: one module path per line, # comments, blank lines skipped.
// Inline comments after a module path are stored as the reason:
//
//	github.com/pkg/errors  # Vendored replacement available
func LoadIgnoreFile(path string) (*IgnoreList, error) {
	il := NewIgnoreList()

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return il, nil
		}
		return nil, err
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Split on inline comment: "module/path  # reason"
		modPath := line
		reason := ""
		if idx := strings.Index(line, "#"); idx >= 0 {
			modPath = strings.TrimSpace(line[:idx])
			reason = strings.TrimSpace(line[idx+1:])
		}
		if modPath != "" {
			il.AddWithReason(modPath, reason)
		}
	}
	return il, scanner.Err()
}

// ParseIgnoreList parses a comma-separated string of module paths.
func ParseIgnoreList(commaSeparated string) *IgnoreList {
	il := NewIgnoreList()
	if commaSeparated == "" {
		return il
	}
	for _, p := range strings.Split(commaSeparated, ",") {
		il.Add(p)
	}
	return il
}

// BuildIgnoreList builds an IgnoreList from the ignore file next to gomodPath
// and inline ignores. If ignoreFile is empty, uses .modrotignore next to gomodPath.
func BuildIgnoreList(gomodDir, ignoreFile, ignoreInline string) *IgnoreList {
	ignoreList := NewIgnoreList()
	filePath := ignoreFile
	if filePath == "" {
		filePath = gomodDir + "/.modrotignore"
	}
	if il, err := LoadIgnoreFile(filePath); err == nil {
		for p, reason := range il.paths {
			ignoreList.AddWithReason(p, reason)
		}
	}
	if ignoreInline != "" {
		inline := ParseIgnoreList(ignoreInline)
		for p := range inline.paths {
			ignoreList.Add(p)
		}
	}
	return ignoreList
}
