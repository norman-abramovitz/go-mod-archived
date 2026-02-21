package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func TestStripVersion(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"github.com/foo/bar@v1.2.3", "github.com/foo/bar"},
		{"github.com/foo/bar/v2@v2.0.0", "github.com/foo/bar/v2"},
		{"github.com/foo/bar@v0.0.0-20210821155943-2d9075ca8770", "github.com/foo/bar"},
		{"github.com/foo/bar", "github.com/foo/bar"},         // no version
		{"cel.dev/expr@v0.25.1", "cel.dev/expr"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := stripVersion(tt.input)
			if got != tt.want {
				t.Errorf("stripVersion(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestAllSeen(t *testing.T) {
	seen := map[string]bool{"a": true, "b": true}

	if !allSeen([]string{"a", "b"}, seen) {
		t.Error("expected true when all items seen")
	}
	if !allSeen([]string{}, seen) {
		t.Error("expected true for empty slice")
	}
	if allSeen([]string{"a", "c"}, seen) {
		t.Error("expected false when 'c' not seen")
	}
}

func TestFindArchivedTransitive(t *testing.T) {
	graph := map[string][]string{
		"root":                         {"github.com/a/b@v1.0.0", "github.com/c/d@v1.0.0"},
		"github.com/a/b@v1.0.0":       {"github.com/x/y@v1.0.0"},
		"github.com/c/d@v1.0.0":       {"github.com/x/y@v1.0.0", "github.com/e/f@v1.0.0"},
		"github.com/x/y@v1.0.0":       {},
		"github.com/e/f@v1.0.0":       {},
	}

	archivedPaths := map[string]bool{
		"github.com/x/y": true,
	}

	result := findArchivedTransitive("github.com/a/b@v1.0.0", graph, archivedPaths, make(map[string]bool))
	if len(result) != 1 || result[0] != "github.com/x/y" {
		t.Errorf("expected [github.com/x/y], got %v", result)
	}
}

func TestFindArchivedTransitive_Cycle(t *testing.T) {
	// Ensure cycles don't cause infinite loops
	graph := map[string][]string{
		"a@v1": {"b@v1"},
		"b@v1": {"a@v1"},
	}

	archivedPaths := map[string]bool{"b": true}

	result := findArchivedTransitive("a@v1", graph, archivedPaths, make(map[string]bool))
	if len(result) != 1 || result[0] != "b" {
		t.Errorf("expected [b], got %v", result)
	}
}

func TestFindArchivedTransitive_Deep(t *testing.T) {
	graph := map[string][]string{
		"a@v1": {"b@v1"},
		"b@v1": {"c@v1"},
		"c@v1": {"d@v1"},
		"d@v1": {},
	}

	archivedPaths := map[string]bool{"d": true}

	result := findArchivedTransitive("a@v1", graph, archivedPaths, make(map[string]bool))
	if len(result) != 1 || result[0] != "d" {
		t.Errorf("expected [d], got %v", result)
	}
}

func TestFindArchivedTransitive_NoArchived(t *testing.T) {
	graph := map[string][]string{
		"a@v1": {"b@v1"},
		"b@v1": {},
	}

	archivedPaths := map[string]bool{}

	result := findArchivedTransitive("a@v1", graph, archivedPaths, make(map[string]bool))
	if len(result) != 0 {
		t.Errorf("expected empty, got %v", result)
	}
}

func TestFmtDate(t *testing.T) {
	ts := time.Date(2024, 7, 22, 14, 30, 45, 0, time.UTC)

	// Default date-only format
	dateFmt = "2006-01-02"
	if got := fmtDate(ts); got != "2024-07-22" {
		t.Errorf("date-only: got %q, want %q", got, "2024-07-22")
	}

	// With time
	dateFmt = "2006-01-02 15:04:05"
	if got := fmtDate(ts); got != "2024-07-22 14:30:45" {
		t.Errorf("with time: got %q, want %q", got, "2024-07-22 14:30:45")
	}

	// Zero time
	if got := fmtDate(time.Time{}); got != "" {
		t.Errorf("zero time: got %q, want empty", got)
	}

	// Reset
	dateFmt = "2006-01-02"
}

func TestFormatArchivedLine_WithTime(t *testing.T) {
	dateFmt = "2006-01-02 15:04:05"
	defer func() { dateFmt = "2006-01-02" }()

	rs := RepoStatus{
		ArchivedAt: time.Date(2024, 7, 22, 14, 30, 45, 0, time.UTC),
		PushedAt:   time.Date(2021, 5, 5, 9, 15, 0, 0, time.UTC),
	}

	got := formatArchivedLine("github.com/foo/bar", "v1.0.0", rs)
	if !strings.Contains(got, "2024-07-22 14:30:45") {
		t.Errorf("expected time in archived date, got %q", got)
	}
	if !strings.Contains(got, "2021-05-05 09:15:00") {
		t.Errorf("expected time in pushed date, got %q", got)
	}
}

func TestFormatArchivedLine(t *testing.T) {
	rs := RepoStatus{
		ArchivedAt: time.Date(2024, 7, 22, 0, 0, 0, 0, time.UTC),
		PushedAt:   time.Date(2021, 5, 5, 0, 0, 0, 0, time.UTC),
	}

	got := formatArchivedLine("github.com/foo/bar", "v1.2.3", rs)
	want := "github.com/foo/bar@v1.2.3 [ARCHIVED 2024-07-22, last pushed 2021-05-05]"
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

func TestFormatArchivedLine_NoVersion(t *testing.T) {
	rs := RepoStatus{
		ArchivedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	got := formatArchivedLine("github.com/foo/bar", "", rs)
	if !strings.Contains(got, "github.com/foo/bar [ARCHIVED") {
		t.Errorf("expected no @ when version empty, got %q", got)
	}
	if strings.Contains(got, "last pushed") {
		t.Errorf("should not show last pushed when zero, got %q", got)
	}
}

func TestFormatArchivedLine_NoDates(t *testing.T) {
	rs := RepoStatus{}

	got := formatArchivedLine("github.com/foo/bar", "v1.0.0", rs)
	want := "github.com/foo/bar@v1.0.0 [ARCHIVED]"
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

// captureStdout captures stdout output during fn execution.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func TestPrintJSON_ArchivedOnly(t *testing.T) {
	results := []RepoStatus{
		{
			Module:     Module{Path: "github.com/foo/bar", Version: "v1.0.0", Direct: true, Owner: "foo", Repo: "bar"},
			IsArchived: true,
			ArchivedAt: time.Date(2024, 7, 22, 0, 0, 0, 0, time.UTC),
			PushedAt:   time.Date(2021, 5, 5, 0, 0, 0, 0, time.UTC),
		},
		{
			Module:     Module{Path: "github.com/baz/qux", Version: "v2.0.0", Direct: false, Owner: "baz", Repo: "qux"},
			IsArchived: false,
			PushedAt:   time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	skippedModules := []Module{
		{Path: "golang.org/x/text", Version: "v0.14.0", Direct: true},
		{Path: "google.golang.org/protobuf", Version: "v1.33.0", Direct: false},
		{Path: "gopkg.in/yaml.v3", Version: "v3.0.1", Direct: true},
		{Path: "cel.dev/expr", Version: "v0.25.1", Direct: false},
		{Path: "golang.org/x/net", Version: "v0.24.0", Direct: true},
	}

	output := captureStdout(t, func() {
		PrintJSON(results, skippedModules, false, nil)
	})

	var out JSONOutput
	if err := json.Unmarshal([]byte(output), &out); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, output)
	}

	if len(out.Archived) != 1 {
		t.Errorf("expected 1 archived, got %d", len(out.Archived))
	}
	if out.Archived[0].Module != "github.com/foo/bar" {
		t.Errorf("archived module = %q", out.Archived[0].Module)
	}
	if out.Active != nil {
		t.Error("expected no active modules when showAll=false")
	}
	if out.SkippedNonGH != 5 {
		t.Errorf("skipped = %d, want 5", out.SkippedNonGH)
	}
	if len(out.SkippedModules) != 5 {
		t.Errorf("skipped_modules length = %d, want 5", len(out.SkippedModules))
	}
	if out.SkippedModules[0].Module != "golang.org/x/text" {
		t.Errorf("skipped_modules[0].module = %q, want %q", out.SkippedModules[0].Module, "golang.org/x/text")
	}
	if out.SkippedModules[0].Version != "v0.14.0" {
		t.Errorf("skipped_modules[0].version = %q, want %q", out.SkippedModules[0].Version, "v0.14.0")
	}
	if !out.SkippedModules[0].Direct {
		t.Error("skipped_modules[0].direct should be true")
	}
	if out.TotalChecked != 2 {
		t.Errorf("total = %d, want 2", out.TotalChecked)
	}
}

func TestPrintJSON_ShowAll(t *testing.T) {
	results := []RepoStatus{
		{
			Module:     Module{Path: "github.com/foo/bar", Version: "v1.0.0", Direct: true, Owner: "foo", Repo: "bar"},
			IsArchived: false,
			PushedAt:   time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	output := captureStdout(t, func() {
		PrintJSON(results, nil, true, nil)
	})

	var out JSONOutput
	if err := json.Unmarshal([]byte(output), &out); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if len(out.Active) != 1 {
		t.Errorf("expected 1 active module with showAll=true, got %d", len(out.Active))
	}
}

func TestPrintJSON_NotFound(t *testing.T) {
	results := []RepoStatus{
		{
			Module:   Module{Path: "github.com/gone/repo", Owner: "gone", Repo: "repo"},
			NotFound: true,
			Error:    "Could not resolve",
		},
	}

	output := captureStdout(t, func() {
		PrintJSON(results, nil, false, nil)
	})

	var out JSONOutput
	if err := json.Unmarshal([]byte(output), &out); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if len(out.NotFound) != 1 {
		t.Errorf("expected 1 not_found, got %d", len(out.NotFound))
	}
	if out.NotFound[0].Error != "Could not resolve" {
		t.Errorf("error = %q", out.NotFound[0].Error)
	}
}

func TestPrintJSON_EmptyArchived(t *testing.T) {
	output := captureStdout(t, func() {
		PrintJSON(nil, nil, false, nil)
	})

	var out JSONOutput
	if err := json.Unmarshal([]byte(output), &out); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// Archived should be empty array, not null
	if !strings.Contains(output, `"archived": []`) {
		t.Error("expected archived to be empty array, not null")
	}
}

func TestPrintTable_ContainsArchivedModule(t *testing.T) {
	results := []RepoStatus{
		{
			Module:     Module{Path: "github.com/foo/bar", Version: "v1.0.0", Direct: true, Owner: "foo", Repo: "bar"},
			IsArchived: true,
			ArchivedAt: time.Date(2024, 7, 22, 0, 0, 0, 0, time.UTC),
			PushedAt:   time.Date(2021, 5, 5, 0, 0, 0, 0, time.UTC),
		},
	}

	output := captureStdout(t, func() {
		PrintTable(results, nil, false)
	})

	if !strings.Contains(output, "github.com/foo/bar") {
		t.Error("table output should contain the module path")
	}
	if !strings.Contains(output, "2024-07-22") {
		t.Error("table output should contain archived date")
	}
	if !strings.Contains(output, "direct") {
		t.Error("table output should show 'direct'")
	}
}

func TestPrintTable_NoArchived(t *testing.T) {
	results := []RepoStatus{
		{
			Module:     Module{Path: "github.com/foo/bar", Version: "v1.0.0", Direct: true, Owner: "foo", Repo: "bar"},
			IsArchived: false,
			PushedAt:   time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	skippedModules := []Module{
		{Path: "golang.org/x/text", Version: "v0.14.0", Direct: true},
		{Path: "google.golang.org/grpc", Version: "v1.60.0", Direct: true},
		{Path: "gopkg.in/yaml.v3", Version: "v3.0.1", Direct: false},
	}

	// Should not print any table to stdout when no archived
	output := captureStdout(t, func() {
		PrintTable(results, skippedModules, false)
	})

	if strings.Contains(output, "github.com/foo/bar") {
		t.Error("should not show active modules when showAll=false")
	}
	// Skipped modules should appear in stdout table
	if !strings.Contains(output, "golang.org/x/text") {
		t.Error("should show skipped module golang.org/x/text")
	}
	if !strings.Contains(output, "google.golang.org/grpc") {
		t.Error("should show skipped module google.golang.org/grpc")
	}
}

func TestPrintTable_ShowAll(t *testing.T) {
	results := []RepoStatus{
		{
			Module:     Module{Path: "github.com/foo/bar", Version: "v1.0.0", Direct: false, Owner: "foo", Repo: "bar"},
			IsArchived: false,
			PushedAt:   time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			Module:     Module{Path: "github.com/archived/repo", Version: "v0.5.0", Direct: true, Owner: "archived", Repo: "repo"},
			IsArchived: true,
			ArchivedAt: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
			PushedAt:   time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	skippedModules := []Module{
		{Path: "golang.org/x/text", Version: "v0.14.0", Direct: true},
		{Path: "google.golang.org/grpc", Version: "v1.60.0", Direct: false},
	}

	output := captureStdout(t, func() {
		PrintTable(results, skippedModules, true)
	})

	if !strings.Contains(output, "github.com/archived/repo") {
		t.Error("should show archived module")
	}
	if !strings.Contains(output, "github.com/foo/bar") {
		t.Error("should show active module when showAll=true")
	}
	if !strings.Contains(output, "indirect") {
		t.Error("should show 'indirect' for indirect dep")
	}
}

func TestPrintTable_NotFoundModule(t *testing.T) {
	results := []RepoStatus{
		{
			Module:   Module{Path: "github.com/gone/repo", Owner: "gone", Repo: "repo"},
			NotFound: true,
			Error:    "Could not resolve",
		},
	}

	// NotFound goes to stderr, stdout should be empty
	output := captureStdout(t, func() {
		PrintTable(results, nil, false)
	})

	if strings.Contains(output, "github.com/gone/repo") {
		t.Error("not-found modules should go to stderr, not stdout")
	}
}

func TestPrintTree_BasicTree(t *testing.T) {
	results := []RepoStatus{
		{
			Module:     Module{Path: "github.com/x/y", Version: "v0.1.0", Owner: "x", Repo: "y"},
			IsArchived: true,
			ArchivedAt: time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC),
			PushedAt:   time.Date(2023, 12, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	allModules := []Module{
		{Path: "github.com/a/b", Version: "v1.0.0", Owner: "a", Repo: "b", Direct: true},
		{Path: "github.com/x/y", Version: "v0.1.0", Owner: "x", Repo: "y", Direct: false},
	}

	graph := map[string][]string{
		"mymodule":                   {"github.com/a/b@v1.0.0"},
		"github.com/a/b@v1.0.0":     {"github.com/x/y@v0.1.0"},
		"github.com/x/y@v0.1.0":     {},
	}

	output := captureStdout(t, func() {
		PrintTree(results, graph, allModules, nil)
	})

	if !strings.Contains(output, "github.com/a/b@v1.0.0") {
		t.Error("should show direct dep with version")
	}
	if !strings.Contains(output, "github.com/x/y@v0.1.0") {
		t.Error("should show archived transitive dep with version")
	}
	if !strings.Contains(output, "ARCHIVED 2024-03-15") {
		t.Error("should show archived date")
	}
	if !strings.Contains(output, "last pushed 2023-12-01") {
		t.Error("should show last pushed date")
	}
}

func TestPrintTree_DirectArchived(t *testing.T) {
	results := []RepoStatus{
		{
			Module:     Module{Path: "github.com/a/b", Version: "v1.0.0", Owner: "a", Repo: "b"},
			IsArchived: true,
			ArchivedAt: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
			PushedAt:   time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	allModules := []Module{
		{Path: "github.com/a/b", Version: "v1.0.0", Owner: "a", Repo: "b", Direct: true},
	}

	graph := map[string][]string{
		"mymodule":               {"github.com/a/b@v1.0.0"},
		"github.com/a/b@v1.0.0": {},
	}

	output := captureStdout(t, func() {
		PrintTree(results, graph, allModules, nil)
	})

	if !strings.Contains(output, "github.com/a/b@v1.0.0 [ARCHIVED 2024-06-01, last pushed 2024-05-01]") {
		t.Errorf("should show direct dep as archived with version and dates, got:\n%s", output)
	}
}

func TestPrintTree_NoArchived(t *testing.T) {
	results := []RepoStatus{
		{
			Module:     Module{Path: "github.com/a/b", Owner: "a", Repo: "b"},
			IsArchived: false,
		},
	}

	allModules := []Module{
		{Path: "github.com/a/b", Owner: "a", Repo: "b", Direct: true},
	}

	graph := map[string][]string{
		"mymodule":               {"github.com/a/b@v1.0.0"},
		"github.com/a/b@v1.0.0": {},
	}

	output := captureStdout(t, func() {
		PrintTree(results, graph, allModules, nil)
	})

	if output != "" {
		t.Errorf("expected no stdout output when no archived deps, got %q", output)
	}
}

func TestPrintTree_EmptyGraph(t *testing.T) {
	results := []RepoStatus{
		{
			Module:     Module{Path: "github.com/a/b", Version: "v1.0.0", Owner: "a", Repo: "b"},
			IsArchived: true,
			ArchivedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	allModules := []Module{
		{Path: "github.com/a/b", Version: "v1.0.0", Owner: "a", Repo: "b", Direct: true},
	}

	// Empty graph — no root key found, fallback to flat list
	graph := map[string][]string{}

	output := captureStdout(t, func() {
		PrintTree(results, graph, allModules, nil)
	})

	if !strings.Contains(output, "github.com/a/b@v1.0.0 [ARCHIVED") {
		t.Errorf("fallback should still list archived deps with version, got:\n%s", output)
	}
}

func TestParseModGraphLines(t *testing.T) {
	input := `root github.com/foo/bar@v1.0.0
root github.com/baz/qux@v2.0.0
github.com/foo/bar@v1.0.0 github.com/x/y@v0.1.0
`
	graph := make(map[string][]string)
	for _, line := range strings.Split(strings.TrimSpace(input), "\n") {
		parts := strings.Fields(line)
		if len(parts) == 2 {
			graph[parts[0]] = append(graph[parts[0]], parts[1])
		}
	}

	if len(graph["root"]) != 2 {
		t.Errorf("root should have 2 children, got %d", len(graph["root"]))
	}
	if len(graph["github.com/foo/bar@v1.0.0"]) != 1 {
		t.Errorf("foo/bar should have 1 child, got %d", len(graph["github.com/foo/bar@v1.0.0"]))
	}
}

func TestPrintFiles_BasicOutput(t *testing.T) {
	results := []RepoStatus{
		{
			Module:     Module{Path: "github.com/foo/bar", Version: "v1.0.0", Owner: "foo", Repo: "bar"},
			IsArchived: true,
		},
		{
			Module:     Module{Path: "github.com/baz/qux", Version: "v2.0.0", Owner: "baz", Repo: "qux"},
			IsArchived: true,
		},
	}

	fileMatches := map[string][]FileMatch{
		"github.com/foo/bar": {
			{File: "audit/hash.go", Line: 14, ImportPath: "github.com/foo/bar"},
			{File: "vault/policy.go", Line: 17, ImportPath: "github.com/foo/bar"},
		},
		"github.com/baz/qux": {
			{File: "cmd/main.go", Line: 5, ImportPath: "github.com/baz/qux"},
		},
	}

	output := captureStdout(t, func() {
		PrintFiles(results, fileMatches)
	})

	if !strings.Contains(output, "github.com/baz/qux (1 file)") {
		t.Errorf("should show singular 'file' for 1 match, got:\n%s", output)
	}
	if !strings.Contains(output, "github.com/foo/bar (2 files)") {
		t.Errorf("should show '2 files', got:\n%s", output)
	}
	if !strings.Contains(output, "audit/hash.go:14") {
		t.Errorf("should show file:line, got:\n%s", output)
	}
	if !strings.Contains(output, "vault/policy.go:17") {
		t.Errorf("should show file:line, got:\n%s", output)
	}
}

func TestPrintFiles_ZeroFiles(t *testing.T) {
	results := []RepoStatus{
		{
			Module:     Module{Path: "github.com/foo/bar", Version: "v1.0.0", Owner: "foo", Repo: "bar"},
			IsArchived: true,
		},
	}

	fileMatches := map[string][]FileMatch{}

	output := captureStdout(t, func() {
		PrintFiles(results, fileMatches)
	})

	if !strings.Contains(output, "github.com/foo/bar (0 files)") {
		t.Errorf("modules with no imports should show 0 files, got:\n%s", output)
	}
}

func TestPrintJSON_WithSourceFiles(t *testing.T) {
	results := []RepoStatus{
		{
			Module:     Module{Path: "github.com/foo/bar", Version: "v1.0.0", Direct: true, Owner: "foo", Repo: "bar"},
			IsArchived: true,
			ArchivedAt: time.Date(2024, 7, 22, 0, 0, 0, 0, time.UTC),
		},
	}

	fileMatches := map[string][]FileMatch{
		"github.com/foo/bar": {
			{File: "audit/hash.go", Line: 14, ImportPath: "github.com/foo/bar"},
		},
	}

	output := captureStdout(t, func() {
		PrintJSON(results, nil, false, fileMatches)
	})

	var out JSONOutput
	if err := json.Unmarshal([]byte(output), &out); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, output)
	}

	if len(out.Archived) != 1 {
		t.Fatalf("expected 1 archived, got %d", len(out.Archived))
	}
	if len(out.Archived[0].SourceFiles) != 1 {
		t.Fatalf("expected 1 source file, got %d", len(out.Archived[0].SourceFiles))
	}
	sf := out.Archived[0].SourceFiles[0]
	if sf.File != "audit/hash.go" {
		t.Errorf("source_files[0].file = %q, want %q", sf.File, "audit/hash.go")
	}
	if sf.Line != 14 {
		t.Errorf("source_files[0].line = %d, want 14", sf.Line)
	}
	if sf.Import != "github.com/foo/bar" {
		t.Errorf("source_files[0].import = %q, want %q", sf.Import, "github.com/foo/bar")
	}
}

func TestPrintJSON_NoSourceFilesWhenNil(t *testing.T) {
	results := []RepoStatus{
		{
			Module:     Module{Path: "github.com/foo/bar", Version: "v1.0.0", Direct: true, Owner: "foo", Repo: "bar"},
			IsArchived: true,
		},
	}

	output := captureStdout(t, func() {
		PrintJSON(results, nil, false, nil)
	})

	if strings.Contains(output, "source_files") {
		t.Errorf("should not include source_files when fileMatches is nil, got:\n%s", output)
	}
}

func TestPrintTree_WithFileCount(t *testing.T) {
	results := []RepoStatus{
		{
			Module:     Module{Path: "github.com/a/b", Version: "v1.0.0", Owner: "a", Repo: "b"},
			IsArchived: true,
			ArchivedAt: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	allModules := []Module{
		{Path: "github.com/a/b", Version: "v1.0.0", Owner: "a", Repo: "b", Direct: true},
	}

	graph := map[string][]string{
		"mymodule":               {"github.com/a/b@v1.0.0"},
		"github.com/a/b@v1.0.0": {},
	}

	fileMatches := map[string][]FileMatch{
		"github.com/a/b": {
			{File: "foo.go", Line: 5, ImportPath: "github.com/a/b"},
			{File: "bar.go", Line: 10, ImportPath: "github.com/a/b"},
		},
	}

	output := captureStdout(t, func() {
		PrintTree(results, graph, allModules, fileMatches)
	})

	if !strings.Contains(output, "(2 files)") {
		t.Errorf("tree output should show file count, got:\n%s", output)
	}
}

func TestBuildTree_BasicEntries(t *testing.T) {
	results := []RepoStatus{
		{
			Module:     Module{Path: "github.com/x/y", Version: "v0.1.0", Owner: "x", Repo: "y"},
			IsArchived: true,
			ArchivedAt: time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC),
		},
	}

	allModules := []Module{
		{Path: "github.com/a/b", Version: "v1.0.0", Owner: "a", Repo: "b", Direct: true},
		{Path: "github.com/x/y", Version: "v0.1.0", Owner: "x", Repo: "y", Direct: false},
	}

	graph := map[string][]string{
		"mymodule":               {"github.com/a/b@v1.0.0"},
		"github.com/a/b@v1.0.0": {"github.com/x/y@v0.1.0"},
		"github.com/x/y@v0.1.0": {},
	}

	entries, ctx := buildTree(results, graph, allModules)

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].directPath != "github.com/a/b" {
		t.Errorf("directPath = %q, want github.com/a/b", entries[0].directPath)
	}
	if len(entries[0].archived) != 1 || entries[0].archived[0] != "github.com/x/y" {
		t.Errorf("archived = %v, want [github.com/x/y]", entries[0].archived)
	}
	if !ctx.archivedPaths["github.com/x/y"] {
		t.Error("expected x/y in archivedPaths")
	}
}

func TestBuildTree_NoArchived(t *testing.T) {
	results := []RepoStatus{
		{
			Module:     Module{Path: "github.com/a/b", Owner: "a", Repo: "b"},
			IsArchived: false,
		},
	}
	allModules := []Module{
		{Path: "github.com/a/b", Owner: "a", Repo: "b"},
	}
	graph := map[string][]string{
		"mymodule":               {"github.com/a/b@v1.0.0"},
		"github.com/a/b@v1.0.0": {},
	}

	entries, _ := buildTree(results, graph, allModules)
	if entries != nil {
		t.Errorf("expected nil entries when no archived, got %v", entries)
	}
}

func TestBuildTree_EmptyGraph(t *testing.T) {
	results := []RepoStatus{
		{
			Module:     Module{Path: "github.com/a/b", Version: "v1.0.0", Owner: "a", Repo: "b"},
			IsArchived: true,
		},
	}
	allModules := []Module{
		{Path: "github.com/a/b", Version: "v1.0.0", Owner: "a", Repo: "b"},
	}

	entries, _ := buildTree(results, map[string][]string{}, allModules)
	if len(entries) != 1 {
		t.Fatalf("expected 1 fallback entry, got %d", len(entries))
	}
	if entries[0].directPath != "github.com/a/b" {
		t.Errorf("directPath = %q", entries[0].directPath)
	}
}

func TestPrintTreeJSON_Basic(t *testing.T) {
	results := []RepoStatus{
		{
			Module:     Module{Path: "github.com/x/y", Version: "v0.1.0", Owner: "x", Repo: "y"},
			IsArchived: true,
			ArchivedAt: time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC),
			PushedAt:   time.Date(2023, 12, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	allModules := []Module{
		{Path: "github.com/a/b", Version: "v1.0.0", Owner: "a", Repo: "b", Direct: true},
		{Path: "github.com/x/y", Version: "v0.1.0", Owner: "x", Repo: "y", Direct: false},
	}

	graph := map[string][]string{
		"mymodule":               {"github.com/a/b@v1.0.0"},
		"github.com/a/b@v1.0.0": {"github.com/x/y@v0.1.0"},
		"github.com/x/y@v0.1.0": {},
	}

	skippedModules := []Module{
		{Path: "golang.org/x/text", Version: "v0.14.0", Direct: true},
		{Path: "google.golang.org/protobuf", Version: "v1.33.0", Direct: false},
		{Path: "gopkg.in/yaml.v3", Version: "v3.0.1", Direct: true},
		{Path: "cel.dev/expr", Version: "v0.25.1", Direct: false},
		{Path: "golang.org/x/net", Version: "v0.24.0", Direct: true},
	}

	output := captureStdout(t, func() {
		PrintTreeJSON(results, graph, allModules, nil, skippedModules)
	})

	var out JSONTreeOutput
	if err := json.Unmarshal([]byte(output), &out); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, output)
	}

	if out.SkippedNonGH != 5 {
		t.Errorf("skipped = %d, want 5", out.SkippedNonGH)
	}
	if len(out.SkippedModules) != 5 {
		t.Errorf("skipped_modules length = %d, want 5", len(out.SkippedModules))
	}
	if out.SkippedModules[0].Module != "golang.org/x/text" {
		t.Errorf("skipped_modules[0].module = %q, want %q", out.SkippedModules[0].Module, "golang.org/x/text")
	}
	if len(out.Tree) != 1 {
		t.Fatalf("expected 1 tree entry, got %d", len(out.Tree))
	}

	entry := out.Tree[0]
	if entry.Module != "github.com/a/b" {
		t.Errorf("module = %q", entry.Module)
	}
	if entry.Version != "v1.0.0" {
		t.Errorf("version = %q", entry.Version)
	}
	if entry.Archived {
		t.Error("direct dep should not be archived")
	}
	if len(entry.ArchivedDependencies) != 1 {
		t.Fatalf("expected 1 archived dep, got %d", len(entry.ArchivedDependencies))
	}

	dep := entry.ArchivedDependencies[0]
	if dep.Module != "github.com/x/y" {
		t.Errorf("dep module = %q", dep.Module)
	}
	if dep.ArchivedAt != "2024-03-15T00:00:00Z" {
		t.Errorf("dep archived_at = %q", dep.ArchivedAt)
	}
}

func TestPrintTreeJSON_DirectArchived(t *testing.T) {
	results := []RepoStatus{
		{
			Module:     Module{Path: "github.com/a/b", Version: "v1.0.0", Owner: "a", Repo: "b"},
			IsArchived: true,
			ArchivedAt: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
			PushedAt:   time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	allModules := []Module{
		{Path: "github.com/a/b", Version: "v1.0.0", Owner: "a", Repo: "b", Direct: true},
	}

	graph := map[string][]string{
		"mymodule":               {"github.com/a/b@v1.0.0"},
		"github.com/a/b@v1.0.0": {},
	}

	output := captureStdout(t, func() {
		PrintTreeJSON(results, graph, allModules, nil, nil)
	})

	var out JSONTreeOutput
	if err := json.Unmarshal([]byte(output), &out); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if len(out.Tree) != 1 {
		t.Fatalf("expected 1 tree entry, got %d", len(out.Tree))
	}
	if !out.Tree[0].Archived {
		t.Error("direct dep should be archived")
	}
	if out.Tree[0].ArchivedAt != "2024-06-01T00:00:00Z" {
		t.Errorf("archived_at = %q", out.Tree[0].ArchivedAt)
	}
}

func TestPrintTreeJSON_WithSourceFiles(t *testing.T) {
	results := []RepoStatus{
		{
			Module:     Module{Path: "github.com/x/y", Version: "v0.1.0", Owner: "x", Repo: "y"},
			IsArchived: true,
			ArchivedAt: time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC),
		},
	}

	allModules := []Module{
		{Path: "github.com/a/b", Version: "v1.0.0", Owner: "a", Repo: "b"},
		{Path: "github.com/x/y", Version: "v0.1.0", Owner: "x", Repo: "y"},
	}

	graph := map[string][]string{
		"mymodule":               {"github.com/a/b@v1.0.0"},
		"github.com/a/b@v1.0.0": {"github.com/x/y@v0.1.0"},
	}

	fileMatches := map[string][]FileMatch{
		"github.com/x/y": {
			{File: "foo.go", Line: 5, ImportPath: "github.com/x/y"},
		},
	}

	output := captureStdout(t, func() {
		PrintTreeJSON(results, graph, allModules, fileMatches, nil)
	})

	var out JSONTreeOutput
	if err := json.Unmarshal([]byte(output), &out); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	dep := out.Tree[0].ArchivedDependencies[0]
	if len(dep.SourceFiles) != 1 {
		t.Fatalf("expected 1 source file, got %d", len(dep.SourceFiles))
	}
	if dep.SourceFiles[0].File != "foo.go" {
		t.Errorf("source file = %q", dep.SourceFiles[0].File)
	}
}

func TestPrintTreeJSON_NoArchived(t *testing.T) {
	results := []RepoStatus{
		{
			Module:     Module{Path: "github.com/a/b", Owner: "a", Repo: "b"},
			IsArchived: false,
		},
	}
	allModules := []Module{
		{Path: "github.com/a/b", Owner: "a", Repo: "b"},
	}
	graph := map[string][]string{
		"mymodule":               {"github.com/a/b@v1.0.0"},
		"github.com/a/b@v1.0.0": {},
	}

	output := captureStdout(t, func() {
		PrintTreeJSON(results, graph, allModules, nil, nil)
	})

	var out JSONTreeOutput
	if err := json.Unmarshal([]byte(output), &out); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if !strings.Contains(output, `"tree": []`) {
		t.Error("expected tree to be empty array, not null")
	}
}

func TestPluralize(t *testing.T) {
	if got := pluralize(0, "file", "files"); got != "files" {
		t.Errorf("pluralize(0) = %q, want %q", got, "files")
	}
	if got := pluralize(1, "file", "files"); got != "file" {
		t.Errorf("pluralize(1) = %q, want %q", got, "file")
	}
	if got := pluralize(2, "file", "files"); got != "files" {
		t.Errorf("pluralize(2) = %q, want %q", got, "files")
	}
}
