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
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	gomidi "gitlab.com/gomidi/midi/v2"
	"gitlab.com/gomidi/midi/v2/drivers/testdrv"

	"github.com/lnorton89/golc/internal/artnet/ipc"
	"github.com/lnorton89/golc/internal/midi"
	"github.com/lnorton89/golc/internal/operatorsurface"
	"github.com/lnorton89/golc/internal/scene"
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
// TestMidiServiceCancelLearnDoubleCallDoesNotPanic proves CR-02's fix:
// calling CancelLearn twice in succession while a learn session is active
// never double-closes session.cancel (which previously panicked with
// "close of closed channel") -- the second call instead observes the
// already-cancelled state (s.learning nil'd under the lock by the first
// call) and returns GOLC_MIDI_LEARN_NOT_ACTIVE.
func TestMidiServiceCancelLearnDoubleCallDoesNotPanic(t *testing.T) {
	svc, root, showPath, _ := newMidiTestFixture(t, "test-cancel-learn-double")

	surfaceSvc := NewSurfaceService("", root, showPath)
	if r := surfaceSvc.CreateSurface("Front of House"); r.ExitCode != 0 {
		t.Fatalf("CreateSurface: exit=%d stderr=%s", r.ExitCode, r.Stderr)
	}
	blackout := ControlRefInput{Kind: "safety", Safety: "blackout"}
	if r := surfaceSvc.AssignItem("Front of House", blackout); r.ExitCode != 0 {
		t.Fatalf("AssignItem: exit=%d stderr=%s", r.ExitCode, r.Stderr)
	}

	resultCh := make(chan Result, 1)
	go func() {
		resultCh <- svc.StartLearn("Front of House", blackout)
	}()
	waitForLearningActive(t, svc)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("CancelLearn panicked on a double call: %v", r)
		}
	}()

	first := svc.CancelLearn()
	if first.ExitCode != 0 {
		t.Fatalf("first CancelLearn failed: exit=%d stderr=%s", first.ExitCode, first.Stderr)
	}
	second := svc.CancelLearn()
	if second.ExitCode == 0 {
		t.Fatal("expected the second CancelLearn to fail (no session active), got success")
	}
	if !strings.Contains(second.Stderr, "GOLC_MIDI_LEARN_NOT_ACTIVE") {
		t.Fatalf("expected GOLC_MIDI_LEARN_NOT_ACTIVE in stderr, got %q", second.Stderr)
	}

	select {
	case r := <-resultCh:
		if r.ExitCode == 0 {
			t.Fatalf("expected StartLearn to fail after cancellation, got success: %+v", r)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for StartLearn to return after CancelLearn")
	}
}

// TestMidiServiceCancelLearnConcurrentDoubleCallDoesNotPanic proves the
// exact double-click race CR-02 describes (MidiLearn.tsx's Cancel button
// not de-duping/disabling itself before the first click's async result
// resolves): two goroutines calling CancelLearn concurrently while a learn
// session is active must never race into a double close(session.cancel)
// panic -- the mutex-guarded nil-out in CancelLearn (mirroring StartLearn's
// own deferred cleanup discipline) ensures only one of the two ever
// observes a non-nil session and actually closes it.
func TestMidiServiceCancelLearnConcurrentDoubleCallDoesNotPanic(t *testing.T) {
	svc, root, showPath, _ := newMidiTestFixture(t, "test-cancel-learn-concurrent")

	surfaceSvc := NewSurfaceService("", root, showPath)
	if r := surfaceSvc.CreateSurface("Front of House"); r.ExitCode != 0 {
		t.Fatalf("CreateSurface: exit=%d stderr=%s", r.ExitCode, r.Stderr)
	}
	blackout := ControlRefInput{Kind: "safety", Safety: "blackout"}
	if r := surfaceSvc.AssignItem("Front of House", blackout); r.ExitCode != 0 {
		t.Fatalf("AssignItem: exit=%d stderr=%s", r.ExitCode, r.Stderr)
	}

	go func() {
		svc.StartLearn("Front of House", blackout)
	}()
	waitForLearningActive(t, svc)

	var wg sync.WaitGroup
	results := make([]Result, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i] = svc.CancelLearn()
		}(i)
	}
	wg.Wait()

	successes := 0
	for _, r := range results {
		if r.ExitCode == 0 {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("expected exactly one of the two concurrent CancelLearn calls to succeed, got %d successes: %+v", successes, results)
	}
}

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
	// 06-UI-SPEC.md's exact mapping-conflict copy embeds the conflicting
	// control's own label ("Blackout"), not the newly-attempted target's.
	if !strings.Contains(second.Stderr, "already mapped to \"Blackout\"") {
		t.Fatalf("expected the UI-SPEC mapping-conflict copy naming the existing control, got stderr=%s", second.Stderr)
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

// The tests below (06-09-PLAN.md Gap B[1] closure) prove
// dispatchToActiveSurface builds and executes the command implied by a
// matched mapping's ControlRef Target -- not merely feedback state:
// TestMidiServiceDispatchSceneNoteSwitchesActiveScene,
// TestMidiServiceDispatchLayerNoteTogglesEnabledPreservingRef,
// TestMidiServiceDispatchMasterCcForwardsOnlyAfterCrossing,
// TestMidiServiceDispatchSafetyNoteForwardsDaemonRoute, and
// TestMidiServiceDispatchUnmappedEventDoesNothing (Task 1), plus the edge
// coverage in TestMidiServiceDispatchSceneEdgeFiresPerPressNotPerMessage,
// TestMidiServiceDispatchMasterCcContinuesWhileArmed, and
// TestMidiServiceDispatchDeletedTargetIsSilentNoOp (Task 3). These fail
// against the pre-Task-2 feedback-only dispatchToActiveSurface: scene/layer
// assertions fail because state never changes, and master/safety
// assertions fail because the injected dial never captures a Request.

// dispatchCapture collects every ipc.Request forwarded during a test,
// guarded by a mutex since dispatchLoop delivers driver events from its own
// goroutine (unlike svc_safety_test.go's synchronous call-under-test).
type dispatchCapture struct {
	mu       sync.Mutex
	requests []ipc.Request
}

func (c *dispatchCapture) dial(_ string, request ipc.Request) ipc.Result {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.requests = append(c.requests, request)
	return ipc.Result{}
}

func (c *dispatchCapture) all() []ipc.Request {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]ipc.Request(nil), c.requests...)
}

func (c *dispatchCapture) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.requests)
}

// waitForDispatchCount polls capture until it holds at least want requests.
func waitForDispatchCount(t *testing.T, capture *dispatchCapture, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if capture.count() >= want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for at least %d dispatched requests, got %d", want, capture.count())
}

// loadShowWithRetry re-reads the ShowState at root/showPath, retrying a
// transient "database is locked" (SQLITE_BUSY) diagnostic before failing.
// This is observed even for a single show.Load immediately following a
// polling loop's own successful read: the show store's SQLite backend sets
// no busy_timeout and performs no retry of its own (internal/show/
// schema.go), and Windows' file-locking semantics can leave a just-closed
// handle's lock briefly outstanding -- mirrors svc_midi.go's own
// showLoadWithRetry production-side rationale, applied here so a test's own
// post-wait assertion read isn't itself a source of flakiness distinct from
// the dispatch behavior under test.
func loadShowWithRetry(t *testing.T, root, showPath string) show.State {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		state, err := show.Load(root, showPath)
		if err == nil {
			return state
		}
		lastErr = err
		if !strings.Contains(err.Error(), "database is locked") {
			t.Fatalf("show.Load: %v", err)
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("show.Load: repeated database-is-locked retries exhausted: %v", lastErr)
	return show.State{}
}

// waitForSceneActive polls the ShowState at root/showPath until sceneName
// is the active scene.
func waitForSceneActive(t *testing.T, root, showPath, sceneName string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		state, err := show.Load(root, showPath)
		if err == nil {
			for _, sc := range state.Scenes {
				if sc.Name == sceneName && sc.Active {
					return
				}
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for scene %q to become active", sceneName)
}

// waitForLayerEnabled polls the ShowState at root/showPath until
// sceneName/kind's layer Enabled flag matches want.
func waitForLayerEnabled(t *testing.T, root, showPath, sceneName, kind string, want bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		state, err := show.Load(root, showPath)
		if err == nil {
			for _, sc := range state.Scenes {
				if sc.Name != sceneName {
					continue
				}
				for _, l := range sc.Layers {
					if string(l.Kind) == kind && l.Enabled == want {
						return
					}
				}
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s/%s Enabled=%v", sceneName, kind, want)
}

// assertMasterSetForward asserts got is an "artnet master set" Request
// carrying --grand wantLevel and --source manual.
func assertMasterSetForward(t *testing.T, got ipc.Request, wantLevel float64) {
	t.Helper()
	if got.Route != "artnet master set" {
		t.Fatalf("forwarded route = %q, want %q", got.Route, "artnet master set")
	}
	idx := -1
	for i, a := range got.Args {
		if a == "--grand" {
			idx = i
			break
		}
	}
	if idx == -1 || idx+1 >= len(got.Args) {
		t.Fatalf("expected --grand in forwarded args, got %v", got.Args)
	}
	gotLevel, err := strconv.ParseFloat(got.Args[idx+1], 64)
	if err != nil {
		t.Fatalf("--grand value %q is not a valid number: %v", got.Args[idx+1], err)
	}
	if math.Abs(gotLevel-wantLevel) > 1e-6 {
		t.Fatalf("forwarded --grand level = %v, want %v", gotLevel, wantLevel)
	}
	found := false
	for i := 0; i < len(got.Args)-1; i++ {
		if got.Args[i] == "--source" && got.Args[i+1] == "manual" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected --source manual in forwarded args, got %v", got.Args)
	}
}

// TestMidiServiceDispatchSceneNoteSwitchesActiveScene proves Gap B[1]'s
// scene half: a Note mapping whose Target is a scene switches the show's
// active scene when pressed (Value>0) -- not merely the on-screen
// armed/ghost marker -- and a following Note-off does not re-switch or
// error.
func TestMidiServiceDispatchSceneNoteSwitchesActiveScene(t *testing.T) {
	svc, root, showPath, out := newMidiTestFixture(t, "test-dispatch-scene-note")

	execRegistry(t, root, "scene", "create", "Bridge", "--bars", "4", "--show", showPath)
	execRegistry(t, root, "scene", "create", "Chorus", "--bars", "4", "--show", showPath)
	execRegistry(t, root, "scene", "activate", "Bridge", "--show", showPath)

	surfaceSvc := NewSurfaceService("", root, showPath)
	if r := surfaceSvc.CreateSurface("Front of House"); r.ExitCode != 0 {
		t.Fatalf("CreateSurface: exit=%d stderr=%s", r.ExitCode, r.Stderr)
	}
	chorus := ControlRefInput{Kind: "scene", Scene: "Chorus"}
	if r := surfaceSvc.AssignItem("Front of House", chorus); r.ExitCode != 0 {
		t.Fatalf("AssignItem: exit=%d stderr=%s", r.ExitCode, r.Stderr)
	}

	state, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load: %v", err)
	}
	surface, found := surfaceByName(state.OperatorSurfaces, "Front of House")
	if !found {
		t.Fatal("surface not found")
	}
	chorusRef, err := resolveSurfaceControlRef(state, chorus)
	if err != nil {
		t.Fatalf("resolveSurfaceControlRef: %v", err)
	}
	surface, err = operatorsurface.AddMidiMapping(surface, operatorsurface.MidiMapping{
		Channel: 1, Kind: operatorsurface.Note, Number: 40, Target: chorusRef,
	})
	if err != nil {
		t.Fatalf("AddMidiMapping: %v", err)
	}
	state.OperatorSurfaces = replaceSurfaceByID(state.OperatorSurfaces, surface)
	if err := show.Save(root, showPath, state); err != nil {
		t.Fatalf("show.Save: %v", err)
	}

	if r := svc.SetActiveSurface("Front of House"); r.ExitCode != 0 {
		t.Fatalf("SetActiveSurface: exit=%d stderr=%s", r.ExitCode, r.Stderr)
	}

	if err := out.Send(gomidi.NoteOn(1, 40, 100).Bytes()); err != nil {
		t.Fatalf("Send: %v", err)
	}
	waitForSceneActive(t, root, showPath, "Chorus")

	// A following Note-off must not error, re-switch, or revert.
	if err := out.Send(gomidi.NoteOff(1, 40).Bytes()); err != nil {
		t.Fatalf("Send (note-off): %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	after := loadShowWithRetry(t, root, showPath)
	for _, sc := range after.Scenes {
		if sc.Name == "Chorus" && !sc.Active {
			t.Fatalf("expected Chorus to remain active after a Note-off, got Active=%v", sc.Active)
		}
		if sc.Name == "Bridge" && sc.Active {
			t.Fatal("expected Bridge to remain inactive after a Note-off on the Chorus mapping")
		}
	}
}

// TestMidiServiceDispatchLayerNoteTogglesEnabledPreservingRef proves Gap
// B[1]'s layer half: a Note mapping whose Target is a layer flips that
// layer's Enabled flag when pressed while preserving its existing Ref
// (mirrors PlaybackService.SetLayerEnabled's WR-01/WR-03 discipline).
func TestMidiServiceDispatchLayerNoteTogglesEnabledPreservingRef(t *testing.T) {
	svc, root, showPath, out := newMidiTestFixture(t, "test-dispatch-layer-note")

	execRegistry(t, root, "scene", "create", "Verse", "--bars", "4", "--show", showPath)
	execRegistry(t, root, "theme", "create", "Warm", "--show", showPath)

	seeded, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load (seed): %v", err)
	}
	if len(seeded.Themes) != 1 {
		t.Fatalf("expected exactly one seeded theme, got %d", len(seeded.Themes))
	}
	themeID := seeded.Themes[0].ID
	execRegistry(t, root, "scene", "layer", "set", "Verse", "--kind", "color_theme", "--ref", themeID.String(), "--show", showPath)

	surfaceSvc := NewSurfaceService("", root, showPath)
	if r := surfaceSvc.CreateSurface("Front of House"); r.ExitCode != 0 {
		t.Fatalf("CreateSurface: exit=%d stderr=%s", r.ExitCode, r.Stderr)
	}
	layerInput := ControlRefInput{Kind: "layer", Scene: "Verse", LayerKind: "color_theme"}
	if r := surfaceSvc.AssignItem("Front of House", layerInput); r.ExitCode != 0 {
		t.Fatalf("AssignItem: exit=%d stderr=%s", r.ExitCode, r.Stderr)
	}

	state, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load: %v", err)
	}
	surface, found := surfaceByName(state.OperatorSurfaces, "Front of House")
	if !found {
		t.Fatal("surface not found")
	}
	layerRef, err := resolveSurfaceControlRef(state, layerInput)
	if err != nil {
		t.Fatalf("resolveSurfaceControlRef: %v", err)
	}
	surface, err = operatorsurface.AddMidiMapping(surface, operatorsurface.MidiMapping{
		Channel: 1, Kind: operatorsurface.Note, Number: 41, Target: layerRef,
	})
	if err != nil {
		t.Fatalf("AddMidiMapping: %v", err)
	}
	state.OperatorSurfaces = replaceSurfaceByID(state.OperatorSurfaces, surface)
	if err := show.Save(root, showPath, state); err != nil {
		t.Fatalf("show.Save: %v", err)
	}

	if r := svc.SetActiveSurface("Front of House"); r.ExitCode != 0 {
		t.Fatalf("SetActiveSurface: exit=%d stderr=%s", r.ExitCode, r.Stderr)
	}

	beforeLayer := findLayer(t, state, "Verse", "color_theme")
	if !beforeLayer.Enabled {
		t.Fatal("expected the seeded layer to start enabled")
	}
	if beforeLayer.Ref != themeID {
		t.Fatalf("expected the seeded layer Ref=%v, got %v", themeID, beforeLayer.Ref)
	}

	if err := out.Send(gomidi.NoteOn(1, 41, 100).Bytes()); err != nil {
		t.Fatalf("Send: %v", err)
	}
	waitForLayerEnabled(t, root, showPath, "Verse", "color_theme", false)

	after := loadShowWithRetry(t, root, showPath)
	toggled := findLayer(t, after, "Verse", "color_theme")
	if toggled.Ref != themeID {
		t.Fatalf("expected Ref to be preserved across the MIDI-driven toggle, got %v want %v", toggled.Ref, themeID)
	}
}

// TestMidiServiceDispatchMasterCcForwardsOnlyAfterCrossing proves Gap B[1]'s
// master half: a CC mapping whose Target is the grand master forwards
// exactly one "artnet master set" daemon Request once the fader crosses the
// seeded ghost/target marker (cross-to-catch, D-11), and forwards nothing
// for pre-arm messages that have not yet crossed.
func TestMidiServiceDispatchMasterCcForwardsOnlyAfterCrossing(t *testing.T) {
	svc, root, showPath, out := newMidiTestFixture(t, "test-dispatch-master-cc")

	surfaceSvc := NewSurfaceService("", root, showPath)
	if r := surfaceSvc.CreateSurface("Front of House"); r.ExitCode != 0 {
		t.Fatalf("CreateSurface: exit=%d stderr=%s", r.ExitCode, r.Stderr)
	}
	grand := ControlRefInput{Kind: "master", MasterKind: "grand"}
	if r := surfaceSvc.AssignItem("Front of House", grand); r.ExitCode != 0 {
		t.Fatalf("AssignItem: exit=%d stderr=%s", r.ExitCode, r.Stderr)
	}

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
		Channel: 1, Kind: operatorsurface.ControlChange, Number: 7, Target: grandRef,
	})
	if err != nil {
		t.Fatalf("AddMidiMapping: %v", err)
	}
	state.OperatorSurfaces = replaceSurfaceByID(state.OperatorSurfaces, surface)
	if err := show.Save(root, showPath, state); err != nil {
		t.Fatalf("show.Save: %v", err)
	}

	capture := &dispatchCapture{}
	svc.dial = capture.dial

	if r := svc.SetActiveSurface("Front of House"); r.ExitCode != 0 {
		t.Fatalf("SetActiveSurface: exit=%d stderr=%s", r.ExitCode, r.Stderr)
	}

	// Below the 0.5 default ghost/target: not yet crossed, must forward
	// nothing.
	if err := out.Send(gomidi.ControlChange(1, 7, 20).Bytes()); err != nil {
		t.Fatalf("Send: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	if got := capture.count(); got != 0 {
		t.Fatalf("expected zero forwards before crossing, got %d: %+v", got, capture.all())
	}

	// Cross the marker: exactly one forward with the crossed value.
	if err := out.Send(gomidi.ControlChange(1, 7, 100).Bytes()); err != nil {
		t.Fatalf("Send: %v", err)
	}
	waitForDispatchCount(t, capture, 1)
	requests := capture.all()
	if len(requests) != 1 {
		t.Fatalf("expected exactly one forward after crossing, got %d: %+v", len(requests), requests)
	}
	assertMasterSetForward(t, requests[0], float64(100)/127)
}

// TestMidiServiceDispatchSafetyNoteForwardsDaemonRoute proves Gap B[1]'s
// safety half: a Note mapping whose Target is a safety control forwards the
// matching "artnet safety ..." daemon route with "--source manual" when
// pressed, and a following Note-off does not re-forward.
func TestMidiServiceDispatchSafetyNoteForwardsDaemonRoute(t *testing.T) {
	svc, root, showPath, out := newMidiTestFixture(t, "test-dispatch-safety-note")

	surfaceSvc := NewSurfaceService("", root, showPath)
	if r := surfaceSvc.CreateSurface("Front of House"); r.ExitCode != 0 {
		t.Fatalf("CreateSurface: exit=%d stderr=%s", r.ExitCode, r.Stderr)
	}
	blackout := ControlRefInput{Kind: "safety", Safety: "blackout"}
	if r := surfaceSvc.AssignItem("Front of House", blackout); r.ExitCode != 0 {
		t.Fatalf("AssignItem: exit=%d stderr=%s", r.ExitCode, r.Stderr)
	}

	state, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load: %v", err)
	}
	surface, found := surfaceByName(state.OperatorSurfaces, "Front of House")
	if !found {
		t.Fatal("surface not found")
	}
	blackoutRef, err := resolveSurfaceControlRef(state, blackout)
	if err != nil {
		t.Fatalf("resolveSurfaceControlRef: %v", err)
	}
	surface, err = operatorsurface.AddMidiMapping(surface, operatorsurface.MidiMapping{
		Channel: 1, Kind: operatorsurface.Note, Number: 50, Target: blackoutRef,
	})
	if err != nil {
		t.Fatalf("AddMidiMapping: %v", err)
	}
	state.OperatorSurfaces = replaceSurfaceByID(state.OperatorSurfaces, surface)
	if err := show.Save(root, showPath, state); err != nil {
		t.Fatalf("show.Save: %v", err)
	}

	capture := &dispatchCapture{}
	svc.dial = capture.dial

	if r := svc.SetActiveSurface("Front of House"); r.ExitCode != 0 {
		t.Fatalf("SetActiveSurface: exit=%d stderr=%s", r.ExitCode, r.Stderr)
	}

	if err := out.Send(gomidi.NoteOn(1, 50, 100).Bytes()); err != nil {
		t.Fatalf("Send: %v", err)
	}
	waitForDispatchCount(t, capture, 1)
	requests := capture.all()
	if len(requests) != 1 {
		t.Fatalf("expected exactly one forward, got %d: %+v", len(requests), requests)
	}
	assertSafetyForward(t, requests[0], "artnet safety blackout", []string{"--on", "true", "--source", "manual"})

	if err := out.Send(gomidi.NoteOff(1, 50).Bytes()); err != nil {
		t.Fatalf("Send (note-off): %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	if got := capture.count(); got != 1 {
		t.Fatalf("expected no additional forward on Note-off, got %d: %+v", got, capture.all())
	}
}

// TestMidiServiceDispatchUnmappedEventDoesNothing proves Gap B[1]'s
// unchanged-behavior guarantee: a message matching no mapping on the active
// surface dispatches nothing and changes no state.
func TestMidiServiceDispatchUnmappedEventDoesNothing(t *testing.T) {
	svc, root, showPath, out := newMidiTestFixture(t, "test-dispatch-unmapped")

	execRegistry(t, root, "scene", "create", "Verse", "--bars", "4", "--show", showPath)

	surfaceSvc := NewSurfaceService("", root, showPath)
	if r := surfaceSvc.CreateSurface("Front of House"); r.ExitCode != 0 {
		t.Fatalf("CreateSurface: exit=%d stderr=%s", r.ExitCode, r.Stderr)
	}

	capture := &dispatchCapture{}
	svc.dial = capture.dial

	if r := svc.SetActiveSurface("Front of House"); r.ExitCode != 0 {
		t.Fatalf("SetActiveSurface: exit=%d stderr=%s", r.ExitCode, r.Stderr)
	}

	before := loadShowWithRetry(t, root, showPath)

	if err := out.Send(gomidi.NoteOn(1, 99, 100).Bytes()); err != nil {
		t.Fatalf("Send: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	after := loadShowWithRetry(t, root, showPath)
	if len(after.Scenes) != len(before.Scenes) {
		t.Fatalf("expected no scene-count change, before=%d after=%d", len(before.Scenes), len(after.Scenes))
	}
	for _, sc := range after.Scenes {
		if sc.Active {
			t.Fatalf("expected no scene to become active from an unmapped event, got %+v", sc)
		}
	}
	if got := capture.count(); got != 0 {
		t.Fatalf("expected zero daemon forwards from an unmapped event, got %d: %+v", got, capture.all())
	}
}

// TestMidiServiceDispatchSceneEdgeFiresPerPressNotPerMessage proves a scene
// Note mapping fires its switch on each activation edge (each Note-on
// press) without erroring on a repeated press, while a Note-off between
// presses dispatches nothing, and the dispatch loop keeps processing
// subsequent mapped events afterward.
func TestMidiServiceDispatchSceneEdgeFiresPerPressNotPerMessage(t *testing.T) {
	svc, root, showPath, out := newMidiTestFixture(t, "test-dispatch-scene-edge")

	execRegistry(t, root, "scene", "create", "Alpha", "--bars", "4", "--show", showPath)
	execRegistry(t, root, "scene", "create", "Beta", "--bars", "4", "--show", showPath)
	execRegistry(t, root, "scene", "create", "Gamma", "--bars", "4", "--show", showPath)
	execRegistry(t, root, "scene", "activate", "Alpha", "--show", showPath)

	surfaceSvc := NewSurfaceService("", root, showPath)
	if r := surfaceSvc.CreateSurface("Front of House"); r.ExitCode != 0 {
		t.Fatalf("CreateSurface: exit=%d stderr=%s", r.ExitCode, r.Stderr)
	}
	beta := ControlRefInput{Kind: "scene", Scene: "Beta"}
	gamma := ControlRefInput{Kind: "scene", Scene: "Gamma"}
	if r := surfaceSvc.AssignItem("Front of House", beta); r.ExitCode != 0 {
		t.Fatalf("AssignItem(beta): exit=%d stderr=%s", r.ExitCode, r.Stderr)
	}
	if r := surfaceSvc.AssignItem("Front of House", gamma); r.ExitCode != 0 {
		t.Fatalf("AssignItem(gamma): exit=%d stderr=%s", r.ExitCode, r.Stderr)
	}

	state, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load: %v", err)
	}
	surface, found := surfaceByName(state.OperatorSurfaces, "Front of House")
	if !found {
		t.Fatal("surface not found")
	}
	betaRef, err := resolveSurfaceControlRef(state, beta)
	if err != nil {
		t.Fatalf("resolveSurfaceControlRef(beta): %v", err)
	}
	gammaRef, err := resolveSurfaceControlRef(state, gamma)
	if err != nil {
		t.Fatalf("resolveSurfaceControlRef(gamma): %v", err)
	}
	surface, err = operatorsurface.AddMidiMapping(surface, operatorsurface.MidiMapping{
		Channel: 1, Kind: operatorsurface.Note, Number: 10, Target: betaRef,
	})
	if err != nil {
		t.Fatalf("AddMidiMapping(beta): %v", err)
	}
	surface, err = operatorsurface.AddMidiMapping(surface, operatorsurface.MidiMapping{
		Channel: 1, Kind: operatorsurface.Note, Number: 11, Target: gammaRef,
	})
	if err != nil {
		t.Fatalf("AddMidiMapping(gamma): %v", err)
	}
	state.OperatorSurfaces = replaceSurfaceByID(state.OperatorSurfaces, surface)
	if err := show.Save(root, showPath, state); err != nil {
		t.Fatalf("show.Save: %v", err)
	}

	if r := svc.SetActiveSurface("Front of House"); r.ExitCode != 0 {
		t.Fatalf("SetActiveSurface: exit=%d stderr=%s", r.ExitCode, r.Stderr)
	}

	// Repeated press without release: fires each time, never errors.
	if err := out.Send(gomidi.NoteOn(1, 10, 100).Bytes()); err != nil {
		t.Fatalf("Send: %v", err)
	}
	waitForSceneActive(t, root, showPath, "Beta")
	if err := out.Send(gomidi.NoteOn(1, 10, 100).Bytes()); err != nil {
		t.Fatalf("Send (repeat press): %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	stillBeta := loadShowWithRetry(t, root, showPath)
	for _, sc := range stillBeta.Scenes {
		if sc.Name == "Beta" && !sc.Active {
			t.Fatal("expected Beta to remain active after a repeated press")
		}
	}

	// A Note-off between presses dispatches nothing.
	if err := out.Send(gomidi.NoteOff(1, 10).Bytes()); err != nil {
		t.Fatalf("Send (note-off): %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	// The dispatch loop keeps processing a subsequent mapped press.
	if err := out.Send(gomidi.NoteOn(1, 11, 100).Bytes()); err != nil {
		t.Fatalf("Send: %v", err)
	}
	waitForSceneActive(t, root, showPath, "Gamma")
}

// TestMidiServiceDispatchMasterCcContinuesWhileArmed proves the deliberate
// asymmetry between a continuous master CC (forwards on every armed
// update) and a discrete scene CC (fires its switch once on the arming
// edge and never re-switches on subsequent armed messages).
func TestMidiServiceDispatchMasterCcContinuesWhileArmed(t *testing.T) {
	svc, root, showPath, out := newMidiTestFixture(t, "test-dispatch-master-continues")

	execRegistry(t, root, "scene", "create", "Alpha", "--bars", "4", "--show", showPath)
	execRegistry(t, root, "scene", "create", "Beta", "--bars", "4", "--show", showPath)
	execRegistry(t, root, "scene", "activate", "Alpha", "--show", showPath)

	surfaceSvc := NewSurfaceService("", root, showPath)
	if r := surfaceSvc.CreateSurface("Front of House"); r.ExitCode != 0 {
		t.Fatalf("CreateSurface: exit=%d stderr=%s", r.ExitCode, r.Stderr)
	}
	grand := ControlRefInput{Kind: "master", MasterKind: "grand"}
	beta := ControlRefInput{Kind: "scene", Scene: "Beta"}
	if r := surfaceSvc.AssignItem("Front of House", grand); r.ExitCode != 0 {
		t.Fatalf("AssignItem(grand): exit=%d stderr=%s", r.ExitCode, r.Stderr)
	}
	if r := surfaceSvc.AssignItem("Front of House", beta); r.ExitCode != 0 {
		t.Fatalf("AssignItem(beta): exit=%d stderr=%s", r.ExitCode, r.Stderr)
	}

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
	betaRef, err := resolveSurfaceControlRef(state, beta)
	if err != nil {
		t.Fatalf("resolveSurfaceControlRef(beta): %v", err)
	}
	surface, err = operatorsurface.AddMidiMapping(surface, operatorsurface.MidiMapping{
		Channel: 1, Kind: operatorsurface.ControlChange, Number: 7, Target: grandRef,
	})
	if err != nil {
		t.Fatalf("AddMidiMapping(grand): %v", err)
	}
	surface, err = operatorsurface.AddMidiMapping(surface, operatorsurface.MidiMapping{
		Channel: 1, Kind: operatorsurface.ControlChange, Number: 8, Target: betaRef,
	})
	if err != nil {
		t.Fatalf("AddMidiMapping(beta): %v", err)
	}
	state.OperatorSurfaces = replaceSurfaceByID(state.OperatorSurfaces, surface)
	if err := show.Save(root, showPath, state); err != nil {
		t.Fatalf("show.Save: %v", err)
	}

	capture := &dispatchCapture{}
	svc.dial = capture.dial

	if r := svc.SetActiveSurface("Front of House"); r.ExitCode != 0 {
		t.Fatalf("SetActiveSurface: exit=%d stderr=%s", r.ExitCode, r.Stderr)
	}

	// A control's very first message can never arm (TakeoverState seeds
	// LastPhysical to NaN, so no crossing check can pass yet) -- establish
	// a below-threshold physical position first, then cross the marker, then
	// hold past it with two further updates -- each must independently
	// forward (continuous).
	if err := out.Send(gomidi.ControlChange(1, 7, 20).Bytes()); err != nil {
		t.Fatalf("Send: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	if got := capture.count(); got != 0 {
		t.Fatalf("expected zero forwards before crossing, got %d: %+v", got, capture.all())
	}
	if err := out.Send(gomidi.ControlChange(1, 7, 100).Bytes()); err != nil {
		t.Fatalf("Send: %v", err)
	}
	waitForDispatchCount(t, capture, 1)
	if err := out.Send(gomidi.ControlChange(1, 7, 110).Bytes()); err != nil {
		t.Fatalf("Send: %v", err)
	}
	waitForDispatchCount(t, capture, 2)
	if err := out.Send(gomidi.ControlChange(1, 7, 120).Bytes()); err != nil {
		t.Fatalf("Send: %v", err)
	}
	waitForDispatchCount(t, capture, 3)

	// Cross the scene CC's own ghost/target marker once (again establishing
	// a below-threshold position first, since its own TakeoverState is
	// independent -- keyed per mapping ID -- and equally cannot arm on its
	// own first message): fires the switch exactly once, then never
	// re-switches on further armed messages, and never dials through the
	// master-set path.
	if err := out.Send(gomidi.ControlChange(1, 8, 20).Bytes()); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if err := out.Send(gomidi.ControlChange(1, 8, 100).Bytes()); err != nil {
		t.Fatalf("Send: %v", err)
	}
	waitForSceneActive(t, root, showPath, "Beta")
	if err := out.Send(gomidi.ControlChange(1, 8, 110).Bytes()); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if err := out.Send(gomidi.ControlChange(1, 8, 120).Bytes()); err != nil {
		t.Fatalf("Send: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	if got := capture.count(); got != 3 {
		t.Fatalf("expected exactly 3 master-set forwards (scene dispatch never dials), got %d: %+v", got, capture.all())
	}
	final := loadShowWithRetry(t, root, showPath)
	for _, sc := range final.Scenes {
		if sc.Name == "Beta" && !sc.Active {
			t.Fatal("expected Beta to remain active (no re-switch needed/attempted)")
		}
	}
}

// TestMidiServiceDispatchDeletedTargetIsSilentNoOp proves a mapping whose
// Target scene was deleted from the show dispatches nothing and does not
// panic, and the dispatch loop continues processing a subsequent, still
// valid mapped event afterward.
func TestMidiServiceDispatchDeletedTargetIsSilentNoOp(t *testing.T) {
	svc, root, showPath, out := newMidiTestFixture(t, "test-dispatch-deleted-target")

	execRegistry(t, root, "scene", "create", "Ghost", "--bars", "4", "--show", showPath)
	execRegistry(t, root, "scene", "create", "Alive", "--bars", "4", "--show", showPath)

	surfaceSvc := NewSurfaceService("", root, showPath)
	if r := surfaceSvc.CreateSurface("Front of House"); r.ExitCode != 0 {
		t.Fatalf("CreateSurface: exit=%d stderr=%s", r.ExitCode, r.Stderr)
	}
	// Ghost is deliberately never assigned to the surface's SceneRefs: the
	// MIDI mapping is added directly (via operatorsurface.AddMidiMapping,
	// mirroring this file's other direct-mapping fixtures), and show.Save
	// itself rejects a surface whose SceneRefs dangle on a deleted scene
	// (GOLC_OPERATORSURFACE_DANGLING_REFERENCE) -- this test is about a
	// mapping's Target outliving its scene, not about surface assignment
	// membership.
	ghost := ControlRefInput{Kind: "scene", Scene: "Ghost"}
	alive := ControlRefInput{Kind: "scene", Scene: "Alive"}
	if r := surfaceSvc.AssignItem("Front of House", alive); r.ExitCode != 0 {
		t.Fatalf("AssignItem(alive): exit=%d stderr=%s", r.ExitCode, r.Stderr)
	}

	state, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load: %v", err)
	}
	surface, found := surfaceByName(state.OperatorSurfaces, "Front of House")
	if !found {
		t.Fatal("surface not found")
	}
	ghostRef, err := resolveSurfaceControlRef(state, ghost)
	if err != nil {
		t.Fatalf("resolveSurfaceControlRef(ghost): %v", err)
	}
	aliveRef, err := resolveSurfaceControlRef(state, alive)
	if err != nil {
		t.Fatalf("resolveSurfaceControlRef(alive): %v", err)
	}
	surface, err = operatorsurface.AddMidiMapping(surface, operatorsurface.MidiMapping{
		Channel: 1, Kind: operatorsurface.Note, Number: 60, Target: ghostRef,
	})
	if err != nil {
		t.Fatalf("AddMidiMapping(ghost): %v", err)
	}
	surface, err = operatorsurface.AddMidiMapping(surface, operatorsurface.MidiMapping{
		Channel: 1, Kind: operatorsurface.Note, Number: 61, Target: aliveRef,
	})
	if err != nil {
		t.Fatalf("AddMidiMapping(alive): %v", err)
	}
	state.OperatorSurfaces = replaceSurfaceByID(state.OperatorSurfaces, surface)

	// Delete the Ghost scene directly from the show, leaving the mapping's
	// Target (the now-nonexistent scene's ID) on the surface untouched --
	// the same read-only projection tolerance sceneNameByID already extends
	// elsewhere in this package.
	filtered := make([]scene.Scene, 0, len(state.Scenes))
	for _, sc := range state.Scenes {
		if sc.Name != "Ghost" {
			filtered = append(filtered, sc)
		}
	}
	state.Scenes = filtered
	if err := show.Save(root, showPath, state); err != nil {
		t.Fatalf("show.Save: %v", err)
	}

	if r := svc.SetActiveSurface("Front of House"); r.ExitCode != 0 {
		t.Fatalf("SetActiveSurface: exit=%d stderr=%s", r.ExitCode, r.Stderr)
	}

	if err := out.Send(gomidi.NoteOn(1, 60, 100).Bytes()); err != nil {
		t.Fatalf("Send: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	// The dispatch loop must keep working afterward.
	if err := out.Send(gomidi.NoteOn(1, 61, 100).Bytes()); err != nil {
		t.Fatalf("Send: %v", err)
	}
	waitForSceneActive(t, root, showPath, "Alive")
}
