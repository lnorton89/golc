---
phase: quick-toolchain-schema-v2
plan: 260723-svv
subsystem: configuration-bootstrap
tags: [go, powershell, toml, bootstrap, toolchain, security]
requires:
  - plan: 260723-s7n
    provides: callable Go bootstrap engine and secure archive acquisition
provides:
  - Atomic strict configuration schema version 2
  - Exact windows-amd64 Go and Node archive authority keys
  - Fail-closed current-platform selection in the Go bootstrap
  - Schema-v2 tools-update and temporary PowerShell compatibility readers
affects: [bootstrap-entrypoint-removal, contributor-tooling, configuration]
tech-stack:
  added: []
  patterns:
    - Tool identity and source policy live on parent tables while executable bytes live under exact platform tables
    - Strict configuration registers every flattened platform key explicitly without wildcard matching
    - Bootstrap resolves PlatformKey before cache, source, or install effects
key-files:
  created: []
  modified:
    - config/toolchain.toml
    - internal/projectconfig/model.go
    - internal/projectconfig/load.go
    - internal/bootstrap/engine.go
    - internal/bootstrap/engine_linear.go
    - internal/command/tools.go
    - golc.ps1
key-decisions:
  - "Only windows-amd64 is a configured Go and Node archive platform; pure filename-layout knowledge does not authorize or imply another platform pin."
  - "archive_url is the sole active archive locator for generic tools and platform archives; archive_uri is accepted nowhere."
  - "The PowerShell entrypoint remains a compatibility shim and was not redirected to the Go Bootstrap API."
requirements-completed: [CONF-01, CONF-02, CONF-03]
duration: ~13min
completed: 2026-07-23
status: complete
---

# Quick Task 260723-svv: Toolchain Schema V2 Summary

**Strict schema-v2 configuration now enumerates exact windows-amd64 archive pins, and every Go, update, fixture, and compatibility reader consumes that explicit shape without fallback or fabricated platform data.**

## Performance

- **Duration:** ~13 min
- **Completed:** 2026-07-23
- **Tasks:** 3 TDD tasks
- **Files modified:** 28

## Accomplishments

- Migrated the root index and all six concerns atomically to schema version 2, moved the four unchanged Go/Node URL and checksum literals under quoted windows-amd64 tables, and updated user/local/redaction fixtures.
- Registered only the exact four platform-qualified archive keys, narrowly admitted internal hyphens in canonical segments, and proved schema-v1, mixed-schema, malformed-key, and unregistered-platform inputs fail closed.
- Split Go bootstrap toolchain parents from their platform archive maps, selected Go and optional Node through `PlatformKey()` before effects, and removed the `archive_uri` compatibility fallback.
- Updated tools-update to read and surgically rewrite two parent versions plus four windows-amd64 archive values while preserving all unrelated bytes.
- Kept `golc.ps1` as the current entrypoint while teaching its narrow TOML reader the exact quoted platform headers and unified `archive_url` field.

## Task Commits

1. **Task 1 RED:** `f0952b4` — strict schema-v2 tests and fixtures.
2. **Task 1 GREEN:** `cfb65a9` — atomic schema-v2 configuration authority.
3. **Task 2 RED:** `783af3f` — explicit bootstrap platform-selection tests.
4. **Task 2 GREEN:** `cdd36cf` — fail-closed current-platform bootstrap selection.
5. **Task 3 RED:** `48362cb` — schema-v2 update and compatibility fixtures.
6. **Task 3 GREEN:** `2b62b0d` — direct reader and PowerShell compatibility changes.

## Verification

- All focused scopes passed: `config-strict`, `config-local`, `config`, `bootstrap-archive`, `bootstrap-engine`, `bootstrap-linear-sync`, `tools-update`, and `secrets`.
- `go test ./...` passed.
- `powershell -NoProfile -File .\golc.ps1 build` passed.
- The local bootstrap acceptance passed corrupt generic-pin rejection, corrected retry, zero-source repeat, quoted windows-amd64 selection, and missing-platform rejection before reaching the pre-existing cache-warm issue below.
- Production manifest parsing tests prove Go and Node each configure exactly `windows-amd64`.
- Static audit found `schema_version = 1` and `archive_uri` only in intentional rejection tests.
- The four locked production URLs/checksums remain byte-identical.
- `go.mod`, `go.sum`, npm manifests/locks, documentation, and README have no executor-created diff.

## Deviations from Plan

None — implementation stayed within the declared files and completed PowerShell-removal Step 2 only.

## Deferred Issues

- `tests/acceptance/bootstrap.ps1` reaches its unrelated repository cache-warm stage, where the existing PowerShell bootstrap runs `go mod download all`, expands the currently incomplete `go.sum`, and then reports `GOLC_BOOTSTRAP_LOCK_MUTATION`. The verification-generated change was restored; dependency locks remain untouched.
- The root `golc.ps1 test` command completed the entire Go suite successfully, then failed its optional Node scope because the Linear workspace is not bootstrapped (`golc-linear-sync-node-not-bootstrapped`). No package-manager install was attempted.

## Known Stubs

None.

## Threat Flags

None — all changed configuration, platform-selection, archive, update, and compatibility trust boundaries are declared in the plan threat model and covered by focused tests.

## Self-Check: PASSED

- All plan-owned implementation/test files and this summary exist.
- Commits `f0952b4`, `cfb65a9`, `783af3f`, `cdd36cf`, `48362cb`, and `2b62b0d` exist in repository history.
- No tracked file was deleted by a task commit.
- The unrelated `260723-sgy` quick task and concurrent sketch work remain untouched.
