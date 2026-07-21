// engine.go implements the exact-plan apply state machine (CONTEXT
// D-17/D-18/D-21): every operation is read before it is written, at most
// one mutation is ever attempted per operation, and that mutation is
// always confirmed by an immediate readback. A not-yet-linked operation
// is discovered by its exact D-14 marker footer before any create is
// attempted, so a timeout after a create that actually succeeded remotely
// is detected instead of retried into a duplicate. As soon as any
// operation is not a clean completed/noop outcome -- a retryable remote
// error, a stale before-state, a readback mismatch, or a blocked
// dependency -- apply stops attempting further operations, so the
// achieved outcome is always an exact contiguous prefix of the plan's
// operations, safe to journal and resume (journal.go).
package apply

import (
	"errors"
	"fmt"

	"github.com/lnorton89/golc/internal/trace/catalog"
	"github.com/lnorton89/golc/internal/trace/reconcile"
	"github.com/lnorton89/golc/internal/trace/transport"
)

// classifyBlocked determines, before any mutation is attempted, which
// operations cannot possibly proceed this run: any operation whose
// DependsOn names a local ID that has no operation in this plan and no
// already-existing remote link (most commonly because that parent was
// excluded by a D-13 conflict).
func classifyBlocked(operations []reconcile.Operation, mappings []catalog.RemoteMapping) map[string]string {
	linkedByID := make(map[string]bool, len(mappings))
	for _, mapping := range mappings {
		if mapping.LinearUUID != nil {
			linkedByID[mapping.RepoID] = true
		}
	}
	presentByID := make(map[string]bool, len(operations))
	for _, op := range operations {
		presentByID[op.LocalID] = true
	}

	blocked := make(map[string]string, len(operations))
	for _, op := range operations {
		for _, dependency := range op.DependsOn {
			if presentByID[dependency] || linkedByID[dependency] {
				continue
			}
			blocked[op.LocalID] = fmt.Sprintf(
				"GOLC_APPLY_BLOCKED: %s depends on %s, which has no operation in this plan and no existing remote link (it was likely excluded by a D-13 conflict)",
				op.LocalID, dependency)
		}
	}
	return blocked
}

// retryableResult converts a RemoteClient error into a pending
// OperationResult, carrying safe retry metadata when the client supplied
// a *RetryableError.
func retryableResult(localID string, err error) OperationResult {
	var retryable *RetryableError
	if errors.As(err, &retryable) {
		return OperationResult{
			LocalID:    localID,
			Status:     StatusPending,
			Reason:     retryable.Error(),
			RetryAfter: nonEmptyPtr(retryable.RetryAfter),
		}
	}
	return OperationResult{
		LocalID: localID,
		Status:  StatusPending,
		Reason:  fmt.Sprintf("GOLC_APPLY_OPERATION_FAILED: %s: %v", localID, err),
	}
}

// confirmMutation reads back uuid immediately after a Create/Update call
// and requires it to exactly match op's desired postcondition (CONTEXT
// D-17/D-21): an uncertain or mismatched outcome never reports as
// completed.
func confirmMutation(client RemoteClient, op reconcile.Operation, uuid string) OperationResult {
	confirmed, found, err := client.ReadByUUID(uuid)
	if err != nil {
		return retryableResult(op.LocalID, err)
	}
	if !found || !fieldsMatch(confirmed, op) {
		return OperationResult{
			LocalID: op.LocalID,
			Status:  StatusPending,
			Reason:  fmt.Sprintf("GOLC_APPLY_READBACK_MISMATCH: %s: mutation did not confirm on readback", op.LocalID),
		}
	}
	updatedAt := confirmed.UpdatedAt
	return OperationResult{LocalID: op.LocalID, Status: StatusCompleted, LinearUUID: strPtr(confirmed.LinearUUID), UpdatedAt: &updatedAt}
}

// applyLinkedOperation applies an operation whose intent already carries
// an immutable Linear UUID (CONTEXT D-14): identity is never in question,
// only whether the object already matches the desired postcondition (a
// safe no-op) or whether its captured before-state is still current (a
// safe one mutation).
func applyLinkedOperation(client RemoteClient, op reconcile.Operation, uuid string) OperationResult {
	current, found, err := client.ReadByUUID(uuid)
	if err != nil {
		return retryableResult(op.LocalID, err)
	}
	if found && current.LinearUUID == uuid && fieldsMatch(current, op) {
		updatedAt := current.UpdatedAt
		return OperationResult{LocalID: op.LocalID, Status: StatusNoop, LinearUUID: strPtr(uuid), UpdatedAt: &updatedAt}
	}
	if found && op.ExpectedUpdatedAt != nil && current.UpdatedAt != *op.ExpectedUpdatedAt {
		return OperationResult{
			LocalID: op.LocalID,
			Status:  StatusPending,
			Reason: fmt.Sprintf(
				"GOLC_APPLY_OPERATION_STALE: %s: expected before-state updated_at %q, remote is now %q; re-run linear preview",
				op.LocalID, *op.ExpectedUpdatedAt, current.UpdatedAt),
		}
	}
	expected := ""
	if op.ExpectedUpdatedAt != nil {
		expected = *op.ExpectedUpdatedAt
	}
	updated, err := client.Update(op, uuid, expected)
	if err != nil {
		return retryableResult(op.LocalID, err)
	}
	return confirmMutation(client, op, updated.LinearUUID)
}

// applyUnlinkedOperation applies an operation with no already-linked
// UUID: it always discovers by exact D-14 marker footer first (CONTEXT
// D-17/D-21), so a create that actually succeeded on a prior, interrupted
// attempt is found and treated as achieved instead of retried into a
// duplicate.
func applyUnlinkedOperation(client RemoteClient, op reconcile.Operation) OperationResult {
	discovered, found, err := client.ReadByMarker(op.LocalID)
	if err != nil {
		return retryableResult(op.LocalID, err)
	}
	if found {
		marker, markerFound, markerErr := reconcile.ParseMarker(discovered.Description)
		if markerErr != nil {
			return OperationResult{LocalID: op.LocalID, Status: StatusPending, Reason: fmt.Sprintf("GOLC_APPLY_DISCOVERY_INVALID: %s: %v", op.LocalID, markerErr)}
		}
		if !markerFound {
			return OperationResult{LocalID: op.LocalID, Status: StatusPending, Reason: fmt.Sprintf("GOLC_APPLY_DISCOVERY_INVALID: %s: discovered record carries no identity footer", op.LocalID)}
		}
		if err := reconcile.ValidateMarkerIdentity(marker, op); err != nil {
			return OperationResult{LocalID: op.LocalID, Status: StatusPending, Reason: err.Error()}
		}
		if fieldsMatch(discovered, op) {
			updatedAt := discovered.UpdatedAt
			return OperationResult{LocalID: op.LocalID, Status: StatusNoop, LinearUUID: strPtr(discovered.LinearUUID), UpdatedAt: &updatedAt}
		}
		// Discovered but not yet at the desired postcondition (a prior
		// interrupted attempt created it before crashing/timing out): one
		// update against the discovered object completes it -- never a
		// second create.
		updated, err := client.Update(op, discovered.LinearUUID, discovered.UpdatedAt)
		if err != nil {
			return retryableResult(op.LocalID, err)
		}
		return confirmMutation(client, op, updated.LinearUUID)
	}

	created, err := client.Create(op)
	if err != nil {
		return retryableResult(op.LocalID, err)
	}
	return confirmMutation(client, op, created.LinearUUID)
}

// applyOperation dispatches to the linked or unlinked apply path based on
// whether op's intent already carries an immutable Linear UUID.
func applyOperation(client RemoteClient, op reconcile.Operation) OperationResult {
	if op.LinearUUID != nil {
		return applyLinkedOperation(client, op, *op.LinearUUID)
	}
	return applyUnlinkedOperation(client, op)
}

// applyOperations attempts operations strictly in plan order. The first
// operation that is not a clean completed/noop outcome -- blocked or
// otherwise -- stops all further attempts; every remaining operation is
// reported StatusPending without ever being attempted, so the achieved
// outcome is always an exact contiguous prefix (CONTEXT D-21).
func applyOperations(client RemoteClient, operations []reconcile.Operation, blocked map[string]string) []OperationResult {
	results := make([]OperationResult, 0, len(operations))
	stopped := false
	for _, op := range operations {
		if stopped {
			results = append(results, OperationResult{
				LocalID: op.LocalID,
				Status:  StatusPending,
				Reason:  "GOLC_APPLY_STOPPED: apply stopped before this operation could be attempted",
			})
			continue
		}
		if reason, isBlocked := blocked[op.LocalID]; isBlocked {
			results = append(results, OperationResult{LocalID: op.LocalID, Status: StatusBlocked, Reason: reason})
			stopped = true
			continue
		}
		result := applyOperation(client, op)
		if result.Status != StatusCompleted && result.Status != StatusNoop {
			stopped = true
		}
		results = append(results, result)
	}
	return results
}

// achievedPrefix returns the leading contiguous run of
// completed/noop results -- the exact achieved prefix RunApply journals
// (CONTEXT D-21).
func achievedPrefix(results []OperationResult) []OperationResult {
	prefix := make([]OperationResult, 0, len(results))
	for _, result := range results {
		if result.Status != StatusCompleted && result.Status != StatusNoop {
			break
		}
		prefix = append(prefix, result)
	}
	return prefix
}

// Apply attempts every operation in plan.Operations against client, in
// plan order, applying the D-17 read-before-write/one-mutation/readback
// discipline per operation (CONTEXT D-17/D-21). It performs no plan-level
// guard checks itself: callers must already have run
// ValidatePlanIntegrity, ValidatePlanFreshness, and
// GuardAgainstPullRequestMutation (guard.go), so a stale or
// pull_request-triggered apply attempt never reaches this function at
// all. As soon as any operation is not a clean completed/noop outcome,
// Apply stops attempting further operations; the returned results are
// always an exact contiguous prefix of clean successes followed by
// exactly one stopping entry and any number of untouched "stopped"
// pending entries -- the exact shape journal.go's achieved-prefix resume
// depends on.
func Apply(client RemoteClient, plan reconcile.Plan, mappings []catalog.RemoteMapping) []OperationResult {
	blocked := classifyBlocked(plan.Operations, mappings)
	return applyOperations(client, plan.Operations, blocked)
}

// RunApply is the full exact-plan apply orchestration (CONTEXT
// D-16/D-17/D-18/D-21): it validates the plan's own hash binding
// (ValidatePlanIntegrity), rejects it if relevant repository or Linear
// state changed since the preview was produced (ValidatePlanFreshness),
// refuses to run at all from a pull_request-triggered CI event
// (GuardAgainstPullRequestMutation), resumes only the exact
// already-achieved prefix recorded in journal (ResumePrefix), and then
// attempts exactly the remaining operations (Apply's per-operation
// engine). It never persists anything itself -- the caller commits the
// returned Report and Journal together, atomically, through
// CommitResultAtomically.
func RunApply(
	client RemoteClient,
	plan reconcile.Plan,
	intents []reconcile.Intent,
	mappings []catalog.RemoteMapping,
	snapshot transport.Snapshot,
	baselines []reconcile.SyncBaseline,
	journal *Journal,
	lookupEnv func(string) (string, bool),
) (Report, Journal, error) {
	if err := ValidatePlanIntegrity(plan); err != nil {
		return Report{}, Journal{}, err
	}
	if err := ValidatePlanFreshness(plan, intents, mappings, snapshot, baselines); err != nil {
		return Report{}, Journal{}, err
	}
	if err := GuardAgainstPullRequestMutation(lookupEnv); err != nil {
		return Report{}, Journal{}, err
	}

	achieved, remaining, err := ResumePrefix(plan, journal, client)
	if err != nil {
		return Report{}, Journal{}, err
	}

	blocked := classifyBlocked(plan.Operations, mappings)
	fresh := applyOperations(client, remaining, blocked)

	allResults := make([]OperationResult, 0, len(achieved)+len(fresh))
	allResults = append(allResults, achieved...)
	allResults = append(allResults, fresh...)

	report := Report{PlanID: plan.PlanID, Results: allResults}
	newJournal := Journal{PlanID: plan.PlanID, Results: achievedPrefix(allResults)}
	return report, newJournal, nil
}
