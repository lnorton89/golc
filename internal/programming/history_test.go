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
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

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

	require.True(t, h.CanUndo(), "expected CanUndo() to be true immediately after Record")
	require.False(t, h.CanRedo(), "expected CanRedo() to be false immediately after Record (nothing undone yet)")

	undone, err := h.Undo()
	require.NoError(t, err, "Undo")
	require.Equal(t, id, undone.ObjectID)
	require.Equal(t, "Sunset", undone.Before, "expected the undone op to carry the exact recorded Before/After")
	require.Equal(t, "Ocean", undone.After, "expected the undone op to carry the exact recorded Before/After")
	require.True(t, h.CanRedo(), "expected CanRedo() to be true after Undo")
	require.False(t, h.CanUndo(), "expected CanUndo() to be false after undoing the only recorded op")

	redone, err := h.Redo()
	require.NoError(t, err, "Redo")
	require.Equal(t, undone, redone, "expected Undo then Redo to return to the identical op")
	require.True(t, h.CanUndo(), "expected History to be back at the fully-applied position after Redo")
	require.False(t, h.CanRedo(), "expected History to be back at the fully-applied position after Redo")
}

func TestHistoryUndoEmptyBoundaryNoCrash(t *testing.T) {
	h := programming.NewHistory()
	_, err := h.Undo()
	require.ErrorContains(t, err, "GOLC_HISTORY_NOTHING_TO_UNDO", "expected GOLC_HISTORY_NOTHING_TO_UNDO on an empty history")
}

func TestHistoryRedoNoTailBoundaryNoCrash(t *testing.T) {
	h := programming.NewHistory()
	_, err := h.Redo()
	require.ErrorContains(t, err, "GOLC_HISTORY_NOTHING_TO_REDO", "expected GOLC_HISTORY_NOTHING_TO_REDO on an empty history")

	h.Record(programming.EditOp{Kind: programming.EditRecord, ObjectType: "theme", ObjectID: uuid.Must(uuid.NewV7())})
	_, err = h.Redo()
	require.ErrorContains(t, err, "GOLC_HISTORY_NOTHING_TO_REDO", "expected GOLC_HISTORY_NOTHING_TO_REDO immediately after Record (nothing undone yet)")
}

func TestHistoryRecordTruncatesRedoTail(t *testing.T) {
	h := programming.NewHistory()
	first := programming.EditOp{Kind: programming.EditRename, ObjectType: "theme", ObjectID: uuid.Must(uuid.NewV7()), After: "first"}
	second := programming.EditOp{Kind: programming.EditRename, ObjectType: "theme", ObjectID: uuid.Must(uuid.NewV7()), After: "second"}
	h.Record(first)
	h.Record(second)

	_, err := h.Undo()
	require.NoError(t, err, "Undo")
	require.True(t, h.CanRedo(), "expected a redo tail (second) to exist after one Undo")

	// Recording a new edit after an Undo must discard the redone-away
	// branch ("second") -- standard linear-history semantics (PROG-07),
	// never a per-object-type stack that would let "second" survive
	// alongside the new edit.
	third := programming.EditOp{Kind: programming.EditRename, ObjectType: "theme", ObjectID: uuid.Must(uuid.NewV7()), After: "third"}
	h.Record(third)

	require.False(t, h.CanRedo(), "expected Record to discard the redo tail, but CanRedo() is still true")
	undone, err := h.Undo()
	require.NoError(t, err, "Undo after Record")
	require.Equal(t, "third", undone.After, "expected the most recently recorded op (third) to be undone")
	redone, err := h.Redo()
	require.NoError(t, err, "Redo")
	require.Equal(t, "third", redone.After, "expected Redo to re-apply third (never the truncated second)")
	_, err = h.Redo()
	require.ErrorContains(t, err, "GOLC_HISTORY_NOTHING_TO_REDO", "expected the truncated 'second' op to never resurface via Redo")
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
	require.NoError(t, err, "expected the first undo to reverse the scene edit")
	require.Equal(t, "scene", first.ObjectType, "expected the first undo to reverse the scene edit")
	second, err := h.Undo()
	require.NoError(t, err, "expected the second undo to reverse the chase edit")
	require.Equal(t, "chase", second.ObjectType, "expected the second undo to reverse the chase edit")
	third, err := h.Undo()
	require.NoError(t, err, "expected the third undo to reverse the theme edit")
	require.Equal(t, "theme", third.ObjectType, "expected the third undo to reverse the theme edit")
	_, err = h.Undo()
	require.ErrorContains(t, err, "GOLC_HISTORY_NOTHING_TO_UNDO", "expected the stack to be exhausted after undoing all three mixed-type edits")

	// Redo walks forward in the same single order: theme, then chase, then
	// scene.
	redoneTheme, err := h.Redo()
	require.NoError(t, err, "expected the first redo to reapply the theme edit")
	require.Equal(t, "theme", redoneTheme.ObjectType, "expected the first redo to reapply the theme edit")
	redoneChase, err := h.Redo()
	require.NoError(t, err, "expected the second redo to reapply the chase edit")
	require.Equal(t, "chase", redoneChase.ObjectType, "expected the second redo to reapply the chase edit")
	redoneScene, err := h.Redo()
	require.NoError(t, err, "expected the third redo to reapply the scene edit")
	require.Equal(t, "scene", redoneScene.ObjectType, "expected the third redo to reapply the scene edit")
}
