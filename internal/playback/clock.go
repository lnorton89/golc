// clock.go implements the pure musical-position clock (CONTEXT SCEN-01/
// D-10, 03-RESEARCH.md Pattern 1): Position derives a scene's bar/beat
// location directly from (bpm, barsPerLoop, loopStart epoch, now) using
// monotonic time.Sub -- never from an accumulated tick counter, so a
// stalled or coalesced tick changes only when the engine (03-07) computes
// the answer, never what the answer is for a given elapsed time (SCEN-09's
// determinism guarantee is structurally free from this property). This
// file also owns numeric/tap-tempo BPM validation (ValidateBPM, TapTempo,
// SCEN-02/SCEN-03) and the preserve-position-or-restart epoch recompute
// (RecomputeEpoch, SCEN-08/D-11) internal/command/playback.go reads/
// writes through, plus the boundary-detection helper the engine (03-07)
// reuses to avoid the equality-check pitfall (03-RESEARCH.md Pitfall 1).
//
// No manual Windows timer-resolution call (winmm/timeBeginPeriod/
// NtSetTimerResolution) is added anywhere in this package: Go 1.26.5
// (this repo's pinned toolchain) already provides ~0.5ms Windows timer
// resolution natively (03-RESEARCH.md Pitfall 2) -- reintroducing that
// pre-Go-1.23 workaround here would be a regression, not a fix.
package playback

import (
	"fmt"
	"math"
	"time"
)

// beatsPerBar is the v1 fixed time signature (4/4) 03-RESEARCH.md
// Assumption A1/Open Question 2 flags for planning confirmation: SCEN-01
// configures a bar count, not beats-per-bar, so this stays a named
// constant rather than a per-scene/per-show parameter until a future
// phase needs configurable time signatures -- at which point it becomes a
// parameter to Position/secondsPerBar without any other architecture
// change.
const beatsPerBar = 4.0

// maxBPM bounds ValidateBPM (DoS/tampering ceiling, CONTEXT threat
// T-03-05, mirrors internal/scene's maxBarsPerLoop precedent): a
// pathologically large BPM is rejected with GOLC_PLAYBACK_BPM_INVALID
// rather than allowed to grow unbounded or approach a degenerate
// near-zero secondsPerBar.
const maxBPM = 999.0

// MusicalPosition is a scene's bar/beat location at one instant: BarIndex
// is the 0-based bar within the configured loop (already wrapped modulo
// barsPerLoop); BeatFraction is the fractional progress through the
// current bar, 0.0 (start of bar) up to (but never reaching) 1.0.
type MusicalPosition struct {
	BarIndex     int
	BeatFraction float64
}

// secondsPerBar converts bpm into the duration of one musical bar under
// the fixed beatsPerBar time signature.
func secondsPerBar(bpm float64) float64 {
	secondsPerBeat := 60.0 / bpm
	return secondsPerBeat * beatsPerBar
}

// Position computes now's musical position within a barsPerLoop-bar loop
// that began at loopStart, against the global bpm -- a pure function of
// its four arguments (03-RESEARCH.md Pattern 1): elapsed is derived from
// now.Sub(loopStart), a monotonic subtraction immune to wall-clock jumps,
// never from any accumulated tick counter. Calling Position twice with
// identical arguments -- from the same goroutine or from many goroutines
// concurrently -- always returns an identical, byte-identical
// MusicalPosition, because it reads and mutates no shared state. A
// position sampled exactly on a bar boundary (elapsed == k *
// secondsPerBar) is attributed to the new bar k, never k-1: barsElapsed
// is exactly k in that case, and truncating it toward zero yields k, not
// k-1 (03-RESEARCH.md Pitfall 1 concerns detecting the transition, not
// this attribution, which the truncation already gets right).
//
// barsPerLoop must be a positive integer (every caller within this repo
// only ever supplies one already validated by scene.ValidateScene's
// [1, maxBarsPerLoop] check). A non-positive barsPerLoop is defensively
// clamped to 1 here (WR-02) rather than left to panic on the integer
// divide/mod below -- a hand-constructed or otherwise-unvalidated
// barsPerLoop from a future direct caller of this exported function
// degrades to a single-bar loop instead of crashing the process.
func Position(now time.Time, bpm float64, barsPerLoop int, loopStart time.Time) MusicalPosition {
	if barsPerLoop <= 0 {
		barsPerLoop = 1
	}
	perBar := secondsPerBar(bpm)
	elapsed := now.Sub(loopStart).Seconds()
	barsElapsed := elapsed / perBar
	wholeBarsElapsed := int(barsElapsed)
	barIndex := wholeBarsElapsed % barsPerLoop
	beatFraction := barsElapsed - float64(wholeBarsElapsed)
	return MusicalPosition{BarIndex: barIndex, BeatFraction: beatFraction}
}

// CrossedBarBoundary reports whether currentBarIndex represents a bar
// transition since lastBarIndex, comparing integer BarIndex values --
// never a BeatFraction == 0.0 equality check, which floating-point
// position sampled at arbitrary tick times essentially never satisfies
// (03-RESEARCH.md Pitfall 1). The engine (03-07) reuses this exact
// helper at every tick to decide whether a staged edit may be adopted
// (CONTEXT D-05).
func CrossedBarBoundary(lastBarIndex, currentBarIndex int) bool {
	return currentBarIndex != lastBarIndex
}

// ValidateBPM rejects a BPM that is not a positive, finite number within
// the declared sane ceiling (CONTEXT threat T-03-05, SCEN-02
// adjacency/empty case): the current value is always accepted again
// (idempotent no-op) since no comparison against a prior value happens
// here -- only the value's own shape is checked.
func ValidateBPM(bpm float64) error {
	if math.IsNaN(bpm) || math.IsInf(bpm, 0) {
		return fmt.Errorf("GOLC_PLAYBACK_BPM_INVALID: bpm must be a finite number, got %v", bpm)
	}
	if bpm <= 0 {
		return fmt.Errorf("GOLC_PLAYBACK_BPM_INVALID: bpm %v must be greater than zero", bpm)
	}
	if bpm > maxBPM {
		return fmt.Errorf("GOLC_PLAYBACK_BPM_INVALID: bpm %v exceeds the maximum of %v", bpm, maxBPM)
	}
	return nil
}

// TapTempo converts ordered tap timestamps (CONTEXT SCEN-03) into a BPM:
// taps are consumed strictly in the order given (arrival order), never
// re-sorted, so the result is deterministic for a given ordered input.
// Fewer than two taps cannot express an interval and is rejected with
// GOLC_PLAYBACK_TAP_INVALID (no BPM change). Two taps at the same instant
// -- or any two consecutive taps that are not strictly increasing -- are
// also rejected with GOLC_PLAYBACK_TAP_INVALID rather than producing an
// infinite or NaN BPM (CONTEXT threat T-03-05). The resulting BPM is the
// mean of every consecutive inter-tap interval, then validated through
// ValidateBPM so an absurdly fast tap sequence cannot exceed the same
// sane ceiling numeric entry is held to.
func TapTempo(taps []time.Time) (float64, error) {
	if len(taps) < 2 {
		return 0, fmt.Errorf("GOLC_PLAYBACK_TAP_INVALID: at least two taps are required to compute a tempo, got %d", len(taps))
	}

	var totalSeconds float64
	for i := 1; i < len(taps); i++ {
		interval := taps[i].Sub(taps[i-1]).Seconds()
		if interval <= 0 {
			return 0, fmt.Errorf(
				"GOLC_PLAYBACK_TAP_INVALID: tap %d must be strictly later than tap %d, got a %.6fs interval", i, i-1, interval)
		}
		totalSeconds += interval
	}
	averageInterval := totalSeconds / float64(len(taps)-1)
	bpm := 60.0 / averageInterval

	if err := ValidateBPM(bpm); err != nil {
		return 0, err
	}
	return bpm, nil
}

// RecomputeEpoch resolves the loopStart epoch a BPM change should use
// (CONTEXT SCEN-08/D-11): when preserve is false, the loop restarts at
// bar 0 from now, so the new epoch is simply now. When preserve is true,
// a new epoch is solved so that Position(now, newBPM, barsPerLoop,
// newEpoch) reports the exact same bar/beat location Position(now,
// oldBPM, barsPerLoop, loopStart) reports right now -- the running
// look/chase/motion neither blanks nor jumps mid-bar (CONTEXT
// prohibition). barsPerLoop does not otherwise enter the epoch math: the
// wrap it controls is applied identically by Position under either BPM,
// so preserving the raw (unwrapped) bars-elapsed fraction is sufficient
// to preserve both BarIndex and BeatFraction.
func RecomputeEpoch(preserve bool, oldBPM, newBPM float64, barsPerLoop int, loopStart, now time.Time) time.Time {
	if !preserve {
		return now
	}
	oldPerBar := secondsPerBar(oldBPM)
	newPerBar := secondsPerBar(newBPM)

	elapsedOld := now.Sub(loopStart).Seconds()
	barsElapsed := elapsedOld / oldPerBar
	elapsedNew := barsElapsed * newPerBar

	return now.Add(-time.Duration(elapsedNew * float64(time.Second)))
}
