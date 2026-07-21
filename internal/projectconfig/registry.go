// registry.go is the one-owner canonical key/reference graph for
// five-layer configuration resolution (CONTEXT D-06/D-07): every canonical
// key ResolveAll/ResolveKey/ExplainRecord can resolve is declared exactly
// once here with its locked/writable disposition, its closed value shape,
// and the one allowlisted environment variable name it may be overridden
// from. A key with an empty EnvVar is not reachable from the environment
// layer at all: resolve.go never scans os.Environ() broadly, it only
// consults the exact allowlisted name a FieldSpec declares.
//
// This registry does not re-own the committed concern authority Plan 03's
// model.go/decode.go already established (Spec/DefaultSpec is the single
// authority for concern ownership and cross-concern "ref:" resolution).
// registry.go adds only the override-eligibility metadata the four
// higher-precedence layers (user, project-local, environment, CLI) need,
// keyed off the same dotted canonical-key grammar.
package projectconfig

import "fmt"

// FieldSpec declares one canonical key's five-layer override eligibility
// and safe-disclosure rule.
type FieldSpec struct {
	// Locked keys always resolve to their committed value. Any higher
	// layer that declares a value for a locked key fails resolution with
	// GOLC_CONFIG_LOCKED_OVERRIDE instead of being silently ignored
	// (T-01-07: locked classes reject every higher-layer attempt).
	Locked bool
	// Sensitive keys render only "<set>" or "<unset>" through
	// ExplainRecord for both the winning value and every shadowed origin;
	// the literal value is still used for resolution itself (T-01-09).
	Sensitive bool
	// AllowedValues is the closed value set every higher-layer override
	// (and the committed layer, by construction of DefaultSpec) must
	// satisfy. An empty set accepts any string.
	AllowedValues []string
	// EnvVar is the single allowlisted environment variable name
	// consulted for this key. Empty means the key is not reachable from
	// the environment layer at all.
	EnvVar string
	// CLIFlag documents the typed CLI override name a future flag parser
	// binds to this key. ResolveAll/ResolveKey never parse raw argv
	// themselves; they read the already-typed Sources.CLIOverrides map
	// keyed by canonical key, so this field is informational only.
	CLIFlag string
}

// Registry is the one-owner canonical key/reference graph: every key this
// package resolves across five layers is declared exactly once, so no two
// layers can disagree about whether a key is locked, sensitive, or which
// external name may supply it.
type Registry struct {
	Fields map[string]FieldSpec
}

// field looks up one canonical key's FieldSpec, failing with a stable
// diagnostic when the registry does not declare the key at all.
func (r Registry) field(key string) (FieldSpec, error) {
	spec, known := r.Fields[key]
	if !known {
		return FieldSpec{}, fmt.Errorf("GOLC_CONFIG_FIELD_UNKNOWN: %q is not declared by the registry", key)
	}
	return spec, nil
}

// DefaultRegistry is the production five-layer override registry. It keeps
// exactly the same single writable canonical key the project-local layer
// already allows (local.go's localKeyRegistry): runtime.log_level. Every
// other canonical key DefaultSpec declares — pins, hashes, schema
// versions, and identity/path values — is locked: those values must never
// vary between the committed checkout and any contributor's machine or
// invocation.
func DefaultRegistry() Registry {
	fields := map[string]FieldSpec{
		"runtime.log_level": {
			Locked:        false,
			AllowedValues: []string{"debug", "error", "info", "warn"},
			EnvVar:        "GOLC_RUNTIME_LOG_LEVEL",
			CLIFlag:       "--log-level",
		},
	}
	for _, concern := range DefaultSpec().Concerns {
		for key := range concern.Keys {
			if _, declared := fields[key]; declared {
				continue
			}
			fields[key] = FieldSpec{Locked: true}
		}
	}
	// schema_version never appears as a flattened concern key (decode.go
	// strips it before flattening), but a higher layer or CLI override
	// could still name it directly; it must be rejected the same way.
	fields["schema_version"] = FieldSpec{Locked: true}
	return Registry{Fields: fields}
}
