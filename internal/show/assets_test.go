// assets_test.go pins SaveAsset/LoadAsset/DeleteAsset's own round-trip
// contract -- mirrors store_test.go's t.TempDir()/"show.golc" root/path
// convention exactly.
package show

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSaveAssetLoadAssetRoundTrip(t *testing.T) {
	root := t.TempDir()
	path := "show.golc"

	original := []byte{0x47, 0x49, 0x46, 0x38, 0x39, 0x61} // "GIF89a" magic bytes, arbitrary payload
	require.NoError(t, SaveAsset(root, path, "asset-1", "image/gif", "test.gif", original))

	mimeType, data, err := LoadAsset(root, path, "asset-1")
	require.NoError(t, err)
	require.Equal(t, "image/gif", mimeType)
	require.Equal(t, original, data)
}

func TestLoadAssetNotFound(t *testing.T) {
	root := t.TempDir()
	path := "show.golc"

	_, _, err := LoadAsset(root, path, "does-not-exist")
	require.ErrorContains(t, err, "GOLC_SHOW_ASSET_NOT_FOUND")
}

func TestSaveAssetReplacesExistingID(t *testing.T) {
	root := t.TempDir()
	path := "show.golc"

	require.NoError(t, SaveAsset(root, path, "asset-1", "image/png", "first.png", []byte("first")))
	require.NoError(t, SaveAsset(root, path, "asset-1", "image/png", "second.png", []byte("second")))

	_, data, err := LoadAsset(root, path, "asset-1")
	require.NoError(t, err)
	require.Equal(t, []byte("second"), data)
}

func TestDeleteAssetThenLoadNotFound(t *testing.T) {
	root := t.TempDir()
	path := "show.golc"

	require.NoError(t, SaveAsset(root, path, "asset-1", "image/png", "test.png", []byte("data")))
	require.NoError(t, DeleteAsset(root, path, "asset-1"))

	_, _, err := LoadAsset(root, path, "asset-1")
	require.ErrorContains(t, err, "GOLC_SHOW_ASSET_NOT_FOUND")
}

// TestDeleteAssetMissingIsNotError pins DeleteAsset's own doc comment:
// removing an already-gone (or never-existed) asset id is success, not an
// error, mirroring api_keys' revoke-in-place idempotency elsewhere in this
// package.
func TestDeleteAssetMissingIsNotError(t *testing.T) {
	root := t.TempDir()
	path := "show.golc"

	require.NoError(t, DeleteAsset(root, path, "never-existed"))
}
