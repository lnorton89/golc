// decode.go implements FIXT-01/FIXT-02's single strict-decode entrypoint
// every fixture-source path (hand-authored YAML now, the OFL-import path
// in 02-03 later) normalizes through: go.yaml.in/yaml/v4's
// WithKnownFields()+WithUniqueKeys() reject duplicate mapping keys and
// unmodeled fields before any typed value is populated (CONTEXT threat
// T-02-01 -- WithUniqueKeys() is documented by the library itself as a
// security feature preventing key-override attacks).
package fixture

import (
	"fmt"

	yaml "go.yaml.in/yaml/v4"
)

// Decode strictly decodes data into a FixtureDefinition. Every rejection
// is a GOLC_FIXTURE_* diagnostic; nothing is trusted before the strict
// decode succeeds.
func Decode(data []byte) (FixtureDefinition, error) {
	var def FixtureDefinition
	if err := yaml.Load(data, &def, yaml.WithKnownFields(), yaml.WithUniqueKeys()); err != nil {
		return FixtureDefinition{}, fmt.Errorf("GOLC_FIXTURE_YAML_INVALID: %v", err)
	}
	return def, nil
}
