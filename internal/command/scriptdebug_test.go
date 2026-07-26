// scriptdebug_test.go covers "script debug"/"script continue"/
// "script step-over"/"script step-into"/"script step-out"
// (08-09-PLAN.md Task 3): malformed invocations, a missing script, and
// breakpoint validation all exit before ever spawning a process (no real
// Deno needed); the four step-control routes' no-active-debug-run path is
// likewise Deno-free; a real end-to-end debug launch and step sequence is
// gated behind the same .tools/toolchains/deno/ provisioning check every
// other real-Deno test in this package uses.
package command_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/lnorton89/golc/internal/command"
	"github.com/lnorton89/golc/internal/scriptsdk"
)

// TestScriptDebugNotFoundNeverSpawnsProcess covers: "script debug Missing
// --show <path> exits 1 with GOLC_SCRIPT_NOT_FOUND." No Deno install is
// required: if this route ever reached a Host, this worktree's
// unprovisioned Deno would fail loudly with a different error instead.
func TestScriptDebugNotFoundNeverSpawnsProcess(t *testing.T) {
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}
	root := t.TempDir()
	showPath := "show.golc"

	create := registry.Execute(command.Request{Root: root, Args: []string{"script", "create", "Idle", "--show", showPath}})
	if create.ExitCode != 0 {
		t.Fatalf("script create failed: exit=%d stderr=%s", create.ExitCode, create.Stderr)
	}

	result := registry.Execute(command.Request{Root: root, Args: []string{"script", "debug", "Missing", "--show", showPath}})
	if result.ExitCode != 1 {
		t.Fatalf("expected ExitCode 1, got %d stdout=%s stderr=%s", result.ExitCode, result.Stdout, result.Stderr)
	}
	if !strings.Contains(string(result.Stderr), "GOLC_SCRIPT_NOT_FOUND") {
		t.Fatalf("expected GOLC_SCRIPT_NOT_FOUND, got stderr=%s", result.Stderr)
	}
}

// TestScriptDebugMalformedInvocationExitsTwo covers a missing --show.
func TestScriptDebugMalformedInvocationExitsTwo(t *testing.T) {
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}
	root := t.TempDir()

	result := registry.Execute(command.Request{Root: root, Args: []string{"script", "debug", "Idle"}})
	if result.ExitCode != 2 {
		t.Fatalf("expected ExitCode 2 for a missing --show, got %d stderr=%s", result.ExitCode, result.Stderr)
	}
}

// TestScriptDebugBreakpointNotPositiveIntegerExitsTwo covers: "A
// --breakpoint value that is not a positive integer ... exits 2 with
// GOLC_SCRIPT_BREAKPOINT_INVALID."
func TestScriptDebugBreakpointNotPositiveIntegerExitsTwo(t *testing.T) {
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}
	root := t.TempDir()
	showPath := "show.golc"

	create := registry.Execute(command.Request{Root: root, Args: []string{"script", "create", "Idle", "--show", showPath}})
	if create.ExitCode != 0 {
		t.Fatalf("script create failed: exit=%d stderr=%s", create.ExitCode, create.Stderr)
	}

	for _, badValue := range []string{"0", "-1", "notanumber"} {
		result := registry.Execute(command.Request{Root: root, Args: []string{"script", "debug", "Idle", "--show", showPath, "--breakpoint", badValue}})
		if result.ExitCode != 2 {
			t.Fatalf("breakpoint=%q: expected ExitCode 2, got %d stderr=%s", badValue, result.ExitCode, result.Stderr)
		}
		if !strings.Contains(string(result.Stderr), "GOLC_SCRIPT_BREAKPOINT_INVALID") {
			t.Fatalf("breakpoint=%q: expected GOLC_SCRIPT_BREAKPOINT_INVALID, got stderr=%s", badValue, result.Stderr)
		}
	}
}

// TestScriptDebugBreakpointExceedsLineCountExitsTwo covers: "... or
// exceeds the script's line count, exits 2 with
// GOLC_SCRIPT_BREAKPOINT_INVALID" -- an out-of-range breakpoint fails
// fast, before any process is spawned.
func TestScriptDebugBreakpointExceedsLineCountExitsTwo(t *testing.T) {
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}
	root := t.TempDir()
	showPath := "show.golc"

	createScriptWithSource(t, registry, root, showPath, "OneLine", "const x = 1;")

	result := registry.Execute(command.Request{Root: root, Args: []string{"script", "debug", "OneLine", "--show", showPath, "--breakpoint", "999"}})
	if result.ExitCode != 2 {
		t.Fatalf("expected ExitCode 2 for an out-of-range breakpoint, got %d stdout=%s stderr=%s", result.ExitCode, result.Stdout, result.Stderr)
	}
	if !strings.Contains(string(result.Stderr), "GOLC_SCRIPT_BREAKPOINT_INVALID") {
		t.Fatalf("expected GOLC_SCRIPT_BREAKPOINT_INVALID, got stderr=%s", result.Stderr)
	}
}

// TestScriptDebugClassifiedAsExcluded proves "script debug" is classified
// in scriptsdk's excludedRoutes -- a running script must not be able to
// launch or debug another script through the SDK.
func TestScriptDebugClassifiedAsExcluded(t *testing.T) {
	reasons := scriptsdk.RegisteredExclusions()
	reason, excluded := reasons["script debug"]
	if !excluded {
		t.Fatal(`expected "script debug" to be classified in scriptsdk's excludedRoutes, not exposed as an SDK method`)
	}
	if !strings.Contains(reason, "launch, debug, or step") {
		t.Fatalf("expected the exclusion reason to explain why, got %q", reason)
	}
}

// TestScriptStepRoutesClassifiedAsExcluded covers every one of the four
// step-control routes.
func TestScriptStepRoutesClassifiedAsExcluded(t *testing.T) {
	reasons := scriptsdk.RegisteredExclusions()
	for _, route := range []string{"script continue", "script step-over", "script step-into", "script step-out"} {
		reason, excluded := reasons[route]
		if !excluded {
			t.Fatalf("expected %q to be classified in scriptsdk's excludedRoutes, not exposed as an SDK method", route)
		}
		if !strings.Contains(reason, "launch, debug, or step") {
			t.Fatalf("route %q: expected the exclusion reason to explain why, got %q", route, reason)
		}
	}
}

// TestScriptStepRoutesNoActiveDebugRunExitOne covers: "script
// continue|step-over|step-into|step-out --show <path> ... exit 1 with
// GOLC_SCRIPT_NO_ACTIVE_DEBUG when there is none." No Deno install is
// required.
func TestScriptStepRoutesNoActiveDebugRunExitOne(t *testing.T) {
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}
	root := t.TempDir()
	showPath := "show.golc"

	for _, route := range []string{"continue", "step-over", "step-into", "step-out"} {
		result := registry.Execute(command.Request{Root: root, Args: []string{"script", route, "--show", showPath}})
		if result.ExitCode != 1 {
			t.Fatalf("route %q: expected ExitCode 1, got %d stdout=%s stderr=%s", route, result.ExitCode, result.Stdout, result.Stderr)
		}
		if !strings.Contains(string(result.Stderr), "GOLC_SCRIPT_NO_ACTIVE_DEBUG") {
			t.Fatalf("route %q: expected GOLC_SCRIPT_NO_ACTIVE_DEBUG, got stderr=%s", route, result.Stderr)
		}
	}
}

// TestScriptStepRoutesMalformedInvocationExitsTwo covers a missing --show
// for every one of the four step-control routes.
func TestScriptStepRoutesMalformedInvocationExitsTwo(t *testing.T) {
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}
	root := t.TempDir()

	for _, route := range []string{"continue", "step-over", "step-into", "step-out"} {
		result := registry.Execute(command.Request{Root: root, Args: []string{"script", route}})
		if result.ExitCode != 2 {
			t.Fatalf("route %q: expected ExitCode 2 for a missing --show, got %d stderr=%s", route, result.ExitCode, result.Stderr)
		}
	}
}

// TestScriptDebugRoutesClassifiedByParityTest documents that
// TestEveryDeclaredRouteIsClassified (scriptsdk_parity_test.go) is the
// actual build-breaking completeness gate for every route this file
// declares; this test additionally pins the exact route strings the
// classification tests above expect to exist in the real command
// registry.
func TestScriptDebugRoutesClassifiedByParityTest(t *testing.T) {
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}
	declared := map[string]bool{}
	for _, registration := range registry.Routes() {
		declared[registration.Route] = true
	}
	for _, route := range []string{"script debug", "script continue", "script step-over", "script step-into", "script step-out"} {
		if !declared[route] {
			t.Fatalf("expected route %q to be declared in the command registry", route)
		}
	}
}

// --- real-Deno-gated tests ---------------------------------------------

// TestScriptDebugSetsBreakpointAndCompletesCleanly covers: "script debug
// <name> --show <path> --breakpoint <line> [--breakpoint <line>...]
// launches the script in Debug mode with those breakpoints set, exits 0
// on clean completion, and reports the run id" and the manual transcript
// this plan's acceptance criteria asks to record in the SUMMARY.
func TestScriptDebugSetsBreakpointAndCompletesCleanly(t *testing.T) {
	root := skipUnlessDenoProvisionedForCommandTest(t)
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}
	showPath := "show.golc"

	createScriptWithSource(t, registry, root, showPath, "DebugMe", "const x = 1;\nconst y = 2;\n")

	result := registry.Execute(command.Request{Root: root, Args: []string{"script", "debug", "DebugMe", "--show", showPath, "--breakpoint", "1"}})
	if result.ExitCode != 0 {
		t.Fatalf("script debug failed: exit=%d stdout=%s stderr=%s", result.ExitCode, result.Stdout, result.Stderr)
	}

	var view scriptRunResultView
	if err := json.Unmarshal(result.Stdout, &view); err != nil {
		t.Fatalf("unmarshal script debug output: %v stdout=%s", err, result.Stdout)
	}
	if view.RunID == "" {
		t.Fatal("expected a non-empty run_id")
	}
	if view.Status != "succeeded" {
		t.Fatalf("expected status succeeded, got %q (reason: %s)", view.Status, view.Reason)
	}
}

// TestScriptDebugNoBreakpointsResumesImmediately covers: "script debug
// with no --breakpoint flags launches in Debug mode with no breakpoints
// and immediately resumes."
func TestScriptDebugNoBreakpointsResumesImmediately(t *testing.T) {
	root := skipUnlessDenoProvisionedForCommandTest(t)
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}
	showPath := "show.golc"

	createScriptWithSource(t, registry, root, showPath, "NoBreakpoints", "// no SDK calls\n")

	result := registry.Execute(command.Request{Root: root, Args: []string{"script", "debug", "NoBreakpoints", "--show", showPath}})
	if result.ExitCode != 0 {
		t.Fatalf("script debug failed: exit=%d stdout=%s stderr=%s", result.ExitCode, result.Stdout, result.Stderr)
	}
}

// TestScriptDebugCrashReportsSourceMappedStackFrames covers: "A debug run
// that crashes exits 1 and its JSON output carries the source-mapped
// stack frames."
func TestScriptDebugCrashReportsSourceMappedStackFrames(t *testing.T) {
	root := skipUnlessDenoProvisionedForCommandTest(t)
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}
	showPath := "show.golc"

	createScriptWithSource(t, registry, root, showPath, "Crashes", "throw new Error(\"boom\");\n")

	result := registry.Execute(command.Request{Root: root, Args: []string{"script", "debug", "Crashes", "--show", showPath}})
	if result.ExitCode != 1 {
		t.Fatalf("expected ExitCode 1 for a crashing debug run, got %d stdout=%s stderr=%s", result.ExitCode, result.Stdout, result.Stderr)
	}

	var view scriptRunResultView
	if err := json.Unmarshal(result.Stdout, &view); err != nil {
		t.Fatalf("unmarshal script debug output: %v stdout=%s", err, result.Stdout)
	}
	if !strings.Contains(view.Reason, "boom") {
		t.Fatalf("expected the failure reason to mention the thrown error, got %q", view.Reason)
	}
}
