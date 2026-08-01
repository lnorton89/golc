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
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/fixture"
	"github.com/lnorton89/golc/internal/strictjson"
)

const validRGBParYAML = `schema_version: 1
manufacturer: Generic
model: RGB PAR
modes:
  - name: Standard
    channels:
      - type: intensity
        occurrence: 0
      - type: color
        occurrence: 0
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
	require.NoError(t, err, "Decode(valid RGB PAR) failed")
	require.Equal(t, "Generic", def.Manufacturer, "unexpected manufacturer/model: %+v", def)
	require.Equal(t, "RGB PAR", def.Model, "unexpected manufacturer/model: %+v", def)
	require.Len(t, def.Modes, 1, "unexpected modes: %+v", def.Modes)
	require.Equal(t, "Standard", def.Modes[0].Name, "unexpected modes: %+v", def.Modes)
	require.Len(t, def.Capabilities, 2, "expected 2 capabilities in declared order, got %+v", def.Capabilities)
	require.Equal(t, fixture.CapabilityIntensity, def.Capabilities[0].Type, "expected first capability intensity (declared order)")
	require.Equal(t, fixture.CapabilityColor, def.Capabilities[1].Type, "expected second capability color (declared order)")
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
			name: "unsupported schema_version",
			yaml: `schema_version: 99
manufacturer: Generic
model: RGB PAR
modes:
  - name: Standard
capabilities:
  - type: intensity
    range: [0, 1]
`,
			wantCode: "GOLC_FIXTURE_SCHEMA_VERSION_UNSUPPORTED",
		},
		{
			name: "empty manufacturer",
			yaml: `schema_version: 1
manufacturer: ""
model: RGB PAR
modes:
  - name: Standard
capabilities:
  - type: intensity
    range: [0, 1]
`,
			wantCode: "GOLC_FIXTURE_MANUFACTURER_EMPTY",
		},
		{
			name: "missing manufacturer",
			yaml: `schema_version: 1
model: RGB PAR
modes:
  - name: Standard
capabilities:
  - type: intensity
    range: [0, 1]
`,
			wantCode: "GOLC_FIXTURE_MANUFACTURER_EMPTY",
		},
		{
			name: "empty model",
			yaml: `schema_version: 1
manufacturer: Generic
model: ""
modes:
  - name: Standard
capabilities:
  - type: intensity
    range: [0, 1]
`,
			wantCode: "GOLC_FIXTURE_MODEL_EMPTY",
		},
		{
			name: "missing model",
			yaml: `schema_version: 1
manufacturer: Generic
modes:
  - name: Standard
capabilities:
  - type: intensity
    range: [0, 1]
`,
			wantCode: "GOLC_FIXTURE_MODEL_EMPTY",
		},
		{
			name: "zero modes",
			yaml: `schema_version: 1
manufacturer: Generic
model: RGB PAR
modes: []
capabilities:
  - type: intensity
    range: [0, 1]
`,
			wantCode: "GOLC_FIXTURE_MODES_EMPTY",
		},
		{
			name: "missing modes",
			yaml: `schema_version: 1
manufacturer: Generic
model: RGB PAR
capabilities:
  - type: intensity
    range: [0, 1]
`,
			wantCode: "GOLC_FIXTURE_MODES_EMPTY",
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
			require.ErrorContains(t, err, testCase.wantCode, "expected Decode to reject %q", testCase.name)
		})
	}
}

func TestDecodeAdjacency(t *testing.T) {
	touching := `schema_version: 1
manufacturer: Generic
model: Strobe PAR
modes:
  - name: Standard
    channels:
      - type: shutter
        occurrence: 0
      - type: shutter
        occurrence: 1
capabilities:
  - type: shutter
    range: [0, 0.5]
    comment: closed
  - type: shutter
    range: [0.5, 1]
    comment: strobe
`
	def, err := fixture.Decode([]byte(touching))
	require.NoError(t, err, "expected exactly-adjacent same-type ranges to load")
	require.Len(t, def.Capabilities, 2)

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
	require.ErrorContains(t, err, "GOLC_FIXTURE_CAPABILITY_RANGE_INVALID", "expected overlapping same-type ranges to be rejected")
}

func TestDecodeDeterministic(t *testing.T) {
	first, err := fixture.Decode([]byte(validRGBParYAML))
	require.NoError(t, err, "first Decode failed")
	second, err := fixture.Decode([]byte(validRGBParYAML))
	require.NoError(t, err, "second Decode failed")

	for i := range first.Capabilities {
		require.Equal(t, second.Capabilities[i].Type, first.Capabilities[i].Type, "capability declared order drifted at index %d", i)
	}

	firstEncoded, err := strictjson.CanonicalEncode(first)
	require.NoError(t, err, "CanonicalEncode(first) failed")
	secondEncoded, err := strictjson.CanonicalEncode(second)
	require.NoError(t, err, "CanonicalEncode(second) failed")
	require.Equal(t, string(secondEncoded), string(firstEncoded), "expected byte-identical canonical summary across repeated decodes")
}

// TestChannelLayout proves D-16/D-17's ordered DMX channel-layout
// contract: a valid multi-channel layout decodes with its ChannelSlot
// entries in declared order; an empty/absent layout, an unknown
// capability type, and an occurrence index with no matching declared
// Capability are each rejected with their own actionable GOLC_FIXTURE_*
// diagnostic.
func TestChannelLayout(t *testing.T) {
	t.Run("valid multi-channel layout", func(t *testing.T) {
		validYAML := `schema_version: 1
manufacturer: Generic
model: RGB PAR
modes:
  - name: Standard
    channels:
      - type: intensity
        occurrence: 0
      - type: color
        occurrence: 0
capabilities:
  - type: intensity
    range: [0, 1]
  - type: color
    range: [0, 1]
`
		def, err := fixture.Decode([]byte(validYAML))
		require.NoError(t, err, "expected a valid multi-channel layout to decode")
		require.Len(t, def.Modes[0].Channels, 2, "expected 2 channel slots, got %+v", def.Modes[0].Channels)
		require.Equal(t, fixture.CapabilityIntensity, def.Modes[0].Channels[0].Type, "expected first channel slot intensity (declared order)")
		require.Equal(t, fixture.CapabilityColor, def.Modes[0].Channels[1].Type, "expected second channel slot color (declared order)")
	})

	t.Run("empty channel layout rejected", func(t *testing.T) {
		yaml := `schema_version: 1
manufacturer: Generic
model: RGB PAR
modes:
  - name: Standard
capabilities:
  - type: intensity
    range: [0, 1]
`
		_, err := fixture.Decode([]byte(yaml))
		require.ErrorContains(t, err, "GOLC_FIXTURE_CHANNEL_LAYOUT_MISSING", "expected a mode with no declared channels to be rejected")
	})

	t.Run("unknown channel capability type rejected", func(t *testing.T) {
		yaml := `schema_version: 1
manufacturer: Generic
model: RGB PAR
modes:
  - name: Standard
    channels:
      - type: not-a-real-capability
        occurrence: 0
capabilities:
  - type: intensity
    range: [0, 1]
`
		_, err := fixture.Decode([]byte(yaml))
		require.ErrorContains(t, err, "GOLC_FIXTURE_CHANNEL_TYPE_UNKNOWN", "expected an unknown channel capability type to be rejected")
	})

	t.Run("invalid occurrence rejected", func(t *testing.T) {
		yaml := `schema_version: 1
manufacturer: Generic
model: RGB PAR
modes:
  - name: Standard
    channels:
      - type: intensity
        occurrence: 1
capabilities:
  - type: intensity
    range: [0, 1]
`
		_, err := fixture.Decode([]byte(yaml))
		require.ErrorContains(t, err, "GOLC_FIXTURE_CHANNEL_OCCURRENCE_INVALID", "expected an occurrence index with no matching declared capability to be rejected")
	})
}
