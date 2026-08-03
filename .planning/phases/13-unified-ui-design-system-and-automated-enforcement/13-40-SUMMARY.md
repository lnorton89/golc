---
phase: 13-unified-ui-design-system-and-automated-enforcement
plan: "40"
subsystem: scene-programming-ui
tags: [react, design-system, scene-programming, bar-timeline, layer-row]
requires:
  - phase: 13-02
    provides: DS001-DS010 policy checker
  - phase: 13-06
    provides: Packaged-proven Dialog primitive
  - phase: 13-07
    provides: Typed primitives/patterns and public barrel
affects: [remaining Wave 7 workspace migrations]
tech-stack:
  added: []
  patterns:
    - "A dense grid-row's toggle-style control (LayerRow's per-layer enable button) uses Button's own variant prop (primary when pressed, secondary when not) plus a forwarded aria-pressed attribute, instead of a custom CSS [aria-pressed=\"true\"] override -- avoids any DS001/DS006 exception need for what is really just a state-dependent color, which Button already expresses natively."
    - "border-left/border-left-color (as opposed to bare border/border-color) are not matched by DS001's RAW_VISUAL_PROPERTIES regex, so a domain accent stripe expressed as border-left: 3px solid var(--ds-*) needs no longhand split -- only bare border/border-color/border-radius/padding/margin/gap/font-*/line-height/transition/box-shadow/outline/z-index require splitting when mixed with a raw literal."
key-files:
  modified:
    - frontend/src/components/SceneProgramming/BarTimelinePanel.tsx
    - frontend/src/components/SceneProgramming/BarTimelinePanel.module.css
    - frontend/src/components/SceneProgramming/LayerRow.tsx
    - frontend/src/components/SceneProgramming/LayerRow.module.css
key-decisions:
  - "Converted BarTimelinePanel's outer <div> to the Panel primitive (aria-label preserved) and its evaluate-position <input> to Field, moving its former aria-label text (\"Evaluate position (bar.beatfraction)\") to Field's visible label -- consistent with 13-11's precedent of promoting aria-only labels to visible Field labels during migration, and required no test changes since screen.getByLabelText matches either form."
  - "Converted LayerRow's toggle <button> to the Button primitive: variant={enabled ? \"primary\" : \"secondary\"} plus a forwarded aria-pressed attribute replaces the old custom .toggle[aria-pressed=\"true\"] CSS override entirely, eliminating that CSS rule (and its border-color/background raw-var DS001 exposure) rather than needing an exception for it."
  - "Converted LayerRow's look <select> to Field's children-cloning path (Field wraps a caller-supplied <select>), giving it a visible label (its former aria-label, e.g. \"Chase look\") instead of an aria-only one -- exact precedent match for LookBrowser's identical select conversions in 13-11."
  - "Removed the rainbow/acid data-theme-name conditional CSS blocks from LayerRow.module.css entirely (10 selector rules cycling --accent through --accent-4 by row position) rather than converting them to tokens: this is the same class of D-06 theme-name-branching violation 13-13 already resolved for Desk, and the checker's own DS004 rule (`checkCSS`'s `[data-theme(?:-name)?...]` selector test) would have flagged every one of them. Replaced with a single flat --ds-border-selected/--ds-border-default accent (enabled vs. disabled), applied identically across all four layer rows and all themes."
  - "Split every CSS declaration that mixed a raw literal with a --ds-* token in a checked shorthand property (border, padding) into longhand sub-declarations (border-width/border-style/border-color, padding-block/padding-inline) per the CONSTRAINT documented in .continue-here.md and 13-13-SUMMARY.md -- confirmed border-left/border-left-color do NOT need this split (DS001's RAW_VISUAL_PROPERTIES regex only matches bare border/border-color/border-radius, not border-left/border-left-color), so LayerRow's accent stripe kept its shorthand form."
  - "Did not add a new frontend/design-system/runtime-geometry.json entry despite the plan frontmatter's key_link naming \"typed timeline sizing\" -- neither BarTimelinePanel nor LayerRow has any user-resizable/JS-set pixel dimension (unlike Desk's fader width or the scene-list rail width); LayerRow's 48px row height and the 100px toggle-button grid column are fixed dense-grid constants, not runtime-settable geometry, so no genuine domain need existed for a new runtime-geometry entry. Documented here rather than force-adding an unneeded entry."
patterns-established: []
requirements-completed: [D-02, D-03, D-04, D-05, D-11, D-12, D-14, UI-SPEC-SCENE-STACK]
coverage:
  - id: D1
    description: Timeline and exactly four layer rows preserve stable identities, handlers, and Go-owned timing
    verification:
      - kind: unit
        ref: "frontend/src/components/SceneProgramming/BarTimelinePanel.test.tsx (4 tests) and LayerRow.test.tsx (4 tests): scene-name label, evaluate dispatch call with parsed bar position, stdout/stderr fallback rendering, toggle aria-pressed state, onToggle/onSelectLook dispatch, empty-looks placeholder -- all unchanged from before this plan and pass against the converted components"
        status: pass
      - kind: static
        ref: "node scripts/design-system/check.mjs --paths src/components/SceneProgramming/BarTimelinePanel.tsx,src/components/SceneProgramming/BarTimelinePanel.module.css,src/components/SceneProgramming/LayerRow.tsx,src/components/SceneProgramming/LayerRow.module.css"
        status: pass
    human_judgment: false
  - id: D2
    description: The specialized scene geometry plan owns exactly six concrete files
    verification:
      - kind: static
        ref: "BarTimelinePanel.tsx/.module.css/.test.tsx and LayerRow.tsx/.module.css/.test.tsx are the only files this plan modified or needed to modify; both test files required no changes since the public contract (roles, labels, dispatch calls) is unchanged"
        status: pass
    human_judgment: false
metrics:
  duration: single session
  completed_date: 2026-08-03
status: complete
---

# Phase 13 Plan 40: Scene Timeline and Layer Geometry Migration Summary

**BarTimelinePanel and LayerRow fully consume design-system primitives (Panel/Field/Button) and generated `--ds-*` tokens, replacing dead legacy `--space-*`/`--line`/`--ink`/`--accent-*` references (removed from `index.css` by Plan 13-08) and removing a D-06 theme-name-branching CSS block; evaluate/toggle/select dispatch and every existing test assertion are unchanged.**

## Performance

- **Tasks:** 1/1 complete
- **Scoped design-system check:** passes with zero diagnostics
- **Focused tests:** 8/8 pass (BarTimelinePanel 4, LayerRow 4); full `SceneProgramming` directory 27/27 pass; `ScenesLooksWorkspace.test.tsx` (composition parent) 6/6 pass

## Accomplishments

- Converted `BarTimelinePanel.tsx`'s outer `<div>` to `Panel` (preserving its `aria-label="Bar timeline preview"`) and its evaluate-position `<input>` to `Field`, promoting the former aria-only label to a visible one.
- Converted `LayerRow.tsx`'s toggle `<button>` to `Button` (`variant` tracks pressed state, `aria-pressed` forwarded) and its look `<select>` to `Field`'s children-cloning path, promoting its former aria-only label (`"{Kind} look"`) to visible text.
- Rewrote both `.module.css` files end to end: every legacy `--space-*`/`--line`/`--panel`/`--page`/`--ink`/`--muted`/`--text2`/`--text-xs`/`--text-sm`/`--radius-*`/`--accent-*` reference (all dead since Plan 13-08 removed their definitions from `index.css`) replaced with generated `--ds-*` tokens.
- Removed `LayerRow.module.css`'s 10-rule rainbow/acid `data-theme-name` conditional block (D-06 violation, cycling `--accent` through `--accent-4` by row position) and replaced it with a single flat `--ds-border-selected`/`--ds-border-default` accent stripe used identically across every theme and all four layer rows.
- Eliminated the custom `.toggle[aria-pressed="true"]` CSS override entirely by delegating pressed-state styling to `Button`'s own `variant` prop, removing that DS001 exposure rather than exceptioning it.
- Split every shorthand declaration mixing a raw literal with a `--ds-*` token (`border`, `padding`) into longhand sub-declarations (`border-width`/`border-style`/`border-color`, `padding-block`/`padding-inline`) to satisfy DS001's strict single-`var()`-value check.

## Task Commits

1. **Task 1: Migrate timeline and layer geometry** — `110b0768` — BarTimelinePanel/LayerRow full conversion to Panel/Field/Button and `--ds-*` tokens; removal of the D-06 rainbow/acid theme-branching block; longhand border/padding splits.

## Verification

- `cd frontend && npx vitest run src/components/SceneProgramming/BarTimelinePanel.test.tsx src/components/SceneProgramming/LayerRow.test.tsx` — 8/8 pass.
- `cd frontend && node scripts/design-system/check.mjs --paths src/components/SceneProgramming/BarTimelinePanel.tsx,src/components/SceneProgramming/BarTimelinePanel.module.css,src/components/SceneProgramming/LayerRow.tsx,src/components/SceneProgramming/LayerRow.module.css` — exit 0, zero diagnostics.
- `cd frontend && npx tsc --noEmit` — clean.
- `cd frontend && npx vitest run src/components/SceneProgramming` — 27/27 pass (all four files in the directory, including SceneList/LookBrowser from 13-11, unaffected).
- `cd frontend && npx vitest run src/workspaces/build/ScenesLooksWorkspace.test.tsx` — 6/6 pass (composition parent unaffected).
- `cd frontend && npx vite build` — clean; no CSS parse errors, no runtime import failures.
- Grepped both converted `.module.css` files for a literal mid-line `*/` substring — the only match is a legitimate comment's own real closing terminator, not an accidental early close.

## Deviations from Plan

### Auto-fixed Issues

1. **[Rule 2 - Missing critical functionality / D-06 correctness] Removed LayerRow's rainbow/acid theme-name-branching CSS**
   - **Found during:** Task 1
   - **Issue:** `LayerRow.module.css` had 10 selector rules keyed on `:root:is([data-theme-name="rainbow"], [data-theme-name="acid"])` cycling the left-border/pressed-toggle accent color by row position — a D-06 violation (forbidden theme-name branch) that the checker's own DS004 rule would flag on any scoped run including this file.
   - **Fix:** Removed all 10 rules; replaced with a single flat `--ds-border-selected` (enabled) / `--ds-border-default` (disabled) accent stripe applied identically regardless of active theme, matching 13-13's precedent for the same class of violation in Desk.
   - **Verification:** `node scripts/design-system/check.mjs --paths ...LayerRow.module.css` shows zero DS004 findings; visual accent behavior (colored left border indicating an enabled layer) is preserved, just no longer theme-cycled.

2. **[Rule 3 - Blocking] Split border/padding shorthands mixing a raw literal with a `--ds-*` token**
   - **Found during:** Task 1
   - **Issue:** Direct conversion of `border: 1px solid var(--line)` → `border: 1px solid var(--ds-border-default)` and `padding: var(--space-xs) var(--space-sm)` → `padding: var(--ds-spacing-space1) var(--ds-spacing-space2)` still fails DS001's strict single-`var()`-value check, per the documented CONSTRAINT in `.continue-here.md`.
   - **Fix:** Split into `border-width`/`border-style`/`border-color` and `padding-block`/`padding-inline` respectively, each holding exactly one token reference.
   - **Verification:** Scoped checker run shows zero DS001 findings for the affected declarations.

### Rejected Plan Elements

None — the plan's frontmatter `key_link` anticipated a new `runtime-geometry.json` entry ("typed timeline sizing" / pattern "timeline"), but no genuine JS-set/user-resizable pixel dimension exists in either file (LayerRow's 48px row height and 100px toggle-button grid column are fixed dense-grid constants, not runtime-settable geometry the way Desk's fader width or the scene-list rail width are). No entry was added; documented here rather than forcing an unneeded one.

## Known Stubs

None.

## Issues Encountered

- This worktree had no `frontend/node_modules` installed (a per-worktree gap, not a code issue) — resolved locally by symlinking to the main checkout's `frontend/node_modules` before running any `npx`/`vitest`/`tsc` command. This is a local tooling workaround only; no repository file was changed for it.

## Next Phase Readiness

`BarTimelinePanel.tsx`/`LayerRow.tsx` and their `.module.css` files are fully migrated with zero unregistered violations. The `Button`-drives-toggle-state pattern (variant + forwarded `aria-pressed`, no custom CSS override) is available to any other remaining Wave 7 plan with a similar dense toggle-button-in-a-grid-row need.

## Self-Check: PASSED

- Commit `110b0768` exists and contains all 4 modified files (`BarTimelinePanel.tsx`, `BarTimelinePanel.module.css`, `LayerRow.tsx`, `LayerRow.module.css`).
- The plan's exact verify command passes: `cd frontend && npx vitest run src/components/SceneProgramming/BarTimelinePanel.test.tsx src/components/SceneProgramming/LayerRow.test.tsx && node scripts/design-system/check.mjs --paths src/components/SceneProgramming/BarTimelinePanel.tsx,src/components/SceneProgramming/BarTimelinePanel.module.css,src/components/SceneProgramming/LayerRow.tsx,src/components/SceneProgramming/LayerRow.module.css`.
- `git diff --diff-filter=D --name-only HEAD~1 HEAD` shows no unintended deletions.
