---
phase: 08-isolated-typescript-automation
fixed_at: 2026-07-30T18:30:00Z
review_path: .planning/phases/08-isolated-typescript-automation/08-14-REVIEW.md
iteration: 1
findings_in_scope: 3
fixed: 3
skipped: 0
status: all_fixed
---

# Phase 08 (08-14 gap closure): Code Review Fix Report

**Fixed at:** 2026-07-30T18:30:00Z
**Source review:** .planning/phases/08-isolated-typescript-automation/08-14-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 3 (WR-01, WR-02, WR-03 — Warning tier; `fix_scope: critical_warning`, so Info findings IN-01/IN-02 were out of scope for this run)
- Fixed: 3
- Skipped: 0

## Fixed Issues

### WR-01: `classifyMemoryExhaustion`'s corroboration floor is far below the actual trigger, and its signatures are freely writable to raw stderr

**Files modified:** `internal/script/capability.go`
**Commit:** 94e36539
**Applied fix:** Raised `memoryExhaustionCorroborationPercent` from 50 to 90 (the review's first suggested fix, kept minimal per explicit user guidance rather than the more invasive "drop signature-substring approach" alternative). The kernel-observed peak now has to reach nearly the same threshold the proactive monitor (`memoryPressureTriggerPercent = 95`) itself uses before a stderr-signature match is corroborated, closing the previously wide, cheaply-reachable 50%-vs-95% forgery window. Updated the constant's doc comment to explain the new rationale. Verified against the existing `TestClassifyMemoryExhaustionSignatureAndCorroboration` table (its "corroborated at 62 of 64 MB" cases, ~96.9%, still clear the new 90% floor; its uncorroborated cases remain far below it) — all subtests pass unchanged.

### WR-02: The post-exit memory backstop is unreachable once `runDispatchIO` has already marked the run `Failed`

**Files modified:** `internal/script/session.go`
**Commit:** d87d1319
**Applied fix:** Decoupled the `classifyMemoryExhaustion` backstop from the `outcome.Status == Succeeded` guard, per the review's suggested code. The branch now triggers on `waitErr != nil` alone; the `Succeeded` -> `Failed` reclassification happens conditionally inside the branch instead of gating entry to it, so the backstop now also runs when `runDispatchIO`'s scan/decode-error paths have already set `outcome.Status = Failed` (the exact truncated-write interleaving a genuine OOM kill can produce). Updated the surrounding doc comment to explain why the guard was removed. Full `internal/script` test suite passes with no regressions.

### WR-03: Proactive 95% trigger can kill an otherwise-successful run in its final moments

**Files modified:** `internal/script/memorywatch.go`
**Commit:** a789abcd
**Applied fix:** Added a debounce per explicit user instruction: `startMemoryWatch`'s poll loop now tracks a consecutive above-trigger streak and only calls `beginTermination`/`terminate` once `checkMemoryPressure` has returned non-nil for `memoryPressureDebounceSamples` (= 2) consecutive ticks in a row (~200ms of sustained pressure at the 100ms poll interval); any tick that samples back below the trigger resets the streak to 0. Added the new `memoryPressureDebounceSamples` constant with a doc comment explaining the rationale, and updated `startMemoryWatch`'s doc comment to describe the debounced behavior. All `TestMemoryWatch*` tests (which poll with bounded timeouts up to 2s) pass unchanged, since the added ~100ms latency is well within their timeout windows.

## Skipped Issues

None — all in-scope findings (WR-01, WR-02, WR-03) were fixed per explicit user guidance to leave nothing skipped.

---

_Fixed: 2026-07-30_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
