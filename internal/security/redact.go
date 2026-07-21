// Package security is the single centralized owner of GOLC's secret-safe
// diagnostic and canary-scan contract (CONTEXT D-19/D-20, T-01-18): every
// caller that needs to render a sensitive declaration's presence, redact a
// value before it can be logged, or prove that no fake-secret byte leaked
// into stdout, stderr, a generated schema, a committed map, a report, a
// manifest, or a ZIP archive goes through this package rather than
// re-implementing its own ad hoc masking.
//
// SafeDiagnostic is deliberately narrow: a stable machine-readable code, a
// safe message, and an optional map of already-safe named fields. No caller
// may attach a raw environment map, HTTP header set, config struct, or
// error/exception value to it -- there is no field shape that would even
// accept one. Every field value SafeDiagnostic.String renders is re-passed
// through Redact, so a caller cannot bypass the canary/pattern scan by
// constructing a SafeDiagnostic directly with an already-leaked value.
package security

import (
	"fmt"
	"sort"
	"strings"
)

// CanaryToken is a unique, obviously-fake secret-shaped byte sequence this
// package's own test suite plants into a controlled buffer to prove
// ScanCanary actually detects leaked secret material, rather than only ever
// observing already-clean output. It must never appear in any real
// committed or generated repository artifact.
const CanaryToken = "GOLC_FAKE_SECRET_CANARY_4f9c2e6b1a7d3f809c21"

// forbiddenPatterns are the byte-level tokens ScanCanary rejects: the exact
// CanaryToken plus a fixed set of common secret-shaped substrings (Linear
// credential declarations, bearer auth headers, and API-key prefixes) that
// must never appear in emitted diagnostics or committed/generated
// artifacts, independent of whether the CanaryToken itself was ever
// planted.
var forbiddenPatterns = []string{
	CanaryToken,
	"LINEAR_API_KEY=",
	"Bearer ",
	"sk-",
	"lin_api_",
}

// ScanCanary reports the first forbidden token found in data, or "" when
// data is clean. It operates on raw bytes so stdout/stderr captures,
// generated schema/map/report/manifest files, and raw ZIP archive bytes are
// all scanned identically -- no output surface receives special-cased
// treatment.
func ScanCanary(data []byte) string {
	text := string(data)
	for _, token := range forbiddenPatterns {
		if strings.Contains(text, token) {
			return token
		}
	}
	return ""
}

// CanaryViolation names one output surface that leaked a forbidden token.
type CanaryViolation struct {
	Source string
	Token  string
}

// ScanCanaryAll scans every named source in sources and returns every
// violation, sorted by source name for deterministic reporting. A nil or
// empty return value means every source is clean. Callers use this instead
// of repeated ScanCanary calls so a failure can name exactly which output
// surface leaked.
func ScanCanaryAll(sources map[string][]byte) []CanaryViolation {
	names := make([]string, 0, len(sources))
	for name := range sources {
		names = append(names, name)
	}
	sort.Strings(names)

	var violations []CanaryViolation
	for _, name := range names {
		if token := ScanCanary(sources[name]); token != "" {
			violations = append(violations, CanaryViolation{Source: name, Token: token})
		}
	}
	return violations
}

// SetState renders whether a sensitive declaration (an environment variable
// such as LINEAR_API_KEY) is populated without ever exposing its value
// (CONTEXT D-19/D-20): "<set>" when a non-blank value is present, "<unset>"
// otherwise. This is the only rendering form provenance/status output may
// use for a declared-secret field.
func SetState(value string) string {
	if strings.TrimSpace(value) == "" {
		return "<unset>"
	}
	return "<set>"
}

// redactedMarker replaces any value ScanCanary flags. Redact never returns
// even a truncated fragment of the original bytes for a flagged value, so a
// coding mistake elsewhere can never leak a real secret through a
// "redacted" value that still contains part of it.
const redactedMarker = "<redacted>"

// Redact returns value unchanged when it is already safe (ScanCanary finds
// no forbidden token in it); otherwise it returns the fixed redactedMarker.
func Redact(value string) string {
	if ScanCanary([]byte(value)) != "" {
		return redactedMarker
	}
	return value
}

// SafeDiagnostic is the only allowlisted diagnostic shape this repository's
// CLI, configuration, and adapter code may build for anything
// environment/header/config/exception shaped: a stable code, a safe
// message, and an optional map of safe named fields. Every field value is
// re-redacted at render time (String), so constructing a SafeDiagnostic
// directly with an unvetted value can never leak it.
type SafeDiagnostic struct {
	Code    string
	Message string
	Fields  map[string]string
}

// String renders d as one stable, redaction-safe line:
// "<code>: <message> (field=value, field=value)", with fields sorted by
// name for deterministic output. The parenthesized field list is omitted
// entirely when d.Fields is empty.
func (d SafeDiagnostic) String() string {
	var builder strings.Builder
	builder.WriteString(d.Code)
	builder.WriteString(": ")
	builder.WriteString(d.Message)
	if len(d.Fields) == 0 {
		return builder.String()
	}

	names := make([]string, 0, len(d.Fields))
	for name := range d.Fields {
		names = append(names, name)
	}
	sort.Strings(names)

	parts := make([]string, 0, len(names))
	for _, name := range names {
		parts = append(parts, fmt.Sprintf("%s=%s", name, Redact(d.Fields[name])))
	}
	builder.WriteString(" (")
	builder.WriteString(strings.Join(parts, ", "))
	builder.WriteString(")")
	return builder.String()
}
