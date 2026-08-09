// auth_test.go pins the API-key authentication, scope-gating, per-key
// rate-limit, and /v1/keys REST contract (07-04-PLAN.md Task 2, RED
// state) before internal/api/auth.go, internal/api/ratelimit.go, and
// internal/api/keys.go exist: a missing/unknown/expired/revoked bearer
// token is rejected 401 on every /v1 request; a valid key proceeds; a
// key over its per-key burst is rejected 429 while an independent key is
// unaffected; and POST/GET/DELETE /v1/keys mint/list/revoke through the
// same internal/show store the CLI uses, with minting gated behind the
// admin scope. It lives in the external api_test package (see
// coverage_test.go's doc comment) so the /v1/keys tests can reach a real
// command registry through internal/routecatalog's test-only bridge.
package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/api"
	"github.com/lnorton89/golc/internal/routecatalog"
	"github.com/lnorton89/golc/internal/show"
)

// seedAPIKey generates and inserts a real api_keys row directly through
// internal/show (bypassing any HTTP/CLI layer, since these tests need a
// key that already exists before the request under test is issued),
// returning the raw token a caller would present as a bearer credential.
func seedAPIKey(t *testing.T, root, showPath string, scopes []show.APIKeyScope, ttl time.Duration) string {
	t.Helper()
	generated, err := show.GenerateAPIKey()
	require.NoError(t, err, "GenerateAPIKey")
	_, err = show.InsertAPIKey(root, showPath, generated, scopes, time.Now().UTC().Add(ttl))
	require.NoError(t, err, "InsertAPIKey")
	return generated.RawToken
}

// doAuthedRequest issues method/target in-process with an optional bearer
// token (empty means no Authorization header at all).
func doAuthedRequest(t *testing.T, handler http.Handler, method, target, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// doAuthedJSONRequest is doAuthedRequest with a JSON request body.
func doAuthedJSONRequest(t *testing.T, handler http.Handler, method, target, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(body)
	require.NoError(t, err, "json.Marshal request body")
	req := httptest.NewRequest(method, target, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// --- TestAuth --------------------------------------------------------

// TestAuthRejectsMissingUnknownExpiredAndRevokedKeys proves every
// non-valid-key case is rejected 401, with no observable difference
// between them (T-07-05: a 401 must never reveal whether a presented
// prefix existed).
func TestAuthRejectsMissingUnknownExpiredAndRevokedKeys(t *testing.T) {
	root := t.TempDir()
	showPath := "show.golc"
	stub := &stubExecutor{exitCode: 0, stdout: []byte(`{"ok":true}`)}
	server := api.NewServer(stub, root, showPath)

	noHeader := doAuthedRequest(t, server.Handler(), http.MethodGet, "/v1/show", "")
	require.Equal(t, http.StatusUnauthorized, noHeader.Code, "expected 401 with no Authorization header (body: %s)", noHeader.Body.String())

	unknown := doAuthedRequest(t, server.Handler(), http.MethodGet, "/v1/show", "totally-unknown-token-value")
	require.Equal(t, http.StatusUnauthorized, unknown.Code, "expected 401 for an unknown token (body: %s)", unknown.Body.String())

	expiredToken := seedAPIKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopePlayback}, -time.Hour)
	expired := doAuthedRequest(t, server.Handler(), http.MethodGet, "/v1/show", expiredToken)
	require.Equal(t, http.StatusUnauthorized, expired.Code, "expected 401 for an expired key (body: %s)", expired.Body.String())

	generated, err := show.GenerateAPIKey()
	require.NoError(t, err, "GenerateAPIKey")
	key, err := show.InsertAPIKey(root, showPath, generated, []show.APIKeyScope{show.APIKeyScopePlayback}, time.Now().UTC().Add(time.Hour))
	require.NoError(t, err, "InsertAPIKey")
	err = show.RevokeAPIKey(root, showPath, key.KeyID)
	require.NoError(t, err, "RevokeAPIKey")
	revoked := doAuthedRequest(t, server.Handler(), http.MethodGet, "/v1/show", generated.RawToken)
	require.Equal(t, http.StatusUnauthorized, revoked.Code, "expected 401 for a revoked key (body: %s)", revoked.Body.String())
}

// TestAuthValidKeyProceeds proves a valid, non-expired, non-revoked key
// is allowed through to the underlying operation.
func TestAuthValidKeyProceeds(t *testing.T) {
	root := t.TempDir()
	showPath := "show.golc"
	stub := &stubExecutor{exitCode: 0, stdout: []byte(`{"ok":true}`)}
	server := api.NewServer(stub, root, showPath)

	token := seedAPIKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopePlayback}, time.Hour)
	rec := doAuthedRequest(t, server.Handler(), http.MethodGet, "/v1/show", token)
	require.Equal(t, http.StatusOK, rec.Code, "expected 200 for a valid key (body: %s)", rec.Body.String())
}

// --- TestRateLimit -----------------------------------------------------

// TestRateLimitPerKeyIndependent proves a key exceeding its own burst is
// rejected 429 while a second, independent key's own bucket is
// unaffected (the concurrency edge case).
func TestRateLimitPerKeyIndependent(t *testing.T) {
	root := t.TempDir()
	showPath := "show.golc"
	stub := &stubExecutor{exitCode: 0, stdout: []byte(`{"ok":true}`)}
	server := api.NewServer(stub, root, showPath, api.WithConfig(api.Config{RatePerMinute: 60, RateBurst: 1}))

	tokenA := seedAPIKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopePlayback}, time.Hour)
	tokenB := seedAPIKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopePlayback}, time.Hour)

	first := doAuthedRequest(t, server.Handler(), http.MethodGet, "/v1/show", tokenA)
	require.Equal(t, http.StatusOK, first.Code, "expected the first request from key A to succeed (body: %s)", first.Body.String())
	second := doAuthedRequest(t, server.Handler(), http.MethodGet, "/v1/show", tokenA)
	require.Equal(t, http.StatusTooManyRequests, second.Code, "expected the second request from key A (over burst=1) to be rate limited (body: %s)", second.Body.String())
	require.Equal(t, "1", second.Header().Get("Retry-After"), "expected a Retry-After hint sized from the configured 60/min rate (1s per token)")

	other := doAuthedRequest(t, server.Handler(), http.MethodGet, "/v1/show", tokenB)
	require.Equal(t, http.StatusOK, other.Code, "expected key B's independent bucket to allow its first request (body: %s)", other.Body.String())
}

// --- TestKeysREST --------------------------------------------------------

// TestKeysRESTMintRequiresAdminListsAndRevokes proves POST /v1/keys
// requires the admin scope (a non-admin key gets 403), GET /v1/keys lists
// metadata only (never a raw token or hash), and DELETE /v1/keys/{id}
// revokes -- all through the same internal/show store the CLI's
// "api-key create"/"api-key list"/"api-key revoke" routes use (single
// execution/storage authority, D-01).
func TestKeysRESTMintRequiresAdminListsAndRevokes(t *testing.T) {
	root := t.TempDir()
	showPath := "show.golc"
	catalog, err := routecatalog.New()
	require.NoError(t, err, "routecatalog.New")
	server := api.NewServer(catalog, root, showPath)

	adminToken := seedAPIKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAdmin}, time.Hour)
	playbackToken := seedAPIKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopePlayback}, time.Hour)

	mintBody := map[string]any{"scopes": []string{"authoring"}, "expires_in": "1h"}

	forbidden := doAuthedJSONRequest(t, server.Handler(), http.MethodPost, "/v1/keys", playbackToken, mintBody)
	require.Equal(t, http.StatusForbidden, forbidden.Code, "expected 403 minting with a non-admin key (body: %s)", forbidden.Body.String())

	minted := doAuthedJSONRequest(t, server.Handler(), http.MethodPost, "/v1/keys", adminToken, mintBody)
	require.True(t, minted.Code >= 200 && minted.Code < 300, "expected a 2xx minting with an admin key, got %d (body: %s)", minted.Code, minted.Body.String())
	var mintedView struct {
		ID       string `json:"id"`
		RawToken string `json:"raw_token"`
	}
	err = json.Unmarshal(minted.Body.Bytes(), &mintedView)
	require.NoError(t, err, "unmarshal mint response")
	require.NotEmpty(t, mintedView.ID, "expected a non-empty id in the mint response, got: %s", minted.Body.String())
	require.NotEmpty(t, mintedView.RawToken, "expected a non-empty raw_token in the mint response, got: %s", minted.Body.String())

	listed := doAuthedRequest(t, server.Handler(), http.MethodGet, "/v1/keys", adminToken)
	require.Equal(t, http.StatusOK, listed.Code, "expected 200 listing keys (body: %s)", listed.Body.String())
	require.NotContains(t, listed.Body.String(), mintedView.RawToken, "expected the list response to never contain a raw token")

	revoked := doAuthedRequest(t, server.Handler(), http.MethodDelete, "/v1/keys/"+mintedView.ID, adminToken)
	require.Equal(t, http.StatusOK, revoked.Code, "expected 200 revoking a key (body: %s)", revoked.Body.String())

	keys, err := show.ListAPIKeys(root, showPath)
	require.NoError(t, err, "ListAPIKeys")
	var foundMinted bool
	for _, key := range keys {
		if key.KeyID == mintedView.ID {
			foundMinted = true
			require.False(t, key.RevokedAt.IsZero(), "expected the minted key %q to be revoked after DELETE /v1/keys/%s", key.KeyID, mintedView.ID)
		}
	}
	require.True(t, foundMinted, "expected to find the minted key %q via ListAPIKeys", mintedView.ID)
}
