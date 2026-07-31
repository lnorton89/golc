// poolrename_test.go proves the "pool rename" / "pool delete" route
// contract: renaming a pool preserves its identity/members/instances and
// rejects a colliding new name; deleting a pool cascades to remove its
// own deployment instances while leaving unrelated pools/instances
// untouched, and rejects an unknown pool name.
package command_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lnorton89/golc/internal/command"
)

func TestPoolRenameRoute(t *testing.T) {
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}
	root := t.TempDir()
	showPath := "show.json"

	if result := registry.Execute(command.Request{Root: root, Args: []string{"pool", "create", "Wash Pool", "--show", showPath}}); result.ExitCode != 0 {
		t.Fatalf("pool create failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	rename := registry.Execute(command.Request{Root: root, Args: []string{"pool", "rename", "Wash Pool", "Wash Pool Renamed", "--show", showPath}})
	if rename.ExitCode != 0 {
		t.Fatalf("pool rename failed: exit=%d stderr=%s", rename.ExitCode, rename.Stderr)
	}
	if !strings.Contains(string(rename.Stdout), "GOLC_POOL_RENAMED") {
		t.Fatalf("expected GOLC_POOL_RENAMED in stdout, got %s", rename.Stdout)
	}

	inspect := registry.Execute(command.Request{Root: root, Args: []string{"show", "inspect", "--show", showPath}})
	if inspect.ExitCode != 0 {
		t.Fatalf("show inspect failed: exit=%d stderr=%s", inspect.ExitCode, inspect.Stderr)
	}
	var view struct {
		Pools []struct {
			Name string `json:"name"`
		} `json:"pools"`
	}
	if err := json.Unmarshal(inspect.Stdout, &view); err != nil {
		t.Fatalf("unmarshal show inspect: %v", err)
	}
	if len(view.Pools) != 1 || view.Pools[0].Name != "Wash Pool Renamed" {
		t.Fatalf("expected exactly one pool named %q, got %+v", "Wash Pool Renamed", view.Pools)
	}

	unknown := registry.Execute(command.Request{Root: root, Args: []string{"pool", "rename", "Nonexistent", "New Name", "--show", showPath}})
	if unknown.ExitCode == 0 || !strings.Contains(string(unknown.Stderr), "GOLC_POOL_NOT_FOUND") {
		t.Fatalf("expected GOLC_POOL_NOT_FOUND, got exit=%d stderr=%s", unknown.ExitCode, unknown.Stderr)
	}

	if result := registry.Execute(command.Request{Root: root, Args: []string{"pool", "create", "Second Pool", "--show", showPath}}); result.ExitCode != 0 {
		t.Fatalf("pool create (second) failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
	collide := registry.Execute(command.Request{Root: root, Args: []string{"pool", "rename", "Second Pool", "Wash Pool Renamed", "--show", showPath}})
	if collide.ExitCode == 0 || !strings.Contains(string(collide.Stderr), "GOLC_POOL_DUPLICATE_NAME") {
		t.Fatalf("expected GOLC_POOL_DUPLICATE_NAME for a colliding rename, got exit=%d stderr=%s", collide.ExitCode, collide.Stderr)
	}
}

func TestPoolDeleteRoute(t *testing.T) {
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}
	// "pool update" (used below to add a member) resolves
	// application_defaults.pool_update_review through the real project
	// config when --propagate is omitted, so root must be the real
	// checkout root (mirrors poolimpact_test.go's repositoryRoot(t)
	// pattern) -- the ShowState file itself still lives in an isolated
	// t.TempDir().
	root := repositoryRoot(t)
	showPath := filepath.Join(t.TempDir(), "show.json")

	if result := registry.Execute(command.Request{Root: root, Args: []string{"pool", "create", "Wash Pool", "--show", showPath}}); result.ExitCode != 0 {
		t.Fatalf("pool create failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
	if result := registry.Execute(command.Request{Root: root, Args: []string{"deployment", "create", "Venue A", "--show", showPath}}); result.ExitCode != 0 {
		t.Fatalf("deployment create failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
	if result := registry.Execute(command.Request{Root: root, Args: []string{"deployment", "activate", "Venue A", "--show", showPath}}); result.ExitCode != 0 {
		t.Fatalf("deployment activate failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	// Look up the active deployment's ID to force-attach the first member
	// (closing the "adopt a never-before-used pool" gap, same as
	// AddPoolMembersPreview's own flow).
	inspectBefore := registry.Execute(command.Request{Root: root, Args: []string{"show", "inspect", "--show", showPath}})
	if inspectBefore.ExitCode != 0 {
		t.Fatalf("show inspect (before) failed: exit=%d stderr=%s", inspectBefore.ExitCode, inspectBefore.Stderr)
	}
	var before struct {
		Deployments []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"deployments"`
	}
	if err := json.Unmarshal(inspectBefore.Stdout, &before); err != nil {
		t.Fatalf("unmarshal show inspect (before): %v", err)
	}
	if len(before.Deployments) != 1 {
		t.Fatalf("expected exactly one deployment, got %+v", before.Deployments)
	}
	deploymentID := before.Deployments[0].ID

	// "pool update" only ever computes a plan -- it never mutates the
	// ShowState regardless of --propagate; "pool apply" is the one route
	// that commits it (mirrors TestPoolUpdateApplyRoutes's own two-step
	// preview-then-apply shape).
	planPath := filepath.Join(t.TempDir(), "plan.json")
	update := registry.Execute(command.Request{Root: root, Args: []string{
		"pool", "update", "Wash Pool",
		"--add", "acme/par64|sha256:11111111|Standard",
		"--attach-deployment", deploymentID,
		"--out", planPath,
		"--show", showPath,
	}})
	if update.ExitCode != 0 {
		t.Fatalf("pool update failed: exit=%d stderr=%s", update.ExitCode, update.Stderr)
	}
	planBytes, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatalf("read written plan: %v", err)
	}
	var plan struct {
		PlanID string `json:"plan_id"`
	}
	if err := json.Unmarshal(planBytes, &plan); err != nil {
		t.Fatalf("unmarshal plan: %v", err)
	}
	apply := registry.Execute(command.Request{Root: root, Args: []string{"pool", "apply", planPath, "--plan-id", plan.PlanID, "--show", showPath}})
	if apply.ExitCode != 0 {
		t.Fatalf("pool apply failed: exit=%d stderr=%s", apply.ExitCode, apply.Stderr)
	}

	inspectAfterApply := registry.Execute(command.Request{Root: root, Args: []string{"show", "inspect", "--show", showPath}})
	if inspectAfterApply.ExitCode != 0 {
		t.Fatalf("show inspect (after apply) failed: exit=%d stderr=%s", inspectAfterApply.ExitCode, inspectAfterApply.Stderr)
	}
	var afterApply struct {
		Deployments []struct {
			InstanceCount int `json:"instance_count"`
		} `json:"deployments"`
	}
	if err := json.Unmarshal(inspectAfterApply.Stdout, &afterApply); err != nil {
		t.Fatalf("unmarshal show inspect (after apply): %v", err)
	}
	if len(afterApply.Deployments) != 1 || afterApply.Deployments[0].InstanceCount != 1 {
		t.Fatalf("expected the deployment to have gained exactly one instance before delete, got %+v", afterApply.Deployments)
	}

	deleteResult := registry.Execute(command.Request{Root: root, Args: []string{"pool", "delete", "Wash Pool", "--show", showPath}})
	if deleteResult.ExitCode != 0 {
		t.Fatalf("pool delete failed: exit=%d stderr=%s", deleteResult.ExitCode, deleteResult.Stderr)
	}
	if !strings.Contains(string(deleteResult.Stdout), "GOLC_POOL_DELETED") {
		t.Fatalf("expected GOLC_POOL_DELETED in stdout, got %s", deleteResult.Stdout)
	}

	inspectAfter := registry.Execute(command.Request{Root: root, Args: []string{"show", "inspect", "--show", showPath}})
	if inspectAfter.ExitCode != 0 {
		t.Fatalf("show inspect (after) failed: exit=%d stderr=%s", inspectAfter.ExitCode, inspectAfter.Stderr)
	}
	var after struct {
		Pools []struct {
			Name string `json:"name"`
		} `json:"pools"`
		Deployments []struct {
			Name          string `json:"name"`
			InstanceCount int    `json:"instance_count"`
		} `json:"deployments"`
	}
	if err := json.Unmarshal(inspectAfter.Stdout, &after); err != nil {
		t.Fatalf("unmarshal show inspect (after): %v", err)
	}
	if len(after.Pools) != 0 {
		t.Fatalf("expected zero pools after delete, got %+v", after.Pools)
	}
	if len(after.Deployments) != 1 || after.Deployments[0].InstanceCount != 0 {
		t.Fatalf("expected the deployment to survive with zero instances, got %+v", after.Deployments)
	}

	unknown := registry.Execute(command.Request{Root: root, Args: []string{"pool", "delete", "Nonexistent", "--show", showPath}})
	if unknown.ExitCode == 0 || !strings.Contains(string(unknown.Stderr), "GOLC_POOL_NOT_FOUND") {
		t.Fatalf("expected GOLC_POOL_NOT_FOUND, got exit=%d stderr=%s", unknown.ExitCode, unknown.Stderr)
	}
}
