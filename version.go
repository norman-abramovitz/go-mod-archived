package main

import (
	"fmt"
	"runtime/debug"
	"strings"
)

// version and buildDate are set via ldflags at build time.
// For local builds, they remain "dev" and "unknown".
var (
	version   = "dev"
	buildDate = "unknown"
)

// claudeAttribution is embedded as a constant so it appears in the compiled
// binary and is discoverable via: strings modrot | grep Claude
const claudeAttribution = "Built with the assistance of Claude, an AI assistant by Anthropic"

// vcsInfo holds version control information extracted from debug.BuildInfo.
type vcsInfo struct {
	Revision   string
	ShortRev   string
	Time       string
	Modified   bool
	GoVersion  string
	ModulePath string
}

// extractVCSInfo extracts VCS metadata from debug.BuildInfo.
func extractVCSInfo(info *debug.BuildInfo) vcsInfo {
	v := vcsInfo{
		GoVersion:  info.GoVersion,
		ModulePath: info.Main.Path,
	}
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			v.Revision = s.Value
			v.ShortRev = s.Value
			if len(v.ShortRev) > 12 {
				v.ShortRev = v.ShortRev[:12]
			}
		case "vcs.time":
			v.Time = s.Value
		case "vcs.modified":
			v.Modified = s.Value == "true"
		}
	}
	return v
}

// formatVersion builds the full version output string.
func formatVersion() string {
	var b strings.Builder
	fmt.Fprintf(&b, "modrot %s\n", version)

	if buildDate != "unknown" {
		fmt.Fprintf(&b, "  Build date:    %s\n", buildDate)
	}

	if info, ok := debug.ReadBuildInfo(); ok {
		vcs := extractVCSInfo(info)
		fmt.Fprintf(&b, "  Go version:    %s\n", vcs.GoVersion)

		if vcs.Revision != "" {
			fmt.Fprintf(&b, "  Commit:        %s (%s)\n", vcs.ShortRev, vcs.Revision)
		}
		if vcs.Time != "" {
			fmt.Fprintf(&b, "  Commit date:   %s\n", vcs.Time)
		}
		if vcs.Revision != "" {
			state := "clean"
			if vcs.Modified {
				state = "dirty"
			}
			fmt.Fprintf(&b, "  Repo state:    %s\n", state)
		}
		if vcs.ModulePath != "" {
			fmt.Fprintf(&b, "  Repository:    https://%s\n", vcs.ModulePath)
		}
	}

	fmt.Fprintf(&b, "\n  %s\n", claudeAttribution)
	return b.String()
}

func printVersion() {
	fmt.Print(formatVersion())
}
