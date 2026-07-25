---
phase: 07-versioned-external-control-api
reviewed: 2026-07-25T00:00:00Z
depth: standard
files_reviewed: 46
files_reviewed_list:
  - config/api.toml
  - docs/api/COMPATIBILITY.md
  - internal/api/audit.go
  - internal/api/audit_test.go
  - internal/api/auth.go
  - internal/api/auth_test.go
  - internal/api/batch.go
  - internal/api/batch_test.go
  - internal/api/config.go
  - internal/api/config_test.go
  - internal/api/coverage_test.go
  - internal/api/deprecation.go
  - internal/api/deprecation_test.go
  - internal/api/doc.go
  - internal/api/dryrun.go
  - internal/api/dryrun_test.go
  - internal/api/events.go
  - internal/api/events_test.go
  - internal/api/generate.go
  - internal/api/generate_test.go
  - internal/api/idempotency.go
  - internal/api/idempotency_test.go
  - internal/api/keys.go
  - internal/api/mutate.go
  - internal/api/mutate_test.go
  - internal/api/observer.go
  - internal/api/ratelimit.go
  - internal/api/revision.go
  - internal/api/router.go
  - internal/api/server.go
  - internal/api/translate.go
  - internal/api/translate_test.go
  - internal/artnet/daemon.go
  - internal/command/apikey.go
  - internal/command/apikey_test.go
  - internal/command/artnet.go
  - internal/command/check.go
  - internal/command/generate.go
  - internal/projectconfig/decode.go
  - internal/projectconfig/local.go
  - internal/projectconfig/model.go
  - internal/projectconfig/registry.go
  - internal/projectconfig/strict_test.go
  - internal/routecatalog/routecatalog.go
  - internal/show/apikeys.go
  - internal/show/apikeys_test.go
  - internal/show/audit.go
  - internal/show/audit_test.go
  - internal/show/backup.go
  - internal/show/schema.go
  - internal/show/store.go
findings:
  critical: 1
  warning: 4
  info: 2
  total: 7
status: issues_found
---

# Phase 7: Code Review Report

**Reviewed:** 2026-07-25T00:00:00Z
**Depth:** standard (extended into cross-file tracing for the batch/events/mutate/audit interaction, given the phase's own stated review-focus areas)
**Files Reviewed:** 50 (all files in scope; `docs/api/openapi.json` and `internal/artnet/daemon_test.go`/`internal/command/artnet_test.go` were consulted for context but are generated/pre-existing and not separately itemized above)
**Status:** issues_found

## Summary

Phase 7 wires a new `/v1` Chi+Huma control API into the existing Art-Net
daemon: auth/scope/rate-limit middleware, a serialized mutation pipeline
(If-Match, dry-run, idempotency), atomic batching via a throwaway-copy +
single-Save strategy, a redacting audit trail, a revisioned SSE stream, and
a generated/drift-checked OpenAPI contract. The implementation is
disciplined about the things it explicitly set out to prove: no
client-supplied show path is ever honored (`TestShowPathInjection` proves
this directly), the batch commit is genuinely atomic at the data layer
(`TestBatchRollback`/`TestBatchIfMatchExternalRace` prove real
all-or-nothing behavior including the residual external-race window),
secrets are stripped before an audit row is ever built, and auth failures
are uniformly 401 regardless of cause.

However, tracing the batch engine's per-sub-request observer fan-out
through into the SSE ring buffer's replay logic (an interaction none of
the existing tests exercise, since `batch_test.go` never opens a live
event stream and `events_test.go` never issues a multi-sub-request batch)
surfaces a genuine data-loss bug: a batch with more than one sub-request
publishes multiple SSE events sharing the *same* revision/id, which
breaks the ring buffer's Last-Event-ID replay filter and can silently
drop an event on reconnect with no resync signal — directly contradicting
the package's own documented "never a silently missing gap" (D-10)
guarantee. Several smaller, related audit-completeness and
idempotency-scoping gaps are also documented below.

## Narrative Findings (AI reviewer)

### CR-01: Multi-sub-request batches publish SSE events with duplicate revisions, breaking Last-Event-ID replay and silently dropping events (D-10 violation)

**File:** `internal/api/batch.go:268-279` (root cause) interacting with `internal/api/events.go:135-155` (`publish`) and `internal/api/events.go:191-219` (`subscribe`'s replay/resync decision)

**Issue:**
`runBatch`'s final step fires one `MutationEvent` per sub-request, but
every one of them carries the *same* `resultingRevision` (the batch's
single aggregate revision bump):

```go
resultingRevision := baseRevision + 1
...
for i, route := range routes {
    fireMutationObservers(MutationEvent{
        Route: route, Args: args[i], Actor: actor, Source: "http",
        CorrelationID: correlationID, ExpectedRevision: expectedRevisionPtr,
        ResultingRevision: &resultingRevision, Outcome: "success", StatusCode: http.StatusOK,
    })
}
```

`publishMutationEvent` (events.go:311-324) turns each of these into a
`ringEvent` and `eventBroadcaster.publish` (events.go:135-155) appends
every one to the ring buffer verbatim — so a 3-sub-request batch inserts
three consecutive `ringEvent`s that all carry `Revision == N` and are all
sent to subscribers with the *same* SSE `id: N` line.

This breaks `subscribe`'s replay/resync decision (events.go:191-219),
which uses `Revision` as the sole ordering/dedupe key:

```go
case lastID >= latest:
    // Already caught up ... no replay, no resync.
    return nil, false, ch, unsubscribe
default:
    for _, bufEv := range b.buffer {
        if bufEv.Revision > lastID {   // strictly greater
            replay = append(replay, bufEv)
        }
    }
```

Concretely: a client that receives only the *first* of the batch's two
`id: N` events and then disconnects, reconnecting with
`Last-Event-ID: N`, will never see the second `id: N` event — either
`lastID >= latest` (if `N` is the buffer's current tail) short-circuits
to "already caught up," or the strict `bufEv.Revision > lastID` replay
filter excludes the second `N`-tagged entry even though the client never
actually received it. No resync event is emitted either, because from
the buffer's point of view nothing has scrolled out of window — the gap
is invisible to the very mechanism (D-10) that exists specifically to
make gaps visible.

This is exactly the kind of interaction `batch_test.go` (no event-stream
assertions) and `events_test.go` (no multi-sub-request-batch scenario)
individually miss, but which the phase's own review-focus areas
("SSE replay-buffer ... correctness" and "the batch atomicity guarantee")
both call out directly. `TestAuditObserverBatchSubEventsEachWriteOneRow`
(audit_test.go:191-211) confirms the *design intent* is genuinely "one
observer event per sub-request, all sharing one revision" — so this is a
structural mismatch between the observer fan-out contract and the SSE
layer's assumption that `Revision` uniquely identifies an event, not an
isolated typo.

**Fix:** Decouple the SSE `id:`/ring-buffer ordering key from
`show.State.Revision`. Give `ringEvent` (and the wire `domainEventPayload`)
a strictly monotonic sequence number (e.g. a package-level
`atomic.Int64` incremented once per `publish` call) used for the SSE
`id:` line and for `subscribe`'s replay/resync comparisons, while still
carrying `Revision` in the JSON body for domain purposes:

```go
type ringEvent struct {
    Seq      int64  // strictly increasing, used for SSE id / replay ordering
    Revision int64  // show revision, carried in the payload body only
    Payload  domainEventPayload
}
```

This preserves the current one-audit-row/one-SSE-event-per-sub-request
design (so `TestAuditObserverBatchSubEventsEachWriteOneRow` stays valid)
while fixing the replay/dedupe correctness bug. Add a test that opens an
event stream, issues a 2+ sub-request batch, disconnects after the first
frame, reconnects with that frame's `Last-Event-ID`, and asserts the
second sub-request's event is still delivered (not silently dropped).

## Warnings

### WR-01: Idempotency-Key store is not scoped by actor or route — cross-actor/cross-route replay risk

**File:** `internal/api/idempotency.go:45-87`, `internal/api/mutate.go:173-182`

**Issue:** `idempotencyStore.entries` is keyed purely by the raw,
client-supplied `Idempotency-Key` header value:

```go
type idempotencyStore struct {
    entries map[string]idempotencyEntry
    ...
}
```

`mutate`'s pipeline looks this up before ever checking which route the
*current* request targets or which actor is making it:

```go
if req.IdempotencyKey != "" {
    if cached, found := server.idempotency.lookup(req.IdempotencyKey); found {
        ... return cached, nil
    }
}
```

Two different API keys (actors) — or, once more mutating routes are
wired per this phase's own documented "future work" (coverage_test.go's
`reasonMutationFutureWork`), the same actor targeting two different
routes — that happen to present the same `Idempotency-Key` string will
silently receive each other's cached response instead of executing their
own request. A client believing its own mutation applied would actually
be looking at someone else's (or a different route's) result body. This
is low-impact today because only one mutating route (`pool create`)
exists and its response body carries no sensitive data beyond a pool id,
but the store's contract does not scale safely to the additional
mutating routes this phase explicitly plans to add later.

**Fix:** Key `idempotencyStore` entries by a composite of
`(actor, route, IdempotencyKey)`, not the raw client string alone —
mirrors the industry-standard (Stripe) practice of scoping idempotency
keys per API credential and per endpoint.

### WR-02: Batch sub-request scope/translation failures never reach the audit trail (unlike the equivalent single mutation)

**File:** `internal/api/batch.go:184-200`

**Issue:** `mutate.go`'s single-mutation pipeline fires a `"failure"`
`MutationEvent` (and therefore writes an `audit_log` row, per D-16/API-06)
when `RequireScope` rejects a request (mutate.go:158-164). `runBatch`'s
own pre-lock loop that translates and scope-checks every sub-request
never calls `fireMutationObservers` on any of its three failure paths:

```go
for i, req := range requests {
    route, reqArgs, translateErr := translateBatchSubRequest(req)
    if translateErr != nil {
        return nil, batchSubRequestError(i, 0, translateErr)      // no observer fired
    }
    requiredScope, scopeLookupErr := requiredScopeForRoute(route)
    if scopeLookupErr != nil {
        return nil, batchSubRequestError(i, 0, huma.Error500InternalServerError(scopeLookupErr.Error()))  // no observer fired
    }
    if scopeErr := RequireScope(ctx, requiredScope); scopeErr != nil {
        return nil, batchSubRequestError(i, 0, scopeErr)          // no observer fired
    }
    ...
}
```

Confirmed by `TestBatchRequiresScope` (batch_test.go:364-386), which
only asserts the 403 status and that the real revision stayed at 0 — it
never asserts an audit row exists. The result: an identical
scope-probing attempt produces an audit row via `POST /v1/pools` but
*no* audit row via `POST /v1/batch`, silently weakening the audit trail
for exactly the surface Rule 2 ("batch must not be a scope-bypass
vector") calls out as security-sensitive.

**Fix:** Fire a `MutationEvent{Outcome: "failure", ...}` for each
translate/scope failure inside this loop before returning, mirroring
`mutate.go`'s own scope-failure branch. Add an audit_test.go assertion
that a scope-rejected batch sub-request still produces one row.

### WR-03: `mutate`'s own internal scope-lookup-error path also skips the audit observer (latent, currently unreachable)

**File:** `internal/api/mutate.go:153-157`

**Issue:** When `requiredScopeForRoute` itself errors
(`GOLC_API_DOMAIN_SCOPE_UNDECLARED` — a route whose domain has no entry
in `domainScope`), `mutate` returns a 500 without ever calling
`fireMutationObservers`, unlike every other failure branch in the same
function (the `RequireScope` failure two lines below it *does* fire the
observer):

```go
requiredScope, scopeErr := requiredScopeForRoute(req.Route)
if scopeErr != nil {
    return mutationResult{}, huma.Error500InternalServerError(scopeErr.Error())   // no observer fired
}
if err := RequireScope(ctx, requiredScope); err != nil {
    fireMutationObservers(MutationEvent{... Outcome: "failure", ...})            // observer fired
    return mutationResult{}, err
}
```

Not reachable today (every route this package currently registers has a
`domainScope` entry), but it's a trap for the next contributor who wires
a new mutating route without also adding its domain to `domainScope`:
the resulting 500 would silently escape the "every attempted mutation
produces exactly one audit row" doctrine this package's own doc comments
promise.

**Fix:** Fire the observer in this branch too, for consistency with
every other early-return in `mutate`.

### WR-04: `DeprecationMiddleware` is fully built and unit-tested but never wired into `buildRouter` — no guard against it being forgotten later

**File:** `internal/api/router.go:113-124` vs `internal/api/deprecation.go:71-93`

**Issue:** `deprecation.go`'s own doc comment states the middleware is
"[i]ntended to be installed via router.go's buildRouter UseMiddleware
call ... once a real deprecation window begins," but `buildRouter` only
installs `AuthMiddleware`/`RateLimitMiddleware` today:

```go
humaAPI.UseMiddleware(AuthMiddleware(humaAPI, server), RateLimitMiddleware(humaAPI, server))
```

There is no test, compile-time check, or lint asserting that the day a
future plan calls `MarkOperationDeprecated` on some operation, someone
also remembers to add `DeprecationMiddleware(humaAPI)` to this
`UseMiddleware` call. If that step is missed, `docs/api/openapi.json`
will correctly show `"deprecated": true` (satisfying `generate.go`'s
contract and its own tests), but the `Deprecation`/`Sunset`/`Link`
response headers documented as load-bearing client-detection signals in
`docs/api/COMPATIBILITY.md` will silently never appear on real HTTP
responses — a documentation/behavior mismatch that would only surface
via a confused integrator, not a test failure.

**Fix:** Either (a) install `DeprecationMiddleware` unconditionally now
(it is a no-op for every currently non-deprecated operation, so this is
safe today and removes the "remember to wire this later" trap entirely),
or (b) add a test that asserts every operation with `op.Deprecated ==
true` actually emits the `Deprecation`/`Sunset` headers on a live
request through `buildRouter`'s real middleware chain — so a future
regression fails loudly instead of silently.

## Info

### IN-01: `POST /v1/keys`'s `expires_in` has no documented upper bound

**File:** `internal/api/keys.go:27-32`, `internal/command/apikey.go:154-160`

**Issue:** `mintAPIKeyInput.Body.ExpiresIn` is forwarded verbatim to
`api-key create --expires <...>`, whose own validation
(`internal/command/apikey.go`) only rejects a non-positive duration —
any admin-scoped caller can mint a key valid for an arbitrarily long
span (e.g. `"876000h"`, ~100 years). Not a vulnerability given minting
already requires the admin scope, but worth a documented cap (or at
least an OpenAPI `doc:` note) given this is exactly the kind of knob an
automated/AI-driven client could misuse by accident.

**Fix:** Consider an upper bound (e.g. reject anything beyond the
documented deprecation-window-scale duration) enforced in
`runAPIKeyCreate`, or explicitly document the current "no upper bound"
behavior as intentional in `keys.go`'s doc comment.

### IN-02: `requires`/scope values joined with `,` have no escaping, so a value containing a literal comma is silently misinterpreted

**File:** `internal/api/batch.go:94-98` (`translateBatchCreatePool`), `internal/api/mutate.go:286-289` (`registerCreatePool`)

**Issue:** Both the single-mutation and batch translation paths build
the downstream CLI's `--requires` argument via
`strings.Join(parsed.Requires, ",")`. A `requires` entry that itself
contains a comma (or a `scopes` entry passed to `keys.go`'s
`strings.Join(input.Body.Scopes, ",")`) would silently split into
multiple unintended values on the CLI side, with no rejection at the
HTTP boundary. This is a pre-existing convention (not introduced by
batch.go) and low-severity given the current controlled vocabularies
(capability types, scope names), but worth flagging since it repeats in
three places across this phase's new code and would compound as more
routes with free-form list fields are wired.

**Fix:** Reject (400) any list element containing a comma at the Huma
input-validation layer before it ever reaches `strings.Join`, or switch
the downstream CLI convention to a delimiter that cannot appear in a
valid capability/scope name.

---

_Reviewed: 2026-07-25T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
