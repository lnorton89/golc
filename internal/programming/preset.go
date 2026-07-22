// preset.go declares the Preset domain model (CONTEXT PROG-04): a reusable
// intensity/color/position/beam preset a show author records from the
// current programmer buffer (03-01's ProgrammerState) and reuses across
// scenes. Preset copies internal/pool/model.go's identity/construction/
// rename/unique-name shape verbatim (03-PATTERNS.md), and adds a
// PresetKind-scoped record/validate step so a preset never captures an
// attribute outside its declared kind.
package programming

import (
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/fixture"
)

// PresetKind names one of the four v1 preset kinds (PROG-04). Each kind
// maps to a fixed, disjoint set of fixture.CapabilityType values a preset
// of that kind may ever capture.
type PresetKind string

// The four v1 preset kinds.
const (
	PresetIntensity PresetKind = "intensity"
	PresetColor     PresetKind = "color"
	PresetPosition  PresetKind = "position"
	PresetBeam      PresetKind = "beam"
)

// presetKindCapabilities maps each PresetKind to its allowed
// fixture.CapabilityType set (CONTEXT D-04's position/beam split: position
// is pan/tilt; beam is zoom/focus/gobo/shutter/strobe). Built once, mirrors
// internal/pool/model.go's supportedCapabilityTypes lookup-set pattern.
var presetKindCapabilities = map[PresetKind]map[fixture.CapabilityType]bool{
	PresetIntensity: {
		fixture.CapabilityIntensity: true,
	},
	PresetColor: {
		fixture.CapabilityColor: true,
	},
	PresetPosition: {
		fixture.CapabilityPan:  true,
		fixture.CapabilityTilt: true,
	},
	PresetBeam: {
		fixture.CapabilityZoom:    true,
		fixture.CapabilityFocus:   true,
		fixture.CapabilityGobo:    true,
		fixture.CapabilityShutter: true,
		fixture.CapabilityStrobe:  true,
	},
}

// PresetAttribute is one captured semantic attribute value inside a
// Preset -- the exact touched (instance, capability, value) triple a
// preset of that kind recorded from the programmer buffer.
type PresetAttribute struct {
	InstanceID uuid.UUID              `json:"instance_id"`
	Capability fixture.CapabilityType `json:"capability"`
	Value      float64                `json:"value"`
}

// Preset is a reusable intensity/color/position/beam preset (PROG-04).
// Identity is a durable UUIDv7 minted once at creation -- never derived
// from Name, and never re-minted by RenamePreset. Attributes records
// exactly the touched attributes of the current programmer buffer for
// its Kind -- never an off-kind or untouched attribute.
type Preset struct {
	ID         uuid.UUID         `json:"id"`
	Name       string            `json:"name"`
	Kind       PresetKind        `json:"kind"`
	Attributes []PresetAttribute `json:"attributes,omitempty"`
}

// validatePresetKind rejects any PresetKind outside the four declared
// values (GOLC_PRESET_KIND_INVALID).
func validatePresetKind(kind PresetKind) error {
	if _, ok := presetKindCapabilities[kind]; !ok {
		return fmt.Errorf("GOLC_PRESET_KIND_INVALID: %q is not a supported preset kind", kind)
	}
	return nil
}

// NewPreset mints a fresh UUIDv7-identified Preset of kind with zero
// captured attributes. IDs are minted only at creation time -- never
// derived from Name, and never re-minted by RenamePreset.
func NewPreset(name string, kind PresetKind) (Preset, error) {
	if strings.TrimSpace(name) == "" {
		return Preset{}, fmt.Errorf("GOLC_PRESET_NAME_EMPTY: preset name must not be empty")
	}
	if err := validatePresetKind(kind); err != nil {
		return Preset{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Preset{}, fmt.Errorf("GOLC_PRESET_ID_MINT_FAILED: %v", err)
	}
	return Preset{ID: id, Name: name, Kind: kind}, nil
}

// RecordPresetFromProgrammer records a new Preset of kind from ps's
// currently touched attributes (PROG-04): only attributes whose
// capability is in kind's allowed set are captured -- an intensity or
// color touched attribute is never captured into a position/beam preset,
// and vice versa. Excluded off-kind attributes are simply not captured
// (no error); a preset that ends up capturing zero attributes for its
// kind is still returned as a valid, empty-but-valid preset, never an
// error.
func RecordPresetFromProgrammer(ps ProgrammerState, kind PresetKind, name string) (Preset, error) {
	preset, err := NewPreset(name, kind)
	if err != nil {
		return Preset{}, err
	}
	allowed := presetKindCapabilities[kind]
	for _, touched := range ps.Touched() {
		if !allowed[touched.Capability] {
			continue
		}
		preset.Attributes = append(preset.Attributes, PresetAttribute{
			InstanceID: touched.InstanceID,
			Capability: touched.Capability,
			Value:      touched.Value,
		})
	}
	return preset, nil
}

// RenamePreset returns p with Name replaced by newName; ID is never
// re-minted (identity is rename-stable).
func RenamePreset(p Preset, newName string) (Preset, error) {
	if strings.TrimSpace(newName) == "" {
		return Preset{}, fmt.Errorf("GOLC_PRESET_NAME_EMPTY: preset name must not be empty")
	}
	p.Name = newName
	return p, nil
}

// ValidatePresetUniqueNames rejects any two presets in presets sharing the
// same Name: a duplicate name is always rejected with a diagnostic, never
// silently permitted (PROG-04 idempotency).
func ValidatePresetUniqueNames(presets []Preset) error {
	seen := make(map[string]bool, len(presets))
	for _, p := range presets {
		if seen[p.Name] {
			return fmt.Errorf("GOLC_PRESET_DUPLICATE_NAME: a preset named %q already exists", p.Name)
		}
		seen[p.Name] = true
	}
	return nil
}

// ValidatePreset re-checks every invariant a hand-edited or otherwise
// untrusted Preset must satisfy before it is trusted: Name is non-empty,
// Kind is one of the four declared kinds, every captured attribute's
// capability belongs to Kind's allowed set (GOLC_PRESET_OFF_KIND_ATTRIBUTE
// otherwise -- catching a hand-tampered preset that RecordPresetFromProgrammer's
// own filter would never produce), and every captured value falls within
// the normalized [0,1] bound (GOLC_PRESET_VALUE_OUT_OF_RANGE otherwise --
// the buffer is never trusted blindly, mirroring
// programming.validateAttribute's own re-check).
func ValidatePreset(p Preset) error {
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("GOLC_PRESET_NAME_EMPTY: preset %s declares an empty name", p.ID)
	}
	allowed, ok := presetKindCapabilities[p.Kind]
	if !ok {
		return fmt.Errorf("GOLC_PRESET_KIND_INVALID: %q is not a supported preset kind", p.Kind)
	}
	for _, attr := range p.Attributes {
		if !allowed[attr.Capability] {
			return fmt.Errorf(
				"GOLC_PRESET_OFF_KIND_ATTRIBUTE: preset %q (kind %q) captures off-kind capability %q",
				p.Name, p.Kind, attr.Capability)
		}
		if attr.Value < 0 || attr.Value > 1 {
			return fmt.Errorf(
				"GOLC_PRESET_VALUE_OUT_OF_RANGE: value %v for capability %q in preset %q is outside the normalized 0-1 range",
				attr.Value, attr.Capability, p.Name)
		}
	}
	return nil
}
