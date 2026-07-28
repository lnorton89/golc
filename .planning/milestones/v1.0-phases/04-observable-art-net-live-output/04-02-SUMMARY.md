---
phase: 04-observable-art-net-live-output
plan: 02
subsystem: artnet
tags: [artnet, networking, windows, target-model, go]

# Dependency graph
requires:
  - phase: 04-observable-art-net-live-output
    plan: 01
    provides: internal/artnet package seeded (packet.go protocol codec, channelmap.go semantic-to-DMX transform), GOLC_ARTNET_* diagnostic convention
provides:
  - artnet.InterfaceInfo / ListCandidateInterfaces() for ARTN-01 interface enumeration
  - artnet.InterfaceManager with pinned-by-index loss detection (Start/Stop/Check, Status/Err, LocalIP) for D-05
  - artnet.Target model with ValidateTarget, ValidateUniqueTargets, SetEnabled for ARTN-02/D-07/D-08/D-12
affects: [04-03, 04-04, 04-05, 04-06, 04-07]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "InterfaceManager mirrors internal/playback/engine.go's context.WithCancel/ticker/goroutine Start(ctx)/Stop() lifecycle for its own independent 1Hz loss-poll loop"
    - "Target model mirrors internal/deployment/model.go's bounds-check-then-diagnostic ValidateInstanceAddress shape and Activate's copy-returning (never mutate caller's slice) discipline"
    - "GOLC_ARTNET_* diagnostic convention extended: GOLC_ARTNET_INTERFACE_ENUM_FAILED, GOLC_ARTNET_INTERFACE_LOST, GOLC_ARTNET_TARGET_INVALID, GOLC_ARTNET_TARGET_DUPLICATE, GOLC_ARTNET_TARGET_NOT_FOUND"

key-files:
  created:
    - internal/artnet/interfacemgr.go
    - internal/artnet/interfacemgr_test.go
    - internal/artnet/target.go
    - internal/artnet/target_test.go
  modified: []

key-decisions:
  - "InterfaceManager pins by net.Interface.Index only; Name is stored strictly for display and is never used to re-resolve the interface (Pitfall 4)."
  - "The interface-loss poll body is exposed as an exported Check() method (not test-only) so both the 1Hz Start() ticker and any future manual re-check (e.g. a CLI 'artnet status --refresh' path) can trigger the same single-iteration check without duplicating logic."
  - "Target.Port's zero value is treated as 'unspecified, defaults to 6454' rather than requiring every caller to pre-fill artNetPort; effectivePort() resolves this consistently across ValidateTarget, ValidateUniqueTargets, and SetEnabled so a target with an explicit port and one relying on the default collide correctly for duplicate/match detection."
  - "ValidateTarget explicitly rejects the IPv4 broadcast address (255.255.255.255), not just the unspecified address (0.0.0.0) -- this enforces D-07's unicast-only guarantee at the data-validation layer, not merely by the absence of a broadcast constructor."

requirements-completed: [ARTN-01, ARTN-02]

coverage:
  - id: D1
    description: "ListCandidateInterfaces enumerates OS network interfaces (Index/Name/Up/Addrs) for ARTN-01 selection; enumeration failure wraps as GOLC_ARTNET_INTERFACE_ENUM_FAILED"
    requirement: "ARTN-01"
    verification:
      - kind: unit
        ref: "internal/artnet/interfacemgr_test.go#TestInterfaceListCandidateInterfacesFindsLoopback"
        status: pass
    human_judgment: false
  - id: D2
    description: "InterfaceManager pins by net.Interface.Index (never Name), detects pinned-interface loss via its own independent 1Hz poll loop, and never auto-switches to another interface (D-05)"
    requirement: "ARTN-01"
    verification:
      - kind: unit
        ref: "internal/artnet/interfacemgr_test.go#TestInterfaceManagerMarkLostTransitionsStatus"
        status: pass
      - kind: unit
        ref: "internal/artnet/interfacemgr_test.go#TestInterfaceManagerBogusIndexLostAfterOnePollIteration"
        status: pass
    human_judgment: false
  - id: D3
    description: "LocalIP() returns the pinned interface's own local IPv4 address for future bind-by-address use (Pitfall 5: no SO_BINDTODEVICE on Windows)"
    requirement: "ARTN-01"
    verification:
      - kind: unit
        ref: "internal/artnet/interfacemgr_test.go#TestInterfaceManagerLocalIPReturnsPinnedInterfaceIP"
        status: pass
      - kind: unit
        ref: "internal/artnet/interfacemgr_test.go#TestInterfaceManagerLocalIPFailsForBogusIndex"
        status: pass
    human_judgment: false
  - id: D4
    description: "Target{Universe, IP, Port, Enabled} validates invalid universe/IP/port as GOLC_ARTNET_TARGET_INVALID, rejects the IPv4 broadcast address (D-07 unicast-only)"
    requirement: "ARTN-02"
    verification:
      - kind: unit
        ref: "internal/artnet/target_test.go#TestTargetValidateTargetAcceptsValidTarget"
        status: pass
      - kind: unit
        ref: "internal/artnet/target_test.go#TestTargetValidateTargetRejectsBroadcastIP"
        status: pass
    human_judgment: false
  - id: D5
    description: "ValidateUniqueTargets rejects duplicate (Universe, IP, Port) triples (GOLC_ARTNET_TARGET_DUPLICATE) while explicitly allowing fan-out (D-08: same universe, multiple targets; same IP/Port, multiple universes)"
    requirement: "ARTN-02"
    verification:
      - kind: unit
        ref: "internal/artnet/target_test.go#TestTargetValidateUniqueTargetsAcceptsFanOutSameUniverseDifferentIPs"
        status: pass
      - kind: unit
        ref: "internal/artnet/target_test.go#TestTargetValidateUniqueTargetsRejectsDuplicateTriple"
        status: pass
    human_judgment: false
  - id: D6
    description: "SetEnabled toggles per-target enable/disable, returning a fresh copy without mutating the caller's slice (D-12)"
    requirement: "ARTN-02"
    verification:
      - kind: unit
        ref: "internal/artnet/target_test.go#TestTargetSetEnabledReturnsFreshSliceLeavingInputUnchanged"
        status: pass
    human_judgment: false

duration: 15min
completed: 2026-07-22
status: complete
---

# Phase 4 Plan 02: Interface Manager & Unicast Target Model Summary

**Pinned-by-index Windows interface manager with independent loss detection (D-05) plus a unicast-only Art-Net target model supporting fan-out, dedupe, and copy-returning enable/disable (D-07/D-08/D-12).**

## Performance

- **Duration:** ~15 min
- **Completed:** 2026-07-22T08:14:25Z
- **Tasks:** 2
- **Files modified:** 4 (4 created, 0 modified)

## Accomplishments
- `internal/artnet/interfacemgr.go`: `ListCandidateInterfaces()` enumerates OS network interfaces for ARTN-01 selection; `InterfaceManager` pins by `net.Interface.Index` (never `Name`, Pitfall 4), runs its own independent 1Hz `pollInterfaceLoss` loop mirroring `engine.go`'s `Start(ctx)`/`Stop()` lifecycle, and never selects a different interface automatically once lost (D-05) -- `markLost`/`Check` are the only status-mutating paths, both terminal-until-reconfigured.
- `LocalIP()` returns the pinned interface's own local IPv4 unicast address, ready for Plan 03's worker to bind `net.ListenUDP`/`net.DialUDP` against a specific local address rather than `0.0.0.0` or a Linux-only device-bind option (Pitfall 5).
- `internal/artnet/target.go`: `Target{Universe, IP, Port, Enabled}` validates via `ValidateTarget` (non-positive universe, nil/unspecified/broadcast IP, out-of-range port all rejected as `GOLC_ARTNET_TARGET_INVALID`); `ValidateUniqueTargets` rejects duplicate `(Universe, IP, Port)` triples (`GOLC_ARTNET_TARGET_DUPLICATE`) while explicitly permitting D-08 fan-out (same universe, multiple targets) and the reverse (same IP/Port, multiple universes).
- `SetEnabled` toggles a single target's enable/disable state and returns a fresh copy of the slice, never mutating the caller's own slice (D-12, mirrors `deployment.Activate`'s copy-returning discipline); an unmatched target fails with `GOLC_ARTNET_TARGET_NOT_FOUND`.

## Task Commits

Each task was committed atomically:

1. **Task 1: Interface enumeration + pinned-by-index loss detection (ARTN-01, D-05)** - `15cd43c` (feat)
2. **Task 2: Unicast target model with fan-out, dedupe, and per-target enable/disable (ARTN-02, D-07/D-08/D-12)** - `19d0322` (feat)

**Plan metadata:** (recorded in final commit)

## Files Created/Modified
- `internal/artnet/interfacemgr.go` - `InterfaceInfo`, `ListCandidateInterfaces`, `InterfaceManager` (pin/poll/Status/Err/LocalIP/Start/Stop)
- `internal/artnet/interfacemgr_test.go` - loopback enumeration, markLost transition, bogus-index single-poll-iteration loss, LocalIP success/failure
- `internal/artnet/target.go` - `Target`, `effectivePort`, `targetKey`/`keyOf`, `ValidateTarget`, `ValidateUniqueTargets`, `SetEnabled`
- `internal/artnet/target_test.go` - valid/invalid target cases, fan-out acceptance, duplicate-triple rejection, default-port dedupe, copy-returning `SetEnabled`

## Decisions Made
- `InterfaceManager` pins strictly by `net.Interface.Index`; `Name` is retained only for display and is never re-resolved against (Pitfall 4).
- The poll body is an exported `Check()` method rather than an unexported test-only helper, since a future CLI refresh path (`golc artnet status --refresh`) can reuse the exact same single-iteration check the 1Hz ticker calls internally.
- `Target.Port == 0` is treated as "unspecified, defaults to 6454" via `effectivePort()`, applied consistently across validation, dedupe, and `SetEnabled` matching, so a target relying on the default and one with an explicit `Port: 6454` are treated identically everywhere.
- `ValidateTarget` explicitly rejects `net.IPv4bcast` (255.255.255.255) in addition to the unspecified address, enforcing D-07's unicast-only guarantee at the data-validation layer rather than relying solely on the absence of a broadcast constructor.

## Deviations from Plan

None - plan executed exactly as written. Both tasks matched the plan's action text and acceptance criteria directly against 04-RESEARCH.md Pattern 2 and 04-PATTERNS.md's `internal/deployment/model.go`/`internal/playback/engine.go` analogs with no corrections needed.

## Issues Encountered

None specific to this plan. (The pre-existing, out-of-scope `internal/trace/catalog` `TestScopeLinearMap` failure noted in 04-01-SUMMARY.md's Issues Encountered remains unrelated to and untouched by this plan.)

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- `internal/artnet` now has both the encoding foundation (Plan 01: `packet.go`, `channelmap.go`) and the configuration primitives (Plan 02: `interfacemgr.go`, `target.go`) Plan 03's worker needs to bind an interface and fan out ArtDMX frames to real unicast targets.
- `InterfaceManager.LocalIP()` and `Status()`/`Err()` are ready for Plan 03's worker to consult before every bind/send cycle, and for a future `golc artnet status` CLI route (Plan 05+) to surface D-11's persistent health indicator.
- `Target`/`ValidateTarget`/`ValidateUniqueTargets`/`SetEnabled` are ready for Plan 03's worker to consume as its configured fan-out list, and for a future `golc artnet target enable/disable` CLI route to call `SetEnabled` directly.
- No blockers for Plan 03 (worker/send-loop implementation).

---
*Phase: 04-observable-art-net-live-output*
*Completed: 2026-07-22*

## Self-Check: PASSED

- FOUND: internal/artnet/interfacemgr.go
- FOUND: internal/artnet/interfacemgr_test.go
- FOUND: internal/artnet/target.go
- FOUND: internal/artnet/target_test.go
- FOUND commits: 15cd43c, 19d0322 (both present in `git log --oneline --all`)
