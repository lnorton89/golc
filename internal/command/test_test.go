package command

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestParseTestArgsAcceptedForms exercises every accepted "test" invocation
// shape documented on parseTestArgs: bare (full suite), "--quick" alone,
// and "--quick --scope <name>" in both "--scope <name>" and
// "--scope=<name>" spellings, in either flag order.
func TestParseTestArgsAcceptedForms(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want testInvocation
	}{
		{
			name: "no arguments means the full suite",
			args: nil,
			want: testInvocation{mode: "full"},
		},
		{
			name: "empty argument slice means the full suite",
			args: []string{},
			want: testInvocation{mode: "full"},
		},
		{
			name: "--quick alone selects the quick mode",
			args: []string{"--quick"},
			want: testInvocation{mode: "quick"},
		},
		{
			name: "--quick given twice is still quick mode",
			args: []string{"--quick", "--quick"},
			want: testInvocation{mode: "quick"},
		},
		{
			name: "--quick --scope <name> selects quick-scope",
			args: []string{"--quick", "--scope", "config-strict"},
			want: testInvocation{mode: "quick-scope", scope: "config-strict"},
		},
		{
			name: "--quick --scope=<name> selects quick-scope",
			args: []string{"--quick", "--scope=config-strict"},
			want: testInvocation{mode: "quick-scope", scope: "config-strict"},
		},
		{
			name: "--scope before --quick still selects quick-scope",
			args: []string{"--scope", "config-strict", "--quick"},
			want: testInvocation{mode: "quick-scope", scope: "config-strict"},
		},
		{
			name: "--scope=<name> before --quick still selects quick-scope",
			args: []string{"--scope=config-strict", "--quick"},
			want: testInvocation{mode: "quick-scope", scope: "config-strict"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTestArgs(tt.args)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

// TestParseTestArgsRejectedForms exercises every invalid "test" invocation
// parseTestArgs must reject: --scope without --quick, a missing/empty scope
// value, --scope given twice, and any unsupported argument.
func TestParseTestArgsRejectedForms(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantError string
	}{
		{
			name:      "--scope without --quick is rejected",
			args:      []string{"--scope", "config-strict"},
			wantError: "GOLC_TEST_USAGE: --scope requires --quick; usage: test [--quick [--scope <scope-name>]]",
		},
		{
			name:      "--scope=<name> without --quick is rejected",
			args:      []string{"--scope=config-strict"},
			wantError: "GOLC_TEST_USAGE: --scope requires --quick; usage: test [--quick [--scope <scope-name>]]",
		},
		{
			name:      "--scope at end of arguments with no value is rejected",
			args:      []string{"--quick", "--scope"},
			wantError: "GOLC_TEST_USAGE: --scope requires a scope name",
		},
		{
			name:      "--scope with an empty value is rejected",
			args:      []string{"--quick", "--scope", ""},
			wantError: "GOLC_TEST_USAGE: --scope requires a scope name",
		},
		{
			name:      "--scope= with an empty value is rejected",
			args:      []string{"--quick", "--scope="},
			wantError: "GOLC_TEST_USAGE: --scope requires a scope name",
		},
		{
			name:      "--scope given twice is rejected",
			args:      []string{"--quick", "--scope", "config-strict", "--scope", "config-local"},
			wantError: "GOLC_TEST_USAGE: --scope may be given only once",
		},
		{
			name:      "--scope= given twice is rejected",
			args:      []string{"--quick", "--scope=config-strict", "--scope=config-local"},
			wantError: "GOLC_TEST_USAGE: --scope may be given only once",
		},
		{
			name:      "an unsupported argument is rejected",
			args:      []string{"--bogus"},
			wantError: `GOLC_TEST_USAGE: unsupported argument "--bogus"; usage: test [--quick [--scope <scope-name>]]`,
		},
		{
			name:      "an unsupported argument after --quick is rejected",
			args:      []string{"--quick", "--bogus"},
			wantError: `GOLC_TEST_USAGE: unsupported argument "--bogus"; usage: test [--quick [--scope <scope-name>]]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTestArgs(tt.args)
			require.Error(t, err)
			require.EqualError(t, err, tt.wantError)
			require.Equal(t, testInvocation{}, got)
		})
	}
}

// TestScopeTestMarker confirms scopeTestMarker's exact PascalCase transform
// (documented on the function: config-local -> TestScopeConfigLocal)
// against real scope names already used elsewhere in this codebase, whose
// TestScope{PascalName} marker functions already exist (e.g.
// internal/projectconfig/strict_test.go's TestScopeConfigStrict,
// internal/projectconfig/local_test.go's TestScopeConfigLocal, and this
// package's own build_test.go's TestScopeBuildArgs for scope
// "build-args").
func TestScopeTestMarker(t *testing.T) {
	tests := []struct {
		scopeName string
		want      string
	}{
		{scopeName: "config-local", want: "TestScopeConfigLocal"},
		{scopeName: "config-strict", want: "TestScopeConfigStrict"},
		{scopeName: "build-args", want: "TestScopeBuildArgs"},
		{scopeName: "pool", want: "TestScopePool"},
		{scopeName: "config", want: "TestScopeConfig"},
		{scopeName: "linear-sync-workflow", want: "TestScopeLinearSyncWorkflow"},
	}

	for _, tt := range tests {
		t.Run(tt.scopeName, func(t *testing.T) {
			require.Equal(t, tt.want, scopeTestMarker(tt.scopeName))
		})
	}
}
