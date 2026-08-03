---
phase: 13-unified-ui-design-system-and-automated-enforcement
plan: "06"
subsystem: ui-testing
tags: [react, playwright, webview2, wails, dialogs]
requires:
  - phase: 13-22
    provides: Dialog and ConfirmDialog primitive foundation
provides:
  - Chromium dialog feasibility contract and deterministic fixture
  - WebView2 CDP proof harness with machine-readable evidence
affects: [dialog migration, desktop qualification]
tech-stack:
  added: []
  patterns: ["A single Playwright contract attaches through CDP when GOLC_WEBVIEW2_CDP_ENDPOINT is supplied."]
key-files:
  created:
    - frontend/e2e/dialog-feasibility.spec.ts
    - frontend/e2e/fixtures/dialogFeasibility.ts
    - frontend/src/design-system/fixtures/DialogFeasibility.tsx
    - scripts/ci/run-packaged-dialog-proof.ps1
  modified:
    - frontend/src/App.tsx
    - frontend/src/components/primitives/Dialog/Dialog.tsx
    - frontend/src/components/primitives/Dialog/Dialog.module.css
key-decisions:
  - "Use an API-compatible custom backdrop foundation because the native open-attribute dialog could not receive a real backdrop interaction."
  - "Keep packaged evidence failed until WebView2 CDP is actually reachable; do not infer packaged compatibility from Chromium."
patterns-established:
  - "Test-only fixtures mount through an explicit e2e query route and never alter normal operator routes."
requirements-completed: []
coverage:
  - id: D1
    description: Chromium dialog focus, dismissal, portal, and safety-control contract
    verification:
      - kind: e2e
        ref: frontend/e2e/dialog-feasibility.spec.ts#keeps focus, dismissal, portals, and safety controls usable
        status: pass
    human_judgment: false
  - id: D2
    description: Packaged WebView2 dialog contract
    verification:
      - kind: automated_ui
        ref: pwsh -NoProfile -File scripts/ci/run-packaged-dialog-proof.ps1
        status: fail
    human_judgment: true
    rationale: "The spawned packaged application never exposed its CDP endpoint, so the identical WebView2 assertions did not run."
metrics:
  duration: 19min
  completed_date: 2026-08-03
status: blocked
---

# Phase 13 Plan 06: Dialog Feasibility Summary

**Chromium-proven dialog contract with an API-compatible backdrop implementation and an honest, currently blocked packaged-WebView2 proof harness.**

## Performance

- **Tasks:** 1/2 complete
- **Chromium:** passed
- **Packaged WebView2:** blocked before CDP attachment

## Accomplishments

- Added a deterministic fixture that proves safe initial focus, focus containment, Escape, allowed/blocked backdrop dismissal, portal visibility, focus return, and persistent safety controls.
- Replaced the private native open-attribute shell with a portal-backed focus-managed dialog foundation without changing the public Dialog or ConfirmDialog API.
- Added a Mage-build, WebView2-CDP harness that records executable hash, endpoint, browser metadata, test result, and failure state.

## Task Commits

1. **Task 1: Prove Dialog in real Chromium** — `774b9d5e` (RED) and `3b326942` (GREEN)
2. **Task 2: Prove identical contract in packaged WebView2** — `55e15944` (harness; evidence updated in the final metadata commit)

## Verification

- `npx playwright test e2e/dialog-feasibility.spec.ts --project=chromium --workers=1` — passed.
- Focused Dialog and ConfirmDialog Vitest suites — passed.
- `powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts/ci/run-packaged-dialog-proof.ps1` — Mage build passed, then WebView2 CDP timed out after 45 seconds. `pwsh` is unavailable in this environment.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Isolated the fixture route from unrelated shell parsing**
- **Found during:** Task 1
- **Issue:** A pre-existing Panel stylesheet syntax error prevented the normal shell import from parsing before the fixture could run.
- **Fix:** Lazily load AppShell so the explicit test-only route mounts only its fixture.
- **Verification:** Chromium proof passed.
- **Committed in:** `3b326942`

**2. [Rule 1 - Bug] Replaced the non-modal native dialog shell**
- **Found during:** Task 1
- **Issue:** The native `open` attribute implementation could not receive an actual backdrop click.
- **Fix:** Used a portal-backed backdrop while retaining the existing Dialog/ConfirmDialog API and focus contract.
- **Verification:** Chromium and focused primitive tests passed.
- **Committed in:** `3b326942`

**3. [Rule 3 - Blocking] Added isolated WebView2 user-data setup**
- **Found during:** Task 2
- **Issue:** The initial CDP launch omitted the documented isolated profile setup.
- **Fix:** Added `WEBVIEW2_USER_DATA_FOLDER`, cleanup, and evidence metadata.
- **Verification:** The second run still timed out, establishing an external packaged-runtime blocker rather than a missing harness prerequisite.

## Known Stubs

None.

## Issues Encountered

- Packaged `golc-desktop.exe` builds successfully but never serves `http://127.0.0.1:19226/json/version` within 45 seconds, even with dedicated WebView2 debugging arguments and user-data folder. The packaged assertion suite therefore has not run and dialog migration remains blocked.

## Next Phase Readiness

Chromium evidence is green. Resolve the WebView2 CDP launch/availability issue, then rerun the packaged proof until `dialog-feasibility.json` records `status: passed` before relying on this dialog foundation for broad migration.
