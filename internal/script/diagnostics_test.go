// diagnostics_test.go covers internal/script/diagnostics.go (08-07-
// PLAN.md Task 1): Diagnostic's stable JSON shape and sortDiagnostics'
// (Line, Column, Code) ordering.
package script

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDiagnosticMarshalsStableJSON(t *testing.T) {
	d := Diagnostic{
		Code:     "GOLC_SCRIPT_IMPORT_FORBIDDEN",
		Message:  "import statements are forbidden",
		Line:     3,
		Column:   1,
		Severity: SeverityError,
	}
	got, err := json.Marshal(d)
	require.NoError(t, err, "json.Marshal: %v", err)
	want := `{"code":"GOLC_SCRIPT_IMPORT_FORBIDDEN","message":"import statements are forbidden","line":3,"column":1,"severity":"error"}`
	require.Equal(t, want, string(got), "Marshal(Diagnostic) = %s, want %s", got, want)
}

func TestValidationResultMarshalsEmptyDiagnosticsAsArray(t *testing.T) {
	result := ValidationResult{ScriptName: "Chase", Diagnostics: []Diagnostic{}, Valid: true}
	got, err := json.Marshal(result)
	require.NoError(t, err, "json.Marshal: %v", err)
	want := `{"script_name":"Chase","diagnostics":[],"valid":true}`
	require.Equal(t, want, string(got), "Marshal(ValidationResult) = %s, want %s", got, want)
}

func TestSortDiagnosticsOrdersByLineColumnCode(t *testing.T) {
	diagnostics := []Diagnostic{
		{Code: "B", Line: 2, Column: 1},
		{Code: "A", Line: 1, Column: 5},
		{Code: "Z", Line: 1, Column: 5},
		{Code: "A", Line: 1, Column: 1},
	}
	sortDiagnostics(diagnostics)

	wantOrder := []struct {
		Line, Column int
		Code         string
	}{
		{1, 1, "A"},
		{1, 5, "A"},
		{1, 5, "Z"},
		{2, 1, "B"},
	}
	require.Len(t, diagnostics, len(wantOrder), "len(diagnostics) = %d, want %d", len(diagnostics), len(wantOrder))
	for i, want := range wantOrder {
		got := diagnostics[i]
		require.True(t, got.Line == want.Line && got.Column == want.Column && got.Code == want.Code,
			"diagnostics[%d] = {Line:%d Column:%d Code:%s}, want {Line:%d Column:%d Code:%s}",
			i, got.Line, got.Column, got.Code, want.Line, want.Column, want.Code)
	}
}
