// scriptvalidate_test.go covers "script validate" (08-07-PLAN.md): a
// malformed invocation, a missing show, a missing script, and a forbidden
// import all fail before ever running a check (no real Deno needed); a
// real validate run -- clean and wrong-field-type scripts -- is gated
// behind the same .tools/toolchains/deno/ provisioning check
// scriptrun_test.go's own tests use.
package command_test

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/lnorton89/golc/internal/command"
	"github.com/lnorton89/golc/internal/scriptsdk"
	"github.com/stretchr/testify/require"
)

type scriptValidateResultView struct {
	Valid       bool `json:"valid"`
	Diagnostics []struct {
		Code     string `json:"code"`
		Message  string `json:"message"`
		Line     int    `json:"line"`
		Column   int    `json:"column"`
		Severity string `json:"severity"`
	} `json:"diagnostics"`
}

// TestScriptValidateNotFoundNeverSpawnsProcess covers: "script validate
// Missing --show <path> exits 1 with GOLC_SCRIPT_NOT_FOUND." No Deno
// install is required for this test to pass.
func TestScriptValidateNotFoundNeverSpawnsProcess(t *testing.T) {
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err, "NewDefaultCommandRegistry: %v", err)
	root := t.TempDir()
	showPath := "show.golc"

	create := registry.Execute(command.Request{Root: root, Args: []string{"script", "create", "Other", "--show", showPath}})
	require.Equal(t, 0, create.ExitCode, "script create failed: exit=%d stderr=%s", create.ExitCode, create.Stderr)

	result := registry.Execute(command.Request{Root: root, Args: []string{"script", "validate", "Missing", "--show", showPath}})
	require.Equal(t, 1, result.ExitCode, "expected ExitCode 1 for a missing script, got %d stdout=%s stderr=%s", result.ExitCode, result.Stdout, result.Stderr)
	require.Contains(t, string(result.Stderr), "GOLC_SCRIPT_NOT_FOUND", "expected GOLC_SCRIPT_NOT_FOUND, got stderr=%s", result.Stderr)
}

// TestScriptValidateShowMissingNeverSpawnsProcess covers: the route never
// runs a check when the show cannot be loaded.
func TestScriptValidateShowMissingNeverSpawnsProcess(t *testing.T) {
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err, "NewDefaultCommandRegistry: %v", err)
	root := t.TempDir()

	result := registry.Execute(command.Request{Root: root, Args: []string{"script", "validate", "Chase", "--show", "never-created.golc"}})
	require.Equal(t, 1, result.ExitCode, "expected ExitCode 1 when the show cannot be loaded, got %d stdout=%s stderr=%s", result.ExitCode, result.Stdout, result.Stderr)
}

// TestScriptValidateMalformedInvocationExitsTwo covers: "a malformed
// invocation exits 2."
func TestScriptValidateMalformedInvocationExitsTwo(t *testing.T) {
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err, "NewDefaultCommandRegistry: %v", err)
	root := t.TempDir()

	missingShow := registry.Execute(command.Request{Root: root, Args: []string{"script", "validate", "Chase"}})
	require.Equal(t, 2, missingShow.ExitCode, "expected ExitCode 2 for a missing --show, got %d stderr=%s", missingShow.ExitCode, missingShow.Stderr)

	unknownFlag := registry.Execute(command.Request{Root: root, Args: []string{"script", "validate", "Chase", "--bogus", "value", "--show", "show.golc"}})
	require.Equal(t, 2, unknownFlag.ExitCode, "expected ExitCode 2 for an unknown flag, got %d stderr=%s", unknownFlag.ExitCode, unknownFlag.Stderr)
}

// TestScriptValidateForbiddenImportNeverSpawnsProcess proves the
// structural zero-import gate is reachable end-to-end through the CLI
// route without any Deno install: a script whose source contains a real
// import is rejected with exactly one GOLC_SCRIPT_IMPORT_FORBIDDEN
// diagnostic and ExitCode 1, and this test passes with no
// .tools/toolchains/deno/ provisioning at all -- if the route had tried
// to spawn a check, it would have failed with a different, Deno-missing
// error instead.
func TestScriptValidateForbiddenImportNeverSpawnsProcess(t *testing.T) {
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err, "NewDefaultCommandRegistry: %v", err)
	root := t.TempDir()
	showPath := "show.golc"

	createScriptWithSource(t, registry, root, showPath, "Bad", `import { evil } from "./mod.ts";`)

	result := registry.Execute(command.Request{Root: root, Args: []string{"script", "validate", "Bad", "--show", showPath}})
	require.Equal(t, 1, result.ExitCode, "expected ExitCode 1 for a script with a forbidden import, got %d stdout=%s stderr=%s", result.ExitCode, result.Stdout, result.Stderr)

	var view scriptValidateResultView
	err = json.Unmarshal(result.Stdout, &view)
	require.NoError(t, err, "unmarshal script validate output: %v stdout=%s", err, result.Stdout)
	require.False(t, view.Valid, "expected valid=false")
	require.Len(t, view.Diagnostics, 1, "expected exactly one GOLC_SCRIPT_IMPORT_FORBIDDEN diagnostic, got %+v", view.Diagnostics)
	require.Equal(t, "GOLC_SCRIPT_IMPORT_FORBIDDEN", view.Diagnostics[0].Code, "expected exactly one GOLC_SCRIPT_IMPORT_FORBIDDEN diagnostic, got %+v", view.Diagnostics)
}

// TestScriptValidateClassifiedAsExcluded proves "script validate" is
// itself classified in scriptsdk's excludedRoutes -- a running script
// must not be able to validate or introspect other scripts through its
// own SDK. TestEveryDeclaredRouteIsClassified (scriptsdk_parity_test.go)
// is the build-breaking completeness gate; this test additionally pins
// the exact expected reason text.
func TestScriptValidateClassifiedAsExcluded(t *testing.T) {
	reasons := scriptsdk.RegisteredExclusions()
	reason, excluded := reasons["script validate"]
	require.True(t, excluded, `expected "script validate" to be classified in scriptsdk's excludedRoutes, not exposed as an SDK method`)
	require.Contains(t, reason, "validate or introspect other scripts", "expected the exclusion reason to explain why, got %q", reason)
}

// TestScriptValidateCleanScript covers: "script validate <name> --show
// <path> exits 0 with {"valid":true,"diagnostics":[]} for a clean
// script."
func TestScriptValidateCleanScript(t *testing.T) {
	root := skipUnlessDenoProvisionedForCommandTest(t)
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err, "NewDefaultCommandRegistry: %v", err)
	// An absolute temp path, not one relative to root: root is the real
	// repository root here (needed for Deno toolchain resolution), and
	// this show must never collide with a repeat run's own script names
	// against a stray repo-root show.golc.
	showPath := filepath.Join(t.TempDir(), "show.golc")

	createScriptWithSource(t, registry, root, showPath, "Clean", `await golc.scene.activate({ name: "Alpha", show: "ignored" });`)

	result := registry.Execute(command.Request{Root: root, Args: []string{"script", "validate", "Clean", "--show", showPath}})
	require.Equal(t, 0, result.ExitCode, "expected ExitCode 0 for a clean script, got %d stdout=%s stderr=%s", result.ExitCode, result.Stdout, result.Stderr)

	var view scriptValidateResultView
	err = json.Unmarshal(result.Stdout, &view)
	require.NoError(t, err, "unmarshal script validate output: %v stdout=%s", err, result.Stdout)
	require.True(t, view.Valid, "expected valid:true with zero diagnostics, got %+v", view)
	require.Empty(t, view.Diagnostics, "expected valid:true with zero diagnostics, got %+v", view)
}

// TestScriptValidateWrongFieldTypeScript covers: "script validate <name>
// --show <path> exits 1 with the diagnostic array for a failing one" and
// proves the .d.ts is actually loaded (a wrong-typed field is a real type
// error, not a nominal pass).
func TestScriptValidateWrongFieldTypeScript(t *testing.T) {
	root := skipUnlessDenoProvisionedForCommandTest(t)
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err, "NewDefaultCommandRegistry: %v", err)
	// See TestScriptValidateCleanScript.
	showPath := filepath.Join(t.TempDir(), "show.golc")

	createScriptWithSource(t, registry, root, showPath, "WrongField", `await golc.scene.activate({ wrongField: 1 });`)

	result := registry.Execute(command.Request{Root: root, Args: []string{"script", "validate", "WrongField", "--show", showPath}})
	require.Equal(t, 1, result.ExitCode, "expected ExitCode 1 for a wrong-field-type script, got %d stdout=%s stderr=%s", result.ExitCode, result.Stdout, result.Stderr)

	var view scriptValidateResultView
	err = json.Unmarshal(result.Stdout, &view)
	require.NoError(t, err, "unmarshal script validate output: %v stdout=%s", err, result.Stdout)
	require.False(t, view.Valid, "expected valid:false with at least one diagnostic, got %+v", view)
	require.NotEmpty(t, view.Diagnostics, "expected valid:false with at least one diagnostic, got %+v", view)
}
