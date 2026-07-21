// migrate.go implements the explicit, lossless schema-1-to-2 migration of
// .planning/linear-map.json (CONTEXT D-11/D-12/D-14, threats T-01-24
// through T-01-26). The stable project/milestone seed and any
// already-recorded remote mappings are preserved exactly; the complete
// local catalog dynamically discovered by BuildCatalog supplies every
// entity plus a pending/null remote mapping for anything not already
// mapped. Check is read-only; Write validates the complete target and
// replaces it atomically. No remote UUID, identifier, URL, or credential
// is ever invented.
package catalog

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lnorton89/golc/internal/strictjson"
)

// linearMapFileName is the fixed repository-relative map file name under
// .planning/.
const linearMapFileName = "linear-map.json"

// EntitySummary is one durable local catalog entity as recorded in the
// schema-2 map: identity, structural parent, display text, and source
// artifact, exactly mirroring catalog.Entity in a JSON-stable shape.
type EntitySummary struct {
	LocalID       string `json:"local_id"`
	Kind          string `json:"kind"`
	ParentLocalID string `json:"parent_local_id"`
	Display       string `json:"display"`
	Source        string `json:"source"`
}

// RemoteMapping is one optional, credential-free link from a durable
// local ID to a Linear remote object. Nullable fields stay nil until an
// explicit, separate synchronization records a real Linear identity;
// nothing here is invented (CONTEXT D-11/D-14).
type RemoteMapping struct {
	RepoID     string  `json:"repo_id"`
	LinearType string  `json:"linear_type"`
	Status     string  `json:"status"`
	LinearUUID *string `json:"linear_uuid"`
	Identifier *string `json:"identifier"`
	URL        *string `json:"url"`
}

// Map is the schema-1/schema-2-compatible shape of .planning/linear-map.json.
// Schema 1 documents populate only Schema, Repository, ActiveMilestone, and
// RemoteMappings; schema 2 documents additionally populate Entities with
// the complete dynamically discovered local catalog.
type Map struct {
	Schema     int `json:"schema"`
	Repository struct {
		ProjectID string `json:"project_id"`
		Name      string `json:"name"`
	} `json:"repository"`
	ActiveMilestone struct {
		MilestoneID string `json:"milestone_id"`
		Name        string `json:"name"`
	} `json:"active_milestone"`
	Entities       []EntitySummary `json:"entities"`
	RemoteMappings []RemoteMapping `json:"remote_mappings"`
}

// linearTypeForKind derives the Linear remote object type an entity kind
// maps to (CONTEXT research/STACK.md "Linear from Day One" hierarchy):
// release/milestone -> Project, phase -> Project Milestone, and
// requirement/plan/task -> Issue. The project kind itself is never
// remote-mapped; it is the local repository root.
func linearTypeForKind(kind Kind) (string, error) {
	switch kind {
	case KindMilestone:
		return "project", nil
	case KindPhase:
		return "project_milestone", nil
	case KindRequirement, KindPlan, KindTask:
		return "issue", nil
	default:
		return "", fmt.Errorf("GOLC_MIGRATE_KIND_UNMAPPED: %q has no Linear remote type", kind)
	}
}

// lookupKind returns the first catalog entity of the given kind. Project
// and milestone are root singletons, so the first match is the only match.
func lookupKind(c *Catalog, kind Kind) (Entity, bool) {
	for _, entity := range c.Entities {
		if entity.Kind == kind {
			return entity, true
		}
	}
	return Entity{}, false
}

// readCurrentMap strictly decodes the current .planning/linear-map.json,
// accepting either schema 1 or schema 2 (both share the same
// repository/active_milestone/remote_mappings shape).
func readCurrentMap(root string) (*Map, error) {
	path := filepath.Join(root, ".planning", linearMapFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("GOLC_MIGRATE_SEED_INVALID: %s: %v", path, err)
	}
	current := &Map{}
	if err := strictjson.DecodeStrict(data, current); err != nil {
		return nil, fmt.Errorf("GOLC_MIGRATE_SEED_INVALID: %s: %v", path, err)
	}
	if current.Schema != 1 && current.Schema != 2 {
		return nil, fmt.Errorf("GOLC_MIGRATE_SEED_INVALID: %s declares unsupported schema %d", path, current.Schema)
	}
	return current, nil
}

// MigrateV1ToV2 builds the canonical schema-2 identity map for root. The
// project/milestone seed and any already-recorded remote mappings are
// carried through exactly from the current .planning/linear-map.json
// (CONTEXT D-14); the dynamically discovered local catalog supplies every
// entity, and any entity without an existing mapping receives a fresh
// pending/null one. Check is read-only by construction: this function
// never writes anything.
func MigrateV1ToV2(root string) (*Map, error) {
	current, err := readCurrentMap(root)
	if err != nil {
		return nil, err
	}

	built, err := BuildCatalog(root)
	if err != nil {
		return nil, err
	}

	projectEntity, ok := lookupKind(built, KindProject)
	if !ok {
		return nil, fmt.Errorf("GOLC_MIGRATE_SEED_MISMATCH: catalog has no project entity")
	}
	if projectEntity.ID != current.Repository.ProjectID {
		return nil, fmt.Errorf("GOLC_MIGRATE_SEED_MISMATCH: catalog project %q does not match seed %q",
			projectEntity.ID, current.Repository.ProjectID)
	}
	milestoneEntity, ok := lookupKind(built, KindMilestone)
	if !ok {
		return nil, fmt.Errorf("GOLC_MIGRATE_SEED_MISMATCH: catalog has no milestone entity")
	}
	if milestoneEntity.ID != current.ActiveMilestone.MilestoneID {
		return nil, fmt.Errorf("GOLC_MIGRATE_SEED_MISMATCH: catalog milestone %q does not match seed %q",
			milestoneEntity.ID, current.ActiveMilestone.MilestoneID)
	}

	existing := make(map[string]RemoteMapping, len(current.RemoteMappings))
	for _, mapping := range current.RemoteMappings {
		if _, duplicate := existing[mapping.RepoID]; duplicate {
			return nil, fmt.Errorf("GOLC_MIGRATE_SEED_INVALID: duplicate remote mapping for %q", mapping.RepoID)
		}
		existing[mapping.RepoID] = mapping
	}

	migrated := &Map{Schema: 2}
	migrated.Repository.ProjectID = current.Repository.ProjectID
	migrated.Repository.Name = current.Repository.Name
	migrated.ActiveMilestone.MilestoneID = current.ActiveMilestone.MilestoneID
	migrated.ActiveMilestone.Name = current.ActiveMilestone.Name

	migrated.Entities = make([]EntitySummary, 0, len(built.Entities))
	migrated.RemoteMappings = make([]RemoteMapping, 0, len(built.Entities))
	for _, entity := range built.Entities {
		migrated.Entities = append(migrated.Entities, EntitySummary{
			LocalID:       entity.ID,
			Kind:          string(entity.Kind),
			ParentLocalID: entity.Parent,
			Display:       entity.Display,
			Source:        entity.Source,
		})
		if entity.Kind == KindProject {
			// The repository root is never remote-mapped.
			continue
		}
		if preserved, alreadyMapped := existing[entity.ID]; alreadyMapped {
			migrated.RemoteMappings = append(migrated.RemoteMappings, preserved)
			continue
		}
		linearType, err := linearTypeForKind(entity.Kind)
		if err != nil {
			return nil, err
		}
		migrated.RemoteMappings = append(migrated.RemoteMappings, RemoteMapping{
			RepoID:     entity.ID,
			LinearType: linearType,
			Status:     "pending",
		})
	}

	if err := validateMap(migrated); err != nil {
		return nil, err
	}
	return migrated, nil
}

// validateMap enforces completeness and the credential-free invariant on
// a fully assembled schema-2 map before it is ever written: every
// non-project entity has exactly one remote mapping, every remote mapping
// refers to a real entity, the project entity is never mapped, and a
// pending mapping never carries a remote identity.
func validateMap(m *Map) error {
	if m.Schema != 2 {
		return fmt.Errorf("GOLC_MIGRATE_TARGET_INVALID: schema must be 2, got %d", m.Schema)
	}
	if _, err := ParseID(m.Repository.ProjectID); err != nil {
		return fmt.Errorf("GOLC_MIGRATE_TARGET_INVALID: repository.project_id: %v", err)
	}
	if _, err := ParseID(m.ActiveMilestone.MilestoneID); err != nil {
		return fmt.Errorf("GOLC_MIGRATE_TARGET_INVALID: active_milestone.milestone_id: %v", err)
	}

	entityByID := make(map[string]EntitySummary, len(m.Entities))
	for _, entity := range m.Entities {
		if _, duplicate := entityByID[entity.LocalID]; duplicate {
			return fmt.Errorf("GOLC_MIGRATE_TARGET_INVALID: duplicate entity %q", entity.LocalID)
		}
		entityByID[entity.LocalID] = entity
	}

	mapped := make(map[string]bool, len(m.RemoteMappings))
	for _, mapping := range m.RemoteMappings {
		entity, known := entityByID[mapping.RepoID]
		if !known {
			return fmt.Errorf("GOLC_MIGRATE_MAPPING_ORPHAN: remote mapping %q has no matching catalog entity", mapping.RepoID)
		}
		if entity.Kind == string(KindProject) {
			return fmt.Errorf("GOLC_MIGRATE_TARGET_INVALID: the project entity %q must not carry a remote mapping", mapping.RepoID)
		}
		if mapped[mapping.RepoID] {
			return fmt.Errorf("GOLC_MIGRATE_TARGET_INVALID: duplicate remote mapping for %q", mapping.RepoID)
		}
		mapped[mapping.RepoID] = true
		if mapping.Status == "pending" && (mapping.LinearUUID != nil || mapping.Identifier != nil || mapping.URL != nil) {
			return fmt.Errorf("GOLC_MIGRATE_TARGET_INVALID: pending mapping %q must not carry a remote identity", mapping.RepoID)
		}
	}
	for _, entity := range m.Entities {
		if entity.Kind == string(KindProject) {
			continue
		}
		if !mapped[entity.LocalID] {
			return fmt.Errorf("GOLC_MIGRATE_TARGET_INCOMPLETE: entity %q has no remote mapping", entity.LocalID)
		}
	}
	return nil
}

// CheckMigration re-derives the canonical schema-2 map for root and
// compares it byte-for-byte against the committed .planning/linear-map.json.
// It performs no writes.
func CheckMigration(root string) error {
	migrated, err := MigrateV1ToV2(root)
	if err != nil {
		return err
	}
	want, err := strictjson.CanonicalEncode(migrated)
	if err != nil {
		return fmt.Errorf("GOLC_MIGRATE_ENCODE: %v", err)
	}
	path := filepath.Join(root, ".planning", linearMapFileName)
	have, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("GOLC_MIGRATE_DRIFT: %s: %v", path, err)
	}
	if !bytes.Equal(want, have) {
		return fmt.Errorf("GOLC_MIGRATE_DRIFT: %s does not match the canonical schema-2 migration output", path)
	}
	return nil
}

// WriteMigration derives the canonical schema-2 map for root, validates
// it, and atomically replaces .planning/linear-map.json through a
// contained temporary file plus rename. Running it twice in a row without
// any repository change produces byte-identical output.
func WriteMigration(root string) error {
	migrated, err := MigrateV1ToV2(root)
	if err != nil {
		return err
	}
	payload, err := strictjson.CanonicalEncode(migrated)
	if err != nil {
		return fmt.Errorf("GOLC_MIGRATE_ENCODE: %v", err)
	}

	destination := filepath.Join(root, ".planning", linearMapFileName)
	temporary, err := os.CreateTemp(filepath.Dir(destination), linearMapFileName+".tmp-*")
	if err != nil {
		return fmt.Errorf("GOLC_MIGRATE_WRITE: staging %s: %v", linearMapFileName, err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	if _, err := temporary.Write(payload); err != nil {
		temporary.Close()
		return fmt.Errorf("GOLC_MIGRATE_WRITE: staging %s: %v", linearMapFileName, err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("GOLC_MIGRATE_WRITE: staging %s: %v", linearMapFileName, err)
	}
	if err := os.Rename(temporaryPath, destination); err != nil {
		return fmt.Errorf("GOLC_MIGRATE_WRITE: atomic replacement of %s: %v", linearMapFileName, err)
	}
	return nil
}
