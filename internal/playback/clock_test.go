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
	"strings"
	"sync"
	"testing"
	"time"

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
			if pos.BarIndex != tc.wantBarIndex {
				t.Errorf("BarIndex = %d, want %d", pos.BarIndex, tc.wantBarIndex)
			}
			if diff := pos.BeatFraction - tc.wantBeatFraction; diff < -1e-9 || diff > 1e-9 {
				t.Errorf("BeatFraction = %v, want %v", pos.BeatFraction, tc.wantBeatFraction)
			}
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
		if pos.BarIndex != 0 {
			t.Errorf("barsPerLoop=%d: expected the clamped single-bar loop to report BarIndex=0, got %d", barsPerLoop, pos.BarIndex)
		}
		if diff := pos.BeatFraction - 0.5; diff < -1e-9 || diff > 1e-9 {
			t.Errorf("barsPerLoop=%d: expected BeatFraction=0.5, got %v", barsPerLoop, pos.BeatFraction)
		}
	}
}

func TestClockPositionDeterministicSameArgs(t *testing.T) {
	loopStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := loopStart.Add(1500 * time.Millisecond)

	first := playback.Position(now, 128.0, 8, loopStart)
	second := playback.Position(now, 128.0, 8, loopStart)
	if first != second {
		t.Fatalf("Position called twice with identical args returned different results: %+v vs %+v", first, second)
	}
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
		if got != want {
			t.Fatalf("goroutine %d: Position = %+v, want byte-identical %+v", i, got, want)
		}
	}
}

func TestClockPositionFloorSemanticsAtBarBoundary(t *testing.T) {
	loopStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	bpm := 120.0 // secondsPerBar = 2s

	// Sample exactly on the boundary between bar 2 and bar 3 (elapsed = 3
	// * secondsPerBar = 6s): must be attributed to bar 3, never bar 2.
	now := loopStart.Add(6 * time.Second)
	pos := playback.Position(now, bpm, 8, loopStart)
	if pos.BarIndex != 3 {
		t.Fatalf("expected exact-boundary sample to be attributed to the new bar 3, got BarIndex=%d", pos.BarIndex)
	}
	if pos.BeatFraction != 0.0 {
		t.Fatalf("expected exact-boundary sample to have BeatFraction=0.0, got %v", pos.BeatFraction)
	}
}

func TestTapTempoComputesPositiveBPM(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	// Three taps, 0.5s apart -> 120 BPM.
	taps := []time.Time{base, base.Add(500 * time.Millisecond), base.Add(1 * time.Second)}

	bpm, err := playback.TapTempo(taps)
	if err != nil {
		t.Fatalf("TapTempo: %v", err)
	}
	if diff := bpm - 120.0; diff < -1e-6 || diff > 1e-6 {
		t.Fatalf("TapTempo = %v, want 120", bpm)
	}
}

func TestTapTempoRejectsFewerThanTwoTaps(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	for _, taps := range [][]time.Time{nil, {base}} {
		_, err := playback.TapTempo(taps)
		if err == nil || !strings.Contains(err.Error(), "GOLC_PLAYBACK_TAP_INVALID") {
			t.Fatalf("expected GOLC_PLAYBACK_TAP_INVALID for %d taps, got %v", len(taps), err)
		}
	}
}

func TestTapTempoRejectsZeroInterval(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	taps := []time.Time{base, base} // same instant

	_, err := playback.TapTempo(taps)
	if err == nil || !strings.Contains(err.Error(), "GOLC_PLAYBACK_TAP_INVALID") {
		t.Fatalf("expected GOLC_PLAYBACK_TAP_INVALID for a zero-interval tap pair, got %v", err)
	}
}

func TestTapTempoRejectsOutOfOrderTaps(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	// Second tap earlier than the first: a negative interval, never an
	// infinite/NaN BPM.
	taps := []time.Time{base, base.Add(-1 * time.Second)}

	_, err := playback.TapTempo(taps)
	if err == nil || !strings.Contains(err.Error(), "GOLC_PLAYBACK_TAP_INVALID") {
		t.Fatalf("expected GOLC_PLAYBACK_TAP_INVALID for out-of-order taps, got %v", err)
	}
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

	if after.BarIndex != before.BarIndex {
		t.Fatalf("preserve=true: BarIndex changed across BPM change: before=%d after=%d", before.BarIndex, after.BarIndex)
	}
	if diff := after.BeatFraction - before.BeatFraction; diff < -1e-6 || diff > 1e-6 {
		t.Fatalf("preserve=true: BeatFraction changed across BPM change: before=%v after=%v", before.BeatFraction, after.BeatFraction)
	}
}

func TestBPMChangeEpochRestartsAtBarZero(t *testing.T) {
	loopStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := loopStart.Add(3500 * time.Millisecond)

	newEpoch := playback.RecomputeEpoch(false, 120.0, 90.0, 8, loopStart, now)
	if !newEpoch.Equal(now) {
		t.Fatalf("preserve=false: expected the new epoch to equal now (%v), got %v", now, newEpoch)
	}

	after := playback.Position(now, 90.0, 8, newEpoch)
	if after.BarIndex != 0 || after.BeatFraction != 0.0 {
		t.Fatalf("preserve=false: expected restart at bar 0, got %+v", after)
	}
}

func TestValidateBPMRejectsNonPositiveAndOutOfRange(t *testing.T) {
	for _, bpm := range []float64{0, -1, 1000} {
		if err := playback.ValidateBPM(bpm); err == nil || !strings.Contains(err.Error(), "GOLC_PLAYBACK_BPM_INVALID") {
			t.Errorf("ValidateBPM(%v): expected GOLC_PLAYBACK_BPM_INVALID, got %v", bpm, err)
		}
	}
}

func TestValidateBPMAcceptsCurrentValueIdempotently(t *testing.T) {
	if err := playback.ValidateBPM(120.0); err != nil {
		t.Fatalf("ValidateBPM(120.0): %v", err)
	}
	if err := playback.ValidateBPM(120.0); err != nil {
		t.Fatalf("ValidateBPM(120.0) second call (idempotent no-op): %v", err)
	}
}

func TestCrossedBarBoundaryDetectsTransitionNotEquality(t *testing.T) {
	if playback.CrossedBarBoundary(2, 2) {
		t.Fatalf("expected no transition when BarIndex is unchanged")
	}
	if !playback.CrossedBarBoundary(2, 3) {
		t.Fatalf("expected a transition when BarIndex changes")
	}
	// Loop wraparound: last bar of an 8-bar loop back to bar 0.
	if !playback.CrossedBarBoundary(7, 0) {
		t.Fatalf("expected a transition across loop wraparound")
	}
}
