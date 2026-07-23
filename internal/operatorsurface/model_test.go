// model_test.go proves the operator-surface model's identity, idempotent
// assignment, and MIDI-conflict-rejection contract (06-01-PLAN.md Task 1):
// NewSurface mints an ID once and rejects an empty name; Rename never
// re-mints that ID; re-assigning an already-assigned item is a no-op
// (PLAY-03); AddMidiMapping rejects a colliding (channel, kind, number)
// tuple and leaves the prior mapping untouched (D-06); and every mutator
// returns a fresh copy, never aliasing the caller's own slices.
package operatorsurface_test

import (
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/operatorsurface"
	"github.com/lnorton89/golc/internal/scene"
)

func mustNewUUID(t *testing.T) uuid.UUID {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("uuid.NewV7: %v", err)
	}
	return id
}

func TestSurfaceModelIdentityStable(t *testing.T) {
	s, err := operatorsurface.NewSurface("Front of House")
	if err != nil {
		t.Fatalf("NewSurface: %v", err)
	}
	if s.ID == (uuid.UUID{}) {
		t.Fatalf("expected NewSurface to mint a non-zero ID")
	}
	originalID := s.ID

	renamed, err := operatorsurface.Rename(s, "Front of House (renamed)")
	if err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if renamed.ID != originalID {
		t.Fatalf("expected ID to survive rename, got %s want %s", renamed.ID, originalID)
	}
	if renamed.Name != "Front of House (renamed)" {
		t.Fatalf("expected renamed surface to carry its new name, got %q", renamed.Name)
	}

	if _, err := operatorsurface.NewSurface(""); err == nil || !strings.Contains(err.Error(), "GOLC_OPERATORSURFACE_NAME_EMPTY") {
		t.Fatalf("expected GOLC_OPERATORSURFACE_NAME_EMPTY for an empty name, got %v", err)
	}
	if _, err := operatorsurface.Rename(s, ""); err == nil || !strings.Contains(err.Error(), "GOLC_OPERATORSURFACE_NAME_EMPTY") {
		t.Fatalf("expected GOLC_OPERATORSURFACE_NAME_EMPTY for an empty rename, got %v", err)
	}
}

func TestSurfaceModelAssignSceneIdempotent(t *testing.T) {
	s, err := operatorsurface.NewSurface("Front of House")
	if err != nil {
		t.Fatalf("NewSurface: %v", err)
	}
	sceneID := mustNewUUID(t)

	once := operatorsurface.AssignScene(s, sceneID)
	if len(once.SceneRefs) != 1 || once.SceneRefs[0] != sceneID {
		t.Fatalf("expected exactly one scene ref after first assign, got %+v", once.SceneRefs)
	}

	// Re-assigning the same scene is an idempotent no-op (PLAY-03): the
	// membership set is unchanged, never duplicated.
	twice := operatorsurface.AssignScene(once, sceneID)
	if len(twice.SceneRefs) != 1 || twice.SceneRefs[0] != sceneID {
		t.Fatalf("expected re-assigning an already-assigned scene to be a no-op, got %+v", twice.SceneRefs)
	}

	unassigned := operatorsurface.UnassignScene(twice, sceneID)
	if len(unassigned.SceneRefs) != 0 {
		t.Fatalf("expected scene to be removed after unassign, got %+v", unassigned.SceneRefs)
	}

	// Unassigning an item not present is a no-op, never an error.
	unassignedAgain := operatorsurface.UnassignScene(unassigned, sceneID)
	if len(unassignedAgain.SceneRefs) != 0 {
		t.Fatalf("expected unassigning an absent scene to remain a no-op, got %+v", unassignedAgain.SceneRefs)
	}
}

func TestSurfaceModelAssignLayerMasterSafety(t *testing.T) {
	s, err := operatorsurface.NewSurface("Front of House")
	if err != nil {
		t.Fatalf("NewSurface: %v", err)
	}
	sceneID := mustNewUUID(t)
	groupID := mustNewUUID(t)

	layerRef := operatorsurface.LayerRef{SceneID: sceneID, Kind: scene.ColorTheme}
	withLayer := operatorsurface.AssignLayer(s, layerRef)
	withLayerAgain := operatorsurface.AssignLayer(withLayer, layerRef)
	if len(withLayerAgain.LayerRefs) != 1 {
		t.Fatalf("expected re-assigning an already-assigned layer to be a no-op, got %+v", withLayerAgain.LayerRefs)
	}
	withoutLayer := operatorsurface.UnassignLayer(withLayerAgain, layerRef)
	if len(withoutLayer.LayerRefs) != 0 {
		t.Fatalf("expected layer to be removed after unassign, got %+v", withoutLayer.LayerRefs)
	}

	grandMasterRef := operatorsurface.MasterRef{Kind: operatorsurface.GrandMaster}
	groupMasterRef := operatorsurface.MasterRef{Kind: operatorsurface.GroupMaster, GroupID: groupID}
	withMasters := operatorsurface.AssignMaster(operatorsurface.AssignMaster(s, grandMasterRef), groupMasterRef)
	if len(withMasters.MasterRefs) != 2 {
		t.Fatalf("expected two distinct master refs, got %+v", withMasters.MasterRefs)
	}
	withMastersAgain := operatorsurface.AssignMaster(withMasters, grandMasterRef)
	if len(withMastersAgain.MasterRefs) != 2 {
		t.Fatalf("expected re-assigning an already-assigned master to be a no-op, got %+v", withMastersAgain.MasterRefs)
	}
	withoutGrandMaster := operatorsurface.UnassignMaster(withMastersAgain, grandMasterRef)
	if len(withoutGrandMaster.MasterRefs) != 1 || withoutGrandMaster.MasterRefs[0].Kind != operatorsurface.GroupMaster {
		t.Fatalf("expected only the group master ref to remain, got %+v", withoutGrandMaster.MasterRefs)
	}

	withSafety := operatorsurface.AssignSafety(s, operatorsurface.RevokeAutomation)
	withSafetyAgain := operatorsurface.AssignSafety(withSafety, operatorsurface.RevokeAutomation)
	if len(withSafetyAgain.SafetyRefs) != 1 {
		t.Fatalf("expected re-assigning an already-assigned safety control to be a no-op, got %+v", withSafetyAgain.SafetyRefs)
	}
	withoutSafety := operatorsurface.UnassignSafety(withSafetyAgain, operatorsurface.RevokeAutomation)
	if len(withoutSafety.SafetyRefs) != 0 {
		t.Fatalf("expected safety control to be removed after unassign, got %+v", withoutSafety.SafetyRefs)
	}
}

func TestSurfaceModelMidiMappingConflictRejected(t *testing.T) {
	s, err := operatorsurface.NewSurface("Front of House")
	if err != nil {
		t.Fatalf("NewSurface: %v", err)
	}
	sceneID := mustNewUUID(t)
	s = operatorsurface.AssignScene(s, sceneID)

	first, err := operatorsurface.AddMidiMapping(s, operatorsurface.MidiMapping{
		Channel: 1,
		Kind:    operatorsurface.Note,
		Number:  36,
		Target:  operatorsurface.SceneControlRef(sceneID),
	})
	if err != nil {
		t.Fatalf("AddMidiMapping (first): %v", err)
	}
	if len(first.MidiMappings) != 1 || first.MidiMappings[0].ID == (uuid.UUID{}) {
		t.Fatalf("expected exactly one minted mapping, got %+v", first.MidiMappings)
	}

	// A second mapping with the identical (channel, kind, number) tuple is
	// rejected outright -- the existing mapping is left untouched, never
	// silently overwritten and never last-writer-wins (D-06).
	_, err = operatorsurface.AddMidiMapping(first, operatorsurface.MidiMapping{
		Channel: 1,
		Kind:    operatorsurface.Note,
		Number:  36,
		Target:  operatorsurface.SceneControlRef(sceneID),
	})
	if err == nil || !strings.Contains(err.Error(), "GOLC_OPERATORSURFACE_MIDI_CONFLICT") {
		t.Fatalf("expected GOLC_OPERATORSURFACE_MIDI_CONFLICT for a colliding mapping, got %v", err)
	}
	if len(first.MidiMappings) != 1 {
		t.Fatalf("expected the prior mapping set to remain untouched after a rejected conflict, got %+v", first.MidiMappings)
	}

	// A non-conflicting mapping (different Number) is appended normally.
	second, err := operatorsurface.AddMidiMapping(first, operatorsurface.MidiMapping{
		Channel: 1,
		Kind:    operatorsurface.Note,
		Number:  37,
		Target:  operatorsurface.SceneControlRef(sceneID),
	})
	if err != nil {
		t.Fatalf("AddMidiMapping (non-conflicting): %v", err)
	}
	if len(second.MidiMappings) != 2 {
		t.Fatalf("expected two mappings after a non-conflicting add, got %+v", second.MidiMappings)
	}
}

// TestSurfaceModelRemoveMidiMapping proves RemoveMidiMapping (06-08-PLAN.md
// Task 2's RemoveMapping) removes exactly the mapping matching mappingID,
// leaves every other mapping untouched, and is an idempotent no-op when
// the ID is not present -- mirroring every other Unassign* mutator's
// idempotent-if-absent discipline.
func TestSurfaceModelRemoveMidiMapping(t *testing.T) {
	s, err := operatorsurface.NewSurface("Front of House")
	if err != nil {
		t.Fatalf("NewSurface: %v", err)
	}
	sceneID := mustNewUUID(t)
	s = operatorsurface.AssignScene(s, sceneID)

	withFirst, err := operatorsurface.AddMidiMapping(s, operatorsurface.MidiMapping{
		Channel: 1, Kind: operatorsurface.Note, Number: 36, Target: operatorsurface.SceneControlRef(sceneID),
	})
	if err != nil {
		t.Fatalf("AddMidiMapping (first): %v", err)
	}
	withBoth, err := operatorsurface.AddMidiMapping(withFirst, operatorsurface.MidiMapping{
		Channel: 1, Kind: operatorsurface.Note, Number: 37, Target: operatorsurface.SceneControlRef(sceneID),
	})
	if err != nil {
		t.Fatalf("AddMidiMapping (second): %v", err)
	}
	if len(withBoth.MidiMappings) != 2 {
		t.Fatalf("expected two mappings before removal, got %+v", withBoth.MidiMappings)
	}

	removedID := withBoth.MidiMappings[0].ID
	keptID := withBoth.MidiMappings[1].ID

	afterRemove := operatorsurface.RemoveMidiMapping(withBoth, removedID)
	if len(afterRemove.MidiMappings) != 1 {
		t.Fatalf("expected exactly one mapping remaining, got %+v", afterRemove.MidiMappings)
	}
	if afterRemove.MidiMappings[0].ID != keptID {
		t.Fatalf("expected the kept mapping's ID to be %v, got %v", keptID, afterRemove.MidiMappings[0].ID)
	}
	if len(withBoth.MidiMappings) != 2 {
		t.Fatalf("expected the caller's own Surface value to be unaffected, got %+v", withBoth.MidiMappings)
	}

	// Removing an ID not present is an idempotent no-op.
	noop := operatorsurface.RemoveMidiMapping(afterRemove, mustNewUUID(t))
	if len(noop.MidiMappings) != 1 {
		t.Fatalf("expected removing an absent ID to be a no-op, got %+v", noop.MidiMappings)
	}
}

func TestSurfaceModelMutationsCopyReturning(t *testing.T) {
	s, err := operatorsurface.NewSurface("Front of House")
	if err != nil {
		t.Fatalf("NewSurface: %v", err)
	}
	sceneID := mustNewUUID(t)
	s = operatorsurface.AssignScene(s, sceneID)

	original := append([]uuid.UUID(nil), s.SceneRefs...)
	mutated := operatorsurface.AssignScene(s, mustNewUUID(t))

	if len(s.SceneRefs) != len(original) {
		t.Fatalf("expected the caller's own Surface value to be unaffected by a later mutation, got %+v", s.SceneRefs)
	}
	if len(mutated.SceneRefs) == len(s.SceneRefs) {
		t.Fatalf("expected the mutated copy to diverge from the original, both have %d refs", len(s.SceneRefs))
	}
}

func TestSurfaceModelIsAssigned(t *testing.T) {
	s, err := operatorsurface.NewSurface("Front of House")
	if err != nil {
		t.Fatalf("NewSurface: %v", err)
	}
	sceneID := mustNewUUID(t)
	unassignedSceneID := mustNewUUID(t)
	s = operatorsurface.AssignScene(s, sceneID)

	if !s.IsAssigned(operatorsurface.SceneControlRef(sceneID)) {
		t.Fatalf("expected an assigned scene to report IsAssigned=true")
	}
	if s.IsAssigned(operatorsurface.SceneControlRef(unassignedSceneID)) {
		t.Fatalf("expected an unassigned scene to report IsAssigned=false")
	}
}

func TestSurfaceModelUniqueNamesRejected(t *testing.T) {
	first := operatorsurface.Surface{Name: "Front of House"}
	second := operatorsurface.Surface{Name: "Front of House"}
	if err := operatorsurface.ValidateUniqueSurfaceNames([]operatorsurface.Surface{first, second}); err == nil ||
		!strings.Contains(err.Error(), "GOLC_OPERATORSURFACE_DUPLICATE_NAME") {
		t.Fatalf("expected GOLC_OPERATORSURFACE_DUPLICATE_NAME for duplicate surface names, got %v", err)
	}
	if err := operatorsurface.ValidateUniqueSurfaceNames([]operatorsurface.Surface{first, {Name: "Monitor Desk"}}); err != nil {
		t.Fatalf("expected distinctly named surfaces to be valid, got %v", err)
	}
}
