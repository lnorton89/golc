// batch_test.go pins batch.go's atomic /v1/batch engine (07-06-PLAN.md
// Task 1/2): a fully-valid ordered batch applies all sub-requests in
// order with exactly one real revision bump; a mid-batch failure leaves
// the real show and its revision completely untouched, including any
// earlier sub-request's effect that had already succeeded against the
// throwaway copy; a stale batch-level If-Match, or a real revision that
// changed underneath the batch between its start and its commit, both
// return 412 and change nothing; an empty batch is rejected 400; a
// one-element batch behaves identically to the equivalent single
// mutation; a failure response names the failing sub-request's index and
// diagnostic; and no throwaway temp copy is ever left behind, on success
// or failure.
//
// This file lives in the external api_test package (see coverage_test.go's
// doc comment for why) so it can reach a real, live command registry
// through internal/routecatalog's test-only bridge for a genuine
// "pool create" -> show.Save round trip against a throwaway copy, not a
// canned stub outcome. It reuses jsonBody/seedKey/doCreatePoolRequest/
// decodeMutationBody from mutate_test.go (same package).
package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lnorton89/golc/internal/api"
	"github.com/lnorton89/golc/internal/routecatalog"
	"github.com/lnorton89/golc/internal/show"
)

// poolCreateBatchSubRequest builds one "pool create" batch sub-request
// body, matching batch.go's translateBatchCreatePool's expected shape.
func poolCreateBatchSubRequest(name string) map[string]any {
	return map[string]any{
		"method":   "POST",
		"resource": "/v1/pools",
		"body":     map[string]any{"name": name},
	}
}

// doBatchRequest issues POST /v1/batch with body {"requests": requests},
// presenting token and (if non-empty) ifMatch as headers.
func doBatchRequest(t *testing.T, handler http.Handler, token, ifMatch string, requests []map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/batch", jsonBody(t, map[string]any{"requests": requests}))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	if ifMatch != "" {
		req.Header.Set("If-Match", ifMatch)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// batchDecodedResponse is the {results, revision} shape a successful
// batch's response body decodes into.
type batchDecodedResponse struct {
	Results []struct {
		Index  int    `json:"index"`
		Result string `json:"result"`
	} `json:"results"`
	Revision *int64 `json:"revision"`
}

// decodeBatchBody decodes rec's body into batchDecodedResponse.
func decodeBatchBody(t *testing.T, rec *httptest.ResponseRecorder) batchDecodedResponse {
	t.Helper()
	var decoded batchDecodedResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("decode batch response %q: %v", rec.Body.String(), err)
	}
	return decoded
}

// assertNoTempCopyLeftBehind fails t if any verifiedBackup-style temp copy
// (show.NewTempCopy's own "<path>.backup-<timestamp>" naming convention,
// internal/show/backup.go) remains next to showPath.
func assertNoTempCopyLeftBehind(t *testing.T, showPath string) {
	t.Helper()
	matches, err := filepath.Glob(showPath + ".backup-*")
	if err != nil {
		t.Fatalf("glob for leftover temp copies: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("expected no leftover temp copy files, found: %v", matches)
	}
}

// --- TestBatchAtomic ---------------------------------------------------

// TestBatchAtomic proves a fully-valid, multi-sub-request batch applies
// every sub-request in the client's exact order, with exactly one real
// revision bump for the whole batch (CONTEXT D-15).
func TestBatchAtomic(t *testing.T) {
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

	requests := []map[string]any{
		poolCreateBatchSubRequest("Alpha"),
		poolCreateBatchSubRequest("Beta"),
		poolCreateBatchSubRequest("Gamma"),
	}
	rec := doBatchRequest(t, server.Handler(), token, "", requests)
	if rec.Code < 200 || rec.Code >= 300 {
		t.Fatalf("expected a 2xx for a fully-valid batch, got %d (body: %s)", rec.Code, rec.Body.String())
	}

	decoded := decodeBatchBody(t, rec)
	if decoded.Revision == nil || *decoded.Revision != 1 {
		t.Fatalf("expected the response revision to be 1 (one bump for the whole batch), got %v", decoded.Revision)
	}
	if len(decoded.Results) != 3 {
		t.Fatalf("expected 3 results, got %d: %+v", len(decoded.Results), decoded.Results)
	}

	after, err := show.CurrentRevision(root, showPath)
	if err != nil {
		t.Fatalf("CurrentRevision after: %v", err)
	}
	if after != 1 {
		t.Fatalf("expected the real revision to be 1 after one atomic batch commit, got %d", after)
	}

	state, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load: %v", err)
	}
	if len(state.Pools) != 3 {
		t.Fatalf("expected exactly 3 pools, got %d", len(state.Pools))
	}
	want := []string{"Alpha", "Beta", "Gamma"}
	for i, name := range want {
		if state.Pools[i].Name != name {
			t.Fatalf("expected pool %d to be %q (client order preserved), got %q", i, name, state.Pools[i].Name)
		}
	}

	assertNoTempCopyLeftBehind(t, showPath)
}

// --- TestBatchOrder ------------------------------------------------------

// TestBatchOrder proves sub-requests apply in the exact client-specified
// order, using a deliberately non-alphabetical order to rule out any
// implicit reordering (CONTEXT D-15, API-04 ordering edge).
func TestBatchOrder(t *testing.T) {
	root := t.TempDir()
	showPath := filepath.Join(root, "show.golc")
	catalog, err := routecatalog.New()
	if err != nil {
		t.Fatalf("routecatalog.New: %v", err)
	}
	server := api.NewServer(catalog, root, showPath)
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	order := []string{"Zulu", "Alpha", "Mike"}
	requests := make([]map[string]any, len(order))
	for i, name := range order {
		requests[i] = poolCreateBatchSubRequest(name)
	}

	rec := doBatchRequest(t, server.Handler(), token, "", requests)
	if rec.Code < 200 || rec.Code >= 300 {
		t.Fatalf("expected a 2xx, got %d (body: %s)", rec.Code, rec.Body.String())
	}

	state, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load: %v", err)
	}
	if len(state.Pools) != len(order) {
		t.Fatalf("expected %d pools, got %d", len(order), len(state.Pools))
	}
	for i, name := range order {
		if state.Pools[i].Name != name {
			t.Fatalf("expected pool %d to be %q (client order preserved), got %q", i, name, state.Pools[i].Name)
		}
	}
}

// --- TestBatchRollback -----------------------------------------------------

// TestBatchRollback proves a 3-sub-request batch whose 2nd sub-request
// fails leaves the real show completely unchanged: the real revision does
// not advance, and sub-request 1's effect (which had already succeeded
// against the throwaway copy) is never durably applied (CONTEXT D-15,
// 07-RESEARCH.md Pitfall 1).
func TestBatchRollback(t *testing.T) {
	root := t.TempDir()
	showPath := filepath.Join(root, "show.golc")
	catalog, err := routecatalog.New()
	if err != nil {
		t.Fatalf("routecatalog.New: %v", err)
	}
	server := api.NewServer(catalog, root, showPath)
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	// Seed one pool via a real single mutation first, so the batch's 2nd
	// sub-request (a duplicate name) genuinely fails show.Save's
	// whole-State validation.
	seedRec := doCreatePoolRequest(t, server.Handler(), token, "", "Beta")
	if seedRec.Code < 200 || seedRec.Code >= 300 {
		t.Fatalf("seeding \"Beta\": expected a 2xx, got %d (body: %s)", seedRec.Code, seedRec.Body.String())
	}

	before, err := show.CurrentRevision(root, showPath)
	if err != nil {
		t.Fatalf("CurrentRevision before: %v", err)
	}
	if before != 1 {
		t.Fatalf("expected revision 1 after seeding, got %d", before)
	}

	requests := []map[string]any{
		poolCreateBatchSubRequest("Alpha"),
		poolCreateBatchSubRequest("Beta"), // duplicate name -> show.Save validation failure
		poolCreateBatchSubRequest("Gamma"),
	}
	rec := doBatchRequest(t, server.Handler(), token, "", requests)
	if rec.Code < 400 {
		t.Fatalf("expected a batch with a failing 2nd sub-request to fail, got %d (body: %s)", rec.Code, rec.Body.String())
	}

	after, err := show.CurrentRevision(root, showPath)
	if err != nil {
		t.Fatalf("CurrentRevision after: %v", err)
	}
	if after != before {
		t.Fatalf("expected the real revision to remain unchanged after a rolled-back batch, got %d (was %d)", after, before)
	}

	state, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load: %v", err)
	}
	if len(state.Pools) != 1 || state.Pools[0].Name != "Beta" {
		t.Fatalf("expected only the pre-existing \"Beta\" pool to exist (sub-request 1's \"Alpha\" must not have persisted), got %d pools: %+v",
			len(state.Pools), state.Pools)
	}

	assertNoTempCopyLeftBehind(t, showPath)
}

// --- TestBatchIfMatch --------------------------------------------------

// TestBatchIfMatch proves a stale batch-level If-Match, checked at the
// batch's start, returns 412 and changes nothing (CONTEXT D-13/D-15).
func TestBatchIfMatch(t *testing.T) {
	root := t.TempDir()
	showPath := filepath.Join(root, "show.golc")
	catalog, err := routecatalog.New()
	if err != nil {
		t.Fatalf("routecatalog.New: %v", err)
	}
	server := api.NewServer(catalog, root, showPath)
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	rec := doBatchRequest(t, server.Handler(), token, "5", []map[string]any{poolCreateBatchSubRequest("Alpha")})
	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("expected 412 for a stale batch-level If-Match, got %d (body: %s)", rec.Code, rec.Body.String())
	}

	revision, err := show.CurrentRevision(root, showPath)
	if err != nil {
		t.Fatalf("CurrentRevision: %v", err)
	}
	if revision != 0 {
		t.Fatalf("expected the real revision to remain 0 after a stale-If-Match batch, got %d", revision)
	}

	state, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load: %v", err)
	}
	if len(state.Pools) != 0 {
		t.Fatalf("expected no pools to exist, got %d", len(state.Pools))
	}
}

// TestBatchIfMatchExternalRace proves a batch racing a concurrent
// external write (simulated via BatchPreCommitHookForTesting -- a real
// external writer, e.g. a CLI process, would bypass this package's
// in-process mutationMutex entirely) is rejected 412 at its final
// pre-commit check, and changes nothing beyond the external write itself
// (07-RESEARCH.md Pitfall 1 residual race, batch-bypass threat T-07-10).
func TestBatchIfMatchExternalRace(t *testing.T) {
	root := t.TempDir()
	showPath := filepath.Join(root, "show.golc")
	catalog, err := routecatalog.New()
	if err != nil {
		t.Fatalf("routecatalog.New: %v", err)
	}
	server := api.NewServer(catalog, root, showPath)
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	api.BatchPreCommitHookForTesting = func() {
		// Simulate a concurrent external writer (outside this package's
		// mutationMutex, which only ever serializes HTTP requests within
		// this one process) racing between this batch's start and its
		// commit: re-save the real show unchanged, which still bumps its
		// revision by one.
		state, loadErr := show.Load(root, showPath)
		if loadErr != nil {
			t.Fatalf("simulated external Load: %v", loadErr)
		}
		if saveErr := show.Save(root, showPath, state); saveErr != nil {
			t.Fatalf("simulated external Save: %v", saveErr)
		}
	}
	t.Cleanup(func() { api.BatchPreCommitHookForTesting = nil })

	rec := doBatchRequest(t, server.Handler(), token, "", []map[string]any{poolCreateBatchSubRequest("Alpha")})
	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("expected 412 for a batch racing a concurrent external write, got %d (body: %s)", rec.Code, rec.Body.String())
	}

	revision, err := show.CurrentRevision(root, showPath)
	if err != nil {
		t.Fatalf("CurrentRevision: %v", err)
	}
	if revision != 1 {
		t.Fatalf("expected only the simulated external write to have advanced the revision (to 1), got %d", revision)
	}

	state, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load: %v", err)
	}
	for _, p := range state.Pools {
		if p.Name == "Alpha" {
			t.Fatalf("expected the batch's \"Alpha\" pool to have been rolled back after the 412, but it exists")
		}
	}

	assertNoTempCopyLeftBehind(t, showPath)
}

// --- TestBatchRequiresScope ----------------------------------------------

// TestBatchRequiresScope proves a batch whose sub-request targets a route
// the authenticated key lacks the required coarse domain scope for is
// rejected 403 and mutates nothing (CONTEXT D-08; Rule 2 -- batch must not
// be a scope-bypass vector around the single-mutation pipeline's own
// scope gate, mutate.go's requiredScopeForRoute/RequireScope).
func TestBatchRequiresScope(t *testing.T) {
	root := t.TempDir()
	showPath := filepath.Join(root, "show.golc")
	catalog, err := routecatalog.New()
	if err != nil {
		t.Fatalf("routecatalog.New: %v", err)
	}
	server := api.NewServer(catalog, root, showPath)
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopePlayback})

	rec := doBatchRequest(t, server.Handler(), token, "", []map[string]any{poolCreateBatchSubRequest("ShouldNotExist")})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for a batch sub-request whose route requires a scope the key lacks, got %d (body: %s)", rec.Code, rec.Body.String())
	}

	revision, err := show.CurrentRevision(root, showPath)
	if err != nil {
		t.Fatalf("CurrentRevision: %v", err)
	}
	if revision != 0 {
		t.Fatalf("expected the real revision to remain 0 after a scope-rejected batch, got %d", revision)
	}
}

// --- TestBatchEmpty ------------------------------------------------------

// TestBatchEmpty proves an empty sub-request list is rejected 400 -- a
// no-op batch is an error, not a silent success (API-04 empty edge).
func TestBatchEmpty(t *testing.T) {
	root := t.TempDir()
	showPath := filepath.Join(root, "show.golc")
	catalog, err := routecatalog.New()
	if err != nil {
		t.Fatalf("routecatalog.New: %v", err)
	}
	server := api.NewServer(catalog, root, showPath)
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	rec := doBatchRequest(t, server.Handler(), token, "", []map[string]any{})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for an empty batch, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "GOLC_API_BATCH_EMPTY") {
		t.Fatalf("expected the error to name GOLC_API_BATCH_EMPTY, got: %s", rec.Body.String())
	}
}

// --- TestBatchSingle -------------------------------------------------------

// TestBatchSingle proves a single-sub-request batch behaves identically
// to the equivalent single mutation: the same resulting revision bump,
// against two otherwise-identical fresh shows (API-04 single edge).
func TestBatchSingle(t *testing.T) {
	catalogSingle, err := routecatalog.New()
	if err != nil {
		t.Fatalf("routecatalog.New: %v", err)
	}
	rootSingle := t.TempDir()
	showPathSingle := filepath.Join(rootSingle, "show.golc")
	serverSingle := api.NewServer(catalogSingle, rootSingle, showPathSingle)
	tokenSingle, _ := seedKey(t, rootSingle, showPathSingle, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	singleRec := doCreatePoolRequest(t, serverSingle.Handler(), tokenSingle, "", "Solo")
	if singleRec.Code < 200 || singleRec.Code >= 300 {
		t.Fatalf("single mutation: expected a 2xx, got %d (body: %s)", singleRec.Code, singleRec.Body.String())
	}
	singleResult, singleRevision := decodeMutationBody(t, singleRec)

	catalogBatch, err := routecatalog.New()
	if err != nil {
		t.Fatalf("routecatalog.New: %v", err)
	}
	rootBatch := t.TempDir()
	showPathBatch := filepath.Join(rootBatch, "show.golc")
	serverBatch := api.NewServer(catalogBatch, rootBatch, showPathBatch)
	tokenBatch, _ := seedKey(t, rootBatch, showPathBatch, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	batchRec := doBatchRequest(t, serverBatch.Handler(), tokenBatch, "", []map[string]any{poolCreateBatchSubRequest("Solo")})
	if batchRec.Code < 200 || batchRec.Code >= 300 {
		t.Fatalf("one-element batch: expected a 2xx, got %d (body: %s)", batchRec.Code, batchRec.Body.String())
	}
	decoded := decodeBatchBody(t, batchRec)

	if decoded.Revision == nil || singleRevision == nil || *decoded.Revision != *singleRevision {
		t.Fatalf("expected a one-element batch's resulting revision (%v) to equal the equivalent single mutation's (%v)", decoded.Revision, singleRevision)
	}
	if len(decoded.Results) != 1 {
		t.Fatalf("expected exactly 1 result, got %d", len(decoded.Results))
	}
	const wantPrefix = "GOLC_POOL_CREATED: Solo ("
	if !strings.HasPrefix(decoded.Results[0].Result, wantPrefix) || !strings.HasPrefix(singleResult, wantPrefix) {
		t.Fatalf("expected both outcomes to start with %q (same effect, different pool ids), got batch=%q single=%q",
			wantPrefix, decoded.Results[0].Result, singleResult)
	}
}

// --- TestBatchFailureReport ------------------------------------------------

// TestBatchFailureReport proves a batch failure response names the
// failing sub-request's index and diagnostic, and states plainly that
// earlier successful-so-far sub-requests were not durably applied
// (Task 2's own acceptance criteria).
func TestBatchFailureReport(t *testing.T) {
	root := t.TempDir()
	showPath := filepath.Join(root, "show.golc")
	catalog, err := routecatalog.New()
	if err != nil {
		t.Fatalf("routecatalog.New: %v", err)
	}
	server := api.NewServer(catalog, root, showPath)
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	requests := []map[string]any{
		poolCreateBatchSubRequest("Alpha"),
		poolCreateBatchSubRequest("Alpha"), // duplicate name -> fails at index 1
		poolCreateBatchSubRequest("Gamma"),
	}
	rec := doBatchRequest(t, server.Handler(), token, "", requests)
	if rec.Code < 400 {
		t.Fatalf("expected a batch with a failing 2nd sub-request to fail, got %d (body: %s)", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	if !strings.Contains(body, "sub-request 1") {
		t.Fatalf("expected the failure response to name the failing sub-request's index (1), got: %s", body)
	}
	if !strings.Contains(body, "rolled back") {
		t.Fatalf("expected the failure response to state the whole batch was rolled back, got: %s", body)
	}
	if !strings.Contains(body, "1 earlier sub-request") {
		t.Fatalf("expected the failure response to report exactly 1 earlier successful-so-far sub-request as not durably applied, got: %s", body)
	}

	revision, err := show.CurrentRevision(root, showPath)
	if err != nil {
		t.Fatalf("CurrentRevision: %v", err)
	}
	if revision != 0 {
		t.Fatalf("expected the real revision to remain 0 (nothing durably applied, including sub-request 0's successful-so-far effect), got %d", revision)
	}

	state, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load: %v", err)
	}
	if len(state.Pools) != 0 {
		t.Fatalf("expected no pools to exist (sub-request 0's \"Alpha\" was rolled back along with the whole batch), got %d", len(state.Pools))
	}
}

// --- TestBatchNoTempCopyLeftBehind -----------------------------------------

// TestBatchNoTempCopyLeftBehind proves the throwaway VACUUM INTO copy
// (internal/show.NewTempCopy) is always deleted, both after a successful
// batch and after a failed one (Task 1's own acceptance criteria).
func TestBatchNoTempCopyLeftBehind(t *testing.T) {
	root := t.TempDir()
	showPath := filepath.Join(root, "show.golc")
	catalog, err := routecatalog.New()
	if err != nil {
		t.Fatalf("routecatalog.New: %v", err)
	}
	server := api.NewServer(catalog, root, showPath)
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	successRec := doBatchRequest(t, server.Handler(), token, "", []map[string]any{poolCreateBatchSubRequest("Alpha")})
	if successRec.Code < 200 || successRec.Code >= 300 {
		t.Fatalf("expected a 2xx, got %d (body: %s)", successRec.Code, successRec.Body.String())
	}
	assertNoTempCopyLeftBehind(t, showPath)

	failRec := doBatchRequest(t, server.Handler(), token, "", []map[string]any{poolCreateBatchSubRequest("Alpha")})
	if failRec.Code < 400 {
		t.Fatalf("expected a duplicate-name batch to fail, got %d (body: %s)", failRec.Code, failRec.Body.String())
	}
	assertNoTempCopyLeftBehind(t, showPath)
}
