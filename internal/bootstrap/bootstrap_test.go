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

	"github.com/stretchr/testify/require"
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
	require.NoError(t, err, "create archive")
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
		require.NoError(t, err, "create entry %q", name)
		_, err = entry.Write([]byte(entries[name]))
		require.NoError(t, err, "write entry %q", name)
	}
	require.NoError(t, writer.Close(), "close zip writer")
	require.NoError(t, file.Close(), "close archive file")

	raw, err := os.ReadFile(archivePath)
	require.NoError(t, err, "read archive back")
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
	require.NoError(t, err, "create zip")
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
		require.NoError(t, err, "create zip entry %q", item.Name)
		_, err = entry.Write([]byte(item.Body))
		require.NoError(t, err, "write zip entry %q", item.Name)
	}
	require.NoError(t, writer.Close(), "close zip writer")
	require.NoError(t, file.Close(), "close zip file")
	return archivePath, digestFile(t, archivePath)
}

func buildTarGzEntries(t *testing.T, dir, name string, entries []testArchiveEntry) (string, string) {
	t.Helper()
	archivePath := filepath.Join(dir, name)
	file, err := os.Create(archivePath)
	require.NoError(t, err, "create tar.gz")
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
			// A directory needs its execute bit to be traversable on
			// POSIX at all; unlike archive.go's own reader (which
			// already defaults an unset mode to 0o755 for directories,
			// 0o644 for files), this test writer used to apply 0o644
			// unconditionally, silently producing untraversable
			// directories that only failed on Linux/macOS (observed
			// live: cross-platform-mage.yml run 30075276470 failed
			// extracting a directory this way, invisibly passing on
			// Windows, which does not enforce POSIX directory
			// permission bits the same way).
			if item.Dir {
				mode = 0o755
			} else {
				mode = 0o644
			}
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
		writeErr := writer.WriteHeader(header)
		require.NoError(t, writeErr, "write tar header %q", item.Name)
		if header.Size > 0 {
			_, writeErr := writer.Write([]byte(item.Body))
			require.NoError(t, writeErr, "write tar entry %q", item.Name)
		}
	}
	require.NoError(t, writer.Close(), "close tar writer")
	require.NoError(t, gzipWriter.Close(), "close gzip writer")
	require.NoError(t, file.Close(), "close tar.gz file")
	return archivePath, digestFile(t, archivePath)
}

func digestFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(t, err, "read %s", path)
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

func TestVerifyArchiveAcceptsMatchingChecksum(t *testing.T) {
	dir := t.TempDir()
	archivePath, digest := buildArchive(t, dir, map[string]string{
		"bin/golc-project.exe": "payload\n",
	})

	require.NoError(t, VerifyArchive(archivePath, digest), "expected matching archive to verify")
}

func TestVerifyArchiveRejectsChecksumMismatch(t *testing.T) {
	dir := t.TempDir()
	archivePath, _ := buildArchive(t, dir, map[string]string{
		"bin/golc-project.exe": "payload\n",
	})
	wrong := strings.Repeat("ab", 32)

	err := VerifyArchive(archivePath, wrong)
	require.Error(t, err, "expected checksum mismatch error, got nil")
	require.Contains(t, err.Error(), "BOOTSTRAP_CHECKSUM_MISMATCH", "expected BOOTSTRAP_CHECKSUM_MISMATCH diagnostic")
}

func TestVerifyArchiveRejectsMalformedExpectedChecksum(t *testing.T) {
	dir := t.TempDir()
	archivePath, _ := buildArchive(t, dir, map[string]string{
		"bin/golc-project.exe": "payload\n",
	})

	err := VerifyArchive(archivePath, "NOT-A-DIGEST")
	require.Error(t, err, "expected malformed checksum error, got nil")
	require.Contains(t, err.Error(), "BOOTSTRAP_CHECKSUM_FORMAT", "expected BOOTSTRAP_CHECKSUM_FORMAT diagnostic")
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
		require.NoError(t, os.MkdirAll(caseDir, 0o755), "mkdir")
		archivePath, digest := buildArchive(t, caseDir, map[string]string{
			entry: "escape\n",
		})

		err := VerifyArchive(archivePath, digest)
		require.Error(t, err, "%s: expected traversal rejection for entry %q, got nil", name, entry)
		require.Contains(t, err.Error(), "BOOTSTRAP_ARCHIVE_TRAVERSAL", "%s: expected BOOTSTRAP_ARCHIVE_TRAVERSAL diagnostic", name)
	}
}

func TestInstallStagedPromotesVerifiedArchiveAtomically(t *testing.T) {
	dir := t.TempDir()
	archivePath, digest := buildArchive(t, dir, map[string]string{
		"bin/golc-project.exe": "tool payload\n",
		"share/notes.txt":      "notes\n",
	})
	installDir := filepath.Join(dir, "install", "golc_project")

	require.NoError(t, InstallStaged(archivePath, digest, installDir), "expected staged install to succeed")

	payload, err := os.ReadFile(filepath.Join(installDir, "bin", "golc-project.exe"))
	require.NoError(t, err, "promoted payload missing")
	require.Equal(t, "tool payload\n", string(payload), "promoted payload bytes changed")

	manifestRaw, err := os.ReadFile(filepath.Join(installDir, ManifestName))
	require.NoError(t, err, "install manifest missing")
	var manifest InstallManifest
	require.NoError(t, json.Unmarshal(manifestRaw, &manifest), "install manifest is not valid JSON")
	require.Equal(t, digest, manifest.ArchiveSHA256, "manifest archive hash does not match")
	require.Len(t, manifest.Files, 2, "manifest should record 2 files")

	// No staging directory may survive promotion.
	parentEntries, err := os.ReadDir(filepath.Dir(installDir))
	require.NoError(t, err, "read install parent")
	for _, entry := range parentEntries {
		require.NotContains(t, entry.Name(), "staging", "staging directory %q survived promotion", entry.Name())
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
	require.Error(t, err, "expected checksum mismatch to fail the install, got nil")
	_, statErr := os.Stat(installDir)
	require.True(t, os.IsNotExist(statErr), "checksum mismatch must leave no install, stat err: %v", statErr)
}

func TestInstalledMatchesMakesSecondInstallSkipArchiveSource(t *testing.T) {
	dir := t.TempDir()
	archivePath, digest := buildArchive(t, dir, map[string]string{
		"bin/golc-project.exe": "tool payload\n",
	})
	installDir := filepath.Join(dir, "install", "golc_project")

	require.NoError(t, InstallStaged(archivePath, digest, installDir), "first install failed")

	matches, err := InstalledMatches(installDir, digest)
	require.NoError(t, err, "InstalledMatches failed")
	require.True(t, matches, "matching installed manifest must report true")

	// The archive source is deleted: a matching manifest means the second
	// bootstrap pass never touches the archive source at all.
	require.NoError(t, os.Remove(archivePath), "remove archive source")
	matches, err = InstalledMatches(installDir, digest)
	require.NoError(t, err, "InstalledMatches after source removal failed")
	require.True(t, matches, "installed state must match without consulting the archive source")
}

func TestInstalledMatchesRejectsTamperedInstall(t *testing.T) {
	dir := t.TempDir()
	archivePath, digest := buildArchive(t, dir, map[string]string{
		"bin/golc-project.exe": "tool payload\n",
	})
	installDir := filepath.Join(dir, "install", "golc_project")

	require.NoError(t, InstallStaged(archivePath, digest, installDir), "install failed")
	require.NoError(t, os.WriteFile(filepath.Join(installDir, "bin", "golc-project.exe"), []byte("tampered\n"), 0o644), "tamper install")

	matches, err := InstalledMatches(installDir, digest)
	require.NoError(t, err, "InstalledMatches failed")
	require.False(t, matches, "tampered installed bytes must not match the manifest")

	otherDigest := strings.Repeat("ef", 32)
	matches, err = InstalledMatches(installDir, otherDigest)
	require.NoError(t, err, "InstalledMatches with other digest failed")
	require.False(t, matches, "a different pinned archive hash must not match the manifest")

	matches, err = InstalledMatches(filepath.Join(dir, "never-installed"), digest)
	require.NoError(t, err, "InstalledMatches on missing install failed")
	require.False(t, matches, "a missing install must not match")
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
	require.NoError(t, err, "TOML decode failed")
	undecoded := metadata.Undecoded()
	require.Empty(t, undecoded, "strict decode left undecoded keys: %v", undecoded)
	require.Equal(t, 2, decoded.SchemaVersion, "decoded unexpected values: %+v", decoded)
	require.Equal(t, "info", decoded.Runtime.LogLevel, "decoded unexpected values: %+v", decoded)

	schema := jsonschema.Reflect(&probeConfig{})
	schemaBytes, err := json.Marshal(schema)
	require.NoError(t, err, "schema marshal failed")
	emitted := string(schemaBytes)
	require.Contains(t, emitted, "https://json-schema.org/draft/2020-12/schema", "schema bytes missing draft 2020-12 marker: %s", emitted)
	for _, fragment := range []string{"schema_version", "log_level", "additionalProperties"} {
		require.Contains(t, emitted, fragment, "schema bytes missing %q: %s", fragment, emitted)
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
	require.NoError(t, os.MkdirAll(configDir, 0o755), "mkdir config")
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "toolchain.toml"), []byte(body.String()), 0o644), "write toolchain.toml")
}

// buildZipWithSymlinkEntry writes a zip archive containing one file entry
// plus one symlink entry (encoded the same way archive/zip's
// FileHeader.SetMode round-trips a Unix symlink mode), returning the
// archive path and its lowercase hex SHA-256.
func buildZipWithSymlinkEntry(t *testing.T, dir string) (string, string) {
	t.Helper()

	archivePath := filepath.Join(dir, "symlink-archive.zip")
	file, err := os.Create(archivePath)
	require.NoError(t, err, "create archive")
	writer := zip.NewWriter(file)

	regular, err := writer.Create("bin/golc-project.exe")
	require.NoError(t, err, "create regular entry")
	_, err = regular.Write([]byte("payload\n"))
	require.NoError(t, err, "write regular entry")

	header := &zip.FileHeader{Name: "bin/evil-link", Method: zip.Deflate}
	header.SetMode(os.ModeSymlink | 0o777)
	linkWriter, err := writer.CreateHeader(header)
	require.NoError(t, err, "create symlink header")
	_, err = linkWriter.Write([]byte("../../../etc/passwd"))
	require.NoError(t, err, "write symlink entry")

	require.NoError(t, writer.Close(), "close zip writer")
	require.NoError(t, file.Close(), "close archive file")

	raw, err := os.ReadFile(archivePath)
	require.NoError(t, err, "read archive back")
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
	require.NoError(t, err, "resolve repository root")
	_, err = os.Stat(filepath.Join(root, "golc.project.toml"))
	require.NoError(t, err, "repository root %q has no golc.project.toml", root)
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
				require.NoError(t, InstallStaged(archivePath, digest, installDir), "InstallStaged(%s)", format)
				var manifest InstallManifest
				raw, err := os.ReadFile(filepath.Join(installDir, ManifestName))
				require.NoError(t, err, "read manifest")
				require.NoError(t, json.Unmarshal(raw, &manifest), "decode manifest")
				require.Equal(t, InstallManifestSchemaVersion, manifest.SchemaVersion, "schema_version mismatch")
				require.Len(t, manifest.Files, 2, "manifest files")
				require.Equal(t, "tool/bin/run", manifest.Files[0].Path, "unexpected executable manifest entry: %+v", manifest.Files[0])
				require.Equal(t, "0755", manifest.Files[0].Mode, "unexpected executable manifest entry: %+v", manifest.Files[0])
				matches, err := InstalledMatches(installDir, digest)
				require.NoError(t, err, "InstalledMatches")
				require.True(t, matches, "InstalledMatches")
				if runtime.GOOS != "windows" {
					info, err := os.Stat(filepath.Join(installDir, "tool", "bin", "run"))
					require.NoError(t, err, "stat executable")
					require.Equal(t, os.FileMode(0o755), info.Mode().Perm(), "executable mode mismatch")
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
					_, err := ExtractVerified(archivePath, digest, parent)
					require.Error(t, err, "expected unsafe path rejection")
					_, statErr := os.Stat(parent)
					require.True(t, os.IsNotExist(statErr), "inspection failure created extraction parent: %v", statErr)
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
				_, err := ExtractVerified(archivePath, digest, parent)
				require.Error(t, err, "%s expected duplicate rejection", format)
				require.Contains(t, err.Error(), "BOOTSTRAP_ARCHIVE_DUPLICATE", "%s expected duplicate rejection, got %v", format, err)
				_, statErr := os.Stat(parent)
				require.True(t, os.IsNotExist(statErr), "%s duplicate created extraction parent: %v", format, statErr)
			}
		})
	})

	t.Run("tar.gz rejects hardlinks and special or unknown entry types", func(t *testing.T) {
		// Symlinks are not in this list: official Node.js Linux/macOS
		// release archives ship bin/npm, bin/npx, and bin/corepack as
		// real symlinks (observed live: cross-platform-mage.yml run
		// 30074378227 failed installing node-v24.18.0-linux-x64 with
		// BOOTSTRAP_ARCHIVE_UNSAFE_TYPE before symlink support existed).
		// A contained symlink is exercised separately below; only a
		// symlink whose target escapes the archive root is still
		// rejected (also covered separately, with
		// BOOTSTRAP_ARCHIVE_TRAVERSAL rather than
		// BOOTSTRAP_ARCHIVE_UNSAFE_TYPE).
		cases := []struct {
			name string
			kind byte
		}{
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
				_, err := ExtractVerified(archivePath, digest, parent)
				require.Error(t, err, "expected unsafe type rejection")
				require.True(t,
					strings.Contains(err.Error(), "BOOTSTRAP_ARCHIVE_UNSAFE_TYPE") || strings.Contains(err.Error(), "BOOTSTRAP_ARCHIVE_FORMAT"),
					"expected unsafe type rejection, got %v", err)
				_, statErr := os.Stat(parent)
				require.True(t, os.IsNotExist(statErr), "unsafe tar created extraction parent: %v", statErr)
			})
		}
	})

	t.Run("tar.gz symlinks are extracted when contained and rejected when their target escapes the archive root", func(t *testing.T) {
		t.Run("a contained symlink extracts, hashes into the manifest, and is verified on a second install", func(t *testing.T) {
			dir := t.TempDir()
			archivePath, digest := buildTarGzEntries(t, dir, "node.tar.gz", []testArchiveEntry{
				{Name: "node-v24.18.0-linux-x64", Dir: true},
				{Name: "node-v24.18.0-linux-x64/lib", Dir: true},
				{Name: "node-v24.18.0-linux-x64/lib/node_modules", Dir: true},
				{Name: "node-v24.18.0-linux-x64/lib/node_modules/npm", Dir: true},
				{Name: "node-v24.18.0-linux-x64/lib/node_modules/npm/bin", Dir: true},
				{Name: "node-v24.18.0-linux-x64/lib/node_modules/npm/bin/npm-cli.js", Body: "#!/usr/bin/env node\n"},
				{Name: "node-v24.18.0-linux-x64/bin", Dir: true},
				{
					Name:     "node-v24.18.0-linux-x64/bin/npm",
					Typeflag: tar.TypeSymlink,
					Linkname: "../lib/node_modules/npm/bin/npm-cli.js",
				},
			})
			installDir := filepath.Join(dir, "install")
			require.NoError(t, InstallStaged(archivePath, digest, installDir), "InstallStaged with a contained symlink")

			symlinkPath := filepath.Join(installDir, "node-v24.18.0-linux-x64", "bin", "npm")
			info, err := os.Lstat(symlinkPath)
			require.NoError(t, err, "lstat extracted symlink")
			require.True(t, info.Mode()&os.ModeSymlink != 0, "expected %s to be a symlink, got mode %v", symlinkPath, info.Mode())
			target, err := os.Readlink(symlinkPath)
			require.NoError(t, err, "readlink")
			require.Equal(t, "../lib/node_modules/npm/bin/npm-cli.js", filepath.ToSlash(target), "symlink target should be the archive-relative link")

			manifestBytes, err := os.ReadFile(filepath.Join(installDir, ManifestName))
			require.NoError(t, err, "read manifest")
			require.Contains(t, string(manifestBytes), "../lib/node_modules/npm/bin/npm-cli.js", "manifest does not record the symlink target: %s", manifestBytes)

			matches, err := InstalledMatches(installDir, digest)
			require.NoError(t, err, "InstalledMatches")
			require.True(t, matches, "expected a second install of the identical archive to match without re-extracting")

			require.NoError(t, os.Remove(symlinkPath), "remove symlink for tamper test")
			require.NoError(t, os.WriteFile(symlinkPath, []byte("not a symlink anymore"), 0o644), "replace symlink with a regular file")
			tampered, err := InstalledMatches(installDir, digest)
			require.NoError(t, err, "InstalledMatches after tampering")
			require.False(t, tampered, "expected a symlink replaced by a regular file to fail InstalledMatches")
		})

		t.Run("a symlink whose target escapes the archive root is rejected before extraction", func(t *testing.T) {
			dir := t.TempDir()
			archivePath, digest := buildTarGzEntries(t, dir, "unsafe-symlink.tar.gz", []testArchiveEntry{{
				Name: "bin/npm", Typeflag: tar.TypeSymlink, Linkname: "../../outside",
			}})
			parent := filepath.Join(dir, "parent")
			_, err := ExtractVerified(archivePath, digest, parent)
			require.Error(t, err, "expected a traversal rejection")
			require.Contains(t, err.Error(), "BOOTSTRAP_ARCHIVE_TRAVERSAL", "expected a traversal rejection, got %v", err)
			_, statErr := os.Stat(parent)
			require.True(t, os.IsNotExist(statErr), "unsafe symlink created extraction parent: %v", statErr)
		})

		t.Run("an absolute symlink target is rejected before extraction", func(t *testing.T) {
			dir := t.TempDir()
			absoluteTarget := "/etc/passwd"
			if runtime.GOOS == "windows" {
				absoluteTarget = `C:\Windows\System32\config`
			}
			archivePath, digest := buildTarGzEntries(t, dir, "absolute-symlink.tar.gz", []testArchiveEntry{{
				Name: "bin/npm", Typeflag: tar.TypeSymlink, Linkname: absoluteTarget,
			}})
			parent := filepath.Join(dir, "parent")
			_, err := ExtractVerified(archivePath, digest, parent)
			require.Error(t, err, "expected a traversal rejection")
			require.Contains(t, err.Error(), "BOOTSTRAP_ARCHIVE_TRAVERSAL", "expected a traversal rejection, got %v", err)
			_, statErr := os.Stat(parent)
			require.True(t, os.IsNotExist(statErr), "absolute symlink created extraction parent: %v", statErr)
		})
	})

	t.Run("archive suffix and content must agree", func(t *testing.T) {
		dir := t.TempDir()
		zipPath, digest := buildZipEntries(t, dir, "tool.tar.gz", []testArchiveEntry{{Name: "tool", Body: "zip"}})
		_, err := ExtractVerified(zipPath, digest, filepath.Join(dir, "parent"))
		require.Error(t, err, "expected suffix/content mismatch")
		require.Contains(t, err.Error(), "BOOTSTRAP_ARCHIVE_FORMAT", "expected suffix/content mismatch, got %v", err)
		unsupported := filepath.Join(dir, "tool.bin")
		require.NoError(t, os.Rename(zipPath, unsupported), "rename fixture")
		_, err = ExtractVerified(unsupported, digest, filepath.Join(dir, "other"))
		require.Error(t, err, "expected unsupported suffix rejection")
		require.Contains(t, err.Error(), "BOOTSTRAP_ARCHIVE_FORMAT", "expected unsupported suffix rejection, got %v", err)
	})

	t.Run("legacy malformed incomplete and tampered manifests never match", func(t *testing.T) {
		dir := t.TempDir()
		archivePath, digest := buildZipEntries(t, dir, "tool.zip", []testArchiveEntry{{Name: "bin/tool", Body: "ok", Mode: 0o755}})
		installDir := filepath.Join(dir, "install")
		require.NoError(t, InstallStaged(archivePath, digest, installDir), "install")
		manifestPath := filepath.Join(installDir, ManifestName)
		current, err := os.ReadFile(manifestPath)
		require.NoError(t, err, "read current manifest")
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
				require.NoError(t, os.WriteFile(manifestPath, []byte(body), 0o644), "write manifest")
				matches, err := InstalledMatches(installDir, digest)
				require.NoError(t, err, "InstalledMatches")
				require.False(t, matches, "invalid manifest matched")
			})
		}
		require.NoError(t, os.WriteFile(manifestPath, current, 0o644), "restore current manifest")
		require.NoError(t, os.Mkdir(filepath.Join(installDir, "unexpected"), 0o755), "create unexpected directory")
		matches, err := InstalledMatches(installDir, digest)
		require.NoError(t, err, "unexpected directory must invalidate manifest")
		require.False(t, matches, "unexpected directory must invalidate manifest")
	})

	t.Run("failed replacement preserves an existing install and successful cutover replaces only it", func(t *testing.T) {
		dir := t.TempDir()
		installDir := filepath.Join(dir, "installs", "tool")
		require.NoError(t, os.MkdirAll(installDir, 0o755), "mkdir old install")
		canary := filepath.Join(installDir, "old.txt")
		require.NoError(t, os.WriteFile(canary, []byte("old"), 0o644), "write old install")
		sibling := filepath.Join(dir, "installs", "sibling.txt")
		require.NoError(t, os.WriteFile(sibling, []byte("keep"), 0o644), "write sibling")
		archivePath, digest := buildZipEntries(t, dir, "replacement.zip", []testArchiveEntry{{Name: "new.txt", Body: "new"}})
		require.Error(t, InstallStaged(archivePath, strings.Repeat("00", 32), installDir), "expected failed replacement")
		body, _ := os.ReadFile(canary)
		require.Equal(t, "old", string(body), "failed replacement changed old install")
		require.NoError(t, InstallStaged(archivePath, digest, installDir), "successful cutover")
		_, statErr := os.Stat(canary)
		require.True(t, os.IsNotExist(statErr), "successful cutover retained old file: %v", statErr)
		body, _ = os.ReadFile(sibling)
		require.Equal(t, "keep", string(body), "cutover changed sibling")
	})

	t.Run("the committed config/toolchain.toml pins exactly the official go.dev source", func(t *testing.T) {
		root := repositoryRoot(t)
		policy, err := LoadOfficialSourcePolicy(root)
		require.NoError(t, err, "LoadOfficialSourcePolicy(repository root) failed")
		require.NoError(t, policy.Allows("https://go.dev/dl/go1.26.5.windows-amd64.zip"), "expected the committed pin to allow the committed Go archive URL")
		require.Error(t, policy.Allows("https://evil.example.com/dl/go1.26.5.windows-amd64.zip"), "expected the committed policy to reject an unofficial host")
	})

	t.Run("OfficialSourcePolicy accepts only the committed official host/path patterns", func(t *testing.T) {
		root := t.TempDir()
		writeTestToolchainManifest(t, root, map[string]SourcePattern{
			"go": {Host: "go.dev", PathPrefix: "/dl/"},
		})

		policy, err := LoadOfficialSourcePolicy(root)
		require.NoError(t, err, "LoadOfficialSourcePolicy failed")
		require.Len(t, policy.Patterns, 1, "expected exactly one committed pattern")

		require.NoError(t, policy.Allows("https://go.dev/dl/go1.26.5.windows-amd64.zip"), "expected committed host/path to be allowed")

		for name, rejected := range map[string]string{
			"different host":       "https://evil.example.com/dl/go1.26.5.windows-amd64.zip",
			"look-alike subdomain": "https://go.dev.evil.example.com/dl/go1.26.5.windows-amd64.zip",
			"different path":       "https://go.dev/other/go1.26.5.windows-amd64.zip",
			"insecure scheme":      "http://go.dev/dl/go1.26.5.windows-amd64.zip",
			"malformed url":        "://not-a-url",
		} {
			require.Error(t, policy.Allows(rejected), "%s: expected %q to be rejected", name, rejected)
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
		require.NoError(t, err, "LoadOfficialSourcePolicy failed")

		signedRedirect := "https://release-assets.githubusercontent.com/github-production-release-asset/104261253/" +
			"02fe83b7-ecdf-4b11-bfbb-6022f5abfb3b?sp=r&sig=example"
		require.NoError(t, policy.Allows(signedRedirect), "expected the GitHub release-asset CDN redirect host to be allowed")

		for name, rejected := range map[string]string{
			"look-alike CDN subdomain": "https://release-assets.githubusercontent.com.evil.example.com/x",
			"unrelated CDN host":       "https://objects.githubusercontent.com/x",
			"insecure scheme":          "http://release-assets.githubusercontent.com/x",
		} {
			require.Error(t, policy.Allows(rejected), "%s: expected %q to still be rejected", name, rejected)
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
		require.NoError(t, err, "LoadOfficialSourcePolicy failed")

		require.NoError(t, policy.Allows("https://dl.google.com/go/go1.26.5.linux-amd64.tar.gz"), "expected the dl.google.com redirect host/path to be allowed")

		for name, rejected := range map[string]string{
			"different path on the same CDN host": "https://dl.google.com/chrome/install.exe",
			"look-alike CDN subdomain":            "https://dl.google.com.evil.example.com/go/x",
			"insecure scheme":                     "http://dl.google.com/go/go1.26.5.linux-amd64.tar.gz",
		} {
			require.Error(t, policy.Allows(rejected), "%s: expected %q to still be rejected", name, rejected)
		}
	})

	t.Run("LoadOfficialSourcePolicy fails closed when no source is pinned", func(t *testing.T) {
		root := t.TempDir()
		writeTestToolchainManifest(t, root, map[string]SourcePattern{
			"go": {},
		})

		_, err := LoadOfficialSourcePolicy(root)
		require.Error(t, err, "expected an empty official-source pin to fail")
	})

	t.Run("VerifySHA256 rejects wrong or malformed hashes", func(t *testing.T) {
		dir := t.TempDir()
		archivePath, digest := buildArchive(t, dir, map[string]string{
			"bin/golc-project.exe": "payload\n",
		})

		require.NoError(t, VerifySHA256(archivePath, digest), "expected matching checksum to verify")

		wrong := strings.Repeat("ab", 32)
		err := VerifySHA256(archivePath, wrong)
		require.Error(t, err, "expected BOOTSTRAP_CHECKSUM_MISMATCH")
		require.Contains(t, err.Error(), "BOOTSTRAP_CHECKSUM_MISMATCH", "expected BOOTSTRAP_CHECKSUM_MISMATCH, got: %v", err)

		err = VerifySHA256(archivePath, "NOT-A-DIGEST")
		require.Error(t, err, "expected BOOTSTRAP_CHECKSUM_FORMAT")
		require.Contains(t, err.Error(), "BOOTSTRAP_CHECKSUM_FORMAT", "expected BOOTSTRAP_CHECKSUM_FORMAT, got: %v", err)
	})

	t.Run("InspectZipEntries rejects traversal and symlink entries before extraction", func(t *testing.T) {
		dir := t.TempDir()
		traversalPath, traversalDigest := buildArchive(t, dir, map[string]string{
			"bin/../../escape.txt": "escape\n",
		})
		err := InspectZipEntries(traversalPath)
		require.Error(t, err, "expected traversal entry to be rejected")
		require.Contains(t, err.Error(), "BOOTSTRAP_ARCHIVE_TRAVERSAL", "expected BOOTSTRAP_ARCHIVE_TRAVERSAL, got: %v", err)
		// Checksum still verifies; structure is the failure being tested.
		require.NoError(t, VerifySHA256(traversalPath, traversalDigest), "fixture checksum should verify")

		linkPath, _ := buildZipWithSymlinkEntry(t, dir)
		err = InspectZipEntries(linkPath)
		require.Error(t, err, "expected BOOTSTRAP_ARCHIVE_UNSAFE_LINK")
		require.Contains(t, err.Error(), "BOOTSTRAP_ARCHIVE_UNSAFE_LINK", "expected BOOTSTRAP_ARCHIVE_UNSAFE_LINK, got: %v", err)

		cleanDir := filepath.Join(dir, "clean")
		require.NoError(t, os.MkdirAll(cleanDir, 0o755), "mkdir clean")
		cleanPath, _ := buildArchive(t, cleanDir, map[string]string{
			"bin/golc-project.exe": "payload\n",
		})
		require.NoError(t, InspectZipEntries(cleanPath), "expected a clean archive to pass inspection")
	})

	t.Run("ExtractVerified writes only staging and leaves no residue on failure", func(t *testing.T) {
		dir := t.TempDir()
		parent := filepath.Join(dir, "install-parent")
		archivePath, digest := buildArchive(t, dir, map[string]string{
			"bin/golc-project.exe": "payload\n",
			"share/notes.txt":      "notes\n",
		})

		stagingDir, err := ExtractVerified(archivePath, digest, parent)
		require.NoError(t, err, "expected extraction to succeed")
		require.Equal(t, parent, filepath.Dir(stagingDir), "expected staging directory under %q, got %q", parent, stagingDir)
		payload, err := os.ReadFile(filepath.Join(stagingDir, "bin", "golc-project.exe"))
		require.NoError(t, err, "staged payload missing")
		require.Equal(t, "payload\n", string(payload), "staged payload wrong")

		before, err := os.ReadDir(parent)
		require.NoError(t, err, "read parent")

		// A checksum mismatch must leave no additional staging residue.
		_, err = ExtractVerified(archivePath, strings.Repeat("cd", 32), parent)
		require.Error(t, err, "expected checksum mismatch to fail extraction")
		// An archive that passes its own checksum but fails entry
		// inspection (a symlink entry) must also leave no additional
		// staging residue.
		linkPath, linkDigest := buildZipWithSymlinkEntry(t, dir)
		_, err = ExtractVerified(linkPath, linkDigest, parent)
		require.Error(t, err, "expected unsafe archive to fail extraction")
		require.Contains(t, err.Error(), "BOOTSTRAP_ARCHIVE_UNSAFE_LINK", "expected BOOTSTRAP_ARCHIVE_UNSAFE_LINK, got: %v", err)

		after, err := os.ReadDir(parent)
		require.NoError(t, err, "read parent")
		require.Len(t, after, len(before), "expected no new staging residue")
	})

	t.Run("PromoteAtomically exposes the complete tree or nothing", func(t *testing.T) {
		dir := t.TempDir()
		parent := filepath.Join(dir, "parent")
		installDir := filepath.Join(dir, "install", "tool")

		firstArchive, firstDigest := buildArchive(t, dir, map[string]string{
			"bin/golc-project.exe": "payload\n",
		})
		firstStaging, err := ExtractVerified(firstArchive, firstDigest, parent)
		require.NoError(t, err, "extract first archive")
		require.NoError(t, PromoteAtomically(firstStaging, installDir), "expected first promotion to succeed")
		_, statErr := os.Stat(firstStaging)
		require.True(t, os.IsNotExist(statErr), "staging directory must not survive promotion, stat err: %v", statErr)
		payload, err := os.ReadFile(filepath.Join(installDir, "bin", "golc-project.exe"))
		require.NoError(t, err, "promoted payload missing")
		require.Equal(t, "payload\n", string(payload), "promoted payload wrong")

		// A corrected retry with different contents must fully replace the
		// prior install, not merge with it: the old file disappears and
		// only the new tree remains.
		secondDir := filepath.Join(dir, "second")
		require.NoError(t, os.MkdirAll(secondDir, 0o755), "mkdir second")
		secondArchive, secondDigest := buildArchive(t, secondDir, map[string]string{
			"share/notes.txt": "notes\n",
		})
		secondStaging, err := ExtractVerified(secondArchive, secondDigest, parent)
		require.NoError(t, err, "extract second archive")
		require.NoError(t, PromoteAtomically(secondStaging, installDir), "expected retry promotion to succeed")
		_, statErr = os.Stat(filepath.Join(installDir, "bin", "golc-project.exe"))
		require.True(t, os.IsNotExist(statErr), "prior install content must not survive promotion, stat err: %v", statErr)
		notes, err := os.ReadFile(filepath.Join(installDir, "share", "notes.txt"))
		require.NoError(t, err, "expected the retried tree to be complete")
		require.Equal(t, "notes\n", string(notes), "expected the retried tree to be complete")
	})

	t.Run("AcquireStaged validates policy before ever calling the source", func(t *testing.T) {
		root := t.TempDir()
		writeTestToolchainManifest(t, root, map[string]SourcePattern{
			"go": {Host: "go.dev", PathPrefix: "/dl/"},
		})
		policy, err := LoadOfficialSourcePolicy(root)
		require.NoError(t, err, "LoadOfficialSourcePolicy failed")

		dir := t.TempDir()
		cacheDir := filepath.Join(dir, "cache")
		source := &fakeSource{payload: map[string][]byte{
			"https://evil.example.com/dl/tool.zip": []byte("bytes\n"),
		}}

		_, err = AcquireStaged(policy, source, "https://evil.example.com/dl/tool.zip", cacheDir)
		require.Error(t, err, "expected an unallowlisted source to be rejected")
		require.Equal(t, 0, source.calls, "policy rejection must happen before any fetch")
		_, statErr := os.Stat(cacheDir)
		require.True(t, os.IsNotExist(statErr), "a rejected source must not even create the staging directory, stat err: %v", statErr)

		allowedSource := &fakeSource{payload: map[string][]byte{
			"https://go.dev/dl/tool.zip": []byte("bytes\n"),
		}}
		archivePath, err := AcquireStaged(policy, allowedSource, "https://go.dev/dl/tool.zip", cacheDir)
		require.NoError(t, err, "expected an allowlisted source to be staged")
		require.Equal(t, 1, allowedSource.calls, "expected exactly one fetch call")
		staged, err := os.ReadFile(archivePath)
		require.NoError(t, err, "staged bytes missing")
		require.Equal(t, "bytes\n", string(staged), "staged bytes wrong")
	})

	t.Run("AcquireAndPromote rejects unofficial sources and corrupt bytes, then a corrected retry promotes atomically", func(t *testing.T) {
		root := t.TempDir()
		writeTestToolchainManifest(t, root, map[string]SourcePattern{
			"go": {Host: "go.dev", PathPrefix: "/dl/"},
		})
		policy, err := LoadOfficialSourcePolicy(root)
		require.NoError(t, err, "LoadOfficialSourcePolicy failed")

		dir := t.TempDir()
		fixtureArchive, digest := buildArchive(t, dir, map[string]string{
			"bin/golc-tool.exe": "tool bytes\n",
		})
		payloadBytes, err := os.ReadFile(fixtureArchive)
		require.NoError(t, err, "read fixture archive")

		cacheDir := filepath.Join(dir, "cache")
		installDir := filepath.Join(dir, "install", "tool")

		// 1. An untrusted host is rejected before any fetch call and before
		// any install is promoted.
		untrusted := &fakeSource{payload: map[string][]byte{
			"https://evil.example.com/dl/tool.zip": payloadBytes,
		}}
		err = AcquireAndPromote(policy, untrusted, "https://evil.example.com/dl/tool.zip", digest, cacheDir, installDir)
		require.Error(t, err, "expected an untrusted source to be rejected")
		require.Equal(t, 0, untrusted.calls, "policy rejection must happen before any fetch")
		_, statErr := os.Stat(installDir)
		require.True(t, os.IsNotExist(statErr), "a rejected source must not promote an install, stat err: %v", statErr)

		// 2. An allowlisted host serving tampered bytes must leave no
		// promoted install.
		tampered := &fakeSource{payload: map[string][]byte{
			"https://go.dev/dl/tool.zip": []byte("tampered bytes that do not match the pin\n"),
		}}
		err = AcquireAndPromote(policy, tampered, "https://go.dev/dl/tool.zip", digest, cacheDir, installDir)
		require.Error(t, err, "expected tampered bytes to fail the checksum")
		_, statErr = os.Stat(installDir)
		require.True(t, os.IsNotExist(statErr), "a checksum mismatch must leave no install, stat err: %v", statErr)

		// 3. A corrected retry with the exact pinned bytes over the
		// allowlisted source promotes a complete verified tree.
		correct := &fakeSource{payload: map[string][]byte{
			"https://go.dev/dl/tool.zip": payloadBytes,
		}}
		require.NoError(t, AcquireAndPromote(policy, correct, "https://go.dev/dl/tool.zip", digest, cacheDir, installDir), "expected the corrected retry to succeed")
		installed, err := os.ReadFile(filepath.Join(installDir, "bin", "golc-tool.exe"))
		require.NoError(t, err, "promoted payload missing")
		require.Equal(t, "tool bytes\n", string(installed), "promoted payload wrong")

		// The downloaded archive is removed from cacheDir once promotion
		// completes; only the extraction staging (already renamed away by
		// PromoteAtomically) and the download itself ever touched disk.
		remaining, err := os.ReadDir(cacheDir)
		require.NoError(t, err, "read cache dir")
		require.Empty(t, remaining, "expected no residual staged downloads in %q, found %v", cacheDir, remaining)
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
		require.NoError(t, err, "expected a valid root to succeed")
		absoluteRoot, err := filepath.Abs(root)
		require.NoError(t, err, "resolve absolute root")
		require.Equal(t, absoluteRoot, layout.Root, "expected matching Root")
		for name, path := range map[string]string{
			"Downloads":    layout.Downloads,
			"GoModCache":   layout.GoModCache,
			"GoBuildCache": layout.GoBuildCache,
			"GoBin":        layout.GoBin,
			"NpmCache":     layout.NpmCache,
			"Manifest":     layout.Manifest,
		} {
			require.True(t, strings.HasPrefix(path, absoluteRoot+string(os.PathSeparator)), "%s path %q is not contained inside root %q", name, path, absoluteRoot)
		}
		// Every directory must be distinct — no two cache concerns may
		// silently collide on the same path.
		seen := map[string]string{}
		for name, path := range map[string]string{
			"Downloads": layout.Downloads, "GoModCache": layout.GoModCache,
			"GoBuildCache": layout.GoBuildCache, "GoBin": layout.GoBin, "NpmCache": layout.NpmCache, "Manifest": layout.Manifest,
		} {
			other, exists := seen[path]
			require.False(t, exists, "%s and %s resolve to the same path %q", name, other, path)
			seen[path] = name
		}
	})

	t.Run("NewProjectCacheLayout rejects an empty root", func(t *testing.T) {
		_, err := NewProjectCacheLayout("")
		require.Error(t, err, "expected an empty root to be rejected")
		require.Contains(t, err.Error(), "BOOTSTRAP_CACHE_ROOT", "expected BOOTSTRAP_CACHE_ROOT diagnostic, got: %v", err)
	})

	t.Run("Validate rejects a layout whose directory escapes root", func(t *testing.T) {
		root := t.TempDir()
		layout, err := NewProjectCacheLayout(root)
		require.NoError(t, err, "construct layout")
		layout.GoBin = filepath.Join(filepath.Dir(layout.Root), "escaped-go-bin")

		err = layout.Validate()
		require.Error(t, err, "expected an escaping cache directory to be rejected")
		require.Contains(t, err.Error(), "BOOTSTRAP_CACHE_ESCAPE", "expected BOOTSTRAP_CACHE_ESCAPE diagnostic, got: %v", err)
	})

	t.Run("Warm creates every cache directory and is a safe idempotent no-op", func(t *testing.T) {
		root := t.TempDir()
		layout, err := NewProjectCacheLayout(root)
		require.NoError(t, err, "construct layout")

		require.NoError(t, layout.Warm(), "first Warm failed")
		for _, dir := range []string{layout.Downloads, layout.GoModCache, layout.GoBuildCache, layout.GoBin, layout.NpmCache, layout.Manifest} {
			info, statErr := os.Stat(dir)
			require.NoError(t, statErr, "expected %q to exist after Warm", dir)
			require.True(t, info.IsDir(), "expected %q to be a directory", dir)
		}

		// A canary file inside a warmed directory must survive a second Warm
		// call: warming is directory provisioning only, never destructive.
		canaryPath := filepath.Join(layout.GoModCache, "canary.txt")
		require.NoError(t, os.WriteFile(canaryPath, []byte("preserved\n"), 0o644), "write canary")
		require.NoError(t, layout.Warm(), "second Warm failed")
		canary, err := os.ReadFile(canaryPath)
		require.NoError(t, err, "expected canary to survive idempotent Warm")
		require.Equal(t, "preserved\n", string(canary), "expected canary to survive idempotent Warm")
	})

	t.Run("Environment derives the exact repository-local Go/Node/Wails variables", func(t *testing.T) {
		root := t.TempDir()
		layout, err := NewProjectCacheLayout(root)
		require.NoError(t, err, "construct layout")

		env := layout.Environment()
		require.Equal(t, "local", env.GOTOOLCHAIN, "expected GOTOOLCHAIN=local")
		require.Equal(t, layout.GoModCache, env.GOMODCACHE, "expected matching GOMODCACHE")
		require.Equal(t, layout.GoBuildCache, env.GOCACHE, "expected matching GOCACHE")
		require.Equal(t, layout.GoBin, env.GOBIN, "expected matching GOBIN")
		require.Equal(t, "-mod=readonly", env.GOFLAGS, "expected GOFLAGS=-mod=readonly")
		require.Equal(t, layout.NpmCache, env.NpmConfigCache, "expected matching NpmConfigCache")

		asMap := env.AsMap()
		expected := map[string]string{
			"GOTOOLCHAIN":      "local",
			"GOMODCACHE":       layout.GoModCache,
			"GOCACHE":          layout.GoBuildCache,
			"GOBIN":            layout.GoBin,
			"GOFLAGS":          "-mod=readonly",
			"NPM_CONFIG_CACHE": layout.NpmCache,
		}
		require.Len(t, asMap, len(expected), "expected exactly %d environment entries, got %v", len(expected), asMap)
		for key, value := range expected {
			require.Equal(t, value, asMap[key], "AsMap()[%q] mismatch", key)
		}
	})

	t.Run("WailsBinaryPath and the pinned Wails module/version are exact and stable", func(t *testing.T) {
		require.Equal(t, "github.com/wailsapp/wails/v2/cmd/wails", WailsModule, "unexpected WailsModule pin")
		require.Equal(t, "v2.13.0", WailsVersion, "unexpected WailsVersion pin")

		root := t.TempDir()
		layout, err := NewProjectCacheLayout(root)
		require.NoError(t, err, "construct layout")
		require.Equal(t, filepath.Join(layout.GoBin, "wails.exe"), layout.WailsBinaryPath(".exe"), "expected matching WailsBinaryPath(.exe)")
		require.Equal(t, filepath.Join(layout.GoBin, "wails"), layout.WailsBinaryPath(""), "expected matching WailsBinaryPath(\"\")")

		// config/toolchain.toml's [go_install.wails] pin (installGoInstallTools'
		// generic go_install provisioning loop) must never drift from these
		// constants -- WailsBinaryPath above reserves the exact GoBin path this
		// pin's install actually lands the binary at, so a version/module
		// mismatch between the two would silently provision the wrong Wails CLI
		// at the path mage RunDev (internal/command/rundev.go) expects.
		document, _, err := readBootstrapManifest(filepath.Join("..", ".."))
		require.NoError(t, err, "read production manifest")
		wails, ok := document.GoInstall["wails"]
		require.True(t, ok, "production manifest missing go_install.wails")
		require.Equal(t, WailsVersion, wails.Version, "go_install.wails version mismatch")
		require.Equal(t, WailsModule, wails.Module, "go_install.wails module mismatch")
	})

	t.Run("EnsureDirectories creates missing directories and rejects a path that is already a file", func(t *testing.T) {
		root := t.TempDir()
		nested := filepath.Join(root, "a", "b", "c")

		require.NoError(t, EnsureDirectories(nested), "expected nested directory creation to succeed")
		info, statErr := os.Stat(nested)
		require.NoError(t, statErr, "expected %q to exist as a directory", nested)
		require.True(t, info.IsDir(), "expected %q to exist as a directory", nested)

		// Idempotent: creating the same directory again must not fail.
		require.NoError(t, EnsureDirectories(nested), "expected idempotent re-creation to succeed")

		blockedPath := filepath.Join(root, "blocked-file")
		require.NoError(t, os.WriteFile(blockedPath, []byte("not a directory\n"), 0o644), "write blocking file")
		err := EnsureDirectories(blockedPath)
		require.Error(t, err, "expected creating a directory where a file already exists to fail")
		require.Contains(t, err.Error(), "BOOTSTRAP_CACHE_DIRECTORY", "expected BOOTSTRAP_CACHE_DIRECTORY diagnostic, got: %v", err)
	})
}
