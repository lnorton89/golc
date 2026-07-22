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
	"strings"
	"testing"

	"github.com/lnorton89/golc/internal/command"
)

func TestPoolDeployRoutes(t *testing.T) {
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}
	root := t.TempDir()
	showPath := "show.json"

	createPool := registry.Execute(command.Request{Root: root, Args: []string{"pool", "create", "Wash Pool", "--requires", "intensity,color", "--show", showPath}})
	if createPool.ExitCode != 0 {
		t.Fatalf("pool create failed: exit=%d stderr=%s", createPool.ExitCode, createPool.Stderr)
	}

	dupPool := registry.Execute(command.Request{Root: root, Args: []string{"pool", "create", "Wash Pool", "--show", showPath}})
	if dupPool.ExitCode == 0 || !strings.Contains(string(dupPool.Stderr), "GOLC_POOL_DUPLICATE_NAME") {
		t.Fatalf("expected GOLC_POOL_DUPLICATE_NAME for duplicate pool create, got exit=%d stderr=%s", dupPool.ExitCode, dupPool.Stderr)
	}

	depA := registry.Execute(command.Request{Root: root, Args: []string{"deployment", "create", "Venue A", "--show", showPath}})
	if depA.ExitCode != 0 {
		t.Fatalf("deployment create (A) failed: exit=%d stderr=%s", depA.ExitCode, depA.Stderr)
	}
	depB := registry.Execute(command.Request{Root: root, Args: []string{"deployment", "create", "Venue B", "--show", showPath}})
	if depB.ExitCode != 0 {
		t.Fatalf("deployment create (B) failed: exit=%d stderr=%s", depB.ExitCode, depB.Stderr)
	}

	dupDeployment := registry.Execute(command.Request{Root: root, Args: []string{"deployment", "create", "Venue A", "--show", showPath}})
	if dupDeployment.ExitCode == 0 || !strings.Contains(string(dupDeployment.Stderr), "GOLC_DEPLOYMENT_DUPLICATE_NAME") {
		t.Fatalf("expected GOLC_DEPLOYMENT_DUPLICATE_NAME for duplicate deployment create, got exit=%d stderr=%s", dupDeployment.ExitCode, dupDeployment.Stderr)
	}

	activateA := registry.Execute(command.Request{Root: root, Args: []string{"deployment", "activate", "Venue A", "--show", showPath}})
	if activateA.ExitCode != 0 {
		t.Fatalf("deployment activate (A) failed: exit=%d stderr=%s", activateA.ExitCode, activateA.Stderr)
	}
	activateB := registry.Execute(command.Request{Root: root, Args: []string{"deployment", "activate", "Venue B", "--show", showPath}})
	if activateB.ExitCode != 0 {
		t.Fatalf("deployment activate (B) failed: exit=%d stderr=%s", activateB.ExitCode, activateB.Stderr)
	}

	inspectFirst := registry.Execute(command.Request{Root: root, Args: []string{"show", "inspect", "--show", showPath}})
	if inspectFirst.ExitCode != 0 {
		t.Fatalf("show inspect failed: exit=%d stderr=%s", inspectFirst.ExitCode, inspectFirst.Stderr)
	}
	inspectSecond := registry.Execute(command.Request{Root: root, Args: []string{"show", "inspect", "--show", showPath}})
	if inspectSecond.ExitCode != 0 {
		t.Fatalf("show inspect (second) failed: exit=%d stderr=%s", inspectSecond.ExitCode, inspectSecond.Stderr)
	}
	if string(inspectFirst.Stdout) != string(inspectSecond.Stdout) {
		t.Fatalf("expected deterministic show inspect output:\nfirst:  %s\nsecond: %s", inspectFirst.Stdout, inspectSecond.Stdout)
	}
	if strings.Contains(string(inspectFirst.Stdout), root) || strings.Contains(string(inspectFirst.Stdout), filepath.Join(root, showPath)) {
		t.Fatalf("expected no absolute filesystem path in show inspect output, got %s", inspectFirst.Stdout)
	}

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
	if err := json.Unmarshal(inspectFirst.Stdout, &view); err != nil {
		t.Fatalf("unmarshal show inspect output: %v", err)
	}
	if len(view.Pools) != 1 || view.Pools[0].Name != "Wash Pool" {
		t.Fatalf("expected exactly one pool named Wash Pool, got %+v", view.Pools)
	}
	activeCount := 0
	for _, d := range view.Deployments {
		if d.Active {
			activeCount++
		}
	}
	if activeCount != 1 {
		t.Fatalf("expected exactly one active deployment in show inspect, got %d among %+v", activeCount, view.Deployments)
	}
}
