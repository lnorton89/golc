# Requirements: GOLC

**Defined:** 2026-07-17
**Core Value:** An operator can author a modular show once, adapt its fixture pools to different deployments in one or two actions, and hand a simple controller surface to another person for reliable playback.

## User Stories

- As a show author, I can program against logical fixture pools so one show adapts to different deployments without being rebuilt.
- As a show author, I can assemble tempo-aware scenes from reusable color, chase, motion, and blend components.
- As a playback operator, I can run a prepared show from a simple keyboard, on-screen, or MIDI control surface without learning the authoring system.
- As an integrator, I can control every supported capability through stable TypeScript and external APIs that behave like the desktop UI.
- As an operator, I can let an LLM author or run show content while retaining an immediate, model-independent way to revoke automation.

## v1 Requirements

### Centralized Configuration

- [x] **CONF-01**: A contributor can discover toolchain versions, setup, generation, validation, build, test, packaging, application-default, and runtime-configuration entrypoints from one documented root project configuration.
- [x] **CONF-02**: Project configuration is separated into independently validatable concerns without duplicating authoritative values across files.
- [x] **CONF-03**: A contributor and CI can invoke the same documented project commands for generation, validation, build, test, and packaging.
- [x] **CONF-04**: Secrets and machine-local values remain outside committed project configuration and are represented by documented names and safe examples.

### Fixture Definitions

- [x] **FIXT-01**: A user can load fixture definitions authored in a documented, versioned YAML schema.
- [x] **FIXT-02**: The application rejects duplicate keys, ambiguous YAML constructs, invalid ranges, and unsupported fixture semantics with actionable diagnostics.
- [x] **FIXT-03**: A user can import an Open Fixture Library definition through GOLC's canonical validation and normalization pipeline.
- [x] **FIXT-04**: A user can create, edit, validate, and share a custom YAML fixture definition.
- [x] **FIXT-05**: A show pins each fixture definition by stable identity, schema version, content revision, and hash so library updates cannot silently change an existing show.
- [x] **FIXT-06**: A user can inspect a fixture definition's source, provenance, validation result, and unsupported or lossy import details before using it.

### Pools and Deployments

- [x] **POOL-01**: A show author can define a logical pool of compatible fixtures independently of the concrete fixture count, addresses, and deployment hardware.
- [x] **POOL-02**: A show author can create a deployment that maps logical pools to concrete fixture instances, modes, universes, and addresses.
- [x] **POOL-03**: A show author can add or remove fixtures from a pool and receive an impact review covering all dependent groups, themes, palettes, scenes, chases, motion presets, and controller mappings.
- [x] **POOL-04**: A show author can configure propagation behavior for each pool update while review-before-apply remains the default.
- [x] **POOL-05**: A reviewed pool update is applied atomically so dependents never observe a partially updated deployment.
- [x] **POOL-06**: A show author can replace a fixture model by mapping shared semantic capabilities rather than raw channel positions.
- [x] **POOL-07**: A fixture replacement review identifies missing, incompatible, and unsupported capabilities and never silently approximates them.
- [x] **POOL-08**: A show author can accept, revise, or cancel a pool or fixture-substitution impact plan before it changes the show.

### Programming

- [x] **PROG-01**: A show author can select fixtures by pool, group, deployment instance, or direct fixture selection.
- [x] **PROG-02**: A show author can edit semantic intensity, color, position, beam, and supported fixture-specific attributes without working in raw DMX channels.
- [x] **PROG-03**: The programmer shows which attributes are touched, their values, their sources, and what will be recorded.
- [x] **PROG-04**: A show author can create reusable color themes and intensity, color, position, and beam presets from programmer state.
- [x] **PROG-05**: A show author can create reusable chases with ordered steps and tempo-relative timing.
- [x] **PROG-06**: A show author can create reusable motion presets using semantic position capabilities.
- [x] **PROG-07**: A show author can record, update, rename, reorder, duplicate, and delete programming objects with undo and redo.

### Tempo-Aware Scenes

- [x] **SCEN-01**: A show author can create a scene that loops for a configured number of musical bars against the global BPM.
- [x] **SCEN-02**: An operator can set global BPM by entering a numeric value.
- [x] **SCEN-03**: An operator can set global BPM through tap tempo.
- [x] **SCEN-04**: Exactly one scene is active at a time during normal playback.
- [x] **SCEN-05**: A scene can combine independently enabled color-theme, chase, motion-preset, and base-look layers.
- [x] **SCEN-06**: An operator can switch the active scene or any scene layer immediately.
- [x] **SCEN-07**: A show author can create and assign reusable blending presets that define transitions between scene and layer states.
- [x] **SCEN-08**: A show author can configure whether a global BPM change preserves the active loop's musical position or restarts the loop.
- [x] **SCEN-09**: Scene timing and layer evaluation remain deterministic when UI rendering, scripts, API clients, or LLM providers are slow or unavailable.

### Playback and Operator Safety

- [ ] **PLAY-01**: An operator can access the complete playback workflow through on-screen controls.
- [ ] **PLAY-02**: An operator can access the complete playback workflow through documented keyboard controls.
- [x] **PLAY-03**: A show author can create a constrained operator surface that exposes only assigned scenes, layers, masters, and safety controls.
- [ ] **PLAY-04**: A show author can map generic MIDI Note and Control Change input to supported playback commands.
- [ ] **PLAY-05**: MIDI fader mappings support soft takeover so connecting or moving a controller does not cause unintended value jumps.
- [ ] **PLAY-06**: An operator can control group masters, a Grand Master, stop/release-all, and an immediate blackout.
- [ ] **PLAY-07**: An operator can see the active scene, enabled layers, current BPM/bar position, controlling source, and final output state.
- [ ] **PLAY-08**: Revoke Automation immediately blocks AI and scripts, cancels their queued actions, freezes the current look, and returns control to manual operation without requiring those runtimes to respond.
- [ ] **PLAY-09**: Blackout remains a separate local priority control that does not depend on the UI, script runtime, API, or LLM provider completing work.

### Art-Net Output

- [x] **ARTN-01**: An operator can select the Windows network interface used for Art-Net output and see its current status.
- [x] **ARTN-02**: An operator can configure Art-Net universes and static unicast targets and discover compatible nodes when discovery is enabled.
- [x] **ARTN-03**: The application emits valid Art-Net 4 output with correct addressing, sequence, payload-length, refresh, and target behavior for supported nodes.
- [x] **ARTN-04**: Art-Net output consumes complete frames from the deterministic playback engine without being backpressured by the UI, persistence, scripts, API, or LLM operations.
- [x] **ARTN-05**: An operator can inspect per-universe final values, frame health, target health, errors, and output enablement.
- [x] **ARTN-06**: A release can demonstrate packet and timing compatibility with an independent simulator and real Art-Net hardware.

### Show Storage and Recovery

- [x] **SHOW-01**: A user can save a complete show and its deployment data as one portable versioned `.golc` file.
- [x] **SHOW-02**: A user can open, save, and save-as a show without stopping deterministic output unexpectedly.
- [x] **SHOW-03**: The application autosaves recoverable authoring changes without performing storage work in the playback timing path.
- [x] **SHOW-04**: A user can recover from an interrupted or failed session using clearly identified rotating recovery points.
- [x] **SHOW-05**: Schema migration creates a verified backup, applies atomically, and refuses unsupported newer formats without rewriting them.
- [x] **SHOW-06**: A user can run integrity diagnostics and export a versioned human-readable JSON representation for interchange and troubleshooting.

### Public API

- [ ] **API-01**: An external program can query and invoke every supported public domain capability through a versioned API that uses the same application command model as the UI.
- [ ] **API-02**: The API publishes an OpenAPI contract, generated client examples, typed errors, and compatibility/deprecation guidance.
- [ ] **API-03**: An external client can subscribe to revisioned server-sent events and recover from an event gap by re-querying authoritative state.
- [ ] **API-04**: Mutating API operations support expected revisions, idempotency, dry-run impact previews, and atomic meaningful batches.
- [ ] **API-05**: The API binds to loopback by default and requires explicit enablement and scoped authentication for remote access.
- [ ] **API-06**: Every API mutation records actor, source, correlation, outcome, and redacted audit details.

### TypeScript Scripting

- [ ] **SCRP-01**: A user can create, edit, validate, run, stop, and debug TypeScript scripts from the application.
- [ ] **SCRP-02**: Scripts use a generated typed GOLC SDK for commands, queries, and events rather than raw DMX access.
- [ ] **SCRP-03**: Scripts execute outside the playback process with no ambient filesystem, network, environment, subprocess, or native-code permissions.
- [ ] **SCRP-04**: A user can assign script capabilities, deadlines, rate limits, and resource limits before execution.
- [ ] **SCRP-05**: A user can inspect structured script logs, diagnostics, source locations, command outcomes, and cancellation status.
- [ ] **SCRP-06**: A runaway, crashed, or blocked script can be terminated without interrupting playback or Art-Net output.

### LLM Integration and Autonomy

- [ ] **LLM-01**: A user can configure common hosted providers and local OpenAI-compatible models through an open-source provider-neutral wrapper.
- [ ] **LLM-02**: Provider credentials are stored outside show files, logs, exported fixtures, and committed project configuration.
- [ ] **LLM-03**: An LLM can draft a YAML fixture definition with evidence and submit it through the same validation, review, and commit pipeline as a human-authored definition.
- [ ] **LLM-04**: An LLM can inspect show state and use typed tools to create or modify pools, deployments, themes, presets, chases, scenes, blends, and playback mappings.
- [ ] **LLM-05**: An operator can grant an LLM autonomous live control through an explicitly armed, visible, time-bounded permission lease.
- [ ] **LLM-06**: LLM actions enforce allowed capabilities, expected state revisions, risk limits, rate limits, batch limits, and stale-state rejection before execution.
- [ ] **LLM-07**: An operator can inspect the model's proposed or executed commands, outcomes, errors, and redacted audit trail.
- [ ] **LLM-08**: LLM inference never owns musical time, evaluates frames, writes raw DMX, or blocks deterministic playback and Art-Net output.
- [ ] **LLM-09**: Revoke Automation remains effective when the model provider is unreachable, the model is unresponsive, or an LLM tool request is in flight.

### Windows Release

- [ ] **WIN-01**: A user can install and launch GOLC on the declared supported Windows versions and architectures without installing a development toolchain.
- [ ] **WIN-02**: A packaged Windows build includes and supervises every required runtime component, including the TypeScript helper.
- [ ] **WIN-03**: A Windows release passes clean-install, save/restore, migration, network-change, suspend/resume, and recovery exercises.
- [ ] **WIN-04**: A Windows release meets measured playback, Art-Net timing, override-latency, memory, and long-running soak budgets under concurrent UI, storage, script, API, and LLM load.

### Linear Traceability

- [x] **LINR-01**: Every milestone, phase, requirement, plan, and executable task has a durable local identifier that remains usable offline.
- [x] **LINR-02**: The repository maintains a credential-free mapping from durable local identifiers to immutable Linear UUIDs without making Linear the only source of planning truth.
- [x] **LINR-03**: A contributor can preview and run an idempotent reconciliation that creates or updates the intended Linear project, milestones, issues, and sub-issues without duplicating retried work.
- [x] **LINR-04**: Linear synchronization reports ambiguity, partial GraphQL errors, pagination, and rate limiting without blocking local planning, builds, tests, or application runtime.

## v1.x Requirements

### Cross-Show Modules

- **MODL-01**: A user can export a selected song, playback page, scene collection, or other programming collection as a reusable module.
- **MODL-02**: A user can import a reusable module into an independent show file.
- **MODL-03**: A user can optionally compare an imported module with its source and review upstream updates before applying them.

### Extended Control and Effects

- **EXTN-01**: An external OSC bridge can translate OSC messages into versioned API commands.
- **EXTN-02**: A show author can create advanced reusable parameter effects beyond v1 chases and motion presets.
- **EXTN-03**: A user can import supported QLC+ show content through an explicit compatibility report.
- **EXTN-04**: A user can install device-specific MIDI profiles after target controllers are validated.

## v2 Requirements

### Protocols and Platforms

- **FUTR-01**: The application can output sACN through the protocol abstraction.
- **FUTR-02**: The application can output through supported USB-DMX devices.
- **FUTR-03**: The application is packaged and qualified for supported macOS versions.
- **FUTR-04**: The application is packaged and qualified for supported Linux distributions.

### Advanced Production

- **FUTR-05**: A user can program professional tracking and move-while-dark cue behavior.
- **FUTR-06**: A user can synchronize playback to timecode, media, or audio analysis.
- **FUTR-07**: A user can program pixel-mapped and 2D/3D visualized output.
- **FUTR-08**: Multiple consoles or users can coordinate redundant or collaborative show operation.
- **FUTR-09**: A user can operate a thin native mobile client.
- **FUTR-10**: A user can discover and install extensions through a managed marketplace.

## Out of Scope

| Feature | Reason |
|---------|--------|
| Raw DMX writes from scripts, APIs, or LLM tools | Bypasses semantic validation, source arbitration, safety controls, and auditability |
| Cloud-required show authoring or playback | Live operation and repository planning must remain functional offline |
| Multiple lighting protocols at launch | Art-Net must be proven before expanding the protocol surface |
| Silent fixture-capability approximation | Unsupported semantics must be reviewed explicitly to avoid unsafe or misleading output |
| LLM-owned frame or musical timing | Network inference cannot provide deterministic live output |
| Unbounded or unreviewable autonomous control | Full autonomy still requires explicit authority, visibility, limits, audit, and immediate revocation |
| Enterprise multi-console redundancy in v1 | The initial release serves a single small-show operator and rig |
| Official macOS or Linux support in v1 | Windows qualification is the initial release constraint |

## MIDI Hardware Decision and Evidence Gates

- **MIDI-HW-01 - RESOLVED 2026-07-19**: Akai MIDImix, Novation Launch Control XL Mk2, and Worlde EasyControl 9 together are the selected Phase 6 physical acceptance set for generic MIDI Note/CC learn and soft-takeover qualification. The documentation gate is resolved by review of the immutable user-supplied manuals `Akai-MIDImix-UserGuide-v1.0.pdf`, `launch_control_xl_programmer_s_reference_guide.pdf`, `Novation-Launch Control XL GSG v2.pdf`, and `Worlde-EasyControl-9-UserManual.pdf`. Selection and manual evidence do not establish compatibility or support.
- **MIDI-HW-02 - OPEN**: Each selected-set member requires independent physical Windows acceptance before any named compatibility or support claim. The evidence must identify the exact hardware revision, firmware, Windows version, and GOLC build and must verify enumeration/hot plug, raw Note/CC behavior, ranges and button semantics, bank/template identity, reconnect, saved mappings, conflicts, and PLAY-05 soft takeover. Device-specific profiles or feedback additionally require applicable output/resync evidence and remain v1.x work under EXTN-04.

MIDI-HW-01 and MIDI-HW-02 are documentation gate labels, not counted feature requirements or Traceability rows. They create no Phase 1 catalog entry, requirement, plan, Linear mapping, or implementation work; the release catalog and dynamic traceability set remain unchanged at 84/84.

## Definition of Done

A v1 requirement is complete only when its implementation is committed, automated verification passes, relevant manual or hardware checks are recorded, the requirement-to-phase and requirement-to-Linear mappings are current, and no unresolved release-blocking finding contradicts the requirement.

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| CONF-01 | Phase 1 | Complete |
| CONF-02 | Phase 1 | Complete |
| CONF-03 | Phase 1 | Complete |
| CONF-04 | Phase 1 | Complete |
| FIXT-01 | Phase 2 | Complete |
| FIXT-02 | Phase 2 | Complete |
| FIXT-03 | Phase 2 | Complete |
| FIXT-04 | Phase 2 | Complete |
| FIXT-05 | Phase 2 | Complete |
| FIXT-06 | Phase 2 | Complete |
| POOL-01 | Phase 2 | Complete |
| POOL-02 | Phase 2 | Complete |
| POOL-03 | Phase 2 | Complete |
| POOL-04 | Phase 2 | Complete |
| POOL-05 | Phase 2 | Complete |
| POOL-06 | Phase 2 | Complete |
| POOL-07 | Phase 2 | Complete |
| POOL-08 | Phase 2 | Complete |
| PROG-01 | Phase 3 | Complete |
| PROG-02 | Phase 3 | Complete |
| PROG-03 | Phase 3 | Complete |
| PROG-04 | Phase 3 | Complete |
| PROG-05 | Phase 3 | Complete |
| PROG-06 | Phase 3 | Complete |
| PROG-07 | Phase 3 | Complete |
| SCEN-01 | Phase 3 | Complete |
| SCEN-02 | Phase 3 | Complete |
| SCEN-03 | Phase 3 | Complete |
| SCEN-04 | Phase 3 | Complete |
| SCEN-05 | Phase 3 | Complete |
| SCEN-06 | Phase 3 | Complete |
| SCEN-07 | Phase 3 | Complete |
| SCEN-08 | Phase 3 | Complete |
| SCEN-09 | Phase 3 | Complete |
| PLAY-01 | Phase 6 | Pending |
| PLAY-02 | Phase 6 | Pending |
| PLAY-03 | Phase 6 | Complete |
| PLAY-04 | Phase 6 | Pending |
| PLAY-05 | Phase 6 | Pending |
| PLAY-06 | Phase 6 | Pending |
| PLAY-07 | Phase 6 | Pending |
| PLAY-08 | Phase 6 | Pending |
| PLAY-09 | Phase 6 | Pending |
| ARTN-01 | Phase 4 | Complete |
| ARTN-02 | Phase 4 | Complete |
| ARTN-03 | Phase 4 | Complete |
| ARTN-04 | Phase 4 | Complete |
| ARTN-05 | Phase 4 | Complete |
| ARTN-06 | Phase 4 | Complete |
| SHOW-01 | Phase 5 | Complete |
| SHOW-02 | Phase 5 | Complete |
| SHOW-03 | Phase 5 | Complete |
| SHOW-04 | Phase 5 | Complete |
| SHOW-05 | Phase 5 | Complete |
| SHOW-06 | Phase 5 | Complete |
| API-01 | Phase 7 | Pending |
| API-02 | Phase 7 | Pending |
| API-03 | Phase 7 | Pending |
| API-04 | Phase 7 | Pending |
| API-05 | Phase 7 | Pending |
| API-06 | Phase 7 | Pending |
| SCRP-01 | Phase 8 | Pending |
| SCRP-02 | Phase 8 | Pending |
| SCRP-03 | Phase 8 | Pending |
| SCRP-04 | Phase 8 | Pending |
| SCRP-05 | Phase 8 | Pending |
| SCRP-06 | Phase 8 | Pending |
| LLM-01 | Phase 9 | Pending |
| LLM-02 | Phase 9 | Pending |
| LLM-03 | Phase 9 | Pending |
| LLM-04 | Phase 9 | Pending |
| LLM-05 | Phase 9 | Pending |
| LLM-06 | Phase 9 | Pending |
| LLM-07 | Phase 9 | Pending |
| LLM-08 | Phase 9 | Pending |
| LLM-09 | Phase 9 | Pending |
| WIN-01 | Phase 10 | Pending |
| WIN-02 | Phase 10 | Pending |
| WIN-03 | Phase 10 | Pending |
| WIN-04 | Phase 10 | Pending |
| LINR-01 | Phase 1 | Complete |
| LINR-02 | Phase 1 | Complete |
| LINR-03 | Phase 1 | Complete |
| LINR-04 | Phase 1 | Complete |

**Coverage:**

- v1 requirements: 84
- Mapped to phases: 84
- Unmapped: 0

---
*Requirements defined: 2026-07-17*
*Last updated: 2026-07-21 after Phase 1 gap-closure: LINR-01/LINR-02 checked off to match delivered internal/trace/catalog implementation; LINR-03/LINR-04 certified Complete against CR-01/CR-02 resolution (plans 01-30/01-31)*
