---
phase: quick-desktop-views-master-detail
plan: 260729-hsx
subsystem: documentation-ui
tags: [react, nextjs, playwright, accessibility, submodule]
requires:
  - phase: quick-desktop-views-documentation
    provides: Generated twelve-view catalog, screenshots, and original documentation route
provides:
  - Catalog-driven grouped master-detail desktop views guide
  - Keyboard-synchronized semantic tab selection across all twelve views
  - Accessible screenshot lightbox with contained focus and restored page state
  - Responsive mobile selector-above-detail layout
affects: [site-docs, desktop-view-catalog, accessibility]
tech-stack:
  added: []
  patterns: [catalog-keyed master-detail selection, dependency-free accessible modal lifecycle]
key-files:
  created: []
  modified:
    - site/src/app/docs/desktop-views/page.tsx
    - site/src/components/docs/DesktopViewExplorer.tsx
    - site/tests/visual.spec.ts
    - site/tests/visual.spec.ts-snapshots/desktop-views-light-chromium-linux.png
    - site/tests/visual.spec.ts-snapshots/desktop-views-dark-chromium-linux.png
key-decisions:
  - "The original generated groups remain the selector rendering source while a flattened ordered projection drives selection and keyboard movement."
  - "The lightbox owns focus and body scroll only while mounted, then restores the exact opener and prior inline overflow value."
patterns-established:
  - "A named vertical tablist can retain catalog grouping while one catalog-ID-keyed tabpanel presents complete selected content."
  - "Site implementation and baselines are committed inside the submodule before the parent records the exact gitlink."
requirements-completed: [FDUI-01, FDUI-02, FDUI-03]
coverage:
  - id: D1
    description: All twelve generated desktop views remain grouped and selectable through one semantic master-detail explorer.
    requirement: FDUI-01
    verification:
      - kind: automated_ui
        ref: site/tests/visual.spec.ts#desktop views expose one grouped selector and complete selected detail
        status: pass
    human_judgment: false
  - id: D2
    description: Keyboard selection and the screenshot lightbox preserve semantic state, focus, backdrop behavior, and page scrolling.
    requirement: FDUI-02
    verification:
      - kind: automated_ui
        ref: site/tests/visual.spec.ts#desktop views keyboard navigation and lightbox tests
        status: pass
    human_judgment: false
  - id: D3
    description: The selected workspace is visually dominant beside a compact selector and stacks without overflow on mobile.
    requirement: FDUI-03
    verification:
      - kind: automated_ui
        ref: site/tests/visual.spec.ts#desktop views light/dark baselines and mobile overflow test
        status: pass
    human_judgment: true
    rationale: Final perceived screenshot comfort and focus contrast benefit from human review on the target display.
duration: 9min
completed: 2026-07-29
status: complete
---

# Quick Task 260729-hsx: Desktop Views Master-Detail Summary

**A compact grouped selector now drives one screenshot-led workspace detail, with complete keyboard navigation and a focus-safe full-size lightbox.**

## Performance

- **Duration:** 9 minutes
- **Started:** 2026-07-29T19:53:06Z
- **Completed:** 2026-07-29T20:01:48Z
- **Tasks:** 2
- **Files modified:** 5

## Accomplishments

- Replaced twelve repeated cards and group filtering with a single catalog-driven master-detail explorer while retaining every group, view, and generated content field.
- Added roving semantic tabs with ArrowUp, ArrowDown, Home, and End selection synchronized to focus.
- Added a named screenshot modal with a close control, Escape and true-backdrop closing, inside-click persistence, focus containment/restoration, and exact body overflow restoration.
- Approved refreshed light/dark desktop baselines and inspected the selector-above-detail mobile rendering at 390x844.

## Task Commits

1. **Task 1 RED:** `6ae7946` (site) - Playwright behavior contract for selection, keyboard, modal lifecycle, and mobile layout.
2. **Tasks 1-2 GREEN:** `c1cebeb` (site) - Master-detail explorer, accessible lightbox, route sizing, and approved baselines.
3. **Parent gitlink:** `494e9df3` - Records site commit `c1cebeb`.

## Files Created/Modified

- `site/src/app/docs/desktop-views/page.tsx` - Expands route framing for the focal screenshot column.
- `site/src/components/docs/DesktopViewExplorer.tsx` - Grouped selector, selected detail, keyboard navigation, and lightbox lifecycle.
- `site/tests/visual.spec.ts` - Interaction, semantic state, focus, scroll, backdrop, and mobile regression coverage.
- `site/tests/visual.spec.ts-snapshots/desktop-views-light-chromium-linux.png` - Approved light master-detail baseline.
- `site/tests/visual.spec.ts-snapshots/desktop-views-dark-chromium-linux.png` - Approved dark master-detail baseline.

## Decisions Made

- Preserve original grouped props for rendering and flatten only to establish stable cross-group ordering.
- Use the stable catalog ID as selected state and tab identity so labels and ordering can evolve without introducing a second inventory.
- Keep the modal dependency-free and save the prior inline body overflow value rather than assuming the page began with an empty scroll style.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Restored opener focus after focus containment cleanup**
- **Found during:** Task 1 targeted Playwright verification.
- **Issue:** Focusing the opener synchronously while the modal focus-containment listener was still active moved focus back into the closing dialog.
- **Fix:** Marked intentional close and restored the captured opener in the effect cleanup after removing the focus listener.
- **Files modified:** `site/src/components/docs/DesktopViewExplorer.tsx`
- **Verification:** Escape, backdrop, and close-control focus assertions all pass.
- **Committed in:** site commit `c1cebeb`

---

**Total deviations:** 1 auto-fixed bug.
**Impact on plan:** The fix is required for the planned focus-restoration contract and adds no scope.

## Issues Encountered

- Playwright on Windows requests `*-chromium-win32.png` while the repository intentionally stores canonical Linux-named baselines. Temporary local filename aliases allowed exact comparison of the two updated desktop-view baselines; all seven focused desktop-view visual and interaction tests passed.
- Thirteen unrelated pre-existing route baselines differ from current Windows rendering. The complete 23-test suite passed with snapshot assertions ignored after the focused desktop-view snapshots passed exactly. No unrelated baseline was modified.

## Known Stubs

None.

## Threat Review

- Catalog values remain repository-owned props rendered without a second inventory; selection is keyed by stable IDs.
- Modal keyboard, pointer, focus, and scroll handlers exist only while open and clean up on close or unmount.
- No network endpoint, authentication path, schema mutation, package, or new runtime file-access surface was introduced.
- The site commits precede the parent gitlink commit.

## Verification

- `npm --prefix site run lint` - passed without warnings.
- `npm --prefix site run typecheck` - passed.
- `npm --prefix site run build` - passed; all 21 static pages generated.
- `npm --prefix site run test:links` - passed; 45 links scanned.
- Focused desktop-view visual and interaction suite - 7 passed, including exact light/dark baseline comparison.
- Complete site Playwright suite with snapshot assertions ignored - 23 passed.

## Human Check Remaining

Open `/docs/desktop-views` in light and dark themes at desktop and 390x844. Confirm the selected screenshot feels comfortably dominant, visible focus has sufficient contrast on the target display, and the full-size lightbox feels comfortable with mouse and keyboard.

## Self-Check: PASSED

- All five declared modified files exist.
- Site commits `6ae7946` and `c1cebeb` exist.
- Parent gitlink commit `494e9df3` exists and points to site commit `c1cebeb`.
- The canonical catalog, generated JSON, and desktop screenshot assets were not modified.
- The site worktree is clean; only this uncommitted quick-task planning directory remains in the parent for orchestrator handling.
