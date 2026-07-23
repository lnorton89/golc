---
phase: 04-observable-art-net-live-output
reviewed: 2026-07-22T00:00:00Z
depth: standard
files_reviewed: 33
files_reviewed_list:
  - docs/artnet/ARTN-06-verification-runbook.md
  - internal/artnet/channelmap.go
  - internal/artnet/channelmap_test.go
  - internal/artnet/daemon.go
  - internal/artnet/daemon_test.go
  - internal/artnet/discovery.go
  - internal/artnet/discovery_test.go
  - internal/artnet/health.go
  - internal/artnet/health_test.go
  - internal/artnet/interfacemgr.go
  - internal/artnet/interfacemgr_test.go
  - internal/artnet/ipc/client.go
  - internal/artnet/ipc/ipc_test.go
  - internal/artnet/ipc/server.go
  - internal/artnet/ipc/types.go
  - internal/artnet/packet.go
  - internal/artnet/packet_test.go
  - internal/artnet/target.go
  - internal/artnet/target_test.go
  - internal/artnet/worker.go
  - internal/artnet/worker_test.go
  - internal/command/artnet.go
  - internal/command/artnet_test.go
  - internal/command/fixture_test.go
  - internal/command/substitution_test.go
  - internal/fixture/decode.go
  - internal/fixture/decode_test.go
  - internal/fixture/identity_test.go
  - internal/fixture/model.go
  - internal/fixture/ofl/model.go
  - internal/fixture/ofl/normalize.go
  - internal/fixture/ofl/normalize_test.go
  - internal/fixture/provenance_test.go
  - internal/substitution/plan_test.go
  - schemas/fixture.schema.json
findings:
  critical: 1
  warning: 4
  info: 1
  total: 6
status: issues_found
---

# Phase 04: Code Review Report

**Reviewed:** 2026-07-22T00:00:00Z
**Depth:** standard
**Files Reviewed:** 33
**Status:** issues_found

## Summary

Reviewed the Art-Net live-output subsystem (packet encode/decode, unicast
target model, health tracking, interface pinning, discovery, the ticker-
driven worker, the local named-pipe IPC daemon bridge, and the CLI routes),
plus the fixture-model files touched to add D-16's ordered DMX channel
layout (`Mode.Channels`) and the OFL importer/schema that project onto it.

The implementation is unusually well-documented and its test suite is
extensive and mostly proves the properties its own doc comments claim
(non-blocking fan-out, bounded discovery, DoS-bounded health tracking,
owner-restricted IPC). However, one gap in `ValidateTarget` lets an
operator configure a `Universe` value that the packet layer's own locked
Port-Address mapping cannot represent without aliasing onto a different,
already-valid universe — a real on-the-wire misrouting risk with no
validation, test, or diagnostic anywhere in the pipeline that catches it.
Several other findings are documentation/implementation mismatches and
robustness gaps in the daemon's interface-IP caching and worker shutdown
sequencing.

## Critical Issues

### CR-01: `ValidateTarget` has no upper bound on `Universe`, so a configured universe above 255 silently aliases onto a different universe's Art-Net Port-Address

**File:** `internal/artnet/target.go:60-75` (see also `internal/artnet/packet.go:70-75`)

**Issue:** `PortAddress` implements the phase's locked Assumption A1 mapping
— `Net=0` fixed, `Sub-Net=(universe>>4)&0xF`, `Universe=universe&0xF` — which
only has 8 usable bits (`Sub-Net` 4 bits + `Universe` 4 bits), i.e. it can
represent universes `1..255` uniquely. Any configured `Universe` value
`>= 256` wraps via the `&0xF`/`&0xF` masks and collides with a lower,
already-valid universe's Port-Address. For example:

```go
PortAddress(1)   // -> 0x0001  (Sub-Net=0, Universe=1)
PortAddress(257) // -> 0x0001  (Sub-Net=0, Universe=1) -- identical!
```

`ValidateTarget` only rejects `Universe < 1`:

```go
func ValidateTarget(t Target) error {
	if t.Universe < 1 {
		return fmt.Errorf("GOLC_ARTNET_TARGET_INVALID: universe %d must be at least 1", t.Universe)
	}
	...
}
```

There is no upper-bound check anywhere in the reachable path: not in
`ValidateTarget`, not in `ValidateUniqueTargets` (which keys uniqueness by
`(Universe, IP, Port)`, not by the resulting Port-Address), not in
`channelmap.Encode`, and not in `worker.go`'s tick loop. `artnet configure
--universe 257 --ip <node>` (via `internal/command/artnet.go`'s
`runArtnetConfigure`, which calls this exact `ValidateTarget`) is accepted
and forwarded to the daemon without complaint, and the daemon accepts and
runs it too (`daemon.go`'s `handleConfigure` calls the identical
`ValidateTarget`).

The project's own `04-RESEARCH.md` explicitly names this exact failure mode
as the reason Assumption A1 had to be locked in the first place ("If
wrong, ARTN-02's universe configuration silently addresses the wrong
Port-Address, producing packets a real/simulated node either ignores or
misroutes"), but no test or validation actually enforces the bound that
makes the locked mapping safe. On a real or simulated multi-port Art-Net
node that routes internally by Port-Address (not just by destination IP),
configuring universe 257 for one fixture rig would deliver its DMX data to
the same on-the-wire address as universe 1's rig, potentially driving the
wrong physical fixture.

**Fix:**
```go
// artNetMaxUniverse is the highest universe value PortAddress's locked
// Net=0-fixed mapping can represent without aliasing onto a lower
// universe's Port-Address (Sub-Net 4 bits + Universe 4 bits = 8 usable
// bits => 1..255).
const artNetMaxUniverse = 255

func ValidateTarget(t Target) error {
	if t.Universe < 1 {
		return fmt.Errorf("GOLC_ARTNET_TARGET_INVALID: universe %d must be at least 1", t.Universe)
	}
	if t.Universe > artNetMaxUniverse {
		return fmt.Errorf("GOLC_ARTNET_TARGET_INVALID: universe %d exceeds the maximum representable Port-Address universe %d (Net=0-fixed mapping)", t.Universe, artNetMaxUniverse)
	}
	...
}
```
Add a regression test asserting `ValidateTarget` rejects `Universe: 256`
and `Universe: 257` (the exact aliasing case above), and a `PortAddress`
test proving `PortAddress(1) != PortAddress(257)` is never silently
expected to hold once the bound is in place.

## Warnings

### WR-01: `ValidateTarget` only rejects the exact limited-broadcast address, not multicast or subnet-directed broadcast addresses

**File:** `internal/artnet/target.go:60-75`

**Issue:** The doc comment and CONTEXT D-07 state output is "strictly
per-target unicast -- there is no broadcast target construct anywhere in
this package," and `ValidateTarget` enforces this only by rejecting
`net.IPv4bcast` (`255.255.255.255`) exactly:

```go
if t.IP.Equal(net.IPv4bcast) {
	return fmt.Errorf("GOLC_ARTNET_TARGET_INVALID: %v is the IPv4 broadcast address; targets are unicast-only (D-07)", t.IP)
}
```

A subnet-directed broadcast address (e.g. `192.168.1.255` on a `/24`) or a
multicast address (`224.0.0.0/4`) both pass this check unchanged, since
neither equals `net.IPv4bcast` and neither is "unspecified." Both violate
the documented unicast-only invariant just as much as the limited
broadcast address does, and depending on router/OS configuration a
directed broadcast can still fan out to every host on that subnet.

**Fix:** Reject `t.IP.IsMulticast()` and, where feasible, cross-check
against any interface-known subnet mask to catch directed broadcast
addresses (or at minimum document that only the limited broadcast address
is currently rejected and directed-broadcast/multicast targets are an
accepted, unenforced gap).

### WR-02: `channelmap.go`'s own doc comments contradict each other about rounding behavior

**File:** `internal/artnet/channelmap.go:47`, `internal/artnet/channelmap.go:114-117`

**Issue:** `Encode`'s doc comment claims byte scaling is
"deterministically (round-half-up)" (line 47), but `scaleToByte`'s own doc
comment two lines above its body says the opposite: "deterministically
truncating toward zero (0.5 -> 127, never 128)" (lines 114-116). The
implementation (`byte(value * 255)`) does in fact truncate, matching the
test (`TestEncodeOffsetAndScaling` asserts `0.5 -> 127`, which is truncation,
not round-half-up, which would produce `128`). The two doc comments in the
same file disagree about the contract; a future maintainer trusting
`Encode`'s "round-half-up" claim (e.g. while cross-referencing expected
byte values against a Wireshark capture per the ARTN-06 verification
runbook) would compute the wrong expected byte for values whose fraction
is exactly `0.5 / 255`-aligned.

**Fix:** Fix `Encode`'s doc comment to say "truncating toward zero" to
match `scaleToByte` and the actual/tested behavior:
```go
// ... scaling each channel's normalized [0,1] value to an 8-bit [0,255]
// byte deterministically (truncating toward zero, 0.5 -> 127, never 128) ...
```

### WR-03: Daemon caches the pinned interface's local IP once at `Run` startup and never re-resolves it, even across "artnet configure" reconfigures

**File:** `internal/artnet/daemon.go:163-181`, `internal/artnet/worker.go:189-200`

**Issue:** `Run` fetches `localIP` exactly once:
```go
localIP, _ := ifaceMgr.LocalIP()
...
d := &daemon{ ... localIP: localIP, ... }
```
This value is stored on `daemon` and reused verbatim by every subsequent
`startWorkerLocked()` call (including every `reconfigureLocked()` triggered
by "artnet configure" / "artnet target enable|disable"), which passes it
straight through to `NewWorker(WorkerConfig{LocalIP: d.localIP, ...})` ->
`dialUDP`'s `laddr`. `ifaceMgr.LocalIP()` itself re-resolves the interface's
current address at call time (its own doc comment says so), but the
daemon never calls it again after the very first `Run` invocation.

Two concrete consequences:
1. If the pinned interface has no IPv4 address yet at daemon startup (a
   documented "degraded state" the code comment accepts), `d.localIP` stays
   `nil` for the entire remaining lifetime of the daemon process, even
   after the interface later acquires an address — every future dial binds
   to no specific local address instead of the pinned interface, silently
   defeating the pinned-interface guarantee (CONTEXT D-05) until the
   daemon is restarted.
2. If the interface's IP address changes later (DHCP lease renewal without
   the interface going down/disappearing, which would not flip
   `InterfaceManager`'s status to `Lost`), every subsequent
   "artnet configure"/"artnet target enable|disable" reconfigure keeps
   dialing with the stale address, which can fail to bind
   ("cannot assign requested address") for every target, not just the one
   being reconfigured.

**Fix:** Re-resolve `ifaceMgr.LocalIP()` inside `startWorkerLocked()` (or at
the top of `reconfigureLocked()`) instead of reading the `daemon`-level
snapshot captured once in `Run`:
```go
func (d *daemon) startWorkerLocked() {
	if ip, err := d.ifaceMgr.LocalIP(); err == nil {
		d.localIP = ip
	}
	d.worker = NewWorker(WorkerConfig{ ... LocalIP: d.localIP, ... })
	d.worker.Start(d.baseCtx)
}
```

### WR-04: `Worker.Stop()` does not wait for in-flight per-target send goroutines, so a straggling send from a just-stopped worker generation can still record into the (shared) `Health` after `Stop()` returns

**File:** `internal/artnet/worker.go:240-257`, `internal/artnet/worker.go:309-337`

**Issue:** `dispatchSend` launches an unawaited goroutine per send
(`go func() { ... health.RecordSend(...) }()`). `Stop()`'s doc comment
claims it "waits for the tick goroutine to exit before closing every
dialed target sender," and it does wait for the tick loop
(`<-w.done`), but it never waits for any outstanding `dispatchSend`
goroutines still mid-flight from the last tick(s) before that. Immediately
after `Stop()` returns, `daemon.reconfigureLocked()` calls
`startWorkerLocked()` to build a brand-new `Worker` sharing the *same*
`*Health` instance (`WorkerConfig.Health: d.health`, by design, to
preserve historical counters). If a straggling goroutine from the just-
stopped worker generation is still executing `sender.Write` /
`health.RecordSend` when the new worker starts, its result (very plausibly
a "use of closed network connection" error, since `Stop()` closes every
sender right after) is recorded into the shared `Health` and can be
misattributed to a target that is otherwise healthy in the new worker
generation — a spurious `SendErr`/`LastError` right after a routine
"artnet target enable/disable" or "artnet configure" call, with no
corresponding real failure in the new worker.

**Fix:** Track in-flight `dispatchSend` goroutines with a `sync.WaitGroup`
and have `Stop()` wait on it (with a bound, e.g. after cancel, before
closing senders) so no send from a stopped generation can still write into
`Health` after `Stop()` returns:
```go
type Worker struct {
	...
	sendWG sync.WaitGroup
}

func (w *Worker) dispatchSend(ts *targetState, pkt []byte) {
	...
	w.sendWG.Add(1)
	go func() {
		defer w.sendWG.Done()
		defer ts.busy.Store(false)
		...
	}()
}

func (w *Worker) Stop() {
	if w.cancel != nil { w.cancel() }
	if w.done != nil { <-w.done }
	w.sendWG.Wait()
	for _, u := range w.universes { ... }
}
```

## Info

### IN-01: `EncodeArtDMX`'s inline comment describes the Net byte mask backwards

**File:** `internal/artnet/packet.go:56-57`

**Issue:** The inline comment says `// Net: top 7 bits, bit 7 reserved/zero`
for `buf[15] = byte((portAddress >> 8) & 0x7f)`. The mask `0x7f` keeps the
*low* 7 bits of the shifted byte and clears bit 7 (the top bit) — the
implementation is correct (matches the Art-Net spec: Net occupies bits 0-6
of that byte, bit 7 reserved zero), but the comment's "top 7 bits" wording
is the reverse of what the mask actually does, which could confuse a
future reader diffing this against a spec table.

**Fix:** Reword to "Net: low 7 bits of this byte, bit 7 reserved/zero" (or
similar) to match the mask.

---

_Reviewed: 2026-07-22T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
