// substitution_test.go proves the "pool substitute" (dry-run) route
// contract (02-06-PLAN.md, Task 1: POOL-06/POOL-07/POOL-08): it loads both
// fixture files plus the ShowState, builds a capability-diff substitution
// plan, writes it without mutating the ShowState file, and the resulting
// plan applies atomically through the already-existing "pool apply" route
// -- no second apply mechanism (D-16).
//
// It follows internal/command/poolimpact_test.go's repositoryRoot
// convention: production concern files are validated exactly as
// committed; the ShowState and fixture files themselves always live in an
// isolated t.TempDir(), so these tests never write into the real
// checkout.
//
// This file compiles against the already-implemented internal/command
// package but fails at RUN time until pool.go self-registers "pool
// substitute" (Task 3) -- that is the RED state this task proves.
package command_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lnorton89/golc/internal/command"
	"github.com/lnorton89/golc/internal/deployment"
	"github.com/lnorton89/golc/internal/pool"
	"github.com/lnorton89/golc/internal/show"
)

const substitutionFromFixtureYAML = `schema_version: 1
manufacturer: Acme
model: PAR64
modes:
  - name: Standard
    channels:
      - type: intensity
        occurrence: 0
capabilities:
  - type: intensity
    range: [0, 1]
`

const substitutionToFixtureYAML = `schema_version: 1
manufacturer: Beta
model: Spot300
modes:
  - name: Standard
    channels:
      - type: intensity
        occurrence: 0
capabilities:
  - type: intensity
    range: [0, 1]
`

// seedSubstitutionShowState builds and saves a minimal ShowState with one
// pool (one existing member pinned to "Acme/PAR64") and one active
// deployment already patched to that member, so "pool substitute" has a
// dependent to propose an operation against. It returns the pool's own
// Name (the CLI's own <pool> selector).
func seedSubstitutionShowState(t *testing.T, root, showPath string) string {
	t.Helper()

	p, err := pool.NewPool("Wash Pool", nil)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	member, err := pool.NewPoolMember("Acme/PAR64", "sha256:11111111")
	if err != nil {
		t.Fatalf("NewPoolMember: %v", err)
	}
	p.Members = append(p.Members, member)

	d, err := deployment.NewDeployment("Venue A")
	if err != nil {
		t.Fatalf("NewDeployment: %v", err)
	}
	d.Active = true
	d.Instances = append(d.Instances, deployment.Instance{
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

type substitutionPlanView struct {
	SchemaVersion int    `json:"schema_version"`
	PlanID        string `json:"plan_id"`
	Warnings      []struct {
		Code string `json:"code"`
	} `json:"warnings"`
	Errors []struct {
		Code string `json:"code"`
	} `json:"errors"`
	Add []struct {
		FixtureStableKey string `json:"fixture_stable_key"`
	} `json:"add"`
	Operations []struct {
		DependentKind string `json:"dependent_kind"`
		Action        string `json:"action"`
	} `json:"operations"`
}

func TestPoolSubstituteRoute(t *testing.T) {
	root := repositoryRoot(t)
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}

	tmp := t.TempDir()
	showPath := filepath.Join(tmp, "show.json")
	poolName := seedSubstitutionShowState(t, root, showPath)

	fromPath := filepath.Join(tmp, "from.yaml")
	if err := os.WriteFile(fromPath, []byte(substitutionFromFixtureYAML), 0o644); err != nil {
		t.Fatalf("write from fixture: %v", err)
	}
	toPath := filepath.Join(tmp, "to.yaml")
	if err := os.WriteFile(toPath, []byte(substitutionToFixtureYAML), 0o644); err != nil {
		t.Fatalf("write to fixture: %v", err)
	}
	planPath := filepath.Join(tmp, "substitution-plan.json")

	before, err := os.ReadFile(showPath)
	if err != nil {
		t.Fatalf("read seed show file: %v", err)
	}

	substitute := registry.Execute(command.Request{Root: root, Args: []string{
		"pool", "substitute", poolName,
		"--from", fromPath,
		"--to", toPath,
		"--out", planPath,
		"--show", showPath,
	}})
	if substitute.ExitCode != 0 {
		t.Fatalf("pool substitute failed: exit=%d stderr=%s", substitute.ExitCode, substitute.Stderr)
	}

	after, err := os.ReadFile(showPath)
	if err != nil {
		t.Fatalf("read show file after dry-run: %v", err)
	}
	if string(before) != string(after) {
		t.Fatal("expected pool substitute (dry-run) to leave the ShowState file byte-unchanged")
	}

	planBytes, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatalf("read written substitution plan: %v", err)
	}
	var view substitutionPlanView
	if err := json.Unmarshal(planBytes, &view); err != nil {
		t.Fatalf("unmarshal substitution plan: %v", err)
	}
	if view.PlanID == "" {
		t.Fatal("expected a non-empty plan_id")
	}
	if len(view.Errors) != 0 {
		t.Fatalf("expected no structural errors for a fully compatible substitution, got %+v", view.Errors)
	}
	if len(view.Add) != 1 || view.Add[0].FixtureStableKey != "Beta/Spot300" {
		t.Fatalf("expected the plan to propose adding the substituted fixture, got %+v", view.Add)
	}
	foundRemoveOp := false
	for _, op := range view.Operations {
		if op.DependentKind == "deployment_instance" && op.Action == "add" {
			foundRemoveOp = true
		}
	}
	if !foundRemoveOp {
		t.Fatalf("expected a proposed deployment_instance operation for the substituted member's dependent, got %+v", view.Operations)
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
	if len(applied.Pools) != 1 || len(applied.Pools[0].Members) != 1 {
		t.Fatalf("expected the pool to still carry exactly one member after substitution, got %+v", applied.Pools)
	}
	if applied.Pools[0].Members[0].FixtureStableKey != "Beta/Spot300" {
		t.Fatalf("expected the pool member to now be pinned to the substituted fixture, got %+v", applied.Pools[0].Members[0])
	}
}

func TestPoolSubstituteTargetInvalid(t *testing.T) {
	root := repositoryRoot(t)
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}

	tmp := t.TempDir()
	showPath := filepath.Join(tmp, "show.json")
	poolName := seedSubstitutionShowState(t, root, showPath)

	fromPath := filepath.Join(tmp, "from.yaml")
	if err := os.WriteFile(fromPath, []byte(substitutionFromFixtureYAML), 0o644); err != nil {
		t.Fatalf("write from fixture: %v", err)
	}
	// An invalid target file: zero declared capabilities fails
	// fixture.Decode's own strict validation before a plan can even be
	// built, surfacing at the route layer as GOLC_SUBSTITUTION_TARGET_INVALID
	// (T-02-14) rather than a bare GOLC_FIXTURE_* passthrough.
	invalidToPath := filepath.Join(tmp, "invalid-to.yaml")
	invalidToYAML := "schema_version: 1\nmanufacturer: Beta\nmodel: Spot300\nmodes:\n  - name: Standard\ncapabilities: []\n"
	if err := os.WriteFile(invalidToPath, []byte(invalidToYAML), 0o644); err != nil {
		t.Fatalf("write invalid to fixture: %v", err)
	}

	substitute := registry.Execute(command.Request{Root: root, Args: []string{
		"pool", "substitute", poolName,
		"--from", fromPath,
		"--to", invalidToPath,
		"--out", filepath.Join(tmp, "unused-plan.json"),
		"--show", showPath,
	}})
	if substitute.ExitCode == 0 {
		t.Fatalf("expected pool substitute to fail for an invalid target fixture, got exit=0")
	}
	if !strings.Contains(string(substitute.Stderr), "GOLC_SUBSTITUTION_TARGET_INVALID") {
		t.Fatalf("expected GOLC_SUBSTITUTION_TARGET_INVALID, got stderr=%s", substitute.Stderr)
	}
}
