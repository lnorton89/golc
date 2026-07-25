// translate_test.go exercises translate.go's HTTP<->routed-command
// translation end-to-end through a *api.Server's real Chi/Huma handler
// (httptest.NewRecorder against server.Handler(), no real network
// listener needed for these in-process checks). It lives in the external
// api_test package (see coverage_test.go's doc comment for why) so
// TestParity can reach a real command registry through
// internal/routecatalog's test-only bridge.
package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/lnorton89/golc/internal/api"
	"github.com/lnorton89/golc/internal/routecatalog"
)

// stubExecutor is a minimal api.Executor whose canned outcome and last
// invocation are both inspectable, for tests that exercise translate.go's
// HTTP<->exit-code mapping and argument-building without needing a real
// command registry.
type stubExecutor struct {
	lastRoute string
	lastArgs  []string
	lastRoot  string

	exitCode int
	stdout   []byte
	stderr   []byte
}

func (s *stubExecutor) Execute(route string, args []string, root string) (int, []byte, []byte) {
	s.lastRoute = route
	s.lastArgs = append([]string(nil), args...)
	s.lastRoot = root
	return s.exitCode, s.stdout, s.stderr
}

// doGet issues a GET against handler in-process (httptest.NewRecorder)
// and returns the recorded response.
func doGet(t *testing.T, handler http.Handler, target string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// --- TestParity ----------------------------------------------------------

const parityRootIndex = `schema_version = 2

[[concerns]]
id = "runtime"
path = "config/runtime.toml"
`

const parityRuntimeConcern = `schema_version = 2

[runtime]
log_level = "info"
`

// newParityRepository builds a minimal, self-contained repository root
// (mirrors internal/projectconfig's own load_test.go fixture) so this
// test never depends on this checkout's own golc.project.toml staying
// unchanged.
func newParityRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, root, "golc.project.toml", parityRootIndex)
	writeFile(t, root, "config/runtime.toml", parityRuntimeConcern)
	return root
}

func writeFile(t *testing.T, root, relative, content string) {
	t.Helper()
	target := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", relative, err)
	}
	if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", relative, err)
	}
}

// decodeJSON unmarshals body into a generic value for structural
// (not byte-literal) comparison -- robust to incidental whitespace
// differences between the two encoding paths under test.
func decodeJSON(t *testing.T, label string, body []byte) any {
	t.Helper()
	var value any
	if err := json.Unmarshal(body, &value); err != nil {
		t.Fatalf("%s: json.Unmarshal(%q): %v", label, body, err)
	}
	return value
}

// TestParity proves GET /v1/config/{concern} returns the same JSON
// outcome "config inspect <concern>" produces via a direct Execute call
// on the same root -- HTTP -> command.Execute -> response parity
// (API-01, this plan's guaranteed-route parity gate).
func TestParity(t *testing.T) {
	root := newParityRepository(t)
	catalog, err := routecatalog.New()
	if err != nil {
		t.Fatalf("routecatalog.New: %v", err)
	}

	server := api.NewServer(catalog, root, filepath.Join(root, "show.golc"))

	rec := doGet(t, server.Handler(), "/v1/config/runtime")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/config/runtime: expected 200, got %d (body: %s)", rec.Code, rec.Body.String())
	}

	exitCode, direct, stderr := catalog.Execute("config inspect", []string{"runtime"}, root)
	if exitCode != 0 {
		t.Fatalf("direct Execute(\"config inspect\") failed: exitCode=%d stderr=%s", exitCode, stderr)
	}

	got := decodeJSON(t, "HTTP response", rec.Body.Bytes())
	want := decodeJSON(t, "direct Execute", direct)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("HTTP and direct Execute outcomes differ:\nHTTP:   %s\ndirect: %s", rec.Body.String(), direct)
	}
}

// TestEmptyCollection proves a read endpoint over an empty domain
// collection returns 200 with an empty JSON array, not 404 or null: an
// unopened show's "pools" field must serialize as "[]", never omitted or
// "null" (07-02-PLAN.md must_haves).
func TestEmptyCollection(t *testing.T) {
	root := t.TempDir()
	catalog, err := routecatalog.New()
	if err != nil {
		t.Fatalf("routecatalog.New: %v", err)
	}

	// A show path that has never been saved loads as a valid, empty
	// State (internal/show.Load's own "never-yet-saved" branch) -- no
	// fixture file needs to exist on disk for this to be a genuine empty
	// collection, not an error case.
	server := api.NewServer(catalog, root, "unopened-show.golc")

	rec := doGet(t, server.Handler(), "/v1/show")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/show: expected 200, got %d (body: %s)", rec.Code, rec.Body.String())
	}

	decoded := decodeJSON(t, "GET /v1/show", rec.Body.Bytes())
	obj, ok := decoded.(map[string]any)
	if !ok {
		t.Fatalf("expected a JSON object, got %T: %v", decoded, decoded)
	}
	pools, present := obj["pools"]
	if !present {
		t.Fatalf("expected a \"pools\" field, got: %s", rec.Body.String())
	}
	poolsArray, ok := pools.([]any)
	if !ok {
		t.Fatalf("expected \"pools\" to decode as a JSON array, got %T (%v) -- body: %s", pools, pools, rec.Body.String())
	}
	if len(poolsArray) != 0 {
		t.Fatalf("expected an empty \"pools\" array, got %d entries", len(poolsArray))
	}
}

// TestShowPathInjection proves translate.go never forwards a
// client-supplied show path: the fixed daemon showPath is always the
// --show value in the built command args, regardless of what a request
// sends (07-RESEARCH.md Pitfall 3, T-07-02) -- GET /v1/show has no path
// input field a client could use to override it, and an attempted
// override via an unrecognized query parameter is simply ignored.
func TestShowPathInjection(t *testing.T) {
	stub := &stubExecutor{exitCode: 0, stdout: []byte(`{"schema_version":1,"revision":0,"pools":[],"deployments":[]}` + "\n")}
	const fixedShowPath = "/daemon/fixed/show.golc"
	server := api.NewServer(stub, "/repo/root", fixedShowPath)

	rec := doGet(t, server.Handler(), "/v1/show?show=/etc/passwd&showPath=../../etc/shadow")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/show: expected 200, got %d (body: %s)", rec.Code, rec.Body.String())
	}

	wantArgs := []string{"--show", fixedShowPath}
	if !reflect.DeepEqual(stub.lastArgs, wantArgs) {
		t.Fatalf("expected the fixed show path %v regardless of query params, got %v", wantArgs, stub.lastArgs)
	}
	if stub.lastRoute != "show inspect" {
		t.Fatalf("expected route \"show inspect\", got %q", stub.lastRoute)
	}
}

// --- TestTranslateResult (exit-code mapping) ------------------------------

// TestTranslateResult proves translateResult's exit-code mapping through
// observable HTTP behavior: ExitCode 0 -> 2xx, ExitCode 2 -> HTTP 400,
// ExitCode 1 -> a typed Huma error (4xx/5xx) carrying the command's own
// Stderr diagnostic.
func TestTranslateResult(t *testing.T) {
	cases := []struct {
		name           string
		exitCode       int
		stdout         []byte
		stderr         []byte
		wantStatusMin  int
		wantStatusMax  int
		wantBodySubstr string
	}{
		{
			name:          "success",
			exitCode:      0,
			stdout:        []byte(`{"runtime":{"log_level":"info"}}` + "\n"),
			wantStatusMin: 200,
			wantStatusMax: 299,
		},
		{
			name:           "malformed-invocation",
			exitCode:       2,
			stderr:         []byte("GOLC_CONFIG_USAGE: exactly one concern id is required\n"),
			wantStatusMin:  400,
			wantStatusMax:  400,
			wantBodySubstr: "GOLC_CONFIG_USAGE",
		},
		{
			name:           "domain-failure",
			exitCode:       1,
			stderr:         []byte("GOLC_CONFIG_CONCERN_UNKNOWN: \"bogus\" is not in golc.project.toml\n"),
			wantStatusMin:  400,
			wantStatusMax:  599,
			wantBodySubstr: "GOLC_CONFIG_CONCERN_UNKNOWN",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stub := &stubExecutor{exitCode: tc.exitCode, stdout: tc.stdout, stderr: tc.stderr}
			server := api.NewServer(stub, "/repo/root", "/repo/root/show.golc")

			rec := doGet(t, server.Handler(), "/v1/config/runtime")
			if rec.Code < tc.wantStatusMin || rec.Code > tc.wantStatusMax {
				t.Fatalf("expected status in [%d,%d], got %d (body: %s)", tc.wantStatusMin, tc.wantStatusMax, rec.Code, rec.Body.String())
			}
			if tc.wantBodySubstr != "" && !strings.Contains(rec.Body.String(), tc.wantBodySubstr) {
				t.Fatalf("expected response body to contain %q, got: %s", tc.wantBodySubstr, rec.Body.String())
			}
		})
	}
}
