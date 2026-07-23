// recovery.go implements the READ side of SHOW-04 (CONTEXT D-07): the
// recovery-point WRITE already lands inside Save's own transaction
// (store.go, D-04/D-05/D-06). This file only detects, offers, discards,
// and accepts what Save already wrote -- it never itself performs the
// autosave write.
//
// DetectRecoveryPoints is a pure read: it never mutates show_meta,
// show_state, or recovery_points, so concurrent callers can each
// independently observe the same offer without racing to overwrite the
// saved file (05-02-PLAN.md must_haves: two-process probe). Accepting or
// discarding a recovery point is always a separate, explicit action a
// caller opts into -- this package never auto-applies a recovery point on
// its own (the recovery prohibition: MUST NOT auto-apply, silently
// overwrite, or discard the user's explicitly-saved .golc contents).
package show

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/lnorton89/golc/internal/strictjson"
)

// RecoveryPoint is the allowlisted view of one recovery_points row that
// DetectRecoveryPoints returns: identity, creation time, and the revision
// it captured. The row's blob stays internal to this package until an
// explicit AcceptRecoveryPoint(id) call promotes it -- callers deciding
// whether to offer/inspect a recovery point never need the raw content.
type RecoveryPoint struct {
	ID        int
	CreatedAt string
	Revision  int
}

// offeredRecoveryRevision returns the last clean save's revision (0 when
// the file has never been cleanly saved), the exact threshold
// DetectRecoveryPoints/DiscardRecoveryPoints both use to decide which
// recovery_points rows are "offered": any row whose revision is greater
// than this threshold was written by a Save whose commit never became the
// file's current show_meta.revision, which only happens when the process
// was interrupted between that Save's recovery-point insert and a later
// clean read of show_meta -- i.e. an interrupted session (CONTEXT D-07).
func offeredRecoveryRevision(db *sql.DB) (int, error) {
	meta, ok, err := readMeta(db)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, nil
	}
	return meta.Revision, nil
}

// DetectRecoveryPoints returns every recovery_points row whose revision is
// greater than the last clean save's revision, newest-first (CONTEXT
// D-07). An empty slice (not an error) is returned when nothing is newer
// -- the normal clean-close case, since Save's own recovery-point insert
// always lands at exactly the revision it just committed to show_meta in
// the same transaction, never ahead of it. Detection never writes: it
// never bumps Revision and never mutates show_state or recovery_points.
func DetectRecoveryPoints(root, path string) ([]RecoveryPoint, error) {
	db, err := openStore(root, path)
	if err != nil {
		return nil, err
	}
	defer checkpointAndClose(db)

	threshold, err := offeredRecoveryRevision(db)
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(`SELECT id, created_at, revision FROM recovery_points WHERE revision > ? ORDER BY id DESC`, threshold)
	if err != nil {
		return nil, fmt.Errorf("GOLC_SHOW_STATE_INVALID: querying recovery_points: %v", err)
	}
	defer rows.Close()

	var points []RecoveryPoint
	for rows.Next() {
		var point RecoveryPoint
		if err := rows.Scan(&point.ID, &point.CreatedAt, &point.Revision); err != nil {
			return nil, fmt.Errorf("GOLC_SHOW_STATE_INVALID: scanning recovery_points: %v", err)
		}
		points = append(points, point)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("GOLC_SHOW_STATE_INVALID: iterating recovery_points: %v", err)
	}
	return points, nil
}

// DiscardRecoveryPoints runs an explicit DELETE against every currently
// offered recovery_points row (the same revision-greater-than-last-clean-
// save predicate DetectRecoveryPoints uses) so declined recovery data is
// actually removed, not merely filtered out of a later offer (CONTEXT
// D-07; 05-RESEARCH.md Security row 5 / threat T-05-05). This is always a
// separate, explicit caller action -- never invoked implicitly by
// DetectRecoveryPoints or Load.
func DiscardRecoveryPoints(root, path string) error {
	db, err := openStore(root, path)
	if err != nil {
		return err
	}
	defer checkpointAndClose(db)

	threshold, err := offeredRecoveryRevision(db)
	if err != nil {
		return err
	}

	if _, err := db.Exec(`DELETE FROM recovery_points WHERE revision > ?`, threshold); err != nil {
		return fmt.Errorf("GOLC_SHOW_STATE_INVALID: discarding recovery points: %v", err)
	}
	return nil
}

// AcceptRecoveryPoint promotes the recovery_points row identified by id
// into the working State: it decodes the chosen blob with
// strictjson.DecodeStrict and runs the same whole-State validate() every
// Load/Save already enforces (CONTEXT threat T-02-10, extended to
// recovery blobs as T-05-01) before ever calling Save. An invalid or
// missing recovery blob is refused with GOLC_SHOW_STATE_INVALID and
// Save is never reached -- a recovery point is either fully applied
// (validated, then persisted through the existing Save path, which itself
// re-validates, stamps SchemaVersion, and bumps Revision exactly as any
// other edit would) or not applied at all, never partially. Like
// DiscardRecoveryPoints, this is only ever invoked by an explicit caller
// id -- this package has no auto-apply path.
func AcceptRecoveryPoint(root, path string, id int) error {
	db, err := openStore(root, path)
	if err != nil {
		return err
	}
	var blob []byte
	scanErr := db.QueryRow(`SELECT blob FROM recovery_points WHERE id = ?`, id).Scan(&blob)
	closeErr := checkpointAndClose(db)
	if scanErr != nil {
		if errors.Is(scanErr, sql.ErrNoRows) {
			return fmt.Errorf("GOLC_SHOW_STATE_INVALID: recovery point %d not found", id)
		}
		return fmt.Errorf("GOLC_SHOW_STATE_INVALID: reading recovery point %d: %v", id, scanErr)
	}
	if closeErr != nil {
		return fmt.Errorf("GOLC_SHOW_STATE_INVALID: closing store after reading recovery point %d: %v", id, closeErr)
	}

	var recovered State
	if err := strictjson.DecodeStrict(blob, &recovered); err != nil {
		return fmt.Errorf("GOLC_SHOW_STATE_INVALID: %v", err)
	}
	if err := validate(recovered); err != nil {
		return fmt.Errorf("GOLC_SHOW_STATE_INVALID: %v", err)
	}

	return Save(root, path, recovered)
}
