// notes_test.go pins the Note domain contract before internal/show/notes.go's
// behavior is trusted: NewNote mints a stable, unique UUIDv7 identity and
// stamps CreatedAt/UpdatedAt; ValidateNote and ValidateNoteUniqueTitles
// reject every declared invalid shape; and a State carrying notes
// round-trips through the existing Save/Load path with Body preserved
// byte-for-byte, and a pre-existing show with no "notes" key at all decodes
// cleanly. This file is package show_test (external), exercising only
// exported package show API.
package show_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/show"
)

func TestNewNote(t *testing.T) {
	n, err := show.NewNote("Load-In Checklist")
	require.NoError(t, err, "NewNote")
	var zero [16]byte
	require.NotEqual(t, zero, n.ID, "expected NewNote to mint a non-nil UUIDv7 ID")
	require.Equal(t, "Load-In Checklist", n.Title)
	require.Empty(t, n.Body, "expected an empty Body")
	require.NotEmpty(t, n.CreatedAt, "expected CreatedAt to be stamped")
	require.Equal(t, n.CreatedAt, n.UpdatedAt, "expected CreatedAt and UpdatedAt to match at creation")
}

func TestNewNoteMintsDistinctIDs(t *testing.T) {
	first, err := show.NewNote("Load-In Checklist")
	require.NoError(t, err, "NewNote (first)")
	second, err := show.NewNote("Load-In Checklist")
	require.NoError(t, err, "NewNote (second)")
	require.NotEqual(t, first.ID, second.ID, "expected two calls to NewNote to mint distinct IDs")
}

func TestNewNoteRejectsEmptyTitle(t *testing.T) {
	_, err := show.NewNote("")
	require.ErrorContains(t, err, "GOLC_NOTE_TITLE_EMPTY", "expected error for an empty title")
	_, err = show.NewNote("   ")
	require.ErrorContains(t, err, "GOLC_NOTE_TITLE_EMPTY", "expected error for a whitespace-only title")
}

func TestValidateNote(t *testing.T) {
	valid, err := show.NewNote("Load-In Checklist")
	require.NoError(t, err, "NewNote")
	require.NoError(t, show.ValidateNote(valid), "expected a NewNote-constructed note to validate cleanly")

	emptyTitle := valid
	emptyTitle.Title = ""
	err = show.ValidateNote(emptyTitle)
	require.ErrorContains(t, err, "GOLC_NOTE_TITLE_EMPTY", "expected error for an empty title")

	whitespaceTitle := valid
	whitespaceTitle.Title = "   "
	err = show.ValidateNote(whitespaceTitle)
	require.ErrorContains(t, err, "GOLC_NOTE_TITLE_EMPTY", "expected error for a whitespace-only title")
}

func TestValidateNoteUniqueTitles(t *testing.T) {
	require.NoError(t, show.ValidateNoteUniqueTitles(nil), "expected an empty slice to be accepted")

	a, err := show.NewNote("Load-In Checklist")
	require.NoError(t, err, "NewNote")
	b, err := show.NewNote("Load-In Checklist")
	require.NoError(t, err, "NewNote")
	err = show.ValidateNoteUniqueTitles([]show.Note{a, b})
	require.ErrorContains(t, err, "GOLC_NOTE_TITLE_DUPLICATE", "expected error for two same-titled notes")
}

// TestShowStateNoteValidation proves notes.go's wiring into show.validate():
// two same-titled notes fail Save, and a single note round-trips through the
// existing Save/Load path unchanged, including its Body bytes verbatim (no
// normalization/reformatting at save time).
func TestShowStateNoteValidation(t *testing.T) {
	root := t.TempDir()

	a, err := show.NewNote("Load-In Checklist")
	require.NoError(t, err, "NewNote")
	b, err := show.NewNote("Load-In Checklist")
	require.NoError(t, err, "NewNote")
	dupState := show.State{Notes: []show.Note{a, b}}
	err = show.Save(root, "dup-notes.golc", dupState)
	require.ErrorContains(t, err, "GOLC_SHOW_STATE_INVALID", "expected error for duplicate note titles")

	body := "<p>Deliberately <strong>preserved</strong> markup.</p>"
	single, err := show.NewNote("Load-In Checklist")
	require.NoError(t, err, "NewNote")
	single.Body = body
	validState := show.State{Notes: []show.Note{single}}
	require.NoError(t, show.Save(root, "single-note.golc", validState), "expected a valid single note to save")

	loaded, err := show.Load(root, "single-note.golc")
	require.NoError(t, err, "Load")
	require.Len(t, loaded.Notes, 1, "expected exactly one note to round-trip")
	require.Equal(t, single.ID, loaded.Notes[0].ID, "note identity did not round-trip: %+v", loaded.Notes[0])
	require.Equal(t, single.Title, loaded.Notes[0].Title, "note identity did not round-trip: %+v", loaded.Notes[0])
	require.Equal(t, body, loaded.Notes[0].Body, "expected Body to round-trip byte-for-byte")
}

// TestShowStateNoNotesKeyDecodesCleanly proves a show saved before the
// Notes field existed (no "notes" key in its persisted blob at all) still
// decodes cleanly into a nil/empty Notes slice -- the same backward-
// compatibility guarantee Scripts established.
func TestShowStateNoNotesKeyDecodesCleanly(t *testing.T) {
	root := t.TempDir()

	state := show.State{}
	require.NoError(t, show.Save(root, "no-notes.golc", state), "expected an empty state with no notes to save")

	loaded, err := show.Load(root, "no-notes.golc")
	require.NoError(t, err, "Load")
	require.Empty(t, loaded.Notes, "expected an empty Notes slice")
}
