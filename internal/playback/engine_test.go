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
// sleeps.
//
// The two lifecycle tests that must drive the real ticker goroutine
// instead of calling tick directly are split deliberately:
// TestEngineStartStopCleanShutdown runs in a testing/synctest bubble
// (stable in Go 1.26, this repo's pinned toolchain) so its ticks come
// from a fake clock and cost no wall-clock time, while
// TestEngineCurrentFrameNonBlockingUnderConcurrentTick deliberately does
// NOT -- see its own comment for why a busy-spin reader is the one shape
// synctest cannot model.
package playback

import (
	"context"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

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
	require.NoError(t, err, "NewPreset(A)")
	presetA.Attributes = []programming.PresetAttribute{{InstanceID: instance.ID, Capability: fixture.CapabilityIntensity, Value: 0.2}}

	presetB, err := programming.NewPreset("B", programming.PresetIntensity)
	require.NoError(t, err, "NewPreset(B)")
	presetB.Attributes = []programming.PresetAttribute{{InstanceID: instance.ID, Capability: fixture.CapabilityIntensity, Value: 0.9}}

	sceneA, err := scene.NewScene("SceneA", 2)
	require.NoError(t, err, "NewScene(A)")
	sceneA.Active = true
	sceneA, err = scene.SetLayer(sceneA, scene.Layer{Kind: scene.BaseLook, Enabled: true, Selection: sel, Ref: presetA.ID})
	require.NoError(t, err, "SetLayer(A)")

	sceneB, err := scene.NewScene("SceneB", 2)
	require.NoError(t, err, "NewScene(B)")
	sceneB, err = scene.SetLayer(sceneB, scene.Layer{Kind: scene.BaseLook, Enabled: true, Selection: sel, Ref: presetB.ID})
	require.NoError(t, err, "SetLayer(B)")

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
	require.NoError(t, err, "NewEngine")
	pinLoopStart(e)
	e.tick(fixedEngineLoopStart) // establish a clean bar-0 baseline

	require.InDelta(t, 0.2, e.CurrentFrame().Values[instanceID].Values[fixture.CapabilityIntensity], 1e-9, "expected initial intensity=0.2 (sceneA)")

	// D-08: SwitchScene against the live-active scene requires no
	// preceding lock/pause/detach call -- Engine exposes no such API.
	require.NoError(t, e.SwitchScene(sceneBName), "SwitchScene")

	// Still mid-bar-0 (secondsPerBar=2s): the staged switch must NOT be
	// adopted yet.
	e.tick(fixedEngineLoopStart.Add(500 * time.Millisecond))
	require.InDelta(t, 0.2, e.CurrentFrame().Values[instanceID].Values[fixture.CapabilityIntensity], 1e-9, "expected the staged switch to NOT be adopted mid-bar")

	// Crossing into bar 1 (elapsed >= 2s): the switch is now adopted.
	e.tick(fixedEngineLoopStart.Add(2 * time.Second))
	require.InDelta(t, 0.9, e.CurrentFrame().Values[instanceID].Values[fixture.CapabilityIntensity], 1e-9, "expected the staged switch to be adopted at the bar boundary")
}

func TestEngineStageEditRejectsInvalidLeavesPlansUntouched(t *testing.T) {
	state, instanceID, _ := newEngineTestState(t)

	e, err := NewEngine(state)
	require.NoError(t, err, "NewEngine")
	pinLoopStart(e)

	beforeActive := e.activePlan.Load()
	beforePending := e.pendingPlan.Load()

	invalid := state
	invalid.Tempo = show.Tempo{BPM: -1}
	err = e.StageEdit(invalid)
	require.ErrorContains(t, err, "GOLC_PLAYBACK_PLAN_INVALID", "expected error for an invalid staged edit")

	require.Same(t, beforeActive, e.activePlan.Load(), "expected activePlan untouched by a rejected StageEdit")
	require.Same(t, beforePending, e.pendingPlan.Load(), "expected pendingPlan untouched by a rejected StageEdit")

	// The engine keeps running the last valid plan -- the running layer is
	// never blanked or disabled by a rejected edit.
	e.tick(fixedEngineLoopStart)
	require.InDelta(t, 0.2, e.CurrentFrame().Values[instanceID].Values[fixture.CapabilityIntensity], 1e-9, "expected the last valid plan (intensity=0.2) to keep running")
}

func TestEngineStageEditLiveActiveObjectNoLockRequired(t *testing.T) {
	state, instanceID, _ := newEngineTestState(t)

	e, err := NewEngine(state)
	require.NoError(t, err, "NewEngine")

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

	require.NoError(t, e.StageEdit(edited), "StageEdit against a live-active object")
}

func TestEngineDelayedTickMatchesSequentialTicks(t *testing.T) {
	state, instanceID, sceneBName := newEngineTestState(t)

	seq, err := NewEngine(state)
	require.NoError(t, err, "NewEngine (seq)")
	pinLoopStart(seq)
	require.NoError(t, seq.SwitchScene(sceneBName), "SwitchScene (seq)")
	for _, offset := range []time.Duration{0, time.Second, 2 * time.Second, 3 * time.Second, 4 * time.Second, 5 * time.Second} {
		seq.tick(fixedEngineLoopStart.Add(offset))
	}
	seqIntensity := seq.CurrentFrame().Values[instanceID].Values[fixture.CapabilityIntensity]

	delayed, err := NewEngine(state)
	require.NoError(t, err, "NewEngine (delayed)")
	pinLoopStart(delayed)
	require.NoError(t, delayed.SwitchScene(sceneBName), "SwitchScene (delayed)")
	// A single coalesced tick jumps straight to the same final "now",
	// skipping every intermediate bar boundary a stalled/late tick would
	// have missed.
	delayed.tick(fixedEngineLoopStart.Add(5 * time.Second))
	delayedIntensity := delayed.CurrentFrame().Values[instanceID].Values[fixture.CapabilityIntensity]

	require.InDelta(t, seqIntensity, delayedIntensity, 1e-9, "delayed single tick should match sequential ticking")
}

// TestEngineStartStopCleanShutdown drives the real ticker goroutine
// Start installs, then proves Stop shuts it down cleanly. It runs in a
// synctest bubble: the engine goroutine parks on ticker.C (durably
// blocked) and the test goroutine parks in time.Sleep, so the fake clock
// advances exactly three tick periods at zero wall-clock cost. The
// synctest.Wait() after Stop is what makes "shut down cleanly"
// observable rather than assumed -- it returns only once the ticker
// goroutine has actually observed ctx.Done() and exited, and the bubble
// itself would fail the test if that goroutine ever leaked.
func TestEngineStartStopCleanShutdown(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		state, _, _ := newEngineTestState(t)

		e, err := NewEngine(state)
		require.NoError(t, err, "NewEngine")

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		e.Start(ctx)

		time.Sleep(3 * tickInterval)
		synctest.Wait()
		require.NotNil(t, e.CurrentFrame(), "expected a non-nil CurrentFrame while running")

		e.Stop()
		synctest.Wait()

		require.NotNil(t, e.CurrentFrame(), "expected a non-nil CurrentFrame after Start/Stop")
	})
}

// TestEngineCurrentFrameNonBlockingUnderConcurrentTick is deliberately
// NOT wrapped in synctest.Test, and must not be. Its eight reader
// goroutines spin on `default: _ = e.CurrentFrame()`, which never
// durably blocks -- and a synctest bubble's clock advances only once
// every goroutine in it is durably blocked, so the fake clock would
// never move and the bubble would hang instead of running the test.
// That is exactly the property this test exists to prove (the lock-free
// atomic.Pointer read never blocks), so it keeps the real clock and a
// real time.Sleep. The sleep is 5 tick periods at tickHz -- ~125ms.
func TestEngineCurrentFrameNonBlockingUnderConcurrentTick(t *testing.T) {
	state, _, _ := newEngineTestState(t)

	e, err := NewEngine(state)
	require.NoError(t, err, "NewEngine")

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
	require.NoError(t, err, "NewEngine")
	pinLoopStart(e)
	e.tick(fixedEngineLoopStart) // establish bar-0 baseline

	edited := state
	edited.Tempo = show.Tempo{BPM: newBPM}
	require.NoError(t, e.StageEdit(edited), "StageEdit (BPM change)")

	// Still bar 0 under the OLD plan's BPM (elapsed=0.5s < secondsPerBar=2s):
	// the staged BPM change must not be adopted yet.
	e.tick(fixedEngineLoopStart.Add(500 * time.Millisecond))
	require.EqualValues(t, 120, e.activePlan.Load().BPM, "expected the staged BPM change to NOT be adopted mid-bar")

	// before is the position the OLD (120bpm) plan would report at the
	// exact instant the crossing tick below fires (2.5s elapsed since
	// loopStart -- bar 1, 0.25 through the bar).
	crossingNow := fixedEngineLoopStart.Add(2500 * time.Millisecond)
	before := Position(crossingNow, 120.0, 2, fixedEngineLoopStart)
	require.Equal(t, 1, before.BarIndex, "test setup: expected bar 1 just before the crossing tick")

	// This tick crosses the bar-1 boundary under the still-active 120bpm
	// plan, so the staged 90bpm plan is adopted here.
	e.tick(crossingNow)
	require.Equal(t, newBPM, e.activePlan.Load().BPM, "expected the staged BPM change to be adopted at the bar boundary")

	after := Position(crossingNow, newBPM, 2, e.loopStart)
	require.Equal(t, before.BarIndex, after.BarIndex, "preserve=true: BarIndex jumped across the BPM change")
	require.InDelta(t, before.BeatFraction, after.BeatFraction, 1e-6, "preserve=true: BeatFraction jumped across the BPM change")
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
	require.NoError(t, err, "NewEngine")
	pinLoopStart(e)
	e.tick(fixedEngineLoopStart)

	edited := state
	edited.Tempo = show.Tempo{BPM: newBPM}
	require.NoError(t, e.StageEdit(edited), "StageEdit (BPM change)")

	e.tick(fixedEngineLoopStart.Add(500 * time.Millisecond))

	crossingNow := fixedEngineLoopStart.Add(2500 * time.Millisecond)
	e.tick(crossingNow)
	require.Equal(t, newBPM, e.activePlan.Load().BPM, "expected the staged BPM change to be adopted at the bar boundary")

	after := Position(crossingNow, newBPM, 2, e.loopStart)
	require.Equal(t, 0, after.BarIndex, "preserve=false: expected a restart at bar 0, got %+v", after)
	require.Equal(t, 0.0, after.BeatFraction, "preserve=false: expected a restart at bar 0, got %+v", after)
}

func TestCrossedBarBoundarySentinelAndWraparound(t *testing.T) {
	require.True(t, crossedBarBoundary(-1, 0, 4), "expected the -1 sentinel to always report crossed")
	require.True(t, crossedBarBoundary(4, 0, 4), "expected an out-of-range lastBar (stale from a differently-sized loop) to always report crossed")
	require.False(t, crossedBarBoundary(2, 2, 4), "expected no transition when BarIndex is unchanged")
	require.True(t, crossedBarBoundary(2, 3, 4), "expected a transition when BarIndex changes")
}

// TestEngineCurrentPlanPositionAndSceneName proves 06-05-PLAN.md's PLAY-07
// status-projection accessors: CurrentPlan/CurrentPosition/ActiveSceneName
// reflect the engine's real running state (including the switch-adopted
// scene's own name, resolved from the most recently staged show.State),
// and CurrentPosition tracks a fresh tick, mirroring CurrentFrame's own
// lock-free read discipline.
func TestEngineCurrentPlanPositionAndSceneName(t *testing.T) {
	state, _, sceneBName := newEngineTestState(t)

	e, err := NewEngine(state)
	require.NoError(t, err, "NewEngine")
	pinLoopStart(e)
	e.tick(fixedEngineLoopStart)

	plan := e.CurrentPlan()
	require.NotNil(t, plan, "expected a non-nil CurrentPlan for a running Engine")
	require.Equal(t, state.Scenes[0].ID, plan.SceneID, "CurrentPlan().SceneID (sceneA)")

	name, ok := e.ActiveSceneName()
	require.True(t, ok)
	require.Equal(t, state.Scenes[0].Name, name)

	pos := e.CurrentPosition()
	require.Equal(t, 0, pos.BarIndex, "CurrentPosition().BarIndex immediately after the baseline tick")

	// Cross into bar 1 and switch to sceneB -- both CurrentPosition and
	// ActiveSceneName must reflect the newly adopted state, never a stale
	// snapshot from before the crossing tick.
	require.NoError(t, e.SwitchScene(sceneBName), "SwitchScene")
	e.tick(fixedEngineLoopStart.Add(2 * time.Second))

	require.Equal(t, 1, e.CurrentPosition().BarIndex, "CurrentPosition().BarIndex after crossing")
	name, ok = e.ActiveSceneName()
	require.True(t, ok)
	require.Equal(t, sceneBName, name)
}

// TestEngineDefensiveZeroValueAccessorsReportIdle proves the PLAY-07 idle
// edge at the accessor level: a defensive zero-value Engine (never
// constructed via NewEngine, e.g. a struct literal a future caller might
// build in error) reports CurrentPlan()==nil, an explicit
// ActiveSceneName()==("", false), and CurrentPosition()==the zero
// MusicalPosition -- never a panic, and never indistinguishable from a
// genuinely running Engine at bar 0.
func TestEngineDefensiveZeroValueAccessorsReportIdle(t *testing.T) {
	var e Engine

	require.Nil(t, e.CurrentPlan(), "expected CurrentPlan() == nil for a zero-value Engine")
	name, ok := e.ActiveSceneName()
	require.False(t, ok, "expected ActiveSceneName() == (\"\", false) for a zero-value Engine")
	require.Empty(t, name, "expected ActiveSceneName() == (\"\", false) for a zero-value Engine")
	require.Equal(t, MusicalPosition{}, e.CurrentPosition(), "expected CurrentPosition() == zero value for a zero-value Engine")
}
