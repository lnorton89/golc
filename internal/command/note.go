// note.go is the note command file: it owns the "note" routing scope and
// self-registers "note create"/"note list"/"note show"/"note edit"/
// "note delete" -- a show author creates, inspects, renames/edits, and
// deletes free-form rich-text notes that live inside show.State
// (internal/show/notes.go). Handlers follow script.go's parse-args-then-
// Load-mutate-Save-Stdout shape; every route writes deterministic JSON to
// Stdout (never plain text). This file duplicates (rather than shares)
// script.go's own parse-args/flag helpers, matching this codebase's
// per-file-owns-its-parsing convention (scene.go and script.go each keep
// their own copies rather than a shared generic parser).
package command

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/lnorton89/golc/internal/show"
	"github.com/lnorton89/golc/internal/strictjson"
)

// maxNoteBodyBytes bounds "note edit --body-file"'s input (DoS
// mitigation, mirrors script.go's maxScriptSourceBytes): a body file
// larger than this is rejected with GOLC_NOTE_BODY_TOO_LARGE before it can
// ever enter the show blob.
const maxNoteBodyBytes = 1 << 20 // 1 MiB

var _ = MustDeclareScope(ScopeRegistration{
	Scope:   "note",
	Summary: "Free-form rich-text notes saved inside a show, edited through a WYSIWYG editor and stored as HTML.",
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "note create",
	Summary: "Create a named, empty note against a ShowState document: note create <name> --show <path>.",
	Handler: runNoteCreate,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "note list",
	Summary: "List every note's id, title, and last-updated time (source omitted): note list --show <path>.",
	Handler: runNoteList,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "note show",
	Summary: "Show one note in full, including its body: note show <name> --show <path>.",
	Handler: runNoteShow,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route: "note edit",
	Summary: "Rename a note and/or replace its body with a file's bytes verbatim (max 1MiB): " +
		"note edit <name> [--title <name>] [--body-file <path>] --show <path>.",
	Handler: runNoteEdit,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "note delete",
	Summary: "Delete a named note: note delete <name> --show <path>.",
	Handler: runNoteDelete,
})

// noteView is "note create"/"note show"/"note edit"'s full per-note JSON
// shape, including Body.
type noteView struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

// noteListEntryView is "note list"'s per-note JSON shape: identity,
// title, and last-updated time. It deliberately omits Body so listing a
// show with many large notes stays cheap.
type noteListEntryView struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

// toNoteView projects a show.Note into its full rendered view shape.
func toNoteView(n show.Note) noteView {
	return noteView{
		ID:        n.ID.String(),
		Title:     n.Title,
		Body:      n.Body,
		CreatedAt: n.CreatedAt,
		UpdatedAt: n.UpdatedAt,
	}
}

// toNoteListEntryView projects a show.Note into "note list"'s compact
// per-entry view shape.
func toNoteListEntryView(n show.Note) noteListEntryView {
	return noteListEntryView{
		ID:        n.ID.String(),
		Title:     n.Title,
		UpdatedAt: n.UpdatedAt,
	}
}

// encodeNoteResult canonically encodes view as this file's uniform
// success-output shape -- every "note *" route writes deterministic JSON
// to Stdout, never plain text.
func encodeNoteResult(view any) Result {
	payload, err := strictjson.CanonicalEncode(view)
	if err != nil {
		return Result{ExitCode: 1, Stderr: fmt.Appendf(nil, "GOLC_NOTE_ENCODE_FAILED: %v\n", err)}
	}
	return Result{Stdout: payload}
}

// noteByTitle returns the note in notes whose Title matches title, plus
// its index (so the caller can splice a mutated copy back into place),
// mirroring script.go's scriptByName exactly.
func noteByTitle(notes []show.Note, title string) (show.Note, int, bool) {
	for i, n := range notes {
		if n.Title == title {
			return n, i, true
		}
	}
	return show.Note{}, -1, false
}

// parseNotePositionalArgs accepts a required positional <name> followed by
// any number of "--flag value"/"--flag=value" pairs, rejecting anything
// else (GOLC_NOTE_USAGE). It returns the full flag map so each route
// validates its own required/optional flags -- mirrors script.go's
// parseScriptPositionalArgs shape.
func parseNotePositionalArgs(usage string, args []string) (name string, flags map[string]string, err error) {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return "", nil, fmt.Errorf("GOLC_NOTE_USAGE: usage: %s", usage)
	}
	name = args[0]
	flags = map[string]string{}

	rest := args[1:]
	for i := 0; i < len(rest); {
		argument := rest[i]
		if !strings.HasPrefix(argument, "--") {
			return "", nil, fmt.Errorf("GOLC_NOTE_USAGE: unsupported argument %q; usage: %s", argument, usage)
		}
		if eq := strings.Index(argument, "="); eq >= 0 {
			flags[argument[2:eq]] = argument[eq+1:]
			i++
			continue
		}
		flagName := strings.TrimPrefix(argument, "--")
		if i+1 >= len(rest) {
			return "", nil, fmt.Errorf("GOLC_NOTE_USAGE: --%s requires a value; usage: %s", flagName, usage)
		}
		flags[flagName] = rest[i+1]
		i += 2
	}
	return name, flags, nil
}

// parseNoteFlags is parseNotePositionalArgs without a positional name --
// used only by "note list", the one note route with no target note.
func parseNoteFlags(usage string, args []string) (map[string]string, error) {
	flags := map[string]string{}
	for i := 0; i < len(args); {
		argument := args[i]
		if !strings.HasPrefix(argument, "--") {
			return nil, fmt.Errorf("GOLC_NOTE_USAGE: unsupported argument %q; usage: %s", argument, usage)
		}
		if eq := strings.Index(argument, "="); eq >= 0 {
			flags[argument[2:eq]] = argument[eq+1:]
			i++
			continue
		}
		flagName := strings.TrimPrefix(argument, "--")
		if i+1 >= len(args) {
			return nil, fmt.Errorf("GOLC_NOTE_USAGE: --%s requires a value; usage: %s", flagName, usage)
		}
		flags[flagName] = args[i+1]
		i += 2
	}
	return flags, nil
}

// rejectUnknownNoteFlags fails with GOLC_NOTE_USAGE (a malformed
// invocation, ExitCode 2) if flags carries any key outside allowed -- every
// note route has an exact known flag set, and an unrecognized flag is
// never silently ignored.
func rejectUnknownNoteFlags(usage string, flags map[string]string, allowed map[string]bool) error {
	for name := range flags {
		if !allowed[name] {
			return fmt.Errorf("GOLC_NOTE_USAGE: unsupported argument %q; usage: %s", "--"+name, usage)
		}
	}
	return nil
}

// runNoteCreate serves the self-registered "note create" route: load the
// ShowState at --show, append a new empty note (show.NewNote), and save
// atomically. A duplicate note title is rejected by show.Save's whole-
// State validation (surfaced as GOLC_NOTE_TITLE_DUPLICATE inside the
// wrapping GOLC_SHOW_STATE_INVALID diagnostic) -- never a silent
// duplicate, and the show is left unchanged.
func runNoteCreate(request Request) Result {
	usage := "note create <name> --show <path>"
	name, flags, err := parseNotePositionalArgs(usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}
	if err := rejectUnknownNoteFlags(usage, flags, map[string]bool{"show": true}); err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}
	showPath, ok := flags["show"]
	if !ok || showPath == "" {
		return Result{ExitCode: 2, Stderr: fmt.Appendf(nil, "GOLC_NOTE_USAGE: --show is required; usage: %s\n", usage)}
	}

	state, err := show.Load(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	newNote, err := show.NewNote(name)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	state.Notes = append(state.Notes, newNote)

	if err := show.Save(request.Root, showPath, state); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	return encodeNoteResult(toNoteView(newNote))
}

// runNoteList serves the self-registered "note list" route: project every
// note in the ShowState at --show into the compact list-view JSON shape
// (id, title, updated_at -- never body). A show with no notes writes an
// empty JSON array, never null.
func runNoteList(request Request) Result {
	usage := "note list --show <path>"
	flags, err := parseNoteFlags(usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}
	if err := rejectUnknownNoteFlags(usage, flags, map[string]bool{"show": true}); err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}
	showPath, ok := flags["show"]
	if !ok || showPath == "" {
		return Result{ExitCode: 2, Stderr: fmt.Appendf(nil, "GOLC_NOTE_USAGE: --show is required; usage: %s\n", usage)}
	}

	state, err := show.Load(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	views := make([]noteListEntryView, 0, len(state.Notes))
	for _, n := range state.Notes {
		views = append(views, toNoteListEntryView(n))
	}
	return encodeNoteResult(views)
}

// runNoteShow serves the self-registered "note show" route: return one
// note in full, including its body. An unknown title fails with
// GOLC_NOTE_NOT_FOUND.
func runNoteShow(request Request) Result {
	usage := "note show <name> --show <path>"
	name, flags, err := parseNotePositionalArgs(usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}
	if err := rejectUnknownNoteFlags(usage, flags, map[string]bool{"show": true}); err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}
	showPath, ok := flags["show"]
	if !ok || showPath == "" {
		return Result{ExitCode: 2, Stderr: fmt.Appendf(nil, "GOLC_NOTE_USAGE: --show is required; usage: %s\n", usage)}
	}

	state, err := show.Load(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	target, _, found := noteByTitle(state.Notes, name)
	if !found {
		return Result{ExitCode: 1, Stderr: fmt.Appendf(nil, "GOLC_NOTE_NOT_FOUND: no note titled %q exists\n", name)}
	}
	return encodeNoteResult(toNoteView(target))
}

// runNoteEdit serves the self-registered "note edit" route: apply only the
// flags the caller actually supplied on this invocation (--title and/or
// --body-file) to the named note -- a field the caller did not mention is
// carried forward unchanged, so a title-only or body-only edit never
// silently clobbers the other field. --body-file's bytes are read verbatim
// -- no normalization or reformatting -- rejecting a file larger than
// maxNoteBodyBytes (GOLC_NOTE_BODY_TOO_LARGE) before it can ever enter the
// show blob. UpdatedAt refreshes whenever either field actually changes.
// Neither flag supplied is a malformed invocation (GOLC_NOTE_USAGE). An
// unknown note title fails with GOLC_NOTE_NOT_FOUND.
func runNoteEdit(request Request) Result {
	usage := "note edit <name> [--title <name>] [--body-file <path>] --show <path>"
	name, flags, err := parseNotePositionalArgs(usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}
	if err := rejectUnknownNoteFlags(usage, flags, map[string]bool{"show": true, "title": true, "body-file": true}); err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}
	showPath, ok := flags["show"]
	if !ok || showPath == "" {
		return Result{ExitCode: 2, Stderr: fmt.Appendf(nil, "GOLC_NOTE_USAGE: --show is required; usage: %s\n", usage)}
	}
	newTitle, hasTitle := flags["title"]
	bodyFile, hasBodyFile := flags["body-file"]
	if !hasTitle && !hasBodyFile {
		return Result{ExitCode: 2, Stderr: fmt.Appendf(nil, "GOLC_NOTE_USAGE: at least one of --title/--body-file is required; usage: %s\n", usage)}
	}

	state, err := show.Load(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	target, index, found := noteByTitle(state.Notes, name)
	if !found {
		return Result{ExitCode: 1, Stderr: fmt.Appendf(nil, "GOLC_NOTE_NOT_FOUND: no note titled %q exists\n", name)}
	}

	if hasTitle {
		target.Title = newTitle
	}
	if hasBodyFile {
		resolvedBodyPath := resolveWritablePath(request.Root, bodyFile)
		info, statErr := os.Stat(resolvedBodyPath)
		if statErr != nil {
			return Result{ExitCode: 1, Stderr: fmt.Appendf(nil, "GOLC_NOTE_BODY_READ_FAILED: %v\n", statErr)}
		}
		if info.Size() > maxNoteBodyBytes {
			return Result{ExitCode: 1, Stderr: fmt.Appendf(nil,
				"GOLC_NOTE_BODY_TOO_LARGE: body file %q is %d bytes, exceeding the %d byte maximum\n", bodyFile, info.Size(), maxNoteBodyBytes)}
		}
		bodyBytes, readErr := os.ReadFile(resolvedBodyPath)
		if readErr != nil {
			return Result{ExitCode: 1, Stderr: fmt.Appendf(nil, "GOLC_NOTE_BODY_READ_FAILED: %v\n", readErr)}
		}
		target.Body = string(bodyBytes)
	}
	target.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	state.Notes[index] = target

	if err := show.Save(request.Root, showPath, state); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	return encodeNoteResult(toNoteView(target))
}

// runNoteDelete serves the self-registered "note delete" route: remove the
// named note from the ShowState at --show and save atomically. An unknown
// note title fails with GOLC_NOTE_NOT_FOUND rather than a silent no-op.
func runNoteDelete(request Request) Result {
	usage := "note delete <name> --show <path>"
	name, flags, err := parseNotePositionalArgs(usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}
	if err := rejectUnknownNoteFlags(usage, flags, map[string]bool{"show": true}); err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}
	showPath, ok := flags["show"]
	if !ok || showPath == "" {
		return Result{ExitCode: 2, Stderr: fmt.Appendf(nil, "GOLC_NOTE_USAGE: --show is required; usage: %s\n", usage)}
	}

	state, err := show.Load(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	_, index, found := noteByTitle(state.Notes, name)
	if !found {
		return Result{ExitCode: 1, Stderr: fmt.Appendf(nil, "GOLC_NOTE_NOT_FOUND: no note titled %q exists\n", name)}
	}
	state.Notes = append(state.Notes[:index], state.Notes[index+1:]...)

	if err := show.Save(request.Root, showPath, state); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	return Result{Stdout: fmt.Appendf(nil, "GOLC_NOTE_DELETED: %s\n", name)}
}
