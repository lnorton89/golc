// reconcile_test.go covers the D-17 exact preview contract (CONTEXT
// D-13/D-14/D-17/D-18): byte-stable canonical plans and plan IDs for
// identical inputs, the fixed hierarchy/tie-break operation order, the
// visible parser-stable local-ID identity footer round-tripping for every
// entity kind and rejecting kind/parent mismatches, and D-13 three-way
// conflict detection that blocks an operation instead of silently picking
// a side.
//
// It is an external test package so it can declare its quick-test scope
// through the command package's exact registration entrypoint (the
// config-local/linear-catalog/linear-map pattern from earlier plans).
package reconcile_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/command"
	"github.com/lnorton89/golc/internal/strictjson"
	"github.com/lnorton89/golc/internal/trace/catalog"
	"github.com/lnorton89/golc/internal/trace/reconcile"
	"github.com/lnorton89/golc/internal/trace/transport"
)

// The linear-preview-contract quick-test scope is declared through the
// exact production entrypoint (01-VALIDATION: every owning Go test task
// registers its scope through MustDeclareScope beside its TestScope
// marker).
var _ = command.MustDeclareScope(command.ScopeRegistration{
	Scope:   "linear-preview-contract",
	Summary: "Canonical reconciliation preview, plan hashing, ordering, and visible identity marker tests.",
})

// repositoryRoot walks upward from the test working directory to the real
// repository root (the directory owning golc.project.toml).
func repositoryRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err, "getwd")
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "golc.project.toml")); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			require.Fail(t, "repository root with golc.project.toml not found above test directory")
		}
		dir = parent
	}
}

func requireErrorCode(t *testing.T, err error, code string) {
	t.Helper()
	require.ErrorContains(t, err, code)
}

func strPtr(s string) *string { return &s }

// previewFixture builds a clean, conflict-free hierarchy spanning every
// remote-mapped catalog kind (milestone, phase, requirement, plan, task)
// so the operation ordering and golden preview both exercise the full
// D-17 hierarchy rank.
func previewFixture() ([]reconcile.Intent, []catalog.RemoteMapping, reconcile.RemoteScope, []reconcile.SyncBaseline) {
	intents := []reconcile.Intent{
		{
			LocalID: "milestone:v1", Kind: "milestone", LinearType: "project",
			ParentLocalID: "project:golc", Fields: map[string]string{"title": "GOLC v1"},
		},
		{
			LocalID: "phase:01", Kind: "phase", LinearType: "project_milestone",
			ParentLocalID: "milestone:v1", Fields: map[string]string{"title": "Offline Foundation and Delivery Traceability"},
		},
		{
			LocalID: "req:CONF-01", Kind: "req", LinearType: "issue",
			ParentLocalID: "phase:01", Fields: map[string]string{"title": "Centralize discoverable project configuration."},
		},
		{
			LocalID: "plan:01-10", Kind: "plan", LinearType: "issue",
			ParentLocalID: "phase:01", Fields: map[string]string{"title": "Plan 01-10"},
		},
		{
			LocalID: "task:01-10.1", Kind: "task", LinearType: "issue",
			ParentLocalID: "plan:01-10", Fields: map[string]string{"title": "Task 1: Define canonical operations and visible identity markers"},
		},
	}
	mappings := []catalog.RemoteMapping{
		{RepoID: "milestone:v1", LinearType: "project", Status: "pending"},
		{RepoID: "phase:01", LinearType: "project_milestone", Status: "pending"},
		{RepoID: "req:CONF-01", LinearType: "issue", Status: "pending"},
		{RepoID: "plan:01-10", LinearType: "issue", Status: "pending"},
		{RepoID: "task:01-10.1", LinearType: "issue", Status: "pending"},
	}
	return intents, mappings, reconcile.RemoteScope{}, nil
}

// conflictFixture builds two clean creates (phase:01, plan:01-10) plus one
// already-linked requirement whose title changed on both the repository
// and Linear sides away from the recorded baseline, so it must block as a
// D-13 conflict instead of producing an operation.
func conflictFixture() ([]reconcile.Intent, []catalog.RemoteMapping, reconcile.RemoteScope, []reconcile.SyncBaseline) {
	intents := []reconcile.Intent{
		{
			LocalID: "phase:01", Kind: "phase", LinearType: "project_milestone",
			ParentLocalID: "milestone:v1", Fields: map[string]string{"title": "Offline Foundation and Delivery Traceability"},
		},
		{
			LocalID: "plan:01-10", Kind: "plan", LinearType: "issue",
			ParentLocalID: "phase:01", Fields: map[string]string{"title": "Plan 01-10"},
		},
		{
			LocalID: "req:CONF-01", Kind: "req", LinearType: "issue",
			ParentLocalID: "phase:01", Fields: map[string]string{"title": "Repository title override"},
		},
	}
	mappings := []catalog.RemoteMapping{
		{RepoID: "phase:01", LinearType: "project_milestone", Status: "pending"},
		{RepoID: "plan:01-10", LinearType: "issue", Status: "pending"},
		{RepoID: "req:CONF-01", LinearType: "issue", Status: "linked", LinearUUID: strPtr("11111111-1111-1111-1111-111111111111")},
	}
	scope := reconcile.RemoteScope{
		Observations: []reconcile.RemoteObservation{
			{LocalID: "req:CONF-01", Fields: map[string]string{"title": "Linear title override"}, UpdatedAt: "2026-07-20T00:00:00Z"},
		},
	}
	baselines := []reconcile.SyncBaseline{
		{LocalID: "req:CONF-01", Fields: map[string]string{"title": "Original title"}},
	}
	return intents, mappings, scope, baselines
}

// TestScopeLinearPreviewContract is the exact quick-test marker for scope
// "linear-preview-contract" (test --quick --scope linear-preview-contract).
func TestScopeLinearPreviewContract(t *testing.T) {
	t.Run("BuildPlan is byte-stable for identical inputs", func(t *testing.T) {
		intents, mappings, scope, baselines := previewFixture()

		first, err := reconcile.BuildPlan(intents, mappings, scope, baselines)
		require.NoError(t, err, "BuildPlan (first)")
		second, err := reconcile.BuildPlan(intents, mappings, scope, baselines)
		require.NoError(t, err, "BuildPlan (second)")

		firstEncoded, err := strictjson.CanonicalEncode(first)
		require.NoError(t, err, "CanonicalEncode (first)")
		secondEncoded, err := strictjson.CanonicalEncode(second)
		require.NoError(t, err, "CanonicalEncode (second)")
		require.Equal(t, string(firstEncoded), string(secondEncoded), "BuildPlan is not byte-stable")
		require.Equal(t, first.PlanID, second.PlanID, "PlanID mismatch between first and second build")
		require.NotEmpty(t, first.PlanID, "PlanID must not be empty")
	})

	t.Run("digests are independent of input order", func(t *testing.T) {
		intents, mappings, scope, baselines := previewFixture()
		reversedIntents := append([]reconcile.Intent(nil), intents...)
		for i, j := 0, len(reversedIntents)-1; i < j; i, j = i+1, j-1 {
			reversedIntents[i], reversedIntents[j] = reversedIntents[j], reversedIntents[i]
		}
		reversedMappings := append([]catalog.RemoteMapping(nil), mappings...)
		for i, j := 0, len(reversedMappings)-1; i < j; i, j = i+1, j-1 {
			reversedMappings[i], reversedMappings[j] = reversedMappings[j], reversedMappings[i]
		}

		forward, err := reconcile.BuildPlan(intents, mappings, scope, baselines)
		require.NoError(t, err, "BuildPlan (forward)")
		reversed, err := reconcile.BuildPlan(reversedIntents, reversedMappings, scope, baselines)
		require.NoError(t, err, "BuildPlan (reversed)")
		require.Equal(t, forward.IntentDigest, reversed.IntentDigest, "IntentDigest differs by input order")
		require.Equal(t, forward.MappingDigest, reversed.MappingDigest, "MappingDigest differs by input order")
		require.Equal(t, forward.PlanID, reversed.PlanID, "PlanID differs by input order")
	})

	t.Run("operations follow the fixed hierarchy order with local-ID tie-break", func(t *testing.T) {
		intents, mappings, scope, baselines := previewFixture()
		plan, err := reconcile.BuildPlan(intents, mappings, scope, baselines)
		require.NoError(t, err, "BuildPlan")
		want := []string{"milestone:v1", "phase:01", "plan:01-10", "req:CONF-01", "task:01-10.1"}
		require.Len(t, plan.Operations, len(want))
		for index, op := range plan.Operations {
			require.Equal(t, want[index], op.LocalID, "Operations[%d].LocalID (full order: %v)", index, operationOrder(plan.Operations))
		}
	})

	t.Run("marker render/parse round-trips the exact local ID and schema for every entity kind", func(t *testing.T) {
		ids := []string{
			"project:golc",
			"milestone:v1",
			"phase:01",
			"req:CONF-01",
			"plan:01-10",
			"task:01-10.1",
		}
		for _, id := range ids {
			rendered, err := reconcile.RenderMarker(id)
			require.NoError(t, err, "RenderMarker(%q)", id)
			description := "Managed by GOLC. Do not edit this footer.\n\n" + rendered
			marker, found, err := reconcile.ParseMarker(description)
			require.NoError(t, err, "ParseMarker(%q)", id)
			require.True(t, found, "ParseMarker(%q): footer not found", id)
			require.Equal(t, id, marker.LocalID)
			require.Equal(t, reconcile.MarkerSchema, marker.Schema)
		}
	})

	t.Run("ParseMarker reports no footer and rejects ambiguous or malformed footers", func(t *testing.T) {
		_, found, err := reconcile.ParseMarker("A description with no footer at all.")
		require.NoError(t, err, "ParseMarker (absent)")
		require.False(t, found, "ParseMarker (absent) unexpectedly found a footer")

		one, err := reconcile.RenderMarker("plan:01-10")
		require.NoError(t, err, "RenderMarker")
		two, err := reconcile.RenderMarker("task:01-10.1")
		require.NoError(t, err, "RenderMarker")
		_, _, err = reconcile.ParseMarker(one + "\n" + two)
		requireErrorCode(t, err, "GOLC_RECONCILE_MARKER_AMBIGUOUS")

		_, _, err = reconcile.ParseMarker("---\nGOLC local ID: not-a-real-id\nGOLC mapping schema: 2\n")
		requireErrorCode(t, err, "GOLC_RECONCILE_MARKER_PARSE")
	})

	t.Run("ValidateMarkerIdentity accepts a matching marker and rejects kind/parent mismatches", func(t *testing.T) {
		taskOp := reconcile.Operation{LocalID: "task:01-10.1", Kind: "task", ParentLocalID: "plan:01-10"}

		matching, _, err := reconcile.ParseMarker(mustRender(t, "task:01-10.1"))
		require.NoError(t, err, "ParseMarker")
		require.NoError(t, reconcile.ValidateMarkerIdentity(matching, taskOp), "ValidateMarkerIdentity (matching)")

		wrongParentTaskOp := reconcile.Operation{LocalID: "task:01-10.1", Kind: "task", ParentLocalID: "plan:01-11"}
		err = reconcile.ValidateMarkerIdentity(matching, wrongParentTaskOp)
		requireErrorCode(t, err, "GOLC_RECONCILE_MARKER_PARENT")

		wrongKindOp := reconcile.Operation{LocalID: "task:01-10.1", Kind: "plan", ParentLocalID: "plan:01-10"}
		err = reconcile.ValidateMarkerIdentity(matching, wrongKindOp)
		requireErrorCode(t, err, "GOLC_RECONCILE_MARKER_KIND")

		mismatchedIDOp := reconcile.Operation{LocalID: "task:01-10.2", Kind: "task", ParentLocalID: "plan:01-10"}
		err = reconcile.ValidateMarkerIdentity(matching, mismatchedIDOp)
		requireErrorCode(t, err, "GOLC_RECONCILE_MARKER_IDENTITY")

		staleSchema := reconcile.Marker{LocalID: "task:01-10.1", Schema: 1}
		err = reconcile.ValidateMarkerIdentity(staleSchema, taskOp)
		requireErrorCode(t, err, "GOLC_RECONCILE_MARKER_SCHEMA")

		planOp := reconcile.Operation{LocalID: "plan:01-10", Kind: "plan", ParentLocalID: "phase:01"}
		planMarker, _, err := reconcile.ParseMarker(mustRender(t, "plan:01-10"))
		require.NoError(t, err, "ParseMarker")
		require.NoError(t, reconcile.ValidateMarkerIdentity(planMarker, planOp), "ValidateMarkerIdentity (plan, matching)")
		wrongParentPlanOp := reconcile.Operation{LocalID: "plan:01-10", Kind: "plan", ParentLocalID: "phase:02"}
		err = reconcile.ValidateMarkerIdentity(planMarker, wrongParentPlanOp)
		requireErrorCode(t, err, "GOLC_RECONCILE_MARKER_PARENT")
	})

	t.Run("BuildPlan blocks a three-way disagreement as a conflict and excludes it from operations", func(t *testing.T) {
		intents, mappings, scope, baselines := conflictFixture()
		plan, err := reconcile.BuildPlan(intents, mappings, scope, baselines)
		require.NoError(t, err, "BuildPlan")
		require.Len(t, plan.Conflicts, 1, "Conflicts: %+v", plan.Conflicts)
		conflict := plan.Conflicts[0]
		require.Equal(t, "req:CONF-01", conflict.LocalID)
		require.Equal(t, "title", conflict.Field)
		require.NotNil(t, conflict.BaseValue)
		require.Equal(t, "Original title", *conflict.BaseValue)
		require.NotNil(t, conflict.RepositoryValue)
		require.Equal(t, "Repository title override", *conflict.RepositoryValue)
		require.NotNil(t, conflict.LinearValue)
		require.Equal(t, "Linear title override", *conflict.LinearValue)
		require.NotEmpty(t, conflict.ResolutionCommand, "conflict.ResolutionCommand is empty")
		for _, op := range plan.Operations {
			require.NotEqual(t, "req:CONF-01", op.LocalID, "req:CONF-01 has an operation despite being conflicted: %+v", op)
		}
		want := []string{"phase:01", "plan:01-10"}
		require.Len(t, plan.Operations, len(want), "operations: %v", operationOrder(plan.Operations))
		for index, op := range plan.Operations {
			require.Equal(t, want[index], op.LocalID)
		}
	})

	t.Run("BuildPlan rejects an intent with no remote mapping", func(t *testing.T) {
		intents, mappings, scope, baselines := previewFixture()
		mappings = mappings[:len(mappings)-1] // drop task:01-10.1's mapping
		_, err := reconcile.BuildPlan(intents, mappings, scope, baselines)
		requireErrorCode(t, err, "GOLC_RECONCILE_MAPPING_MISSING")
	})

	t.Run("preview fixture output matches the committed golden byte-for-byte", func(t *testing.T) {
		intents, mappings, scope, baselines := previewFixture()
		plan, err := reconcile.BuildPlan(intents, mappings, scope, baselines)
		require.NoError(t, err, "BuildPlan")
		encoded, err := strictjson.CanonicalEncode(plan)
		require.NoError(t, err, "CanonicalEncode")
		goldenPath := filepath.Join(repositoryRoot(t), "tests", "golden", "linear-preview.json")
		golden, err := os.ReadFile(goldenPath)
		require.NoError(t, err, "read golden %s", goldenPath)
		require.Equal(t, string(golden), string(encoded), "preview output does not match the committed golden")
	})

	t.Run("conflict fixture output matches the committed golden byte-for-byte", func(t *testing.T) {
		intents, mappings, scope, baselines := conflictFixture()
		plan, err := reconcile.BuildPlan(intents, mappings, scope, baselines)
		require.NoError(t, err, "BuildPlan")
		encoded, err := strictjson.CanonicalEncode(plan)
		require.NoError(t, err, "CanonicalEncode")
		goldenPath := filepath.Join(repositoryRoot(t), "tests", "golden", "linear-conflict-preview.json")
		golden, err := os.ReadFile(goldenPath)
		require.NoError(t, err, "read golden %s", goldenPath)
		require.Equal(t, string(golden), string(encoded), "conflict preview output does not match the committed golden")
	})

	t.Run("canonical plan output never contains an unrelated credential canary", func(t *testing.T) {
		t.Setenv("GOLC_TEST_CREDENTIAL_CANARY", "gsd-fake-secret-9f3d7c21-do-not-leak")
		intents, mappings, scope, baselines := previewFixture()
		plan, err := reconcile.BuildPlan(intents, mappings, scope, baselines)
		require.NoError(t, err, "BuildPlan")
		encoded, err := strictjson.CanonicalEncode(plan)
		require.NoError(t, err, "CanonicalEncode")
		require.NotContains(t, string(encoded), "gsd-fake-secret-9f3d7c21-do-not-leak", "canonical plan output leaked an unrelated environment value")
	})
}

func mustRender(t *testing.T, id string) string {
	t.Helper()
	rendered, err := reconcile.RenderMarker(id)
	require.NoError(t, err, "RenderMarker(%q)", id)
	return rendered
}

func operationOrder(operations []reconcile.Operation) []string {
	ids := make([]string, 0, len(operations))
	for _, op := range operations {
		ids = append(ids, op.LocalID)
	}
	return ids
}

// The linear-reconcile quick-test scope covers the D-17 complete-snapshot
// preview path introduced in Plan 01-23 — ValidateCompleteSnapshot,
// ThreeWayField, marker-based discovery (zero/one/multiple), and
// BuildCompletePreview — plus the explicit D-15 archive/unlink review
// builders, declared beside the existing linear-preview-contract scope
// above.
var _ = command.MustDeclareScope(command.ScopeRegistration{
	Scope:   "linear-reconcile",
	Summary: "Complete-snapshot reconciliation preview, three-way field conflicts, marker discovery, and explicit archive/unlink review tests.",
})

// snapshotFixture is the self-contained JSON shape shared by the
// remote-complete/remote-conflict/remote-ambiguous fixtures: repository
// intent, the credential-free remote mapping set, the last-synchronized
// baseline, and the transport-neutral captured snapshot all live in one
// file, so each fixture is a complete, independently reviewable scenario
// that never depends on live transport.
type snapshotFixture struct {
	Description string                   `json:"description"`
	Intents     []reconcile.Intent       `json:"intents"`
	Mappings    []catalog.RemoteMapping  `json:"mappings"`
	Baselines   []reconcile.SyncBaseline `json:"baselines"`
	Snapshot    transport.Snapshot       `json:"snapshot"`
}

// archiveFixture is the self-contained JSON shape for
// explicit-archive.json: an already-linked managed entity's remote
// mapping, the only input BuildArchivePreview/BuildUnlinkPreview need.
type archiveFixture struct {
	Description string                `json:"description"`
	Mapping     catalog.RemoteMapping `json:"mapping"`
}

func loadSnapshotFixture(t *testing.T, name string) snapshotFixture {
	t.Helper()
	path := filepath.Join(repositoryRoot(t), "tests", "fixtures", "linear", name)
	data, err := os.ReadFile(path)
	require.NoError(t, err, "read fixture %s", path)
	var fixture snapshotFixture
	require.NoError(t, strictjson.DecodeStrict(data, &fixture), "decode fixture %s", path)
	return fixture
}

func loadArchiveFixture(t *testing.T, name string) archiveFixture {
	t.Helper()
	path := filepath.Join(repositoryRoot(t), "tests", "fixtures", "linear", name)
	data, err := os.ReadFile(path)
	require.NoError(t, err, "read fixture %s", path)
	var fixture archiveFixture
	require.NoError(t, strictjson.DecodeStrict(data, &fixture), "decode fixture %s", path)
	return fixture
}

// TestScopeLinearReconcile is the exact quick-test marker for scope
// "linear-reconcile" (test --quick --scope linear-reconcile).
func TestScopeLinearReconcile(t *testing.T) {
	t.Run("ValidateCompleteSnapshot blocks every non-complete status with a stable diagnostic", func(t *testing.T) {
		cases := []struct {
			status transport.SnapshotStatus
			code   string
		}{
			{transport.SnapshotIncomplete, "GOLC_RECONCILE_SNAPSHOT_INCOMPLETE"},
			{transport.SnapshotPartial, "GOLC_RECONCILE_SNAPSHOT_PARTIAL"},
			{transport.SnapshotCursorAnomaly, "GOLC_RECONCILE_SNAPSHOT_CURSOR_ANOMALY"},
			{transport.SnapshotAmbiguous, "GOLC_RECONCILE_SNAPSHOT_AMBIGUOUS"},
			{transport.SnapshotRateLimited, "GOLC_RECONCILE_SNAPSHOT_RATE_LIMITED"},
		}
		for _, tc := range cases {
			t.Run(string(tc.status), func(t *testing.T) {
				err := reconcile.ValidateCompleteSnapshot(transport.Snapshot{Status: tc.status, Reason: "synthetic diagnostic"})
				requireErrorCode(t, err, tc.code)
			})
		}
	})

	t.Run("ValidateCompleteSnapshot accepts a clean complete snapshot with no duplicate identity footers", func(t *testing.T) {
		fixture := loadSnapshotFixture(t, "remote-complete.json")
		require.NoError(t, reconcile.ValidateCompleteSnapshot(fixture.Snapshot), "ValidateCompleteSnapshot")
	})

	t.Run("ValidateCompleteSnapshot and BuildCompletePreview block a complete snapshot with a duplicated identity footer", func(t *testing.T) {
		fixture := loadSnapshotFixture(t, "remote-ambiguous.json")
		err := reconcile.ValidateCompleteSnapshot(fixture.Snapshot)
		requireErrorCode(t, err, "GOLC_RECONCILE_SNAPSHOT_AMBIGUOUS")

		_, err = reconcile.BuildCompletePreview(fixture.Intents, fixture.Mappings, fixture.Snapshot, fixture.Baselines)
		requireErrorCode(t, err, "GOLC_RECONCILE_SNAPSHOT_AMBIGUOUS")
	})

	t.Run("BuildCompletePreview adopts a marker-matched record and creates an unmatched intent", func(t *testing.T) {
		fixture := loadSnapshotFixture(t, "remote-complete.json")
		plan, err := reconcile.BuildCompletePreview(fixture.Intents, fixture.Mappings, fixture.Snapshot, fixture.Baselines)
		require.NoError(t, err, "BuildCompletePreview")
		require.Empty(t, plan.Conflicts, "Conflicts = %+v, want none", plan.Conflicts)
		want := []string{"plan:01-10", "task:01-10.1"}
		require.Len(t, plan.Operations, len(want), "operations: %v", operationOrder(plan.Operations))
		for index, op := range plan.Operations {
			require.Equal(t, want[index], op.LocalID)
		}
		adopted := plan.Operations[0]
		require.Equal(t, `{"title":"Plan 01-10"}`, string(adopted.Before), "adopted plan:01-10 Before, want the marker-matched record's fields")
		created := plan.Operations[1]
		require.Equal(t, `{}`, string(created.Before), "created task:01-10.1 Before, want an empty object (no discovered observation)")
	})

	t.Run("BuildCompletePreview blocks a three-way disagreement discovered through an already-linked UUID", func(t *testing.T) {
		fixture := loadSnapshotFixture(t, "remote-conflict.json")
		plan, err := reconcile.BuildCompletePreview(fixture.Intents, fixture.Mappings, fixture.Snapshot, fixture.Baselines)
		require.NoError(t, err, "BuildCompletePreview")
		require.Empty(t, plan.Operations, "Operations = %+v, want none (blocked)", plan.Operations)
		require.Len(t, plan.Conflicts, 1, "Conflicts: %+v", plan.Conflicts)
		conflict := plan.Conflicts[0]
		require.Equal(t, "req:CONF-01", conflict.LocalID)
		require.Equal(t, "title", conflict.Field)
		require.NotNil(t, conflict.BaseValue)
		require.Equal(t, "Original title", *conflict.BaseValue)
		require.NotNil(t, conflict.RepositoryValue)
		require.Equal(t, "Repository title override", *conflict.RepositoryValue)
		require.NotNil(t, conflict.LinearValue)
		require.Equal(t, "Linear title override", *conflict.LinearValue)
	})

	t.Run("ThreeWayField blocks only when base, repository, and Linear are pairwise distinct", func(t *testing.T) {
		require.Nil(t, reconcile.ThreeWayField("plan:01-10", "title", "A", "A", "B"), "base==repo")
		require.Nil(t, reconcile.ThreeWayField("plan:01-10", "title", "A", "B", "A"), "base==linear")
		require.Nil(t, reconcile.ThreeWayField("plan:01-10", "title", "A", "B", "B"), "repo==linear")
		got := reconcile.ThreeWayField("plan:01-10", "title", "A", "B", "C")
		require.NotNil(t, got, "all three distinct: ThreeWayField = nil, want a blocking Conflict")
		require.Equal(t, "plan:01-10", got.LocalID)
		require.Equal(t, "title", got.Field)
		require.NotNil(t, got.BaseValue)
		require.Equal(t, "A", *got.BaseValue)
		require.NotNil(t, got.RepositoryValue)
		require.Equal(t, "B", *got.RepositoryValue)
		require.NotNil(t, got.LinearValue)
		require.Equal(t, "C", *got.LinearValue)
		require.NotEmpty(t, got.ResolutionCommand, "ResolutionCommand is empty")
	})

	t.Run("BuildArchivePreview and BuildUnlinkPreview build an explicit D-15 removal preview, and reject an unmapped entity", func(t *testing.T) {
		fixture := loadArchiveFixture(t, "explicit-archive.json")

		archived, err := reconcile.BuildArchivePreview(fixture.Mapping)
		require.NoError(t, err, "BuildArchivePreview")
		require.Equal(t, "archive", archived.Action)
		require.Equal(t, fixture.Mapping.RepoID, archived.LocalID)
		require.NotNil(t, archived.LinearUUID)
		require.NotNil(t, fixture.Mapping.LinearUUID)
		require.Equal(t, *fixture.Mapping.LinearUUID, *archived.LinearUUID)

		unlinked, err := reconcile.BuildUnlinkPreview(fixture.Mapping)
		require.NoError(t, err, "BuildUnlinkPreview")
		require.Equal(t, "unlink", unlinked.Action)
		require.Equal(t, fixture.Mapping.RepoID, unlinked.LocalID)

		pending := catalog.RemoteMapping{RepoID: "task:01-99.1", LinearType: "issue", Status: "pending"}
		_, err = reconcile.BuildArchivePreview(pending)
		requireErrorCode(t, err, "GOLC_RECONCILE_ARCHIVE_UNMAPPED")
		_, err = reconcile.BuildUnlinkPreview(pending)
		requireErrorCode(t, err, "GOLC_RECONCILE_ARCHIVE_UNMAPPED")
	})

	t.Run("the complete preview is reachable end to end through a credential-free Fake transport", func(t *testing.T) {
		fixture := loadSnapshotFixture(t, "remote-complete.json")
		fake := transport.NewFake(fixture.Snapshot)

		captured, err := fake.CaptureSnapshot()
		require.NoError(t, err, "Fake.CaptureSnapshot")
		plan, err := reconcile.BuildCompletePreview(fixture.Intents, fixture.Mappings, captured, fixture.Baselines)
		require.NoError(t, err, "BuildCompletePreview via fake transport")
		require.Len(t, plan.Operations, 2, "plan via fake transport = %+v, want 2 operations and no conflicts", plan)
		require.Empty(t, plan.Conflicts, "plan via fake transport = %+v, want 2 operations and no conflicts", plan)

		applied, err := fake.Apply(transport.Mutation{Kind: transport.MutationArchive, LocalID: "task:01-23.9", LinearUUID: "22222222-2222-2222-2222-222222222222"})
		require.NoError(t, err, "Fake.Apply")
		require.Equal(t, transport.MutationArchive, applied.Kind)
		require.Len(t, fake.Applied(), 1, "Fake.Applied() = %+v, want exactly one recorded archive mutation", fake.Applied())
	})
}
