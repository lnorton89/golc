// identity.go implements FIXT-05's content-addressed fixture pinning
// (CONTEXT threat T-02-03): a fixture is pinned by a stable identity,
// schema version, content revision, and content hash so a later library
// update cannot silently change an existing show. ContentHash reuses the
// exact strictjson.CanonicalEncode -> crypto/sha256 binding
// internal/trace/apply/guard.go's recomputePlanID already proves (RESEARCH
// Don't-Hand-Roll) -- no bespoke hash scheme is introduced here.
package fixture

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/lnorton89/golc/internal/strictjson"
)

// Identity is a fixture's content-addressed pin: SchemaVersion and
// StableKey identify the fixture as authored, while ContentHash and
// Revision bind exactly the reviewed bytes so a later silent content
// change is detectable (FIXT-05).
type Identity struct {
	// SchemaVersion is the fixture schema version the pinned definition
	// declared.
	SchemaVersion int `json:"schema_version"`
	// ContentHash is the lowercase hex SHA-256 digest of
	// strictjson.CanonicalEncode(def): identical semantic content always
	// reproduces the identical hash, and a one-byte content change always
	// changes it.
	ContentHash string `json:"content_hash"`
	// Revision is a non-empty content revision marker derived from
	// ContentHash, so it is always present for any valid fixture and
	// changes exactly when ContentHash changes.
	Revision string `json:"revision"`
	// StableKey is the fixture's human-stable identity (manufacturer and
	// model), independent of any single revision's content hash.
	StableKey string `json:"stable_key"`
}

// revisionPrefixLength is the number of leading ContentHash hex
// characters Revision is derived from -- short enough to display, long
// enough that an accidental collision between two genuinely different
// contents is not a practical concern (the full ContentHash remains the
// authoritative binding).
const revisionPrefixLength = 12

// Pin computes def's content-addressed Identity. ContentHash is the
// lowercase hex SHA-256 digest of strictjson.CanonicalEncode(def), the
// exact canonical-encode-then-hash binding
// internal/trace/apply/guard.go's recomputePlanID already proves: byte-
// identical canonical bytes (including key-order-equal fixtures decoded
// from differently-ordered YAML, since CanonicalEncode always emits
// struct fields in their declared order) always pin to the identical
// hash, and any content change always changes it.
func Pin(def FixtureDefinition) (Identity, error) {
	encoded, err := strictjson.CanonicalEncode(def)
	if err != nil {
		return Identity{}, fmt.Errorf("GOLC_FIXTURE_PIN_HASH: %v", err)
	}
	sum := sha256.Sum256(encoded)
	contentHash := hex.EncodeToString(sum[:])

	return Identity{
		SchemaVersion: def.SchemaVersion,
		ContentHash:   contentHash,
		Revision:      contentHash[:revisionPrefixLength],
		StableKey:     def.Manufacturer + "/" + def.Model,
	}, nil
}
