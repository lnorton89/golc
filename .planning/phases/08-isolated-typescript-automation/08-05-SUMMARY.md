---
phase: 08-isolated-typescript-automation
plan: 05
subsystem: scripting
tags: [go, deno, subprocess-sandbox, typescript-automation]

# Dependency graph
requires:
  - phase: 08-isolated-typescript-automation
    provides: "show.Script/CapabilityProfile (08-01), script.ResolveDenoExecutable (08-02), scriptsdk generated golc-runtime.ts + RegisteredSDKMethods (08-03)"
provides:
  - "internal/script/protocol.go: multiplexed newline-delimited-JSON cmd-call/cmd-result session protocol"
  - "internal/script/host.go + session.go: zero-permission Deno subprocess host — no --allow-* flags ever, non-inherited environment, redacted+bounded stdio capture"
  - "internal/command/scriptrun.go: script run CLI route and registryExecutor adapter"
affects: [08-06, 08-07, 08-08, 08-09, 08-10]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "internal/trace/transport/process.go's one-shot Call/ProcessConfig/boundedBuffer/security.Redact precedent generalized into a long-lived multiplexed session (not reused as-is, per 08-PATTERNS.md)"
    - "internal/command/artnet.go's apiCommandExecutor adapter precedent reused for scriptrun.go's registryExecutor — internal/script never imports internal/command, no import cycle"
    - "scriptsdk.RegisteredSDKMethods() is the single route+scope lookup a cmd-call frame resolves through; no second route table in internal/script"

key-files:
  created:
    - internal/script/protocol.go
    - internal/script/protocol_test.go
    - internal/script/host.go
    - internal/script/host_test.go
    - internal/script/session.go
    - internal/script/session_test.go
    - internal/command/scriptrun.go
    - internal/command/scriptrun_test.go
  modified: []

key-decisions:
  - "At most one active script run at a time, globally (v1 scope call, surfaced in the plan's own objective) — a second run request is rejected with GOLC_SCRIPT_RUN_ACTIVE, never queued or pre-empted."
  - "The Deno command line built for every run carries zero --allow-* flags of any kind, asserted structurally by TestDenoCommandLineHasNoAllowFlags rather than by inspection — this is the mechanism SCRP-03's ambient-permission prohibition rests on."
  - "The child process receives an explicit, non-inherited environment; the daemon's own env vars are never passed through to a script process."

requirements-completed: [SCRP-01, SCRP-02, SCRP-03]

coverage:
  - id: D1
    description: "Newline-delimited-JSON session protocol multiplexes cmd-call/cmd-result frames over a single stdio pipe, byte-compatible with 08-03's generated golc-runtime.ts shim; an unknown frame kind is a protocol violation (GOLC_SCRIPT_FRAME_UNKNOWN), not a silent ignore."
    requirement: "SCRP-02"
    verification:
      - kind: unit
        ref: "internal/script/protocol_test.go"
        status: pass
    human_judgment: false
  - id: D2
    description: "A saved script runs in a fresh, zero-permission Deno process whose command line never carries a permission-granting flag, whose environment is never inherited, and whose stdout/stderr passes through security.Redact and a bounded drop-oldest buffer before reaching any caller."
    requirement: "SCRP-01, SCRP-03"
    verification:
      - kind: unit
        ref: "internal/script/host_test.go#TestDenoCommandLineHasNoAllowFlags"
        status: pass
      - kind: unit
        ref: "internal/script/host_test.go#TestNewHostFailsClosedWhenDenoMissing"
        status: pass
      - kind: integration
        ref: "internal/script/session_test.go#TestRunSpawnsDenoWithNoAllowFlagsAndDispatchesSceneActivate"
        status: skip
      - kind: integration
        ref: "internal/script/session_test.go#TestRunTwoSequentialRunsMintDistinctRunIDs"
        status: skip
      - kind: integration
        ref: "internal/script/session_test.go#TestRunRemovesTempDirOnSuccess"
        status: skip
    human_judgment: false
  - id: D3
    description: "`golc-project script run <name> --show <path>` drives the host end-to-end through an injected command.CommandRegistry-backed Executor (no internal/script → internal/command import), exits 0 on success with run_id/status/outcomes/logs JSON, and exits with GOLC_SCRIPT_NOT_FOUND / GOLC_SCRIPT_RUN_ACTIVE on the documented failure paths."
    requirement: "SCRP-01"
    verification:
      - kind: integration
        ref: "internal/command/scriptrun_test.go#TestScriptRunNotFoundNeverSpawnsProcess"
        status: pass
      - kind: integration
        ref: "internal/command/scriptrun_test.go#TestScriptRunShowMissingNeverSpawnsProcess"
        status: pass
      - kind: integration
        ref: "internal/command/scriptrun_test.go#TestScriptRunMalformedInvocationExitsTwo"
        status: pass
      - kind: integration
        ref: "internal/command/scriptrun_test.go#TestScriptRunClassifiedAsExcluded"
        status: pass
      - kind: integration
        ref: "internal/command/scriptrun_test.go#TestScriptRunSuccessfulScript"
        status: skip
      - kind: integration
        ref: "internal/command/scriptrun_test.go#TestScriptRunThrowingScriptFails"
        status: skip
    human_judgment: false

# Metrics
duration: ~35min
completed: 2026-07-26
status: complete
---

# Phase 8 Plan 5: Script Execution Core Summary

**`internal/script`'s multiplexed session protocol, zero-permission Deno host, and the `script run` CLI route that drives it — the plan the phase's central safety claim rests on (SCRP-01 run verb, SCRP-02 SDK-only access, SCRP-03 no ambient permissions).**

## Performance

- **Duration:** ~35 min
- **Tasks:** 3 completed
- **Files modified:** 8 (8 created)

## Accomplishments
- `internal/script/protocol.go` decodes/encodes the multiplexed newline-delimited-JSON `cmd-call`/`cmd-result` frame protocol, byte-compatible with 08-03's generated `golc-runtime.ts` shim; an unrecognized frame `kind` is a hard protocol violation (`GOLC_SCRIPT_FRAME_UNKNOWN`), never silently dropped.
- `internal/script/host.go` + `session.go` spawn a fresh Deno subprocess per run via `script.ResolveDenoExecutable` (08-02) with **zero** `--allow-*` flags, a non-inherited explicit environment, and `security.Redact`-wrapped, bounded drop-oldest stdio capture — the OS-process boundary SCRP-03's "no ambient permissions" claim is structurally, not conventionally, enforced at.
- `internal/command/scriptrun.go` wires `script run <name> --show <path>` through a `registryExecutor` adapter (mirroring `artnet.go`'s `apiCommandExecutor` precedent) so `internal/script` never imports `internal/command` — no import cycle — and returns deterministic JSON (`run_id`, `status`, per-call `outcomes`, captured `logs`).
- Enforces v1's "at most one active run" scope call: a second concurrent run request is rejected with `GOLC_SCRIPT_RUN_ACTIVE`, never queued or pre-empted.

## Task Commits

Each task was committed atomically as a TDD RED/GREEN pair (plus one mid-task correctness fix):

1. **Task 1: Newline-delimited JSON session protocol**
   - `7f3ef50` (test) - RED: failing coverage for multiplexed cmd-call/cmd-result framing
   - `be03c5c` (feat) - GREEN: internal/script/protocol.go
   - `2582fc3` (fix) - align generated runtime shim wire frames with protocol.go
2. **Task 2: Zero-permission Deno host and the run session**
   - `87be06c` (test) - RED: failing coverage for the Deno host and run session
   - `c5c70c2` (feat) - GREEN: internal/script/host.go + session.go
   - `db51e78` (fix) - populate run failure reason from stderr regardless of status timing
3. **Task 3: `script run` CLI route and the injected Executor**
   - `7d89fc6` (feat) - internal/command/scriptrun.go: route + registryExecutor adapter (scriptrun_test.go included in the same change)

**Plan metadata:** committed by the wave orchestrator after merge (worktree mode: this agent does not update STATE.md/ROADMAP.md).

_TDD tasks: Tasks 1 and 2 each have a separate RED (test) commit before their GREEN (feat) commit, per the plan's `tdd="true"` gate; Task 3's route and its test landed in one commit._

## Files Created/Modified
- `internal/script/protocol.go` - `CmdCallFrame`/`CmdResultFrame` decode/encode, multiplexed session line framing
- `internal/script/protocol_test.go` - frame decode/encode round-trip and malformed-input coverage
- `internal/script/host.go` - `HostConfig`, zero-permission Deno command-line construction, `NewHost`
- `internal/script/host_test.go` - `TestDenoCommandLineHasNoAllowFlags` (table-driven across every scope/preset), `TestNewHostFailsClosedWhenDenoMissing`
- `internal/script/session.go` - `Run`, `RunOutcome`, per-run temp dir + subprocess lifecycle, bounded+redacted stdio capture
- `internal/script/session_test.go` - real-Deno-gated integration coverage (skips cleanly when `.tools/toolchains/deno/` is unprovisioned)
- `internal/command/scriptrun.go` - `script run` route, `registryExecutor` adapter injecting `command.CommandRegistry` into `internal/script`
- `internal/command/scriptrun_test.go` - not-found/show-missing/malformed-flag/excluded-from-scriptsdk coverage plus real-Deno-gated success/failure paths

## Decisions Made
- v1 supports at most one active script run at a time, globally — makes D-10's "Stop targets one script" unambiguous and keeps the event stream single-writer (08-RESEARCH.md Open Question 1, resolved in the plan's own objective).
- `internal/command/scriptrun.go` injects a `command.CommandRegistry`-backed `Executor` into `internal/script` — the same seam `internal/api/router.go`'s `Executor` interface establishes — so `internal/script` never imports `internal/command` and no cycle exists.
- Every captured stdout/stderr line passes through `internal/security.Redact` and a bounded drop-oldest buffer before it can reach any caller, log, audit row, or event.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Populate run failure reason from stderr regardless of status timing**
- **Found during:** Task 2 (Deno host and run session), while exercising failure-path coverage
- **Issue:** A script that failed after producing partial stdout could report an empty failure reason if the stderr read raced the process-exit status capture.
- **Fix:** `db51e78` — the run outcome's failure reason is now populated from captured stderr independent of exact status-vs-stderr read ordering.
- **Files modified:** internal/script/session.go
- **Committed in:** `db51e78`

---

**Total deviations:** 1 auto-fixed (1 missing critical)
**Impact on plan:** Necessary for correctness against the plan's own stated behavior (failure runs must surface a real reason). No scope creep.

## Issues Encountered
- Same five pre-existing toolchain-bootstrap failures as 08-01/08-03 (`TestBuildRouteCompilesTheProductionRepository`, `TestBuildablePackagesExcludesMagefiles`, `TestScopeCrossPlatformCI`, `TestScopeGreenSubprocess`, `TestScopeOfflineAcceptance`), unrelated to this plan's changes. `go test ./internal/script/...` is fully green; `go test ./internal/command/...` is green except these five pre-existing failures.
- This worktree's `.tools/toolchains/deno/` is a partial/unverified install, so every test spawning a genuine Deno subprocess skips cleanly with `GOLC_SCRIPT_DENO_MISSING` rather than running for real: `internal/script`'s `TestRunRemovesTempDirOnSuccess`, `TestRunTwoSequentialRunsMintDistinctRunIDs`, `TestRunSpawnsDenoWithNoAllowFlagsAndDispatchesSceneActivate`, and `internal/command`'s `TestScriptRunSuccessfulScript`, `TestScriptRunThrowingScriptFails`. The no-`--allow-*`-flags guarantee these tests would additionally observe live is independently covered by construction (`TestDenoCommandLineHasNoAllowFlags`). Logged to `.planning/phases/08-isolated-typescript-automation/deferred-items.md` — re-run the full suite including these skips on a machine where `mage Bootstrap` has completed with a matching Deno pin.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- `internal/script`'s protocol/host/session trio and `script run` are the execution core every remaining Phase 8 plan builds on: 08-06 (capability/termination enforcement at this same process boundary), 08-07 (validate verb reusing the SDK-only surface), 08-08 (event stream on top of `RunOutcome`), 08-09 (debugger CDP bridge on top of the Deno host).
- No blockers. The "at most one active run" constraint is the explicit assumption 08-06's Stop-Script work is built against.

## Self-Check: PASSED

- FOUND: internal/script/protocol.go
- FOUND: internal/script/protocol_test.go
- FOUND: internal/script/host.go
- FOUND: internal/script/host_test.go
- FOUND: internal/script/session.go
- FOUND: internal/script/session_test.go
- FOUND: internal/command/scriptrun.go
- FOUND: internal/command/scriptrun_test.go
- FOUND: .planning/phases/08-isolated-typescript-automation/08-05-SUMMARY.md
- FOUND: .planning/phases/08-isolated-typescript-automation/deferred-items.md (08-05 section appended)
- FOUND commit: 7f3ef50 (test: session protocol RED)
- FOUND commit: be03c5c (feat: session protocol GREEN)
- FOUND commit: 2582fc3 (fix: align generated runtime shim wire frames)
- FOUND commit: 87be06c (test: Deno host/session RED)
- FOUND commit: c5c70c2 (feat: Deno host/session GREEN)
- FOUND commit: db51e78 (fix: run failure reason from stderr)
- FOUND commit: 7d89fc6 (feat: script run CLI route + registryExecutor)
- `go build ./internal/... ./cmd/golc-project/...`: PASS
- `go test ./internal/script/... ./internal/command/...`: PASS (5 pre-existing unrelated failures + 5 real-Deno-gated skips, both logged in deferred-items.md)

---
*Phase: 08-isolated-typescript-automation*
*Completed: 2026-07-26*
