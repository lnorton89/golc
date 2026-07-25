package api

import (
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
)

// TestMarkOperationDeprecatedSetsOpenAPIFlag proves MarkOperationDeprecated
// sets the OpenAPI-visible Deprecated flag every viewer/generated client
// already understands.
func TestMarkOperationDeprecatedSetsOpenAPIFlag(t *testing.T) {
	op := &huma.Operation{OperationID: "get-widget"}
	sunset := time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC)

	MarkOperationDeprecated(op, DeprecationInfo{Sunset: sunset})

	if !op.Deprecated {
		t.Fatal("expected op.Deprecated to be true after MarkOperationDeprecated")
	}
	info, ok := deprecationInfoFor(op)
	if !ok {
		t.Fatal("expected deprecationInfoFor to find the attached DeprecationInfo")
	}
	if !info.Sunset.Equal(sunset) {
		t.Fatalf("expected Sunset %v, got %v", sunset, info.Sunset)
	}
}

// TestDeprecationInfoForUnmarkedOperation proves a normal, never-marked
// operation reports no deprecation info -- the common case for every /v1
// operation this plan ships.
func TestDeprecationInfoForUnmarkedOperation(t *testing.T) {
	op := &huma.Operation{OperationID: "get-widget"}
	if _, ok := deprecationInfoFor(op); ok {
		t.Fatal("expected an unmarked operation to report no DeprecationInfo")
	}
	if _, ok := deprecationInfoFor(nil); ok {
		t.Fatal("expected a nil operation to report no DeprecationInfo without panicking")
	}
}

// TestDeprecationHeadersForSunsetAndLink proves the exact header set D-02's
// documented policy (COMPATIBILITY.md) requires: Deprecation: true,
// Sunset: <RFC 7231 imf-fixdate>, and an optional Link: <url>; rel="deprecation".
func TestDeprecationHeadersForSunsetAndLink(t *testing.T) {
	sunset := time.Date(2027, time.June, 15, 12, 30, 0, 0, time.UTC)

	headers := deprecationHeadersFor(DeprecationInfo{Sunset: sunset})
	if got := headers.Get("Deprecation"); got != "true" {
		t.Fatalf("expected Deprecation: true, got %q", got)
	}
	if got, want := headers.Get("Sunset"), "Tue, 15 Jun 2027 12:30:00 GMT"; got != want {
		t.Fatalf("expected Sunset: %q, got %q", want, got)
	}
	if got := headers.Get("Link"); got != "" {
		t.Fatalf("expected no Link header when Link is unset, got %q", got)
	}

	withLink := deprecationHeadersFor(DeprecationInfo{
		Sunset: sunset,
		Link:   "https://docs.example.com/migrate-to-v2",
	})
	if got, want := withLink.Get("Link"), `<https://docs.example.com/migrate-to-v2>; rel="deprecation"`; got != want {
		t.Fatalf("expected Link: %q, got %q", want, got)
	}
}

// TestDeprecationHeadersUseUTC proves a non-UTC Sunset time is normalized
// to UTC before formatting (RFC 7231 imf-fixdate is always GMT).
func TestDeprecationHeadersUseUTC(t *testing.T) {
	loc := time.FixedZone("UTC-5", -5*60*60)
	sunset := time.Date(2027, time.March, 10, 9, 0, 0, 0, loc) // 14:00 UTC

	headers := deprecationHeadersFor(DeprecationInfo{Sunset: sunset})
	if got, want := headers.Get("Sunset"), "Wed, 10 Mar 2027 14:00:00 GMT"; got != want {
		t.Fatalf("expected Sunset normalized to UTC: %q, got %q", want, got)
	}
}
