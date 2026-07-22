// decode.go implements FIXT-01/FIXT-02's single strict-decode entrypoint
// every fixture-source path (hand-authored YAML now, the OFL-import path
// in 02-03 later) normalizes through: go.yaml.in/yaml/v4's
// WithKnownFields()+WithUniqueKeys() reject duplicate mapping keys and
// unmodeled fields before any typed value is populated (CONTEXT threat
// T-02-01 -- WithUniqueKeys() is documented by the library itself as a
// security feature preventing key-override attacks). Post-decode
// validation then rejects out-of-[0,1] capability ranges, unsupported
// capability semantics, overlapping same-type capability ranges, and
// empty/zero-capability/null-capability-list documents (T-02-02: bounded,
// small hand-authored input, not a stream) with an actionable
// GOLC_FIXTURE_* diagnostic naming the offending value, key, and origin
// (mirrors internal/projectconfig/decode.go's validateLiteral message
// shape).
package fixture

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	yaml "go.yaml.in/yaml/v4"
)

// Decode strictly decodes data into a FixtureDefinition. Every rejection
// is a GOLC_FIXTURE_* diagnostic; nothing is trusted before the strict
// decode and post-decode validation both succeed.
func Decode(data []byte) (FixtureDefinition, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return FixtureDefinition{}, fmt.Errorf("GOLC_FIXTURE_EMPTY: fixture document is empty")
	}

	var def FixtureDefinition
	if err := yaml.Load(data, &def, yaml.WithKnownFields(), yaml.WithUniqueKeys()); err != nil {
		return FixtureDefinition{}, fmt.Errorf("GOLC_FIXTURE_YAML_INVALID: %v", err)
	}

	if err := Validate(def); err != nil {
		return FixtureDefinition{}, err
	}
	return def, nil
}

// Validate enforces FIXT-02's post-decode rules against an already
// (however sourced) built FixtureDefinition. Decode calls this after its
// own strict YAML decode; internal/fixture/ofl.Normalize (02-03) calls it
// directly after mapping OFL's JSON shape onto FixtureDefinition, so
// hand-authored and OFL-imported fixtures run through the exact same
// validation logic rather than two independently-evolving copies of it
// (RESEARCH D-16 risk this repo's own precedent already warns against).
func Validate(def FixtureDefinition) error {
	return validate(def)
}

// supportedCapabilityTypes is the declared enum as a lookup set, built
// once from the exported, order-preserving SupportedCapabilityTypes.
var supportedCapabilityTypes = func() map[CapabilityType]bool {
	set := make(map[CapabilityType]bool, len(SupportedCapabilityTypes))
	for _, capabilityType := range SupportedCapabilityTypes {
		set[capabilityType] = true
	}
	return set
}()

// validate enforces FIXT-02's post-decode rules against an already
// strictly-decoded FixtureDefinition.
func validate(def FixtureDefinition) error {
	if def.SchemaVersion != 1 {
		return fmt.Errorf(
			"GOLC_FIXTURE_SCHEMA_VERSION_UNSUPPORTED: schema_version %d is not supported (only 1 is supported)",
			def.SchemaVersion)
	}
	if strings.TrimSpace(def.Manufacturer) == "" {
		return fmt.Errorf("GOLC_FIXTURE_MANUFACTURER_EMPTY: fixture manufacturer must not be empty")
	}
	if strings.TrimSpace(def.Model) == "" {
		return fmt.Errorf("GOLC_FIXTURE_MODEL_EMPTY: fixture model must not be empty")
	}
	if len(def.Modes) == 0 {
		return fmt.Errorf(
			"GOLC_FIXTURE_MODES_EMPTY: %s %s declares zero modes; a fixture must declare at least one",
			def.Manufacturer, def.Model)
	}
	if len(def.Capabilities) == 0 {
		return fmt.Errorf(
			"GOLC_FIXTURE_EMPTY: %s %s declares zero capabilities; a fixture must declare at least one",
			def.Manufacturer, def.Model)
	}

	rangesByType := map[CapabilityType][][2]float64{}
	for index, capability := range def.Capabilities {
		if !supportedCapabilityTypes[capability.Type] {
			return fmt.Errorf(
				"GOLC_FIXTURE_CAPABILITY_TYPE_UNSUPPORTED: %q is not a supported capability type for capability %d in %s %s",
				capability.Type, index, def.Manufacturer, def.Model)
		}

		low, high := capability.Range[0], capability.Range[1]
		if low < 0 || low > 1 || high < 0 || high > 1 || low > high {
			return fmt.Errorf(
				"GOLC_FIXTURE_CAPABILITY_RANGE_INVALID: range [%v,%v] is outside [0,1] for capability %d (%s) in %s %s",
				low, high, index, capability.Type, def.Manufacturer, def.Model)
		}

		rangesByType[capability.Type] = append(rangesByType[capability.Type], capability.Range)
	}

	if err := rejectOverlappingRanges(rangesByType, def); err != nil {
		return err
	}
	return nil
}

// rejectOverlappingRanges rejects any two same-type ranges that overlap
// (share more than a single touching boundary point). Exactly-adjacent
// ranges ([0,0.5] and [0.5,1]) are allowed; capability types are walked in
// stable declared order so the reported diagnostic is deterministic.
func rejectOverlappingRanges(rangesByType map[CapabilityType][][2]float64, def FixtureDefinition) error {
	for _, capabilityType := range SupportedCapabilityTypes {
		ranges, present := rangesByType[capabilityType]
		if !present || len(ranges) < 2 {
			continue
		}
		sorted := append([][2]float64(nil), ranges...)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i][0] < sorted[j][0] })
		for i := 1; i < len(sorted); i++ {
			if sorted[i][0] < sorted[i-1][1] {
				return fmt.Errorf(
					"GOLC_FIXTURE_CAPABILITY_RANGE_INVALID: range [%v,%v] overlaps range [%v,%v] for capability type %q in %s %s",
					sorted[i][0], sorted[i][1], sorted[i-1][0], sorted[i-1][1], capabilityType, def.Manufacturer, def.Model)
			}
		}
	}
	return nil
}
