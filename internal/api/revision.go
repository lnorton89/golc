// revision.go implements If-Match parsing and the D-13 optimistic-
// concurrency check against show.State.Revision (07-05-PLAN.md Task 1,
// CONTEXT T-07-07). This check lives here, inside the mutation pipeline
// (mutate.go), deliberately NOT as a generic cross-cutting HTTP ETag
// middleware: If-Match here compares against the domain-meaningful
// show.State.Revision, not a content hash of the HTTP response body
// (07-RESEARCH.md Anti-Pattern "A generic HTTP ETag middleware for
// If-Match").
package api

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/lnorton89/golc/internal/show"
)

// parseIfMatch parses header in D-13's quoted-revision form (e.g.
// `"42"`), tolerating an unquoted bare integer too. An empty (or
// all-whitespace) header means "no revision check requested" (present is
// false); a non-empty header that is not a valid integer is a client
// error, never silently ignored.
func parseIfMatch(header string) (revision int64, present bool, err error) {
	trimmed := strings.TrimSpace(header)
	if trimmed == "" {
		return 0, false, nil
	}
	trimmed = strings.Trim(trimmed, `"`)
	value, convErr := strconv.ParseInt(trimmed, 10, 64)
	if convErr != nil {
		return 0, false, fmt.Errorf("GOLC_API_IF_MATCH_INVALID: %q is not a valid revision", header)
	}
	return value, true, nil
}

// checkRevision compares ifMatchHeader's parsed revision (if present)
// against show.CurrentRevision(root, showPath). It always returns the
// parsed expected revision (even on a mismatch or error, so the caller's
// MutationEvent.ExpectedRevision is populated for the audit trail) plus a
// typed Huma error: nil when the header was absent or matched, a 400 for
// an unparseable header, and a 412 Precondition Failed for a genuine
// mismatch (CONTEXT D-13; T-07-07 -- a mismatch must stop the pipeline
// before Execute, never mutate).
func checkRevision(root, showPath, ifMatchHeader string) (expected *int64, err error) {
	revision, present, parseErr := parseIfMatch(ifMatchHeader)
	if parseErr != nil {
		return nil, huma.Error400BadRequest(parseErr.Error())
	}
	if !present {
		return nil, nil
	}
	expected = &revision

	current, currentErr := show.CurrentRevision(root, showPath)
	if currentErr != nil {
		return expected, huma.Error500InternalServerError(currentErr.Error())
	}
	if revision != current {
		return expected, huma.Error412PreconditionFailed(fmt.Sprintf(
			"GOLC_API_REVISION_MISMATCH: If-Match %d does not match the current revision %d", revision, current))
	}
	return expected, nil
}
