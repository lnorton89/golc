// directory_test.go proves ListDirectory's three behaviors (09-01-PLAN.md
// Task 1 RED / Task 2 GREEN, CONTEXT D-01's local half): a non-recursive
// scan that skips non-fixture files and subdirectories, a per-entry
// decode/pin failure recorded on that entry without aborting the rest of
// the scan (T-09-01-02: one bad file never blanks the whole listing), and
// a not-yet-created directory surfacing as a GOLC_FIXTURE_DIR_READ_FAILED
// error that satisfies errors.Is(err, fs.ErrNotExist) so a caller (e.g.
// FixtureLibraryService.ListLocal) can distinguish "empty library" from a
// genuine read failure. Reuses decode_test.go's own package-level
// validRGBParYAML constant (same package fixture_test) rather than
// inventing a second minimal fixture shape.
package fixture_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lnorton89/golc/internal/fixture"
	"github.com/lnorton89/golc/internal/strictjson"
)

func writeDirectoryTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", name, err)
	}
}

// TestListDirectorySkipsNonFixtureFiles proves a non-recursive scan
// returns exactly the *.yaml/*.yml files directly inside dir, in
// os.ReadDir's own deterministic filename order -- a non-fixture file and
// a subdirectory are both silently skipped, never counted as an entry.
func TestListDirectorySkipsNonFixtureFiles(t *testing.T) {
	dir := t.TempDir()
	writeDirectoryTestFile(t, dir, "a.yaml", validRGBParYAML)
	writeDirectoryTestFile(t, dir, "b.yml", validRGBParYAML)
	writeDirectoryTestFile(t, dir, "notes.txt", "this is not a fixture")
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0o755); err != nil {
		t.Fatalf("Mkdir(subdir): %v", err)
	}

	entries, err := fixture.ListDirectory(dir)
	if err != nil {
		t.Fatalf("ListDirectory: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected exactly 2 YAML entries, got %d: %+v", len(entries), entries)
	}
	if entries[0].FileName != "a.yaml" || entries[1].FileName != "b.yml" {
		t.Fatalf("expected os.ReadDir filename order a.yaml, b.yml, got %q, %q", entries[0].FileName, entries[1].FileName)
	}
}

// TestListDirectoryRecordsPerEntryFailureWithoutAbortingScan proves a
// malformed fixture file's failure is recorded on that entry's own Err
// field -- the scan continues past it and still returns the valid
// entry's populated Identity, never aborting the whole scan on the first
// bad file.
func TestListDirectoryRecordsPerEntryFailureWithoutAbortingScan(t *testing.T) {
	dir := t.TempDir()
	writeDirectoryTestFile(t, dir, "good.yaml", validRGBParYAML)
	// An empty document is Decode's own guaranteed-rejection case
	// (GOLC_FIXTURE_EMPTY), independent of any YAML-syntax edge case.
	writeDirectoryTestFile(t, dir, "bad.yaml", "   \n")

	entries, err := fixture.ListDirectory(dir)
	if err != nil {
		t.Fatalf("ListDirectory: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries (one valid, one malformed), got %d: %+v", len(entries), entries)
	}

	var good, bad *fixture.DirectoryEntry
	for i := range entries {
		switch entries[i].FileName {
		case "good.yaml":
			good = &entries[i]
		case "bad.yaml":
			bad = &entries[i]
		}
	}
	if good == nil || bad == nil {
		t.Fatalf("expected both good.yaml and bad.yaml entries, got %+v", entries)
	}
	if good.Err != nil {
		t.Fatalf("expected good.yaml to carry no error, got %v", good.Err)
	}
	if good.Identity.StableKey == "" {
		t.Fatalf("expected good.yaml to carry a populated Identity, got %+v", good.Identity)
	}
	if bad.Err == nil {
		t.Fatalf("expected bad.yaml to carry a non-nil Err")
	}
}

// TestListDirectoryMissingDirectoryReturnsNotExistError proves a
// nonexistent directory returns an error satisfying
// errors.Is(err, fs.ErrNotExist), carrying GOLC_FIXTURE_DIR_READ_FAILED --
// the exact distinction FixtureLibraryService.ListLocal relies on to
// treat "no fixtures directory yet" as an empty library, not a broken one.
func TestListDirectoryMissingDirectoryReturnsNotExistError(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")

	_, err := fixture.ListDirectory(missing)
	if err == nil {
		t.Fatalf("expected an error for a nonexistent directory")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected errors.Is(err, fs.ErrNotExist), got %v", err)
	}
	if !strings.Contains(err.Error(), "GOLC_FIXTURE_DIR_READ_FAILED") {
		t.Fatalf("expected error to carry GOLC_FIXTURE_DIR_READ_FAILED, got %v", err)
	}
}

// buildImportEnvelopeBytes builds a byte-identical "fixture import --out"
// artifact (fixture.ImportEnvelope, canonically encoded exactly like
// runFixtureImport writes it -- see internal/command/fixture.go) so this
// test breaks if the artifact shape ever drifts.
func buildImportEnvelopeBytes(t *testing.T) []byte {
	t.Helper()
	def, err := fixture.Decode([]byte(validRGBParYAML))
	if err != nil {
		t.Fatalf("Decode(valid RGB PAR): %v", err)
	}
	identity, err := fixture.Pin(def)
	if err != nil {
		t.Fatalf("Pin: %v", err)
	}
	provenance := fixture.NewProvenance(def, identity, "ofl:acme/test")
	payload, err := strictjson.CanonicalEncode(fixture.ImportEnvelope{Definition: def, Provenance: provenance})
	if err != nil {
		t.Fatalf("CanonicalEncode(ImportEnvelope): %v", err)
	}
	return payload
}

// TestListDirectoryIncludesImportArtifacts proves a directory holding one
// .yaml definition and one .json import envelope returns both as valid
// entries with correct pinned identities (09-05-PLAN.md Task 2, FDUI-01) --
// the import artifact is no longer invisible to the library scan. A .json
// file that is not a valid envelope returns an entry carrying a non-nil
// Err, not a scan abort (mirrors a malformed .yaml file's identical
// per-entry-failure discipline).
func TestListDirectoryIncludesImportArtifacts(t *testing.T) {
	dir := t.TempDir()
	writeDirectoryTestFile(t, dir, "hand-authored.yaml", validRGBParYAML)
	writeDirectoryTestFile(t, dir, "imported.json", string(buildImportEnvelopeBytes(t)))
	writeDirectoryTestFile(t, dir, "malformed.json", `{"not":"an envelope"}`)

	entries, err := fixture.ListDirectory(dir)
	if err != nil {
		t.Fatalf("ListDirectory: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries (2 valid, 1 malformed), got %d: %+v", len(entries), entries)
	}

	var yamlEntry, jsonEntry, malformedEntry *fixture.DirectoryEntry
	for i := range entries {
		switch entries[i].FileName {
		case "hand-authored.yaml":
			yamlEntry = &entries[i]
		case "imported.json":
			jsonEntry = &entries[i]
		case "malformed.json":
			malformedEntry = &entries[i]
		}
	}
	if yamlEntry == nil || jsonEntry == nil || malformedEntry == nil {
		t.Fatalf("expected hand-authored.yaml, imported.json, and malformed.json entries, got %+v", entries)
	}

	if yamlEntry.Err != nil {
		t.Fatalf("expected hand-authored.yaml to carry no error, got %v", yamlEntry.Err)
	}
	if yamlEntry.Identity.StableKey == "" {
		t.Fatalf("expected hand-authored.yaml to carry a populated Identity, got %+v", yamlEntry.Identity)
	}

	if jsonEntry.Err != nil {
		t.Fatalf("expected imported.json to carry no error, got %v", jsonEntry.Err)
	}
	if jsonEntry.Identity.StableKey == "" {
		t.Fatalf("expected imported.json to carry a populated pinned Identity, got %+v", jsonEntry.Identity)
	}
	if jsonEntry.Definition.Manufacturer == "" || jsonEntry.Definition.Model == "" {
		t.Fatalf("expected imported.json to carry a populated Definition, got %+v", jsonEntry.Definition)
	}
	if jsonEntry.Provenance.Source != "ofl:acme/test" {
		t.Fatalf("expected imported.json to carry its envelope's Provenance, got %+v", jsonEntry.Provenance)
	}

	if malformedEntry.Err == nil {
		t.Fatalf("expected malformed.json to carry a non-nil Err")
	}
}
