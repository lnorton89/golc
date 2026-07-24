---
name: sketch-findings-golc
description: Validated GOLC UI design decisions, CSS patterns, workflow structure, and visual direction from interactive sketch experiments. Load for frontend planning or implementation.
---

<context>
## Project: GOLC

GOLC is a desktop lighting-control application for small live shows. Its interface must support
fixture patching, modular scene authoring, constrained operator handoff, deterministic playback,
MIDI control, Art-Net output, and independent emergency actions without the frontend becoming
runtime authority.

The validated direction is a focused live-production instrument: dense without being cramped,
calm while idle, unmistakable when state changes, and fast enough to operate from muscle memory.
Reference points are Ableton Live's separate author/performance mental models and uninterrupted
transport, and Bitwig Studio's task-curated views, contextual inspectors, and optional panels.

Sketch session wrapped: 2026-07-23
</context>

<design_direction>
## Overall Direction

- Use a dark, low-distraction console shell based on GOLC's Paper/Ink brand system.
- Reserve Signal Blue for selection, active/manual state, primary actions, and focus.
- Reserve green, gold, red, black, and gray for frame-lock, pickup/warning, revoke/error,
  blackout, and offline/locked semantics.
- Keep show identity, transport, live timing, controlling source, output health, and the
  independent safety cluster visible across workspaces.
- Organize navigation by user intent: Show, Build, Operate, and Output.
- Give each workspace one central job and one contextual right inspector.
- Use compact 4px/8px/12px spacing and 9px–14px labels in operational regions; use larger type
  only for active scene, primary cue, or critical live values.
- Prefer selection-driven detail and bounded internal scroll regions over a page-length dashboard.
</design_direction>

<locked_decisions>
## Validated Winners

1. **Application shell:** Focused Command Rail — Sketch 001 Variant D.
2. **Programming:** Scene Stack + Inspector — Sketch 002 Variant A.
3. **Performance:** Launcher + Masters — Sketch 003 Variant A.
4. **First-show flow:** Guided First Show — Sketch 004 Variant B.

Treat these as validated design decisions. Do not re-open them during implementation unless a
technical constraint or user test produces contradictory evidence.
</locked_decisions>

<findings_index>
## Design Areas

| Area | Reference | Key Decision |
|------|-----------|--------------|
| Application Shell & Navigation | `references/application-shell-navigation.md` | Grouped command rail around a focused canvas and contextual inspector |
| Programming & Scene Authoring | `references/programming-scene-authoring.md` | Scene-led programming with exactly four compact layer rows |
| Live Operation, Safety & MIDI | `references/live-operation-safety-midi.md` | Random-access scene launcher with masters, locked controls, and persistent live truth |
| Onboarding, Readiness & Impact Review | `references/onboarding-readiness-impact.md` | Optional guided first-show flow with explicit evidence and safe exit |

## Theme

The winning shared theme is at `sources/themes/default.css`.

## Interactive Sources

- `sources/001-workspace-shell/index.html`
- `sources/002-programming-workspace/index.html`
- `sources/003-performance-workspace/index.html`
- `sources/004-patch-to-play-flow/index.html`

Each source preserves all alternatives and visibly marks the selected winner.
</findings_index>

<implementation_guidance>
## Implementation Sequence

1. Refactor `frontend/src/App.tsx` into the persistent shell before restyling feature panels.
2. Add navigation state for Show / Build / Operate / Output destinations. Navigation changes only
   the projection; it must not interrupt playback or Art-Net.
3. Establish shared primitives: panel, panel header, command rail group, toolbar, list row, chip,
   button, field, internal scroll region, inspector, and safety footer.
4. Move existing fixture, scene, operator-surface, MIDI, and Art-Net components into focused
   workspaces without changing their Wails command contracts.
5. Implement Scene Stack programming, then Launcher + Masters performance, then Guided First Show.
6. Consolidate repeated offline diagnostics into the global status frame with contextual detail.
7. Add populated visual fixtures and browser screenshot tests at desktop and compact desktop widths.
8. Validate keyboard focus order, resize behavior, long names, 0/1/many entities, locked controls,
   MIDI pickup, daemon loss, and safety availability.

## Implementation Constraints

- React/Zustand remain projections of Go-owned state.
- Do not place playback clocks, Art-Net timing, or safety authority in the frontend.
- Preserve server-side authorization for locked operator-surface controls.
- Pool resizing and fixture substitution continue to require deterministic preview and explicit
  apply.
- Guided setup is optional; direct navigation remains available and progress is retained.
- Physical MIDI compatibility claims remain gated by MIDI-HW-02 evidence.
</implementation_guidance>

<metadata>
## Processed Sketches

- 001-workspace-shell
- 002-programming-workspace
- 003-performance-workspace
- 004-patch-to-play-flow
</metadata>

