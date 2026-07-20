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
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ManifestName is the file recorded inside a promoted install directory that
// binds the installed bytes to the exact archive pin they came from.
const ManifestName = ".golc-install-manifest.json"

// InstallManifest records the archive pin and per-file integrity of a
// promoted install.
type InstallManifest struct {
	ArchiveSHA256 string          `json:"archive_sha256"`
	Files         []InstalledFile `json:"files"`
}

// InstalledFile is one extracted file with its lowercase hex SHA-256.
type InstalledFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
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
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("BOOTSTRAP_ARCHIVE_TRAVERSAL: empty entry name")
	}
	normalized := strings.ReplaceAll(name, "\\", "/")
	if strings.HasPrefix(normalized, "/") || strings.Contains(normalized, ":") {
		return fmt.Errorf("BOOTSTRAP_ARCHIVE_TRAVERSAL: rooted entry %q", name)
	}
	for _, segment := range strings.Split(normalized, "/") {
		if segment == ".." {
			return fmt.Errorf("BOOTSTRAP_ARCHIVE_TRAVERSAL: dot-dot segment in entry %q", name)
		}
	}
	return nil
}

// VerifyArchive confirms that the archive bytes match the exact SHA-256 pin
// and that every contained entry name stays inside the extraction root. It
// must pass before any extraction or promotion happens.
func VerifyArchive(archivePath, expectedSHA256 string) error {
	expected, err := normalizeExpectedSHA256(expectedSHA256)
	if err != nil {
		return err
	}

	actual, err := hashFile(archivePath)
	if err != nil {
		return fmt.Errorf("BOOTSTRAP_ARCHIVE_UNREADABLE: %w", err)
	}
	if actual != expected {
		return fmt.Errorf("BOOTSTRAP_CHECKSUM_MISMATCH: archive %s has sha256 %s, pin requires %s", archivePath, actual, expected)
	}

	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("BOOTSTRAP_ARCHIVE_INVALID: %w", err)
	}
	defer reader.Close()

	for _, entry := range reader.File {
		if err := checkEntryName(entry.Name); err != nil {
			return err
		}
	}
	return nil
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
	if err := VerifyArchive(archivePath, expected); err != nil {
		return err
	}

	parent := filepath.Dir(installDir)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("BOOTSTRAP_INSTALL_PARENT: %w", err)
	}
	staging, err := os.MkdirTemp(parent, ".golc-staging-")
	if err != nil {
		return fmt.Errorf("BOOTSTRAP_STAGING_CREATE: %w", err)
	}
	defer func() {
		if err != nil {
			os.RemoveAll(staging)
		}
	}()

	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("BOOTSTRAP_ARCHIVE_INVALID: %w", err)
	}
	defer reader.Close()

	stagingRoot, err := filepath.Abs(staging)
	if err != nil {
		return fmt.Errorf("BOOTSTRAP_STAGING_CREATE: %w", err)
	}

	var files []InstalledFile
	for _, entry := range reader.File {
		if err := checkEntryName(entry.Name); err != nil {
			return err
		}
		relative := filepath.FromSlash(strings.ReplaceAll(entry.Name, "\\", "/"))
		destination := filepath.Join(stagingRoot, relative)
		// Containment double-check after joining, independent of name checks.
		if destination != stagingRoot && !strings.HasPrefix(destination, stagingRoot+string(os.PathSeparator)) {
			return fmt.Errorf("BOOTSTRAP_ARCHIVE_TRAVERSAL: entry %q escapes staging root", entry.Name)
		}

		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(destination, 0o755); err != nil {
				return fmt.Errorf("BOOTSTRAP_EXTRACT: %w", err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return fmt.Errorf("BOOTSTRAP_EXTRACT: %w", err)
		}
		if err := extractEntry(entry, destination); err != nil {
			return err
		}
		fileHash, err := hashFile(destination)
		if err != nil {
			return fmt.Errorf("BOOTSTRAP_EXTRACT: %w", err)
		}
		files = append(files, InstalledFile{
			Path:   strings.ReplaceAll(strings.ReplaceAll(relative, "\\", "/"), "//", "/"),
			SHA256: fileHash,
		})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })

	manifest := InstallManifest{ArchiveSHA256: expected, Files: files}
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("BOOTSTRAP_MANIFEST_WRITE: %w", err)
	}
	if err := os.WriteFile(filepath.Join(stagingRoot, ManifestName), append(manifestBytes, '\n'), 0o644); err != nil {
		return fmt.Errorf("BOOTSTRAP_MANIFEST_WRITE: %w", err)
	}

	if _, statErr := os.Stat(installDir); statErr == nil {
		if err := os.RemoveAll(installDir); err != nil {
			return fmt.Errorf("BOOTSTRAP_PROMOTE: %w", err)
		}
	}
	if err := os.Rename(staging, installDir); err != nil {
		return fmt.Errorf("BOOTSTRAP_PROMOTE: %w", err)
	}
	return nil
}

func extractEntry(entry *zip.File, destination string) error {
	source, err := entry.Open()
	if err != nil {
		return fmt.Errorf("BOOTSTRAP_EXTRACT: %w", err)
	}
	defer source.Close()

	target, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return fmt.Errorf("BOOTSTRAP_EXTRACT: %w", err)
	}
	if _, err := io.Copy(target, source); err != nil {
		target.Close()
		return fmt.Errorf("BOOTSTRAP_EXTRACT: %w", err)
	}
	if err := target.Close(); err != nil {
		return fmt.Errorf("BOOTSTRAP_EXTRACT: %w", err)
	}
	return nil
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
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return false, nil
	}
	if manifest.ArchiveSHA256 != expected {
		return false, nil
	}
	for _, file := range manifest.Files {
		if err := checkEntryName(file.Path); err != nil {
			return false, nil
		}
		actual, err := hashFile(filepath.Join(installDir, filepath.FromSlash(file.Path)))
		if err != nil {
			if os.IsNotExist(err) {
				return false, nil
			}
			return false, fmt.Errorf("BOOTSTRAP_MANIFEST_READ: %w", err)
		}
		if actual != file.SHA256 {
			return false, nil
		}
	}
	return true, nil
}
