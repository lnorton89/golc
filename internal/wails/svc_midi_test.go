// svc_midi_test.go proves 06-08-PLAN.md Task 2's acceptance criteria
// against gitlab.com/gomidi/midi/v2/drivers/testdrv's mock driver (never
// midicatdrv -- see internal/midi/driver_test.go's doc comment for why
// that package must never be imported from a test binary) plus 06-03's
// pure midi package logic: a learn round-trip persists a mapping
// (TestMidiServiceStartLearnPersistsMapping), a colliding candidate is
// rejected outright with the prior mapping left untouched while the same
// tuple remains free on a different surface
// (TestMidiServiceStartLearnRejectsConflictOnSameSurfaceButNotOther, D-06/
// D-07), only controls assigned to the surface are learnable
// (TestMidiServiceStartLearnRejectsUnassignedControl, D-08), and a fader
// mapping's cross-to-catch soft takeover only controls after crossing
// while still emitting live position throughout, alongside a Note mapping
// acting immediately with no arming delay
// (TestMidiServiceFaderTakeoverCrossToCatchAndButtonActsImmediately,
// D-09..D-12).
package wails

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	gomidi "gitlab.com/gomidi/midi/v2"
	"gitlab.com/gomidi/midi/v2/drivers/testdrv"

	"github.com/lnorton89/golc/internal/midi"
	"github.com/lnorton89/golc/internal/operatorsurface"
	"github.com/lnorton89/golc/internal/show"
)

// newMidiTestFixture constructs a fresh ShowState at a temp path and a
// MidiService wired to an isolated testdrv-backed *midi.Driver, returning
// the service, the temp show's root/showPath, and the testdrv out port
// tests use to inject synthetic MIDI bytes.
func newMidiTestFixture(t *testing.T, name string) (svc *MidiService, root, showPath string, out testdrvOut) {
	t.Helper()
	root = t.TempDir()
	showPath = filepath.Join(t.TempDir(), "show.golc")

	testDrv := testdrv.New(name)
	ins, err := testDrv.Ins()
	if err != nil || len(ins) != 1 {
		t.Fatalf("testdrv.Ins() = %v, %v", ins, err)
	}
	outs, err := testDrv.Outs()
	if err != nil || len(outs) != 1 {
		t.Fatalf("testdrv.Outs() = %v, %v", outs, err)
	}
	if err := outs[0].Open(); err != nil {
		t.Fatalf("out.Open(): %v", err)
	}

	d, err := midi.Open(ins[0])
	if err != nil {
		t.Fatalf("midi.Open: %v", err)
	}

	svc = NewMidiService("", root, showPath)
	if err := svc.AttachDriver(d); err != nil {
		t.Fatalf("AttachDriver: %v", err)
	}
	t.Cleanup(svc.DetachDriver)

	return svc, root, showPath, outs[0]
}

// testdrvOut is the minimal Send surface svc_midi_test.go's fixtures need
// from a testdrv out port -- defined locally rather than importing
// drivers.Out directly to keep the fixture's return type self-documenting.
type testdrvOut interface {
	Send(data []byte) error
}

// waitForLearningActive polls svc's unexported learning field (this file
// is package wails, same as svc_midi.go) until StartLearn has set it,
// bounding the wait so a test fails loudly instead of hanging if
// StartLearn never reaches its capture window.
func waitForLearningActive(t *testing.T, svc *MidiService) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		svc.mu.Lock()
		active := svc.learning != nil
		svc.mu.Unlock()
		if active {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal("timed out waiting for StartLearn to open its capture window")
}

// startLearnAndSend runs StartLearn(surfaceName, ref) in the background,
// waits for its capture window to open, sends msg through out, and
// returns StartLearn's result (or fails the test on timeout).
func startLearnAndSend(t *testing.T, svc *MidiService, surfaceName string, ref ControlRefInput, out testdrvOut, msg gomidi.Message) Result {
	t.Helper()
	resultCh := make(chan Result, 1)
	go func() {
		resultCh <- svc.StartLearn(surfaceName, ref)
	}()
	waitForLearningActive(t, svc)

	if err := out.Send(msg.Bytes()); err != nil {
		t.Fatalf("Send: %v", err)
	}

	select {
	case result := <-resultCh:
		return result
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for StartLearn to return")
		return Result{}
	}
}

// TestMidiServiceStartLearnPersistsMapping proves a full learn round-trip:
// StartLearn blocks until a matching MIDI message arrives, then persists
// the mapping (operatorsurface.AddMidiMapping -> show.Save) reflected by
// ListMappings.
func TestMidiServiceStartLearnPersistsMapping(t *testing.T) {
	svc, root, showPath, out := newMidiTestFixture(t, "test-learn-accept")

	surfaceSvc := NewSurfaceService("", root, showPath)
	if r := surfaceSvc.CreateSurface("Front of House"); r.ExitCode != 0 {
		t.Fatalf("CreateSurface: exit=%d stderr=%s", r.ExitCode, r.Stderr)
	}
	blackout := ControlRefInput{Kind: "safety", Safety: "blackout"}
	if r := surfaceSvc.AssignItem("Front of House", blackout); r.ExitCode != 0 {
		t.Fatalf("AssignItem: exit=%d stderr=%s", r.ExitCode, r.Stderr)
	}

	result := startLearnAndSend(t, svc, "Front of House", blackout, out, gomidi.NoteOn(1, 36, 100))
	if result.ExitCode != 0 {
		t.Fatalf("StartLearn failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	mappings, err := svc.ListMappings("Front of House")
	if err != nil {
		t.Fatalf("ListMappings: %v", err)
	}
	if len(mappings) != 1 {
		t.Fatalf("expected exactly one persisted mapping, got %+v", mappings)
	}
	got := mappings[0]
	if got.Channel != 1 || got.Kind != "note" || got.Number != 36 {
		t.Fatalf("mapping = %+v, want channel=1 kind=note number=36", got)
	}
	if got.Target.Kind != "safety" || got.Target.Safety != "blackout" {
		t.Fatalf("mapping target = %+v, want the blackout safety control", got.Target)
	}
}

// TestMidiServiceStartLearnRejectsConflictOnSameSurfaceButNotOther proves
// D-06 (a colliding candidate is rejected outright, the existing mapping
// left untouched) and D-07 (the identical tuple remains free on a
// different surface).
func TestMidiServiceStartLearnRejectsConflictOnSameSurfaceButNotOther(t *testing.T) {
	svc, root, showPath, out := newMidiTestFixture(t, "test-learn-conflict")

	surfaceSvc := NewSurfaceService("", root, showPath)
	surfaceSvc.CreateSurface("Front of House")
	surfaceSvc.CreateSurface("Backstage")
	blackout := ControlRefInput{Kind: "safety", Safety: "blackout"}
	grand := ControlRefInput{Kind: "master", MasterKind: "grand"}
	surfaceSvc.AssignItem("Front of House", blackout)
	surfaceSvc.AssignItem("Front of House", grand)
	surfaceSvc.AssignItem("Backstage", blackout)

	collidingMsg := gomidi.NoteOn(1, 36, 100)

	first := startLearnAndSend(t, svc, "Front of House", blackout, out, collidingMsg)
	if first.ExitCode != 0 {
		t.Fatalf("first StartLearn failed: exit=%d stderr=%s", first.ExitCode, first.Stderr)
	}

	// Same surface, same (channel, kind, number), a different target
	// control -- rejected outright, the existing mapping left untouched
	// (D-06).
	second := startLearnAndSend(t, svc, "Front of House", grand, out, collidingMsg)
	if second.ExitCode == 0 || !strings.Contains(second.Stderr, "GOLC_MIDI_MAPPING_CONFLICT") {
		t.Fatalf("expected GOLC_MIDI_MAPPING_CONFLICT, got exit=%d stderr=%s", second.ExitCode, second.Stderr)
	}
	mappings, err := svc.ListMappings("Front of House")
	if err != nil {
		t.Fatalf("ListMappings: %v", err)
	}
	if len(mappings) != 1 || mappings[0].Target.Safety != "blackout" {
		t.Fatalf("expected the prior mapping to remain untouched, got %+v", mappings)
	}

	// A different surface's mapping set is independent -- the identical
	// tuple is free there (D-07).
	third := startLearnAndSend(t, svc, "Backstage", blackout, out, collidingMsg)
	if third.ExitCode != 0 {
		t.Fatalf("expected the identical tuple to be learnable on a different surface, got exit=%d stderr=%s", third.ExitCode, third.Stderr)
	}
}

// TestMidiServiceStartLearnRejectsUnassignedControl proves D-08: the
// learnable set is exactly the surface's assignment set -- StartLearn
// against a control never assigned to the surface is rejected immediately
// (command.Authorize), without ever opening a capture window.
func TestMidiServiceStartLearnRejectsUnassignedControl(t *testing.T) {
	svc, root, showPath, _ := newMidiTestFixture(t, "test-learn-unassigned")

	surfaceSvc := NewSurfaceService("", root, showPath)
	surfaceSvc.CreateSurface("Front of House")
	// Blackout is deliberately left unassigned.

	result := svc.StartLearn("Front of House", ControlRefInput{Kind: "safety", Safety: "blackout"})
	if result.ExitCode == 0 || !strings.Contains(result.Stderr, "GOLC_OPERATORSURFACE_LOCKED") {
		t.Fatalf("expected GOLC_OPERATORSURFACE_LOCKED for an unassigned control, got exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	svc.mu.Lock()
	stillIdle := svc.learning == nil
	svc.mu.Unlock()
	if !stillIdle {
		t.Fatal("expected StartLearn to reject before ever opening a capture window")
	}
}

// TestMidiServiceListMappingsResolvesNamesAndLabels proves ListMappings'
// Target carries a resolvable NAME (matching ControlRefInput's
// established name-based contract everywhere else in this package, e.g.
// svc_surface.go's cliSelector/resolveSurfaceControlRef round-trip) rather
// than a raw internal ID, and Label is a human-readable string -- the
// Rule 1 fix this file's own history records (controlRefInputOf originally
// returned raw UUIDs for scene/group targets).
func TestMidiServiceListMappingsResolvesNamesAndLabels(t *testing.T) {
	svc, root, showPath, _ := newMidiTestFixture(t, "test-list-mappings")

	surfaceSvc := NewSurfaceService("", root, showPath)
	surfaceSvc.CreateSurface("Front of House")
	grand := ControlRefInput{Kind: "master", MasterKind: "grand"}
	surfaceSvc.AssignItem("Front of House", grand)

	state, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load: %v", err)
	}
	surface, found := surfaceByName(state.OperatorSurfaces, "Front of House")
	if !found {
		t.Fatal("surface not found")
	}
	grandRef, err := resolveSurfaceControlRef(state, grand)
	if err != nil {
		t.Fatalf("resolveSurfaceControlRef: %v", err)
	}
	surface, err = operatorsurface.AddMidiMapping(surface, operatorsurface.MidiMapping{
		Channel: 3, Kind: operatorsurface.ControlChange, Number: 74, Target: grandRef,
	})
	if err != nil {
		t.Fatalf("AddMidiMapping: %v", err)
	}
	state.OperatorSurfaces = replaceSurfaceByID(state.OperatorSurfaces, surface)
	if err := show.Save(root, showPath, state); err != nil {
		t.Fatalf("show.Save: %v", err)
	}

	mappings, err := svc.ListMappings("Front of House")
	if err != nil {
		t.Fatalf("ListMappings: %v", err)
	}
	if len(mappings) != 1 {
		t.Fatalf("expected exactly one mapping, got %+v", mappings)
	}
	got := mappings[0]
	if got.Target.Kind != "master" || got.Target.MasterKind != "grand" {
		t.Fatalf("Target = %+v, want kind=master masterKind=grand", got.Target)
	}
	if got.Label != "Grand Master" {
		t.Fatalf("Label = %q, want %q", got.Label, "Grand Master")
	}
	if got.Channel != 3 || got.Kind != "control_change" || got.Number != 74 {
		t.Fatalf("mapping = %+v, want channel=3 kind=control_change number=74", got)
	}
}

// TestMidiServiceFaderTakeoverCrossToCatchAndButtonActsImmediately proves
// D-09..D-12 together: a mapped fader (ControlChange) does not control
// before the physical value crosses the ghost/target marker, live
// position is emitted throughout (armed or not), it controls once crossed,
// and a mapped button (Note) acts immediately with Armed=true and no
// arming delay.
func TestMidiServiceFaderTakeoverCrossToCatchAndButtonActsImmediately(t *testing.T) {
	svc, root, showPath, out := newMidiTestFixture(t, "test-takeover")

	surfaceSvc := NewSurfaceService("", root, showPath)
	surfaceSvc.CreateSurface("Front of House")
	grand := ControlRefInput{Kind: "master", MasterKind: "grand"}
	blackout := ControlRefInput{Kind: "safety", Safety: "blackout"}
	surfaceSvc.AssignItem("Front of House", grand)
	surfaceSvc.AssignItem("Front of House", blackout)

	// Seed a fader (CC) mapping and a button (Note) mapping directly
	// against the model -- a live learn round-trip isn't needed just to
	// fixture these.
	state, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load: %v", err)
	}
	surface, found := surfaceByName(state.OperatorSurfaces, "Front of House")
	if !found {
		t.Fatal("surface not found")
	}
	grandRef, err := resolveSurfaceControlRef(state, grand)
	if err != nil {
		t.Fatalf("resolveSurfaceControlRef(grand): %v", err)
	}
	blackoutRef, err := resolveSurfaceControlRef(state, blackout)
	if err != nil {
		t.Fatalf("resolveSurfaceControlRef(blackout): %v", err)
	}
	surface, err = operatorsurface.AddMidiMapping(surface, operatorsurface.MidiMapping{
		Channel: 1, Kind: operatorsurface.ControlChange, Number: 7, Target: grandRef,
	})
	if err != nil {
		t.Fatalf("AddMidiMapping (fader): %v", err)
	}
	surface, err = operatorsurface.AddMidiMapping(surface, operatorsurface.MidiMapping{
		Channel: 1, Kind: operatorsurface.Note, Number: 40, Target: blackoutRef,
	})
	if err != nil {
		t.Fatalf("AddMidiMapping (button): %v", err)
	}
	state.OperatorSurfaces = replaceSurfaceByID(state.OperatorSurfaces, surface)
	if err := show.Save(root, showPath, state); err != nil {
		t.Fatalf("show.Save: %v", err)
	}

	if r := svc.SetActiveSurface("Front of House"); r.ExitCode != 0 {
		t.Fatalf("SetActiveSurface: exit=%d stderr=%s", r.ExitCode, r.Stderr)
	}

	var mu sync.Mutex
	var feedback []MidiFeedback
	svc.events.emit = func(_ context.Context, eventName string, data ...interface{}) {
		if eventName != "midi:feedback" {
			return
		}
		if fb, ok := data[0].(MidiFeedback); ok {
			mu.Lock()
			feedback = append(feedback, fb)
			mu.Unlock()
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc.events.Start(ctx)
	defer svc.events.Stop()

	latestFader := func() (MidiFeedback, bool) {
		mu.Lock()
		defer mu.Unlock()
		for i := len(feedback) - 1; i >= 0; i-- {
			if feedback[i].Kind == string(operatorsurface.ControlChange) {
				return feedback[i], true
			}
		}
		return MidiFeedback{}, false
	}

	// Physical value well below the 0.5 default ghost/target: must not
	// arm/control, but the live position must still be published (D-09).
	if err := out.Send(gomidi.ControlChange(1, 7, 20).Bytes()); err != nil {
		t.Fatalf("Send: %v", err)
	}
	waitForCondition(t, func() bool { _, ok := latestFader(); return ok })

	before, ok := latestFader()
	if !ok {
		t.Fatal("expected at least one fader feedback push before crossing")
	}
	if before.Armed {
		t.Fatalf("expected not armed before crossing, got %+v", before)
	}
	if before.AppValue != defaultTakeoverAppValue {
		t.Fatalf("expected the ghost/target marker to remain at the seeded AppValue before crossing, got %+v", before)
	}
	wantPhysical := float64(20) / 127
	if before.Physical != wantPhysical {
		t.Fatalf("expected live physical position %v before crossing, got %+v", wantPhysical, before)
	}

	// Cross the ghost/target marker: must now control (armed), tracking
	// the physical value.
	if err := out.Send(gomidi.ControlChange(1, 7, 100).Bytes()); err != nil {
		t.Fatalf("Send: %v", err)
	}
	wantPhysicalAfter := float64(100) / 127
	waitForCondition(t, func() bool {
		fb, ok := latestFader()
		return ok && fb.Armed && fb.Physical == wantPhysicalAfter
	})
	after, _ := latestFader()
	if !after.Armed {
		t.Fatalf("expected armed after crossing, got %+v", after)
	}
	if after.AppValue != after.Physical {
		t.Fatalf("expected the controlling AppValue to track the physical value once armed, got %+v", after)
	}

	// A mapped button (Note) acts immediately -- Armed=true with no
	// crossing/arming delay, independent of the fader's own state above
	// (D-12).
	if err := out.Send(gomidi.NoteOn(1, 40, 127).Bytes()); err != nil {
		t.Fatalf("Send: %v", err)
	}
	waitForCondition(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		for _, fb := range feedback {
			if fb.Kind == string(operatorsurface.Note) {
				return true
			}
		}
		return false
	})
	mu.Lock()
	var buttonFeedback MidiFeedback
	for _, fb := range feedback {
		if fb.Kind == string(operatorsurface.Note) {
			buttonFeedback = fb
		}
	}
	mu.Unlock()
	if !buttonFeedback.Armed {
		t.Fatalf("expected a Note mapping to report Armed=true immediately (D-12), got %+v", buttonFeedback)
	}
}

// waitForCondition polls cond until it reports true, bounding the wait so
// a test fails loudly instead of hanging if the throttled emit loop never
// flushes the expected feedback.
func waitForCondition(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting for expected MIDI feedback")
}
