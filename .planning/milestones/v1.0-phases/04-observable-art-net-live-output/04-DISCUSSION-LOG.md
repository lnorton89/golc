# Phase 4: Observable Art-Net Live Output - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-07-22
**Phase:** 4-Observable Art-Net Live Output
**Areas discussed:** Interface surface for this phase, Network interface & target workflow, Health & diagnostics surface, Hardware/simulator compatibility policy

---

## Interface surface for this phase

| Option | Description | Selected |
|--------|-------------|----------|
| CLI only, for now | Configure via `golc artnet` subcommands, matching Phase 1-3's headless precedent; Phase 6 wraps in Wails later | ✓ |
| Minimal standalone live view | A small always-on window/tray icon ahead of Phase 6 | |
| You decide | Claude picks based on precedent fit | |

**User's choice:** CLI only, for now

| Option | Description | Selected |
|--------|-------------|----------|
| Continuous watch + one-shot both | `--watch` continuously-refreshing table plus one-shot snapshot, human table default + `--json` flag | ✓ |
| One-shot snapshot only | Single status command, re-run to refresh | |
| You decide | | |

**User's choice:** Continuous watch + one-shot both

| Option | Description | Selected |
|--------|-------------|----------|
| One long-lived process, CLI attaches | GOLC runs as one long-lived process hosting engine + Art-Net worker; CLI commands are clients via local IPC | ✓ |
| CLI owns the process per invocation | Running the CLI starts/owns output for that process's lifetime | |
| You decide | | |

**User's choice:** One long-lived process, CLI attaches

| Option | Description | Selected |
|--------|-------------|----------|
| Standalone-capable | GOLC can run headless via CLI alone, no Wails window ever required; Phase 6 app is just another client | ✓ |
| Wails-launched only | The long-lived process only exists once Wails is started | |
| You decide | | |

**User's choice:** Standalone-capable
**Notes:** This resolves ROADMAP.md's "UI hint: yes" flag — the operator surface is CLI, and the "independently of the desktop UI" wording in the phase goal is read literally: no Wails window is ever required to run or verify Phase 4.

---

## Network interface & target workflow

| Option | Description | Selected |
|--------|-------------|----------|
| Pin + warn, never auto-switch | Explicit pin; on drop, stop sending + surface clear error, wait for operator reselect | ✓ |
| Auto-fallback to next available interface | Automatically retries on another active interface | |
| You decide | | |

**User's choice:** Pin + warn, never auto-switch

| Option | Description | Selected |
|--------|-------------|----------|
| Discovery suggests, never auto-adds | Discovered nodes are suggestions; adding a target is always an explicit operator action | ✓ |
| Discovery can auto-populate targets | Discovered nodes automatically added as active targets | |
| You decide | | |

**User's choice:** Discovery suggests, never auto-adds

| Option | Description | Selected |
|--------|-------------|----------|
| Strictly unicast | Every target is a specific static unicast address, no broadcast mode | ✓ |
| Support broadcast as an option | Allow broadcasting to a universe's standard broadcast address | |
| You decide | | |

**User's choice:** Strictly unicast

| Option | Description | Selected |
|--------|-------------|----------|
| Universe can fan out to multiple targets | One universe's frames sent to several configured unicast targets simultaneously | ✓ |
| Strictly one target per universe | Each universe maps to exactly one unicast target | |
| You decide | | |

**User's choice:** Universe can fan out to multiple targets

---

## Health & diagnostics surface

| Option | Description | Selected |
|--------|-------------|----------|
| Cadence + staleness | Publish cadence plus staleness of last frame read from `Engine.CurrentFrame()` | ✓ |
| Cadence only | Only whether the send loop is on schedule | |
| You decide | | |

**User's choice:** Cadence + staleness

| Option | Description | Selected |
|--------|-------------|----------|
| Send success + reachability | Last successful send, consecutive error count, plus ArtPollReply-based reachability when available | ✓ |
| Send success only | Track only OS-level send errors | |
| You decide | | |

**User's choice:** Send success + reachability

| Option | Description | Selected |
|--------|-------------|----------|
| Persistent status + structured log | Persistent per-universe/target indicator in watch view plus structured log line per error/state-change | ✓ |
| Transient console messages only | Errors print once and scroll past | |
| You decide | | |

**User's choice:** Persistent status + structured log

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, per-target enable/disable now | A CLI command to enable/disable output to a specific universe/target independently | ✓ |
| No, defer to Phase 6 | Phase 4 only reports enablement status; disabling waits for Phase 6 | |
| You decide | | |

**User's choice:** Yes, per-target enable/disable now

---

## Hardware/simulator compatibility policy

| Option | Description | Selected |
|--------|-------------|----------|
| Name it now | Select a specific simulator + real-hardware set now, mirroring MIDI-HW-01 | ✓ |
| Defer to planning/execution | Leave the choice open for a later pass | |
| You decide | | |

**User's choice:** Name it now

| Option | Description | Selected |
|--------|-------------|----------|
| I'll name specific hardware/tools | Type the exact node(s)/tools owned | |
| No hardware yet — pick a well-known simulator only | No real hardware lined up; lock in only a simulator/monitor for now | ✓ |

**User's choice:** No hardware yet — pick a well-known simulator only

| Option | Description | Selected |
|--------|-------------|----------|
| Use Wireshark + OLA | Wireshark (Art-Net/DMX dissector) for packet inspection, OLA as independent receiving simulator/node | ✓ |
| Let me specify different tools | User has other specific tools in mind | |

**User's choice:** Use Wireshark + OLA

| Option | Description | Selected |
|--------|-------------|----------|
| Recorded packet capture + observed output | Wireshark capture + recorded/photographed correct physical response required before a named hardware claim | ✓ |
| Simulator-only acceptance is sufficient | Simulator verification alone counts as release evidence | |
| You decide | | |

**User's choice:** Recorded packet capture + observed output
**Notes:** No real Art-Net hardware currently owned — tracked as an open item, same pattern as MIDI-HW-02 (selection ≠ support).

---

## Claude's Discretion

None — every gray area discussed converged on the recommended option; no "you decide" selections were made in this session.

## Deferred Ideas

None — discussion stayed within phase scope.
