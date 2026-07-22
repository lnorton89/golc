# Phase 3: Deterministic Show Programming and Playback - Context

**Gathered:** 2026-07-21
**Status:** Ready for planning

<domain>
## Phase Boundary

Show authors can program complete tempo-aware looks — selection, semantic attribute editing, reusable themes/presets/chases/motion presets, and bar-based scenes with independently swappable layers — and run them through a headless deterministic playback engine whose compiled output is unaffected by adapter (UI, persistence, scripts, API, LLM) delay or failure. This phase is headless: domain model, compiler/engine, and CLI/API surface only, following Phase 2's precedent (Wails UI arrives in Phase 6).

Requirements: PROG-01 through PROG-07, SCEN-01 through SCEN-09.

</domain>

<decisions>
## Implementation Decisions

### Scene Layer Semantics
- **D-01:** A "base-look" layer is the scene's foundational static state — it sets a resting intensity, position/beam, and default color that other enabled layers (color-theme, chase, motion) selectively override on top of. It is not color-only and not a full immutable snapshot; it establishes the rest state a scene returns to for any attribute no other enabled layer touches.
- **D-02:** When two enabled layers touch the same attribute simultaneously, resolution uses a **fixed layer-priority order**: base-look < color-theme < chase < motion. A later layer in that order always overrides an earlier one for any attribute it touches. No HTP arbitration and no per-layer blend-weight mixing — priority order alone is fully deterministic.
- **D-03:** Each layer can independently target its own fixture selection (pool/group/deployment instance/direct fixture, per PROG-01) — a chase or motion preset can be scoped narrower than the scene's base-look (e.g. chase only runs on moving heads while base-look covers the whole rig). Layers are not forced to share one scene-wide selection.
- **D-04:** Motion presets touch only position/beam semantic capabilities (pan/tilt plus beam-shaping: zoom/focus/iris/prism). Color-wheel/gobo-wheel indexing is not part of a motion preset's scope — color effects stay with color-theme/base-look even when they share a physical wheel with beam shaping on some fixtures.

### Live Edit Adoption Boundary
- **D-05:** An author's edit/record to a programming object (preset, chase, scene) is compiled and staged, then swapped into the running output atomically at the **start of the next musical bar** — not immediately/mid-frame, and not deferred to the next full loop restart. This keeps live output musically coherent (SCEN-09) while adopting promptly.
- **D-06:** If an edit does not fully compile (e.g. references a fixture capability that no longer resolves), it is **rejected and the engine keeps running the last valid compiled version**, surfacing the error to the author. This is the concrete mechanism behind success criterion 5's "adopts only complete valid show plans at safe boundaries" — invalid plans are never partially adopted, and a rejected edit never blanks or disables the running layer.
- **D-07:** The next-bar adoption boundary applies uniformly to every layer type (base-look, color-theme, chase, motion) — one consistent rule per scene, no layer-specific fast path.
- **D-08:** Authors can always edit any object directly, including one currently live in the active scene — there is no explicit pause/detach/lock step required before editing. The adoption-boundary rule (D-05/D-06) is what keeps live output safe; it is not a workflow gate.

### Chase & Motion Determinism
- **D-09:** Chases and motion presets carry **no randomization in v1** — every step and movement is explicitly authored (ordered steps for chases, authored paths for motion presets). No random-order or random-in-range mode exists to reason about for reproducibility.
- **D-10:** A chase's tempo-relative step advancement is driven by the **same global BPM + bar-position clock** that drives scene looping (SCEN-01/02/03) — one authoritative musical clock for the whole engine, not an independent per-chase rate.
- **D-11:** When global BPM changes while a chase or motion preset is running, its step timing **follows the containing scene's SCEN-08 preserve-position-or-restart setting** — one consistent rule per scene; chases/motion do not have a separate, always-restart override.

### Undo/Redo Scope
- **D-12:** Undo/redo (PROG-07) uses a **single whole-session linear history** — one global stack across the entire authoring session covering record/update/rename/reorder/duplicate/delete on any object type, walked backward/forward in order. No per-object-type stacks.
- **D-13:** Undo/redo behaves identically whether or not the target object is currently part of the active live scene — an undo is just another edit, recompiled and adopted through the same D-05/D-06 live-adoption boundary. No special-casing or blocking for live-active objects.
- **D-14:** Undo history is **session-only** — in-memory for the current application run, reset on close/reopen. It is not persisted into the `.golc` file; SHOW-01/02 (Phase 5) treat the saved show as the durable unit, not the edit history.

### Claude's Discretion
None — every gray area discussed converged on the recommended option; no "you decide" selections were made in this session.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Project-level requirements and roadmap
- `.planning/PROJECT.md` — Core value, requirements, constraints (esp. "UI, persistence, scripts, API, LLM, and Linear never own or block deterministic playback or Art-Net timing"), and the pre-existing Context entries on scenes/BPM/blending that this phase must stay consistent with (bar-based looping, tap tempo, one active scene, immediate switching by default, selectable BPM-change loop behavior).
- `.planning/REQUIREMENTS.md` §Programming and §Tempo-Aware Scenes — PROG-01–07 and SCEN-01–09 requirement text and the Traceability mapping to Phase 3.
- `.planning/ROADMAP.md` §Phase 3: Deterministic Show Programming and Playback — Goal, five success criteria, and the research note flagging playback jitter/override budgets, HTP/LTP and release semantics, live plan adoption, first-user scale ceilings, deterministic effect seeding, and Windows timing behavior as open research areas (this discussion resolved the layer-priority/HTP-LTP, live-adoption-boundary, and effect-seeding questions at the decision level; remaining items — jitter/override budgets, scale ceilings, Windows timing — are for phase research/planning, not user discussion).

### Prior-phase precedent this phase should follow
- `.planning/STATE.md` §Accumulated Context → Decisions — Phase 1/2 decisions this phase must stay consistent with, notably the shared-typed-command-model constraint and the "never own or block deterministic playback" invariant.
- `.planning/phases/02-modular-fixtures-and-deployments/02-CONTEXT.md` — D-04 (shared `internal/command` typed command model), D-09/D-10 (deployment/group mental model that PROG-01 selection builds on), D-16 (staleness/revision-check pattern this phase's live-adoption boundary should stay consistent with in spirit, even though D-05/D-06 define a bar-boundary-specific mechanism rather than reusing revision-staleness directly).

No user-referenced ADRs/specs beyond the project's own planning docs came up during discussion — no additional canonical docs to add.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/show/state.go` (Phase 2): `show.State` is the existing revisioned ShowState container (`Pools`, `Deployments`, `Groups`, monotonic `Revision`, strict-decode `Load`/atomic-write `Save`). Phase 3's programming objects (themes, presets, chases, motion presets, scenes) are new fields/types that extend this same container rather than a parallel document — Save's revision-bump and atomic write pattern should carry forward.
- `internal/command` (Phase 1/2): shared typed command-registration model with `MustDeclareScope` — Phase 3 programming/scene/playback operations should register here so Phase 6 (UI) and Phase 7 (API) can expose them later without rework (matches 02-CONTEXT D-04).
- `internal/pool`, `internal/deployment` (Phase 2): existing pool/group/deployment domain types that PROG-01 selection (pool/group/deployment instance/direct fixture) resolves against directly.
- `internal/strictjson`: strict decode + canonical encode used by `show.State`; the same package should back any new programming-object persistence for consistency with the established strict-decoding convention.

### Established Patterns
- Whole-document validation before trust: `show.validate()` runs invariant checks (unique names, single active deployment, valid addresses, group reference resolution) before Load/Save trust or persist a State. New programming-object types (scenes, chases, presets) should extend this same whole-State validation rather than introducing a separate validation path.
- Write-temp-then-rename atomic persistence (`show.Save`) and revision-based staleness detection (Phase 1 D-18/D-21, reused in Phase 2 D-16) are established repo conventions Phase 3 should stay consistent with wherever it persists compiled show state.
- Diagnostic/error codes follow a `{DOMAIN}_{CONDITION}` convention (e.g. `GOLC_SHOW_STATE_INVALID`, `GOLC_ROUTE_SCOPE_UNDECLARED`) — new Phase 3 diagnostics (e.g. rejected live-edit compilation per D-06) should follow the same naming convention.

### Integration Points
- No scene, chase, theme, preset, motion-preset, programmer, or playback-engine code exists yet — `internal/` currently has `bootstrap`, `command`, `contracts`, `delivery`, `deployment`, `fixture`, `pool`, `projectconfig`, `security`, `show`, `strictjson`, `substitution`, `trace/*`. Phase 3 is greenfield for the programming/playback domain and should establish new package(s) (e.g. `internal/programming`, `internal/scene`, `internal/playback`) following the existing package-per-concern layout, extending `show.State` for persistence.
- The deterministic playback engine (SCEN-09, success criterion 5) is explicitly required to be independent of UI/persistence/scripts/API/LLM timing — its package boundary should make that isolation structurally visible (e.g. no imports from those future packages into the engine/compiler path), consistent with PROJECT.md's "Live reliability" constraint.

</code_context>

<specifics>
## Specific Ideas

- Layer resolution is intentionally simple and fully deterministic: fixed priority order (base-look < color-theme < chase < motion), not a runtime HTP comparison or blend-weight math — this was a deliberate choice to keep playback output mechanically predictable under the "deterministic playback" phase goal.
- The live-edit story is "always editable, safely adopted" rather than "lock before edit" — D-08 explicitly rejects a pause/detach workflow gate in favor of the next-bar adoption boundary doing the safety work.
- No randomization anywhere in chases/motion presets for v1 (D-09) — this was chosen specifically to avoid needing a seeding/reproducibility story now; if randomized effects are wanted later, that's a new capability for a future phase, not a v1 addition.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope. No scope-creep items came up; all four discussed areas (scene layer semantics, live edit adoption boundary, chase & motion determinism, undo/redo scope) were clarifications of how to implement what's already in PROG-01–07 and SCEN-01–09.

</deferred>

---

*Phase: 3-Deterministic Show Programming and Playback*
*Context gathered: 2026-07-21*
