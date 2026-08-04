// "designsystembrowser" (Plan 13-18) is the one serialized-browser Mage
// target this package registers for frontend design-system visual
// enforcement: it dispatches to the self-registered "designsystem
// --browser" command route (internal/command/designsystem.go), which
// runs the pinned-Node Playwright visual suite one worker at a time.
// Unlike checkoffline/build/test above, it is never a step in the
// offline core graph (internal/delivery/graph.go's coreSteps) or the
// commands.pr graph (config/commands.toml) -- it belongs to its own
// required Windows workflow (.github/workflows/design-system.yml, Plan
// 13-18 Task 3), which provisions the lockfile-matched Chromium browser
// explicitly before invoking it. Static/unit design-system checks
// ("designsystem --static"/"--unit") are reachable directly through the
// self-registered command route without a dedicated Mage target of
// their own.
package delivery

import "sort"

// MageTargetKind identifies how the Mage dispatcher implements a target.
// Route targets invoke a self-registered command route; Bootstrap and PR
// use their named Go/config authorities.
type MageTargetKind string

const (
	MageTargetKindRoute     MageTargetKind = "route"
	MageTargetKindBootstrap MageTargetKind = "bootstrap"
	MageTargetKindPR        MageTargetKind = "pr"

	BootstrapEnvironmentName          = "GOLC_BOOTSTRAP_INCLUDE_LINEAR_SYNC"
	BootstrapEnvironmentEnablingValue = "1"
)

// MageEnvironmentOption documents one closed opt-in understood by a target.
type MageEnvironmentOption struct {
	Name          string
	EnablingValue string
	Effect        string
}

// MageTarget is the shared execution and introspection descriptor for one
// public Mage CLI target. Nested slices are copied at every API boundary.
type MageTarget struct {
	Name               string
	Kind               MageTargetKind
	Route              string
	Args               []string
	Authority          string
	EnvironmentOptions []MageEnvironmentOption
}

var mageTargets = []MageTarget{
	{
		Name: "bootstrap", Kind: MageTargetKindBootstrap, Authority: "internal/bootstrap.Bootstrap",
		EnvironmentOptions: []MageEnvironmentOption{{
			Name: BootstrapEnvironmentName, EnablingValue: BootstrapEnvironmentEnablingValue,
			Effect: "bootstrap.Options.IncludeLinearSync",
		}},
	},
	{Name: "build", Kind: MageTargetKindRoute, Route: "build", Authority: "internal/command registry"},
	{Name: "check", Kind: MageTargetKindRoute, Route: "check", Args: []string{"--concern", "project"}, Authority: "internal/command registry"},
	{Name: "checkoffline", Kind: MageTargetKindRoute, Route: "check", Args: []string{"--offline"}, Authority: "internal/command registry"},
	{Name: "designsystembrowser", Kind: MageTargetKindRoute, Route: "designsystem", Args: []string{"--browser"}, Authority: "internal/command registry"},
	{Name: "dev", Kind: MageTargetKindRoute, Route: "dev", Authority: "internal/command registry"},
	{Name: "generate", Kind: MageTargetKindRoute, Route: "generate", Authority: "internal/command registry"},
	{Name: "generatecheck", Kind: MageTargetKindRoute, Route: "generate", Args: []string{"--check"}, Authority: "internal/command registry"},
	{Name: "lint", Kind: MageTargetKindRoute, Route: "lint", Authority: "internal/command registry"},
	{Name: "package", Kind: MageTargetKindRoute, Route: "package", Args: []string{"--foundation"}, Authority: "internal/command registry"},
	{Name: "packagefoundation", Kind: MageTargetKindRoute, Route: "package", Args: []string{"--foundation"}, Authority: "internal/command registry"},
	{Name: "pr", Kind: MageTargetKindPR, Authority: "config/commands.toml: commands.pr.steps, commands.pr.network_steps, commands.pr.mutation_steps"},
	{Name: "run", Kind: MageTargetKindRoute, Route: "run", Authority: "internal/command registry"},
	{Name: "test", Kind: MageTargetKindRoute, Route: "test", Authority: "internal/command registry"},
	{Name: "testquick", Kind: MageTargetKindRoute, Route: "test", Args: []string{"--quick"}, Authority: "internal/command registry"},
}

// MageTargets returns all target descriptors in deterministic Mage CLI name
// order. Both the descriptor slice and every argument slice are defensive
// copies so callers cannot mutate execution authority.
func MageTargets() []MageTarget {
	targets := make([]MageTarget, len(mageTargets))
	for i, target := range mageTargets {
		targets[i] = cloneMageTarget(target)
	}
	sort.Slice(targets, func(i, j int) bool {
		return targets[i].Name < targets[j].Name
	})
	return targets
}

// LookupMageTarget resolves an exact, case-sensitive Mage CLI target name.
// The returned descriptor owns a defensive copy of its argument slice.
func LookupMageTarget(name string) (MageTarget, bool) {
	for _, target := range mageTargets {
		if target.Name == name {
			return cloneMageTarget(target), true
		}
	}
	return MageTarget{}, false
}

func cloneMageTarget(target MageTarget) MageTarget {
	target.Args = append([]string(nil), target.Args...)
	target.EnvironmentOptions = append([]MageEnvironmentOption(nil), target.EnvironmentOptions...)
	return target
}
