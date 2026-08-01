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
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

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
	require.NoError(t, err, "NewPool: %v", err)
	member, err := pool.NewPoolMember("acme/par64", "sha256:11111111")
	require.NoError(t, err, "NewPoolMember: %v", err)
	p.Members = append(p.Members, member)

	d, err := deployment.NewDeployment("Venue A")
	require.NoError(t, err, "NewDeployment: %v", err)
	instanceID, err := uuid.NewV7()
	require.NoError(t, err, "uuid.NewV7: %v", err)
	d.Instances = append(d.Instances, deployment.Instance{
		ID:           instanceID,
		PoolID:       p.ID,
		PoolMemberID: member.ID,
		Mode:         "Standard",
		Universe:     1,
		Address:      1,
	})

	state := show.State{Pools: []pool.Pool{p}, Deployments: []deployment.Deployment{d}}
	err = show.Save(root, showPath, state)
	require.NoError(t, err, "show.Save (seed): %v", err)
	return instanceID
}

func TestProgrammerRoutes(t *testing.T) {
	root := t.TempDir()
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err, "NewDefaultCommandRegistry: %v", err)

	showPath := filepath.Join(t.TempDir(), "show.json")
	instanceID := seedProgrammerShowState(t, root, showPath)

	set := registry.Execute(command.Request{Root: root, Args: []string{
		"programmer", "set",
		"--instance", instanceID.String(),
		"--attr", "intensity=0.8",
		"--show", showPath,
	}})
	require.Equal(t, 0, set.ExitCode, "programmer set failed: exit=%d stderr=%s", set.ExitCode, set.Stderr)

	reloaded, err := show.Load(root, showPath)
	require.NoError(t, err, "show.Load after set: %v", err)
	require.NotNil(t, reloaded.Programmer, "expected exactly one touched attribute to persist, got %+v", reloaded.Programmer)
	require.Len(t, reloaded.Programmer.Touched(), 1, "expected exactly one touched attribute to persist, got %+v", reloaded.Programmer)
	touched := reloaded.Programmer.Touched()[0]
	require.Equal(t, instanceID, touched.InstanceID, "unexpected persisted touched attribute: %+v", touched)
	require.EqualValues(t, "intensity", touched.Capability, "unexpected persisted touched attribute: %+v", touched)
	require.Equal(t, 0.8, touched.Value, "unexpected persisted touched attribute: %+v", touched)

	inspect := registry.Execute(command.Request{Root: root, Args: []string{
		"programmer", "inspect", "--show", showPath,
	}})
	require.Equal(t, 0, inspect.ExitCode, "programmer inspect failed: exit=%d stderr=%s", inspect.ExitCode, inspect.Stderr)
	out := string(inspect.Stdout)
	require.Contains(t, out, instanceID.String(), "expected programmer inspect output to include instance/capability/value/source, got %q", out)
	require.Contains(t, out, "intensity", "expected programmer inspect output to include instance/capability/value/source, got %q", out)
	require.Contains(t, out, "0.8", "expected programmer inspect output to include instance/capability/value/source, got %q", out)
	require.Contains(t, out, "manual", "expected programmer inspect output to include instance/capability/value/source, got %q", out)

	outOfRange := registry.Execute(command.Request{Root: root, Args: []string{
		"programmer", "set",
		"--instance", instanceID.String(),
		"--attr", "intensity=1.5",
		"--show", showPath,
	}})
	require.NotEqual(t, 0, outOfRange.ExitCode, "expected GOLC_PROGRAMMER_VALUE_OUT_OF_RANGE for an out-of-range --attr value, got exit=%d stderr=%s", outOfRange.ExitCode, outOfRange.Stderr)
	require.Contains(t, string(outOfRange.Stderr), "GOLC_PROGRAMMER_VALUE_OUT_OF_RANGE", "expected GOLC_PROGRAMMER_VALUE_OUT_OF_RANGE for an out-of-range --attr value, got exit=%d stderr=%s", outOfRange.ExitCode, outOfRange.Stderr)

	malformed := registry.Execute(command.Request{Root: root, Args: []string{
		"programmer", "set",
		"--instance", instanceID.String(),
		"--attr", "intensity=0.5",
	}})
	require.Equal(t, 2, malformed.ExitCode, "expected exit 2 GOLC_PROGRAMMER_USAGE for a missing --show, got exit=%d stderr=%s", malformed.ExitCode, malformed.Stderr)
	require.Contains(t, string(malformed.Stderr), "GOLC_PROGRAMMER_USAGE", "expected exit 2 GOLC_PROGRAMMER_USAGE for a missing --show, got exit=%d stderr=%s", malformed.ExitCode, malformed.Stderr)

	clear := registry.Execute(command.Request{Root: root, Args: []string{
		"programmer", "clear", "--show", showPath,
	}})
	require.Equal(t, 0, clear.ExitCode, "programmer clear failed: exit=%d stderr=%s", clear.ExitCode, clear.Stderr)
	afterClear, err := show.Load(root, showPath)
	require.NoError(t, err, "show.Load after clear: %v", err)
	require.NotNil(t, afterClear.Programmer, "expected an empty touched-attribute buffer after clear, got %+v", afterClear.Programmer)
	require.Empty(t, afterClear.Programmer.Touched(), "expected an empty touched-attribute buffer after clear, got %+v", afterClear.Programmer)
}

func TestProgrammerSetUnsupportedCapability(t *testing.T) {
	root := t.TempDir()
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err, "NewDefaultCommandRegistry: %v", err)
	showPath := filepath.Join(t.TempDir(), "show.json")
	instanceID := seedProgrammerShowState(t, root, showPath)

	result := registry.Execute(command.Request{Root: root, Args: []string{
		"programmer", "set",
		"--instance", instanceID.String(),
		"--attr", "laser=0.5",
		"--show", showPath,
	}})
	require.NotEqual(t, 0, result.ExitCode, "expected GOLC_PROGRAMMER_CAPABILITY_UNSUPPORTED for an unsupported capability, got exit=%d stderr=%s", result.ExitCode, result.Stderr)
	require.Contains(t, string(result.Stderr), "GOLC_PROGRAMMER_CAPABILITY_UNSUPPORTED", "expected GOLC_PROGRAMMER_CAPABILITY_UNSUPPORTED for an unsupported capability, got exit=%d stderr=%s", result.ExitCode, result.Stderr)
}

func TestProgrammerSetDanglingInstance(t *testing.T) {
	root := t.TempDir()
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err, "NewDefaultCommandRegistry: %v", err)
	showPath := filepath.Join(t.TempDir(), "show.json")
	seedProgrammerShowState(t, root, showPath)

	result := registry.Execute(command.Request{Root: root, Args: []string{
		"programmer", "set",
		"--instance", uuid.New().String(),
		"--attr", "intensity=0.5",
		"--show", showPath,
	}})
	require.NotEqual(t, 0, result.ExitCode, "expected GOLC_SELECTION_DANGLING_REFERENCE for an unknown --instance, got exit=%d stderr=%s", result.ExitCode, result.Stderr)
	require.Contains(t, string(result.Stderr), "GOLC_SELECTION_DANGLING_REFERENCE", "expected GOLC_SELECTION_DANGLING_REFERENCE for an unknown --instance, got exit=%d stderr=%s", result.ExitCode, result.Stderr)
}

func TestProgrammerShowStateRoundTrip(t *testing.T) {
	root := t.TempDir()
	showPath := filepath.Join(t.TempDir(), "show.json")
	instanceID := seedProgrammerShowState(t, root, showPath)

	state, err := show.Load(root, showPath)
	require.NoError(t, err, "show.Load: %v", err)
	state.Programmer = programming.NewProgrammerState()
	err = state.Programmer.SetAttribute(instanceID, fixture.CapabilityIntensity, 0.42, programming.SourceManual)
	require.NoError(t, err, "SetAttribute: %v", err)
	err = show.Save(root, showPath, state)
	require.NoError(t, err, "show.Save: %v", err)

	reloaded, err := show.Load(root, showPath)
	require.NoError(t, err, "show.Load (reloaded): %v", err)
	require.NotNil(t, reloaded.Programmer, "expected the Programmer buffer to round-trip through Save/Load, got %+v", reloaded.Programmer)
	require.Len(t, reloaded.Programmer.Touched(), 1, "expected the Programmer buffer to round-trip through Save/Load, got %+v", reloaded.Programmer)
}
