// memorywatch_test.go covers internal/script/memorywatch.go
// (08-14-PLAN.md Task 1): startMemoryWatch's polling monitor against a
// fakeMemorySampler, entirely platform-independent -- no Deno, no real
// Windows Job Object. Every test in this file must run and pass on any
// GOOS with no toolchain provisioned.
//
// Every test here runs inside a testing/synctest bubble (stable in Go
// 1.26, this repo's pinned toolchain -- it graduated from GOEXPERIMENT).
// Inside a bubble the time package uses a per-bubble fake clock that
// advances only once every goroutine in the bubble is durably blocked,
// so `time.Sleep(2 * memoryWatchInterval)` below advances the monitor's
// ticker by exactly two ticks in zero wall-clock time. That is strictly
// stronger than the poll-with-a-bounded-timeout helper this file used
// before: an assertion that had to be slack-bounded against real
// scheduler jitter ("at most 2 sampler calls") is now exact ("exactly
// one"), and the file's runtime dropped from ~5.4s to milliseconds.
//
// startMemoryWatch is bubble-safe because everything it touches is
// in-bubble: a ticker, two channels, a context, and an injected fake
// sampler. It opens no socket, spawns no child process, and starts no
// goroutine outside the bubble -- the three things synctest forbids.
// (A *Run built by mustNewRun carries a nil bridge/job/cancel, so
// run.terminate() performs no I/O either.)
//
// synctest.Wait() blocks until every other goroutine in the bubble is
// durably blocked, so calling it before an assertion guarantees the
// monitor has finished reacting to the clock the test just advanced.
// No sampler-call count is ever read while the monitor is mid-sample --
// which is also why the pre-synctest race between "stop() returned" and
// "the monitor goroutine actually observed stopCh" is gone.
package script

import (
	"context"
	"errors"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/lnorton89/golc/internal/show"
	"github.com/stretchr/testify/require"
)

// fakeMemorySampler is a memorySampler whose returned value/error and
// call count are guarded by a mutex so the test goroutine and the
// monitor goroutine never race.
type fakeMemorySampler struct {
	mu    sync.Mutex
	calls int
	value uint64
	err   error
}

func (f *fakeMemorySampler) peakMemoryBytes() (uint64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return f.value, f.err
}

func (f *fakeMemorySampler) set(value uint64, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.value = value
	f.err = err
}

func (f *fakeMemorySampler) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func testMemoryWatchLimits() show.ResolvedLimits {
	return show.ResolvedLimits{MemoryLimitMB: 64}
}

// advance moves the bubble's fake clock forward by exactly n monitor
// ticks and then waits for the monitor goroutine to become durably
// blocked again, so every assertion after it observes a settled monitor
// rather than one mid-sample. This is the synctest replacement for this
// file's former waitUntil poller.
func advance(n int) {
	time.Sleep(time.Duration(n) * memoryWatchInterval)
	synctest.Wait()
}

// TestMemoryWatchTerminatesAboveTrigger covers: "A sampler that returns a
// peak above the trigger causes exactly one beginTermination with the
// GOLC_SCRIPT_MEMORY_EXCEEDED reason, and run.terminationReason() reports
// it." Under the fake clock the deadline is exact rather than a 2s
// bound: memoryPressureDebounceSamples consecutive above-trigger samples
// are required, so termination lands on precisely that tick.
func TestMemoryWatchTerminatesAboveTrigger(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		run := mustNewRun(t)
		sampler := &fakeMemorySampler{value: 64 * 1024 * 1024}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		stop := startMemoryWatch(ctx, run, testMemoryWatchLimits(), sampler)
		defer stop()

		// One tick short of the debounce threshold: still no termination.
		advance(memoryPressureDebounceSamples - 1)
		_, terminating := run.terminationReason()
		require.False(t, terminating, "expected no termination before %d consecutive above-trigger samples", memoryPressureDebounceSamples)

		advance(1)
		reason, terminating := run.terminationReason()
		require.True(t, terminating, "expected a GOLC_SCRIPT_MEMORY_EXCEEDED termination, got %+v (terminating=%v)", reason, terminating)
		require.Equal(t, "GOLC_SCRIPT_MEMORY_EXCEEDED", reason.Code, "expected a GOLC_SCRIPT_MEMORY_EXCEEDED termination, got %+v (terminating=%v)", reason, terminating)
	})
}

// TestMemoryWatchNeverTerminatesBelowTrigger covers: "A sampler pinned
// below the trigger never records any termination; run.terminationReason()
// still reports false after 1s" -- ten ticks of the fake clock.
func TestMemoryWatchNeverTerminatesBelowTrigger(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		run := mustNewRun(t)
		sampler := &fakeMemorySampler{value: 1 * 1024 * 1024}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		stop := startMemoryWatch(ctx, run, testMemoryWatchLimits(), sampler)
		defer stop()

		advance(10)
		require.Equal(t, 10, sampler.callCount(), "expected exactly one sample per tick")
		_, terminating := run.terminationReason()
		require.False(t, terminating, "expected no termination for a sampler pinned well below the trigger")
	})
}

// TestMemoryWatchStopsPermanentlyOnUnsupportedSentinel covers: "A sampler
// that returns errMemoryUsageUnsupported stops the loop permanently -- it
// is sampled at most twice and never records a termination (proves the
// non-Windows build spins no busy loop)." The fake clock tightens "at
// most twice" to its true contract: the sentinel is returned on the very
// first tick and the goroutine returns immediately, so it is sampled
// exactly once, ever.
func TestMemoryWatchStopsPermanentlyOnUnsupportedSentinel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		run := mustNewRun(t)
		sampler := &fakeMemorySampler{err: errMemoryUsageUnsupported}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		stop := startMemoryWatch(ctx, run, testMemoryWatchLimits(), sampler)
		defer stop()

		advance(1)
		require.Equal(t, 1, sampler.callCount(), "expected exactly one sampler call before the unsupported sentinel stopped the loop")
		_, terminating := run.terminationReason()
		require.False(t, terminating, "expected no termination from the unsupported-platform sentinel")

		// Ten further ticks must not restart it: the loop is stopped
		// permanently, not merely skipping a sample.
		advance(10)
		require.Equal(t, 1, sampler.callCount(), "expected the loop to have already stopped permanently on the sentinel")
	})
}

// TestMemoryWatchTransientErrorDoesNotStopTheLoop covers: "A sampler
// returning a transient non-sentinel error does NOT stop the loop: a
// later above-trigger sample from the same sampler still records the
// termination."
func TestMemoryWatchTransientErrorDoesNotStopTheLoop(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		run := mustNewRun(t)
		sampler := &fakeMemorySampler{err: errors.New("transient query failure")}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		stop := startMemoryWatch(ctx, run, testMemoryWatchLimits(), sampler)
		defer stop()

		advance(3)
		require.Equal(t, 3, sampler.callCount(), "expected the loop to keep sampling through transient errors")
		_, terminating := run.terminationReason()
		require.False(t, terminating, "expected no termination while the sampler only ever errors transiently")

		sampler.set(64*1024*1024, nil)

		advance(memoryPressureDebounceSamples)
		reason, terminating := run.terminationReason()
		require.True(t, terminating, "expected termination once the transient sampler starts returning an above-trigger sample, got %+v (terminating=%v)", reason, terminating)
		require.Equal(t, "GOLC_SCRIPT_MEMORY_EXCEEDED", reason.Code, "expected termination once the transient sampler starts returning an above-trigger sample, got %+v (terminating=%v)", reason, terminating)
	})
}

// TestMemoryWatchStopFunctionEndsTheGoroutine covers: "Calling the
// returned stop function ends the goroutine: no further sampler calls
// occur after stop returns." Also proves stop is safe to call twice
// (sync.Once). synctest.Wait() after stop() removes the pre-synctest
// race where callsAtStop could be read while the monitor was still
// inside a sample it had already begun.
func TestMemoryWatchStopFunctionEndsTheGoroutine(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		run := mustNewRun(t)
		sampler := &fakeMemorySampler{value: 1 * 1024 * 1024}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		stop := startMemoryWatch(ctx, run, testMemoryWatchLimits(), sampler)
		// Let it tick a couple of times first.
		advance(2)
		stop()
		// A second call must be a no-op, never a panic.
		stop()
		synctest.Wait()

		callsAtStop := sampler.callCount()
		require.Equal(t, 2, callsAtStop, "expected exactly one sample per tick before stop")

		advance(10)
		require.Equal(t, callsAtStop, sampler.callCount(), "expected no further sampler calls after stop")
	})
}

// TestMemoryWatchContextCancelEndsTheGoroutineTheSameWay covers:
// "Cancelling ctx ends the goroutine the same way."
func TestMemoryWatchContextCancelEndsTheGoroutineTheSameWay(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		run := mustNewRun(t)
		sampler := &fakeMemorySampler{value: 1 * 1024 * 1024}
		ctx, cancel := context.WithCancel(context.Background())

		stop := startMemoryWatch(ctx, run, testMemoryWatchLimits(), sampler)
		defer stop()
		advance(2)
		cancel()
		synctest.Wait()

		callsAtCancel := sampler.callCount()
		require.Equal(t, 2, callsAtCancel, "expected exactly one sample per tick before cancellation")

		advance(10)
		require.Equal(t, callsAtCancel, sampler.callCount(), "expected no further sampler calls after context cancellation")
	})
}

// TestMemoryWatchNeverOverwritesAnAlreadyRecordedReason covers D-11
// first-writer-wins: "The monitor never records a second reason over an
// already-recorded one (pre-set the run's termination to a deadline
// reason, then run an above-trigger sampler; run.terminationReason()
// must still report the deadline reason)."
func TestMemoryWatchNeverOverwritesAnAlreadyRecordedReason(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		run := mustNewRun(t)
		deadlineReason := TerminationReason{Code: "GOLC_SCRIPT_DEADLINE_EXCEEDED", Message: "pre-set", At: time.Now()}
		require.True(t, run.beginTermination(deadlineReason), "expected the pre-set deadline reason to be recorded")

		sampler := &fakeMemorySampler{value: 64 * 1024 * 1024}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		stop := startMemoryWatch(ctx, run, testMemoryWatchLimits(), sampler)
		defer stop()

		advance(memoryPressureDebounceSamples + 3)
		reason, terminating := run.terminationReason()
		require.True(t, terminating, "expected the pre-set deadline reason to survive, got %+v (terminating=%v)", reason, terminating)
		require.Equal(t, "GOLC_SCRIPT_DEADLINE_EXCEEDED", reason.Code, "expected the pre-set deadline reason to survive, got %+v (terminating=%v)", reason, terminating)
	})
}
