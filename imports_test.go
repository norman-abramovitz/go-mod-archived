package main

import (
	"reflect"
	"testing"
)

func TestBuildImportPattern(t *testing.T) {
	paths := []string{
		"github.com/mitchellh/copystructure",
		"github.com/pkg/errors",
	}
	got := buildImportPattern(paths)
	want := `"(github\.com/mitchellh/copystructure|github\.com/pkg/errors)(/|")`
	if got != want {
		t.Errorf("buildImportPattern() =\n  %q\nwant:\n  %q", got, want)
	}
}

func TestBuildImportPattern_Single(t *testing.T) {
	got := buildImportPattern([]string{"github.com/foo/bar"})
	want := `"(github\.com/foo/bar)(/|")`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestParseRgLine(t *testing.T) {
	tests := []struct {
		input   string
		file    string
		lineNum int
		content string
		ok      bool
	}{
		{
			input:   `/path/to/file.go:14:	"github.com/mitchellh/copystructure"`,
			file:    "/path/to/file.go",
			lineNum: 14,
			content: `	"github.com/mitchellh/copystructure"`,
			ok:      true,
		},
		{
			input:   `foo.go:1:import "github.com/foo/bar"`,
			file:    "foo.go",
			lineNum: 1,
			content: `import "github.com/foo/bar"`,
			ok:      true,
		},
		{
			input: "no-colons-here",
			ok:    false,
		},
		{
			input: "file:notanumber:content",
			ok:    false,
		},
		{
			input: "file:123",
			ok:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			file, lineNum, content, ok := parseRgLine(tt.input)
			if ok != tt.ok {
				t.Fatalf("ok = %v, want %v", ok, tt.ok)
			}
			if !ok {
				return
			}
			if file != tt.file {
				t.Errorf("file = %q, want %q", file, tt.file)
			}
			if lineNum != tt.lineNum {
				t.Errorf("lineNum = %d, want %d", lineNum, tt.lineNum)
			}
			if content != tt.content {
				t.Errorf("content = %q, want %q", content, tt.content)
			}
		})
	}
}

func TestMatchModule(t *testing.T) {
	// Must be sorted longest-first (as matchModule expects)
	modules := []string{
		"github.com/hashicorp/go-discover/provider/aws",
		"github.com/hashicorp/go-discover",
		"github.com/mitchellh/copystructure",
		"github.com/pkg/errors",
	}

	tests := []struct {
		importPath string
		want       string
	}{
		// Exact match
		{"github.com/mitchellh/copystructure", "github.com/mitchellh/copystructure"},
		// Subpackage match
		{"github.com/hashicorp/go-discover/provider/aws", "github.com/hashicorp/go-discover/provider/aws"},
		// Subpackage match, longer module wins
		{"github.com/hashicorp/go-discover/provider/aws/sub", "github.com/hashicorp/go-discover/provider/aws"},
		// Subpackage match, shorter module
		{"github.com/hashicorp/go-discover/provider/gce", "github.com/hashicorp/go-discover"},
		// No match
		{"github.com/unrelated/thing", ""},
		// Prefix but not at path boundary (should NOT match)
		{"github.com/pkg/errorsx", ""},
	}

	for _, tt := range tests {
		t.Run(tt.importPath, func(t *testing.T) {
			got := matchModule(tt.importPath, modules)
			if got != tt.want {
				t.Errorf("matchModule(%q) = %q, want %q", tt.importPath, got, tt.want)
			}
		})
	}
}

func TestParseRgOutput(t *testing.T) {
	modulePaths := []string{
		"github.com/mitchellh/copystructure",
		"github.com/mitchellh/reflectwalk",
	}

	rgOutput := `/proj/audit/hashstructure.go:14:	"github.com/mitchellh/copystructure"
/proj/vault/policy.go:17:	"github.com/mitchellh/copystructure"
/proj/audit/hashstructure.go:15:	"github.com/mitchellh/reflectwalk"
`

	got := parseRgOutput(rgOutput, "/proj", modulePaths)

	wantCopy := []FileMatch{
		{File: "audit/hashstructure.go", Line: 14, ImportPath: "github.com/mitchellh/copystructure"},
		{File: "vault/policy.go", Line: 17, ImportPath: "github.com/mitchellh/copystructure"},
	}
	wantWalk := []FileMatch{
		{File: "audit/hashstructure.go", Line: 15, ImportPath: "github.com/mitchellh/reflectwalk"},
	}

	if !reflect.DeepEqual(got["github.com/mitchellh/copystructure"], wantCopy) {
		t.Errorf("copystructure matches =\n  %+v\nwant:\n  %+v", got["github.com/mitchellh/copystructure"], wantCopy)
	}
	if !reflect.DeepEqual(got["github.com/mitchellh/reflectwalk"], wantWalk) {
		t.Errorf("reflectwalk matches =\n  %+v\nwant:\n  %+v", got["github.com/mitchellh/reflectwalk"], wantWalk)
	}
}

func TestParseRgOutput_Subpackage(t *testing.T) {
	modulePaths := []string{
		"github.com/hashicorp/go-discover",
	}

	rgOutput := `/proj/foo.go:5:	"github.com/hashicorp/go-discover/provider/aws"
`

	got := parseRgOutput(rgOutput, "/proj", modulePaths)

	want := []FileMatch{
		{File: "foo.go", Line: 5, ImportPath: "github.com/hashicorp/go-discover/provider/aws"},
	}

	if !reflect.DeepEqual(got["github.com/hashicorp/go-discover"], want) {
		t.Errorf("got %+v, want %+v", got["github.com/hashicorp/go-discover"], want)
	}
}

func TestParseRgOutput_NoMatches(t *testing.T) {
	got := parseRgOutput("", "/proj", []string{"github.com/foo/bar"})
	if len(got) != 0 {
		t.Errorf("expected empty map, got %+v", got)
	}
}

func TestParseRgOutput_SortedByFileThenLine(t *testing.T) {
	modulePaths := []string{"github.com/foo/bar"}

	rgOutput := `/proj/z.go:10:	"github.com/foo/bar"
/proj/a.go:20:	"github.com/foo/bar"
/proj/a.go:5:	"github.com/foo/bar"
`

	got := parseRgOutput(rgOutput, "/proj", modulePaths)
	matches := got["github.com/foo/bar"]

	if len(matches) != 3 {
		t.Fatalf("expected 3 matches, got %d", len(matches))
	}
	if matches[0].File != "a.go" || matches[0].Line != 5 {
		t.Errorf("first match should be a.go:5, got %s:%d", matches[0].File, matches[0].Line)
	}
	if matches[1].File != "a.go" || matches[1].Line != 20 {
		t.Errorf("second match should be a.go:20, got %s:%d", matches[1].File, matches[1].Line)
	}
	if matches[2].File != "z.go" || matches[2].Line != 10 {
		t.Errorf("third match should be z.go:10, got %s:%d", matches[2].File, matches[2].Line)
	}
}
