// assets.go implements SaveAsset/LoadAsset/DeleteAsset over the assets
// table schema.go establishes -- see that table's own doc comment for why
// it exists at all (a deliberate, documented exception to CONTEXT D-03,
// recorded as 05-CONTEXT.md's own D-14). Mirrors store.go's own
// openStore/closeStoreCheckingErr connection-lifecycle discipline and
// GOLC_SHOW_* error-code convention exactly -- this is not a second
// storage mechanism, just a second table reached through the same single
// per-call connection every other function in this package already opens.
//
// Unlike show_state (one singleton row, decoded/validated as a whole
// document, CONTEXT threat T-02-10), an asset's own bytes are opaque
// binary data with no domain invariants for this package to validate --
// callers (internal/wails's ShowService) are responsible for whatever
// content-type/size checking they need before calling SaveAsset; this
// file trusts whatever bytes it is handed and stores them verbatim.
package show

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// SaveAsset stores data under id (a caller-generated identifier -- the
// only caller today, internal/wails's ShowService.UploadImage, uses
// uuid.NewV7()) in the .golc SQLite database at path (resolved against
// root), alongside its mimeType and original fileName. INSERT OR REPLACE
// makes this idempotent for a re-save under the same id (not exercised by
// today's single caller, which always generates a fresh id per upload,
// but costs nothing to support for a future "replace this exact asset"
// caller).
func SaveAsset(root, path, id, mimeType, fileName string, data []byte) (err error) {
	db, openErr := openStore(root, path)
	if openErr != nil {
		return openErr
	}
	defer closeStoreCheckingErr(db, &err)

	now := time.Now().UTC().Format(time.RFC3339)
	if _, execErr := db.Exec(
		`INSERT OR REPLACE INTO assets (id, mime_type, file_name, data, created_at) VALUES (?, ?, ?, ?, ?)`,
		id, mimeType, fileName, data, now,
	); execErr != nil {
		return fmt.Errorf("GOLC_SHOW_ASSET_SAVE_FAILED: %v", execErr)
	}
	return nil
}

// LoadAsset reads id's own mimeType and raw bytes back from the .golc
// SQLite database at path (resolved against root). GOLC_SHOW_ASSET_NOT_FOUND
// distinguishes "no such asset" from other read failures, mirroring
// readMeta/decodeAndValidate's own distinct-error-per-cause discipline
// elsewhere in this package.
func LoadAsset(root, path, id string) (mimeType string, data []byte, err error) {
	db, openErr := openStore(root, path)
	if openErr != nil {
		return "", nil, openErr
	}
	defer closeStoreCheckingErr(db, &err)

	scanErr := db.QueryRow(`SELECT mime_type, data FROM assets WHERE id = ?`, id).Scan(&mimeType, &data)
	if errors.Is(scanErr, sql.ErrNoRows) {
		return "", nil, fmt.Errorf("GOLC_SHOW_ASSET_NOT_FOUND: no asset %q in this show", id)
	}
	if scanErr != nil {
		return "", nil, fmt.Errorf("GOLC_SHOW_ASSET_LOAD_FAILED: %v", scanErr)
	}
	return mimeType, data, nil
}

// DeleteAsset removes id from the .golc SQLite database at path (resolved
// against root) -- a no-op, not an error, when id does not exist (mirrors
// this codebase's general "deleting something already gone is success,
// not failure" convention, e.g. api_keys' own revoke-in-place idempotency),
// since the only caller (ShowService.RemoveImage, driven by the Desk
// fixture-style modal's own Reset button) has no way to distinguish
// "already removed by a concurrent edit" from "nothing to do" and should
// not need to.
func DeleteAsset(root, path, id string) (err error) {
	db, openErr := openStore(root, path)
	if openErr != nil {
		return openErr
	}
	defer closeStoreCheckingErr(db, &err)

	if _, execErr := db.Exec(`DELETE FROM assets WHERE id = ?`, id); execErr != nil {
		return fmt.Errorf("GOLC_SHOW_ASSET_DELETE_FAILED: %v", execErr)
	}
	return nil
}
