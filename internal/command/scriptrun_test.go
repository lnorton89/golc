// scriptrun_test.go covers "script run" (08-05-PLAN.md Task 3): a
// malformed invocation and a missing script both exit before ever
// spawning a process (no real Deno needed), and a real script run --
// successful and thrown-error paths -- is gated behind the same
// .tools/toolchains/deno/ provisioning check internal/script's own tests
// use, skipping with a clear message on a machine that has not
// bootstrapped.
package command_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lnorton89/golc/internal/command"
	"github.com/lnorton89/golc/internal/script"
	"github.com/lnorton89/golc/internal/scriptsdk"
)

type scriptRunResultView struct {
	RunID    string `json:"run_id"`
	Status   string `json:"status"`
	Reason   string `json:"reason"`
	Outcomes []struct {
		Method     string `json:"method"`
		Route      string `json:"route"`
		DurationMS int64  `json:"duration_ms"`
		Ok         bool   `json:"ok"`
		Code       string `json:"code"`
		Message    string `json:"message"`
	} `json:"outcomes"`
	Logs []struct {
		Level   string `json:"level"`
		Message string `json:"message"`
		Source  string `json:"source"`
	} `json:"logs"`
}

// skipUnlessDenoProvisionedForCommandTest resolves the repository root
// from this package's directory (internal/command) and skips the calling
// test when no verified Deno install exists there.
func skipUnlessDenoProvisionedForCommandTest(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve project root: %v", err)
	}
	if _, err := script.ResolveDenoExecutable(root); err != nil {
		t.Skipf("Deno toolchain not provisioned under %s (%v); run 'mage Bootstrap' first", root, err)
	}
	return root
}

// createScriptWithSource creates a script named name against showPath (in
// root) and edits its source to source, returning the registry so the
// caller can continue driving routes.
func createScriptWithSource(t *testing.T, registry *command.CommandRegistry, root, showPath, name, source string) {
	t.Helper()
	create := registry.Execute(command.Request{Root: root, Args: []string{"script", "create", name, "--show", showPath}})
	if create.ExitCode != 0 {
		t.Fatalf("script create failed: exit=%d stderr=%s", create.ExitCode, create.Stderr)
	}

	// t.TempDir() rather than root: root may be the real repository root
	// (skipUnlessDenoProvisionedForCommandTest's real-Deno-gated callers
	// need it there for toolchain resolution), and this fixture file must
	// never land in the repository working tree itself.
	sourcePath := filepath.Join(t.TempDir(), name+".ts")
	if err := os.WriteFile(sourcePath, []byte(source), 0o644); err != nil {
		t.Fatalf("write source fixture: %v", err)
	}
	edit := registry.Execute(command.Request{Root: root, Args: []string{"script", "edit", name, "--source-file", sourcePath, "--show", showPath}})
	if edit.ExitCode != 0 {
		t.Fatalf("script edit failed: exit=%d stderr=%s", edit.ExitCode, edit.Stderr)
	}
}

// TestScriptRunNotFoundNeverSpawnsProcess covers: "script run Missing
// --show <path> exits 1 with GOLC_SCRIPT_NOT_FOUND" and "The route never
// spawns a process when ... the script is missing." No Deno install is
// required for this test to pass -- if it ever spawned a process, this
// worktree's unprovisioned Deno would fail loudly with a different error
// instead.
func TestScriptRunNotFoundNeverSpawnsProcess(t *testing.T) {
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}
	root := t.TempDir()
	showPath := "show.golc"

	// Create the show (but not the "Missing" script) via a real script
	// create, so show.Load succeeds and the failure is specifically
	// GOLC_SCRIPT_NOT_FOUND, not a show-load failure.
	create := registry.Execute(command.Request{Root: root, Args: []string{"script", "create", "Other", "--show", showPath}})
	if create.ExitCode != 0 {
		t.Fatalf("script create failed: exit=%d stderr=%s", create.ExitCode, create.Stderr)
	}

	result := registry.Execute(command.Request{Root: root, Args: []string{"script", "run", "Missing", "--show", showPath}})
	if result.ExitCode != 1 {
		t.Fatalf("expected ExitCode 1 for a missing script, got %d stdout=%s stderr=%s", result.ExitCode, result.Stdout, result.Stderr)
	}
	if !strings.Contains(string(result.Stderr), "GOLC_SCRIPT_NOT_FOUND") {
		t.Fatalf("expected GOLC_SCRIPT_NOT_FOUND, got stderr=%s", result.Stderr)
	}
}

// TestScriptRunShowMissingNeverSpawnsProcess covers: "The route never
// spawns a process when the show cannot be loaded ..." -- a --show path
// that has never been created fails at show.Load, before script.NewHost
// is ever constructed.
func TestScriptRunShowMissingNeverSpawnsProcess(t *testing.T) {
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}
	root := t.TempDir()

	result := registry.Execute(command.Request{Root: root, Args: []string{"script", "run", "Chase", "--show", "never-created.golc"}})
	if result.ExitCode != 1 {
		t.Fatalf("expected ExitCode 1 when the show cannot be loaded, got %d stdout=%s stderr=%s", result.ExitCode, result.Stdout, result.Stderr)
	}
}

// TestScriptRunMalformedInvocationExitsTwo covers: "A malformed
// invocation exits 2."
func TestScriptRunMalformedInvocationExitsTwo(t *testing.T) {
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}
	root := t.TempDir()

	missingShow := registry.Execute(command.Request{Root: root, Args: []string{"script", "run", "Chase"}})
	if missingShow.ExitCode != 2 {
		t.Fatalf("expected ExitCode 2 for a missing --show, got %d stderr=%s", missingShow.ExitCode, missingShow.Stderr)
	}

	unknownFlag := registry.Execute(command.Request{Root: root, Args: []string{"script", "run", "Chase", "--bogus", "value", "--show", "show.golc"}})
	if unknownFlag.ExitCode != 2 {
		t.Fatalf("expected ExitCode 2 for an unknown flag, got %d stderr=%s", unknownFlag.ExitCode, unknownFlag.Stderr)
	}
}

// TestScriptRunSuccessfulScript covers: "script run Chase --show <path>
// on a script whose source calls one SDK method exits 0 and writes JSON
// containing run_id, status, per-call outcomes, and captured logs" plus
// "After any completed run, the persisted script's LastRunStatus/
// LastRunReason/LastRunAt reflect that run."
func TestScriptRunSuccessfulScript(t *testing.T) {
	root := skipUnlessDenoProvisionedForCommandTest(t)
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}
	// An absolute path in a fresh temp dir, not a path relative to root:
	// root is the real repository root here (needed for Deno toolchain
	// resolution), and this show must never collide with a repeat run's
	// own script names against a stray repo-root show.golc.
	showPath := filepath.Join(t.TempDir(), "show.golc")

	createScriptWithSource(t, registry, root, showPath, "Chase", `
console.log("running Chase");
const result = await golc.show.inspect({});
console.log("inspected: " + JSON.stringify(result));
`)

	run := registry.Execute(command.Request{Root: root, Args: []string{"script", "run", "Chase", "--show", showPath}})
	if run.ExitCode != 0 {
		t.Fatalf("script run failed: exit=%d stdout=%s stderr=%s", run.ExitCode, run.Stdout, run.Stderr)
	}

	var view scriptRunResultView
	if err := json.Unmarshal(run.Stdout, &view); err != nil {
		t.Fatalf("unmarshal script run output: %v stdout=%s", err, run.Stdout)
	}
	if view.RunID == "" {
		t.Fatal("expected a non-empty run_id")
	}
	if view.Status != "succeeded" {
		t.Fatalf("expected status succeeded, got %q (reason: %s)", view.Status, view.Reason)
	}
	if len(view.Outcomes) != 1 || !view.Outcomes[0].Ok || view.Outcomes[0].Route != "show inspect" {
		t.Fatalf("expected exactly one successful 'show inspect' outcome, got %+v", view.Outcomes)
	}
	if len(view.Logs) == 0 {
		t.Fatal("expected at least one captured log line")
	}

	shown := registry.Execute(command.Request{Root: root, Args: []string{"script", "show", "Chase", "--show", showPath}})
	if shown.ExitCode != 0 {
		t.Fatalf("script show (after run) failed: exit=%d stderr=%s", shown.ExitCode, shown.Stderr)
	}
	var afterRun struct {
		LastRunStatus string `json:"last_run_status"`
		LastRunAt     string `json:"last_run_at"`
	}
	if err := json.Unmarshal(shown.Stdout, &afterRun); err != nil {
		t.Fatalf("unmarshal script show output: %v", err)
	}
	if afterRun.LastRunStatus != "succeeded" {
		t.Fatalf("expected persisted last_run_status succeeded, got %q", afterRun.LastRunStatus)
	}
	if afterRun.LastRunAt == "" {
		t.Fatal("expected a non-empty persisted last_run_at")
	}
}

// TestScriptRunThrowingScriptFails covers: "script run Chase --show
// <path> on a script whose source throws exits 1, reports status:
// 'failed', and includes the error message with its source location."
func TestScriptRunThrowingScriptFails(t *testing.T) {
	root := skipUnlessDenoProvisionedForCommandTest(t)
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}
	// See TestScriptRunSuccessfulScript: an absolute temp path, not one
	// relative to the real repository root.
	showPath := filepath.Join(t.TempDir(), "show.golc")

	createScriptWithSource(t, registry, root, showPath, "Broken", `
throw new Error("deliberate failure");
`)

	run := registry.Execute(command.Request{Root: root, Args: []string{"script", "run", "Broken", "--show", showPath}})
	if run.ExitCode != 1 {
		t.Fatalf("expected ExitCode 1 for a throwing script, got %d stdout=%s stderr=%s", run.ExitCode, run.Stdout, run.Stderr)
	}

	var view scriptRunResultView
	if err := json.Unmarshal(run.Stdout, &view); err != nil {
		t.Fatalf("unmarshal script run output: %v stdout=%s", err, run.Stdout)
	}
	if view.Status != "failed" {
		t.Fatalf("expected status failed, got %q", view.Status)
	}
	if !strings.Contains(view.Reason, "deliberate failure") {
		t.Fatalf("expected the reason to include the thrown error message, got %q", view.Reason)
	}
}

// TestScriptRunClassifiedAsExcluded proves "script run" is itself
// classified in scriptsdk's excludedRoutes -- a script must not be able
// to launch another script. TestEveryDeclaredRouteIsClassified
// (scriptsdk_parity_test.go) is the build-breaking completeness gate;
// this test additionally pins the exact expected reason text.
func TestScriptRunClassifiedAsExcluded(t *testing.T) {
	reasons := scriptsdk.RegisteredExclusions()
	reason, excluded := reasons["script run"]
	if !excluded {
		t.Fatal(`expected "script run" to be classified in scriptsdk's excludedRoutes, not exposed as an SDK method`)
	}
	if !strings.Contains(reason, "must not be able to launch another script") {
		t.Fatalf("expected the exclusion reason to explain why, got %q", reason)
	}
}
