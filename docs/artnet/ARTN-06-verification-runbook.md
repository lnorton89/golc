# ARTN-06 Independent Verification Runbook

**Requirement:** ARTN-06 — "Release demonstrates packet and timing compatibility with an independent simulator and real hardware."
**Decisions this runbook implements:** D-13 (locked tools: Wireshark + Open Lighting Architecture), D-14 (real hardware is an explicit open item, not claimed), D-15 (evidence bar for a future hardware claim), D-18 (OLA host topology).

This is a **manual-only** verification. No CI substitute exists for it (04-RESEARCH.md's Validation Architecture table lists ARTN-06 as manual-only, with no automated command). Follow this checklist against a release candidate build and record the evidence described in Section 4 before the phase is marked complete.

---

## 1. Environment Setup (D-18 / Pitfall 6)

Open Lighting Architecture (OLA) has **no first-class native Windows build**. Do not attempt to install and run OLA directly on the same Windows machine as GOLC — this will stall. GOLC's pinned network interface (D-05) is Windows-only for v1; OLA must run somewhere else and be reached over the network as an ordinary unicast Art-Net target.

1. **Provision a second host for OLA.** One of:
   - A separate physical Linux or macOS machine on the same network as the GOLC Windows machine, or
   - A Linux VM with a **bridged** network adapter (NOT NAT, and NOT WSL2's default NAT-mode networking) — bridged mode is required because OLA needs a directly routable IP address that GOLC's static-unicast-target model (D-07) can address as a normal Art-Net node. NAT/WSL2-default networking hides the VM behind a translated address that is not directly reachable from GOLC's pinned interface.
2. **Install OLA** on that second host per openlighting.org's getting-started instructions (Linux native install, or the package appropriate to the distro/VM).
3. **Note the OLA host's IP address** on the bridged/LAN interface that is reachable from the GOLC Windows machine's pinned interface. Confirm reachability both ways with a basic ping before proceeding.
4. **Install Wireshark** on a capture-capable machine. This can be the GOLC Windows machine itself (to capture outbound traffic at the source) or any machine positioned to see the traffic on the wire (e.g. a mirrored switch port). The Art-Net/DMX dissector ships in Wireshark core — no separate plugin download is required.
5. **Confirm GOLC's pinned interface** (`golc artnet interface list`) is on the same subnet/route as the OLA host, or that routing between them is otherwise correct.

---

## 2. Packet-Level Verification (D-13, success criterion 2)

This section proves GOLC's Art-Net 4 output is spec-correct at the byte level, independent of whether anything is actually listening.

1. **Start the daemon:**
   ```
   golc artnet serve --show <path-to-show> --interface <index> [--fixtures <dir>]
   ```
   Use `golc artnet interface list` first to find the correct `--interface` index for the pinned NIC (D-05 pins by `net.Interface.Index`, never by name).

2. **Configure a universe + the OLA host as a unicast target:**
   ```
   golc artnet configure --universe <n> --ip <OLA-host-IP> [--port 6454] --enabled true
   ```
   Repeat for every universe you want to verify. Port defaults to the fixed Art-Net UDP port 6454 (0x1936) if omitted.

3. **Start playback** on the show so frames are actively being produced (`Engine.CurrentFrame()` is non-nil and advancing).

4. **Capture in Wireshark**, filtered to `udp.port == 6454`. Let the capture run long enough to observe several seconds of steady-state output (at least 10-20 frames).

5. **Confirm every one of the following fields** in the captured ArtDMX packets:

   | Field | Expected value | Why it matters |
   |-------|----------------|-----------------|
   | ID string | `Art-Net\0` (8 bytes) | Packet identity per spec |
   | OpCode | `0x5000` (ArtDMX / "OpOutput"), little-endian on the wire | Confirms this is an output packet, not poll/reply |
   | ProtVerHi/ProtVerLo | `0x00`/`0x0e` (protocol version 14) | GOLC targets Art-Net 4; version 14 is the current wire-format version |
   | Sequence | Advancing across consecutive packets for the same universe, wrapping 1→255→1, **never 0** | Sequence 0 explicitly disables reordering-detection on the receiving node (Pitfall 2) — a stuck-at-0 or wrap-through-0 sequence is a bug, not a quirk |
   | Port-Address (SubUni/Net bytes) | Matches the locked mapping: Net=0 (fixed), Sub-Net=`(universe>>4)&0xF`, Universe=`universe&0xF`, packed as byte14=`SubNet<<4 \| Universe`, byte15=`Net` (top bit reserved zero) | Confirms GOLC's flat `Instance.Universe` integer is reaching the wire as the correct 15-bit Art-Net Port-Address (assumption A1, locked) |
   | Data length | Even, in range `[2, 512]` | Matches `EncodeArtDMX`'s own length validation (`GOLC_ARTNET_DMX_LENGTH_INVALID` is the error path if this is ever violated internally — should never appear on the wire) |
   | Refresh cadence | Approximately 40Hz (~25ms between consecutive ArtDMX packets for the same universe) | Matches the DMX/Art-Net industry-standard refresh band and this repo's own `tickHz = 40` constant (`internal/playback/engine.go`); the Art-Net worker runs its own independent 40Hz ticker (ARTN-04's decoupling guarantee) |

   Cross-reference: `internal/artnet/packet.go` (`EncodeArtDMX`, `PortAddress`) and `internal/artnet/worker.go` (`nextSeq`) are the exact production code this section verifies against.

---

## 3. Independent-Receiver Verification (D-13, success criterion 4)

This section proves an independent, non-GOLC codebase (OLA) sees the values GOLC believes it is sending.

1. With the daemon still running and the universe(s) configured per Section 2, open OLA's web UI (or `ola_dmxmonitor`/equivalent CLI tool) on the OLA host and select the universe corresponding to the configured target.
2. **Confirm the per-universe DMX channel values OLA displays match what GOLC intends to send** for the currently active look/scene — cross-check against:
   ```
   golc artnet status --json
   ```
   which reports frame health (cadence/staleness) and per-target send/error counts and reachability (`SendOK`/`SendErr`/`Reachable`/`LastError` per target, per `internal/command/artnet.go`'s `artnetStatusPayload`).
3. **Confirm target behavior**: toggle a target offline/online via `golc artnet target disable`/`golc artnet target enable` (D-12) and confirm OLA stops/resumes receiving updates for that target's universe accordingly, without affecting other configured universes/targets.
4. **Confirm `golc artnet status`'s reachability signal** for the OLA target reflects OLA's actual presence (D-10): if OLA supports ArtPoll, GOLC's `Reachable` field should reflect that; if not, GOLC should still show accurate `SendOK`/`SendErr` counts without claiming reachability it cannot verify.

---

## 4. Evidence Bar for a Future Real-Hardware Claim (D-15)

**Simulator + packet-level verification (Sections 2-3) is sufficient to claim compatibility with the independent simulator (OLA) and to demonstrate Art-Net 4 packet correctness. It is NOT sufficient, by itself, to claim compatibility with any specific real Art-Net hardware node/fixture.**

Before GOLC may make a **named real-hardware compatibility claim** for any specific node or fixture, both of the following must be recorded as evidence:

1. **A saved Wireshark capture** (`.pcap`/`.pcapng` file) showing correct Art-Net 4 output reaching the real node — same field checklist as Section 2, captured against the real device's IP rather than OLA's.
2. **An observed and recorded correct physical response from the fixture** — e.g. a photo/video, or a written observation log, showing the fixture actually responding correctly (correct channel behaves as expected: intensity, color, position, etc. per the values GOLC sent).

Simulator-only verification (Sections 2-3 alone) does **not** meet this bar. Save both artifacts (capture file + physical-response record) together, referenced from the same verification record, before naming that specific hardware as supported.

---

## 5. Open Real-Hardware Item (D-14)

**No real Art-Net hardware is currently owned by this project.** Real-hardware compatibility is tracked as an **explicit, open, unresolved item** — it is not silently assumed complete, and no named real-hardware compatibility or support claim is made anywhere in this phase's deliverables.

This mirrors the same selection-≠-support pattern already established for Phase 6's MIDI hardware (`MIDI-HW-01`/`MIDI-HW-02` in `.planning/ROADMAP.md` §Phase 6 and `.planning/PROJECT.md`): choosing/naming a tool or device does not itself establish support. For Art-Net:

- Simulator (OLA) + packet-level (Wireshark) verification **can and does proceed now** (Sections 2-4 above) — this is the currently supported and evidenced claim.
- A **named real-hardware compatibility claim requires independent evidence** per Section 4's evidence bar, gathered against that exact device, before it may be asserted anywhere (release notes, documentation, marketing, etc.).
- Until such evidence exists for a specific device, GOLC's Art-Net support should be described as "verified against Wireshark packet capture and the Open Lighting Architecture simulator; real-hardware compatibility not yet independently evidenced" — never as unqualified "hardware support."

**Tracking:** Treat this open item the same way `MIDI-HW-02` is tracked in `.planning/PROJECT.md`'s Blockers/Concerns and Key Decisions sections — resolved only when a specific real Art-Net node/fixture has passed both evidence-bar items in Section 4 above.

---

## Checklist Summary

- [ ] Section 1: OLA running on a separate Linux/macOS host or bridged-adapter VM (not NAT/WSL2 default), reachable from GOLC's pinned interface; Wireshark installed on a capture-capable machine.
- [ ] Section 2: Wireshark capture confirms Art-Net id, OpOutput opcode (0x5000), protocol version 14, correct Port-Address per the locked Net=0/Sub-Net/Universe mapping, sequence advancing and never 0, even payload length in [2,512], ~40Hz cadence.
- [ ] Section 3: OLA-received per-universe DMX values and target enable/disable behavior cross-checked against `golc artnet status --json`.
- [ ] Section 4: Wireshark capture file saved as recorded evidence (required for any future hardware claim, alongside an observed physical-fixture response).
- [ ] Section 5: No named real-hardware compatibility claim is made; the open item is tracked, not silently resolved.
