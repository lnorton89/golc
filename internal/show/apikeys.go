// apikeys.go implements the api_keys store (07-04-PLAN.md Task 1, CONTEXT
// D-05/D-08, 07-RESEARCH.md V6 Cryptography): keys are generated with
// crypto/rand (256-bit, base64url) and persisted only as a short lookup
// prefix plus a hex SHA-256 hash of the raw token -- the raw token is
// returned exactly once, by GenerateAPIKey, and is never written to disk
// or logged by anything in this package. Every function here operates
// through openStore, the same single-writer SQLite machinery Load/Save
// already share (schema.go/store.go) -- never a second sql.Open against
// the same .golc file (T-07-04b).
package show

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// apiKeyRawTokenBytes is the raw entropy GenerateAPIKey reads from
// crypto/rand before base64url-encoding it (256 bits, V6 Cryptography).
const apiKeyRawTokenBytes = 32

// apiKeyPrefixLength is the number of leading characters of a raw
// token's base64url encoding stored (unhashed) as the O(1) lookup key.
// The prefix alone reveals nothing about the token's hash and is not the
// secret material itself -- LookupAPIKeyByPrefix + CompareAPIKeyHash's
// constant-time compare is still required to authenticate.
const apiKeyPrefixLength = 8

// APIKeyScope is one coarse domain scope an API key can carry (CONTEXT
// D-08): playback, authoring, or admin -- never a per-domain read/write
// split.
type APIKeyScope string

// The closed set of coarse domain scopes D-08 defines.
const (
	APIKeyScopePlayback  APIKeyScope = "playback"
	APIKeyScopeAuthoring APIKeyScope = "authoring"
	APIKeyScopeAdmin     APIKeyScope = "admin"
)

// validAPIKeyScopes is the closed set ValidateAPIKeyScopes checks
// against.
var validAPIKeyScopes = map[APIKeyScope]bool{
	APIKeyScopePlayback:  true,
	APIKeyScopeAuthoring: true,
	APIKeyScopeAdmin:     true,
}

// ValidateAPIKeyScopes rejects an empty scope list or any scope outside
// the closed D-08 set (GOLC_APIKEY_SCOPE_INVALID).
func ValidateAPIKeyScopes(scopes []APIKeyScope) error {
	if len(scopes) == 0 {
		return fmt.Errorf("GOLC_APIKEY_SCOPE_INVALID: at least one scope is required")
	}
	for _, scope := range scopes {
		if !validAPIKeyScopes[scope] {
			return fmt.Errorf("GOLC_APIKEY_SCOPE_INVALID: %q is not one of playback, authoring, admin", scope)
		}
	}
	return nil
}

// joinAPIKeyScopes canonically comma-joins scopes for the api_keys.scopes
// column.
func joinAPIKeyScopes(scopes []APIKeyScope) string {
	parts := make([]string, len(scopes))
	for i, scope := range scopes {
		parts[i] = string(scope)
	}
	return strings.Join(parts, ",")
}

// splitAPIKeyScopes reverses joinAPIKeyScopes, trimming whitespace and
// dropping empty entries.
func splitAPIKeyScopes(raw string) []APIKeyScope {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	scopes := make([]APIKeyScope, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		scopes = append(scopes, APIKeyScope(trimmed))
	}
	return scopes
}

// HashAPIKeyToken returns the lowercase hex-encoded SHA-256 digest of a
// raw API key token -- the only form of the token this package (or any
// caller following this contract) ever persists (V6: a crypto/rand-
// generated 256-bit token has no brute-force risk a slow password hash
// like bcrypt would mitigate; SHA-256 is the standard high-entropy-token
// hashing choice, see 07-RESEARCH.md Alternatives Considered).
func HashAPIKeyToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// CompareAPIKeyHash reports whether HashAPIKeyToken(token) constant-time-
// equals hash (both lowercase hex). Every authentication call site must
// use this instead of a naive == comparison, so a partially-matching hash
// prefix can never leak a subtly faster/slower timing signal (T-07-05).
func CompareAPIKeyHash(token, hash string) bool {
	computed := HashAPIKeyToken(token)
	return subtle.ConstantTimeCompare([]byte(computed), []byte(hash)) == 1
}

// GeneratedAPIKey is GenerateAPIKey's return value: RawToken is shown to
// the caller exactly once (mint time) and must never be persisted or
// logged beyond that single point; Prefix/Hash are the only form
// InsertAPIKey stores.
type GeneratedAPIKey struct {
	// RawToken is the full crypto/rand-generated, base64url-encoded
	// secret. Show it to the operator once; never store or log it again.
	RawToken string
	// Prefix is RawToken's leading apiKeyPrefixLength characters -- the
	// unhashed O(1) lookup key stored alongside Hash.
	Prefix string
	// Hash is HashAPIKeyToken(RawToken) -- the only form of the secret
	// InsertAPIKey persists.
	Hash string
}

// GenerateAPIKey creates a new crypto/rand-backed 256-bit API key token,
// base64url encoded, plus its short lookup Prefix and hex SHA-256 Hash.
// The raw token is not derivable from the returned Prefix/Hash alone:
// Prefix reveals only a short leading substring, and Hash is a one-way
// digest.
func GenerateAPIKey() (GeneratedAPIKey, error) {
	raw := make([]byte, apiKeyRawTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return GeneratedAPIKey{}, fmt.Errorf("GOLC_APIKEY_GENERATE_FAILED: %v", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	prefix := token
	if len(prefix) > apiKeyPrefixLength {
		prefix = prefix[:apiKeyPrefixLength]
	}
	return GeneratedAPIKey{RawToken: token, Prefix: prefix, Hash: HashAPIKeyToken(token)}, nil
}

// APIKey is one api_keys row's metadata -- never the hash or raw token.
// ListAPIKeys and InsertAPIKey's own return value are always this
// hash-free shape; only APIKeyRecord (LookupAPIKeyByPrefix) additionally
// carries the stored Hash, and only for the exact authentication call
// site that needs to compare it.
type APIKey struct {
	ID        int64
	KeyID     string
	Prefix    string
	Scopes    []APIKeyScope
	CreatedAt time.Time
	ExpiresAt time.Time
	// RevokedAt is the zero time.Time until RevokeAPIKey marks this row
	// revoked.
	RevokedAt time.Time
}

// APIKeyRecord is one api_keys row including its stored Hash --
// LookupAPIKeyByPrefix's return shape, used only by the authentication
// call site that must constant-time-compare a presented token's hash
// against it (T-07-04: never logged, never exposed through ListAPIKeys).
type APIKeyRecord struct {
	APIKey
	Hash string
}

// IsAPIKeyValid reports whether record is usable as of now: not revoked
// (RevokedAt is the zero time) and not expired (now is strictly before
// ExpiresAt). Both expiry and revocation are checked here, server-side,
// on every call -- never cached or trusted from a prior check.
func IsAPIKeyValid(record APIKeyRecord, now time.Time) bool {
	return record.RevokedAt.IsZero() && now.Before(record.ExpiresAt)
}

// formatAPIKeyTime renders t in the same RFC3339 convention every other
// timestamp column in this package's schema uses (schema.go's
// show_meta.updated_at, recovery_points.created_at).
func formatAPIKeyTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

// parseAPIKeyTime parses an RFC3339 timestamp column value, returning the
// zero time.Time for an empty string (api_keys.revoked_at's "not yet
// revoked" sentinel) rather than treating a parse failure as fatal for a
// column that is legitimately allowed to be blank.
func parseAPIKeyTime(raw string) time.Time {
	if raw == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

// InsertAPIKey persists generated's Prefix/Hash (never RawToken) as a new
// api_keys row scoped to scopes and expiring at expiresAt, through the
// same openStore machinery Load/Save already share. The returned APIKey
// carries a freshly assigned KeyID (a UUID, the stable identifier CLI/
// REST revoke calls reference) and never the hash.
func InsertAPIKey(root, showPath string, generated GeneratedAPIKey, scopes []APIKeyScope, expiresAt time.Time) (key APIKey, err error) {
	if err := ValidateAPIKeyScopes(scopes); err != nil {
		return APIKey{}, err
	}

	db, err := openStore(root, showPath)
	if err != nil {
		return APIKey{}, err
	}
	defer closeStoreCheckingErr(db, &err)

	keyID, err := uuid.NewV7()
	if err != nil {
		return APIKey{}, fmt.Errorf("GOLC_APIKEY_INSERT_FAILED: generating key id: %v", err)
	}
	createdAt := time.Now().UTC()

	if _, execErr := db.Exec(
		`INSERT INTO api_keys (key_id, prefix, hash, scopes, created_at, expires_at, revoked_at) VALUES (?, ?, ?, ?, ?, ?, '')`,
		keyID.String(), generated.Prefix, generated.Hash, joinAPIKeyScopes(scopes), formatAPIKeyTime(createdAt), formatAPIKeyTime(expiresAt),
	); execErr != nil {
		return APIKey{}, fmt.Errorf("GOLC_APIKEY_INSERT_FAILED: %v", execErr)
	}

	return APIKey{
		KeyID:     keyID.String(),
		Prefix:    generated.Prefix,
		Scopes:    scopes,
		CreatedAt: createdAt,
		ExpiresAt: expiresAt.UTC(),
	}, nil
}

// scanAPIKeyRow scans one api_keys row (minus id/hash, supplied
// separately by each caller) into an APIKey.
func scanAPIKeyRow(id int64, keyID, prefix, scopesRaw, createdAtRaw, expiresAtRaw, revokedAtRaw string) APIKey {
	return APIKey{
		ID:        id,
		KeyID:     keyID,
		Prefix:    prefix,
		Scopes:    splitAPIKeyScopes(scopesRaw),
		CreatedAt: parseAPIKeyTime(createdAtRaw),
		ExpiresAt: parseAPIKeyTime(expiresAtRaw),
		RevokedAt: parseAPIKeyTime(revokedAtRaw),
	}
}

// LookupAPIKeyByPrefix returns the api_keys row whose stored prefix
// exactly matches prefix, including its Hash for the caller's own
// constant-time compare (CompareAPIKeyHash) against a presented raw
// token. found is false, with a nil error, when no row matches -- an
// unknown prefix is not itself an error. When more than one row happens
// to share a prefix (astronomically unlikely given apiKeyPrefixLength's
// entropy, but not schema-prevented), the oldest matching row is
// returned; CompareAPIKeyHash still safely rejects a token belonging to
// a different row sharing that prefix; it never grants access to the
// wrong key.
func LookupAPIKeyByPrefix(root, showPath, prefix string) (record APIKeyRecord, found bool, err error) {
	db, err := openStore(root, showPath)
	if err != nil {
		return APIKeyRecord{}, false, err
	}
	defer closeStoreCheckingErr(db, &err)

	var (
		id                                                               int64
		keyID, hash, scopesRaw, createdAtRaw, expiresAtRaw, revokedAtRaw string
	)
	queryErr := db.QueryRow(
		`SELECT id, key_id, hash, scopes, created_at, expires_at, revoked_at FROM api_keys WHERE prefix = ? ORDER BY id ASC LIMIT 1`,
		prefix,
	).Scan(&id, &keyID, &hash, &scopesRaw, &createdAtRaw, &expiresAtRaw, &revokedAtRaw)
	if errors.Is(queryErr, sql.ErrNoRows) {
		return APIKeyRecord{}, false, nil
	}
	if queryErr != nil {
		return APIKeyRecord{}, false, fmt.Errorf("GOLC_APIKEY_LOOKUP_FAILED: %v", queryErr)
	}

	return APIKeyRecord{
		APIKey: scanAPIKeyRow(id, keyID, prefix, scopesRaw, createdAtRaw, expiresAtRaw, revokedAtRaw),
		Hash:   hash,
	}, true, nil
}

// ListAPIKeys returns every api_keys row's metadata, oldest first, never
// including Hash or any raw token -- the shape "api-key list" and its
// REST equivalent (GET /v1/keys) both render.
func ListAPIKeys(root, showPath string) (keys []APIKey, err error) {
	db, err := openStore(root, showPath)
	if err != nil {
		return nil, err
	}
	defer closeStoreCheckingErr(db, &err)

	rows, queryErr := db.Query(`SELECT id, key_id, prefix, scopes, created_at, expires_at, revoked_at FROM api_keys ORDER BY id ASC`)
	if queryErr != nil {
		return nil, fmt.Errorf("GOLC_APIKEY_LIST_FAILED: %v", queryErr)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id                                                                 int64
			keyID, prefix, scopesRaw, createdAtRaw, expiresAtRaw, revokedAtRaw string
		)
		if scanErr := rows.Scan(&id, &keyID, &prefix, &scopesRaw, &createdAtRaw, &expiresAtRaw, &revokedAtRaw); scanErr != nil {
			return nil, fmt.Errorf("GOLC_APIKEY_LIST_FAILED: %v", scanErr)
		}
		keys = append(keys, scanAPIKeyRow(id, keyID, prefix, scopesRaw, createdAtRaw, expiresAtRaw, revokedAtRaw))
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("GOLC_APIKEY_LIST_FAILED: %v", rowsErr)
	}
	return keys, nil
}

// RevokeAPIKey marks the api_keys row identified by keyID as revoked
// (revoked_at set to now), immediately making every later
// IsAPIKeyValid/LookupAPIKeyByPrefix check for it report invalid. Revoking
// an unknown or already-revoked key id is GOLC_APIKEY_NOT_FOUND, never a
// silent no-op.
func RevokeAPIKey(root, showPath, keyID string) (err error) {
	db, err := openStore(root, showPath)
	if err != nil {
		return err
	}
	defer closeStoreCheckingErr(db, &err)

	result, execErr := db.Exec(`UPDATE api_keys SET revoked_at = ? WHERE key_id = ? AND revoked_at = ''`, formatAPIKeyTime(time.Now()), keyID)
	if execErr != nil {
		return fmt.Errorf("GOLC_APIKEY_REVOKE_FAILED: %v", execErr)
	}
	affected, affectedErr := result.RowsAffected()
	if affectedErr != nil {
		return fmt.Errorf("GOLC_APIKEY_REVOKE_FAILED: %v", affectedErr)
	}
	if affected == 0 {
		return fmt.Errorf("GOLC_APIKEY_NOT_FOUND: no active api key with id %q", keyID)
	}
	return nil
}
