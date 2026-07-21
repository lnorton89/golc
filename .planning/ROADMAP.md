# Roadmap: GOLC

## Overview

GOLC v1 grows through ten dependency-ordered MVP slices. The first slice makes configuration and delivery traceability discoverable and offline-safe; the next four prove modular fixture authoring, deterministic playback, real Art-Net output, and recoverable shows without depending on a desktop adapter. The Wails slice then turns those proven capabilities into the complete operator workflow before the same command model is exposed through the public API, an isolated TypeScript runtime, and bounded AI autonomy. Final qualification supports Windows only. Across every phase, UI, persistence, scripts, API, LLM providers, and Linear remain outside the deterministic playback and Art-Net timing path.

Linear traceability is established in Phase 1 and remains a delivery gate for every later phase. Repository planning artifacts and runtime operation remain complete offline; remote mappings are reconciled only through credential-external tooling, and no pending remote identifier is treated as evidence of synchronization.

## Phases

- [ ] **Phase 1: Offline Foundation and Delivery Traceability** - Contributors can build and govern the project from centralized configuration and durable local identities, with Linear reconciliation that never blocks offline work.
- [ ] **Phase 2: Modular Fixtures and Deployments** - Authors can validate fixture definitions and safely adapt logical pools to concrete deployments through reviewable atomic changes.
- [ ] **Phase 3: Deterministic Show Programming and Playback** - Authors can build tempo-aware shows whose compiled playback remains deterministic without any adapter owning musical or frame time.
- [ ] **Phase 4: Observable Art-Net Live Output** - Operators can send and inspect correct Art-Net frames from the independent playback engine through simulated and physical receivers.
- [ ] **Phase 5: Durable Shows and Recovery** - Users can save, restore, migrate, recover, inspect, and export shows without storage work disturbing live output.
- [ ] **Phase 6: Wails Authoring and Operator Surface** - Users can complete authoring and playback on screen or by keyboard, with constrained generic MIDI control and independent local safety actions.
- [ ] **Phase 7: Versioned External Control API** - External programs can safely inspect and control every public capability through the same typed command model as the desktop application.
- [ ] **Phase 8: Isolated TypeScript Automation** - Users can author and debug capability-limited TypeScript automation without scripts owning or blocking playback or Art-Net.
- [ ] **Phase 9: Provider-Neutral AI and Bounded Autonomy** - Users can use hosted or local models for reviewed authoring and explicitly armed live control while retaining auditable limits and immediate override.
- [ ] **Phase 10: Windows Release Qualification** - Operators can install and run a self-contained Windows release with measured timing, recovery, and hardware evidence under concurrent load.

## Phase Details

### Phase 1: Offline Foundation and Delivery Traceability

**Goal:** Contributors can configure, validate, build, and trace GOLC from durable repository-owned sources without requiring Linear or secrets to be available.
**Mode:** mvp
**Depends on:** Nothing (first phase)
**Requirements:** CONF-01, CONF-02, CONF-03, CONF-04, LINR-01, LINR-02, LINR-03, LINR-04
**Success Criteria** (what must be TRUE):

  1. A contributor can start at one documented root configuration and discover pinned toolchains plus setup, generation, validation, build, test, packaging, application-default, and runtime-configuration entrypoints.
  2. Contributors and CI can run the same commands, validate each configuration concern independently, and identify one authoritative value whenever settings are shared.
  3. A clean checkout contains no secrets or machine-local values, while safe examples document the external names needed when optional integrations are configured.
  4. Every milestone, phase, requirement, plan, and task can retain a durable local identity and complete planning context while Linear is unavailable.
  5. A contributor can preview an exact reconciliation and, when access is configured outside the repository, rerun it without duplicates; ambiguity, pagination, partial errors, and rate limits are reported without blocking local planning, builds, tests, or runtime operation.

**Plans:** 26/29 plans executed

- [x] 01-01-PLAN.md
- [x] 01-02-PLAN.md
- [x] 01-03-PLAN.md
- [x] 01-04-PLAN.md
- [x] 01-05-PLAN.md
- [x] 01-06-PLAN.md
- [x] 01-07-PLAN.md
- [x] 01-08-PLAN.md
- [x] 01-09-PLAN.md
- [x] 01-10-PLAN.md
- [x] 01-11-PLAN.md
- [x] 01-12-PLAN.md
- [x] 01-13-PLAN.md
- [x] 01-14-PLAN.md
- [ ] 01-15-PLAN.md
- [x] 01-16-PLAN.md
- [x] 01-17-PLAN.md
- [x] 01-18-PLAN.md
- [x] 01-19-PLAN.md
- [x] 01-20-PLAN.md
- [x] 01-21-PLAN.md
- [x] 01-22-PLAN.md
- [x] 01-23-PLAN.md
- [x] 01-24-PLAN.md
- [x] 01-25-PLAN.md
- [ ] 01-26-PLAN.md
- [ ] 01-27-PLAN.md
- [x] 01-28-PLAN.md
- [x] 01-29-PLAN.md

**Research:** Standard configuration and Linear UUID/reconciliation patterns; phase planning must settle local command boundaries, Linear taxonomy, and credential-external sync behavior without inventing remote IDs.

### Phase 2: Modular Fixtures and Deployments

**Goal:** Show authors can build a trustworthy semantic fixture catalog and adapt logical fixture pools to concrete deployments through explicit, atomic impact review.
**Mode:** mvp
**Depends on:** Phase 1
**Requirements:** FIXT-01, FIXT-02, FIXT-03, FIXT-04, FIXT-05, FIXT-06, POOL-01, POOL-02, POOL-03, POOL-04, POOL-05, POOL-06, POOL-07, POOL-08
**Success Criteria** (what must be TRUE):

  1. A user can load, create, edit, validate, and share versioned YAML fixture definitions, with duplicate keys, ambiguous constructs, invalid ranges, and unsupported semantics rejected by actionable diagnostics.
  2. A user can import an OFL definition through the same canonical normalization path and inspect provenance, validation, lossiness, stable identity, revision, schema version, and content hash before use.
  3. A show author can define logical fixture pools independently of quantity and addresses, then map them to concrete modes, universes, addresses, and fixture instances in a deployment.
  4. Adding or removing pool fixtures produces a deterministic review of every affected group, theme, palette, scene, chase, motion preset, and controller mapping; review-before-apply remains the default even when propagation policy is configurable.
  5. A show author can map replacement fixtures by semantic capability, see every missing or incompatible capability, and accept, revise, or cancel an all-or-nothing change without silent approximation.

**Plans:** TBD
**Research:** Deeper phase research required for canonical fixture semantics, pool propagation rules, representative first-user fixtures, OFL snapshot/licensing, GDTF preservation, hazardous attributes, and physical validation corpus.

### Phase 3: Deterministic Show Programming and Playback

**Goal:** Show authors can program complete tempo-aware looks and run them through a headless engine whose output is deterministic under adapter delay or failure.
**Mode:** mvp
**Depends on:** Phase 2
**Requirements:** PROG-01, PROG-02, PROG-03, PROG-04, PROG-05, PROG-06, PROG-07, SCEN-01, SCEN-02, SCEN-03, SCEN-04, SCEN-05, SCEN-06, SCEN-07, SCEN-08, SCEN-09
**Success Criteria** (what must be TRUE):

  1. A show author can select pools, groups, deployment instances, or individual fixtures; set semantic intensity, color, position, beam, and supported fixture-specific attributes; and inspect touched values, sources, and record scope.
  2. A show author can create and reuse themes, attribute presets, tempo-relative chases, and semantic motion presets, then record, update, rename, reorder, duplicate, or delete them with undo and redo.
  3. A show author can assemble a scene as a configured bar loop with independently enabled base-look, color-theme, chase, and motion layers plus reusable blending presets.
  4. An operator can enter or tap global BPM, switch the one active scene or any layer immediately, and choose whether a BPM change preserves musical position or restarts the loop.
  5. A deterministic playback harness produces the same time-indexed results when UI rendering, persistence, scripts, API clients, or LLM providers are slow, unavailable, or restarted, and adopts only complete valid show plans at safe boundaries.

**Plans:** TBD
**Research:** Deeper phase research required for playback jitter and override budgets, HTP/LTP and release semantics, live plan adoption, first-user scale ceilings, deterministic effect seeding, and Windows timing behavior.

### Phase 4: Observable Art-Net Live Output

**Goal:** Operators can drive a small Art-Net rig from deterministic complete frames and verify protocol, target, and timing health independently of the desktop UI.
**Mode:** mvp
**Depends on:** Phase 3
**Requirements:** ARTN-01, ARTN-02, ARTN-03, ARTN-04, ARTN-05, ARTN-06
**Success Criteria** (what must be TRUE):

  1. An operator can select a Windows network interface, configure universes and static unicast targets, optionally discover compatible nodes, and see current interface and target status.
  2. Independent packet inspection confirms correct Art-Net 4 addressing, sequencing, payload length, refresh, and target behavior for every configured universe.
  3. Playback continues publishing the newest complete frames at its defined cadence while UI, persistence, scripts, API, or LLM work is stalled or overloaded, without those components backpressuring the engine or Art-Net worker.
  4. An operator can inspect per-universe final values, frame health, target health, errors, and output enablement, and a release candidate demonstrates compatibility with both an independent simulator and real Art-Net hardware.

**Plans:** TBD
**UI hint:** yes
**Research:** Deeper phase research required for actual first-user nodes, subscriber/unicast behavior, multi-NIC and VPN cases, static-target workflow, compatibility policy, packet captures, and the physical hardware matrix.

### Phase 5: Durable Shows and Recovery

**Goal:** Users can preserve and recover complete shows in a portable versioned format while storage remains outside the deterministic playback path.
**Mode:** mvp
**Depends on:** Phase 3
**Requirements:** SHOW-01, SHOW-02, SHOW-03, SHOW-04, SHOW-05, SHOW-06
**Success Criteria** (what must be TRUE):

  1. A user can save a complete show and deployment to one portable versioned `.golc` file, then open, save, or save-as without unexpectedly stopping deterministic output.
  2. Authoring changes are autosaved to clearly identified rotating recovery points, and an interrupted session can be restored without storage work entering the playback timing path.
  3. A schema migration creates and verifies a backup, commits atomically, and refuses unsupported newer formats without rewriting them.
  4. A user can run integrity diagnostics and export a versioned human-readable JSON representation for troubleshooting and interchange.

**Plans:** TBD
**Research:** Targeted phase research required for SQLite durability settings, verified backup and retention policy, portable file/export rules, migration support window, read-only recovery, and Windows atomic replacement behavior.

### Phase 6: Wails Authoring and Operator Surface

**Goal:** Authors and playback operators can complete the conventional show workflow through a responsive Wails application, keyboard, and constrained generic MIDI controls without the frontend becoming runtime authority.
**Mode:** mvp
**Depends on:** Phases 2, 3, 4, and 5
**Requirements:** PLAY-01, PLAY-02, PLAY-03, PLAY-04, PLAY-05, PLAY-06, PLAY-07, PLAY-08, PLAY-09
**Success Criteria** (what must be TRUE):

  1. A user can complete fixture, deployment, programming, scene, and playback workflows through on-screen controls, and a documented keyboard workflow exposes every playback action without requiring MIDI hardware.
  2. A show author can create a constrained operator surface containing only assigned scenes, layers, masters, and safety controls, while the operator can always see active scene, layers, BPM/bar position, controlling source, and final output state.
  3. A show author can learn generic MIDI Note and Control Change input for supported playback commands and verify fader soft takeover without unintended value jumps.
  4. An operator can control group masters, Grand Master, stop/release-all, and immediate blackout through local priority paths that do not wait for UI, script, API, or model work to complete.
  5. Revoke Automation immediately blocks scripts and AI, cancels their queued actions, freezes the current look, and returns manual control even when an automation runtime is hung or disconnected.

**Plans:** TBD
**UI hint:** yes
**Validation:** Operator validation required for information density, navigation, patch-to-playback speed versus QLC+, constrained-surface learnability, cue-list needs, and the Wails/MIDI workflow.
**Blocker:** `MIDI-HW-01` RESOLVED 2026-07-19: Akai MIDImix, Novation Launch Control XL Mk2, and Worlde EasyControl 9 together are the selected Phase 6 physical acceptance set for generic MIDI Note/CC learn and soft takeover. `MIDI-HW-02` OPEN: each device requires independent physical evidence for its exact hardware revision, firmware, Windows version, and GOLC build before any named compatibility or support claim; device-specific profiles and feedback remain v1.x work under EXTN-04.

### Phase 7: Versioned External Control API

**Goal:** External programs can inspect and control all public GOLC capabilities through a secure, documented, revision-aware API that behaves like the desktop application.
**Mode:** mvp
**Depends on:** Phase 6
**Requirements:** API-01, API-02, API-03, API-04, API-05, API-06
**Success Criteria** (what must be TRUE):

  1. An external program can query and invoke every supported public domain capability through `/api/v1`, and parity checks show the same commands have the same outcomes through Wails and HTTP.
  2. A client can generate against the published OpenAPI contract, follow working examples, handle typed errors, and understand the documented compatibility and deprecation policy.
  3. A client can consume revisioned server-sent events, detect a replay gap, and recover by querying authoritative state.
  4. Mutations support expected revisions, idempotency, dry-run impact previews, and atomic meaningful batches; every result is auditable, while loopback is the default and remote access requires explicit enablement and scoped authentication.

**Plans:** TBD
**Research:** Standard API patterns; phase planning must define compatibility policy, remote-access threat model, SSE replay retention, rate limits, and audit redaction.

### Phase 8: Isolated TypeScript Automation

**Goal:** Users can create and debug typed TypeScript automation in a supervised, capability-limited process that cannot own or delay deterministic output.
**Mode:** mvp
**Depends on:** Phase 7
**Requirements:** SCRP-01, SCRP-02, SCRP-03, SCRP-04, SCRP-05, SCRP-06
**Success Criteria** (what must be TRUE):

  1. A user can create, edit, validate, run, stop, and debug a TypeScript script from the application.
  2. Scripts use a generated typed GOLC SDK for commands, queries, and events, and have no route to raw DMX or frame evaluation.
  3. Before execution, a user can inspect and assign script capabilities, deadlines, rate limits, and resource limits; the runtime has no ambient filesystem, network, environment, subprocess, native-code, or uncached dependency access.
  4. A user can inspect structured logs, diagnostics, source locations, command outcomes, and cancellation state, and can terminate a runaway, crashed, or blocked script without interrupting playback or Art-Net.

**Plans:** TBD
**UI hint:** yes
**Research:** Deeper phase research required for Deno distribution, offline dependency policy, process and IPC isolation, Windows CPU/memory enforcement, debugger scope, supervision, cancellation, and defensible sandbox claims.

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

## Progress

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Offline Foundation and Delivery Traceability | 26/29 | In Progress|  |
| 2. Modular Fixtures and Deployments | 0/TBD | Not started | - |
| 3. Deterministic Show Programming and Playback | 0/TBD | Not started | - |
| 4. Observable Art-Net Live Output | 0/TBD | Not started | - |
| 5. Durable Shows and Recovery | 0/TBD | Not started | - |
| 6. Wails Authoring and Operator Surface | 0/TBD | Not started | - |
| 7. Versioned External Control API | 0/TBD | Not started | - |
| 8. Isolated TypeScript Automation | 0/TBD | Not started | - |
| 9. Provider-Neutral AI and Bounded Autonomy | 0/TBD | Not started | - |
| 10. Windows Release Qualification | 0/TBD | Not started | - |

---
*Roadmap created: 2026-07-17*
*Coverage target: 84/84 v1 requirements mapped exactly once*
