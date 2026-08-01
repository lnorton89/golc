// audit_test.go proves the append-only audit_log table's write/read path
// (07-07-PLAN.md Task 1, D-16/API-06): WriteAuditRecord inserts one row
// through the same openStore/transaction discipline stageRecoveryPoint uses
// (never a second, uncoordinated sql.Open against the same .golc file --
// 07-RESEARCH.md Anti-Pattern), QueryAuditLog round-trips every field
// including nullable revisions, and sequential writes produce strictly
// increasing ids (append-only, never overwritten). This file is `package
// show` (not show_test), matching store_test.go/recovery_test.go's own
// convention, so TestAuditWriterNeverOpensASecondConnection can read this
// package's own audit.go source file directly.
package show

import (
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestWriteAuditRecordRoundTrip proves a single WriteAuditRecord call
// inserts one row carrying every field, and QueryAuditLog reads it back
// unchanged, in occurred_at/id order.
func TestWriteAuditRecordRoundTrip(t *testing.T) {
	root := t.TempDir()
	path := "show.golc"

	rec := AuditRecord{
		OccurredAt:        "2026-07-25T00:00:01Z",
		Actor:             "key-abc123",
		Source:            "http",
		CorrelationID:     "corr-1",
		Route:             "pool create",
		ExpectedRevision:  nullInt64(3),
		ResultingRevision: nullInt64(4),
		Outcome:           "success",
		StatusCode:        200,
		RedactedDetails:   `{"route":"pool create","args":["Main"]}`,
	}
	require.NoError(t, WriteAuditRecord(root, path, rec), "WriteAuditRecord")

	records, err := QueryAuditLog(root, path)
	require.NoError(t, err, "QueryAuditLog")
	require.Len(t, records, 1, "expected exactly 1 audit_log row, got %+v", records)
	got := records[0]
	require.Equal(t, rec.OccurredAt, got.OccurredAt, "round-tripped record does not match: got %+v, want %+v", got, rec)
	require.Equal(t, rec.Actor, got.Actor, "round-tripped record does not match: got %+v, want %+v", got, rec)
	require.Equal(t, rec.Source, got.Source, "round-tripped record does not match: got %+v, want %+v", got, rec)
	require.Equal(t, rec.CorrelationID, got.CorrelationID, "round-tripped record does not match: got %+v, want %+v", got, rec)
	require.Equal(t, rec.Route, got.Route, "round-tripped record does not match: got %+v, want %+v", got, rec)
	require.Equal(t, rec.Outcome, got.Outcome, "round-tripped record does not match: got %+v, want %+v", got, rec)
	require.Equal(t, rec.StatusCode, got.StatusCode, "round-tripped record does not match: got %+v, want %+v", got, rec)
	require.Equal(t, rec.RedactedDetails, got.RedactedDetails, "round-tripped record does not match: got %+v, want %+v", got, rec)
	require.True(t, got.ExpectedRevision.Valid, "expected expected_revision=3, got %+v", got.ExpectedRevision)
	require.EqualValues(t, 3, got.ExpectedRevision.Int64, "expected expected_revision=3, got %+v", got.ExpectedRevision)
	require.True(t, got.ResultingRevision.Valid, "expected resulting_revision=4, got %+v", got.ResultingRevision)
	require.EqualValues(t, 4, got.ResultingRevision.Int64, "expected resulting_revision=4, got %+v", got.ResultingRevision)
	require.Greater(t, got.ID, int64(0), "expected a positive AUTOINCREMENT id")
}

// TestWriteAuditRecordSequentialIDsIncrease proves audit_log is append-only:
// two sequential WriteAuditRecord calls produce two rows with strictly
// increasing ids, and both remain readable (no overwrite).
func TestWriteAuditRecordSequentialIDsIncrease(t *testing.T) {
	root := t.TempDir()
	path := "show.golc"

	first := AuditRecord{OccurredAt: "2026-07-25T00:00:01Z", Actor: "key-1", Source: "http", CorrelationID: "corr-1", Route: "pool create", Outcome: "success", StatusCode: 200, RedactedDetails: "{}"}
	second := AuditRecord{OccurredAt: "2026-07-25T00:00:02Z", Actor: "key-2", Source: "http", CorrelationID: "corr-2", Route: "pool create", Outcome: "failure", StatusCode: 500, RedactedDetails: "{}"}

	require.NoError(t, WriteAuditRecord(root, path, first), "WriteAuditRecord (first)")
	require.NoError(t, WriteAuditRecord(root, path, second), "WriteAuditRecord (second)")

	records, err := QueryAuditLog(root, path)
	require.NoError(t, err, "QueryAuditLog")
	require.Len(t, records, 2, "expected exactly 2 audit_log rows, got %+v", records)
	require.Less(t, records[0].ID, records[1].ID, "expected strictly increasing ids")
	require.Equal(t, "key-1", records[0].Actor, "expected occurred_at/id order [key-1, key-2]")
	require.Equal(t, "key-2", records[1].Actor, "expected occurred_at/id order [key-1, key-2]")
}

// TestWriteAuditRecordNullResultingRevision proves a record with a nil
// resulting revision (failure/dry-run) stores a true SQL NULL,
// distinguishable from a real (including zero) revision.
func TestWriteAuditRecordNullResultingRevision(t *testing.T) {
	root := t.TempDir()
	path := "show.golc"

	rec := AuditRecord{
		OccurredAt:      "2026-07-25T00:00:01Z",
		Actor:           "key-1",
		Source:          "http",
		CorrelationID:   "corr-1",
		Route:           "pool create",
		Outcome:         "dry_run",
		StatusCode:      200,
		RedactedDetails: "{}",
		// ExpectedRevision/ResultingRevision intentionally left zero-value
		// (sql.NullInt64{Valid:false}) -- a dry-run's own contract (D-14):
		// no resulting revision is ever reported.
	}
	require.NoError(t, WriteAuditRecord(root, path, rec), "WriteAuditRecord")

	records, err := QueryAuditLog(root, path)
	require.NoError(t, err, "QueryAuditLog")
	require.Len(t, records, 1, "expected exactly 1 audit_log row")
	require.False(t, records[0].ExpectedRevision.Valid, "expected NULL expected_revision, got %+v", records[0].ExpectedRevision)
	require.False(t, records[0].ResultingRevision.Valid, "expected NULL resulting_revision, got %+v", records[0].ResultingRevision)
}

// TestAuditWriterNeverOpensASecondConnection proves audit.go never opens its
// own sql.Open connection to the .golc file (07-RESEARCH.md Anti-Pattern:
// "Opening a second, uncoordinated SQLite connection for audit writes") --
// every write must instead reuse openStore's already-hardened
// SetMaxOpenConns(1)/busy_timeout/WAL single-writer machinery.
func TestAuditWriterNeverOpensASecondConnection(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller could not resolve this test file's own path")
	auditGoPath := filepath.Join(filepath.Dir(thisFile), "audit.go")
	source, err := os.ReadFile(auditGoPath)
	require.NoError(t, err, "reading audit.go")
	require.False(t, strings.Contains(string(source), "sql.Open("), "audit.go must never call sql.Open directly -- it must reuse openStore's single-writer machinery")
}

// nullInt64 builds a valid sql.NullInt64 from a plain int64 -- test-only
// shorthand for AuditRecord's nullable revision fields.
func nullInt64(v int64) sql.NullInt64 {
	return sql.NullInt64{Int64: v, Valid: true}
}
