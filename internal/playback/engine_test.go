// engine_test.go proves the real-time engine's next-bar adoption and
// lock-free publish contract (03-07-PLAN.md Task 2, CONTEXT SCEN-06/
// SCEN-09, D-05/D-06/D-07/D-08): a staged edit/switch is adopted only at a
// bar-boundary crossing, never mid-bar; a rejected StageEdit leaves
// activePlan/pendingPlan completely untouched and the engine keeps running
// the last valid plan; a coalesced/delayed tick that jumps straight to a
// final "now" produces the exact same result a sequence of on-time ticks
// reaching that same "now" would have produced (SCEN-09); StageEdit
// against an object live in the currently active scene requires no
// preceding lock/pause/detach call (D-08 -- the engine exposes no such
// API at all); and CurrentFrame is safe to call concurrently with the
// tick loop without blocking (verified under -race). It also proves
// CR-01/SCEN-08/D-11's BPM-change preserve-or-restart contract is
// observable through the real tick loop, not merely at the
// RecomputeEpoch/clock_test.go primitive level: a staged Tempo.BPM change,
// once adopted at a bar-boundary crossing, either preserves the running
// bar/beat position (PreserveMusicalPositionOnBPMChange=true) or restarts
// at bar 0 (=false), per the adopted plan's own PreserveOnBPMChange flag.
//
// This file is an internal (package playback, not playback_test)
// white-box test: it reads/overrides Engine's unexported loopStart/lastBar
// fields and calls the unexported tick method directly so every case is
// deterministic and driven by synthetic timestamps, never real wall-clock
// sleeps (except the Start/Stop lifecycle smoke test, which only checks
// clean shutdown, never an exact tick count).
package playback

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/deployment"
	"github.com/lnorton89/golc/internal/fixture"
	"github.com/lnorton89/golc/internal/pool"
	"github.com/lnorton89/golc/internal/programming"
	"github.com/lnorton89/golc/internal/scene"
	"github.com/lnorton89/golc/internal/show"
)

// fixedEngineLoopStart is the synthetic epoch every engine_test.go case
// pins Engine.loopStart to (overriding NewEngine's own time.Now()
// capture), so every tick() call is driven by a caller-chosen offset from
// a fixed, reproducible origin rather than real wall-clock timing.
var fixedEngineLoopStart = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// pinLoopStart overrides e's loopStart to fixedEngineLoopStart and resets
// lastBar to the "no tick has run yet" sentinel, so every subsequent
// tick() call in a test is measured from a known, fixed origin.
func pinLoopStart(e *Engine) {
	e.loopStart = fixedEngineLoopStart
	e.lastBar = -1
}

// newEngineTestState builds a minimal two-scene show.State: sceneA
// (active, base-look intensity 0.2) and sceneB (inactive, base-look
// intensity 0.9), both 2 bars per loop at BPM 120 (secondsPerBar = 2s),
// sharing one pool/deployment/instance.
func newEngineTestState(t *testing.T) (state show.State, instanceID uuid.UUID, sceneBName string) {
	t.Helper()

	member := pool.PoolMember{ID: uuid.New(), FixtureStableKey: "m1", FixtureContentHash: "hash1"}
	rig := pool.Pool{ID: uuid.New(), Name: "Rig", Members: []pool.PoolMember{member}}
	instance := deployment.Instance{ID: uuid.New(), PoolID: rig.ID, PoolMemberID: member.ID, Universe: 1, Address: 1}
	dep := deployment.Deployment{ID: uuid.New(), Name: "Dep", Active: true, Instances: []deployment.Instance{instance}}
	sel := programming.Selection{PoolIDs: []uuid.UUID{rig.ID}}

	presetA, err := programming.NewPreset("A", programming.PresetIntensity)
	if err != nil {
		t.Fatalf("NewPreset(A): %v", err)
	}
	presetA.Attributes = []programming.PresetAttribute{{InstanceID: instance.ID, Capability: fixture.CapabilityIntensity, Value: 0.2}}

	presetB, err := programming.NewPreset("B", programming.PresetIntensity)
	if err != nil {
		t.Fatalf("NewPreset(B): %v", err)
	}
	presetB.Attributes = []programming.PresetAttribute{{InstanceID: instance.ID, Capability: fixture.CapabilityIntensity, Value: 0.9}}

	sceneA, err := scene.NewScene("SceneA", 2)
	if err != nil {
		t.Fatalf("NewScene(A): %v", err)
	}
	sceneA.Active = true
	sceneA, err = scene.SetLayer(sceneA, scene.Layer{Kind: scene.BaseLook, Enabled: true, Selection: sel, Ref: presetA.ID})
	if err != nil {
		t.Fatalf("SetLayer(A): %v", err)
	}

	sceneB, err := scene.NewScene("SceneB", 2)
	if err != nil {
		t.Fatalf("NewScene(B): %v", err)
	}
	sceneB, err = scene.SetLayer(sceneB, scene.Layer{Kind: scene.BaseLook, Enabled: true, Selection: sel, Ref: presetB.ID})
	if err != nil {
		t.Fatalf("SetLayer(B): %v", err)
	}

	state = show.State{
		Pools:       []pool.Pool{rig},
		Deployments: []deployment.Deployment{dep},
		Presets:     []programming.Preset{presetA, presetB},
		Scenes:      []scene.Scene{sceneA, sceneB},
		Tempo:       show.Tempo{BPM: 120},
	}
	return state, instance.ID, sceneB.Name
}

func TestImmediateSwitch(t *testing.T) {
	state, instanceID, sceneBName := newEngineTestState(t)

	e, err := NewEngine(state)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	pinLoopStart(e)
	e.tick(fixedEngineLoopStart) // establish a clean bar-0 baseline

	if got := e.CurrentFrame().Values[instanceID].Values[fixture.CapabilityIntensity]; got != 0.2 {
		t.Fatalf("expected initial intensity=0.2 (sceneA), got %v", got)
	}

	// D-08: SwitchScene against the live-active scene requires no
	// preceding lock/pause/detach call -- Engine exposes no such API.
	if err := e.SwitchScene(sceneBName); err != nil {
		t.Fatalf("SwitchScene: %v", err)
	}

	// Still mid-bar-0 (secondsPerBar=2s): the staged switch must NOT be
	// adopted yet.
	e.tick(fixedEngineLoopStart.Add(500 * time.Millisecond))
	if got := e.CurrentFrame().Values[instanceID].Values[fixture.CapabilityIntensity]; got != 0.2 {
		t.Fatalf("expected the staged switch to NOT be adopted mid-bar, still want 0.2, got %v", got)
	}

	// Crossing into bar 1 (elapsed >= 2s): the switch is now adopted.
	e.tick(fixedEngineLoopStart.Add(2 * time.Second))
	if got := e.CurrentFrame().Values[instanceID].Values[fixture.CapabilityIntensity]; got != 0.9 {
		t.Fatalf("expected the staged switch to be adopted at the bar boundary, want 0.9, got %v", got)
	}
}

func TestEngineStageEditRejectsInvalidLeavesPlansUntouched(t *testing.T) {
	state, instanceID, _ := newEngineTestState(t)

	e, err := NewEngine(state)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	pinLoopStart(e)

	beforeActive := e.activePlan.Load()
	beforePending := e.pendingPlan.Load()

	invalid := state
	invalid.Tempo = show.Tempo{BPM: -1}
	if err := e.StageEdit(invalid); err == nil || !strings.Contains(err.Error(), "GOLC_PLAYBACK_PLAN_INVALID") {
		t.Fatalf("expected GOLC_PLAYBACK_PLAN_INVALID for an invalid staged edit, got %v", err)
	}

	if e.activePlan.Load() != beforeActive {
		t.Fatalf("expected activePlan untouched by a rejected StageEdit")
	}
	if e.pendingPlan.Load() != beforePending {
		t.Fatalf("expected pendingPlan untouched by a rejected StageEdit")
	}

	// The engine keeps running the last valid plan -- the running layer is
	// never blanked or disabled by a rejected edit.
	e.tick(fixedEngineLoopStart)
	if got := e.CurrentFrame().Values[instanceID].Values[fixture.CapabilityIntensity]; got != 0.2 {
		t.Fatalf("expected the last valid plan (intensity=0.2) to keep running, got %v", got)
	}
}

func TestEngineStageEditLiveActiveObjectNoLockRequired(t *testing.T) {
	state, instanceID, _ := newEngineTestState(t)

	e, err := NewEngine(state)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	// presetA (state.Presets[0]) is live in the currently active scene's
	// base-look layer. Edit it directly -- no pause/detach/lock call
	// precedes this StageEdit (CONTEXT D-08: Engine exposes no such API).
	edited := state
	editedPresets := make([]programming.Preset, len(state.Presets))
	copy(editedPresets, state.Presets)
	editedPresets[0].Attributes = []programming.PresetAttribute{
		{InstanceID: instanceID, Capability: fixture.CapabilityIntensity, Value: 0.42},
	}
	edited.Presets = editedPresets

	if err := e.StageEdit(edited); err != nil {
		t.Fatalf("StageEdit against a live-active object: %v", err)
	}
}

func TestEngineDelayedTickMatchesSequentialTicks(t *testing.T) {
	state, instanceID, sceneBName := newEngineTestState(t)

	seq, err := NewEngine(state)
	if err != nil {
		t.Fatalf("NewEngine (seq): %v", err)
	}
	pinLoopStart(seq)
	if err := seq.SwitchScene(sceneBName); err != nil {
		t.Fatalf("SwitchScene (seq): %v", err)
	}
	for _, offset := range []time.Duration{0, time.Second, 2 * time.Second, 3 * time.Second, 4 * time.Second, 5 * time.Second} {
		seq.tick(fixedEngineLoopStart.Add(offset))
	}
	seqIntensity := seq.CurrentFrame().Values[instanceID].Values[fixture.CapabilityIntensity]

	delayed, err := NewEngine(state)
	if err != nil {
		t.Fatalf("NewEngine (delayed): %v", err)
	}
	pinLoopStart(delayed)
	if err := delayed.SwitchScene(sceneBName); err != nil {
		t.Fatalf("SwitchScene (delayed): %v", err)
	}
	// A single coalesced tick jumps straight to the same final "now",
	// skipping every intermediate bar boundary a stalled/late tick would
	// have missed.
	delayed.tick(fixedEngineLoopStart.Add(5 * time.Second))
	delayedIntensity := delayed.CurrentFrame().Values[instanceID].Values[fixture.CapabilityIntensity]

	if delayedIntensity != seqIntensity {
		t.Fatalf("delayed single tick = %v, want the same result as sequential ticking = %v", delayedIntensity, seqIntensity)
	}
}

func TestEngineStartStopCleanShutdown(t *testing.T) {
	state, _, _ := newEngineTestState(t)

	e, err := NewEngine(state)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	e.Start(ctx)

	time.Sleep(3 * tickInterval)
	e.Stop()

	if e.CurrentFrame() == nil {
		t.Fatalf("expected a non-nil CurrentFrame after Start/Stop")
	}
}

func TestEngineCurrentFrameNonBlockingUnderConcurrentTick(t *testing.T) {
	state, _, _ := newEngineTestState(t)

	e, err := NewEngine(state)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	e.Start(ctx)
	defer e.Stop()

	var wg sync.WaitGroup
	stop := make(chan struct{})
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					_ = e.CurrentFrame()
				}
			}
		}()
	}
	time.Sleep(5 * tickInterval)
	close(stop)
	wg.Wait()
}

// TestEngineBPMChangePreservesPosition proves CR-01/SCEN-08/D-11's
// preserve contract is actually observable through the real tick loop
// (not merely at the RecomputeEpoch/clock_test.go primitive level): a
// staged edit that changes Tempo.BPM on a scene whose
// PreserveMusicalPositionOnBPMChange is true, once adopted at the next
// bar-boundary crossing, leaves the running bar/beat position unchanged
// from what it was, an instant earlier, under the old BPM -- it neither
// blanks nor jumps mid-bar.
func TestEngineBPMChangePreservesPosition(t *testing.T) {
	state, _, _ := newEngineTestState(t)
	state.Scenes[0].PreserveMusicalPositionOnBPMChange = true
	// secondsPerBar(120) = 2s; secondsPerBar(90) = 2.6667s.
	newBPM := 90.0

	e, err := NewEngine(state)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	pinLoopStart(e)
	e.tick(fixedEngineLoopStart) // establish bar-0 baseline

	edited := state
	edited.Tempo = show.Tempo{BPM: newBPM}
	if err := e.StageEdit(edited); err != nil {
		t.Fatalf("StageEdit (BPM change): %v", err)
	}

	// Still bar 0 under the OLD plan's BPM (elapsed=0.5s < secondsPerBar=2s):
	// the staged BPM change must not be adopted yet.
	e.tick(fixedEngineLoopStart.Add(500 * time.Millisecond))
	if e.activePlan.Load().BPM != 120 {
		t.Fatalf("expected the staged BPM change to NOT be adopted mid-bar, activePlan.BPM=%v", e.activePlan.Load().BPM)
	}

	// before is the position the OLD (120bpm) plan would report at the
	// exact instant the crossing tick below fires (2.5s elapsed since
	// loopStart -- bar 1, 0.25 through the bar).
	crossingNow := fixedEngineLoopStart.Add(2500 * time.Millisecond)
	before := Position(crossingNow, 120.0, 2, fixedEngineLoopStart)
	if before.BarIndex != 1 {
		t.Fatalf("test setup: expected bar 1 just before the crossing tick, got BarIndex=%d", before.BarIndex)
	}

	// This tick crosses the bar-1 boundary under the still-active 120bpm
	// plan, so the staged 90bpm plan is adopted here.
	e.tick(crossingNow)
	if got := e.activePlan.Load().BPM; got != newBPM {
		t.Fatalf("expected the staged BPM change to be adopted at the bar boundary, activePlan.BPM=%v", got)
	}

	after := Position(crossingNow, newBPM, 2, e.loopStart)
	if after.BarIndex != before.BarIndex {
		t.Fatalf("preserve=true: BarIndex jumped across the BPM change: before=%d after=%d", before.BarIndex, after.BarIndex)
	}
	if diff := after.BeatFraction - before.BeatFraction; diff < -1e-6 || diff > 1e-6 {
		t.Fatalf("preserve=true: BeatFraction jumped across the BPM change: before=%v after=%v", before.BeatFraction, after.BeatFraction)
	}
}

// TestEngineBPMChangeRestartsAtBarZero proves the mirror-image restart
// contract (CR-01/SCEN-08/D-11): a staged BPM change on a scene whose
// PreserveMusicalPositionOnBPMChange is false, once adopted at the next
// bar-boundary crossing, restarts the loop at bar 0 rather than preserving
// (or arbitrarily jumping past) the prior bar/beat position.
func TestEngineBPMChangeRestartsAtBarZero(t *testing.T) {
	state, _, _ := newEngineTestState(t)
	state.Scenes[0].PreserveMusicalPositionOnBPMChange = false
	newBPM := 90.0

	e, err := NewEngine(state)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	pinLoopStart(e)
	e.tick(fixedEngineLoopStart)

	edited := state
	edited.Tempo = show.Tempo{BPM: newBPM}
	if err := e.StageEdit(edited); err != nil {
		t.Fatalf("StageEdit (BPM change): %v", err)
	}

	e.tick(fixedEngineLoopStart.Add(500 * time.Millisecond))

	crossingNow := fixedEngineLoopStart.Add(2500 * time.Millisecond)
	e.tick(crossingNow)
	if got := e.activePlan.Load().BPM; got != newBPM {
		t.Fatalf("expected the staged BPM change to be adopted at the bar boundary, activePlan.BPM=%v", got)
	}

	after := Position(crossingNow, newBPM, 2, e.loopStart)
	if after.BarIndex != 0 || after.BeatFraction != 0.0 {
		t.Fatalf("preserve=false: expected a restart at bar 0, got %+v", after)
	}
}

func TestCrossedBarBoundarySentinelAndWraparound(t *testing.T) {
	if !crossedBarBoundary(-1, 0, 4) {
		t.Fatalf("expected the -1 sentinel to always report crossed")
	}
	if !crossedBarBoundary(4, 0, 4) {
		t.Fatalf("expected an out-of-range lastBar (stale from a differently-sized loop) to always report crossed")
	}
	if crossedBarBoundary(2, 2, 4) {
		t.Fatalf("expected no transition when BarIndex is unchanged")
	}
	if !crossedBarBoundary(2, 3, 4) {
		t.Fatalf("expected a transition when BarIndex changes")
	}
}
