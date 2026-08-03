---
phase: 13-unified-ui-design-system-and-automated-enforcement
plan: "32"
subsystem: design-system-visual-verification
tags: [playwright, screenshot-baseline, design-system, e2e, visual-regression]
requires:
  - phase: 13-17
    provides: screenshot-tolerance.json (calibrated maxDiffPixelRatio 0), screenshot.css, waitForFonts, and the calibration seed/fixture pattern this plan's captures follow
  - phase: 13-24
    provides: AppShell/TitleBar/GlobalFrame/CommandRail fully consuming generated --ds-* tokens, the shell chrome captured by this plan's persistent-shell baselines
provides:
  - "frontend/e2e/design-system.visual-shell.spec.ts: the first-ever canonical Playwright visual-regression spec for three of UI-SPEC's Required reference matrix surfaces (persistent shell, dialog layer, shared state gallery)"
  - "12 accepted baseline PNGs (3 surfaces x 2 themes x 2 widths) under frontend/e2e/design-system.visual-shell.spec.ts-snapshots/, the ground truth future Wave 9-13 sibling plans and any future re-run diff against"
  - "a reusable mask-intersection guard (PROTECTED_LANDMARK_SELECTORS/assertNoProtectedMaskIntersections) any future mask addition to this spec must pass"
affects: [13-33, 13-34, 13-41]
tech-stack:
  added: []
  patterns:
    - "A canonical baseline-acceptance spec seeds deterministic state (installHealthyBindings + a localStorage theme seed), asserts UI-SPEC's own semantic/safety contract bucket (focus, ARIA landmarks, reduced motion, overflow, safety) BEFORE calling expect(page).toHaveScreenshot(name) with no per-call options, inheriting Plan 13-17's single calibrated tolerance/stylePath/animation defaults from playwright.config.ts."
    - "Reuse existing e2e-only fixture routes (?e2e=dialog-feasibility, ?e2e=design-system-gallery) as the canonical destination for a bounded state rather than inventing new fixtures or routes -- both routes already existed for other proof purposes (Plan 13-13's packaged-dialog-proof, Plan 13-17's calibration) and compose exactly the states this plan's <behavior> named."
    - "A destructive ConfirmDialog's deliberate closeOnEscape={false}/closeOnBackdrop={false} explicit-dismissal-only policy IS the safety contract a 'destructive dialog matrix' captures, not a gap in it -- asserted directly (Escape pressed, dialog still visible, focus unmoved) rather than assumed."
    - "An empty, documented mask-rectangle set (NO_MASKS) plus a mechanically-wired but currently-no-op intersection guard (assertNoProtectedMaskIntersections against safety/navigation/live-truth/dialog-focus landmarks) satisfies a must_haves.truths clause about mask safety without inventing an unnecessary mask, and stays ready to enforce the same guarantee the moment a future edit adds one."
key-files:
  created:
    - frontend/e2e/design-system.visual-shell.spec.ts
    - frontend/e2e/design-system.visual-shell.spec.ts-snapshots/persistent-shell-light-900-win32.png
    - frontend/e2e/design-system.visual-shell.spec.ts-snapshots/persistent-shell-dark-900-win32.png
    - frontend/e2e/design-system.visual-shell.spec.ts-snapshots/persistent-shell-light-1280-win32.png
    - frontend/e2e/design-system.visual-shell.spec.ts-snapshots/persistent-shell-dark-1280-win32.png
    - frontend/e2e/design-system.visual-shell.spec.ts-snapshots/dialog-layer-light-900-win32.png
    - frontend/e2e/design-system.visual-shell.spec.ts-snapshots/dialog-layer-dark-900-win32.png
    - frontend/e2e/design-system.visual-shell.spec.ts-snapshots/dialog-layer-light-1280-win32.png
    - frontend/e2e/design-system.visual-shell.spec.ts-snapshots/dialog-layer-dark-1280-win32.png
    - frontend/e2e/design-system.visual-shell.spec.ts-snapshots/shared-gallery-light-900-win32.png
    - frontend/e2e/design-system.visual-shell.spec.ts-snapshots/shared-gallery-dark-900-win32.png
    - frontend/e2e/design-system.visual-shell.spec.ts-snapshots/shared-gallery-light-1280-win32.png
    - frontend/e2e/design-system.visual-shell.spec.ts-snapshots/shared-gallery-dark-1280-win32.png
    - .planning/phases/13-unified-ui-design-system-and-automated-enforcement/deferred-items.md
  modified:
    - frontend/playwright.config.ts
key-decisions:
  - "Set a custom top-level snapshotPathTemplate ({testDir}/{testFileDir}/{testFileName}-snapshots/{arg}{-snapshotSuffix}{ext}) to drop Playwright's default '-{projectName}' segment: default naming would have produced 'persistent-shell-light-900-chromium-win32.png', not the plan's own declared 'persistent-shell-light-900-win32.png'. Verified with a throwaway probe spec before touching the real spec; this repo only ever runs a single 'chromium' project, so the segment was pure noise. Applies repo-wide, but affects only naming of future new baselines -- no prior committed baseline PNGs existed to rename."
  - "Deferred (not fixed) a pre-existing, reproducible top-bar text-overlap regression at 900px width (Scene/Layers/Bar labels and the 'live' status chip overlapping '120 BPM'), confirmed to reproduce identically on the unmodified pre-plan worktree base via the already-committed frontend/e2e/resize.spec.ts's own 900x720 regression-anchor test -- logged to this phase's new deferred-items.md per the executor's scope-boundary rule rather than editing GlobalFrame/LiveStatusBar/TempoControls/SafetyCluster.module.css, none of which are in this plan's files_modified. The shell test therefore asserts UI-SPEC's literal 'no control/text/overlay escapes its own box or viewport' bullet (findOverflowingControls) plus landmark visibility/safety, but not the stricter zero-cross-element-overlap check that trips on this pre-existing issue."
  - "Used DialogFeasibility.tsx's existing 'blocked dialog' (ConfirmDialog titled 'Discard fixture changes?', reached via the pre-existing ?e2e=dialog-feasibility route) as the destructive-dialog-matrix subject rather than mounting a second, plan-specific ConfirmDialog instance -- it is the only ConfirmDialog reachable anywhere in the app outside its own unit tests, and its deliberate Escape/backdrop-immune policy is exactly the kind of explicit-dismissal-only safety contract a destructive confirmation should demonstrate."
requirements-completed: [D-01, D-02, D-06, D-10, D-12, D-13, UI-SPEC-VISUAL-MATRIX]
coverage:
  - id: D1
    description: "Persistent shell baseline matrix: 4 canonical screenshots (light/dark x 900/1280) of the persistent shell (identity, transport, BPM/bar, source, output health, rail, inspector, safety), each preceded by focus/ARIA-landmark/reduced-motion/overflow/safety assertions."
    requirement: "D-13"
    verification:
      - kind: e2e
        ref: "cd frontend && npx playwright test e2e/design-system.visual-shell.spec.ts --grep \"persistent shell\" --project=chromium --workers=1"
        status: pass
    human_judgment: false
  - id: D2
    description: "Destructive dialog-layer baseline matrix: 4 canonical screenshots of the existing DialogFeasibility ConfirmDialog ('Discard fixture changes?'), with direct-focus/Escape-immune/containment/return-focus assertions and persistent safety."
    requirement: "D-12"
    verification:
      - kind: e2e
        ref: "cd frontend && npx playwright test e2e/design-system.visual-shell.spec.ts --grep \"dialog layer\" --project=chromium --workers=1"
        status: pass
    human_judgment: false
  - id: D3
    description: "Shared state-gallery baseline matrix: 4 canonical screenshots of DesignSystemGallery.tsx covering empty/loading/error/disabled/selected/long-text states, each asserted via role/accessible-name/DOM-state (never color)."
    requirement: "D-02"
    verification:
      - kind: e2e
        ref: "cd frontend && npx playwright test e2e/design-system.visual-shell.spec.ts --grep \"shared gallery\" --project=chromium --workers=1"
        status: pass
    human_judgment: false
duration: "~1 session"
completed: 2026-08-03
status: complete
---

# Phase 13 Plan 32: Bounded Shell/Dialog/Gallery Visual Matrix Summary

**Accepted the first-ever canonical Playwright baseline screenshots for the persistent shell, the destructive ConfirmDialog layer, and the shared design-system state gallery -- 12 PNGs (3 surfaces x 2 themes x 2 widths), each preceded by role/ARIA/focus/overflow/safety assertions and inheriting Plan 13-17's single calibrated zero-tolerance threshold.**

## Performance

- **Duration:** ~1 session
- **Tasks:** 3/3 complete
- **Files modified:** 14 (1 spec file, 12 baseline PNGs, 1 config file, 1 new deferred-items doc)

## Accomplishments

- Added `frontend/e2e/design-system.visual-shell.spec.ts`: three `test.describe` groups ("persistent shell", "dialog layer", "shared gallery"), each parameterized over both themes (`light`/`dark`, seeded via `theme.ts`'s `golc-theme` localStorage key before navigation) and both UI-SPEC regression viewports (900/1280, height 720).
- **Persistent shell** (Task 1): navigates to `/`, seeds `installHealthyBindings`, and before each of the 4 captures asserts no unexpected initial focus, the `Workspace navigation`/`Live status bar` landmarks are visible, `prefers-reduced-motion: reduce` is actually active, `findOverflowingControls` returns empty, `assertNoRuntimeIssues`/`expectSafetyClusterAvailable` pass, and the (currently empty, documented) mask set has no protected-landmark intersection.
- **Dialog layer** (Task 2): reuses the existing `?e2e=dialog-feasibility` fixture's "blocked" `ConfirmDialog` ("Discard fixture changes?") -- the only `ConfirmDialog` mounted anywhere reachable outside its own unit tests. Asserts direct initial focus on the safe (`Keep fixture`) action, that this dialog's deliberate `closeOnEscape={false}` explicit-dismissal-only policy survives a stray Escape keypress, full viewport containment, persistent safety controls, and (after the capture) that an explicit dismissal returns focus to the trigger.
- **Shared gallery** (Task 3): reuses the existing `?e2e=design-system-gallery` fixture (`DesignSystemGallery.tsx`) and asserts all six required bounded states purely via role/accessible-name/DOM-state attributes (never color): empty (`EmptyState`'s default heading), busy (`role=status`), error (`role=alert`), disabled (`aria-disabled`), selected (`data-state="selected"`), and long-text (the deliberately long fixture name).
- Added a reusable `PROTECTED_LANDMARK_SELECTORS`/`assertNoProtectedMaskIntersections` guard satisfying `must_haves.truths`' "no mask intersects safety, navigation, live truth, or dialog focus" clause; all three surfaces currently use an empty, documented mask set (Plan 13-17's calibration found every bounded seeded state byte-identical across repeated captures, so there is no known nondeterministic pixel region to mask), and the guard is wired to mechanically enforce the same guarantee against any future mask addition.
- Confirmed determinism: ran the full 12-test suite three separate times (once via `--update-snapshots` to accept the baselines, twice more without it) with identical pass results each time.

## Task Commits

Each task was committed atomically:

1. **Task 1: Capture persistent shell matrix** - `8020ce89` (feat)
2. **Task 2: Capture destructive dialog matrix** - `2184cff0` (feat)
3. **Task 3: Capture shared state gallery matrix** - `0758e018` (feat)

**Plan metadata:** commit pending (docs: complete plan)

_Note: these tasks are `tdd="true"` calibration/acceptance work (the spec file itself is the deliverable, not a test proving separate production code), matching Plan 13-17's own precedent of one commit per task rather than a RED/GREEN split -- there is no separate feature implementation to gate._

## Files Created/Modified

- `frontend/e2e/design-system.visual-shell.spec.ts` - the three-surface visual-shell matrix spec
- `frontend/e2e/design-system.visual-shell.spec.ts-snapshots/persistent-shell-{light,dark}-{900,1280}-win32.png` - 4 accepted shell baselines
- `frontend/e2e/design-system.visual-shell.spec.ts-snapshots/dialog-layer-{light,dark}-{900,1280}-win32.png` - 4 accepted dialog baselines
- `frontend/e2e/design-system.visual-shell.spec.ts-snapshots/shared-gallery-{light,dark}-{900,1280}-win32.png` - 4 accepted gallery baselines
- `frontend/playwright.config.ts` - custom `snapshotPathTemplate` dropping the redundant `-chromium` project-name segment
- `.planning/phases/13-unified-ui-design-system-and-automated-enforcement/deferred-items.md` - new; logs the pre-existing 900px top-bar overlap discovery

## Decisions Made

See frontmatter `key-decisions`. In short: (1) a custom `snapshotPathTemplate` was required to make committed filenames match the plan's own declared names exactly; (2) a pre-existing, out-of-scope shell-chrome overlap bug at 900px was deferred rather than fixed, with the shell test's own assertions scoped to what UI-SPEC literally requires; (3) the existing `DialogFeasibility` fixture's ConfirmDialog was reused rather than adding a second one.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Added a custom `snapshotPathTemplate` to `playwright.config.ts`**
- **Found during:** Task 1, before writing any assertions
- **Issue:** Playwright's default snapshot naming interpolates `{arg}-{projectName}-{platform}{ext}`. A throwaway probe spec confirmed this repo's default behavior would produce `persistent-shell-light-900-chromium-win32.png`, not the plan's own declared `persistent-shell-light-900-win32.png` -- the twelve accepted baseline filenames could never match the plan's `files_modified` list without a config change.
- **Fix:** Added `snapshotPathTemplate: "{testDir}/{testFileDir}/{testFileName}-snapshots/{arg}{-snapshotSuffix}{ext}"` to `playwright.config.ts`, dropping only the `-{projectName}` segment (this repo only ever runs a single `chromium` project, so the segment carried no information). Verified via a throwaway probe spec (created and deleted before touching the real spec) that this produces exactly `probe-test-win32.png`.
- **Files modified:** `frontend/playwright.config.ts`
- **Verification:** All 12 generated filenames match the plan's declared list byte-for-byte; no prior committed baseline PNGs existed anywhere in the repo to be affected by the naming-template change.
- **Committed in:** `8020ce89` (Task 1 commit)

### Deferred (Logged, Not Fixed)

**1. Pre-existing top-bar text-overlap regression at 900px width**
- **Found during:** Task 1, while writing the shell surface's semantic pre-capture assertions
- **Issue:** `helpers.ts`'s `expectTopBarTextToBeReadable` fails at exactly 900x720 on Overview with `["Scene" overlaps "Layers", "Layers" overlaps "Bar", "live" overlaps "120 BPM"]`.
- **Confirmed pre-existing:** reproduces identically on the unmodified worktree base via the already-committed `frontend/e2e/resize.spec.ts`'s own "900x720 (regression anchor)" test, with zero changes from this plan applied. Not caused by, and not fixable within, this plan's declared files (`design-system.visual-shell.spec.ts` and its snapshot PNGs never touch `GlobalFrame`/`LiveStatusBar`/`TempoControls`/`SafetyCluster`).
- **Likely root cause:** Plan 13-15 deliberately grew `SafetyCluster`'s controls from 28px to `--ds-sizing-control-minimum-target` (44px) for accessibility, shrinking the horizontal budget left for `LiveStatusBar` inside `GlobalFrame`'s fixed, non-wrapping 52px header -- plausible but not independently re-verified against pre-13-15 SHAs.
- **Disposition:** logged to `.planning/phases/13-unified-ui-design-system-and-automated-enforcement/deferred-items.md` per the executor's scope-boundary rule. This plan's own shell test asserts UI-SPEC's literal "no control/text/overlay escapes its own box or viewport" bullet (`findOverflowingControls`, which does not flag this -- nothing clips past the viewport) plus landmark visibility and safety, but does not also assert the stricter zero-cross-element-overlap check that trips on this pre-existing issue. The four accepted `persistent-shell-*-win32.png` baselines therefore visually encode this overlap at 900px, matching the app's real current rendering.

---

**Total deviations:** 1 auto-fixed (1 blocking), 1 deferred (logged, not fixed)
**Impact on plan:** The config fix was necessary to satisfy the plan's own declared file names. The deferred item is a real, pre-existing shell-chrome regression outside this plan's scope; it does not block this plan's own `<done>` criteria (exactly four calibrated baselines per surface, passing this plan's own declared semantic/pixel assertions) but is worth prioritizing in a future shell-chrome pass.

## Issues Encountered

- `node_modules` was missing in this worktree at session start (a known Windows-junction issue per the orchestrator's own warning) -- resolved with a plain `npm install` inside this worktree's own `frontend/` directory.
- Playwright's default snapshot naming needed a config change (see Deviations above) before any baseline could be accepted with the plan's exact declared filenames.
- Discovered (but did not fix) a pre-existing 900px shell-chrome text-overlap regression -- see Deviations above and `deferred-items.md`.

## Next Phase Readiness

`frontend/e2e/design-system.visual-shell.spec.ts` and its 12 accepted baselines are ready as the ground truth for any future re-run of this exact spec. The mask-intersection guard (`PROTECTED_LANDMARK_SELECTORS`/`assertNoProtectedMaskIntersections`) and the `withTheme`/`settleForCapture` helpers are small, reusable patterns available to Wave 9-13 sibling plans (13-33, 13-34, 13-41) needing the same theme-seeding/settle convention. The deferred 900px shell-chrome overlap (`deferred-items.md`) should be picked up by whichever future pass gives `GlobalFrame`'s header row an explicit narrow-width strategy that accounts for `SafetyCluster`'s larger 44px control floor -- the four `persistent-shell-*-win32.png` baselines will need re-capture once that fix lands.

## Self-Check: PASSED

- All 14 declared files (`design-system.visual-shell.spec.ts`, 12 baseline PNGs, `playwright.config.ts`, `deferred-items.md`) confirmed tracked via `git ls-files`.
- All three task commit hashes (`8020ce89`, `2184cff0`, `0758e018`) confirmed present via `git log --oneline --all`.
- All three of the plan's own declared verify commands re-confirmed passing against the final committed state (`--grep "persistent shell"`, `--grep "dialog layer"`, `--grep "shared gallery"`, each 4/4).
- `npx tsc --noEmit` clean; `npx vitest run` 528/528 pass.

---
*Phase: 13-unified-ui-design-system-and-automated-enforcement*
*Completed: 2026-08-03*
