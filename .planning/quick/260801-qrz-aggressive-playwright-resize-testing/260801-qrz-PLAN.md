---
phase: quick-window-resize-responsiveness-testing
plan: 260801-qrz
type: execute
wave: 1
depends_on: []
files_modified:
  - frontend/e2e/helpers.ts
  - frontend/e2e/resize.spec.ts
  - frontend/e2e/responsive.spec.ts
  - frontend/e2e/desktop-view-docs.spec.ts
  - frontend/package.json
autonomous: true
requirements: []
must_haves:
  truths:
    - "The existing responsive suite's hardcoded twelve-destination list is already stale: `frontend/src/shell/desktopViews.json` now declares fourteen routable destinations including Project Fixtures and Desk, so the resize suite must derive destinations from that catalog instead of a second hardcoded list."
    - "Tiling-window-manager users produce discrete, extreme, and rapid viewport changes -- tall/narrow columns, short/wide rows, half/third-width tiles, near-minimum scratchpad windows, 4K maximized windows, fast layout-cycling sequences, and resizes while dialogs/overlays or in-progress panel drags are open -- that the current two fixed-width, fixed-height samples never exercise."
    - "Responsiveness is judged by real rendered geometry (bounding boxes, scroll/clamp overflow, computed grid columns) asserted in Chromium -- never by jsdom, which has no layout engine -- and never by pixel baselines, matching the existing frontend e2e convention and the repository's split that only the `site/` submodule owns visual snapshots."
    - "The shell contract that must survive every resize is D-13's: the command rail, live truth, and safety cluster remain visible and interactive, and the workspace canvas scrolls internally instead of the window overflowing horizontally."
    - "The 1100px compact breakpoint collapses the inspector and caps the rail with `min(var(--rail-width), 160px)`; crossing it in either direction, and repeatedly, must not strand inline `--inspector-width`/`--rail-width` values or leave a stuck grid transition."
    - "Refactoring the shared Wails mocks, settle, destinations, and geometry guards into one `e2e/helpers.ts` must not change the docs capture suite's behavior: the published desktop-view screenshots remain byte-identical."
  artifacts:
    - path: frontend/e2e/helpers.ts
      provides: "Single shared harness: catalog-derived destinations, settle(), healthy Wails bindings, runtime-issue guards, top-bar readability guard, geometry/overflow/safety-cluster checks."
    - path: frontend/e2e/resize.spec.ts
      provides: "Catalog-driven aggressive window-resize suite covering the tiling-WM viewport matrix and dynamic resize scenarios."
    - path: frontend/package.json
      provides: "Focused `test:e2e:resize` script using the already-installed Playwright Chromium."
  key_links:
    - from: frontend/e2e/helpers.ts
      to: frontend/src/shell/desktopViews.json
      via: "catalog navLabels drive the destination sweep instead of a hardcoded list"
      pattern: "desktopViews"
    - from: frontend/e2e/resize.spec.ts
      to: frontend/e2e/helpers.ts
      via: "every scenario imports shared settle/bindings/geometry guards"
      pattern: "helpers"
    - from: frontend/src/shell/AppShell.module.css
      to: frontend/e2e/resize.spec.ts
      via: "the <=1100px compact-breakpoint rule is the subject of the boundary-crossing scenarios"
      pattern: "compact breakpoint"
    - from: frontend/src/shell/AppShell.tsx
      to: frontend/e2e/resize.spec.ts
      via: "inline --rail-width/--inspector-width drive the grid columns under test"
      pattern: "useResizablePanel"
    - from: frontend/src/components/SafetyCluster/SafetyCluster.tsx
      to: frontend/e2e/resize.spec.ts
      via: "D-13 availability assertions target the always-mounted safety cluster"
      pattern: "SafetyCluster"
---

<objective>
Add an aggressive Playwright window-resize responsiveness and adaptability suite for the GOLC desktop UI that exercises the aspect-ratio extremes, rapid discrete resize sequences, breakpoint crossings, overlay/dialog resizes, and degraded-mode states that tiling-window-manager users routinely hit -- while keeping the suite on the existing real-browser geometry-test conventions (catalog-driven destinations, motion-settle waits, layout-engine assertions, no pixel baselines) and sharing harness code with the existing responsive and screenshot suites.

Purpose: The existing responsive.spec.ts samples two fixed widths at one fixed height, so it cannot see vertical resizes, extreme aspect ratios, resize sequences (the tiling-WM case), compact-breakpoint crossings, resize-while-open surfaces, or mid-drag outer resizes. This plan closes that coverage gap and makes the suite maintainable by deriving destinations from the versioned catalog.
Output: A shared e2e harness, a catalog-driven resize suite, a focused npm script, both existing e2e specs refactored onto the harness with unchanged behavior, and a verification run that triages any genuine defects it surfaces.
</objective>

<execution_context>
@C:/Users/Lawrence/.codex/gsd-core/workflows/execute-plan.md
@C:/Users/Lawrence/.codex/gsd-core/templates/summary.md
</execution_context>

<context>
@AGENTS.md
@.planning/STATE.md
@.planning/ROADMAP.md
@.planning/sketches/SKILL.md
@.planning/debug/screenshot-viewport-text-overlap.md
@frontend/playwright.config.ts
@frontend/e2e/responsive.spec.ts
@frontend/e2e/desktop-view-docs.spec.ts
@frontend/src/shell/desktopViews.json
@frontend/src/shell/navigation.ts
@frontend/src/shell/AppShell.tsx
@frontend/src/shell/AppShell.module.css
@frontend/src/hooks/useResizablePanel.ts
@frontend/src/components/primitives/ResizeHandle/ResizeHandle.tsx
@frontend/src/components/SafetyCluster/SafetyCluster.tsx
@frontend/src/shell/useGlobalKeyboardWorkflow.ts
@frontend/package.json

The suite runs through the existing `frontend/playwright.config.ts` (Chromium, baseURL port 4790, `npm run test:e2e`), which its own doc comment deliberately keeps outside `npm test`/`npm run build` because a real browser needs its downloaded binary and real seconds. Do not move the e2e suite into the fast unit/build chain.

The catalog is authoritative. `navigation.ts` projects `NAV_GROUPS` from `desktopViews.json`, and the docs capture suite already clicks nav buttons by `navLabel`. The responsive spec's hardcoded twelve labels are stale (missing `Project Fixtures` and `Desk`), so the resize suite must not replicate that drift; the shared helper must derive the destination labels from the catalog and the refactored responsive sweep must adopt them.

Reuse the deterministic healthy Wails mocks from `desktop-view-docs.spec.ts` so every workspace renders its content-rich state. Also test the daemon-unreachable fallback, because a tiling-WM operator can hit it and D-13 still requires the safety cluster to be visible and interactive regardless of daemon reachability.

The shell animates `grid-template-columns` over `--motion-settle` (AppShell.module.css) and resizable panels clamp/persist via `useResizablePanel` (rail [160,360] under `golc.railWidth`, inspector [220,480] under `golc.inspectorWidth`). Every resize must be followed by the existing `settle()` wait before measuring, and sequence tests must assert after each step, not only at the end.

The `role="separator"` handles are reachable by aria-label: "Resize navigation rail" and "Resize inspector panel". Overlay triggers already used by tests: "Start Guide"/"Exit Guide" and the guide's "First show steps" navigation, the `?` help overlay, Ctrl+K QuickSwitcher (aria-label "Quick switcher"), and the "Leave the guide?" ConfirmModal reached by clicking a rail destination while the guide is open.

No new npm or browser dependency. Only the already-installed Playwright Chromium. This plan does not modify production CSS unless a genuinely trivial defect is found; a genuine defect is either fixed minimally with a passing failing-without-the-fix test, or logged to a `.planning/debug/<name>.md` session log following the convention of `.planning/debug/screenshot-viewport-text-overlap.md`. Never weaken an assertion to make a failing layout pass.
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: Extract the shared Playwright harness and fix the stale destination list</name>
  <files>frontend/e2e/helpers.ts, frontend/e2e/responsive.spec.ts, frontend/e2e/desktop-view-docs.spec.ts</files>
  <behavior>
    - Test 1: helpers export `NAV_LABELS` derived from `desktopViews.json` exactly matching the fourteen catalog navLabels in catalog order, with no hardcoded duplicate list.
    - Test 2: helpers' `settle()` waits out the `--motion-settle` transition with the existing fixed 250ms convention, unchanged.
    - Test 3: `installHealthyBindings()` installs the complete healthy Wails mock surface currently inline in desktop-view-docs.spec.ts, and `assertNoRuntimeIssues`/`expectTopBarTextToBeReadable` keep their existing semantics.
    - Test 4: `findOverflowingControls()` retains the existing per-button checks (content taller than its box, right edge past the viewport, left edge off-screen) and adds the same checks for `input`/`select`/`[contenteditable]` plus a document and `.appShell` horizontal-overflow check; `expectSafetyClusterAvailable()` asserts Blackout / Automation / Stop / Release All are visible and interactive (D-13).
  </behavior>
  <action>
    Create `frontend/e2e/helpers.ts` as the single shared harness. Move the destination sweep labels to derive from `../src/shell/desktopViews.json` (regular group views only), move `settle()` verbatim, move the healthy Wails binding installer and the runtime-issue/top-bar guards verbatim from desktop-view-docs.spec.ts, and build `findOverflowingControls`/`expectSafetyClusterAvailable` from the existing geometry checks in responsive.spec.ts. Keep every behavior byte-identical where it already exists: same viewport, same settle timing, same mocks, same output paths, same assertion thresholds.

    Refactor `responsive.spec.ts` and `desktop-view-docs.spec.ts` to import from helpers with no change in assertion meaning. Because the catalog is authoritative, the responsive sweep now runs all fourteen destinations (Project Fixtures and Desk join the sweep), so its regression coverage is no longer stale. Confirm the docs capture spec still emits exactly the same PNG set and dimensions after the refactor.
  </action>
  <verify>
    <automated>npm --prefix frontend run test:e2e &amp;&amp; npm --prefix frontend run docs:screenshots &amp;&amp; npm --prefix frontend run docs:screenshots</automated>
  </verify>
  <done>The two existing suites consume one shared harness with unchanged behavior, the responsive sweep covers all fourteen catalog destinations, and a second docs-capture run leaves the committed PNG set unchanged.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: Build the catalog-driven tiling-WM resize suite</name>
  <files>frontend/e2e/resize.spec.ts, frontend/package.json</files>
  <behavior>
    - Test 1 (aspect-ratio sweep): for every size in the tiling-WM aspect-ratio set and every catalog destination, after open + settle the workspace heading is visible, `findOverflowingControls` is empty, neither the document nor `.appShell` overflows horizontally, the top bar is readable, the safety cluster is visible and interactive, and the rail remains reachable.
    - Test 2 (height-dominant): at wide-but-short heights the titlebar and global header stay pinned and visible, the canvas scrolls internally instead of the window overflowing, and the safety cluster stays visible.
    - Test 3 (compact-breakpoint crossing): stepping across the 1100px boundary in both directions (1101 -> 1099 -> 1101, plus exact 1100) with inspector content open collapses the inspector track to 0/display:none and caps the rail with `min(var(--rail-width), 160px)`; no stuck inline `--inspector-width`, no clipped controls, and the crossing repeated at least three times stays stable.
    - Test 4 (discrete rapid sequences): WM-style layout cycling and a fast width burst assert correct geometry after every step and stable, settled geometry at the end with no stuck transition or residual horizontal overflow.
    - Test 5 (mid-drag outer resize): starting a rail/inspector drag, resizing the outer viewport mid-drag, then releasing clamps the panel to its [min, max] and leaves a valid layout.
    - Test 6 (open-overlay resize): the Guided First Show overlay, the `?` help overlay, the Ctrl+K QuickSwitcher, the "Leave the guide?" ConfirmModal, and the Scripts Run dialog each stay inside the viewport, remain operable, and close cleanly while the window is resized.
    - Test 7 (degraded mode): without daemon bindings the header warning copy renders, the safety cluster stays visible and interactive (D-13), and there is no horizontal overflow at 900x720 and 320x240.
    - Test 8 (persistence): a dragged rail/inspector size stored in `golc.railWidth`/`golc.inspectorWidth` is re-clamped into [min, max] and yields a valid layout after a reload at a different viewport.
    - Test 9 (large/mixed-DPI): a 3840x2160 sweep over all destinations, plus a deviceScaleFactor 1.5 sample at 1280x720 and 1920x1080, shows no overflow or clipped controls.
  </behavior>
  <action>
    Write `frontend/e2e/resize.spec.ts` importing only the shared helpers, never a second copy of destinations, mocks, settle, or guards. Define the tiling-WM size matrix: near-minimum 320x240 (scratchpad/corner tile), portrait strips 480x1080 and 640x1080 (vertical split / third tile), half tile 960x1080, regression anchors 900x720/1280x900/1920x1080, horizontal-split rows 1920x540 and 1920x300, compact-boundary 1099/1100/1101, and 4K 3840x2160. Sequence scenarios step `page.setViewportSize` with short pauses (about 100-150ms per step, matching WM layout animation cadence) and assert after each step plus a final `settle()`. Mid-drag tests press the rail/inspector separator, resize the viewport, then release the pointer. Overlay tests resize while each surface is open and assert it stays within the viewport and remains operable. Persistence tests clear localStorage, drag, resize, reload, and re-clamp. Add `"test:e2e:resize": "playwright test e2e/resize.spec.ts"` to `frontend/package.json` so the aggressive suite can run without the screenshot suite. Use only geometry and computed-style assertions; no screenshots and no pixel baselines.
  </action>
  <verify>
    <automated>npm --prefix frontend run test:e2e:resize &amp;&amp; npm --prefix frontend run test:e2e</automated>
  </verify>
  <done>The resize suite is green across all fourteen destinations, every tiling-WM scenario above, and every size class -- entirely geometry-based, catalog-driven, and free of new dependencies.</done>
</task>

<task type="auto">
  <name>Task 3: Run full verification and triage any surfaced defects</name>
  <files>.planning/debug/* (created only if a genuine defect is found), plus minimal production CSS only for a genuinely trivial defect</files>
  <action>
    Run the frontend unit/build chain and the complete e2e set: `npm --prefix frontend run build` (tsc + vitest + vite) followed by `npm --prefix frontend run test:e2e`. For every resize-suite failure, classify it as a harness bug (fix the test), an already-documented/out-of-scope behavior (record it), or a genuine UI defect. A genuine defect is either fixed minimally with a passing test that fails without the fix (e.g. a content-box overflow like the prior ListRow fix), or logged to a `.planning/debug/<date>-<slug>.md` session doc in the style of `.planning/debug/screenshot-viewport-text-overlap.md` with hypothesis, reproduction, evidence, and resolution. Never weaken or delete an assertion to make a failing layout pass, and never add a pixel baseline.
  </action>
  <verify>
    <automated>npm --prefix frontend run build &amp;&amp; npm --prefix frontend run test:e2e</automated>
    <human-check>Run the desktop app at 640x1080, 1920x300, 320x240, and 3840x2160 in a real window. Confirm the rail, global header, safety cluster, and active workspace remain usable and readable at each size, and that internal panel drags and the compact-breakpoint collapse still behave after the suite's findings are addressed.</human-check>
  </verify>
  <done>The resize suite is green on current HEAD, every genuine defect surfaced is fixed with test evidence or logged as a debug session doc, the unit/build chain is green, and no existing assertion or convention was relaxed.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| Catalog JSON -> test destinations | The catalog is the single source of nav labels; a suite must never introduce a second hand-maintained list that can drift. |
| Shared helpers -> existing suites | The refactor must preserve the docs capture suite's behavior byte-for-byte, including its output PNG set. |
| Browser-rendered app -> assertions | Runtime state (telemetry, Wails fallback) can make geometry nondeterministic; healthy mocks plus settle() keep it deterministic. |
| Resize suite -> production CSS | A change made to satisfy the suite must fix a real layout defect with a failing-without-the-fix test, never mask one or relax an assertion. |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-qrz-01 | Tampering | helper duplication between specs | medium | mitigate | One `e2e/helpers.ts`; refactor tests require identical behavior and byte-identical docs screenshots. |
| T-qrz-02 | Tampering | destination-list drift | medium | mitigate | `NAV_LABELS` derived from `desktopViews.json`; a catalog change without a passing sweep fails loudly. |
| T-qrz-03 | Denial of Service | flaky geometry during CSS transitions | high | mitigate | `settle()` after every resize and per-step assertions in sequences; only settled geometry is measured. |
| T-qrz-04 | Denial of Service | suite runtime/parallelism on the shared dev server | low | mitigate | Focused `test:e2e:resize` script; fullyParallel is already configured; no new installs. |
| T-qrz-05 | Repudiation | genuine-defect triage ambiguity | low | mitigate | Every failure classified and recorded in verification output or a debug session doc. |
| T-qrz-06 | Information Disclosure | committed screenshots/user data | n/a | accept | No screenshots, pixel baselines, or real show data in this suite. |
| T-qrz-SC | Tampering | npm package execution | low | accept | No new dependency; existing locked Playwright toolchain only. |

</threat_model>

<verification>
- `npm --prefix frontend run test:e2e:resize` is green on current HEAD.
- `npm --prefix frontend run test:e2e` is green (responsive sweep, docs capture, and resize suites together).
- `npm --prefix frontend run build` is green (tsc + vitest + vite) with no production behavior change.
- `npm --prefix frontend run docs:screenshots` run twice: the second pass leaves the committed PNG set and dimensions unchanged, proving the helper refactor is behavior-neutral.
- No new npm dependency and no browser install; no pixel baselines added to the frontend.
- Any genuine defect fixed is covered by a test that fails without the fix; anything else is recorded in a `.planning/debug/` session doc.
</verification>

<success_criteria>
- The resize suite aggressively covers all fourteen destinations across the tiling-WM aspect-ratio extremes, compact-breakpoint crossings in both directions and repeated, rapid discrete sequences and bursts, mid-drag outer resizes, open-overlay resizes, degraded-mode fallback, persistence-after-reload, and 4K/mixed-DPI samples -- all geometry-based with no pixel baselines.
- Existing responsive and docs suites share one harness and keep identical behavior; the docs capture set stays byte-identical.
- The stale hardcoded destination list is gone; catalog drift fails loudly instead of silently skipping workspaces.
- No new dependency; the aggressive suite runs on the installed Chromium via a focused npm script.
- Every genuine defect surfaced is minimally fixed with test evidence or logged to a debug session doc; no assertion was weakened.
</success_criteria>

<source_coverage_audit>

| Source | Item | Status | Plan coverage |
|--------|------|--------|---------------|
| GOAL | Aggressive Playwright window-resize responsiveness/adaptability testing | COVERED | Tasks 1-2 add the harness and catalog-driven resize suite. |
| GOAL | Tiling-WM edge cases | COVERED | Task 2 scenario matrix: extreme aspect ratios, discrete/rapid sequences, breakpoint crossings, mid-drag and open-overlay resizes, degraded mode, persistence, 4K/DPR. |
| GOAL | Follow existing test conventions | COVERED | Tasks 1-2 reuse the real-layout-engine, geometry-assertion, no-snapshot, settle(), and focused-script conventions; catalog-driven destinations fix existing drift. |
| CONTEXT | Existing responsive.spec.ts conventions | COVERED | Task 1 preserves and extends its geometry checks into helpers; Task 2 sweeps them across the full matrix. |
| CONTEXT | D-13 safety cluster availability | COVERED | `expectSafetyClusterAvailable` is defined in Task 1 and asserted at every size class in Task 2, including degraded mode. |
| CONTEXT | Prior viewport text-overlap incident | COVERED | The top-bar readability guard is reused in the sweep; capture viewport, output paths, and published screenshots remain unchanged. |
| CONTEXT | e2e stays outside npm test/build; no new dependencies | COVERED | The new script mirrors `test:e2e`; helpers reuse the installed Playwright stack only. |
| REQ | No matching requirement ID exists; this is a regression gate | EXCLUDED | No new requirement is introduced; constraints come from PROJECT.md/SKILL.md/D-13 and the existing suite conventions. |
| RESEARCH | Quick mode forbids a research phase | EXCLUDED | Existing suites, shell source, and the prior debug session provide sufficient discovery. |

</source_coverage_audit>

<output>
Create `.planning/quick/260801-qrz-aggressive-playwright-resize-testi/260801-qrz-SUMMARY.md` when done.
</output>
