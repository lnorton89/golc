// show_diagnose.go registers "show diagnose" and "show export" on the
// already-declared "show" scope (deployment.go declares the scope and
// "show inspect"; this file must NOT call MustDeclareScope("show") again --
// a duplicate scope registration panics the router at
// NewDefaultCommandRegistry). Both routes are read-only: they load through
// show.Diagnose/show.LoadForRead, which tolerate a newer-than-supported
// .golc format (CONTEXT D-10) without ever rewriting the file, and neither
// runs automatically on "show open" (D-12) -- both are explicit, on-demand
// commands. "show diagnose" combines file-level PRAGMA integrity_check
// with structural validate() (D-11) into one DiagnosticReport; "show
// export" prints the FULL canonical State document byte-identical to
// strictjson.CanonicalEncode(state) (D-13), never
// buildShowInspectView's allowlisted show-inspect projection.
package command

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lnorton89/golc/internal/show"
	"github.com/lnorton89/golc/internal/strictjson"
)

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "show diagnose",
	Summary: "Run combined file-level (PRAGMA integrity_check) and structural (validate()) diagnostics on a .golc file: show diagnose --show <path>.",
	Handler: runShowDiagnose,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "show export",
	Summary: "Print the full canonical, round-trippable JSON document for a .golc file: show export --show <path>.",
	Handler: runShowExport,
})

// parseShowPathArg is the shared "--show <path>"-only argument parser
// (both --flag value and --flag=value forms) parseShowDiagnoseArgs/
// parseShowExportArgs delegate to -- the identical shape
// parseShowInspectArgs already established in deployment.go, duplicated
// here under this file's own GOLC_SHOW_DIAGNOSE_USAGE code since "show
// diagnose"/"show export" are declared in this file, not deployment.go.
func parseShowPathArg(usage string, args []string) (showPath string, err error) {
	for i := 0; i < len(args); {
		argument := args[i]
		switch {
		case argument == "--show":
			if i+1 >= len(args) {
				return "", fmt.Errorf("GOLC_SHOW_DIAGNOSE_USAGE: --show requires a path; usage: %s", usage)
			}
			showPath = args[i+1]
			i += 2
		case strings.HasPrefix(argument, "--show="):
			showPath = strings.TrimPrefix(argument, "--show=")
			i++
		default:
			return "", fmt.Errorf("GOLC_SHOW_DIAGNOSE_USAGE: unsupported argument %q; usage: %s", argument, usage)
		}
	}
	if showPath == "" {
		return "", fmt.Errorf("GOLC_SHOW_DIAGNOSE_USAGE: --show is required; usage: %s", usage)
	}
	return showPath, nil
}

// parseShowDiagnoseArgs accepts exactly a required "--show <path>" for
// "show diagnose".
func parseShowDiagnoseArgs(usage string, args []string) (showPath string, err error) {
	return parseShowPathArg(usage, args)
}

// parseShowExportArgs accepts exactly a required "--show <path>" for
// "show export".
func parseShowExportArgs(usage string, args []string) (showPath string, err error) {
	return parseShowPathArg(usage, args)
}

// runShowDiagnose serves the self-registered "show diagnose" route:
// combined file-level + structural diagnostics (D-11), read-only,
// tolerating a newer-than-supported format (D-10). Exit 0 when healthy,
// exit 1 when issues are found so scripts can gate on it, exit 2 on a
// usage error -- never a crash even for a corrupted .golc file
// (show.Diagnose's own never-crash contract).
func runShowDiagnose(request Request) Result {
	showPath, err := parseShowDiagnoseArgs("show diagnose --show <path>", request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	report, err := show.Diagnose(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	payload, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_SHOW_DIAGNOSE_FAILED: %v\n", err))}
	}
	payload = append(payload, '\n')

	if len(report.FileLevelIssues) > 0 || !report.StructuralOK {
		return Result{ExitCode: 1, Stdout: payload}
	}
	return Result{Stdout: payload}
}

// runShowExport serves the self-registered "show export" route: load the
// ShowState at --show read-only (show.LoadForRead, tolerating a
// newer-than-supported format via D-10) and print
// strictjson.CanonicalEncode(state) directly -- the full canonical
// document (D-13), never buildShowInspectView's allowlisted projection.
func runShowExport(request Request) Result {
	showPath, err := parseShowExportArgs("show export --show <path>", request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	state, err := show.LoadForRead(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	payload, err := strictjson.CanonicalEncode(state)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_SHOW_EXPORT_ENCODE_FAILED: %v\n", err))}
	}
	return Result{Stdout: payload}
}
