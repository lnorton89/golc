# Roadmap: GOLC

## Overview

GOLC v1 grows through eleven dependency-ordered MVP slices. The first slice makes configuration and delivery traceability discoverable and offline-safe; the next four prove modular fixture authoring, deterministic playback, real Art-Net output, and recoverable shows without depending on a desktop adapter. The Wails slice then turns those proven capabilities into the complete operator workflow before the same command model is exposed through the public API, an isolated TypeScript runtime, and bounded AI autonomy. Final qualification supports Windows only. Across every phase, UI, persistence, scripts, API, LLM providers, and Linear remain outside the deterministic playback and Art-Net timing path.

Linear traceability is established in Phase 1 and remains a delivery gate for every later phase. Repository planning artifacts and runtime operation remain complete offline; remote mappings are reconciled only through credential-external tooling, and no pending remote identifier is treated as evidence of synchronization.

## Milestones

- ✅ **v1.0 Core Lighting Console** — Phases 1-8 (shipped 2026-07-27). See `.planning/MILESTONES.md` and `.planning/milestones/v1.0-ROADMAP.md` for the archived summary; full phase detail stays below (not collapsed) because `internal/trace/catalog` requires a live `### Phase N:` section with a `**Requirements:**` line for every phase directory still on disk under `.planning/phases/`.
- 🚧 **Next milestone (unnamed)** — Phases 9-11 (planned). `.planning/POST-PHASE-8-PLAN.md` (owner decisions 2026-07-25) calls for a hygiene phase and a front-door-UI completion phase to be inserted before Phase 9 starts.

## Phases

- [x] **Phase 1: Offline Foundation and Delivery Traceability** - Contributors can build and govern the project from centralized configuration and durable local identities, with Linear reconciliation that never blocks offline work. (completed 2026-07-21)
- [x] **Phase 2: Modular Fixtures and Deployments** - Authors can validate fixture definitions and safely adapt logical pools to concrete deployments through reviewable atomic changes. (completed 2026-07-21)
- [x] **Phase 3: Deterministic Show Programming and Playback** - Authors can build tempo-aware shows whose compiled playback remains deterministic without any adapter owning musical or frame time. (completed 2026-07-21)
- [x] **Phase 4: Observable Art-Net Live Output** - Operators can send and inspect correct Art-Net frames from the independent playback engine through simulated and physical receivers. (completed 2026-07-22)
- [x] **Phase 5: Durable Shows and Recovery** - Users can save, restore, migrate, recover, inspect, and export shows without storage work disturbing live output. (completed 2026-07-23)
- [x] **Phase 6: Wails Authoring and Operator Surface** - Users can complete authoring and playback on screen or by keyboard, with constrained generic MIDI control and independent local safety actions. (completed 2026-07-24)
- [x] **Phase 7: Versioned External Control API** - External programs can safely inspect and control every public capability through the same typed command model as the desktop application. (completed 2026-07-25)
- [x] **Phase 8: Isolated TypeScript Automation** - Users can author and debug capability-limited TypeScript automation without scripts owning or blocking playback or Art-Net. (completed 2026-07-27)
- [ ] **Phase 9: Front-Door UI Completion** - A new operator can go from a fresh checkout to a patched fixture and a scene on screen using only the UI, through the Fixture Library workspace, show open/new/switch, and Guided First Show onboarding.
- [ ] **Phase 10: Provider-Neutral AI and Bounded Autonomy** - Users can use hosted or local models for reviewed authoring and explicitly armed live control while retaining auditable limits and immediate override.
- [ ] **Phase 11: Windows Release Qualification** - Operators can install and run a self-contained Windows release with measured timing, recovery, and hardware evidence under concurrent load.
- [ ] **Phase 12: Telemetry, Usage Statistics, and Auto Crash Submission Pipeline** - Users can opt into anonymized usage/telemetry collection and crashes are automatically captured and submitted for diagnosis without blocking playback or requiring manual repro steps.

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

**Plans:** 32/32 plans complete
**Wave 1**

- [x] 01-01-PLAN.md
- [x] 01-12-PLAN.md
- [x] 01-30-PLAN.md — Gap 1/CR-01: rewire runLinearApply to apply.RunApply (D-18 staleness + D-21 journal resume) so retried apply never duplicates (LINR-03)
- [x] 01-31-PLAN.md — Gap 2/CR-02: wrap adapter.ts readOperation in try/catch returning found:false so a read failure can't stall the NDJSON process (LINR-04)

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 01-16-PLAN.md
- [x] 01-32-PLAN.md — Gap 3: check off LINR-01/LINR-02 and certify LINR-03/LINR-04 in REQUIREMENTS.md after the CR-01/CR-02 fixes (runs last)

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 01-17-PLAN.md

**Wave 4** *(blocked on Wave 3 completion)*

- [x] 01-02-PLAN.md

**Wave 5** *(blocked on Wave 4 completion)*

- [x] 01-03-PLAN.md
- [x] 01-08-PLAN.md

**Wave 6** *(blocked on Wave 5 completion)*

- [x] 01-09-PLAN.md
- [x] 01-18-PLAN.md
- [x] 01-21-PLAN.md

**Wave 7** *(blocked on Wave 6 completion)*

- [x] 01-04-PLAN.md
- [x] 01-05-PLAN.md
- [x] 01-10-PLAN.md

**Wave 8** *(blocked on Wave 7 completion)*

- [x] 01-19-PLAN.md
- [x] 01-23-PLAN.md
- [x] 01-28-PLAN.md

**Wave 9** *(blocked on Wave 8 completion)*

- [x] 01-06-PLAN.md
- [x] 01-11-PLAN.md
- [x] 01-22-PLAN.md

**Wave 10** *(blocked on Wave 9 completion)*

- [x] 01-20-PLAN.md
- [x] 01-24-PLAN.md

**Wave 11** *(blocked on Wave 10 completion)*

- [x] 01-07-PLAN.md

**Wave 12** *(blocked on Wave 11 completion)*

- [x] 01-13-PLAN.md

**Wave 13** *(blocked on Wave 12 completion)*

- [x] 01-29-PLAN.md

**Wave 14** *(blocked on Wave 13 completion)*

- [x] 01-25-PLAN.md

**Wave 15** *(blocked on Wave 14 completion)*

- [x] 01-14-PLAN.md

**Wave 16** *(blocked on Wave 15 completion)*

- [x] 01-26-PLAN.md

**Wave 17** *(blocked on Wave 16 completion)*

- [x] 01-27-PLAN.md

**Wave 18** *(blocked on Wave 17 completion)*

- [x] 01-15-PLAN.md

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

**Plans:** 6/6 plans complete

Plans:
**Wave 1**

- [x] 02-01-PLAN.md — Fixture validate: canonical capability model, generated versioned schema, strict YAML decode + diagnostics, `golc fixture validate` (FIXT-01/02/04) [Wave 1]

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 02-02-PLAN.md — Fixture identity/hash pinning + provenance, `golc fixture inspect` (FIXT-05/06) [Wave 2]
- [x] 02-04-PLAN.md — Pool + deployment domain model + revisioned ShowState, `golc pool/deployment create` (POOL-01/02) [Wave 2]

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 02-03-PLAN.md — OFL import: SSRF-guarded fetch/cache, normalize onto canonical model, lossy warnings, `golc fixture import` (FIXT-03/06) [Wave 3]
- [x] 02-05-PLAN.md — Pool impact review + integrity/freshness gates + atomic apply, `golc pool update`/`pool apply` (POOL-03/04/05/08) [Wave 3]

**Wave 4** *(blocked on Wave 3 completion)*

- [x] 02-06-PLAN.md — Capability-diff fixture substitution + severity taxonomy, `golc pool substitute` (POOL-06/07/08) [Wave 4]

**Waves:** W1: 02-01 · W2: 02-02, 02-04 · W3: 02-03, 02-05 · W4: 02-06
**Research:** Deeper phase research required for canonical fixture semantics, pool propagation rules, representative first-user fixtures, OFL snapshot/licensing, GDTF preservation, hazardous attributes, and physical validation corpus.

### Phase 3: Deterministic Show Programming and Playback

**Goal:** As a show author, I want to program complete tempo-aware looks and run them through a headless playback engine, so that the show's output stays deterministic even when the UI, persistence, scripts, an API client, or an LLM provider is slow, unavailable, or restarted.
**Mode:** mvp
**Depends on:** Phase 2
**Requirements:** PROG-01, PROG-02, PROG-03, PROG-04, PROG-05, PROG-06, PROG-07, SCEN-01, SCEN-02, SCEN-03, SCEN-04, SCEN-05, SCEN-06, SCEN-07, SCEN-08, SCEN-09
**Success Criteria** (what must be TRUE):

  1. A show author can select pools, groups, deployment instances, or individual fixtures; set semantic intensity, color, position, beam, and supported fixture-specific attributes; and inspect touched values, sources, and record scope.
  2. A show author can create and reuse themes, attribute presets, tempo-relative chases, and semantic motion presets, then record, update, rename, reorder, duplicate, or delete them with undo and redo.
  3. A show author can assemble a scene as a configured bar loop with independently enabled base-look, color-theme, chase, and motion layers plus reusable blending presets.
  4. An operator can enter or tap global BPM, switch the one active scene or any layer immediately, and choose whether a BPM change preserves musical position or restarts the loop.
  5. A deterministic playback harness produces the same time-indexed results when UI rendering, persistence, scripts, API clients, or LLM providers are slow, unavailable, or restarted, and adopts only complete valid show plans at safe boundaries.

**Plans:** 7/7 plans complete

Plans:
**Wave 1**

- [x] 03-01-PLAN.md — Selection + programmer semantic attribute editing + inspect (PROG-01/02/03) [Wave 1]

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 03-02-PLAN.md — Reusable color themes + intensity/color/position/beam presets recorded from programmer state (PROG-04) [Wave 2]

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 03-03-PLAN.md — Ordered tempo-relative chases + position/beam-only motion presets (PROG-05/06) [Wave 3]

**Wave 4** *(blocked on Wave 3 completion)*

- [x] 03-04-PLAN.md — Scenes + independently enabled layers with fixed-priority reduce + blend presets + global Tempo (SCEN-01/04/05/07) [Wave 4]

**Wave 5** *(blocked on Wave 4 completion)*

- [x] 03-05-PLAN.md — Session-only linear undo/redo history + full record/update/rename/reorder/duplicate/delete CRUD surface (PROG-07) [Wave 5]
- [x] 03-06-PLAN.md — Pure musical clock + numeric/tap BPM entry + preserve/restart epoch (SCEN-01/02/03/08) [Wave 5]

**Wave 6** *(blocked on Wave 5 completion)*

- [x] 03-07-PLAN.md — All-or-nothing compile + pure evaluate + real-time engine with next-bar adoption + SCEN-09 determinism proof (SCEN-06/09) [Wave 6]

**Waves:** W1: 03-01 · W2: 03-02 · W3: 03-03 · W4: 03-04 · W5: 03-05, 03-06 · W6: 03-07
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

**Plans:** 9/9 plans complete

Plans:
**Wave 1**

- [x] 04-01-PLAN.md — Encoding foundation: additive fixture channel-order (D-16/D-17) + byte-exact ArtDMX codec + semantic→DMX channel map (ARTN-03) [Wave 1]

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 04-02-PLAN.md — Windows interface selection/loss (D-05) + unicast target model with fan-out & enable/disable (ARTN-01/02, D-07/D-08/D-12) [Wave 2]

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 04-03-PLAN.md — Non-blocking ticker-driven worker + frame/target health model (ARTN-03/04/05, D-09/D-10/D-11) [Wave 3]

**Wave 4** *(blocked on Wave 3 completion)*

- [x] 04-04-PLAN.md — Long-lived standalone daemon + local named-pipe IPC via go-winio (ARTN-04, D-03/D-04) [Wave 4]

**Wave 5** *(blocked on Wave 4 completion)*

- [x] 04-05-PLAN.md — CLI operator surface: serve/interface/configure/status (watch+snapshot+--json)/target enable-disable (ARTN-01/02/05, D-01/D-02/D-12) [Wave 5]

**Wave 6** *(blocked on Wave 5 completion)*

- [x] 04-06-PLAN.md — Optional node discovery: ArtPoll/ArtPollReply codec + `artnet discover` suggestions-only (ARTN-02, D-06) [Wave 6]

**Wave 7** *(blocked on Wave 6 completion)*

- [x] 04-07-PLAN.md — ARTN-06 release-candidate verification: Wireshark/OLA runbook + human-verify checkpoint (ARTN-06, D-13/D-14/D-15/D-18) [Wave 7]

**Gap Closure** *(post-verification; closes 04-VERIFICATION.md's 2 gaps, does not touch 04-01…04-07)*

- [x] 04-08-PLAN.md — Surface per-universe final DMX values through `artnet status` (ARTN-05 / Success Criterion 4; VERIFICATION Gap 1) [gap wave 1]
- [x] 04-09-PLAN.md — Surface pinned-interface degraded/lost status via `artnet status` + `interface list` (ARTN-01/D-05 / Success Criterion 1; VERIFICATION Gap 2; depends on 04-08) [gap wave 2]

**Waves:** W1: 04-01 · W2: 04-02 · W3: 04-03 · W4: 04-04 · W5: 04-05 · W6: 04-06 · W7: 04-07 · gap W1: 04-08 · gap W2: 04-09
**UI hint:** yes (resolved to CLI-only per D-01; no Wails GUI until Phase 6)
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

**Plans:** 5/5 plans complete

Plans:
**Wave 1**

- [x] 05-01-PLAN.md — SQLite `.golc` store: save/open round-trip + recovery-point write (SHOW-01/02/03) [Wave 1]

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 05-02-PLAN.md — show open/save/save-as CLI + session recovery detect/offer/discard (SHOW-02/04) [Wave 2]
- [x] 05-03-PLAN.md — verified backup (VACUUM INTO + read-back-validate) + migration engine (SHOW-05) [Wave 2]
- [x] 05-04-PLAN.md — integrity diagnostics + versioned JSON export (SHOW-06) [Wave 2]

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 05-05-PLAN.md — migration-on-open confirm flow + newer-format refusal (SHOW-05, D-08/D-10) [Wave 3]

**Waves:** W1: 05-01 · W2: 05-02, 05-03, 05-04 · W3: 05-05
**Research:** Targeted phase research required for SQLite durability settings, verified backup and retention policy, portable file/export rules, migration support window, read-only recovery, and Windows atomic replacement behavior.

### Phase 6: Wails Authoring and Operator Surface

**Goal:** Authors and playback operators can complete the conventional show workflow through a responsive Wails application, keyboard, and constrained generic MIDI controls without the frontend becoming runtime authority.
**Mode:** mvp
**Depends on:** Phases 2, 3, 4, and 5
**Requirements:** PLAY-01, PLAY-02, PLAY-03, PLAY-04, PLAY-05, PLAY-06, PLAY-07, PLAY-08, PLAY-09, PLAY-10, PLAY-11, PLAY-12
**Success Criteria** (what must be TRUE):

  1. A user can complete fixture, deployment, programming, scene, and playback workflows through on-screen controls, and a documented keyboard workflow exposes every playback action without requiring MIDI hardware.
  2. A show author can create a constrained operator surface containing only assigned scenes, layers, masters, and safety controls, while the operator can always see active scene, layers, BPM/bar position, controlling source, and final output state.
  3. A show author can learn generic MIDI Note and Control Change input for supported playback commands and verify fader soft takeover without unintended value jumps.
  4. An operator can control group masters, Grand Master, stop/release-all, and immediate blackout through local priority paths that do not wait for UI, script, API, or model work to complete.
  5. Revoke Automation immediately blocks scripts and AI, cancels their queued actions, freezes the current look, and returns manual control even when an automation runtime is hung or disconnected.

**Plans:** 12/12 plans executed

Plans:
**Wave 1**

- [x] 06-01-PLAN.md — Operator surface model + persistence + CLI (PLAY-03; D-01/D-02/D-03/D-04/D-06/D-07)
- [x] 06-02-PLAN.md — Daemon-resident safety override + master levels + Worker integration + CLI (PLAY-06/08/09)
- [x] 06-03-PLAN.md — MIDI pure logic: cross-to-catch soft takeover + learn conflict (PLAY-04/05; D-05/D-06/D-09..D-12)

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 06-04-PLAN.md — Wails shell + Go host + OS-level safety hotkeys + daemon supervision (PLAY-01/09; D-13/D-16)

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 06-05-PLAN.md — Safety cluster UI + live status bar (PLAY-06/07/08/09; D-13/D-14/D-15)
- [x] 06-06-PLAN.md — On-screen playback controls + documented keyboard workflow (PLAY-01/02)
- [x] 06-07-PLAN.md — Operator surface builder UI + visible-but-locked renderer (PLAY-03/07; D-01/D-04)

**Wave 4** *(blocked on Wave 3 completion)*

- [x] 06-08-PLAN.md — MIDI driver + per-control learn UI + soft-takeover sliders (PLAY-04/05; D-05/D-08/D-09/D-10)

**Gap Closure** *(post-verification; closes 06-VERIFICATION.md's 2 gaps, does not touch 06-01…06-08)*

- [x] 06-09-PLAN.md — MIDI dispatch wiring: a mapped Note/CC actually switches scenes / toggles layers / sets master level / triggers safety, not only feedback (PLAY-04/05; VERIFICATION Gap B[1]) [gap wave 1]
- [x] 06-10-PLAN.md — On-screen fixture-patch UI: pool create/update-preview/apply + deployment create/activate (PLAY-10; SC1; VERIFICATION Gap B[0]) [gap wave 1]
- [x] 06-11-PLAN.md — On-screen deployment-interface + Art-Net universe/target config UI over the supervised daemon (PLAY-11; SC1; VERIFICATION Gap B[0]; depends on 06-10) [gap wave 2]
- [x] 06-12-PLAN.md — On-screen scene/look programming UI: scenes + base-look/color-theme/chase/motion layers (PLAY-12; SC1; VERIFICATION Gap B[0]; depends on 06-11) [gap wave 3]

**Waves:** W1: 06-01, 06-02, 06-03 · W2: 06-04 · W3: 06-05, 06-06, 06-07 · W4: 06-08 · gap W1: 06-09, 06-10 · gap W2: 06-11 · gap W3: 06-12
**UI hint:** yes
**Validation:** Operator validation required for information density, navigation, patch-to-playback speed versus QLC+, constrained-surface learnability, cue-list needs, and the Wails/MIDI workflow.
**Blocker:** `MIDI-HW-01` RESOLVED 2026-07-19: Akai MIDImix, Novation Launch Control XL Mk2, and Worlde EasyControl 9 together are the selected Phase 6 physical acceptance set for generic MIDI Note/CC learn and soft takeover. `MIDI-HW-02` OPEN: each device requires independent physical evidence for its exact hardware revision, firmware, Windows version, and GOLC build before any named compatibility or support claim; device-specific profiles and feedback remain v1.x work under EXTN-04.

### Phase 7: Versioned External Control API

**Goal:** External programs can inspect and control all public GOLC capabilities through a secure, documented, revision-aware API that behaves like the desktop application.
**Mode:** mvp
**Depends on:** Phase 6
**Requirements:** API-01, API-02, API-03, API-04, API-05, API-06
**Success Criteria** (what must be TRUE):

  1. An external program can, through `/api/v1`: inspect configuration concerns and show state, create a fixture pool, mint/list/revoke scoped API keys, apply an ordered multi-command batch atomically, and subscribe to revisioned change events -- each dispatched through the same internal/command route registry the desktop UI uses, with a committed capability-coverage gate proving every remaining public route is individually named and reasoned rather than silently unmapped. Full `/api/v1` breadth for the remaining show domains (scene, chase, motion, theme, preset, blend, deployment, operatorsurface, playback, programmer, fixture-import, show open/save) and Art-Net runtime control, plus the Wails-versus-HTTP outcome-parity check that only becomes meaningful once more than one mutating domain is exposed, is owned by EXTN-05 (v1.x) and is deferred, not dropped.
  2. A client can generate against the published OpenAPI contract, follow working examples, handle typed errors, and understand the documented compatibility and deprecation policy.
  3. A client can consume revisioned server-sent events, detect a replay gap, and recover by querying authoritative state.
  4. Mutations support expected revisions, idempotency, dry-run impact previews, and atomic meaningful batches; every result is auditable, while loopback is the default and remote access requires explicit enablement and scoped authentication.

**Plans:** 15/15 plans complete

Plans:
**Wave 1**

- [x] 07-01-PLAN.md — Pin chi/huma/x-time deps with a blocking supply-chain checkpoint (API-01/02/05 enabler)

**Wave 2** *(blocked on Wave 1)*

- [x] 07-02-PLAN.md — First slice: Chi+Huma /v1 server hosted in the daemon (D-07) + HTTP->command translation + read endpoints + capability-coverage gate (API-01; D-01/D-02/D-04)

**Wave 3** *(blocked on Wave 2)*

- [x] 07-03-PLAN.md — api config concern + enforced loopback-default bind, remote requires explicit flag+interface (API-05; D-06)

**Wave 4** *(blocked on Wave 3)*

- [x] 07-04-PLAN.md — Scoped/expiring/revocable API keys + auth + per-key rate limit; api_keys table (API-05; D-05/D-08)

**Wave 5** *(blocked on Wave 4)*

- [x] 07-05-PLAN.md — Serialized mutations: If-Match/412 + dry-run + idempotency + post-mutation observer seam (API-04/API-01; D-13/D-14)

**Wave 6** *(blocked on Wave 5)*

- [x] 07-06-PLAN.md — Atomic /v1/batch via copy + single aggregated Save (all-or-nothing, no command-handler refactor) (API-04; D-15)

**Wave 7** *(blocked on Wave 6)*

- [x] 07-07-PLAN.md — audit_log table + redacting post-mutation audit writer (API-06; D-16) [parallel with 07-08]
- [x] 07-08-PLAN.md — Revisioned global SSE + Last-Event-ID replay + resync-on-gap + revocation tick (API-03; D-09/D-10/D-11/D-12) [parallel with 07-07]

**Wave 8** *(blocked on Wave 7)*

- [x] 07-09-PLAN.md — Generated OpenAPI 3.1 contract + drift check + typed errors + compatibility/deprecation policy + coverage closure (API-02; D-03/D-02)

**Gap Closure** *(post-verification; closes 07-VERIFICATION.md's 3 gaps plus 07-REVIEW.md's non-blocking findings, does not touch 07-01…07-09)*

- [x] 07-10-PLAN.md — Re-scope API-01 to the delivered /v1 breadth; name EXTN-05 (v1.x) as the deferral owner for the remaining domains + Wails/HTTP parity, enforced by the coverage gate (API-01; VERIFICATION Gap 1) [gap wave 1 / wave 9]
- [x] 07-11-PLAN.md — Strictly monotonic SSE sequence id decoupled from show revision, so a multi-sub-request batch's events stay individually replayable (API-03; VERIFICATION Gap 2 / REVIEW CR-01) [gap wave 1 / wave 9]
- [x] 07-12-PLAN.md — Audit every batch pre-flight rejection at parity with the single-mutation path; close mutate.go's latent unobserved 500 branch (API-06; VERIFICATION Gap 3 / REVIEW WR-02, WR-03) [gap wave 1 / wave 9]
- [x] 07-13-PLAN.md — Scope Idempotency-Key by (actor, route, key); reject the reserved delimiter in list-valued fields at the boundary (API-04; REVIEW WR-01, IN-02) [gap wave 2 / wave 10]
- [x] 07-14-PLAN.md — Install DeprecationMiddleware so the documented Deprecation/Sunset signals are real; bound API-key lifetime; validate the scopes list (API-02/API-05; REVIEW WR-04, IN-01, IN-02) [gap wave 3 / wave 11]

**Gap Closure Round 2** *(post-re-verification; closes 07-VERIFICATION.md's sole remaining gap, does not touch 07-01…07-14)*

- [x] 07-15-PLAN.md — Audit every failure return in runBatch's LOCKED section (nine branches, not the eight the findings enumerate) at parity with the pre-flight loop; structural gate for the five fault-injection-only branches (API-06; VERIFICATION remaining gap / REVIEW-gaps WR-05) [gap wave 4 / wave 12]

**Waves:** W1: 07-01 · W2: 07-02 · W3: 07-03 · W4: 07-04 · W5: 07-05 · W6: 07-06 · W7: 07-07, 07-08 · W8: 07-09 · gap W1: 07-10, 07-11, 07-12 · gap W2: 07-13 · gap W3: 07-14 · gap W4: 07-15
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

**Plans:** 14/14 plans executed

Plans:
**Wave 1**

- [x] 08-01-PLAN.md — Script entity, capability profile, and `script` CLI CRUD routes in show.State
- [x] 08-02-PLAN.md — Deno toolchain pin, bootstrap provisioning, and the single executable resolver
- [x] 08-03-PLAN.md — Typed SDK generator, committed golc.d.ts/runtime shim, and route-coverage gate

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 08-04-PLAN.md — Scripts workspace: library view, editor, save/delete, and shell navigation
- [x] 08-05-PLAN.md — Zero-permission Deno host, stdio session protocol, and `script run`

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 08-06-PLAN.md — Capability/rate/deadline enforcement, Windows Job Object caps, Stop, Art-Net non-interference
- [x] 08-07-PLAN.md — Zero-import gate, `deno check` validation, and source-mapped diagnostics

**Wave 4** *(blocked on Wave 3 completion)*

- [x] 08-08-PLAN.md — Live script event stream, per-call outcomes, and audit-pipeline integration

**Wave 5** *(blocked on Wave 4 completion)*

- [x] 08-09-PLAN.md — Debug-mode-only inspector, CDP debug bridge, and source-mapped stack traces

**Wave 6** *(blocked on Wave 5 completion)*

- [x] 08-10-PLAN.md — Run/Debug launch dialog, toolbar actions, and live debug panel with terminal states

**Wave 7** *(blocked on Wave 6 completion)*

- [x] 08-11-PLAN.md — Monaco editor with live type-checking against the generated SDK (gated on package legitimacy)

**Wave 8** *(blocked on Wave 7 completion)*

- [x] 08-12-PLAN.md — Breakpoint gutter, execution-line highlight, and step controls

**Wave 9** *(blocked on Wave 8 completion)*

- [x] 08-13-PLAN.md — Phase acceptance: sandbox denial surface and full authoring-to-debugging workflow

**Gap closure** *(from 08-VERIFICATION.md, status: gaps_found)*

- [x] 08-14-PLAN.md — Proactive Job Object memory monitor so a memory-limit kill renders its named Copywriting Contract sentence instead of a raw V8 RangeError

**UI hint:** yes
**Research:** Deeper phase research required for Deno distribution, offline dependency policy, process and IPC isolation, Windows CPU/memory enforcement, debugger scope, supervision, cancellation, and defensible sandbox claims.

### Phase 9: Front-Door UI Completion

**Goal:** A new operator can go from a fresh checkout to a patched fixture and a scene on screen using only the UI, with Guided First Show onboarding as the on-ramp — no CLI required for the happy path.
**Mode:** mvp
**Depends on:** Phase 8
**Requirements:** FDUI-01, FDUI-02, FDUI-03
**Success Criteria** (what must be TRUE):

  1. A user can browse, inspect, and import fixture definitions (OFL and hand-authored YAML) through the Fixture Library workspace, replacing its current `ComingSoon` stub — the existing `fixture validate`/`fixture inspect`/`fixture import` backend routes are already available to wire against.
  2. A user can open an existing show, create a new show, and switch between shows through on-screen controls — the CLI flow (`show open`, recovery accept/discard) already exists; this surfaces it, and `SaveRecoveryWorkspace.tsx` no longer documents a single-show-path-at-startup limitation.
  3. A first-time user can complete Guided First Show onboarding (Sketch 004-B, approved but never built) to go from empty app to a patched fixture and a scene on screen.

**Plans:** 7/7 plans executed

Plans:
**Wave 1**

- [x] 09-01-PLAN.md — Fixture Library: browse and inspect local fixtures (wave 1)

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 09-02-PLAN.md — Shows: open / new / switch via supervised self-relaunch (wave 2)

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 09-03-PLAN.md — Guided First Show: overlay, entry points, Fixtures and Patch stages (wave 3)
- [x] 09-05-PLAN.md — Fixture Library: imported artifacts in the library, OFL manufacturer search (wave 3)

**Wave 4** *(blocked on Wave 3 completion)*

- [x] 09-04-PLAN.md — Guided First Show: Program, Assign, Verify stages and the readiness gate (wave 4)
- [x] 09-06-PLAN.md — Fixture Library: OFL import preview and commit (wave 4)

**Wave 5** *(blocked on Wave 4 completion)*

- [x] 09-07-PLAN.md — Fixture Library: custom YAML fixture via native picker (wave 5)

**UI hint:** yes
**Research:** Scoped by `.planning/POST-PHASE-8-PLAN.md` section 2 (owner decisions 2026-07-25) — this is UI wiring against existing backend routes, not new product design; standard `/gsd-discuss-phase` and `/gsd-plan-phase` should confirm scope and any remaining gray areas (e.g. whether show open/new/switch lives in the existing Show nav group or needs a new entry point) before planning.

### Phase 10: Provider-Neutral AI and Bounded Autonomy

**Goal:** Users can employ hosted or local LLMs for evidence-backed authoring and explicitly bounded live control while deterministic execution and immediate operator authority remain local.
**Mode:** mvp
**Depends on:** Phases 2, 6, 7, 8, and 9
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

### Phase 11: Windows Release Qualification

**Goal:** Operators can install and run a self-contained GOLC v1 on declared Windows systems with measured evidence that full-load operation, recovery, and real Art-Net output meet release budgets.
**Mode:** mvp
**Depends on:** Phases 1 through 10
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
| 1. Offline Foundation and Delivery Traceability | 32/32 | Complete    | 2026-07-21 |
| 2. Modular Fixtures and Deployments | 6/6 | Complete    | 2026-07-21 |
| 3. Deterministic Show Programming and Playback | 7/7 | Complete    | 2026-07-21 |
| 4. Observable Art-Net Live Output | 9/9 | Complete    | 2026-07-22 |
| 5. Durable Shows and Recovery | 5/5 | Complete    | 2026-07-23 |
| 6. Wails Authoring and Operator Surface | 12/12 | Complete    | 2026-07-24 |
| 7. Versioned External Control API | 15/15 | Complete    | 2026-07-25 |
| 8. Isolated TypeScript Automation | 14/14 | In Progress|  |
| 9. Front-Door UI Completion | 7/7 | In Progress|  |
| 10. Provider-Neutral AI and Bounded Autonomy | 0/TBD | Not started | - |
| 11. Windows Release Qualification | 0/TBD | Not started | - |
| 12. Telemetry, Usage Statistics, and Auto Crash Submission Pipeline | 0/TBD | Not started | - |

### Phase 12: Telemetry, Usage Statistics, and Auto Crash Submission Pipeline

**Goal:** Users can opt into anonymized usage/telemetry collection and crashes are automatically captured and submitted for diagnosis without blocking playback or requiring manual repro steps.
**Requirements:** TELE-01, TELE-02, TELE-03, TELE-04
**Depends on:** Phase 11
**Plans:** 0 plans

Plans:

- [ ] TBD (run /gsd-plan-phase 12 to break down)

### Phase 13: Unified UI design system and automated enforcement

**Goal:** Every reachable desktop surface uses one documented Paper/Ink design system whose semantic tokens, typed components, accessibility states, theme parity, safety invariants, and exceptions are mechanically enforced with zero unregistered drift.
**Requirements**: D-01 through D-14 and the approved Phase 13 UI-SPEC acceptance contract
**Depends on:** Phase 12
**Plans:** 32/41 plans executed

Plans:

- [x] 13-01-PLAN.md — Approve and install exact parser pins only (wave 1)
- [x] 13-02-PLAN.md — Fail-closed DS001–DS010 checker and held-out fixtures (wave 3)
- [x] 13-03-PLAN.md — Button, IconButton, Field, and Chip contracts (wave 3)
- [x] 13-04-PLAN.md — Tabs and shared empty/loading/error states (wave 3)
- [x] 13-05-PLAN.md — Twelve-file panel/header/toolbar/row primitive slice (wave 4)
- [x] 13-06-PLAN.md — Chromium and packaged-WebView2 dialog proof (wave 5)
- [x] 13-07-PLAN.md — Product patterns, public inventory/barrel/guide, and gallery (wave 6)
- [x] 13-08-PLAN.md — Four-file generated theme consumption migration (wave 7)
- [x] 13-09-PLAN.md — Twelve-file front-door workspace migration (wave 7)
- [x] 13-10-PLAN.md — Fixture Library, Patch & Pools, and Project Fixtures migration (wave 7)
- [x] 13-11-PLAN.md — Bounded Scenes & Looks and SceneProgramming migration (wave 7)
- [x] 13-12-PLAN.md — Five-file Notes/Tiptap migration (wave 7)
- [x] 13-13-PLAN.md — Dedicated Desk migration and geometry proof (wave 7)
- [x] 13-14-PLAN.md — Bounded Operator Surface migration (wave 7)
- [x] 13-15-PLAN.md — Independent safety, live truth, and tempo migration (wave 7)
- [x] 13-16-PLAN.md — Inspector, overlays, error boundary, and log projection (wave 7)
- [x] 13-17-PLAN.md — Three-capture tolerance calibration before baselines (wave 8)
- [ ] 13-18-PLAN.md — Pinned package/Mage routes and required Windows workflow (wave 10)
- [ ] 13-19-PLAN.md — Evidence-driven exception merge and whole-source policy parity (wave 12)
- [ ] 13-20-PLAN.md — Plan-derived semantic evidence validator and mutation tests (wave 11)
- [x] 13-21-PLAN.md — Strict inert manifests and deterministic token generation (wave 2)
- [x] 13-22-PLAN.md — Six-file Dialog/ConfirmDialog public contract (wave 4)
- [x] 13-23-PLAN.md — Nine-file scroll/tooltip/resize utilities (wave 4)
- [x] 13-24-PLAN.md — Nine-file core shell and command rail migration (wave 7)
- [x] 13-25-PLAN.md — Guided First Show core and five stages with exact 8px spacing (wave 7)
- [x] 13-26-PLAN.md — Complete fourteen-file Art-Net and diagnostics migration in bounded tasks (wave 7)
- [x] 13-27-PLAN.md — Fourteen-file Scripts/Monaco migration in bounded tasks (wave 7)
- [x] 13-28-PLAN.md — Complete thirteen-file generic MIDI mapping/pickup migration in bounded tasks (wave 7)
- [x] 13-29-PLAN.md — Complete eleven-file hotkey and shared workspace chrome migration in bounded tasks (wave 7)
- [x] 13-30-PLAN.md — Pre-settle startup and token-independent ErrorBoundary backstops (wave 9)
- [x] 13-31-PLAN.md — Specialized geometry and explicit 2x-copy reflow backstops (wave 9)
- [x] 13-32-PLAN.md — Twelve shell/dialog/gallery Windows baselines (wave 9)
- [x] 13-33-PLAN.md — Twelve authoring Windows baselines (wave 9)
- [x] 13-34-PLAN.md — Twelve live/editor Windows baselines (wave 9)
- [ ] 13-35-PLAN.md — Explicit CI trigger authority and immutable implementation-tree artifact inspection (wave 15)
- [ ] 13-36-PLAN.md — Final public inventory/theme/contrast parity (wave 14)
- [ ] 13-37-PLAN.md — ConfirmModal implementation and compatibility removal (wave 13)
- [ ] 13-38-PLAN.md — Complete local acceptance with semantic result evidence (wave 16)
- [ ] 13-39-PLAN.md — Machine-checked final Nyquist sign-off (wave 17)
- [x] 13-40-PLAN.md — Six-file scene timeline and layer geometry migration (wave 7)
- [ ] 13-41-PLAN.md — 200% text-zoom and provider/daemon-offline safety acceptance (wave 9)

---
*Roadmap created: 2026-07-17*
*v1.0 (Phases 1-8) shipped 2026-07-27 — see `.planning/MILESTONES.md`. Phase Details below remain complete for every phase (not collapsed) so `internal/trace/catalog` can keep resolving every phase directory under `.planning/phases/`.*
*2026-07-27: Phase 9 (Front-Door UI Completion) inserted per `.planning/POST-PHASE-8-PLAN.md` section 2; former Phases 9/10/11 (AI autonomy, Windows qualification, telemetry) renumbered to 10/11/12. Renumbered as plain integers, not GSD's default decimal insertion (e.g. 8.1) — `internal/trace/catalog`'s phase-directory grammar and `### Phase N:` heading regex only match two-digit integers, so a decimal phase would be invisible to Linear traceability.*
*Coverage target: 84/84 v1 requirements mapped exactly once*
