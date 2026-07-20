package command_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/lawrence/golc/internal/command"
	// Command files self-register their routes and scopes (D-03). Importing
	// the package is the only wiring required for the default registry to
	// serve them, which is exactly what cmd/golc-project/main.go does.
	_ "github.com/lawrence/golc/internal/projectconfig"
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
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry failed: %v", err)
	}
	return registry
}

func TestSelfRegisteredFixtureRouteReachableFromDefaultRegistry(t *testing.T) {
	registry := newDefaultRegistry(t)

	registration, rest, ok := registry.Lookup([]string{"routerfixture", "echo", "value-1"})
	if !ok {
		t.Fatal("fixture route declared via MustDeclareRoute was not reachable from NewDefaultCommandRegistry")
	}
	if registration.Route != "routerfixture echo" {
		t.Fatalf("expected normalized route %q, got %q", "routerfixture echo", registration.Route)
	}
	if !reflect.DeepEqual(rest, []string{"value-1"}) {
		t.Fatalf("expected remaining args [value-1], got %v", rest)
	}

	result := registry.Execute(command.Request{Args: []string{"routerfixture", "echo", "value-1"}})
	if result.ExitCode != 0 {
		t.Fatalf("expected exit 0 from fixture handler, got %d (stderr: %s)", result.ExitCode, result.Stderr)
	}
	if got := string(result.Stdout); got != "fixture:value-1" {
		t.Fatalf("expected handler output fixture:value-1, got %q", got)
	}
}

func TestDefaultRegistryServesConfigInspect(t *testing.T) {
	registry := newDefaultRegistry(t)

	registration, rest, ok := registry.Lookup([]string{"config", "inspect", "runtime", "--format", "json"})
	if !ok {
		t.Fatal("config inspect must self-register into the default registry")
	}
	if registration.Route != "config inspect" {
		t.Fatalf("expected route %q, got %q", "config inspect", registration.Route)
	}
	if !reflect.DeepEqual(rest, []string{"runtime", "--format", "json"}) {
		t.Fatalf("expected remaining args [runtime --format json], got %v", rest)
	}
}

func TestRegisterRouteRejectsDuplicateNormalizedRoutes(t *testing.T) {
	registry := command.NewCommandRegistry()
	if err := registry.RegisterScope(command.ScopeRegistration{Scope: "config"}); err != nil {
		t.Fatalf("RegisterScope failed: %v", err)
	}
	handler := func(command.Request) command.Result { return command.Result{} }
	if err := registry.RegisterRoute(command.CommandRegistration{Route: "config inspect", Handler: handler}); err != nil {
		t.Fatalf("first RegisterRoute failed: %v", err)
	}

	err := registry.RegisterRoute(command.CommandRegistration{Route: "  Config   INSPECT ", Handler: handler})
	if err == nil {
		t.Fatal("expected duplicate normalized route to be rejected")
	}
	if !strings.Contains(err.Error(), "GOLC_ROUTE_DUPLICATE: config inspect") {
		t.Fatalf("expected stable GOLC_ROUTE_DUPLICATE diagnostic, got %q", err.Error())
	}
}

func TestRegisterScopeRejectsDuplicateNormalizedScopes(t *testing.T) {
	registry := command.NewCommandRegistry()
	if err := registry.RegisterScope(command.ScopeRegistration{Scope: "config"}); err != nil {
		t.Fatalf("first RegisterScope failed: %v", err)
	}

	err := registry.RegisterScope(command.ScopeRegistration{Scope: " CONFIG "})
	if err == nil {
		t.Fatal("expected duplicate normalized scope to be rejected")
	}
	if !strings.Contains(err.Error(), "GOLC_SCOPE_DUPLICATE: config") {
		t.Fatalf("expected stable GOLC_SCOPE_DUPLICATE diagnostic, got %q", err.Error())
	}
}

func TestRegisterRouteRequiresDeclaredOwningScope(t *testing.T) {
	registry := command.NewCommandRegistry()
	handler := func(command.Request) command.Result { return command.Result{} }

	err := registry.RegisterRoute(command.CommandRegistration{Route: "orphan run", Handler: handler})
	if err == nil {
		t.Fatal("expected a route without a declared owning scope to be rejected")
	}
	if !strings.Contains(err.Error(), "GOLC_ROUTE_SCOPE_UNDECLARED") {
		t.Fatalf("expected stable GOLC_ROUTE_SCOPE_UNDECLARED diagnostic, got %q", err.Error())
	}
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
			if err := registry.RegisterScope(command.ScopeRegistration{Scope: scope}); err != nil {
				t.Fatalf("RegisterScope(%q) failed: %v", scope, err)
			}
		}
		for _, route := range orderedRoutes {
			if err := registry.RegisterRoute(command.CommandRegistration{Route: route, Handler: handler}); err != nil {
				t.Fatalf("RegisterRoute(%q) failed: %v", route, err)
			}
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
	if got := routeKeys(forward); !reflect.DeepEqual(got, wantRoutes) {
		t.Fatalf("forward Routes() not sorted: %v", got)
	}
	if got := routeKeys(backward); !reflect.DeepEqual(got, wantRoutes) {
		t.Fatalf("backward Routes() not sorted: %v", got)
	}
	if got := scopeKeys(forward); !reflect.DeepEqual(got, wantScopes) {
		t.Fatalf("forward Scopes() not sorted: %v", got)
	}
	if got := scopeKeys(backward); !reflect.DeepEqual(got, wantScopes) {
		t.Fatalf("backward Scopes() not sorted: %v", got)
	}
}

func TestLookupPrefersLongestExactRoute(t *testing.T) {
	registry := command.NewCommandRegistry()
	if err := registry.RegisterScope(command.ScopeRegistration{Scope: "config"}); err != nil {
		t.Fatalf("RegisterScope failed: %v", err)
	}
	handler := func(command.Request) command.Result { return command.Result{} }
	for _, route := range []string{"config", "config inspect"} {
		if err := registry.RegisterRoute(command.CommandRegistration{Route: route, Handler: handler}); err != nil {
			t.Fatalf("RegisterRoute(%q) failed: %v", route, err)
		}
	}

	registration, rest, ok := registry.Lookup([]string{"config", "inspect", "runtime"})
	if !ok || registration.Route != "config inspect" {
		t.Fatalf("expected longest match config inspect, got ok=%v route=%q", ok, registration.Route)
	}
	if !reflect.DeepEqual(rest, []string{"runtime"}) {
		t.Fatalf("expected remaining args [runtime], got %v", rest)
	}

	registration, rest, ok = registry.Lookup([]string{"config", "list"})
	if !ok || registration.Route != "config" {
		t.Fatalf("expected fallback match config, got ok=%v route=%q", ok, registration.Route)
	}
	if !reflect.DeepEqual(rest, []string{"list"}) {
		t.Fatalf("expected remaining args [list], got %v", rest)
	}

	if _, _, ok := registry.Lookup([]string{"unknown"}); ok {
		t.Fatal("expected no match for an unregistered route")
	}
}

func TestExecuteUnknownRouteFailsWithStableCode(t *testing.T) {
	registry := newDefaultRegistry(t)

	result := registry.Execute(command.Request{Args: []string{"definitely", "not", "registered"}})
	if result.ExitCode == 0 {
		t.Fatal("expected a nonzero exit code for an unknown route")
	}
	if !strings.Contains(string(result.Stderr), "GOLC_ROUTE_UNKNOWN") {
		t.Fatalf("expected stable GOLC_ROUTE_UNKNOWN diagnostic, got %q", result.Stderr)
	}
}
