package bootstrap

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/invopop/jsonschema"
)

// The bootstrap-archive quick-test scope is declared through the exact
// production entrypoint (01-VALIDATION: every owning Go test task
// registers its scope through MustDeclareScope beside its TestScope
// marker, pattern set by config/config-local/config-strict). This file is
// package bootstrap (not an external _test package) because
// internal/bootstrap has no import cycle with internal/command — command
// never imports bootstrap.
// The bootstrap-cache quick-test scope covers Plan 01-28's project-local
// cache-layout/offline-environment contract (cache.go) and the directory
// primitive it shares with bootstrap.go's staged install.
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

type testArchiveEntry struct {
	Name     string
	Body     string
	Mode     os.FileMode
	Typeflag byte
	Linkname string
	Dir      bool
}

func buildZipEntries(t *testing.T, dir, name string, entries []testArchiveEntry) (string, string) {
	t.Helper()
	archivePath := filepath.Join(dir, name)
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	writer := zip.NewWriter(file)
	for _, item := range entries {
		header := &zip.FileHeader{Name: item.Name, Method: zip.Store}
		mode := item.Mode
		if mode == 0 {
			mode = 0o644
		}
		if item.Dir {
			mode |= os.ModeDir
		}
		header.SetMode(mode)
		entry, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatalf("create zip entry %q: %v", item.Name, err)
		}
		if _, err := entry.Write([]byte(item.Body)); err != nil {
			t.Fatalf("write zip entry %q: %v", item.Name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close zip file: %v", err)
	}
	return archivePath, digestFile(t, archivePath)
}

func buildTarGzEntries(t *testing.T, dir, name string, entries []testArchiveEntry) (string, string) {
	t.Helper()
	archivePath := filepath.Join(dir, name)
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create tar.gz: %v", err)
	}
	gzipWriter := gzip.NewWriter(file)
	writer := tar.NewWriter(gzipWriter)
	for _, item := range entries {
		typeflag := item.Typeflag
		if typeflag == 0 {
			if item.Dir {
				typeflag = tar.TypeDir
			} else {
				typeflag = tar.TypeReg
			}
		}
		mode := int64(item.Mode.Perm())
		if mode == 0 {
			mode = 0o644
		}
		header := &tar.Header{
			Name:     item.Name,
			Mode:     mode,
			Size:     int64(len(item.Body)),
			Typeflag: typeflag,
			Linkname: item.Linkname,
		}
		if typeflag != tar.TypeReg && typeflag != tar.TypeRegA {
			header.Size = 0
		}
		if err := writer.WriteHeader(header); err != nil {
			t.Fatalf("write tar header %q: %v", item.Name, err)
		}
		if header.Size > 0 {
			if _, err := writer.Write([]byte(item.Body)); err != nil {
				t.Fatalf("write tar entry %q: %v", item.Name, err)
			}
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close tar.gz file: %v", err)
	}
	return archivePath, digestFile(t, archivePath)
}

func digestFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
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
	const document = "schema_version = 2\n\n[runtime]\nlog_level = \"info\"\n"

	var decoded probeConfig
	metadata, err := toml.Decode(document, &decoded)
	if err != nil {
		t.Fatalf("TOML decode failed: %v", err)
	}
	if undecoded := metadata.Undecoded(); len(undecoded) != 0 {
		t.Fatalf("strict decode left undecoded keys: %v", undecoded)
	}
	if decoded.SchemaVersion != 2 || decoded.Runtime.LogLevel != "info" {
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

// writeTestToolchainManifest materializes a minimal config/toolchain.toml
// under a fresh repository root, declaring one official_host/
// official_path_prefix pin per entry in patterns (keyed by tool name).
func writeTestToolchainManifest(t *testing.T, root string, patterns map[string]SourcePattern) {
	t.Helper()

	var body strings.Builder
	body.WriteString("schema_version = 2\n\n")
	for name, pattern := range patterns {
		fmt.Fprintf(&body, "[toolchain.%s]\n", name)
		fmt.Fprintf(&body, "version = \"1.0.0\"\n")
		if pattern.Host != "" {
			fmt.Fprintf(&body, "official_host = %q\n", pattern.Host)
		}
		if pattern.PathPrefix != "" {
			fmt.Fprintf(&body, "official_path_prefix = %q\n", pattern.PathPrefix)
		}
		body.WriteString("\n")
	}

	configDir := filepath.Join(root, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "toolchain.toml"), []byte(body.String()), 0o644); err != nil {
		t.Fatalf("write toolchain.toml: %v", err)
	}
}

// buildZipWithSymlinkEntry writes a zip archive containing one file entry
// plus one symlink entry (encoded the same way archive/zip's
// FileHeader.SetMode round-trips a Unix symlink mode), returning the
// archive path and its lowercase hex SHA-256.
func buildZipWithSymlinkEntry(t *testing.T, dir string) (string, string) {
	t.Helper()

	archivePath := filepath.Join(dir, "symlink-archive.zip")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}
	writer := zip.NewWriter(file)

	regular, err := writer.Create("bin/golc-project.exe")
	if err != nil {
		t.Fatalf("create regular entry: %v", err)
	}
	if _, err := regular.Write([]byte("payload\n")); err != nil {
		t.Fatalf("write regular entry: %v", err)
	}

	header := &zip.FileHeader{Name: "bin/evil-link", Method: zip.Deflate}
	header.SetMode(os.ModeSymlink | 0o777)
	linkWriter, err := writer.CreateHeader(header)
	if err != nil {
		t.Fatalf("create symlink header: %v", err)
	}
	if _, err := linkWriter.Write([]byte("../../../etc/passwd")); err != nil {
		t.Fatalf("write symlink entry: %v", err)
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

// fakeSource is the only Source implementation any bootstrap-archive test
// uses: it serves fixed in-memory payloads keyed by exact URL and records
// call count, so tests can assert that a policy rejection never reaches
// the network layer at all.
type fakeSource struct {
	payload map[string][]byte
	calls   int
}

func (source *fakeSource) Fetch(rawURL string) (io.ReadCloser, error) {
	source.calls++
	body, ok := source.payload[rawURL]
	if !ok {
		return nil, fmt.Errorf("fakeSource has no payload for %q", rawURL)
	}
	return io.NopCloser(bytes.NewReader(body)), nil
}

// repositoryRoot resolves the real checkout root from the package
// directory (pattern set by internal/projectconfig/strict_test.go and
// internal/trace/catalog/catalog_test.go) so config/toolchain.toml is
// validated exactly as committed, not from a synthetic fixture.
func repositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "golc.project.toml")); err != nil {
		t.Fatalf("repository root %q has no golc.project.toml: %v", root, err)
	}
	return root
}

// TestScopeBootstrapArchive is the exact quick-test marker for scope
// "bootstrap-archive" (test --quick --scope bootstrap-archive). Every
// subtest uses only injected sources and locally built fixture archives —
// no live network call is ever made, so the registered scope exits 0
// offline.
func TestScopeBootstrapArchive(t *testing.T) {
	t.Run("ZIP and tar.gz install regular files with a current complete manifest", func(t *testing.T) {
		for _, format := range []string{"zip", "tar.gz"} {
			t.Run(format, func(t *testing.T) {
				dir := t.TempDir()
				entries := []testArchiveEntry{
					{Name: "tool/", Mode: 0o755, Dir: true},
					{Name: "tool/bin/run", Body: "payload\n", Mode: 0o755},
					{Name: "tool/share/readme.txt", Body: "notes\n", Mode: 0o640},
				}
				var archivePath, digest string
				if format == "zip" {
					archivePath, digest = buildZipEntries(t, dir, "tool.zip", entries)
				} else {
					archivePath, digest = buildTarGzEntries(t, dir, "tool.tar.gz", entries)
				}
				installDir := filepath.Join(dir, "install")
				if err := InstallStaged(archivePath, digest, installDir); err != nil {
					t.Fatalf("InstallStaged(%s): %v", format, err)
				}
				var manifest InstallManifest
				raw, err := os.ReadFile(filepath.Join(installDir, ManifestName))
				if err != nil {
					t.Fatalf("read manifest: %v", err)
				}
				if err := json.Unmarshal(raw, &manifest); err != nil {
					t.Fatalf("decode manifest: %v", err)
				}
				if manifest.SchemaVersion != InstallManifestSchemaVersion {
					t.Fatalf("schema_version = %d, want %d", manifest.SchemaVersion, InstallManifestSchemaVersion)
				}
				if len(manifest.Files) != 2 {
					t.Fatalf("manifest files = %d, want 2", len(manifest.Files))
				}
				if manifest.Files[0].Path != "tool/bin/run" || manifest.Files[0].Mode != "0755" {
					t.Fatalf("unexpected executable manifest entry: %+v", manifest.Files[0])
				}
				matches, err := InstalledMatches(installDir, digest)
				if err != nil || !matches {
					t.Fatalf("InstalledMatches = %v, %v", matches, err)
				}
				if runtime.GOOS != "windows" {
					info, err := os.Stat(filepath.Join(installDir, "tool", "bin", "run"))
					if err != nil {
						t.Fatalf("stat executable: %v", err)
					}
					if got := info.Mode().Perm(); got != 0o755 {
						t.Fatalf("executable mode = %04o, want 0755", got)
					}
				}
			})
		}
	})

	t.Run("unsafe ZIP and tar.gz entries are rejected before staging", func(t *testing.T) {
		unsafeNames := []string{"", "/rooted", `C:\rooted`, "../escape", `..\escape`, "safe/../../escape"}
		for _, format := range []string{"zip", "tar.gz"} {
			for index, name := range unsafeNames {
				t.Run(fmt.Sprintf("%s-name-%d", format, index), func(t *testing.T) {
					dir := t.TempDir()
					entryName := name
					if entryName == "" && format == "zip" {
						entryName = " "
					}
					entries := []testArchiveEntry{{Name: entryName, Body: "bad", Mode: 0o644}}
					var archivePath, digest string
					if format == "zip" {
						archivePath, digest = buildZipEntries(t, dir, "bad.zip", entries)
					} else {
						archivePath, digest = buildTarGzEntries(t, dir, "bad.tar.gz", entries)
					}
					parent := filepath.Join(dir, "parent")
					if _, err := ExtractVerified(archivePath, digest, parent); err == nil {
						t.Fatal("expected unsafe path rejection")
					}
					if _, err := os.Stat(parent); !os.IsNotExist(err) {
						t.Fatalf("inspection failure created extraction parent: %v", err)
					}
				})
			}
		}

		t.Run("normalized duplicate", func(t *testing.T) {
			for _, format := range []string{"zip", "tar.gz"} {
				dir := t.TempDir()
				entries := []testArchiveEntry{
					{Name: "bin/tool", Body: "one", Mode: 0o755},
					{Name: `bin\tool`, Body: "two", Mode: 0o755},
				}
				var archivePath, digest string
				if format == "zip" {
					archivePath, digest = buildZipEntries(t, dir, "duplicate.zip", entries)
				} else {
					archivePath, digest = buildTarGzEntries(t, dir, "duplicate.tar.gz", entries)
				}
				parent := filepath.Join(dir, "parent")
				if _, err := ExtractVerified(archivePath, digest, parent); err == nil || !strings.Contains(err.Error(), "BOOTSTRAP_ARCHIVE_DUPLICATE") {
					t.Fatalf("%s expected duplicate rejection, got %v", format, err)
				}
				if _, err := os.Stat(parent); !os.IsNotExist(err) {
					t.Fatalf("%s duplicate created extraction parent: %v", format, err)
				}
			}
		})
	})

	t.Run("tar.gz rejects links and special or unknown entry types", func(t *testing.T) {
		cases := []struct {
			name string
			kind byte
		}{
			{"symlink", tar.TypeSymlink},
			{"hardlink", tar.TypeLink},
			{"character-device", tar.TypeChar},
			{"block-device", tar.TypeBlock},
			{"fifo", tar.TypeFifo},
			{"unknown", byte('S')},
		}
		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				dir := t.TempDir()
				archivePath, digest := buildTarGzEntries(t, dir, "unsafe.tar.gz", []testArchiveEntry{{
					Name: "unsafe", Typeflag: testCase.kind, Linkname: "../outside",
				}})
				parent := filepath.Join(dir, "parent")
				if _, err := ExtractVerified(archivePath, digest, parent); err == nil ||
					(!strings.Contains(err.Error(), "BOOTSTRAP_ARCHIVE_UNSAFE_TYPE") && !strings.Contains(err.Error(), "BOOTSTRAP_ARCHIVE_FORMAT")) {
					t.Fatalf("expected unsafe type rejection, got %v", err)
				}
				if _, err := os.Stat(parent); !os.IsNotExist(err) {
					t.Fatalf("unsafe tar created extraction parent: %v", err)
				}
			})
		}
	})

	t.Run("archive suffix and content must agree", func(t *testing.T) {
		dir := t.TempDir()
		zipPath, digest := buildZipEntries(t, dir, "tool.tar.gz", []testArchiveEntry{{Name: "tool", Body: "zip"}})
		if _, err := ExtractVerified(zipPath, digest, filepath.Join(dir, "parent")); err == nil || !strings.Contains(err.Error(), "BOOTSTRAP_ARCHIVE_FORMAT") {
			t.Fatalf("expected suffix/content mismatch, got %v", err)
		}
		unsupported := filepath.Join(dir, "tool.bin")
		if err := os.Rename(zipPath, unsupported); err != nil {
			t.Fatalf("rename fixture: %v", err)
		}
		if _, err := ExtractVerified(unsupported, digest, filepath.Join(dir, "other")); err == nil || !strings.Contains(err.Error(), "BOOTSTRAP_ARCHIVE_FORMAT") {
			t.Fatalf("expected unsupported suffix rejection, got %v", err)
		}
	})

	t.Run("legacy malformed incomplete and tampered manifests never match", func(t *testing.T) {
		dir := t.TempDir()
		archivePath, digest := buildZipEntries(t, dir, "tool.zip", []testArchiveEntry{{Name: "bin/tool", Body: "ok", Mode: 0o755}})
		installDir := filepath.Join(dir, "install")
		if err := InstallStaged(archivePath, digest, installDir); err != nil {
			t.Fatalf("install: %v", err)
		}
		manifestPath := filepath.Join(installDir, ManifestName)
		current, err := os.ReadFile(manifestPath)
		if err != nil {
			t.Fatalf("read current manifest: %v", err)
		}
		cases := map[string]string{
			"powershell legacy": fmt.Sprintf(`{"archive_sha256":%q,"file_count":1}`, digest),
			"prior Go shape":    fmt.Sprintf(`{"archive_sha256":%q,"files":[{"path":"bin/tool","sha256":%q}]}`, digest, digestFile(t, filepath.Join(installDir, "bin", "tool"))),
			"null files":        fmt.Sprintf(`{"schema_version":1,"archive_sha256":%q,"files":null}`, digest),
			"empty files":       fmt.Sprintf(`{"schema_version":1,"archive_sha256":%q,"files":[]}`, digest),
			"malformed":         `{`,
			"duplicate paths":   fmt.Sprintf(`{"schema_version":1,"archive_sha256":%q,"files":[{"path":"bin/tool","sha256":%q,"mode":"0755"},{"path":"bin/tool","sha256":%q,"mode":"0755"}]}`, digest, digestFile(t, filepath.Join(installDir, "bin", "tool")), digestFile(t, filepath.Join(installDir, "bin", "tool"))),
			"invalid path":      fmt.Sprintf(`{"schema_version":1,"archive_sha256":%q,"files":[{"path":"../tool","sha256":%q,"mode":"0755"}]}`, digest, digestFile(t, filepath.Join(installDir, "bin", "tool"))),
			"invalid hash":      fmt.Sprintf(`{"schema_version":1,"archive_sha256":%q,"files":[{"path":"bin/tool","sha256":"ABC","mode":"0755"}]}`, digest),
			"invalid mode":      fmt.Sprintf(`{"schema_version":1,"archive_sha256":%q,"files":[{"path":"bin/tool","sha256":%q,"mode":"4755"}]}`, digest, digestFile(t, filepath.Join(installDir, "bin", "tool"))),
		}
		for name, body := range cases {
			t.Run(name, func(t *testing.T) {
				if err := os.WriteFile(manifestPath, []byte(body), 0o644); err != nil {
					t.Fatalf("write manifest: %v", err)
				}
				matches, err := InstalledMatches(installDir, digest)
				if err != nil {
					t.Fatalf("InstalledMatches: %v", err)
				}
				if matches {
					t.Fatal("invalid manifest matched")
				}
			})
		}
		if err := os.WriteFile(manifestPath, current, 0o644); err != nil {
			t.Fatalf("restore current manifest: %v", err)
		}
		if err := os.Mkdir(filepath.Join(installDir, "unexpected"), 0o755); err != nil {
			t.Fatalf("create unexpected directory: %v", err)
		}
		if matches, err := InstalledMatches(installDir, digest); err != nil || matches {
			t.Fatalf("unexpected directory must invalidate manifest: matches=%v err=%v", matches, err)
		}
	})

	t.Run("failed replacement preserves an existing install and successful cutover replaces only it", func(t *testing.T) {
		dir := t.TempDir()
		installDir := filepath.Join(dir, "installs", "tool")
		if err := os.MkdirAll(installDir, 0o755); err != nil {
			t.Fatalf("mkdir old install: %v", err)
		}
		canary := filepath.Join(installDir, "old.txt")
		if err := os.WriteFile(canary, []byte("old"), 0o644); err != nil {
			t.Fatalf("write old install: %v", err)
		}
		sibling := filepath.Join(dir, "installs", "sibling.txt")
		if err := os.WriteFile(sibling, []byte("keep"), 0o644); err != nil {
			t.Fatalf("write sibling: %v", err)
		}
		archivePath, digest := buildZipEntries(t, dir, "replacement.zip", []testArchiveEntry{{Name: "new.txt", Body: "new"}})
		if err := InstallStaged(archivePath, strings.Repeat("00", 32), installDir); err == nil {
			t.Fatal("expected failed replacement")
		}
		if body, _ := os.ReadFile(canary); string(body) != "old" {
			t.Fatalf("failed replacement changed old install: %q", body)
		}
		if err := InstallStaged(archivePath, digest, installDir); err != nil {
			t.Fatalf("successful cutover: %v", err)
		}
		if _, err := os.Stat(canary); !os.IsNotExist(err) {
			t.Fatalf("successful cutover retained old file: %v", err)
		}
		if body, _ := os.ReadFile(sibling); string(body) != "keep" {
			t.Fatalf("cutover changed sibling: %q", body)
		}
	})

	t.Run("the committed config/toolchain.toml pins exactly the official go.dev source", func(t *testing.T) {
		root := repositoryRoot(t)
		policy, err := LoadOfficialSourcePolicy(root)
		if err != nil {
			t.Fatalf("LoadOfficialSourcePolicy(repository root) failed: %v", err)
		}
		if err := policy.Allows("https://go.dev/dl/go1.26.5.windows-amd64.zip"); err != nil {
			t.Fatalf("expected the committed pin to allow the committed Go archive URL, got: %v", err)
		}
		if err := policy.Allows("https://evil.example.com/dl/go1.26.5.windows-amd64.zip"); err == nil {
			t.Fatal("expected the committed policy to reject an unofficial host")
		}
	})

	t.Run("OfficialSourcePolicy accepts only the committed official host/path patterns", func(t *testing.T) {
		root := t.TempDir()
		writeTestToolchainManifest(t, root, map[string]SourcePattern{
			"go": {Host: "go.dev", PathPrefix: "/dl/"},
		})

		policy, err := LoadOfficialSourcePolicy(root)
		if err != nil {
			t.Fatalf("LoadOfficialSourcePolicy failed: %v", err)
		}
		if len(policy.Patterns) != 1 {
			t.Fatalf("expected exactly one committed pattern, got %d", len(policy.Patterns))
		}

		if err := policy.Allows("https://go.dev/dl/go1.26.5.windows-amd64.zip"); err != nil {
			t.Fatalf("expected committed host/path to be allowed, got: %v", err)
		}

		for name, rejected := range map[string]string{
			"different host":       "https://evil.example.com/dl/go1.26.5.windows-amd64.zip",
			"look-alike subdomain": "https://go.dev.evil.example.com/dl/go1.26.5.windows-amd64.zip",
			"different path":       "https://go.dev/other/go1.26.5.windows-amd64.zip",
			"insecure scheme":      "http://go.dev/dl/go1.26.5.windows-amd64.zip",
			"malformed url":        "://not-a-url",
		} {
			if err := policy.Allows(rejected); err == nil {
				t.Fatalf("%s: expected %q to be rejected", name, rejected)
			}
		}
	})

	t.Run("OfficialSourcePolicy allows the GitHub release-asset CDN redirect host for any pinned github.com release", func(t *testing.T) {
		// Regression: config/toolchain.toml's [toolchain.mage] pin is a
		// github.com/.../releases/download/... URL, which GitHub always
		// 302s to a signed release-assets.githubusercontent.com CDN URL.
		// URLSource.Fetch's CheckRedirect re-validates every hop against
		// this same policy, so without this exception a clean bootstrap
		// of the mage toolchain fails closed with
		// BOOTSTRAP_SOURCE_NOT_ALLOWLISTED even though the initial
		// request matched its committed pin exactly (observed live in
		// cross-platform-mage.yml run 30072731806 on ubuntu-latest and
		// windows-latest).
		root := t.TempDir()
		writeTestToolchainManifest(t, root, map[string]SourcePattern{
			"mage": {Host: "github.com", PathPrefix: "/magefile/mage/releases/download/"},
		})
		policy, err := LoadOfficialSourcePolicy(root)
		if err != nil {
			t.Fatalf("LoadOfficialSourcePolicy failed: %v", err)
		}

		signedRedirect := "https://release-assets.githubusercontent.com/github-production-release-asset/104261253/" +
			"02fe83b7-ecdf-4b11-bfbb-6022f5abfb3b?sp=r&sig=example"
		if err := policy.Allows(signedRedirect); err != nil {
			t.Fatalf("expected the GitHub release-asset CDN redirect host to be allowed, got: %v", err)
		}

		for name, rejected := range map[string]string{
			"look-alike CDN subdomain": "https://release-assets.githubusercontent.com.evil.example.com/x",
			"unrelated CDN host":       "https://objects.githubusercontent.com/x",
			"insecure scheme":          "http://release-assets.githubusercontent.com/x",
		} {
			if err := policy.Allows(rejected); err == nil {
				t.Fatalf("%s: expected %q to still be rejected", name, rejected)
			}
		}
	})

	t.Run("OfficialSourcePolicy allows the dl.google.com CDN redirect host and path for any pinned go.dev release", func(t *testing.T) {
		// Regression: the committed [toolchain.go] pin is a
		// go.dev/dl/... URL, which go.dev always 302s to
		// dl.google.com/go/... . Unlike GitHub's signed release-asset
		// CDN, this redirect target has a stable, unsigned path shape,
		// so it is pinned with the same host+path-prefix precision as
		// any TOML-declared pattern rather than trusted for any path
		// (observed live in cross-platform-mage.yml run 30073584282 on
		// all three runners).
		root := t.TempDir()
		writeTestToolchainManifest(t, root, map[string]SourcePattern{
			"go": {Host: "go.dev", PathPrefix: "/dl/"},
		})
		policy, err := LoadOfficialSourcePolicy(root)
		if err != nil {
			t.Fatalf("LoadOfficialSourcePolicy failed: %v", err)
		}

		if err := policy.Allows("https://dl.google.com/go/go1.26.5.linux-amd64.tar.gz"); err != nil {
			t.Fatalf("expected the dl.google.com redirect host/path to be allowed, got: %v", err)
		}

		for name, rejected := range map[string]string{
			"different path on the same CDN host": "https://dl.google.com/chrome/install.exe",
			"look-alike CDN subdomain":             "https://dl.google.com.evil.example.com/go/x",
			"insecure scheme":                      "http://dl.google.com/go/go1.26.5.linux-amd64.tar.gz",
		} {
			if err := policy.Allows(rejected); err == nil {
				t.Fatalf("%s: expected %q to still be rejected", name, rejected)
			}
		}
	})

	t.Run("LoadOfficialSourcePolicy fails closed when no source is pinned", func(t *testing.T) {
		root := t.TempDir()
		writeTestToolchainManifest(t, root, map[string]SourcePattern{
			"go": {},
		})

		if _, err := LoadOfficialSourcePolicy(root); err == nil {
			t.Fatal("expected an empty official-source pin to fail")
		}
	})

	t.Run("VerifySHA256 rejects wrong or malformed hashes", func(t *testing.T) {
		dir := t.TempDir()
		archivePath, digest := buildArchive(t, dir, map[string]string{
			"bin/golc-project.exe": "payload\n",
		})

		if err := VerifySHA256(archivePath, digest); err != nil {
			t.Fatalf("expected matching checksum to verify, got: %v", err)
		}

		wrong := strings.Repeat("ab", 32)
		err := VerifySHA256(archivePath, wrong)
		if err == nil || !strings.Contains(err.Error(), "BOOTSTRAP_CHECKSUM_MISMATCH") {
			t.Fatalf("expected BOOTSTRAP_CHECKSUM_MISMATCH, got: %v", err)
		}

		err = VerifySHA256(archivePath, "NOT-A-DIGEST")
		if err == nil || !strings.Contains(err.Error(), "BOOTSTRAP_CHECKSUM_FORMAT") {
			t.Fatalf("expected BOOTSTRAP_CHECKSUM_FORMAT, got: %v", err)
		}
	})

	t.Run("InspectZipEntries rejects traversal and symlink entries before extraction", func(t *testing.T) {
		dir := t.TempDir()
		traversalPath, traversalDigest := buildArchive(t, dir, map[string]string{
			"bin/../../escape.txt": "escape\n",
		})
		if err := InspectZipEntries(traversalPath); err == nil {
			t.Fatal("expected traversal entry to be rejected")
		} else if !strings.Contains(err.Error(), "BOOTSTRAP_ARCHIVE_TRAVERSAL") {
			t.Fatalf("expected BOOTSTRAP_ARCHIVE_TRAVERSAL, got: %v", err)
		}
		// Checksum still verifies; structure is the failure being tested.
		if err := VerifySHA256(traversalPath, traversalDigest); err != nil {
			t.Fatalf("fixture checksum should verify: %v", err)
		}

		linkPath, _ := buildZipWithSymlinkEntry(t, dir)
		err := InspectZipEntries(linkPath)
		if err == nil || !strings.Contains(err.Error(), "BOOTSTRAP_ARCHIVE_UNSAFE_LINK") {
			t.Fatalf("expected BOOTSTRAP_ARCHIVE_UNSAFE_LINK, got: %v", err)
		}

		cleanDir := filepath.Join(dir, "clean")
		if err := os.MkdirAll(cleanDir, 0o755); err != nil {
			t.Fatalf("mkdir clean: %v", err)
		}
		cleanPath, _ := buildArchive(t, cleanDir, map[string]string{
			"bin/golc-project.exe": "payload\n",
		})
		if err := InspectZipEntries(cleanPath); err != nil {
			t.Fatalf("expected a clean archive to pass inspection, got: %v", err)
		}
	})

	t.Run("ExtractVerified writes only staging and leaves no residue on failure", func(t *testing.T) {
		dir := t.TempDir()
		parent := filepath.Join(dir, "install-parent")
		archivePath, digest := buildArchive(t, dir, map[string]string{
			"bin/golc-project.exe": "payload\n",
			"share/notes.txt":      "notes\n",
		})

		stagingDir, err := ExtractVerified(archivePath, digest, parent)
		if err != nil {
			t.Fatalf("expected extraction to succeed, got: %v", err)
		}
		if filepath.Dir(stagingDir) != parent {
			t.Fatalf("expected staging directory under %q, got %q", parent, stagingDir)
		}
		payload, err := os.ReadFile(filepath.Join(stagingDir, "bin", "golc-project.exe"))
		if err != nil || string(payload) != "payload\n" {
			t.Fatalf("staged payload missing or wrong: err=%v payload=%q", err, payload)
		}

		before, err := os.ReadDir(parent)
		if err != nil {
			t.Fatalf("read parent: %v", err)
		}

		// A checksum mismatch must leave no additional staging residue.
		if _, err := ExtractVerified(archivePath, strings.Repeat("cd", 32), parent); err == nil {
			t.Fatal("expected checksum mismatch to fail extraction")
		}
		// An archive that passes its own checksum but fails entry
		// inspection (a symlink entry) must also leave no additional
		// staging residue.
		linkPath, linkDigest := buildZipWithSymlinkEntry(t, dir)
		if _, err := ExtractVerified(linkPath, linkDigest, parent); err == nil {
			t.Fatal("expected unsafe archive to fail extraction")
		} else if !strings.Contains(err.Error(), "BOOTSTRAP_ARCHIVE_UNSAFE_LINK") {
			t.Fatalf("expected BOOTSTRAP_ARCHIVE_UNSAFE_LINK, got: %v", err)
		}

		after, err := os.ReadDir(parent)
		if err != nil {
			t.Fatalf("read parent: %v", err)
		}
		if len(after) != len(before) {
			t.Fatalf("expected no new staging residue, had %d entries, now %d", len(before), len(after))
		}
	})

	t.Run("PromoteAtomically exposes the complete tree or nothing", func(t *testing.T) {
		dir := t.TempDir()
		parent := filepath.Join(dir, "parent")
		installDir := filepath.Join(dir, "install", "tool")

		firstArchive, firstDigest := buildArchive(t, dir, map[string]string{
			"bin/golc-project.exe": "payload\n",
		})
		firstStaging, err := ExtractVerified(firstArchive, firstDigest, parent)
		if err != nil {
			t.Fatalf("extract first archive: %v", err)
		}
		if err := PromoteAtomically(firstStaging, installDir); err != nil {
			t.Fatalf("expected first promotion to succeed, got: %v", err)
		}
		if _, err := os.Stat(firstStaging); !os.IsNotExist(err) {
			t.Fatalf("staging directory must not survive promotion, stat err: %v", err)
		}
		payload, err := os.ReadFile(filepath.Join(installDir, "bin", "golc-project.exe"))
		if err != nil || string(payload) != "payload\n" {
			t.Fatalf("promoted payload missing or wrong: err=%v payload=%q", err, payload)
		}

		// A corrected retry with different contents must fully replace the
		// prior install, not merge with it: the old file disappears and
		// only the new tree remains.
		secondDir := filepath.Join(dir, "second")
		if err := os.MkdirAll(secondDir, 0o755); err != nil {
			t.Fatalf("mkdir second: %v", err)
		}
		secondArchive, secondDigest := buildArchive(t, secondDir, map[string]string{
			"share/notes.txt": "notes\n",
		})
		secondStaging, err := ExtractVerified(secondArchive, secondDigest, parent)
		if err != nil {
			t.Fatalf("extract second archive: %v", err)
		}
		if err := PromoteAtomically(secondStaging, installDir); err != nil {
			t.Fatalf("expected retry promotion to succeed, got: %v", err)
		}
		if _, err := os.Stat(filepath.Join(installDir, "bin", "golc-project.exe")); !os.IsNotExist(err) {
			t.Fatalf("prior install content must not survive promotion, stat err: %v", err)
		}
		notes, err := os.ReadFile(filepath.Join(installDir, "share", "notes.txt"))
		if err != nil || string(notes) != "notes\n" {
			t.Fatalf("expected the retried tree to be complete: err=%v notes=%q", err, notes)
		}
	})

	t.Run("AcquireStaged validates policy before ever calling the source", func(t *testing.T) {
		root := t.TempDir()
		writeTestToolchainManifest(t, root, map[string]SourcePattern{
			"go": {Host: "go.dev", PathPrefix: "/dl/"},
		})
		policy, err := LoadOfficialSourcePolicy(root)
		if err != nil {
			t.Fatalf("LoadOfficialSourcePolicy failed: %v", err)
		}

		dir := t.TempDir()
		cacheDir := filepath.Join(dir, "cache")
		source := &fakeSource{payload: map[string][]byte{
			"https://evil.example.com/dl/tool.zip": []byte("bytes\n"),
		}}

		if _, err := AcquireStaged(policy, source, "https://evil.example.com/dl/tool.zip", cacheDir); err == nil {
			t.Fatal("expected an unallowlisted source to be rejected")
		}
		if source.calls != 0 {
			t.Fatalf("policy rejection must happen before any fetch, got %d calls", source.calls)
		}
		if _, statErr := os.Stat(cacheDir); !os.IsNotExist(statErr) {
			t.Fatalf("a rejected source must not even create the staging directory, stat err: %v", statErr)
		}

		allowedSource := &fakeSource{payload: map[string][]byte{
			"https://go.dev/dl/tool.zip": []byte("bytes\n"),
		}}
		archivePath, err := AcquireStaged(policy, allowedSource, "https://go.dev/dl/tool.zip", cacheDir)
		if err != nil {
			t.Fatalf("expected an allowlisted source to be staged, got: %v", err)
		}
		if allowedSource.calls != 1 {
			t.Fatalf("expected exactly one fetch call, got %d", allowedSource.calls)
		}
		staged, err := os.ReadFile(archivePath)
		if err != nil || string(staged) != "bytes\n" {
			t.Fatalf("staged bytes missing or wrong: err=%v bytes=%q", err, staged)
		}
	})

	t.Run("AcquireAndPromote rejects unofficial sources and corrupt bytes, then a corrected retry promotes atomically", func(t *testing.T) {
		root := t.TempDir()
		writeTestToolchainManifest(t, root, map[string]SourcePattern{
			"go": {Host: "go.dev", PathPrefix: "/dl/"},
		})
		policy, err := LoadOfficialSourcePolicy(root)
		if err != nil {
			t.Fatalf("LoadOfficialSourcePolicy failed: %v", err)
		}

		dir := t.TempDir()
		fixtureArchive, digest := buildArchive(t, dir, map[string]string{
			"bin/golc-tool.exe": "tool bytes\n",
		})
		payloadBytes, err := os.ReadFile(fixtureArchive)
		if err != nil {
			t.Fatalf("read fixture archive: %v", err)
		}

		cacheDir := filepath.Join(dir, "cache")
		installDir := filepath.Join(dir, "install", "tool")

		// 1. An untrusted host is rejected before any fetch call and before
		// any install is promoted.
		untrusted := &fakeSource{payload: map[string][]byte{
			"https://evil.example.com/dl/tool.zip": payloadBytes,
		}}
		if err := AcquireAndPromote(policy, untrusted, "https://evil.example.com/dl/tool.zip", digest, cacheDir, installDir); err == nil {
			t.Fatal("expected an untrusted source to be rejected")
		}
		if untrusted.calls != 0 {
			t.Fatalf("policy rejection must happen before any fetch, got %d calls", untrusted.calls)
		}
		if _, statErr := os.Stat(installDir); !os.IsNotExist(statErr) {
			t.Fatalf("a rejected source must not promote an install, stat err: %v", statErr)
		}

		// 2. An allowlisted host serving tampered bytes must leave no
		// promoted install.
		tampered := &fakeSource{payload: map[string][]byte{
			"https://go.dev/dl/tool.zip": []byte("tampered bytes that do not match the pin\n"),
		}}
		if err := AcquireAndPromote(policy, tampered, "https://go.dev/dl/tool.zip", digest, cacheDir, installDir); err == nil {
			t.Fatal("expected tampered bytes to fail the checksum")
		}
		if _, statErr := os.Stat(installDir); !os.IsNotExist(statErr) {
			t.Fatalf("a checksum mismatch must leave no install, stat err: %v", statErr)
		}

		// 3. A corrected retry with the exact pinned bytes over the
		// allowlisted source promotes a complete verified tree.
		correct := &fakeSource{payload: map[string][]byte{
			"https://go.dev/dl/tool.zip": payloadBytes,
		}}
		if err := AcquireAndPromote(policy, correct, "https://go.dev/dl/tool.zip", digest, cacheDir, installDir); err != nil {
			t.Fatalf("expected the corrected retry to succeed, got: %v", err)
		}
		installed, err := os.ReadFile(filepath.Join(installDir, "bin", "golc-tool.exe"))
		if err != nil || string(installed) != "tool bytes\n" {
			t.Fatalf("promoted payload missing or wrong: err=%v payload=%q", err, installed)
		}

		// The downloaded archive is removed from cacheDir once promotion
		// completes; only the extraction staging (already renamed away by
		// PromoteAtomically) and the download itself ever touched disk.
		remaining, err := os.ReadDir(cacheDir)
		if err != nil {
			t.Fatalf("read cache dir: %v", err)
		}
		if len(remaining) != 0 {
			t.Fatalf("expected no residual staged downloads in %q, found %v", cacheDir, remaining)
		}
	})
}

// TestScopeBootstrapCache is the exact quick-test marker for scope
// "bootstrap-cache" (test --quick --scope bootstrap-cache). Every subtest
// exercises only in-memory paths and t.TempDir() fixtures — no archive
// download, module fetch, or tool install ever happens here, so the
// registered scope exits 0 offline.
func TestScopeBootstrapCache(t *testing.T) {
	t.Run("NewProjectCacheLayout returns every directory contained inside root", func(t *testing.T) {
		root := t.TempDir()
		layout, err := NewProjectCacheLayout(root)
		if err != nil {
			t.Fatalf("expected a valid root to succeed, got: %v", err)
		}
		absoluteRoot, err := filepath.Abs(root)
		if err != nil {
			t.Fatalf("resolve absolute root: %v", err)
		}
		if layout.Root != absoluteRoot {
			t.Fatalf("expected Root %q, got %q", absoluteRoot, layout.Root)
		}
		for name, path := range map[string]string{
			"Downloads":    layout.Downloads,
			"GoModCache":   layout.GoModCache,
			"GoBuildCache": layout.GoBuildCache,
			"GoBin":        layout.GoBin,
			"NpmCache":     layout.NpmCache,
			"Manifest":     layout.Manifest,
		} {
			if !strings.HasPrefix(path, absoluteRoot+string(os.PathSeparator)) {
				t.Fatalf("%s path %q is not contained inside root %q", name, path, absoluteRoot)
			}
		}
		// Every directory must be distinct — no two cache concerns may
		// silently collide on the same path.
		seen := map[string]string{}
		for name, path := range map[string]string{
			"Downloads": layout.Downloads, "GoModCache": layout.GoModCache,
			"GoBuildCache": layout.GoBuildCache, "GoBin": layout.GoBin, "NpmCache": layout.NpmCache, "Manifest": layout.Manifest,
		} {
			if other, exists := seen[path]; exists {
				t.Fatalf("%s and %s resolve to the same path %q", name, other, path)
			}
			seen[path] = name
		}
	})

	t.Run("NewProjectCacheLayout rejects an empty root", func(t *testing.T) {
		if _, err := NewProjectCacheLayout(""); err == nil {
			t.Fatal("expected an empty root to be rejected")
		} else if !strings.Contains(err.Error(), "BOOTSTRAP_CACHE_ROOT") {
			t.Fatalf("expected BOOTSTRAP_CACHE_ROOT diagnostic, got: %v", err)
		}
	})

	t.Run("Validate rejects a layout whose directory escapes root", func(t *testing.T) {
		root := t.TempDir()
		layout, err := NewProjectCacheLayout(root)
		if err != nil {
			t.Fatalf("construct layout: %v", err)
		}
		layout.GoBin = filepath.Join(filepath.Dir(layout.Root), "escaped-go-bin")

		err = layout.Validate()
		if err == nil {
			t.Fatal("expected an escaping cache directory to be rejected")
		}
		if !strings.Contains(err.Error(), "BOOTSTRAP_CACHE_ESCAPE") {
			t.Fatalf("expected BOOTSTRAP_CACHE_ESCAPE diagnostic, got: %v", err)
		}
	})

	t.Run("Warm creates every cache directory and is a safe idempotent no-op", func(t *testing.T) {
		root := t.TempDir()
		layout, err := NewProjectCacheLayout(root)
		if err != nil {
			t.Fatalf("construct layout: %v", err)
		}

		if err := layout.Warm(); err != nil {
			t.Fatalf("first Warm failed: %v", err)
		}
		for _, dir := range []string{layout.Downloads, layout.GoModCache, layout.GoBuildCache, layout.GoBin, layout.NpmCache, layout.Manifest} {
			info, statErr := os.Stat(dir)
			if statErr != nil {
				t.Fatalf("expected %q to exist after Warm, stat err: %v", dir, statErr)
			}
			if !info.IsDir() {
				t.Fatalf("expected %q to be a directory", dir)
			}
		}

		// A canary file inside a warmed directory must survive a second Warm
		// call: warming is directory provisioning only, never destructive.
		canaryPath := filepath.Join(layout.GoModCache, "canary.txt")
		if err := os.WriteFile(canaryPath, []byte("preserved\n"), 0o644); err != nil {
			t.Fatalf("write canary: %v", err)
		}
		if err := layout.Warm(); err != nil {
			t.Fatalf("second Warm failed: %v", err)
		}
		canary, err := os.ReadFile(canaryPath)
		if err != nil || string(canary) != "preserved\n" {
			t.Fatalf("expected canary to survive idempotent Warm: err=%v content=%q", err, canary)
		}
	})

	t.Run("Environment derives the exact repository-local Go/Node/Wails variables", func(t *testing.T) {
		root := t.TempDir()
		layout, err := NewProjectCacheLayout(root)
		if err != nil {
			t.Fatalf("construct layout: %v", err)
		}

		env := layout.Environment()
		if env.GOTOOLCHAIN != "local" {
			t.Fatalf("expected GOTOOLCHAIN=local, got %q", env.GOTOOLCHAIN)
		}
		if env.GOMODCACHE != layout.GoModCache {
			t.Fatalf("expected GOMODCACHE=%q, got %q", layout.GoModCache, env.GOMODCACHE)
		}
		if env.GOCACHE != layout.GoBuildCache {
			t.Fatalf("expected GOCACHE=%q, got %q", layout.GoBuildCache, env.GOCACHE)
		}
		if env.GOBIN != layout.GoBin {
			t.Fatalf("expected GOBIN=%q, got %q", layout.GoBin, env.GOBIN)
		}
		if env.GOFLAGS != "-mod=readonly" {
			t.Fatalf("expected GOFLAGS=-mod=readonly, got %q", env.GOFLAGS)
		}
		if env.NpmConfigCache != layout.NpmCache {
			t.Fatalf("expected NpmConfigCache=%q, got %q", layout.NpmCache, env.NpmConfigCache)
		}

		asMap := env.AsMap()
		expected := map[string]string{
			"GOTOOLCHAIN":      "local",
			"GOMODCACHE":       layout.GoModCache,
			"GOCACHE":          layout.GoBuildCache,
			"GOBIN":            layout.GoBin,
			"GOFLAGS":          "-mod=readonly",
			"NPM_CONFIG_CACHE": layout.NpmCache,
		}
		if len(asMap) != len(expected) {
			t.Fatalf("expected exactly %d environment entries, got %d: %v", len(expected), len(asMap), asMap)
		}
		for key, value := range expected {
			if asMap[key] != value {
				t.Fatalf("AsMap()[%q] = %q, expected %q", key, asMap[key], value)
			}
		}
	})

	t.Run("WailsBinaryPath and the pinned Wails module/version are exact and stable", func(t *testing.T) {
		if WailsModule != "github.com/wailsapp/wails/v2/cmd/wails" {
			t.Fatalf("unexpected WailsModule pin: %q", WailsModule)
		}
		if WailsVersion != "v2.13.0" {
			t.Fatalf("unexpected WailsVersion pin: %q", WailsVersion)
		}

		root := t.TempDir()
		layout, err := NewProjectCacheLayout(root)
		if err != nil {
			t.Fatalf("construct layout: %v", err)
		}
		if got, want := layout.WailsBinaryPath(".exe"), filepath.Join(layout.GoBin, "wails.exe"); got != want {
			t.Fatalf("expected WailsBinaryPath(.exe) = %q, got %q", want, got)
		}
		if got, want := layout.WailsBinaryPath(""), filepath.Join(layout.GoBin, "wails"); got != want {
			t.Fatalf("expected WailsBinaryPath(\"\") = %q, got %q", want, got)
		}
	})

	t.Run("EnsureDirectories creates missing directories and rejects a path that is already a file", func(t *testing.T) {
		root := t.TempDir()
		nested := filepath.Join(root, "a", "b", "c")

		if err := EnsureDirectories(nested); err != nil {
			t.Fatalf("expected nested directory creation to succeed, got: %v", err)
		}
		info, statErr := os.Stat(nested)
		if statErr != nil || !info.IsDir() {
			t.Fatalf("expected %q to exist as a directory: stat err=%v", nested, statErr)
		}

		// Idempotent: creating the same directory again must not fail.
		if err := EnsureDirectories(nested); err != nil {
			t.Fatalf("expected idempotent re-creation to succeed, got: %v", err)
		}

		blockedPath := filepath.Join(root, "blocked-file")
		if err := os.WriteFile(blockedPath, []byte("not a directory\n"), 0o644); err != nil {
			t.Fatalf("write blocking file: %v", err)
		}
		err := EnsureDirectories(blockedPath)
		if err == nil {
			t.Fatal("expected creating a directory where a file already exists to fail")
		}
		if !strings.Contains(err.Error(), "BOOTSTRAP_CACHE_DIRECTORY") {
			t.Fatalf("expected BOOTSTRAP_CACHE_DIRECTORY diagnostic, got: %v", err)
		}
	})
}
