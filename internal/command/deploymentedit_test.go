// deploymentedit_test.go proves the "deployment rename" / "deployment
// delete" / "deployment instance reassign" route contract: renaming
// preserves identity and rejects a colliding name; deleting removes the
// deployment (including deleting the active one, leaving zero active
// without error); reassigning updates mode/universe/address in place,
// omitted flags keep the current value, and a colliding address is
// rejected.
package command_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/lnorton89/golc/internal/command"
	"github.com/lnorton89/golc/internal/show"
	"github.com/stretchr/testify/require"
)

// loadShowStateForTest loads the ShowState directly (bypassing the
// allowlisted "show inspect" projection, which never exposes an
// instance's own UUID) so a test can resolve the concrete instance IDs
// "deployment instance reassign" needs.
func loadShowStateForTest(root, showPath string) (show.State, error) {
	return show.Load(root, showPath)
}

func intToStr(n int) string {
	return strconv.Itoa(n)
}

func TestDeploymentRenameRoute(t *testing.T) {
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err)
	root := t.TempDir()
	showPath := "show.json"

	if result := registry.Execute(command.Request{Root: root, Args: []string{"deployment", "create", "Venue A", "--show", showPath}}); result.ExitCode != 0 {
		require.Equal(t, 0, result.ExitCode, "deployment create failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	rename := registry.Execute(command.Request{Root: root, Args: []string{"deployment", "rename", "Venue A", "Venue A Renamed", "--show", showPath}})
	require.Equal(t, 0, rename.ExitCode, "deployment rename failed: exit=%d stderr=%s", rename.ExitCode, rename.Stderr)
	require.Contains(t, string(rename.Stdout), "GOLC_DEPLOYMENT_RENAMED", "expected GOLC_DEPLOYMENT_RENAMED in stdout, got %s", rename.Stdout)

	inspect := registry.Execute(command.Request{Root: root, Args: []string{"show", "inspect", "--show", showPath}})
	require.Equal(t, 0, inspect.ExitCode, "show inspect failed: exit=%d stderr=%s", inspect.ExitCode, inspect.Stderr)
	var view struct {
		Deployments []struct {
			Name string `json:"name"`
		} `json:"deployments"`
	}
	if err := json.Unmarshal(inspect.Stdout, &view); err != nil {
		require.NoError(t, err)
	}
	require.Len(t, view.Deployments, 1)
	require.Equal(t, "Venue A Renamed", view.Deployments[0].Name, "expected exactly one deployment named %q, got %+v", "Venue A Renamed", view.Deployments)

	unknown := registry.Execute(command.Request{Root: root, Args: []string{"deployment", "rename", "Nonexistent", "New Name", "--show", showPath}})
	require.NotEqual(t, 0, unknown.ExitCode)
	require.Contains(t, string(unknown.Stderr), "GOLC_DEPLOYMENT_NOT_FOUND", "expected GOLC_DEPLOYMENT_NOT_FOUND, got exit=%d stderr=%s", unknown.ExitCode, unknown.Stderr)

	if result := registry.Execute(command.Request{Root: root, Args: []string{"deployment", "create", "Venue B", "--show", showPath}}); result.ExitCode != 0 {
		require.Equal(t, 0, result.ExitCode, "deployment create (second) failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
	collide := registry.Execute(command.Request{Root: root, Args: []string{"deployment", "rename", "Venue B", "Venue A Renamed", "--show", showPath}})
	require.NotEqual(t, 0, collide.ExitCode)
	require.Contains(t, string(collide.Stderr), "GOLC_DEPLOYMENT_DUPLICATE_NAME", "expected GOLC_DEPLOYMENT_DUPLICATE_NAME for a colliding rename, got exit=%d stderr=%s", collide.ExitCode, collide.Stderr)
}

func TestDeploymentDeleteRoute(t *testing.T) {
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err)
	root := t.TempDir()
	showPath := "show.json"

	if result := registry.Execute(command.Request{Root: root, Args: []string{"deployment", "create", "Venue A", "--show", showPath}}); result.ExitCode != 0 {
		require.Equal(t, 0, result.ExitCode, "deployment create failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
	if result := registry.Execute(command.Request{Root: root, Args: []string{"deployment", "activate", "Venue A", "--show", showPath}}); result.ExitCode != 0 {
		require.Equal(t, 0, result.ExitCode, "deployment activate failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	deleteResult := registry.Execute(command.Request{Root: root, Args: []string{"deployment", "delete", "Venue A", "--show", showPath}})
	require.Equal(t, 0, deleteResult.ExitCode, "deployment delete failed: exit=%d stderr=%s", deleteResult.ExitCode, deleteResult.Stderr)
	require.Contains(t, string(deleteResult.Stdout), "GOLC_DEPLOYMENT_DELETED", "expected GOLC_DEPLOYMENT_DELETED in stdout, got %s", deleteResult.Stdout)

	inspect := registry.Execute(command.Request{Root: root, Args: []string{"show", "inspect", "--show", showPath}})
	require.Equal(t, 0, inspect.ExitCode, "show inspect failed (deleting the active deployment must leave a valid, zero-active state): exit=%d stderr=%s", inspect.ExitCode, inspect.Stderr)
	var view struct {
		Deployments []struct {
			Name string `json:"name"`
		} `json:"deployments"`
	}
	if err := json.Unmarshal(inspect.Stdout, &view); err != nil {
		require.NoError(t, err)
	}
	require.Len(t, view.Deployments, 0, "expected zero deployments after delete, got %+v", view.Deployments)

	unknown := registry.Execute(command.Request{Root: root, Args: []string{"deployment", "delete", "Nonexistent", "--show", showPath}})
	require.NotEqual(t, 0, unknown.ExitCode)
	require.Contains(t, string(unknown.Stderr), "GOLC_DEPLOYMENT_NOT_FOUND", "expected GOLC_DEPLOYMENT_NOT_FOUND, got exit=%d stderr=%s", unknown.ExitCode, unknown.Stderr)
}

func TestDeploymentInstanceReassignRoute(t *testing.T) {
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err)
	// "pool update" (used below to add members) resolves
	// application_defaults.pool_update_review through the real project
	// config when --propagate is omitted, so root must be the real
	// checkout root (mirrors poolimpact_test.go's repositoryRoot(t)
	// pattern) -- the ShowState file itself still lives in an isolated
	// t.TempDir().
	root := repositoryRoot(t)
	showPath := filepath.Join(t.TempDir(), "show.json")

	if result := registry.Execute(command.Request{Root: root, Args: []string{"pool", "create", "Wash Pool", "--show", showPath}}); result.ExitCode != 0 {
		require.Equal(t, 0, result.ExitCode, "pool create failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
	if result := registry.Execute(command.Request{Root: root, Args: []string{"deployment", "create", "Venue A", "--show", showPath}}); result.ExitCode != 0 {
		require.Equal(t, 0, result.ExitCode, "deployment create failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
	if result := registry.Execute(command.Request{Root: root, Args: []string{"deployment", "activate", "Venue A", "--show", showPath}}); result.ExitCode != 0 {
		require.Equal(t, 0, result.ExitCode, "deployment activate failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	inspectBefore := registry.Execute(command.Request{Root: root, Args: []string{"show", "inspect", "--show", showPath}})
	require.Equal(t, 0, inspectBefore.ExitCode, "show inspect (before) failed: exit=%d stderr=%s", inspectBefore.ExitCode, inspectBefore.Stderr)
	var before struct {
		Deployments []struct {
			ID string `json:"id"`
		} `json:"deployments"`
	}
	if err := json.Unmarshal(inspectBefore.Stdout, &before); err != nil {
		require.NoError(t, err)
	}
	deploymentID := before.Deployments[0].ID

	// Add two units so a real collision target exists. "pool update" only
	// computes a plan -- "pool apply" is what actually commits it (mirrors
	// TestPoolUpdateApplyRoutes's two-step preview-then-apply shape).
	planPath := filepath.Join(t.TempDir(), "plan.json")
	update := registry.Execute(command.Request{Root: root, Args: []string{
		"pool", "update", "Wash Pool",
		"--add", "acme/par64|sha256:11111111|Standard",
		"--add", "acme/par64|sha256:22222222|Standard",
		"--attach-deployment", deploymentID,
		"--out", planPath,
		"--show", showPath,
	}})
	require.Equal(t, 0, update.ExitCode, "pool update failed: exit=%d stderr=%s", update.ExitCode, update.Stderr)
	planBytes, err := os.ReadFile(planPath)
	require.NoError(t, err)
	var plan struct {
		PlanID string `json:"plan_id"`
	}
	if err := json.Unmarshal(planBytes, &plan); err != nil {
		require.NoError(t, err)
	}
	apply := registry.Execute(command.Request{Root: root, Args: []string{"pool", "apply", planPath, "--plan-id", plan.PlanID, "--show", showPath}})
	require.Equal(t, 0, apply.ExitCode, "pool apply failed: exit=%d stderr=%s", apply.ExitCode, apply.Stderr)

	// Resolve the two concrete instance IDs by loading the ShowState
	// directly -- "show inspect" only ever projects an allowlisted
	// instance_count, never per-instance identifiers.
	state, err := loadShowStateForTest(root, showPath)
	require.NoError(t, err)
	require.Len(t, state.Deployments, 1)
	require.Len(t, state.Deployments[0].Instances, 2, "expected exactly one deployment with 2 instances, got %+v", state.Deployments)
	firstInstanceID := state.Deployments[0].Instances[0].ID.String()
	secondInstanceID := state.Deployments[0].Instances[1].ID.String()
	secondUniverse := state.Deployments[0].Instances[1].Universe
	secondAddress := state.Deployments[0].Instances[1].Address

	// Happy path: reassign only the mode, leaving universe/address (omitted
	// flags keep the current value).
	reassign := registry.Execute(command.Request{Root: root, Args: []string{
		"deployment", "instance", "reassign", "Venue A", firstInstanceID,
		"--mode", "Extended",
		"--show", showPath,
	}})
	require.Equal(t, 0, reassign.ExitCode, "deployment instance reassign (mode only) failed: exit=%d stderr=%s", reassign.ExitCode, reassign.Stderr)
	require.Contains(t, string(reassign.Stdout), "GOLC_DEPLOYMENT_INSTANCE_REASSIGNED", "expected GOLC_DEPLOYMENT_INSTANCE_REASSIGNED in stdout, got %s", reassign.Stdout)

	state, err = loadShowStateForTest(root, showPath)
	require.NoError(t, err)
	require.Equal(t, "Extended", state.Deployments[0].Instances[0].Mode, "expected mode to update to Extended, got %+v", state.Deployments[0].Instances[0])
	require.NotEqual(t, 0, state.Deployments[0].Instances[0].Universe)
	require.NotEqual(t, 0, state.Deployments[0].Instances[0].Address, "expected universe/address to be preserved (omitted flags), got %+v", state.Deployments[0].Instances[0])

	// Collision: reassign the first instance onto the second instance's
	// exact universe/address.
	collide := registry.Execute(command.Request{Root: root, Args: []string{
		"deployment", "instance", "reassign", "Venue A", firstInstanceID,
		"--universe", intToStr(secondUniverse),
		"--address", intToStr(secondAddress),
		"--show", showPath,
	}})
	require.NotEqual(t, 0, collide.ExitCode)
	require.Contains(t, string(collide.Stderr), "GOLC_DEPLOYMENT_ADDRESS_COLLISION", "expected GOLC_DEPLOYMENT_ADDRESS_COLLISION, got exit=%d stderr=%s", collide.ExitCode, collide.Stderr)

	malformedID := registry.Execute(command.Request{Root: root, Args: []string{
		"deployment", "instance", "reassign", "Venue A", secondInstanceID + "0",
		"--mode", "Standard",
		"--show", showPath,
	}})
	require.NotEqual(t, 0, malformedID.ExitCode)
	require.Contains(t, string(malformedID.Stderr), "GOLC_DEPLOYMENT_USAGE", "expected GOLC_DEPLOYMENT_USAGE for a malformed instance-id UUID, got exit=%d stderr=%s", malformedID.ExitCode, malformedID.Stderr)

	unknownInstance := registry.Execute(command.Request{Root: root, Args: []string{
		"deployment", "instance", "reassign", "Venue A", "11111111-1111-1111-1111-111111111111",
		"--mode", "Standard",
		"--show", showPath,
	}})
	require.NotEqual(t, 0, unknownInstance.ExitCode)
	require.Contains(t, string(unknownInstance.Stderr), "GOLC_DEPLOYMENT_INSTANCE_NOT_FOUND", "expected GOLC_DEPLOYMENT_INSTANCE_NOT_FOUND for a well-formed but nonexistent instance id, got exit=%d stderr=%s", unknownInstance.ExitCode, unknownInstance.Stderr)

	usage := registry.Execute(command.Request{Root: root, Args: []string{
		"deployment", "instance", "reassign", "Venue A", secondInstanceID,
		"--universe", "notanumber",
		"--show", showPath,
	}})
	require.NotEqual(t, 0, usage.ExitCode)
	require.Contains(t, string(usage.Stderr), "GOLC_DEPLOYMENT_USAGE", "expected GOLC_DEPLOYMENT_USAGE for a non-numeric --universe, got exit=%d stderr=%s", usage.ExitCode, usage.Stderr)
}
