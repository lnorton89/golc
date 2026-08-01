// svc_script_test.go proves 08-04-PLAN.md Task 1's acceptance criteria
// (SCRP-01, D-16/D-07/D-09/D-14): a Wails-bound ScriptService binds every
// "script *" CLI route (internal/command/script.go) -- create, list, show,
// edit (as SaveScriptSource), delete, and profile set -- so the desktop
// Scripts workspace and the terminal reach the exact same mutation
// implementation, never a second one (mirrors svc_programming_test.go's
// seed-drive-assert shape exactly). This file compiles against the
// already-implemented internal/command package but fails to build/pass at
// RUN time until svc_script.go declares ScriptService and its methods --
// that is the RED state Task 1 proves; svc_script.go is NOT created by
// this task.
package wails

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/script"
)

// newTestScriptService constructs a ScriptService against a fresh per-test
// root/show path, mirroring newTestProgrammingService's identical
// seed-then-exercise-bindings convention.
func newTestScriptService(t *testing.T) (*ScriptService, string, string) {
	t.Helper()
	root := t.TempDir()
	showPath := filepath.Join(t.TempDir(), "show.json")
	return NewScriptService("", root, showPath), root, showPath
}

// findScriptSummary returns a pointer to the ScriptSummaryView in views
// whose Name matches name, or nil if absent.
func findScriptSummary(views []ScriptSummaryView, name string) *ScriptSummaryView {
	for i := range views {
		if views[i].Name == name {
			return &views[i]
		}
	}
	return nil
}

// skipUnlessDenoProvisionedForWailsTest resolves the repository root from
// this package's directory (internal/wails) and skips the calling test
// when no verified Deno install exists there -- mirrors
// internal/command's identical skipUnlessDenoProvisionedForCommandTest
// helper (scriptrun_test.go).
func skipUnlessDenoProvisionedForWailsTest(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err, "resolve project root")
	if _, err := script.ResolveDenoExecutable(root); err != nil {
		t.Skipf("Deno toolchain not provisioned under %s (%v); run 'mage Bootstrap' first", root, err)
	}
	return root
}

// newDenoGatedScriptService constructs a ScriptService targeting the real
// project root (required for script.ResolveDenoExecutable to find the
// provisioned Deno binary, mirroring internal/command's own root-scoped
// Deno-gated test convention -- scriptrun_test.go's own
// createScriptWithSource against a real root) and a gitignored ("*.golc",
// .gitignore) show path unique to this test, removed via t.Cleanup so a
// real-Deno test run never leaves a stray file behind in the repository
// root.
func newDenoGatedScriptService(t *testing.T, root string) *ScriptService {
	t.Helper()
	sanitizedName := strings.ReplaceAll(strings.ReplaceAll(t.Name(), "/", "-"), " ", "-")
	showPath := "wails-svc-script-test-" + sanitizedName + ".golc"
	t.Cleanup(func() {
		_ = os.Remove(filepath.Join(root, showPath))
		_ = os.Remove(filepath.Join(root, showPath+"-wal"))
		_ = os.Remove(filepath.Join(root, showPath+"-shm"))
	})
	return NewScriptService("", root, showPath)
}

// TestScriptServiceListScriptsEmptyAndPopulated proves ListScripts returns
// an explicit empty projection for a fresh show, and reflects a created
// script's name/status/scope/preset once one exists (D-16's library-row
// projection).
func TestScriptServiceListScriptsEmptyAndPopulated(t *testing.T) {
	svc, _, _ := newTestScriptService(t)

	empty, err := svc.ListScripts()
	require.NoError(t, err, "ListScripts (empty show)")
	require.Empty(t, empty, "expected an empty projection for a fresh show, got %+v", empty)

	result := svc.CreateScript("Chase Cycler")
	require.Equal(t, 0, result.ExitCode, "CreateScript failed: stderr=%s", result.Stderr)

	views, err := svc.ListScripts()
	require.NoError(t, err, "ListScripts")
	view := findScriptSummary(views, "Chase Cycler")
	require.NotNil(t, view, "expected script %q in ListScripts, got %+v", "Chase Cycler", views)
	require.Equal(t, "never_run", view.LastRunStatus, "expected a freshly created script's LastRunStatus to be never_run")
	require.Equal(t, "playback", view.Scope, "expected a freshly created script's Scope to default to playback")
	require.Equal(t, "quick-action", view.Preset, "expected a freshly created script's Preset to default to quick-action")
}

// TestScriptServiceListScriptsMissingShow proves ListScripts surfaces an
// error rather than a panic when the show cannot be read (an invalid root
// makes show.Load fail).
func TestScriptServiceListScriptsMissingShow(t *testing.T) {
	svc := NewScriptService("", string([]byte{0}), filepath.Join(string([]byte{0}), "show.json"))
	_, err := svc.ListScripts()
	require.Error(t, err, "expected ListScripts to return an error for an unreadable show")
}

// TestScriptServiceGetScriptIncludesSource proves GetScript returns a
// ScriptDetailView including Source.
func TestScriptServiceGetScriptIncludesSource(t *testing.T) {
	svc, _, _ := newTestScriptService(t)

	result := svc.CreateScript("Chase Cycler")
	require.Equal(t, 0, result.ExitCode, "CreateScript failed: stderr=%s", result.Stderr)

	detail, err := svc.GetScript("Chase Cycler")
	require.NoError(t, err, "GetScript")
	require.Equal(t, "Chase Cycler", detail.Name)
	require.Empty(t, detail.Source, "expected a freshly created script's Source to be empty")
}

// TestScriptServiceCreateScriptRejectsDuplicateName proves CreateScript
// returns Result{ExitCode:0} on success and a Result carrying
// GOLC_SCRIPT_NAME_DUPLICATE in Stderr on a duplicate name.
func TestScriptServiceCreateScriptRejectsDuplicateName(t *testing.T) {
	svc, _, _ := newTestScriptService(t)

	first := svc.CreateScript("Chase Cycler")
	require.Equal(t, 0, first.ExitCode, "CreateScript failed: stderr=%s", first.Stderr)

	duplicate := svc.CreateScript("Chase Cycler")
	require.NotEqual(t, 0, duplicate.ExitCode, "expected a duplicate script name to be rejected")
	require.Contains(t, duplicate.Stderr, "GOLC_SCRIPT_NAME_DUPLICATE")
}

// TestScriptServiceSaveScriptSourceRoundTrips proves SaveScriptSource
// persists the source verbatim (including trailing newlines), and a round
// trip through GetScript returns identical bytes (D-14).
func TestScriptServiceSaveScriptSourceRoundTrips(t *testing.T) {
	svc, _, _ := newTestScriptService(t)

	result := svc.CreateScript("Chase Cycler")
	require.Equal(t, 0, result.ExitCode, "CreateScript failed: stderr=%s", result.Stderr)

	source := "export function run() {\n  console.log(\"hi\");\n}\n\n"
	result = svc.SaveScriptSource("Chase Cycler", source)
	require.Equal(t, 0, result.ExitCode, "SaveScriptSource failed: stderr=%s", result.Stderr)

	detail, err := svc.GetScript("Chase Cycler")
	require.NoError(t, err, "GetScript")
	require.Equal(t, source, detail.Source, "expected source to round-trip verbatim")
}

// TestScriptServiceSaveScriptSourceRejectsOversized proves SaveScriptSource
// rejects a source exceeding the 1 MiB bound with
// GOLC_SCRIPT_SOURCE_TOO_LARGE before writing anything (T-08-03/T-08-12).
func TestScriptServiceSaveScriptSourceRejectsOversized(t *testing.T) {
	svc, _, _ := newTestScriptService(t)

	result := svc.CreateScript("Chase Cycler")
	require.Equal(t, 0, result.ExitCode, "CreateScript failed: stderr=%s", result.Stderr)

	oversized := strings.Repeat("a", (1<<20)+1)
	result = svc.SaveScriptSource("Chase Cycler", oversized)
	require.NotEqual(t, 0, result.ExitCode, "expected an oversized source to be rejected")
	require.Contains(t, result.Stderr, "GOLC_SCRIPT_SOURCE_TOO_LARGE")

	detail, err := svc.GetScript("Chase Cycler")
	require.NoError(t, err, "GetScript")
	require.Empty(t, detail.Source, "expected the rejected oversized source to never persist")
}

// TestScriptServiceDeleteScriptRemovesFromList proves DeleteScript removes
// a script such that a subsequent ListScripts omits it.
func TestScriptServiceDeleteScriptRemovesFromList(t *testing.T) {
	svc, _, _ := newTestScriptService(t)

	result := svc.CreateScript("Chase Cycler")
	require.Equal(t, 0, result.ExitCode, "CreateScript failed: stderr=%s", result.Stderr)
	result = svc.CreateScript("Blackout Fade")
	require.Equal(t, 0, result.ExitCode, "CreateScript(Blackout Fade) failed: stderr=%s", result.Stderr)

	result = svc.DeleteScript("Chase Cycler")
	require.Equal(t, 0, result.ExitCode, "DeleteScript failed: stderr=%s", result.Stderr)

	views, err := svc.ListScripts()
	require.NoError(t, err, "ListScripts")
	require.Nil(t, findScriptSummary(views, "Chase Cycler"), "expected Chase Cycler to be removed, got %+v", views)
	require.NotNil(t, findScriptSummary(views, "Blackout Fade"), "expected Blackout Fade to remain, got %+v", views)
}

// TestScriptServiceSetScriptProfileForwardsOnlySuppliedFields proves
// SetScriptProfile forwards only the non-empty/positive values as flags,
// leaving unspecified profile fields untouched (D-09's partial-edit
// discipline).
func TestScriptServiceSetScriptProfileForwardsOnlySuppliedFields(t *testing.T) {
	svc, _, _ := newTestScriptService(t)

	result := svc.CreateScript("Chase Cycler")
	require.Equal(t, 0, result.ExitCode, "CreateScript failed: stderr=%s", result.Stderr)

	// Set scope+preset first.
	result = svc.SetScriptProfile("Chase Cycler", "authoring", "advanced", 45, 0, 0, 0)
	require.Equal(t, 0, result.ExitCode, "SetScriptProfile failed: stderr=%s", result.Stderr)

	afterFirst, err := svc.GetScript("Chase Cycler")
	require.NoError(t, err, "GetScript")
	require.Equal(t, "authoring", afterFirst.Scope)
	require.Equal(t, "advanced", afterFirst.Preset)
	require.Equal(t, 45, afterFirst.DeadlineSeconds)

	// A second call touching only RatePerSecond must leave Scope/Preset/
	// DeadlineSeconds untouched (zero/empty values here must NOT be
	// forwarded as flags).
	result = svc.SetScriptProfile("Chase Cycler", "", "", 0, 10, 0, 0)
	require.Equal(t, 0, result.ExitCode, "SetScriptProfile (second call) failed: stderr=%s", result.Stderr)

	afterSecond, err := svc.GetScript("Chase Cycler")
	require.NoError(t, err, "GetScript")
	require.Equal(t, "authoring", afterSecond.Scope, "expected Scope to remain authoring")
	require.Equal(t, "advanced", afterSecond.Preset, "expected Preset to remain advanced")
	require.Equal(t, 45, afterSecond.DeadlineSeconds, "expected DeadlineSeconds to remain 45")
	require.Equal(t, 10, afterSecond.RatePerSecond)
}

// TestScriptEventStreamForwardsPublishedEventsToEmit covers 08-08-PLAN.md
// Task 3's StartScriptEventStream/StopScriptEventStream: a live event
// published on script.PublishScriptEvent (internal/script/events.go)
// after the stream starts reaches EventPusher's own emit under
// "script:event", and StopScriptEventStream cleanly unblocks the
// forwarding goroutine without leaking it.
func TestScriptEventStreamForwardsPublishedEventsToEmit(t *testing.T) {
	script.ResetScriptEventsForTesting()
	t.Cleanup(script.ResetScriptEventsForTesting)

	svc, _, _ := newTestScriptService(t)

	var mu sync.Mutex
	var pushed []ScriptEventView
	received := make(chan struct{}, 1)
	svc.events.emit = func(_ context.Context, eventName string, data ...interface{}) {
		if eventName != "script:event" {
			return
		}
		if view, ok := data[0].(ScriptEventView); ok {
			mu.Lock()
			pushed = append(pushed, view)
			mu.Unlock()
			select {
			case received <- struct{}{}:
			default:
			}
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc.StartScriptEventStream(ctx)
	defer svc.StopScriptEventStream()

	runID, err := uuid.NewV7()
	require.NoError(t, err, "uuid.NewV7")
	script.PublishScriptEvent(script.ScriptEvent{
		Kind: script.ScriptEventLog, RunID: runID, ScriptName: "Chase", Message: "hello",
	})

	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-received:
			// A flush tick must actually run before emit fires (the
			// EventPusher's own ~25ms cadence) -- keep waiting for at
			// least one to land in pushed.
		case <-deadline:
			require.Fail(t, "timed out waiting for the published event to reach emit")
		}
		mu.Lock()
		found := len(pushed) > 0
		mu.Unlock()
		if found {
			break
		}
	}

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, pushed, 1, "expected exactly one forwarded event carrying the published payload, got %+v", pushed)
	require.Equal(t, "hello", pushed[0].Message)
	require.Equal(t, "Chase", pushed[0].ScriptName)
}

// TestStopScriptEventStreamBeforeStartIsNoop proves StopScriptEventStream
// is safe to call before StartScriptEventStream, mirroring
// SafetyService.StopStatusPush's own documented no-op contract.
func TestStopScriptEventStreamBeforeStartIsNoop(t *testing.T) {
	svc, _, _ := newTestScriptService(t)
	svc.StopScriptEventStream()
}

// --- 08-10-PLAN.md Task 1: Run/Debug/Stop/Validate/step-control ----------

// TestScriptServiceRunScriptMissingReturnsError proves RunScript surfaces
// "script run"'s own GOLC_SCRIPT_NOT_FOUND diagnostic as a returned error
// rather than a panic, and never spawns a process for an unknown script
// name (no Deno install required).
func TestScriptServiceRunScriptMissingReturnsError(t *testing.T) {
	svc, _, _ := newTestScriptService(t)
	result := svc.CreateScript("Other")
	require.Equal(t, 0, result.ExitCode, "CreateScript failed: stderr=%s", result.Stderr)

	_, err := svc.RunScript("Missing")
	require.ErrorContains(t, err, "GOLC_SCRIPT_NOT_FOUND", "expected RunScript to return an error for a missing script")
}

// TestScriptServiceRunScriptSucceeds proves RunScript decodes a successful
// run's outcome -- non-empty RunID, status succeeded, exactly one
// successful SDK call outcome, and at least one captured log line (D-04/
// D-05) -- gated behind a real provisioned Deno install.
func TestScriptServiceRunScriptSucceeds(t *testing.T) {
	root := skipUnlessDenoProvisionedForWailsTest(t)
	svc := newDenoGatedScriptService(t, root)

	result := svc.CreateScript("Chase")
	require.Equal(t, 0, result.ExitCode, "CreateScript failed: stderr=%s", result.Stderr)
	source := "console.log(\"running Chase\");\n" +
		"const result = await golc.show.inspect({});\n" +
		"console.log(\"inspected: \" + JSON.stringify(result));\n"
	result = svc.SaveScriptSource("Chase", source)
	require.Equal(t, 0, result.ExitCode, "SaveScriptSource failed: stderr=%s", result.Stderr)

	outcome, err := svc.RunScript("Chase")
	require.NoError(t, err, "RunScript")
	require.NotEmpty(t, outcome.RunID, "expected a non-empty RunID")
	require.Equal(t, "succeeded", outcome.Status, "expected status succeeded (reason: %s)", outcome.Reason)
	require.Len(t, outcome.Outcomes, 1, "expected exactly one successful 'show inspect' outcome, got %+v", outcome.Outcomes)
	require.True(t, outcome.Outcomes[0].Ok, "expected exactly one successful 'show inspect' outcome, got %+v", outcome.Outcomes)
	require.Equal(t, "show inspect", outcome.Outcomes[0].Route, "expected exactly one successful 'show inspect' outcome, got %+v", outcome.Outcomes)
	require.NotEmpty(t, outcome.Logs, "expected at least one captured log line")
}

// TestScriptServiceRunScriptCrashDerivesStackFrames proves a crashing
// run's Reason includes the thrown error message and that
// deriveStackFrames extracts at least one expandable trace line from it
// (D-03), gated behind a real provisioned Deno install.
func TestScriptServiceRunScriptCrashDerivesStackFrames(t *testing.T) {
	root := skipUnlessDenoProvisionedForWailsTest(t)
	svc := newDenoGatedScriptService(t, root)

	result := svc.CreateScript("Broken")
	require.Equal(t, 0, result.ExitCode, "CreateScript failed: stderr=%s", result.Stderr)
	result = svc.SaveScriptSource("Broken", "throw new Error(\"deliberate failure\");\n")
	require.Equal(t, 0, result.ExitCode, "SaveScriptSource failed: stderr=%s", result.Stderr)

	outcome, err := svc.RunScript("Broken")
	require.NoError(t, err, "RunScript")
	require.Equal(t, "failed", outcome.Status)
	require.Contains(t, outcome.Reason, "deliberate failure", "expected the reason to include the thrown error message")
	require.NotEmpty(t, outcome.StackFrames, "expected at least one derived stack-trace line for a crash, got none (reason: %q)", outcome.Reason)
}

// TestScriptServiceDebugScriptInvalidBreakpointReturnsError proves
// DebugScript surfaces "script debug"'s own GOLC_SCRIPT_BREAKPOINT_INVALID
// diagnostic for an out-of-range breakpoint before ever spawning a process
// (no Deno install required).
func TestScriptServiceDebugScriptInvalidBreakpointReturnsError(t *testing.T) {
	svc, _, _ := newTestScriptService(t)
	result := svc.CreateScript("OneLine")
	require.Equal(t, 0, result.ExitCode, "CreateScript failed: stderr=%s", result.Stderr)
	result = svc.SaveScriptSource("OneLine", "const x = 1;")
	require.Equal(t, 0, result.ExitCode, "SaveScriptSource failed: stderr=%s", result.Stderr)

	_, err := svc.DebugScript("OneLine", []int{999})
	require.ErrorContains(t, err, "GOLC_SCRIPT_BREAKPOINT_INVALID", "expected DebugScript to return an error for an out-of-range breakpoint")
}

// TestScriptServiceDebugScriptSucceeds proves DebugScript forwards
// breakpointLines as repeated --breakpoint flags and decodes a clean
// debug run's outcome, gated behind a real provisioned Deno install.
func TestScriptServiceDebugScriptSucceeds(t *testing.T) {
	root := skipUnlessDenoProvisionedForWailsTest(t)
	svc := newDenoGatedScriptService(t, root)

	result := svc.CreateScript("DebugMe")
	require.Equal(t, 0, result.ExitCode, "CreateScript failed: stderr=%s", result.Stderr)
	result = svc.SaveScriptSource("DebugMe", "const x = 1;\nconst y = 2;\n")
	require.Equal(t, 0, result.ExitCode, "SaveScriptSource failed: stderr=%s", result.Stderr)

	outcome, err := svc.DebugScript("DebugMe", []int{1})
	require.NoError(t, err, "DebugScript")
	require.NotEmpty(t, outcome.RunID, "expected a non-empty RunID")
	require.Equal(t, "succeeded", outcome.Status, "expected status succeeded (reason: %s)", outcome.Reason)
}

// TestScriptServiceStopScriptNoActiveRunReturnsResult proves StopScript
// returns the raw Result{ExitCode:1} carrying GOLC_SCRIPT_NO_ACTIVE_RUN
// when the named script has no active run (no Deno install required).
func TestScriptServiceStopScriptNoActiveRunReturnsResult(t *testing.T) {
	svc, _, _ := newTestScriptService(t)
	result := svc.CreateScript("Chase")
	require.Equal(t, 0, result.ExitCode, "CreateScript failed: stderr=%s", result.Stderr)

	result = svc.StopScript("Chase")
	require.Equal(t, 1, result.ExitCode, "expected ExitCode 1 for no active run, stdout=%s stderr=%s", result.Stdout, result.Stderr)
	require.Contains(t, result.Stderr, "GOLC_SCRIPT_NO_ACTIVE_RUN")
}

// TestScriptServiceValidateScriptDecodesForbiddenImportDiagnostic proves
// ValidateScript decodes "script validate"'s Valid/Diagnostics JSON --
// exercised via the forbidden-import structural gate, which (unlike a
// full type-check) never requires a Deno install.
func TestScriptServiceValidateScriptDecodesForbiddenImportDiagnostic(t *testing.T) {
	svc, _, _ := newTestScriptService(t)
	result := svc.CreateScript("Imports")
	require.Equal(t, 0, result.ExitCode, "CreateScript failed: stderr=%s", result.Stderr)
	result = svc.SaveScriptSource("Imports", "import \"foo\";\n")
	require.Equal(t, 0, result.ExitCode, "SaveScriptSource failed: stderr=%s", result.Stderr)

	validation, err := svc.ValidateScript("Imports")
	require.NoError(t, err, "ValidateScript")
	require.False(t, validation.Valid, "expected a script with a forbidden import to be invalid")
	require.NotEmpty(t, validation.Diagnostics, "expected at least one diagnostic")
	require.Equal(t, "GOLC_SCRIPT_IMPORT_FORBIDDEN", validation.Diagnostics[0].Code)
}

// TestScriptServiceValidateScriptMissingReturnsError proves ValidateScript
// surfaces GOLC_SCRIPT_NOT_FOUND as a returned error for an unknown script
// name (no Deno install required).
func TestScriptServiceValidateScriptMissingReturnsError(t *testing.T) {
	svc, _, _ := newTestScriptService(t)
	result := svc.CreateScript("Other")
	require.Equal(t, 0, result.ExitCode, "CreateScript failed: stderr=%s", result.Stderr)

	_, err := svc.ValidateScript("Missing")
	require.ErrorContains(t, err, "GOLC_SCRIPT_NOT_FOUND", "expected ValidateScript to return an error for a missing script")
}

// TestScriptServiceControlRoutesNoActiveDebugReturnsResult proves every
// one of ContinueScript/StepOverScript/StepIntoScript/StepOutScript
// returns Result{ExitCode:1} carrying GOLC_SCRIPT_NO_ACTIVE_DEBUG when no
// debug run is active (no Deno install required).
func TestScriptServiceControlRoutesNoActiveDebugReturnsResult(t *testing.T) {
	svc, _, _ := newTestScriptService(t)

	controls := map[string]func() Result{
		"ContinueScript": svc.ContinueScript,
		"StepOverScript": svc.StepOverScript,
		"StepIntoScript": svc.StepIntoScript,
		"StepOutScript":  svc.StepOutScript,
	}
	for name, control := range controls {
		result := control()
		require.Equal(t, 1, result.ExitCode, "%s: expected ExitCode 1, stdout=%s stderr=%s", name, result.Stdout, result.Stderr)
		require.Contains(t, result.Stderr, "GOLC_SCRIPT_NO_ACTIVE_DEBUG", "%s", name)
	}
}

// --- 08-11-PLAN.md Task 2: GetSDKTypeDefinitions (D-15) -------------------

// TestScriptServiceGetSDKTypeDefinitionsReadsCommittedFile proves
// GetSDKTypeDefinitions returns the exact bytes of root's committed
// internal/scriptsdk/generated/golc.d.ts.
func TestScriptServiceGetSDKTypeDefinitionsReadsCommittedFile(t *testing.T) {
	svc, root, _ := newTestScriptService(t)

	typesDir := filepath.Join(root, "internal", "scriptsdk", "generated")
	require.NoError(t, os.MkdirAll(typesDir, 0o755), "MkdirAll")
	want := "declare namespace golc {\n  function ping(): Promise<void>;\n}\n"
	require.NoError(t, os.WriteFile(filepath.Join(typesDir, "golc.d.ts"), []byte(want), 0o644), "WriteFile")

	got, err := svc.GetSDKTypeDefinitions()
	require.NoError(t, err, "GetSDKTypeDefinitions")
	require.Equal(t, want, got, "expected golc.d.ts contents to round-trip verbatim")
}

// TestScriptServiceGetSDKTypeDefinitionsMissingReturnsError proves a root
// with no committed golc.d.ts surfaces GOLC_SCRIPTSDK_TYPES_MISSING rather
// than a raw os.ReadFile error.
func TestScriptServiceGetSDKTypeDefinitionsMissingReturnsError(t *testing.T) {
	svc, _, _ := newTestScriptService(t)

	_, err := svc.GetSDKTypeDefinitions()
	require.ErrorContains(t, err, "GOLC_SCRIPTSDK_TYPES_MISSING", "expected GetSDKTypeDefinitions to return an error when golc.d.ts is absent")
}
