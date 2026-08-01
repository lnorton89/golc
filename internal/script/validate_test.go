// validate_test.go covers internal/script/validate.go (08-07-PLAN.md):
// the structural zero-import gate (Task 1) and deno check type validation
// plus the shim line-offset math (Task 2). It is an internal (white-box)
// test package so it can assert directly against
// checkForbiddenModuleSyntax, stripCommentsAndStringLiterals,
// buildDenoCheckArgs, parseDenoCheckDiagnostics, and shimLineOffsetFor.
package script

import (
	"context"
	"strings"
	"testing"

	"github.com/lnorton89/golc/internal/show"
	"github.com/stretchr/testify/require"
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
	require.Empty(t, diagnostics, "expected zero diagnostics for comment/string-only occurrences, got %+v", diagnostics)
}

func TestCheckForbiddenModuleSyntaxDetectsStaticImport(t *testing.T) {
	source := "await golc.scene.activate({ name: \"Alpha\" });\nimport { evil } from \"./mod.ts\";\n"
	diagnostics := checkForbiddenModuleSyntax(source)
	require.Len(t, diagnostics, 1, "expected exactly one diagnostic, got %+v", diagnostics)
	require.Equal(t, "GOLC_SCRIPT_IMPORT_FORBIDDEN", diagnostics[0].Code, "Code = %q, want GOLC_SCRIPT_IMPORT_FORBIDDEN", diagnostics[0].Code)
	require.Equal(t, 2, diagnostics[0].Line, "Line = %d, want 2", diagnostics[0].Line)
}

func TestCheckForbiddenModuleSyntaxDetectsDynamicImport(t *testing.T) {
	source := "await golc.scene.activate({ name: \"Alpha\" });\nconst mod = await import(\"./mod.ts\");\n"
	diagnostics := checkForbiddenModuleSyntax(source)
	require.Len(t, diagnostics, 1, "expected exactly one diagnostic, got %d: %+v", len(diagnostics), diagnostics)
	require.Equal(t, 2, diagnostics[0].Line, "Line = %d, want 2", diagnostics[0].Line)
	require.Contains(t, diagnostics[0].Message, "dynamic import", "expected the message to name a dynamic import, got %q", diagnostics[0].Message)
}

func TestCheckForbiddenModuleSyntaxDetectsExportDeclaration(t *testing.T) {
	source := "export const shared = 1;\n"
	diagnostics := checkForbiddenModuleSyntax(source)
	require.Len(t, diagnostics, 1, "expected exactly one diagnostic on line 1, got %+v", diagnostics)
	require.Equal(t, 1, diagnostics[0].Line, "expected exactly one diagnostic on line 1, got %+v", diagnostics)
}

func TestCheckForbiddenModuleSyntaxDetectsReexportFrom(t *testing.T) {
	source := "export * from \"./mod.ts\";\n"
	diagnostics := checkForbiddenModuleSyntax(source)
	require.NotEmpty(t, diagnostics, "expected at least one diagnostic for a re-export")
}

func TestCheckForbiddenModuleSyntaxAllowsAmbientGolcCallsOnly(t *testing.T) {
	source := "await golc.scene.create({ name: \"Alpha\", bars: 4, show: \"show.golc\" });\n" +
		"await golc.scene.activate({ name: \"Alpha\", show: \"show.golc\" });\n"
	diagnostics := checkForbiddenModuleSyntax(source)
	require.Empty(t, diagnostics, "expected zero diagnostics for a script that only calls the ambient golc namespace, got %+v", diagnostics)
}

func TestCheckForbiddenModuleSyntaxDetectsImportHiddenInTemplateSubstitution(t *testing.T) {
	source := "const evil = `${await import(\"./mod.ts\")}`;\n"
	diagnostics := checkForbiddenModuleSyntax(source)
	require.NotEmpty(t, diagnostics, "expected the gate to see a dynamic import hidden inside a template literal substitution")
}

func TestCheckForbiddenModuleSyntaxDoesNotFlagIdentifiersContainingImport(t *testing.T) {
	source := "const importantValue = 5;\nconst exportedLabel = \"x\";\n"
	diagnostics := checkForbiddenModuleSyntax(source)
	require.Empty(t, diagnostics, "expected zero diagnostics for identifiers merely containing the substrings, got %+v", diagnostics)
}

// --- Task 2: deno check type validation and shim-offset math -----------

func TestBuildDenoCheckArgs(t *testing.T) {
	got := buildDenoCheckArgs("/tmp/validate/script.ts")
	want := []string{"check", "--no-remote", "/tmp/validate/script.ts"}
	require.Len(t, got, len(want), "buildDenoCheckArgs = %v, want %v", got, want)
	for i := range want {
		require.Equal(t, want[i], got[i], "buildDenoCheckArgs = %v, want %v", got, want)
	}
}

// TestBuildDenoCheckArgsHasNoAllowFlags asserts against host.go's own
// forbiddenDenoArgPrefixes list (T-08-30): the run command line and the
// check command line are two separate composition sites, but both are
// held to the identical zero-permission-flag guarantee.
func TestBuildDenoCheckArgsHasNoAllowFlags(t *testing.T) {
	args := buildDenoCheckArgs("/tmp/validate/script.ts")
	for _, arg := range args {
		for _, forbidden := range forbiddenDenoArgPrefixes {
			require.False(t, strings.HasPrefix(arg, forbidden), "buildDenoCheckArgs produced forbidden argument %q (prefix %q)", arg, forbidden)
		}
	}
}

func TestShimLineOffsetForDerivedFromShimContent(t *testing.T) {
	shortShim := "line1\nline2\n"
	longShim := "line1\nline2\nline3\nline4\n"

	shortOffset := shimLineOffsetFor(shortShim)
	longOffset := shimLineOffsetFor(longShim)

	require.Equal(t, 3, shortOffset, "shimLineOffsetFor(shortShim) = %d, want 3", shortOffset)
	require.Equal(t, 5, longOffset, "shimLineOffsetFor(longShim) = %d, want 5", longOffset)
	require.Equal(t, 2, longOffset-shortOffset, "expected the offset to grow by exactly the shim's added line count (2), got a delta of %d", longOffset-shortOffset)
}

func TestParseDenoCheckDiagnosticsSubtractsShimOffset(t *testing.T) {
	shimOffset := 3
	output := strings.Join([]string{
		`TS2345 [ERROR]: Argument of type '{ wrongField: number; }' is not assignable to parameter of type 'SceneActivateParams'.`,
		`  Object literal may only specify known properties, and 'wrongField' does not exist in type 'SceneActivateParams'.`,
		`    at file:///tmp/golc-script-validate/script.ts:5:31`,
	}, "\n")

	diagnostics := parseDenoCheckDiagnostics(output, shimOffset)
	require.Len(t, diagnostics, 1, "expected exactly one diagnostic, got %d: %+v", len(diagnostics), diagnostics)
	require.Equal(t, 2, diagnostics[0].Line, "Line = %d, want 2 (raw line 5 minus shim offset 3)", diagnostics[0].Line)
	require.Equal(t, 31, diagnostics[0].Column, "Column = %d, want 31", diagnostics[0].Column)
	require.Equal(t, "GOLC_SCRIPT_TYPECHECK_FAILED", diagnostics[0].Code, "Code = %q, want GOLC_SCRIPT_TYPECHECK_FAILED", diagnostics[0].Code)
}

func TestParseDenoCheckDiagnosticsFlagsPositionInsideShim(t *testing.T) {
	shimOffset := 10
	output := strings.Join([]string{
		`TS2304 [ERROR]: Cannot find name 'somethingBroken'.`,
		`    at file:///tmp/golc-script-validate/script.ts:2:1`,
	}, "\n")

	diagnostics := parseDenoCheckDiagnostics(output, shimOffset)
	require.Len(t, diagnostics, 1, "expected exactly one diagnostic, got %d: %+v", len(diagnostics), diagnostics)
	require.Equal(t, "GOLC_SCRIPT_SDK_SHIM_ERROR", diagnostics[0].Code, "Code = %q, want GOLC_SCRIPT_SDK_SHIM_ERROR", diagnostics[0].Code)
	require.Equal(t, 2, diagnostics[0].Line, "expected the raw (materialized-file) line to be preserved for a shim-internal diagnostic, got %d", diagnostics[0].Line)
}

// TestParseDenoCheckDiagnosticsOffsetDerivedFromMaterializedFileNotHardcoded
// proves the shim line offset used to correct a reported position is
// computed from the materialized shim's actual content (shimLineOffsetFor),
// never a constant: two different shim lengths applied to the identical
// raw deno check output must produce two different, correctly-shifted
// user line numbers (T-08-32).
func TestParseDenoCheckDiagnosticsOffsetDerivedFromMaterializedFileNotHardcoded(t *testing.T) {
	output := strings.Join([]string{
		`TS2304 [ERROR]: Cannot find name 'bogus'.`,
		`    at file:///tmp/golc-script-validate/script.ts:8:1`,
	}, "\n")

	shortShimOffset := shimLineOffsetFor("line1\nline2\n")
	longShimOffset := shimLineOffsetFor("line1\nline2\nline3\nline4\nline5\n")

	shortResult := parseDenoCheckDiagnostics(output, shortShimOffset)
	longResult := parseDenoCheckDiagnostics(output, longShimOffset)

	require.Len(t, shortResult, 1, "expected exactly one diagnostic each, got short=%+v long=%+v", shortResult, longResult)
	require.Len(t, longResult, 1, "expected exactly one diagnostic each, got short=%+v long=%+v", shortResult, longResult)
	require.NotEqual(t, longResult[0].Line, shortResult[0].Line, "expected a longer shim to shift the reported user line, got the same line (%d) for both", shortResult[0].Line)
	require.Equal(t, longShimOffset-shortShimOffset, shortResult[0].Line-longResult[0].Line, "expected the line delta to equal exactly the shim-offset delta")
}

// TestValidateModuleGateNeverSpawnsSubprocess covers: "The gate runs
// before any subprocess is spawned; a script with a forbidden import
// never causes a deno check invocation." t.TempDir() is guaranteed to
// have no .tools/toolchains/deno/ install: if Validate ever tried to
// resolve or spawn Deno after the module gate already produced a
// diagnostic, this would surface as a GOLC_SCRIPT_DENO_MISSING error
// instead of a clean diagnostics result.
func TestValidateModuleGateNeverSpawnsSubprocess(t *testing.T) {
	root := t.TempDir()
	source := `import { evil } from "./mod.ts";`

	result, err := Validate(context.Background(), root, show.Script{Name: "Bad", Source: source})
	require.NoError(t, err, "Validate returned an error (implies it tried to resolve/spawn Deno): %v", err)
	require.False(t, result.Valid, "expected Valid=false for a script with a forbidden import")
	require.Len(t, result.Diagnostics, 1, "expected exactly one GOLC_SCRIPT_IMPORT_FORBIDDEN diagnostic, got %+v", result.Diagnostics)
	require.Equal(t, "GOLC_SCRIPT_IMPORT_FORBIDDEN", result.Diagnostics[0].Code, "expected exactly one GOLC_SCRIPT_IMPORT_FORBIDDEN diagnostic, got %+v", result.Diagnostics)
}

// TestValidateSizeGateNeverSpawnsSubprocess is
// TestValidateModuleGateNeverSpawnsSubprocess's size-bound counterpart.
func TestValidateSizeGateNeverSpawnsSubprocess(t *testing.T) {
	root := t.TempDir()
	oversized := strings.Repeat("x", maxScriptSourceBytes+1)

	result, err := Validate(context.Background(), root, show.Script{Name: "Huge", Source: oversized})
	require.NoError(t, err, "Validate returned an error (implies it tried to resolve/spawn Deno): %v", err)
	require.False(t, result.Valid, "expected Valid=false for an oversized script")
	require.Len(t, result.Diagnostics, 1, "expected exactly one GOLC_SCRIPT_SOURCE_TOO_LARGE diagnostic, got %+v", result.Diagnostics)
	require.Equal(t, "GOLC_SCRIPT_SOURCE_TOO_LARGE", result.Diagnostics[0].Code, "expected exactly one GOLC_SCRIPT_SOURCE_TOO_LARGE diagnostic, got %+v", result.Diagnostics)
}

// --- real-Deno-gated tests (skip when .tools/toolchains/deno/ is not
// provisioned, per skipUnlessDenoProvisioned in session_test.go) --------

func TestValidateCleanScriptReportsZeroDiagnostics(t *testing.T) {
	root := skipUnlessDenoProvisioned(t)

	source := `await golc.scene.activate({ name: "Alpha", show: "ignored" });`
	result, err := Validate(context.Background(), root, show.Script{Name: "Clean", Source: source})
	require.NoError(t, err, "Validate: %v", err)
	require.True(t, result.Valid, "expected a clean, valid result, got %+v", result)
	require.Empty(t, result.Diagnostics, "expected a clean, valid result, got %+v", result)
}

func TestValidateWrongFieldTypeReportsDiagnostic(t *testing.T) {
	root := skipUnlessDenoProvisioned(t)

	source := `await golc.scene.activate({ wrongField: 1 });`
	result, err := Validate(context.Background(), root, show.Script{Name: "WrongField", Source: source})
	require.NoError(t, err, "Validate: %v", err)
	require.False(t, result.Valid, "expected at least one type-error diagnostic, got %+v", result)
	require.NotEmpty(t, result.Diagnostics, "expected at least one type-error diagnostic, got %+v", result)
}

func TestValidateUnknownMethodReportsDiagnostic(t *testing.T) {
	root := skipUnlessDenoProvisioned(t)

	source := `await golc.notAMethod();`
	result, err := Validate(context.Background(), root, show.Script{Name: "UnknownMethod", Source: source})
	require.NoError(t, err, "Validate: %v", err)
	require.False(t, result.Valid, "expected at least one type-error diagnostic proving the .d.ts is loaded, got %+v", result)
	require.NotEmpty(t, result.Diagnostics, "expected at least one type-error diagnostic proving the .d.ts is loaded, got %+v", result)
}
