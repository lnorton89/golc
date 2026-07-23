# Phase 5: Durable Shows and Recovery - Research

**Researched:** 2026-07-23
**Domain:** Embedded SQLite persistence for a single-file, single-writer desktop show format (Go, Windows-first)
**Confidence:** HIGH

## Summary

Phase 5 replaces `internal/show.State`'s JSON file I/O with a SQLite-backed `.golc` file while keeping the domain struct, `validate()`, and every command call site untouched (CONTEXT D-01–D-03). The correct driver is **`modernc.org/sqlite`** — a pure-Go, CGo-free `database/sql` driver already picked at HIGH confidence in this project's own prior stack research (`.planning/research/STACK.md`) and reconfirmed this session against the Go module proxy (latest `v1.54.0`, bundling SQLite `3.53.2`). Pure-Go avoids adding a `gcc`/CGo build dependency to a project whose only qualified release target is Windows (Phase 10) and whose CI must stay reproducible without a pinned C toolchain — even though this dev machine already has MSYS2 `gcc` available, that is a local convenience, not a CI guarantee.

The store itself stays exactly the shape CONTEXT.md locked: one `show_state` table holding the canonical JSON blob (`strictjson.CanonicalEncode` bytes, unchanged), one singleton `show_meta` table (`schema_version`, `revision`, checksum), and one `recovery_points` table capped at 5 rows, pruned oldest-first in the same transaction as every save (D-04–D-06). Durability comes from `PRAGMA journal_mode=WAL` + `PRAGMA synchronous=FULL` (this app's writes are per-command, not per-frame, so the extra fsync cost of FULL is negligible and buys power-loss durability, not just crash durability) plus `PRAGMA foreign_keys=ON`. Verified backups use `VACUUM INTO` (simpler than the raw Online Backup API at this file's KB-to-low-MB scale) followed by a fresh read-back connection running `strictjson.DecodeStrict` + `show.validate()` — this is D-09's exact requirement, not a checksum shortcut. Windows atomic replace continues to use `os.Rename` (Go's Windows implementation already wraps `MoveFileEx` with `MOVEFILE_REPLACE_EXISTING|MOVEFILE_COPY_ALLOWED`, so it overwrites like POSIX rename) — the only new constraint is that every connection to the destination path must be closed first, and any stray `-wal`/`-shm` sidecars for the destination must be removed alongside the swap.

One important structural finding: this application invokes each show-mutating command as a **fresh short-lived CLI process** (`golc pool create ...` loads, mutates, saves, exits — there is no long-lived show daemon the way Phase 4's Art-Net worker has one). That means every single command execution opens a *new* SQLite connection to the same `.golc` file. This is exactly SQLite's recommended "one writer, short transaction" pattern, but it also means WAL/SHM sidecar files will appear and disappear across hundreds of short-lived process invocations over a show's editing lifetime — expected behavior, not corruption, but something the migration/diagnose code and its tests must account for explicitly (see Pitfalls).

**Primary recommendation:** Use `modernc.org/sqlite` (pure Go) with `WAL` + `synchronous=FULL` + `foreign_keys=ON`; store the show as one blob table + one metadata table + one recovery-points table exactly as CONTEXT.md locked; back up with `VACUUM INTO` + read-back-and-validate; do **not** adopt a SQL-migration framework (`goose`) for schema evolution — write a small ordered Go function registry keyed by `schema_version` that transforms the decoded blob, since this file's "schema" is a blob shape, not a set of SQL tables that DDL migrations are built for.

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| SHOW-01 | A user can save a complete show and its deployment data as one portable versioned `.golc` file. | Standard Stack (SQLite driver + schema); Architecture Patterns (store package, single-blob-plus-metadata schema); Code Examples (Open/Save). |
| SHOW-02 | A user can open, save, and save-as a show without stopping deterministic output unexpectedly. | Architectural Responsibility Map (storage stays in the command/storage layer, never touches `playback.Engine`); Pitfalls (per-invocation connection lifecycle; Windows atomic replace). |
| SHOW-03 | The application autosaves recoverable authoring changes without performing storage work in the playback timing path. | Architecture Patterns (recovery-point write inside the same transaction as the command's save, D-04); Architectural Responsibility Map. |
| SHOW-04 | A user can recover from an interrupted or failed session using clearly identified rotating recovery points. | Architecture Patterns (`recovery_points` table schema + pruning); Durability settings (WAL+FULL guarantees no loss across app crash). |
| SHOW-05 | Schema migration creates a verified backup, applies atomically, and refuses unsupported newer formats without rewriting them. | Standard Stack (`VACUUM INTO` for backup); Code Examples (migrate flow); Common Pitfalls (goose-vs-hand-rolled migration mismatch; newer-format hard refusal); Migration support window discussion. |
| SHOW-06 | A user can run integrity diagnostics and export a versioned human-readable JSON representation for interchange and troubleshooting. | Standard Stack (`PRAGMA integrity_check` performance at this app's scale); Code Examples (diagnose + export reusing `strictjson.CanonicalEncode`). |
</phase_requirements>

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Storage Format**
- D-01: `.golc` is a SQLite database file, not a JSON envelope.
- D-02: `internal/show.State` stays the in-memory/domain shape. SQLite replaces only `show.Load`/`show.Save`'s disk-I/O internals.
- D-03: Inside SQLite, the whole `State` document is stored as one serialized blob (the same canonically-encoded bytes `strictjson.CanonicalEncode` already produces) plus a small metadata table (`schema_version`, `revision`, checksum).

**Autosave & Recovery Points**
- D-04: Every command mutation triggers the recovery-point write, inside the same SQLite transaction as the save.
- D-05: Rotating recovery points live in a separate table inside the same `.golc` file.
- D-06: Keep the last 5 recovery points, oldest pruned on insert.
- D-07: Recovery is auto-detected and offered on next open, never silently applied.

**Migration & Backup Policy**
- D-08: Schema migration runs automatically on open, with confirmation. Backup first, then migrate in a transaction, replace working file only on full success.
- D-09: A migration backup is "verified" via read-back + whole-State `validate()`, not checksum-only.
- D-10: Opening a `.golc` with a newer-than-supported `schema_version` is a hard refusal for editing/playback, read-only inspect allowed, file never rewritten.

**Integrity Diagnostics & JSON Export**
- D-11: Integrity diagnostics run structural `validate()` plus SQLite-level `PRAGMA integrity_check`.
- D-12: Diagnostics run on-demand only, not automatically on open.
- D-13: SHOW-06's JSON export is the same shape as today's canonical State JSON — literally `strictjson.CanonicalEncode(State)`.

### Claude's Discretion
None — every gray area discussed converged on the recommended option; no "you decide" selections were made in the discussion session. (This research session's own open questions — driver pin, exact PRAGMA values, backup mechanism, migration-window policy — are the "protocol-level implementation detail" CONTEXT.md explicitly deferred to phase research/planning; see Open Questions below for where planner judgment is still needed.)

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope.
</user_constraints>

## Project Constraints (from CLAUDE.md)

No `./CLAUDE.md` or `./.claude/CLAUDE.md` exists in this repository (`.planning/config.json` sets `claude_md_path: "./.claude/CLAUDE.md"` but the file is absent). No project-specific directives to enforce beyond the governing constraint already carried in every phase's CONTEXT/STATE.md: *"UI, persistence, scripts, API, LLM, and Linear never own or block deterministic playback or Art-Net timing."* This constraint is structurally satisfied for this phase because `internal/playback/engine.go`'s `Engine` never calls `show.Load`/`show.Save` — it only ever receives a `State` value via `NewEngine`/`StageEdit` (confirmed by reading `internal/playback/engine.go` usage in CONTEXT.md's Integration Points and cross-checked against `internal/command/playback.go`).

## Architectural Responsibility Map

This is a headless Go CLI application (no Wails UI until Phase 6), so the standard browser/SSR/API/CDN tier table doesn't apply directly. The equivalent tiers for this codebase are the CLI/command layer, the storage layer, the in-memory domain model, and the playback engine.

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| `.golc` file open/save/save-as (SHOW-01/02) | Storage layer (`internal/show` — new SQLite-backed `Load`/`Save`) | CLI/command layer (`internal/command/deployment.go`'s `show` scope) | D-02 locks this: only `Load`/`Save` internals change; command handlers stay callers, not implementers. |
| Autosave / recovery-point write (SHOW-03/04) | Storage layer (same transaction as `Save`) | — | D-04: no separate timer/background-writer; piggybacks on the existing every-command-Save trigger, so it structurally cannot enter the playback timing path. |
| Recovery detection/offer on open (SHOW-04) | CLI/command layer (`show open` handler) | Storage layer (query for rows newer than last clean save) | D-07: surfacing/offering is a command-layer UX decision; the underlying "is there a newer recovery point" fact is a storage-layer query. |
| Schema migration + verified backup (SHOW-05) | Storage layer (`internal/show` migration package) | CLI/command layer (confirmation prompt) | D-08/D-09: the transactional backup-then-migrate-then-swap sequence is pure storage-layer mechanics; only the human confirmation gate is command-layer. |
| Integrity diagnostics (SHOW-06) | Storage layer (`PRAGMA integrity_check` + `show.validate()`) | CLI/command layer (`show diagnose` route, new) | D-11/D-12: on-demand, explicit — a new CLI route calling into storage-layer checks, not a background job. |
| JSON export (SHOW-06) | Storage layer (reuses `strictjson.CanonicalEncode`) | CLI/command layer (`show export` route, new) | D-13: zero new schema; the storage layer already has the decoded `State` in hand after `Load`. |
| Deterministic playback / Art-Net output | Playback engine (`internal/playback/engine.go`) | — | Explicitly out of this phase's scope; confirmed untouched — `Engine` holds `State` via `atomic.Pointer[Frame]`, never calls storage functions. |

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|---------------|
| `modernc.org/sqlite` | v1.54.0 (bundles SQLite 3.53.2) | Pure-Go `database/sql` SQLite driver — the `.golc` file's storage engine | CGo-free: trivially builds on Windows without a pinned `gcc`/MSVC toolchain in CI. Already the HIGH-confidence pick in this project's own `.planning/research/STACK.md` (Context7-sourced against `pkg.go.dev/modernc.org/sqlite`). Version reconfirmed against the Go module proxy this session. Bundled SQLite `3.53.2` is well past the WAL-reset corruption fix landed in `3.51.3` (2026-03-13). [VERIFIED: Go module proxy] [CITED: .planning/research/STACK.md] |

**Version verification performed this session:**
```
go list -m -versions modernc.org/sqlite            # ... v1.54.0 (latest)
go list -m -versions github.com/mattn/go-sqlite3    # ... v1.14.48 (latest, CGo alternative — not selected)
go list -m -versions github.com/ncruces/go-sqlite3  # ... v0.35.2 (pure-Go/WASM alternative — not selected)
go list -m -versions zombiezen.com/go/sqlite        # ... v1.4.2 (pure-Go/WASM alternative — not selected)
go list -m -versions github.com/pressly/goose/v3    # ... v3.27.3 (SQL migration framework — considered, not adopted; see Common Pitfalls)
```
All ran successfully against `https://proxy.golang.org` (the authoritative Go module registry) from this machine — confirms every package name is real and actively released, not hallucinated.

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `strictjson` (already in repo: `internal/strictjson`) | n/a — internal package | `DecodeStrict`/`CanonicalEncode` for the blob payload and JSON export | Already used by `show.Load`/`show.Save` and `internal/command/pool.go`/`linear.go`; D-03/D-13 reuse it verbatim — no new dependency needed. |
| Go standard library `database/sql` | Go 1.26.5 stdlib | Connection/transaction management around the `modernc.org/sqlite` driver | Keeps the store package testable with the standard `sql.DB`/`sql.Tx` interfaces; avoids coupling `internal/show` to driver-specific types except where PRAGMAs/`VACUUM INTO` require raw `Exec` calls (which `database/sql` supports directly — no escape hatch needed for this app's usage pattern). |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `modernc.org/sqlite` (pure Go) | `mattn/go-sqlite3` (CGo) | Requires `CGO_ENABLED=1` + a C compiler in every build/CI environment (TDM-GCC on Windows). This machine happens to have MSYS2 `gcc` installed, but that is not a guarantee for CI runners or contributor machines — pure Go removes the dependency entirely. `mattn/go-sqlite3` is the most battle-tested/widest-used Go SQLite driver historically, so if a future perf problem specifically traces to the transpiled driver, this is the fallback to reconsider. [ASSUMED — package discovered via WebSearch this session, registry-existence-confirmed but not Context7/official-doc-sourced] |
| `modernc.org/sqlite` (pure Go, C-to-Go transpiled) | `ncruces/go-sqlite3` (pure Go, WASM build of real SQLite C source via wazero) | Benchmarks found this session show `ncruces/go-sqlite3` reading faster (147K raw reads/sec vs 38K) and it runs the actual unmodified SQLite C source (bug-for-bug compatible), at the cost of a `wazero` WASM runtime dependency. Not needed at this app's scale (a single small blob table, not a high-throughput relational workload) — `modernc.org/sqlite`'s longer track record and simpler dependency graph win for a v1 release. Revisit only if diagnostics ever show driver overhead mattering. [ASSUMED — WebSearch this session] |
| Hand-rolled Go migration-function registry (this research's recommendation) | `github.com/pressly/goose/v3` (already HIGH-confidence-picked in `.planning/research/STACK.md`) | `goose` is built for SQL-DDL-shaped schema evolution (`CREATE TABLE`, `ALTER TABLE` per migration file). Phase 5's CONTEXT.md D-02/D-03 lock the store to one blob column + one metadata table — future "schema migrations" are almost entirely about transforming the **shape of the decoded Go struct/JSON blob**, not adding SQL columns. Forcing that through `goose`'s SQL-file model would mean writing migrations that mostly `SELECT`/`UPDATE` one blob row with app-level logic anyway, defeating the point of a SQL migration tool. `goose` remains a reasonable fit if this phase's tables ever need real DDL evolution (e.g., adding an index, splitting `recovery_points`) — treat as a **flagged tension with prior project-level research**, not a rejection of `goose` outright. See Open Questions. |

**Installation:**
```bash
go get modernc.org/sqlite@v1.54.0
```

## Package Legitimacy Audit

> The `gsd-tools package-legitimacy check` seam does not support the `go` ecosystem (`npm|pypi|crates` only), so this audit was performed manually against the Go module proxy (`proxy.golang.org` — the authoritative Go module registry) plus release-history inspection. No automated OK/SUS/SLOP verdict is available for Go modules; findings below are manual-verification-based and each package's provenance is tagged accordingly.

| Package | Registry | Age / Release History | Source Repo | Verdict | Disposition |
|---------|----------|------------------------|--------------|---------|-------------|
| `modernc.org/sqlite` | Go module proxy | v1.0.0 → v1.54.0, continuous releases since project inception | gitlab.com/cznic/sqlite (mirrored to github.com/modernc-org/sqlite) | OK (manually verified: long, continuous, active release history; already used by this project's own prior HIGH-confidence stack research) | Approved — primary recommendation |
| `github.com/mattn/go-sqlite3` | Go module proxy | v1.0.0 → v1.14.48, the longest-running Go SQLite binding | github.com/mattn/go-sqlite3 | OK (manually verified) | Not selected (CGo dependency) — kept as documented fallback only |
| `github.com/ncruces/go-sqlite3` | Go module proxy | v0.1.0 → v0.35.2, active | github.com/ncruces/go-sqlite3 | OK (manually verified) | Not selected — documented alternative only |
| `zombiezen.com/go/sqlite` | Go module proxy | v0.1.0 → v1.4.2, active | github.com/zombiezen/go-sqlite | OK (manually verified) | Not evaluated further — not needed given `modernc.org/sqlite` selection |
| `github.com/glebarez/sqlite` | Go module proxy | v1.0.0 → v1.11.0, active | github.com/glebarez/sqlite | OK (manually verified) | Not applicable — this is a GORM driver wrapper; this phase does not use an ORM (`.planning/research/STACK.md` explicitly lists "ORM for show-state mutations" as an anti-pattern for this project) |
| `github.com/pressly/goose/v3` | Go module proxy | v3.27.2 → v3.27.3, active | github.com/pressly/goose | OK (manually verified) | Considered, not adopted this phase — see Alternatives Considered / Open Questions |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

*`modernc.org/sqlite` is tagged `[CITED: .planning/research/STACK.md]` because it was originally sourced via Context7 against official `pkg.go.dev` documentation in this project's prior stack research pass, not merely discovered by this session's WebSearch. Every other package in this table (`mattn/go-sqlite3`, `ncruces/go-sqlite3`, `zombiezen.com/go/sqlite`, `glebarez/sqlite`, `pressly/goose/v3`) was discovered via this session's WebSearch and registry-existence-confirmed — per the package-name provenance rule, these remain tagged `[ASSUMED]` and, since none are being installed as this phase's actual dependency, no `checkpoint:human-verify` gate is required (only `modernc.org/sqlite` is a real install, and it carries `[CITED]` provenance already).*

## Architecture Patterns

### System Architecture Diagram

```
                    ┌─────────────────────────────────────────┐
                    │  CLI process (one per invocation)        │
                    │  e.g. `golc pool update ...`              │
                    │  e.g. `golc show open|save|diagnose|...`  │
                    └───────────────┬───────────────────────────┘
                                    │ 1. Load(root, path)
                                    ▼
                    ┌─────────────────────────────────────────┐
                    │  internal/show (storage layer)            │
                    │  ┌───────────────────────────────────┐   │
                    │  │ Open sql.DB (modernc.org/sqlite)   │   │
                    │  │ PRAGMA journal_mode=WAL            │   │
                    │  │ PRAGMA synchronous=FULL            │   │
                    │  │ PRAGMA foreign_keys=ON             │   │
                    │  └───────────────┬───────────────────┘   │
                    │                  │ 2. read show_meta +    │
                    │                  │    show_state blob     │
                    │                  ▼                        │
                    │  ┌───────────────────────────────────┐   │
                    │  │ strictjson.DecodeStrict(blob)      │   │
                    │  │ show.validate(State)               │   │
                    │  └───────────────┬───────────────────┘   │
                    └──────────────────┼───────────────────────┘
                                       │ 3. State value returned
                                       ▼
                    ┌─────────────────────────────────────────┐
                    │  Command handler (internal/command/*.go)  │
                    │  mutate State in memory                   │
                    └───────────────┬───────────────────────────┘
                                    │ 4. Save(root, path, state)
                                    ▼
                    ┌─────────────────────────────────────────┐
                    │  internal/show (storage layer)             │
                    │  BEGIN TRANSACTION                         │
                    │    UPDATE show_meta (schema_version,       │
                    │      revision+1, checksum)                 │
                    │    UPDATE show_state (blob)                │
                    │    INSERT INTO recovery_points (blob, ...) │
                    │    DELETE oldest recovery_points if >5     │
                    │  COMMIT                                    │
                    │  Close sql.DB (checkpoint on close)         │
                    └─────────────────────────────────────────┘
                                    │
                                    ▼ (never touches)
                    ┌─────────────────────────────────────────┐
                    │  internal/playback/engine.go               │
                    │  Engine holds State via atomic.Pointer      │
                    │  — NEVER calls show.Load/show.Save          │
                    └─────────────────────────────────────────┘

  Separate flow — migration (SHOW-05), triggered by "show open" when
  show_meta.schema_version < current:

    show open --show old.golc
      │
      ▼
    detect schema_version < current → prompt user for confirmation
      │ confirm
      ▼
    VACUUM INTO 'old.golc.backup-<timestamp>'   (D-08/D-09 backup)
      │
      ▼
    open backup in a NEW connection, DecodeStrict + validate()   (D-09 verify)
      │ verified OK
      ▼
    BEGIN TRANSACTION on a temp copy
      apply ordered migration functions (schema_version N -> N+1 -> ... -> current)
      re-validate resulting State
      write new blob + bump schema_version
    COMMIT
      │
      ▼
    close all connections to old.golc and to the temp copy
    os.Rename(tempCopy, old.golc)   (Windows: MoveFileEx REPLACE_EXISTING)
    delete any stray -wal/-shm sidecars left at the destination path
```

### Recommended Project Structure
```
internal/show/
├── state.go           # unchanged: State struct, validate() (D-02)
├── state_test.go       # unchanged existing round-trip tests
├── store.go            # NEW: Load/Save reimplemented over SQLite (same signatures)
├── store_test.go       # NEW: SQLite-backed round-trip, WAL crash-safety, concurrent-open tests
├── schema.go            # NEW: CREATE TABLE statements, PRAGMA setup, application_id stamp
├── migrate.go            # NEW: ordered schema_version migration function registry (D-08)
├── migrate_test.go        # NEW: fixture-per-schema-version migration tests
├── backup.go               # NEW: VACUUM INTO + read-back-and-validate (D-09)
├── backup_test.go
├── recovery.go              # NEW: recovery_points insert/prune/detect/offer (D-04-D-07)
├── recovery_test.go
├── diagnose.go               # NEW: PRAGMA integrity_check + validate() combined report (D-11)
└── diagnose_test.go
```

### Pattern 1: Single-blob-plus-metadata schema (D-03)
**What:** Three small tables — `show_meta` (singleton row: `schema_version`, `revision`, `checksum`, `updated_at`), `show_state` (singleton row: `blob`), `recovery_points` (append/prune, up to 5 rows: `id`, `created_at`, `revision`, `blob`).
**When to use:** Always, for this phase — this is the locked D-03 shape, not a choice.
**Example:**
```sql
-- schema.go: created once, idempotently, on every Open
PRAGMA application_id = 1196574019; -- ASCII 'GOLC' as int32, stamps the file as a .golc format
PRAGMA journal_mode = WAL;
PRAGMA synchronous = FULL;
PRAGMA foreign_keys = ON;

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
```
<!-- Source: SQLite PRAGMA/CREATE TABLE syntax is standard SQL; application_id pattern per https://sqlite.org/pragma.html#pragma_application_id -->

### Pattern 2: Save + recovery-point write in one transaction (D-04)
**What:** `Save` opens a transaction, updates `show_meta`/`show_state`, inserts one `recovery_points` row, deletes rows beyond the newest 5, commits.
**When to use:** Every command mutation — this is the existing `Load → mutate → Save` call site every `internal/command/*.go` handler already uses; no new call-site wiring needed (per CONTEXT.md's Reusable Assets note).
**Example:**
```go
// Source: standard database/sql transaction pattern; SQLite VACUUM INTO/backup
// guidance per https://www.sqlite.org/lang_vacuum.html#vacuuminto
func Save(root, path string, s State) error {
    if err := validate(s); err != nil {
        return fmt.Errorf("GOLC_SHOW_STATE_INVALID: %v", err)
    }
    s.SchemaVersion = SchemaVersion
    s.Revision++
    payload, err := strictjson.CanonicalEncode(s)
    if err != nil {
        return fmt.Errorf("GOLC_SHOW_STATE_INVALID: %v", err)
    }
    checksum := sha256Hex(payload)

    db, err := openStore(root, path) // applies PRAGMAs, ensures schema
    if err != nil {
        return err
    }
    defer db.Close() // triggers a passive checkpoint on close

    tx, err := db.Begin()
    if err != nil {
        return err
    }
    defer tx.Rollback() // no-op after Commit

    now := time.Now().UTC().Format(time.RFC3339)
    if _, err := tx.Exec(`UPDATE show_meta SET schema_version=?, revision=?, checksum=?, updated_at=? WHERE id=1`,
        s.SchemaVersion, s.Revision, checksum, now); err != nil {
        return err
    }
    if _, err := tx.Exec(`UPDATE show_state SET blob=? WHERE id=1`, payload); err != nil {
        return err
    }
    if _, err := tx.Exec(`INSERT INTO recovery_points (created_at, revision, blob) VALUES (?, ?, ?)`,
        now, s.Revision, payload); err != nil {
        return err
    }
    if _, err := tx.Exec(`DELETE FROM recovery_points WHERE id NOT IN
        (SELECT id FROM recovery_points ORDER BY id DESC LIMIT 5)`); err != nil {
        return err
    }
    return tx.Commit()
}
```

### Pattern 3: Verified backup before migration (D-08/D-09)
**What:** `VACUUM INTO` a timestamped backup file, then open that backup **in a fresh connection**, strictly decode and `validate()` it — never trust that the copy succeeded just because `VACUUM INTO` returned no error.
**When to use:** Every time `show open` detects `show_meta.schema_version < SchemaVersion`.
**Example:**
```go
// Source: https://www.sqlite.org/lang_vacuum.html#vacuuminto ,
// cross-checked against this project's own .planning/research/ARCHITECTURE.md
// "Persistence, Migrations, and Recovery" section (rule 3).
func verifiedBackup(root, path string) (backupPath string, err error) {
    backupPath = path + ".backup-" + time.Now().UTC().Format("20060102T150405Z")
    db, err := openStore(root, path)
    if err != nil {
        return "", err
    }
    defer db.Close()
    if _, err := db.Exec(`VACUUM INTO ?`, resolvePath(root, backupPath)); err != nil {
        return "", fmt.Errorf("GOLC_SHOW_BACKUP_FAILED: %v", err)
    }

    // D-09: read-back + whole-State validate, not checksum-only.
    verifyDB, err := openStore(root, backupPath)
    if err != nil {
        return "", fmt.Errorf("GOLC_SHOW_BACKUP_UNVERIFIABLE: %v", err)
    }
    defer verifyDB.Close()
    var blob []byte
    if err := verifyDB.QueryRow(`SELECT blob FROM show_state WHERE id=1`).Scan(&blob); err != nil {
        return "", fmt.Errorf("GOLC_SHOW_BACKUP_UNVERIFIABLE: %v", err)
    }
    var s State
    if err := strictjson.DecodeStrict(blob, &s); err != nil {
        return "", fmt.Errorf("GOLC_SHOW_BACKUP_UNVERIFIABLE: %v", err)
    }
    if err := validate(s); err != nil {
        return "", fmt.Errorf("GOLC_SHOW_BACKUP_UNVERIFIABLE: %v", err)
    }
    return backupPath, nil
}
```

### Pattern 4: Windows atomic replace after migration success (D-08)
**What:** Migrate into a fresh temp SQLite file, close every connection, then `os.Rename(tempPath, destPath)` — reusing `internal/show/state.go`'s existing write-temp-then-rename convention, adapted for a file-based SQLite swap instead of a JSON write.
**When to use:** Only after the migration transaction commits successfully and the migrated copy has itself been read-back-validated (same pattern as Pattern 3).
**Example:**
```go
// Source: golang/go commit 92c5736 ("os: windows Rename should overwrite
// destination file") — Go's Windows os.Rename already uses MoveFileEx with
// MOVEFILE_REPLACE_EXISTING|MOVEFILE_COPY_ALLOWED, so plain os.Rename
// overwrites on Windows exactly like POSIX rename. https://sqlite.org/wal.html
// for why stray sidecars must be cleaned up.
func atomicReplace(root, destPath, tempPath string) error {
    resolvedDest := resolvePath(root, destPath)
    resolvedTemp := resolvePath(root, tempPath)

    // All *sql.DB handles to both paths MUST already be closed here —
    // Windows file locking (unlike POSIX) can block rename of an open handle.
    if err := os.Rename(resolvedTemp, resolvedDest); err != nil {
        return fmt.Errorf("GOLC_SHOW_MIGRATE_SWAP_FAILED: %v", err)
    }
    // Remove any stray sidecars the destination may still have from before
    // the swap (e.g. -wal/-shm from a prior WAL session that never got a
    // clean checkpoint) so the next Open never mixes old and new content.
    _ = os.Remove(resolvedDest + "-wal")
    _ = os.Remove(resolvedDest + "-shm")
    return nil
}
```

### Pattern 5: Newer-format hard refusal, read-only inspect allowed (D-10)
**What:** On `Open`, compare `show_meta.schema_version` against the app's `SchemaVersion` constant. If newer: never migrate, never write, but still allow decoding the blob for `show inspect`/`show export`/`show diagnose` (D-10's "not fully blind" requirement, reusing SHOW-06's surface).
**When to use:** Every `Open` call, before any write path is reachable.
```go
if meta.SchemaVersion > SchemaVersion {
    // Read-only path: decode + validate for inspect/export/diagnose only.
    // The CLI layer must route this state to read-only commands and
    // reject "show save"/any mutating command with GOLC_SHOW_SCHEMA_TOO_NEW.
    return State{}, ErrSchemaTooNew{Found: meta.SchemaVersion, Supported: SchemaVersion}
}
```

### Anti-Patterns to Avoid
- **Copying the `.golc` file with a plain filesystem copy while it might be open in WAL mode:** loses any committed data still sitting only in the `-wal` sidecar. Always use `VACUUM INTO` or the Online Backup API — never `io.Copy`/`os.ReadFile` a live database file. [CITED: sqlite.org/wal.html via this session's WebSearch]
- **Trusting `VACUUM INTO`'s lack of error as proof of a valid backup:** D-09 explicitly requires read-back + `validate()`. A backup file can be byte-copied correctly and still represent an already-invalid or stale State if the source read raced a concurrent writer — hence a fresh connection + full decode + full validate, not a checksum comparison.
- **Reaching for an ORM or a general SQL migration framework by default:** this project's own prior research (`.planning/research/PITFALLS.md` Pitfall 9, `.planning/research/ARCHITECTURE.md`'s persistence rules) already flags "Assuming SQLite Automatically Solves Save, Migration, and Recovery" as a release-blocking pitfall category. `goose` fits normalized-schema migrations; this phase's single-blob model needs Go-level struct/JSON transforms instead (see Common Pitfalls).
- **Running `PRAGMA integrity_check` on every `show open`:** D-12 explicitly scopes diagnostics to on-demand only. At this app's scale it would be fast (sub-second), but doing it automatically contradicts the locked decision and adds unnecessary latency to the common open path.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|--------------|-----|
| Consistent snapshot of a live/open SQLite file | Custom "pause writes, copy file, resume" locking scheme | `VACUUM INTO` (or the SQLite Online Backup API for larger files) | SQLite's own backup mechanisms already handle WAL contents, page-level consistency, and concurrent-writer safety; a hand-rolled copy routine will eventually race a writer and produce a corrupt backup — this is exactly Pitfall 9 in this project's own prior research. |
| Atomic file replace on Windows | Manual `DeleteFile` + `MoveFile` two-step, or a custom retry loop around sharing violations | `os.Rename` (already wraps `MoveFileEx` with `MOVEFILE_REPLACE_EXISTING`) | Go's standard library already solved the historical "Windows Rename doesn't overwrite" problem; reinventing it risks reintroducing the exact race the stdlib fix closed. |
| Database corruption/consistency detection | Custom blob-diffing or ad hoc "does it look right" heuristics | `PRAGMA integrity_check` (file-level) + `show.validate()` (structural) | SQLite's integrity_check already walks every B-tree page, index, and freelist pointer — reimplementing that in application code is both slower and less complete. |
| Schema version tracking | A custom version marker embedded in the JSON blob only | `PRAGMA user_version` (SQLite-native 32-bit integer, plus the existing `show_meta.schema_version` column as the source of truth per D-03) | `user_version` is free, always present in the file header, and lets external tooling (or a future recovery utility) discover a file's schema version without decoding the blob at all — cheap defense in depth. |

**Key insight:** SQLite already solved "durable transactional single-writer file storage" — the only genuinely new code this phase needs to write is the **blob-shape migration function registry** (transforming the Go struct's JSON shape across `schema_version` bumps), because that part is inherently domain-specific and no generic tool (SQL migration framework or otherwise) can do it for you.

## Common Pitfalls

### Pitfall 1: Treating every short-lived CLI process's WAL sidecar files as a bug
**What goes wrong:** A contributor sees `.golc-wal`/`.golc-shm` files appear and disappear across successive `golc pool create`, `golc deployment activate`, etc. invocations and assumes something is leaking or corrupting the file.
**Why it happens:** Unlike Phase 4's Art-Net worker (a long-lived daemon), every show-mutating command in this app is its own short-lived process — `show.Load` → mutate → `show.Save` → process exit. Each invocation opens a fresh `*sql.DB`. In WAL mode, sidecar files are a normal, expected artifact of any open connection and are supposed to persist between separate process invocations that share the same underlying file.
**How to avoid:** Document this explicitly in the store package's doc comment (the way `state.go`'s existing doc comment documents its own atomic-rename convention). Ensure `Save`'s `defer db.Close()` triggers SQLite's automatic passive checkpoint on close so sidecars don't grow unbounded across thousands of invocations over a show's editing lifetime — add an explicit `PRAGMA wal_checkpoint(PASSIVE)` (non-blocking) right before `Close()` as a belt-and-suspenders step, since relying solely on the default `wal_autocheckpoint` (1000 pages) may under-trigger for this app's small, frequent transactions.
**Warning signs:** A test or contributor treats a present `-wal`/`-shm` file as an error condition rather than checking file *content* validity.

### Pitfall 2: Assuming `goose`-style SQL migrations fit this phase's blob-only schema
**What goes wrong:** Reaching for `github.com/pressly/goose/v3` (already recommended in this project's prior `.planning/research/STACK.md`) because it was the project's earlier stack pick, then writing migrations that are 90% Go application logic (`SELECT blob`, transform in Go, `UPDATE blob`) wrapped in an SQL-migration-file shell that adds no value and obscures the actual transform logic.
**Why it happens:** `goose` (and SQL migration tools generally) are designed for evolving table/column DDL. CONTEXT.md's D-02/D-03 deliberately kept the domain model as a single opaque blob specifically to avoid a normalized-schema rewrite — so the vast majority of future "migrations" will be Go struct-shape changes, not SQL DDL changes.
**How to avoid:** Use a small `map[int]func([]byte) ([]byte, error)` (or equivalent ordered slice) keyed by source `schema_version`, executed inside the same transaction as the backup-verified migration flow. Reserve `goose` for the rare case a future phase genuinely needs new SQL tables/indexes (e.g., splitting `recovery_points` into per-entity rows) — apply it there, not to blob content changes.
**Warning signs:** A migration file's SQL body is just `UPDATE show_state SET blob = ?` with all the real logic living in Go code that generated the parameter — a sign the SQL-migration framing added no value.

### Pitfall 3: Backing up while relying on a raw file copy instead of `VACUUM INTO`
**What goes wrong:** A backup routine does `io.Copy` of the `.golc` file (perhaps for simplicity, or because it "worked in testing" on a database whose WAL happened to already be checkpointed). In production, a copy taken while the WAL sidecar holds uncommitted-to-main-file data silently produces a backup missing recent saves.
**Why it happens:** SQLite's WAL mode intentionally defers writing committed data into the main file until a checkpoint; a naive copy of just the main file is a well-documented SQLite corruption/data-loss vector (`sqlite.org/howtocorrupt.html`), independently confirmed by this project's own prior `.planning/research/PITFALLS.md` Pitfall 9.
**How to avoid:** `VACUUM INTO` for this file's scale (fine — it also compacts the file, which is a bonus, not a requirement). Never expose a raw-copy code path even as a "quick" option.
**Warning signs:** A backup file that opens successfully but is missing the most recent edits — the single hardest-to-detect corruption class described in `PITFALLS.md`.

### Pitfall 4: `PRAGMA integrity_check` performance assumptions from large-database benchmarks
**What goes wrong:** A contributor sees widely-cited numbers ("20 minutes on a 4.2GB database") and either avoids adding a `show diagnose` command's integrity check to the default flow, or over-engineers a progress bar / cancellation UX for it.
**Why it happens:** Those numbers come from multi-gigabyte production databases; PROJECT.md/STATE.md's own scale assumption for this app is roughly 10-50 fixtures per rig, meaning the entire `.golc` file (one JSON blob plus a handful of recovery-point copies) will be measured in kilobytes to low single-digit megabytes, not gigabytes.
**How to avoid:** Run the full `PRAGMA integrity_check` (not the lighter `quick_check`) synchronously in `show diagnose` — D-11 asked for the thorough check, and at this scale it will complete in well under a second. Confirm this empirically with a benchmark test in Wave 0 rather than assuming it, since "small rig scale" is itself a project assumption, not a hard-verified fact for this specific phase.
**Warning signs:** A `show diagnose` implementation adds unnecessary async/streaming/cancellation machinery for what is, at this scale, an instant operation.

### Pitfall 5: Forgetting the `application_id` stamp, letting `.golc` accept any SQLite file
**What goes wrong:** `show open some-random.sqlite` (a file that happens to be valid SQLite but was never created by GOLC) proceeds past `Open` and fails deep inside blob-decode with a confusing error, instead of a clean "this is not a `.golc` file" diagnostic at the door.
**Why it happens:** SQLite has no built-in file-type discrimination beyond its own magic header (which just says "I am SQLite," not "I am a GOLC show").
**How to avoid:** Stamp a GOLC-specific `PRAGMA application_id` on every file this app creates (Pattern 1), and check it first thing in `Open`, before attempting to read `show_meta`/`show_state` — mirrors `state.go`'s existing "nothing from disk is trusted before checks pass" doctrine (referenced in its own doc comment as CONTEXT threat T-02-10).
**Warning signs:** `Open`'s error path for a non-`.golc` SQLite file is a generic SQL error ("no such table: show_state") rather than a clean `GOLC_SHOW_NOT_GOLC_FORMAT` diagnostic.

## Code Examples

### Open (Load) reading the metadata + blob, with newer-format refusal
```go
// Source: pattern synthesized from database/sql standard usage +
// this project's existing internal/show/state.go Load doc-comment
// contract ("nothing from disk is trusted before validate() passes").
func Load(root, path string) (State, error) {
    db, err := openStore(root, path) // ensures schema exists; applies PRAGMAs
    if err != nil {
        return State{}, fmt.Errorf("GOLC_SHOW_STATE_INVALID: %v", err)
    }
    defer db.Close()

    var meta struct {
        SchemaVersion int
        Revision      int
        Checksum      string
    }
    err = db.QueryRow(`SELECT schema_version, revision, checksum FROM show_meta WHERE id=1`).
        Scan(&meta.SchemaVersion, &meta.Revision, &meta.Checksum)
    if errors.Is(err, sql.ErrNoRows) {
        return State{SchemaVersion: SchemaVersion}, nil // fresh show, mirrors today's not-yet-existing-file case
    }
    if err != nil {
        return State{}, fmt.Errorf("GOLC_SHOW_STATE_INVALID: %v", err)
    }
    if meta.SchemaVersion > SchemaVersion {
        return State{}, ErrSchemaTooNew{Found: meta.SchemaVersion, Supported: SchemaVersion}
    }
    if meta.SchemaVersion < SchemaVersion {
        return State{}, ErrSchemaMigrationRequired{Found: meta.SchemaVersion, Supported: SchemaVersion}
    }

    var blob []byte
    if err := db.QueryRow(`SELECT blob FROM show_state WHERE id=1`).Scan(&blob); err != nil {
        return State{}, fmt.Errorf("GOLC_SHOW_STATE_INVALID: %v", err)
    }
    var state State
    if err := strictjson.DecodeStrict(blob, &state); err != nil {
        return State{}, fmt.Errorf("GOLC_SHOW_STATE_INVALID: %v", err)
    }
    if err := validate(state); err != nil {
        return State{}, fmt.Errorf("GOLC_SHOW_STATE_INVALID: %v", err)
    }
    return state, nil
}
```

### Diagnose combining structural + file-level checks (D-11)
```go
// Source: PRAGMA integrity_check per https://sqlite.org/pragma.html#pragma_integrity_check
func Diagnose(root, path string) (DiagnosticReport, error) {
    db, err := openStore(root, path)
    if err != nil {
        return DiagnosticReport{}, err
    }
    defer db.Close()

    rows, err := db.Query(`PRAGMA integrity_check`)
    if err != nil {
        return DiagnosticReport{}, fmt.Errorf("GOLC_SHOW_DIAGNOSE_FAILED: %v", err)
    }
    var fileLevelIssues []string
    for rows.Next() {
        var line string
        if err := rows.Scan(&line); err != nil {
            return DiagnosticReport{}, err
        }
        if line != "ok" {
            fileLevelIssues = append(fileLevelIssues, line)
        }
    }

    state, structuralErr := Load(root, path) // reuses the same validate() every Load runs
    return DiagnosticReport{
        FileLevelIssues: fileLevelIssues,
        StructuralOK:    structuralErr == nil,
        StructuralError: structuralErr,
        SchemaVersion:   state.SchemaVersion,
        Revision:        state.Revision,
    }, nil
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|-------------------|---------------|--------|
| Raw filesystem copy for SQLite "backup" | `VACUUM INTO` (added SQLite 3.27.0, 2019) or the Online Backup API | Long-standing (not new this cycle) but still commonly gotten wrong per multiple 2026 blog posts found this session | Raw copy under WAL silently loses recently-committed data; both alternatives are safe while the source is open. |
| `MoveFile` (non-overwriting) as Go's Windows `os.Rename` | `MoveFileEx` with `MOVEFILE_REPLACE_EXISTING|MOVEFILE_COPY_ALLOWED` | Merged into Go stdlib well before this project's pinned `go 1.26.5` (commit `92c5736`) | `os.Rename` on Windows already behaves like POSIX rename (overwrites destination) — the existing `state.go` write-temp-then-rename convention needs no special-casing for Windows beyond closing all handles first. |
| SQLite WAL mode assumed universally safe across concurrent writer/checkpointer combinations | WAL-reset corruption bug (data race between 2+ connections writing/checkpointing simultaneously) | Found and fixed 2026-03-13 in SQLite 3.51.3, backported to 3.44.6/3.50.7 | Confirms the driver pin matters: `modernc.org/sqlite` v1.54.0's bundled 3.53.2 is safely past this fix. Any future driver/version bump must not regress below 3.51.3. |

**Deprecated/outdated:**
- Filesystem-copy-based SQLite backup scripts: superseded by `VACUUM INTO`/Online Backup API for any database that might be open in WAL mode when backed up.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|----------------|
| A1 | `synchronous=FULL` (not `NORMAL`) is the right default given this app's per-command (not per-frame) write frequency | Summary, Standard Stack | If write frequency in practice is much higher than assumed (e.g., very rapid successive command invocations during dense programming sessions), `FULL`'s extra fsync-per-commit could add perceptible CLI latency; `NORMAL` (still durable across app crash, per D-04's actual requirement) is the fallback if this proves too slow in Wave-0 benchmarking. |
| A2 | A hand-rolled Go migration-function registry is more appropriate than adopting `goose` for this phase's blob-only schema | Standard Stack Alternatives, Common Pitfalls #2 | If a future phase needs genuine SQL DDL evolution alongside blob migrations (not just this phase's scope), the hand-rolled registry may need retrofitting to also run DDL; this is a design judgment call, not a locked CONTEXT.md decision, so the planner/user should confirm before committing. |
| A3 | Stamping a GOLC-specific `PRAGMA application_id` is worth doing even though CONTEXT.md never explicitly asked for it | Common Pitfalls #5, Don't Hand-Roll | Low risk if skipped (D-11's diagnostics would still eventually catch a malformed file, just with a less clean error) — but the design pattern is cheap and improves error-message quality at the "wrong file type" door. |
| A4 | No existing committed `.golc`/`show.json` fixture files exist anywhere in the repository that this phase's format change would break | (see codebase verification below — not a RESEARCH.md section, verified directly) | Verified this session: every `internal/command/*_test.go` reference to `"show.json"` uses `t.TempDir()`-scoped ephemeral paths (grep confirmed across ~15 test files); no committed binary/JSON show fixtures exist in the repo. Low risk of this being wrong since it was directly grepped, not assumed — included here for completeness/traceability. |
| A5 | `wal_autocheckpoint`'s default (1000 pages) may under-trigger for this app's small, frequent, per-command-process transaction pattern, justifying an explicit `PRAGMA wal_checkpoint(PASSIVE)` before every `Close()` | Common Pitfalls #1 | If wrong (i.e., default checkpointing is already sufficient at this app's scale), the extra explicit checkpoint call is merely a harmless no-op-ish safety measure, not a correctness risk either way. |

**If this table is empty:** N/A — see entries above; none are release-blocking on their own, but A1 and A2 in particular should be confirmed with the user or explicitly decided by the planner before implementation, since they affect concrete PRAGMA values and package dependencies.

## Open Questions (RESOLVED)

1. **`synchronous=FULL` vs `synchronous=NORMAL` — final call**
   - What we know: `WAL+NORMAL` already satisfies D-04's literal requirement ("survive an unexpected process kill") per SQLite's own documented guarantee. `WAL+FULL` additionally survives OS-level crashes/power loss, at the cost of an extra fsync per transaction commit.
   - What's unclear: Whether that extra fsync cost is perceptible in this app's per-command-process usage pattern (each command already pays process-startup overhead, so an extra fsync may be genuinely negligible in relative terms) — not benchmarked this session.
   - Recommendation: Default to `FULL` (stronger guarantee, likely negligible relative cost given process-per-command overhead already dominates); confirm with a Wave-0 timing test comparing `FULL` vs `NORMAL` on a representative save, and downgrade to `NORMAL` only if `FULL` shows a measurable, user-visible latency regression.
   - **RESOLVED:** Adopted `synchronous=FULL` as the default — implemented in 05-01-PLAN.md Task 2 (PRAGMA configuration on the store connection).

2. **Migration support window policy (ROADMAP.md's explicit "N-version-back" question)**
   - What we know: This is a pre-1.0 desktop app; only `schema_version=1` exists today. D-10 already handles the "too new" direction (hard refuse). Nothing in CONTEXT.md locks a "too old" cutoff.
   - What's unclear: Whether to build an explicit oldest-supported-version floor now (e.g., "refuse to migrate anything older than schema_version N-3") or support migrating from any historical version indefinitely until a real support-window policy is needed post-1.0.
   - Recommendation: Support migration from **any** older `schema_version` up to current for v1 (there is currently only one version to support, so this costs nothing today) — defer a formal N-version-back deprecation policy until 1.0 ships and real user files with old schema versions actually exist to reason about. Document this explicitly as a deferred policy decision so it isn't silently forgotten.
   - **RESOLVED:** Adopted no N-version-back floor — 05-03-PLAN.md Task 2's bounds-check accepts any on-disk `schema_version` in [1, SchemaVersion] and migrates it forward; the formal deprecation-window policy is deferred to post-1.0.

3. **Whether `goose` should still be adopted for any part of this phase (table DDL only, not blob content)**
   - What we know: This phase's three tables (`show_meta`, `show_state`, `recovery_points`) are created once, idempotently, via plain `CREATE TABLE IF NOT EXISTS` in `schema.go` — there is no DDL evolution need yet.
   - What's unclear: Whether the planner wants to establish the `goose`-based DDL-migration convention now (even with zero real migrations to run) for consistency with the project's prior stack research, or defer it entirely until a real DDL change is needed.
   - Recommendation: Skip `goose` entirely for this phase; the `CREATE TABLE IF NOT EXISTS` pattern is sufficient and avoids an unused dependency. Revisit if/when a genuine DDL migration is needed.
   - **RESOLVED:** Skipped `goose` — 05-03-PLAN.md Task 2 uses a plain Go function-map registry (`map[int]func([]byte) ([]byte, error)`) instead of a SQL migration framework, introducing no new dependency.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|--------------|-----------|---------|----------|
| Go toolchain | Building the SQLite-backed store | Yes | go1.26.5 windows/amd64 | — (already the project's pinned toolchain, `go.mod` confirms `go 1.26.5`) |
| `gcc` (MSYS2/mingw64) | Only needed if a CGo driver (`mattn/go-sqlite3`) were selected instead | Yes, present on this dev machine (`gcc.exe (Rev4, Built by MSYS2 project) 13.2.0`) | 13.2.0 | Not required — `modernc.org/sqlite` (the recommended driver) is CGo-free, so this dependency does not gate the recommended path. Documented here only because its presence was checked as part of evaluating the CGo alternative. |
| `sqlite3` CLI (MSYS2/mingw64) | Optional manual `.golc` file inspection during development/debugging | Yes | 3.45.1 (2024-01-30) | Note: this is **older** than the WAL-reset fix (3.51.3) and older than the driver's bundled SQLite (3.53.2 via `modernc.org/sqlite` v1.54.0) — do not use this system CLI to validate production WAL-mode compatibility claims; it is a convenience tool for eyeballing table contents only. |
| Go module proxy network access | Verifying package versions (`go list -m -versions`) | Yes, confirmed reachable this session | — | — |

**Missing dependencies with no fallback:** none.
**Missing dependencies with fallback:** none — the CGo path (gcc) was evaluated and explicitly not selected; its absence would not have blocked this phase.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go standard `testing` package (no external test framework) |
| Config file | none — plain `go test`, following existing repo convention across all `internal/*` packages |
| Quick run command | `go test ./internal/show/...` |
| Full suite command | `go test ./...` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|---------------------|-------------|
| SHOW-01 | Save/Open/Save-As round trip on a SQLite-backed `.golc` file preserves the full `State` | unit | `go test ./internal/show/... -run TestShowStoreRoundTrip -v` | ❌ Wave 0 — new `store_test.go` |
| SHOW-02 | Open/Save/Save-As never touch `internal/playback` — no import cycle, no call into `Engine` | unit (architecture/import check) | `go test ./internal/show/... -run TestShowStoreNoPlaybackImport -v` (or a `go list -deps` assertion in CI) | ❌ Wave 0 |
| SHOW-03 | Every command mutation writes a recovery point in the same transaction as the save | unit | `go test ./internal/show/... -run TestSaveWritesRecoveryPoint -v` | ❌ Wave 0 — new `recovery_test.go` |
| SHOW-04 | Recovery points are capped at 5, oldest pruned first; recovery is offered (not auto-applied) on next open | unit | `go test ./internal/show/... -run TestRecoveryPointPruning -v` and `TestRecoveryOfferedNotApplied` | ❌ Wave 0 |
| SHOW-05 | Migration creates a verified backup (read-back + validate), applies atomically, refuses newer formats without rewriting | unit + integration | `go test ./internal/show/... -run TestMigration -v` (fixture-per-schema-version corpus) | ❌ Wave 0 — new `migrate_test.go` with historical-fixture corpus |
| SHOW-05 (Windows atomic swap) | A migrated file replaces the original only after full transactional success; interrupted migration leaves the original untouched | integration (forced-kill simulation) | `go test ./internal/show/... -run TestMigrationForceKillLeavesOriginalIntact -v` | ❌ Wave 0 |
| SHOW-06 | `PRAGMA integrity_check` + `validate()` combined report; JSON export byte-identical to `strictjson.CanonicalEncode(State)` | unit | `go test ./internal/show/... -run TestDiagnose -v` and `TestExportMatchesCanonicalEncode` | ❌ Wave 0 — new `diagnose_test.go` |

### Sampling Rate
- **Per task commit:** `go test ./internal/show/...`
- **Per wave merge:** `go test ./...`
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `internal/show/store_test.go` — SQLite-backed Load/Save round trip, replacing/extending the existing JSON-based `state_test.go` coverage (SHOW-01/02)
- [ ] `internal/show/recovery_test.go` — recovery-point write/prune/detect/offer (SHOW-03/04)
- [ ] `internal/show/migrate_test.go` — migration corpus: one fixture file per historical `schema_version`, plus a forced-kill-mid-migration test proving the original file survives untouched (SHOW-05)
- [ ] `internal/show/diagnose_test.go` — `integrity_check` + `validate()` combined report, JSON export byte-identity (SHOW-06)
- [ ] `internal/show/backup_test.go` — `VACUUM INTO` + read-back-and-validate (D-09), including a deliberately-corrupted backup fixture proving verification actually rejects a bad backup
- [ ] Framework install: `go get modernc.org/sqlite@v1.54.0` — no test framework install needed (stdlib `testing`)

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|-----------------|---------|---------------------|
| V2 Authentication | No | This phase has no authentication surface — local single-user file I/O only. |
| V3 Session Management | No | Not applicable — no session concept in this phase. |
| V4 Access Control | No | Not applicable — local file, single-user desktop app; no multi-actor access control surface introduced. |
| V5 Input Validation | Yes | Every value read from an opened `.golc` file (untrusted input — a hand-edited or corrupted SQLite file is exactly as untrusted as today's hand-edited JSON file) must pass `strictjson.DecodeStrict` + `show.validate()` before use, exactly as `state.go`'s existing doc comment already establishes for threat T-02-10. Extend this doctrine to the new `show_meta`/`show_state`/`recovery_points` table reads — never trust a `schema_version` or `revision` integer read from disk without range/sanity checks before using it to index into the migration function registry. |
| V6 Cryptography | No new requirement | The `checksum` field in `show_meta` is an integrity check, not a security cryptographic boundary — SHA-256 (already the project's convention per other phases' hashing use, e.g. fixture content hashes in FIXT-05) is sufficient; no encryption requirement exists for this phase (`.golc` files are not described as needing confidentiality protection anywhere in REQUIREMENTS.md/CONTEXT.md). |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|------------------------|
| Malformed/malicious `.golc` file (hand-edited SQLite, or a non-GOLC SQLite file renamed to `.golc`) crashes or misbehaves the opening process | Tampering / Denial of Service | `PRAGMA application_id` check first; then `strictjson.DecodeStrict` + `show.validate()` before trusting any field — exactly the existing T-02-10 pattern this project already applies to the JSON path, extended to the SQLite path. |
| Migration function registry indexed by an untrusted `schema_version` integer read from disk (e.g., a negative number, or a huge number colliding with array bounds) | Tampering | Bounds-check `schema_version` against `[1, SchemaVersion]` before ever using it to look up a migration function or slice index; treat any out-of-range value as `GOLC_SHOW_STATE_INVALID`, never index directly. |
| Migration mid-flight interruption (process killed during the migrate step) leaves the working file in a half-migrated, unusable state | Tampering / Denial of Service | D-08's transactional-migrate-then-atomic-swap design already prevents this structurally: the original file is never touched until the fully-migrated, re-validated copy is ready to atomically replace it (Pattern 3/4). A test proving this (forced-kill mid-migration) is a Wave-0 gap above, not just a design claim. |
| Backup file path collision / directory traversal via a crafted `--show` path argument | Tampering / Information Disclosure | Reuse `resolvePath`'s existing root-relative-vs-absolute resolution convention (already in `state.go`) for backup and temp-migration paths — do not introduce a second, inconsistent path-resolution rule for the new code. |
| Recovery-point blob left readable indefinitely after a user explicitly discards a recovered session (D-07's "discard" option) | Information Disclosure | Low severity for this app (recovery-point data is the operator's own show content on their own machine — matches this project's own Phase 4 precedent of accepting similar local-only disclosure risks, e.g. AR-04-01/AR-04-02 in `04-SECURITY.md`); "discard" should still `DELETE` the recovery-point rows rather than merely hiding them from the CLI's offer prompt, so the file doesn't grow unbounded with declined recovery data. |

## Sources

### Primary (HIGH confidence)
- `.planning/research/STACK.md` (this project's own prior stack research, Context7-sourced against `pkg.go.dev/modernc.org/sqlite`, `sqlite.org/backup.html`, `sqlite.org/wal.html`) — driver pick, backup/atomic-replace pattern, goose recommendation.
- `.planning/research/ARCHITECTURE.md` "Persistence, Migrations, and Recovery" section — single-writer-connection rule, application_id/user_version convention, backup-before-migrate rule, WAL/-shm/-wal co-location rule.
- `.planning/research/PITFALLS.md` Pitfall 9 "Assuming SQLite Automatically Solves Save, Migration, and Recovery" — WAL-reset bug awareness, raw-copy-backup anti-pattern, verified-backup requirement.
- `internal/show/state.go` (read directly this session) — exact current `Load`/`Save` signatures, `validate()` scope, existing write-temp-then-rename convention that Pattern 4 extends.
- `go list -m -versions` against `proxy.golang.org` (run directly this session) — confirmed real, actively-released package names/versions for `modernc.org/sqlite`, `mattn/go-sqlite3`, `ncruces/go-sqlite3`, `zombiezen.com/go/sqlite`, `glebarez/sqlite`, `pressly/goose/v3`.

### Secondary (MEDIUM confidence)
- WebSearch, cross-checked against official SQLite documentation pages (sqlite.org/wal.html, sqlite.org/pragma.html, sqlite.org/lang_vacuum.html) for: WAL vs DELETE journal mode durability, synchronous=FULL/NORMAL tradeoffs, VACUUM INTO semantics, Online Backup API, WAL sidecar file safety, PRAGMA integrity_check performance scaling, PRAGMA user_version migration pattern, WAL checkpoint/autovacuum interaction.
- WebSearch for Go `os.Rename` Windows behavior, cross-referenced against the golang/go commit history (`92c5736`) confirming `MoveFileEx`/`MOVEFILE_REPLACE_EXISTING` usage.
- WebSearch for the SQLite WAL-reset corruption bug (3.51.3 fix), cross-referenced against `sqlite.org/releaselog/3_51_3.html` and multiple independent news summaries (linuxiac.com, alternativeto.net).

### Tertiary (LOW confidence)
- None — every WebSearch finding used in this document was cross-checked against at least one official SQLite documentation page or an authoritative registry lookup before being included; nothing is presented at LOW confidence.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — driver choice reconfirms this project's own prior Context7-sourced research; version numbers freshly verified against the Go module proxy this session.
- Architecture: HIGH — the single-blob-plus-metadata schema is a locked CONTEXT.md decision (D-03), not a research judgment call; the surrounding transaction/backup/atomic-replace patterns are directly grounded in official SQLite documentation and this project's own prior architecture research.
- Pitfalls: MEDIUM-HIGH — most pitfalls are grounded in official SQLite documentation or this project's own prior pitfalls research; the specific claim about per-command-process WAL sidecar accumulation (Pitfall 1) is this session's own architectural inference from reading the codebase, not an externally-sourced claim, so it should be empirically confirmed in Wave 0.

**Research date:** 2026-07-23
**Valid until:** 2026-08-22 (30 days — SQLite/Go ecosystem is stable; re-verify driver version and WAL-reset-fix status if this research is reused after a significant `modernc.org/sqlite` or Go toolchain version bump)
