// status.go serves golc_project_status: the current GSD planning position
// (milestone, phase, execution status, progress counts) parsed from
// .planning/STATE.md. Scalar position metadata comes from YAML frontmatter,
// while the Current Position section is the authority for live activity.
//
// The frontmatter is read with a small purpose-built scanner rather than
// a general YAML decoder: STATE.md's shape is a flat set of "key: value"
// scalars plus one nested "progress:" block, and a strict YAML 1.1
// decoder coerces bare dates like "2026-07-23" into a timestamp type that
// then fails to bind to a string field — exactly the kind of surprise
// this tool exists to avoid inflicting on a caller.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type projectStatusInput struct{}

type projectStatusProgress struct {
	TotalPhases     int `json:"total_phases"`
	CompletedPhases int `json:"completed_phases"`
	TotalPlans      int `json:"total_plans"`
	CompletedPlans  int `json:"completed_plans"`
}

type projectStatusActivity struct {
	Date        string `json:"date"`
	Description string `json:"description"`
}

type projectStatusActivityDrift struct {
	Detected        bool                  `json:"detected"`
	Frontmatter     projectStatusActivity `json:"frontmatter"`
	CurrentPosition projectStatusActivity `json:"current_position"`
}

type projectStatusOutput struct {
	Milestone        string                     `json:"milestone"`
	MilestoneName    string                     `json:"milestone_name"`
	CurrentPhase     string                     `json:"current_phase"`
	CurrentPhaseName string                     `json:"current_phase_name"`
	Status           string                     `json:"status"`
	StoppedAt        string                     `json:"stopped_at"`
	LastUpdated      string                     `json:"last_updated"`
	LastActivity     string                     `json:"last_activity"`
	LastActivityDesc string                     `json:"last_activity_desc"`
	ActivitySource   string                     `json:"activity_source"`
	ActivityDrift    projectStatusActivityDrift `json:"activity_drift"`
	Progress         projectStatusProgress      `json:"progress"`
}

func handleProjectStatus(_ context.Context, _ *mcp.CallToolRequest, _ projectStatusInput) (*mcp.CallToolResult, projectStatusOutput, error) {
	root, err := resolveRepoRoot()
	if err != nil {
		return toolError[projectStatusOutput](err)
	}

	statePath := filepath.Join(root, ".planning", "STATE.md")
	data, err := os.ReadFile(statePath)
	if err != nil {
		return toolError[projectStatusOutput](fmt.Errorf("read .planning/STATE.md: %w (is this a GOLC checkout with GSD planning artifacts?)", err))
	}

	frontmatter, err := extractYAMLFrontmatter(string(data))
	if err != nil {
		return toolError[projectStatusOutput](fmt.Errorf(".planning/STATE.md: %w", err))
	}

	topText, progressText := splitFrontmatterBlock(frontmatter, "progress")
	top := parseFlatScalarMapping(topText)
	progress := parseFlatScalarMapping(progressText)
	currentActivity, err := parseCurrentPositionActivity(string(data))
	if err != nil {
		return toolError[projectStatusOutput](fmt.Errorf(".planning/STATE.md: current position activity: %w", err))
	}
	frontmatterActivity := projectStatusActivity{
		Date:        top["last_activity"],
		Description: top["last_activity_desc"],
	}

	out := projectStatusOutput{
		Milestone:        top["milestone"],
		MilestoneName:    top["milestone_name"],
		CurrentPhase:     top["current_phase"],
		CurrentPhaseName: top["current_phase_name"],
		Status:           top["status"],
		StoppedAt:        top["stopped_at"],
		LastUpdated:      top["last_updated"],
		LastActivity:     currentActivity.Date,
		LastActivityDesc: currentActivity.Description,
		ActivitySource:   "current_position_body",
		ActivityDrift: projectStatusActivityDrift{
			Detected:        frontmatterActivity != currentActivity,
			Frontmatter:     frontmatterActivity,
			CurrentPosition: currentActivity,
		},
		Progress: projectStatusProgress{
			TotalPhases:     atoiOrZero(progress["total_phases"]),
			CompletedPhases: atoiOrZero(progress["completed_phases"]),
			TotalPlans:      atoiOrZero(progress["total_plans"]),
			CompletedPlans:  atoiOrZero(progress["completed_plans"]),
		},
	}
	return nil, out, nil
}

func parseCurrentPositionActivity(content string) (projectStatusActivity, error) {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(content, "\n")

	var sectionStart int
	sectionCount := 0
	for i, line := range lines {
		if line == "## Current Position" {
			sectionStart = i + 1
			sectionCount++
		}
	}
	if sectionCount != 1 {
		return projectStatusActivity{}, fmt.Errorf("expected exactly one %q section, found %d", "## Current Position", sectionCount)
	}

	sectionEnd := len(lines)
	for i := sectionStart; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "## ") {
			sectionEnd = i
			break
		}
	}

	var activityLines []string
	for _, line := range lines[sectionStart:sectionEnd] {
		if strings.HasPrefix(line, "Last activity:") {
			activityLines = append(activityLines, line)
		}
	}
	if len(activityLines) != 1 {
		return projectStatusActivity{}, fmt.Errorf("expected exactly one Last activity record, found %d", len(activityLines))
	}

	const prefix = "Last activity: "
	record := activityLines[0]
	if !strings.HasPrefix(record, prefix) {
		return projectStatusActivity{}, fmt.Errorf("record must start with %q", prefix)
	}
	date, description, ok := strings.Cut(strings.TrimPrefix(record, prefix), " — ")
	if !ok {
		return projectStatusActivity{}, fmt.Errorf("record must use %q between date and description", " — ")
	}
	if parsed, err := time.Parse("2006-01-02", date); err != nil || parsed.Format("2006-01-02") != date {
		return projectStatusActivity{}, fmt.Errorf("date %q must be a valid YYYY-MM-DD value", date)
	}
	description = strings.TrimSpace(description)
	if description == "" {
		return projectStatusActivity{}, fmt.Errorf("description must not be empty")
	}

	return projectStatusActivity{Date: date, Description: description}, nil
}

// extractYAMLFrontmatter returns the content between the first two "---"
// delimiter lines a GSD state/plan document opens with.
func extractYAMLFrontmatter(content string) (string, error) {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", fmt.Errorf("no YAML frontmatter: expected file to start with a \"---\" line")
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return strings.Join(lines[1:i], "\n"), nil
		}
	}
	return "", fmt.Errorf("no closing \"---\" delimiter found for YAML frontmatter")
}

// splitFrontmatterBlock pulls the lines nested under one top-level
// "blockKey:" mapping (indented by whitespace) out of the frontmatter,
// returning the remaining top-level lines separately.
func splitFrontmatterBlock(frontmatter, blockKey string) (topLevel string, block string) {
	var top, nested []string
	inBlock := false
	for _, line := range strings.Split(frontmatter, "\n") {
		if !inBlock && strings.HasPrefix(strings.TrimSpace(line), blockKey+":") {
			inBlock = true
			continue
		}
		if inBlock {
			if line != "" && (strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")) {
				nested = append(nested, strings.TrimLeft(line, " \t"))
				continue
			}
			inBlock = false
		}
		top = append(top, line)
	}
	return strings.Join(top, "\n"), strings.Join(nested, "\n")
}

// parseFlatScalarMapping reads "key: value" lines (no nesting) into a
// map, stripping a single layer of matching quotes from each value.
func parseFlatScalarMapping(text string) map[string]string {
	values := map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		key, value, ok := strings.Cut(trimmed, ":")
		if !ok {
			continue
		}
		values[strings.TrimSpace(key)] = unquoteYAMLScalar(strings.TrimSpace(value))
	}
	return values
}

func unquoteYAMLScalar(value string) string {
	if len(value) >= 2 {
		first, last := value[0], value[len(value)-1]
		if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
			return value[1 : len(value)-1]
		}
	}
	return value
}

func atoiOrZero(value string) int {
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return n
}
