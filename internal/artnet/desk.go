// desk.go implements a QLC+-style "Simple Desk" manual override layer: a
// daemon-resident, atomically-published per-instance capability override
// map the Worker's tick goroutine composites onto the scene-derived Frame
// immediately BEFORE applyOverrides' own safety/master transform (safety.go)
// -- so Blackout/Stop-All/master scaling always wins over a manual desk
// fader, exactly like every other safety invariant in this daemon. Unlike
// safetyState's single-writer-per-field atomics, deskState's overrides map
// is a compound structure mutated by read-modify-write, so every mutating
// method holds mu for its full read-copy-store sequence (ipc/server.go's
// Serve dispatches each accepted connection on its own goroutine, so two
// concurrent "artnet desk set" calls are a real possibility, not a
// theoretical one) -- the tick goroutine's own read path
// (currentOverrides) stays lock-free via the atomic.Pointer, mirroring
// safetyState.masters' publish/read discipline.
//
// A desk override targets a (instance, CapabilityType) pair, mirroring
// scene.AttributeSet's own one-value-per-capability-type shape (and
// therefore channelmap.go's existing Occurrence-blind lookup convention --
// see Encode's own doc comment): this is not a new addressing scheme, just
// this package's first writer of that same per-instance capability map
// outside the scene/playback pipeline.
package artnet

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/fixture"
	"github.com/lnorton89/golc/internal/playback"
	"github.com/lnorton89/golc/internal/scene"
)

// deskState holds every manually-overridden (instance, capability) value
// the Worker's tick goroutine reads lock-free each tick via
// currentOverrides. mu serializes the read-copy-store sequence every
// mutating method performs; overrides itself is only ever read via the
// atomic.Pointer outside of a method already holding mu.
type deskState struct {
	mu        sync.Mutex
	overrides atomic.Pointer[map[uuid.UUID]scene.AttributeSet]
}

// newDeskState returns a deskState with no overrides active -- identical to
// the never-touched Desk fader position for every fixture.
func newDeskState() *deskState {
	d := &deskState{}
	empty := map[uuid.UUID]scene.AttributeSet{}
	d.overrides.Store(&empty)
	return d
}

// currentOverrides returns the current override map via a lock-free atomic
// Load -- safe to call from the tick goroutine every ~25ms without ever
// contending with a concurrent "artnet desk ..." mutation. A nil receiver
// (an unconfigured Worker, mirroring safetyState's own nil-receiver
// defaults) returns nil, which applyDeskOverrides already treats as
// "nothing overridden."
func (d *deskState) currentOverrides() map[uuid.UUID]scene.AttributeSet {
	if d == nil {
		return nil
	}
	if p := d.overrides.Load(); p != nil {
		return *p
	}
	return nil
}

// validateDeskValue rejects a value outside [0,1] as
// GOLC_ARTNET_DESK_VALUE_INVALID -- desk overrides carry the same
// protocol-agnostic normalized range every other capability value in this
// codebase does (fixture.Capability.Range's own doc comment).
func validateDeskValue(value float64) error {
	if value < 0 || value > 1 {
		return fmt.Errorf("GOLC_ARTNET_DESK_VALUE_INVALID: value %v must be within [0,1]", value)
	}
	return nil
}

// validateDeskCapability rejects any CapabilityType outside
// fixture.SupportedCapabilityTypes' declared enum as
// GOLC_ARTNET_DESK_CAPABILITY_INVALID.
func validateDeskCapability(capType fixture.CapabilityType) error {
	for _, supported := range fixture.SupportedCapabilityTypes {
		if capType == supported {
			return nil
		}
	}
	return fmt.Errorf("GOLC_ARTNET_DESK_CAPABILITY_INVALID: %q is not a supported capability type", capType)
}

// copyOverrides returns a fresh top-level copy of current (copy-returning
// discipline, mirrors deployment/model.go's own convention): the per-
// instance AttributeSet values themselves are reused by reference where
// untouched, since a Stored AttributeSet is never mutated in place, only
// ever replaced wholesale.
func copyOverrides(current map[uuid.UUID]scene.AttributeSet) map[uuid.UUID]scene.AttributeSet {
	fresh := make(map[uuid.UUID]scene.AttributeSet, len(current)+1)
	for id, attrs := range current {
		fresh[id] = attrs
	}
	return fresh
}

// setAttribute records instanceID's capType override at value, taking
// effect on the Worker's very next tick with no restart. Validates value
// and capType before ever touching d's state, so a rejected call leaves
// every existing override completely untouched.
func (d *deskState) setAttribute(instanceID uuid.UUID, capType fixture.CapabilityType, value float64) error {
	if err := validateDeskCapability(capType); err != nil {
		return err
	}
	if err := validateDeskValue(value); err != nil {
		return err
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	fresh := copyOverrides(d.currentOverrides())
	existing := fresh[instanceID]
	values := make(map[fixture.CapabilityType]float64, len(existing.Values)+1)
	for k, v := range existing.Values {
		values[k] = v
	}
	values[capType] = value
	fresh[instanceID] = scene.AttributeSet{Values: values}
	d.overrides.Store(&fresh)
	return nil
}

// clearAttribute releases instanceID's capType override, if any, reverting
// that one capability to whatever the active scene (or nothing) drives for
// it. Every other override on instanceID, and every other instance's
// overrides, are left untouched. A no-op (instanceID has no override, or no
// override for capType specifically) never mutates d's published state.
func (d *deskState) clearAttribute(instanceID uuid.UUID, capType fixture.CapabilityType) {
	d.mu.Lock()
	defer d.mu.Unlock()

	current := d.currentOverrides()
	existing, ok := current[instanceID]
	if !ok {
		return
	}
	if _, has := existing.Values[capType]; !has {
		return
	}

	fresh := copyOverrides(current)
	values := make(map[fixture.CapabilityType]float64, len(existing.Values))
	for k, v := range existing.Values {
		if k == capType {
			continue
		}
		values[k] = v
	}
	if len(values) == 0 {
		delete(fresh, instanceID)
	} else {
		fresh[instanceID] = scene.AttributeSet{Values: values}
	}
	d.overrides.Store(&fresh)
}

// clearInstance releases every override on instanceID at once (every other
// instance's overrides are left untouched). A no-op (instanceID has no
// override at all) never mutates d's published state.
func (d *deskState) clearInstance(instanceID uuid.UUID) {
	d.mu.Lock()
	defer d.mu.Unlock()

	current := d.currentOverrides()
	if _, ok := current[instanceID]; !ok {
		return
	}
	fresh := make(map[uuid.UUID]scene.AttributeSet, len(current))
	for id, attrs := range current {
		if id == instanceID {
			continue
		}
		fresh[id] = attrs
	}
	d.overrides.Store(&fresh)
}

// clearAll releases every desk override across every instance at once (the
// desk-cluster equivalent of "release all faders to zero/passthrough").
func (d *deskState) clearAll() {
	d.mu.Lock()
	defer d.mu.Unlock()
	empty := map[uuid.UUID]scene.AttributeSet{}
	d.overrides.Store(&empty)
}

// applyDeskOverrides is the pure, side-effect-free transform worker.go's
// tick() applies to the scene-derived Frame immediately BEFORE
// applyOverrides' own safety/master transform (safety.go's doc comment: a
// Blackout or master scale must always win over a manual desk fader, never
// the other way around). It never mutates frame or its Values map -- every
// returned Frame carries a freshly built Values map, mirroring
// applyOverrides' own aliasing discipline.
//
// An overridden instance absent from frame.Values entirely (no scene
// active, or the instance simply isn't targeted by the current scene) gets
// a synthesized entry carrying only its overridden capabilities -- a desk
// fader must be able to drive a fixture with nothing programmed at all,
// exactly like a real lighting console's manual desk. Every other instance
// already present in frame.Values is left byte-for-byte unchanged when it
// has no override.
func applyDeskOverrides(frame playback.Frame, overrides map[uuid.UUID]scene.AttributeSet) playback.Frame {
	if len(overrides) == 0 {
		return frame
	}

	out := make(map[uuid.UUID]scene.AttributeSet, len(frame.Values)+len(overrides))
	for id, attrs := range frame.Values {
		out[id] = attrs
	}
	for id, override := range overrides {
		out[id] = out[id].Overlay(override)
	}
	return playback.Frame{Values: out}
}
