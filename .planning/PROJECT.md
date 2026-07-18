# GOLC

## What This Is

GOLC is a modern lighting-control application for operators of small live shows, built in Go with a Wails desktop interface and a cross-platform architecture. Its first supported release targets Windows. It combines a fast, modular show-authoring workflow with TypeScript scripting, autonomous LLM control, and a well-documented API so people, scripts, external programs, and AI agents can all create and operate fixture patches, scenes, chases, and show playback through the same system.

The first release will output Art-Net and support complete show authoring and playback. Additional lighting protocols and larger-scale console capabilities can be added after the core workflow and extension model are proven.

## Core Value

An operator can author a modular show once, adapt its fixture pools to different deployments in one or two actions, and hand a simple controller surface to another person for reliable playback.

## Requirements

### Validated

(None yet — ship to validate)

### Active

- [ ] Operators can patch fixtures, organize controllable attributes, create looks/scenes and chases, play them back, save a show, and restore it later.
- [ ] Fixture definitions are human-readable YAML files validated against a versioned schema and compiled into a canonical typed model before use.
- [ ] Users can import fixture definitions from Open Fixture Library and create, edit, validate, and share custom YAML fixture definitions.
- [ ] Show files model reusable fixture pools independently from a deployment's concrete fixture count and addresses.
- [ ] Users can configure how fixture-pool updates propagate through dependents; the default workflow presents an impact review before applying changes.
- [ ] Users can replace fixture models by mapping shared semantic capabilities and reviewing unsupported or incompatible capabilities before committing the deployment change.
- [ ] Scenes can loop for a configured number of musical bars synchronized to a show-wide BPM.
- [ ] Users can combine and swap color themes, chases, and motion presets within tempo-aware scenes using configurable blending transitions.
- [ ] Keyboard and on-screen playback expose the complete operator workflow without requiring a MIDI controller.
- [ ] Users can prepare a constrained MIDI playback surface that another operator can learn and use quickly without exposing the full authoring interface.
- [ ] The application sends reliable, observable Art-Net output suitable for running a small live show.
- [ ] The desktop UI provides a modern, efficient programming and playback workflow that avoids the setup friction and clunky interaction patterns of QLC+.
- [ ] Users can create, edit, run, and debug TypeScript scripts that interact with the supported application and show-control capabilities.
- [ ] Users can connect common hosted or local LLMs through an open-source, provider-neutral integration layer.
- [ ] An LLM can create or refine fixture definitions and autonomously use the program to patch fixtures, program scenes and chases, and control playback.
- [ ] LLM actions are validated, observable, auditable, and subject to immediate operator override even when autonomous control is enabled.
- [ ] External programs and LLM agents can inspect and control the application through a stable, versioned, well-documented API.
- [ ] UI actions, TypeScript scripts, API clients, and LLM tools share a typed application command model so all control surfaces expose consistent behavior.
- [ ] The v1 application installs, runs, saves, restores, and outputs Art-Net reliably on supported Windows systems.
- [ ] Project requirements, roadmap phases, implementation work, and delivery status are tracked in Linear from the start with durable links back to repository planning artifacts.
- [ ] Developer tooling, application defaults, runtime configuration, schemas, generation, validation, build, test, and packaging settings are centralized behind one documented project configuration entrypoint and separated into clear concerns.

### Out of Scope

- Lighting protocols beyond Art-Net — deferred until the output abstraction and Art-Net implementation are proven in real use.
- Enterprise-scale multi-user, distributed, or redundant console operation — v1 focuses on one operator running a small show.
- Reproducing every feature of a high-end professional lighting console — workflow speed, scripting, interoperability, and AI-native control take priority.
- A browser-only or native mobile control application — the initial product is a cross-platform Wails desktop application; remote clients can use the API later.
- Official macOS and Linux support in v1 — preserve portability in the architecture, but qualify and support Windows first.
- Cross-show module import with optional source synchronization — useful for sharing songs, playback pages, and programming collections, but lower priority than modular deployment within one show; target v1.x.
- Proprietary AI orchestration tied to a single model provider — the integration must support common hosted providers and local models through an open-source abstraction.

## Context

- The project is motivated by QLC+: its workflow feels clunky, show setup takes too long, and it does not provide the desired scripting capability.
- The first users are operators of clubs, churches, schools, community venues, and comparable small live shows rather than enterprise productions or large touring rigs.
- The conventional lighting workflow is the v1 proof point: patch fixtures, build scenes and chases, play them back reliably, and persist the show.
- The primary workflow is front-loaded show authoring followed by repeated deployment. A show should be reusable with all or a subset of available fixtures, and pool-size changes should update dependents without rebuilding programming manually.
- Fixture-pool propagation behavior is configurable. The safe default is a review screen showing affected programming, warnings, and errors before applying a change.
- Compatible fixture substitution is semantic rather than channel-number based. Shared intensity, color, position, beam, and other capabilities can be mapped; unsupported behavior is surfaced for review and never approximated silently.
- A scene is a tempo-aware looping performance container spanning a configured number of bars. It can combine independently swappable color themes, chases, and motion presets, with blending behavior controlling transitions between combinations.
- A knowledgeable author prepares the show and MIDI surface; a less-experienced operator should then be able to control the rig quickly from the assigned physical controls.
- The initial MIDI controller is not yet selected. Hardware-specific controller integration and acceptance criteria are blocked until the user identifies the target device; generic MIDI abstractions can be designed earlier.
- Keyboard and on-screen controls must provide the full playback workflow while MIDI hardware remains undecided and after MIDI support is added.
- TypeScript is a first-class automation surface, not an incidental plugin format. Scripts should use the same domain capabilities available to the UI and API.
- LLM support serves two distinct jobs: authoring fixture definitions and operating the application to create or run show content.
- Full autonomous LLM operation is an intended capability. The architecture must therefore separate model interpretation from deterministic command validation and execution, retain an audit trail, and preserve an immediate manual override path.
- The public API is a product surface. It should be designed for external software and agent use, versioned deliberately, documented with examples, and testable independently of the desktop UI.
- Linear is the project-delivery system of record from initialization onward. Planning artifacts should retain stable identifiers and map predictably to Linear projects, milestones, and issues without making offline repository context dependent on Linear availability.
- Fixture source files are intended to be readable, reviewable, portable, and suitable for hand editing or AI generation. YAML is the authoring format; the runtime never consumes unvalidated YAML directly.
- Project configuration covers both development and application concerns. It should be centralized for discoverability while keeping each concern logically separated and independently validatable.

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
- **MIDI hardware**: Do not finalize or claim device-specific controller support until the target controller is selected — controller selection is a blocker for that phase's hardware acceptance tests.
- **Safe structural edits**: Pool resizing and fixture substitution default to previewing a deterministic impact plan before commit — modular reuse must not silently corrupt or reinterpret show programming.
- **Musical timing**: Tempo-aware scenes derive timing from a global BPM and explicit bar structure — scene playback must remain deterministic and independent of UI, script, or LLM latency.

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Build the desktop application in Go with Wails | Provides the requested Go core and cross-platform desktop UI model | — Pending |
| Target small live shows first | Keeps v1 focused on a complete, practical workflow for a single operator and modest rig | — Pending |
| Define v1 success through complete show programming and Art-Net playback | A reliable conventional workflow must exist before AI and integrations can be trusted with it | — Pending |
| Support full LLM autonomy | The LLM is intended to operate the whole program, not merely suggest content | — Pending |
| Make TypeScript the user scripting language | Provides a familiar, capable language for programmable show behavior | — Pending |
| Treat the API as a first-class, versioned product surface | External programs and LLMs need durable interoperability rather than UI automation | — Pending |
| Route UI, scripts, API, and LLM tools through a shared typed command model | Preserves behavioral consistency, testability, and control boundaries across every interface | — Pending |
| Implement Art-Net first behind a protocol abstraction | Delivers the initial real-world output path without blocking later protocol support | — Pending |
| Use Linear from project inception | Keeps requirements, planned work, and delivery status visible and traceable from the first implementation phase | — Pending |
| Store fixture definitions as schema-validated YAML | YAML fits nested fixture modes, channels, capabilities, and ranges better than TOML while remaining friendly to people and LLMs | — Pending |
| Import OFL and support custom fixture definitions | Combines broad ecosystem coverage with an escape hatch for missing or venue-specific fixtures | — Pending |
| Make show files modular around reusable fixture pools | The same authored show must adapt quickly to different quantities and deployments of compatible fixtures | — Pending |
| Default pool and fixture-substitution changes to review-before-apply | Structural edits can affect the whole show and require an understandable impact diff before commit | — Pending |
| Map replacement fixtures by semantic capability | Shows should survive deployment changes across compatible fixture models without relying on raw channel positions | — Pending |
| Model scenes as bar-based loops synchronized to global BPM | Matches the primary performance workflow and makes color, chase, and motion programming musically reusable | — Pending |
| Make color themes, chases, and motion presets independently swappable with blending | Enables fast variation within a prepared show without rebuilding scenes | — Pending |
| Provide complete keyboard and on-screen playback | The application must be fully operable before and independently of the selected MIDI hardware | — Pending |
| Defer cross-show module synchronization to v1.x | Modular deployment inside a show delivers the primary value first | — Pending |
| Support Windows first | Concentrates v1 packaging, timing, networking, and hardware qualification on the user's required platform | — Pending |
| Centralize project configuration while separating concerns | Makes setup and operation discoverable without collapsing unrelated configuration into one unmaintainable file | — Pending |

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
*Last updated: 2026-07-17 after initialization*
