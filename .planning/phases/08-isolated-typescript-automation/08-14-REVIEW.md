---
phase: 08-isolated-typescript-automation
reviewed: 2026-07-30T00:00:00Z
depth: standard
files_reviewed: 10
files_reviewed_list:
  - frontend/src/components/Scripts/ScriptDebugPanel.test.tsx
  - frontend/src/components/Scripts/ScriptDebugPanel.tsx
  - internal/script/capability.go
  - internal/script/capability_test.go
  - internal/script/events_test.go
  - internal/script/jobobject_other.go
  - internal/script/jobobject_windows.go
  - internal/script/memorylimit_windows_test.go
  - internal/script/memorywatch.go
  - internal/script/memorywatch_test.go
  - internal/script/session.go
findings:
  critical: 0
  warning: 3
  info: 2
  total: 5
status: issues_found
---

# Phase 08 (08-14 gap closure): Code Review Report

**Reviewed:** 2026-07-30
**Depth:** standard
**Files Reviewed:** 10
**Status:** issues_found

## Summary

08-14 adds a proactive Job-Object memory-usage poller (`memorywatch.go`) plus a
post-exit stderr-signature backstop (`classifyMemoryExhaustion`,
`capability.go`) so a script that hits its configured memory ceiling now
surfaces the named `GOLC_SCRIPT_MEMORY_EXCEEDED` reason instead of a raw V8
exception. The core mechanism is sound: `startMemoryWatch`'s goroutine
lifecycle is clean (ticker + `ctx.Done()` + `sync.Once`-guarded stop channel,
verified against `memorywatch_test.go`'s explicit no-further-calls-after-stop
and no-further-calls-after-cancel assertions), the Windows/non-Windows build
tags are correct (`jobobject_windows.go` / `jobobject_other.go` /
`memorylimit_windows_test.go` all carry matching `//go:build` lines, and
`memorywatch.go` deliberately carries none so it compiles everywhere), and
`beginTermination`'s first-writer-wins semantics (D-11) are respected by both
the proactive monitor and the post-exit backstop.

I did **not** find a regression to the WR-01 single-redaction-point invariant:
`classifyMemoryExhaustion`'s own doc comment states it must never re-redact or
copy any substring of the (already-redacted) stderr tail into its returned
`TerminationReason`, and the implementation honors that — the returned
`Message` is built exclusively from `memoryLimitReason`'s host-authored
template plus the profile's own `MemoryLimitMB` integer, never from `reason`
itself. I also did not find a WR-02 route-fallback regression; `session.go`'s
`publishCallOutcome` route-fallback logic is untouched by this plan.

The two Warning-level findings below concern the actual design intent this
plan was asked to defend against: (1) the post-exit classifier's
kernel-sourced corroboration floor is set low enough that a script can still
game which failure gets reported as a benign memory-limit kill, and (2) the
backstop is silently skipped whenever `runDispatchIO` has already flagged the
outcome as `Failed` for an unrelated reason (a scan/decode error), which can
happen on the exact kind of truncated-write interleaving a genuine OOM kill
produces.

## Warnings

### WR-01: `classifyMemoryExhaustion`'s corroboration floor is far below the actual trigger, and its signatures are freely writable to raw stderr

**File:** `internal/script/capability.go:166-292`
**Issue:** The proactive monitor (`checkMemoryPressure`) only fires at
`memoryPressureTriggerPercent = 95` of the configured ceiling — a value
genuinely observed by the kernel (`PeakJobMemoryUsed`), never
script-influenceable except by truthfully allocating that much memory. The
post-exit backstop (`classifyMemoryExhaustion`), by contrast, reclassifies
any crash whose (script-controlled) stderr tail merely *contains* one of
`v8AllocationFailureSignatures` as long as the kernel-observed peak reached
only `memoryExhaustionCorroborationPercent = 50` of the ceiling — a bar that
is legitimate, permitted, and cheap for a script to clear on purpose (e.g. a
64 MB profile only needs ~33 MB of real, in-budget allocation).

Because a "zero-permission" Deno script can still write directly to the
process's raw OS-level stderr (e.g. `Deno.stderr.writeSync(...)`, which
bypasses the runtime shim's `console.*` → stdout `LogFrame` interception
entirely — nothing in `session.go`/`capability.go` restricts that), a script
can:
1. Allocate ~55% of its configured `MemoryLimitMB` (a real, in-budget,
   perfectly legal amount of memory).
2. Write `"Fatal JavaScript out of memory"` (or any other listed signature)
   directly to stderr.
3. Crash or exit non-zero for a completely unrelated reason (a real bug, an
   unexpected code path, or something the script author wants hidden from
   the run's audit trail).

`session.go`'s post-`cmd.Wait()` backstop (`internal/script/session.go:949-958`)
will then relabel that run's outcome as
`GOLC_SCRIPT_MEMORY_EXCEEDED: run exceeded its 64 MB memory limit` instead of
surfacing the script's actual failure — exactly the T-08G-02 risk this file's
own comments name ("masking a real bug behind a benign-looking limit
message") but do not fully close. The corroboration check is real (this is
not "trust script-controlled text alone"), but the 50%-vs-95% gap between the
two thresholds leaves a wide, cheaply-reachable window for a script to choose
its own reported failure cause.

**Fix:** Either raise `memoryExhaustionCorroborationPercent` much closer to
`memoryPressureTriggerPercent` (e.g. 90%, so the corroborating peak has to be
nearly as compelling as the proactive trigger itself — a peak that high is
much harder to reach "on purpose but harmlessly" without genuinely being at
risk of the real ceiling), or drop the signature-substring approach in favor
of a mechanism that cannot be forged from inside the sandboxed process (e.g.
having the runtime shim's own uncaught-exception/unhandled-rejection handler
emit a structured, host-verifiable signal over the existing `LogFrame`/`Done`
protocol rather than relying on freeform raw stderr text at all).

### WR-02: The post-exit memory backstop is unreachable once `runDispatchIO` has already marked the run `Failed`

**File:** `internal/script/session.go:928-959`
**Issue:** `classifyMemoryExhaustion` is only invoked inside:
```go
} else if waitErr != nil && outcome.Status == show.ScriptRunStatusSucceeded {
    ...
    if reason := classifyMemoryExhaustion(outcome.Reason, peak, limits); reason != nil {
        outcome.Status = show.ScriptRunStatusTerminated
        outcome.Reason = reason.String()
    }
}
```
`outcome.Status` is only still `Succeeded` at this point if `runDispatchIO`
never hit its own scan-error or decode-error paths (`session.go:576-589`),
each of which sets `outcome.Status = show.ScriptRunStatusFailed` directly. A
genuine OOM kill is exactly the kind of event that can sever the child's
stdout mid-frame (e.g. the process is killed while it is mid-write of a
`CmdCallFrame`), which is plausibly read by `newFrameReader`/`scanFrameLine`
as a scan error rather than a clean EOF. In that interleaving,
`outcome.Status` is already `Failed` by the time this branch is reached, so
the `else if` condition is false and the memory backstop never runs for that
run — it permanently reports a generic protocol-violation-flavored `Failed`
outcome instead of the intended
`Terminated: memory limit exceeded (X MB)` message, even though the run
actually hit the resource cause 08-14 exists to catch.

**Fix:** Decouple the memory-backstop check from the `outcome.Status ==
Succeeded` guard — run `classifyMemoryExhaustion` whenever `waitErr != nil`
(or more precisely, whenever the run is not already known-terminated via
`run.terminationReason()`), regardless of what `runDispatchIO` set
`outcome.Status` to, e.g.:
```go
} else if waitErr != nil {
    if outcome.Status == show.ScriptRunStatusSucceeded {
        outcome.Status = show.ScriptRunStatusFailed
        if outcome.Reason == "" {
            outcome.Reason = waitErr.Error()
        }
    }
    var peak uint64
    if memSampler != nil {
        if sampled, sampleErr := memSampler.peakMemoryBytes(); sampleErr == nil {
            peak = sampled
        }
    }
    if reason := classifyMemoryExhaustion(outcome.Reason, peak, limits); reason != nil {
        outcome.Status = show.ScriptRunStatusTerminated
        outcome.Reason = reason.String()
    }
}
```

### WR-03: Proactive 95% trigger can kill an otherwise-successful run in its final moments

**File:** `internal/script/memorywatch.go:47-98`, `internal/script/capability.go:242-257`
**Issue:** `checkMemoryPressure` fires purely on the kernel-observed peak
crossing 95% of the ceiling, with no awareness of whether the run is about to
finish. A script that legitimately needs to approach (but never exceed) its
configured `MemoryLimitMB` — e.g. building one large buffer right before its
final `console.log`/`Deno.exit` — can have its 100ms-granularity poll observe
a peak ≥95% a few milliseconds before the script would have completed
successfully, and be killed with `Terminated: memory limit exceeded` instead
of allowed to finish. This is a deliberate design trade-off (the code's own
`08-14-PLAN.md's threat register, T-08G-06` comment acknowledges a lower
trigger would be worse), and 95% is a reasonable choice, but there is no
mechanism (e.g. requiring the peak to *also* still be climbing, or a short
debounce) to avoid false-positive kills of scripts intentionally operating
close to their configured ceiling. Worth confirming this trade-off is the
one the product actually wants, since "hard boundary, no grace period"
(the same philosophy `checkDeadline` uses) is more defensible for a
wall-clock deadline than for a noisy, poll-sampled memory curve.
**Fix:** No code change strictly required if this is the accepted trade-off;
consider requiring two consecutive above-trigger samples (200ms of sustained
pressure) before terminating, to reduce the false-positive rate against a
single transient spike while retaining a still-generous safety margin ahead
of the kernel's actual OOM denial.

## Info

### IN-01: No test exercises `classifyMemoryExhaustion`'s corroboration boundary or the forged-signature-with-legitimate-allocation scenario

**File:** `internal/script/capability_test.go:334-377`
**Issue:** `TestClassifyMemoryExhaustionSignatureAndCorroboration` covers
~97% (corroborated) and ~3% (not corroborated) peaks, but never exercises the
`memoryExhaustionCorroborationPercent` boundary itself (peak exactly at or
one byte below 50%), nor the WR-01 scenario above (a signature matched at a
peak that is corroborated-but-clearly-unrelated to the actual crash cause).
Given this file's own comments explicitly call out the spoofing threat
(T-08G-02), a boundary test would make the current 50% choice a deliberate,
visible, test-locked decision rather than an implicit one.
**Fix:** Add a boundary case at exactly `memoryExhaustionCorroborationPercent`
(mirroring `TestCheckMemoryPressureBoundary`'s pattern for the 95% trigger).

### IN-02: `memoryLimitReason`'s "single shared constructor" contract has no test asserting the proactive and backstop code paths render byte-identical strings

**File:** `internal/script/capability.go:227-240`
**Issue:** `session.go:944` documents "Both mechanisms produce the identical
GOLC_SCRIPT_MEMORY_EXCEEDED reason text (memoryLimitReason is their single
shared constructor), so which one wins here is not observable to the user" —
true by construction since both call `memoryLimitReason`, but there is no
single test that pins both `checkMemoryPressure(...).String()` and
`classifyMemoryExhaustion(...).String()` against each other for the same
`MemoryLimitMB`, so a future edit to one call site (e.g. someone inlines a
slightly different `fmt.Sprintf` instead of reusing the constructor) would
not be caught by any existing test other than the two separate
`TestCheckMemoryPressureRendersExactSentence`-style assertions.
**Fix:** Low priority; a single cross-check test would harden this guarantee
but is not required given both paths already funnel through one function.

---

_Reviewed: 2026-07-30_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
