// rate-limit.test.ts is the complete offline contract test for rate-limit
// normalization and the retry policy (CONTEXT D-21; T-01-40; Plan 01-26
// must_haves.artifacts). It proves normalizeRateLimit's allowlist,
// decideRetry's bounded-reads/5xx-only-retry plus typed stop-write
// behavior for a rate-limited or partial mutation, and captureSnapshot's
// probeRateLimit integration -- all against the committed fixture
// tests/fixtures/linear/rate-limited.json, entirely offline, never a real
// LinearClient, live credential, or network call.
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";

import {
  decideRetry,
  normalizeRateLimit,
  type FailureKind,
  type RawRateLimitSignal,
} from "../src/errors.js";
import { captureSnapshot, type ConnectionQuery, type LinearEntityHandle } from "../src/adapter.js";
import type { EntityKind, OperationAction } from "../src/protocol.js";

// ---------------------------------------------------------------------------
// Fixture shape and loader
// ---------------------------------------------------------------------------

interface RetryScenario {
  name: string;
  action: OperationAction;
  kind: FailureKind;
  attempt: number;
  maxAttempts: number;
  expectedOutcome: "retry" | "stop_write" | "stop";
  expectedNextAttempt?: number;
}

interface RateLimitedFixture {
  schemaVersion: number;
  description: string;
  operation: string;
  rateLimitSignal: RawRateLimitSignal;
  expectedDiagnosticRequest: string;
  expectedDiagnosticEndpoint: string;
  expectedDiagnosticComplexity: number;
  expectedDiagnosticReset: string;
  retryScenarios: RetryScenario[];
}

// loadFixture reads the committed golden fixture from disk. process.cwd()
// is exactly repository-root/tools/linear-sync when this scope runs
// (internal/command/test.go's runNodeScopeTest sets the child process
// working directory to the registered scope's Dir), so two levels up
// reaches the repository root (matching pagination.test.ts's precedent).
function loadFixture(): RateLimitedFixture {
  const fixturePath = process.cwd() + "/../../tests/fixtures/linear/rate-limited.json";
  return JSON.parse(readFileSync(fixturePath, "utf8")) as RateLimitedFixture;
}

function emptyPage(): { nodes: LinearEntityHandle[]; pageInfo: { hasNextPage: boolean; endCursor: string | null } } {
  return { nodes: [], pageInfo: { hasNextPage: false, endCursor: null } };
}

test("linear-transport-errors rate-limit and retry policy contract", async (t) => {
  await t.test("normalizeRateLimit exposes only the allowlisted request/endpoint/complexity/reset fields", () => {
    const fixture = loadFixture();
    const diagnostic = normalizeRateLimit(fixture.rateLimitSignal, { operation: fixture.operation });

    assert.strictEqual(diagnostic.operation, fixture.operation);
    assert.strictEqual(diagnostic.request, fixture.expectedDiagnosticRequest);
    assert.strictEqual(diagnostic.endpoint, fixture.expectedDiagnosticEndpoint);
    assert.strictEqual(diagnostic.complexity, fixture.expectedDiagnosticComplexity);
    assert.strictEqual(diagnostic.reset, fixture.expectedDiagnosticReset);

    const allowlistedKeys = ["operation", "path", "code", "request", "endpoint", "complexity", "reset"];
    for (const key of Object.keys(diagnostic)) {
      assert.ok(allowlistedKeys.includes(key), `diagnostic field ${JSON.stringify(key)} is not on the allowlisted surface`);
    }
    assert.strictEqual(
      (diagnostic as unknown as Record<string, unknown>)["authorization"],
      undefined,
      "the raw signal's authorization field must never leak into a normalized diagnostic",
    );
  });

  await t.test("decideRetry matches every fixture retry scenario", async (scenarioContext) => {
    const fixture = loadFixture();
    assert.ok(fixture.retryScenarios.length >= 6, "expected a comprehensive set of retry scenarios");

    const diagnostic = normalizeRateLimit(fixture.rateLimitSignal, { operation: fixture.operation });

    for (const scenario of fixture.retryScenarios) {
      // Each nested subtest must be issued against this callback's own
      // TestContext (scenarioContext), never the outer t: awaiting
      // t.test(...) from inside a callback that is itself an in-flight
      // t.test(...) call on the same TestContext deadlocks Node's test
      // runner (pagination.test.ts documents this same node:test
      // reentrancy hazard).
      await scenarioContext.test(scenario.name, () => {
        const decision = decideRetry({
          action: scenario.action,
          kind: scenario.kind,
          attempt: scenario.attempt,
          maxAttempts: scenario.maxAttempts,
          diagnostic,
        });

        assert.strictEqual(decision.outcome, scenario.expectedOutcome, `scenario ${scenario.name} expected outcome ${scenario.expectedOutcome}`);
        if (decision.outcome === "retry" && scenario.expectedNextAttempt !== undefined) {
          assert.strictEqual(decision.nextAttempt, scenario.expectedNextAttempt);
        }
        if (decision.outcome === "stop_write") {
          assert.ok(scenario.action !== "read", "stop_write must never be returned for a read action");
          assert.ok(scenario.kind === "partial" || scenario.kind === "rate_limited", "stop_write must only follow a partial or rate_limited diagnosis");
        }
      });
    }
  });

  await t.test("captureSnapshot blocks the entire snapshot with status rate_limited when any connection's probe observes a rate-limit signal", async () => {
    const fixture = loadFixture();

    const goodConnection: ConnectionQuery = {
      entity: "requirement_issue" as EntityKind,
      label: "good connection",
      fetchPage: async () => emptyPage(),
    };
    const rateLimitedConnection: ConnectionQuery = {
      entity: "task_subissue" as EntityKind,
      label: fixture.operation,
      fetchPage: async () => emptyPage(),
      probeRateLimit: async () => fixture.rateLimitSignal,
    };

    const snapshot = await captureSnapshot([goodConnection, rateLimitedConnection]);
    assert.strictEqual(snapshot.status, "rate_limited", "expected one connection's rate-limit probe to block the whole snapshot");
    assert.strictEqual(
      snapshot.records.length,
      0,
      "an incomplete snapshot must never expose partial records (even from an already-completed connection) for an identity/create/preview decision",
    );
    assert.ok(snapshot.reason?.includes(fixture.operation), "expected the blocking connection's operation label in the snapshot reason");
    assert.ok(snapshot.reason?.includes(fixture.expectedDiagnosticRequest), "expected the safe request id in the snapshot reason");
  });

  await t.test("captureSnapshot remains complete when a connection's probeRateLimit reports no signal", async () => {
    const cleanConnection: ConnectionQuery = {
      entity: "requirement_issue" as EntityKind,
      label: "clean connection",
      fetchPage: async () => emptyPage(),
      probeRateLimit: async () => undefined,
    };

    const snapshot = await captureSnapshot([cleanConnection]);
    assert.strictEqual(snapshot.status, "complete", "an undefined rate-limit probe result must never block the snapshot");
  });
});
