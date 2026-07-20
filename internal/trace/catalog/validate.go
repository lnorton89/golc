// validate.go enforces the catalog invariants (CONTEXT D-11/D-12/D-14):
// grammar-valid unique IDs, a parent graph with correct kinds and no
// cycles, structural containment between child and parent IDs, sources
// contained inside the repository planning tree, and the fixed typed
// authority split with comment exclusion.
package catalog

import (
	"fmt"
	"sort"
	"strings"
)

// parentKindRules maps each kind to the kind its parent must have. The
// project is the root and must have no parent.
var parentKindRules = map[Kind]Kind{
	KindMilestone:   KindProject,
	KindPhase:       KindMilestone,
	KindRequirement: KindPhase,
	KindPlan:        KindPhase,
	KindTask:        KindPlan,
}

// Validate runs every catalog validator in a fixed order and returns the
// first violation.
func Validate(c *Catalog) error {
	if err := ValidateIDs(c); err != nil {
		return err
	}
	if err := ValidateHierarchy(c); err != nil {
		return err
	}
	if err := ValidateSources(c); err != nil {
		return err
	}
	return ValidateAuthorities(c)
}

// ValidateIDs enforces the ID grammar, ID/kind consistency, and
// uniqueness across the whole catalog.
func ValidateIDs(c *Catalog) error {
	seen := map[string]bool{}
	for _, entity := range c.Entities {
		parsed, err := ParseID(entity.ID)
		if err != nil {
			return err
		}
		if parsed.Kind != entity.Kind {
			return fmt.Errorf("GOLC_CATALOG_KIND_MISMATCH: %s declares kind %q but its id encodes %q",
				entity.ID, entity.Kind, parsed.Kind)
		}
		if seen[entity.ID] {
			return fmt.Errorf("GOLC_CATALOG_ID_DUPLICATE: %s", entity.ID)
		}
		seen[entity.ID] = true
	}
	return nil
}

// ValidateHierarchy enforces parent existence, acyclicity, parent-kind
// rules, and structural containment between child and parent IDs.
func ValidateHierarchy(c *Catalog) error {
	byID := make(map[string]Entity, len(c.Entities))
	for _, entity := range c.Entities {
		byID[entity.ID] = entity
	}

	for _, entity := range c.Entities {
		if entity.Parent == "" {
			continue
		}
		if _, exists := byID[entity.Parent]; !exists {
			return fmt.Errorf("GOLC_CATALOG_PARENT_UNKNOWN: %s references parent %s which is not in the catalog",
				entity.ID, entity.Parent)
		}
	}

	for _, entity := range c.Entities {
		visited := map[string]bool{entity.ID: true}
		current := entity
		for current.Parent != "" {
			if visited[current.Parent] {
				return fmt.Errorf("GOLC_CATALOG_CYCLE: parent chain of %s revisits %s", entity.ID, current.Parent)
			}
			visited[current.Parent] = true
			current = byID[current.Parent]
		}
	}

	for _, entity := range c.Entities {
		if entity.Kind == KindProject {
			if entity.Parent != "" {
				return fmt.Errorf("GOLC_CATALOG_PARENT_KIND: project %s must not have a parent", entity.ID)
			}
			continue
		}
		requiredKind, known := parentKindRules[entity.Kind]
		if !known {
			return fmt.Errorf("GOLC_CATALOG_KIND_MISMATCH: %s has unregistered kind %q", entity.ID, entity.Kind)
		}
		if entity.Parent == "" {
			return fmt.Errorf("GOLC_CATALOG_PARENT_UNKNOWN: %s must have a %s parent", entity.ID, requiredKind)
		}
		parent := byID[entity.Parent]
		if parent.Kind != requiredKind {
			return fmt.Errorf("GOLC_CATALOG_PARENT_KIND: %s requires a %s parent but %s is a %s",
				entity.ID, requiredKind, parent.ID, parent.Kind)
		}
		if err := validateStructuralContainment(entity, parent); err != nil {
			return err
		}
	}
	return nil
}

// validateStructuralContainment checks that the structural metadata
// encoded in a child ID matches its parent's, so a plan can only belong
// to its own phase and a task to its own plan.
func validateStructuralContainment(entity, parent Entity) error {
	childParsed, err := ParseID(entity.ID)
	if err != nil {
		return err
	}
	parentParsed, err := ParseID(parent.ID)
	if err != nil {
		return err
	}
	switch entity.Kind {
	case KindPlan:
		if childParsed.Phase != parentParsed.Phase {
			return fmt.Errorf("GOLC_CATALOG_PARENT_MISMATCH: %s encodes phase %s but is parented to %s",
				entity.ID, childParsed.Phase, parent.ID)
		}
	case KindTask:
		if childParsed.Phase != parentParsed.Phase || childParsed.Plan != parentParsed.Plan {
			return fmt.Errorf("GOLC_CATALOG_PARENT_MISMATCH: %s encodes plan %s-%s but is parented to %s",
				entity.ID, childParsed.Phase, childParsed.Plan, parent.ID)
		}
	}
	return nil
}

// ValidateSources enforces repository containment: every entity source is
// a relative slash path inside .planning/ with no escape or external
// reference. Linear URLs, absolute paths, and anything outside the
// planning tree are rejected (CONTEXT D-11: repository text is the
// authority).
func ValidateSources(c *Catalog) error {
	for _, entity := range c.Entities {
		source := entity.Source
		if source == "" {
			return fmt.Errorf("GOLC_CATALOG_SOURCE_MISSING: %s has no source artifact", entity.ID)
		}
		switch {
		case strings.Contains(source, "://"):
			return fmt.Errorf("GOLC_CATALOG_SOURCE_EXTERNAL: %s source %q is an external reference", entity.ID, source)
		case strings.Contains(source, "\\"):
			return fmt.Errorf("GOLC_CATALOG_SOURCE_EXTERNAL: %s source %q must use forward slashes", entity.ID, source)
		case strings.HasPrefix(source, "/") || strings.Contains(source, ":"):
			return fmt.Errorf("GOLC_CATALOG_SOURCE_EXTERNAL: %s source %q is not repository-relative", entity.ID, source)
		}
		for _, segment := range strings.Split(source, "/") {
			if segment == ".." || segment == "" {
				return fmt.Errorf("GOLC_CATALOG_SOURCE_EXTERNAL: %s source %q escapes the repository", entity.ID, source)
			}
		}
		if source != ".planning" && !strings.HasPrefix(source, ".planning/") {
			return fmt.Errorf("GOLC_CATALOG_SOURCE_EXTERNAL: %s source %q is outside the planning tree", entity.ID, source)
		}
	}
	return nil
}

// ValidateAuthorities re-checks the authority registry against the fixed
// D-11 split so even direct map tampering cannot reassign a repository
// field to Linear, claim a Linear operational field, store comment
// fields, or drop registered fields.
func ValidateAuthorities(c *Catalog) error {
	fields := make([]string, 0, len(c.Authorities))
	for field := range c.Authorities {
		fields = append(fields, field)
	}
	sort.Strings(fields)
	for _, field := range fields {
		owner := c.Authorities[field]
		if commentFieldNames[field] {
			return fmt.Errorf("GOLC_CATALOG_COMMENT_EXCLUDED: %q: Linear comments and discussion are never stored in repository planning artifacts", field)
		}
		fixed, known := fieldOwnership[field]
		if !known {
			return fmt.Errorf("GOLC_CATALOG_FIELD_UNKNOWN: %q is not a registered mapped field", field)
		}
		if owner != fixed {
			if fixed == AuthorityRepository {
				return fmt.Errorf("GOLC_CATALOG_AUTHORITY_REPOSITORY_FIELD: %q is owned by repository artifacts and cannot be reassigned to %s", field, owner)
			}
			return fmt.Errorf("GOLC_CATALOG_AUTHORITY_LINEAR_FIELD: %q is a Linear operational field and cannot be reassigned to %s", field, owner)
		}
	}

	registered := make([]string, 0, len(fieldOwnership))
	for field := range fieldOwnership {
		registered = append(registered, field)
	}
	sort.Strings(registered)
	for _, field := range registered {
		if _, present := c.Authorities[field]; !present {
			return fmt.Errorf("GOLC_CATALOG_AUTHORITY_INCOMPLETE: registered field %q is missing from the authority registry", field)
		}
	}
	return nil
}
