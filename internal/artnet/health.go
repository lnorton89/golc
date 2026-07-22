// health.go implements ARTN-05's frame/target health model (04-03-
// PLAN.md Task 2, CONTEXT D-09/D-10/D-11/D-12): FrameHealth distinguishes
// a stalled playback engine from a healthy one by comparing the time
// since the worker's last recorded frame read against frameStaleAfter (a
// small multiple of workerTickHz's own period, D-09) -- a handful of
// missed ticks, not a single one, so ordinary scheduler jitter never
// falsely reports GOLC_ARTNET_FRAME_STALLED. TargetHealth accumulates
// per-(universe, target) send success/error counts and a Reachable flag
// (set once any send succeeds, D-10) so a target with only errors is
// distinguishable from one merely never yet sent to.
//
// All target tracking is bounded to the explicitly configured target set
// (T-04-04, Security Domain DoS bound): Configure(targets) establishes
// the allowed (universe, Target) key set once, up front; RecordSend for
// any key not in that set is silently dropped rather than allocating a
// new tracking entry, so a future unsolicited/unconfigured reachability
// signal (e.g. an ArtPollReply from an address the operator never
// configured) could never grow these maps unboundedly.
//
// Snapshot() is lock-free (mirrors internal/playback/engine.go's
// atomic.Pointer publish/read convention, extended here to two
// independently-updated fields): Targets is served from an
// atomic.Pointer[map[targetKey]TargetHealth] republished on every
// mutation, and frame staleness is evaluated live from an
// atomic.Int64-stored Unix-nanosecond timestamp -- a concurrent CLI/IPC
// status-read goroutine never takes the send-path's own mutex (D-11:
// persistent status readable without depending on log scrollback, and
// without ever contending with the hot send path).
package artnet

import (
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// frameStaleAfter is the frame-staleness threshold (D-09): several missed
// worker ticks (not one) distinguishes a genuinely stalled engine from an
// on-cadence one subject to ordinary scheduler jitter.
const frameStaleAfter = 10 * workerTickInterval

// artnetLogOutput is the structured D-11 log sink for every recorded
// error. Defaults to os.Stderr; worker_test.go/health_test.go (same
// package) may swap it for a buffer to assert on the emitted line
// deterministically.
var artnetLogOutput io.Writer = os.Stderr

// logArtnetError emits one structured {DOMAIN}_{CONDITION} log line
// (D-11): timestamp, code, universe/target, detail.
func logArtnetError(code string, universe int, target Target, err error) {
	fmt.Fprintf(artnetLogOutput, "%s code=%s universe=%d target=%s:%d detail=%v\n",
		time.Now().UTC().Format(time.RFC3339Nano), code, universe, target.IP, effectivePort(target), err)
}

// FrameHealth reports D-09's frame-health signal: the last time the
// worker recorded a frame read, and whether that read is still within
// frameStaleAfter of now.
type FrameHealth struct {
	LastFrameAt time.Time
	OnCadence   bool
}

// Err returns nil when OnCadence, or a GOLC_ARTNET_FRAME_STALLED
// diagnostic otherwise (mirrors InterfaceManager's Status()/Err() shape).
func (f FrameHealth) Err() error {
	if f.OnCadence {
		return nil
	}
	if f.LastFrameAt.IsZero() {
		return fmt.Errorf("GOLC_ARTNET_FRAME_STALLED: no frame has ever been recorded")
	}
	return fmt.Errorf("GOLC_ARTNET_FRAME_STALLED: no new frame since %s", f.LastFrameAt.UTC().Format(time.RFC3339Nano))
}

// evaluateFrameHealth classifies frame health at now given the last
// recorded frame-read time: on-cadence when the gap is within
// frameStaleAfter, stalled otherwise. A zero lastFrameAt (no frame ever
// recorded) is always stalled.
func evaluateFrameHealth(lastFrameAt, now time.Time) FrameHealth {
	if lastFrameAt.IsZero() {
		return FrameHealth{LastFrameAt: lastFrameAt, OnCadence: false}
	}
	return FrameHealth{LastFrameAt: lastFrameAt, OnCadence: now.Sub(lastFrameAt) <= frameStaleAfter}
}

// TargetHealth reports D-10's per-target health signal: accumulated send
// success/error counts plus a Reachable flag (set once any send
// succeeds, never reset by a later error) so a target with only errors
// and zero successes is distinguishable from one that has been reached
// at least once.
type TargetHealth struct {
	Universe  int
	Target    Target
	SendOK    int
	SendErr   int
	Reachable bool
	LastError string
}

// HealthSnapshot is the immutable, concurrently-readable status Snapshot
// publishes (D-11).
type HealthSnapshot struct {
	Frame   FrameHealth
	Targets map[targetKey]TargetHealth
}

// Health accumulates frame/target health (D-09/D-10) from the worker's
// concurrent tick/send goroutines. See package doc comment for the
// bounded-tracking and lock-free-read design.
type Health struct {
	lastFrameAtNano atomic.Int64
	targetsPtr      atomic.Pointer[map[targetKey]TargetHealth]

	mu         sync.Mutex
	targets    map[targetKey]TargetHealth
	configured map[targetKey]bool
}

// NewHealth returns a Health with no configured targets and no recorded
// frame yet (frame health starts stalled until the first RecordFrame).
func NewHealth() *Health {
	h := &Health{
		targets:    map[targetKey]TargetHealth{},
		configured: map[targetKey]bool{},
	}
	empty := map[targetKey]TargetHealth{}
	h.targetsPtr.Store(&empty)
	return h
}

// Configure declares targets (universe -> its fan-out target list) as
// the complete, explicitly configured target set this Health instance
// tracks (Security Domain T-04-04): RecordSend for any (universe,
// Target) key not present here is silently dropped, never allocating a
// new tracking entry. Safe to call again (e.g. after a reconfiguration)
// -- previously-tracked keys no longer present in targets are dropped
// from the configured set and will no longer accept new RecordSend
// updates, though their last-known TargetHealth entry is preserved in
// Targets for historical display rather than deleted outright.
//
// [04-05-PLAN.md Task 2 deviation, Rule 1 - bug]: an already-tracked key's
// stored Target snapshot (which carries Enabled) is refreshed on every
// call, not just the first -- otherwise a later "artnet target
// enable|disable" reconfigure (CONTEXT D-12) would never change what
// Snapshot() reports for that target's Enabled state, since Configure
// previously only wrote TargetHealth once, at first-configuration time.
// SendOK/SendErr/Reachable/LastError are left untouched so historical
// counters still survive a reconfigure exactly as documented above.
func (h *Health) Configure(targets map[int][]Target) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.configured = map[targetKey]bool{}
	for universe, ts := range targets {
		for _, t := range ts {
			key := keyOf(t)
			h.configured[key] = true
			existing, tracked := h.targets[key]
			if !tracked {
				h.targets[key] = TargetHealth{Universe: universe, Target: t}
				continue
			}
			existing.Universe = universe
			existing.Target = t
			h.targets[key] = existing
		}
	}
	h.publishTargetsLocked()
}

// RecordFrame records that the worker read a frame at readAt (D-09).
// Lock-free: stored as Unix nanoseconds via atomic.Int64 so Snapshot's
// frame-health read never takes h.mu.
func (h *Health) RecordFrame(readAt time.Time) {
	h.lastFrameAtNano.Store(readAt.UnixNano())
}

func (h *Health) loadLastFrameAt() time.Time {
	nano := h.lastFrameAtNano.Load()
	if nano == 0 {
		return time.Time{}
	}
	return time.Unix(0, nano)
}

// RecordSend records one send outcome for (universe, target) (D-10),
// then emits a structured GOLC_ARTNET_SEND_FAILED log line (D-11) when
// err is non-nil. Bounded to the configured target set (Security Domain
// T-04-04): a key never declared via Configure is silently dropped, never
// creating a new tracking entry.
func (h *Health) RecordSend(universe int, target Target, err error) {
	key := keyOf(target)

	h.mu.Lock()
	tracked := h.configured[key]
	if tracked {
		th := h.targets[key]
		th.Universe = universe
		th.Target = target
		if err != nil {
			th.SendErr++
			th.LastError = err.Error()
		} else {
			th.SendOK++
			th.Reachable = true
			th.LastError = ""
		}
		h.targets[key] = th
		h.publishTargetsLocked()
	}
	h.mu.Unlock()

	if err != nil {
		logArtnetError("GOLC_ARTNET_SEND_FAILED", universe, target, err)
	}
}

// RecordEncodeError emits a structured GOLC_ARTNET_ENCODE_FAILED log
// line for a universe-level encode failure (D-11). Encode errors are not
// target-specific, so they are logged rather than folded into
// TargetHealth.
func (h *Health) RecordEncodeError(universe int, err error) {
	logArtnetError("GOLC_ARTNET_ENCODE_FAILED", universe, Target{}, err)
}

// publishTargetsLocked republishes a fresh copy of h.targets into
// targetsPtr. Callers must hold h.mu.
func (h *Health) publishTargetsLocked() {
	fresh := make(map[targetKey]TargetHealth, len(h.targets))
	for k, v := range h.targets {
		fresh[k] = v
	}
	h.targetsPtr.Store(&fresh)
}

// Snapshot returns the current health status via non-blocking reads --
// safe to call from any goroutine (e.g. a concurrent CLI/IPC status
// handler) without ever taking the send path's own mutex.
func (h *Health) Snapshot() HealthSnapshot {
	ptr := h.targetsPtr.Load()
	targets := map[targetKey]TargetHealth{}
	if ptr != nil {
		targets = *ptr
	}
	return HealthSnapshot{
		Frame:   evaluateFrameHealth(h.loadLastFrameAt(), time.Now()),
		Targets: targets,
	}
}
