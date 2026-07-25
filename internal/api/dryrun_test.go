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
	if err != nil {
		t.Fatalf("routecatalog.New: %v", err)
	}
	server := api.NewServer(catalog, root, showPath)
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	before, err := show.CurrentRevision(root, showPath)
	if err != nil {
		t.Fatalf("CurrentRevision before: %v", err)
	}

	rec := doDryRunCreatePoolRequest(t, server.Handler(), token, "PreviewPool")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for a dry-run create, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	result, revision := decodeMutationBody(t, rec)
	if result == "" {
		t.Fatalf("expected a non-empty projected result, got empty string")
	}
	if revision != nil {
		t.Fatalf("expected no revision in a dry-run response, got %v", *revision)
	}

	after, err := show.CurrentRevision(root, showPath)
	if err != nil {
		t.Fatalf("CurrentRevision after: %v", err)
	}
	if after != before {
		t.Fatalf("expected the real revision to stay %d after a dry-run, got %d", before, after)
	}

	state, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load: %v", err)
	}
	if len(state.Pools) != 0 {
		t.Fatalf("expected the real show to have no pools after a dry-run, got %d", len(state.Pools))
	}
}

// --- TestDryRunSurfacesValidationErrorWithoutMutating ----------------------

// TestDryRunSurfacesValidationErrorWithoutMutating proves a dry-run of an
// invalid mutation (a duplicate pool name) returns the same validation
// error the real mutation would, without mutating the real show.
func TestDryRunSurfacesValidationErrorWithoutMutating(t *testing.T) {
	root := t.TempDir()
	showPath := filepath.Join(root, "show.golc")
	catalog, err := routecatalog.New()
	if err != nil {
		t.Fatalf("routecatalog.New: %v", err)
	}
	server := api.NewServer(catalog, root, showPath)
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	// Create a real pool named "Dup" first.
	created := doCreatePoolRequest(t, server.Handler(), token, "", "Dup")
	if created.Code < 200 || created.Code >= 300 {
		t.Fatalf("expected the setup create to succeed, got %d (body: %s)", created.Code, created.Body.String())
	}
	afterSetup, err := show.CurrentRevision(root, showPath)
	if err != nil {
		t.Fatalf("CurrentRevision after setup: %v", err)
	}

	// A dry-run attempting to create another pool named "Dup" must fail
	// with the same duplicate-name validation error the real mutation
	// would produce.
	rec := doDryRunCreatePoolRequest(t, server.Handler(), token, "Dup")
	if rec.Code < 400 {
		t.Fatalf("expected the dry-run of a duplicate-name create to fail, got %d (body: %s)", rec.Code, rec.Body.String())
	}

	after, err := show.CurrentRevision(root, showPath)
	if err != nil {
		t.Fatalf("CurrentRevision after dry-run failure: %v", err)
	}
	if after != afterSetup {
		t.Fatalf("expected the real revision to remain %d after a failed dry-run, got %d", afterSetup, after)
	}

	state, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load: %v", err)
	}
	if len(state.Pools) != 1 {
		t.Fatalf("expected exactly the one real \"Dup\" pool from setup, got %d", len(state.Pools))
	}
}

// --- TestDryRunLeavesNoTempCopy ---------------------------------------------

// TestDryRunLeavesNoTempCopy proves the throwaway VACUUM INTO copy
// dryRunMutate creates is deleted before the request returns, for both a
// successful and a failing dry-run.
func TestDryRunLeavesNoTempCopy(t *testing.T) {
	root := t.TempDir()
	showPath := filepath.Join(root, "show.golc")
	catalog, err := routecatalog.New()
	if err != nil {
		t.Fatalf("routecatalog.New: %v", err)
	}
	server := api.NewServer(catalog, root, showPath)
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	doDryRunCreatePoolRequest(t, server.Handler(), token, "Leftover")
	assertNoBackupFiles(t, showPath)

	created := doCreatePoolRequest(t, server.Handler(), token, "", "Existing")
	if created.Code < 200 || created.Code >= 300 {
		t.Fatalf("expected the setup create to succeed, got %d (body: %s)", created.Code, created.Body.String())
	}
	doDryRunCreatePoolRequest(t, server.Handler(), token, "Existing") // duplicate -> dry-run failure path
	assertNoBackupFiles(t, showPath)
}

// assertNoBackupFiles fails t if any "<showPath>.backup-*" temp copy
// exists (internal/show.NewTempCopy's naming convention, verifiedBackup's
// own doc comment).
func assertNoBackupFiles(t *testing.T, showPath string) {
	t.Helper()
	matches, err := filepath.Glob(showPath + ".backup-*")
	if err != nil {
		t.Fatalf("filepath.Glob: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("expected no leftover temp copy files, found: %v", matches)
	}
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
	if err != nil {
		t.Fatalf("routecatalog.New: %v", err)
	}
	server := api.NewServer(catalog, root, showPath)
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	var events []api.MutationEvent
	api.RegisterMutationObserver(func(ev api.MutationEvent) {
		events = append(events, ev)
	})

	rec := doDryRunCreatePoolRequest(t, server.Handler(), token, "Observed")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body.String())
	}

	if len(events) != 1 {
		t.Fatalf("expected exactly 1 observer event, got %d: %+v", len(events), events)
	}
	if events[0].Outcome != "dry_run" {
		t.Fatalf("expected outcome \"dry_run\", got %q", events[0].Outcome)
	}
	if events[0].ResultingRevision != nil {
		t.Fatalf("expected no resulting revision on a dry-run event, got %v", *events[0].ResultingRevision)
	}
}
