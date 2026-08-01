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
	"testing"

	"github.com/stretchr/testify/require"

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
	require.NoError(t, err, "NewPool: %v", err)
	member, err := pool.NewPoolMember("Acme/PAR64", "sha256:11111111")
	require.NoError(t, err, "NewPoolMember: %v", err)
	p.Members = append(p.Members, member)

	d, err := deployment.NewDeployment("Venue A")
	require.NoError(t, err, "NewDeployment: %v", err)
	d.Active = true
	d.Instances = append(d.Instances, deployment.Instance{
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
	require.NoError(t, err, "NewDefaultCommandRegistry: %v", err)

	tmp := t.TempDir()
	showPath := filepath.Join(tmp, "show.json")
	poolName := seedSubstitutionShowState(t, root, showPath)

	fromPath := filepath.Join(tmp, "from.yaml")
	require.NoError(t, os.WriteFile(fromPath, []byte(substitutionFromFixtureYAML), 0o644), "write from fixture")
	toPath := filepath.Join(tmp, "to.yaml")
	require.NoError(t, os.WriteFile(toPath, []byte(substitutionToFixtureYAML), 0o644), "write to fixture")
	planPath := filepath.Join(tmp, "substitution-plan.json")

	before, err := os.ReadFile(showPath)
	require.NoError(t, err, "read seed show file: %v", err)

	substitute := registry.Execute(command.Request{Root: root, Args: []string{
		"pool", "substitute", poolName,
		"--from", fromPath,
		"--to", toPath,
		"--out", planPath,
		"--show", showPath,
	}})
	require.Equal(t, 0, substitute.ExitCode, "pool substitute failed: exit=%d stderr=%s", substitute.ExitCode, substitute.Stderr)

	after, err := os.ReadFile(showPath)
	require.NoError(t, err, "read show file after dry-run: %v", err)
	require.Equal(t, string(before), string(after), "expected pool substitute (dry-run) to leave the ShowState file byte-unchanged")

	planBytes, err := os.ReadFile(planPath)
	require.NoError(t, err, "read written substitution plan: %v", err)
	var view substitutionPlanView
	require.NoError(t, json.Unmarshal(planBytes, &view), "unmarshal substitution plan")
	require.NotEmpty(t, view.PlanID, "expected a non-empty plan_id")
	require.Empty(t, view.Errors, "expected no structural errors for a fully compatible substitution, got %+v", view.Errors)
	require.Len(t, view.Add, 1, "expected the plan to propose adding the substituted fixture, got %+v", view.Add)
	require.Equal(t, "Beta/Spot300", view.Add[0].FixtureStableKey, "expected the plan to propose adding the substituted fixture, got %+v", view.Add)
	foundRemoveOp := false
	for _, op := range view.Operations {
		if op.DependentKind == "deployment_instance" && op.Action == "add" {
			foundRemoveOp = true
		}
	}
	require.True(t, foundRemoveOp, "expected a proposed deployment_instance operation for the substituted member's dependent, got %+v", view.Operations)

	apply := registry.Execute(command.Request{Root: root, Args: []string{
		"pool", "apply", planPath, "--plan-id", view.PlanID, "--show", showPath,
	}})
	require.Equal(t, 0, apply.ExitCode, "pool apply failed: exit=%d stderr=%s", apply.ExitCode, apply.Stderr)

	applied, err := show.Load(root, showPath)
	require.NoError(t, err, "show.Load after apply: %v", err)
	require.Len(t, applied.Pools, 1, "expected the pool to still carry exactly one member after substitution, got %+v", applied.Pools)
	require.Len(t, applied.Pools[0].Members, 1, "expected the pool to still carry exactly one member after substitution, got %+v", applied.Pools)
	require.Equal(t, "Beta/Spot300", applied.Pools[0].Members[0].FixtureStableKey, "expected the pool member to now be pinned to the substituted fixture, got %+v", applied.Pools[0].Members[0])
}

func TestPoolSubstituteTargetInvalid(t *testing.T) {
	root := repositoryRoot(t)
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err, "NewDefaultCommandRegistry: %v", err)

	tmp := t.TempDir()
	showPath := filepath.Join(tmp, "show.json")
	poolName := seedSubstitutionShowState(t, root, showPath)

	fromPath := filepath.Join(tmp, "from.yaml")
	require.NoError(t, os.WriteFile(fromPath, []byte(substitutionFromFixtureYAML), 0o644), "write from fixture")
	// An invalid target file: zero declared capabilities fails
	// fixture.Decode's own strict validation before a plan can even be
	// built, surfacing at the route layer as GOLC_SUBSTITUTION_TARGET_INVALID
	// (T-02-14) rather than a bare GOLC_FIXTURE_* passthrough.
	invalidToPath := filepath.Join(tmp, "invalid-to.yaml")
	invalidToYAML := "schema_version: 1\nmanufacturer: Beta\nmodel: Spot300\nmodes:\n  - name: Standard\ncapabilities: []\n"
	require.NoError(t, os.WriteFile(invalidToPath, []byte(invalidToYAML), 0o644), "write invalid to fixture")

	substitute := registry.Execute(command.Request{Root: root, Args: []string{
		"pool", "substitute", poolName,
		"--from", fromPath,
		"--to", invalidToPath,
		"--out", filepath.Join(tmp, "unused-plan.json"),
		"--show", showPath,
	}})
	require.NotEqual(t, 0, substitute.ExitCode, "expected pool substitute to fail for an invalid target fixture, got exit=0")
	require.Contains(t, string(substitute.Stderr), "GOLC_SUBSTITUTION_TARGET_INVALID", "expected GOLC_SUBSTITUTION_TARGET_INVALID, got stderr=%s", substitute.Stderr)
}
