// model.go declares the pool/group domain model (CONTEXT POOL-01/D-10):
// a Pool is a named logical grouping of compatible fixtures defined
// independently of concrete member count, address, or deployment
// hardware; a Group is an independent, cross-pool named selection.
// Identity for both is a durable UUIDv7 minted once at creation time --
// never derived from Name, and never re-minted by Rename (POOL-01:
// "identity is never derived from a display name").
package pool

import (
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/fixture"
)

// Pool is a named logical grouping of compatible fixtures, independent of
// concrete member count, address, or deployment hardware (POOL-01).
// RequiredCapabilities documents the semantic capabilities a compatible
// fixture must offer; Members starts empty for a newly created pool and
// may grow to any count -- pool definition never requires a member count
// (POOL-01 boundary probe: 0, 1, and the D-12 ceiling are all valid).
type Pool struct {
	ID                   uuid.UUID               `json:"id"`
	Name                 string                  `json:"name"`
	RequiredCapabilities []fixture.CapabilityType `json:"required_capabilities,omitempty"`
	Members              []PoolMember            `json:"members,omitempty"`
}

// PoolMember is one fixture pinned into a Pool by its stable identity
// (FIXT-05's content-addressed pin). PoolMember carries its own UUIDv7 ID,
// independent of the pinned fixture identity, so an Instance can
// reference a stable PoolMemberID even if the underlying fixture pin is
// later updated.
type PoolMember struct {
	ID                 uuid.UUID `json:"id"`
	FixtureStableKey   string    `json:"fixture_stable_key"`
	FixtureContentHash string    `json:"fixture_content_hash"`
}

// Group is an independent, cross-pool named selection (CONTEXT D-10) --
// orthogonal to Pool, which exists purely to abstract fixture count and
// identity.
type Group struct {
	ID         uuid.UUID   `json:"id"`
	Name       string      `json:"name"`
	MemberRefs []MemberRef `json:"member_refs,omitempty"`
}

// MemberRef selects one PoolMember of one Pool for inclusion in a Group.
type MemberRef struct {
	PoolID       uuid.UUID `json:"pool_id"`
	PoolMemberID uuid.UUID `json:"pool_member_id"`
}

// supportedCapabilityTypes mirrors internal/fixture's declared enum as a
// lookup set (built once), reused to validate a Pool's
// RequiredCapabilities against the same nine-value CapabilityType enum
// FIXT-02 validates fixture capabilities against.
var supportedCapabilityTypes = func() map[fixture.CapabilityType]bool {
	set := make(map[fixture.CapabilityType]bool, len(fixture.SupportedCapabilityTypes))
	for _, capabilityType := range fixture.SupportedCapabilityTypes {
		set[capabilityType] = true
	}
	return set
}()

func validateRequiredCapabilities(name string, requiredCapabilities []fixture.CapabilityType) error {
	for _, capabilityType := range requiredCapabilities {
		if !supportedCapabilityTypes[capabilityType] {
			return fmt.Errorf("GOLC_POOL_CAPABILITY_UNSUPPORTED: %q is not a supported capability type for pool %q", capabilityType, name)
		}
	}
	return nil
}

// NewPool mints a fresh UUIDv7-identified Pool with zero members. IDs are
// minted only at creation time (POOL-01) -- never derived from Name, and
// never re-minted by Rename.
func NewPool(name string, requiredCapabilities []fixture.CapabilityType) (Pool, error) {
	if strings.TrimSpace(name) == "" {
		return Pool{}, fmt.Errorf("GOLC_POOL_NAME_EMPTY: pool name must not be empty")
	}
	if err := validateRequiredCapabilities(name, requiredCapabilities); err != nil {
		return Pool{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Pool{}, fmt.Errorf("GOLC_POOL_ID_MINT_FAILED: %v", err)
	}
	return Pool{ID: id, Name: name, RequiredCapabilities: requiredCapabilities}, nil
}

// Rename returns p with Name replaced by newName; ID is never re-minted
// (POOL-01: identity is rename-stable).
func Rename(p Pool, newName string) (Pool, error) {
	if strings.TrimSpace(newName) == "" {
		return Pool{}, fmt.Errorf("GOLC_POOL_NAME_EMPTY: pool name must not be empty")
	}
	p.Name = newName
	return p, nil
}

// NewPoolMember mints a fresh UUIDv7-identified PoolMember pinned to the
// given fixture stable key/content hash.
func NewPoolMember(fixtureStableKey, fixtureContentHash string) (PoolMember, error) {
	if strings.TrimSpace(fixtureStableKey) == "" {
		return PoolMember{}, fmt.Errorf("GOLC_POOL_MEMBER_KEY_EMPTY: fixture stable key must not be empty")
	}
	id, err := uuid.NewV7()
	if err != nil {
		return PoolMember{}, fmt.Errorf("GOLC_POOL_ID_MINT_FAILED: %v", err)
	}
	return PoolMember{ID: id, FixtureStableKey: fixtureStableKey, FixtureContentHash: fixtureContentHash}, nil
}

// Validate enforces per-Pool invariants: p is valid regardless of member
// count (POOL-01 boundary probe: 0, 1, and the D-12 ceiling are all
// valid), and every RequiredCapabilities entry is a supported
// CapabilityType.
func Validate(p Pool) error {
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("GOLC_POOL_NAME_EMPTY: pool %s declares an empty name", p.ID)
	}
	return validateRequiredCapabilities(p.Name, p.RequiredCapabilities)
}

// ValidateUniqueNames rejects any two pools in pools sharing the same
// Name (POOL-02 idempotency probe, applied to pools): a duplicate name is
// always rejected with a diagnostic, never silently permitted.
func ValidateUniqueNames(pools []Pool) error {
	seen := make(map[string]bool, len(pools))
	for _, p := range pools {
		if seen[p.Name] {
			return fmt.Errorf("GOLC_POOL_DUPLICATE_NAME: a pool named %q already exists", p.Name)
		}
		seen[p.Name] = true
	}
	return nil
}

// ValidateUniqueGroupNames rejects any two groups in groups sharing the
// same Name, mirroring ValidateUniqueNames' duplicate-name guarantee for
// Pool/Deployment: a duplicate Group name is always rejected with a
// diagnostic, never silently permitted (WR-02).
func ValidateUniqueGroupNames(groups []Group) error {
	seen := make(map[string]bool, len(groups))
	for _, g := range groups {
		if seen[g.Name] {
			return fmt.Errorf("GOLC_GROUP_DUPLICATE_NAME: a group named %q already exists", g.Name)
		}
		seen[g.Name] = true
	}
	return nil
}

// ValidateGroupReferences rejects any Group whose MemberRef points at a
// pool or pool member that does not exist in pools (WR-02): a dangling
// reference is always rejected with a diagnostic at Load/Save time,
// never silently permitted to persist.
func ValidateGroupReferences(pools []Pool, groups []Group) error {
	membersByPool := make(map[uuid.UUID]map[uuid.UUID]bool, len(pools))
	for _, p := range pools {
		members := make(map[uuid.UUID]bool, len(p.Members))
		for _, m := range p.Members {
			members[m.ID] = true
		}
		membersByPool[p.ID] = members
	}

	for _, g := range groups {
		for _, ref := range g.MemberRefs {
			members, poolExists := membersByPool[ref.PoolID]
			if !poolExists {
				return fmt.Errorf(
					"GOLC_GROUP_DANGLING_REFERENCE: group %q references pool %s, which does not exist",
					g.Name, ref.PoolID)
			}
			if !members[ref.PoolMemberID] {
				return fmt.Errorf(
					"GOLC_GROUP_DANGLING_REFERENCE: group %q references pool member %s in pool %s, which does not exist",
					g.Name, ref.PoolMemberID, ref.PoolID)
			}
		}
	}
	return nil
}
