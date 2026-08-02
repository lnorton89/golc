---
phase: 13
slug: unified-ui-design-system-and-automated-enforcement
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-08-02
revised: 2026-08-02
plan_count: 40
task_count: 70
---

# Phase 13 — Validation Strategy

> Exact execution contract for the revised 40-plan, 70-task graph. Commands below are normalized from each PLAN task and are pending until semantic evidence validates. The external-mutation authority row runs read-only preflight and remains a blocking checkpoint.

## Test Infrastructure

| Property | Value |
|---|---|
| Framework | Vitest 4.1.10 + jsdom + Testing Library; Playwright Chromium and packaged WebView2 CDP |
| Config | `frontend/vite.config.ts`; `frontend/playwright.config.ts` |
| Quick loop | `cd frontend && npm run check:design-system && npm run test:design-system` |
| Full local loop | Plan 13-38 Task 1 exact command |
| Evidence validator | `cd frontend && npm run validate:phase13-evidence` |
| Windows evidence | Immutable GitHub Actions run id/URL + exact head SHA + downloaded artifact ids/hashes/schemas |

## Plan-Derived Command Contract

The validator parses every `13-NN-PLAN.md`, derives task position, decodes XML entities, normalizes CRLF to LF and surrounding whitespace once, and requires exact command equality plus SHA-256. Shell-equivalent substitutions are not accepted.

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
- `13-05-01` W4 — `cd frontend && npx vitest run src/components/primitives/Panel src/components/primitives/PanelHeader src/components/primitives/Toolbar src/components/primitives/ListRow`
- `13-22-01` W4 — `cd frontend && npx vitest run src/components/primitives/Dialog src/components/primitives/ConfirmDialog`
- `13-23-01` W4 — `cd frontend && npx vitest run src/components/primitives/ScrollRegion src/components/primitives/InfoTooltip`
- `13-23-02` W4 — `cd frontend && npx vitest run src/components/primitives/ResizeHandle`
- `13-06-01` W5 — `cd frontend && npx playwright test e2e/dialog-feasibility.spec.ts --project=chromium --workers=1`
- `13-06-02` W5 — `pwsh -NoProfile -File scripts/ci/run-packaged-dialog-proof.ps1`
- `13-07-01` W6 — `cd frontend && npx vitest run src/design-system/fixtures src/design-system/patterns`
- `13-07-02` W6 — `cd frontend && npm run generate:design-system && npx vitest run src/design-system && node scripts/design-system/check.mjs --rule DS007`

### Wave 7 migrations

- `13-08-01` W7 — `cd frontend && npx vitest run src/lib/theme.test.ts src/App.smoke.test.tsx && node scripts/design-system/check.mjs --paths src/index.css,src/lib/theme.ts,src/App.tsx`
- `13-09-01` W7 — `cd frontend && npx vitest run src/workspaces/show/OverviewWorkspace.test.tsx src/workspaces/show/ShowsWorkspace.test.tsx src/workspaces/show/SaveRecoveryWorkspace.test.tsx src/workspaces/show/SettingsWorkspace.test.tsx && node scripts/design-system/check.mjs --paths src/workspaces/show/OverviewWorkspace.tsx,src/workspaces/show/OverviewWorkspace.module.css,src/workspaces/show/ShowsWorkspace.tsx,src/workspaces/show/ShowsWorkspace.module.css,src/workspaces/show/SaveRecoveryWorkspace.tsx,src/workspaces/show/SaveRecoveryWorkspace.module.css,src/workspaces/show/SettingsWorkspace.tsx,src/workspaces/show/SettingsWorkspace.module.css`
- `13-10-01` W7 — `cd frontend && npx vitest run src/workspaces/build/FixtureLibraryWorkspace.test.tsx && node scripts/design-system/check.mjs --paths src/workspaces/build/FixtureLibraryWorkspace.tsx,src/workspaces/build/FixtureLibraryWorkspace.module.css`
- `13-10-02` W7 — `cd frontend && npx vitest run src/components/FixturePatch src/components/ProjectFixtures && node scripts/design-system/check.mjs --paths src/workspaces/build/PatchPoolsWorkspace.tsx,src/workspaces/build/ProjectFixturesWorkspace.tsx,src/components/FixturePatch,src/components/ProjectFixtures --proposal design-system/exception-proposals/fixtures.json`
- `13-11-01` W7 — `cd frontend && npx vitest run src/workspaces/build/ScenesLooksWorkspace.test.tsx src/components/SceneProgramming/SceneList.test.tsx src/components/SceneProgramming/LookBrowser.test.tsx && node scripts/design-system/check.mjs --paths src/workspaces/build/ScenesLooksWorkspace.tsx,src/workspaces/build/ScenesLooksWorkspace.module.css,src/components/SceneProgramming/SceneList.tsx,src/components/SceneProgramming/SceneList.module.css,src/components/SceneProgramming/LookBrowser.tsx,src/components/SceneProgramming/LookBrowser.module.css`
- `13-40-01` W7 — `cd frontend && npx vitest run src/components/SceneProgramming/BarTimelinePanel.test.tsx src/components/SceneProgramming/LayerRow.test.tsx && node scripts/design-system/check.mjs --paths src/components/SceneProgramming/BarTimelinePanel.tsx,src/components/SceneProgramming/BarTimelinePanel.module.css,src/components/SceneProgramming/LayerRow.tsx,src/components/SceneProgramming/LayerRow.module.css`
- `13-12-01` W7 — `cd frontend && npx vitest run src/workspaces/show/NotesWorkspace.test.tsx src/components/Notes && node scripts/design-system/check.mjs --paths src/workspaces/show/NotesWorkspace.tsx,src/workspaces/show/NotesWorkspace.module.css,src/components/Notes`
- `13-13-01` W7 — `cd frontend && npx vitest run src/components/Desk src/workspaces/perform/DeskWorkspace.test.tsx`
- `13-13-02` W7 — `cd frontend && node scripts/design-system/check.mjs --paths src/workspaces/perform/DeskWorkspace.tsx,src/workspaces/perform/DeskWorkspace.module.css,src/components/Desk --proposal design-system/exception-proposals/desk.json && cd .. && git diff --exit-code -- internal/deskmidi`
- `13-14-01` W7 — `cd frontend && npx vitest run src/components/OperatorSurface/OperatorSurface.activeSurface.test.tsx src/workspaces/operate/OperatorSurfaceWorkspace.test.tsx && node scripts/design-system/check.mjs --paths src/workspaces/operate/OperatorSurfaceWorkspace.tsx,src/workspaces/operate/OperatorSurfaceWorkspace.module.css,src/components/OperatorSurface/OperatorSurface.tsx,src/components/OperatorSurface/OperatorSurface.module.css,src/components/OperatorSurface/AssignmentToggle.tsx,src/components/OperatorSurface/SurfaceList.tsx`
- `13-14-02` W7 — `cd frontend && npx vitest run src/components/OperatorSurface/Launcher.test.tsx src/components/OperatorSurface/ScenePad.test.tsx && node scripts/design-system/check.mjs --paths src/components/OperatorSurface/Launcher.tsx,src/components/OperatorSurface/Launcher.module.css,src/components/OperatorSurface/ScenePad.tsx,src/components/OperatorSurface/ScenePad.module.css`
- `13-15-01` W7 — `cd frontend && npx vitest run src/components/SafetyCluster && node scripts/design-system/check.mjs --paths src/components/SafetyCluster --proposal design-system/exception-proposals/safety-live.json`
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
- `13-28-01` W7 — `cd frontend && npx vitest run src/components/MidiPanel src/components/MidiLearnToggle src/workspaces/operate/MidiMappingWorkspace.test.tsx && node scripts/design-system/check.mjs --paths src/workspaces/operate/MidiMappingWorkspace.tsx,src/workspaces/operate/MidiMappingWorkspace.module.css,src/components/MidiPanel,src/components/MidiLearnToggle --proposal design-system/exception-proposals/operator-midi.json && cd .. && git diff --exit-code -- internal/deskmidi`
- `13-29-01` W7 — `cd frontend && npx vitest run src/components/HotkeySettings src/components/KeyboardShortcuts src/workspaces/ComingSoon.test.tsx && node scripts/design-system/check.mjs --paths src/components/HotkeySettings,src/components/KeyboardShortcuts,src/workspaces/workspace.module.css,src/workspaces/ComingSoon.tsx,src/workspaces/ComingSoon.module.css --proposal design-system/exception-proposals/shell-overlays.json`

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
- `13-34-01` W9 — `cd frontend && npx playwright test e2e/design-system.visual-live-editors.spec.ts --grep "Desk / Operator Surface" --project=chromium --workers=1 && cd .. && git diff --exit-code -- internal/deskmidi`
- `13-34-02` W9 — `cd frontend && npx playwright test e2e/design-system.visual-live-editors.spec.ts --grep "MIDI Mapping" --project=chromium --workers=1`
- `13-34-03` W9 — `cd frontend && npx playwright test e2e/design-system.visual-live-editors.spec.ts --grep "Scripts / Notes" --project=chromium --workers=1`
- `13-18-01` W10 — `go test ./internal/command -run DesignSystem -count=1`
- `13-18-02` W10 — `go test ./internal/delivery ./magefiles -run 'DesignSystem|MageTarget' -count=1 && mage -l`
- `13-18-03` W10 — `mage CheckOffline && git diff --check -- .github/workflows/design-system.yml`
- `13-35-01` W12 — `git rev-parse HEAD && gh auth status && gh workflow view design-system.yml` — read-only preflight; the blocking checkpoint separately records approved trigger mechanism, exact SHA/ref/repository, or denial.
- `13-35-02` W12 — `gh run view $env:GOLC_PHASE13_RUN_ID --json databaseId,url,workflowName,event,headSha,headBranch,status,conclusion,createdAt,updatedAt,jobs && gh api repos/{owner}/{repo}/actions/runs/$env:GOLC_PHASE13_RUN_ID/artifacts && cd frontend && npm run validate:phase13-evidence -- --evidence ../.planning/phases/13-unified-ui-design-system-and-automated-enforcement/evidence/windows-ci-run.json`
- `13-19-01` W12 — `cd frontend && npx vitest run scripts/design-system/check.test.ts && node scripts/design-system/check.mjs`
- `13-37-01` W13 — `cd frontend && npx vitest run src/design-system/design-system.contract.test.ts --testNamePattern="ConfirmModal removal" && node scripts/design-system/check.mjs --rule DS007`
- `13-36-01` W14 — `cd frontend && npx vitest run src/design-system && node scripts/design-system/check.mjs --rule DS007`
- `13-20-01` W11 — `cd frontend && npx vitest run scripts/design-system/validate-phase13-evidence.test.ts`
- `13-20-02` W11 — `cd frontend && npx vitest run scripts/design-system/validate-phase13-evidence.test.ts --testNamePattern="mutation|false sign-off|semantic"`
- `13-38-01` W16 — `cd frontend && npm run build && npm test && npm run test:e2e && npm run test:e2e:design-system && cd .. && mage GenerateCheck && mage CheckOffline && mage Build && mage TestQuick && git diff --exit-code -- internal/deskmidi && git diff --cached --quiet -- site && cd frontend && npm run validate:phase13-evidence -- --evidence ../.planning/phases/13-unified-ui-design-system-and-automated-enforcement/evidence/phase-acceptance.json`
- `13-39-01` W17 — `cd frontend && npm run validate:phase13-evidence && cd .. && node C:/Users/Lawrence/.codex/gsd-core/bin/gsd-tools.cjs query verify.plan-structure .planning/phases/13-unified-ui-design-system-and-automated-enforcement/13-39-PLAN.md && git diff --check -- .planning/phases/13-unified-ui-design-system-and-automated-enforcement/13-VALIDATION.md`

## Four Separately Named UI Backstops

| Backstop ID | Task | Executable assertion | Semantic evidence schema | Status |
|---|---|---|---|---|
| `startup-theme-font-before-settle` | 13-30-01 | Instrument before navigation; sample theme/font/contrast/safety before settle; await fonts only after pre-settle window | `evidence/startup-theme-font.json`: sample timeline, theme sequence, font sequence, computed colors/contrast, boxes, build/browser identity, assertions | pending |
| `error-boundary-before-theme-css` | 13-30-02 | Block generated token CSS, force ErrorBoundary, assert token-independent readable recovery at 900/1280 | `evidence/error-boundary-fallback.json`: blocked asset, computed colors/contrast, role/name/focus/recovery, boxes, SHA/runtime | pending |
| `specialized-geometry-900-1280` | 13-31-01 | Exact fader/timeline/meter/Monaco/Tiptap/min-max resize rectangles, owners, targets, visibility at both widths | `evidence/specialized-geometry.json`: family, viewport, resize state, measured/expected rectangles, overflow owner, safety/navigation result | pending |
| `expanded-copy-2x-reflow` | 13-31-02 | Prove grapheme ratio ≥2.0 and reflow without clipping/overlap/body overflow/focus or safety loss | `evidence/expanded-copy.json`: canonical/expanded counts and ratio, line/box/overflow/focus/safety results by theme/width | pending |

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

Missing files, malformed/duplicate fields, Markdown-only claims, existence-only artifacts, stale SHA/build identity, wrong successful command, or unparseable semantic contents fail closed.

## Wave 0 Artifact Contract

- [ ] Exact approved parser pins and lockfile.
- [ ] Strict token/component/runtime-geometry/empty-exception manifests and four schemas.
- [ ] Deterministic generated CSS/TypeScript.
- [ ] DS001–DS010 checker with polarity, malformed, path, and exception fixtures.
- [ ] Typed primitives/patterns, one public inventory/barrel/guide, and deterministic gallery.
- [ ] Chromium and packaged-WebView2 Dialog proof.
- [ ] Three-capture calibration and one bounded tolerance.
- [ ] Four separately named backstop evidence objects.
- [ ] 36 explicit Paper/Ink Windows baselines across nine surfaces with semantic/mask audits.
- [ ] Pinned package/registry/Mage routes and required Windows workflow.
- [ ] Immutable successful Windows run evidence matching the approved commit.
- [ ] Plan-derived semantic evidence validator with mutation tests.

## Multi-Source Coverage Audit

| Source | Item | Plans | Status |
|---|---|---|---|
| GOAL | One documented Paper/Ink system with zero unregistered drift across every reachable desktop surface | 01–40 | COVERED |
| REQ | D-01 through D-14 and approved UI-SPEC | 01–40 | COVERED |
| RESEARCH | Package legitimacy separated from inert manifest/generation authority | 01,21 | COVERED |
| RESEARCH | DS001–DS010 and deterministic diagnostics | 02,19–20 | COVERED |
| RESEARCH | Typed primitives, patterns, gallery, guide, parity | 03–07,22–23,36–37 | COVERED |
| RESEARCH | Complete migration: theme/shell/front door/guide/fixtures/scenes/output/editors/Desk/Operator/MIDI/safety/overlays/shared chrome | 08–16,24–29,40 | COVERED |
| RESEARCH | Chromium and packaged WebView2 dialog feasibility | 06,22 | COVERED |
| RESEARCH | Three-capture tolerance before baselines | 17 | COVERED |
| RESEARCH | Four UI backstops | 30–31 | COVERED |
| RESEARCH | Nine-surface light/dark 900/1280 matrix | 32–34 | COVERED |
| RESEARCH | Required Windows workflow and immutable observed artifacts | 18,35 | COVERED |
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
| CONTEXT | D-13 persistent independent safety | 06,13–18,24,28,30–35,38–39 | COVERED |
| CONTEXT | D-14 projection-only React | 03–16,22–31,34,38–39 | COVERED |

Deferred ideas: none. No source item is missing.

## Sign-Off Gate

- [ ] All 70 tasks and automated commands match PLAN-derived normalized strings/hashes and the authority checkpoint has an explicit outcome.
- [ ] All D-01 through D-14 and UI-SPEC contracts are covered.
- [ ] Every Wave 0 artifact exists and passes semantic validation.
- [ ] Packaged WebView2 proof matches its executable build hash.
- [ ] Calibration arithmetic recomputes the selected threshold before all 36 baselines.
- [ ] All four separately named backstops pass.
- [ ] Mask audit contains zero protected intersections.
- [ ] Final exception authority contains no broad, spacing, safety, stale, zero-match, or multi-match record.
- [ ] ConfirmModal directory/import/export/inventory/docs/aliases/compatibility are absent.
- [ ] Required Windows run matches the approved SHA and all downloaded artifacts pass schema/hash/build checks.
- [ ] Complete local acceptance passes at the same SHA while `internal/deskmidi/` and unrelated `site` work remain untouched.
- [ ] `wave_0_complete: true`, `nyquist_compliant: true`, and approval are set only after `validate:phase13-evidence` passes.

**Approval:** pending
