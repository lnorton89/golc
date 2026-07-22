// history.go implements the session-only whole-session linear undo/redo
// history (CONTEXT PROG-07, D-12/D-13/D-14): a single global stack of
// EditOp entries covering record/update/rename/reorder/duplicate/delete
// across every programming/scene object type, walked backward and forward
// in insertion order -- one global stack, never per-object-type stacks
// (D-12). History is a plain in-memory structure: it is never a
// show.State field and never touches show.Save (D-14 -- undo history
// resets on process close/reopen; it is not part of the durable show
// document). Undo/redo behavior does not depend on whether the edited
// object is currently part of the active scene (D-13): History is
// object-type-agnostic and never inspects scene activation.
package programming

import (
	"fmt"

	"github.com/google/uuid"
)

// EditKind names the kind of mutation an EditOp records (PROG-07): the six
// CRUD verbs that make up the programming object-library surface.
type EditKind string

// The six v1 edit kinds every EditOp carries.
const (
	EditRecord    EditKind = "record"
	EditUpdate    EditKind = "update"
	EditRename    EditKind = "rename"
	EditReorder   EditKind = "reorder"
	EditDuplicate EditKind = "duplicate"
	EditDelete    EditKind = "delete"
)

// EditOp is one recorded edit: Kind names the mutation, ObjectType/ObjectID
// identify the affected programming object, and Before/After hold whole
// snapshots of the object's state immediately before/after the edit --
// enough for Undo/Redo to restore either side without any partial
// application (all-or-nothing replay, never a half-applied undo). Before/
// After are typed "any" because a single global stack (D-12) holds
// snapshots of every object type (Theme, Preset, Chase, MotionPreset,
// Scene, ...) in one ordered sequence, not per-type stacks that would each
// need their own concrete element type.
type EditOp struct {
	Kind       EditKind
	ObjectType string
	ObjectID   uuid.UUID
	Before     any
	After      any
}

// History is a single whole-session linear undo/redo stack (D-12): one
// global ops slice plus an integer cursor marking how many of those ops
// are currently "applied" -- never per-object-type stacks or maps. The
// zero value is not ready for use; construct with NewHistory. History
// carries no field that is ever reachable from show.State or written by
// show.Save (D-14): it is a plain in-memory structure, reset whenever the
// process restarts.
type History struct {
	ops    []EditOp
	cursor int
}

// NewHistory returns an empty History ready for use.
func NewHistory() *History {
	return &History{}
}

// Record appends op to the history and truncates any existing redo tail
// (PROG-07: a new edit recorded after an Undo discards the redone-away
// branch -- standard linear-history semantics). Record never inspects
// op.ObjectType or op.ObjectID against any notion of "currently active
// scene" (D-13): every edit is recorded identically regardless of what it
// touches.
func (h *History) Record(op EditOp) {
	h.ops = append(h.ops[:h.cursor], op)
	h.cursor = len(h.ops)
}

// CanUndo reports whether Undo has an op to return.
func (h *History) CanUndo() bool {
	return h.cursor > 0
}

// CanRedo reports whether Redo has an op to return.
func (h *History) CanRedo() bool {
	return h.cursor < len(h.ops)
}

// Undo returns the most recently applied EditOp and moves the cursor back
// one step, so the caller can replay op.Before through the same mutate
// pipeline any other edit uses (D-13: undo is just another edit, never
// special-cased for a live-active target). Calling Undo with nothing to
// undo returns GOLC_HISTORY_NOTHING_TO_UNDO rather than panicking or
// silently no-op'ing.
func (h *History) Undo() (EditOp, error) {
	if !h.CanUndo() {
		return EditOp{}, fmt.Errorf("GOLC_HISTORY_NOTHING_TO_UNDO: no recorded edit to undo")
	}
	h.cursor--
	return h.ops[h.cursor], nil
}

// Redo returns the next undone EditOp and moves the cursor forward one
// step, so the caller can replay op.After through the same mutate pipeline
// any other edit uses. Calling Redo with no redo tail available returns
// GOLC_HISTORY_NOTHING_TO_REDO rather than panicking or silently
// no-op'ing.
func (h *History) Redo() (EditOp, error) {
	if !h.CanRedo() {
		return EditOp{}, fmt.Errorf("GOLC_HISTORY_NOTHING_TO_REDO: no undone edit to redo")
	}
	op := h.ops[h.cursor]
	h.cursor++
	return op, nil
}
