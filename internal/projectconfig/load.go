// Package projectconfig loads the committed GOLC root configuration index
// (golc.project.toml) and its concern files with strict validation and
// repository containment (CONTEXT D-05). The root index owns only schema
// and index metadata; each concern file alone owns its values.
//
// This file is also the root config-inspect command file: it self-registers
// the exact route "config inspect" through the command package's
// declaration entrypoint (D-03), so no central router file is edited to
// make it reachable.
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

	"github.com/lnorton89/golc/internal/command"
)

// rootIndexName is the fixed repository-root configuration index (D-05).
const rootIndexName = "golc.project.toml"

// supportedSchemaVersion is the only root/concern schema this build reads.
const supportedSchemaVersion = 1

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

var _ = command.MustDeclareScope(command.ScopeRegistration{
	Scope:   "config",
	Summary: "Project configuration index and concern operations.",
})

var _ = command.MustDeclareRoute(command.CommandRegistration{
	Route:   "config inspect",
	Summary: "Print one indexed configuration concern as deterministic JSON.",
	Handler: runConfigInspect,
})

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

// runConfigInspect serves the self-registered "config inspect" route.
func runConfigInspect(request command.Request) command.Result {
	concernID, err := parseInspectArgs(request.Args)
	if err != nil {
		return command.Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}
	payload, err := InspectConcern(request.Root, concernID)
	if err != nil {
		return command.Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	return command.Result{Stdout: payload}
}

// parseInspectArgs accepts exactly one concern id plus an optional
// "--format json" (the only supported format).
func parseInspectArgs(args []string) (string, error) {
	concernID := ""
	format := "json"
	for i := 0; i < len(args); {
		argument := args[i]
		switch {
		case argument == "--format":
			if i+1 >= len(args) {
				return "", fmt.Errorf("GOLC_CONFIG_USAGE: --format requires a value")
			}
			format = args[i+1]
			i += 2
		case strings.HasPrefix(argument, "--format="):
			format = strings.TrimPrefix(argument, "--format=")
			i++
		case strings.HasPrefix(argument, "-"):
			return "", fmt.Errorf("GOLC_CONFIG_USAGE: unknown flag %q", argument)
		default:
			if concernID != "" {
				return "", fmt.Errorf("GOLC_CONFIG_USAGE: exactly one concern id is required")
			}
			concernID = argument
			i++
		}
	}
	if concernID == "" {
		return "", fmt.Errorf("GOLC_CONFIG_USAGE: usage: config inspect <concern> [--format json]")
	}
	if format != "json" {
		return "", fmt.Errorf("GOLC_CONFIG_FORMAT_UNSUPPORTED: %q (only json is supported)", format)
	}
	return concernID, nil
}
