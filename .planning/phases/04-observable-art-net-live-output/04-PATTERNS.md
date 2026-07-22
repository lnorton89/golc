# Phase 4: Observable Art-Net Live Output - Pattern Map

**Mapped:** 2026-07-22
**Files analyzed:** 17 (new/modified, per RESEARCH.md's Recommended Project Structure + CONTEXT D-16's additive fixture-model change)
**Analogs found:** 14 / 17 (3 are genuinely greenfield — no in-repo protocol/daemon precedent exists, per RESEARCH.md's own explicit callout)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `internal/artnet/packet.go` | utility (protocol codec) | transform | `internal/fixture/decode.go` (strict-decode + validation shape) | partial (no binary-codec precedent exists; imports/error-convention only) |
| `internal/artnet/packet_test.go` | test | transform | `internal/fixture/decode_test.go` | partial |
| `internal/artnet/channelmap.go` | utility (pure mapping) | transform | `internal/scene/layer.go` (`ReduceLayers`/`AttributeSet.Overlay`) | role-match (pure fn of inputs, no I/O) |
| `internal/artnet/channelmap_test.go` | test | transform | `internal/scene/layer_test.go` | role-match |
| `internal/artnet/worker.go` | service (background loop) | streaming | `internal/playback/engine.go` (`Engine.Start`/`tick`/atomic.Pointer) | exact (this is the literal consumer engine.go's own doc comment names) |
| `internal/artnet/worker_test.go` | test | streaming | `internal/playback/engine_test.go` | role-match |
| `internal/artnet/target.go` | model | CRUD | `internal/deployment/model.go` (`Instance`, validation helpers) | role-match |
| `internal/artnet/interfacemgr.go` | service (OS boundary) | event-driven | `internal/playback/engine.go` (Start/Stop/ctx lifecycle) | partial (no NIC-enumeration precedent; lifecycle shape only) |
| `internal/artnet/interfacemgr_test.go` | test | event-driven | `internal/deployment/model_test.go` | partial |
| `internal/artnet/discovery.go` | service (network I/O) | request-response | `internal/artnet/packet.go` (same package, shares codec) | n/a (greenfield, same-package) |
| `internal/artnet/health.go` | model + state machine | event-driven | `internal/playback/clock.go` (`ValidateBPM`/pure state helpers) — see below | partial |
| `internal/artnet/health_test.go` | test | event-driven | `internal/deployment/model_test.go` | partial |
| `internal/artnet/ipc/server.go` | service (daemon transport) | request-response | none — see "No Analog Found" | none |
| `internal/artnet/ipc/client.go` | service (thin client) | request-response | `internal/command/router.go` (`Request`/`Result` shapes to reuse as wire format) | role-match (shape reuse, not transport) |
| `internal/artnet/daemon.go` | service (process entrypoint) | event-driven | `internal/playback/engine.go` (`Start(ctx)`/`Stop()` lifecycle) | role-match |
| `internal/command/artnet.go` | controller (CLI route file) | request-response | `internal/command/playback.go` (route/scope self-registration, arg parsing, JSON output) | exact |
| `internal/fixture/model.go` (additive edit) | model | CRUD | itself — same file, additive field per D-16 | exact (in-place) |
| `internal/fixture/ofl/model.go` / `normalize.go` (additive edit) | model/transform | transform | itself — same file, additive field per D-16/Pitfall 1 | exact (in-place) |

## Pattern Assignments

### `internal/artnet/packet.go` (utility/codec, transform)

**Analog:** `internal/fixture/decode.go` (for error-diagnostic and validation conventions only — no binary-codec precedent exists anywhere in this repo; the actual byte-layout logic must follow RESEARCH.md Pattern 1 exactly, not an in-repo analog).

**Imports pattern** (`internal/fixture/decode.go` lines 17-24):
```go
import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	yaml "go.yaml.in/yaml/v4"
)
```
For `packet.go`, the equivalent import set is `encoding/binary`, `fmt` — see RESEARCH.md Pattern 1's `EncodeArtDMX` example (already a complete, byte-exact reference implementation to copy near-verbatim):
```go
const (
	artNetPort  = 6454
	opOutputDMX = 0x5000
	protVerHi   = 0x00
	protVerLo   = 0x0e
)

func EncodeArtDMX(seq, physical uint8, portAddress uint16, data []byte) ([]byte, error) {
	if len(data) < 2 || len(data) > 512 || len(data)%2 != 0 {
		return nil, fmt.Errorf("GOLC_ARTNET_DMX_LENGTH_INVALID: length %d must be even and in [2,512]", len(data))
	}
	buf := make([]byte, 18+len(data))
	copy(buf[0:8], []byte("Art-Net\x00"))
	binary.LittleEndian.PutUint16(buf[8:10], opOutputDMX)
	buf[10] = protVerHi
	buf[11] = protVerLo
	buf[12] = seq
	buf[13] = physical
	buf[14] = byte(portAddress & 0xff)
	buf[15] = byte((portAddress >> 8) & 0x7f)
	binary.BigEndian.PutUint16(buf[16:18], uint16(len(data)))
	copy(buf[18:], data)
	return buf, nil
}
```

**Error handling pattern** (`internal/fixture/decode.go` lines 26-43): every rejection returns a `fmt.Errorf("GOLC_{DOMAIN}_{CONDITION}: %v", ...)` string error, never a custom error type — copy this exact convention for `GOLC_ARTNET_*` diagnostics (matches D-11's `{DOMAIN}_{CONDITION}` requirement).

**Validation pattern** (`internal/fixture/decode.go` lines 45-113, `validate`/`rejectOverlappingRanges`): a single exported `Validate`/`validate` pair, called from both the primary constructor path and any secondary path (OFL import), so validation logic is never duplicated — apply the same discipline to ArtPollReply field validation (Security Domain V5: bounds-check every length/count field from the wire before use).

---

### `internal/artnet/channelmap.go` (utility, transform)

**Analog:** `internal/scene/layer.go`

**Imports pattern** (lines 12-16):
```go
package scene

import (
	"github.com/lnorton89/golc/internal/fixture"
)
```
`channelmap.go` mirrors this: `package artnet`, importing `internal/scene` (for `AttributeSet`) and `internal/fixture` (for `CapabilityType`/the new channel-order field).

**Core pure-transform pattern** (lines 30-39, `AttributeSet.Overlay`):
```go
func (a AttributeSet) Overlay(other AttributeSet) AttributeSet {
	merged := make(map[fixture.CapabilityType]float64, len(a.Values)+len(other.Values))
	for capability, value := range a.Values {
		merged[capability] = value
	}
	for capability, value := range other.Values {
		merged[capability] = value
	}
	return AttributeSet{Values: merged}
}
```
Copy this "pure function of inputs, mutates nothing, no I/O, no time dependency" style directly for `Encode(universe, frame) []byte`: walk the fixture's `Mode.Channels` ordered slice (D-16's new field), look up each `CapabilityType`'s normalized value from the instance's `scene.AttributeSet`, and scale [0,1] → [0,255] deterministically. A capability declared in the channel layout but absent from the frame's `AttributeSet` must fail loudly (mirrors D-17's "never silently guess" — do not default to 0 silently; treat as a `GOLC_ARTNET_CHANNEL_VALUE_MISSING`-class diagnostic if the plan decides this case is reachable).

**Fixed-order-not-comparison discipline** (lines 57-84, `ReduceLayers`): note the doc-comment convention "PRECEDENCE HERE IS FIXED ORDER, NOT [X] ARBITRATION" — `channelmap.go` should carry an equivalent doc comment clarifying that channel *order* comes strictly from `Mode.Channels`' declared order, never from `Capabilities` declaration order (RESEARCH.md's explicit Anti-Pattern).

---

### `internal/artnet/worker.go` (service, streaming)

**Analog:** `internal/playback/engine.go`

**Imports pattern** (lines 29-39):
```go
package playback

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/lnorton89/golc/internal/scene"
	"github.com/lnorton89/golc/internal/show"
)
```

**Core non-blocking consumption pattern** (lines 206-212, `CurrentFrame`):
```go
func (e *Engine) CurrentFrame() *Frame {
	return e.activeFrame.Load()
}
```
Worker's `tick` must call this exactly once per its own tick and never block regardless of downstream send speed — this is the literal ARTN-04 requirement engine.go's own doc comment (lines 1-7 of `frame.go`) names this phase as the intended consumer of.

**Ticker-driven lifecycle pattern** (lines 214-244, `Start`/`Stop`):
```go
func (e *Engine) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	e.cancel = cancel

	ticker := time.NewTicker(tickInterval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				e.tick(now)
			}
		}
	}()
}

func (e *Engine) Stop() {
	if e.cancel != nil {
		e.cancel()
	}
}
```
Copy this exact shape for `Worker.Start(ctx)`/`Worker.Stop()`: single goroutine, `context.WithCancel`-derived cancellation, `time.NewTicker` at the worker's own independent cadence (RESEARCH.md Assumption A2 recommends NOT sharing the engine's ticker — a separate `tickInterval`-equivalent constant in `internal/artnet`, defaulting to the same 40Hz value but never literally driven by the Engine's own callback).

**Never-block-on-slow-consumer pattern:** see RESEARCH.md Pattern 3's full `tick` example (already provided, copy near-verbatim) — per-target `go func()` fanout with `SetWriteDeadline`, health recording on both success and error paths, `continue` (never `panic`/`return`) on an encode error for one universe so other universes still get their tick.

---

### `internal/artnet/target.go` (model, CRUD)

**Analog:** `internal/deployment/model.go`

**Imports pattern** (lines 10-15):
```go
package deployment

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)
```

**Core CRUD + validation pattern** (lines 119-132, `ValidateInstanceAddress`):
```go
func ValidateInstanceAddress(instance Instance) error {
	if instance.Universe < 1 {
		return fmt.Errorf("GOLC_DEPLOYMENT_ADDRESS_OUT_OF_RANGE: universe %d is below the minimum universe 1", instance.Universe)
	}
	if instance.Address < 1 || instance.Address > channelsPerUniverse {
		return fmt.Errorf("GOLC_DEPLOYMENT_ADDRESS_OUT_OF_RANGE: address %d is outside the valid 1-%d range for universe %d", instance.Address, channelsPerUniverse, instance.Universe)
	}
	return nil
}
```
`target.go`'s `Target{Universe int, IP net.IP, Port int, Enabled bool}` (D-08 fan-out, D-12 enable/disable) should follow the identical bounds-check-then-diagnostic shape, plus a `ValidateUniqueNames`-equivalent (lines 70-79) rejecting duplicate `(Universe, IP, Port)` triples silently colliding.

**Copy-returning mutation pattern** (lines 103-117, `Activate`): "never mutate the caller's own slice — return a fresh copy" — apply this to any target enable/disable toggle (D-12) exactly as `deployment.Activate` does for the single-active-deployment invariant.

---

### `internal/artnet/interfacemgr.go` (service, event-driven)

**Analog:** RESEARCH.md Pattern 2 (already a complete, near-verbatim-ready example — no in-repo NIC-management precedent exists) + `internal/playback/engine.go`'s `Start(ctx)`/goroutine-per-owner lifecycle for the polling loop shape.

**Core pattern** (RESEARCH.md Pattern 2, lines 253-289 of 04-RESEARCH.md):
```go
func ListCandidateInterfaces() ([]InterfaceInfo, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("GOLC_ARTNET_INTERFACE_ENUM_FAILED: %v", err)
	}
	...
}

func (m *InterfaceManager) pollInterfaceLoss(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := net.InterfaceByIndex(m.pinnedIndex); err != nil {
				m.markLost() // never auto-switches (D-05)
			}
		}
	}
}
```
Pin by `net.Interface.Index`, not `Name` (Pitfall 4) — persist the index as the durable identifier, name for display only.

---

### `internal/artnet/ipc/client.go` + `internal/command/artnet.go` (thin CLI client, request-response)

**Analog:** `internal/command/playback.go` (route/scope declaration, arg parsing, JSON output) + `internal/command/router.go` (`Request`/`Result` shapes to reuse verbatim as the IPC wire format per RESEARCH.md Pattern 5).

**Scope + route declaration pattern** (`internal/command/playback.go` lines 36-65):
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
`internal/command/artnet.go` declares `Scope: "artnet"` and routes `artnet configure`, `artnet status`, `artnet discover`, `artnet target enable/disable`, following the exact same package-level `var _ =` self-registration idiom (CONTEXT D-01/RESEARCH's Recommended Project Structure already names this file).

**Arg parsing pattern** (`internal/command/playback.go` lines 74-101, `parsePlaybackBPMSetArgs`): both `--flag value` and `--flag=value` forms accepted, `GOLC_PLAYBACK_USAGE`-class errors (`ExitCode: 2`) for malformed args vs. domain errors (`ExitCode: 1`) for validated-but-rejected values — copy this exact two-tier exit-code convention for `GOLC_ARTNET_USAGE` vs `GOLC_ARTNET_*` domain errors.

**--json output pattern** (`internal/command/playback.go` lines 340-352, `runPlaybackEvaluate`):
```go
if parsed.json {
	payload, err := strictjson.CanonicalEncode(frame)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_PLAYBACK_FRAME_ENCODE_FAILED: %v\n", err))}
	}
	return Result{Stdout: payload}
}
return Result{Stdout: []byte(fmt.Sprintf("GOLC_PLAYBACK_EVALUATE: ...\n"))}
```
D-02's plain-vs-`--json` health output should copy this exact branch shape for `artnet status`.

**IPC-as-thin-client pattern:** RESEARCH.md Pattern 5 (lines 348-373 of 04-RESEARCH.md) is already a complete example built specifically for this repo's `command.Request`/`command.Result` reuse — copy near-verbatim:
```go
func runArtnetStatus(request command.Request) command.Result {
	conn, err := ipcclient.Dial()
	if err != nil {
		return command.Result{ExitCode: 1, Stderr: []byte(
			"GOLC_ARTNET_DAEMON_UNREACHABLE: is the GOLC background process running?\n")}
	}
	defer conn.Close()
	return ipcclient.Forward(conn, request)
}
```

---

### `internal/fixture/model.go` (additive edit, D-16)

**Analog:** itself, in place — no external analog needed; this is a strictly additive field on the existing `Mode` struct.

**Current shape to extend** (lines 61-67):
```go
type Mode struct {
	Name string `yaml:"name" json:"name" jsonschema:"required,minLength=1,description=Mode name."`
}
```
Add an ordered channel-layout field, e.g. `Channels []CapabilityType` (or a small `ChannelLayout` struct if same-type-occurrence indexing is needed per Pitfall 1's "same-type occurrence index" note) — follow the exact same struct-tag convention (`yaml`/`json`/`jsonschema` triple) already used on every other field in this file (see `Capability` lines 55-59 and `FixtureDefinition` lines 74-80 for the tag-shape precedent). Validation for "no channel layout declared" belongs in `internal/fixture/decode.go`'s `validate()` (lines 68-113) as a new `GOLC_FIXTURE_CHANNEL_LAYOUT_MISSING`-class check — D-17 requires this to be a hard rejection, mirroring the existing `GOLC_FIXTURE_EMPTY`/`GOLC_FIXTURE_MODES_EMPTY` shape at lines 80-89.

### `internal/fixture/ofl/model.go` + `normalize.go` (additive edit, D-16/Pitfall 1)

**Analog:** itself, in place.

**Current shape to extend** (`ofl/model.go` lines 60-67):
```go
// Mode is one OFL operating mode. Its Channels list is intentionally not
// modeled here: fixture.Mode carries only a Name...
type Mode struct {
	Name string `json:"name"`
}
```
Re-add the OFL `channels []string` (channel-key references) field here, then wire `normalize.go` to resolve each key against `AvailableChannels`/`TemplateChannels` and populate the new `fixture.Mode.Channels` field — this doc comment explicitly says today "OFL's per-mode channel-order list has nothing to normalize into," which becomes false once D-16 lands; update the doc comment alongside the field addition so it doesn't go stale (this repo's convention, evidenced throughout, is that doc comments cite the CONTEXT/decision that justifies the shape).

---

## Shared Patterns

### Diagnostic code convention (`GOLC_{DOMAIN}_{CONDITION}`)
**Source:** `internal/deployment/model.go` (`GOLC_DEPLOYMENT_ADDRESS_OUT_OF_RANGE`, `GOLC_DEPLOYMENT_NOT_FOUND`, etc.), `internal/fixture/decode.go` (`GOLC_FIXTURE_*`)
**Apply to:** Every new `internal/artnet/*.go` file — use `GOLC_ARTNET_*` (e.g. `GOLC_ARTNET_DMX_LENGTH_INVALID`, `GOLC_ARTNET_INTERFACE_ENUM_FAILED`, `GOLC_ARTNET_DAEMON_UNREACHABLE`, `GOLC_ARTNET_USAGE`) — always a plain `fmt.Errorf("GOLC_X_Y: %v", ...)`, never a custom error struct/type.

### Non-blocking atomic.Pointer publish/read
**Source:** `internal/playback/engine.go` (`activeFrame atomic.Pointer[Frame]`, `CurrentFrame()`, `Start`/`Stop` ticker lifecycle)
**Apply to:** `worker.go` (reading `Engine.CurrentFrame()`), and any health-state struct that must be readable from a concurrent CLI/IPC-serving goroutine without locking (`health.go`'s per-universe/target status, likely `atomic.Pointer[HealthSnapshot]`).

### Command self-registration (scope + route)
**Source:** `internal/command/router.go` (`MustDeclareRoute`/`MustDeclareScope`/`CommandRegistration`), `internal/command/playback.go` (concrete usage)
**Apply to:** `internal/command/artnet.go` — every `golc artnet ...` route declared via package-level `var _ = MustDeclareRoute(...)`, scope `"artnet"` declared once via `MustDeclareScope`.

### Test-scope marker convention
**Source:** `internal/command/build_test.go` (`TestScopeBuildArgs`), `internal/command/tools_test.go` (`TestScopeToolsUpdate`), `internal/command/test.go` (marker-name derivation: `config-local` → `TestScopeConfigLocal`)
**Apply to:** a new `TestScopeArtnet` marker function declared alongside `internal/command/artnet.go`'s `MustDeclareScope(ScopeRegistration{Scope: "artnet", ...})`, per RESEARCH.md's Wave 0 Gaps checklist item.

### Strict decode + single Validate() entrypoint shared by hand-authored and imported paths
**Source:** `internal/fixture/decode.go` (`Decode`/`Validate`/`validate`), `internal/fixture/ofl/normalize.go` (calls the same `fixture.Validate` rather than re-implementing checks)
**Apply to:** the D-16 channel-layout field's validation — one `Validate` path shared by hand-authored YAML fixtures and OFL-imported fixtures, never two independently-evolving copies (this is explicitly the risk RESEARCH.md's Pitfall 1 flags).

### Copy-don't-mutate slice update
**Source:** `internal/deployment/model.go` (`Activate` — "deployments itself is never mutated", returns a fresh copy)
**Apply to:** `target.go`'s per-target enable/disable toggle (D-12) and any universe/target-list update handler in `internal/command/artnet.go`.

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `internal/artnet/ipc/server.go` | service (daemon transport, named-pipe listener) | request-response | No daemon/IPC listener of any kind exists anywhere in this repo yet (RESEARCH.md: "the long-lived-process-with-IPC-clients model ... is entirely new to this repo"). Build directly against `github.com/microsoft/go-winio`'s `net.Listener`-compatible API (RESEARCH.md Standard Stack) and reuse `command.Request`/`command.Result` as the wire shape (Pattern 5) — there is no existing listener/server file to copy structure from. |
| `internal/artnet/discovery.go` | service (ArtPoll broadcast + collection) | request-response | No prior network-discovery code exists in this repo. Follow RESEARCH.md Pattern 4 directly (already a complete example); its only in-repo tie-in is sharing `packet.go`'s codec for `EncodeArtPoll`/`DecodeArtPollReply`. |
| `internal/artnet/health.go` | model + state machine | event-driven | No prior "liveness/staleness" health-tracking model exists in this repo (closest partial precedent, `internal/playback/clock.go`'s pure validation helpers, only informs the "pure function, deterministic" style — not the actual stale-vs-healthy state-transition shape, which is genuinely new per D-09/D-10). Build directly against D-09 (cadence + staleness) / D-10 (send success/error counts + reachability) as specified in CONTEXT.md — no structural copy source exists. |

## Metadata

**Analog search scope:** `internal/command/`, `internal/playback/`, `internal/deployment/`, `internal/fixture/` (+ `internal/fixture/ofl/`), `internal/scene/`
**Files scanned:** `internal/command/router.go`, `internal/command/playback.go`, `internal/playback/engine.go`, `internal/playback/frame.go`, `internal/deployment/model.go`, `internal/fixture/model.go`, `internal/fixture/decode.go`, `internal/fixture/ofl/model.go`, `internal/scene/layer.go`, plus test-scope-marker grep across `internal/command/*_test.go`
**Pattern extraction date:** 2026-07-22
