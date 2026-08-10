// latency.go implements the percentile-accurate timing recorder Phase 11's
// budget evidence rests on: Recorder wraps one HDR histogram behind a
// mutex, Snapshot is its deterministic, JSON-stable projection, and
// Budget/BudgetResult express "the p99 of X must stay under Y" so a soak
// run produces a pass/fail verdict rather than a wall of numbers.
//
// Two deliberate choices are worth knowing before using this:
//
//   - A sample above the configured ceiling is CLAMPED to the ceiling and
//     counted in Snapshot.OutOfRange, never dropped. An HDR histogram
//     rejects out-of-range values, and silently discarding them would
//     delete precisely the outliers a soak exists to catch -- the clamp
//     keeps every percentile computable while OutOfRange > 0 tells the
//     reader the ceiling itself was set too low.
//
//   - RecordInterval exists alongside Record for fixed-cadence
//     measurements (the 40Hz playback tick, the Art-Net worker tick). It
//     applies HDR's coordinated-omission correction: when one tick stalls
//     for 500ms, a naive recorder logs a single 500ms sample and hides the
//     ~20 ticks that never got to run, flattering the p99 enormously.
//     Record is the right call for a one-shot latency (an override
//     round-trip); RecordInterval is the right call for anything expected
//     to repeat on a schedule.
package measure

import (
	"fmt"
	"sync"
	"time"

	hdrhistogram "github.com/HdrHistogram/hdrhistogram-go"
)

// DefaultQuantiles is the fixed, ordered quantile set Snapshot reports.
// It is a slice rather than a map so a Snapshot's JSON encoding is
// byte-deterministic across runs -- release evidence gets committed and
// diffed, and Go map iteration order would make every run differ.
var DefaultQuantiles = []float64{50, 90, 99, 99.9, 100}

// Recorder accumulates one measured quantity (a tick interval, an
// override round-trip, an Art-Net send latency) into an HDR histogram.
//
// Recorder is safe for concurrent use: hdrhistogram.Histogram is not, so
// every access takes mu. Record is a lock, an integer clamp, and a bucket
// increment -- cheap enough for a supervisory or test harness, but this is
// still a mutex, so it belongs on measurement paths, not inside the
// deterministic frame-evaluation loop itself.
type Recorder struct {
	name    string
	lowest  time.Duration
	highest time.Duration

	mu         sync.Mutex
	hist       *hdrhistogram.Histogram
	outOfRange int64
}

// NewRecorder builds a Recorder tracking [lowest, highest] at sigfigs
// significant decimal digits of precision (HDR's own accuracy knob: 3
// means every reported value is within 0.1% of the true value).
//
// Sizing guidance: pick highest a comfortable order of magnitude above the
// worst latency you would still want to see plotted, not just above the
// budget -- a run that clamps is a run whose tail you cannot read. For a
// 40Hz tick (25ms nominal), 1µs..30s at 3 sigfigs is a reasonable start
// and costs a fixed, small allocation.
func NewRecorder(name string, lowest, highest time.Duration, sigfigs int) (*Recorder, error) {
	if name == "" {
		return nil, fmt.Errorf("GOLC_MEASURE_NAME_EMPTY: a recorder name must not be empty")
	}
	if lowest <= 0 || highest <= lowest {
		return nil, fmt.Errorf(
			"GOLC_MEASURE_BOUNDS_INVALID: %q needs 0 < lowest < highest, got lowest=%s highest=%s",
			name, lowest, highest,
		)
	}
	if sigfigs < 1 || sigfigs > 5 {
		return nil, fmt.Errorf(
			"GOLC_MEASURE_SIGFIGS_INVALID: %q needs 1..5 significant figures, got %d",
			name, sigfigs,
		)
	}
	return &Recorder{
		name:    name,
		lowest:  lowest,
		highest: highest,
		hist:    hdrhistogram.New(int64(lowest), int64(highest), sigfigs),
	}, nil
}

// Name reports the measured quantity's name, as given to NewRecorder.
func (r *Recorder) Name() string { return r.name }

// Record adds one observed duration. A negative duration is rejected
// (GOLC_MEASURE_VALUE_NEGATIVE) because it can only mean the caller
// subtracted two clock reads in the wrong order; a duration above the
// configured ceiling is clamped and counted, never dropped.
func (r *Recorder) Record(d time.Duration) error {
	if d < 0 {
		return fmt.Errorf(
			"GOLC_MEASURE_VALUE_NEGATIVE: %q cannot record a negative duration (%s)",
			r.name, d,
		)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.recordLocked(int64(d))
	return nil
}

// RecordInterval adds one observed duration for a quantity expected to
// repeat every expected (a fixed-cadence tick). Beyond recording d itself
// it back-fills the samples a stall swallowed, so a 500ms hang on a 25ms
// tick contributes the ~20 missed 25ms-and-growing intervals instead of a
// single outlier -- HDR's coordinated-omission correction. Pass a
// non-positive expected to fall back to plain Record semantics.
func (r *Recorder) RecordInterval(d, expected time.Duration) error {
	if d < 0 {
		return fmt.Errorf(
			"GOLC_MEASURE_VALUE_NEGATIVE: %q cannot record a negative duration (%s)",
			r.name, d,
		)
	}
	if expected <= 0 {
		return r.Record(d)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	value := int64(d)
	if value > int64(r.highest) {
		r.outOfRange++
		value = int64(r.highest)
	}
	// RecordCorrectedValue only errors on an out-of-range value, which the
	// clamp above has already excluded.
	_ = r.hist.RecordCorrectedValue(value, int64(expected))
	return nil
}

// recordLocked clamps and records one already-non-negative value. Callers
// must hold mu.
func (r *Recorder) recordLocked(value int64) {
	if value > int64(r.highest) {
		r.outOfRange++
		value = int64(r.highest)
	}
	// RecordValue only errors on an out-of-range value, which the clamp
	// above has already excluded.
	_ = r.hist.RecordValue(value)
}

// Reset discards every recorded sample, including the out-of-range count,
// so one Recorder can serve consecutive soak segments.
func (r *Recorder) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.hist.Reset()
	r.outOfRange = 0
}

// QuantileValue is one reported quantile and the duration at it.
type QuantileValue struct {
	Quantile float64       `json:"quantile"`
	Value    time.Duration `json:"value_ns"`
}

// Snapshot is a Recorder's state at one instant -- a value, not a view, so
// it stays valid while recording continues. Durations encode as
// nanosecond integers so the JSON is stable and machine-comparable across
// runs (release evidence is committed and diffed).
type Snapshot struct {
	Name string `json:"name"`
	// Count is how many samples were recorded, including clamped ones.
	Count int64 `json:"count"`
	// OutOfRange is how many of Count exceeded the configured ceiling and
	// were clamped to it. Non-zero means the ceiling was set too low and
	// the upper tail is not trustworthy -- widen it and re-run.
	OutOfRange int64           `json:"out_of_range"`
	Min        time.Duration   `json:"min_ns"`
	Max        time.Duration   `json:"max_ns"`
	Mean       time.Duration   `json:"mean_ns"`
	StdDev     time.Duration   `json:"stddev_ns"`
	Quantiles  []QuantileValue `json:"quantiles"`
}

// Snapshot projects the current histogram, reporting DefaultQuantiles.
func (r *Recorder) Snapshot() Snapshot { return r.SnapshotAt(DefaultQuantiles...) }

// SnapshotAt projects the current histogram at the caller's own quantiles,
// in the order given. An empty quantiles list falls back to
// DefaultQuantiles rather than reporting none.
func (r *Recorder) SnapshotAt(quantiles ...float64) Snapshot {
	if len(quantiles) == 0 {
		quantiles = DefaultQuantiles
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	snapshot := Snapshot{
		Name:       r.name,
		Count:      r.hist.TotalCount(),
		OutOfRange: r.outOfRange,
		Quantiles:  make([]QuantileValue, 0, len(quantiles)),
	}
	if snapshot.Count > 0 {
		snapshot.Min = time.Duration(r.hist.Min())
		snapshot.Max = time.Duration(r.hist.Max())
		snapshot.Mean = time.Duration(r.hist.Mean())
		snapshot.StdDev = time.Duration(r.hist.StdDev())
	}
	for _, q := range quantiles {
		value := time.Duration(0)
		if snapshot.Count > 0 {
			value = time.Duration(r.hist.ValueAtQuantile(q))
		}
		snapshot.Quantiles = append(snapshot.Quantiles, QuantileValue{Quantile: q, Value: value})
	}
	return snapshot
}

// ValueAt reports the duration at one quantile (0..100). It is the single
// lookup Budget evaluation needs; Snapshot is the reporting projection.
func (r *Recorder) ValueAt(quantile float64) time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.hist.TotalCount() == 0 {
		return 0
	}
	return time.Duration(r.hist.ValueAtQuantile(quantile))
}

// Budget is one declared release ceiling: the named quantity's Quantile
// must not exceed Max. Phase 11 planning owns the actual values; this type
// only gives them somewhere to live.
type Budget struct {
	Name     string        `json:"name"`
	Quantile float64       `json:"quantile"`
	Max      time.Duration `json:"max_ns"`
}

// BudgetResult is one evaluated Budget.
type BudgetResult struct {
	Budget   Budget        `json:"budget"`
	Observed time.Duration `json:"observed_ns"`
	Count    int64         `json:"count"`
	Pass     bool          `json:"pass"`
	// Reason is empty on a pass and otherwise names why the budget failed,
	// distinguishing a genuine overrun from an unevaluable one (no samples,
	// or a clamped upper tail).
	Reason string `json:"reason,omitempty"`
}

// Evaluate checks this Snapshot against a Budget. A budget naming a
// different quantity, a snapshot with no samples, and a snapshot whose
// upper tail was clamped all fail explicitly rather than passing by
// default -- absent evidence is never evidence of meeting a budget.
func (s Snapshot) Evaluate(b Budget) BudgetResult {
	result := BudgetResult{Budget: b, Count: s.Count}

	if b.Name != s.Name {
		result.Reason = fmt.Sprintf(
			"GOLC_MEASURE_BUDGET_MISMATCH: budget names %q but the snapshot measures %q",
			b.Name, s.Name,
		)
		return result
	}
	if s.Count == 0 {
		result.Reason = fmt.Sprintf(
			"GOLC_MEASURE_NO_SAMPLES: %q recorded no samples, so its budget is unevaluated, not met",
			s.Name,
		)
		return result
	}

	observed := time.Duration(0)
	found := false
	for _, q := range s.Quantiles {
		if q.Quantile == b.Quantile {
			observed = q.Value
			found = true
			break
		}
	}
	if !found {
		result.Reason = fmt.Sprintf(
			"GOLC_MEASURE_QUANTILE_UNREPORTED: %q reports no q%.4g -- snapshot it with SnapshotAt(%.4g)",
			s.Name, b.Quantile, b.Quantile,
		)
		return result
	}
	result.Observed = observed

	if s.OutOfRange > 0 {
		result.Reason = fmt.Sprintf(
			"GOLC_MEASURE_TAIL_CLAMPED: %d of %d samples exceeded the recorder's ceiling, so q%.4g is a lower bound, not a measurement",
			s.OutOfRange, s.Count, b.Quantile,
		)
		return result
	}
	if observed > b.Max {
		result.Reason = fmt.Sprintf(
			"GOLC_MEASURE_BUDGET_EXCEEDED: %q q%.4g was %s, over the %s budget",
			s.Name, b.Quantile, observed, b.Max,
		)
		return result
	}

	result.Pass = true
	return result
}
