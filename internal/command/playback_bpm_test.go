// playback_bpm_test.go proves the "playback" command scope's route
// contract (03-06-PLAN.md Task 2): "playback bpm set" writes a valid
// numeric BPM to show.State.Tempo.BPM and saves, accepts re-setting the
// current value as an idempotent no-op, rejects a non-numeric or <= 0
// value with GOLC_PLAYBACK_BPM_INVALID (non-zero exit), and rejects a
// missing positional BPM argument with GOLC_PLAYBACK_USAGE (exit 2);
// "playback bpm tap" converts ordered --at tap timestamps into a
// persisted BPM and rejects fewer than two taps with
// GOLC_PLAYBACK_TAP_INVALID; show.Load/Save round-trips Tempo unchanged
// when no BPM route runs. Mirrors scene_test.go's seed-then-exercise-CLI-
// routes convention.
package command_test

import (
	"path/filepath"
	"testing"

	"github.com/lnorton89/golc/internal/command"
	"github.com/lnorton89/golc/internal/show"
	"github.com/stretchr/testify/require"
)

func TestBPMSetValidValue(t *testing.T) {
	root := t.TempDir()
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err)
	showPath := filepath.Join(t.TempDir(), "show.json")

	result := registry.Execute(command.Request{Root: root, Args: []string{
		"playback", "bpm", "set", "128", "--show", showPath,
	}})
	require.Equal(t, 0, result.ExitCode, "playback bpm set failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	require.Contains(t, string(result.Stdout), "GOLC_PLAYBACK_BPM_SET", "expected GOLC_PLAYBACK_BPM_SET in stdout, got %s", result.Stdout)

	state, err := show.Load(root, showPath)
	require.NoError(t, err)
	require.EqualValues(t, 128, state.Tempo.BPM, "expected Tempo.BPM=128, got %v", state.Tempo.BPM)
}

func TestBPMSetCurrentValueIsIdempotentNoOp(t *testing.T) {
	root := t.TempDir()
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err)
	showPath := filepath.Join(t.TempDir(), "show.json")

	first := registry.Execute(command.Request{Root: root, Args: []string{
		"playback", "bpm", "set", "128", "--show", showPath,
	}})
	require.Equal(t, 0, first.ExitCode, "first playback bpm set failed: exit=%d stderr=%s", first.ExitCode, first.Stderr)

	// Setting the exact same value again must be accepted, not rejected.
	second := registry.Execute(command.Request{Root: root, Args: []string{
		"playback", "bpm", "set", "128", "--show", showPath,
	}})
	require.Equal(t, 0, second.ExitCode, "second (idempotent) playback bpm set failed: exit=%d stderr=%s", second.ExitCode, second.Stderr)

	state, err := show.Load(root, showPath)
	require.NoError(t, err)
	require.EqualValues(t, 128, state.Tempo.BPM, "expected Tempo.BPM=128 after idempotent re-set, got %v", state.Tempo.BPM)
}

func TestBPMSetRejectsNonNumericValue(t *testing.T) {
	root := t.TempDir()
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err)
	showPath := filepath.Join(t.TempDir(), "show.json")

	result := registry.Execute(command.Request{Root: root, Args: []string{
		"playback", "bpm", "set", "not-a-number", "--show", showPath,
	}})
	require.NotEqual(t, 0, result.ExitCode)
	require.Contains(t, string(result.Stderr), "GOLC_PLAYBACK_BPM_INVALID", "expected non-zero exit with GOLC_PLAYBACK_BPM_INVALID for a non-numeric value, got exit=%d stderr=%s", result.ExitCode, result.Stderr)
}

func TestBPMSetRejectsNonPositiveValue(t *testing.T) {
	root := t.TempDir()
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err)
	showPath := filepath.Join(t.TempDir(), "show.json")

	result := registry.Execute(command.Request{Root: root, Args: []string{
		"playback", "bpm", "set", "0", "--show", showPath,
	}})
	require.NotEqual(t, 0, result.ExitCode)
	require.Contains(t, string(result.Stderr), "GOLC_PLAYBACK_BPM_INVALID", "expected non-zero exit with GOLC_PLAYBACK_BPM_INVALID for a <=0 value, got exit=%d stderr=%s", result.ExitCode, result.Stderr)
}

func TestBPMSetMissingArgumentUsageExitTwo(t *testing.T) {
	root := t.TempDir()
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err)
	showPath := filepath.Join(t.TempDir(), "show.json")

	result := registry.Execute(command.Request{Root: root, Args: []string{
		"playback", "bpm", "set", "--show", showPath,
	}})
	require.Equal(t, 2, result.ExitCode)
	require.Contains(t, string(result.Stderr), "GOLC_PLAYBACK_USAGE", "expected exit 2 GOLC_PLAYBACK_USAGE for a missing BPM argument, got exit=%d stderr=%s", result.ExitCode, result.Stderr)
}

func TestTapTempoRoutePersistsBPM(t *testing.T) {
	root := t.TempDir()
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err)
	showPath := filepath.Join(t.TempDir(), "show.json")

	// Three taps 0.5s apart -> 120 BPM.
	result := registry.Execute(command.Request{Root: root, Args: []string{
		"playback", "bpm", "tap",
		"--at", "2026-01-01T00:00:00Z",
		"--at", "2026-01-01T00:00:00.5Z",
		"--at", "2026-01-01T00:00:01Z",
		"--show", showPath,
	}})
	require.Equal(t, 0, result.ExitCode, "playback bpm tap failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	require.Contains(t, string(result.Stdout), "GOLC_PLAYBACK_BPM_TAP", "expected GOLC_PLAYBACK_BPM_TAP in stdout, got %s", result.Stdout)

	state, err := show.Load(root, showPath)
	require.NoError(t, err)
	if diff := state.Tempo.BPM - 120.0; diff < -1e-6 || diff > 1e-6 {
		require.GreaterOrEqual(t, diff, -1e-6)
		require.LessOrEqual(t, diff, 1e-6, "expected Tempo.BPM=120 from tap tempo, got %v", state.Tempo.BPM)
	}
}

func TestTapTempoRouteRejectsFewerThanTwoTaps(t *testing.T) {
	root := t.TempDir()
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err)
	showPath := filepath.Join(t.TempDir(), "show.json")

	result := registry.Execute(command.Request{Root: root, Args: []string{
		"playback", "bpm", "tap",
		"--at", "2026-01-01T00:00:00Z",
		"--show", showPath,
	}})
	require.NotEqual(t, 0, result.ExitCode)
	require.Contains(t, string(result.Stderr), "GOLC_PLAYBACK_TAP_INVALID", "expected non-zero exit with GOLC_PLAYBACK_TAP_INVALID for a single tap, got exit=%d stderr=%s", result.ExitCode, result.Stderr)

	// No BPM should have been persisted by the rejected tap call itself --
	// a fresh Load of this never-yet-saved show still reports exactly
	// show.DefaultBPM (its own seeded default, see internal/show/state.go),
	// never something else the tap route might have written.
	state, err := show.Load(root, showPath)
	require.NoError(t, err)
	require.EqualValues(t, show.DefaultBPM, state.Tempo.BPM, "expected Tempo.BPM to remain at show.DefaultBPM (%v) after a rejected tap route call, got %v", show.DefaultBPM, state.Tempo.BPM)
}

func TestTempoRoundTripsUnchangedWithoutBPMRoute(t *testing.T) {
	root := t.TempDir()
	showPath := filepath.Join(t.TempDir(), "show.json")

	seed, err := show.Load(root, showPath)
	require.NoError(t, err)
	seed.Tempo.BPM = 140
	if err := show.Save(root, showPath, seed); err != nil {
		require.NoError(t, err)
	}

	reloaded, err := show.Load(root, showPath)
	require.NoError(t, err)
	require.EqualValues(t, 140, reloaded.Tempo.BPM, "expected Tempo.BPM=140 to round-trip unchanged, got %v", reloaded.Tempo.BPM)
}
