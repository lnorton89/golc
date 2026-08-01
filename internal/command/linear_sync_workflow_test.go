package command

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var _ = MustDeclareScope(ScopeRegistration{
	Scope:   "linear-sync-workflow",
	Summary: "Structural safety checks for .github/workflows/linear-sync.yml.",
})

// TestScopeLinearSyncWorkflow is the Go-native replacement for
// tests/acceptance/linear-transport.ps1 -Mode workflow's structural
// assertions (golc.ps1 removal Step 7): unlike that script's runtime PR
// guard check (redundant with internal/trace/apply's
// GOLC_APPLY_PR_BLOCKED coverage), no Go test previously read
// linear-sync.yml itself, unlike check.yml (internal/command/check_test.go)
// and cross-platform-mage.yml (cross_platform_ci_test.go).
func TestScopeLinearSyncWorkflow(t *testing.T) {
	root := commandParityRepositoryRoot(t)
	path := filepath.Join(root, ".github", "workflows", "linear-sync.yml")
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	text := strings.ReplaceAll(string(raw), "\r\n", "\n")

	for _, forbidden := range []string{
		"  pull_request:",
		"  push:",
		"  schedule:",
		"  repository_dispatch:",
	} {
		assert.NotContains(t, text, forbidden, "linear-sync.yml contains forbidden token %q", forbidden)
	}

	for _, required := range []string{
		"  workflow_dispatch:",
		"plan_file:",
		"plan_id:",
		"confirm_apply:",
		"environment: linear-production",
		"if: ${{ github.event.inputs.confirm_apply == 'CONFIRM' }}",
		"finally {",
		`GOLC_BOOTSTRAP_INCLUDE_LINEAR_SYNC: "1"`,
		"run: mage Bootstrap",
		"scripts/ci/install-pinned-mage.ps1",
		`.tools\installs\golc_project\windows-amd64\bin\golc-project.exe`,
	} {
		assert.Contains(t, text, required, "linear-sync.yml missing required token %q", required)
	}

	// No executable step may invoke the deleted golc.ps1 shim (this scan is
	// scoped to "run:" lines specifically, unlike a blind full-text
	// forbidden-token check, since this file's own header comments
	// legitimately discuss golc.ps1 as migration history).
	for lineIndex, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		assert.False(t, strings.HasPrefix(trimmed, "run:") && strings.Contains(trimmed, "golc.ps1"), "linear-sync.yml line %d invokes the deleted golc.ps1: %q", lineIndex+1, trimmed)
	}

	// No step may echo, log, or embed a secret value directly -- only the
	// two documented ${{ secrets.* }} references that seed the ephemeral
	// .env file are permitted.
	assert.Equal(t, 2, strings.Count(text, "secrets.LINEAR_API_KEY"), "expected exactly 2 references to secrets.LINEAR_API_KEY (one per job), got %d", strings.Count(text, "secrets.LINEAR_API_KEY"))
	assert.Equal(t, 2, strings.Count(text, "secrets.LINEAR_TEAM_ID"), "expected exactly 2 references to secrets.LINEAR_TEAM_ID (one per job), got %d", strings.Count(text, "secrets.LINEAR_TEAM_ID"))
}
