// Package apply implements the D-17/D-18/D-21 exact-plan apply contract:
// a preview built by internal/trace/reconcile is consumed byte-exact,
// rejected outright if it was tampered with or if relevant repository or
// Linear state changed since it was produced, and otherwise executed one
// operation at a time with a strict read-before-write, one-mutation,
// readback discipline so an uncertain remote outcome (a timeout, a
// partial GraphQL error, a rate limit) is discovered rather than blindly
// retried into a duplicate. Removal is never a side effect of this
// package's create/update apply path: only an explicit, already-reviewed
// archive or unlink request may ever change removal state (CONTEXT D-15),
// and mutating apply is refused outright from a pull_request-triggered CI
// event independent of whatever the calling workflow YAML does or does
// not enforce (CONTEXT D-16).
package apply

import (
	"encoding/json"
	"fmt"

	"github.com/lnorton89/golc/internal/trace/reconcile"
)

// RemoteState is one exact current observation of a single remote object,
// as returned by a RemoteClient read or mutation call. Description is the
// only place a D-14 identity footer may appear.
type RemoteState struct {
	LinearUUID  string            `json:"linear_uuid"`
	Fields      map[string]string `json:"fields"`
	Description string            `json:"description"`
	UpdatedAt   string            `json:"updated_at"`
}

// RemoteClient is the per-operation remote contract a real or fake Linear
// adapter implements for exact-plan apply (CONTEXT D-17/D-18/D-21): read
// the current state of an already-linked object by its immutable UUID,
// discover a not-yet-linked object by its exact D-14 marker footer, and
// perform exactly one create or update mutation. RemoteClient
// deliberately has no method that can archive, unlink, or otherwise
// remove a remote object -- only ApplyRemoval (guard.go), acting on an
// already-reviewed reconcile.ArchivePreview through the existing
// transport.Transport contract, may ever do that (CONTEXT D-15). A future
// 01-24 RemoteClientFactory supplies the concrete fake-in-tests / real
// GraphQL-adapter-in-production implementation; this package only depends
// on the interface.
type RemoteClient interface {
	// ReadByUUID returns the exact current state of the already-linked
	// remote object, or found=false if it no longer exists.
	ReadByUUID(uuid string) (state RemoteState, found bool, err error)
	// ReadByMarker discovers the current state of a not-yet-linked remote
	// object via its exact D-14 identity footer (CONTEXT D-14): zero
	// matches returns found=false so the caller may safely create, and
	// more than one match is reported as an error rather than picking a
	// candidate.
	ReadByMarker(localID string) (state RemoteState, found bool, err error)
	// Create performs exactly one create mutation for op and returns the
	// resulting remote state, including its newly assigned UUID.
	Create(op reconcile.Operation) (RemoteState, error)
	// Update performs exactly one update mutation against the
	// already-linked object uuid, guarded by expectedUpdatedAt (empty
	// when the operation carries no captured precondition), and returns
	// the resulting remote state.
	Update(op reconcile.Operation, uuid, expectedUpdatedAt string) (RemoteState, error)
}

// RetryableError is returned by a RemoteClient mutation when the remote
// rejected or throttled the request in a way the caller should stop and
// retry later (CONTEXT D-21): rate limiting or a partial GraphQL error
// mid-mutation. Apply stops attempting further operations as soon as one
// occurs and reports whatever safe retry metadata the client supplied
// instead of guessing.
type RetryableError struct {
	// Reason is a stable, credential-free diagnostic.
	Reason string
	// RetryAfter is an opaque safe-retry hint (for example an ISO-8601
	// duration or timestamp); empty when the client has no guidance.
	RetryAfter string
}

func (e *RetryableError) Error() string {
	return fmt.Sprintf("GOLC_APPLY_RETRYABLE: %s", e.Reason)
}

// OperationStatus is the exact outcome apply recorded for one operation.
type OperationStatus string

const (
	// StatusCompleted means exactly one mutation was performed and
	// confirmed by an immediate readback.
	StatusCompleted OperationStatus = "completed"
	// StatusNoop means the remote object already exactly matched the
	// desired postcondition (including a matching UUID/footer identity),
	// so no mutation was performed.
	StatusNoop OperationStatus = "noop"
	// StatusPending means the operation was not attempted, or was
	// attempted but its outcome was not confirmed (a retryable error, a
	// stale before-state, or a readback mismatch); it is always safe to
	// retry a pending operation from a fresh or resumed apply.
	StatusPending OperationStatus = "pending"
	// StatusBlocked means a DependsOn parent has no operation in this
	// plan and no existing remote link (most commonly because the parent
	// was excluded by a D-13 conflict), so this operation cannot safely
	// proceed this run.
	StatusBlocked OperationStatus = "blocked"
)

// OperationResult is the exact recorded outcome for one plan operation.
type OperationResult struct {
	LocalID    string          `json:"local_id"`
	Status     OperationStatus `json:"status"`
	LinearUUID *string         `json:"linear_uuid,omitempty"`
	UpdatedAt  *string         `json:"updated_at,omitempty"`
	Reason     string          `json:"reason,omitempty"`
	RetryAfter *string         `json:"retry_after,omitempty"`
}

// Report is the complete, human-reviewable apply outcome for one plan:
// every operation's exact status, in plan order. Report is always a
// separate artifact from reconcile.Plan -- timing and retry metadata
// never become part of the canonical hashed plan bytes.
type Report struct {
	PlanID  string            `json:"plan_id"`
	Results []OperationResult `json:"results"`
}

// canonicalFields renders fields the same way reconcile's
// canonicalFieldsJSON does (sorted keys, {} for nil/empty), so a
// RemoteState's fields can be compared byte-for-byte against an
// Operation's already-canonical After snapshot.
func canonicalFields(fields map[string]string) (string, error) {
	if fields == nil {
		fields = map[string]string{}
	}
	encoded, err := json.Marshal(fields)
	if err != nil {
		return "", fmt.Errorf("GOLC_APPLY_ENCODE: %v", err)
	}
	return string(encoded), nil
}

// fieldsMatch reports whether state's fields exactly match op's already-
// canonical desired postcondition (op.After).
func fieldsMatch(state RemoteState, op reconcile.Operation) bool {
	encoded, err := canonicalFields(state.Fields)
	if err != nil {
		return false
	}
	return encoded == string(op.After)
}

func strPtr(s string) *string { return &s }

func nonEmptyPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
