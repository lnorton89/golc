// audit_test.go pins audit.go's redacting post-mutation audit observer
// (07-07-PLAN.md Task 2, CONTEXT D-16, API-06): every attempted mutation --
// success, failure, rejected, dry-run, or an atomic-batch sub-event --
// fires exactly one audit_log row through show.WriteAuditRecord, with
// secrets stripped from redacted_details BEFORE serialization (A5). This
// file is `package api` (not api_test): it needs no live routecatalog
// bridge (see coverage_test.go's own doc comment for why some *_test.go
// files in this package must instead live in api_test) -- it drives the
// observer seam directly via fireMutationObservers, which is unexported.
package api

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/lnorton89/golc/internal/show"
)

// resetAuditObservers clears the package-level observer registry before and
// after t, so this file's RegisterAuditObserver calls never leak into (or
// pick up registrations left by) another test in the same binary.
func resetAuditObservers(t *testing.T) {
	t.Helper()
	ResetMutationObserversForTesting()
	t.Cleanup(ResetMutationObserversForTesting)
}

// int64Ptr is test-only shorthand for MutationEvent's *int64 revision
// fields.
func int64Ptr(v int64) *int64 { return &v }

// TestAuditObserverRecordsSuccessOutcome proves a successful mutation
// produces exactly one audit_log row with actor=key id, source="http", the
// correlation id, outcome="success", and the resulting revision.
func TestAuditObserverRecordsSuccessOutcome(t *testing.T) {
	resetAuditObservers(t)
	root := t.TempDir()
	showPath := filepath.Join(root, "show.golc")
	RegisterAuditObserver(root, showPath)

	fireMutationObservers(MutationEvent{
		Route: "pool create", Args: []string{"Main"}, Actor: "key-abc123", Source: "http",
		CorrelationID: "corr-1", ResultingRevision: int64Ptr(1), Outcome: "success", StatusCode: 200,
	})

	records, err := show.QueryAuditLog(root, showPath)
	if err != nil {
		t.Fatalf("QueryAuditLog: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected exactly 1 audit_log row, got %d (%+v)", len(records), records)
	}
	rec := records[0]
	if rec.Actor != "key-abc123" {
		t.Fatalf("expected actor=key-abc123, got %q", rec.Actor)
	}
	if rec.Source != "http" {
		t.Fatalf("expected source=http, got %q", rec.Source)
	}
	if rec.CorrelationID != "corr-1" {
		t.Fatalf("expected correlation_id=corr-1, got %q", rec.CorrelationID)
	}
	if rec.Outcome != "success" {
		t.Fatalf("expected outcome=success, got %q", rec.Outcome)
	}
	if !rec.ResultingRevision.Valid || rec.ResultingRevision.Int64 != 1 {
		t.Fatalf("expected resulting_revision=1, got %+v", rec.ResultingRevision)
	}
}

// TestAuditObserverRecordsFailureAndRejectedOutcomes proves a failed
// mutation and a rejected (412/403) mutation each produce exactly one row
// with a null resulting_revision -- one row per attempted mutation, none
// lost, none interleaved.
func TestAuditObserverRecordsFailureAndRejectedOutcomes(t *testing.T) {
	resetAuditObservers(t)
	root := t.TempDir()
	showPath := filepath.Join(root, "show.golc")
	RegisterAuditObserver(root, showPath)

	fireMutationObservers(MutationEvent{
		Route: "pool create", Actor: "key-1", Source: "http", CorrelationID: "corr-1",
		Outcome: "failure", StatusCode: 500,
	})
	fireMutationObservers(MutationEvent{
		Route: "pool create", Actor: "key-1", Source: "http", CorrelationID: "corr-2",
		Outcome: "rejected", StatusCode: 412,
	})

	records, err := show.QueryAuditLog(root, showPath)
	if err != nil {
		t.Fatalf("QueryAuditLog: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected exactly 2 audit_log rows, got %d (%+v)", len(records), records)
	}
	for _, rec := range records {
		if rec.ResultingRevision.Valid {
			t.Fatalf("expected a null resulting_revision for outcome %q, got %+v", rec.Outcome, rec.ResultingRevision)
		}
	}
	if records[0].Outcome != "failure" || records[0].StatusCode != 500 {
		t.Fatalf("expected first row outcome=failure status=500, got outcome=%q status=%d", records[0].Outcome, records[0].StatusCode)
	}
	if records[1].Outcome != "rejected" || records[1].StatusCode != 412 {
		t.Fatalf("expected second row outcome=rejected status=412, got outcome=%q status=%d", records[1].Outcome, records[1].StatusCode)
	}
}

// TestAuditObserverRecordsDryRunOutcome proves a dry-run produces one row
// with outcome "dry_run" and a null resulting_revision (D-14 + D-16).
func TestAuditObserverRecordsDryRunOutcome(t *testing.T) {
	resetAuditObservers(t)
	root := t.TempDir()
	showPath := filepath.Join(root, "show.golc")
	RegisterAuditObserver(root, showPath)

	fireMutationObservers(MutationEvent{
		Route: "pool create", Actor: "key-1", Source: "http", CorrelationID: "corr-1",
		Outcome: "dry_run", StatusCode: 200,
	})

	records, err := show.QueryAuditLog(root, showPath)
	if err != nil {
		t.Fatalf("QueryAuditLog: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected exactly 1 audit_log row, got %d", len(records))
	}
	if records[0].Outcome != "dry_run" {
		t.Fatalf("expected outcome=dry_run, got %q", records[0].Outcome)
	}
	if records[0].ResultingRevision.Valid {
		t.Fatalf("expected a null resulting_revision for a dry-run, got %+v", records[0].ResultingRevision)
	}
}

// TestAuditObserverRedactsSensitiveFields proves redacted_details never
// contains a raw value for a field whose name contains key/token/secret/
// password (case-insensitive), nor a raw Authorization/bearer-shaped
// value -- while still preserving non-sensitive detail (the route and any
// non-sensitive argument).
func TestAuditObserverRedactsSensitiveFields(t *testing.T) {
	resetAuditObservers(t)
	root := t.TempDir()
	showPath := filepath.Join(root, "show.golc")
	RegisterAuditObserver(root, showPath)

	fireMutationObservers(MutationEvent{
		Route: "api-key create", Actor: "key-1", Source: "http", CorrelationID: "corr-1",
		Args:    []string{"--name", "ci-key", "--token", "supersecretvalue123"},
		Outcome: "success", StatusCode: 200, ResultingRevision: int64Ptr(1),
	})
	fireMutationObservers(MutationEvent{
		Route: "config set", Actor: "key-1", Source: "http", CorrelationID: "corr-2",
		Args:    []string{"Authorization", "Bearer abc123xyz789"},
		Outcome: "success", StatusCode: 200, ResultingRevision: int64Ptr(2),
	})

	records, err := show.QueryAuditLog(root, showPath)
	if err != nil {
		t.Fatalf("QueryAuditLog: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected exactly 2 audit_log rows, got %d", len(records))
	}
	for _, rec := range records {
		if strings.Contains(rec.RedactedDetails, "supersecretvalue123") {
			t.Fatalf("redacted_details leaked a raw token value: %s", rec.RedactedDetails)
		}
		if strings.Contains(rec.RedactedDetails, "abc123xyz789") {
			t.Fatalf("redacted_details leaked a raw bearer token value: %s", rec.RedactedDetails)
		}
		if strings.Contains(rec.RedactedDetails, "Bearer ") {
			t.Fatalf("redacted_details leaked a raw Authorization header value: %s", rec.RedactedDetails)
		}
	}
	if !strings.Contains(records[0].RedactedDetails, "ci-key") {
		t.Fatalf("expected non-sensitive detail (ci-key) to survive redaction, got %s", records[0].RedactedDetails)
	}
	if !strings.Contains(records[0].RedactedDetails, "api-key create") {
		t.Fatalf("expected the route to survive redaction, got %s", records[0].RedactedDetails)
	}
}

// TestAuditObserverBatchSubEventsEachWriteOneRow proves an atomic batch's
// per-sub-request MutationEvents (batch.go's own fan-out, all outcome
// "success" after the single aggregated commit) each write their own audit
// row -- one row per sub-mutation, not one row for the whole batch.
func TestAuditObserverBatchSubEventsEachWriteOneRow(t *testing.T) {
	resetAuditObservers(t)
	root := t.TempDir()
	showPath := filepath.Join(root, "show.golc")
	RegisterAuditObserver(root, showPath)

	for i := 0; i < 3; i++ {
		fireMutationObservers(MutationEvent{
			Route: "pool create", Actor: "key-1", Source: "http", CorrelationID: "corr-batch",
			ResultingRevision: int64Ptr(1), Outcome: "success", StatusCode: 200,
		})
	}

	records, err := show.QueryAuditLog(root, showPath)
	if err != nil {
		t.Fatalf("QueryAuditLog: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("expected exactly 3 audit_log rows (one per batch sub-event), got %d", len(records))
	}
}
