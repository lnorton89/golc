// model.go declares the deployment/instance domain model (CONTEXT
// POOL-02/D-09): a Deployment is a named, saved mapping of logical pool
// members to concrete fixture Instances (mode/universe/address), with
// exactly one Deployment marked Active at a time. Identity for both
// Deployment and Instance is a durable UUIDv7 minted once at creation
// time -- never derived from Name (POOL-01/POOL-02: "identity is never
// derived from a display name").
package deployment

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// channelsPerUniverse is the DMX/Art-Net universe channel-address bound
// (1-512 inclusive) every Instance's [Address, Address+channelCount-1]
// span must stay within (POOL-01 precision probe: a fixture's channel
// span never crosses a 512-channel universe boundary).
const channelsPerUniverse = 512

// maxUniverseSearch bounds NextFreeAddress's universe scan. D-12's
// small-rig scale target (~10-50 fixtures across 3-8 pools) never comes
// close to needing more than a handful of universes; this ceiling is
// generous headroom, not a design limit.
const maxUniverseSearch = 64

// Deployment is a named, saved mapping of logical pool members to
// concrete fixture Instances (POOL-02/D-09). A show may hold multiple
// named deployments (for example, per venue); exactly one may be Active
// at a time.
type Deployment struct {
	ID        uuid.UUID  `json:"id"`
	Name      string     `json:"name"`
	Active    bool       `json:"active"`
	Instances []Instance `json:"instances,omitempty"`
}

// Instance is the concrete patch one pool member maps to: a mode plus a
// universe/address (CONTEXT D-11: 02-05's impact-plan builder auto-
// assigns Universe/Address for new instances by calling NextFreeAddress;
// this plan only provides that allocation primitive and the Instance
// shape it fills in).
type Instance struct {
	ID           uuid.UUID `json:"id"`
	PoolID       uuid.UUID `json:"pool_id"`
	PoolMemberID uuid.UUID `json:"pool_member_id"`
	Mode         string    `json:"mode"`
	Universe     int       `json:"universe"`
	Address      int       `json:"address"`
}

// NewDeployment mints a fresh UUIDv7-identified, inactive Deployment. IDs
// are minted only at creation time -- never derived from Name.
func NewDeployment(name string) (Deployment, error) {
	if strings.TrimSpace(name) == "" {
		return Deployment{}, fmt.Errorf("GOLC_DEPLOYMENT_NAME_EMPTY: deployment name must not be empty")
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Deployment{}, fmt.Errorf("GOLC_DEPLOYMENT_ID_MINT_FAILED: %v", err)
	}
	return Deployment{ID: id, Name: name}, nil
}

// ValidateUniqueNames rejects any two deployments in deployments sharing
// the same Name (POOL-02 idempotency probe): a duplicate name is always
// rejected with a diagnostic, never silently permitted.
func ValidateUniqueNames(deployments []Deployment) error {
	seen := make(map[string]bool, len(deployments))
	for _, d := range deployments {
		if seen[d.Name] {
			return fmt.Errorf("GOLC_DEPLOYMENT_DUPLICATE_NAME: a deployment named %q already exists", d.Name)
		}
		seen[d.Name] = true
	}
	return nil
}

// ValidateSingleActive rejects any deployments slice with more than one
// Active=true entry, guarding POOL-02/D-09's exactly-one-active
// invariant.
func ValidateSingleActive(deployments []Deployment) error {
	activeCount := 0
	for _, d := range deployments {
		if d.Active {
			activeCount++
		}
	}
	if activeCount > 1 {
		return fmt.Errorf("GOLC_DEPLOYMENT_MULTIPLE_ACTIVE: %d deployments are marked active; exactly one is allowed", activeCount)
	}
	return nil
}

// Activate returns a copy of deployments with exactly the named
// deployment Active and every other deployment inactive, so the caller
// can never observe two active deployments even transiently (POOL-02/
// D-09). It fails with GOLC_DEPLOYMENT_NOT_FOUND if no deployment in
// deployments carries the given name; deployments itself is never
// mutated.
func Activate(deployments []Deployment, name string) ([]Deployment, error) {
	found := false
	activated := make([]Deployment, len(deployments))
	for i, d := range deployments {
		d.Active = d.Name == name
		if d.Active {
			found = true
		}
		activated[i] = d
	}
	if !found {
		return nil, fmt.Errorf("GOLC_DEPLOYMENT_NOT_FOUND: no deployment named %q exists", name)
	}
	return activated, nil
}

// ValidateInstanceAddress rejects any Instance whose Universe or Address
// falls outside the valid 1..channelsPerUniverse DMX/Art-Net range
// (POOL-01 precision probe backstop; NextFreeAddress is the primary
// bounds-respecting allocator -- this guard catches a hand-tampered or
// out-of-range address at Load/Save time).
func ValidateInstanceAddress(instance Instance) error {
	if instance.Universe < 1 {
		return fmt.Errorf("GOLC_DEPLOYMENT_ADDRESS_OUT_OF_RANGE: universe %d is below the minimum universe 1", instance.Universe)
	}
	if instance.Address < 1 || instance.Address > channelsPerUniverse {
		return fmt.Errorf("GOLC_DEPLOYMENT_ADDRESS_OUT_OF_RANGE: address %d is outside the valid 1-%d range for universe %d", instance.Address, channelsPerUniverse, instance.Universe)
	}
	return nil
}

// NextFreeAddress returns the next integer (universe, address) slot whose
// [address, address+channelCount-1] span fits inside one 512-channel
// universe without crossing the boundary and without overlapping any
// existing instance's own footprint (POOL-01 precision probe; CONTEXT
// D-11's auto-assignment primitive). Universes are scanned starting at 1
// with addresses starting at 1 within each; within D-12's small-rig scale
// this search space is always small. Instance does not yet carry its own
// channel width (a future plan's concern), so each existing instance is
// conservatively treated as occupying the same channelCount being
// requested for the new instance -- correct for same-width fixtures in
// one pool and, at worst, over-conservative (skips an address that would
// actually have been free) rather than ever returning a colliding
// address. A channelCount that cannot fit in any universe
// (channelCount > channelsPerUniverse) fails with
// GOLC_DEPLOYMENT_ADDRESS_EXHAUSTED, as does exhausting the entire
// maxUniverseSearch scan range.
func NextFreeAddress(existing []Instance, channelCount int) (universe int, address int, err error) {
	if channelCount < 1 {
		return 0, 0, fmt.Errorf("GOLC_DEPLOYMENT_ADDRESS_EXHAUSTED: channelCount must be at least 1, got %d", channelCount)
	}
	if channelCount > channelsPerUniverse {
		return 0, 0, fmt.Errorf("GOLC_DEPLOYMENT_ADDRESS_EXHAUSTED: channelCount %d cannot fit in any %d-channel universe", channelCount, channelsPerUniverse)
	}

	for candidateUniverse := 1; candidateUniverse <= maxUniverseSearch; candidateUniverse++ {
		for candidateAddress := 1; candidateAddress+channelCount-1 <= channelsPerUniverse; candidateAddress++ {
			if !overlapsExisting(existing, candidateUniverse, candidateAddress, channelCount) {
				return candidateUniverse, candidateAddress, nil
			}
		}
	}
	return 0, 0, fmt.Errorf("GOLC_DEPLOYMENT_ADDRESS_EXHAUSTED: no free %d-channel slot found within %d universes", channelCount, maxUniverseSearch)
}

// overlapsExisting reports whether [address, address+channelCount-1] in
// universe overlaps any existing instance's own channelCount-wide
// footprint (see NextFreeAddress's doc comment for the same-width
// simplification this relies on).
func overlapsExisting(existing []Instance, universe, address, channelCount int) bool {
	newEnd := address + channelCount - 1
	for _, instance := range existing {
		if instance.Universe != universe {
			continue
		}
		existingEnd := instance.Address + channelCount - 1
		if address <= existingEnd && instance.Address <= newEnd {
			return true
		}
	}
	return false
}
