---
phase: 06-wails-authoring-and-operator-surface
plan: 11
subsystem: ui
tags: [wails, react, go, artnet, ipc, css-modules]

# Dependency graph
requires:
  - phase: 04-observable-art-net-live-output
    provides: "artnet interface list / artnet configure / artnet target enable|disable / artnet status CLI routes (internal/command/artnet.go), the supervised long-lived Art-Net daemon (internal/artnet, internal/artnet/ipc), and artnet.ValidateTarget's validate-before-forward contract (T-04-07)"
  - phase: 06-wails-authoring-and-operator-surface (06-04, 06-05, 06-10)
    provides: "Wails Go-host scaffold (internal/wails, cmd/golc-desktop), svc_playback.go's in-process command-registry execute pattern, svc_safety.go's daemon-JSON-mirroring/offline-projection discipline, and wailsBridge.ts's shared Window.go.wails ambient declaration (already extended by 06-10 with FixturePatchService)"
provides:
  - "internal/wails.ArtnetConfigService: ListInterfaces, Configure, EnableTarget, DisableTarget, FetchArtnetStatus"
  - "frontend ArtnetConfig component (on-screen deployment-interface + Art-Net universe/target configuration surface)"
  - "wailsBridge.ts ArtnetConfigService ambient binding + exported helpers (listArtnetInterfaces/configureArtnetTarget/enableArtnetTarget/disableArtnetTarget/fetchArtnetStatus/offlineArtnetStatus)"
affects: ["06-wails-authoring-and-operator-surface end-of-phase UAT", "VERIFICATION.md Gap B[0] closure", "PLAY-11"]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "UI-binding-only Wails service dispatched through the in-process command registry (svc_playback.go's execute pattern, not svc_safety.go's direct daemon dial): ArtnetConfigService builds command.NewDefaultCommandRegistry and runs 'artnet ...' route args, so internal/command/artnet.go's own artnet.ValidateTarget-before-forward discipline (T-04-07) executes unmodified -- there is only one Art-Net configuration mutation path in this codebase."
    - "Mirror-the-wire-shape-locally-without-tags discipline extended to an untagged nested Go type: internal/artnet.Target/TargetHealth declare no json tags, so their JSON wire shape uses plain (capitalized) Go field names even though the enclosing statusPayload IS camelCase-tagged -- svc_artnetconfig.go's artnetTargetWire mirrors that exact untagged nested shape rather than assuming a uniform tagging convention across the whole payload."
    - "Own-package daemon test harness copy: internal/command/artnet_test.go's startTestArtnetDaemon/testArtnetPipeName helpers are unexported in a different package, so svc_artnetconfig_test.go declares its own equivalent copies built against the identical exported surface (artnet.Run/artnet.Config, artnet.ListCandidateInterfaces, artnetipc.Dial) for a genuine daemon round-trip test rather than a mocked/injected seam."

key-files:
  created:
    - internal/wails/svc_artnetconfig.go
    - internal/wails/svc_artnetconfig_test.go
    - frontend/src/components/ArtnetConfig/ArtnetConfig.tsx
    - frontend/src/components/ArtnetConfig/ArtnetConfig.module.css
  modified:
    - frontend/src/lib/wailsBridge.ts
    - frontend/src/App.tsx
    - cmd/golc-desktop/main.go

key-decisions:
  - "ArtnetConfigService's Configure/EnableTarget/DisableTarget/ListInterfaces/FetchArtnetStatus dispatch through command.NewDefaultCommandRegistry (svc_playback.go's execute pattern) rather than dialing the daemon directly (svc_safety.go's dial pattern) -- this task's own read_first names this choice explicitly so the command layer's ValidateTarget-before-forward logic runs for free, with no re-implementation or duplication of that check in the Wails layer."
  - "FetchArtnetStatus decodes only the 'targets' and 'interface' members of 'artnet status --json' via plain encoding/json (never strictjson.DecodeStrict, which would reject the sibling frame/universes/playback members this service has no on-screen use for yet) -- mirrors svc_safety.go's daemonPlaybackEnvelope's identical narrow-decode discipline."
  - "The interface picker is read-only (ListInterfaces lists candidates; there is no on-screen 'select interface' action) since no CLI route exists to change the daemon's already-pinned interface at runtime -- interface pinning happens at 'artnet serve --interface <n>' startup (CONTEXT D-05), out of this slice's scope, matching 06-11-PLAN.md's own flagged assumption that discovery/interface-selection UX is a later addition."

requirements-completed: [PLAY-11]

coverage:
  - id: D1
    description: "A show author can list the available Windows network interfaces through on-screen controls (bound to 'artnet interface list')."
    requirement: "PLAY-11"
    verification:
      - kind: unit
        ref: "internal/wails/svc_artnetconfig_test.go#TestArtnetConfigServiceOfflineWhenDaemonUnreachable"
        status: pass
    human_judgment: true
    rationale: "ListInterfaces's OS-level enumeration and offline-safe behavior are unit-proven; the actual on-screen rendering of the interface list in ArtnetConfig.tsx against a real golc-desktop build is a visual check queued for end-of-phase UAT."
  - id: D2
    description: "A show author can configure an Art-Net universe -> unicast target (IP, optional port, enabled flag) through on-screen controls (bound to 'artnet configure'), and a malformed/out-of-range target is rejected by the route's own diagnostic before any daemon round trip."
    requirement: "PLAY-11"
    verification:
      - kind: unit
        ref: "internal/wails/svc_artnetconfig_test.go#TestArtnetConfigServiceRejectsMalformedTargetBeforeForward"
        status: pass
      - kind: unit
        ref: "internal/wails/svc_artnetconfig_test.go#TestArtnetConfigServiceRejectsOutOfRangePort"
        status: pass
      - kind: unit
        ref: "internal/wails/svc_artnetconfig_test.go#TestArtnetConfigServiceConfigureThenStatusRoundTrip"
        status: pass
    human_judgment: true
    rationale: "The Go-side validate-before-forward and configure/status round trip are unit-proven against a real artnet.Run daemon; the on-screen add-target form's click-through against a running golc-desktop build is a visual check queued for end-of-phase UAT."
  - id: D3
    description: "A show author can enable or disable a configured target through on-screen controls (bound to 'artnet target enable' / 'artnet target disable')."
    requirement: "PLAY-11"
    verification:
      - kind: unit
        ref: "internal/wails/svc_artnetconfig_test.go#TestArtnetConfigServiceConfigureThenStatusRoundTrip"
        status: pass
    human_judgment: true
    rationale: "The Go-side enable/disable round trip against a real daemon is unit-proven; the on-screen toggle button's click-through is a visual check queued for end-of-phase UAT."
  - id: D4
    description: "The current interface and per-universe/target status is visible on screen (read from 'artnet status --json'), with an explicit daemon-unreachable state when the daemon cannot be reached."
    requirement: "PLAY-11"
    verification:
      - kind: unit
        ref: "internal/wails/svc_artnetconfig_test.go#TestArtnetConfigServiceStatusOfflineProjection"
        status: pass
      - kind: unit
        ref: "internal/wails/svc_artnetconfig_test.go#TestArtnetConfigServiceOfflineWhenDaemonUnreachable"
        status: pass
    human_judgment: true
    rationale: "FetchArtnetStatus's explicit offline projection (Reachable=false, non-nil empty Targets) is unit-proven; the on-screen daemon-unreachable panel's rendering (UI-SPEC copy + offline status color) is a visual check queued for end-of-phase UAT."
  - id: D5
    description: "svc_artnetconfig.go does not import internal/artnet directly (T-06-33: no second Art-Net output path)."
    verification:
      - kind: grep
        ref: "grep -n 'internal/artnet\"' internal/wails/svc_artnetconfig.go (no import-statement match; only doc-comment mentions)"
        status: pass
    human_judgment: false

# Metrics
duration: 35min
completed: 2026-07-23
status: complete
---

# Phase 6 Plan 11: ArtnetConfigService and On-Screen Art-Net Configuration Summary

**Wails ArtnetConfigService binds the existing artnet interface/configure/target/status CLI routes to a new ArtnetConfig React component, closing VERIFICATION.md Gap B[0] for PLAY-11 with genuine on-screen deployment-interface listing and Art-Net universe/target configuration driven over the supervised daemon.**

## Performance

- **Duration:** ~35 min
- **Completed:** 2026-07-23T23:04:08Z
- **Tasks:** 3 (TDD: RED / GREEN / polish)
- **Files modified:** 7 (4 created, 3 modified)

## Accomplishments

- `internal/wails.ArtnetConfigService` (ListInterfaces, Configure, EnableTarget, DisableTarget, FetchArtnetStatus) drives the exact `artnet interface list` / `artnet configure` / `artnet target enable` / `artnet target disable` / `artnet status` CLI routes `internal/command/artnet.go` already implements and tests, dispatched through the in-process command registry (mirrors `svc_playback.go`'s `execute` pattern) so `artnet.ValidateTarget`'s validate-before-forward discipline (T-04-07) runs unmodified -- no second Art-Net configuration path.
- A show author can now list candidate network interfaces, configure a universe -> unicast IP target (with optional port and enabled flag), enable/disable a configured target, and read live per-target send/reachability status and the daemon's pinned-interface health -- all through `frontend/src/components/ArtnetConfig/ArtnetConfig.tsx`, on screen rather than CLI-only.
- `FetchArtnetStatus` returns the explicit `offlineArtnetStatus()` projection (`Reachable=false`, a non-nil empty `Targets` slice) on any daemon-unreachable or undecodable-response failure, mirroring `svc_safety.go`'s `offlineStatusSnapshot` discipline -- the frontend never has to infer meaning from a blank/partial payload.
- A genuine per-test `artnet.Run` daemon harness (`startTestArtnetConfigDaemon`, built against the exported `artnet`/`artnet/ipc` surface since `internal/command/artnet_test.go`'s own helpers are unexported in a different package) proves the real configure -> status -> enable/disable round trip end-to-end, not just a mocked seam.
- Interface list, target list, the daemon-unreachable panel (UI-SPEC copy + `offline` status color), empty states, an error banner, a loading skeleton, and fixed-height overflow scrolling are all implemented in `ArtnetConfig.tsx`/`.module.css`.

## Task Commits

Each task was committed atomically:

1. **Task 1: Failing end-to-end ArtnetConfigService tests** - `74ccba8` (test)
2. **Task 2: ArtnetConfigService + ArtnetConfig component + bridge + mount + bind** - `ac5af22` (feat)
3. **Task 3: Client-side out-of-range guard + state-coverage confirmation** - `1e5271a` (feat)

_Note: this is a `tdd="true"` plan; Task 1 is RED, Task 2 is GREEN, Task 3 is the polish/validation pass._

## Files Created/Modified

- `internal/wails/svc_artnetconfig.go` - ArtnetConfigService binding: in-process command-registry dispatch of every `artnet ...` route, JSON-safe interface/target/status projection types mirroring the daemon's wire shapes without importing `internal/artnet`
- `internal/wails/svc_artnetconfig_test.go` - Five `TestArtnetConfigService*` tests (malformed-target rejection, offline-unreachable projection, real-daemon configure/status/enable/disable round trip, dedicated offline-projection test, out-of-range-port rejection)
- `frontend/src/components/ArtnetConfig/ArtnetConfig.tsx` - On-screen interface list, universe->target configure form, enable/disable toggles, status panel, and full state coverage (loading/empty/error/offline/overflow)
- `frontend/src/components/ArtnetConfig/ArtnetConfig.module.css` - CSS Module for the Art-Net configuration feature, reusing GOLC brand tokens (including the `offline`/`frame-lock` status colors)
- `frontend/src/lib/wailsBridge.ts` - Added `ArtnetConfigServiceBinding` interface, `window.go.wails.ArtnetConfigService` ambient property, view types (`ArtnetInterfaceView`/`ArtnetTargetView`/`ArtnetPinnedInterfaceView`/`ArtnetStatusView`), and exported helper functions
- `frontend/src/App.tsx` - Mounted `<ArtnetConfig />` in the feature region, alongside 06-10's `<FixturePatch />`
- `cmd/golc-desktop/main.go` - Constructed and bound `ArtnetConfigService` in `options.App{Bind: [...]}`, alongside 06-10's `FixturePatchService`

## Decisions Made

- Configure/EnableTarget/DisableTarget/ListInterfaces/FetchArtnetStatus all dispatch through the in-process command registry (svc_playback.go's `execute` pattern) rather than dialing the daemon directly (svc_safety.go's `dial` pattern) -- this task's own read_first named this choice explicitly, so the command layer's `artnet.ValidateTarget`-before-forward logic (T-04-07) runs unmodified with no duplicate validation in the Wails layer.
- `FetchArtnetStatus` decodes only the `targets`/`interface` members of `artnet status --json` via plain `encoding/json` (never `strictjson.DecodeStrict`, which would reject the sibling `frame`/`universes`/`playback` members this service does not project yet) -- mirrors `svc_safety.go`'s `daemonPlaybackEnvelope`'s identical narrow-decode discipline.
- `internal/artnet.Target`/`TargetHealth` declare no JSON tags, so their wire shape inside `artnet status --json`'s `targets` array uses plain (capitalized) Go field names even though the enclosing payload is otherwise camelCase-tagged; `svc_artnetconfig.go`'s `artnetTargetWire` mirrors that exact untagged nested shape rather than assuming uniform tagging across the whole payload.
- The interface picker is read-only (no "select interface" action) since no CLI route exists to change the daemon's already-pinned interface at runtime -- interface pinning happens at `artnet serve --interface <n>` startup (CONTEXT D-05), which is out of this slice's scope per the plan's own flagged assumption.

## Deviations from Plan

**1. [Plan-structure consolidation, non-functional] All five `TestArtnetConfigService*` tests were written in Task 1's RED commit rather than splitting three (Task 1) / two (Task 3) across commits**
- **Found during:** Task 1 (writing the failing test file)
- **Issue:** 06-11-PLAN.md's Task 1 acceptance criteria calls for three tests (malformed-target rejection, offline-unreachable, configure/status round trip) and Task 3 calls for two more (dedicated offline-projection test, out-of-range-port rejection). Both sets share the exact same daemon-harness helpers and package, so writing all five together in one RED pass avoided a redundant second edit-and-rerun cycle against the same test file -- mirrors the identical, already-accepted deviation documented in 06-10-SUMMARY.md for FixturePatchService's own six tests.
- **Fix:** All five tests were authored and committed in Task 1's `test(06-11): ...` commit (`74ccba8`); they remained correctly RED (failed to build: `undefined: NewArtnetConfigService`, `undefined: ArtnetStatusView`) at that commit since `svc_artnetconfig.go` did not exist yet, and all five turned GREEN together at Task 2's commit (`ac5af22`).
- **Files modified:** `internal/wails/svc_artnetconfig_test.go`
- **Verification:** `go test ./internal/wails/... -run TestArtnetConfigService` failed to build at the Task 1 commit (RED: confirmed via `go test` output showing `undefined:` errors) and passed with all five tests green at the Task 2 commit (GREEN) -- TDD gate sequence (test commit before feat commit) is intact; only the internal test/task boundary shifted.
- **Committed in:** `74ccba8` (Task 1), confirmed green in `ac5af22` (Task 2)

**2. [Rule 3 - blocking issue, environment setup] `go build ./...` initially failed with `pattern all:frontend/dist: no matching files found`**
- **Found during:** Task 2, before the first build attempt
- **Issue:** `cmd/golc-desktop`'s `//go:embed all:frontend/dist` directive requires a populated `frontend/dist` directory, which did not exist in this freshly checked-out worktree (`frontend/node_modules` was also absent).
- **Fix:** Ran `npm install` followed by `npm run build` inside `frontend/`, which Vite's own configured output redirects into `cmd/golc-desktop/frontend/dist`. Not a plan deviation -- first-build environment setup identical to 06-10-SUMMARY.md's own documented issue.
- **Files modified:** none (build artifacts only; `frontend/dist` is gitignored)
- **Verification:** `go build ./...` and `cd frontend && npm run build` both green afterward.
- **Committed in:** n/a (no source change)

---

**Total deviations:** 2 (1 plan-structure consolidation with no functional impact, 1 environment-setup blocker resolved per Rule 3)
**Impact on plan:** No scope creep; RED/GREEN TDD gate order preserved exactly (test commit precedes feat commit). Task 3's own commit (`1e5271a`) instead focused on genuine additional polish (client-side out-of-range guard on the add-target form) since its assigned tests were already passing as of Task 2.

## Human Verification Queued (end-of-phase UAT)

Per `workflow.human_verify_mode=end-of-phase`, Task 3's `<human-check>` is recorded here rather than treated as a mid-execution checkpoint (the plan's frontmatter is `autonomous: true`; this `<human-check>` tag is an end-of-phase UAT marker, not an interactive gate):

> Launch golc-desktop with the supervised daemon, pick an interface, add an Art-Net universe->IP target, toggle it enabled/disabled, and confirm the status panel reflects the change; kill the daemon and confirm the explicit daemon-unreachable state renders per UI-SPEC.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `ArtnetConfigService` and `ArtnetConfig.tsx` are fully wired (bound in `cmd/golc-desktop/main.go`, mounted in `App.tsx` alongside 06-10's `FixturePatch`) and ready for the end-of-phase UAT pass alongside the rest of Phase 6's on-screen surfaces.
- No conflicts with 06-10's `FixturePatchService`/`FixturePatch.tsx` wiring: both additions to `wailsBridge.ts`, `App.tsx`, and `cmd/golc-desktop/main.go` were applied alongside the already-merged 06-10 edits, not in place of them.

## Self-Check: PASSED

All created files verified present on disk (`internal/wails/svc_artnetconfig.go`, `internal/wails/svc_artnetconfig_test.go`, `frontend/src/components/ArtnetConfig/ArtnetConfig.tsx`, `ArtnetConfig.module.css`) and all three task commits (`74ccba8`, `ac5af22`, `1e5271a`) confirmed present via `git log --oneline`.

---
*Phase: 06-wails-authoring-and-operator-surface*
*Completed: 2026-07-23*
