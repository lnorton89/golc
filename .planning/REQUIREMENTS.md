# Requirements: GOLC (post-v1.0)

**Defined:** 2026-07-17 (original v1 set); **Re-scoped:** 2026-07-27 after v1.0 milestone close
**Core Value:** An operator can author a modular show once, adapt its fixture pools to different deployments in one or two actions, and hand a simple controller surface to another person for reliable playback.

## Scope Note

v1.0 (Phases 1-8: offline foundation, fixtures/pools, deterministic show
programming, Art-Net output, durable storage, Wails authoring/operator surface,
external API, TypeScript scripting) shipped 2026-07-27 — see
`.planning/milestones/v1.0-ROADMAP.md` and `.planning/milestones/v1.0-REQUIREMENTS.md`.

The requirements below (LLM integration/autonomy, Windows release qualification,
telemetry/crash pipeline) were part of the original 2026-07-17 v1 requirements set
and remain scoped to Phases 9-11 of the same roadmap (`.planning/ROADMAP.md`), which
were never archived and are still active. They carry forward as the requirements
for the next milestone.

## User Stories

- As an operator, I can let an LLM author or run show content while retaining an immediate, model-independent way to revoke automation.

## Requirements

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

### Telemetry and Crash Reporting

- [ ] **TELE-01**: A user can explicitly opt into usage/telemetry collection; collection stays off by default and nothing is sent before that opt-in.
- [ ] **TELE-02**: Collected usage/telemetry data is anonymized before it ever leaves the device.
- [ ] **TELE-03**: A crash is automatically captured and submitted for diagnosis without the user having to manually reproduce or describe it.
- [ ] **TELE-04**: Telemetry and crash submission never block or degrade live playback or Art-Net output.

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
- **EXTN-05**: Every remaining public show-domain and Art-Net-runtime capability becomes reachable through the versioned /v1 API using the mechanisms Phase 7 proved (translation seam, scoped auth, serialized mutation pipeline, atomic batch, audit trail, SSE), together with a Wails-versus-HTTP outcome-parity check spanning more than one mutating domain.

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
| Church/school/community-venue positioning | Aspirational at project init, never validated; v1.0 narrowed the target audience to club/DJ operation (see `PROJECT.md`) |

## MIDI Hardware Decision and Evidence Gates

- **MIDI-HW-01 - RESOLVED 2026-07-19**: Akai MIDImix, Novation Launch Control XL Mk2, and Worlde EasyControl 9 together are the selected Phase 6 physical acceptance set for generic MIDI Note/CC learn and soft-takeover qualification. The documentation gate is resolved by review of the immutable user-supplied manuals `Akai-MIDImix-UserGuide-v1.0.pdf`, `launch_control_xl_programmer_s_reference_guide.pdf`, `Novation-Launch Control XL GSG v2.pdf`, and `Worlde-EasyControl-9-UserManual.pdf`. Selection and manual evidence do not establish compatibility or support.
- **MIDI-HW-02 - OPEN**: Each selected-set member requires independent physical Windows acceptance before any named compatibility or support claim. The evidence must identify the exact hardware revision, firmware, Windows version, and GOLC build and must verify enumeration/hot plug, raw Note/CC behavior, ranges and button semantics, bank/template identity, reconnect, saved mappings, conflicts, and PLAY-05 soft takeover. Device-specific profiles or feedback additionally require applicable output/resync evidence and remain v1.x work under EXTN-04.

## Definition of Done

A requirement is complete only when its implementation is committed, automated
verification passes, relevant manual or hardware checks are recorded, the
requirement-to-phase and requirement-to-Linear mappings are current, and no
unresolved release-blocking finding contradicts the requirement.

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
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
| TELE-01 | Phase 11 | Pending |
| TELE-02 | Phase 11 | Pending |
| TELE-03 | Phase 11 | Pending |
| TELE-04 | Phase 11 | Pending |

**Coverage:**

- Remaining requirements: 17
- Mapped to phases: 17
- Unmapped: 0

---
*Requirements originally defined: 2026-07-17*
*Re-scoped 2026-07-27 after v1.0 milestone close: CONF/FIXT/POOL/PROG/SCEN/PLAY/ARTN/SHOW/API/SCRP/LINR (74 requirements) archived complete to `.planning/milestones/v1.0-REQUIREMENTS.md`. LLM/WIN/TELE (17 requirements) carried forward unchanged as the requirements for Phases 9-11, still active in `.planning/ROADMAP.md`.*
