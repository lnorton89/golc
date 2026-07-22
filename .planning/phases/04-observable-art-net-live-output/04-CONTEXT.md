# Phase 4: Observable Art-Net Live Output - Context

**Gathered:** 2026-07-22
**Status:** Ready for planning

<domain>
## Phase Boundary

Operators can drive a small Art-Net rig from deterministic complete frames and independently verify protocol, target, and timing health. This phase delivers: (1) Windows network-interface selection with status, (2) Art-Net universe and static unicast target configuration with optional node discovery, (3) a background Art-Net worker that converts deterministic playback frames into valid Art-Net 4 DMX packets with correct addressing/sequencing/payload/refresh, decoupled from and never backpressured by UI/persistence/scripts/API/LLM, (4) an independent per-universe/target health and diagnostic surface, and (5) hardware/simulator compatibility verification for a release candidate. This phase is CLI-only and headless (see D-01) — matching Phase 1–3's precedent of no Wails UI until Phase 6, resolving ROADMAP.md's "UI hint: yes" flag as CLI, not GUI.

Requirements: ARTN-01 through ARTN-06.

</domain>

<decisions>
## Implementation Decisions

### Interface & Process Model
- **D-01:** Phase 4's operator surface is CLI-only for now (`golc artnet` subcommands) — no standalone GUI or Wails window. This matches Phase 1–3's headless precedent; Phase 6 later wraps these same typed commands in Wails without rework, given the shared `internal/command` model.
- **D-02:** The CLI health/status inspection (ARTN-05) supports both a continuously-refreshing watch view (human-readable table, live monitoring) and a one-shot snapshot (plain output by default, `--json` flag for scripting) — mirrors Phase 2's dry-run/human+JSON precedent (D-15).
- **D-03:** The Art-Net worker runs as part of one long-lived GOLC process alongside the playback engine. CLI commands (`golc artnet configure/status/...`) are clients that connect to this running process (local IPC) to configure or inspect it — they do not each own a separate output process.
- **D-04:** The long-lived process is standalone-capable: it can run entirely headless via CLI, with no Wails window ever required. Phase 6's Wails app is just one more client that attaches to the same running instance later. This is the literal reading of the ROADMAP goal's "independently of the desktop UI."

### Network Interface & Target Configuration
- **D-05:** The selected Windows network interface is explicitly pinned by the operator. If it disappears or changes (multi-NIC machines, VPN adapters), GOLC stops sending on it and surfaces a clear error/degraded health state — it never silently auto-switches to a different interface (which could re-address a different subnet mid-show).
- **D-06:** Optional node discovery (ArtPoll/ArtPollReply) only surfaces compatible nodes as suggestions. Adding a discovered node as an active unicast target is always an explicit operator action — discovery never auto-populates or auto-removes live targets.
- **D-07:** Output is strictly per-target unicast. There is no broadcast mode — matches ROADMAP's own "static unicast targets" framing and keeps traffic scoped on shared networks/VPNs.
- **D-08:** A single universe can be configured to fan out to multiple unicast targets simultaneously (e.g. a universe split across two nodes, or a redundant backup node) — not strictly one universe → one target.

### Health & Diagnostics Surface
- **D-09:** "Frame health" (ARTN-05) is measured as worker publish cadence plus staleness of the last frame read from `Engine.CurrentFrame()` (e.g. "last frame 8ms ago, on cadence" vs "no new frame in 400ms — engine may be stalled"). This distinguishes a stalled engine from a healthy one and directly demonstrates ARTN-04's non-backpressuring guarantee.
- **D-10:** "Target health" per configured unicast target is send success/error count plus a reachability signal (via ArtPollReply when the node supports polling) — not just the absence of OS-level send errors, so a genuinely dead node is distinguishable from one that simply doesn't answer polls.
- **D-11:** Errors (send failures, stalled frames, interface loss) surface two ways: a persistent per-universe/target status indicator in the watch view (never scrolls away), and a structured log line (timestamp, code, target/universe, detail) following the project's existing `{DOMAIN}_{CONDITION}` diagnostic-code convention.
- **D-12:** A per-universe/target output enable/disable control exists in Phase 4 itself (e.g. taking one bad node offline without stopping the whole rig) — independent of and preceding Phase 6's Blackout/Revoke Automation (PLAY-06/08/09), which act as higher-level overrides on top of this, not a replacement for it.

### Hardware/Simulator Compatibility Policy
- **D-13:** ARTN-06's acceptance tools are named now, not deferred: **Wireshark** (with its Art-Net/DMX dissector) for independent packet-level inspection (success criterion 2 — addressing, sequencing, payload length), and **Open Lighting Architecture (OLA)** as the independent receiving simulator/node (success criterion 4 — per-universe values, target behavior). Both are open-source and established in the lighting community.
- **D-14:** No real Art-Net hardware is currently owned. Real-hardware compatibility is tracked as an **open item**, following the same selection-≠-support pattern as Phase 6's MIDI-HW-01/02: simulator+packet-level verification can proceed now, but no named hardware-compatibility claim is made until a real device is independently evidenced.
- **D-15:** The evidence bar for a future real-hardware compatibility claim is a recorded Wireshark packet capture showing correct Art-Net 4 output reaching the node, plus an observed/recorded correct physical response from the fixture — simulator-only verification is not sufficient for a named hardware claim.

### Claude's Discretion
None — every gray area discussed converged on the recommended option; no "you decide" selections were made in this session.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Project-level requirements and roadmap
- `.planning/PROJECT.md` — Core value, and the "Live reliability" constraint ("DMX/Art-Net output and playback timing cannot depend on UI rendering, network-bound LLM inference, or script responsiveness") that D-03/D-04's standalone-process model exists to satisfy.
- `.planning/REQUIREMENTS.md` §Art-Net Output — ARTN-01 through ARTN-06 requirement text.
- `.planning/ROADMAP.md` §Phase 4: Observable Art-Net Live Output — Goal, four success criteria, and the research note flagging first-user nodes, subscriber/unicast behavior, multi-NIC/VPN cases, static-target workflow, compatibility policy, packet captures, and the physical hardware matrix as open research areas (this discussion resolved the interface/process model, network policy, health-surface, and hardware-acceptance-tool questions at the decision level; protocol-level implementation detail — exact sequence-numbering mechanics, refresh-rate tuning, Windows socket behavior — remains for phase research/planning).
- `.planning/ROADMAP.md` §Phase 6: Wails Authoring and Operator Surface — MIDI-HW-01/02 precedent (selection ≠ support, independent per-device evidence required before a named compatibility claim) that D-14/D-15 mirror for Art-Net hardware.

### Prior-phase precedent this phase should follow
- `.planning/phases/03-deterministic-show-programming-and-playback/03-CONTEXT.md` — The next-bar live-adoption boundary (D-05/D-06) that guarantees `Engine.CurrentFrame()` always reflects a complete, valid frame; D-09's frame-health measurement (D-09 above) reads this guarantee's health signal, it does not re-implement adoption logic.
- `.planning/phases/02-modular-fixtures-and-deployments/02-CONTEXT.md` — D-04 (shared `internal/command` typed command model that Phase 4's CLI commands should register through) and D-11 (auto-assigned Universe/Address per fixture instance — Phase 4 consumes this addressing, it does not invent it).

### Code
- `internal/playback/frame.go` — `Frame.Values map[uuid.UUID]scene.AttributeSet`; doc comment explicitly names "Phase 4 Art-Net worker" as an intended lock-free consumer of `Engine.CurrentFrame()`.
- `internal/playback/engine.go` — `Engine.CurrentFrame() *Frame`, the atomic.Pointer[Frame]-backed non-blocking read Phase 4's worker must use.
- `internal/deployment/model.go` — `Instance{Universe int, Address int}`, `channelsPerUniverse = 512`, `ValidateInstanceAddress`, `NextFreeAddress` — existing per-fixture-instance addressing Phase 4's packetizer maps directly to Art-Net universe/DMX-channel output.

No user-referenced ADRs/specs beyond the project's own planning docs came up during discussion — no additional canonical docs to add.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/playback/engine.go` `Engine.CurrentFrame()`: lock-free atomic.Pointer[Frame] read — the exact non-backpressuring consumption point ARTN-04 requires; already documented in-repo as anticipating this phase.
- `internal/playback/frame.go` `Frame.Values map[uuid.UUID]scene.AttributeSet`: semantic (not yet DMX-encoded) per-fixture-instance attribute values — Phase 4 must map these to DMX channel bytes via each fixture's channel/capability definition.
- `internal/deployment/model.go` `Instance.Universe`/`Instance.Address`: universe/address are already assigned per fixture instance from Phase 2 — Phase 4 consumes this, it doesn't design new addressing.
- `internal/command` (Phase 1/2/3 precedent, D-04 in 02-CONTEXT.md): shared typed command-registration model — Phase 4's interface-select/universe-config/target-config/status CLI operations should register here too, so Phase 6/7 can expose them later without rework.

### Established Patterns
- Package-per-concern layout under `internal/` — Phase 4 is greenfield for its own package(s) (e.g. `internal/artnet`).
- `{DOMAIN}_{CONDITION}` diagnostic code convention (e.g. `GOLC_DEPLOYMENT_ADDRESS_OUT_OF_RANGE`) — new Phase 4 diagnostics (interface loss, send failure, stalled frame) should follow the same naming convention.
- No DMX-encoding code exists anywhere in the repo yet — Phase 4 is the first place semantic `AttributeSet` float64 values become DMX byte values; there is no established mapping precedent to reuse, only the semantic model (`internal/scene/layer.go` `AttributeSet`) and fixture capability types to map from.

### Integration Points
- No `internal/artnet` (or networking/output) package exists yet — greenfield.
- The long-lived-process-with-local-IPC-clients model (D-03/D-04) is new architecture for this repo — no existing daemon/service or IPC pattern to reuse. This is the first phase establishing it, and Phase 6 (Wails UI) and Phase 7 (external API) will both need to attach to the same running instance as clients later.

</code_context>

<specifics>
## Specific Ideas

- Wireshark (Art-Net/DMX dissector) + Open Lighting Architecture (OLA) are the locked ARTN-06 verification tools — chosen specifically because they cover the two distinct success criteria (independent packet inspection vs. independent receiving-node behavior) rather than one tool trying to do both.
- No real Art-Net hardware is currently owned — real-hardware acceptance is an explicitly open, tracked item (not silently assumed done), following the MIDI-HW-02 pattern.
- The ROADMAP goal's "independently of the desktop UI" phrase was the key resolved ambiguity in this session: it means a genuinely standalone-capable long-lived process that never requires Wails to be running, not merely "a CLI command within the same app process."

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope. All four discussed areas (interface surface, network interface/target workflow, health/diagnostics surface, hardware/simulator compatibility policy) were clarifications of how to implement what's already in ARTN-01–06.

</deferred>

---

*Phase: 4-Observable Art-Net Live Output*
*Context gathered: 2026-07-22*
