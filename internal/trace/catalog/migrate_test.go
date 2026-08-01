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
	"testing"

	"github.com/stretchr/testify/require"

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
	require.NoError(t, err, "read %s", legacyPath)
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
		require.NoError(t, err, "BuildCatalog")
		migrated, err := catalog.MigrateV1ToV2(root)
		require.NoError(t, err, "MigrateV1ToV2")

		require.Equal(t, 2, migrated.Schema)
		require.Equal(t, "project:golc", migrated.Repository.ProjectID)
		require.Equal(t, "GOLC", migrated.Repository.Name)
		require.Equal(t, "milestone:v1", migrated.ActiveMilestone.MilestoneID)
		require.Equal(t, "GOLC v1", migrated.ActiveMilestone.Name)

		milestoneMapping, ok := remoteMappingFor(migrated, "milestone:v1")
		require.True(t, ok, "milestone:v1 remote mapping missing")
		require.Equal(t, "project", milestoneMapping.LinearType)
		require.Equal(t, "pending", milestoneMapping.Status)
		require.Nil(t, milestoneMapping.LinearUUID, "milestone:v1 mapping carries a non-null remote identity: %+v", milestoneMapping)
		require.Nil(t, milestoneMapping.Identifier, "milestone:v1 mapping carries a non-null remote identity: %+v", milestoneMapping)
		require.Nil(t, milestoneMapping.URL, "milestone:v1 mapping carries a non-null remote identity: %+v", milestoneMapping)

		require.Len(t, migrated.Entities, len(built.Entities))
		for index, entity := range built.Entities {
			summary := migrated.Entities[index]
			require.Equal(t, entity.ID, summary.LocalID, "entity %d, want mirror of catalog entity %+v", index, entity)
			require.Equal(t, string(entity.Kind), summary.Kind, "entity %d, want mirror of catalog entity %+v", index, entity)
			require.Equal(t, entity.Parent, summary.ParentLocalID, "entity %d, want mirror of catalog entity %+v", index, entity)
			require.Equal(t, entity.Display, summary.Display, "entity %d, want mirror of catalog entity %+v", index, entity)
			require.Equal(t, entity.Source, summary.Source, "entity %d, want mirror of catalog entity %+v", index, entity)
		}

		// Every entity except the project root has exactly one remote
		// mapping, and every mapping refers to a real entity.
		wantMappings := len(built.Entities) - 1
		require.Len(t, migrated.RemoteMappings, wantMappings, "want entities minus the project root")
		seen := map[string]bool{}
		for _, mapping := range migrated.RemoteMappings {
			require.False(t, seen[mapping.RepoID], "duplicate remote mapping for %s", mapping.RepoID)
			seen[mapping.RepoID] = true
			_, exists := built.Lookup(mapping.RepoID)
			require.True(t, exists, "remote mapping %s has no matching catalog entity", mapping.RepoID)
		}
		_, projectMapped := remoteMappingFor(migrated, "project:golc")
		require.False(t, projectMapped, "the project root must never carry a remote mapping")
	})

	t.Run("MigrateV1ToV2 assigns the Linear remote type per catalog kind", func(t *testing.T) {
		root := newMigrationFixtureRepository(t)
		migrated, err := catalog.MigrateV1ToV2(root)
		require.NoError(t, err, "MigrateV1ToV2")

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
			require.True(t, ok, "remote mapping for %s missing", repoID)
			require.Equal(t, wantType, mapping.LinearType, "%s linear_type", repoID)
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
		require.NoError(t, err, "read before Check")
		checkErr := catalog.CheckMigration(root)
		require.Error(t, checkErr, "CheckMigration on an unmigrated schema-1 seed unexpectedly reported no drift")
		require.Contains(t, checkErr.Error(), "GOLC_MIGRATE_DRIFT")
		after, err := os.ReadFile(mapPath)
		require.NoError(t, err, "read after Check")
		require.Equal(t, string(before), string(after), "CheckMigration modified the file; it must be read-only")

		require.NoError(t, catalog.WriteMigration(root), "WriteMigration (first run)")
		firstWrite, err := os.ReadFile(mapPath)
		require.NoError(t, err, "read after first WriteMigration")
		require.NoError(t, catalog.CheckMigration(root), "CheckMigration after WriteMigration")

		require.NoError(t, catalog.WriteMigration(root), "WriteMigration (second run)")
		secondWrite, err := os.ReadFile(mapPath)
		require.NoError(t, err, "read after second WriteMigration")
		require.Equal(t, string(firstWrite), string(secondWrite), "WriteMigration is not byte-idempotent")

		entries, err := os.ReadDir(filepath.Join(root, ".planning"))
		require.NoError(t, err, "read .planning dir")
		for _, entry := range entries {
			require.NotContains(t, entry.Name(), ".tmp-", "temporary file leaked after atomic replacement")
		}
	})

	t.Run("WriteMigration preserves an already-synced remote mapping on re-run", func(t *testing.T) {
		root := newMigrationFixtureRepository(t)
		require.NoError(t, catalog.WriteMigration(root), "WriteMigration (first run)")

		mapPath := filepath.Join(root, ".planning", "linear-map.json")
		data, err := os.ReadFile(mapPath)
		require.NoError(t, err, "read")
		var current catalog.Map
		require.NoError(t, strictjson.DecodeStrict(data, &current), "DecodeStrict")
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
		require.True(t, linked, "fixture is missing plan:01-01 remote mapping; test setup is broken")
		encoded, err := strictjson.CanonicalEncode(&current)
		require.NoError(t, err, "CanonicalEncode")
		writeFixtureFile(t, mapPath, string(encoded))

		require.NoError(t, catalog.WriteMigration(root), "WriteMigration (second run, after simulated sync)")
		reread, err := os.ReadFile(mapPath)
		require.NoError(t, err, "read after second WriteMigration")
		var after catalog.Map
		require.NoError(t, strictjson.DecodeStrict(reread, &after), "DecodeStrict after second WriteMigration")
		mapping, ok := remoteMappingFor(&after, "plan:01-01")
		require.True(t, ok, "plan:01-01 remote mapping missing after re-migration")
		require.Equal(t, "linked", mapping.Status, "plan:01-01 mapping was not preserved across re-migration: %+v", mapping)
		require.NotNil(t, mapping.LinearUUID, "plan:01-01 mapping was not preserved across re-migration: %+v", mapping)
		require.Equal(t, uuid, *mapping.LinearUUID)
		require.NotNil(t, mapping.Identifier, "plan:01-01 mapping was not preserved across re-migration: %+v", mapping)
		require.Equal(t, identifier, *mapping.Identifier)
		require.NotNil(t, mapping.URL, "plan:01-01 mapping was not preserved across re-migration: %+v", mapping)
		require.Equal(t, url, *mapping.URL)
		require.NoError(t, catalog.CheckMigration(root), "CheckMigration after preserving a synced mapping")
	})

	t.Run("fixture migration output matches the committed golden byte-for-byte", func(t *testing.T) {
		root := newMigrationFixtureRepository(t)
		migrated, err := catalog.MigrateV1ToV2(root)
		require.NoError(t, err, "MigrateV1ToV2")
		encoded, err := strictjson.CanonicalEncode(migrated)
		require.NoError(t, err, "CanonicalEncode")
		goldenPath := filepath.Join(repositoryRoot(t), "tests", "golden", "linear-map-schema2.json")
		golden, err := os.ReadFile(goldenPath)
		require.NoError(t, err, "read golden %s", goldenPath)
		require.Equal(t, string(golden), string(encoded), "migration output does not match the committed golden")
	})

	t.Run("migration output never contains an unrelated credential canary", func(t *testing.T) {
		t.Setenv("GOLC_TEST_CREDENTIAL_CANARY", "gsd-fake-secret-9f3d7c21-do-not-leak")
		root := newMigrationFixtureRepository(t)
		migrated, err := catalog.MigrateV1ToV2(root)
		require.NoError(t, err, "MigrateV1ToV2")
		encoded, err := strictjson.CanonicalEncode(migrated)
		require.NoError(t, err, "CanonicalEncode")
		require.NotContains(t, string(encoded), "gsd-fake-secret-9f3d7c21-do-not-leak", "migration output leaked an unrelated environment value")
	})

	t.Run("real repository seed migrates end to end offline", func(t *testing.T) {
		root := repositoryRoot(t)
		built, err := catalog.BuildCatalog(root)
		require.NoError(t, err, "BuildCatalog")
		migrated, err := catalog.MigrateV1ToV2(root)
		require.NoError(t, err, "MigrateV1ToV2")
		require.Equal(t, "project:golc", migrated.Repository.ProjectID)
		require.Equal(t, "milestone:v1", migrated.ActiveMilestone.MilestoneID)
		require.Len(t, migrated.Entities, len(built.Entities))
		require.Len(t, migrated.RemoteMappings, len(built.Entities)-1)
		require.NoError(t, catalog.CheckMigration(root), "CheckMigration on the real repository")
	})
}
