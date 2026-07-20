package bootstrap

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/invopop/jsonschema"
)

// buildArchive writes a zip archive containing the given entry names and
// contents, returning the archive path and its lowercase hex SHA-256.
func buildArchive(t *testing.T, dir string, entries map[string]string) (string, string) {
	t.Helper()

	archivePath := filepath.Join(dir, "tool-archive.zip")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}
	writer := zip.NewWriter(file)
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	// Deterministic entry order keeps archive bytes stable per test run.
	for i := 0; i < len(names); i++ {
		for j := i + 1; j < len(names); j++ {
			if names[j] < names[i] {
				names[i], names[j] = names[j], names[i]
			}
		}
	}
	for _, name := range names {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatalf("create entry %q: %v", name, err)
		}
		if _, err := entry.Write([]byte(entries[name])); err != nil {
			t.Fatalf("write entry %q: %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close archive file: %v", err)
	}

	raw, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("read archive back: %v", err)
	}
	digest := sha256.Sum256(raw)
	return archivePath, hex.EncodeToString(digest[:])
}

func TestVerifyArchiveAcceptsMatchingChecksum(t *testing.T) {
	dir := t.TempDir()
	archivePath, digest := buildArchive(t, dir, map[string]string{
		"bin/golc-project.exe": "payload\n",
	})

	if err := VerifyArchive(archivePath, digest); err != nil {
		t.Fatalf("expected matching archive to verify, got: %v", err)
	}
}

func TestVerifyArchiveRejectsChecksumMismatch(t *testing.T) {
	dir := t.TempDir()
	archivePath, _ := buildArchive(t, dir, map[string]string{
		"bin/golc-project.exe": "payload\n",
	})
	wrong := strings.Repeat("ab", 32)

	err := VerifyArchive(archivePath, wrong)
	if err == nil {
		t.Fatal("expected checksum mismatch error, got nil")
	}
	if !strings.Contains(err.Error(), "BOOTSTRAP_CHECKSUM_MISMATCH") {
		t.Fatalf("expected BOOTSTRAP_CHECKSUM_MISMATCH diagnostic, got: %v", err)
	}
}

func TestVerifyArchiveRejectsMalformedExpectedChecksum(t *testing.T) {
	dir := t.TempDir()
	archivePath, _ := buildArchive(t, dir, map[string]string{
		"bin/golc-project.exe": "payload\n",
	})

	err := VerifyArchive(archivePath, "NOT-A-DIGEST")
	if err == nil {
		t.Fatal("expected malformed checksum error, got nil")
	}
	if !strings.Contains(err.Error(), "BOOTSTRAP_CHECKSUM_FORMAT") {
		t.Fatalf("expected BOOTSTRAP_CHECKSUM_FORMAT diagnostic, got: %v", err)
	}
}

func TestVerifyArchiveRejectsPathTraversalEntries(t *testing.T) {
	dir := t.TempDir()
	for name, entry := range map[string]string{
		"dot-dot":    "../escape.txt",
		"rooted":     "/rooted.txt",
		"drive":      "c:/windows/escape.txt",
		"backslash":  "..\\escape.txt",
		"middle-dot": "bin/../../escape.txt",
	} {
		caseDir := filepath.Join(dir, name)
		if err := os.MkdirAll(caseDir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		archivePath, digest := buildArchive(t, caseDir, map[string]string{
			entry: "escape\n",
		})

		err := VerifyArchive(archivePath, digest)
		if err == nil {
			t.Fatalf("%s: expected traversal rejection for entry %q, got nil", name, entry)
		}
		if !strings.Contains(err.Error(), "BOOTSTRAP_ARCHIVE_TRAVERSAL") {
			t.Fatalf("%s: expected BOOTSTRAP_ARCHIVE_TRAVERSAL diagnostic, got: %v", name, err)
		}
	}
}

func TestInstallStagedPromotesVerifiedArchiveAtomically(t *testing.T) {
	dir := t.TempDir()
	archivePath, digest := buildArchive(t, dir, map[string]string{
		"bin/golc-project.exe": "tool payload\n",
		"share/notes.txt":      "notes\n",
	})
	installDir := filepath.Join(dir, "install", "golc_project")

	if err := InstallStaged(archivePath, digest, installDir); err != nil {
		t.Fatalf("expected staged install to succeed, got: %v", err)
	}

	payload, err := os.ReadFile(filepath.Join(installDir, "bin", "golc-project.exe"))
	if err != nil {
		t.Fatalf("promoted payload missing: %v", err)
	}
	if string(payload) != "tool payload\n" {
		t.Fatalf("promoted payload bytes changed: %q", payload)
	}

	manifestRaw, err := os.ReadFile(filepath.Join(installDir, ManifestName))
	if err != nil {
		t.Fatalf("install manifest missing: %v", err)
	}
	var manifest InstallManifest
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		t.Fatalf("install manifest is not valid JSON: %v", err)
	}
	if manifest.ArchiveSHA256 != digest {
		t.Fatalf("manifest archive hash %q does not match %q", manifest.ArchiveSHA256, digest)
	}
	if len(manifest.Files) != 2 {
		t.Fatalf("manifest should record 2 files, got %d", len(manifest.Files))
	}

	// No staging directory may survive promotion.
	parentEntries, err := os.ReadDir(filepath.Dir(installDir))
	if err != nil {
		t.Fatalf("read install parent: %v", err)
	}
	for _, entry := range parentEntries {
		if strings.Contains(entry.Name(), "staging") {
			t.Fatalf("staging directory %q survived promotion", entry.Name())
		}
	}
}

func TestInstallStagedLeavesNoInstallOnChecksumMismatch(t *testing.T) {
	dir := t.TempDir()
	archivePath, _ := buildArchive(t, dir, map[string]string{
		"bin/golc-project.exe": "tool payload\n",
	})
	installDir := filepath.Join(dir, "install", "golc_project")
	wrong := strings.Repeat("cd", 32)

	err := InstallStaged(archivePath, wrong, installDir)
	if err == nil {
		t.Fatal("expected checksum mismatch to fail the install, got nil")
	}
	if _, statErr := os.Stat(installDir); !os.IsNotExist(statErr) {
		t.Fatalf("checksum mismatch must leave no install, stat err: %v", statErr)
	}
}

func TestInstalledMatchesMakesSecondInstallSkipArchiveSource(t *testing.T) {
	dir := t.TempDir()
	archivePath, digest := buildArchive(t, dir, map[string]string{
		"bin/golc-project.exe": "tool payload\n",
	})
	installDir := filepath.Join(dir, "install", "golc_project")

	if err := InstallStaged(archivePath, digest, installDir); err != nil {
		t.Fatalf("first install failed: %v", err)
	}

	matches, err := InstalledMatches(installDir, digest)
	if err != nil {
		t.Fatalf("InstalledMatches failed: %v", err)
	}
	if !matches {
		t.Fatal("matching installed manifest must report true")
	}

	// The archive source is deleted: a matching manifest means the second
	// bootstrap pass never touches the archive source at all.
	if err := os.Remove(archivePath); err != nil {
		t.Fatalf("remove archive source: %v", err)
	}
	matches, err = InstalledMatches(installDir, digest)
	if err != nil {
		t.Fatalf("InstalledMatches after source removal failed: %v", err)
	}
	if !matches {
		t.Fatal("installed state must match without consulting the archive source")
	}
}

func TestInstalledMatchesRejectsTamperedInstall(t *testing.T) {
	dir := t.TempDir()
	archivePath, digest := buildArchive(t, dir, map[string]string{
		"bin/golc-project.exe": "tool payload\n",
	})
	installDir := filepath.Join(dir, "install", "golc_project")

	if err := InstallStaged(archivePath, digest, installDir); err != nil {
		t.Fatalf("install failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(installDir, "bin", "golc-project.exe"), []byte("tampered\n"), 0o644); err != nil {
		t.Fatalf("tamper install: %v", err)
	}

	matches, err := InstalledMatches(installDir, digest)
	if err != nil {
		t.Fatalf("InstalledMatches failed: %v", err)
	}
	if matches {
		t.Fatal("tampered installed bytes must not match the manifest")
	}

	otherDigest := strings.Repeat("ef", 32)
	matches, err = InstalledMatches(installDir, otherDigest)
	if err != nil {
		t.Fatalf("InstalledMatches with other digest failed: %v", err)
	}
	if matches {
		t.Fatal("a different pinned archive hash must not match the manifest")
	}

	matches, err = InstalledMatches(filepath.Join(dir, "never-installed"), digest)
	if err != nil {
		t.Fatalf("InstalledMatches on missing install failed: %v", err)
	}
	if matches {
		t.Fatal("a missing install must not match")
	}
}

// probeRuntime and probeConfig mirror the committed runtime concern shape so
// the bootstrap probe exercises both pinned modules end to end.
type probeRuntime struct {
	LogLevel string `toml:"log_level" json:"log_level"`
}

type probeConfig struct {
	SchemaVersion int          `toml:"schema_version" json:"schema_version"`
	Runtime       probeRuntime `toml:"runtime" json:"runtime"`
}

// TestSchemaProbeDecodesTOMLAndEmitsInvopopSchema is the bootstrap module
// probe: it resolves github.com/BurntSushi/toml by strictly decoding a
// concern-shaped document, then resolves github.com/invopop/jsonschema by
// reflecting a JSON Schema from the same Go type and emitting schema bytes.
// Bootstrap compiles and runs this probe online, and walking-skeleton
// bootstrap mode reruns it with GOPROXY=off, readonly module mode, and a
// fail-on-call network transport.
func TestSchemaProbeDecodesTOMLAndEmitsInvopopSchema(t *testing.T) {
	const document = "schema_version = 1\n\n[runtime]\nlog_level = \"info\"\n"

	var decoded probeConfig
	metadata, err := toml.Decode(document, &decoded)
	if err != nil {
		t.Fatalf("TOML decode failed: %v", err)
	}
	if undecoded := metadata.Undecoded(); len(undecoded) != 0 {
		t.Fatalf("strict decode left undecoded keys: %v", undecoded)
	}
	if decoded.SchemaVersion != 1 || decoded.Runtime.LogLevel != "info" {
		t.Fatalf("decoded unexpected values: %+v", decoded)
	}

	schema := jsonschema.Reflect(&probeConfig{})
	schemaBytes, err := json.Marshal(schema)
	if err != nil {
		t.Fatalf("schema marshal failed: %v", err)
	}
	emitted := string(schemaBytes)
	if !strings.Contains(emitted, "https://json-schema.org/draft/2020-12/schema") {
		t.Fatalf("schema bytes missing draft 2020-12 marker: %s", emitted)
	}
	for _, fragment := range []string{"schema_version", "log_level", "additionalProperties"} {
		if !strings.Contains(emitted, fragment) {
			t.Fatalf("schema bytes missing %q: %s", fragment, emitted)
		}
	}
}
