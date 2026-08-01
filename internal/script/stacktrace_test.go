// stacktrace_test.go covers internal/script/stacktrace.go (08-09-PLAN.md
// Task 1): table-driven coverage over real captured Deno trace text
// shapes, spanning every <behavior> bullet -- a multi-frame trace, an
// in-shim frame, and a trace containing a temp-directory path that must
// never leak into a returned StackFrame.
package script

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestParseStackTrace is the table-driven proof of every stacktrace.go
// <behavior> bullet.
func TestParseStackTrace(t *testing.T) {
	const scriptName = "MyAutomation"
	const tempPath = "file:///C:/Users/op/AppData/Local/Temp/golc-script-run-1234/0199abcd-ef01-7000-8000-000000000001.ts"

	tests := []struct {
		name          string
		raw           string
		shimLineCount int
		want          []StackFrame
	}{
		{
			name:          "multi-frame trace with a named and an anonymous frame",
			shimLineCount: 10,
			raw: "Uncaught Error: boom\n" +
				"    at Object.doThing (" + tempPath + ":15:7)\n" +
				"    at " + tempPath + ":22:1\n",
			want: []StackFrame{
				{Function: "Object.doThing", File: scriptName, Line: 5, Column: 7},
				{Function: "<anonymous>", File: scriptName, Line: 12, Column: 1},
			},
		},
		{
			name:          "a frame inside the injected shim carries the shim marker and never a negative line",
			shimLineCount: 10,
			raw:           "    at shimHelper (" + tempPath + ":4:2)\n",
			want: []StackFrame{
				{Function: "shimHelper: " + shimErrorMarker, File: scriptName, Line: 0, Column: 2},
			},
		},
		{
			name:          "a frame landing exactly on the shim/user boundary line is still in-shim",
			shimLineCount: 10,
			raw:           "    at boundary (" + tempPath + ":10:1)\n",
			want: []StackFrame{
				{Function: "boundary: " + shimErrorMarker, File: scriptName, Line: 0, Column: 1},
			},
		},
		{
			name:          "a non-frame message line is skipped, not reported as a phantom frame",
			shimLineCount: 10,
			raw:           "TypeError: Cannot read properties of undefined (reading 'x')\n",
			want:          nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseStackTrace(tt.raw, tt.shimLineCount, scriptName)
			require.Len(t, got, len(tt.want), "parseStackTrace() = %+v, want %+v", got, tt.want)
			for i := range tt.want {
				require.Equal(t, tt.want[i], got[i], "parseStackTrace()[%d] = %+v, want %+v", i, got[i], tt.want[i])
			}
		})
	}
}

// TestParseStackTraceNeverLeaksTempPath asserts, across every frame of a
// realistic multi-frame trace containing the temp directory path, that
// neither File nor Function on any returned StackFrame ever contains any
// substring of the raw temp path -- the exact T-08-42 mitigation this
// file exists to prove.
func TestParseStackTraceNeverLeaksTempPath(t *testing.T) {
	const scriptName = "LeakCheck"
	const tempDir = `C:\Users\op\AppData\Local\Temp\golc-script-run-9999`
	const tempPath = "file:///C:/Users/op/AppData/Local/Temp/golc-script-run-9999/0199abcd-ef01-7000-8000-000000000002.ts"

	raw := "Uncaught Error: boom\n" +
		"    at Object.doThing (" + tempPath + ":20:3)\n" +
		"    at " + tempPath + ":2:1\n"

	frames := parseStackTrace(raw, 10, scriptName)
	require.NotEmpty(t, frames, "expected at least one parsed frame")
	for _, frame := range frames {
		require.NotContains(t, frame.File, tempDir, "frame leaked the temp directory path: %+v", frame)
		require.NotContains(t, frame.Function, tempDir, "frame leaked the temp directory path: %+v", frame)
		require.NotContains(t, frame.File, "golc-script-run-", "frame leaked a temp run directory name: %+v", frame)
		require.NotContains(t, frame.Function, "golc-script-run-", "frame leaked a temp run directory name: %+v", frame)
		require.Equal(t, scriptName, frame.File, "frame.File = %q, want the script's user-facing name %q, not the temp path", frame.File, scriptName)
	}
}

// TestCorrectLine covers the shared shim-offset-correction helper
// debugbridge.go also reuses.
func TestCorrectLine(t *testing.T) {
	tests := []struct {
		rawLine, shimLineCount int
		wantUserLine           int
		wantInShim             bool
	}{
		{rawLine: 15, shimLineCount: 10, wantUserLine: 5, wantInShim: false},
		{rawLine: 11, shimLineCount: 10, wantUserLine: 1, wantInShim: false},
		{rawLine: 10, shimLineCount: 10, wantUserLine: 0, wantInShim: true},
		{rawLine: 1, shimLineCount: 10, wantUserLine: -9, wantInShim: true},
	}
	for _, tt := range tests {
		userLine, inShim := correctLine(tt.rawLine, tt.shimLineCount)
		require.Equal(t, tt.wantUserLine, userLine, "correctLine(%d, %d) = (%d, %v), want (%d, %v)",
			tt.rawLine, tt.shimLineCount, userLine, inShim, tt.wantUserLine, tt.wantInShim)
		require.Equal(t, tt.wantInShim, inShim, "correctLine(%d, %d) = (%d, %v), want (%d, %v)",
			tt.rawLine, tt.shimLineCount, userLine, inShim, tt.wantUserLine, tt.wantInShim)
	}
}
