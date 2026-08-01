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
	"testing"

	"github.com/stretchr/testify/require"

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
	require.NoError(t, err, "QueryAuditLog")
	require.Len(t, records, 1, "expected exactly 1 audit_log row")
	rec := records[0]
	require.Equal(t, "key-abc123", rec.Actor, "expected actor=key-abc123")
	require.Equal(t, "http", rec.Source, "expected source=http")
	require.Equal(t, "corr-1", rec.CorrelationID, "expected correlation_id=corr-1")
	require.Equal(t, "success", rec.Outcome, "expected outcome=success")
	require.True(t, rec.ResultingRevision.Valid, "expected resulting_revision to be valid")
	require.Equal(t, int64(1), rec.ResultingRevision.Int64, "expected resulting_revision=1")
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
	require.NoError(t, err, "QueryAuditLog")
	require.Len(t, records, 2, "expected exactly 2 audit_log rows")
	for _, rec := range records {
		require.False(t, rec.ResultingRevision.Valid, "expected a null resulting_revision for outcome %q, got %+v", rec.Outcome, rec.ResultingRevision)
	}
	require.Equal(t, "failure", records[0].Outcome, "expected first row outcome=failure")
	require.Equal(t, 500, records[0].StatusCode, "expected first row status=500")
	require.Equal(t, "rejected", records[1].Outcome, "expected second row outcome=rejected")
	require.Equal(t, 412, records[1].StatusCode, "expected second row status=412")
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
	require.NoError(t, err, "QueryAuditLog")
	require.Len(t, records, 1, "expected exactly 1 audit_log row")
	require.Equal(t, "dry_run", records[0].Outcome, "expected outcome=dry_run")
	require.False(t, records[0].ResultingRevision.Valid, "expected a null resulting_revision for a dry-run")
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
	require.NoError(t, err, "QueryAuditLog")
	require.Len(t, records, 2, "expected exactly 2 audit_log rows")
	for _, rec := range records {
		require.NotContains(t, rec.RedactedDetails, "supersecretvalue123", "redacted_details leaked a raw token value")
		require.NotContains(t, rec.RedactedDetails, "abc123xyz789", "redacted_details leaked a raw bearer token value")
		require.NotContains(t, rec.RedactedDetails, "Bearer ", "redacted_details leaked a raw Authorization header value")
	}
	require.Contains(t, records[0].RedactedDetails, "ci-key", "expected non-sensitive detail (ci-key) to survive redaction")
	require.Contains(t, records[0].RedactedDetails, "api-key create", "expected the route to survive redaction")
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
	require.NoError(t, err, "QueryAuditLog")
	require.Len(t, records, 3, "expected exactly 3 audit_log rows (one per batch sub-event)")
}
