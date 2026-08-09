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
	"fmt"
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	gomidi "gitlab.com/gomidi/midi/v2"
	"gitlab.com/gomidi/midi/v2/drivers/testdrv"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/artnet/ipc"
	"github.com/lnorton89/golc/internal/deployment"
	"github.com/lnorton89/golc/internal/deskmidi"
	"github.com/lnorton89/golc/internal/midi"
	"github.com/lnorton89/golc/internal/operatorsurface"
	"github.com/lnorton89/golc/internal/pool"
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
	require.True(t, err == nil && len(ins) == 1, "testdrv.Ins() = %v, %v", ins, err)
	outs, err := testDrv.Outs()
	require.True(t, err == nil && len(outs) == 1, "testdrv.Outs() = %v, %v", outs, err)
	err = outs[0].Open()
	require.NoError(t, err, "out.Open()")

	d, err := midi.Open(ins[0])
	require.NoError(t, err, "midi.Open")

	svc = NewMidiService("", root, showPath)
	err = svc.AttachDriver(d)
	require.NoError(t, err, "AttachDriver")
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
	require.Fail(t, "timed out waiting for StartLearn to open its capture window")
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

	err := out.Send(msg.Bytes())
	require.NoError(t, err, "Send")

	select {
	case result := <-resultCh:
		return result
	case <-time.After(3 * time.Second):
		require.Fail(t, "timed out waiting for StartLearn to return")
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
	r := surfaceSvc.CreateSurface("Front of House")
	require.Equal(t, 0, r.ExitCode, "CreateSurface: stderr=%s", r.Stderr)
	blackout := ControlRefInput{Kind: "safety", Safety: "blackout"}
	r = surfaceSvc.AssignItem("Front of House", blackout)
	require.Equal(t, 0, r.ExitCode, "AssignItem: stderr=%s", r.Stderr)

	resultCh := make(chan Result, 1)
	go func() {
		resultCh <- svc.StartLearn("Front of House", blackout)
	}()
	waitForLearningActive(t, svc)

	defer func() {
		if rec := recover(); rec != nil {
			require.Fail(t, "CancelLearn panicked on a double call", "%v", rec)
		}
	}()

	first := svc.CancelLearn()
	require.Equal(t, 0, first.ExitCode, "first CancelLearn failed: stderr=%s", first.Stderr)
	second := svc.CancelLearn()
	require.NotEqual(t, 0, second.ExitCode, "expected the second CancelLearn to fail (no session active), got success")
	require.Contains(t, second.Stderr, "GOLC_MIDI_LEARN_NOT_ACTIVE")

	select {
	case sr := <-resultCh:
		require.NotEqual(t, 0, sr.ExitCode, "expected StartLearn to fail after cancellation, got success: %+v", sr)
	case <-time.After(3 * time.Second):
		require.Fail(t, "timed out waiting for StartLearn to return after CancelLearn")
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
	r := surfaceSvc.CreateSurface("Front of House")
	require.Equal(t, 0, r.ExitCode, "CreateSurface: stderr=%s", r.Stderr)
	blackout := ControlRefInput{Kind: "safety", Safety: "blackout"}
	r = surfaceSvc.AssignItem("Front of House", blackout)
	require.Equal(t, 0, r.ExitCode, "AssignItem: stderr=%s", r.Stderr)

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
	for _, res := range results {
		if res.ExitCode == 0 {
			successes++
		}
	}
	require.Equal(t, 1, successes, "expected exactly one of the two concurrent CancelLearn calls to succeed: %+v", results)
}

func TestMidiServiceStartLearnPersistsMapping(t *testing.T) {
	svc, root, showPath, out := newMidiTestFixture(t, "test-learn-accept")

	surfaceSvc := NewSurfaceService("", root, showPath)
	r := surfaceSvc.CreateSurface("Front of House")
	require.Equal(t, 0, r.ExitCode, "CreateSurface: stderr=%s", r.Stderr)
	blackout := ControlRefInput{Kind: "safety", Safety: "blackout"}
	r = surfaceSvc.AssignItem("Front of House", blackout)
	require.Equal(t, 0, r.ExitCode, "AssignItem: stderr=%s", r.Stderr)

	result := startLearnAndSend(t, svc, "Front of House", blackout, out, gomidi.NoteOn(1, 36, 100))
	require.Equal(t, 0, result.ExitCode, "StartLearn failed: stderr=%s", result.Stderr)

	mappings, err := svc.ListMappings("Front of House")
	require.NoError(t, err, "ListMappings")
	require.Len(t, mappings, 1, "expected exactly one persisted mapping: %+v", mappings)
	got := mappings[0]
	require.True(t, got.Channel == 1 && got.Kind == "note" && got.Number == 36, "mapping = %+v, want channel=1 kind=note number=36", got)
	require.True(t, got.Target.Kind == "safety" && got.Target.Safety == "blackout", "mapping target = %+v, want the blackout safety control", got.Target)
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
	require.Equal(t, 0, first.ExitCode, "first StartLearn failed: stderr=%s", first.Stderr)

	// Same surface, same (channel, kind, number), a different target
	// control -- rejected outright, the existing mapping left untouched
	// (D-06).
	second := startLearnAndSend(t, svc, "Front of House", grand, out, collidingMsg)
	require.NotEqual(t, 0, second.ExitCode, "expected GOLC_MIDI_MAPPING_CONFLICT")
	require.Contains(t, second.Stderr, "GOLC_MIDI_MAPPING_CONFLICT")
	// 06-UI-SPEC.md's exact mapping-conflict copy embeds the conflicting
	// control's own label ("Blackout"), not the newly-attempted target's.
	require.Contains(t, second.Stderr, `already mapped to "Blackout"`, "expected the UI-SPEC mapping-conflict copy naming the existing control")
	mappings, err := svc.ListMappings("Front of House")
	require.NoError(t, err, "ListMappings")
	require.True(t, len(mappings) == 1 && mappings[0].Target.Safety == "blackout", "expected the prior mapping to remain untouched, got %+v", mappings)

	// A different surface's mapping set is independent -- the identical
	// tuple is free there (D-07).
	third := startLearnAndSend(t, svc, "Backstage", blackout, out, collidingMsg)
	require.Equal(t, 0, third.ExitCode, "expected the identical tuple to be learnable on a different surface, got stderr=%s", third.Stderr)
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
	require.NotEqual(t, 0, result.ExitCode, "expected GOLC_OPERATORSURFACE_LOCKED for an unassigned control")
	require.Contains(t, result.Stderr, "GOLC_OPERATORSURFACE_LOCKED")

	svc.mu.Lock()
	stillIdle := svc.learning == nil
	svc.mu.Unlock()
	require.True(t, stillIdle, "expected StartLearn to reject before ever opening a capture window")
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
	require.NoError(t, err, "show.Load")
	surface, found := surfaceByName(state.OperatorSurfaces, "Front of House")
	require.True(t, found, "surface not found")
	grandRef, err := resolveSurfaceControlRef(state, grand)
	require.NoError(t, err, "resolveSurfaceControlRef")
	surface, err = operatorsurface.AddMidiMapping(surface, operatorsurface.MidiMapping{
		Channel: 3, Kind: operatorsurface.ControlChange, Number: 74, Target: grandRef,
	})
	require.NoError(t, err, "AddMidiMapping")
	state.OperatorSurfaces = replaceSurfaceByID(state.OperatorSurfaces, surface)
	err = show.Save(root, showPath, state)
	require.NoError(t, err, "show.Save")

	mappings, err := svc.ListMappings("Front of House")
	require.NoError(t, err, "ListMappings")
	require.Len(t, mappings, 1, "expected exactly one mapping: %+v", mappings)
	got := mappings[0]
	require.True(t, got.Target.Kind == "master" && got.Target.MasterKind == "grand", "Target = %+v, want kind=master masterKind=grand", got.Target)
	require.Equal(t, "Grand Master", got.Label)
	require.True(t, got.Channel == 3 && got.Kind == "control_change" && got.Number == 74, "mapping = %+v, want channel=3 kind=control_change number=74", got)
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
	require.NoError(t, err, "show.Load")
	surface, found := surfaceByName(state.OperatorSurfaces, "Front of House")
	require.True(t, found, "surface not found")
	grandRef, err := resolveSurfaceControlRef(state, grand)
	require.NoError(t, err, "resolveSurfaceControlRef(grand)")
	blackoutRef, err := resolveSurfaceControlRef(state, blackout)
	require.NoError(t, err, "resolveSurfaceControlRef(blackout)")
	surface, err = operatorsurface.AddMidiMapping(surface, operatorsurface.MidiMapping{
		Channel: 1, Kind: operatorsurface.ControlChange, Number: 7, Target: grandRef,
	})
	require.NoError(t, err, "AddMidiMapping (fader)")
	surface, err = operatorsurface.AddMidiMapping(surface, operatorsurface.MidiMapping{
		Channel: 1, Kind: operatorsurface.Note, Number: 40, Target: blackoutRef,
	})
	require.NoError(t, err, "AddMidiMapping (button)")
	state.OperatorSurfaces = replaceSurfaceByID(state.OperatorSurfaces, surface)
	err = show.Save(root, showPath, state)
	require.NoError(t, err, "show.Save")

	r := svc.SetActiveSurface("Front of House")
	require.Equal(t, 0, r.ExitCode, "SetActiveSurface: stderr=%s", r.Stderr)

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
	err = out.Send(gomidi.ControlChange(1, 7, 20).Bytes())
	require.NoError(t, err, "Send")
	waitForCondition(t, func() bool { _, ok := latestFader(); return ok })

	before, ok := latestFader()
	require.True(t, ok, "expected at least one fader feedback push before crossing")
	require.False(t, before.Armed, "expected not armed before crossing, got %+v", before)
	require.Equal(t, defaultTakeoverAppValue, before.AppValue, "expected the ghost/target marker to remain at the seeded AppValue before crossing, got %+v", before)
	wantPhysical := float64(20) / 127
	require.Equal(t, wantPhysical, before.Physical, "expected live physical position before crossing, got %+v", before)

	// Cross the ghost/target marker: must now control (armed), tracking
	// the physical value.
	err = out.Send(gomidi.ControlChange(1, 7, 100).Bytes())
	require.NoError(t, err, "Send")
	wantPhysicalAfter := float64(100) / 127
	waitForCondition(t, func() bool {
		fb, ok := latestFader()
		return ok && fb.Armed && fb.Physical == wantPhysicalAfter
	})
	after, _ := latestFader()
	require.True(t, after.Armed, "expected armed after crossing, got %+v", after)
	require.Equal(t, after.Physical, after.AppValue, "expected the controlling AppValue to track the physical value once armed, got %+v", after)

	// A mapped button (Note) acts immediately -- Armed=true with no
	// crossing/arming delay, independent of the fader's own state above
	// (D-12).
	err = out.Send(gomidi.NoteOn(1, 40, 127).Bytes())
	require.NoError(t, err, "Send")
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
	require.True(t, buttonFeedback.Armed, "expected a Note mapping to report Armed=true immediately (D-12), got %+v", buttonFeedback)
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
	require.Fail(t, "timed out waiting for expected MIDI feedback")
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
	require.Fail(t, fmt.Sprintf("timed out waiting for at least %d dispatched requests, got %d", want, capture.count()))
}

// loadShowWithRetry re-reads the ShowState at root/showPath, retrying a
// transient "database is locked" (SQLITE_BUSY) diagnostic before failing.
// internal/show/schema.go's openStore applies "PRAGMA busy_timeout = 5000"
// as the first statement on every connection, so SQLite itself already
// waits out most transient cross-process contention internally; this loop
// is a belt-and-suspenders guard against the residual case that PRAGMA
// doesn't reach -- Windows' file-locking semantics can leave a just-closed
// handle's lock briefly outstanding even after SQLite's own busy handler
// gives up -- applied here so a test's own post-wait assertion read isn't
// itself a source of flakiness distinct from the dispatch behavior under
// test.
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
		require.Contains(t, err.Error(), "database is locked", "show.Load: %v", err)
		time.Sleep(5 * time.Millisecond)
	}
	require.Fail(t, fmt.Sprintf("show.Load: repeated database-is-locked retries exhausted: %v", lastErr))
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
	require.Fail(t, fmt.Sprintf("timed out waiting for scene %q to become active", sceneName))
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
	require.Fail(t, fmt.Sprintf("timed out waiting for %s/%s Enabled=%v", sceneName, kind, want))
}

// assertMasterSetForward asserts got is an "artnet master set" Request
// carrying --grand wantLevel and --source manual.
func assertMasterSetForward(t *testing.T, got ipc.Request, wantLevel float64) {
	t.Helper()
	require.Equal(t, "artnet master set", got.Route, "forwarded route")
	idx := -1
	for i, a := range got.Args {
		if a == "--grand" {
			idx = i
			break
		}
	}
	require.True(t, idx != -1 && idx+1 < len(got.Args), "expected --grand in forwarded args, got %v", got.Args)
	gotLevel, err := strconv.ParseFloat(got.Args[idx+1], 64)
	require.NoError(t, err, "--grand value %q is not a valid number", got.Args[idx+1])
	require.LessOrEqual(t, math.Abs(gotLevel-wantLevel), 1e-6, "forwarded --grand level = %v, want %v", gotLevel, wantLevel)
	found := false
	for i := 0; i < len(got.Args)-1; i++ {
		if got.Args[i] == "--source" && got.Args[i+1] == "manual" {
			found = true
		}
	}
	require.True(t, found, "expected --source manual in forwarded args, got %v", got.Args)
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
	r := surfaceSvc.CreateSurface("Front of House")
	require.Equal(t, 0, r.ExitCode, "CreateSurface: stderr=%s", r.Stderr)
	chorus := ControlRefInput{Kind: "scene", Scene: "Chorus"}
	r = surfaceSvc.AssignItem("Front of House", chorus)
	require.Equal(t, 0, r.ExitCode, "AssignItem: stderr=%s", r.Stderr)

	state, err := show.Load(root, showPath)
	require.NoError(t, err, "show.Load")
	surface, found := surfaceByName(state.OperatorSurfaces, "Front of House")
	require.True(t, found, "surface not found")
	chorusRef, err := resolveSurfaceControlRef(state, chorus)
	require.NoError(t, err, "resolveSurfaceControlRef")
	surface, err = operatorsurface.AddMidiMapping(surface, operatorsurface.MidiMapping{
		Channel: 1, Kind: operatorsurface.Note, Number: 40, Target: chorusRef,
	})
	require.NoError(t, err, "AddMidiMapping")
	state.OperatorSurfaces = replaceSurfaceByID(state.OperatorSurfaces, surface)
	err = show.Save(root, showPath, state)
	require.NoError(t, err, "show.Save")

	r = svc.SetActiveSurface("Front of House")
	require.Equal(t, 0, r.ExitCode, "SetActiveSurface: stderr=%s", r.Stderr)

	err = out.Send(gomidi.NoteOn(1, 40, 100).Bytes())
	require.NoError(t, err, "Send")
	waitForSceneActive(t, root, showPath, "Chorus")

	// A following Note-off must not error, re-switch, or revert.
	err = out.Send(gomidi.NoteOff(1, 40).Bytes())
	require.NoError(t, err, "Send (note-off)")
	time.Sleep(50 * time.Millisecond)
	after := loadShowWithRetry(t, root, showPath)
	for _, sc := range after.Scenes {
		if sc.Name == "Chorus" {
			require.True(t, sc.Active, "expected Chorus to remain active after a Note-off, got Active=%v", sc.Active)
		}
		if sc.Name == "Bridge" {
			require.False(t, sc.Active, "expected Bridge to remain inactive after a Note-off on the Chorus mapping")
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
	require.NoError(t, err, "show.Load (seed)")
	require.Len(t, seeded.Themes, 1, "expected exactly one seeded theme")
	themeID := seeded.Themes[0].ID
	execRegistry(t, root, "scene", "layer", "set", "Verse", "--kind", "color_theme", "--ref", themeID.String(), "--show", showPath)

	surfaceSvc := NewSurfaceService("", root, showPath)
	r := surfaceSvc.CreateSurface("Front of House")
	require.Equal(t, 0, r.ExitCode, "CreateSurface: stderr=%s", r.Stderr)
	layerInput := ControlRefInput{Kind: "layer", Scene: "Verse", LayerKind: "color_theme"}
	r = surfaceSvc.AssignItem("Front of House", layerInput)
	require.Equal(t, 0, r.ExitCode, "AssignItem: stderr=%s", r.Stderr)

	state, err := show.Load(root, showPath)
	require.NoError(t, err, "show.Load")
	surface, found := surfaceByName(state.OperatorSurfaces, "Front of House")
	require.True(t, found, "surface not found")
	layerRef, err := resolveSurfaceControlRef(state, layerInput)
	require.NoError(t, err, "resolveSurfaceControlRef")
	surface, err = operatorsurface.AddMidiMapping(surface, operatorsurface.MidiMapping{
		Channel: 1, Kind: operatorsurface.Note, Number: 41, Target: layerRef,
	})
	require.NoError(t, err, "AddMidiMapping")
	state.OperatorSurfaces = replaceSurfaceByID(state.OperatorSurfaces, surface)
	err = show.Save(root, showPath, state)
	require.NoError(t, err, "show.Save")

	r = svc.SetActiveSurface("Front of House")
	require.Equal(t, 0, r.ExitCode, "SetActiveSurface: stderr=%s", r.Stderr)

	beforeLayer := findLayer(t, state, "Verse", "color_theme")
	require.True(t, beforeLayer.Enabled, "expected the seeded layer to start enabled")
	require.Equal(t, themeID, beforeLayer.Ref, "expected the seeded layer Ref")

	err = out.Send(gomidi.NoteOn(1, 41, 100).Bytes())
	require.NoError(t, err, "Send")
	waitForLayerEnabled(t, root, showPath, "Verse", "color_theme", false)

	after := loadShowWithRetry(t, root, showPath)
	toggled := findLayer(t, after, "Verse", "color_theme")
	require.Equal(t, themeID, toggled.Ref, "expected Ref to be preserved across the MIDI-driven toggle")
}

// TestMidiServiceDispatchMasterCcForwardsOnlyAfterCrossing proves Gap B[1]'s
// master half: a CC mapping whose Target is the grand master forwards
// exactly one "artnet master set" daemon Request once the fader crosses the
// seeded ghost/target marker (cross-to-catch, D-11), and forwards nothing
// for pre-arm messages that have not yet crossed.
func TestMidiServiceDispatchMasterCcForwardsOnlyAfterCrossing(t *testing.T) {
	svc, root, showPath, out := newMidiTestFixture(t, "test-dispatch-master-cc")

	surfaceSvc := NewSurfaceService("", root, showPath)
	r := surfaceSvc.CreateSurface("Front of House")
	require.Equal(t, 0, r.ExitCode, "CreateSurface: stderr=%s", r.Stderr)
	grand := ControlRefInput{Kind: "master", MasterKind: "grand"}
	r = surfaceSvc.AssignItem("Front of House", grand)
	require.Equal(t, 0, r.ExitCode, "AssignItem: stderr=%s", r.Stderr)

	state, err := show.Load(root, showPath)
	require.NoError(t, err, "show.Load")
	surface, found := surfaceByName(state.OperatorSurfaces, "Front of House")
	require.True(t, found, "surface not found")
	grandRef, err := resolveSurfaceControlRef(state, grand)
	require.NoError(t, err, "resolveSurfaceControlRef")
	surface, err = operatorsurface.AddMidiMapping(surface, operatorsurface.MidiMapping{
		Channel: 1, Kind: operatorsurface.ControlChange, Number: 7, Target: grandRef,
	})
	require.NoError(t, err, "AddMidiMapping")
	state.OperatorSurfaces = replaceSurfaceByID(state.OperatorSurfaces, surface)
	err = show.Save(root, showPath, state)
	require.NoError(t, err, "show.Save")

	capture := &dispatchCapture{}
	svc.dial = capture.dial

	r = svc.SetActiveSurface("Front of House")
	require.Equal(t, 0, r.ExitCode, "SetActiveSurface: stderr=%s", r.Stderr)

	// Below the 0.5 default ghost/target: not yet crossed, must forward
	// nothing.
	err = out.Send(gomidi.ControlChange(1, 7, 20).Bytes())
	require.NoError(t, err, "Send")
	time.Sleep(50 * time.Millisecond)
	require.Equal(t, 0, capture.count(), "expected zero forwards before crossing: %+v", capture.all())

	// Cross the marker: exactly one forward with the crossed value.
	err = out.Send(gomidi.ControlChange(1, 7, 100).Bytes())
	require.NoError(t, err, "Send")
	waitForDispatchCount(t, capture, 1)
	requests := capture.all()
	require.Len(t, requests, 1, "expected exactly one forward after crossing: %+v", requests)
	assertMasterSetForward(t, requests[0], float64(100)/127)
}

// TestMidiServiceDispatchSafetyNoteForwardsDaemonRoute proves Gap B[1]'s
// safety half: a Note mapping whose Target is a safety control forwards the
// matching "artnet safety ..." daemon route with "--source manual" when
// pressed, and a following Note-off does not re-forward.
func TestMidiServiceDispatchSafetyNoteForwardsDaemonRoute(t *testing.T) {
	svc, root, showPath, out := newMidiTestFixture(t, "test-dispatch-safety-note")

	surfaceSvc := NewSurfaceService("", root, showPath)
	r := surfaceSvc.CreateSurface("Front of House")
	require.Equal(t, 0, r.ExitCode, "CreateSurface: stderr=%s", r.Stderr)
	blackout := ControlRefInput{Kind: "safety", Safety: "blackout"}
	r = surfaceSvc.AssignItem("Front of House", blackout)
	require.Equal(t, 0, r.ExitCode, "AssignItem: stderr=%s", r.Stderr)

	state, err := show.Load(root, showPath)
	require.NoError(t, err, "show.Load")
	surface, found := surfaceByName(state.OperatorSurfaces, "Front of House")
	require.True(t, found, "surface not found")
	blackoutRef, err := resolveSurfaceControlRef(state, blackout)
	require.NoError(t, err, "resolveSurfaceControlRef")
	surface, err = operatorsurface.AddMidiMapping(surface, operatorsurface.MidiMapping{
		Channel: 1, Kind: operatorsurface.Note, Number: 50, Target: blackoutRef,
	})
	require.NoError(t, err, "AddMidiMapping")
	state.OperatorSurfaces = replaceSurfaceByID(state.OperatorSurfaces, surface)
	err = show.Save(root, showPath, state)
	require.NoError(t, err, "show.Save")

	capture := &dispatchCapture{}
	svc.dial = capture.dial

	r = svc.SetActiveSurface("Front of House")
	require.Equal(t, 0, r.ExitCode, "SetActiveSurface: stderr=%s", r.Stderr)

	err = out.Send(gomidi.NoteOn(1, 50, 100).Bytes())
	require.NoError(t, err, "Send")
	waitForDispatchCount(t, capture, 1)
	requests := capture.all()
	require.Len(t, requests, 1, "expected exactly one forward: %+v", requests)
	assertSafetyForward(t, requests[0], "artnet safety blackout", []string{"--on", "true", "--source", "manual"})

	err = out.Send(gomidi.NoteOff(1, 50).Bytes())
	require.NoError(t, err, "Send (note-off)")
	time.Sleep(50 * time.Millisecond)
	require.Equal(t, 1, capture.count(), "expected no additional forward on Note-off: %+v", capture.all())
}

// TestMidiServiceDispatchUnmappedEventDoesNothing proves Gap B[1]'s
// unchanged-behavior guarantee: a message matching no mapping on the active
// surface dispatches nothing and changes no state.
func TestMidiServiceDispatchUnmappedEventDoesNothing(t *testing.T) {
	svc, root, showPath, out := newMidiTestFixture(t, "test-dispatch-unmapped")

	execRegistry(t, root, "scene", "create", "Verse", "--bars", "4", "--show", showPath)

	surfaceSvc := NewSurfaceService("", root, showPath)
	r := surfaceSvc.CreateSurface("Front of House")
	require.Equal(t, 0, r.ExitCode, "CreateSurface: stderr=%s", r.Stderr)

	capture := &dispatchCapture{}
	svc.dial = capture.dial

	r = svc.SetActiveSurface("Front of House")
	require.Equal(t, 0, r.ExitCode, "SetActiveSurface: stderr=%s", r.Stderr)

	before := loadShowWithRetry(t, root, showPath)

	err := out.Send(gomidi.NoteOn(1, 99, 100).Bytes())
	require.NoError(t, err, "Send")
	time.Sleep(50 * time.Millisecond)

	after := loadShowWithRetry(t, root, showPath)
	require.Equal(t, len(before.Scenes), len(after.Scenes), "expected no scene-count change")
	for _, sc := range after.Scenes {
		require.False(t, sc.Active, "expected no scene to become active from an unmapped event, got %+v", sc)
	}
	require.Equal(t, 0, capture.count(), "expected zero daemon forwards from an unmapped event: %+v", capture.all())
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
	r := surfaceSvc.CreateSurface("Front of House")
	require.Equal(t, 0, r.ExitCode, "CreateSurface: stderr=%s", r.Stderr)
	beta := ControlRefInput{Kind: "scene", Scene: "Beta"}
	gamma := ControlRefInput{Kind: "scene", Scene: "Gamma"}
	r = surfaceSvc.AssignItem("Front of House", beta)
	require.Equal(t, 0, r.ExitCode, "AssignItem(beta): stderr=%s", r.Stderr)
	r = surfaceSvc.AssignItem("Front of House", gamma)
	require.Equal(t, 0, r.ExitCode, "AssignItem(gamma): stderr=%s", r.Stderr)

	state, err := show.Load(root, showPath)
	require.NoError(t, err, "show.Load")
	surface, found := surfaceByName(state.OperatorSurfaces, "Front of House")
	require.True(t, found, "surface not found")
	betaRef, err := resolveSurfaceControlRef(state, beta)
	require.NoError(t, err, "resolveSurfaceControlRef(beta)")
	gammaRef, err := resolveSurfaceControlRef(state, gamma)
	require.NoError(t, err, "resolveSurfaceControlRef(gamma)")
	surface, err = operatorsurface.AddMidiMapping(surface, operatorsurface.MidiMapping{
		Channel: 1, Kind: operatorsurface.Note, Number: 10, Target: betaRef,
	})
	require.NoError(t, err, "AddMidiMapping(beta)")
	surface, err = operatorsurface.AddMidiMapping(surface, operatorsurface.MidiMapping{
		Channel: 1, Kind: operatorsurface.Note, Number: 11, Target: gammaRef,
	})
	require.NoError(t, err, "AddMidiMapping(gamma)")
	state.OperatorSurfaces = replaceSurfaceByID(state.OperatorSurfaces, surface)
	err = show.Save(root, showPath, state)
	require.NoError(t, err, "show.Save")

	r = svc.SetActiveSurface("Front of House")
	require.Equal(t, 0, r.ExitCode, "SetActiveSurface: stderr=%s", r.Stderr)

	// Repeated press without release: fires each time, never errors.
	err = out.Send(gomidi.NoteOn(1, 10, 100).Bytes())
	require.NoError(t, err, "Send")
	waitForSceneActive(t, root, showPath, "Beta")
	err = out.Send(gomidi.NoteOn(1, 10, 100).Bytes())
	require.NoError(t, err, "Send (repeat press)")
	time.Sleep(50 * time.Millisecond)
	stillBeta := loadShowWithRetry(t, root, showPath)
	for _, sc := range stillBeta.Scenes {
		if sc.Name == "Beta" {
			require.True(t, sc.Active, "expected Beta to remain active after a repeated press")
		}
	}

	// A Note-off between presses dispatches nothing.
	err = out.Send(gomidi.NoteOff(1, 10).Bytes())
	require.NoError(t, err, "Send (note-off)")
	time.Sleep(50 * time.Millisecond)

	// The dispatch loop keeps processing a subsequent mapped press.
	err = out.Send(gomidi.NoteOn(1, 11, 100).Bytes())
	require.NoError(t, err, "Send")
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
	r := surfaceSvc.CreateSurface("Front of House")
	require.Equal(t, 0, r.ExitCode, "CreateSurface: stderr=%s", r.Stderr)
	grand := ControlRefInput{Kind: "master", MasterKind: "grand"}
	beta := ControlRefInput{Kind: "scene", Scene: "Beta"}
	r = surfaceSvc.AssignItem("Front of House", grand)
	require.Equal(t, 0, r.ExitCode, "AssignItem(grand): stderr=%s", r.Stderr)
	r = surfaceSvc.AssignItem("Front of House", beta)
	require.Equal(t, 0, r.ExitCode, "AssignItem(beta): stderr=%s", r.Stderr)

	state, err := show.Load(root, showPath)
	require.NoError(t, err, "show.Load")
	surface, found := surfaceByName(state.OperatorSurfaces, "Front of House")
	require.True(t, found, "surface not found")
	grandRef, err := resolveSurfaceControlRef(state, grand)
	require.NoError(t, err, "resolveSurfaceControlRef(grand)")
	betaRef, err := resolveSurfaceControlRef(state, beta)
	require.NoError(t, err, "resolveSurfaceControlRef(beta)")
	surface, err = operatorsurface.AddMidiMapping(surface, operatorsurface.MidiMapping{
		Channel: 1, Kind: operatorsurface.ControlChange, Number: 7, Target: grandRef,
	})
	require.NoError(t, err, "AddMidiMapping(grand)")
	surface, err = operatorsurface.AddMidiMapping(surface, operatorsurface.MidiMapping{
		Channel: 1, Kind: operatorsurface.ControlChange, Number: 8, Target: betaRef,
	})
	require.NoError(t, err, "AddMidiMapping(beta)")
	state.OperatorSurfaces = replaceSurfaceByID(state.OperatorSurfaces, surface)
	err = show.Save(root, showPath, state)
	require.NoError(t, err, "show.Save")

	capture := &dispatchCapture{}
	svc.dial = capture.dial

	r = svc.SetActiveSurface("Front of House")
	require.Equal(t, 0, r.ExitCode, "SetActiveSurface: stderr=%s", r.Stderr)

	// A control's very first message can never arm (TakeoverState seeds
	// LastPhysical to NaN, so no crossing check can pass yet) -- establish
	// a below-threshold physical position first, then cross the marker, then
	// hold past it with two further updates -- each must independently
	// forward (continuous).
	err = out.Send(gomidi.ControlChange(1, 7, 20).Bytes())
	require.NoError(t, err, "Send")
	time.Sleep(50 * time.Millisecond)
	require.Equal(t, 0, capture.count(), "expected zero forwards before crossing: %+v", capture.all())
	err = out.Send(gomidi.ControlChange(1, 7, 100).Bytes())
	require.NoError(t, err, "Send")
	waitForDispatchCount(t, capture, 1)
	err = out.Send(gomidi.ControlChange(1, 7, 110).Bytes())
	require.NoError(t, err, "Send")
	waitForDispatchCount(t, capture, 2)
	err = out.Send(gomidi.ControlChange(1, 7, 120).Bytes())
	require.NoError(t, err, "Send")
	waitForDispatchCount(t, capture, 3)

	// Cross the scene CC's own ghost/target marker once (again establishing
	// a below-threshold position first, since its own TakeoverState is
	// independent -- keyed per mapping ID -- and equally cannot arm on its
	// own first message): fires the switch exactly once, then never
	// re-switches on further armed messages, and never dials through the
	// master-set path.
	err = out.Send(gomidi.ControlChange(1, 8, 20).Bytes())
	require.NoError(t, err, "Send")
	err = out.Send(gomidi.ControlChange(1, 8, 100).Bytes())
	require.NoError(t, err, "Send")
	waitForSceneActive(t, root, showPath, "Beta")
	err = out.Send(gomidi.ControlChange(1, 8, 110).Bytes())
	require.NoError(t, err, "Send")
	err = out.Send(gomidi.ControlChange(1, 8, 120).Bytes())
	require.NoError(t, err, "Send")
	time.Sleep(50 * time.Millisecond)

	require.Equal(t, 3, capture.count(), "expected exactly 3 master-set forwards (scene dispatch never dials): %+v", capture.all())
	final := loadShowWithRetry(t, root, showPath)
	for _, sc := range final.Scenes {
		if sc.Name == "Beta" {
			require.True(t, sc.Active, "expected Beta to remain active (no re-switch needed/attempted)")
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
	r := surfaceSvc.CreateSurface("Front of House")
	require.Equal(t, 0, r.ExitCode, "CreateSurface: stderr=%s", r.Stderr)
	// Ghost is deliberately never assigned to the surface's SceneRefs: the
	// MIDI mapping is added directly (via operatorsurface.AddMidiMapping,
	// mirroring this file's other direct-mapping fixtures), and show.Save
	// itself rejects a surface whose SceneRefs dangle on a deleted scene
	// (GOLC_OPERATORSURFACE_DANGLING_REFERENCE) -- this test is about a
	// mapping's Target outliving its scene, not about surface assignment
	// membership.
	ghost := ControlRefInput{Kind: "scene", Scene: "Ghost"}
	alive := ControlRefInput{Kind: "scene", Scene: "Alive"}
	r = surfaceSvc.AssignItem("Front of House", alive)
	require.Equal(t, 0, r.ExitCode, "AssignItem(alive): stderr=%s", r.Stderr)

	state, err := show.Load(root, showPath)
	require.NoError(t, err, "show.Load")
	surface, found := surfaceByName(state.OperatorSurfaces, "Front of House")
	require.True(t, found, "surface not found")
	ghostRef, err := resolveSurfaceControlRef(state, ghost)
	require.NoError(t, err, "resolveSurfaceControlRef(ghost)")
	aliveRef, err := resolveSurfaceControlRef(state, alive)
	require.NoError(t, err, "resolveSurfaceControlRef(alive)")
	surface, err = operatorsurface.AddMidiMapping(surface, operatorsurface.MidiMapping{
		Channel: 1, Kind: operatorsurface.Note, Number: 60, Target: ghostRef,
	})
	require.NoError(t, err, "AddMidiMapping(ghost)")
	surface, err = operatorsurface.AddMidiMapping(surface, operatorsurface.MidiMapping{
		Channel: 1, Kind: operatorsurface.Note, Number: 61, Target: aliveRef,
	})
	require.NoError(t, err, "AddMidiMapping(alive)")
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
	err = show.Save(root, showPath, state)
	require.NoError(t, err, "show.Save")

	r = svc.SetActiveSurface("Front of House")
	require.Equal(t, 0, r.ExitCode, "SetActiveSurface: stderr=%s", r.Stderr)

	err = out.Send(gomidi.NoteOn(1, 60, 100).Bytes())
	require.NoError(t, err, "Send")
	time.Sleep(50 * time.Millisecond)

	// The dispatch loop must keep working afterward.
	err = out.Send(gomidi.NoteOn(1, 61, 100).Bytes())
	require.NoError(t, err, "Send")
	waitForSceneActive(t, root, showPath, "Alive")
}

// seedDeskInstance builds and saves a minimal ShowState with one pool
// (one member) and one deployment with an Instance patched to that member,
// returning the Instance's ID as a string (deskmidi.Mapping's own
// InstanceID addressing) -- mirrors svc_programming_test.go's identical
// seedProgrammingInstance fixture (unexported to that file, so this file
// keeps its own copy).
func seedDeskInstance(t *testing.T, root, showPath string) string {
	t.Helper()

	p, err := pool.NewPool("Wash Pool", nil)
	require.NoError(t, err, "pool.NewPool")
	member, err := pool.NewPoolMember("acme/par64", "sha256:11111111")
	require.NoError(t, err, "pool.NewPoolMember")
	p.Members = append(p.Members, member)

	d, err := deployment.NewDeployment("Venue A")
	require.NoError(t, err, "deployment.NewDeployment")
	instanceID, err := uuid.NewV7()
	require.NoError(t, err, "uuid.NewV7")
	d.Instances = append(d.Instances, deployment.Instance{
		ID: instanceID, PoolID: p.ID, PoolMemberID: member.ID, Mode: "Standard", Universe: 1, Address: 1,
	})

	state := show.State{Pools: []pool.Pool{p}, Deployments: []deployment.Deployment{d}}
	require.NoError(t, show.Save(root, showPath, state), "show.Save (seed)")
	return instanceID.String()
}

// startDeskLearnAndSend runs StartDeskLearn(instanceID, capability) in the
// background, waits for its capture window to open, sends msg through out,
// and returns StartDeskLearn's result (or fails the test on timeout) --
// mirrors startLearnAndSend's identical shape for the surface system.
func startDeskLearnAndSend(t *testing.T, svc *MidiService, instanceID, capability string, out testdrvOut, msg gomidi.Message) Result {
	t.Helper()
	resultCh := make(chan Result, 1)
	go func() {
		resultCh <- svc.StartDeskLearn(instanceID, capability)
	}()
	waitForLearningActive(t, svc)

	err := out.Send(msg.Bytes())
	require.NoError(t, err, "Send")

	select {
	case result := <-resultCh:
		return result
	case <-time.After(3 * time.Second):
		require.Fail(t, "timed out waiting for StartDeskLearn to return")
		return Result{}
	}
}

// assertDeskSetForward asserts got is an "artnet desk set" Request carrying
// --instance wantInstance, --attr wantCapability=wantValue, and --source
// manual -- mirrors assertMasterSetForward's identical shape.
func assertDeskSetForward(t *testing.T, got ipc.Request, wantInstance, wantCapability string, wantValue float64) {
	t.Helper()
	require.Equal(t, "artnet desk set", got.Route, "forwarded route")

	idx := -1
	for i, a := range got.Args {
		if a == "--instance" {
			idx = i
			break
		}
	}
	require.True(t, idx != -1 && idx+1 < len(got.Args), "expected --instance in forwarded args, got %v", got.Args)
	require.Equal(t, wantInstance, got.Args[idx+1], "forwarded --instance")

	idx = -1
	for i, a := range got.Args {
		if a == "--attr" {
			idx = i
			break
		}
	}
	require.True(t, idx != -1 && idx+1 < len(got.Args), "expected --attr in forwarded args, got %v", got.Args)
	attr := got.Args[idx+1]
	prefix := wantCapability + "="
	require.True(t, strings.HasPrefix(attr, prefix), "forwarded --attr = %q, want prefix %q", attr, prefix)
	gotValue, err := strconv.ParseFloat(strings.TrimPrefix(attr, prefix), 64)
	require.NoError(t, err, "--attr value %q is not a valid number", attr)
	require.LessOrEqual(t, math.Abs(gotValue-wantValue), 1e-6, "forwarded --attr value = %v, want %v", gotValue, wantValue)

	found := false
	for i := 0; i < len(got.Args)-1; i++ {
		if got.Args[i] == "--source" && got.Args[i+1] == "manual" {
			found = true
		}
	}
	require.True(t, found, "expected --source manual in forwarded args, got %v", got.Args)
}

// TestMidiServiceStartDeskLearnPersistsMapping proves a full desk-learn
// round-trip: StartDeskLearn blocks until a matching MIDI message arrives,
// then persists the mapping (deskmidi.AddMapping -> show.Save) reflected by
// ListDeskMappings -- entirely without any Operator Surface involved,
// unlike TestMidiServiceStartLearnPersistsMapping's surface-scoped
// counterpart.
func TestMidiServiceStartDeskLearnPersistsMapping(t *testing.T) {
	svc, root, showPath, out := newMidiTestFixture(t, "test-desk-learn-persists")
	instanceID := seedDeskInstance(t, root, showPath)

	result := startDeskLearnAndSend(t, svc, instanceID, "intensity", out, gomidi.ControlChange(2, 74, 100))
	require.Equal(t, 0, result.ExitCode, "StartDeskLearn: stderr=%s", result.Stderr)

	views, err := svc.ListDeskMappings()
	require.NoError(t, err, "ListDeskMappings")
	require.Len(t, views, 1, "expected exactly one desk mapping")
	require.Equal(t, 2, views[0].Channel)
	require.Equal(t, "control_change", views[0].Kind)
	require.Equal(t, 74, views[0].Number)
	require.Equal(t, instanceID, views[0].InstanceID)
	require.Equal(t, "intensity", views[0].Capability)

	state, err := show.Load(root, showPath)
	require.NoError(t, err, "show.Load")
	require.Len(t, state.DeskMidiMappings, 1, "expected the mapping to be persisted in show.State")
}

// TestMidiServiceStartDeskLearnRejectsConflict proves D-06's desk-mapping
// counterpart: a candidate colliding with an already-mapped (channel, kind,
// number) tuple is rejected outright, leaving the prior mapping untouched.
func TestMidiServiceStartDeskLearnRejectsConflict(t *testing.T) {
	svc, root, showPath, out := newMidiTestFixture(t, "test-desk-learn-conflict")
	instanceID := seedDeskInstance(t, root, showPath)

	first := startDeskLearnAndSend(t, svc, instanceID, "intensity", out, gomidi.ControlChange(1, 20, 64))
	require.Equal(t, 0, first.ExitCode, "first StartDeskLearn: stderr=%s", first.Stderr)

	second := startDeskLearnAndSend(t, svc, instanceID, "pan", out, gomidi.ControlChange(1, 20, 64))
	require.NotEqual(t, 0, second.ExitCode, "expected a colliding (channel, kind, number) tuple to be rejected")
	require.Contains(t, second.Stderr, "GOLC_DESKMIDI_MAPPING_CONFLICT")

	views, err := svc.ListDeskMappings()
	require.NoError(t, err, "ListDeskMappings")
	require.Len(t, views, 1, "expected the prior mapping to be left untouched")
	require.Equal(t, "intensity", views[0].Capability)
}

// TestMidiServiceDispatchDeskFaderForwardsOnlyAfterCrossing proves a desk
// mapping's ControlChange takeover behaves identically to a surface master
// mapping's own cross-to-catch contract (TestMidiServiceDispatchMasterCcForwardsOnlyAfterCrossing),
// but dispatched to "artnet desk set" instead of "artnet master set", and
// entirely without any Operator Surface. This test seeds DeskMidiMappings
// directly (mirroring the master-cc test's own direct
// operatorsurface.AddMidiMapping+show.Save seeding) rather than going
// through StartDeskLearn, so it must call svc.refreshDeskMappings itself
// afterward -- the production-only equivalent of SetActiveSurface's own
// disk-refresh role, since StartDeskLearn/RemoveDeskMapping are the only
// production callers that keep the live dispatch cache in sync with disk.
func TestMidiServiceDispatchDeskFaderForwardsOnlyAfterCrossing(t *testing.T) {
	svc, root, showPath, out := newMidiTestFixture(t, "test-dispatch-desk-cc")
	instanceID := seedDeskInstance(t, root, showPath)

	state, err := show.Load(root, showPath)
	require.NoError(t, err, "show.Load")
	updated, err := deskmidi.AddMapping(state.DeskMidiMappings, deskmidi.Mapping{
		Channel: 1, Kind: deskmidi.ControlChange, Number: 7, InstanceID: instanceID, Capability: "intensity",
	})
	require.NoError(t, err, "deskmidi.AddMapping")
	state.DeskMidiMappings = updated
	require.NoError(t, show.Save(root, showPath, state), "show.Save")
	svc.refreshDeskMappings(state)

	capture := &dispatchCapture{}
	svc.dial = capture.dial

	// Below the 0.5 default ghost/target: not yet crossed, must forward
	// nothing.
	err = out.Send(gomidi.ControlChange(1, 7, 20).Bytes())
	require.NoError(t, err, "Send")
	time.Sleep(50 * time.Millisecond)
	require.Equal(t, 0, capture.count(), "expected zero forwards before crossing: %+v", capture.all())

	// Cross the marker: exactly one forward with the crossed value.
	err = out.Send(gomidi.ControlChange(1, 7, 100).Bytes())
	require.NoError(t, err, "Send")
	waitForDispatchCount(t, capture, 1)
	requests := capture.all()
	require.Len(t, requests, 1, "expected exactly one forward after crossing: %+v", requests)
	assertDeskSetForward(t, requests[0], instanceID, "intensity", float64(100)/127)

	// Continues forwarding on every subsequent armed message (continuous,
	// same as a surface master mapping).
	err = out.Send(gomidi.ControlChange(1, 7, 110).Bytes())
	require.NoError(t, err, "Send")
	waitForDispatchCount(t, capture, 2)
}

// TestMidiServiceDispatchDeskFaderNoteTogglesFullAndZero proves a Note-kind
// desk mapping dispatches the full level on press and zero on release, with
// no arming delay (D-12), mirroring dispatchMasterSet's own Note handling
// for a surface master mapping.
func TestMidiServiceDispatchDeskFaderNoteTogglesFullAndZero(t *testing.T) {
	svc, root, showPath, out := newMidiTestFixture(t, "test-dispatch-desk-note")
	instanceID := seedDeskInstance(t, root, showPath)

	state, err := show.Load(root, showPath)
	require.NoError(t, err, "show.Load")
	updated, err := deskmidi.AddMapping(state.DeskMidiMappings, deskmidi.Mapping{
		Channel: 1, Kind: deskmidi.Note, Number: 40, InstanceID: instanceID, Capability: "strobe",
	})
	require.NoError(t, err, "deskmidi.AddMapping")
	state.DeskMidiMappings = updated
	require.NoError(t, show.Save(root, showPath, state), "show.Save")
	svc.refreshDeskMappings(state)

	capture := &dispatchCapture{}
	svc.dial = capture.dial

	err = out.Send(gomidi.NoteOn(1, 40, 100).Bytes())
	require.NoError(t, err, "Send (note-on)")
	waitForDispatchCount(t, capture, 1)
	assertDeskSetForward(t, capture.all()[0], instanceID, "strobe", 1)

	err = out.Send(gomidi.NoteOff(1, 40).Bytes())
	require.NoError(t, err, "Send (note-off)")
	waitForDispatchCount(t, capture, 2)
	assertDeskSetForward(t, capture.all()[1], instanceID, "strobe", 0)
}

// TestMidiServiceRemoveDeskMappingIsIdempotent proves RemoveDeskMapping
// deletes an existing mapping and is a no-op (not an error) on a repeat
// call or an unknown ID, mirroring RemoveMapping's identical contract.
func TestMidiServiceRemoveDeskMappingIsIdempotent(t *testing.T) {
	svc, root, showPath, out := newMidiTestFixture(t, "test-desk-remove-idempotent")
	instanceID := seedDeskInstance(t, root, showPath)

	result := startDeskLearnAndSend(t, svc, instanceID, "intensity", out, gomidi.ControlChange(1, 30, 64))
	require.Equal(t, 0, result.ExitCode, "StartDeskLearn: stderr=%s", result.Stderr)

	views, err := svc.ListDeskMappings()
	require.NoError(t, err, "ListDeskMappings")
	require.Len(t, views, 1)
	id := views[0].ID

	r := svc.RemoveDeskMapping(id)
	require.Equal(t, 0, r.ExitCode, "RemoveDeskMapping: stderr=%s", r.Stderr)
	views, err = svc.ListDeskMappings()
	require.NoError(t, err, "ListDeskMappings")
	require.Empty(t, views)

	// Removing again (already gone) and removing an unrelated unknown ID are
	// both idempotent no-ops, never an error.
	r = svc.RemoveDeskMapping(id)
	require.Equal(t, 0, r.ExitCode, "RemoveDeskMapping (repeat): stderr=%s", r.Stderr)
	r = svc.RemoveDeskMapping(uuid.Nil.String())
	require.Equal(t, 0, r.ExitCode, "RemoveDeskMapping (unknown): stderr=%s", r.Stderr)
}
