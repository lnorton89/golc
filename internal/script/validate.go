// validate.go implements SCRP-01's validate verb (08-07-PLAN.md): the
// structural zero-import gate and a `deno check` type-check against the
// generated GOLC SDK types, with diagnostics mapped back into the
// author's own source coordinates.
//
// Validate's checks run in a fixed order, and the order is load-bearing:
//
//  1. Size bound (checkSourceSize) -- a pathological source is rejected
//     before any byte of it is ever scanned.
//  2. The structural zero-import gate (checkForbiddenModuleSyntax) -- an
//     import/export/dynamic-import must never reach Deno's module
//     resolver, which runs before any permission check (08-RESEARCH.md
//     Pitfall 4). This gate is pure and spawns nothing.
//  3. `deno check` (Task 2) -- the only step that spawns a subprocess, so
//     it must never run until both prior gates have already passed.
//
// Reordering these steps would either let an import statement reach
// Deno's resolver (defeating the whole point of the structural gate) or
// spend a subprocess spawn validating a script this package was always
// going to reject anyway.
package script

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/lnorton89/golc/internal/scriptsdk"
	"github.com/lnorton89/golc/internal/security"
	"github.com/lnorton89/golc/internal/show"
)

// maxScriptSourceBytes bounds a script's source for validation (T-08-33
// DoS mitigation), mirroring internal/command/script.go's identical 1
// MiB bound for "script edit" -- internal/script cannot import
// internal/command (toolchain.go's package doc comment), so this is a
// deliberately duplicated constant rather than a shared one.
const maxScriptSourceBytes = 1 << 20 // 1 MiB

// checkOutputBudget bounds the captured `deno check` stdout+stderr text
// (T-08-20's bounded-buffer discipline applied to the check subprocess),
// mirroring session.go's stderrTailBudget precedent but larger, since a
// script with many type errors can legitimately produce more diagnostic
// text than a single run failure's stderr tail.
const checkOutputBudget = 64 * 1024

// checkSourceSize rejects a script whose source exceeds
// maxScriptSourceBytes with GOLC_SCRIPT_SOURCE_TOO_LARGE, before any byte
// of it is scanned or written to disk (T-08-33).
func checkSourceSize(source string) []Diagnostic {
	if len(source) <= maxScriptSourceBytes {
		return nil
	}
	return []Diagnostic{{
		Code: "GOLC_SCRIPT_SOURCE_TOO_LARGE",
		Message: fmt.Sprintf(
			"script source is %d bytes, exceeding the %d byte validation maximum", len(source), maxScriptSourceBytes),
		Line: 1, Column: 1, Severity: SeverityError,
	}}
}

// forbiddenModuleTokenPattern matches the whole word "import" or "export"
// -- TypeScript reserved keywords that can never legitimately appear as
// an identifier, so any surviving occurrence in comment/string-stripped
// source is definitionally the keyword, not a false positive (SCRP-03,
// T-08-29).
var forbiddenModuleTokenPattern = regexp.MustCompile(`\b(import|export)\b`)

// checkForbiddenModuleSyntax rejects a static import statement, a bare
// export declaration (including a `from "..."` re-export -- "export" is
// flagged regardless of what follows it), and a dynamic import(...)
// expression, each with GOLC_SCRIPT_IMPORT_FORBIDDEN and the offending
// line number. It strips line comments, block comments, and string/
// template literals first (stripCommentsAndStringLiterals) -- a small
// hand-rolled scanner, not a regex over raw text, because a regex over
// raw text would false-positive on any script that merely mentions
// "import" inside a comment or a log message (08-RESEARCH.md Pitfall 4).
// Template literal substitutions (`${...}`) are scanned as real code, not
// blanked, so a dynamic import hidden inside one (for example a template
// literal whose substitution reads "await import(...)") cannot bypass the
// gate.
func checkForbiddenModuleSyntax(source string) []Diagnostic {
	cleaned := stripCommentsAndStringLiterals(source)
	lines := strings.Split(cleaned, "\n")

	var diagnostics []Diagnostic
	for lineIndex, line := range lines {
		lineNumber := lineIndex + 1
		for _, match := range forbiddenModuleTokenPattern.FindAllStringIndex(line, -1) {
			token := line[match[0]:match[1]]
			column := match[0] + 1
			diagnostics = append(diagnostics, forbiddenModuleDiagnostic(token, line[match[1]:], lineNumber, column))
		}
	}
	sortDiagnostics(diagnostics)
	return diagnostics
}

// forbiddenModuleDiagnostic renders one GOLC_SCRIPT_IMPORT_FORBIDDEN
// diagnostic for a matched "import"/"export" token: rest is the matched
// line's remainder, used only to distinguish a dynamic import(...)
// expression from a static import statement in the message text.
func forbiddenModuleDiagnostic(token, rest string, line, column int) Diagnostic {
	const code = "GOLC_SCRIPT_IMPORT_FORBIDDEN"
	if token == "export" {
		return Diagnostic{
			Code: code,
			Message: "export declarations are forbidden: a script is a single ambient-global consumer, " +
				"never a module another file could import",
			Line: line, Column: column, Severity: SeverityError,
		}
	}
	if strings.HasPrefix(strings.TrimLeft(rest, " \t"), "(") {
		return Diagnostic{
			Code: code,
			Message: "dynamic import(...) expressions are forbidden: the GOLC SDK is available as the " +
				"ambient `golc` global and no module specifier is needed or permitted",
			Line: line, Column: column, Severity: SeverityError,
		}
	}
	return Diagnostic{
		Code: code,
		Message: "import statements are forbidden: the GOLC SDK is available as the ambient `golc` global " +
			"and no module specifier is needed or permitted",
		Line: line, Column: column, Severity: SeverityError,
	}
}

// stripCommentsAndStringLiterals returns source with every line comment,
// block comment, and string/template literal's raw character content
// replaced by spaces -- every newline byte is preserved exactly, so line
// numbers computed against the returned text always match the original
// source. Template literal substitutions (`${...}`) are the one region
// never blanked: they are real, executable TypeScript, so
// checkForbiddenModuleSyntax must still be able to see a forbidden token
// hidden inside one.
func stripCommentsAndStringLiterals(source string) string {
	const (
		modeNormal = iota
		modeLineComment
		modeBlockComment
		modeSingleQuote
		modeDoubleQuote
		modeTemplateRaw
	)

	// templateFrame tracks one active template literal: kind is "raw"
	// while scanning the literal's own blanked characters, and "sub"
	// while scanning inside an active `${...}` substitution, in which
	// case braceDepth counts nested `{`/`}` so the substitution's own
	// closing brace can be told apart from a nested object literal's.
	type templateFrame struct {
		kind       string
		braceDepth int
	}

	var out strings.Builder
	out.Grow(len(source))

	var stack []*templateFrame
	mode := modeNormal
	i, n := 0, len(source)

	for i < n {
		c := source[i]
		switch mode {
		case modeNormal:
			switch {
			case c == '/' && i+1 < n && source[i+1] == '/':
				mode = modeLineComment
				out.WriteString("  ")
				i += 2
			case c == '/' && i+1 < n && source[i+1] == '*':
				mode = modeBlockComment
				out.WriteString("  ")
				i += 2
			case c == '\'':
				mode = modeSingleQuote
				out.WriteByte(' ')
				i++
			case c == '"':
				mode = modeDoubleQuote
				out.WriteByte(' ')
				i++
			case c == '`':
				stack = append(stack, &templateFrame{kind: "raw"})
				mode = modeTemplateRaw
				out.WriteByte(' ')
				i++
			case c == '{' && len(stack) > 0 && stack[len(stack)-1].kind == "sub":
				stack[len(stack)-1].braceDepth++
				out.WriteByte(c)
				i++
			case c == '}' && len(stack) > 0 && stack[len(stack)-1].kind == "sub":
				top := stack[len(stack)-1]
				if top.braceDepth > 0 {
					top.braceDepth--
					out.WriteByte(c)
				} else {
					top.kind = "raw"
					mode = modeTemplateRaw
					out.WriteByte(' ')
				}
				i++
			default:
				out.WriteByte(c)
				i++
			}
		case modeLineComment:
			if c == '\n' {
				mode = modeNormal
				out.WriteByte('\n')
			} else {
				out.WriteByte(' ')
			}
			i++
		case modeBlockComment:
			if c == '*' && i+1 < n && source[i+1] == '/' {
				mode = modeNormal
				out.WriteString("  ")
				i += 2
			} else if c == '\n' {
				out.WriteByte('\n')
				i++
			} else {
				out.WriteByte(' ')
				i++
			}
		case modeSingleQuote:
			switch {
			case c == '\\' && i+1 < n:
				out.WriteString("  ")
				i += 2
			case c == '\'':
				mode = modeNormal
				out.WriteByte(' ')
				i++
			case c == '\n':
				// An unterminated string reaching EOL is a syntax error
				// deno check will independently reject -- this gate only
				// needs to avoid hanging in the wrong state.
				mode = modeNormal
				out.WriteByte('\n')
				i++
			default:
				out.WriteByte(' ')
				i++
			}
		case modeDoubleQuote:
			switch {
			case c == '\\' && i+1 < n:
				out.WriteString("  ")
				i += 2
			case c == '"':
				mode = modeNormal
				out.WriteByte(' ')
				i++
			case c == '\n':
				mode = modeNormal
				out.WriteByte('\n')
				i++
			default:
				out.WriteByte(' ')
				i++
			}
		case modeTemplateRaw:
			switch {
			case c == '\\' && i+1 < n:
				out.WriteString("  ")
				i += 2
			case c == '`':
				stack = stack[:len(stack)-1]
				mode = modeNormal
				out.WriteByte(' ')
				i++
			case c == '$' && i+1 < n && source[i+1] == '{':
				stack[len(stack)-1].kind = "sub"
				mode = modeNormal
				out.WriteString("  ")
				i += 2
			case c == '\n':
				out.WriteByte('\n')
				i++
			default:
				out.WriteByte(' ')
				i++
			}
		}
	}
	return out.String()
}

// buildDenoCheckArgs is the single composition site for a `deno check`
// type-check invocation (T-08-30): a dedicated function, deliberately
// separate from host.go's buildDenoArgs (the run command line), so a
// future change to one can never accidentally widen the other.
// --cached-only denies module resolution any network access (superseding
// the removed --no-remote flag, 08-RESEARCH.md State of the Art) --
// belt-and-suspenders alongside the structural zero-import gate, since a
// script that ever reaches this step already contains no import for
// --cached-only to need to deny. No branch of this function may ever
// append a permission-granting flag.
func buildDenoCheckArgs(scriptPath string) []string {
	return []string{"check", "--no-prompt", "--cached-only", scriptPath}
}

// shimLineOffsetFor returns the number of materialized-file lines the
// injected SDK runtime shim occupies ahead of the user's own source --
// derived from shim's actual content (strings.Count, never a hardcoded
// constant), plus the one extra newline Validate/session.go's Run both
// inject between the shim and the user's source. Every diagnostic line
// deno check reports against the materialized file has this value
// subtracted before it is shown to the user (T-08-32).
func shimLineOffsetFor(shim string) int {
	return strings.Count(shim, "\n") + 1
}

// denoCheckDiagnosticHeaderPattern matches deno check's diagnostic header
// line, e.g. "TS2345 [ERROR]: Argument of type ... is not assignable ..."
// (optionally prefixed with "error: "), capturing the message text that
// follows the code/severity tag.
var denoCheckDiagnosticHeaderPattern = regexp.MustCompile(`^(?:error: )?TS\d+ \[(?:ERROR|WARN)\]:\s*(.*)$`)

// denoCheckDiagnosticLocationPattern matches deno check's trailing
// "at file://.../script.ts:LINE:COLUMN" location line, capturing the raw
// (materialized-file) line and column deno check reported.
var denoCheckDiagnosticLocationPattern = regexp.MustCompile(`^\s*at\s+.+:(\d+):(\d+)\s*$`)

// parseDenoCheckDiagnostics parses deno check's combined, already-
// redacted stdout+stderr text into Diagnostic values: every reported raw
// line has shimLineOffset subtracted to recover the user's own source
// coordinate (T-08-32); a position that lands at or before line 0 after
// subtraction falls inside the injected shim itself and is reported under
// the distinct GOLC_SCRIPT_SDK_SHIM_ERROR code instead of a nonsensical
// non-positive user line number.
func parseDenoCheckDiagnostics(output string, shimLineOffset int) []Diagnostic {
	var diagnostics []Diagnostic
	var pendingLines []string
	pending := false

	flush := func(rawLine, rawColumn int) {
		if !pending {
			return
		}
		message := strings.TrimSpace(strings.Join(pendingLines, " "))
		userLine := rawLine - shimLineOffset
		if userLine < 1 {
			diagnostics = append(diagnostics, Diagnostic{
				Code: "GOLC_SCRIPT_SDK_SHIM_ERROR", Message: message,
				Line: rawLine, Column: rawColumn, Severity: SeverityError,
			})
		} else {
			diagnostics = append(diagnostics, Diagnostic{
				Code: "GOLC_SCRIPT_TYPECHECK_FAILED", Message: message,
				Line: userLine, Column: rawColumn, Severity: SeverityError,
			})
		}
		pending = false
		pendingLines = nil
	}

	for _, line := range strings.Split(output, "\n") {
		if m := denoCheckDiagnosticHeaderPattern.FindStringSubmatch(line); m != nil {
			pending = true
			pendingLines = []string{m[1]}
			continue
		}
		if m := denoCheckDiagnosticLocationPattern.FindStringSubmatch(line); m != nil {
			rawLine, _ := strconv.Atoi(m[1])
			rawColumn, _ := strconv.Atoi(m[2])
			flush(rawLine, rawColumn)
			continue
		}
		if pending {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" && !strings.HasPrefix(trimmed, "^") {
				pendingLines = append(pendingLines, trimmed)
			}
		}
	}
	sortDiagnostics(diagnostics)
	return diagnostics
}

// redactLines applies security.Redact independently to every line of
// text, joining the result back with "\n" -- deno check's captured
// stdout/stderr passes through this before parsing or ever reaching a
// caller (T-08-31), matching session.go's per-line LogLine redaction
// discipline rather than redacting the whole blob at once (which would
// discard an entire multi-line diagnostic the instant any one of its
// lines happened to contain a forbidden token).
func redactLines(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = security.Redact(line)
	}
	return strings.Join(lines, "\n")
}

// generatedSDKTypesPath is the committed golc.d.ts path, relative to the
// repository root, Validate reads and copies beside the materialized
// script -- the same file the in-app Monaco editor loads (D-15) and
// internal/scriptsdk/generate.go commits.
const generatedSDKTypesPath = "internal/scriptsdk/generated/golc.d.ts"

// denoCheckConfig is the minimal deno.json Validate emits beside the
// materialized script and golc.d.ts: compilerOptions.types is the
// mechanism deno check honors for an ambient .d.ts that is never
// imported (verified against the pinned Deno version's documented
// compilerOptions support, 08-RESEARCH.md; 08-07-SUMMARY.md records the
// exact behavior observed at implementation time). Deno auto-discovers a
// deno.json in the checked file's own directory, so no extra flag is
// needed to make deno check load it.
const denoCheckConfig = `{"compilerOptions":{"types":["./golc.d.ts"]}}` + "\n"

// Validate runs SCRP-01's validate verb against a saved script: the size
// bound, then the structural zero-import gate, then -- only if both pass
// -- a `deno check` type-check against the committed generated SDK
// types, with every diagnostic mapped back into the user's own source
// coordinates. It never mutates the show, never spawns a subprocess when
// either gate already produced a diagnostic, and removes its temp
// directory on every exit path.
func Validate(ctx context.Context, root string, s show.Script) (ValidationResult, error) {
	result := ValidationResult{ScriptName: s.Name, Diagnostics: []Diagnostic{}}

	if diagnostics := checkSourceSize(s.Source); len(diagnostics) > 0 {
		result.Diagnostics = diagnostics
		return result, nil
	}
	if diagnostics := checkForbiddenModuleSyntax(s.Source); len(diagnostics) > 0 {
		result.Diagnostics = diagnostics
		return result, nil
	}

	denoPath, err := ResolveDenoExecutable(root)
	if err != nil {
		return ValidationResult{}, err
	}

	tempDir, err := os.MkdirTemp("", "golc-script-validate-*")
	if err != nil {
		return ValidationResult{}, fmt.Errorf("GOLC_SCRIPT_VALIDATE_TEMP_DIR_FAILED: %v", err)
	}
	defer os.RemoveAll(tempDir)

	shimLineOffset := shimLineOffsetFor(scriptsdk.RuntimeShimTS)
	materialized := scriptsdk.RuntimeShimTS + "\n" + s.Source
	scriptPath := filepath.Join(tempDir, "script.ts")
	if err := os.WriteFile(scriptPath, []byte(materialized), 0o600); err != nil {
		return ValidationResult{}, fmt.Errorf("GOLC_SCRIPT_VALIDATE_SOURCE_WRITE_FAILED: %v", err)
	}

	dtsBytes, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(generatedSDKTypesPath)))
	if err != nil {
		return ValidationResult{}, fmt.Errorf("GOLC_SCRIPT_VALIDATE_SDK_TYPES_MISSING: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "golc.d.ts"), dtsBytes, 0o600); err != nil {
		return ValidationResult{}, fmt.Errorf("GOLC_SCRIPT_VALIDATE_SDK_TYPES_WRITE_FAILED: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "deno.json"), []byte(denoCheckConfig), 0o600); err != nil {
		return ValidationResult{}, fmt.Errorf("GOLC_SCRIPT_VALIDATE_CONFIG_WRITE_FAILED: %v", err)
	}

	cmd := exec.CommandContext(ctx, denoPath, buildDenoCheckArgs(scriptPath)...)
	cmd.Dir = tempDir
	// Explicit, never-inherited environment (T-08-16), matching
	// session.go's Run exactly.
	cmd.Env = []string{}

	stdoutBuf := newBoundedBuffer(checkOutputBudget)
	stderrBuf := newBoundedBuffer(checkOutputBudget)
	cmd.Stdout = stdoutBuf
	cmd.Stderr = stderrBuf

	runErr := cmd.Run()
	combined := redactLines(stdoutBuf.String() + "\n" + stderrBuf.String())

	diagnostics := parseDenoCheckDiagnostics(combined, shimLineOffset)
	if len(diagnostics) == 0 && runErr != nil {
		// deno check exited non-zero but produced nothing this parser
		// recognized as a structured diagnostic -- surface a generic
		// failure rather than silently reporting a clean result.
		diagnostics = []Diagnostic{{
			Code: "GOLC_SCRIPT_TYPECHECK_FAILED", Message: strings.TrimSpace(combined),
			Line: 1, Column: 1, Severity: SeverityError,
		}}
	}

	sortDiagnostics(diagnostics)
	result.Diagnostics = diagnostics
	result.Valid = len(diagnostics) == 0
	return result, nil
}
