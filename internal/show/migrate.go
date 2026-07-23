// migrate.go implements schema migration (CONTEXT D-08, 05-RESEARCH.md
// Pattern 4/5, Common Pitfall 2): Migrate detects an on-disk
// show_meta.schema_version older than this build's SchemaVersion,
// produces a verifiedBackup first (D-09), migrates a temp copy through
// the ordered `migrations` function registry inside one transaction,
// re-validates the result, and only then atomically replaces the
// original working file via os.Rename (Windows MoveFileEx already
// overwrites, D-08). The original working file is never written to
// before that atomic swap -- a failure or interruption at any earlier
// point leaves it fully intact and openable. The migration registry is a
// Go function map keyed by schema_version, not a SQL migration framework
// (05-RESEARCH.md Common Pitfall 2): this store's "schema" is one blob's
// shape, not evolving SQL DDL.
package show

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/lnorton89/golc/internal/strictjson"
)

// migrations is the ordered blob-shape migration function registry keyed
// by the schema_version a function migrates FROM (its return value is
// the bytes at version+1). The production registry ships empty -- only
// schema_version=1 exists today (05-RESEARCH.md's Recommended Project
// Structure note); tests inject synthetic entries to exercise the engine
// end-to-end and MUST remove them via t.Cleanup so package-level state
// never leaks between tests.
var migrations = map[int]func([]byte) ([]byte, error){}

// RegisterTestMigration registers fn as this build's migration transform
// for fromVersion in the package-level migrations registry. Production
// ships this registry empty since only SchemaVersion=1 has ever existed
// (05-03-PLAN.md/05-03-SUMMARY.md); this exported seam exists solely so
// tests outside this package (05-05-PLAN.md's internal/command/show_test.go)
// can exercise Migrate's verifiedBackup -> migrate-temp -> atomic-replace
// sequence end-to-end through the "show open --confirm-migration" CLI
// route, without this package needing to ship a real historical migration
// function it does not otherwise have. It mirrors migrate_test.go's own
// package-internal registerIdentityMigration helper, exported here because
// that helper lives in a _test.go file and is therefore invisible to other
// packages' tests. Callers MUST invoke the returned cleanup (for example
// via t.Cleanup) so the package-level registry never leaks state between
// tests.
func RegisterTestMigration(fromVersion int, fn func([]byte) ([]byte, error)) (cleanup func()) {
	migrations[fromVersion] = fn
	return func() { delete(migrations, fromVersion) }
}

// migrationMeta reads show_meta.schema_version and show_state.blob
// directly, bypassing store.go's readMeta -- readMeta deliberately
// collapses schema_version==0 into "never saved" for the Load/Save
// fresh-show short circuit, but Migrate needs to distinguish that same
// "truly fresh, still-X''-blob seed row" case from a genuinely-saved
// historical show that happens to carry schema_version==0 (this plan's
// synthetic migration tests exercise exactly that case, since
// schema_version=1 is the only version that has ever shipped in
// production). An empty blob is the seed row's unambiguous signature
// (openStore always seeds show_state.blob as X''), so blob length -- not
// the schema_version integer alone -- is the precise "has this file ever
// actually been saved" signal Migrate needs.
func migrationMeta(db *sql.DB) (schemaVersion int, blob []byte, err error) {
	err = db.QueryRow(`SELECT show_meta.schema_version, show_state.blob FROM show_meta, show_state WHERE show_meta.id = 1 AND show_state.id = 1`).
		Scan(&schemaVersion, &blob)
	if err != nil {
		return 0, nil, fmt.Errorf("GOLC_SHOW_STATE_INVALID: reading migration metadata: %v", err)
	}
	return schemaVersion, blob, nil
}

// Migrate brings the .golc file at path (resolved against root) forward
// to this build's SchemaVersion. A never-yet-saved file (empty
// show_state blob) is a no-op. schema_version == SchemaVersion is a
// no-op. schema_version > SchemaVersion returns ErrSchemaTooNew and
// writes nothing at all (CONTEXT D-10) -- the file is never touched. An
// on-disk schema_version below zero is rejected as GOLC_SHOW_STATE_INVALID
// before it is ever used to index the migrations registry (05-RESEARCH.md
// Security Domain / CONTEXT threat T-05-02; the floor is zero rather than
// one because schema_version==0 is this plan's own synthetic-fixture
// convention for exercising a real historical version, distinguished from
// "never saved" by blob length above, not by the integer itself).
// Otherwise: verifiedBackup runs first (D-09), then the ordered registry
// functions from the found version up to SchemaVersion apply to a temp
// copy inside one transaction, the migrated result is re-validated, and
// only a fully successful migration atomically replaces the original via
// atomicReplace (D-08).
func Migrate(root, path string) (backupPath string, err error) {
	db, openErr := openStore(root, path)
	if openErr != nil {
		return "", openErr
	}
	version, blob, metaErr := migrationMeta(db)
	closeErr := checkpointAndClose(db)
	if metaErr != nil {
		return "", metaErr
	}
	if closeErr != nil {
		return "", fmt.Errorf("GOLC_SHOW_STATE_INVALID: closing store after migration check: %v", closeErr)
	}

	if len(blob) == 0 {
		// A freshly-seeded, never-yet-saved file (openStore's seed row):
		// nothing to migrate, mirroring Load's own not-yet-existing-file
		// short circuit.
		return "", nil
	}
	if version > SchemaVersion {
		// CONTEXT D-10: hard refusal, the file is never touched/rewritten.
		return "", ErrSchemaTooNew{Found: version, Supported: SchemaVersion}
	}
	if version == SchemaVersion {
		return "", nil
	}
	if version < 0 {
		// CONTEXT T-05-02: bounds-check BEFORE ever indexing the
		// migration registry -- an out-of-range on-disk schema_version is
		// never used as a raw map index.
		return "", fmt.Errorf("GOLC_SHOW_STATE_INVALID: on-disk schema_version %d is out of range (must be >= 0)", version)
	}

	backupPath, err = verifiedBackup(root, path)
	if err != nil {
		return "", err
	}

	tempPath := path + ".migrate-tmp"
	if copyErr := copyFile(resolvePath(root, backupPath), resolvePath(root, tempPath)); copyErr != nil {
		return backupPath, fmt.Errorf("GOLC_SHOW_STATE_INVALID: copying verified backup to temp migration file: %v", copyErr)
	}
	defer os.Remove(resolvePath(root, tempPath)) // best-effort cleanup; no-op once atomicReplace has moved it

	if migrateErr := migrateTemp(root, tempPath, version); migrateErr != nil {
		return backupPath, migrateErr
	}
	if replaceErr := atomicReplace(root, path, tempPath); replaceErr != nil {
		return backupPath, replaceErr
	}
	return backupPath, nil
}

// migrateTemp opens tempPath (a copy of the verified backup), applies the
// ordered migrations registry functions from fromVersion up to
// SchemaVersion inside one transaction, re-validates the migrated result,
// bumps schema_version, and commits. The original working file is never
// touched by this step -- it operates entirely on tempPath.
func migrateTemp(root, tempPath string, fromVersion int) error {
	db, err := openStore(root, tempPath)
	if err != nil {
		return fmt.Errorf("GOLC_SHOW_STATE_INVALID: opening temp migration copy: %v", err)
	}
	defer checkpointAndClose(db)

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("GOLC_SHOW_STATE_INVALID: beginning migration transaction: %v", err)
	}
	defer tx.Rollback() // no-op once Commit has succeeded

	var blob []byte
	if err := tx.QueryRow(`SELECT blob FROM show_state WHERE id = 1`).Scan(&blob); err != nil {
		return fmt.Errorf("GOLC_SHOW_STATE_INVALID: reading temp migration blob: %v", err)
	}

	for v := fromVersion; v < SchemaVersion; v++ {
		fn, ok := migrations[v]
		if !ok {
			return fmt.Errorf("GOLC_SHOW_STATE_INVALID: no migration registered for schema_version %d", v)
		}
		next, transformErr := fn(blob)
		if transformErr != nil {
			return fmt.Errorf("GOLC_SHOW_STATE_INVALID: migration from schema_version %d failed: %v", v, transformErr)
		}
		blob = next
	}

	var migrated State
	if err := strictjson.DecodeStrict(blob, &migrated); err != nil {
		return fmt.Errorf("GOLC_SHOW_STATE_INVALID: decoding migrated blob: %v", err)
	}
	migrated.SchemaVersion = SchemaVersion
	if err := validate(migrated); err != nil {
		return fmt.Errorf("GOLC_SHOW_STATE_INVALID: validating migrated state: %v", err)
	}

	finalBlob, err := strictjson.CanonicalEncode(migrated)
	if err != nil {
		return fmt.Errorf("GOLC_SHOW_STATE_INVALID: encoding migrated state: %v", err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := tx.Exec(`UPDATE show_meta SET schema_version = ?, checksum = ?, updated_at = ? WHERE id = 1`,
		SchemaVersion, sha256Hex(finalBlob), now); err != nil {
		return fmt.Errorf("GOLC_SHOW_STATE_INVALID: updating migrated show_meta: %v", err)
	}
	if _, err := tx.Exec(`UPDATE show_state SET blob = ? WHERE id = 1`, finalBlob); err != nil {
		return fmt.Errorf("GOLC_SHOW_STATE_INVALID: updating migrated show_state: %v", err)
	}
	return tx.Commit()
}

// atomicReplace swaps tempPath in over destPath via os.Rename -- Go's
// Windows os.Rename already wraps MoveFileEx with
// MOVEFILE_REPLACE_EXISTING|MOVEFILE_COPY_ALLOWED, so it overwrites
// exactly like POSIX rename (05-RESEARCH.md State of the Art). Every
// *sql.DB handle to both paths must already be closed by the caller --
// migrateTemp's defer and Migrate's earlier checkpointAndClose guarantee
// that here. Any stray -wal/-shm sidecars left at the destination from a
// prior WAL session are removed so the next Open never mixes old and new
// content.
func atomicReplace(root, destPath, tempPath string) error {
	resolvedDest := resolvePath(root, destPath)
	resolvedTemp := resolvePath(root, tempPath)

	if err := os.Rename(resolvedTemp, resolvedDest); err != nil {
		return fmt.Errorf("GOLC_SHOW_MIGRATE_SWAP_FAILED: %v", err)
	}
	_ = os.Remove(resolvedDest + "-wal")
	_ = os.Remove(resolvedDest + "-shm")
	return nil
}

// copyFile raw-copies srcPath's bytes to dstPath. Only ever used to copy
// an already-checkpointed, closed, verified backup file (never a
// possibly-WAL-live working file -- 05-RESEARCH.md Anti-Patterns), so a
// plain byte copy is safe here: the source is a static snapshot, not a
// database that might still have committed data sitting only in a -wal
// sidecar.
func copyFile(srcPath, dstPath string) (err error) {
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := dst.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	_, err = io.Copy(dst, src)
	return err
}
