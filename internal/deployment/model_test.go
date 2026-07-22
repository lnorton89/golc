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
	"strings"
	"testing"

	"github.com/lnorton89/golc/internal/deployment"
)

func TestDeploymentActivateSingle(t *testing.T) {
	a, err := deployment.NewDeployment("Venue A")
	if err != nil {
		t.Fatalf("NewDeployment: %v", err)
	}
	b, err := deployment.NewDeployment("Venue B")
	if err != nil {
		t.Fatalf("NewDeployment: %v", err)
	}
	deployments := []deployment.Deployment{a, b}

	activated, err := deployment.Activate(deployments, "Venue A")
	if err != nil {
		t.Fatalf("Activate: %v", err)
	}
	assertExactlyOneActive(t, activated, "Venue A")

	flipped, err := deployment.Activate(activated, "Venue B")
	if err != nil {
		t.Fatalf("Activate (flip): %v", err)
	}
	assertExactlyOneActive(t, flipped, "Venue B")

	if err := deployment.ValidateSingleActive(flipped); err != nil {
		t.Fatalf("expected the single-active invariant to hold: %v", err)
	}

	if _, err := deployment.Activate(flipped, "Venue Nonexistent"); err == nil {
		t.Fatal("expected activating a nonexistent deployment name to fail")
	}

	// Directly rig two Active=true deployments to prove the guard rejects it.
	rigged := []deployment.Deployment{a, b}
	rigged[0].Active = true
	rigged[1].Active = true
	if err := deployment.ValidateSingleActive(rigged); err == nil || !strings.Contains(err.Error(), "GOLC_DEPLOYMENT_MULTIPLE_ACTIVE") {
		t.Fatalf("expected GOLC_DEPLOYMENT_MULTIPLE_ACTIVE, got %v", err)
	}
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
	if activeCount != 1 {
		t.Fatalf("expected exactly one active deployment, got %d among %+v", activeCount, deployments)
	}
	if activeName != wantActiveName {
		t.Fatalf("expected %q active, got %q", wantActiveName, activeName)
	}
}

func TestNextFreeAddressBoundary(t *testing.T) {
	const channelCount = 4
	var existing []deployment.Instance
	seenSecondUniverse := false
	for i := 0; i < 150; i++ { // 512/4=128 slots per universe; 150 forces rollover.
		universe, address, err := deployment.NextFreeAddress(existing, channelCount)
		if err != nil {
			t.Fatalf("NextFreeAddress iteration %d: %v", i, err)
		}
		if universe < 1 || address < 1 {
			t.Fatalf("iteration %d: expected a positive universe/address, got universe=%d address=%d", i, universe, address)
		}
		if address+channelCount-1 > 512 {
			t.Fatalf("iteration %d: span crosses the 512-channel universe boundary: universe=%d address=%d channelCount=%d", i, universe, address, channelCount)
		}
		if universe > 1 {
			seenSecondUniverse = true
		}
		existing = append(existing, deployment.Instance{Universe: universe, Address: address})
	}
	if !seenSecondUniverse {
		t.Fatal("expected allocation to roll over into a second universe once the first universe filled up")
	}

	// A channel count larger than any universe can ever hold is rejected.
	if _, _, err := deployment.NextFreeAddress(nil, 513); err == nil {
		t.Fatal("expected an error for a channel count that cannot fit in any universe")
	}

	// Duplicate-name deployment creation is rejected.
	d1, err := deployment.NewDeployment("Venue A")
	if err != nil {
		t.Fatalf("NewDeployment: %v", err)
	}
	d2, err := deployment.NewDeployment("Venue A")
	if err != nil {
		t.Fatalf("NewDeployment (duplicate name): %v", err)
	}
	if err := deployment.ValidateUniqueNames([]deployment.Deployment{d1, d2}); err == nil || !strings.Contains(err.Error(), "GOLC_DEPLOYMENT_DUPLICATE_NAME") {
		t.Fatalf("expected GOLC_DEPLOYMENT_DUPLICATE_NAME, got %v", err)
	}
}
