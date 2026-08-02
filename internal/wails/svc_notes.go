// svc_notes.go fills NotesService, the Wails binding for NotesWorkspace.tsx:
// a user creates, inspects, renames/edits, and deletes free-form rich-text
// notes through the exact already-implemented, already-tested "note"* CLI
// routes (internal/command/note.go) via command.NewDefaultCommandRegistry
// -- exactly the ScriptService pattern (svc_script.go) this file mirrors --
// so there is only one note mutation implementation in this codebase, never
// a second one duplicated for the GUI. Unlike ScriptService, NotesService
// carries no streaming/events machinery: notes have no run/debug lifecycle
// to broadcast.
//
// SaveNote cannot pass a multi-line HTML body as an argv value safely, and
// "note edit" takes --body-file: SaveNote writes the body to a temp file
// (os.CreateTemp inside os.TempDir(), removed via defer) and passes that
// path to "note edit --body-file", guarded by the same maxNoteBodyBytes
// (1 MiB) bound internal/command/note.go declares, checked before any
// write.
//
// ListNotes/GetNote decode "note list"/"note show"'s Stdout JSON with
// internal/strictjson.DecodeStrict into unexported wire types that mirror
// internal/command/note.go's own noteListEntryView/noteView shapes
// field-for-field/tag-for-tag, then project into this package's own
// camelCase NoteSummaryView/NoteDetailView shape for the frontend.
package wails

import (
	"fmt"
	"os"

	"github.com/lnorton89/golc/internal/command"
	"github.com/lnorton89/golc/internal/strictjson"
)

// maxNoteBodyBytes bounds SaveNote's input -- mirrors
// internal/command/note.go's identical constant (unexported to that
// package, so this file keeps its own copy); a body larger than this is
// rejected with GOLC_NOTE_BODY_TOO_LARGE before any temp file is ever
// written.
const maxNoteBodyBytes = 1 << 20 // 1 MiB

// NotesService is bound to the frontend via cmd/golc-desktop/main.go's
// options.App{Bind: [...]}. root/showPath are the exact ShowState location
// every method acts against (mirrors ScriptService/ShowService's own
// fields).
type NotesService struct {
	pipeName string
	root     string
	showPath string
}

// NewNotesService constructs a NotesService targeting pipeName (reserved,
// unused by this ShowState-only CRUD -- mirrors ScriptService/ShowService's
// own unused pipeName field) and the ShowState at showPath, resolved
// against root.
func NewNotesService(pipeName, root, showPath string) *NotesService {
	return &NotesService{pipeName: pipeName, root: root, showPath: showPath}
}

// execute builds the default command registry and runs args against it,
// converting the internal/command.Result shape into this package's own
// Result shape (mirrors svc_script.go/svc_show.go's identical helper).
func (s *NotesService) execute(args ...string) Result {
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		return Result{ExitCode: 2, Stderr: fmt.Sprintf("GOLC_WAILS_REGISTRY_BUILD_FAILED: %v", err)}
	}
	result := registry.Execute(command.Request{Root: s.root, Args: args})
	return Result{ExitCode: result.ExitCode, Stdout: string(result.Stdout), Stderr: string(result.Stderr)}
}

// noteListEntryWire mirrors internal/command/note.go's noteListEntryView
// JSON shape exactly ("note list"'s per-note payload -- no Body).
type noteListEntryWire struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

// noteWire mirrors internal/command/note.go's noteView JSON shape exactly
// (the full per-note payload "note create"/"note show"/"note edit" write
// to Stdout, including Body).
type noteWire struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

// NoteSummaryView is the library-row projection: one note's identity,
// title, and last-updated time (Body omitted -- mirrors "note list"'s own
// cheap-listing rationale).
type NoteSummaryView struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

// NoteDetailView is GetNote's return shape: NoteSummaryView's fields plus
// Body (the note's full HTML) and CreatedAt.
type NoteDetailView struct {
	NoteSummaryView
	Body      string `json:"body"`
	CreatedAt string `json:"createdAt,omitempty"`
}

// ListNotes returns a []NoteSummaryView decoded from "note list"'s JSON, or
// an error when the show cannot be read (a non-zero exit from the CLI
// route, or a malformed/unparseable Stdout payload). A show with no notes
// returns an explicit empty (non-nil) slice -- there is no frontend error
// boundary, so a null slice reaching NotesWorkspace.tsx would crash the
// whole React tree.
func (s *NotesService) ListNotes() ([]NoteSummaryView, error) {
	result := s.execute("note", "list", "--show", s.showPath)
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("%s", result.Stderr)
	}

	var wire []noteListEntryWire
	if err := strictjson.DecodeStrict([]byte(result.Stdout), &wire); err != nil {
		return nil, fmt.Errorf("GOLC_WAILS_NOTE_LIST_DECODE_FAILED: %v", err)
	}

	views := make([]NoteSummaryView, 0, len(wire))
	for _, w := range wire {
		views = append(views, NoteSummaryView{ID: w.ID, Title: w.Title, UpdatedAt: w.UpdatedAt})
	}
	return views, nil
}

// GetNote returns a NoteDetailView (including Body) decoded from "note
// show <name>"'s JSON. An unknown note title surfaces the route's own
// GOLC_NOTE_NOT_FOUND diagnostic as the returned error.
func (s *NotesService) GetNote(name string) (NoteDetailView, error) {
	result := s.execute("note", "show", name, "--show", s.showPath)
	if result.ExitCode != 0 {
		return NoteDetailView{}, fmt.Errorf("%s", result.Stderr)
	}

	var wire noteWire
	if err := strictjson.DecodeStrict([]byte(result.Stdout), &wire); err != nil {
		return NoteDetailView{}, fmt.Errorf("GOLC_WAILS_NOTE_SHOW_DECODE_FAILED: %v", err)
	}

	return NoteDetailView{
		NoteSummaryView: NoteSummaryView{ID: wire.ID, Title: wire.Title, UpdatedAt: wire.UpdatedAt},
		Body:            wire.Body,
		CreatedAt:       wire.CreatedAt,
	}, nil
}

// CreateNote creates a new named, empty note via "note create <name>
// --show <path>": Result{ExitCode:0} on success, Result{ExitCode:1,
// Stderr: "...GOLC_NOTE_TITLE_DUPLICATE..."} when the title is already
// taken.
func (s *NotesService) CreateNote(name string) Result {
	return s.execute("note", "create", name, "--show", s.showPath)
}

// SaveNote persists title and body as the named note's new Title/Body (a
// full save, in one call) via "note edit <name> --title <title>
// --body-file <path> --show <path>" -- see the package doc comment for the
// temp-file argv-safety rationale. A body exceeding maxNoteBodyBytes is
// rejected with GOLC_NOTE_BODY_TOO_LARGE before any temp file is written.
func (s *NotesService) SaveNote(name, title, body string) Result {
	if len(body) > maxNoteBodyBytes {
		return Result{ExitCode: 1, Stderr: fmt.Sprintf(
			"GOLC_NOTE_BODY_TOO_LARGE: body is %d bytes, exceeding the %d byte maximum", len(body), maxNoteBodyBytes)}
	}

	tempFile, err := os.CreateTemp("", "golc-note-body-*.html")
	if err != nil {
		return Result{ExitCode: 1, Stderr: fmt.Sprintf("GOLC_WAILS_NOTE_TEMP_FILE_FAILED: %v", err)}
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)

	if _, writeErr := tempFile.WriteString(body); writeErr != nil {
		_ = tempFile.Close()
		return Result{ExitCode: 1, Stderr: fmt.Sprintf("GOLC_WAILS_NOTE_TEMP_FILE_FAILED: %v", writeErr)}
	}
	if closeErr := tempFile.Close(); closeErr != nil {
		return Result{ExitCode: 1, Stderr: fmt.Sprintf("GOLC_WAILS_NOTE_TEMP_FILE_FAILED: %v", closeErr)}
	}

	return s.execute("note", "edit", name, "--title", title, "--body-file", tempPath, "--show", s.showPath)
}

// DeleteNote removes the named note via "note delete <name> --show
// <path>"; a subsequent ListNotes omits it. An unknown note title surfaces
// the route's own GOLC_NOTE_NOT_FOUND diagnostic.
func (s *NotesService) DeleteNote(name string) Result {
	return s.execute("note", "delete", name, "--show", s.showPath)
}
