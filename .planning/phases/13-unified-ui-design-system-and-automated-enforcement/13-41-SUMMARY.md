---
phase: 13-unified-ui-design-system-and-automated-enforcement
plan: 41
subsystem: testing
tags: [playwright, e2e, accessibility, text-zoom, safety, offline, design-system]
requires:
  - phase: 13-15
    provides: Design-system-migrated SafetyCluster with independently-testable useHoldToConfirm and per-control data-safety-control markers
  - phase: 13-17
    provides: Screenshot-tolerance calibration and deterministic capture conventions (waitForFonts, settle)
  - phase: 13-24
    provides: Design-system-migrated AppShell/CommandRail/GlobalFrame shell chrome
  - phase: 13-25
    provides: Migrated Scenes/Programming slice (indirectly exercised via shared shell chrome)
provides:
  - Executable, evidence-producing proof that the approved UI-SPEC 200%-text-zoom
    acceptance clause holds at exactly 900x720 (real Chromium `zoom` CSS
    mechanism, not viewport scaling or a mislabeled transform)
  - Executable, evidence-producing proof that Blackout and Revoke Automation
    remain independently reachable and operable under both a daemon-offline
    and a provider-offline projected state, with zero cross-dispatch and no
    inferred "stopped" claim
  - A genuine Rule 2 fix to GlobalFrame.module.css's `.statusSlot` (60px
    min-width floor) closing a real LiveStatusBar 0-width collapse this
    plan's own new evidence harness discovered
affects: [13-20 (evidence validator, will consume text-zoom-200.json and
  offline-safety.json), any future shell-chrome plan touching GlobalFrame's
  header row]
tech-stack:
  added: []
  patterns:
    - "Real Chromium 'text zoom' simulation for Playwright: set the non-standard `zoom` CSS property directly on `document.documentElement` (Chromium's own internal mechanism for real Ctrl-Plus page zoom) rather than `transform: scale()` (visual-only, no reflow) or shrinking `page.setViewportSize()` (changes the reported viewport, not an overlaid zoom). `document.documentElement` is transiently `null` the instant a `page.addInitScript` callback first fires on a fresh navigation in this exact Playwright/Chromium build -- defer the assignment via a `readystatechange` listener rather than assuming the element already exists."
    - "A `zoom`-ed root's own client width still equals the real viewport width (`page.viewportSize()`), while every *descendant*'s own `clientWidth` reports the effective, halved CSS-pixel budget -- check scrollWidth<=clientWidth per element against its OWN client width, never against the outer viewport's literal pixel value."
    - "'Visible OR keyboard reachable' locators: tag required elements with a throwaway `data-*-probe` attribute (identity-based, immune to legitimately state-varying accessible names like SafetyCluster's Blackout/'Release Blackout' toggle) and drive a real `page.keyboard.press('Tab')` loop recording `document.activeElement`'s probe tag per step, rather than string-matching a locator's name."
    - "Keyboard-operate proof for a hold-to-confirm control: `page.keyboard.down('Enter')`, wait past the component's own hold threshold, `page.keyboard.up('Enter')` -- Playwright's synthetic CDP keydown does not auto-repeat, so this reliably drives exactly one full hold-to-completion cycle without the component's own `event.repeat` guard ever engaging."
    - "Spy on a Wails-mocked bridge method via `page.addInitScript` wrapping (not a full re-mock): read the already-installed `window.go.wails.SafetyService.<Method>`, wrap it to increment a counter and record args, and reassign -- addInitScript scripts fire in registration order, so this safely layers onto `installHealthyBindings`' own mocks without re-declaring them (matches designSystem.ts's installCalibrationBindings precedent)."
key-files:
  created:
    - frontend/e2e/design-system.text-zoom.spec.ts
    - frontend/e2e/design-system.offline-safety.spec.ts
    - frontend/e2e/fixtures/projectedAcceptanceStates.ts
    - .planning/phases/13-unified-ui-design-system-and-automated-enforcement/evidence/text-zoom-200.json
    - .planning/phases/13-unified-ui-design-system-and-automated-enforcement/evidence/offline-safety.json
  modified:
    - frontend/src/shell/GlobalFrame.module.css
key-decisions:
  - "Real-tested (not assumed) the exact Chromium zoom mechanism against the live dev server before writing the final spec: confirmed `document.documentElement.style.zoom = '2'` genuinely reflows descendants (body.clientWidth halves to 450 at a 900px viewport, matching a real 200%-zoomed browser window) and confirmed `page.addInitScript`'s callback sees `document.documentElement === null` at its very first invocation on this exact Playwright/Chromium build, which silently aborted every earlier attempt (all downstream `__zoomLog` writes were lost, not merely the zoom assignment) until deferred via `readystatechange`."
  - "Fixed a genuine, previously-undiscovered Rule 2 gap in GlobalFrame.module.css: `.statusSlot`'s unbounded `min-width: 0` let LiveStatusBar (D-13's persistent live-truth surface) collapse to a literal, Playwright-non-visible 0x0 box at exactly 900x720+200%-zoom. Diagnosed via the real running dev server (not guessed): traced to flexbox's own space-scarcity shrink, distinguished from the already-known/deferred plain-900px text-overlap issue (deferred-items.md's 260803-13-32 entry), and fixed with a 60px floor confirmed pixel-identical to the pre-fix geometry at both already-baselined 900px and 1280px unzoomed acceptance viewports (inert there; only engages under this plan's own zoom-driven scarcity)."
  - "Did NOT attempt a holistic 'header never overflows at 200% zoom' fix (e.g. allowing GlobalFrame to wrap onto two rows): that requires changing AppShell.module.css's fixed 52px grid-row track height, which would very plausibly perturb already-accepted 13-32/13-33 screenshot baselines at plain, unzoomed acceptance widths -- an architectural, cross-plan-baseline-risking change outside this plan's declared file scope. Instead relied on the must_have's own explicit 'visible OR keyboard reachable' alternative: SafetyCluster's three controls remain Playwright-'visible' (non-zero box) and Tab-focusable/keyboard-operable regardless of pointer-clipped position beyond the 900px viewport edge, proven directly rather than assumed."
  - "Interpreted 'active task / dominant next action' (must_haves' own paired phrasing, matching UI-SPEC's GuidedFlow pattern row) as Guided First Show's single primary footer action button -- the only concrete, UI-SPEC-named 'dominant next action' locator in the app. Guided First Show replaces only the canvas (AppShell.tsx's own documented contract: 'SafetyCluster/GlobalFrame/CommandRail... remain [mounted]'), so this locator is exercised simultaneously with every other required locator in one shared state, not a separate navigation."
  - "Modeled 'provider-offline' and 'daemon-offline' (Task 2) using the app's one real connectivity signal (StatusSnapshot.reachable) rather than inventing a fictitious ProviderService binding: no separate AI/automation-provider integration exists yet in this milestone (Phase 10, PROJECT.md Active requirements). daemon-offline sets reachable=false (LiveStatusBar's real 'Can't reach the playback engine' copy). provider-offline sets reachable=true and asserts the operator-facing safety surface is completely unaffected -- proving D-14's 'must not depend on an AI provider' as zero observable coupling, the honest, testable claim available until a real provider integration ships."
patterns-established:
  - "installTextZoom / installProjectedAcceptanceState / installSafetyDispatchSpies (this plan's own fixtures): reusable for any future Phase 13 backstop spec needing a real browser zoom simulation or a spied safety-command dispatch count."
requirements-completed: [D-02, D-03, D-10, D-12, D-13, D-14, UI-SPEC-VISUAL-VERIFICATION, UI-SPEC-MIGRATION-ACCEPTANCE]
coverage:
  - id: D1
    description: "200%-text-zoom acceptance clause is executable: at exactly 900x720 with a real Chromium zoom mechanism, navigation, live truth, the Guided First Show dominant next action, and all three SafetyCluster controls are proven visible or keyboard reachable with zero documentElement/body horizontal overflow, plus a real keyboard-hold dispatch proof for Blackout/Revoke Automation."
    requirement: "D-13"
    verification:
      - kind: e2e
        ref: "frontend/e2e/design-system.text-zoom.spec.ts (Playwright/Chromium, 1 test)"
        status: pass
      - kind: other
        ref: ".planning/phases/13-unified-ui-design-system-and-automated-enforcement/evidence/text-zoom-200.json"
        status: pass
    human_judgment: false
  - id: D2
    description: "Provider-offline and daemon-offline safety-authority acceptance clause is executable: for both named projected states, Blackout and Revoke Automation remain visible, Tab-reachable, and keyboard-operable through their own independent local SafetyService path (dispatched exactly once, zero cross-dispatch), and LiveStatusBar's connectivity copy never infers stopped playback/output beyond the fixture's own explicit Go-owned truth."
    requirement: "D-14"
    verification:
      - kind: e2e
        ref: "frontend/e2e/design-system.offline-safety.spec.ts (Playwright/Chromium, 2 tests: daemon-offline, provider-offline)"
        status: pass
      - kind: other
        ref: ".planning/phases/13-unified-ui-design-system-and-automated-enforcement/evidence/offline-safety.json"
        status: pass
    human_judgment: false
  - id: D3
    description: "Full frontend suite, tsc, and the scoped design-system checker remain green after the GlobalFrame.module.css fix"
    verification:
      - kind: integration
        ref: "cd frontend && npx tsc --noEmit (clean); npx vitest run (528/528 pass); node scripts/design-system/check.mjs --paths src/shell/GlobalFrame.module.css (zero diagnostics)"
        status: pass
    human_judgment: false
metrics:
  duration: unavailable (spans a single continuous execution session, including real Chromium/dev-server experimentation to determine the correct zoom simulation technique)
  completed_date: 2026-08-03
status: complete
---

# Phase 13 Plan 41: 200% Text-Zoom and Provider/Daemon-Offline Safety Evidence Summary

**Two new Playwright specs make the approved UI-SPEC's 200%-text-zoom and provider/daemon-offline safety clauses executable and semantically evidenced against the real running app, discovering and fixing a genuine LiveStatusBar 0-width collapse bug along the way.**

## Performance

- **Tasks:** 2/2 complete
- **Focused e2e tests:** text-zoom 1/1 pass; offline-safety 2/2 pass
- **Full suite:** `tsc --noEmit` clean; `npx vitest run` 528/528 pass; scoped `node scripts/design-system/check.mjs --paths src/shell/GlobalFrame.module.css` zero diagnostics

## Accomplishments

- Determined and validated, against the real running dev server (not assumed), the correct way to simulate genuine browser "text zoom" in Playwright: Chromium's own non-standard `zoom` CSS property applied to `document.documentElement`, deferred past a transient `document.documentElement === null` window in `addInitScript`'s very first callback invocation on this exact Playwright/Chromium build.
- Wrote `design-system.text-zoom.spec.ts`: seeds an active-show Go-owned status snapshot, opens Guided First Show (its "dominant next action" locator), applies real 200% zoom at exactly 900x720, and proves navigation/live-truth/active-task/all three safety controls are visible or keyboard reachable with zero documentElement/body horizontal overflow -- plus a genuine keyboard-hold-to-threshold dispatch proof (exactly one Blackout call, exactly one RevokeAutomation call) via bridge-method spies.
- Discovered, root-caused, and fixed a real Rule 2 gap this plan's own new harness surfaced: `GlobalFrame.module.css`'s `.statusSlot` had `min-width: 0`, letting LiveStatusBar collapse to a literal 0x0 box (Playwright-non-visible) at exactly 900x720+200%-zoom. Fixed with a 60px floor, empirically confirmed pixel-identical to the pre-fix geometry at the already-baselined 900px/1280px unzoomed acceptance viewports.
- Wrote `frontend/e2e/fixtures/projectedAcceptanceStates.ts`: two typed, named states (`daemon-offline`, `provider-offline`) each pairing an explicit Go-owned `StatusSnapshot` with an expected connectivity-copy pattern (or an explicit "no banner" expectation) and a set of forbidden "stopped" inference patterns, plus reusable SafetyService dispatch-count spies.
- Wrote `design-system.offline-safety.spec.ts`: for both states, proves Blackout and Revoke Automation stay visible, Tab-reachable, and keyboard-operable via their own independent local dispatch path (exactly once, zero cross-dispatch to the other safety command), and that LiveStatusBar's live truth (scene/bar/layers/source/output) is unchanged before and after dispatch, with the correct connectivity copy present/absent per state.

## Task Commits

1. **Task 1: Prove 200 percent text zoom at compact desktop size** - `4e48cd02` (feat)
2. **Task 2: Prove provider-offline and daemon-offline safety authority** - `9b685512` (feat)

## Files Created/Modified

- `frontend/e2e/design-system.text-zoom.spec.ts` (new) - 200%-zoom reachability/overflow proof
- `frontend/e2e/design-system.offline-safety.spec.ts` (new) - provider/daemon-offline safety-authority proof
- `frontend/e2e/fixtures/projectedAcceptanceStates.ts` (new) - typed daemon-offline/provider-offline states + dispatch spies
- `.planning/phases/13-unified-ui-design-system-and-automated-enforcement/evidence/text-zoom-200.json` (new) - closed semantic evidence object
- `.planning/phases/13-unified-ui-design-system-and-automated-enforcement/evidence/offline-safety.json` (new) - closed semantic evidence object
- `frontend/src/shell/GlobalFrame.module.css` - `.statusSlot` gained a 60px `min-width` floor (Rule 2 fix)

## Decisions Made

See `key-decisions` in frontmatter for full rationale on each. Summary: validated the real zoom mechanism empirically before finalizing the spec (a naive `addInitScript` assignment silently no-ops on this Chromium build); fixed a genuine LiveStatusBar 0-width collapse this plan's own harness discovered rather than deferring it, since it directly blocked this plan's own explicit must_have, while deliberately NOT attempting a holistic header-wrap redesign that would risk already-accepted 13-32/13-33 screenshot baselines; interpreted "active task" as Guided First Show's primary footer action (the only concrete UI-SPEC-named "dominant next action" locator); and modeled provider/daemon-offline using the app's one real connectivity signal since no separate AI-provider integration exists yet in this milestone.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Fixed LiveStatusBar's 0-width collapse at 200% zoom**
- **Found during:** Task 1, while building the real evidence harness against the live dev server
- **Issue:** `GlobalFrame.module.css`'s `.statusSlot { flex: 1; min-width: 0; }` let LiveStatusBar (D-13's persistent live-truth surface) shrink to a literal 0x0 box under real 200% zoom at 900x720 -- Playwright's real `isVisible()` correctly reported `false`, directly failing this plan's own explicit must_have ("live truth... remain visible or keyboard reachable").
- **Fix:** Changed `.statusSlot`'s `min-width` from `0` to `60px`. Empirically confirmed (via the real dev server, before and after) pixel-identical `statusSlot` geometry at plain 900px (~121px natural width) and 1280px (~501px natural width) unzoomed -- the floor is inert at every already-baselined acceptance viewport and only engages under this plan's own zoom-driven scarcity, forcing the header itself (not this locked-height slot) to be the one that overflows. SafetyCluster's own three buttons remain independently keyboard-reachable via Tab regardless of pointer-clipped position, covered by the must_have's own "visible OR keyboard reachable" alternative.
- **Files modified:** `frontend/src/shell/GlobalFrame.module.css`
- **Verification:** `design-system.text-zoom.spec.ts`'s `liveTruth` locator now reports `visible: true`; scoped `node scripts/design-system/check.mjs --paths src/shell/GlobalFrame.module.css` remains zero diagnostics; full `npx vitest run` (528/528) and `npx tsc --noEmit` remain clean.
- **Committed in:** `4e48cd02`

---

**Total deviations:** 1 auto-fixed (1 missing-critical)
**Impact on plan:** Necessary for correctness -- this plan's own explicit must_have named this exact locator/viewport/zoom combination, and the underlying app genuinely failed it before this fix. No scope creep: the fix is a single, narrowly-targeted, empirically-verified-inert-elsewhere CSS property change, not a redesign.

## Issues Encountered

- This worktree had no `node_modules` (a prior agent's `node_modules` junction into the main checkout was explicitly flagged as dangerous per this plan's own setup instructions) -- ran `npm install` inside this worktree's own `frontend/` directory to get a real, independent copy, matching the documented recovery path. Playwright's Chromium browser binary was already present in the shared `~/AppData/Local/ms-playwright` cache, so no additional download was required.
- Determining the correct "real browser text zoom" simulation technique required real experimentation against the actual running `npm run dev` server rather than reasoning from documentation alone: an initial `transform: scale()`-free but still-naive `addInitScript` assignment of `document.documentElement.style.zoom` silently no-op'd because `document.documentElement` is transiently `null` the very first time an `addInitScript` callback fires on a fresh navigation in this exact Playwright/Chromium build (confirmed via a `try/catch` probe showing a `TypeError: Cannot read properties of null`) -- deferred the assignment via a `readystatechange` listener instead.
- Discovered mid-task that Guided First Show's primary footer button click can trigger a whole-page vertical scroll under 200% zoom (the app's `100vh`-based shell genuinely needs more real screen height than the physical 720px viewport at this zoom factor -- an expected, real zoom consequence, not a bug, and the plan's own must_have is explicitly scoped to horizontal "body overflow" only). Confirmed this doesn't affect any assertion's correctness (Playwright's `isVisible()` and the keyboard Tab traversal are both scroll-position-independent) and documented it in the spec's own comments rather than masking it.

## Next Phase Readiness

Both `evidence/text-zoom-200.json` and `evidence/offline-safety.json` are closed semantic evidence objects ready for Plan 13-20's (not yet executed) `validate-phase13-evidence.mjs` validator to consume once that plan lands -- their shape (state inputs, projected copy/status results, control boxes/focus order/dispatch counts, environment/build SHA, assertions) was written directly from this plan's own `<action>` text. The `GlobalFrame.module.css` fix closes a real, previously-undiscovered accessibility gap without touching any other already-migrated shell file or already-accepted screenshot baseline.

## Self-Check: PASSED

- Commits `4e48cd02` and `9b685512` exist and contain all declared files (verified via `git log --oneline` and `git diff --diff-filter=D` showing zero accidental deletions in either commit).
- `frontend/e2e/design-system.text-zoom.spec.ts`, `frontend/e2e/design-system.offline-safety.spec.ts`, `frontend/e2e/fixtures/projectedAcceptanceStates.ts`, `evidence/text-zoom-200.json`, and `evidence/offline-safety.json` all exist on disk.
- Both plan-declared exact `<verify>` commands pass with exit 0 (re-run after commit).
- Full frontend suite (528/528), `tsc --noEmit`, and the scoped design-system checker all pass cleanly.

---
*Phase: 13-unified-ui-design-system-and-automated-enforcement*
*Completed: 2026-08-03*
