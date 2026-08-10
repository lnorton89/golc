// latency_test.go covers internal/measure/latency.go: construction
// validation, the clamp-and-count contract for out-of-range samples,
// snapshot determinism, coordinated-omission correction, and every
// Budget.Evaluate branch -- especially the ones that must FAIL rather than
// pass by default (no samples, a clamped tail, a name or quantile
// mismatch), since a budget that passes on absent evidence is worse than
// no budget at all.
//
// Every case here is a pure value computation: no clock, no goroutine, no
// I/O. Recorder never reads a clock itself precisely so that its tests
// never have to either.
package measure

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func mustRecorder(t *testing.T, name string) *Recorder {
	t.Helper()
	r, err := NewRecorder(name, time.Microsecond, 30*time.Second, 3)
	require.NoError(t, err, "NewRecorder")
	return r
}

func TestNewRecorderRejectsInvalidConfiguration(t *testing.T) {
	cases := []struct {
		label   string
		name    string
		lowest  time.Duration
		highest time.Duration
		sigfigs int
		code    string
	}{
		{"empty name", "", time.Microsecond, time.Second, 3, "GOLC_MEASURE_NAME_EMPTY"},
		{"zero lowest", "tick", 0, time.Second, 3, "GOLC_MEASURE_BOUNDS_INVALID"},
		{"negative lowest", "tick", -1, time.Second, 3, "GOLC_MEASURE_BOUNDS_INVALID"},
		{"inverted bounds", "tick", time.Second, time.Millisecond, 3, "GOLC_MEASURE_BOUNDS_INVALID"},
		{"equal bounds", "tick", time.Second, time.Second, 3, "GOLC_MEASURE_BOUNDS_INVALID"},
		{"sigfigs too low", "tick", time.Microsecond, time.Second, 0, "GOLC_MEASURE_SIGFIGS_INVALID"},
		{"sigfigs too high", "tick", time.Microsecond, time.Second, 6, "GOLC_MEASURE_SIGFIGS_INVALID"},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			_, err := NewRecorder(tc.name, tc.lowest, tc.highest, tc.sigfigs)
			require.Error(t, err, "expected %s", tc.code)
			require.Contains(t, err.Error(), tc.code, "unexpected error: %v", err)
		})
	}
}

// TestRecordRejectsNegativeDurations covers the one input that can only be
// a caller bug: two clock reads subtracted in the wrong order.
func TestRecordRejectsNegativeDurations(t *testing.T) {
	r := mustRecorder(t, "override")
	require.ErrorContains(t, r.Record(-time.Millisecond), "GOLC_MEASURE_VALUE_NEGATIVE")
	require.ErrorContains(t, r.RecordInterval(-time.Millisecond, time.Millisecond), "GOLC_MEASURE_VALUE_NEGATIVE")
	require.Zero(t, r.Snapshot().Count, "a rejected sample must not be recorded")
}

// TestSnapshotReportsPercentilesWithinHDRPrecision feeds a known
// distribution and asserts each reported quantile lands within the 3
// significant figures the recorder was configured for.
func TestSnapshotReportsPercentilesWithinHDRPrecision(t *testing.T) {
	r := mustRecorder(t, "tick")
	// 1..1000 ms, one sample each: the qN value is the Nth percentile of a
	// uniform 1..1000 distribution, i.e. ~N*10ms.
	for i := 1; i <= 1000; i++ {
		require.NoError(t, r.Record(time.Duration(i)*time.Millisecond))
	}

	snapshot := r.Snapshot()
	require.Equal(t, int64(1000), snapshot.Count)
	require.Zero(t, snapshot.OutOfRange, "nothing here exceeds the 30s ceiling")
	require.InDelta(t, float64(time.Millisecond), float64(snapshot.Min), float64(time.Microsecond))
	require.InDelta(t, float64(time.Second), float64(snapshot.Max), float64(time.Millisecond))

	for _, want := range []struct {
		quantile float64
		expected time.Duration
	}{
		{50, 500 * time.Millisecond},
		{90, 900 * time.Millisecond},
		{99, 990 * time.Millisecond},
	} {
		got := r.ValueAt(want.quantile)
		// 3 significant figures => within 0.1% of the true value.
		require.InDelta(t, float64(want.expected), float64(got), float64(want.expected)*0.001,
			"q%.4g = %s, want ~%s", want.quantile, got, want.expected)
	}
}

// TestOutOfRangeSamplesAreClampedAndCountedNeverDropped covers the
// contract that matters most for a soak: the ceiling being too low must be
// visible in the report, and must never quietly delete the outliers a soak
// exists to find.
func TestOutOfRangeSamplesAreClampedAndCountedNeverDropped(t *testing.T) {
	r, err := NewRecorder("tick", time.Microsecond, 100*time.Millisecond, 3)
	require.NoError(t, err, "NewRecorder")

	require.NoError(t, r.Record(10*time.Millisecond))
	require.NoError(t, r.Record(5*time.Second)) // far above the ceiling

	snapshot := r.Snapshot()
	require.Equal(t, int64(2), snapshot.Count, "the over-ceiling sample must still be counted")
	require.Equal(t, int64(1), snapshot.OutOfRange, "the over-ceiling sample must be reported as clamped")
	require.InDelta(t, float64(100*time.Millisecond), float64(snapshot.Max), float64(time.Millisecond),
		"the clamped sample must land at the ceiling, not be discarded")
}

// TestRecordIntervalCorrectsForCoordinatedOmission proves the difference
// that matters for a fixed-cadence measurement: a single long stall must
// contribute the ticks it swallowed, not one lonely outlier.
func TestRecordIntervalCorrectsForCoordinatedOmission(t *testing.T) {
	const expected = 25 * time.Millisecond

	naive := mustRecorder(t, "tick")
	corrected := mustRecorder(t, "tick")
	for i := 0; i < 99; i++ {
		require.NoError(t, naive.Record(expected))
		require.NoError(t, corrected.RecordInterval(expected, expected))
	}
	// One 500ms stall: twenty ticks' worth of cadence went missing.
	require.NoError(t, naive.Record(500*time.Millisecond))
	require.NoError(t, corrected.RecordInterval(500*time.Millisecond, expected))

	require.Equal(t, int64(100), naive.Snapshot().Count)
	require.Greater(t, corrected.Snapshot().Count, int64(100),
		"the correction must back-fill the ticks the stall swallowed")

	// The naive p99 hides the stall entirely; the corrected one cannot.
	require.InDelta(t, float64(expected), float64(naive.ValueAt(99)), float64(time.Millisecond),
		"naive q99 should still look nominal -- that is the trap this guards against")
	require.Greater(t, corrected.ValueAt(99), expected,
		"corrected q99 must reflect the stall")
}

// TestResetClearsSamplesAndOutOfRangeCount covers reusing one recorder
// across consecutive soak segments.
func TestResetClearsSamplesAndOutOfRangeCount(t *testing.T) {
	r, err := NewRecorder("tick", time.Microsecond, 10*time.Millisecond, 3)
	require.NoError(t, err, "NewRecorder")
	require.NoError(t, r.Record(time.Millisecond))
	require.NoError(t, r.Record(time.Second))
	require.Equal(t, int64(1), r.Snapshot().OutOfRange)

	r.Reset()
	snapshot := r.Snapshot()
	require.Zero(t, snapshot.Count)
	require.Zero(t, snapshot.OutOfRange)
	require.Zero(t, snapshot.Max)
}

// TestEmptySnapshotReportsZerosNotGarbage covers the zero-sample
// projection: every quantile must still be present (so a report's shape
// does not change with its content) and every value must be zero.
func TestEmptySnapshotReportsZerosNotGarbage(t *testing.T) {
	snapshot := mustRecorder(t, "tick").Snapshot()
	require.Zero(t, snapshot.Count)
	require.Len(t, snapshot.Quantiles, len(DefaultQuantiles))
	for _, q := range snapshot.Quantiles {
		require.Zero(t, q.Value, "q%.4g", q.Quantile)
	}
}

// TestSnapshotJSONIsDeterministic covers the reason Quantiles is an
// ordered slice rather than a map: release evidence gets committed and
// diffed, so encoding the same snapshot twice must be byte-identical.
func TestSnapshotJSONIsDeterministic(t *testing.T) {
	r := mustRecorder(t, "artnet-send")
	for i := 1; i <= 50; i++ {
		require.NoError(t, r.Record(time.Duration(i)*time.Millisecond))
	}
	snapshot := r.Snapshot()

	first, err := json.Marshal(snapshot)
	require.NoError(t, err, "marshal")
	for i := 0; i < 20; i++ {
		again, err := json.Marshal(snapshot)
		require.NoError(t, err, "marshal")
		require.Equal(t, string(first), string(again), "snapshot JSON must be byte-stable")
	}
	require.Contains(t, string(first), `"name":"artnet-send"`)
}

// TestSnapshotAtHonoursCallerQuantileOrder covers reporting a caller's own
// quantiles, in the order given.
func TestSnapshotAtHonoursCallerQuantileOrder(t *testing.T) {
	r := mustRecorder(t, "tick")
	require.NoError(t, r.Record(time.Millisecond))

	snapshot := r.SnapshotAt(99.99, 50, 75)
	require.Len(t, snapshot.Quantiles, 3)
	require.Equal(t, 99.99, snapshot.Quantiles[0].Quantile)
	require.Equal(t, 50.0, snapshot.Quantiles[1].Quantile)
	require.Equal(t, 75.0, snapshot.Quantiles[2].Quantile)

	require.Len(t, r.SnapshotAt().Quantiles, len(DefaultQuantiles),
		"an empty quantile list must fall back to DefaultQuantiles, never report none")
}

// TestEvaluatePassesWithinBudget covers the one passing branch.
func TestEvaluatePassesWithinBudget(t *testing.T) {
	r := mustRecorder(t, "tick")
	for i := 0; i < 1000; i++ {
		require.NoError(t, r.Record(20*time.Millisecond))
	}

	result := r.Snapshot().Evaluate(Budget{Name: "tick", Quantile: 99, Max: 25 * time.Millisecond})
	require.True(t, result.Pass, "expected a pass, got %+v", result)
	require.Empty(t, result.Reason)
	require.Equal(t, int64(1000), result.Count)
}

// TestEvaluateFailsClosed covers every branch that must NOT pass by
// default. Each case is a way a budget could look satisfied while the
// evidence for it is missing or untrustworthy.
func TestEvaluateFailsClosed(t *testing.T) {
	t.Run("no samples is unevaluated, not met", func(t *testing.T) {
		result := mustRecorder(t, "tick").Snapshot().
			Evaluate(Budget{Name: "tick", Quantile: 99, Max: time.Hour})
		require.False(t, result.Pass, "an empty recorder must never satisfy a budget")
		require.Contains(t, result.Reason, "GOLC_MEASURE_NO_SAMPLES")
	})

	t.Run("clamped tail is a lower bound, not a measurement", func(t *testing.T) {
		r, err := NewRecorder("tick", time.Microsecond, 10*time.Millisecond, 3)
		require.NoError(t, err, "NewRecorder")
		require.NoError(t, r.Record(time.Millisecond))
		require.NoError(t, r.Record(time.Minute))

		// The clamped value (10ms) is under the budget, so a naive
		// implementation would pass here. It must not.
		result := r.Snapshot().Evaluate(Budget{Name: "tick", Quantile: 99, Max: time.Second})
		require.False(t, result.Pass, "a clamped tail must never satisfy a budget")
		require.Contains(t, result.Reason, "GOLC_MEASURE_TAIL_CLAMPED")
	})

	t.Run("budget naming another quantity", func(t *testing.T) {
		r := mustRecorder(t, "tick")
		require.NoError(t, r.Record(time.Millisecond))
		result := r.Snapshot().Evaluate(Budget{Name: "override", Quantile: 99, Max: time.Hour})
		require.False(t, result.Pass)
		require.Contains(t, result.Reason, "GOLC_MEASURE_BUDGET_MISMATCH")
	})

	t.Run("quantile the snapshot does not report", func(t *testing.T) {
		r := mustRecorder(t, "tick")
		require.NoError(t, r.Record(time.Millisecond))
		result := r.Snapshot().Evaluate(Budget{Name: "tick", Quantile: 99.9999, Max: time.Hour})
		require.False(t, result.Pass)
		require.Contains(t, result.Reason, "GOLC_MEASURE_QUANTILE_UNREPORTED")
	})

	t.Run("genuine overrun", func(t *testing.T) {
		r := mustRecorder(t, "tick")
		for i := 0; i < 100; i++ {
			require.NoError(t, r.Record(80*time.Millisecond))
		}
		result := r.Snapshot().Evaluate(Budget{Name: "tick", Quantile: 99, Max: 25 * time.Millisecond})
		require.False(t, result.Pass)
		require.Contains(t, result.Reason, "GOLC_MEASURE_BUDGET_EXCEEDED")
		require.Greater(t, result.Observed, 25*time.Millisecond)
	})
}

// TestRecorderIsSafeUnderConcurrentUse covers the mutex contract: many
// goroutines recording at once must produce exactly the expected sample
// count and never race (this file runs under -race in CI).
func TestRecorderIsSafeUnderConcurrentUse(t *testing.T) {
	r := mustRecorder(t, "tick")

	const goroutines, perGoroutine = 8, 500
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < perGoroutine; j++ {
				_ = r.Record(time.Millisecond)
			}
		}()
	}
	// Concurrent readers must not corrupt or block the writers either.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 100; j++ {
			_ = r.Snapshot()
		}
	}()
	wg.Wait()

	require.Equal(t, int64(goroutines*perGoroutine), r.Snapshot().Count)
}
