// decode.go is the strict concern decoder (CONTEXT D-09/D-10): every
// concern file validates alone, the combined declarations assign each
// canonical key exactly once, and unknown, duplicate, invalid,
// deprecated-only, old-plus-new, duplicate-authority, unresolved, and
// cyclic inputs each fail with a distinct stable diagnostic. Deprecated
// input is never silently rewritten: it applies to its replacement key
// only alongside an explicit CFG_DEPRECATED_KEY warning with migration
// guidance.
package projectconfig

import (
	"fmt"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// refPrefix marks a typed cross-concern reference value: the key adopts
// the referenced single authority's literal instead of repeating it.
const refPrefix = "ref:"

// ValidateAuthority checks a Spec's own registry before any file is read:
// duplicate canonical key ownership across concerns fails with
// GOLC_CONFIG_DUPLICATE_AUTHORITY, and malformed deprecation entries fail
// with GOLC_CONFIG_DEPRECATION_INVALID.
func ValidateAuthority(spec Spec) error {
	owners, err := specKeyOwners(spec)
	if err != nil {
		return err
	}

	seenOldKeys := map[string]struct{}{}
	for _, deprecation := range spec.Deprecations {
		if !canonicalLocalKeyPattern.MatchString(deprecation.OldKey) {
			return fmt.Errorf("GOLC_CONFIG_DEPRECATION_INVALID: old key %q is not a canonical key", deprecation.OldKey)
		}
		if _, duplicate := seenOldKeys[deprecation.OldKey]; duplicate {
			return fmt.Errorf("GOLC_CONFIG_DEPRECATION_INVALID: old key %q is registered twice", deprecation.OldKey)
		}
		seenOldKeys[deprecation.OldKey] = struct{}{}
		if _, stillOwned := owners[deprecation.OldKey]; stillOwned {
			return fmt.Errorf(
				"GOLC_CONFIG_DEPRECATION_INVALID: old key %q is still owned by a concern; a deprecated key cannot stay canonical",
				deprecation.OldKey)
		}
		if _, owned := owners[deprecation.ReplacementKey]; !owned {
			return fmt.Errorf(
				"GOLC_CONFIG_DEPRECATION_INVALID: replacement key %q for %q is not owned by any concern",
				deprecation.ReplacementKey, deprecation.OldKey)
		}
		if strings.TrimSpace(deprecation.IntroducedIn) == "" || strings.TrimSpace(deprecation.DeprecatedIn) == "" {
			return fmt.Errorf(
				"GOLC_CONFIG_DEPRECATION_INVALID: %q must declare introduced and deprecated versions", deprecation.OldKey)
		}
		if strings.TrimSpace(deprecation.Message) == "" {
			return fmt.Errorf(
				"GOLC_CONFIG_DEPRECATION_INVALID: %q must carry a non-empty migration message", deprecation.OldKey)
		}
	}
	return nil
}

// specKeyOwners builds the canonical key -> owning concern id map and
// rejects duplicate concern ids, malformed keys, and duplicate ownership.
func specKeyOwners(spec Spec) (map[string]string, error) {
	owners := map[string]string{}
	seenConcerns := map[string]struct{}{}
	for _, concern := range spec.Concerns {
		if _, duplicate := seenConcerns[concern.ID]; duplicate {
			return nil, fmt.Errorf("GOLC_CONFIG_CONCERN_DUPLICATE: %s", concern.ID)
		}
		seenConcerns[concern.ID] = struct{}{}
		for _, key := range sortedSpecKeys(concern.Keys) {
			if !canonicalLocalKeyPattern.MatchString(key) {
				return nil, fmt.Errorf("GOLC_CONFIG_KEY_INVALID: %q owned by concern %s", key, concern.ID)
			}
			if owner, taken := owners[key]; taken {
				return nil, fmt.Errorf(
					"GOLC_CONFIG_DUPLICATE_AUTHORITY: %q is declared by both %q and %q; every canonical key has one owner",
					key, owner, concern.ID)
			}
			owners[key] = concern.ID
		}
	}
	return owners, nil
}

// sortedSpecKeys returns a concern's canonical keys in stable order.
func sortedSpecKeys(keys map[string]KeySpec) []string {
	names := make([]string, 0, len(keys))
	for name := range keys {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// findConcern resolves one concern id inside a Spec.
func findConcern(spec Spec, concernID string) (ConcernSpec, error) {
	for _, concern := range spec.Concerns {
		if concern.ID == concernID {
			return concern, nil
		}
	}
	return ConcernSpec{}, fmt.Errorf("GOLC_CONFIG_CONCERN_UNKNOWN: %q is not a registered concern", concernID)
}

// decodeStrictConcernDocument reads one concern file with full strictness:
// contained path resolution, duplicate-definition rejection, schema
// version enforcement, and dotted-key flattening.
func decodeStrictConcernDocument(root string, concern ConcernSpec) (map[string]any, error) {
	concernPath, err := resolveContainedConcernFile(root, concern.Path)
	if err != nil {
		return nil, err
	}

	document := map[string]any{}
	if _, err := toml.DecodeFile(concernPath, &document); err != nil {
		if strings.Contains(err.Error(), "already") {
			return nil, fmt.Errorf("GOLC_CONFIG_DUPLICATE_KEY: %s: %v", concern.Path, err)
		}
		return nil, fmt.Errorf("GOLC_CONFIG_PARSE: %s: %v", concern.Path, err)
	}
	schemaVersion, declared := document["schema_version"]
	version, isInteger := schemaVersion.(int64)
	if !declared || !isInteger || version != supportedSchemaVersion {
		return nil, fmt.Errorf(
			"GOLC_CONFIG_SCHEMA_VERSION: %s must declare schema_version = %d",
			concern.Path, supportedSchemaVersion)
	}
	delete(document, "schema_version")

	flattened := map[string]any{}
	flattenLocalDocument("", document, flattened)
	return flattened, nil
}

// validateLiteral enforces one KeySpec against a resolved literal value.
func validateLiteral(key, value, origin string, spec KeySpec) error {
	if len(spec.AllowedValues) > 0 {
		for _, allowed := range spec.AllowedValues {
			if value == allowed {
				return nil
			}
		}
		return fmt.Errorf(
			"GOLC_CONFIG_VALUE_INVALID: %q is not an allowed value for %s in %s (allowed: %s)",
			value, key, origin, strings.Join(spec.AllowedValues, ", "))
	}
	if spec.Pattern != nil && !spec.Pattern.MatchString(value) {
		return fmt.Errorf(
			"GOLC_CONFIG_VALUE_INVALID: %q does not match the required shape for %s in %s",
			value, key, origin)
	}
	return nil
}

// validateDeclaredValue checks one declared value: it must be a TOML
// string, and either a well-formed typed reference (deferred to
// repository-level resolution) or a literal satisfying the KeySpec.
func validateDeclaredValue(key string, raw any, origin string, spec KeySpec) (string, error) {
	value, isString := raw.(string)
	if !isString {
		return "", fmt.Errorf("GOLC_CONFIG_VALUE_INVALID: %s value for %q must be a string", origin, key)
	}
	if strings.HasPrefix(value, refPrefix) {
		target := strings.TrimPrefix(value, refPrefix)
		if !canonicalLocalKeyPattern.MatchString(target) {
			return "", fmt.Errorf(
				"GOLC_CONFIG_VALUE_INVALID: %q is not a canonical reference target for %s in %s", target, key, origin)
		}
		return value, nil
	}
	if err := validateLiteral(key, value, origin, spec); err != nil {
		return "", err
	}
	return value, nil
}

// ValidateConcern strictly decodes one concern file in isolation and
// returns its declared canonical values plus deprecation warnings.
// Deprecated-only input applies to the replacement key with an explicit
// CFG_DEPRECATED_KEY warning; deprecated input beside its replacement
// fails with CFG_DEPRECATED_COLLISION.
func ValidateConcern(root string, spec Spec, concernID string) (map[string]string, []Diagnostic, error) {
	if err := ValidateAuthority(spec); err != nil {
		return nil, nil, err
	}
	concern, err := findConcern(spec, concernID)
	if err != nil {
		return nil, nil, err
	}
	owners, err := specKeyOwners(spec)
	if err != nil {
		return nil, nil, err
	}
	// Deprecated keys recognized in this file: old key -> register entry
	// whose replacement this concern owns.
	recognizedDeprecations := map[string]Deprecation{}
	for _, deprecation := range spec.Deprecations {
		if owners[deprecation.ReplacementKey] == concern.ID {
			recognizedDeprecations[deprecation.OldKey] = deprecation
		}
	}

	flattened, err := decodeStrictConcernDocument(root, concern)
	if err != nil {
		return nil, nil, err
	}
	declaredKeys := make([]string, 0, len(flattened))
	for key := range flattened {
		declaredKeys = append(declaredKeys, key)
	}
	sort.Strings(declaredKeys)

	values := map[string]string{}
	warnings := []Diagnostic{}
	for _, key := range declaredKeys {
		if keySpec, ownedHere := concern.Keys[key]; ownedHere {
			value, err := validateDeclaredValue(key, flattened[key], concern.Path, keySpec)
			if err != nil {
				return nil, nil, err
			}
			values[key] = value
			continue
		}
		if deprecation, recognized := recognizedDeprecations[key]; recognized {
			if _, replacementDeclared := flattened[deprecation.ReplacementKey]; replacementDeclared {
				return nil, nil, fmt.Errorf(
					"CFG_DEPRECATED_COLLISION: %s declares deprecated %q beside its replacement %q; remove the deprecated key",
					concern.Path, key, deprecation.ReplacementKey)
			}
			replacementSpec := concern.Keys[deprecation.ReplacementKey]
			value, err := validateDeclaredValue(deprecation.ReplacementKey, flattened[key], concern.Path, replacementSpec)
			if err != nil {
				return nil, nil, err
			}
			values[deprecation.ReplacementKey] = value
			removal := "no removal scheduled"
			if strings.TrimSpace(deprecation.RemovalPlanned) != "" {
				removal = "removal planned in " + deprecation.RemovalPlanned
			}
			warnings = append(warnings, Diagnostic{
				Code:   "CFG_DEPRECATED_KEY",
				Key:    key,
				Origin: concern.Path,
				Message: fmt.Sprintf(
					"%q (introduced %s) is deprecated since %s (%s); use %q instead: %s",
					key, deprecation.IntroducedIn, deprecation.DeprecatedIn, removal,
					deprecation.ReplacementKey, deprecation.Message),
			})
			continue
		}
		if owner, ownedElsewhere := owners[key]; ownedElsewhere {
			return nil, nil, fmt.Errorf(
				"GOLC_CONFIG_DUPLICATE_AUTHORITY: %s declares %q, which is owned by concern %q; refer with %q instead of repeating it",
				concern.Path, key, owner, refPrefix+key)
		}
		return nil, nil, fmt.Errorf("GOLC_CONFIG_UNKNOWN_KEY: %s declares %q", concern.Path, key)
	}
	return values, warnings, nil
}

// ValidateRepository validates the whole strict configuration set: the
// registry, the root index discovery contract, every concern in
// isolation, and the cross-concern reference graph. It returns the
// combined resolved values (references replaced by their single-authority
// literals) plus all deprecation warnings.
func ValidateRepository(root string, spec Spec) (map[string]string, []Diagnostic, error) {
	if err := ValidateAuthority(spec); err != nil {
		return nil, nil, err
	}
	if err := validateIndexDiscovery(root, spec); err != nil {
		return nil, nil, err
	}

	combined := map[string]string{}
	originByKey := map[string]string{}
	warnings := []Diagnostic{}
	for _, concern := range spec.Concerns {
		values, concernWarnings, err := ValidateConcern(root, spec, concern.ID)
		if err != nil {
			return nil, nil, err
		}
		warnings = append(warnings, concernWarnings...)
		for key, value := range values {
			combined[key] = value
			originByKey[key] = concern.Path
		}
	}

	keySpecs := map[string]KeySpec{}
	for _, concern := range spec.Concerns {
		for key, keySpec := range concern.Keys {
			keySpecs[key] = keySpec
		}
	}

	resolved := map[string]string{}
	for _, key := range sortedValueKeys(combined) {
		value := combined[key]
		if !strings.HasPrefix(value, refPrefix) {
			resolved[key] = value
			continue
		}
		literal, err := resolveReference(combined, key, []string{})
		if err != nil {
			return nil, nil, err
		}
		if err := validateLiteral(key, literal, originByKey[key], keySpecs[key]); err != nil {
			return nil, nil, err
		}
		resolved[key] = literal
	}
	return resolved, warnings, nil
}

// sortedValueKeys returns map keys in stable order.
func sortedValueKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// resolveReference follows one typed reference chain to its literal,
// failing on undeclared targets and cycles with distinct diagnostics.
func resolveReference(combined map[string]string, key string, chain []string) (string, error) {
	for _, visited := range chain {
		if visited == key {
			return "", fmt.Errorf("GOLC_CONFIG_REF_CYCLE: %s", strings.Join(append(chain, key), " -> "))
		}
	}
	chain = append(chain, key)

	value, declared := combined[key]
	if !declared {
		referrer := key
		if len(chain) > 1 {
			referrer = chain[len(chain)-2]
		}
		return "", fmt.Errorf(
			"GOLC_CONFIG_REF_UNRESOLVED: %q referenced by %q is not declared by any concern", key, referrer)
	}
	if strings.HasPrefix(value, refPrefix) {
		return resolveReference(combined, strings.TrimPrefix(value, refPrefix), chain)
	}
	return value, nil
}

// validateIndexDiscovery enforces D-05 discovery: the root index must
// declare exactly the Spec's concern id/path pairs — a hidden or invented
// concern fails with GOLC_CONFIG_INDEX_MISMATCH.
func validateIndexDiscovery(root string, spec Spec) error {
	index, err := LoadRootIndex(root)
	if err != nil {
		return err
	}
	indexed := map[string]string{}
	for _, concern := range index.Concerns {
		indexed[concern.ID] = concern.Path
	}
	for _, concern := range spec.Concerns {
		path, present := indexed[concern.ID]
		if !present {
			return fmt.Errorf(
				"GOLC_CONFIG_INDEX_MISMATCH: %s does not index registered concern %q", rootIndexName, concern.ID)
		}
		if path != concern.Path {
			return fmt.Errorf(
				"GOLC_CONFIG_INDEX_MISMATCH: %s indexes concern %q at %q; the registry declares %q",
				rootIndexName, concern.ID, path, concern.Path)
		}
		delete(indexed, concern.ID)
	}
	if len(indexed) > 0 {
		extras := make([]string, 0, len(indexed))
		for id := range indexed {
			extras = append(extras, id)
		}
		sort.Strings(extras)
		return fmt.Errorf(
			"GOLC_CONFIG_INDEX_MISMATCH: %s indexes unregistered concerns: %s",
			rootIndexName, strings.Join(extras, ", "))
	}
	return nil
}
