---
phase: 09-front-door-ui-completion
plan: 07
subsystem: fixture-catalog
tags: [go, react, wails, fixture-import, native-file-picker, hand-authored-yaml]

# Dependency graph
requires:
  - phase: 09-front-door-ui-completion (plan 02)
    provides: App's native-picker/cancellation guard contract (PickShowPath/PickNewShowPath) this plan's PickFixtureFile mirrors exactly
  - phase: 09-front-door-ui-completion (plan 06)
    provides: FixtureLibraryService's preview-then-commit staging directory, PreviewOFL/CommitPreview/DiscardPreview, and FixtureLibraryWorkspace.tsx's catalog candidate panel/confirm-action pattern this plan reuses rather than duplicating
provides:
  - internal/wails.App.PickFixtureFile -- native *.yaml/*.yml file picker for the hand-authored fixture add path
  - internal/wails.FixtureLibraryService.PreviewFile -- stages a hand-authored YAML fixture through the canonical "fixture inspect" route, sharing the preview registry with the OFL catalog path
  - an in-memory preview registry (token -> destination file name) that CommitPreview now reads from, letting one commit path serve both a JSON import artifact (OFL) and a raw YAML file (custom) with no second decode in the Wails layer
  - wailsBridge.ts's pickFixtureFile()/previewFixtureFile()
  - FixtureLibraryWorkspace.tsx's "Add Custom Fixture..." affordance (Fixture file path field + Browse... button + Validate action) sharing the exact candidate-preview rendering (renderCandidateBody) the OFL catalog path already uses
affects: []

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "In-memory preview registry (token -> destination) populated only by the methods that stage a preview (PreviewOFL/PreviewFile), never by caller-supplied input -- lets one commit path serve two different staged-artifact shapes (a JSON import envelope and a raw YAML file) without a second decode"
    - "Shared candidate-preview render function (renderCandidateBody) invoked from two otherwise-unrelated UI flows so there is exactly one confirm action and one error/warning presentation in source, not just visually"

key-files:
  created: []
  modified:
    - internal/wails/app.go
    - internal/wails/app_test.go
    - internal/wails/svc_fixturelibrary.go
    - internal/wails/svc_fixturelibrary_test.go
    - frontend/src/lib/wailsBridge.ts
    - frontend/src/workspaces/build/FixtureLibraryWorkspace.tsx
    - frontend/src/workspaces/build/FixtureLibraryWorkspace.test.tsx
    - .planning/phases/09-front-door-ui-completion/deferred-items.md

key-decisions:
  - "CommitPreview's destination file name now comes from an in-memory registry PreviewOFL/PreviewFile populate when they stage a preview, rather than being re-derived by decoding the staged artifact -- a staged custom fixture is raw YAML, not the JSON import envelope PreviewOFL stages, so a single commit path could not otherwise serve both without a second, format-specific decode"
  - "The custom-fixture destination file name is derived from the inspected stable key (lowercased, non [a-z0-9-] runs collapsed to a hyphen) with the source file's own extension appended, mirroring libraryFileNameForSource's identical 'commit destination mechanically derived, never caller-supplied' discipline for the OFL path"
  - "The custom-fixture candidate reuses the exact same previewView/previewing/previewError/committing/alreadyExists state and renderCandidateBody rendering the OFL catalog candidate already uses -- the two flows are mutually exclusive on screen, so one shared slot serves both confirm actions rather than a second copy of the same panel"

patterns-established:
  - "A staged preview's destination identity is decided once, at stage time, by the service itself, and handed to the commit step as an opaque registry lookup -- never re-derived from the staged bytes at commit time, which would require format-specific knowledge the commit step should not need"

requirements-completed: [FDUI-01]

coverage:
  - id: D1
    description: "An operator can add a hand-authored YAML fixture definition by pointing at a local file -- through a native OS file picker or a typed path field -- and it is validated inline before it is added"
    requirement: "FDUI-01"
    verification:
      - kind: unit
        ref: "internal/wails/svc_fixturelibrary_test.go#TestPreviewFileStagesAValidDefinitionWithoutTouchingTheLibrary"
        status: pass
      - kind: unit
        ref: "frontend/src/workspaces/build/FixtureLibraryWorkspace.test.tsx#Browse populates the field from the native picker"
        status: pass
      - kind: unit
        ref: "frontend/src/workspaces/build/FixtureLibraryWorkspace.test.tsx#cancelling the picker leaves the field unchanged"
        status: pass
    human_judgment: false
  - id: D2
    description: "The workspace offers an 'Add Custom Fixture...' action and a 'Fixture file path' field with a 'Browse...' button that opens the native picker"
    requirement: "FDUI-01"
    verification:
      - kind: unit
        ref: "frontend/src/workspaces/build/FixtureLibraryWorkspace.test.tsx#Add Custom Fixture reveals the file path field with a Browse button"
        status: pass
    human_judgment: false
  - id: D3
    description: "There is no in-app YAML text editor -- the custom-fixture path is point-at-a-file plus inline validation"
    requirement: "FDUI-01"
    verification:
      - kind: other
        ref: "grep -v '^//' frontend/src/workspaces/build/FixtureLibraryWorkspace.tsx | grep -c -E \"<textarea|CodeMirror|Monaco|contentEditable\" == 0"
        status: pass
    human_judgment: false
  - id: D4
    description: "While inline validation runs, the field and confirm action are disabled and show a busy label, mirroring ScriptsWorkspace's busy-disable pattern"
    requirement: "FDUI-01"
    verification:
      - kind: unit
        ref: "frontend/src/workspaces/build/FixtureLibraryWorkspace.test.tsx#the field and confirm action are disabled with a busy label while validation runs"
        status: pass
    human_judgment: false
  - id: D5
    description: "A file that cannot be read, or whose definition is invalid, renders the same '{N} error(s)' diagnostic surface the catalog path uses"
    requirement: "FDUI-01"
    verification:
      - kind: unit
        ref: "internal/wails/svc_fixturelibrary_test.go#TestPreviewFileReportsAnUnreadablePath"
        status: pass
      - kind: unit
        ref: "internal/wails/svc_fixturelibrary_test.go#TestPreviewFileReportsAnInvalidDefinition"
        status: pass
      - kind: unit
        ref: "frontend/src/workspaces/build/FixtureLibraryWorkspace.test.tsx#an unreadable or invalid file renders the shared {N} error(s) copy and leaves Add to Library disabled"
        status: pass
    human_judgment: false
  - id: D6
    description: "A hand-authored fixture is validated by the same 'fixture inspect' pipeline the CLI uses; the Wails layer runs no second decode, validation, or pin"
    requirement: "FDUI-01"
    verification:
      - kind: other
        ref: "grep -v '^//' internal/wails/svc_fixturelibrary.go | grep -c -E \"fixture\\.Decode\\(|fixture\\.Validate\\(|ofl\\.Normalize\" == 0"
        status: pass
    human_judgment: false
  - id: D7
    description: "Committing a custom fixture never silently replaces an existing library entry -- the same refuse-and-offer-replace behaviour the catalog import path uses applies unchanged"
    requirement: "FDUI-01"
    verification:
      - kind: unit
        ref: "internal/wails/svc_fixturelibrary_test.go#TestCommitPreviewRefusesExistingDestinationForCustomFixtures"
        status: pass
    human_judgment: false
  - id: D8
    description: "Manual desktop-app verification: click Add Custom Fixture..., Browse... to a real hand-authored fixture YAML, confirm identity/validation render in the shared panel, confirm a deliberately broken YAML shows the {N} error(s) copy with Add to Library disabled, and confirm a valid one lands in My Library immediately after confirming"
    verification: []
    human_judgment: true
    rationale: "Requires launching the real golc-desktop app and using the native OS file picker against a real file on disk -- outside this automated unit-test suite's reach (09-07-PLAN.md Task 3's own <verify><human-check> block)"

# Metrics
duration: ~55min
completed: 2026-07-28
status: complete
---

# Phase 09 Plan 07: Hand-Authored YAML Fixture Add Summary

**FixtureLibraryService gained PreviewFile (a native *.yaml/*.yml picker plus canonical-pipeline validation) and a shared in-memory preview registry that lets one CommitPreview path serve both the OFL catalog import and a hand-authored YAML file, with the workspace's "Add Custom Fixture..." affordance reusing the exact same candidate-preview panel and "Add to Library" confirm action the catalog path already established.**

## Performance

- **Duration:** ~55 min
- **Tasks:** 3
- **Files modified:** 8 (0 created, 8 modified)

## Accomplishments

- `App.PickFixtureFile` opens a native "Add Custom Fixture" file picker filtered to `*.yaml;*.yml`, mirroring `PickShowPath`'s exact `GOLC_WAILS_RUNTIME_CONTEXT_UNAVAILABLE` guard and empty-string-on-cancel contract.
- Replaced `CommitPreview`'s destination-file-name derivation with an in-memory preview registry (`previewRegistry map[string]string`, guarded by `previewMu`): `PreviewOFL` and the new `PreviewFile` each record the token they issued together with the destination they computed, and `CommitPreview` now reads that registry instead of decoding the staged artifact -- the mechanism that lets one commit path serve both a JSON import envelope (OFL) and a raw YAML file (custom) with no second, format-specific decode in the Wails layer. Confirmed non-breaking against every plan 09-06 preview/commit test (`TestPreviewRegistryKeepsCatalogBehaviourUnchanged`).
- `FixtureLibraryService.PreviewFile(path)` runs `s.Inspect(path)` -- the sole validation authority, forwarding to the registered `"fixture inspect"` route -- and, on a valid result, copies the operator's file byte-for-byte into the preview directory, derives the destination file name from the inspected stable key (lowercased, non-`[a-z0-9-]` runs collapsed to a hyphen, source extension appended), and registers the token/destination pair. An unreadable path and an invalid definition both project as `Inspect.Valid:false` with nothing staged and a nil error -- one error surface for both failure modes.
- `wailsBridge.ts` gained `pickFixtureFile()`/`previewFixtureFile(path)`, following the existing never-throw/offline-fallback contract (`AppBinding.PickFixtureFile`, `FixtureLibraryServiceBinding.PreviewFile`).
- `FixtureLibraryWorkspace.tsx`'s "My Library" side gained an "Add Custom Fixture..." action in the list panel's header, revealing a "Fixture file path" `Field` with a "Browse..." button (calls `pickFixtureFile`) and a "Validate" action (calls `previewFixtureFile`) that disables the field/Browse/Validate with a busy "Validating..." label while in flight. The candidate result renders through a single extracted `renderCandidateBody` function shared with the OFL catalog path -- identity/content-hash, schema/revision, validation chip, itemized errors/warnings, and the one "Add to Library" primary confirm action (or "Replace" for an already-present destination) -- so there is exactly one confirm action and one error presentation in source, not merely visually. A staged custom-fixture preview is discarded when the path is edited, the affordance is dismissed, or the operator switches to the catalog side.

## Task Commits

1. **Task 1: Write the failing custom-YAML picker and preview tests** - `77ff6a38` (test)
2. **Task 2: Add the YAML file picker and file-preview staging** - `384c2afc` (feat)
3. **Task 3: Wire the Add Custom Fixture affordance into the workspace** - `8c5e77b6` (feat)

## Files Created/Modified

- `internal/wails/app.go` - `fixtureFileFilter`, `App.PickFixtureFile`
- `internal/wails/app_test.go` - `TestPickFixtureFileWithoutRuntimeContextFails`
- `internal/wails/svc_fixturelibrary.go` - `previewRegistry` field, `registerPreviewDestination`/`previewDestination`/`forgetPreviewDestination`, `libraryFileNameForCustomFixture`, `FixtureLibraryService.PreviewFile`, `nextCustomPreviewPath`, `copyFileBytes`; `CommitPreview`/`DiscardPreview` rewired to the registry
- `internal/wails/svc_fixturelibrary_test.go` - `TestPreviewRegistryKeepsCatalogBehaviourUnchanged`, `TestPreviewFileStagesAValidDefinitionWithoutTouchingTheLibrary`, `TestPreviewFileReportsAnUnreadablePath`, `TestPreviewFileReportsAnInvalidDefinition`, `TestCommitPreviewWritesTheCustomFixtureVerbatim`, `TestCommitPreviewRefusesExistingDestinationForCustomFixtures`
- `frontend/src/lib/wailsBridge.ts` - `AppBinding.PickFixtureFile`, `FixtureLibraryServiceBinding.PreviewFile`, `pickFixtureFile()`, `previewFixtureFile()`
- `frontend/src/workspaces/build/FixtureLibraryWorkspace.tsx` - custom-fixture state/handlers (`addingCustomFixture`, `customFixturePath`, `handleToggleAddCustomFixture`, `handleCustomFixturePathChange`, `handleBrowseCustomFixture`, `handleValidateCustomFixture`), extracted `renderCandidateBody`, the "Add Custom Fixture..." header action and its Field/Browse/Validate body
- `frontend/src/workspaces/build/FixtureLibraryWorkspace.test.tsx` - `installMockAppWithPicker` helper, `PreviewFile` added to the default service mock, and the "Custom YAML fixture add" describe block (5 new tests)
- `.planning/phases/09-front-door-ui-completion/deferred-items.md` - re-confirmed the pre-existing `internal/command` toolchain-bootstrap failures and logged a transient host-disk-space linker failure, neither touching this plan's files

## Decisions Made

- The preview registry is populated only by `PreviewOFL`/`PreviewFile` themselves (never from caller-supplied input), so `CommitPreview`'s existing containment check and already-exists refusal continue to guard the only two ways a token can exist -- see `key-decisions` above.
- The custom-fixture destination file name is derived from the fixture's own pinned stable key rather than the source file's name, so two files with different names but the same manufacturer/model still collide predictably (matching the OFL path's identical "identity decides the destination" rule) rather than silently allowing duplicate library entries under different file names.

## Deviations from Plan

### Notes (no code changes)

**1. Plan's mechanical `grep -c "Add to Library"` acceptance criterion counts comment mentions, not the rendered string**
- **Found during:** Task 3 verification
- **Issue:** The plan's acceptance criterion `grep -c "Add to Library" frontend/src/workspaces/build/FixtureLibraryWorkspace.tsx is 1` returns `2` — one is the actual JSX-rendered button label (`renderCandidateBody`'s single occurrence), the other is the fallback error string `"Add to Library failed"` in `handleCommit`. A raw substring grep also matches this plan's own doc comments mentioning the phrase (5 more, if comments aren't excluded), a known false-positive class already documented identically in `09-06-SUMMARY.md` (its `declare global` count finding).
- **Verification:** `grep -n '"Add to Library"' frontend/src/workspaces/build/FixtureLibraryWorkspace.tsx` shows the literal button-label string rendered exactly once (`{committing ? "Adding…" : "Add to Library"}`), confirming the intent the criterion describes ("one confirm action serves both paths") is satisfied.
- **Impact:** None — no code change made; the substring match against an unrelated string and doc comments is the criterion's own over-broad regex, not a defect.

---

**Total deviations:** 0 code changes; 1 documented grep-approximation note.
**Impact on plan:** No scope creep.

## Issues Encountered

- This worktree had no `frontend/node_modules` and no `cmd/golc-desktop/frontend/dist` (git worktrees don't share npm-installed dependencies or embedded build output with the main checkout) — the same one-time setup gap `09-01-SUMMARY.md`/`09-05-SUMMARY.md`/`09-06-SUMMARY.md` each independently documented. Resolved with `npm ci` in `frontend/` and `npm run build` (which also runs `tsc --noEmit` and the full `vitest run` suite, 273/273 passing) before the final `go build ./...`/`go test ./...` verification.
- `go test ./...` shows the same five pre-existing `internal/command` toolchain-bootstrap failures (`TestBuildRouteCompilesTheProductionRepository`, `TestBuildablePackagesExcludesMagefiles`, `TestScopeCrossPlatformCI`, `TestScopeGreenSubprocess`, `TestScopeOfflineAcceptance`) already documented in `09-01`/`09-02`-`09-06-SUMMARY.md` — all `GOLC_TEST_TOOLCHAIN_MISSING`, requiring `mage Bootstrap` in this worktree. Confirmed unrelated to any file this plan touches (`internal/wails` itself passes fully). A transient host-disk-space condition (`C:` at ~178MB free) additionally caused a one-time linker failure for several unrelated packages' test binaries on the first `go test ./...` run; re-running those packages individually confirmed they pass and the failure does not recur. Logged to `deferred-items.md`; out of this plan's scope.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- FDUI-01 is now fully delivered across plans 09-01/09-05/09-06/09-07: an operator can browse, search, inspect, and import fixture definitions from both the Open Fixture Library and hand-authored YAML, entirely through the workspace, with every validation verdict on screen coming from the canonical CLI pipeline.
- The manual desktop-app verification noted in Task 3's `<human-check>` block (click "Add Custom Fixture...", "Browse..." to a real hand-authored fixture YAML, confirm identity/validation render in the shared panel, confirm a broken YAML shows the "{N} error(s)" copy with "Add to Library" disabled, confirm a valid one lands in "My Library" immediately after confirming) was not run as part of this automated execution and remains open for end-of-phase UAT (coverage `D8`).
- The in-memory preview registry (`registerPreviewDestination`/`previewDestination`/`forgetPreviewDestination`) is now established, reusable precedent for any future plan that stages a third kind of preview through this same `CommitPreview`/`DiscardPreview` pair.

---
*Phase: 09-front-door-ui-completion*
*Completed: 2026-07-28*

## Self-Check: PASSED

All modified files (`internal/wails/app.go`, `internal/wails/app_test.go`,
`internal/wails/svc_fixturelibrary.go`, `internal/wails/svc_fixturelibrary_test.go`,
`frontend/src/lib/wailsBridge.ts`, `frontend/src/workspaces/build/FixtureLibraryWorkspace.tsx`,
`frontend/src/workspaces/build/FixtureLibraryWorkspace.test.tsx`,
`.planning/phases/09-front-door-ui-completion/deferred-items.md`, this SUMMARY.md)
and all three task commit hashes (`77ff6a38`, `384c2afc`, `8c5e77b6`) were
verified present on disk / in `git log --oneline --all` before this file was
finalized.
