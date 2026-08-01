// events_test.go pins events.go's revisioned global SSE stream (07-08-
// PLAN.md Tasks 1/2): monotonic revision order, in-window Last-Event-ID
// replay, out-of-window resync, the Last-Event-ID == latest adjacency
// edge, the empty-buffer/no-header edge, D-11 broadcast-to-every-
// subscriber, dry-run/failure producing no event, D-12 any-valid-key
// access, D-11 cross-scope visibility, and the T-07-12b revocation tick.
//
// This file lives in the external api_test package (see coverage_test.go
// and mutate_test.go's own doc comments for why: internal/routecatalog's
// test-only bridge, needed here to seed a real "pool create" mutation
// through the full pipeline, would create an import cycle from inside
// package api itself). It reuses mutate_test.go's seedKey/jsonBody/
// doCreatePoolRequest and dryrun_test.go's doDryRunCreatePoolRequest
// helpers directly, since all of these *_test.go files share this one
// api_test package.
//
// GET /v1/events streams indefinitely, so it cannot be exercised through
// httptest.NewRecorder()+ServeHTTP() the way every other operation's test
// in this package is (that call blocks until the handler returns, which
// for this handler only happens on client disconnect). Instead these
// tests spin up a real httptest.Server and a real streaming http.Client
// request per connection, reading and parsing SSE frames off the live
// response body as they arrive -- the standard technique for testing a
// long-lived streaming HTTP handler.
package api_test

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/api"
	"github.com/lnorton89/golc/internal/routecatalog"
	"github.com/lnorton89/golc/internal/show"
)

// newEventsTestServer builds a fresh *api.Server (wired to a real command
// registry, against a brand-new temp root/show) with this package's
// singleton event-stream state reset first -- events.go's
// ResetEventStreamForTesting doc comment explains why this is required
// regardless of which other *_test.go files in this package have already
// run in the same test binary.
func newEventsTestServer(t *testing.T) (server *api.Server, root, showPath string) {
	t.Helper()
	api.ResetEventStreamForTesting()
	t.Cleanup(api.ResetEventStreamForTesting)

	root = t.TempDir()
	showPath = filepath.Join(root, "show.golc")
	catalog, err := routecatalog.New()
	require.NoError(t, err, "routecatalog.New")
	server = api.NewServer(catalog, root, showPath)
	return server, root, showPath
}

// sseFrame is one parsed SSE message: Event is the "event:" line's value
// (unset defaults to "message" per the SSE spec, though this package
// never registers that default name), ID is the raw "id:" line's value,
// Data is the raw "data:" payload (still JSON-encoded text).
type sseFrame struct {
	Event string
	ID    string
	Data  string
}

// sseClient wraps one live streaming GET /v1/events connection, letting a
// test read parsed frames one at a time (blocking) and tear the
// connection down deterministically at cleanup.
type sseClient struct {
	resp   *http.Response
	reader *bufio.Reader
	cancel context.CancelFunc
}

// openEventStream issues a real streaming GET /v1/events against ts,
// presenting token and (if non-empty) lastEventID.
func openEventStream(t *testing.T, ts *httptest.Server, token, lastEventID string) *sseClient {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/v1/events", nil)
	if err != nil {
		cancel()
	}
	require.NoError(t, err, "NewRequestWithContext")
	req.Header.Set("Authorization", "Bearer "+token)
	if lastEventID != "" {
		req.Header.Set("Last-Event-ID", lastEventID)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		cancel()
	}
	require.NoError(t, err, "opening event stream")
	client := &sseClient{resp: resp, reader: bufio.NewReader(resp.Body), cancel: cancel}
	t.Cleanup(client.close)
	return client
}

func (c *sseClient) close() {
	c.cancel()
	c.resp.Body.Close()
}

// next reads and parses the next SSE frame from the stream, blocking
// until one fully arrives (terminated by a blank line) or the underlying
// read fails -- e.g. because the connection was canceled/closed.
func (c *sseClient) next() (sseFrame, error) {
	var frame sseFrame
	sawAnyLine := false
	for {
		line, err := c.reader.ReadString('\n')
		if line == "" && err != nil {
			return frame, err
		}
		trimmed := strings.TrimRight(line, "\r\n")
		if trimmed == "" {
			if sawAnyLine {
				return frame, nil
			}
			if err != nil {
				return frame, err
			}
			continue // blank padding line before any field -- keep reading
		}
		sawAnyLine = true
		switch {
		case strings.HasPrefix(trimmed, "event: "):
			frame.Event = strings.TrimPrefix(trimmed, "event: ")
		case strings.HasPrefix(trimmed, "id: "):
			frame.ID = strings.TrimPrefix(trimmed, "id: ")
		case strings.HasPrefix(trimmed, "data: "):
			frame.Data = strings.TrimPrefix(trimmed, "data: ")
		}
		if err != nil {
			return frame, err
		}
	}
}

// nextWithTimeout runs next() in a goroutine and fails t if it does not
// return within d -- used only for the revocation test, where "the
// connection closes" is itself the awaited event.
func (c *sseClient) nextWithTimeout(t *testing.T, d time.Duration) (sseFrame, error) {
	t.Helper()
	type result struct {
		frame sseFrame
		err   error
	}
	done := make(chan result, 1)
	go func() {
		frame, err := c.next()
		done <- result{frame, err}
	}()
	select {
	case r := <-done:
		return r.frame, r.err
	case <-time.After(d):
		require.FailNowf(t, "timed out waiting for the next SSE frame/connection close", "after %s", d)
		return sseFrame{}, nil
	}
}

// decodeDomainPayload decodes frame's Data as a domainEventPayload-shaped
// JSON body (mirroring events.go's own field tags exactly, without
// importing the unexported type).
func decodeDomainPayload(t *testing.T, frame sseFrame) (eventType, route string, revision int64) {
	t.Helper()
	var decoded struct {
		Type     string `json:"type"`
		Route    string `json:"route"`
		Revision int64  `json:"revision"`
	}
	err := json.Unmarshal([]byte(frame.Data), &decoded)
	require.NoError(t, err, "decode domain event payload %q", frame.Data)
	return decoded.Type, decoded.Route, decoded.Revision
}

// --- TestSSEOrder -----------------------------------------------------------

// TestSSEOrder proves that after three successful mutations, a fresh
// subscriber with no Last-Event-ID opens live (no replay) and receives
// subsequent events in monotonic revision order (07-08-PLAN.md Task 1
// behavior).
func TestSSEOrder(t *testing.T) {
	server, root, showPath := newEventsTestServer(t)
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	for _, name := range []string{"Alpha", "Bravo", "Charlie"} {
		rec := doCreatePoolRequest(t, server.Handler(), token, "", name)
		require.Truef(t, rec.Code >= 200 && rec.Code < 300, "expected setup create %q to succeed, got %d (body: %s)", name, rec.Code, rec.Body.String())
	}

	ts := httptest.NewServer(server.Handler())
	t.Cleanup(ts.Close)
	client := openEventStream(t, ts, token, "")
	require.Equal(t, http.StatusOK, client.resp.StatusCode, "expected 200 opening the stream")

	for _, name := range []string{"Delta", "Echo"} {
		rec := doCreatePoolRequest(t, server.Handler(), token, "", name)
		require.Truef(t, rec.Code >= 200 && rec.Code < 300, "expected create %q to succeed, got %d (body: %s)", name, rec.Code, rec.Body.String())
	}

	first, err := client.next()
	require.NoError(t, err, "reading first live frame")
	require.Equalf(t, "state", first.Event, "expected the first live frame to be event=state id=4, got event=%q id=%q", first.Event, first.ID)
	require.Equalf(t, "4", first.ID, "expected the first live frame to be event=state id=4, got event=%q id=%q", first.Event, first.ID)
	second, err := client.next()
	require.NoError(t, err, "reading second live frame")
	require.Equalf(t, "state", second.Event, "expected the second live frame to be event=state id=5, got event=%q id=%q", second.Event, second.ID)
	require.Equalf(t, "5", second.ID, "expected the second live frame to be event=state id=5, got event=%q id=%q", second.Event, second.ID)
}

// --- TestSSEReplay -----------------------------------------------------------

// TestSSEReplay proves a subscriber reconnecting with an in-window
// Last-Event-ID receives exactly the missed events, replayed in order,
// with no duplicates (D-10).
func TestSSEReplay(t *testing.T) {
	server, root, showPath := newEventsTestServer(t)
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	for _, name := range []string{"Alpha", "Bravo", "Charlie"} {
		rec := doCreatePoolRequest(t, server.Handler(), token, "", name)
		require.Truef(t, rec.Code >= 200 && rec.Code < 300, "expected setup create %q to succeed, got %d (body: %s)", name, rec.Code, rec.Body.String())
	}

	ts := httptest.NewServer(server.Handler())
	t.Cleanup(ts.Close)
	client := openEventStream(t, ts, token, "1")

	replayed := make([]string, 0, 2)
	for i := 0; i < 2; i++ {
		frame, err := client.next()
		require.NoErrorf(t, err, "reading replay frame %d", i)
		require.Equalf(t, "state", frame.Event, "expected replay frame %d to be event=state, got %q", i, frame.Event)
		replayed = append(replayed, frame.ID)
	}
	require.Equal(t, []string{"2", "3"}, replayed, "expected replay ids [2 3] in order with no duplicates")
}

// --- TestSSEGapRecovery -------------------------------------------------------

// TestSSEGapRecovery proves a subscriber reconnecting with a
// Last-Event-ID older than the oldest buffered revision receives a single
// resync event, not silently-missing state events (D-10).
func TestSSEGapRecovery(t *testing.T) {
	original := api.EventRingBufferCapacity
	api.EventRingBufferCapacity = 2
	t.Cleanup(func() { api.EventRingBufferCapacity = original })

	server, root, showPath := newEventsTestServer(t)
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	for _, name := range []string{"Alpha", "Bravo", "Charlie", "Delta", "Echo"} {
		rec := doCreatePoolRequest(t, server.Handler(), token, "", name)
		require.Truef(t, rec.Code >= 200 && rec.Code < 300, "expected setup create %q to succeed, got %d (body: %s)", name, rec.Code, rec.Body.String())
	}
	// Capacity 2 after 5 successful creates (revisions 1..5) leaves the
	// buffer holding only [4, 5] -- oldest=4, latest=5.

	ts := httptest.NewServer(server.Handler())
	t.Cleanup(ts.Close)
	client := openEventStream(t, ts, token, "1")

	frame, err := client.next()
	require.NoError(t, err, "reading resync frame")
	require.Equalf(t, "resync", frame.Event, "expected a resync event for an out-of-window Last-Event-ID, got event=%q data=%q", frame.Event, frame.Data)
	var decoded struct {
		Reason string `json:"reason"`
	}
	err = json.Unmarshal([]byte(frame.Data), &decoded)
	require.NoErrorf(t, err, "decode resync payload %q", frame.Data)
	require.NotEmpty(t, decoded.Reason, "expected the resync event to carry a non-empty reason")
}

// --- TestSSEAdjacentNoReplayNoResync -----------------------------------------

// TestSSEAdjacentNoReplayNoResync proves a subscriber reconnecting with
// Last-Event-ID equal to the latest emitted revision receives no
// replayed events and no spurious resync -- the very first frame it sees
// is the next genuinely new live event (API-03 adjacency edge).
func TestSSEAdjacentNoReplayNoResync(t *testing.T) {
	server, root, showPath := newEventsTestServer(t)
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	for _, name := range []string{"Alpha", "Bravo", "Charlie"} {
		rec := doCreatePoolRequest(t, server.Handler(), token, "", name)
		require.Truef(t, rec.Code >= 200 && rec.Code < 300, "expected setup create %q to succeed, got %d (body: %s)", name, rec.Code, rec.Body.String())
	}

	ts := httptest.NewServer(server.Handler())
	t.Cleanup(ts.Close)
	client := openEventStream(t, ts, token, "3") // 3 == latest buffered revision

	rec := doCreatePoolRequest(t, server.Handler(), token, "", "Delta")
	require.Truef(t, rec.Code >= 200 && rec.Code < 300, "expected the new create to succeed, got %d (body: %s)", rec.Code, rec.Body.String())

	frame, err := client.next()
	require.NoError(t, err, "reading first frame")
	require.Equalf(t, "state", frame.Event, "expected the very first frame to be the new live event=state id=4 (no replay, no resync), got event=%q id=%q data=%q", frame.Event, frame.ID, frame.Data)
	require.Equalf(t, "4", frame.ID, "expected the very first frame to be the new live event=state id=4 (no replay, no resync), got event=%q id=%q data=%q", frame.Event, frame.ID, frame.Data)
}

// --- TestSSEEmptyBufferNoLastEventID ------------------------------------------

// TestSSEEmptyBufferNoLastEventID proves subscribing against an empty
// buffer with no Last-Event-ID opens the stream and blocks for future
// events without emitting a spurious resync (API-03 empty edge).
func TestSSEEmptyBufferNoLastEventID(t *testing.T) {
	server, root, showPath := newEventsTestServer(t)
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	ts := httptest.NewServer(server.Handler())
	t.Cleanup(ts.Close)
	client := openEventStream(t, ts, token, "")

	rec := doCreatePoolRequest(t, server.Handler(), token, "", "Alpha")
	require.Truef(t, rec.Code >= 200 && rec.Code < 300, "expected the create to succeed, got %d (body: %s)", rec.Code, rec.Body.String())

	frame, err := client.next()
	require.NoError(t, err, "reading first frame")
	require.Equalf(t, "state", frame.Event, "expected the first frame to be the new live event=state id=1 (no spurious resync against an empty buffer), got event=%q id=%q", frame.Event, frame.ID)
	require.Equalf(t, "1", frame.ID, "expected the first frame to be the new live event=state id=1 (no spurious resync against an empty buffer), got event=%q id=%q", frame.Event, frame.ID)
}

// --- TestSSEBroadcast ---------------------------------------------------------

// TestSSEBroadcast proves two concurrent subscribers both receive every
// event (D-11), and that a dry-run or failed mutation produces no
// state-change event on either.
func TestSSEBroadcast(t *testing.T) {
	server, root, showPath := newEventsTestServer(t)
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	ts := httptest.NewServer(server.Handler())
	t.Cleanup(ts.Close)
	clientA := openEventStream(t, ts, token, "")
	clientB := openEventStream(t, ts, token, "")

	first := doCreatePoolRequest(t, server.Handler(), token, "", "Alpha")
	require.Truef(t, first.Code >= 200 && first.Code < 300, "expected the first create to succeed, got %d (body: %s)", first.Code, first.Body.String())
	for name, c := range map[string]*sseClient{"A": clientA, "B": clientB} {
		frame, err := c.next()
		require.NoErrorf(t, err, "client %s: reading first frame", name)
		require.Equalf(t, "state", frame.Event, "client %s: expected event=state id=1, got event=%q id=%q", name, frame.Event, frame.ID)
		require.Equalf(t, "1", frame.ID, "client %s: expected event=state id=1, got event=%q id=%q", name, frame.Event, frame.ID)
	}

	// A dry-run and a failing (duplicate-name) mutation must both produce
	// no state event on either subscriber.
	dryRun := doDryRunCreatePoolRequest(t, server.Handler(), token, "Bravo")
	require.Equalf(t, http.StatusOK, dryRun.Code, "expected the dry-run to return 200, got %d (body: %s)", dryRun.Code, dryRun.Body.String())
	dup := doCreatePoolRequest(t, server.Handler(), token, "", "Alpha")
	require.GreaterOrEqualf(t, dup.Code, 400, "expected the duplicate-name create to fail, got %d (body: %s)", dup.Code, dup.Body.String())

	second := doCreatePoolRequest(t, server.Handler(), token, "", "Bravo")
	require.Truef(t, second.Code >= 200 && second.Code < 300, "expected the second create to succeed, got %d (body: %s)", second.Code, second.Body.String())
	for name, c := range map[string]*sseClient{"A": clientA, "B": clientB} {
		frame, err := c.next()
		require.NoErrorf(t, err, "client %s: reading second frame", name)
		require.Equalf(t, "state", frame.Event, "client %s: expected the second frame to be event=state id=2 (dry-run/failure produced nothing in between), got event=%q id=%q", name, frame.Event, frame.ID)
		require.Equalf(t, "2", frame.ID, "client %s: expected the second frame to be event=state id=2 (dry-run/failure produced nothing in between), got event=%q id=%q", name, frame.Event, frame.ID)
		eventType, route, revision := decodeDomainPayload(t, frame)
		require.Equalf(t, "pool", eventType, "client %s: expected type=pool route=\"pool create\" revision=2, got type=%q route=%q revision=%d", name, eventType, route, revision)
		require.Equalf(t, "pool create", route, "client %s: expected type=pool route=\"pool create\" revision=2, got type=%q route=%q revision=%d", name, eventType, route, revision)
		require.Equalf(t, int64(2), revision, "client %s: expected type=pool route=\"pool create\" revision=2, got type=%q route=%q revision=%d", name, eventType, route, revision)
	}
}

// --- TestSSEAuth ---------------------------------------------------------------

// TestSSEAuth proves any valid, non-expired key (regardless of scope) can
// open /v1/events (D-12), and an invalid or expired key is rejected 401.
func TestSSEAuth(t *testing.T) {
	server, root, showPath := newEventsTestServer(t)
	playbackToken, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopePlayback})

	generated, err := show.GenerateAPIKey()
	require.NoError(t, err, "GenerateAPIKey")
	_, err = show.InsertAPIKey(root, showPath, generated, []show.APIKeyScope{show.APIKeyScopeAdmin}, time.Now().UTC().Add(-time.Hour))
	require.NoError(t, err, "InsertAPIKey (expired)")
	expiredToken := generated.RawToken

	ts := httptest.NewServer(server.Handler())
	t.Cleanup(ts.Close)

	ok := openEventStream(t, ts, playbackToken, "")
	require.Equalf(t, http.StatusOK, ok.resp.StatusCode, "expected a valid playback-scoped key to open the stream (D-12), got %d", ok.resp.StatusCode)

	badToken := openEventStream(t, ts, "not-a-real-token", "")
	require.Equalf(t, http.StatusUnauthorized, badToken.resp.StatusCode, "expected an unknown token to be rejected 401, got %d", badToken.resp.StatusCode)

	expired := openEventStream(t, ts, expiredToken, "")
	require.Equalf(t, http.StatusUnauthorized, expired.resp.StatusCode, "expected an expired key to be rejected 401, got %d", expired.resp.StatusCode)
}

// --- TestSSECrossScope -----------------------------------------------------

// TestSSECrossScope proves a narrowly-scoped (playback-only) key's open
// stream still receives an authoring-domain event (D-11's documented
// intentional cross-scope exposure).
func TestSSECrossScope(t *testing.T) {
	server, root, showPath := newEventsTestServer(t)
	playbackToken, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopePlayback})
	authoringToken, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	ts := httptest.NewServer(server.Handler())
	t.Cleanup(ts.Close)
	client := openEventStream(t, ts, playbackToken, "")
	require.Equalf(t, http.StatusOK, client.resp.StatusCode, "expected the playback-scoped key to open the stream, got %d", client.resp.StatusCode)

	rec := doCreatePoolRequest(t, server.Handler(), authoringToken, "", "Alpha")
	require.Truef(t, rec.Code >= 200 && rec.Code < 300, "expected the authoring-scoped create to succeed, got %d (body: %s)", rec.Code, rec.Body.String())

	frame, err := client.next()
	require.NoError(t, err, "reading frame on the playback-scoped stream")
	eventType, route, _ := decodeDomainPayload(t, frame)
	require.Equalf(t, "pool", eventType, "expected the playback-scoped stream to still observe the authoring-domain event (D-11), got type=%q route=%q", eventType, route)
	require.Equalf(t, "pool create", route, "expected the playback-scoped stream to still observe the authoring-domain event (D-11), got type=%q route=%q", eventType, route)
}

// --- TestSSERevocationTick ---------------------------------------------------

// TestSSERevocationTick proves revoking an open stream's key closes that
// connection within one revocation-tick interval (T-07-12b).
func TestSSERevocationTick(t *testing.T) {
	original := api.EventRevocationTickInterval
	api.EventRevocationTickInterval = 20 * time.Millisecond
	t.Cleanup(func() { api.EventRevocationTickInterval = original })

	server, root, showPath := newEventsTestServer(t)
	token, keyID := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	ts := httptest.NewServer(server.Handler())
	t.Cleanup(ts.Close)
	client := openEventStream(t, ts, token, "")
	require.Equalf(t, http.StatusOK, client.resp.StatusCode, "expected the stream to open, got %d", client.resp.StatusCode)

	err := show.RevokeAPIKey(root, showPath, keyID)
	require.NoError(t, err, "RevokeAPIKey")

	_, err = client.nextWithTimeout(t, 2*time.Second)
	require.Error(t, err, "expected the connection to close (a read error) within one revocation-tick interval after revocation, got a frame instead")
}

// --- TestSSEBatchMultiSubRequestReconnectDeliversRemainingEvents -------------

// TestSSEBatchMultiSubRequestReconnectDeliversRemainingEvents reproduces
// 07-REVIEW.md CR-01 / 07-VERIFICATION.md's live gap: a 3-sub-request
// /v1/batch commits exactly ONE show revision (D-15) but fires THREE
// separately-addressable domain events, one per sub-request. A client
// that sees only the first of those three, disconnects, and reconnects
// presenting that frame's own id must receive the remaining two -- never
// "already caught up" -- proving the SSE id/replay key is a per-process
// sequence, not show.State.Revision (which all three events legitimately
// share).
func TestSSEBatchMultiSubRequestReconnectDeliversRemainingEvents(t *testing.T) {
	server, root, showPath := newEventsTestServer(t)
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	ts := httptest.NewServer(server.Handler())
	t.Cleanup(ts.Close)
	client := openEventStream(t, ts, token, "")
	require.Equalf(t, http.StatusOK, client.resp.StatusCode, "expected 200 opening the stream, got %d", client.resp.StatusCode)

	rec := doBatchRequest(t, server.Handler(), token, "", []map[string]any{
		poolCreateBatchSubRequest("Alpha"),
		poolCreateBatchSubRequest("Bravo"),
		poolCreateBatchSubRequest("Charlie"),
	})
	require.Truef(t, rec.Code >= 200 && rec.Code < 300, "expected the batch to succeed, got %d (body: %s)", rec.Code, rec.Body.String())

	first, err := client.next()
	require.NoError(t, err, "reading first live frame")
	require.Equalf(t, "state", first.Event, "expected the first live frame to be event=state, got %q", first.Event)
	_, firstRoute, firstRevision := decodeDomainPayload(t, first)
	require.Equalf(t, "pool create", firstRoute, "expected the first frame's route to be \"pool create\", got %q", firstRoute)
	client.close()

	reconnected := openEventStream(t, ts, token, first.ID)
	require.Equalf(t, http.StatusOK, reconnected.resp.StatusCode, "expected 200 reconnecting, got %d", reconnected.resp.StatusCode)

	var ids []int
	var revisions []int64
	for i := 0; i < 2; i++ {
		frame, err := reconnected.nextWithTimeout(t, 2*time.Second)
		require.NoErrorf(t, err, "reading remaining frame %d after reconnect", i)
		require.Equalf(t, "state", frame.Event, "expected remaining frame %d to be event=state, got %q", i, frame.Event)
		_, route, revision := decodeDomainPayload(t, frame)
		require.Equalf(t, "pool create", route, "expected remaining frame %d's route to be \"pool create\", got %q", i, route)
		id, err := strconv.Atoi(frame.ID)
		require.NoErrorf(t, err, "remaining frame %d's id %q did not parse as an integer", i, frame.ID)
		ids = append(ids, id)
		revisions = append(revisions, revision)
	}

	firstID, err := strconv.Atoi(first.ID)
	require.NoErrorf(t, err, "first frame's id %q did not parse as an integer", first.ID)
	require.Falsef(t, ids[0] == firstID || ids[1] == firstID || ids[0] == ids[1], "expected three distinct ids (first=%d, remaining=%v), got a duplicate", firstID, ids)
	require.Lessf(t, ids[0], ids[1], "expected the remaining ids in strictly ascending order, got %v", ids)
	for i, revision := range revisions {
		require.Equalf(t, firstRevision, revision, "expected remaining frame %d's revision (%d) to equal the first frame's revision (%d) -- the batch commits exactly one revision", i, revision, firstRevision)
	}
}

// --- TestSSEEventIDsStrictlyMonotonicAcrossBatchAndSingleMutation ------------

// TestSSEEventIDsStrictlyMonotonicAcrossBatchAndSingleMutation proves ids
// are strictly monotonic across mixed traffic -- a 2-sub-request batch
// followed by a single pool create yields ids 1, 2, 3 -- while the
// payload's domain `revision` field distinguishes the batch's single
// committed revision (1, 1) from the subsequent mutation's (2), proving
// the id sequence and the domain revision are now independently tracked.
func TestSSEEventIDsStrictlyMonotonicAcrossBatchAndSingleMutation(t *testing.T) {
	server, root, showPath := newEventsTestServer(t)
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	ts := httptest.NewServer(server.Handler())
	t.Cleanup(ts.Close)
	client := openEventStream(t, ts, token, "")
	require.Equalf(t, http.StatusOK, client.resp.StatusCode, "expected 200 opening the stream, got %d", client.resp.StatusCode)

	rec := doBatchRequest(t, server.Handler(), token, "", []map[string]any{
		poolCreateBatchSubRequest("Alpha"),
		poolCreateBatchSubRequest("Bravo"),
	})
	require.Truef(t, rec.Code >= 200 && rec.Code < 300, "expected the batch to succeed, got %d (body: %s)", rec.Code, rec.Body.String())
	single := doCreatePoolRequest(t, server.Handler(), token, "", "Charlie")
	require.Truef(t, single.Code >= 200 && single.Code < 300, "expected the single create to succeed, got %d (body: %s)", single.Code, single.Body.String())

	wantIDs := []string{"1", "2", "3"}
	wantRevisions := []int64{1, 1, 2}
	for i, wantID := range wantIDs {
		frame, err := client.next()
		require.NoErrorf(t, err, "reading frame %d", i)
		require.Equalf(t, "state", frame.Event, "expected frame %d to be event=state id=%s, got event=%q id=%q", i, wantID, frame.Event, frame.ID)
		require.Equalf(t, wantID, frame.ID, "expected frame %d to be event=state id=%s, got event=%q id=%q", i, wantID, frame.Event, frame.ID)
		_, _, revision := decodeDomainPayload(t, frame)
		require.Equalf(t, wantRevisions[i], revision, "expected frame %d's revision to be %d, got %d", i, wantRevisions[i], revision)
	}
}

// --- TestSSEFutureLastEventIDResyncs -----------------------------------------

// TestSSEFutureLastEventIDResyncs proves a client reconnecting with a
// Last-Event-ID greater than any id this process has ever issued (the
// daemon-restart case, now reachable because the sequence is per-process
// rather than persisted with the show) receives an explicit resync, not
// silence (D-10).
func TestSSEFutureLastEventIDResyncs(t *testing.T) {
	server, root, showPath := newEventsTestServer(t)
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	rec := doCreatePoolRequest(t, server.Handler(), token, "", "Alpha")
	require.Truef(t, rec.Code >= 200 && rec.Code < 300, "expected the create to succeed, got %d (body: %s)", rec.Code, rec.Body.String())

	ts := httptest.NewServer(server.Handler())
	t.Cleanup(ts.Close)
	client := openEventStream(t, ts, token, "9999")

	frame, err := client.nextWithTimeout(t, 2*time.Second)
	require.NoError(t, err, "reading resync frame")
	require.Equalf(t, "resync", frame.Event, "expected a resync event for a Last-Event-ID greater than any issued id, got event=%q data=%q", frame.Event, frame.Data)
	var decoded struct {
		Reason string `json:"reason"`
	}
	err = json.Unmarshal([]byte(frame.Data), &decoded)
	require.NoErrorf(t, err, "decode resync payload %q", frame.Data)
	require.NotEmpty(t, decoded.Reason, "expected the resync event to carry a non-empty reason")
}

// --- TestSSEScriptLifecycleEvent ---------------------------------------------

// TestSSEScriptLifecycleEvent proves 08-08-PLAN.md Task 2's new "script"
// SSE payload type is registered on this SAME global stream (no second
// endpoint): a call to api.PublishScriptLifecycleEvent -- the seam
// internal/script's Run calls at a run's start and terminal moments --
// arrives as an "event: script" frame carrying the run id/name/status/
// reason, on the identical connection a "state"/"resync" event would
// arrive on.
func TestSSEScriptLifecycleEvent(t *testing.T) {
	server, root, showPath := newEventsTestServer(t)
	token, _ := seedKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopeAuthoring})

	ts := httptest.NewServer(server.Handler())
	t.Cleanup(ts.Close)
	client := openEventStream(t, ts, token, "")
	require.Equalf(t, http.StatusOK, client.resp.StatusCode, "expected 200 opening the stream, got %d", client.resp.StatusCode)

	api.PublishScriptLifecycleEvent("run-123", "Chase", "terminated", "GOLC_SCRIPT_STOPPED_BY_USER: stop requested")

	frame, err := client.nextWithTimeout(t, 2*time.Second)
	require.NoError(t, err, "reading script lifecycle frame")
	require.Equalf(t, "script", frame.Event, "expected event=script, got event=%q data=%q", frame.Event, frame.Data)
	var scriptDecoded struct {
		RunID      string `json:"runId"`
		ScriptName string `json:"scriptName"`
		Status     string `json:"status"`
		Reason     string `json:"reason"`
	}
	err = json.Unmarshal([]byte(frame.Data), &scriptDecoded)
	require.NoErrorf(t, err, "decode script payload %q", frame.Data)
	require.Equalf(t, "run-123", scriptDecoded.RunID, "unexpected script payload: %+v", scriptDecoded)
	require.Equalf(t, "Chase", scriptDecoded.ScriptName, "unexpected script payload: %+v", scriptDecoded)
	require.Equalf(t, "terminated", scriptDecoded.Status, "unexpected script payload: %+v", scriptDecoded)
	require.Equal(t, "GOLC_SCRIPT_STOPPED_BY_USER: stop requested", scriptDecoded.Reason, "Reason")
}
