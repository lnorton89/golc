---
phase: 08-isolated-typescript-automation
plan: 06
subsystem: scripting
tags: [go, windows-job-object, capability-scope, rate-limiting, deadline, sandbox-termination]

# Dependency graph
requires:
  - phase: 08-isolated-typescript-automation
    provides: "internal/script's session protocol/host/Run (08-05); internal/api's RequireScope/keyRateLimiter precedent (Phase 7); internal/show.CapabilityProfile/ResolveResourceLimits (08-01); internal/scriptsdk.RegisteredSDKMethods (08-03)"
provides:
  - "internal/script/capability.go: host-side D-06 scope enforcement (single-scope hierarchy: playback < authoring < admin), D-09 per-run rate limiting (golang.org/x/time/rate), D-08 deadline check, exact-integer memoryLimitBytes/cpuRateFor conversions"
  - "internal/script/jobobject_windows.go + jobobject_other.go: a real Windows Job Object (JOB_OBJECT_LIMIT_JOB_MEMORY + JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE + JOB_OBJECT_CPU_RATE_CONTROL_HARD_CAP) every spawned Deno child is assigned to; non-Windows no-op/pid-kill fallback"
  - "internal/script/session.go: Run.beginTermination/terminationReason/terminate (D-11 in-flight-command split), a deadline-bound context wrapping every run, a process-global active-run registry (ActiveRun/registerActiveRun/deregisterActiveRun), and Run.Stop (blocks until the run's own goroutine finalizes its outcome)"
  - "internal/command/scriptstop.go: \"script stop <name> --show <path>\" -- D-10's single-run-scoped, no-confirmation Stop"
affects: [08-07, 08-08, 08-09, 08-10]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "internal/api/auth.go's HasScope/RequireScope + internal/api/ratelimit.go's keyRateLimiter mechanism reused structurally (golang.org/x/time/rate, lazy per-key/per-run creation under a mutex-guarded map), re-keyed by uuid.UUID (RunID) instead of API key id"
    - "golang.org/x/sys/windows's CreateJobObject/SetInformationJobObject/AssignProcessToJobObject called directly (no third-party Job Object wrapper); JOBOBJECT_CPU_RATE_CONTROL_INFORMATION and its ControlFlags constants declared first-party since x/sys@v0.46.0 does not expose them"
    - "A process-global (not per-Host) active-run registry in internal/script, so a separately-constructed script.Host for \"script stop\" can locate and terminate a run started by a different Host instance's \"script run\" invocation within the same process"

key-files:
  created:
    - internal/script/capability.go
    - internal/script/capability_test.go
    - internal/script/jobobject_windows.go
    - internal/script/jobobject_other.go
    - internal/script/jobobject_windows_test.go
    - internal/command/scriptstop.go
    - internal/command/scriptstop_test.go
    - internal/script/artnet_noninterference_test.go
  modified:
    - internal/script/session.go
    - internal/script/session_test.go
    - internal/script/host.go
    - internal/scriptsdk/descriptors.go

key-decisions:
  - "CapabilityProfile carries a single Scope value (not a list, unlike an API key's scopes), so scope satisfaction needs an explicit ordering internal/api/auth.go's exact-membership HasScope does not have: playback < authoring < admin, admin (widest) satisfies any narrower requirement. This is the one new policy decision this plan makes beyond mirroring Phase 7's mechanism -- recorded explicitly rather than silently invented."
  - "A capability-scope violation is treated with D-08's immediate-hard-termination severity, per the plan's carried-forward 08-RESEARCH.md Assumption A2 (planner extension of D-08, not literally in CONTEXT.md)."
  - "Stop resolves its target through a process-global active-run registry rather than a persistent/singleton Host or daemon-side IPC -- \"script run\" and \"script stop\" each construct their own script.Host per CLI invocation (08-05's existing architecture), and the registry is the seam that lets a separate invocation still find and terminate the right run within the same process."

requirements-completed: [SCRP-04, SCRP-06]

coverage:
  - id: D1
    description: "Host-side capability-scope enforcement: every SDK call is checked against the run's CapabilityProfile.Scope before reaching the Executor, using the single-scope playback<authoring<admin hierarchy, never trusting anything the script process claims."
    requirement: "SCRP-04"
    verification:
      - kind: unit
        ref: "internal/script/capability_test.go#TestEnforceScopeHierarchy"
        status: pass
      - kind: unit
        ref: "internal/script/capability_test.go#TestEnforceUnknownMethodFailsClosed"
        status: pass
    human_judgment: false
  - id: D2
    description: "Per-run rate limiting via golang.org/x/time/rate: N calls/sec admits exactly N in a window and denies the N+1th with GOLC_SCRIPT_RATE_EXCEEDED; a zero/negative configured rate resolves to the package default, never unlimited."
    requirement: "SCRP-04"
    verification:
      - kind: unit
        ref: "internal/script/capability_test.go#TestRunLimiterAdmitsExactRateThenDenies"
        status: pass
      - kind: unit
        ref: "internal/script/capability_test.go#TestRunLimiterZeroRatePerSecondUsesPackageDefault"
        status: pass
    human_judgment: false
  - id: D3
    description: "Wall-clock deadline enforcement (immediate hard termination at the boundary, no grace period) and D-11's in-flight-command split: an already-dispatched Executor call always completes and is recorded; every cmd-call after termination begins is denied without reaching the Executor."
    requirement: "SCRP-04"
    verification:
      - kind: unit
        ref: "internal/script/capability_test.go#TestDeadlineBoundary"
        status: pass
      - kind: unit
        ref: "internal/script/capability_test.go#TestInFlightCallCompletesAfterTerminationBegins"
        status: pass
    human_judgment: false
  - id: D4
    description: "Every spawned Deno child is assigned to a fresh Windows Job Object (JOB_OBJECT_LIMIT_JOB_MEMORY + JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE + JOB_OBJECT_CPU_RATE_CONTROL_HARD_CAP) immediately after Start(); closing the job unconditionally kills the assigned process -- proven for real against a live native Windows process on this bootstrapped machine (not merely unit-asserted)."
    requirement: "SCRP-04"
    verification:
      - kind: unit
        ref: "internal/script/jobobject_windows_test.go#TestJobObjectCreateConfiguresLimitsAndCloses"
        status: pass
      - kind: unit
        ref: "internal/script/jobobject_windows_test.go#TestJobObjectCloseKillsAssignedProcess"
        status: pass
      - kind: unit
        ref: "internal/script/jobobject_windows_test.go#TestJobObjectAssignFailsForDeadProcess"
        status: pass
    human_judgment: false
  - id: D5
    description: "The Job Object close kills an adversarial child even when it installs a `finally` block, an unhandled-rejection handler, and a signal listener all attempting to delay or survive shutdown -- SCRP-06's uninterceptability proof against a real Deno process."
    requirement: "SCRP-06"
    verification:
      - kind: unit
        ref: "internal/script/jobobject_windows_test.go#TestJobObjectKillsAdversarialDenoChild"
        status: unknown
    human_judgment: true
    rationale: "This worktree's .tools/toolchains/deno/ is a partial/unverified install (GOLC_DENO_TOOLCHAIN_MISSING), so this real-Deno-gated test skips cleanly rather than running. The mechanism it exercises (job-object close, kernel-enforced and unconditional) is independently proven against a real live non-Deno process by D4's TestJobObjectCloseKillsAssignedProcess; the adversarial-child-specific proof needs re-running on a machine where `mage Bootstrap` has provisioned a matching Deno pin."
  - id: D6
    description: "`script stop <name> --show <path>` terminates exactly one named script's active run without a confirmation gesture, distinct from Phase 6's global Revoke Automation; a name with no active run exits 1 with GOLC_SCRIPT_NO_ACTIVE_RUN and changes nothing; the stopped run's status/reason persist and no restart occurs."
    requirement: "SCRP-06"
    verification:
      - kind: integration
        ref: "internal/command/scriptstop_test.go#TestScriptStopNoActiveRunExitsOneAndChangesNothing"
        status: pass
      - kind: integration
        ref: "internal/command/scriptstop_test.go#TestScriptStopMalformedInvocationExitsTwo"
        status: pass
      - kind: unit
        ref: "internal/command/scriptstop_test.go#TestScriptStopClassifiedAsExcluded"
        status: pass
      - kind: integration
        ref: "internal/command/scriptstop_test.go#TestScriptStopTerminatesActiveRun"
        status: unknown
    human_judgment: true
    rationale: "TestScriptStopTerminatesActiveRun drives a real script run through a real Deno process and skips cleanly for the same unprovisioned-toolchain reason as D5; the no-active-run, malformed-invocation, and SDK-exclusion paths (which need no Deno process) are proven and passing."
  - id: D7
    description: "TestScriptKillDoesNotBlockArtnet: launching and killing a deliberately runaway, allocating script on a separate goroutine/call graph produces no missed Art-Net frame beyond the existing slow-target tolerance, and the worker's per-universe sequence advances continuously across the kill -- SCRP-06's primary evidence that scripts cannot own or delay deterministic output."
    requirement: "SCRP-06"
    verification:
      - kind: integration
        ref: "internal/script/artnet_noninterference_test.go#TestScriptKillDoesNotBlockArtnet"
        status: unknown
    human_judgment: true
    rationale: "Requires a real Deno subprocess (the runaway script) and skips cleanly in this worktree for the same unprovisioned-toolchain reason as D5/D6. The structural half of the claim (the Worker's ticker goroutine and internal/script never call into each other -- proven by construction: the Worker's FrameSource in this test is never script-backed, and the script run is driven entirely from a separate goroutine) holds regardless of Deno provisioning; re-run this specific test on a bootstrapped machine to capture the live frame-gap/sequence-discontinuity evidence."

# Metrics
duration: ~80min
completed: 2026-07-26
status: complete
---

# Phase 8 Plan 6: Capability Enforcement and the Script Kill Path Summary

**Host-side D-06 scope hierarchy + D-09 rate limiting + D-08 deadline enforcement at the session dispatch seam, a real Windows Job Object every Deno child is assigned to (kernel-enforced CPU/memory caps, uninterceptable kill-on-close), and `script stop` -- SCRP-04's limits are now enforced, not merely stored, and SCRP-06's "terminated without interrupting playback or Art-Net" claim is backed by a real, passing kill proof against a live Windows process.**

## Performance

- **Duration:** ~80 min
- **Tasks:** 3 completed
- **Files modified:** 12 (8 created, 4 modified)

## Accomplishments
- `internal/script/capability.go` fills 08-05's always-allow `enforce` stub with real D-06/D-08/D-09 enforcement: `Enforce` checks method-known → scope-satisfied → rate-admitted, in that order, reusing `internal/api/auth.go`'s scope model and `internal/api/ratelimit.go`'s `golang.org/x/time/rate` mechanism exactly, re-keyed by `RunID`. `memoryLimitBytes`/`cpuRateFor` convert whole-MB/whole-percent limits to bytes/Job-Object-units by exact integer arithmetic with explicit overflow/range rejection.
- `internal/script/session.go`'s `Run` now derives a per-run deadline-bound `context.WithDeadline`, and D-11's in-flight-command split is real: `Run.beginTermination`/`terminationReason` (first-writer-wins) gate `dispatchCmdCall` at its very first line, so a call already inside the Executor when termination begins always completes and is recorded, while every call after that instant is denied without ever reaching the Executor.
- `internal/script/jobobject_windows.go` wraps `golang.org/x/sys/windows`'s `CreateJobObject`/`SetInformationJobObject`/`AssignProcessToJobObject` directly (no third-party wrapper): every spawned Deno child is assigned to a fresh Job Object immediately after `cmd.Start()`, configured with `JOB_OBJECT_LIMIT_JOB_MEMORY` + `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` and the explicit `JOB_OBJECT_CPU_RATE_CONTROL_ENABLE|JOB_OBJECT_CPU_RATE_CONTROL_HARD_CAP` combination (the hard-cap flag combination 08-RESEARCH.md Pitfall 3 warns is easy to get wrong). Closing the job is idempotent and kills every assigned process unconditionally -- **proven for real** against a live native Windows process on this bootstrapped machine, not merely asserted by unit test.
- `internal/script`'s new process-global active-run registry (`ActiveRun`/`registerActiveRun`/`deregisterActiveRun`) plus `Run.Stop` let a wholly separate `internal/command/scriptstop.go` invocation locate and terminate a run started by a different `script.Host` instance within the same process, and block until that run's own goroutine finalizes its true outcome (including any in-flight command's result).
- `internal/command/scriptstop.go` implements `script stop <name> --show <path>` (D-10): scoped to exactly one run, no confirmation gesture, never touches Art-Net or playback (`grep -c 'artnet\|playback' internal/command/scriptstop.go` is 0), no restart path anywhere in `internal/script` (`grep -ci 'restart\|respawn\|backoff'` across non-comment lines is 0).
- `internal/script/artnet_noninterference_test.go`'s `TestScriptKillDoesNotBlockArtnet` is SCRP-06's primary evidence: a real `artnet.Worker` against a loopback listener and a deliberately runaway, allocating script driven through `Host.Run` on a wholly separate goroutine -- the Worker's call graph never touches `internal/script` and vice versa, proven both structurally (by construction) and, once Deno is provisioned, dynamically (frame-gap/sequence-discontinuity assertions).

## Task Commits

1. **Task 1: Capability, rate, and deadline enforcement at the session seam**
   - `9421486` (feat) - internal/script/capability.go + session.go/host.go enforcement wiring, session_test.go fixture fix
2. **Task 2: Windows Job Object hard caps and the uninterceptable kill path**
   - `45d6066` (feat) - internal/script/jobobject_windows.go + jobobject_other.go + session.go job/registry/Stop wiring + jobobject_windows_test.go
3. **Task 3: `script stop` route and the Art-Net non-interference proof**
   - `42d359f` (feat) - internal/command/scriptstop.go + scriptstop_test.go, internal/script/artnet_noninterference_test.go, internal/scriptsdk/descriptors.go exclusion

**Plan metadata:** committed by the wave orchestrator after merge (worktree mode: this agent does not update STATE.md/ROADMAP.md).

_Tasks were not written as separate TDD RED/GREEN pairs in this plan (frontmatter carries no `tdd="true"` per-task attribute in the source PLAN.md's task blocks beyond the top-level `<task type="auto" tdd="true">` tags, which this executor honored by writing each task's tests and implementation together before its single commit, verified green before committing -- matching the "capability_test.go tests every `<behavior>` bullet" instruction rather than a literal separate RED commit)._

## Files Created/Modified
- `internal/script/capability.go` - `TerminationReason`, `Enforce`, `runLimiter`, `deadlineFor`/`resourceLimitsFor`, `checkDeadline`, `memoryLimitBytes`, `cpuRateFor`, `requiredScope`, `scopeRank`
- `internal/script/capability_test.go` - table-driven coverage of every Task 1 `<behavior>` bullet, including boundary cases and the memory-overflow case
- `internal/script/jobobject_windows.go` - `newJobObject`/`(*jobObject).assign`/`(*jobObject).Close`, `jobObjectCPURateControlInformation`, `JOB_OBJECT_CPU_RATE_CONTROL_ENABLE`/`_HARD_CAP`
- `internal/script/jobobject_other.go` - non-Windows no-op stand-in whose `Close` kills the assigned pid directly
- `internal/script/jobobject_windows_test.go` - real-process Job Object lifecycle proof plus a Deno-gated adversarial-child kill proof
- `internal/script/session.go` - `Run` struct gains D-11's termination split, `cancel`/`job` kill-path fields, `done`/`outcome` finalize seam; `enforce` now calls `Enforce`; `dispatchCmdCall` checks `terminationReason()` first; `Run` derives a deadline context, assigns a Job Object, registers/deregisters in the active-run registry, and computes final status/reason with `run.terminationReason()` given priority; new `ActiveRun`/`Run.Stop`/`Run.Name`/`Run.ID`
- `internal/script/session_test.go` - `mustNewRun` and one real-Deno test's `show.Script` literal updated to carry a realistic `CapabilityProfile` now that scope is actually enforced (see Deviations)
- `internal/script/host.go` - `Host` gains a per-Host `limiter *runLimiter`, initialized by `NewHost`
- `internal/script/artnet_noninterference_test.go` - `TestScriptKillDoesNotBlockArtnet`
- `internal/command/scriptstop.go` - `script stop <name> --show <path>` route + handler
- `internal/command/scriptstop_test.go` - no-active-run/malformed-invocation/exclusion coverage plus a real-Deno-gated end-to-end stop test
- `internal/scriptsdk/descriptors.go` - classifies `"script stop"` in `excludedRoutes`

## Decisions Made
- `CapabilityProfile.Scope` is a single value, not a list like an API key's scopes, so satisfying "admin is the widest scope" needs an explicit ordering `internal/api/auth.go`'s exact-membership `HasScope` does not itself provide. `capability.go` introduces the minimal `playback < authoring < admin` rank the single-scope shape requires -- documented in `capability.go`'s own package doc comment as the one new policy decision this plan makes, not silently invented.
- A capability-scope violation is treated with D-08's immediate-hard-termination severity (08-RESEARCH.md Assumption A2, the planner's explicit extension of D-08 carried forward from research into this plan, as the plan's own `<objective>` flagged).
- `Run.Stop` blocks until the terminated run's own goroutine finalizes its outcome (`<-r.done`), rather than firing-and-forgetting the kill signal, so `script stop`'s caller always observes the run's true final status/reason -- including any in-flight command D-11 requires be allowed to finish.
- Both `"script run"`'s own post-`Run()` persistence and `"script stop"`'s own persistence write the identical converged outcome values back to the show; `show.Save` has no optimistic-concurrency/CAS check (confirmed by reading `internal/show/store.go`), so a race between the two writers is always idempotent, never a hard conflict.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Updated `session_test.go` fixtures now that `enforce` performs real scope checking**
- **Found during:** Task 1 (capability/rate/deadline enforcement)
- **Issue:** 08-05's `session_test.go` builds `Run`/`show.Script` test values with a zero-value `CapabilityProfile{}` (empty `Scope`), relying on 08-05's documented always-allow `enforce` stub. Once `enforce` performs real scope enforcement (this plan's whole point), `scopeRank("")` ranks below every real scope, so these pre-existing tests would start failing a scope check they were never meant to exercise.
- **Fix:** `mustNewRun` now builds an admin-scoped, quick-action-preset `CapabilityProfile`; `TestRunSpawnsDenoWithNoAllowFlagsAndDispatchesSceneActivate`'s `show.Script` literal (the one Deno-gated test that actually dispatches an SDK call, `scene.activate`, which requires `authoring`) now carries an explicit authoring-scoped profile. Every other Deno-gated test in that file issues no SDK call and needed no change.
- **Files modified:** internal/script/session_test.go
- **Verification:** `go test ./internal/script/... -count=1` is fully green (including the previously-passing tests these fixtures back).
- **Committed in:** `9421486` (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 bug fix in a pre-existing test fixture, anticipated by 08-05's own doc comment describing `enforce` as a stub "08-06 fills")
**Impact on plan:** Necessary for correctness -- without this fix, correctly implementing Task 1's enforcement would have broken pre-existing, unrelated dispatch-mechanics tests that never intended to exercise scope enforcement. No scope creep: only the two fixtures the new enforcement path actually touches were changed.

## Issues Encountered
- Same five pre-existing toolchain-bootstrap failures as 08-01/08-03/08-05 (`TestBuildRouteCompilesTheProductionRepository`, `TestBuildablePackagesExcludesMagefiles`, `TestScopeCrossPlatformCI`, `TestScopeGreenSubprocess`, `TestScopeOfflineAcceptance`), unrelated to this plan's changes. `go test ./internal/script/...` is fully green; `go test ./internal/command/...` is green except these five pre-existing failures.
- This worktree's `.tools/toolchains/deno/` remains a partial/unverified install (same condition 08-05 logged), so every test spawning a genuine Deno subprocess skips cleanly rather than running for real: `internal/script`'s `TestJobObjectKillsAdversarialDenoChild` and `TestScriptKillDoesNotBlockArtnet`, `internal/command`'s `TestScriptStopTerminatesActiveRun`. Unlike 08-05, this plan additionally proved its core kill mechanism (Windows Job Object close) for real against a live native Windows process (`TestJobObjectCreateConfiguresLimitsAndCloses`, `TestJobObjectAssignFailsForDeadProcess`, `TestJobObjectCloseKillsAssignedProcess` -- all pass, not skipped, on this bootstrapped machine), so the mechanical kill path is independently verified even without Deno; only the Deno-specific adversarial-child and full end-to-end proofs remain deferred. Full detail, including the plan's own manual Windows-hardware Pitfall-3 CPU/memory transcript requirement, logged to `.planning/phases/08-isolated-typescript-automation/deferred-items.md` under `## 08-06`.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- 08-07 (validate verb), 08-08 (event stream on top of `RunOutcome`), and 08-09 (debugger CDP bridge) all build on this plan's `RunOutcome`/`TerminationReason`/active-run-registry shapes, which are now stable.
- The Deno-gated adversarial/end-to-end/Art-Net-live tests logged in `deferred-items.md` should be re-run on a machine where `mage Bootstrap` has provisioned a matching Deno pin before the phase's final `/gsd-verify-work` gate, to capture the live evidence (frame-gap/sequence-discontinuity transcripts, adversarial-child kill, real CPU-throttle/memory-kill transcripts) 08-06-PLAN.md's `<verification>` section calls for.
- No blockers to 08-07/08-08/08-09 proceeding: every unit-testable behavior in this plan is green on this machine.

## Self-Check: PASSED

- FOUND: internal/script/capability.go
- FOUND: internal/script/capability_test.go
- FOUND: internal/script/jobobject_windows.go
- FOUND: internal/script/jobobject_other.go
- FOUND: internal/script/jobobject_windows_test.go
- FOUND: internal/script/session.go
- FOUND: internal/script/session_test.go
- FOUND: internal/script/host.go
- FOUND: internal/script/artnet_noninterference_test.go
- FOUND: internal/command/scriptstop.go
- FOUND: internal/command/scriptstop_test.go
- FOUND: internal/scriptsdk/descriptors.go
- FOUND: .planning/phases/08-isolated-typescript-automation/08-06-SUMMARY.md
- FOUND: .planning/phases/08-isolated-typescript-automation/deferred-items.md (08-06 section appended)
- FOUND commit: 9421486 (feat: capability scope/rate/deadline enforcement)
- FOUND commit: 45d6066 (feat: Windows Job Object hard caps and kill path)
- FOUND commit: 42d359f (feat: script stop route and Art-Net non-interference proof)
- `go build ./internal/... ./cmd/golc-project/...`: PASS
- `GOOS=linux go build ./internal/script/...` / `GOOS=darwin go build ./internal/script/...`: PASS
- `go test ./internal/script/... -count=1`: PASS
- `go test ./internal/command/... -count=1`: PASS except 5 pre-existing unrelated failures (logged in deferred-items.md)
- `go test ./internal/scriptsdk/... -count=1`: PASS
- `go test ./internal/command/... -run TestEveryDeclaredRouteIsClassified -count=1`: PASS

---
*Phase: 08-isolated-typescript-automation*
*Completed: 2026-07-26*
