// Package strictjson implements the duplicate-safe strict JSON guard
// (CONTEXT threat T-01-24): a document is accepted only if it is exactly
// one JSON value and no object anywhere in it repeats a member name.
// encoding/json silently keeps the last value of a duplicate object key,
// which could let a tampered planning artifact quietly redefine identity;
// this package rejects that before any typed decode runs. It also exposes
// a deterministic canonical encoder so migrated artifacts are byte-stable
// and reviewable.
package strictjson

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// ValidateSingleValueNoDuplicateNames walks data as a stream of JSON
// tokens and fails if any object defines the same member name more than
// once at any nesting level, or if the document contains more than one
// top-level JSON value (for example two concatenated objects).
func ValidateSingleValueNoDuplicateNames(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()

	if err := validateValue(decoder); err != nil {
		return err
	}

	trailing, err := decoder.Token()
	switch {
	case err == io.EOF:
		return nil
	case err != nil:
		return fmt.Errorf("STRICTJSON_PARSE: %v", err)
	default:
		return fmt.Errorf("STRICTJSON_MULTIPLE_VALUES: unexpected additional token %v after the single top-level JSON value", trailing)
	}
}

// validateValue consumes exactly one JSON value from decoder — a
// primitive, or a fully balanced object/array — rejecting duplicate
// member names inside every object it recurses into.
func validateValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		if err == io.EOF {
			return fmt.Errorf("STRICTJSON_PARSE: unexpected end of JSON input")
		}
		return fmt.Errorf("STRICTJSON_PARSE: %v", err)
	}

	delim, isDelim := token.(json.Delim)
	if !isDelim {
		// A primitive value: string, json.Number, bool, or nil. Nothing
		// further to consume for this value.
		return nil
	}

	switch delim {
	case '{':
		seen := map[string]bool{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return fmt.Errorf("STRICTJSON_PARSE: %v", err)
			}
			key, isString := keyToken.(string)
			if !isString {
				return fmt.Errorf("STRICTJSON_PARSE: object member name %v is not a string", keyToken)
			}
			if seen[key] {
				return fmt.Errorf("STRICTJSON_DUPLICATE_NAME: duplicate object member %q", key)
			}
			seen[key] = true
			if err := validateValue(decoder); err != nil {
				return err
			}
		}
		if _, err := decoder.Token(); err != nil { // consume closing '}'
			return fmt.Errorf("STRICTJSON_PARSE: %v", err)
		}
		return nil
	case '[':
		for decoder.More() {
			if err := validateValue(decoder); err != nil {
				return err
			}
		}
		if _, err := decoder.Token(); err != nil { // consume closing ']'
			return fmt.Errorf("STRICTJSON_PARSE: %v", err)
		}
		return nil
	default:
		return fmt.Errorf("STRICTJSON_PARSE: unexpected delimiter %v", delim)
	}
}

// DecodeStrict validates data with ValidateSingleValueNoDuplicateNames,
// then decodes it into out, rejecting any JSON member with no matching
// field (encoding/json's DisallowUnknownFields). Duplicate names, multiple
// top-level values, and unknown fields are all rejected before out is
// populated with anything.
func DecodeStrict(data []byte, out any) error {
	if err := ValidateSingleValueNoDuplicateNames(data); err != nil {
		return err
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
	if err := decoder.Decode(out); err != nil {
		return fmt.Errorf("STRICTJSON_DECODE: %v", err)
	}
	if decoder.More() {
		return fmt.Errorf("STRICTJSON_MULTIPLE_VALUES: unexpected additional JSON value after the decoded document")
	}
	return nil
}

// CanonicalEncode renders v as deterministic, LF-terminated, indented
// JSON. encoding/json already sorts map keys and preserves struct field
// declaration order, so repeated calls with equal input produce
// byte-identical output.
func CanonicalEncode(v any) ([]byte, error) {
	encoded, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("STRICTJSON_ENCODE: %v", err)
	}
	encoded = bytes.ReplaceAll(encoded, []byte("\r\n"), []byte("\n"))
	if !bytes.HasSuffix(encoded, []byte("\n")) {
		encoded = append(encoded, '\n')
	}
	return encoded, nil
}
