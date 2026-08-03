---
phase: 13-unified-ui-design-system-and-automated-enforcement
plan: "10"
subsystem: fixture-patch-ui
tags: [react, design-system, fixture-patch, project-fixtures]
requires:
  - phase: 13-02
    provides: DS001-DS010 policy checker
  - phase: 13-06
    provides: Dialog primitive foundation
  - phase: 13-07
    provides: Typed primitives/patterns and public barrel
provides:
  - Fixture Library, Patch & Pools, and Project Fixtures on the public design-system boundary
  - NumberStepper primitive
affects: [scene programming migration, remaining Wave 7 workspace migrations]
tech-stack:
  added: []
  patterns: ["NumberStepper: a labeled Field-equivalent with a compact pointer +/-1 nudge affordance, used when plain typing/native keyboard stepping isn't enough"]
key-files:
  created:
    - frontend/src/components/FixturePatch/FixturePatch.test.tsx
    - frontend/src/components/ProjectFixtures/ProjectFixtures.test.tsx
    - frontend/src/components/primitives/NumberStepper/NumberStepper.tsx
    - frontend/src/components/primitives/NumberStepper/NumberStepper.module.css
    - frontend/src/components/primitives/NumberStepper/NumberStepper.test.tsx
  modified:
    - frontend/src/workspaces/build/FixtureLibraryWorkspace.tsx
    - frontend/src/components/FixturePatch/FixturePatch.tsx
    - frontend/src/components/FixturePatch/FixturePatch.module.css
    - frontend/src/components/ProjectFixtures/ProjectFixtures.tsx
    - frontend/src/components/ProjectFixtures/ProjectFixtures.module.css
    - frontend/src/design-system/index.ts
    - frontend/design-system/components.json
    - frontend/DESIGN_SYSTEM.md
key-decisions:
  - "Add a NumberStepper primitive rather than special-case ProjectFixtures' quantity/universe/address steppers: the exact same hand-rolled input+chevron-button pattern already exists in TempoControls.tsx, and the two raw chevron <button> elements could never pass a narrow DS005 exception (an exception match is only valid when it resolves to exactly one diagnostic; two visually-identical buttons in one file always resolve to two)."
  - "Drop ProjectFixtures' five-color MetaBadge intensity gradient (rename to MetaTag) rather than register six color-mix() exceptions: the fixed uppercase label and the tag's stable column position in the row's CSS-grid subgrid already disambiguate kind without a second, purely decorative signal."
  - "Adopt the Panel primitive for FixturePatch's and ProjectFixtures' outer surface, per Panel.tsx's own doc comment naming FixturePatch as a should-adopt example."
patterns-established:
  - "Replace a shorthand CSS declaration that mixes a raw literal with a --ds-* token (border, outline, transition, multi-value padding) with its longhand sub-properties so each individual declaration is exactly one var(--ds-*) token; DS001 only accepts a declaration whose entire value is a single var()/calc(var()) expression."
requirements-completed: [D-02, D-04, D-05, D-07, D-11, D-12, D-14, UI-SPEC-DATALIST, UI-SPEC-IMPACT-REVIEW]
coverage:
  - id: D1
    description: Fixture Library, Patch & Pools, and Project Fixtures use public DataList/Field/FormActions/ImpactReview contracts
    verification:
      - kind: unit
        ref: frontend/src/components/FixturePatch/FixturePatch.test.tsx
        status: pass
      - kind: unit
        ref: frontend/src/components/ProjectFixtures/ProjectFixtures.test.tsx
        status: pass
      - kind: static
        ref: "node scripts/design-system/check.mjs --paths src/workspaces/build/PatchPoolsWorkspace.tsx,src/workspaces/build/ProjectFixturesWorkspace.tsx,src/components/FixturePatch,src/components/ProjectFixtures --proposal design-system/exception-proposals/fixtures.json"
        status: pass
    human_judgment: false
  - id: D2
    description: Preview-before-apply, revision freshness, atomic dispatch, validation, and destructive safe focus remain unchanged
    verification:
      - kind: unit
        ref: "FixturePatch.test.tsx: previews and applies adding a fixture to a pool; ProjectFixtures.test.tsx: opens the add-from-library dialog and previews+applies adding a fixture"
        status: pass
    human_judgment: false
  - id: D3
    description: Only exact singular domain geometry may be proposed as an exception
    verification:
      - kind: static
        ref: frontend/design-system/exception-proposals/fixtures.json
        status: pass
    human_judgment: false
metrics:
  duration: unavailable (span across a paused/resumed session)
  completed_date: 2026-08-03
status: complete
---

# Phase 13 Plan 10: Fixture Surface Migration Summary

**Fixture Library, Patch & Pools, and Project Fixtures now consume the public design-system boundary exclusively; a new NumberStepper primitive replaces two independent hand-rolled numeric-nudge implementations; preview-before-apply and destructive-confirm behavior are unchanged and regression-tested.**

## Performance

- **Tasks:** 2/2 complete
- **Scoped design-system check:** passes with zero diagnostics and an empty exception proposal
- **Focused tests:** 13/13 pass across FixturePatch, ProjectFixtures, and NumberStepper

## Accomplishments

- Migrated Fixture Library's browse/import/preview/commit flow onto WorkspaceFrame, DataList, Field, and FormActions (Task 1, prior session).
- Migrated FixturePatch (Pools/Deployments) onto Panel, Field, Button, IconButton, EmptyState, LoadingState, ErrorState, ImpactReview, FormActions, and Chip. Fixed a duplicated impact-preview render left mid-conversion from the prior paused session, where ImpactReview's own `summary`/`impacts` props and a parallel hand-written preview list both rendered the same plan.
- Converted the remove-member preview (previously a raw `previewPanel` div) to ImpactReview as well, so both the add- and remove-member flows share one preview composition.
- Rewrote ProjectFixtures (previously fully unconverted) onto the same public contracts, replacing its bespoke focus-trap/Escape/backdrop-click dialog implementation with the shared Dialog primitive, its raw buttons/inputs/selects with Button/IconButton/Field, and its five-tint MetaBadge pill with a single-tint MetaTag (color-mix() tints without a token equivalent would have needed six narrow, un-groupable DS001 exceptions for a decorative effect the fixed label/column position already provides).
- Added the NumberStepper primitive (labeled field + compact pointer +/-1 nudge, chevrons excluded from the tab order since the field itself already supports typing and native keyboard stepping), fully wired into the design-system barrel, `components.json`, and `DESIGN_SYSTEM.md`'s generated inventory/anchors.
- Converted every remaining raw `<input>`/`<select>` in both files to the `Field` primitive, and every remaining CSS custom-property/raw-literal declaration to generated `--ds-*` tokens, including splitting `border`/`outline`/`transition`/compound `padding` shorthand into longhand sub-properties where necessary to satisfy DS001's single-token-value rule.

## Task Commits

1. **Task 1: Migrate Fixture Library browse and import** — `e08024b6` (migration), `0fd53fe9` (scoped checker `--paths`/`--proposal` support required by this plan's own verify command)
2. **Task 2: Migrate Patch & Pools and Project Fixtures** — `4911f0d9` (WorkspaceFrame adoption + token foundation, prior session), `b6eafd97` (complete FixturePatch/ProjectFixtures conversion, NumberStepper primitive, this session)

## Verification

- `cd frontend && npx vitest run src/components/FixturePatch src/components/ProjectFixtures` — 13 tests pass (4 FixturePatch, 5 ProjectFixtures, 4 NumberStepper run separately).
- `cd frontend && node scripts/design-system/check.mjs --paths src/workspaces/build/PatchPoolsWorkspace.tsx,src/workspaces/build/ProjectFixturesWorkspace.tsx,src/components/FixturePatch,src/components/ProjectFixtures --proposal design-system/exception-proposals/fixtures.json` — exit 0, zero diagnostics.
- `cd frontend && npx tsc --noEmit` — passes.
- `cd frontend && node scripts/design-system/check.mjs --rule DS007` — passes (NumberStepper inventory/barrel/guide parity).
- Manual verification in a mock-bridge browser preview (`golc-desktop-frontend-dev`, port 4788): Patch & Pools create/preview/apply-add, reassign-in-place, and Project Fixtures add-from-library dialog (open, Field/NumberStepper entry, Review Impact, Apply, Escape-to-close) all exercised with no console errors.

## Deviations from Plan

### Auto-fixed Issues

1. **[Rule 1 - Bug] Fixed a duplicated ImpactReview render inherited from the paused prior session**
   - **Found during:** Task 2
   - **Issue:** FixturePatch's add-member flow passed `summary`/`impacts` to `ImpactReview` (which renders them itself) while also keeping the old hand-written `previewHeading`/`previewList` JSX as children, rendering the same plan summary and operation list twice.
   - **Fix:** Removed the duplicated children; `ImpactReview`'s own summary/impacts rendering is now the only copy. Warnings/blockers keep their own lists as ImpactReview children since those aren't part of its props contract.
   - **Verification:** `FixturePatch.test.tsx`'s preview/apply test asserts the plan's operation text appears via `getByText`, which would fail on ambiguous duplicate matches if this regressed.

2. **[Rule 3 - Blocking] Added a NumberStepper primitive instead of a DS005 exception**
   - **Found during:** Task 2
   - **Issue:** `ProjectFixtures.tsx`'s local `NumberStepper` helper rendered a raw `<input>` plus two raw `<button>` chevrons. `IconButton`'s smallest size (32px) would visually break the compact nudge affordance, and the exception mechanism can only resolve a match to exactly one diagnostic — two near-identical chevron buttons in one file always produce two matches, making a clean narrow exception structurally impossible.
   - **Fix:** Extracted a new `NumberStepper` primitive under `src/components/primitives/` (exempt from DS005's native-control rule by location, same as `Field`), fixed its own CSS to satisfy DS001/DS006, and wired it into the barrel/inventory/guide.
   - **Verification:** `node scripts/design-system/check.mjs --paths src/components/primitives/NumberStepper` passes; `NumberStepper.test.tsx` (4 tests) passes; `check.mjs --rule DS007` passes.

## Known Stubs

- `TempoControls.tsx` (Plan 13-15-02) still hand-rolls the same input+chevron pattern `NumberStepper` now replaces here. Migrating it is that plan's own scope, not this one's; the new primitive exists for it to adopt when that plan runs.

## Issues Encountered

- The project's own `check:design-system`/`test:design-system` npm scripts referenced by `13-VALIDATION.md`'s "Quick loop" do not exist in `frontend/package.json` yet. Verification in this plan used the exact `node scripts/design-system/check.mjs ...` invocations from `13-10-PLAN.md`'s own `<verify>` blocks instead; wiring the npm scripts belongs to a later CI-integration plan.
- Even shipped primitives (e.g. `Field.module.css`) currently fail `DS001`/`DS006` when checked directly (compound `border`/`outline` shorthand, and class names like `.error` matching the DS006 keyword heuristic) — consistent with `.continue-here.md`'s existing note that whole-source parity remains expected to fail until all Wave 7 directories are migrated. This plan's own scoped paths are fully clean; no attempt was made to fix already-shipped, out-of-scope primitive files.

## Next Phase Readiness

Fixture Library, Patch & Pools, and Project Fixtures are fully migrated with zero unregistered violations in their scoped paths. `NumberStepper` is available for any later plan (Scene Programming's chase/blend step-duration fields, TempoControls' BPM spinner) that needs the same compact-nudge affordance.

## Self-Check: PASSED

- Task 1 commits `e08024b6`, `0fd53fe9` exist. Task 2 commits `4911f0d9`, `b6eafd97` exist.
- Both plan-declared verify commands pass with the exact strings from `13-10-PLAN.md`.
- `design-system/exception-proposals/fixtures.json` remains `{"schemaVersion": 1, "records": []}` — no exception was needed.
