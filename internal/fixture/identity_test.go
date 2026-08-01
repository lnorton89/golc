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

	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/fixture"
)

const identityRGBParYAML = `schema_version: 1
manufacturer: Generic
model: RGB PAR
modes:
  - name: Standard
    channels:
      - type: intensity
        occurrence: 0
      - type: color
        occurrence: 0
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
    channels:
      - type: intensity
        occurrence: 0
      - type: color
        occurrence: 0
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
    channels:
      - type: intensity
        occurrence: 0
      - type: color
        occurrence: 0
`

const identityMinimalYAML = `schema_version: 1
manufacturer: Acme
model: Minimal Spot
modes:
  - name: Standard
    channels:
      - type: intensity
        occurrence: 0
capabilities:
  - type: intensity
    range: [0, 1]
`

func TestIdentityHashStable(t *testing.T) {
	def, err := fixture.Decode([]byte(identityRGBParYAML))
	require.NoError(t, err, "Decode(identityRGBParYAML) failed")
	first, err := fixture.Pin(def)
	require.NoError(t, err, "Pin(def) failed")
	require.NotEmpty(t, first.ContentHash, "expected a non-empty ContentHash")

	// Re-read and re-pin the same bytes: identical hash.
	redecoded, err := fixture.Decode([]byte(identityRGBParYAML))
	require.NoError(t, err, "re-Decode(identityRGBParYAML) failed")
	second, err := fixture.Pin(redecoded)
	require.NoError(t, err, "re-Pin(def) failed")
	require.Equal(t, second.ContentHash, first.ContentHash, "expected re-read/re-pin to reproduce the identical hash")

	// A one-byte content change: different hash.
	changed, err := fixture.Decode([]byte(identityRGBParYAMLOneByteChanged))
	require.NoError(t, err, "Decode(identityRGBParYAMLOneByteChanged) failed")
	changedIdentity, err := fixture.Pin(changed)
	require.NoError(t, err, "Pin(changed) failed")
	require.NotEqual(t, first.ContentHash, changedIdentity.ContentHash, "expected a one-byte content change to change ContentHash")
}

func TestIdentityHashKeyOrderStable(t *testing.T) {
	original, err := fixture.Decode([]byte(identityRGBParYAML))
	require.NoError(t, err, "Decode(identityRGBParYAML) failed")
	reordered, err := fixture.Decode([]byte(identityRGBParYAMLReorderedKeys))
	require.NoError(t, err, "Decode(identityRGBParYAMLReorderedKeys) failed")

	originalIdentity, err := fixture.Pin(original)
	require.NoError(t, err, "Pin(original) failed")
	reorderedIdentity, err := fixture.Pin(reordered)
	require.NoError(t, err, "Pin(reordered) failed")

	require.Equal(t, reorderedIdentity.ContentHash, originalIdentity.ContentHash, "expected key-order-equal fixtures to pin to the identical hash")
}

func TestIdentityComplete(t *testing.T) {
	def, err := fixture.Decode([]byte(identityMinimalYAML))
	require.NoError(t, err, "Decode(identityMinimalYAML) failed")
	identity, err := fixture.Pin(def)
	require.NoError(t, err, "Pin(minimal def) failed")
	require.NotEmpty(t, identity.ContentHash, "expected a non-empty ContentHash for a minimal fixture with no optional metadata")
	require.NotZero(t, identity.SchemaVersion, "expected a non-zero SchemaVersion for a minimal fixture")
	require.NotEmpty(t, identity.Revision, "expected a non-empty Revision for a minimal fixture")
}
