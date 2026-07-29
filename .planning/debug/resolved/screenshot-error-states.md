---
status: resolved
trigger: "several of the screenshots show errors. that isnt desired"
created: 2026-07-29T20:00:00Z
updated: 2026-07-29T21:52:00Z
---

## Current Focus

hypothesis: The screenshot test's missing Wails initialization is the single cause of the published error states: plain Chromium has no window.go/window.runtime, and each frontend fallback deliberately maps that absence to disconnected/error UI.
test: Ask the user to confirm the regenerated screenshots are acceptable in the real documentation workflow.
expecting: User confirms the production documentation screenshots show the intended healthy representative states.
next_action: Complete — human verification confirmed the regenerated screenshots are fixed.
reasoning_checkpoint:
  hypothesis: "Plain Chromium capture lacks window.go/window.runtime, causing every service adapter to select its intentional offline/error fallback."
  confirming_evidence:
    - "All twelve PNGs show the shared unreachable-engine fallback, while service-specific views show their own bridge-unavailable projections."
    - "desktop-view-docs.spec.ts navigates directly to Vite without addInitScript or a Wails host, and wailsBridge.ts explicitly returns those exact visible messages when services are absent."
  falsification_test: "If installing healthy bindings before page.goto leaves any known disconnected/bridge error visible, missing initialization is not sufficient and the affected workspace has another cause."
  fix_rationale: "The capture harness, not production UI, owns documentation state; supplying deterministic healthy bindings at that boundary prevents expected browser fallbacks from being published without weakening real error reporting."
  blind_spots: "The fixture may omit a service method invoked asynchronously after initial navigation, and pixel review is still needed after automated text assertions pass."
tdd_checkpoint: null

## Symptoms

expected: All twelve documentation screenshots show healthy, deterministic, representative desktop app states without errors.
actual: Several published desktop-view screenshots visibly contain error states or messages.
errors: Exact visible messages not yet catalogued.
reproduction: Visit the production /docs/desktop-views page, select each view and open its screenshot, or inspect site/public/desktop-views/*.png.
started: Current screenshot set generated during quick task 260729-gq6 and present in production.

## Eliminated

No hypotheses eliminated yet.

## Evidence

- timestamp: 2026-07-29T21:05:00Z
  checked: Initial repository inventory and debug knowledge base
  found: Twelve committed desktop-view PNGs exist under site/public/desktop-views; no project debug knowledge base exists and no project-local skills were found.
  implication: The investigation must derive the cause from the current screenshot/capture implementation rather than a known project pattern.
- timestamp: 2026-07-29T21:10:00Z
  checked: Visual inspection of all twelve committed PNGs
  found: Every screenshot shows the global "Can't reach the playback engine" warning; Scripts additionally shows "Can't reach the script host"; Operator Surface shows GOLC_WAILS_BINDING_UNAVAILABLE for SurfaceService; Art-Net renders OFFLINE/no interfaces; Diagnostics renders ISSUES FOUND with GOLC_WAILS_BRIDGE_UNAVAILABLE.
  implication: The errors span shared shell state plus workspace-specific bindings, consistent with capture running the browser-safe disconnected fallback rather than a healthy deterministic documentation backend.
- timestamp: 2026-07-29T21:18:00Z
  checked: frontend/e2e/desktop-view-docs.spec.ts and frontend/src/lib/wailsBridge.ts
  found: The screenshot test calls page.goto("/") with no page.addInitScript, route interception, or Wails host; wailsBridge explicitly returns reachable:false, bridge-unavailable diagnostic reports, and failed/offline projections whenever window.go services are absent.
  implication: The application is behaving as designed; the documentation capture harness fails to provide the healthy deterministic host state its output requires.
- timestamp: 2026-07-29T21:38:00Z
  checked: Fixed documentation screenshot capture and visual inspection of all twelve regenerated PNGs
  found: Playwright passed 1/1; every screenshot is free of unreachable-engine/script-host and Wails bridge errors, Art-Net shows a healthy pinned Ethernet interface, and Diagnostics shows Healthy.
  implication: Supplying healthy bindings at the capture boundary is sufficient to remove the original error states across all destinations.
- timestamp: 2026-07-29T21:46:00Z
  checked: Full frontend regression gate
  found: npm run build passed TypeScript checking, all 45 test files and 274 tests, and the Vite production build.
  implication: The documentation-only fixture compiles cleanly and does not regress frontend behavior.
- timestamp: 2026-07-29T21:52:00Z
  checked: Consecutive screenshot capture determinism
  found: A second Playwright capture passed and all twelve PNG SHA-256 hashes were byte-identical to the first fixed run.
  implication: The healthy documentation fixture produces stable reproducible artifacts rather than timing-dependent screenshots.
- timestamp: 2026-07-29T21:53:00Z
  checked: Human verification checkpoint
  found: The user confirmed the regenerated documentation screenshots are fixed.
  implication: The session can be resolved and archived with the capture harness and screenshot artifacts committed.

## Resolution

root_cause: frontend/e2e/desktop-view-docs.spec.ts launched the ordinary Vite frontend in plain Chromium without the Wails-injected window.go/window.runtime services. The frontend correctly mapped those missing bindings to explicit offline, bridge-unavailable, and unhealthy diagnostic states, which the capture harness then published verbatim.
fix: Added a documentation-only healthy Wails binding fixture before navigation and per-destination assertions that fail capture if known disconnected, bridge-unavailable, issues-found, or offline indicators are visible.
verification: Exact screenshot capture passed twice; all twelve regenerated PNGs were visually inspected and contain no error states; consecutive-run SHA-256 hashes were identical; the full frontend build completed with 274/274 tests passing.
files_changed:
  - frontend/e2e/desktop-view-docs.spec.ts
  - site/public/desktop-views/*.png
