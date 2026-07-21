// linear_plan_test.go proves the strict "linear-plan"/"linear-report"
// contract (CONTEXT D-08, D-13, D-14, D-17, D-18, D-21): both descriptors
// are registered exactly once and reachable through the unchanged global
// generator, and the "linear apply {plan-file} --plan-id <id>" route
// self-registered by internal/command/linear.go rejects every malformed
// or illegal plan state -- duplicate JSON member names, unknown fields, a
// tampered plan_id, an out-of-canonical-order operation list, a
// structurally malformed D-13 conflict, and a local id that is both
// planned and unresolved-conflicted -- before any typed value ever reaches
// apply.Apply, and fails LINEAR_TRANSPORT_UNAVAILABLE before any
// credential, subprocess, or mutation access when no RemoteClientFactory
// is wired.
//
// It is an external test package (like generate_test.go) so it can
// declare its quick-test scope through the command package's exact
// registration entrypoint, and can exercise the full self-registered
// "linear apply" route end to end, without an import cycle.
package contracts_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lnorton89/golc/internal/command"
	"github.com/lnorton89/golc/internal/contracts"
	"github.com/lnorton89/golc/internal/strictjson"
	"github.com/lnorton89/golc/internal/trace/catalog"
	"github.com/lnorton89/golc/internal/trace/reconcile"
)

// The linear-plan-contract quick-test scope is declared through the exact
// production entrypoint (01-VALIDATION: every owning Go test file
// registers its scope beside its TestScope marker; duplicate scope
// declarations fail when the default registry is built, before any
// handler could run).
var _ = command.MustDeclareScope(command.ScopeRegistration{
	Scope:   "linear-plan-contract",
	Summary: "Strict linear-plan/linear-report schema registration and malformed/illegal apply-input rejection tests.",
})

func TestScopeLinearPlanContract(t *testing.T) {
	t.Run("linear-plan and linear-report are each registered exactly once", testLinearPlanReportRegisteredOnce)
	t.Run("global generation and drift check reach both descriptors", testLinearPlanReportGenerateAndDrift)
	t.Run("linear apply requires an explicit plan file and a matching plan id", testLinearApplyRequiresMatchingPlanID)
	t.Run("linear apply rejects duplicate JSON member names before typed use", testLinearApplyRejectsDuplicateMembers)
	t.Run("linear apply rejects unknown JSON fields before typed use", testLinearApplyRejectsUnknownFields)
	t.Run("linear apply rejects a plan whose plan_id no longer matches its recomputed hash", testLinearApplyRejectsBadDigest)
	t.Run("linear apply rejects an out-of-canonical-order operation list", testLinearApplyRejectsUnsortedOperations)
	t.Run("linear apply rejects a structurally malformed conflict", testLinearApplyRejectsInvalidConflict)
	t.Run("linear apply rejects a local id that is both planned and unresolved-conflicted", testLinearApplyRejectsIllegalTransition)
	t.Run("linear apply fails LINEAR_TRANSPORT_UNAVAILABLE before any credential subprocess or mutation access", testLinearApplyFailsWithoutFactory)
}

// knownLinearPlanReportDescriptors are the two schema names this plan
// registers.
var knownLinearPlanReportDescriptors = []string{"linear-plan", "linear-report"}

func testLinearPlanReportRegisteredOnce(t *testing.T) {
	counts := map[string]int{}
	for _, descriptor := range contracts.RegisteredSchemas() {
		counts[descriptor.Name]++
	}
	for _, name := range knownLinearPlanReportDescriptors {
		if counts[name] != 1 {
			t.Fatalf("expected %q registered exactly once, got count %d (full registry counts: %v)", name, counts[name], counts)
		}
	}
}

func testLinearPlanReportGenerateAndDrift(t *testing.T) {
	tempDir := t.TempDir()
	if err := contracts.GenerateInto(tempDir); err != nil {
		t.Fatalf("GenerateInto failed: %v", err)
	}
	for _, path := range knownLinearPlanReportDescriptors {
		outputPath := filepath.Join(tempDir, "schemas", path+".schema.json")
		if _, err := os.Stat(outputPath); err != nil {
			t.Fatalf("expected %s to be generated: %v", outputPath, err)
		}
	}

	changed, err := contracts.CheckDrift(tempDir)
	if err != nil {
		t.Fatalf("CheckDrift failed: %v", err)
	}
	for _, path := range changed {
		if path == "schemas/linear-plan.schema.json" || path == "schemas/linear-report.schema.json" {
			t.Fatalf("expected no drift for %s immediately after GenerateInto, got change list %v", path, changed)
		}
	}
}

// newLinearApplyRegistry builds the default command registry: every
// self-registered route/scope in internal/command, including "linear
// apply", is reachable exactly the way cmd/golc-project's real entrypoint
// reaches it.
func newLinearApplyRegistry(t *testing.T) *command.CommandRegistry {
	t.Helper()
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry failed: %v", err)
	}
	return registry
}

// buildValidPlan derives a real, byte-stable, correctly hashed and
// correctly ordered two-operation reconcile.Plan (a milestone and its
// dependent phase) through the exact production BuildPlan entrypoint, so
// every "malformed" test below tampers with an otherwise-legitimate plan
// rather than a hand-forged one.
func buildValidPlan(t *testing.T) reconcile.Plan {
	t.Helper()
	intents := []reconcile.Intent{
		{LocalID: "milestone:v1", Kind: "milestone", LinearType: "project", ParentLocalID: "", Fields: map[string]string{"title": "GOLC v1"}},
		{LocalID: "phase:01", Kind: "phase", LinearType: "project_milestone", ParentLocalID: "milestone:v1", Fields: map[string]string{"title": "Phase One"}},
	}
	mappings := []catalog.RemoteMapping{
		{RepoID: "milestone:v1", LinearType: "project", Status: "pending"},
		{RepoID: "phase:01", LinearType: "project_milestone", Status: "pending"},
	}
	plan, err := reconcile.BuildPlan(intents, mappings, reconcile.RemoteScope{}, nil)
	if err != nil {
		t.Fatalf("BuildPlan failed: %v", err)
	}
	if len(plan.Operations) != 2 {
		t.Fatalf("expected 2 operations, got %d", len(plan.Operations))
	}
	return plan
}

// planBodyMirror exactly mirrors reconcile's unexported planBody shape and
// internal/trace/apply/guard.go's own planBodyMirror (same JSON field
// names and order), so a test can recompute a self-consistent plan_id
// after deliberately tampering with a plan's shape -- proving the
// "unsorted"/"invalid-conflict"/"illegal-transition" rejections are
// independent checks, not merely a side effect of ValidatePlanIntegrity's
// hash self-consistency check.
type planBodyMirror struct {
	SchemaVersion     int                   `json:"schema_version"`
	IntentDigest      string                `json:"intent_digest"`
	MappingDigest     string                `json:"mapping_digest"`
	RemoteScopeDigest string                `json:"remote_scope_digest"`
	Operations        []reconcile.Operation `json:"operations"`
	Conflicts         []reconcile.Conflict  `json:"conflicts"`
}

func recomputePlanID(t *testing.T, plan reconcile.Plan) string {
	t.Helper()
	body := planBodyMirror{
		SchemaVersion:     plan.SchemaVersion,
		IntentDigest:      plan.IntentDigest,
		MappingDigest:     plan.MappingDigest,
		RemoteScopeDigest: plan.RemoteScopeDigest,
		Operations:        plan.Operations,
		Conflicts:         plan.Conflicts,
	}
	encoded, err := strictjson.CanonicalEncode(body)
	if err != nil {
		t.Fatalf("CanonicalEncode failed: %v", err)
	}
	return reconcile.PlanID(encoded)
}

func writePlanFile(t *testing.T, plan reconcile.Plan) string {
	t.Helper()
	payload, err := strictjson.CanonicalEncode(plan)
	if err != nil {
		t.Fatalf("CanonicalEncode failed: %v", err)
	}
	path := filepath.Join(t.TempDir(), "plan.json")
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	return path
}

func executeLinearApply(t *testing.T, root, planFile, planID string) command.Result {
	t.Helper()
	registry := newLinearApplyRegistry(t)
	return registry.Execute(command.Request{
		Args: []string{"linear", "apply", planFile, "--plan-id", planID},
		Root: root,
	})
}

func testLinearApplyRequiresMatchingPlanID(t *testing.T) {
	plan := buildValidPlan(t)
	root := t.TempDir()
	planFile := writePlanFile(t, plan)

	// Missing --plan-id entirely: a usage error, not a plan-content error.
	registry := newLinearApplyRegistry(t)
	usageResult := registry.Execute(command.Request{Args: []string{"linear", "apply", planFile}, Root: root})
	if usageResult.ExitCode != 2 {
		t.Fatalf("expected exit 2 for missing --plan-id, got %d (stderr: %s)", usageResult.ExitCode, usageResult.Stderr)
	}

	// A well-formed, correctly hashed plan, but --plan-id names a
	// different value than the plan's own recorded plan_id.
	mismatchResult := executeLinearApply(t, root, planFile, strings.Repeat("0", 64))
	if mismatchResult.ExitCode == 0 {
		t.Fatal("expected a non-matching --plan-id to fail")
	}
	if !strings.Contains(string(mismatchResult.Stderr), "GOLC_LINEAR_APPLY_PLAN_ID_MISMATCH") {
		t.Fatalf("expected GOLC_LINEAR_APPLY_PLAN_ID_MISMATCH, got %s", mismatchResult.Stderr)
	}
}

func testLinearApplyRejectsDuplicateMembers(t *testing.T) {
	root := t.TempDir()
	digest := strings.Repeat("a", 64)
	duplicateJSON := []byte(`{
  "schema_version": 1,
  "schema_version": 1,
  "intent_digest": "` + digest + `",
  "mapping_digest": "` + digest + `",
  "remote_scope_digest": "` + digest + `",
  "operations": [],
  "conflicts": [],
  "plan_id": "` + digest + `"
}
`)
	planFile := filepath.Join(t.TempDir(), "plan.json")
	if err := os.WriteFile(planFile, duplicateJSON, 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	result := executeLinearApply(t, root, planFile, digest)
	if result.ExitCode == 0 {
		t.Fatal("expected a duplicate top-level JSON member to be rejected")
	}
	if !strings.Contains(string(result.Stderr), "STRICTJSON_DUPLICATE_NAME") {
		t.Fatalf("expected STRICTJSON_DUPLICATE_NAME, got %s", result.Stderr)
	}
}

func testLinearApplyRejectsUnknownFields(t *testing.T) {
	root := t.TempDir()
	digest := strings.Repeat("a", 64)
	unknownFieldJSON := []byte(`{
  "schema_version": 1,
  "intent_digest": "` + digest + `",
  "mapping_digest": "` + digest + `",
  "remote_scope_digest": "` + digest + `",
  "operations": [],
  "conflicts": [],
  "plan_id": "` + digest + `",
  "bogus_field": true
}
`)
	planFile := filepath.Join(t.TempDir(), "plan.json")
	if err := os.WriteFile(planFile, unknownFieldJSON, 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	result := executeLinearApply(t, root, planFile, digest)
	if result.ExitCode == 0 {
		t.Fatal("expected an unknown JSON field to be rejected")
	}
	if !strings.Contains(string(result.Stderr), "STRICTJSON_DECODE") {
		t.Fatalf("expected STRICTJSON_DECODE (unknown field), got %s", result.Stderr)
	}
}

func testLinearApplyRejectsBadDigest(t *testing.T) {
	plan := buildValidPlan(t)
	plan.PlanID = strings.Repeat("0", 64) // recorded id no longer matches its own bytes
	root := t.TempDir()
	planFile := writePlanFile(t, plan)

	result := executeLinearApply(t, root, planFile, plan.PlanID)
	if result.ExitCode == 0 {
		t.Fatal("expected a tampered plan_id to be rejected")
	}
	if !strings.Contains(string(result.Stderr), "GOLC_APPLY_PLAN_HASH") {
		t.Fatalf("expected GOLC_APPLY_PLAN_HASH, got %s", result.Stderr)
	}
}

func testLinearApplyRejectsUnsortedOperations(t *testing.T) {
	plan := buildValidPlan(t)
	// Swap the canonical D-17 hierarchy order (milestone before phase),
	// then recompute a self-consistent hash over the tampered order so
	// ValidatePlanIntegrity's hash self-consistency check alone would
	// otherwise accept this plan.
	plan.Operations[0], plan.Operations[1] = plan.Operations[1], plan.Operations[0]
	plan.PlanID = recomputePlanID(t, plan)
	root := t.TempDir()
	planFile := writePlanFile(t, plan)

	result := executeLinearApply(t, root, planFile, plan.PlanID)
	if result.ExitCode == 0 {
		t.Fatal("expected an out-of-canonical-order operation list to be rejected")
	}
	if !strings.Contains(string(result.Stderr), "GOLC_LINEAR_APPLY_PLAN_UNSORTED") {
		t.Fatalf("expected GOLC_LINEAR_APPLY_PLAN_UNSORTED, got %s", result.Stderr)
	}
}

func testLinearApplyRejectsInvalidConflict(t *testing.T) {
	plan := buildValidPlan(t)
	// A structurally incomplete conflict: blank field and resolution
	// command, no recorded base/repository/linear values.
	plan.Conflicts = append(plan.Conflicts, reconcile.Conflict{LocalID: "phase:01", Field: "", ResolutionCommand: ""})
	plan.PlanID = recomputePlanID(t, plan)
	root := t.TempDir()
	planFile := writePlanFile(t, plan)

	result := executeLinearApply(t, root, planFile, plan.PlanID)
	if result.ExitCode == 0 {
		t.Fatal("expected a structurally malformed conflict to be rejected")
	}
	if !strings.Contains(string(result.Stderr), "GOLC_LINEAR_APPLY_PLAN_CONFLICT_INVALID") {
		t.Fatalf("expected GOLC_LINEAR_APPLY_PLAN_CONFLICT_INVALID, got %s", result.Stderr)
	}
}

func testLinearApplyRejectsIllegalTransition(t *testing.T) {
	plan := buildValidPlan(t)
	base, repo, linear := "old", "new-repo", "new-linear"
	// A well-formed conflict, but for a local id that also owns a planned
	// operation in the same plan -- an illegal simultaneous state.
	plan.Conflicts = append(plan.Conflicts, reconcile.Conflict{
		LocalID: "phase:01", Field: "title",
		BaseValue: &base, RepositoryValue: &repo, LinearValue: &linear,
		ResolutionCommand: "golc linear resolve --local-id phase:01 --field title",
	})
	plan.PlanID = recomputePlanID(t, plan)
	root := t.TempDir()
	planFile := writePlanFile(t, plan)

	result := executeLinearApply(t, root, planFile, plan.PlanID)
	if result.ExitCode == 0 {
		t.Fatal("expected a local id that is both planned and conflicted to be rejected")
	}
	if !strings.Contains(string(result.Stderr), "GOLC_LINEAR_APPLY_PLAN_ILLEGAL_TRANSITION") {
		t.Fatalf("expected GOLC_LINEAR_APPLY_PLAN_ILLEGAL_TRANSITION, got %s", result.Stderr)
	}
}

func testLinearApplyFailsWithoutFactory(t *testing.T) {
	plan := buildValidPlan(t)
	root := t.TempDir()
	planFile := writePlanFile(t, plan)

	// This test binary never wires a RemoteClientFactory (no concrete
	// process transport exists yet, per this plan's explicit scope): the
	// route must fail closed before any credential, subprocess, or
	// mutation access is ever attempted.
	result := executeLinearApply(t, root, planFile, plan.PlanID)
	if result.ExitCode == 0 {
		t.Fatal("expected apply to fail without a wired RemoteClientFactory")
	}
	if !strings.Contains(string(result.Stderr), "LINEAR_TRANSPORT_UNAVAILABLE") {
		t.Fatalf("expected LINEAR_TRANSPORT_UNAVAILABLE, got %s", result.Stderr)
	}
}
