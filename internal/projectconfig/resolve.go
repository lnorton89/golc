// resolve.go implements the five-layer configuration resolution CONTEXT
// D-06/D-07 requires: committed, then user (%APPDATA%\GOLC\config.toml),
// then untracked project-local (golc.local.toml), then an explicit
// environment allowlist, then typed CLI overrides. Every layer above
// committed is optional; the highest-precedence layer that declares a
// value wins, and every other declared layer is reported, in descending
// precedence order, as a safe shadowed origin.
//
// Locked keys (registry.go's FieldSpec.Locked) reject every higher-layer
// attempt outright: resolution fails with GOLC_CONFIG_LOCKED_OVERRIDE
// rather than silently ignoring the attempt (T-01-07). Sensitive keys
// never leak a literal value through ExplainRecord (T-01-09).
//
// This file adds a new five-layer surface; it does not replace the
// existing two-layer ResolveRuntime/Explain in local.go, which the
// committed "config explain" CLI route already serves and this plan does
// not touch. It reuses local.go's readLocalValues, decode.go's
// validateLiteral, and local.go's resolveCommittedOrigin directly (same
// package) instead of re-implementing committed/project-local parsing.
package projectconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// Five-layer precedence names (D-06), in ascending precedence order.
const (
	LayerCommitted    = "committed"
	LayerUser         = "user"
	LayerProjectLocal = "project-local"
	LayerEnvironment  = "environment"
	LayerCLI          = "cli"
)

// userConfigFileName is the fixed leaf name under the user configuration
// directory (D-06: the user layer has one fixed destination, mirroring
// golc.local.toml's fixed project-local destination).
const userConfigFileName = "config.toml"

// Sources bundles the inputs for the four higher-precedence layers plus
// the repository root. Production callers use NewSources, which reads the
// real %APPDATA%\GOLC\config.toml and the real process environment; tests
// construct Sources directly so every layer is independently injectable
// without touching the filesystem or environment beyond an explicit
// temporary root.
type Sources struct {
	// Root is the repository root the committed and project-local layers
	// resolve against.
	Root string
	// UserConfigPath overrides the resolved user-layer file path. Empty
	// resolves to the production %APPDATA%\GOLC\config.toml path; an
	// unresolvable production path (no %APPDATA%) behaves like a missing
	// file, never an error.
	UserConfigPath string
	// LookupEnv overrides the environment lookup function used for the
	// environment layer. Nil defaults to os.LookupEnv. Only the exact
	// allowlisted name a FieldSpec declares is ever consulted — there is
	// no broad os.Environ() scan anywhere in this package.
	LookupEnv func(string) (string, bool)
	// CLIOverrides is the already-typed, already-parsed CLI override
	// input keyed by canonical key. This package never parses raw argv;
	// callers that add a CLI flag surface build this map themselves.
	CLIOverrides map[string]string
}

// NewSources builds the production Sources for root: the real user-layer
// file, the real process environment, and no CLI overrides (a caller that
// adds CLI flag parsing sets CLIOverrides directly on the returned value).
func NewSources(root string) Sources {
	return Sources{Root: root}
}

func (s Sources) userConfigPath() string {
	if s.UserConfigPath != "" {
		return s.UserConfigPath
	}
	return defaultUserConfigPath()
}

func (s Sources) lookupEnv() func(string) (string, bool) {
	if s.LookupEnv != nil {
		return s.LookupEnv
	}
	return os.LookupEnv
}

// defaultUserConfigPath resolves the fixed per-user configuration
// destination (D-06). v1 targets Windows only, so %APPDATA% is the only
// supported location; an unset %APPDATA% means the user layer has nothing
// to read, exactly like a missing file.
func defaultUserConfigPath() string {
	appData := strings.TrimSpace(os.Getenv("APPDATA"))
	if appData == "" {
		return ""
	}
	return filepath.Join(appData, "GOLC", userConfigFileName)
}

// ResolvedRecord is the outcome of five-layer resolution for one canonical
// key: the winning value plus provenance, and every other declared layer
// as an ordered shadowed origin (highest precedence first, committed
// last).
type ResolvedRecord struct {
	Key      string
	Value    string
	Layer    string
	Source   string
	Shadowed []Origin
}

// validateFieldValue enforces one FieldSpec's closed value set against an
// externally supplied override (environment or CLI); user/project-local
// values are already validated while their documents are read.
func validateFieldValue(key, value, origin string, spec FieldSpec) error {
	return validateLiteral(key, value, origin, KeySpec{AllowedValues: spec.AllowedValues})
}

// readUserValues strictly loads the user configuration layer. A missing
// or unresolvable path is an empty layer — the user layer is always
// optional, never an error on its own. Declared keys must be registered,
// unlocked, and satisfy their FieldSpec's allowed value set; an unknown or
// locked declaration fails the whole layer read, mirroring the strictness
// golc.local.toml already enforces (D-09).
func readUserValues(path string, registry Registry) (map[string]string, error) {
	values := map[string]string{}
	if strings.TrimSpace(path) == "" {
		return values, nil
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return values, nil
		}
		return nil, fmt.Errorf("GOLC_CONFIG_USER_READ: %s: %v", path, err)
	}

	document := map[string]any{}
	if _, err := toml.DecodeFile(path, &document); err != nil {
		return nil, fmt.Errorf("GOLC_CONFIG_PARSE: %s: %v", path, err)
	}
	schemaVersion, declared := document["schema_version"]
	version, isInteger := schemaVersion.(int64)
	if !declared || !isInteger || version != supportedSchemaVersion {
		return nil, fmt.Errorf(
			"GOLC_CONFIG_SCHEMA_VERSION: %s must declare schema_version = %d", path, supportedSchemaVersion)
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
		if !canonicalLocalKeyPattern.MatchString(key) {
			return nil, fmt.Errorf("GOLC_CONFIG_USER_KEY_REDIRECT: %q is not a canonical configuration key", key)
		}
		spec, known := registry.Fields[key]
		if !known {
			return nil, fmt.Errorf("GOLC_CONFIG_USER_KEY_UNKNOWN: %s declares %q", path, key)
		}
		if spec.Locked {
			return nil, fmt.Errorf("GOLC_CONFIG_LOCKED_OVERRIDE: user layer cannot override locked key %q", key)
		}
		text, isString := flattened[key].(string)
		if !isString {
			return nil, fmt.Errorf("GOLC_CONFIG_USER_VALUE_INVALID: %s value for %q must be a string", path, key)
		}
		if err := validateFieldValue(key, text, path, spec); err != nil {
			return nil, err
		}
		values[key] = text
	}
	return values, nil
}

// layerCandidate is one declared layer value pending precedence ordering.
type layerCandidate struct {
	layer  string
	origin Origin
}

// resolveOne resolves one canonical key across all five layers.
func resolveOne(registry Registry, sources Sources, key string) (ResolvedRecord, error) {
	spec, err := registry.field(key)
	if err != nil {
		return ResolvedRecord{}, err
	}

	committed, err := resolveCommittedOrigin(sources.Root, key)
	if err != nil {
		return ResolvedRecord{}, err
	}

	resolvedRoot, err := filepath.EvalSymlinks(sources.Root)
	if err != nil {
		return ResolvedRecord{}, fmt.Errorf("GOLC_CONFIG_ROOT_MISSING: %q: %v", sources.Root, err)
	}

	userValues, err := readUserValues(sources.userConfigPath(), registry)
	if err != nil {
		return ResolvedRecord{}, err
	}
	localValues, err := readLocalValues(resolvedRoot)
	if err != nil {
		return ResolvedRecord{}, err
	}

	// Layers are appended strictly in ascending precedence order (D-06),
	// so the last candidate present after this loop is always the winner
	// regardless of which subset of layers actually declared the key.
	candidates := []layerCandidate{}

	if value, present := userValues[key]; present {
		if spec.Locked {
			return ResolvedRecord{}, fmt.Errorf("GOLC_CONFIG_LOCKED_OVERRIDE: user layer cannot override locked key %q", key)
		}
		candidates = append(candidates, layerCandidate{
			layer:  LayerUser,
			origin: Origin{Layer: LayerUser, Source: userConfigFileName, Value: value},
		})
	}

	if value, present := localValues[key]; present {
		if spec.Locked {
			return ResolvedRecord{}, fmt.Errorf("GOLC_CONFIG_LOCKED_OVERRIDE: project-local layer cannot override locked key %q", key)
		}
		candidates = append(candidates, layerCandidate{
			layer:  LayerProjectLocal,
			origin: Origin{Layer: LayerProjectLocal, Source: localFileName, Value: value},
		})
	}

	if spec.EnvVar != "" {
		if value, present := sources.lookupEnv()(spec.EnvVar); present {
			if spec.Locked {
				return ResolvedRecord{}, fmt.Errorf("GOLC_CONFIG_LOCKED_OVERRIDE: environment layer cannot override locked key %q", key)
			}
			if err := validateFieldValue(key, value, "environment:"+spec.EnvVar, spec); err != nil {
				return ResolvedRecord{}, err
			}
			candidates = append(candidates, layerCandidate{
				layer:  LayerEnvironment,
				origin: Origin{Layer: LayerEnvironment, Source: spec.EnvVar, Value: value},
			})
		}
	}

	if sources.CLIOverrides != nil {
		if value, present := sources.CLIOverrides[key]; present {
			if spec.Locked {
				return ResolvedRecord{}, fmt.Errorf("GOLC_CONFIG_LOCKED_OVERRIDE: cli layer cannot override locked key %q", key)
			}
			if err := validateFieldValue(key, value, "cli", spec); err != nil {
				return ResolvedRecord{}, err
			}
			candidates = append(candidates, layerCandidate{
				layer:  LayerCLI,
				origin: Origin{Layer: LayerCLI, Source: "cli", Value: value},
			})
		}
	}

	if len(candidates) == 0 {
		return ResolvedRecord{
			Key: key, Value: committed.Value, Layer: committed.Layer, Source: committed.Source, Shadowed: []Origin{},
		}, nil
	}

	winner := candidates[len(candidates)-1]
	shadowed := make([]Origin, 0, len(candidates))
	for i := len(candidates) - 2; i >= 0; i-- {
		shadowed = append(shadowed, candidates[i].origin)
	}
	shadowed = append(shadowed, committed)

	return ResolvedRecord{
		Key:      key,
		Value:    winner.origin.Value,
		Layer:    winner.layer,
		Source:   winner.origin.Source,
		Shadowed: shadowed,
	}, nil
}

// ResolveKey resolves one canonical key across all five layers in
// committed -> user -> project-local -> environment -> CLI order (D-06).
func ResolveKey(registry Registry, sources Sources, key string) (ResolvedRecord, error) {
	return resolveOne(registry, sources, key)
}

// ResolveAll resolves every canonical key the registry declares.
func ResolveAll(registry Registry, sources Sources) (map[string]ResolvedRecord, error) {
	records := map[string]ResolvedRecord{}
	for key := range registry.Fields {
		record, err := resolveOne(registry, sources, key)
		if err != nil {
			return nil, err
		}
		records[key] = record
	}
	return records, nil
}

// sensitiveDisclosure renders a sensitive value as only "<set>" or
// "<unset>" (D-07/T-01-09): the literal value is never included.
func sensitiveDisclosure(value string) string {
	if strings.TrimSpace(value) == "" {
		return "<unset>"
	}
	return "<set>"
}

// ExplainRecord renders one resolved record as deterministic safe
// provenance JSON: allowlisted fields only, sorted map keys via
// encoding/json, and a single trailing newline. When spec.Sensitive is
// true, both the winning value and every shadowed origin's value render
// as "<set>"/"<unset>" instead of the literal, so a sensitive value can
// never leak through any layer's provenance.
func ExplainRecord(record ResolvedRecord, spec FieldSpec) ([]byte, error) {
	value := record.Value
	shadowed := make([]Origin, 0, len(record.Shadowed))
	for _, origin := range record.Shadowed {
		if spec.Sensitive {
			origin.Value = sensitiveDisclosure(origin.Value)
		}
		shadowed = append(shadowed, origin)
	}
	if spec.Sensitive {
		value = sensitiveDisclosure(value)
	}

	document := map[string]any{
		"key":      record.Key,
		"layer":    record.Layer,
		"source":   record.Source,
		"value":    value,
		"shadowed": shadowed,
	}
	payload, err := json.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("GOLC_CONFIG_ENCODE: %s: %v", record.Key, err)
	}
	return append(payload, '\n'), nil
}
