// mage_targets_test.go covers Plan 13-18's one new registry entry: the
// serialized design-system browser Mage target. The full, exhaustive
// MageTargets() listing (every target, in order) stays the single
// responsibility of delivery_test.go's "MageTargets is the defensive
// deterministic target authority" subtest (updated alongside this file
// so the two can never silently drift apart) -- this file only proves
// the one new entry's own shape in isolation, satisfying this plan's own
// declared verify command's -run 'DesignSystem|MageTarget' filter with a
// distinctly named top-level test function.
package delivery_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/delivery"
)

// TestDesignSystemMageTargetRegistered proves the serialized browser
// design-system target is registry-backed and discoverable through the
// shared Mage descriptor registry: it dispatches to the self-registered
// "designsystem --browser" command route (internal/command/
// designsystem.go), never a second, independently maintained Mage-side
// invocation.
func TestDesignSystemMageTargetRegistered(t *testing.T) {
	target, ok := delivery.LookupMageTarget("designsystembrowser")
	require.True(t, ok, "expected the designsystembrowser Mage target to be registered")
	require.Equal(t, delivery.MageTargetKindRoute, target.Kind, "designsystembrowser target kind")
	require.Equal(t, "designsystem", target.Route, "designsystembrowser target route")
	require.Equal(t, []string{"--browser"}, target.Args, "designsystembrowser target args")
	require.Equal(t, "internal/command registry", target.Authority, "designsystembrowser target authority")
	require.Empty(t, target.EnvironmentOptions, "designsystembrowser target environment options")

	found := false
	for _, candidate := range delivery.MageTargets() {
		if candidate.Name == "designsystembrowser" {
			found = true
			break
		}
	}
	require.True(t, found, "expected designsystembrowser to appear in the full MageTargets() listing")

	_, ok = delivery.LookupMageTarget("designsystem")
	require.False(t, ok, "the bare command route name must not itself be a registered Mage target")
}
