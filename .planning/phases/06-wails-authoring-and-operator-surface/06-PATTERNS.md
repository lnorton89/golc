# Phase 6: Wails Authoring and Operator Surface - Pattern Map

**Mapped:** 2026-07-23
**Files analyzed:** 12 (per RESEARCH.md "Recommended Project Structure")
**Analogs found:** 10 / 12

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|--------------------|------|-----------|-----------------|----------------|
| `internal/artnet/safety.go` (extends `daemon.go`) | service (daemon-resident state) | event-driven | `internal/artnet/daemon.go` (targets/worker mutation state + `handle` dispatch) | exact |
| `internal/artnet/daemon.go` (extended: safety routes + Worker check) | controller (IPC route dispatch) | request-response | `internal/artnet/daemon.go` itself (`handleConfigure`/`handleSetEnabled`) | exact |
| `internal/artnet/worker.go` (extended: per-tick safety-flag check) | service (tick loop) | streaming | `internal/artnet/worker.go` `tick()` | exact |
| `internal/artnet/ipc/types.go` / new safety IPC route | model (wire types) | request-response | `internal/artnet/ipc/types.go` | exact |
| `internal/wails/app.go` | controller (process lifecycle + Wails bindings) | event-driven | `internal/artnet/daemon.go` `Run()` (lifecycle) + `internal/artnet/ipc/client.go` (`Dial`) | role-match |
| `internal/wails/events.go` | service (throttled push) | streaming | `internal/artnet/daemon.go` `handleStatus`/`statusPayload` (snapshot flattening for wire) | role-match |
| `internal/midi/driver.go` | service (device I/O) | event-driven | `internal/artnet/interfacemgr.go` (external-resource lifecycle: open/poll/reconnect) — **no direct analog**, see below | partial |
| `internal/midi/learn.go` | service (capture session + conflict check) | event-driven | `internal/artnet/daemon.go` `handleConfigure` (validate-then-replace-by-key pattern) | role-match |
| `internal/midi/takeover.go` | utility (pure state machine) | transform | `internal/artnet/worker.go` `nextSeq`/`tick` (small per-tick pure state update) — partial; no true analog | partial |
| `internal/operatorsurface/model.go` | model (named collection + item assignment) | CRUD | `internal/pool/model.go` (`Pool`/`Group`/`MemberRef` — named collection referencing member items by ID) | exact |
| `internal/operatorsurface/validate.go` | utility (validation) | transform | `internal/show/state.go` `validate()` (single validate() entry point every object type extends) | exact |
| `internal/command/wails.go` or `internal/command/operatorsurface.go` (CLI/command surface for new state) | controller (command dispatch) | CRUD | `internal/command/playback.go` (parse-args → Load → mutate → Save → Stdout shape) | exact |

## Pattern Assignments

### `internal/artnet/safety.go` (service, event-driven) + `daemon.go` extension

**Analog:** `internal/artnet/daemon.go`

**Imports pattern** (daemon.go lines 33-48):
```go
import (
	"context"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lnorton89/golc/internal/artnet/ipc"
	"github.com/lnorton89/golc/internal/deployment"
	"github.com/lnorton89/golc/internal/playback"
	"github.com/lnorton89/golc/internal/show"
	"github.com/lnorton89/golc/internal/strictjson"
)
```

**Daemon-resident mutable state pattern** (daemon.go lines 76-95, the `daemon` struct's `mu`-guarded fields): add a `safety safetyState` field alongside `targets`/`worker`, guarded by the *same* `d.mu` used for `targets`/`worker` — do not introduce a second lock. `safetyState` itself should use `atomic.Bool`/`atomic.Pointer` fields per RESEARCH.md Pattern 1, since the *Worker's tick goroutine* must read it lock-free every ~25ms without contending with `d.mu`:
```go
type daemon struct {
	// ...existing fields...
	safety safetyState // NEW — read lock-free by Worker.tick, mutated via IPC routes
}
```

**Route dispatch pattern** (daemon.go lines 210-224, `handle`):
```go
func (d *daemon) handle(request ipc.Request) ipc.Result {
	switch request.Route {
	case "artnet status":
		return d.handleStatus()
	case "artnet configure":
		return d.handleConfigure(request.Args)
	// NEW: case "artnet safety blackout": return d.handleBlackout(request.Args)
	// NEW: case "artnet safety revoke-automation": return d.handleRevokeAutomation(request.Args)
	// NEW: case "artnet safety stop-all": return d.handleStopAll(request.Args)
	default:
		return ipc.Result{ExitCode: 2, Stderr: []byte(fmt.Sprintf(
			"GOLC_ARTNET_ROUTE_UNKNOWN: the daemon has no operation for route %q\n", request.Route))}
	}
}
```

**Mutation-then-reconfigure pattern** (daemon.go lines 353-404, `handleConfigure`): parse args -> validate -> lock `d.mu` -> mutate -> unlock. Safety flags should follow the same validate-then-set shape but MUST NOT route through `reconfigureLocked()` (stop/start Worker) — that is disk/config-reload latency inappropriate for a sub-frame-period override. Instead, `safetyState`'s atomic fields are read directly by `Worker.tick` (see worker.go pattern below), so setting them takes effect on the very next tick with no Worker restart.

**Error/diagnostic convention** (daemon.go lines 221-223, 341-344): reuse the `GOLC_ARTNET_{CONDITION}` uppercase-snake diagnostic convention, `ipc.Result{ExitCode: 1|2, Stderr: [...]}`, never a panic.

---

### `internal/artnet/worker.go` (extended: per-tick safety check)

**Analog:** `internal/artnet/worker.go` `tick()` (lines 280-311)

**Core per-tick pattern to extend:**
```go
func (w *Worker) tick(frame *playback.Frame) {
	if frame == nil {
		return
	}
	w.health.RecordFrame(time.Now())
	// NEW: if w.safety.blackout.Load() { frame = blackoutFrame(...) }
	// checked here, BEFORE the per-universe Encode loop, so blackout
	// unconditionally overrides whatever Engine.CurrentFrame() currently holds.
	for _, u := range w.universes {
		buffers, err := Encode(*frame, w.instancesByUniverse[u], w.resolve)
		...
```

**Non-blocking discipline** (worker.go doc comment lines 1-34): the tick goroutine must never block on IPC, locks shared with slow paths, or safety-flag mutation — only atomic loads. This is the same discipline already proven by `dispatchSend`'s bounded-goroutine-per-target design; the safety-flag read must be equally cheap (an `atomic.Bool.Load()`), not a `d.mu.Lock()`.

---

### `internal/artnet/ipc/types.go` (extend for safety routes)

**Analog:** `internal/artnet/ipc/types.go` (whole file, 48 lines) — **no new type needed**. `ipc.Request`/`ipc.Result` are already route+args+root generic; a new safety route (e.g. `"artnet safety blackout"`) is just a new `Route` string value dispatched by `daemon.handle`, exactly like `"artnet configure"`. Do not add a parallel wire-type family for safety — reuse `Request`/`Result` verbatim (see file's own doc comment on why these types are declared locally to avoid an import cycle with `internal/command`).

---

### `internal/wails/app.go` (controller, process lifecycle)

**Analog 1 — daemon reachability + spawn-if-absent:** `internal/artnet/ipc/client.go` `Dial` (lines 38-45):
```go
func Dial(pipeName string) (net.Conn, error) {
	timeout := dialTimeout
	conn, err := winio.DialPipe(pipeName, &timeout)
	if err != nil {
		return nil, fmt.Errorf("GOLC_ARTNET_DAEMON_UNREACHABLE: is the GOLC background process running? (%v)", err)
	}
	return conn, nil
}
```
Wails' `OnStartup` should call this exact `ipc.Dial`/pattern first; on `GOLC_ARTNET_DAEMON_UNREACHABLE`, spawn `golc-project.exe artnet serve` as a supervised child process (RESEARCH.md Open Question 1), then retry Dial — do not reimplement pipe dialing.

**Analog 2 — lifecycle start/stop symmetry:** `internal/artnet/daemon.go` `Run()` (lines 150-204): construct dependencies, start them in order, defer/ordered-stop in reverse order on shutdown. `internal/wails/app.go`'s `OnStartup`/`OnShutdown` should mirror this same ordered start/ordered-reverse-stop discipline (hotkey registration -> MIDI driver open -> IPC dial/spawn on startup; reverse on shutdown).

**Command forwarding pattern:** `internal/artnet/ipc/client.go` `Forward` (lines 53-76) — every Wails-bound Go method that issues a typed command should build an `ipc.Request` (or `command.Request` via the existing `internal/command` registry, per CONTEXT canonical refs) and call `Forward`, converting errors into a Wails-side error/Result the same way `internal/command/artnet.go`'s client routes do (see below) — never construct a second dispatch mechanism.

---

### `internal/wails/events.go` (service, throttled push)

**Analog:** `internal/artnet/daemon.go` `newStatusPayload`/`statusPayload` (lines 226-307): flatten internal state into a small, explicit, JSON-safe struct before pushing over the wire — never marshal an internal map/pointer type directly (the file's own doc comment explains *why*: `encoding/json` cannot marshal non-string/int map keys, and `[]byte` fields base64-encode automatically, both directly reusable conventions for a `StatusSnapshot` pushed via `EventsEmit`).

**Throttling pattern:** RESEARCH.md's own `pushStatus` example (Code Examples section) — call `EventsEmit` at a fixed cadence, never once per Engine/MIDI message. Model the throttle loop on `worker.go`'s `time.NewTicker(workerTickInterval)` `Start`/`Stop` lifecycle (lines 209-238), using an independent ticker interval documented as intentionally decoupled from both the 40Hz worker tick and MIDI message rate, per the project's established "independent cadence, never share one ticker" convention (worker.go doc comment lines 50-54).

---

### `internal/midi/driver.go` (service, external-device I/O)

**No strong analog found.** The closest structural precedent is `internal/artnet/interfacemgr.go` (external-resource lifecycle: pin/poll/reconnect/status), but MIDI hot-plug and gomidi's driver API are different enough that this should be treated as new-pattern code per RESEARCH.md's own Pattern 3/Pitfall 3 guidance (use `midicatdrv`, avoid CGo) rather than forced into interfacemgr's shape. Recommend skimming `interfacemgr.go`'s **status/reconnect surface only** (its exported `Status()`/`Err()` accessor shape) for the MIDI port's own reachability reporting, not its internals.

---

### `internal/midi/learn.go` (service, bounded capture session)

**Analog:** `internal/artnet/daemon.go` `handleConfigure` (lines 353-404) — the "parse candidate -> validate against existing set (uniqueness) -> reject or replace -> return diagnostic" shape maps directly onto D-06's "collision rejected outright, existing mapping untouched" rule:
```go
existing := d.targets[universe]
updated := make([]Target, 0, len(existing)+1)
replaced := false
for _, t := range existing {
	if keyOf(t) == keyOf(target) {
		updated = append(updated, target)
		replaced = true
		continue
	}
	updated = append(updated, t)
}
if !replaced {
	updated = append(updated, target)
}
if err := ValidateUniqueTargets(updated); err != nil {
	return ipc.Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
}
```
For D-06 (learn is a hard rejection, never a silent replace), the learn-conflict path should **stop at the "already exists" check and return an error** rather than following `handleConfigure`'s replace-by-key branch — i.e., adapt only the lookup/validate halves of this pattern, not the replace-in-place half.

---

### `internal/midi/takeover.go` (utility, pure state machine)

**No direct in-repo analog** — RESEARCH.md Pattern 4's `TakeoverState.Update` is the concrete implementation to use (cross-to-catch crossing check, direction-aware, never proximity-based per D-11/Pitfall 2). Structurally, treat this the same way `internal/artnet/worker.go`'s `nextSeq` treats small mutable per-key state (`w.seqMu.Lock(); ...; w.seqMu.Unlock()`) — a tiny, mutex- or atomic-guarded pure-transform helper with no I/O — but the actual crossing algorithm has no precedent in this codebase; write unit tests directly against RESEARCH.md's example rather than adapting an existing test file.

---

### `internal/operatorsurface/model.go` (model, CRUD)

**Analog:** `internal/pool/model.go` (`Pool`, `PoolMember`, `Group`, `MemberRef`, lines 1-58)

**Imports pattern** (pool/model.go lines 8-17):
```go
import (
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/fixture"
)
```

**Named-collection-with-member-refs pattern** (pool/model.go lines 44-53, directly analogous to D-02's "multiple named operator surfaces" + D-01/D-03's "individual item assignment"):
```go
type Group struct {
	ID         uuid.UUID   `json:"id"`
	Name       string      `json:"name"`
	MemberRefs []MemberRef `json:"member_refs,omitempty"`
}

type MemberRef struct {
	PoolID       uuid.UUID `json:"pool_id"`
	PoolMemberID uuid.UUID `json:"pool_member_id"`
}
```
`operatorsurface.Surface` should follow this exact shape: `ID` (UUIDv7, minted once, never re-derived from `Name` — see `NewPool`'s doc comment convention below), `Name string`, and a set of typed reference lists (e.g. `SceneRefs []uuid.UUID`, `LayerRefs []LayerRef`, `MasterRefs []uuid.UUID`, `SafetyRefs []string`) — individual-item refs, no bulk/category ref type (D-03). D-07's per-surface MIDI mapping set is a sibling field on the same `Surface` struct (e.g. `MidiMappings []MidiMapping`), keeping mappings scoped to the surface exactly as `Group.MemberRefs` scopes membership to the group.

**Identity-minting pattern** (pool/model.go lines 78-92, `NewPool`):
```go
func NewPool(name string, requiredCapabilities []fixture.CapabilityType) (Pool, error) {
	if strings.TrimSpace(name) == "" {
		return Pool{}, fmt.Errorf("GOLC_POOL_NAME_EMPTY: pool name must not be empty")
	}
	...
	id, err := uuid.NewV7()
	...
}
```
Apply verbatim to `NewSurface(name string) (Surface, error)`: empty-name rejection with a `GOLC_OPERATORSURFACE_NAME_EMPTY`-style diagnostic, UUIDv7 minted once at creation, never re-minted by a later rename.

---

### `internal/operatorsurface/validate.go` (utility, transform)

**Analog:** `internal/show/state.go` `validate()` (lines 84-167) — the single validate() entry point every new object type extends, called by both Load and Save before trusting/persisting. `internal/show/state.go` lines 106-110 show the exact reference-integrity check shape to copy for surface item assignments (an assignment referencing a since-deleted scene/layer/master must fail validation, mirroring `pool.ValidateGroupReferences`):
```go
if err := pool.ValidateUniqueGroupNames(s.Groups); err != nil {
	return err
}
if err := pool.ValidateGroupReferences(s.Pools, s.Groups); err != nil {
	return err
}
```
`operatorsurface.Validate(surfaces []Surface, scenes []scene.Scene, ...)` should follow this same "unique names, then referential integrity against the owning collections" two-step, and `internal/show/state.go`'s `validate()` must call it — Phase 6 extends the single revisioned document (per CONTEXT's own Integration Points note), not a parallel model.

---

### `internal/command/operatorsurface.go` (or similar) (controller, CRUD)

**Analog:** `internal/command/playback.go` (whole file pattern, lines 1-80 shown)

**Scope/route self-registration pattern** (playback.go lines 33-63):
```go
var _ = MustDeclareScope(ScopeRegistration{
	Scope:   "playback",
	Summary: "...",
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "playback bpm set",
	Summary: "...",
	Handler: runPlaybackBPMSet,
})
```
New operator-surface authoring routes (assign/unassign/create-surface/list-surfaces) should self-register the same way under a new `operatorsurface` (or `surface`) scope, following the file's documented "parse-args-then-Load-mutate-Save-Stdout shape" (playback.go's own doc comment, lines 6-9).

**Two-tier error convention** (playback.go doc comment lines 6-9, `parsePlaybackBPMSetArgs`): malformed args = `GOLC_{DOMAIN}_USAGE` at ExitCode 2; validated-but-rejected domain values = `GOLC_{DOMAIN}_{CONDITION}` at ExitCode 1. Reuse directly for `GOLC_OPERATORSURFACE_USAGE` / `GOLC_OPERATORSURFACE_{CONDITION}`.

## Shared Patterns

### `{DOMAIN}_{CONDITION}` diagnostic convention
**Source:** `internal/artnet/daemon.go` (`GOLC_ARTNET_ROUTE_UNKNOWN`, `GOLC_ARTNET_USAGE`, etc.), `internal/pool/model.go` (`GOLC_POOL_NAME_EMPTY`, `GOLC_POOL_CAPABILITY_UNSUPPORTED`)
**Apply to:** every new diagnostic in `internal/midi`, `internal/operatorsurface`, `internal/artnet/safety.go`, `internal/wails` — `GOLC_MIDI_*`, `GOLC_OPERATORSURFACE_*`, `GOLC_ARTNET_SAFETY_*`, `GOLC_WAILS_*`.
```go
return fmt.Errorf("GOLC_MIDI_MAPPING_CONFLICT: note/CC %v is already mapped to %q on this surface", candidate, existingControl)
```

### Daemon as single owner of runtime-mutable state, guarded by one mutex
**Source:** `internal/artnet/daemon.go` lines 76-140 (`daemon` struct + `startWorkerLocked`/`stopWorkerLocked`/`reconfigureLocked`, all documented "callers must hold d.mu")
**Apply to:** `internal/artnet/safety.go`'s new atomic safety flags (read lock-free by `Worker.tick`, mutated via the daemon's existing `d.mu`-guarded IPC handler path) and any other daemon-resident Phase 6 state.

### Named-pipe IPC client/server reuse — never a second transport
**Source:** `internal/artnet/ipc/client.go` (`Dial`, `Forward`), `internal/artnet/ipc/server.go` (`Serve`, length-prefixed framing)
**Apply to:** `internal/wails/app.go`'s daemon connection and every safety-cluster trigger path (on-screen button AND OS-level hotkey callback) — both must call the same `ipc.Dial`/`ipc.Forward`, never a JS-mediated-only path (RESEARCH.md Pitfall 1).

### Single validate() entry point per document
**Source:** `internal/show/state.go` `validate()` (lines 84-167), called from `store.go`'s Load/Save
**Apply to:** `internal/operatorsurface/validate.go` must be wired into `internal/show/state.go`'s existing `validate()` function as one more step, not a separate validation path invoked elsewhere.

### UUIDv7 identity minted once, never derived from Name
**Source:** `internal/pool/model.go` `NewPool` (lines 78-92) — "IDs are minted only at creation time... never derived from Name, and never re-minted by Rename"
**Apply to:** `operatorsurface.Surface.ID`, any new MIDI-mapping record ID.

### Copy-returning mutation (never alias caller-owned slices/maps)
**Source:** `internal/artnet/daemon.go` `copyTargets` (lines 97-106): "daemon never aliases the caller's own map/slices"
**Apply to:** `internal/operatorsurface`'s assignment-mutation functions and `internal/midi/learn.go`'s mapping-set mutation — always return a fresh copy, mirroring `deployment`'s established copy-returning discipline (referenced directly in the `copyTargets` doc comment).

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `internal/midi/driver.go` | service | event-driven | No MIDI or comparable hot-pluggable-device I/O package exists yet; `interfacemgr.go`'s status/reconnect *surface* is a partial precedent but its internals (network interface polling) do not transfer to MIDI port I/O. Use RESEARCH.md's `gomidi/midi/v2` + `midicatdrv` guidance directly. |
| `internal/midi/takeover.go` | utility | transform | No soft-takeover / cross-to-catch state machine exists anywhere in this codebase; use RESEARCH.md Pattern 4's `TakeoverState.Update` example as the primary reference instead of an in-repo analog. |

## Metadata

**Analog search scope:** `internal/artnet/`, `internal/artnet/ipc/`, `internal/command/`, `internal/show/`, `internal/pool/`, `internal/scene/`, `internal/playback/`
**Files scanned:** `daemon.go`, `worker.go`, `ipc/types.go`, `ipc/client.go`, `ipc/server.go`, `command/router.go`, `command/artnet.go`, `command/playback.go`, `show/state.go`, `show/schema.go`, `pool/model.go`
**Pattern extraction date:** 2026-07-23
