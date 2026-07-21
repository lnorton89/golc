// generate.go is the generate command file: it owns the "generate" scope
// and self-registers the exact generate route through the package
// declaration entrypoints (CONTEXT D-03/D-08) — the central router is
// never edited. The handler delegates to internal/contracts, which stays
// the exclusive deterministic schema registry/generator owner (Plan 04);
// this file only exposes it as a reachable offline command.
//
// Route registration follows the established dash-word precedent
// (internal/command/test.go's "test" route accepting exactly
// "--quick --scope <name>"): router.go's route-word grammar rejects any
// word beginning with "-", so a flag can never itself be a route word.
// One exact route, "generate", is declared here; its handler strictly
// accepts either no arguments (write every committed schema) or exactly
// "--check" (report drift without writing) and rejects anything else with
// a usage diagnostic. The user-facing commands "generate" and
// "generate --check" are both exact and reachable through this single
// registration, mirroring D-10's contributor/CI parity.
package command

import (
	"fmt"
	"strings"

	"github.com/lnorton89/golc/internal/contracts"
)

var _ = MustDeclareScope(ScopeRegistration{
	Scope:   "generate",
	Summary: "Deterministic committed-schema generation and drift checking.",
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "generate",
	Summary: "Write every registered schema, or report drift without writing: generate [--check].",
	Handler: runGenerate,
})

// runGenerate serves the self-registered "generate" route. It never opens
// a network connection or reads .env: internal/contracts is pure
// filesystem/reflection code over the repository-relative committed paths
// each descriptor declares.
func runGenerate(request Request) Result {
	checkOnly, err := parseGenerateArgs(request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	if checkOnly {
		changed, err := contracts.CheckDrift(request.Root)
		if err != nil {
			return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
		}
		if len(changed) > 0 {
			message := fmt.Sprintf("GOLC_GENERATE_DRIFT: %s\n", strings.Join(changed, ", "))
			return Result{ExitCode: 1, Stderr: []byte(message)}
		}
		return Result{Stdout: []byte("generate --check: no drift; every committed schema matches its source.\n")}
	}

	if err := contracts.GenerateAll(request.Root); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	return Result{Stdout: []byte("generate: every registered schema written to its committed path.\n")}
}

// parseGenerateArgs accepts exactly no arguments or exactly "--check".
func parseGenerateArgs(args []string) (bool, error) {
	checkOnly := false
	for _, argument := range args {
		switch argument {
		case "--check":
			if checkOnly {
				return false, fmt.Errorf("GOLC_GENERATE_USAGE: --check may be given only once")
			}
			checkOnly = true
		default:
			return false, fmt.Errorf("GOLC_GENERATE_USAGE: unsupported argument %q; usage: generate [--check]", argument)
		}
	}
	return checkOnly, nil
}
