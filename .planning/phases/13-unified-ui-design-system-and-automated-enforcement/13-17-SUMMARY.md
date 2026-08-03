---
phase: 13-unified-ui-design-system-and-automated-enforcement
plan: "17"
subsystem: design-system-visual-verification
tags: [playwright, screenshot-calibration, design-system, e2e, tolerance]
requires:
  - phase: 13-02
    provides: DS001-DS010 policy checker (unrelated static gate, no direct dependency but shares the design-system/ directory)
  - phase: 13-08
    provides: token layer consumed by the seeded calibration fixtures' rendered chrome
provides:
  - "frontend/design-system/screenshot-tolerance.json: the single reviewed global maxDiffPixelRatio every Wave 9-13 canonical design-system visual spec inherits by default"
  - "frontend/e2e/fixtures/designSystem.ts: reusable deterministic bounded-state fixtures (shell/gallery/dialog/text/specialized-geometry) and calibration arithmetic for later visual specs to build on"
affects: [13-30, 13-31, 13-32, 13-33, 13-34, 13-41]
tech-stack:
  added: []
  patterns:
    - "A calibration harness measures its tolerance using the exact same comparator (Playwright's own toMatchSnapshot pixelmatch-backed comparison) that will later consume the calibrated number -- never a hand-rolled diff algorithm, which would produce a technically-real but non-portable measurement."
    - "Scratch/ephemeral snapshot baselines used only to measure pairwise noise are pre-written directly to disk via testInfo.snapshotPath() + fs.writeFileSync, sidestepping toMatchSnapshot's own CI-mode-dependent auto-write-on-missing behavior, and are always deleted before the test ends -- never mistaken for canonical baselines."
    - "An e2e-only fixture route (?e2e=<name>) mounted from a single early-return branch in App.tsx is the established, reusable seam for exposing a browser-only proof surface (DialogFeasibility, now also DesignSystemGallery) without adding a second routing mechanism."
key-files:
  created:
    - frontend/e2e/fixtures/designSystem.ts
    - frontend/e2e/screenshot.css
    - frontend/e2e/design-system.calibration.spec.ts
    - frontend/design-system/screenshot-tolerance.json
    - .planning/phases/13-unified-ui-design-system-and-automated-enforcement/evidence/screenshot-calibration.json
  modified:
    - frontend/e2e/helpers.ts
    - frontend/playwright.config.ts
    - frontend/src/App.tsx
key-decisions:
  - "Calibration measures maxDiffPixelRatio using Playwright's own toMatchSnapshot comparator (an ascending candidate ladder from 0 up to the ceiling, recording the smallest passing candidate per pair) rather than a self-written pixel-diff algorithm, so the calibrated number is portable to exactly what Waves 9-13's canonical expect(page).toHaveScreenshot() calls will use."
  - "CALIBRATION_CEILING is set to 0.02 (2% of all pixels) -- a conservative, commonly cited visual-regression noise ceiling not sourced from an explicit UI-SPEC number (none exists) but documented as this plan's own reviewed, evidence-driven upper bound per the plan's own 'a configured upper bound rejects excessive noise' wording."
  - "selectedThreshold applies no invented safety margin beyond the literal measured maximum across all pairwise diffs in all five states -- must_haves.truths calls for 'the smallest stable value measured', and padding it would not be evidence-driven."
  - "The five bounded states (shell, gallery, dialog, text, specialized-geometry) share one single deterministic Wails seed (CALIBRATION_PATCH_VIEW/CALIBRATION_LIBRARY_VIEW/CALIBRATION_DESK_UNIVERSE_VALUES) reused for both the 'text' state (Fixture Library's long manufacturer/model name) and 'specialized-geometry' state (Desk fader geometry from the same patched instances) -- one source of truth rather than two independently-invented fixtures."
  - "Added a `?e2e=design-system-gallery` route to App.tsx (Rule 3 - blocking): DesignSystemGallery.tsx existed only as a Vitest-rendered fixture with no reachable browser route, so the 'gallery' bounded state (named in the plan's own <behavior>) could not be captured by a real Chromium page without it. Mirrors the existing ?e2e=dialog-feasibility seam exactly rather than inventing a second routing mechanism."
metrics:
  duration: "~1 session"
  completed_date: 2026-08-03
status: complete
---

# Phase 13 Plan 17: Screenshot-Tolerance Calibration Summary

**Built five deterministic, bounded Playwright capture fixtures (shell/gallery/dialog/long-text/Desk-fader-geometry) sharing one seeded Wails mock, then measured three independent captures of each against Playwright's own `toMatchSnapshot` comparator; every pairwise comparison was byte-identical on this machine, so the evidence-driven, no-margin-added selected tolerance is `maxDiffPixelRatio: 0`, wired as `playwright.config.ts`'s default for every later canonical design-system visual spec.**

## Performance

- **Tasks:** 2/2 complete
- **Calibration runs:** verified stable across three independent full test-suite executions (all three produced identical sha256 hashes per state and `selectedThreshold: 0`)
- **Full suite:** `npx vitest run` — 528/528 pass; `npx tsc --noEmit` — clean; `npm run build` — clean

## Accomplishments

- Added `frontend/e2e/fixtures/designSystem.ts`: one deterministic Wails seed (a long-named pool/deployment/instance pair, a 4-channel RGBW-style calibration fixture, and fixed Desk universe values) reused by two of the five bounded states; the five `CalibrationState` definitions themselves (`shell`, `gallery`, `dialog`, `text`, `specialized-geometry`); `captureState()` (seed → viewport → navigate → wait for fonts → reduced motion → `screenshot.css` → settle → `page.screenshot()`); and the full calibration arithmetic (`CALIBRATION_CEILING`, `CANDIDATE_MAX_DIFF_PIXEL_RATIOS`, `smallestPassingRatio`, scratch-snapshot read/write/cleanup helpers).
- Added `frontend/e2e/screenshot.css`, the `stylePath`/`addStyleTag` backstop for scrollbar/selection-highlight determinism that `animations: "disabled"`/`caret: "hide"` don't reach.
- Extended `frontend/e2e/helpers.ts` with `waitForFonts` (awaits `document.fonts.ready`).
- Wired `frontend/playwright.config.ts` to read `frontend/design-system/screenshot-tolerance.json` (once it exists) into `expect.toHaveScreenshot`'s defaults (`maxDiffPixelRatio`, `threshold`, `animations`, `caret`, `scale`, `stylePath`) — falls back to `maxDiffPixelRatio: 0` (pixel-exact, fail-loud) before this plan's own Task 2 has ever run.
- Added a `?e2e=design-system-gallery` route to `App.tsx` so `DesignSystemGallery.tsx` (previously reachable only inside Vitest's jsdom) is a real browser page for the "gallery" bounded state.
- Added `frontend/e2e/design-system.calibration.spec.ts`: 5 per-state deterministic-reachability tests plus one uniqueness check (Task 1), and one consolidated three-capture calibration test (Task 2) that, for each of the 5 states, captures three independent fresh-page loads, pre-writes captures 1 and 2 as scratch baselines directly to disk, runs all three pairwise comparisons (1v2, 1v3, 2v3) through an ascending `maxDiffPixelRatio` candidate ladder against Playwright's own comparator, and writes both the machine-readable evidence and the single selected tolerance file.
- Measured result: all three captures were byte-identical (`sha256`-equal) for every one of the 5 states on this machine, so `selectedThreshold: 0` — the literal smallest stable value measured, per `must_haves.truths`, with no added safety margin.

## Task Commits

1. **Task 1:** `dba7b08e` — `designSystem.ts` fixtures/seed/state-list, `screenshot.css`, `helpers.ts` (`waitForFonts`), `playwright.config.ts` (tolerance-file wiring), `App.tsx` (`?e2e=design-system-gallery`), and the Task-1 slice of `design-system.calibration.spec.ts` (5 reachability tests + 1 uniqueness test).
2. **Task 2:** `4a9fe3d9` — the three-capture calibration test appended to `design-system.calibration.spec.ts`, `frontend/design-system/screenshot-tolerance.json`, `.planning/phases/13-unified-ui-design-system-and-automated-enforcement/evidence/screenshot-calibration.json`, and a real bug fix in `designSystem.ts`'s `cleanupScratchSnapshots` found while verifying this task.

## Verification

- `cd frontend && npx playwright test e2e/design-system.calibration.spec.ts --list` — 7 tests listed (Task 1's own declared verify command), re-confirmed against the final committed state.
- `cd frontend && npx playwright test e2e/design-system.calibration.spec.ts --project=chromium --workers=1` — 7/7 pass (Task 2's own declared verify command), re-run three times total across this session, each producing identical `sha256` hashes per state and `selectedThreshold: 0`; re-confirmed against the final committed state (evidence/tolerance files reverted to their as-committed content after the final confirmation run, since the test regenerates them with a fresh timestamp on every execution).
- `cd frontend && npx tsc --noEmit` — clean.
- `cd frontend && npx vitest run` — 528/528 pass (App.tsx's new `?e2e=design-system-gallery` branch does not affect `App.smoke.test.tsx`, which never sets `location.search`).
- `cd frontend && npm run build` (`tsc --noEmit && vitest run && vite build`) — clean.
- Confirmed zero leftover untracked files after two independent full calibration runs (verifying the `cleanupScratchSnapshots` fix below actually removes the scratch snapshot directory it creates).

## Deviations from Plan

### Auto-fixed Issues

1. **[Rule 3 - Blocking] Added a `?e2e=design-system-gallery` route to `App.tsx`**
   - **Found during:** Task 1
   - **Issue:** The plan's own `<behavior>` names a "gallery" bounded state, and `DesignSystemGallery.tsx` (a fully static, self-contained fixture composition) already existed — but only ever rendered inside `DesignSystemGallery.test.tsx`'s Vitest/jsdom environment. No real browser route reached it, so Playwright had nothing to navigate to.
   - **Fix:** Added one more `globalThis.location.search === "?e2e=..."` early-return branch to `App.tsx`, mirroring the existing `?e2e=dialog-feasibility` seam exactly (same pattern, same file, same convention) rather than inventing a second routing mechanism.
   - **Verification:** `design-system.calibration.spec.ts`'s "gallery reaches its bounded destination deterministically" test passes; `App.smoke.test.tsx` (which never sets `location.search`) is unaffected; full Vitest suite and `npm run build` stay green.

2. **[Rule 1 - Bug] Fixed `cleanupScratchSnapshots` never actually deleting its scratch directory**
   - **Found during:** Task 2, while verifying no stray artifacts were left behind after a full run
   - **Issue:** `testInfo.snapshotPath("calibration-scratch")` (a single path segment) applies Playwright's default `snapshotPathTemplate`'s platform/project-name suffix to that lone segment itself — the same way it suffixes the real leaf filenames (`baseline-a-chromium-win32.png`) — so it resolved to a nonexistent `calibration-scratch-chromium-win32` path, not the real, unsuffixed `calibration-scratch/` directory every actual baseline write is nested under (`calibration-scratch/<state>/baseline-a-chromium-win32.png`). `rmSync(..., { force: true })` silently no-op'd on the nonexistent path, leaving 10 baseline PNGs (2 per state × 5 states) as untracked files after every run.
   - **Fix:** Changed `cleanupScratchSnapshots` to resolve a two-segment sentinel path (`testInfo.snapshotPath("calibration-scratch", "sentinel")`) and take its `path.dirname()` instead — this mirrors how the real 3-segment writes are resolved (only the *last* segment gets suffixed; earlier segments are literal, un-suffixed directories), so `dirname()` reliably lands on the true root regardless of what the suffixed sentinel leaf itself resolves to.
   - **Verification:** Two full, independent `--project=chromium --workers=1` runs each leave zero untracked files afterward (`git status --short` clean beyond the two intentionally-committed evidence/tolerance files).

### Rejected Plan Elements

None — both tasks were implemented as specified; the two items above are additive fixes discovered while executing, not scope changes.

## Known Stubs

None. All five bounded states render their real, seeded production code paths (Overview, `DesignSystemGallery`, the Keyboard-shortcuts help dialog, Fixture Library, Desk) — no placeholder text or hardcoded-empty UI is introduced by this plan.

## Issues Encountered

- **Measured tolerance is `0`, not a small positive number.** All three repeated captures were byte-identical (`sha256`-equal) for every one of the 5 bounded states on this development machine — a stronger determinism result than merely "close enough," and the literal, honest answer to "the smallest stable value measured from three bounded repeated Windows captures" per `must_haves.truths`. This is evidence-backed and not a shortcut, but it does carry forward risk worth flagging explicitly: if a later Wave 9-13 canonical-baseline run happens on a *different* machine or CI image (different font-cache population state, different sub-pixel rendering), a `0` tolerance could prove too strict and produce false failures unrelated to any real UI regression. The exception/`reviewCondition` pattern this phase already uses elsewhere is the correct mechanism to re-calibrate if that happens — this plan's own scope was measuring what this one canonical Windows Chromium environment actually produces, which it did.
- `node_modules` was missing in this worktree at session start (a prior wave's known Windows-junction issue, per the orchestrator's own warning) — resolved with a plain `npm install` inside this worktree's own `frontend/` directory (no junction/symlink created, nothing outside the worktree touched). Playwright's Chromium 1234 browser binary was already present in the shared `~/AppData/Local/ms-playwright` cache, so no new browser download was needed.

## Next Phase Readiness

`frontend/design-system/screenshot-tolerance.json` and `frontend/playwright.config.ts`'s `expect.toHaveScreenshot` wiring are ready for Waves 9-13's canonical baseline specs (13-30 through 13-34, 13-41): those specs can call `expect(page).toHaveScreenshot(name)` with no per-call threshold options and automatically inherit this plan's single calibrated tolerance, `screenshot.css`, `animations: "disabled"`, `caret: "hide"`, and `scale: "css"`. `frontend/e2e/fixtures/designSystem.ts`'s five `CalibrationState` definitions, `installCalibrationBindings`, and `captureState` are reusable building blocks (not calibration-only one-offs) for whichever of those plans need a seeded shell/dialog/gallery/long-text/Desk-geometry starting point.

## Self-Check: PASSED

- Commits `dba7b08e` and `4a9fe3d9` exist in `git log` and together contain all 9 declared/created files (`frontend/e2e/fixtures/designSystem.ts`, `frontend/e2e/screenshot.css`, `frontend/e2e/design-system.calibration.spec.ts`, `frontend/e2e/helpers.ts`, `frontend/playwright.config.ts`, `frontend/src/App.tsx`, `frontend/design-system/screenshot-tolerance.json`, `.planning/phases/13-unified-ui-design-system-and-automated-enforcement/evidence/screenshot-calibration.json`).
- All files listed above exist on disk at their declared paths.
- Both of the plan's own declared verify commands pass against the final committed state: `cd frontend && npx playwright test e2e/design-system.calibration.spec.ts --list` (7 tests) and `cd frontend && npx playwright test e2e/design-system.calibration.spec.ts --project=chromium --workers=1` (7/7 pass).
- `npx tsc --noEmit`, `npx vitest run` (528/528), and `npm run build` all pass clean.
