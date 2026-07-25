// svc_show_test.go proves ShowService's bindings for OverviewWorkspace.tsx
// and SaveRecoveryWorkspace.tsx: Save/SaveAs execute the real "show
// save"/"show save-as" CLI routes, Inspect/Diagnose project the working
// show's real state, and DetectRecoveryPoints/AcceptRecoveryPoint/
// DiscardRecoveryPoints wire straight through to internal/show's own
// recovery functions -- exhaustive coverage of those functions' own
// invariants (interrupted-session detection, stale-id rejection, blob
// validation) already lives in internal/show/recovery_test.go; this file
// only proves the binding itself round-trips (mirrors svc_surface_test.go's
// identical "binding wiring, not domain re-proof" scope).
package wails

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lnorton89/golc/internal/command"
)

// newTestShowService constructs a ShowService against a fresh per-test
// root/show path, mirroring newTestProgrammingService's identical
// seed-then-exercise-bindings convention.
func newTestShowService(t *testing.T) (*ShowService, string, string) {
	t.Helper()
	root := t.TempDir()
	showPath := filepath.Join(t.TempDir(), "show.golc")
	return NewShowService("", root, showPath), root, showPath
}

// TestShowServiceSaveBumpsRevision proves Save executes the real "show
// save" route: a fresh show starts at revision 0, and each Save bumps it.
func TestShowServiceSaveBumpsRevision(t *testing.T) {
	svc, _, _ := newTestShowService(t)

	before, err := svc.Inspect()
	if err != nil {
		t.Fatalf("Inspect before save: %v", err)
	}

	if result := svc.Save(); result.ExitCode != 0 {
		t.Fatalf("Save failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	after, err := svc.Inspect()
	if err != nil {
		t.Fatalf("Inspect after save: %v", err)
	}
	if after.Revision <= before.Revision {
		t.Fatalf("expected Save to bump Revision beyond %d, got %d", before.Revision, after.Revision)
	}
	if after.ShowPath == "" {
		t.Fatalf("expected Inspect to echo the working show path, got empty")
	}
}

// TestShowServiceSaveAsCopiesWithoutMutatingSource proves SaveAs writes a
// copy at destPath while leaving the working show's own revision untouched
// (internal/command's runShowSaveAs never re-saves the source).
func TestShowServiceSaveAsCopiesWithoutMutatingSource(t *testing.T) {
	svc, root, _ := newTestShowService(t)

	if result := svc.Save(); result.ExitCode != 0 {
		t.Fatalf("initial Save failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
	before, err := svc.Inspect()
	if err != nil {
		t.Fatalf("Inspect before save-as: %v", err)
	}

	destPath := filepath.Join(root, "copy.golc")
	if result := svc.SaveAs(destPath); result.ExitCode != 0 {
		t.Fatalf("SaveAs failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	after, err := svc.Inspect()
	if err != nil {
		t.Fatalf("Inspect after save-as: %v", err)
	}
	if after.Revision != before.Revision {
		t.Fatalf("expected SaveAs to leave the source revision at %d untouched, got %d", before.Revision, after.Revision)
	}

	copySvc := NewShowService("", root, destPath)
	copyView, err := copySvc.Inspect()
	if err != nil {
		t.Fatalf("Inspect on the save-as destination: %v", err)
	}
	// show.Save always increments Revision on write (store.go's own doc
	// comment), including the write SaveAs performs at destPath -- so the
	// copy lands one revision beyond the source it was loaded from.
	if copyView.Revision != before.Revision+1 {
		t.Fatalf("expected the copy's revision to be the source's %d plus one, got %d", before.Revision, copyView.Revision)
	}
}

// TestShowServiceInspectProjectsPoolsAndDeployments proves Inspect's
// pool/deployment projection reflects real mutations made through the
// existing "pool create"/"deployment create" routes -- the identical
// projection "show inspect" already prints, reshaped to camelCase.
func TestShowServiceInspectProjectsPoolsAndDeployments(t *testing.T) {
	svc, root, showPath := newTestShowService(t)

	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}
	if result := registry.Execute(command.Request{Root: root, Args: []string{"pool", "create", "Wash", "--show", showPath}}); result.ExitCode != 0 {
		t.Fatalf("pool create failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
	if result := registry.Execute(command.Request{Root: root, Args: []string{"deployment", "create", "Main Rig", "--show", showPath}}); result.ExitCode != 0 {
		t.Fatalf("deployment create failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	view, err := svc.Inspect()
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if len(view.Pools) != 1 || view.Pools[0].Name != "Wash" {
		t.Fatalf("expected exactly one pool named Wash, got %+v", view.Pools)
	}
	if len(view.Deployments) != 1 || view.Deployments[0].Name != "Main Rig" {
		t.Fatalf("expected exactly one deployment named Main Rig, got %+v", view.Deployments)
	}
}

// TestShowServiceDiagnoseHealthyFile proves Diagnose reports a freshly
// saved show as structurally healthy with no file-level issues.
func TestShowServiceDiagnoseHealthyFile(t *testing.T) {
	svc, _, _ := newTestShowService(t)
	if result := svc.Save(); result.ExitCode != 0 {
		t.Fatalf("Save failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	report, err := svc.Diagnose()
	if err != nil {
		t.Fatalf("Diagnose: %v", err)
	}
	if !report.StructuralOK {
		t.Fatalf("expected a freshly saved show to be structurally OK, got %+v", report)
	}
	if len(report.FileLevelIssues) != 0 {
		t.Fatalf("expected no file-level issues on a healthy file, got %v", report.FileLevelIssues)
	}
	if report.MigrationRequired {
		t.Fatalf("expected a freshly saved show to need no migration, got %+v", report)
	}
}

// TestShowServiceDiagnosticReportNeverOmitsOrNullsFileLevelIssues proves
// DiagnosticReportView's actual over-the-wire JSON shape -- not just the Go
// struct's in-memory field -- always carries "fileLevelIssues" as a present
// JSON array, never an omitted key or a JSON "null". This is a real
// regression test: an earlier version of DiagnosticReportView tagged
// FileLevelIssues "omitempty", and Diagnose() forwarded show.Diagnose's
// FileLevelIssues (a nil slice on a healthy show, the overwhelmingly common
// case) verbatim. Wails marshals every bound method's return value through
// plain encoding/json.Marshal (internal/frontend/dispatcher/calls.go) before
// it ever reaches the frontend, and encoding/json (a) omits an "omitempty"
// slice field entirely when it's empty, and (b) marshals a nil slice as
// JSON "null" even without "omitempty" -- either way,
// DiagnosticsWorkspace.tsx's report.fileLevelIssues.length crashed with no
// error boundary anywhere in this app (main.tsx has none), unmounting the
// whole React tree into a blank window. A hand-authored frontend test mock
// (this codebase's usual window.go.wails.* stub) can never catch this class
// of bug: it never round-trips through real Go JSON encoding, so this test
// -- marshaling the real return value the same way Wails does -- is the one
// place this contract can actually be pinned.
func TestShowServiceDiagnosticReportNeverOmitsOrNullsFileLevelIssues(t *testing.T) {
	svc, _, _ := newTestShowService(t)
	if result := svc.Save(); result.ExitCode != 0 {
		t.Fatalf("Save failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	report, err := svc.Diagnose()
	if err != nil {
		t.Fatalf("Diagnose: %v", err)
	}

	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("json.Marshal(report): %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	raw, present := decoded["fileLevelIssues"]
	if !present {
		t.Fatalf("expected the JSON payload to carry a \"fileLevelIssues\" key even on a healthy show, got %s", encoded)
	}
	if raw == nil {
		t.Fatalf("expected \"fileLevelIssues\" to be a JSON array, got JSON null: %s", encoded)
	}
	issues, ok := raw.([]interface{})
	if !ok {
		t.Fatalf("expected \"fileLevelIssues\" to decode as a JSON array, got %T: %s", raw, encoded)
	}
	if len(issues) != 0 {
		t.Fatalf("expected zero file-level issues on a healthy show, got %v", issues)
	}
}

// TestShowServiceRecoveryDetectEmptyOnCleanShow proves a cleanly saved show
// (no interrupted session) offers zero recovery points.
func TestShowServiceRecoveryDetectEmptyOnCleanShow(t *testing.T) {
	svc, _, _ := newTestShowService(t)
	if result := svc.Save(); result.ExitCode != 0 {
		t.Fatalf("Save failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	points, err := svc.DetectRecoveryPoints()
	if err != nil {
		t.Fatalf("DetectRecoveryPoints: %v", err)
	}
	if len(points) != 0 {
		t.Fatalf("expected zero offered recovery points on a cleanly saved show, got %+v", points)
	}
}

// TestShowServiceAcceptRecoveryPointRejectsUnknownID proves AcceptRecoveryPoint
// wires through to show.AcceptRecoveryPoint's own stale/unknown-id guard
// rather than silently succeeding or panicking.
func TestShowServiceAcceptRecoveryPointRejectsUnknownID(t *testing.T) {
	svc, _, _ := newTestShowService(t)
	if result := svc.Save(); result.ExitCode != 0 {
		t.Fatalf("Save failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	result := svc.AcceptRecoveryPoint(999)
	if result.ExitCode == 0 {
		t.Fatalf("expected accepting an unknown recovery point id to fail, got %+v", result)
	}
	if !strings.Contains(result.Stderr, "GOLC_SHOW_RECOVERY_NOT_FOUND") {
		t.Fatalf("expected GOLC_SHOW_RECOVERY_NOT_FOUND, got %q", result.Stderr)
	}
}

// TestShowServiceDiscardRecoveryPointsNoOpOnCleanShow proves discarding when
// nothing is offered succeeds as a no-op (mirrors show.DiscardRecoveryPoints's
// own "delete matching rows, zero rows is not an error" contract).
func TestShowServiceDiscardRecoveryPointsNoOpOnCleanShow(t *testing.T) {
	svc, _, _ := newTestShowService(t)
	if result := svc.Save(); result.ExitCode != 0 {
		t.Fatalf("Save failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	result := svc.DiscardRecoveryPoints()
	if result.ExitCode != 0 {
		t.Fatalf("expected discarding on a clean show to no-op successfully, got exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
}
