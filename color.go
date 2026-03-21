package main

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// ANSI color codes — colorblind-safe palette.
const (
	colorReset = "\033[0m"

	colorBoldCyan      = "\033[1;36m"   // prominent: new
	colorCyan          = "\033[36m"     // progression
	colorYellow        = "\033[33m"     // middle
	colorMagenta       = "\033[35m"     // progression
	colorBoldMagentaUL = "\033[1;4;35m" // prominent: critical
)

// Color/symbol pairs ordered from newest to oldest.
// Prominent at both ends; progression in the middle.
var levelStyles = []struct {
	color  string
	symbol string
}{
	{colorBoldCyan, "★"},      // newest: just appeared
	{colorCyan, "◇"},          // recent: emerging
	{colorYellow, "◆"},        // moderate: established
	{colorMagenta, "▲"},       // old: growing concern
	{colorBoldMagentaUL, "✖"}, // critical: long-standing
}

// colorThreshold holds a single parsed threshold (years, months, days).
type colorThreshold struct {
	y, m, d int
}

// colorConfig holds the color/symbol feature state.
var colorConfig struct {
	enabled    bool
	thresholds []colorThreshold // N boundaries → N+1 levels
}

// isTerminal returns true if stdout is a terminal (character device).
func isTerminal() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// initColor sets up color support based on terminal detection and environment.
// Called after flag parsing with the user's threshold string (may be empty for defaults).
func initColor(noColor bool, threshold string) error {
	// Disabled by flag or NO_COLOR env var
	if noColor || os.Getenv("NO_COLOR") != "" {
		colorConfig.enabled = false
		return nil
	}

	// Auto-detect: only enable for terminals
	if !isTerminal() {
		colorConfig.enabled = false
		return nil
	}

	colorConfig.enabled = true

	// Default: 3m,1y,2y,5y (non-linear, front-loaded for new issues)
	if threshold == "" {
		threshold = "3m,1y,2y,5y"
	}

	return parseColorThreshold(threshold)
}

// parseColorThreshold parses a comma-separated threshold string with 2–4 values.
// Each part uses the same format as --stale (e.g. "1y", "1y6m", "180d").
//
//	2 values → 3 levels (new, middle, critical)
//	3 values → 4 levels (new, recent, old, critical)
//	4 values → 5 levels (new, recent, moderate, old, critical)
func parseColorThreshold(s string) error {
	parts := strings.Split(s, ",")
	if len(parts) < 2 || len(parts) > 4 {
		return fmt.Errorf("invalid color threshold %q (expected 2–4 values e.g. 1y,3y or 3m,1y,2y,5y)", s)
	}

	colorConfig.thresholds = make([]colorThreshold, len(parts))
	for i, p := range parts {
		y, m, d, err := parseThreshold(p)
		if err != nil {
			return fmt.Errorf("invalid threshold %q: %w", p, err)
		}
		colorConfig.thresholds[i] = colorThreshold{y, m, d}
	}

	return nil
}

// classifyAge returns the level index (0 = newest) for a timestamp.
// Returns -1 if the timestamp is below all thresholds (no decoration).
// The number of possible levels is len(thresholds) + 1.
func classifyAge(t time.Time) int {
	if t.IsZero() {
		return -1
	}
	// Check from oldest threshold to newest, return the highest matching level.
	for i := len(colorConfig.thresholds) - 1; i >= 0; i-- {
		th := colorConfig.thresholds[i]
		if exceedsThreshold(t, th.y, th.m, th.d) {
			return i + 1 // levels are 0-based: 0=newest, N=oldest
		}
	}
	return 0 // below first threshold = newest level
}

// selectStyle picks the color and symbol for a given level index,
// mapping N+1 levels onto the 5-entry style palette.
// Both ends are always prominent; middle levels are distributed evenly.
func selectStyle(level, totalLevels int) (string, string) {
	if totalLevels <= 1 {
		return levelStyles[0].color, levelStyles[0].symbol
	}
	// Map level (0..totalLevels-1) onto style index (0..4)
	idx := level * (len(levelStyles) - 1) / (totalLevels - 1)
	return levelStyles[idx].color, levelStyles[idx].symbol
}

// colorize wraps a string with color and symbol based on the age of a timestamp.
// Returns the string unchanged if color is disabled.
func colorize(s string, t time.Time) string {
	if !colorConfig.enabled {
		return s
	}
	level := classifyAge(t)
	if level < 0 {
		return s
	}
	totalLevels := len(colorConfig.thresholds) + 1
	color, symbol := selectStyle(level, totalLevels)
	return fmt.Sprintf("%s%s %s%s", color, symbol, s, colorReset)
}
