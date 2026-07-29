---
phase: quick-docs-navigation-dropdown
plan: 260729-j3o
subsystem: ui
tags: [nextjs, react, playwright, accessibility, responsive-navigation]

requires:
  - phase: quick-desktop-views-docs
    provides: Existing /docs and /docs/desktop-views routes
provides:
  - Accessible desktop Docs disclosure linking to Docs overview and Desktop Views
  - Responsive mobile Docs group with exact-route current state
  - Focused Playwright coverage for disclosure, navigation, current state, and close behavior
affects: [site-navigation, docs, desktop-views]

tech-stack:
  added: []
  patterns:
    - Native details disclosure with synchronized expanded state and document-level close handlers
    - Exact-route aria-current on child links with family-level active state on the trigger

key-files:
  created:
    - site/src/components/DocsMenu.tsx
  modified:
    - site/src/components/NavLinks.tsx
    - site/src/components/MobileMenu.tsx
    - site/tests/visual.spec.ts
    - site

key-decisions:
  - "Use the existing native details disclosure pattern for desktop navigation and a visible, non-nested Docs group in the mobile dialog."
  - "Preserve the active screenshot-debug changes by committing explicit navigation paths inside the site before advancing only the parent gitlink."

patterns-established:
  - "Docs family state: /docs and descendants activate the trigger, while only an exact child href receives aria-current=page."
  - "Submodule isolation: commit explicit site paths first, then record only the resulting site SHA in the parent."

requirements-completed: [SITE-NAV-01]

coverage:
  - id: D1
    description: "Desktop primary navigation exposes Docs overview and Desktop Views through an accessible disclosure."
    requirement: SITE-NAV-01
    verification:
      - kind: automated_ui
        ref: "site/tests/visual.spec.ts#docs navigation disclosure opens and reaches Desktop Views"
        status: pass
      - kind: automated_ui
        ref: "site/tests/visual.spec.ts#docs navigation communicates family and exact-route state"
        status: pass
      - kind: automated_ui
        ref: "site/tests/visual.spec.ts#docs navigation closes on Escape, outside click, and link activation"
        status: pass
    human_judgment: false
  - id: D2
    description: "Mobile navigation groups Docs overview and Desktop Views and closes after selection."
    requirement: SITE-NAV-01
    verification:
      - kind: automated_ui
        ref: "site/tests/visual.spec.ts#docs navigation is grouped and navigable in the mobile dialog"
        status: pass
    human_judgment: false
  - id: D3
    description: "The production site emits both docs routes without lint or type errors."
    requirement: SITE-NAV-01
    verification:
      - kind: other
        ref: "npm --prefix site run lint && npm --prefix site run typecheck && npm --prefix site run build"
        status: pass
    human_judgment: false

duration: 17min
completed: 2026-07-29
status: complete
---

# Quick Task 260729-j3o: Docs Navigation Dropdown Summary

**Accessible desktop Docs disclosure and grouped mobile navigation now expose both the Docs overview and Desktop Views without disturbing active screenshot-debug work.**

## Performance

- **Duration:** 17 min after crash recovery
- **Started:** 2026-07-29T21:18:00Z
- **Completed:** 2026-07-29T21:35:00Z
- **Tasks:** 2
- **Files modified:** 5

## Accomplishments

- Replaced the flat desktop Docs link with a keyboard- and pointer-operable disclosure containing Docs overview and Desktop Views.
- Added exact-route current state and a visible Docs group to the existing mobile navigation dialog.
- Added and passed four focused Playwright behaviors covering expanded state, active state, navigation, Escape, outside-click, child activation, and mobile closure.
- Preserved the twelve modified desktop-view screenshot assets and the parent screenshot-debug files without staging, restoring, regenerating, or committing them.

## Task Commits

1. **Task 1 RED: Add failing docs navigation coverage** - `20897d1` (test)
2. **Task 1 selector correction: Keep coverage aligned with current page semantics** - `e5d6abf` (test)
3. **Task 1 GREEN: Add responsive Docs navigation** - `beb87b8` (feat)
4. **Task 2: Record the completed site submodule SHA** - `ac2b570e` (feat, parent repository)

## Files Created/Modified

- `site/src/components/DocsMenu.tsx` - Native desktop Docs disclosure with family/exact-route state and deterministic close behavior.
- `site/src/components/NavLinks.tsx` - Replaces the flat Docs link with the disclosure while retaining the surrounding navigation order.
- `site/src/components/MobileMenu.tsx` - Adds the visible Docs group and route-aware child links to the existing mobile dialog.
- `site/tests/visual.spec.ts` - Covers desktop and mobile Docs navigation behavior with accessible selectors.
- `site` - Parent gitlink advanced to site commit `beb87b888668805590bebd6a100cd844362c8399`.

## Decisions Made

- Followed the repository-established `ResourcesMenu` native `<details>` lifecycle instead of adding a dependency or custom popover primitive.
- Kept mobile Docs destinations as a visible group within the existing dialog rather than nesting another disclosure.
- Used exact child href equality for `aria-current="page"` and `/docs` prefix matching only for the Docs family trigger.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Corrected two stale focused-test selectors**
- **Found during:** Task 1 verification
- **Issue:** A non-exact `Desktop Views` role query collided with an existing “Browse the desktop views” content link, and the outside-click step targeted homepage copy that no longer exists.
- **Fix:** Made the navigation-link query exact and exercised outside-click closure against the stable `main` landmark.
- **Files modified:** `site/tests/visual.spec.ts`
- **Verification:** `npm --prefix site run test:visual -- --grep "docs navigation"` passes 4/4 tests.
- **Committed in:** `e5d6abf`

---

**Total deviations:** 1 auto-fixed bug.
**Impact on plan:** The correction keeps the planned behavior coverage intact without changing production scope.

## Issues Encountered

- Direct browser attachment was unavailable during the final visual review. The responsive interaction paths were instead verified through the required Playwright desktop and 390x844 tests, and the production build completed successfully.

## Known Stubs

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Desktop Views is now discoverable from both primary navigation surfaces.
- No navigation blockers remain; active screenshot-debug work is still present and isolated for its owning session.

## Self-Check: PASSED

- Created and modified files exist.
- Site commits `20897d1`, `e5d6abf`, and `beb87b8` exist.
- Parent gitlink commit `ac2b570e` exists and records site SHA `beb87b888668805590bebd6a100cd844362c8399`.
- Final lint, typecheck, production build, and focused Playwright suite pass.

---
*Quick task: 260729-j3o*
*Completed: 2026-07-29*
