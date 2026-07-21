---
phase: 01-offline-foundation-and-delivery-traceability
plan: 28
subsystem: infra
tags: [go, powershell, bootstrap, cache, gobin, gomodcache, gocache, idempotence, wails-pin]

requires:
  - phase: 01-offline-foundation-and-delivery-traceability
    plan: 05
    provides: internal/bootstrap/archive.go and downloader.go (VerifySHA256/InspectZipEntries/ExtractVerified/PromoteAtomically, OfficialSourcePolicy) that this plan's cache-warming layer builds on top of
provides:
  - internal/bootstrap/cache.go — ProjectCacheLayout (Downloads/GoModCache/GoBuildCache/GoBin/Manifest, all validated contained inside Root), Warm() directory provisioning, OfflineEnvironment (GOTOOLCHAIN=local, GOMODCACHE/GOCACHE/GOBIN, GOFLAGS=-mod=readonly) plus AsMap(), and the WailsModule/WailsVersion pin plus WailsBinaryPath() layout helper
  - internal/bootstrap/bootstrap.go — EnsureDirectories shared directory-provisioning primitive, reused by InstallStaged
  - golc.ps1 — repository-local GOBIN wired into Set-ProjectGoEnvironment, an optional cache.gobin manifest override, and up-front warming of every cache directory (downloads/go-mod/go-build/go-bin/manifest) on every bootstrap run
  - tests/acceptance/bootstrap.ps1 — Stage 1 (offline fixture) proves corrupt-pin rejection, corrected retry, and zero-transport idempotent rerun for the generic checksum-pinned tool-archive install; Stage 2 (repository under test) proves the real cache-warm contract and a byte-identical, zero-new-transport second bootstrap
  - Registered bootstrap-cache quick-test scope (TestScopeBootstrapCache), exercised via `golc.ps1 test --quick --scope bootstrap-cache`
affects: [01-06, 01-20, bootstrap, tools-update, foundation-package]

tech-stack:
  added: []
  patterns:
    - "ProjectCacheLayout.Validate() rejects any cache directory that resolves outside Root using the same containment discipline archive.go/bootstrap.go already enforce for extracted archive entries — a hand-edited or corrupted layout can never point outside the checkout."
    - "EnsureDirectories(paths ...string) is the one shared 'make it exist' primitive for both cache-layout warming (cache.go) and staged archive installs (bootstrap.go's InstallStaged), replacing duplicated os.MkdirAll calls."
    - "golc.ps1 mirrors internal/bootstrap/cache.go's OfflineEnvironment field-for-field (GOTOOLCHAIN/GOMODCACHE/GOCACHE/GOBIN/GOFLAGS) as PowerShell script variables and env assignments, following the same intentional PowerShell/Go pin-duplication precedent already established by Test-InstalledManifestMatches mirroring internal/bootstrap.InstalledMatches."
    - "PowerShell array-returning helper functions must use the unary comma operator (`return , @(...)`) — an empty-pipeline `return @(...)` unrolls to zero pipeline objects and is observed by the caller as $null, not an empty array; Mandatory `[string[]]` parameters additionally need [AllowEmptyCollection()] to accept a genuinely empty array argument."

key-files:
  created:
    - internal/bootstrap/cache.go
    - tests/acceptance/bootstrap.ps1
  modified:
    - internal/bootstrap/bootstrap.go
    - internal/bootstrap/bootstrap_test.go
    - golc.ps1

key-decisions:
  - "WailsModule/WailsVersion (github.com/wailsapp/wails/v2/cmd/wails @ v2.13.0) are committed as Go constants in cache.go, not as a config/toolchain.toml pin — config/toolchain.toml is outside this plan's files_modified, and every sibling Phase 1 plan (01-01/01-02/01-15/01-16/01-20) explicitly excludes Wails UI work from this phase. cache.go establishes the GoBin/environment contract a later phase's real `go install` step consumes without redefining GOBIN, module cache, or build cache semantics; no `go install` of Wails runs in this plan (see Known Stubs)."
  - "tests/acceptance/bootstrap.ps1's corrupt/retry/idempotent contract (Stage 1) is proven against a synthetic `[tools.fixture]` entry through golc.ps1's already-existing generic tools.* + Install-ArchivePin path (built in 01-01/01-16), not a real Wails archive — this exercises the identical mechanism a future real Wails pin would use, entirely offline via a file:// URI, with zero real network dependency for that half of the acceptance contract."
  - "Stage 2 of the acceptance script runs golc.ps1 bootstrap twice against the actual repository under test (in place, matching the precedent set by 01-05-SUMMARY.md) to prove the real Go toolchain/module-cache warm and its idempotent zero-transport rerun, since that is the one part of D-01/D-02 this plan can only prove against genuine Go tooling."

requirements-completed: [CONF-01, CONF-03]

coverage:
  - id: D1
    description: "ProjectCacheLayout/OfflineEnvironment set repository-local GOBIN, GOMODCACHE, and GOCACHE plus GOTOOLCHAIN=local, with every cache directory validated as contained inside Root and the exact Wails 2.13.0 pin recorded as a stable Go constant"
    requirement: "CONF-01"
    verification:
      - kind: unit
        ref: "internal/bootstrap/bootstrap_test.go#TestScopeBootstrapCache/NewProjectCacheLayout_returns_every_directory_contained_inside_root"
        status: pass
      - kind: unit
        ref: "internal/bootstrap/bootstrap_test.go#TestScopeBootstrapCache/Validate_rejects_a_layout_whose_directory_escapes_root"
        status: pass
      - kind: unit
        ref: "internal/bootstrap/bootstrap_test.go#TestScopeBootstrapCache/Environment_derives_the_exact_repository-local_Go/Wails_variables"
        status: pass
      - kind: unit
        ref: "internal/bootstrap/bootstrap_test.go#TestScopeBootstrapCache/WailsBinaryPath_and_the_pinned_Wails_module/version_are_exact_and_stable"
        status: pass
    human_judgment: false
  - id: D2
    description: "Warm()/EnsureDirectories provision every cache directory idempotently (a matching second call performs zero destructive action and preserves existing contents)"
    requirement: "CONF-03"
    verification:
      - kind: unit
        ref: "internal/bootstrap/bootstrap_test.go#TestScopeBootstrapCache/Warm_creates_every_cache_directory_and_is_a_safe_idempotent_no-op"
        status: pass
      - kind: unit
        ref: "internal/bootstrap/bootstrap_test.go#TestScopeBootstrapCache/EnsureDirectories_creates_missing_directories_and_rejects_a_path_that_is_already_a_file"
        status: pass
    human_judgment: false
  - id: D3
    description: "golc.ps1 bootstrap warms GOBIN/GOMODCACHE/GOCACHE/downloads/manifest up front, never mutates config/toolchain.toml, go.mod, or go.sum, and a corrupt tool-archive pin fails closed while a corrected retry promotes an install and an archive-source-deleted rerun performs zero transport calls"
    requirement: "CONF-01"
    verification:
      - kind: integration
        ref: "powershell -NoProfile -File .\\tests\\acceptance\\bootstrap.ps1 (Stage 1: corrupt rejection, corrected retry, idempotent rerun)"
        status: pass
    human_judgment: false
  - id: D4
    description: "A matching second bootstrap against the real repository under test performs zero new archive/module transport and leaves the Go module cache, GOBIN, and downloads cache byte-for-byte unchanged, with go.mod/go.sum unmutated throughout"
    requirement: "CONF-03"
    verification:
      - kind: integration
        ref: "powershell -NoProfile -File .\\tests\\acceptance\\bootstrap.ps1 (Stage 2: cache warm plus zero-call/zero-diff second bootstrap)"
        status: pass
    human_judgment: false

duration: ~50min
completed: 2026-07-21
status: complete
---

# Phase 1 Plan 28: Project-Local Cache Layout, Offline Environment, and Idempotent Bootstrap Summary

**ProjectCacheLayout/OfflineEnvironment Go contract (GOBIN/GOMODCACHE/GOCACHE/GOTOOLCHAIN=local, pinned Wails 2.13.0) plus golc.ps1 cache warming, verified end-to-end by `tests/acceptance/bootstrap.ps1`'s corrupt-rejection/correct-retry/cache-warm/idempotent-rerun contract**

## Performance

- **Duration:** ~50 min
- **Started:** 2026-07-21T01:17:00Z
- **Completed:** 2026-07-21T01:27:00Z (implementation); acceptance verification ran through 2026-07-21T01:27:31Z
- **Tasks:** 1 (TDD)
- **Files modified:** 5 (2 created, 3 modified)

## Accomplishments

- `internal/bootstrap/cache.go` establishes the D-01/D-02 project-local cache-layout/offline-environment contract: `ProjectCacheLayout` resolves `Downloads`/`GoModCache`/`GoBuildCache`/`GoBin`/`Manifest` all validated as contained inside `Root` (`Validate()` rejects any escaping path with `BOOTSTRAP_CACHE_ESCAPE`), `Warm()` provisions every directory idempotently without ever touching existing contents, and `Environment()`/`AsMap()` derive the exact `GOTOOLCHAIN=local`/`GOMODCACHE`/`GOCACHE`/`GOBIN`/`GOFLAGS=-mod=readonly` variables every subsequent Go invocation must use. `WailsModule`/`WailsVersion` pin the exact project-local Wails CLI (`github.com/wailsapp/wails/v2/cmd/wails@v2.13.0`) this layout reserves `GoBin` for, and `WailsBinaryPath()` computes where a future install would place it.
- `internal/bootstrap/bootstrap.go` gains `EnsureDirectories(paths ...string) error`, the shared directory-provisioning primitive both `cache.go`'s `Warm()` and `InstallStaged`'s parent-directory creation now use, replacing two independent `os.MkdirAll` call sites with one tested helper (`BOOTSTRAP_CACHE_DIRECTORY` diagnostic on failure).
- `internal/bootstrap/bootstrap_test.go` registers quick-test scope `bootstrap-cache` beside `TestScopeBootstrapCache`, covering: every cache directory is distinct and contained inside `Root`; an empty root is rejected; a layout with an escaping directory is rejected; `Warm()` creates all five directories and a canary file survives a second `Warm()` call; `Environment()`/`AsMap()` return the exact expected values; `WailsModule`/`WailsVersion`/`WailsBinaryPath()` are exact and stable; and `EnsureDirectories` creates nested directories idempotently while rejecting a path that already exists as a file.
- `golc.ps1` adds a repository-local `$GoBinDirectory` (`.tools\cache\go-bin`), wires `$env:GOBIN` into `Set-ProjectGoEnvironment` alongside the existing `GOTOOLCHAIN`/`GOMODCACHE`/`GOCACHE`/`GOFLAGS`, supports an optional `[cache].gobin` manifest override matching the existing `downloads`/`gomodcache`/`gocache` pattern, and proactively warms every cache directory (`downloads`, `go-mod`, `go-build`, `go-bin`, `manifest`) at the start of every `bootstrap` run — always a safe no-op once the directories exist, and observable via the new `"GOLC bootstrap: project-local cache layout warmed..."` output line.
- `tests/acceptance/bootstrap.ps1` (new) proves the complete corrupt-rejection/correct-retry/cache-warm/idempotent-rerun contract in two stages, entirely from repository-owned fixtures: **Stage 1** (offline, a temporary fixture repository with only `golc.ps1` and a synthetic `[tools.fixture]` archive pin referenced via a `file://` URI) proves a wrong SHA-256 fails closed with no install, correcting the pin makes a retry succeed and warms the downloads cache and promoted install, and deleting the archive source entirely then rerunning bootstrap still succeeds with zero archive-source calls (`InstalledMatches` skip) and a byte-identical install. **Stage 2** (the repository under test, in place) proves `GOBIN`/`GOMODCACHE`/`GOCACHE`/downloads/manifest are created, `go.mod`/`go.sum` are never mutated across two runs, and an immediately repeated bootstrap performs zero new archive/module transport with byte-for-byte identical Go module cache, `GOBIN`, and downloads-cache inventories.
- `powershell -NoProfile -File .\tests\acceptance\bootstrap.ps1` exits 0 from the repository root (both stages); `go build ./...`, `go vet ./...`, and the full `go test ./...` continue to pass across every package. `tests/acceptance/walking-skeleton.ps1 -Mode bootstrap` and `-Mode green` (which exercise the same `golc.ps1` code paths this plan modified) both still pass unaffected.

## Task Commits

TDD gates committed atomically:

1. **RED - Task 1: project-local cache layout and offline environment contract** - `3e42c85` (test)
2. **GREEN - Task 1: ProjectCacheLayout/OfflineEnvironment/Wails pin plus EnsureDirectories** - `3777401` (feat)
3. **Task 1 continuation: golc.ps1 GOBIN/cache warming plus tests/acceptance/bootstrap.ps1** - `7cf76d9` (feat)

RED was verified via `go vet ./internal/bootstrap/...` failing with `undefined: NewProjectCacheLayout` before `cache.go` existed; GREEN was verified via `go test ./internal/bootstrap/... -run TestScopeBootstrapCache -v` (all 7 subtests pass) and the full `go test ./...` continuing to pass.

**Plan metadata:** committed with this summary

## Files Created/Modified

- `internal/bootstrap/cache.go` - `ProjectCacheLayout`, `NewProjectCacheLayout`, `Validate`, `Warm`, `OfflineEnvironment`, `Environment`, `AsMap`, `WailsModule`/`WailsVersion`, `WailsBinaryPath`.
- `internal/bootstrap/bootstrap.go` - Adds `EnsureDirectories`; `InstallStaged` now calls it instead of a direct `os.MkdirAll`.
- `internal/bootstrap/bootstrap_test.go` - Registers scope `bootstrap-cache`/`TestScopeBootstrapCache`; 7 subtests covering the layout/environment/warm/pin/directory-helper contract.
- `golc.ps1` - `$GoBinDirectory`, `GOBIN` in `Set-ProjectGoEnvironment`, `[cache].gobin` override support, up-front cache-directory warming in `Invoke-GolcBootstrap`.
- `tests/acceptance/bootstrap.ps1` - New two-stage acceptance script (offline fixture-archive contract; real-repository cache-warm/idempotence contract).

## Decisions Made

- **Wails pin lives in Go code, not `config/toolchain.toml`:** see Deviations/Known Stubs — `config/toolchain.toml` is outside this plan's `files_modified`, and Phase 1 explicitly excludes Wails UI work everywhere else it is mentioned. `WailsModule`/`WailsVersion` in `cache.go` establish the pin/layout contract without performing a real install.
- **Stage 1 of the acceptance script reuses the existing generic `tools.*` mechanism** (built in 01-01/01-16, `Install-ArchivePin`/`Get-VerifiedArchive`/`Test-InstalledManifestMatches`) with a synthetic fixture tool instead of inventing a parallel code path — this proves the exact mechanism any future real Wails/tool pin will use, entirely offline.
- **PowerShell empty-array return bug (`return , @(...)` plus `[AllowEmptyCollection()]`):** documented as a reusable pattern above; without it, `Get-DirectoryInventory` over an empty `GOBIN` directory (expected on a fresh checkout, since nothing installs a Wails binary yet) silently returned `$null` instead of an empty array, and `Compare-Object`/mandatory `[string[]]` parameter binding both rejected that in confusing ways. Caught and fixed during Stage 2 development before any commit.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] PowerShell `Get-DirectoryInventory` returned `$null` instead of an empty array for an empty directory**
- **Found during:** Task 1, developing `tests/acceptance/bootstrap.ps1` Stage 2 (the real `GOBIN` directory is legitimately empty on a fresh checkout since no tool install writes to it yet)
- **Issue:** `return @(pipeline-with-zero-output)` unrolls to zero pipeline objects in PowerShell, which the caller observes as `$null`; a downstream mandatory `[string[]]` parameter then rejected the (correctly non-null but empty) array with `"it is an empty array"`.
- **Fix:** Used the unary comma operator (`return , @(...)` / `return , $entries`) to force a single array object onto the output stream regardless of element count, and added `[AllowEmptyCollection()]` to `Assert-SameInventory`'s `Before`/`After` parameters.
- **Files modified:** `tests/acceptance/bootstrap.ps1`
- **Verification:** `powershell -NoProfile -File .\tests\acceptance\bootstrap.ps1` (both stages) exits 0.
- **Committed in:** `7cf76d9` (fixed before the first commit of this file; no broken intermediate commit exists)

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Bug was internal to the new acceptance script being written in this same task; no production code was affected. No scope creep.

## Issues Encountered

- This worktree had no bootstrapped pinned toolchain (`.tools/` did not exist), matching the pattern noted in 01-05/01-18-SUMMARY.md. `golc.ps1 bootstrap` hit the same transient `Access to the path ... is denied` staging error during Go-toolchain zip extraction documented in those summaries (Windows real-time file scanning racing newly extracted files); the exact same archive was already checksum-cached from the first attempt, so a second `golc.ps1 bootstrap` invocation succeeded without any new network transport, consistent with prior plans' experience. This is one-time worktree setup noise, not a plan deviation, and the acceptance script's own Stage 2 does not depend on this transient behavior (it asserts on the eventual successful run's output/state, not on any specific attempt count).

## Known Stubs

- **No real Wails CLI install runs anywhere in this plan.** `WailsModule`/`WailsVersion` are committed, tested, stable pin constants and `WailsBinaryPath()` computes their eventual `GOBIN` location, but nothing in `golc.ps1` or `internal/bootstrap` invokes `go install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0`. This is intentional: `config/toolchain.toml` (where a real archive/host/path pin would live, per `01-PATTERNS.md`'s "single authority" allocation) is outside this plan's `files_modified`, and 01-01/01-02/01-15/01-16/01-20-PLAN.md all explicitly exclude Wails UI work from Phase 1. A future phase can wire `WailsModule`/`WailsVersion`/`WailsBinaryPath()`/`ProjectCacheLayout.Environment()` into a real install step (either a `go install` call guarded by a recorded version manifest, or a `config/toolchain.toml`-driven archive pin through the existing `AcquireAndPromote`/`OfficialSourcePolicy` machinery from Plan 01-05) without changing any of this plan's public API.

## Threat Flags

None — this plan extends the existing D-01/T-01-13/T-01-SC cache-manifest and containment mitigations (`ProjectCacheLayout.Validate()`'s path-containment check, `EnsureDirectories`' shared provisioning primitive) without introducing new network, process, or credential surface. No new archive source, host, or credential path is added; the Wails pin is inert data with no install code path yet.

## User Setup Required

None - everything is repository-local; no credentials, npm, or additional network access involved beyond the one-time toolchain bootstrap already required by prior plans.

## Next Phase Readiness

- `ProjectCacheLayout`/`OfflineEnvironment`/`WailsModule`/`WailsVersion`/`WailsBinaryPath()` are ready for a future phase to wire into a real Wails CLI install (via `go install` or a `config/toolchain.toml` archive pin through Plan 01-05's `AcquireAndPromote`/`OfficialSourcePolicy`) without further layout/environment changes.
- `EnsureDirectories` is available as the shared directory-provisioning primitive for any future cache or install-path addition in `internal/bootstrap`.
- `golc.ps1`'s `[cache].gobin` override and up-front cache-directory warming are ready to consume a future `config/toolchain.toml` `[cache]` section addition without further shim changes.
- Plan 01-06 (offline core-route acceptance) and 01-20 (foundation package) can rely on `GOBIN`/`GOMODCACHE`/`GOCACHE` already being repository-local and warmed by every `bootstrap` invocation.

## Self-Check: PASSED

- Both created files verified present on disk: `internal/bootstrap/cache.go`, `tests/acceptance/bootstrap.ps1` (both `FOUND`).
- Commits `3e42c85` (test), `3777401` (feat), and `7cf76d9` (feat) verified present in `git log --oneline --all`; `git diff --diff-filter=D --name-only 3e42c85~1 7cf76d9` reports zero deleted files across all three commits; working tree clean before this summary.
- `powershell -NoProfile -File .\tests\acceptance\bootstrap.ps1` exits 0 (both stages); the full pinned/host `go test ./...`, `go vet ./...`, `go build ./...` all exit 0 from the repository root. `tests/acceptance/walking-skeleton.ps1 -Mode bootstrap` and `-Mode green` (prior plans' acceptance gates exercising the same `golc.ps1` code this plan modified) also continue to pass unaffected.

---
*Phase: 01-offline-foundation-and-delivery-traceability*
*Completed: 2026-07-21*
