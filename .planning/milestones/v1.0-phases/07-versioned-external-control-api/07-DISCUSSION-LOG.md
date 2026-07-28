# Phase 7: Versioned External Control API - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-07-24
**Phase:** 7-Versioned External Control API
**Areas discussed:** Transport & versioning, Auth & remote access, Event streaming & gap recovery, Mutation semantics & audit

---

## Transport & versioning

| Option | Description | Selected |
|--------|-------------|----------|
| Thin JSON-RPC wrapper | POST /v1/command {route, args} mirrors internal/command.Request almost 1:1 | |
| Resource-oriented REST | GET /v1/shows/{id}, POST /v1/scenes, etc. — a real mapping/translation layer | ✓ |
| You decide | | |

**User's choice:** Resource-oriented REST

| Option | Description | Selected |
|--------|-------------|----------|
| URL path (/v1/...) | Version baked into the path | ✓ |
| Header-based (Accept/API-Version) | Version negotiated via a request header | |
| You decide | | |

**User's choice:** URL path (/v1/...)

| Option | Description | Selected |
|--------|-------------|----------|
| Generate from Go | Annotate/derive OpenAPI from Go handler + struct definitions | ✓ |
| Spec-first (hand-authored) | Commit a hand-written OpenAPI YAML as source of truth | |
| You decide | | |

**User's choice:** Generate from Go

| Option | Description | Selected |
|--------|-------------|----------|
| Standard library net/http | Go 1.22+'s enhanced ServeMux, no new dependency | |
| A router library (chi/gorilla) | More ergonomic middleware/route-grouping, new pinned dependency | ✓ |
| You decide | | |

**User's choice:** A router library (chi/gorilla)
**Notes:** User explicitly chose ergonomics over the project's general minimal-dependency bias for this one decision.

---

## Auth & remote access

| Option | Description | Selected |
|--------|-------------|----------|
| Static bearer tokens (config-issued) | One or more long-lived tokens in project-local config | |
| Per-token scoped API keys with expiry | Keys minted via dedicated route/CLI, stored hashed, revocable | ✓ |
| You decide | | |

**User's choice:** Per-token scoped API keys with expiry

| Option | Description | Selected |
|--------|-------------|----------|
| Config flag (config/*.toml) | Committed-config-style toggle following existing concern pattern | ✓ |
| CLI/route flag at server start | Explicit startup flag, not persisted config | |
| You decide | | |

**User's choice:** Config flag (config/*.toml)

| Option | Description | Selected |
|--------|-------------|----------|
| Inside the existing daemon | HTTP/API server listens alongside the daemon's existing IPC listener | ✓ |
| Separate API process | A distinct binary/process talking to the daemon over existing IPC | |
| You decide | | |

**User's choice:** Inside the existing daemon

| Option | Description | Selected |
|--------|-------------|----------|
| Coarse domain scopes | A handful of named scopes (playback, authoring, admin) | ✓ |
| Read vs write per domain | Each domain scope splits into read/write | |
| You decide | | |

**User's choice:** Coarse domain scopes

---

## Event streaming & gap recovery

| Option | Description | Selected |
|--------|-------------|----------|
| One global stream | GET /v1/events emits every domain's changes with a type field | ✓ |
| Per-domain streams | Separate endpoints per domain, each with its own revision sequence | |
| You decide | | |

**User's choice:** One global stream

| Option | Description | Selected |
|--------|-------------|----------|
| Last-Event-ID + revision replay buffer | SSE standard header + bounded server-side replay buffer | ✓ |
| Always full re-fetch on reconnect | No replay buffer; any reconnect re-queries authoritative state | |
| You decide | | |

**User's choice:** Last-Event-ID + revision replay buffer

| Option | Description | Selected |
|--------|-------------|----------|
| Server filters by scope | Clients only receive events within their key's scopes | |
| All connected clients see everything | Any authenticated client sees the full stream regardless of scope | ✓ |
| You decide | | |

**User's choice:** All connected clients see everything

| Option | Description | Selected |
|--------|-------------|----------|
| Any valid key streams | Any valid, non-expired API key can open the event stream | ✓ |
| Dedicated streaming capability | A key must be explicitly granted stream access | |
| You decide | | |

**User's choice:** Any valid key streams

---

## Mutation semantics & audit

| Option | Description | Selected |
|--------|-------------|----------|
| If-Match header with revision | HTTP's standard conditional-request pattern | ✓ |
| expected_revision in request body | Revision travels as a normal JSON field | |
| You decide | | |

**User's choice:** If-Match header with revision

| Option | Description | Selected |
|--------|-------------|----------|
| ?dry_run=true query param | Same endpoint, a query flag switches it to preview-only | ✓ |
| Separate /preview sub-resource | A distinct endpoint per mutation | |
| You decide | | |

**User's choice:** ?dry_run=true query param

| Option | Description | Selected |
|--------|-------------|----------|
| One /v1/batch endpoint | A single endpoint accepts an ordered list of sub-requests, atomic | ✓ |
| Per-resource bulk endpoints | Selected resources get their own bulk variant | |
| You decide | | |

**User's choice:** One /v1/batch endpoint

| Option | Description | Selected |
|--------|-------------|----------|
| New table in the existing .golc SQLite store | Reuses Phase 5's durable show-storage engine | ✓ |
| Separate append-only audit log file | A dedicated log file outside the show store | |
| You decide | | |

**User's choice:** New table in the existing .golc SQLite store

---

## Claude's Discretion

None — the user answered all 16 questions directly; no "you decide" selections were made.

## Deferred Ideas

None — discussion stayed within phase scope throughout.
