// impact.go implements the deterministic pool impact-review engine
// (CONTEXT POOL-03/D-11): BuildImpactPlan walks every dependent category
// the show model currently contains -- deployment instances and
// cross-pool groups -- for one pool add/remove ImpactRequest, and
// auto-proposes a next-free universe/address (deployment.NextFreeAddress)
// for each newly proposed instance. The walk iterates the caller's own
// pools/deployments/groups slices in their existing (already
// deterministic) order, so it is structured as an ordered traversal over
// a fixed dependent-category list: a future phase's dependent types
// (themes/scenes/chases/...) extend the walk without a rewrite.
//
// BuildImpactPlan/Apply take plain pool/deployment/group slices plus a
// revision int rather than a show.State value: internal/show already
// imports internal/pool (State embeds []pool.Pool/[]pool.Group), so this
// package cannot import internal/show without an import cycle. Callers
// that hold a show.State (internal/command/pool.go) pass its fields
// through directly; see 02-05-SUMMARY.md's "Deviations" for this
// adaptation from the plan's literal show.State signature.
//
// plan_id binds the exact request plus the computed dependent walk via
// sha256(strictjson.CanonicalEncode(planBody)) -- freshly-minted
// identifiers (a new PoolMember/Instance UUID) are never part of the
// hashed body, so identical inputs always produce an identical plan_id
// (mirrors internal/trace/reconcile.PlanID's binding).
package pool

import (
	"fmt"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/deployment"
	"github.com/lnorton89/golc/internal/strictjson"
	"github.com/lnorton89/golc/internal/trace/apply"
	"github.com/lnorton89/golc/internal/trace/reconcile"
)

// ImpactSchemaVersion is the current ImpactPlan schema version
// ValidatePlanIntegrity checks every plan against.
const ImpactSchemaVersion = 1

// defaultInstanceChannelCount is the channel width every proposed
// deployment.NextFreeAddress call uses. internal/fixture's Mode does not
// yet declare a per-mode channel count (CONTEXT: see
// deployment.NextFreeAddress's own doc comment -- "Instance does not yet
// carry its own channel width, a future plan's concern"); until that
// model gap closes, every proposed instance is conservatively treated as
// occupying exactly one channel.
//
// KNOWN GAP (tracked as a follow-up, not yet fixed): NextFreeAddress's
// own over-conservative-never-under-conservative guarantee only holds
// when channelCount matches every existing instance's *real* channel
// width. Because this constant is 1 and almost no real DMX fixture this
// phase targets (RGB PARs, moving-head spot/wash) is actually 1-channel
// wide, two wider fixtures can still be packed one address apart here --
// a genuine address collision once their real channel span is
// considered. Closing this gap requires threading a per-fixture,
// capability-derived channel count through PoolMemberSpec/ImpactRequest
// into NextFreeAddress instead of this hardcoded constant; until that
// lands, callers must not treat BuildImpactPlan's proposed addresses as
// collision-safe for multi-channel fixtures.
const defaultInstanceChannelCount = 1

// PoolMemberSpec is one fixture reference an ImpactRequest proposes
// adding to a pool as a new PoolMember (mirrors NewPoolMember's inputs),
// plus the Mode its dependent deployment Instance proposals carry.
type PoolMemberSpec struct {
	FixtureStableKey   string `json:"fixture_stable_key"`
	FixtureContentHash string `json:"fixture_content_hash"`
	Mode               string `json:"mode"`
}

// ImpactRequest is one pool add/remove request BuildImpactPlan reviews
// (CONTEXT POOL-03/POOL-04). Propagate is the resolved per-update
// propagation mode ("immediate" or "preview"); BuildImpactPlan itself
// never rejects any other value -- internal/command/pool.go resolves and
// validates the default/override before calling BuildImpactPlan.
type ImpactRequest struct {
	PoolID    uuid.UUID        `json:"pool_id"`
	Add       []PoolMemberSpec `json:"add,omitempty"`
	Remove    []uuid.UUID      `json:"remove,omitempty"`
	Propagate string           `json:"propagate"`
}

// ImpactOp is one planned effect on a single dependent (a deployment
// instance or a group membership) BuildImpactPlan's walk discovered.
// Status reuses internal/trace/apply's OperationStatus vocabulary;
// BuildImpactPlan only ever assigns StatusPending (every proposed op is
// unapplied by construction) -- StatusCompleted/StatusNoop/StatusBlocked
// are reserved for a future apply-time report, matching
// apply.OperationResult's own split between plan-time and apply-time
// status. PoolMemberIndex is meaningful only when Action is "add" (it
// indexes ImpactPlan.Add, since the new PoolMember itself does not exist,
// and so has no ID, until Apply mints it); PoolMemberID is meaningful
// only when Action is "remove" (the existing member being removed).
type ImpactOp struct {
	DependentKind    string                `json:"dependent_kind"`
	DependentRef     string                `json:"dependent_ref"`
	DependentID      uuid.UUID             `json:"dependent_id"`
	Action           string                `json:"action"`
	PoolMemberIndex  int                   `json:"pool_member_index"`
	PoolMemberID     uuid.UUID             `json:"pool_member_id"`
	ProposedUniverse int                   `json:"proposed_universe,omitempty"`
	ProposedAddress  int                   `json:"proposed_address,omitempty"`
	Status           apply.OperationStatus `json:"status"`
}

// Warning is one non-fatal, review-only finding attached to an
// ImpactPlan.
type Warning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Error is one fatal, review-only finding attached to an ImpactPlan. A
// plan carrying any Error is still well-formed and reviewable, but
// Apply refuses to apply it.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ImpactPlan is the complete deterministic D-11/POOL-03 impact review:
// the exact reviewed request plus every dependent-category effect it
// discovered, ready for review before any apply. PlanID binds the
// canonical body (every field below except PlanID) via SHA-256, so an
// identical request against an identical show model always produces a
// byte-identical plan.
type ImpactPlan struct {
	SchemaVersion    int              `json:"schema_version"`
	PoolID           uuid.UUID        `json:"pool_id"`
	Add              []PoolMemberSpec `json:"add,omitempty"`
	Remove           []uuid.UUID      `json:"remove,omitempty"`
	Propagate        string           `json:"propagate"`
	ExpectedRevision int              `json:"expected_revision"`
	Operations       []ImpactOp       `json:"operations"`
	Warnings         []Warning        `json:"warnings,omitempty"`
	Errors           []Error          `json:"errors,omitempty"`
	PlanID           string           `json:"plan_id"`
}

// planBody is the exact byte-hashed subset of ImpactPlan: it excludes
// PlanID itself, since plan_id = sha256(canonical_body), and it carries
// no freshly-minted identifier, timestamp, or random value, so identical
// inputs always hash identically (mirrors internal/trace/reconcile's
// unexported planBody).
type planBody struct {
	SchemaVersion    int              `json:"schema_version"`
	PoolID           uuid.UUID        `json:"pool_id"`
	Add              []PoolMemberSpec `json:"add,omitempty"`
	Remove           []uuid.UUID      `json:"remove,omitempty"`
	Propagate        string           `json:"propagate"`
	ExpectedRevision int              `json:"expected_revision"`
	Operations       []ImpactOp       `json:"operations"`
	Warnings         []Warning        `json:"warnings,omitempty"`
	Errors           []Error          `json:"errors,omitempty"`
}

func bodyOf(plan ImpactPlan) planBody {
	return planBody{
		SchemaVersion:    plan.SchemaVersion,
		PoolID:           plan.PoolID,
		Add:              plan.Add,
		Remove:           plan.Remove,
		Propagate:        plan.Propagate,
		ExpectedRevision: plan.ExpectedRevision,
		Operations:       plan.Operations,
		Warnings:         plan.Warnings,
		Errors:           plan.Errors,
	}
}

// computePlanID computes plan_id = sha256(canonical_body) from body,
// reusing internal/trace/reconcile.DigestBytes so every plan_id binding
// in this codebase (Linear reconciliation plans and pool impact plans
// alike) shares one hash implementation.
func computePlanID(body planBody) (string, error) {
	encoded, err := strictjson.CanonicalEncode(body)
	if err != nil {
		return "", fmt.Errorf("GOLC_POOL_PLAN_ENCODE: %v", err)
	}
	return reconcile.DigestBytes(encoded), nil
}

// findPool returns the pool in pools whose ID matches id.
func findPool(pools []Pool, id uuid.UUID) (Pool, bool) {
	for _, p := range pools {
		if p.ID == id {
			return p, true
		}
	}
	return Pool{}, false
}

// deploymentUsesPool reports whether d already carries at least one
// Instance referencing poolID: only a deployment that has already
// adopted the pool is a "dependent" a newly added member proposes an
// instance against (CONTEXT: an empty pool, or a pool with no
// dependents, must yield zero dependent operations, not an error).
func deploymentUsesPool(d deployment.Deployment, poolID uuid.UUID) bool {
	for _, instance := range d.Instances {
		if instance.PoolID == poolID {
			return true
		}
	}
	return false
}

// BuildImpactPlan computes a deterministic ImpactPlan for req against the
// given show model (pools/deployments/groups/revision): every removed
// member ID must already exist in the target pool, every added fixture
// spec proposes one new deployment.Instance (via deployment.NextFreeAddress)
// in each deployment that has already adopted the pool, and every removed
// member's existing deployment instances and group member refs are
// discovered for removal. Neither BuildImpactPlan nor its returned plan
// ever mints a PoolMember/Instance UUID -- that happens only inside Apply,
// at apply time, so the hashed plan body never has to contain a
// not-yet-existing identifier (CONTEXT: identical inputs always produce
// an identical plan_id).
func BuildImpactPlan(pools []Pool, deployments []deployment.Deployment, groups []Group, revision int, req ImpactRequest) (ImpactPlan, error) {
	targetPool, found := findPool(pools, req.PoolID)
	if !found {
		return ImpactPlan{}, fmt.Errorf("GOLC_POOL_PLAN_UNKNOWN_POOL: pool %s does not exist in the current show state", req.PoolID)
	}

	memberByID := make(map[uuid.UUID]bool, len(targetPool.Members))
	for _, m := range targetPool.Members {
		memberByID[m.ID] = true
	}
	removeIDs := append([]uuid.UUID(nil), req.Remove...)
	for _, id := range removeIDs {
		if !memberByID[id] {
			return ImpactPlan{}, fmt.Errorf("GOLC_POOL_PLAN_UNKNOWN_MEMBER: pool member %s is not a member of pool %s", id, req.PoolID)
		}
	}
	removeSet := make(map[uuid.UUID]bool, len(removeIDs))
	for _, id := range removeIDs {
		removeSet[id] = true
	}

	adds := append([]PoolMemberSpec(nil), req.Add...)

	var operations []ImpactOp

	// Dependent category 1: deployment instances an add proposes.
	// proposedByDeployment seeds NextFreeAddress's overlap search with
	// each deployment's real instances plus every pseudo-instance
	// proposed so far in this same build, so multiple Add specs in one
	// request never collide with each other.
	proposedByDeployment := make(map[uuid.UUID][]deployment.Instance, len(deployments))
	for _, d := range deployments {
		proposedByDeployment[d.ID] = append([]deployment.Instance(nil), d.Instances...)
	}
	for addIndex, spec := range adds {
		_ = spec
		for _, d := range deployments {
			if !deploymentUsesPool(d, req.PoolID) {
				continue
			}
			universe, address, err := deployment.NextFreeAddress(proposedByDeployment[d.ID], defaultInstanceChannelCount)
			if err != nil {
				return ImpactPlan{}, fmt.Errorf("GOLC_POOL_PLAN_ADDRESS_EXHAUSTED: deployment %q: %v", d.Name, err)
			}
			proposedByDeployment[d.ID] = append(proposedByDeployment[d.ID], deployment.Instance{Universe: universe, Address: address})
			operations = append(operations, ImpactOp{
				DependentKind:    "deployment_instance",
				DependentRef:     d.Name,
				DependentID:      d.ID,
				Action:           "add",
				PoolMemberIndex:  addIndex,
				ProposedUniverse: universe,
				ProposedAddress:  address,
				Status:           apply.StatusPending,
			})
		}
	}

	// Dependent category 2: existing deployment instances referencing a
	// removed pool member.
	for _, d := range deployments {
		for _, instance := range d.Instances {
			if instance.PoolID == req.PoolID && removeSet[instance.PoolMemberID] {
				operations = append(operations, ImpactOp{
					DependentKind: "deployment_instance",
					DependentRef:  d.Name,
					DependentID:   d.ID,
					Action:        "remove",
					PoolMemberID:  instance.PoolMemberID,
					Status:        apply.StatusPending,
				})
			}
		}
	}

	// Dependent category 3: existing group member refs referencing a
	// removed pool member.
	for _, g := range groups {
		for _, ref := range g.MemberRefs {
			if ref.PoolID == req.PoolID && removeSet[ref.PoolMemberID] {
				operations = append(operations, ImpactOp{
					DependentKind: "group_member",
					DependentRef:  g.Name,
					DependentID:   g.ID,
					Action:        "remove",
					PoolMemberID:  ref.PoolMemberID,
					Status:        apply.StatusPending,
				})
			}
		}
	}

	plan := ImpactPlan{
		SchemaVersion:    ImpactSchemaVersion,
		PoolID:           req.PoolID,
		Add:              adds,
		Remove:           removeIDs,
		Propagate:        req.Propagate,
		ExpectedRevision: revision,
		Operations:       operations,
	}
	id, err := computePlanID(bodyOf(plan))
	if err != nil {
		return ImpactPlan{}, err
	}
	plan.PlanID = id
	return plan, nil
}
