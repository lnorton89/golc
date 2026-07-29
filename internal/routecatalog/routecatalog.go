// Package routecatalog is a thin, test-only bridge onto the CLI
// command-execution package's registry (07-RESEARCH.md Pattern 1,
// 07-PATTERNS.md "Core dispatch pattern"). internal/api's production code
// must never import that package directly -- doing so would close the
// command<->api<->artnet import cycle 07-02-PLAN.md's Executor-interface
// design exists to avoid -- but internal/api's external test package still
// needs live, ground-truth access to the full registered-route set (for
// the capability-coverage gate) and to real command execution (for the
// HTTP<->CLI parity gate). This package is that seam: it lives outside
// internal/api, so nothing under internal/api/ ever spells out the
// execution package's import path, and it is imported only by
// internal/api's *_test.go files, never by any production package.
package routecatalog

import (
	"fmt"
	"sort"
	"strings"

	"github.com/lnorton89/golc/internal/command"
)

// Registry wraps a freshly built, real command registry for read-only
// route enumeration and direct execution.
type Registry struct {
	registry *command.CommandRegistry
}

// New builds a Registry from the production command declarations (the
// exact same set every CLI invocation resolves against).
func New() (*Registry, error) {
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		return nil, err
	}
	return &Registry{registry: registry}, nil
}

// Names returns every registered route's normalized key, sorted -- the
// coverage gate's ground truth for "every public route is mapped or
// excluded."
func (r *Registry) Names() []string {
	registrations := r.registry.Routes()
	names := make([]string, 0, len(registrations))
	for _, registration := range registrations {
		names = append(names, registration.Route)
	}
	sort.Strings(names)
	return names
}

// Execute runs one route through the real registry and returns its
// outcome as the same (exitCode, stdout, stderr) shape internal/api's
// Executor interface declares, so a *Registry can be handed directly to
// api.NewServer in tests without internal/api ever needing to know this
// package -- let alone the command package -- exists.
//
// Unlike command.CommandRegistry.Execute (which re-derives the route by
// word-matching the front of a single flat Args slice), this method
// resolves route directly via Lookup and passes args to the handler
// unmodified -- exactly the split shape api.Executor's own three
// parameters describe, and the same split every translate.go operation
// already builds.
func (r *Registry) Execute(route string, args []string, root string) (exitCode int, stdout, stderr []byte) {
	registration, rest, ok := r.registry.Lookup(strings.Fields(route))
	if !ok || len(rest) != 0 {
		return 2, nil, fmt.Appendf(nil, "GOLC_ROUTE_UNKNOWN: no registered route matches %q\n", route)
	}
	result := registration.Handler(command.Request{Route: registration.Route, Args: args, Root: root})
	return result.ExitCode, result.Stdout, result.Stderr
}
