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
	"strings"
	"testing"
	"time"

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
	if err != nil {
		t.Fatalf("GenerateAPIKey: %v", err)
	}
	if _, err := show.InsertAPIKey(root, showPath, generated, scopes, time.Now().UTC().Add(ttl)); err != nil {
		t.Fatalf("InsertAPIKey: %v", err)
	}
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
	if err != nil {
		t.Fatalf("json.Marshal request body: %v", err)
	}
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
	if noHeader.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with no Authorization header, got %d (body: %s)", noHeader.Code, noHeader.Body.String())
	}

	unknown := doAuthedRequest(t, server.Handler(), http.MethodGet, "/v1/show", "totally-unknown-token-value")
	if unknown.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for an unknown token, got %d (body: %s)", unknown.Code, unknown.Body.String())
	}

	expiredToken := seedAPIKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopePlayback}, -time.Hour)
	expired := doAuthedRequest(t, server.Handler(), http.MethodGet, "/v1/show", expiredToken)
	if expired.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for an expired key, got %d (body: %s)", expired.Code, expired.Body.String())
	}

	generated, err := show.GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey: %v", err)
	}
	key, err := show.InsertAPIKey(root, showPath, generated, []show.APIKeyScope{show.APIKeyScopePlayback}, time.Now().UTC().Add(time.Hour))
	if err != nil {
		t.Fatalf("InsertAPIKey: %v", err)
	}
	if err := show.RevokeAPIKey(root, showPath, key.KeyID); err != nil {
		t.Fatalf("RevokeAPIKey: %v", err)
	}
	revoked := doAuthedRequest(t, server.Handler(), http.MethodGet, "/v1/show", generated.RawToken)
	if revoked.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for a revoked key, got %d (body: %s)", revoked.Code, revoked.Body.String())
	}
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
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for a valid key, got %d (body: %s)", rec.Code, rec.Body.String())
	}
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
	if first.Code != http.StatusOK {
		t.Fatalf("expected the first request from key A to succeed, got %d (body: %s)", first.Code, first.Body.String())
	}
	second := doAuthedRequest(t, server.Handler(), http.MethodGet, "/v1/show", tokenA)
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("expected the second request from key A (over burst=1) to be rate limited, got %d (body: %s)", second.Code, second.Body.String())
	}

	other := doAuthedRequest(t, server.Handler(), http.MethodGet, "/v1/show", tokenB)
	if other.Code != http.StatusOK {
		t.Fatalf("expected key B's independent bucket to allow its first request, got %d (body: %s)", other.Code, other.Body.String())
	}
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
	if err != nil {
		t.Fatalf("routecatalog.New: %v", err)
	}
	server := api.NewServer(catalog, root, showPath)

	adminToken := seedAPIKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAdmin}, time.Hour)
	playbackToken := seedAPIKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopePlayback}, time.Hour)

	mintBody := map[string]any{"scopes": []string{"authoring"}, "expires_in": "1h"}

	forbidden := doAuthedJSONRequest(t, server.Handler(), http.MethodPost, "/v1/keys", playbackToken, mintBody)
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("expected 403 minting with a non-admin key, got %d (body: %s)", forbidden.Code, forbidden.Body.String())
	}

	minted := doAuthedJSONRequest(t, server.Handler(), http.MethodPost, "/v1/keys", adminToken, mintBody)
	if minted.Code < 200 || minted.Code >= 300 {
		t.Fatalf("expected a 2xx minting with an admin key, got %d (body: %s)", minted.Code, minted.Body.String())
	}
	var mintedView struct {
		ID       string `json:"id"`
		RawToken string `json:"raw_token"`
	}
	if err := json.Unmarshal(minted.Body.Bytes(), &mintedView); err != nil {
		t.Fatalf("unmarshal mint response: %v", err)
	}
	if mintedView.ID == "" || mintedView.RawToken == "" {
		t.Fatalf("expected a non-empty id and raw_token in the mint response, got: %s", minted.Body.String())
	}

	listed := doAuthedRequest(t, server.Handler(), http.MethodGet, "/v1/keys", adminToken)
	if listed.Code != http.StatusOK {
		t.Fatalf("expected 200 listing keys, got %d (body: %s)", listed.Code, listed.Body.String())
	}
	if strings.Contains(listed.Body.String(), mintedView.RawToken) {
		t.Fatalf("expected the list response to never contain a raw token, got: %s", listed.Body.String())
	}

	revoked := doAuthedRequest(t, server.Handler(), http.MethodDelete, "/v1/keys/"+mintedView.ID, adminToken)
	if revoked.Code != http.StatusOK {
		t.Fatalf("expected 200 revoking a key, got %d (body: %s)", revoked.Code, revoked.Body.String())
	}

	keys, err := show.ListAPIKeys(root, showPath)
	if err != nil {
		t.Fatalf("ListAPIKeys: %v", err)
	}
	var foundMinted bool
	for _, key := range keys {
		if key.KeyID == mintedView.ID {
			foundMinted = true
			if key.RevokedAt.IsZero() {
				t.Fatalf("expected the minted key %q to be revoked after DELETE /v1/keys/%s", key.KeyID, mintedView.ID)
			}
		}
	}
	if !foundMinted {
		t.Fatalf("expected to find the minted key %q via ListAPIKeys", mintedView.ID)
	}
}
