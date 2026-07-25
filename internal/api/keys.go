// keys.go implements the /v1/keys REST operations (07-04-PLAN.md Task 2,
// CONTEXT D-05/D-08): POST /v1/keys mints a new scoped, expiring API key
// (admin scope required), GET /v1/keys lists every key's metadata, and
// DELETE /v1/keys/{id} revokes one. Each operation translates into the
// exact same "api-key create"/"api-key list"/"api-key revoke" routes
// internal/command/apikey.go self-registers for the CLI (via server's
// Executor seam, translate.go's established Pattern 1) -- REST and CLI
// share one execution/storage authority over the api_keys table (D-01);
// this file never calls internal/show directly for key CRUD. All three
// operations require the admin scope (RequireScope, auth.go): key
// management is itself security-sensitive administrative surface, not
// merely a mutation of ordinary show content.
package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/lnorton89/golc/internal/show"
)

// maxAPIKeyLifetime is the [ASSUMED] (07-14-PLAN.md, UAT-tunable, matching
// this package's existing convention for EventRingBufferCapacity and
// idempotency.go's defaultIdempotencyTTL) documented ceiling on a minted
// API key's requested lifetime: 365 days. A credential that outlives a
// full release year cannot be reasoned about against the 180-day
// deprecation window docs/api/COMPATIBILITY.md commits to, and re-minting
// is a single admin-scoped call -- so POST /v1/keys refuses to mint a key
// requesting a longer lifetime rather than silently minting an effectively
// immortal one (closes 07-REVIEW.md IN-01).
const maxAPIKeyLifetime = 8760 * time.Hour

// --- POST /v1/keys -> "api-key create" (admin scope required) ----------

// mintAPIKeyInput is POST /v1/keys's Huma input.
type mintAPIKeyInput struct {
	Body struct {
		Scopes    []string `json:"scopes" required:"true" doc:"Coarse domain scopes: playback, authoring, admin. A scope value must not contain a comma (it is forwarded as a comma-delimited list)."`
		ExpiresIn string   `json:"expires_in" required:"true" doc:"Go duration string the key remains valid for, e.g. \"720h\". Maximum accepted duration is 8760h (365 days)."`
	}
}

// registerMintAPIKey wires POST /v1/keys onto humaAPI, translating it
// into an "api-key create --scope <...> --expires <...> --show <daemon's
// own fixed show path> --json" invocation. The raw token is present in
// the response exactly once, in this mint response, mirroring "api-key
// create"'s own single-print guarantee.
func registerMintAPIKey(humaAPI huma.API, server *Server) {
	huma.Register(humaAPI, huma.Operation{
		OperationID: "mint-api-key",
		Method:      http.MethodPost,
		Path:        apiPathPrefix + "/keys",
		Summary:     "Mint a new scoped, expiring API key (admin scope required, D-08). The raw token is returned exactly once.",
	}, func(ctx context.Context, input *mintAPIKeyInput) (*rawJSONOutput, error) {
		if err := RequireScope(ctx, show.APIKeyScopeAdmin); err != nil {
			return nil, err
		}

		if err := validateListValues("scopes", input.Body.Scopes); err != nil {
			return nil, err
		}
		// Only reject at the HTTP boundary when ExpiresIn parses AND exceeds
		// maxAPIKeyLifetime. A malformed duration is deliberately NOT rejected
		// here -- it is forwarded unchanged to "api-key create"'s own
		// time.ParseDuration check (internal/command/apikey.go runAPIKeyCreate),
		// which remains the single authority for the "not a valid duration"
		// diagnostic. This boundary check only adds a ceiling on top of an
		// already-valid duration; it does not take over duration parsing.
		if requested, parseErr := time.ParseDuration(input.Body.ExpiresIn); parseErr == nil && requested > maxAPIKeyLifetime {
			return nil, huma.Error400BadRequest(fmt.Sprintf(
				"GOLC_API_KEY_LIFETIME_TOO_LONG: requested expires_in %q exceeds the maximum accepted duration %q",
				input.Body.ExpiresIn, maxAPIKeyLifetime))
		}

		args := []string{
			"--scope", strings.Join(input.Body.Scopes, ","),
			"--expires", input.Body.ExpiresIn,
			"--show", server.showPath,
			"--json",
		}
		exitCode, stdout, stderr := server.executor.Execute("api-key create", args, server.root)
		body, err := translateResult(exitCode, stdout, stderr)
		if err != nil {
			return nil, err
		}
		return newRawJSONOutput(body), nil
	})
}

var _ = RegisterOperation(OperationRegistration{Route: "api-key create", Register: registerMintAPIKey})

// --- GET /v1/keys -> "api-key list" (admin scope required) --------------

// registerListAPIKeys wires GET /v1/keys onto humaAPI, translating it
// into an "api-key list --show <daemon's own fixed show path> --json"
// invocation -- metadata only, never a hash or raw token (show.ListAPIKeys'
// own hash-free contract).
func registerListAPIKeys(humaAPI huma.API, server *Server) {
	huma.Register(humaAPI, huma.Operation{
		OperationID: "list-api-keys",
		Method:      http.MethodGet,
		Path:        apiPathPrefix + "/keys",
		Summary:     "List every API key's metadata (admin scope required, D-08) -- never the hash or raw token.",
	}, func(ctx context.Context, input *struct{}) (*rawJSONOutput, error) {
		if err := RequireScope(ctx, show.APIKeyScopeAdmin); err != nil {
			return nil, err
		}

		args := []string{"--show", server.showPath, "--json"}
		exitCode, stdout, stderr := server.executor.Execute("api-key list", args, server.root)
		body, err := translateResult(exitCode, stdout, stderr)
		if err != nil {
			return nil, err
		}
		return newRawJSONOutput(body), nil
	})
}

var _ = RegisterOperation(OperationRegistration{Route: "api-key list", Register: registerListAPIKeys})

// --- DELETE /v1/keys/{id} -> "api-key revoke" (admin scope required) ----

// revokeAPIKeyInput is DELETE /v1/keys/{id}'s Huma input.
type revokeAPIKeyInput struct {
	ID string `path:"id" doc:"The api key id to revoke."`
}

// registerRevokeAPIKey wires DELETE /v1/keys/{id} onto humaAPI,
// translating it into an "api-key revoke --id <id> --show <daemon's own
// fixed show path> --json" invocation.
func registerRevokeAPIKey(humaAPI huma.API, server *Server) {
	huma.Register(humaAPI, huma.Operation{
		OperationID: "revoke-api-key",
		Method:      http.MethodDelete,
		Path:        apiPathPrefix + "/keys/{id}",
		Summary:     "Revoke one API key by id (admin scope required, D-08), immediately invalidating it for authentication.",
	}, func(ctx context.Context, input *revokeAPIKeyInput) (*rawJSONOutput, error) {
		if err := RequireScope(ctx, show.APIKeyScopeAdmin); err != nil {
			return nil, err
		}

		args := []string{"--id", input.ID, "--show", server.showPath, "--json"}
		exitCode, stdout, stderr := server.executor.Execute("api-key revoke", args, server.root)
		body, err := translateResult(exitCode, stdout, stderr)
		if err != nil {
			return nil, err
		}
		return newRawJSONOutput(body), nil
	})
}

var _ = RegisterOperation(OperationRegistration{Route: "api-key revoke", Register: registerRevokeAPIKey})
