// session_test.go covers internal/script/session.go (08-05-PLAN.md
// Task 2): the frame-dispatch loop (runDispatchIO/dispatchCmdCall)
// against a fake Executor and io.Pipe()-based fakes for every behavior
// that does not require a real OS process, plus the single-active-run
// rejection and Run's real-process lifecycle. It is an internal
// (white-box) test package for the same reason host_test.go is: direct
// access to unexported fields (Host.running) and methods
// (runDispatchIO/dispatchCmdCall) is the point of these tests.
package script

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/security"
	"github.com/lnorton89/golc/internal/show"
)

// fakeExecutorCall records one Execute invocation.
type fakeExecutorCall struct {
	Route string
	Args  []string
	Root  string
}

// fakeExecutor is a test-only script.Executor recording every call it
// receives; result (when set) controls the returned outcome, defaulting
// to a successful "GOLC_OK" stdout line.
type fakeExecutor struct {
	mu     sync.Mutex
	calls  []fakeExecutorCall
	result func(route string, args []string) (exitCode int, stdout, stderr []byte)
}

func (f *fakeExecutor) Execute(route string, args []string, root string) (int, []byte, []byte) {
	f.mu.Lock()
	f.calls = append(f.calls, fakeExecutorCall{Route: route, Args: append([]string(nil), args...), Root: root})
	f.mu.Unlock()
	if f.result != nil {
		return f.result(route, args)
	}
	return 0, []byte("GOLC_OK\n"), nil
}

// mustNewRun builds a Run carrying an admin-scoped, quick-action-preset
// CapabilityProfile -- the widest scope and an ample default rate/
// deadline -- so these dispatch-mechanics tests never incidentally fail
// a D-06/D-09 capability check 08-06 introduced at the dispatchCmdCall
// seam this file exercises; capability_test.go covers scope/rate
// enforcement itself directly.
func mustNewRun(t *testing.T) *Run {
	t.Helper()
	id, err := uuid.NewV7()
	require.NoError(t, err, "uuid.NewV7: %v", err)
	return &Run{
		RunID:   id,
		Profile: show.CapabilityProfile{Scope: show.APIKeyScopeAdmin, Preset: show.ResourcePresetQuickAction},
	}
}

// TestDispatchCmdCallKnownMethod covers: "A run whose script calls
// golc.scene.activate({name:"Alpha"}) results in exactly one Execute call
// on the injected Executor with route "scene activate", and the child
// receives a cmd-result frame with Ok:true."
func TestDispatchCmdCallKnownMethod(t *testing.T) {
	exec := &fakeExecutor{}
	h := &Host{cfg: HostConfig{Root: "/repo", ShowPath: "/repo/show.golc", Executor: exec}}
	run := mustNewRun(t)

	result, outcome := h.dispatchCmdCall(run, CmdCallFrame{
		ID: "c1", Method: "scene activate", Params: []byte(`{"name":"Alpha"}`),
	})

	require.Len(t, exec.calls, 1, "expected exactly one Execute call, got %d: %+v", len(exec.calls), exec.calls)
	require.Equal(t, "scene activate", exec.calls[0].Route, "Execute route = %q, want %q", exec.calls[0].Route, "scene activate")
	require.True(t, result.Ok, "expected Ok:true, got %+v", result)
	require.Equal(t, "c1", result.ID, "result.ID = %q, want %q", result.ID, "c1")
	require.True(t, outcome.Ok && outcome.Route == "scene activate" && outcome.Method == "scene activate", "unexpected outcome: %+v", outcome)
}

// TestDispatchCmdCallUnknownMethod covers: "A cmd-call naming a method
// not in RegisteredSDKMethods() produces a cmd-result with Ok:false and
// code GOLC_SCRIPT_METHOD_UNKNOWN, and does not reach the Executor."
func TestDispatchCmdCallUnknownMethod(t *testing.T) {
	exec := &fakeExecutor{}
	h := &Host{cfg: HostConfig{Executor: exec}}
	run := mustNewRun(t)

	result, outcome := h.dispatchCmdCall(run, CmdCallFrame{ID: "c1", Method: "bogus method"})

	require.Empty(t, exec.calls, "expected zero Execute calls for an unknown method, got %d", len(exec.calls))
	require.False(t, result.Ok, "expected Ok:false for an unknown method")
	require.Equal(t, "GOLC_SCRIPT_METHOD_UNKNOWN", result.Code, "result.Code = %q, want GOLC_SCRIPT_METHOD_UNKNOWN", result.Code)
	require.True(t, outcome.Code == "GOLC_SCRIPT_METHOD_UNKNOWN" && !outcome.Ok, "unexpected outcome: %+v", outcome)
}

// TestDispatchCmdCallParamsInvalidNeverReachesExecutor proves a Params
// decode failure fails the same way an unknown method does: the Executor
// is never called.
func TestDispatchCmdCallParamsInvalidNeverReachesExecutor(t *testing.T) {
	exec := &fakeExecutor{}
	h := &Host{cfg: HostConfig{Root: "/repo", ShowPath: "/repo/show.golc", Executor: exec}}
	run := mustNewRun(t)

	result, outcome := h.dispatchCmdCall(run, CmdCallFrame{ID: "c1", Method: "scene activate", Params: []byte(`not json`)})

	require.Empty(t, exec.calls, "expected zero Execute calls for invalid Params, got %d", len(exec.calls))
	require.False(t, result.Ok || outcome.Ok, "expected Ok:false for invalid Params, got result=%+v outcome=%+v", result, outcome)
	require.Equal(t, "GOLC_SCRIPT_PARAMS_INVALID", outcome.Code, "outcome.Code = %q, want GOLC_SCRIPT_PARAMS_INVALID", outcome.Code)
}

// TestRunRejectsSecondActiveRun covers: "Starting a second run while one
// is active returns GOLC_SCRIPT_RUN_ACTIVE and does not spawn a process."
// Constructing Host directly (bypassing NewHost) with running already
// true proves Run returns before ever touching exec.Command -- denoPath
// is left empty and would fail immediately if Run ever tried to use it.
func TestRunRejectsSecondActiveRun(t *testing.T) {
	h := &Host{cfg: HostConfig{Root: t.TempDir()}, running: true}

	_, err := h.Run(context.Background(), show.Script{Name: "Chase"}, LaunchModeRun, nil)
	require.Error(t, err, "expected an error for a second run while one is active")
	require.Contains(t, err.Error(), "GOLC_SCRIPT_RUN_ACTIVE", "expected GOLC_SCRIPT_RUN_ACTIVE, got %v", err)
}

// TestRunDispatchIOEndToEnd drives runDispatchIO against an io.Pipe()
// pair playing the role of a well-behaved child: ready is ignored, a log
// line carrying security.CanaryToken reaches the run's captured log only
// in redacted form, a cmd-call dispatches to the fake Executor and is
// answered with exactly one cmd-result, and a trailing done frame sets
// the outcome's Reason. No real Deno process is spawned.
func TestRunDispatchIOEndToEnd(t *testing.T) {
	exec := &fakeExecutor{}
	h := &Host{cfg: HostConfig{Root: "/repo", ShowPath: "/repo/show.golc", Executor: exec}}
	run := mustNewRun(t)

	stdoutR, stdoutW := io.Pipe()
	stdinR, stdinW := io.Pipe()
	stderrR, stderrW := io.Pipe()

	go func() {
		defer stdinR.Close()
		_, _ = io.Copy(io.Discard, stdinR)
	}()
	go func() {
		defer stderrW.Close()
	}()
	go func() {
		defer stdoutW.Close()
		_ = EncodeFrame(stdoutW, ReadyFrame{})
		_ = EncodeFrame(stdoutW, LogFrame{Level: "info", Message: "leaked: " + security.CanaryToken})
		_ = EncodeFrame(stdoutW, CmdCallFrame{ID: "c1", Method: "scene activate", Params: []byte(`{"name":"Alpha"}`)})
		_ = EncodeFrame(stdoutW, DoneFrame{ExitReason: "completed"})
	}()

	outcome := h.runDispatchIO(run, stdinW, stdoutR, stderrR)

	require.NotEmpty(t, outcome.Logs, "expected at least one captured log line")
	for _, line := range outcome.Logs {
		require.NotContains(t, line.Message, security.CanaryToken, "expected the canary token to be redacted, got %q", line.Message)
	}
	require.True(t, len(outcome.Outcomes) == 1 && outcome.Outcomes[0].Ok && outcome.Outcomes[0].Route == "scene activate", "expected one successful scene activate call outcome, got %+v", outcome.Outcomes)
	require.Len(t, exec.calls, 1, "expected exactly one Execute call, got %d", len(exec.calls))
	require.Equal(t, "completed", outcome.Reason, "Reason = %q, want %q", outcome.Reason, "completed")
	require.Equal(t, show.ScriptRunStatusSucceeded, outcome.Status, "Status = %q, want %q", outcome.Status, show.ScriptRunStatusSucceeded)
}

// TestAppendBoundedLogDropsOldest covers T-08-20: captured output beyond
// the bounded buffer's capacity drops oldest entries rather than growing
// without bound.
func TestAppendBoundedLogDropsOldest(t *testing.T) {
	var lines []LogLine
	for i := 0; i < maxCapturedLogLines+5; i++ {
		lines = appendBoundedLog(lines, LogLine{Message: strings.Repeat("x", 1) + itoaForTest(i)})
	}
	require.Len(t, lines, maxCapturedLogLines, "len(lines) = %d, want %d", len(lines), maxCapturedLogLines)
	// The oldest 5 entries (index 0..4) must have been dropped; the first
	// remaining entry must be index 5.
	require.True(t, strings.HasSuffix(lines[0].Message, "5"), "expected the oldest entries to have been dropped, first remaining = %q", lines[0].Message)
}

func itoaForTest(i int) string {
	if i == 0 {
		return "0"
	}
	digits := ""
	for i > 0 {
		digits = string(rune('0'+i%10)) + digits
		i /= 10
	}
	return digits
}

// --- real-Deno-gated tests ---------------------------------------------

// projectRootForTest resolves the repository root from this package's
// directory (internal/script), so skipUnlessDenoProvisioned can look up a
// genuinely provisioned .tools/toolchains/deno/ install on a bootstrapped
// machine.
func projectRootForTest(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err, "resolve project root: %v", err)
	return root
}

// skipUnlessDenoProvisioned skips the calling test with a clear message
// when .tools/toolchains/deno/ is not provisioned on this machine
// (08-05-PLAN.md Task 2's explicit instruction), returning the project
// root otherwise.
func skipUnlessDenoProvisioned(t *testing.T) string {
	t.Helper()
	root := projectRootForTest(t)
	if _, err := ResolveDenoExecutable(root); err != nil {
		t.Skipf("Deno toolchain not provisioned under %s (%v); run 'mage Bootstrap' first", root, err)
	}
	return root
}

// TestRunRemovesTempDirOnSuccess covers: "Starting a run materializes the
// SDK runtime shim concatenated with the user's source into a single .ts
// file in a per-run temp directory created with os.MkdirTemp, and removes
// that directory when the run ends." A script with no SDK calls exits
// cleanly on its own.
func TestRunRemovesTempDirOnSuccess(t *testing.T) {
	root := skipUnlessDenoProvisioned(t)

	host, err := NewHost(HostConfig{Root: root, ShowPath: filepath.Join(root, "fixture.golc"), Executor: &fakeExecutor{}})
	require.NoError(t, err, "NewHost: %v", err)

	beforeEntries, _ := os.ReadDir(os.TempDir())

	outcome, err := host.Run(context.Background(), show.Script{Name: "Noop", Source: "// no SDK calls"}, LaunchModeRun, nil)
	require.NoError(t, err, "Run: %v", err)
	require.Equal(t, show.ScriptRunStatusSucceeded, outcome.Status, "Status = %q, want %q (reason: %s)", outcome.Status, show.ScriptRunStatusSucceeded, outcome.Reason)

	afterEntries, _ := os.ReadDir(os.TempDir())
	for _, entry := range afterEntries {
		if strings.HasPrefix(entry.Name(), "golc-script-run-") {
			found := false
			for _, before := range beforeEntries {
				if before.Name() == entry.Name() {
					found = true
					break
				}
			}
			require.True(t, found, "expected per-run temp directory %q to be removed after Run returns", entry.Name())
		}
	}
}

// TestRunTwoSequentialRunsMintDistinctRunIDs covers: "Two sequential runs
// of the same script produce two different RunID values."
func TestRunTwoSequentialRunsMintDistinctRunIDs(t *testing.T) {
	root := skipUnlessDenoProvisioned(t)

	host, err := NewHost(HostConfig{Root: root, ShowPath: filepath.Join(root, "fixture.golc"), Executor: &fakeExecutor{}})
	require.NoError(t, err, "NewHost: %v", err)

	first, err := host.Run(context.Background(), show.Script{Name: "Noop", Source: "// no SDK calls"}, LaunchModeRun, nil)
	require.NoError(t, err, "first Run: %v", err)
	second, err := host.Run(context.Background(), show.Script{Name: "Noop", Source: "// no SDK calls"}, LaunchModeRun, nil)
	require.NoError(t, err, "second Run: %v", err)
	require.NotEqual(t, first.RunID, second.RunID, "expected distinct RunIDs across sequential runs, got %s twice", first.RunID)
}

// TestRunSpawnsDenoWithNoAllowFlagsAndDispatchesSceneActivate is the
// real-process end-to-end proof of TestDispatchCmdCallKnownMethod: a
// genuine Deno child, spawned with buildDenoArgs' exact zero-permission
// command line, actually reaches the injected Executor.
func TestRunSpawnsDenoWithNoAllowFlagsAndDispatchesSceneActivate(t *testing.T) {
	root := skipUnlessDenoProvisioned(t)

	exec := &fakeExecutor{}
	host, err := NewHost(HostConfig{Root: root, ShowPath: filepath.Join(root, "fixture.golc"), Executor: exec})
	require.NoError(t, err, "NewHost: %v", err)

	source := `await golc.scene.activate({ name: "Alpha", show: "ignored" });`
	outcome, err := host.Run(context.Background(), show.Script{
		Name:   "ActivateAlpha",
		Source: source,
		// scene.activate requires the authoring scope (08-06's newly
		// enforced D-06 host-side scope check) -- 08-05's original zero-
		// value CapabilityProfile{} would now be denied at dispatch time.
		CapabilityProfile: show.CapabilityProfile{Scope: show.APIKeyScopeAuthoring, Preset: show.ResourcePresetQuickAction},
	}, LaunchModeRun, nil)
	require.NoError(t, err, "Run: %v", err)
	require.Equal(t, show.ScriptRunStatusSucceeded, outcome.Status, "Status = %q, want %q (reason: %s)", outcome.Status, show.ScriptRunStatusSucceeded, outcome.Reason)
	require.True(t, len(exec.calls) == 1 && exec.calls[0].Route == "scene activate", "expected exactly one 'scene activate' Execute call, got %+v", exec.calls)
}
