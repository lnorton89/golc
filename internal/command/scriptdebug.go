// scriptdebug.go is the "script debug"/"script continue"/
// "script step-over"/"script step-into"/"script step-out" command file
// (08-09-PLAN.md Task 3, CONTEXT SCRP-01/SCRP-05, D-01/D-02): the CLI
// routes that launch a saved script in Debug mode with a set of
// author-coordinate breakpoints and drive the resulting run's step
// controls. A separate file from script.go/scriptrun.go/scriptstop.go/
// scriptvalidate.go deliberately, so no plan contends for the same
// source file (08-05-PLAN.md's established precedent).
package command

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/lnorton89/golc/internal/script"
	"github.com/lnorton89/golc/internal/show"
	"github.com/lnorton89/golc/internal/strictjson"
)

var _ = MustDeclareRoute(CommandRegistration{
	Route: "script debug",
	Summary: "Launch a saved script in Debug mode with a set of author-coordinate breakpoints, mediated entirely by the Go daemon's own CDP client " +
		"(D-01/D-02): script debug <name> --show <path> [--breakpoint <line>...].",
	Handler: runScriptDebug,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "script continue",
	Summary: "Resume the single active debug run: script continue --show <path>.",
	Handler: runScriptContinue,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "script step-over",
	Summary: "Step over the current statement in the single active debug run: script step-over --show <path>.",
	Handler: runScriptStepOver,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "script step-into",
	Summary: "Step into the current call in the single active debug run: script step-into --show <path>.",
	Handler: runScriptStepInto,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "script step-out",
	Summary: "Step out of the current call in the single active debug run: script step-out --show <path>.",
	Handler: runScriptStepOut,
})

// scriptDebugUsage is "script debug"'s usage string, shared by every
// error path in this file's debug-launch handler.
const scriptDebugUsage = "script debug <name> --show <path> [--breakpoint <line>...]"

// parseScriptDebugArgs accepts the required positional <name>, a
// required --show, and zero or more repeatable --breakpoint <line>
// flags -- distinct from parseScriptPositionalArgs (script.go), which has
// no notion of a repeatable flag. A --breakpoint value that is not a
// positive integer fails immediately with GOLC_SCRIPT_BREAKPOINT_INVALID
// (ExitCode 2); the line-count-exceeded half of that same validation
// happens later in runScriptDebug, once the target script's actual
// source is loaded.
func parseScriptDebugArgs(args []string) (name, showPath string, breakpoints []int, err error) {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return "", "", nil, fmt.Errorf("GOLC_SCRIPT_USAGE: usage: %s", scriptDebugUsage)
	}
	name = args[0]

	rest := args[1:]
	for i := 0; i < len(rest); {
		argument := rest[i]
		if !strings.HasPrefix(argument, "--") {
			return "", "", nil, fmt.Errorf("GOLC_SCRIPT_USAGE: unsupported argument %q; usage: %s", argument, scriptDebugUsage)
		}

		var flagName, value string
		if eq := strings.Index(argument, "="); eq >= 0 {
			flagName, value = argument[2:eq], argument[eq+1:]
			i++
		} else {
			flagName = strings.TrimPrefix(argument, "--")
			if i+1 >= len(rest) {
				return "", "", nil, fmt.Errorf("GOLC_SCRIPT_USAGE: --%s requires a value; usage: %s", flagName, scriptDebugUsage)
			}
			value = rest[i+1]
			i += 2
		}

		switch flagName {
		case "show":
			showPath = value
		case "breakpoint":
			line, convErr := strconv.Atoi(value)
			if convErr != nil || line <= 0 {
				return "", "", nil, fmt.Errorf(
					"GOLC_SCRIPT_BREAKPOINT_INVALID: --breakpoint value %q is not a positive integer; usage: %s", value, scriptDebugUsage)
			}
			breakpoints = append(breakpoints, line)
		default:
			return "", "", nil, fmt.Errorf("GOLC_SCRIPT_USAGE: unsupported argument %q; usage: %s", "--"+flagName, scriptDebugUsage)
		}
	}

	if showPath == "" {
		return "", "", nil, fmt.Errorf("GOLC_SCRIPT_USAGE: --show is required; usage: %s", scriptDebugUsage)
	}
	return name, showPath, breakpoints, nil
}

// scriptLineCount returns source's 1-based line count -- an empty source
// still counts as one line, matching how a single trailing newline-free
// line is normally counted.
func scriptLineCount(source string) int {
	if source == "" {
		return 1
	}
	return strings.Count(source, "\n") + 1
}

// runScriptDebug serves the self-registered "script debug" route: parse
// --show/--breakpoint, load the ShowState, resolve the named script
// (GOLC_SCRIPT_NOT_FOUND if absent), validate every requested breakpoint
// against the script's own line count (GOLC_SCRIPT_BREAKPOINT_INVALID,
// ExitCode 2, if any exceeds it -- an out-of-range breakpoint fails fast
// rather than silently never firing), and drive the run exactly like
// "script run" except in LaunchModeDebug with the parsed breakpoint list.
// A debug run with no --breakpoint flags launches with no breakpoints and
// resumes immediately from the initial break (DebugBridge.SetBreakpoints'
// own documented behavior for an empty slice). Exit 0 on a clean
// completion, 1 on a failed/terminated run or a show/host failure, 2 on a
// malformed invocation.
func runScriptDebug(request Request) Result {
	name, showPath, breakpoints, err := parseScriptDebugArgs(request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	state, err := show.Load(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	target, index, found := scriptByName(state.Scripts, name)
	if !found {
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_SCRIPT_NOT_FOUND: no script named %q exists\n", name))}
	}

	lineCount := scriptLineCount(target.Source)
	for _, breakpoint := range breakpoints {
		if breakpoint > lineCount {
			return Result{ExitCode: 2, Stderr: []byte(fmt.Sprintf(
				"GOLC_SCRIPT_BREAKPOINT_INVALID: --breakpoint %d exceeds the script's %d line(s); usage: %s\n",
				breakpoint, lineCount, scriptDebugUsage))}
		}
	}

	registry, err := NewDefaultCommandRegistry()
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_SCRIPT_RUN_FAILED: %v\n", err))}
	}

	host, err := script.NewHost(script.HostConfig{
		Root:     request.Root,
		ShowPath: showPath,
		Executor: registryExecutor{registry: registry},
	})
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	outcome, runErr := host.Run(context.Background(), target, script.LaunchModeDebug, breakpoints)
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
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_SCRIPT_ENCODE_FAILED: %v\n", encodeErr))}
	}

	exitCode := 0
	if outcome.Status == show.ScriptRunStatusFailed || outcome.Status == show.ScriptRunStatusTerminated {
		exitCode = 1
	}
	return Result{ExitCode: exitCode, Stdout: payload}
}

// scriptDebugControlResultView is every step-control route's uniform
// success JSON shape -- there is no run outcome to report yet (the run is
// still active), only an acknowledgement that the requested control was
// issued.
type scriptDebugControlResultView struct {
	OK bool `json:"ok"`
}

// parseScriptControlArgs validates a control route's only flag, --show,
// required for consistency with every other "script *" route even though
// these routes resolve their target through internal/script's single-
// active-run registry rather than through the show document itself.
func parseScriptControlArgs(usage string, args []string) error {
	flags, err := parseScriptFlags(usage, args)
	if err != nil {
		return err
	}
	if err := rejectUnknownScriptFlags(usage, flags, map[string]bool{"show": true}); err != nil {
		return err
	}
	if showPath, ok := flags["show"]; !ok || showPath == "" {
		return fmt.Errorf("GOLC_SCRIPT_USAGE: --show is required; usage: %s", usage)
	}
	return nil
}

// runScriptDebugControl is the shared handler shape every one of the four
// step-control routes drives: parse --show, resolve the single active
// debug run (script.AnyActiveRun, consistent with 08-05's v1 "at most one
// active run, globally" scope call -- no --name flag is needed to
// disambiguate), fail with GOLC_SCRIPT_NO_ACTIVE_DEBUG (ExitCode 1) when
// no run is active or the active run is not a debug run (its bridge is
// nil), and otherwise call action against the resolved bridge.
func runScriptDebugControl(usage string, request Request, action func(*script.DebugBridge) error) Result {
	if err := parseScriptControlArgs(usage, request.Args); err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	run, found := script.AnyActiveRun()
	if !found {
		return Result{ExitCode: 1, Stderr: []byte("GOLC_SCRIPT_NO_ACTIVE_DEBUG: no active debug run\n")}
	}
	bridge := run.Bridge()
	if bridge == nil {
		return Result{ExitCode: 1, Stderr: []byte("GOLC_SCRIPT_NO_ACTIVE_DEBUG: the active run is not a debug run\n")}
	}

	if err := action(bridge); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	payload, encodeErr := strictjson.CanonicalEncode(scriptDebugControlResultView{OK: true})
	if encodeErr != nil {
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_SCRIPT_ENCODE_FAILED: %v\n", encodeErr))}
	}
	return Result{Stdout: payload}
}

func runScriptContinue(request Request) Result {
	return runScriptDebugControl("script continue --show <path>", request, func(b *script.DebugBridge) error { return b.Continue() })
}

func runScriptStepOver(request Request) Result {
	return runScriptDebugControl("script step-over --show <path>", request, func(b *script.DebugBridge) error { return b.StepOver() })
}

func runScriptStepInto(request Request) Result {
	return runScriptDebugControl("script step-into --show <path>", request, func(b *script.DebugBridge) error { return b.StepInto() })
}

func runScriptStepOut(request Request) Result {
	return runScriptDebugControl("script step-out --show <path>", request, func(b *script.DebugBridge) error { return b.StepOut() })
}
