// events.go implements the revisioned global SSE event stream
// (07-08-PLAN.md Tasks 1/2, CONTEXT D-09/D-10/D-11/D-12): GET /v1/events
// registers a post-mutation observer (observer.go's seam) that appends
// every COMMITTED mutation (outcome "success" only -- never dry_run,
// failure, or idempotent_replay, which never durably change the show) to
// a bounded, revision-keyed ring buffer and broadcasts it to every open
// subscriber. This is one global stream carrying every domain's changes
// (D-09), not separate per-domain streams -- a mirrors' own doc comment in
// internal/wails/events.go documents the same discipline this stream
// follows: it is a lossy hint stream, NEVER the source of truth. The REST
// resource endpoints remain the only authoritative re-fetch path.
//
// A reconnecting client's Last-Event-ID drives either an in-window replay
// from the ring buffer, or, when the requested id has already scrolled
// out of the buffer, a single "resync" event instructing a full REST
// re-fetch before the client resumes consuming the (still open) stream
// (D-10) -- never a silently-missing gap. Any valid, non-expired API key,
// regardless of its coarse domain scope, may open this stream (D-12: no
// separate streaming capability exists on top of D-08's scopes) and, once
// open, receives every domain's events irrespective of that scope (D-11:
// an intentional, reviewed exposure -- T-07-12 accepts this because scopes
// gate mutations, not stream visibility, and the whole surface is
// loopback-by-default, D-06). This falls out of router.go's existing
// AuthMiddleware (valid key required, wired ahead of every operation) by
// this file simply never calling RequireScope -- no additional gating
// code is needed for D-11/D-12 beyond that omission.
//
// A periodic per-connection tick re-validates the connecting key and
// closes the connection within one tick interval of revocation or expiry
// (T-07-12b), closing the standard SSE "revocation does not close an
// already-open stream" gap 07-RESEARCH.md flagged under Security Domain
// V3/V4.
//
// Event-name design: huma/v2/sse.Register maps an SSE "event:" name to
// exactly one Go type via reflection (sse.go's typeToEvent), so multiple
// event names sharing one Go struct type would collide (the last
// registered name would win for every one of them). Only one concrete
// mutating route ("pool create") is wired through the mutation pipeline
// so far (07-05-SUMMARY.md's own documented scope decision defers the
// remaining ~40 mutating routes to a later plan), so inventing one
// distinct Go type per not-yet-existing domain event now would be
// speculative. Instead, this file registers a single generic "state"
// event name whose payload (domainEventPayload) itself carries a Type
// field derived from the producing route's domain -- satisfying D-09's
// "each event tagged with a type field" requirement without a
// premature one-struct-per-domain proliferation. Adding a genuinely
// distinct SSE event NAME per domain later is a compatible, additive
// change to this file, not a breaking one.
package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/sse"
	"github.com/lnorton89/golc/internal/show"
)

// EventRingBufferCapacity bounds the ring buffer's size -- [ASSUMED] per
// 07-RESEARCH.md Pattern 3 (a later plan/UAT may tune this); 256 committed
// mutations at typical desktop-editing cadence comfortably covers a brief
// client disconnect/reconnect without growing memory unbounded. Exported
// as a var, not a const, mirroring huma/v2/sse's own WriteTimeout
// precedent, so tests can shrink it to force a deterministic
// overflow/resync without needing hundreds of real mutations; production
// code should leave it at its default.
var EventRingBufferCapacity = 256

// EventRevocationTickInterval is how often an open /v1/events connection
// re-validates its own API key (T-07-12b) -- a modest cadence, not a
// per-message check, since revocation/expiry are rare compared to a
// connection's typical lifetime. Exported for the same test-tuning reason
// as EventRingBufferCapacity.
var EventRevocationTickInterval = 5 * time.Second

// domainEventPayload is the JSON body every "state" SSE event carries.
// Type is derived from the producing route's domain (its first
// space-separated word, mirroring mutate.go's own domainScope keying) --
// D-09's required "type field". Revision doubles as this SSE message's
// "id:" line. Route/Actor let a client correlate the event with which
// REST resource to re-fetch if it wants full detail (the stream itself
// never carries the mutation's full effect -- it is a hint, D-10).
type domainEventPayload struct {
	Type     string `json:"type" doc:"The changed domain, e.g. \"pool\" (derived from the producing route's first word)."`
	Route    string `json:"route" doc:"The routed command that produced this change, e.g. \"pool create\"."`
	Revision int64  `json:"revision" doc:"show.State.Revision this mutation produced -- also this SSE message's id."`
	Actor    string `json:"actor,omitempty" doc:"The authenticated API key id that produced this change."`
}

// resyncEventPayload is D-10's overflow signal: sent instead of a replay
// when a reconnecting client's Last-Event-ID has already scrolled out of
// the ring buffer. It carries no domain data -- the client must re-fetch
// authoritative state via the REST resource endpoints before treating the
// stream as caught up.
type resyncEventPayload struct {
	Reason string `json:"reason" doc:"Why a resync is required, e.g. \"buffer_overflow\"."`
}

// ringEvent is one buffered/broadcastable event: Revision doubles as the
// SSE message id (D-09) and the ring buffer's ordering key.
type ringEvent struct {
	Revision int64
	Payload  domainEventPayload
}

// eventBroadcaster owns the bounded ring buffer and the set of currently
// open subscriber channels every publish fans out to (D-11: every
// connection sees every event).
type eventBroadcaster struct {
	mu          sync.Mutex
	buffer      []ringEvent
	subscribers map[chan ringEvent]struct{}
}

func newEventBroadcaster() *eventBroadcaster {
	return &eventBroadcaster{subscribers: make(map[chan ringEvent]struct{})}
}

// publish appends ev to the ring buffer (evicting the oldest entries past
// EventRingBufferCapacity) and fans it out to every currently open
// subscriber. Delivery to a slow subscriber never blocks the publisher --
// this is called synchronously from within mutate.go's held
// mutationMutex (observer.go's own doc comment: "Observers must not block
// indefinitely") -- a subscriber whose channel is full simply misses this
// live event and recovers exactly the way a reconnecting client does: a
// Last-Event-ID replay on its next reconnect, or a resync signal if it
// has scrolled out of the buffer window (D-10).
func (b *eventBroadcaster) publish(ev ringEvent) {
	b.mu.Lock()
	b.buffer = append(b.buffer, ev)
	if capacity := EventRingBufferCapacity; len(b.buffer) > capacity {
		trimmed := make([]ringEvent, capacity)
		copy(trimmed, b.buffer[len(b.buffer)-capacity:])
		b.buffer = trimmed
	}
	subs := make([]chan ringEvent, 0, len(b.subscribers))
	for ch := range b.subscribers {
		subs = append(subs, ch)
	}
	b.mu.Unlock()

	for _, ch := range subs {
		select {
		case ch <- ev:
		default:
		}
	}
}

// subscribe registers a new subscriber channel and computes what a
// reconnecting client (or a fresh one) should see first: a replay of
// buffered events strictly newer than lastEventID (D-10 in-window
// recovery), a resync signal (D-10 overflow), or neither (a fresh
// subscriber with no Last-Event-ID, or one whose Last-Event-ID is already
// the latest known revision). unsubscribe must be called (deferred) by
// every caller once the connection ends, or the subscriber map leaks.
func (b *eventBroadcaster) subscribe(lastEventID string) (replay []ringEvent, resync bool, ch chan ringEvent, unsubscribe func()) {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch = make(chan ringEvent, EventRingBufferCapacity)
	b.subscribers[ch] = struct{}{}
	unsubscribe = func() {
		b.mu.Lock()
		delete(b.subscribers, ch)
		b.mu.Unlock()
	}

	if lastEventID == "" {
		// No Last-Event-ID: open live and block for future events. Never a
		// spurious resync, even against an empty buffer (nothing has been
		// missed if nothing has ever been requested).
		return nil, false, ch, unsubscribe
	}
	lastID, err := strconv.ParseInt(lastEventID, 10, 64)
	if err != nil {
		// A malformed Last-Event-ID is treated like an absent one -- never
		// reject a reconnecting client outright for a header it cannot
		// control precisely (SSE clients echo back whatever "id:" they
		// last received verbatim).
		return nil, false, ch, unsubscribe
	}

	if len(b.buffer) == 0 {
		// The buffer is empty (daemon just started, or nothing has ever
		// mutated). lastID == 0 means "I have seen nothing" -- consistent,
		// no gap. lastID > 0 means the client claims to have seen an event
		// this buffer has no record of at all (e.g. a daemon restart reset
		// the buffer) -- cannot prove no gap occurred, so resync rather
		// than silently assuming nothing was missed.
		return nil, lastID > 0, ch, unsubscribe
	}

	oldest := b.buffer[0].Revision
	latest := b.buffer[len(b.buffer)-1].Revision
	switch {
	case lastID < oldest-1:
		// A genuine gap: at least one committed event between lastID and
		// the buffer's oldest retained entry has already scrolled out.
		return nil, true, ch, unsubscribe
	case lastID >= latest:
		// Already caught up (exactly at, or somehow past, the latest known
		// revision) -- no replay, no resync.
		return nil, false, ch, unsubscribe
	default:
		for _, bufEv := range b.buffer {
			if bufEv.Revision > lastID {
				replay = append(replay, bufEv)
			}
		}
		return replay, false, ch, unsubscribe
	}
}

// reset clears every buffered event and drops every currently tracked
// subscriber channel (without closing them -- an open connection's own
// unsubscribe, deferred in handleEventStream, still runs normally against
// the now-forgotten channel). Used only by ResetEventStreamForTesting.
func (b *eventBroadcaster) reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buffer = nil
	b.subscribers = make(map[chan ringEvent]struct{})
}

// eventStreamBroadcaster is this package's singleton broadcaster, a
// package-level var (not a *Server field) mirroring mutate.go's own
// mutationMutex precedent: exactly one *Server is ever constructed per
// daemon process (D-07), so this package's event-distribution state is
// process-wide by the same standing assumption.
var eventStreamBroadcaster = newEventBroadcaster()

// eventObserverMu/eventObserverBound guard against double-registering
// publishMutationEvent with observer.go's global seam.
var (
	eventObserverMu    sync.Mutex
	eventObserverBound bool
)

// ensureEventStreamObserverRegistered idempotently wires
// publishMutationEvent into observer.go's seam. Called from the
// /v1/events operation's own Register callback (once per api.NewServer
// construction) rather than a package-init var initializer, because
// several sibling *_test.go files in this package (mutate_test.go,
// dryrun_test.go) already call observer.go's
// ResetMutationObserversForTesting directly in their own setup/cleanup --
// that clears observer.go's ENTIRE global observer list with no
// restoration, which would otherwise silently and permanently drop this
// package's SSE observer for the remainder of the test binary the first
// time any sibling test's cleanup ran. Binding at *Server-construction
// time instead means every fresh api.NewServer(...) call (every test's
// own setup) has a chance to re-arm it; ResetEventStreamForTesting
// (below) is what actually forces the re-arm for this package's own
// tests.
func ensureEventStreamObserverRegistered() {
	eventObserverMu.Lock()
	defer eventObserverMu.Unlock()
	if eventObserverBound {
		return
	}
	RegisterMutationObserver(publishMutationEvent)
	eventObserverBound = true
}

// ResetEventStreamForTesting clears this package's singleton event-stream
// state -- the ring buffer, every currently tracked subscriber channel,
// and the post-mutation observer registration -- and also wipes
// observer.go's entire global observer registry
// (ResetMutationObserversForTesting), guaranteeing a test using this
// helper gets a clean broadcaster with no orphaned or duplicated observer
// registration left over from an earlier test in the same binary (see
// ensureEventStreamObserverRegistered's doc comment for why this is
// necessary). Exported, like ResetMutationObserversForTesting, solely for
// test setup; production code must never call it.
func ResetEventStreamForTesting() {
	ResetMutationObserversForTesting()
	eventObserverMu.Lock()
	eventObserverBound = false
	eventObserverMu.Unlock()
	eventStreamBroadcaster.reset()
}

// domainFromRoute derives the SSE payload's Type field from route's first
// space-separated word (mirrors mutate.go's own
// domainScope/requiredScopeForRoute keying convention), e.g.
// "pool create" -> "pool".
func domainFromRoute(route string) string {
	if idx := strings.IndexByte(route, ' '); idx >= 0 {
		return route[:idx]
	}
	return route
}

// publishMutationEvent is observer.go's seam callback (07-05): registered
// once (ensureEventStreamObserverRegistered) and invoked, synchronously,
// for every attempted mutation this package processes. Only a committed
// success (ev.Outcome == "success", ResultingRevision populated) becomes
// a broadcastable domain event -- a dry-run never touches the real show,
// a failure never applied, and an idempotent replay's ResultingRevision
// is the ORIGINAL mutation's revision, already published once when that
// original mutation itself succeeded, so re-publishing it on replay would
// be a duplicate (behavior test: "a dry-run or failed mutation produces
// no state-change event").
func publishMutationEvent(ev MutationEvent) {
	if ev.Outcome != "success" || ev.ResultingRevision == nil {
		return
	}
	eventStreamBroadcaster.publish(ringEvent{
		Revision: *ev.ResultingRevision,
		Payload: domainEventPayload{
			Type:     domainFromRoute(ev.Route),
			Route:    ev.Route,
			Revision: *ev.ResultingRevision,
			Actor:    ev.Actor,
		},
	})
}

// apiKeyStillValid re-validates keyID against server's own api_keys store
// (T-07-12b's revocation tick), reimplementing show.IsAPIKeyValid's exact
// predicate (RevokedAt.IsZero() && now.Before(ExpiresAt)) against
// show.ListAPIKeys' hash-free rows rather than importing a new
// lookup-by-id accessor -- this package's existing auth.go already reads
// api_keys exclusively by prefix (the authentication path's own lookup
// key), and this file's files_modified scope (07-08-PLAN.md) is
// internal/api/events.go/events_test.go only, not internal/show. A lookup
// failure (e.g. a transient store error) fails closed: the connection is
// treated as no-longer-valid and closed, rather than left open on an
// infrastructure error that could be masking a real revocation.
func apiKeyStillValid(server *Server, keyID string) bool {
	if keyID == "" {
		return false
	}
	keys, err := show.ListAPIKeys(server.root, server.showPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "GOLC_API_EVENTS_KEY_CHECK_FAILED: %v\n", err)
		return false
	}
	now := time.Now()
	for _, key := range keys {
		if key.KeyID == keyID {
			return key.RevokedAt.IsZero() && now.Before(key.ExpiresAt)
		}
	}
	return false
}

// eventsInput is GET /v1/events's Huma input: Last-Event-ID is the SSE
// standard reconnection header (D-10), bound automatically by huma/v2/sse
// when declared on the input struct.
type eventsInput struct {
	LastEventID string `header:"Last-Event-ID" doc:"The id of the last event this client received, if reconnecting (SSE standard header, D-10). Omit when connecting for the first time; an in-window id replays missed events, an out-of-window id yields a single resync event instead of silently-missing state."`
}

// handleEventStream is GET /v1/events's SSE body: it subscribes to
// eventStreamBroadcaster (replaying/resyncing per Last-Event-ID), then
// blocks delivering live events until ctx is done (the client
// disconnected) or a periodic revocation-tick re-check finds the
// connecting key no longer valid (T-07-12b).
func handleEventStream(ctx context.Context, server *Server, input *eventsInput, send sse.Sender) {
	keyID, _ := KeyIDFromContext(ctx)

	replay, resync, ch, unsubscribe := eventStreamBroadcaster.subscribe(input.LastEventID)
	defer unsubscribe()

	if resync {
		if err := send(sse.Message{Data: resyncEventPayload{Reason: "buffer_overflow"}}); err != nil {
			return
		}
	} else {
		for _, ev := range replay {
			if err := send(sse.Message{ID: int(ev.Revision), Data: ev.Payload}); err != nil {
				return
			}
		}
	}

	ticker := time.NewTicker(EventRevocationTickInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			if err := send(sse.Message{ID: int(ev.Revision), Data: ev.Payload}); err != nil {
				return
			}
		case <-ticker.C:
			if !apiKeyStillValid(server, keyID) {
				return
			}
		}
	}
}

// registerEventsOperation wires GET /v1/events onto humaAPI via
// huma/v2/sse.Register (07-RESEARCH.md Pattern 3), documenting D-11's
// cross-scope exposure directly in the operation's OpenAPI description so
// integrators are not surprised (T-07-12's mitigation).
func registerEventsOperation(humaAPI huma.API, server *Server) {
	ensureEventStreamObserverRegistered()
	sse.Register(humaAPI, huma.Operation{
		OperationID: "watch-events",
		Method:      http.MethodGet,
		Path:        apiPathPrefix + "/events",
		Summary:     "Subscribe to the global revisioned event stream.",
		Description: "Every domain's changes on one global stream (D-09), each tagged with a type and an id equal to the show revision that produced it. Any valid, non-expired API key may open this stream regardless of its coarse domain scope (D-12) -- and, once open, receives every domain's events regardless of that scope too (D-11): scopes gate mutations, not stream visibility. This is a hint stream, never the source of truth -- always re-fetch authoritative state via the REST resource endpoints, especially after a \"resync\" event (D-10). Reconnecting with Last-Event-ID replays events still within the server's bounded buffer; an id already scrolled out of that buffer yields one resync event instead of a silent gap. A revoked or expired key's already-open stream is closed within one revocation-tick interval.",
	}, map[string]any{
		"state":  domainEventPayload{},
		"resync": resyncEventPayload{},
	}, func(ctx context.Context, input *eventsInput, send sse.Sender) {
		handleEventStream(ctx, server, input, send)
	})
}

// "events watch" is a synthetic route key, not a real internal/command
// route -- GET /v1/events is pure new infrastructure (D-09), never a
// translated CLI command, so it has no entry in routecatalog's registry
// and therefore never appears in coverage_test.go's allRoutes set; this
// registration exists solely so this operation participates in router.go's
// buildRouter loop, the same way every other operation in this package
// does (RegisterOperation's own doc comment: every OperationRegistration
// must map to exactly one route, and every route may be claimed by at
// most one registration -- a synthetic, clearly-non-CLI key like this one
// can never collide with a real routed command).
var _ = RegisterOperation(OperationRegistration{Route: "events watch", Register: registerEventsOperation})
