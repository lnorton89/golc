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

func requireErrorContains(t *testing.T, err error, substr string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected an error containing %q, got nil", substr)
	}
	if !strings.Contains(err.Error(), substr) {
		t.Fatalf("expected error containing %q, got: %v", substr, err)
	}
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
			if err := strictjson.ValidateSingleValueNoDuplicateNames([]byte(input)); err != nil {
				t.Fatalf("ValidateSingleValueNoDuplicateNames(%q) = %v, want nil", input, err)
			}
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
			requireErrorContains(t, err, "STRICTJSON_DUPLICATE_NAME")
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
			requireErrorContains(t, err, "STRICTJSON_MULTIPLE_VALUES")
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
			if err := strictjson.ValidateSingleValueNoDuplicateNames([]byte(input)); err == nil {
				t.Fatalf("ValidateSingleValueNoDuplicateNames(%q) accepted malformed JSON", input)
			}
		}
	})

	t.Run("DecodeStrict decodes valid single-shot documents", func(t *testing.T) {
		var out sampleDocument
		if err := strictjson.DecodeStrict([]byte(`{"name":"golc","count":3}`), &out); err != nil {
			t.Fatalf("DecodeStrict: %v", err)
		}
		if out.Name != "golc" || out.Count != 3 {
			t.Fatalf("DecodeStrict decoded %+v, want {golc 3}", out)
		}
	})

	t.Run("DecodeStrict rejects unknown fields", func(t *testing.T) {
		var out sampleDocument
		err := strictjson.DecodeStrict([]byte(`{"name":"golc","count":3,"extra":true}`), &out)
		if err == nil {
			t.Fatal("DecodeStrict accepted an unknown field")
		}
	})

	t.Run("DecodeStrict rejects duplicate names before typed decode", func(t *testing.T) {
		var out sampleDocument
		err := strictjson.DecodeStrict([]byte(`{"name":"golc","name":"drift","count":3}`), &out)
		requireErrorContains(t, err, "STRICTJSON_DUPLICATE_NAME")
	})

	t.Run("DecodeStrict rejects a second concatenated value", func(t *testing.T) {
		var out sampleDocument
		err := strictjson.DecodeStrict([]byte(`{"name":"golc","count":3}{"name":"golc","count":3}`), &out)
		requireErrorContains(t, err, "STRICTJSON_MULTIPLE_VALUES")
	})

	t.Run("CanonicalEncode is deterministic, LF-terminated, and idempotent", func(t *testing.T) {
		value := map[string]any{"z": 1, "a": 2, "m": []int{3, 2, 1}}
		first, err := strictjson.CanonicalEncode(value)
		if err != nil {
			t.Fatalf("CanonicalEncode: %v", err)
		}
		second, err := strictjson.CanonicalEncode(value)
		if err != nil {
			t.Fatalf("CanonicalEncode (second run): %v", err)
		}
		if string(first) != string(second) {
			t.Fatalf("CanonicalEncode is not idempotent:\nfirst:  %q\nsecond: %q", first, second)
		}
		if strings.Contains(string(first), "\r") {
			t.Fatal("CanonicalEncode output must not contain carriage returns")
		}
		if !strings.HasSuffix(string(first), "\n") {
			t.Fatal("CanonicalEncode output must be newline-terminated")
		}
		indexA := strings.Index(string(first), "\"a\"")
		indexM := strings.Index(string(first), "\"m\"")
		indexZ := strings.Index(string(first), "\"z\"")
		if !(indexA < indexM && indexM < indexZ) {
			t.Fatalf("CanonicalEncode did not sort map keys: %s", first)
		}
	})

	t.Run("CanonicalEncode output round-trips through DecodeStrict", func(t *testing.T) {
		type roundTrip struct {
			Name string `json:"name"`
		}
		encoded, err := strictjson.CanonicalEncode(roundTrip{Name: "golc"})
		if err != nil {
			t.Fatalf("CanonicalEncode: %v", err)
		}
		var decoded roundTrip
		if err := strictjson.DecodeStrict(encoded, &decoded); err != nil {
			t.Fatalf("DecodeStrict(CanonicalEncode(...)): %v", err)
		}
		if decoded.Name != "golc" {
			t.Fatalf("round-tripped value %+v, want Name=golc", decoded)
		}
	})
}
