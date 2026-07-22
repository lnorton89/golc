// programmer_test.go proves PROG-02/PROG-03's semantic attribute editing
// and inspection contract (03-01-PLAN.md Task 2): SetAttribute records a
// normalized [0,1] value against a supported fixture.CapabilityType,
// rejecting out-of-range values and unsupported capabilities without
// recording anything; a repeated set on the same (instance, capability)
// overwrites in place; Clear empties the buffer; Touched() reports every
// currently-set attribute, in stable order, with no phantom entries.
package programming_test

import (
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/fixture"
	"github.com/lnorton89/golc/internal/programming"
)

func TestProgrammerSetAttributeInRange(t *testing.T) {
	ps := programming.NewProgrammerState()
	instanceID := uuid.New()

	if err := ps.SetAttribute(instanceID, fixture.CapabilityIntensity, 0.5, programming.SourceManual); err != nil {
		t.Fatalf("SetAttribute: %v", err)
	}
	touched := ps.Touched()
	if len(touched) != 1 {
		t.Fatalf("expected exactly one touched attribute, got %d: %+v", len(touched), touched)
	}
	got := touched[0]
	if got.InstanceID != instanceID || got.Capability != fixture.CapabilityIntensity || got.Value != 0.5 || got.Source != programming.SourceManual {
		t.Fatalf("unexpected touched attribute: %+v", got)
	}
}

func TestProgrammerSetAttributeOutOfRangeRejected(t *testing.T) {
	ps := programming.NewProgrammerState()
	instanceID := uuid.New()

	cases := []float64{-0.01, 1.01, -1, 2}
	for _, value := range cases {
		err := ps.SetAttribute(instanceID, fixture.CapabilityIntensity, value, programming.SourceManual)
		if err == nil || !strings.Contains(err.Error(), "GOLC_PROGRAMMER_VALUE_OUT_OF_RANGE") {
			t.Fatalf("value %v: expected GOLC_PROGRAMMER_VALUE_OUT_OF_RANGE, got %v", value, err)
		}
	}
	if len(ps.Touched()) != 0 {
		t.Fatalf("expected no touched attributes recorded after out-of-range rejections, got %+v", ps.Touched())
	}
}

func TestProgrammerSetAttributeUnsupportedCapabilityRejected(t *testing.T) {
	ps := programming.NewProgrammerState()
	instanceID := uuid.New()

	err := ps.SetAttribute(instanceID, fixture.CapabilityType("laser"), 0.5, programming.SourceManual)
	if err == nil || !strings.Contains(err.Error(), "GOLC_PROGRAMMER_CAPABILITY_UNSUPPORTED") {
		t.Fatalf("expected GOLC_PROGRAMMER_CAPABILITY_UNSUPPORTED, got %v", err)
	}
	if len(ps.Touched()) != 0 {
		t.Fatalf("expected no touched attributes recorded after unsupported-capability rejection, got %+v", ps.Touched())
	}
}

func TestProgrammerSetAttributeOverwrites(t *testing.T) {
	ps := programming.NewProgrammerState()
	instanceID := uuid.New()

	if err := ps.SetAttribute(instanceID, fixture.CapabilityIntensity, 0.2, programming.SourceManual); err != nil {
		t.Fatalf("SetAttribute (first): %v", err)
	}
	if err := ps.SetAttribute(instanceID, fixture.CapabilityIntensity, 0.9, programming.SourcePreset); err != nil {
		t.Fatalf("SetAttribute (second): %v", err)
	}
	touched := ps.Touched()
	if len(touched) != 1 {
		t.Fatalf("expected overwrite in place (one entry), got %d: %+v", len(touched), touched)
	}
	if touched[0].Value != 0.9 || touched[0].Source != programming.SourcePreset {
		t.Fatalf("expected last-write-wins value/source, got %+v", touched[0])
	}
}

func TestProgrammerClearEmptiesBuffer(t *testing.T) {
	ps := programming.NewProgrammerState()
	instanceID := uuid.New()
	if err := ps.SetAttribute(instanceID, fixture.CapabilityColor, 0.3, programming.SourceManual); err != nil {
		t.Fatalf("SetAttribute: %v", err)
	}
	ps.Clear()
	if touched := ps.Touched(); len(touched) != 0 {
		t.Fatalf("expected empty buffer after Clear, got %+v", touched)
	}
}

func TestProgrammerInspectStableOrderNoPhantoms(t *testing.T) {
	ps := programming.NewProgrammerState()
	instanceA := uuid.New()
	instanceB := uuid.New()

	if err := ps.SetAttribute(instanceA, fixture.CapabilityIntensity, 0.4, programming.SourceManual); err != nil {
		t.Fatalf("SetAttribute (A intensity): %v", err)
	}
	if err := ps.SetAttribute(instanceB, fixture.CapabilityPan, 0.6, programming.SourceTheme); err != nil {
		t.Fatalf("SetAttribute (B pan): %v", err)
	}
	if err := ps.SetAttribute(instanceA, fixture.CapabilityColor, 0.2, programming.SourceManual); err != nil {
		t.Fatalf("SetAttribute (A color): %v", err)
	}

	first := ps.Touched()
	second := ps.Touched()
	if len(first) != 3 || len(second) != 3 {
		t.Fatalf("expected exactly 3 touched attributes (no phantom entries), got first=%d second=%d", len(first), len(second))
	}
	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("expected stable order across repeated Touched() calls: first=%+v second=%+v", first, second)
		}
	}
	// First-set order: A/intensity, B/pan, A/color.
	if first[0].InstanceID != instanceA || first[0].Capability != fixture.CapabilityIntensity {
		t.Fatalf("expected first entry to be A/intensity, got %+v", first[0])
	}
	if first[1].InstanceID != instanceB || first[1].Capability != fixture.CapabilityPan {
		t.Fatalf("expected second entry to be B/pan, got %+v", first[1])
	}
	if first[2].InstanceID != instanceA || first[2].Capability != fixture.CapabilityColor {
		t.Fatalf("expected third entry to be A/color, got %+v", first[2])
	}
}

func TestProgrammerValidateProgrammerAcceptsValidState(t *testing.T) {
	ps := programming.NewProgrammerState()
	if err := ps.SetAttribute(uuid.New(), fixture.CapabilityZoom, 0.75, programming.SourceManual); err != nil {
		t.Fatalf("SetAttribute: %v", err)
	}
	if err := programming.ValidateProgrammer(*ps); err != nil {
		t.Fatalf("ValidateProgrammer: unexpected error for a state built entirely through SetAttribute: %v", err)
	}
}

func TestProgrammerValidateProgrammerRejectsHandTamperedState(t *testing.T) {
	tampered := programming.ProgrammerState{
		Attributes: []programming.TouchedAttribute{
			{InstanceID: uuid.New(), Capability: fixture.CapabilityIntensity, Value: 1.5, Source: programming.SourceManual},
		},
	}
	if err := programming.ValidateProgrammer(tampered); err == nil || !strings.Contains(err.Error(), "GOLC_PROGRAMMER_VALUE_OUT_OF_RANGE") {
		t.Fatalf("expected GOLC_PROGRAMMER_VALUE_OUT_OF_RANGE for a hand-tampered out-of-range value, got %v", err)
	}
}
