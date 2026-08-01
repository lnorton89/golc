// model_test.go proves the operator-surface model's identity, idempotent
// assignment, and MIDI-conflict-rejection contract (06-01-PLAN.md Task 1):
// NewSurface mints an ID once and rejects an empty name; Rename never
// re-mints that ID; re-assigning an already-assigned item is a no-op
// (PLAY-03); AddMidiMapping rejects a colliding (channel, kind, number)
// tuple and leaves the prior mapping untouched (D-06); and every mutator
// returns a fresh copy, never aliasing the caller's own slices.
package operatorsurface_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/operatorsurface"
	"github.com/lnorton89/golc/internal/scene"
)

func mustNewUUID(t *testing.T) uuid.UUID {
	t.Helper()
	id, err := uuid.NewV7()
	require.NoError(t, err, "uuid.NewV7")
	return id
}

func TestSurfaceModelIdentityStable(t *testing.T) {
	s, err := operatorsurface.NewSurface("Front of House")
	require.NoError(t, err, "NewSurface")
	require.NotEqual(t, uuid.UUID{}, s.ID, "expected NewSurface to mint a non-zero ID")
	originalID := s.ID

	renamed, err := operatorsurface.Rename(s, "Front of House (renamed)")
	require.NoError(t, err, "Rename")
	require.Equal(t, originalID, renamed.ID, "expected ID to survive rename")
	require.Equal(t, "Front of House (renamed)", renamed.Name, "expected renamed surface to carry its new name")

	_, err = operatorsurface.NewSurface("")
	require.ErrorContains(t, err, "GOLC_OPERATORSURFACE_NAME_EMPTY", "expected GOLC_OPERATORSURFACE_NAME_EMPTY for an empty name")
	_, err = operatorsurface.Rename(s, "")
	require.ErrorContains(t, err, "GOLC_OPERATORSURFACE_NAME_EMPTY", "expected GOLC_OPERATORSURFACE_NAME_EMPTY for an empty rename")
}

func TestSurfaceModelAssignSceneIdempotent(t *testing.T) {
	s, err := operatorsurface.NewSurface("Front of House")
	require.NoError(t, err, "NewSurface")
	sceneID := mustNewUUID(t)

	once := operatorsurface.AssignScene(s, sceneID)
	require.Len(t, once.SceneRefs, 1, "expected exactly one scene ref after first assign")
	require.Equal(t, sceneID, once.SceneRefs[0])

	// Re-assigning the same scene is an idempotent no-op (PLAY-03): the
	// membership set is unchanged, never duplicated.
	twice := operatorsurface.AssignScene(once, sceneID)
	require.Len(t, twice.SceneRefs, 1, "expected re-assigning an already-assigned scene to be a no-op")
	require.Equal(t, sceneID, twice.SceneRefs[0])

	unassigned := operatorsurface.UnassignScene(twice, sceneID)
	require.Empty(t, unassigned.SceneRefs, "expected scene to be removed after unassign")

	// Unassigning an item not present is a no-op, never an error.
	unassignedAgain := operatorsurface.UnassignScene(unassigned, sceneID)
	require.Empty(t, unassignedAgain.SceneRefs, "expected unassigning an absent scene to remain a no-op")
}

func TestSurfaceModelAssignLayerMasterSafety(t *testing.T) {
	s, err := operatorsurface.NewSurface("Front of House")
	require.NoError(t, err, "NewSurface")
	sceneID := mustNewUUID(t)
	groupID := mustNewUUID(t)

	layerRef := operatorsurface.LayerRef{SceneID: sceneID, Kind: scene.ColorTheme}
	withLayer := operatorsurface.AssignLayer(s, layerRef)
	withLayerAgain := operatorsurface.AssignLayer(withLayer, layerRef)
	require.Len(t, withLayerAgain.LayerRefs, 1, "expected re-assigning an already-assigned layer to be a no-op")
	withoutLayer := operatorsurface.UnassignLayer(withLayerAgain, layerRef)
	require.Empty(t, withoutLayer.LayerRefs, "expected layer to be removed after unassign")

	grandMasterRef := operatorsurface.MasterRef{Kind: operatorsurface.GrandMaster}
	groupMasterRef := operatorsurface.MasterRef{Kind: operatorsurface.GroupMaster, GroupID: groupID}
	withMasters := operatorsurface.AssignMaster(operatorsurface.AssignMaster(s, grandMasterRef), groupMasterRef)
	require.Len(t, withMasters.MasterRefs, 2, "expected two distinct master refs")
	withMastersAgain := operatorsurface.AssignMaster(withMasters, grandMasterRef)
	require.Len(t, withMastersAgain.MasterRefs, 2, "expected re-assigning an already-assigned master to be a no-op")
	withoutGrandMaster := operatorsurface.UnassignMaster(withMastersAgain, grandMasterRef)
	require.Len(t, withoutGrandMaster.MasterRefs, 1, "expected only the group master ref to remain")
	require.Equal(t, operatorsurface.GroupMaster, withoutGrandMaster.MasterRefs[0].Kind)

	withSafety := operatorsurface.AssignSafety(s, operatorsurface.RevokeAutomation)
	withSafetyAgain := operatorsurface.AssignSafety(withSafety, operatorsurface.RevokeAutomation)
	require.Len(t, withSafetyAgain.SafetyRefs, 1, "expected re-assigning an already-assigned safety control to be a no-op")
	withoutSafety := operatorsurface.UnassignSafety(withSafetyAgain, operatorsurface.RevokeAutomation)
	require.Empty(t, withoutSafety.SafetyRefs, "expected safety control to be removed after unassign")
}

func TestSurfaceModelMidiMappingConflictRejected(t *testing.T) {
	s, err := operatorsurface.NewSurface("Front of House")
	require.NoError(t, err, "NewSurface")
	sceneID := mustNewUUID(t)
	s = operatorsurface.AssignScene(s, sceneID)

	first, err := operatorsurface.AddMidiMapping(s, operatorsurface.MidiMapping{
		Channel: 1,
		Kind:    operatorsurface.Note,
		Number:  36,
		Target:  operatorsurface.SceneControlRef(sceneID),
	})
	require.NoError(t, err, "AddMidiMapping (first)")
	require.Len(t, first.MidiMappings, 1, "expected exactly one minted mapping")
	require.NotEqual(t, uuid.UUID{}, first.MidiMappings[0].ID)

	// A second mapping with the identical (channel, kind, number) tuple is
	// rejected outright -- the existing mapping is left untouched, never
	// silently overwritten and never last-writer-wins (D-06).
	_, err = operatorsurface.AddMidiMapping(first, operatorsurface.MidiMapping{
		Channel: 1,
		Kind:    operatorsurface.Note,
		Number:  36,
		Target:  operatorsurface.SceneControlRef(sceneID),
	})
	require.ErrorContains(t, err, "GOLC_OPERATORSURFACE_MIDI_CONFLICT", "expected GOLC_OPERATORSURFACE_MIDI_CONFLICT for a colliding mapping")
	require.Len(t, first.MidiMappings, 1, "expected the prior mapping set to remain untouched after a rejected conflict")

	// A non-conflicting mapping (different Number) is appended normally.
	second, err := operatorsurface.AddMidiMapping(first, operatorsurface.MidiMapping{
		Channel: 1,
		Kind:    operatorsurface.Note,
		Number:  37,
		Target:  operatorsurface.SceneControlRef(sceneID),
	})
	require.NoError(t, err, "AddMidiMapping (non-conflicting)")
	require.Len(t, second.MidiMappings, 2, "expected two mappings after a non-conflicting add")
}

// TestSurfaceModelRemoveMidiMapping proves RemoveMidiMapping (06-08-PLAN.md
// Task 2's RemoveMapping) removes exactly the mapping matching mappingID,
// leaves every other mapping untouched, and is an idempotent no-op when
// the ID is not present -- mirroring every other Unassign* mutator's
// idempotent-if-absent discipline.
func TestSurfaceModelRemoveMidiMapping(t *testing.T) {
	s, err := operatorsurface.NewSurface("Front of House")
	require.NoError(t, err, "NewSurface")
	sceneID := mustNewUUID(t)
	s = operatorsurface.AssignScene(s, sceneID)

	withFirst, err := operatorsurface.AddMidiMapping(s, operatorsurface.MidiMapping{
		Channel: 1, Kind: operatorsurface.Note, Number: 36, Target: operatorsurface.SceneControlRef(sceneID),
	})
	require.NoError(t, err, "AddMidiMapping (first)")
	withBoth, err := operatorsurface.AddMidiMapping(withFirst, operatorsurface.MidiMapping{
		Channel: 1, Kind: operatorsurface.Note, Number: 37, Target: operatorsurface.SceneControlRef(sceneID),
	})
	require.NoError(t, err, "AddMidiMapping (second)")
	require.Len(t, withBoth.MidiMappings, 2, "expected two mappings before removal")

	removedID := withBoth.MidiMappings[0].ID
	keptID := withBoth.MidiMappings[1].ID

	afterRemove := operatorsurface.RemoveMidiMapping(withBoth, removedID)
	require.Len(t, afterRemove.MidiMappings, 1, "expected exactly one mapping remaining")
	require.Equal(t, keptID, afterRemove.MidiMappings[0].ID)
	require.Len(t, withBoth.MidiMappings, 2, "expected the caller's own Surface value to be unaffected")

	// Removing an ID not present is an idempotent no-op.
	noop := operatorsurface.RemoveMidiMapping(afterRemove, mustNewUUID(t))
	require.Len(t, noop.MidiMappings, 1, "expected removing an absent ID to be a no-op")
}

func TestSurfaceModelMutationsCopyReturning(t *testing.T) {
	s, err := operatorsurface.NewSurface("Front of House")
	require.NoError(t, err, "NewSurface")
	sceneID := mustNewUUID(t)
	s = operatorsurface.AssignScene(s, sceneID)

	original := append([]uuid.UUID(nil), s.SceneRefs...)
	mutated := operatorsurface.AssignScene(s, mustNewUUID(t))

	require.Len(t, s.SceneRefs, len(original), "expected the caller's own Surface value to be unaffected by a later mutation")
	require.NotEqual(t, len(s.SceneRefs), len(mutated.SceneRefs), "expected the mutated copy to diverge from the original")
}

func TestSurfaceModelIsAssigned(t *testing.T) {
	s, err := operatorsurface.NewSurface("Front of House")
	require.NoError(t, err, "NewSurface")
	sceneID := mustNewUUID(t)
	unassignedSceneID := mustNewUUID(t)
	s = operatorsurface.AssignScene(s, sceneID)

	require.True(t, s.IsAssigned(operatorsurface.SceneControlRef(sceneID)), "expected an assigned scene to report IsAssigned=true")
	require.False(t, s.IsAssigned(operatorsurface.SceneControlRef(unassignedSceneID)), "expected an unassigned scene to report IsAssigned=false")
}

func TestSurfaceModelUniqueNamesRejected(t *testing.T) {
	first := operatorsurface.Surface{Name: "Front of House"}
	second := operatorsurface.Surface{Name: "Front of House"}
	err := operatorsurface.ValidateUniqueSurfaceNames([]operatorsurface.Surface{first, second})
	require.ErrorContains(t, err, "GOLC_OPERATORSURFACE_DUPLICATE_NAME", "expected GOLC_OPERATORSURFACE_DUPLICATE_NAME for duplicate surface names")
	err = operatorsurface.ValidateUniqueSurfaceNames([]operatorsurface.Surface{first, {Name: "Monitor Desk"}})
	require.NoError(t, err, "expected distinctly named surfaces to be valid")
}
