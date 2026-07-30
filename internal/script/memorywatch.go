// memorywatch.go implements D-08's proactive resource-cause monitor
// (08-14-PLAN.md Task 1, CONTEXT SCRP-04/SCRP-05): a Job-Object-bound
// polling loop that observes a live run's own peak committed memory and
// records its termination through the same beginTermination/terminate
// pair (*Host).enforce already uses for a scope or rate violation, so a
// script that catches its own allocation failure and keeps thrashing at
// the ceiling is still terminated with the named GOLC_SCRIPT_MEMORY_
// EXCEEDED reason instead of surfacing a raw, uncaught V8 exception.
// This file carries no build tag -- it must compile on every GOOS so its
// tests run with a fake memorySampler, with no real Windows Job Object
// involved.
package script

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/lnorton89/golc/internal/show"
)

// memorySampler is the seam that makes startMemoryWatch fully testable
// on any GOOS with a fake, without a real Windows Job Object: *jobObject
// satisfies this interface on both platform builds (jobobject_windows.go's
// real PeakJobMemoryUsed query, jobobject_other.go's
// errMemoryUsageUnsupported stand-in).
type memorySampler interface {
	peakMemoryBytes() (uint64, error)
}

// errMemoryUsageUnsupported is the sentinel jobobject_other.go's
// peakMemoryBytes returns. It means "this platform cannot observe job
// memory usage at all" -- a permanent condition, not a transient failure
// -- and is the only error startMemoryWatch's loop treats as a reason to
// stop polling entirely, rather than skip one tick and continue.
var errMemoryUsageUnsupported = errors.New("GOLC_SCRIPT_MEMORY_USAGE_UNSUPPORTED: this platform cannot observe job memory usage")

// memoryWatchInterval is startMemoryWatch's poll period: tight enough
// that a run allocating in small chunks is caught before V8's own
// allocation failure surfaces, cheap enough that one read-only syscall
// per tick per active run is immaterial (at most one run is active
// process-wide, per 08-05's single-active-run scope call), and entirely
// off the playback/Art-Net path.
const memoryWatchInterval = 100 * time.Millisecond

// memoryPressureDebounceSamples is the number of consecutive
// above-trigger samples startMemoryWatch requires before terminating
// (WR-03, 08-14 gap closure): a single transient spike at
// memoryWatchInterval granularity should not be enough to kill a run
// that is about to finish successfully, but requiring this many
// consecutive samples (~200ms of sustained pressure at
// memoryWatchInterval) still leaves a generous safety margin ahead of
// the kernel's actual OOM denial, since memoryPressureTriggerPercent
// itself is already a proactive trigger below the real ceiling.
const memoryPressureDebounceSamples = 2

// startMemoryWatch starts one goroutine polling sampler at
// memoryWatchInterval and returns a stop function. On each tick it calls
// sampler.peakMemoryBytes(): an error satisfying errors.Is(err,
// errMemoryUsageUnsupported) stops the loop immediately and permanently
// (proves the non-Windows build spins no busy loop); any other error
// skips this tick and continues (a transient query failure must never
// disable supervision); a successful sample is passed to
// checkMemoryPressure and, when it returns non-nil for
// memoryPressureDebounceSamples consecutive ticks in a row (WR-03: a
// single transient spike never terminates a run on its own; any tick
// that samples back below the trigger resets the streak), this calls
// run.beginTermination(*reason) then run.terminate() -- the identical
// two-call pattern (*Host).enforce already uses -- and returns.
// beginTermination's first-writer-wins semantics (D-11) mean a deadline,
// rate, scope, or user Stop that already began termination is never
// overwritten by this monitor. The monitor deliberately never closes the
// job handle or cancels the context itself -- terminate() owns that
// ordering. The returned stop function is guarded by sync.Once so a
// caller may defer it and a second call is a no-op; cancelling ctx ends
// the goroutine the same way.
func startMemoryWatch(ctx context.Context, run *Run, limits show.ResolvedLimits, sampler memorySampler) func() {
	stopCh := make(chan struct{})
	var stopOnce sync.Once
	stop := func() {
		stopOnce.Do(func() { close(stopCh) })
	}

	go func() {
		ticker := time.NewTicker(memoryWatchInterval)
		defer ticker.Stop()
		var aboveTriggerStreak int
		for {
			select {
			case <-ctx.Done():
				return
			case <-stopCh:
				return
			case <-ticker.C:
				peak, err := sampler.peakMemoryBytes()
				if err != nil {
					if errors.Is(err, errMemoryUsageUnsupported) {
						return
					}
					continue
				}
				reason := checkMemoryPressure(peak, limits)
				if reason == nil {
					aboveTriggerStreak = 0
					continue
				}
				aboveTriggerStreak++
				if aboveTriggerStreak < memoryPressureDebounceSamples {
					continue
				}
				run.beginTermination(*reason)
				run.terminate()
				return
			}
		}
	}()

	return stop
}
