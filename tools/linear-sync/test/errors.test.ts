// errors.test.ts is the complete offline contract test for HTTP-200
// data-plus-errors normalization and the blocked-snapshot integration path
// (CONTEXT D-21; T-01-40; Plan 01-26 must_haves.artifacts). It registers
// scope "linear-transport-errors" (config/commands.toml,
// internal/command/linear_sync.go) through the exact marker
// "TestScopeLinearTransportErrors" this file defines below, and proves
// normalizeGraphQLResult (errors.ts) and captureSnapshot's
// probeGraphQLResult integration (adapter.ts) against the committed
// fixture tests/fixtures/linear/data-plus-errors.json -- entirely offline,
// against a fixture-driven fake response, never a real LinearClient, live
// credential, or network call.
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";

import { normalizeGraphQLResult, type RawGraphQLResponse } from "../src/errors.js";
import { captureSnapshot, type ConnectionQuery, type LinearEntityHandle } from "../src/adapter.js";
import type { EntityKind } from "../src/protocol.js";

// ---------------------------------------------------------------------------
// Fixture shape and loader
// ---------------------------------------------------------------------------

interface DataPlusErrorsFixture {
  schemaVersion: number;
  description: string;
  operation: string;
  endpoint: string;
  response: RawGraphQLResponse;
  expectedDiagnosticCount: number;
  expectedDiagnosticPath: string;
  expectedDiagnosticCode: string;
  expectedDiagnosticComplexity: number;
  cleanResponse: RawGraphQLResponse;
}

// loadFixture reads the committed golden fixture from disk. process.cwd()
// is exactly repository-root/tools/linear-sync when this scope runs
// (internal/command/test.go's runNodeScopeTest sets the child process
// working directory to the registered scope's Dir), so two levels up
// reaches the repository root (matching pagination.test.ts's precedent).
function loadFixture(): DataPlusErrorsFixture {
  const fixturePath = process.cwd() + "/../../tests/fixtures/linear/data-plus-errors.json";
  return JSON.parse(readFileSync(fixturePath, "utf8")) as DataPlusErrorsFixture;
}

function emptyPage(): { nodes: LinearEntityHandle[]; pageInfo: { hasNextPage: boolean; endCursor: string | null } } {
  return { nodes: [], pageInfo: { hasNextPage: false, endCursor: null } };
}

// ---------------------------------------------------------------------------
// TestScopeLinearTransportErrors -- the exact marker config/commands.toml
// and internal/command/linear_sync.go's MustDeclareNodeScope registration
// name for the "linear-transport-errors" scope.
// ---------------------------------------------------------------------------

test("TestScopeLinearTransportErrors", async (t) => {
  await t.test("normalizeGraphQLResult blocks a data-plus-errors response as partial even though data is populated", () => {
    const fixture = loadFixture();
    assert.strictEqual(fixture.schemaVersion, 1, "expected data-plus-errors.json schemaVersion 1");
    assert.ok(fixture.response.data !== undefined, "fixture response must carry a populated data object");
    assert.ok(Array.isArray(fixture.response.errors) && fixture.response.errors.length > 0, "fixture response must carry a non-empty errors array");

    const outcome = normalizeGraphQLResult(fixture.response, { operation: fixture.operation, endpoint: fixture.endpoint });
    assert.strictEqual(outcome.kind, "partial", "a non-empty errors array must block the read even alongside populated data");
    if (outcome.kind !== "partial") {
      return;
    }
    assert.strictEqual(outcome.diagnostics.length, fixture.expectedDiagnosticCount, "expected exactly one normalized diagnostic per raw error");

    const diagnostic = outcome.diagnostics[0];
    assert.ok(diagnostic, "expected a diagnostic entry");
    assert.strictEqual(diagnostic?.operation, fixture.operation);
    assert.strictEqual(diagnostic?.path, fixture.expectedDiagnosticPath);
    assert.strictEqual(diagnostic?.code, fixture.expectedDiagnosticCode);
    assert.strictEqual(diagnostic?.endpoint, fixture.endpoint);
    assert.strictEqual(diagnostic?.complexity, fixture.expectedDiagnosticComplexity);

    const allowlistedKeys = ["operation", "path", "code", "request", "endpoint", "complexity", "reset"];
    for (const key of Object.keys(diagnostic ?? {})) {
      assert.ok(allowlistedKeys.includes(key), `diagnostic field ${JSON.stringify(key)} is not on the allowlisted path/code/operation/request/endpoint/complexity/reset surface`);
    }
    assert.strictEqual((diagnostic as unknown as Record<string, unknown>)?.["message"], undefined, "raw error message must never leak into a normalized diagnostic");
  });

  await t.test("normalizeGraphQLResult reports clean when errors is absent", () => {
    const fixture = loadFixture();
    const outcome = normalizeGraphQLResult(fixture.cleanResponse, { operation: fixture.operation, endpoint: fixture.endpoint });
    assert.strictEqual(outcome.kind, "clean", "a response with no errors array must normalize as clean");
  });

  await t.test("captureSnapshot blocks the entire snapshot with status partial when any connection's probe observes data-plus-errors", async () => {
    const fixture = loadFixture();

    const goodConnection: ConnectionQuery = {
      entity: "requirement_issue" as EntityKind,
      label: "good connection",
      fetchPage: async () => emptyPage(),
    };
    const partialConnection: ConnectionQuery = {
      entity: "task_subissue" as EntityKind,
      label: fixture.operation,
      fetchPage: async () => emptyPage(),
      probeGraphQLResult: async () => fixture.response,
    };

    const snapshot = await captureSnapshot([goodConnection, partialConnection]);
    assert.strictEqual(snapshot.status, "partial", "expected one connection's data-plus-errors probe to block the whole snapshot");
    assert.strictEqual(
      snapshot.records.length,
      0,
      "an incomplete snapshot must never expose partial records (even from an already-completed connection) for an identity/create/preview decision",
    );
    assert.ok(snapshot.reason?.includes(fixture.operation), "expected the blocking connection's operation label in the snapshot reason");
    assert.ok(snapshot.reason?.includes(fixture.expectedDiagnosticCode), "expected the safe diagnostic code in the snapshot reason");
    assert.ok(!snapshot.reason?.toLowerCase().includes("could not be resolved"), "raw GraphQL error message text must never appear in the snapshot reason");
  });

  await t.test("captureSnapshot remains complete when every connection's probe reports clean", async () => {
    const fixture = loadFixture();
    const cleanConnection: ConnectionQuery = {
      entity: "requirement_issue" as EntityKind,
      label: "clean connection",
      fetchPage: async () => emptyPage(),
      probeGraphQLResult: async () => fixture.cleanResponse,
    };

    const snapshot = await captureSnapshot([cleanConnection]);
    assert.strictEqual(snapshot.status, "complete", "a clean probe must never block the snapshot");
  });
});
