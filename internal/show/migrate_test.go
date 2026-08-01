// migrate_test.go pins Migrate's D-08 backup-then-migrate-temp-then-
// atomic-swap contract (05-03-PLAN.md Task 2): a registered migration
// runs end-to-end and produces a verified backup, a newer-than-supported
// file is refused byte-unchanged, an out-of-range on-disk schema_version
// is rejected before it can index the migrations registry, and a
// mid-migration failure leaves the original working file fully intact.
// Since the production `migrations` registry ships empty and
// SchemaVersion is a fixed const == 1, every test here injects a
// synthetic entry at schema_version 0 (the only "older than current"
// slot available) via t.Cleanup to avoid leaking state between tests.
package show

import (
	"bytes"
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/strictjson"
)

// seedRawShow writes schemaVersion/blob directly into an opened store's
// show_meta/show_state rows, bypassing Save's validate()/Revision-bump
// entirely -- simulating a fixture at an arbitrary on-disk schema_version,
// including ones Save itself would never produce (0, negative, newer than
// supported).
func seedRawShow(t *testing.T, root, path string, schemaVersion int, blob []byte) {
	t.Helper()
	db, err := openStore(root, path)
	require.NoError(t, err, "openStore")
	_, err = db.Exec(`UPDATE show_meta SET schema_version = ?, revision = 1, checksum = '', updated_at = '2026-01-01T00:00:00Z' WHERE id = 1`, schemaVersion)
	if err != nil {
		checkpointAndClose(db)
		require.NoError(t, err, "seeding show_meta")
	}
	_, err = db.Exec(`UPDATE show_state SET blob = ? WHERE id = 1`, blob)
	if err != nil {
		checkpointAndClose(db)
		require.NoError(t, err, "seeding show_state")
	}
	require.NoError(t, checkpointAndClose(db), "closing seeded store")
}

// fixturePayload encodes buildNonTrivialState(t) with SchemaVersion
// overridden to version, returning the canonical bytes seedRawShow needs.
func fixturePayload(t *testing.T, version int) []byte {
	t.Helper()
	fixture := buildNonTrivialState(t)
	fixture.SchemaVersion = version
	payload, err := strictjson.CanonicalEncode(fixture)
	require.NoError(t, err, "CanonicalEncode")
	return payload
}

// registerIdentityMigration registers a synthetic 0->1 migration at
// migrations[0] that leaves the blob unchanged (only schema_version
// advances) -- enough to exercise the registry/transaction/re-validate/
// atomic-swap mechanics end-to-end without needing a real historical
// shape change, since schema_version=1 is the only version that has ever
// shipped in production. Registers a call counter and cleans up via
// t.Cleanup so package-level registry state never leaks between tests.
func registerIdentityMigration(t *testing.T) *int {
	t.Helper()
	calls := 0
	migrations[0] = func(blob []byte) ([]byte, error) {
		calls++
		return blob, nil
	}
	t.Cleanup(func() { delete(migrations, 0) })
	return &calls
}

// TestMigrateAppliesRegisteredTransforms proves SHOW-05's core mechanic:
// a fixture at a synthetic schema_version=0 with a registered 0->1
// migration is brought forward to SchemaVersion, running the registered
// transform exactly once.
func TestMigrateAppliesRegisteredTransforms(t *testing.T) {
	root := t.TempDir()
	path := "show.golc"

	seedRawShow(t, root, path, 0, fixturePayload(t, 0))
	calls := registerIdentityMigration(t)

	backupPath, err := Migrate(root, path)
	require.NoError(t, err, "Migrate")
	require.NotEmpty(t, backupPath, "expected a non-empty backupPath")
	require.Equal(t, 1, *calls, "expected the registered migration to run exactly once")

	migrated, err := Load(root, path)
	require.NoError(t, err, "Load after migration")
	require.Equal(t, SchemaVersion, migrated.SchemaVersion, "expected schema_version after migration")
	require.NoError(t, validate(migrated), "migrated State failed validate()")
}

// TestMigrateAppliesOperatorSurfacesAdditiveMigration proves 06-01-PLAN.md
// Task 2's real production migration (schema_version 1 -> 2, CONTEXT
// PLAY-03): a genuinely pre-OperatorSurfaces-field v1 blob (buildNonTrivialState's
// fixture never sets OperatorSurfaces, so its encoded JSON carries no
// "operator_surfaces" key at all) still opens after Migrate, decoding with
// an empty OperatorSurfaces slice rather than failing -- this exercises
// the production migrations[1] entry directly (not a test-injected
// synthetic one), unlike every other test in this file.
func TestMigrateAppliesOperatorSurfacesAdditiveMigration(t *testing.T) {
	root := t.TempDir()
	path := "show.golc"

	seedRawShow(t, root, path, 1, fixturePayload(t, 1))

	backupPath, err := Migrate(root, path)
	require.NoError(t, err, "Migrate")
	require.NotEmpty(t, backupPath, "expected a non-empty backupPath")

	migrated, err := Load(root, path)
	require.NoError(t, err, "Load after migration")
	require.Equal(t, SchemaVersion, migrated.SchemaVersion, "expected schema_version after migration")
	require.Empty(t, migrated.OperatorSurfaces, "expected a pre-field v1 blob to migrate with an empty OperatorSurfaces slice, got %+v", migrated.OperatorSurfaces)
	require.NoError(t, validate(migrated), "migrated State failed validate()")
}

// TestMigrateProducesVerifiedBackup proves Migrate's backup itself
// opens+validates before the swap -- not merely that Migrate reports a
// backupPath string.
func TestMigrateProducesVerifiedBackup(t *testing.T) {
	root := t.TempDir()
	path := "show.golc"

	seedRawShow(t, root, path, 0, fixturePayload(t, 0))
	registerIdentityMigration(t)

	backupPath, err := Migrate(root, path)
	require.NoError(t, err, "Migrate")

	require.NoError(t, verifyBackupReadBack(root, backupPath), "backup produced by Migrate did not itself pass read-back-and-validate")
}

// TestMigrateRefusesNewerFormat proves D-10: a schema_version newer than
// this build supports is refused with ErrSchemaTooNew and the file is
// never rewritten -- byte-for-byte unchanged.
func TestMigrateRefusesNewerFormat(t *testing.T) {
	root := t.TempDir()
	path := "show.golc"

	newer := SchemaVersion + 1
	seedRawShow(t, root, path, newer, fixturePayload(t, newer))

	resolved := resolvePath(root, path)
	before, err := os.ReadFile(resolved)
	require.NoError(t, err, "reading fixture bytes")

	_, migrateErr := Migrate(root, path)
	var tooNew ErrSchemaTooNew
	require.ErrorAs(t, migrateErr, &tooNew, "expected ErrSchemaTooNew")
	require.Equal(t, ErrSchemaTooNew{Found: newer, Supported: SchemaVersion}, tooNew)

	after, err := os.ReadFile(resolved)
	require.NoError(t, err, "re-reading fixture bytes")
	require.True(t, bytes.Equal(before, after), "Migrate rewrote a newer-than-supported file; expected byte-for-byte unchanged")
}

// TestMigrateBoundsChecksVersion proves T-05-02: an out-of-range on-disk
// schema_version is rejected as GOLC_SHOW_STATE_INVALID before it is ever
// used to index the migrations registry, never as ErrSchemaTooNew and
// never by running a registered migration.
func TestMigrateBoundsChecksVersion(t *testing.T) {
	for _, tc := range []struct {
		name    string
		version int
	}{
		{"negative", -1},
		{"absurdly large negative", -999999999},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			path := "show.golc"

			seedRawShow(t, root, path, tc.version, fixturePayload(t, tc.version))

			calls := 0
			migrations[tc.version] = func(blob []byte) ([]byte, error) {
				calls++
				return blob, nil
			}
			t.Cleanup(func() { delete(migrations, tc.version) })

			_, err := Migrate(root, path)
			require.ErrorContains(t, err, "GOLC_SHOW_STATE_INVALID", "expected an error for out-of-range schema_version %d", tc.version)
			var tooNew ErrSchemaTooNew
			require.False(t, errors.As(err, &tooNew), "expected GOLC_SHOW_STATE_INVALID, not ErrSchemaTooNew, for out-of-range schema_version %d", tc.version)
			require.Equal(t, 0, calls, "expected the out-of-range schema_version to never index the migration registry")
		})
	}
}

// TestMigrationForceKillLeavesOriginalIntact simulates an interruption
// after verifiedBackup but before atomicReplace by making the registered
// migration function itself fail, aborting migrateTemp's transaction
// before Migrate ever reaches atomicReplace. Proves the original working
// file remains fully intact -- byte-for-byte and at the raw meta/blob
// level -- regardless of where in the migrate-temp-copy step the failure
// occurs.
func TestMigrationForceKillLeavesOriginalIntact(t *testing.T) {
	root := t.TempDir()
	path := "show.golc"

	payload := fixturePayload(t, 0)
	seedRawShow(t, root, path, 0, payload)

	resolved := resolvePath(root, path)
	before, err := os.ReadFile(resolved)
	require.NoError(t, err, "reading fixture bytes before migration attempt")

	migrations[0] = func(blob []byte) ([]byte, error) {
		return nil, errors.New("simulated mid-migration failure")
	}
	t.Cleanup(func() { delete(migrations, 0) })

	backupPath, migrateErr := Migrate(root, path)
	require.Error(t, migrateErr, "expected Migrate to fail when the registered migration step fails")
	require.NotEmpty(t, backupPath, "expected Migrate to still report the verified backup it produced before the failure")

	after, err := os.ReadFile(resolved)
	require.NoError(t, err, "reading fixture bytes after failed migration")
	require.True(t, bytes.Equal(before, after), "a failed migration modified the original working file; expected it to remain untouched")

	db, err := openStore(root, path)
	require.NoError(t, err, "openStore on original after failed migration")
	version, blob, metaErr := migrationMeta(db)
	closeErr := checkpointAndClose(db)
	require.NoError(t, closeErr, "closing store after reading original")
	require.NoError(t, metaErr, "migrationMeta after failed migration")
	require.Equal(t, 0, version, "expected schema_version to remain 0 after failed migration")
	require.True(t, bytes.Equal(blob, payload), "expected show_state blob to remain the original fixture payload after failed migration")

	// The verified backup itself must still open and validate -- proving
	// the backup taken before the simulated failure was genuinely usable
	// recovery material, not just a path string.
	require.NoError(t, verifyBackupReadBack(root, backupPath), "backup produced before the simulated failure did not pass read-back-and-validate")
}
