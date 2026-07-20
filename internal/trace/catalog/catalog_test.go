// Package catalog_test covers the repository-owned planning identity
// catalog (CONTEXT D-11/D-12/D-14): durable offline IDs for the project,
// milestone, phases, requirements, dynamically discovered plans, and
// executable tasks, plus the validators that keep the local graph unique,
// source-contained, and authority-safe without Linear.
//
// It is an external test package so it can declare its quick-test scope
// through the command package's exact registration entrypoint (the
// config-local pattern from 01-02).
package catalog_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/lnorton89/golc/internal/command"
	"github.com/lnorton89/golc/internal/trace/catalog"
)

// The linear-catalog quick-test scope is declared through the exact
// production entrypoint (01-VALIDATION: every owning Go test task registers
// its scope through MustDeclareScope beside its TestScope marker).
var _ = command.MustDeclareScope(command.ScopeRegistration{
	Scope:   "linear-catalog",
	Summary: "Repository-owned planning identity catalog build and validation tests.",
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

func writeFixtureFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

const fixturePhaseSlug = "01-test-phase"

// fixtureLinearMap is a schema-1 credential-free seed matching the real
// .planning/linear-map.json boundary: stable local IDs, no remote UUIDs.
const fixtureLinearMap = `{
  "schema": 1,
  "repository": { "project_id": "project:golc", "name": "GOLC" },
  "active_milestone": { "milestone_id": "milestone:v1", "name": "GOLC v1" },
  "remote_mappings": []
}
`

func fixtureRoadmap(phaseTitle string) string {
	return strings.Join([]string{
		"# Roadmap: Fixture",
		"",
		"## Phases",
		"",
		"- [ ] **Phase 1: " + phaseTitle + "** - Fixture phase.",
		"",
		"## Phase Details",
		"",
		"### Phase 1: " + phaseTitle,
		"",
		"**Goal:** Fixture goal.",
		"**Requirements:** TSTA-01, TSTB-02",
		"",
	}, "\n")
}

func fixtureRequirements(firstText, secondText string) string {
	return strings.Join([]string{
		"# Requirements: Fixture",
		"",
		"- [ ] **TSTA-01**: " + firstText,
		"- [x] **TSTB-02**: " + secondText,
		"",
	}, "\n")
}

// fixturePlan renders a minimal PLAN.md with the real frontmatter and
// XML task structure the parser must consume.
func fixturePlan(phaseSlug, plan string, tasks []string) string {
	lines := []string{
		"---",
		"phase: " + phaseSlug,
		"plan: " + plan,
		"type: execute",
		"---",
		"",
		"## Objective",
		"",
		"Fixture plan body.",
		"",
		"<tasks>",
		"",
	}
	lines = append(lines, tasks...)
	lines = append(lines, "", "</tasks>", "")
	return strings.Join(lines, "\n")
}

func fixtureAutoTask(name string) string {
	return strings.Join([]string{
		`<task type="auto" tdd="true">`,
		"  <name>" + name + "</name>",
		"  <action>Do fixture work.</action>",
		"</task>",
	}, "\n")
}

func fixtureCheckpointTask(name string) string {
	return strings.Join([]string{
		`<task type="checkpoint:human-verify" gate="blocking-human">`,
		"  <name>" + name + "</name>",
		"</task>",
	}, "\n")
}

// newFixtureRepository builds a synthetic repository whose planning tree
// exercises dynamic discovery: plans 01, 02, and 10 (numeric ordering),
// one checkpoint task between two executable tasks, and roadmap-mapped
// requirement keys defined in REQUIREMENTS.md.
func newFixtureRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	planning := filepath.Join(root, ".planning")
	phaseDir := filepath.Join(planning, "phases", fixturePhaseSlug)

	writeFixtureFile(t, filepath.Join(planning, "linear-map.json"), fixtureLinearMap)
	writeFixtureFile(t, filepath.Join(planning, "ROADMAP.md"), fixtureRoadmap("Test Phase Display Name"))
	writeFixtureFile(t, filepath.Join(planning, "REQUIREMENTS.md"),
		fixtureRequirements("First fixture requirement text.", "Second fixture requirement text."))

	writeFixtureFile(t, filepath.Join(phaseDir, "01-01-PLAN.md"), fixturePlan(fixturePhaseSlug, "01", []string{
		fixtureAutoTask("Task 1: First executable"),
		"",
		fixtureCheckpointTask("Task 2: Human gate"),
		"",
		fixtureAutoTask("Task 3: Second executable"),
	}))
	writeFixtureFile(t, filepath.Join(phaseDir, "01-02-PLAN.md"), fixturePlan(fixturePhaseSlug, "02", []string{
		fixtureAutoTask("Task 1: Only executable"),
	}))
	writeFixtureFile(t, filepath.Join(phaseDir, "01-10-PLAN.md"), fixturePlan(fixturePhaseSlug, "10", []string{
		fixtureAutoTask("Task 1: Tenth plan executable"),
	}))
	return root
}

// requireErrorCode asserts err is non-nil and carries the stable code.
func requireErrorCode(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error with code %s, got nil", code)
	}
	if !strings.Contains(err.Error(), code) {
		t.Fatalf("expected error code %s, got: %v", code, err)
	}
}

// entityIDs returns the catalog entity IDs in catalog order.
func entityIDs(c *catalog.Catalog) []string {
	ids := make([]string, 0, len(c.Entities))
	for _, entity := range c.Entities {
		ids = append(ids, entity.ID)
	}
	return ids
}

// countID returns how many catalog entities carry the ID.
func countID(c *catalog.Catalog, id string) int {
	count := 0
	for _, entity := range c.Entities {
		if entity.ID == id {
			count++
		}
	}
	return count
}

// validSyntheticEntities returns a minimal valid entity chain used as the
// base for validator rejection tests.
func validSyntheticEntities() []catalog.Entity {
	return []catalog.Entity{
		{ID: "project:golc", Kind: catalog.KindProject, Parent: "", Display: "GOLC", Source: ".planning/linear-map.json"},
		{ID: "milestone:v1", Kind: catalog.KindMilestone, Parent: "project:golc", Display: "GOLC v1", Source: ".planning/linear-map.json"},
		{ID: "phase:01", Kind: catalog.KindPhase, Parent: "milestone:v1", Display: "Phase One", Source: ".planning/ROADMAP.md"},
		{ID: "req:TSTA-01", Kind: catalog.KindRequirement, Parent: "phase:01", Display: "Requirement text.", Source: ".planning/REQUIREMENTS.md"},
		{ID: "plan:01-01", Kind: catalog.KindPlan, Parent: "phase:01", Display: "Plan 01-01", Source: ".planning/phases/01-test-phase/01-01-PLAN.md"},
		{ID: "task:01-01.1", Kind: catalog.KindTask, Parent: "plan:01-01", Display: "Task 1", Source: ".planning/phases/01-test-phase/01-01-PLAN.md"},
	}
}

func syntheticCatalog(entities []catalog.Entity) *catalog.Catalog {
	return &catalog.Catalog{Entities: entities, Authorities: catalog.DefaultAuthorities()}
}

var (
	testPlanFilePattern = regexp.MustCompile(`^([0-9]{2})-([0-9]{2})-PLAN\.md$`)
	testPhaseDirPattern = regexp.MustCompile(`^([0-9]{2})-[a-z0-9]+(?:-[a-z0-9]+)*$`)
	testTaskTagPattern  = regexp.MustCompile(`<task\s[^>]*>`)
	testTaskTypePattern = regexp.MustCompile(`type="([^"]+)"`)
)

// scanExecutableTaskPositions independently re-derives the executable task
// positions (1-based ordinals among all task elements) from a PLAN.md.
func scanExecutableTaskPositions(t *testing.T, planPath string) []int {
	t.Helper()
	content, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatalf("read %s: %v", planPath, err)
	}
	text := string(content)
	start := strings.Index(text, "<tasks>")
	end := strings.Index(text, "</tasks>")
	if start < 0 || end < 0 || end < start {
		t.Fatalf("%s: no <tasks> region", planPath)
	}
	region := text[start:end]
	positions := []int{}
	for ordinal, tag := range testTaskTagPattern.FindAllString(region, -1) {
		typeMatch := testTaskTypePattern.FindStringSubmatch(tag)
		if typeMatch == nil {
			t.Fatalf("%s: task tag without type attribute: %s", planPath, tag)
		}
		if typeMatch[1] == "auto" {
			positions = append(positions, ordinal+1)
		}
	}
	return positions
}

// TestScopeLinearCatalog is the exact quick-test marker for scope
// "linear-catalog" (test --quick --scope linear-catalog).
func TestScopeLinearCatalog(t *testing.T) {
	t.Run("real repository catalog contains the fixed offline identities", func(t *testing.T) {
		root := repositoryRoot(t)
		built, err := catalog.BuildCatalog(root)
		if err != nil {
			t.Fatalf("BuildCatalog: %v", err)
		}
		fixed := []struct {
			id   string
			kind catalog.Kind
		}{
			{"project:golc", catalog.KindProject},
			{"milestone:v1", catalog.KindMilestone},
			{"phase:01", catalog.KindPhase},
			{"req:CONF-01", catalog.KindRequirement},
			{"req:CONF-02", catalog.KindRequirement},
			{"req:CONF-03", catalog.KindRequirement},
			{"req:CONF-04", catalog.KindRequirement},
			{"req:LINR-01", catalog.KindRequirement},
			{"req:LINR-02", catalog.KindRequirement},
			{"req:LINR-03", catalog.KindRequirement},
			{"req:LINR-04", catalog.KindRequirement},
		}
		for _, expectation := range fixed {
			entity, ok := built.Lookup(expectation.id)
			if !ok {
				t.Fatalf("catalog is missing %s", expectation.id)
			}
			if entity.Kind != expectation.kind {
				t.Fatalf("%s: kind %q, want %q", expectation.id, entity.Kind, expectation.kind)
			}
			if countID(built, expectation.id) != 1 {
				t.Fatalf("%s appears %d times, want exactly once", expectation.id, countID(built, expectation.id))
			}
		}
	})

	t.Run("real repository plans and executable tasks are discovered dynamically and exactly once", func(t *testing.T) {
		root := repositoryRoot(t)
		built, err := catalog.BuildCatalog(root)
		if err != nil {
			t.Fatalf("BuildCatalog: %v", err)
		}

		phasesDir := filepath.Join(root, ".planning", "phases")
		phaseEntries, err := os.ReadDir(phasesDir)
		if err != nil {
			t.Fatalf("read phases dir: %v", err)
		}

		discoveredPlanFiles := 0
		discoveredTasks := 0
		for _, phaseEntry := range phaseEntries {
			if !phaseEntry.IsDir() {
				continue
			}
			dirMatch := testPhaseDirPattern.FindStringSubmatch(phaseEntry.Name())
			if dirMatch == nil {
				continue
			}
			phaseNumber := dirMatch[1]
			planEntries, err := os.ReadDir(filepath.Join(phasesDir, phaseEntry.Name()))
			if err != nil {
				t.Fatalf("read phase dir: %v", err)
			}
			for _, planEntry := range planEntries {
				fileMatch := testPlanFilePattern.FindStringSubmatch(planEntry.Name())
				if fileMatch == nil {
					continue
				}
				discoveredPlanFiles++
				planID := "plan:" + phaseNumber + "-" + fileMatch[2]
				if countID(built, planID) != 1 {
					t.Fatalf("%s appears %d times, want exactly once", planID, countID(built, planID))
				}
				planPath := filepath.Join(phasesDir, phaseEntry.Name(), planEntry.Name())
				for _, position := range scanExecutableTaskPositions(t, planPath) {
					discoveredTasks++
					taskID := planID[len("plan:"):]
					fullTaskID := "task:" + taskID + "." + strconv.Itoa(position)
					if countID(built, fullTaskID) != 1 {
						t.Fatalf("%s appears %d times, want exactly once", fullTaskID, countID(built, fullTaskID))
					}
					entity, _ := built.Lookup(fullTaskID)
					if entity.Parent != planID {
						t.Fatalf("%s parent %q, want %q", fullTaskID, entity.Parent, planID)
					}
				}
			}
		}
		if discoveredPlanFiles == 0 {
			t.Fatal("independent scan found no plan files; test harness is broken")
		}

		catalogPlans := 0
		catalogTasks := 0
		for _, entity := range built.Entities {
			switch entity.Kind {
			case catalog.KindPlan:
				catalogPlans++
			case catalog.KindTask:
				catalogTasks++
			}
		}
		if catalogPlans != discoveredPlanFiles {
			t.Fatalf("catalog has %d plans, independent scan found %d", catalogPlans, discoveredPlanFiles)
		}
		if catalogTasks != discoveredTasks {
			t.Fatalf("catalog has %d tasks, independent scan found %d", catalogTasks, discoveredTasks)
		}
	})

	t.Run("fixture discovery sorts plans numerically and excludes checkpoint tasks", func(t *testing.T) {
		root := newFixtureRepository(t)
		built, err := catalog.BuildCatalog(root)
		if err != nil {
			t.Fatalf("BuildCatalog: %v", err)
		}

		planOrder := []string{}
		for _, entity := range built.Entities {
			if entity.Kind == catalog.KindPlan {
				planOrder = append(planOrder, entity.ID)
			}
		}
		wantOrder := []string{"plan:01-01", "plan:01-02", "plan:01-10"}
		if strings.Join(planOrder, ",") != strings.Join(wantOrder, ",") {
			t.Fatalf("plan order %v, want %v", planOrder, wantOrder)
		}

		for _, id := range []string{"task:01-01.1", "task:01-01.3", "task:01-02.1", "task:01-10.1"} {
			if countID(built, id) != 1 {
				t.Fatalf("%s appears %d times, want exactly once", id, countID(built, id))
			}
		}
		if countID(built, "task:01-01.2") != 0 {
			t.Fatal("checkpoint task position 2 must not receive an executable task entity")
		}

		if _, ok := built.Lookup("req:TSTA-01"); !ok {
			t.Fatal("fixture requirement req:TSTA-01 missing")
		}
		if _, ok := built.Lookup("req:TSTB-02"); !ok {
			t.Fatal("fixture requirement req:TSTB-02 missing")
		}
	})

	t.Run("display renames never change any durable id", func(t *testing.T) {
		root := newFixtureRepository(t)
		before, err := catalog.BuildCatalog(root)
		if err != nil {
			t.Fatalf("BuildCatalog before rename: %v", err)
		}

		planning := filepath.Join(root, ".planning")
		phaseDir := filepath.Join(planning, "phases", fixturePhaseSlug)
		writeFixtureFile(t, filepath.Join(planning, "ROADMAP.md"), fixtureRoadmap("Completely Renamed Phase Title"))
		writeFixtureFile(t, filepath.Join(planning, "REQUIREMENTS.md"),
			fixtureRequirements("Renamed first requirement text.", "Renamed second requirement text."))
		writeFixtureFile(t, filepath.Join(phaseDir, "01-01-PLAN.md"), fixturePlan(fixturePhaseSlug, "01", []string{
			fixtureAutoTask("Task 1: Renamed first executable"),
			"",
			fixtureCheckpointTask("Task 2: Renamed human gate"),
			"",
			fixtureAutoTask("Task 3: Renamed second executable"),
		}))

		after, err := catalog.BuildCatalog(root)
		if err != nil {
			t.Fatalf("BuildCatalog after rename: %v", err)
		}
		beforeIDs := strings.Join(entityIDs(before), "\n")
		afterIDs := strings.Join(entityIDs(after), "\n")
		if beforeIDs != afterIDs {
			t.Fatalf("IDs changed across display renames:\nbefore:\n%s\nafter:\n%s", beforeIDs, afterIDs)
		}
	})

	t.Run("plan filenames violating the grammar are rejected", func(t *testing.T) {
		root := newFixtureRepository(t)
		phaseDir := filepath.Join(root, ".planning", "phases", fixturePhaseSlug)

		writeFixtureFile(t, filepath.Join(phaseDir, "01-3-PLAN.md"), fixturePlan(fixturePhaseSlug, "3", []string{
			fixtureAutoTask("Task 1: Bad filename"),
		}))
		_, err := catalog.BuildCatalog(root)
		requireErrorCode(t, err, "GOLC_CATALOG_PLAN_FILENAME")
		if err := os.Remove(filepath.Join(phaseDir, "01-3-PLAN.md")); err != nil {
			t.Fatalf("remove: %v", err)
		}

		writeFixtureFile(t, filepath.Join(phaseDir, "02-05-PLAN.md"), fixturePlan(fixturePhaseSlug, "05", []string{
			fixtureAutoTask("Task 1: Wrong phase prefix"),
		}))
		_, err = catalog.BuildCatalog(root)
		requireErrorCode(t, err, "GOLC_CATALOG_PLAN_FILENAME")
	})

	t.Run("plan frontmatter must match the filename structure", func(t *testing.T) {
		root := newFixtureRepository(t)
		phaseDir := filepath.Join(root, ".planning", "phases", fixturePhaseSlug)

		writeFixtureFile(t, filepath.Join(phaseDir, "01-05-PLAN.md"), fixturePlan(fixturePhaseSlug, "06", []string{
			fixtureAutoTask("Task 1: Mismatched plan number"),
		}))
		_, err := catalog.BuildCatalog(root)
		requireErrorCode(t, err, "GOLC_CATALOG_PLAN_FRONTMATTER")
		if err := os.Remove(filepath.Join(phaseDir, "01-05-PLAN.md")); err != nil {
			t.Fatalf("remove: %v", err)
		}

		writeFixtureFile(t, filepath.Join(phaseDir, "01-06-PLAN.md"), fixturePlan("99-wrong-phase", "06", []string{
			fixtureAutoTask("Task 1: Mismatched phase slug"),
		}))
		_, err = catalog.BuildCatalog(root)
		requireErrorCode(t, err, "GOLC_CATALOG_PLAN_FRONTMATTER")
	})

	t.Run("id grammar accepts durable shapes and rejects display or issue-key shapes", func(t *testing.T) {
		accepted := []struct {
			id   string
			kind catalog.Kind
		}{
			{"project:golc", catalog.KindProject},
			{"milestone:v1", catalog.KindMilestone},
			{"phase:01", catalog.KindPhase},
			{"req:CONF-01", catalog.KindRequirement},
			{"plan:01-08", catalog.KindPlan},
			{"task:01-08.1", catalog.KindTask},
			{"task:01-08.12", catalog.KindTask},
		}
		for _, expectation := range accepted {
			parsed, err := catalog.ParseID(expectation.id)
			if err != nil {
				t.Fatalf("ParseID(%q): %v", expectation.id, err)
			}
			if parsed.Kind != expectation.kind {
				t.Fatalf("ParseID(%q) kind %q, want %q", expectation.id, parsed.Kind, expectation.kind)
			}
		}

		rejected := []string{
			"",
			"golc",
			"unknown:x",
			"project:GOLC",
			"project:",
			"milestone:1",
			"milestone:vNext",
			"phase:1",
			"phase:001",
			"req:conf-01",
			"req:GOLC-123",
			"req:CONF-1",
			"plan:1-8",
			"plan:01-08.1",
			"task:01-08",
			"task:01-08.0",
			"task:01-08.",
			"task:Display Name.1",
		}
		for _, id := range rejected {
			if _, err := catalog.ParseID(id); err == nil {
				t.Fatalf("ParseID(%q) accepted an invalid id", id)
			}
		}

		if id, err := catalog.PhaseID("01"); err != nil || id != "phase:01" {
			t.Fatalf("PhaseID(01) = %q, %v", id, err)
		}
		if _, err := catalog.PhaseID("1"); err == nil {
			t.Fatal("PhaseID(1) accepted a one-digit phase")
		}
		if id, err := catalog.PlanID("01", "08"); err != nil || id != "plan:01-08" {
			t.Fatalf("PlanID(01,08) = %q, %v", id, err)
		}
		if _, err := catalog.PlanID("1", "8"); err == nil {
			t.Fatal("PlanID(1,8) accepted one-digit parts")
		}
		if id, err := catalog.TaskID("01", "08", 1); err != nil || id != "task:01-08.1" {
			t.Fatalf("TaskID(01,08,1) = %q, %v", id, err)
		}
		if _, err := catalog.TaskID("01", "08", 0); err == nil {
			t.Fatal("TaskID position 0 accepted")
		}
		if id, err := catalog.RequirementID("CONF-01"); err != nil || id != "req:CONF-01" {
			t.Fatalf("RequirementID(CONF-01) = %q, %v", id, err)
		}
		if _, err := catalog.RequirementID("GOLC-123"); err == nil {
			t.Fatal("RequirementID accepted an issue-key shape")
		}
	})

	t.Run("validators reject duplicates wrong parents cycles and external sources", func(t *testing.T) {
		base := validSyntheticEntities()

		if err := catalog.Validate(syntheticCatalog(base)); err != nil {
			t.Fatalf("valid synthetic catalog rejected: %v", err)
		}

		fresh := catalog.NewCatalog()
		for _, entity := range base {
			if err := fresh.Add(entity); err != nil {
				t.Fatalf("Add(%s): %v", entity.ID, err)
			}
		}
		requireErrorCode(t, fresh.Add(base[len(base)-1]), "GOLC_CATALOG_ID_DUPLICATE")

		duplicated := append(append([]catalog.Entity{}, base...), base[2])
		requireErrorCode(t, catalog.Validate(syntheticCatalog(duplicated)), "GOLC_CATALOG_ID_DUPLICATE")

		orphan := append(append([]catalog.Entity{}, base...), catalog.Entity{
			ID: "plan:02-01", Kind: catalog.KindPlan, Parent: "phase:02",
			Display: "Orphan", Source: ".planning/phases/01-test-phase/01-01-PLAN.md",
		})
		requireErrorCode(t, catalog.Validate(syntheticCatalog(orphan)), "GOLC_CATALOG_PARENT_UNKNOWN")

		wrongKind := append(append([]catalog.Entity{}, base...), catalog.Entity{
			ID: "task:01-01.2", Kind: catalog.KindTask, Parent: "phase:01",
			Display: "Task parented to a phase", Source: ".planning/phases/01-test-phase/01-01-PLAN.md",
		})
		requireErrorCode(t, catalog.Validate(syntheticCatalog(wrongKind)), "GOLC_CATALOG_PARENT_KIND")

		cyclic := syntheticCatalog([]catalog.Entity{
			{ID: "phase:01", Kind: catalog.KindPhase, Parent: "phase:02", Display: "A", Source: ".planning/ROADMAP.md"},
			{ID: "phase:02", Kind: catalog.KindPhase, Parent: "phase:01", Display: "B", Source: ".planning/ROADMAP.md"},
		})
		requireErrorCode(t, catalog.Validate(cyclic), "GOLC_CATALOG_CYCLE")

		mismatched := append(append([]catalog.Entity{}, base...), catalog.Entity{
			ID: "plan:02-07", Kind: catalog.KindPlan, Parent: "phase:01",
			Display: "Plan from another phase", Source: ".planning/phases/01-test-phase/01-01-PLAN.md",
		})
		requireErrorCode(t, catalog.Validate(syntheticCatalog(mismatched)), "GOLC_CATALOG_PARENT_MISMATCH")

		taskMismatch := append(append([]catalog.Entity{}, base...), catalog.Entity{
			ID: "task:01-02.1", Kind: catalog.KindTask, Parent: "plan:01-01",
			Display: "Task from another plan", Source: ".planning/phases/01-test-phase/01-01-PLAN.md",
		})
		requireErrorCode(t, catalog.Validate(syntheticCatalog(taskMismatch)), "GOLC_CATALOG_PARENT_MISMATCH")

		externalSources := []struct {
			source string
			code   string
		}{
			{"", "GOLC_CATALOG_SOURCE_MISSING"},
			{"C:/outside/evil.md", "GOLC_CATALOG_SOURCE_EXTERNAL"},
			{"/etc/evil.md", "GOLC_CATALOG_SOURCE_EXTERNAL"},
			{".planning/../.env", "GOLC_CATALOG_SOURCE_EXTERNAL"},
			{"https://linear.app/issue/GOLC-123", "GOLC_CATALOG_SOURCE_EXTERNAL"},
			{"docs/development.md", "GOLC_CATALOG_SOURCE_EXTERNAL"},
		}
		for _, expectation := range externalSources {
			entities := append([]catalog.Entity{}, base...)
			entities[3].Source = expectation.source
			requireErrorCode(t, catalog.Validate(syntheticCatalog(entities)), expectation.code)
		}
	})

	t.Run("authority split is typed and comments are excluded", func(t *testing.T) {
		built := syntheticCatalog(validSyntheticEntities())

		for _, field := range []string{"scope", "local_id", "requirement_text", "structure"} {
			requireErrorCode(t, built.SetAuthority(field, catalog.AuthorityLinear),
				"GOLC_CATALOG_AUTHORITY_REPOSITORY_FIELD")
		}
		for _, field := range []string{"status", "assignee", "priority", "estimate", "completed_at"} {
			requireErrorCode(t, built.SetAuthority(field, catalog.AuthorityRepository),
				"GOLC_CATALOG_AUTHORITY_LINEAR_FIELD")
			if err := built.SetAuthority(field, catalog.AuthorityLinear); err != nil {
				t.Fatalf("confirming linear ownership of %s failed: %v", field, err)
			}
		}
		for _, field := range []string{"comment", "comments", "discussion"} {
			requireErrorCode(t, built.SetAuthority(field, catalog.AuthorityRepository),
				"GOLC_CATALOG_COMMENT_EXCLUDED")
			requireErrorCode(t, built.SetAuthority(field, catalog.AuthorityLinear),
				"GOLC_CATALOG_COMMENT_EXCLUDED")
		}
		requireErrorCode(t, built.SetAuthority("unregistered_field", catalog.AuthorityRepository),
			"GOLC_CATALOG_FIELD_UNKNOWN")

		if err := catalog.Validate(built); err != nil {
			t.Fatalf("untampered authorities rejected: %v", err)
		}

		tampered := syntheticCatalog(validSyntheticEntities())
		tampered.Authorities["scope"] = catalog.AuthorityLinear
		requireErrorCode(t, catalog.Validate(tampered), "GOLC_CATALOG_AUTHORITY_REPOSITORY_FIELD")

		commented := syntheticCatalog(validSyntheticEntities())
		commented.Authorities["comment"] = catalog.AuthorityLinear
		requireErrorCode(t, catalog.Validate(commented), "GOLC_CATALOG_COMMENT_EXCLUDED")

		extra := syntheticCatalog(validSyntheticEntities())
		extra.Authorities["surprise"] = catalog.AuthorityRepository
		requireErrorCode(t, catalog.Validate(extra), "GOLC_CATALOG_FIELD_UNKNOWN")

		incomplete := syntheticCatalog(validSyntheticEntities())
		delete(incomplete.Authorities, "status")
		requireErrorCode(t, catalog.Validate(incomplete), "GOLC_CATALOG_AUTHORITY_INCOMPLETE")
	})

	t.Run("real repository catalog validates end to end offline", func(t *testing.T) {
		root := repositoryRoot(t)
		built, err := catalog.BuildCatalog(root)
		if err != nil {
			t.Fatalf("BuildCatalog: %v", err)
		}
		if err := catalog.Validate(built); err != nil {
			t.Fatalf("Validate: %v", err)
		}
	})
}
