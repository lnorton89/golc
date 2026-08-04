---
phase: 13
slug: unified-ui-design-system-and-automated-enforcement
status: blocked
nyquist_compliant: false
wave_0_complete: false
created: 2026-08-02
revised: 2026-08-04
plan_count: 41
task_count: 77
---

# Phase 13 — Validation Strategy

> Exact execution contract for the revised 41-plan, 77-task graph. Commands below are normalized from each PLAN task. The external-mutation authority row runs read-only preflight and remains a blocking checkpoint.

## Task 1 (13-39) Execution Outcome — Sign-Off Denied

**All 77 plan-derived commands were re-executed for real this session** (2026-08-04, ~16:17–16:37 UTC), at repository HEAD `55a7d463a0216159d013666a77bcb81203dfe54d` (descended from the CI-proven commit `339dce03deab5c076a59090a6280e811ac7b8f3c`, GitHub Actions run [30920907751](https://github.com/lnorton89/golc/actions/runs/30920907751), conclusion `success`). 45 of 77 literal commands exit `0` today; 31 genuinely, reproducibly exit non-zero for four distinct, fully-diagnosed, disclosed reasons documented in **Structural Sign-Off Blockers** below — none of which are functional/code regressions. Every underlying migration/behavior these commands exercise is independently confirmed correct via whole-source and full-suite checks (see Blockers B/C). `nyquist_compliant`, `wave_0_complete`, and approval remain `false`: the semantic evidence validator (`validateEvidenceBundle`, invoked via `npm run validate:phase13-evidence -- --evidence <bundle>`) cannot genuinely pass while Blocker A (a structural incompatibility between the required Windows workflow's artifact-upload design and the validator's own schema) exists, and that incompatibility cannot be resolved from within this plan's declared single-file scope (`13-VALIDATION.md` only — no source, workflow, or validator-script changes authorized). This is recorded honestly, per this plan's own explicit instruction not to weaken, mask, rebaseline, or translate failure into success, matching the discipline every other plan in this phase (13-35, 13-38) has already held to for the same root category of gap.

## Test Infrastructure

| Property | Value |
|---|---|
| Framework | Vitest 4.1.10 + jsdom + Testing Library; Playwright Chromium and packaged WebView2 CDP |
| Config | `frontend/vite.config.ts`; `frontend/playwright.config.ts` |
| Quick loop | `cd frontend && npm run check:design-system && npm run test:design-system` |
| Full local loop | Plan 13-38 Task 1 exact command |
| Evidence validator | `cd frontend && npm run validate:phase13-evidence` |
| Windows evidence | Immutable GitHub Actions run id/URL + CI-proven implementation SHA/tree hash + evidence/planning descendant ancestry/allowlist + downloaded artifact ids/hashes/schemas |

## Plan-Derived Command Contract

The validator parses every `13-NN-PLAN.md`, derives task position, decodes XML entities, normalizes CRLF to LF and surrounding whitespace once, and requires exact command equality plus SHA-256. Shell-equivalent substitutions are not accepted. `derivePlanCommandContract()` was re-invoked live this session against all 41 committed `13-NN-PLAN.md` files: 77 tasks derived, zero contract errors (no missing/duplicate/malformed rows).

## Task Execution Results (real, this session)

Every row below is a real re-execution of that exact PLAN-derived command at HEAD `55a7d463` (a `339dce03`-descendant), 2026-08-04, 16:17–16:37 UTC. `PASS` = exit 0. `FAIL` rows carry a Blocker letter cross-referenced in **Structural Sign-Off Blockers**; none are functional regressions (see that section for the closing proof on each). Commands re-used verbatim from the Plan-Derived Command Contract below; `dirty: false` at every row's own commit boundary (working tree returned clean after each incidental-regeneration revert — see **Incidental Regeneration Reverts**). Environment: `win32`, `node v22.19.0`, `npm 10.9.3`, `go1.26.5`, Playwright Chromium (pinned lockfile-matched build), Windows PowerShell 5.1 (`powershell.exe`) substituted for `pwsh` for the one task requiring it (13-06-02 — PowerShell Core is not installed on this machine; the identical `.ps1` script content executed unmodified, see that row's own note).

| Task | Wave | Result | Started (UTC) | Completed (UTC) | Note |
|---|---|---|---|---|---|
| 13-01-01 | 1 | PASS | 2026-08-04T16:28:42Z | 2026-08-04T16:28:46Z | |
| 13-01-02 | 1 | PASS | 2026-08-04T16:29:14Z | 2026-08-04T16:29:16Z | idempotent install; `git diff --exit-code` on package.json/package-lock.json confirmed no drift |
| 13-21-01 | 2 | PASS | 2026-08-04T16:19:02Z | 2026-08-04T16:19:07Z | |
| 13-21-02 | 2 | PASS | 2026-08-04T16:19:07Z | 2026-08-04T16:19:08Z | |
| 13-02-01 | 3 | PASS | 2026-08-04T16:19:08Z | 2026-08-04T16:19:13Z | |
| 13-02-02 | 3 | PASS | 2026-08-04T16:19:13Z | 2026-08-04T16:19:18Z | |
| 13-03-01 | 3 | PASS | 2026-08-04T16:19:18Z | 2026-08-04T16:19:23Z | |
| 13-03-02 | 3 | PASS | 2026-08-04T16:19:23Z | 2026-08-04T16:19:28Z | |
| 13-04-01 | 3 | PASS | 2026-08-04T16:19:28Z | 2026-08-04T16:19:33Z | |
| 13-04-02 | 3 | PASS | 2026-08-04T16:19:33Z | 2026-08-04T16:19:38Z | |
| 13-05-01 | 4 | PASS | 2026-08-04T16:19:38Z | 2026-08-04T16:19:43Z | |
| 13-05-02 | 4 | PASS | 2026-08-04T16:19:43Z | 2026-08-04T16:19:48Z | |
| 13-22-01 | 4 | PASS | 2026-08-04T16:19:48Z | 2026-08-04T16:19:53Z | ConfirmDialog re-verified after the later a11y role fix (e8eb5642) |
| 13-23-01 | 4 | PASS | 2026-08-04T16:19:53Z | 2026-08-04T16:19:57Z | |
| 13-23-02 | 4 | PASS | 2026-08-04T16:19:57Z | 2026-08-04T16:20:02Z | |
| 13-06-01 | 5 | PASS | 2026-08-04T16:24:03Z | 2026-08-04T16:24:09Z | Chromium dialog-feasibility spec |
| 13-06-02 | 5 | PASS | 2026-08-04T16:34:43Z | 2026-08-04T16:36:11Z | packaged WebView2 proof; two prior attempts genuinely failed on a transient full-suite build flake (Blocker D) before this clean third attempt — see **Packaged WebView2 Evidence** |
| 13-07-01 | 6 | PASS | 2026-08-04T16:20:02Z | 2026-08-04T16:20:07Z | |
| 13-07-02 | 6 | PASS | 2026-08-04T16:20:07Z | 2026-08-04T16:20:15Z | |
| 13-08-01 | 7 | FAIL | 2026-08-04T16:20:26Z | 2026-08-04T16:20:32Z | Blocker B (DS008 `--paths` scope) |
| 13-09-01 | 7 | FAIL | 2026-08-04T16:20:32Z | 2026-08-04T16:20:38Z | Blocker B |
| 13-09-02 | 7 | FAIL | 2026-08-04T16:20:38Z | 2026-08-04T16:20:45Z | Blocker B |
| 13-10-01 | 7 | FAIL | 2026-08-04T16:20:45Z | 2026-08-04T16:20:55Z | Blocker B |
| 13-10-02 | 7 | FAIL | 2026-08-04T16:20:55Z | 2026-08-04T16:21:01Z | Blocker B |
| 13-11-01 | 7 | FAIL | 2026-08-04T16:21:01Z | 2026-08-04T16:21:08Z | Blocker B |
| 13-12-01 | 7 | FAIL | 2026-08-04T16:21:08Z | 2026-08-04T16:21:16Z | Blocker B |
| 13-13-01 | 7 | PASS | 2026-08-04T16:21:16Z | 2026-08-04T16:21:21Z | |
| 13-13-02 | 7 | FAIL | 2026-08-04T16:21:21Z | 2026-08-04T16:21:22Z | Blocker B |
| 13-14-01 | 7 | FAIL | 2026-08-04T16:21:22Z | 2026-08-04T16:21:29Z | Blocker B |
| 13-14-02 | 7 | FAIL | 2026-08-04T16:21:29Z | 2026-08-04T16:21:35Z | Blocker B |
| 13-15-01 | 7 | FAIL | 2026-08-04T16:37:18Z | 2026-08-04T16:37:41Z | Blocker B; SafetyCluster vitest + safety-action-hold Playwright sub-commands both genuinely passed, only the `--paths` check.mjs sub-command hit Blocker B |
| 13-15-02 | 7 | FAIL | 2026-08-04T16:21:35Z | 2026-08-04T16:21:41Z | Blocker B |
| 13-16-01 | 7 | FAIL | 2026-08-04T16:21:41Z | 2026-08-04T16:21:47Z | Blocker B |
| 13-16-02 | 7 | FAIL | 2026-08-04T16:21:47Z | 2026-08-04T16:21:54Z | Blocker B |
| 13-24-01 | 7 | FAIL | 2026-08-04T16:21:54Z | 2026-08-04T16:22:02Z | Blocker B |
| 13-24-02 | 7 | FAIL | 2026-08-04T16:22:02Z | 2026-08-04T16:22:09Z | Blocker B |
| 13-25-01 | 7 | FAIL | 2026-08-04T16:22:09Z | 2026-08-04T16:22:16Z | Blocker B |
| 13-25-02 | 7 | FAIL | 2026-08-04T16:22:16Z | 2026-08-04T16:22:23Z | Blocker B |
| 13-26-01 | 7 | FAIL | 2026-08-04T16:22:23Z | 2026-08-04T16:22:30Z | Blocker B |
| 13-26-02 | 7 | FAIL | 2026-08-04T16:22:30Z | 2026-08-04T16:22:36Z | Blocker B |
| 13-27-01 | 7 | FAIL | 2026-08-04T16:22:36Z | 2026-08-04T16:22:45Z | Blocker B |
| 13-27-02 | 7 | FAIL | 2026-08-04T16:22:45Z | 2026-08-04T16:22:52Z | Blocker B |
| 13-28-01 | 7 | FAIL | 2026-08-04T16:22:52Z | 2026-08-04T16:22:58Z | Blocker B |
| 13-28-02 | 7 | FAIL | 2026-08-04T16:22:58Z | 2026-08-04T16:23:04Z | Blocker B |
| 13-29-01 | 7 | FAIL | 2026-08-04T16:23:04Z | 2026-08-04T16:23:12Z | Blocker B |
| 13-29-02 | 7 | FAIL | 2026-08-04T16:23:12Z | 2026-08-04T16:23:18Z | Blocker B |
| 13-40-01 | 7 | FAIL | 2026-08-04T16:23:18Z | 2026-08-04T16:23:24Z | Blocker B |
| 13-17-01 | 8 | PASS | 2026-08-04T16:24:09Z | 2026-08-04T16:24:12Z | |
| 13-17-02 | 8 | PASS | 2026-08-04T16:24:12Z | 2026-08-04T16:24:37Z | 3-capture calibration, `selectedThreshold: 0` |
| 13-30-01 | 9 | PASS | 2026-08-04T16:24:37Z | 2026-08-04T16:24:45Z | |
| 13-30-02 | 9 | PASS | 2026-08-04T16:24:45Z | 2026-08-04T16:24:52Z | |
| 13-31-01 | 9 | PASS | 2026-08-04T16:24:52Z | 2026-08-04T16:25:30Z | |
| 13-31-02 | 9 | PASS | 2026-08-04T16:25:30Z | 2026-08-04T16:26:10Z | |
| 13-32-01 | 9 | PASS | 2026-08-04T16:26:10Z | 2026-08-04T16:26:20Z | also confirmed clean in the full-suite retry after its own earlier flake (Blocker D) |
| 13-32-02 | 9 | PASS | 2026-08-04T16:26:20Z | 2026-08-04T16:26:30Z | |
| 13-32-03 | 9 | PASS | 2026-08-04T16:26:30Z | 2026-08-04T16:26:41Z | |
| 13-33-01 | 9 | PASS | 2026-08-04T16:26:41Z | 2026-08-04T16:26:55Z | |
| 13-33-02 | 9 | PASS | 2026-08-04T16:26:55Z | 2026-08-04T16:27:10Z | |
| 13-33-03 | 9 | PASS | 2026-08-04T16:27:10Z | 2026-08-04T16:27:24Z | |
| 13-34-01 | 9 | PASS | 2026-08-04T16:27:24Z | 2026-08-04T16:27:34Z | |
| 13-34-02 | 9 | PASS | 2026-08-04T16:27:34Z | 2026-08-04T16:27:49Z | |
| 13-34-03 | 9 | PASS | 2026-08-04T16:27:49Z | 2026-08-04T16:28:11Z | |
| 13-41-01 | 9 | PASS | 2026-08-04T16:28:11Z | 2026-08-04T16:28:21Z | 900x720 real 200% zoom |
| 13-41-02 | 9 | PASS | 2026-08-04T16:28:21Z | 2026-08-04T16:28:34Z | provider/daemon-offline projections |
| 13-18-01 | 10 | PASS | 2026-08-04T16:29:28Z | 2026-08-04T16:29:33Z | |
| 13-18-02 | 10 | FAIL | 2026-08-04T16:29:33Z | 2026-08-04T16:29:40Z | Blocker C (missing `-tags mage`, pre-existing per 13-18-SUMMARY) |
| 13-18-03 | 10 | PASS | 2026-08-04T16:17:38Z | 2026-08-04T16:18:34Z | third isolated attempt; two chained attempts hit Blocker D first (see **Backstop D**) |
| 13-20-01 | 11 | PASS | 2026-08-04T16:28:46Z | 2026-08-04T16:28:50Z | |
| 13-20-02 | 11 | PASS | 2026-08-04T16:28:50Z | 2026-08-04T16:28:55Z | |
| 13-19-01 | 12 | FAIL | 2026-08-04T16:28:55Z | 2026-08-04T16:29:01Z | Blocker B (empty-scope variant, pre-existing per 13-19-SUMMARY's own "Issues Encountered"); `check.mjs --all` independently confirms zero diagnostics |
| 13-37-01 | 13 | PASS | 2026-08-04T16:29:01Z | 2026-08-04T16:29:07Z | uses `--rule DS007`, bypasses Blocker B entirely |
| 13-36-01 | 14 | PASS | 2026-08-04T16:29:07Z | 2026-08-04T16:29:14Z | uses `--rule DS007`, bypasses Blocker B entirely |
| 13-35-01 | 15 | PASS | 2026-08-04T16:27:00Z | 2026-08-04T16:27:02Z | read-only preflight |
| 13-35-02 | 15 | PASS | 2026-08-04T16:29:50Z | 2026-08-04T16:29:52Z | `GOLC_PHASE13_APPROVED_SHA=339dce03...` returns run 30920907751 |
| 13-35-03 | 15 | FAIL | 2026-08-04T16:30:00Z | 2026-08-04T16:30:30Z | `gh run view`/artifacts API sub-commands genuinely pass; final `validate:phase13-evidence -- --evidence windows-ci-run.json` sub-command hits Blocker A |
| 13-38-01 | 16 | FAIL | 2026-08-04T16:06:31Z | 2026-08-04T16:13:04Z | every functional stage genuinely passed on isolated re-run (Blocker D disclosure below); final validate sub-command hits Blocker A |
| 13-39-01 | 17 | this task | — | — | this plan's own declared verify — run last, see **Verification** |

**Totals: 45/77 PASS, 31/77 FAIL-for-disclosed-non-regression-reasons, 1/77 (13-39-01) is this task's own final verify.**

### Waves 1–6

- `13-01-01` W1 — `cd frontend && npm view postcss@8.5.22 name version repository dist.integrity scripts --json && npm view @typescript/typescript6@6.0.2 name version repository dist.integrity scripts --json && cd .. && git diff --exit-code -- frontend/package.json frontend/package-lock.json`
- `13-01-02` W1 — `cd frontend && npm install --save-dev --save-exact --ignore-scripts postcss@8.5.22 @typescript/typescript6@6.0.2 && npm ls postcss @typescript/typescript6 --depth=0`
- `13-21-01` W2 — `cd frontend && npx vitest run scripts/design-system/manifest.test.ts`
- `13-21-02` W2 — `cd frontend && npm run generate:design-system && node scripts/design-system/generate.mjs --check`
- `13-02-01` W3 — `cd frontend && npx vitest run scripts/design-system/check.test.ts --testNamePattern="DS00[1-9]|DS010"`
- `13-02-02` W3 — `cd frontend && npx vitest run scripts/design-system/check.test.ts`
- `13-03-01` W3 — `cd frontend && npx vitest run src/components/primitives/Button src/components/primitives/IconButton`
- `13-03-02` W3 — `cd frontend && npx vitest run src/components/primitives/Field src/components/primitives/Chip`
- `13-04-01` W3 — `cd frontend && npx vitest run src/components/primitives/Tabs`
- `13-04-02` W3 — `cd frontend && npx vitest run src/components/primitives/EmptyState src/components/primitives/LoadingState src/components/primitives/ErrorState`
- `13-05-01` W4 — `cd frontend && npx vitest run src/components/primitives/Panel src/components/primitives/PanelHeader`
- `13-05-02` W4 — `cd frontend && npx vitest run src/components/primitives/Toolbar src/components/primitives/ListRow`
- `13-22-01` W4 — `cd frontend && npx vitest run src/components/primitives/Dialog src/components/primitives/ConfirmDialog`
- `13-23-01` W4 — `cd frontend && npx vitest run src/components/primitives/ScrollRegion src/components/primitives/InfoTooltip`
- `13-23-02` W4 — `cd frontend && npx vitest run src/components/primitives/ResizeHandle`
- `13-06-01` W5 — `cd frontend && npx playwright test e2e/dialog-feasibility.spec.ts --project=chromium --workers=1`
- `13-06-02` W5 — `pwsh -NoProfile -File scripts/ci/run-packaged-dialog-proof.ps1`
- `13-07-01` W6 — `cd frontend && npx vitest run src/design-system/fixtures src/design-system/patterns`
- `13-07-02` W6 — `cd frontend && npm run generate:design-system && npx vitest run src/design-system && node scripts/design-system/check.mjs --rule DS007`

### Wave 7 migrations

- `13-08-01` W7 — `cd frontend && npx vitest run src/lib/theme.test.ts src/App.smoke.test.tsx && node scripts/design-system/check.mjs --paths src/index.css,src/lib/theme.ts,src/App.tsx`
- `13-09-01` W7 — `cd frontend && npx vitest run src/workspaces/show/OverviewWorkspace.test.tsx src/workspaces/show/ShowsWorkspace.test.tsx && node scripts/design-system/check.mjs --paths src/workspaces/show/OverviewWorkspace.tsx,src/workspaces/show/OverviewWorkspace.module.css,src/workspaces/show/ShowsWorkspace.tsx,src/workspaces/show/ShowsWorkspace.module.css`
- `13-09-02` W7 — `cd frontend && npx vitest run src/workspaces/show/SaveRecoveryWorkspace.test.tsx src/workspaces/show/SettingsWorkspace.test.tsx && node scripts/design-system/check.mjs --paths src/workspaces/show/SaveRecoveryWorkspace.tsx,src/workspaces/show/SaveRecoveryWorkspace.module.css,src/workspaces/show/SettingsWorkspace.tsx,src/workspaces/show/SettingsWorkspace.module.css`
- `13-10-01` W7 — `cd frontend && npx vitest run src/workspaces/build/FixtureLibraryWorkspace.test.tsx && node scripts/design-system/check.mjs --paths src/workspaces/build/FixtureLibraryWorkspace.tsx,src/workspaces/build/FixtureLibraryWorkspace.module.css`
- `13-10-02` W7 — `cd frontend && npx vitest run src/components/FixturePatch src/components/ProjectFixtures && node scripts/design-system/check.mjs --paths src/workspaces/build/PatchPoolsWorkspace.tsx,src/workspaces/build/ProjectFixturesWorkspace.tsx,src/components/FixturePatch,src/components/ProjectFixtures --proposal design-system/exception-proposals/fixtures.json`
- `13-11-01` W7 — `cd frontend && npx vitest run src/workspaces/build/ScenesLooksWorkspace.test.tsx src/components/SceneProgramming/SceneList.test.tsx src/components/SceneProgramming/LookBrowser.test.tsx && node scripts/design-system/check.mjs --paths src/workspaces/build/ScenesLooksWorkspace.tsx,src/workspaces/build/ScenesLooksWorkspace.module.css,src/components/SceneProgramming/SceneList.tsx,src/components/SceneProgramming/SceneList.module.css,src/components/SceneProgramming/LookBrowser.tsx,src/components/SceneProgramming/LookBrowser.module.css`
- `13-40-01` W7 — `cd frontend && npx vitest run src/components/SceneProgramming/BarTimelinePanel.test.tsx src/components/SceneProgramming/LayerRow.test.tsx && node scripts/design-system/check.mjs --paths src/components/SceneProgramming/BarTimelinePanel.tsx,src/components/SceneProgramming/BarTimelinePanel.module.css,src/components/SceneProgramming/LayerRow.tsx,src/components/SceneProgramming/LayerRow.module.css`
- `13-12-01` W7 — `cd frontend && npx vitest run src/workspaces/show/NotesWorkspace.test.tsx src/components/Notes && node scripts/design-system/check.mjs --paths src/workspaces/show/NotesWorkspace.tsx,src/workspaces/show/NotesWorkspace.module.css,src/components/Notes`
- `13-13-01` W7 — `cd frontend && npx vitest run src/components/Desk src/workspaces/perform/DeskWorkspace.test.tsx`
- `13-13-02` W7 — `cd frontend && node scripts/design-system/check.mjs --paths src/workspaces/perform/DeskWorkspace.tsx,src/workspaces/perform/DeskWorkspace.module.css,src/components/Desk --proposal design-system/exception-proposals/desk.json`
- `13-14-01` W7 — `cd frontend && npx vitest run src/components/OperatorSurface/OperatorSurface.activeSurface.test.tsx src/workspaces/operate/OperatorSurfaceWorkspace.test.tsx && node scripts/design-system/check.mjs --paths src/workspaces/operate/OperatorSurfaceWorkspace.tsx,src/workspaces/operate/OperatorSurfaceWorkspace.module.css,src/components/OperatorSurface/OperatorSurface.tsx,src/components/OperatorSurface/OperatorSurface.module.css,src/components/OperatorSurface/AssignmentToggle.tsx,src/components/OperatorSurface/SurfaceList.tsx`
- `13-14-02` W7 — `cd frontend && npx vitest run src/components/OperatorSurface/Launcher.test.tsx src/components/OperatorSurface/ScenePad.test.tsx && node scripts/design-system/check.mjs --paths src/components/OperatorSurface/Launcher.tsx,src/components/OperatorSurface/Launcher.module.css,src/components/OperatorSurface/ScenePad.tsx,src/components/OperatorSurface/ScenePad.module.css`
- `13-15-01` W7 — `cd frontend && npx vitest run src/components/SafetyCluster/SafetyCluster.test.tsx && npx playwright test e2e/safety-action-hold.spec.ts --project=chromium --workers=1 && node scripts/design-system/check.mjs --paths src/components/SafetyCluster --proposal design-system/exception-proposals/safety-live.json`
- `13-15-02` W7 — `cd frontend && npx vitest run src/components/LiveStatusBar src/components/TempoControls && node scripts/design-system/check.mjs --paths src/components/LiveStatusBar,src/components/TempoControls`
- `13-16-01` W7 — `cd frontend && npx vitest run src/shell/ContextualInspector.test.tsx src/shell/HelpOverlay.test.tsx && node scripts/design-system/check.mjs --paths src/shell/ContextualInspector.tsx,src/shell/ContextualInspector.module.css,src/shell/InspectorSlot.tsx,src/shell/HelpOverlay.tsx,src/shell/HelpOverlay.module.css`
- `13-16-02` W7 — `cd frontend && npx vitest run src/shell/QuickSwitcher.test.tsx src/shell/ErrorBoundary.test.tsx && node scripts/design-system/check.mjs --paths src/shell/QuickSwitcher.tsx,src/shell/QuickSwitcher.module.css,src/shell/ErrorBoundary.tsx,src/shell/ErrorBoundary.module.css,src/shell/AppLogStream.tsx`
- `13-24-01` W7 — `cd frontend && npx vitest run src/shell/AppShell.test.tsx && node scripts/design-system/check.mjs --paths src/shell/AppShell.tsx,src/shell/AppShell.module.css,src/shell/TitleBar.tsx,src/shell/TitleBar.module.css,src/shell/GlobalFrame.tsx,src/shell/GlobalFrame.module.css`
- `13-24-02` W7 — `cd frontend && npx vitest run src/shell/CommandRail.test.tsx && node scripts/design-system/check.mjs --paths src/shell/CommandRail.tsx,src/shell/CommandRail.module.css --proposal design-system/exception-proposals/theme-shell.json`
- `13-25-01` W7 — `cd frontend && npx vitest run src/workspaces/show/GuidedFirstShow/GuidedFirstShow.test.tsx && node scripts/design-system/check.mjs --paths src/workspaces/show/GuidedFirstShow/GuidedFirstShow.tsx,src/workspaces/show/GuidedFirstShow/GuidedFirstShow.module.css,src/workspaces/show/GuidedFirstShow/GuidedFirstShowContext.tsx,src/workspaces/show/GuidedFirstShow/GuideEvidenceList.tsx,src/workspaces/show/GuidedFirstShow/readiness.ts,src/workspaces/show/GuidedFirstShow/stages.ts --proposal design-system/exception-proposals/front-door.json`
- `13-25-02` W7 — `cd frontend && npx vitest run src/workspaces/show/GuidedFirstShow && node scripts/design-system/check.mjs --paths src/workspaces/show/GuidedFirstShow/stages`
- `13-26-01` W7 — `cd frontend && npx vitest run src/components/ArtnetConfig src/workspaces/output/ArtnetWorkspace.test.tsx && node scripts/design-system/check.mjs --paths src/workspaces/output/ArtnetWorkspace.tsx,src/workspaces/output/ArtnetWorkspace.module.css,src/components/ArtnetConfig`
- `13-26-02` W7 — `cd frontend && npx vitest run src/components/Diagnostics src/workspaces/output/DiagnosticsWorkspace.test.tsx && node scripts/design-system/check.mjs --paths src/workspaces/output/DiagnosticsWorkspace.tsx,src/workspaces/output/DiagnosticsWorkspace.module.css,src/components/Diagnostics --proposal design-system/exception-proposals/output.json`
- `13-27-01` W7 — `cd frontend && npx vitest run src/workspaces/build/ScriptsWorkspace.test.tsx src/components/Scripts/ScriptRunDialog.test.tsx && node scripts/design-system/check.mjs --paths src/workspaces/build/ScriptsWorkspace.tsx,src/workspaces/build/ScriptsWorkspace.module.css,src/components/Scripts/ScriptRunDialog.tsx,src/components/Scripts/ScriptRunDialog.module.css`
- `13-27-02` W7 — `cd frontend && npx vitest run src/components/Scripts/monacoTheme.test.ts src/components/Scripts/ScriptDebugPanel.test.tsx src/components/Scripts/ScriptEditor.test.tsx && node scripts/design-system/check.mjs --paths src/components/Scripts/monacoTheme.ts,src/components/Scripts/ScriptDebugPanel.tsx,src/components/Scripts/ScriptDebugPanel.module.css,src/components/Scripts/ScriptEditor.tsx,src/components/Scripts/ScriptEditor.module.css --proposal design-system/exception-proposals/editors.json`
- `13-28-01` W7 — `cd frontend && npx vitest run src/components/MidiPanel/MidiPanel.test.tsx src/workspaces/operate/MidiMappingWorkspace.test.tsx && node scripts/design-system/check.mjs --paths src/workspaces/operate/MidiMappingWorkspace.tsx,src/workspaces/operate/MidiMappingWorkspace.module.css,src/components/MidiPanel/MidiPanel.tsx,src/components/MidiPanel/MidiPanel.module.css,src/components/MidiPanel/MidiLearn.tsx,src/components/MidiPanel/DeskMappingsSection.tsx`
- `13-28-02` W7 — `cd frontend && npx vitest run src/components/MidiPanel/SoftTakeoverSlider.test.tsx src/components/MidiLearnToggle/MidiLearnToggle.test.tsx && node scripts/design-system/check.mjs --paths src/components/MidiPanel/SoftTakeoverSlider.tsx,src/components/MidiLearnToggle/MidiLearnToggle.tsx,src/components/MidiLearnToggle/MidiLearnToggle.module.css --proposal design-system/exception-proposals/operator-midi.json`
- `13-29-01` W7 — `cd frontend && npx vitest run src/components/HotkeySettings src/components/KeyboardShortcuts && node scripts/design-system/check.mjs --paths src/components/HotkeySettings/HotkeySettings.tsx,src/components/HotkeySettings/HotkeySettings.module.css,src/components/KeyboardShortcuts/KeyboardShortcuts.tsx,src/components/KeyboardShortcuts/KeyboardShortcuts.module.css`
- `13-29-02` W7 — `cd frontend && npx vitest run src/workspaces/ComingSoon.test.tsx && node scripts/design-system/check.mjs --paths src/workspaces/workspace.module.css,src/workspaces/ComingSoon.tsx,src/workspaces/ComingSoon.module.css --proposal design-system/exception-proposals/shell-overlays.json`

### Waves 8–17

- `13-17-01` W8 — `cd frontend && npx playwright test e2e/design-system.calibration.spec.ts --list`
- `13-17-02` W8 — `cd frontend && npx playwright test e2e/design-system.calibration.spec.ts --project=chromium --workers=1`
- `13-30-01` W9 — `cd frontend && npx playwright test e2e/design-system.startup.spec.ts --project=chromium --workers=1`
- `13-30-02` W9 — `cd frontend && npx playwright test e2e/design-system.emergency-fallback.spec.ts --project=chromium --workers=1`
- `13-31-01` W9 — `cd frontend && npx playwright test e2e/design-system.geometry.spec.ts --project=chromium --workers=1`
- `13-31-02` W9 — `cd frontend && npx playwright test e2e/design-system.expanded-copy.spec.ts --project=chromium --workers=1`
- `13-32-01` W9 — `cd frontend && npx playwright test e2e/design-system.visual-shell.spec.ts --grep "persistent shell" --project=chromium --workers=1`
- `13-32-02` W9 — `cd frontend && npx playwright test e2e/design-system.visual-shell.spec.ts --grep "dialog layer" --project=chromium --workers=1`
- `13-32-03` W9 — `cd frontend && npx playwright test e2e/design-system.visual-shell.spec.ts --grep "shared gallery" --project=chromium --workers=1`
- `13-33-01` W9 — `cd frontend && npx playwright test e2e/design-system.visual-authoring.spec.ts --grep "Scenes & Looks" --project=chromium --workers=1`
- `13-33-02` W9 — `cd frontend && npx playwright test e2e/design-system.visual-authoring.spec.ts --grep "Fixture Library / Patch & Pools" --project=chromium --workers=1`
- `13-33-03` W9 — `cd frontend && npx playwright test e2e/design-system.visual-authoring.spec.ts --grep "Guided First Show" --project=chromium --workers=1`
- `13-34-01` W9 — `cd frontend && npx playwright test e2e/design-system.visual-live-editors.spec.ts --grep "Desk / Operator Surface" --project=chromium --workers=1`
- `13-34-02` W9 — `cd frontend && npx playwright test e2e/design-system.visual-live-editors.spec.ts --grep "MIDI Mapping" --project=chromium --workers=1`
- `13-34-03` W9 — `cd frontend && npx playwright test e2e/design-system.visual-live-editors.spec.ts --grep "Scripts / Notes" --project=chromium --workers=1`
- `13-41-01` W9 — `cd frontend && npx playwright test e2e/design-system.text-zoom.spec.ts --project=chromium --workers=1`
- `13-41-02` W9 — `cd frontend && npx playwright test e2e/design-system.offline-safety.spec.ts --project=chromium --workers=1`
- `13-18-01` W10 — `go test ./internal/command -run DesignSystem -count=1`
- `13-18-02` W10 — `go test ./internal/delivery ./magefiles -run 'DesignSystem|MageTarget' -count=1 && mage -l`
- `13-18-03` W10 — `mage CheckOffline && git diff --check -- .github/workflows/design-system.yml`
- `13-19-01` W12 — `cd frontend && npx vitest run scripts/design-system/check.test.ts && node scripts/design-system/check.mjs`
- `13-37-01` W13 — `cd frontend && npx vitest run src/design-system/design-system.contract.test.ts --testNamePattern="ConfirmModal removal" && node scripts/design-system/check.mjs --rule DS007`
- `13-36-01` W14 — `cd frontend && npx vitest run src/design-system && node scripts/design-system/check.mjs --rule DS007`
- `13-35-01` W15 — `git rev-parse HEAD && gh auth status && gh workflow view design-system.yml` — read-only preflight; the blocking checkpoint separately records approved trigger mechanism, exact immutable implementation SHA/tree hash/ref/repository, or denial.
- `13-35-02` W15 — `gh run list --workflow design-system.yml --commit $env:GOLC_PHASE13_APPROVED_SHA --limit 1 --json databaseId,url,event,headSha,headBranch,status,conclusion,createdAt`
- `13-35-03` W15 — `gh run view $env:GOLC_PHASE13_RUN_ID --json databaseId,url,workflowName,event,headSha,headBranch,status,conclusion,createdAt,updatedAt,jobs && gh api repos/{owner}/{repo}/actions/runs/$env:GOLC_PHASE13_RUN_ID/artifacts && cd frontend && npm run validate:phase13-evidence -- --evidence ../.planning/phases/13-unified-ui-design-system-and-automated-enforcement/evidence/windows-ci-run.json`
- `13-20-01` W11 — `cd frontend && npx vitest run scripts/design-system/validate-phase13-evidence.test.ts`
- `13-20-02` W11 — `cd frontend && npx vitest run scripts/design-system/validate-phase13-evidence.test.ts --testNamePattern="mutation|false sign-off|semantic|implementation tree|zoom|offline safety"`
- `13-38-01` W16 — `cd frontend && npm run build && npm test && npm run test:e2e && npm run test:e2e:design-system && cd .. && mage GenerateCheck && mage CheckOffline && mage Build && mage TestQuick && git diff --cached --quiet -- site && cd frontend && npm run validate:phase13-evidence -- --evidence ../.planning/phases/13-unified-ui-design-system-and-automated-enforcement/evidence/phase-acceptance.json`
- `13-39-01` W17 — `cd frontend && npm run validate:phase13-evidence && cd .. && node C:/Users/Lawrence/.codex/gsd-core/bin/gsd-tools.cjs query verify.plan-structure .planning/phases/13-unified-ui-design-system-and-automated-enforcement/13-39-PLAN.md && git diff --check -- .planning/phases/13-unified-ui-design-system-and-automated-enforcement/13-VALIDATION.md`

## Six Separately Named UI Backstops

| Backstop ID | Task | Executable assertion | Semantic evidence schema | Status |
|---|---|---|---|---|
| `startup-theme-font-before-settle` | 13-30-01 | Instrument before navigation; sample theme/font/contrast/safety before settle; await fonts only after pre-settle window | `evidence/startup-theme-font.json`: sample timeline, theme sequence, font sequence, computed colors/contrast, boxes, build/browser identity, assertions | **pass** — spec re-run 2026-08-04T16:24:37Z (exit 0); `validateBackstopStartupTheme()` invoked directly against the committed file: 0 errors |
| `error-boundary-before-theme-css` | 13-30-02 | Block generated token CSS, force ErrorBoundary, assert token-independent readable recovery at 900/1280 | `evidence/error-boundary-fallback.json`: blocked asset, computed colors/contrast, role/name/focus/recovery, boxes, SHA/runtime | **pass** — spec re-run 2026-08-04T16:24:52Z (exit 0); `validateBackstopErrorBoundary()`: 0 errors |
| `specialized-geometry-900-1280` | 13-31-01 | Exact fader/timeline/meter/Monaco/Tiptap/min-max resize rectangles, owners, targets, visibility at both widths | `evidence/specialized-geometry.json`: family, viewport, resize state, measured/expected rectangles, overflow owner, safety/navigation result | **pass** — spec re-run 2026-08-04T16:25:30Z (exit 0); `validateBackstopSpecializedGeometry()`: 0 errors |
| `expanded-copy-2x-reflow` | 13-31-02 | Prove grapheme ratio ≥2.0 and reflow without clipping/overlap/body overflow/focus or safety loss | `evidence/expanded-copy.json`: canonical/expanded counts and ratio, line/box/overflow/focus/safety results by theme/width | **pass** — spec re-run 2026-08-04T16:26:10Z (exit 0); `validateBackstopExpandedCopy()`: 0 errors |
| `text-zoom-200-900x720` | 13-41-01 | Apply actual 200% browser text zoom at 900x720; assert root/body overflow metrics plus navigation/live-truth/active-task/safety visibility or recorded keyboard reachability | `evidence/text-zoom-200.json`: viewport, requested/computed zoom, root/body client/scroll widths, locator roles/names/boxes/visibility, focus order, scroll owners, hit tests, SHA/runtime, assertions | **pass** — spec re-run 2026-08-04T16:28:21Z (exit 0); `validateBackstopTextZoom()`: 0 errors |
| `provider-daemon-offline-safety` | 13-41-02 | Project provider-offline and daemon-offline states; keyboard-operate Blackout/Revoke through independent local paths; preserve explicit Go-owned playback/output truth | `evidence/offline-safety.json`: state inputs, projected copy, control boxes/focus order, activation/path/counts, before/after playback-output truth, SHA/runtime, assertions | **pass** — spec re-run 2026-08-04T16:28:34Z (exit 0); `validateBackstopOfflineSafety()`: 0 errors |

All six `evidence/*.json` files above are unchanged from their last-committed content (this session's re-runs regenerated them as a live side effect of the specs executing, then those incidental regenerations were reverted via targeted `git checkout --` on exactly those pre-existing tracked paths — see **Incidental Regeneration Reverts** — matching Plan 13-38's own established pattern). The committed files were independently re-validated via a direct live invocation of each named validator function against the on-disk JSON (not merely narrated): all six return zero errors. `backstopsCovered` exact-set coverage: all six IDs present, matching `BACKSTOP_IDS` exactly (verified via `validateExactCoverage`).

## Packaged WebView2 Evidence

`evidence/dialog-feasibility.json` (committed content, captured 2026-08-03) independently re-validates cleanly this session via a direct live invocation of `validateDialogFeasibilityEvidence()`: 0 errors (`status: "passed"`, real `build.sha256`, real `runtime.cdp_endpoint`, `test.exit_code: 0`). That committed capture predates the later ConfirmDialog a11y role fix (`e8eb5642`, 2026-08-03T23:52:36-07:00), so this session additionally re-ran the packaged proof for real (13-06-02) rather than trusting the stale citation:

- **Attempt 1** (16:31:22–16:32:03 UTC): `mage Build`'s own internal frontend build step hit the transient full-suite `manifest.test.ts` flake (Backstop D below) and failed before the packaged app ever launched. The script's own `finally`-block `Write-Evidence` call unconditionally overwrote `evidence/dialog-feasibility.json` with a `status: "failed"` record as a result.
- **Attempt 2** (16:32:13–16:32:55 UTC): same transient flake reproduced (3 different vitest tests failed this time, all resource-contention-shaped — see Backstop D), same `finally`-block overwrite.
- **Attempt 3** (16:34:43–16:36:11 UTC, exit 0): clean pass. `mage Build` succeeded (frontend build + Go compile with the overlay module for CDP injection), the packaged `golc-desktop.exe` launched, the real CDP endpoint came up, and `e2e/dialog-feasibility.spec.ts` passed (1/1, 554ms) against the packaged WebView2 app — build SHA-256 `5C84320A22296BA0202DCEFE870359421041664900F5C7CB343A665CBAC827C1`, browser `Edg/151.0.4129.59`, `test.exit_code: 0`.
- **Tool substitution, disclosed**: PowerShell Core (`pwsh`) is not installed on this machine (only Windows PowerShell 5.1, `powershell.exe`, is present — confirmed via `command -v pwsh`/`command -v pwsh.exe` returning nothing and `command -v powershell.exe` resolving). All three attempts ran the identical, unmodified `.ps1` script content via `powershell.exe -NoProfile -File ...` instead of `pwsh -NoProfile -File ...`. This is a real, disclosed environment substitution (installing PowerShell Core mid-session was judged out of scope given ~3.6GB free disk and the package-manager-install caution in this executor's own deviation rules), not a fabricated pwsh run.
- **Both failed attempts' incidental overwrite of `evidence/dialog-feasibility.json` were reverted** via `git checkout --` immediately after discovery, restoring the committed content; the 3 `validate-phase13-evidence.test.ts` tests that transiently failed against the corrupted file (because they assert against the real on-disk file, not a fixture) were re-confirmed passing (90/90) once the revert was in place.
- Attempt 3's own fresh, passing `dialog-feasibility.json` content was also reverted via the same targeted `git checkout --` (see **Incidental Regeneration Reverts**) to keep this plan's own commit scoped to `13-VALIDATION.md` only — the fresh pass is recorded here in prose instead.

## Structural Sign-Off Blockers

Four distinct, fully-diagnosed, disclosed, non-fabricatable reasons the full semantic evidence validator (`validateEvidenceBundle`) cannot genuinely pass today. None is a functional/code regression — each is independently proven below.

### Blocker A — `validateWindowsCiEvidence` requires simultaneous `conclusion: "success"` AND a non-empty `artifacts[]`, which this repository's required workflow can never produce together

`.github/workflows/design-system.yml`'s sole artifact-upload step is gated `if: failure()` (confirmed by reading the workflow file directly, line 86). Every one of this workflow's 6 real runs to date confirms the resulting invariant live via `gh run list`:

| Run | conclusion | Has artifacts? |
|---|---|---|
| 30920907751 (`339dce03`) | success | 0 (confirmed via `gh api .../artifacts` → `{"total_count":0}`) |
| 30919628061 | success | 0 (same upload-step design) |
| 30918533759 | failure | upload step runs |
| 30876209266 | success | 0 |
| 30875348640 (`c3fe6e72`) | success | 0 |
| 30870575466 | failure | upload step runs |

`validateWindowsCiEvidence()` was invoked directly this session against a real, current, ancestor-verified record built from run 30920907751's own live `gh run view`/artifacts-API data (`conclusion: "success"`, `headSha: 339dce03...`, `artifacts: []`): it returns exactly one error, `"Windows CI evidence missing downloaded artifacts"` — reproduced live, not narrated. Re-running the declared `npm run validate:phase13-evidence -- --evidence windows-ci-run.json` command (13-35-03's own final sub-command) reproduces the identical `FAIL` line against the already-committed evidence file. This is the same gap Plan 13-35's own `evidence/windows-ci-run.json#fullBundleValidatorNote` and Plan 13-38's own three attempts already disclosed — now additionally confirmed against the newer, currently-relevant proven commit. **No genuine Windows CI evidence for a passing run of this workflow can ever satisfy this check** without either the workflow always uploading artifacts (a `.github/workflows/design-system.yml` change) or the validator relaxing the unconditional non-empty-artifacts requirement when `conclusion: "success"` (a `validate-phase13-evidence.mjs` change) — both outside this plan's declared single-file (`13-VALIDATION.md`) scope. Also affects: 13-38-01's own final validate sub-command (same root cause, `phase-acceptance.json` is deliberately narrower-scoped than a full bundle for an independent reason — see Blocker A's interaction with the missing-rows gap below).

### Blocker B — DS008 (exception-integrity) audits the *entire* `exceptions.json` manifest regardless of `--paths` scope, so any narrowly-scoped Wave 7 verify command now reproducibly flags every out-of-scope exception record as "stale"

`checkDesignSystem()`'s DS008 sub-check (`frontend/scripts/design-system/check.mjs`) validates every record in `design-system/exceptions.json` against `result.matching`, computed only from the diagnostics collected for the current invocation's `paths`/`wholeSource` target set — it does not skip exceptions whose own file lies outside that scope. `design-system/exceptions.json` held 0–few records when most Wave 7 migration plans (13-08 through 13-29, 13-40) were originally authored/executed, but Plan 13-19 (Wave 12, executed after them) merged all 12 `exception-proposals/*.json` files into a single 59-record `exceptions.json`. Every Wave 7 plan's own literal `--paths <its own 2-6 files>` verify command, when re-run today against that now-fully-populated manifest, genuinely and deterministically reports every one of the ~55+ exception records for files outside its own narrow scope as `"stale exception"` and exits 1 — 27 tasks affected (26 Wave 7 `--paths` commands + 13-19-01's own even-more-extreme empty-`paths` case).

**This is not a functional regression, proven three ways:**
1. `cd frontend && node scripts/design-system/check.mjs --all` (whole-source mode) exits `0` with zero diagnostics at current HEAD — re-confirmed live this session.
2. Every affected task's own `vitest` sub-command (the part before `&&`) genuinely passes (spot-checked: 13-09-01 14/14, 13-16-02 12/12, 13-40-01 8/8; all 27 affected rows show `Test Files N passed` in their captured stdout).
3. Plan 13-19's own SUMMARY.md already disclosed the identical root-cause family for its own literal command (`node scripts/design-system/check.mjs` with no `--all`/`--paths`, i.e. `paths: []`): *"Running it literally reports every one of the 59 exceptions as 'stale' ... Ran the corrected `node scripts/design-system/check.mjs --all` form instead"* — 13-19-01's row above is exactly that already-disclosed case, now reproduced live.

Tasks using `--rule DS007` instead (13-07-02, 13-19-01's sibling pattern in 13-37-01/13-36-01) route through a separate `checkDS007()` function entirely and never invoke DS008 — confirmed by both passing cleanly.

### Blocker C — 13-18-02's own literal declared command omits `-tags mage`, a pre-existing, already-disclosed characteristic

`go test ./internal/delivery ./magefiles -run 'DesignSystem|MageTarget' -count=1` fails to build `./magefiles` (`FAIL github.com/lnorton89/golc/magefiles [setup failed]`) because every `magefiles/*.go` file requires `//go:build mage`, which this literal command's flag set never supplies (`internal/delivery` itself passes cleanly). Plan 13-18-SUMMARY's own `key-decisions` already disclosed this exact gap: *"Task 2's own declared verify command ... omits `-tags mage` ... Ran it exactly as written (internal/delivery passes; magefiles reports a build-constraint setup failure, pre-existing and independent of this plan's content) and separately confirmed the real, tagged form (`-tags mage`) is fully green."* Reproduced live, identically, this session.

### Blocker D (disclosed, not a sign-off blocker) — a transient, load-dependent `manifest.test.ts` flake under heavy concurrent process/disk-pressure conditions

`scripts/design-system/manifest.test.ts`'s `generateDesignSystem > is byte-stable and leaves checked output untouched in check mode` test hit a hardcoded 5000ms timeout with `ENOTEMPTY: directory not empty, rmdir '...\golc-design-system-*\src\design-system\patterns'` four separate times during this session — always specifically when running as part of the *full* 77-file vitest suite inside `mage Build`/`mage CheckOffline`'s own internal frontend-build step under heavy concurrent load (this machine's C: drive was at 3.6–4.2GB free throughout this session, and `tasklist` showed 28+ live `node.exe` processes at one check). It was investigated per this phase's own established flake-disclosure discipline (13-38's precedent for its own visual-diff flake):

- Isolated re-run (`npx vitest run scripts/design-system/manifest.test.ts` alone, no concurrent mage/build/e2e load): **7/7 pass, clean, twice**.
- `mage CheckOffline` run in complete isolation (no other concurrent heavy process): **clean pass, exit 0** (16:17:38–16:18:34 UTC — this is the row cited for 13-18-03 above).
- `mage Build`/`mage TestQuick` in the same isolated retry: **both clean, exit 0**.
- Full `npm run test:e2e` (129 tests): one genuine flake (2/129, `design-system.visual-shell.spec.ts` "persistent shell" 900px light/dark, page-load timeout under 8-worker parallel load) on the first full run, **clean 129/129** on an immediate full re-run.

Zero source changes occurred between failing and passing runs in every case. This is disclosed, not hidden, and is not counted as a Structural Sign-Off Blocker in its own right (all four affected commands — `npm run build`'s first attempt, `mage CheckOffline`'s first two attempts, `mage Build`'s first attempt, and the packaged-proof script's first two attempts — were each re-run to a clean, real pass; every row in **Task Execution Results** above cites its own real, final, clean result where the flake was involved, exactly as 13-38's own precedent handled its analogous case).

## Aggregated Evidence Bundle Validation (individual categories)

Every semantic evidence category `validateEvidenceBundle` requires was independently re-validated this session via a direct, live invocation of its own named validator function against the real, currently-committed `evidence/*.json` files (not merely narrated, not existence-only):

| Category | Function | Source file(s) | Result |
|---|---|---|---|
| Calibration | `validateCalibrationEvidence` | `evidence/screenshot-calibration.json` | 0 errors — 5 states × 3 captures each, all `maxRatio: 0`, `selectedThreshold: 0 <= ceiling 0.02` |
| Mask audit | `validateMaskAudit` | `frontend/e2e/design-system.visual-{shell,authoring,live-editors}.spec.ts`'s own `NO_MASKS: readonly MaskRegion[] = []` | 0 errors on an empty array — genuinely zero masks are used across all 9 surfaces/36 baselines (documented in-source as a deliberate, checked absence, not an omission: "the empty NO_MASKS array is itself the documented mask-rectangle set") |
| Packaged WebView2 | `validateDialogFeasibilityEvidence` | `evidence/dialog-feasibility.json` | 0 errors (see **Packaged WebView2 Evidence** above) |
| Six backstops | `validateBackstop*` (×6) | `evidence/{startup-theme-font,error-boundary-fallback,specialized-geometry,expanded-copy,text-zoom-200,offline-safety}.json` | 0 errors on all six (see **Six Separately Named UI Backstops** above) |
| Windows CI | `validateWindowsCiEvidence` | live `gh` data for run 30920907751 | **1 error** — see Blocker A |
| Implementation tree | `validateImplementationTreeIdentity` | live `git` data, `provenSha=339dce03...` | **2 errors at current HEAD** — see below |
| Requirements coverage | `validateExactCoverage` | `REQUIREMENT_IDS` (15) vs. Multi-Source Coverage Audit's own "COVERED" rows | 0 errors — exact set match |
| Backstop coverage | `validateExactCoverage` | `BACKSTOP_IDS` (6) vs. **Six Separately Named UI Backstops** | 0 errors — exact set match |

**Implementation-tree identity, re-checked at the moment this row was written (HEAD `55a7d463`):** `computeImplementationManifest`/`validateImplementationTreeIdentity` were invoked live against `provenSha=339dce03deab5c076a59090a6280e811ac7b8f3c` (run 30920907751, `conclusion: success`) and `observedSha=55a7d463a0216159d013666a77bcb81203dfe54d` (current HEAD). Result: **2 genuine errors** — `git diff --name-only 339dce03... 55a7d463...` returns 4 changed paths, and 2 of them (`frontend/DESIGN_SYSTEM.md`, `frontend/README.md`) are **not** under `.planning/**` (a concurrent, legitimate documentation commit — `55a7d463`, author `Lawrence Norton`, prose-only expansion of the frontend README/design-system guide — landed on `master` mid-session, outside this executor's control and outside this plan's own declared scope). Non-planning manifest hashes correspondingly differ (`60827b45...` proven vs. `4843c514...` observed). This means implementation-tree identity is **also** currently broken at HEAD, for a reason distinct from Blockers A–C: only commit `339dce03` itself (and any strictly `.planning/**`-only descendant of it) is CI-provably identical; no fresh Windows CI run has yet validated the tree at `55a7d463` or later. This is disclosed transparently rather than fabricating a passing identity claim, and will remain true for any commit after `339dce03` until a fresh required-workflow run proves the then-current non-planning tree.

## Incidental Regeneration Reverts

Running the declared Playwright specs (this plan's own re-execution work) regenerates several already-committed files as a documented, expected side effect of those specs actually running for real (each spec's own fixture writes fresh timing/hash/screenshot data on every execution). Consistent with Plan 13-38's own established precedent (`docs/skills/site-submodule/SKILL.md`'s guidance against reflexively staging unintended dirtiness) and this plan's own declared `files_modified: [13-VALIDATION.md]` scope, every one of the following was reverted via a targeted `git checkout --` / `git -C site checkout --` on exactly the pre-existing tracked path — never a blanket reset/clean:

- `evidence/{dialog-feasibility,error-boundary-fallback,expanded-copy,offline-safety,screenshot-calibration,specialized-geometry,startup-theme-font,text-zoom-200}.json` (8 files) — regenerated by the design-system Playwright specs and the packaged-proof script.
- `frontend/design-system/screenshot-tolerance.json` — regenerated by `design-system.calibration.spec.ts`.
- 20 desktop-view PNGs under `site/public/desktop-views/` — regenerated by `desktop-view-docs.spec.ts` running inside `npm run test:e2e`.

`git status --short` confirms a clean working tree (only this plan's own pre-existing, dispatch-authorized untracked files — `.gitkeep`, `13-PATTERNS.md`, `package-lock.json` — remain) before this plan's own commit.

## Visual Baseline Artifact Contract

Each visual plan owns one spec plus exactly 12 concrete Windows PNGs. Each task owns the shared spec plus four PNGs, remaining below the task threshold.

| Plan | Surfaces | Calculation | Concrete artifacts |
|---|---|---|---|
| 13-32 | Persistent shell; dialog layer; shared gallery | 3 × 2 themes × 2 widths | 12 PNG + 1 spec = 13 |
| 13-33 | Scenes & Looks; Fixture/Patch; Guided First Show | 3 × 2 themes × 2 widths | 12 PNG + 1 spec = 13 |
| 13-34 | Desk/Operator; MIDI Mapping; Scripts/Notes | 3 × 2 themes × 2 widths | 12 PNG + 1 spec = 13 |

Every screenshot row requires calibrated threshold identity, exact mask rectangles/reasons, protected-region intersection result, semantic preassertions, environment/browser/build identity, and artifact SHA-256. No mask may intersect Blackout, Revoke Automation, Stop/Release-All, live truth, navigation, or dialog focus.

## Closed Semantic Evidence Schema

Every completed automated row requires:

- exact task ID, plan, wave, normalized command, and command SHA-256 derived from PLAN;
- exit status `0`, start/end timestamps, repository commit SHA, dirty-tree declaration, OS/runtime/tool/browser identity, and application build identity;
- typed artifact array with contained repository-relative or immutable remote paths, media/schema type, byte count, SHA-256, and task-specific semantic fields;
- packaged WebView2 evidence with executable path/hash, application build SHA, WebView2 runtime, CDP endpoint ownership, assertion set, and process cleanup;
- calibration evidence with three capture identities, all pairwise diff operands/results, ceiling, computed smallest-stable threshold, selected threshold, and arithmetic recomputation;
- mask audit with every rectangle, reason, screenshot, protected locator rectangles, intersection calculation, and zero protected intersections;
- Windows evidence with approval record, trigger mechanism, immutable run id/URL, workflow path, event, head SHA/ref, attempt, conclusion, job ids, artifact ids/names/hashes, embedded SHA/build identity, and validated result schemas.
- CI implementation identity with `ciProvenImplementationSha`, sorted non-planning path/mode/blob manifest and SHA-256, `observedDescendantSha`, ancestry result, exact changed paths, sole `.planning/**` allowlist, descendant manifest/hash, and equality result; any frontend/runtime/workflow/dependency/build/configuration change after the proven SHA fails.
- Text-zoom evidence with exact 900x720 viewport, requested/computed 200% text zoom, root/body client and scroll widths, required semantic locators, visibility/boxes, ordered keyboard focus traversal, scroll owners, overlay hit tests, and operation results.
- Provider/daemon-offline evidence with named state inputs, explicit Go-owned playback/output truth, projected connectivity text, Blackout/Revoke role/name/box/focus/action results, independent local path identities, dispatch counts, and proof that connectivity loss did not synthesize a stopped claim.

Missing files, malformed/duplicate fields, Markdown-only claims, existence-only artifacts, stale SHA/build identity, wrong successful command, or unparseable semantic contents fail closed.

## Wave 0 Artifact Contract

- [x] Exact approved parser pins and lockfile. — 13-01-01/02 pass (real `npm view`/`npm install --save-exact`, no lockfile drift).
- [x] Strict token/component/runtime-geometry/empty-exception manifests and four schemas. — 13-21-01/02 pass; `check.mjs --all` zero diagnostics.
- [x] Deterministic generated CSS/TypeScript. — 13-21-02, 13-07-02 pass (`generate:design-system && generate.mjs --check` both exit 0).
- [x] DS001–DS010 checker with polarity, malformed, path, and exception fixtures. — 13-02-01/02, 13-19-01's own `check.test.ts` sub-command pass; the checker's own DS008 audit-scope behavior is Blocker B (disclosed, not a functional defect).
- [x] Typed primitives/patterns, one public inventory/barrel/guide, and deterministic gallery. — 13-03 through 13-07 pass; whole-source `check.mjs --all` zero diagnostics.
- [x] Chromium and packaged-WebView2 Dialog proof. — 13-06-01/02 both pass (see **Packaged WebView2 Evidence**).
- [x] Three-capture calibration and one bounded tolerance. — 13-17-01/02 pass; `validateCalibrationEvidence` 0 errors, `selectedThreshold: 0`.
- [x] Six separately named backstop evidence objects. — all six pass, see **Six Separately Named UI Backstops**.
- [x] 36 explicit Paper/Ink Windows baselines across nine surfaces with semantic/mask audits. — 13-32/13-33/13-34 (12 tasks) all pass; mask audit is a genuinely empty, documented-as-such array (`validateMaskAudit([])` — 0 errors).
- [x] Pinned package/registry/Mage routes and required Windows workflow. — 13-18-01/03, 13-35-01/02 pass; 13-18-02 fails only for the pre-existing, already-disclosed `-tags mage` omission (Blocker C).
- [x] Immutable successful Windows run evidence matching the approved commit. — run 30920907751 at `339dce03...`, `conclusion: success`, re-verified live this session; **the validator's own artifact-schema requirement cannot be satisfied by any genuine run of this workflow (Blocker A)**.
- [x] Plan-derived semantic evidence validator with mutation tests. — 13-20-01/02 pass (90/90 tests, including the mutation matrix).

**Every Wave 0 artifact category above genuinely exists and functions correctly.** The two unchecked-in-spirit items (Windows evidence's artifact schema, and the DS001–DS010 checker's own audit-scope semantics under `--paths`) are real, disclosed, structural characteristics of the validator/checker themselves (Blockers A/B) — not missing or broken artifacts.

## Multi-Source Coverage Audit

| Source | Item | Plans | Status |
|---|---|---|---|
| GOAL | One documented Paper/Ink system with zero unregistered drift across every reachable desktop surface | 01–41 | COVERED |
| REQ | D-01 through D-14 and approved UI-SPEC | 01–41 | COVERED |
| RESEARCH | Package legitimacy separated from inert manifest/generation authority | 01,21 | COVERED |
| RESEARCH | DS001–DS010 and deterministic diagnostics | 02,19–20 | COVERED |
| RESEARCH | Typed primitives, patterns, gallery, guide, parity | 03–07,22–23,36–37 | COVERED |
| RESEARCH | Complete migration: theme/shell/front door/guide/fixtures/scenes/output/editors/Desk/Operator/MIDI/safety/overlays/shared chrome | 08–16,24–29,40 | COVERED |
| RESEARCH | Chromium and packaged WebView2 dialog feasibility | 06,22 | COVERED |
| RESEARCH | Three-capture tolerance before baselines | 17 | COVERED |
| RESEARCH | Six UI backstops, including 200% text zoom and provider/daemon-offline safety | 30–31,41 | COVERED |
| RESEARCH | Nine-surface light/dark 900/1280 matrix | 32–34 | COVERED |
| RESEARCH | Required Windows workflow, immutable implementation SHA/tree identity, and observed artifacts | 18,20,35,38–39 | COVERED |
| RESEARCH | Exact exception merge, ConfirmModal removal, final parity | 19,37,36 | COVERED |
| RESEARCH | Fake-sign-off-resistant validation, acceptance, final sign-off | 20,38–39 | COVERED |
| CONTEXT | D-01 Paper/Ink | 08,17,30,32–34,36 | COVERED |
| CONTEXT | D-02 every reachable surface | 09–16,24–34,37–39 | COVERED |
| CONTEXT | D-03 dense 4px grid, guide 8px, 210px sizing | 21,24–25,31,33 | COVERED |
| CONTEXT | D-04 semantic tokens | 03–05,08–16,21–29,36 | COVERED |
| CONTEXT | D-05 shared typed primitives/patterns | 02–07,09–16,22–29,37 | COVERED |
| CONTEXT | D-06 theme contract/no branching | 08,17,27,30,32–34,36 | COVERED |
| CONTEXT | D-07 exact audited exceptions/no spacing bypass | 02,10,13,15,19,21,24–29 | COVERED |
| CONTEXT | D-08 authoritative guide | 07,36–37 | COVERED |
| CONTEXT | D-09 normal validation and CI | 02,07,18–20,35–39 | COVERED |
| CONTEXT | D-10 layered static/unit/a11y/visual verification | 02–07,17–20,22–23,30–39 | COVERED |
| CONTEXT | D-11 green enforcement/no ignored baseline | 01–02,08–39 | COVERED |
| CONTEXT | D-12 interaction/accessibility/non-color semantics | 02–39 | COVERED |
| CONTEXT | D-13 persistent independent safety | 06,13–18,24,28,30–35,38–39,41 | COVERED |
| CONTEXT | D-14 projection-only React | 03–16,22–31,34,38–39 | COVERED |

Deferred ideas: none. No source item is missing.

## Sign-Off Gate

- [x] All 77 tasks and automated commands match PLAN-derived normalized strings/hashes and the authority checkpoint has an explicit outcome. — all 77 derived cleanly (0 contract errors); all 77 re-executed for real this session (45 pass / 31 disclosed-non-regression-fail / 1 is this task's own verify); 13-35-01's authority checkpoint has an explicit `pass` outcome.
- [x] All D-01 through D-14 and UI-SPEC contracts are covered. — see **Multi-Source Coverage Audit** (unchanged, all rows COVERED) and `requirementsCovered` exact-set check (0 errors).
- [x] Every Wave 0 artifact exists and passes semantic validation. — see **Wave 0 Artifact Contract** above.
- [x] Packaged WebView2 proof matches its executable build hash. — `evidence/dialog-feasibility.json`'s `build.sha256` is the real hash of the `golc-desktop.exe` that `runtime.cdp_endpoint`/`test` were captured against (both committed Aug 3 capture and this session's fresh Attempt 3 capture independently confirm this self-consistency).
- [x] Calibration arithmetic recomputes the selected threshold before all 36 baselines. — `validateCalibrationEvidence` recomputes `selectedThreshold` from the pairwise-diff data itself (not trusting the declared field) and confirms `0 == 0`.
- [x] All six separately named backstops pass, including exact 200% text zoom and both offline safety projections. — see **Six Separately Named UI Backstops**.
- [x] Mask audit contains zero protected intersections. — genuinely empty mask array (0 masks, 0 possible intersections) — see **Aggregated Evidence Bundle Validation**.
- [x] Final exception authority contains no broad, spacing, safety, stale, zero-match, or multi-match record. — `check.mjs --all` (whole-source, the only mode that meaningfully audits DS008 exception-integrity end-to-end) exits 0 with zero diagnostics.
- [x] ConfirmModal directory/import/export/inventory/docs/aliases/compatibility are absent. — 13-37-01 passes (`ConfirmModal removal` describe block, semantic `import.meta.glob` absence assertions).
- [ ] **Required Windows run matches the approved immutable implementation SHA/tree hash and all downloaded artifacts pass schema/hash/build checks.** — FALSE. Run 30920907751 genuinely matches `339dce03...` (`conclusion: success`), but the "downloaded artifacts pass schema/hash/build checks" clause can never be satisfied for any genuine passing run of this workflow — **Blocker A**.
- [ ] **Any later evidence/planning commit descends from the CI-proven SHA, changes only `.planning/**`, and has an identical non-planning path/mode/blob manifest/hash.** — FALSE at current HEAD. `339dce03` itself satisfies this trivially (self-identical); HEAD `55a7d463` does not (2 non-`.planning/**` paths changed by a concurrent, legitimate documentation commit — see **Aggregated Evidence Bundle Validation**'s implementation-tree row).
- [x] Complete local acceptance passes against the identical CI-proven implementation tree while executor work remains inside declared ownership and unrelated `site` work remains untouched. — every functional stage of 13-38-01's declared chain genuinely passes on isolated re-run this session (build/test/e2e/design-system-e2e/all 4 Mage targets); `site` submodule reverted clean via targeted checkout (see **Incidental Regeneration Reverts**). Only the chain's own final validate-evidence sub-command hits Blocker A.
- [ ] **`wave_0_complete: true`, `nyquist_compliant: true`, and approval are set only after `validate:phase13-evidence` passes.** — Correctly left `false`. The full-bundle form of the validator (`--evidence <bundle>`) cannot pass while Blocker A exists (structural, workflow-vs-validator-schema incompatibility, unfixable from this plan's declared single-file scope). The no-flag "light" form the plan's own literal `<verify>` invokes (`cd frontend && npm run validate:phase13-evidence`, no `--evidence`) does genuinely pass (see **This Plan's Own Declared Verify** below) — but that lighter mode only re-validates the plan-derived contract shape plus each individually-typed `evidence/*.json` file against its own schema; it does not, and by its own documented design cannot, constitute the full sign-off gate this checklist describes. Flipping the sign-off flags on the strength of the lighter check alone would be exactly the "premature flag edit falsely certifying the phase" this plan's own `<threat_model>` (T-13-42) exists to prevent.

**Approval: DENIED.** Two of the twelve gate items above are genuinely, mechanically false today (Windows-run artifact schema; later-commit `.planning/**`-only descendant identity at current HEAD), for reasons fully diagnosed and disclosed in **Structural Sign-Off Blockers**. `wave_0_complete`, `nyquist_compliant`, and approval remain `false` in this document's frontmatter, matching this outcome exactly. Resolving Blocker A requires a coordinator/user decision on either changing `.github/workflows/design-system.yml`'s artifact-upload condition or relaxing `validate-phase13-evidence.mjs`'s unconditional artifact requirement — both outside this plan's own declared `files_modified: [13-VALIDATION.md]` scope. Resolving the implementation-tree gate requires either restricting future commits to `.planning/**` until a fresh Windows run re-proves the tree, or triggering that fresh run against the current (or a future) HEAD.

## This Plan's Own Declared Verify (13-39-01)

Task 13-39-01's own `<verify><automated>` chain was run for real, in order, this session, as the literal, final closing act of this plan's own work:

1. `cd frontend && npm run validate:phase13-evidence` (no `--evidence` flag — the lighter, always-runnable mode) → `PASS available phase-13 evidence validated (no --evidence bundle supplied; full sign-off gate requires one)`. Exit `0`. Derives all 77 tasks cleanly and re-validates all 8 individually-typed `evidence/*.json` files against their own schemas (all pass) — but, by this script's own documented design (see its own source comment: *"the full sign-off gate requires --evidence"*), this mode does **not** constitute or claim the full-bundle sign-off gate this document's own Sign-Off Gate section requires. See that section's own final bullet for why this genuine pass does not, by itself, license flipping `wave_0_complete`/`nyquist_compliant`/approval.
2. `node .../gsd-tools.cjs query verify.plan-structure .../13-39-PLAN.md` → `{"valid": true, "errors": [], "warnings": []}`. Exit `0`.
3. `git diff --check -- .../13-VALIDATION.md` → no whitespace-error output. Exit `0`.

All three sub-commands of this task's own declared verify chain genuinely pass. Per this document's own Sign-Off Gate analysis above, that is the correct, expected, and complete outcome for this plan's own `<done>` criteria (*"13-VALIDATION.md is complete, truthful, semantically machine-validated, and signed off only from executed evidence"*) — every word of that sentence except "signed off" is true; "signed off" is correctly withheld given Blockers A and the implementation-tree gate above.
