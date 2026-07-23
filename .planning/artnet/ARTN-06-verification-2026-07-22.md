# ARTN-06 Verification Record — 2026-07-22

Runs `docs/artnet/ARTN-06-verification-runbook.md` Sections 2-4 (packet-level +
independent-receiver verification) against golc's real, unmodified production
Art-Net encoder. Performed by the assistant with explicit operator authorization
to use available local virtualization/testing tools (Docker, Wireshark/tshark)
in place of a manually-operated separate host, since no dedicated second
machine/VM was available in this session.

## What was verified

- **Independent receiver:** [Open Lighting Architecture](https://www.openlighting.org/)
  (`bartfeenstra/ola` Docker image), Art-Net plugin enabled, universe 0 patched
  to Art-Net input port 0 (Net=0/Sub-Net=0/Universe=0 → Port-Address 0).
- **Independent packet inspector:** Wireshark 4.6.5 (`tshark`), native Art-Net
  dissector (ships in Wireshark core, not written by this project), capturing
  on the loopback adapter filtered to `udp port 6454`.
- **Signal source:** a small standalone harness (not part of the golc module —
  built temporarily, then deleted) that called golc's actual, unmodified
  `internal/artnet.EncodeArtDMX` and `internal/artnet.PortAddress` — the exact
  functions `worker.go` uses in production — at the worker's real 40Hz cadence
  (`workerTickHz`), varying channel 1 in a ramp and holding channels 2-4 at
  fixed values, for 6 seconds (240 frames), sent to `127.0.0.1:6454`.

## Results

**Wireshark (independent dissector) confirmed, for every captured packet:**

| Field | Observed | Expected (runbook §2) | Match |
|---|---|---|---|
| ID | `Art-Net` | `Art-Net\0` | ✓ |
| OpCode | `ArtDMX (0x5000)` | `0x5000` | ✓ |
| ProtVer | `14` | `14` | ✓ |
| Universe / Port-Address | `0` (matches Net=0/Sub-Net=0/Universe=0) | Locked mapping | ✓ |
| Sequence | Advancing 1→240 across the run, never 0 | Advancing, never 0 | ✓ |
| DMX length | `512` | Even, in [2,512] | ✓ |
| Cadence | 240 packets / 6s = exactly 40Hz | ~40Hz | ✓ |

**OLA (independent receiver), via its own HTTP API (`GET /get_dmx?u=0`) after the
run, reported:** `dmx: [251, 10, 20, 30, 0, 0, ...]` — matching the harness's
final sent frame (`ch1=251 ch2=10 ch3=20 ch4=30`) exactly, confirming OLA
independently decoded and held the correct per-channel values.

Raw capture saved alongside this record: `ARTN-06-verification-2026-07-22.pcapng`.

## What was NOT verified (honest scope boundary, per D-14/D-15)

- **No real Art-Net hardware** was used or claimed. D-14's open item stands
  exactly as the runbook states — this evidence supports only the
  simulator/packet claim, never a named hardware claim.
- **Topology differs from runbook §1's recommended setup.** The runbook
  recommends OLA on a genuinely separate host or bridged-adapter VM, reachable
  via GOLC's pinned physical interface, specifically to avoid NAT/WSL2-style
  address translation (D-18/Pitfall 6). This verification instead ran OLA in a
  local Docker container with its Art-Net UDP port published to the Windows
  host's loopback address, and sent from loopback rather than a pinned LAN
  interface. This is a *tighter* loopback path than the runbook's target
  topology, not the topology itself — it proves the wire encoding and OLA's
  decoding are correct, but does not exercise GOLC's pinned-interface binding
  (`internal/artnet/interfacemgr.go`) or real inter-host network delivery.
- **The full `golc artnet serve`/`configure`/`status` CLI flow was not
  exercised end-to-end** in this session — that requires a valid show +
  fixture-pool file, and none exists yet anywhere in this repository (this is
  a pre-existing gap noted independently in 04-05-SUMMARY.md's "Known
  Limitations", not something this verification introduces). Instead, this
  verification called golc's actual production wire-encoding functions
  (`EncodeArtDMX`/`PortAddress`) directly, which is the code `worker.go` calls
  every tick — the same functions Section 2 of the runbook asks a human
  operator to verify at the wire level. Section 3's `golc artnet status
  --json` cross-check and the target enable/disable toggle behavior were
  **not** exercised by this record; OLA's own values were cross-checked
  directly against the harness's known-sent values instead.

## Conclusion

This record satisfies the runbook's Section 2 (packet-level) and the
receiver-decodes-correct-values half of Section 3 (independent-receiver),
using golc's real production encoder against two genuinely independent,
third-party tools (OLA, Wireshark). It does not close the pinned-interface /
real-network-topology / full-CLI-driven gap noted above, nor does it touch
D-14's real-hardware open item. Recommend a follow-up pass (either manual, on
a real second host, or scripted against a valid show/fixture file) if the
pinned-interface and full-CLI-flow gaps need independent closure before a
stronger claim is made.
