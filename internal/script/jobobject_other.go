//go:build !windows

// jobobject_other.go provides the non-Windows stand-in for
// jobobject_windows.go's kernel-enforced Job Object (08-06-PLAN.md
// Task 2): Windows is the only qualified and supported v1 platform
// (STATE.md), so this file exists only so the package still builds and
// tests cleanly for contributor CI on Linux/macOS. newJobObject and
// assign are no-ops (there is no real CPU/memory-cap or hard-kill
// mechanism to configure here); Close falls back to killing the
// assigned pid directly -- the same single-process kill
// internal/trace/transport/process.go's killProcessTree already uses on
// its own non-Windows branch, not a full descendant-tree walk (Deno
// children are never granted --allow-run, so they never legitimately
// have descendants of their own to walk).
package script

import (
	"os"
	"sync"

	"github.com/lnorton89/golc/internal/show"
)

// jobObject is the no-op stand-in used on every non-Windows GOOS.
type jobObject struct {
	mu     sync.Mutex
	closed bool
	pid    uint32
}

// newJobObject returns a jobObject carrying no real OS resource -- limits
// is accepted only so this function's signature matches
// jobobject_windows.go's exactly.
func newJobObject(limits show.ResolvedLimits) (*jobObject, error) {
	_ = limits
	return &jobObject{}, nil
}

// assign records pid for Close to kill later; it never fails.
func (j *jobObject) assign(pid uint32) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.pid = pid
	return nil
}

// peakMemoryBytes always returns errMemoryUsageUnsupported: Windows is
// the only qualified and supported v1 platform (STATE.md), so a non-
// Windows build has no real job accounting to read. This sentinel is
// what makes startMemoryWatch (memorywatch.go) stop its polling loop
// immediately here instead of polling forever against a platform that
// can never answer.
func (j *jobObject) peakMemoryBytes() (uint64, error) {
	return 0, errMemoryUsageUnsupported
}

// Close kills the process assign recorded, if any -- the process-tree
// kill fallback this platform relies on in place of a real Job Object.
// Idempotent: a second call is a no-op.
func (j *jobObject) Close() error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.closed {
		return nil
	}
	j.closed = true
	if j.pid == 0 {
		return nil
	}
	if proc, err := os.FindProcess(int(j.pid)); err == nil {
		_ = proc.Kill()
	}
	return nil
}
