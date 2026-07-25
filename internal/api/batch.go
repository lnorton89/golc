// batch.go implements the atomic POST /v1/batch operation (07-06-PLAN.md
// Task 1/2, CONTEXT D-15, 07-RESEARCH.md Pitfall 1): an ordered list of
// sub-requests applies to a throwaway copy of the daemon's own show file
// (internal/show.NewTempCopy, the exact same verified VACUUM INTO copy
// dry-run already uses -- dryrun.go, 07-05-PLAN.md Task 2), each Executed
// in client order against that copy via the same routed-command dispatch
// every mutating REST operation in this package uses. If every sub-request
// succeeds, the copy's aggregated final show.State is committed to the
// REAL show in exactly ONE show.Save call (one revision bump); if any
// sub-request fails, or the real show's revision changed underneath the
// batch between its start and its commit, the copy is discarded and the
// real show is left completely untouched -- no internal/command handler is
// ever modified, and no sub-request's effect is ever durably applied on
// its own (07-RESEARCH.md Pitfall 1's "copy + single aggregated Save"
// strategy, chosen over the ~15-file State-in/State-out refactor).
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/lnorton89/golc/internal/show"
)

// BatchPreCommitHookForTesting, when non-nil, is called by runBatch
// immediately before its final external-write race check (the re-read of
// show.CurrentRevision against the real show, run right before the
// aggregated show.Save) -- a test-only seam for simulating a concurrent
// external writer (e.g. a CLI process calling show.Save directly against
// the real path, entirely outside this package's mutationMutex, which only
// ever serializes HTTP requests within this one process) racing between a
// batch's start and its commit (07-RESEARCH.md Pitfall 1 residual race).
// Production code must never set this; tests must always restore it to nil
// (e.g. via t.Cleanup) once done.
var BatchPreCommitHookForTesting func()

// batchSubRequest is one sub-request's client-supplied shape within a
// POST /v1/batch request body: Method+Resource identify which
// already-registered single-mutation REST operation this sub-request
// stands in for (batchTranslators below), and Body carries that
// operation's own JSON body shape verbatim -- the "same translate.go path
// as single mutations" the plan's action text requires, reused for a
// batch's sub-requests instead of a live HTTP request.
type batchSubRequest struct {
	Method   string          `json:"method" required:"true" doc:"The HTTP method the equivalent single-mutation operation uses, e.g. \"POST\"."`
	Resource string          `json:"resource" required:"true" doc:"The equivalent single-mutation operation's own REST path, e.g. \"/v1/pools\"."`
	Body     json.RawMessage `json:"body,omitempty" doc:"This sub-request's JSON body, matching the equivalent single-mutation operation's own body shape."`
}

// batchTranslator turns one batchSubRequest's raw Body into the routed
// command + args mutate.go's own pipeline would build for the equivalent
// single-mutation HTTP request.
type batchTranslator func(body json.RawMessage) (route string, args []string, err error)

// batchTranslators maps "METHOD RESOURCE" to the translator for the
// equivalent single-mutation operation already registered in this package.
// Only "POST /v1/pools" (mutate.go's registerCreatePool) exists today,
// mirroring 07-05-SUMMARY.md's own documented scope decision (only "pool
// create" is wired as a concrete mutating REST operation so far) -- a
// future plan wiring another mutating route onto the single-mutation
// pipeline adds its own entry here alongside it.
var batchTranslators = map[string]batchTranslator{
	"POST /v1/pools": translateBatchCreatePool,
}

// createPoolBatchBody mirrors createPoolInput.Body (mutate.go) field for
// field -- kept as its own local type rather than importing mutate.go's
// unexported struct, since a batch sub-request's Body is decoded from raw
// JSON bytes, not bound through Huma's own request-body decoding.
type createPoolBatchBody struct {
	Name     string   `json:"name"`
	Requires []string `json:"requires,omitempty"`
}

// translateBatchCreatePool decodes body into createPoolBatchBody and
// builds the exact "pool create" args mutate.go's registerCreatePool
// closure builds for the equivalent live POST /v1/pools request. Errors
// here are typed 400s (client input problems), matching translateResult's
// own exitCode==2 -> 400 convention for a malformed/unroutable invocation.
func translateBatchCreatePool(body json.RawMessage) (route string, args []string, err error) {
	var parsed createPoolBatchBody
	if len(body) > 0 {
		if decodeErr := json.Unmarshal(body, &parsed); decodeErr != nil {
			return "", nil, huma.Error400BadRequest(fmt.Sprintf("GOLC_API_BATCH_SUBREQUEST_BODY_INVALID: %v", decodeErr))
		}
	}
	if parsed.Name == "" {
		return "", nil, huma.Error400BadRequest("GOLC_API_BATCH_SUBREQUEST_BODY_INVALID: \"name\" is required")
	}
	// Same boundary rule mutate.go's registerCreatePool applies to the
	// equivalent single-mutation request, called before this function's own
	// join -- one rule, not two copies (IN-02, 07-REVIEW.md).
	if err := validateListValues("requires", parsed.Requires); err != nil {
		return "", nil, err
	}
	args = []string{parsed.Name}
	if len(parsed.Requires) > 0 {
		args = append(args, "--requires", strings.Join(parsed.Requires, ","))
	}
	return "pool create", args, nil
}

// translateBatchSubRequest resolves sub's routed command + args via
// batchTranslators, a typed 400 (GOLC_API_BATCH_SUBREQUEST_UNSUPPORTED)
// for a method+resource pair with no registered translator.
func translateBatchSubRequest(sub batchSubRequest) (route string, args []string, err error) {
	translator, ok := batchTranslators[sub.Method+" "+sub.Resource]
	if !ok {
		return "", nil, huma.Error400BadRequest(fmt.Sprintf(
			"GOLC_API_BATCH_SUBREQUEST_UNSUPPORTED: %s %s is not a supported batch sub-request", sub.Method, sub.Resource))
	}
	return translator(sub.Body)
}

// batchSubRequestError shapes a failing sub-request's typed Huma error:
// its message names the failing index, the underlying diagnostic, and how
// many earlier sub-requests succeeded against the throwaway copy but were
// NOT durably applied -- because the whole batch (including those earlier
// successes) is rolled back on any failure (Task 2's own acceptance
// criteria). status mirrors whatever HTTP status the underlying failure
// would have produced for the equivalent single-mutation request
// (statusFromHumaErr), so a malformed sub-request body still reports 400
// and a domain-level handler failure still reports 5xx, just wrapped with
// batch context.
func batchSubRequestError(index, succeededSoFar int, err error) error {
	status := statusFromHumaErr(err)
	diagnostic := err.Error()
	message := fmt.Sprintf(
		"GOLC_API_BATCH_SUBREQUEST_FAILED: sub-request %d failed (%s); the whole batch was rolled back -- %d earlier sub-request(s) that succeeded against the throwaway copy were NOT durably applied",
		index, diagnostic, succeededSoFar)
	return huma.NewError(status, message, &huma.ErrorDetail{
		Message:  diagnostic,
		Location: fmt.Sprintf("body.requests[%d]", index),
	})
}

// batchResultItem is one committed sub-request's outcome in a successful
// batch's response body.
type batchResultItem struct {
	Index  int    `json:"index"`
	Result string `json:"result"`
}

// batchOutput is POST /v1/batch's Huma output for a fully-applied batch:
// Results carries every sub-request's own raw stdout (trimmed, index-
// ordered), Revision is the real show's single resulting revision after
// the batch's one aggregated Save.
type batchOutput struct {
	Body struct {
		Results  []batchResultItem `json:"results"`
		Revision *int64            `json:"revision,omitempty"`
	}
}

// batchInput is POST /v1/batch's Huma input: IfMatch carries D-13's
// batch-level expected-revision precondition (checked once at the batch's
// start, and re-verified immediately before its final commit -- Pitfall 1
// residual race).
type batchInput struct {
	IfMatch string `header:"If-Match" doc:"Expected show.State.Revision for the whole batch, quoted per RFC 7232 (D-13). Omit to skip the batch-level optimistic-concurrency check."`
	Body    struct {
		Requests []batchSubRequest `json:"requests" doc:"Ordered list of sub-requests applied atomically: all-or-nothing (D-15). Must be non-empty."`
	}
}

// runBatch is POST /v1/batch's engine (07-06-PLAN.md Task 1's action text,
// steps 1-6). It never partially applies: every check before the
// aggregated show.Save runs against a throwaway copy or in-memory state
// only, and the real show is written to exactly once, only after every
// sub-request has already succeeded against the copy and the batch's
// expected base revision is confirmed to still be current. Every one of
// the nine failure returns inside the locked section (from
// mutationMutex.Lock() through the aggregated show.Save) fires its own
// failure MutationEvent(s) before returning (API-06, 07-15-PLAN.md, closing
// WR-05): a BATCH-LEVEL failure -- one not attributable to any individual
// sub-request -- writes one failure row per sub-request via
// fireBatchFailureObservers, while a SUB-REQUEST-LEVEL failure (the
// translateResult error inside the execution loop below) writes exactly
// one row, for the culpable index alone.
func runBatch(ctx context.Context, server *Server, ifMatch string, requests []batchSubRequest) (*batchOutput, error) {
	if len(requests) == 0 {
		return nil, huma.Error400BadRequest("GOLC_API_BATCH_EMPTY: a batch must carry at least one sub-request")
	}

	actor := actorFromContext(ctx)
	correlationID := correlationIDFromContext(ctx)

	// Translate every sub-request and check its required domain scope up
	// front, before ever touching mutationMutex or copying the real show:
	// a translation or scope failure here has touched nothing durable, so
	// there is nothing to roll back (Rule 2 -- D-08 must apply to a
	// batch's sub-requests exactly as it applies to the equivalent single
	// mutation, otherwise a batch would be a scope-bypass vector). Each of
	// this loop's three early returns fires a failure MutationEvent before
	// returning, mirroring mutate.go's own scope-failure branch, so a
	// rejected batch sub-request leaves the same audit evidence the
	// equivalent rejected single mutation already does (API-06,
	// 07-REVIEW.md WR-02) -- these observers deliberately fire before
	// mutationMutex is ever acquired (nothing durable was ever at risk on
	// any of these three paths, since no copy of the real show has been
	// taken yet), so a rejected batch's audit row carries no ordering
	// guarantee relative to concurrently-committing mutations' rows, only
	// the guarantee that the row exists.
	routes := make([]string, len(requests))
	args := make([][]string, len(requests))
	for i, req := range requests {
		route, reqArgs, translateErr := translateBatchSubRequest(req)
		if translateErr != nil {
			// No command route was ever resolved, so the audit row records
			// the client's own claimed target (method + resource) instead --
			// the only routing identity available. Recording it verbatim,
			// rather than dropping the row or inventing a synthetic route
			// name, keeps an unsupported-endpoint probe auditable; a reader
			// can distinguish this from a real command route because real
			// routes never contain a slash.
			fireMutationObservers(MutationEvent{
				Route: req.Method + " " + req.Resource, Actor: actor, Source: "http",
				CorrelationID: correlationID, Outcome: "failure", StatusCode: statusFromHumaErr(translateErr),
			})
			return nil, batchSubRequestError(i, 0, translateErr)
		}
		requiredScope, scopeLookupErr := requiredScopeForRoute(route)
		if scopeLookupErr != nil {
			fireMutationObservers(MutationEvent{
				Route: route, Args: reqArgs, Actor: actor, Source: "http",
				CorrelationID: correlationID, Outcome: "failure", StatusCode: http.StatusInternalServerError,
			})
			return nil, batchSubRequestError(i, 0, huma.Error500InternalServerError(scopeLookupErr.Error()))
		}
		if scopeErr := RequireScope(ctx, requiredScope); scopeErr != nil {
			fireMutationObservers(MutationEvent{
				Route: route, Args: reqArgs, Actor: actor, Source: "http",
				CorrelationID: correlationID, Outcome: "failure", StatusCode: statusFromHumaErr(scopeErr),
			})
			return nil, batchSubRequestError(i, 0, scopeErr)
		}
		routes[i] = route
		args[i] = reqArgs
	}

	// expectedRevisionPtr carries the client's claimed If-Match revision for
	// every locked-section audit row (success or failure) below. Declared
	// once, here, and read at CALL time by every fire site below --
	// including fireBatchFailureObservers, defined next -- which is exactly
	// why the two call sites that precede If-Match parsing (the initial
	// revision read and the parse failure itself) record a NULL expected
	// revision while the seven call sites that follow it record the
	// client's claimed revision.
	var expectedRevisionPtr *int64

	// fireBatchFailureObservers fans out one failure MutationEvent per
	// sub-request in the batch: a BATCH-LEVEL failure (any of the eight
	// call sites below) is not attributable to any single sub-request, so
	// every sub-request in the batch gets its own failure row -- one cause,
	// N rejected attempts (mirrors the success fan-out below, just with
	// Outcome "failure", StatusCode statusCode, and no ResultingRevision --
	// nothing was durably applied). It is defined here, before
	// mutationMutex.Lock(), only so that the locked region below contains
	// nothing but its call sites; every one of its actual invocations still
	// happens with the mutex held, so unlike this file's pre-flight fires
	// above (fired before the lock, carrying no ordering guarantee against
	// concurrently-committing mutations), these rows ARE strictly ordered
	// against them.
	fireBatchFailureObservers := func(statusCode int) {
		for i, route := range routes {
			fireMutationObservers(MutationEvent{
				Route: route, Args: args[i], Actor: actor, Source: "http",
				CorrelationID: correlationID, ExpectedRevision: expectedRevisionPtr,
				Outcome: "failure", StatusCode: statusCode,
			})
		}
	}

	mutationMutex.Lock()
	defer mutationMutex.Unlock()

	baseRevision, revErr := show.CurrentRevision(server.root, server.showPath)
	if revErr != nil {
		fireBatchFailureObservers(http.StatusInternalServerError)
		return nil, huma.Error500InternalServerError(revErr.Error())
	}

	expectedRevision, ifMatchPresent, parseErr := parseIfMatch(ifMatch)
	if parseErr != nil {
		fireBatchFailureObservers(http.StatusBadRequest)
		return nil, huma.Error400BadRequest(parseErr.Error())
	}
	if ifMatchPresent {
		expectedRevisionPtr = &expectedRevision
	}
	if ifMatchPresent && expectedRevision != baseRevision {
		fireBatchFailureObservers(http.StatusPreconditionFailed)
		return nil, huma.Error412PreconditionFailed(fmt.Sprintf(
			"GOLC_API_REVISION_MISMATCH: If-Match %d does not match the current revision %d", expectedRevision, baseRevision))
	}

	tempShowPath, cleanup, copyErr := show.NewTempCopy(server.root, server.showPath)
	if copyErr != nil {
		fireBatchFailureObservers(http.StatusInternalServerError)
		return nil, huma.Error500InternalServerError(copyErr.Error())
	}
	defer cleanup()

	results := make([]batchResultItem, len(requests))
	for i := range requests {
		execArgs := buildMutationArgs(args[i], tempShowPath)
		exitCode, stdout, stderr := server.executor.Execute(routes[i], execArgs, server.root)
		body, translateErr := translateResult(exitCode, stdout, stderr)
		if translateErr != nil {
			// Attributable to sub-request i alone: exactly one row, for the
			// culpable index, matching the semantic 07-12 established for
			// pre-flight rejections. Earlier sub-requests that succeeded
			// against the throwaway copy get no row -- nothing of theirs was
			// ever durably applied and no attempt of theirs failed.
			fireMutationObservers(MutationEvent{
				Route: routes[i], Args: args[i], Actor: actor, Source: "http",
				CorrelationID: correlationID, ExpectedRevision: expectedRevisionPtr,
				Outcome: "failure", StatusCode: statusFromHumaErr(translateErr),
			})
			return nil, batchSubRequestError(i, i, translateErr)
		}
		results[i] = batchResultItem{Index: i, Result: strings.TrimSpace(string(body))}
	}

	finalState, loadErr := show.Load(server.root, tempShowPath)
	if loadErr != nil {
		fireBatchFailureObservers(http.StatusInternalServerError)
		return nil, huma.Error500InternalServerError(loadErr.Error())
	}

	if BatchPreCommitHookForTesting != nil {
		BatchPreCommitHookForTesting()
	}

	// The pre-commit re-read of the real show's revision, and the race
	// comparison that follows, are two distinct failure returns -- an error
	// reading the revision itself (raceErr, a 500) versus a successfully
	// read revision that no longer matches baseRevision (a 412). Both fire
	// their own audit rows; collapsing them into one branch is exactly the
	// miscount 07-15-PLAN.md's own scope-correction note fixes.
	raceRevision, raceErr := show.CurrentRevision(server.root, server.showPath)
	if raceErr != nil {
		fireBatchFailureObservers(http.StatusInternalServerError)
		return nil, huma.Error500InternalServerError(raceErr.Error())
	}
	if raceRevision != baseRevision {
		// Every sub-request had already succeeded against the throwaway
		// copy and is now being rolled back: the fan-out records that the
		// whole batch was attempted and rolled back, not that nothing was
		// ever attempted.
		fireBatchFailureObservers(http.StatusPreconditionFailed)
		return nil, huma.Error412PreconditionFailed(fmt.Sprintf(
			"GOLC_API_REVISION_MISMATCH: the real show's revision changed from %d to %d while this batch was applying; "+
				"the batch was rolled back -- no sub-request's effect was durably applied", baseRevision, raceRevision))
	}

	// finalState.Revision currently reflects the throwaway copy's own
	// internal Save-per-sub-request bookkeeping (baseRevision + N for N
	// successful sub-requests against the copy); reset it to the real
	// show's own current revision so the single show.Save below produces
	// exactly ONE real revision bump (baseRevision -> baseRevision + 1),
	// regardless of how many sub-requests the batch carried (D-15's
	// "single atomic transaction").
	finalState.Revision = int(baseRevision)

	if saveErr := show.Save(server.root, server.showPath, finalState); saveErr != nil {
		fireBatchFailureObservers(http.StatusInternalServerError)
		return nil, huma.Error500InternalServerError(saveErr.Error())
	}

	resultingRevision := baseRevision + 1
	// expectedRevisionPtr is the same hoisted variable every locked-section
	// failure fire above already read; the success fan-out below sees the
	// identical value it would have computed locally, so this is a pure
	// dedup -- no audit-row or SSE-event content changes here.
	for i, route := range routes {
		fireMutationObservers(MutationEvent{
			Route: route, Args: args[i], Actor: actor, Source: "http",
			CorrelationID: correlationID, ExpectedRevision: expectedRevisionPtr,
			ResultingRevision: &resultingRevision, Outcome: "success", StatusCode: http.StatusOK,
		})
	}

	output := &batchOutput{}
	output.Body.Results = results
	output.Body.Revision = &resultingRevision
	return output, nil
}

// registerBatch wires POST /v1/batch onto humaAPI.
func registerBatch(humaAPI huma.API, server *Server) {
	huma.Register(humaAPI, huma.Operation{
		OperationID: "run-batch",
		Method:      http.MethodPost,
		Path:        apiPathPrefix + "/batch",
		Summary:     "Apply an ordered list of sub-requests as a single atomic transaction (all-or-nothing, D-15).",
	}, func(ctx context.Context, input *batchInput) (*batchOutput, error) {
		return runBatch(ctx, server, input.IfMatch, input.Body.Requests)
	})
}

// "batch apply" is not a real internal/command route (batch is an
// API-only capability that fans out to N already-registered routes, each
// with its own real command route); coverage_test.go's TestCapabilityCoverage
// only requires every REAL command-registry route to be covered or
// excluded, never the reverse, so registering a synthetic Route name here
// is safe and does not need its own excludedRoutes entry.
var _ = RegisterOperation(OperationRegistration{Route: "batch apply", Register: registerBatch})
