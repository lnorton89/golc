// scriptvalidate.go is the "script validate" command file (08-07-
// PLAN.md, CONTEXT SCRP-01/SCRP-03): the CLI route that runs SCRP-01's
// validate verb against a saved script -- the structural zero-import
// gate plus a deno check type-check against the generated SDK types --
// without ever executing it. A separate file from scriptrun.go
// deliberately, for the same reason scriptrun.go is separate from
// script.go (08-05-PLAN.md's precedent): each script lifecycle route
// owns its own source file.
package command

import (
	"context"
	"fmt"

	"github.com/lnorton89/golc/internal/script"
	"github.com/lnorton89/golc/internal/show"
	"github.com/lnorton89/golc/internal/strictjson"
)

var _ = MustDeclareRoute(CommandRegistration{
	Route: "script validate",
	Summary: "Validate a saved script's structural zero-import boundary and type-check it against the generated SDK, without ever executing it: " +
		"script validate <name> --show <path>.",
	Handler: runScriptValidate,
})

// scriptValidateDiagnosticView is one script.Diagnostic's rendered JSON
// shape.
type scriptValidateDiagnosticView struct {
	Code     string `json:"code"`
	Message  string `json:"message"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	Severity string `json:"severity"`
}

// scriptValidateResultView is "script validate"'s uniform JSON shape:
// valid, plus every diagnostic found (never null, always at least an
// empty array).
type scriptValidateResultView struct {
	Valid       bool                           `json:"valid"`
	Diagnostics []scriptValidateDiagnosticView `json:"diagnostics"`
}

// toScriptValidateResultView projects a script.ValidationResult into its
// rendered view shape.
func toScriptValidateResultView(result script.ValidationResult) scriptValidateResultView {
	diagnostics := make([]scriptValidateDiagnosticView, 0, len(result.Diagnostics))
	for _, d := range result.Diagnostics {
		diagnostics = append(diagnostics, scriptValidateDiagnosticView{
			Code: d.Code, Message: d.Message, Line: d.Line, Column: d.Column, Severity: string(d.Severity),
		})
	}
	return scriptValidateResultView{Valid: result.Valid, Diagnostics: diagnostics}
}

// runScriptValidate serves the self-registered "script validate" route:
// parse --show, load the ShowState, resolve the named script
// (GOLC_SCRIPT_NOT_FOUND if absent -- this route never runs a check when
// the show cannot be loaded or the script is missing), and run
// script.Validate. Exit 0 with {"valid":true,"diagnostics":[]} for a
// clean script, exit 1 with the diagnostic array for a failing one, exit
// 2 on a malformed invocation.
func runScriptValidate(request Request) Result {
	usage := "script validate <name> --show <path>"
	name, flags, err := parseScriptPositionalArgs(usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}
	if err := rejectUnknownScriptFlags(usage, flags, map[string]bool{"show": true}); err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}
	showPath, ok := flags["show"]
	if !ok || showPath == "" {
		return Result{ExitCode: 2, Stderr: fmt.Appendf(nil, "GOLC_SCRIPT_USAGE: --show is required; usage: %s\n", usage)}
	}

	state, err := show.Load(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	target, _, found := scriptByName(state.Scripts, name)
	if !found {
		return Result{ExitCode: 1, Stderr: fmt.Appendf(nil, "GOLC_SCRIPT_NOT_FOUND: no script named %q exists\n", name)}
	}

	result, err := script.Validate(context.Background(), request.Root, target)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	payload, encodeErr := strictjson.CanonicalEncode(toScriptValidateResultView(result))
	if encodeErr != nil {
		return Result{ExitCode: 1, Stderr: fmt.Appendf(nil, "GOLC_SCRIPT_ENCODE_FAILED: %v\n", encodeErr)}
	}

	exitCode := 0
	if !result.Valid {
		exitCode = 1
	}
	return Result{ExitCode: exitCode, Stdout: payload}
}
