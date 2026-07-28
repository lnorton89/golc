---
phase: 04-observable-art-net-live-output
plan: 06
subsystem: artnet
tags: [artnet, discovery, security, cli, go]

# Dependency graph
requires:
  - phase: 04-observable-art-net-live-output
    plan: 05
    provides: internal/command/artnet.go's "artnet" scope/route self-registration model and two-tier arg-parsing convention (parseArtnetArgs), which this plan's "artnet discover" route reuses unchanged
  - phase: 04-observable-art-net-live-output
    plan: 02
    provides: internal/artnet/interfacemgr.go's InterfaceInfo/ListCandidateInterfaces and the unexported addrIP helper, which this plan's Discover/localIPv4FromInterfaceInfo reuse directly (same package)
provides:
  - internal/artnet/packet.go -- EncodeArtPoll/DecodeArtPollReply (opPoll=0x2000, opPollReply=0x2100), added additively to the existing ArtDMX codec, with strict bounds-checked parsing of untrusted ArtPollReply fields (GOLC_ARTNET_POLLREPLY_INVALID)
  - internal/artnet/discovery.go -- DiscoveredNode + Discover(ctx, iface, window), a bounded ArtPoll broadcast/ArtPollReply collection scan returning suggestions only (CONTEXT D-06)
  - internal/command/artnet.go -- "artnet discover --interface <index> [--window <duration>] [--json]" route, rendering suggestions with no target-mutation path
affects: []

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "discoverListenAndSend package-level function-variable seam (mirrors worker.go's dialFunc/artNetSender pattern): production wiring opens a real net.ListenUDP-backed conn and best-effort broadcasts the ArtPoll (a send failure does not fail Discover, since the collect loop still runs the full window and returns a well-formed empty list either way); discovery_test.go overrides the var with a real loopback UDP responder, proving ArtPollReply collection against genuine UDP sockets without depending on OS-level SO_BROADCAST permission semantics, which vary by platform and are not portably testable in a unit test."
    - "'artnet discover' is a direct OS/network operation (no daemon dial), exactly like Plan 05's 'artnet interface list' -- it resolves --interface via artnet.ListCandidateInterfaces() and calls artnet.Discover() in-process, never touching the running daemon's target/state map, structurally guaranteeing it cannot mutate a live target."
    - "ArtPollReply's declared port count (NumPortsLo) is the one variable/count-like field in the real Art-Net wire format; it is bounds-checked against a hard ceiling (artPollReplyMaxPorts=4) before indexing the fixed-4-element SwOut array, mirroring internal/fixture/decode.go's 'never trust a length field from the wire' discipline for this phase's own untrusted-network-input path."

key-files:
  created:
    - internal/artnet/discovery.go
    - internal/artnet/discovery_test.go
  modified:
    - internal/artnet/packet.go
    - internal/artnet/packet_test.go
    - internal/command/artnet.go
    - internal/command/artnet_test.go

key-decisions:
  - "ArtPollReply is decoded against a real, spec-shaped 239-byte fixed-offset layout (id, opcode, IP, NetSwitch/SubSwitch, ShortName[18]/LongName[64] null-terminated fields, NumPorts, SwOut[4]) rather than a simplified ad hoc shape -- this keeps the wire format genuinely Art-Net-compatible (not merely internally self-consistent) while still only exposing the fields this plan's must_haves require (IP, short/long name, Port-Address(es))."
  - "Discover's initial ArtPoll broadcast send is best-effort: discoverListenAndSend's production implementation does not explicitly set the SO_BROADCAST socket option (Go's net.UDPConn exposes no portable way to do so without dropping to platform-specific syscalls), so a real deployment's broadcast send may fail with an OS permission error on some platforms/configurations. This is deliberately non-fatal -- Discover still returns a well-formed (possibly empty) list after window elapses either way, satisfying the plan's own explicit backstop requirement (zero/malformed replies never error the daemon). Flagged here as a known limitation for real-hardware verification (ARTN-06), not silently pretended solved."
  - "Task 1's ArtPoll/ArtPollReply test names were renamed (TestEncodeArtPollGoldenVector -> TestArtPollEncodeGoldenVector, etc.) after discovering the plan's own <verify> command (`-run 'TestArtPoll|TestPollReply|...'`) did not match the originally-written names as contiguous substrings -- renamed so the phase's own literal quality-gate command actually exercises these tests, with no behavior change (see Deviations)."

requirements-completed: [ARTN-02]

coverage:
  - id: D1
    description: "EncodeArtPoll matches a golden vector; DecodeArtPollReply parses a good reply's IP/name/port-address"
    requirement: "ARTN-02"
    verification:
      - kind: unit
        ref: "internal/artnet/packet_test.go#TestArtPollEncodeGoldenVector"
        status: pass
      - kind: unit
        ref: "internal/artnet/packet_test.go#TestArtPollReplyDecodeGoodVector"
        status: pass
    human_judgment: false
  - id: D2
    description: "Every malformed ArtPollReply input (empty, short header, wrong id, wrong opcode, oversized declared port count) returns GOLC_ARTNET_POLLREPLY_INVALID without panic, and reply length/count fields are bounds-checked before indexing (Security Domain V5, T-04-01)"
    requirement: "ARTN-02"
    verification:
      - kind: unit
        ref: "internal/artnet/packet_test.go#TestArtPollReplyDecodeMalformed"
        status: pass
    human_judgment: false
  - id: D3
    description: "Discover against a real loopback UDP responder returns the node as a suggestion; a malformed reply is skipped and the scan still returns a well-formed list; zero replies returns a non-nil empty list"
    requirement: "ARTN-02"
    verification:
      - kind: unit
        ref: "internal/artnet/discovery_test.go#TestDiscoverReturnsGoodNodeAsSuggestion"
        status: pass
      - kind: unit
        ref: "internal/artnet/discovery_test.go#TestDiscoverSkipsMalformedReply"
        status: pass
      - kind: unit
        ref: "internal/artnet/discovery_test.go#TestDiscoverZeroRepliesReturnsEmptyList"
        status: pass
    human_judgment: false
  - id: D4
    description: "'artnet discover' performs no target add/remove/modify -- the daemon's configured target set (including Enabled state) is unchanged before and after a scan; unknown/missing --interface fails clearly"
    requirement: "ARTN-02"
    verification:
      - kind: unit
        ref: "internal/command/artnet_test.go#TestArtnetDiscoverPerformsNoTargetMutation"
        status: pass
      - kind: unit
        ref: "internal/command/artnet_test.go#TestArtnetDiscoverUsageErrors"
        status: pass
      - kind: unit
        ref: "internal/command/artnet_test.go#TestArtnetDiscoverUnknownInterfaceReturnsNotFound"
        status: pass
      - kind: unit
        ref: "internal/command/artnet_test.go#TestArtnetDiscoverRendersSuggestions"
        status: pass
    human_judgment: false

duration: ~40min
completed: 2026-07-22
status: complete
---

# Phase 4 Plan 06: Observable Art-Net Node Discovery Summary

**ArtPoll/ArtPollReply codec plus a bounded, suggestion-only discovery scan (`golc artnet discover`) that never auto-adds a live unicast target -- completing ARTN-02.**

## Performance

- **Duration:** ~40 min
- **Started:** ~2026-07-22T08:55:00Z
- **Completed:** 2026-07-22T09:35:28Z
- **Tasks:** 2
- **Files modified:** 6 (2 created, 4 modified)

## Accomplishments

- `internal/artnet/packet.go` gains `EncodeArtPoll()` and `DecodeArtPollReply([]byte) (ArtPollReply, error)` (`opPoll=0x2000`, `opPollReply=0x2100`), added additively beside the existing `EncodeArtDMX` codec. `DecodeArtPollReply` decodes a real, spec-shaped 239-byte ArtPollReply layout (id, opcode, IP, NetSwitch/SubSwitch, null-terminated ShortName/LongName, declared port count, SwOut ports) and bounds-checks buffer length, id, opcode, and the declared port count (against a hard ceiling of 4) before any indexing or allocation -- every malformed/truncated/spoofed-shaped input returns `GOLC_ARTNET_POLLREPLY_INVALID` rather than panicking or reading out of bounds (Security Domain V5, T-04-01).
- `internal/artnet/discovery.go` implements `Discover(ctx, iface, window) ([]DiscoveredNode, error)`: broadcasts one ArtPoll on the interface's own computed local-subnet broadcast address, then collects ArtPollReply datagrams for a bounded window (default 3s per the Art-Net spec's own guidance), skipping malformed replies and de-duplicating by IP. Zero or malformed replies never error the scan -- Discover always returns a well-formed, non-nil (possibly empty) slice, satisfying this plan's explicit backstop requirement.
- `internal/command/artnet.go` adds `artnet discover --interface <index> [--window <duration>] [--json]`: resolves the interface directly via `artnet.ListCandidateInterfaces()` (no daemon round trip, exactly like Plan 05's `artnet interface list`) and renders results as suggestions only. It never dials the running daemon and never calls any configure/enable/disable path -- there is no "add all discovered nodes" bulk-apply anywhere in this file, so promoting a suggestion to a live unicast target always requires a separate, explicit `artnet configure` invocation (CONTEXT D-06).
- Introduced `discoverListenAndSend`, a package-level function-variable seam mirroring `worker.go`'s `dialFunc`/`artNetSender` pattern: production wiring opens a real UDP conn and best-effort broadcasts; `discovery_test.go` overrides the var with a real loopback UDP responder (a genuine `*net.UDPConn` sending real datagrams), proving ArtPollReply collection end-to-end without depending on OS-level `SO_BROADCAST` permission semantics, which vary by platform and are not portably testable in a unit test.

## Task Commits

Each task was committed atomically:

1. **Task 1: ArtPoll/ArtPollReply codec with strict reply-field bounds checks (ARTN-02, Security V5)** - `d62b28e` (feat)
2. **Task 2: Bounded discovery scan + `golc artnet discover` as suggestions only (D-06/D-07)** - `8ac2da7` (feat)
3. **Fix: renamed ArtPoll/ArtPollReply test names to match the plan's own `<verify>` regex** - `98040d9` (fix, Rule 1)

**Plan metadata:** (recorded in final commit)

## Files Created/Modified

- `internal/artnet/packet.go` - `EncodeArtPoll`/`DecodeArtPollReply`, `ArtPollReply` type, `opPoll`/`opPollReply`/`artPollReplyMinLen`/`artPollReplyMaxPorts` constants
- `internal/artnet/packet_test.go` - golden ArtPoll encode vector, good ArtPollReply decode, malformed-input table (empty/short/wrong id/wrong opcode/oversized port count)
- `internal/artnet/discovery.go` - `DiscoveredNode`, `Discover`, `discoverListenAndSend` seam, local-IP/broadcast-address helpers
- `internal/artnet/discovery_test.go` - real loopback UDP responder tests for good/malformed/zero-reply scans
- `internal/command/artnet.go` - `artnet discover` route, arg parsing, interface resolution, plain/`--json` rendering
- `internal/command/artnet_test.go` - usage/domain-error coverage, no-target-mutation proof, route self-registration list updated

## Decisions Made

- ArtPollReply is decoded against the real, spec-shaped 239-byte fixed-offset Art-Net layout, not a simplified custom shape -- keeps the wire format genuinely Art-Net-compatible while exposing only the fields this plan needs (IP, short/long name, Port-Address(es)).
- Discover's ArtPoll broadcast send is best-effort (no explicit `SO_BROADCAST` socket-option wiring, which Go's stdlib `net.UDPConn` does not expose portably without platform-specific syscalls); a send failure never fails Discover -- it still returns a well-formed list after the window elapses, matching the plan's own backstop requirement. See Known Limitations.
- Renamed Task 1's ArtPoll/ArtPollReply test functions so the plan's own literal `<verify>` regex (`TestArtPoll|TestPollReply|...`) actually matches and exercises them (see Deviations).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Task 1's own `<verify>` regex did not match its originally-written test names**
- **Found during:** Running this plan's own `<verify>` command after both tasks were committed
- **Issue:** The plan's `<verify>` block specifies `go test ... -run 'TestArtPoll|TestPollReply|TestDiscover|TestArtnetDiscover'`. The originally-written test names (`TestEncodeArtPollGoldenVector`, `TestDecodeArtPollReplyGoodVector`, `TestDecodeArtPollReplyMalformed`) do not contain either `TestArtPoll` or `TestPollReply` as a *contiguous* substring (`Test` is followed by `Encode`/`Decode` before `ArtPoll`/`PollReply`), so `go test -run` silently skipped them even though they passed when run directly -- the phase's own declared quality gate was not actually exercising this task's tests.
- **Fix:** Renamed to `TestArtPollEncodeGoldenVector`, `TestArtPollReplyDecodeGoodVector`, `TestArtPollReplyDecodeMalformed` -- each now starts with the literal `TestArtPoll` substring the verify regex matches. No test behavior changed, only the function names.
- **Files modified:** internal/artnet/packet_test.go
- **Verification:** `go test ./internal/artnet/... ./internal/command/... -run 'TestArtPoll|TestPollReply|TestDiscover|TestArtnetDiscover' -count=1` now runs and passes all 4 ArtPoll/ArtPollReply tests plus the Discover/artnet-discover tests (9 top-level tests, all green).
- **Committed in:** `98040d9`

## Known Limitations

- **Discover's ArtPoll broadcast send does not explicitly enable the `SO_BROADCAST` socket option.** Go's standard-library `net.UDPConn` exposes no portable way to set this without dropping to platform-specific syscalls (a new dependency/complexity this plan's file scope does not cover). On some OS/network configurations, sending a UDP datagram to a broadcast destination without this option set returns a permission error; `discoverListenAndSend`'s production implementation treats this as best-effort and ignores the send error, so `Discover` still runs its full collection window and returns a well-formed (possibly empty) list either way -- this satisfies the plan's own explicit backstop requirement (zero/malformed replies never error the daemon), but means real broadcast delivery on an actual multi-device Art-Net network is unverified in this plan's own automated tests (which use a direct-loopback-responder seam specifically to avoid depending on this OS behavior). Flagged for ARTN-06's hardware/simulator verification pass, where this should be confirmed against a real interface and, if the permission error is hit in practice, addressed with a platform-specific `SO_BROADCAST` implementation.
- **`golc test --quick --scope artnet` could not be executed end-to-end in this worktree** (same pre-existing condition already documented in `04-05-SUMMARY.md`): it requires the project's pinned Go toolchain to be bootstrapped under `.tools/toolchains/go/...` via `golc.ps1 bootstrap`, which has not been run in this worktree. Independently confirmed instead: `go test ./internal/command/... -list '^TestScopeArtnet$'` finds the marker, and `go test ./internal/command/... -run '^TestScopeArtnet$' -v` passes all 5 subtests (including the updated route-registration list covering `artnet discover`) using the ambient (non-pinned) ` go1.26.5 windows/amd64` toolchain already on this machine.

## Issues Encountered

None beyond the one deviation documented above.

## User Setup Required

None - no external service configuration required. See Known Limitations above for the pre-existing toolchain-bootstrap gap (unrelated to this plan) affecting only the `golc test --quick --scope artnet` wrapper command, not the underlying `go test` suite.

## Next Phase Readiness

- ARTN-02 is now fully complete: operators can configure universes/static unicast targets (Plan 02/05), toggle them online/offline (Plan 05), and now also discover compatible nodes on a pinned interface as suggestions (`golc artnet discover`), with promotion to a live target always remaining a separate, explicit `artnet configure` action (CONTEXT D-06).
- `internal/artnet.DiscoveredNode`/`Discover` are exported and available for a future phase's UI (e.g. Phase 6's Wails app) to build a richer "click to add" discovery experience on top of, without needing to touch the underlying ArtPoll/ArtPollReply codec.
- The `SO_BROADCAST` limitation documented above is the one open item worth revisiting before ARTN-06's real-hardware verification pass; it does not block this phase's remaining plans, since Discover's zero-reply backstop behavior is already correct and tested regardless of whether the broadcast send itself succeeds.
- No blockers for the remaining phase 4 plans.

## Self-Check: PASSED
- FOUND: internal/artnet/discovery.go
- FOUND: internal/artnet/discovery_test.go
- FOUND: internal/artnet/packet.go (modified)
- FOUND: internal/artnet/packet_test.go (modified)
- FOUND: internal/command/artnet.go (modified)
- FOUND: internal/command/artnet_test.go (modified)
- FOUND commit: d62b28e (Task 1)
- FOUND commit: 8ac2da7 (Task 2)
- FOUND commit: 98040d9 (fix: test rename)

---
*Phase: 04-observable-art-net-live-output*
*Completed: 2026-07-22*
