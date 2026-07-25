// deprecation.go implements D-02's compatibility/deprecation-window
// mechanism (07-09-PLAN.md Task 2, mirroring 07-RESEARCH.md Pattern 2): a
// helper that marks a Huma operation deprecated in the published OpenAPI
// document AND emits the Deprecation/Sunset response-header signals (per
// the emerging IETF drafts -- draft-ietf-httpapi-deprecation-header) on
// every real HTTP response for that operation. Nothing in this package is
// deprecated today (this plan ships /v1 only, no /v2 yet), so this file is
// scaffolded and unit-tested now, ready for a future breaking-change plan
// to apply the moment it mounts a parallel /v2 and starts /v1's documented
// deprecation window -- see docs/api/COMPATIBILITY.md for the policy this
// mechanism enforces.
package api

import (
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
)

// deprecationInfoMetadataKey is the huma.Operation.Metadata key
// MarkOperationDeprecated stashes a DeprecationInfo under, and
// DeprecationMiddleware reads back at request time. A metadata key (not a
// second side-table keyed by OperationID) keeps the deprecation data
// attached to the exact operation value it describes, so it can never
// silently drift out of sync with a renamed or removed operation.
const deprecationInfoMetadataKey = "golc-deprecation-info"

// httpDateLayout is RFC 7231's imf-fixdate format, the format both the
// Sunset header (RFC 8594) and the Deprecation header's optional date
// value use -- time.RFC1123 with a fixed "GMT" zone rather than a numeric
// offset, since imf-fixdate requires the literal "GMT" abbreviation.
const httpDateLayout = "Mon, 02 Jan 2006 15:04:05 GMT"

// DeprecationInfo carries the Sunset date and optional migration link a
// deprecated operation's responses advertise.
type DeprecationInfo struct {
	// Sunset is the date this operation stops being served. Formatted as
	// an HTTP-date in the Sunset response header (RFC 8594).
	Sunset time.Time
	// Link is an optional absolute URL to migration documentation, sent as
	// a Link response header with rel="deprecation" alongside
	// Deprecation/Sunset when non-empty.
	Link string
}

// MarkOperationDeprecated marks op deprecated in the generated OpenAPI
// document (op.Deprecated = true, the field every OpenAPI viewer and
// generated client already understands -- generate.go's own document
// carries this through unchanged) and attaches info as op.Metadata so
// DeprecationMiddleware can emit the matching response headers for every
// real request this operation serves.
func MarkOperationDeprecated(op *huma.Operation, info DeprecationInfo) {
	op.Deprecated = true
	if op.Metadata == nil {
		op.Metadata = map[string]any{}
	}
	op.Metadata[deprecationInfoMetadataKey] = info
}

// deprecationInfoFor returns the DeprecationInfo MarkOperationDeprecated
// attached to op, if any.
func deprecationInfoFor(op *huma.Operation) (DeprecationInfo, bool) {
	if op == nil || op.Metadata == nil {
		return DeprecationInfo{}, false
	}
	info, ok := op.Metadata[deprecationInfoMetadataKey].(DeprecationInfo)
	return info, ok
}

// DeprecationMiddleware returns Huma middleware that, for any request
// whose matched operation MarkOperationDeprecated marked, sets a
// "Deprecation: true" header (draft-ietf-httpapi-deprecation-header) and a
// "Sunset: <HTTP-date>" header (RFC 8594) naming the operation's
// DeprecationInfo.Sunset date -- plus a "Link: <url>; rel=\"deprecation\""
// header when DeprecationInfo.Link is set. A request against a
// non-deprecated operation passes through untouched. Intended to be
// installed via router.go's buildRouter UseMiddleware call (alongside
// AuthMiddleware/RateLimitMiddleware) once a real deprecation window
// begins; not installed today because no operation is deprecated yet.
func DeprecationMiddleware(humaAPI huma.API) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		info, deprecated := deprecationInfoFor(ctx.Operation())
		if deprecated {
			ctx.SetHeader("Deprecation", "true")
			ctx.SetHeader("Sunset", info.Sunset.UTC().Format(httpDateLayout))
			if info.Link != "" {
				ctx.SetHeader("Link", "<"+info.Link+">; rel=\"deprecation\"")
			}
		}
		next(ctx)
	}
}

// deprecationHeadersFor is a small, dependency-free helper exposing
// exactly the header set DeprecationMiddleware would emit for info,
// without requiring a live huma.Context -- used by this file's own unit
// test to assert the header contract directly against http.Header, and
// available to any future caller that needs the same values outside a
// request-handling path.
func deprecationHeadersFor(info DeprecationInfo) http.Header {
	headers := http.Header{}
	headers.Set("Deprecation", "true")
	headers.Set("Sunset", info.Sunset.UTC().Format(httpDateLayout))
	if info.Link != "" {
		headers.Set("Link", "<"+info.Link+">; rel=\"deprecation\"")
	}
	return headers
}
