---
phase: 04
slug: observable-art-net-live-output
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: 2026-07-23
---

# Phase 04 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| Network → GOLC (ArtPollReply) | Untrusted reply data received during optional node discovery (ArtPoll/ArtPollReply) over the pinned local network interface | Raw UDP bytes from unauthenticated network peers |
| IPC client → daemon | Local `golc artnet` CLI invocations talk to the long-lived daemon over a local named pipe | Command.Request/command.Result — configuration, status queries |
| GOLC → network (ArtDMX) | Outbound DMX output to configured unicast Art-Net targets | DMX frame data, addressing |

All 9 plans (04-01 through 04-09) authored a `<threat_model>` block at plan time (`register_authored_at_plan_time: true`); this register consolidates all of them. No open threats at or above the `high` block-on threshold remain (`threats_open: 0`) — per the L1 short-circuit rule, this is a consolidation of plan-time-verified dispositions, not a fresh L2/L3 audit.

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-04-01 | Tampering / Denial of Service | `DecodeArtPollReply` parsing untrusted/oversized replies (04-06) | high | mitigate | Strict length/count bounds-check on every parsed field against a hard ceiling before use; verify id/opcode; malformed input returns `GOLC_ARTNET_POLLREPLY_INVALID`, never a panic or out-of-range read (Security V5). Test: `TestArtPollReplyDecodeMalformed`. | closed |
| T-04-02 | Elevation of Privilege | IPC client → daemon named pipe (originated 04-04; inherited by 04-05/04-08/04-09) | high | mitigate | Windows named pipe with owner-only ACL/SDDL (`D:P(A;;GA;;;OW)`) restricting connect to the owning principal; never binds a routable/TCP address. Every later plan that extends the status payload reuses this same channel, introducing no new routable surface. Test: `TestOwnerOnlySDDLRestrictsToOwner`. | closed |
| T-04-03 | Spoofing | Spoofed ArtPollReply injected as a fake discovered node (04-06) | medium | mitigate | D-06 suggestion-only design: discovery never auto-adds/removes a live target; no bulk "add all" without per-node operator confirmation. | closed |
| T-04-04 | Denial of Service | `health.go` target/health tracking maps (04-03) | medium | mitigate | Bound all tracking to the explicitly configured target set; never create tracking entries from unsolicited inbound traffic from an unconfigured address. Test: `TestHealthUnconfiguredTargetNeverTracked`. | closed |
| T-04-05 | Tampering | `fixture.Mode.Channels` decode/validate (04-01) | low | mitigate | Strict validation of every channel slot (type/occurrence) and hard-reject of missing layout (D-17) before any value drives DMX; bounds-checked buffer offset. | closed |
| T-04-06 | Tampering | Interface pinned by unstable Name (04-02) | low | mitigate | Pin by `net.Interface.Index`, re-validate by index; never persist name-only. | closed |
| T-04-07 | Tampering | `artnet configure`/`target` arg parsing (04-05) | low | mitigate | Two-tier exit codes; every target/interface value validated via Plan 02's validators before forwarding; malformed selectors rejected as `GOLC_ARTNET_USAGE`. | closed |
| T-04-08 | Repudiation | ARTN-06 compatibility claim (04-07) | medium | mitigate | Requires a recorded Wireshark capture + observed physical response before any named real-hardware claim (D-15); real hardware tracked as an explicit open item (D-14) so the release never implies unevidenced support. | closed |
| T-04-09 | Spoofing/DoS | Mid-show pinned-interface loss silently retargeting a different subnet (04-02) | medium | mitigate | D-05 stop-and-degrade; loss never auto-switches to another interface. Test: `TestInterfaceManagerMarkLostTransitionsStatus`. | closed |
| T-04-10 | Denial of Service | Worker per-target send fan-out (04-03) | medium | mitigate | Per-send write deadline + at most one in-flight send per target so a slow/hung node cannot pile up goroutines or stall the tick. Test: `TestWorkerSlowTargetDoesNotStallHealthyTarget`. | closed |
| T-04-11 | Denial of Service | `Health` per-universe values map (04-08) | medium | mitigate | `RecordUniverseValues` gated by `configuredUniverses` (rebuilt only by `Configure` from the operator's explicit target map); a universe never configured can never allocate a values entry. Test: `TestHealthUnconfiguredUniverseValuesNeverTracked`. | closed |
| T-04-12 | Information Disclosure | Per-universe DMX bytes in status output (04-08) | low | accept | The DMX values are the operator's own live output data, disclosed only to the pipe owner (themselves) on their own machine; no cross-user or network exposure. | closed |
| T-04-12b | Information Disclosure | Pinned interface name/index/error in status + interface list (04-09) | low | accept | The interface metadata is the operator's own NIC information on their own machine, disclosed only to the pipe owner (themselves); no cross-user or network exposure. | closed |
| T-04-13 | Denial of Service | `interface list`'s best-effort daemon round trip (04-09) | low | mitigate | Reuses `fetchArtnetStatus`, which surfaces an unreachable daemon as an immediate non-hanging failure; the route swallows that outcome to fall back to the plain list — no hang, no unbounded wait. Test: `TestArtnetInterfaceListWorksWithNoDaemon`. | closed |

*Status: open · closed · open — below `high` threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above `workflow.security_block_on` (`high`) count toward `threats_open`*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-04-01 | T-04-12 | Per-universe DMX output values disclosed only to the local pipe owner (the operator themselves); no cross-user or network exposure | orchestrator (plan-time disposition, 04-08-PLAN.md) | 2026-07-23 |
| AR-04-02 | T-04-12b | Pinned interface name/index/error disclosed only to the local pipe owner (the operator themselves); no cross-user or network exposure | orchestrator (plan-time disposition, 04-09-PLAN.md) | 2026-07-23 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-07-23 | 14 | 14 | 0 | Claude (orchestrator, L1 short-circuit — register_authored_at_plan_time: true, threats_open: 0, asvs_level: 1; consolidated from all 9 plans' plan-time `<threat_model>` blocks and 04-08/04-09 SUMMARY.md's `## Threat Flags` confirmations, per secure-phase.md's documented short-circuit rule) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-07-23
