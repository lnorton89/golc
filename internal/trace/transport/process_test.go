// process_test.go proves ProcessClient's generic transport-boundary
// contract (CONTEXT D-21; T-01-42/T-01-44) against a small, dependency-free
// Node test double fixture — not the real tools/linear-sync adapter, which
// tests/acceptance/linear-transport.ps1 exercises separately end to end,
// including the fake-official-SDK hierarchy preview/apply/replay. This
// file only proves the launch/protocol/deadline/redaction mechanics every
// concrete adapter — fake or real — relies on: executable-path enforcement,
// strict one-line request/response framing, protocol-noise rejection,
// context cancellation, deadline timeout with process-tree termination,
// missing-adapter failure, and stderr redaction.
package transport

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// resolveTestNode locates a Node executable to drive these tests: the
// pinned project-local install if bootstrap has already provisioned it
// (preferred, matching production resolution exactly), otherwise a host
// PATH "node". If neither is available the test skips rather than
// failing, since this package must remain buildable and testable before
// any Node toolchain is ever provisioned (CONTEXT D-21: a missing adapter
// affects only the explicit remote command, never core Go test/build).
func resolveTestNode(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		matches, _ := filepath.Glob(filepath.Join("..", "..", "..", ".tools", "toolchains", "node", "*", "windows-amd64", "node-v*-win-x64", "node.exe"))
		if len(matches) > 0 {
			absolute, err := filepath.Abs(matches[0])
			if err == nil {
				return absolute
			}
			return matches[0]
		}
	}
	if path, err := exec.LookPath("node"); err == nil {
		return path
	}
	t.Skip("no Node executable available (pinned or PATH); run 'golc.ps1 bootstrap --include linear-sync' first")
	return ""
}

// writeFixture writes the small CommonJS test-double script this file's
// tests drive, returning its absolute path. The fixture reads one
// NDJSON-ish line at a time and behaves according to the request's "mode"
// field:
//   - "echo": writes back {"echo": <fields>} once, immediately.
//   - "noise": writes a non-JSON line instead of any valid response.
//   - "sleep": sleeps for the requested "ms" before responding, so a
//     caller-side deadline shorter than that sleep proves timeout handling.
//   - "stderr-canary": writes a canary-laden line to stderr, then exits 1
//     without ever writing a stdout response line, proving a failure
//     diagnostic never leaks that raw content.
//   - "sentinel": after "ms" milliseconds, writes sentinelPath so a test
//     can assert the process tree was actually killed before that file
//     ever appears (proving Call's timeout path truly terminates the
//     child rather than merely giving up waiting on it).
func writeFixture(t *testing.T, sentinelPath string) string {
	t.Helper()
	script := `
const readline = require("node:readline");
const fs = require("node:fs");

const rl = readline.createInterface({ input: process.stdin, terminal: false });

rl.on("line", (raw) => {
  let request;
  try {
    request = JSON.parse(raw);
  } catch (error) {
    process.stdout.write("not-json-noise\n");
    return;
  }

  if (request.mode === "noise") {
    process.stdout.write("this is not a JSON object\n");
    return;
  }

  if (request.mode === "sleep") {
    setTimeout(() => {
      process.stdout.write(JSON.stringify({ echo: request }) + "\n");
    }, request.ms || 0);
    return;
  }

  if (request.mode === "sentinel") {
    setTimeout(() => {
      fs.writeFileSync(process.env.GOLC_TEST_SENTINEL_PATH, "reached\n");
      process.stdout.write(JSON.stringify({ echo: request }) + "\n");
    }, request.ms || 0);
    return;
  }

  if (request.mode === "stderr-canary") {
    process.stderr.write("GOLC_FAKE_SECRET_CANARY_4f9c2e6b1a7d3f809c21 leaked-looking text\n");
    process.exitCode = 1;
    process.exit(1);
  }

  process.stdout.write(JSON.stringify({ echo: request }) + "\n");
});
`
	dir := t.TempDir()
	path := filepath.Join(dir, "fixture.js")
	if err := os.WriteFile(path, []byte(script), 0o644); err != nil {
		t.Fatalf("writing fixture script: %v", err)
	}
	return path
}

func newTestClient(t *testing.T, sentinelPath string, timeout time.Duration) *ProcessClient {
	t.Helper()
	node := resolveTestNode(t)
	script := writeFixture(t, sentinelPath)
	env := append(os.Environ(), "GOLC_TEST_SENTINEL_PATH="+sentinelPath)
	client, err := NewProcessClient(ProcessConfig{
		NodeExecutable: node,
		ScriptPath:     script,
		WorkDir:        filepath.Dir(script),
		Env:            env,
		Timeout:        timeout,
	})
	if err != nil {
		t.Fatalf("NewProcessClient: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestScopeTraceTransportProcess(t *testing.T) {
	t.Run("Call round-trips one strict JSON line", func(t *testing.T) {
		client := newTestClient(t, filepath.Join(t.TempDir(), "unused"), 5*time.Second)
		response, err := client.Call(context.Background(), []byte(`{"mode":"echo","value":"hello"}`))
		if err != nil {
			t.Fatalf("Call: %v", err)
		}
		var decoded struct {
			Echo struct {
				Value string `json:"value"`
			} `json:"echo"`
		}
		if err := json.Unmarshal(response, &decoded); err != nil {
			t.Fatalf("decoding response %s: %v", response, err)
		}
		if decoded.Echo.Value != "hello" {
			t.Fatalf("expected echoed value %q, got %q", "hello", decoded.Echo.Value)
		}
	})

	t.Run("Call rejects a request containing a newline", func(t *testing.T) {
		client := newTestClient(t, filepath.Join(t.TempDir(), "unused"), 5*time.Second)
		_, err := client.Call(context.Background(), []byte("{\"mode\":\"echo\"}\n"))
		requireRPCCode(t, err, "GOLC_TRANSPORT_REQUEST_INVALID")
	})

	t.Run("Call rejects protocol noise instead of a strict JSON object line", func(t *testing.T) {
		client := newTestClient(t, filepath.Join(t.TempDir(), "unused"), 5*time.Second)
		_, err := client.Call(context.Background(), []byte(`{"mode":"noise"}`))
		requireRPCCode(t, err, "GOLC_TRANSPORT_PROTOCOL_NOISE")
	})

	t.Run("Call enforces a caller deadline and kills the process tree", func(t *testing.T) {
		sentinel := filepath.Join(t.TempDir(), "sentinel.txt")
		client := newTestClient(t, sentinel, 0)
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		_, err := client.Call(ctx, []byte(`{"mode":"sentinel","ms":5000}`))
		requireRPCCode(t, err, "GOLC_TRANSPORT_TIMEOUT")

		// The fixture would write the sentinel file 5s after receiving the
		// request; the process tree must already be dead well before then.
		time.Sleep(300 * time.Millisecond)
		if _, statErr := os.Stat(sentinel); statErr == nil {
			t.Fatalf("sentinel file exists: timed-out process was not actually killed before completing its work")
		}

		// The client is permanently closed after a timeout: a further Call
		// must fail fast rather than silently reusing a dead process.
		_, err = client.Call(context.Background(), []byte(`{"mode":"echo"}`))
		requireRPCCode(t, err, "GOLC_TRANSPORT_CLOSED")
	})

	t.Run("Call honors explicit cancellation", func(t *testing.T) {
		client := newTestClient(t, filepath.Join(t.TempDir(), "unused"), 5*time.Second)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := client.Call(ctx, []byte(`{"mode":"echo"}`))
		requireRPCCode(t, err, "GOLC_TRANSPORT_CANCELED")
	})

	t.Run("Call never leaks a raw canary-laden stderr line into its error", func(t *testing.T) {
		client := newTestClient(t, filepath.Join(t.TempDir(), "unused"), 5*time.Second)
		_, err := client.Call(context.Background(), []byte(`{"mode":"stderr-canary"}`))
		if err == nil {
			t.Fatalf("expected a failure calling a process that exits without a response")
		}
		if strings.Contains(err.Error(), "GOLC_FAKE_SECRET_CANARY_4f9c2e6b1a7d3f809c21") {
			t.Fatalf("error leaked the raw canary token: %v", err)
		}
		if !strings.Contains(err.Error(), "<redacted>") {
			t.Fatalf("expected the redacted marker in the error, got: %v", err)
		}
	})

	t.Run("NewProcessClient fails closed on a missing node executable", func(t *testing.T) {
		_, err := NewProcessClient(ProcessConfig{
			NodeExecutable: filepath.Join(t.TempDir(), "does-not-exist-node.exe"),
			ScriptPath:     filepath.Join(t.TempDir(), "does-not-exist-cli.js"),
			WorkDir:        t.TempDir(),
		})
		requireRPCCode(t, err, "GOLC_TRANSPORT_ADAPTER_MISSING")
	})

	t.Run("NewProcessClient fails closed on a missing compiled adapter script", func(t *testing.T) {
		node := resolveTestNode(t)
		_, err := NewProcessClient(ProcessConfig{
			NodeExecutable: node,
			ScriptPath:     filepath.Join(t.TempDir(), "does-not-exist-cli.js"),
			WorkDir:        t.TempDir(),
		})
		requireRPCCode(t, err, "GOLC_TRANSPORT_ADAPTER_MISSING")
	})

	t.Run("Close is idempotent and safe to call more than once", func(t *testing.T) {
		client := newTestClient(t, filepath.Join(t.TempDir(), "unused"), 5*time.Second)
		if err := client.Close(); err != nil {
			t.Fatalf("first Close: %v", err)
		}
		if err := client.Close(); err != nil {
			t.Fatalf("second Close: %v", err)
		}
	})
}

func requireRPCCode(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected an error with code %s, got nil", code)
	}
	rpcErr, ok := err.(*RPCError)
	if !ok {
		t.Fatalf("expected *RPCError, got %T: %v", err, err)
	}
	if rpcErr.Code != code {
		t.Fatalf("expected code %s, got %s (%v)", code, rpcErr.Code, err)
	}
}
