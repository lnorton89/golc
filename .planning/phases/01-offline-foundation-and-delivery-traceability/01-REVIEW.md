---
phase: 01-offline-foundation-and-delivery-traceability
reviewed: 2026-07-21T22:12:40Z
depth: standard
files_reviewed: 5
files_reviewed_list:
  - internal/command/linear.go
  - internal/command/linear_test.go
  - tools/linear-sync/src/adapter.ts
  - tools/linear-sync/test/mutation.test.ts
  - tests/acceptance/linear-transport.ps1
findings:
  critical: 0
  warning: 2
  info: 1
  total: 3
status: clean
fixed_post_review:
  - "CR-01 (GITHUB_EVENT_NAME isolation) fixed by orchestrator immediately after this review: t.Setenv(\"GITHUB_EVENT_NAME\", \"\") added to both TestScopeLinearApplyReplay subtests in internal/command/linear_test.go. Re-verified: passes both unset and with GITHUB_EVENT_NAME=pull_request; full `go test ./...` clean."
---

# Phase 01: Code Review Report (Gap-Closure Re-Review)

**Reviewed:** 2026-07-21T22:12:40Z
**Depth:** standard
**Files Reviewed:** 5
**Status:** clean (1 Critical finding fixed post-review — see below; 2 Warnings + 1 Info remain as non-blocking future work)

## Summary

This is a scoped re-review of exactly the files touched by the three gap-closure plans (01-30, 01-31, 01-32) that were meant to resolve the previous round's two Critical findings (CR-01: `runLinearApply` bypassing `apply.RunApply`'s freshness/resume orchestration; CR-02: `adapter.ts`'s `readOperation` missing a try/catch, risking a stalled shared NDJSON reader). Both original findings are **confirmed resolved** by direct code reading (see Verification section below) — the production fixes are correct and the new automated tests target the right seams.

However, one of this round's own new additions (the command-level replay test proving CR-01) introduces a fresh Critical defect: it is not isolated from the real process environment, and it deterministically fails when run inside the project's own required CI gate (`check.yml`, triggered only by `pull_request`). This was reproduced empirically (see CR-01 below), not just inferred from reading the code. Two Warnings and one Info item round out the rest of the findings; none of the pre-existing production logic showed new correctness regressions beyond the CI-isolation gap.

## Verification of Prior Critical Findings

- **Prior CR-01 (`runLinearApply` called `apply.Apply` instead of `apply.RunApply`): CONFIRMED RESOLVED.** `runLinearApply` (internal/command/linear.go:1466) now calls `apply.RunApply(client, plan, intentsFromMigratedMap(migrated), migrated.RemoteMappings, snapshot, nil, journal, os.LookupEnv)`, which internally re-validates plan integrity/freshness (guard.go), enforces the PR guard, and resumes only the journaled achieved prefix (`ResumePrefix`, journal.go:61) before attempting the remainder. `internal/command/linear_test.go`'s new `TestScopeLinearApplyReplay` proves both the staleness-rejection and within-lineage-resume behaviors against a fake client — production-shaped, not a reimplementation of the policy under test.
- **Prior CR-02 (`adapter.ts`'s `readOperation` missing a try/catch around `readByEntity`): CONFIRMED RESOLVED.** `tools/linear-sync/src/adapter.ts:210-221`'s `readOperation` now wraps its `readByEntity` call in `try { ... } catch { return { found: false }; }`, matching `confirmReadback`'s existing shape. `tools/linear-sync/test/mutation.test.ts`'s new sub-test ("read operation against a throwing issue() resolves to found:false...") and `tests/acceptance/linear-transport.ps1`'s new `-Mode readfailure` both exercise this path end-to-end and assert the run completes without a `GOLC_TRANSPORT_TIMEOUT`/`GOLC_TRANSPORT_PROCESS_EXITED` stall.

## Critical Issues

### CR-01: The new `TestScopeLinearApplyReplay` command-level test is not isolated from `GITHUB_EVENT_NAME` and fails deterministically inside the project's own required CI workflow

**File:** `internal/command/linear_test.go:328-457` (both subtests), caused by `internal/command/linear.go:1431` and `internal/command/linear.go:1466`

**Issue:** `runLinearApply` calls `apply.GuardAgainstPullRequestMutation(os.LookupEnv)` directly against the real process environment (line 1431), and again passes `os.LookupEnv` into `apply.RunApply` (line 1466, which re-checks the same guard internally at `internal/trace/apply/engine.go:271`). Every other caller in this codebase that exercises `GuardAgainstPullRequestMutation`/`RunApply` injects an explicit, test-controlled `lookup` function instead of the real environment — see `internal/trace/apply/apply_test.go:332-344,500-625` (always passes `pullRequest`/`push`/`absent`/`nil`, never `os.LookupEnv`), and `tests/acceptance/linear-transport.ps1:833,844` (explicitly sets and then `Remove-Item Env:\GITHUB_EVENT_NAME` around the one scenario that needs it, precisely to avoid this exact hazard).

The new `TestScopeLinearApplyReplay` in `linear_test.go` is the **only** place in the Go test suite that calls `runLinearApply` in-process, and it does no such isolation. `internal/command/test.go`'s `projectGoEnvironment` (used by `golc.ps1 test` → `go test -count=1 ./...`) passes `os.Environ()` through unfiltered except for `GOTOOLCHAIN`/`GOPROXY`/`GOMODCACHE`/`GOCACHE`/`GOFLAGS`, so the test binary inherits the invoking shell's `GITHUB_EVENT_NAME`. `.github/workflows/check.yml`'s **only** trigger is `pull_request` (line 21), and its `test` step runs `golc.ps1 test` (line 46) — meaning GitHub Actions' own default `GITHUB_EVENT_NAME=pull_request` env var is present for that entire job, including this test binary.

**Reproduced empirically** (not just inferred):
```
$ GITHUB_EVENT_NAME=pull_request go test ./internal/command/ -run TestScopeLinearApplyReplay -v
=== RUN   TestScopeLinearApplyReplay/stale_re-apply_...
    linear_test.go:340: first apply: ExitCode = 1, want 0; stderr: GOLC_APPLY_PR_BLOCKED: mutating apply is never permitted from a pull_request-triggered CI event (CONTEXT D-16)
=== RUN   TestScopeLinearApplyReplay/a_within-lineage_retry_...
    linear_test.go:394: first (partial) apply: ExitCode = 1, want 0 ...; stderr: GOLC_APPLY_PR_BLOCKED: ...
--- FAIL: TestScopeLinearApplyReplay (0.01s)
FAIL

$ unset GITHUB_EVENT_NAME; go test ./internal/command/ -run TestScopeLinearApplyReplay -v
--- PASS: TestScopeLinearApplyReplay (0.02s)
PASS
```

Both subtests fail on their very *first* `runLinearApply` call — which the test expects to succeed with `ExitCode == 0` — when `GITHUB_EVENT_NAME=pull_request` is present, exactly the condition present in `check.yml`'s job environment. This means the new replay test added to prove the CR-01 fix will itself break the project's only CI gate on every pull request, including the PR that lands this very fix.

**Fix:** Isolate the test from the real process environment the same way the rest of the codebase already does. The simplest fix is to force a definitively non-`"pull_request"` value at the top of `TestScopeLinearApplyReplay` (or each subtest), using `t.Setenv` so it is automatically restored:

```go
func TestScopeLinearApplyReplay(t *testing.T) {
	// GuardAgainstPullRequestMutation (reached through runLinearApply) reads
	// the real process environment; isolate this test from whatever
	// GITHUB_EVENT_NAME the invoking `go test` process happens to have
	// inherited (for example "pull_request" inside .github/workflows/check.yml
	// itself), matching apply_test.go's injected-lookup precedent.
	t.Setenv("GITHUB_EVENT_NAME", "workflow_dispatch")

	t.Run("stale re-apply of an already-achieved plan is rejected without any duplicate create", func(t *testing.T) {
		...
```

(A longer-term structural fix would give `runLinearApply` the same injectable `lookupEnv` seam `RunApply` already has, e.g. a package-level variable analogous to `applyRemoteClientFactory`, so no test ever needs to touch real env vars at all — but the `t.Setenv` fix above is sufficient to stop the CI break.)

## Warnings

### WR-01: `commitApplyResults` reimplements `apply.RunApply`'s own achieved-prefix rule instead of using its returned `Journal`

**File:** `internal/command/linear.go:1300-1315` (`achievedApplyPrefix`), `internal/command/linear.go:1361-1373` (`commitApplyResults`), `internal/command/linear.go:1466` (`report, _, err := apply.RunApply(...)`)

**Issue:** `apply.RunApply` already computes and returns the exact journal `runLinearApply` needs (`internal/trace/apply/engine.go:288`, `newJournal := Journal{PlanID: plan.PlanID, Results: achievedPrefix(allResults)}`), but `runLinearApply` discards that second return value (`_`) and instead has `commitApplyResults` recompute an equivalent journal from scratch via its own copy of the "leading contiguous run of completed/noop results" rule (`achievedApplyPrefix`, a line-for-line duplicate of engine.go's unexported `achievedPrefix`). Nothing enforces the two copies stay identical beyond a doc comment; if `engine.go`'s rule is ever extended (e.g., a new terminal status is added, or the prefix rule gains an edge case), `linear.go`'s independent copy can silently diverge from what `RunApply` itself actually computed and journaled internally, producing a journal on disk that doesn't match `RunApply`'s own notion of "achieved."

**Fix:** Use the `Journal` `RunApply` already returns instead of rebuilding it:
```go
report, newJournal, err := apply.RunApply(client, plan, intentsFromMigratedMap(migrated), migrated.RemoteMappings, snapshot, nil, journal, os.LookupEnv)
if err != nil {
	return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
}

if err := commitApplyResults(request.Root, resolvedPlanFile, migrated, newJournal, report); err != nil {
	...
```
and change `commitApplyResults`/`mergeApplyResultsIntoMap` to take the pre-built `apply.Journal` directly (still short-circuiting when `len(newJournal.Results) == 0`), removing `achievedApplyPrefix` entirely.

### WR-02: CR-02's regression coverage exercises only the Issue-shaped read path, never `project()`/`projectMilestone()`

**File:** `tools/linear-sync/test/mutation.test.ts:173-202`, `tests/acceptance/linear-transport.ps1:606-725` (`Invoke-ReadFailureRecoveryAcceptance`)

**Issue:** Both new CR-02 regression tests exercise the throwing-read fix only through the `issue()` SDK accessor:
- `mutation.test.ts`'s new sub-test hardcodes `entity: "task_subissue"` (line 181), so it only ever drives `readByEntity`'s `case "parent_issue" | "requirement_issue" | "task_subissue": return client.issue(...)` branch.
- `linear-transport.ps1`'s `readfailure` mode picks its archive target with `Where-Object { $_.linear_type -eq "issue" ... }` (line 667), so it likewise only ever archives (and re-reads) an Issue-shaped object, never a Project or ProjectMilestone.

`readByEntity`'s try/catch wraps the entire `switch` in `readOperation` (adapter.ts:210-221), so the fix is structurally uniform across all three branches and the residual risk is low — but as written, neither automated suite would catch a regression specific to the `project()`/`projectMilestone()` accessors (for example, if a future change moved the try/catch inside individual switch cases instead of wrapping the whole dispatch).

**Fix:** Add a second `mutation.test.ts` sub-test (or parameterize the existing one over `entity: "project"` and `entity: "project_milestone"`) asserting the same found:false-on-throw behavior for those two accessors, and/or have `linear-transport.ps1`'s `readfailure` mode pick its `$archiveTarget` from a project/project_milestone mapping instead of (or in addition to) an issue mapping.

## Info

### IN-01: `runLinearApply` performs the PR guard and plan-integrity checks twice, with no comment tying the two call sites together

**File:** `internal/command/linear.go:1285` (`ValidatePlanIntegrity` inside `decodeAndValidatePlanStrict`), `internal/command/linear.go:1431` (`GuardAgainstPullRequestMutation`), both re-run again inside `apply.RunApply` (`internal/trace/apply/engine.go:265,271`)

**Issue:** This is almost certainly intentional (failing fast before spawning the Node subprocess or reading `LINEAR_API_KEY`/`LINEAR_TEAM_ID`, consistent with this file's D-19/D-20 "never touch credentials unless necessary" discipline elsewhere), and both checks are cheap and idempotent, so this is not a functional bug. It's flagged only because no comment currently explains *why* the outer checks are kept even though `RunApply` performs the same checks internally, which makes the duplication read as accidental leftover from before the CR-01 fix (when `runLinearApply` called the guard-less `apply.Apply` and needed its own explicit guard) rather than a deliberate fail-fast design.

**Fix:** Add a short comment at the outer `GuardAgainstPullRequestMutation`/integrity call sites noting they are a deliberate fail-fast duplicate of the checks `RunApply` performs internally (to avoid spawning the transport subprocess or touching credentials under a blocked PR event), so a future reader doesn't "simplify" one side without realizing the other still needs it.

---

_Reviewed: 2026-07-21T22:12:40Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
