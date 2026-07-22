// programming_test.go proves the "programmer" command scope's route
// contract (03-01-PLAN.md Task 3): "programmer set" resolves a selection
// and edits semantic attributes on a ShowState document, "programmer
// inspect" reports every touched attribute with its value/source/record
// scope, and "programmer clear" empties the buffer -- all persisted
// through the existing show.Load/show.Save round trip. It follows
// poolimpact_test.go's seed-a-ShowState-directly-then-exercise-CLI-routes
// convention: production config isn't involved here, so root is a plain
// t.TempDir(), not the real repository root.
//
// This file compiles against the already-implemented internal/command
// package but fails at RUN time until programming.go self-registers the
// "programmer" scope/routes (Task 3) -- that is the RED state this task
// proves.
package command_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/command"
	"github.com/lnorton89/golc/internal/deployment"
	"github.com/lnorton89/golc/internal/fixture"
	"github.com/lnorton89/golc/internal/pool"
	"github.com/lnorton89/golc/internal/programming"
	"github.com/lnorton89/golc/internal/show"
)

// seedProgrammerShowState builds and saves a minimal ShowState with one
// pool (one member), one deployment with an Instance patched to that
// member, and returns the deployment Instance's ID -- the target
// "programmer set --instance <id>" resolves and edits.
func seedProgrammerShowState(t *testing.T, root, showPath string) uuid.UUID {
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
	return instanceID
}

func TestProgrammerRoutes(t *testing.T) {
	root := t.TempDir()
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}

	showPath := filepath.Join(t.TempDir(), "show.json")
	instanceID := seedProgrammerShowState(t, root, showPath)

	set := registry.Execute(command.Request{Root: root, Args: []string{
		"programmer", "set",
		"--instance", instanceID.String(),
		"--attr", "intensity=0.8",
		"--show", showPath,
	}})
	if set.ExitCode != 0 {
		t.Fatalf("programmer set failed: exit=%d stderr=%s", set.ExitCode, set.Stderr)
	}

	reloaded, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load after set: %v", err)
	}
	if reloaded.Programmer == nil || len(reloaded.Programmer.Touched()) != 1 {
		t.Fatalf("expected exactly one touched attribute to persist, got %+v", reloaded.Programmer)
	}
	touched := reloaded.Programmer.Touched()[0]
	if touched.InstanceID != instanceID || touched.Capability != "intensity" || touched.Value != 0.8 {
		t.Fatalf("unexpected persisted touched attribute: %+v", touched)
	}

	inspect := registry.Execute(command.Request{Root: root, Args: []string{
		"programmer", "inspect", "--show", showPath,
	}})
	if inspect.ExitCode != 0 {
		t.Fatalf("programmer inspect failed: exit=%d stderr=%s", inspect.ExitCode, inspect.Stderr)
	}
	out := string(inspect.Stdout)
	if !strings.Contains(out, instanceID.String()) || !strings.Contains(out, "intensity") ||
		!strings.Contains(out, "0.8") || !strings.Contains(out, "manual") {
		t.Fatalf("expected programmer inspect output to include instance/capability/value/source, got %q", out)
	}

	outOfRange := registry.Execute(command.Request{Root: root, Args: []string{
		"programmer", "set",
		"--instance", instanceID.String(),
		"--attr", "intensity=1.5",
		"--show", showPath,
	}})
	if outOfRange.ExitCode == 0 || !strings.Contains(string(outOfRange.Stderr), "GOLC_PROGRAMMER_VALUE_OUT_OF_RANGE") {
		t.Fatalf("expected GOLC_PROGRAMMER_VALUE_OUT_OF_RANGE for an out-of-range --attr value, got exit=%d stderr=%s", outOfRange.ExitCode, outOfRange.Stderr)
	}

	malformed := registry.Execute(command.Request{Root: root, Args: []string{
		"programmer", "set",
		"--instance", instanceID.String(),
		"--attr", "intensity=0.5",
	}})
	if malformed.ExitCode != 2 || !strings.Contains(string(malformed.Stderr), "GOLC_PROGRAMMER_USAGE") {
		t.Fatalf("expected exit 2 GOLC_PROGRAMMER_USAGE for a missing --show, got exit=%d stderr=%s", malformed.ExitCode, malformed.Stderr)
	}

	clear := registry.Execute(command.Request{Root: root, Args: []string{
		"programmer", "clear", "--show", showPath,
	}})
	if clear.ExitCode != 0 {
		t.Fatalf("programmer clear failed: exit=%d stderr=%s", clear.ExitCode, clear.Stderr)
	}
	afterClear, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load after clear: %v", err)
	}
	if afterClear.Programmer == nil || len(afterClear.Programmer.Touched()) != 0 {
		t.Fatalf("expected an empty touched-attribute buffer after clear, got %+v", afterClear.Programmer)
	}
}

func TestProgrammerSetUnsupportedCapability(t *testing.T) {
	root := t.TempDir()
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}
	showPath := filepath.Join(t.TempDir(), "show.json")
	instanceID := seedProgrammerShowState(t, root, showPath)

	result := registry.Execute(command.Request{Root: root, Args: []string{
		"programmer", "set",
		"--instance", instanceID.String(),
		"--attr", "laser=0.5",
		"--show", showPath,
	}})
	if result.ExitCode == 0 || !strings.Contains(string(result.Stderr), "GOLC_PROGRAMMER_CAPABILITY_UNSUPPORTED") {
		t.Fatalf("expected GOLC_PROGRAMMER_CAPABILITY_UNSUPPORTED for an unsupported capability, got exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
}

func TestProgrammerSetDanglingInstance(t *testing.T) {
	root := t.TempDir()
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}
	showPath := filepath.Join(t.TempDir(), "show.json")
	seedProgrammerShowState(t, root, showPath)

	result := registry.Execute(command.Request{Root: root, Args: []string{
		"programmer", "set",
		"--instance", uuid.New().String(),
		"--attr", "intensity=0.5",
		"--show", showPath,
	}})
	if result.ExitCode == 0 || !strings.Contains(string(result.Stderr), "GOLC_SELECTION_DANGLING_REFERENCE") {
		t.Fatalf("expected GOLC_SELECTION_DANGLING_REFERENCE for an unknown --instance, got exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
}

func TestProgrammerShowStateRoundTrip(t *testing.T) {
	root := t.TempDir()
	showPath := filepath.Join(t.TempDir(), "show.json")
	instanceID := seedProgrammerShowState(t, root, showPath)

	state, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load: %v", err)
	}
	state.Programmer = programming.NewProgrammerState()
	if err := state.Programmer.SetAttribute(instanceID, fixture.CapabilityIntensity, 0.42, programming.SourceManual); err != nil {
		t.Fatalf("SetAttribute: %v", err)
	}
	if err := show.Save(root, showPath, state); err != nil {
		t.Fatalf("show.Save: %v", err)
	}

	reloaded, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load (reloaded): %v", err)
	}
	if reloaded.Programmer == nil || len(reloaded.Programmer.Touched()) != 1 {
		t.Fatalf("expected the Programmer buffer to round-trip through Save/Load, got %+v", reloaded.Programmer)
	}
}
