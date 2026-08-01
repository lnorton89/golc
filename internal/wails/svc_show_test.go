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
	"testing"

	"github.com/stretchr/testify/require"

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
	require.NoError(t, err, "Inspect before save")

	result := svc.Save()
	require.Equal(t, 0, result.ExitCode, "Save failed: stderr=%s", result.Stderr)

	after, err := svc.Inspect()
	require.NoError(t, err, "Inspect after save")
	require.Greater(t, after.Revision, before.Revision, "expected Save to bump Revision")
	require.NotEmpty(t, after.ShowPath, "expected Inspect to echo the working show path")
}

// TestShowServiceSaveAsCopiesWithoutMutatingSource proves SaveAs writes a
// copy at destPath while leaving the working show's own revision untouched
// (internal/command's runShowSaveAs never re-saves the source).
func TestShowServiceSaveAsCopiesWithoutMutatingSource(t *testing.T) {
	svc, root, _ := newTestShowService(t)

	result := svc.Save()
	require.Equal(t, 0, result.ExitCode, "initial Save failed: stderr=%s", result.Stderr)
	before, err := svc.Inspect()
	require.NoError(t, err, "Inspect before save-as")

	destPath := filepath.Join(root, "copy.golc")
	result = svc.SaveAs(destPath)
	require.Equal(t, 0, result.ExitCode, "SaveAs failed: stderr=%s", result.Stderr)

	after, err := svc.Inspect()
	require.NoError(t, err, "Inspect after save-as")
	require.Equal(t, before.Revision, after.Revision, "expected SaveAs to leave the source revision untouched")

	copySvc := NewShowService("", root, destPath)
	copyView, err := copySvc.Inspect()
	require.NoError(t, err, "Inspect on the save-as destination")
	// show.Save always increments Revision on write (store.go's own doc
	// comment), including the write SaveAs performs at destPath -- so the
	// copy lands one revision beyond the source it was loaded from.
	require.Equal(t, before.Revision+1, copyView.Revision, "expected the copy's revision to be the source's plus one")
}

// TestShowServiceInspectProjectsPoolsAndDeployments proves Inspect's
// pool/deployment projection reflects real mutations made through the
// existing "pool create"/"deployment create" routes -- the identical
// projection "show inspect" already prints, reshaped to camelCase.
func TestShowServiceInspectProjectsPoolsAndDeployments(t *testing.T) {
	svc, root, showPath := newTestShowService(t)

	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err, "NewDefaultCommandRegistry")
	result := registry.Execute(command.Request{Root: root, Args: []string{"pool", "create", "Wash", "--show", showPath}})
	require.Equal(t, 0, result.ExitCode, "pool create failed: stderr=%s", result.Stderr)
	result = registry.Execute(command.Request{Root: root, Args: []string{"deployment", "create", "Main Rig", "--show", showPath}})
	require.Equal(t, 0, result.ExitCode, "deployment create failed: stderr=%s", result.Stderr)

	view, err := svc.Inspect()
	require.NoError(t, err, "Inspect")
	require.Len(t, view.Pools, 1, "expected exactly one pool named Wash, got %+v", view.Pools)
	require.Equal(t, "Wash", view.Pools[0].Name)
	require.Len(t, view.Deployments, 1, "expected exactly one deployment named Main Rig, got %+v", view.Deployments)
	require.Equal(t, "Main Rig", view.Deployments[0].Name)
}

// TestShowServiceDiagnoseHealthyFile proves Diagnose reports a freshly
// saved show as structurally healthy with no file-level issues.
func TestShowServiceDiagnoseHealthyFile(t *testing.T) {
	svc, _, _ := newTestShowService(t)
	result := svc.Save()
	require.Equal(t, 0, result.ExitCode, "Save failed: stderr=%s", result.Stderr)

	report, err := svc.Diagnose()
	require.NoError(t, err, "Diagnose")
	require.True(t, report.StructuralOK, "expected a freshly saved show to be structurally OK, got %+v", report)
	require.Empty(t, report.FileLevelIssues, "expected no file-level issues on a healthy file")
	require.False(t, report.MigrationRequired, "expected a freshly saved show to need no migration, got %+v", report)
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
	result := svc.Save()
	require.Equal(t, 0, result.ExitCode, "Save failed: stderr=%s", result.Stderr)

	report, err := svc.Diagnose()
	require.NoError(t, err, "Diagnose")

	encoded, err := json.Marshal(report)
	require.NoError(t, err, "json.Marshal(report)")

	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal(encoded, &decoded), "json.Unmarshal")

	raw, present := decoded["fileLevelIssues"]
	require.True(t, present, "expected the JSON payload to carry a \"fileLevelIssues\" key even on a healthy show, got %s", encoded)
	require.NotNil(t, raw, "expected \"fileLevelIssues\" to be a JSON array, got JSON null: %s", encoded)
	issues, ok := raw.([]interface{})
	require.True(t, ok, "expected \"fileLevelIssues\" to decode as a JSON array, got %T: %s", raw, encoded)
	require.Empty(t, issues, "expected zero file-level issues on a healthy show")
}

// TestShowServiceRecoveryDetectEmptyOnCleanShow proves a cleanly saved show
// (no interrupted session) offers zero recovery points.
func TestShowServiceRecoveryDetectEmptyOnCleanShow(t *testing.T) {
	svc, _, _ := newTestShowService(t)
	result := svc.Save()
	require.Equal(t, 0, result.ExitCode, "Save failed: stderr=%s", result.Stderr)

	points, err := svc.DetectRecoveryPoints()
	require.NoError(t, err, "DetectRecoveryPoints")
	require.Empty(t, points, "expected zero offered recovery points on a cleanly saved show")
}

// TestShowServiceAcceptRecoveryPointRejectsUnknownID proves AcceptRecoveryPoint
// wires through to show.AcceptRecoveryPoint's own stale/unknown-id guard
// rather than silently succeeding or panicking.
func TestShowServiceAcceptRecoveryPointRejectsUnknownID(t *testing.T) {
	svc, _, _ := newTestShowService(t)
	result := svc.Save()
	require.Equal(t, 0, result.ExitCode, "Save failed: stderr=%s", result.Stderr)

	result = svc.AcceptRecoveryPoint(999)
	require.NotEqual(t, 0, result.ExitCode, "expected accepting an unknown recovery point id to fail, got %+v", result)
	require.Contains(t, result.Stderr, "GOLC_SHOW_RECOVERY_NOT_FOUND")
}

// TestShowServiceDiscardRecoveryPointsNoOpOnCleanShow proves discarding when
// nothing is offered succeeds as a no-op (mirrors show.DiscardRecoveryPoints's
// own "delete matching rows, zero rows is not an error" contract).
func TestShowServiceDiscardRecoveryPointsNoOpOnCleanShow(t *testing.T) {
	svc, _, _ := newTestShowService(t)
	result := svc.Save()
	require.Equal(t, 0, result.ExitCode, "Save failed: stderr=%s", result.Stderr)

	result = svc.DiscardRecoveryPoints()
	require.Equal(t, 0, result.ExitCode, "expected discarding on a clean show to no-op successfully, stderr=%s", result.Stderr)
}
