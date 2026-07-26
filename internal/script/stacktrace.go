// stacktrace.go implements D-03's source-mapped stack traces
// (08-09-PLAN.md Task 1, CONTEXT SCRP-05): parses Deno's own
// already-source-mapped stack trace text (Deno executes TypeScript
// directly and maps traces back to the original .ts source natively --
// this file never re-derives positions from a source map itself,
// 08-RESEARCH.md's "Don't Hand-Roll" table) into a stable []StackFrame
// value, offset-corrected into the author's own source coordinates and
// with the materialized temp-file path replaced by the script's
// user-facing name everywhere.
//
// correctLine is the single shim-offset-correction helper both this
// file's parseStackTrace (Deno's own textual stack traces, captured from
// a run's stderr) and debugbridge.go (every CDP-reported position, a
// paused frame or an exception location) call -- so the two places this
// package corrects a raw materialized-file line number into the author's
// own coordinate can never independently drift apart.
package script

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/lnorton89/golc/internal/security"
)

// StackFrame is one source-mapped stack frame: Function is the frame's
// call-site name (or "<anonymous>" when Deno's own trace omits one),
// File is always the script's user-facing name -- never the temp
// directory path a raw Deno trace actually names -- and Line/Column are
// in the author's own source coordinates. A frame whose raw position
// falls inside the injected SDK runtime shim carries the
// GOLC_SCRIPT_SDK_SHIM_ERROR marker in Function and Line 0 (never a
// negative line) instead of a nonsensical user-coordinate line number.
type StackFrame struct {
	Function string
	File     string
	Line     int
	Column   int
}

// shimErrorMarker is the exact marker parseStackTrace and debugbridge.go
// both append to Function when a frame's raw position resolves inside
// the injected SDK runtime shim rather than the user's own source
// (08-09-PLAN.md Task 1's exact <behavior> bullet).
const shimErrorMarker = "GOLC_SCRIPT_SDK_SHIM_ERROR"

// correctLine subtracts shimLineCount from rawLine (both 1-based) to
// recover the author's own source coordinate, reporting inShim=true when
// the result falls at or before line 0 -- the position lands inside the
// injected shim rather than the user's own source, so userLine must
// never be reported as a negative or zero "user line" on its own.
func correctLine(rawLine, shimLineCount int) (userLine int, inShim bool) {
	userLine = rawLine - shimLineCount
	return userLine, userLine < 1
}

// stackFrameAtPattern matches one Deno/V8 stack trace frame line, e.g.
// "    at doThing (file:///C:/tmp/run/0199....ts:12:5)" or the
// function-less form "    at file:///C:/tmp/run/0199....ts:12:5".
var stackFrameAtPattern = regexp.MustCompile(`^\s*at\s+(.+)$`)

// stackFrameLocationPattern matches a trailing "<anything>:LINE:COLUMN"
// location suffix (the file:// URL portion is discarded entirely --
// parseStackTrace never trusts or surfaces it, since File is always the
// script's own user-facing name, per this file's package doc comment).
var stackFrameLocationPattern = regexp.MustCompile(`^(.*?):(\d+):(\d+)$`)

// parseStackFrameLine parses one line of Deno's own captured stack trace
// text into a StackFrame, reporting ok=false for a line that is not a
// recognizable "at ..." stack frame (e.g. the leading "Uncaught Error:
// ..." message line, which parseStackTrace's caller never treats as a
// frame).
func parseStackFrameLine(line string, shimLineCount int, scriptName string) (StackFrame, bool) {
	atMatch := stackFrameAtPattern.FindStringSubmatch(line)
	if atMatch == nil {
		return StackFrame{}, false
	}
	rest := strings.TrimSpace(atMatch[1])

	function := ""
	locationSpec := rest
	if strings.HasSuffix(rest, ")") {
		if open := strings.LastIndex(rest, " ("); open >= 0 {
			function = strings.TrimSpace(rest[:open])
			locationSpec = strings.TrimSuffix(rest[open+2:], ")")
		}
	}

	locationMatch := stackFrameLocationPattern.FindStringSubmatch(locationSpec)
	if locationMatch == nil {
		return StackFrame{}, false
	}
	rawLine, lineErr := strconv.Atoi(locationMatch[2])
	rawColumn, columnErr := strconv.Atoi(locationMatch[3])
	if lineErr != nil || columnErr != nil {
		return StackFrame{}, false
	}

	if function == "" {
		function = "<anonymous>"
	}
	function = security.Redact(function)
	file := security.Redact(scriptName)

	userLine, inShim := correctLine(rawLine, shimLineCount)
	if inShim {
		return StackFrame{Function: function + ": " + shimErrorMarker, File: file, Line: 0, Column: rawColumn}, true
	}
	return StackFrame{Function: function, File: file, Line: userLine, Column: rawColumn}, true
}

// parseStackTrace converts raw Deno stack trace text (as captured from a
// run's stderr on an uncaught exception) into []StackFrame, one entry per
// recognized "at ..." line, offset-corrected into the author's own
// source coordinates via shimLineCount and always reporting File as
// scriptName -- the materialized temp-file path Deno's own raw trace
// text actually names never appears in the returned value.
func parseStackTrace(raw string, shimLineCount int, scriptName string) []StackFrame {
	var frames []StackFrame
	for _, line := range strings.Split(raw, "\n") {
		frame, ok := parseStackFrameLine(line, shimLineCount, scriptName)
		if ok {
			frames = append(frames, frame)
		}
	}
	return frames
}
