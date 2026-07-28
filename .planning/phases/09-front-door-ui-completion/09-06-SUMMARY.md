---
phase: 09-front-door-ui-completion
plan: 06
subsystem: fixture-catalog
tags: [go, react, wails, fixture-import, preview-then-commit, open-fixture-library]

# Dependency graph
requires:
  - phase: 09-front-door-ui-completion (plan 01)
    provides: FixtureLibraryWorkspace.tsx's browse/search/inline-inspect skeleton and FixtureLibraryService's ListLocal/Inspect bindings
  - phase: 09-front-door-ui-completion (plan 05)
    provides: the source toggle, OFL manufacturer catalog search (SearchOFL), and fixture.ImportEnvelope/DecodeEnvelope as the shared import-artifact shape this plan's PreviewOFL/CommitPreview decode through
provides:
  - FixtureLibraryService.PreviewOFL/CommitPreview/DiscardPreview -- preview-then-commit OFL import over the existing "fixture import" CLI route
  - wailsBridge.ts's previewOflFixture/commitFixturePreview/discardFixturePreview
  - FixtureLibraryWorkspace.tsx's catalog candidate panel (fixture-key field, Preview action, inline inspect result, "Add to Library" confirm action, "Replace" for an already-present destination)
affects: [09-07-front-door-ui-completion]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Per-service-instance preview directory (os.MkdirTemp, lazily created, outside both project root and library directory) as a staging area for a mutation the operator has not yet confirmed"
    - "Containment check via filepath.Abs + filepath.Rel on a path-separator boundary (isPathWithinDir) rather than a bare string-prefix comparison, to validate a webview-supplied token before any filesystem write"
    - "Destination file name re-derived from the previewed artifact's own Provenance.Source at commit time, never trusted from caller-supplied input at the API boundary"

key-files:
  created: []
  modified:
    - internal/wails/svc_fixturelibrary.go
    - internal/wails/svc_fixturelibrary_test.go
    - frontend/src/lib/wailsBridge.ts
    - frontend/src/workspaces/build/FixtureLibraryWorkspace.tsx
    - frontend/src/workspaces/build/FixtureLibraryWorkspace.test.tsx
    - frontend/src/workspaces/build/FixtureLibraryWorkspace.module.css

key-decisions:
  - "CommitPreview re-derives the destination file name from the previewed artifact's own Provenance.Source (read back from disk) rather than trusting a filename argument from the frontend -- the commit-time destination can never be spoofed independently of what was actually previewed"
  - "The preview token round-tripped between Go and the frontend is the previewed artifact's own absolute filesystem path -- an internal opaque handle, never rendered in the UI, always re-validated against the service's own preview directory before every CommitPreview/DiscardPreview call"
  - "PreviewOFL's mirror/allow-mirror test seam (previewMirror/previewAllowMirror, unexported fields) mirrors ofl.OFLRef's existing Mirror/AllowMirror shape exactly, so the RED test suite exercises PreviewOFL against a local httptest server rather than the live catalog with no new mechanism"
  - "FDUI-01 is NOT marked complete in REQUIREMENTS.md by this plan: FDUI-01 requires both Open Fixture Library import (this plan) and hand-authored YAML import (09-07, not yet executed) -- matching 09-05-SUMMARY.md's identical restraint on the same requirement"

patterns-established:
  - "Pattern: a mutation the operator must explicitly confirm gets staged into a dedicated, non-scanned directory first; the commit step re-validates both the token's containment and the destination's own derived identity before writing, rather than trusting either side of a two-step RPC independently"

requirements-completed: []

coverage:
  - id: D1
    description: "Choosing a manufacturer and supplying a fixture key produces an inline inspect view of the candidate (identity, provenance, validation, lossy-import warnings) before anything is written into the library"
    requirement: "FDUI-01"
    verification:
      - kind: unit
        ref: "internal/wails/svc_fixturelibrary_test.go#TestPreviewOFLWritesNothingIntoTheLibrary"
        status: pass
      - kind: unit
        ref: "internal/wails/svc_fixturelibrary_test.go#TestPreviewOFLReturnsInspectViewWithWarnings"
        status: pass
      - kind: unit
        ref: "frontend/src/workspaces/build/FixtureLibraryWorkspace.test.tsx#renders the candidate inspect panel before anything is committed"
        status: pass
    human_judgment: false
  - id: D2
    description: "'Add to Library' is the single confirm action and stays disabled until the candidate passes validation"
    requirement: "FDUI-01"
    verification:
      - kind: unit
        ref: "internal/wails/svc_fixturelibrary_test.go#TestPreviewOFLReturnsErrorsForInvalidCandidate"
        status: pass
      - kind: unit
        ref: "frontend/src/workspaces/build/FixtureLibraryWorkspace.test.tsx#Add to Library is disabled for an invalid candidate"
        status: pass
    human_judgment: false
  - id: D3
    description: "An import carrying unsupported/approximated attributes renders the non-blocking 'unsupported or approximated attribute(s)' warning, itemized per attribute, with Add to Library still enabled"
    requirement: "FIXT-06"
    verification:
      - kind: unit
        ref: "internal/wails/svc_fixturelibrary_test.go#TestPreviewOFLReturnsInspectViewWithWarnings"
        status: pass
      - kind: unit
        ref: "frontend/src/workspaces/build/FixtureLibraryWorkspace.test.tsx#renders the lossy-import warning copy with the count and the per-attribute details, with Add to Library still enabled"
        status: pass
    human_judgment: false
  - id: D4
    description: "Committing an import writes the exact bytes the existing 'fixture import' route produced -- no second normalization, re-encoding, or independent pin in the Wails layer"
    requirement: "FDUI-01"
    verification:
      - kind: unit
        ref: "internal/wails/svc_fixturelibrary_test.go#TestCommitPreviewWritesTheExactPreviewedBytes"
        status: pass
      - kind: other
        ref: "grep -v '^//' internal/wails/svc_fixturelibrary.go | grep -c -E 'ofl\\.Normalize\\(|ofl\\.Fetch\\(|fixture\\.Decode\\(' == 0 (exact-call-form check; see Deviations for a noted grep-approximation false positive in the plan's own broader regex)"
        status: pass
    human_judgment: false
  - id: D5
    description: "Committing never silently replaces an existing library entry: an existing destination is refused with a distinct diagnostic, and replacing is a separate explicit operator action"
    requirement: "FDUI-01"
    verification:
      - kind: unit
        ref: "internal/wails/svc_fixturelibrary_test.go#TestCommitPreviewRefusesExistingDestination"
        status: pass
      - kind: unit
        ref: "internal/wails/svc_fixturelibrary_test.go#TestCommitPreviewReplacesOnlyWithExplicitOverwrite"
        status: pass
      - kind: unit
        ref: "frontend/src/workspaces/build/FixtureLibraryWorkspace.test.tsx#an already-present fixture is reported, not silently replaced"
        status: pass
    human_judgment: false
  - id: D6
    description: "A preview that is never committed leaves nothing behind in the fixture library directory; a preview token outside the service's own preview directory is refused before any filesystem operation"
    requirement: "FDUI-01"
    verification:
      - kind: unit
        ref: "internal/wails/svc_fixturelibrary_test.go#TestPreviewOFLWritesNothingIntoTheLibrary"
        status: pass
      - kind: unit
        ref: "internal/wails/svc_fixturelibrary_test.go#TestCommitPreviewRejectsATokenOutsideThePreviewDirectory"
        status: pass
      - kind: unit
        ref: "internal/wails/svc_fixturelibrary_test.go#TestDiscardPreviewRemovesTheCandidate"
        status: pass
    human_judgment: false
  - id: D7
    description: "Manual desktop-app verification: searching the catalog, previewing a real fixture key, seeing its identity/warnings inline, adding it, and confirming a repeat import offers Replace rather than silently overwriting"
    verification: []
    human_judgment: true
    rationale: "Requires running the built golc-desktop app, a real network-reachable Open Fixture Library fetch (or a configured mirror), and visually confirming the candidate panel/Add to Library/Replace flow -- outside this automated unit-test suite's reach (09-06-PLAN.md Task 3's own <human-check> block)."

# Metrics
duration: 55min
completed: 2026-07-28
status: complete
---

# Phase 09 Plan 06: Preview-Then-Commit Open Fixture Library Import Summary

**FixtureLibraryService gained PreviewOFL/CommitPreview/DiscardPreview -- a staged, containment-checked preview-then-commit import over the existing "fixture import" CLI route -- and the Fixture Library workspace's catalog side gained the fixture-key field, inline candidate inspect panel, and "Add to Library" confirm action, closing FDUI-01's import half.**

## Performance

- **Duration:** ~55 min
- **Started:** 2026-07-28T00:00:00Z (approx.)
- **Completed:** 2026-07-28T00:55:00Z (approx.)
- **Tasks:** 3
- **Files modified:** 6 (0 created, 6 modified)

## Accomplishments
- `FixtureLibraryService.PreviewOFL` stages an OFL import for an operator-supplied manufacturer/fixture-key pair into a dedicated, per-service-instance preview directory (outside both the project root and the library directory, never scanned by `ListLocal`) by forwarding to the existing, already-tested `"fixture import"` route -- the Wails layer performs no fetch, normalization, pinning, or validation of its own. A rejected candidate (fetch/normalize failure) projects as `Inspect.Valid:false` with `Errors` from the route's own stderr, never a thrown error.
- `CommitPreview` verifies a caller-supplied preview token resolves strictly inside the service's own preview directory (`isPathWithinDir`, an absolute-path + `filepath.Rel` separator-boundary check, never a bare string prefix) before touching the filesystem, re-derives the destination file name from the previewed artifact's own `Provenance.Source`, refuses an already-present destination with `GOLC_WAILS_FIXTURE_IMPORT_EXISTS` unless `overwrite` is set, and moves (`os.Rename`, never a copy-and-re-encode) the previewed bytes into the library so the committed file is byte-identical to what `"fixture import"` originally wrote.
- `DiscardPreview` removes an abandoned staged preview under the identical containment check.
- `wailsBridge.ts` gained `FixturePreviewView` and `previewOflFixture`/`commitFixturePreview`/`discardFixturePreview`, following the existing never-throw / offline-fallback contract (`offlineFixturePreviewView` keeps `inspect.valid` false so a missing bridge never lies about candidate health).
- `FixtureLibraryWorkspace.tsx`'s catalog side: selecting a manufacturer reveals a `Field` for the fixture key and a "Preview" action, all inline in the existing workspace body (never a modal/dialog/wizard). The returned candidate renders in the same inline inspect panel the local side already uses -- stable key, content hash, revision, validation chip, itemized lossy-import warnings (non-blocking, always visible), and the `"Add to Library"` primary confirm action, disabled until the candidate validates. A commit that finds the destination already present renders a distinct "already in your library" message with a separate "Replace" action instead of silently overwriting anything. Changing the manufacturer selection, editing the fixture key, or switching back to "My Library" discards any staged preview.

## Task Commits

Each task was committed atomically:

1. **Task 1: Write the failing preview-then-commit tests** - `6f7e6147` (test)
2. **Task 2: Implement preview-then-commit import on the Wails service** - `8b61aab8` (feat)
3. **Task 3: Wire the catalog candidate panel and the Add to Library confirm action** - `0ed76ced` (feat)

## Files Created/Modified
- `internal/wails/svc_fixturelibrary.go` - `FixturePreviewView`, `PreviewOFL`, `CommitPreview`, `DiscardPreview`, `isPathWithinDir`, `libraryFileNameForSource`, per-instance preview directory (`ensurePreviewDir`/`nextPreviewPath`), `previewMirror`/`previewAllowMirror` test seam
- `internal/wails/svc_fixturelibrary_test.go` - 8 new tests against an `httptest` mirror serving the shared `chauvet-dj/led-par-64-tri-b` OFL corpus fixture (write-nothing, warnings, invalid candidate, byte-identical commit, existing-destination refusal, explicit overwrite, token containment, discard)
- `frontend/src/lib/wailsBridge.ts` - `FixturePreviewView` type, extended `FixtureLibraryServiceBinding`, `previewOflFixture`/`commitFixturePreview`/`discardFixturePreview`, `offlineFixturePreviewView`
- `frontend/src/workspaces/build/FixtureLibraryWorkspace.tsx` - catalog candidate state/handlers (`handleSelectManufacturer`, `handleFixtureKeyChange`, `handlePreview`, `handleCommit`, `resetCandidate`/`discardCandidatePreview`), the fixture-key `Field` + Preview action, and the candidate inspect/commit UI
- `frontend/src/workspaces/build/FixtureLibraryWorkspace.test.tsx` - 4 new tests for the candidate flow (inspect panel, disabled Add to Library, lossy-warning copy, already-present reporting)
- `frontend/src/workspaces/build/FixtureLibraryWorkspace.module.css` - `.candidateBody`/`.candidateManufacturer`/`.alreadyExists` classes, declared spacing scale only

## Decisions Made
- `CommitPreview` re-derives the destination file name from the previewed artifact's own `Provenance.Source` (read back from disk at commit time) rather than accepting a filename from the frontend -- the commit destination can never diverge from what was actually previewed.
- The preview token is the previewed artifact's own absolute filesystem path, treated as an opaque handle: never rendered in the UI, always re-validated against the service's own preview directory before every `CommitPreview`/`DiscardPreview` call (T-09-06-01).
- `FDUI-01` is intentionally left unmarked in `REQUIREMENTS.md` by this plan (`requirements-completed: []`) -- it spans plans 09-01/09-05/09-06/09-07, and 09-07 (hand-authored YAML add) has not executed yet, matching `09-05-SUMMARY.md`'s identical restraint on the same requirement.

## Deviations from Plan

### Notes (no code changes)

**1. Plan's mechanical acceptance-criteria regex has a false-positive pre-existing match**
- **Found during:** Task 2 verification
- **Issue:** The plan's acceptance criterion `grep -v '^//' internal/wails/svc_fixturelibrary.go | grep -c -E "ofl\.Normalize|ofl\.Fetch|fixture\.Decode\(" is 0` returns `1`, not `0`. The sole match is the pre-existing `ofl.FetchManufacturers(...)` call added by plan 09-05 for the manufacturer-index catalog search -- the `ofl\.Fetch` alternative (with no trailing paren) matches it as a substring, which is unrelated to this plan's fixture-import pipeline.
- **Verification:** `grep -n "ofl\.Normalize(\|ofl\.Fetch(\|fixture\.Decode(" internal/wails/svc_fixturelibrary.go` (exact call forms, trailing paren) returns zero matches -- confirming no second fetch/normalize/decode import pipeline exists in this plan's new code. The intent the criterion describes ("no second import pipeline exists in the Wails layer") is satisfied; the literal regex is over-broad against a call this plan did not introduce.
- **Impact:** None -- no code change made. Documented here so the discrepancy is not mistaken for an unaddressed gap.

**2. Plan's `declare global` count criterion counts comment mentions, not blocks**
- **Found during:** Task 2 verification
- **Issue:** `grep -c "declare global" frontend/src/lib/wailsBridge.ts` returns `3` (pre-existing, unrelated to this plan): one real `declare global { ... }` block plus two comment lines that mention the phrase while explaining why a second block must never be added. The criterion ("is 1") was already numerically false before this plan touched the file.
- **Verification:** Confirmed exactly one actual `declare global {` block exists before and after this plan's edit -- this plan only extended the existing `FixtureLibraryServiceBinding` interface, adding no second block.
- **Impact:** None -- no code change made.

---

**Total deviations:** 0 code changes; 2 documented grep-approximation notes.
**Impact on plan:** No scope creep; both notes concern pre-existing conditions from prior plans, not this plan's own code.

## Issues Encountered
- This worktree had no `frontend/node_modules` and no `cmd/golc-desktop/frontend/dist` (git worktrees don't share npm-installed dependencies or embedded build output with the main checkout), so an initial `go build ./...` failed on the `//go:embed all:frontend/dist` directive. Resolved with `npm ci` in `frontend/` (background) and `npm run build` (which also runs `tsc --noEmit` and the full `vitest run` suite) before final verification -- one-time worktree setup, not a code change. Matches the identical issue documented in `09-05-SUMMARY.md`.
- `go test ./...` shows the same five pre-existing failures in `internal/command` (`TestBuildRouteCompilesTheProductionRepository`, `TestBuildablePackagesExcludesMagefiles`, `TestScopeCrossPlatformCI`, `TestScopeGreenSubprocess`, `TestScopeOfflineAcceptance`) already documented in `09-05-SUMMARY.md` -- all `GOLC_TEST_TOOLCHAIN_MISSING`/pinned-binary-not-built, requiring `mage Bootstrap` in this worktree. Confirmed unrelated to any file this plan touches (`internal/wails` itself passes fully); out of this plan's scope per the deviation rules' scope boundary.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- `FDUI-01` spans plans 09-01, 09-05, 09-06, and 09-07; this plan does not mark it complete in `REQUIREMENTS.md` since 09-07 (hand-authored YAML add) has not executed yet.
- The operator can now discover, inspect, review lossy-import warnings for, and import an Open Fixture Library fixture entirely on screen, with a hand-edited library entry protected from silent replacement.
- `09-07-PLAN.md` (custom YAML add, D-04) is unaffected by this plan's changes -- it uses a different source path (native file picker + `fixture validate`) and remains ready independently.
- The manual desktop-app verification noted in Task 3's `<human-check>` block (search the catalog for a real manufacturer, preview a real fixture key, confirm identity/warnings render inline, add it, re-import and confirm Replace is offered rather than silent overwrite) was not run as part of this automated execution and remains open for a human verification pass (coverage `D7`).

---
*Phase: 09-front-door-ui-completion*
*Completed: 2026-07-28*

## Self-Check: PASSED

All modified files (`internal/wails/svc_fixturelibrary.go`, `internal/wails/svc_fixturelibrary_test.go`, `frontend/src/lib/wailsBridge.ts`, `frontend/src/workspaces/build/FixtureLibraryWorkspace.tsx`, `frontend/src/workspaces/build/FixtureLibraryWorkspace.test.tsx`, `frontend/src/workspaces/build/FixtureLibraryWorkspace.module.css`, this SUMMARY.md) and all three task commit hashes (`6f7e6147`, `8b61aab8`, `0ed76ced`) were verified present on disk / in `git log --oneline --all` before this file was finalized.
