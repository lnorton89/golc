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
)

func TestGenerateAPIKey(t *testing.T) {
	generated, err := GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey: %v", err)
	}

	if len(generated.RawToken) < 32 {
		t.Fatalf("expected a high-entropy raw token, got %d chars: %q", len(generated.RawToken), generated.RawToken)
	}
	if generated.Prefix == "" || len(generated.Prefix) >= len(generated.RawToken) {
		t.Fatalf("expected a short lookup prefix shorter than the raw token, got prefix=%q token=%q", generated.Prefix, generated.RawToken)
	}
	if !strings.HasPrefix(generated.RawToken, generated.Prefix) {
		t.Fatalf("expected prefix %q to be a prefix of the raw token %q", generated.Prefix, generated.RawToken)
	}
	if generated.Hash == "" || generated.Hash == generated.RawToken {
		t.Fatalf("expected a distinct hex-encoded hash, got %q", generated.Hash)
	}
	wantHash := HashAPIKeyToken(generated.RawToken)
	if generated.Hash != wantHash {
		t.Fatalf("expected Hash to equal HashAPIKeyToken(RawToken): got %q want %q", generated.Hash, wantHash)
	}

	// The stored form alone must not reveal the raw token: nothing in
	// Prefix/Hash equals RawToken, and a second generation produces an
	// independent, non-colliding token (crypto/rand, not a deterministic
	// derivation from any prior call).
	second, err := GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey (second): %v", err)
	}
	if second.RawToken == generated.RawToken {
		t.Fatalf("expected two independently generated tokens to differ")
	}
}

func TestAPIKeyInsertAndLookup(t *testing.T) {
	root := t.TempDir()
	showPath := "show.golc"

	generated, err := GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey: %v", err)
	}
	scopes := []APIKeyScope{APIKeyScopePlayback, APIKeyScopeAdmin}
	expiresAt := time.Now().UTC().Add(24 * time.Hour)

	inserted, err := InsertAPIKey(root, showPath, generated, scopes, expiresAt)
	if err != nil {
		t.Fatalf("InsertAPIKey: %v", err)
	}
	if inserted.KeyID == "" {
		t.Fatalf("expected InsertAPIKey to assign a non-empty KeyID")
	}
	if inserted.Prefix != generated.Prefix {
		t.Fatalf("expected Prefix %q, got %q", generated.Prefix, inserted.Prefix)
	}

	record, found, err := LookupAPIKeyByPrefix(root, showPath, generated.Prefix)
	if err != nil {
		t.Fatalf("LookupAPIKeyByPrefix: %v", err)
	}
	if !found {
		t.Fatalf("expected LookupAPIKeyByPrefix to find the inserted key")
	}
	if record.KeyID != inserted.KeyID {
		t.Fatalf("expected KeyID %q, got %q", inserted.KeyID, record.KeyID)
	}
	if len(record.Scopes) != 2 || record.Scopes[0] != APIKeyScopePlayback || record.Scopes[1] != APIKeyScopeAdmin {
		t.Fatalf("expected scopes [playback admin], got %v", record.Scopes)
	}

	// Constant-time compare matches only the correct token.
	if !CompareAPIKeyHash(generated.RawToken, record.Hash) {
		t.Fatalf("expected CompareAPIKeyHash to match the correct raw token")
	}
	if CompareAPIKeyHash("wrong-token-entirely", record.Hash) {
		t.Fatalf("expected CompareAPIKeyHash to reject an incorrect token")
	}

	// A prefix with no matching row is reported as not found, never an
	// error.
	_, found, err = LookupAPIKeyByPrefix(root, showPath, "does-not-exist")
	if err != nil {
		t.Fatalf("LookupAPIKeyByPrefix (missing): %v", err)
	}
	if found {
		t.Fatalf("expected LookupAPIKeyByPrefix to report not-found for an unknown prefix")
	}

	keys, err := ListAPIKeys(root, showPath)
	if err != nil {
		t.Fatalf("ListAPIKeys: %v", err)
	}
	if len(keys) != 1 || keys[0].KeyID != inserted.KeyID {
		t.Fatalf("expected exactly one listed key matching %q, got %+v", inserted.KeyID, keys)
	}
}

func TestAPIKeyExpiryAndRevocation(t *testing.T) {
	root := t.TempDir()
	showPath := "show.golc"
	now := time.Now().UTC()

	fresh := APIKeyRecord{APIKey: APIKey{ExpiresAt: now.Add(time.Hour)}}
	if !IsAPIKeyValid(fresh, now) {
		t.Fatalf("expected a fresh, non-expired, non-revoked key to be valid")
	}

	expired := APIKeyRecord{APIKey: APIKey{ExpiresAt: now.Add(-time.Hour)}}
	if IsAPIKeyValid(expired, now) {
		t.Fatalf("expected an expired key to be reported invalid")
	}

	revoked := APIKeyRecord{APIKey: APIKey{ExpiresAt: now.Add(time.Hour), RevokedAt: now.Add(-time.Minute)}}
	if IsAPIKeyValid(revoked, now) {
		t.Fatalf("expected a revoked key to be reported invalid")
	}

	// End-to-end through the real store: RevokeAPIKey marks the row
	// revoked, and a subsequent lookup reflects that via IsAPIKeyValid.
	generated, err := GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey: %v", err)
	}
	inserted, err := InsertAPIKey(root, showPath, generated, []APIKeyScope{APIKeyScopePlayback}, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("InsertAPIKey: %v", err)
	}
	if err := RevokeAPIKey(root, showPath, inserted.KeyID); err != nil {
		t.Fatalf("RevokeAPIKey: %v", err)
	}
	record, found, err := LookupAPIKeyByPrefix(root, showPath, generated.Prefix)
	if err != nil {
		t.Fatalf("LookupAPIKeyByPrefix: %v", err)
	}
	if !found {
		t.Fatalf("expected the revoked key to still be findable by prefix")
	}
	if IsAPIKeyValid(record, now) {
		t.Fatalf("expected the revoked key to be reported invalid after RevokeAPIKey")
	}

	// Revoking an unknown key id is a clean, reported error, never a
	// silent no-op.
	if err := RevokeAPIKey(root, showPath, "does-not-exist"); err == nil {
		t.Fatalf("expected RevokeAPIKey to fail for an unknown key id")
	}
}
