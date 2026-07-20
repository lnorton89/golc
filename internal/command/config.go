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

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "config set",
	Summary: "Persist one allowlisted value to the ignored machine-local layer: config set --local <key> <value>.",
	Handler: runConfigSet,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "config explain",
	Summary: "Print deterministic safe provenance for one canonical key: config explain <key> [--format json].",
	Handler: runConfigExplain,
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

// runConfigSet serves the self-registered "config set" route. Only the
// --local target exists in this plan (D-06: the untracked project-local
// layer); WriteLocal owns every containment and allowlist gate.
func runConfigSet(request Request) Result {
	key, value, err := parseSetArgs(request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}
	if err := projectconfig.WriteLocal(request.Root, key, value); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	confirmation := fmt.Sprintf("local: %s = %q written to golc.local.toml\n", key, value)
	return Result{Stdout: []byte(confirmation)}
}

// parseSetArgs accepts exactly the supported local form:
// config set --local <key> <value>.
func parseSetArgs(args []string) (string, string, error) {
	local := false
	positionals := []string{}
	for _, argument := range args {
		switch {
		case argument == "--local":
			local = true
		case strings.HasPrefix(argument, "-"):
			return "", "", fmt.Errorf("GOLC_CONFIG_USAGE: unknown flag %q", argument)
		default:
			positionals = append(positionals, argument)
		}
	}
	if !local || len(positionals) != 2 {
		return "", "", fmt.Errorf("GOLC_CONFIG_USAGE: usage: config set --local <key> <value>")
	}
	return positionals[0], positionals[1], nil
}

// runConfigExplain serves the self-registered "config explain" route with
// deterministic safe provenance JSON (D-07).
func runConfigExplain(request Request) Result {
	key, err := parseExplainArgs(request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}
	payload, err := projectconfig.Explain(request.Root, key)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	return Result{Stdout: payload}
}

// parseExplainArgs accepts exactly one canonical key plus an optional
// "--format json" (the only supported format).
func parseExplainArgs(args []string) (string, error) {
	key := ""
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
			if key != "" {
				return "", fmt.Errorf("GOLC_CONFIG_USAGE: exactly one configuration key is required")
			}
			key = argument
			i++
		}
	}
	if key == "" {
		return "", fmt.Errorf("GOLC_CONFIG_USAGE: usage: config explain <key> [--format json]")
	}
	if format != "json" {
		return "", fmt.Errorf("GOLC_CONFIG_FORMAT_UNSUPPORTED: %q (only json is supported)", format)
	}
	return key, nil
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
