---
phase: 01-offline-foundation-and-delivery-traceability
plan: 16
subsystem: bootstrap
tags: [powershell, go, sha256, toml, jsonschema, offline, gomodcache]

requires:
  - phase: 01-offline-foundation-and-delivery-traceability
    plan: 01
    provides: Red/bootstrap/green acceptance harness and checksum-first archive fixture contract
provides:
  - Checksum-verified staged/atomic project-local tool installation (PowerShell shim and Go package)
  - Pinned Go 1.26.5 toolchain provisioning with GOTOOLCHAIN=local and repository-local caches
  - Exact go.mod/go.sum authority for BurntSushi TOML v1.6.0 and Invopop jsonschema v0.14.0
  - Online module-graph warm plus network-denied schema-resolution proof in bootstrap acceptance mode
affects: [01-17, 01-04, 01-05, 01-28, 01-29, bootstrap, project-configuration]

tech-stack:
  added:
    - github.com/BurntSushi/toml v1.6.0
    - github.com/invopop/jsonschema v0.14.0
    - Go 1.26.5 (pinned project-local toolchain, official archive SHA-256)
  patterns:
    - Verify-before-extract with staging directory and single-rename promotion
    - Install manifest idempotency (matching manifest means zero archive-source calls)
    - Content-addressed verified downloads cache under .tools/cache/downloads
    - Lock-mutation hash guard around every bootstrap module operation

key-files:
  created:
    - golc.ps1
    - go.mod
    - go.sum
    - internal/bootstrap/bootstrap.go
    - internal/bootstrap/bootstrap_test.go
    - config/toolchain.toml
  modified:
    - tests/acceptance/walking-skeleton.ps1
    - .gitignore

key-decisions:
  - "Each tool archive promotes into its own atomic install unit (.tools/installs/<name>, .tools/toolchains/go/<version>/windows-amd64) so a single rename is the promotion boundary."
  - "The Go InstalledMatches re-hashes every recorded file; the PowerShell shim manifest records the exact archive pin and file count, keeping the 12k-file Go toolchain idempotency check fast."
  - "Bootstrap hashes go.mod/go.sum before and after every module operation and fails with GOLC_BOOTSTRAP_LOCK_MUTATION on any change, enforcing D-04 immutability mechanically."

patterns-established:
  - "Stable bootstrap diagnostics: GOLC_BOOTSTRAP_CHECKSUM_MISMATCH, GOLC_BOOTSTRAP_TRAVERSAL, GOLC_BOOTSTRAP_OFFLINE_ARTIFACT_MISSING, GOLC_TOOL_MISSING, GOLC_BOOTSTRAP_LOCK_MUTATION."
  - "Network denial for offline probes: GOPROXY=off, GOFLAGS=-mod=readonly, GOTOOLCHAIN=local, and HTTP(S)_PROXY pointed at a dead local proxy so any transport call fails."

requirements-completed: [CONF-01, CONF-03]

coverage:
  - id: D1
    description: "Matching SHA-256 bytes extract to staging and promote atomically; mismatches leave no install."
    requirement: CONF-03
    verification:
      - kind: unit
        ref: "internal/bootstrap/bootstrap_test.go TestInstallStagedPromotesVerifiedArchiveAtomically, TestInstallStagedLeavesNoInstallOnChecksumMismatch"
        status: pass
    human_judgment: false
  - id: D2
    description: "A matching installed manifest makes the second bootstrap perform zero archive-source calls."
    requirement: CONF-03
    verification:
      - kind: e2e
        ref: "walking-skeleton.ps1 bootstrap stage 2 deletes the archive source and downloads cache before the second bootstrap"
        status: pass
    human_judgment: false
  - id: D3
    description: "Project-local paths and GOTOOLCHAIN=local prevent host fallback."
    requirement: CONF-01
    verification:
      - kind: e2e
        ref: "golc.ps1 Set-ProjectGoEnvironment plus explicit .tools go.exe path; host Go 1.22.2 never invoked"
        status: pass
    human_judgment: false
  - id: D4
    description: "Online bootstrap warms the complete module graph; the GOPROXY=off readonly probe resolves both direct modules and emits schema bytes with zero download or lock/cache mutation."
    requirement: CONF-03
    verification:
      - kind: e2e
        ref: "powershell -NoProfile -File .\\tests\\acceptance\\walking-skeleton.ps1 -Mode bootstrap"
        status: pass
    human_judgment: false

duration: 25min
completed: 2026-07-20
status: complete
---

# Phase 1 Plan 16: Checksum-Controlled Bootstrap Summary

**PowerShell shim plus Go bootstrap package provisioning a SHA-256-pinned Go 1.26.5 toolchain and warmed TOML/Invopop module graph, proven resolvable offline with zero download or lock mutation**

## Performance

- **Duration:** 25 min
- **Started:** 2026-07-20T17:58:13Z
- **Completed:** 2026-07-20T18:23:00Z
- **Tasks:** 1 (TDD)
- **Files modified:** 8

## Accomplishments

- `golc.ps1` root shim reads exact pins from `config/toolchain.toml`, verifies archive bytes and entry containment before extraction, stages and atomically promotes installs, and records install manifests that make a matching second bootstrap skip the archive source entirely.
- `internal/bootstrap` provides the tested `VerifyArchive`/`InstallStaged`/`InstalledMatches` contract: checksum format/mismatch rejection, dot-dot/rooted/drive traversal rejection across both slash conventions, staged atomic promotion, per-file manifest re-verification, and tamper detection.
- `go.mod`/`go.sum` pin `github.com/BurntSushi/toml v1.6.0` and `github.com/invopop/jsonschema v0.14.0` with the complete verified transitive graph; bootstrap downloads the graph into the repository-local `GOMODCACHE`, runs `go mod verify`, confirms both exact direct pins in `go list -m all`, records the selected graph, and fails on any `go.mod`/`go.sum` byte change.
- Walking-skeleton bootstrap mode now proves the full sequence: checksum-controlled temp-checkout install, zero-archive-source second bootstrap, online repository warm, and a network-denied (`GOPROXY=off`, readonly, dead-proxy transport) rerun of the TOML-decode/Invopop-reflection probe with byte-identical locks and module-cache inventory.

## Task Commits

TDD gates committed atomically:

1. **RED - Task 1: bootstrap contract and module probe tests** - `cdaec3b` (test)
2. **GREEN - Task 1: shim, bootstrap package, pins, harness extension** - `f237eb9` (feat)

**Plan metadata:** committed with this summary

## Files Created/Modified

- `golc.ps1` - Root command shim: bootstrap authority and delegation to the pinned project-local command.
- `go.mod` / `go.sum` - Module authority declaring `github.com/lnorton89/golc` (originally created as `github.com/lawrence/golc` from a planning error; corrected in 01-17 commit `0286186`), Go `1.26.5`, both exact direct pins, and the complete verified sum graph.
- `internal/bootstrap/bootstrap.go` - Verified staged installation: `VerifyArchive`, `InstallStaged`, `InstalledMatches`, install manifest.
- `internal/bootstrap/bootstrap_test.go` - Contract tests plus the TOML/Invopop schema probe run online by bootstrap and offline by the harness.
- `config/toolchain.toml` - Committed exact-pin concern: Go 1.26.5 official archive URL/SHA-256 and repository-local cache policy.
- `tests/acceptance/walking-skeleton.ps1` - Bootstrap mode extended with idempotency, online warm, and network-denied probe stages.
- `.gitignore` - Ignores the provisioned `.tools/` toolchains, caches, and installs.

## Decisions Made

- Verified archives are cached content-addressed under `.tools/cache/downloads/<sha256>.zip`; a cached copy is re-hashed against the pin before reuse, so checksum-before-use holds on every path.
- Each install target is its own atomic promotion unit (`.tools/installs/<tool>`, `.tools/toolchains/go/<version>/windows-amd64`) rather than a shared `.tools/bin`, so promotion is always a single rename and partial installs cannot exist.
- The shim's install manifest records the exact archive pin plus file count for fast idempotency over the ~12k-file Go toolchain, while the Go `InstalledMatches` implementation re-hashes every recorded file for tool archives it manages; both make a matching second bootstrap consult no archive source.
- Bootstrap explicitly sets `GOTOOLCHAIN=local`, `GOFLAGS=-mod=readonly`, and repository-local `GOMODCACHE`/`GOCACHE`, and treats any resulting `go.mod`/`go.sum` byte change as a hard `GOLC_BOOTSTRAP_LOCK_MUTATION` failure.
- Network denial in the acceptance probe combines `GOPROXY=off` (module resolution forbidden) with `HTTP_PROXY`/`HTTPS_PROXY` aimed at a dead local proxy so any transport call fails immediately.

## Deviations from Plan

None - plan executed exactly as written. Layout choices (per-tool install units, content-addressed downloads cache) fall under the context's explicit discretion for cache locations and verification mechanics.

## Issues Encountered

- Host Go is 1.22.2, which cannot build a `go 1.26.5` module under `GOTOOLCHAIN=local`; this is exactly the host-fallback hazard D-01/D-02 guard against. Bootstrap provisions the pinned Go 1.26.5 toolchain project-locally (official archive hash matched the researched pin `97e6b2...fd38`), and no host Go is ever invoked.

## Known Stubs

- `golc.ps1` delegation targets `.tools/installs/golc_project/bin/golc-project.exe`, which does not exist in the repository yet; Plan 01-17 builds the real CLI, and non-bootstrap subcommands currently fail with the stable `GOLC_TOOL_MISSING` diagnostic by design.
- Walking-skeleton `red` mode described the pre-implementation contract (absent `golc.ps1`) and no longer passes now that the root command exists; `bootstrap` mode is the wave-2 gate, and `green` mode remains red until Plan 01-17.
- `config/toolchain.toml` pins only the Go toolchain; Node/Wails pins arrive with the plans that need them (01-13/01-25 for Node after the approved npm gate), keeping this plan npm-free.

## User Setup Required

None - bootstrap provisions everything project-locally; no credentials, npm, or Linear access involved.

## Next Phase Readiness

- Plan 01-17 can build `cmd/golc-project` and the command router against the working bootstrap and delegation path.
- Plan 01-04's network-denied contracts probe can rely on the warmed, verified module cache and the lock-mutation guard.
- Plans 01-05/01-28 can extend `internal/bootstrap` (official-source boundary, cache-warm acceptance) from the established verify/stage/promote contract.

## Self-Check: PASSED

- All seven owned files exist on disk (`golc.ps1`, `go.mod`, `go.sum`, `internal/bootstrap/bootstrap.go`, `internal/bootstrap/bootstrap_test.go`, `config/toolchain.toml`, `tests/acceptance/walking-skeleton.ps1`).
- Both TDD gate commits `cdaec3b` (test) and `f237eb9` (feat) exist in git history.
- The exact verification command `powershell -NoProfile -File .\tests\acceptance\walking-skeleton.ps1 -Mode bootstrap` exits 0 from the repository root.

---
*Phase: 01-offline-foundation-and-delivery-traceability*
*Completed: 2026-07-20*
