//go:build windows

// memorylimit_windows_test.go covers 08-14-PLAN.md Task 2's real-OS proof
// of D-08's memory-limit resource cause: the mechanical
// QueryInformationJobObject round-trip against a real Job Object (no Deno
// required, runs on every bootstrapped Windows machine), plus the three
// real-Deno run-level proofs gated behind skipUnlessDenoProvisioned
// (session_test.go) -- exactly like jobobject_windows_test.go's own split
// between mechanical and adversarial-real-process coverage.
package script

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lnorton89/golc/internal/show"
	"github.com/stretchr/testify/require"
)

// TestJobObjectPeakMemoryReadsConfiguredLimit covers: "(*jobObject).
// peakMemoryBytes() on Windows, called on a fresh job with a live
// assigned child, returns a non-zero byte count and a nil error", "...
// called after Close() returns a non-nil error and never touches the
// released handle", and "the same query round-trips the configured
// ceiling: a job created with MemoryLimitMB: 64 reports JobMemoryLimit
// equal to exactly 64 * 1024 * 1024 bytes."
func TestJobObjectPeakMemoryReadsConfiguredLimit(t *testing.T) {
	limits := testResolvedLimits()
	limits.MemoryLimitMB = 64

	job, err := newJobObject(limits)
	require.NoError(t, err, "newJobObject: %v", err)

	configuredLimit, err := job.jobMemoryLimitBytes()
	require.NoError(t, err, "jobMemoryLimitBytes: %v", err)
	want := uint64(64) * 1024 * 1024
	require.Equal(t, want, configuredLimit, "jobMemoryLimitBytes() = %d, want %d (proves the query reads the same job newJobObject configured)", configuredLimit, want)

	cmd := spawnLongRunningProcess(t)
	assignErr := job.assign(uint32(cmd.Process.Pid))
	if assignErr != nil {
		_ = job.Close()
		_ = cmd.Process.Kill()
	}
	require.NoError(t, assignErr, "assign: %v", assignErr)

	peak, err := job.peakMemoryBytes()
	require.NoError(t, err, "peakMemoryBytes on a fresh job with a live assigned child: %v", err)
	require.NotZero(t, peak, "expected a non-zero peak memory usage for a live assigned child")

	closeErr := job.Close()
	if closeErr != nil {
		_ = cmd.Process.Kill()
	}
	require.NoError(t, closeErr, "Close: %v", closeErr)

	_, err = job.peakMemoryBytes()
	require.Error(t, err, "expected peakMemoryBytes to error after Close, never touching the released handle")
}

// memoryPressureScriptSource is a Deno script that pushes retained 2 MiB
// Uint8Array chunks into a module-scope array, .fill(1)s each so the
// pages are genuinely committed rather than merely reserved, and awaits
// a 20ms setTimeout per iteration -- deliberately slower than the 100ms
// monitor tick's own period so the proactive monitor gets multiple
// chances to observe the climb before the OS denies the commit that
// would cross the ceiling. The top-level `await run();` is load-bearing:
// session.go's runCompletionTrailerTS only reaches its own Deno.exit(0)
// once every top-level statement in the user's own source has settled,
// so a fire-and-forget `run();` (no top-level await) would let the
// trailer's own top-level await race ahead and exit the process before
// the loop ever climbs toward the ceiling.
//
// Observed empirically against a real, freshly-provisioned Deno 2.9.4 on
// this machine (08-14-PLAN.md Task 2's acceptance pass): which of two
// genuine V8-authored signals wins the race to actually end the process
// is nondeterministic run to run -- either the proactive monitor
// terminates the run first (job.Close() before V8 ever hits the denied
// commit), or V8 itself hits it first, and even then which of two V8 OOM
// shapes surfaces is nondeterministic: a catchable `RangeError: Array
// buffer allocation failed` from the explicit `new Uint8Array` call, or
// an uncatchable "Fatal JavaScript out of memory: MarkCompactCollector"
// engine abort from internal GC heap growth. classifyMemoryExhaustion's
// v8AllocationFailureSignatures list (capability.go) recognizes all of
// these; this test asserts only the final, user-observable outcome
// (Status/Reason), never which internal mechanism produced it -- exactly
// the "which one wins is not observable to the user" contract Task 2's
// session.go wiring documents.
const memoryPressureScriptSource = `
const chunks: Uint8Array[] = [];
async function run() {
  while (true) {
    const chunk = new Uint8Array(2 * 1024 * 1024);
    chunk.fill(1);
    chunks.push(chunk);
    await new Promise((r) => setTimeout(r, 20));
  }
}
await run();
`

// TestRunMemoryLimitTerminatesWithNamedReason covers: "Real Deno,
// Windows: a script with an advanced profile at MemoryLimitMB: 64
// running a retained allocating loop ends with RunOutcome.Status ==
// show.ScriptRunStatusTerminated and RunOutcome.Reason whose first line
// is exactly GOLC_SCRIPT_MEMORY_EXCEEDED: run exceeded its 64 MB memory
// limit. The reason contains no V8 stack-trace text."
func TestRunMemoryLimitTerminatesWithNamedReason(t *testing.T) {
	root := skipUnlessDenoProvisioned(t)

	host, err := NewHost(HostConfig{Root: root, ShowPath: filepath.Join(root, "fixture.golc"), Executor: &fakeExecutor{}})
	require.NoError(t, err, "NewHost: %v", err)

	target := show.Script{
		Name:   "MemoryPressure",
		Source: memoryPressureScriptSource,
		CapabilityProfile: show.CapabilityProfile{
			Scope:           show.APIKeyScopePlayback,
			Preset:          show.ResourcePresetAdvanced,
			MemoryLimitMB:   64,
			DeadlineSeconds: 20,
			RatePerSecond:   20,
			CPUCapPercent:   25,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	outcome, err := host.Run(ctx, target, LaunchModeRun, nil)
	require.NoError(t, err, "Run: %v", err)
	require.Equal(t, show.ScriptRunStatusTerminated, outcome.Status, "Status = %q, want %q (reason: %s)", outcome.Status, show.ScriptRunStatusTerminated, outcome.Reason)

	firstLine := strings.SplitN(outcome.Reason, "\n", 2)[0]
	wantFirstLine := "GOLC_SCRIPT_MEMORY_EXCEEDED: run exceeded its 64 MB memory limit"
	require.Equal(t, wantFirstLine, firstLine, "Reason first line = %q, want exactly %q (never a substring/prefix match)", firstLine, wantFirstLine)
	require.NotContains(t, outcome.Reason, "at file:", "expected no leaked V8 stack frame in the reason, got %q", outcome.Reason)
}

// TestRunWithinMemoryLimitSucceeds covers: "Real Deno, Windows: a
// trivial script under the default quick-action preset (256 MB) still
// ends with RunOutcome.Status == show.ScriptRunStatusSucceeded -- no
// false-positive memory termination." This is the false-positive guard
// for the 95% trigger.
func TestRunWithinMemoryLimitSucceeds(t *testing.T) {
	root := skipUnlessDenoProvisioned(t)

	host, err := NewHost(HostConfig{Root: root, ShowPath: filepath.Join(root, "fixture.golc"), Executor: &fakeExecutor{}})
	require.NoError(t, err, "NewHost: %v", err)

	target := show.Script{
		Name:   "TrivialQuickAction",
		Source: `console.log("hello");`,
		CapabilityProfile: show.CapabilityProfile{
			Scope:  show.APIKeyScopePlayback,
			Preset: show.ResourcePresetQuickAction,
		},
	}

	outcome, err := host.Run(context.Background(), target, LaunchModeRun, nil)
	require.NoError(t, err, "Run: %v", err)
	require.Equal(t, show.ScriptRunStatusSucceeded, outcome.Status, "Status = %q, want %q (reason: %s) -- a well-behaved script must never be falsely terminated", outcome.Status, show.ScriptRunStatusSucceeded, outcome.Reason)
}

// TestRunUnrelatedCrashStillReportsFailed covers: "Real Deno, Windows: a
// script that throws an ordinary uncaught error under a 256 MB limit
// still ends show.ScriptRunStatusFailed with its stack trace as the
// reason -- the classifier does not swallow genuine crashes."
func TestRunUnrelatedCrashStillReportsFailed(t *testing.T) {
	root := skipUnlessDenoProvisioned(t)

	host, err := NewHost(HostConfig{Root: root, ShowPath: filepath.Join(root, "fixture.golc"), Executor: &fakeExecutor{}})
	require.NoError(t, err, "NewHost: %v", err)

	target := show.Script{
		Name:   "OrdinaryCrash",
		Source: `throw new Error("deliberate ordinary failure");`,
		CapabilityProfile: show.CapabilityProfile{
			Scope:  show.APIKeyScopePlayback,
			Preset: show.ResourcePresetQuickAction,
		},
	}

	outcome, err := host.Run(context.Background(), target, LaunchModeRun, nil)
	require.NoError(t, err, "Run: %v", err)
	require.Equal(t, show.ScriptRunStatusFailed, outcome.Status, "Status = %q, want %q (reason: %s) -- the classifier must not swallow an unrelated crash", outcome.Status, show.ScriptRunStatusFailed, outcome.Reason)
	require.Contains(t, outcome.Reason, "deliberate ordinary failure", "expected the crash's own message in the reason, got %q", outcome.Reason)
}
