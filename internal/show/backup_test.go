// backup_test.go pins verifiedBackup's D-09 read-back-and-validate
// contract (05-03-PLAN.md Task 1): a genuine backup round-trips as an
// openable, valid show, and a backup whose blob is corrupted after the
// fact is provably rejected -- VACUUM INTO succeeding is never trusted as
// proof of a valid backup on its own.
package show

import (
	"os"
	"strings"
	"testing"
)

// TestVerifiedBackupRoundTrips proves a backup produced by verifiedBackup
// opens, decodes, and validates as an identical show to the source it was
// copied from.
func TestVerifiedBackupRoundTrips(t *testing.T) {
	root := t.TempDir()
	path := "show.golc"

	state := buildNonTrivialState(t)
	if err := Save(root, path, state); err != nil {
		t.Fatalf("Save: %v", err)
	}
	saved, err := Load(root, path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	backupPath, err := verifiedBackup(root, path)
	if err != nil {
		t.Fatalf("verifiedBackup: %v", err)
	}
	if backupPath == "" {
		t.Fatalf("expected a non-empty backupPath")
	}

	resolvedBackup := resolvePath(root, backupPath)
	if _, statErr := os.Stat(resolvedBackup); statErr != nil {
		t.Fatalf("expected backup file to exist at %s: %v", resolvedBackup, statErr)
	}

	backedUp, err := Load(root, backupPath)
	if err != nil {
		t.Fatalf("Load(backupPath): %v", err)
	}
	assertDomainEqual(t, saved, backedUp)
	if backedUp.Revision != saved.Revision {
		t.Fatalf("expected backup Revision to match source (%d), got %d", saved.Revision, backedUp.Revision)
	}
}

// TestVerifiedBackupRejectsCorruptBackup proves D-09's core guarantee:
// after a valid backup is produced, corrupting its blob directly (as if
// bit-rot or a hand-edit had struck the backup file after VACUUM INTO
// succeeded) causes a fresh read-back-and-validate over that backup to be
// rejected with GOLC_SHOW_BACKUP_UNVERIFIABLE -- proving verification
// actually rejects a bad backup, not just reports success once and never
// checks again.
func TestVerifiedBackupRejectsCorruptBackup(t *testing.T) {
	root := t.TempDir()
	path := "show.golc"

	state := buildNonTrivialState(t)
	if err := Save(root, path, state); err != nil {
		t.Fatalf("Save: %v", err)
	}

	backupPath, err := verifiedBackup(root, path)
	if err != nil {
		t.Fatalf("verifiedBackup: %v", err)
	}

	db, err := openStore(root, backupPath)
	if err != nil {
		t.Fatalf("openStore(backupPath): %v", err)
	}
	if _, err := db.Exec(`UPDATE show_state SET blob = ? WHERE id = 1`, []byte("not valid json{{{")); err != nil {
		db.Close()
		t.Fatalf("corrupting backup blob: %v", err)
	}
	if err := checkpointAndClose(db); err != nil {
		t.Fatalf("closing corrupted backup store: %v", err)
	}

	err = verifyBackupReadBack(root, backupPath)
	if err == nil {
		t.Fatalf("expected verifyBackupReadBack to reject a corrupted backup, got no error")
	}
	if !strings.Contains(err.Error(), "GOLC_SHOW_BACKUP_UNVERIFIABLE") {
		t.Fatalf("expected GOLC_SHOW_BACKUP_UNVERIFIABLE, got %v", err)
	}
}
