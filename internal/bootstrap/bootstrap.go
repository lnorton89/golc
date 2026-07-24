// Package bootstrap implements the checksum-controlled installation boundary
// for pinned project-local tools.
//
// Archive bytes are verified against an exact SHA-256 pin and every contained
// entry is checked for path traversal before extraction. Extraction happens in
// a staging directory beside the install target and is promoted with a single
// rename, so a failed verification or extraction leaves no install. A recorded
// install manifest lets a matching second bootstrap skip the archive source
// entirely. Pins are immutable inputs: nothing in this package consults an
// update feed or rewrites a pinned version or hash.
package bootstrap

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

// ManifestName is the file recorded inside a promoted install directory that
// binds the installed bytes to the exact archive pin they came from.
const (
	ManifestName                 = ".golc-install-manifest.json"
	InstallManifestSchemaVersion = 1
)

// InstallManifest records the archive pin and per-file integrity of a
// promoted install.
type InstallManifest struct {
	SchemaVersion int             `json:"schema_version"`
	ArchiveSHA256 string          `json:"archive_sha256"`
	Files         []InstalledFile `json:"files"`
}

// InstalledFile is one extracted file with its lowercase hex SHA-256 and
// ordinary permission bits formatted as a four-digit octal string.
type InstalledFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Mode   string `json:"mode"`
}

// EnsureDirectories creates every directory in paths (including parents) if
// missing, so cache-layout warming (cache.go's ProjectCacheLayout.Warm) and
// staged installs share one exact "make it exist" primitive instead of
// duplicating os.MkdirAll calls with inconsistent permissions. It fails if
// a path already exists as a non-directory.
func EnsureDirectories(paths ...string) error {
	for _, path := range paths {
		if err := os.MkdirAll(path, 0o755); err != nil {
			return fmt.Errorf("BOOTSTRAP_CACHE_DIRECTORY: %s: %w", path, err)
		}
	}
	return nil
}

// normalizeExpectedSHA256 validates and canonicalizes an exact pin.
func normalizeExpectedSHA256(expected string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(expected))
	if len(normalized) != sha256.Size*2 {
		return "", fmt.Errorf("BOOTSTRAP_CHECKSUM_FORMAT: expected 64 hex characters, got %d", len(normalized))
	}
	if _, err := hex.DecodeString(normalized); err != nil {
		return "", fmt.Errorf("BOOTSTRAP_CHECKSUM_FORMAT: pin is not hexadecimal: %w", err)
	}
	return normalized, nil
}

func hashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

// checkEntryName rejects archive entry names that could escape the
// extraction root: absolute paths, drive-qualified paths, and any dot-dot
// segment, using both slash conventions.
func checkEntryName(name string) error {
	_, err := normalizeArchiveEntryName(name)
	return err
}

// VerifyArchive confirms that the archive bytes match the exact SHA-256 pin
// and that every contained entry name stays inside the extraction root. It
// must pass before any extraction or promotion happens.
func VerifyArchive(archivePath, expectedSHA256 string) error {
	if err := VerifySHA256(archivePath, expectedSHA256); err != nil {
		return err
	}
	format, err := archiveFormatFromPath(archivePath)
	if err != nil {
		return err
	}
	_, err = inspectArchive(archivePath, format)
	return err
}

// InstallStaged verifies the archive, extracts it into a staging directory
// beside the install target, records the install manifest, and promotes the
// staging directory to installDir with a single rename. A verification or
// extraction failure leaves no install and no staging residue.
func InstallStaged(archivePath, expectedSHA256, installDir string) (err error) {
	expected, err := normalizeExpectedSHA256(expectedSHA256)
	if err != nil {
		return err
	}
	parent := filepath.Dir(installDir)
	staging, err := ExtractVerified(archivePath, expected, parent)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(staging)
		}
	}()
	format, err := archiveFormatFromPath(archivePath)
	if err != nil {
		return err
	}
	archiveEntries, err := inspectArchive(archivePath, format)
	if err != nil {
		return err
	}
	files, err := inventoryInstalledFiles(staging, archiveEntries)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("BOOTSTRAP_MANIFEST_WRITE: archive contains no regular files")
	}
	manifest := InstallManifest{SchemaVersion: InstallManifestSchemaVersion, ArchiveSHA256: expected, Files: files}
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("BOOTSTRAP_MANIFEST_WRITE: %w", err)
	}
	if err := os.WriteFile(filepath.Join(staging, ManifestName), append(manifestBytes, '\n'), 0o644); err != nil {
		return fmt.Errorf("BOOTSTRAP_MANIFEST_WRITE: %w", err)
	}
	return PromoteAtomically(staging, installDir)
}

func inventoryInstalledFiles(root string, archiveEntries []archiveEntry) ([]InstalledFile, error) {
	archiveModes := make(map[string]os.FileMode, len(archiveEntries))
	for _, entry := range archiveEntries {
		if !entry.isDir {
			archiveModes[entry.name] = entry.mode.Perm()
		}
	}
	var files []InstalledFile
	err := filepath.WalkDir(root, func(current string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if current == root || entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("BOOTSTRAP_MANIFEST_WRITE: %q is not a regular file", current)
		}
		relative, err := filepath.Rel(root, current)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(relative)
		digest, err := hashFile(current)
		if err != nil {
			return err
		}
		mode, ok := archiveModes[name]
		if !ok {
			return fmt.Errorf("archive inventory has no regular file %q", name)
		}
		files = append(files, InstalledFile{Path: name, SHA256: digest, Mode: fmt.Sprintf("%04o", mode)})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("BOOTSTRAP_MANIFEST_WRITE: %w", err)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

// InstalledMatches reports whether installDir already holds a promoted
// install of the exact pinned archive: the recorded manifest must name the
// same archive hash and every recorded file must still hash identically.
// When it returns true, bootstrap makes zero archive-source calls.
func InstalledMatches(installDir, expectedSHA256 string) (bool, error) {
	expected, err := normalizeExpectedSHA256(expectedSHA256)
	if err != nil {
		return false, err
	}

	manifestBytes, err := os.ReadFile(filepath.Join(installDir, ManifestName))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("BOOTSTRAP_MANIFEST_READ: %w", err)
	}
	var manifest InstallManifest
	decoder := json.NewDecoder(bytes.NewReader(manifestBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return false, nil
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return false, nil
	}
	if manifest.SchemaVersion != InstallManifestSchemaVersion || manifest.ArchiveSHA256 != expected || len(manifest.Files) == 0 {
		return false, nil
	}
	expectedFiles := make(map[string]InstalledFile, len(manifest.Files))
	expectedDirs := map[string]struct{}{".": {}}
	for _, file := range manifest.Files {
		normalized, err := normalizeArchiveEntryName(file.Path)
		if err != nil || normalized != file.Path {
			return false, nil
		}
		if _, exists := expectedFiles[file.Path]; exists {
			return false, nil
		}
		if len(file.SHA256) != sha256.Size*2 || strings.ToLower(file.SHA256) != file.SHA256 {
			return false, nil
		}
		if _, err := hex.DecodeString(file.SHA256); err != nil {
			return false, nil
		}
		modeValue, err := strconv.ParseUint(file.Mode, 8, 12)
		if err != nil || len(file.Mode) != 4 || modeValue > 0o777 {
			return false, nil
		}
		expectedFiles[file.Path] = file
		for directory := pathDirectory(file.Path); directory != "."; directory = pathDirectory(directory) {
			expectedDirs[directory] = struct{}{}
		}
		actualPath := filepath.Join(installDir, filepath.FromSlash(file.Path))
		info, err := os.Lstat(actualPath)
		if err != nil {
			if os.IsNotExist(err) {
				return false, nil
			}
			return false, fmt.Errorf("BOOTSTRAP_MANIFEST_READ: %w", err)
		}
		if !info.Mode().IsRegular() {
			return false, nil
		}
		actual, err := hashFile(actualPath)
		if err != nil {
			return false, fmt.Errorf("BOOTSTRAP_MANIFEST_READ: %w", err)
		}
		if actual != file.SHA256 {
			return false, nil
		}
		if runtime.GOOS != "windows" && info.Mode().Perm() != os.FileMode(modeValue) {
			return false, nil
		}
	}
	err = filepath.WalkDir(installDir, func(current string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if current == installDir {
			return nil
		}
		relative, err := filepath.Rel(installDir, current)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(relative)
		if name == ManifestName {
			return nil
		}
		if entry.IsDir() {
			if _, ok := expectedDirs[name]; !ok {
				return errManifestMismatch
			}
			return nil
		}
		if _, ok := expectedFiles[name]; !ok {
			return errManifestMismatch
		}
		return nil
	})
	if err == errManifestMismatch {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("BOOTSTRAP_MANIFEST_READ: %w", err)
	}
	return true, nil
}

var errManifestMismatch = fmt.Errorf("manifest inventory mismatch")

func pathDirectory(name string) string {
	index := strings.LastIndex(name, "/")
	if index < 0 {
		return "."
	}
	return name[:index]
}
