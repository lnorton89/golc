package deskmidi

import (
	"fmt"

	"github.com/google/uuid"
)

// MidiMessageKind names the two generic MIDI message types a desk mapping
// can learn -- mirrors operatorsurface.MidiMessageKind's wire values
// exactly (this package duplicates the tiny enum rather than importing
// operatorsurface, see doc.go).
type MidiMessageKind string

const (
	Note          MidiMessageKind = "note"
	ControlChange MidiMessageKind = "control_change"
)

// Mapping binds one incoming MIDI Note/CC message (Channel, Kind, Number)
// directly to one Desk fader, identified by InstanceID (a patch instance
// ID, internal/wails/svc_desk.go's own addressing) and Capability (a
// fixture.CapabilityType wire value, e.g. "intensity"). ID is minted once
// by AddMapping and never derived from the mapping's own content --
// mirrors operatorsurface.MidiMapping's identity discipline.
type Mapping struct {
	ID         uuid.UUID       `json:"id"`
	Channel    int             `json:"channel"`
	Kind       MidiMessageKind `json:"kind"`
	Number     int             `json:"number"`
	InstanceID string          `json:"instance_id"`
	Capability string          `json:"capability"`
}

// AddMapping mints a fresh UUIDv7 for candidate and appends it to existing,
// unless a mapping already in existing shares candidate's (Channel, Kind,
// Number) tuple -- in which case AddMapping rejects the candidate outright
// with GOLC_DESKMIDI_MAPPING_CONFLICT and returns existing unmodified:
// mirrors operatorsurface.AddMidiMapping's D-06 discipline exactly (never a
// silent overwrite, never last-writer-wins).
func AddMapping(existing []Mapping, candidate Mapping) ([]Mapping, error) {
	for _, m := range existing {
		if m.Channel == candidate.Channel && m.Kind == candidate.Kind && m.Number == candidate.Number {
			return nil, fmt.Errorf(
				"GOLC_DESKMIDI_MAPPING_CONFLICT: channel=%d kind=%s number=%d is already mapped to a Desk fader",
				candidate.Channel, candidate.Kind, candidate.Number)
		}
	}
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("GOLC_DESKMIDI_ID_MINT_FAILED: %v", err)
	}
	candidate.ID = id

	clone := append([]Mapping(nil), existing...)
	clone = append(clone, candidate)
	return clone, nil
}

// RemoveMapping returns a copy of existing with the mapping whose ID
// matches mappingID removed. Removing a mapping not present is a no-op,
// mirroring operatorsurface.RemoveMidiMapping's idempotent-if-absent
// discipline.
func RemoveMapping(existing []Mapping, mappingID uuid.UUID) []Mapping {
	filtered := make([]Mapping, 0, len(existing))
	for _, m := range existing {
		if m.ID != mappingID {
			filtered = append(filtered, m)
		}
	}
	return filtered
}
