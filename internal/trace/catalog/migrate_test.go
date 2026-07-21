// migrate_test.go covers the explicit, lossless schema-1-to-2 migration
// (CONTEXT D-11/D-12/D-14, threats T-01-24 through T-01-26): the stable
// project/milestone seed and any already-recorded remote mappings survive
// exactly, the dynamic catalog supplies the complete entity set, checking
// is read-only, writing is atomic and byte-idempotent, and nothing
// credential-bearing is ever invented or leaked.
//
// It shares the catalog_test external package and its fixture helpers
// (newFixtureRepository, repositoryRoot, writeFixtureFile) with
// catalog_test.go so it can declare its own quick-test scope through the
// command package's exact registration entrypoint.
package catalog_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lnorton89/golc/internal/command"
	"github.com/lnorton89/golc/internal/strictjson"
	"github.com/lnorton89/golc/internal/trace/catalog"
)

// The linear-map quick-test scope spans this package and
// internal/strictjson; both owned test files declare it identically
// (01-VALIDATION: every owning Go test task registers its scope through
// MustDeclareScope beside its TestScope marker).
var _ = command.MustDeclareScope(command.ScopeRegistration{
	Scope:   "linear-map",
	Summary: "Strict JSON guard and schema-1-to-2 linear map migration tests.",
})

// newMigrationFixtureRepository builds the standard dynamic-discovery
// fixture repository (plans 01, 02, 10; one checkpoint task; two
// requirements) and seeds its .planning/linear-map.json from the
// committed tests/fixtures/linear/map-schema1.json artifact, so migration
// tests exercise the exact same legacy seed shipped in the repository.
func newMigrationFixtureRepository(t *testing.T) string {
	t.Helper()
	root := newFixtureRepository(t)
	legacyPath := filepath.Join(repositoryRoot(t), "tests", "fixtures", "linear", "map-schema1.json")
	legacy, err := os.ReadFile(legacyPath)
	if err != nil {
		t.Fatalf("read %s: %v", legacyPath, err)
	}
	writeFixtureFile(t, filepath.Join(root, ".planning", "linear-map.json"), string(legacy))
	return root
}

func remoteMappingFor(m *catalog.Map, repoID string) (catalog.RemoteMapping, bool) {
	for _, mapping := range m.RemoteMappings {
		if mapping.RepoID == repoID {
			return mapping, true
		}
	}
	return catalog.RemoteMapping{}, false
}

// TestScopeLinearMap is the exact quick-test marker for scope "linear-map"
// (test --quick --scope linear-map).
func TestScopeLinearMap(t *testing.T) {
	t.Run("MigrateV1ToV2 preserves the seed identity and the existing remote mapping", func(t *testing.T) {
		root := newMigrationFixtureRepository(t)

		built, err := catalog.BuildCatalog(root)
		if err != nil {
			t.Fatalf("BuildCatalog: %v", err)
		}
		migrated, err := catalog.MigrateV1ToV2(root)
		if err != nil {
			t.Fatalf("MigrateV1ToV2: %v", err)
		}

		if migrated.Schema != 2 {
			t.Fatalf("Schema = %d, want 2", migrated.Schema)
		}
		if migrated.Repository.ProjectID != "project:golc" || migrated.Repository.Name != "GOLC" {
			t.Fatalf("repository = %+v, want project:golc/GOLC", migrated.Repository)
		}
		if migrated.ActiveMilestone.MilestoneID != "milestone:v1" || migrated.ActiveMilestone.Name != "GOLC v1" {
			t.Fatalf("active_milestone = %+v, want milestone:v1/GOLC v1", migrated.ActiveMilestone)
		}

		milestoneMapping, ok := remoteMappingFor(migrated, "milestone:v1")
		if !ok {
			t.Fatal("milestone:v1 remote mapping missing")
		}
		if milestoneMapping.LinearType != "project" || milestoneMapping.Status != "pending" {
			t.Fatalf("milestone:v1 mapping = %+v, want linear_type=project status=pending", milestoneMapping)
		}
		if milestoneMapping.LinearUUID != nil || milestoneMapping.Identifier != nil || milestoneMapping.URL != nil {
			t.Fatalf("milestone:v1 mapping carries a non-null remote identity: %+v", milestoneMapping)
		}

		if len(migrated.Entities) != len(built.Entities) {
			t.Fatalf("Entities has %d entries, catalog has %d", len(migrated.Entities), len(built.Entities))
		}
		for index, entity := range built.Entities {
			summary := migrated.Entities[index]
			if summary.LocalID != entity.ID || summary.Kind != string(entity.Kind) ||
				summary.ParentLocalID != entity.Parent || summary.Display != entity.Display || summary.Source != entity.Source {
				t.Fatalf("entity %d = %+v, want mirror of catalog entity %+v", index, summary, entity)
			}
		}

		// Every entity except the project root has exactly one remote
		// mapping, and every mapping refers to a real entity.
		wantMappings := len(built.Entities) - 1
		if len(migrated.RemoteMappings) != wantMappings {
			t.Fatalf("RemoteMappings has %d entries, want %d (entities minus the project root)", len(migrated.RemoteMappings), wantMappings)
		}
		seen := map[string]bool{}
		for _, mapping := range migrated.RemoteMappings {
			if seen[mapping.RepoID] {
				t.Fatalf("duplicate remote mapping for %s", mapping.RepoID)
			}
			seen[mapping.RepoID] = true
			if _, exists := built.Lookup(mapping.RepoID); !exists {
				t.Fatalf("remote mapping %s has no matching catalog entity", mapping.RepoID)
			}
		}
		if _, projectMapped := remoteMappingFor(migrated, "project:golc"); projectMapped {
			t.Fatal("the project root must never carry a remote mapping")
		}
	})

	t.Run("MigrateV1ToV2 assigns the Linear remote type per catalog kind", func(t *testing.T) {
		root := newMigrationFixtureRepository(t)
		migrated, err := catalog.MigrateV1ToV2(root)
		if err != nil {
			t.Fatalf("MigrateV1ToV2: %v", err)
		}

		wantTypes := map[string]string{
			"milestone:v1": "project",
			"phase:01":     "project_milestone",
			"req:TSTA-01":  "issue",
			"req:TSTB-02":  "issue",
			"plan:01-01":   "issue",
			"task:01-01.1": "issue",
			"task:01-01.3": "issue",
			"plan:01-02":   "issue",
			"task:01-02.1": "issue",
			"plan:01-10":   "issue",
			"task:01-10.1": "issue",
		}
		for repoID, wantType := range wantTypes {
			mapping, ok := remoteMappingFor(migrated, repoID)
			if !ok {
				t.Fatalf("remote mapping for %s missing", repoID)
			}
			if mapping.LinearType != wantType {
				t.Fatalf("%s linear_type = %q, want %q", repoID, mapping.LinearType, wantType)
			}
		}
	})

	t.Run("MigrateV1ToV2 rejects a legacy seed with duplicate or unknown JSON members", func(t *testing.T) {
		root := newMigrationFixtureRepository(t)

		writeFixtureFile(t, filepath.Join(root, ".planning", "linear-map.json"), `{
  "schema": 1,
  "schema": 1,
  "repository": { "project_id": "project:golc", "name": "GOLC" },
  "active_milestone": { "milestone_id": "milestone:v1", "name": "GOLC v1" },
  "remote_mappings": []
}
`)
		_, err := catalog.MigrateV1ToV2(root)
		requireErrorCode(t, err, "STRICTJSON_DUPLICATE_NAME")

		writeFixtureFile(t, filepath.Join(root, ".planning", "linear-map.json"), `{
  "schema": 1,
  "repository": { "project_id": "project:golc", "name": "GOLC" },
  "active_milestone": { "milestone_id": "milestone:v1", "name": "GOLC v1" },
  "remote_mappings": [],
  "linear_api_key": "should-never-be-here"
}
`)
		_, err = catalog.MigrateV1ToV2(root)
		requireErrorCode(t, err, "GOLC_MIGRATE_SEED_INVALID")
	})

	t.Run("Check is read-only and detects drift; Write is atomic and idempotent", func(t *testing.T) {
		root := newMigrationFixtureRepository(t)
		mapPath := filepath.Join(root, ".planning", "linear-map.json")

		before, err := os.ReadFile(mapPath)
		if err != nil {
			t.Fatalf("read before Check: %v", err)
		}
		if err := catalog.CheckMigration(root); err == nil {
			t.Fatal("CheckMigration on an unmigrated schema-1 seed unexpectedly reported no drift")
		} else if !strings.Contains(err.Error(), "GOLC_MIGRATE_DRIFT") {
			t.Fatalf("expected GOLC_MIGRATE_DRIFT, got: %v", err)
		}
		after, err := os.ReadFile(mapPath)
		if err != nil {
			t.Fatalf("read after Check: %v", err)
		}
		if string(before) != string(after) {
			t.Fatal("CheckMigration modified the file; it must be read-only")
		}

		if err := catalog.WriteMigration(root); err != nil {
			t.Fatalf("WriteMigration (first run): %v", err)
		}
		firstWrite, err := os.ReadFile(mapPath)
		if err != nil {
			t.Fatalf("read after first WriteMigration: %v", err)
		}
		if err := catalog.CheckMigration(root); err != nil {
			t.Fatalf("CheckMigration after WriteMigration: %v", err)
		}

		if err := catalog.WriteMigration(root); err != nil {
			t.Fatalf("WriteMigration (second run): %v", err)
		}
		secondWrite, err := os.ReadFile(mapPath)
		if err != nil {
			t.Fatalf("read after second WriteMigration: %v", err)
		}
		if string(firstWrite) != string(secondWrite) {
			t.Fatalf("WriteMigration is not byte-idempotent:\nfirst:\n%s\nsecond:\n%s", firstWrite, secondWrite)
		}

		entries, err := os.ReadDir(filepath.Join(root, ".planning"))
		if err != nil {
			t.Fatalf("read .planning dir: %v", err)
		}
		for _, entry := range entries {
			if strings.Contains(entry.Name(), ".tmp-") {
				t.Fatalf("temporary file %q leaked after atomic replacement", entry.Name())
			}
		}
	})

	t.Run("WriteMigration preserves an already-synced remote mapping on re-run", func(t *testing.T) {
		root := newMigrationFixtureRepository(t)
		if err := catalog.WriteMigration(root); err != nil {
			t.Fatalf("WriteMigration (first run): %v", err)
		}

		mapPath := filepath.Join(root, ".planning", "linear-map.json")
		data, err := os.ReadFile(mapPath)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		var current catalog.Map
		if err := strictjson.DecodeStrict(data, &current); err != nil {
			t.Fatalf("DecodeStrict: %v", err)
		}
		linked := false
		uuid := "11111111-1111-1111-1111-111111111111"
		identifier := "GOLC-42"
		url := "https://linear.app/example/issue/GOLC-42"
		for index := range current.RemoteMappings {
			if current.RemoteMappings[index].RepoID == "plan:01-01" {
				current.RemoteMappings[index].Status = "linked"
				current.RemoteMappings[index].LinearUUID = &uuid
				current.RemoteMappings[index].Identifier = &identifier
				current.RemoteMappings[index].URL = &url
				linked = true
			}
		}
		if !linked {
			t.Fatal("fixture is missing plan:01-01 remote mapping; test setup is broken")
		}
		encoded, err := strictjson.CanonicalEncode(&current)
		if err != nil {
			t.Fatalf("CanonicalEncode: %v", err)
		}
		writeFixtureFile(t, mapPath, string(encoded))

		if err := catalog.WriteMigration(root); err != nil {
			t.Fatalf("WriteMigration (second run, after simulated sync): %v", err)
		}
		reread, err := os.ReadFile(mapPath)
		if err != nil {
			t.Fatalf("read after second WriteMigration: %v", err)
		}
		var after catalog.Map
		if err := strictjson.DecodeStrict(reread, &after); err != nil {
			t.Fatalf("DecodeStrict after second WriteMigration: %v", err)
		}
		mapping, ok := remoteMappingFor(&after, "plan:01-01")
		if !ok {
			t.Fatal("plan:01-01 remote mapping missing after re-migration")
		}
		if mapping.Status != "linked" || mapping.LinearUUID == nil || *mapping.LinearUUID != uuid ||
			mapping.Identifier == nil || *mapping.Identifier != identifier || mapping.URL == nil || *mapping.URL != url {
			t.Fatalf("plan:01-01 mapping was not preserved across re-migration: %+v", mapping)
		}
		if err := catalog.CheckMigration(root); err != nil {
			t.Fatalf("CheckMigration after preserving a synced mapping: %v", err)
		}
	})

	t.Run("fixture migration output matches the committed golden byte-for-byte", func(t *testing.T) {
		root := newMigrationFixtureRepository(t)
		migrated, err := catalog.MigrateV1ToV2(root)
		if err != nil {
			t.Fatalf("MigrateV1ToV2: %v", err)
		}
		encoded, err := strictjson.CanonicalEncode(migrated)
		if err != nil {
			t.Fatalf("CanonicalEncode: %v", err)
		}
		goldenPath := filepath.Join(repositoryRoot(t), "tests", "golden", "linear-map-schema2.json")
		golden, err := os.ReadFile(goldenPath)
		if err != nil {
			t.Fatalf("read golden %s: %v", goldenPath, err)
		}
		if string(encoded) != string(golden) {
			t.Fatalf("migration output does not match the committed golden:\ngot:\n%s\nwant:\n%s", encoded, golden)
		}
	})

	t.Run("migration output never contains an unrelated credential canary", func(t *testing.T) {
		t.Setenv("GOLC_TEST_CREDENTIAL_CANARY", "gsd-fake-secret-9f3d7c21-do-not-leak")
		root := newMigrationFixtureRepository(t)
		migrated, err := catalog.MigrateV1ToV2(root)
		if err != nil {
			t.Fatalf("MigrateV1ToV2: %v", err)
		}
		encoded, err := strictjson.CanonicalEncode(migrated)
		if err != nil {
			t.Fatalf("CanonicalEncode: %v", err)
		}
		if strings.Contains(string(encoded), "gsd-fake-secret-9f3d7c21-do-not-leak") {
			t.Fatal("migration output leaked an unrelated environment value")
		}
	})

	t.Run("real repository seed migrates end to end offline", func(t *testing.T) {
		root := repositoryRoot(t)
		built, err := catalog.BuildCatalog(root)
		if err != nil {
			t.Fatalf("BuildCatalog: %v", err)
		}
		migrated, err := catalog.MigrateV1ToV2(root)
		if err != nil {
			t.Fatalf("MigrateV1ToV2: %v", err)
		}
		if migrated.Repository.ProjectID != "project:golc" {
			t.Fatalf("Repository.ProjectID = %q, want project:golc", migrated.Repository.ProjectID)
		}
		if migrated.ActiveMilestone.MilestoneID != "milestone:v1" {
			t.Fatalf("ActiveMilestone.MilestoneID = %q, want milestone:v1", migrated.ActiveMilestone.MilestoneID)
		}
		if len(migrated.Entities) != len(built.Entities) {
			t.Fatalf("Entities has %d entries, catalog has %d", len(migrated.Entities), len(built.Entities))
		}
		if len(migrated.RemoteMappings) != len(built.Entities)-1 {
			t.Fatalf("RemoteMappings has %d entries, want %d", len(migrated.RemoteMappings), len(built.Entities)-1)
		}
		if err := catalog.CheckMigration(root); err != nil {
			t.Fatalf("CheckMigration on the real repository: %v", err)
		}
	})
}
