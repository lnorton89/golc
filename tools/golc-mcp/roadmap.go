// roadmap.go serves the two golc_*_phase* tools over .planning/ROADMAP.md:
// a lightweight structured list of every phase's checked/unchecked status,
// and a verbatim fetch of one phase's full detail section (goal, mode,
// dependencies, requirements, plan waves) for a caller that needs the
// prose rather than just the status.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const roadmapRelPath = ".planning/ROADMAP.md"

// phaseBulletPattern matches one "## Phases" list entry, e.g.:
//
//   - [x] **Phase 1: Offline Foundation and Delivery Traceability** - Contributors can ... (completed 2026-07-21)
//   - [ ] **Phase 6: Wails Authoring and Operator Surface** - Users can ...
var phaseBulletPattern = regexp.MustCompile(`^- \[( |x)\] \*\*Phase (\d+): ([^*]+)\*\* - (.+)$`)

// completedDatePattern pulls the trailing "(completed YYYY-MM-DD)" note a
// finished phase bullet carries, when present.
var completedDatePattern = regexp.MustCompile(`\(completed (\d{4}-\d{2}-\d{2})\)\s*$`)

// planCountPattern pulls a phase detail section's "**Plans:** N/M plans
// ..." progress line, when the phase has moved past "TBD" planning.
var planCountPattern = regexp.MustCompile(`\*\*Plans:\*\*\s*(\d+)/(\d+)\s*plans`)

// phaseHeadingPattern matches one phase detail section heading, e.g.
// "### Phase 6: Wails Authoring and Operator Surface".
var phaseHeadingPattern = regexp.MustCompile(`^### Phase (\d+):`)

type listPhasesInput struct{}

type phaseSummary struct {
	Number         int    `json:"number"`
	Title          string `json:"title"`
	Complete       bool   `json:"complete"`
	Summary        string `json:"summary"`
	CompletedDate  string `json:"completed_date,omitempty"`
	PlansCompleted int    `json:"plans_completed,omitempty"`
	PlansTotal     int    `json:"plans_total,omitempty"`
}

type listPhasesOutput struct {
	Phases []phaseSummary `json:"phases"`
}

func handleListPhases(_ context.Context, _ *mcp.CallToolRequest, _ listPhasesInput) (*mcp.CallToolResult, listPhasesOutput, error) {
	root, err := resolveRepoRoot()
	if err != nil {
		return toolError[listPhasesOutput](err)
	}
	text, err := readRoadmap(root)
	if err != nil {
		return toolError[listPhasesOutput](err)
	}

	planCounts := planCountsByPhase(text)

	var phases []phaseSummary
	for _, line := range strings.Split(text, "\n") {
		match := phaseBulletPattern.FindStringSubmatch(strings.TrimRight(line, "\r"))
		if match == nil {
			continue
		}
		number, convErr := strconv.Atoi(match[2])
		if convErr != nil {
			continue
		}
		summary := strings.TrimSpace(match[4])
		completedDate := ""
		if dateMatch := completedDatePattern.FindStringSubmatch(summary); dateMatch != nil {
			completedDate = dateMatch[1]
			summary = strings.TrimSpace(completedDatePattern.ReplaceAllString(summary, ""))
		}
		phase := phaseSummary{
			Number:        number,
			Title:         strings.TrimSpace(match[3]),
			Complete:      match[1] == "x",
			Summary:       summary,
			CompletedDate: completedDate,
		}
		if counts, ok := planCounts[number]; ok {
			phase.PlansCompleted = counts[0]
			phase.PlansTotal = counts[1]
		}
		phases = append(phases, phase)
	}

	if len(phases) == 0 {
		return toolError[listPhasesOutput](fmt.Errorf("%s: no \"- [ ] **Phase N: ...**\" bullets found under \"## Phases\"", roadmapRelPath))
	}
	return nil, listPhasesOutput{Phases: phases}, nil
}

// planCountsByPhase scans every phase detail section for its "**Plans:**
// N/M plans ..." line, keyed by phase number.
func planCountsByPhase(roadmapText string) map[int][2]int {
	counts := map[int][2]int{}
	currentPhase := -1
	for _, line := range strings.Split(roadmapText, "\n") {
		if heading := phaseHeadingPattern.FindStringSubmatch(line); heading != nil {
			currentPhase, _ = strconv.Atoi(heading[1])
			continue
		}
		if currentPhase == -1 {
			continue
		}
		if match := planCountPattern.FindStringSubmatch(line); match != nil {
			completed, _ := strconv.Atoi(match[1])
			total, _ := strconv.Atoi(match[2])
			counts[currentPhase] = [2]int{completed, total}
		}
	}
	return counts
}

type getPhaseDetailInput struct {
	Phase int `json:"phase" jsonschema:"the phase number to fetch, e.g. 6 for Phase 6"`
}

type getPhaseDetailOutput struct {
	Phase   int    `json:"phase"`
	Heading string `json:"heading"`
	Detail  string `json:"detail"`
}

func handleGetPhaseDetail(_ context.Context, _ *mcp.CallToolRequest, input getPhaseDetailInput) (*mcp.CallToolResult, getPhaseDetailOutput, error) {
	if input.Phase <= 0 {
		return toolError[getPhaseDetailOutput](fmt.Errorf("phase must be a positive integer, got %d", input.Phase))
	}
	root, err := resolveRepoRoot()
	if err != nil {
		return toolError[getPhaseDetailOutput](err)
	}
	text, err := readRoadmap(root)
	if err != nil {
		return toolError[getPhaseDetailOutput](err)
	}

	lines := strings.Split(text, "\n")
	start := -1
	heading := ""
	for i, line := range lines {
		if match := phaseHeadingPattern.FindStringSubmatch(line); match != nil {
			number, _ := strconv.Atoi(match[1])
			if number == input.Phase {
				start = i
				heading = strings.TrimSpace(strings.TrimPrefix(line, "###"))
				break
			}
		}
	}
	if start == -1 {
		return toolError[getPhaseDetailOutput](fmt.Errorf("no \"### Phase %d:\" section found in %s", input.Phase, roadmapRelPath))
	}

	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if phaseHeadingPattern.MatchString(lines[i]) || strings.HasPrefix(lines[i], "## ") {
			end = i
			break
		}
	}

	return nil, getPhaseDetailOutput{
		Phase:   input.Phase,
		Heading: heading,
		Detail:  strings.TrimSpace(strings.Join(lines[start:end], "\n")),
	}, nil
}

func readRoadmap(root string) (string, error) {
	path := filepath.Join(root, filepath.FromSlash(roadmapRelPath))
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", roadmapRelPath, err)
	}
	return string(data), nil
}
