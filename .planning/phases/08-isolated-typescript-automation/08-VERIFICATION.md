---
phase: 08-isolated-typescript-automation
verified: 2026-07-30T19:15:00Z
status: human_needed
score: 12/12 must-haves verified
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: gaps_found
  previous_score: 7/8
  gaps_closed:
    - "A deadline, rate, resource, or capability-scope termination renders its exact Copywriting Contract sentence naming the limit or the attempted command and scope (D-08) — the resource/memory case now renders \"Terminated: memory limit exceeded (N MB). Increase the limit in this script's profile if this is expected.\" exactly, proven by a passing (not skipped) real-Deno test."
  gaps_remaining: []
  regressions: []
human_verification:
  - test: "Task 1 live-observation steps (sandbox denial + resource enforcement watched in real time), including the new memory-limit kill"
    expected: "No visible Art-Net interruption during any termination (deadline, rate, scope, or the new memory kill); Stop is a genuine single click with no confirmation; a plain Run opens no inspector socket (verified by OS-level port enumeration, not just the textual absence check the automated test uses)."
    why_human: "Requires a live webview, live Art-Net hardware/target, and real-time visual observation — none of which this verification pass had access to."
  - test: "Task 2's sixteen-step authoring-to-debugging GUI walkthrough"
    expected: "Every step matches 08-UI-SPEC.md's Copywriting Contract and D-01/D-04/D-05/D-12 behavior exactly as specified."
    why_human: "Visual/interaction quality (autocomplete dropdown content, gutter click precision, banner copy transitions, OS color-scheme following) cannot be verified by static analysis or headless tests; the backend/CLI/Wails-service layer underneath every one of these steps is proven for real (per this report's spot-checks), which de-risks but does not substitute for the visual pass."
  - test: "08-14 Task 3's live desktop check: create an Advanced-profile script at Memory limit 64 MB with a retained allocating loop, click Run, and confirm the debug panel's terminal banner reads exactly \"Stopped: memory limit exceeded (64 MB). Increase the limit in this script's profile if this is expected.\" with Dismiss and Run Again present, and that Art-Net output is uninterrupted throughout."
    expected: "Terminal banner renders the exact Copywriting Contract sentence with Dismiss/Run Again present; Art-Net output shows no interruption."
    why_human: "Requires a live webview and live Art-Net observation. Backend behavior (Go-side termination reason, frontend sentence rendering) is independently proven by passing automated tests in this pass; only the live-GUI/live-hardware observation is deferred."
---

# Phase 8: Isolated TypeScript Automation Verification Report (Re-Verification)

**Phase Goal:** Users can create and debug typed TypeScript automation in a supervised, capability-limited process that cannot own or delay deterministic output.
**Verified:** 2026-07-30T19:15:00Z
**Status:** human_needed
**Re-verification:** Yes — after gap closure (08-14-PLAN.md)

> **MVP-mode note (carried forward unchanged):** ROADMAP.md marks Phase 8 `Mode: mvp`, but the phase's own goal line is not written in User Story form, and every plan in the phase (including 08-14) explicitly surfaced this as non-blocking rather than inventing a story. Per that already-recorded phase-level decision, this report applies the standard goal-backward methodology against the four ROADMAP Success Criteria, not the MVP-mode User Flow Coverage table.

## Priority Verification Item: Gap Closure (08-14) — Memory-Limit Termination Copy

The prior verification pass (08-VERIFICATION.md, `status: gaps_found`) found exactly one gap: a script that exceeded its Job Object memory ceiling was bounded correctly at the OS level but surfaced a raw, uncaught V8 `RangeError` instead of 08-UI-SPEC.md:126's promised `"Terminated: {limit name} exceeded ({limit value})..."` sentence. 08-14-PLAN.md (a proactive Go-side memory-usage monitor plus a post-exit stderr-signature classifier) was executed to close it, then code-reviewed (08-14-REVIEW.md, 3 Warning findings), then fixed (08-14-REVIEW-FIX.md). All of this was independently re-derived from the codebase in this pass, not taken from any report's narrative.

| Item | Independent evidence |
|---|---|
| Gap fix present and correct | `internal/script/capability.go:239-245` (`memoryLimitReason`) is the single constructor for Code `GOLC_SCRIPT_MEMORY_EXCEEDED` / Message `"run exceeded its %d MB memory limit"`, called from both `checkMemoryPressure` (proactive, capability.go:255) and `classifyMemoryExhaustion` (post-exit backstop, capability.go:277). `internal/script/memorywatch.go` implements `startMemoryWatch`, a 100ms-poll goroutine bound to the run's own Job Object, using the identical `run.beginTermination`/`run.terminate()` pair `(*Host).enforce` already uses. `internal/script/session.go:864-866` wires it (`startMemoryWatch(runCtx, run, limits, memSampler)` + `defer stopMemoryWatch()`); `session.go:962-970` extends only the fourth post-`cmd.Wait()` branch with the classifier backstop, branches 1-3 untouched (confirmed by direct read, lines 908-970). |
| Real end-to-end proof, not just compiling code | `go test ./internal/script/... -run 'TestRunMemoryLimitTerminatesWithNamedReason\|TestRunWithinMemoryLimitSucceeds\|TestRunUnrelatedCrashStillReportsFailed\|TestJobObjectPeakMemoryReadsConfiguredLimit' -v -count=1` — **all 4 tests `--- PASS`, none `--- SKIP`** (this worktree has a genuinely provisioned Deno toolchain), independently re-run in this pass. `TestRunMemoryLimitTerminatesWithNamedReason` asserts `RunOutcome.Reason`'s first line is exactly `GOLC_SCRIPT_MEMORY_EXCEEDED: run exceeded its 64 MB memory limit` (exact-match assertion in the test source, not weakened to a substring/prefix check — confirmed by reading `memorylimit_windows_test.go:151`). |
| Frontend sentence renders exactly | `frontend/src/components/Scripts/ScriptDebugPanel.tsx:164-170` — the fourth `describeTermination` regex branch, `isCrash: false`. `npx vitest run src/components/Scripts/ScriptDebugPanel.test.tsx` — 24/24 pass, including the new 64 MB / 256 MB (figure-is-parsed-not-hardcoded) / stopped-banner cases, independently re-run in this pass. |
| Deadline/rate/scope sentences unchanged (no regression) | Read `ScriptDebugPanel.tsx:142-183` in full: the deadline (`148-154`), rate (`156-162`), and scope (`172-179`) regex branches and their rendered sentences are byte-identical to the previous verification pass's recorded evidence. |
| WR-01 (94e36539) genuinely applied | `internal/script/capability.go:189`: `const memoryExhaustionCorroborationPercent = 90` (was 50) — confirmed by direct read, not the fix report's word. |
| WR-02 (d87d1319) genuinely applied | `internal/script/session.go:933`: the branch guard is `} else if waitErr != nil {` (not `&& outcome.Status == show.ScriptRunStatusSucceeded`); the `Succeeded`→`Failed` reclassification (934-939) is nested *inside* that branch rather than gating entry to it, so the classifier at 962-970 runs unconditionally whenever `waitErr != nil` — confirmed by direct read of the current file, matching the fix's intent exactly (a genuine OOM kill that severs stdout mid-frame, setting `outcome.Status = Failed` via `runDispatchIO`'s scan/decode-error path, still reaches the classifier). |
| WR-03 (a789abcd) genuinely applied | `internal/script/memorywatch.go:56` (`const memoryPressureDebounceSamples = 2`) and the poll loop (`memorywatch.go:88-114`) tracks `aboveTriggerStreak`, only calling `beginTermination`/`terminate` once the streak reaches 2 consecutive above-trigger samples (~200ms), resetting to 0 on any below-trigger sample — confirmed by direct read. `go test ./internal/script/... -race -run 'TestMemoryWatch' -count=1` passes clean (no data race in the debounce-streak state). |
| No dependency/build-system side effects | `git diff --stat -- go.mod go.sum frontend/package.json frontend/package-lock.json` — empty output. `go run ./cmd/golc-project generate && go run ./cmd/golc-project check --concern project` — zero drift. `mage GenerateCheck` — exits 0, "no drift". `GOOS=linux go build ./internal/script/...` and `GOOS=darwin go build ./internal/script/...` — both clean. `grep -v '^[[:space:]]*//' internal/script/jobobject_windows.go \| grep -c 'SetInformationJobObject'` returns exactly `2` (unchanged) — the Job Object's kernel-enforced memory/CPU configuration provably gained no new write path. |
| Unrelated infra fix (linear-map.json) confirmed genuinely unrelated to phase-08 scope | `git show a5e3eec5 --stat` touches only `.planning/linear-map.json` (60 insertions), no files under `internal/script`, `internal/wails`, or `frontend/`. `go test ./internal/trace/...` (the package this regenerated file backs) passes clean. This is GSD-tooling infrastructure drift, not phase-08 application code — confirmed, not assumed. |

**Conclusion: the single remaining gap from the prior verification pass is genuinely closed, not merely claimed.** All three WR-01/WR-02/WR-03 code-review fixes are present and correctly address what they claim to address, independently re-derived from the current codebase. No regression was found to the original (non-gap) 08-REVIEW.md's CR-01/WR-01/WR-02 fixes, which were re-checked directly (event stream start/stop still present in `main.go`; single-redaction-point invariant intact in `dispatchCmdCall`; route-fallback logic in `publishCallOutcome` untouched).

## Goal Achievement

### Observable Truths

| # | Truth (mapped to ROADMAP Success Criteria / gap-closure must-haves) | Status | Evidence |
|---|---|---|---|
| 1 | SC1: A user can create, edit, validate, run, stop, and debug a TypeScript script from the application. | ✓ VERIFIED | Unchanged from prior pass; full regression suite (`internal/script`, `internal/wails`, `internal/command`, `internal/show`, `internal/scriptsdk`) re-run clean in this pass. |
| 2 | SC2: Scripts use a generated typed GOLC SDK for commands/queries/events, with no route to raw DMX or frame evaluation. | ✓ VERIFIED | `go run ./cmd/golc-project generate && go run ./cmd/golc-project check --concern project` re-run in this pass — zero drift, `excludedRoutes["playback evaluate"]` unchanged. |
| 3 | SC3: Before execution, a user can inspect/assign capabilities, deadlines, rate limits, resource limits; the runtime has no ambient filesystem/network/env/subprocess/native-code/uncached-dependency access. | ✓ VERIFIED | Full regression suite re-run clean; Job Object memory/CPU configuration provably unchanged (`SetInformationJobObject` count still exactly 2). |
| 4 | SC4: A user can inspect structured logs, diagnostics, source locations, command outcomes, cancellation state, and can terminate a runaway/crashed/blocked script without interrupting playback or Art-Net. | ✓ VERIFIED (fully, no remaining sub-gap) | The one previously-open sub-case (memory-limit termination copy) is now closed — see Priority Verification Item above. `TestScriptKillDoesNotBlockArtnet` re-run and passes. |
| 5 | Requirement traceability: every SCRP-01..06 requirement ID is claimed by at least one plan and has supporting evidence. | ✓ VERIFIED | `.planning/REQUIREMENTS.md:120-125` lists all six as `[x]`/Complete. `08-14-PLAN.md` correctly claims `SCRP-04, SCRP-05` for the narrow gap it closes; no orphaned Phase-8 requirement ID found. |
| 6 | CR-01 fix (original 08-REVIEW.md): `ScriptService`'s live event stream is started/stopped in production `main.go`. | ✓ VERIFIED (no regression) | `cmd/golc-desktop/main.go:123,158` — re-confirmed present and unchanged. |
| 7 | WR-01/WR-02 fixes (original 08-REVIEW.md): redaction single-point invariant and route-fallback consistency in `session.go`. | ✓ VERIFIED (no regression) | Re-confirmed by direct read: `dispatchCmdCall`'s `redactedMessage` pattern and `publishCallOutcome`'s `route` fallback both intact. |
| 8 | Deadline, rate, resource, and capability-scope terminations each render their exact Copywriting Contract sentence (D-08). | ✓ VERIFIED (gap closed) | All four causes now render exact copy. Memory case: `Terminated: memory limit exceeded (64 MB). Increase the limit in this script's profile if this is expected.` — proven by passing (not skipped) real-Deno test `TestRunMemoryLimitTerminatesWithNamedReason` plus passing `ScriptDebugPanel.test.tsx` cases. |
| 9 | A proactive Go-side monitor observes a live run's own Job Object memory usage and terminates it through the existing `beginTermination`/`terminate` path before/as it exhausts the ceiling (D-08, D-11). | ✓ VERIFIED | `TestMemoryWatchTerminatesAboveTrigger`, `TestMemoryWatchNeverOverwritesAnAlreadyRecordedReason` (fake-sampler, any platform) and `TestRunMemoryLimitTerminatesWithNamedReason` (real Job Object + real Deno) all pass — re-run in this pass, not taken from the SUMMARY. |
| 10 | A script that stays comfortably inside its memory limit still completes `Succeeded` — no false-positive termination. | ✓ VERIFIED | `TestRunWithinMemoryLimitSucceeds` — real Deno, `--- PASS`. `TestCheckMemoryPressureBoundary`'s 50%-of-ceiling case also confirms this at the unit level. |
| 11 | The Windows Job Object's memory/CPU configuration is unchanged — the new usage query is read-only. | ✓ VERIFIED | `grep -v '^[[:space:]]*//' internal/script/jobobject_windows.go \| grep -c 'SetInformationJobObject'` = exactly `2` (re-run in this pass); `QueryInformationJobObject` count = 1; four pre-existing Job Object tests (`TestJobObjectCreateConfiguresLimitsAndCloses`, `TestJobObjectKillsAdversarialDenoChild`) still pass. |
| 12 | WR-01/WR-02/WR-03 (08-14-REVIEW.md fixes) are genuinely present and correctly address what they claim. | ✓ VERIFIED | Directly re-derived from current source, not from 08-14-REVIEW-FIX.md's narrative — see Priority Verification Item table rows above. |

**Score:** 12/12 truths verified. No FAILED truths. No behavior-dependent truth was left unproven — every one asserting a runtime state transition (proactive termination, false-positive avoidance, debounce, backstop reachability) was confirmed by a passing, non-skipped test independently re-run in this pass, not by symbol presence alone.

### Required Artifacts

| Artifact | Expected | Status | Details |
|---|---|---|---|
| `internal/script/memorywatch.go` | `memorySampler` seam, `errMemoryUsageUnsupported`, `startMemoryWatch` with debounce | ✓ VERIFIED | Present, substantive, wired into `session.go`; `memoryPressureDebounceSamples = 2` confirmed (WR-03). |
| `internal/script/capability.go` | `checkMemoryPressure`, `classifyMemoryExhaustion`, `memoryLimitReason`, `memoryTriggerBytes`, `v8AllocationFailureSignatures` | ✓ VERIFIED | All present; `memoryExhaustionCorroborationPercent = 90` confirmed (WR-01). |
| `internal/script/jobobject_windows.go` | `peakMemoryBytes`/`jobMemoryLimitBytes` reading `PeakJobMemoryUsed`, read-only | ✓ VERIFIED | Present; `SetInformationJobObject` count unchanged at 2. |
| `internal/script/jobobject_other.go` | `peakMemoryBytes` returning `errMemoryUsageUnsupported` | ✓ VERIFIED | Present; `GOOS=linux`/`GOOS=darwin` builds clean. |
| `internal/script/session.go` | `startMemoryWatch` call site, decoupled post-exit classifier branch | ✓ VERIFIED | Present and wired; WR-02 fix confirmed (branch no longer gated on `outcome.Status == Succeeded`). |
| `frontend/src/components/Scripts/ScriptDebugPanel.tsx` | `GOLC_SCRIPT_MEMORY_EXCEEDED` branch of `describeTermination` | ✓ VERIFIED | Present, `isCrash: false`, exact sentence rendered. |
| `internal/script/memorylimit_windows_test.go`, `internal/script/memorywatch_test.go` | Platform-independent + real-Deno proofs | ✓ VERIFIED | All target tests pass, none skipped, in this environment. |

### Key Link Verification

| From | To | Via | Status | Details |
|---|---|---|---|---|
| `jobObject.handle` | `checkMemoryPressure` | `windows.QueryInformationJobObject` → `PeakJobMemoryUsed` → `(*jobObject).peakMemoryBytes` | ✓ WIRED | Confirmed by `TestJobObjectPeakMemoryReadsConfiguredLimit` (real query round-trips the configured 64 MB ceiling). |
| `checkMemoryPressure` (proactive) | `run.beginTermination` → `run.terminate()` | `startMemoryWatch`'s poll loop, debounced at 2 consecutive samples | ✓ WIRED | `TestMemoryWatchTerminatesAboveTrigger`, `-race`-clean. |
| `RunOutcome.Reason` | `ScriptDebugPanel` rendered sentence | `computeTerminalEvent` → `PublishScriptEvent` → `EventsEmit("script:event")` → `describeTermination` | ✓ WIRED | Full chain unchanged from the prior pass (CR-01's fix); the new memory branch is a fourth regex on the same already-wired pipeline. |
| Job Object memory/CPU limits | Termination reason surfaced to user | All four D-08 causes (deadline/rate/scope/memory) → named `Terminated: ...` message | ✓ WIRED (gap closed) | Previously ⚠️ PARTIAL for the memory cause; now fully wired, proven by the real-Deno test. |

### Behavioral Spot-Checks (this pass)

| Behavior | Command | Result | Status |
|---|---|---|---|
| Memory-pressure decision functions and debounced monitor | `go test ./internal/script/... -run 'TestCheckMemoryPressure\|TestClassifyMemoryExhaustion\|TestMemoryWatch\|TestMemoryTriggerBytes' -v -count=1` | 15/15 subtests PASS | ✓ PASS |
| Monitor has no data race | `go test ./internal/script/... -race -run 'TestMemoryWatch' -count=1` | PASS | ✓ PASS |
| Real Deno: memory-limit kill renders exact named reason | `go test ./internal/script/... -run 'TestRunMemoryLimitTerminatesWithNamedReason\|TestRunWithinMemoryLimitSucceeds\|TestRunUnrelatedCrashStillReportsFailed\|TestJobObjectPeakMemoryReadsConfiguredLimit' -v -count=1 -timeout 5m` | 4/4 `--- PASS`, 0 `--- SKIP` | ✓ PASS |
| Frontend memory-limit sentence, incl. 256 MB parsed-not-hardcoded and stopped-banner form | `npx vitest run src/components/Scripts/ScriptDebugPanel.test.tsx` | 24/24 PASS | ✓ PASS |
| Full Scripts frontend component suite (≥73 required) | `npx vitest run src/components/Scripts src/workspaces/build/ScriptsWorkspace.test.tsx` | 73/73 PASS | ✓ PASS |
| Frontend type-check | `npx tsc --noEmit` | Clean | ✓ PASS |
| Full script-domain Go regression (once) | `go test ./internal/script/... ./internal/wails/... ./internal/command/... ./internal/show/... ./internal/scriptsdk/... -count=1` | All `ok` | ✓ PASS |
| Non-Windows contributor-CI build | `GOOS=linux go build ./internal/script/...` / `GOOS=darwin go build ./internal/script/...` | Both clean | ✓ PASS |
| `go vet` on phase packages | `go vet ./internal/script/... ./internal/wails/... ./cmd/golc-desktop/...` | Clean | ✓ PASS |
| Job Object write-path unchanged | `SetInformationJobObject` non-comment count | Exactly `2` | ✓ PASS |
| Placeholder diagnostic code removed | `grep -rc 'GOLC_SCRIPT_JOBOBJECT_MEMORY_LIMIT_EXCEEDED' internal/script/` | `0` everywhere | ✓ PASS |
| Regression: deadline/scope/Art-Net-noninterference | `go test ./internal/script/... -run 'TestDeadlineBoundary\|TestEnforceScopeHierarchy\|TestEnforceUnknownMethodFailsClosed\|TestScriptKillDoesNotBlockArtnet\|TestJobObjectKillsAdversarialDenoChild\|TestJobObjectCreateConfiguresLimitsAndCloses' -v -count=1` | All PASS | ✓ PASS |
| SDK/generated-docs drift | `go run ./cmd/golc-project generate && go run ./cmd/golc-project check --concern project` and `mage GenerateCheck` | Zero drift | ✓ PASS |
| Dependency manifests untouched | `git diff --stat -- go.mod go.sum frontend/package.json frontend/package-lock.json` | Empty | ✓ PASS |
| Unrelated linear-map.json fix scope | `git show a5e3eec5 --stat` + `go test ./internal/trace/...` | Touches only `.planning/linear-map.json`; trace tests pass | ✓ PASS (confirmed unrelated to phase-08 app code) |

### Probe Execution

No repository-committed `scripts/*/tests/probe-*.sh`-style probes are declared by this phase's plans (unchanged from the prior pass — 08-13's manual Deno-script probes are hand-run transcripts, not committed shell scripts). Step 7c: SKIPPED (no repository-committed probe scripts for this phase).

### Requirements Coverage

| Requirement | Source Plan(s) | Description | Status | Evidence |
|---|---|---|---|---|
| SCRP-01 | 08-01, 08-04, 08-05, 08-07, 08-09, 08-11, 08-12, 08-13 | Create/edit/validate/run/stop/debug from the application | ✓ SATISFIED | Unchanged; full regression re-confirmed. |
| SCRP-02 | 08-02(indirect), 08-03, 08-05 | Generated typed SDK, no raw DMX/frame-eval route | ✓ SATISFIED | Unchanged; drift-checked. |
| SCRP-03 | 08-02, 08-03, 08-05, 08-07 | No ambient FS/net/env/subprocess/native-code access | ✓ SATISFIED | Unchanged; zero `--allow-*` flags. |
| SCRP-04 | 08-01, 08-06, 08-09(indirect), 08-10, **08-14** | Assign capabilities/deadlines/rate/resource limits pre-execution | ✓ SATISFIED (no remaining sub-gap) | Resource limit now reports its own violation by name; false-positive guard proven. |
| SCRP-05 | 08-08, 08-09, 08-10, 08-12, **08-14** | Inspect structured logs/diagnostics/source locations/outcomes | ✓ SATISFIED (no remaining sub-gap) | Memory-limit termination reason and rendered sentence both confirmed present and correct. |
| SCRP-06 | 08-06 | Terminate runaway/crashed/blocked script without interrupting playback/Art-Net | ✓ SATISFIED | `TestScriptKillDoesNotBlockArtnet` re-confirmed passing. |

No orphaned Phase-8 requirement IDs found. `08-14-PLAN.md`'s `requirements: [SCRP-04, SCRP-05]` frontmatter correctly narrows to the two IDs its narrow gap-closure scope actually touches; SCRP-01/02/03/06 remain correctly claimed and verified elsewhere.

### Anti-Patterns Found

None (blocker or warning level) in the 08-14-modified files (`internal/script/memorywatch.go`, `internal/script/capability.go`, `internal/script/session.go`, `internal/script/jobobject_windows.go`, `internal/script/jobobject_other.go`, `internal/script/memorylimit_windows_test.go`, `internal/script/memorywatch_test.go`, `internal/script/events_test.go`, `frontend/src/components/Scripts/ScriptDebugPanel.tsx`, `frontend/src/components/Scripts/ScriptDebugPanel.test.tsx`). No `TODO`/`FIXME`/`XXX`/`TBD`/`HACK`/`PLACEHOLDER` debt markers found (the `EMPTY_PLACEHOLDER` UI-copy constant in `ScriptDebugPanel.tsx` is pre-existing, legitimate Copywriting Contract text, not a stub marker — already noted in the prior pass).

Two Info-level findings from 08-14-REVIEW.md (IN-01: no test pins the exact `memoryExhaustionCorroborationPercent` boundary; IN-02: no cross-check test pinning both code paths' rendered strings against each other) were correctly left unfixed — they are Info tier, explicitly out of scope for the fix run (`fix_scope: critical_warning`), and do not affect correctness of the shipped behavior.

### Gaps Summary

**No gaps remain.** The single gap the prior verification pass found (`08-VERIFICATION.md`, memory-limit termination not rendering its Copywriting Contract sentence) is closed: a proactive Go-side memory-usage monitor and a post-exit stderr-signature backstop both mint the single `GOLC_SCRIPT_MEMORY_EXCEEDED` reason via `memoryLimitReason`, wired through the existing `beginTermination`/`terminate` path and the existing `script:event` pipeline, rendering `Terminated: memory limit exceeded ({N} MB). Increase the limit in this script's profile if this is expected.` exactly as 08-UI-SPEC.md:126 specifies — proven end-to-end by a passing (not skipped) real-Deno test in this environment, independently re-run in this verification pass. All three code-review Warning fixes (WR-01: 90% corroboration floor, WR-02: backstop reachable even when `outcome.Status` is already `Failed`, WR-03: 2-sample debounce) are genuinely present in the current source and correctly address what they claim. No regression was found to the original 08-REVIEW.md's CR-01/WR-01/WR-02 fixes.

### Human Verification Required

These are carried forward, not new findings — the same GUI-only observations the prior verification pass and `deferred-items.md` (item 7) already documented as deliberately deferred by the repository owner to their own informal testing session. They are restated here (plus one new item from 08-14 Task 3's own `<human-check>`) because they remain genuinely unverified against the live desktop app as of this re-verification pass, and per this workflow's rules must not be silently dropped even though the memory-limit-copy item that previously accompanied them is now closed by code, not by override.

See the `human_verification` frontmatter block above for the structured list (3 items: the sandbox-denial/resource-enforcement live-observation walkthrough, the sixteen-step authoring-to-debugging GUI walkthrough, and 08-14 Task 3's specific memory-limit live-desktop check).

**This is the only reason this report's status is `human_needed` rather than `passed`.** Every programmatically-verifiable truth (12/12) is VERIFIED with independently re-run evidence; no gap remains; the phase's backend/CLI/service-layer/frontend-unit-test surface is proven end-to-end. What remains is exclusively the live-webview/live-Art-Net visual and interaction pass the repository owner has already chosen to run personally rather than gate the roadmap on.

---

_Verified: 2026-07-30T19:15:00Z_
_Verifier: Claude (gsd-verifier)_
