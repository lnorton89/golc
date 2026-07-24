package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const projectStatusFrontmatter = `---
gsd_state_version: 1.0
milestone: v1.0
milestone_name: milestone
current_phase: 06
current_phase_name: Wails Authoring and Operator Surface
status: executing
stopped_at: Phase 6 UI-SPEC approved
last_updated: "2026-07-23T22:16:28.867Z"
last_activity: 2026-07-23
last_activity_desc: "Completed quick task task-a: stale frontmatter"
progress:
  total_phases: 7
  completed_phases: 5
  total_plans: 71
  completed_plans: 67
---
`

func TestProjectStatusReportsCurrentPositionActivityAndDrift(t *testing.T) {
	content := projectStatusFrontmatter + `
# Project State

## Current Position

Phase: 06 (Wails Authoring and Operator Surface) — EXECUTING
Last activity: 2026-07-24 — Completed quick task task-b: live body

## Performance Metrics

Last activity: 1999-01-01 — must not be scanned
`

	got := callProjectStatus(t, content)
	assertJSONValue(t, got, "last_activity", "2026-07-24")
	assertJSONValue(t, got, "last_activity_desc", "Completed quick task task-b: live body")
	assertJSONValue(t, got, "activity_source", "current_position_body")
	assertJSONValue(t, got, "activity_drift.detected", true)
	assertJSONValue(t, got, "activity_drift.frontmatter.date", "2026-07-23")
	assertJSONValue(t, got, "activity_drift.frontmatter.description", "Completed quick task task-a: stale frontmatter")
	assertJSONValue(t, got, "activity_drift.current_position.date", "2026-07-24")
	assertJSONValue(t, got, "activity_drift.current_position.description", "Completed quick task task-b: live body")

	assertJSONValue(t, got, "milestone", "v1.0")
	assertJSONValue(t, got, "milestone_name", "milestone")
	assertJSONValue(t, got, "current_phase", "06")
	assertJSONValue(t, got, "current_phase_name", "Wails Authoring and Operator Surface")
	assertJSONValue(t, got, "status", "executing")
	assertJSONValue(t, got, "stopped_at", "Phase 6 UI-SPEC approved")
	assertJSONValue(t, got, "last_updated", "2026-07-23T22:16:28.867Z")
	assertJSONValue(t, got, "progress.total_phases", float64(7))
	assertJSONValue(t, got, "progress.completed_phases", float64(5))
	assertJSONValue(t, got, "progress.total_plans", float64(71))
	assertJSONValue(t, got, "progress.completed_plans", float64(67))
}

func TestProjectStatusMatchingActivityHasNoDrift(t *testing.T) {
	content := strings.ReplaceAll(
		projectStatusFrontmatter+`
## Current Position

Last activity: 2026-07-23 — Completed quick task task-a: stale frontmatter
`,
		"stale frontmatter",
		"same activity",
	)

	got := callProjectStatus(t, content)
	assertJSONValue(t, got, "last_activity", "2026-07-23")
	assertJSONValue(t, got, "last_activity_desc", "Completed quick task task-a: same activity")
	assertJSONValue(t, got, "activity_source", "current_position_body")
	assertJSONValue(t, got, "activity_drift.detected", false)
}

func TestProjectStatusLineEndingsProduceEquivalentActivity(t *testing.T) {
	lf := projectStatusFrontmatter + `
## Current Position

Last activity: 2026-07-24 — line ending invariant

## Next Section
`
	crlf := strings.ReplaceAll(lf, "\n", "\r\n")

	gotLF := callProjectStatus(t, lf)
	gotCRLF := callProjectStatus(t, crlf)

	for _, path := range []string{
		"last_activity",
		"last_activity_desc",
		"activity_source",
		"activity_drift",
	} {
		if left, right := jsonPath(t, gotLF, path), jsonPath(t, gotCRLF, path); !reflect.DeepEqual(left, right) {
			t.Fatalf("%s differs for LF and CRLF:\nLF: %#v\nCRLF: %#v", path, left, right)
		}
	}
}

func TestProjectStatusRejectsInvalidCurrentPositionActivity(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "missing section",
			body: "## Performance Metrics\n\nLast activity: 2026-07-24 — outside authority\n",
		},
		{
			name: "zero activity lines",
			body: "## Current Position\n\nStatus: executing\n",
		},
		{
			name: "multiple activity lines",
			body: "## Current Position\n\nLast activity: 2026-07-24 — one\nLast activity: 2026-07-24 — two\n",
		},
		{
			name: "malformed date",
			body: "## Current Position\n\nLast activity: 2026-7-24 — description\n",
		},
		{
			name: "invalid date",
			body: "## Current Position\n\nLast activity: 2026-02-30 — description\n",
		},
		{
			name: "malformed separator",
			body: "## Current Position\n\nLast activity: 2026-07-24 - description\n",
		},
		{
			name: "empty description",
			body: "## Current Position\n\nLast activity: 2026-07-24 —    \n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := callProjectStatusError(t, projectStatusFrontmatter+tt.body)
			if err == nil {
				t.Fatal("expected invalid Current Position activity to fail")
			}
			const diagnostic = ".planning/STATE.md: current position activity:"
			if !strings.Contains(err.Error(), diagnostic) {
				t.Fatalf("error %q does not contain stable diagnostic %q", err, diagnostic)
			}
		})
	}
}

func callProjectStatus(t *testing.T, content string) map[string]any {
	t.Helper()
	out, err := callProjectStatusError(t, content)
	if err != nil {
		t.Fatalf("handleProjectStatus() error = %v", err)
	}
	data, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal project status: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal project status: %v", err)
	}
	return decoded
}

func callProjectStatusError(t *testing.T, content string) (projectStatusOutput, error) {
	t.Helper()
	root := t.TempDir()
	stateDir := filepath.Join(root, ".planning")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("mkdir .planning: %v", err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, "STATE.md"), []byte(content), 0o600); err != nil {
		t.Fatalf("write STATE.md: %v", err)
	}
	t.Setenv(repoRootEnvName, root)

	_, out, err := handleProjectStatus(context.Background(), nil, projectStatusInput{})
	return out, err
}

func assertJSONValue(t *testing.T, root map[string]any, path string, want any) {
	t.Helper()
	got := jsonPath(t, root, path)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s = %#v, want %#v", path, got, want)
	}
}

func jsonPath(t *testing.T, root map[string]any, path string) any {
	t.Helper()
	var value any = root
	for _, part := range strings.Split(path, ".") {
		object, ok := value.(map[string]any)
		if !ok {
			t.Fatalf("%s: parent of %q is %T, want object", path, part, value)
		}
		value, ok = object[part]
		if !ok {
			t.Fatalf("%s: missing %q", path, part)
		}
	}
	return value
}
