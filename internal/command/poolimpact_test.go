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
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "golc.project.toml")); err != nil {
		t.Fatalf("repository root %q has no golc.project.toml: %v", root, err)
	}
	return root
}

// seedPoolShowState builds and saves a minimal ShowState with one pool
// (one existing member) and one active deployment already patched to that
// member, so "pool update --add" has a dependent to propose an instance
// against. It returns the pool's own Name (the CLI's own <pool> selector).
func seedPoolShowState(t *testing.T, root, showPath string) string {
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
		DependentKind string `json:"dependent_kind"`
		Action        string `json:"action"`
	} `json:"operations"`
}

func TestPoolUpdateApplyRoutes(t *testing.T) {
	root := repositoryRoot(t)
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}

	showPath := filepath.Join(t.TempDir(), "show.json")
	poolName := seedPoolShowState(t, root, showPath)
	planPath := filepath.Join(t.TempDir(), "plan.json")

	before, err := os.ReadFile(showPath)
	if err != nil {
		t.Fatalf("read seed show file: %v", err)
	}

	update := registry.Execute(command.Request{Root: root, Args: []string{
		"pool", "update", poolName,
		"--add", "acme/par64|sha256:22222222|Standard",
		"--out", planPath,
		"--show", showPath,
	}})
	if update.ExitCode != 0 {
		t.Fatalf("pool update failed: exit=%d stderr=%s", update.ExitCode, update.Stderr)
	}

	after, err := os.ReadFile(showPath)
	if err != nil {
		t.Fatalf("read show file after dry-run: %v", err)
	}
	if string(before) != string(after) {
		t.Fatal("expected pool update (dry-run) to leave the ShowState file byte-unchanged")
	}

	planBytes, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatalf("read written plan: %v", err)
	}
	var view poolPlanView
	if err := json.Unmarshal(planBytes, &view); err != nil {
		t.Fatalf("unmarshal plan: %v", err)
	}
	if view.PlanID == "" {
		t.Fatal("expected a non-empty plan_id")
	}
	if len(view.Add) != 1 || view.Add[0].FixtureStableKey != "acme/par64" {
		t.Fatalf("expected the plan to carry the requested add spec, got %+v", view.Add)
	}
	foundAddOp := false
	for _, op := range view.Operations {
		if op.DependentKind == "deployment_instance" && op.Action == "add" {
			foundAddOp = true
		}
	}
	if !foundAddOp {
		t.Fatalf("expected a proposed deployment_instance add operation, got %+v", view.Operations)
	}

	apply := registry.Execute(command.Request{Root: root, Args: []string{
		"pool", "apply", planPath, "--plan-id", view.PlanID, "--show", showPath,
	}})
	if apply.ExitCode != 0 {
		t.Fatalf("pool apply failed: exit=%d stderr=%s", apply.ExitCode, apply.Stderr)
	}

	applied, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load after apply: %v", err)
	}
	if len(applied.Pools) != 1 || len(applied.Pools[0].Members) != 2 {
		t.Fatalf("expected the pool to gain the new member, got %+v", applied.Pools)
	}
	if len(applied.Deployments) != 1 || len(applied.Deployments[0].Instances) != 2 {
		t.Fatalf("expected the deployment to gain the proposed instance, got %+v", applied.Deployments)
	}

	// A stale re-apply of the exact same plan file is rejected (single-use):
	// the ShowState revision moved when the first apply saved.
	staleApply := registry.Execute(command.Request{Root: root, Args: []string{
		"pool", "apply", planPath, "--plan-id", view.PlanID, "--show", showPath,
	}})
	if staleApply.ExitCode == 0 || !strings.Contains(string(staleApply.Stderr), "GOLC_POOL_PLAN_STALE") {
		t.Fatalf("expected GOLC_POOL_PLAN_STALE for a stale re-apply, got exit=%d stderr=%s", staleApply.ExitCode, staleApply.Stderr)
	}

	// A tampered plan file (bytes altered after hashing) is rejected by
	// the integrity gate before freshness is even considered.
	tamperedPath := filepath.Join(t.TempDir(), "tampered-plan.json")
	tampered := strings.Replace(string(planBytes), "\"preview\"", "\"immediate\"", 1)
	if tampered == string(planBytes) {
		t.Fatal("expected the tamper substitution to change the plan bytes")
	}
	if err := os.WriteFile(tamperedPath, []byte(tampered), 0o644); err != nil {
		t.Fatalf("write tampered plan: %v", err)
	}
	tamperedApply := registry.Execute(command.Request{Root: root, Args: []string{
		"pool", "apply", tamperedPath, "--plan-id", view.PlanID, "--show", showPath,
	}})
	if tamperedApply.ExitCode == 0 || !strings.Contains(string(tamperedApply.Stderr), "GOLC_POOL_PLAN_HASH") {
		t.Fatalf("expected GOLC_POOL_PLAN_HASH for a tampered plan, got exit=%d stderr=%s", tamperedApply.ExitCode, tamperedApply.Stderr)
	}
}

func TestPropagationDefaultReview(t *testing.T) {
	root := repositoryRoot(t)
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}

	showPath := filepath.Join(t.TempDir(), "show.json")
	poolName := seedPoolShowState(t, root, showPath)

	defaultPlanPath := filepath.Join(t.TempDir(), "default-plan.json")
	defaultUpdate := registry.Execute(command.Request{Root: root, Args: []string{
		"pool", "update", poolName,
		"--add", "acme/par64|sha256:33333333|Standard",
		"--out", defaultPlanPath,
		"--show", showPath,
	}})
	if defaultUpdate.ExitCode != 0 {
		t.Fatalf("pool update (no --propagate) failed: exit=%d stderr=%s", defaultUpdate.ExitCode, defaultUpdate.Stderr)
	}
	var defaultView poolPlanView
	defaultBytes, err := os.ReadFile(defaultPlanPath)
	if err != nil {
		t.Fatalf("read default plan: %v", err)
	}
	if err := json.Unmarshal(defaultBytes, &defaultView); err != nil {
		t.Fatalf("unmarshal default plan: %v", err)
	}
	if defaultView.Propagate != "preview" {
		t.Fatalf("expected the unset propagation default to resolve to review-required (preview), got %q", defaultView.Propagate)
	}

	immediatePlanPath := filepath.Join(t.TempDir(), "immediate-plan.json")
	immediateUpdate := registry.Execute(command.Request{Root: root, Args: []string{
		"pool", "update", poolName,
		"--add", "acme/par64|sha256:44444444|Standard",
		"--propagate", "immediate",
		"--out", immediatePlanPath,
		"--show", showPath,
	}})
	if immediateUpdate.ExitCode != 0 {
		t.Fatalf("pool update (--propagate immediate) failed: exit=%d stderr=%s", immediateUpdate.ExitCode, immediateUpdate.Stderr)
	}
	var immediateView poolPlanView
	immediateBytes, err := os.ReadFile(immediatePlanPath)
	if err != nil {
		t.Fatalf("read immediate plan: %v", err)
	}
	if err := json.Unmarshal(immediateBytes, &immediateView); err != nil {
		t.Fatalf("unmarshal immediate plan: %v", err)
	}
	if immediateView.Propagate != "immediate" {
		t.Fatalf("expected --propagate immediate to override the default, got %q", immediateView.Propagate)
	}

	invalid := registry.Execute(command.Request{Root: root, Args: []string{
		"pool", "update", poolName,
		"--propagate", "bogus",
		"--out", filepath.Join(t.TempDir(), "bogus-plan.json"),
		"--show", showPath,
	}})
	if invalid.ExitCode == 0 || !strings.Contains(string(invalid.Stderr), "GOLC_POOL_APPLY_USAGE") {
		t.Fatalf("expected GOLC_POOL_APPLY_USAGE for an invalid --propagate value, got exit=%d stderr=%s", invalid.ExitCode, invalid.Stderr)
	}
}
