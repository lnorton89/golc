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
	"strings"
	"testing"

	"github.com/lnorton89/golc/internal/show"
)

func TestNewScript(t *testing.T) {
	s, err := show.NewScript("Chase Cycler")
	if err != nil {
		t.Fatalf("NewScript: %v", err)
	}
	var zero [16]byte
	if s.ID == zero {
		t.Fatalf("expected NewScript to mint a non-nil UUIDv7 ID")
	}
	if s.Name != "Chase Cycler" {
		t.Fatalf("expected Name %q, got %q", "Chase Cycler", s.Name)
	}
	if s.Source != "" {
		t.Fatalf("expected an empty Source, got %q", s.Source)
	}
	if s.CapabilityProfile.Preset != show.ResourcePresetQuickAction {
		t.Fatalf("expected the quick-action preset, got %q", s.CapabilityProfile.Preset)
	}
	if s.CapabilityProfile.Scope != show.APIKeyScopePlayback {
		t.Fatalf("expected the least-privileged playback scope, got %q", s.CapabilityProfile.Scope)
	}
	if s.LastRunStatus != show.ScriptRunStatusNeverRun {
		t.Fatalf("expected LastRunStatus never_run, got %q", s.LastRunStatus)
	}
}

func TestNewScriptMintsDistinctIDs(t *testing.T) {
	first, err := show.NewScript("Chase Cycler")
	if err != nil {
		t.Fatalf("NewScript (first): %v", err)
	}
	second, err := show.NewScript("Chase Cycler")
	if err != nil {
		t.Fatalf("NewScript (second): %v", err)
	}
	if first.ID == second.ID {
		t.Fatalf("expected two calls to NewScript to mint distinct IDs, both got %s", first.ID)
	}
}

func TestNewScriptRejectsEmptyName(t *testing.T) {
	if _, err := show.NewScript(""); err == nil || !strings.Contains(err.Error(), "GOLC_SCRIPT_NAME_EMPTY") {
		t.Fatalf("expected GOLC_SCRIPT_NAME_EMPTY for an empty name, got %v", err)
	}
	if _, err := show.NewScript("   "); err == nil || !strings.Contains(err.Error(), "GOLC_SCRIPT_NAME_EMPTY") {
		t.Fatalf("expected GOLC_SCRIPT_NAME_EMPTY for a whitespace-only name, got %v", err)
	}
}

func TestValidateScript(t *testing.T) {
	valid, err := show.NewScript("Chase Cycler")
	if err != nil {
		t.Fatalf("NewScript: %v", err)
	}
	if err := show.ValidateScript(valid); err != nil {
		t.Fatalf("expected a NewScript-constructed script to validate cleanly, got %v", err)
	}

	emptyName := valid
	emptyName.Name = ""
	if err := show.ValidateScript(emptyName); err == nil || !strings.Contains(err.Error(), "GOLC_SCRIPT_NAME_EMPTY") {
		t.Fatalf("expected GOLC_SCRIPT_NAME_EMPTY for an empty name, got %v", err)
	}

	whitespaceName := valid
	whitespaceName.Name = "   "
	if err := show.ValidateScript(whitespaceName); err == nil || !strings.Contains(err.Error(), "GOLC_SCRIPT_NAME_EMPTY") {
		t.Fatalf("expected GOLC_SCRIPT_NAME_EMPTY for a whitespace-only name, got %v", err)
	}

	invalidScope := valid
	invalidScope.CapabilityProfile.Scope = "bogus"
	if err := show.ValidateScript(invalidScope); err == nil || !strings.Contains(err.Error(), "GOLC_SCRIPT_SCOPE_INVALID") {
		t.Fatalf("expected GOLC_SCRIPT_SCOPE_INVALID for an invalid scope, got %v", err)
	}
}

func TestValidateScriptUniqueNames(t *testing.T) {
	if err := show.ValidateScriptUniqueNames(nil); err != nil {
		t.Fatalf("expected an empty slice to be accepted, got %v", err)
	}

	a, err := show.NewScript("Chase Cycler")
	if err != nil {
		t.Fatalf("NewScript: %v", err)
	}
	b, err := show.NewScript("Chase Cycler")
	if err != nil {
		t.Fatalf("NewScript: %v", err)
	}
	if err := show.ValidateScriptUniqueNames([]show.Script{a, b}); err == nil || !strings.Contains(err.Error(), "GOLC_SCRIPT_NAME_DUPLICATE") {
		t.Fatalf("expected GOLC_SCRIPT_NAME_DUPLICATE for two same-named scripts, got %v", err)
	}
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
	if limits.Deadline.Seconds() != 30 {
		t.Fatalf("expected quick-action deadline 30s, got %v", limits.Deadline)
	}
	if limits.RatePerSecond != 20 {
		t.Fatalf("expected quick-action rate 20/s, got %d", limits.RatePerSecond)
	}
	if limits.MemoryLimitMB != 256 {
		t.Fatalf("expected quick-action memory 256MB, got %d", limits.MemoryLimitMB)
	}
	if limits.CPUCapPercent != 25 {
		t.Fatalf("expected quick-action CPU cap 25%%, got %d", limits.CPUCapPercent)
	}
}

func TestResolveResourceLimitsLongRunning(t *testing.T) {
	profile := show.CapabilityProfile{Preset: show.ResourcePresetLongRunning}
	limits := profile.ResolveResourceLimits()
	if limits.Deadline.Seconds() != 3600 {
		t.Fatalf("expected long-running deadline 3600s, got %v", limits.Deadline)
	}
	if limits.RatePerSecond != 5 {
		t.Fatalf("expected long-running rate 5/s, got %d", limits.RatePerSecond)
	}
	if limits.MemoryLimitMB != 512 {
		t.Fatalf("expected long-running memory 512MB, got %d", limits.MemoryLimitMB)
	}
	if limits.CPUCapPercent != 25 {
		t.Fatalf("expected long-running CPU cap 25%%, got %d", limits.CPUCapPercent)
	}
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
			if limits.Deadline.Seconds() != 30 {
				t.Fatalf("expected the package default deadline 30s (not zero, not unlimited), got %v", limits.Deadline)
			}
			if limits.RatePerSecond != 20 {
				t.Fatalf("expected the package default rate 20/s, got %d", limits.RatePerSecond)
			}
			if limits.MemoryLimitMB != 256 {
				t.Fatalf("expected the package default memory 256MB, got %d", limits.MemoryLimitMB)
			}
			if limits.CPUCapPercent != 25 {
				t.Fatalf("expected the package default CPU cap 25%%, got %d", limits.CPUCapPercent)
			}
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
	if limits.Deadline.Seconds() != 120 {
		t.Fatalf("expected the explicit deadline 120s, got %v", limits.Deadline)
	}
	if limits.RatePerSecond != 7 {
		t.Fatalf("expected the explicit rate 7/s, got %d", limits.RatePerSecond)
	}
	if limits.MemoryLimitMB != 64 {
		t.Fatalf("expected the explicit memory 64MB, got %d", limits.MemoryLimitMB)
	}
	if limits.CPUCapPercent != 10 {
		t.Fatalf("expected the explicit CPU cap 10%%, got %d", limits.CPUCapPercent)
	}
}

// TestShowStateScriptValidation proves script.go's wiring into
// show.validate(): two same-named scripts fail Save, and a single script
// round-trips through the existing Save/Load path unchanged, including
// its Source bytes verbatim (no normalization/transpilation/reformatting
// at save time).
func TestShowStateScriptValidation(t *testing.T) {
	root := t.TempDir()

	a, err := show.NewScript("Chase Cycler")
	if err != nil {
		t.Fatalf("NewScript: %v", err)
	}
	b, err := show.NewScript("Chase Cycler")
	if err != nil {
		t.Fatalf("NewScript: %v", err)
	}
	dupState := show.State{Scripts: []show.Script{a, b}}
	if err := show.Save(root, "dup-scripts.golc", dupState); err == nil || !strings.Contains(err.Error(), "GOLC_SHOW_STATE_INVALID") {
		t.Fatalf("expected GOLC_SHOW_STATE_INVALID for duplicate script names, got %v", err)
	}

	source := "export function run() {\n  // deliberately preserved formatting\n\tconsole.log('hi');\n}\n"
	single, err := show.NewScript("Chase Cycler")
	if err != nil {
		t.Fatalf("NewScript: %v", err)
	}
	single.Source = source
	validState := show.State{Scripts: []show.Script{single}}
	if err := show.Save(root, "single-script.golc", validState); err != nil {
		t.Fatalf("expected a valid single script to save, got %v", err)
	}

	loaded, err := show.Load(root, "single-script.golc")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded.Scripts) != 1 {
		t.Fatalf("expected exactly one script to round-trip, got %d", len(loaded.Scripts))
	}
	if loaded.Scripts[0].ID != single.ID || loaded.Scripts[0].Name != single.Name {
		t.Fatalf("script identity did not round-trip: %+v", loaded.Scripts[0])
	}
	if loaded.Scripts[0].Source != source {
		t.Fatalf("expected Source to round-trip byte-for-byte:\nwant %q\ngot  %q", source, loaded.Scripts[0].Source)
	}
}
