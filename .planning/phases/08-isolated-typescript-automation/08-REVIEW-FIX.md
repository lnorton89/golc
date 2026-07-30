---
phase: 08-isolated-typescript-automation
fixed_at: 2026-07-30T17:15:00Z
review_path: .planning/phases/08-isolated-typescript-automation/08-REVIEW.md
iteration: 1
findings_in_scope: 3
fixed: 3
skipped: 0
status: all_fixed
---

# Phase 08: Code Review Fix Report

**Fixed at:** 2026-07-30T17:15:00Z
**Source review:** .planning/phases/08-isolated-typescript-automation/08-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 3 (critical_warning scope — CR-* and WR-* only, IN-* excluded)
- Fixed: 3
- Skipped: 0

## Fixed Issues

### CR-01: ScriptService's live event stream is never started, so the Scripts debug/log UI is non-functional in the shipped app

**Files modified:** `cmd/golc-desktop/main.go`
**Commit:** 78b2e920
**Applied fix:** Added `scriptService.StartScriptEventStream(ctx)` to `OnStartup` (immediately after `safetyService.StartStatusPush(ctx)`, mirroring the existing `midiService.StartFeedback(ctx)` sibling pattern) and `scriptService.StopScriptEventStream()` to `OnShutdown` (immediately after `midiService.StopFeedback()`, before `safetyService.StopStatusPush()`, preserving the file's documented reverse-order shutdown discipline). Verified both methods exist on `ScriptService` with matching signatures in `internal/wails/svc_script.go:621,666`. `gofmt -l` reports no issues; `go build ./cmd/golc-desktop/...` fails only on a pre-existing, unrelated `//go:embed all:frontend/dist` error because `frontend/dist` is not built in this worktree — not caused by this change.

### WR-01: `CallOutcome.Message` bypasses the single-redaction-point invariant for two failure branches, leaking into the returned run outcome

**Files modified:** `internal/script/session.go`
**Commit:** 67651f66
**Applied fix:** In `dispatchCmdCall`, all three failure branches (already-terminating, `h.enforce` scope/rate/deadline denial, and `buildRouteArgs` params-decode failure) now compute `security.Redact(...)` exactly once into a local `redactedMessage` and reuse that same redacted value for both the `CmdResultFrame.Message` sent to the child and the `CallOutcome.Message` returned into `RunOutcome.Outcomes`. Previously the already-terminating and params-decode branches set `CallOutcome.Message` to the raw, unredacted string while only the `CmdResultFrame` was redacted. `go build ./internal/script/...` and `go test ./internal/script/...` (including `TestDispatchCmdCallKnownMethod`, `TestDispatchCmdCallUnknownMethod`, `TestDispatchCmdCallParamsInvalidNeverReachesExecutor`) pass.

### WR-02: `publishCallOutcome`'s route-fallback doc comment overstates what the code does

**Files modified:** `internal/script/session.go`
**Commit:** 0b05e316
**Applied fix:** `publishCallOutcome`'s `PublishScriptEvent(ScriptEvent{...})` call now uses the already-computed fallback `route` (falls back to `method` when `outcome.Route` is empty) instead of the raw, possibly-empty `outcome.Route`. This makes the live-streamed `script.outcome` event carry the same fallback-applied route as the audit-trail `api.MutationEvent`, matching the function's existing doc comment ("an audited/observed outcome always carries the caller's attempted route"). No doc comment change was needed since the comment already described this as the intended behavior — the code now matches it. `go build ./internal/script/...` and `go test ./internal/script/...` pass.

## Skipped Issues

None — all in-scope findings were fixed.

---

_Fixed: 2026-07-30T17:15:00Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
