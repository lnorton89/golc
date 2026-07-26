// validate_test.go covers internal/script/validate.go (08-07-PLAN.md):
// the structural zero-import gate (Task 1) and deno check type validation
// plus the shim line-offset math (Task 2). It is an internal (white-box)
// test package so it can assert directly against
// checkForbiddenModuleSyntax, stripCommentsAndStringLiterals,
// buildDenoCheckArgs, parseDenoCheckDiagnostics, and shimLineOffsetFor.
package script

import (
	"strings"
	"testing"
)

func TestCheckForbiddenModuleSyntaxIgnoresCommentsAndStrings(t *testing.T) {
	source := strings.Join([]string{
		`// import "leaked" -- this is only a comment`,
		`/* export something -- also only a comment */`,
		`const message = "please import this string, it is not code";`,
		"const template = `this template mentions export but is not code`;",
		`await golc.scene.activate({ name: "Alpha", show: "show.golc" });`,
	}, "\n")

	diagnostics := checkForbiddenModuleSyntax(source)
	if len(diagnostics) != 0 {
		t.Fatalf("expected zero diagnostics for comment/string-only occurrences, got %+v", diagnostics)
	}
}

func TestCheckForbiddenModuleSyntaxDetectsStaticImport(t *testing.T) {
	source := "await golc.scene.activate({ name: \"Alpha\" });\nimport { evil } from \"./mod.ts\";\n"
	diagnostics := checkForbiddenModuleSyntax(source)
	if len(diagnostics) != 1 {
		t.Fatalf("expected exactly one diagnostic, got %d: %+v", len(diagnostics), diagnostics)
	}
	if diagnostics[0].Code != "GOLC_SCRIPT_IMPORT_FORBIDDEN" {
		t.Fatalf("Code = %q, want GOLC_SCRIPT_IMPORT_FORBIDDEN", diagnostics[0].Code)
	}
	if diagnostics[0].Line != 2 {
		t.Fatalf("Line = %d, want 2", diagnostics[0].Line)
	}
}

func TestCheckForbiddenModuleSyntaxDetectsDynamicImport(t *testing.T) {
	source := "await golc.scene.activate({ name: \"Alpha\" });\nconst mod = await import(\"./mod.ts\");\n"
	diagnostics := checkForbiddenModuleSyntax(source)
	if len(diagnostics) != 1 {
		t.Fatalf("expected exactly one diagnostic, got %d: %+v", len(diagnostics), diagnostics)
	}
	if diagnostics[0].Line != 2 {
		t.Fatalf("Line = %d, want 2", diagnostics[0].Line)
	}
	if !strings.Contains(diagnostics[0].Message, "dynamic import") {
		t.Fatalf("expected the message to name a dynamic import, got %q", diagnostics[0].Message)
	}
}

func TestCheckForbiddenModuleSyntaxDetectsExportDeclaration(t *testing.T) {
	source := "export const shared = 1;\n"
	diagnostics := checkForbiddenModuleSyntax(source)
	if len(diagnostics) != 1 || diagnostics[0].Line != 1 {
		t.Fatalf("expected exactly one diagnostic on line 1, got %+v", diagnostics)
	}
}

func TestCheckForbiddenModuleSyntaxDetectsReexportFrom(t *testing.T) {
	source := "export * from \"./mod.ts\";\n"
	diagnostics := checkForbiddenModuleSyntax(source)
	if len(diagnostics) == 0 {
		t.Fatal("expected at least one diagnostic for a re-export")
	}
}

func TestCheckForbiddenModuleSyntaxAllowsAmbientGolcCallsOnly(t *testing.T) {
	source := "await golc.scene.create({ name: \"Alpha\", bars: 4, show: \"show.golc\" });\n" +
		"await golc.scene.activate({ name: \"Alpha\", show: \"show.golc\" });\n"
	diagnostics := checkForbiddenModuleSyntax(source)
	if len(diagnostics) != 0 {
		t.Fatalf("expected zero diagnostics for a script that only calls the ambient golc namespace, got %+v", diagnostics)
	}
}

func TestCheckForbiddenModuleSyntaxDetectsImportHiddenInTemplateSubstitution(t *testing.T) {
	source := "const evil = `${await import(\"./mod.ts\")}`;\n"
	diagnostics := checkForbiddenModuleSyntax(source)
	if len(diagnostics) == 0 {
		t.Fatal("expected the gate to see a dynamic import hidden inside a template literal substitution")
	}
}

func TestCheckForbiddenModuleSyntaxDoesNotFlagIdentifiersContainingImport(t *testing.T) {
	source := "const importantValue = 5;\nconst exportedLabel = \"x\";\n"
	diagnostics := checkForbiddenModuleSyntax(source)
	if len(diagnostics) != 0 {
		t.Fatalf("expected zero diagnostics for identifiers merely containing the substrings, got %+v", diagnostics)
	}
}
