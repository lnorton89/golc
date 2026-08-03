---
phase: 13-unified-ui-design-system-and-automated-enforcement
plan: "25"
subsystem: guided-first-show-onboarding
tags: [react, design-system, guided-first-show, onboarding, listrow, emptystate]
requires:
  - phase: 13-02
    provides: DS001-DS010 policy checker
  - phase: 13-06
    provides: Packaged-proven Dialog primitive
  - phase: 13-07
    provides: Typed primitives/patterns and public barrel
provides:
  - Guided First Show rail migrated to the ListRow primitive (real --ds-border-selected/--ds-surface-selected tokens replacing locked-sketch custom properties)
  - GuidedFirstShow.module.css fully token-driven, including the 8px guide gap (D-03 supersedes Phase 9's 7px) and the 210px rail width via pre-generated tokens
  - GuideEvidenceList empty state migrated to the shared EmptyState primitive
  - All five guide stages (Fixtures/Patch/Program/Assign/Verify) render loading/error text through shared LoadingState/ErrorState primitives
  - Two narrow, individually-verified DS001 exceptions for the locked-sketch .impact-preview rgba() values
affects: [remaining Wave 7 workspace migrations]
tech-stack:
  added: []
  patterns:
    - "A locked-sketch 'current step' selection treatment (left-border accent + tinted background) that used raw custom properties is replaced by the shared ListRow primitive's own `selected` prop rather than reproduced with new custom properties -- ListRow already implements this exact visual contract with real --ds-border-selected/--ds-surface-selected tokens."
    - "A CSS Module class whose NAME alone matches the DS006 CONTROLLED_CLASS regex (empty/loading/error/button/field/dialog/tab/toolbar/chip/badge/focus) and which sets any SHARED_VISUAL_PROPERTIES (color/background/padding/margin/border/border-radius/outline/font-*/line-height/transition/box-shadow/z-index) is a DS006 violation regardless of whether the class also does legitimate feature layout -- fixed by using the actual shared primitive (EmptyState/LoadingState/ErrorState) instead of a same-named local class."
    - "Pre-generated component-specific tokens (frontend/design-system/tokens.json's sizing.guidedFirstShowStageRail / spacing.guidedFirstShowGap) exist ahead of the migrating plan that needs them -- check tokens.generated.css for a token literally named after the component before assuming a new token or exception is needed."
key-files:
  modified:
    - frontend/src/workspaces/show/GuidedFirstShow/GuidedFirstShow.tsx
    - frontend/src/workspaces/show/GuidedFirstShow/GuidedFirstShow.module.css
    - frontend/src/workspaces/show/GuidedFirstShow/GuideEvidenceList.tsx
    - frontend/src/workspaces/show/GuidedFirstShow/stages/FixturesStage.tsx
    - frontend/src/workspaces/show/GuidedFirstShow/stages/PatchStage.tsx
    - frontend/src/workspaces/show/GuidedFirstShow/stages/ProgramStage.tsx
    - frontend/src/workspaces/show/GuidedFirstShow/stages/AssignStage.tsx
    - frontend/src/workspaces/show/GuidedFirstShow/stages/VerifyStage.tsx
  created:
    - frontend/design-system/exception-proposals/front-door.json
key-decisions:
  - "Used the ListRow primitive for the stage rail instead of the GuidedFlow pattern named in the plan's must_haves: GuidedFlow's actual implementation (frontend/src/design-system/patterns/index.tsx) renders a plain, non-interactive `<ol>` of numbered steps (title/description/state only -- no onClick, no icon slot, no aria-current support) and is out of this plan's file scope to extend. Forcing it onto the rail would either break the plan's own required behavior (click-to-select stages, aria-current=\"step\", Back/Exit Guide/primary footer, all directly asserted by GuidedFirstShow.test.tsx) or require modifying a shared pattern file used across the wave. ListRow is the codebase's actual established primitive for a selectable, icon-labeled row and already implements the exact 'current step' visual contract (left-border accent + tinted background) the locked sketch specifies, via real --ds-border-selected/--ds-surface-selected tokens instead of ad hoc custom properties."
  - "Removed --color-primary/--color-primary-soft (DS002 custom-property declarations in a non-design-system file) entirely rather than converting them to --ds-* aliases: they existed only to drive the now-deleted .guide-step styling, which ListRow's own selected state supersedes."
  - "Converted GuideEvidenceList's empty state from a local `.emptyPreview` paragraph class to the shared EmptyState primitive: the class name alone (containing 'empty') combined with its color/font-family/font-size declarations tripped DS006 ('shared visual class'), and EmptyState is exactly the primitive D-05 reserves for this case."
  - "Deleted the file's dead `.loading`/`.errorText` CSS rules (also DS006 violations by class name) -- neither was referenced anywhere; the actual loading/error text in GuidedFirstShow.tsx and every stage file renders as plain unstyled <p> with no className."
  - "Registered two narrow DS001 exceptions (design-system/exception-proposals/front-door.json) for .impact-preview's border/background rgba(200, 162, 75, ...) values instead of rounding them to a token: this rule is a verbatim reproduction of the locked Sketch 004 Variant B design (.planning/sketches/references/onboarding-readiness-impact.md), whose own header note in the pre-existing file explicitly said not to round it to a token. The two declarations have distinct alpha values (.5 border / .08 background) so they are non-identical diagnostics and each individually exceptable."
  - "Converted the five stage files' loading/error <p> tags to the shared LoadingState/ErrorState primitives (D-05) -- no test asserts their exact prior text, and this closes a real, if not checker-policed, consistency gap (bare unstyled paragraphs next to an otherwise fully token-driven shell)."
  - "Left VerifyStage's own <ul aria-label=\"Readiness summary\"> blocker/warning/evidence rows completely unconverted: GuidedFirstShow.test.tsx asserts this structure's exact text (\"0 blockers\", \"1 blocker\", \"0 warnings\", \"4 evidence items\", no progressbar role, no % sign) directly, and no inventory primitive renders this three-category singular/plural-agreeing count shape -- the locked design's own 'no combined score/progressbar' contract stays easiest to keep verifiably true in plain markup."
  - "Made no changes to GuidedFirstShowContext.tsx, readiness.ts, stages.ts, or GuidedFirstShow.test.tsx: none contain any CSS, any raw native-control JSX, or any theme-branch string, so none had a DS001-DS010 violation or a behavior gap to close. They are listed in the plan's files_modified but required no diff."
  - "13-PATTERNS.md, referenced in the plan's files_to_read as the authority for the FixtureLibraryWorkspace.tsx/workspace.module.css analog mapping, does not exist in this worktree (present only as another agent's untracked file in a different checkout). Proceeded directly from FixtureLibraryWorkspace.tsx/workspace.module.css themselves plus 13-CONTEXT.md, 13-11-SUMMARY.md, and 13-13-SUMMARY.md, all of which were available and sufficient."
patterns-established:
  - "When a plan's must_haves names a design-system pattern that doesn't actually fit the required interaction contract (no click handler, no icon slot), use the primitive that genuinely matches the need (here, ListRow) and document the substitution rather than force a bad fit or silently deviate."
requirements-completed: [D-02, D-03, D-04, D-05, D-07, D-11, D-12, D-14, UI-SPEC-GUIDED-FLOW, UI-CONSIDERATIONS]
coverage:
  - id: D1
    description: "Guided First Show shell/evidence (rail, footer, evidence aside) migrated to design-system primitives and tokens with unchanged readiness/navigation/exit behavior"
    requirement: "D-02"
    verification:
      - kind: unit
        ref: "frontend/src/workspaces/show/GuidedFirstShow/GuidedFirstShow.test.tsx (20 tests): stage rail order/aria-current, Exit Guide, Fixtures/Patch readiness, VerifyStage blocker gating and category counts, readiness derivation pure functions, GuideEvidenceList rendering"
        status: pass
      - kind: static
        ref: "node scripts/design-system/check.mjs --paths src/workspaces/show/GuidedFirstShow/GuidedFirstShow.tsx,src/workspaces/show/GuidedFirstShow/GuidedFirstShow.module.css,src/workspaces/show/GuidedFirstShow/GuidedFirstShowContext.tsx,src/workspaces/show/GuidedFirstShow/GuideEvidenceList.tsx,src/workspaces/show/GuidedFirstShow/readiness.ts,src/workspaces/show/GuidedFirstShow/stages.ts --proposal design-system/exception-proposals/front-door.json"
        status: pass
    human_judgment: false
  - id: D2
    description: "The guide gap is exactly 8px (D-03 supersedes Phase 9's 7px) and no spacing exception exists"
    requirement: "D-03"
    verification:
      - kind: static
        ref: "GuidedFirstShow.module.css uses var(--ds-spacing-guided-first-show-gap) (generated token, value 8px, frontend/design-system/tokens.json spacing.guidedFirstShowGap); zero DS001 spacing exceptions registered anywhere in front-door.json"
        status: pass
    human_judgment: false
  - id: D3
    description: "All five guide stages (Fixtures/Patch/Program/Assign/Verify) are policy-clean and behavior-unchanged"
    requirement: "D-05"
    verification:
      - kind: unit
        ref: "npx vitest run src/workspaces/show/GuidedFirstShow (20 tests, includes all five stages' readiness derivation and rendering)"
        status: pass
      - kind: static
        ref: "node scripts/design-system/check.mjs --paths src/workspaces/show/GuidedFirstShow/stages"
        status: pass
    human_judgment: false
metrics:
  duration: unavailable (single continuous session)
  completed_date: 2026-08-03
status: complete
---

# Phase 13 Plan 25: Guided First Show (Onboarding) Migration Summary

**Guided First Show's stage rail moved from a hand-rolled raw-button list to the ListRow primitive (real selected-state tokens replacing locked-sketch custom properties), the module CSS is fully token-driven including the exact 8px D-03 guide gap, and all five stages plus the evidence aside now use shared EmptyState/LoadingState/ErrorState primitives -- readiness, navigation, exit, and Perform-only blocking are all unchanged and covered by the existing 20-test suite.**

## Performance

- **Tasks:** 2/2 complete
- **Scoped design-system check:** passes with zero diagnostics (front-door.json's 2 exceptions cover `.impact-preview`'s locked-sketch rgba() border/background; every other declaration in scope converted to a real `--ds-*` token)
- **Focused tests:** 20/20 pass (`GuidedFirstShow.test.tsx`); full frontend suite 476/476 pass; `tsc --noEmit` clean; `npm run build` (tsc + vitest + vite build) clean

## Accomplishments

- Replaced the rail's raw `<button className={styles["guide-step"]}>` list with `ListRow` (icon/label/selected/onSelect/aria-current), eliminating the `--color-primary`/`--color-primary-soft` custom-property declarations and the entire `.guide-step`/`.stepIcon`/hover/focus-visible CSS block -- ListRow's own `selected` state already renders the identical left-border-accent + tinted-background treatment with real tokens.
- Converted every remaining spacing/radius/color/typography declaration in `GuidedFirstShow.module.css` to generated `--ds-*` tokens (space1/space2/space4/space6, radii-small/medium, text-primary/secondary, typography-font-family-ui/font-size-heading/font-size-body/font-weight-semibold), including two pre-generated component-specific tokens (`--ds-spacing-guided-first-show-gap` = 8px, `--ds-sizing-guided-first-show-stage-rail` = 210px) that were already seeded in `frontend/design-system/tokens.json` for exactly this migration.
- Converted `GuideEvidenceList`'s empty state from a local `.emptyPreview` class (a DS006 violation) to the shared `EmptyState` primitive, and deleted the file's dead, unreferenced `.loading`/`.errorText` rules (also DS006 by class name).
- Converted all five stage files' loading/error text to the shared `LoadingState`/`ErrorState` primitives; each stage's own description paragraph is unchanged and inherits its typography from `GuidedFirstShow.module.css`'s `.stageBody` ancestor.
- Registered two narrow, individually-verified DS001 exceptions in the new `design-system/exception-proposals/front-door.json` for `.impact-preview`'s locked-sketch `rgba(200, 162, 75, ...)` border/background values (Sketch 004 Variant B verbatim, no equivalent generated token, distinct alpha values so each is its own diagnostic).

## Task Commits

1. **Task 1: Migrate guide shell, readiness, and evidence** - `14afcbde` (feat)
2. **Task 2: Migrate all five guide stages** - `eb502786` (feat)

## Files Created/Modified

- `frontend/src/workspaces/show/GuidedFirstShow/GuidedFirstShow.tsx` - rail now composed with ListRow; Button import moved to the design-system barrel
- `frontend/src/workspaces/show/GuidedFirstShow/GuidedFirstShow.module.css` - fully token-driven; `.guide-step`/custom properties removed; `.emptyPreview`/`.loading`/`.errorText` removed
- `frontend/src/workspaces/show/GuidedFirstShow/GuideEvidenceList.tsx` - empty state uses `EmptyState` primitive
- `frontend/src/workspaces/show/GuidedFirstShow/stages/FixturesStage.tsx` - loading/error via `LoadingState`/`ErrorState`
- `frontend/src/workspaces/show/GuidedFirstShow/stages/PatchStage.tsx` - loading/error via `LoadingState`/`ErrorState`
- `frontend/src/workspaces/show/GuidedFirstShow/stages/ProgramStage.tsx` - loading/error via `LoadingState`/`ErrorState`
- `frontend/src/workspaces/show/GuidedFirstShow/stages/AssignStage.tsx` - loading/error via `LoadingState`/`ErrorState`
- `frontend/src/workspaces/show/GuidedFirstShow/stages/VerifyStage.tsx` - loading/error via `LoadingState`/`ErrorState`; readiness `<ul>` unchanged
- `frontend/design-system/exception-proposals/front-door.json` - new, 2 records

## Decisions Made

See `key-decisions` in frontmatter for the full list. The most consequential: ListRow (not GuidedFlow) for the rail, because GuidedFlow's actual shape (a static numbered-step display with no click/icon support) cannot implement the plan's own required navigation behavior without modifying a shared, out-of-scope pattern file.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Used ListRow instead of the plan's named GuidedFlow pattern for the stage rail**
- **Found during:** Task 1
- **Issue:** The plan's `must_haves` says "All five guide stages use GuidedFlow with unchanged readiness semantics," but `GuidedFlow` (`frontend/src/design-system/patterns/index.tsx`) renders a plain, non-interactive numbered-step list with no `onClick`, no icon slot, and no `aria-current` support -- it cannot implement the rail's required click-to-select/current-stage/icon behavior, all of which `GuidedFirstShow.test.tsx` asserts directly. `patterns/index.tsx` is also outside this plan's declared `files_modified` and shared across the wave, so extending it was not a safe option for a parallel worktree agent.
- **Fix:** Used the `ListRow` primitive instead -- it already implements the exact locked "current step" visual contract (left-border accent + tinted background) via real `--ds-border-selected`/`--ds-surface-selected` tokens, with native `icon`/`label`/`selected`/`onSelect` support and `aria-current` passthrough via prop spreading.
- **Files modified:** `GuidedFirstShow.tsx`, `GuidedFirstShow.module.css`
- **Verification:** All 20 `GuidedFirstShow.test.tsx` tests pass unchanged, including stage-rail order, `aria-current="step"` targeting, and Exit Guide/Back/primary navigation.

**2. [Rule 1 - Bug] Converted GuideEvidenceList's empty state and removed dead CSS to resolve DS006**
- **Found during:** Task 1
- **Issue:** `.emptyPreview` (class name matches DS006's `empty` pattern, sets `color`/`font-family`/`font-size`) and the unreferenced `.loading`/`.errorText` rules (match `loading`/`error`) were all DS006 "shared visual class" violations once this file came into scope.
- **Fix:** Replaced the empty-state paragraph with the shared `EmptyState` primitive (legacy children form, preserving the exact locked copy); deleted the two dead rules entirely (neither was referenced by any component).
- **Files modified:** `GuideEvidenceList.tsx`, `GuidedFirstShow.module.css`
- **Verification:** Scoped checker shows zero DS006 findings; `GuideEvidenceList`'s own two tests (empty state text, blocker/warning/evidence rendering) pass unchanged.

**3. [Rule 3 - Blocking] Registered two narrow DS001 exceptions for `.impact-preview`'s locked-sketch rgba() values**
- **Found during:** Task 1
- **Issue:** `.impact-preview`'s `border`/`background` rgba() values are a verbatim reproduction of the locked Sketch 004 Variant B design (the file's own pre-existing header comment already said not to round this rule to a token); no generated token expresses these exact alpha values.
- **Fix:** Registered two exceptions in the new `design-system/exception-proposals/front-door.json` (border, background -- distinct alpha values, so distinct diagnostics), each with rationale referencing the locked sketch source.
- **Files modified:** `design-system/exception-proposals/front-door.json` (new)
- **Verification:** `node scripts/design-system/check.mjs --paths ... --proposal design-system/exception-proposals/front-door.json` exits 0 with zero diagnostics.

**4. [Rule 3 - Blocking] `13-PATTERNS.md` referenced in files_to_read does not exist in this worktree**
- **Found during:** initial context load
- **Issue:** The orchestrator prompt's `files_to_read` and the plan's own `<action>` reference `13-PATTERNS.md:16-19` for the FixtureLibraryWorkspace.tsx analog mapping. This file does not exist anywhere in this worktree's `.planning/phases/13-.../` directory (git status shows it as another agent's untracked file in a different checkout, not yet committed).
- **Fix:** Proceeded directly from `FixtureLibraryWorkspace.tsx`/`workspace.module.css` (the plan's own named analogs), `13-CONTEXT.md`, `13-11-SUMMARY.md`, and `13-13-SUMMARY.md` -- all of which were present and gave sufficient precedent (token mapping table, exception-registration conventions, ListRow/EmptyState usage patterns) to complete the migration without it.
- **Files modified:** none (context-gathering deviation only)
- **Verification:** All plan verify commands pass; token/exception conventions match the already-migrated sibling plans exactly (cross-checked against `Desk.module.css`, `SceneList.module.css`, `FixtureStyleModal.module.css`).

---

**Total deviations:** 4 auto-fixed (2 Rule 1 bug fixes, 1 Rule 3 blocking exception registration, 1 Rule 3 blocking missing-context substitution)
**Impact on plan:** All deviations were necessary to satisfy the plan's own behavior-preservation and policy-clean requirements given the actual shape of the referenced pattern/context files. No scope creep -- no files outside the plan's declared `files_modified` (plus the expected new exception-proposal file) were touched.

## Issues Encountered

- This worktree had no `node_modules` installed (a fresh git worktree checkout does not carry gitignored directories). Created Windows directory junctions from this worktree's `frontend/node_modules` and repo-root `node_modules` to the main checkout's already-installed copies (`C:\Users\Lawrence\Documents\Dev\golc\{,frontend\}node_modules`) so `npx vitest`/`npx tsc`/`node scripts/design-system/check.mjs`/`npm run build` could run at all. This is a filesystem-only, non-git-tracked change (junctions do not appear in `git status`) needed purely to execute this plan's own required verification commands.

## Next Phase Readiness

Guided First Show's eight-file shell/evidence slice and all five stage files are fully migrated with zero unregistered DS001-DS010 violations. The "GuidedFlow-named-but-doesn't-fit, ListRow-is-the-real-primitive" substitution pattern documented here is available to any other still-pending Wave 7 plan whose must_haves similarly name a pattern that doesn't match its actual required interaction contract.

## Self-Check: PASSED

- Commits `14afcbde` and `eb502786` exist in `git log` and contain all 9 declared files (8 modified + 1 new).
- The plan's exact Task 1 verify command passes: `cd frontend && npx vitest run src/workspaces/show/GuidedFirstShow/GuidedFirstShow.test.tsx && node scripts/design-system/check.mjs --paths src/workspaces/show/GuidedFirstShow/GuidedFirstShow.tsx,src/workspaces/show/GuidedFirstShow/GuidedFirstShow.module.css,src/workspaces/show/GuidedFirstShow/GuidedFirstShowContext.tsx,src/workspaces/show/GuidedFirstShow/GuideEvidenceList.tsx,src/workspaces/show/GuidedFirstShow/readiness.ts,src/workspaces/show/GuidedFirstShow/stages.ts --proposal design-system/exception-proposals/front-door.json`.
- The plan's exact Task 2 verify command passes: `cd frontend && npx vitest run src/workspaces/show/GuidedFirstShow && node scripts/design-system/check.mjs --paths src/workspaces/show/GuidedFirstShow/stages`.
- `cd frontend && npx tsc --noEmit` and `cd frontend && npm run build` (tsc + full 476-test vitest suite + vite build) both pass clean.

---
*Phase: 13-unified-ui-design-system-and-automated-enforcement*
*Completed: 2026-08-03*
