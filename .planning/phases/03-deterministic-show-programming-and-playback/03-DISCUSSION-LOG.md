# Phase 3: Deterministic Show Programming and Playback - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-07-21
**Phase:** 3-Deterministic Show Programming and Playback
**Areas discussed:** Scene layer semantics, Live edit adoption boundary, Chase & motion determinism, Undo/redo scope

---

## Scene Layer Semantics

| Option | Description | Selected |
|--------|-------------|----------|
| Foundational static state | Base-look sets a resting intensity + position/beam + default color that other layers selectively override on top of | ✓ |
| Intensity/position only | Base-look covers intensity and position/beam only — color always from color-theme | |
| Full static snapshot | Base-look is a complete static scene that other layers wholesale-replace | |

**User's choice:** Foundational static state

| Option | Description | Selected |
|--------|-------------|----------|
| Fixed layer-priority order | base-look < color-theme < chase < motion — later layer overrides earlier for touched attributes | ✓ |
| HTP for intensity, LTP elsewhere | Classic desk convention: HTP for intensity, LTP for color/position/beam | |
| Per-layer blend weight | Explicit opacity/weight per layer, mixed proportionally | |

**User's choice:** Fixed layer-priority order

| Option | Description | Selected |
|--------|-------------|----------|
| Per-layer selection | Each layer independently picks its own pool/group/deployment/fixture selection | ✓ |
| One shared scene selection | All layers share the scene's fixture selection | |

**User's choice:** Per-layer selection

| Option | Description | Selected |
|--------|-------------|----------|
| Position/beam only | Motion presets are pan/tilt + beam-shaping only; color/intensity stay elsewhere | ✓ |
| Position/beam + wheel effects | Motion presets can also include gobo/color-wheel indexing | |

**User's choice:** Position/beam only
**Notes:** All four questions in this area converged on the recommended option with no follow-up discussion requested.

---

## Live Edit Adoption Boundary

| Option | Description | Selected |
|--------|-------------|----------|
| Next bar boundary | Edit compiled and staged, swapped in atomically at the start of the next bar | ✓ |
| Next full loop restart | Edit only takes effect when the bar-loop restarts from bar 1 | |
| Immediately (next frame) | Edit adopted as soon as it compiles, on the next evaluated frame | |

**User's choice:** Next bar boundary

| Option | Description | Selected |
|--------|-------------|----------|
| Reject, keep last-good running | Invalid plan rejected outright; engine keeps running last valid compiled version | ✓ |
| Reject and pause the layer | Invalid layer disables itself (goes dark/inert) until fixed | |

**User's choice:** Reject, keep last-good running

| Option | Description | Selected |
|--------|-------------|----------|
| Same boundary for all layers | Base-look, color-theme, chase, motion all wait for the same next-bar boundary | ✓ |
| Faster for base-look/color | Static-ish layers swap immediately; only tempo-relative layers wait | |

**User's choice:** Same boundary for all layers

| Option | Description | Selected |
|--------|-------------|----------|
| Always editable live | Authors can edit any object, including live ones, at any time — no explicit pause step | ✓ |
| Require explicit pause for live objects | Editing a live-active object requires pausing/detaching it first | |

**User's choice:** Always editable live
**Notes:** All four questions converged on the recommended option; no follow-up requested.

---

## Chase & Motion Determinism

| Option | Description | Selected |
|--------|-------------|----------|
| No randomization in v1 | Chases/motion presets are explicit authored steps only, no random mode | ✓ |
| Seeded randomization allowed | Random-order/random-in-range mode allowed but reproducible via fixed seed | |

**User's choice:** No randomization in v1

| Option | Description | Selected |
|--------|-------------|----------|
| Global BPM + bar position | Chase step advancement derives from the same clock driving scene looping | ✓ |
| Per-chase independent rate | Each chase can run at its own fixed rate independent of global BPM | |

**User's choice:** Global BPM + bar position

| Option | Description | Selected |
|--------|-------------|----------|
| Follows the scene's SCEN-08 choice | Chases/motion inherit the containing scene's preserve-position-or-restart behavior | ✓ |
| Chases always restart on BPM change | Chase/motion sequences always restart from step 1 on BPM change regardless of scene setting | |

**User's choice:** Follows the scene's SCEN-08 choice
**Notes:** This area was scoped to three questions (not four) — the fourth-slot question about a separate per-layer randomization override was judged unnecessary once "no randomization in v1" was chosen. All three converged on the recommended option.

---

## Undo/Redo Scope

| Option | Description | Selected |
|--------|-------------|----------|
| Whole-session linear history | One global undo/redo stack across the entire session, covering every object type | ✓ |
| Per-object-type stacks | Separate undo stacks per object type | |

**User's choice:** Whole-session linear history

| Option | Description | Selected |
|--------|-------------|----------|
| Yes — same behavior, adoption-boundary gated | Undo is just another edit, recompiled and adopted through the same live-adoption boundary | ✓ |
| Undo disabled for live-active objects | Undo/redo blocked on objects currently part of the active scene | |

**User's choice:** Yes — same behavior, adoption-boundary gated

| Option | Description | Selected |
|--------|-------------|----------|
| Session-only, resets on close | Undo history is in-memory for the current run, resets on reopen | ✓ |
| Persisted across sessions | Undo/redo history saved with the show file itself | |

**User's choice:** Session-only, resets on close
**Notes:** All three questions converged on the recommended option; no follow-up requested. User confirmed ready for context immediately after this area.

---

## Claude's Discretion

None — every gray area discussed converged on the recommended option; no "you decide" selections were made in this session.

## Deferred Ideas

None — discussion stayed within phase scope. No scope-creep items came up.
