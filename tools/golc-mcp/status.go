// status.go serves golc_project_status: the current GSD planning position
// (milestone, phase, execution status, progress counts) parsed from the
// YAML frontmatter block .planning/STATE.md's GSD tooling already
// maintains as its single source of truth for "where are we right now."
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

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type projectStatusInput struct{}

type projectStatusProgress struct {
	TotalPhases     int `json:"total_phases"`
	CompletedPhases int `json:"completed_phases"`
	TotalPlans      int `json:"total_plans"`
	CompletedPlans  int `json:"completed_plans"`
}

type projectStatusOutput struct {
	Milestone        string                `json:"milestone"`
	MilestoneName    string                `json:"milestone_name"`
	CurrentPhase     string                `json:"current_phase"`
	CurrentPhaseName string                `json:"current_phase_name"`
	Status           string                `json:"status"`
	StoppedAt        string                `json:"stopped_at"`
	LastUpdated      string                `json:"last_updated"`
	LastActivity     string                `json:"last_activity"`
	LastActivityDesc string                `json:"last_activity_desc"`
	Progress         projectStatusProgress `json:"progress"`
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

	out := projectStatusOutput{
		Milestone:        top["milestone"],
		MilestoneName:    top["milestone_name"],
		CurrentPhase:     top["current_phase"],
		CurrentPhaseName: top["current_phase_name"],
		Status:           top["status"],
		StoppedAt:        top["stopped_at"],
		LastUpdated:      top["last_updated"],
		LastActivity:     top["last_activity"],
		LastActivityDesc: top["last_activity_desc"],
		Progress: projectStatusProgress{
			TotalPhases:     atoiOrZero(progress["total_phases"]),
			CompletedPhases: atoiOrZero(progress["completed_phases"]),
			TotalPlans:      atoiOrZero(progress["total_plans"]),
			CompletedPlans:  atoiOrZero(progress["completed_plans"]),
		},
	}
	return nil, out, nil
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
