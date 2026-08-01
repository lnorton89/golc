package command_test

import (
	"strings"
	"testing"

	"github.com/lnorton89/golc/internal/command"
	// Command files self-register their routes and scopes (D-03). Importing
	// the package is the only wiring required for the default registry to
	// serve them, which is exactly what cmd/golc-project/main.go does.
	_ "github.com/lnorton89/golc/internal/projectconfig"

	"github.com/stretchr/testify/require"
)

// The fixture route and scope below are declared through the exact
// package-level entrypoints every later command file uses. If they are
// reachable from NewDefaultCommandRegistry, any future command file can
// self-register without editing internal/command/router.go.
var _ = command.MustDeclareScope(command.ScopeRegistration{
	Scope:   "routerfixture",
	Summary: "Router contract fixture scope.",
})

var _ = command.MustDeclareRoute(command.CommandRegistration{
	Route:   "routerfixture echo",
	Summary: "Router contract fixture route.",
	Handler: func(req command.Request) command.Result {
		return command.Result{Stdout: []byte("fixture:" + strings.Join(req.Args, ","))}
	},
})

func newDefaultRegistry(t *testing.T) *command.CommandRegistry {
	t.Helper()
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err, "NewDefaultCommandRegistry failed: %v", err)
	return registry
}

func TestSelfRegisteredFixtureRouteReachableFromDefaultRegistry(t *testing.T) {
	registry := newDefaultRegistry(t)

	registration, rest, ok := registry.Lookup([]string{"routerfixture", "echo", "value-1"})
	require.True(t, ok, "fixture route declared via MustDeclareRoute was not reachable from NewDefaultCommandRegistry")
	require.Equal(t, "routerfixture echo", registration.Route, "expected normalized route %q, got %q", "routerfixture echo", registration.Route)
	require.Equal(t, []string{"value-1"}, rest, "expected remaining args [value-1], got %v", rest)

	result := registry.Execute(command.Request{Args: []string{"routerfixture", "echo", "value-1"}})
	require.Equal(t, 0, result.ExitCode, "expected exit 0 from fixture handler, got %d (stderr: %s)", result.ExitCode, result.Stderr)
	got := string(result.Stdout)
	require.Equal(t, "fixture:value-1", got, "expected handler output fixture:value-1, got %q", got)
}

func TestDefaultRegistryServesConfigInspect(t *testing.T) {
	registry := newDefaultRegistry(t)

	registration, rest, ok := registry.Lookup([]string{"config", "inspect", "runtime", "--format", "json"})
	require.True(t, ok, "config inspect must self-register into the default registry")
	require.Equal(t, "config inspect", registration.Route, "expected route %q, got %q", "config inspect", registration.Route)
	require.Equal(t, []string{"runtime", "--format", "json"}, rest, "expected remaining args [runtime --format json], got %v", rest)
}

func TestRegisterRouteRejectsDuplicateNormalizedRoutes(t *testing.T) {
	registry := command.NewCommandRegistry()
	err := registry.RegisterScope(command.ScopeRegistration{Scope: "config"})
	require.NoError(t, err, "RegisterScope failed: %v", err)
	handler := func(command.Request) command.Result { return command.Result{} }
	err = registry.RegisterRoute(command.CommandRegistration{Route: "config inspect", Handler: handler})
	require.NoError(t, err, "first RegisterRoute failed: %v", err)

	err = registry.RegisterRoute(command.CommandRegistration{Route: "  Config   INSPECT ", Handler: handler})
	require.Error(t, err, "expected duplicate normalized route to be rejected")
	require.Contains(t, err.Error(), "GOLC_ROUTE_DUPLICATE: config inspect", "expected stable GOLC_ROUTE_DUPLICATE diagnostic, got %q", err.Error())
}

func TestRegisterScopeRejectsDuplicateNormalizedScopes(t *testing.T) {
	registry := command.NewCommandRegistry()
	err := registry.RegisterScope(command.ScopeRegistration{Scope: "config"})
	require.NoError(t, err, "first RegisterScope failed: %v", err)

	err = registry.RegisterScope(command.ScopeRegistration{Scope: " CONFIG "})
	require.Error(t, err, "expected duplicate normalized scope to be rejected")
	require.Contains(t, err.Error(), "GOLC_SCOPE_DUPLICATE: config", "expected stable GOLC_SCOPE_DUPLICATE diagnostic, got %q", err.Error())
}

func TestRegisterRouteRequiresDeclaredOwningScope(t *testing.T) {
	registry := command.NewCommandRegistry()
	handler := func(command.Request) command.Result { return command.Result{} }

	err := registry.RegisterRoute(command.CommandRegistration{Route: "orphan run", Handler: handler})
	require.Error(t, err, "expected a route without a declared owning scope to be rejected")
	require.Contains(t, err.Error(), "GOLC_ROUTE_SCOPE_UNDECLARED", "expected stable GOLC_ROUTE_SCOPE_UNDECLARED diagnostic, got %q", err.Error())
}

func TestRoutesAndScopesAreDeterministicAcrossDeclarationOrder(t *testing.T) {
	handler := func(command.Request) command.Result { return command.Result{} }
	scopes := []string{"zeta", "alpha", "mid"}
	routes := []string{"zeta run", "alpha run", "mid check", "alpha list"}

	build := func(reverse bool) *command.CommandRegistry {
		registry := command.NewCommandRegistry()
		orderedScopes := append([]string(nil), scopes...)
		orderedRoutes := append([]string(nil), routes...)
		if reverse {
			for i, j := 0, len(orderedScopes)-1; i < j; i, j = i+1, j-1 {
				orderedScopes[i], orderedScopes[j] = orderedScopes[j], orderedScopes[i]
			}
			for i, j := 0, len(orderedRoutes)-1; i < j; i, j = i+1, j-1 {
				orderedRoutes[i], orderedRoutes[j] = orderedRoutes[j], orderedRoutes[i]
			}
		}
		for _, scope := range orderedScopes {
			err := registry.RegisterScope(command.ScopeRegistration{Scope: scope})
			require.NoError(t, err, "RegisterScope(%q) failed: %v", scope, err)
		}
		for _, route := range orderedRoutes {
			err := registry.RegisterRoute(command.CommandRegistration{Route: route, Handler: handler})
			require.NoError(t, err, "RegisterRoute(%q) failed: %v", route, err)
		}
		return registry
	}

	forward := build(false)
	backward := build(true)

	routeKeys := func(registry *command.CommandRegistry) []string {
		keys := []string{}
		for _, registration := range registry.Routes() {
			keys = append(keys, registration.Route)
		}
		return keys
	}
	scopeKeys := func(registry *command.CommandRegistry) []string {
		keys := []string{}
		for _, registration := range registry.Scopes() {
			keys = append(keys, registration.Scope)
		}
		return keys
	}

	wantRoutes := []string{"alpha list", "alpha run", "mid check", "zeta run"}
	wantScopes := []string{"alpha", "mid", "zeta"}
	require.Equal(t, wantRoutes, routeKeys(forward), "forward Routes() not sorted: %v", routeKeys(forward))
	require.Equal(t, wantRoutes, routeKeys(backward), "backward Routes() not sorted: %v", routeKeys(backward))
	require.Equal(t, wantScopes, scopeKeys(forward), "forward Scopes() not sorted: %v", scopeKeys(forward))
	require.Equal(t, wantScopes, scopeKeys(backward), "backward Scopes() not sorted: %v", scopeKeys(backward))
}

func TestLookupPrefersLongestExactRoute(t *testing.T) {
	registry := command.NewCommandRegistry()
	err := registry.RegisterScope(command.ScopeRegistration{Scope: "config"})
	require.NoError(t, err, "RegisterScope failed: %v", err)
	handler := func(command.Request) command.Result { return command.Result{} }
	for _, route := range []string{"config", "config inspect"} {
		err := registry.RegisterRoute(command.CommandRegistration{Route: route, Handler: handler})
		require.NoError(t, err, "RegisterRoute(%q) failed: %v", route, err)
	}

	registration, rest, ok := registry.Lookup([]string{"config", "inspect", "runtime"})
	require.True(t, ok && registration.Route == "config inspect", "expected longest match config inspect, got ok=%v route=%q", ok, registration.Route)
	require.Equal(t, []string{"runtime"}, rest, "expected remaining args [runtime], got %v", rest)

	registration, rest, ok = registry.Lookup([]string{"config", "list"})
	require.True(t, ok && registration.Route == "config", "expected fallback match config, got ok=%v route=%q", ok, registration.Route)
	require.Equal(t, []string{"list"}, rest, "expected remaining args [list], got %v", rest)

	_, _, ok = registry.Lookup([]string{"unknown"})
	require.False(t, ok, "expected no match for an unregistered route")
}

func TestExecuteUnknownRouteFailsWithStableCode(t *testing.T) {
	registry := newDefaultRegistry(t)

	result := registry.Execute(command.Request{Args: []string{"definitely", "not", "registered"}})
	require.NotEqual(t, 0, result.ExitCode, "expected a nonzero exit code for an unknown route")
	require.Contains(t, string(result.Stderr), "GOLC_ROUTE_UNKNOWN", "expected stable GOLC_ROUTE_UNKNOWN diagnostic, got %q", result.Stderr)
}

// TestExecuteBareInvocationPrintsUsage proves a zero-argument invocation
// (the exact shape a user gets from running the CLI binary with no
// subcommand) gets an actionable route listing instead of the generic
// GOLC_ROUTE_UNKNOWN: no registered route matches "" diagnostic.
func TestExecuteBareInvocationPrintsUsage(t *testing.T) {
	registry := newDefaultRegistry(t)

	result := registry.Execute(command.Request{Args: []string{}})
	require.NotEqual(t, 0, result.ExitCode, "expected a nonzero exit code for a bare invocation")
	stderr := string(result.Stderr)
	require.NotContains(t, stderr, `GOLC_ROUTE_UNKNOWN: no registered route matches ""`, "expected a usage listing, not the generic unknown-route diagnostic, got %q", stderr)
	require.Contains(t, stderr, "GOLC_ROUTE_MISSING", "expected GOLC_ROUTE_MISSING diagnostic, got %q", stderr)
	require.Contains(t, stderr, "routerfixture echo", "expected the usage listing to include a registered route, got %q", stderr)
}
