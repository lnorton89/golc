---
phase: 08-isolated-typescript-automation
plan: 14
subsystem: scripting
tags: [go, windows-job-object, deno, typescript, react, vitest]

# Dependency graph
requires:
  - phase: 08-isolated-typescript-automation
    provides: the Job Object sandbox (08-06), the CDP debug bridge and ScriptDebugPanel (08-09/08-10), and the deadline/rate/scope termination pipeline this plan extends with the memory cause
provides:
  - "A proactive Go-side memory-usage monitor (memorywatch.go) that terminates a script through the existing beginTermination/terminate path while it is still alive"
  - "A post-exit classifier (classifyMemoryExhaustion) that reclassifies a V8 out-of-memory crash into the same named reason when the proactive monitor loses the race"
  - "A read-only QueryInformationJobObject query on both platform builds (peakMemoryBytes/jobMemoryLimitBytes)"
  - "The fourth Copywriting Contract branch in ScriptDebugPanel.tsx's describeTermination, rendering 08-UI-SPEC.md:126's exact memory-limit sentence"
affects: [08-isolated-typescript-automation]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "memorySampler seam (memorywatch.go): a single-method interface satisfied by *jobObject on both platform builds, so the polling monitor is fully unit-testable with a fake, with no real Windows Job Object or Deno toolchain involved"
    - "Single reason-text constructor (memoryLimitReason) shared by both the proactive and post-exit code paths, so the GOLC_SCRIPT_MEMORY_EXCEEDED Code/Message text is minted in exactly one place"

key-files:
  created:
    - internal/script/memorywatch.go
    - internal/script/memorywatch_test.go
    - internal/script/memorylimit_windows_test.go
  modified:
    - internal/script/capability.go
    - internal/script/capability_test.go
    - internal/script/jobobject_windows.go
    - internal/script/jobobject_other.go
    - internal/script/session.go
    - internal/script/events_test.go
    - frontend/src/components/Scripts/ScriptDebugPanel.tsx
    - frontend/src/components/Scripts/ScriptDebugPanel.test.tsx

key-decisions:
  - "Built the proactive Go-side memory-usage monitor per the repository owner's explicit decision for this gap-closure run, rather than amending 08-UI-SPEC.md to accept the raw V8 exception as intentional."
  - "Widened v8AllocationFailureSignatures with a fourth signature ('fatal javascript out of memory') beyond the three the plan specified, after real-Deno acceptance testing on this machine showed the same Job Object ceiling collision nondeterministically produces either a catchable RangeError or an uncatchable V8 engine-abort crash."

patterns-established:
  - "memorySampler interface seam for OS-resource-usage polling testable without the real OS resource"
  - "Proactive-monitor-plus-post-exit-classifier pair producing identical user-facing text so which mechanism wins a timing race is never observable to the user"

requirements-completed: [SCRP-04, SCRP-05]

coverage:
  - id: D1
    description: "Proactive Go-side memory monitor terminates a live run through beginTermination/terminate before or as it exhausts its Job Object memory ceiling"
    requirement: "SCRP-04"
    verification:
      - kind: unit
        ref: "internal/script/memorywatch_test.go#TestMemoryWatchTerminatesAboveTrigger"
        status: pass
      - kind: unit
        ref: "internal/script/memorywatch_test.go#TestMemoryWatchNeverOverwritesAnAlreadyRecordedReason"
        status: pass
      - kind: integration
        ref: "internal/script/memorylimit_windows_test.go#TestRunMemoryLimitTerminatesWithNamedReason"
        status: pass
    human_judgment: false
  - id: D2
    description: "A well-behaved script under the default 256 MB preset never suffers a false-positive memory termination"
    requirement: "SCRP-04"
    verification:
      - kind: unit
        ref: "internal/script/capability_test.go#TestCheckMemoryPressureBoundary"
        status: pass
      - kind: integration
        ref: "internal/script/memorylimit_windows_test.go#TestRunWithinMemoryLimitSucceeds"
        status: pass
    human_judgment: false
  - id: D3
    description: "An ordinary uncaught crash under a healthy memory profile still reports failed with its own message, never swallowed by the memory classifier"
    requirement: "SCRP-05"
    verification:
      - kind: unit
        ref: "internal/script/capability_test.go#TestClassifyMemoryExhaustionSignatureAndCorroboration"
        status: pass
      - kind: integration
        ref: "internal/script/memorylimit_windows_test.go#TestRunUnrelatedCrashStillReportsFailed"
        status: pass
    human_judgment: false
  - id: D4
    description: "The Scripts debug panel renders 08-UI-SPEC.md:126's exact Copywriting Contract sentence for a memory-limit kill, with the megabyte figure parsed from the reason rather than hardcoded, and the three previously-working termination causes unchanged"
    requirement: "SCRP-05"
    verification:
      - kind: unit
        ref: "frontend/src/components/Scripts/ScriptDebugPanel.test.tsx#renders the exact memory-limit-termination sentence (D-08)"
        status: pass
      - kind: unit
        ref: "frontend/src/components/Scripts/ScriptDebugPanel.test.tsx#renders the memory-limit-termination sentence with a parsed (not hardcoded) megabyte figure"
        status: pass
    human_judgment: false
  - id: D5
    description: "Live desktop walkthrough: create an Advanced-profile script at 64 MB with a retained allocating loop, click Run, confirm the terminal banner copy and uninterrupted Art-Net output"
    verification: []
    human_judgment: true
    rationale: "Deferred to the repository owner's own GUI pass per the plan's <human-check>, consistent with 08-13-SUMMARY.md item 7 and 08-VERIFICATION.md's Human Verification section (both already owner-accepted deferrals)."

# Metrics
duration: 35min
completed: 2026-07-30
status: complete
---

# Phase 8 Plan 14: Memory-Limit Termination Copy Gap Closure Summary

**Proactive Windows Job Object memory-usage monitor plus a post-exit V8-crash classifier, both minting the single `GOLC_SCRIPT_MEMORY_EXCEEDED` reason text, close 08-VERIFICATION.md's last open gap by making the Scripts debug panel render 08-UI-SPEC.md:126's promised sentence for a memory-limit kill instead of a raw V8 exception.**

## Performance

- **Duration:** ~35 min (includes a `mage Bootstrap` re-provision of this worktree's Deno toolchain)
- **Completed:** 2026-07-30T18:02:55Z
- **Tasks:** 3/3
- **Files modified:** 8 modified, 3 created

## Accomplishments

- `checkMemoryPressure`/`classifyMemoryExhaustion` (capability.go) and `startMemoryWatch` (memorywatch.go, new) implement D-08's resource cause: a 100ms polling monitor bound to the run's own Job Object terminates it proactively, and a post-exit backstop reclassifies a V8 allocation-failure crash the same way when the monitor loses the timing race.
- `(*jobObject).peakMemoryBytes`/`jobMemoryLimitBytes` (jobobject_windows.go) add a strictly read-only `QueryInformationJobObject` query — the two existing `SetInformationJobObject` calls remain unchanged, provably (grep-pinned at exactly 2).
- `session.go`'s `Run` wires the monitor as a `defer`red goroutine started before the debug-bridge block, and extends only the fourth branch of the post-`cmd.Wait()` chain with the classifier backstop — branches 1-3 untouched.
- `ScriptDebugPanel.tsx`'s `describeTermination` gains the fourth D-08 regex branch, rendering `Terminated: memory limit exceeded ({N} MB). Increase the limit in this script's profile if this is expected.` — the exact 08-UI-SPEC.md:126 sentence, parsed from the reason rather than hardcoded.
- Real-Deno acceptance evidence (see below) confirms the fix end-to-end on a genuinely provisioned Windows/Deno 2.9.4 toolchain, not just against fakes/mocks.

## Task Commits

1. **Task 1: Memory-pressure decision functions and the polling monitor** - `0a214b40` (feat)
2. **Task 2: Read-only Job Object peak-memory query and session.go wiring** - `811e9aa9` (feat)
3. **Task 3: Copywriting Contract branch in the debug panel and full cross-layer regression** - `0e44a9b9` (feat)

_No TDD RED/GREEN/REFACTOR split commits — each task's tests and implementation landed together per task, matching this plan's own `tdd="true"` intent (failing tests written first, then the implementation, verified locally before commit)._

## Files Created/Modified

- `internal/script/memorywatch.go` - `memorySampler` seam, `errMemoryUsageUnsupported` sentinel, `memoryWatchInterval`, `startMemoryWatch` polling monitor
- `internal/script/memorywatch_test.go` - `fakeMemorySampler` plus every platform-independent monitor behavior, including a `-race`-clean proof
- `internal/script/memorylimit_windows_test.go` - real Job Object query proof (`TestJobObjectPeakMemoryReadsConfiguredLimit`) plus three real-Deno run-level proofs
- `internal/script/capability.go` - `memoryPressureTriggerPercent`/`memoryExhaustionCorroborationPercent`/`v8AllocationFailureSignatures`, `memoryTriggerBytes`, `memoryLimitReason`, `checkMemoryPressure`, `classifyMemoryExhaustion`
- `internal/script/capability_test.go` - table tests for every new function above
- `internal/script/jobobject_windows.go` - `queryExtendedLimits`, `peakMemoryBytes`, `jobMemoryLimitBytes`
- `internal/script/jobobject_other.go` - `peakMemoryBytes` returning the unsupported sentinel
- `internal/script/session.go` - `memSampler` local, `startMemoryWatch`/`defer stopMemoryWatch()` wiring, and the classifier call in the post-Wait failed branch
- `internal/script/events_test.go` - replaced the never-produced `GOLC_SCRIPT_JOBOBJECT_MEMORY_LIMIT_EXCEEDED` fixture with the real code/status a memory-limit kill now produces
- `frontend/src/components/Scripts/ScriptDebugPanel.tsx` - fourth `describeTermination` regex branch, rewritten doc comment
- `frontend/src/components/Scripts/ScriptDebugPanel.test.tsx` - three new memory-limit-termination cases

## Decisions Made

- Followed the repository owner's explicit decision recorded in the plan: build the proactive monitor, do not amend 08-UI-SPEC.md to accept the raw exception.
- Widened `v8AllocationFailureSignatures` to four entries (see Deviations below) after real-Deno testing showed a fourth, genuinely V8-authored OOM shape on this machine.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Widened `v8AllocationFailureSignatures` with a fourth V8 OOM signature**
- **Found during:** Task 2's real-Deno acceptance pass (`TestRunMemoryLimitTerminatesWithNamedReason`)
- **Issue:** The plan specified exactly three signatures sourced from a prior recorded repro (`array buffer allocation failed`, `javascript heap out of memory`, `reached heap limit`). Running the real memory-pressure reproduction script repeatedly against a genuinely provisioned Deno 2.9.4 on this machine showed the same Job Object ceiling collision nondeterministically produces one of two distinct V8-authored crash texts: the catchable `RangeError: Array buffer allocation failed` (matched an existing signature) or an uncatchable `Fatal JavaScript out of memory: MarkCompactCollector: young object promotion failed` engine abort (matched none of the three), causing the post-exit classifier to miss it and the test to fail intermittently (observed failing 1 of the first 3 real runs before the fix, then 5/5 and a further 3/3 passing after it).
- **Fix:** Added `"fatal javascript out of memory"` (lowercased) as a fourth entry, with an updated doc comment explaining both crash shapes are genuine V8-authored OOM signals for the identical underlying condition (an OS-denied commit against the Job Object ceiling), never text a script could plausibly print on its own.
- **Files modified:** `internal/script/capability.go`
- **Verification:** `TestRunMemoryLimitTerminatesWithNamedReason` run 8 times total across two stability passes after the fix, 8/8 pass; `TestClassifyMemoryExhaustionSignatureAndCorroboration`'s table still covers every original case unchanged.
- **Committed in:** `811e9aa9` (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 bug fix)
**Impact on plan:** The fix only adds robustness to real-machine V8 crash-text variability; it does not change the Code, the Message shape, the trigger/corroboration percentages, or any other contract Task 1/3 depend on. No scope creep.

## Real-Deno Acceptance Evidence (for the `<output>` spec's required record)

Machine: this worktree, freshly bootstrapped this session (`mage Bootstrap`, Deno 2.9.4 pinned, had never been provisioned in this worktree before). All real-Deno-gated tests ran with `--- PASS`, never `--- SKIP`.

- **Configured ceiling used:** 64 MB (`MemoryLimitMB: 64`) — usable on first attempt; **never had to be raised to 128 MB**.
- **Peak memory observed:** an early diagnostic run (debug `console.log` per 2-chunk step, since removed from the committed test) showed the retained-chunk allocation climbing cleanly in 2 MiB steps up to at least 44 MB (22 chunks) before the run ended; the exact `PeakJobMemoryUsed` at the instant of termination was not separately logged in the committed test, but by `checkMemoryPressure`'s own boundary semantics it necessarily falls between the 60.8 MB proactive trigger (95% of 64 MB) and the 64 MB hard ceiling whenever the proactive monitor is what fires.
- **Which mechanism produced the reason:** genuinely nondeterministic run to run on this machine. Across roughly ten real-Deno executions during this pass (5-run and 3-run stability checks plus earlier diagnostic runs), all three of the following were directly observed:
  1. The proactive monitor terminating the run with no V8 crash text at all (job closed before V8 ever hit the denied commit).
  2. The post-exit classifier reclassifying a catchable `RangeError: Array buffer allocation failed`.
  3. The post-exit classifier reclassifying an uncatchable `Fatal JavaScript out of memory: MarkCompactCollector: young object promotion failed` V8 engine abort.

  All three converge on the identical final `RunOutcome`, confirming Task 2's own design note ("which one wins is not observable to the user") holds in practice, not just in the code comment.
- **Exact rendered `RunOutcome.Reason` first line:** `GOLC_SCRIPT_MEMORY_EXCEEDED: run exceeded its 64 MB memory limit` (asserted exact-match, never substring/prefix, in every passing run; the reason was also confirmed to contain no `at file:` stack frame).
- **Exact rendered ScriptDebugPanel sentence (asserted in `ScriptDebugPanel.test.tsx`, matches 08-UI-SPEC.md:126 byte-for-byte):**
  - Termination detail: `Terminated: memory limit exceeded (64 MB). Increase the limit in this script's profile if this is expected.`
  - Stopped banner: `Stopped: memory limit exceeded (64 MB). Increase the limit in this script's profile if this is expected.`
  - 256 MB variant (proves the figure is parsed, not hardcoded): `Terminated: memory limit exceeded (256 MB). Increase the limit in this script's profile if this is expected.`

## Issues Encountered

- The Deno toolchain was not provisioned in this fresh worktree; `mage Bootstrap` (~2 min) was run once at the start of Task 2 to provision it before the real-Deno-gated tests could run for real rather than skip. This is the expected, documented first-provision cost this plan's own acceptance criteria call out ("A skip here means the Deno toolchain is not provisioned and the gap is NOT closed — run `mage Bootstrap`").
- See the Deviations section above for the one behavior-affecting issue (the fourth V8 crash-text signature) discovered and fixed during real-Deno testing.

## Next Phase Readiness

- 08-VERIFICATION.md's single remaining `gaps_found` item (the memory/resource termination cause not rendering its Copywriting Contract sentence) is closable on re-verification: all four D-08 causes (deadline, rate, resource/memory, capability-scope) now render their exact sentences, proven by passing (not skipped) real-Deno and frontend tests.
- The only remaining open item for Phase 8 is the human/GUI walkthrough already deferred to the repository owner (08-13-SUMMARY.md item 7, this plan's Task 3 `<human-check>`) — no new deferral introduced by this plan.
- No blockers for Phase 9 (Front-Door UI Completion) or any later milestone-9-11 work; this plan touched only `internal/script` and the Scripts debug panel.

## Self-Check: PASSED

- `internal/script/memorywatch.go` — FOUND
- `internal/script/memorywatch_test.go` — FOUND
- `internal/script/memorylimit_windows_test.go` — FOUND
- `internal/script/capability.go` (memory additions) — FOUND
- `frontend/src/components/Scripts/ScriptDebugPanel.tsx` (memory branch) — FOUND
- Commit `0a214b40` — FOUND in `git log --oneline`
- Commit `811e9aa9` — FOUND in `git log --oneline`
- Commit `0e44a9b9` — FOUND in `git log --oneline`

---
*Phase: 08-isolated-typescript-automation*
*Completed: 2026-07-30*
