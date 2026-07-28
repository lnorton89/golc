---
phase: 01-offline-foundation-and-delivery-traceability
plan: 02
subsystem: project-configuration
tags: [go, powershell, toml, json, provenance, local-config, quick-tests]

requires:
  - phase: 01-offline-foundation-and-delivery-traceability
    plan: 17
    provides: Self-registering CommandRegistry (MustDeclareRoute/MustDeclareScope), strict root-index/concern loading, green walking skeleton with real pinned CLI
  - phase: 01-offline-foundation-and-delivery-traceability
    plan: 16
    provides: Checksum-controlled bootstrap, pinned project-local Go 1.26.5, GOTOOLCHAIN=local conventions
provides:
  - Contained atomic machine-local persistence (WriteLocal -> golc.local.toml only) with fixed key/value allowlist
  - Two-layer provenance (ResolveRuntime/Explain) reporting winning layer, safe source, and ordered shadowed origins as deterministic JSON
  - Generic quick-test dispatcher `test --quick --scope {name}` with exact TestScope{PascalName} marker discovery and fail-on-zero
  - Self-registered `config set --local` and `config explain {key} --format json` routes
  - Contributor documentation for the full bootstrap/inspect/set/explain/quick-test sequence
affects: [01-03, 01-18, 01-19, project-configuration, command-router]

tech-stack:
  added: []
  patterns:
    - External test packages (package x_test) declare quick-test scopes via command.MustDeclareScope beside their TestScope marker, avoiding import cycles
    - projectconfig is a pure library; all config CLI routes live in internal/command/config.go
    - Fixed-allowlist local writes with deterministic TOML rendering and contained temp-file + rename atomic replacement
    - Provenance JSON via sorted-map encoding/json marshal, allowlisted fields only, trailing newline

key-files:
  created:
    - internal/command/test.go
    - internal/command/config.go
    - internal/projectconfig/local.go
    - internal/projectconfig/local_test.go
    - docs/development.md
  modified:
    - internal/projectconfig/load.go
    - tests/acceptance/walking-skeleton.ps1
    - golc.ps1
    - .gitignore

key-decisions:
  - "Route keys cannot contain dash-prefixed words, so the quick dispatcher registers route 'test' and strictly accepts only the '--quick --scope <name>' form."
  - "The config inspect route moved from load.go into internal/command/config.go so projectconfig never imports command; set/explain handlers call WriteLocal/Explain without an import cycle."
  - "Local writes are double-gated: WriteLocal validates key shape/allowlist/value, and readLocalValues re-validates the on-disk file strictly, so hand-edited unknown or locked keys fail resolution loudly."
  - "Provenance layer names are 'committed' and 'project-local' (D-06 order); explain output carries exactly key/layer/shadowed/source/value and never environment or credentials."

patterns-established:
  - "Stable diagnostics: GOLC_CONFIG_LOCAL_KEY_UNKNOWN, GOLC_CONFIG_LOCAL_KEY_LOCKED, GOLC_CONFIG_LOCAL_KEY_REDIRECT, GOLC_CONFIG_LOCAL_VALUE_INVALID, GOLC_CONFIG_LOCAL_PATH_ESCAPE, GOLC_TEST_USAGE, GOLC_TEST_SCOPE_INVALID, GOLC_TEST_SCOPE_NO_MARKERS, GOLC_TEST_TOOLCHAIN_MISSING."
  - "Quick-test contract: scope name -> TestScope{PascalName}; markers are listed via 'go test -list' before any execution; zero markers is a hard failure."

requirements-completed: [CONF-01, CONF-02, CONF-04]

duration: 10min
completed: 2026-07-20
status: complete
---

# Phase 1 Plan 02: Local Write and Deterministic Provenance Summary

**Contributors can now write runtime.log_level=debug to an ignored golc.local.toml through `config set --local`, read it back cross-process with `config explain` reporting project-local/committed provenance as byte-identical JSON, and run the suite via the new generic `test --quick --scope` dispatcher**

## Performance

- **Duration:** ~10 min
- **Started:** 2026-07-20T18:43:16Z
- **Completed:** 2026-07-20T18:53:13Z
- **Tasks:** 2 (both TDD)
- **Files modified:** 9

## Accomplishments

- `internal/projectconfig/local.go` implements the first real local mutation (D-06/D-07): `WriteLocal` writes only the fixed repository-root `golc.local.toml` through a contained temporary file plus atomic rename, against a fixed key allowlist (`runtime.log_level` with closed value set) and explicit locked-key registry (schema versions, toolchain pins). Path-like keys and `.env` targets fail before any registry lookup; symlinked destinations are rejected via `Lstat`.
- `ResolveRuntime`/`Explain` resolve a canonical key across committed (`config/runtime.toml` via the strict root index) and project-local layers, reporting winning layer, safe source filename, and ordered shadowed origins as deterministic sorted-key JSON with a trailing newline — exactly the fields `key`, `layer`, `shadowed`, `source`, `value`, never environment or credentials (T-01-05).
- `internal/command/test.go` self-registers the generic quick dispatcher: safe scope names only, exact `TestScope{PascalName}` marker derivation, marker listing through the pinned project-local toolchain (`go test -list`) before execution, hard failure on zero markers, and execution restricted to the packages that declared the marker. The committed Go pin is shape-validated before path join (T-01-SC).
- `internal/command/config.go` is now the single CLI surface over projectconfig: `config inspect` (moved from load.go), plus new `config set --local <key> <value>` and `config explain <key> --format json`, all self-registered — `internal/command/router.go` untouched.
- Green walking skeleton extended: after the byte-identical inspect assertions, one process writes the local value, `golc.local.toml` existence is asserted, and two fresh-process explains must be byte-identical with full provenance and field-allowlist checks. `powershell -NoProfile -File .\tests\acceptance\walking-skeleton.ps1 -Mode green` exits 0.
- `docs/development.md` documents the repository-root bootstrap/inspect/set/explain/quick-test sequence and states the Phase 1 adaptation (CLI -> Go registry -> TOML) with explicit Wails UI, SQLite show storage, and NSIS packaging exclusions.

## Task Commits

TDD gates committed atomically:

1. **RED - Task 1: local write/provenance/scope contract** - `1d4057c` (test)
2. **GREEN - Task 1: WriteLocal/ResolveRuntime/Explain, quick dispatcher, config command file, ignores** - `d334cc0` (feat)
3. **RED - Task 2: green acceptance demands cross-process write and safe provenance** - `91593ed` (test)
4. **GREEN - Task 2: config set/explain routes and contributor docs** - `23597c5` (feat)
5. **FIX - toolchain pin shape validation before path join** - `903755c` (fix)

**Plan metadata:** committed with this summary

## Files Created/Modified

- `internal/projectconfig/local.go` - Allowlisted contained atomic local write, strict local-file re-validation, two-layer resolution, deterministic safe provenance JSON.
- `internal/projectconfig/local_test.go` - External test package (`projectconfig_test`); declares scope `config-local` via `command.MustDeclareScope` beside marker `TestScopeConfigLocal`; covers persistence across fresh reads, atomic overwrite, unknown/locked/redirect/.env/value rejection, strict hand-edited-file failure, committed-only explain, deterministic allowlisted explain, and symlink rejection (skips where symlinks are unavailable).
- `internal/command/test.go` - Generic `test --quick --scope` dispatcher with safe-name gate, marker listing before execution, fail-on-zero, pinned-toolchain-only execution.
- `internal/command/config.go` - Config command file: scope `config` plus `config inspect`/`config set`/`config explain` routes delegating to projectconfig.
- `internal/projectconfig/load.go` - Now a pure library: route declarations and inspect handler moved out; loading/containment logic unchanged.
- `tests/acceptance/walking-skeleton.ps1` - Green mode asserts local write, cross-process readback, byte-identical explain, provenance fields, and forbidden-content absence.
- `golc.ps1` - Usage line now advertises the `config` scope; delegation unchanged.
- `.gitignore` - Adds exact `/golc.local.toml` beside existing `/.tools/`; `.env` ignore preserved; committed contracts/maps/locks remain trackable.
- `docs/development.md` - Canonical contributor sequence plus Phase 1 boundary.

## Decisions Made

- **Route shape for the dispatcher:** registry route words may not begin with `-`, so the registered route is `test` and the handler accepts exactly `--quick --scope <name>` (dash-prefixed tokens arrive as arguments). The exact user-facing command `test --quick --scope {scope-name}` is unchanged.
- **Import-cycle resolution:** `config set/explain` handlers must call `projectconfig.WriteLocal/Explain`, but projectconfig imported command for its self-registered inspect route. The declarations and handler moved into `internal/command/config.go`; projectconfig is now a pure library. The D-03 contract is preserved — no central switch, router untouched.
- **Test-scope declarations from external test packages:** `local_test.go` is `package projectconfig_test` so it can import command for `MustDeclareScope` without a cycle; this is the pattern for every future Go test scope.
- **Two-sided strictness:** the on-disk local file is re-validated on every read, so a hand-edited `golc.local.toml` with unknown/locked keys or invalid values fails resolution with the same stable diagnostics as a rejected write.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Import cycle between command and projectconfig**
- **Found during:** Task 1 design
- **Issue:** The plan requires `internal/command/config.go` handlers to call `projectconfig.WriteLocal`/`Explain`, but `internal/projectconfig/load.go` imported `internal/command` for its self-registered `config inspect` route — a compile-blocking import cycle.
- **Fix:** Moved the `config` scope declaration, `config inspect` route, and its handler/arg parser from `load.go` into the new `internal/command/config.go` (a file this plan creates anyway). `load.go` was modified although not in `files_modified`; behavior and the registry contract are unchanged, and `cmd/golc-project/main.go` and `internal/command/router.go` were not touched.
- **Files modified:** `internal/projectconfig/load.go`, `internal/command/config.go`
- **Verification:** `go vet ./...`, full test suite, and both route regressions (`config inspect`, router self-registration tests) pass.
- **Committed in:** `d334cc0`

**2. [Rule 2 - Missing critical validation] Toolchain pin shape-validated before path join**
- **Found during:** Post-task threat-surface review
- **Issue:** `resolvePinnedGoExecutable` joined the committed `toolchain.go.version` string into the executable path; a path-shaped value could escape `.tools/`, weakening the T-01-SC "verified project-local toolchain only" mitigation.
- **Fix:** `toolchain.go.version` must match dotted digits (`^[0-9]+(\.[0-9]+)*$`) before any join.
- **Files modified:** `internal/command/test.go`
- **Verification:** vet/tests pass; `test --quick --scope config-local` exits 0 through the rebuilt CLI.
- **Committed in:** `903755c`

**3. [Adaptation] Registered route is `test`, not the literal string `test --quick --scope`**
- **Found during:** Task 1 design
- **Issue:** The registry's normalized route words reject dash-prefixed tokens by design (01-17 contract), so the literal route key `test --quick --scope` cannot exist.
- **Fix:** Route `test` self-registers via `MustDeclareRoute` and its handler accepts only the `--quick --scope <name>` form, rejecting everything else with `GOLC_TEST_USAGE` (exit 2). The user-facing command is exactly as planned.
- **Files modified:** `internal/command/test.go`
- **Committed in:** `d334cc0`

---

**Total deviations:** 1 Rule 3 blocking fix, 1 Rule 2 hardening, 1 route-shape adaptation
**Impact on plan:** No scope change. The cycle fix improves layering (projectconfig is a pure library); all planned behavior and acceptance criteria are intact.

## Issues Encountered

None beyond the deviations above. PowerShell 5.1 passes `--local`, `--quick`, and `--scope` through `ValueFromRemainingArguments` intact, as established in 01-17.

## Known Stubs

- `golc.ps1` usage advertises `check|generate|build|package|linear`, which still fail with stable `GOLC_ROUTE_UNKNOWN` (exit 2) until their owning plans self-register them — the intended D-03 growth path (carried from 01-17).
- `test` without `--quick --scope` (the full-suite form) fails with `GOLC_TEST_USAGE` (exit 2); the full `test` route belongs to a later plan per 01-VALIDATION.
- `config set` supports only the `--local` target; user-level and environment layers arrive with the full D-06 precedence in Plan 01-18.

## Threat Flags

| Flag | File | Description |
|------|------|-------------|
| threat_flag: process-execution | internal/command/test.go | New surface: the dispatcher executes the pinned project-local go.exe. Mitigated in-plan: committed pin only, shape-validated version, fixed path under `.tools/`, `GOTOOLCHAIN=local` enforced in the child environment. |

## User Setup Required

None - everything is repository-local; no credentials, npm, or network access involved.

## Next Phase Readiness

- Plan 01-03 (strict diagnostics) and 01-18 (five-layer precedence) can extend `localKeyRegistry`, `ResolveRuntime`, and `Explain` — the provenance shape (layer/source/shadowed) is already the D-07 contract.
- Every future Go test scope follows the `config-local` pattern: external test package, `MustDeclareScope` beside the exact `TestScope{PascalName}` marker, invoked via `golc.ps1 test --quick --scope {name}`.
- The green walking skeleton now proves the full contributor loop: bootstrap -> inspect -> local set -> cross-process explain -> deterministic bytes.

## Self-Check: PASSED

- All nine created/modified files exist on disk (`internal/command/test.go`, `internal/command/config.go`, `internal/projectconfig/local.go`, `internal/projectconfig/local_test.go`, `internal/projectconfig/load.go`, `docs/development.md`, `tests/acceptance/walking-skeleton.ps1`, `golc.ps1`, `.gitignore`).
- Commits `1d4057c` (test), `d334cc0` (feat), `91593ed` (test), `23597c5` (feat), `903755c` (fix) exist in git history.
- `powershell -NoProfile -File .\golc.ps1 test --quick --scope config-local` and `powershell -NoProfile -File .\tests\acceptance\walking-skeleton.ps1 -Mode green` both exit 0 from the repository root.

---
*Phase: 01-offline-foundation-and-delivery-traceability*
*Completed: 2026-07-20*
