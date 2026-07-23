---
status: complete
phase: 04-observable-art-net-live-output
source: [04-01-SUMMARY.md, 04-02-SUMMARY.md, 04-03-SUMMARY.md, 04-04-SUMMARY.md, 04-05-SUMMARY.md, 04-06-SUMMARY.md, 04-07-SUMMARY.md, 04-08-SUMMARY.md, 04-09-SUMMARY.md]
started: 2026-07-23T04:15:00Z
updated: 2026-07-23T04:16:00Z
---

## Current Test

[testing complete]

## Tests

### 1. fixture.Mode gains an ordered, validated D-16/D-17 DMX channel layout (ChannelSlot{Type, Occurrence}); missing/invalid layouts hard-reject at decode time for both hand-authored and OFL-imported fixtures.
expected: covered by internal/fixture/decode_test.go#TestChannelLayout, internal/fixture/ofl/normalize_test.go#TestNormalizeModeChannels
result: pass
source: automated
coverage_id: D1 (04-01)

### 2. internal/artnet.EncodeArtDMX produces a byte-exact Art-Net 4 ArtDMX packet (id, opcode, protocol version, seq, physical, Port-Address, length, data); PortAddress implements the locked universe-to-Port-Address mapping; sequence wraps 1->255->1 never emitting 0.
expected: covered by internal/artnet/packet_test.go#TestEncodeArtDMXGoldenVector, #TestEncodeArtDMXLengthRejections, #TestPortAddressDistinct, #TestSequenceNeverZero
result: pass
source: automated
coverage_id: D2 (04-01)

### 3. internal/artnet.Encode turns a playback.Frame's semantic per-instance attribute values into per-universe 512-byte DMX buffers, driven strictly by each instance's Mode.Channels declared order, with backstop blackout and loud missing-value diagnostics.
expected: covered by internal/artnet/channelmap_test.go#TestEncodeOffsetAndScaling, #TestEncodeTwoInstancesSharedBuffer, #TestEncodeBlackoutUniverse, #TestEncodeMissingChannelValueFails
result: pass
source: automated
coverage_id: D3 (04-01)

### 4. ListCandidateInterfaces enumerates OS network interfaces (Index/Name/Up/Addrs) for ARTN-01 selection; enumeration failure wraps as GOLC_ARTNET_INTERFACE_ENUM_FAILED.
expected: covered by internal/artnet/interfacemgr_test.go#TestInterfaceListCandidateInterfacesFindsLoopback
result: pass
source: automated
coverage_id: D1 (04-02)

### 5. InterfaceManager pins by net.Interface.Index (never Name), detects pinned-interface loss via its own independent 1Hz poll loop, and never auto-switches to another interface (D-05).
expected: covered by internal/artnet/interfacemgr_test.go#TestInterfaceManagerMarkLostTransitionsStatus, #TestInterfaceManagerBogusIndexLostAfterOnePollIteration
result: pass
source: automated
coverage_id: D2 (04-02)

### 6. LocalIP() returns the pinned interface's own local IPv4 address for future bind-by-address use (Pitfall 5: no SO_BINDTODEVICE on Windows).
expected: covered by internal/artnet/interfacemgr_test.go#TestInterfaceManagerLocalIPReturnsPinnedInterfaceIP, #TestInterfaceManagerLocalIPFailsForBogusIndex
result: pass
source: automated
coverage_id: D3 (04-02)

### 7. Target{Universe, IP, Port, Enabled} validates invalid universe/IP/port as GOLC_ARTNET_TARGET_INVALID, rejects the IPv4 broadcast address (D-07 unicast-only).
expected: covered by internal/artnet/target_test.go#TestTargetValidateTargetAcceptsValidTarget, #TestTargetValidateTargetRejectsBroadcastIP
result: pass
source: automated
coverage_id: D4 (04-02)

### 8. ValidateUniqueTargets rejects duplicate (Universe, IP, Port) triples (GOLC_ARTNET_TARGET_DUPLICATE) while explicitly allowing fan-out (D-08: same universe, multiple targets; same IP/Port, multiple universes).
expected: covered by internal/artnet/target_test.go#TestTargetValidateUniqueTargetsAcceptsFanOutSameUniverseDifferentIPs, #TestTargetValidateUniqueTargetsRejectsDuplicateTriple
result: pass
source: automated
coverage_id: D5 (04-02)

### 9. SetEnabled toggles per-target enable/disable, returning a fresh copy without mutating the caller's slice (D-12).
expected: covered by internal/artnet/target_test.go#TestTargetSetEnabledReturnsFreshSliceLeavingInputUnchanged
result: pass
source: automated
coverage_id: D6 (04-02)

### 10. artnet.Worker reads playback.Engine.CurrentFrame() on its own independent 40Hz ticker (never the engine's tick callback) and emits a byte-exact ArtDMX packet per configured universe to every enabled unicast target, with per-universe sequence advancing and never emitting 0.
expected: covered by internal/artnet/worker_test.go#TestWorkerLoopbackReceivesDecodableArtDMX, #TestWorkerSequenceAdvancesPerUniverseNeverZero
result: pass
source: automated
coverage_id: D1 (04-03)

### 11. A hung/slow target's send never delays the worker's next tick or backpressures the playback engine.
expected: covered by internal/artnet/worker_test.go#TestWorkerSlowTargetDoesNotStallHealthyTarget
result: pass
source: automated
coverage_id: D2 (04-03)

### 12. A disabled target receives zero packets while its universe's remaining enabled targets keep receiving on cadence (D-12); ctx cancel via Stop() ends the worker's tick goroutine cleanly.
expected: covered by internal/artnet/worker_test.go#TestWorkerDisabledTargetReceivesNothing, #TestWorkerStopEndsGoroutine
result: pass
source: automated
coverage_id: D3 (04-03)

### 13. artnet.Health classifies frame health as on-cadence vs stalled (GOLC_ARTNET_FRAME_STALLED), and per-target health accumulates send success/error counts with a reachable-vs-unreachable distinction (D-09/D-10).
expected: covered by internal/artnet/health_test.go#TestFrameHealthOnCadenceVsStalled, #TestHealthTargetSendAccumulatesAndDistinguishesReachability
result: pass
source: automated
coverage_id: D4 (04-03)

### 14. Health tracking is bounded to the explicitly configured target set (T-04-04); the published snapshot is lock-free; every recorded error emits a structured {DOMAIN}_{CONDITION} log line (D-11).
expected: covered by internal/artnet/health_test.go#TestHealthUnconfiguredTargetNeverTracked, #TestHealthSnapshotConcurrentWithRecordSendNoRace, #TestHealthRecordSendErrorEmitsStructuredLogLine
result: pass
source: automated
coverage_id: D5 (04-03)

### 15. A local, ACL-restricted (owner-only SDDL) Windows named-pipe IPC carries command.Request/command.Result between CLI clients and the daemon, never binding a routable/TCP address; a Request round-trips to a Result unchanged.
expected: covered by internal/artnet/ipc/ipc_test.go#TestIPCRequestRoundTripsToResult, #TestOwnerOnlySDDLRestrictsToOwner
result: pass
source: automated
coverage_id: D1 (04-04)

### 16. A dial to a nonexistent/unreachable daemon pipe returns GOLC_ARTNET_DAEMON_UNREACHABLE fast, never a hang or a raw dial error.
expected: covered by internal/artnet/ipc/ipc_test.go#TestIPCDialNonexistentPipeReturnsDaemonUnreachable
result: pass
source: automated
coverage_id: D2 (04-04)

### 17. go.mod pins a concrete, verified github.com/Microsoft/go-winio version and go mod tidy leaves the tree clean.
expected: covered by `go mod tidy` producing no diff
result: pass
source: automated
coverage_id: D3 (04-04)

### 18. Run starts the playback Engine, the pinned InterfaceManager, and the Worker, then serves the IPC listener end-to-end in-process: an 'artnet status' Request returns a health snapshot Result.
expected: covered by internal/artnet/daemon_test.go#TestDaemonRunServesStatusAndShutsDownCleanly
result: pass
source: automated
coverage_id: D4 (04-04)

### 19. Context cancel triggers ordered shutdown with no goroutine leak; an unrecognized route is rejected rather than silently succeeding.
expected: covered by internal/artnet/daemon_test.go#TestDaemonRunServesStatusAndShutsDownCleanly, #TestDaemonUnknownRouteReturnsRouteUnknown
result: pass
source: automated
coverage_id: D5 (04-04)

### 20. The daemon is the single owner of worker/target/interface state (D-03): 'artnet configure' and 'artnet target enable|disable' work without touching the rest of the rig; unknown/malformed input fails cleanly.
expected: covered by internal/artnet/daemon_test.go#TestDaemonConfigureThenTargetDisableEnable, #TestDaemonMalformedConfigureArgsReturnUsageError
result: pass
source: automated
coverage_id: D6 (04-04)

### 21. The artnet scope and the serve/interface-list/configure/status/target-enable/target-disable routes self-register.
expected: covered by internal/command/artnet_test.go#TestScopeArtnet
result: pass
source: automated
coverage_id: D1 (04-05)

### 22. Malformed 'artnet configure' args return GOLC_ARTNET_USAGE/ExitCode 2; a validated-but-rejected target value returns GOLC_ARTNET_TARGET_INVALID/ExitCode 1, both without ever dialing a daemon.
expected: covered by internal/command/artnet_test.go#TestArtnetConfigureUsageErrors, #TestArtnetConfigureInvalidTargetReturnsDomainError
result: pass
source: automated
coverage_id: D2 (04-05)

### 23. A client route with no daemon running on the given pipe returns GOLC_ARTNET_DAEMON_UNREACHABLE, ExitCode 1, never a hang.
expected: covered by internal/command/artnet_test.go#TestArtnetNoDaemonReturnsDaemonUnreachable
result: pass
source: automated
coverage_id: D3 (04-05)

### 24. 'artnet status --json' emits canonical JSON containing per-universe frame health and per-target health (send counts, reachability, enablement).
expected: covered by internal/command/artnet_test.go#TestArtnetStatusJSONContainsHealthFields
result: pass
source: automated
coverage_id: D4 (04-05)

### 25. Plain 'artnet status' renders a persistent per-target status table (D-11) including a freshly configured target.
expected: covered by internal/command/artnet_test.go#TestArtnetStatusPlainRendersPersistentTable
result: pass
source: automated
coverage_id: D5 (04-05)

### 26. 'artnet target disable' then 'artnet target enable' visibly toggle one target's Enabled state in a subsequent status; an unknown target selector fails with GOLC_ARTNET_TARGET_NOT_FOUND.
expected: covered by internal/command/artnet_test.go#TestArtnetTargetEnableDisableRoundTrip, #TestArtnetTargetUnknownReturnsNotFound
result: pass
source: automated
coverage_id: D6 (04-05)

### 27. EncodeArtPoll matches a golden vector; DecodeArtPollReply parses a good reply's IP/name/port-address.
expected: covered by internal/artnet/packet_test.go#TestArtPollEncodeGoldenVector, #TestArtPollReplyDecodeGoodVector
result: pass
source: automated
coverage_id: D1 (04-06)

### 28. Every malformed ArtPollReply input returns GOLC_ARTNET_POLLREPLY_INVALID without panic, and reply length/count fields are bounds-checked before indexing (Security Domain V5, T-04-01).
expected: covered by internal/artnet/packet_test.go#TestArtPollReplyDecodeMalformed
result: pass
source: automated
coverage_id: D2 (04-06)

### 29. Discover against a real loopback UDP responder returns the node as a suggestion; a malformed reply is skipped and the scan still returns a well-formed list; zero replies returns a non-nil empty list.
expected: covered by internal/artnet/discovery_test.go#TestDiscoverReturnsGoodNodeAsSuggestion, #TestDiscoverSkipsMalformedReply, #TestDiscoverZeroRepliesReturnsEmptyList
result: pass
source: automated
coverage_id: D3 (04-06)

### 30. 'artnet discover' performs no target add/remove/modify -- the daemon's configured target set is unchanged before and after a scan; unknown/missing --interface fails clearly.
expected: covered by internal/command/artnet_test.go#TestArtnetDiscoverPerformsNoTargetMutation, #TestArtnetDiscoverUsageErrors, #TestArtnetDiscoverUnknownInterfaceReturnsNotFound, #TestArtnetDiscoverRendersSuggestions
result: pass
source: automated
coverage_id: D4 (04-06)

### 31. docs/artnet/ARTN-06-verification-runbook.md exists with all five required sections, names Wireshark and OLA, documents the OLA-on-separate-host/bridged-VM topology, and drives verification through the real golc artnet serve/configure/status routes.
expected: covered by grep-gate over docs/artnet/ARTN-06-verification-runbook.md (Wireshark, Open Lighting, ARTN-06, "open item")
result: pass
source: automated
coverage_id: D1 (04-07)

### 32. ARTN-06 release-candidate compatibility evidence (D2, 04-07)
expected: |
  A release candidate demonstrates packet + timing compatibility against the independent
  simulator (OLA) with a recorded Wireshark capture, following the runbook, before the
  phase is marked complete.

  Already resolved 2026-07-22: the checkpoint:human-verify was accepted via a recorded
  independent verification (Docker-hosted OLA + Wireshark/tshark against the real
  EncodeArtDMX/PortAddress functions) in place of a self-operated separate-host run.
  Evidence: .planning/artnet/ARTN-06-verification-2026-07-22.md + .pcapng capture. Real
  hardware (D-14) remains untouched and openly tracked, not claimed.
result: pass

### 33. Health model records and lock-free-publishes each configured universe's final per-tick DMX buffer, bounded to the configured universe set (T-04-04).
expected: covered by internal/artnet/health_test.go#TestHealthRecordUniverseValuesSnapshotReflectsConfiguredUniverse, #TestHealthUnconfiguredUniverseValuesNeverTracked, #TestHealthRecordUniverseValuesIsDefensivelyCopied
result: pass
source: automated
coverage_id: D1 (04-08)

### 34. worker.tick() records its previously-discarded per-universe final DMX buffer into Health for every configured universe each tick.
expected: covered by internal/artnet/daemon_test.go#TestDaemonStatusPayloadIncludesConfiguredUniverseValues
result: pass
source: automated
coverage_id: D2 (04-08)

### 35. golc artnet status --json exposes a universes field whose per-universe values decode to a real 512-byte DMX buffer (byte-length assertion, not substring presence) -- corrects 04-05's false-pass acceptance test.
expected: covered by internal/command/artnet_test.go#TestArtnetStatusJSONContainsUniverseValues, #TestArtnetStatusJSONContainsHealthFields
result: pass
source: automated
coverage_id: D3 (04-08)

### 36. Plain and watch golc artnet status render a GOLC_ARTNET_UNIVERSE: line per configured universe with channel count and nonzero byte pairs.
expected: covered by internal/command/artnet_test.go#TestArtnetStatusPlainRendersUniverseValues, #TestArtnetStatusPlainRendersPersistentTable
result: pass
source: automated
coverage_id: D4 (04-08)

### 37. The pinned interface's live Status()/Err() is surfaced through golc artnet status (plain + --json + watch), reporting index/name/status/error.
expected: covered by internal/artnet/daemon_test.go#TestDaemonStatusPayloadIncludesPinnedInterfaceStatus, internal/command/artnet_test.go#TestArtnetStatusPlainRendersPinnedInterface, #TestArtnetStatusJSONIncludesInterfaceStatus
result: pass
source: automated
coverage_id: D1 (04-09)

### 38. A lost/degraded pinned interface reports status=lost plus a GOLC_ARTNET_INTERFACE_LOST error through artnet status, never a silent switch (D-05).
expected: covered by internal/artnet/daemon_test.go#TestDaemonStatusPayloadSurfacesLostInterface
result: pass
source: automated
coverage_id: D2 (04-09)

### 39. golc artnet interface list annotates which candidate is the daemon's pinned interface and its live status when a daemon is reachable, in both plain and --json rendering.
expected: covered by internal/command/artnet_test.go#TestArtnetInterfaceListAnnotatesPinnedWhenDaemonRunning
result: pass
source: automated
coverage_id: D3 (04-09)

### 40. golc artnet interface list still lists every candidate gracefully (ExitCode 0, no GOLC_ARTNET_DAEMON_UNREACHABLE) when no daemon is running -- no regression of the previously-tested no-daemon behavior.
expected: covered by internal/command/artnet_test.go#TestArtnetInterfaceListWorksWithNoDaemon
result: pass
source: automated
coverage_id: D4 (04-09)

## Summary

total: 40
passed: 40
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps

None.
