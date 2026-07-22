---
phase: 04-observable-art-net-live-output
plan: 04
subsystem: artnet
tags: [artnet, ipc, named-pipe, go-winio, daemon, concurrency, go]

# Dependency graph
requires:
  - phase: 04-observable-art-net-live-output
    plan: 01
    provides: internal/artnet.EncodeArtDMX/PortAddress (byte-exact ArtDMX codec)
  - phase: 04-observable-art-net-live-output
    plan: 02
    provides: internal/artnet.Target/ValidateTarget/ValidateUniqueTargets/SetEnabled, internal/artnet.InterfaceManager/ListCandidateInterfaces
  - phase: 04-observable-art-net-live-output
    plan: 03
    provides: internal/artnet.Worker/NewWorker/Start/Stop (non-blocking send loop reading playback.Engine.CurrentFrame), internal/artnet.Health/NewHealth/Snapshot (frame/target health model)
provides:
  - internal/artnet/ipc package -- NewListener/Serve (ACL-restricted named-pipe server, length-prefixed strictjson-canonical framing) and Dial/Forward (thin client), reusing internal/command.Request/Result as the wire shape
  - internal/artnet.Run(ctx, Config) -- the long-lived, standalone-capable daemon (D-03/D-04): constructs/starts the playback Engine, pinned InterfaceManager, and Worker, then serves the IPC listener; handler dispatches "artnet status"/"artnet configure"/"artnet target enable"/"artnet target disable" to daemon-owned in-memory state
affects: [04-05, 04-06, 04-07]

# Tech tracking
tech-stack:
  added: ["github.com/Microsoft/go-winio v0.6.2 (pinned via go list -m -versions + go mod tidy)"]
  patterns:
    - "Named-pipe request/response framed with a 4-byte big-endian length prefix (writeFrame/readFrame) since go-winio's byte-mode pipes have no message boundary of their own and no in-repo framing precedent existed to copy"
    - "ipc.NewListener/Dial take an explicit pipeName parameter rather than a hardcoded constant, so tests (and this plan's own daemon) can use an isolated per-test/per-instance pipe path without colliding on the shared production ipc.PipeName across concurrently-running test packages"
    - "Daemon-side configure/enable/disable mutations apply via a full stop/rebuild/start of the Worker (worker.go exposes no dynamic reconfigure API by design) while reusing the same *Health instance across the rebuild, so historical send counts/reachability survive a reconfigure instead of resetting to zero"
    - "GOLC_ARTNET_* diagnostic convention extended: GOLC_ARTNET_IPC_LISTEN_FAILED, GOLC_ARTNET_IPC_ACCEPT_FAILED, GOLC_ARTNET_IPC_DECODE_FAILED, GOLC_ARTNET_IPC_ENCODE_FAILED, GOLC_ARTNET_IPC_WRITE_FAILED, GOLC_ARTNET_DAEMON_UNREACHABLE, GOLC_ARTNET_DAEMON_ENGINE_FAILED, GOLC_ARTNET_DAEMON_IPC_LISTEN_FAILED, GOLC_ARTNET_ROUTE_UNKNOWN, GOLC_ARTNET_STATUS_ENCODE_FAILED, GOLC_ARTNET_USAGE"

key-files:
  created:
    - internal/artnet/ipc/server.go
    - internal/artnet/ipc/client.go
    - internal/artnet/ipc/ipc_test.go
    - internal/artnet/daemon.go
    - internal/artnet/daemon_test.go
  modified:
    - go.mod
    - go.sum

key-decisions:
  - "ipc.NewListener/Dial require an explicit pipeName parameter instead of the plan's literally-worded zero-arg Dial() hardcoded to a single constant -- a hardcoded production pipe name would make this package's own tests and internal/artnet's daemon tests collide on the same named pipe when go test runs packages in parallel (the default). ipc.PipeName remains exported as the production default for Plan 05's future CLI routes to pass explicitly."
  - "Named-pipe connections are framed with a 4-byte big-endian length prefix (writeFrame/readFrame) rather than relying on EOF/CloseWrite: go-winio's byte-mode pipe (the default, non-message-mode) does not implement a write-side half-close, so length-prefixing is the only reliable way to bound exactly one request and one response per connection."
  - "The daemon applies 'artnet configure'/'artnet target enable|disable' by stopping the current Worker and starting a fresh one built from the updated target map (passing the same *Health instance through WorkerConfig.Health) rather than adding a dynamic reconfigure method to worker.go, which is outside this plan's file scope (files_modified: only ipc/server.go, ipc/client.go, ipc_test.go, daemon.go, daemon_test.go, go.mod, go.sum)."
  - "Defined a local, JSON-safe statusPayload type in daemon.go (Frame + a sorted []TargetHealth) rather than encoding HealthSnapshot directly or modifying health.go's exported Snapshot() shape (health.go is also outside this plan's file scope) -- see Deviations #1."

requirements-completed: [ARTN-04]

coverage:
  - id: D1
    description: "A local, ACL-restricted (owner-only SDDL) Windows named-pipe IPC carries command.Request/command.Result between CLI clients and the daemon, never binding a routable/TCP address; a Request round-trips to a Result unchanged"
    requirement: "ARTN-04"
    verification:
      - kind: unit
        ref: "internal/artnet/ipc/ipc_test.go#TestIPCRequestRoundTripsToResult"
        status: pass
      - kind: unit
        ref: "internal/artnet/ipc/ipc_test.go#TestOwnerOnlySDDLRestrictsToOwner"
        status: pass
    human_judgment: false
  - id: D2
    description: "A dial to a nonexistent/unreachable daemon pipe returns GOLC_ARTNET_DAEMON_UNREACHABLE fast, never a hang or a raw dial error"
    requirement: "ARTN-04"
    verification:
      - kind: unit
        ref: "internal/artnet/ipc/ipc_test.go#TestIPCDialNonexistentPipeReturnsDaemonUnreachable"
        status: pass
    human_judgment: false
  - id: D3
    description: "go.mod pins a concrete, verified github.com/Microsoft/go-winio version (v0.6.2, current tag per go list -m -versions) and go mod tidy leaves the tree clean"
    requirement: "ARTN-04"
    verification:
      - kind: other
        ref: "go mod tidy (no diff produced against the committed go.mod/go.sum)"
        status: pass
    human_judgment: false
  - id: D4
    description: "Run starts the playback Engine, the pinned InterfaceManager, and the Worker (against Engine.CurrentFrame), then serves the IPC listener end-to-end in-process: an 'artnet status' Request returns a health snapshot Result"
    requirement: "ARTN-04"
    verification:
      - kind: unit
        ref: "internal/artnet/daemon_test.go#TestDaemonRunServesStatusAndShutsDownCleanly"
        status: pass
    human_judgment: false
  - id: D5
    description: "Context cancel triggers ordered shutdown (worker, interface poll, engine, IPC listener) with no goroutine leak; an unrecognized route is rejected rather than silently succeeding"
    requirement: "ARTN-04"
    verification:
      - kind: unit
        ref: "internal/artnet/daemon_test.go#TestDaemonRunServesStatusAndShutsDownCleanly (cleanup deadline asserts Run returns within 5s of cancel)"
        status: pass
      - kind: unit
        ref: "internal/artnet/daemon_test.go#TestDaemonUnknownRouteReturnsRouteUnknown"
        status: pass
    human_judgment: false
  - id: D6
    description: "The daemon is the single owner of worker/target/interface state (D-03): 'artnet configure' adds/updates a fan-out target and 'artnet target enable|disable' toggles one target without touching the rest of the rig, via a stop/rebuild/start of the Worker; an unknown target selector fails with GOLC_ARTNET_TARGET_NOT_FOUND; malformed args fail as GOLC_ARTNET_USAGE"
    requirement: "ARTN-04"
    verification:
      - kind: unit
        ref: "internal/artnet/daemon_test.go#TestDaemonConfigureThenTargetDisableEnable"
        status: pass
      - kind: unit
        ref: "internal/artnet/daemon_test.go#TestDaemonMalformedConfigureArgsReturnUsageError"
        status: pass
    human_judgment: false

duration: 22min
completed: 2026-07-22
status: complete
---

# Phase 4 Plan 04: Local IPC Bridge & Standalone Daemon Summary

**ACL-restricted Windows named-pipe IPC (go-winio, length-prefixed strictjson framing) carrying command.Request/Result between short-lived `golc artnet` CLI clients and one long-lived headless daemon (engine+worker+interface manager+IPC listener) that owns all Art-Net worker/target/interface state.**

## Performance

- **Duration:** ~22 min
- **Started:** 2026-07-22T08:28:11Z
- **Completed:** 2026-07-22T08:50:08Z
- **Tasks:** 2
- **Files modified:** 7 (5 created, 2 modified: go.mod/go.sum)

## Accomplishments
- `internal/artnet/ipc/server.go`: `NewListener(pipeName)` opens a go-winio named pipe with an owner-only security descriptor (`D:P(A;;GA;;;OW)`, Security Domain V4) and never binds a routable/TCP address by construction (the pipe transport makes that structurally impossible). `Serve(ctx, listener, handler)` accepts connections until `ctx` is cancelled, decoding one length-prefixed `command.Request` per connection (`strictjson.DecodeStrict`), dispatching to an injected `command.CommandHandler`, and writing back a length-prefixed `command.Result` (`strictjson.CanonicalEncode`).
- `internal/artnet/ipc/client.go`: `Dial(pipeName)` surfaces an unreachable daemon as `GOLC_ARTNET_DAEMON_UNREACHABLE` (proven to fail fast, not hang) rather than a raw dial error; `Forward(conn, request)` marshals the Request and returns the decoded Result, reporting any transport/decode failure as an `ExitCode:1` Result.
- Added `github.com/Microsoft/go-winio` at v0.6.2 (the current tag per `go list -m -versions`), the one new external dependency this phase's RESEARCH.md approved (verdict OK/Approved, Microsoft-maintained); `go mod tidy` leaves the tree clean.
- `internal/artnet/daemon.go`: `Run(ctx, Config)` is the long-lived, standalone-capable process (D-03/D-04) -- it constructs and starts the playback `Engine`, the pinned `InterfaceManager`, and the `Worker` (against `Engine.CurrentFrame`), then serves the IPC listener until `ctx` is cancelled, after which it stops the worker, interface poll, and engine (the IPC listener itself closes first, as part of `Serve`'s own `ctx.Done` handling). `Run`'s own import graph never reaches Wails/UI, so it is genuinely headless.
- The daemon's IPC handler dispatches `"artnet status"` (renders the current `Health` snapshot), `"artnet configure"` (adds/updates one D-08 fan-out target, validated via Plan 02's `ValidateTarget`/`ValidateUniqueTargets`), and `"artnet target enable"`/`"artnet target disable"` (D-12 per-target toggle via Plan 02's copy-returning `SetEnabled`) -- every mutation applies through a stop/rebuild/start of the Worker, since `worker.go` has no dynamic reconfigure API by design; the same `*Health` instance is threaded through every rebuild so historical send counts/reachability survive a reconfigure.

## Task Commits

Each task was committed atomically:

1. **Task 1: Local named-pipe IPC server + client reusing command.Request/Result (D-03/D-04)** - `da2af85` (feat)
2. **Task 2: Long-lived standalone daemon wiring engine + worker + interface + IPC (D-03/D-04)** - `5a03964` (feat)

**Plan metadata:** (recorded in final commit)

## Files Created/Modified
- `internal/artnet/ipc/server.go` - `PipeName`, `NewListener`, `Serve`, `handleConn`, `writeResult`, `writeFrame`/`readFrame` (length-prefixed framing), owner-only SDDL constant
- `internal/artnet/ipc/client.go` - `Dial`, `Forward`, `dialTimeout`
- `internal/artnet/ipc/ipc_test.go` - round-trip, dial-unreachable, owner-only-SDDL sanity, oversized-frame-rejection tests
- `internal/artnet/daemon.go` - `Config`, `daemon` (unexported), `Run`, `handle`/`handleStatus`/`handleConfigure`/`handleSetEnabled`, `statusPayload`/`newStatusPayload`, `parseFlags`/`parseTargetSelector`
- `internal/artnet/daemon_test.go` - status round trip + clean shutdown, unknown-route rejection, configure/enable/disable + not-found, malformed-args-usage-error tests
- `go.mod`/`go.sum` - added `github.com/Microsoft/go-winio v0.6.2` (+ transitive `golang.org/x/sys`)

## Decisions Made
- `ipc.NewListener`/`Dial` take an explicit `pipeName` parameter instead of a hardcoded constant, so this package's own tests and `internal/artnet`'s daemon tests never collide on the same named pipe when `go test` parallelizes across packages (the default) -- `ipc.PipeName` remains the exported production default for Plan 05's future CLI routes.
- Named-pipe connections are framed with a 4-byte big-endian length prefix rather than relying on EOF/`CloseWrite`, since go-winio's default byte-mode pipe has no write-side half-close.
- Daemon-side configure/enable/disable apply via a full Worker stop/rebuild/start (reusing the same `*Health` instance) rather than adding a dynamic reconfigure method to `worker.go`, which sits outside this plan's declared file scope.
- `statusPayload` (a new local type in `daemon.go`) flattens `HealthSnapshot.Targets` into a sorted `[]TargetHealth` before JSON encoding rather than encoding the raw map or modifying `health.go` (see Deviations #1) -- both keep this plan's changes confined to its own declared files.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `HealthSnapshot` cannot be JSON-encoded directly -- `handleStatus` would always fail**
- **Found during:** Task 2 (wiring the `"artnet status"` handler's `strictjson.CanonicalEncode` call)
- **Issue:** `health.go`'s `HealthSnapshot.Targets` field is `map[targetKey]TargetHealth`, where `targetKey` is an unexported struct with no `encoding.TextMarshaler` implementation. Go's `encoding/json` can only marshal a map whose key type is a string, an integer kind, or implements `TextMarshaler` -- attempting to marshal a struct-keyed map fails with `json: unsupported type: map[artnet.targetKey]artnet.TargetHealth` (confirmed via a scratch reproduction before writing the fix). Since Task 2's own acceptance criteria require `"artnet status"` to serve a health snapshot Result end-to-end, this would have made the plan's primary success criterion fail on every invocation.
- **Fix:** Added a local `statusPayload` type (`Frame FrameHealth`, `Targets []TargetHealth`) and `newStatusPayload(snapshot)` in `daemon.go`, which flattens the map into a slice sorted by `(Universe, IP, Port)` for deterministic output -- no information is lost, since each `TargetHealth` value already carries its own `Universe` and `Target` fields (the exact data `targetKey` packs). `handleStatus` now encodes `newStatusPayload(...)` instead of the raw snapshot. `health.go`/`target.go` (out of this plan's file scope) were left untouched.
- **Files modified:** internal/artnet/daemon.go
- **Verification:** `TestDaemonRunServesStatusAndShutsDownCleanly` asserts a non-empty, successfully-encoded health snapshot (`ExitCode 0`, JSON containing `OnCadence`) comes back over the real IPC pipe.
- **Committed in:** `5a03964` (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 bug fix).
**Impact on plan:** Necessary for Task 2's own stated acceptance criterion ("serves a status/health Request end-to-end in-process") to actually pass; no architectural change, no scope creep, no files touched outside daemon.go.

## Issues Encountered
None beyond the deviation documented above.

## User Setup Required

None - no external service configuration required. (Wireshark/OLA acceptance environment setup remains ARTN-06 scope, unrelated to this plan.)

## Next Phase Readiness
- `internal/artnet/ipc` and `internal/artnet.Run`/`Config` are ready for Plan 05's `golc artnet` CLI routes: every client route is expected to `ipc.Dial(ipc.PipeName)` then `ipc.Forward(conn, request)`, and `artnet serve` is expected to call `artnet.Run(ctx, cfg)` directly (the one route that *is* the server, not a client).
- **Architecture note for Plan 05:** `internal/artnet/ipc` (and `internal/artnet/daemon.go`, via its handler signature) import `internal/command` for the `Request`/`Result`/`CommandHandler` wire types, per this plan's own locked key_link ("Reuses command.Request/command.Result as the wire shape, Pattern 5"). When Plan 05 adds `internal/command/artnet.go` and it imports `internal/artnet/ipc` (for `Dial`/`Forward`) and/or `internal/artnet` (for `Run`), that creates a two-package import cycle (`command` -> `artnet/ipc` -> `command`) that will fail to compile. Plan 05's author will need to resolve this -- e.g. by not importing `internal/artnet/ipc` directly from `internal/command` (routing through some indirection), or another restructuring; this was flagged during Plan 04 research/execution but resolving it is outside Plan 04's own file scope (`internal/artnet/ipc`, `internal/artnet/daemon.go`, `go.mod`/`go.sum` only).
- The daemon's wire contract for `"artnet configure"`/`"artnet target enable"`/`"artnet target disable"` (flags: `--universe`, `--ip`, `--port`, `--enabled` for configure) was authored in this plan (Plan 05's own PLAN.md does not specify wire-level Args shape) -- Plan 05's CLI-side arg parsing should construct/forward `command.Request.Args` in this exact `--flag value`/`--flag=value` shape so the daemon's existing `parseFlags`/`parseTargetSelector` in `daemon.go` can interpret it without further daemon-side changes.
- `internal/artnet.ListCandidateInterfaces()` (Plan 02) remains directly callable for Plan 05's `artnet interface list` route without needing a daemon round trip, per Plan 05's own PLAN.md action text.
- No blockers for Plan 05.

---
*Phase: 04-observable-art-net-live-output*
*Completed: 2026-07-22*
