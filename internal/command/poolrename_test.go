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
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/command"
)

func TestPoolRenameRoute(t *testing.T) {
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err, "NewDefaultCommandRegistry")
	root := t.TempDir()
	showPath := "show.json"

	result := registry.Execute(command.Request{Root: root, Args: []string{"pool", "create", "Wash Pool", "--show", showPath}})
	require.Equal(t, 0, result.ExitCode, "pool create failed: stderr=%s", result.Stderr)

	rename := registry.Execute(command.Request{Root: root, Args: []string{"pool", "rename", "Wash Pool", "Wash Pool Renamed", "--show", showPath}})
	require.Equal(t, 0, rename.ExitCode, "pool rename failed: stderr=%s", rename.Stderr)
	require.Contains(t, string(rename.Stdout), "GOLC_POOL_RENAMED", "expected GOLC_POOL_RENAMED in stdout, got %s", rename.Stdout)

	inspect := registry.Execute(command.Request{Root: root, Args: []string{"show", "inspect", "--show", showPath}})
	require.Equal(t, 0, inspect.ExitCode, "show inspect failed: stderr=%s", inspect.Stderr)
	var view struct {
		Pools []struct {
			Name string `json:"name"`
		} `json:"pools"`
	}
	require.NoError(t, json.Unmarshal(inspect.Stdout, &view), "unmarshal show inspect")
	require.Len(t, view.Pools, 1, "expected exactly one pool named %q, got %+v", "Wash Pool Renamed", view.Pools)
	require.Equal(t, "Wash Pool Renamed", view.Pools[0].Name, "expected exactly one pool named %q, got %+v", "Wash Pool Renamed", view.Pools)

	unknown := registry.Execute(command.Request{Root: root, Args: []string{"pool", "rename", "Nonexistent", "New Name", "--show", showPath}})
	require.NotEqual(t, 0, unknown.ExitCode, "expected GOLC_POOL_NOT_FOUND, got exit=%d stderr=%s", unknown.ExitCode, unknown.Stderr)
	require.Contains(t, string(unknown.Stderr), "GOLC_POOL_NOT_FOUND", "expected GOLC_POOL_NOT_FOUND, got exit=%d stderr=%s", unknown.ExitCode, unknown.Stderr)

	result = registry.Execute(command.Request{Root: root, Args: []string{"pool", "create", "Second Pool", "--show", showPath}})
	require.Equal(t, 0, result.ExitCode, "pool create (second) failed: stderr=%s", result.Stderr)
	collide := registry.Execute(command.Request{Root: root, Args: []string{"pool", "rename", "Second Pool", "Wash Pool Renamed", "--show", showPath}})
	require.NotEqual(t, 0, collide.ExitCode, "expected GOLC_POOL_DUPLICATE_NAME for a colliding rename, got exit=%d stderr=%s", collide.ExitCode, collide.Stderr)
	require.Contains(t, string(collide.Stderr), "GOLC_POOL_DUPLICATE_NAME", "expected GOLC_POOL_DUPLICATE_NAME for a colliding rename, got exit=%d stderr=%s", collide.ExitCode, collide.Stderr)
}

func TestPoolDeleteRoute(t *testing.T) {
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err, "NewDefaultCommandRegistry")
	// "pool update" (used below to add a member) resolves
	// application_defaults.pool_update_review through the real project
	// config when --propagate is omitted, so root must be the real
	// checkout root (mirrors poolimpact_test.go's repositoryRoot(t)
	// pattern) -- the ShowState file itself still lives in an isolated
	// t.TempDir().
	root := repositoryRoot(t)
	showPath := filepath.Join(t.TempDir(), "show.json")

	result := registry.Execute(command.Request{Root: root, Args: []string{"pool", "create", "Wash Pool", "--show", showPath}})
	require.Equal(t, 0, result.ExitCode, "pool create failed: stderr=%s", result.Stderr)
	result = registry.Execute(command.Request{Root: root, Args: []string{"deployment", "create", "Venue A", "--show", showPath}})
	require.Equal(t, 0, result.ExitCode, "deployment create failed: stderr=%s", result.Stderr)
	result = registry.Execute(command.Request{Root: root, Args: []string{"deployment", "activate", "Venue A", "--show", showPath}})
	require.Equal(t, 0, result.ExitCode, "deployment activate failed: stderr=%s", result.Stderr)

	// Look up the active deployment's ID to force-attach the first member
	// (closing the "adopt a never-before-used pool" gap, same as
	// AddPoolMembersPreview's own flow).
	inspectBefore := registry.Execute(command.Request{Root: root, Args: []string{"show", "inspect", "--show", showPath}})
	require.Equal(t, 0, inspectBefore.ExitCode, "show inspect (before) failed: stderr=%s", inspectBefore.Stderr)
	var before struct {
		Deployments []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"deployments"`
	}
	require.NoError(t, json.Unmarshal(inspectBefore.Stdout, &before), "unmarshal show inspect (before)")
	require.Len(t, before.Deployments, 1, "expected exactly one deployment, got %+v", before.Deployments)
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
	require.Equal(t, 0, update.ExitCode, "pool update failed: stderr=%s", update.Stderr)
	planBytes, err := os.ReadFile(planPath)
	require.NoError(t, err, "read written plan")
	var plan struct {
		PlanID string `json:"plan_id"`
	}
	require.NoError(t, json.Unmarshal(planBytes, &plan), "unmarshal plan")
	apply := registry.Execute(command.Request{Root: root, Args: []string{"pool", "apply", planPath, "--plan-id", plan.PlanID, "--show", showPath}})
	require.Equal(t, 0, apply.ExitCode, "pool apply failed: stderr=%s", apply.Stderr)

	inspectAfterApply := registry.Execute(command.Request{Root: root, Args: []string{"show", "inspect", "--show", showPath}})
	require.Equal(t, 0, inspectAfterApply.ExitCode, "show inspect (after apply) failed: stderr=%s", inspectAfterApply.Stderr)
	var afterApply struct {
		Deployments []struct {
			InstanceCount int `json:"instance_count"`
		} `json:"deployments"`
	}
	require.NoError(t, json.Unmarshal(inspectAfterApply.Stdout, &afterApply), "unmarshal show inspect (after apply)")
	require.Len(t, afterApply.Deployments, 1, "expected the deployment to have gained exactly one instance before delete, got %+v", afterApply.Deployments)
	require.Equal(t, 1, afterApply.Deployments[0].InstanceCount, "expected the deployment to have gained exactly one instance before delete, got %+v", afterApply.Deployments)

	deleteResult := registry.Execute(command.Request{Root: root, Args: []string{"pool", "delete", "Wash Pool", "--show", showPath}})
	require.Equal(t, 0, deleteResult.ExitCode, "pool delete failed: stderr=%s", deleteResult.Stderr)
	require.Contains(t, string(deleteResult.Stdout), "GOLC_POOL_DELETED", "expected GOLC_POOL_DELETED in stdout, got %s", deleteResult.Stdout)

	inspectAfter := registry.Execute(command.Request{Root: root, Args: []string{"show", "inspect", "--show", showPath}})
	require.Equal(t, 0, inspectAfter.ExitCode, "show inspect (after) failed: stderr=%s", inspectAfter.Stderr)
	var after struct {
		Pools []struct {
			Name string `json:"name"`
		} `json:"pools"`
		Deployments []struct {
			Name          string `json:"name"`
			InstanceCount int    `json:"instance_count"`
		} `json:"deployments"`
	}
	require.NoError(t, json.Unmarshal(inspectAfter.Stdout, &after), "unmarshal show inspect (after)")
	require.Empty(t, after.Pools, "expected zero pools after delete, got %+v", after.Pools)
	require.Len(t, after.Deployments, 1, "expected the deployment to survive with zero instances, got %+v", after.Deployments)
	require.Equal(t, 0, after.Deployments[0].InstanceCount, "expected the deployment to survive with zero instances, got %+v", after.Deployments)

	unknown := registry.Execute(command.Request{Root: root, Args: []string{"pool", "delete", "Nonexistent", "--show", showPath}})
	require.NotEqual(t, 0, unknown.ExitCode, "expected GOLC_POOL_NOT_FOUND, got exit=%d stderr=%s", unknown.ExitCode, unknown.Stderr)
	require.Contains(t, string(unknown.Stderr), "GOLC_POOL_NOT_FOUND", "expected GOLC_POOL_NOT_FOUND, got exit=%d stderr=%s", unknown.ExitCode, unknown.Stderr)
}
