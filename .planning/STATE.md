---
gsd_state_version: 1.0
milestone: v1.0-shipped
milestone_name: milestone
current_phase: 08
current_phase_name: isolated-typescript-automation
status: milestone_complete
stopped_at: v1.0 milestone archived (Phases 1-8); next milestone (Phases 9-11) awaiting hygiene/front-door-UI phase insertion per POST-PHASE-8-PLAN.md
last_updated: "2026-07-27T23:15:00.000Z"
last_activity: 2026-07-27
last_activity_desc: v1.0 milestone closed and archived to .planning/milestones/ (Phases 1-8, 99 plans); REQUIREMENTS.md re-scoped to the remaining LLM/Windows/telemetry requirements for the next milestone
progress:
  total_phases: 11
  completed_phases: 8
  total_plans: 99
  completed_plans: 99
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-07-27)

**Core value:** An operator can author a modular show once, adapt its fixture pools to different deployments in one or two actions, and hand a simple controller surface to another person for reliable playback.
**Current focus:** v1.0 shipped (Phases 1-8, archived to `.planning/milestones/v1.0-ROADMAP.md`). Next: insert the hygiene and front-door-UI phases from `.planning/POST-PHASE-8-PLAN.md` before starting Phase 9 (Provider-Neutral AI and Bounded Autonomy).

## Current Position

Milestone v1.0: COMPLETE (Phases 1-8, 99/99 plans, shipped 2026-07-27)
Next: Phase 9 (Provider-Neutral AI and Bounded Autonomy), pending insertion of the hygiene and front-door-UI phases `.planning/POST-PHASE-8-PLAN.md` calls for first.
Last activity: 2026-07-27 — v1.0 milestone closed and archived (see .planning/MILESTONES.md and .planning/milestones/v1.0-ROADMAP.md)

Note: Phase 9 depends on Phases 2/6/7/8 (all satisfied), Phase 10 depends on Phases 1-9, and Phase 11 depends on Phase 10. These phases remain active (not archived) in `.planning/ROADMAP.md` since they were carried forward rather than shipped as part of v1.0.

Progress: [███████░░░] 73% (8/11 phases, 99/99 plans in completed phases)

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

### Roadmap Evolution

- Phase 11 added: Telemetry, Usage Statistics, and Auto Crash Submission Pipeline — users can opt into anonymized usage/telemetry collection and crashes are automatically captured and submitted for diagnosis without blocking playback or requiring manual repro steps.

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
| 260723-rq4 | Add cross-platform transport for internal/artnet/ipc as Step 0 of the PowerShell removal plan | 2026-07-23 | 09d25fc | Complete | [260723-rq4-add-cross-platform-transport-for-interna](./quick/260723-rq4-add-cross-platform-transport-for-interna/) |
| 260723-rym | Update packaged UI sketch assets to match the golc-site light theme | 2026-07-23 | c8dfb9a | Complete | [260723-rym-update-packaged-ui-sketch-assets-to-matc](./quick/260723-rym-update-packaged-ui-sketch-assets-to-matc/) |
| 260723-s7n | Implement the complete Go bootstrap engine from Step 1 of the PowerShell removal plan | 2026-07-23 | 33b1242 | Complete | [260723-s7n-implement-the-complete-go-bootstrap-engi](./quick/260723-s7n-implement-the-complete-go-bootstrap-engi/) |
| 260723-svv | Migrate toolchain configuration and strict project schema for PowerShell removal Step 2 | 2026-07-23 | 2b62b0d | Complete | [260723-svv-migrate-toolchain-configuration-and-stri](./quick/260723-svv-migrate-toolchain-configuration-and-stri/) |
| 260723-tgd | Replace hardcoded platform strings and propagate project root for PowerShell removal Step 3 | 2026-07-23 | e024305 | Complete | [260723-tgd-replace-hardcoded-platform-strings-and-p](./quick/260723-tgd-replace-hardcoded-platform-strings-and-p/) |
| 260723-u0p | Add Mage targets and pin Mage toolchain archives for PowerShell removal Steps 4 and 5 | 2026-07-23 | afc4623 | Complete | [260723-u0p-add-mage-targets-and-pin-mage-toolchain-](./quick/260723-u0p-add-mage-targets-and-pin-mage-toolchain-/) |
| 260723-ule | Improve golc-mcp status freshness and expose Mage migration surfaces | 2026-07-23 | 7ead8ed | Complete | [260723-ule-improve-golc-mcp-status-freshness-and-ex](./quick/260723-ule-improve-golc-mcp-status-freshness-and-ex/) |
| 260723-v4o | Rewrite command parity around Mage targets for PowerShell removal Step 6 | 2026-07-23 | fbab0bd | Complete | [260723-v4o-rewrite-command-parity-around-mage-targe](./quick/260723-v4o-rewrite-command-parity-around-mage-targe/) |
| 260723-vj8 | Add nonblocking cross-platform Mage CI observation matrix for PowerShell removal Step 8; fixed a pre-existing build-route regression (magefiles excluded from the build package sweep) found while verifying it | 2026-07-23 | f318c3a | Complete | [260723-vj8-add-nonblocking-cross-platform-mage-ci-a](./quick/260723-vj8-add-nonblocking-cross-platform-mage-ci-a/) |
| 260723-sgy | Port the full golc-site design language into all approved UI sketch assets | 2026-07-23 | 5a5a55b | Needs Review | [260723-sgy-port-the-full-golc-site-design-language-](./quick/260723-sgy-port-the-full-golc-site-design-language-/) |
| 260723-tyl | Make active command-rail accents symmetrical in all four UI sketches | 2026-07-23 | 7889800 | Complete | [260723-tyl-make-active-command-rail-accents-symmetr](./quick/260723-tyl-make-active-command-rail-accents-symmetr/) |
| 260723-uj9 | Fold the sketch findings skill into .planning/sketches and remove .kimi-code | 2026-07-23 | 549d2cf | Complete | [260723-uj9-fold-the-sketch-findings-skill-into-plan](./quick/260723-uj9-fold-the-sketch-findings-skill-into-plan/) |
| 260724-w3f | Harden cross-platform-mage.yml to a fully green three-OS run | 2026-07-24 | a795150 | Complete | [260724-w3f-harden-cross-platform-mage-ci-to-a-fu](./quick/260724-w3f-harden-cross-platform-mage-ci-to-a-fu/) |
| 260724-x7n | Delete golc.ps1 and retire every reference (PowerShell removal Step 7) | 2026-07-24 | f32fdf1 | Complete | [260724-x7n-delete-golc-ps1-and-retire-every-refe](./quick/260724-x7n-delete-golc-ps1-and-retire-every-refe/) |
| 260724-y2t | Final byte-for-byte verification across three CI workflows (PowerShell removal Step 9) | 2026-07-24 | 8658b32 | Complete | [260724-y2t-final-verification-across-all-three-c](./quick/260724-y2t-final-verification-across-all-three-c/) |

## Deferred Items

| Category | Item | Status | Deferred At |
|----------|------|--------|-------------|
| Platforms | macOS and Linux qualification | v2 | Roadmap creation |
| MIDI | Device-specific profiles and feedback | v1.x, gated independently per device by `MIDI-HW-02` and `EXTN-04` | MIDI-HW-01 resolution |

## Session Continuity

Last session: 2026-07-25T23:48:30.467Z
Stopped at: Phase 8 UI-SPEC approved
Resume file: .planning/phases/08-isolated-typescript-automation/08-UI-SPEC.md
