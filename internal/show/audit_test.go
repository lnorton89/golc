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
)

// TestWriteAuditRecordRoundTrip proves a single WriteAuditRecord call
// inserts one row carrying every field, and QueryAuditLog reads it back
// unchanged, in occurred_at/id order.
func TestWriteAuditRecordRoundTrip(t *testing.T) {
	root := t.TempDir()
	path := "show.golc"

	rec := AuditRecord{
		OccurredAt:         "2026-07-25T00:00:01Z",
		Actor:              "key-abc123",
		Source:             "http",
		CorrelationID:      "corr-1",
		Route:              "pool create",
		ExpectedRevision:   nullInt64(3),
		ResultingRevision:  nullInt64(4),
		Outcome:            "success",
		StatusCode:         200,
		RedactedDetails:    `{"route":"pool create","args":["Main"]}`,
	}
	if err := WriteAuditRecord(root, path, rec); err != nil {
		t.Fatalf("WriteAuditRecord: %v", err)
	}

	records, err := QueryAuditLog(root, path)
	if err != nil {
		t.Fatalf("QueryAuditLog: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected exactly 1 audit_log row, got %d (%+v)", len(records), records)
	}
	got := records[0]
	if got.OccurredAt != rec.OccurredAt || got.Actor != rec.Actor || got.Source != rec.Source ||
		got.CorrelationID != rec.CorrelationID || got.Route != rec.Route || got.Outcome != rec.Outcome ||
		got.StatusCode != rec.StatusCode || got.RedactedDetails != rec.RedactedDetails {
		t.Fatalf("round-tripped record does not match: got %+v, want %+v", got, rec)
	}
	if !got.ExpectedRevision.Valid || got.ExpectedRevision.Int64 != 3 {
		t.Fatalf("expected expected_revision=3, got %+v", got.ExpectedRevision)
	}
	if !got.ResultingRevision.Valid || got.ResultingRevision.Int64 != 4 {
		t.Fatalf("expected resulting_revision=4, got %+v", got.ResultingRevision)
	}
	if got.ID <= 0 {
		t.Fatalf("expected a positive AUTOINCREMENT id, got %d", got.ID)
	}
}

// TestWriteAuditRecordSequentialIDsIncrease proves audit_log is append-only:
// two sequential WriteAuditRecord calls produce two rows with strictly
// increasing ids, and both remain readable (no overwrite).
func TestWriteAuditRecordSequentialIDsIncrease(t *testing.T) {
	root := t.TempDir()
	path := "show.golc"

	first := AuditRecord{OccurredAt: "2026-07-25T00:00:01Z", Actor: "key-1", Source: "http", CorrelationID: "corr-1", Route: "pool create", Outcome: "success", StatusCode: 200, RedactedDetails: "{}"}
	second := AuditRecord{OccurredAt: "2026-07-25T00:00:02Z", Actor: "key-2", Source: "http", CorrelationID: "corr-2", Route: "pool create", Outcome: "failure", StatusCode: 500, RedactedDetails: "{}"}

	if err := WriteAuditRecord(root, path, first); err != nil {
		t.Fatalf("WriteAuditRecord (first): %v", err)
	}
	if err := WriteAuditRecord(root, path, second); err != nil {
		t.Fatalf("WriteAuditRecord (second): %v", err)
	}

	records, err := QueryAuditLog(root, path)
	if err != nil {
		t.Fatalf("QueryAuditLog: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected exactly 2 audit_log rows, got %d (%+v)", len(records), records)
	}
	if records[0].ID >= records[1].ID {
		t.Fatalf("expected strictly increasing ids, got %d then %d", records[0].ID, records[1].ID)
	}
	if records[0].Actor != "key-1" || records[1].Actor != "key-2" {
		t.Fatalf("expected occurred_at/id order [key-1, key-2], got [%s, %s]", records[0].Actor, records[1].Actor)
	}
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
	if err := WriteAuditRecord(root, path, rec); err != nil {
		t.Fatalf("WriteAuditRecord: %v", err)
	}

	records, err := QueryAuditLog(root, path)
	if err != nil {
		t.Fatalf("QueryAuditLog: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected exactly 1 audit_log row, got %d", len(records))
	}
	if records[0].ExpectedRevision.Valid {
		t.Fatalf("expected NULL expected_revision, got %+v", records[0].ExpectedRevision)
	}
	if records[0].ResultingRevision.Valid {
		t.Fatalf("expected NULL resulting_revision, got %+v", records[0].ResultingRevision)
	}
}

// TestAuditWriterNeverOpensASecondConnection proves audit.go never opens its
// own sql.Open connection to the .golc file (07-RESEARCH.md Anti-Pattern:
// "Opening a second, uncoordinated SQLite connection for audit writes") --
// every write must instead reuse openStore's already-hardened
// SetMaxOpenConns(1)/busy_timeout/WAL single-writer machinery.
func TestAuditWriterNeverOpensASecondConnection(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller could not resolve this test file's own path")
	}
	auditGoPath := filepath.Join(filepath.Dir(thisFile), "audit.go")
	source, err := os.ReadFile(auditGoPath)
	if err != nil {
		t.Fatalf("reading audit.go: %v", err)
	}
	if strings.Contains(string(source), "sql.Open(") {
		t.Fatalf("audit.go must never call sql.Open directly -- it must reuse openStore's single-writer machinery")
	}
}

// nullInt64 builds a valid sql.NullInt64 from a plain int64 -- test-only
// shorthand for AuditRecord's nullable revision fields.
func nullInt64(v int64) sql.NullInt64 {
	return sql.NullInt64{Int64: v, Valid: true}
}
