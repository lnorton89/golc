// apikeys_test.go pins the API-key store contract (07-04-PLAN.md Task 1,
// RED state) before internal/show/apikeys.go exists: GenerateAPIKey
// produces a high-entropy raw token plus a hash-only stored form,
// InsertAPIKey/LookupAPIKeyByPrefix round-trip through the exact same
// openStore machinery Load/Save already use (never a second sql.Open),
// and the validity predicate correctly reports an expired or revoked key
// as invalid. This file is `package show` (not show_test), mirroring
// store_test.go's precedent, so it fails to compile until Task 1's
// implementation lands -- the intended RED state this task proves.
package show

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGenerateAPIKey(t *testing.T) {
	generated, err := GenerateAPIKey()
	require.NoError(t, err, "GenerateAPIKey")

	require.GreaterOrEqual(t, len(generated.RawToken), 32, "expected a high-entropy raw token, got %d chars: %q", len(generated.RawToken), generated.RawToken)
	require.NotEmpty(t, generated.Prefix, "expected a short lookup prefix shorter than the raw token")
	require.Less(t, len(generated.Prefix), len(generated.RawToken), "expected a short lookup prefix shorter than the raw token, got prefix=%q token=%q", generated.Prefix, generated.RawToken)
	require.True(t, strings.HasPrefix(generated.RawToken, generated.Prefix), "expected prefix %q to be a prefix of the raw token %q", generated.Prefix, generated.RawToken)
	require.NotEmpty(t, generated.Hash, "expected a distinct hex-encoded hash")
	require.NotEqual(t, generated.RawToken, generated.Hash, "expected a distinct hex-encoded hash, got %q", generated.Hash)
	wantHash := HashAPIKeyToken(generated.RawToken)
	require.Equal(t, wantHash, generated.Hash, "expected Hash to equal HashAPIKeyToken(RawToken)")

	// The stored form alone must not reveal the raw token: nothing in
	// Prefix/Hash equals RawToken, and a second generation produces an
	// independent, non-colliding token (crypto/rand, not a deterministic
	// derivation from any prior call).
	second, err := GenerateAPIKey()
	require.NoError(t, err, "GenerateAPIKey (second)")
	require.NotEqual(t, generated.RawToken, second.RawToken, "expected two independently generated tokens to differ")
}

func TestAPIKeyInsertAndLookup(t *testing.T) {
	root := t.TempDir()
	showPath := "show.golc"

	generated, err := GenerateAPIKey()
	require.NoError(t, err, "GenerateAPIKey")
	scopes := []APIKeyScope{APIKeyScopePlayback, APIKeyScopeAdmin}
	expiresAt := time.Now().UTC().Add(24 * time.Hour)

	inserted, err := InsertAPIKey(root, showPath, generated, scopes, expiresAt)
	require.NoError(t, err, "InsertAPIKey")
	require.NotEmpty(t, inserted.KeyID, "expected InsertAPIKey to assign a non-empty KeyID")
	require.Equal(t, generated.Prefix, inserted.Prefix)

	record, found, err := LookupAPIKeyByPrefix(root, showPath, generated.Prefix)
	require.NoError(t, err, "LookupAPIKeyByPrefix")
	require.True(t, found, "expected LookupAPIKeyByPrefix to find the inserted key")
	require.Equal(t, inserted.KeyID, record.KeyID)
	require.Equal(t, []APIKeyScope{APIKeyScopePlayback, APIKeyScopeAdmin}, record.Scopes, "expected scopes [playback admin]")

	// Constant-time compare matches only the correct token.
	require.True(t, CompareAPIKeyHash(generated.RawToken, record.Hash), "expected CompareAPIKeyHash to match the correct raw token")
	require.False(t, CompareAPIKeyHash("wrong-token-entirely", record.Hash), "expected CompareAPIKeyHash to reject an incorrect token")

	// A prefix with no matching row is reported as not found, never an
	// error.
	_, found, err = LookupAPIKeyByPrefix(root, showPath, "does-not-exist")
	require.NoError(t, err, "LookupAPIKeyByPrefix (missing)")
	require.False(t, found, "expected LookupAPIKeyByPrefix to report not-found for an unknown prefix")

	keys, err := ListAPIKeys(root, showPath)
	require.NoError(t, err, "ListAPIKeys")
	require.Len(t, keys, 1, "expected exactly one listed key matching %q, got %+v", inserted.KeyID, keys)
	require.Equal(t, inserted.KeyID, keys[0].KeyID)
}

func TestAPIKeyExpiryAndRevocation(t *testing.T) {
	root := t.TempDir()
	showPath := "show.golc"
	now := time.Now().UTC()

	fresh := APIKeyRecord{APIKey: APIKey{ExpiresAt: now.Add(time.Hour)}}
	require.True(t, IsAPIKeyValid(fresh, now), "expected a fresh, non-expired, non-revoked key to be valid")

	expired := APIKeyRecord{APIKey: APIKey{ExpiresAt: now.Add(-time.Hour)}}
	require.False(t, IsAPIKeyValid(expired, now), "expected an expired key to be reported invalid")

	revoked := APIKeyRecord{APIKey: APIKey{ExpiresAt: now.Add(time.Hour), RevokedAt: now.Add(-time.Minute)}}
	require.False(t, IsAPIKeyValid(revoked, now), "expected a revoked key to be reported invalid")

	// End-to-end through the real store: RevokeAPIKey marks the row
	// revoked, and a subsequent lookup reflects that via IsAPIKeyValid.
	generated, err := GenerateAPIKey()
	require.NoError(t, err, "GenerateAPIKey")
	inserted, err := InsertAPIKey(root, showPath, generated, []APIKeyScope{APIKeyScopePlayback}, now.Add(time.Hour))
	require.NoError(t, err, "InsertAPIKey")
	require.NoError(t, RevokeAPIKey(root, showPath, inserted.KeyID), "RevokeAPIKey")
	record, found, err := LookupAPIKeyByPrefix(root, showPath, generated.Prefix)
	require.NoError(t, err, "LookupAPIKeyByPrefix")
	require.True(t, found, "expected the revoked key to still be findable by prefix")
	require.False(t, IsAPIKeyValid(record, now), "expected the revoked key to be reported invalid after RevokeAPIKey")

	// Revoking an unknown key id is a clean, reported error, never a
	// silent no-op.
	err = RevokeAPIKey(root, showPath, "does-not-exist")
	require.Error(t, err, "expected RevokeAPIKey to fail for an unknown key id")
}
