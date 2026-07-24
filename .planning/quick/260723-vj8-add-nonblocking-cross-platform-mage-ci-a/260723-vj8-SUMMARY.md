---
phase: quick-cross-platform-mage-ci
plan: 260723-vj8
subsystem: developer-tooling
tags: [mage, ci, bootstrap, toolchain, cross-platform, tdd]

requires:
  - phase: quick-260723-v4o
    provides: Descriptor-backed closed-world PR workflow parity and the six-target Mage command graph
provides:
  - Five-platform (windows-amd64, linux-amd64, linux-arm64, darwin-amd64, darwin-arm64) checksum-pinned Go and Node archive authority alongside the existing Mage authority
  - Atomic, all-or-nothing tools-update proposals for Go/Node across all five platforms
  - One shared Mage Bootstrap environment option (GOLC_BOOTSTRAP_INCLUDE_LINEAR_SYNC) projected identically through Mage and the read-only MCP server
  - An additive, nonblocking three-runner (windows-latest/ubuntu-latest/macos-latest) observation workflow exercising the real six-target Mage graph
  - A working "build" route: excludes the non-independently-buildable magefiles package from its package sweep, fixing a build break that predates this task
affects: [developer-entrypoint, pr-delivery, powershell-removal, ci]

tech-stack:
  added: []
  patterns: [explicit-platform pin selection seam, closed five-platform strict authority, descriptor-owned environment options, production-root end-to-end route tests]

key-files:
  created:
    - internal/command/cross_platform_ci_test.go
    - .github/workflows/cross-platform-mage.yml
  modified:
    - config/toolchain.toml
    - internal/projectconfig/model.go
    - internal/projectconfig/strict_test.go
    - internal/bootstrap/engine.go
    - internal/bootstrap/engine_test.go
    - internal/bootstrap/engine_linear_test.go
    - internal/command/tools.go
    - internal/command/tools_test.go
    - internal/command/build.go
    - internal/command/build_test.go
    - internal/delivery/mage_targets.go
    - internal/delivery/delivery_test.go
    - magefiles/magefile.go
    - magefiles/magefile_test.go
    - tools/golc-mcp/mage.go
    - tools/golc-mcp/protocol_test.go
    - tools/golc-mcp/README.md

key-decisions:
  - "Both prior Windows-only Go/Node archive records are retained byte-identical; the eight new Linux/Darwin records use the exact URLs and SHA-256 digests specified in the plan (official go.dev/nodejs.org distribution archives) — no metadata was fetched at runtime."
  - "Cross-platform contributor bootstrap availability is explicitly documented as separate from GOLC application platform support: the roadmap (.planning/ROADMAP.md Phase 10) and STATE.md decision log still declare Windows the only qualified/supported v1 platform; this task only proves the Go-native tooling graph runs on all three OSes."
  - "The ambient `go install github.com/magefile/mage@v1.17.2` step in the observation workflow is a deliberate, narrow exception to config/toolchain.toml's checksum-pin authority: it installs mage through the Go module system (GOSUMDB-verified) purely to launch repository-owned Mage targets; project bootstrap remains the sole authority for installing/verifying project-local Go, Node, and Mage toolchains."
  - "GOLC_BOOTSTRAP_INCLUDE_LINEAR_SYNC is owned by one shared descriptor in internal/delivery, consumed identically by both Mage Bootstrap and the bootstrap step inside Mage Pr, and projected read-only through the MCP server — no second copy of the option name/value/effect exists anywhere."
  - "runBuild's bare package sweep excludes the magefiles import path (buildablePackages in internal/command/build.go): magefiles/magefile.go intentionally has no func main() (mage supplies its own generated main when it compiles targets), so it was never an independently buildable package and plain `go build ./...` broke for the whole module the moment magefiles/ was added. This had gone undetected across six prior GREEN commits because no test ever ran the bare build route end to end against the real repository; TestBuildRouteCompilesTheProductionRepository and TestBuildablePackagesExcludesMagefiles close that coverage gap."

requirements-completed: [CONF-01, CONF-02, CONF-03, CONF-04]

coverage:
  - id: D1
    description: "Go and Node each have exact five-platform checksum-pinned archive records, and strict configuration/bootstrap/tools-update treat all five as one closed atomic authority."
    requirement: CONF-01
    verification:
      - kind: unit
        ref: "go test ./internal/projectconfig ./internal/bootstrap ./internal/command -run 'TestScope(ConfigStrict|BootstrapEngine|BootstrapLinearSync|ToolsUpdate)$' -count=1"
        status: pass
    human_judgment: false
  - id: D2
    description: "One shared Mage Bootstrap environment option enables the pinned Linear/Node path identically for direct Bootstrap and the bootstrap step inside Pr, and is projected read-only through MCP without a second authority."
    requirement: CONF-02
    verification:
      - kind: unit
        ref: "go test ./internal/delivery ./magefiles ./tools/golc-mcp -count=1"
        status: pass
    human_judgment: false
  - id: D3
    description: "A new nonblocking three-runner workflow runs the exact six existing Mage targets in the documented order, and the existing check.yml parity workflow is untouched."
    requirement: CONF-03
    verification:
      - kind: unit
        ref: "go test ./internal/command -run '^TestScope(CrossPlatformCI|CommandParity)$' -count=1"
        status: pass
    human_judgment: false
  - id: D4
    description: "The observation workflow is read-only, secret-free, and carries no Linux/macOS support or qualification claim; the bare build route (a prerequisite for every Mage target) works end to end on the production repository."
    requirement: CONF-04
    verification:
      - kind: regression
        ref: "go test ./... -count=1"
        status: pass
      - kind: unit
        ref: "internal/command/build_test.go#TestBuildRouteCompilesTheProductionRepository"
        status: pass
    human_judgment: false

duration: unresumed (rate-limited mid-wrap-up; finished in a follow-up session)
completed: 2026-07-23
status: complete
---

# Quick Plan 260723-vj8: Nonblocking Cross-Platform Mage CI Summary

**Five-platform Go/Node/Mage archive authority, one shared pinned-Node bootstrap option, and an additive nonblocking three-OS Mage observation workflow — plus a build-route regression fix discovered while verifying the finished work.**

## Performance

- **Tasks:** 3 (per plan) + 1 unplanned regression fix found during finishing verification
- **Files modified:** 17 (16 per plan + build.go/build_test.go for the regression fix)
- **Completed:** 2026-07-23

## Accomplishments

- Extended `config/toolchain.toml`'s Go and Node authority from Windows-only to five checksum-pinned platforms each (windows-amd64, linux-amd64, linux-arm64, darwin-amd64, darwin-arm64), with `internal/projectconfig/model.go`'s strict registry, `internal/bootstrap`'s explicit-platform selector, and `internal/command/tools.go`'s update-proposal engine all agreeing on one closed five-platform set per tool — no partial-platform state is representable.
- Added one shared `GOLC_BOOTSTRAP_INCLUDE_LINEAR_SYNC` environment option, owned by a single descriptor in `internal/delivery`, consumed identically by `mage Bootstrap` and the bootstrap step inside `mage Pr`, and projected read-only through `tools/golc-mcp`.
- Added `.github/workflows/cross-platform-mage.yml`: an additive, `continue-on-error: true`, `fail-fast: false` matrix job on `windows-latest`/`ubuntu-latest`/`macos-latest` that installs mage via ambient `go install`, then runs the same six Mage targets (`Bootstrap`, `GenerateCheck`, `CheckOffline`, `Build`, `Test`, `PackageFoundation`) the blocking `check.yml` parity workflow already enforces on Windows — proven structurally and via 5-platform cross-compilation plus native Windows execution in `internal/command/cross_platform_ci_test.go`.
- **Found and fixed while verifying the finished work**: `magefiles/magefile.go` has no `func main()` by design (mage supplies its own generated main when compiling targets), which meant plain `go build ./...` — exactly what the `build` route and thus `mage Build` runs — failed on every OS since the package was first added, undetected across six prior commits because no test exercised the bare build route end to end. Fixed by excluding the magefiles import path from `runBuild`'s package sweep (`buildablePackages` in `internal/command/build.go`), with two new regression tests (`TestBuildRouteCompilesTheProductionRepository`, `TestBuildablePackagesExcludesMagefiles`) that run the real route against the actual repository root so this class of gap can't recur silently.

## Task Commits

1. **Task 1 RED: platform authority failure coverage** - `51fe708`
2. **Task 1 GREEN: five-platform Go/Node archive authority** - `e364af6`
3. **Task 2 RED: Mage bootstrap option failure coverage** - `bce2bbf`
4. **Task 2 GREEN: shared pinned-Node bootstrap option** - `cf2352f`
5. **Task 3 RED: cross-platform CI contract failure coverage** - `cdb284e`
6. **Task 3 GREEN: nonblocking Mage observation matrix** - `abcc0aa`
7. **Regression RED: failing build-route coverage** - `e5ae690`
8. **Regression GREEN: exclude magefiles from the build sweep** - `f318c3a`

## Files Created/Modified

- `config/toolchain.toml` - Eight new Go/Node platform records (both prior Windows records retained byte-identical); comments distinguish contributor-bootstrap availability from application platform support.
- `internal/projectconfig/model.go` / `strict_test.go` - Closed ten-leaf-per-tool key registry with platform-correct official URL shapes; missing/extra/malformed/wrong-platform input fails strict validation.
- `internal/bootstrap/engine.go` / `engine_test.go` / `engine_linear_test.go` - Explicit `(goos, goarch)` selection seam behind the existing runtime `PlatformKey()` wrapper; missing/mismatched selections cause zero source calls.
- `internal/command/tools.go` / `tools_test.go` - Tools-update proposals carry exactly five platforms per tool, reject partial/extra maps, rewrite all twenty archive fields atomically, and the production default proposal stays a byte-for-byte no-op.
- `internal/delivery/mage_targets.go` / `delivery_test.go` - Bootstrap-only environment-option descriptor with defensive-copy semantics.
- `magefiles/magefile.go` / `magefile_test.go` - One option parser shared by direct `Bootstrap` and the bootstrap step inside `Pr`; invalid values fail before any bootstrap/network/cache effect.
- `tools/golc-mcp/mage.go` / `protocol_test.go` / `README.md` - Read-only projection of the same option metadata; production MCP sources still execute nothing.
- `internal/command/cross_platform_ci_test.go` (new) - Structural workflow contract test plus 5-platform `go test -c ./magefiles` cross-compilation and native Windows execution.
- `.github/workflows/cross-platform-mage.yml` (new) - The additive nonblocking observation workflow.
- `internal/command/build.go` / `build_test.go` - `buildablePackages` excludes `magefiles` from the bare build sweep; two new production-root regression tests.

## Decisions Made

- See `key-decisions` in the frontmatter above; the build-route fix and its rationale are also recorded there since it was discovered, not planned.

## Deviations from Plan

- The plan's three tasks were already committed (RED+GREEN) before this session picked the work back up after a rate-limit interruption; this session's only new work was closing out the plan/summary/state bookkeeping and the unplanned build-route regression (fix + two tests), which was necessary because `mage Build` — and therefore the new workflow's `Build` step this task adds — did not actually work before the fix.

## Issues Encountered

- `go build ./...` failed on every OS due to `magefiles/magefile.go` lacking `func main()` (see Accomplishments/key-decisions). Root cause predates this task (introduced when `magefiles/magefile.go` was first added in quick task 260723-u0p) but was never exercised by a production-root test until this task's own workflow made it observable. Fixed in this session.

## Verification

- `go test ./internal/projectconfig ./internal/bootstrap ./internal/command -run 'TestScope(ConfigStrict|BootstrapEngine|BootstrapLinearSync|ToolsUpdate)$' -count=1` - PASS
- `go test ./internal/delivery ./magefiles ./tools/golc-mcp -count=1` - PASS
- `go test ./internal/command -run '^TestScope(CrossPlatformCI|CommandParity)$' -count=1` - PASS
- `go test ./... -count=1` - PASS (full suite, all packages)
- `go build -o <tmp> ./cmd/golc-project && ./golc-project build` (real bare build route against the production repository, pinned toolchain) - PASS, exit 0
- `git diff --exit-code -- .github/workflows/check.yml golc.ps1 config/commands.toml tests/acceptance` - PASS (unchanged)
- `git diff --exit-code -- go.mod go.sum tools/linear-sync/package.json tools/linear-sync/package-lock.json` - PASS (unchanged)

## Known Stubs

None.

## User Setup Required

None — the new workflow is additive and requires no repository secrets or settings changes to observe.

## Next Phase Readiness

- PowerShell-removal Steps 0-6 and (this task) Step 8's nonblocking observation matrix are complete. Step 7 (deleting `golc.ps1` and retiring every remaining PowerShell reference — the five other `tests/acceptance/*.ps1` scripts, `.github/workflows/linear-sync.yml`, `internal/docgen/docgen.go`'s generated doc text, README/docs references) has not been started and remains the last step before the cutover.
- Recommend running the new `cross-platform-mage.yml` workflow once (via `workflow_dispatch` or a pull request) to get real evidence from hosted `ubuntu-latest`/`macos-latest`/`windows-latest` runners before treating cross-platform contributor tooling as proven — this session only proved it locally on Windows plus 5-platform cross-compilation, not a real run on non-Windows hosted runners.

## Self-Check: PASSED

- All created and modified key files exist.
- All eight RED/GREEN/fix commits exist on `master` and are pushed to `origin/master`.
- Full `go test ./...` passes; the previously-undetected build break is fixed and covered by regression tests.

---
*Plan: 260723-vj8*
*Completed: 2026-07-23*
