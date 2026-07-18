# Architecture Research

**Domain:** Cross-platform desktop lighting control with deterministic Art-Net playback, TypeScript automation, public API, and autonomous LLM control
**Project:** GOLC
**Researched:** 2026-07-17
**Confidence:** MEDIUM

## Executive Recommendation

Build GOLC as a **headless, deterministic Go core wrapped by adapters**, with Wails as only one adapter. Keep the v1 deployment a modular monolith, but give the live engine an explicit real-time-shaped seam: one goroutine owns mutable playback state; it consumes immutable compiled-show snapshots and bounded control mailboxes; it publishes complete universe frame sets to a latest-value mailbox; an independent Art-Net worker performs network I/O. No UI bridge call, event subscriber, database transaction, script, API request, or model invocation may execute on that path or apply backpressure to it.

All control surfaces converge on a versioned application contract. Wails, TypeScript, HTTP/WebSocket clients, and LLM tools decode their wire inputs into the same typed Go commands, call the same dispatcher and safety policy, and query the same revisioned snapshots. They do not call domain objects or the renderer directly. The public contract is distinct from internal domain structs so v1 compatibility can survive domain refactoring.

Treat authoring state and playback state as different models. A mutable `ShowDocument` is validated and compiled off the live path into an immutable `RenderPlan`. The renderer atomically adopts a complete plan at a frame boundary; an invalid or slow compile leaves the last valid plan active. This is the central boundary that lets fixture import, UI editing, scripts, APIs, and agents evolve without destabilizing output.

This architecture provides deterministic semantics and bounded failure propagation, not hard real-time guarantees. Go and general-purpose desktop operating systems can introduce scheduling and timer jitter. GOLC should therefore define and measure frame budgets, use monotonic elapsed time, skip missed intermediate frames instead of building a backlog, and gate releases with per-platform soak and fault tests. Official Go documentation confirms that tickers may drop ticks for slow receivers and that timer resolution varies by OS, which is why show position must be derived from monotonic target time rather than a tick counter ([Go `time`](https://pkg.go.dev/time)).

## Standard Architecture

### System Overview

```text
 CONTROL SURFACES (untrusted or slow; never own show state)
 +-------------+  +---------------+  +---------------+  +---------------+
 | Wails UI    |  | TS Script SDK |  | Public API   |  | LLM planner   |
 +------+------+  +-------+-------+  +-------+-------+  +-------+-------+
        |                 |                  |                  |
        +-----------------+------------------+------------------+
                                  |
                    versioned contracts/v1 DTOs
                                  v
 APPLICATION / SAFETY BOUNDARY
 +-----------------------------------------------------------------------+
 | Adapter auth -> Command Dispatcher -> Policy -> Handler -> Audit/Store |
 |                           |                 |                          |
 | Dedicated manual safety lane          Query Service          Event fanout
 +---------------------------+-----------------+--------------------------+
                             |                 |
                  serialized mutations        +--> revisioned snapshots
                             v
 AUTHORING CORE (not timing critical)
 +-----------------------------------------------------------------------+
 | Fixture catalog -> Patch -> Scenes/Cues/Chases/Effects -> ShowDocument |
 |             semantic validation -> RenderPlan compiler                 |
 +-----------------------------------+-----------------------------------+
                                     | atomic publish after validation
                                     v
 LIVE CORE (single mutable owner; no blocking I/O, locks, DB, or callbacks)
 +-----------------------------------------------------------------------+
 | priority mailbox -> Playback state machine -> pure frame renderer      |
 |                         ^                         |                     |
 |                 immutable RenderPlan       complete FrameSet           |
 +---------------------------------------------------+-------------------+
                                                     | overwrite-latest
                                                     v
 OUTPUT / INFRASTRUCTURE
 +------------------+   +------------------+   +--------------------------+
 | Art-Net worker   |   | ShowStore/backup |   | Metrics/audit diagnostics|
 | discovery + UDP  |   | migrations       |   | and simulators           |
 +------------------+   +------------------+   +--------------------------+

 DEVELOPMENT-ONLY SIDE BOUNDARY
 repository IDs/artifacts <-> traceability manifest <-> Linear sync worker
 (no dependency from the shipped renderer or desktop application)
```

### Deployment Decision

Use one Go process for the v1 application and live core, because the target is a single-operator small-show console and a separate engine process would add IPC, packaging, lifecycle, and upgrade failure modes before they are justified. Preserve a process-ready seam by keeping `engine`, `app`, and `transport` free of Wails imports and by communicating only with typed messages and immutable snapshots.

Run user TypeScript in a separate helper process. A Goja runtime is not goroutine-safe and must have one owning goroutine; it can be interrupted from another goroutine, but interruption alone does not isolate memory exhaustion or a faulty host binding ([goja API](https://pkg.go.dev/github.com/dop251/goja)). A worker process with capability-only IPC, deadlines, kill/restart, and no direct state or renderer references is the appropriate v1 failure boundary. This is process isolation, not a claim of a perfect cross-platform security sandbox; OS-level filesystem/network restrictions require platform-specific hardening.

Split the live engine into its own process only if measured release criteria require surviving a Wails/Go host crash, local model resource contention cannot be bounded, or future redundancy/multi-console scope is accepted. The package boundaries below make that an operational change instead of a domain rewrite.

## Component Responsibilities and Ownership

| Component | Owns | Accepts / publishes | Hard invariants |
|---|---|---|---|
| Domain/state model | Typed IDs, values, show aggregates, invariants, domain errors | Pure values and validated mutations | No Wails, HTTP, SQL, Art-Net, JavaScript, or model-provider types |
| `contracts/v1` | Stable command/query/event DTOs, discriminators, revisions, error codes | JSON/Wails/script/LLM schemas | Versioned separately from internal domain structs; additive change within v1 |
| Command dispatcher | Authentication context, idempotency, optimistic revision checks, handler routing, mutation serialization | `CommandEnvelope` -> `CommandResult` + committed events | Every non-safety mutation goes through one path; no adapter-specific behavior |
| Query service | Immutable authoring/runtime/health read models | Revisioned snapshots and bounded projections | Queries never expose live mutable objects or hold engine locks |
| Event fanout | Subscription cursors and coalescing policy | Committed events / telemetry to Wails, API, scripts | Slow subscribers are dropped or coalesced; every gap is detectable and recoverable by re-query |
| Fixture catalog | Canonical fixture definitions, modes, attributes, provenance, schema version | Validated definitions and catalog snapshots | Imported source data is never the runtime model |
| Fixture import pipeline | Source adapters, raw artifact quarantine, schema and semantic validation, normalization, diff/preview | Source bytes -> draft canonical definition -> explicit commit command | Invalid/ambiguous imports cannot enter catalog; original and digest retained |
| Patch/addressing | Fixture instances, selected modes, logical universes, addresses, grouping | Patch commands -> validated patch snapshot | DMX footprint in 1..512; no overlap unless a future explicit merge feature owns it; protocol address is not a UI string |
| Show authoring | Scenes, cue lists, chases, effects, palettes/presets, metadata | Typed commands -> `ShowDocument` revision | Referential integrity and deterministic ordering; no live clocks |
| RenderPlan compiler | Flattens fixture/patch/show intent into immutable evaluators and slot maps | `ShowDocument` snapshot -> validated `RenderPlan` | Off live path; all-or-nothing publication; plan retains source revision |
| Scheduler/render engine | Playback position, active cue/chase/effect state, manual layers, merge order | Plan snapshots + priority controls -> complete `FrameSet` | One goroutine owns mutable playback state; no blocking I/O, DB, logging callbacks, unbounded allocation, or foreign code |
| Art-Net transport | Node discovery/topology, subscriber targets, per-universe sequence numbers, UDP sockets, send health | Latest complete frame set -> ArtDmx/ArtSync | Network latency cannot backpressure renderer; frame set is kept coherent; protocol mapping isolated |
| Show persistence | SQLite transaction boundary, schema migrations, backup/recovery, autosave, catalog persistence | Committed authoring transactions and load snapshots | Never queried by renderer; failed save blocks later authoring but not the active plan |
| Wails adapter | Generated bridge methods, lifecycle, window events, snapshot subscriptions | Wails calls/events <-> contracts/v1 | Thin; no business rules; frontend state is a cache |
| TypeScript worker | Type checking/transformation, per-script runtime, SDK, debug output, quotas | Script calls over capability IPC -> dispatcher/query service | No direct filesystem/network/state unless permission explicitly grants a host capability; killable without affecting playback |
| External API | Local HTTP + event stream, authentication, rate limits, OpenAPI docs | API v1 DTOs -> dispatcher/query service | Loopback by default; remote access explicit; API version does not leak Go package layout |
| LLM planning/tool adapter | Provider-neutral conversation, planning, structured tool definitions, dry-run/diff | Queries -> proposed commands -> dispatcher | Model text never mutates state; tools are typed, permissioned, rate-limited, auditable, and cancellable |
| Safety/permissions/audit | Principals, grants, live-mode policy, autonomous lease, safety lane, append-only action records | Authorization decisions, override state, audit entries | Manual safety action wins; loss of audit fails closed for autonomous/remote mutation but never blocks manual safety |
| Observability | Frame, transport, command, script, model, store health; diagnostic ring buffers | Non-blocking counters/samples -> health snapshots | Metrics/log sinks cannot block live output; requested output and successful transport are distinct |
| Testing/simulation | Virtual clock, deterministic node simulator, fake stores/adapters, golden frames, fault injection | Test vectors and recorded traces | Core is runnable headlessly without Wails, network hardware, Linear, or a model |
| Traceability sync | Repository-to-Linear mapping and reconciliation | Planning artifacts/manifest <-> Linear GraphQL | Developer tooling only; credentials/cache excluded from repo; offline planning remains usable |

## Core Interfaces and Data Direction

The names below are architectural contracts, not a requirement to implement a reflection-heavy generic bus.

```go
// contracts/v1: stable wire DTOs
type CommandEnvelope struct {
    ID               CommandID
    Kind             CommandKind
    Actor            ActorRef
    ExpectedRevision *uint64
    IdempotencyKey   string
    Payload          json.RawMessage
}

// application boundary
type Dispatcher interface {
    Execute(context.Context, Principal, CommandEnvelope) (CommandResult, error)
}
type Queries interface {
    Snapshot(context.Context, Principal, Query) (RevisionedSnapshot, error)
}
type SafetyController interface {
    ApplyManualOverride(ManualOverride) OverrideReceipt
}

// core seams
type Compiler interface {
    Compile(ShowSnapshot) (*RenderPlan, ValidationReport)
}
type PlanPublisher interface {
    Publish(*RenderPlan) // atomic; engine adopts only at a frame boundary
}
type OutputSink interface {
    Offer(FrameSet) bool // never waits; false means an older pending set was replaced
}
```

Use concrete typed handler registration internally (`Handler[PatchFixture]`, `Handler[CreateScene]`) and an explicit discriminator registry at wire boundaries. Avoid an `any`-valued global event bus: it hides dependencies, defers errors until runtime, and makes API/LLM permission review impossible.

### Command, Query, and Event Semantics

| Message | Delivery and consistency | Consumer rule |
|---|---|---|
| Command | Exactly one serialized decision per `CommandID`/idempotency key; transactional authoring commit; optimistic `ExpectedRevision` | Retry only with same idempotency key; revision conflicts require re-query |
| Command result | Returns accepted/rejected, resulting revision, structured validation errors, and correlation ID | A timeout is not proof of failure; query by command ID before retry |
| Domain event | Emitted only after a committed mutation; monotonic sequence within a show | Audit/store consumer is reliable; UI/API notifications are bounded and may gap |
| Query snapshot | Immutable, authorization-filtered, tagged with authoring revision and runtime frame/plan revision | Client replaces cache; never patches state without checking revisions |
| Telemetry event | Best effort and coalescible; includes timestamp/counters | Do not infer persisted state; re-query health/snapshot after a gap |
| Manual safety command | Dedicated priority lane and atomic override flag; available even if normal dispatcher is saturated | Only an authorized human/local control can clear or supersede it |

## Concurrency and Mutable-State Ownership

### Ownership Model

```text
adapter goroutines
    | bounded requests
    v
application mutation worker ----transaction----> ShowStore writer
    | owns mutable ShowDocument                     |
    | snapshot                                      +--> audit sequence
    v
compiler worker(s) --validated immutable RenderPlan--atomic pointer--+
                                                                 |
manual safety atomic + high-priority mailbox ---------------------+
                                                                 v
                                                       engine goroutine
                                                       owns runtime state
                                                                 |
                                                       complete FrameSet
                                                                 v
                                                    latest-value mailbox
                                                                 |
                                                                 v
                                                     Art-Net I/O worker
```

- **Authoring mutable state:** one application worker serializes mutations per open show. Long operations such as fixture parsing, model calls, script execution, file import, and compilation happen before or after that critical section and return a proposed immutable result. The worker rechecks the expected revision before commit.
- **Playback mutable state:** exactly one engine goroutine owns cue state, chase step, effect phase anchors, playback sequence, and manual layer state. Other goroutines send typed control messages; they never share maps or slices with it.
- **Published state:** read models, topology, render plans, and frame sets are immutable after publication. Use atomic pointer/value swaps for the latest plan and read snapshots; do not mutate an object after storing its pointer.
- **Network state:** the Art-Net worker alone owns sockets, node subscriptions, and packet sequence counters. Discovery publishes an immutable topology snapshot to queries and transport targeting.
- **Script state:** one Goja runtime is owned by one script-worker goroutine. Run user code in the helper process so killing or restarting it cannot corrupt core state.
- **UI state:** the frontend owns only view-local drafts, selection, layout, and a revisioned cache. Saved/show/playback truth remains in Go.

### Deterministic Frame Algorithm

Use a monotonic epoch and target deadlines. A default 40 Hz cadence (25 ms) stays below the Art-Net DMX512 gateway maximum of 44 Hz; cap output at the lowest applicable discovered/static target limit. At deadline `Tn`:

1. Read the manual override atomic first.
2. Drain a bounded number of priority controls, preserving sequence order.
3. Adopt the newest valid plan only at this frame boundary; record its source revision.
4. Compute show position from `Tn - playbackEpoch`, not from the number of received ticker messages.
5. Evaluate pure cue/chase/effect functions into preallocated semantic attribute buffers.
6. Resolve layers in explicit order: safety override, manual control, playback layers; within layers apply attribute-specific HTP/LTP rules using engine sequence numbers, never wall-clock timestamps.
7. Map semantic values through the compiled patch into a complete 512-slot frame per active logical universe.
8. Offer the complete `FrameSet` to the one-element/latest-value transport mailbox and publish counters without waiting.
9. Schedule the next target from the epoch. If late, increment a miss counter and jump to the next future target; never emit a catch-up burst.

The same `Render(plan, runtimeState, monotonicPosition)` function must produce byte-identical frames in simulation. Random effects require an explicit persisted seed. Time zones, wall-clock changes, UI animation frames, and model timestamps must not influence output.

### Failure Isolation and Observable Behavior

| Failure / overload | Required behavior | Observable signal |
|---|---|---|
| UI freezes or event queue is slow | Engine continues; Wails event stream coalesces/drops; UI re-queries snapshot on recovery | Subscriber gap, dropped-event count, stale-cache badge |
| Normal command flood | Per-principal rate limits and bounded queue reject excess; safety lane remains available | Structured `busy/rate_limited`; queue depth |
| Compile fails or runs long | Keep last valid active plan; return validation report; never partially adopt | Active vs draft revision and compile status |
| Persistence write fails | Reject/rollback authoring mutation; continue active plan and output | Store degraded/readonly; unsaved draft clearly marked |
| Renderer misses deadline | Compute current correct state at next future frame; no backlog/catch-up | Frame duration, lateness, misses, worst interval |
| Art-Net send stalls/errors | Transport applies write deadline, overwrites pending stale frame with newest complete set, and reports degraded output; renderer continues | Last successful send per universe, error count, age of transmitted frame |
| Network disappears | Active logical output continues to be rendered; transport health turns red. A remote gateway may hold its last frame; GOLC cannot guarantee blackout across a broken link | Link/socket errors and last-send age, not a false “blackout confirmed” state |
| Script loops, allocates excessively, or crashes | Interrupt deadline; kill/restart helper if unresponsive; reject unfinished call; engine and app remain live | Script termination reason, quota counters, correlation ID |
| LLM/provider hangs or emits invalid tools | Cancel/timeout planner; validate tool schema and policy; no command means no state change | Model/tool trace, rejection reason, autonomous lease state |
| Audit store unavailable | Deny new autonomous/remote mutations; allow local manual programming if explicitly configured and always allow manual safety actions | Fail-closed audit health alarm |
| Wails/Go host exits | v1 output stops; gateways may retain last value. On orderly shutdown optionally send repeated final blackout frames, but UDP delivery cannot be guaranteed | Crash marker and recovery prompt on restart |

## Domain and Runtime Models

### Canonical Domain Model

Keep transport-independent concepts in `internal/domain`:

- `FixtureDefinition`, `FixtureMode`, `AttributeDefinition`, `Capability`, `FineChannel`, `SwitchRule`, and optional pixel/matrix metadata.
- `FixtureInstance`, `PatchAssignment`, `LogicalUniverse`, `Address`, `Group`, and `Selection`.
- `Scene`, `Cue`, `CueList`, `Chase`, `Effect`, `Palette/Preset`, `FadeCurve`, and `TrackingPolicy`.
- `ShowDocument` as an immutable snapshot returned from the authoring aggregate, with stable IDs and a monotonically increasing revision.
- `PlaybackIntent` commands (`Go`, `Back`, `Stop`, `Resume`, `Jump`, `SetMaster`, `SetManualValue`, `ReleaseManual`, `Blackout`) rather than transport packets.

Do not store raw DMX byte arrays as the primary scene/cue model. Store semantic fixture attributes plus provenance/selection expansion, then compile to slot operations. Raw slot overrides can exist later as an explicit escape hatch, isolated from fixture semantics.

### Fixture Import Boundary

```text
source file / OFL / future GDTF / LLM draft
        |
        v
source adapter -> syntax/schema validation -> normalized draft
        |                                      |
        +--> raw bytes + source version + hash  v
                                     semantic validation
                                                 |
                                      preview/diff/warnings
                                                 |
                                      CommitFixtureDefinition command
                                                 |
                                      canonical versioned catalog
```

OFL is a valuable ingestion source, but its own documentation says the internal JSON format may introduce breaking changes and recommends transforming it through a plugin into an application-stable format. It represents modes, channels, fine/switching channels, capability ranges, matrices, and schema/programmatic validation ([OFL fixture format](https://github.com/OpenLightingProject/open-fixture-library/blob/master/docs/fixture-format.md), [OFL plugins](https://open-fixture-library.org/about/plugins)). Therefore:

- Version every source adapter and the canonical GOLC fixture schema independently.
- Retain source URI/file, source schema version, importer version, content digest, warnings, and original bytes for reproducibility.
- Normalize capability units and attribute names through an explicit vocabulary; never let a source-specific string become a renderer switch statement.
- Require a validation report and a preview of footprint/mode changes before replacing an in-use definition. Recompile affected patch/show content and reject destructive ambiguity.
- Route LLM-authored definitions through exactly this draft/validate/diff/commit pipeline. “Generated by a model” is provenance, not a trust exemption.

### Patch and Addressing Boundary

Patch owns all transformations among human-facing addresses, logical universes, fixture mode footprints, and protocol-specific universe identifiers. Its invariants are checked before a show can compile:

- A fixture instance selects exactly one mode and has a stable instance ID independent of its address.
- The selected mode expands to ordered coarse/fine slots; every occupied DMX address is within 1..512.
- Overlap is rejected in v1. If merging is added later, it must be an explicit layer/merge policy, not accidental double patching.
- A mode/definition change produces an impact report for patch footprint and references from scenes/cues/effects.
- Art-Net’s 15-bit Port-Address is created only in the transport mapper. Internal logical-universe IDs are not serialized by bit-twiddling throughout the domain.

### Cue, Chase, and Effect Timing

- Compile cue content into attribute operations and fade segments with explicit start value, target, duration, curve, and tracking/release policy.
- Model a chase as a deterministic step state machine; its position is a function of epoch, step durations, and commands, not repeated sleeping goroutines.
- Model effects as pure, bounded functions of normalized phase, parameters, fixture index, and explicit seed. Precompute fixture ordering and expensive geometry in the compiler.
- Use engine sequence numbers for LTP recency. HTP/LTP belongs to attribute/layer semantics, not to Art-Net network merging.
- A scene is static intent; playback and manual layers are runtime state. Editing a scene produces a new plan but does not retroactively mutate the currently evaluated frame until plan adoption policy says so.

## Art-Net Transport Boundary

The official Art-Net 4 revision 1.4dp (2025-10-23) specifies the current packet rules. ArtDmx uses a 15-bit Port-Address, sequence values 1..255 with zero disabling resequencing, and an even data length of 2..512. A DMX512 gateway advertises a maximum 44 Hz refresh. Current Art-Net requires ArtDmx unicast to subscribed nodes discovered through ArtPoll/ArtPollReply; broadcast ArtDmx is not allowed. ArtSync may follow the universe packets to make a multi-universe set visible together, and nodes return to asynchronous operation after four seconds without ArtSync ([Art-Net 4 specification](https://art-net.org.uk/downloads/art-net.pdf)).

Implement:

- `Transport` as a protocol-neutral interface over complete logical `FrameSet` values, with Art-Net in `internal/transport/artnet`.
- A discovery worker that periodically polls, parses replies, maintains expiry, target refresh limits, and a revisioned topology snapshot. Also support explicitly configured static unicast targets for controlled networks/testing.
- A send worker that increments sequence numbers per universe, sends every universe in a frame set, then optionally sends ArtSync. Never let discovery callbacks or packet logging execute in the renderer.
- A compatibility/configuration report when no subscriber exists. Following the current spec means no ArtDmx transmission to an undiscovered universe; do not silently fall back to broadcast.
- A transport simulator that can delay, drop, reorder, reject malformed packets, expire subscriptions, and validate sequence/length/address/sync behavior.

## Persistence, Migrations, and Recovery

Use a single SQLite show database behind `ShowStore`; keep the public `ShowStore` interface independent from the driver. SQLite supplies transactional recovery, application-controlled `user_version`/`application_id`, WAL snapshot reads, online backup, and integrity checks ([file format](https://www.sqlite.org/fileformat.html), [WAL](https://sqlite.org/wal.html), [backup API](https://www.sqlite.org/backup.html), [PRAGMA checks](https://www.sqlite.org/pragma.html#pragma_integrity_check)).

Recommended rules:

1. One application writer connection owns migrations and authoring commits. Read models come from committed snapshots, not long-lived UI transactions.
2. Set an application ID and explicit schema version. Run ordered forward migrations in one transaction where supported.
3. Before migration, create a consistent backup using the SQLite backup API or `VACUUM INTO`; never copy only the main file while WAL is active. Keep main, `-wal`, and `-shm` together during recovery/copy.
4. On open, validate application ID/version, run a quick/integrity check policy plus foreign-key check, and record a crash marker. If invalid, open read-only when possible, preserve evidence, and offer restore from the last verified backup.
5. A command that changes persisted authoring state commits state, command ID/idempotency record, and durable audit/event metadata transactionally. Publish the new authoring snapshot only after commit.
6. Autosave is command-driven and coalesced outside the engine. Never serialize on every render frame or persist transient effect phase.
7. Loading/migrating a show compiles and validates before activation. A bad store or bad migration cannot replace the active plan.

Do not make the entire domain event-sourced in v1. A transactional current-state model plus append-only audit records provides recovery and accountability without forcing high-frequency playback controls to become a replay protocol.

## Adapters Sharing the Same Capabilities

### Wails

Wails v2 binds exported Go methods and generates JavaScript/TypeScript wrappers and models; frontend calls return Promises. Its lifecycle callbacks provide the context used for runtime operations ([Wails application development](https://wails.io/docs/guides/application-development), [how Wails works](https://wails.io/docs/howdoesitwork)). Bind one thin façade, not domain services:

```go
type DesktopAPI struct {
    dispatch app.Dispatcher
    queries  app.Queries
    safety   app.SafetyController
}

func (a *DesktopAPI) Execute(ctx context.Context, c contractsv1.CommandEnvelope) (contractsv1.CommandResult, error)
func (a *DesktopAPI) Query(ctx context.Context, q contractsv1.Query) (contractsv1.Snapshot, error)
func (a *DesktopAPI) ManualOverride(ctx context.Context, c contractsv1.ManualOverride) (contractsv1.OverrideReceipt, error)
```

Use Wails runtime events only as change/telemetry hints. When the frontend sees a sequence gap or reconnects, it calls `Query`. Do not emit full high-rate DMX frames to the frontend by default; expose sampled/selected universe inspection through a diagnostic query.

### TypeScript Scripting

Use a two-stage pipeline: a TypeScript language-service/type-check step for editor diagnostics and SDK compatibility, then esbuild’s Go API to transform/bundle code for execution. esbuild deliberately transforms TypeScript without full type checking, so transformation success is not validation success ([esbuild TypeScript guidance](https://esbuild.github.io/content-types/#typescript), [esbuild transform API](https://esbuild.github.io/api/#transform)).

The helper process hosts one runtime per active script or a strictly scheduled pool, each owned by one goroutine. Its SDK exposes asynchronous capabilities such as `query`, `execute`, `subscribe`, `sleepShowTime`, and logging. It does not expose Go pointers, arbitrary host reflection, raw sockets, or direct filesystem access. Each call carries the script principal, correlation ID, permissions, timeout, and expected revision into the same application dispatcher. Subscription buffers are bounded; a gap causes SDK re-query.

### External API

Expose versioned REST commands/queries plus SSE or WebSocket events under `/api/v1`. Keep OpenAPI operation IDs and schemas in `api/openapi`, with compatibility/golden tests mapping every operation to `contracts/v1`. OpenAPI has explicit published specification versions and machine-readable schemas ([OpenAPI specification](https://spec.openapis.org/oas/)).

- Bind loopback only by default. Remote binding requires explicit enablement, authenticated principals, origin policy, TLS strategy, and visible live-mode status.
- Separate `command accepted/committed` from event-stream telemetry. Preserve idempotency keys and correlation IDs across retries.
- Keep transport DTOs versioned. A future `/api/v2` may map to the same current application handlers while `/api/v1` remains supported.
- Generate the TypeScript SDK and LLM tool schemas from the same versioned contract definitions; conformance tests, not convention, prevent drift.

### LLM Planning and Tool Adapter

The provider-neutral model wrapper is outside the trust boundary. It may read authorization-filtered snapshots and produce plans, fixture drafts, or structured tool calls. Only the deterministic tool adapter can submit commands.

```text
prompt + query snapshot -> provider-neutral LLM -> proposed typed tool batch
       -> schema validation -> permission/policy -> dry-run/impact report
       -> optional approval or active autonomy lease -> dispatcher
       -> committed results/events -> audit + next model observation
```

Use short-lived autonomy leases scoped by capabilities, show, maximum action rate, and expiry. A manual edit/override revokes or pauses the lease immediately. Large plans execute as validated batches only where atomicity is meaningful; otherwise expose explicit partial progress and compensation, never pretend a multi-command sequence was atomic. Fixture-definition authoring goes through the same importer validation boundary. Network/model latency is never represented as a playback clock.

## Permissions, Audit, and Manual Override

Authorize in the application layer using a `Principal` and explicit capabilities such as `show.read`, `show.edit`, `patch.edit`, `playback.control`, `fixture.import`, `fixture.commit`, `script.manage`, `autonomy.enable`, and `safety.override`. The adapter establishes the principal; handlers enforce capabilities and live-mode policies.

Manual override is a typed application capability with a dedicated priority path, not a privileged UI mutation. Recommended precedence:

1. Local manual safety (`blackout`, `stop all`, `revoke autonomy`) via atomic state/priority mailbox.
2. Local manual playback/programmer layer.
3. Authorized remote/script/LLM playback layer.
4. Programmed cue/chase/effect layers.

Only an authorized local human action clears a safety override. Record actor, adapter, command kind, sanitized parameters/diff, policy decision, result, affected revisions, timestamps, and correlation IDs. Never log model secrets or raw credentials. Because audit is part of the autonomy safety contract, audit failure denies autonomous/remote mutations while preserving local safety control and ongoing output.

## Observability

Expose a revisioned `HealthSnapshot` and bounded diagnostic history. Minimum metrics:

- Engine: target rate, actual interval histogram, render duration, deadline misses, maximum lateness, active plan revision, adoption failures, allocations if measured.
- Output: rendered frame revision, offered/replaced frame sets, last successful send per universe/target, packet rate, sequence, socket errors, discovery age, subscriber count, optional ArtSync state.
- Commands: accepted/rejected/duplicate/conflict counts, queue depth, handler latency, actor/adapter category, safety-lane latency.
- Store: commit latency/failures, schema version, last backup/check result, recovery mode.
- Scripts/LLMs: active workers/leases, timeouts, kills, tool validation failures, command rate, provider latency and cancellation.
- Subscribers: event gaps, dropped/coalesced messages, reconnects.

Write metrics through atomics or bounded non-blocking buffers. Console/file exporters consume outside the engine. A diagnostic UI must distinguish **desired/rendered**, **offered to transport**, and **successfully sent**; none proves that a physical luminaire changed.

## Testing and Simulation Architecture

The core must be fully testable without Wails or physical nodes.

| Test layer | Required tests | Proof supplied |
|---|---|---|
| Domain | Table/property tests for units, fixture capabilities, patch footprints, overlaps, cue references | Invariants reject invalid shows before compilation |
| Contract | Golden JSON, OpenAPI validation, generated TS compile, version compatibility, command registry coverage | UI/script/API/LLM use the same typed capability set |
| Compiler/renderer | Fake monotonic clock, golden universe frames, random explicit seeds, plan-boundary adoption, missed-deadline simulation | Same inputs/time produce byte-identical frames |
| Scheduler concurrency | Race detector, queue saturation, slow subscribers, plan swaps, override latency | Single ownership and no live-path backpressure |
| Art-Net | In-process UDP node, packet corpus, sequence wrap, length/address validation, discovery expiry, rate cap, ArtSync, drop/reorder/failure injection | Transport conforms independently of renderer |
| Persistence | Migration fixtures from every schema version, crash/rollback, backup/restore, WAL handling, corrupt/truncated DB, idempotent retry | Show data survives upgrade and recoverable failure |
| Fixtures/import | Official-source examples, malformed/fuzzed inputs, ambiguous switching/fine channels, provenance round-trip | External/LLM data cannot bypass canonical validation |
| Adapter | Wails façade, API auth/rate/idempotency, script process kill, LLM invalid tool batches | Adapter failure cannot mutate outside dispatcher |
| End-to-end | Headless show -> simulated time -> Art-Net node; UI/API/script/LLM issue equivalent commands and compare results | Complete workflow and behavioral convergence |
| Release soak | Hours-long playback on each supported OS under UI redraw, autosave, script timeout, API flood, and LLM activity | Measured jitter and failure behavior meet release budget |

Use Go’s race detector and native fuzzing for concurrency and parser boundaries ([race detector](https://go.dev/doc/articles/race_detector), [Go fuzzing](https://go.dev/doc/security/fuzz/)). Define release thresholds during engine planning; examples should include zero unbounded queue growth, zero data races, no output-loop blockage, and a documented acceptable miss/jitter envelope on every supported OS.

## Project Traceability Boundary

Linear is the delivery-status system of record, but repository planning artifacts must remain readable and navigable offline. Implement traceability as a separate developer CLI/CI tool, not a runtime application service.

### Stable Mapping

Version-control a credential-free manifest such as `.planning/LINEAR-MAP.json`:

```json
{
  "schema": 1,
  "workspace": "golc",
  "entities": [
    {"repoId":"M1", "linearType":"project", "linearUuid":"...", "url":"..."},
    {"repoId":"PHASE-01", "linearType":"projectMilestone", "linearUuid":"...", "url":"..."},
    {"repoId":"REQ-PLAY-001", "linearType":"issue", "linearUuid":"...", "identifier":"GOL-12", "url":"..."},
    {"repoId":"01-02-TASK-03", "linearType":"issue", "linearUuid":"...", "identifier":"GOL-31", "url":"..."}
  ]
}
```

Recommended mapping:

- Repository milestone -> one Linear Project.
- Roadmap phase ID -> one Linear Project Milestone within that project.
- Requirement ID -> durable Linear parent/spec issue linked to its phase milestone.
- Plan/task ID -> Linear implementation issue or sub-issue related to the requirement.
- Repository artifacts include Linear URLs/UUIDs; Linear descriptions/attachments include stable repository IDs and permalinks.

Linear’s GraphQL API exposes model UUIDs and supports query/mutation; issues can be assigned to project milestones. GraphQL responses may partially succeed with HTTP 200, so the sync worker must inspect the `errors` array ([Linear GraphQL](https://linear.app/developers/graphql), [project milestones](https://linear.app/docs/project-milestones)). Linear discourages polling and provides signed, delivery-ID webhooks, but a local offline-first CLI should reconcile explicitly or in CI; a hosted webhook consumer is optional later ([Linear webhooks](https://linear.app/developers/webhooks), [rate limits](https://linear.app/developers/rate-limiting)).

### Sync Rules

1. Local stable IDs are never regenerated when titles change. Linear UUIDs, not human issue identifiers, are the durable remote keys.
2. `plan-sync push` creates/reconciles missing remote entities idempotently and records mappings only after confirmed success. Partial errors remain pending.
3. Delivery status flows from Linear into a generated, gitignored cache or explicit status report; it does not rewrite historical planning prose automatically.
4. Requirements and phase definitions remain available offline. If Linear is down, planning continues and the next sync reports divergence.
5. Credentials live in environment/OS secret storage. Webhook deliveries are signature-verified and de-duplicated by delivery UUID if a hosted consumer is introduced.
6. CI checks that every active requirement, phase, and implementation plan has a unique mapping and that no two local IDs claim the same Linear UUID.

## Recommended Project Structure

```text
cmd/
  golc/                    # Wails composition root only
  golc-script-host/        # killable TypeScript/Goja helper process
  golc-headless/           # simulation, soak, and future engine-process seam
  golc-plan-sync/          # repository <-> Linear developer tool
internal/
  domain/
    fixture/               # canonical fixture vocabulary and definitions
    patch/                 # instances, modes, addresses, validation
    show/                  # scenes, cues, chases, effects, aggregates
    playback/              # intents and pure timing/value types
  contracts/
    v1/                    # stable command/query/event DTOs and error codes
  app/
    dispatch/              # typed handlers, serialization, idempotency
    query/                 # revisioned read models
    events/                # reliable committed events + bounded fanout
    safety/                # permissions, autonomy lease, manual override
  compile/                 # ShowSnapshot -> immutable RenderPlan
  engine/                  # scheduler, state machine, renderer, merge
  transport/
    transport.go           # protocol-neutral frame-set sink
    artnet/                # discovery, packet codec, UDP sender, health
  fixtureio/               # OFL/source import adapters and provenance
  persistence/
    sqlite/                # store, migrations, backups, recovery
  adapters/
    wails/                 # thin DesktopAPI and event bridge
    httpapi/               # REST/event stream/auth/rate limits
    scriptipc/             # capability IPC client/server
    llm/                   # provider wrapper, planner, typed tool adapter
  observability/           # counters, ring buffers, health snapshots
api/
  openapi/                 # public API v1 source/golden contract
  typescript/              # generated public/script SDK
frontend/                  # Wails web UI; no core/domain logic
testkit/
  clock/                   # virtual monotonic clock
  artnetnode/              # simulated gateway/subscriber
  stores/                  # fake/fault-injected persistence
  scenarios/               # headless show and fault fixtures
.planning/
  LINEAR-MAP.json          # credential-free durable traceability mapping
```

The dependency direction is inward: adapters and infrastructure depend on `contracts/app/domain`; `domain`, `compile`, and `engine` never import Wails, SQLite, HTTP, JavaScript, LLM, or Linear packages. Enforce this with package-level architecture tests or a dependency linter in CI.

## Dependency-Driven Build Order

| Order | Phase boundary | Build | Must precede / exit proof |
|---:|---|---|---|
| 0 | Project foundation and traceability | Stable repo requirement/phase/task IDs, Linear project/milestone/issues, mapping manifest, sync/reconcile CLI skeleton, CI uniqueness check | Required from inception; independent of product core; proves offline artifacts and Linear links coexist |
| 1 | Domain and application contracts | Typed IDs/values/errors, `contracts/v1`, dispatcher/query interfaces, principal/permission model, idempotency/revision semantics, virtual clock, architecture dependency checks | Every adapter and later phase depends on these names and invariants; prove identical command vectors through in-memory dispatcher |
| 2 | Fixture catalog and patch | Canonical fixture model, OFL/manual/LLM-draft ingestion pipeline, semantic validation, modes/fine channels, patch/address invariants and impact reports | Cue authoring and compilation require stable semantic attributes and slot maps; prove malformed/overlapping fixtures cannot commit |
| 3 | Show authoring and deterministic engine | Scenes, cue lists, chases, effects, RenderPlan compiler, single-owner playback state, HTP/LTP layers, manual safety lane, virtual-time golden renderer | Art-Net needs complete frames, and AI/UI must not define semantics; prove byte-identical frames, plan boundary swaps, override priority, missed-frame policy |
| 4 | Art-Net output and simulation | Packet codec, discovery, static targets, sequence/rate/address rules, latest-frame mailbox, ArtSync option, node simulator, output health | Establishes the first real vertical proof before UI complexity; prove headless patch -> cue -> simulated/physical node under saturation |
| 5 | Persistence, migrations, and recovery | SQLite store, transactional command/audit records, backups, migration corpus, autosave, corrupt/crash recovery | UI authoring must not be built on disposable state; prove every schema fixture migrates and failed writes never replace active plan |
| 6 | Wails operator workflow | Thin façade, generated TS types, revisioned frontend cache, patch/program/playback screens, bounded events, diagnostics and local manual override | Uses proven headless capabilities; prove UI freeze/reload does not stall engine and snapshot recovery handles gaps |
| 7 | Stable external API | OpenAPI v1, auth, loopback/remote policy, rate limits, idempotency, event stream, generated SDK, contract tests | Freezes the public surface only after core semantics have real use; prove API and Wails commands return equivalent results |
| 8 | TypeScript scripting | Editor/type diagnostics, transform pipeline, helper process, capability SDK, quotas, debug/audit, kill/restart | Depends on stable API/application contracts and recovery; prove infinite/crashing scripts cannot affect frame cadence and script/UI equivalence |
| 9 | LLM fixture and control integration | Provider-neutral wrapper, structured tools, plan/dry-run/diff, fixture drafts, autonomy lease, policy/audit/manual revocation | Last mutating adapter because it needs complete, tested commands, permissions, observability, and recovery; prove invalid/hung model behavior makes no uncontrolled change |
| 10 | Cross-platform release hardening | Long soak/fault tests, timer/jitter budgets, network-loss policy, packaging/signing, upgrade/restore exercises, operator runbooks | Converts architectural claims into measured release evidence on every supported OS |

**Ordering rationale:** deterministic semantics and a headless vertical Art-Net slice come before UI, scripting, or AI. Persistence precedes serious desktop authoring so save/recovery behavior is designed rather than bolted on. The external API is stabilized after conventional workflow semantics have been exercised, then becomes the contract base for scripting and model tools. LLM autonomy is deliberately last because it amplifies every missing validation, permission, recovery, and observability gap.

## Architectural Patterns

### Pattern 1: Hexagonal Core with One Application Capability Layer

**What:** Domain, compiler, and engine sit inside; Wails, HTTP, script IPC, LLM providers, Art-Net, SQLite, and Linear are adapters.

**When to use:** Always. The project explicitly requires behavioral equivalence across control surfaces and protocol/platform isolation.

**Trade-offs:** More mapping code and contract tests, but platform/provider changes cannot redefine show behavior.

### Pattern 2: Mutable Authoring, Immutable Compiled Playback

**What:** Commands update a validated authoring aggregate; compiler workers create immutable plans; the engine swaps a complete pointer at a frame boundary.

**When to use:** Any edit that can affect patch, cues, effects, or fixture semantics.

**Trade-offs:** Compilation and impact reporting add latency, but invalid drafts and half-applied edits cannot corrupt live state.

### Pattern 3: Single Writer plus Latest-Value Mailboxes

**What:** One owner per mutable state machine; downstream telemetry/output queues are bounded, coalesced, or overwrite-latest.

**When to use:** Playback, transport handoff, UI events, and health publication.

**Trade-offs:** Consumers may skip intermediate states. Revisions and re-query are mandatory, and command/audit paths remain durable rather than lossy.

### Pattern 4: Capability-Based Untrusted Adapters

**What:** Script and model code see typed capabilities, not Go objects or arbitrary host access. Each call is authenticated, scoped, timed, validated, and audited.

**When to use:** TypeScript, external API, and every LLM tool.

**Trade-offs:** Less ad hoc extensibility, but a stable SDK and enforceable safety boundary.

## Anti-Patterns

### UI-Owned or Shared Mutable Show State

**What people do:** Keep the canonical show in a frontend store or share maps/slices among Wails calls and the renderer.

**Why it is wrong:** UI lifecycle, stale caches, races, and bridge behavior become playback semantics.

**Do this instead:** Go-owned authoring aggregate, revisioned snapshots, immutable plans, and single engine ownership.

### Calling Scripts, Models, Logs, or SQL from a Frame

**What people do:** Evaluate script callbacks/effects, ask an agent, persist state, or synchronously log while rendering.

**Why it is wrong:** Every slow/untrusted dependency becomes an output deadline dependency.

**Do this instead:** Scripts/models submit future typed commands; persistence and observability run outside; renderer executes only precompiled bounded functions.

### Unbounded Queues to “Avoid Dropping Data”

**What people do:** Buffer every UI event or every pending frame.

**Why it is wrong:** Latency and memory grow until consumers see obsolete states or the process fails.

**Do this instead:** Durable commands/audit, revisioned snapshots, coalesced telemetry, and overwrite-latest frame sets.

### One DTO Type for Domain, Database, Wails, API, and LLM

**What people do:** Export internal structs everywhere to avoid mapping.

**Why it is wrong:** Schema migration, API compatibility, frontend generation, and model-tool safety become coupled to internal refactors.

**Do this instead:** Internal domain types plus explicit versioned contract and persistence mappings.

### Full Event Sourcing Before Playback Semantics Exist

**What people do:** Make every fader/effect/playback transition a replay event and build projections first.

**Why it is wrong:** It increases recovery/versioning complexity and can confuse audit intent with high-rate runtime state.

**Do this instead:** Transactional current state, stable commands, committed domain events, and an append-only audit trail; keep ephemeral frame state ephemeral.

### Treating Go/Wails as Hard Real-Time

**What people do:** Assume a goroutine/ticker guarantees deadlines.

**Why it is wrong:** desktop scheduling, timer resolution, GC, and load introduce jitter.

**Do this instead:** monotonic deadline math, no blocking path, measured budgets, skip-not-backlog behavior, and per-OS soak gates.

## Scaling Considerations

This is a single-user desktop system; scale is universes, fixtures, active effects, and adapter load, not SaaS user count.

| Scale | Architecture adjustment |
|---|---|
| Small-show v1: roughly 1-8 universes, hundreds of fixtures/attributes | Modular monolith, 40 Hz engine, preallocated buffers, one Art-Net worker, SQLite, bounded subscribers |
| Larger local rigs: tens/hundreds of universes | Profile compiler/render allocations, shard frame encoding by immutable universe partitions if measured, batch UDP sends where portable, topology-aware rate caps; preserve one logical playback owner |
| Distributed/redundant console | New milestone: external engine process, replicated clock/state protocol, failover/fencing, deterministic command log, protocol redundancy. Do not infer this from v1 seams alone |

Optimize compiler and renderer work per active attribute/universe first. Do not split into network microservices for v1; process/network boundaries add failure and time-consistency problems the project does not need.

## Integration Points

### External Services and Processes

| Service/process | Integration pattern | Failure policy |
|---|---|---|
| Art-Net nodes | UDP discovery + unicast ArtDmx; optional ArtSync; static unicast configuration | Non-blocking transport degradation; active render continues; visible last-send age |
| Linear | Separate GraphQL CLI/CI reconciler, UUID mapping, optional signed webhooks later | Offline planning continues; partial errors/rate limits retry outside product runtime |
| Hosted/local LLMs | Provider-neutral async adapter with structured tools and cancellation | Timeout/cancel/reject; no direct state access |
| Script helper | Local authenticated capability IPC, process supervision, deadline/kill | Restart worker; reject unfinished command; output unaffected |
| SQLite | Single writer, short transactions, backup/migration/recovery boundary | Roll back authoring; preserve current active plan |

### Internal Boundaries

| Boundary | Communication | Direction |
|---|---|---|
| Adapter -> application | contracts/v1 command/query DTOs | Inward only |
| Application -> domain/store | typed handlers and transaction ports | Inward mutation, outward result |
| Authoring -> compiler | immutable show snapshot | Toward playback only |
| Compiler -> engine | atomic immutable RenderPlan publication | Toward engine only |
| Safety -> engine | atomic override + bounded priority mailbox | Toward engine only |
| Engine -> transport | complete latest-value FrameSet | Toward I/O only |
| Engine/transport -> query/observability | atomic counters and immutable health snapshots | Outward only; no callback |
| Linear sync -> repository/Linear | developer-tool reconciliation | No import/dependency into product core |

## Confidence Assessment

| Area | Confidence | Basis / remaining uncertainty |
|---|---|---|
| Wails adapter boundary | MEDIUM | Current Wails v2 docs retrieved through Context7 confirm generated bindings, Promise calls, models, and lifecycle; exact frontend event throughput must be measured |
| Engine ownership and snapshots | MEDIUM | Strong Go concurrency/time primitives and established deterministic-systems pattern; release jitter budgets require implementation benchmarks on supported OSes |
| Art-Net transport | MEDIUM | Current official Art-Net 4 revision 1.4dp directly supports packet/address/rate/unicast/sync claims; real device interoperability still needs hardware matrix testing |
| Fixture ingestion | MEDIUM | Official OFL model and compatibility warning are clear; GOLC canonical vocabulary and future GDTF coverage need phase-specific design |
| Persistence/recovery | MEDIUM | Official SQLite transaction/WAL/backup/integrity mechanisms support the boundary; Go driver and exact durability settings belong in stack/implementation research |
| TypeScript isolation | MEDIUM | Context7/official goja and esbuild docs support runtime ownership, interruption, and transformation limits; cross-platform OS sandbox strength remains a hardening research item |
| API/LLM convergence | MEDIUM | OpenAPI and shared-dispatcher pattern are well supported; final command granularity and v1 compatibility policy need conventional workflow prototyping |
| Linear traceability | MEDIUM | Current official GraphQL, milestone, webhook, and rate-limit docs support stable UUID mapping and async reconciliation; exact workspace taxonomy must be chosen during setup |

## Phase-Specific Research Flags

- **Deterministic engine phase:** define measured frame/jitter/override budgets per OS, effect semantics, cue tracking, pause/resume, and plan-adoption behavior before implementation.
- **Fixture phase:** design the canonical attribute/capability vocabulary and evaluate GDTF/OFL/QLC+ import coverage with a representative fixture corpus.
- **Art-Net phase:** validate current unicast discovery behavior against the actual nodes first users own; document static-target and compatibility UX without violating the current spec by default.
- **Persistence phase:** select the Go SQLite driver, durability pragmas, backup retention, show-file packaging/export, and migration rollback/support policy.
- **Script phase:** threat-model process IPC, module imports, memory/CPU limits, filesystem/network capabilities, debugger protocol, and platform-specific sandboxing.
- **LLM phase:** select the provider-neutral wrapper only after tool contracts exist; research structured-output portability, cancellation, local-model deployment, context limits, and audit redaction.
- **Release hardening:** general-purpose desktop OSes are not hard real-time; measured soak criteria are a release blocker, not an optional optimization.

## Sources

All library/framework documentation questions were routed through the required research-plan seam. Wails, goja, and esbuild were resolved and queried through Context7 first. Web findings below were restricted to primary official specifications, documentation, or official repositories. The confidence-classification seam rates Context7 as MEDIUM and cross-checked web search as MEDIUM, so no source-derived claim is tagged HIGH.

- **[MEDIUM, Context7 + official]** Wails v2: [How Wails works](https://wails.io/docs/howdoesitwork), [Application development](https://wails.io/docs/guides/application-development), [Runtime events](https://wails.io/docs/reference/runtime/events)
- **[MEDIUM, Context7 + official repository/API]** goja: [official repository](https://github.com/dop251/goja), [`Runtime.Interrupt`](https://pkg.go.dev/github.com/dop251/goja#Runtime.Interrupt)
- **[MEDIUM, Context7 + official]** esbuild: [TypeScript content type](https://esbuild.github.io/content-types/#typescript), [Transform API](https://esbuild.github.io/api/#transform)
- **[MEDIUM, official specification; revision 1.4dp dated 2025-10-23]** [Art-Net 4 specification](https://art-net.org.uk/downloads/art-net.pdf)
- **[MEDIUM, official repository/site]** Open Fixture Library: [fixture format](https://github.com/OpenLightingProject/open-fixture-library/blob/master/docs/fixture-format.md), [plugin model](https://open-fixture-library.org/about/plugins), [OFL JSON compatibility warning](https://open-fixture-library.org/about/plugins/ofl)
- **[MEDIUM, official]** Go: [`time` and monotonic clocks/tickers](https://pkg.go.dev/time), [`sync/atomic`](https://pkg.go.dev/sync/atomic), [race detector](https://go.dev/doc/articles/race_detector), [fuzzing](https://go.dev/doc/security/fuzz/)
- **[MEDIUM, official]** SQLite: [database file format](https://www.sqlite.org/fileformat.html), [WAL](https://sqlite.org/wal.html), [online backup](https://www.sqlite.org/backup.html), [PRAGMA checks](https://www.sqlite.org/pragma.html#pragma_integrity_check), [isolation](https://www.sqlite.org/isolation.html)
- **[MEDIUM, official]** Linear: [GraphQL API](https://linear.app/developers/graphql), [project milestones](https://linear.app/docs/project-milestones), [webhooks](https://linear.app/developers/webhooks), [rate limits](https://linear.app/developers/rate-limiting), [attachments/idempotent URLs](https://linear.app/developers/attachments)
- **[MEDIUM, official specification]** [OpenAPI Specification](https://spec.openapis.org/oas/)

---
*Architecture research for: GOLC greenfield lighting-control system*
*Researched: 2026-07-17*
