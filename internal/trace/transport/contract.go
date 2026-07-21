// Package transport defines the transport-neutral complete-snapshot and
// mutation contract every Linear adapter implements (CONTEXT D-21;
// RESEARCH.md Pattern 7). A Transport never talks to reconcile directly:
// it only reports an exact, exhaustively captured remote read (or the
// exact reason a read could not be trusted) and applies explicit,
// already-reviewed archive/unlink mutations. No credential, network, or
// SDK access is declared here — only the shape a fake or real adapter
// must satisfy.
package transport

// SnapshotStatus enumerates the exact completeness outcome a Transport
// reports for one capture (CONTEXT D-21). Only SnapshotComplete may ever
// feed a reconciliation preview; every other status is a diagnostic that
// blocks without breaking local planning, builds, tests, or application
// runtime.
type SnapshotStatus string

const (
	// SnapshotComplete means every page of the remote read finished and
	// every observed record decoded cleanly.
	SnapshotComplete SnapshotStatus = "complete"
	// SnapshotIncomplete means the capture stopped before reaching the end
	// of the remote result set (for example, the caller aborted early).
	SnapshotIncomplete SnapshotStatus = "incomplete"
	// SnapshotPartial means some pages returned data and others failed
	// (for example, one partial GraphQL error among several successful
	// pages).
	SnapshotPartial SnapshotStatus = "partial"
	// SnapshotCursorAnomaly means pagination cursors did not behave
	// monotonically (a repeated, skipped, or invalidated cursor), so the
	// captured set cannot be trusted as exhaustive.
	SnapshotCursorAnomaly SnapshotStatus = "cursor_anomaly"
	// SnapshotAmbiguous means the capture itself could not resolve a
	// single unambiguous remote state (for example duplicate records for
	// the same remote object within one capture).
	SnapshotAmbiguous SnapshotStatus = "ambiguous"
	// SnapshotRateLimited means the remote API rejected or throttled the
	// capture before it could complete.
	SnapshotRateLimited SnapshotStatus = "rate_limited"
)

// RemoteRecord is one transport-neutral remote object exactly as observed
// during a complete snapshot capture, before any local-identity discovery
// has run. Description is the only place a D-14 visible identity footer
// may appear; Title is diagnostic display text only and never
// establishes identity (CONTEXT D-14; RESEARCH.md Pattern 6).
type RemoteRecord struct {
	LinearUUID  string            `json:"linear_uuid"`
	LinearType  string            `json:"linear_type"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Fields      map[string]string `json:"fields"`
	UpdatedAt   string            `json:"updated_at"`
}

// Snapshot is the complete transport-neutral captured remote read
// (RESEARCH.md Pattern 5/7): the exact evidence a preview would be
// computed against, or the exact reason it cannot be.
type Snapshot struct {
	Status  SnapshotStatus `json:"status"`
	Reason  string         `json:"reason,omitempty"`
	Records []RemoteRecord `json:"records,omitempty"`
}

// MutationKind names one transport-neutral write action. Only explicit,
// already-reviewed archive/unlink actions ever mutate; local absence
// never implies a delete (CONTEXT D-15).
type MutationKind string

const (
	// MutationArchive archives the linked remote object.
	MutationArchive MutationKind = "archive"
	// MutationUnlink removes only the local-to-remote link, leaving the
	// remote object itself untouched.
	MutationUnlink MutationKind = "unlink"
)

// Mutation is one transport-neutral write request against a single
// already-linked managed local entity.
type Mutation struct {
	Kind       MutationKind `json:"kind"`
	LocalID    string       `json:"local_id"`
	LinearUUID string       `json:"linear_uuid"`
}

// Transport is the transport-neutral complete-snapshot/mutation contract
// every Linear adapter (fake or real) implements. reconcile/diff never
// imports a concrete adapter; it only ever depends on this interface, so
// preview logic stays provable against a credential-free fake before any
// live transport exists.
type Transport interface {
	// CaptureSnapshot returns the complete current remote read, or a
	// non-complete Snapshot describing exactly why it could not be
	// trusted as exhaustive (CONTEXT D-21). It never returns a partial
	// Snapshot disguised as complete.
	CaptureSnapshot() (Snapshot, error)

	// Apply executes one already-reviewed explicit mutation and returns
	// the mutation actually performed.
	Apply(Mutation) (Mutation, error)
}
