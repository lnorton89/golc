# Deferred Items — Phase 8

Out-of-scope discoveries logged during plan execution (not fixed, per the
SCOPE BOUNDARY rule: only issues directly caused by the current task's
changes are auto-fixed).

## 08-01

- **Pinned Go toolchain not bootstrapped in this worktree.** `.tools/` does
  not exist here, so `TestBuildRouteCompilesTheProductionRepository`,
  `TestBuildablePackagesExcludesMagefiles`, `TestScopeCrossPlatformCI`,
  `TestScopeGreenSubprocess`, and `TestScopeOfflineAcceptance` in
  `internal/command` fail with `GOLC_TEST_TOOLCHAIN_MISSING` / "pinned
  golc-project binary not built (run mage Bootstrap first)". This is a
  pre-existing environment condition (this worktree never ran `mage
  Bootstrap`), unrelated to 08-01's `internal/show/scripts.go` /
  `internal/command/script.go` changes — every other test in
  `internal/show`, `internal/command`, and `internal/routecatalog` passes.
  Not fixed here; run `mage Bootstrap` in this worktree (or verify in a
  bootstrapped environment) before relying on those five tests.

- **`go build ./...` fails on `cmd/golc-desktop`, unrelated to this
  plan.** `cmd\golc-desktop\main.go:28:12: pattern all:frontend/dist: no
  matching files found` — the desktop binary's `//go:embed` directive
  requires a built `frontend/dist` that does not exist in this worktree
  (no frontend build has run here). `go build ./internal/...
  ./cmd/golc-project/...` (the packages 08-01 actually touches) builds
  cleanly. Not fixed here; run the frontend build (or verify in an
  environment where it already ran) before relying on a whole-repo
  `go build ./...`.

## 08-03

- **Pre-existing, out-of-scope toolchain bootstrap failures.** While
  running `go test ./internal/command/... ./internal/scriptsdk/...` after
  wiring `internal/scriptsdk` into `generate`/`check` (08-03-PLAN.md Task 3),
  the same five pre-existing tests failed as under 08-01 because this
  worktree has never run `mage Bootstrap` (`.tools/toolchains` and
  `.tools/installs` do not exist):

  - `TestBuildRouteCompilesTheProductionRepository`
  - `TestBuildablePackagesExcludesMagefiles`
  - `TestScopeCrossPlatformCI/Mage_tests_cross-compile_for_every_configured_contributor_platform`
  - `TestScopeGreenSubprocess`
  - `TestScopeOfflineAcceptance`

  All five require the pinned Go toolchain and/or a pre-built
  `golc-project.exe` under `.tools/`, neither of which this fresh worktree
  checkout has provisioned. This is an environment-setup precondition for
  the whole `internal/command` test suite, unrelated to any change 08-03
  made (confirmed: `.tools/toolchains` and `.tools/installs` are absent from
  disk entirely, not stale/mismatched). Out of scope for this plan's
  task-level auto-fix rules — resolving it means running `mage Bootstrap`, a
  repository-wide provisioning step, not a fix to 08-03's own files.

  `go test ./internal/scriptsdk/...` (the package this plan owns) is fully
  green. `go run ./cmd/golc-project generate` and `go run ./cmd/golc-project
  check --concern project` (which do not require the pinned toolchain, only
  the local `go` already on PATH) both exit 0 with zero drift.

## 08-05

- **Same five pre-existing toolchain-bootstrap failures as 08-01/08-03**
  (`TestBuildRouteCompilesTheProductionRepository`,
  `TestBuildablePackagesExcludesMagefiles`, `TestScopeCrossPlatformCI`,
  `TestScopeGreenSubprocess`, `TestScopeOfflineAcceptance`) — unrelated to
  this plan's `internal/script`/`internal/command/scriptrun.go` changes.
  `go test ./internal/script/...` is fully green; `go test
  ./internal/command/...` is green except these five pre-existing
  failures.

- **Real-Deno-gated tests skip in this worktree.** `.tools/toolchains/deno/`
  contains a partial/unverified install
  (`GOLC_DENO_TOOLCHAIN_MISSING: verified install does not match pin`), so
  every test that spawns a genuine Deno subprocess skips with a clear
  message rather than running for real, per the plan's own instruction
  ("gate the tests that spawn a real Deno process behind a helper that
  skips... when `.tools/toolchains/deno/` is not provisioned"):
  - `internal/script`: `TestRunRemovesTempDirOnSuccess`,
    `TestRunTwoSequentialRunsMintDistinctRunIDs`,
    `TestRunSpawnsDenoWithNoAllowFlagsAndDispatchesSceneActivate`
  - `internal/command`: `TestScriptRunSuccessfulScript`,
    `TestScriptRunThrowingScriptFails`

  Every other 08-05 `<behavior>` bullet these tests would additionally
  exercise for real (Deno's actual filesystem/network/env/subprocess
  denial surface named in the plan's `<verification>` section: a script
  attempting `Deno.readTextFile`, `fetch`, `Deno.env.get`, and
  `Deno.Command` being denied and surfacing as run diagnostics) is
  covered by construction (zero `--allow-*` flags ever passed, asserted
  structurally by `TestDenoCommandLineHasNoAllowFlags`) but was not
  independently observed against a real Deno process in this environment.
  Re-run this plan's full suite, including the skipped tests above and the
  plan's manual Deno-denial-surface check, on a machine where `mage
  Bootstrap` (with a matching Deno pin) has completed successfully.

## 08-07

- **Same five pre-existing toolchain-bootstrap failures as 08-01/08-03/
  08-05** (`TestBuildRouteCompilesTheProductionRepository`,
  `TestBuildablePackagesExcludesMagefiles`, `TestScopeCrossPlatformCI`,
  `TestScopeGreenSubprocess`, `TestScopeOfflineAcceptance`) — unrelated to
  this plan's `internal/script/validate.go` /
  `internal/command/scriptvalidate.go` changes. `go test
  ./internal/script/... ./internal/scriptsdk/...` is fully green; `go test
  ./internal/command/...` is green except these five pre-existing
  failures.

- **Real-Deno-gated tests skip in this worktree**, same partial/unverified
  `.tools/toolchains/deno/` condition 08-05 recorded
  (`GOLC_DENO_TOOLCHAIN_MISSING: verified install does not match pin`):
  - `internal/script`: `TestValidateCleanScriptReportsZeroDiagnostics`,
    `TestValidateWrongFieldTypeReportsDiagnostic`,
    `TestValidateUnknownMethodReportsDiagnostic`
  - `internal/command`: `TestScriptValidateCleanScript`,
    `TestScriptValidateWrongFieldTypeScript`

  Every other `<behavior>`/acceptance-criteria bullet these tests would
  additionally exercise for real (a clean script validating with zero
  diagnostics; `golc.scene.activate({wrongField:1})` and
  `golc.notAMethod()` each producing a real `deno check` type-error
  diagnostic, proving the generated `.d.ts` is actually loaded; the exact
  transcript the plan's acceptance criteria ask to be recorded in this
  SUMMARY) could not be captured in this environment. The size bound and
  the structural zero-import gate are independently proven never to spawn
  a subprocess by `TestValidateModuleGateNeverSpawnsSubprocess` /
  `TestValidateSizeGateNeverSpawnsSubprocess` (real environment, no Deno
  toolchain present, no mock), and the shim-offset math
  (`shimLineOffsetFor`/`parseDenoCheckDiagnostics`) is independently unit
  tested against hand-crafted fixture text that mirrors `deno check`'s
  documented `TS#### [ERROR]: ...` / `at file://...:LINE:COL` diagnostic
  format — but that fixture format was not independently verified against
  a real `deno check` invocation on the pinned Deno version in this
  environment. **Action required before this plan's validate verb is
  trusted in production:** on a machine where `mage Bootstrap` (with a
  matching Deno 2.9.4 pin) has completed, run
  `go test ./internal/script/... ./internal/command/... -count=1` and
  confirm (a) the five skipped tests above pass for real, and (b) the
  `deno check` output `internal/script/validate.go`'s
  `denoCheckDiagnosticHeaderPattern`/`denoCheckDiagnosticLocationPattern`
  regexes assume actually matches what that Deno version emits -- if it
  does not, `parseDenoCheckDiagnostics` will fall back to the generic
  `GOLC_SCRIPT_TYPECHECK_FAILED` diagnostic (still correct/fail-closed,
  but loses per-diagnostic line/column precision) rather than silently
  reporting a clean result.

- **`deno.json`'s `compilerOptions.types` was not independently verified
  against a real `deno check` invocation**, for the same toolchain-
  availability reason. The plan's action step explicitly asks for this
  verification ("verify against the pinned Deno version's behaviour at
  implementation time and record which you used"); `denoCheckConfig` in
  `internal/script/validate.go` documents the choice made
  (`{"compilerOptions":{"types":["./golc.d.ts"]}}`, auto-discovered by
  `deno check` from the checked file's own directory) but this could not
  be confirmed against a live `deno check` process here. If this does not
  work on the pinned Deno version, the single call site to change is
  `denoCheckConfig`'s definition in `internal/script/validate.go` (e.g.
  switching to a `/// <reference path="./golc.d.ts" />` comment prepended
  to the materialized script instead).
