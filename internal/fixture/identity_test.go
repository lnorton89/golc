// identity_test.go proves FIXT-05's content-addressed pinning contract
// (02-02-PLAN.md, Task 1): Pin(def) computes a deterministic ContentHash
// over strictjson.CanonicalEncode(def) -- re-reading and re-pinning the
// same bytes reproduces the identical hash, a one-byte content change
// changes it, and two FixtureDefinitions built from semantically-equal
// YAML with different key order pin to the identical hash (canonical
// encoding sorts keys / preserves stable struct field order). A minimal
// fixture with no optional metadata still yields a complete, non-empty
// Identity.
//
// This file intentionally fails to compile until internal/fixture/identity.go
// exists (Task 2 of 02-02-PLAN.md) -- that is the RED state this task
// proves.
package fixture_test

import (
	"testing"

	"github.com/lnorton89/golc/internal/fixture"
)

const identityRGBParYAML = `schema_version: 1
manufacturer: Generic
model: RGB PAR
modes:
  - name: Standard
capabilities:
  - type: intensity
    range: [0, 1]
    comment: Master dimmer
  - type: color
    range: [0, 1]
    comment: RGB color mix
`

// identityRGBParYAMLOneByteChanged differs from identityRGBParYAML by
// exactly one content byte: the model name's trailing "R" becomes "X".
const identityRGBParYAMLOneByteChanged = `schema_version: 1
manufacturer: Generic
model: RGB PAX
modes:
  - name: Standard
capabilities:
  - type: intensity
    range: [0, 1]
    comment: Master dimmer
  - type: color
    range: [0, 1]
    comment: RGB color mix
`

// identityRGBParYAMLReorderedKeys is semantically identical to
// identityRGBParYAML but declares its top-level keys in a different
// source order.
const identityRGBParYAMLReorderedKeys = `manufacturer: Generic
schema_version: 1
capabilities:
  - comment: Master dimmer
    type: intensity
    range: [0, 1]
  - comment: RGB color mix
    type: color
    range: [0, 1]
model: RGB PAR
modes:
  - name: Standard
`

const identityMinimalYAML = `schema_version: 1
manufacturer: Acme
model: Minimal Spot
modes:
  - name: Standard
capabilities:
  - type: intensity
    range: [0, 1]
`

func TestIdentityHashStable(t *testing.T) {
	def, err := fixture.Decode([]byte(identityRGBParYAML))
	if err != nil {
		t.Fatalf("Decode(identityRGBParYAML) failed: %v", err)
	}
	first, err := fixture.Pin(def)
	if err != nil {
		t.Fatalf("Pin(def) failed: %v", err)
	}
	if first.ContentHash == "" {
		t.Fatal("expected a non-empty ContentHash")
	}

	// Re-read and re-pin the same bytes: identical hash.
	redecoded, err := fixture.Decode([]byte(identityRGBParYAML))
	if err != nil {
		t.Fatalf("re-Decode(identityRGBParYAML) failed: %v", err)
	}
	second, err := fixture.Pin(redecoded)
	if err != nil {
		t.Fatalf("re-Pin(def) failed: %v", err)
	}
	if first.ContentHash != second.ContentHash {
		t.Fatalf("expected re-read/re-pin to reproduce the identical hash: %q != %q", first.ContentHash, second.ContentHash)
	}

	// A one-byte content change: different hash.
	changed, err := fixture.Decode([]byte(identityRGBParYAMLOneByteChanged))
	if err != nil {
		t.Fatalf("Decode(identityRGBParYAMLOneByteChanged) failed: %v", err)
	}
	changedIdentity, err := fixture.Pin(changed)
	if err != nil {
		t.Fatalf("Pin(changed) failed: %v", err)
	}
	if changedIdentity.ContentHash == first.ContentHash {
		t.Fatalf("expected a one-byte content change to change ContentHash, both were %q", first.ContentHash)
	}
}

func TestIdentityHashKeyOrderStable(t *testing.T) {
	original, err := fixture.Decode([]byte(identityRGBParYAML))
	if err != nil {
		t.Fatalf("Decode(identityRGBParYAML) failed: %v", err)
	}
	reordered, err := fixture.Decode([]byte(identityRGBParYAMLReorderedKeys))
	if err != nil {
		t.Fatalf("Decode(identityRGBParYAMLReorderedKeys) failed: %v", err)
	}

	originalIdentity, err := fixture.Pin(original)
	if err != nil {
		t.Fatalf("Pin(original) failed: %v", err)
	}
	reorderedIdentity, err := fixture.Pin(reordered)
	if err != nil {
		t.Fatalf("Pin(reordered) failed: %v", err)
	}

	if originalIdentity.ContentHash != reorderedIdentity.ContentHash {
		t.Fatalf("expected key-order-equal fixtures to pin to the identical hash: %q != %q",
			originalIdentity.ContentHash, reorderedIdentity.ContentHash)
	}
}

func TestIdentityComplete(t *testing.T) {
	def, err := fixture.Decode([]byte(identityMinimalYAML))
	if err != nil {
		t.Fatalf("Decode(identityMinimalYAML) failed: %v", err)
	}
	identity, err := fixture.Pin(def)
	if err != nil {
		t.Fatalf("Pin(minimal def) failed: %v", err)
	}
	if identity.ContentHash == "" {
		t.Fatal("expected a non-empty ContentHash for a minimal fixture with no optional metadata")
	}
	if identity.SchemaVersion == 0 {
		t.Fatal("expected a non-zero SchemaVersion for a minimal fixture")
	}
	if identity.Revision == "" {
		t.Fatal("expected a non-empty Revision for a minimal fixture")
	}
}
