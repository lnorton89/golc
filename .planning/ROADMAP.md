# Roadmap: GOLC

## Overview

GOLC grows through dependency-ordered MVP slices. v1.0 (Phases 1-8, shipped
2026-07-27) proved the deterministic core: offline-safe configuration and delivery
traceability, modular fixture authoring, deterministic show programming and
playback, real Art-Net output, recoverable shows, the full Wails operator
workflow, the public API, and an isolated TypeScript runtime — all without a
desktop adapter ever owning playback or Art-Net timing. The next milestone
(Phases 9-11) adds bounded AI autonomy, then qualifies a self-contained Windows
release, then a telemetry/crash pipeline. Across every phase, UI, persistence,
scripts, API, LLM providers, and Linear remain outside the deterministic playback
and Art-Net timing path.

Linear traceability, established in Phase 1, remains a delivery gate for every
later phase. Repository planning artifacts and runtime operation remain complete
offline; remote mappings are reconciled only through credential-external tooling,
and no pending remote identifier is treated as evidence of synchronization.

## Milestones

- ✅ **v1.0 Core Lighting Console** — Phases 1-8 (shipped 2026-07-27)
- 🚧 **Next milestone (unnamed)** — Phases 9-11 (planned; hygiene and front-door-UI
  phases from `.planning/POST-PHASE-8-PLAN.md` to be inserted before Phase 9 starts)

## Phases

<details>
<summary>✅ v1.0 Core Lighting Console (Phases 1-8) — SHIPPED 2026-07-27</summary>

- [x] Phase 1: Offline Foundation and Delivery Traceability (32/32 plans) — completed 2026-07-21
- [x] Phase 2: Modular Fixtures and Deployments (6/6 plans) — completed 2026-07-21
- [x] Phase 3: Deterministic Show Programming and Playback (7/7 plans) — completed 2026-07-21
- [x] Phase 4: Observable Art-Net Live Output (9/9 plans) — completed 2026-07-22
- [x] Phase 5: Durable Shows and Recovery (5/5 plans) — completed 2026-07-23
- [x] Phase 6: Wails Authoring and Operator Surface (12/12 plans) — completed 2026-07-24
- [x] Phase 7: Versioned External Control API (15/15 plans) — completed 2026-07-25
- [x] Phase 8: Isolated TypeScript Automation (13/13 plans) — completed 2026-07-27

Full phase-by-phase detail (goals, success criteria, plan waves, requirements) is
archived at `.planning/milestones/v1.0-ROADMAP.md`.

</details>

### 🚧 Next Milestone (Phases 9-11, planned)

- [ ] **Phase 9: Provider-Neutral AI and Bounded Autonomy** - Users can use hosted or local models for reviewed authoring and explicitly armed live control while retaining auditable limits and immediate override.
- [ ] **Phase 10: Windows Release Qualification** - Operators can install and run a self-contained Windows release with measured timing, recovery, and hardware evidence under concurrent load.
- [ ] **Phase 11: Telemetry, Usage Statistics, and Auto Crash Submission Pipeline** - Users can opt into anonymized usage/telemetry collection and crashes are automatically captured and submitted for diagnosis without blocking playback or requiring manual repro steps.

`.planning/POST-PHASE-8-PLAN.md` (owner decisions recorded 2026-07-25) calls for a
short hygiene phase (doc/state drift, `internal/trace` race, catalog test
failures) and a front-door-UI completion phase (Fixture Library, show
open/new/switch, Guided First Show onboarding) to be inserted before Phase 9
starts, with phase numbers assigned at insertion time via `gsd-phase insert`.

## Phase Details

### Phase 9: Provider-Neutral AI and Bounded Autonomy

**Goal:** Users can employ hosted or local LLMs for evidence-backed authoring and explicitly bounded live control while deterministic execution and immediate operator authority remain local.
**Mode:** mvp
**Depends on:** Phases 2, 6, 7, and 8
**Requirements:** LLM-01, LLM-02, LLM-03, LLM-04, LLM-05, LLM-06, LLM-07, LLM-08, LLM-09
**Success Criteria** (what must be TRUE):

  1. A user can configure common hosted providers or a local OpenAI-compatible model through a provider-neutral adapter, with credentials excluded from show files, logs, fixture exports, and committed configuration.
  2. An LLM can produce an evidence-backed fixture draft and submit it through exactly the same validation, impact review, and commit pipeline as a human-authored fixture.
  3. An LLM can inspect a revisioned show snapshot and use typed tools to propose or modify pools, deployments, themes, presets, chases, scenes, blends, and playback mappings without inventing or bypassing domain identities.
  4. Live autonomy operates only under an explicitly armed, visible, time-bounded lease and rejects stale state or actions outside capability, risk, rate, time, and batch limits before execution.
  5. An operator can inspect proposed and executed commands, outcomes, errors, and redacted audit history, then revoke automation even during an in-flight or unreachable provider call; model inference never owns musical time, frame evaluation, raw DMX, or output cadence.

**Plans:** TBD
**UI hint:** yes
**Research:** Deeper phase research required for provider-wrapper maturity and parity, hosted/local structured outputs, context limits, cancellation, local deployment, evaluation corpus, safety policy, hazardous fixture restrictions, audit redaction, and staged validation before live autonomy.

### Phase 10: Windows Release Qualification

**Goal:** Operators can install and run a self-contained GOLC v1 on declared Windows systems with measured evidence that full-load operation, recovery, and real Art-Net output meet release budgets.
**Mode:** mvp
**Depends on:** Phases 1 through 9
**Requirements:** WIN-01, WIN-02, WIN-03, WIN-04
**Success Criteria** (what must be TRUE):

  1. A user can install and launch GOLC on every declared supported Windows version and architecture without a development toolchain.
  2. The packaged application includes and supervises every required runtime component, including the TypeScript helper, and reports missing or failed dependencies clearly.
  3. Clean Windows machines pass install, launch, save/restore, migration, network-change, suspend/resume, integrity, and recovery exercises.
  4. Long-running tests with real Art-Net hardware meet defined playback cadence, Art-Net timing, override latency, memory, and soak budgets while UI, storage, scripts, API clients, and LLM work run concurrently or fail.

**Plans:** TBD
**Research:** Deeper Windows qualification research required for the supported OS/architecture matrix, installer and signing policy, WebView/runtime dependencies, timer and jitter budgets, clean-machine lab, representative fixtures, physical Art-Net nodes, and release runbooks; macOS and Linux qualification remain outside v1.

### Phase 11: Telemetry, Usage Statistics, and Auto Crash Submission Pipeline

**Goal:** Users can opt into anonymized usage/telemetry collection and crashes are automatically captured and submitted for diagnosis without blocking playback or requiring manual repro steps.
**Requirements:** TELE-01, TELE-02, TELE-03, TELE-04
**Depends on:** Phase 10
**Plans:** 0 plans

Plans:

- [ ] TBD (run /gsd-plan-phase 11 to break down)

## Progress

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1. Offline Foundation and Delivery Traceability | v1.0 | 32/32 | Complete | 2026-07-21 |
| 2. Modular Fixtures and Deployments | v1.0 | 6/6 | Complete | 2026-07-21 |
| 3. Deterministic Show Programming and Playback | v1.0 | 7/7 | Complete | 2026-07-21 |
| 4. Observable Art-Net Live Output | v1.0 | 9/9 | Complete | 2026-07-22 |
| 5. Durable Shows and Recovery | v1.0 | 5/5 | Complete | 2026-07-23 |
| 6. Wails Authoring and Operator Surface | v1.0 | 12/12 | Complete | 2026-07-24 |
| 7. Versioned External Control API | v1.0 | 15/15 | Complete | 2026-07-25 |
| 8. Isolated TypeScript Automation | v1.0 | 13/13 | Complete | 2026-07-27 |
| 9. Provider-Neutral AI and Bounded Autonomy | next | 0/TBD | Not started | - |
| 10. Windows Release Qualification | next | 0/TBD | Not started | - |
| 11. Telemetry, Usage Statistics, and Auto Crash Submission Pipeline | next | 0/TBD | Not started | - |

---
*Roadmap created: 2026-07-17*
*Reorganized: 2026-07-27 after v1.0 milestone close (Phases 1-8 collapsed and archived to `.planning/milestones/v1.0-ROADMAP.md`)*
*Coverage target (v1.0, archived): 74/74 requirements mapped exactly once*
