// linear_validate_test.go covers the exact "linear validate" route
// (linear_validate.go) that was previously reachable only in production:
// parseOfflineArgs' accepted argument grammar, catalogCounts' per-kind
// tally, and runLinearValidate's full branching -- generated-schema drift
// (T-01-24), committed-map migration drift (catalog.CheckMigration), and
// the successful offline catalog build. It is package command (an
// internal test) so it can call the unexported parseOfflineArgs,
// catalogCounts, and runLinearValidate entrypoints directly, matching
// linear_test.go's exact convention for the sibling "linear apply" route.
package command

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lnorton89/golc/internal/contracts"
	"github.com/lnorton89/golc/internal/trace/catalog"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// parseOfflineArgs: the exact "linear validate --offline" argument grammar.
// ---------------------------------------------------------------------------

func TestParseOfflineArgs(t *testing.T) {
	const usage = "linear validate --offline"

	t.Run("accepts exactly --offline", func(t *testing.T) {
		err := parseOfflineArgs(usage, []string{"--offline"})
		require.NoError(t, err)
	})

	t.Run("rejects an empty argument list", func(t *testing.T) {
		err := parseOfflineArgs(usage, []string{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "GOLC_LINEAR_USAGE")
		require.Contains(t, err.Error(), usage)
	})

	t.Run("rejects a nil argument list", func(t *testing.T) {
		err := parseOfflineArgs(usage, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "GOLC_LINEAR_USAGE")
	})

	t.Run("rejects an unsupported argument", func(t *testing.T) {
		err := parseOfflineArgs(usage, []string{"--bogus"})
		require.Error(t, err)
		require.Contains(t, err.Error(), "GOLC_LINEAR_USAGE")
		require.Contains(t, err.Error(), `"--bogus"`)
	})

	t.Run("rejects --offline followed by an unsupported argument", func(t *testing.T) {
		err := parseOfflineArgs(usage, []string{"--offline", "--extra"})
		require.Error(t, err)
		require.Contains(t, err.Error(), `"--extra"`)
	})

	t.Run("accepts a repeated --offline flag (documents the literal accepted grammar)", func(t *testing.T) {
		// parseOfflineArgs only ever rejects a non-"--offline" token; it never
		// rejects the flag appearing more than once. This subtest documents
		// that literal, slightly permissive behavior rather than an assumed
		// stricter one.
		err := parseOfflineArgs(usage, []string{"--offline", "--offline"})
		require.NoError(t, err)
	})
}

// ---------------------------------------------------------------------------
// catalogCounts: per-kind entity tally over a built catalog.
// ---------------------------------------------------------------------------

func TestCatalogCounts(t *testing.T) {
	t.Run("tallies mixed entity kinds by Kind string", func(t *testing.T) {
		built := &catalog.Catalog{
			Entities: []catalog.Entity{
				{ID: "project:golc", Kind: catalog.KindProject, Display: "GOLC", Source: ".planning/linear-map.json"},
				{ID: "milestone:v1", Kind: catalog.KindMilestone, Parent: "project:golc", Display: "GOLC v1", Source: ".planning/linear-map.json"},
				{ID: "phase:01", Kind: catalog.KindPhase, Parent: "milestone:v1", Display: "Phase One", Source: ".planning/ROADMAP.md"},
				{ID: "req:TSTA-01", Kind: catalog.KindRequirement, Parent: "phase:01", Display: "First requirement.", Source: ".planning/REQUIREMENTS.md"},
				{ID: "req:TSTB-02", Kind: catalog.KindRequirement, Parent: "phase:01", Display: "Second requirement.", Source: ".planning/REQUIREMENTS.md"},
				{ID: "plan:01-01", Kind: catalog.KindPlan, Parent: "phase:01", Display: "Plan 01-01", Source: ".planning/phases/01-test/01-01-PLAN.md"},
				{ID: "task:01-01.1", Kind: catalog.KindTask, Parent: "plan:01-01", Display: "Task 1", Source: ".planning/phases/01-test/01-01-PLAN.md"},
				{ID: "task:01-01.2", Kind: catalog.KindTask, Parent: "plan:01-01", Display: "Task 2", Source: ".planning/phases/01-test/01-01-PLAN.md"},
				{ID: "task:01-01.3", Kind: catalog.KindTask, Parent: "plan:01-01", Display: "Task 3", Source: ".planning/phases/01-test/01-01-PLAN.md"},
			},
		}

		counts := catalogCounts(built)

		require.Equal(t, map[string]int{
			"project":   1,
			"milestone": 1,
			"phase":     1,
			"req":       2,
			"plan":      1,
			"task":      3,
		}, counts)
	})

	t.Run("returns an empty map for a catalog with no entities", func(t *testing.T) {
		built := &catalog.Catalog{}
		counts := catalogCounts(built)
		require.Equal(t, map[string]int{}, counts)
	})

	t.Run("counts only Kind, ignoring ID/Parent/Display/Source", func(t *testing.T) {
		// Two distinct entities that happen to share a Kind still collapse
		// into one tally bucket -- catalogCounts is a pure per-Kind count,
		// never a per-entity dedupe or identity check.
		built := &catalog.Catalog{
			Entities: []catalog.Entity{
				{ID: "task:01-01.1", Kind: catalog.KindTask, Display: "Task 1"},
				{ID: "task:01-01.2", Kind: catalog.KindTask, Display: "Task 2"},
			},
		}
		require.Equal(t, map[string]int{"task": 2}, catalogCounts(built))
	})
}

// ---------------------------------------------------------------------------
// runLinearValidate fixture repository: mirrors linear_test.go's own
// newApplyReplayFixtureRepository shape (one phase, one requirement, one
// plan, one task -- five non-project catalog entities, six including the
// project root), kept independent (distinct names) so this file owns its
// own fixture and never depends on a sibling test file's unexported
// helpers.
// ---------------------------------------------------------------------------

const validateFixturePhaseSlug = "01-validate-phase"

// validateFixtureSeedLinearMap is the raw schema-1 seed exactly as a fresh
// repository would commit it: a credential-free identity seed with no
// remote mappings yet. catalog.MigrateV1ToV2 always derives a fresh
// "pending" remote mapping for every non-project entity, so this seed's
// empty remote_mappings array never byte-matches the canonical migration
// output -- deliberately used below to exercise runLinearValidate's
// catalog.CheckMigration drift branch without hand-corrupting any bytes.
const validateFixtureSeedLinearMap = `{
  "schema": 1,
  "repository": { "project_id": "project:golc", "name": "GOLC" },
  "active_milestone": { "milestone_id": "milestone:v1", "name": "GOLC v1" },
  "remote_mappings": []
}
`

func writeValidateFixtureFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755), "mkdir %s", filepath.Dir(path))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644), "write %s", path)
}

func validateFixtureRoadmap() string {
	return strings.Join([]string{
		"# Roadmap: Validate Fixture",
		"",
		"## Phases",
		"",
		"- [ ] **Phase 1: Validate Fixture Phase** - Fixture phase.",
		"",
		"## Phase Details",
		"",
		"### Phase 1: Validate Fixture Phase",
		"",
		"**Goal:** Fixture goal.",
		"**Requirements:** TVAL-01",
		"",
	}, "\n")
}

func validateFixtureRequirements() string {
	return strings.Join([]string{
		"# Requirements: Validate Fixture",
		"",
		"- [ ] **TVAL-01**: Fixture requirement text.",
		"",
	}, "\n")
}

func validateFixturePlan() string {
	return strings.Join([]string{
		"---",
		"phase: " + validateFixturePhaseSlug,
		"plan: 01",
		"type: execute",
		"---",
		"",
		"## Objective",
		"",
		"Fixture plan body.",
		"",
		"<tasks>",
		"",
		`<task type="auto" tdd="true">`,
		"  <name>Task 1: Only executable</name>",
		"  <action>Do fixture work.</action>",
		"</task>",
		"",
		"</tasks>",
		"",
	}, "\n")
}

// newValidateFixtureRepository builds a synthetic repository root with a
// complete, offline-buildable .planning/ tree: one phase, one requirement,
// one plan, one task -- exactly six catalog entities (project, milestone,
// phase, req, plan, task), one of each kind, once built. The committed
// .planning/linear-map.json is the raw, not-yet-migrated schema-1 seed
// (validateFixtureSeedLinearMap); callers that need a fully migrated,
// drift-free repository must additionally call catalog.WriteMigration.
func newValidateFixtureRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	planning := filepath.Join(root, ".planning")
	phaseDir := filepath.Join(planning, "phases", validateFixturePhaseSlug)

	writeValidateFixtureFile(t, filepath.Join(planning, "linear-map.json"), validateFixtureSeedLinearMap)
	writeValidateFixtureFile(t, filepath.Join(planning, "ROADMAP.md"), validateFixtureRoadmap())
	writeValidateFixtureFile(t, filepath.Join(planning, "REQUIREMENTS.md"), validateFixtureRequirements())
	writeValidateFixtureFile(t, filepath.Join(phaseDir, "01-01-PLAN.md"), validateFixturePlan())
	return root
}

// seedCommittedSchemas generates every registered contracts schema
// (including "linear-map") directly at its committed repository-relative
// path under root, matching internal/contracts/generate_test.go's own
// "seed with GenerateInto, then CheckDrift reports no changes" convention.
// Without this, contracts.CheckDrift always reports schemas/linear-map.schema.json
// as changed (missing) against a bare fixture root.
func seedCommittedSchemas(t *testing.T, root string) {
	t.Helper()
	require.NoError(t, contracts.GenerateInto(root), "seed committed schemas via GenerateInto")
}

// ---------------------------------------------------------------------------
// runLinearValidate
// ---------------------------------------------------------------------------

func TestRunLinearValidate(t *testing.T) {
	t.Run("invalid offline arguments are rejected before touching the repository", func(t *testing.T) {
		// No fixture repository is even built: parseOfflineArgs runs first,
		// so an unusable/nonexistent root never matters for this case.
		request := Request{Root: t.TempDir(), Args: []string{"--bogus"}}

		result := runLinearValidate(request)

		require.Equal(t, 2, result.ExitCode)
		require.Contains(t, string(result.Stderr), "GOLC_LINEAR_USAGE")
		require.Empty(t, result.Stdout)
	})

	t.Run("generated-schema drift is reported before any catalog access", func(t *testing.T) {
		// A bare root with no committed schemas/ directory at all: every
		// registered schema (including linear-map) is "missing" relative to
		// a fresh generation, so the T-01-24 drift check fails first --
		// before runLinearValidate ever reaches .planning/ or catalog.
		root := t.TempDir()
		request := Request{Root: root, Args: []string{"--offline"}}

		result := runLinearValidate(request)

		require.Equal(t, 1, result.ExitCode)
		require.Contains(t, string(result.Stderr), "GOLC_LINEAR_VALIDATE_SCHEMA_DRIFT")
		require.Contains(t, string(result.Stderr), linearMapSchemaOutputPath)
		require.Empty(t, result.Stdout)
	})

	t.Run("committed map migration drift is reported once the schema check passes", func(t *testing.T) {
		root := newValidateFixtureRepository(t)
		seedCommittedSchemas(t, root)
		// Deliberately skip catalog.WriteMigration: the committed
		// .planning/linear-map.json is still the raw schema-1 seed with an
		// empty remote_mappings array, which never byte-matches the
		// canonical schema-2 migration catalog.CheckMigration re-derives.
		request := Request{Root: root, Args: []string{"--offline"}}

		result := runLinearValidate(request)

		require.Equal(t, 1, result.ExitCode)
		require.Contains(t, string(result.Stderr), "GOLC_MIGRATE_")
		require.NotContains(t, string(result.Stderr), "GOLC_LINEAR_VALIDATE_SCHEMA_DRIFT",
			"a map-migration drift failure should not also report a schema drift failure")
		require.Empty(t, result.Stdout)
	})

	t.Run("a fully migrated, drift-free repository returns ok with counts and entities", func(t *testing.T) {
		root := newValidateFixtureRepository(t)
		seedCommittedSchemas(t, root)
		require.NoError(t, catalog.WriteMigration(root), "canonicalize the committed linear-map.json")
		request := Request{Root: root, Args: []string{"--offline"}}

		result := runLinearValidate(request)

		require.Equal(t, 0, result.ExitCode, "stderr: %s", result.Stderr)
		require.Empty(t, result.Stderr)

		var view linearValidateView
		require.NoError(t, json.Unmarshal(result.Stdout, &view), "decode runLinearValidate stdout: %s", result.Stdout)

		require.Equal(t, "ok", view.Status)
		require.Equal(t, map[string]int{
			"project":   1,
			"milestone": 1,
			"phase":     1,
			"req":       1,
			"plan":      1,
			"task":      1,
		}, view.Counts)

		require.Len(t, view.Entities, 6)
		gotIDs := make([]string, 0, len(view.Entities))
		for _, entity := range view.Entities {
			gotIDs = append(gotIDs, entity.ID)
			require.NotEmpty(t, entity.Kind, "entity %s has no Kind", entity.ID)
			require.NotEmpty(t, entity.Source, "entity %s has no Source", entity.ID)
		}
		require.ElementsMatch(t, []string{
			"project:golc",
			"milestone:v1",
			"phase:01",
			"req:TVAL-01",
			"plan:01-01",
			"task:01-01.1",
		}, gotIDs)
	})

	t.Run("replaying the same fully migrated repository is stable (no drift introduced by validation itself)", func(t *testing.T) {
		root := newValidateFixtureRepository(t)
		seedCommittedSchemas(t, root)
		require.NoError(t, catalog.WriteMigration(root))
		request := Request{Root: root, Args: []string{"--offline"}}

		first := runLinearValidate(request)
		require.Equal(t, 0, first.ExitCode, "stderr: %s", first.Stderr)

		second := runLinearValidate(request)
		require.Equal(t, 0, second.ExitCode, "stderr: %s", second.Stderr)
		require.Equal(t, first.Stdout, second.Stdout, "runLinearValidate is read-only; replaying it must produce byte-identical output")
	})
}
