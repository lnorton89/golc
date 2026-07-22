// preset_test.go proves PROG-04's Preset identity/construction/kind-
// filtered-record/duplicate-name/validation contract (03-02-PLAN.md
// Task 1): NewPreset mints a UUIDv7 ID for a valid kind and rejects an
// unknown kind; RecordPresetFromProgrammer filters a ProgrammerState's
// touched attributes down to exactly the kind's allowed capabilities,
// capturing zero attributes without error when none match; ValidatePreset
// re-checks captured values against the normalized [0,1] bound and
// rejects an off-kind captured attribute.
package programming_test

import (
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/fixture"
	"github.com/lnorton89/golc/internal/programming"
)

func TestThemePresetNewPresetValidKindMintsID(t *testing.T) {
	preset, err := programming.NewPreset("Full Wash", programming.PresetIntensity)
	if err != nil {
		t.Fatalf("NewPreset: %v", err)
	}
	if preset.ID.String() == "" {
		t.Fatalf("expected a minted UUIDv7 ID, got zero value")
	}
	if preset.Name != "Full Wash" || preset.Kind != programming.PresetIntensity {
		t.Fatalf("unexpected preset: %+v", preset)
	}
	if len(preset.Attributes) != 0 {
		t.Fatalf("expected a freshly created preset to have zero attributes, got %+v", preset.Attributes)
	}
}

func TestThemePresetNewPresetEmptyNameRejected(t *testing.T) {
	_, err := programming.NewPreset("  ", programming.PresetIntensity)
	if err == nil || !strings.Contains(err.Error(), "GOLC_PRESET_NAME_EMPTY") {
		t.Fatalf("expected GOLC_PRESET_NAME_EMPTY, got %v", err)
	}
}

func TestThemePresetNewPresetUnknownKindRejected(t *testing.T) {
	_, err := programming.NewPreset("Weird", programming.PresetKind("laser"))
	if err == nil || !strings.Contains(err.Error(), "GOLC_PRESET_KIND_INVALID") {
		t.Fatalf("expected GOLC_PRESET_KIND_INVALID, got %v", err)
	}
}

func TestThemePresetRenamePresetPreservesID(t *testing.T) {
	preset, err := programming.NewPreset("Full Wash", programming.PresetIntensity)
	if err != nil {
		t.Fatalf("NewPreset: %v", err)
	}
	originalID := preset.ID

	renamed, err := programming.RenamePreset(preset, "Half Wash")
	if err != nil {
		t.Fatalf("RenamePreset: %v", err)
	}
	if renamed.ID != originalID {
		t.Fatalf("expected ID to be preserved by rename, got original=%s renamed=%s", originalID, renamed.ID)
	}
	if renamed.Name != "Half Wash" {
		t.Fatalf("expected renamed Name %q, got %q", "Half Wash", renamed.Name)
	}
}

func TestThemePresetValidatePresetUniqueNamesRejectsDuplicate(t *testing.T) {
	a, err := programming.NewPreset("Full Wash", programming.PresetIntensity)
	if err != nil {
		t.Fatalf("NewPreset(a): %v", err)
	}
	b, err := programming.NewPreset("Full Wash", programming.PresetColor)
	if err != nil {
		t.Fatalf("NewPreset(b): %v", err)
	}
	err = programming.ValidatePresetUniqueNames([]programming.Preset{a, b})
	if err == nil || !strings.Contains(err.Error(), "GOLC_PRESET_DUPLICATE_NAME") {
		t.Fatalf("expected GOLC_PRESET_DUPLICATE_NAME, got %v", err)
	}
}

func TestThemePresetRecordPresetFromProgrammerFiltersOffKind(t *testing.T) {
	ps := programming.NewProgrammerState()
	instanceA := uuid.New()
	instanceB := uuid.New()

	if err := ps.SetAttribute(instanceA, fixture.CapabilityPan, 0.25, programming.SourceManual); err != nil {
		t.Fatalf("SetAttribute (pan): %v", err)
	}
	if err := ps.SetAttribute(instanceA, fixture.CapabilityTilt, 0.75, programming.SourceManual); err != nil {
		t.Fatalf("SetAttribute (tilt): %v", err)
	}
	// Off-kind for a position preset -- must be excluded, never captured.
	if err := ps.SetAttribute(instanceB, fixture.CapabilityIntensity, 0.5, programming.SourceManual); err != nil {
		t.Fatalf("SetAttribute (intensity): %v", err)
	}
	if err := ps.SetAttribute(instanceB, fixture.CapabilityColor, 0.4, programming.SourceManual); err != nil {
		t.Fatalf("SetAttribute (color): %v", err)
	}

	preset, err := programming.RecordPresetFromProgrammer(*ps, programming.PresetPosition, "Center Stage")
	if err != nil {
		t.Fatalf("RecordPresetFromProgrammer: %v", err)
	}
	if preset.Kind != programming.PresetPosition || preset.Name != "Center Stage" {
		t.Fatalf("unexpected preset identity: %+v", preset)
	}
	if len(preset.Attributes) != 2 {
		t.Fatalf("expected exactly 2 position attributes captured, got %d: %+v", len(preset.Attributes), preset.Attributes)
	}
	for _, attr := range preset.Attributes {
		if attr.Capability != fixture.CapabilityPan && attr.Capability != fixture.CapabilityTilt {
			t.Fatalf("expected only pan/tilt captured for a position preset, got capability %q", attr.Capability)
		}
		if attr.InstanceID != instanceA {
			t.Fatalf("expected only instanceA's touched attributes captured, got instance %s", attr.InstanceID)
		}
	}
}

func TestThemePresetRecordPresetFromProgrammerZeroMatchesIsValidEmptyPreset(t *testing.T) {
	ps := programming.NewProgrammerState()
	if err := ps.SetAttribute(uuid.New(), fixture.CapabilityIntensity, 0.5, programming.SourceManual); err != nil {
		t.Fatalf("SetAttribute: %v", err)
	}

	preset, err := programming.RecordPresetFromProgrammer(*ps, programming.PresetPosition, "Empty Position")
	if err != nil {
		t.Fatalf("expected no error for a preset that captures zero attributes, got %v", err)
	}
	if len(preset.Attributes) != 0 {
		t.Fatalf("expected zero captured attributes, got %+v", preset.Attributes)
	}
}

func TestThemePresetValidatePresetRejectsOutOfRangeValue(t *testing.T) {
	preset := programming.Preset{
		ID:   uuid.Must(uuid.NewV7()),
		Name: "Tampered",
		Kind: programming.PresetIntensity,
		Attributes: []programming.PresetAttribute{
			{InstanceID: uuid.New(), Capability: fixture.CapabilityIntensity, Value: 1.5},
		},
	}
	err := programming.ValidatePreset(preset)
	if err == nil || !strings.Contains(err.Error(), "GOLC_PRESET_VALUE_OUT_OF_RANGE") {
		t.Fatalf("expected GOLC_PRESET_VALUE_OUT_OF_RANGE, got %v", err)
	}
}

func TestThemePresetValidatePresetRejectsOffKindAttribute(t *testing.T) {
	preset := programming.Preset{
		ID:   uuid.Must(uuid.NewV7()),
		Name: "Tampered",
		Kind: programming.PresetPosition,
		Attributes: []programming.PresetAttribute{
			{InstanceID: uuid.New(), Capability: fixture.CapabilityIntensity, Value: 0.5},
		},
	}
	err := programming.ValidatePreset(preset)
	if err == nil || !strings.Contains(err.Error(), "GOLC_PRESET_OFF_KIND_ATTRIBUTE") {
		t.Fatalf("expected GOLC_PRESET_OFF_KIND_ATTRIBUTE, got %v", err)
	}
}

func TestThemePresetValidatePresetAcceptsValidPreset(t *testing.T) {
	preset := programming.Preset{
		ID:   uuid.Must(uuid.NewV7()),
		Name: "Valid",
		Kind: programming.PresetBeam,
		Attributes: []programming.PresetAttribute{
			{InstanceID: uuid.New(), Capability: fixture.CapabilityZoom, Value: 0.3},
			{InstanceID: uuid.New(), Capability: fixture.CapabilityGobo, Value: 0.6},
		},
	}
	if err := programming.ValidatePreset(preset); err != nil {
		t.Fatalf("expected a valid beam preset to pass validation, got %v", err)
	}
}
