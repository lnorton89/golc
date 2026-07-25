// idempotency.go implements Idempotency-Key dedupe within a TTL window
// (07-05-PLAN.md Task 3, 07-RESEARCH.md Assumptions Log A6): mutate.go's
// pipeline checks server.idempotency before ever comparing If-Match or
// calling Execute -- a live entry for the presented key returns the
// stored first response, without re-executing, so the underlying mutation
// applies exactly once no matter how many times a client retries with the
// same key. Only a SUCCESSFUL mutation's response is stored (a failed
// attempt is not cached, so a client can safely retry after fixing
// whatever caused the failure); only mutating requests participate --
// read routes never consult this store, and dry-run responses are never
// cached either (dryrun.go's own outcome is "dry_run", never "success").
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

// idempotencyEntry is one stored successful-mutation response, replayable
// until expiresAt.
type idempotencyEntry struct {
	result    mutationResult
	expiresAt time.Time
}

// idempotencyStore is a per-*Server, in-memory, mutex-guarded map from a
// client-supplied Idempotency-Key to the first successful mutation's
// response, retained for ttl. now is overridable only by this package's
// own tests (a real *Server always uses time.Now); production code never
// needs to inject a different clock.
type idempotencyStore struct {
	mu      sync.Mutex
	entries map[string]idempotencyEntry
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
	return &idempotencyStore{entries: map[string]idempotencyEntry{}, ttl: ttl, now: time.Now}
}

// lookup returns the stored response for key if a live (non-expired)
// entry exists, deleting it first if it has expired (so a stale entry is
// never returned once past its TTL and does not linger in the map
// forever).
func (s *idempotencyStore) lookup(key string) (mutationResult, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.entries[key]
	if !ok {
		return mutationResult{}, false
	}
	if s.now().After(entry.expiresAt) {
		delete(s.entries, key)
		return mutationResult{}, false
	}
	return entry.result, true
}

// store records result under key, replayable until s.ttl from now.
func (s *idempotencyStore) store(key string, result mutationResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[key] = idempotencyEntry{result: result, expiresAt: s.now().Add(s.ttl)}
}
