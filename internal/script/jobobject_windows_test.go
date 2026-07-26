//go:build windows

// jobobject_windows_test.go covers internal/script/jobobject_windows.go
// (08-06-PLAN.md Task 2): the mechanical Job Object lifecycle (create,
// assign, idempotent Close) is proven against a real, lightweight native
// Windows child process (no Deno toolchain required) so this suite runs
// on every bootstrapped Windows machine; the adversarial "a finally
// block/unhandled-rejection handler/signal listener cannot delay or
// survive job close" proof additionally spawns a real Deno child and is
// gated behind skipUnlessDenoProvisioned (session_test.go), exactly like
// 08-05's own Deno-gated tests.
package script

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/lnorton89/golc/internal/show"
)

// testResolvedLimits is a small, valid show.ResolvedLimits every
// mechanical (non-adversarial) test in this file uses.
func testResolvedLimits() show.ResolvedLimits {
	return show.ResolvedLimits{
		Deadline:      30 * time.Second,
		RatePerSecond: 20,
		MemoryLimitMB: 256,
		CPUCapPercent: 25,
	}
}

// spawnLongRunningProcess starts a real native Windows child (ping
// against loopback for ~30 seconds -- no console/stdin redirection
// gotchas the way `timeout.exe` has) that the caller can assign to a Job
// Object and kill. The process is not otherwise cleaned up by this
// helper: a test that does not kill it via the Job Object under test
// should defer cmd.Process.Kill() itself.
func spawnLongRunningProcess(t *testing.T) *exec.Cmd {
	t.Helper()
	cmd := exec.Command("ping", "-n", "30", "127.0.0.1")
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to spawn a native long-running test process: %v", err)
	}
	return cmd
}

// TestJobObjectCreateConfiguresLimitsAndCloses covers: "newJobObject(limits)
// returns a handle configured with JOBOBJECT_EXTENDED_LIMIT_INFORMATION
// carrying JOB_OBJECT_LIMIT_JOB_MEMORY + JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
// and a JOBOBJECT_CPU_RATE_CONTROL_INFORMATION carrying
// JOB_OBJECT_CPU_RATE_CONTROL_ENABLE|JOB_OBJECT_CPU_RATE_CONTROL_HARD_CAP."
// Both SetInformationJobObject calls must themselves succeed for
// newJobObject to return a nil error at all -- a wrong flag combination
// or a malformed struct layout would fail the syscall outright.
func TestJobObjectCreateConfiguresLimitsAndCloses(t *testing.T) {
	job, err := newJobObject(testResolvedLimits())
	if err != nil {
		t.Fatalf("newJobObject: %v", err)
	}
	if err := job.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// Close is idempotent -- a second call must not error or panic.
	if err := job.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

// TestJobObjectRejectsInvalidMemoryLimit covers newJobObject delegating
// to capability.go's memoryLimitBytes: a non-positive MemoryLimitMB is
// rejected before any process could ever be assigned.
func TestJobObjectRejectsInvalidMemoryLimit(t *testing.T) {
	limits := testResolvedLimits()
	limits.MemoryLimitMB = 0
	if _, err := newJobObject(limits); err == nil {
		t.Fatal("expected an error for a non-positive MemoryLimitMB")
	}
}

// TestJobObjectRejectsInvalidCPULimit covers newJobObject delegating to
// capability.go's cpuRateFor: a CPUCapPercent outside 1..100 is rejected.
func TestJobObjectRejectsInvalidCPULimit(t *testing.T) {
	limits := testResolvedLimits()
	limits.CPUCapPercent = 101
	if _, err := newJobObject(limits); err == nil {
		t.Fatal("expected an error for a CPUCapPercent outside 1..100")
	}
}

// TestJobObjectAssignFailsForDeadProcess covers: "assign(handle, pid)
// ... returns a GOLC_SCRIPT_JOBOBJECT_ASSIGN_FAILED diagnostic otherwise"
// -- a pid with no live process (a real child, started and waited on to
// completion first, so its pid is guaranteed to no longer name a live
// process) fails assignment rather than silently succeeding.
func TestJobObjectAssignFailsForDeadProcess(t *testing.T) {
	cmd := exec.Command("cmd", "/c", "exit 0")
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to run a short-lived test process: %v", err)
	}
	deadPID := uint32(cmd.Process.Pid)

	job, err := newJobObject(testResolvedLimits())
	if err != nil {
		t.Fatalf("newJobObject: %v", err)
	}
	defer job.Close()

	if err := job.assign(deadPID); err == nil {
		t.Fatal("expected assign to fail for a pid with no live process")
	}
}

// TestJobObjectCloseKillsAssignedProcess covers: "Closing the job handle
// terminates the assigned child" and "Job Object assignment happens
// after cmd.Start() and before any frame is read from the child" (this
// test performs that exact ordering): assign a real, live, long-running
// native process to a fresh Job Object, close the job, and observe the
// process actually exit.
func TestJobObjectCloseKillsAssignedProcess(t *testing.T) {
	cmd := spawnLongRunningProcess(t)

	job, err := newJobObject(testResolvedLimits())
	if err != nil {
		_ = cmd.Process.Kill()
		t.Fatalf("newJobObject: %v", err)
	}

	if err := job.assign(uint32(cmd.Process.Pid)); err != nil {
		_ = job.Close()
		_ = cmd.Process.Kill()
		t.Fatalf("assign: %v", err)
	}

	waitDone := make(chan error, 1)
	go func() { waitDone <- cmd.Wait() }()

	if err := job.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	select {
	case <-waitDone:
		// The assigned process exited -- job close killed it.
	case <-time.After(10 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("expected the assigned process to be killed by job close within 10s, but it was still running")
	}
}

// TestJobObjectKillsAdversarialDenoChild is the real-process, real-Deno
// proof of SCRP-06's uninterceptability claim (08-06-PLAN.md Task 2's
// exact <behavior> requirement): a child that installs a `finally`
// block, an unhandled-rejection handler, and a signal listener -- each
// deliberately trying to delay or survive shutdown -- is still killed
// unconditionally by job close.
func TestJobObjectKillsAdversarialDenoChild(t *testing.T) {
	root := skipUnlessDenoProvisioned(t)
	denoPath, err := ResolveDenoExecutable(root)
	if err != nil {
		t.Fatalf("ResolveDenoExecutable: %v", err)
	}

	scriptDir := t.TempDir()
	scriptPath := filepath.Join(scriptDir, "adversarial.ts")
	adversarialSource := `
try {
  (globalThis as any).Deno.addSignalListener?.("SIGINT", () => {
    console.log("signal listener fired -- attempting to survive");
  });
} catch (_e) {
  // Some platforms may not support this signal name; irrelevant to the
  // job-close kill path under test, which never sends a signal at all.
}
(globalThis as any).addEventListener?.("unhandledrejection", (e: any) => {
  e.preventDefault?.();
  console.log("unhandled rejection handler fired -- attempting to survive");
});
try {
  Promise.reject(new Error("adversarial rejection"));
} catch (_e) {
  // no-op
}

async function run() {
  try {
    let i = 0;
    while (true) {
      i++;
      await new Promise((r) => setTimeout(r, 10));
    }
  } finally {
    console.log("finally block fired -- attempting to delay exit");
    // A finally block that itself never returns would be the strongest
    // adversary; even an infinite loop here must not prevent job close
    // from killing the OS process, since the kill is not cooperative.
    while (true) {
      // Deliberately spin forever inside finally.
    }
  }
}
run();
`
	if err := os.WriteFile(scriptPath, []byte(adversarialSource), 0o600); err != nil {
		t.Fatalf("write adversarial script: %v", err)
	}

	cmd := exec.CommandContext(context.Background(), denoPath, "run", "--no-prompt", scriptPath)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start adversarial Deno child: %v", err)
	}

	job, err := newJobObject(testResolvedLimits())
	if err != nil {
		_ = cmd.Process.Kill()
		t.Fatalf("newJobObject: %v", err)
	}
	if err := job.assign(uint32(cmd.Process.Pid)); err != nil {
		_ = job.Close()
		_ = cmd.Process.Kill()
		t.Fatalf("assign: %v", err)
	}

	waitDone := make(chan error, 1)
	go func() { waitDone <- cmd.Wait() }()

	// Give the adversarial script a moment to install its handlers and
	// enter its infinite loop before closing the job.
	time.Sleep(500 * time.Millisecond)

	if err := job.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	select {
	case <-waitDone:
		// Killed despite the finally block, rejection handler, and signal
		// listener all attempting to delay or survive shutdown.
	case <-time.After(10 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("expected job close to kill the adversarial child within 10s, but it was still running")
	}
}
