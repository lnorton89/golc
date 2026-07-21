// pagination.test.ts is the complete offline contract test for exhaustive
// Relay connection pagination (CONTEXT D-21; T-01-39; Plan 01-14
// must_haves.artifacts). It registers scope "linear-transport-pagination"
// (config/commands.toml, internal/command/linear_sync.go) through the
// exact marker "TestScopeLinearTransportPagination" this file defines
// below, and proves fetchAllPages (pagination.ts) and captureSnapshot
// (adapter.ts) against two committed fixtures --
// tests/fixtures/linear/paginated-51.json (51 objects across two pages,
// with a page-two-only identity marker and a distinct page-one marker) and
// tests/fixtures/linear/cursor-loop.json (repeated/null/indirectly-looped
// cursor anomalies) -- entirely offline, against fixture-driven fake page
// fetchers, never a real LinearClient, live credential, or network call.
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";

import { fetchAllPages, type FetchPage, type PageInfo } from "../src/pagination.js";
import { captureSnapshot, type ConnectionQuery, type LinearEntityHandle } from "../src/adapter.js";
import type { EntityKind } from "../src/protocol.js";

// ---------------------------------------------------------------------------
// Fixture shapes and loader
// ---------------------------------------------------------------------------

interface FixtureNode {
  id: string;
  name?: string;
  title?: string;
  description?: string;
  updatedAt: string;
}

interface FixturePage {
  requestCursor: string | null;
  nodes: FixtureNode[];
  pageInfo: PageInfo;
}

interface PaginatedFixture {
  schemaVersion: number;
  description: string;
  pageSize: number;
  totalNodes: number;
  pages: FixturePage[];
  expectedFooterNodeId: string;
  expectedFooterSubstring: string;
  secondMarkerNodeId: string;
  secondMarkerSubstring: string;
}

interface CursorScenario {
  name: string;
  pages: FixturePage[];
  expectedCode: string;
}

interface CursorLoopFixture {
  schemaVersion: number;
  description: string;
  scenarios: CursorScenario[];
}

// loadJsonFixture reads a committed golden fixture from disk.
// process.cwd() is exactly repository-root/tools/linear-sync when this
// scope runs (internal/command/test.go's runNodeScopeTest sets the child
// process working directory to the registered scope's Dir), so two levels
// up reaches the repository root.
function loadJsonFixture<T>(name: string): T {
  const fixturePath = process.cwd() + "/../../tests/fixtures/linear/" + name;
  return JSON.parse(readFileSync(fixturePath, "utf8")) as T;
}

// toHandle converts one fixture node into the exact LinearEntityHandle
// shape adapter.ts's normalize() consumes, matching operations.test.ts's
// existing toHandle pattern (exactOptionalPropertyTypes-safe: only assign
// an optional field when the fixture actually supplies it).
function toHandle(node: FixtureNode): LinearEntityHandle {
  const handle: LinearEntityHandle = {
    id: node.id,
    updatedAt: new Date(node.updatedAt),
  };
  if (node.name !== undefined) {
    handle.name = node.name;
  }
  if (node.title !== undefined) {
    handle.title = node.title;
  }
  if (node.description !== undefined) {
    handle.description = node.description;
  }
  return handle;
}

// fetchPageFromFixturePages builds a FetchPage that resolves the exact
// fixture page registered for the requested cursor. It never falls back to
// "closest" or "next available" page: a cursor with no exact registered
// page is a fixture-authoring error, not a value fetchAllPages should ever
// see, so this throws rather than guessing.
function fetchPageFromFixturePages(pages: FixturePage[]): FetchPage<LinearEntityHandle> {
  return async (cursor: string | null) => {
    const page = pages.find((candidate) => candidate.requestCursor === cursor);
    if (!page) {
      throw new Error(`no fixture page registered for cursor ${JSON.stringify(cursor)}`);
    }
    return { nodes: page.nodes.map(toHandle), pageInfo: page.pageInfo };
  };
}

// ---------------------------------------------------------------------------
// TestScopeLinearTransportPagination -- the exact marker config/commands.toml
// and internal/command/linear_sync.go's MustDeclareNodeScope registration
// name for the "linear-transport-pagination" scope.
// ---------------------------------------------------------------------------

test("TestScopeLinearTransportPagination", async (t) => {
  await t.test("fetchAllPages exhausts both pages of 51 objects and finds the exact page-two footer", async () => {
    const fixture = loadJsonFixture<PaginatedFixture>("paginated-51.json");
    assert.strictEqual(fixture.schemaVersion, 1, "expected paginated-51.json schemaVersion 1");
    assert.strictEqual(fixture.totalNodes, 51, "fixture must model exactly 51 objects across two pages");
    assert.strictEqual(fixture.pages.length, 2, "fixture must model exactly two pages");

    const result = await fetchAllPages<LinearEntityHandle>(fetchPageFromFixturePages(fixture.pages));
    assert.strictEqual(result.complete, true, "fetchAllPages must exhaust both pages of the fixture connection");
    if (!result.complete) {
      return;
    }

    assert.strictEqual(result.pageCount, 2, "expected exactly two pages consumed");
    assert.strictEqual(result.nodes.length, 51, "expected all 51 objects across both pages");

    const footerNode = result.nodes.find((node) => node.id === fixture.expectedFooterNodeId);
    assert.ok(footerNode, `expected node ${fixture.expectedFooterNodeId} to be present after exhausting both pages`);
    assert.ok(
      (footerNode?.description ?? "").includes(fixture.expectedFooterSubstring),
      "expected the exact page-two GOLC identity footer to be reachable only after exhausting pagination",
    );

    const secondMarkerNode = result.nodes.find((node) => node.id === fixture.secondMarkerNodeId);
    assert.ok(secondMarkerNode, `expected node ${fixture.secondMarkerNodeId} to be present`);
    assert.ok(
      (secondMarkerNode?.description ?? "").includes(fixture.secondMarkerSubstring),
      "expected a second, distinct GOLC identity marker earlier in the connection to survive alongside the page-two marker",
    );
  });

  await t.test("cursor anomalies (repeated, null, indirect loop) produce complete=false and block identity decisions", async (scenarioContext) => {
    const fixture = loadJsonFixture<CursorLoopFixture>("cursor-loop.json");
    assert.strictEqual(fixture.schemaVersion, 1, "expected cursor-loop.json schemaVersion 1");
    assert.ok(fixture.scenarios.length >= 3, "expected at least repeated/null/indirect-loop cursor scenarios");

    for (const scenario of fixture.scenarios) {
      // Each nested subtest must be issued against this callback's own
      // TestContext (scenarioContext), never the outer t: awaiting
      // t.test(...) from inside a callback that is itself still an
      // in-flight t.test(...) call on the same TestContext deadlocks
      // Node's test runner (reentrant subtest scheduling on one context
      // never resolves) instead of throwing or timing out.
      await scenarioContext.test(`scenario: ${scenario.name}`, async () => {
        const result = await fetchAllPages<LinearEntityHandle>(fetchPageFromFixturePages(scenario.pages));
        assert.strictEqual(result.complete, false, `expected scenario ${scenario.name} to fail closed`);
        if (result.complete) {
          return;
        }
        assert.strictEqual(result.code, scenario.expectedCode, `expected scenario ${scenario.name} to report ${scenario.expectedCode}`);
      });
    }
  });

  await t.test("captureSnapshot marks the whole snapshot complete only once every intended connection is exhausted", async () => {
    const fixture = loadJsonFixture<PaginatedFixture>("paginated-51.json");
    const connections: ConnectionQuery[] = [
      {
        entity: "requirement_issue" as EntityKind,
        label: "fixture connection",
        fetchPage: fetchPageFromFixturePages(fixture.pages),
      },
    ];

    const snapshot = await captureSnapshot(connections);
    assert.strictEqual(snapshot.status, "complete", "expected a fully exhausted single connection to mark the snapshot complete");
    assert.strictEqual(snapshot.records.length, 51, "expected every normalized record from every intended connection");

    const markedRecords = snapshot.records.filter((record) => record.description.includes("GOLC local ID:"));
    assert.strictEqual(
      markedRecords.length,
      2,
      "expected both distinct GOLC identity markers preserved individually, not collapsed to one",
    );
  });

  await t.test("captureSnapshot blocks the entire snapshot when any one intended connection has a cursor anomaly", async () => {
    const goodFixture = loadJsonFixture<PaginatedFixture>("paginated-51.json");
    const cursorFixture = loadJsonFixture<CursorLoopFixture>("cursor-loop.json");
    const anomalyScenario = cursorFixture.scenarios[0];
    if (!anomalyScenario) {
      throw new Error("expected at least one cursor-loop scenario");
    }

    const connections: ConnectionQuery[] = [
      {
        entity: "requirement_issue" as EntityKind,
        label: "good connection",
        fetchPage: fetchPageFromFixturePages(goodFixture.pages),
      },
      {
        entity: "task_subissue" as EntityKind,
        label: "anomalous connection",
        fetchPage: fetchPageFromFixturePages(anomalyScenario.pages),
      },
    ];

    const snapshot = await captureSnapshot(connections);
    assert.strictEqual(snapshot.status, "cursor_anomaly", "expected one anomalous connection to block the whole snapshot");
    assert.strictEqual(
      snapshot.records.length,
      0,
      "an incomplete snapshot must never expose partial records (even from an already-completed connection) for an identity/create/preview decision",
    );
    assert.ok(snapshot.reason?.includes("anomalous connection"), "expected the blocking connection's label in the snapshot reason");
  });
});
