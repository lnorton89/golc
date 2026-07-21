---
phase: 01-offline-foundation-and-delivery-traceability
plan: 31
subsystem: infra
tags: [typescript, linear-sdk, ndjson-transport, error-handling, powershell]

# Dependency graph
requires:
  - phase: 01-offline-foundation-and-delivery-traceability
    provides: LinearSdkAdapter (tools/linear-sync/src/adapter.ts), the strict NDJSON transport (cli.ts/protocol.ts), and the mutation-uncertain hostile-SDK test contract (Plan 01-27)
provides:
  - readOperation now catches any exception from readByEntity and returns a found:false ReadResult, matching createOperation/updateOperation/confirmReadback's existing try/catch discipline
  - A committed regression test proving a throwing read resolves to found:false with zero canary leak
  - A Go-side offline acceptance scenario (linear-transport.ps1 -Mode readfailure) documenting the end-to-end CaptureSnapshot recovery
affects: [linear-sync, delivery-traceability, LINR-04]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "readOperation mirrors confirmReadback's try/catch-around-readByEntity shape: a caught exception on a read returns the same safe outcome as a genuine miss, carrying zero bytes from the raw error."
    - "Acceptance scenarios that need a bootstrapped .tools toolchain still carry an always-runnable structural grep gate, so verification worktrees without the toolchain still get an automated check."

key-files:
  created: []
  modified:
    - tools/linear-sync/src/adapter.ts
    - tools/linear-sync/test/mutation.test.ts
    - tests/acceptance/linear-transport.ps1

key-decisions:
  - "readOperation's catch is bare (no error binding, no diagnostic) -- a genuine miss and an SDK exception on a miss are already treated identically by confirmReadback elsewhere in this file, and CR-02 requires zero bytes from the caught error to reach the returned ReadResult (D-20 secret isolation)."
  - "The Task-1 regression test reuses the fixture's existing readbackFails:true scenario (rather than adding a new fixture scenario) purely for HostileLinearClient's throw-on-issue() behavior -- the read path under test never touches create/update."
  - "The Go-side acceptance scenario (-Mode readfailure) archives a remote object by its fake-SDK-generated Linear UUID (read from .planning/linear-map.json's remote_mappings after an initial full-hierarchy link), then asserts a fresh linear preview --remote completes well under the configured transport timeout with no GOLC_TRANSPORT_TIMEOUT/GOLC_TRANSPORT_PROCESS_EXITED -- proving the read exception never stalls the shared NDJSON reader loop, rather than merely asserting the immediate found:false value."

patterns-established:
  - "Sibling SDK call sites in adapter.ts (create/update/confirmReadback/read) now uniformly wrap readByEntity in try/catch and never let a raw SDK exception escape LinearSdkAdapter.execute."

requirements-completed: [LINR-04]

coverage:
  - id: D1
    description: "readOperation wraps readByEntity in try/catch and returns {found:false} on any exception, matching confirmReadback/createOperation/updateOperation."
    requirement: "LINR-04"
    verification:
      - kind: unit
        ref: "tools/linear-sync/test/mutation.test.ts#TestScopeLinearTransportNode > read operation against a throwing issue() resolves to found:false, not a thrown/rejected error"
        status: pass
    human_judgment: false
  - id: D2
    description: "A throwing read (HostileLinearClient-style issue()/project()/projectMilestone()) drives adapter.execute of a read operation to a resolved {found:false} ReadResult, not a thrown or rejected promise, so the shared NDJSON reader process keeps serving subsequent operations."
    requirement: "LINR-04"
    verification:
      - kind: unit
        ref: "tools/linear-sync/test/mutation.test.ts#TestScopeLinearTransportNode > read operation against a throwing issue() resolves to found:false, not a thrown/rejected error"
        status: pass
      - kind: e2e
        ref: "tests/acceptance/linear-transport.ps1 -Mode readfailure (Invoke-ReadFailureRecoveryAcceptance)"
        status: unknown
    human_judgment: true
    rationale: "The PowerShell acceptance scenario requires a bootstrapped .tools toolchain (golc.ps1 bootstrap --include linear-sync + golc.ps1 build --scope linear-sdk) to exercise the real Go<->Node process boundary end-to-end; it was authored, PowerShell-AST-parse-validated, and structurally gated in this worktree, but not run to completion because bootstrap would require downloading pinned Go/Node archives. A human or CI run with the bootstrapped toolchain should execute it once and flip this to pass."
  - id: D3
    description: "No leaked byte from the thrown error reaches the returned ReadResult (the {found:false} outcome carries no diagnostic text at all), preserving D-20 secret isolation on the read path."
    requirement: "LINR-04"
    verification:
      - kind: unit
        ref: "tools/linear-sync/test/mutation.test.ts#TestScopeLinearTransportNode > read operation against a throwing issue() resolves to found:false, not a thrown/rejected error"
        status: pass
    human_judgment: false

# Metrics
duration: 8min
completed: 2026-07-21
status: complete
---

# Phase 01 Plan 31: Read-Failure Recovery for LinearSdkAdapter.readOperation Summary

**readOperation now catches any readByEntity exception and returns found:false, closing the one uncaught SDK read-failure path (CR-02) that could stall the shared long-lived NDJSON reader loop.**

## Performance

- **Duration:** 8 min
- **Started:** 2026-07-21T21:44:00Z (approx, from first task commit)
- **Completed:** 2026-07-21T21:49:02Z
- **Tasks:** 3
- **Files modified:** 3

## Accomplishments
- `readOperation` (tools/linear-sync/src/adapter.ts) wraps its `readByEntity` call in try/catch and returns `{found:false}` on any exception, mirroring `confirmReadback`/`createOperation`/`updateOperation`'s existing discipline -- a missing/archived/deleted remote object on a read can no longer propagate uncaught out of `LinearSdkAdapter.execute` into `cli.ts`'s unwrapped `handleLine`.
- A new regression test in `mutation.test.ts` (`TestScopeLinearTransportNode` scope) drives a throwing `issue()` read through `adapter.execute` and asserts a resolved `{found:false}` ReadResult with zero canary/credential leak and exactly one SDK call -- proven RED against the unmodified adapter, then GREEN after the fix.
- A new offline Go-side acceptance scenario (`tests/acceptance/linear-transport.ps1 -Mode readfailure`) links a full hierarchy against the fake SDK, archives one already-linked task's remote object so its next read throws, and asserts a fresh `linear preview --remote` (which drives `processLinearClient.CaptureSnapshot`) completes promptly instead of stalling on `GOLC_LINEAR_SYNC_TIMEOUT_MS`.

## Task Commits

Each task was committed atomically (TDD: RED -> GREEN, plus one non-TDD acceptance-scenario task):

1. **Task 1: Add a failing read-failure test proving a throwing read resolves to found:false (RED)** - `b09c258` (test)
2. **Task 2: Wrap readOperation's readByEntity call in try/catch returning found:false (GREEN)** - `84af712` (fix)
3. **Task 3: Add a Go-side acceptance scenario for read-failure recovery under CaptureSnapshot** - `380f9dc` (test)

_Note: Tasks 1-2 are the plan's TDD pair (RED then GREEN); Task 3 is a plain `auto` task._

## Files Created/Modified
- `tools/linear-sync/src/adapter.ts` - `readOperation` now declares a `LinearEntityHandle | undefined` handle, attempts `readByEntity` inside try, and returns `{found:false}` on catch (bare catch, no diagnostic) before falling through to the existing missing-handle guard and found-true normalization.
- `tools/linear-sync/test/mutation.test.ts` - New sub-test inside the existing `TestScopeLinearTransportNode` scope: constructs `HostileLinearClient` with the fixture's `readbackFails:true` scenario, executes a `task_subissue` read against `linearUUID: "archived-uuid"`, and asserts a resolved `{found:false}` result, zero canary leak, and exactly one SDK call.
- `tests/acceptance/linear-transport.ps1` - New `-Mode readfailure` (`Invoke-ReadFailureRecoveryAcceptance`): links a hierarchy, archives one issue's remote object via a new `GOLC_TEST_ARCHIVE_IDS` env var consumed by the fake SDK's `project()`/`projectMilestone()`/`issue()` methods, then times a fresh `linear preview --remote` and asserts it completes with no timeout/process-exit diagnostics and well under the configured transport timeout.

## Decisions Made
- **Bare catch, no diagnostic on the read path:** matches the plan's explicit requirement that the found-false outcome carry zero bytes from the caught error (D-20 secret isolation), and mirrors the accepted-by-design equivalence (T-01-31-03 in the plan's threat register) that a genuine miss and an SDK exception on a miss are already treated identically by `confirmReadback`.
- **Reused the existing `readbackFails:true` fixture scenario for the new read-failure test** rather than adding a dedicated fixture scenario, since `HostileLinearClient.issue()`'s throw-on-`readbackFails` behavior is exactly what's needed to simulate a throwing read, and the test only exercises the read path (never create/update).
- **Acceptance scenario archives by Linear UUID (not local ID)**, read back from `.planning/linear-map.json`'s `remote_mappings` after an initial full-hierarchy link -- this matches exactly what `processLinearClient.CaptureSnapshot` reads by (the recorded `LinearUUID` per local ID), so the induced failure targets the real read path CR-02 describes.

## Deviations from Plan

None - plan executed exactly as written. All three tasks' `<action>`/`<verify>`/`<acceptance_criteria>` were followed precisely; no Rule 1-4 auto-fixes were needed.

## Issues Encountered
- The double-cast `(await adapter.execute(operation)) as ReadResult` failed to compile (`TS2352: neither type sufficiently overlaps`) because `OperationResult<TOperation>`'s conditional type resolves to `MutationOutcome` when `TOperation` is the full `Operation` union (the read operation object is cast `as unknown as Operation`, not a narrowed `ReadOperation`). Fixed by widening to `as unknown as ReadResult`, matching the file's own existing `as MutationOutcome` cast pattern for the identical situation. This was a pre-existing type-inference quirk in the test's own cast style, not a bug in adapter.ts or protocol.ts; no plan deviation, just a build-fix while authoring the test.
- A bootstrapped Node toolchain was available in this environment (system `node`/`npm`), so Tasks 1-2's TDD RED/GREEN cycle was verified behaviorally end-to-end (`npm install` -> `tsc` build -> `node --test`), not only via the structural grep gates: confirmed RED (thrown `AbortError` propagating out of `adapter.execute`) against the unmodified adapter, and confirmed GREEN (47/47 tests passing) after the fix.
- A bootstrapped `.tools` Go+Node toolchain (via `golc.ps1 bootstrap --include linear-sync` + `golc.ps1 build --scope linear-sdk`) was **not** available/attempted in this worktree (it would require downloading pinned Go 1.26.5 and Node 24.18.0 archives), so Task 3's acceptance scenario was validated via PowerShell AST parsing (`[System.Management.Automation.Language.Parser]::ParseFile`, confirmed no syntax errors) and careful tracing against `internal/command/linear.go`'s `CaptureSnapshot`/`readRecord` and `tools/linear-sync/src/cli.ts`'s `handleLine` reader-loop semantics, rather than a live end-to-end run. This matches the plan's own `<notes>` acknowledgment that the Node-scope behavioral test and PowerShell acceptance scenario "require a bootstrapped .tools toolchain (absent in verification worktrees)" and that each task therefore also carries an always-runnable structural gate.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- LINR-04's previously-failed truth (the read-exception path) now holds: `readOperation` reports a safe `{found:false}` outcome instead of propagating an uncaught exception, matching every sibling SDK call site in `adapter.ts`.
- A human or CI run with the bootstrapped `.tools` toolchain should execute `tests/acceptance/linear-transport.ps1 -Mode readfailure` once end-to-end to flip coverage item D2's acceptance-scenario verification from `unknown` to `pass`.
- No blockers for closing out the remaining Phase 01 gap-closure plans (01-30, 01-32).

---
*Phase: 01-offline-foundation-and-delivery-traceability*
*Completed: 2026-07-21*
