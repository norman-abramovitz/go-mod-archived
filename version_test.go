package main

import (
	"runtime/debug"
	"strings"
	"testing"
)

func TestExtractVCSInfo(t *testing.T) {
	info := &debug.BuildInfo{
		GoVersion: "go1.25.0",
		Main:      debug.Module{Path: "github.com/norman-abramovitz/modrot"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "a1b2c3d4e5f6789abcdef0123456789abcdef01"},
			{Key: "vcs.time", Value: "2026-01-15T10:30:00Z"},
			{Key: "vcs.modified", Value: "false"},
		},
	}

	vcs := extractVCSInfo(info)

	if vcs.Revision != "a1b2c3d4e5f6789abcdef0123456789abcdef01" {
		t.Errorf("Revision = %q", vcs.Revision)
	}
	if vcs.ShortRev != "a1b2c3d4e5f6" {
		t.Errorf("ShortRev = %q, want %q", vcs.ShortRev, "a1b2c3d4e5f6")
	}
	if vcs.Time != "2026-01-15T10:30:00Z" {
		t.Errorf("Time = %q", vcs.Time)
	}
	if vcs.Modified {
		t.Error("expected Modified = false")
	}
	if vcs.GoVersion != "go1.25.0" {
		t.Errorf("GoVersion = %q", vcs.GoVersion)
	}
	if vcs.ModulePath != "github.com/norman-abramovitz/modrot" {
		t.Errorf("ModulePath = %q", vcs.ModulePath)
	}
}

func TestExtractVCSInfo_Dirty(t *testing.T) {
	info := &debug.BuildInfo{
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "abc123"},
			{Key: "vcs.modified", Value: "true"},
		},
	}

	vcs := extractVCSInfo(info)
	if !vcs.Modified {
		t.Error("expected Modified = true")
	}
	if vcs.ShortRev != "abc123" {
		t.Errorf("ShortRev = %q, want %q", vcs.ShortRev, "abc123")
	}
}

func TestExtractVCSInfo_NoSettings(t *testing.T) {
	info := &debug.BuildInfo{
		GoVersion: "go1.25.0",
	}

	vcs := extractVCSInfo(info)
	if vcs.Revision != "" {
		t.Errorf("expected empty revision, got %q", vcs.Revision)
	}
	if vcs.Time != "" {
		t.Errorf("expected empty time, got %q", vcs.Time)
	}
}

func TestFormatVersion_ContainsAttribution(t *testing.T) {
	output := formatVersion()
	if !strings.Contains(output, claudeAttribution) {
		t.Error("version output should contain Claude attribution")
	}
}

func TestFormatVersion_ContainsVersionString(t *testing.T) {
	saved := version
	defer func() { version = saved }()

	version = "v1.2.3"
	output := formatVersion()
	if !strings.Contains(output, "modrot v1.2.3") {
		t.Errorf("output should contain version, got:\n%s", output)
	}
}

func TestFormatVersion_DevVersion(t *testing.T) {
	saved := version
	defer func() { version = saved }()

	version = "dev"
	output := formatVersion()
	if !strings.Contains(output, "modrot dev") {
		t.Errorf("output should show dev version, got:\n%s", output)
	}
}

func TestFormatVersion_BuildDateShownWhenSet(t *testing.T) {
	savedV, savedBD := version, buildDate
	defer func() { version = savedV; buildDate = savedBD }()

	version = "v1.0.0"
	buildDate = "2026-02-21T10:30:00Z"
	output := formatVersion()
	if !strings.Contains(output, "Build date:") {
		t.Error("should show build date when set")
	}
	if !strings.Contains(output, "2026-02-21T10:30:00Z") {
		t.Errorf("should contain build date value, got:\n%s", output)
	}
}

func TestFormatVersion_BuildDateHiddenWhenUnknown(t *testing.T) {
	savedBD := buildDate
	defer func() { buildDate = savedBD }()

	buildDate = "unknown"
	output := formatVersion()
	if strings.Contains(output, "Build date:") {
		t.Error("should not show Build date line when buildDate is 'unknown'")
	}
}

func TestClaudeAttribution_NotEmpty(t *testing.T) {
	if claudeAttribution == "" {
		t.Error("claudeAttribution constant should not be empty")
	}
	if !strings.Contains(claudeAttribution, "Claude") {
		t.Error("attribution should mention Claude")
	}
	if !strings.Contains(claudeAttribution, "Anthropic") {
		t.Error("attribution should mention Anthropic")
	}
}
