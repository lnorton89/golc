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

	"github.com/stretchr/testify/require"

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
	require.NoError(t, err, "GenerateAPIKey")
	key, err := show.InsertAPIKey(root, showPath, generated, scopes, time.Now().UTC().Add(time.Hour))
	require.NoError(t, err, "InsertAPIKey")
	return generated.RawToken, key.KeyID
}

// jsonBody canonically encodes value as an io.Reader suitable for
// httptest.NewRequest's body parameter -- shared by every *_test.go file
// in this package that issues a JSON request body.
func jsonBody(t *testing.T, value any) *bytes.Reader {
	t.Helper()
	payload, err := json.Marshal(value)
	require.NoError(t, err, "json.Marshal")
	return bytes.NewReader(payload)
}

// doCreatePoolRequest issues POST /v1/pools with body {"name": name},
// presenting token and (if non-empty) ifMatch as headers.
func doCreatePoolRequest(t *testing.T, handler http.Handler, token, ifMatch, name string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/pools", jsonBody(t, map[string]any{"name": name}))
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
	err := json.Unmarshal(rec.Body.Bytes(), &decoded)
	require.NoError(t, err, "decode mutation response %q", rec.Body.String())
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
	require.NoError(t, err, "routecatalog.New")
	server := api.NewServer(catalog, root, showPath)
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	before, err := show.CurrentRevision(root, showPath)
	require.NoError(t, err, "CurrentRevision before")
	require.Equal(t, int64(0), before, "expected a never-yet-saved show to report revision 0")

	rec := doCreatePoolRequest(t, server.Handler(), token, "0", "Main")
	require.True(t, rec.Code >= 200 && rec.Code < 300, "expected a 2xx creating with a matching If-Match, got %d (body: %s)", rec.Code, rec.Body.String())
	_, revision := decodeMutationBody(t, rec)
	require.NotNil(t, revision, "expected the response revision to be 1, got nil")
	require.Equal(t, int64(1), *revision, "expected the response revision to be 1")

	after, err := show.CurrentRevision(root, showPath)
	require.NoError(t, err, "CurrentRevision after")
	require.Equal(t, int64(1), after, "expected the real revision to be 1 after one successful mutation")

	// Repeat with the now-stale If-Match "0": must return 412 and create
	// nothing.
	stale := doCreatePoolRequest(t, server.Handler(), token, "0", "Second")
	require.Equal(t, http.StatusPreconditionFailed, stale.Code, "expected 412 for a stale If-Match (body: %s)", stale.Body.String())

	stillAfter, err := show.CurrentRevision(root, showPath)
	require.NoError(t, err, "CurrentRevision after stale attempt")
	require.Equal(t, int64(1), stillAfter, "expected the real revision to remain 1 after a rejected stale mutation")

	state, err := show.Load(root, showPath)
	require.NoError(t, err, "show.Load")
	require.Len(t, state.Pools, 1, "expected exactly one pool to exist (the stale attempt must not have created \"Second\")")
}

// --- TestMutateRequiresScope -----------------------------------------------

// TestMutateRequiresScope proves a mutation without the required coarse
// domain scope returns 403 and mutates nothing (CONTEXT D-08).
func TestMutateRequiresScope(t *testing.T) {
	root := t.TempDir()
	showPath := filepath.Join(root, "show.golc")
	catalog, err := routecatalog.New()
	require.NoError(t, err, "routecatalog.New")
	server := api.NewServer(catalog, root, showPath)
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopePlayback})

	rec := doCreatePoolRequest(t, server.Handler(), token, "", "ShouldNotExist")
	require.Equal(t, http.StatusForbidden, rec.Code, "expected 403 for a key lacking the authoring scope (body: %s)", rec.Body.String())

	revision, err := show.CurrentRevision(root, showPath)
	require.NoError(t, err, "CurrentRevision")
	require.Equal(t, int64(0), revision, "expected the real revision to remain 0 after a scope-rejected mutation")
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
	require.NoError(t, err, "routecatalog.New")
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
		require.True(t, code >= 200 && code < 300, "expected request %d to succeed, got status %d", i, code)
	}

	revision, err := show.CurrentRevision(root, showPath)
	require.NoError(t, err, "CurrentRevision")
	require.Equal(t, int64(concurrency), revision, "expected the real revision to advance by exactly %d", concurrency)

	state, err := show.Load(root, showPath)
	require.NoError(t, err, "show.Load")
	require.Len(t, state.Pools, concurrency, "expected no lost update")
	seen := map[string]bool{}
	for _, p := range state.Pools {
		seen[p.Name] = true
	}
	for _, name := range names {
		require.True(t, seen[name], "expected pool %q to have been durably saved, but it is missing", name)
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
	require.NoError(t, err, "routecatalog.New")
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
	require.True(t, ok.Code >= 200 && ok.Code < 300, "expected the first create to succeed, got %d (body: %s)", ok.Code, ok.Body.String())

	// A failing mutation: a duplicate pool name is rejected by show.Save's
	// whole-State validation (ExitCode 1 -> a typed 5xx).
	dup := doCreatePoolRequest(t, server.Handler(), token, "", "Alpha")
	require.GreaterOrEqual(t, dup.Code, 400, "expected the duplicate-name create to fail (body: %s)", dup.Body.String())

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, events, 2, "expected exactly 2 observer events: %+v", events)

	success := events[0]
	require.Equal(t, "success", success.Outcome, "expected the first event's outcome to be \"success\"")
	require.Equal(t, "pool create", success.Route, "expected the first event's route to be \"pool create\"")
	require.Equal(t, keyID, success.Actor, "expected the first event's actor to match the minted key")
	require.Equal(t, "http", success.Source, "expected the first event's source to be \"http\"")
	require.NotNil(t, success.ResultingRevision, "expected the first event's resulting revision to be 1, got nil")
	require.Equal(t, int64(1), *success.ResultingRevision, "expected the first event's resulting revision to be 1")
	require.True(t, success.StatusCode >= 200 && success.StatusCode < 300, "expected the first event's status code to be 2xx, got %d", success.StatusCode)

	failure := events[1]
	require.Equal(t, "failure", failure.Outcome, "expected the second event's outcome to be \"failure\"")
	require.Nil(t, failure.ResultingRevision, "expected the second event's resulting revision to be nil")
	require.GreaterOrEqual(t, failure.StatusCode, 400, "expected the second event's status code to be an error status")
}
