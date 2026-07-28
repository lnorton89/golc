---
phase: 4
slug: observable-art-net-live-output
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-07-22
---

# Phase 4 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` (stdlib), via the project's own `test` command route |
| **Config file** | none — `go test` driven by `_test.go` files; project-local scope markers follow the existing `TestScope{PascalName}` convention (`internal/command/test.go`) |
| **Quick run command** | `./golc.ps1 test --quick --scope artnet` (once an `artnet` scope is declared via `command.MustDeclareScope`) |
| **Full suite command** | `./golc.ps1 test` |
| **Estimated runtime** | ~30 seconds (consistent with Phase 1–3's project-wide suite) |

---

## Sampling Rate

- **After every task commit:** Run `./golc.ps1 test --quick --scope artnet`
- **After every plan wave:** Run `./golc.ps1 test`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| TBD | TBD | TBD | ARTN-01 | — | Interface enumeration/status returned correctly; pinned-interface loss detected (D-05) | unit | `go test ./internal/artnet/... -run TestInterfaceManager` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | ARTN-02 | — | Universe/target config round-trips (D-08 fan-out); ArtPoll discovery parses ArtPollReply; discovered nodes never auto-added (D-06) | unit | `go test ./internal/artnet/... -run TestUniverseConfig` / `TestDiscovery` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | ARTN-03 | T-04-01 / mitigate | ArtDMX packet byte-exact against golden vectors (OpCode, ProtVer, sequence wraparound, Port-Address packing, length) | unit | `go test ./internal/artnet/... -run TestEncodeArtDMX` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | ARTN-04 | T-04-02 / mitigate | Worker send loop never blocks the engine's tick even when a target send hangs (simulated slow/unreachable target) | unit/integration | `go test ./internal/artnet/... -run TestWorkerNonBlocking` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | ARTN-05 | — | Health model distinguishes stalled-engine vs. healthy cadence (D-09), reachable vs. unreachable target (D-10) | unit | `go test ./internal/artnet/... -run TestHealth` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | ARTN-06 | — | Packet capture / OLA-received-value verification against locked D-13 tooling | manual-only (see below) | N/A — human-verify checkpoint | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*
*Task IDs and Threat Refs are TBD pending PLAN.md creation — to be backfilled by gsd-plan-checker/gsd-verifier once plans exist, following Phase 3's own backfill precedent.*

---

## Wave 0 Requirements

- [ ] `internal/artnet/packet_test.go` — golden-byte-vector tests for ArtDMX/ArtPoll/ArtPollReply encode/decode (ARTN-03)
- [ ] `internal/artnet/channelmap_test.go` — `AttributeSet` → DMX byte mapping tests, including the not-yet-designed channel-order data model (ARTN-03; depends on RESEARCH.md Open Questions 2/3 being resolved during planning)
- [ ] `internal/artnet/interfacemgr_test.go` — interface enumeration and pinned-loss-detection tests (ARTN-01)
- [ ] `internal/artnet/worker_test.go` — non-blocking-send-loop test using a deliberately slow/unreachable fake target (ARTN-04)
- [ ] `internal/artnet/health_test.go` — frame/target health state-transition tests (ARTN-05)
- [ ] Test scope declaration: `command.MustDeclareScope(command.ScopeRegistration{Scope: "artnet", ...})` plus a matching `TestScopeArtnet` marker, following this repo's existing convention
- [ ] Framework install: none — Go `testing` used project-wide, no new framework needed

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Independent packet inspection and simulator verification | ARTN-06 | Requires Wireshark (Art-Net/DMX dissector) and Open Lighting Architecture (OLA, no native Windows build — needs a separate host/bridged VM per RESEARCH.md Pitfall 6) on a second machine/VM; not automatable in CI | Capture live output with Wireshark and confirm addressing/sequencing/payload-length/refresh against the Art-Net 4 spec; separately point configured unicast targets at a running OLA instance and confirm received per-universe values match `Engine.CurrentFrame()`. Record capture + received-values evidence per D-15's evidence bar. |
| Real-hardware compatibility claim | ARTN-06 | No real Art-Net hardware currently owned (D-14, open item, mirrors MIDI-HW-02) — cannot be exercised in this environment | Once hardware is available: recorded Wireshark packet capture showing correct output reaching the node, plus an observed/recorded correct physical response, per D-15. Do not claim named hardware compatibility without this evidence. |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
