// normalize_test.go proves FIXT-03/FIXT-06's OFL-import normalization
// contract (02-03-PLAN.md, Task 1 Wave-0 scaffold): a corpus OFL fixture
// normalizes into a FixtureDefinition whose capabilities validate and pin
// through the exact same pipeline an equivalent hand-authored fixture
// uses; an OFL construct outside the v1 target set (pixel/matrix) still
// imports, surfacing its unmodeled constructs as explicit warnings rather
// than failing (D-06); and every OFL capability the v1 canonical model
// does not represent is accounted for by at least one warning -- nothing
// vanishes silently.
//
// This file compiles today only once package
// github.com/lnorton89/golc/internal/fixture/ofl exists; until Task 2
// creates model.go/normalize.go, "go test ./internal/fixture/ofl/..."
// fails to build at all -- that is the RED state this task proves.
package ofl_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/lnorton89/golc/internal/fixture"
	"github.com/lnorton89/golc/internal/fixture/ofl"
)

// equivalentHandAuthoredYAML declares the same capability-type set
// TestNormalizeCanonicalPipeline's OFL corpus fixture normalizes into
// (intensity, color, shutter, strobe): a hand-authored fixture proving
// the OFL-imported result runs the identical validate+pin pipeline, not
// a parallel one.
const equivalentHandAuthoredYAML = `schema_version: 1
manufacturer: Test
model: Equivalent RGB PAR
modes:
  - name: Standard
    channels:
      - type: intensity
        occurrence: 0
      - type: color
        occurrence: 0
      - type: shutter
        occurrence: 0
      - type: strobe
        occurrence: 0
capabilities:
  - type: intensity
    range: [0, 1]
  - type: color
    range: [0, 1]
  - type: shutter
    range: [0, 1]
  - type: strobe
    range: [0, 1]
`

func readCorpusFile(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join("..", "..", "..", "tests", "fixtures", "ofl", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading OFL corpus file %s: %v", name, err)
	}
	return data
}

func capabilityTypes(def fixture.FixtureDefinition) []fixture.CapabilityType {
	types := make([]fixture.CapabilityType, 0, len(def.Capabilities))
	for _, capability := range def.Capabilities {
		types = append(types, capability.Type)
	}
	return types
}

func warningsMention(warnings []fixture.LossyImportWarning, substr string) bool {
	for _, warning := range warnings {
		if strings.Contains(warning.Detail, substr) {
			return true
		}
	}
	return false
}

// TestNormalizeCanonicalPipeline proves FIXT-03: an OFL fixture (a
// generic RGB PAR: chauvet-dj LED PAR 64 TRI-B) normalizes into a
// FixtureDefinition whose capabilities validate and pin identically to an
// equivalent hand-authored fixture declaring the same capability types.
func TestNormalizeCanonicalPipeline(t *testing.T) {
	raw := readCorpusFile(t, "chauvet-dj_led-par-64-tri-b.json")

	def, provenance, err := ofl.Normalize(raw, "chauvet-dj/led-par-64-tri-b")
	if err != nil {
		t.Fatalf("Normalize failed: %v", err)
	}
	if def.SchemaVersion != 1 {
		t.Fatalf("expected schema_version 1, got %d", def.SchemaVersion)
	}
	if def.Manufacturer != "chauvet-dj" {
		t.Fatalf("expected manufacturer %q, got %q", "chauvet-dj", def.Manufacturer)
	}
	if provenance.ValidationResult != "valid" {
		t.Fatalf("expected ValidationResult %q, got %q", "valid", provenance.ValidationResult)
	}
	if len(provenance.ContentHash) != 64 {
		t.Fatalf("expected a 64-character hex content hash, got %q", provenance.ContentHash)
	}
	if provenance.Source != "ofl:chauvet-dj/led-par-64-tri-b" {
		t.Fatalf("expected Source %q, got %q", "ofl:chauvet-dj/led-par-64-tri-b", provenance.Source)
	}

	wantTypes := []fixture.CapabilityType{
		fixture.CapabilityIntensity, fixture.CapabilityColor, fixture.CapabilityShutter, fixture.CapabilityStrobe,
	}
	if gotTypes := capabilityTypes(def); !reflect.DeepEqual(gotTypes, wantTypes) {
		t.Fatalf("expected capability types %v in declared-enum order, got %v", wantTypes, gotTypes)
	}

	// The identical pipeline must accept an equivalent hand-authored
	// fixture declaring the same capability types (FIXT-03: OFL import
	// lands in the same canonical model and runs the same validate+pin
	// pipeline as hand-authored YAML, not a parallel one).
	handAuthored, err := fixture.Decode([]byte(equivalentHandAuthoredYAML))
	if err != nil {
		t.Fatalf("hand-authored equivalent fixture failed to decode/validate: %v", err)
	}
	if gotTypes := capabilityTypes(handAuthored); !reflect.DeepEqual(gotTypes, wantTypes) {
		t.Fatalf("hand-authored equivalent fixture capability types diverged from the OFL-normalized types: got %v", gotTypes)
	}
	if _, err := fixture.Pin(handAuthored); err != nil {
		t.Fatalf("hand-authored equivalent fixture failed to pin: %v", err)
	}
}

// TestNormalizeModeChannels proves D-16: an OFL mode declaring channel
// keys normalizes into a populated fixture.Mode.Channels, resolved
// through the same fixture.Validate path a hand-authored fixture's
// channel layout runs through (no second, independently-evolving copy of
// the validation logic).
func TestNormalizeModeChannels(t *testing.T) {
	raw := readCorpusFile(t, "chauvet-dj_led-par-64-tri-b.json")

	def, _, err := ofl.Normalize(raw, "chauvet-dj/led-par-64-tri-b")
	if err != nil {
		t.Fatalf("Normalize failed: %v", err)
	}
	if len(def.Modes) == 0 {
		t.Fatal("expected at least one normalized mode")
	}
	if len(def.Modes[0].Channels) == 0 {
		t.Fatalf("expected mode %q to normalize into a non-empty Channels layout (D-16), got %+v", def.Modes[0].Name, def.Modes[0])
	}
}

// TestNormalizeLossyWarning proves FIXT-06/D-06: an OFL fixture outside
// the v1 target set (chauvet-dj WashFX, a pixel/matrix wash) still
// imports successfully, surfacing its unmodeled pixel/matrix construct as
// an explicit LossyImportWarning rather than failing.
func TestNormalizeLossyWarning(t *testing.T) {
	raw := readCorpusFile(t, "chauvet-dj_washfx.json")

	def, provenance, err := ofl.Normalize(raw, "chauvet-dj/washfx")
	if err != nil {
		t.Fatalf("Normalize failed for an out-of-v1-target-set (pixel/matrix) OFL fixture: %v", err)
	}
	if len(def.Capabilities) == 0 {
		t.Fatal("expected the fixture to still import with at least one mapped capability")
	}
	if len(provenance.Warnings) == 0 {
		t.Fatal("expected at least one LossyImportWarning for the unmodeled pixel/matrix construct")
	}
	if !warningsMention(provenance.Warnings, "pixel/matrix") {
		t.Fatalf("expected a warning naming the unmodeled pixel/matrix construct, got %+v", provenance.Warnings)
	}
}

// TestNormalizeNoSilentDrop proves FIXT-06/D-06's strongest guarantee:
// every OFL availableChannel/templateChannel capability the v1 canonical
// model does not represent is accounted for by at least one warning.
// chauvet-dj WashFX's exact, hand-counted unmapped-construct set (18
// "Auto Program" entries + 1 "Auto Program Speed" + 3 "Auto or Sound
// Program" entries + 3 pixel/matrix template channels = 25) is asserted
// exactly, so a future accidental silent drop (or accidental
// double-count) is caught immediately.
func TestNormalizeNoSilentDrop(t *testing.T) {
	raw := readCorpusFile(t, "chauvet-dj_washfx.json")

	_, provenance, err := ofl.Normalize(raw, "chauvet-dj/washfx")
	if err != nil {
		t.Fatalf("Normalize failed: %v", err)
	}

	const wantWarnings = 25
	if len(provenance.Warnings) != wantWarnings {
		t.Fatalf("expected exactly %d warnings (every unmapped OFL construct accounted for), got %d: %+v",
			wantWarnings, len(provenance.Warnings), provenance.Warnings)
	}

	mustMention := []string{
		"Auto Program",
		"Auto Program Speed",
		"Auto or Sound Program",
		"Red $pixelKey",
		"Green $pixelKey",
		"Blue $pixelKey",
	}
	for _, name := range mustMention {
		if !warningsMention(provenance.Warnings, name) {
			t.Fatalf("expected at least one warning naming unmapped construct %q, got %+v", name, provenance.Warnings)
		}
	}
}

// TestNormalizeCorpusFixturesAllImport proves the full pinned corpus --
// spanning D-05's v1 target set (a generic RGB PAR, an LED wash, a
// moving-head spot, and a moving-head wash) -- imports without error and
// always reports a non-nil Warnings slice (fixture.Provenance's own
// contract: never nil-vs-empty-ambiguous JSON).
func TestNormalizeCorpusFixturesAllImport(t *testing.T) {
	corpus := map[string]string{
		"chauvet-dj_led-par-64-tri-b.json":     "chauvet-dj/led-par-64-tri-b",
		"chauvet-dj_washfx.json":               "chauvet-dj/washfx",
		"chauvet-dj_intimidator-spot-260.json": "chauvet-dj/intimidator-spot-260",
		"american-dj_vizi-q-wash7.json":        "american-dj/vizi-q-wash7",
	}
	for file, source := range corpus {
		t.Run(file, func(t *testing.T) {
			raw := readCorpusFile(t, file)
			def, provenance, err := ofl.Normalize(raw, source)
			if err != nil {
				t.Fatalf("Normalize(%s) failed: %v", file, err)
			}
			if len(def.Capabilities) == 0 {
				t.Fatalf("Normalize(%s) produced zero capabilities", file)
			}
			if provenance.Warnings == nil {
				t.Fatalf("Normalize(%s) produced a nil Warnings slice; must always be non-nil", file)
			}
		})
	}
}
