// svc_fixturepatch_test.go proves 06-10-PLAN.md Task 1's acceptance
// criteria (VERIFICATION.md Gap B[0], PLAY-10): a real on-screen
// fixture-patch surface must create pools, add members via a
// non-committing impact preview, apply that preview, and create/activate
// deployments -- all through the exact same "pool"/"deployment" CLI
// routes internal/command/pool.go and internal/command/deployment.go
// already declare and test (mirrors svc_surface_test.go's seed-drive-assert
// shape exactly). This file compiles against the already-implemented
// internal/command package but fails to build/pass at RUN time until
// svc_fixturepatch.go declares FixturePatchService and its methods -- that
// is the RED state Task 1 proves; svc_fixturepatch.go is NOT created by
// this task.
package wails

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/command"
	"github.com/lnorton89/golc/internal/deployment"
	"github.com/lnorton89/golc/internal/pool"
	"github.com/lnorton89/golc/internal/show"
)

// decodeImpactPlan unmarshals a preview Result's Stdout (a
// pool.ImpactPlan's canonical JSON encoding, exactly as
// internal/command/pool.go's "pool update --json" route already emits it)
// into a pool.ImpactPlan value, so this test asserts against the exact
// same operations[]/proposed_universe/proposed_address shape
// poolimpact_test.go already proves the backend route produces.
func decodeImpactPlan(stdout string) (pool.ImpactPlan, error) {
	var plan pool.ImpactPlan
	err := json.Unmarshal([]byte(stdout), &plan)
	return plan, err
}

// findPatchPoolView returns a pointer to the PatchPoolView in pools whose
// Name matches name, or nil if absent.
func findPatchPoolView(pools []PatchPoolView, name string) *PatchPoolView {
	for i := range pools {
		if pools[i].Name == name {
			return &pools[i]
		}
	}
	return nil
}

// findPatchDeploymentView returns a pointer to the PatchDeploymentView in
// deployments whose Name matches name, or nil if absent.
func findPatchDeploymentView(deployments []PatchDeploymentView, name string) *PatchDeploymentView {
	for i := range deployments {
		if deployments[i].Name == name {
			return &deployments[i]
		}
	}
	return nil
}

// seedFixturePatchShowState builds and saves a minimal ShowState with one
// pool (one existing member) and one active deployment that already
// references that pool via an existing instance -- mirroring
// internal/command/poolimpact_test.go's seedPoolShowState fixture exactly,
// so AddPoolMemberPreview has a dependent deployment to propose a new
// system-computed universe/address against (impact.go's deploymentUsesPool
// gate: proposed instances are only generated for a deployment that has
// already adopted the pool). Returns the pool's own Name and the seeded
// deployment's own Name (the service's own <pool>/<deployment> selectors).
func seedFixturePatchShowState(t *testing.T, root, showPath string) (poolName, deploymentName string) {
	t.Helper()

	p, err := pool.NewPool("Wash Pool", nil)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	member, err := pool.NewPoolMember("acme/par64", "sha256:11111111")
	if err != nil {
		t.Fatalf("NewPoolMember: %v", err)
	}
	p.Members = append(p.Members, member)

	d, err := deployment.NewDeployment("Venue A")
	if err != nil {
		t.Fatalf("NewDeployment: %v", err)
	}
	d.Active = true
	instanceID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("uuid.NewV7: %v", err)
	}
	d.Instances = append(d.Instances, deployment.Instance{
		ID:           instanceID,
		PoolID:       p.ID,
		PoolMemberID: member.ID,
		Mode:         "Standard",
		Universe:     1,
		Address:      1,
	})

	state := show.State{Pools: []pool.Pool{p}, Deployments: []deployment.Deployment{d}}
	if err := show.Save(root, showPath, state); err != nil {
		t.Fatalf("show.Save (seed): %v", err)
	}
	return p.Name, d.Name
}

// TestFixturePatchServiceCreateAndListPool proves CreatePool followed by
// ListPatch reflects the new pool with zero members, and that an empty
// show (before creation) reads as the explicit empty state.
func TestFixturePatchServiceCreateAndListPool(t *testing.T) {
	root := t.TempDir()
	showPath := filepath.Join(t.TempDir(), "show.golc")
	svc := NewFixturePatchService("", root, showPath)

	before, err := svc.ListPatch()
	if err != nil {
		t.Fatalf("ListPatch (empty show): %v", err)
	}
	if len(before.Pools) != 0 {
		t.Fatalf("expected zero pools on a fresh show, got %+v", before.Pools)
	}

	if result := svc.CreatePool("Wash Pool", nil); result.ExitCode != 0 {
		t.Fatalf("CreatePool failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	after, err := svc.ListPatch()
	if err != nil {
		t.Fatalf("ListPatch after create: %v", err)
	}
	if len(after.Pools) != 1 || after.Pools[0].Name != "Wash Pool" {
		t.Fatalf("expected exactly one pool named Wash Pool, got %+v", after.Pools)
	}
	if len(after.Pools[0].Members) != 0 {
		t.Fatalf("expected a freshly created pool to have zero members, got %+v", after.Pools[0])
	}
}

// TestFixturePatchServiceAddMemberPreviewThenApply proves the add-member
// preview-then-apply flow: AddPoolMemberPreview against a seed where a
// deployment already references the pool returns the backend's
// non-committing impact preview whose operations[] carry a
// deployment_instance add with a non-zero system-computed
// proposed_universe/proposed_address, the pool's members are UNCHANGED
// until ApplyPatch commits, and after apply ListPatch's deployment
// projection exposes the new instance's Universe/Address.
func TestFixturePatchServiceAddMemberPreviewThenApply(t *testing.T) {
	root := t.TempDir()
	showPath := filepath.Join(t.TempDir(), "show.golc")
	poolName, deploymentName := seedFixturePatchShowState(t, root, showPath)
	svc := NewFixturePatchService("", root, showPath)

	preview := svc.AddPoolMemberPreview(poolName, "acme/par64", "sha256:22222222", "Standard")
	if preview.ExitCode != 0 {
		t.Fatalf("AddPoolMemberPreview failed: exit=%d stderr=%s", preview.ExitCode, preview.Stderr)
	}
	plan, err := decodeImpactPlan(preview.Stdout)
	if err != nil {
		t.Fatalf("decode impact preview: %v", err)
	}
	if plan.PlanID == "" {
		t.Fatal("expected a non-empty plan_id in the preview")
	}
	foundAdd := false
	for _, op := range plan.Operations {
		if op.DependentKind == "deployment_instance" && op.Action == "add" {
			foundAdd = true
			if op.ProposedUniverse == 0 || op.ProposedAddress == 0 {
				t.Fatalf("expected a non-zero system-computed proposed_universe/proposed_address, got %+v", op)
			}
		}
	}
	if !foundAdd {
		t.Fatalf("expected a proposed deployment_instance add operation, got %+v", plan.Operations)
	}

	// The pool's members must be UNCHANGED until ApplyPatch commits --
	// preview never mutates the ShowState document (POOL-04/D-15).
	afterPreview, err := svc.ListPatch()
	if err != nil {
		t.Fatalf("ListPatch after preview: %v", err)
	}
	previewPool := findPatchPoolView(afterPreview.Pools, poolName)
	if previewPool == nil || len(previewPool.Members) != 1 {
		t.Fatalf("expected the pool to still carry exactly its original member before apply, got %+v", previewPool)
	}

	if result := svc.ApplyPatch(plan.PlanID); result.ExitCode != 0 {
		t.Fatalf("ApplyPatch failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	afterApply, err := svc.ListPatch()
	if err != nil {
		t.Fatalf("ListPatch after apply: %v", err)
	}
	appliedPool := findPatchPoolView(afterApply.Pools, poolName)
	if appliedPool == nil || len(appliedPool.Members) != 2 {
		t.Fatalf("expected the pool to gain the new member after apply, got %+v", appliedPool)
	}

	appliedDeployment := findPatchDeploymentView(afterApply.Deployments, deploymentName)
	if appliedDeployment == nil {
		t.Fatalf("expected deployment %q to be present, got %+v", deploymentName, afterApply.Deployments)
	}
	if len(appliedDeployment.Instances) != 2 {
		t.Fatalf("expected the deployment to gain the proposed instance, got %+v", appliedDeployment.Instances)
	}
	foundInstance := false
	for _, instance := range appliedDeployment.Instances {
		if instance.Mode == "Standard" && instance.Universe > 0 && instance.Address > 0 {
			foundInstance = true
		}
	}
	if !foundInstance {
		t.Fatalf("expected at least one instance with a positive universe/address, got %+v", appliedDeployment.Instances)
	}
}

// TestFixturePatchServiceCreateAndActivateDeployment proves
// CreateDeployment followed by ActivateDeployment leaves exactly one
// deployment active in ListPatch's projection.
func TestFixturePatchServiceCreateAndActivateDeployment(t *testing.T) {
	root := t.TempDir()
	showPath := filepath.Join(t.TempDir(), "show.golc")
	svc := NewFixturePatchService("", root, showPath)

	if result := svc.CreateDeployment("Venue B"); result.ExitCode != 0 {
		t.Fatalf("CreateDeployment failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
	if result := svc.CreateDeployment("Venue C"); result.ExitCode != 0 {
		t.Fatalf("CreateDeployment failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
	if result := svc.ActivateDeployment("Venue B"); result.ExitCode != 0 {
		t.Fatalf("ActivateDeployment failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	view, err := svc.ListPatch()
	if err != nil {
		t.Fatalf("ListPatch: %v", err)
	}
	activeCount := 0
	for _, d := range view.Deployments {
		if d.Active {
			activeCount++
			if d.Name != "Venue B" {
				t.Fatalf("expected Venue B to be the active deployment, got %+v", d)
			}
		}
	}
	if activeCount != 1 {
		t.Fatalf("expected exactly one active deployment, got %d", activeCount)
	}
}

// TestFixturePatchServiceRejectsMalformedMember proves a malformed member
// triple never panics and instead returns the route's own
// GOLC_POOL_APPLY_USAGE diagnostic.
func TestFixturePatchServiceRejectsMalformedMember(t *testing.T) {
	root := t.TempDir()
	showPath := filepath.Join(t.TempDir(), "show.golc")
	poolName, _ := seedFixturePatchShowState(t, root, showPath)
	svc := NewFixturePatchService("", root, showPath)

	result := svc.AddPoolMemberPreview(poolName, "", "", "")
	if result.ExitCode == 0 || !strings.Contains(result.Stderr, "GOLC_POOL_APPLY_USAGE") {
		t.Fatalf("expected GOLC_POOL_APPLY_USAGE for a malformed member triple, got exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
}

// TestFixturePatchServiceRejectsEmbeddedDelimiterInMemberFields proves
// AddPoolMemberPreview rejects a stableKey/contentHash/mode field containing
// the "|" delimiter internal/command/pool.go's parsePoolMemberSpec splits
// the constructed spec string on, instead of silently mis-splitting the
// spec into the wrong three fields (CR-02).
func TestFixturePatchServiceRejectsEmbeddedDelimiterInMemberFields(t *testing.T) {
	root := t.TempDir()
	showPath := filepath.Join(t.TempDir(), "show.golc")
	poolName, _ := seedFixturePatchShowState(t, root, showPath)
	svc := NewFixturePatchService("", root, showPath)

	cases := []struct {
		name                  string
		stableKey, hash, mode string
	}{
		{"stableKey", "acme|par64", "sha256:22222222", "Standard"},
		{"contentHash", "acme/par64", "sha256:2222|2222", "Standard"},
		{"mode", "acme/par64", "sha256:22222222", "Standard|Extended"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := svc.AddPoolMemberPreview(poolName, tc.stableKey, tc.hash, tc.mode)
			if result.ExitCode == 0 || !strings.Contains(result.Stderr, "GOLC_WAILS_POOL_MEMBER_FIELD_INVALID") {
				t.Fatalf("expected GOLC_WAILS_POOL_MEMBER_FIELD_INVALID for an embedded delimiter in %s, got exit=%d stderr=%s", tc.name, result.ExitCode, result.Stderr)
			}
		})
	}
}

// TestFixturePatchServiceEmptyAndCountStates proves ListPatch on a show
// with no pools returns an empty projection, and singular/plural pool
// counts read correctly once pools exist.
func TestFixturePatchServiceEmptyAndCountStates(t *testing.T) {
	root := t.TempDir()
	showPath := filepath.Join(t.TempDir(), "show.golc")
	svc := NewFixturePatchService("", root, showPath)

	empty, err := svc.ListPatch()
	if err != nil {
		t.Fatalf("ListPatch (empty): %v", err)
	}
	if len(empty.Pools) != 0 || len(empty.Deployments) != 0 {
		t.Fatalf("expected zero pools and deployments on a fresh show, got %+v", empty)
	}

	if result := svc.CreatePool("Solo Pool", nil); result.ExitCode != 0 {
		t.Fatalf("CreatePool failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
	one, err := svc.ListPatch()
	if err != nil {
		t.Fatalf("ListPatch (one pool): %v", err)
	}
	if len(one.Pools) != 1 {
		t.Fatalf("expected exactly one pool, got %+v", one.Pools)
	}

	if result := svc.CreatePool("Second Pool", nil); result.ExitCode != 0 {
		t.Fatalf("CreatePool failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
	many, err := svc.ListPatch()
	if err != nil {
		t.Fatalf("ListPatch (many pools): %v", err)
	}
	if len(many.Pools) != 2 {
		t.Fatalf("expected exactly two pools, got %+v", many.Pools)
	}
}

// TestFixturePatchServiceApplyStalePlanRejected proves applying a
// stale/unknown plan-id surfaces the pool route's own freshness/integrity
// diagnostic (POOL-08), never a silent success.
func TestFixturePatchServiceApplyStalePlanRejected(t *testing.T) {
	root := t.TempDir()
	showPath := filepath.Join(t.TempDir(), "show.golc")
	poolName, _ := seedFixturePatchShowState(t, root, showPath)
	svc := NewFixturePatchService("", root, showPath)

	preview := svc.AddPoolMemberPreview(poolName, "acme/par64", "sha256:33333333", "Standard")
	if preview.ExitCode != 0 {
		t.Fatalf("AddPoolMemberPreview failed: exit=%d stderr=%s", preview.ExitCode, preview.Stderr)
	}
	plan, err := decodeImpactPlan(preview.Stdout)
	if err != nil {
		t.Fatalf("decode impact preview: %v", err)
	}

	// A registry-level "pool create" mutation between preview and apply
	// moves the ShowState revision, staling the previewed plan.
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}
	if result := registry.Execute(command.Request{Root: root, Args: []string{
		"pool", "create", "Unrelated Pool", "--show", showPath,
	}}); result.ExitCode != 0 {
		t.Fatalf("pool create (stale trigger) failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	stale := svc.ApplyPatch(plan.PlanID)
	if stale.ExitCode == 0 || !strings.Contains(stale.Stderr, "GOLC_POOL_PLAN_STALE") {
		t.Fatalf("expected GOLC_POOL_PLAN_STALE for a stale apply, got exit=%d stderr=%s", stale.ExitCode, stale.Stderr)
	}

	unknown := svc.ApplyPatch("not-a-real-plan-id")
	if unknown.ExitCode == 0 {
		t.Fatalf("expected an unknown plan-id apply to fail, got exit=%d stdout=%s", unknown.ExitCode, unknown.Stdout)
	}
}
