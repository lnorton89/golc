// Package strictjson_test covers the duplicate-safe strict JSON guard
// (CONTEXT threat T-01-24): duplicate object member names, more than one
// top-level JSON value, and unknown fields must all fail before any typed
// decode happens, and canonical output must be deterministic and
// idempotent.
//
// It is an external test package so it can declare its quick-test scope
// through the command package's exact registration entrypoint (the
// config-local/linear-catalog pattern).
package strictjson_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/command"
	"github.com/lnorton89/golc/internal/strictjson"
)

// The linear-map quick-test scope spans this package and
// internal/trace/catalog; both owned test files declare it identically
// (01-VALIDATION: every owning Go test task registers its scope through
// MustDeclareScope beside its TestScope marker).
var _ = command.MustDeclareScope(command.ScopeRegistration{
	Scope:   "linear-map",
	Summary: "Strict JSON guard and schema-1-to-2 linear map migration tests.",
})

type sampleDocument struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// TestScopeLinearMap is the exact quick-test marker for scope "linear-map"
// (test --quick --scope linear-map).
func TestScopeLinearMap(t *testing.T) {
	t.Run("ValidateSingleValueNoDuplicateNames accepts well-formed single values", func(t *testing.T) {
		valid := []string{
			`{"a":1,"b":{"c":2,"d":[1,2,3]},"e":[{"f":1},{"g":2}]}`,
			`[]`,
			`{}`,
			`"just a string"`,
			`42`,
			`null`,
			"  {\n  \"a\": 1\n}\n\n",
		}
		for _, input := range valid {
			require.NoError(t, strictjson.ValidateSingleValueNoDuplicateNames([]byte(input)), "ValidateSingleValueNoDuplicateNames(%q)", input)
		}
	})

	t.Run("ValidateSingleValueNoDuplicateNames rejects duplicate names at any nesting level", func(t *testing.T) {
		cases := []string{
			`{"a":1,"a":2}`,
			`{"a":{"b":1,"b":2}}`,
			`[{"a":1},{"a":1,"a":2}]`,
			`{"outer":{"inner":{"x":1,"x":2}}}`,
		}
		for _, input := range cases {
			err := strictjson.ValidateSingleValueNoDuplicateNames([]byte(input))
			require.ErrorContains(t, err, "STRICTJSON_DUPLICATE_NAME")
		}
	})

	t.Run("ValidateSingleValueNoDuplicateNames rejects more than one top-level value", func(t *testing.T) {
		cases := []string{
			`{}{}`,
			`{} {}`,
			`1 2`,
			"{\"a\":1}\n{\"a\":1}",
			`[]null`,
		}
		for _, input := range cases {
			err := strictjson.ValidateSingleValueNoDuplicateNames([]byte(input))
			require.ErrorContains(t, err, "STRICTJSON_MULTIPLE_VALUES")
		}
	})

	t.Run("ValidateSingleValueNoDuplicateNames rejects malformed JSON", func(t *testing.T) {
		cases := []string{
			`{`,
			`{"a":}`,
			`{"a" 1}`,
			``,
			`{"a": 1,}`,
		}
		for _, input := range cases {
			require.Error(t, strictjson.ValidateSingleValueNoDuplicateNames([]byte(input)), "ValidateSingleValueNoDuplicateNames(%q) accepted malformed JSON", input)
		}
	})

	t.Run("DecodeStrict decodes valid single-shot documents", func(t *testing.T) {
		var out sampleDocument
		require.NoError(t, strictjson.DecodeStrict([]byte(`{"name":"golc","count":3}`), &out), "DecodeStrict")
		require.True(t, out.Name == "golc" && out.Count == 3, "DecodeStrict decoded %+v, want {golc 3}", out)
	})

	t.Run("DecodeStrict rejects unknown fields", func(t *testing.T) {
		var out sampleDocument
		err := strictjson.DecodeStrict([]byte(`{"name":"golc","count":3,"extra":true}`), &out)
		require.Error(t, err, "DecodeStrict accepted an unknown field")
	})

	t.Run("DecodeStrict rejects duplicate names before typed decode", func(t *testing.T) {
		var out sampleDocument
		err := strictjson.DecodeStrict([]byte(`{"name":"golc","name":"drift","count":3}`), &out)
		require.ErrorContains(t, err, "STRICTJSON_DUPLICATE_NAME")
	})

	t.Run("DecodeStrict rejects a second concatenated value", func(t *testing.T) {
		var out sampleDocument
		err := strictjson.DecodeStrict([]byte(`{"name":"golc","count":3}{"name":"golc","count":3}`), &out)
		require.ErrorContains(t, err, "STRICTJSON_MULTIPLE_VALUES")
	})

	t.Run("CanonicalEncode is deterministic, LF-terminated, and idempotent", func(t *testing.T) {
		value := map[string]any{"z": 1, "a": 2, "m": []int{3, 2, 1}}
		first, err := strictjson.CanonicalEncode(value)
		require.NoError(t, err, "CanonicalEncode")
		second, err := strictjson.CanonicalEncode(value)
		require.NoError(t, err, "CanonicalEncode (second run)")
		require.Equal(t, string(second), string(first), "CanonicalEncode is not idempotent")
		require.NotContains(t, string(first), "\r", "CanonicalEncode output must not contain carriage returns")
		require.True(t, strings.HasSuffix(string(first), "\n"), "CanonicalEncode output must be newline-terminated")
		indexA := strings.Index(string(first), "\"a\"")
		indexM := strings.Index(string(first), "\"m\"")
		indexZ := strings.Index(string(first), "\"z\"")
		require.True(t, indexA < indexM && indexM < indexZ, "CanonicalEncode did not sort map keys: %s", first)
	})

	t.Run("CanonicalEncode output round-trips through DecodeStrict", func(t *testing.T) {
		type roundTrip struct {
			Name string `json:"name"`
		}
		encoded, err := strictjson.CanonicalEncode(roundTrip{Name: "golc"})
		require.NoError(t, err, "CanonicalEncode")
		var decoded roundTrip
		require.NoError(t, strictjson.DecodeStrict(encoded, &decoded), "DecodeStrict(CanonicalEncode(...))")
		require.Equal(t, "golc", decoded.Name, "round-tripped value %+v, want Name=golc", decoded)
	})
}
