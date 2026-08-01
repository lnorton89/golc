// programmer_test.go proves PROG-02/PROG-03's semantic attribute editing
// and inspection contract (03-01-PLAN.md Task 2): SetAttribute records a
// normalized [0,1] value against a supported fixture.CapabilityType,
// rejecting out-of-range values and unsupported capabilities without
// recording anything; a repeated set on the same (instance, capability)
// overwrites in place; Clear empties the buffer; Touched() reports every
// currently-set attribute, in stable order, with no phantom entries.
package programming_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/fixture"
	"github.com/lnorton89/golc/internal/programming"
)

func TestProgrammerSetAttributeInRange(t *testing.T) {
	ps := programming.NewProgrammerState()
	instanceID := uuid.New()

	require.NoError(t, ps.SetAttribute(instanceID, fixture.CapabilityIntensity, 0.5, programming.SourceManual), "SetAttribute")
	touched := ps.Touched()
	require.Len(t, touched, 1, "expected exactly one touched attribute, got %+v", touched)
	got := touched[0]
	require.Equal(t, instanceID, got.InstanceID)
	require.Equal(t, fixture.CapabilityIntensity, got.Capability)
	require.Equal(t, 0.5, got.Value)
	require.Equal(t, programming.SourceManual, got.Source)
}

func TestProgrammerSetAttributeOutOfRangeRejected(t *testing.T) {
	ps := programming.NewProgrammerState()
	instanceID := uuid.New()

	cases := []float64{-0.01, 1.01, -1, 2}
	for _, value := range cases {
		err := ps.SetAttribute(instanceID, fixture.CapabilityIntensity, value, programming.SourceManual)
		require.ErrorContains(t, err, "GOLC_PROGRAMMER_VALUE_OUT_OF_RANGE", "value %v", value)
	}
	require.Empty(t, ps.Touched(), "expected no touched attributes recorded after out-of-range rejections")
}

func TestProgrammerSetAttributeUnsupportedCapabilityRejected(t *testing.T) {
	ps := programming.NewProgrammerState()
	instanceID := uuid.New()

	err := ps.SetAttribute(instanceID, fixture.CapabilityType("laser"), 0.5, programming.SourceManual)
	require.ErrorContains(t, err, "GOLC_PROGRAMMER_CAPABILITY_UNSUPPORTED")
	require.Empty(t, ps.Touched(), "expected no touched attributes recorded after unsupported-capability rejection")
}

func TestProgrammerSetAttributeOverwrites(t *testing.T) {
	ps := programming.NewProgrammerState()
	instanceID := uuid.New()

	require.NoError(t, ps.SetAttribute(instanceID, fixture.CapabilityIntensity, 0.2, programming.SourceManual), "SetAttribute (first)")
	require.NoError(t, ps.SetAttribute(instanceID, fixture.CapabilityIntensity, 0.9, programming.SourcePreset), "SetAttribute (second)")
	touched := ps.Touched()
	require.Len(t, touched, 1, "expected overwrite in place (one entry), got %+v", touched)
	require.Equal(t, 0.9, touched[0].Value, "expected last-write-wins value/source")
	require.Equal(t, programming.SourcePreset, touched[0].Source, "expected last-write-wins value/source")
}

func TestProgrammerClearEmptiesBuffer(t *testing.T) {
	ps := programming.NewProgrammerState()
	instanceID := uuid.New()
	require.NoError(t, ps.SetAttribute(instanceID, fixture.CapabilityColor, 0.3, programming.SourceManual), "SetAttribute")
	ps.Clear()
	require.Empty(t, ps.Touched(), "expected empty buffer after Clear")
}

func TestProgrammerInspectStableOrderNoPhantoms(t *testing.T) {
	ps := programming.NewProgrammerState()
	instanceA := uuid.New()
	instanceB := uuid.New()

	require.NoError(t, ps.SetAttribute(instanceA, fixture.CapabilityIntensity, 0.4, programming.SourceManual), "SetAttribute (A intensity)")
	require.NoError(t, ps.SetAttribute(instanceB, fixture.CapabilityPan, 0.6, programming.SourceTheme), "SetAttribute (B pan)")
	require.NoError(t, ps.SetAttribute(instanceA, fixture.CapabilityColor, 0.2, programming.SourceManual), "SetAttribute (A color)")

	first := ps.Touched()
	second := ps.Touched()
	require.Len(t, first, 3, "expected exactly 3 touched attributes (no phantom entries)")
	require.Len(t, second, 3, "expected exactly 3 touched attributes (no phantom entries)")
	require.Equal(t, second, first, "expected stable order across repeated Touched() calls")
	// First-set order: A/intensity, B/pan, A/color.
	require.Equal(t, instanceA, first[0].InstanceID, "expected first entry to be A/intensity")
	require.Equal(t, fixture.CapabilityIntensity, first[0].Capability, "expected first entry to be A/intensity")
	require.Equal(t, instanceB, first[1].InstanceID, "expected second entry to be B/pan")
	require.Equal(t, fixture.CapabilityPan, first[1].Capability, "expected second entry to be B/pan")
	require.Equal(t, instanceA, first[2].InstanceID, "expected third entry to be A/color")
	require.Equal(t, fixture.CapabilityColor, first[2].Capability, "expected third entry to be A/color")
}

func TestProgrammerValidateProgrammerAcceptsValidState(t *testing.T) {
	ps := programming.NewProgrammerState()
	require.NoError(t, ps.SetAttribute(uuid.New(), fixture.CapabilityZoom, 0.75, programming.SourceManual), "SetAttribute")
	require.NoError(t, programming.ValidateProgrammer(*ps), "ValidateProgrammer: unexpected error for a state built entirely through SetAttribute")
}

func TestProgrammerValidateProgrammerRejectsHandTamperedState(t *testing.T) {
	tampered := programming.ProgrammerState{
		Attributes: []programming.TouchedAttribute{
			{InstanceID: uuid.New(), Capability: fixture.CapabilityIntensity, Value: 1.5, Source: programming.SourceManual},
		},
	}
	err := programming.ValidateProgrammer(tampered)
	require.ErrorContains(t, err, "GOLC_PROGRAMMER_VALUE_OUT_OF_RANGE", "expected error for a hand-tampered out-of-range value")
}
