---
phase: 04-observable-art-net-live-output
plan: 07
subsystem: artnet
tags: [artnet, verification, wireshark, ola, documentation]

# Dependency graph
requires:
  - phase: 04-observable-art-net-live-output
    plan: 05
    provides: internal/command/artnet.go's "artnet serve/configure/status/target" routes, which this plan's runbook drives to produce and inspect Art-Net 4 output
  - phase: 04-observable-art-net-live-output
    plan: 06
    provides: "artnet discover" and the completed ARTN-02 CLI surface referenced alongside the other artnet routes in the runbook's context
provides:
  - docs/artnet/ARTN-06-verification-runbook.md -- the ARTN-06 independent-verification checklist (Wireshark packet inspection, OLA-on-separate-host setup, evidence bar, open real-hardware tracking entry)
  - .planning/artnet/ARTN-06-verification-2026-07-22.md -- recorded verification evidence (Task 2 checkpoint resolution) and its accompanying .pcapng capture
affects: []

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Verification-environment documentation (not code) is the deliverable for a manual-only requirement with no CI substitute -- the runbook exists purely to make ARTN-06's human verification repeatable and to bound what compatibility claim the evidence actually supports (D-15), mirroring Phase 6's MIDI-HW-01/02 selection-≠-support precedent for the same class of hardware-acceptance requirement."

key-files:
  created:
    - docs/artnet/ARTN-06-verification-runbook.md
    - .planning/artnet/ARTN-06-verification-2026-07-22.md
    - .planning/artnet/ARTN-06-verification-2026-07-22.pcapng
  modified: []

key-decisions:
  - "The runbook documents OLA running on a separate Linux/macOS host or a bridged-adapter Linux VM (never NAT/WSL2 default), since OLA has no first-class native Windows build (D-18/Pitfall 6) -- this is a verification-environment setup instruction, not a GOLC feature change."
  - "The runbook explicitly separates what Sections 2-3 (simulator + packet) currently prove from what Section 4/5 (a named real-hardware claim) would require, per D-14/D-15, so no reader could mistake simulator-only verification for a hardware compatibility claim."

requirements-completed: [ARTN-06]

# Task 2's human-verify checkpoint was resolved by the operator accepting
# recorded verification evidence (.planning/artnet/ARTN-06-verification-2026-07-22.md)
# in place of a self-operated manual run, with documented topology/scope
# caveats. See "Checkpoint Status" below for the full resolution record.

coverage:
  - id: D1
    description: "docs/artnet/ARTN-06-verification-runbook.md exists with all five required sections (environment setup, packet-level verification, independent-receiver verification, evidence bar, open real-hardware item), names Wireshark and OLA, documents the OLA-on-separate-host/bridged-VM topology, and drives verification through the real golc artnet serve/configure/status routes"
    requirement: "ARTN-06"
    verification:
      - kind: other
        ref: "grep -qi 'Wireshark' && grep -qi 'Open Lighting' && grep -q 'ARTN-06' && grep -qi 'open item' docs/artnet/ARTN-06-verification-runbook.md"
        status: pass
    human_judgment: false
  - id: D2
    description: "A release candidate demonstrates packet + timing compatibility against the independent simulator (OLA) with a recorded Wireshark capture, following the runbook, before the phase is marked complete (backstop truth in PLAN.md must_haves)"
    verification:
      - kind: other
        ref: ".planning/artnet/ARTN-06-verification-2026-07-22.md + accompanying .pcapng capture"
        status: pass
    human_judgment: true
    rationale: "Task 2's checkpoint:human-verify was resolved by the operator (2026-07-22): recorded independent verification (OLA + Wireshark, run via Docker rather than a manually-operated separate host) confirmed all Section 2 packet fields and Section 3's received-value cross-check. The operator explicitly accepted this evidence in place of a self-operated manual run, with topology (loopback+Docker vs. separate-host/bridged-VM) and full-CLI-flow gaps documented rather than silently closed. Real hardware (D-14) remains untouched and open."

# Metrics
duration: ~55min (Task 1 ~15min + Task 2 verification ~40min)
completed: 2026-07-22
status: complete
---

# Phase 4 Plan 07: ARTN-06 Independent-Verification Runbook Summary

**Authored the ARTN-06 verification runbook (Wireshark packet checklist + OLA-on-separate-host setup + D-15 evidence bar + D-14 open real-hardware item); Task 2's human-verify checkpoint was resolved by the operator accepting a recorded, independently-verified Docker/Wireshark/OLA evidence run against golc's real production encoder, with topology and CLI-flow scope caveats documented rather than silently closed.**

## Performance

- **Duration:** ~55 min (Task 1 ~15 min + Task 2 verification ~40 min)
- **Started:** 2026-07-22T09:40:00Z (approximate)
- **Completed:** Both tasks complete
- **Tasks:** 2 of 2 complete
- **Files modified:** 3 (created)

## Accomplishments

- `docs/artnet/ARTN-06-verification-runbook.md` created as a five-section runnable checklist:
  1. **Environment setup** (D-18/Pitfall 6): provisioning OLA on a separate Linux/macOS host or a bridged-adapter Linux VM (explicitly not NAT/WSL2 default), plus Wireshark on a capture-capable machine (dissector ships in Wireshark core).
  2. **Packet-level verification** (D-13, success criterion 2): exact steps to `golc artnet serve`/`configure`/status-check, with a field-by-field Wireshark checklist (Art-Net id, OpCode `0x5000`, protocol version 14, Port-Address per the locked Net=0/Sub-Net/Universe mapping, sequence advancing and never 0, even payload length in [2,512], ~40Hz cadence) cross-referenced against the real `internal/artnet/packet.go`/`worker.go` production code.
  3. **Independent-receiver verification** (D-13, success criterion 4): confirming OLA-received per-universe DMX values and target enable/disable behavior against `golc artnet status --json`.
  4. **Evidence bar for a future real-hardware claim** (D-15): explicitly states that simulator-only verification is insufficient — both a saved Wireshark capture AND an observed/recorded correct physical fixture response are required before any named hardware claim.
  5. **Open real-hardware item** (D-14): a tracking entry stating no real Art-Net hardware is owned, mirroring the `MIDI-HW-01`/`MIDI-HW-02` selection-≠-support pattern already established in `.planning/PROJECT.md`/`.planning/ROADMAP.md` for Phase 6.
- The runbook drives verification exclusively through the real, already-implemented `golc artnet serve/configure/status/target enable/target disable` routes (`internal/command/artnet.go`) — no new CLI surface was invented for verification purposes.

## Task Commits

Each task was committed atomically:

1. **Task 1: Author the ARTN-06 independent-verification runbook (D-13/D-14/D-15/D-18)** - `f037a36` (docs)
2. **Task 2: Human-verify ARTN-06 packet + simulator compatibility (D-13/D-15)** - `8886fc9` (docs — verification evidence record + capture)

## Files Created/Modified

- `docs/artnet/ARTN-06-verification-runbook.md` - Five-section ARTN-06 verification checklist (environment setup, packet-level, independent-receiver, evidence bar, open real-hardware item)
- `.planning/artnet/ARTN-06-verification-2026-07-22.md` - Written record of the Task 2 checkpoint resolution: what was verified, results, and honest scope caveats
- `.planning/artnet/ARTN-06-verification-2026-07-22.pcapng` - Raw Wireshark capture of 240 ArtDMX packets, the recorded evidence per runbook Section 4

## Decisions Made

- No new decisions beyond what CONTEXT.md's D-13/D-14/D-15/D-18 already locked; the runbook implements those decisions directly rather than introducing new ones.
- The runbook was structured to make explicit, in its own text, the boundary between what Sections 2-3 currently prove (simulator + packet compatibility) and what a future named hardware claim (Section 4/5) would require — so the document itself cannot be misread as already claiming hardware support.

## Deviations from Plan

Task 1 executed exactly as written. For Task 2, the executor correctly stopped and returned the checkpoint per plan (the plan's Task 2 is `type="checkpoint:human-verify" gate="blocking"`, and the executor must never attempt or simulate it). The orchestrator then, at the operator's explicit direction ("you have access to hyper-v, qemu, docker, whatever tools you need to test this yourself"), performed a real independent verification using Docker (OLA) and Wireshark/tshark rather than the operator personally running the runbook on a separate host. This substitutes automated-but-genuine tooling for the originally-envisioned fully-manual process; the operator was shown the results, including honest scope caveats (topology and CLI-flow gaps), and explicitly chose to accept this evidence as satisfying the checkpoint rather than requiring a fully manual re-run.

## Issues Encountered

None for Task 1. For Task 2: no dedicated second host/VM was available in this session, so the verification ran OLA in a local Docker container with its Art-Net UDP port published to loopback, rather than the runbook's recommended separate-host/bridged-VM topology (D-18/Pitfall 6's concern). The full `golc artnet serve/configure/status` CLI flow was also not exercised (no show/fixture file exists in the repo yet) — the verification called golc's production `EncodeArtDMX`/`PortAddress` functions directly instead. Both gaps are documented in `.planning/artnet/ARTN-06-verification-2026-07-22.md` and were disclosed to the operator before they accepted the checkpoint resolution.

## Checkpoint Status

**Task 2's `checkpoint:human-verify` (`gate="blocking"`) is RESOLVED.** Resolution path:

1. The executor reached the checkpoint and returned it without attempting Task 2, exactly per plan.
2. The orchestrator, explicitly authorized by the operator to use available local virtualization/testing tools, performed a real (not simulated) verification: OLA running in Docker with its Art-Net plugin enabled and universe 0 patched to receive; golc's actual production `internal/artnet.EncodeArtDMX`/`PortAddress` driven at the real 40Hz worker cadence for 6 seconds (240 frames) to `127.0.0.1:6454`; a simultaneous `tshark` capture on loopback filtered to `udp port 6454`.
3. **Results:** Wireshark's own (independent, unmodified) Art-Net dissector confirmed every required Section 2 field across all 240 packets (Art-Net ID, OpDMX `0x5000`, ProtVer 14, correct Port-Address, sequence advancing 1→240 never 0, 512-byte payload, exact 40Hz cadence). OLA's own HTTP API independently confirmed it received and held the exact final sent values (`251,10,20,30`).
4. **Scope caveats disclosed and accepted:** topology used loopback+Docker port-publish rather than a genuinely separate host/bridged VM over a real pinned interface; the full CLI flow (`serve`/`configure`/`status`) was not exercised end-to-end (no show/fixture file exists yet in this repo — a pre-existing gap, not introduced here); D-14's real-hardware open item remains untouched and open.
5. The operator was presented with all of the above, including the caveats, and explicitly chose **"Accept — close Wave 7 now"** over the alternative options (full manual runbook first, or closing the CLI-flow gap first).

Full record: `.planning/artnet/ARTN-06-verification-2026-07-22.md` (commit `8886fc9`), with the raw capture at `.planning/artnet/ARTN-06-verification-2026-07-22.pcapng`.

This SUMMARY marks `requirements-completed: [ARTN-06]` and frontmatter `status: complete` on that basis. The topology and full-CLI-flow gaps remain legitimate follow-up items (not hidden — see the verification record's "What was NOT verified" section) should a stronger claim be wanted later; they do not block this plan or phase from completing.

## User Setup Required

**Yes — see `docs/artnet/ARTN-06-verification-runbook.md` Section 1 for full details.** Summary:
- Provision a second Linux/macOS host, or a Linux VM with a **bridged** (not NAT/WSL2-default) network adapter, reachable from GOLC's pinned Windows interface, and install OLA on it.
- Install Wireshark (Art-Net/DMX dissector ships in Wireshark core, no plugin download needed) on a capture-capable machine.
- No environment variables or GOLC-side configuration changes are required — this is external verification infrastructure, not application configuration.

## Next Phase Readiness

- Both tasks complete; this is the phase's final wave/plan. ARTN-06 is marked complete.
- Follow-up (optional, not blocking): a fuller manual pass on a genuinely separate host/bridged VM, and/or building a minimal show/fixture file to drive the full `golc artnet serve/configure/status` CLI end-to-end, would close the topology and CLI-flow gaps noted in `.planning/artnet/ARTN-06-verification-2026-07-22.md` if a stronger claim is wanted later.
- Real Art-Net hardware (D-14) remains an explicit, open, un-claimed item — untouched by this plan, exactly as designed.
- No blockers for other phases — Phase 4's automated coverage (ARTN-01 through ARTN-05, Plans 01-06) plus this plan's ARTN-06 resolution completes the phase's requirement set.

## Self-Check: PASSED
- FOUND: docs/artnet/ARTN-06-verification-runbook.md
- FOUND: .planning/artnet/ARTN-06-verification-2026-07-22.md
- FOUND: .planning/artnet/ARTN-06-verification-2026-07-22.pcapng
- FOUND commit: f037a36 (Task 1)
- FOUND commit: 8886fc9 (Task 2 checkpoint resolution)

---
*Phase: 04-observable-art-net-live-output*
*Completed: both tasks — Task 2 checkpoint resolved by operator-accepted recorded evidence*
