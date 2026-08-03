---
phase: 13-unified-ui-design-system-and-automated-enforcement
plan: "33"
subsystem: design-system-visual-verification
tags: [playwright, screenshot-baseline, design-system, e2e, visual-regression, authoring]
requires:
  - phase: 13-17
    provides: screenshot-tolerance.json (calibrated maxDiffPixelRatio 0), screenshot.css, waitForFonts, and the calibration seed/fixture pattern this plan's captures follow
  - phase: 13-32
    provides: design-system.visual-shell.spec.ts's withTheme/settleForCapture/mask-intersection conventions, mirrored verbatim by this plan's own spec file
provides:
  - "frontend/e2e/design-system.visual-authoring.spec.ts: the second canonical Playwright visual-regression spec for three more of UI-SPEC's Required reference matrix surfaces (Scenes & Looks, the combined Fixture Library / Patch & Pools surface, Guided First Show)"
  - "12 accepted baseline PNGs (3 surfaces x 2 themes x 2 widths) under frontend/e2e/design-system.visual-authoring.spec.ts-snapshots/"
  - "three real CSS min-width:0 truncation fixes (ScenesLooksWorkspace.module.css, patterns.module.css's .sceneStack, LookBrowser.module.css/.tsx) directly required by this plan's own long-scene-name/populated-inspector baselines"
affects: [13-34, 13-41]
tech-stack:
  added: []
  patterns:
    - "Reused design-system.visual-shell.spec.ts's exact withTheme/settleForCapture/mask-intersection-guard conventions rather than inventing a second pattern for this sibling spec."
    - "A UI-SPEC 'Required reference matrix' row that lists two workspaces together (Fixture Library / Patch & Pools) is captured as ONE composite bounded state on the single workspace (Patch & Pools) that already demonstrates both halves (populated browse + inline impact-review inspect), rather than doubling the screenshot count to 8."
    - "Seeding useResizablePanel's own localStorage-persisted width (golc.inspectorWidth) is a legitimate, user-reachable way to give a narrow resizable column enough room for a deterministic capture -- not a hack, since any real user dragging the handle reaches the identical state."
    - "page.waitForFunction polling for an actual layout property (here: the resizable inspector aside's rendered width) is more robust than a fixed settle timeout when a CSS transition has no prefers-reduced-motion override -- emulateMedia alone does not skip such a transition."
key-files:
  created:
    - frontend/e2e/design-system.visual-authoring.spec.ts
    - frontend/e2e/design-system.visual-authoring.spec.ts-snapshots/scenes-looks-{light,dark}-{900,1280}-win32.png
    - frontend/e2e/design-system.visual-authoring.spec.ts-snapshots/fixtures-patch-{light,dark}-{900,1280}-win32.png
    - frontend/e2e/design-system.visual-authoring.spec.ts-snapshots/guided-first-show-{light,dark}-{900,1280}-win32.png
  modified:
    - frontend/src/workspaces/build/ScenesLooksWorkspace.module.css
    - frontend/src/design-system/patterns/patterns.module.css
    - frontend/src/components/SceneProgramming/LookBrowser.module.css
    - frontend/src/components/SceneProgramming/LookBrowser.tsx
    - .planning/phases/13-unified-ui-design-system-and-automated-enforcement/deferred-items.md
key-decisions:
  - "Fixed three separate, real, pre-existing CSS truncation bugs (all the same class: a flex/grid item missing min-width: 0, silently defeating an already-declared overflow: hidden/text-overflow: ellipsis) rather than working around them, because each one directly blocked this plan's own must_haves.truths (the long scene name overlapped the LIVE chip and the row beneath it; LookBrowser's populated rows bled action buttons past the resizable inspector column). None of the three fixes changes any component's public API or behavior contract -- purely a layout-containment correction."
  - "Diagnosed an intermittent 8-45px overflow as a real timing race, not a flaky assertion: AppShell's inspector-column grid-template-columns transition (--ds-motion-settle, 200ms) has no prefers-reduced-motion override, so a fixed capture-wait could sample mid-transition. Replaced the guess with page.waitForFunction polling the aside's actual rendered width."
  - "Captured Task 2's UI-SPEC row (Fixture Library / Patch & Pools, one combined matrix entry) as a single bounded state on Patch & Pools alone -- its own FixturePatch.tsx already composes populated pool/deployment browsing with an inline impact-review inspect panel, so mocking AddPoolMemberPreview to a plan carrying both a warning and a blocker demonstrates the full required truth without a second surface or screenshot pair."
  - "Seeded useResizablePanel's golc.inspectorWidth to 360px (within its own documented 220-480 legitimate range) for the Scenes & Looks capture -- headroom for the Looks panel's count-summary line, not a workaround for the truncation bugs above (those are fixed directly in CSS)."
requirements-completed: [D-01, D-02, D-03, D-06, D-10, D-12, D-14, UI-SPEC-VISUAL-MATRIX]
coverage:
  - id: D1
    description: "Scenes & Looks baseline matrix: 4 canonical screenshots (light/dark x 900/1280) of a populated Scene Stack with one scene carrying all four fixed layers enabled and a deliberately long name, preceded by role/layer-count/selection/inspector/timeline/containment/focus assertions."
    requirement: "UI-SPEC-VISUAL-MATRIX"
    verification:
      - kind: e2e
        ref: "cd frontend && npx playwright test e2e/design-system.visual-authoring.spec.ts --grep \"Scenes & Looks\" --project=chromium --workers=1"
        status: pass
    human_judgment: false
  - id: D2
    description: "Fixture Library / Patch & Pools baseline matrix: 4 canonical screenshots of Patch & Pools' populated pool/deployment browsing plus an inline impact-review panel carrying both a warning and a blocker, Apply disabled."
    requirement: "UI-SPEC-VISUAL-MATRIX"
    verification:
      - kind: e2e
        ref: "cd frontend && npx playwright test e2e/design-system.visual-authoring.spec.ts --grep \"Fixture Library / Patch & Pools\" --project=chromium --workers=1"
        status: pass
    human_judgment: false
  - id: D3
    description: "Guided First Show baseline matrix: 4 canonical screenshots of the Verify stage's rolled-up mixed blocker/warning/evidence state, with Perform (the dominant next action) disabled by the outstanding blocker, and the 8px D-03 grid gap verified."
    requirement: "UI-SPEC-VISUAL-MATRIX"
    verification:
      - kind: e2e
        ref: "cd frontend && npx playwright test e2e/design-system.visual-authoring.spec.ts --grep \"Guided First Show\" --project=chromium --workers=1"
        status: pass
    human_judgment: false
duration: "~1 session"
completed: 2026-08-03
status: complete
---

# Phase 13 Plan 33: Bounded Authoring Visual Matrix Summary

**Accepted the second canonical Playwright baseline set -- 12 PNGs (Scenes & Looks, the combined Fixture Library / Patch & Pools surface, and Guided First Show, each at 2 themes x 2 widths) -- and fixed three real pre-existing CSS truncation bugs plus a real capture-timing race directly blocking this plan's own long-scene-name and populated-inspector requirements.**

## Performance

- **Duration:** ~1 session
- **Tasks:** 3/3 complete
- **Files modified:** 17 (1 spec file, 12 baseline PNGs, 4 CSS/TSX fix files) + 1 deferred-items.md log entry

## Accomplishments

- Added `frontend/e2e/design-system.visual-authoring.spec.ts`, mirroring Plan 13-32's `design-system.visual-shell.spec.ts` conventions (`withTheme`, `settleForCapture`, the mask-intersection guard) rather than inventing a second pattern.
- **Scenes & Looks** (Task 1): seeded one active scene with all four fixed layers (base_look/color_theme/chase/motion) enabled and pointed at real looks, with a deliberately long name (UI-SPEC's long-text backstop). Asserted the populated Scene Stack, exactly-four layer count, selection (`aria-pressed`/`data-state`), the LookBrowser inspector's populated counts, the bar-timeline evaluation panel, containment, and focus before each capture.
- **Fixture Library / Patch & Pools** (Task 2): UI-SPEC's Required reference matrix lists this as one combined row -- captured as a single bounded state on Patch & Pools' own `FixturePatch.tsx`, which already composes populated pool/deployment browsing with an inline "review impact before apply" panel. Mocked `AddPoolMemberPreview` to a plan carrying both a warning and a blocker, demonstrating the full required warning/blocker truth (Apply disabled) without inventing a second surface.
- **Guided First Show** (Task 3): seeded one blocker (no pools), one warning (a fixture-library validation failure), and multiple evidence rows, then navigated to the Verify stage, which rolls every prior stage's derived readiness into one flattened list -- producing the required "mixed blocker/warning/evidence state" with Perform (the guide's one dominant next action) visibly disabled.
- Confirmed determinism: ran the full 12-test suite 3+ separate times consecutively with identical pass results.

## Task Commits

Each task was committed atomically:

1. **Task 1: Capture Scenes & Looks matrix** - `8cf81ce4` (feat)
2. **Task 2: Capture Fixture Library and Patch & Pools matrix** - `2da3b3f5` (feat)
3. **Task 3: Capture Guided First Show matrix** - `097337e7` (feat)

**Plan metadata:** commit pending (docs: complete plan)

_Note: these tasks are `tdd="true"` calibration/acceptance work (the spec file itself is the deliverable, not a test proving separate production code), matching Plan 13-17/13-32's own precedent of one commit per task rather than a RED/GREEN split. The spec file grew incrementally across the three commits (Task 1's commit contains only its own describe block; Task 2's commit appends its block; Task 3's commit appends the final block), mirroring 13-32's own git history shape exactly._

## Files Created/Modified

- `frontend/e2e/design-system.visual-authoring.spec.ts` - the three-surface authoring matrix spec
- `frontend/e2e/design-system.visual-authoring.spec.ts-snapshots/scenes-looks-{light,dark}-{900,1280}-win32.png` - 4 accepted Scenes & Looks baselines
- `frontend/e2e/design-system.visual-authoring.spec.ts-snapshots/fixtures-patch-{light,dark}-{900,1280}-win32.png` - 4 accepted Fixture Library / Patch & Pools baselines
- `frontend/e2e/design-system.visual-authoring.spec.ts-snapshots/guided-first-show-{light,dark}-{900,1280}-win32.png` - 4 accepted Guided First Show baselines
- `frontend/src/workspaces/build/ScenesLooksWorkspace.module.css` - `min-width: 0` on `.sceneHeader`/`.sceneName` (long scene name truncation fix)
- `frontend/src/design-system/patterns/patterns.module.css` - `.sceneStack` split into its own rule with `grid-template-columns: minmax(0, 1fr)` (same truncation-defeating implicit-grid-track bug, scoped away from `DataList`/`LauncherMasters`/`GuidedFlow`/`ImpactReview`)
- `frontend/src/components/SceneProgramming/LookBrowser.module.css` - `min-width: 0` on `.lookRow`, new `.lookName` class (truncation) applied to the five theme/chase/motion/preset/blend name spans
- `frontend/src/components/SceneProgramming/LookBrowser.tsx` - applies the new `.lookName` className to its five look-name spans
- `.planning/phases/13-unified-ui-design-system-and-automated-enforcement/deferred-items.md` - logs a pre-existing, out-of-scope `VerifyStage.tsx` copy/spacing nit found while reviewing the Guided First Show captures

## Decisions Made

See frontmatter `key-decisions`. In short: (1) fixed three real pre-existing `min-width: 0` truncation bugs directly blocking this plan's own required states, rather than working around them; (2) diagnosed and fixed a genuine capture-timing race (AppShell's inspector-column transition has no reduced-motion override) via `page.waitForFunction` instead of a longer guessed timeout; (3) captured Task 2's combined UI-SPEC matrix row as one bounded state on Patch & Pools alone; (4) seeded a legitimate, user-reachable resizable-panel width via localStorage for headroom, separate from the CSS fixes.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `.sceneHeader`/`.sceneName` missing `min-width: 0` defeated ellipsis truncation for long scene names**

- **Found during:** Task 1, while visually reviewing the first accepted `scenes-looks-*` captures.
- **Issue:** The selected scene's own heading (`ScenesLooksWorkspace.module.css`'s `.sceneName`, a flex item of `.sceneHeader`) already declared `overflow: hidden; text-overflow: ellipsis; white-space: nowrap`, but with no `min-width: 0` a flex item's automatic minimum width defaults to its unwrapped content size -- silently defeating the truncation. A 120+ character scene name (this task's own long-text backstop) rendered at full width, visually overlapping the LIVE chip and the layer row beneath it.
- **Fix:** Added `min-width: 0` to both `.sceneHeader` (itself a grid item of `.mainColumn`'s own grid, subject to the identical implicit-minimum-size behavior one level up) and `.sceneName`.
- **Files modified:** `frontend/src/workspaces/build/ScenesLooksWorkspace.module.css`
- **Verification:** Visually re-confirmed via the regenerated `scenes-looks-*-win32.png` captures -- the long name now ellipsis-truncates cleanly with no overlap.
- **Committed in:** `8cf81ce4` (Task 1 commit)

**2. [Rule 1 - Bug] `patterns.module.css`'s `.sceneStack` had no `grid-template-columns`, defeating the identical truncation for the Scene Stack readout row**

- **Found during:** Task 1, same visual review pass.
- **Issue:** `.sceneStack { display: grid; gap: ...}` (shared with `.dataList`/`.masterList`/`.guidedFlow`/`.impactReview` in one combined rule) declares no explicit column track, so its single implicit "auto" column sizes to its widest child's min-content -- the exact same truncation-defeating bug, one level up from the fix above, now affecting the Scene Stack's own status-chip row rather than the workspace's own scene-name heading.
- **Fix:** Split `.sceneStack` out of the shared selector into its own rule with `grid-template-columns: minmax(0, 1fr)`, scoped narrowly so the fix does not touch `DataList`/`LauncherMasters`/`GuidedFlow`/`ImpactReview` (none of which were independently verified against this same fix).
- **Files modified:** `frontend/src/design-system/patterns/patterns.module.css`
- **Verification:** Visually re-confirmed -- the Scene Stack row now truncates with a visible "LIVE" chip instead of bleeding across the full window width.
- **Committed in:** `8cf81ce4` (Task 1 commit)

**3. [Rule 1 - Bug] `LookBrowser.module.css`'s `.lookRow` and its unstyled name span both lacked `min-width: 0`, letting populated rows bleed action buttons past the resizable inspector column**

- **Found during:** Task 1, debugging an intermittent `findOverflowingControls` failure ("Delete Full Wash" measuring past the viewport) that did not resolve even at the resizable inspector's documented maximum width (480px) -- ruling out "insufficient inspector width" as the actual cause.
- **Issue:** `.lookRow` (a flex item of `.rows`' column flex container) had no `min-width: 0`, and its unstyled `<span title={...}>{...}</span>` name element (the only one of its three children actually able to shrink; the kind label and the rename/delete action group are both non-shrinking) also had no `min-width: 0` or truncation styling. `ContextualInspector.module.css`'s own `.inspector` declares only `overflow-y` (never `overflow-x`), so the row's un-shrinkable content simply rendered past the column's right edge with nothing to clip it.
- **Fix:** Added `min-width: 0` to `.lookRow`; added a new `.lookName` class (`min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap`) applied to all five theme/chase/motion/preset/blend name spans in `LookBrowser.tsx`.
- **Files modified:** `frontend/src/components/SceneProgramming/LookBrowser.module.css`, `frontend/src/components/SceneProgramming/LookBrowser.tsx`
- **Verification:** `findOverflowingControls` returns `[]` consistently across 8+ repeated runs post-fix (0/8 failures), versus reproducing on nearly every run beforehand at both a 258px default and a 480px maximum resizable width.
- **Committed in:** `8cf81ce4` (Task 1 commit)

**4. [Rule 3 - Blocking] Test-level timing race: AppShell's inspector-column transition has no reduced-motion override**

- **Found during:** Task 1, while root-causing why fix #3 above still intermittently failed even at maximum inspector width.
- **Issue:** `AppShell.module.css`'s `.appShell` animates `grid-template-columns` over `--ds-motion-settle` (200ms) whenever `ContextualInspector.tsx`'s async `MutationObserver` flips `inspectorHasContent` true -- this transition has no `@media (prefers-reduced-motion: reduce)` override, so `page.emulateMedia({ reducedMotion: "reduce" })` alone does not skip it. A fixed 250ms settle wait could occasionally sample mid-transition if the `MutationObserver` fired late relative to test start, yielding a transiently narrower-than-settled inspector column and real (if temporary) content overflow.
- **Fix:** Replaced the assumption with an explicit `page.waitForFunction` polling the inspector aside's actual rendered width against the seeded target, only above the compact-width breakpoint where the column is not `display: none`.
- **Files modified:** `frontend/e2e/design-system.visual-authoring.spec.ts` (test-only; no production fix -- the underlying missing reduced-motion override in `AppShell.module.css` is a separate, out-of-scope polish item not required to pass this plan's own baselines once the test itself waits correctly)
- **Verification:** 5+ consecutive full-suite runs, 0 failures, versus reproducing on most runs beforehand.
- **Committed in:** `8cf81ce4` (Task 1 commit)

### Deferred (Logged, Not Fixed)

**1. `VerifyStage.tsx`'s readiness-summary rows render their tone label and count with no separating space**

- **Found during:** Task 3, while visually reviewing the accepted `guided-first-show-*` captures.
- **Issue:** `<li><span>Blocker</span><span>{pluralize(...)}</span></li>` renders as `Blocker1 blocker` with no visible gap between the tone word and its count.
- **Confirmed pre-existing, not caused by this plan:** `VerifyStage.tsx` is not in this plan's declared files; the crowding is data-independent (a markup/CSS gap, not an artifact of this plan's specific mocked rollup).
- **Disposition:** logged to `.planning/phases/13-unified-ui-design-system-and-automated-enforcement/deferred-items.md`. Purely cosmetic -- every semantic assertion this plan makes (`toContainText("1 blocker")` etc.) passes regardless of surrounding whitespace. The accepted baselines visually encode this crowding, matching the app's real current rendering.

---

**Total deviations:** 4 auto-fixed (3 bugs, 1 blocking), 1 deferred (logged, not fixed)
**Impact on plan:** All four fixes were necessary to satisfy this plan's own `<done>` criteria (four calibrated baselines per surface, passing this plan's own declared semantic and containment assertions) and to make the resulting 12-test suite genuinely deterministic rather than intermittently flaky. None changes any component's public API, and the design-system checker (`node scripts/design-system/check.mjs --paths ...`) reports zero violations against all four modified files. The deferred item is a real, pre-existing, purely cosmetic gap outside this plan's scope.

## Issues Encountered

- `node_modules` was missing in this worktree at session start (known Windows-junction issue) -- resolved with a plain `npm install` inside this worktree's own `frontend/` directory.
- Extensive debugging was required to distinguish a genuine CSS truncation bug (fixable, in-scope per Rule 1) from a resizable-panel-width tuning question -- ultimately traced to three separate `min-width: 0` omissions plus one real test-timing race, all now fixed/corrected. See Deviations above for the full root-cause chain.
- Discovered (but did not fix) a pre-existing, purely cosmetic `VerifyStage.tsx` spacing nit -- see Deviations above and `deferred-items.md`.

## Next Phase Readiness

`frontend/e2e/design-system.visual-authoring.spec.ts` and its 12 accepted baselines are ready as ground truth for any future re-run. The three `min-width: 0` fixes are general-purpose correctness fixes (not narrowly scoped workarounds) available to any future plan touching `ScenesLooksWorkspace`, the shared `SceneStack` pattern, or `LookBrowser`. The deferred `VerifyStage.tsx` spacing nit and the (separately noted, not this plan's responsibility) missing `prefers-reduced-motion` override on `AppShell.module.css`'s inspector-column transition are both available for a future Guided First Show / shell-chrome polish pass.

## Self-Check: PASSED

- All 17 declared files (`design-system.visual-authoring.spec.ts`, 12 baseline PNGs, 4 CSS/TSX fix files) confirmed tracked via `git status --short` (clean tree) and `git log`.
- All three task commit hashes (`8cf81ce4`, `2da3b3f5`, `097337e7`) confirmed present via `git log --oneline -5`.
- All three of the plan's own declared verify commands re-confirmed passing against the final committed state (`--grep "Scenes & Looks"`, `--grep "Fixture Library / Patch & Pools"`, `--grep "Guided First Show"`, each 4/4).
- `npx tsc --noEmit` clean; `npx vitest run` 528/528 pass; `node scripts/design-system/check.mjs --paths <the 4 modified files>` exits 0 (no violations).
- Full 12-test suite re-confirmed passing 3+ consecutive times on the final committed state.

---
*Phase: 13-unified-ui-design-system-and-automated-enforcement*
*Completed: 2026-08-03*
