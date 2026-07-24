package bootstrap

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

type archiveFormat uint8

const (
	archiveZIP archiveFormat = iota + 1
	archiveTarGz
)

type archiveEntry struct {
	name  string
	mode  os.FileMode
	isDir bool
}

func archiveFormatFromPath(archivePath string) (archiveFormat, error) {
	lower := strings.ToLower(archivePath)
	switch {
	case strings.HasSuffix(lower, ".tar.gz"):
		return archiveTarGz, nil
	case strings.HasSuffix(lower, ".zip"):
		return archiveZIP, nil
	default:
		return 0, fmt.Errorf("BOOTSTRAP_ARCHIVE_FORMAT: %q must end in .zip or .tar.gz", archivePath)
	}
}

func normalizeArchiveEntryName(name string) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("BOOTSTRAP_ARCHIVE_TRAVERSAL: empty entry name")
	}
	normalized := strings.ReplaceAll(name, "\\", "/")
	if strings.HasPrefix(normalized, "/") || strings.Contains(normalized, ":") {
		return "", fmt.Errorf("BOOTSTRAP_ARCHIVE_TRAVERSAL: rooted entry %q", name)
	}
	for _, segment := range strings.Split(normalized, "/") {
		if segment == ".." {
			return "", fmt.Errorf("BOOTSTRAP_ARCHIVE_TRAVERSAL: dot-dot segment in entry %q", name)
		}
	}
	normalized = path.Clean(normalized)
	if normalized == "." || normalized == "" || strings.HasPrefix(normalized, "../") {
		return "", fmt.Errorf("BOOTSTRAP_ARCHIVE_TRAVERSAL: invalid entry %q", name)
	}
	return normalized, nil
}

func validateUniqueEntry(seen map[string]struct{}, name string) error {
	if _, exists := seen[name]; exists {
		return fmt.Errorf("BOOTSTRAP_ARCHIVE_DUPLICATE: normalized entry %q occurs more than once", name)
	}
	seen[name] = struct{}{}
	return nil
}

// VerifySHA256 confirms archivePath's bytes match the exact committed pin.
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

// InspectZipEntries validates the complete ZIP before any staging directory is
// created. Only directories and regular files are accepted.
func InspectZipEntries(archivePath string) error {
	_, err := inspectZipEntries(archivePath)
	return err
}

func inspectZipEntries(archivePath string) ([]archiveEntry, error) {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, fmt.Errorf("BOOTSTRAP_ARCHIVE_FORMAT: expected ZIP content for %q: %w", archivePath, err)
	}
	defer reader.Close()

	seen := make(map[string]struct{}, len(reader.File))
	entries := make([]archiveEntry, 0, len(reader.File))
	for _, item := range reader.File {
		name, err := normalizeArchiveEntryName(item.Name)
		if err != nil {
			return nil, err
		}
		if err := validateUniqueEntry(seen, name); err != nil {
			return nil, err
		}
		mode := item.Mode()
		isDir := item.FileInfo().IsDir()
		if !isDir && !mode.IsRegular() {
			if mode&os.ModeSymlink != 0 {
				return nil, fmt.Errorf("BOOTSTRAP_ARCHIVE_UNSAFE_LINK: entry %q is a symlink", item.Name)
			}
			return nil, fmt.Errorf("BOOTSTRAP_ARCHIVE_UNSAFE_TYPE: ZIP entry %q is not a directory or regular file", item.Name)
		}
		perm := mode.Perm()
		if perm == 0 {
			if isDir {
				perm = 0o755
			} else {
				perm = 0o644
			}
		}
		entries = append(entries, archiveEntry{name: name, mode: perm, isDir: isDir})
	}
	return entries, nil
}

func inspectTarGzEntries(archivePath string) ([]archiveEntry, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return nil, fmt.Errorf("BOOTSTRAP_ARCHIVE_UNREADABLE: %w", err)
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return nil, fmt.Errorf("BOOTSTRAP_ARCHIVE_FORMAT: expected gzip content for %q: %w", archivePath, err)
	}
	defer gzipReader.Close()
	reader := tar.NewReader(gzipReader)

	seen := map[string]struct{}{}
	var entries []archiveEntry
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("BOOTSTRAP_ARCHIVE_FORMAT: invalid tar.gz %q: %w", archivePath, err)
		}
		name, err := normalizeArchiveEntryName(header.Name)
		if err != nil {
			return nil, err
		}
		if err := validateUniqueEntry(seen, name); err != nil {
			return nil, err
		}
		isDir := false
		switch header.Typeflag {
		case tar.TypeDir:
			isDir = true
		case tar.TypeReg, tar.TypeRegA:
		default:
			return nil, fmt.Errorf("BOOTSTRAP_ARCHIVE_UNSAFE_TYPE: tar entry %q has rejected type %d", header.Name, header.Typeflag)
		}
		perm := os.FileMode(header.Mode) & os.ModePerm
		if perm == 0 {
			if isDir {
				perm = 0o755
			} else {
				perm = 0o644
			}
		}
		entries = append(entries, archiveEntry{name: name, mode: perm, isDir: isDir})
	}
	return entries, nil
}

func inspectArchive(archivePath string, format archiveFormat) ([]archiveEntry, error) {
	switch format {
	case archiveZIP:
		return inspectZipEntries(archivePath)
	case archiveTarGz:
		return inspectTarGzEntries(archivePath)
	default:
		return nil, fmt.Errorf("BOOTSTRAP_ARCHIVE_FORMAT: unsupported archive format")
	}
}

func containedDestination(root, name string) (string, error) {
	destination := filepath.Join(root, filepath.FromSlash(name))
	relative, err := filepath.Rel(root, destination)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("BOOTSTRAP_ARCHIVE_TRAVERSAL: entry %q escapes staging root", name)
	}
	return destination, nil
}

func openExtractedFile(destination string, mode os.FileMode) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return nil, fmt.Errorf("BOOTSTRAP_EXTRACT: %w", err)
	}
	file, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, fmt.Errorf("BOOTSTRAP_EXTRACT: %w", err)
	}
	return file, nil
}

func finishExtractedFile(file *os.File, mode os.FileMode) error {
	if err := file.Close(); err != nil {
		return fmt.Errorf("BOOTSTRAP_EXTRACT: %w", err)
	}
	if err := os.Chmod(file.Name(), mode.Perm()); err != nil {
		return fmt.Errorf("BOOTSTRAP_EXTRACT: %w", err)
	}
	return nil
}

func extractZipArchive(archivePath, root string, inspected []archiveEntry) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("BOOTSTRAP_ARCHIVE_FORMAT: expected ZIP content for %q: %w", archivePath, err)
	}
	defer reader.Close()
	entries := append([]*zip.File(nil), reader.File...)
	sort.SliceStable(entries, func(i, j int) bool {
		left, _ := normalizeArchiveEntryName(entries[i].Name)
		right, _ := normalizeArchiveEntryName(entries[j].Name)
		return left < right
	})
	modes := make(map[string]archiveEntry, len(inspected))
	for _, item := range inspected {
		modes[item.name] = item
	}
	for _, item := range entries {
		name, _ := normalizeArchiveEntryName(item.Name)
		entry := modes[name]
		destination, err := containedDestination(root, name)
		if err != nil {
			return err
		}
		if entry.isDir {
			if err := os.MkdirAll(destination, 0o755); err != nil {
				return fmt.Errorf("BOOTSTRAP_EXTRACT: %w", err)
			}
			continue
		}
		source, err := item.Open()
		if err != nil {
			return fmt.Errorf("BOOTSTRAP_EXTRACT: %w", err)
		}
		target, err := openExtractedFile(destination, entry.mode)
		if err != nil {
			source.Close()
			return err
		}
		_, copyErr := io.Copy(target, source)
		sourceErr := source.Close()
		if copyErr != nil {
			target.Close()
			return fmt.Errorf("BOOTSTRAP_EXTRACT: %w", copyErr)
		}
		if sourceErr != nil {
			target.Close()
			return fmt.Errorf("BOOTSTRAP_EXTRACT: %w", sourceErr)
		}
		if err := finishExtractedFile(target, entry.mode); err != nil {
			return err
		}
	}
	return applyDirectoryModes(root, inspected)
}

func extractTarGzArchive(archivePath, root string, inspected []archiveEntry) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("BOOTSTRAP_ARCHIVE_UNREADABLE: %w", err)
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("BOOTSTRAP_ARCHIVE_FORMAT: expected gzip content for %q: %w", archivePath, err)
	}
	defer gzipReader.Close()
	reader := tar.NewReader(gzipReader)
	modes := make(map[string]archiveEntry, len(inspected))
	for _, item := range inspected {
		modes[item.name] = item
	}
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("BOOTSTRAP_ARCHIVE_FORMAT: invalid tar.gz %q: %w", archivePath, err)
		}
		name, _ := normalizeArchiveEntryName(header.Name)
		entry := modes[name]
		destination, err := containedDestination(root, name)
		if err != nil {
			return err
		}
		if entry.isDir {
			if err := os.MkdirAll(destination, 0o755); err != nil {
				return fmt.Errorf("BOOTSTRAP_EXTRACT: %w", err)
			}
			continue
		}
		target, err := openExtractedFile(destination, entry.mode)
		if err != nil {
			return err
		}
		if _, err := io.Copy(target, reader); err != nil {
			target.Close()
			return fmt.Errorf("BOOTSTRAP_EXTRACT: %w", err)
		}
		if err := finishExtractedFile(target, entry.mode); err != nil {
			return err
		}
	}
	return applyDirectoryModes(root, inspected)
}

func applyDirectoryModes(root string, entries []archiveEntry) error {
	var directories []archiveEntry
	for _, entry := range entries {
		if entry.isDir {
			directories = append(directories, entry)
		}
	}
	sort.Slice(directories, func(i, j int) bool {
		leftDepth := strings.Count(directories[i].name, "/")
		rightDepth := strings.Count(directories[j].name, "/")
		if leftDepth == rightDepth {
			return directories[i].name > directories[j].name
		}
		return leftDepth > rightDepth
	})
	for _, directory := range directories {
		destination, err := containedDestination(root, directory.name)
		if err != nil {
			return err
		}
		if err := os.Chmod(destination, directory.mode.Perm()); err != nil {
			return fmt.Errorf("BOOTSTRAP_EXTRACT: %w", err)
		}
	}
	return nil
}

// ExtractVerified verifies checksum and the complete archive structure before
// creating a staging directory, then reopens the archive for extraction.
func ExtractVerified(archivePath, expectedSHA256, targetParent string) (stagingDir string, err error) {
	if err := VerifySHA256(archivePath, expectedSHA256); err != nil {
		return "", err
	}
	format, err := archiveFormatFromPath(archivePath)
	if err != nil {
		return "", err
	}
	entries, err := inspectArchive(archivePath, format)
	if err != nil {
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
			_ = os.RemoveAll(staging)
		}
	}()
	root, err := filepath.Abs(staging)
	if err != nil {
		return "", fmt.Errorf("BOOTSTRAP_STAGING_CREATE: %w", err)
	}
	switch format {
	case archiveZIP:
		err = extractZipArchive(archivePath, root, entries)
	case archiveTarGz:
		err = extractTarGzArchive(archivePath, root, entries)
	}
	if err != nil {
		return "", err
	}
	return root, nil
}

// PromoteAtomically swaps a fully staged directory into installDir. If a prior
// install exists it is renamed aside first and restored if promotion fails.
func PromoteAtomically(stagingDir, installDir string) error {
	parent := filepath.Dir(installDir)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("BOOTSTRAP_INSTALL_PARENT: %w", err)
	}
	backup := ""
	if _, err := os.Stat(installDir); err == nil {
		placeholder, err := os.MkdirTemp(parent, ".golc-backup-")
		if err != nil {
			return fmt.Errorf("BOOTSTRAP_PROMOTE: %w", err)
		}
		if err := os.Remove(placeholder); err != nil {
			return fmt.Errorf("BOOTSTRAP_PROMOTE: %w", err)
		}
		backup = placeholder
		if err := os.Rename(installDir, backup); err != nil {
			return fmt.Errorf("BOOTSTRAP_PROMOTE: %w", err)
		}
	}
	if err := os.Rename(stagingDir, installDir); err != nil {
		if backup != "" {
			_ = os.Rename(backup, installDir)
		}
		return fmt.Errorf("BOOTSTRAP_PROMOTE: %w", err)
	}
	if backup != "" {
		if err := os.RemoveAll(backup); err != nil {
			return fmt.Errorf("BOOTSTRAP_PROMOTE_CLEANUP: %w", err)
		}
	}
	return nil
}
