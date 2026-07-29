// apikey.go is the api-key command file (07-04-PLAN.md Task 1, CONTEXT
// D-05/D-08): it owns the "api-key" routing scope and self-registers
// "api-key create"/"api-key list"/"api-key revoke", the CLI's own key
// management routes -- the same internal/show.GenerateAPIKey/InsertAPIKey/
// ListAPIKeys/RevokeAPIKey functions the REST /v1/keys operations
// (07-04-PLAN.md Task 2) call through the Executor seam, giving both
// surfaces one execution/storage authority (D-01) over the api_keys
// table. "api-key create" is the bootstrap path for the very first key:
// it never dials a daemon and never requires an existing key, so an
// operator with local filesystem access can always mint the first
// credential the HTTP API will accept. The raw token is printed exactly
// once, by "api-key create", and is never logged or persisted beyond that
// single point (T-07-04).
package command

import (
	"fmt"
	"strings"
	"time"

	"github.com/lnorton89/golc/internal/show"
	"github.com/lnorton89/golc/internal/strictjson"
)

var _ = MustDeclareScope(ScopeRegistration{
	Scope:   "api-key",
	Summary: "Manage scoped, expiring API keys for the external control API (CONTEXT D-05/D-08).",
})

var _ = MustDeclareRoute(CommandRegistration{
	Route: "api-key create",
	Summary: "Mint a new scoped, expiring API key, printing the raw token exactly once -- store it now, it will not be shown again: " +
		"api-key create --scope <playback|authoring|admin>[,...] --expires <duration> --show <path> [--json].",
	Handler: runAPIKeyCreate,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "api-key list",
	Summary: "List every API key's metadata (id/prefix/scopes/created/expires/revoked -- never the hash or raw token): api-key list --show <path> [--json].",
	Handler: runAPIKeyList,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "api-key revoke",
	Summary: "Revoke one API key by id, immediately invalidating it for authentication: api-key revoke --id <id> --show <path> [--json].",
	Handler: runAPIKeyRevoke,
})

// apiKeyCreateView is "api-key create --json"'s canonical JSON shape --
// the one output that ever carries RawToken, since it is shown exactly
// once at mint time.
type apiKeyCreateView struct {
	ID        string   `json:"id"`
	Prefix    string   `json:"prefix"`
	Scopes    []string `json:"scopes"`
	CreatedAt string   `json:"created_at"`
	ExpiresAt string   `json:"expires_at"`
	RawToken  string   `json:"raw_token"`
}

// apiKeyView is "api-key list --json"'s per-key shape -- metadata only,
// never the hash or raw token.
type apiKeyView struct {
	ID        string   `json:"id"`
	Prefix    string   `json:"prefix"`
	Scopes    []string `json:"scopes"`
	CreatedAt string   `json:"created_at"`
	ExpiresAt string   `json:"expires_at"`
	Revoked   bool     `json:"revoked"`
	RevokedAt string   `json:"revoked_at,omitempty"`
}

// apiKeyListView wraps every listed key under a stable top-level "keys"
// field.
type apiKeyListView struct {
	Keys []apiKeyView `json:"keys"`
}

// apiKeyRevokeView is "api-key revoke --json"'s confirmation shape.
type apiKeyRevokeView struct {
	ID      string `json:"id"`
	Revoked bool   `json:"revoked"`
}

// scopeStrings renders scopes as plain strings for JSON/plain-text
// rendering.
func scopeStrings(scopes []show.APIKeyScope) []string {
	out := make([]string, len(scopes))
	for i, scope := range scopes {
		out[i] = string(scope)
	}
	return out
}

// parseAPIKeyScopeList splits raw on commas, trims whitespace, drops
// empty entries, and validates the result against the closed D-08 scope
// set (show.ValidateAPIKeyScopes) -- an empty or all-blank raw value
// yields the same "at least one scope is required" diagnostic as a
// wholly missing --scope flag.
func parseAPIKeyScopeList(raw string) ([]show.APIKeyScope, error) {
	var scopes []show.APIKeyScope
	for _, part := range strings.Split(raw, ",") {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		scopes = append(scopes, show.APIKeyScope(trimmed))
	}
	if err := show.ValidateAPIKeyScopes(scopes); err != nil {
		return nil, err
	}
	return scopes, nil
}

// toAPIKeyView projects a show.APIKey (metadata only) into its rendered
// view shape.
func toAPIKeyView(key show.APIKey) apiKeyView {
	view := apiKeyView{
		ID:        key.KeyID,
		Prefix:    key.Prefix,
		Scopes:    scopeStrings(key.Scopes),
		CreatedAt: key.CreatedAt.UTC().Format(time.RFC3339),
		ExpiresAt: key.ExpiresAt.UTC().Format(time.RFC3339),
	}
	if !key.RevokedAt.IsZero() {
		view.Revoked = true
		view.RevokedAt = key.RevokedAt.UTC().Format(time.RFC3339)
	}
	return view
}

// runAPIKeyCreate serves the self-registered "api-key create" route: it
// generates a fresh crypto/rand token (show.GenerateAPIKey), validates
// the requested scopes and expiry duration, persists only the token's
// prefix+hash (show.InsertAPIKey), and prints the raw token exactly once
// -- plain text by default, or the same information as canonical JSON
// with --json (the shape REST POST /v1/keys forwards verbatim).
func runAPIKeyCreate(request Request) Result {
	usage := "api-key create --scope <playback|authoring|admin>[,...] --expires <duration> --show <path> [--json]"
	values, err := parseArtnetArgs(usage, request.Args, map[string]bool{"json": true})
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	scopes, err := parseAPIKeyScopeList(values["scope"])
	if err != nil {
		return Result{ExitCode: 2, Stderr: fmt.Appendf(nil, "GOLC_APIKEY_USAGE: %v; usage: %s\n", err, usage)}
	}

	rawExpires, ok := values["expires"]
	if !ok || rawExpires == "" {
		return Result{ExitCode: 2, Stderr: fmt.Appendf(nil, "GOLC_APIKEY_USAGE: --expires is required; usage: %s\n", usage)}
	}
	duration, parseErr := time.ParseDuration(rawExpires)
	if parseErr != nil {
		return Result{ExitCode: 2, Stderr: fmt.Appendf(nil, "GOLC_APIKEY_USAGE: --expires value %q is not a valid duration; usage: %s\n", rawExpires, usage)}
	}
	if duration <= 0 {
		return Result{ExitCode: 2, Stderr: fmt.Appendf(nil, "GOLC_APIKEY_USAGE: --expires must be positive; usage: %s\n", usage)}
	}

	showPath, ok := values["show"]
	if !ok || showPath == "" {
		return Result{ExitCode: 2, Stderr: fmt.Appendf(nil, "GOLC_APIKEY_USAGE: --show is required; usage: %s\n", usage)}
	}

	generated, err := show.GenerateAPIKey()
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	expiresAt := time.Now().UTC().Add(duration)
	key, err := show.InsertAPIKey(request.Root, showPath, generated, scopes, expiresAt)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	if values["json"] == "true" {
		payload, encodeErr := strictjson.CanonicalEncode(apiKeyCreateView{
			ID:        key.KeyID,
			Prefix:    key.Prefix,
			Scopes:    scopeStrings(key.Scopes),
			CreatedAt: key.CreatedAt.UTC().Format(time.RFC3339),
			ExpiresAt: key.ExpiresAt.UTC().Format(time.RFC3339),
			RawToken:  generated.RawToken,
		})
		if encodeErr != nil {
			return Result{ExitCode: 1, Stderr: fmt.Appendf(nil, "GOLC_APIKEY_ENCODE_FAILED: %v\n", encodeErr)}
		}
		return Result{Stdout: payload}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "GOLC_APIKEY_CREATED: id=%s prefix=%s scopes=%s expires_at=%s\n",
		key.KeyID, key.Prefix, strings.Join(scopeStrings(key.Scopes), ","), key.ExpiresAt.Format(time.RFC3339))
	fmt.Fprintf(&b, "GOLC_APIKEY_RAW_TOKEN: %s\n", generated.RawToken)
	fmt.Fprintf(&b, "GOLC_APIKEY_WARNING: store this token now -- it will not be shown again.\n")
	return Result{Stdout: []byte(b.String())}
}

// runAPIKeyList serves the self-registered "api-key list" route: it lists
// every api_keys row's metadata (show.ListAPIKeys, hash-free by
// construction) and renders it as a plain table or, with --json, the same
// data as canonical JSON (the shape REST GET /v1/keys forwards verbatim).
func runAPIKeyList(request Request) Result {
	usage := "api-key list --show <path> [--json]"
	values, err := parseArtnetArgs(usage, request.Args, map[string]bool{"json": true})
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}
	showPath, ok := values["show"]
	if !ok || showPath == "" {
		return Result{ExitCode: 2, Stderr: fmt.Appendf(nil, "GOLC_APIKEY_USAGE: --show is required; usage: %s\n", usage)}
	}

	keys, err := show.ListAPIKeys(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	if values["json"] == "true" {
		views := make([]apiKeyView, 0, len(keys))
		for _, key := range keys {
			views = append(views, toAPIKeyView(key))
		}
		payload, encodeErr := strictjson.CanonicalEncode(apiKeyListView{Keys: views})
		if encodeErr != nil {
			return Result{ExitCode: 1, Stderr: fmt.Appendf(nil, "GOLC_APIKEY_ENCODE_FAILED: %v\n", encodeErr)}
		}
		return Result{Stdout: payload}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%-38s %-10s %-24s %-24s %-24s %s\n", "ID", "PREFIX", "SCOPES", "CREATED_AT", "EXPIRES_AT", "REVOKED_AT")
	for _, key := range keys {
		revokedAt := ""
		if !key.RevokedAt.IsZero() {
			revokedAt = key.RevokedAt.Format(time.RFC3339)
		}
		fmt.Fprintf(&b, "%-38s %-10s %-24s %-24s %-24s %s\n",
			key.KeyID, key.Prefix, strings.Join(scopeStrings(key.Scopes), ","),
			key.CreatedAt.Format(time.RFC3339), key.ExpiresAt.Format(time.RFC3339), revokedAt)
	}
	return Result{Stdout: []byte(b.String())}
}

// runAPIKeyRevoke serves the self-registered "api-key revoke" route: it
// marks the identified api_keys row revoked (show.RevokeAPIKey), which
// every later authentication check (internal/api/auth.go's
// show.IsAPIKeyValid check) immediately honors. Revoking an unknown or
// already-revoked id is a clean, reported failure, never a silent no-op.
func runAPIKeyRevoke(request Request) Result {
	usage := "api-key revoke --id <id> --show <path> [--json]"
	values, err := parseArtnetArgs(usage, request.Args, map[string]bool{"json": true})
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}
	id, ok := values["id"]
	if !ok || id == "" {
		return Result{ExitCode: 2, Stderr: fmt.Appendf(nil, "GOLC_APIKEY_USAGE: --id is required; usage: %s\n", usage)}
	}
	showPath, ok := values["show"]
	if !ok || showPath == "" {
		return Result{ExitCode: 2, Stderr: fmt.Appendf(nil, "GOLC_APIKEY_USAGE: --show is required; usage: %s\n", usage)}
	}

	if err := show.RevokeAPIKey(request.Root, showPath, id); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	if values["json"] == "true" {
		payload, encodeErr := strictjson.CanonicalEncode(apiKeyRevokeView{ID: id, Revoked: true})
		if encodeErr != nil {
			return Result{ExitCode: 1, Stderr: fmt.Appendf(nil, "GOLC_APIKEY_ENCODE_FAILED: %v\n", encodeErr)}
		}
		return Result{Stdout: payload}
	}
	return Result{Stdout: fmt.Appendf(nil, "GOLC_APIKEY_REVOKED: %s\n", id)}
}
