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
	if err != nil {
		t.Fatalf("routecatalog.New: %v", err)
	}
	server := api.NewServer(catalog, root, showPath, api.WithIdempotencyTTL(time.Hour))
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	const key = "idem-key-1"

	first := doIdempotentCreatePoolRequest(t, server.Handler(), token, key, "Alpha")
	if first.Code < 200 || first.Code >= 300 {
		t.Fatalf("expected the first request to succeed, got %d (body: %s)", first.Code, first.Body.String())
	}
	firstResult, firstRevision := decodeMutationBody(t, first)
	if firstRevision == nil || *firstRevision != 1 {
		t.Fatalf("expected the first response revision to be 1, got %v", firstRevision)
	}

	second := doIdempotentCreatePoolRequest(t, server.Handler(), token, key, "Alpha")
	if second.Code != first.Code {
		t.Fatalf("expected the replayed request to return the same status %d, got %d", first.Code, second.Code)
	}
	secondResult, secondRevision := decodeMutationBody(t, second)
	if secondResult != firstResult {
		t.Fatalf("expected the replayed response body to match the original: %q vs %q", firstResult, secondResult)
	}
	if secondRevision == nil || *secondRevision != 1 {
		t.Fatalf("expected the replayed response revision to still be 1, got %v", secondRevision)
	}

	revision, err := show.CurrentRevision(root, showPath)
	if err != nil {
		t.Fatalf("CurrentRevision: %v", err)
	}
	if revision != 1 {
		t.Fatalf("expected the real revision to have advanced by exactly 1 (not 2), got %d", revision)
	}

	state, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load: %v", err)
	}
	if len(state.Pools) != 1 {
		t.Fatalf("expected exactly one pool (the effect applied exactly once), got %d", len(state.Pools))
	}
}

// --- TestIdempotencyReExecutesAfterTTLExpires -------------------------------

// TestIdempotencyReExecutesAfterTTLExpires proves the same key, presented
// again after its TTL has expired, re-executes the mutation rather than
// replaying a stale response.
func TestIdempotencyReExecutesAfterTTLExpires(t *testing.T) {
	root := t.TempDir()
	showPath := filepath.Join(root, "show.golc")
	catalog, err := routecatalog.New()
	if err != nil {
		t.Fatalf("routecatalog.New: %v", err)
	}
	const shortTTL = 20 * time.Millisecond
	server := api.NewServer(catalog, root, showPath, api.WithIdempotencyTTL(shortTTL))
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	const key = "idem-key-expiring"

	first := doIdempotentCreatePoolRequest(t, server.Handler(), token, key, "Beta")
	if first.Code < 200 || first.Code >= 300 {
		t.Fatalf("expected the first request to succeed, got %d (body: %s)", first.Code, first.Body.String())
	}

	time.Sleep(shortTTL * 3)

	second := doIdempotentCreatePoolRequest(t, server.Handler(), token, key, "Gamma")
	if second.Code < 200 || second.Code >= 300 {
		t.Fatalf("expected the post-TTL request to succeed as a fresh mutation, got %d (body: %s)", second.Code, second.Body.String())
	}
	_, secondRevision := decodeMutationBody(t, second)
	if secondRevision == nil || *secondRevision != 2 {
		t.Fatalf("expected the post-TTL request to genuinely re-execute (revision 2), got %v", secondRevision)
	}

	state, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load: %v", err)
	}
	if len(state.Pools) != 2 {
		t.Fatalf("expected two distinct pools (Beta from the first call, Gamma from the post-TTL re-execution), got %d", len(state.Pools))
	}
}

// --- TestIdempotencyDifferentKeysIndependent ---------------------------------

// TestIdempotencyDifferentKeysIndependent proves two different
// Idempotency-Key values execute independently -- neither dedupes against
// the other.
func TestIdempotencyDifferentKeysIndependent(t *testing.T) {
	root := t.TempDir()
	showPath := filepath.Join(root, "show.golc")
	catalog, err := routecatalog.New()
	if err != nil {
		t.Fatalf("routecatalog.New: %v", err)
	}
	server := api.NewServer(catalog, root, showPath, api.WithIdempotencyTTL(time.Hour))
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	first := doIdempotentCreatePoolRequest(t, server.Handler(), token, "key-a", "Delta")
	if first.Code < 200 || first.Code >= 300 {
		t.Fatalf("expected the first key's request to succeed, got %d (body: %s)", first.Code, first.Body.String())
	}
	second := doIdempotentCreatePoolRequest(t, server.Handler(), token, "key-b", "Epsilon")
	if second.Code < 200 || second.Code >= 300 {
		t.Fatalf("expected the second key's request to succeed, got %d (body: %s)", second.Code, second.Body.String())
	}

	revision, err := show.CurrentRevision(root, showPath)
	if err != nil {
		t.Fatalf("CurrentRevision: %v", err)
	}
	if revision != 2 {
		t.Fatalf("expected two independent mutations to advance the revision by 2, got %d", revision)
	}
}
