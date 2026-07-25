package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
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

// TestDeprecationMiddlewareEmitsHeadersOnLiveRequest proves the
// Deprecation/Sunset/Link headers docs/api/COMPATIBILITY.md documents as
// load-bearing client-detection signals are emitted by a real request
// through a real middleware chain (07-14-PLAN.md Task 1, closes
// 07-REVIEW.md WR-04) -- not merely computable by deprecationHeadersFor in
// isolation, which is what the earlier tests in this file exercised. It
// builds a chi router and a humachi huma API the same way buildRouter and
// generate.go do, installs only DeprecationMiddleware, and registers one
// operation MarkOperationDeprecated marked before huma.Register and one
// left untouched.
func TestDeprecationMiddlewareEmitsHeadersOnLiveRequest(t *testing.T) {
	router := chi.NewRouter()
	humaAPI := humachi.New(router, huma.DefaultConfig("Deprecation Test API", "1.0.0"))
	humaAPI.UseMiddleware(DeprecationMiddleware(humaAPI))

	sunset := time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC)
	link := "https://docs.example.com/migrate-to-v2"

	deprecatedOp := huma.Operation{
		OperationID: "get-deprecated-widget",
		Method:      http.MethodGet,
		Path:        "/deprecated-widget",
	}
	MarkOperationDeprecated(&deprecatedOp, DeprecationInfo{Sunset: sunset, Link: link})
	huma.Register(humaAPI, deprecatedOp, func(ctx context.Context, input *struct{}) (*struct{}, error) {
		return &struct{}{}, nil
	})

	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-current-widget",
		Method:      http.MethodGet,
		Path:        "/current-widget",
	}, func(ctx context.Context, input *struct{}) (*struct{}, error) {
		return &struct{}{}, nil
	})

	deprecatedRec := httptest.NewRecorder()
	router.ServeHTTP(deprecatedRec, httptest.NewRequest(http.MethodGet, "/deprecated-widget", nil))

	wantHeaders := deprecationHeadersFor(DeprecationInfo{Sunset: sunset, Link: link})
	for _, header := range []string{"Deprecation", "Sunset", "Link"} {
		if got, want := deprecatedRec.Header().Get(header), wantHeaders.Get(header); got != want {
			t.Fatalf("deprecated operation: expected %s: %q, got %q", header, want, got)
		}
	}

	currentRec := httptest.NewRecorder()
	router.ServeHTTP(currentRec, httptest.NewRequest(http.MethodGet, "/current-widget", nil))
	for _, header := range []string{"Deprecation", "Sunset", "Link"} {
		if got := currentRec.Header().Get(header); got != "" {
			t.Fatalf("non-deprecated operation: expected no %s header, got %q", header, got)
		}
	}
}

// TestBuildRouterInstallsDeprecationMiddleware is a deliberate source-
// discipline assertion, the same technique this repository already uses
// to pin the audit writer's single-writer discipline: huma's middleware
// chain is captured by value at registration time and offers no runtime
// introspection to assert against, so this test reads router.go's own
// source and asserts its single UseMiddleware call names
// DeprecationMiddleware alongside AuthMiddleware and RateLimitMiddleware.
// It fails the day that wiring is dropped from buildRouter (07-14-PLAN.md
// Task 1, closes 07-REVIEW.md WR-04).
func TestBuildRouterInstallsDeprecationMiddleware(t *testing.T) {
	source, err := os.ReadFile("router.go")
	if err != nil {
		t.Fatalf("os.ReadFile(router.go): %v", err)
	}
	text := string(source)

	useMiddlewareIdx := strings.Index(text, "humaAPI.UseMiddleware(")
	if useMiddlewareIdx < 0 {
		t.Fatal("expected router.go to contain a humaAPI.UseMiddleware( call")
	}
	// Find the matching close paren for the outer UseMiddleware( call by
	// tracking nesting depth -- a naive first-')' search would stop at the
	// inner AuthMiddleware(...) call's own close paren instead.
	openIdx := useMiddlewareIdx + len("humaAPI.UseMiddleware")
	depth := 0
	endIdx := -1
	for i := openIdx; i < len(text); i++ {
		switch text[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				endIdx = i
			}
		}
		if endIdx >= 0 {
			break
		}
	}
	if endIdx < 0 {
		t.Fatal("expected the humaAPI.UseMiddleware( call to close with a matching ')'")
	}
	call := text[useMiddlewareIdx : endIdx+1]

	for _, want := range []string{"AuthMiddleware(", "RateLimitMiddleware(", "DeprecationMiddleware("} {
		if !strings.Contains(call, want) {
			t.Fatalf("expected buildRouter's UseMiddleware call to include %s, got: %s", want, call)
		}
	}
}
