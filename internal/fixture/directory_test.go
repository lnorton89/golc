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
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/fixture"
	"github.com/lnorton89/golc/internal/strictjson"
)

func writeDirectoryTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644), "WriteFile(%s)", name)
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
	require.NoError(t, os.Mkdir(filepath.Join(dir, "subdir"), 0o755), "Mkdir(subdir)")

	entries, err := fixture.ListDirectory(dir)
	require.NoError(t, err, "ListDirectory")
	require.Len(t, entries, 2, "expected exactly 2 YAML entries, got %+v", entries)
	require.Equal(t, "a.yaml", entries[0].FileName, "expected os.ReadDir filename order a.yaml, b.yml")
	require.Equal(t, "b.yml", entries[1].FileName, "expected os.ReadDir filename order a.yaml, b.yml")
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
	require.NoError(t, err, "ListDirectory")
	require.Len(t, entries, 2, "expected 2 entries (one valid, one malformed), got %+v", entries)

	var good, bad *fixture.DirectoryEntry
	for i := range entries {
		switch entries[i].FileName {
		case "good.yaml":
			good = &entries[i]
		case "bad.yaml":
			bad = &entries[i]
		}
	}
	require.NotNil(t, good, "expected both good.yaml and bad.yaml entries, got %+v", entries)
	require.NotNil(t, bad, "expected both good.yaml and bad.yaml entries, got %+v", entries)
	require.NoError(t, good.Err, "expected good.yaml to carry no error")
	require.NotEmpty(t, good.Identity.StableKey, "expected good.yaml to carry a populated Identity, got %+v", good.Identity)
	require.Error(t, bad.Err, "expected bad.yaml to carry a non-nil Err")
}

// TestListDirectoryMissingDirectoryReturnsNotExistError proves a
// nonexistent directory returns an error satisfying
// errors.Is(err, fs.ErrNotExist), carrying GOLC_FIXTURE_DIR_READ_FAILED --
// the exact distinction FixtureLibraryService.ListLocal relies on to
// treat "no fixtures directory yet" as an empty library, not a broken one.
func TestListDirectoryMissingDirectoryReturnsNotExistError(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")

	_, err := fixture.ListDirectory(missing)
	require.Error(t, err, "expected an error for a nonexistent directory")
	require.True(t, errors.Is(err, fs.ErrNotExist), "expected errors.Is(err, fs.ErrNotExist), got %v", err)
	require.Contains(t, err.Error(), "GOLC_FIXTURE_DIR_READ_FAILED")
}

// buildImportEnvelopeBytes builds a byte-identical "fixture import --out"
// artifact (fixture.ImportEnvelope, canonically encoded exactly like
// runFixtureImport writes it -- see internal/command/fixture.go) so this
// test breaks if the artifact shape ever drifts.
func buildImportEnvelopeBytes(t *testing.T) []byte {
	t.Helper()
	def, err := fixture.Decode([]byte(validRGBParYAML))
	require.NoError(t, err, "Decode(valid RGB PAR)")
	identity, err := fixture.Pin(def)
	require.NoError(t, err, "Pin")
	provenance := fixture.NewProvenance(def, identity, "ofl:acme/test")
	payload, err := strictjson.CanonicalEncode(fixture.ImportEnvelope{Definition: def, Provenance: provenance})
	require.NoError(t, err, "CanonicalEncode(ImportEnvelope)")
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
	require.NoError(t, err, "ListDirectory")
	require.Len(t, entries, 3, "expected 3 entries (2 valid, 1 malformed), got %+v", entries)

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
	require.NotNil(t, yamlEntry, "expected hand-authored.yaml, imported.json, and malformed.json entries, got %+v", entries)
	require.NotNil(t, jsonEntry, "expected hand-authored.yaml, imported.json, and malformed.json entries, got %+v", entries)
	require.NotNil(t, malformedEntry, "expected hand-authored.yaml, imported.json, and malformed.json entries, got %+v", entries)

	require.NoError(t, yamlEntry.Err, "expected hand-authored.yaml to carry no error")
	require.NotEmpty(t, yamlEntry.Identity.StableKey, "expected hand-authored.yaml to carry a populated Identity, got %+v", yamlEntry.Identity)

	require.NoError(t, jsonEntry.Err, "expected imported.json to carry no error")
	require.NotEmpty(t, jsonEntry.Identity.StableKey, "expected imported.json to carry a populated pinned Identity, got %+v", jsonEntry.Identity)
	require.NotEmpty(t, jsonEntry.Definition.Manufacturer, "expected imported.json to carry a populated Definition, got %+v", jsonEntry.Definition)
	require.NotEmpty(t, jsonEntry.Definition.Model, "expected imported.json to carry a populated Definition, got %+v", jsonEntry.Definition)
	require.Equal(t, "ofl:acme/test", jsonEntry.Provenance.Source, "expected imported.json to carry its envelope's Provenance, got %+v", jsonEntry.Provenance)

	require.Error(t, malformedEntry.Err, "expected malformed.json to carry a non-nil Err")
}
