// fetch_test.go proves D-07/T-02-06's SSRF-guarded OFL fetch contract
// (02-03-PLAN.md, Task 1 Wave-0 scaffold): a mirror URL with a non-http(s)
// scheme, or a non-default host without explicit --allow-mirror opt-in,
// is refused before any network request is made. Two bonus tests
// (TestFetchAllowsApprovedMirrorAndCaches, TestFetchRejectsOversizedResponse)
// exercise the approved-mirror success path and the response-size cap
// against a local httptest server, so this file never depends on live
// network access to run.
//
// This file compiles today only once package
// github.com/lnorton89/golc/internal/fixture/ofl exists; until Task 2/3
// create model.go/normalize.go/fetch.go, "go test ./internal/fixture/ofl/..."
// fails to build at all -- that is the RED state this task proves.
package ofl_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lnorton89/golc/internal/fixture/ofl"
)

// TestFetchRejectsBadScheme proves a non-http(s) mirror scheme is refused
// with GOLC_FIXTURE_OFL_MIRROR_SCHEME before any request is issued.
func TestFetchRejectsBadScheme(t *testing.T) {
	ref := ofl.OFLRef{
		Manufacturer: "chauvet-dj",
		Key:          "led-par-64-tri-b",
		Mirror:       "file:///etc/passwd",
		AllowMirror:  true,
	}
	_, err := ofl.Fetch(context.Background(), ref)
	if err == nil {
		t.Fatal("expected Fetch to reject a non-http(s) mirror scheme")
	}
	if !strings.Contains(err.Error(), "GOLC_FIXTURE_OFL_MIRROR_SCHEME") {
		t.Fatalf("expected GOLC_FIXTURE_OFL_MIRROR_SCHEME, got %v", err)
	}
}

// TestFetchRejectsUnapprovedHost proves a non-default mirror host without
// --allow-mirror is refused with GOLC_FIXTURE_OFL_MIRROR_HOST before any
// request is issued.
func TestFetchRejectsUnapprovedHost(t *testing.T) {
	ref := ofl.OFLRef{
		Manufacturer: "chauvet-dj",
		Key:          "led-par-64-tri-b",
		Mirror:       "https://example.com",
		AllowMirror:  false,
	}
	_, err := ofl.Fetch(context.Background(), ref)
	if err == nil {
		t.Fatal("expected Fetch to reject a non-default mirror host without --allow-mirror")
	}
	if !strings.Contains(err.Error(), "GOLC_FIXTURE_OFL_MIRROR_HOST") {
		t.Fatalf("expected GOLC_FIXTURE_OFL_MIRROR_HOST, got %v", err)
	}
}

// TestFetchAllowsApprovedMirrorAndCaches proves an explicitly
// --allow-mirror-opted-in host succeeds against a local httptest server
// (never live network) and returns exactly the server's bytes.
func TestFetchAllowsApprovedMirrorAndCaches(t *testing.T) {
	body := []byte(`{"name":"Test Fixture","availableChannels":{},"modes":[]}`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/fixtures/acme/test.json" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(body)
	}))
	defer server.Close()

	ref := ofl.OFLRef{Manufacturer: "acme", Key: "test", Mirror: server.URL, AllowMirror: true}
	got, err := ofl.Fetch(context.Background(), ref)
	if err != nil {
		t.Fatalf("expected an approved mirror fetch to succeed, got %v", err)
	}
	if string(got) != string(body) {
		t.Fatalf("expected fetched bytes to match the server response, got %q", got)
	}
}

// TestFetchRejectsOversizedResponse proves the response-size cap (T-02-06)
// rejects a response exceeding the limit with GOLC_FIXTURE_OFL_TOO_LARGE.
func TestFetchRejectsOversizedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		oversized := make([]byte, 3*1024*1024) // exceeds the 2 MiB cap
		_, _ = w.Write(oversized)
	}))
	defer server.Close()

	ref := ofl.OFLRef{Manufacturer: "acme", Key: "big", Mirror: server.URL, AllowMirror: true}
	_, err := ofl.Fetch(context.Background(), ref)
	if err == nil {
		t.Fatal("expected Fetch to reject an oversized response")
	}
	if !strings.Contains(err.Error(), "GOLC_FIXTURE_OFL_TOO_LARGE") {
		t.Fatalf("expected GOLC_FIXTURE_OFL_TOO_LARGE, got %v", err)
	}
}
