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

	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/fixture"
)

const provenanceRGBParYAML = `schema_version: 1
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

func TestProvenance(t *testing.T) {
	def, err := fixture.Decode([]byte(provenanceRGBParYAML))
	require.NoError(t, err, "Decode(provenanceRGBParYAML) failed")
	identity, err := fixture.Pin(def)
	require.NoError(t, err, "Pin(def) failed")

	const source = "internal/fixture/testdata/rgb-par.yaml"
	provenance := fixture.NewProvenance(def, identity, source)

	require.Equal(t, source, provenance.Source, "expected Source %q, got %q", source, provenance.Source)
	require.Equal(t, identity.SchemaVersion, provenance.SchemaVersion, "expected SchemaVersion %d, got %d", identity.SchemaVersion, provenance.SchemaVersion)
	require.Equal(t, identity.ContentHash, provenance.ContentHash, "expected ContentHash %q, got %q", identity.ContentHash, provenance.ContentHash)
	require.Equal(t, "valid", provenance.ValidationResult, `expected ValidationResult "valid", got %q`, provenance.ValidationResult)
	require.Empty(t, provenance.Warnings, "expected an initially empty Warnings list, got %+v", provenance.Warnings)

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
	require.Len(t, withWarning.Warnings, 1, "expected exactly one warning, got %d", len(withWarning.Warnings))
	warning := withWarning.Warnings[0]
	require.Equal(t, "warning", warning.Severity, "expected a distinct, fully-populated warning, got %+v", warning)
	require.Equal(t, string(fixture.CapabilityColor), warning.CapabilityType, "expected a distinct, fully-populated warning, got %+v", warning)
	require.NotEmpty(t, warning.Detail, "expected a distinct, fully-populated warning, got %+v", warning)
	require.Empty(t, provenance.Warnings, "expected the original provenance's Warnings to remain untouched (copy, not alias)")
}
