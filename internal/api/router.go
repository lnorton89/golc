package api

import (
	"fmt"
	"sort"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// apiPathPrefix is the D-02 URL-path version prefix every operation this
// package registers is mounted under.
const apiPathPrefix = "/v1"

// Executor is the sole seam through which this package reaches domain
// logic (CONTEXT Phase Boundary / D-01): every translated request is
// dispatched through Execute, never through a direct import of a domain
// package. route is the exact normalized route key the daemon-side
// command registry expects (e.g. "config inspect"); args are the built
// argv-shaped arguments (translate.go's buildArgs); root is the
// repository root the invocation operates on. The returned
// (exitCode, stdout, stderr) triple mirrors the execution package's own
// Result shape field-for-field without this package importing that
// package's type.
type Executor interface {
	Execute(route string, args []string, root string) (exitCode int, stdout, stderr []byte)
}

// OperationRegistration binds one REST operation to the single routed
// command it translates into (Register wires the operation itself onto a
// live huma.API for a specific *Server). Route is recorded eagerly at
// package-init time (RegisterOperation), before any Server exists, so the
// capability-coverage gate can read the full covered-route set without
// constructing a server.
type OperationRegistration struct {
	// Route is the routed command key this operation maps to, e.g.
	// "config inspect". Every OperationRegistration must map to exactly
	// one route, and every route may be claimed by at most one
	// registration (RegisterOperation panics on a second claim) so a
	// route can never silently double-map.
	Route string
	// Register wires this operation's Huma registration onto humaAPI for
	// the given *Server.
	Register func(humaAPI huma.API, server *Server)
}

// operationRegistrations collects every package-level self-registration
// made through RegisterOperation (mirrors the CLI command-execution
// package's own MustDeclareRoute self-registration idiom, 07-PATTERNS.md
// "Shared Patterns: Self-registration idiom").
var operationRegistrations []OperationRegistration

// RegisterOperation is the self-registration entrypoint every operation
// declaration in this package calls from a package-level var initializer:
//
//	var _ = api.RegisterOperation(api.OperationRegistration{...})
//
// It validates the declaration immediately and panics on a route claimed
// twice, so a route can never silently double-map to two REST operations.
func RegisterOperation(registration OperationRegistration) OperationRegistration {
	if registration.Route == "" {
		panic("GOLC_API_OPERATION_INVALID: Route is empty")
	}
	if registration.Register == nil {
		panic("GOLC_API_OPERATION_INVALID: Register is nil")
	}
	for _, existing := range operationRegistrations {
		if existing.Route == registration.Route {
			panic(fmt.Sprintf("GOLC_API_ROUTE_DOUBLE_MAPPED: %q is already registered by another operation", registration.Route))
		}
	}
	operationRegistrations = append(operationRegistrations, registration)
	return registration
}

// RegisteredRoutes returns, sorted, every routed command currently mapped
// to a REST operation -- the capability-coverage gate's covered-route set
// (coverage_test.go), available without constructing a *Server.
func RegisteredRoutes() []string {
	names := make([]string, 0, len(operationRegistrations))
	for _, registration := range operationRegistrations {
		names = append(names, registration.Route)
	}
	sort.Strings(names)
	return names
}

// buildRouter constructs the Chi router carrying server's operations:
// chi/middleware.RequestID seeds a correlation id, and chi/middleware.
// Recoverer guarantees a panicking handler can never crash the daemon
// process that also owns deterministic playback and Art-Net output
// (T-07-01, ARTN-04 invariant). humachi.New layers Huma's OpenAPI-typed
// request/response handling on top without replacing Chi (07-RESEARCH.md
// D-04). Every currently self-registered operation
// (operationRegistrations) is wired onto the resulting huma.API.
func buildRouter(server *Server) chi.Router {
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.Recoverer)

	humaAPI := humachi.New(router, huma.DefaultConfig("GOLC API", "1.0.0"))
	for _, registration := range operationRegistrations {
		registration.Register(humaAPI, server)
	}
	return router
}
