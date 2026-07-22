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
affects: []

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Verification-environment documentation (not code) is the deliverable for a manual-only requirement with no CI substitute -- the runbook exists purely to make ARTN-06's human verification repeatable and to bound what compatibility claim the evidence actually supports (D-15), mirroring Phase 6's MIDI-HW-01/02 selection-≠-support precedent for the same class of hardware-acceptance requirement."

key-files:
  created:
    - docs/artnet/ARTN-06-verification-runbook.md
  modified: []

key-decisions:
  - "The runbook documents OLA running on a separate Linux/macOS host or a bridged-adapter Linux VM (never NAT/WSL2 default), since OLA has no first-class native Windows build (D-18/Pitfall 6) -- this is a verification-environment setup instruction, not a GOLC feature change."
  - "The runbook explicitly separates what Sections 2-3 (simulator + packet) currently prove from what Section 4/5 (a named real-hardware claim) would require, per D-14/D-15, so no reader could mistake simulator-only verification for a hardware compatibility claim."

requirements-completed: []

# Task 2 (human-verify checkpoint) has not yet been performed -- see "Checkpoint Status" below.
# requirements-completed intentionally left empty until the human-verify checkpoint is resolved
# and this plan's ARTN-06 requirement can be marked complete by the orchestrator.

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
    verification: []
    human_judgment: true
    rationale: "This is Task 2's checkpoint:human-verify -- it requires real Wireshark + OLA hardware/VM setup that only a human operator can perform (no CI substitute exists per 04-RESEARCH.md's Validation Architecture table). Not yet performed in this execution; recorded here as pending, not silently assumed complete."

# Metrics
duration: ~15min (Task 1 only; Task 2 pending)
completed: 2026-07-22
status: blocked
---

# Phase 4 Plan 07: ARTN-06 Independent-Verification Runbook Summary

**Authored the ARTN-06 verification runbook (Wireshark packet checklist + OLA-on-separate-host setup + D-15 evidence bar + D-14 open real-hardware item); Task 2's human-verify checkpoint (running the actual Wireshark capture and OLA cross-check against a release candidate) remains pending and requires human execution with real hardware/VM setup.**

## Performance

- **Duration:** ~15 min (Task 1 only)
- **Started:** 2026-07-22T09:40:00Z (approximate)
- **Completed:** Task 1 complete; Task 2 (checkpoint) not yet started
- **Tasks:** 1 of 2 complete
- **Files modified:** 1 (created)

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
2. **Task 2: Human-verify ARTN-06 packet + simulator compatibility (D-13/D-15)** - **NOT STARTED** (blocking human-verify checkpoint, gate="blocking")

## Files Created/Modified

- `docs/artnet/ARTN-06-verification-runbook.md` - Five-section ARTN-06 verification checklist (environment setup, packet-level, independent-receiver, evidence bar, open real-hardware item)

## Decisions Made

- No new decisions beyond what CONTEXT.md's D-13/D-14/D-15/D-18 already locked; the runbook implements those decisions directly rather than introducing new ones.
- The runbook was structured to make explicit, in its own text, the boundary between what Sections 2-3 currently prove (simulator + packet compatibility) and what a future named hardware claim (Section 4/5) would require — so the document itself cannot be misread as already claiming hardware support.

## Deviations from Plan

None — Task 1 executed exactly as written. The plan's own structure (Task 1 `type="auto"`, Task 2 `type="checkpoint:human-verify" gate="blocking"`) required this executor to stop after Task 1 and return the checkpoint rather than attempting or simulating Task 2's manual verification.

## Issues Encountered

None for Task 1. Task 2 cannot be completed by this executor: it requires an operator with access to real Wireshark + OLA hardware/VM infrastructure (see Checkpoint Status below and the CHECKPOINT REACHED block returned alongside this summary).

## Checkpoint Status

**Task 2 is a `checkpoint:human-verify` with `gate="blocking"` and has NOT been performed in this execution.** Per this plan's own instructions and the orchestrator's spawn context for this worktree agent, the executor must not attempt or simulate the manual Wireshark/OLA verification — it requires:
- A second host (Linux/macOS or bridged-adapter Linux VM) running OLA, reachable from GOLC's pinned Windows interface.
- Wireshark installed on a capture-capable machine.
- A human operator to run the verification steps in `docs/artnet/ARTN-06-verification-runbook.md` Sections 2-3, save the Wireshark capture (Section 4), and confirm the open real-hardware item (Section 5) remains correctly un-claimed.

This SUMMARY intentionally does not mark `requirements-completed: [ARTN-06]` and sets frontmatter `status: blocked` (not `complete`) because Task 2's coverage item (D2) has `human_judgment: true` and has not yet been resolved. The orchestrator should treat this plan as paused at a blocking checkpoint, not finished, until a human runs the checkpoint and the result (approved / discrepancy) is recorded.

## User Setup Required

**Yes — see `docs/artnet/ARTN-06-verification-runbook.md` Section 1 for full details.** Summary:
- Provision a second Linux/macOS host, or a Linux VM with a **bridged** (not NAT/WSL2-default) network adapter, reachable from GOLC's pinned Windows interface, and install OLA on it.
- Install Wireshark (Art-Net/DMX dissector ships in Wireshark core, no plugin download needed) on a capture-capable machine.
- No environment variables or GOLC-side configuration changes are required — this is external verification infrastructure, not application configuration.

## Next Phase Readiness

- Task 1's runbook is complete and self-contained; it does not block any other phase-4 plan (this is the phase's final wave/plan).
- Task 2's checkpoint must be resolved (human runs the runbook against a release candidate, records the Wireshark capture, and confirms OLA-received values) before ARTN-06 can be marked complete and before the phase gate closes, per the plan's own `<verification>`/`<success_criteria>` blocks.
- No blockers for other phases — Phase 4's automated coverage (ARTN-01 through ARTN-05, Plans 01-06) is already complete and independent of this plan's pending checkpoint.

## Self-Check: PASSED
- FOUND: docs/artnet/ARTN-06-verification-runbook.md
- FOUND commit: f037a36 (Task 1)

---
*Phase: 04-observable-art-net-live-output*
*Completed: Task 1 only — Task 2 checkpoint pending human verification*
