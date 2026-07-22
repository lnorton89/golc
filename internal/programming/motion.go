// motion.go declares the MotionPreset domain model (CONTEXT PROG-06/D-04):
// a reusable motion preset built only from position/beam semantic
// capabilities -- pan/tilt plus the beam-shaping capabilities declared in
// fixture.SupportedCapabilityTypes for v1 (zoom/focus; iris/prism are not
// yet part of the declared nine-value enum). Color and gobo/color-wheel
// indexing are deliberately out of scope even on a fixture sharing a
// physical wheel between beam-shaping and gobo/color (D-04): those effects
// stay with color-theme/base-look. MotionPreset copies internal/pool/
// model.go's identity/construction/rename/unique-name shape verbatim
// (03-PATTERNS.md).
package programming

import (
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/fixture"
)

// motionScopedCapabilityTypes is the exact position/beam capability set a
// MotionPreset keyframe may touch (CONTEXT D-04), built once as a lookup
// set mirroring internal/pool/model.go's supportedCapabilityTypes pattern.
var motionScopedCapabilityTypes = map[fixture.CapabilityType]bool{
	fixture.CapabilityPan:   true,
	fixture.CapabilityTilt:  true,
	fixture.CapabilityZoom:  true,
	fixture.CapabilityFocus: true,
}

// MotionScopedCapabilities returns the exact position/beam capability set a
// MotionPreset keyframe may touch (CONTEXT D-04), in declaration order.
// Color and gobo/color-wheel indexing are never included.
func MotionScopedCapabilities() []fixture.CapabilityType {
	return []fixture.CapabilityType{
		fixture.CapabilityPan,
		fixture.CapabilityTilt,
		fixture.CapabilityZoom,
		fixture.CapabilityFocus,
	}
}

// MotionKeyframeValue is one captured position/beam capability value inside
// a MotionKeyframe.
type MotionKeyframeValue struct {
	Capability fixture.CapabilityType `json:"capability"`
	Value      float64                `json:"value"`
}

// MotionKeyframe is one authored point along a MotionPreset's path: a
// normalized [0,1] Phase (the fraction of the preset's run this keyframe
// applies at) plus the position/beam capability values authored at that
// phase.
type MotionKeyframe struct {
	Phase  float64               `json:"phase"`
	Values []MotionKeyframeValue `json:"values,omitempty"`
}

// MotionPreset is a reusable motion preset scoped strictly to position/beam
// semantic capabilities (PROG-06, CONTEXT D-04). Identity is a durable
// UUIDv7 minted once at creation -- never derived from Name, and never
// re-minted by RenameMotionPreset.
type MotionPreset struct {
	ID        uuid.UUID        `json:"id"`
	Name      string           `json:"name"`
	Keyframes []MotionKeyframe `json:"keyframes,omitempty"`
}

// validateMotionKeyframe checks one keyframe's Phase and every captured
// value against the normalized [0,1] bound, and every captured capability
// against the position/beam scope (CONTEXT D-04): a color or gobo/color-
// wheel indexing capability is rejected with
// GOLC_MOTION_PRESET_CAPABILITY_OUT_OF_SCOPE, never silently accepted.
func validateMotionKeyframe(k MotionKeyframe) error {
	if k.Phase < 0 || k.Phase > 1 {
		return fmt.Errorf("GOLC_MOTION_PRESET_PHASE_OUT_OF_RANGE: phase %v is outside the normalized 0-1 range", k.Phase)
	}
	for _, v := range k.Values {
		if !motionScopedCapabilityTypes[v.Capability] {
			return fmt.Errorf("GOLC_MOTION_PRESET_CAPABILITY_OUT_OF_SCOPE: %q is not a position/beam capability a motion preset may touch", v.Capability)
		}
		if v.Value < 0 || v.Value > 1 {
			return fmt.Errorf("GOLC_MOTION_PRESET_VALUE_OUT_OF_RANGE: value %v for capability %q is outside the normalized 0-1 range", v.Value, v.Capability)
		}
	}
	return nil
}

// ValidateMotionPreset re-checks every invariant a hand-edited or
// otherwise-untrusted MotionPreset must satisfy before it is trusted: Name
// is non-empty, and every keyframe's Phase and captured capability/value
// pairs pass validateMotionKeyframe (position/beam scope + normalized
// [0,1] bound).
func ValidateMotionPreset(m MotionPreset) error {
	if strings.TrimSpace(m.Name) == "" {
		return fmt.Errorf("GOLC_MOTION_PRESET_NAME_EMPTY: motion preset %s declares an empty name", m.ID)
	}
	for _, k := range m.Keyframes {
		if err := validateMotionKeyframe(k); err != nil {
			return err
		}
	}
	return nil
}

// NewMotionPreset mints a fresh UUIDv7-identified MotionPreset from the
// given keyframes (PROG-06). A keyframe referencing an out-of-scope
// capability (color, gobo/color-wheel indexing, or anything else outside
// CONTEXT D-04's position/beam set) or an out-of-range Phase/value is
// rejected before an ID is ever minted.
func NewMotionPreset(name string, keyframes []MotionKeyframe) (MotionPreset, error) {
	preset := MotionPreset{Name: name, Keyframes: keyframes}
	if err := ValidateMotionPreset(preset); err != nil {
		return MotionPreset{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return MotionPreset{}, fmt.Errorf("GOLC_MOTION_PRESET_ID_MINT_FAILED: %v", err)
	}
	preset.ID = id
	return preset, nil
}

// RenameMotionPreset returns m with Name replaced by newName; ID is never
// re-minted (identity is rename-stable), and Keyframes is left untouched.
func RenameMotionPreset(m MotionPreset, newName string) (MotionPreset, error) {
	if strings.TrimSpace(newName) == "" {
		return MotionPreset{}, fmt.Errorf("GOLC_MOTION_PRESET_NAME_EMPTY: motion preset name must not be empty")
	}
	m.Name = newName
	return m, nil
}

// ValidateMotionPresetUniqueNames rejects any two motion presets in presets
// sharing the same Name: a duplicate name is always rejected with a
// diagnostic, never silently permitted (PROG-06 idempotency).
func ValidateMotionPresetUniqueNames(presets []MotionPreset) error {
	seen := make(map[string]bool, len(presets))
	for _, p := range presets {
		if seen[p.Name] {
			return fmt.Errorf("GOLC_MOTION_PRESET_DUPLICATE_NAME: a motion preset named %q already exists", p.Name)
		}
		seen[p.Name] = true
	}
	return nil
}
