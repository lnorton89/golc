// clock_test.go proves the pure musical-position clock's contract
// (03-06-PLAN.md Task 1): Position advances BarIndex as elapsed time
// crosses secondsPerBar, wraps via modulo barsPerLoop, is deterministic
// (including across many concurrent goroutines with identical
// arguments), and attributes a position sampled exactly on a bar boundary
// to the new bar (floor semantics), never the previous one. TapTempo
// converts two or more ordered taps into a positive BPM, rejects fewer
// than two taps and a zero-interval tap pair. RecomputeEpoch preserves
// the current bar/beat position across a BPM change when preserve=true,
// and restarts at bar 0 (now) when preserve=false.
package playback_test

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/playback"
)

func TestClockPositionAdvancesAndWraps(t *testing.T) {
	loopStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	bpm := 120.0 // secondsPerBeat = 0.5s; secondsPerBar (4/4) = 2s

	cases := []struct {
		name             string
		elapsed          time.Duration
		barsPerLoop      int
		wantBarIndex     int
		wantBeatFraction float64
	}{
		{"start of loop", 0, 4, 0, 0.0},
		{"mid first bar", 1 * time.Second, 4, 0, 0.5},
		{"start of second bar", 2 * time.Second, 4, 1, 0.0},
		{"mid second bar", 3 * time.Second, 4, 1, 0.5},
		{"wraps at loop boundary (barsPerLoop=2)", 4 * time.Second, 2, 0, 0.0},
		{"barsPerLoop=1 loops every bar", 2 * time.Second, 1, 0, 0.0},
		{"barsPerLoop=1 mid loop", 3 * time.Second, 1, 0, 0.5},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			now := loopStart.Add(tc.elapsed)
			pos := playback.Position(now, bpm, tc.barsPerLoop, loopStart)
			assert.Equal(t, tc.wantBarIndex, pos.BarIndex)
			assert.InDelta(t, tc.wantBeatFraction, pos.BeatFraction, 1e-9)
		})
	}
}

// TestClockPositionNonPositiveBarsPerLoopDoesNotPanic proves WR-02:
// Position defensively clamps a non-positive barsPerLoop to 1 rather than
// panicking with an integer divide-by-zero -- a future direct caller of
// this exported function that passes an unvalidated barsPerLoop degrades
// to a single-bar loop instead of crashing the process.
func TestClockPositionNonPositiveBarsPerLoopDoesNotPanic(t *testing.T) {
	loopStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	bpm := 120.0 // secondsPerBar = 2s

	for _, barsPerLoop := range []int{0, -1, -100} {
		now := loopStart.Add(3 * time.Second) // 1.5 bars elapsed
		pos := playback.Position(now, bpm, barsPerLoop, loopStart)
		assert.Equal(t, 0, pos.BarIndex, "barsPerLoop=%d: expected the clamped single-bar loop to report BarIndex=0", barsPerLoop)
		assert.InDelta(t, 0.5, pos.BeatFraction, 1e-9, "barsPerLoop=%d", barsPerLoop)
	}
}

func TestClockPositionDeterministicSameArgs(t *testing.T) {
	loopStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := loopStart.Add(1500 * time.Millisecond)

	first := playback.Position(now, 128.0, 8, loopStart)
	second := playback.Position(now, 128.0, 8, loopStart)
	require.Equal(t, first, second, "Position called twice with identical args returned different results")
}

func TestClockPositionDeterministicAcrossGoroutines(t *testing.T) {
	loopStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := loopStart.Add(3700 * time.Millisecond)
	const bpm = 140.0
	const barsPerLoop = 8

	want := playback.Position(now, bpm, barsPerLoop, loopStart)

	const goroutines = 100
	results := make([]playback.MusicalPosition, goroutines)
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(i int) {
			defer wg.Done()
			results[i] = playback.Position(now, bpm, barsPerLoop, loopStart)
		}(i)
	}
	wg.Wait()

	for i, got := range results {
		require.Equal(t, want, got, "goroutine %d: Position not byte-identical", i)
	}
}

func TestClockPositionFloorSemanticsAtBarBoundary(t *testing.T) {
	loopStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	bpm := 120.0 // secondsPerBar = 2s

	// Sample exactly on the boundary between bar 2 and bar 3 (elapsed = 3
	// * secondsPerBar = 6s): must be attributed to bar 3, never bar 2.
	now := loopStart.Add(6 * time.Second)
	pos := playback.Position(now, bpm, 8, loopStart)
	require.Equal(t, 3, pos.BarIndex, "expected exact-boundary sample to be attributed to the new bar 3")
	require.Equal(t, 0.0, pos.BeatFraction, "expected exact-boundary sample to have BeatFraction=0.0")
}

func TestTapTempoComputesPositiveBPM(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	// Three taps, 0.5s apart -> 120 BPM.
	taps := []time.Time{base, base.Add(500 * time.Millisecond), base.Add(1 * time.Second)}

	bpm, err := playback.TapTempo(taps)
	require.NoError(t, err, "TapTempo")
	require.InDelta(t, 120.0, bpm, 1e-6, "TapTempo")
}

func TestTapTempoRejectsFewerThanTwoTaps(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	for _, taps := range [][]time.Time{nil, {base}} {
		_, err := playback.TapTempo(taps)
		require.ErrorContains(t, err, "GOLC_PLAYBACK_TAP_INVALID", "expected error for %d taps", len(taps))
	}
}

func TestTapTempoRejectsZeroInterval(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	taps := []time.Time{base, base} // same instant

	_, err := playback.TapTempo(taps)
	require.ErrorContains(t, err, "GOLC_PLAYBACK_TAP_INVALID", "expected error for a zero-interval tap pair")
}

func TestTapTempoRejectsOutOfOrderTaps(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	// Second tap earlier than the first: a negative interval, never an
	// infinite/NaN BPM.
	taps := []time.Time{base, base.Add(-1 * time.Second)}

	_, err := playback.TapTempo(taps)
	require.ErrorContains(t, err, "GOLC_PLAYBACK_TAP_INVALID", "expected error for out-of-order taps")
}

func TestBPMChangeEpochPreservesPosition(t *testing.T) {
	loopStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	oldBPM := 120.0
	newBPM := 90.0
	barsPerLoop := 8

	now := loopStart.Add(3500 * time.Millisecond) // 1.75 bars elapsed at 120bpm
	before := playback.Position(now, oldBPM, barsPerLoop, loopStart)

	newEpoch := playback.RecomputeEpoch(true, oldBPM, newBPM, barsPerLoop, loopStart, now)
	after := playback.Position(now, newBPM, barsPerLoop, newEpoch)

	require.Equal(t, before.BarIndex, after.BarIndex, "preserve=true: BarIndex changed across BPM change")
	require.InDelta(t, before.BeatFraction, after.BeatFraction, 1e-6, "preserve=true: BeatFraction changed across BPM change")
}

func TestBPMChangeEpochRestartsAtBarZero(t *testing.T) {
	loopStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := loopStart.Add(3500 * time.Millisecond)

	newEpoch := playback.RecomputeEpoch(false, 120.0, 90.0, 8, loopStart, now)
	require.True(t, newEpoch.Equal(now), "preserve=false: expected the new epoch to equal now (%v), got %v", now, newEpoch)

	after := playback.Position(now, 90.0, 8, newEpoch)
	require.Equal(t, 0, after.BarIndex, "preserve=false: expected restart at bar 0, got %+v", after)
	require.Equal(t, 0.0, after.BeatFraction, "preserve=false: expected restart at bar 0, got %+v", after)
}

func TestValidateBPMRejectsNonPositiveAndOutOfRange(t *testing.T) {
	for _, bpm := range []float64{0, -1, 1000} {
		err := playback.ValidateBPM(bpm)
		assert.ErrorContains(t, err, "GOLC_PLAYBACK_BPM_INVALID", "ValidateBPM(%v)", bpm)
	}
}

func TestValidateBPMAcceptsCurrentValueIdempotently(t *testing.T) {
	require.NoError(t, playback.ValidateBPM(120.0), "ValidateBPM(120.0)")
	require.NoError(t, playback.ValidateBPM(120.0), "ValidateBPM(120.0) second call (idempotent no-op)")
}

func TestCrossedBarBoundaryDetectsTransitionNotEquality(t *testing.T) {
	require.False(t, playback.CrossedBarBoundary(2, 2), "expected no transition when BarIndex is unchanged")
	require.True(t, playback.CrossedBarBoundary(2, 3), "expected a transition when BarIndex changes")
	// Loop wraparound: last bar of an 8-bar loop back to bar 0.
	require.True(t, playback.CrossedBarBoundary(7, 0), "expected a transition across loop wraparound")
}
