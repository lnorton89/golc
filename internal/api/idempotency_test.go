// idempotency_test.go pins idempotency.go's Idempotency-Key dedupe
// contract (07-05-PLAN.md Task 3, 07-RESEARCH.md Assumptions Log A6): two
// identical requests carrying the same Idempotency-Key within the TTL
// apply the underlying mutation exactly once (the second returns the
// stored first response, and the real revision advances by 1, not 2); the
// same key after the TTL expires re-executes; and different keys execute
// independently.
//
// This file lives in the external api_test package (see coverage_test.go's
// doc comment for why) so it can reach a real, live command registry
// through internal/routecatalog's test-only bridge.
package api_test

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/api"
	"github.com/lnorton89/golc/internal/routecatalog"
	"github.com/lnorton89/golc/internal/show"
)

// doIdempotentCreatePoolRequest issues POST /v1/pools with body
// {"name": name} and the given Idempotency-Key header (omitted if empty).
func doIdempotentCreatePoolRequest(t *testing.T, handler http.Handler, token, idempotencyKey, name string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/pools", jsonBody(t, map[string]any{"name": name}))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// --- TestIdempotencyReplayWithinTTLAppliesOnce ------------------------------

// TestIdempotencyReplayWithinTTLAppliesOnce proves two identical requests
// carrying the same Idempotency-Key within the TTL apply the mutation
// exactly once: the second call returns the stored first response, and
// the real revision advances by 1, not 2.
func TestIdempotencyReplayWithinTTLAppliesOnce(t *testing.T) {
	root := t.TempDir()
	showPath := filepath.Join(root, "show.golc")
	catalog, err := routecatalog.New()
	require.NoError(t, err, "routecatalog.New")
	server := api.NewServer(catalog, root, showPath, api.WithIdempotencyTTL(time.Hour))
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	const key = "idem-key-1"

	first := doIdempotentCreatePoolRequest(t, server.Handler(), token, key, "Alpha")
	require.True(t, first.Code >= 200 && first.Code < 300, "expected the first request to succeed, got %d (body: %s)", first.Code, first.Body.String())
	firstResult, firstRevision := decodeMutationBody(t, first)
	require.NotNil(t, firstRevision, "expected the first response revision to be 1, got nil")
	require.Equal(t, int64(1), *firstRevision, "expected the first response revision to be 1")

	second := doIdempotentCreatePoolRequest(t, server.Handler(), token, key, "Alpha")
	require.Equal(t, first.Code, second.Code, "expected the replayed request to return the same status")
	secondResult, secondRevision := decodeMutationBody(t, second)
	require.Equal(t, firstResult, secondResult, "expected the replayed response body to match the original")
	require.NotNil(t, secondRevision, "expected the replayed response revision to still be 1, got nil")
	require.Equal(t, int64(1), *secondRevision, "expected the replayed response revision to still be 1")

	revision, err := show.CurrentRevision(root, showPath)
	require.NoError(t, err, "CurrentRevision")
	require.Equal(t, int64(1), revision, "expected the real revision to have advanced by exactly 1 (not 2)")

	state, err := show.Load(root, showPath)
	require.NoError(t, err, "show.Load")
	require.Len(t, state.Pools, 1, "expected exactly one pool (the effect applied exactly once)")
}

// --- TestIdempotencyReExecutesAfterTTLExpires -------------------------------

// TestIdempotencyReExecutesAfterTTLExpires proves the same key, presented
// again after its TTL has expired, re-executes the mutation rather than
// replaying a stale response.
func TestIdempotencyReExecutesAfterTTLExpires(t *testing.T) {
	root := t.TempDir()
	showPath := filepath.Join(root, "show.golc")
	catalog, err := routecatalog.New()
	require.NoError(t, err, "routecatalog.New")
	const shortTTL = 20 * time.Millisecond
	server := api.NewServer(catalog, root, showPath, api.WithIdempotencyTTL(shortTTL))
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	const key = "idem-key-expiring"

	first := doIdempotentCreatePoolRequest(t, server.Handler(), token, key, "Beta")
	require.True(t, first.Code >= 200 && first.Code < 300, "expected the first request to succeed, got %d (body: %s)", first.Code, first.Body.String())

	time.Sleep(shortTTL * 3)

	second := doIdempotentCreatePoolRequest(t, server.Handler(), token, key, "Gamma")
	require.True(t, second.Code >= 200 && second.Code < 300, "expected the post-TTL request to succeed as a fresh mutation, got %d (body: %s)", second.Code, second.Body.String())
	_, secondRevision := decodeMutationBody(t, second)
	require.NotNil(t, secondRevision, "expected the post-TTL request to genuinely re-execute (revision 2), got nil")
	require.Equal(t, int64(2), *secondRevision, "expected the post-TTL request to genuinely re-execute (revision 2)")

	state, err := show.Load(root, showPath)
	require.NoError(t, err, "show.Load")
	require.Len(t, state.Pools, 2, "expected two distinct pools (Beta from the first call, Gamma from the post-TTL re-execution)")
}

// --- TestIdempotencyKeyScopedByActor -----------------------------------------

// TestIdempotencyKeyScopedByActor proves the idempotency store is keyed by
// the (actor, route, key) triple, not the raw client-supplied key string
// alone (WR-01, 07-REVIEW.md): two distinct authoring-scoped actors
// presenting the identical Idempotency-Key each execute their own
// mutation and receive their own response -- neither ever receives the
// other's stored result. The cross-ROUTE half of the composite key cannot
// be exercised end to end today, because "pool create" is the only
// mutating route wired to the pipeline; the route component is included
// in the key now precisely so that future wiring (EXTN-05) is safe by
// construction, not because a second route exists to prove it against yet.
func TestIdempotencyKeyScopedByActor(t *testing.T) {
	root := t.TempDir()
	showPath := filepath.Join(root, "show.golc")
	catalog, err := routecatalog.New()
	require.NoError(t, err, "routecatalog.New")
	server := api.NewServer(catalog, root, showPath, api.WithIdempotencyTTL(time.Hour))
	tokenA, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAuthoring})
	tokenB, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	const sharedKey = "shared-idem-key"

	recA := doIdempotentCreatePoolRequest(t, server.Handler(), tokenA, sharedKey, "ActorAPool")
	require.True(t, recA.Code >= 200 && recA.Code < 300, "expected actor A's request to succeed, got %d (body: %s)", recA.Code, recA.Body.String())
	resultA, revisionA := decodeMutationBody(t, recA)
	require.NotNil(t, revisionA, "expected actor A's response revision to be 1, got nil")
	require.Equal(t, int64(1), *revisionA, "expected actor A's response revision to be 1")

	recB := doIdempotentCreatePoolRequest(t, server.Handler(), tokenB, sharedKey, "ActorBPool")
	require.True(t, recB.Code >= 200 && recB.Code < 300, "expected actor B's request to succeed, got %d (body: %s)", recB.Code, recB.Body.String())
	resultB, revisionB := decodeMutationBody(t, recB)
	require.NotNil(t, revisionB, "expected actor B's response revision to be 2 (its own mutation, not a replay of actor A's), got nil")
	require.Equal(t, int64(2), *revisionB, "expected actor B's response revision to be 2 (its own mutation, not a replay of actor A's)")

	require.NotEqual(t, resultA, resultB, "expected distinct result bodies for actor A and actor B")
	require.NotEqual(t, *revisionA, *revisionB, "expected the two actors' reported revisions to differ")

	revision, err := show.CurrentRevision(root, showPath)
	require.NoError(t, err, "CurrentRevision")
	require.Equal(t, int64(2), revision, "expected the real revision to have advanced by 2 (both actors' mutations applied)")

	state, err := show.Load(root, showPath)
	require.NoError(t, err, "show.Load")
	require.Len(t, state.Pools, 2, "expected exactly 2 pools (neither actor received the other's cached response)")
}

// --- TestIdempotencyDifferentKeysIndependent ---------------------------------

// TestIdempotencyDifferentKeysIndependent proves two different
// Idempotency-Key values execute independently -- neither dedupes against
// the other.
func TestIdempotencyDifferentKeysIndependent(t *testing.T) {
	root := t.TempDir()
	showPath := filepath.Join(root, "show.golc")
	catalog, err := routecatalog.New()
	require.NoError(t, err, "routecatalog.New")
	server := api.NewServer(catalog, root, showPath, api.WithIdempotencyTTL(time.Hour))
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	first := doIdempotentCreatePoolRequest(t, server.Handler(), token, "key-a", "Delta")
	require.True(t, first.Code >= 200 && first.Code < 300, "expected the first key's request to succeed, got %d (body: %s)", first.Code, first.Body.String())
	second := doIdempotentCreatePoolRequest(t, server.Handler(), token, "key-b", "Epsilon")
	require.True(t, second.Code >= 200 && second.Code < 300, "expected the second key's request to succeed, got %d (body: %s)", second.Code, second.Body.String())

	revision, err := show.CurrentRevision(root, showPath)
	require.NoError(t, err, "CurrentRevision")
	require.Equal(t, int64(2), revision, "expected two independent mutations to advance the revision by 2")
}
