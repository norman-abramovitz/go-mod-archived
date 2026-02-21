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

// durationEnabled and durationEndDate control the --duration feature.
var (
	durationEnabled bool
	durationEndDate time.Time
)

// fmtDate formats a time using the current dateFmt setting.
func fmtDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(dateFmt)
}

// calcDuration computes the calendar duration (years, months, days) between
// two dates. Both dates are normalized to midnight UTC. The result is
// inclusive: same-day yields (0, 0, 1) because we add 1 day per the spec.
func calcDuration(archivedAt, endDate time.Time) (years, months, days int) {
	from := time.Date(archivedAt.Year(), archivedAt.Month(), archivedAt.Day(), 0, 0, 0, 0, time.UTC)
	to := time.Date(endDate.Year(), endDate.Month(), endDate.Day(), 0, 0, 0, 0, time.UTC)
	// +1 day: "archived date to end date" is inclusive
	to = to.AddDate(0, 0, 1)

	years = to.Year() - from.Year()
	months = int(to.Month()) - int(from.Month())
	days = to.Day() - from.Day()

	if days < 0 {
		months--
		// Days in the previous month relative to 'to'
		days += time.Date(to.Year(), to.Month(), 0, 0, 0, 0, 0, time.UTC).Day()
	}
	if months < 0 {
		years--
		months += 12
	}
	return years, months, days
}

// formatDuration returns a human-readable duration string for how long a
// dependency has been archived. Returns "" if duration mode is off or the
// archived date is zero.
func formatDuration(archivedAt time.Time) string {
	if !durationEnabled || archivedAt.IsZero() {
		return ""
	}
	y, m, d := calcDuration(archivedAt, durationEndDate)
	var parts []string
	if y > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", y, pluralize(y, "year", "years")))
	}
	if m > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", m, pluralize(m, "month", "months")))
	}
	if d > 0 || len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%d %s", d, pluralize(d, "day", "days")))
	}
	return strings.Join(parts, ", ")
}

// formatDurationShort returns a compact duration string (e.g. "2y 3m 15d")
// for use in tree output. Returns "" if duration mode is off or the
// archived date is zero.
func formatDurationShort(archivedAt time.Time) string {
	if !durationEnabled || archivedAt.IsZero() {
		return ""
	}
	y, m, d := calcDuration(archivedAt, durationEndDate)
	var parts []string
	if y > 0 {
		parts = append(parts, fmt.Sprintf("%dy", y))
	}
	if m > 0 {
		parts = append(parts, fmt.Sprintf("%dm", m))
	}
	if d > 0 || len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%dd", d))
	}
	return strings.Join(parts, " ")
}

// hostDomain extracts the hosting domain from a module path.
func hostDomain(modulePath string) string {
	parts := strings.SplitN(modulePath, "/", 2)
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

// PrintSkippedTable outputs a section listing non-GitHub modules with enrichment data.
func PrintSkippedTable(modules []Module) {
	sort.Slice(modules, func(i, j int) bool {
		return modules[i].Path < modules[j].Path
	})
	fmt.Fprintf(os.Stderr, "\nNON-GITHUB MODULES (%d non-GitHub %s)\n\n", len(modules), pluralize(len(modules), "module", "modules"))
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "MODULE\tVERSION\tLATEST\tDIRECT\tPUBLISHED\tSOURCE")
	for _, m := range modules {
		direct := "indirect"
		if m.Direct {
			direct = "direct"
		}
		latest := m.LatestVersion
		if latest != "" && latest == m.Version {
			latest = "-"
		}
		published := fmtDate(m.VersionTime)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", m.Path, m.Version, latest, direct, published, m.SourceURL)
	}
	w.Flush()
}

// PrintTable outputs archived (or all) results in a human-readable table.
// If deprecatedModules is non-nil, a DEPRECATED MODULES section is appended.
func PrintTable(results []RepoStatus, nonGitHubModules []Module, showAll bool, deprecatedModules ...[]Module) {
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
		if durationEnabled {
			fmt.Fprintln(w, "MODULE\tVERSION\tDIRECT\tARCHIVED AT\tDURATION\tLAST PUSHED")
		} else {
			fmt.Fprintln(w, "MODULE\tVERSION\tDIRECT\tARCHIVED AT\tLAST PUSHED")
		}
		for _, r := range archived {
			direct := "indirect"
			if r.Module.Direct {
				direct = "direct"
			}
			archivedAt := fmtDate(r.ArchivedAt)
			pushedAt := fmtDate(r.PushedAt)
			if durationEnabled {
				dur := formatDuration(r.ArchivedAt)
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", r.Module.Path, r.Module.Version, direct, archivedAt, dur, pushedAt)
			} else {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", r.Module.Path, r.Module.Version, direct, archivedAt, pushedAt)
			}
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

	// Deprecated modules section
	if len(deprecatedModules) > 0 && len(deprecatedModules[0]) > 0 {
		deps := deprecatedModules[0]
		sort.Slice(deps, func(i, j int) bool {
			return deps[i].Path < deps[j].Path
		})
		fmt.Fprintf(os.Stderr, "\nDEPRECATED MODULES (%d %s)\n\n", len(deps), pluralize(len(deps), "module", "modules"))
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "MODULE\tVERSION\tDIRECT\tMESSAGE")
		for _, m := range deps {
			direct := "indirect"
			if m.Direct {
				direct = "direct"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", m.Path, m.Version, direct, m.Deprecated)
		}
		w.Flush()
	}

	if len(nonGitHubModules) > 0 {
		PrintSkippedTable(nonGitHubModules)
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

// PrintDeprecatedTable outputs a standalone deprecated modules table.
// Used when --tree mode needs to append a deprecated section separately.
func PrintDeprecatedTable(modules []Module) {
	sort.Slice(modules, func(i, j int) bool {
		return modules[i].Path < modules[j].Path
	})
	fmt.Fprintf(os.Stderr, "\nDEPRECATED MODULES (%d %s)\n\n", len(modules), pluralize(len(modules), "module", "modules"))
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "MODULE\tVERSION\tDIRECT\tMESSAGE")
	for _, m := range modules {
		direct := "indirect"
		if m.Direct {
			direct = "direct"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", m.Path, m.Version, direct, m.Deprecated)
	}
	w.Flush()
}

// pluralize returns singular or plural form based on count.
func pluralize(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}

// JSONSkippedModule represents a non-GitHub module in JSON output.
type JSONSkippedModule struct {
	Module        string `json:"module"`
	Version       string `json:"version"`
	Direct        bool   `json:"direct"`
	LatestVersion string `json:"latest_version,omitempty"`
	Published     string `json:"published,omitempty"`
	Host          string `json:"host,omitempty"`
	SourceURL     string `json:"source_url,omitempty"`
}

// JSONOutput is the structure for JSON output mode.
type JSONOutput struct {
	Archived        []JSONModule        `json:"archived"`
	Deprecated      []JSONModule        `json:"deprecated,omitempty"`
	NotFound        []JSONModule        `json:"not_found,omitempty"`
	Active          []JSONModule        `json:"active,omitempty"`
	NonGitHubCount  int                 `json:"non_github_count"`
	NonGitHubModules []JSONSkippedModule `json:"non_github_modules,omitempty"`
	TotalChecked    int                 `json:"total_checked"`
}

type JSONModule struct {
	Module            string           `json:"module"`
	Version           string           `json:"version"`
	Direct            bool             `json:"direct"`
	Owner             string           `json:"owner"`
	Repo              string           `json:"repo"`
	ArchivedAt        string           `json:"archived_at,omitempty"`
	ArchivedDuration  string           `json:"archived_duration,omitempty"`
	PushedAt          string           `json:"pushed_at,omitempty"`
	Error             string           `json:"error,omitempty"`
	DeprecatedMessage string           `json:"deprecated_message,omitempty"`
	SourceFiles       []JSONSourceFile `json:"source_files,omitempty"`
}

// JSONSourceFile represents a source file match in JSON output.
type JSONSourceFile struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Import string `json:"import"`
}

// buildJSONOutput creates the JSONOutput data structure without writing it.
// deprecatedModules is optional; if provided, the first element is used.
func buildJSONOutput(results []RepoStatus, nonGitHubModules []Module, showAll bool, fileMatches map[string][]FileMatch, deprecatedModules ...[]Module) JSONOutput {
	out := JSONOutput{
		NonGitHubCount: len(nonGitHubModules),
		TotalChecked:   len(results),
		Archived:       []JSONModule{},
	}

	for _, m := range nonGitHubModules {
		jsm := JSONSkippedModule{
			Module:  m.Path,
			Version: m.Version,
			Direct:  m.Direct,
			Host:    hostDomain(m.Path),
		}
		if m.LatestVersion != "" {
			jsm.LatestVersion = m.LatestVersion
		}
		if !m.VersionTime.IsZero() {
			jsm.Published = m.VersionTime.Format("2006-01-02T15:04:05Z")
		}
		if m.SourceURL != "" {
			jsm.SourceURL = m.SourceURL
		}
		out.NonGitHubModules = append(out.NonGitHubModules, jsm)
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
			if dur := formatDuration(r.ArchivedAt); dur != "" {
				jm.ArchivedDuration = dur
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

	// Add deprecated modules if provided.
	if len(deprecatedModules) > 0 && len(deprecatedModules[0]) > 0 {
		for _, m := range deprecatedModules[0] {
			out.Deprecated = append(out.Deprecated, JSONModule{
				Module:            m.Path,
				Version:           m.Version,
				Direct:            m.Direct,
				Owner:             m.Owner,
				Repo:              m.Repo,
				DeprecatedMessage: m.Deprecated,
			})
		}
	}

	return out
}

// PrintJSON outputs results as JSON. If fileMatches is non-nil, archived
// modules will include source_files arrays.
// deprecatedModules is optional; if provided, the first element is used.
func PrintJSON(results []RepoStatus, nonGitHubModules []Module, showAll bool, fileMatches map[string][]FileMatch, deprecatedModules ...[]Module) {
	out := buildJSONOutput(results, nonGitHubModules, showAll, fileMatches, deprecatedModules...)
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
	if dur := formatDurationShort(rs.ArchivedAt); dur != "" {
		b.WriteString(", ")
		b.WriteString(dur)
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
	archivedPaths    map[string]bool
	deprecatedByPath map[string]string // module path → deprecation message
	versionByPath    map[string]string
	getStatus        func(string) (RepoStatus, bool)
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

	// Build lookup from module path → version, owner/repo, and deprecation (from go.mod)
	versionByPath := make(map[string]string)
	repoByPath := make(map[string]string)    // module path → "owner/repo"
	deprecatedByPath := make(map[string]string) // module path → deprecation message
	for _, m := range allModules {
		versionByPath[m.Path] = m.Version
		if m.Owner != "" {
			repoByPath[m.Path] = m.Owner + "/" + m.Repo
		}
		if m.Deprecated != "" {
			deprecatedByPath[m.Path] = m.Deprecated
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
		archivedPaths:    archivedPaths,
		deprecatedByPath: deprecatedByPath,
		versionByPath:    versionByPath,
		getStatus:        getStatus,
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

	// deprecatedSuffix returns " [DEPRECATED]" if the module is deprecated.
	deprecatedSuffix := func(modPath string) string {
		if ctx.deprecatedByPath[modPath] != "" {
			return " [DEPRECATED]"
		}
		return ""
	}

	for _, e := range entries {
		if ctx.archivedPaths[e.directPath] {
			if rs, ok := ctx.getStatus(e.directPath); ok {
				fmt.Printf("%s%s%s\n", formatArchivedLine(e.directPath, ctx.versionByPath[e.directPath], rs), deprecatedSuffix(e.directPath), fileCountSuffix(e.directPath))
			} else {
				fmt.Printf("%s [ARCHIVED]%s%s\n", e.directPath, deprecatedSuffix(e.directPath), fileCountSuffix(e.directPath))
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
				fmt.Printf("  %s%s%s%s\n", connector, formatArchivedLine(a, ctx.versionByPath[a], rs), deprecatedSuffix(a), fileCountSuffix(a))
			} else {
				fmt.Printf("  %s%s [ARCHIVED]%s%s\n", connector, a, deprecatedSuffix(a), fileCountSuffix(a))
			}
		}
	}
}

// JSONTreeOutput is the structure for --tree --json output mode.
type JSONTreeOutput struct {
	Tree             []JSONTreeEntry     `json:"tree"`
	Deprecated       []JSONModule        `json:"deprecated,omitempty"`
	NonGitHubCount   int                 `json:"non_github_count"`
	NonGitHubModules []JSONSkippedModule `json:"non_github_modules,omitempty"`
	TotalChecked     int                 `json:"total_checked"`
}

// JSONTreeEntry represents a direct dependency in the JSON tree.
type JSONTreeEntry struct {
	Module                 string                `json:"module"`
	Version                string                `json:"version"`
	Archived               bool                  `json:"archived"`
	ArchivedAt             string                `json:"archived_at,omitempty"`
	ArchivedDuration       string                `json:"archived_duration,omitempty"`
	PushedAt               string                `json:"pushed_at,omitempty"`
	DeprecatedMessage      string                `json:"deprecated_message,omitempty"`
	SourceFiles            []JSONSourceFile      `json:"source_files,omitempty"`
	ArchivedDependencies   []JSONTreeArchivedDep `json:"archived_dependencies"`
}

// JSONTreeArchivedDep represents an archived transitive dependency.
type JSONTreeArchivedDep struct {
	Module            string           `json:"module"`
	Version           string           `json:"version"`
	ArchivedAt        string           `json:"archived_at,omitempty"`
	ArchivedDuration  string           `json:"archived_duration,omitempty"`
	PushedAt          string           `json:"pushed_at,omitempty"`
	DeprecatedMessage string           `json:"deprecated_message,omitempty"`
	SourceFiles       []JSONSourceFile `json:"source_files,omitempty"`
}

// buildTreeJSONOutput creates the JSONTreeOutput data structure without writing it.
// deprecatedModules is optional; if provided, the first element is used.
func buildTreeJSONOutput(results []RepoStatus, graph map[string][]string, allModules []Module, fileMatches map[string][]FileMatch, nonGitHubModules []Module, deprecatedModules ...[]Module) JSONTreeOutput {
	entries, ctx := buildTree(results, graph, allModules)

	out := JSONTreeOutput{
		Tree:           []JSONTreeEntry{},
		NonGitHubCount: len(nonGitHubModules),
		TotalChecked:   len(results),
	}

	for _, m := range nonGitHubModules {
		jsm := JSONSkippedModule{
			Module:  m.Path,
			Version: m.Version,
			Direct:  m.Direct,
			Host:    hostDomain(m.Path),
		}
		if m.LatestVersion != "" {
			jsm.LatestVersion = m.LatestVersion
		}
		if !m.VersionTime.IsZero() {
			jsm.Published = m.VersionTime.Format("2006-01-02T15:04:05Z")
		}
		if m.SourceURL != "" {
			jsm.SourceURL = m.SourceURL
		}
		out.NonGitHubModules = append(out.NonGitHubModules, jsm)
	}

	// Add deprecated modules if provided.
	if len(deprecatedModules) > 0 && len(deprecatedModules[0]) > 0 {
		for _, m := range deprecatedModules[0] {
			out.Deprecated = append(out.Deprecated, JSONModule{
				Module:            m.Path,
				Version:           m.Version,
				Direct:            m.Direct,
				Owner:             m.Owner,
				Repo:              m.Repo,
				DeprecatedMessage: m.Deprecated,
			})
		}
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
			DeprecatedMessage:    ctx.deprecatedByPath[e.directPath],
			ArchivedDependencies: []JSONTreeArchivedDep{},
		}

		if entry.Archived {
			if rs, ok := ctx.getStatus(e.directPath); ok {
				if !rs.ArchivedAt.IsZero() {
					entry.ArchivedAt = rs.ArchivedAt.Format("2006-01-02T15:04:05Z")
				}
				if dur := formatDuration(rs.ArchivedAt); dur != "" {
					entry.ArchivedDuration = dur
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
				Module:            a,
				Version:           ctx.versionByPath[a],
				DeprecatedMessage: ctx.deprecatedByPath[a],
			}
			if rs, ok := ctx.getStatus(a); ok {
				if !rs.ArchivedAt.IsZero() {
					dep.ArchivedAt = rs.ArchivedAt.Format("2006-01-02T15:04:05Z")
				}
				if dur := formatDuration(rs.ArchivedAt); dur != "" {
					dep.ArchivedDuration = dur
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
// deprecatedModules is optional; if provided, the first element is used.
func PrintTreeJSON(results []RepoStatus, graph map[string][]string, allModules []Module, fileMatches map[string][]FileMatch, nonGitHubModules []Module, deprecatedModules ...[]Module) {
	out := buildTreeJSONOutput(results, graph, allModules, fileMatches, nonGitHubModules, deprecatedModules...)
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
	GoVersion  string `json:"go_version,omitempty"`
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
	GoVersion  string `json:"go_version,omitempty"`
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
