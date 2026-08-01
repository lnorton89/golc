// scripts_test.go pins the Script/CapabilityProfile domain contract
// (08-01-PLAN.md Task 1, RED state) before internal/show/scripts.go's
// behavior is trusted: NewScript mints a stable, unique UUIDv7 identity
// and a least-privileged quick-action default profile; ValidateScript and
// ValidateScriptUniqueNames reject every declared invalid shape;
// ResolveResourceLimits resolves every preset deterministically, with a
// zero/negative/absent advanced-preset field always falling back to the
// package safe default (D-09, never "unlimited"); and a State carrying
// scripts round-trips through the existing Save/Load path with Source
// preserved byte-for-byte. This file is package show_test (external),
// exercising only exported package show API.
package show_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/show"
)

func TestNewScript(t *testing.T) {
	s, err := show.NewScript("Chase Cycler")
	require.NoError(t, err, "NewScript")
	var zero [16]byte
	require.NotEqual(t, zero, s.ID, "expected NewScript to mint a non-nil UUIDv7 ID")
	require.Equal(t, "Chase Cycler", s.Name)
	require.Empty(t, s.Source, "expected an empty Source")
	require.Equal(t, show.ResourcePresetQuickAction, s.CapabilityProfile.Preset, "expected the quick-action preset")
	require.Equal(t, show.APIKeyScopePlayback, s.CapabilityProfile.Scope, "expected the least-privileged playback scope")
	require.Equal(t, show.ScriptRunStatusNeverRun, s.LastRunStatus)
}

func TestNewScriptMintsDistinctIDs(t *testing.T) {
	first, err := show.NewScript("Chase Cycler")
	require.NoError(t, err, "NewScript (first)")
	second, err := show.NewScript("Chase Cycler")
	require.NoError(t, err, "NewScript (second)")
	require.NotEqual(t, first.ID, second.ID, "expected two calls to NewScript to mint distinct IDs")
}

func TestNewScriptRejectsEmptyName(t *testing.T) {
	_, err := show.NewScript("")
	require.ErrorContains(t, err, "GOLC_SCRIPT_NAME_EMPTY", "expected error for an empty name")
	_, err = show.NewScript("   ")
	require.ErrorContains(t, err, "GOLC_SCRIPT_NAME_EMPTY", "expected error for a whitespace-only name")
}

func TestValidateScript(t *testing.T) {
	valid, err := show.NewScript("Chase Cycler")
	require.NoError(t, err, "NewScript")
	require.NoError(t, show.ValidateScript(valid), "expected a NewScript-constructed script to validate cleanly")

	emptyName := valid
	emptyName.Name = ""
	err = show.ValidateScript(emptyName)
	require.ErrorContains(t, err, "GOLC_SCRIPT_NAME_EMPTY", "expected error for an empty name")

	whitespaceName := valid
	whitespaceName.Name = "   "
	err = show.ValidateScript(whitespaceName)
	require.ErrorContains(t, err, "GOLC_SCRIPT_NAME_EMPTY", "expected error for a whitespace-only name")

	invalidScope := valid
	invalidScope.CapabilityProfile.Scope = "bogus"
	err = show.ValidateScript(invalidScope)
	require.ErrorContains(t, err, "GOLC_SCRIPT_SCOPE_INVALID", "expected error for an invalid scope")
}

func TestValidateScriptUniqueNames(t *testing.T) {
	require.NoError(t, show.ValidateScriptUniqueNames(nil), "expected an empty slice to be accepted")

	a, err := show.NewScript("Chase Cycler")
	require.NoError(t, err, "NewScript")
	b, err := show.NewScript("Chase Cycler")
	require.NoError(t, err, "NewScript")
	err = show.ValidateScriptUniqueNames([]show.Script{a, b})
	require.ErrorContains(t, err, "GOLC_SCRIPT_NAME_DUPLICATE", "expected error for two same-named scripts")
}

func TestResolveResourceLimitsQuickAction(t *testing.T) {
	profile := show.CapabilityProfile{
		Preset: show.ResourcePresetQuickAction,
		// Custom fields set on a quick-action profile must be ignored
		// entirely -- the preset always resolves to its own fixed values.
		DeadlineSeconds: 999999,
		RatePerSecond:   999999,
		MemoryLimitMB:   999999,
		CPUCapPercent:   999999,
	}
	limits := profile.ResolveResourceLimits()
	require.Equal(t, float64(30), limits.Deadline.Seconds(), "expected quick-action deadline 30s")
	require.Equal(t, 20, limits.RatePerSecond, "expected quick-action rate 20/s")
	require.Equal(t, 256, limits.MemoryLimitMB, "expected quick-action memory 256MB")
	require.Equal(t, 25, limits.CPUCapPercent, "expected quick-action CPU cap 25%%")
}

func TestResolveResourceLimitsLongRunning(t *testing.T) {
	profile := show.CapabilityProfile{Preset: show.ResourcePresetLongRunning}
	limits := profile.ResolveResourceLimits()
	require.Equal(t, float64(3600), limits.Deadline.Seconds(), "expected long-running deadline 3600s")
	require.Equal(t, 5, limits.RatePerSecond, "expected long-running rate 5/s")
	require.Equal(t, 512, limits.MemoryLimitMB, "expected long-running memory 512MB")
	require.Equal(t, 25, limits.CPUCapPercent, "expected long-running CPU cap 25%%")
}

func TestResolveResourceLimitsAdvancedFallsBackToSafeDefaults(t *testing.T) {
	cases := []struct {
		name            string
		deadlineSeconds int
		ratePerSecond   int
		memoryLimitMB   int
		cpuCapPercent   int
	}{
		{name: "zero", deadlineSeconds: 0, ratePerSecond: 0, memoryLimitMB: 0, cpuCapPercent: 0},
		{name: "negative", deadlineSeconds: -1, ratePerSecond: -1, memoryLimitMB: -1, cpuCapPercent: -1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			profile := show.CapabilityProfile{
				Preset:          show.ResourcePresetAdvanced,
				DeadlineSeconds: tc.deadlineSeconds,
				RatePerSecond:   tc.ratePerSecond,
				MemoryLimitMB:   tc.memoryLimitMB,
				CPUCapPercent:   tc.cpuCapPercent,
			}
			limits := profile.ResolveResourceLimits()
			require.Equal(t, float64(30), limits.Deadline.Seconds(), "expected the package default deadline 30s (not zero, not unlimited)")
			require.Equal(t, 20, limits.RatePerSecond, "expected the package default rate 20/s")
			require.Equal(t, 256, limits.MemoryLimitMB, "expected the package default memory 256MB")
			require.Equal(t, 25, limits.CPUCapPercent, "expected the package default CPU cap 25%%")
		})
	}
}

func TestResolveResourceLimitsAdvancedHonorsExplicitValues(t *testing.T) {
	profile := show.CapabilityProfile{
		Preset:          show.ResourcePresetAdvanced,
		DeadlineSeconds: 120,
		RatePerSecond:   7,
		MemoryLimitMB:   64,
		CPUCapPercent:   10,
	}
	limits := profile.ResolveResourceLimits()
	require.Equal(t, float64(120), limits.Deadline.Seconds(), "expected the explicit deadline 120s")
	require.Equal(t, 7, limits.RatePerSecond, "expected the explicit rate 7/s")
	require.Equal(t, 64, limits.MemoryLimitMB, "expected the explicit memory 64MB")
	require.Equal(t, 10, limits.CPUCapPercent, "expected the explicit CPU cap 10%%")
}

// TestShowStateScriptValidation proves script.go's wiring into
// show.validate(): two same-named scripts fail Save, and a single script
// round-trips through the existing Save/Load path unchanged, including
// its Source bytes verbatim (no normalization/transpilation/reformatting
// at save time).
func TestShowStateScriptValidation(t *testing.T) {
	root := t.TempDir()

	a, err := show.NewScript("Chase Cycler")
	require.NoError(t, err, "NewScript")
	b, err := show.NewScript("Chase Cycler")
	require.NoError(t, err, "NewScript")
	dupState := show.State{Scripts: []show.Script{a, b}}
	err = show.Save(root, "dup-scripts.golc", dupState)
	require.ErrorContains(t, err, "GOLC_SHOW_STATE_INVALID", "expected error for duplicate script names")

	source := "export function run() {\n  // deliberately preserved formatting\n\tconsole.log('hi');\n}\n"
	single, err := show.NewScript("Chase Cycler")
	require.NoError(t, err, "NewScript")
	single.Source = source
	validState := show.State{Scripts: []show.Script{single}}
	require.NoError(t, show.Save(root, "single-script.golc", validState), "expected a valid single script to save")

	loaded, err := show.Load(root, "single-script.golc")
	require.NoError(t, err, "Load")
	require.Len(t, loaded.Scripts, 1, "expected exactly one script to round-trip")
	require.Equal(t, single.ID, loaded.Scripts[0].ID, "script identity did not round-trip: %+v", loaded.Scripts[0])
	require.Equal(t, single.Name, loaded.Scripts[0].Name, "script identity did not round-trip: %+v", loaded.Scripts[0])
	require.Equal(t, source, loaded.Scripts[0].Source, "expected Source to round-trip byte-for-byte")
}
