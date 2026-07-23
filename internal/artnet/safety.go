// safety.go implements PLAY-06/08/09's daemon-resident, in-memory
// local-priority safety and master-level overrides (06-02-PLAN.md Task 1,
// 06-RESEARCH.md Pattern 1): Blackout, Stop/Release-All, Revoke Automation,
// Grand Master, and group masters are never routed through show.Load/
// show.Save or any SQLite write -- they are atomic fields on a
// daemon-resident safetyState the Worker's tick goroutine reads
// lock-free every ~25ms (worker.go's non-blocking-tick doctrine), so a
// safety action takes effect on the very next tick regardless of disk
// latency, a hung IPC client, or a stalled automation runtime.
//
// blackout/stopAll/revokeAutomation are atomic.Bool: a single CPU-word
// read/write, no lock contention with the daemon's own d.mu (which guards
// unrelated target/worker-lifecycle state, daemon.go's own doc comment).
// masters is an atomic.Pointer[masterLevels] snapshot: grand master and
// every group master live together in one immutable struct swapped
// atomically, so the tick goroutine always observes one consistent set
// (T-06-06) -- never a half-mutated map read mid-write. Every setter below
// builds a fresh masterLevels (or a fresh groups map when only one group
// changes) and Stores it; a live snapshot already Loaded by a concurrent
// tick is never mutated in place (mirrors daemon.go's copyTargets
// copy-returning discipline).
//
// applyOverrides is the pure per-frame transform worker.go's tick() calls
// immediately before the per-universe Encode loop (06-PATTERNS.md): it
// never mutates the caller's Frame or its Values map, always returning a
// fresh Frame, so tick()'s own frame pointer (shared with other readers
// via playback.Engine.CurrentFrame()) is never aliased or corrupted.
package artnet

import (
	"fmt"
	"sync/atomic"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/fixture"
	"github.com/lnorton89/golc/internal/playback"
	"github.com/lnorton89/golc/internal/scene"
)

// masterLevels is an immutable snapshot of the grand master level plus
// every group's own master level, swapped atomically via safetyState's
// masters field (Pattern 1) so the tick goroutine always reads one
// consistent set, never a half-mutated map (T-06-06). A masterLevels value
// is never mutated after being Stored -- every setter method builds and
// Stores a fresh one instead.
type masterLevels struct {
	grand  float64
	groups map[uuid.UUID]float64
}

// identityMasterLevels returns the identity master snapshot: grand master
// 1.0 and no group overrides -- scaling a value by these levels changes
// nothing (the "all overrides at identity" behavior applyOverrides must
// preserve exactly).
func identityMasterLevels() *masterLevels {
	return &masterLevels{grand: 1.0, groups: map[uuid.UUID]float64{}}
}

// safetyState holds every daemon-resident safety/master override flag the
// Worker's tick goroutine reads lock-free each tick (RESEARCH.md Pattern
// 1). blackout/stopAll/revokeAutomation are independent atomic.Bool
// flags -- blackout and stopAll both drive output to the safe/zero state
// (applyOverrides treats them identically) but are tracked separately so
// each has its own IPC route and CLI trigger and its own diagnostic
// identity. revokeAutomation is read by revokeActive() (Task 2's
// command-source gate); it does not itself affect applyOverrides' output
// scaling -- it blocks non-manual-source commands at the daemon's request
// dispatch, a separate enforcement point from the per-frame transform.
type safetyState struct {
	blackout         atomic.Bool
	stopAll          atomic.Bool
	revokeAutomation atomic.Bool
	masters          atomic.Pointer[masterLevels]
}

// newSafetyState returns a safetyState at its identity values: no
// blackout, no stop-all, automation not revoked, grand master 1.0, and no
// group master overrides.
func newSafetyState() *safetyState {
	s := &safetyState{}
	s.masters.Store(identityMasterLevels())
	return s
}

// currentMasters returns s's current masterLevels snapshot, falling back
// to the identity snapshot if s or its masters pointer was never
// initialized (defensive: a zero-value safetyState, e.g. from a test
// constructing one directly rather than via newSafetyState, must still
// behave as identity rather than panicking on a nil dereference).
func (s *safetyState) currentMasters() *masterLevels {
	if s == nil {
		return identityMasterLevels()
	}
	if m := s.masters.Load(); m != nil {
		return m
	}
	return identityMasterLevels()
}

// setBlackout sets the blackout flag. Read lock-free by the next Worker
// tick -- takes effect within one tick period, no Worker restart.
func (s *safetyState) setBlackout(on bool) {
	s.blackout.Store(on)
}

// setStopAll sets the stop/release-all flag. Read lock-free by the next
// Worker tick -- takes effect within one tick period, no Worker restart.
func (s *safetyState) setStopAll(on bool) {
	s.stopAll.Store(on)
}

// setRevokeAutomation sets the revoke-automation flag (PLAY-08): once on,
// revokeActive() reports true so the daemon's command dispatch (Task 2)
// can reject any forwarded Request carrying a non-manual source tag,
// without waiting for the automation runtime to acknowledge or respond.
func (s *safetyState) setRevokeAutomation(on bool) {
	s.revokeAutomation.Store(on)
}

// revokeActive reports whether Revoke Automation is currently active
// (PLAY-08). Read lock-free.
func (s *safetyState) revokeActive() bool {
	if s == nil {
		return false
	}
	return s.revokeAutomation.Load()
}

// validateMasterLevel rejects a level outside [0,1] as
// GOLC_ARTNET_SAFETY_MASTER_INVALID -- this repo's ExitCode-1 domain-error
// convention (Task 2/3 map this to ipc.Result{ExitCode: 1}).
func validateMasterLevel(level float64) error {
	if level < 0 || level > 1 {
		return fmt.Errorf("GOLC_ARTNET_SAFETY_MASTER_INVALID: master level %v must be within [0,1]", level)
	}
	return nil
}

// setGrandMaster replaces the grand master level, preserving every
// existing group master (the groups map itself is reused by reference --
// safe because a Stored masterLevels' groups map is never mutated in
// place, only ever replaced wholesale by setGroupMaster below, so sharing
// the reference across snapshots cannot race).
func (s *safetyState) setGrandMaster(level float64) error {
	if err := validateMasterLevel(level); err != nil {
		return err
	}
	current := s.currentMasters()
	s.masters.Store(&masterLevels{grand: level, groups: current.groups})
	return nil
}

// setGroupMaster replaces groupID's master level, preserving the current
// grand master and every other group's level. Builds a fresh groups map
// (copy-returning discipline) rather than mutating the live snapshot's
// map in place.
func (s *safetyState) setGroupMaster(groupID uuid.UUID, level float64) error {
	if err := validateMasterLevel(level); err != nil {
		return err
	}
	current := s.currentMasters()
	groups := make(map[uuid.UUID]float64, len(current.groups)+1)
	for id, lvl := range current.groups {
		groups[id] = lvl
	}
	groups[groupID] = level
	s.masters.Store(&masterLevels{grand: current.grand, groups: groups})
	return nil
}

// clampUnit clamps v to [0,1] (applyOverrides' own output-level clamp: a
// composed grand x group multiplier can never drive a programmed value
// outside the valid normalized range).
func clampUnit(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// effectiveMultiplier composes instanceID's grand master and every group
// master for the groups instanceID belongs to (per membership), always
// multiplicatively, never additively (PLAY-06 adjacency: group=0.5,
// grand=0.5 -> 0.25, not 0.0 or 1.0).
func effectiveMultiplier(instanceID uuid.UUID, masters *masterLevels, membership map[uuid.UUID][]uuid.UUID) float64 {
	multiplier := masters.grand
	for _, groupID := range membership[instanceID] {
		if level, ok := masters.groups[groupID]; ok {
			multiplier *= level
		}
	}
	return multiplier
}

// zeroedAttributeSet returns a fresh AttributeSet with attrs'
// CapabilityIntensity value forced to 0 and every other attribute
// preserved unchanged -- blackout/stop-all only zero intensity (the
// channel that gates visible output), never overwrite color/position/beam
// programming, so a look is not destructively lost while blacked out.
func zeroedAttributeSet(attrs scene.AttributeSet) scene.AttributeSet {
	values := make(map[fixture.CapabilityType]float64, len(attrs.Values))
	for capability, value := range attrs.Values {
		if capability == fixture.CapabilityIntensity {
			values[capability] = 0
			continue
		}
		values[capability] = value
	}
	return scene.AttributeSet{Values: values}
}

// scaledAttributeSet returns a fresh AttributeSet with attrs'
// CapabilityIntensity value scaled by multiplier and clamped to [0,1];
// every other attribute is preserved unchanged.
func scaledAttributeSet(attrs scene.AttributeSet, multiplier float64) scene.AttributeSet {
	values := make(map[fixture.CapabilityType]float64, len(attrs.Values))
	for capability, value := range attrs.Values {
		if capability == fixture.CapabilityIntensity {
			values[capability] = clampUnit(value * multiplier)
			continue
		}
		values[capability] = value
	}
	return scene.AttributeSet{Values: values}
}

// applyOverrides is the pure, side-effect-free transform worker.go's
// tick() applies to frame immediately before the per-universe Encode loop
// (06-PATTERNS.md's exact insertion point). It never mutates frame or its
// Values map -- every returned Frame carries a freshly built Values map,
// even when no attribute actually changes (the identity-overrides case),
// so tick()'s own frame pointer (shared with other CurrentFrame() readers)
// is never aliased.
//
// blackout or stopAll (either one) drives every instance's intensity to 0,
// regardless of input -- both are safety states that always win over
// master scaling. An empty input Frame (nothing playing) is a safe no-op:
// there are no instances to zero, and downstream Encode already produces
// an all-zero universe buffer for any universe with no instance entries
// (worker.go tick()'s own `data, ok := buffers[u]; if !ok { data =
// make([]byte, channelsPerUniverse) }` default), so the empty case never
// needs special-cased zero-instance synthesis here (PLAY-06 empty edge).
//
// Otherwise, each instance's intensity is scaled by effectiveMultiplier:
// grand master times every group master for the groups membership says
// that instance belongs to, multiplicative, never additive (PLAY-06
// adjacency: group=0.5, grand=0.5, programmed=full -> 0.25). A nil s or a
// nil/empty membership behaves as identity (grand=1.0, no groups), so an
// unconfigured Worker (e.g. one built by existing tests with no Safety
// field) sees no behavior change from before this transform was
// introduced.
func applyOverrides(frame playback.Frame, s *safetyState, membership map[uuid.UUID][]uuid.UUID) playback.Frame {
	if len(frame.Values) == 0 {
		return playback.Frame{}
	}

	blackout := false
	if s != nil {
		blackout = s.blackout.Load() || s.stopAll.Load()
	}
	masters := s.currentMasters()

	out := make(map[uuid.UUID]scene.AttributeSet, len(frame.Values))
	for instanceID, attrs := range frame.Values {
		if blackout {
			out[instanceID] = zeroedAttributeSet(attrs)
			continue
		}
		multiplier := effectiveMultiplier(instanceID, masters, membership)
		out[instanceID] = scaledAttributeSet(attrs, multiplier)
	}
	return playback.Frame{Values: out}
}
