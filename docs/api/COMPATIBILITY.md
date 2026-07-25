# GOLC External Control API: Compatibility & Deprecation Policy

This document is API-02's published compatibility/deprecation guidance for
the `/v1` GOLC external control API (07-RESEARCH.md Pattern 2, D-02). It
covers versioning, what counts as a breaking change, the deprecation
window, the response-header signals a deprecated operation carries, and
working client examples for every category of request the API supports.

The machine-readable contract this policy governs is
[`docs/api/openapi.json`](./openapi.json), generated directly from the Go
handler structs that implement it (`internal/api/generate.go`) and guarded
by a byte-stable drift check (`generate --check` / `mage generatecheck`) so
the published contract can never silently diverge from the running code.

## Versioning: URL-path, additive within a version

Every operation this API exposes is mounted under a fixed URL-path version
prefix, currently `/v1` (`internal/api/router.go`'s `apiPathPrefix`). A
version prefix is the whole of this API's versioning scheme -- there is no
header-based or content-negotiated versioning.

**Within `/v1`, only additive, backward-compatible changes are made:**

- adding a new operation (new route, new path)
- adding a new optional request field
- adding a new response field a well-behaved client already ignores
- adding a new typed error response code to an operation that did not
  previously declare it (a client that already handles "some 4xx/5xx I
  don't recognize" generically is unaffected)
- widening an accepted value set (e.g. a new enum member)

None of the above requires a new version. A client built against `/v1`
today keeps working against `/v1` indefinitely, as long as it follows
ordinary REST-client tolerance (ignore unknown response fields, treat
unrecognized error codes as failures rather than crashing).

## What counts as a breaking change

A breaking change is anything that could cause a well-behaved `/v1` client
to fail in a way it did not before. This includes, but is not limited to:

- removing or renaming an operation, path, or route
- removing or renaming a request or response field
- narrowing an accepted value set (removing an enum member, tightening
  validation on a previously-accepted value)
- changing a field's type or semantic meaning (e.g. a field that used to
  carry seconds now carries milliseconds)
- changing the meaning of a status code already documented for an
  operation (e.g. an operation that used to return `200` for success now
  returns `202` for the same request shape)
- changing authentication or authorization requirements in a way that
  invalidates a previously-valid request (tightening a scope requirement,
  changing the auth scheme)

**A breaking change is never made in place inside `/v1`.** It always ships
as a new version, mounted alongside the existing one, per the deprecation
window below.

## The deprecation window

When a breaking change is needed, this API introduces a new version prefix
(e.g. `/v2`) mounted as its own Chi sub-router alongside the still-fully-
functional `/v1` (07-RESEARCH.md Pattern 2's `r.Mount("/v1", ...)` /
`r.Mount("/v2", ...)` idiom), sharing as much handler code between the two
versions as the actual breaking change allows.

- **`/v1` keeps working, unmodified, for a minimum of 180 days** after
  `/v2` first ships (the deprecation window). This window is a floor, not
  a target -- operators are encouraged to keep a deprecated version alive
  longer when real integrator traffic against it persists.
- From the moment `/v2` ships, every response from a deprecated `/v1`
  operation the new version replaces carries the response headers
  described below, so a client can detect deprecation programmatically
  without reading this document.
- At the end of the deprecation window, the deprecated version's routes
  stop being served. A request against a sunset route after that date
  returns `410 Gone`.
- This policy applies per-operation, not only per-version: if `/v2` only
  replaces some `/v1` operations, the untouched `/v1` operations are never
  marked deprecated and continue indefinitely under `/v1` -- only the
  operations a breaking change actually touches enter a deprecation
  window.

## Deprecation response headers

A deprecated operation's every response (success or error) carries:

| Header | Meaning | Reference |
|---|---|---|
| `Deprecation: true` | This operation is deprecated. | draft-ietf-httpapi-deprecation-header (IETF, emerging) |
| `Sunset: <HTTP-date>` | The exact date this operation stops being served, in RFC 7231 `imf-fixdate` form (e.g. `Sat, 01 Jan 2028 00:00:00 GMT`). | RFC 8594 |
| `Link: <url>; rel="deprecation"` | Present only when a migration guide URL is configured; links to documentation for migrating off the deprecated operation. | RFC 8288 |

`internal/api/deprecation.go`'s `MarkOperationDeprecated` +
`DeprecationMiddleware` implement this mechanism: `MarkOperationDeprecated`
marks an operation `Deprecated: true` in the published OpenAPI document
(the field every OpenAPI viewer and generated client already understands)
and attaches its `Sunset`/`Link` values; `DeprecationMiddleware` reads that
back at request time and sets the three headers above on every response
for that operation. No operation in `/v1` is deprecated today -- this
mechanism is scaffolded and unit-tested now (`internal/api/deprecation_test.go`),
ready for the first breaking-change plan that ships a `/v2` to apply it to
whichever `/v1` operations that change obsoletes.

## Typed errors

Every `/v1` operation can return any of the following typed error
responses, declared in the published OpenAPI document
(`internal/api/generate.go`'s `commonErrorStatuses`):

| Status | Meaning | Produced by |
|---|---|---|
| `400 Bad Request` | The request was malformed, or translated into an invocation the routed command layer rejected as unroutable. | `translate.go`'s `translateResult` |
| `401 Unauthorized` | No valid `Authorization: Bearer <token>` API key was presented. | `auth.go`'s `AuthMiddleware`, installed ahead of every `/v1` operation |
| `403 Forbidden` | The authenticated key lacks the coarse domain scope (`playback`, `authoring`, or `admin`) the requested operation requires. | `auth.go`'s `RequireScope` |
| `412 Precondition Failed` | An `If-Match` header's expected revision did not match the show's current revision (optimistic concurrency), or (for `/v1/batch`) the show's revision changed while the batch was applying. | `revision.go`'s `checkRevision`, `batch.go`'s own If-Match/race check |
| `429 Too Many Requests` | The authenticated key exceeded its per-key rate limit. | `ratelimit.go`'s `RateLimitMiddleware`, installed ahead of every `/v1` operation |
| `5xx` (domain error) | A routed command handler failed for a domain-specific reason; the response carries that handler's own diagnostic message verbatim. | `translate.go`'s `translateResult` (any non-0/non-2 exit code) |

Every typed error response body follows Huma's standard RFC 9457
`application/problem+json` shape (`title`, `status`, `detail`), so a
generic HTTP-problem-aware client library can parse any `/v1` error
without operation-specific handling.

## Client examples

All examples assume a running daemon at `http://127.0.0.1:4590` (the
default loopback bind) and an already-minted API key (`$TOKEN`).

### A query: inspect the running show

```bash
curl -s http://127.0.0.1:4590/v1/show \
  -H "Authorization: Bearer $TOKEN"
```

### A mutation with `If-Match` (optimistic concurrency)

```bash
# Read the current revision from a prior response's "revision" field, or
# from a fresh GET /v1/show, then send it back as If-Match:
curl -s -X POST http://127.0.0.1:4590/v1/pools \
  -H "Authorization: Bearer $TOKEN" \
  -H "If-Match: 42" \
  -H "Content-Type: application/json" \
  -d '{"name": "Front Truss", "requires": ["dmx"]}'
```

A stale `If-Match` (the show's revision has already moved past `42`)
returns `412 Precondition Failed` and applies nothing.

### A dry run: preview a mutation without applying it

```bash
curl -s -X POST "http://127.0.0.1:4590/v1/pools?dry_run=true" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "Front Truss", "requires": ["dmx"]}'
```

The response has the same shape a real mutation would return, but the real
show is never touched and no `revision` is reported.

### A batch: multiple sub-requests applied atomically

```bash
curl -s -X POST http://127.0.0.1:4590/v1/batch \
  -H "Authorization: Bearer $TOKEN" \
  -H "If-Match: 42" \
  -H "Content-Type: application/json" \
  -d '{
        "requests": [
          {"route": "pool create", "args": ["Front Truss"]},
          {"route": "pool create", "args": ["Rear Truss"]}
        ]
      }'
```

If any sub-request fails, or the show's revision changes while the batch
is applying, the whole batch rolls back -- no sub-request's effect is
durably applied, and the response reports which sub-request failed.

### Minting a new API key (admin scope required)

```bash
curl -s -X POST http://127.0.0.1:4590/v1/keys \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"scopes": ["authoring"], "expires_in": "720h"}'
```

The response's raw token is returned exactly once, in this mint response
-- it is never retrievable again (`GET /v1/keys` only ever returns
metadata).

### Opening the event stream (SSE)

```bash
curl -N http://127.0.0.1:4590/v1/events \
  -H "Authorization: Bearer $TOKEN" \
  -H "Accept: text/event-stream"
```

Reconnecting after a drop: send the last received event's id back as
`Last-Event-ID` to replay everything since, or receive a synthetic
`event: resync` message if that id has already scrolled out of the
server's replay buffer (in which case, re-fetch authoritative state via
`GET /v1/show` before resuming):

```bash
curl -N http://127.0.0.1:4590/v1/events \
  -H "Authorization: Bearer $TOKEN" \
  -H "Accept: text/event-stream" \
  -H "Last-Event-ID: 137"
```

## Generating a client from the published contract

`docs/api/openapi.json` is a standard, drift-checked OpenAPI 3.1 document.
Any OpenAPI 3.1-compatible code generator (e.g. `openapi-generator-cli`,
`openapi-typescript`, Huma's own SDK generation tooling) can produce a
typed client directly from it:

```bash
npx openapi-typescript docs/api/openapi.json -o golc-api-client.d.ts
```

Because the document is generated from the same Go handler structs that
implement `/v1` and guarded by `generate --check`'s drift test, a
generated client is guaranteed to match the running server's actual
request/response shapes for the committed revision it was generated
against.
