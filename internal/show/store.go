// store.go implements Load/Save/LoadForRead over the SQLite-backed .golc
// store schema.go establishes (CONTEXT D-01/D-02/D-03): the domain shape
// (State), validate(), and every internal/command/*.go call site stay
// exactly as they were before Phase 5 -- only the disk-I/O internals move
// from a single JSON file to a single-writer SQLite database. Load and
// LoadForRead both preserve state.go's original "nothing from disk is
// trusted before validate() passes" doctrine (CONTEXT threat T-02-10):
// every returned State has been through strictjson.DecodeStrict and
// validate() first. Save's single transaction folds the recovery-point
// write and prune into the same commit as the state save (CONTEXT D-04:
// so a crash mid-command commits both atomically, or neither), piggy-
// backing on the existing every-command Save trigger rather than adding a
// background writer, timer, or dirty-flag -- this is structurally why
// storage can never enter the playback timing path (internal/show does
// not, and must not, import internal/playback).
package show

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/lnorton89/golc/internal/strictjson"
)

// ErrSchemaTooNew is returned by Load when an opened .golc's
// show_meta.schema_version is greater than this build's SchemaVersion
// (CONTEXT D-10): editing must hard-refuse -- the file is never decoded
// for edit, never rewritten. LoadForRead tolerates this case for
// read-only callers (show inspect/export/diagnose, D-10's "not fully
// blind" requirement).
type ErrSchemaTooNew struct {
	Found     int
	Supported int
}

func (e ErrSchemaTooNew) Error() string {
	return fmt.Sprintf("GOLC_SHOW_SCHEMA_TOO_NEW: file schema_version %d is newer than this build supports (%d)", e.Found, e.Supported)
}

// ErrSchemaMigrationRequired is returned by Load and LoadForRead when an
// opened .golc's show_meta.schema_version is older than this build's
// SchemaVersion: the file must be migrated (05-03-PLAN.md) before it can
// be edited or fully read.
type ErrSchemaMigrationRequired struct {
	Found     int
	Supported int
}

func (e ErrSchemaMigrationRequired) Error() string {
	return fmt.Sprintf("GOLC_SHOW_SCHEMA_MIGRATION_REQUIRED: file schema_version %d requires migration to %d before it can be opened", e.Found, e.Supported)
}

// sha256Hex returns the lowercase hex-encoded SHA-256 digest of payload --
// an integrity check (detects accidental corruption), not a
// confidentiality/security boundary (05-RESEARCH.md Security Domain: no
// encryption requirement exists for this phase).
func sha256Hex(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

// storeMeta mirrors the show_meta singleton row's columns this package
// reads back after a Save (or before a Load/LoadForRead decode).
type storeMeta struct {
	SchemaVersion int
	Revision      int
	Checksum      string
}

// readMeta reads the show_meta singleton row alongside show_state's blob
// length. A schema_version of 0 AND an empty blob marks a freshly-seeded,
// never-yet-saved file (openStore's seed row, which always leaves
// show_state.blob as X'') and is treated identically to sql.ErrNoRows:
// both mean "no show has been saved at this path yet," mirroring the
// pre-SQLite Load's not-yet-existing-file short circuit. A schema_version
// of 0 WITH a non-empty blob is a genuinely-saved historical show at that
// version -- migrate.go's migrationMeta established this exact blob-length
// signal first (05-03-PLAN.md) because schema_version==0 is this
// codebase's only "older than current" value while SchemaVersion stays
// pinned at 1; readMeta reuses the identical signal here so Load/LoadForRead
// can actually surface ErrSchemaMigrationRequired for such a file instead
// of silently treating its saved content as "never saved" (the schema_
// version-only collapse this function used before was dead code from
// Load's perspective: it made ErrSchemaMigrationRequired unreachable via
// any real on-disk file). The bool return reports whether a real
// (ever-saved) show_meta row was found.
func readMeta(db *sql.DB) (storeMeta, bool, error) {
	var meta storeMeta
	var blobLen int
	err := db.QueryRow(`SELECT show_meta.schema_version, show_meta.revision, show_meta.checksum, length(show_state.blob) FROM show_meta, show_state WHERE show_meta.id = 1 AND show_state.id = 1`).
		Scan(&meta.SchemaVersion, &meta.Revision, &meta.Checksum, &blobLen)
	if errors.Is(err, sql.ErrNoRows) {
		return storeMeta{}, false, nil
	}
	if err != nil {
		return storeMeta{}, false, fmt.Errorf("GOLC_SHOW_STATE_INVALID: reading show_meta: %v", err)
	}
	if meta.SchemaVersion == 0 && blobLen == 0 {
		return storeMeta{}, false, nil
	}
	return meta, true, nil
}

// decodeAndValidate reads show_state's blob and returns the strictly
// decoded, whole-State-validated State -- the same "nothing from disk is
// trusted before validate() passes" doctrine state.go's original Load
// established (CONTEXT threat T-02-10), now applied to a SQLite blob
// column instead of a JSON file's raw bytes.
func decodeAndValidate(db *sql.DB) (State, error) {
	var blob []byte
	if err := db.QueryRow(`SELECT blob FROM show_state WHERE id = 1`).Scan(&blob); err != nil {
		return State{}, fmt.Errorf("GOLC_SHOW_STATE_INVALID: reading show_state: %v", err)
	}
	var state State
	if err := strictjson.DecodeStrict(blob, &state); err != nil {
		return State{}, fmt.Errorf("GOLC_SHOW_STATE_INVALID: %v", err)
	}
	if err := validate(state); err != nil {
		return State{}, fmt.Errorf("GOLC_SHOW_STATE_INVALID: %v", err)
	}
	return state, nil
}

// Load opens the .golc SQLite database at path (resolved against root),
// verifies the GOLC application_id door check, and strictly decodes +
// whole-State-validates the stored show (CONTEXT threat T-02-10,
// extended to SQLite as T-05-01). A never-yet-saved file (openStore's
// seed row, schema_version 0) is not an error: it returns a fresh, empty
// State at the current SchemaVersion, exactly mirroring the pre-SQLite
// Load's not-yet-existing-file case, so the first "pool create"/
// "deployment create" against a new show still starts cleanly. A
// newer-than-supported schema_version returns ErrSchemaTooNew (D-10:
// never decode-for-edit); an older one returns
// ErrSchemaMigrationRequired. Load is read-only: no write ever reaches
// the database on this path.
func Load(root, path string) (State, error) {
	db, err := openStore(root, path)
	if err != nil {
		return State{}, err
	}
	defer checkpointAndClose(db)

	meta, ok, err := readMeta(db)
	if err != nil {
		return State{}, err
	}
	if !ok {
		return State{SchemaVersion: SchemaVersion}, nil
	}
	if meta.SchemaVersion > SchemaVersion {
		return State{}, ErrSchemaTooNew{Found: meta.SchemaVersion, Supported: SchemaVersion}
	}
	if meta.SchemaVersion < SchemaVersion {
		return State{}, ErrSchemaMigrationRequired{Found: meta.SchemaVersion, Supported: SchemaVersion}
	}
	return decodeAndValidate(db)
}

// LoadForRead is identical to Load except a newer-than-supported
// schema_version is tolerated (decoded, validated, and returned) rather
// than refused, so read-only callers (show inspect/export/diagnose) are
// not fully blind to a file saved by a newer GOLC build (CONTEXT D-10).
// An older schema_version still returns ErrSchemaMigrationRequired --
// reading an unmigrated older document's blob through the current
// build's validate() would not be a meaningful check. Like Load,
// LoadForRead never writes.
func LoadForRead(root, path string) (State, error) {
	db, err := openStore(root, path)
	if err != nil {
		return State{}, err
	}
	defer checkpointAndClose(db)

	meta, ok, err := readMeta(db)
	if err != nil {
		return State{}, err
	}
	if !ok {
		return State{SchemaVersion: SchemaVersion}, nil
	}
	if meta.SchemaVersion < SchemaVersion {
		return State{}, ErrSchemaMigrationRequired{Found: meta.SchemaVersion, Supported: SchemaVersion}
	}
	return decodeAndValidate(db)
}

// Save validates s, stamps the current SchemaVersion, increments
// Revision, canonically encodes it, and commits it to the .golc SQLite
// database at path (resolved against root) inside one transaction that
// also writes and prunes a recovery point (CONTEXT D-04/D-05/D-06):
// UPDATE show_meta, UPDATE show_state, INSERT INTO recovery_points, then
// DELETE recovery points beyond the newest 5, COMMIT. Because the
// recovery-point write shares this single transaction, a crash mid-save
// commits both the state save and its recovery point atomically, or
// neither (SHOW-03). s is passed by value and never mutated in place:
// callers observe the bumped Revision by calling Load again, exactly like
// the pre-SQLite Save's contract.
func Save(root, path string, s State) error {
	if err := validate(s); err != nil {
		return fmt.Errorf("GOLC_SHOW_STATE_INVALID: %v", err)
	}
	s.SchemaVersion = SchemaVersion
	s.Revision++

	payload, err := strictjson.CanonicalEncode(s)
	if err != nil {
		return fmt.Errorf("GOLC_SHOW_STATE_INVALID: %v", err)
	}
	checksum := sha256Hex(payload)

	db, err := openStore(root, path)
	if err != nil {
		return err
	}
	defer checkpointAndClose(db)

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("GOLC_SHOW_STATE_INVALID: beginning save transaction: %v", err)
	}
	defer tx.Rollback() // no-op once Commit has succeeded

	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := tx.Exec(`UPDATE show_meta SET schema_version = ?, revision = ?, checksum = ?, updated_at = ? WHERE id = 1`,
		s.SchemaVersion, s.Revision, checksum, now); err != nil {
		return fmt.Errorf("GOLC_SHOW_STATE_INVALID: updating show_meta: %v", err)
	}
	if _, err := tx.Exec(`UPDATE show_state SET blob = ? WHERE id = 1`, payload); err != nil {
		return fmt.Errorf("GOLC_SHOW_STATE_INVALID: updating show_state: %v", err)
	}
	// SHOW-03 (CONTEXT D-04): the recovery-point write shares this exact
	// transaction, so a crash mid-command commits both the state save
	// and its recovery point atomically, or neither.
	if _, err := tx.Exec(`INSERT INTO recovery_points (created_at, revision, blob) VALUES (?, ?, ?)`,
		now, s.Revision, payload); err != nil {
		return fmt.Errorf("GOLC_SHOW_STATE_INVALID: inserting recovery point: %v", err)
	}
	// CONTEXT D-06: keep only the newest 5 recovery points, oldest pruned
	// first, in the same transaction as the insert above.
	if _, err := tx.Exec(`DELETE FROM recovery_points WHERE id NOT IN (SELECT id FROM recovery_points ORDER BY id DESC LIMIT 5)`); err != nil {
		return fmt.Errorf("GOLC_SHOW_STATE_INVALID: pruning recovery points: %v", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("GOLC_SHOW_STATE_INVALID: committing save transaction: %v", err)
	}
	return nil
}
