---
gsd_state_version: 1.0
milestone: v1.0
milestone_name: milestone
current_phase: 3
current_phase_name: Deterministic Show Programming and Playback
status: planning
stopped_at: Phase 2 context gathered
last_updated: "2026-07-22T02:02:38.297Z"
last_activity: 2026-07-21
last_activity_desc: Phase 02 complete, transitioned to Phase 3
progress:
  total_phases: 2
  completed_phases: 2
  total_plans: 38
  completed_plans: 38
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-07-17)

**Core value:** An operator can author a modular show once, adapt its fixture pools to different deployments in one or two actions, and hand a simple controller surface to another person for reliable playback.
**Current focus:** Phase 02 — modular-fixtures-and-deployments

## Current Position

Phase: 3 — Deterministic Show Programming and Playback
Plan: Not started
Status: Ready to plan
Last activity: 2026-07-21 — Phase 02 complete, transitioned to Phase 3

Progress: [██░░░░░░░░] 24%

## Performance Metrics

**Velocity:**

- Total plans completed: 38
- Average duration: -
- Total execution time: 0 hours

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 01 | 32 | - | - |
| 02 | 6 | - | - |

**Recent Trend:**

- Last 5 plans: none
- Trend: Not started

| Phase 01 P01 | 7min | 1 tasks | 4 files |
| Phase 01 P12 | 8min | 1 tasks | 0 files |
**Per-Plan Metrics:**

| Plan | Duration | Tasks | Files |
|------|----------|-------|-------|
| Phase 01 P16 | 25min | 1 tasks | 8 files |
| Phase 01 P17 | 35min | 1 tasks | 10 files |
| Phase 01 P02 | 10min | 2 tasks | 9 files |
| Phase 01 P03 | 7min | 1 tasks | 9 files |
| Phase 01 P08 | 16min | 1 tasks | 5 files |

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table. Recent roadmap constraints:

- [Phase 1]: Centralized configuration and credential-free, offline-capable Linear traceability begin before product feature work and remain gates throughout v1.
- [All phases]: UI, persistence, scripts, API, LLM, and Linear never own or block deterministic playback or Art-Net timing.
- [Phase 6]: Akai MIDImix, Novation Launch Control XL Mk2, and Worlde EasyControl 9 are the selected Phase 6 physical acceptance set for generic MIDI Note/CC learn and soft takeover; selection is not support, and MIDI-HW-02 requires independent per-device evidence for the exact hardware revision, firmware, Windows version, and GOLC build before a named claim.
- [Phase 10]: Windows is the only qualified and supported v1 platform; portability is preserved without macOS/Linux release claims.
- [Phase 01]: Acceptance fixtures are data-only and restricted to the three expected TOML files; only the repository-owned root command may be executed. — Prevents untrusted fixture content from becoming executable while preserving a clean-checkout test.
- [Phase 01]: Bootstrap fixture metadata is populated only after hashing a locally built archive, and green acceptance compares raw output bytes. — Locks checksum-before-use and byte-determinism into the first contributor contract.
- [Phase 01]: Bootstrap archives promote as per-tool atomic install units with content-addressed verified download caching; a matching install manifest makes second bootstrap consult no archive source.
- [Phase 01]: Bootstrap hashes go.mod/go.sum around every module operation and hard-fails on mutation, mechanically enforcing D-04 pin immutability.
- [Phase ?]: Routes must belong to a declared scope; MustDeclareScope is a mechanical precondition for every command graph (GOLC_ROUTE_SCOPE_UNDECLARED).
- [Phase ?]: Green acceptance packages the real built golc-project.exe as the checksum-pinned archive payload; bootstrap mode keeps the inert payload.
- [Phase ?]: Go module path corrected to github.com/lnorton89/golc across go.mod and all imports (user correction).
- [Phase ?]: Registry routes cannot contain dash-prefixed words: the quick dispatcher registers route 'test' and strictly accepts only '--quick --scope <name>'; the user-facing command is unchanged.
- [Phase ?]: internal/projectconfig is a pure library: all config CLI routes (inspect/set/explain) self-register from internal/command/config.go, resolving the command<->projectconfig import cycle.
- [Phase ?]: Go test scopes are declared from external test packages via command.MustDeclareScope beside their exact TestScope{PascalName} marker (pattern set by config-local).
- [Phase ?]: golc.local.toml is re-validated strictly on every read, so hand-edited unknown/locked keys fail resolution with the same stable diagnostics as rejected writes.
- [Phase ?]: Strict concern decoding is Spec-driven: DefaultSpec is the production single-authority registry (six concerns, sixteen canonical keys); tests inject synthetic Specs for every failure mode.
- [Phase ?]: Cross-concern values use typed ref:<canonical.key> references resolved at repository level, so no authority literal (e.g. the Go pin) is ever duplicated across concern files.
- [Phase ?]: Deprecation outcomes use plan-specified codes CFG_DEPRECATED_KEY (non-fatal warning with migration guidance) and CFG_DEPRECATED_COLLISION (fatal); production deprecation register starts empty.
- [Phase ?]: Durable local ID grammar (project:slug, milestone:vN, phase:NN, req:KEY-NN, plan:NN-MM, task:NN-MM.p) derives only from structural metadata — linear-map seed IDs, two-digit numbers, XML task positions — never titles or issue keys; renames cannot change identity.
- [Phase ?]: Executable-task identity is the 1-based position among ALL task elements in a plan's <tasks> block; checkpoint tasks keep their position but receive no catalog entity.
- [Phase ?]: The D-11 authority split is a fixed typed registry: repository fields (scope, local_id, requirement_text, structure) and Linear operational fields (status, assignee, priority, estimate, completed_at) cannot be reassigned in either direction, and comment/discussion fields cannot be stored at all (D-12).
- [Phase ?]: Catalog entity sources must be repository-relative slash paths inside .planning/; near-miss plan filenames and drifted frontmatter fail the whole catalog build loudly instead of being skipped.

### Pending Todos

None yet.

### Blockers/Concerns

- `MIDI-HW-01` RESOLVED 2026-07-19: Akai MIDImix, Novation Launch Control XL Mk2, and Worlde EasyControl 9 form the selected Phase 6 physical acceptance set; manual evidence is recorded in `Akai-MIDImix-UserGuide-v1.0.pdf`, `launch_control_xl_programmer_s_reference_guide.pdf`, `Novation-Launch Control XL GSG v2.pdf`, and `Worlde-EasyControl-9-UserManual.pdf`. Selection and manual review do not establish compatibility or support.
- `MIDI-HW-02` OPEN: independent physical acceptance is required for each device's exact hardware revision, firmware, Windows version, and GOLC build before any named compatibility or support claim; device-specific profiles and feedback remain under EXTN-04/v1.x.
- Linear remote mappings are intentionally pending and contain no invented IDs. Local planning remains authoritative and usable offline; credentials are not part of repository configuration.
- Deeper phase research is required for fixture/pool semantics, playback timing, Art-Net, TypeScript isolation, AI, and Windows qualification; targeted storage research and Wails/MIDI operator validation are also required.

### Quick Tasks Completed

| # | Description | Date | Commit | Status | Directory |
|---|-------------|------|--------|--------|-----------|
| 260719-pgw | Research and record the Akai MIDImix, Novation Launch Control XL Mk2, and Worlde EasyControl 9 acceptance set; clear the selection blocker; verify Phase 1 readiness | 2026-07-20 | 6af8a48 | Verified | [260719-pgw-research-akai-midimix-novation-launch-co](./quick/260719-pgw-research-akai-midimix-novation-launch-co/) |

## Deferred Items

| Category | Item | Status | Deferred At |
|----------|------|--------|-------------|
| Platforms | macOS and Linux qualification | v2 | Roadmap creation |
| MIDI | Device-specific profiles and feedback | v1.x, gated independently per device by `MIDI-HW-02` and `EXTN-04` | MIDI-HW-01 resolution |

## Session Continuity

Last session: 2026-07-21T22:33:43.340Z
Stopped at: Phase 2 context gathered
Resume file: .planning/phases/02-modular-fixtures-and-deployments/02-CONTEXT.md
