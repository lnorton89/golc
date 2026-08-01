// blend_test.go proves BlendPreset's identity/construction/duration-
// boundary/curve-validation/duplicate-name contract (03-04-PLAN.md Task 2):
// NewBlendPreset mints a UUIDv7 ID and accepts duration_bars=0 (instant);
// a negative duration_bars or an unsupported curve is rejected;
// ValidateBlendPresetUniqueNames rejects a duplicate name; an empty name is
// rejected.
package scene_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/scene"
)

func TestBlendPresetMintsIDAndAcceptsInstantDuration(t *testing.T) {
	instant, err := scene.NewBlendPreset("Snap", 0, scene.BlendCurveLinear)
	require.NoError(t, err, "expected duration_bars=0 (instant) to be valid")
	require.NotEmpty(t, instant.ID.String(), "expected a minted UUIDv7 ID, got zero value")
	require.Equal(t, "Snap", instant.Name)
	require.Equal(t, float64(0), instant.DurationBars)
	require.Equal(t, scene.BlendCurveLinear, instant.Curve)
}

func TestBlendPresetNegativeDurationRejected(t *testing.T) {
	_, err := scene.NewBlendPreset("Bad", -1, scene.BlendCurveLinear)
	require.ErrorContains(t, err, "GOLC_BLEND_PRESET_INVALID", "expected error for a negative duration")
}

func TestBlendPresetUnsupportedCurveRejected(t *testing.T) {
	_, err := scene.NewBlendPreset("Bad Curve", 1, "bounce")
	require.ErrorContains(t, err, "GOLC_BLEND_PRESET_INVALID", "expected error for an unsupported curve")
}

func TestBlendPresetEmptyNameRejected(t *testing.T) {
	_, err := scene.NewBlendPreset("  ", 1, scene.BlendCurveLinear)
	require.ErrorContains(t, err, "GOLC_BLEND_PRESET_NAME_EMPTY")
}

func TestBlendPresetUniqueNamesRejectsDuplicates(t *testing.T) {
	a, err := scene.NewBlendPreset("Fade", 2, scene.BlendCurveEaseIn)
	require.NoError(t, err, "NewBlendPreset")
	b, err := scene.NewBlendPreset("Fade", 2, scene.BlendCurveEaseOut)
	require.NoError(t, err, "NewBlendPreset")
	err = scene.ValidateBlendPresetUniqueNames([]scene.BlendPreset{a, b})
	require.ErrorContains(t, err, "GOLC_BLEND_PRESET_DUPLICATE_NAME")
}
