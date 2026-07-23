// engine.go implements the real-time playback engine (CONTEXT SCEN-06/
// SCEN-09, D-05/D-06/D-07/D-08, 03-RESEARCH.md Pattern 3): Engine's
// activeFrame/activePlan/pendingPlan are published and read exclusively
// through atomic.Pointer -- every crossing of the tick loop <-> adapter
// (UI/persistence/scripts/API/LLM) boundary is a non-blocking Load/Store,
// never an awaited call, so no adapter can ever block or backpressure
// playback (CONTEXT "Live reliability" constraint). tick(now) detects a
// bar-boundary crossing by comparing integer BarIndex values
// (crossedBarBoundary), never a BeatFraction equality check
// (03-RESEARCH.md Pitfall 1), and promotes a staged pendingPlan to
// activePlan only at that crossing (CONTEXT D-05). StageEdit/SwitchScene
// reject an invalid edit with GOLC_PLAYBACK_PLAN_INVALID and leave
// activePlan/pendingPlan completely untouched (CONTEXT D-06) -- the
// running layer is never blanked or disabled by a rejected edit. Neither
// StageEdit nor SwitchScene requires any preceding pause/detach/lock call
// (CONTEXT D-08): the engine exposes no such API at all -- the next-bar
// adoption boundary alone keeps live output safe. When the plan adopted at
// a bar-boundary crossing carries a different BPM than the plan it
// replaces, tick recomputes loopStart via playback.RecomputeEpoch (CONTEXT
// SCEN-08/D-11) according to the newly adopted plan's own
// PreserveOnBPMChange flag, so the running look/chase/motion neither
// blanks nor jumps mid-bar across a BPM change either.
//
// No manual Windows timer-resolution call (winmm/timeBeginPeriod/
// NtSetTimerResolution) is added anywhere in this file: Go 1.26.5 (this
// repo's pinned toolchain) already provides ~0.5ms Windows timer
// resolution natively (03-RESEARCH.md Pitfall 2) -- reintroducing that
// pre-Go-1.23 workaround here would be a regression, not a fix.
package playback

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/lnorton89/golc/internal/scene"
	"github.com/lnorton89/golc/internal/show"
)

// tickHz is the engine's tick cadence (03-RESEARCH.md Open Question 1/
// Assumption A2): 40Hz is the recommended concrete constant within the
// DMX/Art-Net industry-standard 30-40Hz refresh band, documented as
// adjustable -- Phase 4 (Art-Net) may tune this single constant without
// any architecture change.
const tickHz = 40

// tickInterval is the concrete engine tick period derived from tickHz.
const tickInterval = time.Second / tickHz

// Engine is the real-time-isolated playback tick loop (CONTEXT SCEN-09):
// activeFrame/activePlan/pendingPlan are published and read exclusively
// via atomic.Pointer, so every reader (CurrentFrame) and every writer
// (StageEdit/SwitchScene) is non-blocking. loopStart is the engine's
// musical-clock epoch, established at construction (03-RESEARCH.md
// Pitfall 3: never resume mid-bar across a restart -- every fresh Engine
// begins at bar 0 from a fresh time.Now() origin) and thereafter
// recomputed by tick, exactly once per adopted plan whose BPM differs from
// the plan it replaces, via RecomputeEpoch (CONTEXT SCEN-08/D-11) -- never
// otherwise reassigned; lastBar is touched only by the tick goroutine
// itself (single-writer, read only by tick), so it needs no atomic
// wrapper. lastState holds the most recently successfully compiled
// show.State (via atomic.Pointer, since SwitchScene reads it from
// whichever goroutine calls it) so SwitchScene(name) can activate a scene
// without the caller having to resupply the whole show.State again.
type Engine struct {
	activeFrame atomic.Pointer[Frame]
	activePlan  atomic.Pointer[CompiledPlan]
	pendingPlan atomic.Pointer[CompiledPlan]
	lastState   atomic.Pointer[show.State]
	// position is the most recently computed MusicalPosition (06-05-PLAN.md
	// PLAY-07), published via the same lock-free atomic.Pointer discipline
	// as activeFrame -- every tick (and NewEngine's own first position
	// computation) Stores a fresh value here so CurrentPosition() never
	// needs to re-derive "now" against loopStart itself (loopStart is
	// mutated by the tick goroutine alone; re-deriving from a reader
	// goroutine would race).
	position atomic.Pointer[MusicalPosition]

	loopStart time.Time
	lastBar   int

	cancel context.CancelFunc
}

// NewEngine compiles state (CONTEXT D-06: an invalid initial state is
// rejected before any Engine is ever returned -- there is no such thing as
// an Engine running an invalid plan), establishes a fresh loopStart epoch
// at bar 0 (03-RESEARCH.md Pitfall 3), and publishes the first activePlan/
// activeFrame.
func NewEngine(state show.State) (*Engine, error) {
	plan, err := Compile(state)
	if err != nil {
		return nil, err
	}

	e := &Engine{loopStart: time.Now(), lastBar: -1}
	e.activePlan.Store(&plan)
	e.lastState.Store(&state)

	pos := Position(e.loopStart, plan.BPM, plan.BarsPerLoop, e.loopStart)
	frame := Evaluate(plan, pos)
	e.activeFrame.Store(&frame)
	e.position.Store(&pos)
	e.lastBar = pos.BarIndex

	return e, nil
}

// crossedBarBoundary reports whether curBar represents a bar transition
// since lastBar, given the CURRENT plan's barsPerLoop (03-RESEARCH.md
// Pitfall 1: never a BeatFraction equality check). lastBar == -1 is the
// engine's own "no tick has run yet" sentinel and always reports crossed,
// so the very first tick establishes a baseline cleanly. A stale lastBar
// left over from a prior plan whose own BarsPerLoop no longer matches
// barsPerLoop (for example immediately after a scene switch to a
// differently-sized loop) is out of range for the new loop and also
// always reports crossed, since it cannot meaningfully be compared against
// curBar under the new loop length.
func crossedBarBoundary(lastBar, curBar, barsPerLoop int) bool {
	if lastBar < 0 || lastBar >= barsPerLoop {
		return true
	}
	return curBar != lastBar
}

// tick computes now's musical position against the currently active plan,
// promotes a staged pendingPlan to activePlan when (and only when) this
// tick crosses a bar boundary (CONTEXT D-05), and publishes the resulting
// Frame. tick is a pure function of (e's current published state, now):
// feeding a "late" now that jumped forward past one or more bar boundaries
// (a stalled/coalesced tick) produces the exact same final activePlan and
// Frame a sequence of on-time ticks reaching that same final now would
// have produced (SCEN-09) -- because Position is itself a pure function
// of now (03-RESEARCH.md Pattern 1) and a single already-staged
// pendingPlan is promoted exactly once, at the first crossing tick
// observes, regardless of how many boundaries that tick's jump spanned.
//
// When the newly adopted plan's BPM differs from the plan it replaces,
// loopStart is recomputed via RecomputeEpoch (CONTEXT SCEN-08/D-11) before
// the new plan's position is computed: the newly adopted plan's own
// PreserveOnBPMChange flag decides whether the running bar/beat location
// is preserved across the change or the loop restarts at bar 0 -- either
// way the adoption never blanks or jumps mid-bar.
func (e *Engine) tick(now time.Time) {
	plan := e.activePlan.Load()
	if plan == nil {
		return
	}
	pos := Position(now, plan.BPM, plan.BarsPerLoop, e.loopStart)

	if pending := e.pendingPlan.Load(); pending != nil && crossedBarBoundary(e.lastBar, pos.BarIndex, plan.BarsPerLoop) {
		if pending.BPM != plan.BPM {
			e.loopStart = RecomputeEpoch(pending.PreserveOnBPMChange, plan.BPM, pending.BPM, plan.BarsPerLoop, e.loopStart, now)
		}
		e.activePlan.Store(pending)
		e.pendingPlan.Store(nil)
		plan = pending
		// Recompute pos against the newly adopted plan's own BPM/
		// BarsPerLoop: a switch may change BarsPerLoop, and recomputing
		// keeps BarIndex wrapping correct under the newly active loop
		// length, and reflects any loopStart recompute above.
		pos = Position(now, plan.BPM, plan.BarsPerLoop, e.loopStart)
	}

	e.lastBar = pos.BarIndex
	frame := Evaluate(*plan, pos)
	e.activeFrame.Store(&frame)
	e.position.Store(&pos)
}

// StageEdit compiles state and, on success, stages the result as
// pendingPlan for adoption at the next bar-boundary crossing (CONTEXT
// D-05); on failure it returns GOLC_PLAYBACK_PLAN_INVALID and leaves
// activePlan/pendingPlan completely untouched -- the engine keeps running
// the last valid compiled plan (CONTEXT D-06), and the running layer is
// never blanked or disabled by a rejected edit. StageEdit never blocks:
// storing pendingPlan is a single non-blocking atomic.Pointer.Store the
// tick loop later reads without ever awaiting this call. StageEdit
// requires no preceding pause/detach/lock call (CONTEXT D-08) -- it may be
// called against an object that is live in the currently active scene at
// any time.
func (e *Engine) StageEdit(state show.State) error {
	plan, err := Compile(state)
	if err != nil {
		return err
	}
	e.pendingPlan.Store(&plan)
	e.lastState.Store(&state)
	return nil
}

// SwitchScene stages an active-scene switch (CONTEXT SCEN-06): it
// activates the named scene on the most recently staged show.State (never
// mutating the caller's own state -- ActivateScene returns a fresh
// scenes slice), recompiles, and stages the result through the same
// StageEdit path -- adopted at the next bar boundary exactly like any
// other edit (CONTEXT D-05/D-07: one consistent adoption rule for every
// layer type and for a scene switch alike). An unknown scene name fails
// with GOLC_PLAYBACK_SWITCH_UNKNOWN_SCENE before any compile is attempted.
func (e *Engine) SwitchScene(name string) error {
	statePtr := e.lastState.Load()
	if statePtr == nil {
		return fmt.Errorf("GOLC_PLAYBACK_SWITCH_UNKNOWN_SCENE: no show state has been staged yet")
	}
	state := *statePtr

	activated, err := scene.ActivateScene(state.Scenes, name)
	if err != nil {
		return fmt.Errorf("GOLC_PLAYBACK_SWITCH_UNKNOWN_SCENE: %v", err)
	}
	state.Scenes = activated
	return e.StageEdit(state)
}

// CurrentFrame returns the latest published Frame via a non-blocking
// atomic Load -- safe to call from any goroutine, any number of times,
// without ever blocking or slowing the tick loop (CONTEXT SCEN-09: no
// adapter may backpressure playback).
func (e *Engine) CurrentFrame() *Frame {
	return e.activeFrame.Load()
}

// CurrentPlan returns the most recently adopted CompiledPlan via a
// non-blocking atomic Load, matching CurrentFrame's own lock-free read
// discipline (06-05-PLAN.md PLAY-07 status projection: SceneID/BPM/
// BarsPerLoop/Layers). Returns nil only for a defensive zero-value Engine
// (e.g. constructed directly by a test rather than via NewEngine) --
// every Engine returned by NewEngine has already Stored its first plan
// before returning. Callers must treat a nil result as "no active plan"
// (PLAY-07 idle edge) rather than assuming one always exists.
func (e *Engine) CurrentPlan() *CompiledPlan {
	return e.activePlan.Load()
}

// CurrentPosition returns the most recently published MusicalPosition via
// a non-blocking atomic Load (06-05-PLAN.md PLAY-07: BPM/bar position).
// Returns the zero MusicalPosition (BarIndex 0, BeatFraction 0) for a
// defensive zero-value Engine that has never ticked or been constructed
// via NewEngine.
func (e *Engine) CurrentPosition() MusicalPosition {
	if p := e.position.Load(); p != nil {
		return *p
	}
	return MusicalPosition{}
}

// ActiveSceneName resolves CurrentPlan()'s SceneID against the most
// recently staged show.State's Scenes (06-05-PLAN.md PLAY-07). Returns
// ("", false) when there is no current plan, no staged state, or the
// referenced scene can no longer be found in that state (e.g. a
// concurrent edit removed it) -- callers must render this as an explicit
// idle/unknown-scene state (PLAY-07 idle edge), never a blank name
// standing in for "found, but empty."
func (e *Engine) ActiveSceneName() (string, bool) {
	plan := e.activePlan.Load()
	if plan == nil {
		return "", false
	}
	statePtr := e.lastState.Load()
	if statePtr == nil {
		return "", false
	}
	for _, s := range statePtr.Scenes {
		if s.ID == plan.SceneID {
			return s.Name, true
		}
	}
	return "", false
}

// Start drives tick via a time.Ticker at tickInterval until ctx is
// cancelled or Stop is called. Start spawns exactly one goroutine; the
// caller owns the Engine's lifecycle (Start/Stop are not intended to be
// called concurrently with each other from multiple goroutines -- a
// single owner starts and later stops one Engine, matching this repo's
// existing context-based shutdown idiom).
func (e *Engine) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	e.cancel = cancel

	ticker := time.NewTicker(tickInterval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				e.tick(now)
			}
		}
	}()
}

// Stop cancels the context Start derived, terminating the tick goroutine
// cleanly. Calling Stop before Start (or more than once) is a safe no-op.
func (e *Engine) Stop() {
	if e.cancel != nil {
		e.cancel()
	}
}
