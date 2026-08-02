---
phase: 13
slug: unified-ui-design-system-and-automated-enforcement
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-08-02
revised: 2026-08-02
---

# Phase 13 — Validation Strategy

> Exact execution contract for the current 20-plan, 47-task graph. Pending rows are intentionally not evidence of success.

## Test Infrastructure

| Property | Value |
|---|---|
| Framework | Vitest 4.1.10 + jsdom + Testing Library; Playwright 1.62.0 Chromium and packaged WebView2 CDP |
| Config | `frontend/vite.config.ts`; `frontend/playwright.config.ts` |
| Quick loop | `cd frontend && npm run check:design-system && npm run test:design-system` |
| Final loop | `cd frontend && npm run build && npm test && npm run test:e2e && npm run test:e2e:design-system` |
| Evidence validator | `cd frontend && npm run validate:phase13-evidence` |
| Task latency | Focused static/Vitest commands target under 30 seconds; browser and full suites occur only at proof/wave/final gates |

## Sampling Contract

- After each implementation task: run its focused Vitest/static command exactly as listed in the owning plan.
- After Wave 6 migration: run Plan 13-17 calibration and browser matrix, not the full browser suite after each migration task.
- Before sign-off: run every command in Plan 13-20 Task 2 and validate this document mechanically.
- Every command chain uses fail-fast `&&`; PowerShell-native control flow explicitly checks `$LASTEXITCODE`.

## Exact Per-Task Verification Map

| Task ID | Plan/Wave | Decision or contract | Automated evidence command | Required artifact/evidence | Status |
|---|---|---|---|---|---|
| 13-01-01 | 01/W1 | package legitimacy checkpoint | `npm view` for both exact pins + pre-install diff | registry JSON and human approval | pending |
| 13-01-02 | 01/W1 | D-01,D-03,D-04,D-06,D-07,D-11 | manifest Vitest | manifests, schemas, zero initial exceptions | pending |
| 13-01-03 | 01/W1 | D-04,D-06 | generator + check mode | generated CSS/TS freshness | pending |
| 13-02-01 | 02/W2 | D-05,D-07,D-09,D-10,D-12,D-13 | focused DS001-DS010 fixtures | CSS/TSX policies and polarity fixtures | pending |
| 13-02-02 | 02/W2 | D-07,D-09,D-11 | complete checker Vitest | deterministic scoped/whole-source orchestration | pending |
| 13-03-01 | 03/W2 | D-04,D-05,D-06,D-12 | Button/IconButton Vitest | action primitives | pending |
| 13-03-02 | 03/W2 | D-04,D-05,D-06,D-12 | Field/Chip Vitest | field/status primitives | pending |
| 13-04-01 | 04/W2 | D-05,D-12 | Tabs Vitest | Tabs contract | pending |
| 13-04-02 | 04/W2 | D-05,D-12,D-14,UI Considerations | shared-state Vitest | Empty/Loading/Error states | pending |
| 13-05-01 | 05/W3 | D-05,D-12 | structural primitive Vitest | Panel/Header/Toolbar/ListRow | pending |
| 13-05-02 | 05/W3 | D-03,D-05,D-12 | utility primitive Vitest | Scroll/Tooltip/Resize | pending |
| 13-05-03 | 05/W3 | D-10,D-12,D-13,D-14 | Dialog/ConfirmDialog Vitest | stable dialog API | pending |
| 13-06-01 | 06/W4 | D-10,D-12,D-13 | focused real Chromium proof | dialog feasibility spec | pending |
| 13-06-02 | 06/W4 | D-02,D-10,D-12,D-13,D-14,WebView2 | packaged proof PowerShell harness | `evidence/dialog-feasibility.json` | pending |
| 13-07-01 | 07/W5 | D-05,D-10,D-12,D-13,D-14,UI Considerations | pattern/gallery Vitest | patterns and deterministic gallery | pending |
| 13-07-02 | 07/W5 | D-08,D-09 | generate + design-system Vitest + DS007 | inventory/barrel/guide parity | pending |
| 13-08-01 | 08/W6 | D-01,D-04,D-06 | theme Vitest + scoped checker | generated theme consumption | pending |
| 13-08-02 | 08/W6 | D-02,D-03,D-05,D-07,D-11,D-13,D-14 | shell Vitest + scoped checker | core shell and exact proposal | pending |
| 13-09-01 | 09/W6 | D-02,D-05,D-11,D-12,D-14 | four workspace tests + scoped checker | front-door workspaces | pending |
| 13-09-02 | 09/W6 | D-02,D-03,D-04,D-07,D-11,D-14 | guide tests + scoped checker | five stages, 8px spacing, no spacing exception | pending |
| 13-10-01 | 10/W6 | D-02,D-05,D-11,D-12,D-14 | Fixture Library test + checker | browse/import migration | pending |
| 13-10-02 | 10/W6 | D-02,D-05,D-07,D-11,D-12,D-14 | fixture components tests + checker | preview-safe patch/project fixture migration | pending |
| 13-11-01 | 11/W6 | D-02,D-03,D-04,D-05,D-07,D-11,D-14 | Scene tests + checker | SceneStack migration | pending |
| 13-11-02 | 11/W6 | D-02,D-04,D-05,D-07,D-11,D-14 | output tests + checker | Art-Net/diagnostics migration | pending |
| 13-12-01 | 12/W6 | D-02,D-04,D-05,D-07,D-11,D-12,D-14 | Notes tests + checker | Notes/Tiptap migration | pending |
| 13-12-02 | 12/W6 | D-02,D-04,D-05,D-06,D-07,D-11,D-12,D-14 | Scripts tests + checker | Scripts/Monaco/dialog migration | pending |
| 13-13-01 | 13/W6 | D-02,D-03,D-04,D-05,D-12,D-13,D-14 | Desk tests | Desk migration | pending |
| 13-13-02 | 13/W6 | D-07,D-11,D-14 | scoped checker + `internal/deskmidi` diff | exact Desk geometry proposal; user work untouched | pending |
| 13-14-01 | 14/W6 | D-02,D-04,D-05,D-11,D-12,D-14 | Operator tests + checker | Operator Surface migration | pending |
| 13-14-02 | 14/W6 | D-02,D-03,D-04,D-05,D-07,D-11,D-12,D-14 | MIDI tests + checker + user-tree diff | MIDI/pickup migration; no support claim | pending |
| 13-15-01 | 15/W6 | D-02,D-04,D-05,D-07,D-11,D-12,D-13,D-14 | SafetyCluster tests + checker | independent safety migration; no safety exception | pending |
| 13-15-02 | 15/W6 | D-02,D-03,D-04,D-05,D-11,D-12,D-14 | live/tempo tests + checker | projection-only live truth and tempo | pending |
| 13-16-01 | 16/W6 | D-02,D-03,D-04,D-05,D-11,D-12,D-13,D-14 | inspector/help tests + checker | inspector and help migration | pending |
| 13-16-02 | 16/W6 | D-02,D-04,D-05,D-11,D-12,D-14 | quick/error tests + checker | quick switch, error, and log migration | pending |
| 13-16-03 | 16/W6 | D-02,D-03,D-04,D-05,D-07,D-11,D-12,D-14 | hotkey/workspace tests + checker | shared workspace migration | pending |
| 13-17-01 | 17/W7 | D-01,D-06,D-10 | calibration spec list | deterministic fixtures/helpers | pending |
| 13-17-02 | 17/W7 | D-01,D-06,D-10 | three-capture calibration spec | tolerance JSON + calibration evidence | pending |
| 13-17-03 | 17/W7 | D-02,D-03,D-10,D-12,D-13,D-14,visual matrix/backstops | focused visual/a11y specs | canonical snapshots; zero safety masks | pending |
| 13-18-01 | 18/W8 | D-09,D-10,D-11,D-14 | focused internal command tests | package and pinned command routes | pending |
| 13-18-02 | 18/W8 | D-09,D-10,D-11 | delivery/Mage tests + Mage list | registry/Mage/command graph | pending |
| 13-18-03 | 18/W8 | D-10,D-13 | Mage offline + workflow diff check | required Windows workflow and failure artifacts | pending |
| 13-19-01 | 19/W9 | D-07,D-09,D-11 | checker tests + whole-source checker | final exceptions and proposal removal | pending |
| 13-19-02 | 19/W9 | D-01,D-04,D-06,D-08,D-09,D-12 | design-system tests + DS007 | public/theme parity and contrast | pending |
| 13-19-03 | 19/W9 | D-02,D-05,D-08,D-11 | focused contract absence test + DS007 | ConfirmModal directory/import/export/inventory/compatibility absence | pending |
| 13-20-01 | 20/W10 | D-09,D-10,D-11,Nyquist contract | validator mutation Vitest | strict evidence validator | pending |
| 13-20-02 | 20/W10 | D-01..D-14,full UI-SPEC | build + all tests/E2E/Mage + user-tree diffs | complete command results | pending |
| 13-20-03 | 20/W10 | D-01..D-14,Nyquist sign-off | evidence validator + plan structure + diff check | truthful completed validation document | pending |

## Wave 0 Artifact Contract

All rows remain pending until execution creates and validates them.

- [ ] Strict token, component, runtime-geometry, and initially empty exception manifests plus schemas.
- [ ] Generated CSS/TypeScript with deterministic freshness proof.
- [ ] DS001-DS010 checker with allowed/forbidden/malformed/path/exception fixtures.
- [ ] Public inventory/barrel/guide parity and deterministic gallery.
- [ ] Dialog real-Chromium proof and packaged-WebView2 proof evidence.
- [ ] Three-capture screenshot calibration evidence and one bounded global tolerance.
- [ ] Visual, a11y, geometry, focus, zoom, motion, theme, and safety matrix with zero safety masks.
- [ ] Pinned package/registry/Mage routes and required Windows CI artifact upload.
- [ ] Validation-contract parser with mutation tests.

## Evidence Fields Required Per Completed Row

The validator must require:

- exact task ID and owning plan/wave;
- decision/UI-SPEC identifiers;
- executed command string and exit status;
- artifact or immutable CI evidence location;
- execution environment when browser/WebView2/Windows-sensitive;
- safety-mask audit result for visual rows;
- measured capture values and selected threshold for calibration;
- no unsupported `green`, `complete`, approval, `wave_0_complete: true`, or `nyquist_compliant: true`.

## Multi-Source Coverage Audit

| Source | ID | Feature / requirement | Plans | Status |
|---|---|---|---|---|
| GOAL | — | Every reachable desktop surface uses one documented Paper/Ink system with zero unregistered drift | 01-20 | COVERED |
| REQ | D-01..D-14 + approved UI-SPEC | Full phase requirement assignment | 01-20 | COVERED |
| RESEARCH | manifests/generator/checker | Strict inert authority, DS001-DS010, deterministic diagnostics | 01-02,18-20 | COVERED |
| RESEARCH | dialog runtime uncertainty | Early Chromium plus packaged WebView2 proof or proven private alternative | 05-06 | COVERED |
| RESEARCH | screenshot stability | Three repeated captures and evidence-driven global tolerance before baselines | 17 | COVERED |
| RESEARCH | exception uncertainty | Zero initial entries; exact per-slice proposals; one evidence-driven final merge | 01-02,08-16,19 | COVERED |
| RESEARCH | primitive/pattern inventory | Typed primitives, product patterns, gallery, guide, parity | 03-07 | COVERED |
| RESEARCH | complete migration | Shell, front door, fixtures, scenes/output, editors, Desk, Operator/MIDI, safety | 08-16 | COVERED |
| RESEARCH | Windows CI | Pinned registry-backed browser gate and diff artifacts | 17-18 | COVERED |
| CONTEXT | D-01 | Preserve Paper/Ink | 01,08,17,19-20 | COVERED |
| CONTEXT | D-02 | Migrate every reachable surface | 06,08-17,19-20 | COVERED |
| CONTEXT | D-03 | Dense 4px grid; guide 8px; 210px sizing | 01,08-09,11,13-17,19-20 | COVERED |
| CONTEXT | D-04 | Semantic token consumption | 01,03-04,08-16,19-20 | COVERED |
| CONTEXT | D-05 | Shared behavior in typed primitives/patterns | 02-05,07-16,19-20 | COVERED |
| CONTEXT | D-06 | Theme parity; no theme branching | 01,03-04,08,12,17,19-20 | COVERED |
| CONTEXT | D-07 | Exact audited exceptions; no spacing bypass | 01-02,08-16,19-20 | COVERED |
| CONTEXT | D-08 | Concise authoritative guide | 07,19-20 | COVERED |
| CONTEXT | D-09 | Normal validation/CI enforcement | 02,07,18-20 | COVERED |
| CONTEXT | D-10 | Layered static/unit/a11y/visual verification | 02-07,17-20 | COVERED |
| CONTEXT | D-11 | Enforcement begins green; no ignored baseline | 01-02,08-20 | COVERED |
| CONTEXT | D-12 | Complete interaction states and native semantics | 02-17,19-20 | COVERED |
| CONTEXT | D-13 | Preserve independent persistent safety | 02,05-08,13-18,20 | COVERED |
| CONTEXT | D-14 | React remains projection-only | 03-16,18-20 | COVERED |

Deferred ideas: none. No source item is missing.

## Validation Sign-Off

- [ ] All 47 actual tasks have command and evidence records.
- [ ] All D-01 through D-14 and approved UI-SPEC contracts are covered.
- [ ] Every Wave 0 artifact exists and passes its mapped gate.
- [ ] Packaged WebView2 dialog proof passes in-phase.
- [ ] Screenshot threshold is derived from three bounded captures before baseline acceptance.
- [ ] Final exception count is evidence-driven and contains no broad, spacing, or safety record.
- [ ] ConfirmModal directory, imports, exports, inventory, docs, aliases, and compatibility layer are absent.
- [ ] No visual mask intersects safety, navigation, live truth, or dialog focus.
- [ ] Full acceptance and required Windows CI evidence pass.
- [ ] `wave_0_complete: true`, `nyquist_compliant: true`, and approval are set only after `validate:phase13-evidence` passes.

**Approval:** pending
