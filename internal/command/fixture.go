// fixture.go is the fixture command file: it owns the "fixture" routing
// scope and self-registers the "fixture validate" route (CONTEXT
// D-01/D-02/D-04): a show author hand-writes a fixture definition YAML
// file and this route strictly decodes and validates it, printing a
// deterministic canonical summary on success. It performs no
// scaffold/generator behavior (D-02: validate-only) and no network
// access.
package command

import (
	"fmt"
	"os"
	"strings"

	"github.com/lnorton89/golc/internal/fixture"
	"github.com/lnorton89/golc/internal/strictjson"
)

var _ = MustDeclareScope(ScopeRegistration{
	Scope:   "fixture",
	Summary: "Hand-authored YAML fixture definition validation.",
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "fixture validate",
	Summary: "Strictly decode and validate a hand-authored YAML fixture definition, printing its canonical summary on success: fixture validate <file>.",
	Handler: runFixtureValidate,
})

// runFixtureValidate serves the self-registered "fixture validate" route.
// It never writes anything (D-02/D-03: validate-only, file-level share):
// success and failure alike only read the given file and print a result.
func runFixtureValidate(request Request) Result {
	path, err := parseFixtureValidateArgs("fixture validate <file>", request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	data, err := os.ReadFile(resolveWritablePath(request.Root, path))
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(fmt.Sprintf("GOLC_FIXTURE_READ_FAILED: %v\n", err))}
	}

	def, err := fixture.Decode(data)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	summary, err := strictjson.CanonicalEncode(def)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(fmt.Sprintf("GOLC_FIXTURE_ENCODE_FAILED: %v\n", err))}
	}
	return Result{Stdout: summary}
}

// parseFixtureValidateArgs accepts exactly one positional file path: not
// missing, not more than one, and never a dash-prefixed word (router.go's
// route-word grammar already rejects a dash-prefixed route word, but the
// remaining positional argument is handler-owned, so this handler rejects
// it explicitly too).
func parseFixtureValidateArgs(usage string, args []string) (string, error) {
	if len(args) != 1 {
		return "", fmt.Errorf("GOLC_FIXTURE_USAGE: usage: %s", usage)
	}
	if strings.HasPrefix(args[0], "-") {
		return "", fmt.Errorf("GOLC_FIXTURE_USAGE: usage: %s", usage)
	}
	return args[0], nil
}
