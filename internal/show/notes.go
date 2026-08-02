// notes.go declares the Note domain model: a single free-form rich-text
// note saved inside show.State as another entity in the single revisioned
// document, exactly like Script (scripts.go) -- it inherits autosave,
// recovery, migration, and export for free. Note copies scripts.go's
// identity/construction/unique-name shape: identity is a durable UUIDv7
// minted once at creation, never re-minted. Body is stored verbatim as the
// WYSIWYG editor's own serialized HTML -- no sanitization or normalization
// at save time, mirroring Script's own verbatim-Source-storage discipline.
package show

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Note is a single free-form rich-text note saved inside show.State.
// Identity is a durable UUIDv7 minted once at creation -- never derived
// from Title, and never re-minted by a later rename or body edit. Body is
// the note's WYSIWYG editor content, stored verbatim (byte-for-byte) as
// HTML, with no normalization or reformatting applied at save time.
// CreatedAt is stamped once by NewNote and never changes again; UpdatedAt
// refreshes on every successful title or body change.
type Note struct {
	ID        uuid.UUID `json:"id"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	CreatedAt string    `json:"created_at,omitempty"`
	UpdatedAt string    `json:"updated_at,omitempty"`
}

// NewNote mints a fresh UUIDv7-identified Note with an empty Body and
// CreatedAt/UpdatedAt both stamped to the current time in RFC3339Nano
// (nanosecond precision, so two notes created within the same second still
// carry distinguishable timestamps). IDs are minted only at creation time
// -- never derived from Title, and never re-minted by a later rename or
// body edit.
func NewNote(title string) (Note, error) {
	if strings.TrimSpace(title) == "" {
		return Note{}, errors.New("GOLC_NOTE_TITLE_EMPTY: note title must not be empty")
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Note{}, fmt.Errorf("GOLC_NOTE_ID_MINT_FAILED: %v", err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	return Note{
		ID:        id,
		Title:     title,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// ValidateNote re-checks the one invariant a hand-edited or otherwise
// untrusted Note must satisfy before it is trusted: Title is non-empty
// (GOLC_NOTE_TITLE_EMPTY).
func ValidateNote(n Note) error {
	if strings.TrimSpace(n.Title) == "" {
		return fmt.Errorf("GOLC_NOTE_TITLE_EMPTY: note %s declares an empty title", n.ID)
	}
	return nil
}

// ValidateNoteUniqueTitles rejects any two notes in notes sharing the same
// Title: a duplicate title is always rejected before any save commits,
// never silently permitted.
func ValidateNoteUniqueTitles(notes []Note) error {
	seen := make(map[string]bool, len(notes))
	for _, n := range notes {
		if seen[n.Title] {
			return fmt.Errorf("GOLC_NOTE_TITLE_DUPLICATE: a note titled %q already exists", n.Title)
		}
		seen[n.Title] = true
	}
	return nil
}
