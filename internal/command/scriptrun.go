// scriptrun.go is the "script run" command file (08-05-PLAN.md Task 3,
// CONTEXT SCRP-01): the CLI route that drives a saved script's real
// execution. It is a separate file from script.go (08-01) deliberately,
// so later plans adding "script stop", "script validate", and
// "script debug" each own their own file and never contend for the same
// source file.
//
// registryExecutor is this package's script.Executor adapter, mirroring
// artnet.go's apiCommandExecutor exactly (07-02-PLAN.md Task 1/Task 2
// precedent): internal/script depends only on the three-method Executor
// contract, never on this package's import path, so this package -- the
// only one that may legally import both internal/script and this file's
// own registry -- is where the adapter lives.
package command

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lnorton89/golc/internal/script"
	"github.com/lnorton89/golc/internal/scriptsdk"
	"github.com/lnorton89/golc/internal/show"
	"github.com/lnorton89/golc/internal/strictjson"
)

var _ = MustDeclareRoute(CommandRegistration{
	Route: "script run",
	Summary: "Run a saved script in a fresh, zero-permission Deno process, reaching GOLC only through the generated typed SDK (SCRP-01/SCRP-02/SCRP-03): " +
		"script run <name> --show <path>.",
	Handler: runScriptRun,
})

// registryExecutor adapts a *CommandRegistry to script.Executor
// structurally (the same seam shape internal/api's Executor interface
// establishes, artnet.go's apiCommandExecutor is the existing precedent
// for wrapping the registry this way). Execute carries one extra
// defense-in-depth check beyond apiCommandExecutor's: it refuses any
// route scriptsdk did not classify as an exposed SDK method, even though
// internal/script's own method lookup (session.go's
// methodDescriptorsByRoute) already guarantees the same thing before ever
// building an Executor.Execute call -- so a future caller of
// registryExecutor.Execute directly, bypassing that lookup, still cannot
// reach a route the SDK did not declare.
type registryExecutor struct {
	registry *CommandRegistry
}

// Execute implements script.Executor.
func (e registryExecutor) Execute(route string, args []string, root string) (exitCode int, stdout, stderr []byte) {
	if !isExposedSDKRoute(route) {
		return 2, nil, fmt.Appendf(nil, "GOLC_SCRIPT_ROUTE_UNKNOWN: %q is not a route the generated SDK exposes to scripts\n", route)
	}

	registration, rest, ok := e.registry.Lookup(strings.Fields(route))
	if !ok || len(rest) != 0 {
		return 2, nil, fmt.Appendf(nil, "GOLC_ROUTE_UNKNOWN: no registered route matches %q\n", route)
	}
	result := registration.Handler(Request{Route: registration.Route, Args: args, Root: root})
	return result.ExitCode, result.Stdout, result.Stderr
}

// isExposedSDKRoute reports whether route is one of scriptsdk.
// RegisteredSDKMethods()'s declared routes -- the same closed set
// internal/script's dispatch loop already checks before ever building an
// Executor.Execute call.
func isExposedSDKRoute(route string) bool {
	for _, descriptor := range scriptsdk.RegisteredSDKMethods() {
		if descriptor.Route == route {
			return true
		}
	}
	return false
}

// scriptRunOutcomeView is one CallOutcome's rendered JSON shape.
type scriptRunOutcomeView struct {
	Method     string `json:"method"`
	Route      string `json:"route,omitempty"`
	DurationMS int64  `json:"duration_ms"`
	Ok         bool   `json:"ok"`
	Code       string `json:"code,omitempty"`
	Message    string `json:"message,omitempty"`
}

// scriptRunLogView is one LogLine's rendered JSON shape.
type scriptRunLogView struct {
	Level   string `json:"level"`
	Message string `json:"message"`
	Source  string `json:"source,omitempty"`
}

// scriptRunResultView is "script run"'s uniform success/failure JSON
// shape: run_id, status, per-call outcomes, and captured logs (08-05-
// PLAN.md Task 3's exact <behavior> requirement).
type scriptRunResultView struct {
	RunID    string                 `json:"run_id"`
	Status   string                 `json:"status"`
	Reason   string                 `json:"reason,omitempty"`
	Outcomes []scriptRunOutcomeView `json:"outcomes"`
	Logs     []scriptRunLogView     `json:"logs"`
}

// toScriptRunResultView projects a script.RunOutcome into its rendered
// view shape. Outcomes/Logs always render as JSON arrays, never null.
func toScriptRunResultView(outcome script.RunOutcome) scriptRunResultView {
	outcomes := make([]scriptRunOutcomeView, 0, len(outcome.Outcomes))
	for _, o := range outcome.Outcomes {
		outcomes = append(outcomes, scriptRunOutcomeView{
			Method: o.Method, Route: o.Route, DurationMS: o.DurationMS, Ok: o.Ok, Code: o.Code, Message: o.Message,
		})
	}
	logs := make([]scriptRunLogView, 0, len(outcome.Logs))
	for _, l := range outcome.Logs {
		logs = append(logs, scriptRunLogView{Level: l.Level, Message: l.Message, Source: l.Source})
	}
	return scriptRunResultView{
		RunID:    outcome.RunID.String(),
		Status:   string(outcome.Status),
		Reason:   outcome.Reason,
		Outcomes: outcomes,
		Logs:     logs,
	}
}

// runScriptRun serves the self-registered "script run" route: parse
// --show, load the ShowState, resolve the named script (GOLC_SCRIPT_NOT_
// FOUND if absent -- this route never spawns a process when the show
// cannot be loaded or the script is missing), build a script.Host wired
// to a fresh registryExecutor over this daemon's own default command
// registry, and drive the run to completion. The persisted script's
// LastRunStatus/LastRunReason/LastRunAt are written back and saved --
// this is a show mutation and therefore bumps Revision (D-16's library
// view reflects run status through the same refresh path as every other
// show change). Exit 0 on a successful run, 1 on a failed or terminated
// run (including a script.Host construction/dispatch failure), 2 on a
// malformed invocation.
func runScriptRun(request Request) Result {
	usage := "script run <name> --show <path>"
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
	target, index, found := scriptByName(state.Scripts, name)
	if !found {
		return Result{ExitCode: 1, Stderr: fmt.Appendf(nil, "GOLC_SCRIPT_NOT_FOUND: no script named %q exists\n", name)}
	}

	registry, err := NewDefaultCommandRegistry()
	if err != nil {
		return Result{ExitCode: 1, Stderr: fmt.Appendf(nil, "GOLC_SCRIPT_RUN_FAILED: %v\n", err)}
	}

	host, err := script.NewHost(script.HostConfig{
		Root:     request.Root,
		ShowPath: showPath,
		Executor: registryExecutor{registry: registry},
	})
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	outcome, runErr := host.Run(context.Background(), target, script.LaunchModeRun, nil)
	if runErr != nil {
		return Result{ExitCode: 1, Stderr: []byte(runErr.Error() + "\n")}
	}

	target.LastRunStatus = outcome.Status
	target.LastRunReason = outcome.Reason
	target.LastRunAt = time.Now().UTC().Format(time.RFC3339)
	state.Scripts[index] = target
	if err := show.Save(request.Root, showPath, state); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	payload, encodeErr := strictjson.CanonicalEncode(toScriptRunResultView(outcome))
	if encodeErr != nil {
		return Result{ExitCode: 1, Stderr: fmt.Appendf(nil, "GOLC_SCRIPT_ENCODE_FAILED: %v\n", encodeErr)}
	}

	exitCode := 0
	if outcome.Status == show.ScriptRunStatusFailed || outcome.Status == show.ScriptRunStatusTerminated {
		exitCode = 1
	}
	return Result{ExitCode: exitCode, Stdout: payload}
}
