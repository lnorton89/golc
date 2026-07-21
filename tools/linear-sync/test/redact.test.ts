// redact.test.ts is the complete offline contract test for this
// workspace's canary-scan and safe-uncertain-outcome contract (CONTEXT
// D-19/D-20; T-01-41; Plan 01-27 must_haves.artifacts). It proves
// scanCanary/scanCanaryAll detect exactly the same forbidden-token surface
// internal/security/redact.go's ScanCanary/ScanCanaryAll already enforce
// Go-side, and that safeError never lets a raw, hostile create/update
// exception's message, stack, or any attached property reach a returned
// TransportDiagnostic -- entirely offline, against plain in-memory Error
// objects, never a real LinearClient, live credential, or network call.
// Runs alongside mutation.test.ts under the registered "linear-transport-
// node" scope (config/commands.toml, internal/command/linear_sync.go),
// whose "TestScopeLinearTransportNode" marker lives in mutation.test.ts.
import { test } from "node:test";
import assert from "node:assert/strict";

import { CANARY_TOKEN, safeError, scanCanary, scanCanaryAll } from "../src/redact.js";

test("scanCanary detects the exact CANARY_TOKEN and every forbidden secret-shaped substring", () => {
  assert.strictEqual(scanCanary("clean text with no secrets"), undefined, "clean text must scan clean");
  assert.strictEqual(scanCanary(`leaked ${CANARY_TOKEN} in output`), CANARY_TOKEN, "the exact canary token must be detected");
  assert.strictEqual(scanCanary("Authorization: LINEAR_API_KEY=abc123"), "LINEAR_API_KEY=", "a raw LINEAR_API_KEY declaration must be detected");
  assert.strictEqual(scanCanary("Authorization: Bearer abc123"), "Bearer ", "a raw bearer auth header must be detected");
  assert.strictEqual(scanCanary("token sk-abc123"), "sk-", "an API-key-shaped secret prefix must be detected");
  assert.strictEqual(scanCanary("token lin_api_abc123"), "lin_api_", "a raw Linear personal API key prefix must be detected");
});

test("scanCanaryAll scans every named source and reports violations sorted by source name", () => {
  const violations = scanCanaryAll({
    zzz_clean: "nothing sensitive here",
    aaa_leaked: `contains ${CANARY_TOKEN}`,
    mmm_also_leaked: "Bearer some-token",
  });
  assert.deepStrictEqual(
    violations,
    [
      { source: "aaa_leaked", token: CANARY_TOKEN },
      { source: "mmm_also_leaked", token: "Bearer " },
    ],
    "expected exactly the two leaking sources, sorted by source name, and the clean source excluded entirely",
  );
  assert.deepStrictEqual(scanCanaryAll({ clean: "safe" }), [], "an entirely clean source map must report zero violations");
});

test("safeError never leaks a raw exception's message, stack, or attached fields even when they embed the canary token and forbidden headers", () => {
  const hostileMessage = `request failed: LINEAR_API_KEY=${CANARY_TOKEN} Authorization: Bearer sk-should-never-appear`;
  const hostileError = new Error(hostileMessage) as Error & { headers?: unknown; requestBody?: unknown; client?: unknown };
  hostileError.headers = { authorization: `Bearer ${CANARY_TOKEN}` };
  hostileError.requestBody = { query: "mutation { createIssue }", variables: { apiKey: CANARY_TOKEN } };
  hostileError.client = { apiKey: CANARY_TOKEN };

  const diagnostic = safeError(hostileError, { operation: "task_subissue create", endpoint: "https://api.linear.app/graphql" });

  assert.strictEqual(diagnostic.operation, "task_subissue create");
  assert.strictEqual(diagnostic.endpoint, "https://api.linear.app/graphql");
  assert.strictEqual(diagnostic.code, "LINEAR_MUTATION_UNCERTAIN", "a generic Error must classify as the fixed unknown code");

  const allowlistedKeys = ["operation", "path", "code", "request", "endpoint", "complexity", "reset"];
  for (const key of Object.keys(diagnostic)) {
    assert.ok(allowlistedKeys.includes(key), `diagnostic field ${JSON.stringify(key)} is not on the allowlisted surface`);
  }

  const rendered = JSON.stringify(diagnostic);
  assert.strictEqual(scanCanary(rendered), undefined, "the rendered diagnostic must never contain the hostile error's leaked canary/header/credential content");
  assert.ok(!rendered.includes(hostileMessage), "the raw exception message must never appear in the diagnostic");
  assert.ok(!rendered.includes("headers"), "the raw exception's attached headers field name must never appear in the diagnostic");
  assert.ok(!rendered.includes("client"), "the raw exception's attached client field name must never appear in the diagnostic");
});

test("safeError classifies AbortError/TimeoutError as a timeout and a TypeError with a structured cause as a network error, both without reading any raw content", () => {
  const abortError = new Error("aborted");
  abortError.name = "AbortError";
  const timeoutDiagnostic = safeError(abortError, { operation: "task_subissue update" });
  assert.strictEqual(timeoutDiagnostic.code, "LINEAR_MUTATION_TIMEOUT");

  const networkError = new TypeError("fetch failed") as TypeError & { cause?: unknown };
  networkError.cause = { code: "ECONNRESET" };
  const networkDiagnostic = safeError(networkError, { operation: "task_subissue update" });
  assert.strictEqual(networkDiagnostic.code, "LINEAR_MUTATION_NETWORK_ERROR");

  const plainTypeError = new TypeError("no cause here");
  const plainDiagnostic = safeError(plainTypeError, { operation: "task_subissue update" });
  assert.strictEqual(plainDiagnostic.code, "LINEAR_MUTATION_UNCERTAIN", "a TypeError without a structured cause must fall back to the generic unknown code");

  const nonErrorThrow = safeError("a raw string throw, not even an Error instance", { operation: "task_subissue update" });
  assert.strictEqual(nonErrorThrow.code, "LINEAR_MUTATION_UNCERTAIN", "a non-Error thrown value must still classify safely, never crash normalization");
});
