// dryrun_test.go pins dryrun.go's ?dry_run=true preview contract
// (07-05-PLAN.md Task 2, CONTEXT D-14): a dry-run returns the projected
// result and leaves the real show.State.Revision unchanged, a dry-run of
// an invalid mutation surfaces the same validation error the real
// mutation would without touching the real show, the throwaway VACUUM
// INTO copy is always deleted afterward, and the post-mutation observer
// fires with outcome "dry_run" (never "success").
//
// This file lives in the external api_test package (see coverage_test.go's
// doc comment for why) so it can reach a real, live command registry
// through internal/routecatalog's test-only bridge -- a dry-run's whole
// point is to Execute a genuine "pool create" against a throwaway copy,
// not a canned stub outcome.
package api_test

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/api"
	"github.com/lnorton89/golc/internal/routecatalog"
	"github.com/lnorton89/golc/internal/show"
)

// doDryRunCreatePoolRequest issues POST /v1/pools?dry_run=true with body
// {"name": name}, presenting token as a bearer credential.
func doDryRunCreatePoolRequest(t *testing.T, handler http.Handler, token, name string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/pools?dry_run=true", jsonBody(t, map[string]any{"name": name}))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// --- TestDryRunLeavesRealRevisionUnchanged ---------------------------------

// TestDryRunLeavesRealRevisionUnchanged proves POST /v1/pools?dry_run=true
// returns the projected result with HTTP 200, and the real
// show.State.Revision is unchanged afterward.
func TestDryRunLeavesRealRevisionUnchanged(t *testing.T) {
	root := t.TempDir()
	showPath := filepath.Join(root, "show.golc")
	catalog, err := routecatalog.New()
	require.NoError(t, err, "routecatalog.New")
	server := api.NewServer(catalog, root, showPath)
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	before, err := show.CurrentRevision(root, showPath)
	require.NoError(t, err, "CurrentRevision before")

	rec := doDryRunCreatePoolRequest(t, server.Handler(), token, "PreviewPool")
	require.Equal(t, http.StatusOK, rec.Code, "expected 200 for a dry-run create (body: %s)", rec.Body.String())
	result, revision := decodeMutationBody(t, rec)
	require.NotEmpty(t, result, "expected a non-empty projected result")
	require.Nil(t, revision, "expected no revision in a dry-run response")

	after, err := show.CurrentRevision(root, showPath)
	require.NoError(t, err, "CurrentRevision after")
	require.Equal(t, before, after, "expected the real revision to stay unchanged after a dry-run")

	state, err := show.Load(root, showPath)
	require.NoError(t, err, "show.Load")
	require.Empty(t, state.Pools, "expected the real show to have no pools after a dry-run")
}

// --- TestDryRunSurfacesValidationErrorWithoutMutating ----------------------

// TestDryRunSurfacesValidationErrorWithoutMutating proves a dry-run of an
// invalid mutation (a duplicate pool name) returns the same validation
// error the real mutation would, without mutating the real show.
func TestDryRunSurfacesValidationErrorWithoutMutating(t *testing.T) {
	root := t.TempDir()
	showPath := filepath.Join(root, "show.golc")
	catalog, err := routecatalog.New()
	require.NoError(t, err, "routecatalog.New")
	server := api.NewServer(catalog, root, showPath)
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	// Create a real pool named "Dup" first.
	created := doCreatePoolRequest(t, server.Handler(), token, "", "Dup")
	require.True(t, created.Code >= 200 && created.Code < 300, "expected the setup create to succeed, got %d (body: %s)", created.Code, created.Body.String())
	afterSetup, err := show.CurrentRevision(root, showPath)
	require.NoError(t, err, "CurrentRevision after setup")

	// A dry-run attempting to create another pool named "Dup" must fail
	// with the same duplicate-name validation error the real mutation
	// would produce.
	rec := doDryRunCreatePoolRequest(t, server.Handler(), token, "Dup")
	require.GreaterOrEqual(t, rec.Code, 400, "expected the dry-run of a duplicate-name create to fail, got %d (body: %s)", rec.Code, rec.Body.String())

	after, err := show.CurrentRevision(root, showPath)
	require.NoError(t, err, "CurrentRevision after dry-run failure")
	require.Equal(t, afterSetup, after, "expected the real revision to remain unchanged after a failed dry-run")

	state, err := show.Load(root, showPath)
	require.NoError(t, err, "show.Load")
	require.Len(t, state.Pools, 1, "expected exactly the one real \"Dup\" pool from setup")
}

// --- TestDryRunLeavesNoTempCopy ---------------------------------------------

// TestDryRunLeavesNoTempCopy proves the throwaway VACUUM INTO copy
// dryRunMutate creates is deleted before the request returns, for both a
// successful and a failing dry-run.
func TestDryRunLeavesNoTempCopy(t *testing.T) {
	root := t.TempDir()
	showPath := filepath.Join(root, "show.golc")
	catalog, err := routecatalog.New()
	require.NoError(t, err, "routecatalog.New")
	server := api.NewServer(catalog, root, showPath)
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	doDryRunCreatePoolRequest(t, server.Handler(), token, "Leftover")
	assertNoBackupFiles(t, showPath)

	created := doCreatePoolRequest(t, server.Handler(), token, "", "Existing")
	require.True(t, created.Code >= 200 && created.Code < 300, "expected the setup create to succeed, got %d (body: %s)", created.Code, created.Body.String())
	doDryRunCreatePoolRequest(t, server.Handler(), token, "Existing") // duplicate -> dry-run failure path
	assertNoBackupFiles(t, showPath)
}

// assertNoBackupFiles fails t if any "<showPath>.backup-*" temp copy
// exists (internal/show.NewTempCopy's naming convention, verifiedBackup's
// own doc comment).
func assertNoBackupFiles(t *testing.T, showPath string) {
	t.Helper()
	matches, err := filepath.Glob(showPath + ".backup-*")
	require.NoError(t, err, "filepath.Glob")
	require.Empty(t, matches, "expected no leftover temp copy files")
}

// --- TestDryRunObserverOutcome -----------------------------------------------

// TestDryRunObserverOutcome proves a successful dry-run fires no
// "success" observer -- only "dry_run" (07-05-PLAN.md Task 2 behavior).
func TestDryRunObserverOutcome(t *testing.T) {
	api.ResetMutationObserversForTesting()
	t.Cleanup(api.ResetMutationObserversForTesting)

	root := t.TempDir()
	showPath := filepath.Join(root, "show.golc")
	catalog, err := routecatalog.New()
	require.NoError(t, err, "routecatalog.New")
	server := api.NewServer(catalog, root, showPath)
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	var events []api.MutationEvent
	api.RegisterMutationObserver(func(ev api.MutationEvent) {
		events = append(events, ev)
	})

	rec := doDryRunCreatePoolRequest(t, server.Handler(), token, "Observed")
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	require.Len(t, events, 1, "expected exactly 1 observer event: %+v", events)
	require.Equal(t, "dry_run", events[0].Outcome)
	require.Nil(t, events[0].ResultingRevision, "expected no resulting revision on a dry-run event")
}
