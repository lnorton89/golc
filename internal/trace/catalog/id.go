// id.go defines the durable local ID grammar (CONTEXT D-11/D-14). Every
// ID derives from durable structural metadata — slugs pinned in
// linear-map.json, two-digit phase and plan numbers, requirement keys,
// and XML task positions — never from display titles or issue keys, so
// renames and Linear identifiers can never change local identity.
package catalog

import (
	"fmt"
	"regexp"
	"strconv"
)

// The exact accepted shape for each ID kind.
var (
	projectIDPattern     = regexp.MustCompile(`^project:[a-z0-9]+(?:-[a-z0-9]+)*$`)
	milestoneIDPattern   = regexp.MustCompile(`^milestone:v[0-9]+$`)
	phaseIDPattern       = regexp.MustCompile(`^phase:([0-9]{2})$`)
	requirementIDPattern = regexp.MustCompile(`^req:([A-Z]{2,10}-[0-9]{2})$`)
	planIDPattern        = regexp.MustCompile(`^plan:([0-9]{2})-([0-9]{2})$`)
	taskIDPattern        = regexp.MustCompile(`^task:([0-9]{2})-([0-9]{2})\.([1-9][0-9]*)$`)

	phaseNumberPattern    = regexp.MustCompile(`^[0-9]{2}$`)
	requirementKeyPattern = regexp.MustCompile(`^[A-Z]{2,10}-[0-9]{2}$`)
)

// ParsedID is the structural decomposition of one durable local ID.
type ParsedID struct {
	Kind Kind
	// Slug is the project slug for project IDs.
	Slug string
	// Version is the milestone version number for milestone IDs.
	Version int
	// Phase is the two-digit phase number for phase, plan, and task IDs.
	Phase string
	// Plan is the two-digit plan number for plan and task IDs.
	Plan string
	// Task is the 1-based XML task position for task IDs.
	Task int
	// Requirement is the requirement key (e.g. "CONF-01") for req IDs.
	Requirement string
}

// ParseID validates one ID against the grammar and decomposes it.
func ParseID(id string) (ParsedID, error) {
	switch {
	case projectIDPattern.MatchString(id):
		return ParsedID{Kind: KindProject, Slug: id[len("project:"):]}, nil
	case milestoneIDPattern.MatchString(id):
		version, err := strconv.Atoi(id[len("milestone:v"):])
		if err != nil {
			return ParsedID{}, fmt.Errorf("GOLC_CATALOG_ID_INVALID: %q: %v", id, err)
		}
		return ParsedID{Kind: KindMilestone, Version: version}, nil
	case phaseIDPattern.MatchString(id):
		match := phaseIDPattern.FindStringSubmatch(id)
		return ParsedID{Kind: KindPhase, Phase: match[1]}, nil
	case requirementIDPattern.MatchString(id):
		match := requirementIDPattern.FindStringSubmatch(id)
		return ParsedID{Kind: KindRequirement, Requirement: match[1]}, nil
	case planIDPattern.MatchString(id):
		match := planIDPattern.FindStringSubmatch(id)
		return ParsedID{Kind: KindPlan, Phase: match[1], Plan: match[2]}, nil
	case taskIDPattern.MatchString(id):
		match := taskIDPattern.FindStringSubmatch(id)
		position, err := strconv.Atoi(match[3])
		if err != nil {
			return ParsedID{}, fmt.Errorf("GOLC_CATALOG_ID_INVALID: %q: %v", id, err)
		}
		return ParsedID{Kind: KindTask, Phase: match[1], Plan: match[2], Task: position}, nil
	}
	return ParsedID{}, fmt.Errorf("GOLC_CATALOG_ID_INVALID: %q does not match any durable id grammar", id)
}

// PhaseID derives the durable phase ID from a two-digit phase number.
func PhaseID(phaseNumber string) (string, error) {
	if !phaseNumberPattern.MatchString(phaseNumber) {
		return "", fmt.Errorf("GOLC_CATALOG_ID_INVALID: phase number %q is not two digits", phaseNumber)
	}
	return "phase:" + phaseNumber, nil
}

// PlanID derives the durable plan ID from two-digit phase and plan numbers.
func PlanID(phaseNumber, planNumber string) (string, error) {
	if !phaseNumberPattern.MatchString(phaseNumber) {
		return "", fmt.Errorf("GOLC_CATALOG_ID_INVALID: phase number %q is not two digits", phaseNumber)
	}
	if !phaseNumberPattern.MatchString(planNumber) {
		return "", fmt.Errorf("GOLC_CATALOG_ID_INVALID: plan number %q is not two digits", planNumber)
	}
	return "plan:" + phaseNumber + "-" + planNumber, nil
}

// TaskID derives the durable executable-task ID from the two-digit phase
// and plan numbers plus the 1-based XML task position within the plan's
// <tasks> block. The task's display name plays no role (CONTEXT D-14).
func TaskID(phaseNumber, planNumber string, position int) (string, error) {
	planID, err := PlanID(phaseNumber, planNumber)
	if err != nil {
		return "", err
	}
	if position < 1 {
		return "", fmt.Errorf("GOLC_CATALOG_ID_INVALID: task position %d must be at least 1", position)
	}
	return "task:" + planID[len("plan:"):] + "." + strconv.Itoa(position), nil
}

// RequirementID derives the durable requirement ID from a requirement key
// such as "CONF-01". Issue-key shapes such as "GOLC-123" do not match the
// grammar, so human tracker keys can never become local identity.
func RequirementID(key string) (string, error) {
	if !requirementKeyPattern.MatchString(key) {
		return "", fmt.Errorf("GOLC_CATALOG_ID_INVALID: requirement key %q does not match the KEY-NN grammar", key)
	}
	return "req:" + key, nil
}
