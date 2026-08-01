// memorywatch_test.go covers internal/script/memorywatch.go
// (08-14-PLAN.md Task 1): startMemoryWatch's polling monitor against a
// fakeMemorySampler, entirely platform-independent -- no Deno, no real
// Windows Job Object. Every test in this file must run and pass on any
// GOOS with no toolchain provisioned; every "fires within Ns" assertion
// polls with a bounded timeout rather than a fixed sleep.
package script

import (
	"context"
	"errors"
	"sync"
	"testing"
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

// waitUntil polls cond every 5ms up to timeout, failing the test if cond
// never becomes true -- used instead of a fixed sleep for every "fires
// within Ns" assertion in this file.
func waitUntil(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	require.True(t, cond(), "condition not met within %s", timeout)
}

// TestMemoryWatchTerminatesAboveTrigger covers: "A sampler that returns a
// peak above the trigger causes exactly one beginTermination with the
// GOLC_SCRIPT_MEMORY_EXCEEDED reason, and run.terminationReason() reports
// it, within 2s."
func TestMemoryWatchTerminatesAboveTrigger(t *testing.T) {
	run := mustNewRun(t)
	sampler := &fakeMemorySampler{value: 64 * 1024 * 1024}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stop := startMemoryWatch(ctx, run, testMemoryWatchLimits(), sampler)
	defer stop()

	waitUntil(t, 2*time.Second, func() bool {
		_, terminating := run.terminationReason()
		return terminating
	})

	reason, terminating := run.terminationReason()
	require.True(t, terminating, "expected a GOLC_SCRIPT_MEMORY_EXCEEDED termination, got %+v (terminating=%v)", reason, terminating)
	require.Equal(t, "GOLC_SCRIPT_MEMORY_EXCEEDED", reason.Code, "expected a GOLC_SCRIPT_MEMORY_EXCEEDED termination, got %+v (terminating=%v)", reason, terminating)
}

// TestMemoryWatchNeverTerminatesBelowTrigger covers: "A sampler pinned
// below the trigger never records any termination; run.terminationReason()
// still reports false after 1s."
func TestMemoryWatchNeverTerminatesBelowTrigger(t *testing.T) {
	run := mustNewRun(t)
	sampler := &fakeMemorySampler{value: 1 * 1024 * 1024}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stop := startMemoryWatch(ctx, run, testMemoryWatchLimits(), sampler)
	defer stop()

	time.Sleep(1 * time.Second)
	_, terminating := run.terminationReason()
	require.False(t, terminating, "expected no termination for a sampler pinned well below the trigger")
}

// TestMemoryWatchStopsPermanentlyOnUnsupportedSentinel covers: "A sampler
// that returns errMemoryUsageUnsupported stops the loop permanently -- it
// is sampled at most twice and never records a termination (proves the
// non-Windows build spins no busy loop)."
func TestMemoryWatchStopsPermanentlyOnUnsupportedSentinel(t *testing.T) {
	run := mustNewRun(t)
	sampler := &fakeMemorySampler{err: errMemoryUsageUnsupported}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stop := startMemoryWatch(ctx, run, testMemoryWatchLimits(), sampler)
	defer stop()

	time.Sleep(500 * time.Millisecond)
	calls := sampler.callCount()
	require.LessOrEqual(t, calls, 2, "expected at most 2 sampler calls after the unsupported sentinel, got %d", calls)
	_, terminating := run.terminationReason()
	require.False(t, terminating, "expected no termination from the unsupported-platform sentinel")

	callsAfterSettle := sampler.callCount()
	time.Sleep(300 * time.Millisecond)
	require.Equal(t, callsAfterSettle, sampler.callCount(), "expected the loop to have already stopped permanently on the sentinel")
}

// TestMemoryWatchTransientErrorDoesNotStopTheLoop covers: "A sampler
// returning a transient non-sentinel error does NOT stop the loop: a
// later above-trigger sample from the same sampler still records the
// termination."
func TestMemoryWatchTransientErrorDoesNotStopTheLoop(t *testing.T) {
	run := mustNewRun(t)
	sampler := &fakeMemorySampler{err: errors.New("transient query failure")}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stop := startMemoryWatch(ctx, run, testMemoryWatchLimits(), sampler)
	defer stop()

	time.Sleep(250 * time.Millisecond)
	_, terminating := run.terminationReason()
	require.False(t, terminating, "expected no termination while the sampler only ever errors transiently")

	sampler.set(64*1024*1024, nil)

	waitUntil(t, 2*time.Second, func() bool {
		_, terminating := run.terminationReason()
		return terminating
	})
	reason, terminating := run.terminationReason()
	require.True(t, terminating, "expected termination once the transient sampler starts returning an above-trigger sample, got %+v (terminating=%v)", reason, terminating)
	require.Equal(t, "GOLC_SCRIPT_MEMORY_EXCEEDED", reason.Code, "expected termination once the transient sampler starts returning an above-trigger sample, got %+v (terminating=%v)", reason, terminating)
}

// TestMemoryWatchStopFunctionEndsTheGoroutine covers: "Calling the
// returned stop function ends the goroutine: no further sampler calls
// occur more than 1s after stop returns." Also proves stop is safe to
// call twice (sync.Once).
func TestMemoryWatchStopFunctionEndsTheGoroutine(t *testing.T) {
	run := mustNewRun(t)
	sampler := &fakeMemorySampler{value: 1 * 1024 * 1024}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stop := startMemoryWatch(ctx, run, testMemoryWatchLimits(), sampler)
	// Let it tick a couple of times first.
	time.Sleep(250 * time.Millisecond)
	stop()
	// A second call must be a no-op, never a panic.
	stop()

	callsAtStop := sampler.callCount()
	time.Sleep(1 * time.Second)
	got := sampler.callCount()
	require.Equal(t, callsAtStop, got, "expected no further sampler calls after stop, calls at stop = %d, now = %d", callsAtStop, got)
}

// TestMemoryWatchContextCancelEndsTheGoroutineTheSameWay covers:
// "Cancelling ctx ends the goroutine the same way."
func TestMemoryWatchContextCancelEndsTheGoroutineTheSameWay(t *testing.T) {
	run := mustNewRun(t)
	sampler := &fakeMemorySampler{value: 1 * 1024 * 1024}
	ctx, cancel := context.WithCancel(context.Background())

	stop := startMemoryWatch(ctx, run, testMemoryWatchLimits(), sampler)
	defer stop()
	time.Sleep(250 * time.Millisecond)
	cancel()

	callsAtCancel := sampler.callCount()
	time.Sleep(1 * time.Second)
	got := sampler.callCount()
	require.Equal(t, callsAtCancel, got, "expected no further sampler calls after context cancellation, calls at cancel = %d, now = %d", callsAtCancel, got)
}

// TestMemoryWatchNeverOverwritesAnAlreadyRecordedReason covers D-11
// first-writer-wins: "The monitor never records a second reason over an
// already-recorded one (pre-set the run's termination to a deadline
// reason, then run an above-trigger sampler; run.terminationReason()
// must still report the deadline reason)."
func TestMemoryWatchNeverOverwritesAnAlreadyRecordedReason(t *testing.T) {
	run := mustNewRun(t)
	deadlineReason := TerminationReason{Code: "GOLC_SCRIPT_DEADLINE_EXCEEDED", Message: "pre-set", At: time.Now()}
	require.True(t, run.beginTermination(deadlineReason), "expected the pre-set deadline reason to be recorded")

	sampler := &fakeMemorySampler{value: 64 * 1024 * 1024}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stop := startMemoryWatch(ctx, run, testMemoryWatchLimits(), sampler)
	defer stop()

	time.Sleep(500 * time.Millisecond)
	reason, terminating := run.terminationReason()
	require.True(t, terminating, "expected the pre-set deadline reason to survive, got %+v (terminating=%v)", reason, terminating)
	require.Equal(t, "GOLC_SCRIPT_DEADLINE_EXCEEDED", reason.Code, "expected the pre-set deadline reason to survive, got %+v (terminating=%v)", reason, terminating)
}
