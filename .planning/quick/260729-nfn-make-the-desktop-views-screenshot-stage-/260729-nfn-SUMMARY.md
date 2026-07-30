---
phase: quick-desktop-views-stage-separation
plan: 260729-nfn
subsystem: documentation-ui
tags: [react, nextjs, playwright, responsive-design, netlify, submodule]
requires:
  - phase: quick-desktop-views-master-detail
    provides: Single catalog-driven Desktop Views master-detail explorer and accessible lightbox
  - phase: quick-desktop-views-screenshot-refresh
    provides: Current GUI-derived screenshot set and pinned production deployment command
provides:
  - Theme-coherent inset screenshot stage separated from the selected-view detail
  - Computed-style and geometry regression coverage across desktop/mobile and light/dark
  - Pushed submodule release and verified Netlify production deployment
affects: [site-docs, desktop-view-explorer, visual-regression, production-release]
tech-stack:
  added: []
  patterns: [observable computed-style boundary assertions, site-first submodule release ordering]
key-files:
  created:
    - .planning/quick/260729-nfn-make-the-desktop-views-screenshot-stage-/260729-nfn-SUMMARY.md
  modified:
    - site/src/components/docs/DesktopViewExplorer.tsx
    - site/tests/visual.spec.ts
    - .planning/STATE.md
key-decisions:
  - "The existing full-width enlarge button remains the single screenshot stage, using responsive inset padding, the page surface, a line-token frame, and a restrained shadow rather than adding another nested card."
  - "Regression coverage asserts computed geometry, surface distinction, border visibility, ordering, and overflow instead of Tailwind class strings or new pixel baselines."
patterns-established:
  - "Presentation boundaries expose stable region test IDs while preserving the underlying tabpanel and lightbox semantics."
  - "A production site change is pushed in the site submodule first, recorded by the parent gitlink, and deployed exactly once only after both repositories are pushed."
requirements-completed: [DOCS-VISUAL-01, DOCS-RELEASE-01]
coverage:
  - id: D1
    description: The screenshot stage is visibly distinct from the following detail in light and dark at desktop and mobile widths.
    requirement: DOCS-VISUAL-01
    verification:
      - kind: automated_ui
        ref: site/tests/visual.spec.ts#desktop views screenshot stage stays distinct from detail across themes and viewports
        status: pass
      - kind: manual_procedural
        ref: Local and canonical browser inspection at 1280x900 and 390x844 in light/dark
        status: pass
    human_judgment: false
  - id: D2
    description: Selector ordering, horizontal overflow, tab switching, lightbox Escape handling, focus restoration, and scroll restoration remain intact.
    requirement: DOCS-VISUAL-01
    verification:
      - kind: automated_ui
        ref: npx playwright test tests/visual.spec.ts --grep "desktop views"
        status: pass
      - kind: manual_procedural
        ref: Canonical production Overview/Scripts and lightbox interaction review
        status: pass
    human_judgment: false
  - id: D3
    description: The exact pushed site revision was recorded by the parent and deployed once through the pinned Netlify script.
    requirement: DOCS-RELEASE-01
    verification:
      - kind: integration
        ref: Netlify deploy 6a6a966e21eaaa4135d9aa9d and canonical HTTP/browser verification
        status: pass
    human_judgment: false
duration: 17min
completed: 2026-07-29
status: complete
---

# Quick Task 260729-nfn: Desktop Views Screenshot Stage Separation Summary

**A responsive inset screenshot stage now creates a restrained, theme-coherent break before the selected-view detail, backed by computed-layout tests and a verified production deployment.**

## Performance

- **Duration:** 17 minutes
- **Started:** 2026-07-29T23:58:00Z
- **Completed:** 2026-07-30T00:15:00Z
- **Tasks:** 3
- **Files modified:** 5

## Accomplishments

- Added a page-surface screenshot stage with 12px mobile and 20px desktop inset, line-token image frame, restrained shadow, and an explicit lower boundary before the panel-surface detail.
- Added non-snapshot Playwright coverage for region order, computed surfaces, responsive inset geometry, lower border visibility, and page/main overflow in all four viewport/theme combinations.
- Preserved all grouped-tab, lightbox naming, Escape/backdrop/close, focus restoration, scroll restoration, and responsive stacking behavior.
- Pushed the site revision before the parent gitlink, deployed exactly once with the pinned command, and verified the canonical production route visually and structurally.

## Task Commits

1. **Task 1 RED: Define screenshot-stage boundary behavior** - `0557d82` (site test)
2. **Task 1 GREEN: Implement the screenshot-stage boundary** - `95fa31b` (site feature)
3. **Task 2: Validate themes and responsive widths** - no source commit (verification-only task)
4. **Task 3: Record and push the release gitlink and plan** - `8566feb7` (parent release)

## Release and Deployment Evidence

- **Site SHA:** `95fa31b5a8ad797e2133e11337a38b01caf2cae4`
- **Site remote:** `origin/master` confirmed at the exact site SHA before deployment.
- **Parent release commit:** `8566feb74f0ff039446578953a48cca2ba02d925`
- **Parent pre-deploy HEAD:** `389aaa3824610a697ed774f60795c06cc315c1d3`
- **Parent gitlink:** `95fa31b5a8ad797e2133e11337a38b01caf2cae4`
- **Deploy command invocations:** exactly one `npm run deploy`
- **Netlify deploy ID:** `6a6a966e21eaaa4135d9aa9d`
- **Unique deploy URL:** `https://6a6a966e21eaaa4135d9aa9d--golc-site.netlify.app`
- **Canonical route:** `https://golc-site.netlify.app/docs/desktop-views` returned HTTP 200.
- **Production browser evidence:** 1280x900 and 390x844 in light and dark all showed a distinct inset stage, visible lower border, different stage/detail surfaces, correct source order, and no horizontal overflow. Overview and Scripts switched correctly; enlarge, Escape close, and focus restoration passed.

The parent pre-deploy HEAD is one commit after the task release because a concurrent, non-overlapping CI fix landed on the shared `master` branch after `8566feb7`. The task release commit remains in its history and both revisions carry the exact pushed site gitlink.

## Verification

- `npm --prefix site run lint` - passed
- `npm --prefix site run typecheck` - passed
- `npm --prefix site run build` - passed
- `npx playwright test tests/visual.spec.ts --grep "desktop views"` - 7 passed
- Direct local browser inspection - passed in desktop/mobile light/dark
- Direct canonical production browser inspection - passed in desktop/mobile light/dark
- `site/deno.lock` SHA-256 remained `BFB04A4A2A7EEF152AD62BE18596F0056907CB0183B29EAD49952CA3F0BEB858`
- No screenshot, pixel-baseline, GUI-source, package, Netlify configuration, or lockfile changes were made.

## Files Created/Modified

- `site/src/components/docs/DesktopViewExplorer.tsx` - Adds the inset screenshot stage, image frame, and stable stage/detail hooks.
- `site/tests/visual.spec.ts` - Adds four-mode computed-style, geometry, order, and overflow regression coverage.
- `.planning/quick/260729-nfn-make-the-desktop-views-screenshot-stage-/260729-nfn-PLAN.md` - Records the validated quick-task plan.
- `.planning/quick/260729-nfn-make-the-desktop-views-screenshot-stage-/260729-nfn-SUMMARY.md` - Records execution and deployment evidence.
- `.planning/STATE.md` - Records quick-task completion.

## Decisions Made

- Kept the screenshot, enlarge label, and following detail inside the existing single tabpanel; no nested detail card or alternate master-detail structure was introduced.
- Used observable computed styles and geometry for regression evidence so theme token behavior is tested without coupling assertions to utility class spelling.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

- The first GREEN Playwright invocation served the prior static export, so it could not see the new test hooks. Rebuilding the planned static export resolved the stale-bundle condition; the fresh focused and full semantic runs passed.
- A concurrent, non-overlapping parent CI commit landed after the task release commit. It was preserved, the task gitlink remained exact, and both commits were confirmed on `origin/master` before deployment.
- Two production mobile capture calls timed out while requesting a long full-page image. The page remained responsive; disposable viewport captures of the exact stage/detail boundary and computed layout checks completed successfully without retrying the production deployment.

## Known Stubs

None.

## TDD Gate Compliance

- RED commit `0557d82` added the failing stage/detail boundary test before implementation.
- GREEN commit `95fa31b` implemented the boundary and made the focused suite pass.
- No refactor commit was needed.

## User Setup Required

None.

## Next Phase Readiness

- The Desktop Views master-detail experience is ready for continued documentation work with explicit screenshot/detail hierarchy and focused semantic regression coverage.
- No blockers remain.

## Self-Check: PASSED

- All task-owned component, test, plan, summary, and STATE files exist.
- Site commits `0557d82` and `95fa31b` and parent release commit `8566feb7` exist.
- The pushed site SHA, parent gitlink, and canonical HTTP 200 response were reconfirmed after evidence authoring.

---
*Quick task: 260729-nfn*
*Completed: 2026-07-29*
