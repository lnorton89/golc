// memory.go implements the cross-process memory evidence Phase 11 needs.
// golc-desktop is not one process: it supervises "golc-project artnet
// serve" (internal/wails/app.go), the Deno script host (internal/script),
// and midicat. A memory budget for "GOLC" is therefore a budget on a
// process TREE, which runtime.ReadMemStats cannot see at all and which
// internal/script's Job-Object watcher only sees for one script run.
//
// TreeSampler is the seam: SystemSampler reads real resident-set sizes via
// github.com/shirou/gopsutil/v4 (no cgo on any GOOS this repo builds for),
// and every test in this package drives a fake instead, so none of this
// needs a real process tree to be verifiable.
//
// PeakWatcher is shaped deliberately like internal/script's
// startMemoryWatch -- a ticker, a stop channel, a context, and an injected
// sampler -- because that shape is already proven testable here under
// testing/synctest, with no wall-clock sleeps at all.
package measure

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/process"
)

// maxTreeDepth bounds the supervised-tree walk. GOLC's real supervision
// depth is 2 (desktop -> artnet daemon / script host -> a Deno worker); the
// bound exists so a pathological or recursive process graph can never make
// a sampler run unbounded inside a soak harness.
const maxTreeDepth = 8

// ProcessMemory is one process's contribution to a tree sample.
type ProcessMemory struct {
	PID int32 `json:"pid"`
	// Name is best-effort: a process that exits between enumeration and
	// the name query contributes an empty Name rather than failing the
	// whole sample.
	Name string `json:"name"`
	// RSSBytes is resident set size -- physical memory actually held. It
	// is the number a release budget should be written against; VMS
	// (address space reserved) routinely runs an order of magnitude higher
	// on Windows and means very little.
	RSSBytes uint64 `json:"rss_bytes"`
	Depth    int    `json:"depth"`
}

// TreeMemory is one sample of a supervised process tree.
type TreeMemory struct {
	Root ProcessMemory `json:"root"`
	// Descendants is every reachable descendant that was still alive and
	// readable at sample time, ordered by (Depth, PID) so the JSON is
	// deterministic across runs.
	Descendants []ProcessMemory `json:"descendants"`
	// TotalRSSBytes is Root plus every entry in Descendants.
	TotalRSSBytes uint64 `json:"total_rss_bytes"`
	// Skipped counts processes that were enumerated but could not be read
	// -- almost always one that exited mid-walk, which is normal in a soak
	// and must not fail the sample. A persistently high Skipped instead
	// suggests a permissions problem worth investigating.
	Skipped int `json:"skipped"`
}

// TreeSampler samples one process tree's memory. Tests inject a fake;
// production uses SystemSampler.
type TreeSampler interface {
	SampleTree(pid int32) (TreeMemory, error)
}

// SystemSampler is the real, gopsutil-backed TreeSampler.
type SystemSampler struct{}

// SampleTree walks pid and its descendants, summing resident set size. A
// descendant that disappears mid-walk is counted in Skipped rather than
// failing the sample; only an unreadable ROOT is an error, since that means
// the caller asked about a process that is not there.
func (SystemSampler) SampleTree(pid int32) (TreeMemory, error) {
	root, err := process.NewProcess(pid)
	if err != nil {
		return TreeMemory{}, fmt.Errorf("GOLC_MEASURE_PROCESS_UNREADABLE: pid %d: %w", pid, err)
	}
	rootMemory, ok := readProcess(root, 0)
	if !ok {
		return TreeMemory{}, fmt.Errorf("GOLC_MEASURE_PROCESS_UNREADABLE: pid %d has no readable memory info", pid)
	}

	tree := TreeMemory{Root: rootMemory, TotalRSSBytes: rootMemory.RSSBytes}
	seen := map[int32]bool{pid: true}

	frontier := []*process.Process{root}
	for depth := 1; depth <= maxTreeDepth && len(frontier) > 0; depth++ {
		var next []*process.Process
		for _, parent := range frontier {
			children, err := parent.Children()
			if err != nil {
				// A leaf process reports an error rather than an empty
				// slice on several platforms; that is not a failure.
				continue
			}
			for _, child := range children {
				if seen[child.Pid] {
					continue
				}
				seen[child.Pid] = true
				memory, ok := readProcess(child, depth)
				if !ok {
					tree.Skipped++
					continue
				}
				tree.Descendants = append(tree.Descendants, memory)
				tree.TotalRSSBytes += memory.RSSBytes
				next = append(next, child)
			}
		}
		frontier = next
	}

	sortDescendants(tree.Descendants)
	return tree, nil
}

// readProcess reads one process's RSS and best-effort name. ok is false
// when the process has gone away or its memory info is unreadable.
func readProcess(p *process.Process, depth int) (ProcessMemory, bool) {
	info, err := p.MemoryInfo()
	if err != nil || info == nil {
		return ProcessMemory{}, false
	}
	name, err := p.Name()
	if err != nil {
		name = ""
	}
	return ProcessMemory{PID: p.Pid, Name: name, RSSBytes: info.RSS, Depth: depth}, true
}

// sortDescendants orders by (Depth, PID) so a TreeMemory's JSON encoding
// is stable across samples of the same tree.
func sortDescendants(descendants []ProcessMemory) {
	sort.Slice(descendants, func(i, j int) bool {
		if descendants[i].Depth != descendants[j].Depth {
			return descendants[i].Depth < descendants[j].Depth
		}
		return descendants[i].PID < descendants[j].PID
	})
}

// PeakSummary is a PeakWatcher's accumulated result.
type PeakSummary struct {
	Samples int `json:"samples"`
	// Failures counts sampling attempts that errored. A watcher keeps
	// polling through them: a transient read failure must not silently end
	// memory evidence collection halfway through a soak.
	Failures int `json:"failures"`
	// PeakTotalRSSBytes is the highest TotalRSSBytes observed, and PeakAt
	// is the sample that produced it -- kept whole so a report can name
	// which child process was responsible for the peak.
	PeakTotalRSSBytes uint64     `json:"peak_total_rss_bytes"`
	PeakAt            TreeMemory `json:"peak_at"`
	LastErr           string     `json:"last_error,omitempty"`
}

// PeakWatcher polls a process tree on a fixed interval and retains the
// highest total resident set observed -- the "memory budget" half of Phase
// 11's criterion 4. It is safe to read Summary concurrently with polling.
type PeakWatcher struct {
	sampler  TreeSampler
	pid      int32
	interval time.Duration

	mu      sync.Mutex
	summary PeakSummary
}

// NewPeakWatcher builds a watcher over sampler. A nil sampler defaults to
// SystemSampler so production callers need only supply a pid.
func NewPeakWatcher(sampler TreeSampler, pid int32, interval time.Duration) (*PeakWatcher, error) {
	if interval <= 0 {
		return nil, fmt.Errorf("GOLC_MEASURE_INTERVAL_INVALID: poll interval must be positive, got %s", interval)
	}
	if sampler == nil {
		sampler = SystemSampler{}
	}
	return &PeakWatcher{sampler: sampler, pid: pid, interval: interval}, nil
}

// Start begins polling in its own goroutine and returns a stop function.
// The goroutine ends on ctx cancellation or on stop, whichever comes
// first; stop is idempotent, mirroring internal/script's startMemoryWatch.
func (w *PeakWatcher) Start(ctx context.Context) func() {
	stopCh := make(chan struct{})
	var stopOnce sync.Once
	stop := func() { stopOnce.Do(func() { close(stopCh) }) }

	go func() {
		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-stopCh:
				return
			case <-ticker.C:
				w.sampleOnce()
			}
		}
	}()

	return stop
}

// sampleOnce takes one sample and folds it into the summary.
func (w *PeakWatcher) sampleOnce() {
	tree, err := w.sampler.SampleTree(w.pid)

	w.mu.Lock()
	defer w.mu.Unlock()
	w.summary.Samples++
	if err != nil {
		w.summary.Failures++
		w.summary.LastErr = err.Error()
		return
	}
	if tree.TotalRSSBytes > w.summary.PeakTotalRSSBytes {
		w.summary.PeakTotalRSSBytes = tree.TotalRSSBytes
		w.summary.PeakAt = tree
	}
}

// Summary reports the peak observed so far.
func (w *PeakWatcher) Summary() PeakSummary {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.summary
}

// MemoryBudget is one declared resident-set ceiling for a process tree.
type MemoryBudget struct {
	Name     string `json:"name"`
	MaxBytes uint64 `json:"max_bytes"`
}

// MemoryBudgetResult is one evaluated MemoryBudget.
type MemoryBudgetResult struct {
	Budget   MemoryBudget `json:"budget"`
	Observed uint64       `json:"observed_bytes"`
	Samples  int          `json:"samples"`
	Pass     bool         `json:"pass"`
	Reason   string       `json:"reason,omitempty"`
}

// ErrNoSamples reports that a summary carries no successful sample, so its
// budget is unevaluated rather than met.
var ErrNoSamples = errors.New("GOLC_MEASURE_NO_SAMPLES: no successful memory sample was taken")

// Evaluate checks this summary against a memory budget. As with the
// latency side, a summary with no successful samples fails explicitly --
// a soak that never managed to read memory has produced no evidence that
// the budget was met.
func (s PeakSummary) Evaluate(b MemoryBudget) MemoryBudgetResult {
	result := MemoryBudgetResult{Budget: b, Observed: s.PeakTotalRSSBytes, Samples: s.Samples}

	if s.Samples == 0 || s.Samples == s.Failures {
		result.Observed = 0
		result.Reason = fmt.Sprintf("%v (%d attempts, %d failed)", ErrNoSamples, s.Samples, s.Failures)
		return result
	}
	if s.PeakTotalRSSBytes > b.MaxBytes {
		result.Reason = fmt.Sprintf(
			"GOLC_MEASURE_BUDGET_EXCEEDED: %q peaked at %d bytes, over the %d-byte budget",
			b.Name, s.PeakTotalRSSBytes, b.MaxBytes,
		)
		return result
	}

	result.Pass = true
	return result
}
