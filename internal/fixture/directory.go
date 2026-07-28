// directory.go implements the single local-fixture-directory scan
// (extracted verbatim from internal/command/artnet.go's now-superseded
// private loadFixtureDirectory loop, CONTEXT D-01's local half):
// ListDirectory non-recursively reads every *.yaml/*.yml file directly
// inside dir through the exact same Decode/Pin pipeline "fixture
// validate"/"fixture inspect" already use. A per-entry read/decode/pin
// failure is recorded on that entry's own Err and the scan continues --
// one malformed file never aborts the whole listing (T-09-01-02). Only
// the top-level os.ReadDir failure returns a non-nil error, wrapped under
// GOLC_FIXTURE_DIR_READ_FAILED so a caller can distinguish "directory
// does not exist yet" (errors.Is(err, fs.ErrNotExist)) from a genuine
// read failure -- internal/wails.FixtureLibraryService.ListLocal relies
// on exactly this distinction to treat a not-yet-created fixtures
// directory as an empty library, not a broken one.
//
// internal/command/artnet.go's loadFixtureDirectory and
// internal/wails/svc_fixturelibrary.go's ListLocal both call this
// function -- there is exactly one directory-scan implementation in this
// repository after this file's introduction.
package fixture

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DirectoryEntry is one file ListDirectory found directly inside a
// scanned directory. FileName is the bare file name (never a path).
// Definition/Identity are populated only when Err is nil; Err carries
// that entry's own read/decode/pin failure without aborting the rest of
// the scan.
type DirectoryEntry struct {
	FileName   string
	Definition FixtureDefinition
	Identity   Identity
	Err        error
}

// ListDirectory reads dir non-recursively via os.ReadDir, skipping
// subdirectories and any file whose lowercased extension is not .yaml or
// .yml. Each remaining file is read, decoded (Decode), then pinned
// (Pin); a per-entry failure at any of those three steps is recorded on
// that entry's Err field and the scan continues onto the next file.
// Only the top-level os.ReadDir failure returns a non-nil error. Entry
// order is os.ReadDir's own deterministic filename order.
func ListDirectory(dir string) ([]DirectoryEntry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("GOLC_FIXTURE_DIR_READ_FAILED: %w", err)
	}

	results := make([]DirectoryEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		result := DirectoryEntry{FileName: entry.Name()}
		data, readErr := os.ReadFile(filepath.Join(dir, entry.Name()))
		if readErr != nil {
			result.Err = readErr
			results = append(results, result)
			continue
		}
		def, decodeErr := Decode(data)
		if decodeErr != nil {
			result.Err = decodeErr
			results = append(results, result)
			continue
		}
		identity, pinErr := Pin(def)
		if pinErr != nil {
			result.Err = pinErr
			results = append(results, result)
			continue
		}
		result.Definition = def
		result.Identity = identity
		results = append(results, result)
	}
	return results, nil
}
