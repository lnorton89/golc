# Phase 7: Versioned External Control API - Research

**Researched:** 2026-07-24
**Domain:** Go HTTP/REST + OpenAPI + SSE public API, layered over an existing CLI command-router/daemon architecture
**Confidence:** HIGH (grounded directly in this repository's own code; library choices verified live against the Go module proxy)

## Summary

Phase 7 does not need a new domain layer — it needs a **translation layer**. `internal/command`'s `CommandRegistry` (`Request{Route,Args,Root}` → `Result{ExitCode,Stdout,Stderr}`) is already the single execution authority every CLI route goes through; the REST API's job is to turn an HTTP request into a synthetic `command.Request` (D-01), call `Execute`, and turn the `Result` back into an HTTP response. This is exactly the same shape the daemon's existing IPC layer (`internal/artnet/ipc`) already does for the Wails desktop app, just over a different wire.

The two libraries that make the 16 locked decisions cheap to implement are **Chi** (`github.com/go-chi/chi/v5`) as the router (D-04, zero non-stdlib dependencies) and **Huma** (`github.com/danielgtaylor/huma/v2`) layered on top of it via the `humachi` adapter. Huma reflects OpenAPI 3.1 straight out of Go request/response structs — the exact same "generate from Go, never hand-author" discipline `internal/contracts` already established for JSON Schema (D-03) — and ships a first-class SSE helper (`huma/v2/sse`) that documents event payload types in the same OpenAPI document (D-09). This pairing was already pre-selected with pinned, verified versions in the project's day-one `.planning/research/STACK.md`; this research re-verifies both packages are still current and adds the phase-specific integration detail STACK.md didn't need.

The single hardest problem this phase must solve, and the one the locked decisions don't resolve by themselves, is **D-15's atomic `/v1/batch`**: every existing mutating `internal/command` handler independently does `show.Load` → mutate → `show.Save` (verified directly in `pool.go`, and by grep there is no shared "load once, mutate many, save once" helper anywhere in the package). A batch endpoint that just calls `registry.Execute` N times in a loop is **not** atomic — a failure on sub-request 3 of 5 leaves 1–2 already durably saved with their own revision bumps. Making the batch genuinely all-or-nothing requires either a new State-in/State-out seam factored out of the existing handlers, or a two-phase dry-run-then-commit strategy layered on top of the existing per-command Load/Save. See Common Pitfalls and Open Questions — this is the one place planning should spend disproportionate care.

**Primary recommendation:** Build a new `internal/api` package hosting a Huma-over-Chi HTTP/SSE server, started/stopped by the existing daemon lifecycle in `internal/artnet.Run` (D-07) — not `internal/wails`, which is the Go↔WebView bridge layer, not the daemon. Every REST handler is a thin translator into `command.CommandRegistry.Execute`; every mutation's optimistic concurrency (D-13) maps directly onto the `show.State.Revision` field that already exists and already increments on every `Save`. Solve D-15 by extracting a `State`-in/`State`-out core from each mutating handler (see Architecture Patterns) rather than trying to retrofit atomicity around the existing Load/Save-per-command shape.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| REST resource routing, URL versioning | API/Backend (`internal/api`) | — | New translation layer in front of `internal/command` (D-01/D-02); no existing tier owns HTTP today |
| Command execution / domain mutation | API/Backend → `internal/command` | — | D-01: `CommandRegistry.Execute` remains the sole authority; API never calls domain packages directly |
| OpenAPI contract generation | API/Backend (build-time generator) | — | Mirrors `internal/contracts`'s generate+`CheckDrift` pattern (D-03), but driven by Huma's reflection instead of `invopop/jsonschema` |
| API-key authentication & scopes | API/Backend | Database/Storage (`.golc` key table) | D-05/D-08: keys minted via a route, hashed at rest, scope-checked per request |
| Remote-access enablement | API/Backend + Config (`internal/projectconfig`) | — | D-06: new `api` concern, loopback always open regardless of the flag |
| Process lifecycle (start/stop) | Process/Daemon (`internal/artnet.Run`) | — | D-07: same supervised process Wails already spawns/dials; API listener starts/stops alongside the IPC listener, not inside `internal/wails` |
| SSE event stream & replay buffer | API/Backend | — | D-09/D-10/D-11/D-12: in-memory ring buffer keyed by monotonic revision, fed by a post-mutation hook |
| Optimistic concurrency (`If-Match`) | API/Backend | Database/Storage (`show.State.Revision`) | D-13: no new revision concept needed — the field already exists and already increments every `Save` |
| Dry-run impact preview | API/Backend → `internal/command` (existing impact-review handlers) | — | D-14: `pool update`/`pool apply`'s existing plan/apply split is the direct precedent |
| Atomic batch | API/Backend (new orchestration) | Database/Storage (single `.golc` transaction) | D-15: needs a new State-in/State-out seam — see Common Pitfalls |
| Audit records | Database/Storage (`.golc` new `audit_log` table) | API/Backend (writer, one row per completed mutation) | D-16: reuses `internal/show`'s existing SQLite store, migration, and single-writer discipline |

## User Constraints (from CONTEXT.md)

<user_constraints>

### Locked Decisions

**Transport & versioning**
- D-01: Resource-oriented REST (e.g. `GET /v1/shows/{id}`, `POST /v1/scenes`), not a thin JSON-RPC passthrough of `internal/command`'s `Request{Route,Args}` shape. Requires a real mapping/translation layer in front of the existing route registry — `internal/command` remains the single execution authority underneath.
- D-02: URL-path versioning (`/v1/...`). Breaking changes get a new path prefix (`/v2/...`) served alongside `/v1/` during a deprecation window (API-02's compatibility/deprecation guidance).
- D-03: OpenAPI contract is generated from Go code (handler/struct definitions), not hand-authored spec-first. Mirrors the existing `internal/contracts` pattern already used for JSON-Schema generation (fixtures, config concerns) — spec always matches implementation by construction.
- D-04: HTTP routing uses a router library (chi or gorilla), not stdlib `net/http` alone, despite the project's general minimal-dependency bias — the user explicitly chose ergonomics for route/middleware grouping over avoiding a new pinned dependency. This new dependency must be checksum-pinned the same way `config/toolchain.toml` pins everything else offline-buildable in this repo.

**Auth & remote access**
- D-05: Clients authenticate with per-token scoped API keys with expiry (not static long-lived bearer tokens). Keys are minted through a dedicated route/CLI, stored hashed, individually revocable.
- D-06: Remote-access enablement is a config flag under `config/*.toml` (following the existing per-concern TOML pattern: `config/runtime.toml`, etc., with the same provenance/explain tooling `internal/projectconfig` already provides) — not a CLI startup flag. Loopback stays open regardless of this flag; the flag only governs binding beyond loopback.
- D-07: The API server runs inside the existing supervised `golc-project` daemon process (the same one `internal/wails` already spawns/dials over named pipe or Unix socket, with Phase 6's orphan-cleanup and supervised-lifecycle already built) — not a separate process/binary.
- D-08: API-key scopes are coarse domain scopes (e.g. `playback`, `authoring`, `admin`), not per-domain read/write splits. Mirrors ROADMAP.md/REQUIREMENTS.md's existing domain grouping (`PLAY-*`, `POOL-*`, etc.).

**Event streaming & gap recovery**
- D-09: One global SSE stream (`GET /v1/events`) emitting every domain's changes with a `type` field, each event carrying a monotonic revision number — not separate per-domain streams.
- D-10: Gap recovery uses SSE's standard `Last-Event-ID` header plus a bounded server-side revision replay buffer; on reconnect, missed events within the buffer window are replayed, and a buffer overflow (satisfying API-03's "recover from an event gap by re-querying authoritative state") tells the client to fall back to a full re-fetch via the REST resource endpoints before resuming the stream.
- D-11: All connected, authenticated clients see the full event stream regardless of their key's domain scopes — domain scopes only gate REST mutations, not stream visibility. (A narrowly-scoped playback-only integration will also observe authoring/config events.)
- D-12: Any valid, non-expired API key can open the event stream — no separate "streaming" capability/flag is required on top of the coarse domain scopes from D-08.

**Mutation semantics & audit**
- D-13: Expected-revision concurrency uses the standard HTTP conditional-request pattern: `If-Match: "<revision>"` on mutating requests, `412 Precondition Failed` on mismatch — not a body-embedded `expected_revision` field.
- D-14: Dry-run previews use a `?dry_run=true` query parameter on the same mutating endpoint, returning the would-be impact/result without applying it — not a separate `/preview` sub-resource. Mirrors the desktop UI's existing impact-review pattern (fixture pool updates, deployment changes).
- D-15: Atomic batches go through one generic `/v1/batch` endpoint accepting an ordered list of sub-requests, applied as a single atomic transaction (all-or-nothing) — not per-resource bulk endpoints designed individually.
- D-16: Audit records (actor, source, correlation, outcome, redacted details per API-06) are written to a new table in the existing `.golc` SQLite store (Phase 5's durable storage engine — migrations, integrity checks, lock-retry already built), not a separate append-only log file. Audit records travel with the show file.

### Claude's Discretion
None — the user answered all 16 questions directly; no "you decide" selections were made in this discussion. (Rate limiting, idempotency-key mechanics, redaction rules, and SSE buffer sizing were mentioned in ROADMAP.md's phase note but never resolved to a specific decision — this research proposes concrete defaults for those below, flagged `[ASSUMED]` for the planner/discuss-phase to confirm if desired.)

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope.

</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| API-01 | External program can query/invoke every public domain capability through a versioned API using the same command model as the UI | D-01 translation layer over `command.CommandRegistry.Execute`; Architecture Pattern 1 |
| API-02 | OpenAPI contract, generated client examples, typed errors, compatibility/deprecation guidance | Huma auto-generates OpenAPI 3.1 from Go structs (D-03); Pattern 2 (versioned routing + deprecation window) |
| API-03 | Revisioned SSE, recover from a gap by re-querying authoritative state | D-09/D-10 ring buffer + `Last-Event-ID`; Pattern 3 |
| API-04 | Expected revisions, idempotency, dry-run previews, atomic batches | D-13 (`If-Match` on `show.State.Revision`), D-14 (`?dry_run=true`), D-15 (batch — see Pitfall 1 and Open Question 1) |
| API-05 | Loopback default, explicit enablement + scoped auth for remote access | D-06 new `api` config concern; Security Domain section |
| API-06 | Every mutation records actor/source/correlation/outcome/redacted details | D-16 new `audit_log` table in `.golc`; Pattern 5 |

</phase_requirements>

## Project Constraints (from CLAUDE.md)

The project's `.claude/CLAUDE.md` routes only to `.planning/sketches/SKILL.md` for "Sketch findings" (validated UI/UX/layout decisions) — not directly applicable to this backend-only phase, but if this phase's plan produces any operator-facing surface (e.g. an API-key management screen), that skill's sketch findings should be consulted before designing it. No other project-wide directives apply to Phase 7's scope.

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/go-chi/chi/v5` | **v5.3.1** `[VERIFIED: go list -m -versions, live proxy.golang.org query 2026-07-24]` | HTTP router underlying the API server (D-04) | Idiomatic, `net/http`-compatible (`http.Handler` all the way down, no custom context type), literally zero non-stdlib dependencies in its own `go.mod`, actively released (v5.3.1 is newer than `gorilla/mux`'s last release, v1.8.1 from 2022 — `gorilla/mux` is effectively feature-frozen). Already the exact router `.planning/research/STACK.md` pre-selected for this phase at project inception. |
| `github.com/danielgtaylor/huma/v2` | **v2.39.0**, released 2026-07-15 `[VERIFIED: proxy.golang.org module info, live query 2026-07-24]` | OpenAPI-3.1-from-Go framework layered on Chi via `humachi` (D-03) | Reflects request/response Go structs directly into the OpenAPI document — the HTTP-specific analog of `internal/contracts`'s "generate, never hand-author" discipline this repo already lives by. Ships typed-error responses (`huma.Error*`) and a first-class SSE registration helper (`huma/v2/sse`) that documents event payload shapes in the same spec (satisfies API-02's "generated client examples, typed errors" directly). |
| `github.com/danielgtaylor/huma/v2/adapters/humachi` | same v2.39.0 module | Wires Huma onto an existing `chi.Router` | `humachi.New(router, config)` — confirms Huma does not replace Chi, it rides on top of it, so D-04's literal "chi or gorilla" choice is honored, not relitigated. |
| `github.com/danielgtaylor/huma/v2/sse` | same v2.39.0 module | Typed SSE registration (D-09) | `sse.Register(api, operation, map[string]any{"pool:created": PoolCreatedEvent{}, ...})` documents every event `type` name and payload shape in the OpenAPI doc, rather than a bare untyped `text/event-stream` handler. |
| `golang.org/x/time/rate` | **v0.15.0** `[VERIFIED: go list -m -versions, live query 2026-07-24]` — **new pin, not yet in go.mod** | Per-key token-bucket rate limiting | Official Go org package, single-purpose, stdlib-adjacent trust level. No rate-limiting decision was locked in CONTEXT.md; this is the standard Go primitive to build one with (`[ASSUMED]` — see Open Question 2). |
| `crypto/sha256` + `crypto/rand` (Go 1.26 stdlib) | Go 1.26.5 (already pinned) | API-key generation and storage (D-05) | High-entropy random tokens (32 bytes from `crypto/rand`, base64url-encoded) do not need a slow password hash — SHA-256 of the token is the standard pattern (GitHub PAT / Stripe secret-key model): store only the hash, compare in constant time, keep a short unhashed prefix for O(1) lookup. Zero new dependency — `golang.org/x/crypto/bcrypt` is unnecessary here and would add per-request latency with no security benefit for random (not user-chosen) tokens. |
| `github.com/google/uuid` | v1.6.0 (already pinned, direct) | Correlation IDs, idempotency-key storage, API-key IDs | Already the repo's UUID convention (fixtures, groups, scenes, chases all use it) — reuse verbatim, no new decision needed. |
| `modernc.org/sqlite` | v1.54.0 (already pinned, direct) | Audit table storage (D-16) | Reused as-is via `internal/show`'s existing `openStore`/schema machinery — see Pattern 5. |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/go-chi/chi/v5/middleware` (ships inside the `chi` module, no extra pin) | v5.3.1 | `RequestID`, `Recoverer`, `Timeout`, `RealIP` | Use `middleware.RequestID` to seed the audit-log `correlation_id`/`X-Request-Id` response header; use `Recoverer` so a handler panic never crashes the daemon process (the daemon also owns the Art-Net worker and playback engine — a panicking HTTP handler must not take those down). |
| `log/slog` (Go 1.26 stdlib) | stdlib | Structured operational logs, separate from the audit table | Mirrors `.planning/research/STACK.md`'s existing guidance: SQLite audit is the durable truth (D-16), `slog` is diagnostics only. |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `go-chi/chi` | `gorilla/mux` | Both satisfy D-04's literal wording. `gorilla/mux` v1.8.1 (2022) is effectively unmaintained relative to chi's active v5.3.1 releases; chi's zero-dependency `go.mod` is a better fit for this repo's minimal-dependency ethos when a router is unavoidable. Recommend chi. |
| `danielgtaylor/huma` for OpenAPI generation | Hand-written OpenAPI YAML | Violates D-03 explicitly ("not hand-authored spec-first"). |
| `danielgtaylor/huma` for OpenAPI generation | Reuse `internal/contracts`'s `invopop/jsonschema` generator directly and hand-wire it into HTTP responses | `invopop/jsonschema` only produces JSON Schema, not a full OpenAPI document (paths, operations, security schemes, parameters). Huma internally uses a JSON-Schema-shaped reflector for the same purpose `internal/contracts` already trusts, then adds the HTTP-specific envelope — less net-new code than hand-building an OpenAPI 3.1 assembler on top of `invopop/jsonschema`. |
| SHA-256 API-key hashing | `golang.org/x/crypto/bcrypt` | `bcrypt` is designed to be slow, which defends low-entropy user-chosen passwords against offline brute force. A `crypto/rand`-generated 256-bit API key has no brute-force risk that bcrypt's deliberate slowness mitigates, and paying that cost on every authenticated request (D-12: "any valid key opens the event stream") is pure overhead. Use SHA-256. |
| One `/v1/batch` executing existing handlers in a loop | Per-resource bulk endpoints | Explicitly rejected by D-15. |

**Installation:**
```bash
# Run once, online, by a contributor (not inside GOPROXY=off bootstrap/build/test):
go get github.com/go-chi/chi/v5@v5.3.1
go get github.com/danielgtaylor/huma/v2@v2.39.0
go get golang.org/x/time@v0.15.0
# then commit the resulting go.mod/go.sum diff — see "Pinning a new Go dependency" below.
```

**Version verification:** Confirmed live against `proxy.golang.org` on 2026-07-24 via `go list -m -versions <module>` and the module's `@v/<version>.info` endpoint (see table above). All three are current tags, not stale/pre-release.

### Pinning a new Go dependency in this repo (grounded in `internal/bootstrap`)

This repo's bootstrap/build/test path runs with `GOFLAGS=-mod=readonly` and `GOTOOLCHAIN=local` (`internal/bootstrap/cache.go`'s `OfflineEnvironment`), and `internal/bootstrap/engine.go` **hard-fails** (`GOLC_BOOTSTRAP_LOCK_MUTATION`) if `go.mod`/`go.sum` change during a bootstrap run. This does **not** mean dependencies can never be added — it means adding one is a deliberate, separate, online contributor action, never something that happens implicitly during `mage build`/`mage test`:

1. A contributor runs `go get <module>@<exact version>` with normal network access (default `GOPROXY`, not the repo's offline-forced environment) — this is the *only* step that needs the network.
2. This updates `go.mod` (adds the require line) and `go.sum` (adds the verified hash entries) exactly the same way `modernc.org/sqlite@v1.54.0` was pinned for Phase 5 (`.planning/research/STACK.md` line 116: `go get modernc.org/sqlite@v1.54.0`).
3. Both files are committed as a normal, reviewable diff — the toolchain pins in `config/toolchain.toml` (Go/Node/Mage binaries with SHA-256-verified archives) are a *separate* pinning mechanism for the toolchain itself, not for Go module dependencies; `go.sum`'s own hashes are Go's native equivalent for the module graph and need no additional GOLC-specific pinning step.
4. After that commit, every subsequent `GOPROXY=off`/`-mod=readonly` bootstrap, build, and test run resolves the new modules from `go.sum` + the local `GOMODCACHE` without ever touching the network again — exactly like every other dependency this repo already has.
5. `go-chi/chi` has no transitive dependencies of its own; `golang.org/x/time` has none beyond stdlib. `danielgtaylor/huma/v2`'s core has no mandatory third-party dependency either — verify with `go mod graph | grep huma` after the `go get` to confirm nothing unexpected was pulled in before committing.

## Package Legitimacy Audit

Ecosystem is Go modules (`proxy.golang.org`), not npm/PyPI/crates — the automated `gsd-tools query package-legitimacy check` seam targets those three ecosystems and does not cover Go, so this audit was performed manually per the "Ecosystem-specific registry verification" step of the protocol, using `go list -m -versions` and the module proxy's `.info` endpoint (both executed live against the real proxy on 2026-07-24 — see command output captured during this research session).

| Package | Registry | Age / Latest Release | Downstream Adoption | Source Repo | Verdict | Disposition |
|---------|----------|-----------------------|----------------------|--------------|---------|-------------|
| `github.com/go-chi/chi/v5` | Go module proxy | v5.3.1, actively released since 2015 (chi v1) | One of the most widely adopted Go HTTP routers; already known to and pre-selected by this project's own `.planning/research/STACK.md` | `github.com/go-chi/chi` | OK | Approved |
| `github.com/danielgtaylor/huma/v2` | Go module proxy | v2.39.0, released 2026-07-15 (9 days before this research) — mature v2 line since 2022 | Well-documented (huma.rocks), maintained by a known Go community author, already pre-selected by this project's own `.planning/research/STACK.md` | `github.com/danielgtaylor/huma` | OK | Approved |
| `golang.org/x/time` | Go module proxy | v0.15.0 | Official `golang.org/x/...` extended standard library | `golang.org/x/time` (Go project) | OK | Approved |

**Packages removed due to SLOP verdict:** none.
**Packages flagged as suspicious [SUS]:** none.

All three package names above were carried over from this project's own pre-existing `.planning/research/STACK.md` (written 2026-07-17, before this phase's discussion), which itself resolved them through Context7/official-docs lookups at project inception — not freshly hallucinated in this research session. Per the provenance rule, the package *names* are still tagged `[ASSUMED]` in the sense that their selection is a design recommendation, not a locked CONTEXT.md decision, but their *existence and current version* were independently `[VERIFIED]` live against the Go module proxy in this session (command output above), which is the strongest verification available for the Go ecosystem short of a full `go mod download` inside the sandbox.

## Architecture Patterns

### System Architecture Diagram

```
 External program (curl, Postman, future TS/LLM client)
        │  HTTPS/HTTP  (loopback by default, D-06 gates remote bind)
        │  Authorization: Bearer <api-key>           GET /v1/events (SSE)
        ▼                                                    │
 ┌───────────────────────────────────────────────────────────┴──────┐
 │  internal/api  (NEW — hosted inside internal/artnet.Run daemon)   │
 │                                                                    │
 │  Chi router  ──▶  Huma API layer (OpenAPI 3.1 + typed errors)     │
 │       │                 │                                         │
 │       │      auth middleware: hash(Authorization) lookup ─────────┼──▶ .golc: api_keys table (D-05)
 │       │      scope check (coarse domain scope, D-08)              │
 │       │      rate limiter (per-key token bucket)                  │
 │       │      If-Match / dry_run / batch orchestration             │
 │       ▼                                                            │
 │  translate HTTP request ──▶ command.Request{Route,Args,Root}      │
 └───────────────────────────┬────────────────────────────────────────┘
                              │  (the ONE execution authority — D-01)
                              ▼
            internal/command.CommandRegistry.Execute
                              │
              ┌───────────────┼────────────────┐
              ▼               ▼                ▼
      internal/show     internal/pool    internal/scene ...
      (Load/mutate/Save,      (existing domain packages, unmodified)
       Revision++, D-13)
              │
              ▼
   .golc SQLite (show_meta/show_state, + NEW audit_log + api_keys tables)
              │
   on successful mutation ──▶ post-commit hook ──▶ SSE ring buffer (D-10)
                                                       │
                                                       ▼
                                          broadcast to every open
                                          /v1/events connection (D-11)
```

### Recommended Project Structure
```
internal/api/
├── doc.go              # package overview, mirrors internal/artnet/ipc/doc.go's style
├── server.go            # http.Server construction, graceful Shutdown, wired into artnet.Run's start/stop ordering (D-07)
├── router.go             # Chi router + humachi.New(...) wiring, /v1 prefix (D-02)
├── translate.go          # HTTP request -> command.Request{Route,Args,Root} translation (D-01)
├── auth.go               # API-key hashing, scope check, expiry check (D-05/D-08)
├── ratelimit.go           # golang.org/x/time/rate per-key limiter
├── revision.go            # If-Match parsing, 412 handling against show.State.Revision (D-13)
├── dryrun.go              # ?dry_run=true handling, reusing pool update/apply-style plan/apply split (D-14)
├── batch.go               # /v1/batch orchestration -- see Pattern 4 / Pitfall 1
├── events.go              # SSE ring buffer, Last-Event-ID replay, broadcaster (D-09/D-10/D-11/D-12)
├── audit.go               # post-mutation audit_log writer (D-16)
├── keys.go                # api-key CRUD routes (mint/list/revoke) -- self-registers its OWN internal/command
│                           # route(s) too (e.g. "api-key create"), per D-01's "single execution authority"
├── generate.go             # OpenAPI doc generation entrypoint, mirrors internal/contracts's GenerateAll/CheckDrift
└── *_test.go
```

### Pattern 1: HTTP → `command.Request` translation (D-01)

**What:** Every REST handler parses its typed Huma input struct, converts it into the exact `--flag value` argv shape `internal/command`'s handlers already parse (see `internal/command/pool.go`'s `parsePoolCreateArgs`), and calls the shared `CommandRegistry.Execute`. The REST layer never imports `internal/pool`, `internal/scene`, etc. directly.

**When to use:** Every mutating and query route.

**Example (grounded in the actual existing route shape):**
```go
// Source: internal/command/router.go (Request/Result/Execute), internal/command/pool.go (existing arg shape)
type CreatePoolInput struct {
    Body struct {
        Name     string   `json:"name" required:"true"`
        Requires []string `json:"requires,omitempty"`
    }
}

func (s *Server) handleCreatePool(ctx context.Context, in *CreatePoolInput) (*PoolOutput, error) {
    args := []string{in.Body.Name}
    if len(in.Body.Requires) > 0 {
        args = append(args, "--requires", strings.Join(in.Body.Requires, ","))
    }
    args = append(args, "--show", s.showPath) // daemon's own configured show, never client-supplied (D-07: one running show per daemon)

    result := s.registry.Execute(command.Request{Route: "pool create", Args: args, Root: s.root})
    return translateResult(result) // ExitCode 0 -> 201, ExitCode 1 -> typed domain error, ExitCode 2 -> 400
}
```

**Why the show path is server-side, not client-supplied:** the daemon (`internal/artnet.Run`) is started once against one `cfg.State`/show file (D-07); a REST client interacts with "the show this daemon is running", not an arbitrary filesystem path, so `--show` is always injected server-side from the daemon's own configuration, never accepted from the request body.

### Pattern 2: URL-path versioning + deprecation window (D-02)

**What:** Mount `/v1` as one Chi sub-router; when a breaking change is needed, mount a parallel `/v2` sub-router that shares as much handler code as possible, and keep `/v1` alive and fully functional for a documented window.

**Example:**
```go
// Source: go-chi/chi router-mounting idiom (github.com/go-chi/chi/v5)
r := chi.NewRouter()
r.Mount("/v1", v1Router(deps))
if cfg.V2Enabled { // flips on only once a v2 actually exists
    r.Mount("/v2", v2Router(deps))
}
```
Document the deprecation policy itself (how long `/v1` outlives `/v2`'s introduction, what response header signals deprecation — e.g. `Sunset:`/`Deprecation:` per the emerging IETF drafts) as part of API-02's "compatibility/deprecation guidance"; this is documentation content the plan must write, not a library feature.

### Pattern 3: SSE with `Last-Event-ID` replay (D-09/D-10)

**What:** A bounded in-memory ring buffer keyed by the same monotonic `show.State.Revision` every mutation already produces. On connect, if the client sends `Last-Event-ID`, replay everything in the buffer newer than that ID; if the requested ID has already scrolled out of the buffer, respond with a distinguishable "gap" signal (a synthetic `event: resync` message) telling the client to re-fetch authoritative state via the REST endpoints before resuming — this is exactly D-10's requirement and mirrors the Wails `EventPusher`'s own documented anti-pattern warning ("a throttled hint stream, never the ... source of truth ... the frontend re-queries authoritative state on any detected gap", `internal/wails/events.go`).

**Example (using Huma's typed SSE registration):**
```go
// Source: github.com/danielgtaylor/huma/v2/sse (huma.rocks/features/server-sent-events-sse/)
sse.Register(api, huma.Operation{
    OperationID: "watch-events",
    Method:      http.MethodGet,
    Path:        "/v1/events",
}, map[string]any{
    "pool:created":   PoolCreatedEvent{},
    "scene:switched": SceneSwitchedEvent{},
    "resync":         ResyncEvent{}, // D-10's overflow signal
    // ... one entry per domain event type, D-09's single global stream
}, func(ctx context.Context, in *WatchEventsInput, send sse.Sender) {
    lastID := in.LastEventID // huma binds the Last-Event-ID header automatically when declared
    replayFrom(lastID, send) // ring-buffer replay or resync signal
    broadcaster.Subscribe(ctx, send) // D-11: every connection sees every domain's events
})
```

**Where events originate:** hook the SSE publish call immediately after a successful `show.Save` inside the API layer's own post-mutation step (Pattern 1's `translateResult`), not inside `internal/show` itself — `internal/show` must stay decoupled from event distribution the same way its own doc comments insist it stay decoupled from `internal/playback` ("internal/show does not, and must not, import internal/playback").

### Pattern 4: Dry-run reusing the existing plan/apply split (D-14)

**What:** `internal/command/pool.go` already implements exactly the shape D-14 wants for pools: `pool update` computes and returns a deterministic impact-review plan **without mutating** the show, and a separate `pool apply` validates + applies it. `?dry_run=true` on a REST mutating endpoint should call the "compute plan, don't mutate" side of a handler pair (where one exists) or, for domains without an existing plan/apply split, call the real mutation logic against a **discarded** in-memory copy of `State` (Load, mutate, diff, never Save) and return the projected result.

**Anti-pattern:** Do not build a second, parallel "preview" implementation of each mutation's logic — reuse whichever of (a) an existing plan/apply command pair, or (b) the same mutate-in-memory function used by the real path, whichever the target domain already has.

### Pattern 5: Audit table (D-16), following `internal/show/schema.go`'s exact conventions

**What:** Add one new table to the same `createTablesSQL` block `internal/show/schema.go` already owns, using the same `IF NOT EXISTS` / singleton-vs-append-only conventions the existing three tables use (`recovery_points` is the closest analog: append-only, `AUTOINCREMENT` id).

**Example:**
```sql
-- Source: internal/show/schema.go's createTablesSQL block (existing style, extended)
CREATE TABLE IF NOT EXISTS audit_log (
  id              INTEGER PRIMARY KEY AUTOINCREMENT,
  occurred_at     TEXT    NOT NULL,   -- RFC3339, matches show_meta.updated_at's format
  actor           TEXT    NOT NULL,   -- api-key id (not the raw key) or "cli"/"wails"
  source          TEXT    NOT NULL,   -- "http", "wails", "cli" (extensible for Phase 8/9 script/LLM sources)
  correlation_id  TEXT    NOT NULL,   -- request ID / idempotency key, propagated in X-Request-Id
  route           TEXT    NOT NULL,   -- the internal/command route actually executed, e.g. "pool create"
  expected_revision INTEGER,          -- NULL if the request omitted If-Match
  resulting_revision INTEGER,         -- show.State.Revision after Save, NULL on failure/dry-run
  outcome         TEXT    NOT NULL,   -- "success" | "failure" | "dry_run" | "rejected"
  status_code     INTEGER NOT NULL,
  redacted_details TEXT   NOT NULL    -- canonical JSON, secrets/tokens stripped before write (see Security Domain)
);
```
Write the audit row in the **same transaction discipline** `store.go`'s `stageRecoveryPoint`/`promoteState` already use (a dedicated `db.Begin()`/`tx.Exec`/`tx.Commit()`), called from the API layer immediately after a mutation completes — reuse `openStore`'s already-hardened connection setup (`busy_timeout`, `WAL`, single-writer `SetMaxOpenConns(1)`) rather than opening a second, uncoordinated connection to the same `.golc` file.

### Anti-Patterns to Avoid
- **A second command-dispatch surface:** Building REST handlers that call `internal/pool`/`internal/scene`/etc. directly, bypassing `command.CommandRegistry.Execute`, would create exactly the "second, independently-maintained command surface" CONTEXT.md's Phase Boundary explicitly rules out.
- **Treating the SSE stream as authoritative:** Per `internal/wails/events.go`'s own documented anti-pattern for the *existing* Wails hint-stream, the new public SSE stream must be the same kind of hint — clients must always be able to re-fetch ground truth from the REST endpoints, which D-10 already requires structurally.
- **A generic HTTP ETag middleware for `If-Match`:** Chi ships no built-in conditional-request middleware, and none should be bolted on generically — `If-Match` here compares against the domain-meaningful `show.State.Revision`, not a content hash of the HTTP response body, so the check belongs in each mutating handler (Pattern 1), not a cross-cutting middleware.
- **Opening a second, uncoordinated SQLite connection for audit writes:** `internal/show`'s store is explicitly single-writer (`SetMaxOpenConns(1)`) with a `busy_timeout` PRAGMA added specifically because multiple OS processes (Wails app + artnet daemon + CLI) already contend on the same `.golc` file. A naive `sql.Open` in `internal/api` for audit writes reintroduces the exact contention class that PRAGMA was added to fix. Route audit writes through the same store machinery `internal/show` already exposes (or extend it with an explicit exported audit-write function) rather than hand-rolling a parallel connection.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| OpenAPI 3.1 document generation | A custom Go-struct-to-OpenAPI reflector | Huma (`danielgtaylor/huma/v2`) | Huma already solves this exact problem the way `internal/contracts` solves it for JSON Schema; hand-building path/operation/security-scheme reflection is a second, worse version of the same idea. |
| SSE framing (`event:`/`data:`/`id:` wire format, heartbeats) | Manual `text/event-stream` writer | `huma/v2/sse` | Handles the wire format, per-event typing, and OpenAPI documentation of event shapes in one call. |
| Router with named path params, middleware chaining, sub-router mounting | Custom `net/http.ServeMux`-based dispatch | Chi | This is precisely why D-04 was locked in the first place. |
| API-key generation entropy | A custom PRNG-based token scheme | `crypto/rand` (stdlib) directly, 256 bits, base64url | `crypto/rand` is already the correct, audited primitive; nothing to add. |
| Rate limiting | A hand-rolled counter+timestamp map | `golang.org/x/time/rate` | Textbook token-bucket, already the official Go extended-stdlib implementation; a hand-rolled version would need its own correctness testing for no benefit. |
| Audit-log durability/atomicity | A separate append-only log file with its own fsync/rotation logic | `internal/show`'s existing `.golc` SQLite store | Explicitly what D-16 chose, and the store already has migrations, integrity checks, and lock-retry built for Phase 5. |

**Key insight:** almost everything this phase needs is a **wiring** problem against libraries and patterns this repository (or the Go ecosystem's obvious standard tools) already has, not a **build new infrastructure** problem. The one genuine new-infrastructure problem is D-15's batch atomicity (see below) — that is where hand-rolling is unavoidable because no existing seam in this codebase currently supports "load once, apply many command effects, save once."

## Common Pitfalls

### Pitfall 1: `/v1/batch` cannot be atomic by looping over `registry.Execute` (D-15)

**What goes wrong:** Every mutating `internal/command` handler independently calls `show.Load(root, path)` then, later, `show.Save(root, path, state)` (verified directly in `pool.go`'s `runPoolCreate`, and confirmed by grep across the package — there is no shared "load once, mutate many, save once" helper anywhere in `internal/command`). If `/v1/batch` naively calls `registry.Execute` once per sub-request, each call does its **own** independent Load/Save/Revision-bump. A failure on sub-request 3 of 5 leaves 1–2 already durably committed with their own revision bumps — not atomic, and not rollback-able after the fact.

**Why it happens:** `internal/command`'s handlers were designed for the "one short-lived CLI process per command" model (`internal/show/schema.go`'s own doc comment: "Every show-mutating CLI invocation is its own short-lived process"), where each command owning its full load-mutate-save cycle is exactly right. A long-lived, in-process batch orchestrator breaks that assumption.

**How to avoid — two viable strategies for the plan to choose between:**
1. **State-in/State-out extraction (structurally correct, higher effort):** factor each mutating handler's core logic into a `func(show.State, args) (show.State, error)` shape, separate from the `Load`/`Save` orchestration wrapper `runPoolCreate` etc. currently inline. The batch endpoint then: `Load` once, apply each sub-request's mutate function against the same in-memory `State` value in order, and only if every step succeeds, `Save` once (one revision bump for the whole batch). Any failure mid-sequence discards the in-memory `State` and touches disk not at all. This preserves D-01 (still calling into the same domain logic `internal/command` owns) while fixing atomicity, but touches every mutating command file — sequence this as its own wave in the phase plan.
2. **Pre-validate-then-commit (pragmatic MVP fallback):** run every sub-request's dry-run path (Pattern 4) first; only if all dry-runs succeed, execute the real mutations in sequence via the existing per-command `Execute`. This reduces (but does not eliminate) mid-batch-failure risk and does **not** protect against a concurrent external write racing between the dry-run pass and the real pass — mitigate that residual gap with a batch-level `If-Match` checked once at the start and re-checked before each real sub-request's own `Save`. This is weaker than option 1 but requires no changes to existing command handlers.

Document whichever strategy the plan picks explicitly in `PLAN.md`; this is the single highest-risk design decision in the whole phase and deserves its own dedicated task/wave rather than being folded into general "build the batch endpoint" work.

**Warning signs during planning:** any task description for `/v1/batch` that says "loop and call Execute" without addressing what happens to already-applied sub-requests on a later failure has not actually solved D-15.

### Pitfall 2: Bursty batch/HTTP mutation traffic can serialize behind the existing `busy_timeout=5000ms`

**What goes wrong:** `internal/show/schema.go`'s `openStore` already documents that the artnet daemon and the Wails app's short-lived per-call connections contend on the same `.golc` file, and added `PRAGMA busy_timeout = 5000` specifically to paper over that. An HTTP API adding a third concurrent writer (and a batch endpoint potentially issuing several sequential `Save`s) increases contention further. A client issuing rapid successive mutating requests could see multi-second latency spikes as SQLite's single-writer lock queues them.

**How to avoid:** serialize mutating requests at the `internal/api` layer itself (a single mutex or a bounded worker queue guarding the "translate → Execute → audit-write" critical section) so the API server never itself becomes a second source of concurrent writers beyond what `busy_timeout` already tolerates from the CLI/Wails processes. This also directly simplifies Pitfall 1's batch design (a serialized queue is a natural place to hold the in-memory `State` across a batch's steps).

### Pitfall 3: `--show` must never be client-suppliable over HTTP

**What goes wrong:** Every existing `internal/command` mutating route takes `--show <path>` as an argument, because each CLI invocation targets an arbitrary file. A REST translation layer that naively forwards a client-supplied `show_path` field into that `--show` argument reopens a path-traversal / arbitrary-file-write surface no CLI threat model needed to consider (a local operator already has filesystem access; a remote HTTP client should not gain equivalent access through the API).

**How to avoid:** the API server is started against exactly one show (the daemon's own `cfg.ShowPath`, D-07); every translated `command.Request` must inject that fixed path server-side (Pattern 1) and reject any client-supplied path field outright.

### Pitfall 4: Loopback-default must be enforced at bind time, not just documented

**What goes wrong:** D-06 requires loopback-by-default with explicit opt-in for remote binding. A common mistake is binding `0.0.0.0` in code and only *documenting* "don't do this in production" — that is not an enforced default.

**How to avoid:** the `http.Server`'s listener address must be derived from the `api` config concern's resolved value (through `internal/projectconfig`, D-06), defaulting to `127.0.0.1:<port>` when the concern's remote-enable key is absent/false, and only binding `0.0.0.0` (or a specific non-loopback interface) when that key is explicitly `true`. Since `internal/projectconfig`'s locked/writable model (`registry.go`) treats most keys as `Locked: true` by default, the new `api.remote_enabled` key needs to be declared with `Locked: false` (mirroring the one existing writable key, `runtime.log_level`) so an operator can actually flip it without hitting `GOLC_CONFIG_LOCKED_OVERRIDE`.

## Code Examples

### Deriving loopback-vs-remote bind address from the new `api` config concern
```go
// Source: pattern mirrors internal/projectconfig/registry.go's existing
// runtime.log_level writable-key precedent
addr := "127.0.0.1:" + strconv.Itoa(cfg.Port)
if resolved.RemoteEnabled { // resolved via internal/projectconfig.ResolveAll against the new "api" concern
    addr = cfg.BindInterface + ":" + strconv.Itoa(cfg.Port) // still requires an explicit, non-default interface value
}
srv := &http.Server{Addr: addr, Handler: chiRouter}
```

### Graceful shutdown mirroring `internal/artnet/ipc/server.go`'s existing discipline
```go
// Source: internal/artnet/ipc/server.go's Serve() ctx.Done()-closes-listener pattern, adapted for net/http
go func() {
    <-ctx.Done()
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    _ = srv.Shutdown(shutdownCtx) // stops accepting new conns, lets in-flight requests (incl. SSE) drain
}()
if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
    return fmt.Errorf("GOLC_API_LISTEN_FAILED: %v", err)
}
```
Wire this into `internal/artnet.Run`'s existing ordered start (`engine.Start` → `ifaceMgr.Start` → `startWorkerLocked` → `ipc.Serve`) and reverse-ordered stop, as one more subsystem in that same sequence (D-07) — start the API listener after the IPC listener is up, stop it before the IPC listener during shutdown, matching the file's own documented "subsystems start in order ... and stop in the reverse order" discipline.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|-------------------|---------------|--------|
| Hand-authored OpenAPI YAML kept in sync manually with handlers | Reflection-generated OpenAPI from typed request/response structs (Huma, and this repo's own `invopop/jsonschema`-based `internal/contracts`) | This has been the dominant Go API-framework pattern since roughly 2021 (Huma v1) and is standard practice by 2026 | Directly satisfies D-03's "generated from Go code" requirement with no custom tooling to build |
| WebSocket-first bidirectional public APIs | One-way replayable SSE + HTTP command endpoints | `.planning/research/STACK.md` already made this call project-wide ("WebSocket-first public API ... Creates a second bidirectional command protocol ... Use HTTP JSON commands plus replayable SSE events") | D-09 already committed to SSE; no reconsideration needed |

**Deprecated/outdated:**
- `gorilla/mux` as the default Go router recommendation: still functional and widely deployed in legacy code, but its release cadence has slowed dramatically relative to `chi`; new projects in 2026 default to `chi` (or `net/http`'s own now-native pattern-based `ServeMux` for the simplest cases, which doesn't provide the middleware/mounting ergonomics D-04 explicitly wanted).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Chi is the better of the two D-04-locked options (chi vs. gorilla) | Standard Stack / Alternatives Considered | Low — both satisfy the locked decision's literal text; switching to gorilla later is a router-layer swap, not a re-architecture, since Huma is router-agnostic via its adapter layer |
| A2 | Huma (layered on Chi) is the right implementation of D-03, even though CONTEXT.md's D-03 text only says "generated from Go code" and doesn't name Huma specifically | Standard Stack | Medium — if the planner/user prefers a leaner hand-rolled generator reusing `internal/contracts`'s existing `invopop/jsonschema` machinery instead of adding Huma, that is a viable (if more effort-intensive) alternative; flag for confirmation before locking the plan |
| A3 | No rate-limiting mechanics were locked in CONTEXT.md; `golang.org/x/time/rate` per-API-key token bucket is proposed | Standard Stack, Don't Hand-Roll | Low — rate limiting was named in ROADMAP.md's phase note as something "phase planning must define"; this is exactly that definition, open to adjustment |
| A4 | Batch atomicity (D-15) requires a State-in/State-out refactor of existing command handlers, OR a pre-validate-then-commit fallback; no smaller fix exists | Common Pitfalls, Open Questions | High if wrong — this is the phase's central technical risk; if a smaller fix does exist it should be found before planning, since the refactor-vs-fallback choice affects how many command files this phase's tasks must touch |
| A5 | The audit table's `redacted_details` column should store canonical JSON with secrets already stripped before write (not stored raw and redacted at read time) | Pattern 5, Security Domain | Medium — storing raw-then-redacting-at-read risks a code path that forgets to redact; strip-before-write is safer but means the audit table can never retroactively surface a field that redaction rules initially missed |
| A6 | Idempotency keys (API-04's "idempotency") should follow the `Idempotency-Key` HTTP header convention (Stripe-style: client-supplied key, server stores/returns the first response for a given key within a TTL) rather than a body field | Open Questions | Low — this is a well-established REST convention, not a codebase-specific inference, but was not discussed in CONTEXT.md and deserves explicit confirmation |

## Open Questions

1. **How exactly should `/v1/batch` achieve true atomicity without a large refactor?**
   - What we know: every existing mutating command handler independently does Load→mutate→Save (verified by direct code reading); no shared State-in/State-out helper exists today.
   - What's unclear: whether the phase's `mvp` mode tolerates the pragmatic pre-validate-then-commit fallback (Pitfall 1, option 2) for v1, deferring the full State-in/State-out extraction (option 1) to a later hardening pass, or whether "atomic ... all-or-nothing" (D-15's own wording) requires option 1 from day one.
   - Recommendation: raise this explicitly with the user/planner before committing to a wave structure — it changes whether this phase touches ~15 existing command files or none of them.

2. **What are the concrete default rate limits, per-scope or global?**
   - What we know: D-08 establishes coarse domain scopes (`playback`, `authoring`, `admin`); ROADMAP.md's phase note names "rate limits" as something phase planning must define, but no number was ever proposed or discussed.
   - What's unclear: whether limits should differ per scope (e.g., `admin` more permissive, `playback` tightly bounded given its latency sensitivity) or be a single global per-key limit.
   - Recommendation: propose a conservative default (e.g., 60 req/min per key, burst 10) as a starting point in the `api` config concern, adjustable per D-06's existing writable-key precedent; confirm with the user during planning rather than treating as pre-locked.

3. **What exact fields does "redacted audit details" (API-06) redact, beyond the obvious (API keys, tokens)?**
   - What we know: D-16 requires actor/source/correlation/outcome/redacted-details; no specific redaction list was discussed.
   - What's unclear: whether show content itself (fixture addresses, deployment network config) needs any redaction, or whether redaction is scoped narrowly to credential-shaped values only.
   - Recommendation: start narrow (strip anything matching the request's own `Authorization` header and any field name containing `key`/`token`/`secret`/`password`) and let discuss-phase/UAT widen the list if a real sensitive field surfaces.

4. **Default listen port for the API server?**
   - What we know: no existing TCP listener precedent exists anywhere in this codebase (the only prior "no daemon/IPC listener of any kind existed" precedent, per `internal/artnet/ipc/server.go`'s own doc comment, was for the named-pipe/Unix-socket IPC transport, not TCP).
   - What's unclear: what port to default to; must avoid colliding with Art-Net's own UDP `0x1936`/6454 (different protocol/port space, so no actual conflict, but worth picking something memorable and documenting it in the new `api` config concern).
   - Recommendation: pick and document one fixed default (e.g. `4590`) in `config/api.toml`, following the exact same "committed default, locked unless the writable-key precedent applies" pattern `config/runtime.toml` already uses.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|--------------|-----------|---------|----------|
| Go module proxy (network) | One-time `go get` to pin chi/huma/x-time (see "Pinning a new Go dependency") | ✓ (verified live during this research session) | proxy.golang.org, current | None needed — this is a one-time online contributor action, not a runtime dependency |
| Loopback TCP binding on the target OS | D-06/D-07's API server | ✓ (standard OS capability, no special provisioning) | — | — |
| `modernc.org/sqlite` (pure-Go, no CGo) | Audit table (D-16) | ✓ (already a direct dependency, v1.54.0) | v1.54.0 | — |

**Missing dependencies with no fallback:** none — every dependency this phase needs is either already present in `go.mod` or a standard, easily verified Go-ecosystem addition.
**Missing dependencies with fallback:** none applicable.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` (this repo does not actually use `testify`/`rapid` despite `.planning/research/STACK.md`'s original aspirational recommendation — verified by grep: no `stretchr/testify` or `pgregory.net/rapid` import exists anywhere in `internal/`) |
| Config file | none — plain `go test` |
| Quick run command | `mage testquick` (maps to the `"testquick"` scope target in `magefiles/magefile.go`) |
| Full suite command | `mage test` (maps to `"test"`) |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|---------------------|--------------|
| API-01 | HTTP mutation produces the same outcome as the equivalent CLI/Wails call (parity) | integration | `go test ./internal/api/... -run TestParity -v` | ❌ Wave 0 |
| API-02 | Generated OpenAPI doc has no drift from committed snapshot | unit (mirrors `internal/contracts.CheckDrift`) | `go test ./internal/api/... -run TestOpenAPIDrift -v` | ❌ Wave 0 |
| API-03 | Client reconnecting with a stale `Last-Event-ID` past the buffer window receives a resync signal, not silently-missing events | unit | `go test ./internal/api/... -run TestSSEGapRecovery -v` | ❌ Wave 0 |
| API-04 | Stale `If-Match` returns 412; dry-run mutates nothing; batch either fully applies or fully rolls back | unit + integration | `go test ./internal/api/... -run TestRevision|TestDryRun|TestBatchAtomic -v` | ❌ Wave 0 |
| API-05 | Server binds loopback-only when `api.remote_enabled` is unset/false; remote bind requires the flag AND a valid scoped key | unit | `go test ./internal/api/... -run TestBindAddress|TestAuth -v` | ❌ Wave 0 |
| API-06 | Every mutation writes exactly one `audit_log` row with the actor/source/correlation/outcome/redacted fields populated | unit + integration | `go test ./internal/show/... -run TestAuditLog -v` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `mage testquick`
- **Per wave merge:** `mage test`
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `internal/api/` package and its test files do not exist yet — this entire phase is Wave 0 for test infrastructure.
- [ ] `internal/show/audit_test.go` — covers the new `audit_log` table's write/read path (API-06), following `internal/show/store_test.go`'s existing style.
- [ ] Framework install: none — plain `testing`, no new test framework needed.

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|----------------|---------|--------------------|
| V2 Authentication | Yes | Per-token scoped API keys, `crypto/rand`-generated, SHA-256-hashed at rest (D-05); expiry enforced server-side on every request |
| V3 Session Management | Partially | API keys are bearer tokens, not sessions — no cookie/session-fixation surface; SSE connections are long-lived but re-authenticate at connect time only (standard SSE limitation — document that revoking a key does not immediately close an already-open stream unless the plan adds an explicit revocation-check tick) |
| V4 Access Control | Yes | Coarse domain scope check (D-08) on every mutating route; D-11/D-12 deliberately do *not* scope-gate the SSE stream itself — document this as an intentional, reviewed exception, not an oversight |
| V5 Input Validation | Yes | Huma's typed request structs give schema-level validation for free (required fields, basic type/format checks); domain-level validation still happens inside `internal/command`'s existing handlers (unchanged) |
| V6 Cryptography | Yes | `crypto/rand` for key generation, `crypto/sha256` for at-rest hashing (stdlib only, never hand-rolled) — never store or log a raw API key after mint-time |
| V7 Error Handling and Logging | Yes | Huma's typed error responses (`huma.Error400BadRequest` etc.) map `ExitCode` 0/1/2 to typed HTTP problem responses; audit log is separate durable truth from `slog` diagnostics (D-16) |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|------------------------|
| Remote binding accidentally exposing the API beyond intended scope | Information Disclosure / Elevation of Privilege | Enforce loopback-by-default at bind time (Pitfall 4), not just in documentation; `api.remote_enabled` must be an explicit, reviewed config flag |
| API key leaked via logs/audit table | Information Disclosure | Never log or store the raw key past mint-time; audit `redacted_details` strips anything key/token/secret-shaped before write (Open Question 3) |
| Path-traversal via a client-supplied show path | Tampering / Elevation of Privilege | Server always injects its own fixed `--show <daemonShowPath>`; never accept a client-supplied path (Pitfall 3) |
| Batch endpoint used to bypass per-mutation revision checks by racing many requests | Tampering | Batch-level `If-Match` checked once at batch start and re-verified per-step in the pre-validate-then-commit fallback (Pitfall 1, option 2); the State-in/State-out design (option 1) is immune by construction since the whole batch shares one in-memory `State` snapshot |
| SSE stream exposing authoring/config events to a narrowly-scoped playback-only key | Information Disclosure | Explicitly accepted by D-11 as a deliberate design choice, not a bug — document this clearly in the OpenAPI/AsyncAPI-equivalent description so integrators aren't surprised |
| Rate-limit exhaustion / DoS from a compromised or misbehaving key | Denial of Service | Per-key `golang.org/x/time/rate` token bucket (Open Question 2); revocation (D-05) is the immediate remediation path for a compromised key |
| Handler panic taking down the daemon (which also owns the playback engine and Art-Net worker) | Denial of Service | `chi/middleware.Recoverer` on every route — a panicking HTTP handler must never crash the process that also owns ARTN-04's deterministic frame pipeline |

## Sources

### Primary (HIGH confidence)
- `internal/command/router.go`, `internal/command/pool.go`, `internal/command/artnet.go` (this repository) — command execution authority, existing arg-parsing shape, plan/apply precedent
- `internal/artnet/daemon.go`, `internal/artnet/ipc/server.go`, `internal/artnet/ipc/types.go` (this repository) — daemon lifecycle, existing IPC listener discipline, "no analog found" precedent for a new listener type
- `internal/wails/app.go`, `internal/wails/events.go` (this repository) — supervised-daemon spawn/dial pattern (D-07), throttled-hint-stream anti-pattern precedent (informs D-10)
- `internal/show/schema.go`, `internal/show/store.go` (this repository) — `.golc` SQLite store, `Revision` field (informs D-13), multi-process contention/`busy_timeout` precedent (informs Pitfall 2)
- `internal/contracts/generate.go` (this repository) — generate+`CheckDrift` pattern D-03 mirrors
- `internal/projectconfig/registry.go`, `internal/projectconfig/model.go`, `config/runtime.toml` (this repository) — five-layer config resolution, locked-vs-writable key precedent (informs D-06/Pitfall 4)
- `internal/bootstrap/cache.go`, `internal/bootstrap/engine.go`, `config/toolchain.toml` (this repository) — offline-buildable toolchain pinning model, `GOLC_BOOTSTRAP_LOCK_MUTATION` guard (informs "Pinning a new Go dependency")
- `go.mod`/`go.sum` (this repository, read directly) — confirmed current dependency set, confirmed `golang.org/x/crypto` and `labstack/echo` already present only as indirect Wails dependencies
- Live `go list -m -versions` / module-proxy `.info` queries against `proxy.golang.org`, executed 2026-07-24 during this research session — chi v5.3.1, huma v2.39.0 (released 2026-07-15), golang.org/x/time v0.15.0, gorilla/mux v1.8.1 (stale)

### Secondary (MEDIUM confidence)
- `.planning/research/STACK.md` (this repository's own day-one project research, 2026-07-17) — pre-selected Huma v2.39.0 + Chi v5.3.1 for exactly this phase's API, with citations to `huma.rocks`, `pkg.go.dev/github.com/go-chi/chi/v5`
- [huma.rocks — Server Sent Events (SSE)](https://huma.rocks/features/server-sent-events-sse/) — SSE registration API shape, OpenAPI-typed event payloads
- [huma.rocks — Bring Your Own Router](https://huma.rocks/features/bring-your-own-router/) and [pkg.go.dev humachi](https://pkg.go.dev/github.com/danielgtaylor/huma/v2/adapters/humachi) — confirms Huma rides on Chi via an adapter, doesn't replace D-04's router choice
- [pkg.go.dev chi/v5/middleware](https://pkg.go.dev/github.com/go-chi/chi/v5/middleware) — confirms no built-in ETag/If-Match middleware, `net/http`-compatible middleware chain

### Tertiary (LOW confidence)
- General WebSearch results on Stripe/GitHub-style API-key hashing conventions (SHA-256 for high-entropy tokens vs. bcrypt for passwords) — well-established industry practice, not sourced from a single authoritative spec; flagged in Assumptions Log where relevant

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — chi/huma/x-time versions verified live against the Go module proxy in this session; SHA-256-for-tokens and rate-limiting choices are well-established practice but not codebase-verified (flagged `[ASSUMED]` in Assumptions Log)
- Architecture: HIGH — every pattern is grounded in direct reads of this repository's own `internal/command`, `internal/artnet`, `internal/show`, `internal/contracts`, and `internal/projectconfig` source, not generic API-design advice
- Pitfalls: HIGH for Pitfalls 1–3 (directly derived from reading actual handler code and store internals); MEDIUM for Pitfall 4 (loopback-enforcement pattern is a reasonable extrapolation from `internal/projectconfig`'s existing locked/writable model, not yet proven against a real `api` concern)

**Research date:** 2026-07-24
**Valid until:** 30 days (Go dependency versions move quickly; re-verify chi/huma/x-time versions with `go list -m -versions` if planning is deferred past late August 2026)

---
*Phase: 7-Versioned External Control API*
*Research completed: 2026-07-24*
