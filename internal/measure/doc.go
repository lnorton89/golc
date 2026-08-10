// Package measure provides the two measurement primitives Phase 11
// (Windows Release Qualification) needs to make its success criterion 4
// checkable rather than asserted: percentile-accurate timing evidence and
// cross-process memory evidence.
//
// That criterion reads "Long-running tests with real Art-Net hardware meet
// defined playback cadence, Art-Net timing, override latency, memory, and
// soak budgets while UI, storage, scripts, API clients, and LLM work run
// concurrently or fail." Two properties of that sentence drive this
// package's whole design:
//
//   - "budgets" over a "long-running" soak means percentiles, not means. A
//     mean tick interval says nothing about the one frame in ten thousand
//     that arrived 80ms late, and that frame is exactly what an operator
//     sees. Recorder is backed by an HDR histogram
//     (github.com/HdrHistogram/hdrhistogram-go), which holds a configurable
//     precision across a wide dynamic range in fixed memory -- so a
//     multi-hour soak costs the same bytes as a one-minute run, unlike a
//     growing slice of samples.
//
//   - "memory" in an application that supervises child processes
//     (golc-project artnet serve, the Deno script host, midicat) is not
//     runtime.ReadMemStats. It is the resident set of a process tree.
//     TreeSampler is backed by github.com/shirou/gopsutil/v4, which reads
//     that per-process without cgo on every GOOS this repository builds for
//     -- consistent with the pure-Go modernc.org/sqlite choice.
//
// Nothing here runs on, or synchronizes with, the deterministic output
// path. A Recorder is a passive sink a caller pushes an already-measured
// duration into; it never reads a clock itself, so it cannot perturb the
// cadence it is measuring, and a test can drive it with synthetic values.
// The one component that owns a clock (PeakWatcher) is a supervisory
// polling loop shaped exactly like internal/script's startMemoryWatch, and
// like that one it is fully testable on any GOOS against an injected fake.
//
// Scope note: this package is deliberately measurement only. Deciding
// which quantities to record, at what cadence, against what ceilings, and
// how the resulting evidence is archived for a release is Phase 11
// planning's job -- see .planning/ROADMAP.md's Phase 11 research note.
// Budget/BudgetResult exist so that planning has a declarative shape to
// express those ceilings in, not because this package presumes any
// particular value.
package measure
