# Phase 4: Observable Art-Net Live Output - Research

**Researched:** 2026-07-22
**Domain:** Art-Net 4 protocol output, Windows networking, long-lived process/IPC architecture, DMX channel encoding
**Confidence:** MEDIUM

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Interface & Process Model**
- **D-01:** Phase 4's operator surface is CLI-only for now (`golc artnet` subcommands) — no standalone GUI or Wails window. This matches Phase 1–3's headless precedent; Phase 6 later wraps these same typed commands in Wails without rework, given the shared `internal/command` model.
- **D-02:** The CLI health/status inspection (ARTN-05) supports both a continuously-refreshing watch view (human-readable table, live monitoring) and a one-shot snapshot (plain output by default, `--json` flag for scripting) — mirrors Phase 2's dry-run/human+JSON precedent (D-15).
- **D-03:** The Art-Net worker runs as part of one long-lived GOLC process alongside the playback engine. CLI commands (`golc artnet configure/status/...`) are clients that connect to this running process (local IPC) to configure or inspect it — they do not each own a separate output process.
- **D-04:** The long-lived process is standalone-capable: it can run entirely headless via CLI, with no Wails window ever required. Phase 6's Wails app is just one more client that attaches to the same running instance later. This is the literal reading of the ROADMAP goal's "independently of the desktop UI."

**Network Interface & Target Configuration**
- **D-05:** The selected Windows network interface is explicitly pinned by the operator. If it disappears or changes (multi-NIC machines, VPN adapters), GOLC stops sending on it and surfaces a clear error/degraded health state — it never silently auto-switches to a different interface (which could re-address a different subnet mid-show).
- **D-06:** Optional node discovery (ArtPoll/ArtPollReply) only surfaces compatible nodes as suggestions. Adding a discovered node as an active unicast target is always an explicit operator action — discovery never auto-populates or auto-removes live targets.
- **D-07:** Output is strictly per-target unicast. There is no broadcast mode — matches ROADMAP's own "static unicast targets" framing and keeps traffic scoped on shared networks/VPNs.
- **D-08:** A single universe can be configured to fan out to multiple unicast targets simultaneously (e.g. a universe split across two nodes, or a redundant backup node) — not strictly one universe → one target.

**Health & Diagnostics Surface**
- **D-09:** "Frame health" (ARTN-05) is measured as worker publish cadence plus staleness of the last frame read from `Engine.CurrentFrame()` (e.g. "last frame 8ms ago, on cadence" vs "no new frame in 400ms — engine may be stalled"). This distinguishes a stalled engine from a healthy one and directly demonstrates ARTN-04's non-backpressuring guarantee.
- **D-10:** "Target health" per configured unicast target is send success/error count plus a reachability signal (via ArtPollReply when the node supports polling) — not just the absence of OS-level send errors, so a genuinely dead node is distinguishable from one that simply doesn't answer polls.
- **D-11:** Errors (send failures, stalled frames, interface loss) surface two ways: a persistent per-universe/target status indicator in the watch view (never scrolls away), and a structured log line (timestamp, code, target/universe, detail) following the project's existing `{DOMAIN}_{CONDITION}` diagnostic-code convention.
- **D-12:** A per-universe/target output enable/disable control exists in Phase 4 itself (e.g. taking one bad node offline without stopping the whole rig) — independent of and preceding Phase 6's Blackout/Revoke Automation (PLAY-06/08/09), which act as higher-level overrides on top of this, not a replacement for it.

**Hardware/Simulator Compatibility Policy**
- **D-13:** ARTN-06's acceptance tools are named now, not deferred: **Wireshark** (with its Art-Net/DMX dissector) for independent packet-level inspection, and **Open Lighting Architecture (OLA)** as the independent receiving simulator/node. Both are open-source and established in the lighting community.
- **D-14:** No real Art-Net hardware is currently owned. Real-hardware compatibility is tracked as an **open item**, following the same selection-≠-support pattern as Phase 6's MIDI-HW-01/02.
- **D-15:** The evidence bar for a future real-hardware compatibility claim is a recorded Wireshark packet capture showing correct Art-Net 4 output reaching the node, plus an observed/recorded correct physical response from the fixture — simulator-only verification is not sufficient for a named hardware claim.

### Claude's Discretion
None — every gray area discussed converged on the recommended option; no "you decide" selections were made in this session.

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope. All four discussed areas (interface surface, network interface/target workflow, health/diagnostics surface, hardware/simulator compatibility policy) were clarifications of how to implement what's already in ARTN-01–06.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| ARTN-01 | Operator selects the Windows network interface used for Art-Net output and sees its current status | Windows interface enumeration via `net.Interfaces()`; interface-loss detection pattern (polling vs. `winipcfg`); see Architecture Patterns Pattern 2 |
| ARTN-02 | Operator configures Art-Net universes and static unicast targets and discovers compatible nodes when discovery is enabled | ArtPoll/ArtPollReply packet structure; universe→Port-Address mapping; D-06/D-07/D-08 unicast fan-out model; see Architecture Patterns Pattern 1/4 |
| ARTN-03 | Application emits valid Art-Net 4 output with correct addressing, sequence, payload-length, refresh, target behavior | ArtDMX packet byte layout, sequence wraparound, refresh cadence norms; see Code Examples, Common Pitfalls |
| ARTN-04 | Art-Net output consumes complete frames from the deterministic playback engine without being backpressured | `Engine.CurrentFrame()` non-blocking read (already built, Phase 3); non-blocking ticker-driven send loop pattern; see Architecture Patterns Pattern 3 |
| ARTN-05 | Operator inspects per-universe final values, frame health, target health, errors, output enablement | D-09/D-10/D-11 health model; CLI watch/snapshot pattern; see Architecture Patterns Pattern 5 |
| ARTN-06 | Release demonstrates packet and timing compatibility with an independent simulator and real hardware | Wireshark/OLA verification workflow, OLA's Windows-support gap; see Environment Availability, Common Pitfalls Pitfall 6 |
</phase_requirements>

## Summary

Phase 4 has two genuinely hard technical problems and one that is mostly a matter of following the spec carefully. The Art-Net 4 wire protocol itself (ArtDMX, ArtPoll/ArtPollReply) is small, well-documented, and stable — it is a good candidate to hand-encode directly against the spec in Go rather than depend on a third-party library, because every Go Art-Net library found during this research is either explicitly unstable (`jsimonetti/go-artnet`: "unfinished... API highly unstable") or too new/unproven to trust for a correctness-critical send path (`IanShelanskey/artnet-lib`: 0 stars, published days before this research, no adoption signal). The playback engine side of ARTN-04 is already built: `Engine.CurrentFrame()` is a lock-free `atomic.Pointer[Frame]` read Phase 3 built specifically anticipating this consumer, and `tickHz = 40` already encodes the DMX/Art-Net-standard ~40Hz refresh cadence as a named constant in `internal/playback/engine.go`. Phase 4's own worker only needs to read that pointer on its own ticker and never await a slow send.

The two hard problems are: (1) **there is currently no data anywhere in this codebase that says which DMX channel offset within a fixture's addressed span corresponds to which semantic capability** — `fixture.FixtureDefinition`/`fixture.Mode` carry no channel-order field at all (confirmed by reading `internal/fixture/model.go` and the OFL importer's own doc comment: "the canonical model has no channel-index concept at all"), so Phase 4 must design and add this mapping as new, additive data, not "reuse" something that already exists; and (2) **the long-lived-process-with-IPC-clients architecture (D-03/D-04) is entirely new to this repo** — every existing `internal/command` route today is a synchronous, in-process load-mutate-save-on-disk handler (see `internal/command/playback.go`), not a client of a separate running process. Both problems are addressable with well-understood, idiomatic Go patterns, but neither has an existing in-repo precedent to copy.

**Primary recommendation:** Hand-roll a minimal, spec-exact ArtDMX/ArtPoll/ArtPollReply encoder/decoder in a new `internal/artnet` package (do not adopt an unstable or unproven third-party Art-Net library for the correctness-critical send path); add an additive per-mode DMX-channel-order field to the fixture model so `scene.AttributeSet` → DMX byte mapping has real data to work from; and reuse this repo's existing `command.Request`/`command.Result` JSON-serializable shapes as the wire format for a new named-pipe (Windows) IPC bridge between short-lived `golc artnet ...` CLI invocations and one long-lived worker process.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Frame evaluation (semantic AttributeSet values) | API/Backend (playback engine, existing) | — | Already built in Phase 3; Phase 4 only reads `Engine.CurrentFrame()`, never re-evaluates |
| AttributeSet → DMX byte mapping | API/Backend (new `internal/artnet` or `internal/fixture` extension) | — | Pure, deterministic transform; must live next to fixture channel-order data, not in the network worker |
| ArtDMX/ArtPoll packet encode/decode | API/Backend (new `internal/artnet`) | — | Protocol-level concern, no UI/CLI dependency; must be independently unit-testable against fixed byte vectors |
| UDP unicast send loop / cadence | API/Backend (new `internal/artnet` worker, long-lived process) | — | Must never block on network I/O (ARTN-04); owns its own ticker, independent of the CLI process lifetime |
| Windows interface selection & loss detection | OS/Backend boundary (new `internal/artnet` interface manager) | — | Requires OS-level interface enumeration (`net.Interfaces()`) and optionally Windows-specific change notification |
| CLI configuration/status commands (`golc artnet ...`) | CLI client (existing `internal/command` model, new routes) | IPC transport | Per D-01/D-03, these are thin clients; the long-lived worker process owns state |
| Long-lived worker + IPC listener | API/Backend (new standalone-capable process) | — | Per D-03/D-04; this is the "server" side every CLI invocation and, later, Wails/API client attaches to |
| Node discovery (ArtPoll) | API/Backend (new `internal/artnet`) | CLI (suggestions surface) | Discovery results are read-only suggestions (D-06); the CLI only renders them, never auto-applies them |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go standard library `net`/`net.UDPConn` | go1.26.5 (this repo's pin) | UDP unicast send/receive for ArtDMX/ArtPoll | No external dependency needed; `net.ListenUDP`/`net.DialUDP` are sufficient for unicast-only output (D-07 rules out any broadcast-socket-option complexity) `[CITED: pkg.go.dev/net]` |
| Go standard library `net` (`net.Interfaces`, `net.InterfaceAddrs`) | go1.26.5 | Windows network interface enumeration for ARTN-01 | Cross-platform, already a transitive dependency of every Go program; Windows exposes interface up/down via `Flags` `[CITED: pkg.go.dev/net, cross-checked against Windows IF_OPER_STATUS docs]` |
| `github.com/microsoft/go-winio` | latest tagged (verify at implementation time) | Windows named-pipe listener/dialer implementing `net.Listener`/`net.Conn` for the D-03/D-04 local IPC transport | Official Microsoft-maintained package, widely used by Docker/containerd/Moby for exactly this purpose (Windows named-pipe IPC with a `net`-compatible interface) `[CITED: github.com/microsoft/go-winio README]` |
| `encoding/json` (already in use via this repo's `internal/strictjson`) | stdlib | Wire format for the IPC request/response bridge | Matches this repo's existing `command.Request`/`command.Result` shapes and `strictjson.CanonicalEncode` determinism convention already used elsewhere (e.g. `internal/command/playback.go`'s `--json` output) `[VERIFIED: internal/strictjson package already in this repo]` |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `golang.zx2c4.com/wireguard/windows/tunnel/winipcfg` | latest | Real-time Windows interface change notification (`ChangeCallback`) instead of polling `net.Interfaces()` | Only if a ~1Hz poll of `net.Interfaces()` proves too slow to satisfy D-05's "stops sending... surfaces degraded health" requirement in practice; adds a heavier dependency tree (used by WireGuard for Windows) `[CITED: pkg.go.dev/golang.zx2c4.com/wireguard/windows/tunnel/winipcfg, cross-checked against wireguard-windows source]` |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Hand-rolled ArtDMX/ArtPoll encoder | `jsimonetti/go-artnet` | Implements full Art-Net 4 opcode set including ArtPoll/ArtPollReply and RDM, but its own README states "unfinished... API highly unstable" — a risky foundation for ARTN-03's correctness requirement `[ASSUMED — package existence and README wording confirmed via WebFetch, but "unstable" self-assessment is the maintainer's own characterization, not independently verified against a specific commit]` |
| Hand-rolled ArtDMX/ArtPoll encoder | `IanShelanskey/artnet-lib` | Published August 2025, 0 GitHub stars, 0 forks, no visible adoption — classic slopsquatting-adjacent risk profile even though it is a real, existing package; see Package Legitimacy Audit below |
| Hand-rolled ArtDMX/ArtPoll encoder | `github.com/qmsk/dmx` (artnet subpackage) | More mature (31 stars, ~192 commits) and already implements unicast subscription + ArtPoll/ArtPollReply discovery, but is not a standalone module import path documented for modern Go modules usage and its maintenance activity/date could not be confirmed in this session `[ASSUMED]` |
| Windows named pipes (`go-winio`) | Loopback TCP (`127.0.0.1:<port>`) | Simpler (no extra dependency), but requires manual application-layer auth since any local process/user can connect to a loopback port; named pipes integrate with Windows ACLs natively `[CITED: comcomponent.com Windows IPC decision guidance, cross-checked against Microsoft's own gRPC-named-pipes docs]` |
| Polling `net.Interfaces()` for D-05 loss detection | `winipcfg.ChangeCallback` (event-driven) | Polling is dependency-free and matches this repo's near-zero-dependency convention (`go.mod` currently has 4 direct deps); event-driven is lower-latency but pulls in the WireGuard-for-Windows dependency tree for one callback |

**Installation:**
```bash
go get github.com/microsoft/go-winio@latest
# Only if event-driven interface-change detection is chosen over polling:
go get golang.zx2c4.com/wireguard/windows/tunnel/winipcfg@latest
```

**Version verification:** Run before locking versions in the plan:
```bash
go list -m -versions github.com/microsoft/go-winio
go list -m -versions golang.zx2c4.com/wireguard/windows/tunnel/winipcfg
```
This session did not run `go list -m -versions` against the live proxy (no network egress budget spent on registry pings beyond the searches above); the planner/executor must confirm the current tagged version before adding either dependency to `go.mod`, per this repo's own `go.sum`/pin-immutability convention already enforced elsewhere (see STATE.md: "Bootstrap hashes go.mod/go.sum around every module operation and hard-fails on mutation").

## Package Legitimacy Audit

This phase's core protocol path (ArtDMX/ArtPoll encode/decode) is recommended to be **hand-rolled**, not imported, specifically because of the findings below. `go-winio` is the one new external dependency this research recommends actually adding.

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| `github.com/microsoft/go-winio` | Go module proxy | Long-established (Docker/Moby-era origin) | N/A (Go modules have no download-count registry) | github.com/microsoft/go-winio, actively maintained by Microsoft | OK | Approved — recommended for D-03/D-04 IPC transport |
| `github.com/jsimonetti/go-artnet` | Go module proxy | Multi-year, 38 stars / 22 forks, commits into 2025 | N/A | github.com/jsimonetti/go-artnet | SUS | Flagged — self-described "unfinished... API highly unstable" in its own README; **not recommended** for the correctness-critical ArtDMX send path. If the planner still wants to depend on it (e.g. only for ArtPoll parsing convenience), add a `checkpoint:human-verify` task to pin an exact commit/tag and byte-verify its ArtDMX output against the spec before trusting it. |
| `github.com/IanShelanskey/artnet-lib` | Go module proxy | Days old at research time (initial release dated 2026-08-03 per its own tag — note: this date is in the future relative to typical training cutoffs and should be re-verified at plan time, as it may indicate either a very recent real release or a WebFetch date-rendering artifact) | N/A | github.com/IanShelanskey/artnet-lib | SLOP-adjacent (SUS) | **Removed from recommendations.** Zero stars, zero forks, no visible adoption, unverified author reputation. Discovered only via WebSearch/training-adjacent lookup, not an official doc — tag any future mention `[ASSUMED]` and gate behind `checkpoint:human-verify` if ever considered. |
| `github.com/qmsk/dmx` (artnet subpackage) | Go module proxy | Multi-year, 31 stars, ~192 commits | N/A | github.com/qmsk/dmx | SUS (unverified currency) | Flagged — functionally the most complete (unicast + ArtPoll discovery), but this session could not confirm last-commit recency or current Go-modules compatibility. Add `checkpoint:human-verify` before adopting; otherwise treat as reference material only for how a mature Go Art-Net implementation shapes its API, not as a dependency. |

**Packages removed due to [SLOP] verdict:** none formally SLOP-classified, but `IanShelanskey/artnet-lib` is treated as removed/do-not-use given its adoption profile.
**Packages flagged as suspicious [SUS]:** `jsimonetti/go-artnet`, `qmsk/dmx` — both are real, legitimate open-source projects (not hallucinated), but neither should be adopted as a dependency without an explicit `checkpoint:human-verify` task confirming current maintenance state and byte-exact packet correctness against the spec, given the Go ecosystem tooling used elsewhere in this research (`npm view`-equivalent registry download-count checks) does not exist for Go modules.

*No package in this phase was discovered via an official-docs-confirmed source with a download-count signal (Go modules have none); every third-party candidate above is tagged with its actual verification limitation rather than presented as verified.*

## Architecture Patterns

### System Architecture Diagram

```
                     ┌────────────────────────────────────────────────────────┐
                     │         Long-lived GOLC process (D-03/D-04)              │
                     │                                                          │
  Playback Engine    │   ┌──────────────┐        ┌───────────────────────┐    │
  (Phase 3, exists) ─┼──▶│ activeFrame  │──Load──▶│  Art-Net Worker       │    │
  atomic.Pointer     │   │ atomic.Ptr   │ (non-   │  (own ticker, 40Hz)   │    │
  [Frame]            │   └──────────────┘ blocking)│                       │    │
                     │                              │  1. Read CurrentFrame│    │
                     │                              │  2. Map AttributeSet │    │
                     │                              │     → DMX bytes per  │    │
                     │                              │     fixture/universe │    │
                     │                              │  3. Build ArtDMX pkt │    │
                     │                              │     (seq++, len,     │    │
                     │                              │     Net/SubUni)      │    │
                     │                              │  4. Fan out to N     │    │
                     │                              │     unicast targets  │    │
                     │                              │     per universe     │    │
                     │                              │     (bounded, per-   │    │
                     │                              │     target timeout)  │    │
                     │                              └──────────┬────────────┘    │
                     │                                         │ UDP unicast     │
                     │                              ┌──────────▼────────────┐    │
                     │                              │ Interface Manager     │    │
                     │                              │ (pinned NIC, D-05)    │    │
                     │                              │ - net.Interfaces()    │    │
                     │                              │   poll or winipcfg    │    │
                     │                              │   change callback     │    │
                     │                              │ - health/error state  │    │
                     │                              └──────────┬────────────┘    │
                     │                                         │                │
                     │   ┌──────────────────────────┐          │                │
                     │   │  IPC Listener (named pipe │          │                │
                     │   │  or loopback TCP)         │          ▼                │
                     │   │  serves command.Request/  │   Windows NIC (pinned)    │
                     │   │  Result over the wire      │   ──▶ Art-Net node(s)     │
                     │   └──────────▲──────────────┬─┘    (OLA / hardware /     │
                     └──────────────┼──────────────┼──────  Wireshark capture)  ┘
                                    │              │
                         IPC client │              │ ArtPoll/ArtPollReply
                                    │              │ (discovery, D-06)
                  ┌─────────────────┴───┐   ┌───────▼────────────┐
                  │ golc artnet         │   │ Optional discovery │
                  │ configure/status    │   │ scan on operator    │
                  │ (short-lived CLI    │   │ request only         │
                  │ process, D-01/D-03) │   └──────────────────────┘
                  └──────────────────────┘
```

### Recommended Project Structure
```
internal/
├── artnet/
│   ├── packet.go          # ArtDMX/ArtPoll/ArtPollReply encode/decode, byte-exact, no I/O
│   ├── packet_test.go      # Golden byte-vector tests against the spec
│   ├── channelmap.go       # AttributeSet -> ordered DMX byte slice, per fixture Mode
│   ├── channelmap_test.go
│   ├── worker.go           # Ticker-driven, non-blocking send loop (ARTN-04)
│   ├── worker_test.go
│   ├── target.go           # Unicast target config, per-target send state/health (D-08/D-10)
│   ├── interfacemgr.go     # Pinned-interface selection, loss detection (D-05)
│   ├── interfacemgr_test.go
│   ├── discovery.go        # ArtPoll broadcast + ArtPollReply collection (D-06, suggestions only)
│   ├── health.go           # Frame/target health model (D-09/D-10/D-11)
│   ├── ipc/
│   │   ├── server.go        # Named-pipe/TCP listener wrapping command.Request/Result
│   │   └── client.go        # CLI-side dialer used by internal/command's artnet routes
│   └── daemon.go            # Long-lived process entrypoint wiring Engine + worker + IPC listener
internal/command/
└── artnet.go               # golc artnet configure/status/... routes; thin IPC clients (D-01/D-03)
internal/fixture/
└── model.go                # Additive: Mode gains an ordered DMX-channel-layout field (see Pitfall 1)
```

### Pattern 1: Byte-exact ArtDMX packet construction (hand-rolled, spec-driven)

**What:** Build the 18-byte-plus-data ArtDMX header directly from documented field offsets rather than through a third-party library.
**When to use:** Every outbound DMX frame, every universe, every unicast target.
**Example:**
```go
// Source: cross-checked art-net.org.uk (Art-Net 4 spec) field layout against
// en.wikipedia.org/wiki/Art-Net and github.com/jsimonetti/go-artnet/packet/code/opcode.go
// [CITED: art-net.org.uk Art-Net 4 spec; cross-checked, MEDIUM confidence]
const (
	artNetPort   = 6454 // 0x1936, fixed Art-Net UDP port
	opOutputDMX  = 0x5000
	protVerHi    = 0x00
	protVerLo    = 0x0e // protocol version 14
)

// EncodeArtDMX builds one ArtDMX packet. seq is 1..255 (0x00 disables
// sequencing per spec — never send 0 once sequencing is enabled).
// portAddress packs Net (bits 8-14) + Sub-Net (bits 4-7) + Universe (bits 0-3)
// into the 15-bit Port-Address; data length must be even, 2..512.
func EncodeArtDMX(seq, physical uint8, portAddress uint16, data []byte) ([]byte, error) {
	if len(data) < 2 || len(data) > 512 || len(data)%2 != 0 {
		return nil, fmt.Errorf("GOLC_ARTNET_DMX_LENGTH_INVALID: length %d must be even and in [2,512]", len(data))
	}
	buf := make([]byte, 18+len(data))
	copy(buf[0:8], []byte("Art-Net\x00"))
	binary.LittleEndian.PutUint16(buf[8:10], opOutputDMX) // OpCode is little-endian on the wire
	buf[10] = protVerHi
	buf[11] = protVerLo
	buf[12] = seq
	buf[13] = physical
	buf[14] = byte(portAddress & 0xff)        // SubUni: low nibble Sub-Net, high nibble Universe
	buf[15] = byte((portAddress >> 8) & 0x7f) // Net: top 7 bits, bit 7 reserved/zero
	binary.BigEndian.PutUint16(buf[16:18], uint16(len(data)))
	copy(buf[18:], data)
	return buf, nil
}
```

### Pattern 2: Windows interface enumeration + pinned-interface health

**What:** List candidate NICs for ARTN-01's selection UI, and detect when the pinned one disappears (D-05).
**When to use:** Startup selection prompt, and the interface manager's own health loop.
**Example:**
```go
// Source: pkg.go.dev/net, cross-checked against Windows IF_OPER_STATUS docs
// [CITED: pkg.go.dev/net; MEDIUM confidence]
func ListCandidateInterfaces() ([]InterfaceInfo, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("GOLC_ARTNET_INTERFACE_ENUM_FAILED: %v", err)
	}
	var out []InterfaceInfo
	for _, iface := range ifaces {
		addrs, _ := iface.Addrs()
		out = append(out, InterfaceInfo{
			Index: iface.Index,
			Name:  iface.Name, // Note: Windows may report a GUID-shaped name on some builds
			Up:    iface.Flags&net.FlagUp != 0,
			Addrs: addrs,
		})
	}
	return out, nil
}

// pollInterfaceLoss runs on its own low-frequency ticker (independent of the
// 40Hz send ticker) and re-checks that the pinned interface (by Index, not
// by Name — names can be GUID-shaped/unstable) is still present and up.
func (m *InterfaceManager) pollInterfaceLoss(ctx context.Context) {
	ticker := time.NewTicker(time.Second) // 1Hz is adequate for D-05's "clear degraded state," not a hot path
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := net.InterfaceByIndex(m.pinnedIndex); err != nil {
				m.markLost() // never auto-switches to a different interface (D-05)
			}
		}
	}
}
```

### Pattern 3: Non-blocking, ticker-driven, bounded-fanout send loop

**What:** Read `Engine.CurrentFrame()` on its own cadence and fan out to every configured unicast target without ever blocking on a slow/unreachable node.
**When to use:** The Art-Net worker's main loop — this is the mechanism that satisfies ARTN-04.
**Example:**
```go
// Source: pattern synthesized from Go community backpressure guidance
// (per-target timeout + bounded goroutine fanout; drop-and-count over block)
// [ASSUMED — general Go concurrency pattern, not a named library; cross-check
// against this repo's own internal/playback/engine.go non-blocking-read
// convention before finalizing in the plan]
func (w *Worker) tick(frame *playback.Frame) {
	for _, universe := range w.configuredUniverses() {
		data := w.channelMap.Encode(universe, frame) // pure, in-memory, must not block
		pkt, err := EncodeArtDMX(w.nextSeq(universe), 0, universe.PortAddress, data)
		if err != nil {
			w.health.RecordEncodeError(universe, err) // D-11 diagnostic, never panics the loop
			continue
		}
		for _, target := range universe.Targets {
			target := target
			go func() {
				// Per-send deadline bounds worst case; a hung target can
				// never stall the next tick, which fires on its own ticker
				// regardless of whether prior sends completed (ARTN-04).
				_ = target.conn.SetWriteDeadline(time.Now().Add(w.sendTimeout))
				_, sendErr := target.conn.Write(pkt)
				w.health.RecordSend(universe, target, sendErr) // D-10
			}()
		}
	}
}
```
**Note:** an unbounded `go func()` per target per tick is acceptable only because target counts are small (D-08's "fan out to multiple targets" is a handful, not hundreds); if the plan wants a hard ceiling, bound concurrent in-flight sends per target to 1 (skip a tick's send for a target still busy from the previous tick) rather than letting goroutines pile up under a persistently slow target.

### Pattern 4: ArtPoll discovery as a read-only suggestion list (D-06)

**What:** Broadcast ArtPoll, collect ArtPollReply responses for a bounded window, surface them as suggestions only.
**When to use:** Only when the operator explicitly triggers `golc artnet discover`.
**Example:**
```go
// Source: cross-checked art-net.org.uk spec description against
// jsimonetti/go-artnet opcode constants (OpPoll=0x2000, OpPollReply=0x2100)
// [CITED: art-net.org.uk; MEDIUM confidence]
// ArtPoll itself is broadcast per spec (2.255.255.255) purely for discovery;
// this does not conflict with D-07's "no broadcast" rule, which governs the
// live DMX *output* path, not the opt-in discovery scan.
func Discover(ctx context.Context, iface InterfaceInfo, window time.Duration) ([]DiscoveredNode, error) {
	// send one ArtPoll, then collect ArtPollReply for `window`
	// (spec: controller may assume max 3s timeout for replies)
	// every result returned here is a suggestion; nothing is auto-added
	// as a live target (D-06) — the CLI layer owns that explicit-action gate.
	...
}
```

### Pattern 5: CLI as thin IPC client, reusing existing Request/Result shapes

**What:** `golc artnet configure/status` routes marshal `command.Request` and get back `command.Result` — but over IPC to the long-lived worker process, not via the existing in-process load-mutate-save pattern every other command in `internal/command` uses today.
**When to use:** Every `golc artnet ...` route (D-01/D-03).
**Example:**
```go
// This repo's existing command.Request/command.Result (internal/command/router.go)
// are already small, JSON-serializable structs. Reusing them as the IPC wire
// shape avoids inventing a second protocol for the same concept.
// [VERIFIED: internal/command/router.go Request/Result types, read this session]
var _ = command.MustDeclareRoute(command.CommandRegistration{
	Route:   "artnet status",
	Summary: "Show per-universe/target frame health, target health, errors, and output enablement.",
	Handler: runArtnetStatus, // dials the IPC listener, forwards Request, relays Result
})

func runArtnetStatus(request command.Request) command.Result {
	conn, err := ipcclient.Dial() // named pipe on Windows via go-winio
	if err != nil {
		return command.Result{ExitCode: 1, Stderr: []byte(
			"GOLC_ARTNET_DAEMON_UNREACHABLE: is the GOLC background process running?\n")}
	}
	defer conn.Close()
	return ipcclient.Forward(conn, request)
}
```

### Anti-Patterns to Avoid
- **Deriving DMX channel order from `fixture.Capabilities`' declaration order:** the canonical fixture model's `Capabilities` slice order reflects the YAML author's/OFL importer's arbitrary declaration order, not the fixture's real wiring order. Treating declaration order as channel order will silently produce *wrong* DMX output for any multi-capability fixture whose author ordered capabilities differently than the fixture's real channel layout (see Pitfall 1).
- **Depending on an unstable/unproven Art-Net library for the send path:** see Package Legitimacy Audit — both realistic candidates found in this research are flagged; hand-roll the small, stable, spec-defined encode/decode instead.
- **Blocking the tick loop on any single target's send:** matches this repo's own `internal/playback/engine.go` convention (never await a slow consumer) — apply the identical discipline to the Art-Net worker's own send fanout, not just to the Engine→Worker boundary.
- **Auto-switching the pinned interface on loss:** explicitly forbidden by D-05; the correct behavior is "stop sending + surface degraded health," never silently retarget a different NIC/subnet mid-show.
- **Running OLA directly on the same Windows box expecting native support:** OLA does not have first-class native Windows support (see Environment Availability) — plan the verification workflow around a separate host, not an in-place Windows install.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Windows named-pipe transport internals (IOCP wiring, pipe security descriptors) | A custom Win32 named-pipe wrapper via raw syscalls | `github.com/microsoft/go-winio` | Already solves IOCP-based non-blocking pipe I/O correctly and exposes a `net.Listener`/`net.Conn`-compatible API; this is exactly the kind of low-level OS plumbing worth depending on rather than reimplementing `[CITED: github.com/microsoft/go-winio README]` |
| Wireshark Art-Net dissection | A custom packet capture/decode tool for manual verification | Wireshark's built-in Art-Net/DMX dissector (`packet-artnet.c`, upstreamed in Wireshark itself) | D-13 already locks this; Wireshark's dissector is maintained as part of Wireshark core and handles every documented Art-Net opcode `[CITED: github.com/wireshark/wireshark packet-artnet.c]` |
| An independent Art-Net receiving node/simulator for ARTN-06 | A GOLC-internal "fake receiver" test harness only | Open Lighting Architecture (OLA) as the primary independent verification target, per D-13 | Using GOLC's own code to verify GOLC's own output is not independent verification; OLA is a separate, established, non-GOLC codebase — but see Environment Availability for the Windows-hosting caveat |

**Key insight:** The one place this phase *should* hand-roll rather than reuse is the ArtDMX/ArtPoll wire encoding itself — not because "don't hand-roll" doesn't apply, but because the two realistic third-party options both carry more correctness risk (unstable API, zero-adoption package) than a ~40-line, spec-exact, fully unit-testable encoder that this repo's own engineers can byte-verify against golden vectors. This is a deliberate exception, not a contradiction of the general rule.

## Common Pitfalls

### Pitfall 1: No DMX channel-order data exists anywhere in the fixture model today
**What goes wrong:** A plan that assumes "map AttributeSet to DMX bytes" is a small, mechanical step will discover mid-implementation that `fixture.FixtureDefinition`/`fixture.Mode` (as built in Phase 2, status "Complete") carry zero information about which channel offset a capability occupies within a fixture's addressed span. `internal/fixture/ofl/normalize.go`'s own doc comment states this explicitly: OFL's per-mode channel-order list "has nothing to normalize into" in the current canonical model.
**Why it happens:** Phase 2 deliberately kept the canonical fixture model channel-agnostic (protocol-agnostic by design, per its own doc comment: "Capability.Range values are normalized 0..1, never raw DMX... the model stays protocol-agnostic"). That was the right call for Phase 2's scope, but it means Phase 4 is the first phase that needs channel-order data, and nothing upstream produced it.
**How to avoid:** Add an **additive** (non-breaking) per-mode ordered channel-layout field — e.g. `fixture.Mode.Channels []fixture.CapabilityType` (with a same-type occurrence index for fixtures declaring more than one capability of the same type) — populated at fixture-authoring time (hand-authored YAML) and, for OFL imports, derived from OFL's own `modes[].channels` array (which the current importer already decodes as `Mode{Name}` only and discards the channel list — re-adding it is additive to `ofl/model.go`'s existing `Mode` struct, not a breaking change). Existing fixtures/shows with no `Channels` populated need an explicit fallback rule (e.g. reject at Art-Net-configure time with a clear diagnostic, rather than silently guessing an order) — this fallback rule is itself something CONTEXT.md left undecided and must be resolved during planning/discussion.
**Warning signs:** Any plan task that says "encode AttributeSet to DMX bytes using the fixture's Capabilities order" without first adding a channel-order field is building on a phantom precedent — verify this data exists before writing that task's action.

### Pitfall 2: Sequence number 0 has special meaning — do not start counters at 0
**What goes wrong:** Art-Net's Sequence field is 1-255 for reordering; 0x00 explicitly *disables* sequence checking on the receiving node. A naive `uint8` counter starting at 0 and incrementing will send one 0x00 packet before wrapping into the 1-255 range, and will periodically wrap back through 0 every 256 packets, intermittently disabling the receiver's reordering logic.
**Why it happens:** The natural Go idiom (`seq++` on a zero-initialized `uint8`) walks through 0 every 256 increments.
**How to avoid:** Wrap 1→255→1, skipping 0 entirely in the increment step (`if seq == 255 { seq = 1 } else { seq++ }`, starting the first packet at 1). `[CITED: art-net.org.uk / cross-checked Wikipedia Art-Net summary; MEDIUM confidence]`
**Warning signs:** A byte-vector unit test asserting the sequence never equals 0 across a long simulated run is cheap and catches this immediately.

### Pitfall 3: GOLC's flat "universe" integer is not the same as Art-Net's 15-bit Port-Address
**What goes wrong:** `internal/deployment/model.go`'s `Instance.Universe` is a plain `int`, scanned 1..64 by `NextFreeAddress` (`maxUniverseSearch = 64`). Art-Net's addressing is a 15-bit Port-Address split into Net (7 bits) + Sub-Net (4 bits) + Universe (4 bits), packed across two packet bytes (byte 14 = Sub-Net<<4 | Universe, byte 15 = Net, top bit reserved zero). These are not the same number space, and no existing code decides how one maps to the other.
**Why it happens:** Phase 2's `Instance.Universe` was scoped to same-network small-rig addressing (its own doc comment: "small-rig scale target (~10-50 fixtures across 3-8 pools)"), not to Art-Net's wire-level Net/Sub-Net/Universe split.
**How to avoid:** The simplest correct mapping for this project's declared scale (small rig, universes 1-64) is Net=0 fixed, Sub-Net = `(universe >> 4) & 0xF`, Universe = `universe & 0xF` — this covers Port-Addresses 0-255 (16 sub-nets × 16 universes) with headroom well beyond `maxUniverseSearch`'s ceiling of 64. `[ASSUMED — this is a reasonable, spec-consistent mapping given the existing small-rig-scale addressing convention, but it was not explicitly decided in CONTEXT.md and should be confirmed/locked during planning, not silently assumed by an implementation task]`
**Warning signs:** Any ARTN-02 configuration UI/CLI that lets an operator type an arbitrary "universe" number without this mapping being explicit and tested will produce Port-Addresses that silently collide or wrap in ways that are hard to debug from a packet capture alone.

### Pitfall 4: `net.Interfaces()`'s `Name` field is not a stable identifier on Windows
**What goes wrong:** Pinning an interface "by name" (D-05) and later re-resolving it by name can fail or resolve to the wrong adapter, because Windows sometimes reports GUID-shaped interface names (e.g. `{A3768A7A-8E5F-42F8-81E3-3BCAACEC9FBE}`) rather than a stable human-friendly name, and adapter friendly-name vs. GUID exposure has been inconsistent across Windows versions in Go's `net` package history.
**Why it happens:** Go's cross-platform `net.Interfaces()` abstraction doesn't fully normalize Windows' own adapter-naming quirks.
**How to avoid:** Pin by `net.Interface.Index` (a stable integer for the life of the adapter instance) as the primary identity, and store the friendly name only for display purposes; re-validate by index, not by name, in the health-loop and at process restart. `[CITED: haydz.github.io Go-Windows-NIC writeup, cross-checked against golang/go issue #12301 "net: missing interfaces on Windows"; MEDIUM confidence]`
**Warning signs:** An interface-selection flow that persists only a name string, with no numeric index recorded alongside it, is a sign this pitfall wasn't accounted for.

### Pitfall 5: `SO_BINDTODEVICE` does not exist on Windows
**What goes wrong:** A Linux-influenced design that assumes "bind the outbound UDP socket to a specific NIC via a socket option" will not compile/work as-is on Windows — `SO_BINDTODEVICE` is Linux-specific.
**Why it happens:** Many UDP-multi-homing tutorials and Stack Overflow answers default to Linux socket-option examples.
**How to avoid:** On Windows, constrain outbound routing by binding `net.ListenUDP`/`net.DialUDP` to the pinned interface's *local IP address* (not `0.0.0.0`), which is the portable, cross-platform-correct approach `net`'s own API already supports; this also matches D-05/D-07's model (one pinned interface, unicast-only) without needing any Windows-specific syscall. `[CITED: multiple WebSearch-aggregated sources on Windows SO_BINDTODEVICE unavailability; LOW-MEDIUM confidence — recommend a small spike/smoke test against the actual target Windows machine before finalizing, since exact routing behavior with multiple active adapters on the same subnet can still be OS-routing-table-dependent even when bound to a specific local IP]`
**Warning signs:** Any code referencing `syscall.SO_BINDTODEVICE` under a Windows build tag is a red flag — that constant does not have a meaningful Windows equivalent to bind to.

### Pitfall 6: OLA (the D-13-locked simulator) does not have first-class native Windows support
**What goes wrong:** A plan/verification task that assumes "install and run OLA on the same Windows machine as GOLC" will stall — OLA's own project documentation states it runs on Linux and Mac OS X, with only some features working on Windows, and the project's own tutorial for Windows users is "OLA on Windows via VMWare" (i.e., run OLA inside a Linux VM).
**Why it happens:** D-13 locked the *tool choice* (OLA) as the independent-simulator answer, but CONTEXT.md's discussion did not resolve *where OLA physically runs* relative to a Windows-only GOLC v1 target.
**How to avoid:** This does not block D-13/D-14 — OLA only needs to be reachable as a unicast Art-Net target over the network, it does not need to run on the GOLC Windows host itself. Plan the ARTN-06 verification workflow around OLA running on a separate Linux/macOS host (a spare Linux box, a Raspberry Pi, a Linux VM with a **bridged** network adapter — not NAT/WSL2's default NAT mode, which would not present OLA with a directly routable IP for GOLC's static-unicast-target model) reachable from the GOLC Windows machine's pinned interface. `[CITED: openlighting.org getting-started/downloads and "OLA on Windows via VMWare" tutorial page; MEDIUM confidence]`
**Warning signs:** A verification plan task with no explicit second machine/VM/network topology called out for where OLA runs is a sign this constraint wasn't accounted for.

## Code Examples

### DMX512/Art-Net refresh cadence — already encoded in this repo
```go
// internal/playback/engine.go, already committed (Phase 3):
// tickHz is the engine's tick cadence (03-RESEARCH.md Open Question 1/
// Assumption A2): 40Hz is the recommended concrete constant within the
// DMX/Art-Net industry-standard 30-40Hz refresh band, documented as
// adjustable -- Phase 4 (Art-Net) may tune this single constant without
// any architecture change.
const tickHz = 40
```
`[VERIFIED: internal/playback/engine.go, read this session]` — this confirms ARTN-03's "refresh" requirement can piggyback directly on the existing 40Hz tick rather than inventing a second cadence; the Art-Net worker's own send ticker should either share this constant or run at the same 40Hz independently (a design choice for the plan: sharing avoids drift between engine ticks and Art-Net sends, but couples the two tick loops more tightly — recommend the worker runs its *own* independent 40Hz ticker rather than being driven directly by the engine's tick callback, preserving ARTN-04's decoupling guarantee even if the two rates are numerically identical).

### Non-blocking Engine read — already committed (Phase 3)
```go
// internal/playback/engine.go
func (e *Engine) CurrentFrame() *Frame {
	return e.activeFrame.Load()
}
```
`[VERIFIED: internal/playback/engine.go, read this session]` — this is the exact non-backpressuring consumption point ARTN-04 requires; the Art-Net worker calls this once per its own tick and never blocks the Engine's own tick loop regardless of how slow the worker's own send fanout is.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| Art-Net 2/3: 8-bit then 15-bit universe addressing, single-IP-per-4-ports gateway limitation | Art-Net 4: same 15-bit Port-Address, but adds a multi-homing-free scheme letting one IP address support 1000+ DMX ports, plus optional sACN-as-data-path-with-Art-Net-as-management-layer | Art-Net 4 spec publication (Artistic Licence) | 100% backwards compatible per the spec; GOLC targeting "Art-Net 4" per ARTN-03 mainly matters for the addressing/gateway-management features, not for ArtDMX's own wire format, which is unchanged from Art-Net 3 `[CITED: artisticlicence.com Art-Net overview, cross-checked against art-net.org.uk background page; MEDIUM confidence]` |
| Linux `SO_BINDTODEVICE` interface binding | Windows: bind by local IP address via `net.ListenUDP`, or `IP_UNICAST_IF` for lower-level control | N/A (platform difference, not a version change) | Directly affects how the Interface Manager (D-05) implements "pin this NIC" on the actual target OS |

**Deprecated/outdated:**
- Broadcast-based Art-Net discovery/output as the only model: this phase's D-07 (no broadcast for output) and D-06 (discovery is opt-in, suggestion-only) already reflect the more security-conscious, VPN/shared-network-aware pattern that has become standard practice in professional Art-Net deployments, versus the historically common "just broadcast everything on 2.255.255.255" pattern still described in older Art-Net tutorials.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | GOLC's flat `Instance.Universe` integer should map to Art-Net Port-Address as Net=0, Sub-Net=`universe>>4`, Universe=`universe&0xF` | Common Pitfalls Pitfall 3 | If wrong, ARTN-02's universe configuration silently addresses the wrong Port-Address, producing packets a real/simulated node either ignores or misroutes — must be locked as an explicit decision (not left implicit in code) before implementation |
| A2 | The Art-Net worker should run its own independent 40Hz ticker rather than being driven by the playback Engine's own tick callback | Code Examples | Low risk either way for correctness, but sharing a single ticker callback would re-couple the two components CONTEXT.md and Phase 3 deliberately decoupled (ARTN-04); worth an explicit planning decision rather than defaulting silently |
| A3 | A per-mode DMX channel-order field should be added additively to `fixture.Mode` (rather than a Phase-4-scoped side table keyed by fixture identity) | Common Pitfalls Pitfall 1, Don't Hand-Roll | If the alternative (side table) is actually preferred to avoid touching a "Complete"-status Phase 2 schema, the plan's task shape for this data model change is different; this choice needs an explicit CONTEXT-style decision before planning locks task structure |
| A4 | `jsimonetti/go-artnet` and `qmsk/dmx` should not be adopted as dependencies for the ArtDMX send path; the encoder should be hand-rolled instead | Package Legitimacy Audit, Don't Hand-Roll | If this assumption is wrong (e.g. one of these libraries is in fact stable and well-tested at the exact version the plan would pin), the phase spends effort re-implementing something that already exists; the downside of being wrong is bounded (hand-rolled code is small and testable either way) |
| A5 | OLA should run on a separate Linux/macOS host or bridged-network VM, not directly on the Windows GOLC machine | Common Pitfalls Pitfall 6, Environment Availability | If wrong (e.g. some current OLA Windows build actually works well enough), the plan adds unnecessary verification-environment complexity; if the assumption is *not* accounted for at all, ARTN-06 verification stalls entirely trying to install OLA natively on Windows |
| A6 | `IanShelanskey/artnet-lib`'s "2026-08-03" tag date reflects a real recent release rather than a WebFetch rendering artifact | Package Legitimacy Audit | Low risk — regardless of the exact date, the package's zero-adoption signal (0 stars/forks) is the actual basis for excluding it, not the date itself |

**If this table is empty:** N/A — see entries above; several open items genuinely need a locked decision before planning proceeds confidently.

## Open Questions

1. **How should GOLC's flat universe number map to Art-Net's Net/Sub-Net/Universe Port-Address?**
   - What we know: The bit-packing rule itself (Net 7 bits / Sub-Net 4 bits / Universe 4 bits) is spec-clear (`[CITED: art-net.org.uk]`).
   - What's unclear: Which specific mapping GOLC should use from its own already-assigned flat `Instance.Universe` integers (Phase 2, "Complete") onto that 15-bit space.
   - Recommendation: Lock Assumption A1's Net=0 mapping as an explicit decision during planning/discussion (not left implicit in code), since it directly determines whether ARTN-02's "configure universe N" maps correctly onto the wire.

2. **Should the DMX-channel-order field be added to the canonical `fixture.FixtureDefinition`/`Mode` (Phase 2 schema, status Complete) or live in a Phase-4-scoped side model?**
   - What we know: The data does not exist anywhere today (Pitfall 1); some new construct is unavoidable.
   - What's unclear: Whether touching a "Complete" Phase 2 schema is acceptable process-wise for this project, versus keeping Phase 4 additive-only against its own new package.
   - Recommendation: Surface this explicitly to the user/planner before locking task structure — this is exactly the kind of schema-ownership question CONTEXT.md's discussion did not reach (it focused on interface/process/health/hardware-policy decisions, not on the fixture-model gap, which this research is the first artifact to surface in concrete terms).

3. **What is the fallback behavior when a fixture/mode has no DMX channel-order declared (e.g. an existing show authored before this field existed)?**
   - What we know: Silently guessing an order (e.g. falling back to `Capabilities` declaration order) risks producing incorrect-but-plausible-looking DMX output — a safety-relevant silent-approximation risk this project's own `POOL-07`/`FIXT-06` precedent explicitly avoids elsewhere ("never silently approximates").
   - What's unclear: Whether Phase 4 should hard-reject Art-Net configuration for any fixture lacking this data (forcing an explicit re-authoring step) or offer some other explicit, reviewable resolution path.
   - Recommendation: Treat this the same way Phase 2's `POOL-07` treats capability mismatches — explicit, reviewable, never silent — and make the exact mechanism a planning-time decision.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | All of Phase 4's implementation | ✓ (checked this session) | go1.26.5 windows/amd64 (matches `go.mod` pin) | — |
| Wireshark (with Art-Net/DMX dissector) | ARTN-06 packet-inspection verification, D-13/D-15 | ✗ (not found on this development machine this session) | — | Install on a verification machine before the ARTN-06 verification task; the dissector ships with mainline Wireshark, no separate plugin download needed `[CITED: github.com/wireshark/wireshark packet-artnet.c is upstreamed in core]` |
| Open Lighting Architecture (OLA) | ARTN-06 independent-simulator verification, D-13/D-14 | ✗ (not found on this development machine this session; also has no native first-class Windows build per Pitfall 6) | — | Run OLA on a separate Linux/macOS host or a bridged-adapter Linux VM reachable from the GOLC Windows machine's pinned interface — see Common Pitfalls Pitfall 6. No fallback exists for skipping this entirely; ARTN-06's simulator-verification success criterion has no substitute tool once D-13 is locked. |
| `github.com/microsoft/go-winio` | D-03/D-04 Windows named-pipe IPC | Not yet added to `go.mod` | Verify current tag via `go list -m -versions` before pinning | Loopback TCP (127.0.0.1) is a viable fallback transport if named pipes prove awkward to wire up in the plan's timeframe, at the cost of needing an application-layer auth check (see Standard Stack Alternatives) |
| Real Art-Net hardware | ARTN-06 hardware-compatibility claim, D-14/D-15 | ✗ (explicitly tracked as an open item per D-14 — no hardware currently owned) | — | None — this is an intentionally deferred, tracked gap (mirrors MIDI-HW-02's pattern), not something this phase's plan should attempt to work around by claiming compatibility without evidence |

**Missing dependencies with no fallback:**
- Real Art-Net hardware (D-14, tracked open item — not a blocker for this phase's plan, but the plan must not claim named hardware compatibility without it)

**Missing dependencies with fallback:**
- Wireshark, OLA — both need to be installed/provisioned on a verification machine before the ARTN-06 verification task runs; neither blocks implementation of ARTN-01 through ARTN-05
- `go-winio` — loopback TCP is a viable substitute transport if named pipes are deprioritized

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go's built-in `testing` package (`go test`), consistent with every prior phase in this repo (611 existing `_test.go` files found under `internal/`) |
| Config file | none — this repo's own `golc test`/`golc test --quick --scope <name>` wrapper (see `internal/command/test.go`, `config/commands.toml`) is the documented entrypoint, not a separate test-framework config file |
| Quick run command | `golc test --quick --scope artnet` (once an `artnet` test scope is declared, following this repo's existing `command.MustDeclareScope`/`TestScope{PascalName}` marker convention) |
| Full suite command | `golc test` (wraps `go test ./...`) |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| ARTN-01 | Interface enumeration/status returned correctly; pinned-interface loss detected | unit | `go test ./internal/artnet/... -run TestInterfaceManager` | ❌ Wave 0 |
| ARTN-02 | Universe/target config round-trips; ArtPoll discovery parses ArtPollReply correctly; discovered nodes never auto-added | unit | `go test ./internal/artnet/... -run TestUniverseConfig|TestDiscovery` | ❌ Wave 0 |
| ARTN-03 | ArtDMX packet byte-exact against golden vectors (OpCode, ProtVer, sequence wraparound, Port-Address packing, length) | unit | `go test ./internal/artnet/... -run TestEncodeArtDMX` | ❌ Wave 0 |
| ARTN-04 | Worker send loop never blocks the Engine's tick even when a target send hangs (simulated slow/unreachable target) | unit/integration | `go test ./internal/artnet/... -run TestWorkerNonBlocking` | ❌ Wave 0 |
| ARTN-05 | Health model correctly distinguishes stalled-engine vs. healthy cadence, reachable vs. unreachable target | unit | `go test ./internal/artnet/... -run TestHealth` | ❌ Wave 0 |
| ARTN-06 | Packet capture / OLA-received-value verification | manual-only (justification: requires Wireshark + OLA on a second host + optional real hardware; not automatable in CI) | N/A — human-verify checkpoint | ❌ Wave 0 (no automated substitute exists) |

### Sampling Rate
- **Per task commit:** `golc test --quick --scope artnet`
- **Per wave merge:** `golc test` (full suite)
- **Phase gate:** Full suite green before `/gsd-verify-work`; ARTN-06's manual verification (Wireshark capture + OLA-received-values) recorded as evidence before the phase is marked complete, per D-13/D-15's evidence bar

### Wave 0 Gaps
- [ ] `internal/artnet/packet_test.go` — golden-byte-vector tests for ArtDMX/ArtPoll/ArtPollReply encode/decode (REQ ARTN-03)
- [ ] `internal/artnet/channelmap_test.go` — AttributeSet → DMX byte mapping tests, including the not-yet-designed channel-order data model (REQ ARTN-03, depends on Open Question 2/3 being resolved first)
- [ ] `internal/artnet/interfacemgr_test.go` — interface enumeration and pinned-loss-detection tests (REQ ARTN-01)
- [ ] `internal/artnet/worker_test.go` — non-blocking-send-loop test using a deliberately slow/unreachable fake target (REQ ARTN-04)
- [ ] `internal/artnet/health_test.go` — frame/target health state-transition tests (REQ ARTN-05)
- [ ] Test scope declaration: `command.MustDeclareScope(command.ScopeRegistration{Scope: "artnet", ...})` plus a matching `TestScopeArtnet` marker, following this repo's existing convention (e.g. `internal/command/playback_engine_test.go`'s pattern)
- [ ] A manual ARTN-06 verification runbook (Wireshark capture steps + OLA-on-separate-host setup steps) — not a `_test.go` file, but should exist as a documented checklist before the phase gate

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | Partially | The IPC channel between short-lived CLI clients and the long-lived worker process is local-machine-only (named pipe or loopback TCP); named pipes can use Windows' native ACLs to restrict which local principals may connect. If loopback TCP is chosen instead, add a lightweight shared-secret/token check (e.g. a token file written with restrictive ACLs at process start) since loopback TCP alone permits any local process/user to connect `[CITED: comcomponent.com Windows IPC guidance]` |
| V3 Session Management | No | No user-facing session/login model in this phase — the IPC "session" is a single-machine process-to-process control channel, not a multi-user session |
| V4 Access Control | Yes | The named-pipe ACL (or loopback-TCP token) is the access-control boundary; ensure the daemon does not also bind a non-loopback address by default (matches this project's own `API-05` precedent for the future public API: "binds to loopback by default and requires explicit enablement... for remote access") — the Art-Net worker's IPC control-plane should follow the identical loopback-by-default discipline even though it's a different subsystem than Phase 7's API |
| V5 Input Validation | Yes | Every field parsed from an inbound ArtPollReply (node discovery, D-06) is untrusted network input from a device that could be spoofed or malformed — bounds-check every length/count field before use (mirrors this repo's own `fixture.Validate`/`decode.go` strict-decode discipline already established for YAML/OFL input) |
| V6 Cryptography | No | No cryptographic requirement identified for this phase; Art-Net itself is a plaintext UDP protocol by design (not a gap this phase can or should fix — out of scope per REQUIREMENTS.md's "Multiple lighting protocols at launch" boundary, and Art-Net's own spec has no encryption provision) |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Malformed/oversized ArtPollReply from an untrusted or spoofed network device | Denial of Service / Tampering | Strict length/bounds validation on every parsed field before use, matching this repo's existing `GOLC_FIXTURE_*`-style strict-decode convention; never trust a length field from the wire without a hard ceiling check |
| Local privilege escalation via an unauthenticated loopback TCP control-plane port | Elevation of Privilege | Prefer named pipes with Windows ACLs (default-deny non-owning-user connect) over bare loopback TCP; if loopback TCP is used, require an application-layer shared secret |
| Spoofed ArtPollReply used to inject a fake "discovered node" suggestion | Spoofing | D-06 already mitigates this at the design level (discovery only *suggests*, never auto-adds a live target) — this phase's implementation must not weaken that guarantee even for convenience (e.g. no "add all discovered nodes" bulk-apply shortcut without individual operator confirmation) |
| A malicious/misbehaving unicast target flooding the worker with reachability responses to induce excessive health-tracking memory growth | Denial of Service | Bound the health/target-tracking data structures to the explicitly configured target list (D-07/D-08) — never grow tracking state based on unsolicited inbound traffic from an address not in the configured target set |

## Sources

### Primary (HIGH confidence)
None — no Context7/official-docs-MCP tool call succeeded this session (no such MCP tool was available in this environment); all findings below are WebSearch/WebFetch-sourced and tagged accordingly at MEDIUM or LOW confidence per the classify-confidence seam's own tiering (`websearch` alone = LOW, cross-checked across ≥2 independent sources = MEDIUM).

### Secondary (MEDIUM confidence)
- art-net.org.uk (Art-Net 4 official specification site) — ArtDMX/ArtPoll/ArtPollReply field layout, Port-Address bit-packing, sequence-number semantics — cross-checked against Wikipedia's Art-Net summary and `jsimonetti/go-artnet`'s actual opcode constants (`packet/code/opcode.go`)
- `github.com/jsimonetti/go-artnet` (GitHub, fetched directly) — package structure, maintenance-status self-assessment ("unfinished... API highly unstable")
- `github.com/qmsk/dmx` (GitHub, fetched directly) — star count, feature set (unicast + ArtPoll discovery)
- `github.com/IanShelanskey/artnet-lib` (GitHub, fetched directly) — star/fork count, license, release date
- `github.com/microsoft/go-winio` (WebSearch-aggregated) — named-pipe `net.Listener`/`net.Conn` API shape, IOCP-based non-blocking design
- `golang.zx2c4.com/wireguard/windows/tunnel/winipcfg` (WebSearch-aggregated, cross-checked against `WireGuard/wireguard-windows` source on GitHub) — `ChangeCallback` interface-change notification API
- openlighting.org getting-started/downloads and "OLA on Windows via VMWare" tutorial page — OLA's Windows-hosting limitation
- github.com/wireshark/wireshark `packet-artnet.c` (WebSearch-aggregated) — confirms Art-Net dissector is upstreamed in Wireshark core

### Tertiary (LOW confidence)
- General DMX512/Art-Net refresh-rate community sources (uking-online.com, y-link.no, various forum threads) for the ~40Hz/25ms convention — cross-checked against this repo's own already-committed `tickHz = 40` constant and its doc-comment citation of "03-RESEARCH.md" (Phase 3's own research), which raises this to effectively MEDIUM confidence for this specific repo's context, even though the general web sources alone would be LOW
- Windows `SO_BINDTODEVICE`-unavailability and general IPC-choice guidance (comcomponent.com, various Medium posts, Stack Overflow-adjacent aggregations) — directionally consistent across multiple independent sources but not sourced from a single authoritative Microsoft doc page in this session

## Metadata

**Confidence breakdown:**
- Standard stack: MEDIUM — Go stdlib usage is HIGH-confidence (stable, unchanging API); the one new recommended dependency (`go-winio`) is well-established but its exact current version was not verified against the live module proxy this session
- Architecture: MEDIUM — the daemon/IPC pattern and non-blocking send-loop pattern are well-understood general Go patterns, but this repo has zero existing precedent for either, so the specific shape recommended here is a synthesis, not a verified in-repo pattern
- Pitfalls: MEDIUM-HIGH — several pitfalls (channel-order gap, universe-to-Port-Address mapping, OLA's Windows limitation) were discovered by directly reading this repository's own code and cross-referencing official/near-official sources, giving high confidence that these gaps are real and material, even though the exact fix chosen for each remains an open planning decision

**Research date:** 2026-07-22
**Valid until:** 30 days (protocol/spec content is stable; the package-legitimacy verdicts for `go-winio`/`go-artnet`/`qmsk/dmx`/`artnet-lib` should be re-checked at plan/implementation time regardless, since Go-modules tooling has no download-count signal and adoption/maintenance status can shift quickly for small packages)
