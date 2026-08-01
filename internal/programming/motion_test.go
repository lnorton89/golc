// motion_test.go proves PROG-06's MotionPreset identity/construction/
// capability-scope/validation contract (03-03-PLAN.md Task 2): NewMotionPreset
// mints a UUIDv7 ID and accepts keyframes touching only pan/tilt/zoom/focus
// (CONTEXT D-04); a keyframe referencing color or a color/gobo-wheel
// indexing capability is rejected with GOLC_MOTION_PRESET_CAPABILITY_
// OUT_OF_SCOPE; duplicate/empty names are rejected; keyframe values are
// validated against the normalized [0,1] range.
package programming_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/fixture"
	"github.com/lnorton89/golc/internal/programming"
)

func TestMotionPresetNewMotionPresetMintsIDAndAcceptsScopedKeyframes(t *testing.T) {
	keyframes := []programming.MotionKeyframe{
		{
			Phase: 0,
			Values: []programming.MotionKeyframeValue{
				{Capability: fixture.CapabilityPan, Value: 0.1},
				{Capability: fixture.CapabilityTilt, Value: 0.2},
			},
		},
		{
			Phase: 0.5,
			Values: []programming.MotionKeyframeValue{
				{Capability: fixture.CapabilityZoom, Value: 0.3},
				{Capability: fixture.CapabilityFocus, Value: 0.4},
			},
		},
	}
	preset, err := programming.NewMotionPreset("Sweep Arc", keyframes)
	require.NoError(t, err, "NewMotionPreset")
	require.NotEmpty(t, preset.ID.String(), "expected a minted UUIDv7 ID, got zero value")
	require.Equal(t, "Sweep Arc", preset.Name)
	require.Len(t, preset.Keyframes, 2)
}

func TestMotionPresetNewMotionPresetRejectsColorCapability(t *testing.T) {
	keyframes := []programming.MotionKeyframe{
		{Values: []programming.MotionKeyframeValue{{Capability: fixture.CapabilityColor, Value: 0.5}}},
	}
	_, err := programming.NewMotionPreset("Bad Color", keyframes)
	require.ErrorContains(t, err, "GOLC_MOTION_PRESET_CAPABILITY_OUT_OF_SCOPE", "expected rejection for color")
}

func TestMotionPresetNewMotionPresetRejectsGoboCapability(t *testing.T) {
	keyframes := []programming.MotionKeyframe{
		{Values: []programming.MotionKeyframeValue{{Capability: fixture.CapabilityGobo, Value: 0.5}}},
	}
	_, err := programming.NewMotionPreset("Bad Gobo", keyframes)
	require.ErrorContains(t, err, "GOLC_MOTION_PRESET_CAPABILITY_OUT_OF_SCOPE", "expected rejection for gobo")
}

func TestMotionPresetNewMotionPresetRejectsOutOfRangeValue(t *testing.T) {
	keyframes := []programming.MotionKeyframe{
		{Values: []programming.MotionKeyframeValue{{Capability: fixture.CapabilityPan, Value: 1.5}}},
	}
	_, err := programming.NewMotionPreset("Out Of Range", keyframes)
	require.ErrorContains(t, err, "GOLC_MOTION_PRESET_VALUE_OUT_OF_RANGE")
}

func TestMotionPresetNewMotionPresetEmptyNameRejected(t *testing.T) {
	_, err := programming.NewMotionPreset("  ", nil)
	require.ErrorContains(t, err, "GOLC_MOTION_PRESET_NAME_EMPTY")
}

func TestMotionPresetRenameMotionPresetPreservesID(t *testing.T) {
	preset, err := programming.NewMotionPreset("Sweep Arc", nil)
	require.NoError(t, err, "NewMotionPreset")
	originalID := preset.ID

	renamed, err := programming.RenameMotionPreset(preset, "Sweep Arc Renamed")
	require.NoError(t, err, "RenameMotionPreset")
	require.Equal(t, originalID, renamed.ID, "expected ID to be preserved by rename")
	require.Equal(t, "Sweep Arc Renamed", renamed.Name)
}

func TestMotionPresetRenameMotionPresetEmptyNameRejected(t *testing.T) {
	preset, err := programming.NewMotionPreset("Sweep Arc", nil)
	require.NoError(t, err, "NewMotionPreset")
	_, err = programming.RenameMotionPreset(preset, "   ")
	require.ErrorContains(t, err, "GOLC_MOTION_PRESET_NAME_EMPTY")
}

func TestMotionPresetValidateMotionPresetUniqueNamesRejectsDuplicate(t *testing.T) {
	a, err := programming.NewMotionPreset("Sweep Arc", nil)
	require.NoError(t, err, "NewMotionPreset(a)")
	b, err := programming.NewMotionPreset("Sweep Arc", nil)
	require.NoError(t, err, "NewMotionPreset(b)")
	err = programming.ValidateMotionPresetUniqueNames([]programming.MotionPreset{a, b})
	require.ErrorContains(t, err, "GOLC_MOTION_PRESET_DUPLICATE_NAME")
}

func TestMotionPresetValidateMotionPresetAcceptsValidPreset(t *testing.T) {
	preset, err := programming.NewMotionPreset("Sweep Arc", []programming.MotionKeyframe{
		{Phase: 1, Values: []programming.MotionKeyframeValue{{Capability: fixture.CapabilityTilt, Value: 0.9}}},
	})
	require.NoError(t, err, "NewMotionPreset")
	require.NoError(t, programming.ValidateMotionPreset(preset), "expected a valid motion preset to pass validation")
}

func TestMotionPresetScopedCapabilitiesExcludesColorAndGobo(t *testing.T) {
	scoped := programming.MotionScopedCapabilities()
	seen := make(map[fixture.CapabilityType]bool, len(scoped))
	for _, c := range scoped {
		seen[c] = true
	}
	require.False(t, seen[fixture.CapabilityColor] || seen[fixture.CapabilityGobo], "expected MotionScopedCapabilities to exclude color/gobo, got %+v", scoped)
	require.True(t, seen[fixture.CapabilityPan] && seen[fixture.CapabilityTilt] && seen[fixture.CapabilityZoom] && seen[fixture.CapabilityFocus],
		"expected MotionScopedCapabilities to include pan/tilt/zoom/focus, got %+v", scoped)
}
