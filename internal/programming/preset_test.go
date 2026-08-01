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
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/fixture"
	"github.com/lnorton89/golc/internal/programming"
)

func TestThemePresetNewPresetValidKindMintsID(t *testing.T) {
	preset, err := programming.NewPreset("Full Wash", programming.PresetIntensity)
	require.NoError(t, err, "NewPreset")
	require.NotEmpty(t, preset.ID.String(), "expected a minted UUIDv7 ID, got zero value")
	require.Equal(t, "Full Wash", preset.Name)
	require.Equal(t, programming.PresetIntensity, preset.Kind)
	require.Empty(t, preset.Attributes, "expected a freshly created preset to have zero attributes")
}

func TestThemePresetNewPresetEmptyNameRejected(t *testing.T) {
	_, err := programming.NewPreset("  ", programming.PresetIntensity)
	require.ErrorContains(t, err, "GOLC_PRESET_NAME_EMPTY")
}

func TestThemePresetNewPresetUnknownKindRejected(t *testing.T) {
	_, err := programming.NewPreset("Weird", programming.PresetKind("laser"))
	require.ErrorContains(t, err, "GOLC_PRESET_KIND_INVALID")
}

func TestThemePresetRenamePresetPreservesID(t *testing.T) {
	preset, err := programming.NewPreset("Full Wash", programming.PresetIntensity)
	require.NoError(t, err, "NewPreset")
	originalID := preset.ID

	renamed, err := programming.RenamePreset(preset, "Half Wash")
	require.NoError(t, err, "RenamePreset")
	require.Equal(t, originalID, renamed.ID, "expected ID to be preserved by rename")
	require.Equal(t, "Half Wash", renamed.Name)
}

func TestThemePresetValidatePresetUniqueNamesRejectsDuplicate(t *testing.T) {
	a, err := programming.NewPreset("Full Wash", programming.PresetIntensity)
	require.NoError(t, err, "NewPreset(a)")
	b, err := programming.NewPreset("Full Wash", programming.PresetColor)
	require.NoError(t, err, "NewPreset(b)")
	err = programming.ValidatePresetUniqueNames([]programming.Preset{a, b})
	require.ErrorContains(t, err, "GOLC_PRESET_DUPLICATE_NAME")
}

func TestThemePresetRecordPresetFromProgrammerFiltersOffKind(t *testing.T) {
	ps := programming.NewProgrammerState()
	instanceA := uuid.New()
	instanceB := uuid.New()

	require.NoError(t, ps.SetAttribute(instanceA, fixture.CapabilityPan, 0.25, programming.SourceManual), "SetAttribute (pan)")
	require.NoError(t, ps.SetAttribute(instanceA, fixture.CapabilityTilt, 0.75, programming.SourceManual), "SetAttribute (tilt)")
	// Off-kind for a position preset -- must be excluded, never captured.
	require.NoError(t, ps.SetAttribute(instanceB, fixture.CapabilityIntensity, 0.5, programming.SourceManual), "SetAttribute (intensity)")
	require.NoError(t, ps.SetAttribute(instanceB, fixture.CapabilityColor, 0.4, programming.SourceManual), "SetAttribute (color)")

	preset, err := programming.RecordPresetFromProgrammer(*ps, programming.PresetPosition, "Center Stage")
	require.NoError(t, err, "RecordPresetFromProgrammer")
	require.Equal(t, programming.PresetPosition, preset.Kind)
	require.Equal(t, "Center Stage", preset.Name)
	require.Len(t, preset.Attributes, 2, "expected exactly 2 position attributes captured, got %+v", preset.Attributes)
	for _, attr := range preset.Attributes {
		require.True(t, attr.Capability == fixture.CapabilityPan || attr.Capability == fixture.CapabilityTilt,
			"expected only pan/tilt captured for a position preset, got capability %q", attr.Capability)
		require.Equal(t, instanceA, attr.InstanceID, "expected only instanceA's touched attributes captured")
	}
}

func TestThemePresetRecordPresetFromProgrammerZeroMatchesIsValidEmptyPreset(t *testing.T) {
	ps := programming.NewProgrammerState()
	require.NoError(t, ps.SetAttribute(uuid.New(), fixture.CapabilityIntensity, 0.5, programming.SourceManual), "SetAttribute")

	preset, err := programming.RecordPresetFromProgrammer(*ps, programming.PresetPosition, "Empty Position")
	require.NoError(t, err, "expected no error for a preset that captures zero attributes")
	require.Empty(t, preset.Attributes, "expected zero captured attributes")
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
	require.ErrorContains(t, err, "GOLC_PRESET_VALUE_OUT_OF_RANGE")
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
	require.ErrorContains(t, err, "GOLC_PRESET_OFF_KIND_ATTRIBUTE")
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
	require.NoError(t, programming.ValidatePreset(preset), "expected a valid beam preset to pass validation")
}
