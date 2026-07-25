// translate.go implements 07-RESEARCH.md Pattern 1 (HTTP -> routed
// command translation): every operation registered here parses its typed
// Huma input, builds the exact argv-shaped arguments the daemon-side
// command registry expects, calls the injected Executor, and turns the
// outcome back into an HTTP response via translateResult. This plan ships
// the first two read routes; later plans add auth, mutations, dry-run,
// batch, and SSE through the same RegisterOperation seam (router.go).
package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
)

// translateResult maps one routed command outcome onto Huma's typed
// response/error contract, mirroring the execution package's own
// Result-to-exit-code convention (07-PATTERNS.md "Error handling /
// exit-code convention"): ExitCode 0 is success, whose Stdout becomes the
// raw response body; ExitCode 2 (malformed/unroutable invocation) becomes
// an HTTP 400; any other exit code (a domain-level handler failure)
// becomes a typed 5xx Huma error carrying the command's own Stderr
// diagnostic verbatim, never a bare Go error that would swallow it.
func translateResult(exitCode int, stdout, stderr []byte) ([]byte, error) {
	switch exitCode {
	case 0:
		return stdout, nil
	case 2:
		return nil, huma.Error400BadRequest(diagnosticMessage(stderr))
	default:
		return nil, huma.Error500InternalServerError(diagnosticMessage(stderr))
	}
}

// diagnosticMessage trims a routed command's raw Stderr into the message
// text a Huma typed error carries, falling back to a stable diagnostic of
// its own when a failing handler left Stderr empty (never an empty error
// message).
func diagnosticMessage(stderr []byte) string {
	message := strings.TrimSpace(string(stderr))
	if message == "" {
		message = "GOLC_API_UNKNOWN_FAILURE: the routed command failed with no diagnostic"
	}
	return message
}

// buildShowArgs appends the daemon's own fixed show path server-side
// (07-RESEARCH.md Pitfall 3, T-07-02): no input struct in this package
// ever declares a client-suppliable path field, so there is no argument
// position a request could use to override it.
func buildShowArgs(showPath string) []string {
	return []string{"--show", showPath}
}

// rawJSONOutput is the shared Huma output shape for every operation in
// this file: Body is written to the response exactly as given (Huma's
// raw-passthrough convention for a `Body []byte` field, never
// re-encoded), so the routed command's own deterministic JSON reaches the
// client byte-for-byte -- the exact property the HTTP<->CLI parity gate
// (coverage_test.go's TestParity) checks.
type rawJSONOutput struct {
	ContentType string `header:"Content-Type"`
	Body        []byte
}

// newRawJSONOutput wraps body as an application/json rawJSONOutput.
func newRawJSONOutput(body []byte) *rawJSONOutput {
	return &rawJSONOutput{ContentType: "application/json", Body: body}
}

// --- GET /v1/config/{concern} -> "config inspect" ----------------------

// configInspectInput is GET /v1/config/{concern}'s Huma input: concern is
// the configuration concern id (e.g. "runtime"), the only argument
// "config inspect" accepts.
type configInspectInput struct {
	Concern string `path:"concern" doc:"Configuration concern id, e.g. \"runtime\"."`
}

// registerConfigInspect wires GET /v1/config/{concern} onto humaAPI,
// translating it into a "config inspect <concern>" invocation.
func registerConfigInspect(humaAPI huma.API, server *Server) {
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-config-concern",
		Method:      http.MethodGet,
		Path:        apiPathPrefix + "/config/{concern}",
		Summary:     "Inspect one indexed configuration concern as JSON.",
	}, func(ctx context.Context, input *configInspectInput) (*rawJSONOutput, error) {
		exitCode, stdout, stderr := server.executor.Execute("config inspect", []string{input.Concern}, server.root)
		body, err := translateResult(exitCode, stdout, stderr)
		if err != nil {
			return nil, err
		}
		return newRawJSONOutput(body), nil
	})
}

var _ = RegisterOperation(OperationRegistration{Route: "config inspect", Register: registerConfigInspect})

// --- GET /v1/show -> "show inspect" -------------------------------------

// registerShowInspect wires GET /v1/show onto humaAPI, translating it
// into a "show inspect --show <daemon's own fixed show path>" invocation.
// An empty show's "pools"/"deployments" arrays serialize as JSON "[]",
// never "null" (the underlying command handler's own allowlisted view
// always builds non-nil slices).
func registerShowInspect(humaAPI huma.API, server *Server) {
	huma.Register(humaAPI, huma.Operation{
		OperationID: "get-show",
		Method:      http.MethodGet,
		Path:        apiPathPrefix + "/show",
		Summary:     "Inspect the daemon's own running show document's pools and deployments.",
	}, func(ctx context.Context, input *struct{}) (*rawJSONOutput, error) {
		args := buildShowArgs(server.showPath)
		exitCode, stdout, stderr := server.executor.Execute("show inspect", args, server.root)
		body, err := translateResult(exitCode, stdout, stderr)
		if err != nil {
			return nil, err
		}
		return newRawJSONOutput(body), nil
	})
}

var _ = RegisterOperation(OperationRegistration{Route: "show inspect", Register: registerShowInspect})
