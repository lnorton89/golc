// local.go implements the machine-local configuration layer (CONTEXT
// D-06/D-07): WriteLocal persists allowlisted values only to the fixed
// repository-root golc.local.toml through contained atomic replacement,
// ResolveRuntime resolves a key across the committed and project-local
// layers, and Explain renders deterministic safe provenance JSON.
//
// The local file is machine state, never committed (D-05): .gitignore owns
// that boundary. Provenance intentionally exposes only the canonical key,
// the non-sensitive value, the winning layer, the safe source name, and
// ordered shadowed origins — never environment or credentials.
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

// localFileName is the only file WriteLocal may create or replace (D-06:
// the untracked project-local layer has one fixed destination).
const localFileName = "golc.local.toml"

// Layer names reported by provenance, ordered per D-06 precedence.
const (
	layerCommitted    = "committed"
	layerProjectLocal = "project-local"
)

// localKeySpec describes one canonical configuration key this plan knows.
type localKeySpec struct {
	// writable marks keys the project-local layer may override. Locked
	// keys (pins, hashes, schema versions, identity) always reject
	// overrides (01-PATTERNS resolution rules).
	writable bool
	// allowedValues is the closed value set for a writable key.
	allowedValues []string
}

// localKeyRegistry is the fixed allowlist of canonical keys. Everything
// else is unknown and rejected (D-09 strictness).
var localKeyRegistry = map[string]localKeySpec{
	"runtime.log_level": {
		writable:      true,
		allowedValues: []string{"debug", "error", "info", "warn"},
	},
	"schema_version":              {writable: false},
	"toolchain.go.version":        {writable: false},
	"toolchain.go.archive_url":    {writable: false},
	"toolchain.go.archive_sha256": {writable: false},
}

// canonicalLocalKeyPattern is the only accepted key shape: dotted
// lowercase words. Path separators, dot-dot, leading dots (.env), and
// every other redirection shape fail before any registry lookup.
var canonicalLocalKeyPattern = regexp.MustCompile(`^[a-z0-9_]+(\.[a-z0-9_]+)*$`)

// Origin is one shadowed provenance entry. Field order is alphabetical so
// the marshaled JSON keys are sorted like the rest of the document.
type Origin struct {
	Layer  string `json:"layer"`
	Source string `json:"source"`
	Value  string `json:"value"`
}

// ResolvedValue is the outcome of resolving one canonical key across the
// committed and project-local layers.
type ResolvedValue struct {
	Key      string
	Value    string
	Layer    string
	Source   string
	Shadowed []Origin
}

// assertWritableLocalKey applies the redirect, unknown, and locked gates
// in that order so each failure mode has one stable diagnostic.
func assertWritableLocalKey(key string) (localKeySpec, error) {
	if !canonicalLocalKeyPattern.MatchString(key) {
		return localKeySpec{}, fmt.Errorf(
			"GOLC_CONFIG_LOCAL_KEY_REDIRECT: %q is not a canonical configuration key; path-like or .env targets are rejected", key)
	}
	spec, known := localKeyRegistry[key]
	if !known {
		return localKeySpec{}, fmt.Errorf("GOLC_CONFIG_LOCAL_KEY_UNKNOWN: %q", key)
	}
	if !spec.writable {
		return localKeySpec{}, fmt.Errorf("GOLC_CONFIG_LOCAL_KEY_LOCKED: %q rejects local overrides", key)
	}
	return spec, nil
}

// assertAllowedLocalValue enforces the closed value set of a writable key.
func assertAllowedLocalValue(key, value string, spec localKeySpec) error {
	for _, allowed := range spec.allowedValues {
		if value == allowed {
			return nil
		}
	}
	return fmt.Errorf(
		"GOLC_CONFIG_LOCAL_VALUE_INVALID: %q is not an allowed value for %s (allowed: %s)",
		value, key, strings.Join(spec.allowedValues, ", "))
}

// resolveLocalDestination resolves the fixed local file path inside the
// resolved repository root and rejects symlinked or otherwise redirected
// destinations before any byte is written (T-01-04 containment).
func resolveLocalDestination(root string) (string, error) {
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("GOLC_CONFIG_ROOT_MISSING: %q: %v", root, err)
	}
	destination := filepath.Join(resolvedRoot, localFileName)
	info, err := os.Lstat(destination)
	if err != nil {
		if os.IsNotExist(err) {
			return destination, nil
		}
		return "", fmt.Errorf("GOLC_CONFIG_LOCAL_PATH_ESCAPE: %s: %v", localFileName, err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf(
			"GOLC_CONFIG_LOCAL_PATH_ESCAPE: %s must be a regular file inside the repository, not a link or reparse point", localFileName)
	}
	return destination, nil
}

// WriteLocal persists one allowlisted key/value pair to the fixed
// repository-root golc.local.toml. Existing local values are preserved,
// the document is rendered deterministically, and the file is replaced
// atomically through a contained temporary file plus rename.
func WriteLocal(root, key, value string) error {
	spec, err := assertWritableLocalKey(key)
	if err != nil {
		return err
	}
	if err := assertAllowedLocalValue(key, value, spec); err != nil {
		return err
	}

	destination, err := resolveLocalDestination(root)
	if err != nil {
		return err
	}
	values, err := readLocalValues(filepath.Dir(destination))
	if err != nil {
		return err
	}
	values[key] = value

	payload := renderLocalDocument(values)
	temporary, err := os.CreateTemp(filepath.Dir(destination), localFileName+".tmp-*")
	if err != nil {
		return fmt.Errorf("GOLC_CONFIG_LOCAL_WRITE: staging %s: %v", localFileName, err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	if _, err := temporary.Write([]byte(payload)); err != nil {
		temporary.Close()
		return fmt.Errorf("GOLC_CONFIG_LOCAL_WRITE: staging %s: %v", localFileName, err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("GOLC_CONFIG_LOCAL_WRITE: staging %s: %v", localFileName, err)
	}
	if err := os.Rename(temporaryPath, destination); err != nil {
		return fmt.Errorf("GOLC_CONFIG_LOCAL_WRITE: atomic replacement of %s: %v", localFileName, err)
	}
	return nil
}

// renderLocalDocument renders the local values as deterministic TOML:
// fixed header, schema_version, then sorted tables with sorted keys.
func renderLocalDocument(values map[string]string) string {
	tables := map[string]map[string]string{}
	for key, value := range values {
		lastDot := strings.LastIndex(key, ".")
		table := key[:lastDot]
		leaf := key[lastDot+1:]
		if tables[table] == nil {
			tables[table] = map[string]string{}
		}
		tables[table][leaf] = value
	}
	tableNames := make([]string, 0, len(tables))
	for name := range tables {
		tableNames = append(tableNames, name)
	}
	sort.Strings(tableNames)

	var builder strings.Builder
	builder.WriteString("# Machine-local GOLC configuration overrides (CONTEXT D-06).\n")
	builder.WriteString("# Written by 'golc config set --local'; ignored by git; never committed.\n\n")
	builder.WriteString("schema_version = 1\n")
	for _, name := range tableNames {
		builder.WriteString("\n[" + name + "]\n")
		leaves := make([]string, 0, len(tables[name]))
		for leaf := range tables[name] {
			leaves = append(leaves, leaf)
		}
		sort.Strings(leaves)
		for _, leaf := range leaves {
			builder.WriteString(fmt.Sprintf("%s = %q\n", leaf, tables[name][leaf]))
		}
	}
	return builder.String()
}

// readLocalValues strictly loads golc.local.toml from the resolved root:
// a missing file is an empty layer; unknown keys, locked keys, wrong
// schema versions, and non-string values fail with stable diagnostics.
func readLocalValues(resolvedRoot string) (map[string]string, error) {
	values := map[string]string{}
	localPath := filepath.Join(resolvedRoot, localFileName)
	if _, err := os.Stat(localPath); err != nil {
		if os.IsNotExist(err) {
			return values, nil
		}
		return nil, fmt.Errorf("GOLC_CONFIG_LOCAL_READ: %s: %v", localFileName, err)
	}

	document := map[string]any{}
	if _, err := toml.DecodeFile(localPath, &document); err != nil {
		return nil, fmt.Errorf("GOLC_CONFIG_PARSE: %s: %v", localFileName, err)
	}
	schemaVersion, declared := document["schema_version"]
	version, isInteger := schemaVersion.(int64)
	if !declared || !isInteger || version != supportedSchemaVersion {
		return nil, fmt.Errorf(
			"GOLC_CONFIG_SCHEMA_VERSION: %s must declare schema_version = %d",
			localFileName, supportedSchemaVersion)
	}
	delete(document, "schema_version")

	flattened := map[string]any{}
	flattenLocalDocument("", document, flattened)
	keys := make([]string, 0, len(flattened))
	for key := range flattened {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		spec, known := localKeyRegistry[key]
		if !known {
			return nil, fmt.Errorf("GOLC_CONFIG_LOCAL_KEY_UNKNOWN: %s declares %q", localFileName, key)
		}
		if !spec.writable {
			return nil, fmt.Errorf("GOLC_CONFIG_LOCAL_KEY_LOCKED: %s declares %q which rejects local overrides", localFileName, key)
		}
		text, isString := flattened[key].(string)
		if !isString {
			return nil, fmt.Errorf("GOLC_CONFIG_LOCAL_VALUE_INVALID: %s value for %q must be a string", localFileName, key)
		}
		if err := assertAllowedLocalValue(key, text, spec); err != nil {
			return nil, err
		}
		values[key] = text
	}
	return values, nil
}

// flattenLocalDocument converts nested TOML tables into dotted keys.
func flattenLocalDocument(prefix string, document map[string]any, into map[string]any) {
	for key, value := range document {
		dotted := key
		if prefix != "" {
			dotted = prefix + "." + key
		}
		if nested, isTable := value.(map[string]any); isTable {
			flattenLocalDocument(dotted, nested, into)
			continue
		}
		into[dotted] = value
	}
}

// resolveCommittedOrigin reads the committed authority for one canonical
// key through the strict root index (D-05): the key's first segment names
// the owning concern.
func resolveCommittedOrigin(root, key string) (Origin, error) {
	concernID := strings.SplitN(key, ".", 2)[0]
	index, err := LoadRootIndex(root)
	if err != nil {
		return Origin{}, err
	}
	var found *Concern
	for i := range index.Concerns {
		if index.Concerns[i].ID == concernID {
			found = &index.Concerns[i]
			break
		}
	}
	if found == nil {
		return Origin{}, fmt.Errorf("GOLC_CONFIG_CONCERN_UNKNOWN: %q is not in %s", concernID, rootIndexName)
	}
	concernPath, err := resolveContainedConcernFile(root, found.Path)
	if err != nil {
		return Origin{}, err
	}

	document := map[string]any{}
	if _, err := toml.DecodeFile(concernPath, &document); err != nil {
		return Origin{}, fmt.Errorf("GOLC_CONFIG_PARSE: %s: %v", found.Path, err)
	}
	delete(document, "schema_version")
	flattened := map[string]any{}
	flattenLocalDocument("", document, flattened)
	raw, declared := flattened[key]
	if !declared {
		return Origin{}, fmt.Errorf("GOLC_CONFIG_COMMITTED_MISSING: %s does not declare %q", found.Path, key)
	}
	text, isString := raw.(string)
	if !isString {
		return Origin{}, fmt.Errorf("GOLC_CONFIG_LOCAL_VALUE_INVALID: %s value for %q must be a string", found.Path, key)
	}
	return Origin{Layer: layerCommitted, Source: found.Path, Value: text}, nil
}

// ResolveRuntime resolves one canonical key across the two layers this
// plan implements — committed defaults, then the untracked project-local
// file — and reports the winning layer, safe source, and ordered shadowed
// origins (D-06/D-07).
func ResolveRuntime(root, key string) (ResolvedValue, error) {
	if _, err := assertWritableLocalKey(key); err != nil {
		return ResolvedValue{}, err
	}
	committed, err := resolveCommittedOrigin(root, key)
	if err != nil {
		return ResolvedValue{}, err
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return ResolvedValue{}, fmt.Errorf("GOLC_CONFIG_ROOT_MISSING: %q: %v", root, err)
	}
	local, err := readLocalValues(resolvedRoot)
	if err != nil {
		return ResolvedValue{}, err
	}
	if value, declared := local[key]; declared {
		return ResolvedValue{
			Key:      key,
			Value:    value,
			Layer:    layerProjectLocal,
			Source:   localFileName,
			Shadowed: []Origin{committed},
		}, nil
	}
	return ResolvedValue{
		Key:      key,
		Value:    committed.Value,
		Layer:    committed.Layer,
		Source:   committed.Source,
		Shadowed: []Origin{},
	}, nil
}

// Explain renders deterministic safe provenance JSON for one canonical
// key: allowlisted fields only, sorted keys via encoding/json map
// marshaling, and a single trailing newline. runtime.log_level is
// non-sensitive, so its value is included; environment and credentials
// are never serialized (T-01-05).
func Explain(root, key string) ([]byte, error) {
	resolved, err := ResolveRuntime(root, key)
	if err != nil {
		return nil, err
	}
	document := map[string]any{
		"key":      resolved.Key,
		"layer":    resolved.Layer,
		"source":   resolved.Source,
		"value":    resolved.Value,
		"shadowed": resolved.Shadowed,
	}
	payload, err := json.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("GOLC_CONFIG_ENCODE: %s: %v", key, err)
	}
	return append(payload, '\n'), nil
}
