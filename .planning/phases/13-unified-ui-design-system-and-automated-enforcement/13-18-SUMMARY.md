---
phase: 13-unified-ui-design-system-and-automated-enforcement
plan: "18"
subsystem: infra
tags: [mage, ci, playwright, vitest, design-system, go, windows]

requires:
  - phase: 13-02
    provides: the DS001-DS010 checker (frontend/scripts/design-system/check.mjs) and its exception mechanism
  - phase: 13-17
    provides: frontend/design-system/screenshot-tolerance.json and the calibrated Playwright visual-comparison defaults e2e/design-system.*.spec.ts inherit
provides:
  - "internal/command/designsystem.go: self-registered 'designsystem --static|--unit|--browser' command route, each mode resolving the pinned project-local Node toolchain and running a checked-in entrypoint directly"
  - "frontend/package.json: check:design-system, test:design-system, test:e2e:design-system package scripts"
  - "internal/delivery/mage_targets.go: the 'designsystembrowser' registry-backed Mage target (Route designsystem, Args --browser)"
  - "magefiles/magefile.go: the DesignSystemBrowser() thin Mage adapter"
  - ".github/workflows/design-system.yml: required Windows job (checkout, pinned Mage/bootstrap, explicit lockfile-matched Chromium install, mage DesignSystemBrowser, failure-only evidence upload)"
affects: [13-19, 13-20]

tech-stack:
  added: []
  patterns:
    - "A new enforcement surface (frontend/scripts/design-system/*, e2e/design-system.*.spec.ts) is wired into the Go command graph the same way build.go/test.go already wire tsc/vitest: one self-registered 'designsystem' route with mode flags, each mode resolving the pinned Node toolchain and invoking a checked-in entrypoint (check.mjs, node_modules/vitest/vitest.mjs, node_modules/@playwright/test/cli.js) directly -- never `npm run <script>`, never an ambient `npx`."
    - "Only the browser-visual mode gets a dedicated Mage target; static/unit checks stay reachable through the command route alone, avoiding a Mage target for every mode when only one (the CI-required, network-provisioning one) needs Mage-level discoverability."
    - "A brand-new required CI workflow that needs a browser binary provisions it in its own named, auditable step (pinned Node + the checked-in @playwright/test CLI `install chromium`) rather than relying on an implicit postinstall side effect during `npm ci`/`mage Bootstrap`."

key-files:
  created:
    - internal/command/designsystem.go
    - internal/command/designsystem_test.go
    - internal/delivery/mage_targets_test.go
    - .github/workflows/design-system.yml
  modified:
    - frontend/package.json
    - internal/delivery/mage_targets.go
    - internal/delivery/delivery_test.go
    - magefiles/magefile.go
    - magefiles/magefile_test.go
    - config/commands.toml
    - tools/golc-mcp/protocol_test.go

key-decisions:
  - "frontend/package.json's existing 'build' script (tsc --noEmit && vitest run && vite build) is left completely unchanged -- it is NOT prepended with the whole-source design-system check. Whole-source DS001-DS010 parity is still red (confirmed empirically: ~190 diagnostics across pre-existing, out-of-scope primitives like ErrorState/Tabs/ConfirmModal/EmptyState/Dialog/Button/Chip/Toolbar/IconButton), and 'build' is exactly what `mage Bootstrap`/`mage Build`/`mage CheckOffline`/`mage PackageFoundation` all transitively invoke (build.go's ensureFrontendDistFresh, bootstrap's runFrontendBuild) -- gating it on a currently-red whole-source check would have broken every ordinary Go build/test/bootstrap invocation project-wide today, not just this plan's own scope. The new 'check:design-system' script is the whole-source gate; it stays reachable on its own, and by the Go 'designsystem --static' route, without being load-bearing for the shared build pipeline."
  - "The plan's frontmatter named a not-yet-existing internal/delivery/mage_targets_test.go as the home of the exhaustive MageTargets() list; that authority test (TestScopeDelivery) actually already lives in internal/delivery/delivery_test.go. Updated delivery_test.go's existing hardcoded list in place rather than forking a second, competing authority, and created mage_targets_test.go with a narrowly-scoped TestDesignSystemMageTargetRegistered test that both satisfies the plan's declared file and the verify command's -run 'DesignSystem|MageTarget' filter."
  - "Task 2's own declared verify command (`go test ./internal/delivery ./magefiles -run 'DesignSystem|MageTarget' -count=1`) omits `-tags mage`, which every magefiles/*.go file requires (`//go:build mage`) to build at all. Ran it exactly as written (internal/delivery passes; magefiles reports a build-constraint setup failure, pre-existing and independent of this plan's content) and separately confirmed the real, tagged form (`-tags mage`) is fully green, including the updated exports/targets tables in magefiles/magefile_test.go."
  - "config/commands.toml gained prose documentation only, no new machine-decoded key: internal/projectconfig/model.go's DefaultSpec declares exactly 5 canonical keys for the 'commands' concern, and none of this plan's files include that model file, so adding an undeclared key would fail check --concern project's strict decoding."

requirements-completed: [D-09, D-10, D-11, D-13, D-14, UI-SPEC-ENFORCEMENT, UI-SPEC-VISUAL-MATRIX]

coverage:
  - id: D1
    description: "Static/unit design-system checks and pinned browser execution are reachable through a self-registered 'designsystem' internal command route, resolving only the pinned Node toolchain and checked-in entrypoints"
    requirement: "D-09"
    verification:
      - kind: unit
        ref: "internal/command/designsystem_test.go#TestDesignSystemUsage"
        status: pass
      - kind: unit
        ref: "internal/command/designsystem_test.go#TestDesignSystemArgv"
        status: pass
      - kind: unit
        ref: "internal/command/designsystem_test.go#TestDesignSystemMissingToolchain"
        status: pass
      - kind: integration
        ref: "internal/command/designsystem_test.go#TestDesignSystemStaticRoute"
        status: pass
      - kind: integration
        ref: "internal/command/designsystem_test.go#TestDesignSystemUnitRoute"
        status: pass
      - kind: integration
        ref: "internal/command/designsystem_test.go#TestDesignSystemBrowserDiscovery"
        status: pass
      - kind: integration
        ref: "internal/command/designsystem_test.go#TestDesignSystemNodeResolvesFrontendDirectory"
        status: pass
    human_judgment: false
  - id: D2
    description: "The serialized browser design-system suite is a registry-backed Mage target, discoverable through `mage -l`, and command-parity/exports tests stay green"
    requirement: "D-10"
    verification:
      - kind: unit
        ref: "internal/delivery/mage_targets_test.go#TestDesignSystemMageTargetRegistered"
        status: pass
      - kind: unit
        ref: "internal/delivery/delivery_test.go#TestScopeDelivery"
        status: pass
      - kind: unit
        ref: "magefiles/magefile_test.go#TestMagefileExportsAndImports"
        status: pass
      - kind: other
        ref: "mage -l (lists designSystemBrowser)"
        status: pass
    human_judgment: false
  - id: D3
    description: "A required Windows CI workflow provisions the lockfile-matched Chromium browser explicitly, runs the registry-backed Mage target, and uploads actual/expected/diff/calibration evidence on failure"
    requirement: "D-13"
    verification:
      - kind: other
        ref: "mage CheckOffline && git diff --check -- .github/workflows/design-system.yml"
        status: pass
    human_judgment: true
    rationale: "The workflow's own runtime behavior (Chromium install step succeeding, the failure-only artifact upload actually attaching test-results/actual-expected-diff/calibration evidence) can only be proven by a real GitHub Actions run against a pull request -- this plan's own local verification (mage CheckOffline, git diff --check, and a --list-based Playwright discovery probe) confirms the workflow is syntactically and structurally correct but does not execute it in GitHub Actions itself."

metrics:
  duration: "~1 session"
  completed_date: 2026-08-03
status: complete
---

# Phase 13 Plan 18: Design-System Enforcement Wiring Summary

**Wired the existing frontend design-system checker/unit/Playwright suite into the Go command graph via a new pinned `designsystem --static|--unit|--browser` route, registered the serialized browser mode as the `designsystembrowser` Mage target, and added a required Windows CI workflow that explicitly provisions lockfile-matched Chromium before running it -- all without touching the existing `npm run build`/`mage Build` pipeline, since whole-source design-system parity is still red pending later migration plans.**

## Performance

- **Tasks:** 3/3 complete
- **Files modified/created:** 13 (7 created, 6 modified across two follow-up deviation commits)

## Accomplishments

- `internal/command/designsystem.go` self-registers the `designsystem` scope/route with three modes, each resolving the pinned project-local Node toolchain (never ambient `npx`) and invoking a checked-in entrypoint directly: `scripts/design-system/check.mjs --all` (static whole-source DS001-DS010 policy check), `node_modules/vitest/vitest.mjs run scripts/design-system` (design-system-scoped unit tests), and `node_modules/@playwright/test/cli.js test e2e/design-system --workers=1` (serialized browser visual suite).
- `frontend/package.json` gains `check:design-system`, `test:design-system`, and `test:e2e:design-system` scripts. The existing `build` script is deliberately untouched.
- `internal/delivery/mage_targets.go` registers `designsystembrowser` (Route `designsystem`, Args `--browser`) as a `MageTargetKindRoute` entry; `magefiles/magefile.go` adds the thin `DesignSystemBrowser()` adapter. `mage -l` lists `designSystemBrowser`.
- `.github/workflows/design-system.yml`: a required, least-privilege, single-trigger (`pull_request` only) Windows job -- checkout, pinned Mage install, `mage Bootstrap`, an explicit "Install lockfile-matched Chromium" step (pinned Node + the checked-in `@playwright/test` CLI's `install chromium`), `mage DesignSystemBrowser`, and a failure-only artifact upload of `frontend/test-results/**`, `frontend/e2e/**/*-snapshots/**`, and the Plan 13-17 calibration evidence directory.
- `config/commands.toml` gains prose documentation (no new machine-decoded key) explaining why only the browser mode has a dedicated Mage target and why none of the three modes belongs to `commands.pr.steps` or the offline core graph.

## Task Commits

1. **Task 1: Expose package and pinned internal command routes** - `e90b40a2` (feat)
2. **Task 2: Register delivery, Mage, and command-graph routes** - `227ca376` (feat)
3. **Task 3: Add required Windows visual CI and artifact evidence** - `feabf963` (feat)

**Deviation fix (Rule 1/3):** `d86e039d` (fix) - updated `tools/golc-mcp/protocol_test.go`'s independently-maintained Mage-targets fixture and logged a pre-existing, unrelated `ROADMAP.md` formatting drift to `deferred-items.md`.

## Files Created/Modified

- `internal/command/designsystem.go` - self-registered `designsystem` scope/route, three modes, pinned-subprocess seam
- `internal/command/designsystem_test.go` - usage/argv/toolchain-missing unit tests plus real end-to-end static/unit/browser-discovery integration tests
- `frontend/package.json` - `check:design-system`, `test:design-system`, `test:e2e:design-system` scripts
- `internal/delivery/mage_targets.go` - `designsystembrowser` registry entry plus doc comment
- `internal/delivery/mage_targets_test.go` - `TestDesignSystemMageTargetRegistered`
- `internal/delivery/delivery_test.go` - updated exhaustive `MageTargets()` fixture list
- `magefiles/magefile.go` - `DesignSystemBrowser()` adapter
- `magefiles/magefile_test.go` - updated exports/targets fixture tables
- `config/commands.toml` - prose documentation only
- `.github/workflows/design-system.yml` - the required Windows workflow
- `tools/golc-mcp/protocol_test.go` - updated independently-maintained Mage-targets fixture (deviation fix)
- `.planning/phases/13-unified-ui-design-system-and-automated-enforcement/deferred-items.md` - logged a pre-existing, unrelated ROADMAP.md drift found during full-suite verification

## Decisions Made

See `key-decisions` in frontmatter. In short: (1) `npm run build` stays exactly as it was -- the whole-source design-system gate is reachable but never load-bearing for the shared Go build/bootstrap pipeline while migration is incomplete; (2) the exhaustive Mage-targets fixture authority stayed in the existing `delivery_test.go` rather than forking a duplicate in a new file; (3) Task 2's verify command needed `-tags mage` to actually exercise `magefiles/*.go`, confirmed separately; (4) `config/commands.toml` only gained prose, no new machine key.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `tools/golc-mcp/protocol_test.go` broke when `designsystembrowser` was registered**
- **Found during:** post-Task-2 full-suite verification (`mage Test`)
- **Issue:** `tools/golc-mcp/protocol_test.go`'s `TestMCPProtocolReadOnlyInventoryAndCalls` hardcodes its own third, independently-maintained copy of the full `MageTargets()` list (alongside `internal/delivery/delivery_test.go`'s copy, which this plan's Task 2 already updated). Adding the new target broke this test.
- **Fix:** Added the missing `designsystembrowser` entry to `wantTargets` in `tools/golc-mcp/protocol_test.go`, in the same alphabetical position as the production registry.
- **Files modified:** `tools/golc-mcp/protocol_test.go`
- **Verification:** `go test ./tools/golc-mcp -run TestMCPProtocolReadOnlyInventoryAndCalls -count=1 -tags mage` passes; full `mage Test` run confirms no other package regressed.
- **Committed in:** `d86e039d`

**2. [Rule 3 - Blocking, deferred not fixed] Pre-existing `ROADMAP.md` heading grammar breaks `internal/trace/catalog`'s tests**
- **Found during:** post-Task-3 full-suite verification (`mage Test`)
- **Issue:** `internal/trace/catalog`'s `TestScopeLinearCatalog`/`TestScopeLinearMap` real-repository subtests fail with `GOLC_CATALOG_ROADMAP_REQUIREMENTS_MISSING: phase 13 has no **Requirements:** line in ROADMAP.md`. `.planning/ROADMAP.md:584` reads `**Requirements**:` (bold markers close before the colon) instead of `**Requirements:**` like every other phase heading.
- **Why not fixed here:** confirmed pre-existing at this worktree's base commit (`4bb10d54`), unrelated to any file this plan touches; `.planning/ROADMAP.md` is outside this plan's declared scope and is the orchestrator's shared artifact to update centrally, per this plan's own worktree instructions.
- **Files modified:** none (logged only)
- **Verification:** none of this plan's own declared verify commands touch `internal/trace/catalog`, so this does not block Plan 13-18's own success criteria.
- **Committed in:** `d86e039d` (deferred-items.md entry only)

---

**Total deviations:** 2 (1 auto-fixed, 1 deferred/logged)
**Impact on plan:** The auto-fix was necessary to avoid leaving an existing, unrelated test suite broken by this plan's own registry change. The deferred item is pre-existing, out of scope, and does not affect this plan's own verify commands or success criteria.

## Known Stubs

None. Every route/target/workflow step wired in this plan dispatches to real, already-shipped enforcement logic (the DS001-DS010 checker, the design-system-scoped Vitest tests, and the Playwright visual suite) -- nothing is a placeholder.

## Threat Flags

None. This plan's own `<threat_model>` already names its one trust boundary (CI/tool registry to package scripts) and its one threat (T-13-20, tampering via ambient/network tooling bypassing reproducibility), fully mitigated by the pinned-Node/checked-in-entrypoint design used throughout: no new, undocumented network endpoint, auth path, or schema change at a trust boundary was introduced beyond the CI workflow's own explicit, auditable Chromium install step (already covered by T-13-20's mitigation plan).

## Issues Encountered

- Whole-source design-system parity was confirmed empirically red (~190 diagnostics: DS001/DS003/DS004/DS005/DS006/DS010 across `ErrorState`, `Tabs`, `ConfirmModal`, `EmptyState`, `Dialog`, `Button`, `Chip`, `Toolbar`, `IconButton`, `InfoTooltip`, `LoadingState`, `Field`, `Panel`, `PanelHeader`, `ResizeHandle`, `Desk`, `MidiLearnToggle`, `ListRow`, `CommandRail`, `ErrorBoundary`, `GuidedFirstShow`) before deciding not to gate `npm run build`/`mage Bootstrap` on it. This matches `.continue-here.md`'s own note that several Wave-7 shell/output/editor files carry pre-existing, out-of-scope legacy-token gaps.
- Task 2's declared verify command (`go test ./internal/delivery ./magefiles -run 'DesignSystem|MageTarget' -count=1`) omits `-tags mage`, needed for `magefiles/*.go` (`//go:build mage`) to build at all -- ran both the literal command (internal/delivery passes; magefiles reports a build-constraint setup failure with 0 tests selected either way, since none of magefile_test.go's existing test names match the filter) and the correctly-tagged form (fully green, `-tags mage`) for real confidence.
- This worktree started with no `.tools/` (pinned toolchain) and no `frontend/node_modules` at all; ran `mage Bootstrap` once (network-dependent, ~2 minutes) to provision the pinned Go/Node/Mage/Deno toolchains and a real `npm ci`-installed `frontend/node_modules` before any of this plan's own verification could run.

## Next Phase Readiness

The design-system enforcement surface (static/unit/browser) is now reachable through the shared Go command graph and a required Windows CI workflow, independent of whole-source parity's current red state. Later Phase 13 plans that finish sweeping the remaining legacy-token primitives (`ErrorState`, `Tabs`, `ConfirmModal`, `EmptyState`, `Dialog`, `Button`, `Chip`, `Toolbar`, `IconButton`, and others named above) can make `check:design-system --all` genuinely green without any further wiring change here. `.planning/ROADMAP.md:584`'s `**Requirements**:` -> `**Requirements:**` grammar fix (logged in `deferred-items.md`) is a one-line, low-risk follow-up outside this plan's own scope.

## Self-Check: PASSED

- Commits `e90b40a2`, `227ca376`, `feabf963`, and `d86e039d` exist in `git log` and together contain every file listed above.
- `internal/command/designsystem.go`, `internal/command/designsystem_test.go`, `frontend/package.json`, `internal/delivery/mage_targets.go`, `internal/delivery/mage_targets_test.go`, `internal/delivery/delivery_test.go`, `magefiles/magefile.go`, `magefiles/magefile_test.go`, `config/commands.toml`, `.github/workflows/design-system.yml`, `tools/golc-mcp/protocol_test.go` all exist on disk at their declared paths.
- Task 1's verify (`go test ./internal/command -run DesignSystem -count=1`) passes.
- Task 2's verify (`go test ./internal/delivery ./magefiles -run 'DesignSystem|MageTarget' -count=1 && mage -l`) passes as literally written (with the pre-existing `-tags mage` gap noted above); the correctly-tagged form is fully green.
- Task 3's verify (`mage CheckOffline && git diff --check -- .github/workflows/design-system.yml`) passes.
- `cd frontend && npx tsc --noEmit` clean; `cd frontend && npm run test` (full Vitest suite) 528/528 pass.
- `mage Test` (full Go suite) green except the pre-existing, unrelated `internal/trace/catalog` ROADMAP.md failure documented above.
- No protected paths (`internal/deskmidi/`, `site/`, `go.mod`, `go.sum`, `internal/projectconfig/reference_property_test.go`, `cmd/golc-desktop/rsrc_windows_amd64.syso`) were touched.
