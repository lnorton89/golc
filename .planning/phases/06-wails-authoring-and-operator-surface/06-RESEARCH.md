# Phase 6: Wails Authoring and Operator Surface - Research

**Researched:** 2026-07-23
**Domain:** Go desktop GUI (Wails v2), generic MIDI Note/CC input, local-priority safety-control architecture
**Confidence:** MEDIUM

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Operator Surface Builder**
- **D-01:** A show author builds each constrained operator surface by **assigning directly from the full authoring view** — toggling "add to this operator surface" on scenes/layers/masters/safety controls in place (checkbox or right-click), not through a separate dedicated builder screen or a drag-and-drop canvas. No new layout-design screen needs to be built.
- **D-02:** A show can define **multiple named operator surfaces** (e.g. different surfaces per venue or per operator), not just one.
- **D-03:** Assignment granularity is **individual items only** — an author picks specific scenes, layers, masters, and safety controls one at a time. There is no group/category-level bulk-assign shortcut in this phase.
- **D-04:** Anything not assigned to a given operator surface is **visible but locked** (shown grayed out/disabled), not hidden entirely. "Constrained" is enforced by interaction, not by visibility.

**MIDI Learn & Conflicts**
- **D-05:** MIDI learn is initiated **per-control** — each mappable control has its own small "Learn" affordance. There is no global "MIDI Learn mode" toggle and no MIDI-activity-monitor-and-assign panel in this phase.
- **D-06:** Mapping a MIDI Note/CC that's already assigned to a different control is **blocked until the existing mapping is explicitly removed** — rejected outright, never silently overwritten or confirm-to-reassign.
- **D-07:** MIDI mappings are **per operator surface**, not global to the show. Each named surface (D-02) carries its own independent MIDI mapping set.
- **D-08:** Any control assigned to a given operator surface is automatically MIDI-learnable — **there is no separate fixed list of MIDI-mappable commands independent of what's on the surface.**

**Soft Takeover Feedback**
- **D-09:** While a physical fader hasn't caught up to the app's current value, the **on-screen slider visually follows the physical fader's live position in real time**, shown in a distinct "not armed"/pickup visual state.
- **D-10:** A **ghost/target marker shows the app's actual current value** as a fixed reference point the physical fader must reach.
- **D-11:** The catch-up mechanic is **cross-to-catch (standard soft pickup)**: the physical fader's value must cross/pass through the app's current value before it takes control. Proximity-threshold takeover was not chosen.
- **D-12:** Soft takeover logic applies **only to continuous CC/fader controls**, not to Note/button controls. Buttons act immediately on press with no pickup/arming state.

**Safety Control Placement (Blackout / Revoke Automation / Stop-Release-All)**
- **D-13:** Blackout, Revoke Automation, and Stop/Release-All live in a **persistent global bar present on every screen** — authoring, programming, and playback views alike.
- **D-14:** Activation uses **hold-to-confirm** (press and hold roughly 500ms-1s) rather than a single immediate click or a two-step arm-then-confirm flow.
- **D-15:** The three controls are grouped into a **dedicated, visually distinct safety cluster** (e.g. a red-bordered panel) that stays in the same screen position at all times.
- **D-16:** Each safety control gets a **fixed, global, unmodifiable default keyboard shortcut** that works regardless of on-screen focus. Shortcuts are not user-rebindable in this phase.

### Claude's Discretion
None — every gray area discussed converged on an explicit user selection; no "you decide" selections were made in this session.

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope. All four discussed areas (operator surface builder, MIDI learn & conflicts, soft takeover feedback, safety control placement) were clarifications of how to implement what's already in PLAY-01 through PLAY-09.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| PLAY-01 | An operator can access the complete playback workflow through on-screen controls. | Architecture Pattern (Wails webview + `internal/command` bindings); Standard Stack (React/Zustand per locked STACK.md); Validation Architecture test map. |
| PLAY-02 | An operator can access the complete playback workflow through documented keyboard controls. | Architectural Responsibility Map (in-webview keyboard handling, distinct from D-16's OS-level path); Pitfall 4 (don't over-extend global hotkeys to this requirement). |
| PLAY-03 | A show author can create a constrained operator surface that exposes only assigned scenes, layers, masters, and safety controls. | Architectural Responsibility Map; Recommended Project Structure (`internal/operatorsurface`); Security Domain V4 Access Control (server-side enforcement of visible-but-locked). |
| PLAY-04 | A show author can map generic MIDI Note and Control Change input to supported playback commands. | Standard Stack (`gitlab.com/gomidi/midi/v2` + `midicatdrv`); Architecture Pattern 3 (bounded MIDI learn capture session); Package Legitimacy Audit. |
| PLAY-05 | MIDI fader mappings support soft takeover so connecting or moving a controller does not cause unintended value jumps. | Architecture Pattern 4 (cross-to-catch soft-takeover state machine); Common Pitfall 2 (proximity-vs-crossing). |
| PLAY-06 | An operator can control group masters, a Grand Master, stop/release-all, and an immediate blackout. | Architecture Pattern 1 (daemon-resident local-priority safety override); System Architecture Diagram. |
| PLAY-07 | An operator can see the active scene, enabled layers, current BPM/bar position, controlling source, and final output state. | Architectural Responsibility Map (throttled `EventsEmit` status push read via `Engine.CurrentFrame()`/daemon status, reusing Phase 4's watch-view data path); Open Question 3 (throttling cadence). |
| PLAY-08 | Revoke Automation immediately blocks AI and scripts, cancels their queued actions, freezes the current look, and returns control to manual operation without requiring those runtimes to respond. | Architecture Pattern 1; Open Question 2 (forward-looking interface for Phases 8/9); Validation Architecture (`TestRevokeAutomation`). |
| PLAY-09 | Blackout remains a separate local priority control that does not depend on the UI, script runtime, API, or LLM provider completing work. | Architecture Pattern 1 and 2 (daemon-resident flag + OS-level hotkey independent of the webview/JS thread); System Architecture Diagram trace. |
</phase_requirements>

## Summary

Phase 6 wraps the already-complete typed `internal/command` registry (Phases 1-5) in a Wails v2 desktop shell and adds two genuinely new subsystems: generic MIDI Note/CC learn with soft takeover, and a local-priority safety cluster (Blackout / Revoke Automation / Stop-Release-All) that must survive a hung webview, script runtime, API, or LLM. Phase 4 already locked the Wails app's relationship to the live core: the Art-Net worker and playback engine run inside one long-lived, standalone-capable daemon process (`internal/artnet/daemon.go`, started via `golc-project.exe artnet serve`) that every control surface — including CLI, and now Wails — attaches to as an IPC client over a Windows named pipe (`internal/artnet/ipc`, `github.com/Microsoft/go-winio`). Phase 6 does not reopen that decision; it adds one more client and, new to this phase, adds a small set of **daemon-resident, in-memory** safety-override primitives (blackout/revoke-automation/stop-all flags) that the Worker checks every tick, so the override survives regardless of what the Wails frontend, scripts, the API, or an LLM are doing.

The project's own prior research already locked Wails v2.13.0 (not v3, which remains alpha) and a React/Zustand frontend in `.planning/research/STACK.md` — this research treats that as a locked decision, not an open question, and focuses on what STACK.md did not yet resolve: the MIDI library/driver choice, the soft-takeover algorithm, and — the phase's hardest architectural problem — how a "fixed, global, unmodifiable... regardless of on-screen focus" keyboard shortcut (D-16) is even possible in Wails v2, which has no native global-shortcut API. The answer is `golang.design/x/hotkey`, a pure-Go (no CGo on Windows) library that registers a true OS-level hotkey via `RegisterHotKey`, run from the Wails Go host process (not the webview/JS thread), so the shortcut keeps firing even if the frontend's render/JS thread is completely stalled.

**Primary recommendation:** Keep the Wails app a thin IPC client of the existing daemon (per Phase 4 D-03/D-04); implement Blackout/Revoke Automation/Stop-Release-All as new atomic in-memory flags on the daemon (not show-state mutations, not SQLite writes) set via a dedicated low-latency IPC route; drive keyboard triggers for those three controls through `golang.design/x/hotkey` registered by the Go host process, not JS keydown listeners; read MIDI Note/CC input directly in the Wails Go process via `gitlab.com/gomidi/midi/v2` with the `midicatdrv` driver (avoids CGo in the main binary, consistent with the project's CGo-free toolchain policy); and implement soft takeover as a direction-aware value-crossing check, not proximity/equality.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| On-screen/keyboard playback controls (PLAY-01/02) | Wails webview (frontend) | Wails Go host (bindings) | Standard authoring/operator UI; issues typed commands through the existing `internal/command` bridge — no new authority. |
| Operator surface builder & assignment (PLAY-03, D-01..D-04) | Wails webview (frontend) | `internal/show` (persistence) | Authoring-time configuration; belongs in `show.State` like every other Phase 2/3/5 authoring object. |
| MIDI Note/CC input capture (PLAY-04) | Wails Go host process | — | Physical MIDI device I/O must be native Go (gomidi driver); JS/webview has no MIDI device access suitable for this project's offline desktop model. |
| MIDI learn & mapping persistence (D-05..D-08) | Wails Go host (learn logic) | `internal/show` (mapping storage) | Learn is a live capture interaction (Go-side), but the resulting mapping is authoring data persisted per operator surface. |
| Soft takeover crossing logic (D-09..D-12) | Wails Go host process | Wails webview (visual feedback only) | The crossing/armed state is authoritative control-arbitration logic; the frontend only renders the two visual layers (live slider + ghost marker) it's told about. |
| Safety cluster execution: Blackout / Revoke Automation / Stop-Release-All (PLAY-06/08/09, D-13..D-16) | Long-lived daemon process (`internal/artnet` / playback engine) | Wails Go host (hotkey/MIDI trigger source) | Must not depend on UI, script, API, or LLM completing work — the enforcement point has to be the same process that already owns the Art-Net Worker's per-tick output, not a round trip through show persistence. |
| Keyboard shortcut registration for the safety cluster (D-16) | Wails Go host process (native, OS-level) | — | Must fire "regardless of on-screen focus," including a stalled webview — only an OS-level global hotkey registered outside the JS event loop satisfies this. |
| Live status display: active scene, layers, BPM/bar, controlling source, output state (PLAY-07) | Wails Go host (event push) | Wails webview (render) | Reads `Engine.CurrentFrame()`/daemon status the same lock-free/IPC path Phase 4's CLI watch view already uses; Go host throttles before `EventsEmit`. |
| Show/deployment/programming/scene persistence | `internal/show` (SQLite via Phase 5) | — | Unchanged from Phase 5; Phase 6 adds new fields to the same single-document model, not a parallel store. |

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/wailsapp/wails/v2` | v2.13.0 (locked by `.planning/research/STACK.md`) `[VERIFIED: proxy.golang.org, latest v2 tag matches]` | Desktop shell, Go<->JS binding, event bridge | Already the project's locked stack decision; v3 is explicitly excluded as alpha in STACK.md's "What NOT to Use." |
| `gitlab.com/gomidi/midi/v2` | v2.3.24 (2026-06-15) `[VERIFIED: proxy.golang.org @latest]` | MIDI message types (Note on/off, Control Change) and driver abstraction | Actively maintained, the de facto standard Go MIDI library; typed Note/CC helpers map directly onto D-05/D-08's per-control learn model. `[ASSUMED: package name/quality assessed via WebSearch + registry lookup, not an official docs endorsement — confirm during planning that the module resolves and builds cleanly in this repo's CI before locking.]` |
| `gitlab.com/gomidi/midi/v2/drivers/midicatdrv` | same module/version as above | CGo-free MIDI I/O driver for the main `golc-project.exe` binary | Shells out to a separately-built `midicat` helper binary over stdin/stdout; keeps the main binary CGo-free, matching this repo's existing pure-Go toolchain policy (`modernc.org/sqlite`, no C compiler pinned in `config/toolchain.toml`). `[ASSUMED — see Package Legitimacy Audit]` |
| `golang.design/x/hotkey` | v0.6.1 (2026-06-06) `[VERIFIED: proxy.golang.org @latest]` | OS-level global keyboard shortcut registration (D-16) | Only viable path to a true global hotkey in Wails v2 (which has no native global-shortcut API — that only shipped in Wails v3 alpha). Windows implementation (`hotkey_windows.go`) is pure Go: no CGo, calls `RegisterHotKey` via an internal `win` syscall wrapper. `[ASSUMED — see Package Legitimacy Audit]` |
| `github.com/Microsoft/go-winio` | v0.6.2 (already in `go.mod`) `[VERIFIED: existing dependency, used by internal/artnet/ipc]` | Named-pipe IPC client from the Wails Go host to the daemon | Already the established GOLC pattern (Phase 4); reuse `internal/artnet/ipc.Dial`/`Forward` directly, do not reinvent. |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| Frontend framework (React 19.2.7 + Zustand 5.0.14, per STACK.md) | as locked in STACK.md | Wails webview UI | STACK.md already evaluated Svelte vs. React for this project and chose React for its editor/testing/accessibility ecosystem fit for "a dense application" — Phase 6 should follow that decision, not re-litigate it. Note Wails' own maintainers favor Svelte as a first choice for new small apps generally; this is a documented alternative, not a reason to deviate from the already-locked project decision. |
| `gitlab.com/gomidi/midi/v2/drivers/rtmididrv` or `portmididrv` | n/a | Alternative CGo-based MIDI driver | Only if `midicatdrv` proves unworkable in practice (e.g., helper-binary packaging friction) — both require a CGo-capable Windows toolchain this project does not currently pin. |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Wails v2 | Wails v3 (alpha) | v3 ships a native `app.GlobalShortcut`, removing the need for `golang.design/x/hotkey` — but v3 is explicitly excluded by the project's own locked STACK.md as unacceptable prerelease risk for a live controller. Not a live option for Phase 6. |
| `midicatdrv` (external helper binary) | `rtmididrv`/`portmididrv` (CGo, in-process) | Simpler single-binary build if a CGo toolchain were already required elsewhere in the project — but this project deliberately has none (pure-Go SQLite driver, no C compiler pinned), so introducing one just for MIDI would be a toolchain regression. |
| `golang.design/x/hotkey` (OS-level) | In-webview JS `keydown` capture-phase listener | Simpler (no new Go dependency), and sufficient if D-16's "regardless of on-screen focus" is read narrowly as "regardless of which input field has focus within the app." Does **not** satisfy "remains responsive even if...the Wails frontend...is hung" (orchestrator note 5) — a stalled JS thread cannot dispatch a `keydown` handler. Recommend the OS-level approach as primary; JS listener as a documented fallback binding only. |
| Daemon-resident atomic safety flags | Route Blackout/Revoke/Stop through `internal/command` -> `show.Load`/`show.Save` (SQLite) | The existing show-mutation path is disk I/O (Phase 5 SQLite write) — orders of magnitude too slow and too failure-prone for a "local priority path... independent of UI/script/API/model completion." This is exactly the "storage remains outside the deterministic playback path" anti-pattern the project has guarded against since Phase 1. |

**Installation:**
```bash
go get github.com/wailsapp/wails/v2@v2.13.0
go get gitlab.com/gomidi/midi/v2@v2.3.24
go get golang.design/x/hotkey@v0.6.1
# go-winio is already a go.mod dependency; no new install needed.
```

**Version verification:** Versions above were confirmed live against `proxy.golang.org/<module>/@latest` on 2026-07-23 (see command output captured during this research session). `gomidi/midi/v2` and `golang.design/x/hotkey` were discovered via WebSearch/training knowledge, not an official docs endorsement — their existence on the Go module proxy is confirmed, but this does not by itself establish they are the *correct* or safest choice; treat as `[ASSUMED]` per the Package Legitimacy Audit below until a `checkpoint:human-verify` review during planning/execution.

## Package Legitimacy Audit

> Go modules are not covered by the `gsd-tools package-legitimacy check` seam (npm/PyPI/crates only). Verification below was performed manually against the Go module proxy (`proxy.golang.org`) — the ecosystem-correct registry equivalent to `npm view`/`pip index versions` — plus source-repository inspection.

| Package | Registry | Age/Activity | Source Repo | Verdict | Disposition |
|---------|----------|-----|-------------|---------|-------------|
| `github.com/wailsapp/wails/v2` | Go module proxy | Mature (v2.13.0, actively released; already locked in project STACK.md) | github.com/wailsapp/wails, ~28k+ stars, large community | OK | Approved (pre-existing project decision, not newly introduced by this research) |
| `gitlab.com/gomidi/midi/v2` | Go module proxy | Active; latest tag 2026-06-15 per proxy | gitlab.com/gomidi/midi (mirrored to github.com/gomidi/midi); long-running project, MIT license, multiple driver sub-packages actively maintained | OK, with caveat | Approved — `[ASSUMED]`, planner must add `checkpoint:human-verify` before first `go get` to confirm module resolves cleanly against this repo's Go 1.26.5 toolchain and GOPROXY policy |
| `gitlab.com/gomidi/midi/v2/drivers/midicatdrv` + `midicat` helper binary | Go module proxy (driver) / GitLab releases (binary) | Same repo/maintainer as above | Same as above | SUS | Flagged — the driver is a thin Go wrapper, but the actual MIDI I/O happens in a **separately distributed prebuilt binary** fetched outside `go.mod`/`go.sum`'s checksum protection; planner must add `checkpoint:human-verify` to pin and hash-verify the exact `midicat` binary the same way `config/toolchain.toml` pins Go/Node archives (SHA-256 + fixed official URL), not a floating "latest" download |
| `golang.design/x/hotkey` | Go module proxy | Active; latest tag 2026-06-06 | github.com/golang-design/hotkey, small but focused single-purpose library, MIT license, used in a real-world Wails integration referenced in `wailsapp/wails` discussion #2320 | OK, with caveat | Approved — `[ASSUMED]`, planner must add `checkpoint:human-verify` given this package's small size/maintainer surface controls a security-critical (D-16 safety-cluster) code path |
| `github.com/Microsoft/go-winio` | Go module proxy | Already a pinned `go.mod` dependency at v0.6.2, official Microsoft-maintained repo | github.com/microsoft/go-winio | OK | Approved — already in use, no new risk introduced |

**Packages removed due to `[SLOP]` verdict:** none
**Packages flagged as suspicious `[SUS]`:** `midicatdrv`/`midicat` helper binary — external binary distribution outside Go's module checksum database requires the same pin-and-hash discipline this project already applies to Go/Node in `config/toolchain.toml`.

*All four newly-introduced packages above were discovered via WebSearch and training knowledge, not an official-docs recommendation; they are tagged `[ASSUMED]` and each gates a `checkpoint:human-verify` task per the Package Legitimacy Protocol.*

## Architecture Patterns

### System Architecture Diagram

```
                     ┌─────────────────────────────────────────────────────────┐
                     │  Wails desktop process (per-user launch)                 │
                     │                                                          │
  keyboard  ───────▶ │  golang.design/x/hotkey (OS-level, RegisterHotKey)      │
  (D-16 safety keys)  │        │ fires even if webview JS thread is hung        │
                     │        ▼                                                │
  MIDI device ─────▶ │  gomidi/midi + midicatdrv (Note/CC input)               │
  (learn + play)      │        │                                                │
                     │        ▼                                                │
                     │  Go host: command dispatch / MIDI-learn / soft-takeover │
                     │  arbitration / operator-surface authorization           │
                     │        │                     ▲                          │
                     │        │ EventsEmit (throttled)│ Wails-generated bindings│
                     │        ▼                     │                          │
                     │  Webview (React) ─────────────┘ on-screen controls,     │
                     │                                  operator surface UI     │
                     └───────┬─────────────────────────────────────────────────┘
                             │ named-pipe IPC (go-winio), reused from Phase 4
                             │  - normal typed commands (scene/layer/master/...)
                             │  - NEW: low-latency safety-override route
                             │    (blackout / revoke-automation / stop-all)
                             ▼
                  ┌───────────────────────────────────────────┐
                  │  Long-lived daemon process (unchanged      │
                  │  entrypoint: golc-project.exe artnet serve)│
                  │                                             │
                  │  playback.Engine (atomic.Pointer[Frame])   │
                  │        │                                    │
                  │  NEW: atomic safety-override flags          │
                  │  (blackout, revokeAutomation, stopAll)      │
                  │        │  checked every tick, before output │
                  │        ▼                                    │
                  │  artnet.Worker  ──────▶  Art-Net UDP output │
                  └───────────────────────────────────────────┘
```

A reader tracing "operator presses the Blackout hotkey": keyboard -> `golang.design/x/hotkey` callback in the Wails Go host (no JS involved) -> `ipc.Dial`/`Forward` over the existing named pipe -> daemon sets an atomic `blackoutActive` flag -> the very next `Worker` tick (within one Art-Net frame period, ~25ms) outputs zero/blacked-out values regardless of what `Engine.CurrentFrame()` currently holds -- with no dependency on the webview, show persistence, scripts, API, or an LLM having done anything.

### Recommended Project Structure
```
frontend/                       # Wails webview (React, per STACK.md) -- projection only, no authority
  src/
    components/
      SafetyCluster/            # persistent global bar (D-13/D-15), hold-to-confirm (D-14)
      OperatorSurface/          # constrained surface renderer (D-01..D-04: visible-but-locked)
      MidiLearn/                # per-control Learn affordance (D-05)
      SoftTakeoverSlider/       # live-position slider + ghost/target marker (D-09/D-10)
    store/                      # Zustand: cache of Go-pushed snapshots, never authoritative
internal/
  wails/                        # thin façade bound to the webview -- no business rules
    app.go                      # lifecycle, daemon supervision (spawn/attach), hotkey registration
    events.go                   # throttled EventsEmit wrappers (status, MIDI feedback)
  midi/                         # NEW: greenfield package
    driver.go                   # gomidi/midi + midicatdrv setup, port open/close, reconnect
    learn.go                    # per-control learn session (D-05), conflict rejection (D-06)
    takeover.go                 # cross-to-catch soft-takeover state machine (D-09..D-12)
  operatorsurface/               # NEW: greenfield package (or extend internal/show directly)
    model.go                    # named surface (D-02), item assignment (D-01/D-03), MIDI map (D-07)
    validate.go                 # assignment/mapping invariants, following internal/show's validate() pattern
  artnet/
    daemon.go                   # EXTENDED: add safety-override atomic flags + new IPC routes
    safety.go                   # NEW: blackout/revoke-automation/stop-all state + Worker integration
cmd/golc-project/main.go        # unchanged entrypoint; Wails binary may be a distinct cmd/ target
```

### Pattern 1: Daemon-Resident Local-Priority Safety Override

**What:** Add small atomic state (`atomic.Bool` for blackout/stop-all, an `atomic.Pointer` or similar for revoke-automation's richer "blocked + frozen look" state) directly on the daemon struct in `internal/artnet/daemon.go`, checked by the `Worker` immediately before building each outbound packet.

**When to use:** Any control in the D-13/D-15 safety cluster (Blackout, Revoke Automation, Stop/Release-All) and the group/Grand Master paths in PLAY-06 that must act "through local priority paths that do not wait for UI, script, API, or model work to complete."

**Example (illustrative shape, following `daemon.go`'s existing mutex-guarded-state convention):**
```go
// Source: pattern extrapolated from internal/artnet/daemon.go's existing
// mu-guarded targets/worker fields and worker.go's tick-time frame read.
type daemon struct {
    // ...existing fields...
    safety safetyState // NEW
}

type safetyState struct {
    blackout atomic.Bool
    // revokeAutomation carries enough state to block queued script/AI
    // commands and record the frozen frame; exact shape is Phase 6/8/9
    // shared design, not finalized here.
    revokeAutomation atomic.Bool
}

// In Worker's per-tick send path (worker.go), before building the packet:
if d.safety.blackout.Load() {
    frame = blackoutFrame // zero all channels, bypass Engine.CurrentFrame()
}
```

### Pattern 2: OS-Level Global Hotkey Owned by the Go Host, Not JS

**What:** Register the three safety-cluster shortcuts via `golang.design/x/hotkey` from Wails' Go `OnStartup`, with the callback directly invoking the same low-latency IPC call the on-screen button uses -- never routing through a JS event handler.

**When to use:** D-16's fixed, unmodifiable, focus-independent shortcuts. Do not use this pattern for the general keyboard workflow (PLAY-02), which is ordinary in-webview `keydown` handling -- reserve the OS-level path for the safety cluster specifically, since global hotkeys are a scarce, potentially-conflicting OS resource and registering dozens of them is an anti-pattern.

### Pattern 3: MIDI Note/CC Learn as a Bounded Capture Session

**What:** Per-control Learn (D-05) opens a short-lived capture window in the Go MIDI listener: the next Note-on or CC message received is proposed as the mapping candidate. Before committing, check the candidate `(channel, type, number)` tuple against every other mapping already registered for the *current* operator surface (D-07 -- mappings are per-surface, not global) and reject with a clear diagnostic if it collides (D-06), leaving the existing mapping untouched.

**When to use:** Every MIDI-mappable control on a built operator surface (D-08 -- there is no separate fixed list; whatever is on the surface is learnable).

### Pattern 4: Cross-to-Catch Soft Takeover State Machine

**What:** For continuous CC/fader controls only (D-12 -- never Note/button controls), track three values per mapped control: `appValue` (authoritative, set by the app/other sources), `lastPhysical` (most recent raw CC value received), and `armed` (bool). On each incoming CC message: if `!armed`, check whether `lastPhysical` and the new value bracket (or land exactly on) `appValue` in the direction of travel -- i.e., the physical value has reached or crossed `appValue` -- and if so set `armed = true` and let this message's value become the new `appValue`. While `!armed`, the physical message is ignored for control purposes but its raw value is still published to the frontend (D-09: the on-screen slider follows the live physical position even while not armed) alongside the fixed `appValue` as the ghost/target marker (D-10).

**Example (illustrative, direction-aware crossing check):**
```go
// Source: derived from documented pickup/soft-takeover behavior
// (Ableton/DJ TechTools/Surge community documentation); no official
// MIDI 1.0 spec section defines "soft takeover" as a required behavior,
// so this is a synthesized-from-multiple-sources pattern, not a single
// authoritative citation.
func (t *TakeoverState) Update(physical float64) (armed bool, controlValue float64) {
    if !t.Armed {
        crossedUp := t.LastPhysical <= t.AppValue && physical >= t.AppValue
        crossedDown := t.LastPhysical >= t.AppValue && physical <= t.AppValue
        if crossedUp || crossedDown {
            t.Armed = true
        }
    }
    t.LastPhysical = physical
    if t.Armed {
        t.AppValue = physical
    }
    return t.Armed, t.AppValue
}
```

### Anti-Patterns to Avoid

- **Routing safety-cluster actions through `show.Load`/`show.Save`:** Reintroduces disk I/O and the SQLite write path into what must be a sub-frame-period response; violates the project's own "storage remains outside the deterministic playback path" constraint (Phase 5 D-01/D-02 and the project-wide "Live reliability" rule).
- **Treating Wails `EventsEmit` as the playback/status source of truth:** the project's own `ARCHITECTURE.md` (Pitfall 1) already documents this exact failure mode for Phase 6 generally -- events are change/telemetry hints; the frontend re-queries authoritative state on any detected gap.
- **Registering MIDI-learnable controls independent of the operator surface builder:** contradicts D-08 directly; there is no separate "MIDI-mappable command list" to maintain.
- **Using JS `keydown` as the *only* binding for the safety cluster:** satisfies a narrow reading of D-16 but not the phase's governing "does not depend on the UI...completing work" constraint if the webview itself is unresponsive.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| OS-level global hotkey on Windows | Custom `RegisterHotKey`/message-loop syscall wrapper | `golang.design/x/hotkey` | Correctly handles the Win32 message loop, modifier normalization, and unregister-on-exit cleanup; a hand-rolled wrapper duplicates real Win32 edge cases (conflicting registrations, WM_HOTKEY dispatch) for no benefit. |
| MIDI device enumeration/hot-plug/message parsing | Custom WinMM `midiInOpen`/`midiInStart` bindings | `gitlab.com/gomidi/midi/v2` + a driver | MIDI message parsing (running status, SysEx framing, 14-bit CC pairs) has enough edge cases that a mature library is clearly cheaper than re-deriving from the MIDI 1.0 spec; this is exactly the kind of "deceptively complex" protocol work the project's own `Don't Hand-Roll` philosophy targets (cf. the existing decision not to hand-roll the SQLite driver or the Art-Net codec's transport primitives). |
| Named-pipe IPC framing | Custom length-prefixed pipe protocol | Reuse `internal/artnet/ipc` verbatim | Already built, tested, and is the literal integration point Phase 4 D-04 names for Phase 6 -- building a second IPC mechanism would violate "one more client attaches to the same running instance." |

**Key insight:** every piece of genuinely new infrastructure this phase needs (global hotkeys, MIDI I/O) sits directly on top of a well-established OS primitive (`RegisterHotKey`, WinMM/MIDI) that a small number of mature, focused libraries already wrap correctly; the project's own precedent (pure-Go SQLite driver, no hand-rolled Art-Net library) argues for the same discipline here rather than a bespoke syscall layer.

## Common Pitfalls

### Pitfall 1: Treating the Wails Webview as Authoritative for Playback/Safety State
**What goes wrong:** A developer wires Blackout or a scene switch to run entirely inside a Wails-bound Go method invoked by JS, with no independent trigger path; a frozen render thread or a slow IPC round trip inside that same call path then silently delays or drops the safety action.
**Why it happens:** Wails' binding model makes "call a Go method from JS" feel like the natural single path for every action, including the ones this phase explicitly requires to be independent of that path.
**How to avoid:** The safety cluster gets two independent trigger sources into the same daemon-side override state: the on-screen button (via the normal Wails-bound IPC call) and the OS-level hotkey (via a separate `golang.design/x/hotkey` callback that also calls the IPC layer directly, not through any JS-mediated path).
**Warning signs:** The safety-cluster hotkey handler lives in the frontend's JS bundle instead of `internal/wails/app.go`'s `OnStartup`.

### Pitfall 2: MIDI Soft Takeover Using Proximity Instead of Crossing
**What goes wrong:** Implementing takeover as "arm when within N units of the target" (a threshold/proximity model) instead of true crossing. D-11 explicitly rejects this in favor of cross-to-catch.
**Why it happens:** Proximity is simpler to implement and is a real, named alternative takeover mode (used by some DJ software) -- easy to reach for without checking which mode was actually decided.
**How to avoid:** Implement the direction-aware crossing check in Pattern 4 above; never compare `abs(physical - appValue) < threshold`.
**Warning signs:** A takeover implementation that has a tunable "sensitivity" or "threshold" constant is very likely the wrong (proximity) mode.

### Pitfall 3: CGo Creeping Into the Main Binary via a MIDI Driver
**What goes wrong:** Choosing `rtmididrv` or `portmididrv` for convenience (in-process, no helper binary) silently sets `CGO_ENABLED=1` as a requirement for the whole `golc-project.exe` build, which this project's `config/toolchain.toml` bootstrap (Go-only, no C toolchain pinned) does not support reproducibly.
**Why it happens:** Both drivers are the first results in gomidi's own documentation and "just work" on a developer machine that happens to already have a C toolchain installed (e.g., from a different project), masking the dependency until CI or a clean-checkout build fails.
**How to avoid:** Use `midicatdrv` with a pinned, hash-verified `midicat` helper binary (matching `config/toolchain.toml`'s existing pin-by-SHA256 discipline for Go/Node), or explicitly add and document a CGo-capable toolchain requirement if the team decides the CGo drivers are worth it -- do not let the choice happen implicitly.
**Warning signs:** `go build` fails only on a clean CI runner or a fresh contributor machine, not on the original author's machine.

### Pitfall 4: Registering Too Many OS-Level Global Hotkeys
**What goes wrong:** Extending the `golang.design/x/hotkey` pattern from the three safety-cluster shortcuts to the entire PLAY-02 keyboard workflow, causing conflicts with other running applications' global shortcuts and requiring the operator to hunt down which app "stole" a keystroke.
**Why it happens:** Once the mechanism exists, it is tempting to reuse it everywhere "for consistency."
**How to avoid:** Reserve OS-level global hotkeys strictly for the three D-13 safety-cluster controls (and the Grand Master/stop-all controls in PLAY-06 if the plan extends the same trigger path to them); everything else in PLAY-02 is ordinary in-webview keyboard handling scoped to the app window.

## Code Examples

### Wails Go-to-frontend event push (throttled status)
```go
// Source: pattern from Wails v2 runtime.EventsEmit documentation
// (wails.io/docs/reference/runtime/events) plus this project's own
// ARCHITECTURE.md guidance to treat events as coalesced hints.
func (a *App) pushStatus(ctx context.Context, snapshot StatusSnapshot) {
    // Call at a fixed cadence (e.g. one push per rendered frame budget,
    // not once per Engine tick) -- never emit on every 40Hz engine tick.
    runtime.EventsEmit(ctx, "status:update", snapshot)
}
```

### Reusing the existing Phase 4 IPC client from the Wails Go host
```go
// Source: internal/artnet/ipc/client.go (existing code, unmodified usage)
conn, err := ipc.Dial(ipc.PipeName)
if err != nil {
    // GOLC_ARTNET_DAEMON_UNREACHABLE -- surface as daemon-not-running,
    // and (new for Phase 6) attempt to spawn `golc-project.exe artnet serve`
    // as a supervised child process, matching the WIN-02 pattern already
    // used for the TypeScript helper.
}
result := ipc.Forward(conn, ipc.Request{Route: "artnet safety blackout", Root: root})
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| Wails v1 (CGo-heavy, less mature bindings) | Wails v2 (current production line) | v2 GA, ongoing | Already reflected in this project's locked stack; no action needed. |
| Wails v2 lacking any global-shortcut primitive | Wails v3 alpha adds native `app.GlobalShortcut` | v3.0.0-alpha2.108 (2026-06-28) | Confirms this is a known, real gap in v2 (not an oversight in this research) -- the project's `golang.design/x/hotkey` workaround is the documented community answer, not a stopgap invented here. |

**Deprecated/outdated:**
- None identified specific to this phase's chosen stack; Wails v2 and gomidi/midi v2 are both current, actively maintained major-version lines.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `gitlab.com/gomidi/midi/v2` is the right MIDI library choice (vs. an unlisted alternative) | Standard Stack | Wrong choice means reworking the entire `internal/midi` package; mitigated by `checkpoint:human-verify` before first install. |
| A2 | `midicatdrv` + a separately pinned `midicat` binary is workable for this project's packaging model (WIN-02) | Standard Stack, Package Legitimacy Audit | If the helper-binary approach proves too brittle in practice, fallback is a CGo build with an explicitly added toolchain requirement -- a real scope increase for WIN-01/WIN-02 that should be flagged early, not discovered late in Phase 6 execution. |
| A3 | `golang.design/x/hotkey` reliably delivers hotkey callbacks from a background goroutine while Wails' own Win32 message loop is also running, with no event-loop conflict | Standard Stack, Pattern 2 | If there is an undocumented conflict between Wails' internal window message loop and `hotkey`'s own Win32 message pump, the safety-cluster shortcuts could silently fail to register or fire — this must be spiked/prototyped early in Phase 6 execution, not assumed to "just work" from library docs alone. |
| A4 | The cross-to-catch soft-takeover algorithm in Pattern 4 (direction-aware crossing, not proximity) matches what MIDI-HW-02's physical acceptance testing will actually need against the three selected controllers | Architecture Patterns Pattern 4 | If a controller's CC resolution/jitter causes false "crossed" detections near the target, an additional hysteresis/debounce band may be needed — flag for MIDI-HW-02 acceptance testing, not assumed correct from documentation alone. |
| A5 | React + Zustand (STACK.md's locked choice) is adequate for the D-09/D-10 real-time slider/ghost-marker animation at MIDI CC message rates | Standard Stack (Supporting) | If React re-render overhead proves too slow for smooth 60fps-equivalent fader tracking, a canvas/WebGL-rendered control or a non-React micro-layer for just the takeover sliders may be needed — this is a performance risk to validate early, not late. |

## Open Questions

1. **Does the Wails process spawn/supervise the daemon, or assume it is already running?**
   - What we know: Phase 4 established the daemon as standalone-capable and CLI-launchable (`golc-project.exe artnet serve`); Phase 6's CONTEXT.md quotes Phase 4 D-04 verbatim ("Phase 6's Wails app is just one more client").
   - What's unclear: whether Wails' `OnStartup` should itself spawn the daemon as a supervised child process (matching the WIN-02 "supervises every required runtime component" framing already used for the TypeScript/Deno helper) or whether a separate always-on service/launch mechanism is expected.
   - Recommendation: Plan for Wails to check reachability (`ipc.Dial`) on startup and spawn+supervise the daemon if unreachable, mirroring the Deno sidecar supervision pattern in `STACK.md`; this keeps a single-click desktop launch experience for a single-operator console.

2. **Exact shape of Revoke Automation's "frozen look" state.**
   - What we know: D-13/D-16 place Revoke Automation in the safety cluster; PLAY-08 requires it to "cancel queued actions" and "freeze the current look" even with a hung automation runtime.
   - What's unclear: Phases 8 (scripting) and 9 (LLM autonomy) don't exist yet, so there is no queued-action or automation-lease model to cancel yet. Phase 6 can only build the *manual* trigger and the daemon-side flag/freeze primitive; the actual script/AI queue-cancellation semantics are necessarily a forward-looking interface this phase defines but Phases 8/9 fully exercise.
   - Recommendation: Design the daemon-side `revokeAutomation` flag now as a simple "block any command whose `Request` carries a non-manual source tag" gate (the source-tagging convention already implied by PLAY-06/`internal/command`'s typed `Request`), so Phase 8/9 have a stable contract to build against without Phase 6 needing to fully implement automation itself.

3. **Frontend animation performance for D-09/D-10 at real MIDI message rates.**
   - What we know: gomidi delivers Note/CC messages as fast as the physical controller sends them (potentially >100 msg/sec on a fast fader sweep).
   - What's unclear: whether pushing every message straight to `EventsEmit` for D-09's "real-time" slider tracking causes visible frontend jank, vs. needing a fixed-rate coalescing layer (per this project's own ARCHITECTURE.md event-coalescing guidance).
   - Recommendation: Prototype/benchmark early against one of the MIDI-HW-01 controllers before committing to an emit-every-message approach; default to a throttled emit (e.g. one push per ~16ms) with the Go host retaining the authoritative unthrottled state for the actual takeover-crossing decision (which must never be throttled, only its visual reflection).

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | All Go code | check (repo-pinned, not probed live in this session) | 1.26.5 (per `config/toolchain.toml`) | — |
| WebView2 runtime (Windows) | Wails production builds | Not probed this session (Windows-only requirement, verify on target machine per `.planning/research/PITFALLS.md`'s Wails packaging pitfall) | — | Wails installer bootstraps WebView2 if missing (existing project research) |
| Physical MIDI hardware (Akai MIDImix / Novation Launch Control XL Mk2 / Worlde EasyControl 9) | PLAY-04/05 acceptance, MIDI-HW-02 | Not probed this session — see `.planning/midi/MIDI-HW-02-CHECKLIST.md` (physical evidence still pending per device) | — | Development/unit testing can proceed against `gomidi`'s `testdrv` mock driver; real-device acceptance remains gated by the open MIDI-HW-02 blocker, unaffected by this research |
| C/CGo toolchain (MinGW/MSVC) | Only if `rtmididrv`/`portmididrv` are chosen instead of `midicatdrv` | Not present per `config/toolchain.toml` (Go-only pin) | — | Recommended: use `midicatdrv` and avoid this dependency entirely (see Pitfall 3) |

**Missing dependencies with no fallback:**
- None identified — every dependency above has a documented fallback or is independently gated by the pre-existing MIDI-HW-02 blocker, which this phase's planning already accounts for per ROADMAP.md.

**Missing dependencies with fallback:**
- CGo toolchain (fallback: `midicatdrv`, avoiding the need entirely — recommended default).
- Physical MIDI hardware for full acceptance (fallback: `testdrv` mock for development; real acceptance tracked separately under MIDI-HW-02).

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go `testing` + race detector (existing project standard, `.planning/research/STACK.md`); no frontend test framework yet configured in this repo — Vitest/Playwright are STACK.md recommendations, not yet installed |
| Config file | none yet — see Wave 0 |
| Quick run command | `go test ./internal/midi/... ./internal/artnet/... -run <Test>` (per-package, once packages exist) |
| Full suite command | `go test -race ./...` (existing project-wide convention) |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| PLAY-01/02 | Every playback action reachable via on-screen + documented keyboard controls | manual + smoke (Playwright/WebView2, per STACK.md) | none yet | ❌ Wave 0 |
| PLAY-03 | Constrained operator surface: visible-but-locked enforcement | unit (Go, `internal/operatorsurface`) | `go test ./internal/operatorsurface/... -run TestAssignment` | ❌ Wave 0 |
| PLAY-04/05 | MIDI Note/CC learn, conflict rejection, soft takeover crossing | unit (Go, `internal/midi`), using `testdrv` mock | `go test ./internal/midi/... -run TestLearn` / `TestTakeover` | ❌ Wave 0 |
| PLAY-06/09 | Group masters/Grand Master/stop-all/blackout via local-priority path | integration (Go, `internal/artnet`), asserting Worker output flips within one tick of the daemon flag being set, independent of a simulated slow/hung IPC client | `go test ./internal/artnet/... -run TestSafetyOverride` | ❌ Wave 0 |
| PLAY-07 | Live status visibility (scene/layers/BPM/source/output state) | unit/contract (Go, status payload shape) + manual UAT | `go test ./internal/wails/... -run TestStatusPayload` | ❌ Wave 0 |
| PLAY-08 | Revoke Automation blocks/cancels/freezes/restores even with a hung automation runtime | integration (Go), simulating a non-responsive automation source | `go test ./internal/artnet/... -run TestRevokeAutomation` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** targeted `go test ./internal/<package>/... -run <Test>`
- **Per wave merge:** `go test -race ./...`
- **Phase gate:** Full suite green before `/gsd-verify-work`; PLAY-06/08/09's timing-sensitive tests should assert on elapsed wall-clock bound (e.g. override visible within N milliseconds), not just eventual correctness, given the "does not wait for...work to complete" requirement wording.

### Wave 0 Gaps
- [ ] `internal/midi/` package and its test files — covers PLAY-04/PLAY-05
- [ ] `internal/operatorsurface/` (or `internal/show` extension) and its test files — covers PLAY-03
- [ ] `internal/artnet/safety.go` and its test files — covers PLAY-06/08/09
- [ ] `internal/wails/` package (Go host, app.go/events.go) and its test files — covers PLAY-01/02/07
- [ ] Frontend test tooling (Vitest/Playwright per STACK.md) is not yet installed anywhere in this repo — first Wails-touching phase, so this is expected, not a regression

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | No | This phase is a local desktop app with no network-exposed auth surface (loopback API is Phase 7 scope, not this phase). |
| V3 Session Management | No | No session/auth tokens introduced by this phase. |
| V4 Access Control | Yes | Operator-surface visible-but-locked enforcement (D-04) must be enforced server-side (Go host / daemon), never trusted from frontend-only hiding of controls — the frontend can render locked controls disabled, but the Go command dispatch must independently reject an action against an item not assigned to the active surface. |
| V5 Input Validation | Yes | MIDI Note/CC values, hotkey registrations, and operator-surface assignment payloads from the frontend must be validated the same way every other `internal/command` request already is (bounds/shape checks before mutation), following the existing `{DOMAIN}_{CONDITION}` diagnostic convention. |
| V6 Cryptography | No | No new cryptographic surface in this phase. |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Frontend-only enforcement of operator-surface locking (D-04) bypassed via a crafted/replayed IPC/Wails-bound call | Tampering / Elevation of Privilege | Re-validate every command against the active operator surface's assignment set in the Go host/daemon, not just in the React render layer. |
| Untrusted `midicat` helper binary substituted at the pinned download path | Tampering | Apply the same SHA-256-pinned, allowlisted-host download discipline already used for Go/Node in `config/toolchain.toml` to the `midicat` binary (flagged `[SUS]` in the Package Legitimacy Audit above pending this control). |
| A malicious or misbehaving OS-level global hotkey conflict causing the safety cluster to silently fail to register | Denial of Service | Verify `hotkey.Register` return values at startup and surface a clear, visible error/warning in the UI (and a log line) if any of the three D-16 shortcuts fails to register, rather than failing silently. |

## Sources

### Primary (HIGH confidence)
- `internal/command/router.go`, `internal/artnet/daemon.go`, `internal/artnet/ipc/*.go`, `internal/playback/engine.go`, `internal/show/state.go`, `internal/show/schema.go` — existing repository code read directly during this research session.
- `.planning/research/STACK.md`, `.planning/research/ARCHITECTURE.md`, `.planning/research/PITFALLS.md`, `.planning/research/FEATURES.md` — prior project-level research, already locked/committed.
- `.planning/phases/04-observable-art-net-live-output/04-CONTEXT.md`, `.planning/phases/05-durable-shows-and-recovery/05-CONTEXT.md` — prior-phase locked decisions this phase must follow.
- `proxy.golang.org` live queries for `github.com/wailsapp/wails/v2`, `gitlab.com/gomidi/midi/v2`, `golang.design/x/hotkey` `@latest` — run directly during this session (2026-07-23).

### Secondary (MEDIUM confidence)
- [Wails Events reference](https://wails.io/docs/reference/runtime/events/) and [wailsapp/wails discussion #2844](https://github.com/wailsapp/wails/discussions/2844) — EventsEmit/EventsOn architecture.
- [Wails Templates](https://wails.io/docs/community/templates/) — frontend scaffold options.
- [wailsapp/wails issue #3112](https://github.com/wailsapp/wails/issues/3112), [wailsapp/wails discussion #2320](https://github.com/wailsapp/wails/discussions/2320) — v2's missing global-shortcut API and the community `golang.design/x/hotkey` workaround.
- [v3alpha.wails.io Keyboard Shortcuts](https://v3alpha.wails.io/features/keyboard/shortcuts/) and [v3 changelog](https://v3.wails.io/changelog/) — confirms `app.GlobalShortcut` is v3-only, added alpha2.108.
- [github.com/golang-design/hotkey](https://github.com/golang-design/hotkey), [hotkey_windows.go](https://github.com/golang-design/hotkey/blob/main/hotkey_windows.go) — pure-Go Windows implementation confirmation.
- [gitlab.com/gomidi/midi GitLab issue #28](https://gitlab.com/gomidi/midi/-/issues/28) — CGo/MSVC Windows driver friction.
- [gitlab.com/gomidi/tools midicat](https://gitlab.com/gomidi/tools/-/tree/main/midicat) — midicat helper-binary driver.
- Community documentation on MIDI soft takeover/pickup mode: [Synthstrom forum](https://forums.synthstrom.com/discussion/5470/midi-knob-takeover-modes), [DJ TechTools Takeover Mode](https://techtools.zendesk.com/hc/en-us/articles/202165744-Takeover-Mode), [Surge synthesizer issue #7510](https://github.com/surge-synthesizer/surge/issues/7510) — no single official MIDI 1.0 spec section defines takeover; this is a cross-checked community convention, not a protocol requirement.
- [github.com/microsoft/go-winio](https://github.com/microsoft/go-winio) — named-pipe IPC library (already in use).

### Tertiary (LOW confidence)
- General JS debounce/throttle articles (not Wails-specific) consulted for the event-throttling question in Open Question 3 — no Wails-specific authoritative guidance was found; treat the throttling cadence as a value to benchmark, not a documented best practice.

## Metadata

**Confidence breakdown:**
- Standard stack: MEDIUM — Wails v2/go-winio are HIGH-confidence (locked project decisions/existing code); gomidi and golang.design/x/hotkey are newly introduced and only WebSearch-verified, hence the `[ASSUMED]`/`checkpoint:human-verify` gating throughout.
- Architecture: MEDIUM — the daemon-resident safety-override pattern is a direct, low-risk extension of Phase 4's already-locked, already-implemented daemon architecture; the OS-level hotkey approach is a reasonable but not yet prototyped answer to a genuinely hard constraint (D-16 + orchestrator note 5).
- Pitfalls: MEDIUM — Pitfall 1 is HIGH confidence (already documented in this project's own ARCHITECTURE.md); Pitfalls 2-4 are MEDIUM, synthesized from community sources and this session's own architectural reasoning, not from an official spec.

**Research date:** 2026-07-23
**Valid until:** 30 days for the Wails/architecture guidance (stable, locked project decisions); 7-14 days for the specific gomidi/hotkey package version numbers, given this research session found gomidi/midi v2 tagged as recently as 2026-06-15 and golang.design/x/hotkey as recently as 2026-06-06 — re-verify exact versions immediately before `go get` during planning/execution.

---
*Phase: 6-Wails Authoring and Operator Surface*
*Research completed: 2026-07-23*
