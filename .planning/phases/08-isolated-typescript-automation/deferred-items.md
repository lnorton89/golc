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

## 08-06

- **Same five pre-existing toolchain-bootstrap failures as 08-01/08-03/
  08-05** (`TestBuildRouteCompilesTheProductionRepository`,
  `TestBuildablePackagesExcludesMagefiles`, `TestScopeCrossPlatformCI`,
  `TestScopeGreenSubprocess`, `TestScopeOfflineAcceptance`) — unrelated to
  this plan's `internal/script`/`internal/command/scriptstop.go` changes.
  `go test ./internal/script/...` is fully green; `go test
  ./internal/command/...` is green except these five pre-existing
  failures.

- **Real-Deno-gated tests skip in this worktree**, same
  `.tools/toolchains/deno/` partial/unverified install condition 08-05
  already logged:
  - `internal/script`: `TestJobObjectKillsAdversarialDenoChild`,
    `TestScriptKillDoesNotBlockArtnet`
  - `internal/command`: `TestScriptStopTerminatesActiveRun`

  This plan's Windows Job Object *mechanics* (create/configure/assign/
  idempotent Close, and closing a job actually killing a real, live
  process) were independently verified for real on this bootstrapped
  Windows machine against a native (non-Deno) child process
  (`TestJobObjectCreateConfiguresLimitsAndCloses`,
  `TestJobObjectAssignFailsForDeadProcess`,
  `TestJobObjectCloseKillsAssignedProcess` — all pass for real, not
  skipped). What remains unverified in this environment specifically is
  (a) the adversarial Deno child (`finally`/unhandled-rejection/signal-
  listener) kill proof, (b) `TestScriptKillDoesNotBlockArtnet`'s full
  script-run-through-`Host.Run`-against-a-live-`artnet.Worker` proof, and
  (c) `TestScriptStopTerminatesActiveRun`'s end-to-end CLI-route proof —
  all three require a genuine Deno subprocess.

- **08-06-PLAN.md's `<verification>` section's manual real-Windows-
  hardware Pitfall-3 validation was not performed in this environment**:
  "a script with a tight allocating loop is terminated by the memory cap,
  and one with a tight CPU loop is throttled to the configured cap and
  then killed by the deadline. Record both transcripts in the SUMMARY."
  This specific check needs a script actually running through
  `script.Host.Run` (i.e., a provisioned Deno toolchain), which this
  worktree does not have. The underlying Job Object mechanism this check
  would exercise (`JOB_OBJECT_LIMIT_JOB_MEMORY` +
  `JOB_OBJECT_CPU_RATE_CONTROL_HARD_CAP`, both wired with the exact flag
  combination 08-RESEARCH.md Pitfall 3 warns is easy to miss) is unit-
  tested and configured correctly (`TestJobObjectCreateConfiguresLimitsAndCloses`
  asserts both `SetInformationJobObject` calls themselves succeed, which
  requires the correct struct layout and flag combination), and the kill
  path is proven against a real live process. The specific "does the
  configured percentage actually throttle CPU, and does the configured
  megabyte figure actually trigger kernel termination on genuinely
  excessive allocation" transcript is deferred to a machine with `mage
  Bootstrap` (matching Deno pin) completed — re-run this plan's Deno-
  gated tests there and additionally drive a script through `script.Host.Run`
  with a tight CPU-spin script and a tight allocating script under the
  "quick-action" preset's default 25% CPU cap / 256MB memory cap to
  capture the transcripts this section calls for.
