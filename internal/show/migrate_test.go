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
	"fmt"
	"os"
	"strings"
	"testing"

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
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}
	if _, err := db.Exec(`UPDATE show_meta SET schema_version = ?, revision = 1, checksum = '', updated_at = '2026-01-01T00:00:00Z' WHERE id = 1`, schemaVersion); err != nil {
		checkpointAndClose(db)
		t.Fatalf("seeding show_meta: %v", err)
	}
	if _, err := db.Exec(`UPDATE show_state SET blob = ? WHERE id = 1`, blob); err != nil {
		checkpointAndClose(db)
		t.Fatalf("seeding show_state: %v", err)
	}
	if err := checkpointAndClose(db); err != nil {
		t.Fatalf("closing seeded store: %v", err)
	}
}

// fixturePayload encodes buildNonTrivialState(t) with SchemaVersion
// overridden to version, returning the canonical bytes seedRawShow needs.
func fixturePayload(t *testing.T, version int) []byte {
	t.Helper()
	fixture := buildNonTrivialState(t)
	fixture.SchemaVersion = version
	payload, err := strictjson.CanonicalEncode(fixture)
	if err != nil {
		t.Fatalf("CanonicalEncode: %v", err)
	}
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
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if backupPath == "" {
		t.Fatalf("expected a non-empty backupPath")
	}
	if *calls != 1 {
		t.Fatalf("expected the registered migration to run exactly once, ran %d times", *calls)
	}

	migrated, err := Load(root, path)
	if err != nil {
		t.Fatalf("Load after migration: %v", err)
	}
	if migrated.SchemaVersion != SchemaVersion {
		t.Fatalf("expected schema_version %d after migration, got %d", SchemaVersion, migrated.SchemaVersion)
	}
	if err := validate(migrated); err != nil {
		t.Fatalf("migrated State failed validate(): %v", err)
	}
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
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	if err := verifyBackupReadBack(root, backupPath); err != nil {
		t.Fatalf("backup produced by Migrate did not itself pass read-back-and-validate: %v", err)
	}
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
	if err != nil {
		t.Fatalf("reading fixture bytes: %v", err)
	}

	_, migrateErr := Migrate(root, path)
	var tooNew ErrSchemaTooNew
	if !errors.As(migrateErr, &tooNew) {
		t.Fatalf("expected ErrSchemaTooNew, got %v", migrateErr)
	}
	if tooNew.Found != newer || tooNew.Supported != SchemaVersion {
		t.Fatalf("expected ErrSchemaTooNew{Found: %d, Supported: %d}, got %+v", newer, SchemaVersion, tooNew)
	}

	after, err := os.ReadFile(resolved)
	if err != nil {
		t.Fatalf("re-reading fixture bytes: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("Migrate rewrote a newer-than-supported file; expected byte-for-byte unchanged")
	}
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
			if err == nil {
				t.Fatalf("expected an error for out-of-range schema_version %d, got nil", tc.version)
			}
			if !strings.Contains(err.Error(), "GOLC_SHOW_STATE_INVALID") {
				t.Fatalf("expected GOLC_SHOW_STATE_INVALID, got %v", err)
			}
			var tooNew ErrSchemaTooNew
			if errors.As(err, &tooNew) {
				t.Fatalf("expected GOLC_SHOW_STATE_INVALID, not ErrSchemaTooNew, for out-of-range schema_version %d", tc.version)
			}
			if calls != 0 {
				t.Fatalf("expected the out-of-range schema_version to never index the migration registry, but it was called %d times", calls)
			}
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
	if err != nil {
		t.Fatalf("reading fixture bytes before migration attempt: %v", err)
	}

	migrations[0] = func(blob []byte) ([]byte, error) {
		return nil, fmt.Errorf("simulated mid-migration failure")
	}
	t.Cleanup(func() { delete(migrations, 0) })

	backupPath, migrateErr := Migrate(root, path)
	if migrateErr == nil {
		t.Fatalf("expected Migrate to fail when the registered migration step fails")
	}
	if backupPath == "" {
		t.Fatalf("expected Migrate to still report the verified backup it produced before the failure")
	}

	after, err := os.ReadFile(resolved)
	if err != nil {
		t.Fatalf("reading fixture bytes after failed migration: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("a failed migration modified the original working file; expected it to remain untouched")
	}

	db, err := openStore(root, path)
	if err != nil {
		t.Fatalf("openStore on original after failed migration: %v", err)
	}
	version, blob, metaErr := migrationMeta(db)
	if closeErr := checkpointAndClose(db); closeErr != nil {
		t.Fatalf("closing store after reading original: %v", closeErr)
	}
	if metaErr != nil {
		t.Fatalf("migrationMeta after failed migration: %v", metaErr)
	}
	if version != 0 {
		t.Fatalf("expected schema_version to remain 0 after failed migration, got %d", version)
	}
	if !bytes.Equal(blob, payload) {
		t.Fatalf("expected show_state blob to remain the original fixture payload after failed migration")
	}

	// The verified backup itself must still open and validate -- proving
	// the backup taken before the simulated failure was genuinely usable
	// recovery material, not just a path string.
	if err := verifyBackupReadBack(root, backupPath); err != nil {
		t.Fatalf("backup produced before the simulated failure did not pass read-back-and-validate: %v", err)
	}
}
