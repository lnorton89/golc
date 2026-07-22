// plan.go implements POOL-06/POOL-07/POOL-08's semantic fixture
// substitution (CONTEXT D-14/D-16): BuildSubstitutionPlan diffs two
// canonical internal/fixture.FixtureDefinition capability sets --
// comparing by fixture.CapabilityType (semantic), never by raw DMX channel
// position (D-08) -- and emits the exact same internal/pool.ImpactPlan
// shape a pool update produces, reusing pool.BuildImpactPlan's
// Add/Remove-based dependent walk (deployment instances/groups) and
// pool.ValidatePlanIntegrity/ValidatePlanFreshness/Apply's proven
// integrity/freshness/atomic-apply gates verbatim (D-16: one mechanism,
// not a second).
//
// A substitution's PoolMemberSpec Add/Remove pair is derived from the
// existing pool member(s) whose FixtureStableKey matches
// SubstitutionRequest.FromFixtureRef: those members are removed, and one
// new member pinned to the target fixture's own content-addressed
// identity (internal/fixture.Pin) is added per removed member -- a
// one-to-one model replacement, never a per-item partial substitution
// (D-13: revise means re-running with a different target, never editing a
// computed plan).
//
// Every capability gap the diff finds is classified by the D-14 severity
// taxonomy (missing/incompatible/unsupported, a GOLC-original design --
// neither OFL nor GDTF define a comparable field, per
// 02-RESEARCH.md Common Pitfalls #2) and carried as a pool.Warning: never
// silently resolved, approximated, or dropped (POOL-07). A structurally
// invalid target fixture is instead a hard-blocking pool.Error
// (GOLC_SUBSTITUTION_TARGET_INVALID, T-02-14) -- distinct from the
// accept-past warning list, exactly like pool.Apply already refuses any
// plan carrying an Error regardless of its Warnings.
package substitution

import (
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/fixture"
	"github.com/lnorton89/golc/internal/pool"
	"github.com/lnorton89/golc/internal/show"
)

// CapabilityGapSeverity names one D-14 severity class a capability-diff gap
// falls into. All three are accept-past warnings, never a silent
// resolution and never (on their own) a hard block -- only a structural
// target-fixture problem hard-blocks (see CapabilityGap's doc comment).
type CapabilityGapSeverity string

const (
	// SeverityMissing marks a capability type the source fixture declares
	// that the target fixture does not represent at all.
	SeverityMissing CapabilityGapSeverity = "missing"
	// SeverityIncompatible marks a capability type present in both
	// fixtures whose value range the target cannot fully reproduce (the
	// target's declared range(s) do not cover the source's declared
	// range(s)).
	SeverityIncompatible CapabilityGapSeverity = "incompatible"
	// SeverityUnsupported marks a capability type present in both
	// fixtures, with the target's range(s) fully covering the source's
	// range(s), but where the source declares more distinct sub-ranges
	// (finer-grained behavior, for example a shutter's separate closed and
	// strobe sub-ranges) than the target can represent.
	SeverityUnsupported CapabilityGapSeverity = "unsupported"
)

// CapabilityGap names one specific capability-diff finding: a source
// capability type that the target fixture cannot fully represent, at the
// given severity. CapabilityGap is carried through the returned
// pool.ImpactPlan as a pool.Warning (Code names the severity, Message
// carries the capability type and Detail) since pool.Warning itself has no
// structured field for it -- reusing pool's exact Warning shape rather
// than introducing a parallel one (D-16).
type CapabilityGap struct {
	Severity       CapabilityGapSeverity  `json:"severity"`
	CapabilityType fixture.CapabilityType `json:"capability_type"`
	Detail         string                 `json:"detail"`
}

// SubstitutionRequest is one fixture-substitution review request
// (POOL-06/POOL-07/POOL-08): replace every existing pool member of PoolID
// pinned to FromFixtureRef (a fixture stable key, internal/fixture.
// Identity.StableKey shape: "<manufacturer>/<model>") with a new member
// pinned to the target fixture's own computed identity. ToFixtureRef
// documents the caller's intended target for review/audit; the actual
// PoolMemberSpec added to the plan is always derived from the target
// fixture.FixtureDefinition's own internal/fixture.Pin identity, never
// trusted verbatim from this field, so a caller can never force a plan to
// carry a fixture reference inconsistent with the target's real content.
type SubstitutionRequest struct {
	PoolID         uuid.UUID `json:"pool_id"`
	FromFixtureRef string    `json:"from_fixture_ref"`
	ToFixtureRef   string    `json:"to_fixture_ref"`
}

// capabilityWarningCode derives the pool.Warning Code for a capability gap
// at the given severity, following the repo-wide GOLC_{DOMAIN}_{CONDITION}
// diagnostic naming convention.
func capabilityWarningCode(severity CapabilityGapSeverity) string {
	return "GOLC_SUBSTITUTION_CAPABILITY_" + strings.ToUpper(string(severity))
}

// groupRangesByType groups capabilities' declared [0,1] ranges by their
// CapabilityType, preserving declared order within each type.
func groupRangesByType(capabilities []fixture.Capability) map[fixture.CapabilityType][][2]float64 {
	grouped := make(map[fixture.CapabilityType][][2]float64, len(capabilities))
	for _, c := range capabilities {
		grouped[c.Type] = append(grouped[c.Type], c.Range)
	}
	return grouped
}

// mergeRanges returns the sorted, non-overlapping union of ranges (two
// exactly-touching or overlapping ranges merge into one covering span).
func mergeRanges(ranges [][2]float64) [][2]float64 {
	sorted := append([][2]float64(nil), ranges...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i][0] < sorted[j][0] })
	merged := make([][2]float64, 0, len(sorted))
	for _, r := range sorted {
		if len(merged) > 0 && r[0] <= merged[len(merged)-1][1] {
			if r[1] > merged[len(merged)-1][1] {
				merged[len(merged)-1][1] = r[1]
			}
			continue
		}
		merged = append(merged, r)
	}
	return merged
}

// containedInMerged reports whether r falls entirely within one span of
// merged (merged is already the non-overlapping union mergeRanges
// returns).
func containedInMerged(r [2]float64, merged [][2]float64) bool {
	for _, m := range merged {
		if r[0] >= m[0] && r[1] <= m[1] {
			return true
		}
	}
	return false
}

// rangesCovered reports whether every range in fromRanges falls entirely
// within the merged union of toRanges -- "the target can reproduce every
// value the source's declared range(s) could produce."
func rangesCovered(fromRanges, toRanges [][2]float64) bool {
	mergedTo := mergeRanges(toRanges)
	for _, r := range fromRanges {
		if !containedInMerged(r, mergedTo) {
			return false
		}
	}
	return true
}

// diffCapabilities walks from's capability types (in the declared
// fixture.SupportedCapabilityTypes order, so the result is deterministic
// regardless of either fixture's own declaration order) and classifies
// every gap the target cannot fully represent, per the D-14 taxonomy:
// missing (target has none of this type), incompatible (target has this
// type but its range(s) do not cover from's), or unsupported (target's
// range(s) cover from's, but from declares more distinct sub-ranges than
// target can preserve). A capability type from does not declare is never
// diffed -- gaining a capability the source never had is not a gap.
func diffCapabilities(from, to fixture.FixtureDefinition) []CapabilityGap {
	fromByType := groupRangesByType(from.Capabilities)
	toByType := groupRangesByType(to.Capabilities)

	var gaps []CapabilityGap
	for _, capType := range fixture.SupportedCapabilityTypes {
		fromRanges, hasFrom := fromByType[capType]
		if !hasFrom {
			continue
		}
		toRanges, hasTo := toByType[capType]
		if !hasTo {
			gaps = append(gaps, CapabilityGap{
				Severity:       SeverityMissing,
				CapabilityType: capType,
				Detail: fmt.Sprintf(
					"%s capability declared by %s %s has no counterpart in %s %s",
					capType, from.Manufacturer, from.Model, to.Manufacturer, to.Model),
			})
			continue
		}
		if !rangesCovered(fromRanges, toRanges) {
			gaps = append(gaps, CapabilityGap{
				Severity:       SeverityIncompatible,
				CapabilityType: capType,
				Detail: fmt.Sprintf(
					"%s capability range(s) %v declared by %s %s are not fully reproducible by %s %s's %s range(s) %v",
					capType, fromRanges, from.Manufacturer, from.Model, to.Manufacturer, to.Model, capType, toRanges),
			})
			continue
		}
		if len(fromRanges) > len(toRanges) {
			gaps = append(gaps, CapabilityGap{
				Severity:       SeverityUnsupported,
				CapabilityType: capType,
				Detail: fmt.Sprintf(
					"%s capability's %d distinct sub-range(s) declared by %s %s cannot be fully represented by %s %s's %d sub-range(s)",
					capType, len(fromRanges), from.Manufacturer, from.Model, to.Manufacturer, to.Model, len(toRanges)),
			})
		}
	}
	return gaps
}

// poolByID returns the pool in pools whose ID matches id.
func poolByID(pools []pool.Pool, id uuid.UUID) (pool.Pool, bool) {
	for _, p := range pools {
		if p.ID == id {
			return p, true
		}
	}
	return pool.Pool{}, false
}

// modeFor returns def's first declared Mode name, or "" if def declares no
// modes -- the Mode a new deployment.Instance proposal (via
// pool.BuildImpactPlan's Add-driven walk) is stamped with for the
// substituted fixture.
func modeFor(def fixture.FixtureDefinition) string {
	if len(def.Modes) > 0 {
		return def.Modes[0].Name
	}
	return ""
}

// BuildSubstitutionPlan computes a deterministic capability-diff
// substitution review (POOL-06/POOL-07/POOL-08) for req against state:
// every existing member of the target pool pinned to req.FromFixtureRef is
// replaced one-for-one by a new member pinned to to's own computed
// identity (internal/fixture.Pin), producing the exact ImpactOps a
// pool.BuildImpactPlan Add+Remove request would (deployment instances and
// group member refs affected by the swap, D-16), with the capability-diff
// gaps between from and to attached as pool.Warnings (D-14, never silently
// resolved) and any structural target-fixture problem attached as a
// pool.Error (GOLC_SUBSTITUTION_TARGET_INVALID, hard-blocking, distinct
// from the warnings). The returned plan_id binds the full reviewed body --
// including the capability-diff Warnings/Errors, which pool.BuildImpactPlan
// itself never produces -- via pool.RecomputePlanID, the exact same
// canonical-hash mechanism pool.ValidatePlanIntegrity itself uses, so the
// returned plan remains integrity/freshness-checkable by the unmodified
// pool gates.
func BuildSubstitutionPlan(state show.State, from, to fixture.FixtureDefinition, req SubstitutionRequest) (pool.ImpactPlan, error) {
	targetPool, found := poolByID(state.Pools, req.PoolID)
	if !found {
		return pool.ImpactPlan{}, fmt.Errorf("GOLC_SUBSTITUTION_POOL_NOT_FOUND: pool %s does not exist in the current show state", req.PoolID)
	}

	var removeIDs []uuid.UUID
	for _, m := range targetPool.Members {
		if m.FixtureStableKey == req.FromFixtureRef {
			removeIDs = append(removeIDs, m.ID)
		}
	}
	if len(removeIDs) == 0 {
		return pool.ImpactPlan{}, fmt.Errorf(
			"GOLC_SUBSTITUTION_SOURCE_NOT_FOUND: no pool member in pool %s matches fixture ref %q", req.PoolID, req.FromFixtureRef)
	}

	toIdentity, err := fixture.Pin(to)
	if err != nil {
		return pool.ImpactPlan{}, fmt.Errorf("GOLC_SUBSTITUTION_TARGET_INVALID: %v", err)
	}

	adds := make([]pool.PoolMemberSpec, len(removeIDs))
	for i := range removeIDs {
		adds[i] = pool.PoolMemberSpec{
			FixtureStableKey:   toIdentity.StableKey,
			FixtureContentHash: toIdentity.ContentHash,
			Mode:               modeFor(to),
		}
	}

	implReq := pool.ImpactRequest{
		PoolID:    req.PoolID,
		Add:       adds,
		Remove:    removeIDs,
		Propagate: "preview",
	}
	plan, err := pool.BuildImpactPlan(state.Pools, state.Deployments, state.Groups, state.Revision, implReq)
	if err != nil {
		return pool.ImpactPlan{}, err
	}

	gaps := diffCapabilities(from, to)
	warnings := make([]pool.Warning, 0, len(gaps))
	for _, gap := range gaps {
		warnings = append(warnings, pool.Warning{
			Code:    capabilityWarningCode(gap.Severity),
			Message: fmt.Sprintf("%s: %s", gap.CapabilityType, gap.Detail),
		})
	}

	var errs []pool.Error
	if validationErr := fixture.Validate(to); validationErr != nil {
		errs = append(errs, pool.Error{
			Code:    "GOLC_SUBSTITUTION_TARGET_INVALID",
			Message: validationErr.Error(),
		})
	}

	plan.Warnings = warnings
	plan.Errors = errs

	planID, err := pool.RecomputePlanID(plan)
	if err != nil {
		return pool.ImpactPlan{}, err
	}
	plan.PlanID = planID

	return plan, nil
}
