// diagnostics.go declares the Diagnostic/ValidationResult shapes every
// "script validate" caller consumes (08-07-PLAN.md Task 1, CONTEXT
// SCRP-01/SCRP-03): a stable, JSON-marshalable diagnostic carrying a
// machine-readable Code, a human message, and a Line/Column pointing at
// the *user's own source* -- never the materialized shim+source file
// Deno actually type-checks (validate.go's shim-offset math is what keeps
// that true).
//
// Diagnostics always sort by (Line, Column, Code) before being returned
// to a caller (sortDiagnostics), so "script validate"'s JSON output is
// byte-stable across repeated runs of the identical script, regardless
// of the order the module gate and deno check happened to produce them
// in.
package script

import "sort"

// Severity is one diagnostic's closed severity vocabulary.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

// Diagnostic is one validation finding: a stable machine-readable Code, a
// human-readable Message, a Line/Column in the *user's own source*
// coordinates, and a Severity.
type Diagnostic struct {
	Code     string   `json:"code"`
	Message  string   `json:"message"`
	Line     int      `json:"line"`
	Column   int      `json:"column"`
	Severity Severity `json:"severity"`
}

// ValidationResult is Validate's return value: the validated script's
// name, every Diagnostic found (never nil -- always at least an empty
// slice), and Valid, true exactly when Diagnostics is empty.
type ValidationResult struct {
	ScriptName  string       `json:"script_name"`
	Diagnostics []Diagnostic `json:"diagnostics"`
	Valid       bool         `json:"valid"`
}

// sortDiagnostics sorts diagnostics by (Line, Column, Code) in place, so
// every caller -- "script validate"'s JSON output chief among them --
// renders diagnostics in a byte-stable order regardless of the order the
// module gate and deno check happened to produce them in.
func sortDiagnostics(diagnostics []Diagnostic) {
	sort.Slice(diagnostics, func(i, j int) bool {
		a, b := diagnostics[i], diagnostics[j]
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Column != b.Column {
			return a.Column < b.Column
		}
		return a.Code < b.Code
	})
}
