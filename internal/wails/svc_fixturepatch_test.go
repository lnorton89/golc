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
	"sort"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

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
	require.NoError(t, err, "NewPool")
	member, err := pool.NewPoolMember("acme/par64", "sha256:11111111")
	require.NoError(t, err, "NewPoolMember")
	p.Members = append(p.Members, member)

	d, err := deployment.NewDeployment("Venue A")
	require.NoError(t, err, "NewDeployment")
	d.Active = true
	instanceID, err := uuid.NewV7()
	require.NoError(t, err, "uuid.NewV7")
	d.Instances = append(d.Instances, deployment.Instance{
		ID:           instanceID,
		PoolID:       p.ID,
		PoolMemberID: member.ID,
		Mode:         "Standard",
		Universe:     1,
		Address:      1,
	})

	state := show.State{Pools: []pool.Pool{p}, Deployments: []deployment.Deployment{d}}
	err = show.Save(root, showPath, state)
	require.NoError(t, err, "show.Save (seed)")
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
	require.NoError(t, err, "ListPatch (empty show)")
	require.Len(t, before.Pools, 0, "expected zero pools on a fresh show: %+v", before.Pools)

	result := svc.CreatePool("Wash Pool", nil)
	require.Equal(t, 0, result.ExitCode, "CreatePool failed: stderr=%s", result.Stderr)

	after, err := svc.ListPatch()
	require.NoError(t, err, "ListPatch after create")
	require.True(t, len(after.Pools) == 1 && after.Pools[0].Name == "Wash Pool", "expected exactly one pool named Wash Pool, got %+v", after.Pools)
	require.Len(t, after.Pools[0].Members, 0, "expected a freshly created pool to have zero members: %+v", after.Pools[0])
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

	preview := svc.AddPoolMemberPreview(poolName, "acme/par64", "sha256:22222222", "Standard", 0)
	require.Equal(t, 0, preview.ExitCode, "AddPoolMemberPreview failed: stderr=%s", preview.Stderr)
	plan, err := decodeImpactPlan(preview.Stdout)
	require.NoError(t, err, "decode impact preview")
	require.NotEmpty(t, plan.PlanID, "expected a non-empty plan_id in the preview")
	foundAdd := false
	for _, op := range plan.Operations {
		if op.DependentKind == "deployment_instance" && op.Action == "add" {
			foundAdd = true
			require.True(t, op.ProposedUniverse != 0 && op.ProposedAddress != 0, "expected a non-zero system-computed proposed_universe/proposed_address, got %+v", op)
		}
	}
	require.True(t, foundAdd, "expected a proposed deployment_instance add operation, got %+v", plan.Operations)

	// The pool's members must be UNCHANGED until ApplyPatch commits --
	// preview never mutates the ShowState document (POOL-04/D-15).
	afterPreview, err := svc.ListPatch()
	require.NoError(t, err, "ListPatch after preview")
	previewPool := findPatchPoolView(afterPreview.Pools, poolName)
	require.True(t, previewPool != nil && len(previewPool.Members) == 1, "expected the pool to still carry exactly its original member before apply, got %+v", previewPool)

	result := svc.ApplyPatch(plan.PlanID)
	require.Equal(t, 0, result.ExitCode, "ApplyPatch failed: stderr=%s", result.Stderr)

	afterApply, err := svc.ListPatch()
	require.NoError(t, err, "ListPatch after apply")
	appliedPool := findPatchPoolView(afterApply.Pools, poolName)
	require.True(t, appliedPool != nil && len(appliedPool.Members) == 2, "expected the pool to gain the new member after apply, got %+v", appliedPool)

	appliedDeployment := findPatchDeploymentView(afterApply.Deployments, deploymentName)
	require.NotNil(t, appliedDeployment, "expected deployment %q to be present, got %+v", deploymentName, afterApply.Deployments)
	require.Len(t, appliedDeployment.Instances, 2, "expected the deployment to gain the proposed instance: %+v", appliedDeployment.Instances)
	foundInstance := false
	for _, instance := range appliedDeployment.Instances {
		if instance.Mode == "Standard" && instance.Universe > 0 && instance.Address > 0 {
			foundInstance = true
		}
	}
	require.True(t, foundInstance, "expected at least one instance with a positive universe/address, got %+v", appliedDeployment.Instances)
}

// TestFixturePatchServiceCreateAndActivateDeployment proves
// CreateDeployment followed by ActivateDeployment leaves exactly one
// deployment active in ListPatch's projection.
func TestFixturePatchServiceCreateAndActivateDeployment(t *testing.T) {
	root := t.TempDir()
	showPath := filepath.Join(t.TempDir(), "show.golc")
	svc := NewFixturePatchService("", root, showPath)

	result := svc.CreateDeployment("Venue B")
	require.Equal(t, 0, result.ExitCode, "CreateDeployment failed: stderr=%s", result.Stderr)
	result = svc.CreateDeployment("Venue C")
	require.Equal(t, 0, result.ExitCode, "CreateDeployment failed: stderr=%s", result.Stderr)
	result = svc.ActivateDeployment("Venue B")
	require.Equal(t, 0, result.ExitCode, "ActivateDeployment failed: stderr=%s", result.Stderr)

	view, err := svc.ListPatch()
	require.NoError(t, err, "ListPatch")
	activeCount := 0
	for _, d := range view.Deployments {
		if d.Active {
			activeCount++
			require.Equal(t, "Venue B", d.Name, "expected Venue B to be the active deployment, got %+v", d)
		}
	}
	require.Equal(t, 1, activeCount, "expected exactly one active deployment")
}

// TestFixturePatchServiceRejectsMalformedMember proves a malformed member
// triple never panics and instead returns the route's own
// GOLC_POOL_APPLY_USAGE diagnostic.
func TestFixturePatchServiceRejectsMalformedMember(t *testing.T) {
	root := t.TempDir()
	showPath := filepath.Join(t.TempDir(), "show.golc")
	poolName, _ := seedFixturePatchShowState(t, root, showPath)
	svc := NewFixturePatchService("", root, showPath)

	result := svc.AddPoolMemberPreview(poolName, "", "", "", 0)
	require.NotEqual(t, 0, result.ExitCode, "expected GOLC_POOL_APPLY_USAGE for a malformed member triple")
	require.Contains(t, result.Stderr, "GOLC_POOL_APPLY_USAGE")
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
			result := svc.AddPoolMemberPreview(poolName, tc.stableKey, tc.hash, tc.mode, 0)
			require.NotEqual(t, 0, result.ExitCode, "expected GOLC_WAILS_POOL_MEMBER_FIELD_INVALID for an embedded delimiter in %s", tc.name)
			require.Contains(t, result.Stderr, "GOLC_WAILS_POOL_MEMBER_FIELD_INVALID", "embedded delimiter in %s", tc.name)
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
	require.NoError(t, err, "ListPatch (empty)")
	require.True(t, len(empty.Pools) == 0 && len(empty.Deployments) == 0, "expected zero pools and deployments on a fresh show, got %+v", empty)

	result := svc.CreatePool("Solo Pool", nil)
	require.Equal(t, 0, result.ExitCode, "CreatePool failed: stderr=%s", result.Stderr)
	one, err := svc.ListPatch()
	require.NoError(t, err, "ListPatch (one pool)")
	require.Len(t, one.Pools, 1, "expected exactly one pool: %+v", one.Pools)

	result = svc.CreatePool("Second Pool", nil)
	require.Equal(t, 0, result.ExitCode, "CreatePool failed: stderr=%s", result.Stderr)
	many, err := svc.ListPatch()
	require.NoError(t, err, "ListPatch (many pools)")
	require.Len(t, many.Pools, 2, "expected exactly two pools: %+v", many.Pools)
}

// TestFixturePatchServiceApplyStalePlanRejected proves applying a
// stale/unknown plan-id surfaces the pool route's own freshness/integrity
// diagnostic (POOL-08), never a silent success.
func TestFixturePatchServiceApplyStalePlanRejected(t *testing.T) {
	root := t.TempDir()
	showPath := filepath.Join(t.TempDir(), "show.golc")
	poolName, _ := seedFixturePatchShowState(t, root, showPath)
	svc := NewFixturePatchService("", root, showPath)

	preview := svc.AddPoolMemberPreview(poolName, "acme/par64", "sha256:33333333", "Standard", 0)
	require.Equal(t, 0, preview.ExitCode, "AddPoolMemberPreview failed: stderr=%s", preview.Stderr)
	plan, err := decodeImpactPlan(preview.Stdout)
	require.NoError(t, err, "decode impact preview")

	// A registry-level "pool create" mutation between preview and apply
	// moves the ShowState revision, staling the previewed plan.
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err, "NewDefaultCommandRegistry")
	createResult := registry.Execute(command.Request{Root: root, Args: []string{
		"pool", "create", "Unrelated Pool", "--show", showPath,
	}})
	require.Equal(t, 0, createResult.ExitCode, "pool create (stale trigger) failed: stderr=%s", createResult.Stderr)

	stale := svc.ApplyPatch(plan.PlanID)
	require.NotEqual(t, 0, stale.ExitCode, "expected a stale apply to fail")
	require.Contains(t, stale.Stderr, "GOLC_POOL_PLAN_STALE")

	unknown := svc.ApplyPatch("not-a-real-plan-id")
	require.NotEqual(t, 0, unknown.ExitCode, "expected an unknown plan-id apply to fail, got stdout=%s", unknown.Stdout)
}

// seedFreshFixturePatchShowState builds and saves a minimal ShowState with
// one pool (no members yet) and one deployment with zero instances of it --
// the exact state a brand-new pool starts in, with no adopting deployment.
// Returns the pool's own Name and the deployment's own ID string.
func seedFreshFixturePatchShowState(t *testing.T, root, showPath string) (poolName, deploymentID string) {
	t.Helper()

	p, err := pool.NewPool("Fresh Pool", nil)
	require.NoError(t, err, "NewPool")
	d, err := deployment.NewDeployment("Venue A")
	require.NoError(t, err, "NewDeployment")
	d.Active = true

	state := show.State{Pools: []pool.Pool{p}, Deployments: []deployment.Deployment{d}}
	err = show.Save(root, showPath, state)
	require.NoError(t, err, "show.Save (seed)")
	return p.Name, d.ID.String()
}

// TestAddPoolMembersPreview proves the batch-add-with-force-attach flow: a
// fresh pool with zero dependents plus attachDeploymentID still yields one
// proposed instance per unit, all distinct and in-bounds, and the returned
// plan applies successfully with ListPatch reflecting every new instance
// afterward. Passing a real channelCount (5, as a 5-channel RGBW+strobe
// mode would resolve to) proves each proposed instance is spaced by that
// width -- addresses 1, 6, 11 -- not packed one address apart the way the
// pre-channel-count 1-channel fallback would (the exact bug this field
// closes: three fixtures added from the library at "channel_count 5,
// starting address 1" landing at addresses 1, 2, 3 instead of 1, 6, 11).
func TestAddPoolMembersPreview(t *testing.T) {
	root := t.TempDir()
	showPath := filepath.Join(t.TempDir(), "show.golc")
	poolName, deploymentID := seedFreshFixturePatchShowState(t, root, showPath)
	svc := NewFixturePatchService("", root, showPath)

	preview := svc.AddPoolMembersPreview(poolName, "acme/par64", "sha256:aaaaaaaa", "Standard", 3, deploymentID, 1, 1, 5)
	require.Equal(t, 0, preview.ExitCode, "AddPoolMembersPreview failed: stderr=%s", preview.Stderr)
	plan, err := decodeImpactPlan(preview.Stdout)
	require.NoError(t, err, "decode impact preview")

	var addOps []pool.ImpactOp
	for _, op := range plan.Operations {
		if op.DependentKind == "deployment_instance" && op.Action == "add" {
			addOps = append(addOps, op)
		}
	}
	require.Len(t, addOps, 3, "expected 3 proposed instances: %+v", addOps)
	sort.Slice(addOps, func(i, j int) bool { return addOps[i].ProposedAddress < addOps[j].ProposedAddress })
	wantAddresses := []int{1, 6, 11}
	seen := map[[2]int]bool{}
	for i, op := range addOps {
		require.True(t, op.ProposedUniverse == 1 && op.ProposedAddress == wantAddresses[i], "expected 5-channel-spaced addresses %v, got op[%d]=%+v", wantAddresses, i, op)
		key := [2]int{op.ProposedUniverse, op.ProposedAddress}
		require.False(t, seen[key], "expected distinct proposed addresses across the batch, got a collision at %+v", op)
		seen[key] = true
	}

	result := svc.ApplyPatch(plan.PlanID)
	require.Equal(t, 0, result.ExitCode, "ApplyPatch failed: stderr=%s", result.Stderr)

	after, err := svc.ListPatch()
	require.NoError(t, err, "ListPatch after apply")
	appliedDeployment := findPatchDeploymentView(after.Deployments, "Venue A")
	require.True(t, appliedDeployment != nil && len(appliedDeployment.Instances) == 3, "expected the deployment to gain 3 new instances, got %+v", appliedDeployment)
}

// TestRenamePool proves RenamePool renames a pool in place (members/
// instances untouched) and surfaces the underlying route's own rejection
// of an unknown pool name.
func TestRenamePool(t *testing.T) {
	root := t.TempDir()
	showPath := filepath.Join(t.TempDir(), "show.golc")
	poolName, _ := seedFixturePatchShowState(t, root, showPath)
	svc := NewFixturePatchService("", root, showPath)

	result := svc.RenamePool(poolName, "Renamed Pool")
	require.Equal(t, 0, result.ExitCode, "RenamePool failed: stderr=%s", result.Stderr)

	view, err := svc.ListPatch()
	require.NoError(t, err, "ListPatch")
	renamed := findPatchPoolView(view.Pools, "Renamed Pool")
	require.True(t, renamed != nil && len(renamed.Members) == 1, "expected the pool to survive rename with its member intact, got %+v", view.Pools)

	notFound := svc.RenamePool("Nonexistent", "Whatever")
	require.NotEqual(t, 0, notFound.ExitCode, "expected GOLC_POOL_NOT_FOUND")
	require.Contains(t, notFound.Stderr, "GOLC_POOL_NOT_FOUND")
}

// TestRenameDeployment proves RenameDeployment renames a deployment in
// place (instances/active flag untouched).
func TestRenameDeployment(t *testing.T) {
	root := t.TempDir()
	showPath := filepath.Join(t.TempDir(), "show.golc")
	_, deploymentName := seedFixturePatchShowState(t, root, showPath)
	svc := NewFixturePatchService("", root, showPath)

	result := svc.RenameDeployment(deploymentName, "Renamed Deployment")
	require.Equal(t, 0, result.ExitCode, "RenameDeployment failed: stderr=%s", result.Stderr)

	view, err := svc.ListPatch()
	require.NoError(t, err, "ListPatch")
	renamed := findPatchDeploymentView(view.Deployments, "Renamed Deployment")
	require.True(t, renamed != nil && len(renamed.Instances) == 1, "expected the deployment to survive rename with its instance intact, got %+v", view.Deployments)
}

// TestDeletePoolCascadesThroughListPatch proves DeletePool cascade-deletes
// through the Wails boundary: the pool and its dependent deployment
// instance both disappear from ListPatch's projection.
func TestDeletePoolCascadesThroughListPatch(t *testing.T) {
	root := t.TempDir()
	showPath := filepath.Join(t.TempDir(), "show.golc")
	poolName, deploymentName := seedFixturePatchShowState(t, root, showPath)
	svc := NewFixturePatchService("", root, showPath)

	result := svc.DeletePool(poolName)
	require.Equal(t, 0, result.ExitCode, "DeletePool failed: stderr=%s", result.Stderr)

	view, err := svc.ListPatch()
	require.NoError(t, err, "ListPatch")
	require.Len(t, view.Pools, 0, "expected zero pools after DeletePool: %+v", view.Pools)
	dep := findPatchDeploymentView(view.Deployments, deploymentName)
	require.True(t, dep != nil && len(dep.Instances) == 0, "expected the deployment to survive with zero instances, got %+v", dep)

	notFound := svc.DeletePool("Nonexistent")
	require.NotEqual(t, 0, notFound.ExitCode, "expected GOLC_POOL_NOT_FOUND")
	require.Contains(t, notFound.Stderr, "GOLC_POOL_NOT_FOUND")
}

// TestDeleteDeploymentThroughListPatch proves DeleteDeployment removes a
// deployment from ListPatch's projection.
func TestDeleteDeploymentThroughListPatch(t *testing.T) {
	root := t.TempDir()
	showPath := filepath.Join(t.TempDir(), "show.golc")
	svc := NewFixturePatchService("", root, showPath)

	result := svc.CreateDeployment("Venue A")
	require.Equal(t, 0, result.ExitCode, "CreateDeployment failed: stderr=%s", result.Stderr)
	result = svc.DeleteDeployment("Venue A")
	require.Equal(t, 0, result.ExitCode, "DeleteDeployment failed: stderr=%s", result.Stderr)

	view, err := svc.ListPatch()
	require.NoError(t, err, "ListPatch")
	require.Len(t, view.Deployments, 0, "expected zero deployments after DeleteDeployment: %+v", view.Deployments)

	notFound := svc.DeleteDeployment("Nonexistent")
	require.NotEqual(t, 0, notFound.ExitCode, "expected GOLC_DEPLOYMENT_NOT_FOUND")
	require.Contains(t, notFound.Stderr, "GOLC_DEPLOYMENT_NOT_FOUND")
}

// TestReassignInstanceThroughWails proves ReassignInstance updates an
// instance's mode/universe/address in place, and a collision error
// surfaces via Result.Stderr with ListPatch left unchanged.
func TestReassignInstanceThroughWails(t *testing.T) {
	root := t.TempDir()
	showPath := filepath.Join(t.TempDir(), "show.golc")
	poolName, deploymentName := seedFixturePatchShowState(t, root, showPath)
	svc := NewFixturePatchService("", root, showPath)

	before, err := svc.ListPatch()
	require.NoError(t, err, "ListPatch (before)")
	seededDeployment := findPatchDeploymentView(before.Deployments, deploymentName)
	require.True(t, seededDeployment != nil && len(seededDeployment.Instances) == 1, "expected exactly one seeded instance, got %+v", seededDeployment)
	instanceID := seededDeployment.Instances[0].ID

	result := svc.ReassignInstance(deploymentName, instanceID, "Extended", 3, 50)
	require.Equal(t, 0, result.ExitCode, "ReassignInstance failed: stderr=%s", result.Stderr)

	after, err := svc.ListPatch()
	require.NoError(t, err, "ListPatch (after)")
	updatedDeployment := findPatchDeploymentView(after.Deployments, deploymentName)
	require.True(t, updatedDeployment != nil && len(updatedDeployment.Instances) == 1, "expected exactly one instance to survive, got %+v", updatedDeployment)
	updated := updatedDeployment.Instances[0]
	require.True(t, updated.Mode == "Extended" && updated.Universe == 3 && updated.Address == 50, "expected mode/universe/address to update, got %+v", updated)

	// Add a second instance to create a real collision target.
	preview := svc.AddPoolMembersPreview(poolName, "acme/par64", "sha256:99999999", "Standard", 1, "", 0, 0, 0)
	require.Equal(t, 0, preview.ExitCode, "AddPoolMembersPreview failed: stderr=%s", preview.Stderr)
	plan, err := decodeImpactPlan(preview.Stdout)
	require.NoError(t, err, "decode impact preview")
	applyResult := svc.ApplyPatch(plan.PlanID)
	require.Equal(t, 0, applyResult.ExitCode, "ApplyPatch failed: stderr=%s", applyResult.Stderr)

	collide := svc.ReassignInstance(deploymentName, instanceID, "Extended", 1, 1)
	require.NotEqual(t, 0, collide.ExitCode, "expected GOLC_DEPLOYMENT_ADDRESS_COLLISION")
	require.Contains(t, collide.Stderr, "GOLC_DEPLOYMENT_ADDRESS_COLLISION")

	unchanged, err := svc.ListPatch()
	require.NoError(t, err, "ListPatch (after failed collision)")
	unchangedDeployment := findPatchDeploymentView(unchanged.Deployments, deploymentName)
	for _, instance := range unchangedDeployment.Instances {
		if instance.ID == instanceID {
			require.True(t, instance.Universe == 3 && instance.Address == 50, "expected the instance to be left unchanged after a failed reassign, got %+v", instance)
		}
	}
}
