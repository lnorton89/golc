---
phase: quick-desktop-views-documentation
plan: 260729-gq6
subsystem: documentation
tags: [react, playwright, nextjs, docgen, submodule]
requires:
  - phase: 09-front-door-ui-completion
    provides: Twelve routed Wails desktop workspaces
provides:
  - Versioned canonical desktop-view catalog consumed by shell navigation
  - Strict deterministic generator for independently buildable site content
  - Reproducible real-browser screenshots for every desktop destination
  - Responsive static desktop views documentation route
affects: [desktop-shell, site-docs, future-in-app-help]
tech-stack:
  added: []
  patterns: [catalog-derived navigation, strict generated submodule content, deterministic browser capture]
key-files:
  created:
    - frontend/src/shell/desktopViews.json
    - internal/docgen/desktopviews.go
    - frontend/e2e/desktop-view-docs.spec.ts
    - site/src/app/docs/desktop-views/page.tsx
    - site/src/components/docs/DesktopViewExplorer.tsx
  modified:
    - frontend/src/shell/navigation.ts
    - internal/docgen/docgen.go
    - site/src/app/docs/page.tsx
    - site/src/app/sitemap.ts
    - site/tests/visual.spec.ts
key-decisions:
  - "The desktop shell owns the schema-v1 catalog; the independent site consumes a strict generated copy."
  - "Screenshot filenames are fixed to /desktop-views/<destination-id>.png and validated before filesystem writes."
  - "The guide renders all views in static HTML and adds optional client-side group filtering."
patterns-established:
  - "Repository metadata drives navigation, documentation inventory, screenshot capture, and site rendering."
  - "Site content is committed in the submodule before the parent repository records its gitlink."
requirements-completed: [FDUI-01, FDUI-02, FDUI-03]
coverage:
  - id: D1
    description: Canonical twelve-view catalog drives shell navigation and generated site content.
    requirement: FDUI-01
    verification:
      - kind: integration
        ref: go test ./internal/docgen ./internal/command -count=1
        status: pass
      - kind: unit
        ref: frontend/src/shell/AppShell.navigation.test.tsx
        status: pass
    human_judgment: false
  - id: D2
    description: Real-browser capture writes one fixed 1440x900 screenshot for every catalog destination.
    requirement: FDUI-02
    verification:
      - kind: automated_ui
        ref: npm --prefix frontend run docs:screenshots (two consecutive passes)
        status: pass
    human_judgment: false
  - id: D3
    description: Responsive desktop views guide publishes all current destinations from generated content.
    requirement: FDUI-03
    verification:
      - kind: automated_ui
        ref: site/tests/visual.spec.ts desktop views light/dark and mobile tests
        status: pass
      - kind: integration
        ref: npm --prefix site run lint/typecheck/build/test:links
        status: pass
    human_judgment: true
    rationale: Final keyboard-focus clarity and subjective screenshot legibility require human review.
duration: 17min
completed: 2026-07-29
status: complete
---

# Quick Task 260729-gq6: Desktop Views Documentation Summary

**One versioned catalog now drives twelve desktop destinations, deterministic screenshots, generated site data, and a responsive static guide.**

## Performance

- **Duration:** 17 minutes
- **Started:** 2026-07-29T19:09:00Z
- **Completed:** 2026-07-29T19:26:00Z
- **Tasks:** 3
- **Files modified:** 28

## Accomplishments

- Replaced the shell's duplicated navigation inventory with a schema-v1 catalog containing factual documentation for all twelve workspaces.
- Added strict Go validation, deterministic normalization, real-browser screenshot capture, and committed 1440x900 assets.
- Published `/docs/desktop-views` with generated content, responsive filtering, full metadata, sitemap/docs links, mobile coverage, and inspected light/dark baselines.

## Task Commits

1. **Task 1 RED:** `6526a172` - failing catalog and generator tests
2. **Task 1 GREEN:** `1b265f7e` - catalog-derived navigation and deterministic generator
3. **Task 2 site:** `ebabdbe` - generated catalog copy and twelve screenshots
4. **Task 2 parent:** `467ef863` - screenshot tooling and site gitlink
5. **Task 3 site:** `3c57a2f` - responsive documentation route and visual coverage
6. **Task 3 parent:** `4f6d8d1b` - final site gitlink

## Decisions Made

- The canonical catalog stays beside the desktop shell, while generation creates the committed copy required by the independently checked-out site.
- Catalog screenshot paths must exactly match each destination ID under `/desktop-views/`; traversal, stale files, duplicate paths, and wrong dimensions fail capture.
- The server-rendered page contains the complete inventory before hydration; client state only filters the already-present content.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Updated disposable command-route fixture**
- **Found during:** Task 1
- **Issue:** The existing disposable docs-route test did not create the newly required catalog source.
- **Fix:** Added a minimal valid desktop catalog to the synthetic repository fixture.
- **Files modified:** `internal/command/docs_test.go`
- **Verification:** `go test ./internal/docgen ./internal/command -count=1`
- **Committed in:** `1b265f7e`

**2. [Rule 3 - Blocking] Kept link crawling local before production deployment**
- **Found during:** Task 3
- **Issue:** Linkinator followed canonical/OpenGraph self-metadata to the currently deployed site, where the new route cannot exist before this commit ships.
- **Fix:** Excluded production self-metadata URLs while retaining recursive validation of all local static-export navigation and assets.
- **Files modified:** `site/package.json`
- **Verification:** `npm --prefix site run test:links`
- **Committed in:** site commit `3c57a2f`

## Issues Encountered

- Chromium did not paint far-offscreen images in the first tall two-column baseline. The desktop guide now uses a responsive three-column layout at wide viewports and the visual test settles each image, producing complete inspected baselines.
- Windows uses a platform-specific Playwright suffix. The inspected outputs were recorded under the plan-required canonical Linux baseline filenames; the full 19-test interaction suite passed locally with snapshot comparison disabled after targeted light/dark comparison passed.

## Known Stubs

None.

## Threat Review

- Strict decoding rejects unknown fields, unsupported versions, invalid references, duplicate identities, and unsafe screenshot paths.
- Screenshot capture uses deterministic browser fallback state and all twelve assets were inspected before commit.
- No new network endpoint, authentication path, schema trust boundary, or runtime file-access surface was introduced.

## Human Check Remaining

Open `/docs/desktop-views` at 1280x900 and 390x844 in both themes. Confirm keyboard focus/selection is clear and the captured application text is comfortably legible at the desired reading distance.

## Self-Check: PASSED

- All declared catalog, generator, capture, site route, component, content, screenshot, test, and baseline files exist.
- Parent commits `6526a172`, `1b265f7e`, `467ef863`, and `4f6d8d1b` exist.
- Site commits `ebabdbe` and `3c57a2f` exist and precede their parent gitlink commits.
- The unrelated pre-existing `internal/command/build.go` modification remains unstaged and untouched.
