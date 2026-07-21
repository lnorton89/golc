// guard.go implements the two independent gates every apply must pass
// before any mutation is attempted (CONTEXT D-16/D-17/D-18):
// ValidatePlanIntegrity/ValidatePlanFreshness reject a plan that was
// tampered with or that no longer matches current repository/Linear
// state, and GuardAgainstPullRequestMutation refuses to run at all from a
// pull_request-triggered CI event -- independent of whatever the calling
// workflow YAML does or does not enforce, since a Go-level check cannot
// be bypassed by a misconfigured or compromised workflow file. ApplyRemoval
// is the only function in this package that can ever produce an archive
// or unlink mutation (CONTEXT D-15): the regular create/update Apply path
// (engine.go) has no code path that can express removal at all.
package apply

import (
	"fmt"

	"github.com/lnorton89/golc/internal/strictjson"
	"github.com/lnorton89/golc/internal/trace/catalog"
	"github.com/lnorton89/golc/internal/trace/reconcile"
	"github.com/lnorton89/golc/internal/trace/transport"
)

// pullRequestEventEnvVar is the exact environment variable GitHub Actions
// sets to "pull_request" for pull_request-triggered workflow runs.
const pullRequestEventEnvVar = "GITHUB_EVENT_NAME"

// pullRequestEventValue is the exact value that identifies a
// pull_request-triggered CI event.
const pullRequestEventValue = "pull_request"

// GuardAgainstPullRequestMutation blocks apply when running from a
// pull_request-triggered CI event (CONTEXT D-16: PR CI performs read-only
// drift checks and must never mutate Linear). lookup is typically
// os.LookupEnv; injecting it keeps this guard testable without mutating
// real process-global environment state, and it is applied independently
// of whatever the calling workflow YAML does or does not enforce.
func GuardAgainstPullRequestMutation(lookup func(string) (string, bool)) error {
	if lookup == nil {
		return nil
	}
	if value, ok := lookup(pullRequestEventEnvVar); ok && value == pullRequestEventValue {
		return fmt.Errorf("GOLC_APPLY_PR_BLOCKED: mutating apply is never permitted from a pull_request-triggered CI event (CONTEXT D-16)")
	}
	return nil
}

// planBodyMirror exactly mirrors reconcile's unexported planBody shape
// (same JSON field names and order) so this package can independently
// recompute a plan's canonical hash binding without reconcile exporting
// its private body type.
type planBodyMirror struct {
	SchemaVersion     int                   `json:"schema_version"`
	IntentDigest      string                `json:"intent_digest"`
	MappingDigest     string                `json:"mapping_digest"`
	RemoteScopeDigest string                `json:"remote_scope_digest"`
	Operations        []reconcile.Operation `json:"operations"`
	Conflicts         []reconcile.Conflict  `json:"conflicts"`
}

// recomputePlanID recomputes plan_id = sha256(canonical_body) from plan's
// own fields, exactly mirroring reconcile.PlanID's binding.
func recomputePlanID(plan reconcile.Plan) (string, error) {
	body := planBodyMirror{
		SchemaVersion:     plan.SchemaVersion,
		IntentDigest:      plan.IntentDigest,
		MappingDigest:     plan.MappingDigest,
		RemoteScopeDigest: plan.RemoteScopeDigest,
		Operations:        plan.Operations,
		Conflicts:         plan.Conflicts,
	}
	encoded, err := strictjson.CanonicalEncode(body)
	if err != nil {
		return "", fmt.Errorf("GOLC_APPLY_PLAN_HASH: %v", err)
	}
	return reconcile.PlanID(encoded), nil
}

// ValidatePlanIntegrity rejects plan outright before ValidatePlanFreshness
// or any mutation is attempted (CONTEXT D-17/D-18): its schema_version
// must match the current reconcile contract, and its own recorded
// plan_id must match the SHA-256 binding recomputed from its own bytes --
// a plan edited after being hashed, or hand-forged with an arbitrary
// plan_id, fails here before anything else runs.
func ValidatePlanIntegrity(plan reconcile.Plan) error {
	if plan.SchemaVersion != reconcile.SchemaVersion {
		return fmt.Errorf("GOLC_APPLY_PLAN_SCHEMA: plan schema_version %d does not match expected %d", plan.SchemaVersion, reconcile.SchemaVersion)
	}
	recomputed, err := recomputePlanID(plan)
	if err != nil {
		return err
	}
	if recomputed != plan.PlanID {
		return fmt.Errorf("GOLC_APPLY_PLAN_HASH: plan_id %q does not match its own recomputed canonical hash %q; the plan bytes were altered after hashing", plan.PlanID, recomputed)
	}
	return nil
}

// ValidatePlanFreshness rejects plan if recomputing the exact same D-17
// complete-snapshot preview from the given current repository intent,
// remote mapping set, transport snapshot, and sync baselines no longer
// produces a byte-identical plan_id (CONTEXT D-18): repository or Linear
// state that changed after the preview was produced, a newly discovered
// or newly resolved D-13 conflict, and a snapshot that has become
// incomplete or ambiguous are all caught this way, before any mutation is
// attempted.
func ValidatePlanFreshness(plan reconcile.Plan, intents []reconcile.Intent, mappings []catalog.RemoteMapping, snapshot transport.Snapshot, baselines []reconcile.SyncBaseline) error {
	fresh, err := reconcile.BuildCompletePreview(intents, mappings, snapshot, baselines)
	if err != nil {
		return fmt.Errorf("GOLC_APPLY_PLAN_STALE: recomputing the current preview failed: %v", err)
	}
	if fresh.PlanID != plan.PlanID {
		return fmt.Errorf("GOLC_APPLY_PLAN_STALE: plan %s no longer matches current repository/remote state (recomputed %s); re-run linear preview", plan.PlanID, fresh.PlanID)
	}
	return nil
}

// ApplyRemoval performs one explicit, already-reviewed D-15 archive or
// unlink mutation through the existing transport.Transport contract
// (CONTEXT D-15/D-16). It is the only function in this package that can
// ever produce a MutationArchive or MutationUnlink call -- removal is
// never a side effect of a normal create/update plan apply -- and it
// enforces the exact same pull-request guard as the regular Apply path.
func ApplyRemoval(client transport.Transport, preview reconcile.ArchivePreview, lookup func(string) (string, bool)) (transport.Mutation, error) {
	if err := GuardAgainstPullRequestMutation(lookup); err != nil {
		return transport.Mutation{}, err
	}
	if preview.LinearUUID == nil {
		return transport.Mutation{}, fmt.Errorf("GOLC_APPLY_REMOVAL_UNMAPPED: %s has no linked Linear object to %s", preview.LocalID, preview.Action)
	}
	var kind transport.MutationKind
	switch preview.Action {
	case "archive":
		kind = transport.MutationArchive
	case "unlink":
		kind = transport.MutationUnlink
	default:
		return transport.Mutation{}, fmt.Errorf("GOLC_APPLY_REMOVAL_ACTION_UNKNOWN: %q is not archive or unlink", preview.Action)
	}
	return client.Apply(transport.Mutation{Kind: kind, LocalID: preview.LocalID, LinearUUID: *preview.LinearUUID})
}
