// envelope_test.go proves ImportEnvelope/DecodeEnvelope's contract
// (09-05-PLAN.md Task 1 RED / Task 2 GREEN): the exact bytes "fixture
// import" writes decode into an ImportEnvelope whose definition passes
// fixture.Validate (the identical validation the YAML path and
// ofl.Normalize both use) and whose provenance warnings survive; a bare
// definition document with no "definition" key, and a document whose
// nested definition fails fixture.Validate, are both rejected with
// GOLC_FIXTURE_ENVELOPE_INVALID rather than silently producing a
// zero-valued definition.
package fixture_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/fixture"
	"github.com/lnorton89/golc/internal/strictjson"
)

// TestDecodeEnvelopeAcceptsImportArtifact builds the envelope test fixture
// by literally running the encode path "fixture import" uses
// (strictjson.CanonicalEncode(fixture.ImportEnvelope{...}), mirroring
// internal/command/fixture.go's runFixtureImport) so this test breaks if
// the artifact shape ever drifts.
func TestDecodeEnvelopeAcceptsImportArtifact(t *testing.T) {
	def, err := fixture.Decode([]byte(validRGBParYAML))
	require.NoError(t, err, "Decode(valid RGB PAR)")
	identity, err := fixture.Pin(def)
	require.NoError(t, err, "Pin")
	provenance := fixture.NewProvenance(def, identity, "ofl:acme/test")
	provenance.Warnings = []fixture.LossyImportWarning{
		{Severity: "warning", CapabilityType: "gobo", Detail: "gobo wheel approximated as a static color"},
	}

	payload, err := strictjson.CanonicalEncode(fixture.ImportEnvelope{Definition: def, Provenance: provenance})
	require.NoError(t, err, "CanonicalEncode(ImportEnvelope)")

	envelope, err := fixture.DecodeEnvelope(payload)
	require.NoError(t, err, "DecodeEnvelope")
	require.Equal(t, def.Manufacturer, envelope.Definition.Manufacturer, "expected envelope definition to match the source definition, got %+v", envelope.Definition)
	require.Equal(t, def.Model, envelope.Definition.Model, "expected envelope definition to match the source definition, got %+v", envelope.Definition)
	require.NoError(t, fixture.Validate(envelope.Definition), "expected the decoded envelope's definition to pass Validate")
	require.Len(t, envelope.Provenance.Warnings, 1, "expected provenance warnings to survive decode, got %+v", envelope.Provenance.Warnings)
	require.Equal(t, provenance.Warnings[0].Detail, envelope.Provenance.Warnings[0].Detail, "expected provenance warnings to survive decode")
}

// TestDecodeEnvelopeRejectsBareDefinition proves a bare definition document
// (the definition's own fields at the JSON top level, with no nested
// "definition" key) is rejected with GOLC_FIXTURE_ENVELOPE_INVALID rather
// than silently producing a zero-valued definition -- DecodeEnvelope never
// falls back to treating the whole document as a definition.
func TestDecodeEnvelopeRejectsBareDefinition(t *testing.T) {
	bare := []byte(`{"schema_version":1,"manufacturer":"Generic","model":"RGB PAR","modes":[],"capabilities":[]}`)
	_, err := fixture.DecodeEnvelope(bare)
	require.ErrorContains(t, err, "GOLC_FIXTURE_ENVELOPE_INVALID", "expected a bare definition document (no \"definition\" key) to be rejected")
}

// TestDecodeEnvelopeRejectsInvalidDefinition proves an envelope whose
// nested definition fails fixture.Validate (an empty manufacturer, here)
// is rejected with GOLC_FIXTURE_ENVELOPE_INVALID -- DecodeEnvelope
// introduces no new validation rule and no lenient fallback, it runs the
// exact same fixture.Validate the YAML path and ofl.Normalize both use.
func TestDecodeEnvelopeRejectsInvalidDefinition(t *testing.T) {
	invalid := []byte(`{
		"definition": {"schema_version":1,"manufacturer":"","model":"RGB PAR","modes":[],"capabilities":[]},
		"provenance": {"source":"ofl:acme/test","schema_version":1,"content_hash":"x","revision":"x","validation_result":"valid","warnings":[]}
	}`)
	_, err := fixture.DecodeEnvelope(invalid)
	require.ErrorContains(t, err, "GOLC_FIXTURE_ENVELOPE_INVALID", "expected an envelope with an invalid nested definition to be rejected")
}
