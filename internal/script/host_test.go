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
	"github.com/stretchr/testify/require"
)

func TestBuildDenoArgsRunMode(t *testing.T) {
	got := buildDenoArgs("/tmp/run/script.ts", LaunchModeRun, 0)
	want := []string{"run", "--no-prompt", "/tmp/run/script.ts"}
	require.Equal(t, want, got, "buildDenoArgs = %v, want %v", got, want)
}

// TestBuildDenoArgsDebugMode covers 08-09-PLAN.md Task 1's exact
// <behavior> bullet: LaunchModeDebug produces exactly one inspector
// argument, bound to a loopback address and the exact non-zero port
// passed in, positioned before the script path (Deno requires flags
// ahead of the script argument), and still no permission-granting flag.
func TestBuildDenoArgsDebugMode(t *testing.T) {
	got := buildDenoArgs("/tmp/run/script.ts", LaunchModeDebug, 54321)
	want := []string{"run", "--no-prompt", "--inspect-brk=127.0.0.1:54321", "/tmp/run/script.ts"}
	require.Equal(t, want, got, "buildDenoArgs = %v, want %v", got, want)
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
// profile, which is itself the property SCRP-03 requires. It additionally
// asserts Run mode never produces an inspector argument, and Debug mode
// always produces exactly one, for every profile in the table
// (08-09-PLAN.md Task 1's explicit extension of this test).
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
					args := buildDenoArgs("/tmp/run/script.ts", mode, 12345)
					inspectorArgs := 0
					for _, arg := range args {
						for _, forbidden := range forbiddenDenoArgPrefixes {
							require.False(t, strings.HasPrefix(arg, forbidden), "buildDenoArgs(mode=%s) produced forbidden argument %q (prefix %q)", mode, arg, forbidden)
						}
						if strings.HasPrefix(arg, "--inspect") {
							inspectorArgs++
						}
					}
					switch mode {
					case LaunchModeRun:
						require.Equal(t, 0, inspectorArgs, "buildDenoArgs(mode=%s) produced %d inspector argument(s), want 0", mode, inspectorArgs)
					case LaunchModeDebug:
						require.Equal(t, 1, inspectorArgs, "buildDenoArgs(mode=%s) produced %d inspector argument(s), want exactly 1", mode, inspectorArgs)
					}
				})
			}
		}
	}
}

// TestPickEphemeralLoopbackPort covers 08-09-PLAN.md Task 1's exact
// <behavior> bullet: the returned port is non-zero, and two consecutive
// calls do not collide.
func TestPickEphemeralLoopbackPort(t *testing.T) {
	first, err := pickEphemeralLoopbackPort()
	require.NoError(t, err, "pickEphemeralLoopbackPort: %v", err)
	require.NotZero(t, first, "expected a non-zero ephemeral port")
	second, err := pickEphemeralLoopbackPort()
	require.NoError(t, err, "pickEphemeralLoopbackPort: %v", err)
	require.NotZero(t, second, "expected a non-zero ephemeral port")
	require.NotEqual(t, first, second, "expected two consecutive calls not to collide, both returned %d", first)
}

func TestNewHostFailsClosedWhenDenoMissing(t *testing.T) {
	root := t.TempDir()
	_, err := NewHost(HostConfig{Root: root})
	require.Error(t, err, "expected an error when no Deno install exists")
	require.Contains(t, err.Error(), "GOLC_SCRIPT_DENO_MISSING", "expected GOLC_SCRIPT_DENO_MISSING, got %v", err)
}

// TestNoInspectorOutsideDebugMode is the real-process proof behind D-02:
// a genuine Run-mode script's captured stdout/stderr never contains
// Deno's own "Debugger listening on ws://..." banner -- the exact text
// Deno prints only when --inspect/--inspect-brk was passed. Enumerating
// a child process's actual open listening sockets from a Go test is
// platform-specific and far more fragile than this direct textual proof:
// Deno emits that banner unconditionally whenever an inspector is
// active, so its absence is a reliable, portable witness that no
// inspector server exists for this run, exactly matching
// TestDenoCommandLineHasNoAllowFlags's structural proof that Run mode's
// command line never carries the flag that would have started one.
func TestNoInspectorOutsideDebugMode(t *testing.T) {
	root := skipUnlessDenoProvisioned(t)

	host, err := NewHost(HostConfig{Root: root, ShowPath: root + "/fixture.golc", Executor: &fakeExecutor{}})
	require.NoError(t, err, "NewHost: %v", err)

	outcome, runErr := host.Run(t.Context(), show.Script{Name: "NoInspector", Source: "// no SDK calls"}, LaunchModeRun, nil)
	require.NoError(t, runErr, "Run: %v", runErr)
	require.Equal(t, show.ScriptRunStatusSucceeded, outcome.Status, "Status = %q, want %q (reason: %s)", outcome.Status, show.ScriptRunStatusSucceeded, outcome.Reason)
	for _, line := range outcome.Logs {
		require.False(t, strings.Contains(line.Message, "Debugger listening") || strings.Contains(line.Message, "ws://"), "Run mode produced an inspector banner in captured output: %q", line.Message)
	}
}
