// motion_test.go proves PROG-06's MotionPreset identity/construction/
// capability-scope/validation contract (03-03-PLAN.md Task 2): NewMotionPreset
// mints a UUIDv7 ID and accepts keyframes touching only pan/tilt/zoom/focus
// (CONTEXT D-04); a keyframe referencing color or a color/gobo-wheel
// indexing capability is rejected with GOLC_MOTION_PRESET_CAPABILITY_
// OUT_OF_SCOPE; duplicate/empty names are rejected; keyframe values are
// validated against the normalized [0,1] range.
package programming_test

import (
	"strings"
	"testing"

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
	if err != nil {
		t.Fatalf("NewMotionPreset: %v", err)
	}
	if preset.ID.String() == "" {
		t.Fatalf("expected a minted UUIDv7 ID, got zero value")
	}
	if preset.Name != "Sweep Arc" {
		t.Fatalf("unexpected preset name: %+v", preset)
	}
	if len(preset.Keyframes) != 2 {
		t.Fatalf("expected 2 keyframes, got %d", len(preset.Keyframes))
	}
}

func TestMotionPresetNewMotionPresetRejectsColorCapability(t *testing.T) {
	keyframes := []programming.MotionKeyframe{
		{Values: []programming.MotionKeyframeValue{{Capability: fixture.CapabilityColor, Value: 0.5}}},
	}
	_, err := programming.NewMotionPreset("Bad Color", keyframes)
	if err == nil || !strings.Contains(err.Error(), "GOLC_MOTION_PRESET_CAPABILITY_OUT_OF_SCOPE") {
		t.Fatalf("expected GOLC_MOTION_PRESET_CAPABILITY_OUT_OF_SCOPE for color, got %v", err)
	}
}

func TestMotionPresetNewMotionPresetRejectsGoboCapability(t *testing.T) {
	keyframes := []programming.MotionKeyframe{
		{Values: []programming.MotionKeyframeValue{{Capability: fixture.CapabilityGobo, Value: 0.5}}},
	}
	_, err := programming.NewMotionPreset("Bad Gobo", keyframes)
	if err == nil || !strings.Contains(err.Error(), "GOLC_MOTION_PRESET_CAPABILITY_OUT_OF_SCOPE") {
		t.Fatalf("expected GOLC_MOTION_PRESET_CAPABILITY_OUT_OF_SCOPE for gobo, got %v", err)
	}
}

func TestMotionPresetNewMotionPresetRejectsOutOfRangeValue(t *testing.T) {
	keyframes := []programming.MotionKeyframe{
		{Values: []programming.MotionKeyframeValue{{Capability: fixture.CapabilityPan, Value: 1.5}}},
	}
	_, err := programming.NewMotionPreset("Out Of Range", keyframes)
	if err == nil || !strings.Contains(err.Error(), "GOLC_MOTION_PRESET_VALUE_OUT_OF_RANGE") {
		t.Fatalf("expected GOLC_MOTION_PRESET_VALUE_OUT_OF_RANGE, got %v", err)
	}
}

func TestMotionPresetNewMotionPresetEmptyNameRejected(t *testing.T) {
	_, err := programming.NewMotionPreset("  ", nil)
	if err == nil || !strings.Contains(err.Error(), "GOLC_MOTION_PRESET_NAME_EMPTY") {
		t.Fatalf("expected GOLC_MOTION_PRESET_NAME_EMPTY, got %v", err)
	}
}

func TestMotionPresetRenameMotionPresetPreservesID(t *testing.T) {
	preset, err := programming.NewMotionPreset("Sweep Arc", nil)
	if err != nil {
		t.Fatalf("NewMotionPreset: %v", err)
	}
	originalID := preset.ID

	renamed, err := programming.RenameMotionPreset(preset, "Sweep Arc Renamed")
	if err != nil {
		t.Fatalf("RenameMotionPreset: %v", err)
	}
	if renamed.ID != originalID {
		t.Fatalf("expected ID to be preserved by rename, got original=%s renamed=%s", originalID, renamed.ID)
	}
	if renamed.Name != "Sweep Arc Renamed" {
		t.Fatalf("expected renamed Name %q, got %q", "Sweep Arc Renamed", renamed.Name)
	}
}

func TestMotionPresetRenameMotionPresetEmptyNameRejected(t *testing.T) {
	preset, err := programming.NewMotionPreset("Sweep Arc", nil)
	if err != nil {
		t.Fatalf("NewMotionPreset: %v", err)
	}
	_, err = programming.RenameMotionPreset(preset, "   ")
	if err == nil || !strings.Contains(err.Error(), "GOLC_MOTION_PRESET_NAME_EMPTY") {
		t.Fatalf("expected GOLC_MOTION_PRESET_NAME_EMPTY, got %v", err)
	}
}

func TestMotionPresetValidateMotionPresetUniqueNamesRejectsDuplicate(t *testing.T) {
	a, err := programming.NewMotionPreset("Sweep Arc", nil)
	if err != nil {
		t.Fatalf("NewMotionPreset(a): %v", err)
	}
	b, err := programming.NewMotionPreset("Sweep Arc", nil)
	if err != nil {
		t.Fatalf("NewMotionPreset(b): %v", err)
	}
	err = programming.ValidateMotionPresetUniqueNames([]programming.MotionPreset{a, b})
	if err == nil || !strings.Contains(err.Error(), "GOLC_MOTION_PRESET_DUPLICATE_NAME") {
		t.Fatalf("expected GOLC_MOTION_PRESET_DUPLICATE_NAME, got %v", err)
	}
}

func TestMotionPresetValidateMotionPresetAcceptsValidPreset(t *testing.T) {
	preset, err := programming.NewMotionPreset("Sweep Arc", []programming.MotionKeyframe{
		{Phase: 1, Values: []programming.MotionKeyframeValue{{Capability: fixture.CapabilityTilt, Value: 0.9}}},
	})
	if err != nil {
		t.Fatalf("NewMotionPreset: %v", err)
	}
	if err := programming.ValidateMotionPreset(preset); err != nil {
		t.Fatalf("expected a valid motion preset to pass validation, got %v", err)
	}
}

func TestMotionPresetScopedCapabilitiesExcludesColorAndGobo(t *testing.T) {
	scoped := programming.MotionScopedCapabilities()
	seen := make(map[fixture.CapabilityType]bool, len(scoped))
	for _, c := range scoped {
		seen[c] = true
	}
	if seen[fixture.CapabilityColor] || seen[fixture.CapabilityGobo] {
		t.Fatalf("expected MotionScopedCapabilities to exclude color/gobo, got %+v", scoped)
	}
	if !seen[fixture.CapabilityPan] || !seen[fixture.CapabilityTilt] || !seen[fixture.CapabilityZoom] || !seen[fixture.CapabilityFocus] {
		t.Fatalf("expected MotionScopedCapabilities to include pan/tilt/zoom/focus, got %+v", scoped)
	}
}
