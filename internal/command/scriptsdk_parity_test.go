// scriptsdk_parity_test.go is the one file in this repository allowed to
// import both internal/command and internal/scriptsdk (08-03-PLAN.md
// Task 3, CONTEXT SCRP-02): internal/scriptsdk never imports
// internal/command (avoiding an import cycle with internal/command/
// generate.go), so the route-string agreement between the real command
// registry and the SDK's descriptor/exclusion registries is asserted here,
// externally, instead.
//
// TestEveryDeclaredRouteIsClassified proves the coverage-completeness
// contract descriptors.go's package doc comment describes: a route
// declared in NewDefaultCommandRegistry() that is neither an SDK method nor
// an excluded route fails the build, naming the offending route.
// TestNoSDKMethodTargetsUndeclaredRoute proves the reverse: no
// scriptsdk.SDKMethodDescriptor names a route the command registry does
// not actually declare, so the generated SDK can never advertise a
// capability that does not exist.
package command_test

import (
	"sort"
	"strings"
	"testing"

	"github.com/lnorton89/golc/internal/command"
	"github.com/lnorton89/golc/internal/scriptsdk"
	"github.com/stretchr/testify/require"
)

// testOnlyRoutes names every route that exists solely because a _test.go
// file in this package self-registers it through the exact production
// MustDeclareRoute entrypoint (router_test.go's "routerfixture echo" fixture
// route, which proves the self-registration contract itself). Such a route
// is reachable from NewDefaultCommandRegistry() only inside a `go test`
// binary -- it never ships in cmd/golc-project's real binary -- so it is
// not a real command surface for scriptsdk to classify one way or the
// other, and is excluded from both completeness checks below.
var testOnlyRoutes = map[string]bool{
	"routerfixture echo": true,
}

func TestEveryDeclaredRouteIsClassified(t *testing.T) {
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err, "NewDefaultCommandRegistry failed: %v", err)

	exposed := map[string]bool{}
	for _, descriptor := range scriptsdk.RegisteredSDKMethods() {
		exposed[descriptor.Route] = true
	}
	excluded := scriptsdk.RegisteredExclusions()

	var unclassified []string
	for _, registration := range registry.Routes() {
		if testOnlyRoutes[registration.Route] {
			continue
		}
		if exposed[registration.Route] {
			continue
		}
		if _, isExcluded := excluded[registration.Route]; isExcluded {
			continue
		}
		unclassified = append(unclassified, registration.Route)
	}

	sort.Strings(unclassified)
	require.Empty(t, unclassified,
		"GOLC_SCRIPTSDK_ROUTE_UNCLASSIFIED: the following route(s) declared in internal/command are neither "+
			"an exposed scriptsdk SDK method nor a scriptsdk excludedRoutes entry with a reason -- classify "+
			"each one in internal/scriptsdk/descriptors.go: %s",
		strings.Join(unclassified, ", "))
}

func TestNoSDKMethodTargetsUndeclaredRoute(t *testing.T) {
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err, "NewDefaultCommandRegistry failed: %v", err)

	declared := map[string]bool{}
	for _, registration := range registry.Routes() {
		declared[registration.Route] = true
	}

	var undeclared []string
	for _, descriptor := range scriptsdk.RegisteredSDKMethods() {
		if !declared[descriptor.Route] {
			undeclared = append(undeclared, descriptor.Route)
		}
	}
	for route := range scriptsdk.RegisteredExclusions() {
		if !declared[route] {
			undeclared = append(undeclared, route)
		}
	}

	sort.Strings(undeclared)
	require.Empty(t, undeclared,
		"GOLC_SCRIPTSDK_ROUTE_UNDECLARED: the following scriptsdk route(s) do not exist in "+
			"internal/command's registry -- fix the Route string in internal/scriptsdk/descriptors.go: %s",
		strings.Join(undeclared, ", "))
}
