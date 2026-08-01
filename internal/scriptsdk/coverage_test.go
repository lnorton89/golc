// coverage_test.go proves 08-03-PLAN.md Task 2's capability-surface
// completeness discipline: every real descriptor has a non-empty Method, a
// valid Scope, and a Params/Result type that reflects cleanly; no route
// appears in both the exposed set and excludedRoutes; every exclusion
// reason is non-empty; "playback evaluate" is excluded with a reason
// naming frame evaluation; and the committed generated files match what
// GenerateAll produces (CheckDrift returns empty). Route-classification
// EXHAUSTIVENESS against the real internal/command registry (a route
// existing in neither set) is asserted by the external parity test,
// internal/command/scriptsdk_parity_test.go (Task 3) -- this file only
// proves properties of scriptsdk's own registry in isolation, since it
// never imports internal/command.
package scriptsdk_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/scriptsdk"
	"github.com/lnorton89/golc/internal/show"
)

func TestScriptsdkCoverage(t *testing.T) {
	t.Run("every descriptor has a non-empty Method, a valid Scope, and a reflectable Params/Result", testEveryDescriptorWellFormed)
	t.Run("no route appears in both the exposed set and excludedRoutes", testNoRouteExposedAndExcluded)
	t.Run("every exclusion reason is non-empty", testEveryExclusionReasonNonEmpty)
	t.Run("playback evaluate is excluded with a reason naming frame evaluation", testPlaybackEvaluateExcluded)
	t.Run("the committed generated files match GenerateAll's output", testCommittedFilesMatchGenerated)
}

func testEveryDescriptorWellFormed(t *testing.T) {
	validScopes := map[show.APIKeyScope]bool{
		show.APIKeyScopePlayback:  true,
		show.APIKeyScopeAuthoring: true,
		show.APIKeyScopeAdmin:     true,
	}

	descriptors := scriptsdk.RegisteredSDKMethods()
	require.NotEmpty(t, descriptors, "expected at least one registered SDK method")

	for _, descriptor := range descriptors {
		require.NotEmpty(t, strings.TrimSpace(descriptor.Method), "route %q: expected a non-empty Method", descriptor.Route)
		require.True(t, validScopes[descriptor.Scope], "route %q: expected Scope to be one of playback/authoring/admin, got %q", descriptor.Route, descriptor.Scope)
		require.NotNil(t, descriptor.Params, "route %q: expected a non-nil Params value", descriptor.Route)
		require.NotNil(t, descriptor.Result, "route %q: expected a non-nil Result value", descriptor.Route)
	}
}

func testNoRouteExposedAndExcluded(t *testing.T) {
	exposed := map[string]bool{}
	for _, descriptor := range scriptsdk.RegisteredSDKMethods() {
		exposed[descriptor.Route] = true
	}
	for route := range scriptsdk.RegisteredExclusions() {
		require.False(t, exposed[route], "route %q is both exposed and excluded", route)
	}
}

func testEveryExclusionReasonNonEmpty(t *testing.T) {
	exclusions := scriptsdk.RegisteredExclusions()
	require.NotEmpty(t, exclusions, "expected at least one excluded route")
	for route, reason := range exclusions {
		require.NotEmpty(t, strings.TrimSpace(reason), "route %q has a blank exclusion reason", route)
	}
}

func testPlaybackEvaluateExcluded(t *testing.T) {
	exclusions := scriptsdk.RegisteredExclusions()
	reason, excluded := exclusions["playback evaluate"]
	require.True(t, excluded, `expected "playback evaluate" to be excluded`)
	require.Contains(t, reason, "frame evaluation", `expected "playback evaluate"'s exclusion reason to mention frame evaluation, got %q`, reason)
}

func testCommittedFilesMatchGenerated(t *testing.T) {
	changed, err := scriptsdk.CheckDrift(coverageRepositoryRoot(t))
	require.NoError(t, err, "CheckDrift failed")
	require.Empty(t, changed, "expected the committed generated files to match GenerateAll's output, drift: %v", changed)
}

// coverageRepositoryRoot resolves the real repository root from this
// package's test working directory (internal/scriptsdk -> repo root is two
// levels up), mirroring internal/command/check_test.go's identical
// commandParityRepositoryRoot helper.
func coverageRepositoryRoot(t *testing.T) string {
	t.Helper()
	workingDirectory, err := os.Getwd()
	require.NoError(t, err, "os.Getwd")
	return filepath.Clean(filepath.Join(workingDirectory, "..", ".."))
}
