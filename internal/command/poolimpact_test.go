// poolimpact_test.go proves the "pool update" (dry-run) / "pool apply"
// route contract (02-05-PLAN.md, Task 1: POOL-03/POOL-04/POOL-05/POOL-08):
// "pool update" computes and writes a deterministic impact plan without
// mutating the ShowState file, the resolved propagation default is
// review-required (preview) when no --propagate override is given, and
// "pool apply" validates (integrity then freshness) before an atomic
// apply, rejecting a stale re-apply or a tampered plan.
//
// It follows internal/projectconfig/strict_test.go's repositoryRoot
// convention: production concern files (config/application-defaults.toml)
// are validated exactly as committed, so the default propagation
// resolution exercises the real committed value rather than a synthetic
// fixture. The ShowState file itself always lives in an isolated
// t.TempDir(), so these tests never write into the real checkout.
//
// This file compiles against the already-implemented internal/command
// package but fails at RUN time until pool.go self-registers "pool
// update"/"pool apply" (Task 3) -- that is the RED state this task
// proves.
package command_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/command"
	"github.com/lnorton89/golc/internal/deployment"
	"github.com/lnorton89/golc/internal/pool"
	"github.com/lnorton89/golc/internal/show"
)

// repositoryRoot resolves the real checkout root from this package
// directory (internal/command -> internal -> root) so "pool update"'s
// application_defaults.pool_update_review resolution exercises the real
// committed config/application-defaults.toml.
func repositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err, "resolve repository root")
	_, err = os.Stat(filepath.Join(root, "golc.project.toml"))
	require.NoErrorf(t, err, "repository root %q has no golc.project.toml", root)
	return root
}

// seedPoolShowState builds and saves a minimal ShowState with one pool
// (one existing member) and one active deployment already patched to that
// member, so "pool update --add" has a dependent to propose an instance
// against. It returns the pool's own Name (the CLI's own <pool> selector).
func seedPoolShowState(t *testing.T, root, showPath string) string {
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
	require.NoError(t, show.Save(root, showPath, state), "show.Save (seed)")
	return p.Name
}

type poolPlanView struct {
	SchemaVersion int    `json:"schema_version"`
	PlanID        string `json:"plan_id"`
	Propagate     string `json:"propagate"`
	Add           []struct {
		FixtureStableKey string `json:"fixture_stable_key"`
	} `json:"add"`
	Operations []struct {
		DependentKind    string `json:"dependent_kind"`
		Action           string `json:"action"`
		ProposedUniverse int    `json:"proposed_universe"`
		ProposedAddress  int    `json:"proposed_address"`
	} `json:"operations"`
}

// seedFreshPoolShowState builds and saves a minimal ShowState with one pool
// (no members yet) and one deployment with zero instances of it -- i.e. a
// pool with no adopting deployment, the exact state a brand-new pool starts
// in. It returns the pool's Name and the deployment's own ID string, for
// exercising "pool update --attach-deployment".
func seedFreshPoolShowState(t *testing.T, root, showPath string) (poolName, deploymentID string) {
	t.Helper()

	p, err := pool.NewPool("Fresh Pool", nil)
	require.NoError(t, err, "NewPool")
	d, err := deployment.NewDeployment("Venue A")
	require.NoError(t, err, "NewDeployment")
	d.Active = true

	state := show.State{Pools: []pool.Pool{p}, Deployments: []deployment.Deployment{d}}
	require.NoError(t, show.Save(root, showPath, state), "show.Save (seed)")
	return p.Name, d.ID.String()
}

func TestPoolUpdateApplyRoutes(t *testing.T) {
	root := repositoryRoot(t)
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err, "NewDefaultCommandRegistry")

	showPath := filepath.Join(t.TempDir(), "show.json")
	poolName := seedPoolShowState(t, root, showPath)
	planPath := filepath.Join(t.TempDir(), "plan.json")

	before, err := os.ReadFile(showPath)
	require.NoError(t, err, "read seed show file")

	update := registry.Execute(command.Request{Root: root, Args: []string{
		"pool", "update", poolName,
		"--add", "acme/par64|sha256:22222222|Standard",
		"--out", planPath,
		"--show", showPath,
	}})
	require.Equalf(t, 0, update.ExitCode, "pool update failed: stderr=%s", update.Stderr)

	after, err := os.ReadFile(showPath)
	require.NoError(t, err, "read show file after dry-run")
	require.Equal(t, string(before), string(after), "expected pool update (dry-run) to leave the ShowState file byte-unchanged")

	planBytes, err := os.ReadFile(planPath)
	require.NoError(t, err, "read written plan")
	var view poolPlanView
	require.NoError(t, json.Unmarshal(planBytes, &view), "unmarshal plan")
	require.NotEmpty(t, view.PlanID, "expected a non-empty plan_id")
	require.Lenf(t, view.Add, 1, "expected the plan to carry the requested add spec, got %+v", view.Add)
	require.Equal(t, "acme/par64", view.Add[0].FixtureStableKey)
	foundAddOp := false
	for _, op := range view.Operations {
		if op.DependentKind == "deployment_instance" && op.Action == "add" {
			foundAddOp = true
		}
	}
	require.Truef(t, foundAddOp, "expected a proposed deployment_instance add operation, got %+v", view.Operations)

	apply := registry.Execute(command.Request{Root: root, Args: []string{
		"pool", "apply", planPath, "--plan-id", view.PlanID, "--show", showPath,
	}})
	require.Equalf(t, 0, apply.ExitCode, "pool apply failed: stderr=%s", apply.Stderr)

	applied, err := show.Load(root, showPath)
	require.NoError(t, err, "show.Load after apply")
	require.Lenf(t, applied.Pools, 1, "expected the pool to gain the new member, got %+v", applied.Pools)
	require.Lenf(t, applied.Pools[0].Members, 2, "expected the pool to gain the new member, got %+v", applied.Pools)
	require.Lenf(t, applied.Deployments, 1, "expected the deployment to gain the proposed instance, got %+v", applied.Deployments)
	require.Lenf(t, applied.Deployments[0].Instances, 2, "expected the deployment to gain the proposed instance, got %+v", applied.Deployments)

	// A stale re-apply of the exact same plan file is rejected (single-use):
	// the ShowState revision moved when the first apply saved.
	staleApply := registry.Execute(command.Request{Root: root, Args: []string{
		"pool", "apply", planPath, "--plan-id", view.PlanID, "--show", showPath,
	}})
	require.NotEqualf(t, 0, staleApply.ExitCode, "expected GOLC_POOL_PLAN_STALE for a stale re-apply, got stderr=%s", staleApply.Stderr)
	require.Contains(t, string(staleApply.Stderr), "GOLC_POOL_PLAN_STALE")

	// A tampered plan file (bytes altered after hashing) is rejected by
	// the integrity gate before freshness is even considered.
	tamperedPath := filepath.Join(t.TempDir(), "tampered-plan.json")
	tampered := strings.Replace(string(planBytes), "\"preview\"", "\"immediate\"", 1)
	require.NotEqual(t, string(planBytes), tampered, "expected the tamper substitution to change the plan bytes")
	require.NoError(t, os.WriteFile(tamperedPath, []byte(tampered), 0o644), "write tampered plan")
	tamperedApply := registry.Execute(command.Request{Root: root, Args: []string{
		"pool", "apply", tamperedPath, "--plan-id", view.PlanID, "--show", showPath,
	}})
	require.NotEqualf(t, 0, tamperedApply.ExitCode, "expected GOLC_POOL_PLAN_HASH for a tampered plan, got stderr=%s", tamperedApply.Stderr)
	require.Contains(t, string(tamperedApply.Stderr), "GOLC_POOL_PLAN_HASH")
}

func TestPropagationDefaultReview(t *testing.T) {
	root := repositoryRoot(t)
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err, "NewDefaultCommandRegistry")

	showPath := filepath.Join(t.TempDir(), "show.json")
	poolName := seedPoolShowState(t, root, showPath)

	defaultPlanPath := filepath.Join(t.TempDir(), "default-plan.json")
	defaultUpdate := registry.Execute(command.Request{Root: root, Args: []string{
		"pool", "update", poolName,
		"--add", "acme/par64|sha256:33333333|Standard",
		"--out", defaultPlanPath,
		"--show", showPath,
	}})
	require.Equalf(t, 0, defaultUpdate.ExitCode, "pool update (no --propagate) failed: stderr=%s", defaultUpdate.Stderr)
	var defaultView poolPlanView
	defaultBytes, err := os.ReadFile(defaultPlanPath)
	require.NoError(t, err, "read default plan")
	require.NoError(t, json.Unmarshal(defaultBytes, &defaultView), "unmarshal default plan")
	require.Equalf(t, "preview", defaultView.Propagate, "expected the unset propagation default to resolve to review-required (preview), got %q", defaultView.Propagate)

	immediatePlanPath := filepath.Join(t.TempDir(), "immediate-plan.json")
	immediateUpdate := registry.Execute(command.Request{Root: root, Args: []string{
		"pool", "update", poolName,
		"--add", "acme/par64|sha256:44444444|Standard",
		"--propagate", "immediate",
		"--out", immediatePlanPath,
		"--show", showPath,
	}})
	require.Equalf(t, 0, immediateUpdate.ExitCode, "pool update (--propagate immediate) failed: stderr=%s", immediateUpdate.Stderr)
	var immediateView poolPlanView
	immediateBytes, err := os.ReadFile(immediatePlanPath)
	require.NoError(t, err, "read immediate plan")
	require.NoError(t, json.Unmarshal(immediateBytes, &immediateView), "unmarshal immediate plan")
	require.Equalf(t, "immediate", immediateView.Propagate, "expected --propagate immediate to override the default, got %q", immediateView.Propagate)

	invalid := registry.Execute(command.Request{Root: root, Args: []string{
		"pool", "update", poolName,
		"--propagate", "bogus",
		"--out", filepath.Join(t.TempDir(), "bogus-plan.json"),
		"--show", showPath,
	}})
	require.NotEqualf(t, 0, invalid.ExitCode, "expected GOLC_POOL_APPLY_USAGE for an invalid --propagate value, got stderr=%s", invalid.Stderr)
	require.Contains(t, string(invalid.Stderr), "GOLC_POOL_APPLY_USAGE")
}

// TestPoolUpdateAttachDeploymentFlag proves "pool update --attach-deployment"
// closes the "adopt a never-before-used pool" gap at the CLI layer: a pool
// with no dependent deployment still yields a proposed instance for the
// named deployment.
func TestPoolUpdateAttachDeploymentFlag(t *testing.T) {
	root := repositoryRoot(t)
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err, "NewDefaultCommandRegistry")

	showPath := filepath.Join(t.TempDir(), "show.json")
	poolName, deploymentID := seedFreshPoolShowState(t, root, showPath)
	planPath := filepath.Join(t.TempDir(), "plan.json")

	update := registry.Execute(command.Request{Root: root, Args: []string{
		"pool", "update", poolName,
		"--add", "acme/par64|sha256:55555555|Standard",
		"--attach-deployment", deploymentID,
		"--out", planPath,
		"--show", showPath,
	}})
	require.Equalf(t, 0, update.ExitCode, "pool update --attach-deployment failed: stderr=%s", update.Stderr)

	planBytes, err := os.ReadFile(planPath)
	require.NoError(t, err, "read written plan")
	var view poolPlanView
	require.NoError(t, json.Unmarshal(planBytes, &view), "unmarshal plan")
	foundAddOp := false
	for _, op := range view.Operations {
		if op.DependentKind == "deployment_instance" && op.Action == "add" {
			foundAddOp = true
			require.Equalf(t, 1, op.ProposedUniverse, "expected the first proposed instance in a fresh deployment to land at (1, 1), got (%d, %d)", op.ProposedUniverse, op.ProposedAddress)
			require.Equalf(t, 1, op.ProposedAddress, "expected the first proposed instance in a fresh deployment to land at (1, 1), got (%d, %d)", op.ProposedUniverse, op.ProposedAddress)
		}
	}
	require.Truef(t, foundAddOp, "expected --attach-deployment to force-propose a deployment_instance add operation, got %+v", view.Operations)
}

// TestPoolUpdateAttachDeploymentRejectsUnknownID proves an
// --attach-deployment value that doesn't name a real deployment fails
// before any operation is computed.
func TestPoolUpdateAttachDeploymentRejectsUnknownID(t *testing.T) {
	root := repositoryRoot(t)
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err, "NewDefaultCommandRegistry")

	showPath := filepath.Join(t.TempDir(), "show.json")
	poolName, _ := seedFreshPoolShowState(t, root, showPath)

	unknownID, err := uuid.NewV7()
	require.NoError(t, err, "uuid.NewV7")
	update := registry.Execute(command.Request{Root: root, Args: []string{
		"pool", "update", poolName,
		"--add", "acme/par64|sha256:66666666|Standard",
		"--attach-deployment", unknownID.String(),
		"--out", filepath.Join(t.TempDir(), "plan.json"),
		"--show", showPath,
	}})
	require.NotEqualf(t, 0, update.ExitCode, "expected GOLC_POOL_PLAN_UNKNOWN_DEPLOYMENT, got stderr=%s", update.Stderr)
	require.Contains(t, string(update.Stderr), "GOLC_POOL_PLAN_UNKNOWN_DEPLOYMENT")
}

// TestPoolUpdateStartAddressFlags proves --start-universe/--start-address
// anchor the proposed addresses for a multi-add batch, auto-incrementing
// the second unit past the first.
func TestPoolUpdateStartAddressFlags(t *testing.T) {
	root := repositoryRoot(t)
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err, "NewDefaultCommandRegistry")

	showPath := filepath.Join(t.TempDir(), "show.json")
	poolName := seedPoolShowState(t, root, showPath)
	planPath := filepath.Join(t.TempDir(), "plan.json")

	update := registry.Execute(command.Request{Root: root, Args: []string{
		"pool", "update", poolName,
		"--add", "acme/par64|sha256:77777777|Standard",
		"--add", "acme/par64|sha256:88888888|Standard",
		"--start-universe", "3",
		"--start-address", "50",
		"--out", planPath,
		"--show", showPath,
	}})
	require.Equalf(t, 0, update.ExitCode, "pool update --start-universe/--start-address failed: stderr=%s", update.Stderr)

	planBytes, err := os.ReadFile(planPath)
	require.NoError(t, err, "read written plan")
	var view poolPlanView
	require.NoError(t, json.Unmarshal(planBytes, &view), "unmarshal plan")
	var addOps []struct {
		DependentKind    string `json:"dependent_kind"`
		Action           string `json:"action"`
		ProposedUniverse int    `json:"proposed_universe"`
		ProposedAddress  int    `json:"proposed_address"`
	}
	for _, op := range view.Operations {
		if op.DependentKind == "deployment_instance" && op.Action == "add" {
			addOps = append(addOps, op)
		}
	}
	require.Lenf(t, addOps, 2, "expected 2 proposed instances, got %d: %+v", len(addOps), addOps)
	require.Equalf(t, 3, addOps[0].ProposedUniverse, "expected the first proposed instance to anchor at (3, 50), got (%d, %d)", addOps[0].ProposedUniverse, addOps[0].ProposedAddress)
	require.Equalf(t, 50, addOps[0].ProposedAddress, "expected the first proposed instance to anchor at (3, 50), got (%d, %d)", addOps[0].ProposedUniverse, addOps[0].ProposedAddress)
	require.Equalf(t, 3, addOps[1].ProposedUniverse, "expected the second proposed instance to auto-increment to (3, 51), got (%d, %d)", addOps[1].ProposedUniverse, addOps[1].ProposedAddress)
	require.Equalf(t, 51, addOps[1].ProposedAddress, "expected the second proposed instance to auto-increment to (3, 51), got (%d, %d)", addOps[1].ProposedUniverse, addOps[1].ProposedAddress)
}

// TestPoolUpdateStartUniverseParseUsage proves a non-numeric
// --start-universe value is rejected as a usage error.
func TestPoolUpdateStartUniverseParseUsage(t *testing.T) {
	root := repositoryRoot(t)
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err, "NewDefaultCommandRegistry")

	showPath := filepath.Join(t.TempDir(), "show.json")
	poolName := seedPoolShowState(t, root, showPath)

	update := registry.Execute(command.Request{Root: root, Args: []string{
		"pool", "update", poolName,
		"--add", "acme/par64|sha256:99999999|Standard",
		"--start-universe", "notanumber",
		"--out", filepath.Join(t.TempDir(), "plan.json"),
		"--show", showPath,
	}})
	require.NotEqualf(t, 0, update.ExitCode, "expected GOLC_POOL_APPLY_USAGE for a non-numeric --start-universe, got stderr=%s", update.Stderr)
	require.Contains(t, string(update.Stderr), "GOLC_POOL_APPLY_USAGE")
}

// TestPoolUpdateAddChannelCountSpacing proves --add's optional trailing
// channel_count field spaces each newly proposed instance by that real
// width instead of the 1-channel fallback: three 5-channel --add specs
// starting at address 1 land at 1, 6, 11 -- not 1, 2, 3 (the exact
// multi-add address-collision bug this field closes).
func TestPoolUpdateAddChannelCountSpacing(t *testing.T) {
	root := repositoryRoot(t)
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err, "NewDefaultCommandRegistry")

	showPath := filepath.Join(t.TempDir(), "show.json")
	poolName, deploymentID := seedFreshPoolShowState(t, root, showPath)
	planPath := filepath.Join(t.TempDir(), "plan.json")

	update := registry.Execute(command.Request{Root: root, Args: []string{
		"pool", "update", poolName,
		"--add", "acme/colorband|sha256:aaaaaaaa|5ch|5",
		"--add", "acme/colorband|sha256:bbbbbbbb|5ch|5",
		"--add", "acme/colorband|sha256:cccccccc|5ch|5",
		"--attach-deployment", deploymentID,
		"--start-universe", "1",
		"--start-address", "1",
		"--out", planPath,
		"--show", showPath,
	}})
	require.Equalf(t, 0, update.ExitCode, "pool update --add with channel_count failed: stderr=%s", update.Stderr)

	planBytes, err := os.ReadFile(planPath)
	require.NoError(t, err, "read written plan")
	var view poolPlanView
	require.NoError(t, json.Unmarshal(planBytes, &view), "unmarshal plan")
	var addOps []struct {
		DependentKind    string `json:"dependent_kind"`
		Action           string `json:"action"`
		ProposedUniverse int    `json:"proposed_universe"`
		ProposedAddress  int    `json:"proposed_address"`
	}
	for _, op := range view.Operations {
		if op.DependentKind == "deployment_instance" && op.Action == "add" {
			addOps = append(addOps, op)
		}
	}
	require.Lenf(t, addOps, 3, "expected 3 proposed instances, got %d: %+v", len(addOps), addOps)
	wantAddresses := []int{1, 6, 11}
	for i, op := range addOps {
		require.Equalf(t, 1, op.ProposedUniverse, "expected 5-channel-spaced addresses %v, got op[%d]=(%d, %d)", wantAddresses, i, op.ProposedUniverse, op.ProposedAddress)
		require.Equalf(t, wantAddresses[i], op.ProposedAddress, "expected 5-channel-spaced addresses %v, got op[%d]=(%d, %d)", wantAddresses, i, op.ProposedUniverse, op.ProposedAddress)
	}
}

// TestPoolUpdateAddChannelCountUsage proves a non-numeric trailing
// channel_count field is rejected as a usage error rather than silently
// falling back to the 1-channel default.
func TestPoolUpdateAddChannelCountUsage(t *testing.T) {
	root := repositoryRoot(t)
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err, "NewDefaultCommandRegistry")

	showPath := filepath.Join(t.TempDir(), "show.json")
	poolName := seedPoolShowState(t, root, showPath)

	update := registry.Execute(command.Request{Root: root, Args: []string{
		"pool", "update", poolName,
		"--add", "acme/par64|sha256:dddddddd|Standard|notanumber",
		"--out", filepath.Join(t.TempDir(), "plan.json"),
		"--show", showPath,
	}})
	require.NotEqualf(t, 0, update.ExitCode, "expected GOLC_POOL_APPLY_USAGE for a non-numeric channel_count, got stderr=%s", update.Stderr)
	require.Contains(t, string(update.Stderr), "GOLC_POOL_APPLY_USAGE")
}
