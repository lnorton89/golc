# Phase 5: Durable Shows and Recovery - Context

**Gathered:** 2026-07-23
**Status:** Ready for planning

<domain>
## Phase Boundary

Users can preserve and recover complete shows in a portable versioned `.golc` format while storage stays outside the deterministic playback path. This phase delivers: (1) a durable `.golc` file that saves the complete show and deployment (open/save/save-as, SHOW-01/02), (2) autosaved rotating recovery points for interrupted-session recovery (SHOW-03/04), (3) schema migration with verified backup and atomic apply, refusing unsupported newer formats without rewriting (SHOW-05), and (4) integrity diagnostics plus a versioned human-readable JSON export for troubleshooting and interchange (SHOW-06). This phase is headless, following Phase 1-4's precedent — no Wails UI until Phase 6.

Requirements: SHOW-01 through SHOW-06.

</domain>

<decisions>
## Implementation Decisions

### Storage Format
- **D-01:** `.golc` is a **SQLite database file**, not a JSON envelope. This matches ROADMAP.md's research note explicitly flagging "SQLite durability settings" and gives migration/backup a transactional foundation (WAL durability, atomic transactions, `VACUUM INTO`/online backup) instead of hand-built file-copy operations.
- **D-02:** `internal/show.State` (the existing struct every command in `internal/command/*.go` already loads, mutates, and saves) **stays the in-memory/domain shape**. SQLite replaces only `show.Load`/`show.Save`'s disk-I/O internals — the command layer is largely unaffected. This was chosen over redesigning a normalized SQLite schema (tables per pool/deployment/scene) to keep this phase's scope to durability/recovery, not a domain-model rewrite.
- **D-03:** Inside the SQLite file, the whole `State` document is stored as **one serialized blob** (the same canonically-encoded bytes `strictjson.CanonicalEncode` already produces) in a table, alongside a small metadata table (`schema_version`, `revision`, checksum). This preserves today's whole-document `validate()`-before-trust model exactly — SQLite provides transactional/durable file semantics here, it does not become a relational data model for show entities.

### Autosave & Recovery Points
- **D-04:** Every command mutation triggers the recovery-point write — same trigger as today's existing every-edit-saves pattern (`show.Load` → mutate → `show.Save`). The recovery-point write happens inside the same SQLite transaction as the save, so no separate timer/background-writer/dirty-flag mechanism is needed.
- **D-05:** Rotating recovery points live in a **separate table inside the same `.golc` SQLite file** (not a sibling file) — one file to manage, atomic with the main save.
- **D-06:** **Keep the last 5** recovery points, oldest pruned on insert.
- **D-07:** Recovery is **auto-detected and offered on next open** — if `golc show open` finds recovery-table points newer than the last clean Save, GOLC surfaces "recovered session found" and lets the user inspect/accept/discard. It never silently overwrites the explicitly-saved file (matches SHOW-04's "clearly identified" framing).

### Migration & Backup Policy
- **D-08:** Schema migration runs **automatically on open, with confirmation** — opening a file with an older `schema_version` prompts the user that a backup will be made first, then on confirm: copy the original untouched (the verified backup), migrate in a transaction, and only replace the working file on full success.
- **D-09:** A migration backup counts as "verified" (SHOW-05) via **read-back + whole-State validate** — after copying, GOLC opens the backup copy fresh, strictly decodes it, and runs the same `show.validate()` invariants the main Load path uses. This proves the backup is a genuinely loadable, valid show, not just byte-identical (rejected: checksum-only comparison, which doesn't prove backed-up content was itself valid).
- **D-10:** Opening a `.golc` file with a `schema_version` **newer than supported** is a **hard refusal for editing/playback, but read-only inspect is still allowed** (reusing SHOW-06's integrity/inspect surface) — the file itself is never touched/rewritten. The user isn't left fully blind to a newer file's contents (e.g. after a downgrade).

### Integrity Diagnostics & JSON Export
- **D-11:** Integrity diagnostics (`golc show diagnose` or similar) run **structural validate (today's `show.validate()`) plus SQLite-level integrity** (`PRAGMA integrity_check`/page-corruption detection) — this catches both "logically inconsistent show document" and "bit-rotted/corrupted file on disk," which `validate()` alone can't see.
- **D-12:** Diagnostics run **on-demand only**, via an explicit command — not automatically on every open. Keeps the everyday open/save path fast; diagnose is a deliberate troubleshooting action, matching SHOW-06's framing as a distinct capability.
- **D-13:** SHOW-06's JSON export is the **same shape as today's canonical State JSON** — literally `strictjson.CanonicalEncode(State)`, the exact bytes that were the whole `.golc` file before this phase's move to SQLite. Zero new schema to design, and it stays naturally round-trippable back into a fresh SQLite `.golc` via the existing Load path (an import command reusing this path is a natural but unconfirmed follow-on — see Specific Ideas).

### Claude's Discretion
None — every gray area discussed converged on the recommended option; no "you decide" selections were made in this session.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Project-level requirements and roadmap
- `.planning/PROJECT.md` — Core value, and the "Live reliability" constraint ("UI, persistence, scripts, API, LLM, and Linear never own or block deterministic playback or Art-Net timing") that D-01/D-02's storage-layer-only migration exists to preserve.
- `.planning/REQUIREMENTS.md` §Durable Shows — SHOW-01 through SHOW-06 requirement text.
- `.planning/ROADMAP.md` §Phase 5: Durable Shows and Recovery — Goal, four success criteria, and the research note flagging SQLite durability settings, verified backup and retention policy, portable file/export rules, migration support window, read-only recovery, and Windows atomic replacement behavior as open research areas (this discussion resolved the storage-engine choice, State-vs-SQLite-schema relationship, autosave trigger/retention/restore-flow, migration-timing/backup-verification/newer-format-refusal, and diagnostics-scope/export-shape questions at the decision level; protocol-level implementation detail — exact SQLite pragma tuning, WAL checkpoint behavior, Windows atomic-replace mechanics — remains for phase research/planning).

### Prior-phase precedent this phase should follow
- `.planning/phases/03-deterministic-show-programming-and-playback/03-CONTEXT.md` D-14 — "Undo history is session-only... It is not persisted into the `.golc` file; SHOW-01/02 (Phase 5) treat the saved show as the durable unit, not the edit history." Phase 5's `.golc`/recovery design must not attempt to persist or recover in-memory undo/redo history — only the saved/autosaved `State` document.
- `.planning/phases/02-modular-fixtures-and-deployments/02-CONTEXT.md` D-16 — Phase 1's staleness-detection pattern (expected-revision check before apply) that this phase's SQLite `revision` metadata field (D-03) continues, not replaces.
- `.planning/STATE.md` §Accumulated Context → Decisions — "UI, persistence, scripts, API, LLM, and Linear never own or block deterministic playback or Art-Net timing" — the governing constraint behind D-01/D-02's storage-layer-only scope.

No user-referenced ADRs/specs beyond the project's own planning docs came up during discussion — no additional canonical docs to add.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/show/state.go`: `show.State` (the domain struct: `SchemaVersion`, `Revision`, `Pools`, `Deployments`, `Groups`, `Programmer`, `Themes`, `Presets`, `Chases`, `MotionPresets`, `Scenes`, `BlendPresets`, `Tempo`), `Load`/`Save` (currently `os.ReadFile`/write-temp-then-rename JSON), and `validate()` (whole-document invariant checks) — D-01/D-02/D-03 keep this struct and its validation exactly as-is; only `Load`/`Save`'s disk-I/O internals move from JSON file to SQLite.
- `internal/strictjson`: `DecodeStrict`/`CanonicalEncode` — the exact canonical JSON encoding D-03 stores as the SQLite blob and D-13 reuses verbatim as the SHOW-06 export format.
- `internal/command/*.go` (deployment.go, pool.go, playback.go, programming.go, scene.go, artnet.go): every one of these already follows the `show.Load(root, showPath)` → mutate → `show.Save(root, showPath, state)` pattern per command — this is the exact call site D-04's "every command mutation triggers the recovery-point write" plugs into; no new call-site wiring is needed beyond `show.Save`'s internals.
- `internal/trace/apply` (Phase 1 D-18 staleness + D-21 journal-resume pattern, referenced again in 02-CONTEXT D-16): established precedent for revision-based staleness detection that D-03's `revision` metadata field continues rather than reinvents.

### Established Patterns
- Write-temp-then-rename atomic persistence and whole-document `validate()`-before-trust (`show.Load`/`show.Save`) — D-01-D-03 replace the write-temp-then-rename *mechanism* with SQLite transactions while keeping the validate-before-trust *behavior* identical.
- `{DOMAIN}_{CONDITION}` diagnostic code convention (e.g. `GOLC_SHOW_STATE_INVALID`) — new Phase 5 diagnostics (migration failure, corrupted file, newer-unsupported format, recovery-point conflict) should follow the same naming convention.
- Dry-run/apply-style CLI UX (Phase 2 D-15: preview then explicit apply/confirm) — D-08's "automatic on open, with confirmation" migration flow and D-07's "offer on next open" recovery flow should present the same preview-then-confirm shape operators already know from `golc pool update`/`apply`.

### Integration Points
- `internal/playback/engine.go`: `Engine` never calls `show.Load`/`show.Save` directly — it's handed a `State` via `NewEngine`/`StageEdit` and holds it in an `atomic.Pointer[Frame]`-backed structure entirely in memory. This confirms D-01/D-02's premise: SQLite-backed persistence lives entirely in the command/storage layer and never touches the playback engine's hot path, satisfying the "storage remains outside the deterministic playback path" phase goal structurally, not just by convention.
- No SQLite dependency exists in the repo yet (`go.mod` currently has no `mattn/go-sqlite3`, `modernc.org/sqlite`, or similar) — Phase 5 is the first place a SQLite driver is introduced; phase research should resolve the CGo-vs-pure-Go driver tradeoff given the project's Windows-only qualification target (Phase 10).

</code_context>

<specifics>
## Specific Ideas

- The `.golc` file existing today is conceptually already the JSON `show.State` document (per `internal/show/state.go`'s own doc comment: "Phase 5 will later supersede this working representation with the durable .golc format") — this discussion confirmed that supersession is a storage-engine swap (JSON file → SQLite file), not a domain-model rewrite.
- D-13's export-format choice (reuse canonical State JSON verbatim) naturally sets up a `--from-json` style import path back into a fresh SQLite `.golc`, but this was not explicitly asked about or locked as a requirement — noted as an open, unconfirmed follow-on for planning to consider, not a locked decision.
- Recovery and migration both follow the same "never silently change the user's file" thread that runs through the whole discussion: recovery points are offered, not auto-applied (D-07); migration prompts before touching anything (D-08); newer-unsupported files are never rewritten even to make them readable (D-10).

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope. All four discussed areas (storage format, autosave & recovery points, migration & backup policy, integrity diagnostics scope) were clarifications of how to implement what's already in SHOW-01 through SHOW-06.

</deferred>

---

*Phase: 5-Durable Shows and Recovery*
*Context gathered: 2026-07-23*
