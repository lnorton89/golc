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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lnorton89/golc/internal/api"
	"github.com/lnorton89/golc/internal/routecatalog"
	"github.com/lnorton89/golc/internal/show"
)

// newAuditedBatchServer builds a fresh *api.Server against its own
// t.TempDir() root (NewServer's own doc comment: registers that root's
// audit observer at construction time), so show.QueryAuditLog(root,
// showPath) below sees exactly the rows this test's own traffic
// produced -- no other test's accumulated observer writes to this root.
func newAuditedBatchServer(t *testing.T) (server *api.Server, root, showPath string) {
	t.Helper()
	root = t.TempDir()
	showPath = filepath.Join(root, "show.golc")
	catalog, err := routecatalog.New()
	if err != nil {
		t.Fatalf("routecatalog.New: %v", err)
	}
	return api.NewServer(catalog, root, showPath), root, showPath
}

// requireAuditRowCount fails t unless show.QueryAuditLog(root, showPath)
// returns exactly want rows, returning them for further assertions.
func requireAuditRowCount(t *testing.T, root, showPath string, want int) []show.AuditRecord {
	t.Helper()
	records, err := show.QueryAuditLog(root, showPath)
	if err != nil {
		t.Fatalf("QueryAuditLog: %v", err)
	}
	if len(records) != want {
		t.Fatalf("expected exactly %d audit_log row(s), got %d: %+v", want, len(records), records)
	}
	return records
}

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

// --- TestBatchScopeRejectionIsAudited ---------------------------------

// TestBatchScopeRejectionIsAudited proves a batch sub-request rejected for
// a missing domain scope produces exactly one audit_log row (actor, source
// "http", correlation id, route, outcome "failure", status 403, null
// resulting_revision) -- identical in kind to the row the equivalent
// single POST /v1/pools rejection already writes (API-06, D-08, D-16;
// 07-REVIEW.md WR-02, 07-VERIFICATION.md gap 3). The revision-unchanged
// assertion from TestBatchRequiresScope's own pattern is kept alongside
// it, so atomicity stays pinned together with auditability.
func TestBatchScopeRejectionIsAudited(t *testing.T) {
	server, root, showPath := newAuditedBatchServer(t)
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

	records := requireAuditRowCount(t, root, showPath, 1)
	rec0 := records[0]
	if rec0.Outcome != "failure" {
		t.Fatalf("expected outcome %q, got %q", "failure", rec0.Outcome)
	}
	if rec0.StatusCode != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec0.StatusCode)
	}
	if rec0.Source != "http" {
		t.Fatalf("expected source %q, got %q", "http", rec0.Source)
	}
	if rec0.Actor == "" {
		t.Fatalf("expected a non-empty actor")
	}
	if rec0.CorrelationID == "" {
		t.Fatalf("expected a non-empty correlation id")
	}
	if rec0.ResultingRevision.Valid {
		t.Fatalf("expected a null resulting_revision for a rejected sub-request, got %v", rec0.ResultingRevision)
	}
	if rec0.Route != "pool create" {
		t.Fatalf("expected route %q, got %q", "pool create", rec0.Route)
	}
}

// --- TestBatchAndSingleMutationScopeRejectionsAuditIdentically --------

// TestBatchAndSingleMutationScopeRejectionsAuditIdentically proves an
// identical scope-rejected attempt through POST /v1/pools and through
// POST /v1/batch leaves identical evidence -- the parity assertion that
// makes the WR-02 regression impossible to reintroduce on only one of the
// two surfaces (API-06).
func TestBatchAndSingleMutationScopeRejectionsAuditIdentically(t *testing.T) {
	server, root, showPath := newAuditedBatchServer(t)
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopePlayback})

	singleRec := doCreatePoolRequest(t, server.Handler(), token, "", "ShouldNotExistSingle")
	if singleRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for the single-mutation scope rejection, got %d (body: %s)", singleRec.Code, singleRec.Body.String())
	}

	batchRec := doBatchRequest(t, server.Handler(), token, "", []map[string]any{poolCreateBatchSubRequest("ShouldNotExistBatch")})
	if batchRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for the batch scope rejection, got %d (body: %s)", batchRec.Code, batchRec.Body.String())
	}

	records := requireAuditRowCount(t, root, showPath, 2)
	for i, rec := range records {
		if rec.Outcome != "failure" {
			t.Fatalf("row %d: expected outcome %q, got %q", i, "failure", rec.Outcome)
		}
		if rec.StatusCode != http.StatusForbidden {
			t.Fatalf("row %d: expected status %d, got %d", i, http.StatusForbidden, rec.StatusCode)
		}
		if rec.Route != "pool create" {
			t.Fatalf("row %d: expected route %q, got %q", i, "pool create", rec.Route)
		}
	}
}

// --- TestBatchTranslationFailureIsAudited -----------------------------

// TestBatchTranslationFailureIsAudited proves a batch sub-request whose
// method+resource has no registered translator produces exactly one
// audit_log row recording the client's own claimed target, so a probe for
// unsupported endpoints is auditable too (API-06).
func TestBatchTranslationFailureIsAudited(t *testing.T) {
	server, root, showPath := newAuditedBatchServer(t)
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	unsupported := map[string]any{
		"method":   "POST",
		"resource": "/v1/widgets",
	}
	rec := doBatchRequest(t, server.Handler(), token, "", []map[string]any{unsupported})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for an unsupported batch sub-request, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "GOLC_API_BATCH_SUBREQUEST_UNSUPPORTED") {
		t.Fatalf("expected the error to name GOLC_API_BATCH_SUBREQUEST_UNSUPPORTED, got: %s", rec.Body.String())
	}

	records := requireAuditRowCount(t, root, showPath, 1)
	rec0 := records[0]
	if rec0.Outcome != "failure" {
		t.Fatalf("expected outcome %q, got %q", "failure", rec0.Outcome)
	}
	if rec0.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec0.StatusCode)
	}
	if !strings.Contains(rec0.RedactedDetails, "/v1/widgets") {
		t.Fatalf("expected redacted_details to record the claimed target (/v1/widgets), got: %s", rec0.RedactedDetails)
	}
}

// --- TestBatchSubRequestAuditRowsFollowClientOrder ---------------------

// TestBatchSubRequestAuditRowsFollowClientOrder proves a successful
// multi-sub-request batch's audit rows appear in the client's sub-request
// order, distinguishable by each row's redacted_details args (API-06
// ordering edge).
func TestBatchSubRequestAuditRowsFollowClientOrder(t *testing.T) {
	server, root, showPath := newAuditedBatchServer(t)
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	order := []string{"Zulu", "Alpha", "Mike"}
	requests := make([]map[string]any, len(order))
	for i, name := range order {
		requests[i] = poolCreateBatchSubRequest(name)
	}

	rec := doBatchRequest(t, server.Handler(), token, "", requests)
	if rec.Code < 200 || rec.Code >= 300 {
		t.Fatalf("expected a 2xx for a fully-valid batch, got %d (body: %s)", rec.Code, rec.Body.String())
	}

	records := requireAuditRowCount(t, root, showPath, len(order))
	var sharedRevision int64
	for i, name := range order {
		got := records[i]
		if got.Outcome != "success" {
			t.Fatalf("row %d: expected outcome %q, got %q", i, "success", got.Outcome)
		}
		if !got.ResultingRevision.Valid {
			t.Fatalf("row %d: expected a non-null resulting_revision", i)
		}
		if i == 0 {
			sharedRevision = got.ResultingRevision.Int64
		} else if got.ResultingRevision.Int64 != sharedRevision {
			t.Fatalf("row %d: expected the shared resulting_revision %d, got %d", i, sharedRevision, got.ResultingRevision.Int64)
		}
		if !strings.Contains(got.RedactedDetails, name) {
			t.Fatalf("row %d: expected redacted_details to contain %q (this sub-request's own name), got: %s", i, name, got.RedactedDetails)
		}
		for _, other := range order {
			if other == name {
				continue
			}
			if strings.Contains(got.RedactedDetails, other) {
				t.Fatalf("row %d: expected redacted_details NOT to contain %q (a different sub-request's name), got: %s", i, other, got.RedactedDetails)
			}
		}
	}
}

// --- TestCreatePoolRejectsCommaInRequires -------------------------------

// TestCreatePoolRejectsCommaInRequires proves POST /v1/pools rejects a
// "requires" element containing a comma with a typed 400 naming
// GOLC_API_LIST_VALUE_INVALID, and creates nothing (IN-02, 07-REVIEW.md):
// the comma is the reserved delimiter registerCreatePool's own
// strings.Join uses to build the downstream --requires CLI argument, so a
// value containing one must never silently split into two.
func TestCreatePoolRejectsCommaInRequires(t *testing.T) {
	root := t.TempDir()
	showPath := filepath.Join(root, "show.golc")
	catalog, err := routecatalog.New()
	if err != nil {
		t.Fatalf("routecatalog.New: %v", err)
	}
	server := api.NewServer(catalog, root, showPath)
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	req := httptest.NewRequest(http.MethodPost, "/v1/pools", jsonBody(t, map[string]any{
		"name":     "ShouldNotExist",
		"requires": []string{"color", "pan,tilt"},
	}))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a \"requires\" element containing a comma, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "GOLC_API_LIST_VALUE_INVALID") {
		t.Fatalf("expected the error to name GOLC_API_LIST_VALUE_INVALID, got: %s", rec.Body.String())
	}

	revision, err := show.CurrentRevision(root, showPath)
	if err != nil {
		t.Fatalf("CurrentRevision: %v", err)
	}
	if revision != 0 {
		t.Fatalf("expected the show's revision to remain unchanged (0), got %d", revision)
	}
}

// TestCreatePoolAllowsCommaFreeRequires proves a "requires" list with
// ordinary comma-free elements still succeeds exactly as before
// (IN-02 regression guard: the new boundary rule must not reject valid
// input).
func TestCreatePoolAllowsCommaFreeRequires(t *testing.T) {
	root := t.TempDir()
	showPath := filepath.Join(root, "show.golc")
	catalog, err := routecatalog.New()
	if err != nil {
		t.Fatalf("routecatalog.New: %v", err)
	}
	server := api.NewServer(catalog, root, showPath)
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	req := httptest.NewRequest(http.MethodPost, "/v1/pools", jsonBody(t, map[string]any{
		"name":     "Main",
		"requires": []string{"color", "pan"},
	}))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code < 200 || rec.Code >= 300 {
		t.Fatalf("expected a 2xx for comma-free \"requires\" elements, got %d (body: %s)", rec.Code, rec.Body.String())
	}
}

// --- TestBatchSubRequestRejectsCommaInRequires --------------------------

// poolCreateBatchSubRequestWithRequires builds a "pool create" batch
// sub-request body carrying a "requires" list, mirroring
// poolCreateBatchSubRequest but with an explicit requires slice.
func poolCreateBatchSubRequestWithRequires(name string, requires []string) map[string]any {
	return map[string]any{
		"method":   "POST",
		"resource": "/v1/pools",
		"body":     map[string]any{"name": name, "requires": requires},
	}
}

// TestBatchSubRequestRejectsCommaInRequires proves the equivalent /v1/batch
// sub-request rejects a comma-bearing "requires" element with the same
// GOLC_API_LIST_VALUE_INVALID diagnostic, surfaced through the batch's own
// sub-request error wrapper, creates nothing, leaves no leftover temp
// copy, and (per 07-12) writes exactly one audit row (IN-02, 07-REVIEW.md).
func TestBatchSubRequestRejectsCommaInRequires(t *testing.T) {
	server, root, showPath := newAuditedBatchServer(t)
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	requests := []map[string]any{
		poolCreateBatchSubRequestWithRequires("ShouldNotExist", []string{"pan,tilt"}),
	}
	rec := doBatchRequest(t, server.Handler(), token, "", requests)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a batch sub-request's \"requires\" element containing a comma, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "GOLC_API_LIST_VALUE_INVALID") {
		t.Fatalf("expected the error to name GOLC_API_LIST_VALUE_INVALID, got: %s", rec.Body.String())
	}

	revision, err := show.CurrentRevision(root, showPath)
	if err != nil {
		t.Fatalf("CurrentRevision: %v", err)
	}
	if revision != 0 {
		t.Fatalf("expected the show's revision to remain unchanged (0), got %d", revision)
	}

	assertNoTempCopyLeftBehind(t, showPath)
	requireAuditRowCount(t, root, showPath, 1)
}

// --- TestBatchEmptyWritesNoAuditRow -------------------------------------

// TestBatchEmptyWritesNoAuditRow proves an empty batch is rejected 400 and
// writes ZERO audit rows -- no sub-request was ever identified, so there
// is no attempted mutation to record (API-06 empty edge).
func TestBatchEmptyWritesNoAuditRow(t *testing.T) {
	server, root, showPath := newAuditedBatchServer(t)
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	rec := doBatchRequest(t, server.Handler(), token, "", []map[string]any{})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for an empty batch, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "GOLC_API_BATCH_EMPTY") {
		t.Fatalf("expected the error to name GOLC_API_BATCH_EMPTY, got: %s", rec.Body.String())
	}

	requireAuditRowCount(t, root, showPath, 0)
}

// --- TestBatchStaleIfMatchIsAudited --------------------------------------

// TestBatchStaleIfMatchIsAudited proves a stale batch-level If-Match (the
// most consequential of runBatch's nine locked-section failure returns --
// a routine optimistic-concurrency conflict, not an adversarial probe)
// writes one failure audit row PER SUB-REQUEST, each carrying the client's
// claimed expected_revision and its own sub-request's name in
// redacted_details, closing 07-REVIEW-gaps.md WR-05 / 07-VERIFICATION.md's
// sole remaining gap (API-06).
func TestBatchStaleIfMatchIsAudited(t *testing.T) {
	server, root, showPath := newAuditedBatchServer(t)
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	names := []string{"Alpha", "Bravo"}
	requests := make([]map[string]any, len(names))
	for i, name := range names {
		requests[i] = poolCreateBatchSubRequest(name)
	}

	rec := doBatchRequest(t, server.Handler(), token, "5", requests)
	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("expected 412 for a stale batch-level If-Match, got %d (body: %s)", rec.Code, rec.Body.String())
	}

	// Keep TestBatchIfMatch's own atomicity assertions alongside the new
	// audit ones.
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

	records := requireAuditRowCount(t, root, showPath, len(names))
	for i, name := range names {
		got := records[i]
		if got.Outcome != "failure" {
			t.Fatalf("row %d: expected outcome %q, got %q", i, "failure", got.Outcome)
		}
		if got.StatusCode != http.StatusPreconditionFailed {
			t.Fatalf("row %d: expected status %d, got %d", i, http.StatusPreconditionFailed, got.StatusCode)
		}
		if got.Source != "http" {
			t.Fatalf("row %d: expected source %q, got %q", i, "http", got.Source)
		}
		if got.Actor == "" {
			t.Fatalf("row %d: expected a non-empty actor", i)
		}
		if got.CorrelationID == "" {
			t.Fatalf("row %d: expected a non-empty correlation id", i)
		}
		if got.Route != "pool create" {
			t.Fatalf("row %d: expected route %q, got %q", i, "pool create", got.Route)
		}
		if got.ResultingRevision.Valid {
			t.Fatalf("row %d: expected a null resulting_revision, got %v", i, got.ResultingRevision)
		}
		if !got.ExpectedRevision.Valid || got.ExpectedRevision.Int64 != 5 {
			t.Fatalf("row %d: expected a valid expected_revision of 5, got %v", i, got.ExpectedRevision)
		}
		// The fan-out must be per-sub-request and correctly ordered: row i's
		// details must contain that sub-request's own name and none of the
		// others', so a collapsed single row, a duplicated row, or a
		// reordering all fail.
		if !strings.Contains(got.RedactedDetails, name) {
			t.Fatalf("row %d: expected redacted_details to contain %q (this sub-request's own name), got: %s", i, name, got.RedactedDetails)
		}
		for _, other := range names {
			if other == name {
				continue
			}
			if strings.Contains(got.RedactedDetails, other) {
				t.Fatalf("row %d: expected redacted_details NOT to contain %q (a different sub-request's name), got: %s", i, other, got.RedactedDetails)
			}
		}
	}
}

// --- TestBatchAndSingleMutationStaleIfMatchAuditIdentically --------------

// TestBatchAndSingleMutationStaleIfMatchAuditIdentically is the
// precondition-path counterpart of
// TestBatchAndSingleMutationScopeRejectionsAuditIdentically (07-12): it
// proves an identical stale-If-Match rejection through POST /v1/pools and
// through POST /v1/batch leaves identical evidence, so the WR-05
// regression cannot be reintroduced on only one of the two surfaces.
func TestBatchAndSingleMutationStaleIfMatchAuditIdentically(t *testing.T) {
	server, root, showPath := newAuditedBatchServer(t)
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	singleRec := doCreatePoolRequest(t, server.Handler(), token, "9", "ShouldNotExistSingle")
	if singleRec.Code != http.StatusPreconditionFailed {
		t.Fatalf("expected 412 for the single-mutation stale If-Match, got %d (body: %s)", singleRec.Code, singleRec.Body.String())
	}

	batchRec := doBatchRequest(t, server.Handler(), token, "9", []map[string]any{poolCreateBatchSubRequest("ShouldNotExistBatch")})
	if batchRec.Code != http.StatusPreconditionFailed {
		t.Fatalf("expected 412 for the batch stale If-Match, got %d (body: %s)", batchRec.Code, batchRec.Body.String())
	}

	records := requireAuditRowCount(t, root, showPath, 2)
	for i, rec := range records {
		if rec.Outcome != "failure" {
			t.Fatalf("row %d: expected outcome %q, got %q", i, "failure", rec.Outcome)
		}
		if rec.StatusCode != http.StatusPreconditionFailed {
			t.Fatalf("row %d: expected status %d, got %d", i, http.StatusPreconditionFailed, rec.StatusCode)
		}
		if rec.Route != "pool create" {
			t.Fatalf("row %d: expected route %q, got %q", i, "pool create", rec.Route)
		}
		if !rec.ExpectedRevision.Valid || rec.ExpectedRevision.Int64 != 9 {
			t.Fatalf("row %d: expected a valid expected_revision of 9, got %v", i, rec.ExpectedRevision)
		}
	}
}

// --- TestBatchMalformedIfMatchIsAudited -----------------------------------

// TestBatchMalformedIfMatchIsAudited proves an unparseable batch-level
// If-Match writes one failure row with status 400 and a NULL
// expected_revision -- NULL precisely because nothing was ever
// successfully parsed, distinguishing a rejected header (this test) from a
// mismatched one (TestBatchStaleIfMatchIsAudited) in the audit trail
// itself.
func TestBatchMalformedIfMatchIsAudited(t *testing.T) {
	server, root, showPath := newAuditedBatchServer(t)
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	rec := doBatchRequest(t, server.Handler(), token, "banana", []map[string]any{poolCreateBatchSubRequest("ShouldNotExist")})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for an unparseable batch-level If-Match, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "GOLC_API_IF_MATCH_INVALID") {
		t.Fatalf("expected the error to name GOLC_API_IF_MATCH_INVALID, got: %s", rec.Body.String())
	}

	records := requireAuditRowCount(t, root, showPath, 1)
	rec0 := records[0]
	if rec0.Outcome != "failure" {
		t.Fatalf("expected outcome %q, got %q", "failure", rec0.Outcome)
	}
	if rec0.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec0.StatusCode)
	}
	if rec0.ExpectedRevision.Valid {
		t.Fatalf("expected a null expected_revision (nothing was ever successfully parsed), got %v", rec0.ExpectedRevision)
	}
}

// --- TestBatchExternalWriteRaceIsAudited ----------------------------------

// TestBatchExternalWriteRaceIsAudited proves the pre-commit external-write
// race writes one failure row per sub-request, even though every
// sub-request had already succeeded against the throwaway copy -- the row
// set records that the whole batch was attempted and rolled back, not that
// nothing happened.
func TestBatchExternalWriteRaceIsAudited(t *testing.T) {
	server, root, showPath := newAuditedBatchServer(t)
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	api.BatchPreCommitHookForTesting = func() {
		state, loadErr := show.Load(root, showPath)
		if loadErr != nil {
			t.Fatalf("simulated external Load: %v", loadErr)
		}
		if saveErr := show.Save(root, showPath, state); saveErr != nil {
			t.Fatalf("simulated external Save: %v", saveErr)
		}
	}
	t.Cleanup(func() { api.BatchPreCommitHookForTesting = nil })

	names := []string{"Alpha", "Bravo"}
	requests := make([]map[string]any, len(names))
	for i, name := range names {
		requests[i] = poolCreateBatchSubRequest(name)
	}

	rec := doBatchRequest(t, server.Handler(), token, "", requests)
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
		if p.Name == "Alpha" || p.Name == "Bravo" {
			t.Fatalf("expected the batch's sub-requests to have been rolled back after the 412, but %q exists", p.Name)
		}
	}

	records := requireAuditRowCount(t, root, showPath, len(names))
	for i, name := range names {
		got := records[i]
		if got.Outcome != "failure" {
			t.Fatalf("row %d: expected outcome %q, got %q", i, "failure", got.Outcome)
		}
		if got.StatusCode != http.StatusPreconditionFailed {
			t.Fatalf("row %d: expected status %d, got %d", i, http.StatusPreconditionFailed, got.StatusCode)
		}
		if got.ExpectedRevision.Valid {
			t.Fatalf("row %d: expected a null expected_revision (no If-Match was sent), got %v", i, got.ExpectedRevision)
		}
		if !strings.Contains(got.RedactedDetails, name) {
			t.Fatalf("row %d: expected redacted_details to contain %q (this sub-request's own name), got: %s", i, name, got.RedactedDetails)
		}
		for _, other := range names {
			if other == name {
				continue
			}
			if strings.Contains(got.RedactedDetails, other) {
				t.Fatalf("row %d: expected redacted_details NOT to contain %q (a different sub-request's name), got: %s", i, other, got.RedactedDetails)
			}
		}
	}
}

// --- TestBatchSubRequestExecutionFailureIsAudited -------------------------

// TestBatchSubRequestExecutionFailureIsAudited proves a mid-batch
// sub-request execution failure writes exactly ONE failure row, for the
// culpable index only, whose status equals the HTTP status the client
// received -- preserving 07-12's established "one row for the failing
// sub-request" semantic for sub-request-attributable failures.
func TestBatchSubRequestExecutionFailureIsAudited(t *testing.T) {
	server, root, showPath := newAuditedBatchServer(t)
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	// Seed "Beta" through a successful single mutation -- this legitimately
	// writes one success row, and its post-seed revision is what the
	// rolled-back batch below must leave untouched.
	seedRec := doCreatePoolRequest(t, server.Handler(), token, "", "Beta")
	if seedRec.Code < 200 || seedRec.Code >= 300 {
		t.Fatalf("seeding \"Beta\": expected a 2xx, got %d (body: %s)", seedRec.Code, seedRec.Body.String())
	}
	postSeedRevision, err := show.CurrentRevision(root, showPath)
	if err != nil {
		t.Fatalf("CurrentRevision after seed: %v", err)
	}

	requests := []map[string]any{
		poolCreateBatchSubRequest("Alpha"),
		poolCreateBatchSubRequest("Beta"), // duplicate name -> fails against the throwaway copy at index 1
	}
	rec := doBatchRequest(t, server.Handler(), token, "", requests)
	if rec.Code < 400 {
		t.Fatalf("expected a batch with a failing 2nd sub-request to fail, got %d (body: %s)", rec.Code, rec.Body.String())
	}

	revision, err := show.CurrentRevision(root, showPath)
	if err != nil {
		t.Fatalf("CurrentRevision: %v", err)
	}
	if revision != postSeedRevision {
		t.Fatalf("expected the real revision to remain unchanged from the post-seed value (%d), got %d", postSeedRevision, revision)
	}

	records := requireAuditRowCount(t, root, showPath, 2)
	seedRow := records[0]
	if seedRow.Outcome != "success" {
		t.Fatalf("row 0 (the seed): expected outcome %q, got %q", "success", seedRow.Outcome)
	}

	failRow := records[1]
	if failRow.Outcome != "failure" {
		t.Fatalf("row 1: expected outcome %q, got %q", "failure", failRow.Outcome)
	}
	if failRow.Route != "pool create" {
		t.Fatalf("row 1: expected route %q, got %q", "pool create", failRow.Route)
	}
	if failRow.ResultingRevision.Valid {
		t.Fatalf("row 1: expected a null resulting_revision, got %v", failRow.ResultingRevision)
	}
	if !strings.Contains(failRow.RedactedDetails, "Beta") {
		t.Fatalf("row 1: expected redacted_details to contain %q, got: %s", "Beta", failRow.RedactedDetails)
	}
	// The absence of an "Alpha" failure row is the pinned semantic: a
	// sub-request-attributable failure writes exactly one row, for the
	// culpable index -- sub-request 0 succeeded against the throwaway copy
	// and was then rolled back along with the whole batch, so it gets no
	// row of its own.
	if strings.Contains(failRow.RedactedDetails, "Alpha") {
		t.Fatalf("row 1: expected redacted_details NOT to contain %q (a non-culpable sub-request's name), got: %s", "Alpha", failRow.RedactedDetails)
	}
	// Deliberate: the audit row's status must be whatever the client
	// actually received, which is what makes the row usable for
	// reconciling a client-reported failure against the server's own
	// record -- assert against rec.Code, never a hardcoded status.
	if failRow.StatusCode != rec.Code {
		t.Fatalf("row 1: expected status to equal the response's own status %d, got %d", rec.Code, failRow.StatusCode)
	}
}

// --- TestBatchLockedSectionFailureReturnsAreAllAudited --------------------

// TestBatchLockedSectionFailureReturnsAreAllAudited is a source-structure
// test, the deliberate substitute for behavioral coverage of the five
// branches that need fault injection to reach (both show.CurrentRevision
// calls, show.NewTempCopy, show.Load, and show.Save failing): the show
// file is also the auth middleware's own key store, so corrupting it to
// force an infrastructure failure gets the request rejected at 401 before
// runBatch is ever entered, and adding a production fault-injection seam
// purely for audit-trail completeness would widen the blast radius well
// beyond this gap-closure fix. This test reads batch.go's own source (the
// same technique deprecation_test.go's
// TestBuildRouterInstallsDeprecationMiddleware uses against router.go) and
// mechanically proves every one of the nine `return nil, ` statements
// between `mutationMutex.Lock()` and `resultingRevision := baseRevision +
// 1` is preceded by an audit fire -- the gate that caught the source
// findings' own undercount (07-REVIEW-gaps.md and 07-VERIFICATION.md each
// enumerated eight returns by hand; this region has nine).
func TestBatchLockedSectionFailureReturnsAreAllAudited(t *testing.T) {
	source, err := os.ReadFile("batch.go")
	if err != nil {
		t.Fatalf("os.ReadFile(batch.go): %v", err)
	}
	lines := strings.Split(string(source), "\n")

	startLine, endLine := -1, -1
	for i, line := range lines {
		// Skip comment lines when locating the region markers themselves --
		// runBatch's own doc comment above the function references
		// "mutationMutex.Lock()" in prose, which must never be mistaken for
		// the real statement that opens the locked region.
		if strings.HasPrefix(strings.TrimSpace(line), "//") {
			continue
		}
		if startLine < 0 && strings.Contains(line, "mutationMutex.Lock()") {
			startLine = i
			continue
		}
		if startLine >= 0 && strings.Contains(line, "resultingRevision := baseRevision + 1") {
			endLine = i
			break
		}
	}
	if startLine < 0 || endLine < 0 || endLine <= startLine {
		t.Fatalf("expected batch.go to contain a `mutationMutex.Lock()` line followed later by a `resultingRevision := baseRevision + 1` line -- runBatch was restructured and this test's region markers need updating (found startLine=%d, endLine=%d)", startLine, endLine)
	}

	fired := false
	returnCount, fireCount := 0, 0
	for i := startLine + 1; i < endLine; i++ {
		trimmed := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trimmed, "//") {
			// A doc comment can never satisfy the gate.
			continue
		}
		if strings.Contains(trimmed, "fireMutationObservers(") || strings.Contains(trimmed, "fireBatchFailureObservers(") {
			fired = true
			fireCount++
		}
		if strings.HasPrefix(trimmed, "return nil, ") {
			if !fired {
				t.Fatalf("batch.go line %d is an unaudited failure return inside runBatch's locked section: %q -- every failure return inside runBatch's locked section must emit its audit rows before returning (API-06, WR-05)", i+1, trimmed)
			}
			fired = false
			returnCount++
		}
	}

	const wantCount = 9
	if returnCount != wantCount {
		t.Fatalf("expected exactly %d failure returns in runBatch's locked section, found %d -- if a failure return was legitimately added or removed there, update this expectation and confirm the new branch fires the observer", wantCount, returnCount)
	}
	if fireCount != wantCount {
		t.Fatalf("expected exactly %d audit-fire statements in runBatch's locked section, found %d -- if a failure return was legitimately added or removed there, update this expectation and confirm the new branch fires the observer", wantCount, fireCount)
	}
}
