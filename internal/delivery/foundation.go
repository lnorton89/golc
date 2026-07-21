// foundation.go implements Plan 01-20's deterministic foundation package
// (01-RESEARCH.md Open Question 3, T-01-16): a reproducible Windows AMD64
// developer-tool ZIP, its canonical sorted manifest, and a SHA-256
// checksum sidecar for the manifest and the archive itself.
//
// This package never imports internal/command (see graph.go's package
// doc for why); internal/command/package.go is the only self-registered
// route that calls BuildFoundationBundle, mirroring check.go's use of
// LoadGraph/RunOffline.
//
// Bundle contents are sourced from the one authoritative graph inventory
// LoadGraph already owns (CommandInventory.Entrypoint/CLIBinary) plus a
// bounded, explicit set of committed contributor-facing paths (root
// config index, every config/**/*.toml concern, every committed
// schemas/*.json contract, and docs/development.md). FoundationInventory
// never walks an unbounded tree: it is a fixed, sorted allowlist, so an
// unrelated new top-level file (a future Wails frontend, an NSIS
// installer script, lighting-domain source) can never silently enter this
// "developer tooling only" deliverable (Phase 1 boundary,
// 01-CONTEXT.md).
package delivery

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// FoundationEntry is one file the deterministic foundation bundle
// carries: its canonical forward-slash archive path and the exact
// repository-relative source path its bytes are read from.
type FoundationEntry struct {
	// ArchivePath is the forward-slash path this file occupies inside the
	// ZIP and the manifest — never a machine-specific or backslash path.
	ArchivePath string
	// SourcePath is the repository-relative path (OS-native separators
	// are accepted; ArchivePath is always the normalized forward-slash
	// form) BuildFoundationBundle reads this entry's bytes from.
	SourcePath string
}

// foundationFixedEntries are the exact non-directory-derived paths every
// foundation bundle carries in addition to the graph inventory's
// Entrypoint/CLIBinary and the config/schemas directory scans below.
var foundationFixedEntries = []string{
	"golc.project.toml",
	"docs/development.md",
}

// FoundationInventory returns the sorted, duplicate-free allowlist of
// files the foundation ZIP carries: the graph's own Entrypoint and
// CLIBinary, golc.project.toml, docs/development.md, every committed
// config/**/*.toml concern file, and every committed schemas/*.json
// contract. It is the single declarative allowlist BuildFoundationBundle
// consumes (T-01-16: no second, independently maintained file list exists
// elsewhere in this package).
func FoundationInventory(root string, inventory CommandInventory) ([]FoundationEntry, error) {
	entries := make([]FoundationEntry, 0, len(foundationFixedEntries)+8)

	if strings.TrimSpace(inventory.Entrypoint) == "" || strings.TrimSpace(inventory.CLIBinary) == "" {
		return nil, fmt.Errorf("GOLC_FOUNDATION_INVENTORY: graph inventory is incomplete")
	}
	entries = append(entries,
		FoundationEntry{ArchivePath: filepath.ToSlash(inventory.Entrypoint), SourcePath: inventory.Entrypoint},
		FoundationEntry{ArchivePath: filepath.ToSlash(inventory.CLIBinary), SourcePath: inventory.CLIBinary},
	)
	for _, relative := range foundationFixedEntries {
		entries = append(entries, FoundationEntry{ArchivePath: relative, SourcePath: relative})
	}

	configFiles, err := collectSortedFiles(root, "config", ".toml")
	if err != nil {
		return nil, err
	}
	entries = append(entries, configFiles...)

	schemaFiles, err := collectSortedFiles(root, "schemas", ".json")
	if err != nil {
		return nil, err
	}
	entries = append(entries, schemaFiles...)

	sort.Slice(entries, func(i, j int) bool { return entries[i].ArchivePath < entries[j].ArchivePath })

	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		if _, duplicate := seen[entry.ArchivePath]; duplicate {
			return nil, fmt.Errorf("GOLC_FOUNDATION_INVENTORY: duplicate archive path %q", entry.ArchivePath)
		}
		seen[entry.ArchivePath] = struct{}{}
	}
	return entries, nil
}

// collectSortedFiles walks root/subdir (which must already exist) and
// returns every file whose lowercased extension matches extension as a
// sorted FoundationEntry slice, keyed by its exact repository-relative
// forward-slash path. It never recurses outside subdir and never skips a
// walk error, so a missing or unreadable committed directory fails the
// build closed instead of silently shrinking the bundle.
func collectSortedFiles(root, subdir, extension string) ([]FoundationEntry, error) {
	base := filepath.Join(root, subdir)
	entries := []FoundationEntry{}
	err := filepath.Walk(base, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		if !strings.EqualFold(filepath.Ext(path), extension) {
			return nil
		}
		relative, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		archivePath := filepath.ToSlash(relative)
		entries = append(entries, FoundationEntry{ArchivePath: archivePath, SourcePath: relative})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("GOLC_FOUNDATION_INVENTORY: %s: %v", subdir, err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].ArchivePath < entries[j].ArchivePath })
	return entries, nil
}

// ManifestFileEntry is one canonical, hashed, sorted manifest record: no
// machine path, file mode, or timestamp is present (D-08, T-01-16).
type ManifestFileEntry struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

// Manifest is the canonical sorted inventory a foundation bundle carries.
type Manifest struct {
	SchemaVersion int                 `json:"schema_version"`
	Files         []ManifestFileEntry `json:"files"`
}

// CanonicalManifest sorts entries by archive path, rejects a blank or
// duplicate archive path, and reads every entry's exact bytes from root.
// It returns the canonical Manifest plus each entry's raw payload in the
// same sorted order: BuildFoundationBundle writes ZIP entries in this
// exact order so archive bytes and manifest bytes always stay in
// lockstep, and repeating the call against unchanged inputs is
// byte-identical.
func CanonicalManifest(root string, entries []FoundationEntry) (Manifest, [][]byte, error) {
	sorted := append([]FoundationEntry(nil), entries...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ArchivePath < sorted[j].ArchivePath })

	seen := make(map[string]struct{}, len(sorted))
	files := make([]ManifestFileEntry, 0, len(sorted))
	payloads := make([][]byte, 0, len(sorted))
	for _, entry := range sorted {
		archivePath := strings.TrimSpace(entry.ArchivePath)
		if archivePath == "" {
			return Manifest{}, nil, fmt.Errorf("GOLC_FOUNDATION_MANIFEST: entry has a blank archive path")
		}
		if _, duplicate := seen[archivePath]; duplicate {
			return Manifest{}, nil, fmt.Errorf("GOLC_FOUNDATION_MANIFEST: duplicate archive path %q", archivePath)
		}
		seen[archivePath] = struct{}{}

		sourcePath := filepath.Join(root, filepath.FromSlash(entry.SourcePath))
		data, err := os.ReadFile(sourcePath)
		if err != nil {
			return Manifest{}, nil, fmt.Errorf("GOLC_FOUNDATION_MANIFEST: %s: %v", archivePath, err)
		}
		sum := sha256.Sum256(data)
		files = append(files, ManifestFileEntry{
			Path:   archivePath,
			SHA256: hex.EncodeToString(sum[:]),
			Size:   int64(len(data)),
		})
		payloads = append(payloads, data)
	}
	return Manifest{SchemaVersion: 1, Files: files}, payloads, nil
}

// EncodeManifest renders m as canonical, byte-stable JSON: fixed struct
// field order, two-space indentation, and a single trailing LF. CanonicalManifest
// already returns Files in sorted order, so no further reordering is
// needed here; no timestamp or random value is ever present.
func EncodeManifest(m Manifest) ([]byte, error) {
	encoded, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("GOLC_FOUNDATION_MANIFEST_ENCODE: %v", err)
	}
	return append(encoded, '\n'), nil
}

// foundationZIPEpoch is the fixed, machine-independent timestamp every
// ZIP entry carries. ZIP metadata carries a per-entry modification time;
// using any wall-clock value would make two otherwise byte-identical
// builds diverge, so every entry uses this one fixed value (the DOS date
// floor, 1980-01-01) instead of the real file time.
var foundationZIPEpoch = time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC)

// manifestArchiveName is the fixed archive-internal name the encoded
// manifest itself occupies, alongside every file it describes.
const manifestArchiveName = "foundation-manifest.json"

// FoundationBundle is BuildFoundationBundle's exact deterministic output.
type FoundationBundle struct {
	Manifest         Manifest
	ManifestBytes    []byte
	ZIPBytes         []byte
	ZIPChecksum      string
	ManifestChecksum string
}

// BuildFoundationBundle produces the deterministic Windows AMD64
// developer-tool ZIP archive and its canonical manifest (T-01-16):
// identical repository inputs always produce byte-identical ZIP bytes,
// manifest bytes, and checksums. Every ZIP entry uses the fixed epoch
// timestamp, a normalized 0644 file mode, forward-slash archive paths,
// and entries are written in the manifest's exact sorted order, so no
// machine path, real timestamp, or map-iteration-order artifact can leak
// into the archive bytes. The package is developer tooling only: it never
// builds, stages, or references a Wails UI or an NSIS product installer.
func BuildFoundationBundle(root string) (FoundationBundle, error) {
	graph, err := LoadGraph(root)
	if err != nil {
		return FoundationBundle{}, err
	}

	entries, err := FoundationInventory(root, graph.Inventory)
	if err != nil {
		return FoundationBundle{}, err
	}

	manifest, payloads, err := CanonicalManifest(root, entries)
	if err != nil {
		return FoundationBundle{}, err
	}

	manifestBytes, err := EncodeManifest(manifest)
	if err != nil {
		return FoundationBundle{}, err
	}

	zipBytes, err := buildDeterministicZIP(manifest, payloads, manifestBytes)
	if err != nil {
		return FoundationBundle{}, err
	}

	zipSum := sha256.Sum256(zipBytes)
	manifestSum := sha256.Sum256(manifestBytes)

	return FoundationBundle{
		Manifest:         manifest,
		ManifestBytes:    manifestBytes,
		ZIPBytes:         zipBytes,
		ZIPChecksum:      hex.EncodeToString(zipSum[:]),
		ManifestChecksum: hex.EncodeToString(manifestSum[:]),
	}, nil
}

// buildDeterministicZIP writes manifest.Files (in their already-sorted
// order) followed by one fixed-name manifest entry into a new in-memory
// ZIP archive, normalizing every entry's metadata.
func buildDeterministicZIP(manifest Manifest, payloads [][]byte, manifestBytes []byte) ([]byte, error) {
	if len(manifest.Files) != len(payloads) {
		return nil, fmt.Errorf("GOLC_FOUNDATION_ZIP: manifest/payload count mismatch")
	}

	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)

	writeEntry := func(name string, data []byte) error {
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		header.SetMode(0o644)
		header.Modified = foundationZIPEpoch
		entryWriter, err := writer.CreateHeader(header)
		if err != nil {
			return fmt.Errorf("GOLC_FOUNDATION_ZIP: %s: %v", name, err)
		}
		if _, err := entryWriter.Write(data); err != nil {
			return fmt.Errorf("GOLC_FOUNDATION_ZIP: %s: %v", name, err)
		}
		return nil
	}

	for i, file := range manifest.Files {
		if err := writeEntry(file.Path, payloads[i]); err != nil {
			return nil, err
		}
	}
	if err := writeEntry(manifestArchiveName, manifestBytes); err != nil {
		return nil, err
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("GOLC_FOUNDATION_ZIP: %v", err)
	}
	return buffer.Bytes(), nil
}

// FoundationOutputPaths are the exact repository-relative output
// locations `package --foundation` writes (D-08: generated distribution
// artifacts are never committed source).
type FoundationOutputPaths struct {
	ZIPPath      string
	ManifestPath string
	ChecksumPath string
}

// DefaultFoundationOutputPaths is the one fixed output location
// `package --foundation` writes to under root/dist/foundation. Running
// the command twice overwrites the exact same paths, so
// tests/acceptance/offline.ps1's repeat-and-compare verification observes
// the same file identity rather than two differently-named artifacts.
func DefaultFoundationOutputPaths(root string) FoundationOutputPaths {
	base := filepath.Join(root, "dist", "foundation")
	return FoundationOutputPaths{
		ZIPPath:      filepath.Join(base, "golc-foundation-windows-amd64.zip"),
		ManifestPath: filepath.Join(base, "golc-foundation-windows-amd64.manifest.json"),
		ChecksumPath: filepath.Join(base, "golc-foundation-windows-amd64.zip.sha256"),
	}
}

// WriteFoundationBundle atomically writes bundle's ZIP, manifest, and a
// checksum sidecar (the standard "<hex>  <filename>\n" shape sha256sum-
// family tools emit) to paths, replacing any prior output at those exact
// locations.
func WriteFoundationBundle(bundle FoundationBundle, paths FoundationOutputPaths) error {
	if err := os.MkdirAll(filepath.Dir(paths.ZIPPath), 0o755); err != nil {
		return fmt.Errorf("GOLC_FOUNDATION_WRITE: %v", err)
	}
	if err := writeFileAtomic(paths.ZIPPath, bundle.ZIPBytes); err != nil {
		return err
	}
	if err := writeFileAtomic(paths.ManifestPath, bundle.ManifestBytes); err != nil {
		return err
	}
	checksumLine := fmt.Sprintf("%s  %s\n", bundle.ZIPChecksum, filepath.Base(paths.ZIPPath))
	if err := writeFileAtomic(paths.ChecksumPath, []byte(checksumLine)); err != nil {
		return err
	}
	return nil
}

// writeFileAtomic stages data beside path and promotes it with a single
// rename, so a reader can never observe a partially written output file.
func writeFileAtomic(path string, data []byte) error {
	staging := path + ".staging"
	if err := os.WriteFile(staging, data, 0o644); err != nil {
		return fmt.Errorf("GOLC_FOUNDATION_WRITE: %s: %v", path, err)
	}
	if err := os.Rename(staging, path); err != nil {
		return fmt.Errorf("GOLC_FOUNDATION_WRITE: %s: %v", path, err)
	}
	return nil
}
