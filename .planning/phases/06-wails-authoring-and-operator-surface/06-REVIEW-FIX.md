---
phase: 06-wails-authoring-and-operator-surface
fixed_at: 2026-07-23T23:51:00Z
review_path: .planning/phases/06-wails-authoring-and-operator-surface/06-REVIEW.md
iteration: 1
findings_in_scope: 5
fixed: 5
skipped: 0
status: all_fixed
---

# Phase 06: Code Review Fix Report

**Fixed at:** 2026-07-23T23:51:00Z
**Source review:** .planning/phases/06-wails-authoring-and-operator-surface/06-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 5 (2 Critical, 3 Warning -- `fix_scope: critical_warning`; IN-01/IN-02 excluded from scope)
- Fixed: 5
- Skipped: 0

_Note: this REVIEW.md is scoped to the gap-closure round (06-09..06-12: MIDI safety/master dispatch, FixturePatch/ArtnetConfig/SceneProgramming on-screen surfaces). It supersedes an earlier full-phase REVIEW.md/REVIEW-FIX.md pair for this same phase directory, whose CR-01..CR-03/WR-01..WR-03 findings (a separate set, already fixed in a prior pass) are unrelated to the CR-01/CR-02/WR-01..WR-03 IDs fixed below -- the IDs are reused per-review, not globally unique across review passes._

## Fixed Issues

### CR-01: MIDI-triggered safety/master dispatch silently discards the daemon's response

**Files modified:** `internal/wails/svc_midi.go`
**Commit:** 3cb0d73
**Applied fix:** Applied the review's minimum-bar suggested fix exactly: `dispatchSafetyTrigger` and `dispatchMasterSet` now capture the `ipc.Result` their direct daemon dial returns and `log.Printf` a `GOLC_WAILS_MIDI_SAFETY_DISPATCH_FAILED`/`GOLC_WAILS_MIDI_MASTER_DISPATCH_FAILED` diagnostic (route/ref plus the daemon's own `Stderr`) whenever `ExitCode != 0`, instead of discarding the result outright. This mirrors `app.go`'s existing `log.Printf("GOLC_WAILS_..._FAILED: ...")` convention (hotkey registration/daemon spawn failures) rather than inventing a new diagnostic format. Did not implement the review's "Better" alternative (a new operator-visible feedback/banner push) -- that would require a new event schema and frontend wiring, a materially larger change than a narrow bug fix; flagged below for human follow-up.

**Verification:** `go build ./...` / `go vet ./...` pass. `TestMidiServiceFaderTakeoverCrossToCatchAndButtonActsImmediately` (an existing test that exercises both `dispatchSafetyTrigger` and `dispatchMasterSet` against an unreachable daemon in the test environment) now visibly emits both new log lines (`GOLC_WAILS_MIDI_MASTER_DISPATCH_FAILED: ref=grand_master: ...` and `GOLC_WAILS_MIDI_SAFETY_DISPATCH_FAILED: route=artnet safety blackout: ...`) confirming the fix fires exactly as intended, with the test itself still passing unmodified.

### CR-02: `AddPoolMemberPreview`'s pipe-delimited spec construction has no delimiter guard

**Files modified:** `internal/wails/svc_fixturepatch.go`, `internal/wails/svc_fixturepatch_test.go`
**Commit:** 0ec1e4a
**Applied fix:** Applied the review's suggested fix exactly: `AddPoolMemberPreview` now rejects (before ever building the `"%s|%s|%s"` spec string) any of `stableKey`/`contentHash`/`mode` that contains the `|` delimiter `internal/command/pool.go`'s `parsePoolMemberSpec` splits on, returning `Result{ExitCode: 2, Stderr: "GOLC_WAILS_POOL_MEMBER_FIELD_INVALID: ..."}` instead of silently constructing a spec that mis-splits into the wrong three fields.

**Verification:** Added `TestFixturePatchServiceRejectsEmbeddedDelimiterInMemberFields` (three subtests: an embedded `|` in `stableKey`, `contentHash`, and `mode` respectively, each asserting the new `GOLC_WAILS_POOL_MEMBER_FIELD_INVALID` diagnostic). Full `internal/wails` package test suite passes.

### WR-01: `FixturePatch.tsx` re-implements its own Wails binding layer instead of using `wailsBridge.ts`

**Files modified:** `frontend/src/lib/wailsBridge.ts`, `frontend/src/components/FixturePatch/FixturePatch.tsx`
**Commit:** 1b36642
**Applied fix:** Applied the review's suggested fix: added `createPool`/`addPoolMemberPreview`/`removePoolMemberPreview`/`applyPatch`/`createDeployment`/`activateDeployment`/`listPatch` wrapper functions (plus `offlinePatchView`, mirroring `offlineArtnetStatus`/`offlineProgrammingView`'s "never blank" fallback contract) and exported `PatchView`/`PatchPoolView`/`PatchPoolMemberView`/`PatchInstanceView`/`PatchDeploymentView` to `wailsBridge.ts`. `FixturePatch.tsx` now imports these instead of declaring its own local `FixturePatchServiceBinding`/`GoResult`/`fixturePatchService()`/`PatchView`-family types, removing the `as unknown as` cast entirely (the bridge's `FixturePatchServiceBinding` now structurally matches the already-declared global `window.go.wails.FixturePatchService` shape, so no cast is needed). `refreshPatch`'s initial `ListPatch` load also converges on the same "call the wrapper, let it degrade silently to `offlinePatchView()`" pattern `ArtnetConfig.tsx`/`SceneProgramming.tsx` already use, rather than FixturePatch.tsx's own pre-existing bespoke bridge-unavailable pre-check; mutation handlers (`CreatePool`/`AddPoolMemberPreview`/`ApplyPatch`/`CreateDeployment`/`ActivateDeployment`) are unaffected -- each wrapper's `bridgeUnavailableResult()` fallback carries the identical `GOLC_WAILS_BRIDGE_UNAVAILABLE` stderr text the removed pre-checks used to hardcode, so `assertOk`+`errorMessage` still surface the same banner text on a missing bridge. `removePoolMemberPreview` was added to `wailsBridge.ts` for completeness (mirroring the pre-existing, still-unused `RemovePoolMemberPreview` binding declaration) but is not yet called from any component -- this component has no remove-member control today, unchanged from before this fix.

**Verification:** `npx tsc --noEmit` (frontend) passes with zero errors. `npm run build` (`tsc --noEmit && vite build`) passes. `go build ./...` / `go vet ./...` / `internal/wails` test suite unaffected (Go-side files untouched by this fix).

### WR-02: `EventPusher`'s single-key-per-event-name queue drops concurrent MIDI feedback across mappings

**Files modified:** `internal/wails/events.go`, `internal/wails/events_test.go` (new)
**Commit:** 29080f3
**Applied fix:** Applied the review's suggested fix: added a `midiFeedback map[string]MidiFeedback` field to `EventPusher`, staged separately from `latest`'s single-value-per-event-name map. `QueueMidiFeedback` now keys by `snapshot.MappingID` into this new map instead of overwriting the single `"midi:feedback"` slot in `latest`. `flush` drains both maps each tick: `latest` unchanged (still one push per distinct event name, e.g. `"status:update"`), and `midiFeedback` now emits one `"midi:feedback"` `EventsEmit` call per staged mapping ID -- so a tick in which two distinct mappings (e.g. two faders) both produced feedback now delivers both, instead of only the most-recently-queued mapping's feedback silently overwriting the other. The emitted event name (`"midi:feedback"`) and payload shape (`MidiFeedback`) are unchanged on the wire -- only the server-side staging granularity changed -- so `onMidiFeedback` in `wailsBridge.ts` needed no changes.

**Verification:** Added `internal/wails/events_test.go` with three new unit tests directly against `EventPusher` (no real Wails runtime needed): `TestEventPusherFlushDeliversEveryStagedMappingsMidiFeedback` (two distinct mapping IDs staged before one flush -> both delivered, reproducing the exact WR-02 scenario), `TestEventPusherFlushOverwritesSameMappingWithLatest` (two updates to the SAME mapping ID within one tick still coalesce to the latest value only -- proving the fix doesn't turn per-mapping staging into an unbounded backlog), and `TestEventPusherFlushKeepsStatusUpdateSingleValueBehavior` (proving `QueueStatus`'s pre-existing single-value `"status:update"` slot is unaffected). All three pass; full `internal/wails` package suite (including the pre-existing `TestMidiServiceFaderTakeoverCrossToCatchAndButtonActsImmediately`, which the review noted does not itself exercise the multi-mapping-per-tick scenario) passes under `-race`.

### WR-03: Duplicated ad-hoc bridge-helper boilerplate across the three new components

**Files modified:** `frontend/src/lib/wailsBridge.ts`, `frontend/src/components/FixturePatch/FixturePatch.tsx`, `frontend/src/components/ArtnetConfig/ArtnetConfig.tsx`, `frontend/src/components/SceneProgramming/SceneProgramming.tsx`
**Commit:** e5b56e4
**Applied fix:** Applied the review's suggested fix: moved `errorMessage(err: unknown): string` and `assertOk(result: WailsResult, action: string): void` into `wailsBridge.ts` as exported functions (using the file's own existing `WailsResult` type rather than `SceneProgramming.tsx`'s previously locally-duplicated structural type), and removed the byte-for-byte-identical local copies from all three components, importing from `wailsBridge.ts` instead. `ArtnetConfig.tsx` only duplicated `errorMessage` (it never had its own `assertOk`, using inline `throw new Error(...)` instead) -- only that one import was added there.

**Verification:** `npx tsc --noEmit` (frontend) passes with zero errors. `npm run build` (`tsc --noEmit && vite build`) passes.

## Skipped Issues

None -- all in-scope findings were fixed. IN-01 (`findConflictingMapping` pass-through wrapper) and IN-02 (orphaned cached impact plan on preview-target switch) were excluded by `fix_scope: critical_warning` and left untouched.

## Full Verification (run after every fix in this report)

- `go build ./...` -- pass
- `go vet ./...` -- pass
- `go test ./internal/wails/... -race` -- pass (all tests, including the 3 new `events_test.go` tests and the existing `svc_fixturepatch_test.go`/`svc_midi_test.go` suites)
- `cd frontend && npm run build` (`tsc --noEmit && vite build`) -- pass

## Notes for Human Review

- **CR-01** implemented the review's minimum-bar fix (server-side logging) only, not the review's "Better" alternative (a distinct operator-visible "dispatch failed" push/banner so a live-show operator -- not just a log file -- learns a MIDI-triggered Blackout/master-level change never reached the daemon). That richer fix would need a new feedback event schema and frontend wiring (a new component, not a narrow bug patch) and was judged out of scope for an atomic code-review fix; recommend a human/product decision on whether the richer operator-visible signal is warranted before the next live-show use of MIDI-mapped safety controls.
- **WR-01** changed `FixturePatch.tsx`'s initial `ListPatch` load to silently degrade to an empty `PatchView` on a missing bridge (matching `ArtnetConfig.tsx`/`SceneProgramming.tsx`'s own established convention) rather than the pre-existing FixturePatch-specific behavior of showing an explicit `GOLC_WAILS_BRIDGE_UNAVAILABLE` error banner on initial load. All *mutation* handlers (Create/Add/Apply/Activate) still surface that exact same diagnostic text via `assertOk`+`errorMessage` when the bridge is missing -- only the very first, pre-any-user-action list read's error-banner behavior changed, converging FixturePatch.tsx onto the same pattern its two sibling components already use. Recommend a human confirm this convergence (rather than divergence) is the intended UX before end-of-phase UAT.

---

_Fixed: 2026-07-23T23:51:00Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
