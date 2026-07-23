// model.go declares the operator-surface domain model (CONTEXT PLAY-03,
// D-01/D-02/D-03/D-06/D-07): a Surface is a named, individually-assigned
// constrained playback surface -- an author builds one or more per show
// (D-02) and assigns individual scenes, layers, masters, and safety
// controls to it one at a time (D-01/D-03: no group/category-level
// bulk-assign path exists anywhere in this package). Each Surface also
// carries its own independent MIDI Note/CC mapping set (D-07), never a
// show-global mapping table. Surface copies internal/pool/model.go's
// identity/construction/rename shape verbatim (06-PATTERNS.md): identity
// is a durable UUIDv7 minted once at creation, never derived from Name,
// and never re-minted by Rename. Every mutator here returns a fresh copy
// (internal/artnet/daemon.go's copyTargets discipline) -- the caller's own
// slices are never aliased -- and every assignment mutator is idempotent:
// assigning an already-assigned item is a no-op, mirroring PLAY-03's
// idempotency edge. AddMidiMapping is the one exception to "idempotent
// no-op": a colliding (channel, kind, number) tuple is a hard rejection
// (GOLC_OPERATORSURFACE_MIDI_CONFLICT, D-06) that leaves the existing
// mapping untouched -- never a silent overwrite, never last-writer-wins.
package operatorsurface

import (
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/scene"
)

// MasterKind names which kind of master control a MasterRef selects
// (CONTEXT D-01): GrandMaster is the single show-wide master (its GroupID
// is always the zero uuid.UUID); GroupMaster selects one specific group's
// master, identified by GroupID.
type MasterKind string

const (
	GrandMaster MasterKind = "grand_master"
	GroupMaster MasterKind = "group_master"
)

// SafetyControl names one of the three fixed, independent operator safety
// actions (CONTEXT "Revoke Automation is the independent operator safety
// action" / PROJECT.md Context): this is a closed enum -- a SafetyRef
// never dangles because there is no external collection it could
// reference (internal/operatorsurface/validate.go's Validate never needs
// to resolve it against anything).
type SafetyControl string

const (
	Blackout         SafetyControl = "blackout"
	StopReleaseAll   SafetyControl = "stop_release_all"
	RevokeAutomation SafetyControl = "revoke_automation"
)

// MidiMessageKind names the two generic MIDI message types an operator
// surface's mappings can learn (CONTEXT: "generic MIDI Note/CC learn").
type MidiMessageKind string

const (
	Note          MidiMessageKind = "note"
	ControlChange MidiMessageKind = "control_change"
)

// LayerRef selects one of a scene's four fixed layer slots (scene.Layer's
// own identity is (scene ID, LayerKind) -- a Layer carries no independent
// UUID of its own, so a LayerRef must carry both fields to be resolvable).
type LayerRef struct {
	SceneID uuid.UUID       `json:"scene_id"`
	Kind    scene.LayerKind `json:"kind"`
}

// MasterRef selects either the show-wide grand master (Kind=GrandMaster,
// GroupID always the zero value) or one specific group's master
// (Kind=GroupMaster, GroupID identifies the pool.Group).
type MasterRef struct {
	Kind    MasterKind `json:"kind"`
	GroupID uuid.UUID  `json:"group_id,omitempty"`
}

// ControlKind discriminates which of ControlRef's four value fields is
// meaningful.
type ControlKind string

const (
	ControlScene  ControlKind = "scene"
	ControlLayer  ControlKind = "layer"
	ControlMaster ControlKind = "master"
	ControlSafety ControlKind = "safety"
)

// ControlRef names exactly one individually-assignable control by
// discriminated union (D-03: individual-item granularity only -- there is
// deliberately no bulk/category variant here or anywhere else in this
// package). Kind selects which of Scene/Layer/Master/Safety is populated;
// the other three fields are the type's zero value and are not
// meaningful. A MidiMapping's Target and every server-side Authorize call
// (internal/command/operatorsurface.go) both reuse this exact shape --
// there is only one control-identity representation in this codebase, not
// one per call site.
type ControlRef struct {
	Kind   ControlKind   `json:"kind"`
	Scene  uuid.UUID     `json:"scene,omitempty"`
	Layer  LayerRef      `json:"layer,omitempty"`
	Master MasterRef     `json:"master,omitempty"`
	Safety SafetyControl `json:"safety,omitempty"`
}

// SceneControlRef, LayerControlRef, MasterControlRef, and SafetyControlRef
// construct a ControlRef of the named kind -- the only supported way to
// build one, so a ControlRef can never end up with more than one of its
// value fields meaningfully populated.
func SceneControlRef(sceneID uuid.UUID) ControlRef {
	return ControlRef{Kind: ControlScene, Scene: sceneID}
}
func LayerControlRef(ref LayerRef) ControlRef   { return ControlRef{Kind: ControlLayer, Layer: ref} }
func MasterControlRef(ref MasterRef) ControlRef { return ControlRef{Kind: ControlMaster, Master: ref} }
func SafetyControlRef(sc SafetyControl) ControlRef {
	return ControlRef{Kind: ControlSafety, Safety: sc}
}

// layerRefEqual reports whether a and b select the same scene layer slot.
func layerRefEqual(a, b LayerRef) bool {
	return a.SceneID == b.SceneID && a.Kind == b.Kind
}

// masterRefEqual reports whether a and b select the same master control.
func masterRefEqual(a, b MasterRef) bool {
	return a.Kind == b.Kind && a.GroupID == b.GroupID
}

// ControlRefEqual reports whether a and b name the identical control --
// used by AddMidiMapping's Target bookkeeping and by
// internal/command/operatorsurface.go's Authorize to compare a requested
// control against a surface's assignment set.
func ControlRefEqual(a, b ControlRef) bool {
	if a.Kind != b.Kind {
		return false
	}
	switch a.Kind {
	case ControlScene:
		return a.Scene == b.Scene
	case ControlLayer:
		return layerRefEqual(a.Layer, b.Layer)
	case ControlMaster:
		return masterRefEqual(a.Master, b.Master)
	case ControlSafety:
		return a.Safety == b.Safety
	default:
		return false
	}
}

// MidiMapping binds one incoming MIDI Note/CC message (Channel, Kind,
// Number) to one control on the surface that owns it (D-07: mappings are
// per-surface, never global). ID is minted once by AddMidiMapping and
// never derived from the mapping's own content.
type MidiMapping struct {
	ID      uuid.UUID       `json:"id"`
	Channel int             `json:"channel"`
	Kind    MidiMessageKind `json:"kind"`
	Number  int             `json:"number"`
	Target  ControlRef      `json:"target"`
}

// Surface is a named, individually-assigned constrained playback surface
// (CONTEXT PLAY-03, D-01/D-02): an author may create any number of
// independently named surfaces on one show (D-02); each carries its own
// individual-item assignment sets (D-01/D-03 -- no bulk/category ref type
// exists) and its own independent MIDI mapping set (D-07). Identity is a
// durable UUIDv7 minted once at creation -- never derived from Name, and
// never re-minted by Rename.
type Surface struct {
	ID           uuid.UUID       `json:"id"`
	Name         string          `json:"name"`
	SceneRefs    []uuid.UUID     `json:"scene_refs,omitempty"`
	LayerRefs    []LayerRef      `json:"layer_refs,omitempty"`
	MasterRefs   []MasterRef     `json:"master_refs,omitempty"`
	SafetyRefs   []SafetyControl `json:"safety_refs,omitempty"`
	MidiMappings []MidiMapping   `json:"midi_mappings,omitempty"`
}

// cloneSurface returns a Surface carrying entirely fresh backing slices,
// so every mutator below can build its result from clone without ever
// aliasing the caller's own slices (mirrors internal/artnet/daemon.go's
// copyTargets discipline).
func cloneSurface(s Surface) Surface {
	clone := s
	clone.SceneRefs = append([]uuid.UUID(nil), s.SceneRefs...)
	clone.LayerRefs = append([]LayerRef(nil), s.LayerRefs...)
	clone.MasterRefs = append([]MasterRef(nil), s.MasterRefs...)
	clone.SafetyRefs = append([]SafetyControl(nil), s.SafetyRefs...)
	clone.MidiMappings = append([]MidiMapping(nil), s.MidiMappings...)
	return clone
}

// NewSurface mints a fresh UUIDv7-identified, empty Surface. IDs are
// minted only at creation time -- never derived from Name, and never
// re-minted by Rename.
func NewSurface(name string) (Surface, error) {
	if strings.TrimSpace(name) == "" {
		return Surface{}, fmt.Errorf("GOLC_OPERATORSURFACE_NAME_EMPTY: operator surface name must not be empty")
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Surface{}, fmt.Errorf("GOLC_OPERATORSURFACE_ID_MINT_FAILED: %v", err)
	}
	return Surface{ID: id, Name: name}, nil
}

// Rename returns s with Name replaced by newName; ID is never re-minted
// (identity is rename-stable).
func Rename(s Surface, newName string) (Surface, error) {
	if strings.TrimSpace(newName) == "" {
		return Surface{}, fmt.Errorf("GOLC_OPERATORSURFACE_NAME_EMPTY: operator surface name must not be empty")
	}
	clone := cloneSurface(s)
	clone.Name = newName
	return clone, nil
}

// AssignScene returns a copy of s with sceneID added to SceneRefs.
// Assigning an already-assigned scene is an idempotent no-op (PLAY-03):
// the membership set is left unchanged, never duplicated.
func AssignScene(s Surface, sceneID uuid.UUID) Surface {
	clone := cloneSurface(s)
	for _, existing := range clone.SceneRefs {
		if existing == sceneID {
			return clone
		}
	}
	clone.SceneRefs = append(clone.SceneRefs, sceneID)
	return clone
}

// UnassignScene returns a copy of s with sceneID removed from SceneRefs.
// Unassigning an item not present is a no-op.
func UnassignScene(s Surface, sceneID uuid.UUID) Surface {
	clone := cloneSurface(s)
	filtered := make([]uuid.UUID, 0, len(clone.SceneRefs))
	for _, existing := range clone.SceneRefs {
		if existing != sceneID {
			filtered = append(filtered, existing)
		}
	}
	clone.SceneRefs = filtered
	return clone
}

// AssignLayer returns a copy of s with ref added to LayerRefs. Assigning
// an already-assigned layer is an idempotent no-op.
func AssignLayer(s Surface, ref LayerRef) Surface {
	clone := cloneSurface(s)
	for _, existing := range clone.LayerRefs {
		if layerRefEqual(existing, ref) {
			return clone
		}
	}
	clone.LayerRefs = append(clone.LayerRefs, ref)
	return clone
}

// UnassignLayer returns a copy of s with ref removed from LayerRefs.
// Unassigning an item not present is a no-op.
func UnassignLayer(s Surface, ref LayerRef) Surface {
	clone := cloneSurface(s)
	filtered := make([]LayerRef, 0, len(clone.LayerRefs))
	for _, existing := range clone.LayerRefs {
		if !layerRefEqual(existing, ref) {
			filtered = append(filtered, existing)
		}
	}
	clone.LayerRefs = filtered
	return clone
}

// AssignMaster returns a copy of s with ref added to MasterRefs. Assigning
// an already-assigned master is an idempotent no-op.
func AssignMaster(s Surface, ref MasterRef) Surface {
	clone := cloneSurface(s)
	for _, existing := range clone.MasterRefs {
		if masterRefEqual(existing, ref) {
			return clone
		}
	}
	clone.MasterRefs = append(clone.MasterRefs, ref)
	return clone
}

// UnassignMaster returns a copy of s with ref removed from MasterRefs.
// Unassigning an item not present is a no-op.
func UnassignMaster(s Surface, ref MasterRef) Surface {
	clone := cloneSurface(s)
	filtered := make([]MasterRef, 0, len(clone.MasterRefs))
	for _, existing := range clone.MasterRefs {
		if !masterRefEqual(existing, ref) {
			filtered = append(filtered, existing)
		}
	}
	clone.MasterRefs = filtered
	return clone
}

// AssignSafety returns a copy of s with sc added to SafetyRefs. Assigning
// an already-assigned safety control is an idempotent no-op.
func AssignSafety(s Surface, sc SafetyControl) Surface {
	clone := cloneSurface(s)
	for _, existing := range clone.SafetyRefs {
		if existing == sc {
			return clone
		}
	}
	clone.SafetyRefs = append(clone.SafetyRefs, sc)
	return clone
}

// UnassignSafety returns a copy of s with sc removed from SafetyRefs.
// Unassigning an item not present is a no-op.
func UnassignSafety(s Surface, sc SafetyControl) Surface {
	clone := cloneSurface(s)
	filtered := make([]SafetyControl, 0, len(clone.SafetyRefs))
	for _, existing := range clone.SafetyRefs {
		if existing != sc {
			filtered = append(filtered, existing)
		}
	}
	clone.SafetyRefs = filtered
	return clone
}

// AddMidiMapping mints a fresh UUIDv7 for candidate and appends it to s's
// MidiMappings, unless a mapping already on s shares candidate's
// (Channel, Kind, Number) tuple -- in which case AddMidiMapping rejects
// the candidate outright with GOLC_OPERATORSURFACE_MIDI_CONFLICT (D-06)
// and returns s unmodified: the existing mapping is never overwritten,
// there is no confirm-to-reassign path, and there is no last-writer-wins
// behavior anywhere in this function.
func AddMidiMapping(s Surface, candidate MidiMapping) (Surface, error) {
	for _, existing := range s.MidiMappings {
		if existing.Channel == candidate.Channel && existing.Kind == candidate.Kind && existing.Number == candidate.Number {
			return Surface{}, fmt.Errorf(
				"GOLC_OPERATORSURFACE_MIDI_CONFLICT: channel=%d kind=%s number=%d is already mapped on surface %q",
				candidate.Channel, candidate.Kind, candidate.Number, s.Name)
		}
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Surface{}, fmt.Errorf("GOLC_OPERATORSURFACE_ID_MINT_FAILED: %v", err)
	}
	candidate.ID = id

	clone := cloneSurface(s)
	clone.MidiMappings = append(clone.MidiMappings, candidate)
	return clone, nil
}

// IsAssigned reports whether ref is currently a member of s's assignment
// set (SceneRefs/LayerRefs/MasterRefs/SafetyRefs, selected by ref.Kind).
// This is the membership check internal/command/operatorsurface.go's
// Authorize calls to enforce visible-but-locked server-side (D-04):
// Authorize itself lives in the command package (its diagnostic format
// matches that package's other server-side rejections), but the
// authoritative membership test lives here, next to the assignment sets
// it reads.
func (s Surface) IsAssigned(ref ControlRef) bool {
	switch ref.Kind {
	case ControlScene:
		for _, id := range s.SceneRefs {
			if id == ref.Scene {
				return true
			}
		}
	case ControlLayer:
		for _, existing := range s.LayerRefs {
			if layerRefEqual(existing, ref.Layer) {
				return true
			}
		}
	case ControlMaster:
		for _, existing := range s.MasterRefs {
			if masterRefEqual(existing, ref.Master) {
				return true
			}
		}
	case ControlSafety:
		for _, existing := range s.SafetyRefs {
			if existing == ref.Safety {
				return true
			}
		}
	}
	return false
}

// ValidateUniqueSurfaceNames rejects any two surfaces in surfaces sharing
// the same Name, mirroring internal/pool/model.go's
// ValidateUniqueGroupNames exactly: a duplicate name is always rejected
// with a diagnostic, never silently permitted.
func ValidateUniqueSurfaceNames(surfaces []Surface) error {
	seen := make(map[string]bool, len(surfaces))
	for _, s := range surfaces {
		if seen[s.Name] {
			return fmt.Errorf("GOLC_OPERATORSURFACE_DUPLICATE_NAME: an operator surface named %q already exists", s.Name)
		}
		seen[s.Name] = true
	}
	return nil
}
