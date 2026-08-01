// translate_test.go exercises translate.go's HTTP<->routed-command
// translation end-to-end through a *api.Server's real Chi/Huma handler
// (httptest.NewRecorder against server.Handler(), no real network
// listener needed for these in-process checks). It lives in the external
// api_test package (see coverage_test.go's doc comment for why) so
// TestParity can reach a real command registry through
// internal/routecatalog's test-only bridge.
//
// Every request now requires a valid API key (07-04-PLAN.md Task 2, D-05:
// AuthMiddleware applies to every /v1 request) -- doGet always seeds and
// presents one via seedAPIKey (auth_test.go, same package), against a
// real root/showPath every server in this file is now constructed with.
// Root/showPath values that used to be arbitrary placeholder strings
// (e.g. "/repo/root") were switched to real t.TempDir() locations for the
// same reason: AuthMiddleware's key lookup opens a real .golc SQLite
// store at that path (internal/show.LookupAPIKeyByPrefix), which a
// non-existent placeholder path cannot satisfy.
package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/api"
	"github.com/lnorton89/golc/internal/routecatalog"
	"github.com/lnorton89/golc/internal/show"
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

// doGet issues a GET against handler in-process (httptest.NewRecorder),
// presenting token as a bearer credential (AuthMiddleware now applies to
// every /v1 request), and returns the recorded response.
func doGet(t *testing.T, handler http.Handler, target, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.Header.Set("Authorization", "Bearer "+token)
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
	err := os.MkdirAll(filepath.Dir(target), 0o755)
	require.NoError(t, err, "MkdirAll(%q)", relative)
	err = os.WriteFile(target, []byte(content), 0o644)
	require.NoError(t, err, "WriteFile(%q)", relative)
}

// decodeJSON unmarshals body into a generic value for structural
// (not byte-literal) comparison -- robust to incidental whitespace
// differences between the two encoding paths under test.
func decodeJSON(t *testing.T, label string, body []byte) any {
	t.Helper()
	var value any
	err := json.Unmarshal(body, &value)
	require.NoError(t, err, "%s: json.Unmarshal(%q)", label, body)
	return value
}

// TestParity proves GET /v1/config/{concern} returns the same JSON
// outcome "config inspect <concern>" produces via a direct Execute call
// on the same root -- HTTP -> command.Execute -> response parity
// (API-01, this plan's guaranteed-route parity gate).
func TestParity(t *testing.T) {
	root := newParityRepository(t)
	catalog, err := routecatalog.New()
	require.NoError(t, err, "routecatalog.New")

	showPath := filepath.Join(root, "show.golc")
	server := api.NewServer(catalog, root, showPath)
	token := seedAPIKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopePlayback}, time.Hour)

	rec := doGet(t, server.Handler(), "/v1/config/runtime", token)
	require.Equal(t, http.StatusOK, rec.Code, "GET /v1/config/runtime (body: %s)", rec.Body.String())

	exitCode, direct, stderr := catalog.Execute("config inspect", []string{"runtime"}, root)
	require.Equal(t, 0, exitCode, "direct Execute(\"config inspect\") failed: stderr=%s", stderr)

	got := decodeJSON(t, "HTTP response", rec.Body.Bytes())
	want := decodeJSON(t, "direct Execute", direct)
	require.Equal(t, want, got, "HTTP and direct Execute outcomes differ")
}

// TestEmptyCollection proves a read endpoint over an empty domain
// collection returns 200 with an empty JSON array, not 404 or null: an
// unopened show's "pools" field must serialize as "[]", never omitted or
// "null" (07-02-PLAN.md must_haves).
func TestEmptyCollection(t *testing.T) {
	root := t.TempDir()
	catalog, err := routecatalog.New()
	require.NoError(t, err, "routecatalog.New")

	// A show path that has never been saved loads as a valid, empty
	// State (internal/show.Load's own "never-yet-saved" branch) -- no
	// fixture file needs to exist on disk for this to be a genuine empty
	// collection, not an error case.
	const showPath = "unopened-show.golc"
	server := api.NewServer(catalog, root, showPath)
	token := seedAPIKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopePlayback}, time.Hour)

	rec := doGet(t, server.Handler(), "/v1/show", token)
	require.Equal(t, http.StatusOK, rec.Code, "GET /v1/show (body: %s)", rec.Body.String())

	decoded := decodeJSON(t, "GET /v1/show", rec.Body.Bytes())
	obj, ok := decoded.(map[string]any)
	require.True(t, ok, "expected a JSON object, got %T: %v", decoded, decoded)
	pools, present := obj["pools"]
	require.True(t, present, "expected a \"pools\" field, got: %s", rec.Body.String())
	poolsArray, ok := pools.([]any)
	require.True(t, ok, "expected \"pools\" to decode as a JSON array, got %T (%v) -- body: %s", pools, pools, rec.Body.String())
	require.Len(t, poolsArray, 0, "expected an empty \"pools\" array")
}

// TestShowPathInjection proves translate.go never forwards a
// client-supplied show path: the fixed daemon showPath is always the
// --show value in the built command args, regardless of what a request
// sends (07-RESEARCH.md Pitfall 3, T-07-02) -- GET /v1/show has no path
// input field a client could use to override it, and an attempted
// override via an unrecognized query parameter is simply ignored.
func TestShowPathInjection(t *testing.T) {
	stub := &stubExecutor{exitCode: 0, stdout: []byte(`{"schema_version":1,"revision":0,"pools":[],"deployments":[]}` + "\n")}
	root := t.TempDir()
	fixedShowPath := filepath.Join(root, "daemon-fixed-show.golc")
	server := api.NewServer(stub, root, fixedShowPath)
	token := seedAPIKey(t, root, fixedShowPath, []show.APIKeyScope{show.APIKeyScopePlayback}, time.Hour)

	rec := doGet(t, server.Handler(), "/v1/show?show=/etc/passwd&showPath=../../etc/shadow", token)
	require.Equal(t, http.StatusOK, rec.Code, "GET /v1/show (body: %s)", rec.Body.String())

	wantArgs := []string{"--show", fixedShowPath}
	require.Equal(t, wantArgs, stub.lastArgs, "expected the fixed show path regardless of query params")
	require.Equal(t, "show inspect", stub.lastRoute)
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

	root := t.TempDir()
	showPath := filepath.Join(root, "show.golc")
	token := seedAPIKey(t, root, showPath, []show.APIKeyScope{show.APIKeyScopePlayback}, time.Hour)

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stub := &stubExecutor{exitCode: tc.exitCode, stdout: tc.stdout, stderr: tc.stderr}
			server := api.NewServer(stub, root, showPath)

			rec := doGet(t, server.Handler(), "/v1/config/runtime", token)
			require.True(t, rec.Code >= tc.wantStatusMin && rec.Code <= tc.wantStatusMax, "expected status in [%d,%d], got %d (body: %s)", tc.wantStatusMin, tc.wantStatusMax, rec.Code, rec.Body.String())
			if tc.wantBodySubstr != "" {
				require.Contains(t, rec.Body.String(), tc.wantBodySubstr)
			}
		})
	}
}
