// schema.go establishes the .golc SQLite schema every internal/show store
// connection shares (CONTEXT D-01/D-03, 05-RESEARCH.md Pattern 1): three
// small tables -- show_meta (singleton: schema_version/revision/checksum/
// updated_at), show_state (singleton: the canonically-encoded State blob),
// and recovery_points (append/prune, capped at 5 rows, D-04/D-05/D-06) --
// plus the PRAGMA application_id stamp that marks a file as GOLC's own
// format (05-RESEARCH.md Pitfall 5: a foreign SQLite file must be rejected
// cleanly at the door with GOLC_SHOW_NOT_GOLC_FORMAT, not a confusing "no
// such table" error two layers deeper).
//
// Every show-mutating CLI invocation is its own short-lived process
// (Load -> mutate -> Save -> exit); each opens a fresh connection to the
// same .golc file. In WAL mode this means -wal/-shm sidecar files are
// expected to appear and disappear across successive invocations over a
// show's editing lifetime -- this is normal SQLite WAL behavior, not
// corruption or a leak (05-RESEARCH.md Pitfall 1). checkpointAndClose runs
// an explicit PRAGMA wal_checkpoint(PASSIVE) before every Close so
// sidecars do not grow unbounded across the thousands of per-command
// processes a long editing session accumulates.
package show

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // pure-Go database/sql driver, registered as "sqlite" (05-RESEARCH.md Standard Stack)
)

// applicationID stamps every .golc file this application creates with the
// ASCII bytes "GOLC" packed as a big-endian int32 (SQLite's PRAGMA
// application_id, https://sqlite.org/pragma.html#pragma_application_id).
// openStore checks this before ever touching show_meta/show_state, so a
// structurally-valid-but-foreign SQLite file is rejected with a clean
// GOLC_SHOW_NOT_GOLC_FORMAT diagnostic instead of a deep "no such table"
// error (05-RESEARCH.md Pitfall 5).
const applicationID = 1196574019

// createTablesSQL is the locked D-03 single-blob-plus-metadata schema
// (05-RESEARCH.md Pattern 1) -- copied verbatim, not a judgment call.
const createTablesSQL = `
CREATE TABLE IF NOT EXISTS show_meta (
  id             INTEGER PRIMARY KEY CHECK (id = 1),
  schema_version INTEGER NOT NULL,
  revision       INTEGER NOT NULL,
  checksum       TEXT    NOT NULL,
  updated_at     TEXT    NOT NULL
);

CREATE TABLE IF NOT EXISTS show_state (
  id   INTEGER PRIMARY KEY CHECK (id = 1),
  blob BLOB NOT NULL
);

CREATE TABLE IF NOT EXISTS recovery_points (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  created_at TEXT    NOT NULL,
  revision   INTEGER NOT NULL,
  blob       BLOB    NOT NULL
);
`

// openStore opens (creating if necessary) the .golc SQLite database at
// path (resolved against root via the same single resolvePath rule every
// other path in this package uses -- CONTEXT T-05-04), applies this
// package's durability PRAGMAs (WAL + synchronous=FULL + foreign_keys=ON),
// verifies or stamps the GOLC application_id door check, ensures the
// three-table schema exists, and seeds the show_meta/show_state singleton
// rows on first create so every later Save can rely on a plain UPDATE
// always finding a row to update. Every failure short of a clean
// GOLC_SHOW_NOT_GOLC_FORMAT door rejection is wrapped as
// GOLC_SHOW_STATE_INVALID -- nothing from disk is trusted before these
// checks pass (CONTEXT threat T-02-10, extended to SQLite as T-05-01).
func openStore(root, path string) (*sql.DB, error) {
	resolved := resolvePath(root, path)
	if err := os.MkdirAll(filepath.Dir(resolved), 0o755); err != nil {
		return nil, fmt.Errorf("GOLC_SHOW_STATE_INVALID: creating directory for %s: %v", resolved, err)
	}

	db, err := sql.Open("sqlite", resolved)
	if err != nil {
		return nil, fmt.Errorf("GOLC_SHOW_STATE_INVALID: opening %s: %v", resolved, err)
	}
	// Single-writer SQLite file: avoid database/sql's connection pool
	// opening a second concurrent connection against the same process's
	// own handle, which would otherwise defeat the point of one
	// short-lived-process-per-command transaction.
	db.SetMaxOpenConns(1)

	for _, pragma := range []string{
		`PRAGMA journal_mode = WAL`,
		`PRAGMA synchronous = FULL`,
		`PRAGMA foreign_keys = ON`,
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("GOLC_SHOW_STATE_INVALID: opening %s: %v", resolved, err)
		}
	}

	var existingAppID int64
	if err := db.QueryRow(`PRAGMA application_id`).Scan(&existingAppID); err != nil {
		db.Close()
		return nil, fmt.Errorf("GOLC_SHOW_STATE_INVALID: reading application_id for %s: %v", resolved, err)
	}
	switch existingAppID {
	case 0:
		// A brand-new (or never-stamped) file: claim it as GOLC's own.
		if _, err := db.Exec(fmt.Sprintf(`PRAGMA application_id = %d`, applicationID)); err != nil {
			db.Close()
			return nil, fmt.Errorf("GOLC_SHOW_STATE_INVALID: stamping application_id for %s: %v", resolved, err)
		}
	case applicationID:
		// Already a GOLC-format file; nothing to do.
	default:
		db.Close()
		return nil, fmt.Errorf("GOLC_SHOW_NOT_GOLC_FORMAT: %s is a SQLite file but was not created by GOLC (application_id %d)", resolved, existingAppID)
	}

	if _, err := db.Exec(createTablesSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("GOLC_SHOW_STATE_INVALID: creating schema in %s: %v", resolved, err)
	}
	// Seed the singleton rows on first create (schema_version=0 marks
	// "never saved yet" -- see store.go's readMeta) so Save's plain
	// UPDATE...WHERE id=1 statements always find a row to update instead
	// of silently affecting zero rows.
	if _, err := db.Exec(`INSERT OR IGNORE INTO show_meta (id, schema_version, revision, checksum, updated_at) VALUES (1, 0, 0, '', '')`); err != nil {
		db.Close()
		return nil, fmt.Errorf("GOLC_SHOW_STATE_INVALID: seeding show_meta in %s: %v", resolved, err)
	}
	if _, err := db.Exec(`INSERT OR IGNORE INTO show_state (id, blob) VALUES (1, X'')`); err != nil {
		db.Close()
		return nil, fmt.Errorf("GOLC_SHOW_STATE_INVALID: seeding show_state in %s: %v", resolved, err)
	}

	return db, nil
}

// checkpointAndClose runs a passive (non-blocking) WAL checkpoint before
// closing db, so -wal/-shm sidecars do not grow unbounded across the many
// short-lived per-command processes a show's editing lifetime accumulates
// (05-RESEARCH.md Pitfall 1). The connection is always closed even if the
// checkpoint itself fails; a checkpoint failure is reported to the caller
// but is never treated as data loss (the WAL content remains valid and
// durable even when a passive checkpoint could not fully drain it).
func checkpointAndClose(db *sql.DB) error {
	_, checkpointErr := db.Exec(`PRAGMA wal_checkpoint(PASSIVE)`)
	closeErr := db.Close()
	if closeErr != nil {
		return closeErr
	}
	return checkpointErr
}

// closeStoreCheckingErr runs checkpointAndClose and, only when *errp is
// still nil (the function's other work has not already failed for a more
// specific reason), sets *errp to the checkpoint/close failure so it
// actually reaches the caller instead of being silently discarded by a
// bare `defer checkpointAndClose(db)`. Intended for
// `defer closeStoreCheckingErr(db, &err)` with a named error return.
func closeStoreCheckingErr(db *sql.DB, errp *error) {
	if closeErr := checkpointAndClose(db); closeErr != nil && *errp == nil {
		*errp = fmt.Errorf("GOLC_SHOW_STATE_INVALID: closing store: %v", closeErr)
	}
}
