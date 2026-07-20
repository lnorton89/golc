// Package command implements the GOLC route/scope self-registration
// contract (CONTEXT D-03): each command file declares its exact route with
// MustDeclareRoute and each owning command graph declares its scope with
// MustDeclareScope from package-level var initializers. The entrypoint
// builds the default registry from those declarations, so later commands
// become reachable without editing this file or any central switch.
//
// Keys are normalized exact strings (lowercased, single-space-joined
// words). Duplicate routes or scopes fail deterministically with stable
// diagnostics before any handler executes, and introspection order is
// byte-stable regardless of declaration order.
package command

import (
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
)

// Request carries one routed invocation to a handler.
type Request struct {
	// Route is the normalized route key that matched, e.g. "config inspect".
	Route string
	// Args holds the arguments remaining after the route words.
	Args []string
	// Root is the absolute repository root the invocation operates on.
	Root string
}

// Result is a handler outcome with a stable result-to-exit mapping:
// ExitCode 0 means success, 1 means the command failed, and 2 means the
// invocation could not be routed or was malformed.
type Result struct {
	ExitCode int
	Stdout   []byte
	Stderr   []byte
}

// CommandHandler executes one routed request.
type CommandHandler func(Request) Result

// CommandRegistration binds one exact normalized route to its handler.
type CommandRegistration struct {
	Route   string
	Summary string
	Handler CommandHandler
}

// ScopeRegistration declares one exact normalized ownership scope (the
// first route word) for a command graph.
type ScopeRegistration struct {
	Scope   string
	Summary string
}

var routeWordPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// normalizeKey lowercases, trims, and single-space-joins the words of a
// route or scope name so lookup keys are exact and byte-stable.
func normalizeKey(name string) (string, error) {
	words := strings.Fields(strings.ToLower(name))
	if len(words) == 0 {
		return "", fmt.Errorf("name is empty")
	}
	for _, word := range words {
		if !routeWordPattern.MatchString(word) {
			return "", fmt.Errorf("word %q is not a valid route word", word)
		}
	}
	return strings.Join(words, " "), nil
}

// CommandRegistry resolves normalized exact route keys to handlers.
type CommandRegistry struct {
	routes map[string]CommandRegistration
	scopes map[string]ScopeRegistration
}

// NewCommandRegistry returns an empty registry.
func NewCommandRegistry() *CommandRegistry {
	return &CommandRegistry{
		routes: map[string]CommandRegistration{},
		scopes: map[string]ScopeRegistration{},
	}
}

// RegisterScope adds one scope, rejecting duplicates deterministically.
func (r *CommandRegistry) RegisterScope(registration ScopeRegistration) error {
	key, err := normalizeKey(registration.Scope)
	if err != nil {
		return fmt.Errorf("GOLC_SCOPE_INVALID: %q: %v", registration.Scope, err)
	}
	if strings.Contains(key, " ") {
		return fmt.Errorf("GOLC_SCOPE_INVALID: %q: a scope is a single word", registration.Scope)
	}
	if _, exists := r.scopes[key]; exists {
		return fmt.Errorf("GOLC_SCOPE_DUPLICATE: %s", key)
	}
	registration.Scope = key
	r.scopes[key] = registration
	return nil
}

// RegisterRoute adds one route. The route's owning scope (its first word)
// must already be registered, and duplicates are rejected with a stable
// diagnostic before any handler could execute.
func (r *CommandRegistry) RegisterRoute(registration CommandRegistration) error {
	key, err := normalizeKey(registration.Route)
	if err != nil {
		return fmt.Errorf("GOLC_ROUTE_INVALID: %q: %v", registration.Route, err)
	}
	if registration.Handler == nil {
		return fmt.Errorf("GOLC_ROUTE_INVALID: %s: handler is nil", key)
	}
	scope := strings.SplitN(key, " ", 2)[0]
	if _, declared := r.scopes[scope]; !declared {
		return fmt.Errorf("GOLC_ROUTE_SCOPE_UNDECLARED: route %q requires scope %q to be declared", key, scope)
	}
	if _, exists := r.routes[key]; exists {
		return fmt.Errorf("GOLC_ROUTE_DUPLICATE: %s", key)
	}
	registration.Route = key
	r.routes[key] = registration
	return nil
}

// Lookup resolves argument words to the longest exactly-matching declared
// route and returns the remaining arguments. Resolution depends only on
// the normalized exact keys, never on declaration order.
func (r *CommandRegistry) Lookup(words []string) (CommandRegistration, []string, bool) {
	for length := len(words); length >= 1; length-- {
		key, err := normalizeKey(strings.Join(words[:length], " "))
		if err != nil {
			continue
		}
		if registration, ok := r.routes[key]; ok {
			rest := append([]string(nil), words[length:]...)
			return registration, rest, true
		}
	}
	return CommandRegistration{}, nil, false
}

// Routes returns every registration sorted by normalized route key.
func (r *CommandRegistry) Routes() []CommandRegistration {
	keys := make([]string, 0, len(r.routes))
	for key := range r.routes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	registrations := make([]CommandRegistration, 0, len(keys))
	for _, key := range keys {
		registrations = append(registrations, r.routes[key])
	}
	return registrations
}

// Scopes returns every scope sorted by normalized scope key.
func (r *CommandRegistry) Scopes() []ScopeRegistration {
	keys := make([]string, 0, len(r.scopes))
	for key := range r.scopes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	registrations := make([]ScopeRegistration, 0, len(keys))
	for _, key := range keys {
		registrations = append(registrations, r.scopes[key])
	}
	return registrations
}

// Execute routes one invocation and runs its handler. Unroutable
// invocations fail with a stable diagnostic and exit code 2.
func (r *CommandRegistry) Execute(request Request) Result {
	registration, rest, ok := r.Lookup(request.Args)
	if !ok {
		diagnostic := fmt.Sprintf("GOLC_ROUTE_UNKNOWN: no registered route matches %q\n", strings.Join(request.Args, " "))
		return Result{ExitCode: 2, Stderr: []byte(diagnostic)}
	}
	return registration.Handler(Request{
		Route: registration.Route,
		Args:  rest,
		Root:  request.Root,
	})
}

// WriteResult emits a result's raw bytes and returns its exit code. If an
// otherwise-successful result cannot be written, the exit code becomes 1
// so truncated output is never reported as success.
func WriteResult(stdout, stderr io.Writer, result Result) int {
	exitCode := result.ExitCode
	if len(result.Stdout) > 0 {
		if _, err := stdout.Write(result.Stdout); err != nil && exitCode == 0 {
			exitCode = 1
		}
	}
	if len(result.Stderr) > 0 {
		if _, err := stderr.Write(result.Stderr); err != nil && exitCode == 0 {
			exitCode = 1
		}
	}
	return exitCode
}

// declaredRoutes and declaredScopes collect the package-level declarations
// made by command files through MustDeclareRoute and MustDeclareScope.
var (
	declaredRoutes []CommandRegistration
	declaredScopes []ScopeRegistration
)

// MustDeclareRoute is the compile-safe self-registration entrypoint every
// command file calls from a package-level var initializer:
//
//	var _ = command.MustDeclareRoute(command.CommandRegistration{...})
//
// It validates the declaration shape immediately; duplicate normalized
// routes across files fail deterministically when the default registry is
// built, before any handler executes.
func MustDeclareRoute(registration CommandRegistration) CommandRegistration {
	key, err := normalizeKey(registration.Route)
	if err != nil {
		panic(fmt.Sprintf("GOLC_ROUTE_INVALID: %q: %v", registration.Route, err))
	}
	if registration.Handler == nil {
		panic(fmt.Sprintf("GOLC_ROUTE_INVALID: %s: handler is nil", key))
	}
	registration.Route = key
	declaredRoutes = append(declaredRoutes, registration)
	return registration
}

// MustDeclareScope is the compile-safe scope declaration entrypoint each
// owning command graph calls once from a package-level var initializer.
func MustDeclareScope(registration ScopeRegistration) ScopeRegistration {
	key, err := normalizeKey(registration.Scope)
	if err != nil {
		panic(fmt.Sprintf("GOLC_SCOPE_INVALID: %q: %v", registration.Scope, err))
	}
	if strings.Contains(key, " ") {
		panic(fmt.Sprintf("GOLC_SCOPE_INVALID: %q: a scope is a single word", registration.Scope))
	}
	registration.Scope = key
	declaredScopes = append(declaredScopes, registration)
	return registration
}

// NewDefaultCommandRegistry builds the registry every entrypoint uses from
// the package-level declarations. Scopes register before routes and both
// are processed in sorted key order, so the outcome — including duplicate
// rejection — is deterministic regardless of declaration order across
// files.
func NewDefaultCommandRegistry() (*CommandRegistry, error) {
	registry := NewCommandRegistry()

	scopes := append([]ScopeRegistration(nil), declaredScopes...)
	sort.SliceStable(scopes, func(i, j int) bool { return scopes[i].Scope < scopes[j].Scope })
	for _, registration := range scopes {
		if err := registry.RegisterScope(registration); err != nil {
			return nil, err
		}
	}

	routes := append([]CommandRegistration(nil), declaredRoutes...)
	sort.SliceStable(routes, func(i, j int) bool { return routes[i].Route < routes[j].Route })
	for _, registration := range routes {
		if err := registry.RegisterRoute(registration); err != nil {
			return nil, err
		}
	}
	return registry, nil
}
