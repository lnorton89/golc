// provenance_test.go proves FIXT-06's provenance record contract
// (02-02-PLAN.md, Task 1): a hand-authored fixture's Provenance reports
// Source, SchemaVersion, ContentHash, ValidationResult, and an initially
// empty Warnings list; a Provenance carrying a LossyImportWarning
// surfaces it distinctly (populated by 02-03's OFL import).
//
// This file intentionally fails to compile until
// internal/fixture/provenance.go exists (Task 2 of 02-02-PLAN.md) -- that
// is the RED state this task proves.
package fixture_test

import (
	"testing"

	"github.com/lnorton89/golc/internal/fixture"
)

const provenanceRGBParYAML = `schema_version: 1
manufacturer: Generic
model: RGB PAR
modes:
  - name: Standard
capabilities:
  - type: intensity
    range: [0, 1]
  - type: color
    range: [0, 1]
`

func TestProvenance(t *testing.T) {
	def, err := fixture.Decode([]byte(provenanceRGBParYAML))
	if err != nil {
		t.Fatalf("Decode(provenanceRGBParYAML) failed: %v", err)
	}
	identity, err := fixture.Pin(def)
	if err != nil {
		t.Fatalf("Pin(def) failed: %v", err)
	}

	const source = "internal/fixture/testdata/rgb-par.yaml"
	provenance := fixture.NewProvenance(def, identity, source)

	if provenance.Source != source {
		t.Fatalf("expected Source %q, got %q", source, provenance.Source)
	}
	if provenance.SchemaVersion != identity.SchemaVersion {
		t.Fatalf("expected SchemaVersion %d, got %d", identity.SchemaVersion, provenance.SchemaVersion)
	}
	if provenance.ContentHash != identity.ContentHash {
		t.Fatalf("expected ContentHash %q, got %q", identity.ContentHash, provenance.ContentHash)
	}
	if provenance.ValidationResult != "valid" {
		t.Fatalf(`expected ValidationResult "valid", got %q`, provenance.ValidationResult)
	}
	if len(provenance.Warnings) != 0 {
		t.Fatalf("expected an initially empty Warnings list, got %+v", provenance.Warnings)
	}

	// A Provenance carrying a LossyImportWarning surfaces it distinctly,
	// independent of whether the source was hand-authored or imported.
	withWarning := provenance
	withWarning.Warnings = []fixture.LossyImportWarning{
		{
			Severity:       "warning",
			CapabilityType: string(fixture.CapabilityColor),
			Detail:         "OFL capability had no direct GOLC equivalent; approximated to color",
		},
	}
	if len(withWarning.Warnings) != 1 {
		t.Fatalf("expected exactly one warning, got %d", len(withWarning.Warnings))
	}
	warning := withWarning.Warnings[0]
	if warning.Severity != "warning" || warning.CapabilityType != string(fixture.CapabilityColor) || warning.Detail == "" {
		t.Fatalf("expected a distinct, fully-populated warning, got %+v", warning)
	}
	if len(provenance.Warnings) != 0 {
		t.Fatal("expected the original provenance's Warnings to remain untouched (copy, not alias)")
	}
}
