// model_test.go proves Deployment's single-active invariant and
// NextFreeAddress's universe-boundary guarantee (02-04-PLAN.md, Task 1
// Wave-0 scaffold): activating one of several deployments always leaves
// exactly one active, ValidateSingleActive rejects a rigged
// multiple-active slice, and NextFreeAddress never returns a span that
// crosses a 512-channel universe boundary even after many allocations
// force a rollover into a new universe.
//
// This file fails at build time until internal/deployment exists
// (Task 2) -- that is the RED state this task proves.
package deployment_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/deployment"
)

func TestDeploymentActivateSingle(t *testing.T) {
	a, err := deployment.NewDeployment("Venue A")
	require.NoError(t, err, "NewDeployment")
	b, err := deployment.NewDeployment("Venue B")
	require.NoError(t, err, "NewDeployment")
	deployments := []deployment.Deployment{a, b}

	activated, err := deployment.Activate(deployments, "Venue A")
	require.NoError(t, err, "Activate")
	assertExactlyOneActive(t, activated, "Venue A")

	flipped, err := deployment.Activate(activated, "Venue B")
	require.NoError(t, err, "Activate (flip)")
	assertExactlyOneActive(t, flipped, "Venue B")

	require.NoError(t, deployment.ValidateSingleActive(flipped), "expected the single-active invariant to hold")

	_, err = deployment.Activate(flipped, "Venue Nonexistent")
	require.Error(t, err, "expected activating a nonexistent deployment name to fail")

	// Directly rig two Active=true deployments to prove the guard rejects it.
	rigged := []deployment.Deployment{a, b}
	rigged[0].Active = true
	rigged[1].Active = true
	err = deployment.ValidateSingleActive(rigged)
	require.ErrorContains(t, err, "GOLC_DEPLOYMENT_MULTIPLE_ACTIVE")
}

func assertExactlyOneActive(t *testing.T, deployments []deployment.Deployment, wantActiveName string) {
	t.Helper()
	activeCount := 0
	var activeName string
	for _, d := range deployments {
		if d.Active {
			activeCount++
			activeName = d.Name
		}
	}
	require.Equal(t, 1, activeCount, "expected exactly one active deployment, got %d among %+v", activeCount, deployments)
	require.Equal(t, wantActiveName, activeName)
}

func TestNextFreeAddressBoundary(t *testing.T) {
	const channelCount = 4
	var existing []deployment.Instance
	seenSecondUniverse := false
	for i := 0; i < 150; i++ { // 512/4=128 slots per universe; 150 forces rollover.
		universe, address, err := deployment.NextFreeAddress(existing, channelCount)
		require.NoError(t, err, "NextFreeAddress iteration %d", i)
		require.False(t, universe < 1 || address < 1, "iteration %d: expected a positive universe/address, got universe=%d address=%d", i, universe, address)
		require.LessOrEqual(t, address+channelCount-1, 512, "iteration %d: span crosses the 512-channel universe boundary: universe=%d address=%d channelCount=%d", i, universe, address, channelCount)
		if universe > 1 {
			seenSecondUniverse = true
		}
		existing = append(existing, deployment.Instance{Universe: universe, Address: address})
	}
	require.True(t, seenSecondUniverse, "expected allocation to roll over into a second universe once the first universe filled up")

	// A channel count larger than any universe can ever hold is rejected.
	_, _, err := deployment.NextFreeAddress(nil, 513)
	require.Error(t, err, "expected an error for a channel count that cannot fit in any universe")

	// Duplicate-name deployment creation is rejected.
	d1, err := deployment.NewDeployment("Venue A")
	require.NoError(t, err, "NewDeployment")
	d2, err := deployment.NewDeployment("Venue A")
	require.NoError(t, err, "NewDeployment (duplicate name)")
	err = deployment.ValidateUniqueNames([]deployment.Deployment{d1, d2})
	require.ErrorContains(t, err, "GOLC_DEPLOYMENT_DUPLICATE_NAME")
}

func TestNextFreeAddressFromStartOverride(t *testing.T) {
	universe, address, err := deployment.NextFreeAddressFrom(nil, 1, 5, 100)
	require.NoError(t, err, "NextFreeAddressFrom")
	require.Equal(t, 5, universe, "expected the scan to anchor at (5, 100)")
	require.Equal(t, 100, address, "expected the scan to anchor at (5, 100)")

	existing := []deployment.Instance{{Universe: 5, Address: 100}}
	universe, address, err = deployment.NextFreeAddressFrom(existing, 1, 5, 100)
	require.NoError(t, err, "NextFreeAddressFrom (occupied start)")
	require.Equal(t, 5, universe, "expected the scan to skip the occupied slot and land at (5, 101)")
	require.Equal(t, 101, address, "expected the scan to skip the occupied slot and land at (5, 101)")

	// Forcing exhaustion of universe 5 from address 510 (channelCount=4, so
	// only address 509 would fit -- 510 doesn't) rolls over to universe 6,
	// address 1, mirroring NextFreeAddress's own per-universe reset.
	universe, address, err = deployment.NextFreeAddressFrom(nil, 4, 5, 510)
	require.NoError(t, err, "NextFreeAddressFrom (rollover)")
	require.Equal(t, 6, universe, "expected rollover to (6, 1)")
	require.Equal(t, 1, address, "expected rollover to (6, 1)")

	_, _, err = deployment.NextFreeAddressFrom(nil, 1, 65, 1)
	require.ErrorContains(t, err, "GOLC_DEPLOYMENT_ADDRESS_EXHAUSTED", "expected error for a start universe beyond the search ceiling")
}

func TestNextFreeAddressFromZeroMeansDefault(t *testing.T) {
	var existing []deployment.Instance
	for i := 0; i < 10; i++ {
		wantUniverse, wantAddress, err := deployment.NextFreeAddress(existing, 4)
		require.NoError(t, err, "NextFreeAddress iteration %d", i)
		gotUniverse, gotAddress, err := deployment.NextFreeAddressFrom(existing, 4, 0, 0)
		require.NoError(t, err, "NextFreeAddressFrom iteration %d", i)
		require.Equal(t, wantUniverse, gotUniverse, "iteration %d", i)
		require.Equal(t, wantAddress, gotAddress, "iteration %d", i)
		existing = append(existing, deployment.Instance{Universe: gotUniverse, Address: gotAddress})
	}
}
