// auth.go implements API-key authentication and coarse domain scope
// checking for every /v1 request (07-04-PLAN.md Task 2, CONTEXT D-05/
// D-08). AuthMiddleware is wired onto every request through router.go's
// buildRouter, before any operation handler runs: a missing, malformed,
// unknown, expired, or revoked bearer token is rejected 401 with an
// identical diagnostic regardless of cause (T-07-05 -- a 401 must never
// leak whether a presented prefix existed), and a valid key's id and
// coarse domain scopes (D-08) are attached to the request context for
// downstream handlers (RequireScope, keys.go's own admin-scope gate) and
// ratelimit.go's per-key bucket lookup. This file imports internal/show
// directly (not through the Executor seam): api_keys authentication is
// infrastructure this package itself owns, not a translated domain
// command -- the CLI's "api-key create"/"api-key list"/"api-key revoke"
// routes (internal/command/apikey.go) and this package's own key lookup
// both call into the exact same internal/show functions, giving them one
// storage authority over the api_keys table without requiring this
// package to import the CLI command-execution package (still never
// imported anywhere under internal/api/).
package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/lnorton89/golc/internal/show"
)

// authFailureMessage is returned for every authentication failure --
// missing header, malformed header, unknown prefix, hash mismatch,
// expired key, or revoked key -- so a 401 response never distinguishes
// which of these actually occurred (T-07-05).
const authFailureMessage = "GOLC_API_UNAUTHORIZED: a valid, non-expired, non-revoked API key is required"

// apiKeyContextKey is the private context key type authMiddleware
// attaches the authenticated key's id/scopes under, and
// ScopesFromContext/KeyIDFromContext/HasScope read back.
type apiKeyContextKey struct{}

// apiKeyContext is the authenticated request's identity: the key's id
// (for rate limiting and revocation, never the raw token or hash) and
// its parsed coarse domain scopes (D-08).
type apiKeyContext struct {
	KeyID  string
	Scopes []show.APIKeyScope
}

// KeyIDFromContext returns the authenticated key's id attached by
// AuthMiddleware, or ("", false) if ctx carries none -- ratelimit.go's
// per-key bucket lookup key.
func KeyIDFromContext(ctx context.Context) (string, bool) {
	value, ok := ctx.Value(apiKeyContextKey{}).(apiKeyContext)
	if !ok {
		return "", false
	}
	return value.KeyID, true
}

// ScopesFromContext returns the authenticated key's coarse domain scopes
// attached by AuthMiddleware, or (nil, false) if ctx carries none (never
// reachable for a request that passed AuthMiddleware, since auth applies
// to every /v1 request) -- exported for later mutation-gating plans
// (07-05) to read directly.
func ScopesFromContext(ctx context.Context) ([]show.APIKeyScope, bool) {
	value, ok := ctx.Value(apiKeyContextKey{}).(apiKeyContext)
	if !ok {
		return nil, false
	}
	return value.Scopes, true
}

// HasScope reports whether ctx's authenticated key carries scope.
func HasScope(ctx context.Context, scope show.APIKeyScope) bool {
	scopes, ok := ScopesFromContext(ctx)
	if !ok {
		return false
	}
	for _, s := range scopes {
		if s == scope {
			return true
		}
	}
	return false
}

// RequireScope returns a typed Huma 403 error unless ctx's authenticated
// key carries scope (D-08: mutation/admin routes gate by coarse domain
// scope; reads are never scope-gated). This never returns a 401 --
// AuthMiddleware has already run and guaranteed a valid, scoped key by
// the time any operation handler (and therefore this check) executes.
func RequireScope(ctx context.Context, scope show.APIKeyScope) error {
	if HasScope(ctx, scope) {
		return nil
	}
	return huma.Error403Forbidden(
		"GOLC_API_SCOPE_REQUIRED: this operation requires the \"" + string(scope) + "\" scope")
}

// bearerToken extracts the raw token from an "Authorization: Bearer
// <token>" header value. A missing "Bearer " prefix or an empty token
// after it is reported as not-ok, exactly like a missing header --
// AuthMiddleware treats both identically.
func bearerToken(header string) (string, bool) {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", false
	}
	token := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	if token == "" {
		return "", false
	}
	return token, true
}

// AuthMiddleware returns the Huma middleware every /v1 request passes
// through before any operation handler runs (wired globally in
// router.go's buildRouter, ahead of every operation's own Register call,
// so ordering never depends on which file a later operation happens to
// be declared in). It reads "Authorization: Bearer <token>", computes the
// token's lookup prefix (show.APIKeyPrefixFromToken), looks up the stored
// row (show.LookupAPIKeyByPrefix against server's own root/showPath --
// the same api_keys table the CLI's "api-key create"/"api-key list"/
// "api-key revoke" routes read and write), constant-time-compares the
// presented token's hash (show.CompareAPIKeyHash) against the stored
// hash, and checks the row's validity (show.IsAPIKeyValid: not revoked,
// not expired). Any failure along this path returns the exact same 401
// diagnostic (authFailureMessage) -- an unknown prefix, a wrong token for
// a known prefix, and a known-but-expired-or-revoked key are all
// indistinguishable to the caller (T-07-05). On success, the key's id and
// scopes are attached to the request context (huma.WithValue) before
// calling next.
func AuthMiddleware(humaAPI huma.API, server *Server) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		token, ok := bearerToken(ctx.Header("Authorization"))
		if !ok {
			huma.WriteErr(humaAPI, ctx, http.StatusUnauthorized, authFailureMessage)
			return
		}

		prefix := show.APIKeyPrefixFromToken(token)
		record, found, err := show.LookupAPIKeyByPrefix(server.root, server.showPath, prefix)
		if err != nil || !found {
			huma.WriteErr(humaAPI, ctx, http.StatusUnauthorized, authFailureMessage)
			return
		}
		if !show.CompareAPIKeyHash(token, record.Hash) {
			huma.WriteErr(humaAPI, ctx, http.StatusUnauthorized, authFailureMessage)
			return
		}
		if !show.IsAPIKeyValid(record, time.Now()) {
			huma.WriteErr(humaAPI, ctx, http.StatusUnauthorized, authFailureMessage)
			return
		}

		next(huma.WithValue(ctx, apiKeyContextKey{}, apiKeyContext{KeyID: record.KeyID, Scopes: record.Scopes}))
	}
}
