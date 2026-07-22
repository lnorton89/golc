// plan_test.go proves BuildSubstitutionPlan's capability-diff severity
// taxonomy and its reuse of the pool.ImpactPlan integrity/freshness/
// atomic-apply contract (02-06-PLAN.md, Task 1: POOL-06/POOL-07/POOL-08,
// D-14/D-16): a target fixture missing a capability the source declares
// yields a "missing" gap, a shared capability type with an incompatible
// value range yields "incompatible", a shared capability type whose
// distinct sub-ranges the target cannot preserve yields "unsupported",
// every gap always surfaces (never silently resolved), a structurally
// invalid target hard-blocks separately from the accept-past warnings, and
// the resulting plan validates/applies through internal/pool's own
// ValidatePlanIntegrity/ValidatePlanFreshness/Apply gates unmodified in
// call shape.
//
// This file fails to compile until internal/substitution exists (Task 2)
// -- that is the RED state this task proves.
package substitution_test

import (
	"strings"
	"testing"

	"github.com/lnorton89/golc/internal/deployment"
	"github.com/lnorton89/golc/internal/fixture"
	"github.com/lnorton89/golc/internal/pool"
	"github.com/lnorton89/golc/internal/show"
	"github.com/lnorton89/golc/internal/substitution"
)

// fromFixture builds a minimal, otherwise-valid "source" fixture (the
// fixture model already patched into the pool) carrying exactly the given
// capabilities.
func fromFixture(caps ...fixture.Capability) fixture.FixtureDefinition {
	return fixture.FixtureDefinition{
		SchemaVersion: 1,
		Manufacturer:  "Acme",
		Model:         "PAR64",
		Modes:         []fixture.Mode{{Name: "Standard"}},
		Capabilities:  caps,
	}
}

// toFixture builds a minimal, otherwise-valid "target" fixture (the
// candidate replacement) carrying exactly the given capabilities.
func toFixture(caps ...fixture.Capability) fixture.FixtureDefinition {
	return fixture.FixtureDefinition{
		SchemaVersion: 1,
		Manufacturer:  "Beta",
		Model:         "Spot300",
		Modes:         []fixture.Mode{{Name: "Standard"}},
		Capabilities:  caps,
	}
}

// substitutionFixture builds a minimal show model reused across this
// file's tests: one pool with one existing member pinned to fromFixture's
// stable key, and one active deployment with one instance already patched
// to that member -- so BuildSubstitutionPlan's reused pool.BuildImpactPlan
// dependent walk has a deployment instance to discover (mirrors
// internal/pool's own newFixtureState fixture shape, 02-05-PLAN.md).
func substitutionFixture(t *testing.T) (state show.State, target pool.Pool, member pool.PoolMember) {
	t.Helper()

	p, err := pool.NewPool("Wash Pool", nil)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	m, err := pool.NewPoolMember("Acme/PAR64", "sha256:aaaaaaaa")
	if err != nil {
		t.Fatalf("NewPoolMember: %v", err)
	}
	p.Members = append(p.Members, m)

	d, err := deployment.NewDeployment("Venue A")
	if err != nil {
		t.Fatalf("NewDeployment: %v", err)
	}
	d.Active = true
	d.Instances = append(d.Instances, deployment.Instance{
		PoolID:       p.ID,
		PoolMemberID: m.ID,
		Mode:         "Standard",
		Universe:     1,
		Address:      1,
	})

	state = show.State{Pools: []pool.Pool{p}, Deployments: []deployment.Deployment{d}, Revision: 1}
	return state, p, m
}

// hasWarningSeverity reports whether warnings contains an entry whose Code
// names severity (GOLC_SUBSTITUTION_CAPABILITY_{SEVERITY}) and whose
// Message names capabilityType -- the observable shape a CapabilityGap is
// carried through a pool.Warning as (pool.Warning has no structured field
// for it).
func hasWarningSeverity(warnings []pool.Warning, severity, capabilityType string) bool {
	wantCode := "GOLC_SUBSTITUTION_CAPABILITY_" + strings.ToUpper(severity)
	for _, w := range warnings {
		if w.Code == wantCode && strings.Contains(w.Message, capabilityType) {
			return true
		}
	}
	return false
}

func TestCapabilityDiffMissing(t *testing.T) {
	state, target, _ := substitutionFixture(t)
	from := fromFixture(
		fixture.Capability{Type: fixture.CapabilityIntensity, Range: [2]float64{0, 1}},
		fixture.Capability{Type: fixture.CapabilityGobo, Range: [2]float64{0, 1}},
	)
	to := toFixture(
		fixture.Capability{Type: fixture.CapabilityIntensity, Range: [2]float64{0, 1}},
	)
	req := substitution.SubstitutionRequest{PoolID: target.ID, FromFixtureRef: "Acme/PAR64", ToFixtureRef: "Beta/Spot300"}

	plan, err := substitution.BuildSubstitutionPlan(state, from, to, req)
	if err != nil {
		t.Fatalf("BuildSubstitutionPlan: %v", err)
	}
	if !hasWarningSeverity(plan.Warnings, string(substitution.SeverityMissing), string(fixture.CapabilityGobo)) {
		t.Fatalf("expected a missing-severity gobo warning, got %+v", plan.Warnings)
	}
	if len(plan.Errors) != 0 {
		t.Fatalf("expected no structural errors for a valid target, got %+v", plan.Errors)
	}
}

func TestCapabilityDiffIncompatible(t *testing.T) {
	state, target, _ := substitutionFixture(t)
	from := fromFixture(
		fixture.Capability{Type: fixture.CapabilityPan, Range: [2]float64{0, 1}},
	)
	to := toFixture(
		fixture.Capability{Type: fixture.CapabilityPan, Range: [2]float64{0, 0.5}},
	)
	req := substitution.SubstitutionRequest{PoolID: target.ID, FromFixtureRef: "Acme/PAR64", ToFixtureRef: "Beta/Spot300"}

	plan, err := substitution.BuildSubstitutionPlan(state, from, to, req)
	if err != nil {
		t.Fatalf("BuildSubstitutionPlan: %v", err)
	}
	if !hasWarningSeverity(plan.Warnings, string(substitution.SeverityIncompatible), string(fixture.CapabilityPan)) {
		t.Fatalf("expected an incompatible-severity pan warning, got %+v", plan.Warnings)
	}
}

func TestCapabilityDiffUnsupported(t *testing.T) {
	state, target, _ := substitutionFixture(t)
	from := fromFixture(
		fixture.Capability{Type: fixture.CapabilityShutter, Range: [2]float64{0, 0.5}, Comment: "closed"},
		fixture.Capability{Type: fixture.CapabilityShutter, Range: [2]float64{0.5, 1}, Comment: "strobe"},
	)
	to := toFixture(
		fixture.Capability{Type: fixture.CapabilityShutter, Range: [2]float64{0, 1}},
	)
	req := substitution.SubstitutionRequest{PoolID: target.ID, FromFixtureRef: "Acme/PAR64", ToFixtureRef: "Beta/Spot300"}

	plan, err := substitution.BuildSubstitutionPlan(state, from, to, req)
	if err != nil {
		t.Fatalf("BuildSubstitutionPlan: %v", err)
	}
	if !hasWarningSeverity(plan.Warnings, string(substitution.SeverityUnsupported), string(fixture.CapabilityShutter)) {
		t.Fatalf("expected an unsupported-severity shutter warning, got %+v", plan.Warnings)
	}
}

func TestSubstitutionNeverApproximates(t *testing.T) {
	state, target, _ := substitutionFixture(t)
	from := fromFixture(
		fixture.Capability{Type: fixture.CapabilityIntensity, Range: [2]float64{0, 1}},
		fixture.Capability{Type: fixture.CapabilityGobo, Range: [2]float64{0, 1}},
		fixture.Capability{Type: fixture.CapabilityPan, Range: [2]float64{0, 1}},
		fixture.Capability{Type: fixture.CapabilityShutter, Range: [2]float64{0, 0.5}},
		fixture.Capability{Type: fixture.CapabilityShutter, Range: [2]float64{0.5, 1}},
	)
	to := toFixture(
		fixture.Capability{Type: fixture.CapabilityIntensity, Range: [2]float64{0, 1}},
		fixture.Capability{Type: fixture.CapabilityPan, Range: [2]float64{0, 0.5}},
		fixture.Capability{Type: fixture.CapabilityShutter, Range: [2]float64{0, 1}},
	)
	req := substitution.SubstitutionRequest{PoolID: target.ID, FromFixtureRef: "Acme/PAR64", ToFixtureRef: "Beta/Spot300"}

	beforeMemberCount := len(state.Pools[0].Members)

	plan, err := substitution.BuildSubstitutionPlan(state, from, to, req)
	if err != nil {
		t.Fatalf("BuildSubstitutionPlan: %v", err)
	}

	if !hasWarningSeverity(plan.Warnings, string(substitution.SeverityMissing), string(fixture.CapabilityGobo)) {
		t.Errorf("expected the missing gobo gap to surface, got %+v", plan.Warnings)
	}
	if !hasWarningSeverity(plan.Warnings, string(substitution.SeverityIncompatible), string(fixture.CapabilityPan)) {
		t.Errorf("expected the incompatible pan gap to surface, got %+v", plan.Warnings)
	}
	if !hasWarningSeverity(plan.Warnings, string(substitution.SeverityUnsupported), string(fixture.CapabilityShutter)) {
		t.Errorf("expected the unsupported shutter gap to surface, got %+v", plan.Warnings)
	}
	if len(plan.Warnings) != 3 {
		t.Fatalf("expected exactly 3 warnings (one per gap, none dropped, merged, or silently resolved), got %d: %+v", len(plan.Warnings), plan.Warnings)
	}

	// BuildSubstitutionPlan never mutates the input show state: it is a
	// pure computation the author reviews, never an auto-accept.
	if len(state.Pools[0].Members) != beforeMemberCount {
		t.Fatalf("expected BuildSubstitutionPlan to leave the input show state unmutated, got %d members", len(state.Pools[0].Members))
	}
}

func TestSubstitutionStructuralError(t *testing.T) {
	state, target, _ := substitutionFixture(t)
	from := fromFixture(
		fixture.Capability{Type: fixture.CapabilityIntensity, Range: [2]float64{0, 1}},
	)
	// An invalid target: zero declared capabilities fails fixture.Validate
	// (GOLC_FIXTURE_EMPTY) -- a structural problem, distinct from any
	// capability-gap warning.
	to := fixture.FixtureDefinition{
		SchemaVersion: 1,
		Manufacturer:  "Beta",
		Model:         "Spot300",
		Modes:         []fixture.Mode{{Name: "Standard"}},
	}
	req := substitution.SubstitutionRequest{PoolID: target.ID, FromFixtureRef: "Acme/PAR64", ToFixtureRef: "Beta/Spot300"}

	plan, err := substitution.BuildSubstitutionPlan(state, from, to, req)
	if err != nil {
		t.Fatalf("BuildSubstitutionPlan: %v", err)
	}
	if len(plan.Errors) != 1 || plan.Errors[0].Code != "GOLC_SUBSTITUTION_TARGET_INVALID" {
		t.Fatalf("expected exactly one GOLC_SUBSTITUTION_TARGET_INVALID error, got %+v", plan.Errors)
	}

	// The structural error hard-blocks apply distinctly from the
	// accept-past warning list (D-14): pool.Apply refuses any plan
	// carrying an Error, regardless of Warnings content.
	if _, _, _, err := pool.Apply(state.Pools, state.Deployments, state.Groups, plan); err == nil || !strings.Contains(err.Error(), "GOLC_POOL_APPLY_PLAN_ERRORS") {
		t.Fatalf("expected pool.Apply to refuse a plan carrying a structural error, got %v", err)
	}
}

func TestSubstitutionAtomicAcceptCancel(t *testing.T) {
	state, target, member := substitutionFixture(t)
	from := fromFixture(fixture.Capability{Type: fixture.CapabilityIntensity, Range: [2]float64{0, 1}})
	to := toFixture(fixture.Capability{Type: fixture.CapabilityIntensity, Range: [2]float64{0, 1}})
	req := substitution.SubstitutionRequest{PoolID: target.ID, FromFixtureRef: "Acme/PAR64", ToFixtureRef: "Beta/Spot300"}

	plan, err := substitution.BuildSubstitutionPlan(state, from, to, req)
	if err != nil {
		t.Fatalf("BuildSubstitutionPlan: %v", err)
	}
	if len(plan.Errors) != 0 || len(plan.Warnings) != 0 {
		t.Fatalf("expected a fully compatible substitution to carry no warnings/errors, got warnings=%+v errors=%+v", plan.Warnings, plan.Errors)
	}

	// Integrity gate: internal/pool's own ValidatePlanIntegrity, called
	// unmodified against a substitution-produced plan (D-16).
	if err := pool.ValidatePlanIntegrity(plan); err != nil {
		t.Fatalf("expected a freshly built substitution plan to pass pool.ValidatePlanIntegrity, got %v", err)
	}
	tampered := plan
	tampered.Propagate = "immediate"
	if err := pool.ValidatePlanIntegrity(tampered); err == nil || !strings.Contains(err.Error(), "GOLC_POOL_PLAN_HASH") {
		t.Fatalf("expected a tampered substitution plan to fail pool.ValidatePlanIntegrity, got %v", err)
	}

	// Freshness gate: internal/pool's own ValidatePlanFreshness, called
	// unmodified against a substitution-produced plan (D-16).
	if err := pool.ValidatePlanFreshness(plan, state.Pools, state.Deployments, state.Groups, state.Revision); err != nil {
		t.Fatalf("expected a fresh substitution plan to pass pool.ValidatePlanFreshness, got %v", err)
	}
	if err := pool.ValidatePlanFreshness(plan, state.Pools, state.Deployments, state.Groups, state.Revision+1); err == nil || !strings.Contains(err.Error(), "GOLC_POOL_PLAN_STALE") {
		t.Fatalf("expected a stale substitution plan to fail pool.ValidatePlanFreshness, got %v", err)
	}

	// Accept: pool.Apply performs the substitution atomically, through the
	// same apply mechanism a pool update uses.
	newPools, _, _, err := pool.Apply(state.Pools, state.Deployments, state.Groups, plan)
	if err != nil {
		t.Fatalf("pool.Apply: %v", err)
	}
	foundNew := false
	for _, m := range newPools[0].Members {
		if m.FixtureStableKey == "Beta/Spot300" {
			foundNew = true
		}
	}
	if !foundNew {
		t.Fatalf("expected the pool to carry a member pinned to the substituted fixture, got %+v", newPools[0].Members)
	}
	for _, m := range newPools[0].Members {
		if m.ID == member.ID {
			t.Fatalf("expected the original member to be removed by the substitution, got %+v", newPools[0].Members)
		}
	}

	// Cancel: discarding a plan without calling Apply leaves the original
	// show state untouched -- BuildSubstitutionPlan already proved this in
	// TestSubstitutionNeverApproximates, and Apply's own copy-on-write
	// contract (internal/pool) proves the input slices here are unchanged
	// too.
	if len(state.Pools[0].Members) != 1 || state.Pools[0].Members[0].ID != member.ID {
		t.Fatalf("expected the original show state to be left unmutated (cancel = discard), got %+v", state.Pools[0].Members)
	}

	// Revise: re-running with a different target produces a distinct plan
	// -- never a partial, per-item edit of the same one (D-13).
	to2 := toFixture(
		fixture.Capability{Type: fixture.CapabilityIntensity, Range: [2]float64{0, 1}},
		fixture.Capability{Type: fixture.CapabilityColor, Range: [2]float64{0, 1}},
	)
	revised, err := substitution.BuildSubstitutionPlan(state, from, to2, req)
	if err != nil {
		t.Fatalf("BuildSubstitutionPlan (revise): %v", err)
	}
	if revised.PlanID == plan.PlanID {
		t.Fatalf("expected revising with a different target to produce a distinct plan_id")
	}
}
