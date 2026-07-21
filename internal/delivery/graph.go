// Package delivery is the single declarative owner of GOLC's offline core
// command graph (CONTEXT D-02/D-03/D-10): the fixed generate/check/build/
// test execution order, each step's network policy, and the offline
// environment/deny-transport contract every network-denied step runs
// under. internal/command's route files (generate.go, check.go, build.go,
// test.go) self-register the exact reachable commands (D-03); this
// package never registers a route itself and never imports
// internal/command, so command -> delivery is the only import direction
// and no cycle can form.
//
// This package also never imports internal/bootstrap: internal/bootstrap's
// own test files import internal/command (to self-register their quick-
// test scopes, per the established 01-VALIDATION pattern), and
// internal/command imports internal/delivery (check.go), so a
// delivery -> bootstrap import would close a cycle through bootstrap's
// test binary (bootstrap[test] -> command -> delivery -> bootstrap).
// resolveOfflineEnvironment below instead computes the repository-local
// cache paths directly, mirroring the same direct-computation pattern
// internal/command/test.go's projectGoEnvironment already establishes,
// rather than depending on internal/bootstrap.ProjectCacheLayout.
//
// LoadGraph, Run, RunOffline, and ValidateParity all operate on the one
// Graph value LoadGraph returns; no second, independently maintained list
// of steps exists anywhere else in the repository (T-01-17: one parsed
// graph). LoadGraph consumes exactly the three canonical keys
// internal/projectconfig/model.go's "commands" concern already owns in
// config/commands.toml (commands.entrypoint, commands.cli_binary,
// commands.go_version) — the same file internal/command/test.go's
// resolvePinnedGoExecutable-style direct TOML decode pattern already
// establishes for config/toolchain.toml. It never introduces a second
// command-inventory source.
package delivery

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// NetworkPolicy declares whether a graph step may open a network
// connection. Every step in the core graph is NetworkDenied per D-02; a
// future network-only step (dependency refresh, Linear sync) would
// declare NetworkAllowed explicitly, and RunOffline refuses to execute a
// graph containing one.
type NetworkPolicy int

const (
	// NetworkDenied is the exact policy every core graph step declares:
	// no step in generate/check/build/test may open a network connection.
	NetworkDenied NetworkPolicy = iota
	// NetworkAllowed marks a step that may reach the network. RunOffline
	// refuses to execute any graph containing one.
	NetworkAllowed
)

// String renders the policy for diagnostics and reports.
func (policy NetworkPolicy) String() string {
	if policy == NetworkAllowed {
		return "allowed"
	}
	return "denied"
}

// CommandInventory is the exact command-identity data this graph consumes
// from config/commands.toml: the contributor entrypoint script name, the
// delegated project-local CLI binary path, and the pinned Go version
// reference. LoadGraph never consults any other file for this inventory.
type CommandInventory struct {
	Entrypoint string
	CLIBinary  string
	GoVersion  string
}

// Step is one node of the offline core delivery graph: a stable name, the
// exact self-registered command.CommandRegistry route it invokes, the
// fixed argument vector this graph's default invocation uses, and its
// network policy.
type Step struct {
	Name    string
	Route   string
	Args    []string
	Network NetworkPolicy
}

// Graph is the one declarative root/core command graph (T-01-17): the
// exact generate/check/build/test steps in the fixed contributor/CI
// execution order (D-10 parity) plus the config/commands.toml-sourced
// inventory every step's registered route ultimately serves.
type Graph struct {
	Root      string
	Inventory CommandInventory
	Steps     []Step
}

// coreSteps is the fixed, single-owner declaration of the offline core
// graph (D-02/D-03/D-10). Order matters: generate runs before check (so a
// project check observes freshly generated, non-drifted schemas), build
// runs before test (so a test failure is never masked by a stale binary),
// and check invokes "--concern project" rather than "--offline" so a
// check-driven graph run can never recurse into itself.
func coreSteps() []Step {
	return []Step{
		{Name: "generate", Route: "generate", Args: nil, Network: NetworkDenied},
		{Name: "check", Route: "check", Args: []string{"--concern", "project"}, Network: NetworkDenied},
		{Name: "build", Route: "build", Args: nil, Network: NetworkDenied},
		{Name: "test", Route: "test", Args: []string{"--quick"}, Network: NetworkDenied},
	}
}

// commandsConcernDocument is the minimal decode shape for
// config/commands.toml: exactly the three canonical keys the "commands"
// concern owns. A second, independently maintained struct is deliberately
// avoided elsewhere in this package.
type commandsConcernDocument struct {
	Commands struct {
		Entrypoint string `toml:"entrypoint"`
		CLIBinary  string `toml:"cli_binary"`
		GoVersion  string `toml:"go_version"`
	} `toml:"commands"`
}

// LoadGraph reads config/commands.toml under root and returns the fixed
// offline core delivery graph. It is the exclusive command-inventory
// consumer: Run, RunOffline, and ValidateParity all operate on the Graph
// value this function returns rather than re-reading configuration
// themselves.
func LoadGraph(root string) (Graph, error) {
	manifestPath := filepath.Join(root, "config", "commands.toml")
	var document commandsConcernDocument
	if _, err := toml.DecodeFile(manifestPath, &document); err != nil {
		return Graph{}, fmt.Errorf("GOLC_DELIVERY_INVENTORY_MISSING: config/commands.toml: %v", err)
	}
	inventory := CommandInventory{
		Entrypoint: strings.TrimSpace(document.Commands.Entrypoint),
		CLIBinary:  strings.TrimSpace(document.Commands.CLIBinary),
		GoVersion:  strings.TrimSpace(document.Commands.GoVersion),
	}
	if inventory.Entrypoint == "" || inventory.CLIBinary == "" || inventory.GoVersion == "" {
		return Graph{}, fmt.Errorf(
			"GOLC_DELIVERY_INVENTORY_INCOMPLETE: config/commands.toml must declare entrypoint, cli_binary, and go_version")
	}
	return Graph{Root: root, Inventory: inventory, Steps: coreSteps()}, nil
}

// ValidateParity confirms g is well-formed and duplicate-safe (T-01-17):
// every step has a non-blank name and route, no two steps share a name,
// no two steps declare the identical route-plus-arguments invocation, and
// the command inventory is complete. A corrupted or hand-edited Graph
// value can never silently alias two steps together or run with a partial
// inventory.
func ValidateParity(g Graph) error {
	if len(g.Steps) == 0 {
		return fmt.Errorf("GOLC_DELIVERY_GRAPH_EMPTY: graph declares zero steps")
	}
	seenNames := map[string]struct{}{}
	seenInvocations := map[string]struct{}{}
	for _, step := range g.Steps {
		name := strings.TrimSpace(step.Name)
		route := strings.TrimSpace(step.Route)
		if name == "" {
			return fmt.Errorf("GOLC_DELIVERY_STEP_INVALID: a step has a blank name")
		}
		if route == "" {
			return fmt.Errorf("GOLC_DELIVERY_STEP_INVALID: step %q has a blank route", name)
		}
		if _, duplicate := seenNames[name]; duplicate {
			return fmt.Errorf("GOLC_DELIVERY_STEP_DUPLICATE: step name %q is declared twice", name)
		}
		seenNames[name] = struct{}{}
		invocation := route + " " + strings.Join(step.Args, " ")
		if _, duplicate := seenInvocations[invocation]; duplicate {
			return fmt.Errorf("GOLC_DELIVERY_STEP_DUPLICATE: invocation %q is declared twice", invocation)
		}
		seenInvocations[invocation] = struct{}{}
	}
	if g.Inventory.Entrypoint == "" || g.Inventory.CLIBinary == "" || g.Inventory.GoVersion == "" {
		return fmt.Errorf("GOLC_DELIVERY_INVENTORY_INCOMPLETE: graph inventory is incomplete")
	}
	return nil
}

// StepExecutor invokes one step's registered route with its default
// arguments and returns its process-shaped exit code and captured output.
// internal/command supplies this by closing over its own
// command.CommandRegistry so internal/delivery never imports
// internal/command (which would form an import cycle, since
// internal/command's check.go/test.go/build.go import internal/delivery
// to orchestrate this graph).
type StepExecutor func(route string, args []string) (exitCode int, stdout, stderr []byte)

// StepResult is one executed step's outcome.
type StepResult struct {
	Step     Step
	ExitCode int
	Stdout   []byte
	Stderr   []byte
}

// Run executes every step in g in order through execute, stopping at the
// first non-zero exit. It never inspects or mutates the process
// environment or transport itself: RunOffline is the only caller that
// layers the network-denied contract on top of this shared execution
// path, so contributor and CI invocations of the graph are byte-identical
// except for that one wrapper (D-10 parity).
func Run(g Graph, execute StepExecutor) ([]StepResult, error) {
	results := make([]StepResult, 0, len(g.Steps))
	for _, step := range g.Steps {
		exitCode, stdout, stderr := execute(step.Route, step.Args)
		results = append(results, StepResult{Step: step, ExitCode: exitCode, Stdout: stdout, Stderr: stderr})
		if exitCode != 0 {
			return results, fmt.Errorf("GOLC_DELIVERY_STEP_FAILED: step %q (route %q) exited %d", step.Name, step.Route, exitCode)
		}
	}
	return results, nil
}

// OfflineEnvironment is the exact process-environment contract every
// offline graph step's child Go invocation must observe (D-02): the
// repository-local Go toolchain/module/build/bin caches
// internal/bootstrap.ProjectCacheLayout already owns (Plan 28), plus
// GOPROXY=off so a step can never silently reach a module proxy even if
// GOFLAGS=-mod=readonly were ever bypassed downstream.
type OfflineEnvironment struct {
	GOTOOLCHAIN string
	GOPROXY     string
	GOMODCACHE  string
	GOCACHE     string
	GOBIN       string
	GOFLAGS     string
}

// AsMap returns env as a name->value map suitable for installing into the
// current process environment.
func (env OfflineEnvironment) AsMap() map[string]string {
	return map[string]string{
		"GOTOOLCHAIN": env.GOTOOLCHAIN,
		"GOPROXY":     env.GOPROXY,
		"GOMODCACHE":  env.GOMODCACHE,
		"GOCACHE":     env.GOCACHE,
		"GOBIN":       env.GOBIN,
		"GOFLAGS":     env.GOFLAGS,
	}
}

// resolveOfflineEnvironment derives OfflineEnvironment from the exact
// repository-local cache directory layout internal/bootstrap/cache.go's
// ProjectCacheLayout (Plan 28) and internal/command/test.go's
// projectGoEnvironment both already establish (.tools/cache/go-mod,
// .tools/cache/go-build, .tools/cache/go-bin under root), adding
// GOPROXY=off. It never imports internal/bootstrap (see the package
// doc comment for why) and never re-declares a fourth, independently
// maintained copy of these paths beyond the two that already exist.
func resolveOfflineEnvironment(root string) (OfflineEnvironment, error) {
	if strings.TrimSpace(root) == "" {
		return OfflineEnvironment{}, fmt.Errorf("GOLC_DELIVERY_OFFLINE_ENV: root must not be empty")
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return OfflineEnvironment{}, fmt.Errorf("GOLC_DELIVERY_OFFLINE_ENV: %v", err)
	}
	return OfflineEnvironment{
		GOTOOLCHAIN: "local",
		GOPROXY:     "off",
		GOMODCACHE:  filepath.Join(absoluteRoot, ".tools", "cache", "go-mod"),
		GOCACHE:     filepath.Join(absoluteRoot, ".tools", "cache", "go-build"),
		GOBIN:       filepath.Join(absoluteRoot, ".tools", "cache", "go-bin"),
		GOFLAGS:     "-mod=readonly",
	}, nil
}

// DenyTransport is an http.RoundTripper that fails closed: every request
// returns a named GOLC_DELIVERY_NETWORK_DENIED diagnostic before any dial
// is attempted. RunOffline installs it as http.DefaultTransport for the
// duration of the offline run so a step that unexpectedly attempts an
// HTTP call inside this process fails loudly with a named diagnostic
// instead of silently succeeding on a machine that happens to have real
// network access (D-02, T-01-15).
type DenyTransport struct{}

// RoundTrip always fails with GOLC_DELIVERY_NETWORK_DENIED.
func (DenyTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return nil, fmt.Errorf(
		"GOLC_DELIVERY_NETWORK_DENIED: network access denied for offline step (%s %s)", request.Method, request.URL)
}

// RunOffline runs every step in g with the offline environment and deny
// transport installed (D-02), restoring the exact prior process state
// once execution completes. It refuses to execute if any step declares a
// policy other than NetworkDenied — such a step has no place in the
// offline core graph.
func RunOffline(g Graph, execute StepExecutor) ([]StepResult, error) {
	for _, step := range g.Steps {
		if step.Network != NetworkDenied {
			return nil, fmt.Errorf(
				"GOLC_DELIVERY_STEP_NETWORK_POLICY: step %q is not network-denied; RunOffline refuses to execute it", step.Name)
		}
	}

	env, err := resolveOfflineEnvironment(g.Root)
	if err != nil {
		return nil, err
	}
	restoreEnvironment := setEnvironment(env.AsMap())
	defer restoreEnvironment()

	previousTransport := http.DefaultTransport
	http.DefaultTransport = DenyTransport{}
	defer func() { http.DefaultTransport = previousTransport }()

	return Run(g, execute)
}

// setEnvironment sets every name/value pair and returns a function that
// restores the exact previous state (including "unset" for a name that
// had no prior value) once execution completes.
func setEnvironment(values map[string]string) func() {
	type saved struct {
		value string
		had   bool
	}
	previous := make(map[string]saved, len(values))
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		value, had := os.LookupEnv(name)
		previous[name] = saved{value: value, had: had}
		os.Setenv(name, values[name])
	}
	return func() {
		for _, name := range names {
			state := previous[name]
			if state.had {
				os.Setenv(name, state.value)
			} else {
				os.Unsetenv(name)
			}
		}
	}
}
