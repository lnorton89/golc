---
phase: 04-observable-art-net-live-output
reviewed: 2026-07-22T00:00:00Z
depth: standard
files_reviewed: 7
files_reviewed_list:
  - internal/artnet/health.go
  - internal/artnet/worker.go
  - internal/artnet/health_test.go
  - internal/artnet/daemon.go
  - internal/artnet/daemon_test.go
  - internal/command/artnet.go
  - internal/command/artnet_test.go
findings:
  critical: 0
  warning: 2
  info: 2
  total: 4
status: issues_found
---

# Phase 04: Code Review Report (Gap-Closure: 04-08 UniverseValues, 04-09 Pinned-Interface Status)

**Reviewed:** 2026-07-22T00:00:00Z
**Depth:** standard
**Files Reviewed:** 7
**Status:** issues_found

## Summary

Reviewed the two gap-closure plans layered onto the already-reviewed Phase 4
Art-Net subsystem: 04-08's per-universe DMX value recording/publishing
(`Health.RecordUniverseValues`, the second `atomic.Pointer`-published map,
and its threading through `statusPayload`/`artnetStatusPayload`), and
04-09's pinned-interface status surfacing (`InterfaceManager.Status()`/
`Err()` read into the daemon's status payload, plus the best-effort daemon
round trip in `artnet interface list`).

The `UniverseValues` bounded-tracking, defensive-copy, and lock-free-publish
design mirrors the existing `Targets` pattern correctly and is well covered
by `health_test.go`/`daemon_test.go`/`artnet_test.go`. The manually-mirrored
JSON wire structs between `daemon.go` and `command/artnet.go` (`statusPayload`
vs `artnetStatusPayload`, `interfaceStatusPayload` vs `artnetInterfaceStatus`,
`universeValues` vs `artnetUniverseValues`) were checked field-by-field
against `strictjson.DecodeStrict`'s `DisallowUnknownFields` behavior and are
currently consistent — no wire-shape mismatch found there.

Two real defects were found in the 04-09 interface-status surfacing: a
non-atomic split read in `daemon.go`'s `handleStatus()` that can produce a
status payload violating its own documented invariant during the interface-
loss transition, and a JSON-mode information loss in `artnet interface
list --json` where the plain-text rendering surfaces the interface error
but the JSON rendering silently drops it. Per the gap-closure scope note,
WR-01/WR-03/WR-04 from the original `04-REVIEW.md` are known/triaged and are
not re-flagged here.

## Warnings

### WR-01: `handleStatus`'s non-atomic `Err()`/`Status()` split read can produce a payload that violates its own documented invariant during an interface-loss transition

**File:** `internal/artnet/daemon.go:314-324`

**Issue:** `handleStatus` reads the pinned interface's error and status via
two separate, independently-synchronized calls:

```go
func (d *daemon) handleStatus() ipc.Result {
	ifaceErr := ""
	if err := d.ifaceMgr.Err(); err != nil {
		ifaceErr = err.Error()
	}
	iface := interfaceStatusPayload{
		PinnedIndex: d.ifaceMgr.PinnedIndex(),
		PinnedName:  d.ifaceMgr.PinnedName(),
		Status:      d.ifaceMgr.Status().String(),
		Error:       ifaceErr,
	}
	...
```

`InterfaceManager.Status()`/`Err()` each independently `Load()` the same
`atomic.Int32` at the moment they're called (`interfacemgr.go`), and the
interface's health poll loop can flip that status from OK to Lost
concurrently, at any time, on its own ticker. If that flip happens in the
gap between the `Err()` call (line 316, still reads OK, so `ifaceErr`
stays `""`) and the `Status()` call a few lines later (line 322, now reads
`Lost`), the resulting `interfaceStatusPayload` has `Status: "lost"` and
`Error: ""` simultaneously — directly contradicting this same file's own
doc comment for `interfaceStatusPayload` (lines 258-259): *"Error is the
GOLC_ARTNET_INTERFACE_LOST diagnostic string when lost, else empty."*

Because the transition is one-directional and terminal (`markLost` never
reverts to OK, per `interfacemgr.go`'s own doc comment), the fix is simple:
read `Status()` once, and only derive the error from it via that same
already-observed value (once observed `Lost`, it is guaranteed to remain
`Lost`, so a subsequent `Err()` call after that check can never race back to
empty).

**Fix:**
```go
func (d *daemon) handleStatus() ipc.Result {
	status := d.ifaceMgr.Status()
	ifaceErr := ""
	if status == InterfaceStatusLost {
		ifaceErr = d.ifaceMgr.Err().Error()
	}
	iface := interfaceStatusPayload{
		PinnedIndex: d.ifaceMgr.PinnedIndex(),
		PinnedName:  d.ifaceMgr.PinnedName(),
		Status:      status.String(),
		Error:       ifaceErr,
	}
	...
```
(Note `FrameHealth`'s own `evaluateFrameHealth` already avoids this exact
class of bug by computing `LastFrameAt`/`OnCadence` from a single evaluation
— the interface-status path should follow the same single-read discipline.)

### WR-02: `artnet interface list --json` silently drops the pinned interface's error diagnostic that the plain-text rendering of the same command includes

**File:** `internal/command/artnet.go:394-401`, `internal/command/artnet.go:423-455`

**Issue:** `interfaceListEntry`, the JSON wire shape for `artnet interface
list --json`, has no field to carry the interface error text:

```go
type interfaceListEntry struct {
	Index  int      `json:"index"`
	Name   string   `json:"name"`
	Up     bool     `json:"up"`
	Addrs  []string `json:"addrs"`
	Pinned bool     `json:"pinned"`
	Status string   `json:"status"`
}
```

The plain-text rendering path (same function, lines 464-472) appends
`pinnedError` into the displayed status string when non-empty:

```go
if daemonReachable && iface.Index == pinnedIndex {
    pinned = "yes"
    status = pinnedStatus
    if pinnedError != "" {
        status = status + " " + pinnedError
    }
}
```

but the JSON branch (lines 443-447) only ever sets `entry.Status =
pinnedStatus`, never surfacing `pinnedError` anywhere in the JSON output:

```go
entry := interfaceListEntry{Index: iface.Index, Name: iface.Name, Up: iface.Up, Addrs: addrs}
if daemonReachable && iface.Index == pinnedIndex {
    entry.Pinned = true
    entry.Status = pinnedStatus
}
```

A scripting/automation consumer of `artnet interface list --json` can see
`"status": "lost"` for the pinned candidate but has no way to learn *why*
from this command's output at all — they would have to separately call
`artnet status --json` (a different route) to recover the
`GOLC_ARTNET_INTERFACE_LOST` diagnostic text. This is an unnecessary
asymmetry between the two rendering modes of the same command and a real
functional gap for the documented "annotating... its live status" contract
(route Summary, `artnet.go:75-77`).

**Fix:** Add an `Error` field to `interfaceListEntry` and populate it
alongside `Status` in the JSON branch:
```go
type interfaceListEntry struct {
	Index  int      `json:"index"`
	Name   string   `json:"name"`
	Up     bool     `json:"up"`
	Addrs  []string `json:"addrs"`
	Pinned bool     `json:"pinned"`
	Status string   `json:"status"`
	Error  string    `json:"error"`
}
...
if daemonReachable && iface.Index == pinnedIndex {
    entry.Pinned = true
    entry.Status = pinnedStatus
    entry.Error = pinnedError
}
```

## Info

### IN-01: The daemon-side and CLI-side JSON wire structs for the status payload are hand-mirrored with no shared type or tag-consistency guard

**File:** `internal/artnet/daemon.go:247-274`, `internal/command/artnet.go:735-760`

**Issue:** `statusPayload`/`interfaceStatusPayload`/`universeValues`
(daemon.go) and `artnetStatusPayload`/`artnetInterfaceStatus`/
`artnetUniverseValues` (command/artnet.go) are independently declared
structs that must stay field-for-field, tag-for-tag identical for
`strictjson.DecodeStrict`'s `DisallowUnknownFields` round trip to keep
working. They are currently consistent (verified during this review), and
existing tests (`TestDaemonStatusPayloadIncludesPinnedInterfaceStatus`,
`TestArtnetStatusJSONIncludesInterfaceStatus`, etc.) would catch a runtime
drift immediately — but nothing catches it at compile time, and a future
edit to one side's tag/field name (e.g. renaming `Error` on one struct only)
would only surface as a `STRICTJSON_DECODE` failure at test/runtime, not a
build error.

**Fix:** Consider having `command/artnet.go` decode directly into
`artnet.FrameHealth`/`artnet.TargetHealth`-style shared types for the new
`Universes`/`Interface` shapes too (define `artnet.UniverseValues` and
`artnet.InterfaceStatusPayload` exported types in the `artnet` package and
import them from `command`, the same way `artnet.TargetHealth` already is),
rather than maintaining a second hand-written mirror per new field added.

### IN-02: `RecordUniverseValues`'s historical-retention behavior on universe removal is undocumented, unlike the equivalent (and intentional) `Targets` behavior it mirrors

**File:** `internal/artnet/health.go:160-186`, `internal/artnet/health.go:253-270`

**Issue:** `Configure`'s doc comment explicitly documents that a
previously-tracked `Targets` entry survives a reconfigure that drops it from
the configured set ("preserved in Targets for historical display rather
than deleted outright," lines 148-150). `RecordUniverseValues`'s doc comment
(lines 253-260) only describes the bound on *new* entries ("a universe never
declared via Configure is silently dropped, never allocating a new tracking
entry") and says nothing about what happens to an already-recorded
universe's entry once a later `Configure` call removes it from
`configuredUniverses` — in fact `h.universeValues` is never pruned in
`Configure` (`daemon.go:180-186`), so the same historical-retention behavior
applies silently by omission rather than by documented design. A future
maintainer reading only `RecordUniverseValues`'s doc comment could
reasonably (and incorrectly) assume a de-configured universe's stale
512-byte buffer disappears from `Snapshot().UniverseValues` after a
reconfigure; it does not.

**Fix:** Extend `RecordUniverseValues`'s (or `Configure`'s) doc comment to
state explicitly, mirroring the `Targets` wording: "a universe dropped from
the configured set on a later `Configure` call keeps its last-recorded
`UniverseValues` entry for historical display rather than having it
deleted."

---

_Reviewed: 2026-07-22T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
