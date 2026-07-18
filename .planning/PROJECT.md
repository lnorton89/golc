# GOLC

## What This Is

GOLC is a modern, cross-platform lighting-control application for operators of small live shows, built in Go with a Wails desktop interface. It combines a fast conventional programming workflow with TypeScript scripting, autonomous LLM control, and a well-documented API so people, scripts, external programs, and AI agents can all create and operate fixture patches, scenes, chases, and show playback through the same system.

The first release will output Art-Net and support complete show authoring and playback. Additional lighting protocols and larger-scale console capabilities can be added after the core workflow and extension model are proven.

## Core Value

A small-show operator can go from fixture patch to reliable live playback dramatically faster than in QLC+, without sacrificing the power to automate or extend the show.

## Requirements

### Validated

(None yet — ship to validate)

### Active

- [ ] Operators can patch fixtures, organize controllable attributes, create looks/scenes and chases, play them back, save a show, and restore it later.
- [ ] The application sends reliable, observable Art-Net output suitable for running a small live show.
- [ ] The desktop UI provides a modern, efficient programming and playback workflow that avoids the setup friction and clunky interaction patterns of QLC+.
- [ ] Users can create, edit, run, and debug TypeScript scripts that interact with the supported application and show-control capabilities.
- [ ] Users can connect common hosted or local LLMs through an open-source, provider-neutral integration layer.
- [ ] An LLM can create or refine fixture definitions and autonomously use the program to patch fixtures, program scenes and chases, and control playback.
- [ ] LLM actions are validated, observable, auditable, and subject to immediate operator override even when autonomous control is enabled.
- [ ] External programs and LLM agents can inspect and control the application through a stable, versioned, well-documented API.
- [ ] UI actions, TypeScript scripts, API clients, and LLM tools share a typed application command model so all control surfaces expose consistent behavior.
- [ ] The application runs on the desktop platforms supported by Go and Wails, with platform-specific differences isolated from show behavior.

### Out of Scope

- Lighting protocols beyond Art-Net — deferred until the output abstraction and Art-Net implementation are proven in real use.
- Enterprise-scale multi-user, distributed, or redundant console operation — v1 focuses on one operator running a small show.
- Reproducing every feature of a high-end professional lighting console — workflow speed, scripting, interoperability, and AI-native control take priority.
- A browser-only or native mobile control application — the initial product is a cross-platform Wails desktop application; remote clients can use the API later.
- Proprietary AI orchestration tied to a single model provider — the integration must support common hosted providers and local models through an open-source abstraction.

## Context

- The project is motivated by QLC+: its workflow feels clunky, show setup takes too long, and it does not provide the desired scripting capability.
- The first users are operators of clubs, churches, schools, community venues, and comparable small live shows rather than enterprise productions or large touring rigs.
- The conventional lighting workflow is the v1 proof point: patch fixtures, build scenes and chases, play them back reliably, and persist the show.
- TypeScript is a first-class automation surface, not an incidental plugin format. Scripts should use the same domain capabilities available to the UI and API.
- LLM support serves two distinct jobs: authoring fixture definitions and operating the application to create or run show content.
- Full autonomous LLM operation is an intended capability. The architecture must therefore separate model interpretation from deterministic command validation and execution, retain an audit trail, and preserve an immediate manual override path.
- The public API is a product surface. It should be designed for external software and agent use, versioned deliberately, documented with examples, and testable independently of the desktop UI.

## Constraints

- **Application stack**: Go with Wails — required by the chosen cross-platform desktop architecture.
- **Scripting**: TypeScript — required for user-authored automation and extensibility.
- **Initial protocol**: Art-Net — all other lighting-output protocols are deferred beyond v1.
- **AI portability**: Use an open-source provider-neutral wrapper that supports common hosted providers and local models — users must not be locked to one LLM vendor.
- **Live reliability**: DMX/Art-Net output and playback timing cannot depend on UI rendering, network-bound LLM inference, or script responsiveness — show output must remain deterministic under load or component failure.
- **Control consistency**: UI, scripts, API calls, and LLM operations must converge on shared domain commands and state — otherwise automation and interoperation will become incomplete or unsafe.
- **Autonomy safety**: Autonomous AI control must remain observable and interruptible by the operator — live lighting changes need a dependable human override even when confirmation is not required for each action.

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
