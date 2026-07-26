// host_test.go covers internal/script/host.go (08-05-PLAN.md Task 2):
// buildDenoArgs's exact Run-mode command line, the zero-permission
// guarantee TestDenoCommandLineHasNoAllowFlags asserts across every
// launch mode and capability profile, and NewHost's fail-closed
// GOLC_SCRIPT_DENO_MISSING behavior. It is an internal (white-box) test
// package so it can assert directly against buildDenoArgs and
// forbiddenDenoArgPrefixes -- the whole point of
// TestDenoCommandLineHasNoAllowFlags is that the assertion is derived
// from the same list buildDenoArgs is implicitly bound by, so the two can
// never silently drift apart.
package script

import (
	"strings"
	"testing"

	"github.com/lnorton89/golc/internal/show"
)

func TestBuildDenoArgsRunMode(t *testing.T) {
	got := buildDenoArgs("/tmp/run/script.ts", LaunchModeRun)
	want := []string{"run", "--no-prompt", "/tmp/run/script.ts"}
	if len(got) != len(want) {
		t.Fatalf("buildDenoArgs = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("buildDenoArgs = %v, want %v", got, want)
		}
	}
}

// TestDenoCommandLineHasNoAllowFlags asserts that for every launch mode
// and every capability profile in a table spanning all three
// show.APIKeyScope values and all three show.ResourcePreset values, no
// argument buildDenoArgs produces begins with any prefix in
// forbiddenDenoArgPrefixes (SCRP-03: zero Deno permission flags are ever
// passed for a script run). buildDenoArgs takes no profile parameter at
// all -- capability/scope assignment is enforced host-side (08-06), never
// encoded as a Deno permission grant (08-RESEARCH.md Pitfall 1) -- so this
// test also documents that the command line is identical regardless of
// profile, which is itself the property SCRP-03 requires.
func TestDenoCommandLineHasNoAllowFlags(t *testing.T) {
	scopes := []show.APIKeyScope{show.APIKeyScopePlayback, show.APIKeyScopeAuthoring, show.APIKeyScopeAdmin}
	presets := []show.ResourcePreset{show.ResourcePresetQuickAction, show.ResourcePresetLongRunning, show.ResourcePresetAdvanced}
	modes := []LaunchMode{LaunchModeRun, LaunchModeDebug}

	for _, scope := range scopes {
		for _, preset := range presets {
			profile := show.CapabilityProfile{Scope: scope, Preset: preset}
			for _, mode := range modes {
				t.Run(string(scope)+"/"+string(preset)+"/"+string(mode), func(t *testing.T) {
					_ = profile // profile intentionally does not influence buildDenoArgs; see doc comment.
					args := buildDenoArgs("/tmp/run/script.ts", mode)
					for _, arg := range args {
						for _, forbidden := range forbiddenDenoArgPrefixes {
							if strings.HasPrefix(arg, forbidden) {
								t.Fatalf("buildDenoArgs(mode=%s) produced forbidden argument %q (prefix %q)", mode, arg, forbidden)
							}
						}
					}
				})
			}
		}
	}
}

func TestNewHostFailsClosedWhenDenoMissing(t *testing.T) {
	root := t.TempDir()
	_, err := NewHost(HostConfig{Root: root})
	if err == nil {
		t.Fatal("expected an error when no Deno install exists")
	}
	if !strings.Contains(err.Error(), "GOLC_SCRIPT_DENO_MISSING") {
		t.Fatalf("expected GOLC_SCRIPT_DENO_MISSING, got %v", err)
	}
}
