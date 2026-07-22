// decode_test.go proves FIXT-01/FIXT-02's strict YAML fixture decode
// contract (02-01-PLAN.md, Task 1 Wave-0 scaffold): a valid RGB PAR
// definition decodes into a FixtureDefinition with its capabilities in
// declared order; a duplicate mapping key, an unknown/unmodeled field, an
// out-of-[0,1] capability range, an unsupported capability type, and an
// empty/zero-capability/null-capability-list document are all rejected
// with an actionable GOLC_FIXTURE_* diagnostic before any typed value is
// trusted; two capabilities of the same type may touch at a shared
// boundary but never overlap; and decoding the same bytes twice is
// byte-identical (declared order and canonical summary alike).
//
// This file intentionally fails to compile until internal/fixture exists
// (Task 2/3 of 02-01-PLAN.md) -- that is the RED state this task proves.
package fixture_test

import (
	"strings"
	"testing"

	"github.com/lnorton89/golc/internal/fixture"
	"github.com/lnorton89/golc/internal/strictjson"
)

const validRGBParYAML = `schema_version: 1
manufacturer: Generic
model: RGB PAR
modes:
  - name: Standard
capabilities:
  - type: intensity
    range: [0, 1]
    comment: Master dimmer
  - type: color
    range: [0, 1]
    comment: RGB color mix
`

func TestLoad(t *testing.T) {
	def, err := fixture.Decode([]byte(validRGBParYAML))
	if err != nil {
		t.Fatalf("Decode(valid RGB PAR) failed: %v", err)
	}
	if def.Manufacturer != "Generic" || def.Model != "RGB PAR" {
		t.Fatalf("unexpected manufacturer/model: %+v", def)
	}
	if len(def.Modes) != 1 || def.Modes[0].Name != "Standard" {
		t.Fatalf("unexpected modes: %+v", def.Modes)
	}
	if len(def.Capabilities) != 2 {
		t.Fatalf("expected 2 capabilities in declared order, got %d: %+v", len(def.Capabilities), def.Capabilities)
	}
	if def.Capabilities[0].Type != fixture.CapabilityIntensity {
		t.Fatalf("expected first capability intensity (declared order), got %q", def.Capabilities[0].Type)
	}
	if def.Capabilities[1].Type != fixture.CapabilityColor {
		t.Fatalf("expected second capability color (declared order), got %q", def.Capabilities[1].Type)
	}
}

func TestDecodeRejects(t *testing.T) {
	cases := []struct {
		name     string
		yaml     string
		wantCode string
	}{
		{
			name: "duplicate mapping key",
			yaml: `schema_version: 1
manufacturer: Generic
manufacturer: Generic Duplicate
model: RGB PAR
modes:
  - name: Standard
capabilities:
  - type: intensity
    range: [0, 1]
`,
			wantCode: "GOLC_FIXTURE_YAML_INVALID",
		},
		{
			name: "unknown field",
			yaml: `schema_version: 1
manufacturer: Generic
model: RGB PAR
modes:
  - name: Standard
capabilities:
  - type: intensity
    range: [0, 1]
unknown_field: true
`,
			wantCode: "GOLC_FIXTURE_YAML_INVALID",
		},
		{
			name: "capability range outside 0..1",
			yaml: `schema_version: 1
manufacturer: Generic
model: RGB PAR
modes:
  - name: Standard
capabilities:
  - type: intensity
    range: [0, 1.5]
`,
			wantCode: "GOLC_FIXTURE_CAPABILITY_RANGE_INVALID",
		},
		{
			name: "unsupported capability type",
			yaml: `schema_version: 1
manufacturer: Generic
model: RGB PAR
modes:
  - name: Standard
capabilities:
  - type: not-a-real-capability
    range: [0, 1]
`,
			wantCode: "GOLC_FIXTURE_CAPABILITY_TYPE_UNSUPPORTED",
		},
		{
			name:     "empty file",
			yaml:     "",
			wantCode: "GOLC_FIXTURE_EMPTY",
		},
		{
			name: "zero capabilities",
			yaml: `schema_version: 1
manufacturer: Generic
model: RGB PAR
modes:
  - name: Standard
capabilities: []
`,
			wantCode: "GOLC_FIXTURE_EMPTY",
		},
		{
			name: "null capability list",
			yaml: `schema_version: 1
manufacturer: Generic
model: RGB PAR
modes:
  - name: Standard
capabilities: null
`,
			wantCode: "GOLC_FIXTURE_EMPTY",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := fixture.Decode([]byte(testCase.yaml))
			if err == nil {
				t.Fatalf("expected Decode to reject %q, got nil error", testCase.name)
			}
			if !strings.Contains(err.Error(), testCase.wantCode) {
				t.Fatalf("expected error to contain %s, got %v", testCase.wantCode, err)
			}
		})
	}
}

func TestDecodeAdjacency(t *testing.T) {
	touching := `schema_version: 1
manufacturer: Generic
model: Strobe PAR
modes:
  - name: Standard
capabilities:
  - type: shutter
    range: [0, 0.5]
    comment: closed
  - type: shutter
    range: [0.5, 1]
    comment: strobe
`
	def, err := fixture.Decode([]byte(touching))
	if err != nil {
		t.Fatalf("expected exactly-adjacent same-type ranges to load, got error: %v", err)
	}
	if len(def.Capabilities) != 2 {
		t.Fatalf("expected 2 capabilities, got %d", len(def.Capabilities))
	}

	overlapping := `schema_version: 1
manufacturer: Generic
model: Strobe PAR
modes:
  - name: Standard
capabilities:
  - type: shutter
    range: [0, 0.6]
    comment: closed
  - type: shutter
    range: [0.5, 1]
    comment: strobe
`
	_, err = fixture.Decode([]byte(overlapping))
	if err == nil {
		t.Fatal("expected overlapping same-type ranges to be rejected, got nil error")
	}
	if !strings.Contains(err.Error(), "GOLC_FIXTURE_CAPABILITY_RANGE_INVALID") {
		t.Fatalf("expected GOLC_FIXTURE_CAPABILITY_RANGE_INVALID for an overlap, got %v", err)
	}
}

func TestDecodeDeterministic(t *testing.T) {
	first, err := fixture.Decode([]byte(validRGBParYAML))
	if err != nil {
		t.Fatalf("first Decode failed: %v", err)
	}
	second, err := fixture.Decode([]byte(validRGBParYAML))
	if err != nil {
		t.Fatalf("second Decode failed: %v", err)
	}

	for i := range first.Capabilities {
		if first.Capabilities[i].Type != second.Capabilities[i].Type {
			t.Fatalf("capability declared order drifted at index %d: %q vs %q", i, first.Capabilities[i].Type, second.Capabilities[i].Type)
		}
	}

	firstEncoded, err := strictjson.CanonicalEncode(first)
	if err != nil {
		t.Fatalf("CanonicalEncode(first) failed: %v", err)
	}
	secondEncoded, err := strictjson.CanonicalEncode(second)
	if err != nil {
		t.Fatalf("CanonicalEncode(second) failed: %v", err)
	}
	if string(firstEncoded) != string(secondEncoded) {
		t.Fatalf("expected byte-identical canonical summary across repeated decodes:\nfirst:  %s\nsecond: %s", firstEncoded, secondEncoded)
	}
}
