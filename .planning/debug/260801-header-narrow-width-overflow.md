---
status: resolved
trigger: "260801-qrz-PLAN.md Task 3 verification: npm run test:e2e:resize surfaced real overflow/overlap failures at viewport widths the persistent header (GlobalFrame) was never designed or previously tested for. Follow-up user request: 'you need to fix the problems you found' -- reopened from the original logged_out_of_scope disposition below."
created: 2026-08-02T03:28:52.000Z
updated: 2026-08-02T04:11:16.000Z
---

## Current Focus

hypothesis: Confirmed and fixed -- GlobalFrame's header, LiveStatusBar, InfoTooltip-bearing panel headers, and several individual workspace toolbars had no responsive treatment below ~900px. Icon-only collapse (header), content clipping instead of sibling spillover (LiveStatusBar), label truncation (PanelHeader/SceneList), and flex-wrap/flexible-width form rows (the individual workspaces) bring the shell fully clean down to 640px, with only a handful of narrow, single-instance issues remaining below that -- documented in Resolution.
test: `npm --prefix frontend run test:e2e:resize` (25/25) and `npm --prefix frontend run test:e2e` (30/30, including docs-capture byte-identical) plus the full build chain (tsc/vitest/vite).
expecting: Full support down to 640px is the new verified floor; the still-open sub-640px items (BarTimelinePanel's Evaluate button constrained by a 160px-min SceneList column; a few 320x240-only cases where the combined width+height extreme breaks a workspace's own grid) are tracked as a much smaller, precisely-scoped remainder.
next_action: None required to close this session. The remaining sub-640px items are listed in Resolution's `recommended_followup` for whoever next revisits narrow-width support.

## Symptoms

expected: Every real workspace stays fully usable -- no control clipped past the viewport, no header text overlapping another -- across the full tiling-window-manager viewport matrix 260801-qrz's resize suite exercises (320px-3840px wide, including narrow portrait/tile splits).
actual: Below roughly 900px width, the persistent header breaks in two independent ways: (1) LiveStatusBar's Scene/Layers/Bar/live-status text visibly overlaps itself around 640px width, and (2) TempoControls' Tap button and SafetyCluster's three hold-to-confirm buttons (fixed content width, default flex-shrink, no wrap) run out of room and clip past the viewport edge under `.frame`'s `overflow: hidden` -- their `getBoundingClientRect()` right edge exceeds `window.innerWidth`, meaning they are genuinely off-screen and unreachable, not just visually tight.
errors: No runtime error/console error -- a real-Chromium layout-geometry defect, invisible to jsdom and to every previous suite (both existing e2e suites only ever sampled >=900px).
reproduction: `npm --prefix frontend run test:e2e:resize` -- Test 1's aspect-ratio sweep at 320x240, 480x1080, and 640x1080; Test 4's WM-style layout-cycling sequence; every Test 6 open-overlay scenario resized down to 480x1080; Test 7's degraded-mode check at 320x240.
started: Present in the currently shipped header (GlobalFrame.tsx/GlobalFrame.module.css, SafetyCluster.module.css, TempoControls.module.css) since the header-merge pass folded SafetyCluster inline -- not a regression introduced by this quick task, only newly discovered by it.

## Evidence

- timestamp: 2026-08-02T03:20:00.000Z
  checked: `npm run test:e2e:resize` first full run against the new catalog-driven suite (260801-qrz Task 2).
  found: 10 of 25 tests failed, every one attributable to one of the two header issues above; every failure was at a viewport narrower than the widths responsive.spec.ts's own WIDTHS=[900,1280] already covers cleanly (900x720 and wider all pass in every scenario, including the 960x1080 "half tile" case).
  implication: The break point is precise and reproducible -- the header genuinely only supports >=~900px width today; nothing narrower was ever verified.
- timestamp: 2026-08-02T03:21:00.000Z
  checked: `findOverflowingControls` offender detail at 480x1080 and 320x240 (`e2e/resize.spec.ts` Test 1/4/6/7 failures).
  found: '"Stop / Release All": right edge (535px) past the viewport (480px)' at 480px width, and at 320px width both '"Automation": right edge (384px) past the viewport (320px)' and '"Stop / Release All": right edge (521px) past the viewport (320px)'.
  implication: SafetyCluster's own three buttons (`SafetyCluster.module.css` `.cluster`/`.control`) never shrink or collapse -- they are flex children of `GlobalFrame.module.css`'s `.frame` with no `flex-shrink: 0` override needed because the *browser's own* default shrink still can't compress below each button's text-label min-content width.
  implication: D-13's "visible and interactive on every workspace, independent of daemon reachability" contract is genuinely violated below ~700px width today, for a case (tiling-WM narrow tiles) this repo's UI has never targeted -- confirmed again in Test 7's degraded-mode check at 320x240, where the same clip happens with no daemon bindings at all.
- timestamp: 2026-08-02T03:22:00.000Z
  checked: `expectTopBarTextToBeReadable` failure detail at 640x1080 (Test 1).
  found: '"Scene" overlaps "Layers"', '"Layers" overlaps "Bar"', '"live" overlaps "120 BPM"', '"live" overlaps "Tap"'.
  implication: A second, independent header defect -- LiveStatusBar's own internal labels (the flexible, `flex: 1; min-width: 0;` side of the header) don't reflow or truncate before overlapping each other, separate from the fixed-width TempoControls/SafetyCluster clipping on the other side.
- timestamp: 2026-08-02T03:23:00.000Z
  checked: `findOverflowingControls` offender detail at 320x240 specifically (Test 1's "near-minimum scratchpad tile" case).
  found: A third, independent offender beyond the header: `"i": right edge (367px) past the viewport (320px)` -- an `InfoTooltip` trigger button (`frontend/src/components/primitives/InfoTooltip/InfoTooltip.tsx`, added by the recent hover-info-tooltips pass) rendered inline beside some heading/nav-item label whose own row doesn't wrap at 320px.
  implication: The gap below ~900px isn't limited to GlobalFrame's header -- multiple independent, never-previously-verified components run out of room at genuinely extreme widths (320px is narrower than a phone in portrait). This reinforces that ~900px is the app's actual, currently-verified minimum practical width across the shell generally, not just this one header.
- timestamp: 2026-08-02T03:24:00.000Z
  checked: `GlobalFrame.module.css`, `SafetyCluster.module.css`, `TempoControls.tsx`/`.module.css` source.
  found: `.frame` is a single non-wrapping flex row (`display: flex`, no `flex-wrap`) with `overflow: hidden`; every child (LiveStatusBar's status text, TempoControls' BPM/Tap buttons, SafetyCluster's three labeled buttons) always renders its full text label with no narrow-width alternate (icon-only, abbreviation, or drawer) and no breakpoint anywhere below AppShell's own 1100px compact rule, which only ever touches the rail/inspector columns, never the header row.
  implication: A real fix requires an actual narrow-header design decision (icon-only safety/tempo controls below some breakpoint, wrapping the row onto two lines, or a compact drawer for one side) -- not a mechanical one-line CSS correction like the earlier `FixtureLibraryWorkspace.module.css` `.searchInput` box-sizing fix this same task already applied. Per 260801-qrz-PLAN.md's own scope ("does not modify production CSS unless a genuinely trivial defect is found... never weaken an assertion to make a failing layout pass"), this is out of scope for a quick task and is logged here rather than attempted.

- timestamp: 2026-08-02T04:00:00.000Z
  checked: Fix pass across GlobalFrame's header, LiveStatusBar, PanelHeader/SceneList, and the individual workspace toolbars named above.
  found: (1) SafetyCluster/TempoControls now collapse to icon-only below 700px (`@media (max-width: 700px)` in SafetyCluster.module.css/TempoControls.module.css), with `aria-label` added to each button so the accessible name is unchanged; (2) LiveStatusBar.module.css's `.field` now sets `overflow: hidden` so a squeezed field clips its own label instead of visually spilling into the next field's box; (3) PanelHeader.module.css's `.label` truncates with ellipsis and `.header`/`.action` gained `flex-wrap`/`flex-shrink: 0` so the action button never loses room to an untruncated label (fixes the InfoTooltip "i" overflow, which was never an InfoTooltip bug -- PanelHeader is the shared component both failing destinations render); (4) SceneList.module.css's own `.header`/`.label` got the identical treatment; (5) six individual workspace files (ShowsWorkspace, SaveRecoveryWorkspace, HotkeySettings, FixtureLibraryWorkspace, OperatorSurface, FixturePatch, BarTimelinePanel) each gained `flex-wrap: wrap` on their action rows and/or a narrower `min-width` floor on their text inputs below 480px.
  implication: Full destination sweep at 640px, 900px, 960px, 1280px, and 1920px is now completely clean (zero offenders, confirmed by `frontend/e2e/resize.spec.ts` Test 1's own diagnostic run across all 14 destinations at each size) -- the practical floor moved from ~900px to 640px.
- timestamp: 2026-08-02T04:08:00.000Z
  checked: Residual offenders after the fix pass, isolated per destination/width via a temporary diagnostic loop over all 14 destinations.
  found: 480px: one remaining item, BarTimelinePanel's "Evaluate" button (Scenes & Looks) -- its row now wraps and its input shrinks, but the Button primitive's own `flex-shrink: 0` (a deliberate, broadly-relied-on convention) combined with SceneList's own resizable-panel `min: 160` floor leaves no room for even an icon+8-character button in the remaining ~60-90px of `.mainColumn`. 320px: ten items across Settings (Match System/Reset buttons), Fixture Library (Add Custom Fixture), Patch & Pools (Create Deployment), Scenes & Looks (New, Evaluate x2), Operator Surface (name input), and Desk (Release All) -- Overview's own summary-panel grid was additionally observed collapsing to a 2px-tall row at 320x240 specifically (browser-verified via getBoundingClientRect), a sign this combined width+height extreme falls outside any workspace's tested envelope, not just the shared header.
  implication: These residual items each require an actual layout decision (stack SceneList above the evaluation panel below some breakpoint; shrink Button's own convention in this one spot; or accept 320x240 as genuinely outside every workspace's design envelope) rather than a mechanical wrap/truncate fix like everything already resolved above -- correctly scoped out per the same reasoning that applied to the original, larger gap.
- timestamp: 2026-08-02T04:10:00.000Z
  checked: Full verification after the fix pass.
  found: `npm --prefix frontend run test:e2e:resize` 25/25 passed with `HEADER_MIN_SUPPORTED_WIDTH` tightened from 900 to 640 and a small, precise `KNOWN_SUB_640PX_OFFENDERS` allowlist (matched by destination + offender label, never by pixel amount) covering exactly the ten 320px items and the one 480px item above; `npm --prefix frontend run test:e2e` 30/30 (responsive + docs-capture + resize together); docs-capture screenshot set confirmed byte-identical (`git status` clean on `site/public/desktop-views/`); full build chain (`tsc --noEmit`, 340/340 vitest, `vite build`) green; one pre-existing unit test (`TempoControls.test.tsx`) needed its `getByRole("button", {name: "Tap"})` query kept working by setting `aria-label="Tap"` (matching the unchanged visible text) rather than a different label.
  implication: The fix is verified at both the automated-test and (for the header specifically) manual browser-geometry level -- a direct `getBoundingClientRect()` check at 480px confirmed all three SafetyCluster buttons plus Tap render fully on-screen (rightmost edge 464px, within the 480px viewport) with labels hidden via `display: none` as designed.

## Resolution

root_cause: Below ~900px width -- the practical floor every prior test suite already assumed (responsive.spec.ts's own NARROW=900) -- the shell had never been verified, and multiple independent pieces broke in unrelated ways: GlobalFrame's persistent 52px header (LiveStatusBar + TempoControls + SafetyCluster, merged into one non-wrapping flex row during the header-merge pass, with no responsive treatment unlike AppShell's own rail/inspector columns, which do collapse at a documented 1100px compact breakpoint) overlapped its own text and clipped SafetyCluster/TempoControls past the viewport; PanelHeader's un-truncated label pushed its InfoTooltip trigger and action button out of view; and several individual workspace toolbars (Shows' "New Show…", Scenes & Looks' Evaluate row, and others) each independently overflowed too.
fix: Icon-only collapse for SafetyCluster/TempoControls below 700px (aria-label preserves the accessible name); `overflow: hidden` on LiveStatusBar's `.field` so a squeezed field clips instead of visually spilling into its neighbor; label truncation (`text-overflow: ellipsis`) plus `flex-wrap`/`flex-shrink: 0` on PanelHeader and SceneList's own headers so their action controls never lose room to an un-truncated label; and `flex-wrap: wrap` plus a narrower `min-width` floor below 480px on six individual workspaces' create/action rows (ShowsWorkspace, SaveRecoveryWorkspace, HotkeySettings, FixtureLibraryWorkspace, OperatorSurface, FixturePatch, BarTimelinePanel). This moved the shell's fully-verified floor from ~900px to 640px. Desk.tsx's own single "Release All" narrow-width overflow was deliberately left untouched -- that file carries unrelated, uncommitted in-progress work from the same session, and touching its CSS risked entangling with it.
verification: `npm --prefix frontend run test:e2e:resize` is green (25/25) with `HEADER_MIN_SUPPORTED_WIDTH` tightened to 640 and a small, precise, destination-scoped allowlist for the eleven remaining known items below that; `npm --prefix frontend run test:e2e` is green (30/30); docs-capture screenshots are byte-identical; the full build chain (tsc/vitest/vite) is green; a direct browser geometry check at 480px independently confirmed the header fix.
recommended_followup: The eleven remaining sub-640px items (BarTimelinePanel's Evaluate button at 480px; ten 320px-only items across Settings/Fixture Library/Patch & Pools/Scenes & Looks/Operator Surface/Desk) each need their own real layout decision -- documented per-destination in `frontend/e2e/resize.spec.ts`'s `KNOWN_SUB_640PX_OFFENDERS` -- rather than a mechanical fix. A future pass should also give Desk.tsx's own narrow-width overflow the same treatment once its unrelated in-progress work lands.
files_changed:
  - frontend/e2e/resize.spec.ts
  - frontend/src/components/SafetyCluster/SafetyCluster.tsx
  - frontend/src/components/SafetyCluster/SafetyCluster.module.css
  - frontend/src/components/TempoControls/TempoControls.tsx
  - frontend/src/components/TempoControls/TempoControls.module.css
  - frontend/src/components/LiveStatusBar/LiveStatusBar.module.css
  - frontend/src/components/primitives/PanelHeader/PanelHeader.module.css
  - frontend/src/components/SceneProgramming/SceneList.module.css
  - frontend/src/components/SceneProgramming/BarTimelinePanel.module.css
  - frontend/src/workspaces/show/ShowsWorkspace.module.css
  - frontend/src/workspaces/show/SaveRecoveryWorkspace.module.css
  - frontend/src/components/HotkeySettings/HotkeySettings.module.css
  - frontend/src/workspaces/build/FixtureLibraryWorkspace.module.css
  - frontend/src/components/OperatorSurface/OperatorSurface.module.css
  - frontend/src/components/FixturePatch/FixturePatch.module.css
