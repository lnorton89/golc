// envelope.go implements FDUI-01's shared fixture-import artifact shape
// (09-05-PLAN.md Task 2): ImportEnvelope is the one definition of the
// {definition, provenance} document "fixture import --out" writes
// (internal/command/fixture.go's runFixtureImport) -- previously a
// private, independently-declared fixtureImportOutput struct in that
// package, now the single shared type both the command layer and
// ListDirectory's .json support decode through. DecodeEnvelope strictly
// decodes the JSON envelope and then runs its definition through the
// already-exported Validate -- the identical validation the hand-authored
// YAML path and ofl.Normalize both use. It introduces no new validation
// rule and no lenient fallback: a bare definition document with no
// "definition" key, or a definition that fails Validate, is a rejection.
package fixture

import (
	"encoding/json"
	"errors"
	"fmt"
)

// ImportEnvelope is the canonical fixture+provenance artifact shape
// "fixture import --out" writes: the full pinned FixtureDefinition plus
// its Provenance (including any LossyImportWarning entries). json tags
// mirror the bytes runFixtureImport already writes exactly, so widening
// ListDirectory to decode this shape is byte-compatible with every
// existing --out artifact on disk.
type ImportEnvelope struct {
	Definition FixtureDefinition `json:"definition"`
	Provenance Provenance        `json:"provenance"`
}

// rawEnvelope mirrors ImportEnvelope's JSON shape but leaves Definition
// undecoded as raw JSON so DecodeEnvelope can distinguish "no definition
// key present at all" (a bare definition document, rejected) from "a
// definition key present but its value fails Validate" (also rejected,
// with the same diagnostic) -- both are GOLC_FIXTURE_ENVELOPE_INVALID,
// but detecting the first case requires seeing whether the key existed
// before attempting to decode/validate it.
type rawEnvelope struct {
	Definition *json.RawMessage `json:"definition"`
	Provenance Provenance       `json:"provenance"`
}

// DecodeEnvelope strictly decodes data as an ImportEnvelope and runs its
// definition through Validate -- the identical post-decode validation the
// hand-authored YAML path (Decode) and ofl.Normalize both already use.
// Every rejection carries GOLC_FIXTURE_ENVELOPE_INVALID: malformed JSON, a
// document with no "definition" key (a bare definition document is never
// treated as an implicit envelope), and a definition that fails Validate
// are all rejected rather than silently producing a zero-valued
// definition.
func DecodeEnvelope(data []byte) (ImportEnvelope, error) {
	var raw rawEnvelope
	if err := json.Unmarshal(data, &raw); err != nil {
		return ImportEnvelope{}, fmt.Errorf("GOLC_FIXTURE_ENVELOPE_INVALID: %v", err)
	}
	if raw.Definition == nil {
		return ImportEnvelope{}, errors.New("GOLC_FIXTURE_ENVELOPE_INVALID: document has no \"definition\" key")
	}

	var def FixtureDefinition
	if err := json.Unmarshal(*raw.Definition, &def); err != nil {
		return ImportEnvelope{}, fmt.Errorf("GOLC_FIXTURE_ENVELOPE_INVALID: %v", err)
	}
	if err := Validate(def); err != nil {
		return ImportEnvelope{}, fmt.Errorf("GOLC_FIXTURE_ENVELOPE_INVALID: %v", err)
	}

	return ImportEnvelope{Definition: def, Provenance: raw.Provenance}, nil
}
