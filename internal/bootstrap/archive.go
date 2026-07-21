// archive.go decomposes the D-01 executable-byte trust boundary into
// small, independently testable building blocks that sit in front of
// bootstrap.go's existing staged-install primitive: VerifySHA256 checks
// exact archive bytes, InspectZipEntries rejects unsafe archive structure
// (absolute paths, traversal, and symlink entries) before anything is
// extracted, ExtractVerified writes only into a fresh staging directory
// after both checks pass, and PromoteAtomically is the single place that
// moves staged bytes to an install location. Together they let
// downloader.go (and any future caller) fail closed at the earliest
// possible point instead of discovering a bad archive mid-extraction.
package bootstrap

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// VerifySHA256 confirms archivePath's bytes match the exact committed
// SHA-256 pin. It is the byte-integrity half of the trust boundary;
// archive structure is checked separately by InspectZipEntries.
func VerifySHA256(archivePath, expectedSHA256 string) error {
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
	return nil
}

// InspectZipEntries opens archivePath and rejects any entry that could
// escape extraction or promote unsafe bytes: absolute paths,
// drive-qualified paths, dot-dot segments (via checkEntryName, shared with
// bootstrap.go), and symlink entries. It never extracts anything; a
// symlink entry is rejected outright because GOLC never needs to install
// one and a malicious archive could otherwise point a link outside the
// staging root.
func InspectZipEntries(archivePath string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("BOOTSTRAP_ARCHIVE_INVALID: %w", err)
	}
	defer reader.Close()

	for _, entry := range reader.File {
		if err := checkEntryName(entry.Name); err != nil {
			return err
		}
		if entry.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("BOOTSTRAP_ARCHIVE_UNSAFE_LINK: entry %q is a symlink", entry.Name)
		}
	}
	return nil
}

// ExtractVerified verifies archivePath's checksum (VerifySHA256) and entry
// safety (InspectZipEntries), then extracts it into a fresh staging
// directory created under targetParent. It writes only inside that
// staging directory — never to targetParent's siblings or any install
// location — and returns the staging directory's absolute path. A failed
// verification or extraction leaves no staging residue. PromoteAtomically
// is the only function that may move the returned directory to an install
// location.
func ExtractVerified(archivePath, expectedSHA256, targetParent string) (stagingDir string, err error) {
	if err := VerifySHA256(archivePath, expectedSHA256); err != nil {
		return "", err
	}
	if err := InspectZipEntries(archivePath); err != nil {
		return "", err
	}

	if err := os.MkdirAll(targetParent, 0o755); err != nil {
		return "", fmt.Errorf("BOOTSTRAP_INSTALL_PARENT: %w", err)
	}
	staging, err := os.MkdirTemp(targetParent, ".golc-staging-")
	if err != nil {
		return "", fmt.Errorf("BOOTSTRAP_STAGING_CREATE: %w", err)
	}
	defer func() {
		if err != nil {
			os.RemoveAll(staging)
		}
	}()

	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", fmt.Errorf("BOOTSTRAP_ARCHIVE_INVALID: %w", err)
	}
	defer reader.Close()

	stagingRoot, err := filepath.Abs(staging)
	if err != nil {
		return "", fmt.Errorf("BOOTSTRAP_STAGING_CREATE: %w", err)
	}

	entries := append([]*zip.File(nil), reader.File...)
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })

	for _, entry := range entries {
		relative := filepath.FromSlash(strings.ReplaceAll(entry.Name, "\\", "/"))
		destination := filepath.Join(stagingRoot, relative)
		// Containment double-check after joining, independent of the name
		// check InspectZipEntries already performed.
		if destination != stagingRoot && !strings.HasPrefix(destination, stagingRoot+string(os.PathSeparator)) {
			return "", fmt.Errorf("BOOTSTRAP_ARCHIVE_TRAVERSAL: entry %q escapes staging root", entry.Name)
		}

		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(destination, 0o755); err != nil {
				return "", fmt.Errorf("BOOTSTRAP_EXTRACT: %w", err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return "", fmt.Errorf("BOOTSTRAP_EXTRACT: %w", err)
		}
		if err := extractEntry(entry, destination); err != nil {
			return "", err
		}
	}

	return stagingRoot, nil
}

// PromoteAtomically moves a verified staging directory to installDir with
// a single rename, exposing the complete verified tree or nothing. Any
// prior install at installDir is fully removed first so promotion never
// merges staged content with stale files; if the rename itself fails,
// installDir is left in whatever state os.RemoveAll left it and the
// staging directory is not cleaned up here, so the caller can inspect or
// retry.
func PromoteAtomically(stagingDir, installDir string) error {
	if err := os.MkdirAll(filepath.Dir(installDir), 0o755); err != nil {
		return fmt.Errorf("BOOTSTRAP_INSTALL_PARENT: %w", err)
	}
	if _, statErr := os.Stat(installDir); statErr == nil {
		if err := os.RemoveAll(installDir); err != nil {
			return fmt.Errorf("BOOTSTRAP_PROMOTE: %w", err)
		}
	}
	if err := os.Rename(stagingDir, installDir); err != nil {
		return fmt.Errorf("BOOTSTRAP_PROMOTE: %w", err)
	}
	return nil
}
