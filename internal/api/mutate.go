// mutate.go implements the serialized mutation pipeline every mutating
// REST operation in this package funnels through (07-05-PLAN.md Task 1,
// CONTEXT D-08/D-13, 07-RESEARCH.md Pitfall 2): (1) require the route's
// declared coarse domain scope from the authenticated request context
// (D-08, else 403); (2) if an If-Match header is present, compare it
// against show.CurrentRevision (D-13, else 412); (3) call the injected
// Executor; (4) read the resulting revision; (5) fire the post-mutation
// observer seam (observer.go) with the outcome. Every mutating request is
// serialized behind mutationMutex so this package never becomes a second
// source of concurrent SQLite writers beyond what busy_timeout already
// tolerates from the CLI/Wails processes (07-RESEARCH.md Pitfall 2) --
// this also gives dry-run (dryrun.go) and idempotency (idempotency.go) a
// single choke point to hook into without their own separate locking.
package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/lnorton89/golc/internal/show"
)

// mutationMutex is the single critical-section lock every mutating
// request (real, dry-run, or idempotent replay) is serialized behind
// (07-RESEARCH.md Pitfall 2). It is a package-level var, not a per-Server
// field, matching this package's existing single-process-per-daemon
// assumption (mirrors router.go's own operationRegistrations idiom) --
// exactly one *Server is ever constructed per daemon process (D-07).
var mutationMutex sync.Mutex

// domainScope maps a mutating route's domain (its first word) to the
// coarse D-08 scope required to invoke any route in that domain
// (playback, authoring, or admin) -- the documented map Task 1's action
// requires. A domain with no entry here is refused with
// GOLC_API_DOMAIN_SCOPE_UNDECLARED rather than silently defaulting to the
// most permissive scope: every mutating route this package ever registers
// must have an explicit, reviewed scope assignment.
var domainScope = map[string]show.APIKeyScope{
	"pool":            show.APIKeyScopeAuthoring,
	"fixture":         show.APIKeyScopeAuthoring,
	"deployment":      show.APIKeyScopeAuthoring,
	"scene":           show.APIKeyScopeAuthoring,
	"blend":           show.APIKeyScopeAuthoring,
	"chase":           show.APIKeyScopeAuthoring,
	"motion":          show.APIKeyScopeAuthoring,
	"theme":           show.APIKeyScopeAuthoring,
	"preset":          show.APIKeyScopeAuthoring,
	"programmer":      show.APIKeyScopeAuthoring,
	"operatorsurface": show.APIKeyScopeAuthoring,
	"show":            show.APIKeyScopeAuthoring,
	"playback":        show.APIKeyScopePlayback,
	"artnet":          show.APIKeyScopePlayback,
	"config":          show.APIKeyScopeAdmin,
	"api-key":         show.APIKeyScopeAdmin,
}

// requiredScopeForRoute resolves the D-08 coarse scope route's domain
// (its first space-separated word) requires.
func requiredScopeForRoute(route string) (show.APIKeyScope, error) {
	domain := route
	if idx := strings.IndexByte(route, ' '); idx >= 0 {
		domain = route[:idx]
	}
	scope, ok := domainScope[domain]
	if !ok {
		return "", fmt.Errorf("GOLC_API_DOMAIN_SCOPE_UNDECLARED: no coarse scope is declared for domain %q", domain)
	}
	return scope, nil
}

// validateListValues is the single boundary rule for every list-valued
// field this package forwards to a downstream comma-joined CLI flag (e.g.
// registerCreatePool's "--requires" argument below, and batch.go's
// translateBatchCreatePool, which calls this same helper before its own
// join): it returns a typed 400 naming field and the offending value for
// any element of values containing a comma, since the comma is the
// reserved delimiter strings.Join uses to flatten the list into a single
// CLI argument -- a value that contains one cannot be represented
// faithfully in that encoding and would silently split into two
// downstream values if allowed through (IN-02, 07-REVIEW.md). Rejection is
// chosen over introducing an escaping scheme, because the current
// vocabularies this package forwards this way (capability types, scope
// names) have no legitimate need for an embedded comma, and an escaping
// scheme would be a wider, unrequested change to the CLI contract.
// Returns nil for a nil or empty values.
func validateListValues(field string, values []string) error {
	for _, value := range values {
		if strings.Contains(value, ",") {
			return huma.Error400BadRequest(fmt.Sprintf(
				"GOLC_API_LIST_VALUE_INVALID: field %q contains a value with a comma (%q); the comma is the reserved delimiter for this field and cannot appear inside a single element",
				field, value))
		}
	}
	return nil
}

// mutateRequest carries everything mutate needs to run one mutating REST
// operation's request through the serialized critical section.
type mutateRequest struct {
	// Route is the routed command key to Execute, e.g. "pool create".
	Route string
	// Args are the business arguments to Execute, WITHOUT the
	// "--show <path>" pair -- mutate appends the correct show path
	// itself (the real daemon path, or a throwaway dry-run copy).
	Args []string
	// IfMatch is the raw If-Match header value, "" if absent.
	IfMatch string
	// DryRun is true when the request carried ?dry_run=true (D-14):
	// mutate branches to dryRunMutate (dryrun.go) before ever comparing
	// If-Match or calling Execute against the real show.
	DryRun bool
	// IdempotencyKey is the raw Idempotency-Key header value, "" if
	// absent (idempotency.go, 07-RESEARCH.md Assumptions Log A6). A live
	// stored entry for this key short-circuits the pipeline before
	// If-Match/Execute, returning the original response.
	IdempotencyKey string
	// Actor is the authenticated key's id (KeyIDFromContext).
	Actor string
	// CorrelationID is the request's chi/middleware.RequestID value.
	CorrelationID string
}

// mutationResult is mutate's structured outcome, carrying the HTTP
// response body Result and the resulting show revision (nil for a
// failure) so callers can build their own typed JSON response envelope
// (translate.go's rawJSONOutput is not used here: unlike the read routes
// this package registered in 07-02, most CLI mutating handlers -- "pool
// create" included -- print a plain-text confirmation line, not JSON;
// mutate.go builds its own small JSON envelope instead of demanding every
// mutating internal/command handler grow a --json flag).
type mutationResult struct {
	// Result is the routed command's raw stdout, trimmed.
	Result string
	// Revision is show.State.Revision immediately after this mutation
	// applied, or nil when it does not apply (failure, dry-run,
	// idempotent replay against a stored response).
	Revision *int64
}

// buildMutationArgs appends "--show <showPath>" to args, matching
// translate.go's buildShowArgs convention (07-RESEARCH.md Pitfall 3): the
// show path is always injected server-side, by mutate's own caller
// (mutate for a real mutation, dryRunMutate for a throwaway copy), never
// accepted from the request body.
func buildMutationArgs(args []string, showPath string) []string {
	built := make([]string, 0, len(args)+2)
	built = append(built, args...)
	built = append(built, "--show", showPath)
	return built
}

// statusFromHumaErr extracts the HTTP status a typed Huma error (any of
// translate.go's translateResult outcomes, or revision.go's 412/400)
// would produce, for the MutationEvent.StatusCode field -- falling back
// to 500 for a plain Go error that does not carry its own status.
func statusFromHumaErr(err error) int {
	var statusErr huma.StatusError
	if errors.As(err, &statusErr) {
		return statusErr.GetStatus()
	}
	return http.StatusInternalServerError
}

// mutate is the single serialized mutation pipeline every mutating REST
// operation registered in this package calls. See this file's package
// doc comment for the five-step pipeline; observers always fire exactly
// once per attempted mutation, for both success and failure (Task 1's
// own behavior requirement) -- an undeclared-scope lookup failure and a
// scope rejection are both reported "failure" without ever acquiring
// mutationMutex (nothing durable was ever at risk on either path), while
// every check from the revision comparison onward runs inside the held
// mutex so audit/SSE ordering matches application order (observer.go's
// own doc comment).
func mutate(ctx context.Context, server *Server, req mutateRequest) (mutationResult, error) {
	requiredScope, scopeErr := requiredScopeForRoute(req.Route)
	if scopeErr != nil {
		// req.Route resolved to a domain with no declared D-08 scope
		// (GOLC_API_DOMAIN_SCOPE_UNDECLARED) -- this is every other early
		// return's sibling (WR-03): without this call, the first mutating
		// route ever wired without a domainScope entry would silently
		// un-audit itself.
		fireMutationObservers(MutationEvent{
			Route: req.Route, Args: req.Args, Actor: req.Actor, Source: "http",
			CorrelationID: req.CorrelationID, Outcome: "failure", StatusCode: http.StatusInternalServerError,
		})
		return mutationResult{}, huma.Error500InternalServerError(scopeErr.Error())
	}
	if err := RequireScope(ctx, requiredScope); err != nil {
		fireMutationObservers(MutationEvent{
			Route: req.Route, Args: req.Args, Actor: req.Actor, Source: "http",
			CorrelationID: req.CorrelationID, Outcome: "failure", StatusCode: http.StatusForbidden,
		})
		return mutationResult{}, err
	}

	mutationMutex.Lock()
	defer mutationMutex.Unlock()

	if req.DryRun {
		return dryRunMutate(server, req)
	}

	if req.IdempotencyKey != "" {
		// Looked up (and, on success below, stored) as the composite
		// (actor, route, key) triple, both while mutationMutex is held --
		// this is what makes idempotency exactly-once under concurrent
		// arrival of the same triple, and what stops one actor's stored
		// response from ever leaking to a different actor or route that
		// merely reused the same client-chosen key string (WR-01).
		if cached, found := server.idempotency.lookup(req.Actor, req.Route, req.IdempotencyKey); found {
			fireMutationObservers(MutationEvent{
				Route: req.Route, Args: req.Args, Actor: req.Actor, Source: "http",
				CorrelationID: req.CorrelationID, ResultingRevision: cached.Revision,
				Outcome: "idempotent_replay", StatusCode: http.StatusOK,
			})
			return cached, nil
		}
	}

	expectedRevision, revisionErr := checkRevision(server.root, server.showPath, req.IfMatch)
	if revisionErr != nil {
		fireMutationObservers(MutationEvent{
			Route: req.Route, Args: req.Args, Actor: req.Actor, Source: "http",
			CorrelationID: req.CorrelationID, ExpectedRevision: expectedRevision,
			Outcome: "failure", StatusCode: statusFromHumaErr(revisionErr),
		})
		return mutationResult{}, revisionErr
	}

	args := buildMutationArgs(req.Args, server.showPath)
	exitCode, stdout, stderr := server.executor.Execute(req.Route, args, server.root)
	body, translateErr := translateResult(exitCode, stdout, stderr)

	outcome := "success"
	statusCode := http.StatusOK
	var resultingRevision *int64
	if translateErr != nil {
		outcome = "failure"
		statusCode = statusFromHumaErr(translateErr)
	} else if current, revErr := show.CurrentRevision(server.root, server.showPath); revErr == nil {
		value := current
		resultingRevision = &value
	}

	fireMutationObservers(MutationEvent{
		Route: req.Route, Args: req.Args, Actor: req.Actor, Source: "http",
		CorrelationID: req.CorrelationID, ExpectedRevision: expectedRevision,
		ResultingRevision: resultingRevision, Outcome: outcome, StatusCode: statusCode,
	})

	if translateErr != nil {
		return mutationResult{}, translateErr
	}
	result := mutationResult{Result: strings.TrimSpace(string(body)), Revision: resultingRevision}
	if req.IdempotencyKey != "" {
		server.idempotency.store(req.Actor, req.Route, req.IdempotencyKey, result)
	}
	return result, nil
}

// actorFromContext returns the authenticated key's id (KeyIDFromContext),
// or "" if ctx carries none -- unreachable for any request that passed
// AuthMiddleware, since auth applies to every /v1 request, but never
// panics if called from a test context that skipped it.
func actorFromContext(ctx context.Context) string {
	if keyID, ok := KeyIDFromContext(ctx); ok {
		return keyID
	}
	return ""
}

// correlationIDFromContext returns router.go's chi/middleware.RequestID
// value for ctx, the correlation id every MutationEvent carries.
func correlationIDFromContext(ctx context.Context) string {
	return middleware.GetReqID(ctx)
}

// --- POST /v1/pools -> "pool create" (authoring scope required) --------

// createPoolInput is POST /v1/pools's Huma input: If-Match carries D-13's
// expected-revision precondition.
type createPoolInput struct {
	IfMatch        string `header:"If-Match" doc:"Expected show.State.Revision, quoted per RFC 7232 (D-13). Omit to skip the optimistic-concurrency check."`
	DryRun         bool   `query:"dry_run" doc:"Preview this mutation's effect without applying it (D-14); the real show is never touched, and no resulting revision is reported."`
	IdempotencyKey string `header:"Idempotency-Key" doc:"Client-supplied key (Stripe-style, [ASSUMED] A6); replaying the same key within the TTL returns the original response instead of re-applying the mutation."`
	// (kept as a literal string tag, matching dryRunQueryDoc's wording --
	// struct tags cannot reference a package const.)
	Body struct {
		Name     string   `json:"name" required:"true" doc:"The new pool's name."`
		Requires []string `json:"requires,omitempty" doc:"Capability types every pool member must support. Forwarded as a comma-delimited list downstream, so a value must not itself contain a comma."`
	}
}

// mutationOutput is the shared Huma output shape for every mutating
// operation in this package: Result is the routed command's own raw
// stdout (trimmed), Revision is the resulting show.State.Revision (nil
// when it does not apply -- see mutationResult's own doc comment).
type mutationOutput struct {
	Body struct {
		Result   string `json:"result"`
		Revision *int64 `json:"revision,omitempty"`
	}
}

// newMutationOutput wraps result as a mutationOutput response body.
func newMutationOutput(result mutationResult) *mutationOutput {
	out := &mutationOutput{}
	out.Body.Result = result.Result
	out.Body.Revision = result.Revision
	return out
}

// registerCreatePool wires POST /v1/pools onto humaAPI, translating it
// into a "pool create" invocation through the mutation pipeline.
func registerCreatePool(humaAPI huma.API, server *Server) {
	huma.Register(humaAPI, huma.Operation{
		OperationID: "create-pool",
		Method:      http.MethodPost,
		Path:        apiPathPrefix + "/pools",
		Summary:     "Create a named logical pool (authoring scope required, D-08).",
	}, func(ctx context.Context, input *createPoolInput) (*mutationOutput, error) {
		if err := validateListValues("requires", input.Body.Requires); err != nil {
			return nil, err
		}
		args := []string{input.Body.Name}
		if len(input.Body.Requires) > 0 {
			args = append(args, "--requires", strings.Join(input.Body.Requires, ","))
		}
		result, err := mutate(ctx, server, mutateRequest{
			Route:          "pool create",
			Args:           args,
			IfMatch:        input.IfMatch,
			DryRun:         input.DryRun,
			IdempotencyKey: input.IdempotencyKey,
			Actor:          actorFromContext(ctx),
			CorrelationID:  correlationIDFromContext(ctx),
		})
		if err != nil {
			return nil, err
		}
		return newMutationOutput(result), nil
	})
}

var _ = RegisterOperation(OperationRegistration{Route: "pool create", Register: registerCreatePool})
