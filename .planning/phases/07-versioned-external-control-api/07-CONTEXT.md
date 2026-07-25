# Phase 7: Versioned External Control API - Context

**Gathered:** 2026-07-24
**Status:** Ready for planning

<domain>
## Phase Boundary

External programs (and, later, TypeScript automation and LLM agents from Phases 8-9) can inspect and control every public GOLC capability through a secure, versioned, documented API that behaves like the desktop application — reusing `internal/command`'s existing `Request{Route,Args}`/`Result` route registry as the single source of truth for what capabilities exist, not a second, independently-maintained command surface.

This phase clarifies HOW to expose that registry externally (transport, versioning, auth, streaming, mutation semantics, audit). It does not add new domain capabilities beyond what Phases 1-6 already implemented and `internal/command` already routes.

</domain>

<decisions>
## Implementation Decisions

### Transport & versioning
- **D-01:** Resource-oriented REST (e.g. `GET /v1/shows/{id}`, `POST /v1/scenes`), not a thin JSON-RPC passthrough of `internal/command`'s `Request{Route,Args}` shape. Requires a real mapping/translation layer in front of the existing route registry — internal/command remains the single execution authority underneath.
- **D-02:** URL-path versioning (`/v1/...`). Breaking changes get a new path prefix (`/v2/...`) served alongside `/v1/` during a deprecation window (API-02's compatibility/deprecation guidance).
- **D-03:** OpenAPI contract is generated from Go code (handler/struct definitions), not hand-authored spec-first. Mirrors the existing `internal/contracts` pattern already used for JSON-Schema generation (fixtures, config concerns) — spec always matches implementation by construction.
- **D-04:** HTTP routing uses a router library (chi or gorilla), not stdlib `net/http` alone, despite the project's general minimal-dependency bias — the user explicitly chose ergonomics for route/middleware grouping over avoiding a new pinned dependency. This new dependency must be checksum-pinned the same way `config/toolchain.toml` pins everything else offline-buildable in this repo.

### Auth & remote access
- **D-05:** Clients authenticate with per-token scoped API keys with expiry (not static long-lived bearer tokens). Keys are minted through a dedicated route/CLI, stored hashed, individually revocable.
- **D-06:** Remote-access enablement is a config flag under `config/*.toml` (following the existing per-concern TOML pattern: `config/runtime.toml`, etc., with the same provenance/explain tooling `internal/projectconfig` already provides) — not a CLI startup flag. Loopback stays open regardless of this flag; the flag only governs binding beyond loopback.
- **D-07:** The API server runs inside the existing supervised `golc-project` daemon process (the same one `internal/wails` already spawns/dials over named pipe or Unix socket, with Phase 6's orphan-cleanup and supervised-lifecycle already built) — not a separate process/binary.
- **D-08:** API-key scopes are coarse domain scopes (e.g. `playback`, `authoring`, `admin`), not per-domain read/write splits. Mirrors ROADMAP.md/REQUIREMENTS.md's existing domain grouping (`PLAY-*`, `POOL-*`, etc.).

### Event streaming & gap recovery
- **D-09:** One global SSE stream (`GET /v1/events`) emitting every domain's changes with a `type` field, each event carrying a monotonic revision number — not separate per-domain streams.
- **D-10:** Gap recovery uses SSE's standard `Last-Event-ID` header plus a bounded server-side revision replay buffer; on reconnect, missed events within the buffer window are replayed, and a buffer overflow (satisfying API-03's "recover from an event gap by re-querying authoritative state") tells the client to fall back to a full re-fetch via the REST resource endpoints before resuming the stream.
- **D-11:** All connected, authenticated clients see the full event stream regardless of their key's domain scopes — domain scopes only gate REST mutations, not stream visibility. (A narrowly-scoped playback-only integration will also observe authoring/config events.)
- **D-12:** Any valid, non-expired API key can open the event stream — no separate "streaming" capability/flag is required on top of the coarse domain scopes from D-08.

### Mutation semantics & audit
- **D-13:** Expected-revision concurrency uses the standard HTTP conditional-request pattern: `If-Match: "<revision>"` on mutating requests, `412 Precondition Failed` on mismatch — not a body-embedded `expected_revision` field.
- **D-14:** Dry-run previews use a `?dry_run=true` query parameter on the same mutating endpoint, returning the would-be impact/result without applying it — not a separate `/preview` sub-resource. Mirrors the desktop UI's existing impact-review pattern (fixture pool updates, deployment changes).
- **D-15:** Atomic batches go through one generic `/v1/batch` endpoint accepting an ordered list of sub-requests, applied as a single atomic transaction (all-or-nothing) — not per-resource bulk endpoints designed individually.
- **D-16:** Audit records (actor, source, correlation, outcome, redacted details per API-06) are written to a new table in the existing `.golc` SQLite store (Phase 5's durable storage engine — migrations, integrity checks, lock-retry already built), not a separate append-only log file. Audit records travel with the show file.

### Claude's Discretion
None — the user answered all 16 questions directly; no "you decide" selections were made in this discussion.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements this phase must satisfy
- `.planning/REQUIREMENTS.md` §"Public API" (API-01 through API-06) — the six locked requirements this phase's implementation decisions above were made to satisfy.
- `.planning/ROADMAP.md` §"Phase 7: Versioned External Control API" — goal, dependencies (Phase 6), success criteria.

### Existing command/routing authority this phase must reuse, not duplicate
- `internal/command/router.go` — `Request`/`Result`/`CommandRegistry.Execute` — the single execution authority every new REST handler must call into.
- `internal/command/test.go`, `internal/command/build.go` — examples of the self-registration pattern (`MustDeclareRoute`) new API-adjacent routes should follow if any new internal/command routes are needed to back REST handlers.

### Existing daemon/process/config patterns this phase builds on
- `internal/wails/app.go` — the supervised-daemon-child spawn/dial/orphan-cleanup pattern (Phase 6) the API server will run alongside inside the same daemon process (D-07).
- `internal/bootstrap/engine.go` (`PlatformExecutablePath`, `PlatformKey`) — platform-aware path resolution convention already fixed once this session (see `internal/wails/app.go`'s daemon-executable fix) — any new API-related executable path resolution must follow the same pattern, not reintroduce a hardcoded relative path.
- `internal/projectconfig` — the concern/key registry, resolved-JSON inspection, and provenance/explain API a new `api` config concern (D-06) should be added to, following `config/runtime.toml`'s existing shape.
- `internal/contracts` — the existing Go-to-JSON-Schema generation pattern (D-03: OpenAPI generated from Go, not hand-authored).

### Storage authority this phase builds on
- `.planning/phases/05-durable-shows-and-recovery/05-CONTEXT.md` and `internal/show` — the `.golc` SQLite store's existing migration/integrity/retry patterns the new audit table (D-16) must follow, not reimplement.

No external (non-repository) specs apply — all requirements are captured in `.planning/REQUIREMENTS.md` and the decisions above.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/command`'s `Request{Route,Args}`/`Result` registry — every REST handler this phase adds is a thin adapter in front of this, not a parallel command implementation.
- `internal/wails`'s supervised-daemon lifecycle (spawn, dial, orphan cleanup, ordered start/stop) — the API server's own start/stop should slot into this same lifecycle (D-07), not invent a second one.
- `internal/show`'s SQLite store (migrations, integrity, retry-on-lock) — reused directly for the new audit table (D-16).
- `internal/contracts`'s Go-to-JSON-Schema generator — the closest existing precedent for D-03's "generate OpenAPI from Go" decision.
- `internal/projectconfig`'s concern/key registry and provenance/explain tooling — reused for the new `api` config concern (D-06).

### Established Patterns
- Per-concern `config/*.toml` files, each owning a disjoint set of canonical keys, validated by `internal/projectconfig` — the pattern D-06's remote-access flag must follow (a new `api` concern, not a bolt-on to an existing one, unless research finds a better fit).
- `bootstrap.PlatformExecutablePath`/`PlatformKey` — the now-consistently-applied (post-fix, this session) convention for resolving any platform-specific installed executable path; relevant if the API's own tooling (e.g. an api-key CLI) needs to locate `golc-project.exe`.
- Self-registering routes via `MustDeclareRoute`/`MustDeclareScope` (`internal/command/test.go`, `build.go`) — the established pattern for adding any new internal/command-level routes this phase's REST layer might need underneath it (e.g. an `api-key create` route).

### Integration Points
- New REST/SSE server code lives alongside `internal/wails`'s existing daemon-side IPC listener (D-07) — likely a new `internal/api` (or similar) package, started/stopped by the same daemon lifecycle code in `internal/wails/app.go` or its daemon-side counterpart, not `internal/wails` itself (which is the Go-host/Wails-bridge layer, not the daemon).
- New `.golc` SQLite audit table (D-16) integrates via `internal/show`'s existing schema/migration mechanism.
- New `api` config concern (D-06) integrates via `internal/projectconfig`'s `DefaultSpec()` registry, the same way `runtime`/`toolchain`/`commands`/etc. concerns already do (discoverable via `golc_list_config_concerns`/`golc_config_inspect` MCP tools).

</code_context>

<specifics>
## Specific Ideas

No particular external references or "I want it like X" examples were given during this discussion — all 16 decisions were concrete answers to concrete implementation-choice questions, not open-ended vision statements.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope. No scope-creep suggestions arose during the 4 discussed areas.

</deferred>

---

*Phase: 7-Versioned External Control API*
*Context gathered: 2026-07-24*
