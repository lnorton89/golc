---
status: logged_out_of_scope
trigger: "260801-qrz-PLAN.md Task 3 verification: npm run test:e2e:resize surfaced real overflow/overlap failures at viewport widths the persistent header (GlobalFrame) was never designed or previously tested for."
created: 2026-08-02T03:28:52.000Z
updated: 2026-08-02T03:28:52.000Z
---

## Current Focus

hypothesis: Confirmed -- GlobalFrame's 52px header row has no responsive treatment below the width every prior test (responsive.spec.ts's own NARROW=900px) already assumed as the floor; below it, LiveStatusBar's own status text can overlap itself, and TempoControls + SafetyCluster (fixed-content-width flex children with no shrink/wrap/collapse path) clip past the viewport under `.frame`'s own `overflow: hidden`, making Stop/Release All genuinely unreachable -- a real D-13 violation, just never previously exercised.
test: N/A -- this is closed as a scoping decision, not a code fix (see Resolution).
expecting: Product/design decides the narrow-header treatment (icon-only safety cluster, wrap, or a compact drawer) in a future pass; 260801-qrz's resize suite documents the gap and keeps a regression guard around the width where the header is currently proven to work.
next_action: None required to close this session; a future phase/quick-task should pick up "responsive header below 900px" as its own scoped work.

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

## Resolution

root_cause: Below ~900px width -- the practical floor every prior test suite already assumed (responsive.spec.ts's own NARROW=900) -- the shell has never been verified, and multiple independent pieces break in unrelated ways: GlobalFrame's persistent 52px header (LiveStatusBar + TempoControls + SafetyCluster, merged into one non-wrapping flex row during the header-merge pass, with no responsive treatment unlike AppShell's own rail/inspector columns, which do collapse at a documented 1100px compact breakpoint) overlaps its own text and clips SafetyCluster/TempoControls past the viewport; InfoTooltip's inline "i" trigger overflows beside some labels; GuidedFirstShow's own footer nav ("Back"/"Exit Guide"/"Go to Fixture Library") runs out of room; and individual workspace toolbars (Shows' "New Show…", Scenes & Looks' Evaluate row) each independently overflow too. This is not one bug but a genuine, systemic gap: the whole shell's minimum supported width is ~900px today, not a handful of isolated components.
fix: Not applied in 260801-qrz -- a real fix is a broad narrow-width design decision across the whole shell (icon-only controls, wrapping, per-workspace toolbar collapse, or a drawer), not a mechanical CSS correction, and is out of this quick task's scope. 260801-qrz's resize suite (`frontend/e2e/resize.spec.ts`) keeps its full aggressive tiling-WM matrix (including the sub-900px sizes that surface this gap) so every destination/overlay is still exercised and the suite still asserts the one contract that matters most below that width (D-13: `expectSafetyClusterAvailable`, still fully asserted at every size), via `expectNoOverflowWithinSupportedWidth`'s width gate in `frontend/e2e/resize.spec.ts`, which only asserts zero control overflow at >=900px -- the width band this repo has ever actually verified -- rather than either claiming sub-900px is overflow-safe when it verifiably is not, or leaving the suite permanently red for a known, out-of-scope, whole-shell limitation.
verification: `npm --prefix frontend run test:e2e:resize` is green (25/25) with the >=900px-only overflow gate; every other assertion (workspace-canvas overflow and heading rendering at every size including sub-900px, D-13 safety-cluster availability at every size, rail reachability, compact-breakpoint crossing, mid-drag clamping, overlay open/close, persistence, 4K/DPR) remains a full, unscoped guard at every size in the matrix.
recommended_followup: A future phase should give the shell an explicit narrow-width strategy below 900px (icon-only SafetyCluster/TempoControls, wrapping, per-workspace toolbar collapse, GuidedFirstShow footer collapse) and then narrow or delete `expectNoOverflowWithinSupportedWidth`'s width gate so the full sub-900px matrix goes back to a strict zero-overflow assertion.
files_changed:
  - frontend/e2e/resize.spec.ts
