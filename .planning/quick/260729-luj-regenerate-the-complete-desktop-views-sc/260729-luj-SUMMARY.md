---
phase: quick-desktop-views-screenshot-refresh
plan: 260729-luj
subsystem: documentation
tags: [playwright, screenshots, nextjs, netlify, submodule]

requires:
  - phase: quick-260729-gq6
    provides: "Catalog-driven Desktop Views documentation and deterministic capture harness"
  - phase: quick-260729-ia1
    provides: "Pinned Netlify production deployment workflow"
provides:
  - "Twelve current, reviewed 1920x1080 desktop workspace captures"
  - "Site-owned shell-free npm command for delegating to the parent capture harness"
  - "Pushed site and parent revisions plus a verified production deployment"
affects: [desktop-documentation, site-release, screenshot-maintenance]

tech-stack:
  added: []
  patterns:
    - "Submodule-owned commands resolve parent tooling from import.meta.url and delegate without a shell"
    - "Screenshot releases preserve reviewed hashes through local, Git, CDN, and browser verification"

key-files:
  created:
    - site/scripts/regenerate-desktop-views.mjs
  modified:
    - site/package.json
    - site/public/desktop-views
    - site

key-decisions:
  - "Keep catalog entries, Playwright capture logic, Wails mocks, viewport, and layout guards exclusively in the parent harness; the site wrapper only validates and delegates."
  - "Run only Windows/local site gates and focused non-snapshot browser checks; do not update Linux pixel baselines."
  - "Treat the reported screenshot-stage/detail visual blending as a separate styling follow-up, not part of this screenshot-refresh release."

patterns-established:
  - "Use process.execPath plus npm_execpath, an argument array, shell:false, an absolute cwd, and CI=1 for cross-repository npm delegation on Windows."

requirements-completed: [DOCS-SCREENSHOTS-01, DOCS-RELEASE-01]

coverage:
  - id: D1
    description: "A site maintainer can regenerate Desktop Views with one safe npm command, including from checkout paths containing spaces."
    requirement: DOCS-SCREENSHOTS-01
    verification:
      - kind: integration
        ref: "site: npm run docs:screenshots"
        status: pass
      - kind: integration
        ref: "GUID path-with-spaces delegation and missing-parent diagnostic probe"
        status: pass
    human_judgment: false
  - id: D2
    description: "All twelve delivered captures are current, deterministic, reviewed, and exactly 1920x1080."
    requirement: DOCS-SCREENSHOTS-01
    verification:
      - kind: automated_ui
        ref: "Two clean-server hash-identical capture passes plus wrapper reproduction"
        status: pass
      - kind: manual_procedural
        ref: "Original-resolution inspection of all 12 PNGs"
        status: pass
      - kind: integration
        ref: "Frontend 274 tests and full build"
        status: pass
    human_judgment: false
  - id: D3
    description: "The pushed revisions were deployed once and the production route, assets, and all twelve live panels were verified."
    requirement: DOCS-RELEASE-01
    verification:
      - kind: integration
        ref: "npm ci, lint, typecheck, build, 45-link crawl, and 12 focused Playwright tests"
        status: pass
      - kind: e2e
        ref: "Netlify deploy 6a6a8e83cc3eb0bfe11441d4 and canonical production verification"
        status: pass
    human_judgment: false

duration: 12min
completed: 2026-07-29
status: complete
---

# Quick Task 260729-luj: Desktop Views Screenshot Refresh Summary

**Twelve current 1920x1080 workspace captures now ship with a site-local, shell-free regeneration command and byte-identical production verification.**

## Performance

- **Duration:** 12 minutes
- **Started:** 2026-07-29T23:29:35Z
- **Completed:** 2026-07-29T23:41:21Z
- **Tasks:** 3
- **Files modified:** 15

## Accomplishments

- Preserved the completed evidence from two clean-server, hash-identical capture passes, 12/12 original-resolution visual inspections, and the frontend's 274-test full build.
- Added `site/npm run docs:screenshots`, which validates the expected parent checkout and delegates to its existing Playwright harness with `CI=1`, `shell: false`, and path-safe Node arguments.
- Passed all revised Windows/local site gates without running Linux containers or changing visual baselines.
- Pushed the site first, recorded its exact gitlink in the parent, deployed production exactly once, and verified every live asset and selected panel.

## Task Commits

1. **Tasks 1-2 (site):** `ce08de0` - refreshed twelve screenshots and added the maintained regeneration entrypoint.
2. **Task 3 (parent):** `1a000325` - recorded the already-pushed site revision.
3. **Concurrent remote integration:** `155a99b7` - merged the non-overlapping upstream `shell.nix` cleanup before the parent push.

Planning artifacts remain uncommitted for the quick-task orchestrator.

## Deployment Evidence

- **Site SHA:** `ce08de0b07e2cd851d718d108950382a988b2a20`
- **Site remote:** `origin/master` resolves to the same SHA.
- **Parent task commit:** `1a000325917257ff48a58370fd8125ba01b2905e`
- **Parent pushed SHA:** `155a99b727f3bd26ffb20f1a1b6fb70c63f1c001`
- **Parent remote:** `origin/master` resolves to the same pushed SHA.
- **Parent gitlink:** resolves to site SHA `ce08de0b07e2cd851d718d108950382a988b2a20`.
- **Netlify deploy ID:** `6a6a8e83cc3eb0bfe11441d4`
- **Build logs:** https://app.netlify.com/projects/golc-site/deploys/6a6a8e83cc3eb0bfe11441d4
- **Production URL:** https://golc-site.netlify.app
- **Unique deploy URL:** https://6a6a8e83cc3eb0bfe11441d4--golc-site.netlify.app

## Verification

- `npm run docs:screenshots` from `site/` passed and reproduced the reviewed 12-file hash map exactly at 1920x1080.
- A disposable GUID-named checkout with spaces proved shell-free parent delegation and `CI=1` propagation.
- Removing only the disposable parent produced exit 1 with `GOLC_DESKTOP_VIEWS_PARENT_MISSING` and the exact expected absolute frontend path.
- `npm ci`, lint, typecheck, the 21-page static build, and the 45-link recursive crawl passed locally.
- Twelve focused metadata, navigation, keyboard, lightbox, responsive-layout, and non-snapshot visual tests passed locally.
- No file under `tests/visual.spec.ts-snapshots` changed.
- The production route returned current Desktop Views markup.
- Every production screenshot returned HTTP 200, `image/png`, a valid PNG signature, 1920x1080 dimensions, and the same SHA-256 as its committed local file.

## Capture and Production Results

| View | Original-resolution review | Wrapper hash | Production asset | Live selected panel |
|------|----------------------------|--------------|------------------|---------------------|
| Overview | Pass | Identical | Pass | Pass |
| Shows | Pass | Identical | Pass | Pass |
| Save & Recovery | Pass | Identical | Pass | Pass |
| Settings | Pass | Identical | Pass | Pass |
| Fixture Library | Pass | Identical | Pass | Pass |
| Patch & Pools | Pass | Identical | Pass | Pass |
| Scenes & Looks | Pass | Identical | Pass | Pass |
| Scripts | Pass | Identical | Pass | Pass |
| Operator Surface | Pass | Identical | Pass | Pass |
| MIDI Mapping | Pass | Identical | Pass | Pass |
| Art-Net | Pass | Identical | Pass | Pass |
| Diagnostics | Pass | Identical | Pass | Pass |

For every live tab, the browser observed the matching current image as fully loaded at natural size 1920x1080, with zero broken images, detected overlap, clipping, panel escape, or horizontal overflow.

## Files Created/Modified

- `site/scripts/regenerate-desktop-views.mjs` - validates the parent layout and delegates safely to the canonical capture harness.
- `site/package.json` - exposes `docs:screenshots`.
- `site/public/desktop-views/*.png` - twelve refreshed desktop workspace screenshots.
- `site` - parent gitlink updated to the pushed site revision.

## Decisions Made

- The wrapper contains no duplicated catalog, viewport, binding, or Playwright behavior; all capture authority remains in `frontend/e2e/desktop-view-docs.spec.ts`.
- The existing reviewed screenshot hashes are the release identity and were required to remain unchanged after site-local delegation.
- The newly reported screenshot-stage/detail visual blending is intentionally deferred to a separate focused styling workflow so this task does not mutate GUI or site presentation code.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Ran focused Playwright checks from the actual site working directory**
- **Found during:** Task 2
- **Issue:** On this npm/PowerShell combination, `npm --prefix site exec` launched Playwright from the parent, so `site/playwright.config.ts` was not loaded and all relative URLs were invalid.
- **Fix:** Re-ran the exact focused test selection after `Push-Location site`, loading the planned config and local static server.
- **Files modified:** None
- **Verification:** 12 focused tests passed.

**2. [Rule 3 - Blocking] Integrated a concurrent non-overlapping parent remote commit**
- **Found during:** Task 3
- **Issue:** The parent push was rejected because `origin/master` advanced with a `shell.nix` cleanup after execution began.
- **Fix:** Fetched and inspected the divergence, confirmed no `site` overlap, merged the remote tip without rewriting existing GUI commits, revalidated the gitlink, and pushed.
- **Files modified:** `shell.nix` by the upstream commit only; no task-authored change.
- **Verification:** Parent `origin/master` equals `155a99b727f3bd26ffb20f1a1b6fb70c63f1c001`; the gitlink still equals the pushed site SHA.

---

**Total deviations:** 2 auto-fixed blocking issues.
**Impact on plan:** Verification and push ordering remain intact; no screenshot, GUI, baseline, or task scope was weakened.

## Issues Encountered

- `npm ci` retained the pre-existing dependency audit result of 35 findings (2 low, 1 moderate, 32 high); no unplanned dependency mutation was attempted.
- The user reported that the screenshot stage visually blends into the detail section below. This is a presentation concern outside the screenshot-refresh file boundary and is deferred to a new focused workflow; no styling was changed here.

## Authentication Gates

None - existing Git and Netlify authentication completed both pushes and the single production deployment.

## Known Stubs

None.

## Threat Review

- Parent discovery is rooted in `import.meta.url`; missing manifest, catalog, or capture spec fails before screenshots change.
- Delegation uses `process.execPath`, `npm_execpath`, an argument array, `shell: false`, an absolute frontend `cwd`, inherited stdio, and forced `CI=1`.
- No unplanned endpoint, authentication path, schema boundary, or runtime network surface was introduced.
- The protected `.syso` state and SHA-256 remained unchanged; its disabled alternate remained absent.
- `site/deno.lock` remained untracked, unstaged, and byte-identical.
- Only task-owned site paths and the parent gitlink were staged.

## User Setup Required

None.

## Next Phase Readiness

- Desktop Views screenshots and their regeneration/release contract are complete and live.
- The screenshot-stage/detail visual separation concern is ready for a separate scoped styling task.

## Self-Check: PASSED

- The wrapper, site manifest, summary, and exactly twelve screenshot PNGs exist.
- Site commit `ce08de0`, parent task commit `1a000325`, and pushed integration commit `155a99b7` exist.
- Site and parent `origin/master` both equal their local pushed revisions.
- The production route, twelve byte-identical assets, and twelve live selected panels passed final verification.
