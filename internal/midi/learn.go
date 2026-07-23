package midi

import "fmt"

// MessageKind identifies whether a ControlKey refers to a Note message
// or a ControlChange (CC) message. It is part of a ControlKey's
// identity: a Note and a ControlChange sharing the same channel/number
// are distinct, independently mappable controls.
type MessageKind int

const (
	// Note identifies a MIDI Note-on message (a button/momentary
	// control). Note controls never use TakeoverState (D-12): they act
	// immediately on press.
	Note MessageKind = iota
	// ControlChange identifies a MIDI CC message (a continuous
	// fader/knob control). Only ControlChange controls are eligible
	// for soft takeover (D-12).
	ControlChange
)

// String renders a MessageKind for diagnostics.
func (k MessageKind) String() string {
	switch k {
	case Note:
		return "Note"
	case ControlChange:
		return "CC"
	default:
		return "Unknown"
	}
}

// ControlKey identifies a single physical MIDI control by its message
// identity: which MIDI channel, whether it's a Note or a ControlChange
// message, and its note/CC number. It is the live-side counterpart to
// the persisted mapping tuple (channel, kind, number) stored by
// operatorsurface.MidiMapping; the command/host layer maps between the
// two (06-03-PLAN.md <interfaces>) -- this package intentionally never
// imports operatorsurface, keeping the pure MIDI logic decoupled from
// persistence.
type ControlKey struct {
	Channel int
	Kind    MessageKind
	Number  int
}

// String renders a ControlKey for diagnostics, e.g. "channel 1 CC 74".
func (k ControlKey) String() string {
	return fmt.Sprintf("channel %d %s %d", k.Channel, k.Kind, k.Number)
}

// ProposeMapping checks candidate against every mapping in existing --
// the current surface's registered ControlKeys (D-07: mappings are
// per-surface, not global, so the caller passes exactly one surface's
// set) -- and returns nil if candidate collides with none of them.
//
// If candidate's (Channel, Kind, Number) tuple already appears in
// existing, ProposeMapping returns GOLC_MIDI_MAPPING_CONFLICT and never
// mutates existing (D-06: a colliding candidate is rejected outright;
// the prior mapping is left untouched, never silently overwritten and
// never offered a reassign-confirm path). Kind is part of a
// ControlKey's identity, so a Note and a ControlChange sharing the same
// channel/number never collide with each other.
func ProposeMapping(existing []ControlKey, candidate ControlKey) error {
	for _, mapped := range existing {
		if mapped == candidate {
			return fmt.Errorf("GOLC_MIDI_MAPPING_CONFLICT: note/CC %v is already mapped on this surface", candidate)
		}
	}
	return nil
}

// CaptureCandidate implements the bounded per-control learn capture
// window (D-05): the next ControlKey received on next becomes the
// mapping candidate. If timeout fires first, the capture window closes
// with GOLC_MIDI_LEARN_TIMEOUT instead of waiting indefinitely -- there
// is no unbounded wait path.
//
// next and timeout are supplied by the caller: phase 06 plan 08's live
// gomidi driver feeds next with decoded MIDI messages and a wall-clock
// timer feeds timeout, keeping this bounded-capture logic itself free of
// any external MIDI or timer dependency.
func CaptureCandidate(next <-chan ControlKey, timeout <-chan struct{}) (ControlKey, error) {
	select {
	case candidate := <-next:
		return candidate, nil
	case <-timeout:
		return ControlKey{}, fmt.Errorf("GOLC_MIDI_LEARN_TIMEOUT: no MIDI message received before the learn capture window closed")
	}
}
