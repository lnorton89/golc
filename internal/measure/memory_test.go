// memory_test.go covers internal/measure/memory.go against an injected
// fake TreeSampler -- no real process tree, so every case runs and passes
// on any GOOS with nothing provisioned.
//
// Every PeakWatcher case runs inside a testing/synctest bubble (stable in
// Go 1.26, this repo's pinned toolchain). The watcher is bubble-safe by
// construction: a ticker, a stop channel, a context, and an injected
// sampler, with no socket, no child process, and no goroutine started
// outside the bubble. That means "poll ten times and check the peak" costs
// zero wall-clock time and asserts an EXACT sample count rather than a
// jitter-tolerant range -- the same reason internal/script's
// memorywatch_test.go was converted.
//
// The one test that must NOT be bubbled is called out in place.
package measure

import (
	"context"
	"errors"
	"math"
	"os"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
)

// fakeTreeSampler is a TreeSampler whose returned tree and error are
// swappable mid-test, with a mutex so the watcher goroutine and the test
// goroutine never race.
type fakeTreeSampler struct {
	mu    sync.Mutex
	calls int
	tree  TreeMemory
	err   error
}

func (f *fakeTreeSampler) SampleTree(int32) (TreeMemory, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return f.tree, f.err
}

func (f *fakeTreeSampler) set(tree TreeMemory, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tree = tree
	f.err = err
}

func (f *fakeTreeSampler) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

// treeOf builds a TreeMemory whose total is the sum of the given
// per-process resident sizes, root first.
func treeOf(sizes ...uint64) TreeMemory {
	tree := TreeMemory{Root: ProcessMemory{PID: 1, Name: "golc-desktop", RSSBytes: sizes[0]}}
	tree.TotalRSSBytes = sizes[0]
	for i, size := range sizes[1:] {
		tree.Descendants = append(tree.Descendants, ProcessMemory{
			PID: int32(100 + i), Name: "child", RSSBytes: size, Depth: 1,
		})
		tree.TotalRSSBytes += size
	}
	return tree
}

const testPollInterval = 100 * time.Millisecond

// advance moves the bubble's fake clock forward by exactly n poll
// intervals and waits for the watcher goroutine to settle, so every
// assertion after it observes a watcher between samples rather than
// mid-sample.
func advance(n int) {
	time.Sleep(time.Duration(n) * testPollInterval)
	synctest.Wait()
}

func TestNewPeakWatcherRejectsNonPositiveInterval(t *testing.T) {
	for _, interval := range []time.Duration{0, -time.Second} {
		_, err := NewPeakWatcher(&fakeTreeSampler{}, 1, interval)
		require.ErrorContains(t, err, "GOLC_MEASURE_INTERVAL_INVALID", "interval=%s", interval)
	}
}

func TestNewPeakWatcherDefaultsToTheSystemSampler(t *testing.T) {
	w, err := NewPeakWatcher(nil, 1, time.Second)
	require.NoError(t, err, "NewPeakWatcher")
	require.IsType(t, SystemSampler{}, w.sampler,
		"a nil sampler must default to the real one so production callers need only pass a pid")
}

// TestPeakWatcherRetainsTheHighestTotalNotTheLatest is the watcher's whole
// reason to exist: a soak's memory evidence is the peak, and a later,
// smaller sample must never overwrite it.
func TestPeakWatcherRetainsTheHighestTotalNotTheLatest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sampler := &fakeTreeSampler{tree: treeOf(100, 50)} // total 150
		w, err := NewPeakWatcher(sampler, 1, testPollInterval)
		require.NoError(t, err, "NewPeakWatcher")

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		stop := w.Start(ctx)
		defer stop()

		advance(1)
		require.Equal(t, uint64(150), w.Summary().PeakTotalRSSBytes)

		sampler.set(treeOf(400, 200), nil) // total 600 -- the peak
		advance(1)
		require.Equal(t, uint64(600), w.Summary().PeakTotalRSSBytes)

		sampler.set(treeOf(10, 10), nil) // total 20 -- must not lower the peak
		advance(3)

		summary := w.Summary()
		require.Equal(t, uint64(600), summary.PeakTotalRSSBytes, "a later smaller sample must not lower the peak")
		require.Equal(t, 5, summary.Samples, "expected exactly one sample per tick")
		require.Zero(t, summary.Failures)
		require.Equal(t, uint64(400), summary.PeakAt.Root.RSSBytes,
			"PeakAt must retain the whole tree that produced the peak, so a report can name the culprit")
		require.Len(t, summary.PeakAt.Descendants, 1)
	})
}

// TestPeakWatcherKeepsPollingThroughSampleFailures covers the soak
// contract: a transient read failure must be counted, not fatal -- memory
// evidence must not silently stop halfway through a multi-hour run.
func TestPeakWatcherKeepsPollingThroughSampleFailures(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sampler := &fakeTreeSampler{err: errors.New("transient read failure")}
		w, err := NewPeakWatcher(sampler, 1, testPollInterval)
		require.NoError(t, err, "NewPeakWatcher")

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		stop := w.Start(ctx)
		defer stop()

		advance(3)
		summary := w.Summary()
		require.Equal(t, 3, summary.Samples)
		require.Equal(t, 3, summary.Failures)
		require.Contains(t, summary.LastErr, "transient read failure")
		require.Zero(t, summary.PeakTotalRSSBytes)

		// Recovery: the loop is still alive and records the peak.
		sampler.set(treeOf(700), nil)
		advance(2)

		summary = w.Summary()
		require.Equal(t, 5, summary.Samples)
		require.Equal(t, 3, summary.Failures, "earlier failures stay counted")
		require.Equal(t, uint64(700), summary.PeakTotalRSSBytes,
			"the loop must have survived the failures to record this")
	})
}

// TestPeakWatcherStopEndsTheGoroutine covers stop's idempotence and the
// no-further-samples guarantee. synctest.Wait() after stop() removes the
// window where a call count could be read while a sample was in flight.
func TestPeakWatcherStopEndsTheGoroutine(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sampler := &fakeTreeSampler{tree: treeOf(10)}
		w, err := NewPeakWatcher(sampler, 1, testPollInterval)
		require.NoError(t, err, "NewPeakWatcher")

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		stop := w.Start(ctx)

		advance(2)
		stop()
		stop() // a second call must be a no-op, never a panic
		synctest.Wait()

		callsAtStop := sampler.callCount()
		require.Equal(t, 2, callsAtStop)

		advance(10)
		require.Equal(t, callsAtStop, sampler.callCount(), "no further samples after stop")
	})
}

// TestPeakWatcherContextCancelEndsTheGoroutineTheSameWay covers the other
// shutdown path.
func TestPeakWatcherContextCancelEndsTheGoroutineTheSameWay(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sampler := &fakeTreeSampler{tree: treeOf(10)}
		w, err := NewPeakWatcher(sampler, 1, testPollInterval)
		require.NoError(t, err, "NewPeakWatcher")

		ctx, cancel := context.WithCancel(context.Background())
		stop := w.Start(ctx)
		defer stop()

		advance(2)
		cancel()
		synctest.Wait()

		callsAtCancel := sampler.callCount()
		require.Equal(t, 2, callsAtCancel)

		advance(10)
		require.Equal(t, callsAtCancel, sampler.callCount(), "no further samples after cancellation")
	})
}

func TestPeakSummaryEvaluatePassesWithinBudget(t *testing.T) {
	summary := PeakSummary{Samples: 10, PeakTotalRSSBytes: 400 << 20}
	result := summary.Evaluate(MemoryBudget{Name: "golc tree", MaxBytes: 512 << 20})
	require.True(t, result.Pass, "expected a pass, got %+v", result)
	require.Empty(t, result.Reason)
}

// TestPeakSummaryEvaluateFailsClosed mirrors the latency side: a summary
// with no successful sample has produced no evidence, and must never be
// read as evidence that a budget was met.
func TestPeakSummaryEvaluateFailsClosed(t *testing.T) {
	t.Run("no samples at all", func(t *testing.T) {
		result := PeakSummary{}.Evaluate(MemoryBudget{Name: "golc tree", MaxBytes: 1 << 30})
		require.False(t, result.Pass, "an unsampled tree must never satisfy a budget")
		require.Contains(t, result.Reason, "GOLC_MEASURE_NO_SAMPLES")
	})

	t.Run("every sample failed", func(t *testing.T) {
		summary := PeakSummary{Samples: 7, Failures: 7, LastErr: "boom"}
		result := summary.Evaluate(MemoryBudget{Name: "golc tree", MaxBytes: 1 << 30})
		require.False(t, result.Pass, "an all-failed run must never satisfy a budget")
		require.Contains(t, result.Reason, "GOLC_MEASURE_NO_SAMPLES")
		require.Zero(t, result.Observed)
	})

	t.Run("genuine overrun", func(t *testing.T) {
		summary := PeakSummary{Samples: 10, PeakTotalRSSBytes: 900 << 20}
		result := summary.Evaluate(MemoryBudget{Name: "golc tree", MaxBytes: 512 << 20})
		require.False(t, result.Pass)
		require.Contains(t, result.Reason, "GOLC_MEASURE_BUDGET_EXCEEDED")
		require.Equal(t, uint64(900<<20), result.Observed)
	})
}

// TestSortDescendantsIsDeterministic covers the ordering that keeps a
// TreeMemory's JSON stable across samples of the same tree.
func TestSortDescendantsIsDeterministic(t *testing.T) {
	descendants := []ProcessMemory{
		{PID: 300, Depth: 1}, {PID: 100, Depth: 2}, {PID: 200, Depth: 1}, {PID: 50, Depth: 2},
	}
	sortDescendants(descendants)
	require.Equal(t, []ProcessMemory{
		{PID: 200, Depth: 1}, {PID: 300, Depth: 1}, {PID: 50, Depth: 2}, {PID: 100, Depth: 2},
	}, descendants)
}

// TestSystemSamplerReadsThisProcess is the one case here that touches a
// real OS process, and it is deliberately NOT bubbled: gopsutil issues
// real syscalls, which is exactly what a synctest bubble forbids. It is
// kept minimal and self-targeted -- the test binary samples itself, so it
// needs no fixture process and cannot hang waiting on one. Its job is to
// prove the gopsutil wiring is real, not to assert any particular size.
func TestSystemSamplerReadsThisProcess(t *testing.T) {
	tree, err := SystemSampler{}.SampleTree(int32(os.Getpid()))
	require.NoError(t, err, "SampleTree on our own pid")
	require.Equal(t, int32(os.Getpid()), tree.Root.PID)
	require.Positive(t, tree.Root.RSSBytes, "a live Go test binary must hold some resident memory")
	require.GreaterOrEqual(t, tree.TotalRSSBytes, tree.Root.RSSBytes,
		"the tree total must include at least the root")
}

// TestSystemSamplerRejectsAnUnreadableRoot covers the one genuine error
// path: a pid that is not there.
func TestSystemSamplerRejectsAnUnreadableRoot(t *testing.T) {
	// A pid this large is not assignable on any platform this repo builds
	// for, so it can never collide with a live process.
	_, err := SystemSampler{}.SampleTree(math.MaxInt32)
	require.Error(t, err, "expected an unreadable-root error")
	require.Contains(t, err.Error(), "GOLC_MEASURE_PROCESS_UNREADABLE")
}
