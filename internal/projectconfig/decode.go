// decode.go is the strict concern decoder (CONTEXT D-09/D-10): every
// concern file validates alone, the combined declarations assign each
// canonical key exactly once, and unknown, duplicate, invalid, deprecated,
// collided, duplicate-authority, unresolved, and cyclic inputs each fail
// with a distinct stable diagnostic.
package projectconfig

import "errors"

// ValidateAuthority checks a Spec's own registry before any file is read:
// duplicate canonical key ownership across concerns fails with
// GOLC_CONFIG_DUPLICATE_AUTHORITY, and malformed deprecation entries fail
// with GOLC_CONFIG_DEPRECATION_INVALID.
func ValidateAuthority(spec Spec) error {
	return errors.New("GOLC_CONFIG_UNIMPLEMENTED: ValidateAuthority")
}

// ValidateConcern strictly decodes one concern file in isolation and
// returns its declared canonical values plus deprecation warnings.
func ValidateConcern(root string, spec Spec, concernID string) (map[string]string, []Diagnostic, error) {
	return nil, nil, errors.New("GOLC_CONFIG_UNIMPLEMENTED: ValidateConcern")
}

// ValidateRepository validates the whole strict configuration set: the
// registry, the root index discovery contract, every concern in isolation,
// and the cross-concern reference graph. It returns the combined resolved
// values (references replaced by their single-authority literals) plus all
// deprecation warnings.
func ValidateRepository(root string, spec Spec) (map[string]string, []Diagnostic, error) {
	return nil, nil, errors.New("GOLC_CONFIG_UNIMPLEMENTED: ValidateRepository")
}
