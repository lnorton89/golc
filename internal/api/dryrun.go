// dryrun.go implements ?dry_run=true previews (07-05-PLAN.md Task 2,
// CONTEXT D-14): mutate.go's pipeline branches here, before the real
// Execute, whenever a request carries DryRun. dryRunMutate Executes the
// exact same route/args against a throwaway, verified VACUUM INTO copy of
// the daemon's own show file (internal/show.NewTempCopy, reusing Phase
// 5's verified-backup machinery -- 07-RESEARCH.md Pattern 4) -- the real
// .golc file is never opened for write on this path (T-07-08). The copy
// is always deleted before returning, whether the previewed mutation
// would have succeeded or failed. A post-mutation observer always fires
// with outcome "dry_run" (never "success"/"failure"), and never carries a
// resulting revision, so 07-07's audit records the preview attempt
// without a success row and 07-08 emits no state-change SSE event.
package api

import (
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/lnorton89/golc/internal/show"
)

// dryRunMutate previews req's effect against a throwaway copy of server's
// own show file, guaranteed to leave the real show.State.Revision
// unchanged. It must be called from within mutate's already-held
// mutationMutex (mutate.go's own branch point), matching real mutations'
// serialization discipline even though a dry-run never durably writes to
// the real store.
func dryRunMutate(server *Server, req mutateRequest) (mutationResult, error) {
	tempShowPath, cleanup, copyErr := show.NewTempCopy(server.root, server.showPath)
	if copyErr != nil {
		wrapped := huma.Error500InternalServerError(copyErr.Error())
		fireMutationObservers(MutationEvent{
			Route: req.Route, Args: req.Args, Actor: req.Actor, Source: "http",
			CorrelationID: req.CorrelationID, Outcome: "dry_run", StatusCode: statusFromHumaErr(wrapped),
		})
		return mutationResult{}, wrapped
	}
	defer cleanup()

	args := buildMutationArgs(req.Args, tempShowPath)
	exitCode, stdout, stderr := server.executor.Execute(req.Route, args, server.root)
	body, translateErr := translateResult(exitCode, stdout, stderr)

	statusCode := http.StatusOK
	if translateErr != nil {
		statusCode = statusFromHumaErr(translateErr)
	}
	fireMutationObservers(MutationEvent{
		Route: req.Route, Args: req.Args, Actor: req.Actor, Source: "http",
		CorrelationID: req.CorrelationID, Outcome: "dry_run", StatusCode: statusCode,
	})

	if translateErr != nil {
		return mutationResult{}, translateErr
	}
	return mutationResult{Result: strings.TrimSpace(string(body))}, nil
}

// dryRunQueryDoc is shared by every mutating operation's DryRun input
// field doc string, kept in one place so the wording stays identical
// across every route this package registers.
const dryRunQueryDoc = "Preview this mutation's effect without applying it (D-14); the real show is never touched, and no resulting revision is reported."
