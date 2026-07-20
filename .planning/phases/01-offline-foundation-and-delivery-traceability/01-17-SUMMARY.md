---
phase: 01-offline-foundation-and-delivery-traceability
plan: 17
subsystem: project-configuration
tags: [go, powershell, toml, json, command-router, walking-skeleton]

requires:
  - phase: 01-offline-foundation-and-delivery-traceability
    plan: 01
    provides: Red/bootstrap/green acceptance harness and data-only TOML fixture contract
  - phase: 01-offline-foundation-and-delivery-traceability
    plan: 16
    provides: Checksum-controlled bootstrap, pinned Go 1.26.5 toolchain, golc.ps1 shim, warmed module graph
provides:
  - Compile-safe route/scope self-registration contract (MustDeclareRoute/MustDeclareScope -> NewDefaultCommandRegistry)
  - Deterministic duplicate-rejecting command registry with byte-stable Routes/Scopes/Lookup
  - Contained strict root-index/concern TOML inspection with deterministic sorted-JSON output
  - Green walking skeleton: bootstrap -> real Go CLI -> committed runtime concern, byte-identical JSON
affects: [01-02, 01-03, 01-04, 01-18, project-configuration, command-router]

tech-stack:
  added: []
  patterns:
    - Package-level var MustDeclareRoute/MustDeclareScope self-registration; main only blank-imports command files
    - Normalized exact route/scope keys with stable GOLC_* diagnostics and sorted deterministic introspection
    - Concern-path containment; syntactic rejection plus final EvalSymlinks repository-containment check
    - Deterministic JSON via encoding/json sorted map keys plus trailing newline

key-files:
  created:
    - cmd/golc-project/main.go
    - internal/command/router.go
    - internal/command/router_test.go
    - internal/projectconfig/load.go
    - internal/projectconfig/load_test.go
    - golc.project.toml
    - config/runtime.toml
  modified:
    - golc.ps1
    - tests/acceptance/walking-skeleton.ps1
    - go.mod

key-decisions:
  - "Routes must belong to a declared scope (GOLC_ROUTE_SCOPE_UNDECLARED), making MustDeclareScope a real precondition for every command graph."
  - "The default registry sorts declarations before registering, so duplicate rejection and lookup are deterministic regardless of file declaration order."
  - "golc.ps1 delegation exports GOLC_PROJECT_ROOT so command behavior never depends on the caller's working directory."
  - "Green acceptance packages the real built golc-project.exe as the checksum-pinned archive payload, replacing the 01-01 placeholder text payload."

patterns-established:
  - "Stable command diagnostics: GOLC_ROUTE_DUPLICATE, GOLC_SCOPE_DUPLICATE, GOLC_ROUTE_SCOPE_UNDECLARED, GOLC_ROUTE_UNKNOWN, GOLC_CONFIG_PATH_ESCAPE, GOLC_CONFIG_UNKNOWN_KEY, GOLC_CONFIG_SCHEMA_VERSION, GOLC_CONFIG_CONCERN_UNKNOWN."
  - "Result-to-exit mapping: 0 success, 1 command failure, 2 routing/usage/startup failure."

requirements-completed: [CONF-01, CONF-03]

coverage:
  - id: D1
    description: "A fixture route/scope declared through the exact production entrypoints is reachable from NewDefaultCommandRegistry without editing router.go."
    requirement: CONF-01
    verification:
      - kind: unit
        ref: "internal/command/router_test.go TestSelfRegisteredFixtureRouteReachableFromDefaultRegistry"
        status: pass
    human_judgment: false
  - id: D2
    description: "Duplicate normalized routes/scopes are rejected with stable codes and Routes/Scopes/Lookup are deterministic across declaration order."
    requirement: CONF-01
    verification:
      - kind: unit
        ref: "internal/command/router_test.go TestRegisterRouteRejectsDuplicateNormalizedRoutes, TestRoutesAndScopesAreDeterministicAcrossDeclarationOrder, TestLookupPrefersLongestExactRoute"
        status: pass
    human_judgment: false
  - id: D3
    description: "Concern paths cannot escape through absolute, parent, drive, or symlink paths; inspection JSON is byte-identical across runs."
    requirement: CONF-01
    verification:
      - kind: unit
        ref: "internal/projectconfig/load_test.go TestConcernPathsCannotEscapeRepository, TestConcernPathsCannotEscapeThroughSymlinks, TestInspectConcernEmitsDeterministicSortedJSON"
        status: pass
    human_judgment: false
  - id: D4
    description: "Green walking skeleton: clean temporary checkout bootstraps the checksum-pinned real CLI and 'config inspect runtime --format json' is byte-identical with runtime.log_level=info."
    requirement: CONF-03
    verification:
      - kind: e2e
        ref: "powershell -NoProfile -File .\\tests\\acceptance\\walking-skeleton.ps1 -Mode green"
        status: pass
    human_judgment: false

duration: 35min
completed: 2026-07-20
status: complete
---

# Phase 1 Plan 17: Walking Skeleton Completion and Command Registry Summary

**Green walking skeleton served by a real Go CLI whose routes self-register via MustDeclareRoute/MustDeclareScope, with duplicate-rejecting deterministic lookup and contained strict TOML-to-JSON runtime inspection**

## Performance

- **Duration:** ~35 min (across a session cut)
- **Started:** 2026-07-20T18:16:55Z
- **Completed:** 2026-07-20T18:42:00Z
- **Tasks:** 1 (TDD)
- **Files modified:** 10

## Accomplishments

- `internal/command/router.go` implements the D-03 contract: command files declare `MustDeclareRoute`/`MustDeclareScope` from package-level var initializers, `NewDefaultCommandRegistry` builds from those declarations in sorted order, duplicates fail with stable `GOLC_ROUTE_DUPLICATE`/`GOLC_SCOPE_DUPLICATE` codes before any handler runs, and `Routes`/`Scopes`/`Lookup` are deterministic byte-stable regardless of declaration order.
- `internal/projectconfig/load.go` self-registers the exact route `config inspect` (no central switch anywhere), strictly validates `golc.project.toml` (unknown keys, schema version, duplicate concern ids), and resolves concern files with both syntactic rejection (absolute/parent/drive/dot segments) and a final `EvalSymlinks` repository-containment check (T-01-03).
- `cmd/golc-project/main.go` contains no route switch: it blank-imports command files, builds the default registry, and applies the stable 0/1/2 result-to-exit mapping. `golc.ps1 bootstrap` now builds this command with `-trimpath` via the pinned Go 1.26.5 toolchain, and delegation exports `GOLC_PROJECT_ROOT`.
- Committed `golc.project.toml` owns only schema/index metadata; `config/runtime.toml` alone owns `runtime.log_level = "info"`. Green acceptance exits 0: the temporary clean checkout installs the real checksum-pinned CLI and two `config inspect runtime --format json` runs are byte-identical JSON.

## Task Commits

TDD gates committed atomically:

1. **RED - Task 1: registry and contained inspection contracts** - `b770e48` (test)
2. **GREEN - Task 1: registry, config inspection, entrypoint, concern files, shim/harness wiring** - `849f8f6` (feat)
3. **FIX - Task 1: module path corrected to github.com/lnorton89/golc (user correction)** - `0286186` (fix)

**Plan metadata:** committed with this summary

## Files Created/Modified

- `internal/command/router.go` - Registry, normalization, duplicate rejection, deterministic introspection, declaration entrypoints, `Execute`, `WriteResult`.
- `internal/command/router_test.go` - Fixture self-registration reachability, duplicate/scope rejection, order determinism, longest-exact-match lookup, unknown-route code.
- `internal/projectconfig/load.go` - Strict root-index load, contained concern resolution, deterministic JSON inspection, self-registered `config inspect` handler.
- `internal/projectconfig/load_test.go` - Byte-identity, unknown key/concern, schema-version, duplicate-id, and escape-path (incl. symlink) tests.
- `cmd/golc-project/main.go` - Switch-free entrypoint with stable result-to-exit mapping and `GOLC_PROJECT_ROOT` resolution.
- `golc.project.toml` - Root index: schema/index metadata only; concerns `toolchain` and `runtime`.
- `config/runtime.toml` - Sole owner of `runtime.log_level` (committed value `info`).
- `golc.ps1` - Bootstrap builds `golc-project.exe` into `.tools\installs\golc_project\bin\`; delegation passes the repository root.
- `tests/acceptance/walking-skeleton.ps1` - Green mode bootstraps the repository under test and packages the real built executable as the checksum-pinned archive payload.

## Decisions Made

- Scope declarations are enforced: `RegisterRoute` fails with `GOLC_ROUTE_SCOPE_UNDECLARED` unless the route's first word matches a declared scope, so "each owning command graph declares its scope" is mechanical, not conventional.
- `NewDefaultCommandRegistry` sorts declared scopes and routes before registering, making duplicate detection and diagnostics independent of Go file initialization order.
- Deterministic JSON comes from `encoding/json`'s sorted map-key marshaling over the strictly decoded TOML document (schema_version validated then stripped), with a single trailing newline.
- The shim, not the CLI, owns root discovery: delegation exports `GOLC_PROJECT_ROOT=$RepoRoot`; the CLI falls back to the working directory only when invoked without the shim.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Green-mode fixture archive carried an inert text payload**
- **Found during:** Task 1 design (before GREEN implementation)
- **Issue:** `New-ChecksumToolchainFixture` packaged the 01-01 placeholder text file as `golc-project.exe` (a documented 01-01 known stub), so the temporary checkout could never execute `config inspect` and green could never pass.
- **Fix:** Green mode first runs the repository-under-test bootstrap (offline after the 01-16 warm), asserts the built `golc-project.exe` exists, and packages that real executable via a new optional `-PayloadExecutable` parameter. Bootstrap mode keeps the inert payload, and its acceptance still passes.
- **Files modified:** `tests/acceptance/walking-skeleton.ps1`
- **Verification:** Both `-Mode green` and `-Mode bootstrap` exit 0.
- **Committed in:** `849f8f6`

**2. [Rule 2 - Missing critical tests] Added `internal/projectconfig/load_test.go`**
- **Found during:** Task 1 RED phase
- **Issue:** The plan's artifact list had no test file covering the T-01-03 mitigation (concern-path containment) or byte-identical inspection, both explicit `<behavior>` items.
- **Fix:** Added unit tests for escape rejection (absolute, parent, drive, dot, symlink), strict unknown-key/schema-version/duplicate-id validation, and deterministic byte-identical JSON.
- **Files modified:** `internal/projectconfig/load_test.go`
- **Verification:** All tests pass under the pinned toolchain (symlink case skips when the host denies symlink creation).
- **Committed in:** `b770e48`

### User-Directed Changes

**3. [User correction] Module path renamed to `github.com/lnorton89/golc`**
- **Found during:** Task 1 finalization (user correction relayed mid-execution)
- **Issue:** `go.mod` and all Go imports used `github.com/lawrence/golc`, a wrong path originating in 01-16 planning.
- **Fix:** Renamed the `module` directive and every import in `cmd/golc-project/main.go`, `internal/command/router_test.go`, `internal/projectconfig/load.go`, and `internal/projectconfig/load_test.go`; re-ran tests, vet, gofmt, and both green and bootstrap acceptance modes after the rename.
- **Files modified:** `go.mod`, `cmd/golc-project/main.go`, `internal/command/router_test.go`, `internal/projectconfig/load.go`, `internal/projectconfig/load_test.go`
- **Verification:** `go test ./...` all ok under the corrected path; `-Mode green` and `-Mode bootstrap` both exit 0.
- **Committed in:** `0286186`

---

**Total deviations:** 2 auto-fixed (1 Rule 3 blocking, 1 Rule 2 missing tests) + 1 user-directed correction
**Impact on plan:** The auto-fixes were required to make the plan's own green contract and threat-model mitigations real; the module rename corrects project identity with no behavioral change. No scope expansion beyond the walking skeleton.

## Issues Encountered

None beyond the deviations above. `--format json` passes through Windows PowerShell 5.1 `ValueFromRemainingArguments` binding intact, so no shim argument rewriting was needed.

## Known Stubs

- `golc.ps1` usage advertises `check|generate|build|test|package|linear`, but only the `config` scope with route `config inspect` is registered; other subcommands delegate and fail with stable `GOLC_ROUTE_UNKNOWN` (exit 2) until their owning plans self-register them. This is the intended D-03 growth path, not a missing wire for this plan's goal.
- Walking-skeleton `red` mode still describes the pre-implementation contract (absent `golc.ps1`) and no longer passes by design (documented since 01-16); `bootstrap` and `green` are the live gates.

## User Setup Required

None - everything is repository-local; no credentials, npm, or Linear access involved.

## Next Phase Readiness

- Every later command plan can add routes by creating a file with `MustDeclareRoute`/`MustDeclareScope` and (at most) one blank import in `cmd/golc-project/main.go` — `internal/command/router.go` needs no further edits.
- `internal/projectconfig` provides the strict root-index/concern loading seam that 01-02/01-03 (validation, provenance layers) can extend.
- The green walking skeleton is the standing end-to-end gate: shim -> checksum-pinned real CLI -> committed TOML concern -> deterministic JSON.

## Self-Check: PASSED

- All nine created/modified files exist on disk (`cmd/golc-project/main.go`, `internal/command/router.go`, `internal/command/router_test.go`, `internal/projectconfig/load.go`, `internal/projectconfig/load_test.go`, `golc.project.toml`, `config/runtime.toml`, `golc.ps1`, `tests/acceptance/walking-skeleton.ps1`).
- TDD gate commits `b770e48` (test) and `849f8f6` (feat) plus correction commit `0286186` (fix) exist in git history.
- `powershell -NoProfile -File .\tests\acceptance\walking-skeleton.ps1 -Mode green` and `-Mode bootstrap` both exit 0 from the repository root.

---
*Phase: 01-offline-foundation-and-delivery-traceability*
*Completed: 2026-07-20*
