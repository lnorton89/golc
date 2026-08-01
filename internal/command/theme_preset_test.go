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
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/command"
	"github.com/lnorton89/golc/internal/programming"
	"github.com/lnorton89/golc/internal/show"
)

func TestThemePresetRoutes(t *testing.T) {
	root := t.TempDir()
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err, "NewDefaultCommandRegistry: %v", err)

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
	require.Equal(t, 0, setResult.ExitCode, "programmer set failed: exit=%d stderr=%s", setResult.ExitCode, setResult.Stderr)

	presetResult := registry.Execute(command.Request{Root: root, Args: []string{
		"preset", "record", "Center Stage",
		"--kind", "position",
		"--show", showPath,
	}})
	require.Equal(t, 0, presetResult.ExitCode, "preset record failed: exit=%d stderr=%s", presetResult.ExitCode, presetResult.Stderr)

	reloaded, err := show.Load(root, showPath)
	require.NoError(t, err, "show.Load after preset record: %v", err)
	require.Len(t, reloaded.Presets, 1, "expected exactly one persisted preset, got %+v", reloaded.Presets)
	preset := reloaded.Presets[0]
	require.Equal(t, "Center Stage", preset.Name, "unexpected persisted preset identity: %+v", preset)
	require.Equal(t, programming.PresetPosition, preset.Kind, "unexpected persisted preset identity: %+v", preset)
	require.Len(t, preset.Attributes, 2, "expected exactly 2 position attributes captured (off-kind intensity excluded), got %+v", preset.Attributes)
	for _, attr := range preset.Attributes {
		require.True(t, attr.Capability == "pan" || attr.Capability == "tilt", "expected only pan/tilt captured, got capability %q", attr.Capability)
	}

	themeResult := registry.Execute(command.Request{Root: root, Args: []string{
		"theme", "create", "Sunset", "--show", showPath,
	}})
	require.Equal(t, 0, themeResult.ExitCode, "theme create failed: exit=%d stderr=%s", themeResult.ExitCode, themeResult.Stderr)

	afterTheme, err := show.Load(root, showPath)
	require.NoError(t, err, "show.Load after theme create: %v", err)
	require.Len(t, afterTheme.Themes, 1, "expected exactly one persisted theme named Sunset, got %+v", afterTheme.Themes)
	require.Equal(t, "Sunset", afterTheme.Themes[0].Name, "expected exactly one persisted theme named Sunset, got %+v", afterTheme.Themes)

	duplicateTheme := registry.Execute(command.Request{Root: root, Args: []string{
		"theme", "create", "Sunset", "--show", showPath,
	}})
	require.NotEqual(t, 0, duplicateTheme.ExitCode, "expected GOLC_THEME_DUPLICATE_NAME for a duplicate theme name, got exit=%d stderr=%s", duplicateTheme.ExitCode, duplicateTheme.Stderr)
	require.Contains(t, string(duplicateTheme.Stderr), "GOLC_THEME_DUPLICATE_NAME", "expected GOLC_THEME_DUPLICATE_NAME for a duplicate theme name, got exit=%d stderr=%s", duplicateTheme.ExitCode, duplicateTheme.Stderr)
	require.Contains(t, string(duplicateTheme.Stderr), "GOLC_SHOW_STATE_INVALID", "expected the duplicate-name diagnostic to be wrapped in GOLC_SHOW_STATE_INVALID, got stderr=%s", duplicateTheme.Stderr)
}

func TestThemePresetPresetRecordMissingKindUsage(t *testing.T) {
	root := t.TempDir()
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err, "NewDefaultCommandRegistry: %v", err)
	showPath := filepath.Join(t.TempDir(), "show.json")
	seedProgrammerShowState(t, root, showPath)

	result := registry.Execute(command.Request{Root: root, Args: []string{
		"preset", "record", "No Kind", "--show", showPath,
	}})
	require.Equal(t, 2, result.ExitCode, "expected exit 2 GOLC_PRESET_USAGE for a missing --kind, got exit=%d stderr=%s", result.ExitCode, result.Stderr)
	require.Contains(t, string(result.Stderr), "GOLC_PRESET_USAGE", "expected exit 2 GOLC_PRESET_USAGE for a missing --kind, got exit=%d stderr=%s", result.ExitCode, result.Stderr)
}

func TestThemePresetPresetRecordInvalidKind(t *testing.T) {
	root := t.TempDir()
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err, "NewDefaultCommandRegistry: %v", err)
	showPath := filepath.Join(t.TempDir(), "show.json")
	seedProgrammerShowState(t, root, showPath)

	result := registry.Execute(command.Request{Root: root, Args: []string{
		"preset", "record", "Bad Kind", "--kind", "laser", "--show", showPath,
	}})
	require.NotEqual(t, 0, result.ExitCode, "expected GOLC_PRESET_KIND_INVALID for an unknown --kind, got exit=%d stderr=%s", result.ExitCode, result.Stderr)
	require.Contains(t, string(result.Stderr), "GOLC_PRESET_KIND_INVALID", "expected GOLC_PRESET_KIND_INVALID for an unknown --kind, got exit=%d stderr=%s", result.ExitCode, result.Stderr)
}

func TestThemePresetShowStateRoundTrip(t *testing.T) {
	root := t.TempDir()
	path := "show.json"

	theme, err := programming.NewTheme("Sunset")
	require.NoError(t, err, "NewTheme: %v", err)
	preset, err := programming.NewPreset("Full Wash", programming.PresetIntensity)
	require.NoError(t, err, "NewPreset: %v", err)

	state := show.State{
		Themes:  []programming.Theme{theme},
		Presets: []programming.Preset{preset},
	}
	err = show.Save(root, path, state)
	require.NoError(t, err, "show.Save: %v", err)

	reloaded, err := show.Load(root, path)
	require.NoError(t, err, "show.Load: %v", err)
	require.Len(t, reloaded.Themes, 1, "theme did not round-trip: %+v", reloaded.Themes)
	require.Equal(t, theme.ID, reloaded.Themes[0].ID, "theme did not round-trip: %+v", reloaded.Themes)
	require.Equal(t, theme.Name, reloaded.Themes[0].Name, "theme did not round-trip: %+v", reloaded.Themes)
	require.Len(t, reloaded.Presets, 1, "preset did not round-trip: %+v", reloaded.Presets)
	require.Equal(t, preset.ID, reloaded.Presets[0].ID, "preset did not round-trip: %+v", reloaded.Presets)
	require.Equal(t, preset.Kind, reloaded.Presets[0].Kind, "preset did not round-trip: %+v", reloaded.Presets)
}
