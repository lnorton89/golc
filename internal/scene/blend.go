// blend.go declares the BlendPreset domain model (CONTEXT SCEN-07): a
// reusable, named transition preset a show author assigns to describe how
// a scene/layer state change blends into another. BlendPreset copies
// internal/pool/model.go's identity/construction/rename/unique-name shape
// verbatim (03-PATTERNS.md). This is the data model + validation only --
// evaluating a transition's interpolation over time is the engine's
// concern (03-07); DurationBars = 0 is a valid instant transition.
package scene

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// The small, explicit v1 set of BlendPreset transition curves (SCEN-07):
// an unsupported Curve value is rejected with GOLC_BLEND_PRESET_INVALID.
const (
	BlendCurveLinear  = "linear"
	BlendCurveEaseIn  = "ease_in"
	BlendCurveEaseOut = "ease_out"
)

// supportedBlendCurves is the exact declared curve enum, built once,
// mirroring internal/pool/model.go's supportedCapabilityTypes lookup-set
// pattern.
var supportedBlendCurves = map[string]bool{
	BlendCurveLinear:  true,
	BlendCurveEaseIn:  true,
	BlendCurveEaseOut: true,
}

// BlendPreset is a reusable, named transition preset (SCEN-07). Identity is
// a durable UUIDv7 minted once at creation -- never derived from Name, and
// never re-minted by RenameBlendPreset. DurationBars = 0 is a valid instant
// transition; a negative DurationBars is never valid.
type BlendPreset struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	DurationBars float64   `json:"duration_bars"`
	Curve        string    `json:"curve"`
}

// validateBlendCurve rejects any Curve outside the small declared set
// (GOLC_BLEND_PRESET_INVALID).
func validateBlendCurve(curve string) error {
	if !supportedBlendCurves[curve] {
		return fmt.Errorf("GOLC_BLEND_PRESET_INVALID: %q is not a supported blend curve", curve)
	}
	return nil
}

// ValidateBlendPreset re-checks every invariant a hand-edited or otherwise
// untrusted BlendPreset must satisfy before it is trusted: Name is
// non-empty, DurationBars is not negative (0 is a valid instant transition,
// GOLC_BLEND_PRESET_INVALID otherwise), and Curve is one of the small
// declared set (GOLC_BLEND_PRESET_INVALID otherwise).
func ValidateBlendPreset(b BlendPreset) error {
	if strings.TrimSpace(b.Name) == "" {
		return fmt.Errorf("GOLC_BLEND_PRESET_NAME_EMPTY: blend preset %s declares an empty name", b.ID)
	}
	if b.DurationBars < 0 {
		return fmt.Errorf("GOLC_BLEND_PRESET_INVALID: duration_bars %v must not be negative", b.DurationBars)
	}
	if err := validateBlendCurve(b.Curve); err != nil {
		return err
	}
	return nil
}

// NewBlendPreset mints a fresh UUIDv7-identified BlendPreset. IDs are
// minted only at creation time -- never derived from Name, and never
// re-minted by RenameBlendPreset. A negative durationBars or an
// unsupported curve is rejected before an ID is ever minted.
func NewBlendPreset(name string, durationBars float64, curve string) (BlendPreset, error) {
	preset := BlendPreset{Name: name, DurationBars: durationBars, Curve: curve}
	if err := ValidateBlendPreset(preset); err != nil {
		return BlendPreset{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return BlendPreset{}, fmt.Errorf("GOLC_BLEND_PRESET_ID_MINT_FAILED: %v", err)
	}
	preset.ID = id
	return preset, nil
}

// RenameBlendPreset returns b with Name replaced by newName; ID is never
// re-minted (identity is rename-stable).
func RenameBlendPreset(b BlendPreset, newName string) (BlendPreset, error) {
	if strings.TrimSpace(newName) == "" {
		return BlendPreset{}, fmt.Errorf("GOLC_BLEND_PRESET_NAME_EMPTY: blend preset name must not be empty")
	}
	b.Name = newName
	return b, nil
}

// ValidateBlendPresetUniqueNames rejects any two blend presets in presets
// sharing the same Name: a duplicate name is always rejected with a
// diagnostic, never silently permitted (SCEN-07 idempotency).
func ValidateBlendPresetUniqueNames(presets []BlendPreset) error {
	seen := make(map[string]bool, len(presets))
	for _, p := range presets {
		if seen[p.Name] {
			return fmt.Errorf("GOLC_BLEND_PRESET_DUPLICATE_NAME: a blend preset named %q already exists", p.Name)
		}
		seen[p.Name] = true
	}
	return nil
}
