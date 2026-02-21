package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"
)

// dateFmt controls the date format used in output. Default is date-only;
// set to "2006-01-02 15:04:05" with --time flag to include time.
var dateFmt = "2006-01-02"

// fmtDate formats a time using the current dateFmt setting.
func fmtDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(dateFmt)
}

// PrintTable outputs archived (or all) results in a human-readable table.
func PrintTable(results []RepoStatus, nonGitHubCount int, showAll bool) {
	// Separate archived, not-found, and active
	var archived, notFound, active []RepoStatus
	for _, r := range results {
		switch {
		case r.NotFound:
			notFound = append(notFound, r)
		case r.IsArchived:
			archived = append(archived, r)
		default:
			active = append(active, r)
		}
	}

	sort.Slice(archived, func(i, j int) bool {
		return archived[i].Module.Path < archived[j].Module.Path
	})

	totalChecked := len(results)

	if len(archived) > 0 {
		fmt.Fprintf(os.Stderr, "\nARCHIVED DEPENDENCIES (%d of %d github.com modules)\n\n", len(archived), totalChecked)
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "MODULE\tVERSION\tDIRECT\tARCHIVED AT\tLAST PUSHED")
		for _, r := range archived {
			direct := "indirect"
			if r.Module.Direct {
				direct = "direct"
			}
			archivedAt := fmtDate(r.ArchivedAt)
			pushedAt := fmtDate(r.PushedAt)
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", r.Module.Path, r.Module.Version, direct, archivedAt, pushedAt)
		}
		w.Flush()
	} else {
		fmt.Fprintf(os.Stderr, "\nNo archived dependencies found among %d github.com modules.\n", totalChecked)
	}

	if len(notFound) > 0 {
		fmt.Fprintf(os.Stderr, "\nNOT FOUND (%d modules):\n", len(notFound))
		for _, r := range notFound {
			fmt.Fprintf(os.Stderr, "  %s — %s\n", r.Module.Path, r.Error)
		}
	}

	if showAll && len(active) > 0 {
		fmt.Fprintf(os.Stderr, "\nACTIVE DEPENDENCIES (%d modules)\n\n", len(active))
		sort.Slice(active, func(i, j int) bool {
			return active[i].Module.Path < active[j].Module.Path
		})
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "MODULE\tVERSION\tDIRECT\tLAST PUSHED")
		for _, r := range active {
			direct := "indirect"
			if r.Module.Direct {
				direct = "direct"
			}
			pushedAt := fmtDate(r.PushedAt)
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", r.Module.Path, r.Module.Version, direct, pushedAt)
		}
		w.Flush()
	}

	if nonGitHubCount > 0 {
		fmt.Fprintf(os.Stderr, "\nSkipped %d non-GitHub modules.\n", nonGitHubCount)
	}
}

// PrintFiles outputs a section showing source files that import archived modules.
func PrintFiles(results []RepoStatus, fileMatches map[string][]FileMatch) {
	// Collect archived modules in sorted order
	var archivedPaths []string
	for _, r := range results {
		if r.IsArchived {
			archivedPaths = append(archivedPaths, r.Module.Path)
		}
	}
	sort.Strings(archivedPaths)

	fmt.Fprintf(os.Stderr, "\nSOURCE FILES IMPORTING ARCHIVED MODULES\n")

	for _, modPath := range archivedPaths {
		matches := fileMatches[modPath]
		// Deduplicate by file (show each file only once per module)
		uniqueFiles := make(map[string]bool)
		for _, m := range matches {
			uniqueFiles[m.File] = true
		}

		fmt.Fprintf(os.Stdout, "\n%s (%d %s)\n", modPath, len(uniqueFiles), pluralize(len(uniqueFiles), "file", "files"))
		for _, m := range matches {
			fmt.Fprintf(os.Stdout, "  %s:%d\n", m.File, m.Line)
		}
	}
}

// pluralize returns singular or plural form based on count.
func pluralize(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}

// JSONOutput is the structure for JSON output mode.
type JSONOutput struct {
	Archived       []JSONModule `json:"archived"`
	NotFound       []JSONModule `json:"not_found,omitempty"`
	Active         []JSONModule `json:"active,omitempty"`
	SkippedNonGH   int          `json:"skipped_non_github"`
	TotalChecked   int          `json:"total_checked"`
}

type JSONModule struct {
	Module      string           `json:"module"`
	Version     string           `json:"version"`
	Direct      bool             `json:"direct"`
	Owner       string           `json:"owner"`
	Repo        string           `json:"repo"`
	ArchivedAt  string           `json:"archived_at,omitempty"`
	PushedAt    string           `json:"pushed_at,omitempty"`
	Error       string           `json:"error,omitempty"`
	SourceFiles []JSONSourceFile `json:"source_files,omitempty"`
}

// JSONSourceFile represents a source file match in JSON output.
type JSONSourceFile struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Import string `json:"import"`
}

// buildJSONOutput creates the JSONOutput data structure without writing it.
func buildJSONOutput(results []RepoStatus, nonGitHubCount int, showAll bool, fileMatches map[string][]FileMatch) JSONOutput {
	out := JSONOutput{
		SkippedNonGH: nonGitHubCount,
		TotalChecked: len(results),
		Archived:     []JSONModule{},
	}

	for _, r := range results {
		jm := JSONModule{
			Module:  r.Module.Path,
			Version: r.Module.Version,
			Direct:  r.Module.Direct,
			Owner:   r.Module.Owner,
			Repo:    r.Module.Repo,
		}
		if !r.PushedAt.IsZero() {
			jm.PushedAt = r.PushedAt.Format("2006-01-02T15:04:05Z")
		}

		switch {
		case r.NotFound:
			jm.Error = r.Error
			out.NotFound = append(out.NotFound, jm)
		case r.IsArchived:
			if !r.ArchivedAt.IsZero() {
				jm.ArchivedAt = r.ArchivedAt.Format("2006-01-02T15:04:05Z")
			}
			if fileMatches != nil {
				for _, fm := range fileMatches[r.Module.Path] {
					jm.SourceFiles = append(jm.SourceFiles, JSONSourceFile{
						File:   fm.File,
						Line:   fm.Line,
						Import: fm.ImportPath,
					})
				}
			}
			out.Archived = append(out.Archived, jm)
		default:
			if showAll {
				out.Active = append(out.Active, jm)
			}
		}
	}

	return out
}

// PrintJSON outputs results as JSON. If fileMatches is non-nil, archived
// modules will include source_files arrays.
func PrintJSON(results []RepoStatus, nonGitHubCount int, showAll bool, fileMatches map[string][]FileMatch) {
	out := buildJSONOutput(results, nonGitHubCount, showAll, fileMatches)
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(out)
}

// formatArchivedLine returns a formatted string with version, archived date, and last pushed date.
// modPath and version come from the go.mod entry; rs provides the archived/pushed dates from GitHub.
func formatArchivedLine(modPath, version string, rs RepoStatus) string {
	var b strings.Builder
	b.WriteString(modPath)
	if version != "" {
		b.WriteString("@")
		b.WriteString(version)
	}
	b.WriteString(" [ARCHIVED")
	if !rs.ArchivedAt.IsZero() {
		b.WriteString(" ")
		b.WriteString(fmtDate(rs.ArchivedAt))
	}
	if !rs.PushedAt.IsZero() {
		b.WriteString(", last pushed ")
		b.WriteString(fmtDate(rs.PushedAt))
	}
	b.WriteString("]")
	return b.String()
}

// treeEntry represents a direct dependency and its archived transitive deps.
type treeEntry struct {
	directPath string
	archived   []string // deduplicated module paths
}

// treeContext holds precomputed lookups needed to render tree entries.
type treeContext struct {
	archivedPaths map[string]bool
	versionByPath map[string]string
	getStatus     func(string) (RepoStatus, bool)
}

// buildTree computes the tree entries and lookup context from results, graph,
// and allModules. Returns nil entries if there are no archived dependencies.
func buildTree(results []RepoStatus, graph map[string][]string, allModules []Module) ([]treeEntry, *treeContext) {
	// Build lookup from owner/repo → RepoStatus (for archived/pushed dates)
	statusByRepo := make(map[string]RepoStatus)
	archivedPaths := make(map[string]bool)
	for _, r := range results {
		if r.IsArchived {
			statusByRepo[r.Module.Owner+"/"+r.Module.Repo] = r
			archivedPaths[r.Module.Path] = true
		}
	}
	// Also map owner/repo → module paths for multi-path repos
	repoToModules := make(map[string][]string)
	for _, m := range allModules {
		if m.Owner != "" {
			key := m.Owner + "/" + m.Repo
			repoToModules[key] = append(repoToModules[key], m.Path)
		}
	}
	for _, r := range results {
		if r.IsArchived {
			for _, p := range repoToModules[r.Module.Owner+"/"+r.Module.Repo] {
				archivedPaths[p] = true
			}
		}
	}

	// Build lookup from module path → version and owner/repo (from go.mod)
	versionByPath := make(map[string]string)
	repoByPath := make(map[string]string) // module path → "owner/repo"
	for _, m := range allModules {
		versionByPath[m.Path] = m.Version
		if m.Owner != "" {
			repoByPath[m.Path] = m.Owner + "/" + m.Repo
		}
	}

	// Helper to get RepoStatus for a module path (via its owner/repo)
	getStatus := func(modPath string) (RepoStatus, bool) {
		repo := repoByPath[modPath]
		if repo == "" {
			owner, repoName := extractGitHub(modPath)
			if owner != "" {
				repo = owner + "/" + repoName
			}
		}
		rs, ok := statusByRepo[repo]
		return rs, ok
	}

	ctx := &treeContext{
		archivedPaths: archivedPaths,
		versionByPath: versionByPath,
		getStatus:     getStatus,
	}

	if len(archivedPaths) == 0 {
		return nil, ctx
	}

	// Find root module: the only graph key without an "@" (no version suffix)
	var rootKey string
	for key := range graph {
		if !strings.Contains(key, "@") {
			rootKey = key
			break
		}
	}
	if rootKey == "" {
		// Fallback: pick the key with the most children
		maxChildren := 0
		for key, children := range graph {
			if len(children) > maxChildren {
				maxChildren = len(children)
				rootKey = key
			}
		}
	}

	if rootKey == "" {
		// No graph data — return one entry per archived result
		var entries []treeEntry
		for _, r := range results {
			if r.IsArchived {
				entries = append(entries, treeEntry{directPath: r.Module.Path})
			}
		}
		return entries, ctx
	}

	// For each direct dependency (child of root), find archived transitive deps
	var entries []treeEntry
	for _, child := range graph[rootKey] {
		childMod := stripVersion(child)
		selfArchived := archivedPaths[childMod]
		archivedTransitive := findArchivedTransitive(child, graph, archivedPaths, make(map[string]bool))

		if selfArchived || len(archivedTransitive) > 0 {
			entry := treeEntry{directPath: childMod}
			for _, a := range archivedTransitive {
				if a != childMod {
					entry.archived = append(entry.archived, a)
				}
			}
			entries = append(entries, entry)
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].directPath < entries[j].directPath
	})

	return entries, ctx
}

// PrintTree outputs a dependency tree showing which direct dependencies
// pull in archived indirect dependencies. If fileMatches is non-nil,
// file counts are appended to archived labels.
func PrintTree(results []RepoStatus, graph map[string][]string, allModules []Module, fileMatches map[string][]FileMatch) {
	entries, ctx := buildTree(results, graph, allModules)

	if entries == nil {
		fmt.Fprintf(os.Stderr, "\nNo archived dependencies found.\n")
		return
	}

	fmt.Fprintf(os.Stderr, "\nDEPENDENCY TREE (archived dependencies marked with [ARCHIVED])\n\n")

	// fileCountSuffix returns " (N files)" if fileMatches has entries for modPath.
	fileCountSuffix := func(modPath string) string {
		if fileMatches == nil {
			return ""
		}
		matches := fileMatches[modPath]
		uniqueFiles := make(map[string]bool)
		for _, m := range matches {
			uniqueFiles[m.File] = true
		}
		n := len(uniqueFiles)
		return fmt.Sprintf(" (%d %s)", n, pluralize(n, "file", "files"))
	}

	for _, e := range entries {
		if ctx.archivedPaths[e.directPath] {
			if rs, ok := ctx.getStatus(e.directPath); ok {
				fmt.Printf("%s%s\n", formatArchivedLine(e.directPath, ctx.versionByPath[e.directPath], rs), fileCountSuffix(e.directPath))
			} else {
				fmt.Printf("%s [ARCHIVED]%s\n", e.directPath, fileCountSuffix(e.directPath))
			}
		} else {
			ver := ctx.versionByPath[e.directPath]
			if ver != "" {
				fmt.Printf("%s@%s\n", e.directPath, ver)
			} else {
				fmt.Printf("%s\n", e.directPath)
			}
		}
		seen := make(map[string]bool)
		for i, a := range e.archived {
			if seen[a] {
				continue
			}
			seen[a] = true
			connector := "├── "
			if i == len(e.archived)-1 || allSeen(e.archived[i+1:], seen) {
				connector = "└── "
			}
			if rs, ok := ctx.getStatus(a); ok {
				fmt.Printf("  %s%s%s\n", connector, formatArchivedLine(a, ctx.versionByPath[a], rs), fileCountSuffix(a))
			} else {
				fmt.Printf("  %s%s [ARCHIVED]%s\n", connector, a, fileCountSuffix(a))
			}
		}
	}
}

// JSONTreeOutput is the structure for --tree --json output mode.
type JSONTreeOutput struct {
	Tree         []JSONTreeEntry `json:"tree"`
	SkippedNonGH int             `json:"skipped_non_github"`
	TotalChecked int             `json:"total_checked"`
}

// JSONTreeEntry represents a direct dependency in the JSON tree.
type JSONTreeEntry struct {
	Module                 string              `json:"module"`
	Version                string              `json:"version"`
	Archived               bool                `json:"archived"`
	ArchivedAt             string              `json:"archived_at,omitempty"`
	PushedAt               string              `json:"pushed_at,omitempty"`
	SourceFiles            []JSONSourceFile    `json:"source_files,omitempty"`
	ArchivedDependencies   []JSONTreeArchivedDep `json:"archived_dependencies"`
}

// JSONTreeArchivedDep represents an archived transitive dependency.
type JSONTreeArchivedDep struct {
	Module      string           `json:"module"`
	Version     string           `json:"version"`
	ArchivedAt  string           `json:"archived_at,omitempty"`
	PushedAt    string           `json:"pushed_at,omitempty"`
	SourceFiles []JSONSourceFile `json:"source_files,omitempty"`
}

// buildTreeJSONOutput creates the JSONTreeOutput data structure without writing it.
func buildTreeJSONOutput(results []RepoStatus, graph map[string][]string, allModules []Module, fileMatches map[string][]FileMatch, nonGitHubCount int) JSONTreeOutput {
	entries, ctx := buildTree(results, graph, allModules)

	out := JSONTreeOutput{
		Tree:         []JSONTreeEntry{},
		SkippedNonGH: nonGitHubCount,
		TotalChecked: len(results),
	}

	if entries == nil {
		return out
	}

	buildSourceFiles := func(modPath string) []JSONSourceFile {
		if fileMatches == nil {
			return nil
		}
		var sf []JSONSourceFile
		for _, fm := range fileMatches[modPath] {
			sf = append(sf, JSONSourceFile{
				File:   fm.File,
				Line:   fm.Line,
				Import: fm.ImportPath,
			})
		}
		return sf
	}

	for _, e := range entries {
		entry := JSONTreeEntry{
			Module:               e.directPath,
			Version:              ctx.versionByPath[e.directPath],
			Archived:             ctx.archivedPaths[e.directPath],
			ArchivedDependencies: []JSONTreeArchivedDep{},
		}

		if entry.Archived {
			if rs, ok := ctx.getStatus(e.directPath); ok {
				if !rs.ArchivedAt.IsZero() {
					entry.ArchivedAt = rs.ArchivedAt.Format("2006-01-02T15:04:05Z")
				}
				if !rs.PushedAt.IsZero() {
					entry.PushedAt = rs.PushedAt.Format("2006-01-02T15:04:05Z")
				}
			}
			entry.SourceFiles = buildSourceFiles(e.directPath)
		}

		seen := make(map[string]bool)
		for _, a := range e.archived {
			if seen[a] {
				continue
			}
			seen[a] = true

			dep := JSONTreeArchivedDep{
				Module:  a,
				Version: ctx.versionByPath[a],
			}
			if rs, ok := ctx.getStatus(a); ok {
				if !rs.ArchivedAt.IsZero() {
					dep.ArchivedAt = rs.ArchivedAt.Format("2006-01-02T15:04:05Z")
				}
				if !rs.PushedAt.IsZero() {
					dep.PushedAt = rs.PushedAt.Format("2006-01-02T15:04:05Z")
				}
			}
			dep.SourceFiles = buildSourceFiles(a)
			entry.ArchivedDependencies = append(entry.ArchivedDependencies, dep)
		}

		out.Tree = append(out.Tree, entry)
	}

	return out
}

// PrintTreeJSON outputs the dependency tree as JSON.
func PrintTreeJSON(results []RepoStatus, graph map[string][]string, allModules []Module, fileMatches map[string][]FileMatch, nonGitHubCount int) {
	out := buildTreeJSONOutput(results, graph, allModules, fileMatches, nonGitHubCount)
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(out)
}

// RecursiveJSONOutput wraps per-module results for --recursive --json.
type RecursiveJSONOutput struct {
	Modules []RecursiveJSONEntry `json:"modules"`
}

// RecursiveJSONEntry holds results for a single go.mod in recursive mode.
type RecursiveJSONEntry struct {
	GoMod      string `json:"go_mod"`
	ModulePath string `json:"module_path"`
	JSONOutput
}

// RecursiveJSONTreeOutput wraps per-module tree results for --recursive --tree --json.
type RecursiveJSONTreeOutput struct {
	Modules []RecursiveJSONTreeEntry `json:"modules"`
}

// RecursiveJSONTreeEntry holds tree results for a single go.mod in recursive mode.
type RecursiveJSONTreeEntry struct {
	GoMod      string `json:"go_mod"`
	ModulePath string `json:"module_path"`
	JSONTreeOutput
}

// allSeen returns true if all items in slice are already in the seen set.
func allSeen(items []string, seen map[string]bool) bool {
	for _, item := range items {
		if !seen[item] {
			return false
		}
	}
	return true
}

func stripVersion(s string) string {
	// go mod graph entries look like "github.com/foo/bar@v1.2.3"
	if idx := strings.LastIndex(s, "@"); idx > 0 {
		return s[:idx]
	}
	return s
}

func findArchivedTransitive(node string, graph map[string][]string, archivedPaths map[string]bool, visited map[string]bool) []string {
	if visited[node] {
		return nil
	}
	visited[node] = true

	var result []string
	for _, child := range graph[node] {
		childMod := stripVersion(child)
		if archivedPaths[childMod] {
			result = append(result, childMod)
		}
		result = append(result, findArchivedTransitive(child, graph, archivedPaths, visited)...)
	}
	return result
}
