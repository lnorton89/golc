# Walking Skeleton — GOLC

**Phase:** 1
**Generated:** 2026-07-17

## Capability Proven End-to-End

> A contributor can run `golc.ps1` to bootstrap the pinned project command, write an ignored local configuration value, read it back through the Go command router, and inspect deterministic provenance without Linear, credentials, a product UI, or a product database.

Phase 1 deliberately adapts the generic walking-skeleton “UI → API → database” wording to its locked developer-tooling boundary. The user interaction is the root PowerShell command, routing is the project-local Go CLI, and the real data layer is strict repository-owned TOML plus ignored local TOML and credential-free JSON identity state. Pulling the Wails operator UI or SQLite show storage into this phase would violate the roadmap boundary and move Phase 6 or Phase 5 work forward.

## Architectural Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Framework | Windows PowerShell 5.1 bootstrap shim delegating to a Go 1.26.5 project-local CLI | A clean supported Windows checkout has a bootstrap-capable shell, while tested command behavior belongs in Go and remains independent of the later Wails adapter. |
| Data layer | Strict TOML 1.0 root/concern files, ignored local TOML overrides, and canonical credential-free JSON maps/plans | These are the real persisted records for this contributor-facing phase; they provide deterministic read/write behavior without inventing the Phase 5 show database. |
| Auth | No authentication for offline commands; an externally supplied Linear credential is accepted only by explicit remote subcommands | Local configuration, build, test, package, and trace validation remain complete without secrets. |
| Deployment target | Documented local full-stack command on Windows plus Windows CI invoking the same root command | Phase 1 qualifies the developer foundation; Wails application deployment and NSIS installation remain in their owning phases. |
| Directory layout | `golc.ps1` at the root; Go command under `cmd/golc-project`; domain tooling under `internal/projectconfig`, `internal/contracts`, and `internal/trace`; isolated SDK adapter under `tools/linear-sync` | The layout makes authority boundaries explicit and prevents Linear transport or future UI code from becoming project-state authority. |

## Stack Touched in Phase 1

- [x] Project scaffold — Go module, PowerShell root command, exact tool manifests, Go and Node test harnesses
- [x] Routing — `golc.ps1` delegates normal subcommands to `.tools/bin/golc-project.exe`
- [x] Data persistence adaptation — a real ignored local TOML value and credential-free JSON identity mapping are written and read back
- [x] User interaction adaptation — contributor CLI commands drive the Go router and receive deterministic machine-readable output
- [x] Deployment adaptation — a documented local command exercises the complete slice and Windows CI uses the same root entrypoint

## Out of Scope (Deferred to Later Slices)

- Wails product UI and operator interaction (Phase 6)
- SQLite `.golc` show persistence and recovery (Phase 5)
- Wails/NSIS product installer and Windows release qualification (Phase 10)
- Fixture authoring and deployment behavior (Phase 2)
- Playback, Art-Net, scripting, public API, and AI runtime behavior (Phases 3, 4, 7, 8, and 9)
- Live Linear mutation in autonomous tests; the first real apply remains an explicit reviewed action when taxonomy and credentials are supplied outside Git

## Subsequent Slice Plan

Each later phase adds one vertical slice on top of this skeleton without changing its repository-owned authority or offline/remote separation:

- Phase 2: Authors validate fixtures and review atomic deployment adaptations.
- Phase 3: Authors program and run deterministic tempo-aware shows.
- Phase 4: Operators emit and inspect real Art-Net output.
- Phase 5: Users persist, migrate, and recover portable shows.
- Phase 6: Authors and operators use the Wails, keyboard, and generic MIDI surfaces.
- Phase 7: External programs use the versioned command API and event stream.
- Phase 8: Users run isolated, typed TypeScript automation.
- Phase 9: Users employ provider-neutral AI under bounded, revocable authority.
- Phase 10: Operators install and run a qualified Windows release.
