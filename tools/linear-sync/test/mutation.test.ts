// mutation.test.ts is the complete offline contract test for adapter.ts's
// create/update uncertain-outcome path (CONTEXT D-20/D-21; T-01-40/T-01-41;
// Plan 01-27 must_haves.artifacts). It registers scope
// "linear-transport-node" (config/commands.toml,
// internal/command/linear_sync.go) through the exact marker
// "TestScopeLinearTransportNode" this file defines below, and proves --
// against the committed fixture
// tests/fixtures/linear/mutation-uncertain.json and a hostile fake SDK
// client that throws a deliberately canary/credential-laden raw error for
// every scenario -- that LinearSdkAdapter.execute's create/update path
// returns a typed "unknown" MutationOutcome immediately (redact.ts's
// safeError), that exactly one SDK call is ever attempted per mutation
// (plus exactly one more for its mandatory readback when that readback is
// the scenario's own failure point), zero automatic write retry, and that
// not one leaked byte reaches the returned outcome. Entirely offline: never
// a real LinearClient, live credential, or network call.
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";

import type { LinearClient } from "@linear/sdk";

import { LinearSdkAdapter } from "../src/adapter.js";
import { scanCanary } from "../src/redact.js";
import type { EntityKind, MutationOutcome, Operation } from "../src/protocol.js";

// ---------------------------------------------------------------------------
// Fixture shape and loader
// ---------------------------------------------------------------------------

interface MutationScenario {
  name: string;
  action: "create" | "update";
  errorName: string;
  attachCause: boolean;
  readbackFails: boolean;
  expectedCode: string;
}

interface MutationUncertainFixture {
  schemaVersion: number;
  description: string;
  canaryMessage: string;
  scenarios: MutationScenario[];
}

// loadFixture reads the committed golden fixture from disk. process.cwd()
// is exactly repository-root/tools/linear-sync when this scope runs
// (internal/command/test.go's runNodeScopeTest sets the child process
// working directory to the registered scope's Dir), so two levels up
// reaches the repository root (matching errors.test.ts/rate-limit.test.ts's
// precedent).
function loadFixture(): MutationUncertainFixture {
  const fixturePath = process.cwd() + "/../../tests/fixtures/linear/mutation-uncertain.json";
  return JSON.parse(readFileSync(fixturePath, "utf8")) as MutationUncertainFixture;
}

// ---------------------------------------------------------------------------
// Hostile fake SDK client -- every method either throws the scenario's
// deliberately hostile, canary/credential-laden raw error, or (for a
// readbackFails scenario) succeeds on the mutation call itself and throws
// only on the immediately following mandatory readback.
// ---------------------------------------------------------------------------

function buildHostileError(scenario: MutationScenario, canaryMessage: string): Error {
  const error = new Error(canaryMessage) as Error & { headers?: unknown; requestBody?: unknown; client?: unknown; cause?: unknown };
  error.name = scenario.errorName;
  error.headers = { authorization: canaryMessage };
  error.requestBody = { variables: { secret: canaryMessage } };
  error.client = { apiKey: canaryMessage };
  if (scenario.attachCause) {
    error.cause = { code: "ECONNRESET" };
  }
  return error;
}

class HostileLinearClient {
  readonly calls: string[] = [];

  constructor(
    private readonly scenario: MutationScenario,
    private readonly canaryMessage: string,
  ) {}

  private mutate(method: string): { id: string; updatedAt: Date } {
    this.calls.push(method);
    if (this.scenario.readbackFails) {
      return { id: "hostile-created-id", updatedAt: new Date("2026-07-21T00:00:00.000Z") };
    }
    throw buildHostileError(this.scenario, this.canaryMessage);
  }

  async createIssue(_fields: unknown) {
    const handle = this.mutate("createIssue");
    return { issue: Promise.resolve(handle) };
  }

  async updateIssue(_id: string, _fields: unknown) {
    this.mutate("updateIssue");
    return {};
  }

  async issue(_id: string) {
    this.calls.push("issue");
    if (this.scenario.readbackFails) {
      throw buildHostileError(this.scenario, this.canaryMessage);
    }
    return undefined;
  }
}

// ---------------------------------------------------------------------------
// TestScopeLinearTransportNode -- the exact marker config/commands.toml and
// internal/command/linear_sync.go's MustDeclareNodeScope registration name
// for the "linear-transport-node" scope.
// ---------------------------------------------------------------------------

test("TestScopeLinearTransportNode", async (t) => {
  const fixture = loadFixture();
  assert.strictEqual(fixture.schemaVersion, 1, "expected mutation-uncertain.json schemaVersion 1");
  assert.ok(fixture.scenarios.length >= 4, "expected a comprehensive set of commit/timeout/readback scenarios");

  for (const scenario of fixture.scenarios) {
    await t.test(scenario.name, async () => {
      const fakeClient = new HostileLinearClient(scenario, fixture.canaryMessage);
      const adapter = new LinearSdkAdapter(fakeClient as unknown as LinearClient);

      const operation = (
        scenario.action === "create"
          ? { entity: "task_subissue" as EntityKind, action: "create", fields: { title: "hostile task", teamId: "team-1" } }
          : { entity: "task_subissue" as EntityKind, action: "update", linearUUID: "existing-uuid", fields: { title: "hostile task" } }
      ) as unknown as Operation;

      const outcome = (await adapter.execute(operation)) as MutationOutcome;

      assert.strictEqual(outcome.status, "unknown", `scenario ${scenario.name} must return a typed unknown outcome, never throw or silently succeed`);
      if (outcome.status !== "unknown") {
        return;
      }
      assert.strictEqual(outcome.diagnostic.code, scenario.expectedCode, `scenario ${scenario.name} expected diagnostic code ${scenario.expectedCode}`);
      assert.strictEqual(outcome.diagnostic.operation, `task_subissue ${scenario.action}`);

      const allowlistedKeys = ["operation", "path", "code", "request", "endpoint", "complexity", "reset"];
      for (const key of Object.keys(outcome.diagnostic)) {
        assert.ok(allowlistedKeys.includes(key), `diagnostic field ${JSON.stringify(key)} is not on the allowlisted surface`);
      }

      const rendered = JSON.stringify(outcome);
      assert.strictEqual(scanCanary(rendered), undefined, `scenario ${scenario.name}: the emitted MutationOutcome must never leak the hostile error's canary/credential content`);
      assert.ok(!rendered.includes(fixture.canaryMessage), `scenario ${scenario.name}: the raw hostile error message must never appear in the emitted outcome`);
      assert.ok(!rendered.includes("headers"), `scenario ${scenario.name}: the raw error's attached headers field name must never appear in the emitted outcome`);
      assert.ok(!rendered.includes("requestBody"), `scenario ${scenario.name}: the raw error's attached requestBody field name must never appear in the emitted outcome`);
      assert.ok(!rendered.includes("client"), `scenario ${scenario.name}: the raw error's attached client field name must never appear in the emitted outcome`);

      const expectedCallCount = scenario.readbackFails ? 2 : 1;
      assert.strictEqual(
        fakeClient.calls.length,
        expectedCallCount,
        `scenario ${scenario.name}: expected exactly ${expectedCallCount} SDK call(s) (the mutation attempt${scenario.readbackFails ? " plus its mandatory readback" : ""}), zero automatic retry`,
      );
    });
  }
});
