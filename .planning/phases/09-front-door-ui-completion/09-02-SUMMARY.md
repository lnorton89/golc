---
phase: 09-front-door-ui-completion
plan: 02
subsystem: ui
tags: [go, react, wails, show, typescript, vitest, self-relaunch]

# Dependency graph
requires:
  - phase: 06-wails-shell
    provides: the Wails service/App pattern (Bind list, injectable dial/spawn test-double discipline, Result shape) this plan's PickShowPath/PickNewShowPath/RelaunchWithShow follow
  - phase: 05-durable-shows
    provides: internal/show.Save/Load and the "show save" CLI route RelaunchWithShow's save step forwards to, unmodified
  - phase: 09-01
    provides: the FixtureLibraryWorkspace-established Panel/PanelHeader/Button workspace shape and wailsBridge.ts bridge-accessor/offline-fallback/try-catch trio convention ShowsWorkspace.tsx and its bridge exports mirror
provides:
  - internal/wails.DesktopShowPathEnvName, the single GOLC_DESKTOP_SHOW literal shared by cmd/golc-desktop/main.go's startup read and RelaunchWithShow's spawn
  - internal/wails.App.PickShowPath/PickNewShowPath (native *.golc OS file pickers) and App.RelaunchWithShow (supervised self-relaunch: save -> resolve exe -> free daemon pipe -> spawn replacement -> quit only on spawn success)
  - internal/wails.App.stopSupervisedDaemon, extracted from OnShutdown and reused by RelaunchWithShow
  - frontend/src/lib/wailsBridge.ts AppBinding + pickShowPath/pickNewShowPath/relaunchWithShow bridge exports
  - a real Shows destination/workspace (frontend/src/workspaces/show/ShowsWorkspace.tsx) in the Show nav group -- open, create, and switch shows entirely on screen
affects: [09-03, 09-04, 09-05, 09-06, 09-07]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Supervised self-relaunch: exec.Command(os.Executable()) with a modified os.Environ() slice, never exec.CommandContext (the stored ctx is cancelled by this process's own exit and would kill the replacement)"
    - "Compare-and-set mutex guard (App.relaunching) making a mutating Wails-bound method non-reentrant, with every failure path clearing the guard and the success path deliberately leaving it set"
    - "A mutating Wails method reuses the exact same command.NewDefaultCommandRegistry()/command.Request execute path an existing Service method (ShowService.Save) already uses, rather than a second save implementation"

key-files:
  created:
    - frontend/src/workspaces/show/ShowsWorkspace.tsx
    - frontend/src/workspaces/show/ShowsWorkspace.test.tsx
    - frontend/src/workspaces/show/ShowsWorkspace.module.css
  modified:
    - internal/wails/app.go
    - internal/wails/app_test.go
    - cmd/golc-desktop/main.go
    - frontend/src/lib/wailsBridge.ts
    - frontend/src/shell/navigation.ts
    - frontend/src/shell/destinationIcons.tsx
    - frontend/src/shell/WorkspaceRouter.tsx
    - frontend/src/workspaces/show/SaveRecoveryWorkspace.tsx

key-decisions:
  - "RelaunchWithShow auto-saves the working show before relaunching rather than prompting (resolves 09-RESEARCH.md Open Question 2) -- a save that fails aborts the switch with GOLC_WAILS_RELAUNCH_SAVE_FAILED, never proceeding to a process exit that would drop work"
  - "PickShowPath/PickNewShowPath/RelaunchWithShow live on App (already owns process-lifecycle concerns: daemon spawn/kill, hotkeys, event pusher), not ShowService, which owns ShowState CRUD -- a different concern (resolves 09-RESEARCH.md Open Question 3)"
  - "'New Show...' is the identical RelaunchWithShow mechanism pointed at a path that does not exist yet (D-06) -- no separate new-show setup flow, no second backend concept; step 1's empty/whitespace-only rejection is the only special case, verified by a dedicated test that a not-yet-existing path and the current show's own path both take the unmodified relaunch path"

patterns-established:
  - "stopSupervisedDaemon() runs before a replacement process is spawned so the named pipe is free -- otherwise the replacement's own ensureDaemon dials successfully and silently attaches to a daemon still bound to the OLD show path (T-09-02-04)"

requirements-completed: [FDUI-02]

coverage:
  - id: D1
    description: "The Show nav group has a fourth destination, 'Shows', whose workspace shows the current show path and an 'Open Show...'/'New Show...' action pair"
    requirement: "FDUI-02"
    verification:
      - kind: unit
        ref: "frontend/src/shell/AppShell.navigation.test.tsx#mounts Shows without throwing or logging a console error"
        status: pass
      - kind: unit
        ref: "frontend/src/workspaces/show/ShowsWorkspace.test.tsx#renders the current show path with a full-value tooltip"
        status: pass
    human_judgment: false
  - id: D2
    description: "'Open Show...' opens a native OS file picker filtered to *.golc, and choosing an existing file relaunches the application against that show without the operator touching GOLC_DESKTOP_SHOW or restarting by hand"
    requirement: "FDUI-02"
    verification:
      - kind: unit
        ref: "internal/wails/app_test.go#TestRelaunchWithShowPassesShowPathThroughEnvironmentVerbatim"
        status: pass
      - kind: unit
        ref: "internal/wails/app_test.go#TestRelaunchWithShowQuitsOnlyAfterSuccessfulSpawn"
        status: pass
    human_judgment: true
    rationale: "The plan's own <verify><human-check> requires launching the real golc-desktop app, picking a different .golc file via the native OS dialog, and confirming the app actually relaunches into that show -- not exercisable from an automated test in this environment"
  - id: D3
    description: "'New Show...' uses the identical relaunch mechanism pointed at a path that does not exist yet -- no separate new-show setup flow, no second backend concept"
    requirement: "FDUI-02"
    verification:
      - kind: unit
        ref: "internal/wails/app_test.go#TestRelaunchWithShowAcceptsNonExistentAndCurrentPathsWithNoSpecialCase/path_whose_file_does_not_exist_yet"
        status: pass
    human_judgment: false
  - id: D4
    description: "Cancelling either picker leaves the application exactly as it was: no save, no spawn, no quit, no error"
    requirement: "FDUI-02"
    verification:
      - kind: unit
        ref: "frontend/src/workspaces/show/ShowsWorkspace.test.tsx#cancelling the picker is a no-op"
        status: pass
    human_judgment: false
  - id: D5
    description: "While a switch is in flight the workspace renders the 'Switching shows -- GOLC will reload in a moment...' transient and both action buttons are disabled"
    requirement: "FDUI-02"
    verification:
      - kind: unit
        ref: "frontend/src/workspaces/show/ShowsWorkspace.test.tsx#renders the switching transient and disables both actions"
        status: pass
    human_judgment: false
  - id: D6
    description: "When the replacement process fails to start, the workspace renders the relaunch-failure copy and the current process is still running its original show"
    requirement: "FDUI-02"
    verification:
      - kind: unit
        ref: "frontend/src/workspaces/show/ShowsWorkspace.test.tsx#renders the relaunch failure copy"
        status: pass
      - kind: unit
        ref: "internal/wails/app_test.go#TestRelaunchWithShowQuitsOnlyAfterSuccessfulSpawn/spawn_fails"
        status: pass
    human_judgment: false
  - id: D7
    description: "With no show path resolved the workspace renders the empty-state copy; a long show path truncates in place with the full value on hover"
    requirement: "FDUI-02"
    verification:
      - kind: unit
        ref: "frontend/src/workspaces/show/ShowsWorkspace.test.tsx#renders the empty state when no show path is resolved"
        status: pass
      - kind: unit
        ref: "frontend/src/workspaces/show/ShowsWorkspace.test.tsx#renders the current show path with a full-value tooltip"
        status: pass
    human_judgment: false
  - id: D8
    description: "BOUNDARY/PRECISION/IDEMPOTENCY/CONCURRENCY edge-probe classes: empty/whitespace path refused; a non-existent-file path and the current show's own path both take the unmodified relaunch path; the picked path reaches the replacement byte-for-byte (space + non-ASCII) via GOLC_DESKTOP_SHOW; a concurrent relaunch call is refused while one is in flight; the working show is saved before any spawn, and Quit runs only after a successful spawn"
    requirement: "FDUI-02"
    verification:
      - kind: unit
        ref: "internal/wails/app_test.go#TestRelaunchWithShowRejectsEmptyPath"
        status: pass
      - kind: unit
        ref: "internal/wails/app_test.go#TestRelaunchWithShowAcceptsNonExistentAndCurrentPathsWithNoSpecialCase"
        status: pass
      - kind: unit
        ref: "internal/wails/app_test.go#TestRelaunchWithShowPassesShowPathThroughEnvironmentVerbatim"
        status: pass
      - kind: unit
        ref: "internal/wails/app_test.go#TestRelaunchWithShowIsNotReentrant"
        status: pass
      - kind: unit
        ref: "internal/wails/app_test.go#TestRelaunchWithShowAbortsWhenSaveFails"
        status: pass
    human_judgment: false
  - id: D9
    description: "SaveRecoveryWorkspace.tsx no longer documents a single-show-path-at-startup limitation and points at the Shows workspace instead"
    requirement: "FDUI-02"
    verification:
      - kind: other
        ref: "grep -c 'flow to wire yet' frontend/src/workspaces/show/SaveRecoveryWorkspace.tsx == 0; grep -c 'ShowsWorkspace' frontend/src/workspaces/show/SaveRecoveryWorkspace.tsx >= 1"
        status: pass
    human_judgment: false

# Metrics
duration: ~35min
completed: 2026-07-27
status: complete
---

# Phase 9 Plan 2: Show Open/New/Switch (Supervised Self-Relaunch) Summary

**On-screen show open/new/switch via a supervised self-relaunch of golc-desktop.exe with a new GOLC_DESKTOP_SHOW, surfaced through a new "Shows" destination in the Show nav group.**

## Performance

- **Duration:** ~35 min
- **Tasks:** 3 (RED test authoring, Go relaunch mechanics GREEN, frontend Shows workspace GREEN)
- **Files modified:** 12 (3 created, 9 modified/appended)

## Accomplishments

- Exported `internal/wails.DesktopShowPathEnvName` (`GOLC_DESKTOP_SHOW`) as the single literal both `cmd/golc-desktop/main.go`'s startup read and `RelaunchWithShow`'s spawn use
- Added `App.PickShowPath`/`App.PickNewShowPath` (native OS file/save dialogs filtered to `*.golc`, guarded by `GOLC_WAILS_RUNTIME_CONTEXT_UNAVAILABLE` when `OnStartup` has never run) and `App.RelaunchWithShow` (save the working show through the existing `"show save"` route -> resolve the running executable -> free the supervised daemon's named pipe -> spawn a replacement `golc-desktop` process bound to the new show path -> quit only once the replacement has actually started)
- Extracted `App.stopSupervisedDaemon` from `OnShutdown`, now reused by both the normal shutdown path and `RelaunchWithShow`'s pre-spawn daemon-pipe-release step
- Added a compare-and-set `relaunching` guard making `RelaunchWithShow` non-reentrant (`GOLC_WAILS_RELAUNCH_IN_PROGRESS`), with every failure path clearing the guard and the success path deliberately leaving it set
- Extended `frontend/src/lib/wailsBridge.ts` with `AppBinding` and `pickShowPath`/`pickNewShowPath`/`relaunchWithShow` bridge exports, mirroring the existing bridge-accessor/offline-fallback/try-catch trio
- Added a real `Shows` destination (`show-shows`) to the Show nav group and a new `ShowsWorkspace.tsx`: current-show display (truncated in place with the full path on hover, matching `OverviewWorkspace`), `Open Show…`/`New Show…` actions, a client-side "switching" transient, and relaunch-failure copy
- Rewrote `SaveRecoveryWorkspace.tsx`'s doc comment to point at `ShowsWorkspace.tsx` instead of documenting the now-removed single-show-path-at-startup limitation

## Task Commits

1. **Task 1: Write the failing relaunch-mechanics and Shows-workspace tests** - `3f73d6c6` (test)
2. **Task 2: Implement the show picker and supervised self-relaunch on App** - `b8712bc3` (feat)
3. **Task 3: Add the Shows destination and workspace, and retire the single-show-path note** - `903908aa` (feat)

**Deviation (added test coverage):** `786a02fa` (test) — see Deviations below.

## Files Created/Modified

- `internal/wails/app.go` - `DesktopShowPathEnvName`, `App.ctx`/`relaunchSpawn`/`quit`/`relaunching` fields, `stopSupervisedDaemon`, `PickShowPath`/`PickNewShowPath`/`RelaunchWithShow`, `defaultRelaunchSpawn`/`defaultQuit`
- `internal/wails/app_test.go` - RelaunchWithShow/PickShowPath/PickNewShowPath contract tests (10 new top-level tests, several with subtests)
- `cmd/golc-desktop/main.go` - `showPathEnvName` now references `golcwails.DesktopShowPathEnvName` instead of a second hardcoded literal
- `frontend/src/lib/wailsBridge.ts` - `AppBinding` type + `pickShowPath`/`pickNewShowPath`/`relaunchWithShow` bridge functions
- `frontend/src/shell/navigation.ts` - `"show-shows"` `DestinationId` member + nav entry
- `frontend/src/shell/destinationIcons.tsx` - `"show-shows": FolderOpen`
- `frontend/src/shell/WorkspaceRouter.tsx` - `case "show-shows"` switch arm
- `frontend/src/workspaces/show/ShowsWorkspace.tsx` - new Shows workspace (current-show display, Open/New actions, switching transient, failure copy)
- `frontend/src/workspaces/show/ShowsWorkspace.module.css` - workspace styles (spacing scale per UI-SPEC)
- `frontend/src/workspaces/show/ShowsWorkspace.test.tsx` - full test coverage for the workspace's five behaviors
- `frontend/src/workspaces/show/SaveRecoveryWorkspace.tsx` - doc-comment rewrite pointing at `ShowsWorkspace.tsx`
- `.planning/phases/09-front-door-ui-completion/deferred-items.md` - re-confirmed pre-existing, unrelated toolchain-bootstrap test failures

## Decisions Made

- Auto-save-before-relaunch (no confirmation prompt) — see `key-decisions` above; the lower-risk default that never silently loses work and needs no new Copywriting Contract copy.
- `PickShowPath`/`PickNewShowPath`/`RelaunchWithShow` live on `App`, not `ShowService` — see `key-decisions` above.
- `RelaunchWithShow`'s default `relaunchSpawn` implementation uses `exec.Command`, deliberately not `exec.CommandContext` — the stored `ctx` is cancelled by this process's own exit (via `Quit`) and would kill the replacement process before it could finish starting.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Installed frontend dependencies in this worktree**
- **Found during:** Task 1 (confirming RED for `ShowsWorkspace.test.tsx`)
- **Issue:** This git worktree had no `frontend/node_modules` (untracked/gitignored, not shared across worktrees — same finding as 09-01's own worktree)
- **Fix:** Ran `npm ci` in `frontend/` against the existing `package-lock.json`; no dependency versions changed
- **Files modified:** none (`node_modules` is gitignored)
- **Verification:** `npx vitest run` executed successfully afterward

**2. [Rule 2 - Missing Critical Test Coverage] Added the remaining two BOUNDARY edge-probe test clauses**
- **Found during:** Post-Task-3 review of the plan's own locked `must_haves` BOUNDARY edge probe, which names three clauses: (a) empty/whitespace-only path refused, (b) a not-yet-existing path accepted, (c) a path equal to the currently-open show accepted with no special-case short circuit. Task 1's authored tests only covered (a) directly; (b) was only implicitly exercised as a side effect of the verbatim-environment test, and (c) had no test at all.
- **Fix:** Added `TestRelaunchWithShowAcceptsNonExistentAndCurrentPathsWithNoSpecialCase` with two subtests, explicitly asserting both remaining clauses (spawn+quit called exactly once in each case, no special-case short circuit).
- **Files modified:** `internal/wails/app_test.go`
- **Verification:** `go test ./internal/wails/... -run TestRelaunchWithShow -v` — both new subtests pass
- **Commit:** `786a02fa`

### Plan-Authoring Notes (not defects)

**3. `go test ./...` pre-existing unrelated failures (re-confirmed, matches 09-01's identical finding)**
- The same five `internal/command` toolchain-bootstrap tests (`TestBuildRouteCompilesTheProductionRepository`, `TestBuildablePackagesExcludesMagefiles`, `TestScopeCrossPlatformCI`, `TestScopeGreenSubprocess`, `TestScopeOfflineAcceptance`) fail with `GOLC_TEST_TOOLCHAIN_MISSING` because this fresh worktree has never run `mage Bootstrap`. None touch `internal/wails`, `cmd/golc-desktop`, or any other file this plan modified. Logged to `deferred-items.md`; not fixed (out of scope, pre-existing, environmental). Unlike 09-01's worktree, `internal/trace/catalog`'s `TestScopeLinearMap` test passed cleanly here (no `.planning/linear-map.json` drift in this worktree).

---

**Total deviations:** 2 auto-fixed (1 blocking dependency install, 1 missing test coverage), 1 documented environment mismatch.
**Impact on plan:** No scope creep. The added test coverage closes a gap against the plan's own locked acceptance criteria; the environment finding is identical to 09-01's own pre-existing gap.

## Issues Encountered

- `go build ./cmd/golc-desktop/...` fails until `frontend/dist` exists (the package embeds it via `//go:embed all:frontend/dist`), the same pre-existing build-order note 09-01 documented. Resolved by running `npm run build` (which itself runs `tsc --noEmit && vitest run && vite build`, 234/234 tests passing) before the final `go build ./...`/`go vet ./internal/wails/...` verification.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- FDUI-02 is delivered: an operator can open, create, and switch shows entirely from the Show nav group, with every relaunch failure mode leaving exactly one running instance and the working show saved before any process exit.
- `GOLC_DESKTOP_SHOW` exists as a literal in exactly one Go package (`internal/wails`), read from `cmd/golc-desktop/main.go` via `golcwails.DesktopShowPathEnvName`.
- Human verification still needed: launching `golc-desktop`, using "Open Show…"/"New Show…" against real `.golc` files, and confirming the actual relaunch (D2 above) — flagged for end-of-phase UAT per this plan's own `<verify><human-check>` step.
- `App.relaunchSpawn`/`App.quit` are now established injectable-field precedent (alongside the existing `dial`/`spawn`) for any future App-level process-lifecycle behavior a later plan might add.

---
*Phase: 09-front-door-ui-completion*
*Completed: 2026-07-27*

## Self-Check: PASSED

All claimed files verified present on disk (3 created, 9 modified); all 4
referenced commit hashes (`3f73d6c6`, `b8712bc3`, `903908aa`, `786a02fa`)
verified present in `git log`.
