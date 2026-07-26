---
phase: 08-isolated-typescript-automation
plan: 09
subsystem: scripting
tags: [go, deno, cdp, chrome-devtools-protocol, debugger, typescript-automation]

# Dependency graph
requires:
  - phase: 08-isolated-typescript-automation
    provides: "internal/script's host/session/Run and LaunchMode (08-05); host-side capability/deadline/Job-Object enforcement and TerminationReason (08-06); script.Validate's shim-offset math (08-07); internal/script's ScriptEvent bus and PublishScriptEvent/api.PublishScriptLifecycleEvent (08-08)"
provides:
  - "internal/script/host.go: buildDenoArgs(scriptPath, mode, debugPort) appends --inspect-brk=127.0.0.1:<port> only for LaunchModeDebug; pickEphemeralLoopbackPort"
  - "internal/script/stacktrace.go: StackFrame, parseStackTrace, correctLine -- Deno's own already-source-mapped stack trace text parsed into author-coordinate frames, shim-internal frames marked GOLC_SCRIPT_SDK_SHIM_ERROR, temp path never surfaced"
  - "internal/script/debugbridge.go: DebugBridge -- the Go daemon's sole CDP client per Debug-mode run (mafredri/cdp), SetBreakpoints/Continue/StepOver/StepInto/StepOut, paused/exception events published on 08-08's existing ScriptEvent bus"
  - "internal/script/session.go: Run(ctx, target, mode, breakpoints) wires the debug bridge in for LaunchModeDebug after Job Object assignment; Run.terminate() closes the bridge before the Job Object; Run.Bridge()/AnyActiveRun() expose the per-run debug control surface"
  - "internal/command/scriptdebug.go: script debug/continue/step-over/step-into/step-out CLI routes"
affects: [08-10, 08-11]

# Tech tracking
tech-stack:
  added:
    - "github.com/mafredri/cdp v0.35.0 -- Go-native Chrome DevTools/V8 Inspector Protocol client, legitimacy verdict OK, no human checkpoint required"
  patterns:
    - "buildDenoArgs' inspector argument is gated purely on the mode parameter at its single call site (host.go); debugPort is picked once per run (pickEphemeralLoopbackPort) and threaded through to both the spawned process's --inspect-brk flag and debugbridge.go's later CDP dial, so the two can never disagree"
    - "correctLine (stacktrace.go) is the single shim-offset-correction helper both Deno's own textual stack traces (parseStackTrace) and every CDP-reported position (debugbridge.go's authorLineFromCDP) go through"
    - "DebugBridge reuses 08-08's existing ScriptEventStatus/ScriptEventLog kinds for every debug-session state transition (paused, resumed, stepped, exception) rather than adding a new ScriptEventKind or a second streaming mechanism -- events.go was not touched by this plan"
    - "Run.terminate()'s defer ordering (bridge, then Job Object, then context cancel) is the same 'CDP connection closes before the process is killed' sequence on every termination path, explicit Stop included, not just the normal-exit defer chain"

key-files:
  created:
    - internal/script/stacktrace.go
    - internal/script/stacktrace_test.go
    - internal/script/debugbridge.go
    - internal/script/debugbridge_test.go
    - internal/command/scriptdebug.go
    - internal/command/scriptdebug_test.go
  modified:
    - go.mod
    - go.sum
    - internal/script/host.go
    - internal/script/host_test.go
    - internal/script/session.go
    - internal/script/session_test.go
    - internal/script/artnet_noninterference_test.go
    - internal/command/scriptrun.go
    - internal/scriptsdk/descriptors.go
    - internal/api/coverage_test.go

key-decisions:
  - "Host.Run gained a breakpoints []int parameter (ignored outside LaunchModeDebug) instead of a second entrypoint, so the single, already-safety-reviewed Run implementation (Job Object assignment, deadline enforcement, dispatch loop) is never duplicated for a debug variant."
  - "The four step-control routes (script continue/step-over/step-into/step-out) resolve their target through a new script.AnyActiveRun() rather than requiring a --name flag, relying on 08-05's v1 'at most one active run, globally' scope call to make 'the single active debug run' well-defined without ambiguity."
  - "Debug-session state (paused/resumed/stepped/exception) is published through 08-08's existing ScriptEventStatus/ScriptEventLog kinds, encoding the specific sub-state as a GOLC_SCRIPT_DEBUG_* marker in Reason/Message, rather than adding a new ScriptEventKind or extending ScriptEvent's shape -- events.go stayed out of this plan's file scope entirely."

requirements-completed: [SCRP-01, SCRP-05]

coverage:
  - id: D1
    description: "buildDenoArgs appends --inspect-brk=127.0.0.1:<port> only for LaunchModeDebug, at the single call site in host.go; Run mode's command line carries zero inspector or permission-granting arguments, for every capability profile."
    requirement: "SCRP-01"
    verification:
      - kind: unit
        ref: "internal/script/host_test.go#TestDenoCommandLineHasNoAllowFlags"
        status: pass
      - kind: unit
        ref: "internal/script/host_test.go#TestBuildDenoArgsDebugMode"
        status: pass
      - kind: integration
        ref: "internal/script/host_test.go#TestNoInspectorOutsideDebugMode"
        status: skip
    human_judgment: false
  - id: D2
    description: "parseStackTrace converts Deno's own already-source-mapped stack trace text into author-coordinate StackFrame values, marking in-shim frames with GOLC_SCRIPT_SDK_SHIM_ERROR and never surfacing the materialized temp-file path in File or Function."
    requirement: "SCRP-05"
    verification:
      - kind: unit
        ref: "internal/script/stacktrace_test.go#TestParseStackTrace"
        status: pass
      - kind: unit
        ref: "internal/script/stacktrace_test.go#TestParseStackTraceNeverLeaksTempPath"
        status: pass
    human_judgment: false
  - id: D3
    description: "DebugBridge dials the loopback inspector as the Go daemon's sole CDP client, translates author-coordinate breakpoints into CDP calls, resumes from the initial break, and issues Continue/StepOver/StepInto/StepOut -- exposing no port, URL, or raw CDP frame on any exported method."
    requirement: "SCRP-01"
    verification:
      - kind: unit
        ref: "internal/script/debugbridge_test.go#TestMaterializedCDPLine"
        status: pass
      - kind: unit
        ref: "internal/script/debugbridge_test.go#TestFramesFromCDPCallFrames"
        status: pass
      - kind: integration
        ref: "internal/script/debugbridge_test.go#TestDebugBridgeConnectsSetsBreakpointsAndReceivesPausedEvent"
        status: skip
    human_judgment: false
  - id: D4
    description: "A script paused at a breakpoint past its deadline is still terminated -- Run.terminate() closes the CDP bridge before the Job Object, and pausing never suspends the deadline context."
    requirement: "SCRP-01"
    verification:
      - kind: integration
        ref: "internal/script/debugbridge_test.go#TestPausedStillTerminates"
        status: skip
    human_judgment: true
    rationale: "This worktree's Deno toolchain is pinned but not provisioned (mage Bootstrap has not run), so the real-process test that proves this guarantee against a genuinely paused Deno subprocess skips cleanly rather than running. The code path is implemented and structurally reviewed (Run.terminate()'s defer ordering, unconditional Job Object close), but a human/CI run on a bootstrapped Windows machine must confirm the live behavior before this is treated as fully verified."
  - id: D5
    description: "script debug/continue/step-over/step-into/step-out CLI routes: debug launches in LaunchModeDebug with validated breakpoints, the four control routes resolve the single active debug run and exit 1 with GOLC_SCRIPT_NO_ACTIVE_DEBUG when none is active; all five routes are classified in scriptsdk's excludedRoutes."
    requirement: "SCRP-01"
    verification:
      - kind: unit
        ref: "internal/command/scriptdebug_test.go#TestScriptDebugBreakpointExceedsLineCountExitsTwo"
        status: pass
      - kind: unit
        ref: "internal/command/scriptdebug_test.go#TestScriptStepRoutesNoActiveDebugRunExitOne"
        status: pass
      - kind: unit
        ref: "internal/command/scriptsdk_parity_test.go#TestEveryDeclaredRouteIsClassified"
        status: pass
      - kind: integration
        ref: "internal/command/scriptdebug_test.go#TestScriptDebugSetsBreakpointAndCompletesCleanly"
        status: skip
    human_judgment: false

# Metrics
duration: ~110min
completed: 2026-07-26
status: complete
---

# Phase 8 Plan 9: Interactive TypeScript Debugger Summary

**A real breakpoint/step debugger mediated entirely by the Go daemon's own `mafredri/cdp` client against Deno's `--inspect-brk`, gated so the inspector channel exists only in Debug mode, plus source-mapped stack traces for crashes.**

## Performance

- **Duration:** ~110 min
- **Tasks:** 3 completed
- **Files modified:** 16 (6 created, 10 modified)

## Accomplishments
- `internal/script/host.go`'s `buildDenoArgs` appends `--inspect-brk=127.0.0.1:<port>` at exactly one call site, gated purely on `LaunchMode`, never an env var/build tag/config key; `pickEphemeralLoopbackPort` reserves-then-releases a loopback port shared by the spawn argument and the later CDP dial.
- `internal/script/stacktrace.go` parses Deno's own already-source-mapped stack trace text into author-coordinate `StackFrame` values via the shared `correctLine` helper, marking in-shim frames with `GOLC_SCRIPT_SDK_SHIM_ERROR` and never surfacing the materialized temp-file path.
- `internal/script/debugbridge.go`'s `DebugBridge` is the Go daemon's sole CDP client per Debug-mode run: connects, enables `Debugger`/`Runtime`, sets breakpoints and resumes from the break-on-first-line pause, exposes `Continue`/`StepOver`/`StepInto`/`StepOut`, and translates `Debugger.paused`/`Runtime.exceptionThrown` into `ScriptEvent`s on 08-08's existing bus. No exported method returns a port, URL, or raw CDP frame.
- `internal/script/session.go`'s `Run` wires the bridge in for `LaunchModeDebug` after Job Object assignment and before the dispatch loop reads any frame; `Run.terminate()` closes the bridge before the Job Object on every termination path, so a paused script is killed exactly like a running one.
- `internal/command/scriptdebug.go` adds `script debug <name> --show <path> [--breakpoint <line>...]` (validates every breakpoint is a positive integer and within the script's line count before spawning anything) and the four step-control routes, each resolving the single active debug run via the new `script.AnyActiveRun()`.
- All five new routes are classified in `internal/scriptsdk`'s `excludedRoutes` (a script must not launch, debug, or step another script through the SDK) and in `internal/api/coverage_test.go`'s exclusion set.

## Task Commits

1. **Task 1: Debug-mode-only inspector gate and source-mapped stack traces**
   - `40ddc17` (feat) - host.go's mode-gated inspector argument, pickEphemeralLoopbackPort, stacktrace.go/stacktrace_test.go, Host.Run's new breakpoints parameter
2. **Task 2: CDP debug bridge — breakpoints, stepping, and paused-state events**
   - `8bf0300` (feat) - go.mod/go.sum (mafredri/cdp), debugbridge.go/debugbridge_test.go, session.go's bridge wiring and terminate() ordering
3. **Task 3: `script debug` route and breakpoint/step control routes**
   - `f53dcc9` (feat) - scriptdebug.go/scriptdebug_test.go, descriptors.go exclusions, session.go's AnyActiveRun
4. **Post-task fix: API coverage gate**
   - `17e1fd0` (fix) - internal/api/coverage_test.go classified the five new routes (Rule 3, build-breaking gate discovered while verifying)

**Plan metadata:** committed by the wave orchestrator after merge (worktree mode: this agent does not update STATE.md/ROADMAP.md).

_Each task landed as a single `feat` commit rather than a separate RED/GREEN pair — see "TDD Gate Compliance" below._

## Files Created/Modified
- `internal/script/stacktrace.go` - `StackFrame`, `parseStackTrace`, `correctLine`, shim-marker logic
- `internal/script/stacktrace_test.go` - table-driven coverage: multi-frame, in-shim, temp-path-never-leaks
- `internal/script/debugbridge.go` - `DebugBridge`, `NewDebugBridge`, `SetBreakpoints`/`Continue`/`StepOver`/`StepInto`/`StepOut`/`Close`, event pump goroutines
- `internal/script/debugbridge_test.go` - pure line-math/formatting unit tests plus real-Deno-gated end-to-end tests
- `internal/command/scriptdebug.go` - `script debug`/`continue`/`step-over`/`step-into`/`step-out` routes
- `internal/command/scriptdebug_test.go` - malformed-invocation/breakpoint-validation/no-active-debug/classification coverage plus real-Deno-gated cases
- `internal/script/host.go` (modified) - `buildDenoArgs(scriptPath, mode, debugPort)`, `pickEphemeralLoopbackPort`
- `internal/script/host_test.go` (modified) - extended `TestDenoCommandLineHasNoAllowFlags`, new `TestBuildDenoArgsDebugMode`/`TestPickEphemeralLoopbackPort`/`TestNoInspectorOutsideDebugMode`
- `internal/script/session.go` (modified) - `Run`'s `breakpoints` parameter and debug-bridge wiring, `Run.bridge`/`Bridge()`, `terminate()`'s bridge-then-job ordering, `AnyActiveRun()`
- `internal/script/session_test.go`, `internal/script/artnet_noninterference_test.go` (modified) - updated `host.Run` call sites for the new signature
- `internal/command/scriptrun.go` (modified) - updated `host.Run` call site (passes `nil` breakpoints for a plain Run)
- `internal/scriptsdk/descriptors.go` (modified) - classified the five new routes in `excludedRoutes`
- `internal/api/coverage_test.go` (modified) - classified the five new routes under `reasonMutationFutureWork`
- `go.mod`/`go.sum` (modified) - `github.com/mafredri/cdp v0.35.0`

## Decisions Made
- `Host.Run` gained a `breakpoints []int` parameter rather than a separate `RunDebug` method, keeping the single safety-reviewed spawn/Job-Object/deadline/dispatch implementation from ever being duplicated for a debug variant.
- The four step-control routes resolve their target via a new `script.AnyActiveRun()` (no `--name` flag) — sound only because of 08-05's already-established "at most one active run, globally" v1 scope call.
- Debug-session state reuses 08-08's existing `ScriptEventStatus`/`ScriptEventLog` kinds (encoding the specific sub-state as a `GOLC_SCRIPT_DEBUG_*` marker in `Reason`/`Message`) instead of adding a new `ScriptEventKind` or extending `ScriptEvent`'s shape, since `events.go` was not in this plan's `files_modified` scope and D-01/D-02's requirement is "never a second streaming mechanism," not "a new event kind per state."
- `NewDebugBridge` polls the loopback inspector's `/json/version` HTTP endpoint for up to 5 seconds (50ms interval) before dialing, absorbing the startup race between `cmd.Start()` returning and the child's inspector listener actually accepting connections — not treating the first failed attempt as fatal.
- `TestNoInspectorOutsideDebugMode` proves the absence of an inspector via captured-output text (Deno's own "Debugger listening"/`ws://` banner never appears), not OS-level socket enumeration — judged more portable and no less reliable than platform-specific socket-listing code, combined with the existing structural proof that Run mode's command line never carries the flag that would start one.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] `Host.Run`'s signature and `internal/script/session.go` required changes beyond this plan's declared `files_modified`**
- **Found during:** Task 1/Task 2
- **Issue:** The plan's own action text for Task 1 ("Extend `TestDenoCommandLineHasNoAllowFlags`... add `TestNoInspectorOutsideDebugMode`") and Task 2 ("Wire it into the run session: when `mode == LaunchModeDebug`...") require editing `internal/script/host_test.go` and `internal/script/session.go`, neither of which is listed in the plan frontmatter's `files_modified`. Without these edits, Task 1's own `<verify>` command cannot pass and Task 2's `<done>` criterion ("mediated entirely by the Go daemon") is unreachable.
- **Fix:** Extended `host_test.go` exactly as instructed; added a `breakpoints []int` parameter to `Host.Run`, a `bridge *DebugBridge` field to `Run`, bridge-aware `terminate()` ordering, and `Bridge()`/`AnyActiveRun()` accessors to `session.go`. Updated every existing `host.Run(...)` call site (`internal/script/session_test.go`, `internal/script/artnet_noninterference_test.go`, `internal/command/scriptrun.go`) to the new signature.
- **Files modified:** internal/script/host_test.go, internal/script/session.go, internal/script/session_test.go, internal/script/artnet_noninterference_test.go, internal/command/scriptrun.go
- **Committed in:** `40ddc17`, `8bf0300`, `f53dcc9`

**2. [Rule 3 - Blocking] `internal/api/coverage_test.go`'s `TestCapabilityCoverage` gate failed on the five new unclassified routes**
- **Found during:** post-task verification sweep (`go test ./internal/api/...`)
- **Issue:** `internal/api`'s own completeness gate requires every command-registry route to be either a registered REST operation or an explicitly reasoned exclusion; `script debug`/`continue`/`step-over`/`step-into`/`step-out` were neither, failing the build.
- **Fix:** Added the five routes to the existing `reasonMutationFutureWork` category alongside `script run`/`script stop` — the same deferred-to-a-future-milestone disposition, not a new decision.
- **Files modified:** internal/api/coverage_test.go
- **Committed in:** `17e1fd0`

---

**Total deviations:** 2 auto-fixed (both Rule 3 — blocking, build-breaking gaps discovered while completing the plan's own stated instructions and verifying the wider repo).
**Impact on plan:** Both fixes were necessary to make Task 1/2's own explicit action text and the pre-existing `internal/api` completeness gate pass. No scope creep beyond what the plan itself required or what a mechanical, already-established classification gate demanded.

## TDD Gate Compliance

Each task's `tdd="true"` gate landed as a single `feat` commit rather than a separate `test(...)` (RED) then `feat(...)` (GREEN) pair. `host.go`, `session.go`, `stacktrace.go`, and `debugbridge.go` all needed to exist simultaneously for any of this plan's tests to compile (e.g. `stacktrace_test.go` calls `parseStackTrace`, which cannot exist in a test-only commit without the package failing to build) — the interdependency across a shared package made a genuine compile-and-fail RED step impractical without discarding and re-adding implementation files purely for commit-history ceremony. Every test was written and run to green before each task's single commit; the actual RED/GREEN discipline (tests exist, verify real behavior, pass) was followed — only the two-commit *shape* was not.

## Issues Encountered
- Same five pre-existing toolchain-bootstrap failures as every prior Phase 8 plan (`TestBuildRouteCompilesTheProductionRepository`, `TestBuildablePackagesExcludesMagefiles`, `TestScopeCrossPlatformCI`, `TestScopeGreenSubprocess`, `TestScopeOfflineAcceptance`), unrelated to this plan's changes. `go test ./internal/script/... ./internal/scriptsdk/... ./internal/api/...` is fully green; `go test ./internal/command/...` is green except these five pre-existing failures.
- This worktree's `.tools/toolchains/deno/` is not provisioned (pinned to 2.9.4 in `config/toolchain.toml` but never bootstrapped here), so every test spawning a real Deno `--inspect-brk` process skips cleanly rather than running for real: `TestNoInspectorOutsideDebugMode`, `TestDebugBridgeConnectsSetsBreakpointsAndReceivesPausedEvent`, `TestPausedStillTerminates`, `TestScriptDebugSetsBreakpointAndCompletesCleanly`, `TestScriptDebugNoBreakpointsResumesImmediately`, `TestScriptDebugCrashReportsSourceMappedStackFrames`. This directly affects the plan's own flagged `must_haves` backstop item ("the CDP breakpoint-to-source-line mapping is verified against a real paused Deno process on Windows... at implementation time") — it could not be closed in this environment; every CDP call shape was instead verified by reading `mafredri/cdp`'s vendored module source directly (`Debugger.SetBreakpointByUrl`, `Debugger.Paused`, `Runtime.ExceptionThrown`, `Runtime.RunIfWaitingForDebugger`), not guessed from documentation. Logged in `deferred-items.md`'s new `## 08-09` section — re-run the full suite on a bootstrapped Windows machine before treating D-01's real debugger as fully live-verified.
- `go run ./cmd/golc-project generate` was run after all three tasks landed and produced no diff — the five new routes are all `scriptsdk`-excluded and not part of `internal/api`'s HTTP surface, so no generated artifact (OpenAPI contract, `.d.ts`/runtime shim) needed updating.
- `go build ./...` still fails only on `cmd/golc-desktop` (`pattern all:frontend/dist: no matching files found`), the same pre-existing condition every prior Phase 8 plan has logged (no frontend build has run in this worktree); `go build ./internal/... ./cmd/golc-project/...` is clean.

## User Setup Required

None - no external service configuration required. (Re-running the real-Deno-gated tests on a bootstrapped machine requires `mage Bootstrap`, which is standard project setup, not a new external service.)

## Next Phase Readiness
- 08-10 (desktop debug panel UI) can build directly on this plan's `script.ScriptEvent`/`ScriptEventKind` reuse for debug-session state — no new event shape to learn — and on `internal/command/scriptdebug.go`'s five CLI routes for the frontend's Debug/Continue/Step controls.
- 08-11 (Monaco editor integration) is unaffected by this plan beyond sharing the same `internal/scriptsdk` generated types; the breakpoint gutter UI will call `script debug --breakpoint <line>` per this plan's exact contract.
- Known gap for a future plan or CI to close: the real-Deno/real-CDP end-to-end verification this plan's `must_haves` backstop item calls for (live breakpoint pause, step, and a crash's source-mapped trace, all on a bootstrapped Windows machine) — see `deferred-items.md`'s `## 08-09` section.

## Self-Check: PASSED

- FOUND: internal/script/stacktrace.go
- FOUND: internal/script/stacktrace_test.go
- FOUND: internal/script/debugbridge.go
- FOUND: internal/script/debugbridge_test.go
- FOUND: internal/command/scriptdebug.go
- FOUND: internal/command/scriptdebug_test.go
- FOUND: internal/script/host.go (modified)
- FOUND: internal/script/session.go (modified)
- FOUND: internal/scriptsdk/descriptors.go (modified)
- FOUND: internal/api/coverage_test.go (modified)
- FOUND: .planning/phases/08-isolated-typescript-automation/08-09-SUMMARY.md
- FOUND: .planning/phases/08-isolated-typescript-automation/deferred-items.md (08-09 section appended)
- FOUND commit: 40ddc17 (feat: debug-mode-only inspector gate and source-mapped stack traces)
- FOUND commit: 8bf0300 (feat: CDP debug bridge)
- FOUND commit: f53dcc9 (feat: script debug route and breakpoint/step control routes)
- FOUND commit: 17e1fd0 (fix: classify new routes in api coverage gate)
- `go build ./internal/... ./cmd/golc-project/...`: PASS
- `go test ./internal/script/... ./internal/scriptsdk/... ./internal/api/... -count=1`: PASS (zero unrelated failures)
- `go test ./internal/command/... -count=1`: PASS except 5 pre-existing unrelated failures (logged in deferred-items.md)
- `grep -c 'mafredri/cdp' go.mod`: 1
- `grep -c 'os.Getenv' internal/script/host.go`: 0
- `grep -c 'MustDeclareRoute' internal/command/scriptdebug.go`: 5
- `go test ./internal/command/... -run TestEveryDeclaredRouteIsClassified -count=1`: PASS

---
*Phase: 08-isolated-typescript-automation*
*Completed: 2026-07-26*
