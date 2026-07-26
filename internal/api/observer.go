// observer.go implements the post-mutation observer seam (07-05-PLAN.md
// Task 1, CONTEXT key_links): mutate.go's serialized critical section
// fires exactly one MutationEvent per attempted mutation -- success,
// failure, dry_run (dryrun.go), or an idempotent replay (idempotency.go)
// -- through every currently-registered observer. This is the single seam
// 07-07 (audit) and 07-08 (SSE) both attach to via RegisterMutationObserver,
// mirroring the CLI command-execution package's own declaredRoutes
// self-registration idiom (07-PATTERNS.md "Shared Patterns:
// Self-registration idiom") but for runtime callbacks rather than
// package-init-time declarations.
package api

import "sync"

// MutationEvent describes the outcome of one attempted mutating request,
// carrying everything an observer needs to write an audit row (07-07) or
// publish an SSE event (07-08) without re-deriving it from the HTTP
// request itself.
type MutationEvent struct {
	// Route is the routed command key the request translated into, e.g.
	// "pool create".
	Route string
	// Args are the business arguments Execute was (or would have been)
	// called with, excluding the injected "--show <path>" pair.
	Args []string
	// Actor is the authenticated API key's id (KeyIDFromContext), never
	// the raw token or hash.
	Actor string
	// Source is always "http" for events this package fires -- other
	// control surfaces (wails, cli, script) are out of this package's
	// scope but share the same MutationEvent shape by convention.
	Source string
	// CorrelationID is the request's chi/middleware.RequestID value.
	CorrelationID string
	// ExpectedRevision is the parsed If-Match revision, or nil when the
	// request omitted the header.
	ExpectedRevision *int64
	// ResultingRevision is show.State.Revision after a successful,
	// durably-applied mutation, or nil for a failure, a dry-run
	// (D-14 -- the real show is never touched), or an idempotent replay
	// against an already-applied effect.
	ResultingRevision *int64
	// Outcome is one of "success", "failure", "dry_run", or
	// "idempotent_replay".
	Outcome string
	// StatusCode is the HTTP status the request ultimately resolved to.
	StatusCode int
}

// mutationObserversMu guards mutationObservers against concurrent
// RegisterMutationObserver/fireMutationObservers calls -- observers can be
// registered at daemon-startup time (07-07/07-08 wiring) while mutating
// requests are already being served.
var (
	mutationObserversMu sync.Mutex
	mutationObservers    []func(MutationEvent)
)

// RegisterMutationObserver adds observer to the set fireMutationObservers
// notifies after every attempted mutation. Intended to be called once per
// observer at daemon-startup wiring time (mirrors router.go's
// RegisterOperation self-registration idiom, but a runtime call rather
// than a package-level var initializer, since 07-07/08's observers need a
// live audit-writer/SSE-broadcaster instance to close over).
func RegisterMutationObserver(observer func(MutationEvent)) {
	mutationObserversMu.Lock()
	defer mutationObserversMu.Unlock()
	mutationObservers = append(mutationObservers, observer)
}

// PublishMutationEvent is the exported seam a non-HTTP control surface
// (wails, cli, script -- see MutationEvent.Source's own doc comment above)
// uses to enter the same audit/SSE pipeline the HTTP path uses
// (08-08-PLAN.md Task 2, CONTEXT D-05): a thin wrapper over
// fireMutationObservers, with identical semantics -- every registered
// observer notified exactly once, in registration order, synchronously.
// This seam does not exist so a caller can synthesize an HTTP-shaped
// event: ev.Source must accurately name the originating surface (e.g.
// "script", never "http"), so an audit row or SSE event correctly
// attributes a non-HTTP-issued mutation to its real origin rather than
// impersonating an HTTP request. fireMutationObservers itself stays
// unexported -- mutate.go continues to call it directly from within its
// own serialized critical section, so that section's ordering guarantee
// is untouched by this additional entry point.
func PublishMutationEvent(ev MutationEvent) {
	fireMutationObservers(ev)
}

// fireMutationObservers notifies every currently-registered observer of
// ev, synchronously, in registration order. mutate.go calls this as the
// final step of its serialized critical section (still holding
// mutationMutex), so observers -- and therefore audit writes (07-07) and
// SSE publishes (07-08) -- are guaranteed to run in the same strict
// revision order the mutations themselves were applied in; a slow
// observer therefore also throttles the next mutation, an accepted MVP
// tradeoff for a stronger audit-ordering guarantee. Observers must not
// block indefinitely.
func fireMutationObservers(ev MutationEvent) {
	mutationObserversMu.Lock()
	observers := append([]func(MutationEvent){}, mutationObservers...)
	mutationObserversMu.Unlock()
	for _, observer := range observers {
		observer(ev)
	}
}

// ResetMutationObserversForTesting clears every currently-registered
// observer. Exported (rather than a package-private helper) because the
// tests that need it live in the external api_test package -- routecatalog's
// own import of the CLI command-execution package, which internal/command
// itself imports this package back through (internal/command/artnet.go's
// apiCommandExecutor adapter), makes any *_test.go file inside package api
// itself that also imports routecatalog an import cycle (api -> routecatalog
// -> command -> api); api_test is a distinct package, so it is not
// affected. This function exists solely so a test can guarantee it starts
// and ends with a clean observer registry regardless of what earlier
// tests in the same test binary registered -- production code must never
// call it.
func ResetMutationObserversForTesting() {
	mutationObserversMu.Lock()
	defer mutationObserversMu.Unlock()
	mutationObservers = nil
}
