// pooldeploy_test.go proves the "pool create" / "deployment create" /
// "deployment activate" / "show inspect" route contract (02-04-PLAN.md,
// Task 1 Wave-0 scaffold): the walking skeleton lets a show author create
// a pool and deployments, activate exactly one deployment, and inspect
// the resulting ShowState document through a deterministic, path-free
// JSON envelope. It follows router_test.go's exact route-invocation
// convention: build the default registry (command files self-register
// their routes/scopes per D-04), Execute a Request, assert
// Result.ExitCode/Stdout/Stderr.
//
// This file compiles today (it only depends on the already-implemented
// command package) but fails at RUN time until Task 3 of 02-04-PLAN.md
// self-registers the pool/deployment/show routes -- that is the RED
// state this task proves.
package command_test

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/command"
)

func TestPoolDeployRoutes(t *testing.T) {
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err, "NewDefaultCommandRegistry")
	root := t.TempDir()
	showPath := "show.json"

	createPool := registry.Execute(command.Request{Root: root, Args: []string{"pool", "create", "Wash Pool", "--requires", "intensity,color", "--show", showPath}})
	require.Equalf(t, 0, createPool.ExitCode, "pool create failed: stderr=%s", createPool.Stderr)

	dupPool := registry.Execute(command.Request{Root: root, Args: []string{"pool", "create", "Wash Pool", "--show", showPath}})
	require.NotEqualf(t, 0, dupPool.ExitCode, "expected GOLC_POOL_DUPLICATE_NAME for duplicate pool create, got stderr=%s", dupPool.Stderr)
	require.Contains(t, string(dupPool.Stderr), "GOLC_POOL_DUPLICATE_NAME")

	depA := registry.Execute(command.Request{Root: root, Args: []string{"deployment", "create", "Venue A", "--show", showPath}})
	require.Equalf(t, 0, depA.ExitCode, "deployment create (A) failed: stderr=%s", depA.Stderr)
	depB := registry.Execute(command.Request{Root: root, Args: []string{"deployment", "create", "Venue B", "--show", showPath}})
	require.Equalf(t, 0, depB.ExitCode, "deployment create (B) failed: stderr=%s", depB.Stderr)

	dupDeployment := registry.Execute(command.Request{Root: root, Args: []string{"deployment", "create", "Venue A", "--show", showPath}})
	require.NotEqualf(t, 0, dupDeployment.ExitCode, "expected GOLC_DEPLOYMENT_DUPLICATE_NAME for duplicate deployment create, got stderr=%s", dupDeployment.Stderr)
	require.Contains(t, string(dupDeployment.Stderr), "GOLC_DEPLOYMENT_DUPLICATE_NAME")

	activateA := registry.Execute(command.Request{Root: root, Args: []string{"deployment", "activate", "Venue A", "--show", showPath}})
	require.Equalf(t, 0, activateA.ExitCode, "deployment activate (A) failed: stderr=%s", activateA.Stderr)
	activateB := registry.Execute(command.Request{Root: root, Args: []string{"deployment", "activate", "Venue B", "--show", showPath}})
	require.Equalf(t, 0, activateB.ExitCode, "deployment activate (B) failed: stderr=%s", activateB.Stderr)

	inspectFirst := registry.Execute(command.Request{Root: root, Args: []string{"show", "inspect", "--show", showPath}})
	require.Equalf(t, 0, inspectFirst.ExitCode, "show inspect failed: stderr=%s", inspectFirst.Stderr)
	inspectSecond := registry.Execute(command.Request{Root: root, Args: []string{"show", "inspect", "--show", showPath}})
	require.Equalf(t, 0, inspectSecond.ExitCode, "show inspect (second) failed: stderr=%s", inspectSecond.Stderr)
	require.Equalf(t, string(inspectSecond.Stdout), string(inspectFirst.Stdout), "expected deterministic show inspect output")
	require.NotContains(t, string(inspectFirst.Stdout), root)
	require.NotContainsf(t, string(inspectFirst.Stdout), filepath.Join(root, showPath), "expected no absolute filesystem path in show inspect output, got %s", inspectFirst.Stdout)

	var view struct {
		Pools []struct {
			Name                 string   `json:"name"`
			RequiredCapabilities []string `json:"required_capabilities"`
			MemberCount          int      `json:"member_count"`
		} `json:"pools"`
		Deployments []struct {
			Name          string `json:"name"`
			Active        bool   `json:"active"`
			InstanceCount int    `json:"instance_count"`
		} `json:"deployments"`
	}
	require.NoError(t, json.Unmarshal(inspectFirst.Stdout, &view), "unmarshal show inspect output")
	require.Lenf(t, view.Pools, 1, "expected exactly one pool named Wash Pool, got %+v", view.Pools)
	require.Equal(t, "Wash Pool", view.Pools[0].Name)
	activeCount := 0
	for _, d := range view.Deployments {
		if d.Active {
			activeCount++
		}
	}
	require.Equalf(t, 1, activeCount, "expected exactly one active deployment in show inspect, got %d among %+v", activeCount, view.Deployments)
}
