package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	// Reorder args so flags can appear after the positional argument.
	// Go's flag package stops parsing at the first non-flag argument.
	reorderArgs()

	jsonFlag := flag.Bool("json", false, "Output as JSON")
	allFlag := flag.Bool("all", false, "Show all modules, not just archived ones")
	directOnly := flag.Bool("direct-only", false, "Only check direct dependencies")
	workers := flag.Int("workers", 50, "Number of repos per GitHub GraphQL batch request")
	treeFlag := flag.Bool("tree", false, "Show dependency tree for archived modules (uses go mod graph)")
	filesFlag := flag.Bool("files", false, "Show source files that import archived modules")
	timeFlag := flag.Bool("time", false, "Include time in date output (2006-01-02 15:04:05 instead of 2006-01-02)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: go-mod-archived [flags] [path/to/go.mod | path/to/dir]\n\nDetect archived GitHub dependencies in a Go project.\n\nFlags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	// Set date format
	if *timeFlag {
		dateFmt = "2006-01-02 15:04:05"
	}

	// Determine go.mod path
	gomodPath := "go.mod"
	if flag.NArg() > 0 {
		gomodPath = flag.Arg(0)
	}
	gomodPath, err := filepath.Abs(gomodPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(2)
	}
	// If the path is a directory, look for go.mod inside it.
	if info, err := os.Stat(gomodPath); err == nil && info.IsDir() {
		gomodPath = filepath.Join(gomodPath, "go.mod")
	}

	// Parse go.mod
	allModules, err := ParseGoMod(gomodPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(2)
	}

	// Filter to GitHub modules and deduplicate
	githubModules, nonGitHubCount := FilterGitHub(allModules, *directOnly)

	if len(githubModules) == 0 {
		fmt.Fprintf(os.Stderr, "No GitHub modules found in %s\n", gomodPath)
		os.Exit(0)
	}

	fmt.Fprintf(os.Stderr, "Checking %d GitHub modules...\n", len(githubModules))

	// Query GitHub
	results, err := CheckRepos(githubModules, *workers)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(2)
	}

	// Enrich results with direct/indirect info from all modules (not just deduplicated)
	// The deduplicated set loses some info, but we kept Direct from the first occurrence.

	// Check if any archived
	hasArchived := false
	var archivedModulePaths []string
	for _, r := range results {
		if r.IsArchived {
			hasArchived = true
			archivedModulePaths = append(archivedModulePaths, r.Module.Path)
		}
	}

	// Scan source files for imports of archived modules
	var fileMatches map[string][]FileMatch
	if *filesFlag && hasArchived {
		fm, err := ScanImports(filepath.Dir(gomodPath), archivedModulePaths)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error scanning imports: %v\n", err)
			os.Exit(2)
		}
		fileMatches = fm
	}

	// Handle --tree mode
	if *treeFlag && hasArchived {
		graph, err := parseModGraph(filepath.Dir(gomodPath))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not run go mod graph: %v\n", err)
		} else {
			if *jsonFlag {
				PrintTreeJSON(results, graph, allModules, fileMatches, nonGitHubCount)
			} else {
				PrintTree(results, graph, allModules, fileMatches)
				if nonGitHubCount > 0 {
					fmt.Fprintf(os.Stderr, "\nSkipped %d non-GitHub modules.\n", nonGitHubCount)
				}
			}
			if hasArchived {
				os.Exit(1)
			}
			os.Exit(0)
		}
	}

	// Output
	if *jsonFlag {
		PrintJSON(results, nonGitHubCount, *allFlag, fileMatches)
	} else {
		PrintTable(results, nonGitHubCount, *allFlag)
		if fileMatches != nil {
			PrintFiles(results, fileMatches)
		}
	}

	if hasArchived {
		os.Exit(1)
	}
}

// valueFlagNames lists flags that take a value argument (not boolean).
var valueFlagNames = map[string]bool{
	"-workers": true, "--workers": true,
}

// reorderArgs moves flags after positional arguments to before them,
// so Go's flag package can parse them. For example:
//
//	go-mod-archived path/to/go.mod --files --tree
//
// becomes:
//
//	go-mod-archived --files --tree path/to/go.mod
func reorderArgs() {
	var flags, positional []string
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			flags = append(flags, arg)
			// If this flag takes a value and it's not using = syntax, consume the next arg too.
			if valueFlagNames[arg] && !strings.Contains(arg, "=") && i+1 < len(args) {
				i++
				flags = append(flags, args[i])
			}
		} else {
			positional = append(positional, arg)
		}
	}
	reordered := make([]string, 0, 1+len(flags)+len(positional))
	reordered = append(reordered, os.Args[0])
	reordered = append(reordered, flags...)
	reordered = append(reordered, positional...)
	os.Args = reordered
}

// parseModGraph runs `go mod graph` in the given directory and returns
// a map of parent → []child (both as "module@version" strings).
func parseModGraph(dir string) (map[string][]string, error) {
	cmd := exec.Command("go", "mod", "graph")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	graph := make(map[string][]string)
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) != 2 {
			continue
		}
		parent, child := parts[0], parts[1]
		graph[parent] = append(graph[parent], child)
	}
	return graph, scanner.Err()
}
