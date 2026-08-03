---
phase: 13-unified-ui-design-system-and-automated-enforcement
plan: "30"
subsystem: design-system-visual-verification
tags: [playwright, error-boundary, theme, fonts, contrast, wcag, accessibility, e2e]
requires:
  - phase: 13-17
    provides: waitForFonts helper, screenshot-tolerance calibration, and the deterministic-fixture/evidence-JSON conventions this plan builds on
  - phase: 13-16
    provides: token-independent ErrorBoundary fallback with its shell.json exception registrations
  - phase: 13-24
    provides: core shell/command rail (unrelated code path, declared as a dependency for wave ordering only)
provides:
  - "frontend/e2e/fixtures/startupProbe.ts: a page.addInitScript-based per-animation-frame instrumentation probe proving no unreadable theme/font flash at startup"
  - "frontend/e2e/fixtures/emergencyFallback.ts: a served-stylesheet interception helper that genuinely blocks --ds-* tokens without breaking ES module resolution"
  - "frontend/src/design-system/fixtures/EmergencyFallbackFixture.tsx + App.tsx's ?e2e=emergency-fallback route: a deterministic, browser-reachable render-time failure"
  - "evidence/startup-theme-font.json and evidence/error-boundary-fallback.json: semantic, re-derivable backstop evidence"
affects: [13-31, 13-32, 13-33, 13-34, 13-41]
tech-stack:
  added: []
  patterns:
    - "A pre-navigation instrumentation probe (page.addInitScript, installed before page.goto) samples DOM/style state every animation frame from before any page script runs through the settle window -- proving a startup invariant holds across an entire sequence of frames, not just a single after-the-fact screenshot."
    - "Blocking a 'generated stylesheet' in Vite dev mode means intercepting the real served module request (/src/index.css) and rewriting its response body, not routing a same-named source file (tokens.generated.css) that Vite's dev CSS pipeline inlines server-side and the browser never requests separately."
    - "A rootMounted signal (root element has children) distinguishes the ordinary pre-JS-load blank-page window (nothing to be unreadable yet) from a genuine post-mount theme/content flash -- only the latter is a violation."
    - "ErrorBoundary's own hardcoded hex/rgb literals are duplicated, independent values (never the shared --ds-status-revoked token) -- correcting one for contrast does not touch the brand token used elsewhere."
key-files:
  created:
    - frontend/e2e/fixtures/startupProbe.ts
    - frontend/e2e/design-system.startup.spec.ts
    - frontend/e2e/fixtures/emergencyFallback.ts
    - frontend/e2e/design-system.emergency-fallback.spec.ts
    - frontend/src/design-system/fixtures/EmergencyFallbackFixture.tsx
    - .planning/phases/13-unified-ui-design-system-and-automated-enforcement/evidence/startup-theme-font.json
    - .planning/phases/13-unified-ui-design-system-and-automated-enforcement/evidence/error-boundary-fallback.json
  modified:
    - frontend/src/App.tsx
    - frontend/src/shell/ErrorBoundary.module.css
    - frontend/design-system/exception-proposals/shell.json
key-decisions:
  - "Startup readability/theme/safety-label checks only apply to samples where React has mounted content under #root (rootMounted). The genuinely blank pre-JS-load window (transparent background, default UA font, no data-theme attribute) is an ordinary, unavoidable dev-server module-fetch/parse delay with no text content to be unreadable -- not the 'theme flash' this backstop targets. Verified empirically: without this filter the probe flagged ~20 early frames on every run."
  - "Blocking 'the generated token stylesheet' means intercepting the real served request (/src/index.css) and regex-stripping every --ds-* declaration from its response body, not attempting to route tokens.generated.css directly -- confirmed empirically that Vite's dev CSS pipeline inlines @import content server-side into one response, so the source-named file is never a separate network request. Stripping declarations (rather than aborting the whole response) keeps ES module resolution intact so React still mounts and can hit the real ErrorBoundary path."
  - "Added a `?e2e=emergency-fallback` route (App.tsx) and EmergencyFallbackFixture.tsx (Rule 3 - blocking): no deterministic, browser-reachable way to force an ErrorBoundary render failure existed before this plan (only a Vitest-local `Bomb` helper). Mirrors the existing ?e2e=dialog-feasibility / ?e2e=design-system-gallery seams exactly."
requirements-completed: [D-01, D-06, D-10, D-12, D-13, UI-CONSIDERATIONS-BACKSTOP-STARTUP, UI-CONSIDERATIONS-BACKSTOP-ERROR]
coverage:
  - id: D1
    description: "Startup theme/font backstop proves no unreadable pre-settle frame in persisted light or dark themes, using a continuous per-frame probe rather than a single screenshot"
    requirement: "UI-CONSIDERATIONS-BACKSTOP-STARTUP"
    verification:
      - kind: e2e
        ref: "frontend/e2e/design-system.startup.spec.ts#startup theme/font pre-settle backstop: no unreadable frame in light or dark persisted themes"
        status: pass
    human_judgment: false
  - id: D2
    description: "Emergency-fallback backstop proves ErrorBoundary's token-independent literals remain readable, WCAG-AA-contrast-compliant, keyboard-operable, and viewport-contained with the generated theme stylesheet genuinely blocked"
    requirement: "UI-CONSIDERATIONS-BACKSTOP-ERROR"
    verification:
      - kind: e2e
        ref: "frontend/e2e/design-system.emergency-fallback.spec.ts#emergency fallback is readable, operable, and token-independent before theme CSS"
        status: pass
    human_judgment: false
duration: ~1 session
completed: 2026-08-03
status: complete
---

# Phase 13 Plan 30: Startup and Emergency-Fallback UI Backstops Summary

**Built two independent Playwright backstops: a per-animation-frame startup probe proving no unreadable theme/font flash in persisted light/dark themes, and a served-stylesheet-blocking emergency-fallback proof that found and fixed a real WCAG AA contrast gap (4.27:1, needed 4.5:1) in ErrorBoundary's own token-independent literals.**

## Performance

- **Tasks:** 2/2 complete
- **Files created:** 7
- **Files modified:** 3

## Accomplishments

- `frontend/e2e/fixtures/startupProbe.ts`: a `page.addInitScript`-based instrumentation probe that samples DOM/style state (data-theme attribute, computed background/color, font family, `document.fonts.status`, safety-cluster presence/visibility/labels, and whether React has mounted content) every animation frame, starting before any page script runs and continuing through the theme/font settle window. Includes a from-scratch WCAG contrast-ratio implementation (`parseCssColor`, `relativeLuminance`, `contrastRatio`) and a `getBuildSha()` helper.
- `frontend/e2e/design-system.startup.spec.ts`: tests persisted light and dark themes independently. Reads the probe's accumulated samples *before* ever calling `waitForFonts` (proving the pre-settle window was genuinely captured, not merely asserted), then checks every mounted frame for transparent backgrounds, wrong-theme attributes, incomplete safety-cluster labels, and sub-WCAG-AA contrast; asserts the settled state uses Archivo/JetBrains Mono and the correct `data-theme`. Writes `evidence/startup-theme-font.json` with full per-frame theme/font sequences, contrast findings, build SHA, and environment metadata.
- `frontend/e2e/fixtures/emergencyFallback.ts`: intercepts the real served `/src/index.css` module request (Vite's dev CSS pipeline inlines `tokens.generated.css`'s `@import` server-side, so it is never fetched separately) and regex-strips every `--ds-*` custom-property declaration from the response body, making every `var(--ds-*)` reference in the entire app genuinely invalid at computed-value time without breaking ES module resolution (which aborting the request outright would have done, preventing React from ever mounting).
- `frontend/src/design-system/fixtures/EmergencyFallbackFixture.tsx` + a new `?e2e=emergency-fallback` route in `App.tsx`: a deterministic, browser-reachable component that always throws on render, wrapped in its own `ErrorBoundary` -- mirrors the existing `?e2e=dialog-feasibility` / `?e2e=design-system-gallery` seams.
- `frontend/e2e/design-system.emergency-fallback.spec.ts`: blocks the token stylesheet, forces the render failure, and asserts the fallback's exact registered literal colors render (proving no `var(--ds-*)` silently resolved to an invalid/inherited value), meets WCAG AA 4.5:1 contrast for every text pair, stays inside the viewport at 900x720 and 1280x720 with no document horizontal overflow, and that the Reload action is keyboard-reachable (via repeated Tab, since the scrollable stack-trace `<pre>` is itself tab-stop before the button) with a real native focus ring, and genuinely triggers and recovers from a page reload. Writes `evidence/error-boundary-fallback.json`.

## Task Commits

1. **Task 1: Backstop theme and font initialization before settle** - `428dd7cc` (feat)
2. **Task 2: Backstop token-independent ErrorBoundary before theme CSS** - `f2eedc83` (feat)

_No separate test/feat commit split: both tasks are `tdd="true"` e2e-evidence tasks in the same style as sibling Plan 13-17 -- the fixture, spec, and evidence file were implemented and verified together per task, matching that plan's precedent rather than a literal unit-test RED/GREEN cycle._

## Files Created/Modified

- `frontend/e2e/fixtures/startupProbe.ts` - Pre-navigation per-frame instrumentation probe + WCAG contrast arithmetic + build-SHA helper
- `frontend/e2e/design-system.startup.spec.ts` - Light/dark persisted-theme pre-settle backstop spec + evidence writer
- `frontend/e2e/fixtures/emergencyFallback.ts` - Served-token-stylesheet interception + expected-literal constants
- `frontend/e2e/design-system.emergency-fallback.spec.ts` - Token-blocked ErrorBoundary readability/contrast/focus/activation/viewport backstop spec + evidence writer
- `frontend/src/design-system/fixtures/EmergencyFallbackFixture.tsx` - Deterministic render-failure fixture (new)
- `frontend/src/App.tsx` - Added `?e2e=emergency-fallback` route
- `frontend/src/shell/ErrorBoundary.module.css` - Corrected title/reload literal colors from the canonical revoked red (4.27:1) to a minimal hue-preserving lightening (4.82:1) that clears WCAG AA
- `frontend/design-system/exception-proposals/shell.json` - Updated the two matching exception `match` fields (`shell-error-boundary-title-color`, `shell-error-boundary-reload-color`) in lockstep with the CSS change
- `.planning/phases/13-unified-ui-design-system-and-automated-enforcement/evidence/startup-theme-font.json` - Task 1 evidence
- `.planning/phases/13-unified-ui-design-system-and-automated-enforcement/evidence/error-boundary-fallback.json` - Task 2 evidence

## Decisions Made

- **`rootMounted` filter for startup samples.** The probe's very first ~20 frames (before `main.tsx` and its imports finish fetching/parsing over the dev-server network) are a genuinely blank `#root` with no CSS or fonts loaded yet -- transparent background, default UA "Times New Roman", no `data-theme` attribute. This is ordinary, unavoidable browser-loading behavior with zero text content to be unreadable, not the "theme flash" the backstop targets. Readability/theme/safety-label assertions only apply to samples where `document.getElementById("root").children.length > 0`. Verified empirically: without this filter, every run failed on the pre-mount frames regardless of theme.
- **Token-stylesheet blocking targets the real served request, not the source filename.** Verified empirically (via a throwaway script hitting the dev server directly) that Vite's dev CSS pipeline resolves `index.css`'s `@import "./design-system/tokens.generated.css"` server-side into one JS-wrapped response for `/src/index.css` -- the browser never requests `tokens.generated.css` separately. `page.route` intercepts that real request and regex-strips `--ds-*` declarations from its body (858 stripped in the current build), leaving the rest of `index.css` (box-sizing reset, body/scrollbar rules) structurally intact but every token-referencing value invalid.
- **Corrected ErrorBoundary's title/reload literal colors, not the shared brand token.** ErrorBoundary.module.css's colors are documented as intentionally independent hardcoded literals (never `var(--ds-*)`), duplicated from -- but not reading -- the shared `--ds-status-revoked` token. This makes a narrowly-scoped, safe fix possible: lightening only these two declarations (`#e23a2e` → `#e54e43`, `rgb(226, 58, 46)` → `rgb(229, 78, 67)`) restores WCAG AA 4.5:1 (measured 4.82:1) without touching the canonical revoked-red token used throughout the rest of the app.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Added `EmergencyFallbackFixture.tsx` + `?e2e=emergency-fallback` route to `App.tsx`**
- **Found during:** Task 2
- **Issue:** The plan's `<action>` requires "induce the deterministic ErrorBoundary fixture failure," but no browser-reachable way to force a render-time exception existed -- only a Vitest-local `Bomb` component inside `ErrorBoundary.test.tsx`, unreachable from a real Chromium page.
- **Fix:** Added a minimal component that always throws, wrapped in its own `ErrorBoundary`, mounted from a new `?e2e=emergency-fallback` route in `App.tsx` -- mirrors the existing `?e2e=dialog-feasibility` / `?e2e=design-system-gallery` seams exactly rather than inventing a second routing mechanism (same pattern Plan 13-17 used for its own gallery route).
- **Verification:** `design-system.emergency-fallback.spec.ts` passes; `npx vitest run` (528/528) confirms no regression; `npx tsc --noEmit` clean.
- **Committed in:** `f2eedc83` (Task 2 commit)

**2. [Rule 1 - Bug] Fixed a real WCAG AA contrast violation in ErrorBoundary's title/reload text**
- **Found during:** Task 2, while measuring exact fallback contrast per the plan's own `<action>`
- **Issue:** ErrorBoundary's canonical revoked-red literals (`#e23a2e` title, `rgb(226, 58, 46)` reload) against its own `#131419` background literal measure 4.27:1 -- below the WCAG AA 4.5:1 normal-text floor the design system's own Accessibility Contract requires ("Light and dark modes meet WCAG AA contrast"). This is a pre-existing gap from Phase 13-16 (unrelated to this plan's own changes) in the exact contrast pairing this task's `<behavior>` explicitly requires proving, not an out-of-scope unrelated file.
- **Fix:** Corrected both declarations to a minimal, hue-preserving lightening (`#e54e43` / `rgb(229, 78, 67)`, measured at 4.82:1) that stays clearly within the same "revoked/error red" family. This is ErrorBoundary's own independent, hardcoded literal -- never the shared `--ds-status-revoked` token used broadly elsewhere in the app -- so the fix is scoped to only this fallback screen. Updated the two matching `design-system/exception-proposals/shell.json` entries (`match` field and rationale) in lockstep so the exception registry stays accurate.
- **Verification:** `design-system.emergency-fallback.spec.ts` asserts and confirms 4.82:1 for both pairs; `npx vitest run src/shell/ErrorBoundary.test.tsx` (3/3, unaffected -- no color assertions there); `node scripts/design-system/check.mjs --paths src/shell/ErrorBoundary.module.css,src/shell/ErrorBoundary.tsx,src/shell/QuickSwitcher.tsx --proposal design-system/exception-proposals/shell.json` clean (the narrower `--paths` scope without `QuickSwitcher.tsx` reports an expected "stale exception" artifact for that file's *unrelated* proposal entries -- confirmed as a scoping artifact, not a regression, by including it).
- **Committed in:** `f2eedc83` (Task 2 commit)

---

**Total deviations:** 2 auto-fixed (1 blocking, 1 bug)
**Impact on plan:** Both changes were necessary to make the plan's own declared behavior provable in a real browser. No scope creep beyond ErrorBoundary's own fallback screen; the shared `--ds-status-revoked` brand token is untouched.

## Issues Encountered

- **Vite dev CSS inlining.** Initially assumed `tokens.generated.css` would be a separately interceptable network request (matching the plan's literal "Route/intercept the generated token stylesheet" wording). Verified empirically against the running dev server that Vite's dev CSS pipeline resolves `@import` server-side into a single response for the actually-imported file (`/src/index.css`). Adjusted the interception target accordingly rather than routing a URL the browser never requests.
- **`<pre>` stack-trace element is itself a keyboard tab stop.** `ErrorBoundary.module.css`'s `.detail` (`overflow: auto`, content taller than its `max-height`) is a genuinely scrollable region, which Chromium makes keyboard-focusable so it can be scrolled with arrow keys -- real, expected accessibility behavior. Tab order is body → `<pre>` → Reload button, not straight to Reload. The spec loops Tab presses (bounded at 5 attempts) rather than assuming a fixed count.
- **Startup probe's pre-mount window.** See "Decisions Made" above (`rootMounted` filter) -- this was discovered empirically on the first real test run, not anticipated in the plan.

## Next Phase Readiness

Both backstops are independently executable and evidence-producing, matching Plan 13-17's established conventions (deterministic fixtures, per-frame/per-viewport JSON evidence with build SHA and environment metadata). `startupProbe.ts`'s contrast/color-parsing utilities (`parseCssColor`, `contrastRatio`, `relativeLuminance`, `WCAG_AA_NORMAL_TEXT_MINIMUM`) and `getBuildSha()` are reusable by any later Wave 9-13 visual spec needing the same arithmetic. No blockers for Plans 13-31 through 13-34/13-41.

## Self-Check: PASSED

- Commits `428dd7cc` and `f2eedc83` exist in `git log` and together contain all declared/created files.
- All files listed above exist on disk at their declared paths.
- Both of the plan's own declared verify commands pass against the final committed state: `cd frontend && npx playwright test e2e/design-system.startup.spec.ts --project=chromium --workers=1` (1/1 pass) and `cd frontend && npx playwright test e2e/design-system.emergency-fallback.spec.ts --project=chromium --workers=1` (1/1 pass).
- `npx tsc --noEmit` clean.
- `npx vitest run` — 528/528 pass (no regression from the ErrorBoundary color/App.tsx route changes).
- Evidence JSON files reverted to their as-committed content after final confirmation re-runs (they regenerate a fresh timestamp/build SHA on every execution, matching Plan 13-17's precedent).

---
*Phase: 13-unified-ui-design-system-and-automated-enforcement*
*Completed: 2026-08-03*
