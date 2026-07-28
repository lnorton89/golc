# Phase 3: Deterministic Show Programming and Playback - Research

**Researched:** 2026-07-21
**Domain:** Deterministic real-time show engine (Go), tempo-aware scene/layer compilation, headless authoring domain model
**Confidence:** MEDIUM (Go/Windows timing mechanics are CITED against official sources; scale/jitter/budget numbers are reasoned engineering judgment applied to this project's own stated scale, not externally benchmarked — flagged LOW/ASSUMED where so)

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Scene Layer Semantics**
- **D-01:** A "base-look" layer is the scene's foundational static state — it sets a resting intensity, position/beam, and default color that other enabled layers (color-theme, chase, motion) selectively override on top of. It is not color-only and not a full immutable snapshot; it establishes the rest state a scene returns to for any attribute no other enabled layer touches.
- **D-02:** When two enabled layers touch the same attribute simultaneously, resolution uses a **fixed layer-priority order**: base-look < color-theme < chase < motion. A later layer in that order always overrides an earlier one for any attribute it touches. No HTP arbitration and no per-layer blend-weight mixing — priority order alone is fully deterministic.
- **D-03:** Each layer can independently target its own fixture selection (pool/group/deployment instance/direct fixture, per PROG-01) — a chase or motion preset can be scoped narrower than the scene's base-look (e.g. chase only runs on moving heads while base-look covers the whole rig). Layers are not forced to share one scene-wide selection.
- **D-04:** Motion presets touch only position/beam semantic capabilities (pan/tilt plus beam-shaping: zoom/focus/iris/prism). Color-wheel/gobo-wheel indexing is not part of a motion preset's scope — color effects stay with color-theme/base-look even when they share a physical wheel with beam shaping on some fixtures.

**Live Edit Adoption Boundary**
- **D-05:** An author's edit/record to a programming object (preset, chase, scene) is compiled and staged, then swapped into the running output atomically at the **start of the next musical bar** — not immediately/mid-frame, and not deferred to the next full loop restart. This keeps live output musically coherent (SCEN-09) while adopting promptly.
- **D-06:** If an edit does not fully compile (e.g. references a fixture capability that no longer resolves), it is **rejected and the engine keeps running the last valid compiled version**, surfacing the error to the author. This is the concrete mechanism behind success criterion 5's "adopts only complete valid show plans at safe boundaries" — invalid plans are never partially adopted, and a rejected edit never blanks or disables the running layer.
- **D-07:** The next-bar adoption boundary applies uniformly to every layer type (base-look, color-theme, chase, motion) — one consistent rule per scene, no layer-specific fast path.
- **D-08:** Authors can always edit any object directly, including one currently live in the active scene — there is no explicit pause/detach/lock step required before editing. The adoption-boundary rule (D-05/D-06) is what keeps live output safe; it is not a workflow gate.

**Chase & Motion Determinism**
- **D-09:** Chases and motion presets carry **no randomization in v1** — every step and movement is explicitly authored (ordered steps for chases, authored paths for motion presets). No random-order or random-in-range mode exists to reason about for reproducibility.
- **D-10:** A chase's tempo-relative step advancement is driven by the **same global BPM + bar-position clock** that drives scene looping (SCEN-01/02/03) — one authoritative musical clock for the whole engine, not an independent per-chase rate.
- **D-11:** When global BPM changes while a chase or motion preset is running, its step timing **follows the containing scene's SCEN-08 preserve-position-or-restart setting** — one consistent rule per scene; chases/motion do not have a separate, always-restart override.

**Undo/Redo Scope**
- **D-12:** Undo/redo (PROG-07) uses a **single whole-session linear history** — one global stack across the entire authoring session covering record/update/rename/reorder/duplicate/delete on any object type, walked backward/forward in order. No per-object-type stacks.
- **D-13:** Undo/redo behaves identically whether or not the target object is currently part of the active live scene — an undo is just another edit, recompiled and adopted through the same D-05/D-06 live-adoption boundary. No special-casing or blocking for live-active objects.
- **D-14:** Undo history is **session-only** — in-memory for the current application run, reset on close/reopen. It is not persisted into the `.golc` file; SHOW-01/02 (Phase 5) treat the saved show as the durable unit, not the edit history.

### Claude's Discretion
None — every gray area discussed converged on the recommended option; no "you decide" selections were made in this session.

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope. No scope-creep items came up; all four discussed areas (scene layer semantics, live edit adoption boundary, chase & motion determinism, undo/redo scope) were clarifications of how to implement what's already in PROG-01–07 and SCEN-01–09.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-------------------|
| PROG-01 | A show author can select fixtures by pool, group, deployment instance, or direct fixture selection. | `internal/programming/selection.go` resolves against Phase 2's `pool`/`deployment` types directly (Architecture → Recommended Project Structure) |
| PROG-02 | A show author can edit semantic intensity, color, position, beam, and supported fixture-specific attributes without working in raw DMX channels. | Reuse `fixture.Capability.Range` normalized-value model (Don't Hand-Roll); programmer state lives in `internal/programming/programmer.go` |
| PROG-03 | The programmer shows which attributes are touched, their values, their sources, and what will be recorded. | Programmer state design in `internal/programming/programmer.go` (Recommended Project Structure) |
| PROG-04 | A show author can create reusable color themes and intensity, color, position, and beam presets from programmer state. | `internal/programming/theme.go`/`preset.go`, persisted as new `show.State` fields following the Pool/Deployment pattern (Standard Stack, Architecture Patterns) |
| PROG-05 | A show author can create reusable chases with ordered steps and tempo-relative timing. | `internal/programming/chase.go`; step timing driven by the pure `MusicalPosition` function (Pattern 1, D-10); Open Question 3 flags the bar/beat step-unit decision |
| PROG-06 | A show author can create reusable motion presets using semantic position capabilities. | `internal/programming/motion.go`; scope limited to position/beam per D-04 |
| PROG-07 | A show author can record, update, rename, reorder, duplicate, and delete programming objects with undo and redo. | `internal/programming/history.go`: session-only whole-history linear stack (D-12/13/14), routes through the same compile/stage/adopt pipeline as any edit |
| SCEN-01 | A show author can create a scene that loops for a configured number of musical bars against the global BPM. | `internal/playback/clock.go`'s pure `Position()` function (Pattern 1) computes bar index directly from BPM + bars-per-loop + epoch |
| SCEN-02 | An operator can set global BPM by entering a numeric value. | BPM mutation updates the clock's authoritative state in `internal/playback`; V5 input validation covers sane BPM ranges (Security Domain) |
| SCEN-03 | An operator can set global BPM through tap tempo. | Same clock-mutation path as SCEN-02; tap-tempo-to-BPM conversion is a command-layer concern outside the engine's pure core |
| SCEN-04 | Exactly one scene is active at a time during normal playback. | `internal/scene` model + `activePlan` single-pointer design in the engine (System Architecture Diagram); trivial to enforce given Architectural Responsibility Map |
| SCEN-05 | A scene can combine independently enabled color-theme, chase, motion-preset, and base-look layers. | Pattern 2 (Fixed-Priority Layer Reduce) directly implements this per D-01–D-04 |
| SCEN-06 | An operator can switch the active scene or any scene layer immediately. | Handled the same way as any staged edit — compiled and adopted at the next bar boundary per D-05 (not literally instantaneous mid-frame, per the locked adoption boundary) |
| SCEN-07 | A show author can create and assign reusable blending presets that define transitions between scene and layer states. | `internal/scene/blend.go` data model (Recommended Project Structure); evaluation is engine-internal |
| SCEN-08 | A show author can configure whether a global BPM change preserves the active loop's musical position or restarts the loop. | Epoch recomputation logic in `internal/playback/clock.go` (Open Questions do not flag this — mechanism is a direct epoch-math consequence of Pattern 1) |
| SCEN-09 | Scene timing and layer evaluation remain deterministic when UI rendering, scripts, API clients, or LLM providers are slow or unavailable. | Central architectural insight (Summary): pure-function musical position + lock-free `atomic.Pointer` publish (Pattern 3) makes this structurally guaranteed, verified by the property-style test in Validation Architecture |
</phase_requirements>

## Summary

Phase 3 has almost no open *product* questions left — 03-CONTEXT.md's fourteen decisions (D-01–D-14) already fix layer resolution (priority order, not HTP), the live-edit adoption boundary (next full bar, reject-and-keep-last-valid), chase/motion determinism (no randomization), and undo/redo scope (session-only, whole-history, linear). What remains is **how to build it**: a concrete Go architecture for a real-time-safe musical clock and compiler that is structurally isolated from every adapter (UI, persistence, scripts, API, LLM), sized correctly for this project's own small-rig scale, and correct under Windows timer/scheduler behavior.

The central architectural insight this research surfaces: **musical position, layer resolution, and chase/motion step selection should all be modeled as pure functions of elapsed monotonic time**, not as accumulated/mutable simulation state. Unlike a physics simulation (where skipping a step changes the outcome), "what bar/beat are we on" and "what does layer priority resolve to at this instant" are both idempotently recomputable from `(BPM, loop-start epoch, now)`. This is what makes SCEN-09's determinism-under-adapter-delay requirement structurally free rather than something the engine has to work to guarantee: a stalled tick, a late GC pause, or a slow Windows scheduler wake-up changes *when* the engine computes the answer, never *what* the answer is for a given elapsed time. The only genuinely stateful, order-dependent pieces are the next-bar live-edit adoption boundary (D-05/D-06) and BPM-change epoch recomputation (SCEN-08/D-11), both of which are small, explicit, testable state transitions layered on top of the otherwise-pure position/evaluation functions.

Go 1.26.5 (this repo's pinned toolchain) is well past Go 1.23's Windows high-resolution timer fix, so `time.Ticker`/`time.Timer`/`time.Sleep` already get ~0.5ms resolution on Windows 10+ without any manual `timeBeginPeriod` calls — a legacy technique that is now unnecessary and can even hurt. Combined with a target ~30–40Hz engine tick cadence (the DMX/Art-Net-industry-standard refresh band), the timing budget is generous: sub-millisecond timer jitter against a ~25–33ms frame period. This project's own declared small-rig scale (Phase 2 D-12: ~10–50 fixtures across 3–8 pools, one active scene per SCEN-04) means per-tick evaluation cost is trivial for Go — the real scale question is bounded object counts (chase steps, bar-loop length, saved presets), not per-frame throughput.

**Primary recommendation:** Build the engine as a strictly layered, one-way-dependency package trio — `internal/programming` (selection + programmer state + reusable objects), `internal/scene` (layer/scene model + fixed-priority resolution), `internal/playback` (pure musical clock, pure compiler/evaluator, and the real-time tick-loop engine) — where `internal/playback` imports nothing from `internal/command` or any future UI/API/script/LLM package, and publishes its output via a lock-free `atomic.Pointer[Frame]` that Phase 4's Art-Net worker and Phase 6/7's UI/API layers read without ever being able to block or backpressure the tick loop.

## Architectural Responsibility Map

> Phase 3 is headless (per 03-CONTEXT.md `<domain>`): there is no Browser/SSR/CDN tier yet — Phase 6 adds the Wails UI, Phase 7 the API. Every capability below therefore maps to "API/Backend" (this project's Go domain/engine layer) as the only tier that exists today; the Rationale column captures the finer real-time-engine-vs-domain-command sub-boundary this phase must still enforce structurally so Phase 6/7 can attach without rework.

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Fixture/pool/group/instance selection resolution (PROG-01) | API/Backend | Database/Storage | Resolves against Phase 2's persisted `pool`/`deployment` types; pure lookup, no timing dependency |
| Semantic attribute editing + programmer state (PROG-02/03) | API/Backend | — | In-memory authoring state; not on the real-time tick path |
| Themes/presets/chases/motion presets (PROG-04/05/06) | API/Backend | Database/Storage | Reusable objects persisted as new `show.State` fields, same pattern as Phase 2's pools/deployments |
| Undo/redo (PROG-07) | API/Backend | — | Session-only in-memory stack (D-14); never persisted, never touches the engine's real-time path directly — routes through the same compile/adopt pipeline as any edit (D-13) |
| Scene/layer assembly + fixed-priority resolution (SCEN-01/05) | API/Backend | — | Pure, testable reduce function; must live where `internal/playback` can call it without importing adapter packages |
| BPM entry/tap tempo (SCEN-02/03) | API/Backend | — | Mutates the clock's epoch; command-layer concern, not itself real-time |
| One active scene / immediate switch (SCEN-04/06) | API/Backend (real-time sub-tier: engine) | — | Must be structurally isolated inside `internal/playback` — this is the actual "Live reliability" boundary PROJECT.md names |
| Blending presets (SCEN-07) | API/Backend | — | Data model this phase defines; transition evaluation is engine-internal |
| BPM-change preserve/restart (SCEN-08) | API/Backend (real-time sub-tier: engine) | — | Epoch recomputation must happen inside the engine's authoritative clock, not in a caller |
| Deterministic playback under adapter delay/failure (SCEN-09) | API/Backend (real-time sub-tier: engine) | — | The whole point of `internal/playback`'s isolation; no other tier may touch this path |

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `time` (stdlib) | Go 1.26.5 (pinned, `config/toolchain.toml`) | Monotonic clock reads, ticker-driven tick loop | `time.Now()` carries a monotonic reading automatically; `Sub`/`Since` strip wall-clock jumps for free — no third-party clock library needed [CITED: pkg.go.dev/time monotonic-clock documentation, training knowledge] |
| `sync/atomic` (stdlib) | Go 1.26.5 | Lock-free single-writer/multi-reader `Frame`/`CompiledPlan` snapshot handoff | `atomic.Pointer[T]`/`atomic.Value` is the established Go idiom for "one writer periodically publishes, many readers Load without blocking" [CITED: pkg.go.dev/sync/atomic] |
| `context` (stdlib) | Go 1.26.5 | Engine goroutine lifecycle (start/stop), not per-tick cancellation | Standard Go shutdown idiom; avoid using `context` for anything on the per-tick hot path (allocation-sensitive) |
| `github.com/google/uuid` | v1.6.0 (already in `go.mod`, verified Phase 1/2) | Identity for new programming objects (theme/preset/chase/motion-preset/scene/blend-preset), matching `pool.Pool`/`deployment.Deployment`'s `uuid.NewV7()` pattern | Already vetted and in active use — reuse, do not add a second UUID library [VERIFIED: go.mod] |
| `internal/strictjson` | in-repo (Phase 1) | Strict decode + canonical encode for the new `show.State` fields | Established repo convention every persisted domain type already follows [VERIFIED: internal/show/state.go] |

No new third-party Go modules are required for this phase. The deterministic engine is buildable entirely on the Go standard library plus already-vetted in-repo dependencies.

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `testing` + table-driven tests (stdlib) | Go 1.26.5 | Pure-function determinism tests for the compiler/evaluator (feed synthetic `MusicalPosition` values, assert identical `Frame` output) | Every `internal/playback` pure function — this is what makes SCEN-09 mechanically verifiable in CI, not just architecturally true |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Hand-rolled monotonic epoch math | A third-party "musical clock"/beat-tracking library (e.g. MIDI-clock-oriented packages) | Third-party Go musical-clock libraries target audio/MIDI sync (external clock sources, tempo detection), not "bar-based loop position as a pure function of BPM + epoch" — this project's need is simpler and better served by ~30 lines of stdlib `time` arithmetic than by pulling in an audio-domain dependency with a mismatched problem shape [ASSUMED — no specific library evaluated by name; general domain-fit judgment] |
| `atomic.Pointer[Frame]` broadcast | Buffered channel fan-out to subscribers | A channel-based push model risks the engine blocking (or dropping) if a slow/unavailable consumer doesn't drain — directly against SCEN-09/PROJECT.md's "never backpressure playback" constraint; atomic-pointer pull (readers `Load()` on their own cadence) has no such failure mode [CITED: Go atomic.Value/Pointer documentation pattern] |
| Fixed-priority layer reduce (already locked by D-02) | Runtime HTP/LTP comparison | Explicitly rejected by CONTEXT D-02 — not re-litigated here; included only to note the standard lighting-console alternative this project deliberately does not use |

**Installation:**
```bash
# No new dependencies. Existing go.mod is sufficient:
go build ./...
```

**Version verification:** `github.com/google/uuid` v1.6.0 is already declared in `go.mod` and was verified during Phase 1/2; no re-verification needed since no new package is introduced. Go toolchain pin (`go1.26.5`) confirmed directly from `config/toolchain.toml` [VERIFIED: config/toolchain.toml].

## Package Legitimacy Audit

No external packages are introduced by this phase — the engine is built entirely on the Go standard library (`time`, `sync/atomic`, `context`) plus the already-verified in-repo dependency `github.com/google/uuid` (audited in Phase 1/2). The Package Legitimacy Gate is not applicable; no `npm view`/`pip index`/`cargo search`-equivalent check is needed because no new module is being added to `go.mod`.

**Packages removed due to [SLOP] verdict:** none — no candidate packages were considered.
**Packages flagged as suspicious [SUS]:** none.

## Architecture Patterns

### System Architecture Diagram

```
 Author/operator command (via internal/command, future API/UI/script/LLM adapters)
        │
        ▼
 ┌──────────────────────────┐
 │ internal/programming      │  PROG-01..07
 │  - selection resolution   │  resolves pool/group/deployment-instance/fixture refs
 │  - programmer state        │  touched attrs, values, sources, record scope
 │  - theme/preset/chase/     │  reusable objects, persisted into show.State
 │    motion-preset CRUD      │
 │  - undo/redo history        │  session-only linear stack (D-12/13/14)
 └───────────┬──────────────┘
             │ compiled object refs
             ▼
 ┌──────────────────────────┐
 │ internal/scene            │  SCEN-01,05,07
 │  - scene = bar-loop length │
 │    + 4 independently       │
 │    enabled layers          │
 │  - fixed layer-priority    │
 │    resolve (D-01..D-04)    │
 │  - blending preset model   │
 └───────────┬──────────────┘
             │ Compile(show.State) — validates + flattens
             ▼
 ┌───────────────────────────────────────────────────────────┐
 │ internal/playback  (real-time isolated tier)                │
 │                                                               │
 │  CompiledPlan (immutable)         MusicalClock                │
 │       │                                │                     │
 │       │        ┌───────────────────────┘                     │
 │       ▼        ▼                                              │
 │   Evaluate(plan, position) -> Frame   ◄── pure function        │
 │       │                                                        │
 │       ▼                                                        │
 │  Engine tick loop (time.Ticker, ~30-40Hz)                      │
 │   - reads active atomic.Pointer[CompiledPlan]                  │
 │   - computes position = f(now, bpm, epoch)  [pure]             │
 │   - on bar-boundary crossing: CAS pending→active (D-05/D-06)   │
 │   - publishes atomic.Pointer[Frame]                             │
 └───────────┬──────────────────────────────────────────────────┘
             │ atomic Load() — never blocks, never awaited by engine
             ▼
   Phase 4 Art-Net worker   Phase 6 UI   Phase 7 API   (all future, all read-only consumers)
```

A reader can trace the primary use case end to end: an author edits a scene through `internal/programming`/`internal/scene` → the edit is compiled and staged → `internal/playback`'s engine adopts it at the next bar boundary → the pure `Evaluate` function turns `(CompiledPlan, position)` into a `Frame` every tick → downstream consumers pull the latest `Frame` on their own schedule, never blocking the engine.

### Recommended Project Structure
```
internal/
├── programming/       # PROG-01..07: selection, programmer state, theme/preset/chase/motion CRUD, undo/redo
│   ├── selection.go
│   ├── programmer.go
│   ├── theme.go
│   ├── preset.go
│   ├── chase.go
│   ├── motion.go
│   └── history.go
├── scene/              # SCEN-01,05,07: scene/layer model, fixed-priority resolution, blend presets
│   ├── scene.go
│   ├── layer.go
│   └── blend.go
└── playback/           # SCEN-02,03,04,06,08,09: musical clock, compiler, evaluator, real-time engine
    ├── clock.go        # MusicalPosition, epoch math, BPM set/tap, preserve/restart recompute
    ├── compile.go      # show.State -> CompiledPlan (validates, flattens active scene)
    ├── evaluate.go     # Evaluate(plan, position) -> Frame — pure, deterministic
    ├── engine.go        # tick loop, atomic active/pending CompiledPlan, atomic Frame publish
    └── frame.go         # Frame type: fixture instance -> resolved semantic attribute values
```

`show.State` (in `internal/show`) gains new fields (`Themes`, `Presets`, `Chases`, `MotionPresets`, `Scenes`, `BlendPresets`) following the exact pattern `Pools`/`Deployments`/`Groups` already establish — same `Load`/`Save`/`validate()` shape, same revision bump, same atomic write-temp-then-rename persistence [VERIFIED: internal/show/state.go].

### Pattern 1: Pure Musical Position as a Function of Elapsed Time
**What:** Compute `(barIndex, beatFraction)` directly from `(bpm, barsPerLoop, loopStartEpoch, now)` — never from an accumulated tick counter.
**When to use:** Every place bar/beat position, chase step index, or motion-preset phase is needed.
**Example:**
```go
// Source: pattern synthesized from Go monotonic-time guarantees
// [CITED: pkg.go.dev/time — time.Now() carries a monotonic reading;
// Sub/Since are computed on the monotonic reading when both operands
// have one, making elapsed-time math immune to wall-clock adjustment]
type MusicalPosition struct {
    BarIndex     int     // 0-based bar within the loop
    BeatFraction float64 // 0.0 (start of bar) .. 1.0 (exclusive, next bar)
}

func Position(now time.Time, bpm float64, barsPerLoop int, loopStart time.Time) MusicalPosition {
    secondsPerBeat := 60.0 / bpm
    const beatsPerBar = 4.0 // v1 fixed time signature — confirm/lock during planning if not already fixed elsewhere
    secondsPerBar := secondsPerBeat * beatsPerBar

    elapsed := now.Sub(loopStart).Seconds() // monotonic subtraction, immune to wall-clock jumps
    barsElapsed := elapsed / secondsPerBar
    loopBar := int(barsElapsed) % barsPerLoop
    beatFraction := barsElapsed - float64(int(barsElapsed))

    return MusicalPosition{BarIndex: loopBar, BeatFraction: beatFraction}
}
```
Calling `Position` twice with the same `(now, bpm, barsPerLoop, loopStart)` always returns the same answer — whether `now` was captured right on a tick or 200ms late after a GC pause. This is the mechanism behind SCEN-09.

### Pattern 2: Fixed-Priority Layer Reduce (D-01–D-04)
**What:** For the union of fixture instances selected by any enabled layer, walk layers in the fixed order `base-look < color-theme < chase < motion` and let a later layer overwrite only the semantic attributes it actually touches.
**When to use:** `Evaluate(plan, position) -> Frame`, the core compiler step.
**Example:**
```go
// Source: derived directly from 03-CONTEXT.md D-01..D-04 (locked decisions)
var layerPriority = []LayerKind{BaseLook, ColorTheme, Chase, Motion}

func Evaluate(plan CompiledPlan, pos MusicalPosition) Frame {
    frame := Frame{Values: map[FixtureInstanceID]AttributeSet{}}
    for _, kind := range layerPriority {
        layer, enabled := plan.Layers[kind]
        if !enabled {
            continue
        }
        touched := layer.Resolve(pos) // pure: e.g. chase step = floor(pos-relative-time / stepDuration) % len(steps)
        for instance, attrs := range touched {
            frame.Values[instance] = frame.Values[instance].Overlay(attrs) // later layer wins per-attribute, not per-fixture
        }
    }
    return frame
}
```
No HTP comparison, no blend-weight math — `Overlay` is a plain per-attribute map merge where the caller (loop order) determines precedence, matching D-02 exactly.

### Pattern 3: Lock-Free Snapshot Publish (Engine → Downstream Consumers)
**What:** The engine publishes each computed `Frame` (and, separately, the active `CompiledPlan`) via `atomic.Pointer[T]`; consumers `Load()` independently.
**When to use:** Every boundary between `internal/playback` and any future adapter (Phase 4 Art-Net worker, Phase 6 UI, Phase 7 API).
**Example:**
```go
// Source: pattern synthesized from Go sync/atomic documentation
// [CITED: pkg.go.dev/sync/atomic — atomic.Pointer[T] guarantees a Load
// after a Store observes a fully-formed value, never a torn/partial one]
type Engine struct {
    activeFrame atomic.Pointer[Frame]
    activePlan  atomic.Pointer[CompiledPlan]
    pendingPlan atomic.Pointer[CompiledPlan] // staged edit awaiting next bar (D-05)
}

func (e *Engine) CurrentFrame() *Frame { return e.activeFrame.Load() } // never blocks; safe from any goroutine

func (e *Engine) tick(now time.Time) {
    plan := e.activePlan.Load()
    pos := Position(now, plan.BPM, plan.BarsPerLoop, plan.LoopStart)
    if pending := e.pendingPlan.Load(); pending != nil && crossedBarBoundary(e.lastBar, pos.BarIndex) {
        e.activePlan.Store(pending)
        e.pendingPlan.Store(nil)
        plan = pending
    }
    e.lastBar = pos.BarIndex
    e.activeFrame.Store(Evaluate(plan, pos))
}
```

### Anti-Patterns to Avoid
- **Free-running tick counter as the position source:** if `barIndex` is derived from "how many ticks have fired" rather than elapsed wall/monotonic time, a delayed or coalesced tick (GC pause, Windows scheduler jitter, adapter stall) permanently desyncs playback from real musical time. Always derive position from `now.Sub(epoch)`, never from a counter.
- **Persisting `time.Time` loop-start epochs directly into `show.State`:** Go strips the monotonic reading on any value that has been through an operation like `time.Time` JSON marshaling or `AddDate`; a persisted epoch compared against a fresh `time.Now()` after process restart silently falls back to wall-clock comparison. Treat the loop-start epoch as strictly session-scoped in-memory state — on reload, recompute from the compiled plan's declared BPM/position with a fresh `time.Now()` origin, don't attempt to resume mid-bar across a restart.
- **HTP-style "highest wins" attribute arbitration:** explicitly rejected by D-02. A contributor with prior lighting-console experience may reach for this by habit — guard with a code comment at the layer-reduce site and a unit test asserting priority-order-not-value-magnitude determines the winner.
- **Blocking the engine tick on any adapter call:** any `internal/programming`/`internal/scene` mutation must be staged into `pendingPlan` via a non-blocking path (an already-computed `atomic.Pointer` swap or a bounded/non-blocking channel with a "reject if full" policy) — never a call the tick loop itself waits on.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Monotonic elapsed-time math | A custom wall-clock-diffing helper or manual `syscall.QueryPerformanceCounter` wrapper | `time.Now()` / `time.Since()` / `time.Sub()` (stdlib) | Go's `time.Time` already carries a monotonic reading captured via the OS monotonic clock on every platform including Windows; `Sub`/`Since` use it automatically when both operands have one — reinventing this risks reintroducing exactly the wall-clock-jump bugs the stdlib already solved [CITED: pkg.go.dev/time] |
| Single-writer/many-reader state broadcast | A custom mutex-guarded global plus manual copy-on-read | `atomic.Pointer[T]` (stdlib, Go 1.19+) | Purpose-built for this exact shape; lock-free reads, guaranteed non-torn values, no risk of a reader blocking the writer [CITED: pkg.go.dev/sync/atomic] |
| Attribute value clamping/range enforcement | A new normalized-range validator for intensity/color/position/beam | `fixture.Capability.Range` (Phase 2, `internal/fixture/model.go`) | Phase 2 already established the normalized `[0,1]` capability range model FIXT-02 validates against — Phase 3's programmer/attribute editing should validate against the same source of truth, not a parallel range concept |
| Whole-document referential integrity checking | A bespoke validation pass for scene→layer→chase/theme/preset reference resolution | Extend `show.validate()` (Phase 1/2 pattern) | `show.validate()` already runs whole-State invariant checks (unique names, dangling group references) before Load/Save trusts anything; new object types should extend this single validation path, not introduce a second one [VERIFIED: internal/show/state.go] |

**Key insight:** Every piece of infrastructure this phase needs — monotonic time, lock-free publish, range validation, whole-document integrity — either already exists in the Go standard library or was already built in Phase 1/2 of this exact repository. The only genuinely new code is the domain-specific composition: the musical-position formula, the fixed-priority layer reduce, and the next-bar adoption state machine, none of which have a reusable library that fits this project's specific "bar-based lighting loop with a fixed 4-layer priority" model.

## Common Pitfalls

### Pitfall 1: Treating Bar-Boundary Detection as an Equality Check
**What goes wrong:** Code that checks `if position.BeatFraction == 0.0` to detect "we just crossed into a new bar" will almost never fire, because floating-point position sampled at arbitrary tick times essentially never lands exactly on a boundary.
**Why it happens:** Intuitive translation of "at the start of the bar" into a floating-point equality test.
**How to avoid:** Compare integer `BarIndex` between the previous tick and the current tick (`pos.BarIndex != lastBarIndex`, handling loop wraparound via modulo) — detect the *transition*, not a zero value.
**Warning signs:** Live-edit adoption (D-05) appears to "randomly" skip bars or never adopt; unit tests pass with a mocked exact-tick clock but fail under jittered/randomized tick timing.

### Pitfall 2: Windows Timer Resolution Assumptions From Pre-1.23 Go Experience
**What goes wrong:** A contributor with prior Go/Windows experience may add a manual `winmm.timeBeginPeriod(1)` call (a common pre-Go-1.23 workaround for ~15.6ms default Windows timer granularity), unaware this repo's pinned Go 1.26.5 already includes Go 1.23's high-resolution timer support (~0.5ms) built into the runtime.
**Why it happens:** This is genuinely how Go on Windows behaved for years; the fix (Go 1.23, mid-2024) is recent enough that plenty of existing guidance/blog posts still recommend the manual workaround.
**How to avoid:** Do not add manual Windows timer-resolution calls. Rely on stock `time.NewTicker`/`time.Sleep`; verify actual achieved tick jitter with a test/benchmark on the target Windows CI runner rather than assuming either the old or new behavior.
**Warning signs:** Any new `winmm`/`timeBeginPeriod`/`NtSetTimerResolution` syscall wrapper appearing in a PR for this phase should be treated as a red flag requiring justification — it is very likely solving an already-solved problem and may even degrade performance under the current runtime [CITED: github.com/Microsoft/go-winio issue #69 — "forcing 1ms timer resolution... degrades performance under go1.9+"].

### Pitfall 3: Losing Determinism by Persisting Session-Relative State
**What goes wrong:** Serializing the engine's `loopStart` epoch, a chase's "steps elapsed," or any other time-anchored value directly into `show.State`/the `.golc` file, then expecting playback to resume exactly where it left off after a restart.
**Why it happens:** Feels natural to "save what's playing," especially since D-14 already establishes undo history is session-only — it's easy to assume playback position should be the opposite (durable).
**How to avoid:** Persist only the *authoring* objects (scenes, layers, chases, BPM, bars-per-loop) — never a live epoch or elapsed-tick count. On engine start (including after a restart), always establish a fresh `loopStart = time.Now()` and begin the configured scene at bar 0, consistent with SCEN-08's restart semantics. This keeps the deterministic-harness guarantee (SCEN-09: same time-indexed results) meaningful — "time-indexed" means indexed from a fresh, declared epoch, not from wall-clock history.
**Warning signs:** A test that saves a show mid-bar, reloads it, and asserts the exact prior bar/beat position resumed — this is testing the wrong invariant; the correct invariant is "restart begins deterministically at bar 0 (or the author's configured start point), every time."

## Code Examples

Verified/derived patterns for this phase's core mechanics (see also Patterns 1–3 above, which contain the primary reference snippets):

### Next-Bar Adoption With Reject-on-Invalid-Compile (D-05/D-06)
```go
// Source: derived directly from 03-CONTEXT.md D-05/D-06 (locked decisions)
func (e *Engine) StageEdit(state show.State) error {
    plan, err := Compile(state) // whole-plan compile; validates every reference
    if err != nil {
        // D-06: reject and keep running the last valid compiled version.
        // The engine's active plan is never touched here.
        return fmt.Errorf("GOLC_PLAYBACK_PLAN_INVALID: %w", err)
    }
    e.pendingPlan.Store(&plan) // adopted atomically at the next bar boundary by tick()
    return nil
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| Windows Go timers require manual `timeBeginPeriod(1)`/winmm workaround for sub-15ms resolution | Go runtime natively provides ~0.5ms Windows timer resolution via `NtAssociateWaitCompletionPacket`+IOCP | Go 1.23 (released ~Aug 2024) | Removes the need for any manual Windows timer-resolution code in the engine's tick loop; do not port pre-1.23 guidance into this codebase [CITED: devblogs.microsoft.com/go/high-resolution-timers-windows] |
| Go pre-1.16 also had high-res Windows timers | Removed in 1.16 due to scheduler conflicts, reintroduced correctly in 1.23 via IOCP integration | Go 1.16 (removal) → Go 1.23 (correct reintroduction) | Historical detail explaining why some older Go/Windows advice contradicts current behavior — a search on this topic will surface both eras' guidance; only Go 1.23+ behavior applies to this repo's pinned 1.26.5 toolchain |

**Deprecated/outdated:**
- Manual `timeBeginPeriod`/winmm-based Windows timer-resolution forcing: unnecessary and potentially performance-degrading on Go 1.23+ [CITED: github.com/Microsoft/go-winio issue #69].

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | A 4/4 (`beatsPerBar = 4`) fixed time signature is assumed for the `Position` formula example | Architecture Patterns → Pattern 1 | If the project intends variable/configurable time signatures, the formula needs a signature parameter — check REQUIREMENTS.md/CONTEXT.md wording ("musical bars") doesn't specify; low risk since SCEN-01 only says "configured number of musical bars," not beats-per-bar, and Phase 3's CONTEXT.md never raises this as a discussed decision — planner should confirm or explicitly lock 4/4 as a v1 simplification |
| A2 | A ~30–40Hz engine tick cadence is the right target for this project (based on general DMX/Art-Net industry-standard refresh rates, not on any GOLC-specific benchmark or user requirement) | Summary; Common Pitfalls; Standard Stack | If the actual required output cadence differs (e.g., Phase 4's Art-Net work later demands a different fixed rate for protocol reasons), the engine's tick interval is a one-line constant change, not an architecture change — low risk, but the exact number should be confirmed against Phase 4 needs before being hard-coded as a magic constant |
| A3 | No suitable third-party Go "musical clock" library exists that fits this project's bar-based-loop-position-as-pure-function need better than ~30 lines of stdlib `time` arithmetic | Standard Stack → Alternatives Considered | Low risk — this is a judgment call favoring simplicity/dependency-minimization over an unresearched alternative; if wrong, worst case is a missed opportunity to reuse tested code, not a correctness bug |
| A4 | Chase step index and motion-preset phase are computed as pure functions of elapsed musical time (no independent stateful "current step" field) | Architecture Patterns → Pattern 2 | If a future requirement needs a chase to remember state across a scene deactivation/reactivation (e.g., "resume from step 3"), this pure-function model would need an explicit anchor-point concept added — D-09 (no randomization) and D-11 (chases follow the scene's SCEN-08 rule) are consistent with the pure-function model, so risk is low, but this wasn't explicitly confirmed in 03-CONTEXT.md |

**If this table is empty:** N/A — see entries above. All Go/Windows timing mechanics claims are CITED against official sources (go.dev, Microsoft's official Go devblog, github.com/Microsoft/go-winio); the assumptions above are engineering judgment calls applied to this project's specific scale and requirements, not unverified factual claims about external systems.

## Open Questions

1. **Exact engine tick rate (Hz)**
   - What we know: DMX/Art-Net industry practice commonly runs 30–44Hz; some consoles lock a unified ~30–40Hz rate across their whole pipeline [LOW confidence, web-search-only].
   - What's unclear: Whether GOLC should pick a specific fixed rate now (Phase 3) or defer the exact number to Phase 4 (Art-Net), since Phase 3's engine only needs to be "fast enough that no human-perceptible stepping occurs" and doesn't itself emit Art-Net packets.
   - Recommendation: Planner should pick a concrete constant (e.g. 40Hz / 25ms) now so `internal/playback`'s tick loop and tests have a fixed target, documented as adjustable — Phase 4 can tune it later without an architecture change since it's a single constant, not a structural dependency.

2. **Time signature (beats per bar)**
   - What we know: SCEN-01 says scenes "loop for a configured number of musical bars against the global BPM" — bars, not beats, are the configured unit.
   - What's unclear: Whether beats-per-bar is always 4/4 in v1, or configurable per show/scene.
   - Recommendation: Treat as a v1 simplification (fixed 4/4) unless the planner finds contrary evidence in REQUIREMENTS.md/PROJECT.md — flagged as Assumption A1 above; cheap to leave as a named constant either way.

3. **Whether chase/motion "step duration" is bar-relative or beat-relative**
   - What we know: PROG-05 says chases have "tempo-relative timing"; D-10 says chase step advancement is driven by the same global BPM+bar-position clock as scene looping.
   - What's unclear: Whether an individual chase step's duration is authored in bars, beats, or a fraction thereof (e.g., "this chase has 8 steps across 1 bar" vs. "each step is 1 beat").
   - Recommendation: This is a data-modeling decision for the planner/author-facing PROG-05 task, not a research gap — the engine architecture in this document (pure position function + per-layer `Resolve(pos)`) accommodates either unit without changing the tick-loop or adoption-boundary design.

## Environment Availability

Skipped — this phase adds headless domain/engine code only, with no new external tool, service, or runtime dependency beyond the already-bootstrapped, already-verified Go 1.26.5 toolchain (`config/toolchain.toml`, confirmed present via Phase 1's bootstrap). No database, no network service, no OS-level feature beyond the stdlib `time`/`sync/atomic` packages already exercised in prior phases.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing` (stdlib), via the project's own `test` command route |
| Config file | none — `go test` driven by `_test.go` files; project-local scope markers follow the existing `TestScope{PascalName}` convention (`internal/command/test.go`) |
| Quick run command | `./golc.ps1 test --quick --scope <scope-name>` (per-package/scope quick gate) |
| Full suite command | `./golc.ps1 test` (every project Go package's tests, plus registered Node scopes) |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| PROG-01 | Resolve pool/group/deployment-instance/direct-fixture selection into a fixture-instance set | unit | `go test ./internal/programming/... -run TestSelection` | ❌ Wave 0 |
| PROG-02 | Edit semantic intensity/color/position/beam attributes without raw DMX | unit | `go test ./internal/programming/... -run TestProgrammer` | ❌ Wave 0 |
| PROG-03 | Programmer surfaces touched attrs, values, sources, record scope | unit | `go test ./internal/programming/... -run TestProgrammerInspect` | ❌ Wave 0 |
| PROG-04 | Create/reuse themes and intensity/color/position/beam presets | unit | `go test ./internal/programming/... -run TestThemePreset` | ❌ Wave 0 |
| PROG-05 | Create reusable chases with ordered steps + tempo-relative timing | unit | `go test ./internal/programming/... -run TestChase` | ❌ Wave 0 |
| PROG-06 | Create reusable motion presets (position/beam only, per D-04) | unit | `go test ./internal/programming/... -run TestMotionPreset` | ❌ Wave 0 |
| PROG-07 | Record/update/rename/reorder/duplicate/delete with undo/redo (D-12/13) | unit | `go test ./internal/programming/... -run TestHistory` | ❌ Wave 0 |
| SCEN-01 | Scene loops for configured bar count against global BPM | unit | `go test ./internal/playback/... -run TestClockPosition` | ❌ Wave 0 |
| SCEN-02 | Numeric BPM entry | unit | `go test ./internal/playback/... -run TestBPMSet` | ❌ Wave 0 |
| SCEN-03 | Tap-tempo BPM | unit | `go test ./internal/playback/... -run TestTapTempo` | ❌ Wave 0 |
| SCEN-04 | Exactly one active scene | unit | `go test ./internal/scene/... -run TestSingleActiveScene` | ❌ Wave 0 |
| SCEN-05 | Scene combines independently enabled layers | unit | `go test ./internal/scene/... -run TestLayerCombination` | ❌ Wave 0 |
| SCEN-06 | Immediate scene/layer switch | unit | `go test ./internal/playback/... -run TestImmediateSwitch` | ❌ Wave 0 |
| SCEN-07 | Reusable blending presets | unit | `go test ./internal/scene/... -run TestBlendPreset` | ❌ Wave 0 |
| SCEN-08 | BPM-change preserve-position-or-restart | unit | `go test ./internal/playback/... -run TestBPMChangeEpoch` | ❌ Wave 0 |
| SCEN-09 | Deterministic time-indexed output under simulated adapter delay/failure | unit + property | `go test ./internal/playback/... -run TestDeterministicEvaluate` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `./golc.ps1 test --quick --scope <touched-scope>`
- **Per wave merge:** `./golc.ps1 test`
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `internal/programming/*_test.go` — new package, no existing tests
- [ ] `internal/scene/*_test.go` — new package, no existing tests
- [ ] `internal/playback/*_test.go` — new package, no existing tests; should include a property-style test that feeds `Evaluate` the same `(plan, position)` pair many times (and via multiple goroutines) and asserts byte-identical `Frame` output — the direct mechanical proof of SCEN-09
- [ ] Framework install: none — Go `testing` is already in use project-wide, no new framework needed

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-------------------|
| V2 Authentication | no | No auth surface exists yet — headless engine, no network/API boundary until Phase 7 |
| V3 Session Management | no | Same as above |
| V4 Access Control | no | Same as above |
| V5 Input Validation | yes | Extend `show.validate()` (whole-document pattern) to check: attribute values against `fixture.Capability.Range` (reuse Phase 2, don't re-derive bounds), BPM within a sane positive range, chase-step/bar-loop counts against declared ceilings, and every scene→layer→theme/preset/chase/motion-preset reference resolves (mirrors `pool.ValidateGroupReferences`) |
| V6 Cryptography | no | Not applicable — no secrets, no crypto operations in this phase |

### Known Threat Patterns for This Stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|----------------------|
| Hand-edited/malformed `show.State` introducing a dangling scene→chase/theme/preset/motion-preset reference | Tampering | Extend `show.validate()`'s whole-document referential-integrity check, same pattern as `pool.ValidateGroupReferences` — reject at Load/Save time with a `GOLC_SHOW_STATE_INVALID`-style diagnostic, never silently drop or null the reference |
| Pathologically large chase step count / bar-loop count causing excessive memory or slow compile | Denial of Service | Declare and enforce explicit ceilings during validation (mirrors `internal/deployment`'s `maxUniverseSearch` precedent) — reject with a clear diagnostic rather than allowing unbounded growth |
| A partially-valid compiled plan reaching the live engine mid-promotion | Tampering / integrity | D-06's reject-and-keep-last-valid mechanism: `Compile` is all-or-nothing: any single unresolved reference or invalid value fails the whole compile, the engine's `pendingPlan` is never set to a partial result |
| An adapter (UI/persistence/script/API/LLM) stalling or blocking the engine's tick loop | Denial of Service (against playback itself) | Structural isolation: `internal/playback` never imports adapter packages; all engine reads are `atomic.Pointer.Load()` (non-blocking); all engine writes into `pendingPlan` are non-blocking swaps, never a call the tick loop awaits |

## Sources

### Primary (HIGH confidence)
- `internal/show/state.go`, `internal/pool/model.go`, `internal/deployment/model.go`, `internal/fixture/model.go`, `internal/command/router.go`, `internal/command/test.go`, `config/toolchain.toml`, `config/commands.toml`, `go.mod` — direct repo inspection [VERIFIED: read directly this session]
- `.planning/phases/03-deterministic-show-programming-and-playback/03-CONTEXT.md` — locked decisions D-01 through D-14 [VERIFIED: read directly this session]
- `.planning/REQUIREMENTS.md`, `.planning/ROADMAP.md`, `.planning/PROJECT.md`, `.planning/STATE.md` — project requirements and constraints [VERIFIED: read directly this session]

### Secondary (MEDIUM confidence)
- [High-Resolution Timers on Windows | Microsoft for Go Developers](https://devblogs.microsoft.com/go/high-resolution-timers-windows/) — Go 1.23 Windows timer resolution change (~15.6ms → ~0.5ms), `NtAssociateWaitCompletionPacket`/IOCP mechanism, requires Windows 10+ [CITED: official Microsoft Go team blog, cross-checked via WebFetch]
- [Go Wiki: Go 1.23 Timer Channel Changes](https://go.dev/wiki/Go123Timer) — official Go documentation on Go 1.23 timer semantics [CITED: go.dev]
- [go-winio Issue #69](https://github.com/Microsoft/go-winio/issues/69) — confirms manual `timeBeginPeriod(1)` forcing is unnecessary and degrades performance under recent Go [CITED: official Microsoft repo issue tracker]
- [pkg.go.dev/sync/atomic](https://pkg.go.dev/sync/atomic) — `atomic.Pointer[T]`/`atomic.Value` semantics for lock-free publish [CITED: official Go package documentation]

### Tertiary (LOW confidence)
- DMX/Art-Net 30–44Hz refresh-rate figures — general web search across lighting-console forums (Wolfmix, garageCube, MA Lighting, The Lighting Controller Classic, open-lighting Google Group), not a single authoritative spec citation; treat as informed community consensus, not a hard standard [LOW confidence, marked for validation against DMX512/Art-Net 4 spec text if an exact number becomes load-bearing]
- Fixed-timestep game-loop pattern (accumulator, catch-up semantics) — general web search (gameprogrammingpatterns.com and similar), included for the general design-pattern shape only; this project's actual model (pure position function, not an accumulator) diverges from the classic game-loop pattern for good domain-specific reasons explained in Architecture Patterns [LOW confidence, used only as background, not as the recommended implementation]
- Go `runtime.LockOSThread`/GOMAXPROCS/GC-pause generalities — general web search, background context only; no specific benchmark numbers for this project's workload were found or claimed [LOW confidence]

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new dependencies; all recommended stdlib APIs verified against official Go documentation and this repo's own `go.mod`/toolchain pin
- Architecture: MEDIUM — the pure-position-function design and package-boundary recommendations are reasoned engineering judgment consistent with this repo's established patterns (Phase 1/2 precedent) and the project's own locked decisions (D-01–D-14), not independently benchmarked against a reference GOLC-like implementation
- Pitfalls: MEDIUM — Windows timer-resolution pitfall is CITED against official sources; the other pitfalls (bar-boundary equality check, persisted-epoch determinism loss) are derived directly from the architecture this document proposes, not observed in an external system

**Research date:** 2026-07-21
**Valid until:** 2026-08-20 (30 days — Go toolchain/runtime timing behavior is stable; re-verify if the pinned Go version changes before Phase 3 planning begins)
