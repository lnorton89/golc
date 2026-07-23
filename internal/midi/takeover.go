// Package midi implements the pure, device-independent MIDI logic for
// GOLC's operator surface: cross-to-catch soft takeover (CONTEXT.md
// D-09..D-12) and bounded learn capture + conflict rejection (D-05/D-06).
// Neither this file nor learn.go imports gomidi or any external MIDI
// library, and neither performs device I/O -- the live gomidi driver and
// device wiring are added by phase 06 plan 08. This isolates the phase's
// hardest correctness question (crossing vs. proximity, RESEARCH.md
// Pitfall 2) into fast, dependency-free unit tests.
package midi

import "math"

// TakeoverState implements cross-to-catch soft takeover
// (RESEARCH.md Pattern 4; CONTEXT.md D-09..D-12) for a single continuous
// CC/fader control. It applies ONLY to continuous controls: Note/button
// controls (D-12) never use TakeoverState at all -- they act immediately
// on press with no pickup/arming state, so a caller must branch on
// control kind before deciding whether to construct one.
//
// There is deliberately no proximity/threshold constant anywhere in this
// file (RESEARCH.md Pitfall 2). A control arms only when the physical
// value reaches or crosses AppValue in the direction of travel -- never
// merely when it gets close -- so `abs(physical-appValue) < threshold`
// must never appear here.
type TakeoverState struct {
	// Armed is true once the physical control has reached or crossed
	// AppValue at least once. While true, AppValue tracks the physical
	// value on every Update call (the control is "caught").
	Armed bool
	// AppValue is the authoritative, app-controlled value. While not
	// armed, it is the fixed ghost/target marker (D-10) the physical
	// control must cross. While armed, Update sets it to the physical
	// value on every call.
	AppValue float64
	// LastPhysical is the most recent raw physical value received. It
	// is published to the frontend as the live slider position even
	// while not armed (D-09), so the operator can see where the
	// hardware currently sits relative to the ghost marker.
	LastPhysical float64
}

// NewTakeoverState seeds a TakeoverState for a continuous control whose
// current app value is appValue, with Armed=false: a physical control
// must reach or cross appValue before it takes control.
//
// LastPhysical is deliberately seeded to NaN rather than appValue (or
// zero): a real prior physical reading does not exist yet, and NaN's
// IEEE 754 comparison semantics (every `NaN <= x` and `NaN >= x` is
// false) make Update's crossing check correctly evaluate to "no crossing
// yet" on the very first message, regardless of which side of appValue
// it lands on. Seeding LastPhysical to appValue itself would instead
// make the very first message always satisfy the crossing check
// trivially (LastPhysical <= AppValue and LastPhysical >= AppValue both
// hold at equality), spuriously arming on message one.
func NewTakeoverState(appValue float64) TakeoverState {
	return TakeoverState{Armed: false, AppValue: appValue, LastPhysical: math.NaN()}
}

// SetAppValue re-seeds AppValue (and therefore the ghost/target marker,
// D-10) when the app value changes from a source other than this
// control's own physical crossing -- e.g. a scene recall or another
// control setting the value directly -- so the physical control must
// cross the new target rather than a stale one.
//
// SetAppValue has no effect while Armed: an armed control's AppValue is
// driven by Update tracking the live physical position, and Armed only
// ever reflects this control's own prior crossing, not an external
// value change.
func (t *TakeoverState) SetAppValue(appValue float64) {
	if !t.Armed {
		t.AppValue = appValue
	}
}

// Update processes one incoming physical value for this continuous
// control (RESEARCH.md Pattern 4, implemented verbatim). It returns the
// current armed state and the value the control should apply
// (controlValue): the physical value once armed, or the fixed AppValue
// (ghost/target) while still not armed. A not-armed message never
// mutates AppValue -- its only effects are updating LastPhysical (so the
// live slider keeps tracking the physical position per D-09) and
// possibly satisfying the crossing check on this or a later call. An
// exact landing on AppValue counts as a crossing (arms), because the
// check compares with <= / >=, not strict < / >.
func (t *TakeoverState) Update(physical float64) (armed bool, controlValue float64) {
	if !t.Armed {
		crossedUp := t.LastPhysical <= t.AppValue && physical >= t.AppValue
		crossedDown := t.LastPhysical >= t.AppValue && physical <= t.AppValue
		if crossedUp || crossedDown {
			t.Armed = true
		}
	}
	t.LastPhysical = physical
	if t.Armed {
		t.AppValue = physical
	}
	return t.Armed, t.AppValue
}
