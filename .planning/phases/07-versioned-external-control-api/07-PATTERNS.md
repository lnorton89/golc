# Phase 7: Versioned External Control API - Pattern Map

**Mapped:** 2026-07-24
**Files analyzed:** 15 (new `internal/api` package files per RESEARCH.md's recommended structure, plus the `internal/show` audit table and `internal/projectconfig` `api` concern)
**Analogs found:** 15 / 15 (all resolved against real, currently-committed code — this phase has no true "no analog" gap; RESEARCH.md already did the analog-finding legwork and this file makes it concrete with line numbers)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|--------------------|------|-----------|-----------------|----------------|
| `internal/api/router.go` (Chi + humachi wiring, `/v1` mount) | route | request-response | `internal/command/router.go` (`CommandRegistry`/`Lookup`/`Execute`) | role-match (dispatch shape, not HTTP-specific) |
| `internal/api/translate.go` (HTTP → `command.Request` translation) | controller | request-response | `internal/command/pool.go` (`runPoolCreate`, `parsePoolCreateArgs`) | exact (this *is* the arg-shape the translator must produce) |
| `internal/api/server.go` (`http.Server` construction, graceful `Shutdown`) | service | request-response | `internal/artnet/ipc/server.go` (`Serve`, `NewListener`) | exact (same ctx-cancel-closes-listener discipline, different transport) |
| `internal/api/events.go` (SSE ring buffer, `Last-Event-ID` replay, broadcaster) | service | streaming | `internal/wails/events.go` (`EventPusher`, hint-stream anti-pattern doc) | role-match (only existing streaming/push precedent in repo) |
| `internal/api/auth.go` (API-key hash lookup, scope check, expiry) | middleware | request-response | `internal/artnet/ipc/server.go`'s `handleConn` decode-then-dispatch gate | partial (closest existing "gate before handler runs" shape; no prior auth code exists) |
| `internal/api/ratelimit.go` (per-key token bucket) | middleware | request-response | none in-repo — new infrastructure (`golang.org/x/time/rate`) | no analog |
| `internal/api/revision.go` (`If-Match` → `show.State.Revision`, 412) | middleware/utility | request-response | `internal/show/store.go` (`Revision` field lifecycle) — see below | role-match (revision semantics only, no HTTP precedent) |
| `internal/api/dryrun.go` (`?dry_run=true` plan/no-mutate) | service | request-response | `internal/command/pool.go` (`runPoolUpdate`/`runPoolApply` plan/apply split) | exact |
| `internal/api/batch.go` (`/v1/batch` atomic orchestration) | service | transform/batch | `internal/command/pool.go` (Load→mutate→Save shape, to be extracted into State-in/State-out) | role-match (no existing batch/atomicity precedent — Pitfall 1 in RESEARCH.md) |
| `internal/api/keys.go` (API-key CRUD routes, self-registers own `internal/command` route) | controller | CRUD | `internal/command/pool.go` (`MustDeclareScope`/`MustDeclareRoute` self-registration) | exact |
| `internal/api/audit.go` (post-mutation `audit_log` writer) | service | event-driven | `internal/show/store.go` (`stageRecoveryPoint`/transaction discipline) | exact |
| `internal/api/generate.go` (OpenAPI doc generation entrypoint) | utility/config | transform | `internal/contracts/generate.go` (`RegisterSchema`/`GenerateAll`/`CheckDrift`) | exact (explicitly named analog in RESEARCH.md D-03) |
| `internal/show/schema.go` (add `audit_log` table to `createTablesSQL`) | model/migration | CRUD | `internal/show/schema.go` itself, `recovery_points` table definition | exact (same file, same append-only convention) |
| `internal/projectconfig` `api` concern (`config/api.toml` + registry entry) | config | request-response | `internal/projectconfig/registry.go` (`DefaultRegistry`, `runtime.log_level` writable-key precedent) | exact |
| `internal/artnet.Run` (wire `internal/api` server into daemon start/stop ordering) | service | event-driven | `internal/artnet/daemon.go` (`Run`, ordered `engine.Start`→`ifaceMgr.Start`→`startWorkerLocked`→`ipc.Serve`, reverse-order stop) | exact |

## Pattern Assignments

### `internal/api/router.go` and `internal/api/translate.go` (controller, request-response)

**Analog:** `internal/command/router.go` + `internal/command/pool.go`

**Core dispatch pattern** (`internal/command/router.go` lines 174-187):
```go
// Execute routes one invocation and runs its handler. Unroutable
// invocations fail with a stable diagnostic and exit code 2.
func (r *CommandRegistry) Execute(request Request) Result {
	registration, rest, ok := r.Lookup(request.Args)
	if !ok {
		diagnostic := fmt.Sprintf("GOLC_ROUTE_UNKNOWN: no registered route matches %q\n", strings.Join(request.Args, " "))
		return Result{ExitCode: 2, Stderr: []byte(diagnostic)}
	}
	return registration.Handler(Request{
		Route: registration.Route,
		Args:  rest,
		Root:  request.Root,
	})
}
```
Every REST handler in `internal/api/translate.go` must build a `command.Request{Route, Args, Root}` and call this `Execute` — never call `internal/pool`, `internal/scene`, etc. directly (RESEARCH.md D-01, Anti-Pattern "A second command-dispatch surface").

**Self-registration convention any new `internal/command` route (e.g. `api-key create`, backing `keys.go`) must follow** (`internal/command/pool.go` lines 36-64):
```go
var _ = MustDeclareScope(ScopeRegistration{
	Scope:   "pool",
	Summary: "Logical fixture pool definitions, independent of concrete count/address/hardware.",
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "pool create",
	Summary: "Create a named logical pool against a ShowState document: pool create <name> [--requires <cap1,cap2,...>] --show <path>.",
	Handler: runPoolCreate,
})
```

**Load → mutate → Save + arg-parsing shape a translator's target handler already has** (`internal/command/pool.go` lines 78-141):
```go
func runPoolCreate(request Request) Result {
	name, showPath, requires, err := parsePoolCreateArgs("pool create <name> [--requires <cap1,cap2,...>] --show <path>", request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}
	state, err := show.Load(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	newPool, err := pool.NewPool(name, requires)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	state.Pools = append(state.Pools, newPool)
	if err := show.Save(request.Root, showPath, state); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	return Result{Stdout: []byte(fmt.Sprintf("GOLC_POOL_CREATED: %s (%s)\n", newPool.Name, newPool.ID))}
}
```
The `--show <path>` argument here is exactly why `internal/api/translate.go` must inject the daemon's own fixed show path server-side and never accept a client-supplied one (Pitfall 3).

**Error handling / exit-code convention:** ExitCode 0 = success, 1 = handler-level failure, 2 = malformed/unroutable invocation — `translate.go`'s `translateResult` maps these three onto Huma typed HTTP errors (201/200, domain 4xx/5xx, 400) per RESEARCH.md Pattern 1.

---

### `internal/api/server.go` (service, request-response)

**Analog:** `internal/artnet/ipc/server.go`

**ctx-cancellation-closes-listener pattern** (lines 56-79):
```go
func Serve(ctx context.Context, listener net.Listener, handler Handler) error {
	closeOnce := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = listener.Close()
		case <-closeOnce:
		}
	}()
	defer close(closeOnce)

	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				return fmt.Errorf("GOLC_ARTNET_IPC_ACCEPT_FAILED: %v", err)
			}
		}
		go handleConn(conn, handler)
	}
}
```
`internal/api/server.go` should mirror this discipline via `http.Server.Shutdown` (RESEARCH.md's own "Graceful shutdown" code example already adapts this exact pattern for `net/http`) — same "ctx.Done() triggers listener/server close, in-flight work drains" shape, different API (`Shutdown(ctx)` vs `listener.Close()`).

**Request/response frame + strict decode/encode discipline** (lines 86-127) — informs `internal/api/auth.go`'s "decode/validate before dispatch, always answer with a typed result even on decode failure" gate, since this is the only existing "gate before handler runs" precedent in the repo:
```go
func handleConn(conn net.Conn, handler Handler) {
	defer conn.Close()
	payload, err := readFrame(conn)
	if err != nil {
		_ = writeResult(conn, Result{ExitCode: 1, Stderr: []byte(
			fmt.Sprintf("GOLC_ARTNET_IPC_DECODE_FAILED: %v\n", err))})
		return
	}
	var request Request
	if err := strictjson.DecodeStrict(payload, &request); err != nil {
		_ = writeResult(conn, Result{ExitCode: 1, Stderr: []byte(
			fmt.Sprintf("GOLC_ARTNET_IPC_DECODE_FAILED: %v\n", err))})
		return
	}
	result := handler(request)
	_ = writeResult(conn, result)
}
```

---

### `internal/artnet.Run` modification (wiring `internal/api` into daemon lifecycle)

**Analog:** `internal/artnet/daemon.go` lines 165-225

```go
func Run(ctx context.Context, cfg Config) error {
	engine, err := playback.NewEngine(cfg.State)
	...
	engine.Start(ctx)

	ifaceMgr := NewInterfaceManager(cfg.InterfaceIndex, cfg.InterfaceName)
	ifaceMgr.Start(ctx)
	...
	d.mu.Lock()
	d.startWorkerLocked()
	d.mu.Unlock()

	listener, err := ipc.NewListener(pipeNameOrDefault(cfg))
	if err != nil {
		d.mu.Lock()
		d.stopWorkerLocked()
		d.mu.Unlock()
		ifaceMgr.Stop()
		engine.Stop()
		return fmt.Errorf("GOLC_ARTNET_DAEMON_IPC_LISTEN_FAILED: %v", err)
	}

	serveErr := ipc.Serve(ctx, listener, d.handle)

	d.mu.Lock()
	d.stopWorkerLocked()
	d.mu.Unlock()
	ifaceMgr.Stop()
	engine.Stop()

	return serveErr
}
```
D-07 requires the API server to start after `ipc.Serve` is up and stop before it during shutdown, as one more subsystem in this same ordered start/reverse-order-stop sequence — add `apiSrv.Start(ctx)` after `listener` is created and before/alongside `ipc.Serve`, and `apiSrv.Shutdown(...)` in the same stop block as `ifaceMgr.Stop()`/`engine.Stop()`, following this file's own error-unwind-on-partial-failure discipline (each failed step tears down everything already started, in reverse order, before returning).

---

### `internal/api/dryrun.go` (service, request-response)

**Analog:** `internal/command/pool.go`'s `pool update`/`pool apply` plan/apply split (doc comment lines 1-10, route declarations lines 47-64)

```go
var _ = MustDeclareRoute(CommandRegistration{
	Route: "pool update",
	Summary: "Compute and write/print a deterministic pool impact-review plan without mutating the ShowState document: ...",
	Handler: runPoolUpdate,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route: "pool apply",
	Summary: "Validate (integrity then freshness) and atomically apply an already-reviewed pool impact plan: ...",
	Handler: runPoolApply,
})
```
`?dry_run=true` on a mutating REST endpoint should call the "compute plan, don't mutate" side of a handler pair where one exists (pools already have exactly this shape) — reuse, do not build a second preview implementation (RESEARCH.md Pattern 4 anti-pattern).

---

### `internal/api/audit.go` (service, event-driven) and `internal/show/schema.go` (model, CRUD — add `audit_log` table)

**Analog:** `internal/show/schema.go` (full file read; audit table must extend `createTablesSQL`, lines 42-62)

```go
const createTablesSQL = `
CREATE TABLE IF NOT EXISTS show_meta (
  id             INTEGER PRIMARY KEY CHECK (id = 1),
  schema_version INTEGER NOT NULL,
  revision       INTEGER NOT NULL,
  checksum       TEXT    NOT NULL,
  updated_at     TEXT    NOT NULL
);

CREATE TABLE IF NOT EXISTS show_state (
  id   INTEGER PRIMARY KEY CHECK (id = 1),
  blob BLOB NOT NULL
);

CREATE TABLE IF NOT EXISTS recovery_points (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  created_at TEXT    NOT NULL,
  revision   INTEGER NOT NULL,
  blob       BLOB    NOT NULL
);
`
```
`recovery_points` is the direct analog for the new `audit_log` table: append-only, `AUTOINCREMENT` id, `IF NOT EXISTS`. Add `audit_log` as a fourth table in this same block (RESEARCH.md Pattern 5 already drafts its exact DDL).

**Connection/PRAGMA discipline `audit.go`'s writer must reuse, not duplicate** (`internal/show/schema.go` lines 75-161, `openStore`): single-writer (`db.SetMaxOpenConns(1)`), `busy_timeout` applied FIRST before `journal_mode=WAL`, `application_id` door check. Anti-pattern explicitly called out in RESEARCH.md: "Opening a second, uncoordinated SQLite connection for audit writes" — `audit.go` must route writes through `internal/show`'s existing store machinery (or an explicit exported audit-write function added to that package), never `sql.Open` a second connection to the same `.golc` file.

---

### `internal/api/keys.go` and API-key self-registration

**Analog:** same `MustDeclareScope`/`MustDeclareRoute` block shown above (`internal/command/pool.go` lines 36-64) — an `api-key create` (or similar) `internal/command` route backing the REST `keys.go` handler must self-register the same way, giving it CLI reachability too and keeping `internal/command` the single execution authority (D-01).

---

### `internal/api/generate.go` (utility/config, transform)

**Analog:** `internal/contracts/generate.go`

**Self-registering schema/descriptor pattern** (lines 33-95):
```go
type SchemaDescriptor struct {
	Name       string
	OutputPath string
	Schema     func() *jsonschema.Schema
}

func RegisterSchema(descriptor SchemaDescriptor) error {
	name := strings.TrimSpace(descriptor.Name)
	if name == "" {
		return fmt.Errorf("GOLC_CONTRACTS_NAME_EMPTY: schema descriptor name is blank")
	}
	...
	if _, exists := registry.names[name]; exists {
		return fmt.Errorf("GOLC_CONTRACTS_NAME_DUPLICATE: schema %q is already registered", name)
	}
	...
	registry.byName = append(registry.byName, descriptor)
	return nil
}
```
`internal/api/generate.go`'s OpenAPI generation entrypoint mirrors this generate+`CheckDrift` discipline (byte-stable output, read-only drift check that never rewrites committed bytes, `GENERATED ... DO NOT EDIT` marker convention at line 31) — but driven by Huma's reflection over Go request/response structs instead of `invopop/jsonschema` directly (D-03).

---

### `internal/projectconfig` `api` concern (config, request-response)

**Analog:** `internal/projectconfig/registry.go`

**Writable-key precedent the new `api.remote_enabled` key must follow** (lines 65-94):
```go
func DefaultRegistry() Registry {
	fields := map[string]FieldSpec{
		"runtime.log_level": {
			Locked:        false,
			AllowedValues: []string{"debug", "error", "info", "warn"},
			EnvVar:        "GOLC_RUNTIME_LOG_LEVEL",
			CLIFlag:       "--log-level",
		},
	}
	for _, concern := range DefaultSpec().Concerns {
		for key := range concern.Keys {
			if _, declared := fields[key]; declared {
				continue
			}
			fields[key] = FieldSpec{Locked: true}
		}
	}
	...
	return Registry{Fields: fields}
}
```
Every canonical key defaults to `Locked: true` unless explicitly declared otherwise, exactly like `runtime.log_level` above — `api.remote_enabled` needs its own `Locked: false` entry here (Pitfall 4: loopback-default must be enforced at bind time, and an operator needs to actually be able to flip this key without `GOLC_CONFIG_LOCKED_OVERRIDE`). The new `api` concern's key registry itself belongs in `internal/projectconfig`'s `DefaultSpec()` (model.go/decode.go), following `config/runtime.toml`'s existing shape (not read in this pass — same pattern class as `runtime.log_level`, follow `config/runtime.toml` directly when implementing).

## Shared Patterns

### Self-registration idiom (route/scope declarations, schema descriptors)
**Source:** `internal/command/router.go` lines 214-248 (`MustDeclareRoute`/`MustDeclareScope`), mirrored by `internal/contracts/generate.go` lines 97+ (`MustRegisterSchema`)
**Apply to:** `internal/api/keys.go`'s backing `internal/command` route(s), `internal/api/generate.go`'s OpenAPI operation registrations — every new "list of things a later file can add to without editing a central switch" surface in this phase should follow this same package-level `var _ = MustDeclare...(...)` idiom already established twice in this repo.

### Load → mutate → Save + Revision lifecycle
**Source:** `internal/command/pool.go` lines 78-98 (`show.Load`/`show.Save`), `internal/show/schema.go`'s `show_meta.revision` column (line 46)
**Apply to:** `internal/api/translate.go` (every mutating handler), `internal/api/revision.go` (`If-Match` against `show.State.Revision`), `internal/api/batch.go` (must NOT loop this per sub-request — see Pitfall 1 in RESEARCH.md; needs a State-in/State-out extraction or pre-validate-then-commit fallback, there is no existing "load once, mutate many, save once" helper anywhere in `internal/command` today).

### Single-writer SQLite connection discipline
**Source:** `internal/show/schema.go` lines 85-122 (`db.SetMaxOpenConns(1)`, `busy_timeout` ordering, WAL/synchronous/foreign_keys PRAGMAs)
**Apply to:** `internal/api/audit.go` — must route through `internal/show`'s existing store machinery, never open a second `sql.Open` connection to the same `.golc` file.

### ctx-cancellation-driven graceful shutdown
**Source:** `internal/artnet/ipc/server.go` lines 56-79 (`Serve`'s `ctx.Done()` closes listener, unblocks `Accept`, returns nil)
**Apply to:** `internal/api/server.go`'s `http.Server.Shutdown` wiring, and `internal/artnet.Run`'s ordered start/reverse-order stop sequence that must now include the API server as one more subsystem.

### Stable diagnostic/error-code convention
**Source:** `internal/command/router.go` lines 174-187 (`GOLC_ROUTE_UNKNOWN`, ExitCode 0/1/2), `internal/show/schema.go` (`GOLC_SHOW_STATE_INVALID`, `GOLC_SHOW_NOT_GOLC_FORMAT`)
**Apply to:** every new file in `internal/api` — errors should carry a stable `GOLC_API_*`-prefixed identifier (e.g. `GOLC_API_LISTEN_FAILED` already used in RESEARCH.md's own example), matching this repo's existing all-caps-underscore diagnostic convention rather than inventing a new error-shape convention for HTTP.

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `internal/api/ratelimit.go` | middleware | request-response | No prior rate-limiting code exists anywhere in the repo; RESEARCH.md proposes `golang.org/x/time/rate` per-key token bucket as new infrastructure — planner should treat this as greenfield, following the library's own documented usage rather than an in-repo analog. |
| `internal/api/events.go`'s SSE ring buffer / replay-on-gap logic | service | streaming | `internal/wails/events.go`'s `EventPusher` is a same-process Go→WebView push channel, not a network-facing replayable stream — useful for its "hint stream, never authoritative" doc-comment precedent (informs D-10's anti-pattern avoidance) but structurally not a close analog for the ring-buffer/`Last-Event-ID` mechanics themselves, which are new. |

## Metadata

**Analog search scope:** `internal/command/`, `internal/artnet/` (incl. `internal/artnet/ipc/`), `internal/show/`, `internal/projectconfig/`, `internal/contracts/`, `internal/wails/` (referenced via RESEARCH.md, not independently reread — its `events.go` anti-pattern doc comment was already extracted verbatim by RESEARCH.md's own Sources section)
**Files scanned:** `internal/command/router.go`, `internal/command/pool.go`, `internal/artnet/daemon.go`, `internal/artnet/ipc/server.go`, `internal/show/schema.go`, `internal/projectconfig/registry.go`, `internal/contracts/generate.go` (plus directory listings of `internal/command/*.go`, `internal/artnet/**/*.go`, `internal/show/*.go`, `internal/projectconfig/*.go`, `internal/contracts/*.go`)
**Pattern extraction date:** 2026-07-24
