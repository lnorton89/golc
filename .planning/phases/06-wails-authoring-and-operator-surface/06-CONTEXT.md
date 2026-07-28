# Phase 6: Wails Authoring and Operator Surface - Context

**Gathered:** 2026-07-23
**Status:** Ready for planning

<domain>
## Phase Boundary

Authors and playback operators can complete the conventional show workflow (patch, program, play) through a responsive Wails desktop application, keyboard, and constrained generic MIDI controls, without the frontend ever becoming runtime authority over playback or Art-Net. This phase delivers: (1) on-screen + documented keyboard controls exposing the complete workflow without requiring MIDI hardware (PLAY-01/02), (2) a show author's ability to build a constrained operator surface containing only assigned scenes, layers, masters, and safety controls, with the operator always able to see active scene, layers, BPM/bar position, controlling source, and final output state (PLAY-03/07), (3) generic MIDI Note/CC learn for supported playback commands with fader soft takeover (PLAY-04/05), (4) group masters, Grand Master, stop/release-all, and immediate blackout through local-priority paths independent of UI/script/API/LLM completion (PLAY-06/09), and (5) Revoke Automation as an immediate, independent override that blocks AI/scripts, cancels their queued actions, freezes the current look, and returns manual control even when an automation runtime is hung (PLAY-08).

This is the first phase introducing a GUI — no Wails scaffolding or frontend directory exists in the repo yet. Every backend capability this phase surfaces already exists behind the shared typed `internal/command` Request/Result registry built in Phases 1-5.

Requirements: PLAY-01 through PLAY-09.

</domain>

<decisions>
## Implementation Decisions

### Operator Surface Builder
- **D-01:** A show author builds each constrained operator surface by **assigning directly from the full authoring view** — toggling "add to this operator surface" on scenes/layers/masters/safety controls in place (checkbox or right-click), not through a separate dedicated builder screen or a drag-and-drop canvas. No new layout-design screen needs to be built.
- **D-02:** A show can define **multiple named operator surfaces** (e.g. different surfaces per venue or per operator), not just one. This is more state to design and persist than a single-surface model, but matches the "hand a simple controller surface to another person" core-value framing when a show may be handed off differently in different contexts.
- **D-03:** Assignment granularity is **individual items only** — an author picks specific scenes, layers, masters, and safety controls one at a time. There is no group/category-level bulk-assign shortcut (e.g. "all layers of Scene X") in this phase.
- **D-04:** Anything not assigned to a given operator surface is **visible but locked** (shown grayed out/disabled), not hidden entirely. The operator can see the full show's scope on that surface but cannot interact with anything outside what was assigned — "constrained" is enforced by interaction, not by visibility.

### MIDI Learn & Conflicts
- **D-05:** MIDI learn is initiated **per-control** — each mappable control has its own small "Learn" affordance; clicking it and then moving/pressing the physical control completes the mapping. There is no global "MIDI Learn mode" toggle and no MIDI-activity-monitor-and-assign panel in this phase.
- **D-06:** Mapping a MIDI Note/CC that's already assigned to a different control is **blocked until the existing mapping is explicitly removed** — the new mapping is rejected outright rather than silently overwriting or prompting a confirm-to-reassign dialog. This is the safest option against accidental reassignment from a stray physical control movement.
- **D-07:** MIDI mappings are **per operator surface**, not global to the show. Each named surface (D-02) carries its own independent MIDI mapping set, so a different controller/venue setup can be swapped in with a different surface without disturbing another surface's mappings.
- **D-08:** Any control that's assigned to a given operator surface is automatically MIDI-learnable — **there is no separate fixed list of MIDI-mappable commands independent of what's on the surface.** This keeps the builder (D-01) and the MIDI mapping surface (D-07) consistent: what the author put on the surface is exactly what can be learned.

### Soft Takeover Feedback
- **D-09:** While a physical fader hasn't caught up to the app's current value, the **on-screen slider visually follows the physical fader's live position in real time**, shown in a distinct "not armed"/pickup visual state (not the normal live-controlling color).
- **D-10:** A **ghost/target marker shows the app's actual current value** as a fixed reference point the physical fader must reach. Combined with D-09, the visible slider tracks the incoming physical position while the ghost marker shows the value it must cross to take over — the gap between the two is the operator's visual cue for which way and how far to move the physical control.
- **D-11:** The catch-up mechanic is **cross-to-catch (standard soft pickup)**: the physical fader's value must cross/pass through the app's current value before it takes control. Proximity-threshold takeover (arming within some tolerance without crossing) was not chosen.
- **D-12:** Soft takeover logic applies **only to continuous CC/fader controls**, not to Note/button controls (scene select, toggles, momentary triggers). Buttons act immediately on press with no pickup/arming state, matching PLAY-05's fader-specific wording.

### Safety Control Placement (Blackout / Revoke Automation / Stop-Release-All)
- **D-13:** Blackout, Revoke Automation, and Stop/Release-All live in a **persistent global bar present on every screen** — authoring, programming, and playback views alike — not only within the playback/operator view. This matches PLAY-08/09's "does not depend on UI" framing: these controls must be reachable regardless of what screen an author or operator is on.
- **D-14:** Activation uses **hold-to-confirm** (press and hold roughly 500ms-1s) rather than a single immediate click or a two-step arm-then-confirm flow. This guards against a stray misclick without meaningfully slowing down a genuine emergency action.
- **D-15:** The three controls are grouped into a **dedicated, visually distinct safety cluster** (e.g. a red-bordered panel) that stays in the same screen position at all times, rather than being placed individually near the context each one affects. This is chosen to build fast, predictable muscle memory under pressure.
- **D-16:** Each safety control gets a **fixed, global, unmodifiable default keyboard shortcut** that works regardless of on-screen focus. Shortcuts are not user-rebindable in this phase, preserving predictable emergency-control behavior across every show and every user.

### Claude's Discretion
None — every gray area discussed converged on an explicit user selection; no "you decide" selections were made in this session.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Project-level requirements and roadmap
- `.planning/PROJECT.md` — Core value ("hand a simple controller surface to another person for reliable playback"), the "Live reliability" and "Automation override" constraints (Revoke Automation and Blackout are local-priority paths that cannot depend on UI/script/API/LLM completion), and the pre-existing Context entries on Revoke Automation being distinct from Blackout.
- `.planning/REQUIREMENTS.md` §Playback and Operator Surface — PLAY-01 through PLAY-09 requirement text.
- `.planning/ROADMAP.md` §Phase 6: Wails Authoring and Operator Surface — Goal, five success criteria, the `MIDI-HW-01`/`MIDI-HW-02` blocker note (selection resolved, per-device physical acceptance still open), and the Validation note flagging information density, navigation, patch-to-playback speed vs. QLC+, constrained-surface learnability, cue-list needs, and the Wails/MIDI workflow as requiring operator validation.

### MIDI hardware evidence (selected acceptance set, MIDI-HW-02 still open)
- `.planning/midi/README.md` — Evidence index: Akai MIDImix, Novation Launch Control XL Mk2, and Worlde EasyControl 9 are the resolved MIDI-HW-01 selection; each device's manual-review status, SHA-256, and outstanding MIDI-HW-02 physical-evidence gaps (exact default channels/CC/Note addresses, button behavior, bank identity, Send-All ordering, current Windows endpoints, editor persistence, reconnect behavior). This phase implements generic Note/CC learn (D-05 through D-08) and soft takeover (D-09 through D-12) against this set — it does not make or need a named hardware-compatibility claim to do so.
- `.planning/midi/MIDI-HW-02-CHECKLIST.md` — Per-device physical acceptance checklist; relevant if this phase's planning wants to cross-check that generic learn/takeover behavior will be exercisable against the selected set's actual button/fader semantics.

### Brand/design system (status vocabulary this phase's UI should reuse)
- `.planning/brand/GOLC-Brand-Guidelines.html` — Interactive brand guide; extract the real design system via computed styles per prior project convention (the `.md` files are a token summary, not the canonical source).
- `.planning/brand/GOLC-Brand-Tokens.md` — Quick-reference tokens: status colors already defined for `live` (#1B44D9), `frame-lock` (#5AC26A), `armed` (#C8A24B), `revoked` (#E23A2E), `blackout` (#17181C), and `offline` (#8A887F) — a ready-made vocabulary for PLAY-07's "controlling source and final output state" display and for D-15's safety-cluster styling (revoked/blackout colors are pre-defined). Also defines motion timing (snap/tap/settle/frame) relevant to the soft-takeover ghost-marker animation (D-09/D-10).

### Prior-phase precedent this phase should follow
- `.planning/phases/04-observable-art-net-live-output/04-CONTEXT.md` D-01/D-04 — "Phase 6 later wraps these same typed commands in Wails without rework, given the shared `internal/command` model" and "Phase 6's Wails app is just one more client that attaches to the same running [Art-Net worker] instance later." This phase's Wails frontend must bind to the existing `internal/command` Request/Result registry and the long-lived Art-Net worker process (Phase 4 D-03/D-04) as a client, not reimplement or duplicate either.
- `.planning/phases/04-observable-art-net-live-output/04-CONTEXT.md` D-12 — A per-universe/target output enable/disable control already exists in Phase 4, independent of and preceding this phase's Blackout/Revoke Automation, which act as higher-level overrides on top of it, not a replacement for it.
- `.planning/STATE.md` §Accumulated Context → Decisions — "UI, persistence, scripts, API, LLM, and Linear never own or block deterministic playback or Art-Net timing"; the governing constraint behind D-13/D-14/D-16's local-priority-path design for the safety cluster.

No user-referenced ADRs/specs beyond the project's own planning docs and the MIDI/brand assets above came up during discussion — no additional canonical docs to add.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/command` (Phases 1-5): shared typed `Request`/`Result`/`CommandRegistry` model (`internal/command/router.go`) that every domain command (`playback.go`, `scene.go`, `deployment.go`, `pool.go`, `programming.go`, `artnet.go`) already registers through. This phase's Wails bindings should call into this existing registry rather than reimplementing command dispatch — matches the explicit Phase 4 precedent that Phase 6 would do this "without rework."
- `.planning/brand/GOLC-Brand-Tokens.md`: complete color/status/type/motion token set already defined and ready to apply to the new Wails frontend — no need to invent a visual language from scratch.
- `internal/playback/engine.go` `Engine.CurrentFrame()`: lock-free atomic-pointer read already established as the non-backpressuring consumption point for downstream consumers (used by the Art-Net worker in Phase 4); the Wails frontend's live status display (PLAY-07: active scene, layers, BPM/bar position, final output state) should read through the same non-blocking path rather than polling/locking show state directly.

### Established Patterns
- `{DOMAIN}_{CONDITION}` diagnostic code convention (e.g. `GOLC_SHOW_STATE_INVALID`) — new Phase 6 diagnostics (MIDI conflict rejection, mapping errors) should follow the same naming convention.
- Long-lived standalone process with local IPC clients (Phase 4 D-03/D-04, `internal/artnet`) — the Wails desktop app is one more client attaching to this running instance, not a new process-ownership model.
- No frontend directory, `wails.json`, or any Wails scaffolding exists yet anywhere in the repo — this phase is greenfield for the entire GUI layer.

### Integration Points
- No `internal/midi` (or equivalent) package exists yet — MIDI learn/mapping/soft-takeover (D-05 through D-12) is new, greenfield code.
- No operator-surface/named-surface persistence model exists yet in `internal/show` — D-01/D-02/D-03/D-04's per-surface assignment state and D-07's per-surface MIDI mappings are new fields/types this phase must add, likely extending `show.State` (per the established Phase 2/3/5 pattern of extending the single revisioned document rather than a parallel model).

</code_context>

<specifics>
## Specific Ideas

- The soft-takeover visual design that emerged from discussion is a specific, coherent pairing: the on-screen slider tracks the physical fader's raw live position in real time (D-09), while a separate ghost/target marker shows the fixed app value it must cross (D-10) — together this gives the operator both "where is the hardware right now" and "where does it need to get to" without a numeric readout.
- The safety cluster (Blackout / Revoke Automation / Stop-Release-All) is envisioned as a single, visually distinct, fixed-position panel (D-15) — the user was clear this should read as one dedicated "emergency" zone rather than three controls scattered contextually near what they affect.
- Assignment to an operator surface and MIDI-learnability were explicitly tied together (D-08): whatever an author puts on a named surface is what becomes MIDI-mappable for that surface — there is no separate, independently-maintained list of "MIDI-mappable commands."

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope. All four discussed areas (operator surface builder, MIDI learn & conflicts, soft takeover feedback, safety control placement) were clarifications of how to implement what's already in PLAY-01 through PLAY-09.

</deferred>

---

*Phase: 6-Wails Authoring and Operator Surface*
*Context gathered: 2026-07-23*
