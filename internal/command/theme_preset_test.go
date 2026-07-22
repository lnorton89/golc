// theme_preset_test.go proves the "theme"/"preset" command scopes' route
// contract (03-02-PLAN.md Task 2): "theme create" appends a named Theme
// and saves, rejecting a duplicate name through the existing
// GOLC_SHOW_STATE_INVALID wrapping diagnostic; "preset record" records a
// kind-filtered Preset from the persisted Programmer buffer and saves,
// rejecting a missing --kind at usage time; show.Load/Save round-trips a
// State carrying Themes and Presets without loss. Reuses
// seedProgrammerShowState from programming_test.go (same package).
package command_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/lnorton89/golc/internal/command"
	"github.com/lnorton89/golc/internal/programming"
	"github.com/lnorton89/golc/internal/show"
)

func TestThemePresetRoutes(t *testing.T) {
	root := t.TempDir()
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}

	showPath := filepath.Join(t.TempDir(), "show.json")
	instanceID := seedProgrammerShowState(t, root, showPath)

	// Populate the Programmer buffer with position-kind attributes (plus an
	// off-kind intensity attribute) so "preset record --kind position" has
	// something real to filter.
	setResult := registry.Execute(command.Request{Root: root, Args: []string{
		"programmer", "set",
		"--instance", instanceID.String(),
		"--attr", "pan=0.3",
		"--attr", "tilt=0.6",
		"--attr", "intensity=0.9",
		"--show", showPath,
	}})
	if setResult.ExitCode != 0 {
		t.Fatalf("programmer set failed: exit=%d stderr=%s", setResult.ExitCode, setResult.Stderr)
	}

	presetResult := registry.Execute(command.Request{Root: root, Args: []string{
		"preset", "record", "Center Stage",
		"--kind", "position",
		"--show", showPath,
	}})
	if presetResult.ExitCode != 0 {
		t.Fatalf("preset record failed: exit=%d stderr=%s", presetResult.ExitCode, presetResult.Stderr)
	}

	reloaded, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load after preset record: %v", err)
	}
	if len(reloaded.Presets) != 1 {
		t.Fatalf("expected exactly one persisted preset, got %+v", reloaded.Presets)
	}
	preset := reloaded.Presets[0]
	if preset.Name != "Center Stage" || preset.Kind != programming.PresetPosition {
		t.Fatalf("unexpected persisted preset identity: %+v", preset)
	}
	if len(preset.Attributes) != 2 {
		t.Fatalf("expected exactly 2 position attributes captured (off-kind intensity excluded), got %+v", preset.Attributes)
	}
	for _, attr := range preset.Attributes {
		if attr.Capability != "pan" && attr.Capability != "tilt" {
			t.Fatalf("expected only pan/tilt captured, got capability %q", attr.Capability)
		}
	}

	themeResult := registry.Execute(command.Request{Root: root, Args: []string{
		"theme", "create", "Sunset", "--show", showPath,
	}})
	if themeResult.ExitCode != 0 {
		t.Fatalf("theme create failed: exit=%d stderr=%s", themeResult.ExitCode, themeResult.Stderr)
	}

	afterTheme, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load after theme create: %v", err)
	}
	if len(afterTheme.Themes) != 1 || afterTheme.Themes[0].Name != "Sunset" {
		t.Fatalf("expected exactly one persisted theme named Sunset, got %+v", afterTheme.Themes)
	}

	duplicateTheme := registry.Execute(command.Request{Root: root, Args: []string{
		"theme", "create", "Sunset", "--show", showPath,
	}})
	if duplicateTheme.ExitCode == 0 || !strings.Contains(string(duplicateTheme.Stderr), "GOLC_THEME_DUPLICATE_NAME") {
		t.Fatalf("expected GOLC_THEME_DUPLICATE_NAME for a duplicate theme name, got exit=%d stderr=%s", duplicateTheme.ExitCode, duplicateTheme.Stderr)
	}
	if !strings.Contains(string(duplicateTheme.Stderr), "GOLC_SHOW_STATE_INVALID") {
		t.Fatalf("expected the duplicate-name diagnostic to be wrapped in GOLC_SHOW_STATE_INVALID, got stderr=%s", duplicateTheme.Stderr)
	}
}

func TestThemePresetPresetRecordMissingKindUsage(t *testing.T) {
	root := t.TempDir()
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}
	showPath := filepath.Join(t.TempDir(), "show.json")
	seedProgrammerShowState(t, root, showPath)

	result := registry.Execute(command.Request{Root: root, Args: []string{
		"preset", "record", "No Kind", "--show", showPath,
	}})
	if result.ExitCode != 2 || !strings.Contains(string(result.Stderr), "GOLC_PRESET_USAGE") {
		t.Fatalf("expected exit 2 GOLC_PRESET_USAGE for a missing --kind, got exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
}

func TestThemePresetPresetRecordInvalidKind(t *testing.T) {
	root := t.TempDir()
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}
	showPath := filepath.Join(t.TempDir(), "show.json")
	seedProgrammerShowState(t, root, showPath)

	result := registry.Execute(command.Request{Root: root, Args: []string{
		"preset", "record", "Bad Kind", "--kind", "laser", "--show", showPath,
	}})
	if result.ExitCode == 0 || !strings.Contains(string(result.Stderr), "GOLC_PRESET_KIND_INVALID") {
		t.Fatalf("expected GOLC_PRESET_KIND_INVALID for an unknown --kind, got exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
}

func TestThemePresetShowStateRoundTrip(t *testing.T) {
	root := t.TempDir()
	path := "show.json"

	theme, err := programming.NewTheme("Sunset")
	if err != nil {
		t.Fatalf("NewTheme: %v", err)
	}
	preset, err := programming.NewPreset("Full Wash", programming.PresetIntensity)
	if err != nil {
		t.Fatalf("NewPreset: %v", err)
	}

	state := show.State{
		Themes:  []programming.Theme{theme},
		Presets: []programming.Preset{preset},
	}
	if err := show.Save(root, path, state); err != nil {
		t.Fatalf("show.Save: %v", err)
	}

	reloaded, err := show.Load(root, path)
	if err != nil {
		t.Fatalf("show.Load: %v", err)
	}
	if len(reloaded.Themes) != 1 || reloaded.Themes[0].ID != theme.ID || reloaded.Themes[0].Name != theme.Name {
		t.Fatalf("theme did not round-trip: %+v", reloaded.Themes)
	}
	if len(reloaded.Presets) != 1 || reloaded.Presets[0].ID != preset.ID || reloaded.Presets[0].Kind != preset.Kind {
		t.Fatalf("preset did not round-trip: %+v", reloaded.Presets)
	}
}
