// idempotency.go implements Idempotency-Key dedupe within a TTL window
// (07-05-PLAN.md Task 3, 07-RESEARCH.md Assumptions Log A6; composite
// keying per 07-13-PLAN.md Task 1, closing 07-REVIEW.md WR-01): mutate.go's
// pipeline checks server.idempotency before ever comparing If-Match or
// calling Execute -- a live entry for the presented (actor, route, key)
// triple returns the stored first response, without re-executing, so the
// underlying mutation applies exactly once no matter how many times a
// client retries with the same key. Only a SUCCESSFUL mutation's response
// is stored (a failed attempt is not cached, so a client can safely retry
// after fixing whatever caused the failure); only mutating requests
// participate -- read routes never consult this store, and dry-run
// responses are never cached either (dryrun.go's own outcome is
// "dry_run", never "success").
//
// An Idempotency-Key is a client-chosen opaque string and is only ever
// honored for the same authenticated actor and the same routed command
// that first stored it -- mirroring the industry convention (e.g. Stripe)
// of scoping idempotency keys per API credential and per endpoint. Without
// this scoping, two different clients (or the same client against two
// different mutating routes) that happen to choose the identical key
// string would receive each other's cached response, a cross-actor/
// cross-route information leak (WR-01). That risk grows with every
// mutating route this package wires (EXTN-05), so the store is keyed on
// the full (actor, route, key) triple from the start, even while "pool
// create" remains the only mutating route in the pipeline.
//
// This in-memory store is per-daemon-run: a daemon restart clears it.
// That is an accepted MVP tradeoff, not an oversight -- a durable
// idempotency table (mirroring D-16's audit_log) is a documented future
// hardening item, not required by this phase's locked decisions. The
// Idempotency-Key header convention itself and its TTL are [ASSUMED] (A6)
// pending a later discuss/UAT confirmation, per 07-RESEARCH.md's own
// Assumptions Log.
package api

import (
	"sync"
	"time"
)

// defaultIdempotencyTTL is the [ASSUMED] (A6) window a stored response
// remains replayable for, mirroring the Stripe-style Idempotency-Key
// convention 07-RESEARCH.md's Assumptions Log proposes (24h is a common
// industry default for this exact header).
const defaultIdempotencyTTL = 24 * time.Hour

// idempotencyKey is the composite, comparable key an entry is stored
// under: the authenticated actor, the routed command, and the client's
// own Idempotency-Key string. Using a struct (rather than a concatenated
// string with a chosen separator) means no separator choice can ever make
// two distinct (actor, route, key) triples collide -- each field compares
// independently as itself.
type idempotencyKey struct {
	actor string
	route string
	key   string
}

// idempotencyEntry is one stored successful-mutation response, replayable
// until expiresAt.
type idempotencyEntry struct {
	result    mutationResult
	expiresAt time.Time
}

// idempotencyStore is a per-*Server, in-memory, mutex-guarded map from an
// (actor, route, key) triple to the first successful mutation's response,
// retained for ttl. now is overridable only by this package's own tests (a
// real *Server always uses time.Now); production code never needs to
// inject a different clock.
type idempotencyStore struct {
	mu      sync.Mutex
	entries map[idempotencyKey]idempotencyEntry
	ttl     time.Duration
	now     func() time.Time
}

// newIdempotencyStore builds an empty store. A non-positive ttl (the zero
// value, e.g. a *Server constructed without WithIdempotencyTTL) falls
// back to defaultIdempotencyTTL -- idempotency dedupe is always active for
// a request that presents the header, never silently disabled by an
// unconfigured *Server.
func newIdempotencyStore(ttl time.Duration) *idempotencyStore {
	if ttl <= 0 {
		ttl = defaultIdempotencyTTL
	}
	return &idempotencyStore{entries: map[idempotencyKey]idempotencyEntry{}, ttl: ttl, now: time.Now}
}

// lookup returns the stored response for the (actor, route, key) triple if
// a live (non-expired) entry exists, deleting it first if it has expired
// (so a stale entry is never returned once past its TTL and does not
// linger in the map forever). Building the composite key internally, from
// three separate parameters, means no caller can construct or pass around
// a partial/pre-joined key.
func (s *idempotencyStore) lookup(actor, route, key string) (mutationResult, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	compositeKey := idempotencyKey{actor: actor, route: route, key: key}
	entry, ok := s.entries[compositeKey]
	if !ok {
		return mutationResult{}, false
	}
	if s.now().After(entry.expiresAt) {
		delete(s.entries, compositeKey)
		return mutationResult{}, false
	}
	return entry.result, true
}

// store records result under the (actor, route, key) triple, replayable
// until s.ttl from now.
func (s *idempotencyStore) store(actor, route, key string, result mutationResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	compositeKey := idempotencyKey{actor: actor, route: route, key: key}
	s.entries[compositeKey] = idempotencyEntry{result: result, expiresAt: s.now().Add(s.ttl)}
}
