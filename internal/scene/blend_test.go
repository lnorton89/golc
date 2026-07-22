// blend_test.go proves BlendPreset's identity/construction/duration-
// boundary/curve-validation/duplicate-name contract (03-04-PLAN.md Task 2):
// NewBlendPreset mints a UUIDv7 ID and accepts duration_bars=0 (instant);
// a negative duration_bars or an unsupported curve is rejected;
// ValidateBlendPresetUniqueNames rejects a duplicate name; an empty name is
// rejected.
package scene_test

import (
	"strings"
	"testing"

	"github.com/lnorton89/golc/internal/scene"
)

func TestBlendPresetMintsIDAndAcceptsInstantDuration(t *testing.T) {
	instant, err := scene.NewBlendPreset("Snap", 0, scene.BlendCurveLinear)
	if err != nil {
		t.Fatalf("expected duration_bars=0 (instant) to be valid, got %v", err)
	}
	if instant.ID.String() == "" {
		t.Fatalf("expected a minted UUIDv7 ID, got zero value")
	}
	if instant.Name != "Snap" || instant.DurationBars != 0 || instant.Curve != scene.BlendCurveLinear {
		t.Fatalf("unexpected blend preset: %+v", instant)
	}
}

func TestBlendPresetNegativeDurationRejected(t *testing.T) {
	_, err := scene.NewBlendPreset("Bad", -1, scene.BlendCurveLinear)
	if err == nil || !strings.Contains(err.Error(), "GOLC_BLEND_PRESET_INVALID") {
		t.Fatalf("expected GOLC_BLEND_PRESET_INVALID for a negative duration, got %v", err)
	}
}

func TestBlendPresetUnsupportedCurveRejected(t *testing.T) {
	_, err := scene.NewBlendPreset("Bad Curve", 1, "bounce")
	if err == nil || !strings.Contains(err.Error(), "GOLC_BLEND_PRESET_INVALID") {
		t.Fatalf("expected GOLC_BLEND_PRESET_INVALID for an unsupported curve, got %v", err)
	}
}

func TestBlendPresetEmptyNameRejected(t *testing.T) {
	_, err := scene.NewBlendPreset("  ", 1, scene.BlendCurveLinear)
	if err == nil || !strings.Contains(err.Error(), "GOLC_BLEND_PRESET_NAME_EMPTY") {
		t.Fatalf("expected GOLC_BLEND_PRESET_NAME_EMPTY, got %v", err)
	}
}

func TestBlendPresetUniqueNamesRejectsDuplicates(t *testing.T) {
	a, err := scene.NewBlendPreset("Fade", 2, scene.BlendCurveEaseIn)
	if err != nil {
		t.Fatalf("NewBlendPreset: %v", err)
	}
	b, err := scene.NewBlendPreset("Fade", 2, scene.BlendCurveEaseOut)
	if err != nil {
		t.Fatalf("NewBlendPreset: %v", err)
	}
	err = scene.ValidateBlendPresetUniqueNames([]scene.BlendPreset{a, b})
	if err == nil || !strings.Contains(err.Error(), "GOLC_BLEND_PRESET_DUPLICATE_NAME") {
		t.Fatalf("expected GOLC_BLEND_PRESET_DUPLICATE_NAME, got %v", err)
	}
}
