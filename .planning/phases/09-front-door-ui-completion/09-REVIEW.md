---
phase: 09-front-door-ui-completion
reviewed: 2026-07-28T00:00:00Z
depth: standard
files_reviewed: 40
files_reviewed_list:
  - cmd/golc-desktop/main.go
  - frontend/src/lib/wailsBridge.ts
  - frontend/src/shell/AppShell.tsx
  - frontend/src/shell/WorkspaceRouter.tsx
  - frontend/src/shell/destinationIcons.tsx
  - frontend/src/shell/navigation.ts
  - frontend/src/workspaces/build/FixtureLibraryWorkspace.module.css
  - frontend/src/workspaces/build/FixtureLibraryWorkspace.test.tsx
  - frontend/src/workspaces/build/FixtureLibraryWorkspace.tsx
  - frontend/src/workspaces/show/GuidedFirstShow/GuideEvidenceList.tsx
  - frontend/src/workspaces/show/GuidedFirstShow/GuidedFirstShow.module.css
  - frontend/src/workspaces/show/GuidedFirstShow/GuidedFirstShow.test.tsx
  - frontend/src/workspaces/show/GuidedFirstShow/GuidedFirstShow.tsx
  - frontend/src/workspaces/show/GuidedFirstShow/GuidedFirstShowContext.tsx
  - frontend/src/workspaces/show/GuidedFirstShow/readiness.ts
  - frontend/src/workspaces/show/GuidedFirstShow/stages.ts
  - frontend/src/workspaces/show/GuidedFirstShow/stages/AssignStage.tsx
  - frontend/src/workspaces/show/GuidedFirstShow/stages/FixturesStage.tsx
  - frontend/src/workspaces/show/GuidedFirstShow/stages/PatchStage.tsx
  - frontend/src/workspaces/show/GuidedFirstShow/stages/ProgramStage.tsx
  - frontend/src/workspaces/show/GuidedFirstShow/stages/VerifyStage.tsx
  - frontend/src/workspaces/show/OverviewWorkspace.module.css
  - frontend/src/workspaces/show/OverviewWorkspace.test.tsx
  - frontend/src/workspaces/show/OverviewWorkspace.tsx
  - frontend/src/workspaces/show/SaveRecoveryWorkspace.tsx
  - frontend/src/workspaces/show/ShowsWorkspace.module.css
  - frontend/src/workspaces/show/ShowsWorkspace.test.tsx
  - frontend/src/workspaces/show/ShowsWorkspace.tsx
  - internal/command/artnet.go
  - internal/command/fixture.go
  - internal/fixture/directory.go
  - internal/fixture/directory_test.go
  - internal/fixture/envelope.go
  - internal/fixture/envelope_test.go
  - internal/fixture/ofl/fetch.go
  - internal/fixture/ofl/manufacturers.go
  - internal/fixture/ofl/manufacturers_test.go
  - internal/wails/app.go
  - internal/wails/app_test.go
  - internal/wails/svc_fixturelibrary.go
  - internal/wails/svc_fixturelibrary_test.go
findings:
  critical: 0
  warning: 4
  info: 3
  total: 7
status: issues_found
---

# Phase 09: Code Review Report

**Reviewed:** 2026-07-28T00:00:00Z
**Depth:** standard
**Files Reviewed:** 40
**Status:** issues_found

## Summary

Reviewed the front-door UI completion phase: the desktop shell wiring (`main.go`, `AppShell`, `WorkspaceRouter`, `navigation`), the Wails bridge layer (`wailsBridge.ts`), the Fixture Library workspace (browse/inspect/OFL-catalog-import/custom-YAML-import), the Guided First Show overlay (context, stages, readiness derivations), the Shows/Save-Recovery/Overview workspaces, and their Go-side backends (`FixtureLibraryService`, `internal/fixture` directory/envelope decode, `internal/fixture/ofl` fetch/manufacturers, `internal/wails/app.go`'s relaunch/picker plumbing, `internal/command/fixture.go` and `artnet.go`).

The Go side is solid: the SSRF guard in `ofl/fetch.go` (scheme + host allowlist, redirect re-validation, size cap, timeout) is correctly implemented and well tested; `FixtureLibraryService`'s preview-then-commit flow correctly contains preview tokens inside its own temp directory (`isPathWithinDir`) and derives commit destinations by stripping path separators, so no path-traversal vector was found despite the destination filename being partly built from operator-supplied text. Test coverage on the Go side is thorough (positive/negative/boundary cases for directory scanning, envelope decoding, OFL fetch/parse, and app relaunch semantics).

The frontend has one real, unaddressed correctness gap: `FixtureLibraryWorkspace.tsx`'s catalog-preview and row-inspect handlers have no request-generation guard, unlike the catalog *search* debounce a few lines above them (which correctly uses `catalogRequestRef`). A stale in-flight `PreviewOFL`/`Inspect` response can silently overwrite state after the operator has already moved on to a different candidate, and the "Fixture key" field is not disabled while a preview is in flight (unlike the otherwise-identical custom-fixture path field), which widens the window for this race. There is also duplicated JSX/logic between the shared `renderCandidateBody` and the local-library inline inspect block, and a duplicated `readSurfaceCount` helper across two stage components.

## Warnings

### WR-01: Stale async responses can overwrite newer selection state in FixtureLibraryWorkspace

**File:** `frontend/src/workspaces/build/FixtureLibraryWorkspace.tsx:196-334`
**Issue:** `handleSelectRow` (row inspect), `handlePreview` (OFL candidate preview), and `handleValidateCustomFixture` (custom-fixture preview) each fire an async bridge call and then unconditionally call `setInspectView`/`setPreviewView` when it resolves, with no check that the operator hasn't since selected something else. This is the same class of race the catalog *search* effect at line ~163 already guards against with `catalogRequestRef`/`requestId` — but that guard was not applied to these three handlers.

Concretely: an operator selects manufacturer A, clicks **Preview**, then (before the network call resolves) selects manufacturer B via `handleSelectManufacturer` (which calls `resetCandidate()`, clearing `previewView`). When A's `PreviewOFL` promise resolves after B is selected, `setPreviewView(view)` fires anyway and silently redisplays A's candidate (including its `previewToken`) underneath B's name/fixture-key field. If the operator then clicks **Add to Library**, `handleCommit` commits `previewView.previewToken` — A's staged artifact — even though the visible manufacturer name is B's. The identical issue applies to rapid double-row-clicks in the local library list (`handleSelectRow`) and to rapid edits of the custom-fixture path before `PreviewFile` resolves (partially mitigated there — see WR-02).

**Fix:** Add a monotonically increasing request id (mirroring `catalogRequestRef`) captured before each async call and checked before applying its result, e.g.:
```tsx
const previewRequestRef = useRef(0);

const handlePreview = () => {
  if (!selectedManufacturer) return;
  const key = candidateFixtureKey.trim();
  if (key === "") return;
  const requestId = ++previewRequestRef.current;
  setPreviewing(true);
  setPreviewError(null);
  setAlreadyExists(false);
  void (async () => {
    const view = await previewOflFixture(selectedManufacturer.key, key);
    if (previewRequestRef.current !== requestId) return; // superseded
    setPreviewView(view);
    setPreviewing(false);
  })();
};
```
Apply the same pattern to `handleValidateCustomFixture` and `handleSelectRow` (a shared ref per "candidate" concept, since they are mutually exclusive on screen, would work).

### WR-02: "Fixture key" field stays editable during an in-flight preview, unlike the custom-fixture path field

**File:** `frontend/src/workspaces/build/FixtureLibraryWorkspace.tsx:610-622`
**Issue:** The custom-fixture flow correctly disables its path `Field` while `previewing` is true (line 535: `disabled={previewing}`), preventing the operator from changing the input mid-flight. The OFL catalog flow's "Fixture key" `Field` has no such `disabled` prop, so the operator can keep typing (each keystroke calls `handleFixtureKeyChange` → `resetCandidate()`) while the previous `PreviewOFL` call is still in flight — widening the window for WR-01's race and creating a visibly inconsistent interaction model between the two "preview a candidate" flows that otherwise share every other piece of UI (`renderCandidateBody`).
**Fix:** Add `disabled={previewing}` to the "Fixture key" `Field` (and consider disabling the manufacturer list rows while `previewing` too), matching the custom-fixture path's existing discipline.

### WR-03: Near-duplicate inspect rendering between `renderCandidateBody` and the local-library inline inspect block

**File:** `frontend/src/workspaces/build/FixtureLibraryWorkspace.tsx:341-391` (`renderCandidateBody`) vs. `frontend/src/workspaces/build/FixtureLibraryWorkspace.tsx:558-596` (inline `inspectView` rendering)
**Issue:** The two blocks render nearly identical markup (tech readout line, schema/revision line, valid/invalid chip, the same "{N} error(s)" and "{N} unsupported…attribute(s)" copy, the same diagnostic list) — one operating on `view.inspect`, the other on `inspectView` directly. `renderCandidateBody` was explicitly introduced (per its own doc comment) to be "the SAME candidate-preview rendering both the OFL catalog path and the custom-fixture path render through… never duplicated per source" — but the local-library selected-row inspect view was left as a third, hand-duplicated copy of the same rendering, which can silently drift from the shared one (e.g. a future copy/wording change applied to `renderCandidateBody` but missed here).
**Fix:** Extract a `renderInspectBody(view: FixtureInspectView)` helper (or accept a bare `FixtureInspectView` in `renderCandidateBody` alongside the existing wrapper for `FixturePreviewView`) and call it from both places.

### WR-04: `readSurfaceCount` duplicated verbatim across two stage components

**File:** `frontend/src/workspaces/show/GuidedFirstShow/stages/AssignStage.tsx:26-33` and `frontend/src/workspaces/show/GuidedFirstShow/stages/VerifyStage.tsx:49-56`
**Issue:** Both files independently declare the identical `readSurfaceCount()` helper (same body, same doc-comment wording acknowledging the duplication: "mirrors AssignStage.tsx's own identical helper"). `wailsBridge.ts` already documents a "cast-through escape hatch" convention for services without a dedicated helper (used by both call sites) specifically so this kind of ad hoc access has one home; this helper itself was not centralized there, so a future change to the escape-hatch shape (or to the fallback-to-zero behavior) requires editing two files in lockstep with no compiler check that both were updated.
**Fix:** Move `readSurfaceCount` into `wailsBridge.ts` (e.g. `export async function surfaceCount(): Promise<number> { ... }`) and import it from both stage components.

## Info

### IN-01: Row/preview path built with a literal `/` separator instead of a path-join helper

**File:** `frontend/src/workspaces/build/FixtureLibraryWorkspace.tsx:200`
**Issue:** `const path = directory ? \`${directory}/${row.fileName}\` : row.fileName;` hardcodes a forward slash rather than using a path-join utility. This happens to work today (Windows accepts `/` as a separator, and `directory` never carries a trailing slash per `fixtureLibraryDirectoryLabel`'s doc contract), but it's inconsistent with the rest of the codebase's path handling and would silently produce a doubled/missing separator if that invariant ever changes.
**Fix:** Extract a tiny `joinLibraryPath(directory, fileName)` helper (even a one-liner) so the "how do we join a library-relative path" decision has one place, mirroring the pattern `wailsBridge.ts` already uses for shared cross-cutting helpers.

### IN-02: `parseFixtureImportArgs`/route usage strings are duplicated across two constants in `runFixtureImport`

**File:** `internal/command/fixture.go:319-321`
**Issue:** The `usage` string literal for `fixture import` is declared once in `runFixtureImport` but its exact wording is also independently duplicated in the route's `Summary` registration text (lines 51-54). Not a functional bug, but a future flag addition to `parseFixtureImportArgs` requires remembering to update both copies, since nothing ties them together.
**Fix:** Low priority — consider deriving one from the other, or leave as-is if this mirrors an established repo-wide convention (several other route files in this package follow the same pattern).

### IN-03: `FixtureLibraryWorkspace.test.tsx` never exercises the WR-01/WR-02 race directly

**File:** `frontend/src/workspaces/build/FixtureLibraryWorkspace.test.tsx`
**Issue:** The test suite is otherwise thorough (search, catalog toggle, preview/commit, custom-fixture add, disabled-during-validation states) but has no test that resolves a `PreviewOFL`/`Inspect` promise out of order relative to a subsequent selection change — the exact scenario WR-01 describes. The existing "field disabled while validation runs" test (line 619) proves the custom-fixture path's mitigation but doesn't prove the OFL catalog path lacks the same protection.
**Fix:** Add a regression test using a manually-resolved `Promise` (the pattern already used at line 620) that selects manufacturer A, triggers Preview, switches to manufacturer B before resolving A's promise, then resolves A's promise and asserts A's preview is not shown under B's selection.

---

_Reviewed: 2026-07-28T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
