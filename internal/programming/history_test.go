// history_test.go proves PROG-07's session-only whole-session linear
// undo/redo history contract (03-05-PLAN.md Task 1, CONTEXT D-12/D-13/
// D-14): Record pushes an EditOp and truncates any redo tail; Undo/Redo
// round-trip to the identical recorded op; the empty-history/no-redo-tail
// boundaries return GOLC_HISTORY_NOTHING_TO_UNDO/GOLC_HISTORY_NOTHING_TO_
// REDO rather than crashing; and a single ordered stack walks mixed
// object-type edits in exact insertion order -- never per-object-type
// stacks.
package programming_test

import (
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/programming"
)

func TestHistoryRecordUndoRedoRoundTrip(t *testing.T) {
	h := programming.NewHistory()
	id := uuid.Must(uuid.NewV7())
	op := programming.EditOp{
		Kind:       programming.EditRename,
		ObjectType: "theme",
		ObjectID:   id,
		Before:     "Sunset",
		After:      "Ocean",
	}
	h.Record(op)

	if !h.CanUndo() {
		t.Fatalf("expected CanUndo() to be true immediately after Record")
	}
	if h.CanRedo() {
		t.Fatalf("expected CanRedo() to be false immediately after Record (nothing undone yet)")
	}

	undone, err := h.Undo()
	if err != nil {
		t.Fatalf("Undo: %v", err)
	}
	if undone.ObjectID != id || undone.Before != "Sunset" || undone.After != "Ocean" {
		t.Fatalf("expected the undone op to carry the exact recorded Before/After, got %+v", undone)
	}
	if !h.CanRedo() {
		t.Fatalf("expected CanRedo() to be true after Undo")
	}
	if h.CanUndo() {
		t.Fatalf("expected CanUndo() to be false after undoing the only recorded op")
	}

	redone, err := h.Redo()
	if err != nil {
		t.Fatalf("Redo: %v", err)
	}
	if redone != undone {
		t.Fatalf("expected Undo then Redo to return to the identical op, undo=%+v redo=%+v", undone, redone)
	}
	if !h.CanUndo() || h.CanRedo() {
		t.Fatalf("expected History to be back at the fully-applied position after Redo, canUndo=%t canRedo=%t", h.CanUndo(), h.CanRedo())
	}
}

func TestHistoryUndoEmptyBoundaryNoCrash(t *testing.T) {
	h := programming.NewHistory()
	_, err := h.Undo()
	if err == nil || !strings.Contains(err.Error(), "GOLC_HISTORY_NOTHING_TO_UNDO") {
		t.Fatalf("expected GOLC_HISTORY_NOTHING_TO_UNDO on an empty history, got %v", err)
	}
}

func TestHistoryRedoNoTailBoundaryNoCrash(t *testing.T) {
	h := programming.NewHistory()
	if _, err := h.Redo(); err == nil || !strings.Contains(err.Error(), "GOLC_HISTORY_NOTHING_TO_REDO") {
		t.Fatalf("expected GOLC_HISTORY_NOTHING_TO_REDO on an empty history, got %v", err)
	}

	h.Record(programming.EditOp{Kind: programming.EditRecord, ObjectType: "theme", ObjectID: uuid.Must(uuid.NewV7())})
	if _, err := h.Redo(); err == nil || !strings.Contains(err.Error(), "GOLC_HISTORY_NOTHING_TO_REDO") {
		t.Fatalf("expected GOLC_HISTORY_NOTHING_TO_REDO immediately after Record (nothing undone yet), got %v", err)
	}
}

func TestHistoryRecordTruncatesRedoTail(t *testing.T) {
	h := programming.NewHistory()
	first := programming.EditOp{Kind: programming.EditRename, ObjectType: "theme", ObjectID: uuid.Must(uuid.NewV7()), After: "first"}
	second := programming.EditOp{Kind: programming.EditRename, ObjectType: "theme", ObjectID: uuid.Must(uuid.NewV7()), After: "second"}
	h.Record(first)
	h.Record(second)

	if _, err := h.Undo(); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	if !h.CanRedo() {
		t.Fatalf("expected a redo tail (second) to exist after one Undo")
	}

	// Recording a new edit after an Undo must discard the redone-away
	// branch ("second") -- standard linear-history semantics (PROG-07),
	// never a per-object-type stack that would let "second" survive
	// alongside the new edit.
	third := programming.EditOp{Kind: programming.EditRename, ObjectType: "theme", ObjectID: uuid.Must(uuid.NewV7()), After: "third"}
	h.Record(third)

	if h.CanRedo() {
		t.Fatalf("expected Record to discard the redo tail, but CanRedo() is still true")
	}
	undone, err := h.Undo()
	if err != nil {
		t.Fatalf("Undo after Record: %v", err)
	}
	if undone.After != "third" {
		t.Fatalf("expected the most recently recorded op (third) to be undone, got %+v", undone)
	}
	redone, err := h.Redo()
	if err != nil {
		t.Fatalf("Redo: %v", err)
	}
	if redone.After != "third" {
		t.Fatalf("expected Redo to re-apply third (never the truncated second), got %+v", redone)
	}
	if _, err := h.Redo(); err == nil || !strings.Contains(err.Error(), "GOLC_HISTORY_NOTHING_TO_REDO") {
		t.Fatalf("expected the truncated 'second' op to never resurface via Redo, got %v", err)
	}
}

func TestHistoryMixedObjectTypeSingleGlobalStack(t *testing.T) {
	h := programming.NewHistory()
	themeOp := programming.EditOp{Kind: programming.EditUpdate, ObjectType: "theme", ObjectID: uuid.Must(uuid.NewV7()), After: "theme-edit"}
	chaseOp := programming.EditOp{Kind: programming.EditUpdate, ObjectType: "chase", ObjectID: uuid.Must(uuid.NewV7()), After: "chase-edit"}
	sceneOp := programming.EditOp{Kind: programming.EditUpdate, ObjectType: "scene", ObjectID: uuid.Must(uuid.NewV7()), After: "scene-edit"}

	// Interleave record order across three distinct object types -- a
	// single ordered walk backward must visit scene, then chase, then
	// theme (exact reverse insertion order). A per-object-type stack
	// implementation would instead let an earlier theme edit be undone
	// independently of the later chase/scene edits; this test fails that
	// implementation.
	h.Record(themeOp)
	h.Record(chaseOp)
	h.Record(sceneOp)

	first, err := h.Undo()
	if err != nil || first.ObjectType != "scene" {
		t.Fatalf("expected the first undo to reverse the scene edit, got %+v err=%v", first, err)
	}
	second, err := h.Undo()
	if err != nil || second.ObjectType != "chase" {
		t.Fatalf("expected the second undo to reverse the chase edit, got %+v err=%v", second, err)
	}
	third, err := h.Undo()
	if err != nil || third.ObjectType != "theme" {
		t.Fatalf("expected the third undo to reverse the theme edit, got %+v err=%v", third, err)
	}
	if _, err := h.Undo(); err == nil || !strings.Contains(err.Error(), "GOLC_HISTORY_NOTHING_TO_UNDO") {
		t.Fatalf("expected the stack to be exhausted after undoing all three mixed-type edits, got %v", err)
	}

	// Redo walks forward in the same single order: theme, then chase, then
	// scene.
	redoneTheme, err := h.Redo()
	if err != nil || redoneTheme.ObjectType != "theme" {
		t.Fatalf("expected the first redo to reapply the theme edit, got %+v err=%v", redoneTheme, err)
	}
	redoneChase, err := h.Redo()
	if err != nil || redoneChase.ObjectType != "chase" {
		t.Fatalf("expected the second redo to reapply the chase edit, got %+v err=%v", redoneChase, err)
	}
	redoneScene, err := h.Redo()
	if err != nil || redoneScene.ObjectType != "scene" {
		t.Fatalf("expected the third redo to reapply the scene edit, got %+v err=%v", redoneScene, err)
	}
}
