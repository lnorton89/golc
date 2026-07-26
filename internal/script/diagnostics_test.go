// diagnostics_test.go covers internal/script/diagnostics.go (08-07-
// PLAN.md Task 1): Diagnostic's stable JSON shape and sortDiagnostics'
// (Line, Column, Code) ordering.
package script

import (
	"encoding/json"
	"testing"
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
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	want := `{"code":"GOLC_SCRIPT_IMPORT_FORBIDDEN","message":"import statements are forbidden","line":3,"column":1,"severity":"error"}`
	if string(got) != want {
		t.Fatalf("Marshal(Diagnostic) = %s, want %s", got, want)
	}
}

func TestValidationResultMarshalsEmptyDiagnosticsAsArray(t *testing.T) {
	result := ValidationResult{ScriptName: "Chase", Diagnostics: []Diagnostic{}, Valid: true}
	got, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	want := `{"script_name":"Chase","diagnostics":[],"valid":true}`
	if string(got) != want {
		t.Fatalf("Marshal(ValidationResult) = %s, want %s", got, want)
	}
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
	if len(diagnostics) != len(wantOrder) {
		t.Fatalf("len(diagnostics) = %d, want %d", len(diagnostics), len(wantOrder))
	}
	for i, want := range wantOrder {
		got := diagnostics[i]
		if got.Line != want.Line || got.Column != want.Column || got.Code != want.Code {
			t.Fatalf("diagnostics[%d] = {Line:%d Column:%d Code:%s}, want {Line:%d Column:%d Code:%s}",
				i, got.Line, got.Column, got.Code, want.Line, want.Column, want.Code)
		}
	}
}
