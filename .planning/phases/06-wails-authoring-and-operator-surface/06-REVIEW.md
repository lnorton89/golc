---
phase: 06-wails-authoring-and-operator-surface
reviewed: 2026-07-23T16:33:02-07:00
depth: standard
files_reviewed: 17
files_reviewed_list:
  - internal/wails/svc_midi.go
  - internal/wails/svc_midi_test.go
  - internal/wails/svc_fixturepatch.go
  - internal/wails/svc_fixturepatch_test.go
  - internal/wails/svc_artnetconfig.go
  - internal/wails/svc_artnetconfig_test.go
  - internal/wails/svc_programming.go
  - internal/wails/svc_programming_test.go
  - frontend/src/lib/wailsBridge.ts
  - frontend/src/components/FixturePatch/FixturePatch.tsx
  - frontend/src/components/FixturePatch/FixturePatch.module.css
  - frontend/src/components/ArtnetConfig/ArtnetConfig.tsx
  - frontend/src/components/ArtnetConfig/ArtnetConfig.module.css
  - frontend/src/components/SceneProgramming/SceneProgramming.tsx
  - frontend/src/components/SceneProgramming/SceneProgramming.module.css
  - frontend/src/App.tsx
  - cmd/golc-desktop/main.go
findings:
  critical: 2
  warning: 3
  info: 2
  total: 7
status: issues_found
---

# Phase 06: Code Review Report (gap-closure scope: 06-09..06-12)

**Reviewed:** 2026-07-23T16:33:02-07:00
**Depth:** standard
**Files Reviewed:** 17
**Status:** issues_found

## Summary

This review is scoped to the four gap-closure plans that closed VERIFICATION.md's two
FAILED truths (MIDI dispatch, plus the FixturePatch/ArtnetConfig/SceneProgramming on-screen
surfaces); the phase's original 8 plans were already reviewed/fixed in the prior pass
(superseded by this file for this narrowed scope). The three new services
(`FixturePatchService`, `ArtnetConfigService`, `ProgrammingService`) correctly mirror
`svc_playback.go`/`svc_surface.go`'s build-a-fresh-registry-and-`Execute`-through-it
pattern for every mutation — no second, GUI-only mutation path was found for
scene/theme/chase/motion/preset/blend/pool/deployment mutations. `svc_midi.go`'s new
dispatch logic (Gap B[1]) also routes scene/layer mutations through the same registry.

Two things did not hold up under closer inspection:

1. **`svc_midi.go`'s new safety/master dispatch path** (the one part of this round that
   deliberately bypasses the command registry and dials the daemon directly, mirroring
   `svc_safety.go`'s `toggle`) discards the daemon's response entirely, with no synchronous
   caller and no logging to surface a failure — unlike every other mutation path in this
   package, a MIDI-triggered Blackout/Stop-Release-All/Revoke-Automation/master-level
   change that fails to reach the daemon has **no operator-visible or log-visible failure
   signal anywhere**.
2. **`FixturePatchService.AddPoolMemberPreview`** builds the `pool update --add` spec via
   raw `fmt.Sprintf("%s|%s|%s", ...)` string concatenation without validating that the
   three GUI-supplied fields (`stableKey`/`contentHash`/`mode`) don't themselves contain the
   `|` delimiter the backend route splits on — a value containing an embedded `|` is
   silently misattributed to the wrong field rather than rejected.

The three components' shared files (`wailsBridge.ts`, `App.tsx`, `cmd/golc-desktop/main.go`)
are consistent for `App.tsx` (each panel mounted exactly once) and `main.go` (all three
services constructed and bound once, names matching the TS binding declarations), but
`wailsBridge.ts` itself is asymmetric across the three passes: `ArtnetConfigService` and
`ProgrammingService` both get exported, centrally-typed helper functions (the pattern the
file's own header comment prescribes), while `FixturePatchService` gets only a bare
interface declaration — `FixturePatch.tsx` re-declares its own local, structurally-similar-
but-independent binding types and casts through `as unknown as` instead.

## Critical Issues

### CR-01: MIDI-triggered safety/master dispatch silently discards the daemon's response

**File:** `internal/wails/svc_midi.go:373-420`
**Issue:** `dispatchSafetyTrigger` and `dispatchMasterSet` are the two dispatch paths this
gap-closure round added that intentionally bypass the command registry and dial the daemon
directly (mirroring `SafetyService.toggle`'s established pattern for safety-critical
actions). Unlike `SafetyService.toggle`, which returns the `ipc.Result` to a synchronous
Wails-bound caller that can render it as an error banner, these two methods are called from
`dispatchMapping`, itself called from `dispatchToActiveSurface` inside the background
`dispatchLoop` goroutine — there is no caller left to hand a failure to:

```go
func (s *MidiService) dispatchSafetyTrigger(control operatorsurface.SafetyControl) {
	route := safetyRouteFor(control)
	if route == "" {
		return
	}
	s.dialFn()(s.pipeName, ipc.Request{
		Route: string(route),
		Args:  []string{"--on", "true", "--source", "manual"},
	})
}
```

The returned `ipc.Result` (which would carry `ExitCode`/`Stderr` on a failed dial, e.g.
daemon offline) is discarded outright. `dispatchMasterSet` has the identical pattern. Both
are invoked from `dispatchToActiveSurface`, which unconditionally calls `emitMidiFeedback`
right after `dispatchMapping` regardless of whether the dial succeeded — so a MIDI-mapped
Blackout button press while the daemon is unreachable produces zero indication anywhere
(no console log, no pushed event, no error banner) that the safety action never reached the
daemon. Every other mutation path added in this same round (`FixturePatchService`,
`ArtnetConfigService`, `ProgrammingService`, and `svc_midi.go`'s own
`dispatchSceneSwitch`/`dispatchLayerToggle`, which go through `executeWithRetry`) at least
returns a `Result` a caller could inspect; this is the one path in the newly-reviewed code
that cannot fail loudly anywhere.

**Fix:** At minimum, log the failure server-side so it is diagnosable (`ipc.Result` already
carries `Stderr`):
```go
func (s *MidiService) dispatchSafetyTrigger(control operatorsurface.SafetyControl) {
	route := safetyRouteFor(control)
	if route == "" {
		return
	}
	result := s.dialFn()(s.pipeName, ipc.Request{
		Route: string(route),
		Args:  []string{"--on", "true", "--source", "manual"},
	})
	if result.ExitCode != 0 {
		log.Printf("GOLC_WAILS_MIDI_SAFETY_DISPATCH_FAILED: route=%s: %s", route, result.Stderr)
	}
}
```
Better: surface a distinct "dispatch failed" push (a new/extended feedback event, or a
`QueueStatus`-style banner) so the operator — not just a log file nobody is tailing during a
live show — learns the Blackout press did not take effect, rather than seeing the on-screen
fader/button state update as if it had.

### CR-02: `AddPoolMemberPreview`'s pipe-delimited spec construction has no delimiter guard

**File:** `internal/wails/svc_fixturepatch.go:117-127`
**Issue:**
```go
func (s *FixturePatchService) AddPoolMemberPreview(poolName, stableKey, contentHash, mode string) Result {
	spec := fmt.Sprintf("%s|%s|%s", stableKey, contentHash, mode)
	result := s.execute([]string{
		"pool", "update", poolName,
		"--add", spec,
		...
```
`internal/command/pool.go`'s `parsePoolMemberSpec` splits this value with
`strings.SplitN(raw, "|", 3)` and only checks that exactly 3 non-empty parts resulted — it
has no way to detect that one of the *original* three fields itself contained a `|`. If a
GUI author pastes a `stableKey` or `contentHash` value that contains an embedded `|` (e.g.
`"acme|par64"`), the resulting spec `"acme|par64|sha256:...|Standard"` still splits into
exactly 3 non-empty parts — just the *wrong* ones: `FixtureStableKey="acme"`,
`FixtureContentHash="par64"`, `Mode="sha256:...|Standard"`. No error is raised anywhere;
the pool member is silently created with a corrupted stable key / content hash / mode
triple. Once `ApplyPatch` commits it, this bad fixture identity is now the on-disk source
of truth for that pool member with no diagnostic ever having fired.

**Fix:** Reject (client-side, before ever building the spec) any of the three inputs that
contain the delimiter, surfacing an explicit error instead of silently mis-splitting:
```go
func (s *FixturePatchService) AddPoolMemberPreview(poolName, stableKey, contentHash, mode string) Result {
	for _, field := range []string{stableKey, contentHash, mode} {
		if strings.Contains(field, "|") {
			return Result{ExitCode: 2, Stderr: "GOLC_WAILS_POOL_MEMBER_FIELD_INVALID: fixture stable key/content hash/mode must not contain \"|\"\n"}
		}
	}
	spec := fmt.Sprintf("%s|%s|%s", stableKey, contentHash, mode)
	...
```

## Warnings

### WR-01: `FixturePatch.tsx` re-implements its own Wails binding layer instead of using `wailsBridge.ts`

**File:** `frontend/src/components/FixturePatch/FixturePatch.tsx:117-157`; `frontend/src/lib/wailsBridge.ts:129-142`
**Issue:** `wailsBridge.ts`'s own header comment states the project convention explicitly:
"every component imports its binding call through this file's helper functions ... rather
than re-declaring `declare global {...}` itself." `ArtnetConfig.tsx` and
`SceneProgramming.tsx` (06-11/06-12, written after 06-10) both follow this: they import
`configureArtnetTarget`/`fetchArtnetStatus`/... and `createScene`/`listProgramming`/... from
`wailsBridge.ts`, which exports typed, graceful-degradation wrapper functions for every
bound method. `FixturePatch.tsx` (06-10) does not — it declares its own local, independent
copy of the binding shape and a locally-duplicated `GoResult` type (structurally identical
to `wailsBridge.ts`'s exported `WailsResult`, but a separate declaration two files can drift
apart on), and reaches into the global directly:
```ts
function fixturePatchService(): FixturePatchServiceBinding | undefined {
  return window.go?.wails?.FixturePatchService as unknown as
    | FixturePatchServiceBinding
    | undefined;
}
```
The `as unknown as` cast bypasses TypeScript's structural checking entirely, so if
`svc_fixturepatch.go`'s bound method signatures ever change, `FixturePatch.tsx`'s local
interface would silently go stale with no compiler error — exactly the risk
`wailsBridge.ts`'s single-source-of-truth convention exists to prevent, and exactly the
"three separate executor passes" drift this round should have caught since 06-11/06-12
landed after 06-10 and had the correct pattern already in front of them.
**Fix:** Add `createPool`/`addPoolMemberPreview`/`removePoolMemberPreview`/`applyPatch`/
`createDeployment`/`activateDeployment`/`listPatch` wrapper functions to `wailsBridge.ts`
(mirroring `listArtnetInterfaces`/`configureArtnetTarget`'s shape), export `PatchView` and
friends from there, and have `FixturePatch.tsx` import them instead of declaring its own
`FixturePatchServiceBinding`/`GoResult`.

### WR-02: `EventPusher`'s single-key-per-event-name queue drops concurrent MIDI feedback across mappings

**File:** `internal/wails/svc_midi.go:528-539`; `internal/wails/events.go:80-88` (referenced, not in this round's file list)
**Issue:** `dispatchToActiveSurface` calls `emitMidiFeedback` → `QueueMidiFeedback` once per
arbitrated MIDI message, for whichever mapping just received a message. `EventPusher.queue`
stages snapshots in `map[string]interface{}` keyed by **event name** only
(`"midi:feedback"`), not per-mapping-ID:
```go
func (p *EventPusher) queue(eventName string, snapshot interface{}) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.latest[eventName] = snapshot
}
```
Every 25ms tick flushes at most one `MidiFeedback` value total, system-wide, regardless of
how many distinct mappings produced feedback in that window. A surface with more than one
active mapping (e.g. two faders, or a fader plus a button) will only ever have the
most-recently-queued mapping's feedback survive to the next flush — the other mapping(s)'
feedback for that tick is silently overwritten and never delivered, so their on-screen
slider/armed indicators go stale whenever another control is touched in the same ~25ms
window. `svc_midi_test.go`'s own `TestMidiServiceFaderTakeoverCrossToCatchAndButtonActsImmediately`
does not catch this because it spaces out each `out.Send` with a `waitForCondition` that lets
one control's feedback flush before the next control is touched — the test never exercises
two mappings updating within the same tick. `events.go` is not part of this round's changed
files, but `svc_midi.go`'s per-mapping feedback design (new in this round) is the first
caller to actually depend on multi-mapping delivery, and it cannot get it from this queue.
**Fix:** Key `EventPusher.latest`'s MIDI slot by mapping ID (e.g.
`"midi:feedback:" + mappingID`, or a `map[string]MidiFeedback` staged separately from the
single-value `status:update` slot) and flush every staged mapping's feedback each tick.

### WR-03: Duplicated ad-hoc bridge-helper boilerplate across the three new components

**File:** `frontend/src/components/FixturePatch/FixturePatch.tsx:155-157`; `frontend/src/components/ArtnetConfig/ArtnetConfig.tsx:50-52`; `frontend/src/components/SceneProgramming/SceneProgramming.tsx:91-99`
**Issue:** `errorMessage(err: unknown): string` is defined identically (byte-for-byte) in
all three components; `assertOk` is defined identically in `FixturePatch.tsx` and
`SceneProgramming.tsx`. None of these are exported from `wailsBridge.ts`, so any future
change to the "how do we render a caught error" convention has to be applied in three
places by hand.
**Fix:** Move `errorMessage`/`assertOk` into `wailsBridge.ts` (or a small shared
`frontend/src/lib/wailsResult.ts`) and import from there.

## Info

### IN-01: `findConflictingMapping` is a same-signature pass-through wrapper around `findMapping`

**File:** `internal/wails/svc_midi.go:874-880`
**Issue:** `findConflictingMapping(mappings, candidate)` is `return findMapping(mappings, candidate)` verbatim — a distinct name for the exact same call, kept only so call sites read differently. Harmless, but it's an extra indirection a reader has to chase to confirm it isn't doing anything different.
**Fix:** Either inline the call at its one call site in `StartLearn`, or leave as-is with a one-line comment noting it's purely a readability alias (the current doc comment already gestures at this but a future reader may still expect divergent behavior from the distinct name).

### IN-02: Orphaned cached impact plan when an operator switches "Add Fixture" target mid-preview

**File:** `frontend/src/components/FixturePatch/FixturePatch.tsx:236-242`
**Issue:** `handleStartAddMember` unconditionally resets `pendingPreview` to `null` whenever
the operator clicks "Add Fixture" on a (possibly different) pool row, discarding any
already-computed preview without calling `ApplyPatch` or otherwise releasing it. The plan
remains cached server-side in `FixturePatchService.plans` (keyed by `PlanID`) indefinitely —
harmless functionally (it can never be applied again since the frontend lost its `PlanID`
reference), but it is dead memory that accumulates with normal "start a preview, then decide
to add a different fixture instead" usage.
**Fix:** Not required for correctness; if `FixturePatchService.plans` growth becomes a
concern, consider an explicit `DiscardPlan(planId)` call from `handleStartAddMember`/
`handleCancelPreview`, or a bounded/TTL'd cache.

---

_Reviewed: 2026-07-23T16:33:02-07:00_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
