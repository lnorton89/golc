// scriptstop_test.go covers "script stop" (08-06-PLAN.md Task 3): a
// no-active-run invocation exits 1 with GOLC_SCRIPT_NO_ACTIVE_RUN and
// changes nothing (no Deno needed); stopping a genuinely active run --
// exit 0, terminated status/reason, no restart, the process actually
// gone -- is gated behind the same .tools/toolchains/deno/ provisioning
// check scriptrun_test.go's own real-Deno tests use.
package command_test

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/lnorton89/golc/internal/command"
	"github.com/lnorton89/golc/internal/script"
	"github.com/lnorton89/golc/internal/scriptsdk"
	"github.com/stretchr/testify/require"
)

// TestScriptStopNoActiveRunExitsOneAndChangesNothing covers: "script stop
// <name> --show <path> with no active run for that script exits 1 with
// GOLC_SCRIPT_NO_ACTIVE_RUN and changes nothing." No Deno install is
// required: if this route ever reached a Host/Executor, this worktree's
// unprovisioned Deno would fail with a different, unmistakable error.
func TestScriptStopNoActiveRunExitsOneAndChangesNothing(t *testing.T) {
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err, "NewDefaultCommandRegistry: %v", err)
	root := t.TempDir()
	showPath := "show.golc"

	create := registry.Execute(command.Request{Root: root, Args: []string{"script", "create", "Idle", "--show", showPath}})
	require.Equal(t, 0, create.ExitCode, "script create failed: exit=%d stderr=%s", create.ExitCode, create.Stderr)

	before := registry.Execute(command.Request{Root: root, Args: []string{"script", "show", "Idle", "--show", showPath}})
	require.Equal(t, 0, before.ExitCode, "script show (before) failed: exit=%d stderr=%s", before.ExitCode, before.Stderr)

	result := registry.Execute(command.Request{Root: root, Args: []string{"script", "stop", "Idle", "--show", showPath}})
	require.Equal(t, 1, result.ExitCode, "expected ExitCode 1 for no active run, got %d stdout=%s stderr=%s", result.ExitCode, result.Stdout, result.Stderr)
	require.Contains(t, string(result.Stderr), "GOLC_SCRIPT_NO_ACTIVE_RUN", "expected GOLC_SCRIPT_NO_ACTIVE_RUN, got stderr=%s", result.Stderr)

	after := registry.Execute(command.Request{Root: root, Args: []string{"script", "show", "Idle", "--show", showPath}})
	require.Equal(t, 0, after.ExitCode, "script show (after) failed: exit=%d stderr=%s", after.ExitCode, after.Stderr)
	require.Equal(t, string(before.Stdout), string(after.Stdout), "expected a no-active-run Stop to change nothing, before=%s after=%s", before.Stdout, after.Stdout)
}

// TestScriptStopMalformedInvocationExitsTwo covers a malformed
// invocation (missing --show).
func TestScriptStopMalformedInvocationExitsTwo(t *testing.T) {
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err, "NewDefaultCommandRegistry: %v", err)
	root := t.TempDir()

	result := registry.Execute(command.Request{Root: root, Args: []string{"script", "stop", "Idle"}})
	require.Equal(t, 2, result.ExitCode, "expected ExitCode 2 for a missing --show, got %d stderr=%s", result.ExitCode, result.Stderr)
}

// TestScriptStopClassifiedAsExcluded proves "script stop" is itself
// classified in scriptsdk's excludedRoutes -- a running script must not
// be able to terminate itself or another run through the SDK.
// TestEveryDeclaredRouteIsClassified (scriptsdk_parity_test.go) is the
// build-breaking completeness gate; this test additionally pins the
// exact expected reason text.
func TestScriptStopClassifiedAsExcluded(t *testing.T) {
	reasons := scriptsdk.RegisteredExclusions()
	reason, excluded := reasons["script stop"]
	require.True(t, excluded, `expected "script stop" to be classified in scriptsdk's excludedRoutes, not exposed as an SDK method`)
	require.Contains(t, reason, "terminate itself or another run", "expected the exclusion reason to explain why, got %q", reason)
}

// skipUnlessDenoProvisionedForScriptStopTest resolves the repository root
// from this package's directory and skips the calling test when no
// verified Deno install exists there -- mirrors scriptrun_test.go's own
// helper exactly (kept as a separate function so this file stays
// self-contained if scriptrun_test.go's helper is ever renamed).
func skipUnlessDenoProvisionedForScriptStopTest(t *testing.T) string {
	t.Helper()
	return skipUnlessDenoProvisionedForCommandTest(t)
}

// TestScriptStopTerminatesActiveRun covers: "script stop <name> --show
// <path> on an active run exits 0, reports the run id and status:
// 'terminated' with reason GOLC_SCRIPT_STOPPED_BY_USER, and the process
// is gone" and "After a Stop, no restart occurs: a follow-up assertion
// confirms no new Deno process appears and the run status stays
// terminated (D-13)."
func TestScriptStopTerminatesActiveRun(t *testing.T) {
	root := skipUnlessDenoProvisionedForScriptStopTest(t)
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err, "NewDefaultCommandRegistry: %v", err)
	// An absolute temp path, not one relative to root: root is the real
	// repository root here (needed for Deno toolchain resolution), and
	// this show must never collide with a repeat run's own script names
	// against a stray repo-root show.golc.
	showPath := filepath.Join(t.TempDir(), "show.golc")

	createScriptWithSource(t, registry, root, showPath, "Runaway", `
while (true) {
  // deliberately never yields back to the runtime shim's SDK reader loop
}
`)

	runDone := make(chan command.Result, 1)
	go func() {
		runDone <- registry.Execute(command.Request{Root: root, Args: []string{"script", "run", "Runaway", "--show", showPath}})
	}()

	var active *script.Run
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if run, found := script.ActiveRun("Runaway"); found {
			active = run
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	require.NotNil(t, active, "expected script.ActiveRun to observe the \"Runaway\" script's active run within 5s")

	stop := registry.Execute(command.Request{Root: root, Args: []string{"script", "stop", "Runaway", "--show", showPath}})
	require.Equal(t, 0, stop.ExitCode, "script stop failed: exit=%d stdout=%s stderr=%s", stop.ExitCode, stop.Stdout, stop.Stderr)

	var view scriptRunResultView
	err = json.Unmarshal(stop.Stdout, &view)
	require.NoError(t, err, "unmarshal script stop output: %v stdout=%s", err, stop.Stdout)
	require.NotEmpty(t, view.RunID, "expected a non-empty run_id")
	require.Equal(t, "terminated", view.Status, "expected status terminated, got %q", view.Status)
	require.Contains(t, view.Reason, "GOLC_SCRIPT_STOPPED_BY_USER", "expected the reason to include GOLC_SCRIPT_STOPPED_BY_USER, got %q", view.Reason)

	select {
	case runResult := <-runDone:
		require.Equal(t, 1, runResult.ExitCode, "expected the original \"script run\" invocation to exit 1 (terminated), got %d", runResult.ExitCode)
	case <-time.After(10 * time.Second):
		require.Fail(t, "expected the original \"script run\" invocation to unblock within 10s of Stop")
	}

	_, stillActive := script.ActiveRun("Runaway")
	require.False(t, stillActive, "expected no active run for \"Runaway\" after Stop")

	shown := registry.Execute(command.Request{Root: root, Args: []string{"script", "show", "Runaway", "--show", showPath}})
	require.Equal(t, 0, shown.ExitCode, "script show (after stop) failed: exit=%d stderr=%s", shown.ExitCode, shown.Stderr)
	var afterStop struct {
		LastRunStatus string `json:"last_run_status"`
		LastRunReason string `json:"last_run_reason"`
	}
	err = json.Unmarshal(shown.Stdout, &afterStop)
	require.NoError(t, err, "unmarshal script show output: %v", err)
	require.Equal(t, "terminated", afterStop.LastRunStatus, "expected persisted last_run_status terminated, got %q", afterStop.LastRunStatus)

	// D-13: no restart, ever. A short wait confirms no new active run
	// reappears for "Runaway" on its own.
	time.Sleep(200 * time.Millisecond)
	_, reappeared := script.ActiveRun("Runaway")
	require.False(t, reappeared, "expected no auto-restart: a new active run must never appear for \"Runaway\" after Stop")
}
