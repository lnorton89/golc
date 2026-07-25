// audit.go implements the redacting post-mutation audit observer (07-07-
// PLAN.md Task 2, CONTEXT D-16, API-06): for every MutationEvent
// observer.go's fireMutationObservers fans out -- success, failure,
// rejected, dry-run, idempotent-replay, and each of an atomic batch's
// (07-06) per-sub-request events -- this file builds one AuditRecord and
// writes it through show.WriteAuditRecord, never a direct sql.Open (Task 1
// already owns that single-writer discipline). Details are redacted BEFORE
// serialization (A5, 07-RESEARCH.md Security Domain: "strip anything
// matching the request's own Authorization header and any field name
// containing key/token/secret/password") -- this file never stores a raw
// value and redacts it later at read time.
package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/lnorton89/golc/internal/security"
	"github.com/lnorton89/golc/internal/show"
)

// redactedPlaceholder replaces any value this file's redactor flags,
// matching internal/security's own "<redacted>" marker convention
// (redact.go's unexported redactedMarker) so a reader sees one consistent
// redaction marker across every diagnostic and audit surface in this
// repository, never a truncated fragment of the original value.
const redactedPlaceholder = "<redacted>"

// sensitiveFlagSubstrings are the credential-shaped flag-name fragments
// (case-insensitive) an audited mutation's args are checked against
// (07-RESEARCH.md Open Question 3 [ASSUMED]: "start narrow -- credential-
// shaped names only -- and let discuss-phase/UAT widen the list if a real
// sensitive field surfaces").
var sensitiveFlagSubstrings = []string{"key", "token", "secret", "password"}

// isSensitiveFlagName reports whether name (a "--flag"-style arg with its
// leading dashes already stripped, case-insensitive) matches any
// credential-shaped substring.
func isSensitiveFlagName(name string) bool {
	lower := strings.ToLower(name)
	for _, substr := range sensitiveFlagSubstrings {
		if strings.Contains(lower, substr) {
			return true
		}
	}
	return false
}

// redactArgs returns a copy of args with every credential-shaped value
// stripped before it is ever serialized (A5): a "--<flag>" arg whose own
// name contains key/token/secret/password redacts the value immediately
// following it (the flag's own value, e.g. "--token", "raw-value"), and
// internal/security.Redact additionally scans every individual value for
// this repository's centrally-owned forbidden-token patterns (Bearer
// auth headers, api-key prefixes, etc. -- redact.go's own
// forbiddenPatterns), so a raw Authorization header value or bearer token
// is stripped even when it does not follow a credential-named flag.
func redactArgs(args []string) []string {
	redacted := make([]string, len(args))
	for i, arg := range args {
		redacted[i] = security.Redact(arg)
	}
	for i, arg := range args {
		flag, isFlag := strings.CutPrefix(arg, "--")
		if isFlag && isSensitiveFlagName(flag) && i+1 < len(redacted) {
			redacted[i+1] = redactedPlaceholder
		}
	}
	return redacted
}

// auditDetails is the canonical shape redacted_details serializes: just
// enough to reconstruct what was attempted (route + args) without ever
// carrying a secret past this file's redaction step.
type auditDetails struct {
	Route string   `json:"route"`
	Args  []string `json:"args,omitempty"`
}

// buildRedactedDetails builds ev's redacted_details JSON, with every
// credential-shaped field already stripped (A5: strip-before-write, never
// stored raw and redacted at read time).
func buildRedactedDetails(ev MutationEvent) (string, error) {
	payload, err := json.Marshal(auditDetails{Route: ev.Route, Args: redactArgs(ev.Args)})
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

// nullableRevision converts a MutationEvent's *int64 revision field
// (nil for "does not apply") to the sql.NullInt64 AuditRecord stores, so a
// genuinely-absent revision is a true SQL NULL rather than a sentinel 0
// that would be indistinguishable from a real revision 0.
func nullableRevision(value *int64) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *value, Valid: true}
}

// auditObserver returns the post-mutation observer (registered via
// RegisterAuditObserver) that writes one redacted audit_log row per
// MutationEvent, through show.WriteAuditRecord, against the show file at
// showPath (resolved against root) -- the same show file the mutation
// itself was attempted against.
func auditObserver(root, showPath string) func(MutationEvent) {
	return func(ev MutationEvent) {
		redactedDetails, redactErr := buildRedactedDetails(ev)
		if redactErr != nil {
			// A redaction/serialization failure must never lose the row's
			// other accountable fields, nor risk leaking a raw arg by
			// falling back to some unredacted representation -- fall back
			// to a fixed, still-safe placeholder instead.
			redactedDetails = `{"route":"` + ev.Route + `","args":"<redaction failed>"}`
		}
		rec := show.AuditRecord{
			OccurredAt:        time.Now().UTC().Format(time.RFC3339),
			Actor:             ev.Actor,
			Source:            ev.Source,
			CorrelationID:     ev.CorrelationID,
			Route:             ev.Route,
			ExpectedRevision:  nullableRevision(ev.ExpectedRevision),
			ResultingRevision: nullableRevision(ev.ResultingRevision),
			Outcome:           ev.Outcome,
			StatusCode:        ev.StatusCode,
			RedactedDetails:   redactedDetails,
		}
		// Best-effort: an audit-write failure must never fail or reverse
		// the mutation it is recording (the write happens strictly after
		// mutate.go's own pipeline has already committed its outcome), and
		// must never crash the daemon process that also owns deterministic
		// playback and Art-Net output -- mirrors server.go's own isolated-
		// background-goroutine doctrine for a post-bind Serve failure.
		if writeErr := show.WriteAuditRecord(root, showPath, rec); writeErr != nil {
			fmt.Fprintf(os.Stderr, "GOLC_API_AUDIT_WRITE_FAILED: %v\n", writeErr)
		}
	}
}

// RegisterAuditObserver wires the redacting audit writer onto the post-
// mutation observer seam (observer.go's RegisterMutationObserver) for the
// show file at showPath (resolved against root): every mutation attempted
// against that show -- success, failure, rejected, dry-run, and each of an
// atomic batch's (07-06) per-sub-request events -- writes exactly one
// audit_log row (D-16, API-06). Intended to be called once per daemon
// *Server at startup wiring time (observer.go's own doc comment: "call
// once per observer at daemon-startup wiring time"), mirroring 07-08's own
// SSE observer registration; no edit to mutate.go/batch.go is needed to
// wire this in.
func RegisterAuditObserver(root, showPath string) {
	RegisterMutationObserver(auditObserver(root, showPath))
}
