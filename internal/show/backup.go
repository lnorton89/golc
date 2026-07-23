// backup.go implements the verified-backup mechanism CONTEXT D-09 and
// 05-RESEARCH.md Pattern 3 require: a migration backup is produced with
// VACUUM INTO (never a raw file copy of a possibly-WAL-mode-live
// database -- 05-RESEARCH.md Pitfall 3/Anti-Patterns), then re-opened in
// a FRESH connection and re-validated (strictjson.DecodeStrict +
// validate()) before it is ever trusted. VACUUM INTO returning no error
// is never treated as proof of a valid backup on its own -- only a
// backup that itself decodes and validates as a genuinely loadable show
// is accepted (D-09); a byte-copied-but-corrupt backup is rejected with
// GOLC_SHOW_BACKUP_UNVERIFIABLE.
package show

import (
	"fmt"
	"time"

	"github.com/lnorton89/golc/internal/strictjson"
)

// verifiedBackup produces a timestamped VACUUM INTO snapshot of the .golc
// file at path (resolved against root) and returns its path only after a
// fresh connection to that snapshot has read back show_state's blob,
// strictly decoded it, and run the same validate() every Load/Save trusts
// (CONTEXT D-09, 05-RESEARCH.md Pattern 3). A VACUUM INTO failure is
// GOLC_SHOW_BACKUP_FAILED; a backup that copies successfully but fails
// read-back-and-validate is GOLC_SHOW_BACKUP_UNVERIFIABLE.
func verifiedBackup(root, path string) (backupPath string, err error) {
	db, openErr := openStore(root, path)
	if openErr != nil {
		return "", openErr
	}
	defer closeStoreCheckingErr(db, &err)

	backupPath = path + ".backup-" + time.Now().UTC().Format("20060102T150405Z")
	resolvedBackup := resolvePath(root, backupPath)
	if _, err := db.Exec(`VACUUM INTO ?`, resolvedBackup); err != nil {
		return "", fmt.Errorf("GOLC_SHOW_BACKUP_FAILED: vacuuming %s into %s: %v", path, backupPath, err)
	}

	if err := verifyBackupReadBack(root, backupPath); err != nil {
		return "", err
	}
	return backupPath, nil
}

// verifyBackupReadBack opens the backup at backupPath (resolved against
// root) in a fresh connection, SELECTs its show_state blob, and runs
// strictjson.DecodeStrict + validate() -- the exact read-back-and-validate
// check D-09 requires. Kept separate from verifiedBackup (rather than
// inlined) so tests can exercise this check in isolation against a
// deliberately-corrupted backup file, proving GOLC_SHOW_BACKUP_UNVERIFIABLE
// is actually returned for an invalid backup, not merely claimed by a
// round-trip test alone.
func verifyBackupReadBack(root, backupPath string) (err error) {
	verifyDB, openErr := openStore(root, backupPath)
	if openErr != nil {
		return fmt.Errorf("GOLC_SHOW_BACKUP_UNVERIFIABLE: opening backup %s for verification: %v", backupPath, openErr)
	}
	defer func() {
		if closeErr := checkpointAndClose(verifyDB); closeErr != nil && err == nil {
			err = fmt.Errorf("GOLC_SHOW_BACKUP_UNVERIFIABLE: closing backup %s: %v", backupPath, closeErr)
		}
	}()

	var blob []byte
	if err := verifyDB.QueryRow(`SELECT blob FROM show_state WHERE id = 1`).Scan(&blob); err != nil {
		return fmt.Errorf("GOLC_SHOW_BACKUP_UNVERIFIABLE: reading backup %s blob: %v", backupPath, err)
	}
	var state State
	if err := strictjson.DecodeStrict(blob, &state); err != nil {
		return fmt.Errorf("GOLC_SHOW_BACKUP_UNVERIFIABLE: decoding backup %s: %v", backupPath, err)
	}
	if err := validate(state); err != nil {
		return fmt.Errorf("GOLC_SHOW_BACKUP_UNVERIFIABLE: validating backup %s: %v", backupPath, err)
	}
	return nil
}
