// audit.go implements the audit_log table's write/read path (07-07-PLAN.md
// Task 1, CONTEXT D-16, API-06): WriteAuditRecord inserts one append-only
// row per attempted API mutation, reusing openStore's already-hardened
// connection setup (SetMaxOpenConns(1), busy_timeout, WAL) and the exact
// same db.Begin/tx.Exec/tx.Commit transaction discipline
// store.go's stageRecoveryPoint uses -- never a second, uncoordinated
// sql.Open against the same .golc file (07-RESEARCH.md Anti-Pattern).
// internal/api's redacting post-mutation observer (07-07-PLAN.md Task 2)
// is the sole production caller: it builds an AuditRecord per
// MutationEvent and calls WriteAuditRecord after the mutation itself has
// already completed, so an audit-write failure never blocks or reverses
// the mutation it is recording.
package show

import (
	"database/sql"
	"fmt"
)

// AuditRecord is one audit_log row (07-RESEARCH.md Pattern 5): actor is the
// authenticated API key's id (never the raw key), never the raw
// Authorization header value; ExpectedRevision/ResultingRevision use
// sql.NullInt64 so a failure/dry-run/rejected mutation's genuinely-absent
// revision is stored as a true SQL NULL, distinguishable from a real
// (including zero) revision.
type AuditRecord struct {
	// ID is the row's AUTOINCREMENT primary key; zero on a record not yet
	// written (WriteAuditRecord ignores it -- SQLite assigns it), populated
	// on every record QueryAuditLog returns.
	ID int64
	// OccurredAt is RFC3339, matching show_meta.updated_at's format.
	OccurredAt string
	// Actor is the api-key id (not the raw key) or "cli"/"wails" for a
	// future non-HTTP source.
	Actor string
	// Source is "http", "wails", "cli", etc. (extensible for later
	// script/LLM sources).
	Source string
	// CorrelationID is the request id / idempotency key that ties this row
	// back to one attempted mutation.
	CorrelationID string
	// Route is the internal/command route actually executed, e.g.
	// "pool create".
	Route string
	// ExpectedRevision is the parsed If-Match revision, NULL if the
	// request omitted it.
	ExpectedRevision sql.NullInt64
	// ResultingRevision is show.State.Revision after a successful,
	// durably-applied mutation, NULL on failure/dry-run/rejected/replay
	// against a no-op.
	ResultingRevision sql.NullInt64
	// Outcome is "success" | "failure" | "dry_run" | "rejected", or any
	// other outcome value the caller's own observer seam defines (this
	// column carries no CHECK constraint -- see 07-RESEARCH.md Pattern 5).
	Outcome string
	// StatusCode is the HTTP status the request ultimately resolved to.
	StatusCode int
	// RedactedDetails is canonical JSON with secrets stripped BEFORE this
	// struct was ever built (A5: strip-before-write, never stored raw and
	// redacted at read time).
	RedactedDetails string
}

// WriteAuditRecord inserts one audit_log row for the .golc file at path
// (resolved against root), through the SAME openStore machinery and the
// same db.Begin/tx.Exec/tx.Commit transaction discipline
// stageRecoveryPoint (store.go) already uses -- never a second sql.Open to
// this file. audit_log is append-only: this is always an INSERT, never an
// UPDATE.
func WriteAuditRecord(root, path string, rec AuditRecord) (err error) {
	db, openErr := openStore(root, path)
	if openErr != nil {
		return openErr
	}
	defer closeStoreCheckingErr(db, &err)

	tx, beginErr := db.Begin()
	if beginErr != nil {
		return fmt.Errorf("GOLC_SHOW_STATE_INVALID: beginning audit-log transaction: %v", beginErr)
	}
	defer tx.Rollback() // no-op once Commit has succeeded

	if _, execErr := tx.Exec(
		`INSERT INTO audit_log (occurred_at, actor, source, correlation_id, route, expected_revision, resulting_revision, outcome, status_code, redacted_details) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.OccurredAt, rec.Actor, rec.Source, rec.CorrelationID, rec.Route,
		rec.ExpectedRevision, rec.ResultingRevision, rec.Outcome, rec.StatusCode, rec.RedactedDetails,
	); execErr != nil {
		return fmt.Errorf("GOLC_SHOW_STATE_INVALID: inserting audit_log row: %v", execErr)
	}
	if commitErr := tx.Commit(); commitErr != nil {
		return fmt.Errorf("GOLC_SHOW_STATE_INVALID: committing audit-log transaction: %v", commitErr)
	}
	return nil
}

// QueryAuditLog returns every audit_log row for the .golc file at path
// (resolved against root), ordered by occurred_at then id -- the
// append-only insertion order -- for the audit trail (D-16).
func QueryAuditLog(root, path string) (records []AuditRecord, err error) {
	db, openErr := openStore(root, path)
	if openErr != nil {
		return nil, openErr
	}
	defer closeStoreCheckingErr(db, &err)

	rows, queryErr := db.Query(
		`SELECT id, occurred_at, actor, source, correlation_id, route, expected_revision, resulting_revision, outcome, status_code, redacted_details FROM audit_log ORDER BY occurred_at ASC, id ASC`)
	if queryErr != nil {
		return nil, fmt.Errorf("GOLC_SHOW_STATE_INVALID: querying audit_log: %v", queryErr)
	}
	defer rows.Close()

	for rows.Next() {
		var rec AuditRecord
		if scanErr := rows.Scan(
			&rec.ID, &rec.OccurredAt, &rec.Actor, &rec.Source, &rec.CorrelationID, &rec.Route,
			&rec.ExpectedRevision, &rec.ResultingRevision, &rec.Outcome, &rec.StatusCode, &rec.RedactedDetails,
		); scanErr != nil {
			return nil, fmt.Errorf("GOLC_SHOW_STATE_INVALID: scanning audit_log row: %v", scanErr)
		}
		records = append(records, rec)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("GOLC_SHOW_STATE_INVALID: iterating audit_log: %v", rowsErr)
	}
	return records, nil
}
