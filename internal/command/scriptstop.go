// scriptstop.go is the "script stop" command file (08-06-PLAN.md Task 3,
// CONTEXT D-10/D-11/D-13): a separate, lightweight action scoped to
// exactly one named script's active run. D-10 is explicit that this is
// distinct from Phase 6's global Revoke Automation action (which blocks
// every script/AI action and freezes the current look across the whole
// rig): Stop never touches live output or any other subsystem's state
// and never iterates other runs -- it terminates exactly one run.
//
// There is no restart path anywhere in this file or in internal/script:
// no retry loop, no backoff, no supervisor goroutine that re-spawns a
// stopped run (D-13). A crashed, blocked, or user-stopped script always
// requires an explicit new "script run" invocation -- Phase 8 grows no
// autonomous-looping behaviour here; that belongs to a later bounded-
// autonomy model.
//
// Stop resolves its target through internal/script's process-global
// active-run registry (script.ActiveRun), not through a Host or Executor
// of its own: it only ever needs to terminate an already-running
// process, never to dispatch a new SDK call.
package command

import (
	"fmt"
	"time"

	"github.com/lnorton89/golc/internal/script"
	"github.com/lnorton89/golc/internal/show"
	"github.com/lnorton89/golc/internal/strictjson"
)

var _ = MustDeclareRoute(CommandRegistration{
	Route: "script stop",
	Summary: "Stop the active run of a named script immediately, no confirmation gesture required (D-10 -- a " +
		"separate, single-run-scoped action, distinct from Phase 6's global automation-revoke control): " +
		"script stop <name> --show <path>.",
	Handler: runScriptStop,
})

// runScriptStop serves the self-registered "script stop" route: resolve
// the named script's active run from internal/script's active-run
// registry (GOLC_SCRIPT_NO_ACTIVE_RUN, unchanged state, if none exists),
// terminate it with reason GOLC_SCRIPT_STOPPED_BY_USER, and block until
// the run's own goroutine finalizes its outcome (script.Run.Stop, D-11:
// any command already accepted by the command registry is allowed to
// finish before the run's outcome is recorded). The persisted script's
// LastRunStatus/LastRunReason/LastRunAt are then written back exactly
// like "script run"'s own completion path -- both writers converge on
// the identical outcome values, so a race between this route and the
// still-unwinding "script run" invocation's own post-Run persistence is
// always idempotent. Exit 0 on a successful stop, 1 on
// GOLC_SCRIPT_NO_ACTIVE_RUN or a show load/save failure, 2 on a
// malformed invocation.
func runScriptStop(request Request) Result {
	usage := "script stop <name> --show <path>"
	name, flags, err := parseScriptPositionalArgs(usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}
	if err := rejectUnknownScriptFlags(usage, flags, map[string]bool{"show": true}); err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}
	showPath, ok := flags["show"]
	if !ok || showPath == "" {
		return Result{ExitCode: 2, Stderr: []byte(fmt.Sprintf("GOLC_SCRIPT_USAGE: --show is required; usage: %s\n", usage))}
	}

	run, found := script.ActiveRun(name)
	if !found {
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_SCRIPT_NO_ACTIVE_RUN: no active run for script %q\n", name))}
	}

	outcome := run.Stop(script.TerminationReason{
		Code:    "GOLC_SCRIPT_STOPPED_BY_USER",
		Message: fmt.Sprintf("script %q was stopped by user request", name),
		At:      time.Now(),
	})

	state, err := show.Load(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	if target, index, targetFound := scriptByName(state.Scripts, name); targetFound {
		target.LastRunStatus = outcome.Status
		target.LastRunReason = outcome.Reason
		target.LastRunAt = time.Now().UTC().Format(time.RFC3339)
		state.Scripts[index] = target
		if err := show.Save(request.Root, showPath, state); err != nil {
			return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
		}
	}

	payload, encodeErr := strictjson.CanonicalEncode(toScriptRunResultView(outcome))
	if encodeErr != nil {
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_SCRIPT_ENCODE_FAILED: %v\n", encodeErr))}
	}
	return Result{Stdout: payload}
}
