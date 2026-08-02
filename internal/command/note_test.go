// note_test.go pins the "note create"/"note list"/"note show"/"note edit"/
// "note delete" route contract: every route writes deterministic JSON, a
// duplicate create is rejected without mutating the show, "note list"
// projects the compact id/title/updated_at shape (never body), "note edit"
// persists a body file's bytes verbatim and/or renames without clobbering
// the field not mentioned, an oversized body file is rejected, and every
// malformed invocation exits 2. It follows script_test.go's exact
// route-invocation convention.
package command_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/command"
)

type noteTestView struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func TestNoteRoutesFullLifecycle(t *testing.T) {
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err, "NewDefaultCommandRegistry: %v", err)
	root := t.TempDir()
	showPath := "show.golc"

	// "note create" on a fresh show exits 0 and writes JSON containing the
	// new note's id and title.
	create := registry.Execute(command.Request{Root: root, Args: []string{"note", "create", "Load-In Checklist", "--show", showPath}})
	require.Equal(t, 0, create.ExitCode, "note create failed: exit=%d stderr=%s", create.ExitCode, create.Stderr)
	var created noteTestView
	err = json.Unmarshal(create.Stdout, &created)
	require.NoError(t, err, "unmarshal note create output: %v stdout=%s", err, create.Stdout)
	require.True(t, created.ID != "" && created.Title == "Load-In Checklist", "expected a created note with id/title, got %+v", created)
	require.Empty(t, created.Body, "expected an empty body")
	require.NotEmpty(t, created.CreatedAt, "expected created_at to be stamped")

	// A second "note create" with the same title exits 1 with
	// GOLC_NOTE_TITLE_DUPLICATE and leaves the show unchanged.
	dup := registry.Execute(command.Request{Root: root, Args: []string{"note", "create", "Load-In Checklist", "--show", showPath}})
	require.True(t, dup.ExitCode == 1 && strings.Contains(string(dup.Stderr), "GOLC_NOTE_TITLE_DUPLICATE"), "expected GOLC_NOTE_TITLE_DUPLICATE for a duplicate title, got exit=%d stderr=%s", dup.ExitCode, dup.Stderr)

	// "note list" exits 0 and writes a JSON array whose entries carry id,
	// title, and updated_at -- never body.
	list := registry.Execute(command.Request{Root: root, Args: []string{"note", "list", "--show", showPath}})
	require.Equal(t, 0, list.ExitCode, "note list failed: exit=%d stderr=%s", list.ExitCode, list.Stderr)
	var listed []struct {
		ID        string `json:"id"`
		Title     string `json:"title"`
		UpdatedAt string `json:"updated_at"`
	}
	err = json.Unmarshal(list.Stdout, &listed)
	require.NoError(t, err, "unmarshal note list output: %v stdout=%s", err, list.Stdout)
	require.True(t, len(listed) == 1 && listed[0].Title == "Load-In Checklist", "expected exactly one listed note titled Load-In Checklist, got %+v", listed)
	require.NotContains(t, string(list.Stdout), "\"body\"", "expected note list to omit body, got: %s", list.Stdout)

	// "note show" exits 0 and writes the full note including body.
	showResult := registry.Execute(command.Request{Root: root, Args: []string{"note", "show", "Load-In Checklist", "--show", showPath}})
	require.Equal(t, 0, showResult.ExitCode, "note show failed: exit=%d stderr=%s", showResult.ExitCode, showResult.Stderr)
	var shown noteTestView
	err = json.Unmarshal(showResult.Stdout, &shown)
	require.NoError(t, err, "unmarshal note show output: %v stdout=%s", err, showResult.Stdout)
	require.True(t, shown.ID == created.ID && shown.Body == "", "expected note show to return the full (empty-body) note, got %+v", shown)

	// "note edit" persists a body file's bytes verbatim and renames in the
	// same call.
	bodyPath := filepath.Join(root, "body.html")
	bodyBytes := []byte("<p>Deliberately <strong>preserved</strong> markup.</p>")
	err = os.WriteFile(bodyPath, bodyBytes, 0o644)
	require.NoError(t, err, "write body fixture: %v", err)
	edit := registry.Execute(command.Request{Root: root, Args: []string{
		"note", "edit", "Load-In Checklist", "--title", "Load-In & Strike Checklist", "--body-file", bodyPath, "--show", showPath,
	}})
	require.Equal(t, 0, edit.ExitCode, "note edit failed: exit=%d stderr=%s", edit.ExitCode, edit.Stderr)
	afterEdit := registry.Execute(command.Request{Root: root, Args: []string{"note", "show", "Load-In & Strike Checklist", "--show", showPath}})
	require.Equal(t, 0, afterEdit.ExitCode, "note show (after edit) failed: exit=%d stderr=%s", afterEdit.ExitCode, afterEdit.Stderr)
	var afterEditView noteTestView
	err = json.Unmarshal(afterEdit.Stdout, &afterEditView)
	require.NoError(t, err, "unmarshal note show (after edit) output: %v", err)
	require.Equal(t, string(bodyBytes), afterEditView.Body, "expected persisted body to equal the file's bytes verbatim:\nwant %q\ngot  %q", bodyBytes, afterEditView.Body)
	require.NotEqual(t, created.UpdatedAt, afterEditView.UpdatedAt, "expected updated_at to refresh after an edit")

	// A title-only edit does not clobber the existing body.
	titleOnly := registry.Execute(command.Request{Root: root, Args: []string{
		"note", "edit", "Load-In & Strike Checklist", "--title", "Strike Checklist", "--show", showPath,
	}})
	require.Equal(t, 0, titleOnly.ExitCode, "note edit (title only) failed: exit=%d stderr=%s", titleOnly.ExitCode, titleOnly.Stderr)
	afterTitleOnly := registry.Execute(command.Request{Root: root, Args: []string{"note", "show", "Strike Checklist", "--show", showPath}})
	var afterTitleOnlyView noteTestView
	err = json.Unmarshal(afterTitleOnly.Stdout, &afterTitleOnlyView)
	require.NoError(t, err, "unmarshal note show (after title-only edit) output: %v", err)
	require.Equal(t, string(bodyBytes), afterTitleOnlyView.Body, "expected a title-only edit to leave body unchanged")

	// "note edit" with neither --title nor --body-file exits 2.
	editNeither := registry.Execute(command.Request{Root: root, Args: []string{"note", "edit", "Strike Checklist", "--show", showPath}})
	require.True(t, editNeither.ExitCode == 2 && strings.Contains(string(editNeither.Stderr), "GOLC_NOTE_USAGE"), "expected GOLC_NOTE_USAGE when neither --title nor --body-file is supplied, got exit=%d stderr=%s", editNeither.ExitCode, editNeither.Stderr)

	// "note edit" against an unknown note title exits 1 with
	// GOLC_NOTE_NOT_FOUND.
	editMissing := registry.Execute(command.Request{Root: root, Args: []string{"note", "edit", "Missing", "--title", "Whatever", "--show", showPath}})
	require.True(t, editMissing.ExitCode == 1 && strings.Contains(string(editMissing.Stderr), "GOLC_NOTE_NOT_FOUND"), "expected GOLC_NOTE_NOT_FOUND editing an unknown note, got exit=%d stderr=%s", editMissing.ExitCode, editMissing.Stderr)

	// An oversized body file is rejected with GOLC_NOTE_BODY_TOO_LARGE.
	oversizedPath := filepath.Join(root, "oversized.html")
	err = os.WriteFile(oversizedPath, make([]byte, (1<<20)+1), 0o644)
	require.NoError(t, err, "write oversized body fixture: %v", err)
	oversized := registry.Execute(command.Request{Root: root, Args: []string{
		"note", "edit", "Strike Checklist", "--body-file", oversizedPath, "--show", showPath,
	}})
	require.True(t, oversized.ExitCode == 1 && strings.Contains(string(oversized.Stderr), "GOLC_NOTE_BODY_TOO_LARGE"), "expected GOLC_NOTE_BODY_TOO_LARGE for an oversized body file, got exit=%d stderr=%s", oversized.ExitCode, oversized.Stderr)

	// "note delete" exits 0 and "note list" afterwards returns an empty
	// array.
	deleteResult := registry.Execute(command.Request{Root: root, Args: []string{"note", "delete", "Strike Checklist", "--show", showPath}})
	require.Equal(t, 0, deleteResult.ExitCode, "note delete failed: exit=%d stderr=%s", deleteResult.ExitCode, deleteResult.Stderr)
	afterDelete := registry.Execute(command.Request{Root: root, Args: []string{"note", "list", "--show", showPath}})
	require.Equal(t, 0, afterDelete.ExitCode, "note list (after delete) failed: exit=%d stderr=%s", afterDelete.ExitCode, afterDelete.Stderr)
	require.Equal(t, "[]", strings.TrimSpace(string(afterDelete.Stdout)), "expected an empty JSON array after delete, got: %s", afterDelete.Stdout)
}

func TestNoteRoutesUsageErrors(t *testing.T) {
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err, "NewDefaultCommandRegistry: %v", err)
	root := t.TempDir()
	showPath := "show.golc"

	// A malformed invocation (missing --show) exits 2.
	missingShow := registry.Execute(command.Request{Root: root, Args: []string{"note", "create", "Checklist"}})
	require.Equal(t, 2, missingShow.ExitCode, "expected ExitCode 2 for a missing --show, got %d stderr=%s", missingShow.ExitCode, missingShow.Stderr)

	// A malformed invocation (unknown flag) exits 2.
	unknownFlag := registry.Execute(command.Request{Root: root, Args: []string{"note", "create", "Checklist", "--bogus", "value", "--show", showPath}})
	require.Equal(t, 2, unknownFlag.ExitCode, "expected ExitCode 2 for an unknown flag, got %d stderr=%s", unknownFlag.ExitCode, unknownFlag.Stderr)
}

// TestNoteCommandBinary proves "note list" on a brand-new show prints
// exactly "[]" via the in-process registry (mirrors
// TestScriptCommandBinary's own acceptance criterion for notes).
func TestNoteCommandBinary(t *testing.T) {
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err, "NewDefaultCommandRegistry: %v", err)
	root := t.TempDir()
	result := registry.Execute(command.Request{Root: root, Args: []string{"note", "list", "--show", "fresh.golc"}})
	require.Equal(t, 0, result.ExitCode, "note list on a fresh show failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	require.Equal(t, "[]", strings.TrimSpace(string(result.Stdout)), "expected note list on a fresh show to print exactly [], got: %s", result.Stdout)
}
