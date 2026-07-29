---
phase: quick-netlify-explicit-production-deploy
plan: 260729-ia1
subsystem: deployment
tags: [netlify, npm, playwright, github-actions, visual-regression]

requires:
  - phase: quick-260729-hsx
    provides: "Redesigned desktop views master-detail explorer and lightbox"
provides:
  - "Pinned repository-local Netlify production deployment command"
  - "Four trace-mapped Linux visual baselines"
  - "Pushed, CI-green, verified production deployment"
affects: [site-release, visual-regression, deployment]

tech-stack:
  added: [netlify-cli@27.0.1]
  patterns:
    - "Production deploys run through an exact repo-local npm script"
    - "Linux baseline repairs use inspected Playwright actual attachments only"

key-files:
  created: []
  modified:
    - site/package.json
    - site/package-lock.json
    - site/README.md
    - site/tests/visual.spec.ts-snapshots/docs-light-chromium-linux.png
    - site/tests/visual.spec.ts-snapshots/docs-dark-chromium-linux.png
    - site/tests/visual.spec.ts-snapshots/desktop-views-light-chromium-linux.png
    - site/tests/visual.spec.ts-snapshots/desktop-views-dark-chromium-linux.png
    - site

key-decisions:
  - "Keep the requested `netlify deploy --build --prod` script: CLI 27.0.1 accepts it and only warns that build is now the default."
  - "Treat the pushed site SHA, exact-SHA green Actions run, successful deploy result, HTTP responses, and live browser interactions as one release contract."

patterns-established:
  - "Commit and push the independently versioned site before recording its gitlink in the parent."

requirements-completed: [SITE-DEPLOY-01, SITE-VIS-01]

coverage:
  - id: D1
    description: "Exact repository-local production deployment command"
    requirement: SITE-DEPLOY-01
    verification:
      - kind: integration
        ref: "npm run deploy"
        status: pass
    human_judgment: false
  - id: D2
    description: "Four authoritative Linux visual baselines accepted without weakening assertions"
    requirement: SITE-VIS-01
    verification:
      - kind: automated_ui
        ref: "Playwright 1.61.1 Linux container: 23 passed"
        status: pass
      - kind: automated_ui
        ref: "GitHub Actions run 30488453395: Visual regression tests"
        status: pass
    human_judgment: false
  - id: D3
    description: "Redesigned desktop views guide deployed and exercised in production"
    requirement: SITE-DEPLOY-01
    verification:
      - kind: e2e
        ref: "https://golc-site.netlify.app/docs/desktop-views"
        status: pass
    human_judgment: false

duration: 28min
completed: 2026-07-29
status: complete
---

# Quick Task 260729-ia1: Explicit Netlify Production Deploy Summary

**Pinned Netlify CLI 27.0.1 now builds and publishes the linked site through `npm run deploy`, with four repaired Linux baselines, green exact-SHA CI, and live production interaction verification.**

## Performance

- **Duration:** 28 min
- **Completed:** 2026-07-29T20:34:31Z
- **Tasks:** 3
- **Files modified:** 8

## Accomplishments

- Added exact `netlify-cli` 27.0.1 manifest/lock pins and the exact `netlify deploy --build --prod` npm script, with README release-contract documentation.
- Replaced only the four planned Linux screenshots with CI actual attachments after verifying their trace names, expected/diff evidence, dimensions, hashes, and visual coherence.
- Passed all local gates, a canonical Linux Playwright run, and GitHub Actions for the exact pushed site SHA.
- Deployed the existing linked site to production and verified canonical/unique URLs plus live master-detail and lightbox behavior.

## Task Commits

1. **Site tasks 1-3:** `9808dcf` (site) — pinned deploy tooling, documentation, and four Linux baseline repairs.
2. **Parent gitlink:** `ecfde167` — records pushed and deployed site revision `9808dcf`.

The PLAN, SUMMARY, and STATE artifacts remain uncommitted for the quick-task orchestrator, per execution instructions.

## Deployment Evidence

- **Site SHA:** `9808dcfd1e17bf6892301d760bfec925c53efd71`
- **Remote branch:** `origin/master` resolves to the same SHA.
- **GitHub Actions:** [run 30488453395](https://github.com/lnorton89/golc-site/actions/runs/30488453395), conclusion `success`
- **CI job:** `build-and-test` job `90699979901`, all steps green, including 23 Playwright tests
- **Netlify site ID:** `31538e21-e9a1-40a0-bf49-8f75307399c7` (preserved; no relink or site creation)
- **Deploy ID:** `6a6a62e865762f73bc8480eb`
- **Deploy logs:** https://app.netlify.com/projects/golc-site/deploys/6a6a62e865762f73bc8480eb
- **Production URL:** https://golc-site.netlify.app
- **Unique deploy URL:** https://6a6a62e865762f73bc8480eb--golc-site.netlify.app

## Verification

- Node `v22.19.0` satisfies CLI 27.0.1's `>=22.13.0` engine.
- npm registry metadata matched package `netlify-cli`, version `27.0.1`, repository `netlify/cli`, MIT license, and planned integrity.
- `npm ci`, lint, typecheck, 21-page static build, 45-link crawl, and Lighthouse across seven routes passed locally.
- Playwright 1.61.1 in the official Linux container passed all 23 tests using the canonical `chromium-linux` baselines.
- The canonical and unique deploy routes both returned HTTP 200 and contained the redesigned heading and grouped selector content.
- Live light and dark themes rendered coherently with the grouped master-detail hierarchy intact.
- Live browser checks found Show, Build, Operate, and Output groups, 12 tabs, one selected tabpanel, and complete Scripts details.
- The Overview lightbox opened with its close control focused; Escape removed the dialog, restored body overflow, and restored focus to the opener.

## Baseline Provenance

| Baseline | CI actual SHA-1 name | Copied SHA-256 |
|---|---|---|
| `docs-light-chromium-linux.png` | `97a14840c1e669c10fe8d81ed141361f8e359ecc` | `1B4F138DA4C91AA15219BA4D8605F70376F71B75C3EFEEF4F9288900EC2B8787` |
| `docs-dark-chromium-linux.png` | `ab6dc03cbc61c524e8b71cd62d02c046aa3c9b4e` | `E3FBC711B1CB2F355DFC85BA023445178288C0EF524CB376F69E830AD57768EC` |
| `desktop-views-light-chromium-linux.png` | `fbc386b906aecbd317d819c310c2b4336de6ee4e` | `17136D0C682D4B04B836C5A98CEEC9D82E9129610C874CFB0F20DF501F7CCC25` |
| `desktop-views-dark-chromium-linux.png` | `fa43ad8d92d0f16a315fe8e064fad0e99b6186a2` | `193250D1AC072A547954CF9ADA5AB6EA5870D46DA0F01687EFAED0D6A11BF367` |

## Decisions Made

- Preserved the exact requested deploy script because CLI 27.0.1 accepts `--build`; its warning confirms that building remains the default.
- Kept authentication entirely in the existing runtime CLI context and recorded no credential material.
- Accepted no expected image, diff image, Windows alias, global screenshot update, or tolerance change.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

- The exact Netlify CLI dependency graph reports 35 npm audit findings (2 low, 1 moderate, 32 high). The planned official package/version/integrity was preserved; no unrelated forced dependency rewrite was attempted.
- GitHub Actions emitted non-blocking annotations that v4 actions are being forced from deprecated Node 20 to Node 24 and that `.lighthouseci/` had no files to upload. The Lighthouse gate itself passed and the workflow concluded successfully.
- A full-page live dark-theme browser capture timed out once; a fresh viewport capture and DOM state check confirmed the dark theme and 12-tab structure without affecting the page or deployment.

## Authentication Gates

None - the existing authenticated Netlify CLI context completed the deployment.

## Known Stubs

None.

## Threat Review

- Package identity, exact version, integrity, runtime engine, and committed lock state were verified before deployment.
- The site ID remained `31538e21-e9a1-40a0-bf49-8f75307399c7`; no creation or relinking command ran.
- No token or credential was printed, written, staged, or summarized.
- The site commit was pushed and its exact-SHA CI run passed before production deployment.
- The parent records only the already-pushed site revision.

## Residual Human Review

No human action is required for the automated acceptance contract. An operator may optionally review the live light/dark page on the target display for subjective typography and contrast comfort.

## Self-Check: PASSED

- All seven site files and the parent gitlink exist.
- Site commit `9808dcf` exists in the site repository and on `origin/master`.
- Parent commit `ecfde167` points `site` to `9808dcf`.
- GitHub Actions run `30488453395` concluded `success`.
- Netlify deploy `6a6a62e865762f73bc8480eb` is live at the canonical production URL.
- The site worktree is clean.
- Only the uncommitted quick-task planning directory remains in the parent for orchestrator close-out.
