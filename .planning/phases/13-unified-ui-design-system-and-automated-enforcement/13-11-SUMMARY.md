---
phase: 13-unified-ui-design-system-and-automated-enforcement
plan: "11"
subsystem: scene-programming-ui
tags: [react, design-system, scene-programming, scenes-and-looks]
requires:
  - phase: 13-02
    provides: DS001-DS010 policy checker
  - phase: 13-07
    provides: Typed primitives/patterns and public barrel, including the SceneStack pattern seed
affects: [remaining Wave 7 workspace migrations]
tech-stack:
  added: []
  patterns: ["SceneStack now accepts an optional `label` distinct from its `tone`, so a status chip's brand-required display text (e.g. \"LIVE\") can differ from the semantic tone key (e.g. \"live\") backing its color/icon."]
key-files:
  modified:
    - frontend/src/components/SceneProgramming/SceneList.tsx
    - frontend/src/components/SceneProgramming/SceneList.module.css
    - frontend/src/components/SceneProgramming/LookBrowser.tsx
    - frontend/src/components/SceneProgramming/LookBrowser.module.css
    - frontend/src/workspaces/build/ScenesLooksWorkspace.tsx
    - frontend/src/workspaces/build/ScenesLooksWorkspace.module.css
    - frontend/src/design-system/patterns/index.tsx
    - frontend/design-system/runtime-geometry.json
key-decisions:
  - "Fix SceneStack's scene-status mapping (tone and label) rather than leave the interrupted prior session's version: idle scenes were mapped to the \"armed\" (amber/warning) tone, which misrepresents a merely-not-currently-playing scene as a warning condition, and the Chip rendered the raw lowercase tone string (\"live\"/\"armed\") as its own text instead of the brand-required uppercase \"LIVE\" label."
  - "Remove the scene-detail header's own LIVE badge for the active scene instead of accepting a third redundant LIVE source: the composition test (243fc2a7's RED assertion) fixes the total on-screen \"LIVE\" count at exactly 2 (SceneList's row meta + SceneStack's tag); the header's own copy, only ~40px away from SceneStack in the same viewport, was pure duplication once SceneStack correctly displayed the status."
  - "Register --ds-scenelist-width in runtime-geometry.json instead of an inline CSS var() fallback, matching the existing --ds-guided-first-show-stage-rail precedent for user-resized panel widths."
  - "Target ListRow's own [data-state] attribute instead of the bare button element type for SceneList.module.css's background-flattening override: a bare `button` selector is a DS005 violation regardless of legitimate cross-component intent, and this file may not depend on ListRow's private generated class name."
patterns-established:
  - "A Field/NumberStepper pair meant to sit side-by-side in a narrow container should use `flex-wrap: wrap` with a `flex-basis` minimum (not a plain `flex: 1`), so a longer label wraps its whole field onto its own row instead of wrapping just the label text within a too-narrow column."
requirements-completed: [D-02, D-03, D-04, D-05, D-07, D-11, D-12, D-14, UI-SPEC-SCENE-STACK, UI-SPEC-OUTPUT]
coverage:
  - id: D1
    description: Scenes & Looks uses SceneStack without changing programming or playback commands
    verification:
      - kind: unit
        ref: "frontend/src/workspaces/build/ScenesLooksWorkspace.test.tsx: loads and displays scenes, defaulting the selection to the active scene (asserts getByLabelText(\"Scene stack\"))"
        status: pass
      - kind: static
        ref: "node scripts/design-system/check.mjs --paths src/workspaces/build/ScenesLooksWorkspace.tsx,src/workspaces/build/ScenesLooksWorkspace.module.css,src/components/SceneProgramming/SceneList.tsx,src/components/SceneProgramming/SceneList.module.css,src/components/SceneProgramming/LookBrowser.tsx,src/components/SceneProgramming/LookBrowser.module.css"
        status: pass
    human_judgment: false
  - id: D2
    description: Selected scene, exactly four layers, looks, timeline, inspector, handlers, and stable identities remain unchanged
    verification:
      - kind: unit
        ref: "ScenesLooksWorkspace.test.tsx (6 tests), SceneList.test.tsx (9 tests), LookBrowser.test.tsx (10 tests) — all pre-existing behavior assertions pass unchanged"
        status: pass
    human_judgment: false
metrics:
  duration: unavailable (span across a paused/resumed session)
  completed_date: 2026-08-03
status: complete
---

# Phase 13 Plan 11: Scene Programming Migration Summary

**SceneList, LookBrowser, and ScenesLooksWorkspace fully consume the public design-system boundary; the SceneStack pattern's tone/label mapping is corrected; scene programming and playback authority are unchanged.**

## Performance

- **Tasks:** 1/1 complete
- **Scoped design-system check:** passes with zero diagnostics
- **Focused tests:** 37/37 pass (ScenesLooksWorkspace 6, SceneList 9, LookBrowser 10, design-system pattern suite 12)

## Accomplishments

- Converted SceneList's create-form/rename inputs and rename/delete action buttons from raw `<input>`/`<button>` to `Field`/`IconButton`; converted its empty state to `EmptyState`.
- Converted LookBrowser's ~19 raw `<input>`/`<select>` controls across the theme/motion/chase/preset/blend create-forms and rename/edit rows to `Field`, preserving every existing `aria-label` string as the new visible `Field` label (verified directly against the pre-existing test file before writing the conversion, avoiding label-text regressions).
- Converted ScenesLooksWorkspace's loading paragraph, empty state, and scene-detail header to `LoadingState`, `EmptyState`, `Button`, and (initially) `Chip`.
- Fixed a real bug in the SceneStack pattern (`frontend/src/design-system/patterns/index.tsx`) inherited from the interrupted prior session: idle scenes were tagged with the "armed" (amber/warning) tone instead of "neutral", and the Chip displayed the raw lowercase tone string instead of the brand-required uppercase "LIVE" label for the active scene. Extended `SceneStack` with an optional `label` prop (defaults to the tone string, so the existing gallery fixture call site needed no change) to let callers supply the correct display text independent of tone.
- Removed the scene-detail header's now-redundant LIVE indicator for the active scene (SceneStack already shows it, ~40px away in the same viewport); the header still shows an "Activate" button when the selected scene is not yet live. This keeps the composition test's `getAllByText("LIVE")).toHaveLength(2)` assertion accurate rather than working around it.
- Registered the resizable scene-list rail's `--ds-scenelist-width` (renamed from the ungoverned `--scenelist-width`) in `design-system/runtime-geometry.json`, matching the existing `--ds-guided-first-show-stage-rail` precedent.
- Converted every remaining raw CSS custom property/literal in all three `.module.css` files to generated `--ds-*` tokens, including the same shorthand-to-longhand splits (`border`, `padding`) used in Plan 13-10.

## Task Commits

1. **Task 1: Migrate Scenes & Looks workspace, scene list, and look browser** — `4911f0d9` (WorkspaceFrame adoption, prior session), `243fc2a7` (RED SceneStack composition test, prior session), `02dccd3d` (complete conversion + SceneStack tone/label fix, this session)

## Verification

- `cd frontend && npx vitest run src/workspaces/build/ScenesLooksWorkspace.test.tsx src/components/SceneProgramming/SceneList.test.tsx src/components/SceneProgramming/LookBrowser.test.tsx` — 25 tests pass.
- `cd frontend && node scripts/design-system/check.mjs --paths src/workspaces/build/ScenesLooksWorkspace.tsx,src/workspaces/build/ScenesLooksWorkspace.module.css,src/components/SceneProgramming/SceneList.tsx,src/components/SceneProgramming/SceneList.module.css,src/components/SceneProgramming/LookBrowser.tsx,src/components/SceneProgramming/LookBrowser.module.css` — exit 0, zero diagnostics.
- `cd frontend && npx tsc --noEmit` — passes.
- `cd frontend && npx vitest run src/design-system` — 12 tests pass (SceneStack's extended `label` prop is backward compatible with the existing gallery fixture).
- `cd frontend && node scripts/design-system/check.mjs --rule DS007` — passes.
- Manual verification in a mock-bridge browser preview (`golc-desktop-frontend-dev`, port 4788): scene selection, create/rename/delete, the Activate/LIVE header transition, and every LookBrowser create-form (including the chase unit/step-duration pair after the flex-wrap layout fix) exercised with no console errors.

## Deviations from Plan

### Auto-fixed Issues

1. **[Rule 1 - Bug] Fixed SceneStack's tone/label mapping inherited from the interrupted prior session**
   - **Found during:** Task 1
   - **Issue:** `ScenesLooksWorkspace.tsx` mapped idle scenes to `status: "armed"` (misusing the warning tone) and let `SceneStack`'s Chip render the raw tone string as text, so the active scene showed lowercase "live" instead of the brand-required "LIVE".
   - **Fix:** Mapped idle scenes to `"neutral"` and added an optional `label` prop to `SceneStack` so the active scene renders `tone="live"` with text "LIVE" and idle scenes render `tone="neutral"` with the bar count as label.
   - **Verification:** `ScenesLooksWorkspace.test.tsx`'s `getAllByText("LIVE")).toHaveLength(2)` assertion (unchanged from before this plan) passes; `src/design-system` suite (12 tests) confirms the gallery fixture's existing SceneStack call site is unaffected.

2. **[Rule 1 - Bug] Removed a resulting third "LIVE" source**
   - **Found during:** Task 1
   - **Issue:** After fixing SceneStack's label, the scene-detail header's own migrated `Chip` (replacing the old raw `activeChip` span) produced a third "LIVE" text on screen, breaking the `toHaveLength(2)` assertion.
   - **Fix:** Removed the header's own LIVE chip for the active-scene case; it now renders nothing extra there (SceneStack already shows it), keeping the Activate button only for the non-active case.
   - **Verification:** `ScenesLooksWorkspace.test.tsx` full suite passes (6/6).

3. **[Rule 3 - Blocking] Registered `--ds-scenelist-width` in runtime-geometry.json**
   - **Found during:** Task 1
   - **Issue:** `grid-template-columns: var(--scenelist-width, 205px) ...` used an ungoverned custom property with an inline fallback, which DS003 flags (unknown custom property, and fallback usage outside the design-system authority).
   - **Fix:** Renamed to `--ds-scenelist-width`, removed the inline fallback (the workspace always sets it before render), and added a matching `design-system/runtime-geometry.json` entry (min 160px, max 400px, fallback 205px) mirroring the existing `--ds-guided-first-show-stage-rail` entry.
   - **Verification:** `node scripts/design-system/check.mjs --paths ...` passes; `scripts/design-system/manifest.test.ts` (7 tests) still passes.

## Known Stubs

None.

## Issues Encountered

- LookBrowser's chase-unit/step-duration and blend-duration/curve field pairs, once given visible `Field` labels, wrapped the longer label ("Chase step duration") onto two lines inside the narrow contextual-inspector drawer when forced into an equal 50/50-width row. Fixed by switching the row to `flex-wrap: wrap` with a `flex-basis` minimum instead of a plain `flex: 1` split, so a cramped pair stacks onto its own full-width rows instead of wrapping label text.

## Next Phase Readiness

Scene programming's three files are fully migrated with zero unregistered violations. The `SceneStack` pattern's `label` extension is available to any other consumer needing brand-correct status text independent of Chip tone.

## Self-Check: PASSED

- Task 1 commits `4911f0d9`, `243fc2a7`, `02dccd3d` exist.
- The plan's exact verify command passes: `npx vitest run src/workspaces/build/ScenesLooksWorkspace.test.tsx src/components/SceneProgramming/SceneList.test.tsx src/components/SceneProgramming/LookBrowser.test.tsx && node scripts/design-system/check.mjs --paths src/workspaces/build/ScenesLooksWorkspace.tsx,src/workspaces/build/ScenesLooksWorkspace.module.css,src/components/SceneProgramming/SceneList.tsx,src/components/SceneProgramming/SceneList.module.css,src/components/SceneProgramming/LookBrowser.tsx,src/components/SceneProgramming/LookBrowser.module.css`.
