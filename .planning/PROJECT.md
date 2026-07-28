# GOLC

## What This Is

GOLC is a modern lighting-control application for club/DJ operators running small live shows, built in Go with a Wails desktop interface and a cross-platform architecture. Its first supported release targets Windows. It combines a fast, modular show-authoring workflow with TypeScript scripting, autonomous LLM control, and a well-documented API so people, scripts, external programs, and AI agents can all create and operate fixture patches, scenes, chases, and show playback through the same system.

v1.0 (shipped 2026-07-27) delivers the deterministic core: modular fixture
authoring, tempo-aware show programming, real Art-Net output, durable show
storage, the full Wails authoring/operator workflow with generic MIDI control, a
versioned external API, and isolated TypeScript scripting. LLM-driven authoring
and autonomy, Windows release qualification, and telemetry/crash reporting are
scoped for the next milestone. Additional lighting protocols and larger-scale
console capabilities can be added after the core workflow and extension model are
proven.

## Core Value

An operator can author a modular show once, adapt its fixture pools to different deployments in one or two actions, and hand a simple controller surface to another person for reliable playback.

## Requirements

### Validated

- [x] Project requirements, roadmap phases, implementation work, and delivery status are tracked in Linear from the start with durable links back to repository planning artifacts. — *Validated in Phase 1: Offline Foundation and Delivery Traceability (2026-07-21)*
- [x] Developer tooling, application defaults, runtime configuration, schemas, generation, validation, build, test, and packaging settings are centralized behind one documented project configuration entrypoint and separated into clear concerns. — *Validated in Phase 1: Offline Foundation and Delivery Traceability (2026-07-21)*
- [x] Fixture definitions are human-readable YAML files validated against a versioned schema and compiled into a canonical typed model before use. — *Validated in Phase 2: Modular Fixtures and Deployments (2026-07-21)*
- [x] Users can import fixture definitions from Open Fixture Library and create, edit, validate, and share custom YAML fixture definitions. — *Validated in Phase 2: Modular Fixtures and Deployments (2026-07-21)*
- [x] Show files model reusable fixture pools independently from a deployment's concrete fixture count and addresses. — *Validated in Phase 2: Modular Fixtures and Deployments (2026-07-21)*
- [x] Users can configure how fixture-pool updates propagate through dependents; the default workflow presents an impact review before applying changes. — *Validated in Phase 2: Modular Fixtures and Deployments (2026-07-21)*
- [x] Users can replace fixture models by mapping shared semantic capabilities and reviewing unsupported or incompatible capabilities before committing the deployment change. — *Validated in Phase 2: Modular Fixtures and Deployments (2026-07-21)*
- [x] Scenes can loop for a configured number of musical bars synchronized to a show-wide BPM. — *Validated in Phase 3: Deterministic Show Programming and Playback (2026-07-21)*
- [x] Users can combine and swap color themes, chases, and motion presets within tempo-aware scenes using configurable blending transitions. — *Validated in Phase 3: Deterministic Show Programming and Playback (2026-07-21)*
- [x] The application sends reliable, observable Art-Net output suitable for running a small live show. — *Validated in Phase 4: Observable Art-Net Live Output (2026-07-23), including gap-closure plans 04-08/04-09 for per-universe status values and pinned-interface degraded status*
- [x] Operators can patch fixtures, organize controllable attributes, create looks/scenes and chases, play them back, save a show, and restore it later. — *Validated in Phase 5: Durable Shows and Recovery (2026-07-23): SQLite-backed `.golc` store, rotating recovery points genuinely reachable from an interrupted session (post-review structural fix), verified-backup schema migration, and integrity diagnostics/export*
- [x] External programs can query and invoke a documented, coverage-gated subset of public capabilities — configuration and show inspection, fixture-pool creation, scoped API-key lifecycle, atomic multi-command batches, and revisioned change events — through a versioned HTTP API with a published OpenAPI contract, typed errors, and a compatibility/deprecation policy; every not-yet-exposed public route is explicitly named and deferred (tracked as `EXTN-05`) rather than silently unmapped. — *Validated in Phase 7: Versioned External Control API (2026-07-25), after two gap-closure rounds: fixed a live-reproduced SSE data-loss bug (batched mutations sharing one revision silently dropped replay events) and closed the mutation audit trail for all of `/v1/batch`'s failure paths (pre-flight and locked-section, 9/9 traced)*
- [x] Keyboard and on-screen playback expose the complete operator workflow without requiring a MIDI controller. — *Validated in Phase 6: Wails Authoring and Operator Surface (2026-07-24)*
- [x] Users can prepare a constrained MIDI playback surface that another operator can learn and use quickly without exposing the full authoring interface. — *Validated in Phase 6: Wails Authoring and Operator Surface (2026-07-24): named per-surface assignment, cross-to-catch soft-takeover MIDI learn, and server-side visible-but-locked enforcement*
- [x] Users can create, edit, run, and debug TypeScript scripts that interact with the supported application and show-control capabilities. — *Validated in Phase 8: Isolated TypeScript Automation (2026-07-27): generated typed SDK, zero-permission Deno host, capability/rate/deadline enforcement backed by a real Windows Job Object, and a real CDP breakpoint/step debugger*

### Active

- [ ] The desktop UI provides a modern, efficient programming and playback workflow that avoids the setup friction and clunky interaction patterns of QLC+. — v1.0 delivered the UI; the comparative claim against QLC+ is unvalidated until a First Real Show session runs (gated on the front-door-UI phase per `POST-PHASE-8-PLAN.md`).
- [ ] Users can connect common hosted or local LLMs through an open-source, provider-neutral integration layer.
- [ ] An LLM can create or refine fixture definitions and autonomously use the program to patch fixtures, program scenes and chases, and control playback.
- [ ] LLM actions are validated, observable, auditable, and subject to immediate operator override even when autonomous control is enabled.
- [ ] LLM agents can inspect and control the application through the same typed command model external programs use (Phase 7 delivered the external-program-facing subset above; remaining show-domain/Art-Net-runtime routes and LLM-specific access are tracked separately under `EXTN-05` and the AI/bounded-autonomy phase).
- [ ] UI actions, TypeScript scripts, API clients, and LLM tools share a typed application command model so all control surfaces expose consistent behavior. (UI, scripts, and the API already converge on the same command registry as of v1.0; LLM tools are the remaining unproven surface.)
- [ ] The v1 application installs, runs, saves, restores, and outputs Art-Net reliably on supported Windows systems. (Currently dev-machine `mage Run` only; Phase 11 owns packaging and qualification.)
- [ ] A new operator can go from a fresh checkout to a patched fixture and a scene on screen using only the UI, with Guided First Show onboarding as the on-ramp. (Phase 9; Fixture Library workspace is still a `ComingSoon` stub, show open/new/switch and onboarding are not yet built.)

### Out of Scope

- Lighting protocols beyond Art-Net — deferred until the output abstraction and Art-Net implementation are proven in real use.
- Enterprise-scale multi-user, distributed, or redundant console operation — v1 focuses on one operator running a small show.
- Reproducing every feature of a high-end professional lighting console — workflow speed, scripting, interoperability, and AI-native control take priority.
- A browser-only or native mobile control application — the initial product is a cross-platform Wails desktop application; remote clients can use the API later.
- Official macOS and Linux support in v1 — preserve portability in the architecture, but qualify and support Windows first.
- Cross-show module import with optional source synchronization — useful for sharing songs, playback pages, and programming collections, but lower priority than modular deployment within one show; target v1.x.
- Proprietary AI orchestration tied to a single model provider — the integration must support common hosted providers and local models through an open-source abstraction.
- Cue-stack/timed-fade playback — the shipped v1.0 scene/BPM/layer model is the proven workflow; not an open question, not a deferred feature to revisit without a new signal (owner decision 2026-07-25, see `POST-PHASE-8-PLAN.md`).
- Church/school/community-venue positioning — aspirational at project init, never validated; narrowed to club/DJ at v1.0 close (owner decision 2026-07-25).

## Context

- The project is motivated by QLC+: its workflow feels clunky, show setup takes too long, and it does not provide the desired scripting capability.
- The first users are club/DJ operators running small live shows rather than enterprise productions or large touring rigs. (Narrowed 2026-07-27 at v1.0 close from earlier church/school/community-venue language, which was aspirational and never validated — the shipped scene/BPM/layer model fits club/DJ use.)
- The conventional lighting workflow is the v1 proof point: patch fixtures, build scenes and chases, play them back reliably, and persist the show.
- The primary workflow is front-loaded show authoring followed by repeated deployment. A show should be reusable with all or a subset of available fixtures, and pool-size changes should update dependents without rebuilding programming manually.
- Fixture-pool propagation behavior is configurable. The safe default is a review screen showing affected programming, warnings, and errors before applying a change.
- Compatible fixture substitution is semantic rather than channel-number based. Shared intensity, color, position, beam, and other capabilities can be mapped; unsupported behavior is surfaced for review and never approximated silently.
- A scene is a tempo-aware looping performance container spanning a configured number of bars. It can combine independently swappable color themes, chases, and motion presets, with blending behavior controlling transitions between combinations.
- Global BPM can be entered numerically or established through tap tempo.
- Only one scene is active at a time. Its color, chase, and motion layers can be combined, enabled, disabled, and replaced independently.
- Scene and layer changes take effect immediately by default. Blending behavior comes from reusable transition presets rather than ad hoc timing on every control.
- When global BPM changes, the operator can choose whether the active loop preserves musical position or restarts.
- A knowledgeable author prepares the show and MIDI surface; a less-experienced operator should then be able to control the rig quickly from the assigned physical controls.
- Akai MIDImix, Novation Launch Control XL Mk2, and Worlde EasyControl 9 together are the selected Phase 6 physical acceptance set for generic MIDI Note/CC learn and soft takeover. Selection does not establish support; each device must independently pass MIDI-HW-02 for its exact hardware revision, firmware, Windows version, and GOLC build before a named compatibility or support claim.
- Keyboard and on-screen controls must provide the full playback workflow while MIDI hardware remains undecided and after MIDI support is added.
- `Revoke Automation` is the independent operator safety action: it blocks AI and scripts, cancels their queued actions, freezes the current look, and returns control to manual operation. Blackout remains a separate immediate intensity control.
- TypeScript is a first-class automation surface, not an incidental plugin format. Scripts should use the same domain capabilities available to the UI and API.
- LLM support serves two distinct jobs: authoring fixture definitions and operating the application to create or run show content.
- Full autonomous LLM operation is an intended capability. The architecture must therefore separate model interpretation from deterministic command validation and execution, retain an audit trail, and preserve an immediate manual override path.
- The public API is a product surface. It should be designed for external software and agent use, versioned deliberately, documented with examples, and testable independently of the desktop UI.
- Linear is the project-delivery system of record from initialization onward. Planning artifacts should retain stable identifiers and map predictably to Linear projects, milestones, and issues without making offline repository context dependent on Linear availability.
- Fixture source files are intended to be readable, reviewable, portable, and suitable for hand editing or AI generation. YAML is the authoring format; the runtime never consumes unvalidated YAML directly.
- Project configuration covers both development and application concerns. It should be centralized for discoverability while keeping each concern logically separated and independently validatable.

## Current State

**Shipped: v1.0 (2026-07-27), Phases 1-8, 99 plans, ~66.5k LOC (Go + TypeScript).**

The deterministic lighting-console core is complete and verified: offline-safe
project configuration and Linear traceability; a semantic fixture catalog (OFL
import + custom YAML) with reviewed pool/deployment impact analysis; tempo-aware
show programming (themes, chases, motion presets, bar-loop scenes) compiled into a
deterministic, next-bar-boundary playback engine; real Art-Net 4 output verified
against Wireshark, an independent OLA simulator, and physical hardware; durable
SQLite-backed show storage with recovery points and verified-backup migrations; a
full Wails desktop authoring/operator surface with constrained operator surfaces,
generic MIDI Note/CC learn with soft takeover, and daemon-resident safety overrides
(Blackout, Stop/Release-All, Revoke Automation) that never wait on UI/script/API/LLM
completion; a versioned, audited, OpenAPI-documented external HTTP API with SSE
events and atomic batches; and isolated, capability-limited TypeScript scripting
with a generated typed SDK, Windows Job Object sandboxing, and a real CDP
breakpoint/step debugger.

Known gaps carried into the next milestone (see `.planning/POST-PHASE-8-PLAN.md`,
owner decisions recorded 2026-07-25):

- The Fixture Library workspace is still a `ComingSoon` stub; show open/new/switch
  and Guided First Show onboarding are not yet built through the UI (CLI-only).
- No "First Real Show" validation session has run yet — gated on the above.
- Doc/state drift (README phase-status wording, STATE.md velocity metrics, a
  `internal/trace` data race, two catalog test failures) is tracked as a short
  hygiene phase, not yet executed.
- `MIDI-HW-02` (per-device physical Windows acceptance evidence) remains open.

## Next Milestone Goals

Scoped in `.planning/ROADMAP.md`, requirements defined in `.planning/REQUIREMENTS.md`.
The hygiene phase from `.planning/POST-PHASE-8-PLAN.md` section 1 was completed
inline during v1.0 milestone close (2026-07-27, not as a separate roadmap phase).
The front-door-UI phase from section 2 was inserted as **Phase 9**, pushing the
originally-numbered Phase 9/10/11 to 10/11/12:

1. **Phase 9 — Front-Door UI Completion**: Fixture Library workspace, show
   open/new/switch, and Guided First Show onboarding — closes the last UI-only
   gaps so no CLI is required for the happy path.
2. **Phase 10 — Provider-Neutral AI and Bounded Autonomy**: LLM-backed fixture
   authoring and explicitly armed, time-bounded live control with immediate
   operator override.
3. **Phase 11 — Windows Release Qualification**: packaged, supervised, self-contained
   Windows install with measured timing/recovery/hardware evidence.
4. **Phase 12 — Telemetry, Usage Statistics, and Auto Crash Submission Pipeline**:
   opt-in anonymized telemetry and automatic crash capture.

Phase 9 was numbered as a plain integer, not GSD's default decimal insertion
(e.g. `8.1`) — `internal/trace/catalog`'s phase-directory grammar and
`### Phase N:` heading regex only match two-digit integers, so a decimal phase
would have been invisible to Linear traceability (no `phase:08.1` entity, no
LINR-01/02 coverage). Next: `/gsd-discuss-phase 9` then `/gsd-plan-phase 9`.

## Constraints

- **Application stack**: Go with Wails — required by the chosen cross-platform desktop architecture.
- **Initial platform**: Windows only for v1 qualification and support — other desktop platforms are deferred even though portability remains an architectural goal.
- **Scripting**: TypeScript — required for user-authored automation and extensibility.
- **Fixture source format**: Use a strict YAML 1.2 subset with schema validation, duplicate-key rejection, explicit schema versioning, and deterministic normalization — fixture files must remain approachable without introducing ambiguous runtime behavior.
- **Fixture ecosystem**: Support Open Fixture Library import plus first-class custom definitions — imported definitions must pass through GOLC's canonical validation and pinning pipeline.
- **Initial protocol**: Art-Net — all other lighting-output protocols are deferred beyond v1.
- **AI portability**: Use an open-source provider-neutral wrapper that supports common hosted providers and local models — users must not be locked to one LLM vendor.
- **Live reliability**: DMX/Art-Net output and playback timing cannot depend on UI rendering, network-bound LLM inference, or script responsiveness — show output must remain deterministic under load or component failure.
- **Control consistency**: UI, scripts, API calls, and LLM operations must converge on shared domain commands and state — otherwise automation and interoperation will become incomplete or unsafe.
- **Autonomy safety**: Autonomous AI control must remain observable and interruptible by the operator — live lighting changes need a dependable human override even when confirmation is not required for each action.
- **Project tracking**: Use Linear from the start — requirements, roadmap phases, and implementation issues need explicit repository-to-Linear traceability.
- **Developer experience**: Centralize project configuration behind one documented root entrypoint with logically separated subconfiguration — contributors and automation should not need to discover scattered sources of truth.
- **MIDI hardware**: Akai MIDImix, Novation Launch Control XL Mk2, and Worlde EasyControl 9 are the selected Phase 6 physical acceptance set; selection does not establish support. Each device must independently pass MIDI-HW-02 for its exact hardware revision, firmware, Windows version, and GOLC build before a named compatibility or support claim, and device-specific profiles or feedback remain v1.x work under EXTN-04.
- **Safe structural edits**: Pool resizing and fixture substitution default to previewing a deterministic impact plan before commit — modular reuse must not silently corrupt or reinterpret show programming.
- **Musical timing**: Tempo-aware scenes derive timing from a global BPM and explicit bar structure — scene playback must remain deterministic and independent of UI, script, or LLM latency.
- **Automation override**: Revoke Automation must not depend on an AI provider, script runtime, or queued application command completing — it is a local priority control path distinct from blackout.

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Build the desktop application in Go with Wails | Provides the requested Go core and cross-platform desktop UI model | ✓ Good — proven through v1.0's full authoring/operator/safety-hotkey surface |
| Target small live shows first | Keeps v1 focused on a complete, practical workflow for a single operator and modest rig | ✓ Good — narrowed further to club/DJ at v1.0 close |
| Define v1 success through complete show programming and Art-Net playback | A reliable conventional workflow must exist before AI and integrations can be trusted with it | ✓ Good — v1.0 shipped this core before Phase 10 (AI) begins |
| Support full LLM autonomy | The LLM is intended to operate the whole program, not merely suggest content | — Pending (Phase 10, next milestone) |
| Make TypeScript the user scripting language | Provides a familiar, capable language for programmable show behavior | ✓ Good — validated in Phase 8 with a real sandbox and debugger |
| Treat the API as a first-class, versioned product surface | External programs and LLMs need durable interoperability rather than UI automation | ✓ Good — validated in Phase 7 with OpenAPI contract, SSE, audit trail |
| Route UI, scripts, API, and LLM tools through a shared typed command model | Preserves behavioral consistency, testability, and control boundaries across every interface | ✓ Good for UI/scripts/API (Phases 6-8); LLM tools remain Pending (Phase 10) |
| Implement Art-Net first behind a protocol abstraction | Delivers the initial real-world output path without blocking later protocol support | ✓ Good — verified against simulator and real hardware in Phase 4 |
| Use Linear from project inception | Keeps requirements, planned work, and delivery status visible and traceable from the first implementation phase | ✓ Good — credential-free, idempotent reconciliation proven in Phase 1 |
| Store fixture definitions as schema-validated YAML | YAML fits nested fixture modes, channels, capabilities, and ranges better than TOML while remaining friendly to people and LLMs | ✓ Good — validated in Phase 2 |
| Import OFL and support custom fixture definitions | Combines broad ecosystem coverage with an escape hatch for missing or venue-specific fixtures | ✓ Good — validated in Phase 2 |
| Make show files modular around reusable fixture pools | The same authored show must adapt quickly to different quantities and deployments of compatible fixtures | ✓ Good — validated in Phase 2 |
| Default pool and fixture-substitution changes to review-before-apply | Structural edits can affect the whole show and require an understandable impact diff before commit | ✓ Good — validated in Phase 2 |
| Map replacement fixtures by semantic capability | Shows should survive deployment changes across compatible fixture models without relying on raw channel positions | ✓ Good — validated in Phase 2 |
| Model scenes as bar-based loops synchronized to global BPM | Matches the primary performance workflow and makes color, chase, and motion programming musically reusable | ✓ Good — validated in Phase 3 |
| Make color themes, chases, and motion presets independently swappable with blending | Enables fast variation within a prepared show without rebuilding scenes | ✓ Good — validated in Phase 3 |
| Accept typed BPM and tap tempo | Covers deliberate show setup and quick live tempo adjustment without requiring external clock hardware | ✓ Good — validated in Phase 3 |
| Run one scene at a time with combinable internal layers | Keeps operator state understandable while still allowing independent color, chase, and motion variation | ✓ Good — validated in Phase 3 |
| Apply scene and layer changes immediately by default | Matches the intended responsive playback workflow | ✓ Good — validated in Phase 3 |
| Express blending through reusable presets | Makes transitions consistent, shareable, and quick to assign | ✓ Good — validated in Phase 3 |
| Make tempo-change loop behavior selectable | Different shows may need continuity or a clean restart when BPM changes | ✓ Good — validated in Phase 3 |
| Provide an independent Revoke Automation action | The operator must be able to stop AI and scripts without waiting for them and without forcing blackout | ✓ Good — validated in Phase 6, daemon-resident and independent of UI/script/API/LLM |
| Provide complete keyboard and on-screen playback | The application must be fully operable before and independently of the selected MIDI hardware | ✓ Good — validated in Phase 6 |
| Defer cross-show module synchronization to v1.x | Modular deployment inside a show delivers the primary value first | ✓ Good — still deferred, unchanged |
| Support Windows first | Concentrates v1 packaging, timing, networking, and hardware qualification on the user's required platform | — Pending (Phase 11, next milestone; dev-machine `mage Run` only today) |
| Centralize project configuration while separating concerns | Makes setup and operation discoverable without collapsing unrelated configuration into one unmaintainable file | ✓ Good — validated in Phase 1 |
| Select Akai MIDImix, Novation Launch Control XL Mk2, and Worlde EasyControl 9 together for Phase 6 physical acceptance | The three user-owned controllers can independently qualify generic MIDI learn and soft takeover without turning selection or manual evidence into a support claim | Decided 2026-07-19; MIDI-HW-01 resolved, MIDI-HW-02 remains open per device |
| Close v1.0 at Phase 8, splitting the originally-unified 11-phase "v1" scope into v1.0 (core) and a later milestone (AI/Windows/telemetry) | Ship the proven deterministic core now rather than block release on unstarted AI/Windows/telemetry work | ✓ Good — decided 2026-07-25 (`POST-PHASE-8-PLAN.md`), executed at milestone close 2026-07-27 |
| Narrow target audience from clubs/churches/schools/community venues to club/DJ only | Church/school/community language was aspirational and never validated; the shipped scene/BPM/layer model fits club/DJ use | ✓ Good — decided 2026-07-25, applied 2026-07-27 |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `$gsd-transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `$gsd-complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---
*Last updated: 2026-07-27 after v1.0 milestone (Phases 1-8) completion*
