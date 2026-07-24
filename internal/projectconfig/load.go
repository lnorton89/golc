// Package projectconfig loads the committed GOLC root configuration index
// (golc.project.toml) and its concern files with strict validation and
// repository containment (CONTEXT D-05). The root index owns only schema
// and index metadata; each concern file alone owns its values.
//
// This package is a pure configuration library: the config command routes
// that expose it self-register from internal/command/config.go (D-03), so
// projectconfig never imports the command package and command handlers can
// call into it without an import cycle.
package projectconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// rootIndexName is the fixed repository-root configuration index (D-05).
const rootIndexName = "golc.project.toml"

// supportedSchemaVersion is the only root/concern schema this build reads.
const supportedSchemaVersion = 2

// Concern is one logically separated configuration file indexed by the
// root manifest.
type Concern struct {
	ID   string `toml:"id"`
	Path string `toml:"path"`
}

// RootIndex is the strict shape of golc.project.toml: schema and index
// metadata only, never configuration values.
type RootIndex struct {
	SchemaVersion int       `toml:"schema_version"`
	Concerns      []Concern `toml:"concerns"`
}

var concernIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// LoadRootIndex reads and strictly validates the repository root index:
// unknown keys, unsupported schema versions, invalid or duplicate concern
// ids, and non-contained concern paths all fail with stable diagnostics.
func LoadRootIndex(root string) (RootIndex, error) {
	indexPath := filepath.Join(root, rootIndexName)
	if _, err := os.Stat(indexPath); err != nil {
		return RootIndex{}, fmt.Errorf("GOLC_CONFIG_ROOT_INDEX_MISSING: %s: %v", rootIndexName, err)
	}

	var index RootIndex
	metadata, err := toml.DecodeFile(indexPath, &index)
	if err != nil {
		return RootIndex{}, fmt.Errorf("GOLC_CONFIG_PARSE: %s: %v", rootIndexName, err)
	}
	if undecoded := metadata.Undecoded(); len(undecoded) > 0 {
		keys := make([]string, 0, len(undecoded))
		for _, key := range undecoded {
			keys = append(keys, key.String())
		}
		sort.Strings(keys)
		return RootIndex{}, fmt.Errorf("GOLC_CONFIG_UNKNOWN_KEY: %s: %s", rootIndexName, strings.Join(keys, ", "))
	}
	if index.SchemaVersion != supportedSchemaVersion {
		return RootIndex{}, fmt.Errorf(
			"GOLC_CONFIG_SCHEMA_VERSION: %s declares schema_version %d; this build supports %d",
			rootIndexName, index.SchemaVersion, supportedSchemaVersion,
		)
	}

	seen := map[string]struct{}{}
	for _, concern := range index.Concerns {
		if !concernIDPattern.MatchString(concern.ID) {
			return RootIndex{}, fmt.Errorf("GOLC_CONFIG_CONCERN_ID_INVALID: %q", concern.ID)
		}
		if _, duplicate := seen[concern.ID]; duplicate {
			return RootIndex{}, fmt.Errorf("GOLC_CONFIG_CONCERN_DUPLICATE: %s", concern.ID)
		}
		seen[concern.ID] = struct{}{}
		if err := assertRelativeConcernPath(concern.Path); err != nil {
			return RootIndex{}, err
		}
	}
	return index, nil
}

// assertRelativeConcernPath rejects absolute, drive-qualified, parent, and
// dot path segments before any filesystem access happens.
func assertRelativeConcernPath(relative string) error {
	if strings.TrimSpace(relative) == "" {
		return fmt.Errorf("GOLC_CONFIG_PATH_ESCAPE: concern path is empty")
	}
	normalized := strings.ReplaceAll(relative, "\\", "/")
	if strings.HasPrefix(normalized, "/") || strings.Contains(normalized, ":") || filepath.IsAbs(relative) {
		return fmt.Errorf("GOLC_CONFIG_PATH_ESCAPE: %q must be repository-relative", relative)
	}
	for _, segment := range strings.Split(normalized, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return fmt.Errorf("GOLC_CONFIG_PATH_ESCAPE: %q contains a forbidden path segment", relative)
		}
	}
	return nil
}

// resolveContainedConcernFile joins the validated relative path onto the
// repository root and applies the final symlink/reparse containment check:
// the fully resolved file must remain inside the resolved repository root.
func resolveContainedConcernFile(root, relative string) (string, error) {
	if err := assertRelativeConcernPath(relative); err != nil {
		return "", err
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("GOLC_CONFIG_ROOT_MISSING: %q: %v", root, err)
	}
	joined := filepath.Join(resolvedRoot, filepath.FromSlash(strings.ReplaceAll(relative, "\\", "/")))
	resolved, err := filepath.EvalSymlinks(joined)
	if err != nil {
		return "", fmt.Errorf("GOLC_CONFIG_CONCERN_FILE_MISSING: %s: %v", relative, err)
	}
	rootWithSeparator := resolvedRoot + string(os.PathSeparator)
	if resolved != resolvedRoot && !strings.HasPrefix(resolved, rootWithSeparator) {
		return "", fmt.Errorf("GOLC_CONFIG_PATH_ESCAPE: %q resolves outside the repository", relative)
	}
	return resolved, nil
}

// InspectConcern loads one indexed concern file and returns its values as
// deterministic JSON: encoding/json marshals map keys in sorted order, so
// repeated inspection of unchanged input is byte-identical.
func InspectConcern(root, concernID string) ([]byte, error) {
	index, err := LoadRootIndex(root)
	if err != nil {
		return nil, err
	}

	var found *Concern
	for i := range index.Concerns {
		if index.Concerns[i].ID == concernID {
			found = &index.Concerns[i]
			break
		}
	}
	if found == nil {
		return nil, fmt.Errorf("GOLC_CONFIG_CONCERN_UNKNOWN: %q is not in %s", concernID, rootIndexName)
	}

	concernPath, err := resolveContainedConcernFile(root, found.Path)
	if err != nil {
		return nil, err
	}

	document := map[string]any{}
	if _, err := toml.DecodeFile(concernPath, &document); err != nil {
		return nil, fmt.Errorf("GOLC_CONFIG_PARSE: %s: %v", found.Path, err)
	}
	schemaVersion, declared := document["schema_version"]
	version, isInteger := schemaVersion.(int64)
	if !declared || !isInteger || version != supportedSchemaVersion {
		return nil, fmt.Errorf(
			"GOLC_CONFIG_SCHEMA_VERSION: %s must declare schema_version = %d",
			found.Path, supportedSchemaVersion,
		)
	}
	delete(document, "schema_version")

	payload, err := json.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("GOLC_CONFIG_ENCODE: %s: %v", found.Path, err)
	}
	return append(payload, '\n'), nil
}
