---
phase: 13
slug: unified-ui-design-system-and-automated-enforcement
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-08-02
---

# Phase 13 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Vitest 4.1.10 + jsdom + Testing Library; Playwright 1.62.0 Chromium |
| **Config file** | `frontend/vite.config.ts`; `frontend/playwright.config.ts` |
| **Quick run command** | `cd frontend && npm run check:design-system && npm run test:design-system` |
| **Full suite command** | `cd frontend && npm run build && npm run test:e2e:design-system` |
| **Estimated runtime** | Quick <30 seconds; browser suite is a separate slower gate |

---

## Sampling Rate

- **After every task commit:** Run the design-system checker, design-system Vitest suite, and touched workspace tests.
- **After every plan wave:** Run `npm run build` plus the targeted 900×720 and 1280×720 Playwright states for that wave.
- **Before `/gsd-verify-work`:** Full frontend, current Playwright, design-system visual/a11y, and root Mage/CI gates must be green.
- **Max feedback latency:** 30 seconds for the quick loop; browser work does not block per-task feedback.

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 13-01-01 | 01 | 1 | D-04, D-06 | T-13-01 | Manifest is inert, contained data | token contract | `npm run test:design-system -- tokens` | ❌ W0 | ⬜ pending |
| 13-01-02 | 01 | 1 | D-07, D-09, D-11 | T-13-01..04 | Checker fails closed with exact diagnostics | checker fixtures | `npm run check:design-system` | ❌ W0 | ⬜ pending |
| 13-02-01 | 02 | 2 | D-05, D-08, D-12 | — | Primitives own semantics and states | component/a11y | `npm run test:design-system -- primitives` | ❌ W0; seeds exist | ⬜ pending |
| 13-03-01 | 03 | 3 | D-01..D-06 | — | Shell/shared workspaces use canonical surface | regression | `npm run build` | Partial | ⬜ pending |
| 13-04-01 | 04 | 4 | D-02, D-12, D-13, D-14 | T-13-05 | Safety remains visible and independent | browser integration | `npm run test:e2e:design-system -- --grep safety` | Partial | ⬜ pending |
| 13-05-01 | 05 | 5 | D-02, D-11 | — | No unregistered legacy violations remain | static integration | `npm run check:design-system` | ❌ W0 | ⬜ pending |
| 13-06-01 | 06 | 6 | D-10, UI-SPEC | T-13-06 | Visual masks cannot hide safety regressions | visual/a11y | `npm run test:e2e:design-system` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] Token, component, runtime-geometry, and exception manifests with strict schemas.
- [ ] DS001–DS010 checker plus allowed and forbidden fixtures for every rule.
- [ ] Generated token CSS/TypeScript exports and freshness test.
- [ ] Public inventory/barrel/guide drift test.
- [ ] Primitive state gallery with deterministic fixtures.
- [ ] Dialog focus-management browser proof.
- [ ] `design-system.visual.spec.ts`, `design-system.a11y.spec.ts`, capture stylesheet, and Windows baselines.
- [ ] Dedicated Windows Mage/CI browser target with diff artifact upload.
- [ ] Exact parser/tooling pins after package-legitimacy verification.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Native dialog behavior in supported WebView2 | D-12 | Browser emulation may differ from packaged WebView2 | Exercise open, safe initial focus, Escape, backdrop, and return-focus behavior in the packaged app |
| Baseline screenshot tolerance | D-10 | Final threshold depends on Windows CI font/raster stability | Review first three CI runs and choose the smallest stable threshold without masking semantic drift |

---

## Validation Sign-Off

- [ ] All tasks have automated verification or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verification
- [ ] Wave 0 covers all missing references
- [ ] No watch-mode flags
- [ ] Quick feedback latency <30s
- [ ] `nyquist_compliant: true` set after execution evidence exists

**Approval:** pending
