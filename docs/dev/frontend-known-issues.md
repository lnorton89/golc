# Frontend known issues

A punch list of findings from ad-hoc reviews of `frontend/src` — bugs, edge
cases, and rough spots noticed but not yet fixed. Not a GSD phase artifact;
this is a plain backlog doc for anyone (human or agent) about to touch the
frontend to skim first.

Each entry should carry:

- **Severity**: `bug` (wrong behavior), `edge-case` (rare/unlikely but real),
  `smell` (works, but risky or confusing), `perf` (works, but wasteful)
- **File:line** pointing at the exact site
- What's wrong, concretely — the failure scenario, not a vague impression
- Confidence: state it if you're not sure it's real (e.g. "looks wrong but
  didn't trace the caller")

Findings are appended, not reorganized — add a dated section per review
pass rather than editing prior entries, so this stays an honest record of
what was found when.

---

## 2026-08-10 — Opus review pass

<!-- Findings appended below by the review agent. -->

### ~~Rebinding a hotkey in Settings also fires the action currently bound to the key you press~~
**Fixed:** `lib/hotkeys.ts` gained `beginHotkeyCapture()`/`isHotkeyCaptureActive()`, a module-scoped suppression flag both live matchers check before acting. DOM propagation control could not do this job: `stopImmediatePropagation()` only suppresses listeners registered *after* the caller, and `useKeyboardWorkflow`'s effect re-registers on binding/snapshot changes, so it can sit on either side of the recording listener. `HotkeySettings` now raises the flag for the lifetime of a recording (and uses `stopImmediatePropagation` rather than `stopPropagation`, which is strictly more correct but not load-bearing).
**Severity:** bug
**File:** `frontend/src/components/HotkeySettings/HotkeySettings.tsx:69`
**Confidence:** high

The rebind capture listener calls `event.stopPropagation()`, but both playback (`useKeyboardWorkflow.ts:156`) and navigation (`useGlobalKeyboardWorkflow.ts:186`) matchers are registered on `window` in the *same* capture phase. `stopPropagation()` only stops the event moving to a different node in the path — it does not stop sibling listeners on the same node (that would need `stopImmediatePropagation`), and `isTypingTarget` doesn't help because the event target is the rebind `<Button>`, not an input. Repro: Settings > Hotkeys, click the key button for "Toggle Base Look", press `W` — the Color Theme layer on the live scene actually toggles before the rebind is recorded. Pressing a digit is worse: `3` switches the show to scene 3 and *then* reports "That key is reserved for scene switching". The same applies to nav-chord recording (`Ctrl+Alt+ArrowDown` navigates the rail while recording).

### "Release all overrides" on the Desk leaves every fader thumb stuck in the "touched" state
**Severity:** bug
**File:** `frontend/src/components/Desk/Desk.tsx:1264`
**Confidence:** high

`handleClearAll` does `setOverrides({})` and never clears `touchedKeys`, unlike `handleFaderClear` (`:1216`) and `releaseLocalOverridesFor` (`:1244`), which both explicitly drop the key so the thumb returns to dimmed grey. The comment at `:1230` even claims `handleClearAll` owns the same `overrides`/`touchedKeys` pair. Repro: drag any fader (thumb turns accent blue), click the global release-all control — the override clears and the value falls back to live, but every previously-dragged thumb stays blue, which per `Fader.tsx:110`'s contract means "currently doing something". Per-channel, per-fixture, and per-universe release all behave correctly; only release-all is wrong.

### ~~Operator-surface detail fetch has no request-id guard — a slow response repopulates the panel for the wrong surface~~
**Fixed:** New shared `hooks/useLatestRequest.ts` generation guard (see the closing note on this section); `refreshDetail` and `MidiPanel`'s `refreshSurfaceDetail` both check it before every commit. Regression tests in `OperatorSurface.detailRace.test.tsx` and `MidiPanel.test.tsx` land two overlapping responses out of order and assert the newer one survives (both verified failing before the fix).
**Severity:** bug
**File:** `frontend/src/components/OperatorSurface/OperatorSurface.tsx:140`
**Confidence:** high

`refreshDetail(name)` awaits `ShowSurface(name)` and unconditionally `setControls(detail.controls)` with no check that `name` is still `selectedName`. Repro: click surface A, then click surface B before A's round trip finishes; if A resolves last, the detail panel header reads "B" while listing A's controls. `handleToggle` then calls `AssignItem(selectedName /* B */, selector(controlFromA))`, assigning A's control to B. `MidiPanel.tsx:149` has the identical unguarded shape for `ShowSurface`+`ListMappings` (there the follow-on `StartLearn(B, controlRefFromA)` is rejected server-side at `command.Authorize`, so it surfaces as an authorization error on a control the user can see listed).

### ~~FixturePatch keeps a stale add-preview after you change the fixture or mode, so Apply commits the wrong plan~~
**Fixed:** `handleSelectFixture` and a new `handleSelectAddMode` both `setPendingPreview(null)`, matching `ProjectFixtures.tsx`. `handlePreviewAddMember` is also generation-guarded so rapid Review Impact clicks can't land out of order.
**Severity:** bug
**File:** `frontend/src/components/FixturePatch/FixturePatch.tsx:237`
**Confidence:** high

`handleSelectFixture` sets `selectedFixture`/`addMode` but never clears `pendingPreview`, and the mode `<Select onValueChange={setAddMode}>` doesn't either. The render gate at `:683` only checks `pendingPreview.poolName === p.name`. Repro: open Add Fixture on a pool, pick a 4-channel fixture, click Review Impact, then switch the Combobox to a 32-channel fixture — the old proposed universe/address stays on screen and clicking Apply commits the *first* fixture's `plan_id` at the *first* footprint. `ProjectFixtures.tsx:293` does the right thing (`setPendingPreview(null)` in its own `handleSelectFixture`); FixturePatch is the outlier.

### ~~Remove-member impact preview races: an earlier member's plan renders and applies under a different member's row~~
**Fixed:** Generation-guarded via `useLatestRequest`, and the render gate now compares `pendingRemovePreview.poolName`/`.memberId` as well as `removeTarget`. `ProjectFixtures.tsx:381` got the same guard — its `pendingRemovePreview` is a bare `ImpactPlan` with no member identity, so the render-gate half wasn't available there and discarding the stale response is the whole fix.
**Severity:** bug
**File:** `frontend/src/components/FixturePatch/FixturePatch.tsx:388`
**Confidence:** high

`handleStartRemoveMember` fires `removePoolMemberPreview` with no generation guard, and the render gate at `:628` keys only on `removeTarget`, never on `pendingRemovePreview.memberId` — even though that state object already carries `poolName`/`memberId` for exactly this comparison. Repro: click Remove on member A, then click Remove on member B inside the round trip; when A's slower response lands, A's impact list renders under B's row and `handleApplyRemoveMember` (`:412`) commits A's `plan_id`, deleting member A instead of B. `ProjectFixtures.tsx:381` (gate at `:578`) has the same shape.

### Script launch failures are silently swallowed — the `.catch` handler is unreachable
**Severity:** bug
**File:** `frontend/src/workspaces/build/ScriptsWorkspace.tsx:465`
**Confidence:** high

`handleDialogSubmit` does `void launch.catch(...)` to synthesize a failed terminal state, but `runScript`/`debugScript` (`wailsBridge.ts:2378`/`:2392`) both catch internally and *resolve* with `offlineScriptRunOutcome()` — they never reject. The `.catch` body is dead code and the resolved `ScriptRunOutcomeView` (which carries the real `status: "failed"` / `reason`) is discarded entirely. Repro: delete the script from the CLI between opening the dialog and submitting, or run with the script host detached — the dialog closes, the debug panel sits at its idle placeholder forever, no error and no terminal banner. The block comment at `:437` describes behavior the code cannot produce.

### Art-Net targets on universes outside the active deployment become invisible and unreachable
**Severity:** bug
**File:** `frontend/src/components/ArtnetConfig/ArtnetConfig.tsx:334`
**Confidence:** high

`targetsByUniverse` (`:229`) is built from the full `status.targets`, but the list renders by iterating `patchedUniverses`, which is filtered to universes with instances in the **active** deployment (`:96`). Any target whose universe leaves that set is dropped from the DOM while the daemon keeps unicasting to it, and there is no orphan row. Repro: enable a target on universe 3, then activate a different deployment (or readdress the universe-3 fixtures) — universe 3 vanishes from Universe Targets and there is no in-app way to disable the still-live target. `ArtnetConfig.tsx` is the only consumer of `fetchArtnetStatus`/`enableArtnetTarget`/`disableArtnetTarget` in the frontend, so no other surface can recover from it.

### Safety hold-progress fill stays at 100% after a completed hold
**Severity:** bug
**File:** `frontend/src/components/SafetyCluster/SafetyCluster.tsx:104`
**Confidence:** high

The threshold timer sets `setProgress(1)` and latches `timers.current.completed = true` (`:129`); `cancel()` then early-returns on `completed`, and `completed` is only reset inside `start()`. Nothing calls `setProgress(0)` after a successful hold. Repro: hold Blackout to completion and release — the `.fill` wash (`transform: scaleX(1)`, `inset: 0`) stays covering the whole button until the *next* hold begins, so a safety control that already fired permanently reads as mid-press. The `completed` latch is right for suppressing duplicate `onComplete`; it just also needs to reset the visual progress.

### Cancelling a MIDI Learn produces a spurious "No MIDI input received" error
**Severity:** bug
**File:** `frontend/src/components/MidiPanel/MidiLearn.tsx:104`
**Confidence:** high

`CancelLearn` closes `session.cancel` on the Go side, which unblocks the still-pending `StartLearn` so it returns `GOLC_MIDI_LEARN_TIMEOUT` (documented at `svc_midi.go:741`). `handleCancel` sets `status = "idle"` but never invalidates the in-flight `StartLearn` promise; when it resolves a moment later, `handleLearn`'s `stderr.includes(TIMEOUT_MARKER)` branch fires and sets `status = "timeout"` plus the timeout copy. Repro: click Learn, then click Cancel — a "No MIDI input received. Try again." error appears under the button for an action the user deliberately aborted. Needs a per-attempt generation counter or `cancelled` ref checked before every `setStatus`. (`Desk.tsx:1029` already implements exactly this guard via `capturingKeyRef`; `MidiLearn.tsx` doesn't.)

### Renaming the selected scene makes the selection jump to a different scene
**Severity:** bug
**File:** `frontend/src/workspaces/build/ScenesLooksWorkspace.tsx:218`
**Confidence:** high

`handleRenameScene` calls `setSelectedSceneName(newName)` *before* `await refresh()`. That commits a render where `selectedSceneName` is the new name but `view.scenes` still holds the old one, so the selection-validity effect at `:107` sees no match and overwrites the selection with `view.scenes.find(s => s.active) ?? view.scenes[0]`. Repro: with three scenes, select the third, rename it — after the round trip the main column shows the first (or live) scene's layers instead of the renamed one. `handleCreateScene` (`:120`) does it in the correct order, so this handler is the outlier.

### A successful script run is reported to the user as "Terminated: <stderr line>"
**Severity:** bug
**File:** `frontend/src/components/Scripts/ScriptDebugPanel.tsx:141`
**Confidence:** high

`describeTermination` special-cases only `status === "failed"`; every other terminal status falls through the four `GOLC_SCRIPT_*` regexes to `Terminated: ${summary}`. But a **succeeded** run routinely carries a non-empty `Reason`: `internal/script/session.go:658` fills `outcome.Reason` from the captured stderr tail regardless of status. Repro: write a script that ends with `console.error("done")` and exits cleanly, click Run — the panel renders "Terminated: done" and the banner reads "Stopped: done" while the status chip simultaneously says "Succeeded".

### ~~Nav hotkeys navigate invisibly while the Guided First Show is open~~
**Fixed:** `GuardedCommandRail` now publishes its already-guarded select handler upward (`onGuardedNavigate`); `ShellBody` routes both the keyboard nav chords and the quick switcher through it instead of through the raw `setActiveDestination`. The hook stays above `GuidedFirstShowProvider` — only what it is handed as `onNavigate` changed. One guard, one `ConfirmDialog`, three entry points.
**Severity:** bug
**File:** `frontend/src/shell/useGlobalKeyboardWorkflow.ts:163`
**Confidence:** high

The nav-chord handler calls `onNavigate` (= `AppShell`'s raw `setActiveDestination`) directly, bypassing `GuidedFirstShowContext`'s leave-the-guide confirm. `ShellCanvas` (`AppShell.tsx:66`) renders `<GuidedFirstShow />` in place of `<WorkspaceRouter />` while `open`, so the destination changes underneath the still-open overlay with zero visible effect. Repro: launch the guide, press `Alt+ArrowDown` twice, press Exit Guide — you land on a workspace you never chose (and `exitGuide` returns you to the *entry* destination, so the two keystrokes are silently discarded instead). This is the exact bug `GuardedCommandRail` was added to fix for rail clicks; the file's own header comment acknowledges the keyboard path was left behind because the hook sits above `GuidedFirstShowProvider`.

### Monaco editor's `aria-label` goes stale on every script switch
**Severity:** bug
**File:** `frontend/src/components/Scripts/ScriptEditor.tsx:267`
**Confidence:** high

`ariaLabel` is passed into `monaco.editor.create()` inside the mount-only effect (`:212`) with no ref mirror and no sync effect — unlike `value`, `readOnly`, `sdkTypeDefinitions`, `breakpointLines`, and `currentExecutionLine`, which all have one. `ScriptsWorkspace.tsx:848` passes `` `${selectedScript.name} source` `` and the editor is never keyed or remounted on selection change (`:843`). Repro: select script A, then script B — the editor textarea is still announced as "A source" while displaying B's code, for the rest of the session.

### Pressing Learn in MidiPanel destroys keyboard focus
**Severity:** bug
**File:** `frontend/src/components/MidiPanel/MidiLearn.tsx:119`
**Confidence:** high

When `status` becomes `"listening"` the component returns an entirely different subtree: the focused `<Button>Learn</Button>` unmounts and is replaced by a `<div role="status">` plus a new Cancel button, with no `focus()` handoff. Repro: tab to a control's Learn button, press Enter — focus falls to `<body>`, so a keyboard user cannot reach Cancel without tabbing from the top of the document, and when the learn resolves and the Learn button remounts focus is not returned to it either.

### Breakpoint line numbers drift from the gutter glyphs after an edit
**Severity:** bug
**File:** `frontend/src/workspaces/build/ScriptsWorkspace.tsx:261`
**Confidence:** medium

`breakpointLines` is a plain `number[]` in workspace state, never adjusted for edits, but Monaco's decoration collection (`ScriptEditor.tsx:413`) tracks model edits and shifts the glyph with the text. Repro: set a breakpoint on line 5, put the cursor on line 1 and press Enter three times — the glyph is now at line 8 but state still holds `[5]`, so Debug breaks on the wrong statement. Toggling any other breakpoint re-runs the sync effect's `.set(breakpointDecorations([5, ...]))` and visibly snaps the glyph back to line 5. Either the collection's tracked ranges need to feed back into state, or breakpoints need clearing on edit.

### `onReorder` is invoked from inside a `setState` updater — fires twice under StrictMode
**Severity:** bug
**File:** `frontend/src/components/SceneProgramming/SceneList.tsx:238`
**Confidence:** high

`handleDragEnd` calls `onReorder(next)` inside the `setOrder` updater. `main.tsx:32` wraps the app in `React.StrictMode`, which deliberately double-invokes state updaters in development, so every drag-drop issues `ReorderScenes` twice against the Go host. The second call is computed against already-reordered server state if `refresh()` landed between them (`ScenesLooksWorkspace.tsx:247`) — a genuine ordering race, and at minimum a duplicate show-file mutation per drag. `next` is derivable from `order` outside the updater.

### The Launcher swallows every dispatch failure — a rejected scene launch looks like nothing happened
**Severity:** bug
**File:** `frontend/src/components/OperatorSurface/Launcher.tsx:43`
**Confidence:** high

`handleLaunch` and `handleToggleLayer` (`:51`) both `await dispatch.switchScene(...)` / `dispatch.setLayerEnabled(...)` and discard the returned `WailsResult` entirely, and the component renders no error surface of any kind. Repro: enter operate mode with the daemon unreachable (or trigger a server-side `AuthorizeControl` rejection) and tap a scene pad — `refreshState()` runs, the UI doesn't change, and the operator gets zero indication that the launch failed. This is the surface handed to a player mid-show, so silent failure is the worst place for it.

### ~~The playback keyboard workflow's window listener is torn down and re-added on every render~~
**Fixed:** `layerEnabled`/`sceneNames` are `useMemo`'d on `activeScene`/`state?.scenes`. Query's structural sharing keeps those references stable across an unchanged poll, so the listener is armed once instead of once per second. Regression test asserts the `keydown` `addEventListener` count is unchanged across re-renders (verified failing before the fix).
**Severity:** perf
**File:** `frontend/src/shell/useGlobalKeyboardWorkflow.ts:110`
**Confidence:** high

`layerEnabled` (built as a fresh object literal at `:110`) and `sceneNames` (`:114`, a fresh `.map()`) are new references on every render, and both sit in `useKeyboardWorkflow`'s effect dependency array (`useKeyboardWorkflow.ts:158`). Since `usePlaybackSnapshot` repolls every 1s and this hook lives at shell level, the `window.addEventListener("keydown", …, {capture:true})` / `removeEventListener` pair runs at least once per second for the whole session, rebuilding the `keyToAction` map each time. Memoizing `sceneNames`/`layerEnabled` (or passing the snapshot through a ref) removes the churn without changing behavior.

### `useResizablePanel` writes localStorage synchronously on every pointermove frame
**Severity:** perf
**File:** `frontend/src/hooks/useResizablePanel.ts:86`
**Confidence:** high

The persist effect is keyed on `[storageKey, size]`, and `size` is set from `handlePointerMove` on every pointer event during a drag — so a single one-second rail drag performs hundreds of synchronous `localStorage.setItem` calls. Desk makes this worse: every `UniverseRow` instantiates two of these hooks (`Desk.tsx:600` height, `:610` width), so dragging one row's width handle writes per frame while every other row's effect is also live. Persisting on `pointerup` (or debouncing) would be equivalent for the user-visible contract.

### A resize drag never ends if the pointer is released outside the window, and any mouse button starts one
**Severity:** edge-case
**File:** `frontend/src/hooks/useResizablePanel.ts:47`
**Confidence:** medium

`handlePointerDown` never calls `setPointerCapture` and never checks `event.button`, and the drag is terminated only by a `pointerup`/`pointercancel` that reaches `window`. Repro A: start dragging the rail handle, drag past the window edge, release outside the webview — no `pointerup` arrives, `isResizing` stays true, and the panel resumes resizing the moment the cursor re-enters. Repro B: right-click or middle-click the handle and move — the panel resizes from a non-primary button press (and `preventDefault()` at `:49` suppresses the context menu that would otherwise explain it).

### ~~Guided-First-Show stage status leaks across a stage switch~~
**Fixed:** All five `stages/*.tsx` now hold a `useLatestRequest` guard across their read and check it before every `onStatusChange`/`setError`/`setLoading`. Switching stages unmounts the previous one, so the guard's mounted-check is what does the work here. Regression test verified failing before the fix.
**Severity:** bug
**File:** `frontend/src/workspaces/show/GuidedFirstShow/stages/VerifyStage.tsx:81`
**Confidence:** medium

Every stage reports upward through `onStatusChange` (= `GuidedFirstShow.tsx:89`'s `setStatus`) from an async `refresh()` with no unmount/generation guard. Repro: open the Verify stage (which fires four parallel reads — the slowest stage by construction), then immediately click "Fixtures" in the stage rail. `GuidedFirstShow`'s reset effect (`:94`) sets the placeholder, FixturesStage reports its own status, and then Verify's late response lands and overwrites it — the footer primary button now reads "Perform" (and inherits Verify's blocker-derived `primaryDisabled`) while the Fixtures stage body is on screen. Same shape in every `stages/*.tsx`.

### ~~A playback hotkey can be bound to `?` or `Escape`, which then double-fires with the help overlay~~
**Fixed:** `findHotkeyConflict` now returns a `"shell-reserved"` verdict for the two keys `useGlobalKeyboardWorkflow` hard-codes, and Settings renders it as its own inline message.
**Severity:** edge-case
**File:** `frontend/src/lib/hotkeys.ts:169`
**Confidence:** high

`findHotkeyConflict` rejects only digits `1-9` and other playback bindings. It knows nothing about the two hard-coded keys `useGlobalKeyboardWorkflow.ts:124` owns (`?` toggles the help overlay, `Escape` closes it). Repro: Settings > Hotkeys, bind "Evaluate/preview the active scene" to `?` — it saves with no conflict warning, and from then on pressing `?` both opens the help overlay and evaluates the scene. Binding to `Escape` is blocked by accident (the capture handler treats it as cancel), but `?` is not.

### ~~No `event.repeat` guard anywhere in the keyboard workflow — holding Space machine-guns tap tempo~~
**Fixed:** Both `onKeyDown` handlers (playback and nav/help) return early on `event.repeat`. The BPM nudges lose nothing: they compute from a once-a-second snapshot, so a held key re-sent the same target value rather than ramping.
**Severity:** edge-case
**File:** `frontend/src/hooks/useKeyboardWorkflow.ts:92`
**Confidence:** high

Neither `onKeyDown` handler checks `event.repeat`. Repro A: hold Space — OS key auto-repeat (~30/s) floods `dispatch.recordTap()`, and since `createTapTempoRecorder` (`playbackDispatch.ts:87`) only resets after a >2s gap, the 8-tap buffer fills with repeat-interval timestamps and `TapTempo` computes a nonsense BPM from the keyboard repeat rate. Repro B: hold `?` or `Ctrl+K` — `setHelpOpen(c => !c)` / `setQuickSwitcherOpen(c => !c)` (`useGlobalKeyboardWorkflow.ts:126`/`:155`) toggle on every repeat, flickering the overlay open/closed.

### Layer toggles compute the target state from a snapshot that can be up to a second stale
**Severity:** edge-case
**File:** `frontend/src/hooks/useKeyboardWorkflow.ts:127`
**Confidence:** medium

The handler sends `!layerEnabled[layerKind]`, where `layerEnabled` comes from `usePlaybackStateSnapshot`'s 1s poll (`usePlaybackStateSnapshot.ts:19`) and there is no optimistic local update. Repro: press `Q` twice inside one poll interval — both keypresses read the same cached `enabled: true` and both dispatch `SetLayerEnabled(scene, base_look, false)`, so the layer ends up off when the operator expects it back on. The same applies to `Launcher.tsx:109`'s on-screen layer buttons, which do refresh after each call but still race a rapid second click.

### Discarding all recovery points is irreversible with no confirmation step
**Severity:** edge-case
**File:** `frontend/src/workspaces/show/SaveRecoveryWorkspace.tsx:111`
**Confidence:** medium

`handleDiscardAll` calls `discardRecoveryPoints()` straight from the button's `onClick` — no `ConfirmDialog`, no `window.confirm`, despite this permanently deleting every crash-recovery snapshot for the open show and despite `ConfirmDialog` being the design system's public confirmation contract (used by `AppShell.tsx:97` and mirrored by `window.confirm` in `SurfaceList.tsx:56` / `SceneList.tsx:276`). The button sits directly next to per-row "Accept", so a misclick on a list the operator is scanning destroys the only copy of unsaved work. Medium confidence only because I did not check whether the Go route has its own guard.

### ~~Cancelling and re-establishing the operate-mode active surface race each other~~
**Fixed:** The `SetActiveSurface` calls are serialized through one promise chain (`activeSurfaceChainRef`), so a clear issued before a set can never resolve after it. Rejections are swallowed deliberately — the chain exists only to order the calls.
**Severity:** edge-case
**File:** `frontend/src/components/OperatorSurface/OperatorSurface.tsx:173`
**Confidence:** medium

Switching the selected surface while in operate mode runs the effect cleanup (`setSafetyActiveSurface("")`, `setPlaybackActiveSurface("")`) and the new effect body (`setSafetyActiveSurface(B)`) back to back, all as unawaited `void` calls. Nothing sequences them, so if the clear resolves on the Go side after the set, dispatch is left unrestricted while the UI shows surface B in locked operate mode — the exact D-04 enforcement the CR-01 fix exists to guarantee. Repro requires overlapping bridge latency, so it's rare, but the two calls are genuinely unordered.

### ~~MidiPanel stays pinned to a surface deleted elsewhere in the app~~
**Fixed:** A reconciliation effect deselects `selectedSurface` when it is absent from a freshly-fetched `surfaces` list, so the panel collapses to its empty state instead of sitting on a `GOLC_OPERATORSURFACE_NOT_FOUND` banner under a dangling Select value.
**Severity:** edge-case
**File:** `frontend/src/components/MidiPanel/MidiPanel.tsx:132`
**Confidence:** medium

`surfaceListVersion` correctly re-runs `refreshSurfaces`, but nothing reconciles `selectedSurface` against the returned list. Repro: select surface X in MidiPanel, delete X from the Operator Surfaces view (both are permanently mounted side by side). `surfaces` loses X but `selectedSurface` still holds `"X"`, so the Select renders a value with no matching option, `refreshSurfaceDetail("X")` fails with `GOLC_OPERATORSURFACE_NOT_FOUND`, and the assigned-controls / mappings sections stay mounted showing an error banner instead of collapsing.

### Picking a look for a disabled layer silently re-enables it
**Severity:** edge-case
**File:** `frontend/src/workspaces/build/ScenesLooksWorkspace.tsx:150`
**Confidence:** medium

`handleSelectLayerLook` hard-codes `enabled: true` in `setSceneLayer(scene.name, kind, refId, true)`, and nothing prevents the picker being used while the layer is off (`LayerRow.tsx`'s header comment states the select stays live regardless of the toggle). Repro: disable the Chase layer on the *live* scene, then use its still-enabled dropdown to pre-stage a different chase — the layer immediately goes enabled and starts contributing to output. On a lighting console that is a visible-on-stage side effect from what reads as a staging action.

### A rejected scene reorder is never rolled back
**Severity:** edge-case
**File:** `frontend/src/components/SceneProgramming/SceneList.tsx:238`
**Confidence:** medium

`SceneList` commits the new order to local state optimistically, and its reset effect (`:219`) deliberately only resets when the *name set* changes — which a failed reorder never does. `handleReorderScenes` (`ScenesLooksWorkspace.tsx:246`) either bails silently on an invalid permutation or sets an error banner; in both cases the local order stays wrong permanently. Repro: drag a scene while the `scene reorder` route rejects — the list keeps the new order for the rest of the session and the operator only discovers the truth on the next launch. There is no `onReorder` failure signal back into `SceneList` at all.

### Run/Debug executes the last-saved source, not what the editor shows
**Severity:** edge-case
**File:** `frontend/src/workspaces/build/ScriptsWorkspace.tsx:442`
**Confidence:** high

`handleDialogSubmit` calls `setScriptProfile` → `refresh` → `runScript`/`debugScript` and never calls `saveScriptSource`. There is no dirty-state tracking in the file and no unsaved-changes indicator on the Save button. Repro: edit a script, don't press Save, press Run — the backend runs the on-disk version while the editor shows different code, and the log/stack-trace line numbers refer to the on-disk file. Especially confusing combined with breakpoints, which *are* taken from the live editor gutter.

### "Run Again" after a debug run silently downgrades to a plain Run
**Severity:** edge-case
**File:** `frontend/src/workspaces/build/ScriptsWorkspace.tsx:540`
**Confidence:** high

`const handleRunAgain = () => setDialogMode("run")` — hard-coded, and the workspace keeps no memory of which mode the finished run used. Repro: Debug a script, hit a breakpoint, let it crash, click "Run Again" in the terminal banner. The dialog opens titled "Run <name>", and submitting calls `runScript()`, which ignores `breakpointLines` entirely (`session.go`'s `Run` reads breakpoints only when `mode == LaunchModeDebug`). The gutter still shows the breakpoints, which will never fire, with no indication why.

### ~~Overlapping Evaluate requests can display a stale bar preview~~
**Fixed:** Generation-guarded via `useLatestRequest`; the Evaluate button is disabled while a call is outstanding; and a thrown dispatch now clears `previewOutput` and renders an `ErrorState` instead of leaving the previous evaluation on screen as if current.
**Severity:** edge-case
**File:** `frontend/src/components/SceneProgramming/BarTimelinePanel.tsx:23`
**Confidence:** medium

`handleEvaluate` awaits `dispatch.evaluate(parsed)` and unconditionally writes the result into `previewOutput` — no in-flight guard, no request id, and the Evaluate button is never disabled while a call is outstanding. Repro: type `4`, click Evaluate, immediately change to `8` and click again; if the first dispatch resolves last, the `<pre>` shows bar 4's evaluation while the field reads 8, with nothing signalling the mismatch. The panel also never clears `previewOutput` on failure, so an empty result leaves the previous evaluation on screen as if current.

### Blackout and Stop/Release-All derive their toggle argument from a signal that can't distinguish them
**Severity:** edge-case
**File:** `frontend/src/components/SafetyCluster/SafetyCluster.tsx:278`
**Confidence:** medium

The file's header comment scopes the `outputState === "blackout"` ambiguity to the *indicator*, but `blackoutOrStopActive` also feeds the boolean sent to the daemon (`:289`, `:305`). Repro: engage Blackout only — both buttons now read "Release…" with `aria-pressed={true}`. Hold "Release Stop / Release All" and it sends `safetyStopReleaseAll(false)`, releasing something that was never engaged; `outputState` stays `"blackout"` and the button still says "Release". The reverse case sends `safetyBlackout(false)` for the same reason. Recoverable via the other button, so confusing rather than fatal.

### Impact plans are `JSON.parse`d and cast with no schema validation at the Wails boundary
**Severity:** smell
**File:** `frontend/src/components/FixturePatch/FixturePatch.tsx:257`
**Confidence:** medium

`JSON.parse(result.stdout) as ImpactPlan` (also `:395`, and `ProjectFixtures.tsx:354`/`:388`) is the remaining unvalidated cast on the patch path — push-event payloads moved behind zod (`wailsBridge.ts:1257`), but this stdout-carried plan did not. A plan missing `plan_id` sails through the cast into `applyPatch(undefined as unknown as string)`, and absent `proposed_universe`/`proposed_address` render as the literal string "Universe undefined, Address undefined" (`:126`, `:180`) instead of failing loudly. `dispatch.getState()` (`lib/playbackDispatch.ts:135`) has the same shape — a bare `JSON.parse(result.stdout) as PlaybackStateSummary` feeding the whole transport readout and the keyboard workflow.

### One global `actionLoading` flag freezes every per-universe Art-Net row
**Severity:** smell
**File:** `frontend/src/components/ArtnetConfig/ArtnetConfig.tsx:74`
**Confidence:** high

`drafts` is deliberately keyed per universe (`:77`: "so every row … can be filled in and submitted on its own"), but the busy flag is a single boolean used at `:359` and `:409`. Repro: with three patched universes, click Add Target on universe 1 — all three rows show "Configuring…" and every existing target's Enable/Disable button goes disabled. `selectingIndex` a few lines above is the correct per-row pattern.

### Launcher treats a missing layer control as unlocked rather than unassigned
**Severity:** smell
**File:** `frontend/src/components/OperatorSurface/Launcher.tsx:101`
**Confidence:** low

`const locked = control ? !control.assigned : false` — when no layer control entry exists for a kind, the button renders enabled and dispatches, the opposite of the scene-pad path (`:82`, `locked={!control.assigned}`) and of the component's own "unassigned … locked (never dispatch)" contract. I did not confirm whether `ShowSurface` can ever omit a layer control (author mode renders every control assigned-or-not, which suggests the list is exhaustive and this branch is dead), so this may be unreachable defensive code with an inverted default rather than a live bug — but if it is reachable, combined with the swallowed-failure finding above it produces a button that appears live and does nothing.

### `seedAppLog` sorts synthesized "gap" markers to the very top of the log
**Severity:** edge-case
**File:** `frontend/src/store/store.ts:127`
**Confidence:** low

`seedAppLog` merges the `RecentAppLogs` backlog and re-sorts by `seq`, but a live-synthesized "gap" entry carries `seq: 0` (unset — the code at `:119` says so explicitly), so it sorts ahead of every real line. Repro requires an overflow gap to arrive in the narrow window between `onAppLog` subscribing and `fetchRecentAppLogs` resolving (`AppLogStream.tsx:40`), after which the Diagnostics log shows "entries dropped" as its first row rather than at the point in the stream where the drop happened. Narrow, cosmetic, and I did not reproduce it.

### `window.confirm` is used for six destructive actions with no exceptions.json record
**Severity:** smell
**File:** `frontend/src/components/OperatorSurface/SurfaceList.tsx:56`
**Confidence:** medium

`window.confirm` is the destructive-action gate in `SurfaceList.tsx:56`, `SceneList.tsx:276`, `LookBrowser.tsx:159`/`:182`, `MidiPanel.tsx:209`, `DeskMappingsSection.tsx:86`, and `FixturePatch.tsx:336`/`:373`, while `ConfirmDialog` is the design system's stated confirmation contract and is used by `AppShell.tsx:97`. I grepped all 65 records in `frontend/design-system/exceptions.json` and none mention `confirm` or any of these files, so this looks like drift rather than a reviewed decision. It is also a real behavior difference in a Wails webview: `window.confirm` blocks the JS thread and renders unstyled native chrome that the app's own focus/return-focus contract doesn't cover.

### Loading/fetching patterns are split three ways across the app
**Severity:** smell
**File:** `frontend/src/workspaces/show/SaveRecoveryWorkspace.tsx:50`
**Confidence:** high

Three coexisting patterns: TanStack Query (`FixtureLibraryWorkspace`, `usePlaybackStateSnapshot`, `Desk.tsx:986`), hand-rolled `useCallback` + `useEffect` + `loading`/`error` `useState` (most workspaces — `SaveRecoveryWorkspace`, `ShowsWorkspace`, `DiagnosticsWorkspace`, `OperatorSurface`, `NotesWorkspace`, the guide stages), and a couple of ad-hoc `void fn().then(setState)` calls. Noting once, not per file: the practical cost is that the unguarded hand-rolled ones are where every "slow response overwrites newer state" finding in this pass lives (`OperatorSurface.tsx:140`, `MidiPanel.tsx:149`, `FixturePatch.tsx:388`) — Query's `queryKey` would have given those a generation guard for free.
