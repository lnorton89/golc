---
phase: 08-isolated-typescript-automation
reviewed: 2026-07-30T00:00:00Z
depth: standard
files_reviewed: 79
files_reviewed_list:
  - cmd/golc-desktop/main.go
  - config/toolchain.toml
  - frontend/package-lock.json
  - frontend/package.json
  - frontend/src/components/Scripts/ScriptDebugPanel.module.css
  - frontend/src/components/Scripts/ScriptDebugPanel.test.tsx
  - frontend/src/components/Scripts/ScriptDebugPanel.tsx
  - frontend/src/components/Scripts/ScriptEditor.module.css
  - frontend/src/components/Scripts/ScriptEditor.test.tsx
  - frontend/src/components/Scripts/ScriptEditor.tsx
  - frontend/src/components/Scripts/ScriptRunDialog.module.css
  - frontend/src/components/Scripts/ScriptRunDialog.test.tsx
  - frontend/src/components/Scripts/ScriptRunDialog.tsx
  - frontend/src/components/Scripts/monacoTheme.test.ts
  - frontend/src/components/Scripts/monacoTheme.ts
  - frontend/src/lib/wailsBridge.ts
  - frontend/src/shell/WorkspaceRouter.tsx
  - frontend/src/shell/navigation.ts
  - frontend/src/workspaces/build/ScriptsWorkspace.module.css
  - frontend/src/workspaces/build/ScriptsWorkspace.test.tsx
  - frontend/src/workspaces/build/ScriptsWorkspace.tsx
  - frontend/vite.config.ts
  - internal/api/coverage_test.go
  - internal/api/events.go
  - internal/api/events_test.go
  - internal/api/observer.go
  - internal/api/observer_test.go
  - internal/bootstrap/engine.go
  - internal/bootstrap/engine_test.go
  - internal/command/check.go
  - internal/command/generate.go
  - internal/command/script.go
  - internal/command/script_test.go
  - internal/command/scriptdebug.go
  - internal/command/scriptdebug_test.go
  - internal/command/scriptrun.go
  - internal/command/scriptrun_test.go
  - internal/command/scriptsdk_parity_test.go
  - internal/command/scriptstop.go
  - internal/command/scriptstop_test.go
  - internal/command/scriptvalidate.go
  - internal/command/scriptvalidate_test.go
  - internal/projectconfig/model.go
  - internal/projectconfig/model_test.go
  - internal/projectconfig/strict_test.go
  - internal/script/artnet_noninterference_test.go
  - internal/script/capability.go
  - internal/script/capability_test.go
  - internal/script/debugbridge.go
  - internal/script/debugbridge_test.go
  - internal/script/diagnostics.go
  - internal/script/diagnostics_test.go
  - internal/script/events.go
  - internal/script/events_test.go
  - internal/script/host.go
  - internal/script/host_test.go
  - internal/script/jobobject_other.go
  - internal/script/jobobject_windows.go
  - internal/script/jobobject_windows_test.go
  - internal/script/protocol.go
  - internal/script/protocol_test.go
  - internal/script/session.go
  - internal/script/session_audit_test.go
  - internal/script/session_test.go
  - internal/script/stacktrace.go
  - internal/script/stacktrace_test.go
  - internal/script/toolchain.go
  - internal/script/toolchain_test.go
  - internal/script/validate.go
  - internal/script/validate_test.go
  - internal/scriptsdk/coverage_test.go
  - internal/scriptsdk/descriptors.go
  - internal/scriptsdk/generate.go
  - internal/scriptsdk/generate_test.go
  - internal/scriptsdk/generated/golc-runtime.ts
  - internal/scriptsdk/generated/golc.d.ts
  - internal/show/scripts.go
  - internal/show/scripts_test.go
  - internal/show/state.go
  - internal/wails/events.go
  - internal/wails/events_test.go
  - internal/wails/svc_script.go
  - internal/wails/svc_script_test.go
findings:
  critical: 1
  warning: 2
  info: 2
  total: 5
status: issues_found
---

# Phase 08: Code Review Report

**Reviewed:** 2026-07-30T00:00:00Z
**Depth:** standard
**Files Reviewed:** 79
**Status:** issues_found

## Summary

Reviewed the isolated-TypeScript-automation phase end to end: the Deno subprocess host/session protocol, capability/rate/deadline enforcement, the Windows Job Object kill path, the CDP debug bridge, the generated SDK, the `script *` CLI routes, the Wails `ScriptService` binding, and the Scripts workspace/editor/debug-panel/run-dialog frontend. The Go-side sandboxing, redaction, and termination discipline is careful and mostly self-consistent (single-writer termination reason, guaranteed terminal event, Job-Object-first kill ordering, structural zero-import gate ahead of `deno check`).

The one finding that changes the ship/no-ship verdict is structural, not a subtle edge case: `cmd/golc-desktop/main.go` constructs and binds `ScriptService` but never calls its `StartScriptEventStream`/`StopScriptEventStream` lifecycle methods anywhere in `OnStartup`/`OnShutdown` (unlike every sibling service's push mechanism, e.g. `safetyService.StartStatusPush`/`midiService.StartFeedback`). Because the entire live debug/log experience (`ScriptDebugPanel`, breakpoint pause detection, "Stopping…" clearing) is wired exclusively through the `"script:event"` push this stream produces, the shipped desktop app never receives a single live script event, and the Debug feature this phase's own 08-12 plan built is non-functional as committed.

## Critical Issues

### CR-01: ScriptService's live event stream is never started, so the Scripts debug/log UI is non-functional in the shipped app

**File:** `cmd/golc-desktop/main.go:92,114-151,152-159`
**Issue:** `main()` constructs `scriptService := golcwails.NewScriptService(...)` and binds it to the webview, but `OnStartup` never calls `scriptService.StartScriptEventStream(ctx)`, and `OnShutdown` never calls `scriptService.StopScriptEventStream()`. Every other push-based service wires this explicitly (`safetyService.StartStatusPush(ctx)` / `safetyService.StopStatusPush()`, `midiService.StartFeedback(ctx)` / `midiService.StopFeedback()`), but the equivalent `scriptService.Start/StopScriptEventStream()` calls are simply absent.

`ScriptService.StartScriptEventStream` (`internal/wails/svc_script.go:621-636`) is what starts the service's own `EventPusher.Start(ctx)` loop *and* subscribes to `script.SubscribeScriptEvents`, forwarding every live `ScriptEvent` into `QueueScriptEvent` under the `"script:event"` Wails event name. Without that call, `EventPusher.flush` never runs for this service and nothing ever subscribes to the process-wide script event bus — `runtime.EventsEmit(ctx, "script:event", ...)` is never invoked in production.

The frontend depends on this stream exclusively:
- `ScriptsWorkspace.tsx`'s `onScriptEvent` subscription (`frontend/src/workspaces/build/ScriptsWorkspace.tsx:289-299`) is the *only* place `panelStateByScript` (and therefore `pausedLine`/`liveStatus`) is updated while a run is in flight.
- `reduceScriptEvent` derives `pausedLine` from a live `script.status` event's `GOLC_SCRIPT_DEBUG_PAUSED: line=<N>` reason (`ScriptsWorkspace.tsx:146-156`). With no live events, `pausedLine` never becomes non-null, so `ScriptDebugPanel` never shows the "Paused at breakpoint" chip and never renders the Continue/Step Over/Step Into/Step Out controls (`ScriptDebugPanel.tsx:259-274`).
- Because `DebugScript`/`RunScript` (`svc_script.go:440-459`) are full blocking Wails calls that only resolve once the *entire* run finishes, a script paused at a breakpoint (`--inspect-brk`, `host.go`/`session.go`) blocks the backend call indefinitely with no UI signal and no way for the operator to click Continue — the run only ever ends via its deadline timeout.
- `handleStop`'s "Stopping — finishing in-flight commands…" state (`ScriptsWorkspace.tsx:470-485`) is documented to clear only "the moment the run's own guaranteed terminal event arrives over `onScriptEvent`" — which also never arrives, so a stopped run's UI is stuck on "Stopping…" until the operator navigates away and back (triggering a fresh `ListScripts`/`GetScript`, not a state repair).

This is a complete, provable regression of 08-08's live-streaming feature and 08-12's breakpoint/step debugger feature as actually shipped in the desktop binary — every unit/integration test in `svc_script_test.go` exercises `StartScriptEventStream` directly and therefore never catches the missing production call site.

**Fix:**
```go
// cmd/golc-desktop/main.go
OnStartup: func(ctx context.Context) {
    app.OnStartup(ctx)
    safetyService.StartStatusPush(ctx)
    scriptService.StartScriptEventStream(ctx) // <-- add

    for _, killErr := range midi.KillOrphanedMidicatProcesses() {
        log.Printf("GOLC_WAILS_MIDI_ORPHAN_CLEANUP_FAILED: %v", killErr)
    }
    midiService.StartFeedback(ctx)
    // ...
},
OnShutdown: func(ctx context.Context) {
    midiService.DetachDriver()
    midiService.StopFeedback()
    scriptService.StopScriptEventStream() // <-- add
    safetyService.StopStatusPush()
    app.OnShutdown(ctx)
},
```

## Warnings

### WR-01: `CallOutcome.Message` bypasses the single-redaction-point invariant for two failure branches, leaking into the returned run outcome

**File:** `internal/script/session.go:428-434, 446-451, 454-461`
**Issue:** `dispatchCmdCall` builds two different representations of the same failure: the `CmdResultFrame` sent back over the child's stdio (always passed through `security.Redact(...)`) and the `CallOutcome` appended to `RunOutcome.Outcomes` (returned verbatim from `Host.Run`, and ultimately serialized into `"script run"`/`"script debug"`'s Stdout JSON and `ScriptRunOutcomeView.outcomes` in the Wails binding, rendered directly in `ScriptDebugPanel`'s outcome rows).

For the "already terminating" branch (line 428-434) and the `buildRouteArgs` params-decode-failure branch (line 454-461), `CallOutcome.Message` is set to the **raw, unredacted** `reason.Message` / `buildErr.Error()`, while the parallel `CmdResultFrame.Message` sent to the child *is* redacted (`security.Redact(reason.Message)` / `security.Redact(buildErr.Error())`). The package's own doc comments (`events.go`'s "T-08-34: redaction happens once, at the single publication point, so no future sink... can forget it") assert this invariant holds everywhere a `Message`/`Reason` string is externally observable — it does not hold here, because `RunOutcome.Outcomes` is a second externally-observable sink that never passes through `eventBus.publish`'s redaction.

In practice the two vulnerable messages here are host-generated diagnostic text (deadline/rate/scope-violation summaries, a JSON-decode error over the script's own Params), so the immediate exposure is low, but this is exactly the kind of "forgot one sink" gap the single-redaction-point design was meant to structurally prevent, and `buildErr.Error()` in particular echoes back arbitrary attacker-controlled `call.Params` decode failures verbatim.

**Fix:** Redact once, and reuse the redacted value for both the `CmdResultFrame` and the `CallOutcome`:
```go
if reason, terminating := run.terminationReason(); terminating {
    redacted := security.Redact(reason.Message)
    outcome := CallOutcome{
        Method: call.Method, DurationMS: time.Since(started).Milliseconds(),
        Ok: false, Code: reason.Code, Message: redacted,
    }
    return CmdResultFrame{ID: call.ID, Ok: false, Code: reason.Code, Message: redacted}, outcome
}
```
Apply the same pattern to the `h.enforce` branch (line 446-451) and the `buildRouteArgs` branch (line 454-461).

### WR-02: `publishCallOutcome`'s route-fallback doc comment overstates what the code does

**File:** `internal/script/session.go:501-536`
**Issue:** The doc comment states: "route falls back to `call.Method` when descriptor resolution never happened... so an audited/observed outcome always carries the caller's attempted route even when it never reached scriptsdk's registry." The code computes `route := outcome.Route; if route == "" { route = method }` but only uses that fallback value for `api.PublishMutationEvent(api.MutationEvent{Route: route, ...})` (the audit trail). The live `ScriptEvent` published via `PublishScriptEvent` still uses `Route: outcome.Route` directly — the un-fallback-applied, possibly-empty value. So a `script.outcome` event for an unknown-method/scope-denied/rate-denied call (all of which never resolve a `descriptor.Route`) carries an empty `Route` in the live push and debug-panel-visible stream, contradicting "observed... always carries."

**Fix:** Either apply the same fallback to the `ScriptEvent.Route` field, or correct the comment to state the fallback only applies to the audit-trail `MutationEvent`, not the live-streamed `ScriptEvent`:
```go
PublishScriptEvent(ScriptEvent{
    ...
    Route: route, // use the same fallback-applied value
    ...
})
```

## Info

### IN-01: `ScriptDebugPanel`'s `LogRow` `script.status` branch is unreachable dead code

**File:** `frontend/src/components/Scripts/ScriptDebugPanel.tsx:207-213, 243`
**Issue:** `LogRow` has an explicit branch returning `null` for `event.kind === "script.status"` with a comment explaining why status events don't render their own row. But the only caller filters status events out before ever calling `LogRow`: `const streamRows = events.filter((event) => event.kind !== "script.status");` (line 243). The branch inside `LogRow` can therefore never execute.
**Fix:** Either remove the dead branch from `LogRow` (relying on the upstream filter alone) or remove the upstream filter and keep the in-component guard — not both, to avoid a reader having to reconcile two enforcement points for the same invariant.

### IN-02: Run dialog's Advanced numeric fields can render below their own declared `min`

**File:** `frontend/src/components/Scripts/ScriptRunDialog.tsx:189-229`, `internal/show/scripts.go:159-176`
**Issue:** `show.NewScript` leaves `CapabilityProfile.DeadlineSeconds/RatePerSecond/MemoryLimitMB/CPUCapPercent` at their Go zero value (`0`) for a brand-new, never-configured script. If an operator switches a fresh script's Run/Debug dialog to the "Advanced (custom)" preset before ever setting explicit limits, every numeric `Field` (`min={1}` for deadline/rate/CPU, etc.) initializes displaying `0`, which is below its own declared minimum. The value is harmless server-side (`resolvePositiveOrDefault` treats `<=0` as "use the safe default," never as literal zero), but the UI momentarily shows an input pre-populated outside its own validation range.
**Fix:** Seed the dialog's advanced-field initial state with the resolved safe defaults (e.g. via `CapabilityProfile.ResolveResourceLimits()`'s Go-side constants mirrored client-side, or a `Math.max(1, value)` guard) rather than the raw possibly-zero profile fields when `preset === "advanced"` and the field is otherwise unset.

---

_Reviewed: 2026-07-30T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
