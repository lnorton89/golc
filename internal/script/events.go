// events.go implements the script event bus (08-08-PLAN.md Task 1,
// CONTEXT D-04/D-05): a bounded ring buffer + strictly monotonic
// per-process Seq + subscriber fan-out, structurally mirroring
// internal/api/events.go's eventBroadcaster -- SAME SHAPE, not the same
// code path. That package's broadcaster is bound to domainEventPayload and
// to HTTP subscriber lifecycles (huma/v2/sse); this bus carries a
// different, flat ScriptEvent payload and serves two non-HTTP sinks: the
// desktop webview (internal/wails' EventPusher, via SubscribeScriptEvents)
// and internal/api's SSE stream (a "script" Type tag on the existing
// global /v1/events stream, via api.PublishScriptLifecycleEvent). A reader
// familiar with internal/api/events.go will recognize every mechanism here
// -- ring buffer, Seq assignment, replay-vs-resync -- but this file is not
// an accidental duplicate: it exists because this package must not import
// internal/api (see 08-06-SUMMARY.md's own import-direction note: internal/
// command imports internal/api, so internal/script importing internal/api
// stays cycle-free, but internal/api must never import internal/script --
// TestScriptEventBusPackageNeverImportsAPI in events_test.go pins this).
//
// Every published Message/Reason string passes through security.Redact
// inside publish -- the single publication point -- so no future sink
// (webview, SSE, or anything added later) can forget to redact (T-08-34).
package script

import (
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/security"
	"github.com/lnorton89/golc/internal/show"
)

// ScriptEventKind names one of the four event kinds this bus carries
// (08-08-PLAN.md Task 1's exact <behavior> list): script.log (a captured
// stdout/stderr line), script.outcome (one SDK call's real-time result,
// D-05's live half), script.status (a lifecycle transition, e.g. "a run
// started"), and script.terminal (the run's guaranteed final event, T-08-38).
type ScriptEventKind string

const (
	ScriptEventLog      ScriptEventKind = "script.log"
	ScriptEventOutcome  ScriptEventKind = "script.outcome"
	ScriptEventStatus   ScriptEventKind = "script.status"
	ScriptEventTerminal ScriptEventKind = "script.terminal"
)

// ScriptEvent is one flat payload carrying every field any of the four
// kinds might need -- a single struct rather than a union/interface, so a
// consuming switch (both the frontend's and internal/wails') stays one
// simple statement per kind rather than a type-per-kind assertion chain.
// RunID/ScriptName are always populated, regardless of Kind. Seq is
// assigned exactly once, by publish -- callers never set it themselves.
type ScriptEvent struct {
	Seq        int64
	Kind       ScriptEventKind
	RunID      uuid.UUID
	ScriptName string
	At         time.Time

	// Level/Message/Source: script.log fields.
	Level   string
	Message string
	Source  string

	// Method/Route/DurationMS/Ok/Code: script.outcome fields (mirrors
	// CallOutcome field-for-field, session.go).
	Method     string
	Route      string
	DurationMS int64
	Ok         bool
	Code       string

	// Status/Reason: script.status and script.terminal fields.
	Status show.ScriptRunStatus
	Reason string
}

// ScriptEventRingCapacity bounds the bus's ring buffer -- exported as a
// var (not a const), mirroring internal/api/events.go's
// EventRingBufferCapacity precedent exactly, so a test can shrink it to
// force a deterministic resync without needing hundreds of real events;
// production code should leave it at its default. 512 comfortably covers
// a brief debug-panel reconnect at a chatty script's typical logging
// cadence (the flagged SCRP-05 backstop truth: the exact capacity at which
// resync begins triggering is measured, not assumed, at implementation
// time via TestScriptEventBusOverflowTriggersResyncAtMeasuredCapacity).
var ScriptEventRingCapacity = 512

// eventBus owns the bounded ring buffer and the set of currently open
// subscriber channels every publish fans out to -- structurally identical
// to internal/api/events.go's eventBroadcaster (mu/buffer/subscribers/
// nextSeq), copied rather than imported per this file's own doc comment.
type eventBus struct {
	mu          sync.Mutex
	buffer      []ScriptEvent
	subscribers map[chan ScriptEvent]struct{}
	nextSeq     int64
}

func newEventBus() *eventBus {
	return &eventBus{subscribers: make(map[chan ScriptEvent]struct{})}
}

// publish is the SINGLE place a ScriptEvent's Seq is assigned and the
// single place Message/Reason are redacted (T-08-34: redaction happens
// once, at the single publication point, so no sink can forget it) --
// mirrors internal/api/events.go's publish exactly: increment nextSeq
// under mu, append (evicting the oldest entries past
// ScriptEventRingCapacity), then fan out to every currently open
// subscriber without blocking the publisher on a slow/full subscriber
// channel.
func (b *eventBus) publish(ev ScriptEvent) {
	ev.Message = security.Redact(ev.Message)
	ev.Reason = security.Redact(ev.Reason)

	b.mu.Lock()
	b.nextSeq++
	ev.Seq = b.nextSeq
	b.buffer = append(b.buffer, ev)
	if capacity := ScriptEventRingCapacity; len(b.buffer) > capacity {
		trimmed := make([]ScriptEvent, capacity)
		copy(trimmed, b.buffer[len(b.buffer)-capacity:])
		b.buffer = trimmed
	}
	subs := make([]chan ScriptEvent, 0, len(b.subscribers))
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

// Subscribe registers a new subscriber channel and computes what a
// reconnecting caller (lastSeq > 0) should see first -- mirrors
// internal/api/events.go's subscribe exactly, re-keyed to an int64 Seq
// directly rather than a string Last-Event-ID header (this bus has no HTTP
// request to parse one from; its two callers -- internal/wails and
// internal/api's own script SSE observer -- already have a typed int64).
// lastSeq <= 0 means "no prior Seq known": a fresh subscriber attaches live
// with no replay and no spurious resync, exactly matching
// internal/api/events.go's empty-Last-Event-ID branch.
func (b *eventBus) Subscribe(lastSeq int64) (replay []ScriptEvent, resync bool, ch chan ScriptEvent, unsubscribe func()) {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch = make(chan ScriptEvent, ScriptEventRingCapacity)
	b.subscribers[ch] = struct{}{}
	unsubscribe = func() {
		b.mu.Lock()
		delete(b.subscribers, ch)
		b.mu.Unlock()
	}

	if lastSeq <= 0 {
		return nil, false, ch, unsubscribe
	}

	if len(b.buffer) == 0 {
		// The buffer is empty (fresh process, or nothing has ever been
		// published) but the caller claims to have seen a Seq already --
		// cannot prove no gap occurred, so resync rather than silently
		// assuming nothing was missed.
		return nil, true, ch, unsubscribe
	}

	oldest := b.buffer[0].Seq
	latest := b.buffer[len(b.buffer)-1].Seq
	switch {
	case lastSeq < oldest-1:
		// A genuine gap: at least one published event between lastSeq and
		// the buffer's oldest retained entry has already scrolled out.
		return nil, true, ch, unsubscribe
	case lastSeq == latest:
		// Already caught up -- no replay, no resync.
		return nil, false, ch, unsubscribe
	case lastSeq > latest:
		// A Seq this bus never issued (e.g. reset() zeroed the counter
		// since the caller last saw it) -- cannot prove no gap occurred.
		return nil, true, ch, unsubscribe
	default:
		for _, bufEv := range b.buffer {
			if bufEv.Seq > lastSeq {
				replay = append(replay, bufEv)
			}
		}
		return replay, false, ch, unsubscribe
	}
}

// reset clears every buffered event, drops every currently tracked
// subscriber channel (without closing them), and zeroes nextSeq -- used
// only by ResetScriptEventsForTesting.
func (b *eventBus) reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buffer = nil
	b.subscribers = make(map[chan ScriptEvent]struct{})
	b.nextSeq = 0
}

// scriptEventBus is this package's singleton bus -- a package-level var
// (not a Host field), since a script event stream is process-wide, the
// same standing assumption internal/api/events.go's own singleton
// eventStreamBroadcaster makes for that package.
var scriptEventBus = newEventBus()

// PublishScriptEvent publishes ev onto the process-wide script event bus.
// session.go is this seam's only production caller -- once per captured
// log line, once per recorded CallOutcome, once at run start, and exactly
// once (guaranteed by a defer registered before any early return) at every
// run's end.
func PublishScriptEvent(ev ScriptEvent) {
	scriptEventBus.publish(ev)
}

// SubscribeScriptEvents subscribes to the process-wide script event bus --
// the seam internal/wails.ScriptService.StartScriptEventStream (08-08-PLAN.md
// Task 3) and any future in-process SSE-style consumer use to receive
// live/replayed/resync-signaled events without a second bus.
func SubscribeScriptEvents(lastSeq int64) (replay []ScriptEvent, resync bool, ch chan ScriptEvent, unsubscribe func()) {
	return scriptEventBus.Subscribe(lastSeq)
}

// ResetScriptEventsForTesting clears the bus's entire state so a test gets
// a deterministic Seq sequence starting at 1, independent of what any
// earlier test in the same binary published. Exported solely for test
// setup; production code must never call it.
func ResetScriptEventsForTesting() {
	scriptEventBus.reset()
}
