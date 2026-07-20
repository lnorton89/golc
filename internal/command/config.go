// config.go is the config command file: it owns the "config" scope and
// self-registers the exact config routes through the package declaration
// entrypoints (CONTEXT D-03) — the central router is never edited. The
// handlers delegate to internal/projectconfig, which stays a pure
// configuration library (no command import), so this file is the single
// CLI surface over the root-index/concern data layer (D-05).
package command

import (
	"fmt"
	"strings"

	"github.com/lnorton89/golc/internal/projectconfig"
)

var _ = MustDeclareScope(ScopeRegistration{
	Scope:   "config",
	Summary: "Project configuration index and concern operations.",
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "config inspect",
	Summary: "Print one indexed configuration concern as deterministic JSON.",
	Handler: runConfigInspect,
})

// runConfigInspect serves the self-registered "config inspect" route.
func runConfigInspect(request Request) Result {
	concernID, err := parseInspectArgs(request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}
	payload, err := projectconfig.InspectConcern(request.Root, concernID)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	return Result{Stdout: payload}
}

// parseInspectArgs accepts exactly one concern id plus an optional
// "--format json" (the only supported format).
func parseInspectArgs(args []string) (string, error) {
	concernID := ""
	format := "json"
	for i := 0; i < len(args); {
		argument := args[i]
		switch {
		case argument == "--format":
			if i+1 >= len(args) {
				return "", fmt.Errorf("GOLC_CONFIG_USAGE: --format requires a value")
			}
			format = args[i+1]
			i += 2
		case strings.HasPrefix(argument, "--format="):
			format = strings.TrimPrefix(argument, "--format=")
			i++
		case strings.HasPrefix(argument, "-"):
			return "", fmt.Errorf("GOLC_CONFIG_USAGE: unknown flag %q", argument)
		default:
			if concernID != "" {
				return "", fmt.Errorf("GOLC_CONFIG_USAGE: exactly one concern id is required")
			}
			concernID = argument
			i++
		}
	}
	if concernID == "" {
		return "", fmt.Errorf("GOLC_CONFIG_USAGE: usage: config inspect <concern> [--format json]")
	}
	if format != "json" {
		return "", fmt.Errorf("GOLC_CONFIG_FORMAT_UNSUPPORTED: %q (only json is supported)", format)
	}
	return concernID, nil
}
