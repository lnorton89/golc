// keys_test.go pins keys.go's POST /v1/keys boundary validation (07-14-
// PLAN.md Task 2): an admin-scoped mint request with an expires_in beyond
// maxAPIKeyLifetime is rejected 400 GOLC_API_KEY_LIFETIME_TOO_LONG and
// mints nothing; a request within the bound still mints successfully; a
// scopes element containing a comma is rejected 400
// GOLC_API_LIST_VALUE_INVALID and mints nothing; and an unparseable
// expires_in still reaches "api-key create"'s own existing downstream
// duration diagnostic unchanged (the boundary check adds a ceiling, it
// does not take over duration parsing).
//
// This file lives in the external api_test package (see coverage_test.go's
// doc comment for why) so it can reach a real, live command registry
// through internal/routecatalog's test-only bridge for a genuine
// "api-key create" -> show.InsertAPIKey round trip, not a canned stub
// outcome, proving "mints nothing" against show.ListAPIKeys rather than
// assuming it. It reuses jsonBody/seedKey from mutate_test.go (same
// package).
package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/api"
	"github.com/lnorton89/golc/internal/routecatalog"
	"github.com/lnorton89/golc/internal/show"
)

// newKeysTestServer builds a fresh *api.Server against its own
// t.TempDir() root, mirroring newAuditedBatchServer's (batch_test.go) own
// per-test-root construction so this file's api_keys rows never leak
// across tests.
func newKeysTestServer(t *testing.T) (server *api.Server, root, showPath string) {
	t.Helper()
	root = t.TempDir()
	showPath = filepath.Join(root, "show.golc")
	catalog, err := routecatalog.New()
	require.NoError(t, err, "routecatalog.New")
	return api.NewServer(catalog, root, showPath), root, showPath
}

// doMintKeyRequest issues POST /v1/keys with body {"scopes": scopes,
// "expires_in": expiresIn}, presenting token as the bearer credential.
func doMintKeyRequest(t *testing.T, handler http.Handler, token string, scopes []string, expiresIn string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/keys", jsonBody(t, map[string]any{
		"scopes":     scopes,
		"expires_in": expiresIn,
	}))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// decodeProblemDetail decodes rec's body as Huma's RFC 9457
// application/problem+json shape, returning its "detail" field -- the
// field carrying the diagnostic code every rejection test greps for.
func decodeProblemDetail(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var decoded struct {
		Detail string `json:"detail"`
	}
	err := json.Unmarshal(rec.Body.Bytes(), &decoded)
	require.NoError(t, err, "decode problem detail %q", rec.Body.String())
	return decoded.Detail
}

// requireAPIKeyCount fails t unless show.ListAPIKeys(root, showPath)
// returns exactly want keys -- proving a rejected mint attempt minted
// nothing, not merely assuming it from the HTTP status alone.
func requireAPIKeyCount(t *testing.T, root, showPath string, want int) {
	t.Helper()
	keys, err := show.ListAPIKeys(root, showPath)
	require.NoError(t, err, "ListAPIKeys")
	require.Len(t, keys, want, "%+v", keys)
}

// TestMintKeyRejectsLifetimeBeyondBound proves an expires_in beyond
// maxAPIKeyLifetime returns 400 GOLC_API_KEY_LIFETIME_TOO_LONG naming the
// bound and mints nothing (closes 07-REVIEW.md IN-01).
func TestMintKeyRejectsLifetimeBeyondBound(t *testing.T) {
	server, root, showPath := newKeysTestServer(t)
	handler := server.Handler()
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAdmin})
	requireAPIKeyCount(t, root, showPath, 1) // the seeded admin key itself

	rec := doMintKeyRequest(t, handler, token, []string{"authoring"}, "8761h")
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	detail := decodeProblemDetail(t, rec)
	require.True(t, containsDiagnostic(detail, "GOLC_API_KEY_LIFETIME_TOO_LONG"), "expected GOLC_API_KEY_LIFETIME_TOO_LONG diagnostic, got %q", detail)
	requireAPIKeyCount(t, root, showPath, 1) // unchanged: nothing minted
}

// TestMintKeyAcceptsLifetimeWithinBound proves an expires_in inside
// maxAPIKeyLifetime still mints successfully and returns its raw token
// exactly once, unchanged behavior from before this plan.
func TestMintKeyAcceptsLifetimeWithinBound(t *testing.T) {
	server, root, showPath := newKeysTestServer(t)
	handler := server.Handler()
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAdmin})
	requireAPIKeyCount(t, root, showPath, 1)

	rec := doMintKeyRequest(t, handler, token, []string{"authoring"}, "720h")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.True(t, containsDiagnostic(rec.Body.String(), "raw_token"), "expected the mint response to carry a raw_token field, got %q", rec.Body.String())
	requireAPIKeyCount(t, root, showPath, 2) // the seeded key plus the new mint
}

// TestMintKeyRejectsCommaInScopes proves a scopes element containing a
// comma returns 400 GOLC_API_LIST_VALUE_INVALID and mints nothing (closes
// 07-REVIEW.md IN-02 for keys.go).
func TestMintKeyRejectsCommaInScopes(t *testing.T) {
	server, root, showPath := newKeysTestServer(t)
	handler := server.Handler()
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAdmin})
	requireAPIKeyCount(t, root, showPath, 1)

	rec := doMintKeyRequest(t, handler, token, []string{"authoring,admin"}, "720h")
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	detail := decodeProblemDetail(t, rec)
	require.True(t, containsDiagnostic(detail, "GOLC_API_LIST_VALUE_INVALID"), "expected GOLC_API_LIST_VALUE_INVALID diagnostic, got %q", detail)
	requireAPIKeyCount(t, root, showPath, 1)
}

// TestMintKeyMalformedLifetimeUsesDownstreamDiagnostic proves an
// unparseable expires_in is forwarded unchanged to "api-key create"'s own
// existing time.ParseDuration diagnostic (internal/command/apikey.go
// runAPIKeyCreate's GOLC_APIKEY_USAGE) rather than being intercepted by
// the new boundary check -- the established diagnostic for a malformed
// duration remains the single authority, and no existing behavior
// regresses.
func TestMintKeyMalformedLifetimeUsesDownstreamDiagnostic(t *testing.T) {
	server, root, showPath := newKeysTestServer(t)
	handler := server.Handler()
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAdmin})
	requireAPIKeyCount(t, root, showPath, 1)

	rec := doMintKeyRequest(t, handler, token, []string{"authoring"}, "not-a-duration")
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	detail := decodeProblemDetail(t, rec)
	require.True(t, containsDiagnostic(detail, "GOLC_APIKEY_USAGE"), "expected the existing downstream GOLC_APIKEY_USAGE diagnostic, got %q", detail)
	require.False(t, containsDiagnostic(detail, "GOLC_API_KEY_LIFETIME_TOO_LONG"), "expected the boundary check NOT to intercept a malformed duration, got %q", detail)
	requireAPIKeyCount(t, root, showPath, 1)
}

// containsDiagnostic reports whether haystack contains needle -- shared
// across this file's rejection assertions.
func containsDiagnostic(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}
