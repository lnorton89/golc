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
	"strings"
	"testing"

	"github.com/lnorton89/golc/internal/command"
	"github.com/lnorton89/golc/internal/strictjson"
	"github.com/lnorton89/golc/internal/trace/catalog"
	"github.com/lnorton89/golc/internal/trace/reconcile"
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
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "golc.project.toml")); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("repository root with golc.project.toml not found above test directory")
		}
		dir = parent
	}
}

func requireErrorCode(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected an error containing %q, got nil", code)
	}
	if !strings.Contains(err.Error(), code) {
		t.Fatalf("error = %v, want it to contain %q", err, code)
	}
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
		if err != nil {
			t.Fatalf("BuildPlan (first): %v", err)
		}
		second, err := reconcile.BuildPlan(intents, mappings, scope, baselines)
		if err != nil {
			t.Fatalf("BuildPlan (second): %v", err)
		}

		firstEncoded, err := strictjson.CanonicalEncode(first)
		if err != nil {
			t.Fatalf("CanonicalEncode (first): %v", err)
		}
		secondEncoded, err := strictjson.CanonicalEncode(second)
		if err != nil {
			t.Fatalf("CanonicalEncode (second): %v", err)
		}
		if string(firstEncoded) != string(secondEncoded) {
			t.Fatalf("BuildPlan is not byte-stable:\nfirst:\n%s\nsecond:\n%s", firstEncoded, secondEncoded)
		}
		if first.PlanID != second.PlanID || first.PlanID == "" {
			t.Fatalf("PlanID = %q / %q, want equal and non-empty", first.PlanID, second.PlanID)
		}
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
		if err != nil {
			t.Fatalf("BuildPlan (forward): %v", err)
		}
		reversed, err := reconcile.BuildPlan(reversedIntents, reversedMappings, scope, baselines)
		if err != nil {
			t.Fatalf("BuildPlan (reversed): %v", err)
		}
		if forward.IntentDigest != reversed.IntentDigest {
			t.Fatalf("IntentDigest differs by input order: %q vs %q", forward.IntentDigest, reversed.IntentDigest)
		}
		if forward.MappingDigest != reversed.MappingDigest {
			t.Fatalf("MappingDigest differs by input order: %q vs %q", forward.MappingDigest, reversed.MappingDigest)
		}
		if forward.PlanID != reversed.PlanID {
			t.Fatalf("PlanID differs by input order: %q vs %q", forward.PlanID, reversed.PlanID)
		}
	})

	t.Run("operations follow the fixed hierarchy order with local-ID tie-break", func(t *testing.T) {
		intents, mappings, scope, baselines := previewFixture()
		plan, err := reconcile.BuildPlan(intents, mappings, scope, baselines)
		if err != nil {
			t.Fatalf("BuildPlan: %v", err)
		}
		want := []string{"milestone:v1", "phase:01", "plan:01-10", "req:CONF-01", "task:01-10.1"}
		if len(plan.Operations) != len(want) {
			t.Fatalf("Operations has %d entries, want %d", len(plan.Operations), len(want))
		}
		for index, op := range plan.Operations {
			if op.LocalID != want[index] {
				t.Fatalf("Operations[%d].LocalID = %q, want %q (full order: %v)", index, op.LocalID, want[index], operationOrder(plan.Operations))
			}
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
			if err != nil {
				t.Fatalf("RenderMarker(%q): %v", id, err)
			}
			description := "Managed by GOLC. Do not edit this footer.\n\n" + rendered
			marker, found, err := reconcile.ParseMarker(description)
			if err != nil {
				t.Fatalf("ParseMarker(%q): %v", id, err)
			}
			if !found {
				t.Fatalf("ParseMarker(%q): footer not found", id)
			}
			if marker.LocalID != id {
				t.Fatalf("marker.LocalID = %q, want %q", marker.LocalID, id)
			}
			if marker.Schema != reconcile.MarkerSchema {
				t.Fatalf("marker.Schema = %d, want %d", marker.Schema, reconcile.MarkerSchema)
			}
		}
	})

	t.Run("ParseMarker reports no footer and rejects ambiguous or malformed footers", func(t *testing.T) {
		_, found, err := reconcile.ParseMarker("A description with no footer at all.")
		if err != nil {
			t.Fatalf("ParseMarker (absent): %v", err)
		}
		if found {
			t.Fatal("ParseMarker (absent) unexpectedly found a footer")
		}

		one, err := reconcile.RenderMarker("plan:01-10")
		if err != nil {
			t.Fatalf("RenderMarker: %v", err)
		}
		two, err := reconcile.RenderMarker("task:01-10.1")
		if err != nil {
			t.Fatalf("RenderMarker: %v", err)
		}
		_, _, err = reconcile.ParseMarker(one + "\n" + two)
		requireErrorCode(t, err, "GOLC_RECONCILE_MARKER_AMBIGUOUS")

		_, _, err = reconcile.ParseMarker("---\nGOLC local ID: not-a-real-id\nGOLC mapping schema: 2\n")
		requireErrorCode(t, err, "GOLC_RECONCILE_MARKER_PARSE")
	})

	t.Run("ValidateMarkerIdentity accepts a matching marker and rejects kind/parent mismatches", func(t *testing.T) {
		taskOp := reconcile.Operation{LocalID: "task:01-10.1", Kind: "task", ParentLocalID: "plan:01-10"}

		matching, _, err := reconcile.ParseMarker(mustRender(t, "task:01-10.1"))
		if err != nil {
			t.Fatalf("ParseMarker: %v", err)
		}
		if err := reconcile.ValidateMarkerIdentity(matching, taskOp); err != nil {
			t.Fatalf("ValidateMarkerIdentity (matching): %v", err)
		}

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
		if err != nil {
			t.Fatalf("ParseMarker: %v", err)
		}
		if err := reconcile.ValidateMarkerIdentity(planMarker, planOp); err != nil {
			t.Fatalf("ValidateMarkerIdentity (plan, matching): %v", err)
		}
		wrongParentPlanOp := reconcile.Operation{LocalID: "plan:01-10", Kind: "plan", ParentLocalID: "phase:02"}
		err = reconcile.ValidateMarkerIdentity(planMarker, wrongParentPlanOp)
		requireErrorCode(t, err, "GOLC_RECONCILE_MARKER_PARENT")
	})

	t.Run("BuildPlan blocks a three-way disagreement as a conflict and excludes it from operations", func(t *testing.T) {
		intents, mappings, scope, baselines := conflictFixture()
		plan, err := reconcile.BuildPlan(intents, mappings, scope, baselines)
		if err != nil {
			t.Fatalf("BuildPlan: %v", err)
		}
		if len(plan.Conflicts) != 1 {
			t.Fatalf("Conflicts has %d entries, want 1: %+v", len(plan.Conflicts), plan.Conflicts)
		}
		conflict := plan.Conflicts[0]
		if conflict.LocalID != "req:CONF-01" || conflict.Field != "title" {
			t.Fatalf("conflict = %+v, want req:CONF-01/title", conflict)
		}
		if conflict.BaseValue == nil || *conflict.BaseValue != "Original title" {
			t.Fatalf("conflict.BaseValue = %v, want %q", conflict.BaseValue, "Original title")
		}
		if conflict.RepositoryValue == nil || *conflict.RepositoryValue != "Repository title override" {
			t.Fatalf("conflict.RepositoryValue = %v, want %q", conflict.RepositoryValue, "Repository title override")
		}
		if conflict.LinearValue == nil || *conflict.LinearValue != "Linear title override" {
			t.Fatalf("conflict.LinearValue = %v, want %q", conflict.LinearValue, "Linear title override")
		}
		if conflict.ResolutionCommand == "" {
			t.Fatal("conflict.ResolutionCommand is empty")
		}
		for _, op := range plan.Operations {
			if op.LocalID == "req:CONF-01" {
				t.Fatalf("req:CONF-01 has an operation despite being conflicted: %+v", op)
			}
		}
		want := []string{"phase:01", "plan:01-10"}
		if len(plan.Operations) != len(want) {
			t.Fatalf("Operations has %d entries, want %d: %v", len(plan.Operations), len(want), operationOrder(plan.Operations))
		}
		for index, op := range plan.Operations {
			if op.LocalID != want[index] {
				t.Fatalf("Operations[%d].LocalID = %q, want %q", index, op.LocalID, want[index])
			}
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
		if err != nil {
			t.Fatalf("BuildPlan: %v", err)
		}
		encoded, err := strictjson.CanonicalEncode(plan)
		if err != nil {
			t.Fatalf("CanonicalEncode: %v", err)
		}
		goldenPath := filepath.Join(repositoryRoot(t), "tests", "golden", "linear-preview.json")
		golden, err := os.ReadFile(goldenPath)
		if err != nil {
			t.Fatalf("read golden %s: %v", goldenPath, err)
		}
		if string(encoded) != string(golden) {
			t.Fatalf("preview output does not match the committed golden:\ngot:\n%s\nwant:\n%s", encoded, golden)
		}
	})

	t.Run("conflict fixture output matches the committed golden byte-for-byte", func(t *testing.T) {
		intents, mappings, scope, baselines := conflictFixture()
		plan, err := reconcile.BuildPlan(intents, mappings, scope, baselines)
		if err != nil {
			t.Fatalf("BuildPlan: %v", err)
		}
		encoded, err := strictjson.CanonicalEncode(plan)
		if err != nil {
			t.Fatalf("CanonicalEncode: %v", err)
		}
		goldenPath := filepath.Join(repositoryRoot(t), "tests", "golden", "linear-conflict-preview.json")
		golden, err := os.ReadFile(goldenPath)
		if err != nil {
			t.Fatalf("read golden %s: %v", goldenPath, err)
		}
		if string(encoded) != string(golden) {
			t.Fatalf("conflict preview output does not match the committed golden:\ngot:\n%s\nwant:\n%s", encoded, golden)
		}
	})

	t.Run("canonical plan output never contains an unrelated credential canary", func(t *testing.T) {
		t.Setenv("GOLC_TEST_CREDENTIAL_CANARY", "gsd-fake-secret-9f3d7c21-do-not-leak")
		intents, mappings, scope, baselines := previewFixture()
		plan, err := reconcile.BuildPlan(intents, mappings, scope, baselines)
		if err != nil {
			t.Fatalf("BuildPlan: %v", err)
		}
		encoded, err := strictjson.CanonicalEncode(plan)
		if err != nil {
			t.Fatalf("CanonicalEncode: %v", err)
		}
		if strings.Contains(string(encoded), "gsd-fake-secret-9f3d7c21-do-not-leak") {
			t.Fatal("canonical plan output leaked an unrelated environment value")
		}
	})
}

func mustRender(t *testing.T, id string) string {
	t.Helper()
	rendered, err := reconcile.RenderMarker(id)
	if err != nil {
		t.Fatalf("RenderMarker(%q): %v", id, err)
	}
	return rendered
}

func operationOrder(operations []reconcile.Operation) []string {
	ids := make([]string, 0, len(operations))
	for _, op := range operations {
		ids = append(ids, op.LocalID)
	}
	return ids
}
