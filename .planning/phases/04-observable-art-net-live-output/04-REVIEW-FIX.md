---
phase: 04-observable-art-net-live-output
fixed_at: 2026-07-23T04:05:00Z
review_path: .planning/phases/04-observable-art-net-live-output/04-REVIEW.md
iteration: 1
findings_in_scope: 2
fixed: 2
skipped: 0
status: all_fixed
---

# Phase 04: Code Review Fix Report

**Fixed at:** 2026-07-23T04:05:00Z
**Source review:** .planning/phases/04-observable-art-net-live-output/04-REVIEW.md
**Iteration:** 1

**Scope note:** This fix pass covers only the two new findings from the
"## Gap-Closure Review (04-08, 04-09)" section at the end of 04-REVIEW.md
(GC-WR-01, GC-WR-02). The original review's CR-01/WR-02/IN-01 were already
fixed and committed in `f12d05e` prior to this pass, and WR-01/WR-03/WR-04
carry an explicit prior decision to leave them open as non-blocking
follow-up (documented in 04-REVIEW.md's "## Resolution" section) -- none
of the six original findings were touched or re-opened here.

**Summary:**
- Findings in scope: 2
- Fixed: 2
- Skipped: 0

## Fixed Issues

### GC-WR-01: `handleStatus`'s non-atomic `Err()`/`Status()` split read can produce a payload that violates its own documented invariant during an interface-loss transition

**Files modified:** `internal/artnet/daemon.go`
**Commit:** `bb6ba3b`
**Applied fix:** `handleStatus` now reads `d.ifaceMgr.Status()` exactly
once into a local variable, and only calls `d.ifaceMgr.Err()` when that
single observed status equals `InterfaceStatusLost`. This eliminates the
window where an OK->Lost transition between two separate atomic loads
could previously produce `Status: "lost"` with `Error: ""`. Safe because
`InterfaceManager`'s Lost transition is one-directional/terminal
(`markLost` never reverts to OK), so deriving `Err()` from an
already-observed-Lost status can never race back to OK. Mirrors the
single-read discipline already used by `FrameHealth.evaluateFrameHealth`
in the same package (`internal/artnet/health.go`).

Verified: `gofmt -l`, `go build ./internal/artnet/...`, and the full
`go test ./internal/artnet/... ./internal/command/... -count=1 -race`
suite all pass with no regressions.

### GC-WR-02: `artnet interface list --json` silently drops the pinned interface's error diagnostic that the plain-text rendering of the same command includes

**Files modified:** `internal/command/artnet.go`
**Commit:** `179b74c`
**Applied fix:** Added `Error string \`json:"error"\`` to
`interfaceListEntry` and populated it (`entry.Error = pinnedError`)
alongside `Status` in the JSON-rendering branch of
`runArtnetInterfaceList`, mirroring how the plain-text branch already
appends `pinnedError` to the status column when non-empty. A scripting
consumer of `--json` can now read the error diagnostic directly instead
of needing a separate `artnet status --json` call.

Verified: `gofmt -l`, `go build ./internal/command/...`, and the full
`go test ./internal/artnet/... ./internal/command/... -count=1 -race`
suite all pass with no regressions. Existing tests
(`TestArtnetInterfaceListAnnotatesPinnedWhenDaemonRunning`,
`TestArtnetInterfaceListWorksWithNoDaemon`) continue to pass unchanged
since the new field is additive and zero-valued in the no-error/no-daemon
cases they assert on.

## Skipped Issues

None -- both in-scope findings were fixed.

---

_Fixed: 2026-07-23T04:05:00Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
