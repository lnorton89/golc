---
phase: 04-observable-art-net-live-output
plan: 09
subsystem: artnet
tags: [go, ipc, cli, dmx, observability]

# Dependency graph
requires:
  - phase: 04-observable-art-net-live-output
    provides: "InterfaceManager pinning/loss-detection (04-02), daemon status IPC route and statusPayload wire type (04-04/04-05), artnet status CLI route and rendering (04-05), statusPayload.Universes (04-08)"
provides:
  - "daemon statusPayload.Interface / command artnetStatusPayload.Interface: the pinned interface's live PinnedIndex/PinnedName/Status/Error, read from the daemon's own InterfaceManager"
  - "renderArtnetStatusPlain GOLC_ARTNET_INTERFACE_STATUS: line (plain and watch views)"
  - "runArtnetInterfaceList --pipe support with best-effort pinned-candidate annotation (PINNED/STATUS columns plain, pinned/status fields --json) and graceful no-daemon fallback"
  - "Corrected Run() comment: no longer falsely claims interface loss was already surfaced before this plan"
affects: [04-VERIFICATION.md Gap 2 closure, future artnet observability/UI work reading artnet status or interface list]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Daemon reads its own already-built, already-tested InterfaceManager surface (PinnedIndex/PinnedName/Status/Err) directly into the status payload rather than modifying interfacemgr.go itself"
    - "Best-effort daemon round trip that swallows a fetchArtnetStatus failure to preserve a route's daemon-free graceful-degradation behavior (interface list's no-daemon path)"

key-files:
  created: []
  modified:
    - internal/artnet/daemon.go
    - internal/artnet/daemon_test.go
    - internal/command/artnet.go
    - internal/command/artnet_test.go

key-decisions:
  - "artnet status is the authoritative pinned-status surface; artnet interface list additionally annotates the pinned candidate only when a daemon is reachable, per the orchestrator's option (a)+(b) design choice"
  - "interface list's daemon round trip ignores fetchArtnetStatus's error Result entirely on failure -- the existing no-daemon ExitCode-0 candidate enumeration must never regress to a DAEMON_UNREACHABLE failure"

patterns-established:
  - "Both daemon and CLI wire-type mirrors (interfaceStatusPayload / artnetInterfaceStatus) are added atomically in the same task/commit -- required by strictjson.DecodeStrict's DisallowUnknownFields, same discipline 04-08 established for Universes"

requirements-completed: [ARTN-01]

coverage:
  - id: D1
    description: "The pinned interface's live Status()/Err() is surfaced through golc artnet status (plain + --json + watch), reporting index/name/status/error"
    requirement: "ARTN-01"
    verification:
      - kind: unit
        ref: "internal/artnet/daemon_test.go#TestDaemonStatusPayloadIncludesPinnedInterfaceStatus"
        status: pass
      - kind: integration
        ref: "internal/command/artnet_test.go#TestArtnetStatusPlainRendersPinnedInterface"
        status: pass
      - kind: integration
        ref: "internal/command/artnet_test.go#TestArtnetStatusJSONIncludesInterfaceStatus"
        status: pass
    human_judgment: false
  - id: D2
    description: "A lost/degraded pinned interface reports status=lost plus a GOLC_ARTNET_INTERFACE_LOST error through artnet status, never a silent switch (D-05)"
    requirement: "ARTN-01"
    verification:
      - kind: unit
        ref: "internal/artnet/daemon_test.go#TestDaemonStatusPayloadSurfacesLostInterface"
        status: pass
    human_judgment: false
  - id: D3
    description: "golc artnet interface list annotates which candidate is the daemon's pinned interface and its live status when a daemon is reachable, in both plain (PINNED/STATUS columns) and --json (pinned/status fields) rendering"
    requirement: "ARTN-01"
    verification:
      - kind: integration
        ref: "internal/command/artnet_test.go#TestArtnetInterfaceListAnnotatesPinnedWhenDaemonRunning"
        status: pass
    human_judgment: false
  - id: D4
    description: "golc artnet interface list still lists every candidate gracefully (ExitCode 0, no GOLC_ARTNET_DAEMON_UNREACHABLE) when no daemon is running -- no regression of the previously-tested no-daemon behavior"
    requirement: "ARTN-01"
    verification:
      - kind: integration
        ref: "internal/command/artnet_test.go#TestArtnetInterfaceListWorksWithNoDaemon"
        status: pass
    human_judgment: false

duration: 5min
completed: 2026-07-23
status: complete
---

# Phase 04 Plan 09: Pinned Interface Status Surfacing Summary

**Closed 04-VERIFICATION.md Gap 2 by threading the daemon's already-built, already-tested `InterfaceManager.Status()`/`Err()` into `golc artnet status` (authoritative surface) and annotating the pinned candidate in `golc artnet interface list` (best-effort, gracefully degrading with no daemon), correcting a false doc comment along the way.**

## Performance

- **Duration:** 5 min
- **Started:** 2026-07-23T03:29:18Z
- **Completed:** 2026-07-23T03:33:39Z
- **Tasks:** 2 completed
- **Files modified:** 4

## Accomplishments
- `daemon.statusPayload` and `command.artnetStatusPayload` both gained a matching `interface` JSON field (`interfaceStatusPayload`/`artnetInterfaceStatus`, identical json tags) so `strictjson.DecodeStrict`'s `DisallowUnknownFields` never breaks plain/watch decode.
- `handleStatus` reads `d.ifaceMgr.PinnedIndex()/PinnedName()/Status()/Err()` on every status request -- the daemon's own already-tested interface-manager surface, unmodified by this plan.
- `renderArtnetStatusPlain` emits a `GOLC_ARTNET_INTERFACE_STATUS: index=<n> name=<name> status=<ok|lost>` line (with `error=<...>` appended when non-empty), inherited automatically by the `--watch` view.
- `runArtnetInterfaceList` now accepts `--pipe`, makes a best-effort `fetchArtnetStatus` round trip, and annotates the pinned candidate's live status via new `PINNED`/`STATUS` plain columns and a self-describing `--json` `interfaceListEntry` shape (`index`/`name`/`up`/`addrs`/`pinned`/`status`) -- falling back to the unchanged plain candidate list (ExitCode 0, no `GOLC_ARTNET_DAEMON_UNREACHABLE`) when no daemon is reachable.
- Corrected the misleading `Run()` comment: it no longer asserts the interface loss was "already" surfaced to any status caller before this plan existed.

## Task Commits

Each task was committed atomically:

1. **Task 1: Surface the pinned interface's live Status()/Err() through the daemon status payload and CLI status rendering (ARTN-01/D-05)** - `c7e3a6d` (feat)
2. **Task 2: Annotate golc artnet interface list with the pinned interface + graceful no-daemon fallback (ARTN-01/D-05)** - `1cff374` (feat)

_Note: this plan's SUMMARY/metadata commit is created separately per worktree-mode convention (STATE.md/ROADMAP.md excluded; the orchestrator updates those after merge)._

## Files Created/Modified
- `internal/artnet/daemon.go` - `interfaceStatusPayload` type, `statusPayload.Interface`, `newStatusPayload` takes an `interfaceStatusPayload` parameter, `handleStatus` builds it from `d.ifaceMgr`, corrected `Run()` comment
- `internal/artnet/daemon_test.go` - `TestDaemonStatusPayloadIncludesPinnedInterfaceStatus`, `TestDaemonStatusPayloadSurfacesLostInterface`
- `internal/command/artnet.go` - `artnetInterfaceStatus` mirror type, `artnetStatusPayload.Interface`, `renderArtnetStatusPlain` interface-status line, `interfaceListEntry` JSON render struct, `runArtnetInterfaceList` `--pipe` support + best-effort pinned annotation with no-daemon fallback, updated route summary doc comment
- `internal/command/artnet_test.go` - `TestArtnetStatusPlainRendersPinnedInterface`, `TestArtnetStatusJSONIncludesInterfaceStatus`, `TestArtnetInterfaceListAnnotatesPinnedWhenDaemonRunning`, `TestArtnetInterfaceListWorksWithNoDaemon`

## Decisions Made
- `artnet status` is the authoritative pinned-interface status surface; `artnet interface list` additionally annotates the pinned candidate only when a daemon round trip succeeds, matching the orchestrator's option (a)+(b) design choice and satisfying every wording of the original must-have (a status route plus the original truth naming `interface list`).
- `interface list`'s best-effort round trip discards `fetchArtnetStatus`'s error `Result` entirely on failure rather than surfacing any part of it, so the previously-tested no-daemon ExitCode-0 full-candidate-list behavior is preserved byte-for-byte.

## Deviations from Plan

None - plan executed exactly as written. `interfacemgr.go` was read but not modified, matching the plan's explicit scope boundary; both tasks' `read_first` guidance (the 04-08-added `Universes` field to build alongside, `DisallowUnknownFields`'s two-sided-struct-edit requirement, the existing no-daemon `interface list` test pattern) matched the codebase exactly, so no auto-fixes, blocking issues, or architectural changes were needed.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- 04-VERIFICATION.md Gap 2 is closed: the pinned interface's live status is surfaced through `golc artnet status` (authoritative, plain/json/watch) and annotated in `golc artnet interface list` (best-effort, graceful no-daemon fallback), and a lost pinned interface shows `status=lost` plus `GOLC_ARTNET_INTERFACE_LOST`, never a silent switch (D-05).
- 04-01 through 04-08 are untouched; this plan only extended `daemon.go`/`command/artnet.go` and their tests, plus reading (not modifying) `interfacemgr.go`.
- Both of 04-VERIFICATION.md's tracked gaps (Gap 1 via 04-08, Gap 2 via this plan) are now closed.

## Known Stubs
None.

## Threat Flags
None - this plan adds a field to the existing owner-ACL'd named-pipe status payload (Plan 04's ACL, unchanged) and one additional best-effort dial to that same local pipe from `interface list`; no new network-facing surface. See the plan's own `<threat_model>` (T-04-02 elevation-of-privilege mitigated at its Plan-04 origin, T-04-12 information-disclosure accepted as local-operator-only, T-04-13 DoS mitigated via `fetchArtnetStatus`'s existing non-hanging dial-failure surfacing).

## Self-Check: PASSED

All 4 modified files confirmed present on disk; commits `c7e3a6d` (Task 1) and `1cff374` (Task 2) confirmed present in git log.

---
*Phase: 04-observable-art-net-live-output*
*Completed: 2026-07-23*
