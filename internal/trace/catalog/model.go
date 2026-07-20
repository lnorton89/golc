// Package catalog implements the repository-owned planning identity
// catalog (CONTEXT D-11/D-12/D-14): durable offline local IDs for the
// project, active milestone, phases, requirements, dynamically discovered
// plans, and executable tasks, together with the typed authority split
// between repository artifacts and Linear operational fields.
//
// The catalog is built entirely from committed planning artifacts and is
// complete offline. Linear is never consulted; remote UUID mappings live
// in .planning/linear-map.json and are optional, never local identity.
package catalog

import (
	"fmt"
	"strings"
)

// Kind names one durable entity kind in the local identity graph.
type Kind string

// The six catalog entity kinds, ordered from root to leaf.
const (
	KindProject     Kind = "project"
	KindMilestone   Kind = "milestone"
	KindPhase       Kind = "phase"
	KindRequirement Kind = "req"
	KindPlan        Kind = "plan"
	KindTask        Kind = "task"
)

// Authority names which side owns a mapped field (CONTEXT D-11).
type Authority string

const (
	// AuthorityRepository marks fields owned by committed repository
	// artifacts: scope, durable local IDs, requirement text, and roadmap
	// phase structure.
	AuthorityRepository Authority = "repository"
	// AuthorityLinear marks operational execution fields owned by Linear:
	// status, assignee, priority, estimate, and completion timestamps.
	AuthorityLinear Authority = "linear"
)

// Entity is one durable node in the local identity graph. The ID derives
// exclusively from durable structural metadata; Display carries
// human-facing text that may be renamed at any time without changing
// identity (CONTEXT D-14). Entities deliberately have no comment or
// discussion storage (CONTEXT D-12).
type Entity struct {
	// ID is the durable local identifier, e.g. "task:01-08.1".
	ID string
	// Kind is the entity kind encoded by the ID.
	Kind Kind
	// Parent is the ID of the containing entity; empty only for the project.
	Parent string
	// Display is display-only text; renaming it never alters identity.
	Display string
	// Source is the repository-relative planning artifact (slash-separated,
	// under .planning/) this entity was parsed from.
	Source string
}

// Catalog is the complete local identity graph plus the typed field
// authority registry. Entity order is deterministic build order.
type Catalog struct {
	Entities    []Entity
	Authorities map[string]Authority
}

// fieldOwnership is the fixed D-11 authority registry. Repository fields
// can never be reassigned to Linear and Linear operational fields can
// never be claimed by the repository; the split is typed, not negotiated.
var fieldOwnership = map[string]Authority{
	"scope":            AuthorityRepository,
	"local_id":         AuthorityRepository,
	"requirement_text": AuthorityRepository,
	"structure":        AuthorityRepository,
	"status":           AuthorityLinear,
	"assignee":         AuthorityLinear,
	"priority":         AuthorityLinear,
	"estimate":         AuthorityLinear,
	"completed_at":     AuthorityLinear,
}

// commentFieldNames enumerates the excluded comment/discussion fields:
// Linear comments and discussion stay in Linear and are never stored in
// repository planning artifacts (CONTEXT D-12).
var commentFieldNames = map[string]bool{
	"comment":     true,
	"comments":    true,
	"discussion":  true,
	"discussions": true,
}

// DefaultAuthorities returns a fresh copy of the fixed D-11 authority
// registry.
func DefaultAuthorities() map[string]Authority {
	authorities := make(map[string]Authority, len(fieldOwnership))
	for field, owner := range fieldOwnership {
		authorities[field] = owner
	}
	return authorities
}

// NewCatalog returns an empty catalog carrying the fixed authority split.
func NewCatalog() *Catalog {
	return &Catalog{Authorities: DefaultAuthorities()}
}

// Add appends one entity after enforcing ID grammar, kind consistency,
// and uniqueness.
func (c *Catalog) Add(entity Entity) error {
	parsed, err := ParseID(entity.ID)
	if err != nil {
		return err
	}
	if parsed.Kind != entity.Kind {
		return fmt.Errorf("GOLC_CATALOG_KIND_MISMATCH: %s declares kind %q but its id encodes %q",
			entity.ID, entity.Kind, parsed.Kind)
	}
	if _, exists := c.Lookup(entity.ID); exists {
		return fmt.Errorf("GOLC_CATALOG_ID_DUPLICATE: %s", entity.ID)
	}
	c.Entities = append(c.Entities, entity)
	return nil
}

// Lookup returns the entity with the given ID, scanning catalog order.
func (c *Catalog) Lookup(id string) (Entity, bool) {
	for _, entity := range c.Entities {
		if entity.ID == id {
			return entity, true
		}
	}
	return Entity{}, false
}

// SetAuthority records field ownership. The D-11 split is fixed: a
// repository field cannot be reassigned to Linear, a Linear operational
// field cannot be claimed by the repository, comment fields cannot be
// stored at all, and unknown fields are rejected outright.
func (c *Catalog) SetAuthority(field string, owner Authority) error {
	normalized := strings.ToLower(strings.TrimSpace(field))
	if commentFieldNames[normalized] {
		return fmt.Errorf("GOLC_CATALOG_COMMENT_EXCLUDED: %q: Linear comments and discussion are never stored in repository planning artifacts", field)
	}
	fixed, known := fieldOwnership[normalized]
	if !known {
		return fmt.Errorf("GOLC_CATALOG_FIELD_UNKNOWN: %q is not a registered mapped field", field)
	}
	if owner != fixed {
		if fixed == AuthorityRepository {
			return fmt.Errorf("GOLC_CATALOG_AUTHORITY_REPOSITORY_FIELD: %q is owned by repository artifacts and cannot be reassigned to %s", field, owner)
		}
		return fmt.Errorf("GOLC_CATALOG_AUTHORITY_LINEAR_FIELD: %q is a Linear operational field and cannot be reassigned to %s", field, owner)
	}
	if c.Authorities == nil {
		c.Authorities = DefaultAuthorities()
	}
	c.Authorities[normalized] = owner
	return nil
}
