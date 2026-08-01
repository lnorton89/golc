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

	"github.com/stretchr/testify/require"

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
	require.NoError(t, err, "routecatalog.New")
	return api.NewServer(catalog, root, showPath), root, showPath
}

// requireAuditRowCount fails t unless show.QueryAuditLog(root, showPath)
// returns exactly want rows, returning them for further assertions.
func requireAuditRowCount(t *testing.T, root, showPath string, want int) []show.AuditRecord {
	t.Helper()
	records, err := show.QueryAuditLog(root, showPath)
	require.NoError(t, err, "QueryAuditLog")
	require.Len(t, records, want, "expected exactly %d audit_log row(s): %+v", want, records)
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
	err := json.Unmarshal(rec.Body.Bytes(), &decoded)
	require.NoError(t, err, "decode batch response %q", rec.Body.String())
	return decoded
}

// assertNoTempCopyLeftBehind fails t if any verifiedBackup-style temp copy
// (show.NewTempCopy's own "<path>.backup-<timestamp>" naming convention,
// internal/show/backup.go) remains next to showPath.
func assertNoTempCopyLeftBehind(t *testing.T, showPath string) {
	t.Helper()
	matches, err := filepath.Glob(showPath + ".backup-*")
	require.NoError(t, err, "glob for leftover temp copies")
	require.Empty(t, matches, "expected no leftover temp copy files")
}

// --- TestBatchAtomic ---------------------------------------------------

// TestBatchAtomic proves a fully-valid, multi-sub-request batch applies
// every sub-request in the client's exact order, with exactly one real
// revision bump for the whole batch (CONTEXT D-15).
func TestBatchAtomic(t *testing.T) {
	root := t.TempDir()
	showPath := filepath.Join(root, "show.golc")
	catalog, err := routecatalog.New()
	require.NoError(t, err, "routecatalog.New")
	server := api.NewServer(catalog, root, showPath)
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	before, err := show.CurrentRevision(root, showPath)
	require.NoError(t, err, "CurrentRevision before")
	require.Equal(t, int64(0), before, "expected a never-yet-saved show to report revision 0")

	requests := []map[string]any{
		poolCreateBatchSubRequest("Alpha"),
		poolCreateBatchSubRequest("Beta"),
		poolCreateBatchSubRequest("Gamma"),
	}
	rec := doBatchRequest(t, server.Handler(), token, "", requests)
	require.True(t, rec.Code >= 200 && rec.Code < 300, "expected a 2xx for a fully-valid batch, got %d (body: %s)", rec.Code, rec.Body.String())

	decoded := decodeBatchBody(t, rec)
	require.NotNil(t, decoded.Revision, "expected the response revision to be 1 (one bump for the whole batch)")
	require.Equal(t, int64(1), *decoded.Revision, "expected the response revision to be 1 (one bump for the whole batch)")
	require.Len(t, decoded.Results, 3)

	after, err := show.CurrentRevision(root, showPath)
	require.NoError(t, err, "CurrentRevision after")
	require.Equal(t, int64(1), after, "expected the real revision to be 1 after one atomic batch commit")

	state, err := show.Load(root, showPath)
	require.NoError(t, err, "show.Load")
	require.Len(t, state.Pools, 3)
	want := []string{"Alpha", "Beta", "Gamma"}
	for i, name := range want {
		require.Equal(t, name, state.Pools[i].Name, "expected pool %d (client order preserved)", i)
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
	require.NoError(t, err, "routecatalog.New")
	server := api.NewServer(catalog, root, showPath)
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	order := []string{"Zulu", "Alpha", "Mike"}
	requests := make([]map[string]any, len(order))
	for i, name := range order {
		requests[i] = poolCreateBatchSubRequest(name)
	}

	rec := doBatchRequest(t, server.Handler(), token, "", requests)
	require.True(t, rec.Code >= 200 && rec.Code < 300, "expected a 2xx, got %d (body: %s)", rec.Code, rec.Body.String())

	state, err := show.Load(root, showPath)
	require.NoError(t, err, "show.Load")
	require.Len(t, state.Pools, len(order))
	for i, name := range order {
		require.Equal(t, name, state.Pools[i].Name, "expected pool %d (client order preserved)", i)
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
	require.NoError(t, err, "routecatalog.New")
	server := api.NewServer(catalog, root, showPath)
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	// Seed one pool via a real single mutation first, so the batch's 2nd
	// sub-request (a duplicate name) genuinely fails show.Save's
	// whole-State validation.
	seedRec := doCreatePoolRequest(t, server.Handler(), token, "", "Beta")
	require.True(t, seedRec.Code >= 200 && seedRec.Code < 300, "seeding \"Beta\": expected a 2xx, got %d (body: %s)", seedRec.Code, seedRec.Body.String())

	before, err := show.CurrentRevision(root, showPath)
	require.NoError(t, err, "CurrentRevision before")
	require.Equal(t, int64(1), before, "expected revision 1 after seeding")

	requests := []map[string]any{
		poolCreateBatchSubRequest("Alpha"),
		poolCreateBatchSubRequest("Beta"), // duplicate name -> show.Save validation failure
		poolCreateBatchSubRequest("Gamma"),
	}
	rec := doBatchRequest(t, server.Handler(), token, "", requests)
	require.True(t, rec.Code >= 400, "expected a batch with a failing 2nd sub-request to fail, got %d (body: %s)", rec.Code, rec.Body.String())

	after, err := show.CurrentRevision(root, showPath)
	require.NoError(t, err, "CurrentRevision after")
	require.Equal(t, before, after, "expected the real revision to remain unchanged after a rolled-back batch")

	state, err := show.Load(root, showPath)
	require.NoError(t, err, "show.Load")
	require.Len(t, state.Pools, 1, "expected only the pre-existing \"Beta\" pool to exist (sub-request 1's \"Alpha\" must not have persisted): %+v", state.Pools)
	require.Equal(t, "Beta", state.Pools[0].Name, "expected only the pre-existing \"Beta\" pool to exist (sub-request 1's \"Alpha\" must not have persisted)")

	assertNoTempCopyLeftBehind(t, showPath)
}

// --- TestBatchIfMatch --------------------------------------------------

// TestBatchIfMatch proves a stale batch-level If-Match, checked at the
// batch's start, returns 412 and changes nothing (CONTEXT D-13/D-15).
func TestBatchIfMatch(t *testing.T) {
	root := t.TempDir()
	showPath := filepath.Join(root, "show.golc")
	catalog, err := routecatalog.New()
	require.NoError(t, err, "routecatalog.New")
	server := api.NewServer(catalog, root, showPath)
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	rec := doBatchRequest(t, server.Handler(), token, "5", []map[string]any{poolCreateBatchSubRequest("Alpha")})
	require.Equal(t, http.StatusPreconditionFailed, rec.Code, "expected 412 for a stale batch-level If-Match (body: %s)", rec.Body.String())

	revision, err := show.CurrentRevision(root, showPath)
	require.NoError(t, err, "CurrentRevision")
	require.Equal(t, int64(0), revision, "expected the real revision to remain 0 after a stale-If-Match batch")

	state, err := show.Load(root, showPath)
	require.NoError(t, err, "show.Load")
	require.Len(t, state.Pools, 0)
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
	require.NoError(t, err, "routecatalog.New")
	server := api.NewServer(catalog, root, showPath)
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	api.BatchPreCommitHookForTesting = func() {
		// Simulate a concurrent external writer (outside this package's
		// mutationMutex, which only ever serializes HTTP requests within
		// this one process) racing between this batch's start and its
		// commit: re-save the real show unchanged, which still bumps its
		// revision by one.
		state, loadErr := show.Load(root, showPath)
		require.NoError(t, loadErr, "simulated external Load")
		saveErr := show.Save(root, showPath, state)
		require.NoError(t, saveErr, "simulated external Save")
	}
	t.Cleanup(func() { api.BatchPreCommitHookForTesting = nil })

	rec := doBatchRequest(t, server.Handler(), token, "", []map[string]any{poolCreateBatchSubRequest("Alpha")})
	require.Equal(t, http.StatusPreconditionFailed, rec.Code, "expected 412 for a batch racing a concurrent external write (body: %s)", rec.Body.String())

	revision, err := show.CurrentRevision(root, showPath)
	require.NoError(t, err, "CurrentRevision")
	require.Equal(t, int64(1), revision, "expected only the simulated external write to have advanced the revision (to 1)")

	state, err := show.Load(root, showPath)
	require.NoError(t, err, "show.Load")
	for _, p := range state.Pools {
		require.NotEqual(t, "Alpha", p.Name, "expected the batch's \"Alpha\" pool to have been rolled back after the 412, but it exists")
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
	require.NoError(t, err, "routecatalog.New")
	server := api.NewServer(catalog, root, showPath)
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopePlayback})

	rec := doBatchRequest(t, server.Handler(), token, "", []map[string]any{poolCreateBatchSubRequest("ShouldNotExist")})
	require.Equal(t, http.StatusForbidden, rec.Code, "expected 403 for a batch sub-request whose route requires a scope the key lacks (body: %s)", rec.Body.String())

	revision, err := show.CurrentRevision(root, showPath)
	require.NoError(t, err, "CurrentRevision")
	require.Equal(t, int64(0), revision, "expected the real revision to remain 0 after a scope-rejected batch")
}

// --- TestBatchEmpty ------------------------------------------------------

// TestBatchEmpty proves an empty sub-request list is rejected 400 -- a
// no-op batch is an error, not a silent success (API-04 empty edge).
func TestBatchEmpty(t *testing.T) {
	root := t.TempDir()
	showPath := filepath.Join(root, "show.golc")
	catalog, err := routecatalog.New()
	require.NoError(t, err, "routecatalog.New")
	server := api.NewServer(catalog, root, showPath)
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	rec := doBatchRequest(t, server.Handler(), token, "", []map[string]any{})
	require.Equal(t, http.StatusBadRequest, rec.Code, "expected 400 for an empty batch (body: %s)", rec.Body.String())
	require.Contains(t, rec.Body.String(), "GOLC_API_BATCH_EMPTY", "expected the error to name GOLC_API_BATCH_EMPTY")
}

// --- TestBatchSingle -------------------------------------------------------

// TestBatchSingle proves a single-sub-request batch behaves identically
// to the equivalent single mutation: the same resulting revision bump,
// against two otherwise-identical fresh shows (API-04 single edge).
func TestBatchSingle(t *testing.T) {
	catalogSingle, err := routecatalog.New()
	require.NoError(t, err, "routecatalog.New")
	rootSingle := t.TempDir()
	showPathSingle := filepath.Join(rootSingle, "show.golc")
	serverSingle := api.NewServer(catalogSingle, rootSingle, showPathSingle)
	tokenSingle, _ := seedKey(t, rootSingle, showPathSingle, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	singleRec := doCreatePoolRequest(t, serverSingle.Handler(), tokenSingle, "", "Solo")
	require.True(t, singleRec.Code >= 200 && singleRec.Code < 300, "single mutation: expected a 2xx, got %d (body: %s)", singleRec.Code, singleRec.Body.String())
	singleResult, singleRevision := decodeMutationBody(t, singleRec)

	catalogBatch, err := routecatalog.New()
	require.NoError(t, err, "routecatalog.New")
	rootBatch := t.TempDir()
	showPathBatch := filepath.Join(rootBatch, "show.golc")
	serverBatch := api.NewServer(catalogBatch, rootBatch, showPathBatch)
	tokenBatch, _ := seedKey(t, rootBatch, showPathBatch, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	batchRec := doBatchRequest(t, serverBatch.Handler(), tokenBatch, "", []map[string]any{poolCreateBatchSubRequest("Solo")})
	require.True(t, batchRec.Code >= 200 && batchRec.Code < 300, "one-element batch: expected a 2xx, got %d (body: %s)", batchRec.Code, batchRec.Body.String())
	decoded := decodeBatchBody(t, batchRec)

	require.NotNil(t, decoded.Revision, "expected a one-element batch's resulting revision to equal the equivalent single mutation's (%v)", singleRevision)
	require.NotNil(t, singleRevision, "expected a one-element batch's resulting revision (%v) to equal the equivalent single mutation's", decoded.Revision)
	require.Equal(t, *singleRevision, *decoded.Revision, "expected a one-element batch's resulting revision to equal the equivalent single mutation's")
	require.Len(t, decoded.Results, 1)
	const wantPrefix = "GOLC_POOL_CREATED: Solo ("
	require.True(t, strings.HasPrefix(decoded.Results[0].Result, wantPrefix) && strings.HasPrefix(singleResult, wantPrefix),
		"expected both outcomes to start with %q (same effect, different pool ids), got batch=%q single=%q",
		wantPrefix, decoded.Results[0].Result, singleResult)
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
	require.NoError(t, err, "routecatalog.New")
	server := api.NewServer(catalog, root, showPath)
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	requests := []map[string]any{
		poolCreateBatchSubRequest("Alpha"),
		poolCreateBatchSubRequest("Alpha"), // duplicate name -> fails at index 1
		poolCreateBatchSubRequest("Gamma"),
	}
	rec := doBatchRequest(t, server.Handler(), token, "", requests)
	require.True(t, rec.Code >= 400, "expected a batch with a failing 2nd sub-request to fail, got %d (body: %s)", rec.Code, rec.Body.String())

	body := rec.Body.String()
	require.Contains(t, body, "sub-request 1", "expected the failure response to name the failing sub-request's index (1)")
	require.Contains(t, body, "rolled back", "expected the failure response to state the whole batch was rolled back")
	require.Contains(t, body, "1 earlier sub-request", "expected the failure response to report exactly 1 earlier successful-so-far sub-request as not durably applied")

	revision, err := show.CurrentRevision(root, showPath)
	require.NoError(t, err, "CurrentRevision")
	require.Equal(t, int64(0), revision, "expected the real revision to remain 0 (nothing durably applied, including sub-request 0's successful-so-far effect)")

	state, err := show.Load(root, showPath)
	require.NoError(t, err, "show.Load")
	require.Len(t, state.Pools, 0, "expected no pools to exist (sub-request 0's \"Alpha\" was rolled back along with the whole batch)")
}

// --- TestBatchNoTempCopyLeftBehind -----------------------------------------

// TestBatchNoTempCopyLeftBehind proves the throwaway VACUUM INTO copy
// (internal/show.NewTempCopy) is always deleted, both after a successful
// batch and after a failed one (Task 1's own acceptance criteria).
func TestBatchNoTempCopyLeftBehind(t *testing.T) {
	root := t.TempDir()
	showPath := filepath.Join(root, "show.golc")
	catalog, err := routecatalog.New()
	require.NoError(t, err, "routecatalog.New")
	server := api.NewServer(catalog, root, showPath)
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	successRec := doBatchRequest(t, server.Handler(), token, "", []map[string]any{poolCreateBatchSubRequest("Alpha")})
	require.True(t, successRec.Code >= 200 && successRec.Code < 300, "expected a 2xx, got %d (body: %s)", successRec.Code, successRec.Body.String())
	assertNoTempCopyLeftBehind(t, showPath)

	failRec := doBatchRequest(t, server.Handler(), token, "", []map[string]any{poolCreateBatchSubRequest("Alpha")})
	require.True(t, failRec.Code >= 400, "expected a duplicate-name batch to fail, got %d (body: %s)", failRec.Code, failRec.Body.String())
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
	require.Equal(t, http.StatusForbidden, rec.Code, "expected 403 for a batch sub-request whose route requires a scope the key lacks (body: %s)", rec.Body.String())

	revision, err := show.CurrentRevision(root, showPath)
	require.NoError(t, err, "CurrentRevision")
	require.Equal(t, int64(0), revision, "expected the real revision to remain 0 after a scope-rejected batch")

	records := requireAuditRowCount(t, root, showPath, 1)
	rec0 := records[0]
	require.Equal(t, "failure", rec0.Outcome)
	require.Equal(t, http.StatusForbidden, rec0.StatusCode)
	require.Equal(t, "http", rec0.Source)
	require.NotEmpty(t, rec0.Actor, "expected a non-empty actor")
	require.NotEmpty(t, rec0.CorrelationID, "expected a non-empty correlation id")
	require.False(t, rec0.ResultingRevision.Valid, "expected a null resulting_revision for a rejected sub-request, got %v", rec0.ResultingRevision)
	require.Equal(t, "pool create", rec0.Route)
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
	require.Equal(t, http.StatusForbidden, singleRec.Code, "expected 403 for the single-mutation scope rejection (body: %s)", singleRec.Body.String())

	batchRec := doBatchRequest(t, server.Handler(), token, "", []map[string]any{poolCreateBatchSubRequest("ShouldNotExistBatch")})
	require.Equal(t, http.StatusForbidden, batchRec.Code, "expected 403 for the batch scope rejection (body: %s)", batchRec.Body.String())

	records := requireAuditRowCount(t, root, showPath, 2)
	for i, rec := range records {
		require.Equal(t, "failure", rec.Outcome, "row %d", i)
		require.Equal(t, http.StatusForbidden, rec.StatusCode, "row %d", i)
		require.Equal(t, "pool create", rec.Route, "row %d", i)
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
	require.Equal(t, http.StatusBadRequest, rec.Code, "expected 400 for an unsupported batch sub-request (body: %s)", rec.Body.String())
	require.Contains(t, rec.Body.String(), "GOLC_API_BATCH_SUBREQUEST_UNSUPPORTED", "expected the error to name GOLC_API_BATCH_SUBREQUEST_UNSUPPORTED")

	records := requireAuditRowCount(t, root, showPath, 1)
	rec0 := records[0]
	require.Equal(t, "failure", rec0.Outcome)
	require.Equal(t, http.StatusBadRequest, rec0.StatusCode)
	require.Contains(t, rec0.RedactedDetails, "/v1/widgets", "expected redacted_details to record the claimed target (/v1/widgets)")
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
	require.True(t, rec.Code >= 200 && rec.Code < 300, "expected a 2xx for a fully-valid batch, got %d (body: %s)", rec.Code, rec.Body.String())

	records := requireAuditRowCount(t, root, showPath, len(order))
	var sharedRevision int64
	for i, name := range order {
		got := records[i]
		require.Equal(t, "success", got.Outcome, "row %d", i)
		require.True(t, got.ResultingRevision.Valid, "row %d: expected a non-null resulting_revision", i)
		if i == 0 {
			sharedRevision = got.ResultingRevision.Int64
		} else {
			require.Equal(t, sharedRevision, got.ResultingRevision.Int64, "row %d: expected the shared resulting_revision", i)
		}
		require.Contains(t, got.RedactedDetails, name, "row %d: expected redacted_details to contain %q (this sub-request's own name)", i, name)
		for _, other := range order {
			if other == name {
				continue
			}
			require.NotContains(t, got.RedactedDetails, other, "row %d: expected redacted_details NOT to contain %q (a different sub-request's name)", i, other)
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
	require.NoError(t, err, "routecatalog.New")
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

	require.Equal(t, http.StatusBadRequest, rec.Code, "expected 400 for a \"requires\" element containing a comma (body: %s)", rec.Body.String())
	require.Contains(t, rec.Body.String(), "GOLC_API_LIST_VALUE_INVALID", "expected the error to name GOLC_API_LIST_VALUE_INVALID")

	revision, err := show.CurrentRevision(root, showPath)
	require.NoError(t, err, "CurrentRevision")
	require.Equal(t, int64(0), revision, "expected the show's revision to remain unchanged (0)")
}

// TestCreatePoolAllowsCommaFreeRequires proves a "requires" list with
// ordinary comma-free elements still succeeds exactly as before
// (IN-02 regression guard: the new boundary rule must not reject valid
// input).
func TestCreatePoolAllowsCommaFreeRequires(t *testing.T) {
	root := t.TempDir()
	showPath := filepath.Join(root, "show.golc")
	catalog, err := routecatalog.New()
	require.NoError(t, err, "routecatalog.New")
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

	require.True(t, rec.Code >= 200 && rec.Code < 300, "expected a 2xx for comma-free \"requires\" elements, got %d (body: %s)", rec.Code, rec.Body.String())
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
	require.Equal(t, http.StatusBadRequest, rec.Code, "expected 400 for a batch sub-request's \"requires\" element containing a comma (body: %s)", rec.Body.String())
	require.Contains(t, rec.Body.String(), "GOLC_API_LIST_VALUE_INVALID", "expected the error to name GOLC_API_LIST_VALUE_INVALID")

	revision, err := show.CurrentRevision(root, showPath)
	require.NoError(t, err, "CurrentRevision")
	require.Equal(t, int64(0), revision, "expected the show's revision to remain unchanged (0)")

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
	require.Equal(t, http.StatusBadRequest, rec.Code, "expected 400 for an empty batch (body: %s)", rec.Body.String())
	require.Contains(t, rec.Body.String(), "GOLC_API_BATCH_EMPTY", "expected the error to name GOLC_API_BATCH_EMPTY")

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
	require.Equal(t, http.StatusPreconditionFailed, rec.Code, "expected 412 for a stale batch-level If-Match (body: %s)", rec.Body.String())

	// Keep TestBatchIfMatch's own atomicity assertions alongside the new
	// audit ones.
	revision, err := show.CurrentRevision(root, showPath)
	require.NoError(t, err, "CurrentRevision")
	require.Equal(t, int64(0), revision, "expected the real revision to remain 0 after a stale-If-Match batch")
	state, err := show.Load(root, showPath)
	require.NoError(t, err, "show.Load")
	require.Len(t, state.Pools, 0)

	records := requireAuditRowCount(t, root, showPath, len(names))
	for i, name := range names {
		got := records[i]
		require.Equal(t, "failure", got.Outcome, "row %d", i)
		require.Equal(t, http.StatusPreconditionFailed, got.StatusCode, "row %d", i)
		require.Equal(t, "http", got.Source, "row %d", i)
		require.NotEmpty(t, got.Actor, "row %d: expected a non-empty actor", i)
		require.NotEmpty(t, got.CorrelationID, "row %d: expected a non-empty correlation id", i)
		require.Equal(t, "pool create", got.Route, "row %d", i)
		require.False(t, got.ResultingRevision.Valid, "row %d: expected a null resulting_revision, got %v", i, got.ResultingRevision)
		require.True(t, got.ExpectedRevision.Valid, "row %d: expected a valid expected_revision of 5", i)
		require.Equal(t, int64(5), got.ExpectedRevision.Int64, "row %d: expected a valid expected_revision of 5", i)
		// The fan-out must be per-sub-request and correctly ordered: row i's
		// details must contain that sub-request's own name and none of the
		// others', so a collapsed single row, a duplicated row, or a
		// reordering all fail.
		require.Contains(t, got.RedactedDetails, name, "row %d: expected redacted_details to contain %q (this sub-request's own name)", i, name)
		for _, other := range names {
			if other == name {
				continue
			}
			require.NotContains(t, got.RedactedDetails, other, "row %d: expected redacted_details NOT to contain %q (a different sub-request's name)", i, other)
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
	require.Equal(t, http.StatusPreconditionFailed, singleRec.Code, "expected 412 for the single-mutation stale If-Match (body: %s)", singleRec.Body.String())

	batchRec := doBatchRequest(t, server.Handler(), token, "9", []map[string]any{poolCreateBatchSubRequest("ShouldNotExistBatch")})
	require.Equal(t, http.StatusPreconditionFailed, batchRec.Code, "expected 412 for the batch stale If-Match (body: %s)", batchRec.Body.String())

	records := requireAuditRowCount(t, root, showPath, 2)
	for i, rec := range records {
		require.Equal(t, "failure", rec.Outcome, "row %d", i)
		require.Equal(t, http.StatusPreconditionFailed, rec.StatusCode, "row %d", i)
		require.Equal(t, "pool create", rec.Route, "row %d", i)
		require.True(t, rec.ExpectedRevision.Valid, "row %d: expected a valid expected_revision of 9", i)
		require.Equal(t, int64(9), rec.ExpectedRevision.Int64, "row %d: expected a valid expected_revision of 9", i)
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
	require.Equal(t, http.StatusBadRequest, rec.Code, "expected 400 for an unparseable batch-level If-Match (body: %s)", rec.Body.String())
	require.Contains(t, rec.Body.String(), "GOLC_API_IF_MATCH_INVALID", "expected the error to name GOLC_API_IF_MATCH_INVALID")

	records := requireAuditRowCount(t, root, showPath, 1)
	rec0 := records[0]
	require.Equal(t, "failure", rec0.Outcome)
	require.Equal(t, http.StatusBadRequest, rec0.StatusCode)
	require.False(t, rec0.ExpectedRevision.Valid, "expected a null expected_revision (nothing was ever successfully parsed), got %v", rec0.ExpectedRevision)
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
		require.NoError(t, loadErr, "simulated external Load")
		saveErr := show.Save(root, showPath, state)
		require.NoError(t, saveErr, "simulated external Save")
	}
	t.Cleanup(func() { api.BatchPreCommitHookForTesting = nil })

	names := []string{"Alpha", "Bravo"}
	requests := make([]map[string]any, len(names))
	for i, name := range names {
		requests[i] = poolCreateBatchSubRequest(name)
	}

	rec := doBatchRequest(t, server.Handler(), token, "", requests)
	require.Equal(t, http.StatusPreconditionFailed, rec.Code, "expected 412 for a batch racing a concurrent external write (body: %s)", rec.Body.String())

	revision, err := show.CurrentRevision(root, showPath)
	require.NoError(t, err, "CurrentRevision")
	require.Equal(t, int64(1), revision, "expected only the simulated external write to have advanced the revision (to 1)")
	state, err := show.Load(root, showPath)
	require.NoError(t, err, "show.Load")
	for _, p := range state.Pools {
		require.False(t, p.Name == "Alpha" || p.Name == "Bravo", "expected the batch's sub-requests to have been rolled back after the 412, but %q exists", p.Name)
	}

	records := requireAuditRowCount(t, root, showPath, len(names))
	for i, name := range names {
		got := records[i]
		require.Equal(t, "failure", got.Outcome, "row %d", i)
		require.Equal(t, http.StatusPreconditionFailed, got.StatusCode, "row %d", i)
		require.False(t, got.ExpectedRevision.Valid, "row %d: expected a null expected_revision (no If-Match was sent), got %v", i, got.ExpectedRevision)
		require.Contains(t, got.RedactedDetails, name, "row %d: expected redacted_details to contain %q (this sub-request's own name)", i, name)
		for _, other := range names {
			if other == name {
				continue
			}
			require.NotContains(t, got.RedactedDetails, other, "row %d: expected redacted_details NOT to contain %q (a different sub-request's name)", i, other)
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
	require.True(t, seedRec.Code >= 200 && seedRec.Code < 300, "seeding \"Beta\": expected a 2xx, got %d (body: %s)", seedRec.Code, seedRec.Body.String())
	postSeedRevision, err := show.CurrentRevision(root, showPath)
	require.NoError(t, err, "CurrentRevision after seed")

	requests := []map[string]any{
		poolCreateBatchSubRequest("Alpha"),
		poolCreateBatchSubRequest("Beta"), // duplicate name -> fails against the throwaway copy at index 1
	}
	rec := doBatchRequest(t, server.Handler(), token, "", requests)
	require.True(t, rec.Code >= 400, "expected a batch with a failing 2nd sub-request to fail, got %d (body: %s)", rec.Code, rec.Body.String())

	revision, err := show.CurrentRevision(root, showPath)
	require.NoError(t, err, "CurrentRevision")
	require.Equal(t, postSeedRevision, revision, "expected the real revision to remain unchanged from the post-seed value")

	records := requireAuditRowCount(t, root, showPath, 2)
	seedRow := records[0]
	require.Equal(t, "success", seedRow.Outcome, "row 0 (the seed)")

	failRow := records[1]
	require.Equal(t, "failure", failRow.Outcome, "row 1")
	require.Equal(t, "pool create", failRow.Route, "row 1")
	require.False(t, failRow.ResultingRevision.Valid, "row 1: expected a null resulting_revision, got %v", failRow.ResultingRevision)
	require.Contains(t, failRow.RedactedDetails, "Beta", "row 1")
	// The absence of an "Alpha" failure row is the pinned semantic: a
	// sub-request-attributable failure writes exactly one row, for the
	// culpable index -- sub-request 0 succeeded against the throwaway copy
	// and was then rolled back along with the whole batch, so it gets no
	// row of its own.
	require.NotContains(t, failRow.RedactedDetails, "Alpha", "row 1: expected redacted_details NOT to contain a non-culpable sub-request's name")
	// Deliberate: the audit row's status must be whatever the client
	// actually received, which is what makes the row usable for
	// reconciling a client-reported failure against the server's own
	// record -- assert against rec.Code, never a hardcoded status.
	require.Equal(t, rec.Code, failRow.StatusCode, "row 1: expected status to equal the response's own status")
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
	require.NoError(t, err, "os.ReadFile(batch.go)")
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
	require.False(t, startLine < 0 || endLine < 0 || endLine <= startLine,
		"expected batch.go to contain a `mutationMutex.Lock()` line followed later by a `resultingRevision := baseRevision + 1` line -- runBatch was restructured and this test's region markers need updating (found startLine=%d, endLine=%d)", startLine, endLine)

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
			require.True(t, fired, "batch.go line %d is an unaudited failure return inside runBatch's locked section: %q -- every failure return inside runBatch's locked section must emit its audit rows before returning (API-06, WR-05)", i+1, trimmed)
			fired = false
			returnCount++
		}
	}

	const wantCount = 9
	require.Equal(t, wantCount, returnCount, "expected exactly %d failure returns in runBatch's locked section -- if a failure return was legitimately added or removed there, update this expectation and confirm the new branch fires the observer", wantCount)
	require.Equal(t, wantCount, fireCount, "expected exactly %d audit-fire statements in runBatch's locked section -- if a failure return was legitimately added or removed there, update this expectation and confirm the new branch fires the observer", wantCount)
}
