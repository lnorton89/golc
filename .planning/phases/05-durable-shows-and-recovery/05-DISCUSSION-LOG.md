# Phase 5: Durable Shows and Recovery - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-07-23
**Phase:** 5-Durable Shows and Recovery
**Areas discussed:** Storage format, Autosave & recovery points, Migration & backup policy, Integrity diagnostics scope

---

## Storage format

| Option | Description | Selected |
|--------|-------------|----------|
| SQLite database | Single-file SQLite container; WAL durability, atomic transactions, built-in verified-backup primitives | ✓ |
| JSON envelope (today's format, versioned) | Keep existing show.State JSON shape with an outer envelope | |
| SQLite for autosave/recovery only, JSON for the saved .golc | Split: durable save-as/open stays JSON, autosave/recovery uses SQLite | |

**User's choice:** SQLite database
**Notes:** Matches ROADMAP.md's research note explicitly flagging "SQLite durability settings."

| Option | Description | Selected |
|--------|-------------|----------|
| State stays the in-memory/domain shape; SQLite replaces its persistence | Only Load/Save's disk I/O changes; command-layer code unaffected | ✓ |
| New SQLite-native schema, State becomes a translation layer | Normalized SQLite tables as source of truth, State becomes a projection | |

**User's choice:** State stays the in-memory/domain shape; SQLite replaces its persistence

| Option | Description | Selected |
|--------|-------------|----------|
| Single serialized blob + metadata | One row holds the whole canonically-encoded State document as a BLOB, plus a metadata table | ✓ |
| Normalized tables per entity | Each entity type gets its own SQLite table with columns/foreign keys | |

**User's choice:** Single serialized blob + metadata

---

## Autosave & recovery points

| Option | Description | Selected |
|--------|-------------|----------|
| Every command mutation | Each authoring command's Save also writes/refreshes the recovery point in the same transaction | ✓ |
| Timed interval | Background ticker writes a recovery point periodically if dirty | |
| Debounced after a quiet period | Wait for a pause in editing activity before writing | |

**User's choice:** Every command mutation

| Option | Description | Selected |
|--------|-------------|----------|
| Same .golc SQLite file, separate table, keep last N | A recovery_points table in the same file, oldest pruned on insert | ✓ |
| Separate recovery file alongside .golc, keep last N | e.g. myshow.golc.recovery independent of the main file | |

**User's choice:** Same .golc SQLite file, separate table, keep last N

| Option | Description | Selected |
|--------|-------------|----------|
| Keep 5, auto-detect + offer on next open | On next open, if newer recovery points exist, surface "recovered session found" | ✓ |
| Keep 10, explicit `golc show recover` command only | More headroom, never automatic | |

**User's choice:** Keep 5, auto-detect + offer on next open

---

## Migration & backup policy

| Option | Description | Selected |
|--------|-------------|----------|
| Automatic on open, with confirmation | Detects older schema_version, prompts, backs up, migrates in a transaction | ✓ |
| Automatic on open, no confirmation needed | Migration happens silently since a backup is always made | |
| Explicit `golc show migrate` command only | Opening an older file requires an explicit migrate command | |

**User's choice:** Automatic on open, with confirmation

| Option | Description | Selected |
|--------|-------------|----------|
| Read-back + whole-State validate | Open the backup copy fresh, strictly decode, run show.validate() | ✓ |
| Checksum comparison only | Hash source and backup, confirm byte-identical | |

**User's choice:** Read-back + whole-State validate

| Option | Description | Selected |
|--------|-------------|----------|
| Hard refusal, read-only inspect only | Refuse to open for editing/playback but allow read-only inspect | ✓ |
| Hard refusal, no access at all | Refuse outright, no inspect path | |

**User's choice:** Hard refusal, read-only inspect only

---

## Integrity diagnostics scope

| Option | Description | Selected |
|--------|-------------|----------|
| Structural validate + SQLite-level integrity | Today's show.validate() plus PRAGMA integrity_check/corruption detection | ✓ |
| Structural validate only | Reuses exactly today's show.validate() invariants | |

**User's choice:** Structural validate + SQLite-level integrity

| Option | Description | Selected |
|--------|-------------|----------|
| On-demand only via explicit command | Separate, deliberate troubleshooting action | ✓ |
| Automatically on every open, plus on-demand | Quick integrity pass before every use | |

**User's choice:** On-demand only via explicit command

| Option | Description | Selected |
|--------|-------------|----------|
| Same shape as today's canonical State JSON | Export reuses strictjson.CanonicalEncode(State) verbatim | ✓ |
| Distinct export schema | Separate, more human-readable JSON shape | |

**User's choice:** Same shape as today's canonical State JSON

---

## Claude's Discretion

None — every gray area discussed converged on the recommended option; no "you decide" selections were made in this session.

## Deferred Ideas

None — discussion stayed within phase scope. All four discussed areas were clarifications of how to implement what's already in SHOW-01 through SHOW-06.
