// chase_motion_test.go proves the "chase"/"motion" command scopes' route
// contract (03-03-PLAN.md Task 3): "chase create" appends a new Chase and
// saves, rejecting a duplicate name through the existing
// GOLC_SHOW_STATE_INVALID wrapping diagnostic; "motion create" appends a
// new MotionPreset and saves; show.Load/Save round-trips a State carrying
// Chases and MotionPresets without loss. Mirrors theme_preset_test.go's
// seed-a-ShowState-directly-then-exercise-CLI-routes convention.
//
// The former "a hand-edited State with an over-scope motion capability
// fails Load" coverage now lives in
// internal/show/store_test.go's TestShowLoadRejectsOverScopeMotionCapability
// (05-01-PLAN.md Task 2 deviation): that test wrote raw JSON bytes
// directly to the show path to simulate a hand-edited document, a
// technique the Phase 5 SQLite-backed store's application_id door check
// now rejects before ever reaching validate() -- the equivalent
// direct-write simulation has to happen at the show_state blob-column
// level, which requires internal/show's unexported openStore.
package command_test

import (
	"path/filepath"
	"testing"

	"github.com/lnorton89/golc/internal/command"
	"github.com/lnorton89/golc/internal/fixture"
	"github.com/lnorton89/golc/internal/programming"
	"github.com/lnorton89/golc/internal/show"
	"github.com/stretchr/testify/require"
)

func TestChaseMotionRoutes(t *testing.T) {
	root := t.TempDir()
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err)

	showPath := filepath.Join(t.TempDir(), "show.json")

	chaseResult := registry.Execute(command.Request{Root: root, Args: []string{
		"chase", "create", "Sweep",
		"--unit", "bar",
		"--step-duration", "1",
		"--show", showPath,
	}})
	require.Equal(t, 0, chaseResult.ExitCode, "chase create failed: exit=%d stderr=%s", chaseResult.ExitCode, chaseResult.Stderr)

	reloaded, err := show.Load(root, showPath)
	require.NoError(t, err)
	require.Len(t, reloaded.Chases, 1)
	require.Equal(t, "Sweep", reloaded.Chases[0].Name, "expected exactly one persisted chase named Sweep, got %+v", reloaded.Chases)
	require.Equal(t, programming.StepUnitBar, reloaded.Chases[0].StepUnit)
	require.EqualValues(t, 1, reloaded.Chases[0].StepDuration, "unexpected chase step unit/duration: %+v", reloaded.Chases[0])

	duplicateChase := registry.Execute(command.Request{Root: root, Args: []string{
		"chase", "create", "Sweep",
		"--unit", "bar",
		"--step-duration", "1",
		"--show", showPath,
	}})
	require.NotEqual(t, 0, duplicateChase.ExitCode)
	require.Contains(t, string(duplicateChase.Stderr), "GOLC_CHASE_DUPLICATE_NAME", "expected GOLC_CHASE_DUPLICATE_NAME for a duplicate chase name, got exit=%d stderr=%s", duplicateChase.ExitCode, duplicateChase.Stderr)
	require.Contains(t, string(duplicateChase.Stderr), "GOLC_SHOW_STATE_INVALID", "expected the duplicate-name diagnostic to be wrapped in GOLC_SHOW_STATE_INVALID, got stderr=%s", duplicateChase.Stderr)

	motionResult := registry.Execute(command.Request{Root: root, Args: []string{
		"motion", "create", "Arc",
		"--show", showPath,
	}})
	require.Equal(t, 0, motionResult.ExitCode, "motion create failed: exit=%d stderr=%s", motionResult.ExitCode, motionResult.Stderr)

	afterMotion, err := show.Load(root, showPath)
	require.NoError(t, err)
	require.Len(t, afterMotion.MotionPresets, 1)
	require.Equal(t, "Arc", afterMotion.MotionPresets[0].Name, "expected exactly one persisted motion preset named Arc, got %+v", afterMotion.MotionPresets)

	duplicateMotion := registry.Execute(command.Request{Root: root, Args: []string{
		"motion", "create", "Arc",
		"--show", showPath,
	}})
	require.NotEqual(t, 0, duplicateMotion.ExitCode)
	require.Contains(t, string(duplicateMotion.Stderr), "GOLC_MOTION_PRESET_DUPLICATE_NAME", "expected GOLC_MOTION_PRESET_DUPLICATE_NAME for a duplicate motion preset name, got exit=%d stderr=%s", duplicateMotion.ExitCode, duplicateMotion.Stderr)
}

func TestChaseMotionChaseCreateMissingUsage(t *testing.T) {
	root := t.TempDir()
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err)
	showPath := filepath.Join(t.TempDir(), "show.json")

	result := registry.Execute(command.Request{Root: root, Args: []string{
		"chase", "create", "No Unit",
		"--step-duration", "1",
		"--show", showPath,
	}})
	require.Equal(t, 2, result.ExitCode)
	require.Contains(t, string(result.Stderr), "GOLC_CHASE_USAGE", "expected exit 2 GOLC_CHASE_USAGE for a missing --unit, got exit=%d stderr=%s", result.ExitCode, result.Stderr)
}

func TestChaseMotionShowStateRoundTrip(t *testing.T) {
	root := t.TempDir()
	path := "show.json"

	chase, err := programming.NewChase("Sweep", nil, programming.StepUnitBar, 1)
	require.NoError(t, err)
	motionPreset, err := programming.NewMotionPreset("Arc", []programming.MotionKeyframe{
		{Phase: 0, Values: []programming.MotionKeyframeValue{{Capability: fixture.CapabilityPan, Value: 0.5}}},
	})
	require.NoError(t, err)

	state := show.State{
		Chases:        []programming.Chase{chase},
		MotionPresets: []programming.MotionPreset{motionPreset},
	}
	if err := show.Save(root, path, state); err != nil {
		require.NoError(t, err)
	}

	reloaded, err := show.Load(root, path)
	require.NoError(t, err)
	require.Len(t, reloaded.Chases, 1)
	require.Equal(t, chase.ID, reloaded.Chases[0].ID)
	require.Equal(t, chase.Name, reloaded.Chases[0].Name, "chase did not round-trip: %+v", reloaded.Chases)
	require.Len(t, reloaded.MotionPresets, 1)
	require.Equal(t, motionPreset.ID, reloaded.MotionPresets[0].ID, "motion preset did not round-trip: %+v", reloaded.MotionPresets)
}
