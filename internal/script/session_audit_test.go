// session_audit_test.go covers 08-08-PLAN.md Task 2's D-05 audit half:
// every dispatched SDK call fires both a script.outcome ScriptEvent (the
// live debug-panel half, Task 1) and an api.MutationEvent through
// api.PublishMutationEvent (Task 2's exported seam), which -- once an
// audit observer is registered for the show under test, exactly as a real
// "script run" invocation's own process would need to (mirrors
// internal/api/audit_test.go's own RegisterAuditObserver setup) -- writes
// exactly one audit_log row per call, in addition to the live event. This
// file lives in package script (white-box, matching session_test.go) so
// it can drive h.runDispatchIO directly against io.Pipe()-based fakes,
// exactly like session_test.go's own TestRunDispatchIOEndToEnd, without
// spawning a real Deno process.
package script

import (
	"io"
	"path/filepath"
	"testing"
	"time"

	"github.com/lnorton89/golc/internal/api"
	"github.com/lnorton89/golc/internal/show"
	"github.com/stretchr/testify/require"
)

// TestSDKCallProducesBothScriptOutcomeEventAndAuditRow is 08-08-PLAN.md
// Task 2's own required acceptance test: one dispatched SDK call must
// produce BOTH a script.outcome event (Task 1's live half) AND an
// audit_log row (Task 2's audit half) in the same run -- D-05's "in
// addition to the audit trail" clause, proven together rather than
// separately.
func TestSDKCallProducesBothScriptOutcomeEventAndAuditRow(t *testing.T) {
	resetScriptEventsForTest(t)
	api.ResetMutationObserversForTesting()
	t.Cleanup(api.ResetMutationObserversForTesting)

	root := t.TempDir()
	showPath := filepath.Join(root, "show.golc")
	api.RegisterAuditObserver(root, showPath)

	exec := &fakeExecutor{}
	h := &Host{cfg: HostConfig{Root: root, ShowPath: showPath, Executor: exec}}
	run := mustNewRun(t)
	run.ScriptName = "Chase"

	_, _, ch, unsubscribe := SubscribeScriptEvents(0)
	defer unsubscribe()

	stdoutR, stdoutW := io.Pipe()
	stdinR, stdinW := io.Pipe()
	stderrR, stderrW := io.Pipe()
	go func() {
		defer stdinR.Close()
		_, _ = io.Copy(io.Discard, stdinR)
	}()
	go func() { defer stderrW.Close() }()
	go func() {
		defer stdoutW.Close()
		_ = EncodeFrame(stdoutW, CmdCallFrame{ID: "c1", Method: "scene activate", Params: []byte(`{"name":"Alpha"}`)})
		_ = EncodeFrame(stdoutW, DoneFrame{ExitReason: "completed"})
	}()

	h.runDispatchIO(run, stdinW, stdoutR, stderrR)

	var sawOutcome bool
	select {
	case ev := <-ch:
		if ev.Kind == ScriptEventOutcome && ev.Route == "scene activate" && ev.Ok {
			sawOutcome = true
		}
	case <-time.After(2 * time.Second):
		require.Fail(t, "timed out waiting for the script.outcome event")
	}
	require.True(t, sawOutcome, "expected a script.outcome event for the dispatched SDK call")

	records, err := show.QueryAuditLog(root, showPath)
	require.NoError(t, err, "QueryAuditLog: %v", err)
	require.Len(t, records, 1, "expected exactly 1 audit_log row, got %d: %+v", len(records), records)
	rec := records[0]
	require.Equal(t, "script", rec.Source, "audit row Source = %q, want %q", rec.Source, "script")
	require.Equal(t, "scene activate", rec.Route, "audit row Route = %q, want %q", rec.Route, "scene activate")
	require.Equal(t, "success", rec.Outcome, "audit row Outcome = %q, want %q", rec.Outcome, "success")
	require.Equal(t, run.RunID.String(), rec.CorrelationID, "audit row CorrelationID = %q, want the run id %q", rec.CorrelationID, run.RunID.String())
	require.Equal(t, "script:Chase", rec.Actor, "audit row Actor = %q, want %q", rec.Actor, "script:Chase")
}
