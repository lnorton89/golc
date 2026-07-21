// normalize.go is the single ordering and formatting authority every
// generated contract passes through (CONTEXT D-08): recursively sorted
// JSON object keys, stable two-space indentation, and LF-only line
// endings. Generation determinism on Windows and CI depends only on this
// pass, never on invopop's internal marshaling order or on
// encoding/json's incidental (and not language-guaranteed) map-key sort
// behavior.
package contracts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
)

// NormalizeSchema renders schema as canonical, comparison-ready bytes:
// marshal to JSON, recursively sort every object's keys (SortJSON),
// two-space indent, and normalize line endings to a single trailing LF
// (NormalizeLF).
func NormalizeSchema(schema any) ([]byte, error) {
	raw, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("GOLC_CONTRACTS_ENCODE: %v", err)
	}
	sorted, err := SortJSON(raw)
	if err != nil {
		return nil, err
	}
	var indented bytes.Buffer
	if err := json.Indent(&indented, sorted, "", "  "); err != nil {
		return nil, fmt.Errorf("GOLC_CONTRACTS_INDENT: %v", err)
	}
	return NormalizeLF(indented.Bytes()), nil
}

// SortJSON parses arbitrary JSON bytes and re-encodes every object with
// its keys in stable lexicographic order, recursively. Array element
// order is preserved exactly — order is meaningful for "enum" and
// "required" lists — and scalars pass through unchanged.
func SortJSON(data []byte) ([]byte, error) {
	var decoded any
	if err := json.Unmarshal(data, &decoded); err != nil {
		return nil, fmt.Errorf("GOLC_CONTRACTS_DECODE: %v", err)
	}
	encoded, err := json.Marshal(sortJSONValue(decoded))
	if err != nil {
		return nil, fmt.Errorf("GOLC_CONTRACTS_ENCODE: %v", err)
	}
	return encoded, nil
}

// sortedField is one ordered key/value pair inside a sortedObject.
type sortedField struct {
	key   string
	value any
}

// sortedObject is a canonical ordered JSON object. Its explicit
// MarshalJSON writes keys in the exact order they were appended —
// sortJSONValue always appends in already-sorted order — so canonical key
// ordering is guaranteed by this type's own encoding, never left to rely
// on encoding/json's incidental map-marshaling order.
type sortedObject []sortedField

// MarshalJSON writes the object's fields in their stored order.
func (object sortedObject) MarshalJSON() ([]byte, error) {
	var buffer bytes.Buffer
	buffer.WriteByte('{')
	for index, field := range object {
		if index > 0 {
			buffer.WriteByte(',')
		}
		keyBytes, err := json.Marshal(field.key)
		if err != nil {
			return nil, err
		}
		buffer.Write(keyBytes)
		buffer.WriteByte(':')
		valueBytes, err := json.Marshal(field.value)
		if err != nil {
			return nil, err
		}
		buffer.Write(valueBytes)
	}
	buffer.WriteByte('}')
	return buffer.Bytes(), nil
}

// sortJSONValue recursively rewrites decoded JSON values (as produced by
// json.Unmarshal into `any`) into their canonical sorted form: objects
// become a sortedObject with lexicographically ordered keys, array
// elements are each recursively normalized in place without reordering,
// and every other value is returned unchanged.
func sortJSONValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		object := make(sortedObject, 0, len(keys))
		for _, key := range keys {
			object = append(object, sortedField{key: key, value: sortJSONValue(typed[key])})
		}
		return object
	case []any:
		element := make([]any, len(typed))
		for index, item := range typed {
			element[index] = sortJSONValue(item)
		}
		return element
	default:
		return value
	}
}

// NormalizeLF strips carriage returns and ensures data ends with exactly
// one trailing LF byte (D-08: Windows and CI must produce byte-identical
// committed output regardless of the host's native line ending).
func NormalizeLF(data []byte) []byte {
	normalized := bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	normalized = bytes.TrimRight(normalized, "\n")
	normalized = append(normalized, '\n')
	return normalized
}
