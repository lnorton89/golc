---
phase: 04-observable-art-net-live-output
plan: 05
subsystem: cli
tags: [artnet, cli, ipc, command-routing, go]

# Dependency graph
requires:
  - phase: 04-observable-art-net-live-output
    plan: 04
    provides: internal/artnet/ipc (NewListener/Serve, Dial/Forward) and internal/artnet.Run/Config (the long-lived daemon), whose wire contract for "artnet configure"/"artnet target enable|disable" this plan's CLI forwards into unchanged
provides:
  - internal/command/artnet.go -- the "artnet" CLI scope: "artnet serve" (foreground daemon), "artnet interface list" (ARTN-01), "artnet configure" (D-08), "artnet status" (ARTN-05, D-02, plain/--json/--watch), "artnet target enable|disable" (D-12), all self-registered via MustDeclareScope/MustDeclareRoute
  - internal/artnet/ipc.Request/Result/Handler -- local wire types (types.go) that let internal/command import internal/artnet and internal/artnet/ipc directly without an import cycle
affects: [04-06, 04-07]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "IPC wire-type extraction to break a subsystem<->command import cycle: internal/artnet/ipc now declares its own Request/Result/Handler (field-for-field identical to command.Request/Result/CommandHandler, no struct tags on either side so the JSON wire shape is unchanged) instead of importing internal/command, mirroring internal/projectconfig's existing 'subsystem never imports command' precedent (internal/command/config.go's own doc comment) -- internal/command/artnet.go converts at the call boundary via forwardToDaemon."
    - "Two-tier --flag value/--flag=value arg parsing with an explicit boolFlags set (parseArtnetArgs), reused across every artnet route exactly like internal/command/playback.go's convention -- malformed args are GOLC_ARTNET_USAGE (ExitCode 2), validated-but-rejected domain values are GOLC_ARTNET_* (ExitCode 1)."
    - "Client routes never re-implement daemon-side wire parsing: targetSelectorArgs constructs the exact --universe/--ip/--port shape internal/artnet/daemon.go's parseTargetSelector already expects, rather than passing the CLI's own raw args (which may carry an unrelated --pipe flag) straight through."

key-files:
  created:
    - internal/artnet/ipc/types.go
    - internal/command/artnet.go
    - internal/command/artnet_test.go
  modified:
    - internal/artnet/ipc/server.go
    - internal/artnet/ipc/client.go
    - internal/artnet/ipc/ipc_test.go
    - internal/artnet/daemon.go
    - internal/artnet/daemon_test.go
    - internal/artnet/health.go

key-decisions:
  - "internal/artnet/ipc and internal/artnet/daemon.go no longer import internal/command at all (04-04's original design did, purely to reuse Request/Result/CommandHandler) -- this plan's internal/command/artnet.go needs to import both internal/artnet (Run, ListCandidateInterfaces) and internal/artnet/ipc (Dial, Forward) directly, which would otherwise create a hard command -> artnet(/ipc) -> command import cycle. Fixed by extracting local, field-identical ipc.Request/Result/Handler types (ipc/types.go) and updating every 04-04 call site (server.go, client.go, daemon.go) plus their existing tests (ipc_test.go, daemon_test.go) to the local types -- the wire JSON is byte-identical, only the Go type's declaring package changed."
  - "'artnet serve' resolves each deployment.Instance to its fixture via an explicit, optional --fixtures <dir> flag (decode every *.yaml/*.yml under it with fixture.Decode, index by fixture.Pin's StableKey) rather than any new fixture-store/lookup service: this repository has no such service anywhere yet, and building one is a larger architectural addition outside this plan's declared file scope. Omitting --fixtures is safe (Resolve is only ever called for an active deployment's instances) and, if ever called, fails loudly with GOLC_ARTNET_FIXTURES_DIR_REQUIRED rather than silently guessing (mirrors D-17's convention) -- documented as a known limitation below, not silently pretended-complete."
  - "'artnet configure'/'artnet target enable|disable' construct the forwarded Request.Args explicitly from parsed+validated values (targetSelectorArgs) rather than passing the CLI's own raw args through, so this file's own additional flags (--pipe) never reach internal/artnet/daemon.go's parser."

requirements-completed: [ARTN-01, ARTN-02, ARTN-05]

coverage:
  - id: D1
    description: "The artnet scope and the serve/interface-list/configure/status/target-enable/target-disable routes self-register via MustDeclareScope/MustDeclareRoute"
    requirement: "ARTN-01"
    verification:
      - kind: unit
        ref: "internal/command/artnet_test.go#TestScopeArtnet/the_artnet_scope_and_every_route_self-register"
        status: pass
    human_judgment: false
  - id: D2
    description: "Malformed 'artnet configure' args return GOLC_ARTNET_USAGE/ExitCode 2; a validated-but-rejected target value returns GOLC_ARTNET_TARGET_INVALID/ExitCode 1, both without ever dialing a daemon"
    requirement: "ARTN-02"
    verification:
      - kind: unit
        ref: "internal/command/artnet_test.go#TestArtnetConfigureUsageErrors"
        status: pass
      - kind: unit
        ref: "internal/command/artnet_test.go#TestArtnetConfigureInvalidTargetReturnsDomainError"
        status: pass
    human_judgment: false
  - id: D3
    description: "A client route with no daemon running on the given pipe returns GOLC_ARTNET_DAEMON_UNREACHABLE, ExitCode 1, never a hang"
    requirement: "ARTN-02"
    verification:
      - kind: unit
        ref: "internal/command/artnet_test.go#TestArtnetNoDaemonReturnsDaemonUnreachable"
        status: pass
    human_judgment: false
  - id: D4
    description: "'artnet status --json' emits canonical JSON containing per-universe frame health and per-target health (send counts, reachability, enablement)"
    requirement: "ARTN-05"
    verification:
      - kind: unit
        ref: "internal/command/artnet_test.go#TestArtnetStatusJSONContainsHealthFields"
        status: pass
    human_judgment: false
  - id: D5
    description: "Plain 'artnet status' renders a persistent per-target status table (D-11) including a freshly configured target"
    requirement: "ARTN-05"
    verification:
      - kind: unit
        ref: "internal/command/artnet_test.go#TestArtnetStatusPlainRendersPersistentTable"
        status: pass
    human_judgment: false
  - id: D6
    description: "'artnet target disable' then 'artnet target enable' visibly toggle one target's Enabled state in a subsequent status; an unknown target selector fails with GOLC_ARTNET_TARGET_NOT_FOUND"
    requirement: "ARTN-05"
    verification:
      - kind: unit
        ref: "internal/command/artnet_test.go#TestArtnetTargetEnableDisableRoundTrip"
        status: pass
      - kind: unit
        ref: "internal/command/artnet_test.go#TestArtnetTargetUnknownReturnsNotFound"
        status: pass
    human_judgment: false

duration: ~45min
completed: 2026-07-22
status: complete
---

# Phase 4 Plan 05: Observable Art-Net CLI Summary

**The `golc artnet` CLI (serve/interface-list/configure/status/target-enable/target-disable) as thin clients over Plan 04's local IPC daemon, with the command<->artnet(/ipc) import cycle resolved by extracting field-identical local wire types into internal/artnet/ipc.**

## Performance

- **Duration:** ~45 min
- **Started:** ~2026-07-22T08:30:00Z
- **Completed:** 2026-07-22T09:17:08Z
- **Tasks:** 2
- **Files modified:** 9 (3 created, 6 modified)

## Accomplishments
- `internal/command/artnet.go` declares the `artnet` scope and self-registers all six routes the phase's core observable capability needs: `artnet serve` (foreground daemon, D-03/D-04), `artnet interface list` (ARTN-01, direct `artnet.ListCandidateInterfaces()` call, no daemon round trip), `artnet configure` (D-08, client-side `artnet.ValidateTarget` before ever forwarding), `artnet status` (ARTN-05/D-02, one-shot plain/`--json` snapshot or a continuously-refreshing `--watch` view), and `artnet target enable`/`artnet target disable` (D-12).
- Resolved a real, build-breaking import cycle discovered while wiring Task 1: `internal/artnet/ipc` and `internal/artnet/daemon.go` (04-04) originally imported `internal/command` purely to reuse `Request`/`Result`/`CommandHandler`; since this plan's CLI needs to import `internal/artnet`/`internal/artnet/ipc` directly, that would be a hard `command -> artnet(/ipc) -> command` cycle. Fixed by extracting field-identical local types into a new `internal/artnet/ipc/types.go` (mirroring `internal/projectconfig`'s existing "subsystem never imports command" precedent) and updating every 04-04 call site plus its own tests -- the wire JSON is byte-identical, only the declaring package changed.
- Found and fixed a real correctness bug in `internal/artnet/health.go`'s `Health.Configure` while proving Task 2's own enable/disable acceptance criterion: it never refreshed an already-tracked target's stored `Target` snapshot (which carries `Enabled`) on a reconfigure, so `artnet target enable|disable` had no visible effect on a subsequent status. Fixed to refresh `Universe`/`Target` on every call while preserving accumulated `SendOK`/`SendErr`/`Reachable`/`LastError`.
- `internal/command/artnet_test.go` declares the `TestScopeArtnet` marker (`test --quick --scope artnet` resolves) and covers both tasks end-to-end, including starting a real `artnet.Run` daemon on an isolated per-test named pipe for the status/target-toggle tests.

## Task Commits

Each task was committed atomically:

1. **Task 1: Declare the artnet scope + serve/interface/configure routes (D-01/D-03, ARTN-01/02)** - `16b00da` (feat)
2. **Task 2: `golc artnet status` (watch + snapshot + --json) and `target enable|disable` (ARTN-05, D-02/D-11/D-12)** - `12af3ed` (feat)

**Plan metadata:** (recorded in final commit)

## Files Created/Modified
- `internal/command/artnet.go` - `artnet` scope + serve/interface-list/configure/status/target-enable/target-disable routes, two-tier arg parsing, fixture resolver, status render/watch
- `internal/command/artnet_test.go` - `TestScopeArtnet` marker + offline usage/domain-error/unreachable coverage + daemon-backed status/target coverage
- `internal/artnet/ipc/types.go` - local `Request`/`Result`/`Handler` types breaking the import cycle
- `internal/artnet/ipc/server.go` - `Serve`/`handleConn`/`writeResult` retyped to the local `Request`/`Result`/`Handler`
- `internal/artnet/ipc/client.go` - `Forward` retyped to the local `Request`/`Result`
- `internal/artnet/ipc/ipc_test.go` - retyped to the local `Request`/`Result`
- `internal/artnet/daemon.go` - `handle`/`handleStatus`/`handleConfigure`/`handleSetEnabled` retyped to `ipc.Request`/`ipc.Result`; no longer imports `internal/command`
- `internal/artnet/daemon_test.go` - retyped to `ipc.Request`
- `internal/artnet/health.go` - `Health.Configure` bug fix (see Deviations)

## Decisions Made
- Local `ipc.Request`/`Result`/`Handler` types (field-for-field identical, no struct tags) replace `internal/command`'s types inside `internal/artnet`/`internal/artnet/ipc`, resolving the import cycle without changing wire-format behavior.
- `artnet serve`'s fixture resolution is an explicit, optional `--fixtures <dir>` flag (decode-on-startup via the existing `fixture.Decode`/`fixture.Pin` pipeline) rather than a new fixture-store service, since none exists anywhere in this repository yet; omitted, it fails loudly per-instance rather than silently guessing.
- `artnet configure`/`artnet target enable|disable` construct the daemon-forwarded `Request.Args` explicitly from parsed values (`targetSelectorArgs`), never passing the CLI's own raw args through, so this file's own extra flags (`--pipe`) never reach the daemon's parser.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Circular dependency] `internal/artnet`/`internal/artnet/ipc` importing `internal/command` created a hard import cycle with this plan's CLI**
- **Found during:** Task 1 (wiring `artnet.go`'s import of `internal/artnet` and `internal/artnet/ipc`)
- **Issue:** 04-04 built `internal/artnet/ipc` (server.go, client.go) and `internal/artnet/daemon.go` against `command.Request`/`command.Result`/`command.CommandHandler` directly (04-04's own locked design, flagged as a risk in its SUMMARY). This plan's `internal/command/artnet.go` must import `internal/artnet` (for `Run`, `ListCandidateInterfaces`) and `internal/artnet/ipc` (for `Dial`, `Forward`) directly -- with `internal/artnet`(/ipc) also importing `internal/command`, that is a direct `command -> artnet(/ipc) -> command` cycle, which fails to compile (confirmed: `go build ./...` failed with an import cycle error before the fix).
- **Fix:** Added `internal/artnet/ipc/types.go` declaring local `Request`/`Result`/`Handler` types, field-for-field identical to `command`'s (no struct tags on either side, so `encoding/json`'s default field-name marshaling produces byte-identical wire JSON). Updated `internal/artnet/ipc/server.go`, `client.go`, `daemon.go` to use these local types instead of `command.*`, removing the `internal/command` import from all three. Updated `internal/artnet/ipc/ipc_test.go` and `internal/artnet/daemon_test.go` (04-04's own tests) to the local types.
- **Files modified:** internal/artnet/ipc/types.go (new), internal/artnet/ipc/server.go, internal/artnet/ipc/client.go, internal/artnet/ipc/ipc_test.go, internal/artnet/daemon.go, internal/artnet/daemon_test.go
- **Verification:** `go build ./...` succeeds; `go test ./internal/artnet/...` (04-04's own suite) still passes unchanged after the retype.
- **Committed in:** `16b00da` (Task 1 commit)

**2. [Rule 1 - Bug] `Health.Configure` never refreshed an already-tracked target's `Enabled` state on a reconfigure**
- **Found during:** Task 2 (proving `TestArtnetTargetEnableDisableRoundTrip`'s own acceptance criterion)
- **Issue:** `Health.Configure` (04-03) only wrote a target's `TargetHealth{Universe, Target}` snapshot the *first* time a `(Universe, IP, Port)` key was seen; on every later `Configure` call (every `artnet configure`/`target enable`/`target disable` reconfigure), an already-tracked key's stored `Target` -- including its `Enabled` flag -- was left untouched. This meant `artnet target disable` succeeded (`ExitCode 0`) but a subsequent `artnet status` always reported the target's original `Enabled` value from first configuration, never the toggled one -- directly breaking this task's own required "target disable then status shows disabled" behavior.
- **Fix:** `Configure` now refreshes `Universe`/`Target` on every call (whether or not the key was already tracked), leaving `SendOK`/`SendErr`/`Reachable`/`LastError` untouched so historical counters still survive a reconfigure exactly as the existing doc comment promises.
- **Files modified:** internal/artnet/health.go
- **Verification:** `TestArtnetTargetEnableDisableRoundTrip` (disable then enable, asserting `Target.Enabled` in status after each) and the full `internal/artnet` test suite (health_test.go) all pass.
- **Committed in:** `12af3ed` (Task 2 commit)

---

**Total deviations:** 2 auto-fixed (1 circular-dependency fix, 1 bug fix).
**Impact on plan:** Both fixes were necessary for this plan's own stated acceptance criteria to pass (a real build-breaking cycle, and a real functional bug in the enable/disable round trip); no architectural change, no scope creep beyond the two already-committed 04-04 packages this plan's CLI must call into.

## Known Limitations

- **`artnet serve`'s fixture resolution requires an explicit `--fixtures <dir>` flag.** This repository has no fixture-store/lookup service anywhere yet (verified: no code path resolves a `pool.PoolMember`'s `FixtureStableKey` to a `fixture.FixtureDefinition` except by reading an explicit file path, e.g. `pool substitute --from/--to`). Building such a service is a larger architectural addition outside this plan's declared file scope (`internal/command/artnet.go`/`artnet_test.go` only). `artnet serve` without `--fixtures` is safe when no deployment is active (the common case before a rig is patched) and fails loudly with `GOLC_ARTNET_FIXTURES_DIR_REQUIRED` per-instance rather than silently guessing (mirrors D-17's convention) once an active deployment's instances need real DMX channel data.
- **`artnet interface list` does not surface the daemon's live pinned-interface status.** CONTEXT D-05 describes a lost pinned interface surfacing as a degraded/error status; `artnet interface list` enumerates OS candidates directly (satisfying ARTN-01's literal requirement), but 04-04's own committed daemon status payload (`statusPayload{Frame, Targets}`) does not include interface health, so there is no IPC route today that surfaces the daemon's live `InterfaceManager.Status()`. Fixing this would require adding a field to 04-04's `statusPayload`/`handleStatus` (outside this plan's file scope) -- flagged here for a future plan rather than silently left undocumented.

## Issues Encountered
None beyond the two deviations documented above.

## User Setup Required

None - no external service configuration required. `golc test --quick --scope artnet` requires the project's pinned Go toolchain to be bootstrapped (`.tools/toolchains/go/...`, via `golc.ps1 bootstrap`) to actually execute -- this worktree has not run that bootstrap step, which is a pre-existing environment condition unrelated to this plan; the `TestScopeArtnet` marker itself was independently confirmed discoverable via `go test -list '^TestScopeArtnet$'` using the ambient toolchain.

## Next Phase Readiness
- Operators can now serve, configure, inspect (plain/JSON/watch), and enable/disable Art-Net targets entirely from `golc artnet` (D-01/D-02, ARTN-01/02/05, D-12) against Plan 04's daemon.
- `internal/artnet/ipc`'s local `Request`/`Result`/`Handler` types (this plan's `types.go`) are now the wire-type surface any future client (e.g. Phase 6's Wails app) should convert to/from at its own call boundary, exactly as `internal/command/artnet.go` does here -- neither `internal/artnet` nor `internal/artnet/ipc` import `internal/command`.
- Two known, explicitly documented gaps remain for a future plan: a real fixture-store/lookup service (so `artnet serve` never needs an operator-supplied `--fixtures` directory), and exposing the daemon's live pinned-interface health through the status IPC route.
- No blockers for the remaining phase 4 plans.

---
*Phase: 04-observable-art-net-live-output*
*Completed: 2026-07-22*
