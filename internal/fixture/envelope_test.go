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
	"strings"
	"testing"

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
	if err != nil {
		t.Fatalf("Decode(valid RGB PAR): %v", err)
	}
	identity, err := fixture.Pin(def)
	if err != nil {
		t.Fatalf("Pin: %v", err)
	}
	provenance := fixture.NewProvenance(def, identity, "ofl:acme/test")
	provenance.Warnings = []fixture.LossyImportWarning{
		{Severity: "warning", CapabilityType: "gobo", Detail: "gobo wheel approximated as a static color"},
	}

	payload, err := strictjson.CanonicalEncode(fixture.ImportEnvelope{Definition: def, Provenance: provenance})
	if err != nil {
		t.Fatalf("CanonicalEncode(ImportEnvelope): %v", err)
	}

	envelope, err := fixture.DecodeEnvelope(payload)
	if err != nil {
		t.Fatalf("DecodeEnvelope: %v", err)
	}
	if envelope.Definition.Manufacturer != def.Manufacturer || envelope.Definition.Model != def.Model {
		t.Fatalf("expected envelope definition to match the source definition, got %+v", envelope.Definition)
	}
	if err := fixture.Validate(envelope.Definition); err != nil {
		t.Fatalf("expected the decoded envelope's definition to pass Validate, got %v", err)
	}
	if len(envelope.Provenance.Warnings) != 1 || envelope.Provenance.Warnings[0].Detail != provenance.Warnings[0].Detail {
		t.Fatalf("expected provenance warnings to survive decode, got %+v", envelope.Provenance.Warnings)
	}
}

// TestDecodeEnvelopeRejectsBareDefinition proves a bare definition document
// (the definition's own fields at the JSON top level, with no nested
// "definition" key) is rejected with GOLC_FIXTURE_ENVELOPE_INVALID rather
// than silently producing a zero-valued definition -- DecodeEnvelope never
// falls back to treating the whole document as a definition.
func TestDecodeEnvelopeRejectsBareDefinition(t *testing.T) {
	bare := []byte(`{"schema_version":1,"manufacturer":"Generic","model":"RGB PAR","modes":[],"capabilities":[]}`)
	_, err := fixture.DecodeEnvelope(bare)
	if err == nil {
		t.Fatal("expected a bare definition document (no \"definition\" key) to be rejected")
	}
	if !strings.Contains(err.Error(), "GOLC_FIXTURE_ENVELOPE_INVALID") {
		t.Fatalf("expected GOLC_FIXTURE_ENVELOPE_INVALID, got %v", err)
	}
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
	if err == nil {
		t.Fatal("expected an envelope with an invalid nested definition to be rejected")
	}
	if !strings.Contains(err.Error(), "GOLC_FIXTURE_ENVELOPE_INVALID") {
		t.Fatalf("expected GOLC_FIXTURE_ENVELOPE_INVALID, got %v", err)
	}
}
