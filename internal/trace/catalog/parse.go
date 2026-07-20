// parse.go builds the complete dynamic repository-owned planning identity
// catalog from committed planning artifacts only (CONTEXT D-11): the
// stable seed in .planning/linear-map.json, roadmap phase structure,
// requirement keys, dynamically discovered NN-MM-PLAN.md files, and the
// XML task positions inside each plan. Nothing is fetched from Linear and
// no plan count is ever assumed.
package catalog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	// phaseDirectoryPattern is the enforced phase directory grammar:
	// two-digit phase number plus a lowercase slug.
	phaseDirectoryPattern = regexp.MustCompile(`^([0-9]{2})-[a-z0-9]+(?:-[a-z0-9]+)*$`)
	// planFileNamePattern is the enforced plan filename grammar, e.g.
	// 01-08-PLAN.md: the phase prefix must match the owning directory.
	planFileNamePattern = regexp.MustCompile(`^([0-9]{2})-([0-9]{2})-PLAN\.md$`)
	// planFileSuffixPattern flags anything that looks like a plan file so
	// grammar violations fail loudly instead of being silently skipped.
	planFileSuffixPattern = regexp.MustCompile(`-PLAN\.md$`)

	roadmapPhaseHeadingPattern = regexp.MustCompile(`(?m)^### Phase ([0-9]+): (.+?)\s*$`)
	roadmapRequirementsPattern = regexp.MustCompile(`(?m)^\*\*Requirements:\*\*\s*(.+?)\s*$`)
	requirementLinePattern     = regexp.MustCompile(`(?m)^- \[[ x]\] \*\*([A-Z]{2,10}-[0-9]{2})\*\*:\s*(.+?)\s*$`)

	taskTagPattern  = regexp.MustCompile(`<task\s[^>]*>`)
	taskTypePattern = regexp.MustCompile(`type="([^"]+)"`)
	taskNamePattern = regexp.MustCompile(`<name>(.*?)</name>`)
)

// linearMapSeed is the credential-free schema-1 stable identity seed in
// .planning/linear-map.json. Only the durable local IDs and display names
// are consumed; remote mappings never contribute local identity.
type linearMapSeed struct {
	Schema     int `json:"schema"`
	Repository struct {
		ProjectID string `json:"project_id"`
		Name      string `json:"name"`
	} `json:"repository"`
	ActiveMilestone struct {
		MilestoneID string `json:"milestone_id"`
		Name        string `json:"name"`
	} `json:"active_milestone"`
}

// BuildCatalog assembles and validates the complete local identity graph
// for every phase directory, plan file, and executable task discovered
// under root. It is offline by construction: only committed repository
// files are read.
func BuildCatalog(root string) (*Catalog, error) {
	built := NewCatalog()

	seed, err := loadLinearMapSeed(root)
	if err != nil {
		return nil, err
	}
	if err := built.Add(Entity{
		ID:      seed.Repository.ProjectID,
		Kind:    KindProject,
		Display: seed.Repository.Name,
		Source:  ".planning/linear-map.json",
	}); err != nil {
		return nil, err
	}
	if err := built.Add(Entity{
		ID:      seed.ActiveMilestone.MilestoneID,
		Kind:    KindMilestone,
		Parent:  seed.Repository.ProjectID,
		Display: seed.ActiveMilestone.Name,
		Source:  ".planning/linear-map.json",
	}); err != nil {
		return nil, err
	}

	roadmapContent, err := os.ReadFile(filepath.Join(root, ".planning", "ROADMAP.md"))
	if err != nil {
		return nil, fmt.Errorf("GOLC_CATALOG_SOURCE_UNREADABLE: .planning/ROADMAP.md: %v", err)
	}
	requirementTexts, err := loadRequirementTexts(root)
	if err != nil {
		return nil, err
	}

	phaseDirs, err := discoverPhaseDirectories(root)
	if err != nil {
		return nil, err
	}
	for _, phaseDir := range phaseDirs {
		if err := buildPhase(built, root, phaseDir, string(roadmapContent), requirementTexts, seed.ActiveMilestone.MilestoneID); err != nil {
			return nil, err
		}
	}

	if err := Validate(built); err != nil {
		return nil, err
	}
	return built, nil
}

// loadLinearMapSeed reads and grammar-checks the stable identity seed.
func loadLinearMapSeed(root string) (*linearMapSeed, error) {
	content, err := os.ReadFile(filepath.Join(root, ".planning", "linear-map.json"))
	if err != nil {
		return nil, fmt.Errorf("GOLC_CATALOG_SOURCE_UNREADABLE: .planning/linear-map.json: %v", err)
	}
	seed := &linearMapSeed{}
	if err := json.Unmarshal(content, seed); err != nil {
		return nil, fmt.Errorf("GOLC_CATALOG_SEED_INVALID: .planning/linear-map.json: %v", err)
	}
	projectParsed, err := ParseID(seed.Repository.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("GOLC_CATALOG_SEED_INVALID: repository.project_id: %v", err)
	}
	if projectParsed.Kind != KindProject {
		return nil, fmt.Errorf("GOLC_CATALOG_SEED_INVALID: repository.project_id %q is not a project id", seed.Repository.ProjectID)
	}
	milestoneParsed, err := ParseID(seed.ActiveMilestone.MilestoneID)
	if err != nil {
		return nil, fmt.Errorf("GOLC_CATALOG_SEED_INVALID: active_milestone.milestone_id: %v", err)
	}
	if milestoneParsed.Kind != KindMilestone {
		return nil, fmt.Errorf("GOLC_CATALOG_SEED_INVALID: active_milestone.milestone_id %q is not a milestone id", seed.ActiveMilestone.MilestoneID)
	}
	return seed, nil
}

// loadRequirementTexts indexes the authoritative requirement text by key
// from .planning/REQUIREMENTS.md (repository-owned per D-11).
func loadRequirementTexts(root string) (map[string]string, error) {
	content, err := os.ReadFile(filepath.Join(root, ".planning", "REQUIREMENTS.md"))
	if err != nil {
		return nil, fmt.Errorf("GOLC_CATALOG_SOURCE_UNREADABLE: .planning/REQUIREMENTS.md: %v", err)
	}
	texts := map[string]string{}
	for _, match := range requirementLinePattern.FindAllStringSubmatch(string(content), -1) {
		texts[match[1]] = match[2]
	}
	return texts, nil
}

// discoverPhaseDirectories lists phase directories under .planning/phases
// in sorted order, rejecting entries that carry a numeric phase prefix
// but violate the directory grammar.
func discoverPhaseDirectories(root string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(root, ".planning", "phases"))
	if err != nil {
		return nil, fmt.Errorf("GOLC_CATALOG_SOURCE_UNREADABLE: .planning/phases: %v", err)
	}
	names := []string{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if phaseDirectoryPattern.MatchString(name) {
			names = append(names, name)
			continue
		}
		if regexp.MustCompile(`^[0-9]{2}-`).MatchString(name) {
			return nil, fmt.Errorf("GOLC_CATALOG_PHASE_DIRNAME: %q does not match the NN-slug phase directory grammar", name)
		}
	}
	sort.Strings(names)
	return names, nil
}

// buildPhase adds one phase, its roadmap-mapped requirements, its
// dynamically discovered plans, and their executable tasks.
func buildPhase(built *Catalog, root, phaseDirName, roadmap string, requirementTexts map[string]string, milestoneID string) error {
	phaseNumber := phaseDirectoryPattern.FindStringSubmatch(phaseDirName)[1]
	phaseID, err := PhaseID(phaseNumber)
	if err != nil {
		return err
	}

	title, requirementKeys, err := roadmapPhaseStructure(roadmap, phaseNumber)
	if err != nil {
		return err
	}
	if err := built.Add(Entity{
		ID:      phaseID,
		Kind:    KindPhase,
		Parent:  milestoneID,
		Display: title,
		Source:  ".planning/ROADMAP.md",
	}); err != nil {
		return err
	}

	for _, key := range requirementKeys {
		requirementID, err := RequirementID(key)
		if err != nil {
			return err
		}
		text, defined := requirementTexts[key]
		if !defined {
			return fmt.Errorf("GOLC_CATALOG_REQUIREMENT_UNDEFINED: %q is mapped to phase %s in ROADMAP.md but not defined in REQUIREMENTS.md", key, phaseNumber)
		}
		if err := built.Add(Entity{
			ID:      requirementID,
			Kind:    KindRequirement,
			Parent:  phaseID,
			Display: text,
			Source:  ".planning/REQUIREMENTS.md",
		}); err != nil {
			return err
		}
	}

	planNumbers, err := discoverPlanFiles(root, phaseDirName, phaseNumber)
	if err != nil {
		return err
	}
	for _, planNumber := range planNumbers {
		if err := buildPlan(built, root, phaseDirName, phaseNumber, planNumber, phaseID); err != nil {
			return err
		}
	}
	return nil
}

// roadmapPhaseStructure extracts the display title and requirement keys
// for one phase from the authoritative roadmap structure.
func roadmapPhaseStructure(roadmap, phaseNumber string) (string, []string, error) {
	numeric, err := strconv.Atoi(phaseNumber)
	if err != nil {
		return "", nil, fmt.Errorf("GOLC_CATALOG_PHASE_DIRNAME: phase number %q: %v", phaseNumber, err)
	}
	headings := roadmapPhaseHeadingPattern.FindAllStringSubmatchIndex(roadmap, -1)
	for index, heading := range headings {
		number := roadmap[heading[2]:heading[3]]
		if number != strconv.Itoa(numeric) {
			continue
		}
		title := roadmap[heading[4]:heading[5]]
		sectionEnd := len(roadmap)
		if index+1 < len(headings) {
			sectionEnd = headings[index+1][0]
		}
		section := roadmap[heading[1]:sectionEnd]
		requirementsMatch := roadmapRequirementsPattern.FindStringSubmatch(section)
		if requirementsMatch == nil {
			return "", nil, fmt.Errorf("GOLC_CATALOG_ROADMAP_REQUIREMENTS_MISSING: phase %s has no **Requirements:** line in ROADMAP.md", phaseNumber)
		}
		keys := []string{}
		for _, key := range strings.Split(requirementsMatch[1], ",") {
			keys = append(keys, strings.TrimSpace(key))
		}
		return title, keys, nil
	}
	return "", nil, fmt.Errorf("GOLC_CATALOG_ROADMAP_PHASE_MISSING: ROADMAP.md has no '### Phase %d:' section", numeric)
}

// discoverPlanFiles finds every valid NN-MM-PLAN.md in the phase directory
// and returns the plan numbers sorted by parsed numeric plan ID. Files
// that look like plan files but violate the grammar — including a phase
// prefix that does not match the owning directory — fail loudly.
func discoverPlanFiles(root, phaseDirName, phaseNumber string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(root, ".planning", "phases", phaseDirName))
	if err != nil {
		return nil, fmt.Errorf("GOLC_CATALOG_SOURCE_UNREADABLE: .planning/phases/%s: %v", phaseDirName, err)
	}
	planNumbers := []string{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !planFileSuffixPattern.MatchString(name) {
			continue
		}
		match := planFileNamePattern.FindStringSubmatch(name)
		if match == nil {
			return nil, fmt.Errorf("GOLC_CATALOG_PLAN_FILENAME: %q does not match the NN-MM-PLAN.md grammar", name)
		}
		if match[1] != phaseNumber {
			return nil, fmt.Errorf("GOLC_CATALOG_PLAN_FILENAME: %q carries phase prefix %s inside phase directory %s", name, match[1], phaseDirName)
		}
		planNumbers = append(planNumbers, match[2])
	}
	sort.Slice(planNumbers, func(i, j int) bool {
		left, _ := strconv.Atoi(planNumbers[i])
		right, _ := strconv.Atoi(planNumbers[j])
		return left < right
	})
	return planNumbers, nil
}

// buildPlan adds one plan entity plus one task entity per executable
// (type="auto") task, identified by its 1-based position among all task
// elements in the plan's <tasks> block.
func buildPlan(built *Catalog, root, phaseDirName, phaseNumber, planNumber, phaseID string) error {
	fileName := phaseNumber + "-" + planNumber + "-PLAN.md"
	source := ".planning/phases/" + phaseDirName + "/" + fileName
	content, err := os.ReadFile(filepath.Join(root, ".planning", "phases", phaseDirName, fileName))
	if err != nil {
		return fmt.Errorf("GOLC_CATALOG_SOURCE_UNREADABLE: %s: %v", source, err)
	}
	text := strings.ReplaceAll(string(content), "\r\n", "\n")

	frontmatterPhase, frontmatterPlan, err := parsePlanFrontmatter(text, source)
	if err != nil {
		return err
	}
	if frontmatterPlan != planNumber {
		return fmt.Errorf("GOLC_CATALOG_PLAN_FRONTMATTER: %s declares plan %q but its filename encodes %q", source, frontmatterPlan, planNumber)
	}
	if frontmatterPhase != phaseDirName {
		return fmt.Errorf("GOLC_CATALOG_PLAN_FRONTMATTER: %s declares phase %q but lives in phase directory %q", source, frontmatterPhase, phaseDirName)
	}

	planID, err := PlanID(phaseNumber, planNumber)
	if err != nil {
		return err
	}
	if err := built.Add(Entity{
		ID:      planID,
		Kind:    KindPlan,
		Parent:  phaseID,
		Display: "Plan " + phaseNumber + "-" + planNumber,
		Source:  source,
	}); err != nil {
		return err
	}

	tasksStart := strings.Index(text, "<tasks>")
	tasksEnd := strings.Index(text, "</tasks>")
	if tasksStart < 0 || tasksEnd < 0 || tasksEnd < tasksStart {
		return fmt.Errorf("GOLC_CATALOG_PLAN_TASKS_MISSING: %s has no <tasks> block", source)
	}
	region := text[tasksStart:tasksEnd]

	tagIndexes := taskTagPattern.FindAllStringIndex(region, -1)
	for ordinal, tagIndex := range tagIndexes {
		tag := region[tagIndex[0]:tagIndex[1]]
		typeMatch := taskTypePattern.FindStringSubmatch(tag)
		if typeMatch == nil {
			return fmt.Errorf("GOLC_CATALOG_TASK_TYPE: %s task %d has no type attribute", source, ordinal+1)
		}
		if typeMatch[1] != "auto" {
			continue
		}
		segmentEnd := len(region)
		if ordinal+1 < len(tagIndexes) {
			segmentEnd = tagIndexes[ordinal+1][0]
		}
		display := ""
		if nameMatch := taskNamePattern.FindStringSubmatch(region[tagIndex[1]:segmentEnd]); nameMatch != nil {
			display = strings.TrimSpace(nameMatch[1])
		}
		taskID, err := TaskID(phaseNumber, planNumber, ordinal+1)
		if err != nil {
			return err
		}
		if err := built.Add(Entity{
			ID:      taskID,
			Kind:    KindTask,
			Parent:  planID,
			Display: display,
			Source:  source,
		}); err != nil {
			return err
		}
	}
	return nil
}

// parsePlanFrontmatter extracts the top-level phase and plan fields from a
// plan's leading YAML frontmatter block.
func parsePlanFrontmatter(text, source string) (phase, plan string, err error) {
	lines := strings.Split(text, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", "", fmt.Errorf("GOLC_CATALOG_PLAN_FRONTMATTER: %s does not start with a frontmatter block", source)
	}
	closed := false
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "---" {
			closed = true
			break
		}
		switch {
		case strings.HasPrefix(line, "phase:"):
			phase = strings.TrimSpace(strings.TrimPrefix(line, "phase:"))
		case strings.HasPrefix(line, "plan:"):
			plan = strings.TrimSpace(strings.TrimPrefix(line, "plan:"))
		}
	}
	if !closed {
		return "", "", fmt.Errorf("GOLC_CATALOG_PLAN_FRONTMATTER: %s frontmatter block is not closed", source)
	}
	if phase == "" || plan == "" {
		return "", "", fmt.Errorf("GOLC_CATALOG_PLAN_FRONTMATTER: %s frontmatter must declare both phase and plan", source)
	}
	return phase, plan, nil
}
