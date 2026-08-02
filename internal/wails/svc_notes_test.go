// svc_notes_test.go proves NotesService's bindings for NotesWorkspace.tsx:
// ListNotes/GetNote/CreateNote/SaveNote/DeleteNote execute the real
// "note"* CLI routes (internal/command/note.go already carries exhaustive
// route-level coverage in internal/command/note_test.go) -- this file only
// proves the binding itself round-trips (mirrors svc_show_test.go's
// identical "binding wiring, not domain re-proof" scope), plus a pinned
// never-null-slice regression test for ListNotes mirroring
// TestShowServiceDiagnosticReportNeverOmitsOrNullsFileLevelIssues.
package wails

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// newTestNotesService constructs a NotesService against a fresh per-test
// root/show path, mirroring newTestShowService's identical
// seed-then-exercise-bindings convention.
func newTestNotesService(t *testing.T) (*NotesService, string, string) {
	t.Helper()
	root := t.TempDir()
	showPath := filepath.Join(t.TempDir(), "show.golc")
	return NewNotesService("", root, showPath), root, showPath
}

func TestNotesServiceCreateListGetDeleteRoundTrip(t *testing.T) {
	svc, _, _ := newTestNotesService(t)

	result := svc.CreateNote("Load-In Checklist")
	require.Equal(t, 0, result.ExitCode, "CreateNote failed: stderr=%s", result.Stderr)

	list, err := svc.ListNotes()
	require.NoError(t, err, "ListNotes")
	require.Len(t, list, 1, "expected exactly one note")
	require.Equal(t, "Load-In Checklist", list[0].Title)
	require.NotEmpty(t, list[0].ID)

	detail, err := svc.GetNote("Load-In Checklist")
	require.NoError(t, err, "GetNote")
	require.Equal(t, list[0].ID, detail.ID)
	require.Empty(t, detail.Body, "expected a freshly created note to have an empty body")
	require.NotEmpty(t, detail.CreatedAt)

	deleteResult := svc.DeleteNote("Load-In Checklist")
	require.Equal(t, 0, deleteResult.ExitCode, "DeleteNote failed: stderr=%s", deleteResult.Stderr)

	afterDelete, err := svc.ListNotes()
	require.NoError(t, err, "ListNotes after delete")
	require.Empty(t, afterDelete, "expected no notes after delete")
}

func TestNotesServiceSaveNoteUpdatesTitleAndBody(t *testing.T) {
	svc, _, _ := newTestNotesService(t)

	create := svc.CreateNote("Load-In Checklist")
	require.Equal(t, 0, create.ExitCode, "CreateNote failed: stderr=%s", create.Stderr)

	save := svc.SaveNote("Load-In Checklist", "Load-In & Strike Checklist", "<p>Deliberately <strong>preserved</strong> markup.</p>")
	require.Equal(t, 0, save.ExitCode, "SaveNote failed: stderr=%s", save.Stderr)

	detail, err := svc.GetNote("Load-In & Strike Checklist")
	require.NoError(t, err, "GetNote")
	require.Equal(t, "<p>Deliberately <strong>preserved</strong> markup.</p>", detail.Body, "expected body to round-trip byte-for-byte")
}

func TestNotesServiceSaveNoteRejectsOversizedBody(t *testing.T) {
	svc, _, _ := newTestNotesService(t)

	create := svc.CreateNote("Load-In Checklist")
	require.Equal(t, 0, create.ExitCode, "CreateNote failed: stderr=%s", create.Stderr)

	oversized := strings.Repeat("a", maxNoteBodyBytes+1)
	save := svc.SaveNote("Load-In Checklist", "Load-In Checklist", oversized)
	require.True(t, save.ExitCode == 1 && strings.Contains(save.Stderr, "GOLC_NOTE_BODY_TOO_LARGE"),
		"expected GOLC_NOTE_BODY_TOO_LARGE for an oversized body, got exit=%d stderr=%s", save.ExitCode, save.Stderr)
}

func TestNotesServiceGetNoteUnknownName(t *testing.T) {
	svc, _, _ := newTestNotesService(t)

	_, err := svc.GetNote("Missing")
	require.ErrorContains(t, err, "GOLC_NOTE_NOT_FOUND")
}

// TestNotesServiceListNotesNeverNullsSlice proves ListNotes' actual
// over-the-wire JSON shape -- not just the Go slice's in-memory value --
// always carries a present JSON array, never a JSON "null", on a show with
// no notes. Wails marshals every bound method's return value through plain
// encoding/json.Marshal before it ever reaches the frontend, and
// encoding/json marshals a nil slice as JSON "null" -- which would crash
// NotesWorkspace.tsx's .map()/.length calls with no error boundary
// anywhere in this app. Mirrors
// TestShowServiceDiagnosticReportNeverOmitsOrNullsFileLevelIssues' exact
// rationale and technique.
func TestNotesServiceListNotesNeverNullsSlice(t *testing.T) {
	svc, _, _ := newTestNotesService(t)

	list, err := svc.ListNotes()
	require.NoError(t, err, "ListNotes")
	require.NotNil(t, list, "expected ListNotes to return a non-nil slice")

	encoded, err := json.Marshal(list)
	require.NoError(t, err, "json.Marshal(list)")
	require.Equal(t, "[]", string(encoded), "expected an empty notes list to marshal as [], never null")
}
