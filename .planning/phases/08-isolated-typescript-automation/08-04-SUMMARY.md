---
phase: 08-isolated-typescript-automation
plan: 04
subsystem: ui
tags: [wails, react, typescript, scripts, capability-profile, css-modules]

requires:
  - phase: 08-isolated-typescript-automation
    provides: "08-01's internal/command/script.go CLI routes (script create/list/show/edit/delete/profile set) and internal/show/scripts.go's Script/CapabilityProfile domain model"
provides:
  - "ScriptService Wails binding (internal/wails/svc_script.go) forwarding every mutation to the existing script * CLI routes"
  - "Build -> Scripts workspace: D-16 library view, create/edit/save/delete round trip, capability-profile inspector summary"
  - "frontend/src/lib/wailsBridge.ts ScriptService client (types + async wrappers)"
affects: ["08-10 (Run/Debug dialog extends this Toolbar action slot)", "08-11 (Monaco editor replaces this plan's textarea in place)"]

tech-stack:
  added: []
  patterns:
    - "ScriptService mirrors ProgrammingService/ShowService's execute()+command.NewDefaultCommandRegistry() pattern exactly"
    - "SaveScriptSource writes source to an os.CreateTemp file and passes --source-file, never argv-interpolating multi-line TypeScript (T-08-12)"
    - "ListScripts/GetScript decode CLI Stdout JSON via internal/strictjson.DecodeStrict into unexported wire types, then flatten into a public flat view struct for the frontend"
    - "ScriptsWorkspace.tsx follows ScenesLooksWorkspace.tsx's load/refresh/error and selection-validity-repair effect verbatim"

key-files:
  created:
    - internal/wails/svc_script.go
    - internal/wails/svc_script_test.go
    - frontend/src/workspaces/build/ScriptsWorkspace.tsx
    - frontend/src/workspaces/build/ScriptsWorkspace.module.css
    - frontend/src/workspaces/build/ScriptsWorkspace.test.tsx
  modified:
    - cmd/golc-desktop/main.go
    - frontend/src/lib/wailsBridge.ts
    - frontend/src/shell/navigation.ts
    - frontend/src/shell/WorkspaceRouter.tsx

key-decisions:
  - "SaveScriptSource checks the 1 MiB maxScriptSourceBytes bound before writing any temp file, mirroring internal/command/script.go's identical constant"
  - "ScriptsWorkspace detects a missing window.go.wails.ScriptService itself (separately from the never-throwing listScripts wrapper) to render the UI-SPEC's 'Can't reach the script host' copy alongside the empty state, satisfying the plan's 'empty state AND inline error, never throwing' behavior bullet"
  - "Delete Script confirmation is an inline rendered block (UI-SPEC's exact copy), not window.confirm() -- consistent with this phase's declared HelpOverlay.tsx-style modal precedent rather than Phase 6's native-dialog pattern"

requirements-completed: [SCRP-01]

coverage:
  - id: D1
    description: "ScriptService Wails binding: ListScripts/GetScript/CreateScript/SaveScriptSource/DeleteScript/SetScriptProfile, all routed through the shared command registry"
    requirement: "SCRP-01"
    verification:
      - kind: unit
        ref: "internal/wails/svc_script_test.go (TestScriptService*, 8 tests)"
        status: pass
    human_judgment: false
  - id: D2
    description: "Build -> Scripts workspace: library list (name/status chip/scope), create/select/edit/save/delete round trip, empty state, capability-profile inspector"
    requirement: "SCRP-01"
    verification:
      - kind: unit
        ref: "frontend/src/workspaces/build/ScriptsWorkspace.test.tsx (9 tests)"
        status: pass
    human_judgment: false
  - id: D3
    description: "Manual end-to-end verification: launch the desktop app, create/edit/save a script, navigate away and back, confirm persistence"
    verification: []
    human_judgment: true
    rationale: "Requires a running golc-desktop.exe against a real show file; not exercised by this plan's automated test suite"

duration: 45min
completed: 2026-07-26
status: complete
---

# Phase 8 Plan 4: ScriptService Wails Binding and Scripts Workspace Summary

**Wails-bound ScriptService plus a Build -> Scripts library/editor/delete workspace, both routing through 08-01's `script *` CLI routes with no second persistence implementation**

## Performance

- **Duration:** ~45 min
- **Completed:** 2026-07-26
- **Tasks:** 2
- **Files modified:** 9 (5 created, 4 modified)

## Accomplishments

- `ScriptService` binds every `script *` CLI route (create/list/show/edit/delete/profile set) into the Wails webview, with `SaveScriptSource` safely round-tripping multi-line TypeScript source via a guarded temp file instead of argv interpolation
- Build -> Scripts is now a real, navigable, tested workspace: the D-16 library view (name, last-run status chip, capability-scope summary per row), a plain bounded `<textarea>` editor, and New Script / Save / Delete Script actions
- The selected script's capability profile (scope, resource-limit preset) renders into the shell's contextual inspector
- The workspace degrades gracefully with no bridge present, rendering the UI-SPEC's empty state plus its "Can't reach the script host" inline copy rather than throwing

## Task Commits

Each task was committed atomically (TDD RED -> GREEN per task):

1. **Task 1: ScriptService Wails binding**
   - `74dad28` test(08-04): add failing test for ScriptService Wails binding
   - `126ed3e` feat(08-04): implement ScriptService Wails binding
2. **Task 2: Scripts destination, bridge client, and ScriptsWorkspace library + editor**
   - `ba45cf7` test(08-04): add failing test for ScriptsWorkspace and wire Scripts destination
   - `5534b1c` feat(08-04): implement ScriptsWorkspace library/editor/delete round trip

_No separate plan-metadata commit: per the parallel-worktree execution contract, this SUMMARY.md is committed alongside these task commits; STATE.md/ROADMAP.md are updated centrally by the orchestrator after the wave merges._

## Files Created/Modified

- `internal/wails/svc_script.go` - ScriptService: ListScripts/GetScript/CreateScript/SaveScriptSource/DeleteScript/SetScriptProfile
- `internal/wails/svc_script_test.go` - 8 tests covering every `<behavior>` bullet (list empty/populated, missing show, get-with-source, duplicate-name rejection, source round trip, oversized-source rejection, delete, partial profile update)
- `cmd/golc-desktop/main.go` - constructs `scriptService` and adds it to the Wails `Bind` list
- `frontend/src/lib/wailsBridge.ts` - `ScriptSummaryView`/`ScriptDetailView` types, `ScriptServiceBinding`, `listScripts`/`getScript`/`createScript`/`saveScriptSource`/`deleteScript`/`setScriptProfile`/`offlineScriptList`
- `frontend/src/shell/navigation.ts` - `"build-scripts"` `DestinationId` + `{ id: "build-scripts", label: "Scripts" }` nav entry
- `frontend/src/shell/WorkspaceRouter.tsx` - `case "build-scripts": return <ScriptsWorkspace />;`
- `frontend/src/workspaces/build/ScriptsWorkspace.tsx` - the workspace component
- `frontend/src/workspaces/build/ScriptsWorkspace.module.css` - CSS Module using only the declared design tokens
- `frontend/src/workspaces/build/ScriptsWorkspace.test.tsx` - 9 tests covering every `<behavior>` bullet

## Decisions Made

- Reused `strictjson.DecodeStrict` (not plain `encoding/json`) to decode `script list`/`script show`'s Stdout, matching the field-complete round trip those routes already guarantee, then flattened the nested `capability_profile` JSON member into a flat Go/TS view type for simpler frontend consumption.
- The Delete Script confirmation is rendered inline (not `window.confirm()`), matching 08-UI-SPEC.md's exact locked copy and this phase's declared `HelpOverlay.tsx`-style modal precedent rather than reusing Phase 6's `SurfaceList.tsx` native-dialog convention.
- `ScriptsWorkspace` checks `window.go?.wails?.ScriptService` directly (in addition to calling the never-throwing `listScripts()` wrapper) so a missing bridge renders both the empty state and the UI-SPEC's "Can't reach the script host" inline message, satisfying the plan's explicit "never throws, but still shows an inline error" behavior bullet.

## Deviations from Plan

None - plan executed exactly as written. `go build ./...` initially failed with `pattern all:frontend/dist: no matching files found` because the frontend had never been built in this fresh worktree; running `npm --prefix frontend run build` (Task 2's own verification step) produced `frontend/dist` and resolved this — not a deviation, just build-order within the plan's own two tasks.

## Issues Encountered

- Early test-authoring pass used `screen.getByText(name)` for the created/selected script's name, which threw "found multiple elements" once the same name legitimately renders in both the library row and the editor Toolbar title (selection defaults to the sole/first script). Fixed by scoping row-level assertions to the `<ul aria-label="Script list">` container via `within()`.
- Used `&rsquo;`/typographic apostrophes in two copy strings that didn't byte-match the UI-SPEC's straight-apostrophe Copywriting Contract text used in tests; switched both to plain `'` via template-literal strings so the rendered DOM text matches the locked copy exactly.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- 08-10 (Run/Debug dialog, Stop Script) extends this plan's Toolbar action slot in place; no rewrite needed.
- 08-11 (Monaco editor) replaces this plan's plain `<textarea>` in place; the element carries an explicit source comment marking the handoff.
- Manual end-to-end verification (launch golc-desktop.exe, create/edit/save a script, confirm persistence across navigation) remains open — flagged as D3 in this summary's `coverage` block for the verifier.

---
*Phase: 08-isolated-typescript-automation*
*Completed: 2026-07-26*

## Self-Check: PASSED

All created files verified present on disk; all 4 task commits (74dad28, 126ed3e, ba45cf7, 5534b1c) verified present in git log.
