// backup_test.go pins verifiedBackup's D-09 read-back-and-validate
// contract (05-03-PLAN.md Task 1): a genuine backup round-trips as an
// openable, valid show, and a backup whose blob is corrupted after the
// fact is provably rejected -- VACUUM INTO succeeding is never trusted as
// proof of a valid backup on its own.
package show

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestVerifiedBackupRoundTrips proves a backup produced by verifiedBackup
// opens, decodes, and validates as an identical show to the source it was
// copied from.
func TestVerifiedBackupRoundTrips(t *testing.T) {
	root := t.TempDir()
	path := "show.golc"

	state := buildNonTrivialState(t)
	require.NoError(t, Save(root, path, state), "Save")
	saved, err := Load(root, path)
	require.NoError(t, err, "Load")

	backupPath, err := verifiedBackup(root, path)
	require.NoError(t, err, "verifiedBackup")
	require.NotEmpty(t, backupPath, "expected a non-empty backupPath")

	resolvedBackup := resolvePath(root, backupPath)
	_, statErr := os.Stat(resolvedBackup)
	require.NoError(t, statErr, "expected backup file to exist at %s", resolvedBackup)

	backedUp, err := Load(root, backupPath)
	require.NoError(t, err, "Load(backupPath)")
	assertDomainEqual(t, saved, backedUp)
	require.Equal(t, saved.Revision, backedUp.Revision, "expected backup Revision to match source")
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
	require.NoError(t, Save(root, path, state), "Save")

	backupPath, err := verifiedBackup(root, path)
	require.NoError(t, err, "verifiedBackup")

	db, err := openStore(root, backupPath)
	require.NoError(t, err, "openStore(backupPath)")
	_, err = db.Exec(`UPDATE show_state SET blob = ? WHERE id = 1`, []byte("not valid json{{{"))
	if err != nil {
		db.Close()
		require.NoError(t, err, "corrupting backup blob")
	}
	require.NoError(t, checkpointAndClose(db), "closing corrupted backup store")

	err = verifyBackupReadBack(root, backupPath)
	require.ErrorContains(t, err, "GOLC_SHOW_BACKUP_UNVERIFIABLE", "expected verifyBackupReadBack to reject a corrupted backup")
}
