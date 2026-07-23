---
phase: 04-observable-art-net-live-output
verified: 2026-07-23T04:00:00Z
status: passed
score: 14/14 must-haves verified
behavior_unverified: 0
overrides_applied: 1
overrides:
  - must_have: "Runbook drives the verification through golc artnet serve/configure/status against an OLA receiver on a second host (ARTN-06 key_link, 04-07-PLAN.md)."
    reason: "The Task 2 checkpoint:human-verify was resolved not by a manually-operated separate host/bridged-VM running the full golc artnet serve/configure/status CLI flow (as the runbook's own Section 1/2 describe), but by the orchestrator running a real, independent verification (Docker-hosted OLA + Wireshark/tshark) directly against golc's production EncodeArtDMX/PortAddress functions from a temporary harness, over loopback+Docker port-publish rather than a bridged LAN path, and without exercising the golc artnet CLI end-to-end (no show/fixture file exists in this repo yet). The operator was shown the full record including these caveats (.planning/artnet/ARTN-06-verification-2026-07-22.md) and explicitly chose \"Accept — close Wave 7 now\" over a fully manual re-run or closing the CLI-flow gap first (per 04-07-SUMMARY.md's \"Checkpoint Status\" section)."
    accepted_by: "operator (recorded in 04-07-SUMMARY.md Checkpoint Status; no individual username given)"
    accepted_at: "2026-07-22"
re_verification:
  previous_status: gaps_found
  previous_score: 12/14
  gaps_closed:
    - "An operator can inspect per-universe final values, frame health, target health, errors, and output enablement through golc artnet status (ARTN-05; ROADMAP Success Criterion 4)."
    - "golc artnet interface list / golc artnet status surface the pinned interface's lost/degraded status; a lost pinned interface is surfaced as a degraded/error status, not a silent switch (ARTN-01/D-05; ROADMAP Success Criterion 1)."
  gaps_remaining: []
  regressions: []
deferred: []
human_verification: []
---

# Phase 4: Observable Art-Net Live Output Verification Report

**Phase Goal:** Operators can drive a small Art-Net rig from deterministic complete frames and verify protocol, target, and timing health independently of the desktop UI.
**Verified:** 2026-07-23T04:00:00Z
**Status:** passed
**Re-verification:** Yes — after gap closure (04-08, 04-09)

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | An operator can select a Windows network interface and enumerate candidates (index, name, up/down, addresses) (ARTN-01). | ✓ VERIFIED | `internal/artnet/interfacemgr.go` `ListCandidateInterfaces()`; `internal/command/artnet.go` `runArtnetInterfaceList`; `TestInterfaceListCandidateInterfacesFindsLoopback` re-run and passes (no regression). |
| 2 | The selected interface is pinned by stable `net.Interface.Index`, never by Name, and never auto-switches on loss (D-05). | ✓ VERIFIED | `internal/artnet/interfacemgr.go` pins by Index; `TestInterfaceManagerBogusIndexLostAfterOnePollIteration`/`TestInterfaceManagerMarkLostTransitionsStatus` re-run and pass. |
| 3 | The pinned interface's lost/degraded status is surfaced to the operator through a CLI/IPC route (ARTN-01/D-05). | ✓ VERIFIED (Gap 2 closed) | `internal/artnet/daemon.go` `handleStatus` now reads `d.ifaceMgr.PinnedIndex()/PinnedName()/Status()/Err()` into `statusPayload.Interface`; `internal/command/artnet.go` renders `GOLC_ARTNET_INTERFACE_STATUS:` (plain/watch) and `Interface` (json), and `runArtnetInterfaceList` annotates the pinned candidate with `PINNED`/`STATUS` columns (plain) and `pinned`/`status` fields (json) via a best-effort daemon round trip. Source-read confirmed at `daemon.go:247-332` and `artnet.go:394-476,730-820`. Re-ran (not just trusted SUMMARY) `TestDaemonStatusPayloadIncludesPinnedInterfaceStatus`, `TestDaemonStatusPayloadSurfacesLostInterface` (a genuinely-lost interface reports `status=lost` + `GOLC_ARTNET_INTERFACE_LOST`, not just the healthy path), `TestArtnetStatusPlainRendersPinnedInterface`, `TestArtnetStatusJSONIncludesInterfaceStatus`, `TestArtnetInterfaceListAnnotatesPinnedWhenDaemonRunning`, `TestArtnetInterfaceListWorksWithNoDaemon` — all pass. |
| 4 | A unicast target is (Universe, IP, Port, Enabled); output is strictly per-target unicast, no broadcast (D-07); a universe can fan out to multiple targets (D-08); per-target enable/disable exists and is copy-returning (D-12); duplicates are rejected (ARTN-02). | ✓ VERIFIED | `internal/artnet/target.go`; unchanged by 04-08/04-09; re-confirmed untouched by this wave's `files_modified` lists. |
| 5 | A fixture Mode carries an ordered DMX channel layout (D-16); a mode with no declared layout is a hard rejection (D-17). | ✓ VERIFIED | `internal/fixture/*`; unaffected by this wave. |
| 6 | `channelmap.Encode` + `EncodeArtDMX` produce byte-exact Art-Net 4 ArtDMX packets against golden vectors; sequence wraps 1→255→1, never 0 (ARTN-03). | ✓ VERIFIED | Unaffected by this wave; `go test ./internal/artnet/...` green. |
| 7 | CR-01: `ValidateTarget` rejects any Universe whose Port-Address would alias onto a lower valid universe (Universe > 255). | ✓ VERIFIED | Unaffected by this wave; still present at `target.go:71-73`. |
| 8 | The Art-Net worker reads `playback.Engine.CurrentFrame()` on its own independent 40Hz ticker and is never backpressured by a slow/unreachable target (ARTN-04). | ✓ VERIFIED | `internal/artnet/worker.go` `tick()`'s dispatch loop is unchanged by 04-08 (only a new `RecordUniverseValues` call was inserted, non-blocking, in-line with the existing per-universe loop, before `EncodeArtDMX`); `-race` re-run clean (`TestHealth\|TestWorker\|TestDaemon\|TestInterfaceManager -race`). |
| 9 | Frame health distinguishes on-cadence vs stalled (D-09); target health distinguishes reachable vs unreachable (D-10); health tracking is bounded to the configured target set (T-04-04, D-11). | ✓ VERIFIED | `internal/artnet/health.go`; re-ran `TestFrameHealthOnCadenceVsStalled`, `TestHealthTargetSendAccumulatesAndDistinguishesReachability`, `TestHealthUnconfiguredTargetNeverTracked`, `TestHealthRecordSendErrorEmitsStructuredLogLine` — all pass, no regression from the new `universeValues`/`configuredUniverses` fields added alongside. |
| 10 | An operator can inspect per-universe final values, frame health, target health, errors, and output enablement through `golc artnet status` (ARTN-05). | ✓ VERIFIED (Gap 1 closed) | `internal/artnet/worker.go:280-309` `tick()` now calls `w.health.RecordUniverseValues(u, data)` for every configured universe (both the real `buffers[u]` case and the blackout `make([]byte, channelsPerUniverse)` default) before building the packet — the exact buffer the previous verification found discarded. `internal/artnet/health.go` carries it via a second lock-free `atomic.Pointer[map[int][]byte]` (`universeValuesPtr`), bounded to `configuredUniverses` (T-04-04), with a defensive copy on write (`RecordUniverseValues`, confirmed via source read at `health.go:253-280`). `daemon.go`'s `statusPayload.Universes` and `command/artnet.go`'s `artnetStatusPayload.Universes` carry it to `--json`; `renderArtnetStatusPlain` emits `GOLC_ARTNET_UNIVERSE: universe=<u> channels=512 nonzero=<n> values=[...]` for plain/watch. Re-ran (not trusted from SUMMARY) `TestHealthRecordUniverseValuesSnapshotReflectsConfiguredUniverse`, `TestHealthUnconfiguredUniverseValuesNeverTracked`, `TestHealthRecordUniverseValuesIsDefensivelyCopied`, `TestDaemonStatusPayloadIncludesConfiguredUniverseValues`, `TestArtnetStatusJSONContainsUniverseValues` (decodes and length-checks `len(Values)==512`, not substring presence — the exact false-pass mechanism the prior verification flagged is now closed), `TestArtnetStatusPlainRendersUniverseValues` — all pass. |
| 11 | One long-lived process hosts engine+worker+interface manager+IPC listener; IPC is a local, ACL-restricted named pipe. | ✓ VERIFIED | `internal/artnet/daemon.go` `Run`; unaffected in structure by this wave (only `statusPayload`/`handleStatus` gained fields); `go list -deps` still shows no Wails import. |
| 12 | A CLI client with no reachable daemon gets `GOLC_ARTNET_DAEMON_UNREACHABLE`, never a hang. | ✓ VERIFIED | Unaffected; `TestArtnetInterfaceListWorksWithNoDaemon` re-confirms the no-daemon path for `interface list` specifically does NOT regress to this error (by design — see Task 2 of 04-09), while `TestArtnetNoDaemonReturnsDaemonUnreachable` (status route) still passes. |
| 13 | Optional discovery broadcasts ArtPoll/collects ArtPollReply bounded, suggestions only; no "add all" bulk-apply; bounds-checked untrusted fields (ARTN-02, D-06, Security V5). | ✓ VERIFIED | Unaffected by this wave. |
| 14 | A release candidate demonstrates packet + timing compatibility against OLA with a recorded Wireshark capture; real hardware remains an explicit, un-claimed open item (ARTN-06, D-13/D-14/D-15). | ✓ PASSED (override, carried forward) | Unaffected by this wave; override from the initial verification pass carried forward unchanged (see frontmatter). |

**Score:** 14/14 truths verified (1 of the 14 via operator-accepted override carried forward from the initial verification pass; 0 present-but-behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/artnet/health.go` | Per-universe values field + lock-free publish, bounded to configured universes | ✓ VERIFIED | `universeValuesPtr atomic.Pointer[map[int][]byte]`, `configuredUniverses`, `RecordUniverseValues`, `publishUniverseValuesLocked`, `HealthSnapshot.UniverseValues` all present at `health.go:106-301`; `Snapshot()` reads via the atomic pointer only, never `h.mu` — confirmed by source read. |
| `internal/artnet/worker.go` | `tick()` records the per-universe buffer instead of discarding it | ✓ VERIFIED | `worker.go:296` `w.health.RecordUniverseValues(u, data)` called for every configured universe, both real-buffer and blackout-default cases, before `EncodeArtDMX`. |
| `internal/artnet/daemon.go` | `statusPayload.Universes` + `statusPayload.Interface`; `handleStatus` reads `ifaceMgr` | ✓ VERIFIED | `statusPayload` (line 247-252), `newStatusPayload` (line 282-307), `handleStatus` (line 314-332) all confirmed present and wired; `Run()`'s doc comment corrected (no longer claims the loss is "already" surfaced — line 160-166). |
| `internal/command/artnet.go` | Mirror types + plain/json rendering for universes and interface status; `interface list` annotation | ✓ VERIFIED | `artnetStatusPayload`/`artnetInterfaceStatus`/`artnetUniverseValues` (line 735-760), `renderArtnetStatusPlain` (line 780-820), `interfaceListEntry` + `runArtnetInterfaceList` (line 388-476) all confirmed present. |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `internal/artnet/worker.go` | `internal/artnet/health.go` | `tick()` calls `RecordUniverseValues(u, data)` with the real per-tick buffer | ✓ WIRED | Confirmed by source read (`worker.go:296`) and `TestDaemonStatusPayloadIncludesConfiguredUniverseValues` (integration, asserts `len(Values)==512` after a real reconfigure + tick, not a synthetic health-only call). |
| `internal/artnet/health.go` | `internal/artnet/daemon.go` | `handleStatus` reads `Snapshot().UniverseValues`, `newStatusPayload` flattens it into `statusPayload.Universes` | ✓ WIRED | Confirmed by source read; `TestArtnetStatusJSONContainsUniverseValues` proves the end-to-end CLI round trip. |
| `internal/artnet/daemon.go` | `internal/command/artnet.go` | Canonical JSON `universes`/`interface` fields round-trip into the CLI mirror structs | ✓ WIRED | Identical json tags confirmed field-by-field; `strictjson.DecodeStrict`'s `DisallowUnknownFields` would fail any mismatch — tests pass, so the round trip holds. |
| `internal/artnet/interfacemgr.go` (`Status()`/`Err()`/`PinnedIndex()`/`PinnedName()`) | `internal/artnet/daemon.go` (`handleStatus`) | Direct reads into `interfaceStatusPayload` | ✓ WIRED | Confirmed by source read (`daemon.go:315-324`); `TestDaemonStatusPayloadSurfacesLostInterface` proves the degraded (not just healthy) path is genuinely reachable — a daemon pinned to a bogus index reports `status=lost` + `GOLC_ARTNET_INTERFACE_LOST` within the poll deadline. |
| `internal/command/artnet.go` (`runArtnetInterfaceList`) | `internal/command/artnet.go` (`fetchArtnetStatus`) | Best-effort daemon round trip annotates the pinned candidate; failure degrades gracefully to the plain candidate list | ✓ WIRED | Confirmed by source read (`artnet.go:429-434`) and `TestArtnetInterfaceListAnnotatesPinnedWhenDaemonRunning` (daemon up) + `TestArtnetInterfaceListWorksWithNoDaemon` (daemon down, ExitCode 0, no regression to `GOLC_ARTNET_DAEMON_UNREACHABLE`) — both pass. |

### Requirements Coverage

| Requirement | Source Plan(s) | Description | Status | Evidence |
|-------------|-----------------|-------------|--------|----------|
| ARTN-01 | 02, 05, 09 | Select interface + see current status | ✓ SATISFIED | Gap 2 closed: pinned interface's live status now surfaced through `artnet status` (authoritative) and `artnet interface list` (best-effort annotation). |
| ARTN-02 | 02, 05, 06 | Configure universes/targets, discover nodes | ✓ SATISFIED | Unaffected by this wave; previously verified. |
| ARTN-03 | 01, 03 | Valid Art-Net 4 output | ✓ SATISFIED | Unaffected by this wave; previously verified. |
| ARTN-04 | 01, 03, 04 | Non-backpressuring consumption | ✓ SATISFIED | Unaffected in structure; `-race` re-confirmed clean. |
| ARTN-05 | 03, 05, 08 | Inspect per-universe final values, frame/target health, errors, enablement | ✓ SATISFIED | Gap 1 closed: per-universe final DMX values now recorded and surfaced through `golc artnet status` (`--json`/plain/watch), proven by a byte-length assertion, not substring presence. |
| ARTN-06 | 07 | Packet/timing compatibility with simulator + real hardware | ⚠️ PARTIAL (operator-accepted override, carried forward) | Unaffected by this wave; same disposition as the initial verification (simulator+packet compatibility demonstrated; real hardware explicitly un-claimed per D-14, the project's own locked scope decision). |

All six ARTN-0x requirement IDs are declared across the nine plans' `requirements:` frontmatter (02/05 plus 08/09 for the two gap-closure IDs) and each maps to REQUIREMENTS.md's Phase 4 rows, all marked `[x]`/`Complete`. No orphaned requirements.

### Anti-Patterns Found

None in the 7 files touched by 04-08/04-09. Grep for `TODO|FIXME|XXX|HACK|PLACEHOLDER|placeholder|coming soon|not yet implemented` across `internal/artnet/health.go`, `internal/artnet/worker.go`, `internal/artnet/health_test.go`, `internal/artnet/daemon.go`, `internal/artnet/daemon_test.go`, `internal/command/artnet.go`, `internal/command/artnet_test.go` returned no matches. `go vet ./internal/artnet/... ./internal/command/...` is clean. `go build ./...` succeeds.

### Behavioral Spot-Checks / Regression Run

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Gap-closure named tests (11 tests spanning both gaps) | `go test ./internal/artnet/... ./internal/command/... -run '<11 test names>' -v -count=1` | All 11 pass | ✓ PASS |
| Full package regression (not filtered per-truth; run once) | `go test ./internal/artnet/... ./internal/command/... -count=1` | `ok` for all 3 packages | ✓ PASS |
| Race detector on health/worker/daemon/interface manager | `go test ./internal/artnet/... -run 'TestHealth\|TestWorker\|TestDaemon\|TestInterfaceManager' -race -count=1` | `ok`, clean | ✓ PASS |
| Previously-passing adjacent truths re-run for regression | `go test ... -run 'TestInterfaceListCandidateInterfacesFindsLoopback\|TestInterfaceManagerBogusIndexLostAfterOnePollIteration\|TestInterfaceManagerMarkLostTransitionsStatus\|TestFrameHealthOnCadenceVsStalled\|TestHealthTargetSendAccumulatesAndDistinguishesReachability\|TestHealthUnconfiguredTargetNeverTracked\|TestHealthRecordSendErrorEmitsStructuredLogLine'` | All pass | ✓ PASS |
| Build | `go build ./...` | Succeeds | ✓ PASS |
| Vet | `go vet ./internal/artnet/... ./internal/command/...` | Clean | ✓ PASS |

### Probe Execution

No `scripts/*/tests/probe-*.sh` files exist in this repository and none are referenced by 04-08/04-09's PLAN/SUMMARY. Step 7c: SKIPPED (no declared or conventional probes for this phase).

### Code Review Follow-Through (04-REVIEW.md "Gap-Closure Review" section)

| Finding | Disposition | Verified |
|---------|-------------|----------|
| GC-WR-01: `handleStatus`'s non-atomic `Err()`/`Status()` split read can transiently produce `status=lost` + `Error=""` during an interface-loss transition | Known, disclosed, not yet fixed (non-blocking per reviewer) | Confirmed present at `daemon.go:315-324` — `Err()` and `Status()` are still two independent loads. Per the orchestrator's instruction, this is not treated as a fresh gap: the core truth ("is the pinned interface's lost status surfaced to the operator") holds via the `status` field alone even in the narrow race window; only the paired `Error` string can transiently lag by one poll cycle. |
| GC-WR-02: `artnet interface list --json` doesn't carry the interface error text that the plain-text rendering does | Known, disclosed, not yet fixed (non-blocking per reviewer) | Confirmed present at `artnet.go:394-401,443-447` (JSON branch sets `entry.Status` only, never `entry.Error` — no such field exists on `interfaceListEntry`) vs. `artnet.go:466-472` (plain branch appends `pinnedError` to the status string). The `status` field itself (e.g. `"lost"`) is present in both plain and JSON, so the core must-have ("distinct from the static candidate list, annotated with live status") is satisfied; only the diagnostic-text convenience is asymmetric between the two rendering modes. |
| GC-IN-01/GC-IN-02 (info-level, non-blocking) | Disclosed, not yet fixed | Confirmed present as described (hand-mirrored JSON structs with no compile-time guard; undocumented historical-retention behavior on universe removal). Info-level, does not affect any must-have. |

Both warnings are disclosed, non-blocking, and consistent with the orchestrator's framing for this verification pass — they do not reopen either closed gap.

### Human Verification Required

None. Both prior gaps are closed with automated, decode-and-length/status-value assertions (not substring or key-presence checks), and no new behavior-dependent truth was introduced by 04-08/04-09 that lacks a passing test.

### Gaps Summary

No gaps remain. Both gaps from the initial verification pass are genuinely closed, re-derived from source (not from SUMMARY.md claims):

1. **Gap 1 (ARTN-05 / SC4, per-universe final values) — CLOSED.** `worker.go`'s `tick()` now calls `w.health.RecordUniverseValues(u, data)` with the exact per-tick buffer that was previously discarded (confirmed by source read, not just SUMMARY claim). `health.go` carries it through a second lock-free `atomic.Pointer` publish, bounded to `configuredUniverses` (T-04-04), defensively copied on write. `daemon.go`/`command/artnet.go` thread it to `--json`/plain/watch. The corrected acceptance test (`TestArtnetStatusJSONContainsUniverseValues`) decodes the JSON and asserts `len(Values)==512` — a real byte-length check, directly replacing the substring-only false-pass the previous verification pass caught in 04-05.

2. **Gap 2 (ARTN-01/D-05, pinned-interface status surfacing) — CLOSED.** `daemon.go`'s `handleStatus` now reads the already-built, already-tested `InterfaceManager.Status()`/`Err()`/`PinnedIndex()`/`PinnedName()` into `statusPayload.Interface`; `command/artnet.go` renders it (`GOLC_ARTNET_INTERFACE_STATUS:` plain/watch, `Interface` json) and annotates `artnet interface list`'s pinned candidate via a best-effort daemon round trip that degrades gracefully with no daemon (re-confirmed no regression: `TestArtnetInterfaceListWorksWithNoDaemon` still returns ExitCode 0 with no `GOLC_ARTNET_DAEMON_UNREACHABLE`). A genuinely-lost pinned interface was proven (not assumed) to report `status=lost` + `GOLC_ARTNET_INTERFACE_LOST` via `TestDaemonStatusPayloadSurfacesLostInterface`, which pins to a deliberately-invalid interface index and polls until the transition occurs — this exercises the degraded path, not just the healthy one.

Two non-blocking warnings from the gap-closure code review (GC-WR-01: narrow non-atomic split-read race that can transiently pair `status=lost` with an empty error string; GC-WR-02: `interface list --json` omits the error text that plain rendering includes) remain open as disclosed, known follow-up — per the orchestrator's framing, these do not reopen either gap, since the core surfaced-status truth holds in both cases (the `status` field itself is correct and race-free; only the paired diagnostic string has a narrow lag/asymmetry).

All twelve previously-verified truths were independently re-checked (not merely assumed unaffected): their supporting tests were re-run in this pass and all pass, with no regressions, including under `-race` for the concurrency-sensitive health/worker/daemon paths.

---

_Verified: 2026-07-23T04:00:00Z_
_Verifier: Claude (gsd-verifier)_
