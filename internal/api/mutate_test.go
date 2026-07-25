// mutate_test.go pins mutate.go's serialized mutation pipeline (07-05-
// PLAN.md Task 1): a matching If-Match applies and bumps the real
// show.State.Revision by exactly one, a stale If-Match returns 412 and
// mutates nothing, a missing scope returns 403 and mutates nothing,
// concurrent mutating requests serialize (the real revision advances by
// exactly the number of successful mutations, with no corrupted/
// partially-applied state), and the post-mutation observer seam fires
// exactly once per attempted mutation, for both success and failure.
//
// This file lives in the external api_test package (see coverage_test.go's
// doc comment for why) so it can reach a real, live command registry
// through internal/routecatalog's test-only bridge for
// TestMutateIfMatchRevisionLifecycle and
// TestMutateSerializesConcurrentRequests, which need a genuine
// "pool create" -> show.Save round trip, not a canned stub outcome.
package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/lnorton89/golc/internal/api"
	"github.com/lnorton89/golc/internal/routecatalog"
	"github.com/lnorton89/golc/internal/show"
)

// seedKey generates and inserts a real api_keys row scoped to scopes,
// returning the raw token to present as a bearer credential plus the
// minted key's own id (for asserting MutationEvent.Actor).
func seedKey(t *testing.T, root, showPath string, scopes []show.APIKeyScope) (token, keyID string) {
	t.Helper()
	generated, err := show.GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey: %v", err)
	}
	key, err := show.InsertAPIKey(root, showPath, generated, scopes, time.Now().UTC().Add(time.Hour))
	if err != nil {
		t.Fatalf("InsertAPIKey: %v", err)
	}
	return generated.RawToken, key.KeyID
}

// doCreatePoolRequest issues POST /v1/pools with body {"name": name},
// presenting token and (if non-empty) ifMatch as headers.
func doCreatePoolRequest(t *testing.T, handler http.Handler, token, ifMatch, name string) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(map[string]any{"name": name})
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/pools", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	if ifMatch != "" {
		req.Header.Set("If-Match", ifMatch)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// decodeMutationBody decodes rec's body into the {result, revision} shape
// mutationOutput serializes.
func decodeMutationBody(t *testing.T, rec *httptest.ResponseRecorder) (result string, revision *int64) {
	t.Helper()
	var decoded struct {
		Result   string `json:"result"`
		Revision *int64 `json:"revision"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("decode mutation response %q: %v", rec.Body.String(), err)
	}
	return decoded.Result, decoded.Revision
}

// --- TestMutateIfMatchRevisionLifecycle -----------------------------------

// TestMutateIfMatchRevisionLifecycle proves a matching If-Match applies a
// mutation and bumps the real revision by exactly one, and a repeat with
// the now-stale If-Match returns 412 and creates nothing (CONTEXT D-13).
func TestMutateIfMatchRevisionLifecycle(t *testing.T) {
	root := t.TempDir()
	showPath := filepath.Join(root, "show.golc")
	catalog, err := routecatalog.New()
	if err != nil {
		t.Fatalf("routecatalog.New: %v", err)
	}
	server := api.NewServer(catalog, root, showPath)
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	before, err := show.CurrentRevision(root, showPath)
	if err != nil {
		t.Fatalf("CurrentRevision before: %v", err)
	}
	if before != 0 {
		t.Fatalf("expected a never-yet-saved show to report revision 0, got %d", before)
	}

	rec := doCreatePoolRequest(t, server.Handler(), token, "0", "Main")
	if rec.Code < 200 || rec.Code >= 300 {
		t.Fatalf("expected a 2xx creating with a matching If-Match, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	_, revision := decodeMutationBody(t, rec)
	if revision == nil || *revision != 1 {
		t.Fatalf("expected the response revision to be 1, got %v", revision)
	}

	after, err := show.CurrentRevision(root, showPath)
	if err != nil {
		t.Fatalf("CurrentRevision after: %v", err)
	}
	if after != 1 {
		t.Fatalf("expected the real revision to be 1 after one successful mutation, got %d", after)
	}

	// Repeat with the now-stale If-Match "0": must return 412 and create
	// nothing.
	stale := doCreatePoolRequest(t, server.Handler(), token, "0", "Second")
	if stale.Code != http.StatusPreconditionFailed {
		t.Fatalf("expected 412 for a stale If-Match, got %d (body: %s)", stale.Code, stale.Body.String())
	}

	stillAfter, err := show.CurrentRevision(root, showPath)
	if err != nil {
		t.Fatalf("CurrentRevision after stale attempt: %v", err)
	}
	if stillAfter != 1 {
		t.Fatalf("expected the real revision to remain 1 after a rejected stale mutation, got %d", stillAfter)
	}

	state, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load: %v", err)
	}
	if len(state.Pools) != 1 {
		t.Fatalf("expected exactly one pool to exist (the stale attempt must not have created \"Second\"), got %d", len(state.Pools))
	}
}

// --- TestMutateRequiresScope -----------------------------------------------

// TestMutateRequiresScope proves a mutation without the required coarse
// domain scope returns 403 and mutates nothing (CONTEXT D-08).
func TestMutateRequiresScope(t *testing.T) {
	root := t.TempDir()
	showPath := filepath.Join(root, "show.golc")
	catalog, err := routecatalog.New()
	if err != nil {
		t.Fatalf("routecatalog.New: %v", err)
	}
	server := api.NewServer(catalog, root, showPath)
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopePlayback})

	rec := doCreatePoolRequest(t, server.Handler(), token, "", "ShouldNotExist")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for a key lacking the authoring scope, got %d (body: %s)", rec.Code, rec.Body.String())
	}

	revision, err := show.CurrentRevision(root, showPath)
	if err != nil {
		t.Fatalf("CurrentRevision: %v", err)
	}
	if revision != 0 {
		t.Fatalf("expected the real revision to remain 0 after a scope-rejected mutation, got %d", revision)
	}
}

// --- TestMutateSerializesConcurrentRequests --------------------------------

// TestMutateSerializesConcurrentRequests proves concurrent mutating
// requests do not interleave writes: all complete, the real revision
// advances by exactly the number of requests, and every request's pool
// ends up durably saved (no lost update, 07-RESEARCH.md Pitfall 2).
func TestMutateSerializesConcurrentRequests(t *testing.T) {
	root := t.TempDir()
	showPath := filepath.Join(root, "show.golc")
	catalog, err := routecatalog.New()
	if err != nil {
		t.Fatalf("routecatalog.New: %v", err)
	}
	server := api.NewServer(catalog, root, showPath)
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	const concurrency = 5
	names := []string{"PoolA", "PoolB", "PoolC", "PoolD", "PoolE"}

	var wg sync.WaitGroup
	codes := make([]int, concurrency)
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			rec := doCreatePoolRequest(t, server.Handler(), token, "", names[i])
			codes[i] = rec.Code
		}(i)
	}
	wg.Wait()

	for i, code := range codes {
		if code < 200 || code >= 300 {
			t.Fatalf("expected request %d to succeed, got status %d", i, code)
		}
	}

	revision, err := show.CurrentRevision(root, showPath)
	if err != nil {
		t.Fatalf("CurrentRevision: %v", err)
	}
	if revision != int64(concurrency) {
		t.Fatalf("expected the real revision to advance by exactly %d, got %d", concurrency, revision)
	}

	state, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load: %v", err)
	}
	if len(state.Pools) != concurrency {
		t.Fatalf("expected exactly %d pools (no lost update), got %d", concurrency, len(state.Pools))
	}
	seen := map[string]bool{}
	for _, p := range state.Pools {
		seen[p.Name] = true
	}
	for _, name := range names {
		if !seen[name] {
			t.Fatalf("expected pool %q to have been durably saved, but it is missing", name)
		}
	}
}

// --- TestMutateObserverFires ------------------------------------------------

// TestMutateObserverFires proves the post-mutation observer seam fires
// exactly once per attempted mutation: once with outcome "success" and
// the resulting revision populated after a successful mutation, and once
// with outcome "failure" and no resulting revision after a failed one
// (07-05-PLAN.md Task 1 behavior).
func TestMutateObserverFires(t *testing.T) {
	api.ResetMutationObserversForTesting()
	t.Cleanup(api.ResetMutationObserversForTesting)

	root := t.TempDir()
	showPath := filepath.Join(root, "show.golc")
	catalog, err := routecatalog.New()
	if err != nil {
		t.Fatalf("routecatalog.New: %v", err)
	}
	server := api.NewServer(catalog, root, showPath)
	token, keyID := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	var mu sync.Mutex
	var events []api.MutationEvent
	api.RegisterMutationObserver(func(ev api.MutationEvent) {
		mu.Lock()
		defer mu.Unlock()
		events = append(events, ev)
	})

	// A successful mutation.
	ok := doCreatePoolRequest(t, server.Handler(), token, "", "Alpha")
	if ok.Code < 200 || ok.Code >= 300 {
		t.Fatalf("expected the first create to succeed, got %d (body: %s)", ok.Code, ok.Body.String())
	}

	// A failing mutation: a duplicate pool name is rejected by show.Save's
	// whole-State validation (ExitCode 1 -> a typed 5xx).
	dup := doCreatePoolRequest(t, server.Handler(), token, "", "Alpha")
	if dup.Code < 400 {
		t.Fatalf("expected the duplicate-name create to fail, got %d (body: %s)", dup.Code, dup.Body.String())
	}

	mu.Lock()
	defer mu.Unlock()
	if len(events) != 2 {
		t.Fatalf("expected exactly 2 observer events, got %d: %+v", len(events), events)
	}

	success := events[0]
	if success.Outcome != "success" {
		t.Fatalf("expected the first event's outcome to be \"success\", got %q", success.Outcome)
	}
	if success.Route != "pool create" {
		t.Fatalf("expected the first event's route to be \"pool create\", got %q", success.Route)
	}
	if success.Actor != keyID {
		t.Fatalf("expected the first event's actor to be %q, got %q", keyID, success.Actor)
	}
	if success.Source != "http" {
		t.Fatalf("expected the first event's source to be \"http\", got %q", success.Source)
	}
	if success.ResultingRevision == nil || *success.ResultingRevision != 1 {
		t.Fatalf("expected the first event's resulting revision to be 1, got %v", success.ResultingRevision)
	}
	if success.StatusCode < 200 || success.StatusCode >= 300 {
		t.Fatalf("expected the first event's status code to be 2xx, got %d", success.StatusCode)
	}

	failure := events[1]
	if failure.Outcome != "failure" {
		t.Fatalf("expected the second event's outcome to be \"failure\", got %q", failure.Outcome)
	}
	if failure.ResultingRevision != nil {
		t.Fatalf("expected the second event's resulting revision to be nil, got %v", *failure.ResultingRevision)
	}
	if failure.StatusCode < 400 {
		t.Fatalf("expected the second event's status code to be an error status, got %d", failure.StatusCode)
	}
}
