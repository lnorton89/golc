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
	"fmt"
	"regexp"
	"strings"
)

// maxScriptSourceBytes bounds a script's source for validation (T-08-33
// DoS mitigation), mirroring internal/command/script.go's identical 1
// MiB bound for "script edit" -- internal/script cannot import
// internal/command (toolchain.go's package doc comment), so this is a
// deliberately duplicated constant rather than a shared one.
const maxScriptSourceBytes = 1 << 20 // 1 MiB

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
