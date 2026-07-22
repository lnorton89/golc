// programmer.go implements PROG-02/PROG-03's semantic attribute editing
// and inspection surface: ProgrammerState is the in-memory scratch
// buffer of every semantic intensity/color/position/beam (or other
// supported fixture.CapabilityType) value a show author has touched
// since the last Clear. Every recorded value is normalized [0,1] --
// reusing fixture.Capability.Range's own normalized-value model rather
// than re-deriving a parallel bounds concept -- and every capability
// must be one of fixture.SupportedCapabilityTypes; neither an
// out-of-range value nor an unsupported capability is ever coerced or
// silently accepted.
package programming

import (
	"fmt"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/fixture"
)

// AttributeSource identifies which authoring mechanism supplied a touched
// attribute's value (PROG-03): a direct manual programmer edit versus a
// value applied from a preset or theme. v1 does not model a richer
// per-attribute provenance chain beyond this three-value enum
// (03-01-PLAN.md flagged_assumptions).
type AttributeSource string

// The three v1 attribute sources.
const (
	SourceManual AttributeSource = "manual"
	SourcePreset AttributeSource = "preset"
	SourceTheme  AttributeSource = "theme"
)

// TouchedAttribute is one semantic attribute a programmer edit has set on
// one resolved fixture instance (PROG-02/PROG-03). Value is always a
// normalized [0,1] value, never a raw DMX channel number. InstanceID
// doubles as this attribute's record scope: the exact fixture instance it
// will be recorded into (03-01-PLAN.md flagged_assumptions: v1's "record
// scope" is the resolved instance set an edit targets, which for one
// touched attribute is exactly the one instance named here).
type TouchedAttribute struct {
	InstanceID uuid.UUID              `json:"instance_id"`
	Capability fixture.CapabilityType `json:"capability"`
	Value      float64                `json:"value"`
	Source     AttributeSource        `json:"source"`
}

// ProgrammerState is the show author's in-memory scratch buffer of every
// semantic attribute touched since the last Clear (PROG-02/PROG-03).
// Attributes preserves first-set order (a slice, never a map) so
// Touched() output is stable and deterministic across repeated calls; a
// repeated SetAttribute on the same (instance, capability) overwrites the
// existing entry in place rather than appending or reordering.
type ProgrammerState struct {
	Attributes []TouchedAttribute `json:"touched,omitempty"`
}

// supportedCapabilityTypes mirrors internal/pool/model.go's lookup-set
// pattern, built once from fixture.SupportedCapabilityTypes -- the single
// source of truth for which capability types v1 permits editing.
var supportedCapabilityTypes = func() map[fixture.CapabilityType]bool {
	set := make(map[fixture.CapabilityType]bool, len(fixture.SupportedCapabilityTypes))
	for _, capabilityType := range fixture.SupportedCapabilityTypes {
		set[capabilityType] = true
	}
	return set
}()

// NewProgrammerState returns a fresh, empty ProgrammerState.
func NewProgrammerState() *ProgrammerState {
	return &ProgrammerState{}
}

// indexOf returns the index of the touched attribute matching
// (instanceID, capType) in ps.Attributes, or -1 if none is recorded yet.
func (ps *ProgrammerState) indexOf(instanceID uuid.UUID, capType fixture.CapabilityType) int {
	for i, touched := range ps.Attributes {
		if touched.InstanceID == instanceID && touched.Capability == capType {
			return i
		}
	}
	return -1
}

// validateAttribute enforces the two write-time invariants every
// SetAttribute call and ValidateProgrammer's re-check both share: capType
// must be a supported fixture.CapabilityType
// (GOLC_PROGRAMMER_CAPABILITY_UNSUPPORTED otherwise), and value must fall
// within the normalized [0,1] bound
// (GOLC_PROGRAMMER_VALUE_OUT_OF_RANGE otherwise). Semantic attribute
// editing never falls back to a raw DMX channel number: an unsupported
// capability or out-of-range value is always rejected, never coerced.
func validateAttribute(capType fixture.CapabilityType, value float64) error {
	if !supportedCapabilityTypes[capType] {
		return fmt.Errorf("GOLC_PROGRAMMER_CAPABILITY_UNSUPPORTED: %q is not a supported capability type", capType)
	}
	if value < 0 || value > 1 {
		return fmt.Errorf("GOLC_PROGRAMMER_VALUE_OUT_OF_RANGE: value %v for capability %q is outside the normalized 0-1 range", value, capType)
	}
	return nil
}

// SetAttribute records value as instanceID's normalized capType attribute
// (PROG-02). Rejection (out-of-range value or unsupported capability)
// records nothing -- ps is left unchanged. Setting the same (instanceID,
// capType) twice overwrites the prior entry in place (last write wins),
// never appending a second entry.
func (ps *ProgrammerState) SetAttribute(instanceID uuid.UUID, capType fixture.CapabilityType, value float64, source AttributeSource) error {
	if err := validateAttribute(capType, value); err != nil {
		return err
	}
	entry := TouchedAttribute{InstanceID: instanceID, Capability: capType, Value: value, Source: source}
	if i := ps.indexOf(instanceID, capType); i >= 0 {
		ps.Attributes[i] = entry
		return nil
	}
	ps.Attributes = append(ps.Attributes, entry)
	return nil
}

// Clear empties ps's touched-attribute buffer.
func (ps *ProgrammerState) Clear() {
	ps.Attributes = nil
}

// Touched returns ps's touched attributes in stable, first-set order
// (PROG-03): every entry was explicitly set through SetAttribute -- no
// phantom or default-valued entries ever appear. The returned slice is a
// copy; mutating it never affects ps's internal buffer.
func (ps *ProgrammerState) Touched() []TouchedAttribute {
	return append([]TouchedAttribute(nil), ps.Attributes...)
}

// ValidateProgrammer re-checks every touched attribute in ps against the
// same rules SetAttribute enforces at write time. This is the hook
// show.validate() (Task 3) calls whenever a State carries a non-nil
// Programmer buffer, so a hand-edited show file cannot smuggle an
// out-of-range value or unsupported capability past Load/Save
// (GOLC_SHOW_STATE_INVALID wraps whatever this returns).
func ValidateProgrammer(ps ProgrammerState) error {
	for _, touched := range ps.Attributes {
		if err := validateAttribute(touched.Capability, touched.Value); err != nil {
			return err
		}
	}
	return nil
}
