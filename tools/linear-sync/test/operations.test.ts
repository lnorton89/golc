// operations.test.ts is the complete fake-SDK hierarchy contract for this
// isolated workspace (CONTEXT D-21; Plan 01-25 must_haves.artifacts). It
// registers scope "linear-sdk-operations" (config/commands.toml,
// internal/command/linear_sync.go) through the exact marker
// "TestScopeLinearSdkOperations" this file defines below, and asserts one
// create/update/readback sequence per entity (project, project_milestone,
// parent_issue, requirement_issue, task_subissue) against the committed
// exact SDK method/input/output transcript at
// tests/fixtures/linear/hierarchy-operations.json -- entirely offline,
// against a fake in-memory SDK client, never a real LinearClient, live
// credential, or network call.
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";

import type { LinearClient } from "@linear/sdk";

import { LinearSdkAdapter } from "../src/adapter.js";
import { decodeOperation, ProtocolDecodeError, type EntityKind, type MutationResult, type Operation, type ReadResult } from "../src/protocol.js";
import { runCLI, type CliOutput } from "../src/cli.js";

// ---------------------------------------------------------------------------
// Fixture shape and loader
// ---------------------------------------------------------------------------

interface FixtureRecord {
  linearUUID: string;
  linearType: EntityKind;
  title: string;
  description: string;
  fields: Record<string, string>;
  updatedAt: string;
}

interface FixtureSdkEntity {
  id: string;
  name?: string;
  title?: string;
  description?: string;
  updatedAt: string;
}

interface FixtureCreate {
  input: Record<string, unknown>;
  linearUUID: string;
  sdkOutput: FixtureSdkEntity;
  record: FixtureRecord;
}

interface FixtureUpdate {
  input: Record<string, unknown>;
  sdkOutput: FixtureSdkEntity;
  record: FixtureRecord;
}

interface FixtureCall {
  method: string;
  input: unknown;
}

interface FixtureEntity {
  entity: EntityKind;
  create: FixtureCreate;
  update: FixtureUpdate;
  calls: FixtureCall[];
}

interface HierarchyFixture {
  schemaVersion: number;
  entities: FixtureEntity[];
}

// loadFixture reads the committed golden transcript from disk.
// process.cwd() is exactly repository-root/tools/linear-sync when this
// scope runs (internal/command/test.go's runNodeScopeTest sets the child
// process working directory to the registered scope's Dir), so two levels
// up reaches the repository root.
function loadFixture(): HierarchyFixture {
  const fixturePath = process.cwd() + "/../../tests/fixtures/linear/hierarchy-operations.json";
  const raw = readFileSync(fixturePath, "utf8");
  return JSON.parse(raw) as HierarchyFixture;
}

// ---------------------------------------------------------------------------
// Fake SDK client
// ---------------------------------------------------------------------------

/**
 * FakeLinearClient is the only "SDK" any test in this file ever talks to:
 * it never opens a socket, never resolves a real GraphQL query, and never
 * needs an API key. It implements exactly the LinearClient accessor/
 * mutation methods adapter.ts calls, recording every call
 * (method + input) in declaration order and returning the fixture's exact
 * pre-authored output for create/update, or its current in-memory state
 * for a read -- mirroring a real backend closely enough that
 * adapter.ts's own mandatory-readback and field-mapping logic (normalize)
 * is genuinely exercised, not bypassed.
 */
class FakeLinearClient {
  readonly calls: FixtureCall[] = [];
  private current: FixtureSdkEntity | undefined;

  constructor(private readonly entityFixture: FixtureEntity) {}

  private record(method: string, input: unknown): void {
    this.calls.push({ method, input });
  }

  private toHandle(entity: FixtureSdkEntity): { id: string; name?: string; title?: string; description?: string; updatedAt: Date } {
    const handle: { id: string; name?: string; title?: string; description?: string; updatedAt: Date } = {
      id: entity.id,
      updatedAt: new Date(entity.updatedAt),
    };
    if (entity.name !== undefined) {
      handle.name = entity.name;
    }
    if (entity.title !== undefined) {
      handle.title = entity.title;
    }
    if (entity.description !== undefined) {
      handle.description = entity.description;
    }
    return handle;
  }

  async project(id: string) {
    this.record("project", id);
    return this.current ? this.toHandle(this.current) : undefined;
  }

  async projectMilestone(id: string) {
    this.record("projectMilestone", id);
    return this.current ? this.toHandle(this.current) : undefined;
  }

  async issue(id: string) {
    this.record("issue", id);
    return this.current ? this.toHandle(this.current) : undefined;
  }

  async createProject(fields: unknown) {
    this.record("createProject", fields);
    this.current = this.entityFixture.create.sdkOutput;
    return { project: Promise.resolve(this.toHandle(this.current)) };
  }

  async createProjectMilestone(fields: unknown) {
    this.record("createProjectMilestone", fields);
    this.current = this.entityFixture.create.sdkOutput;
    return { projectMilestone: Promise.resolve(this.toHandle(this.current)) };
  }

  async createIssue(fields: unknown) {
    this.record("createIssue", fields);
    this.current = this.entityFixture.create.sdkOutput;
    return { issue: Promise.resolve(this.toHandle(this.current)) };
  }

  async updateProject(id: string, fields: unknown) {
    this.record("updateProject", [id, fields]);
    this.current = this.entityFixture.update.sdkOutput;
    return {};
  }

  async updateProjectMilestone(id: string, fields: unknown) {
    this.record("updateProjectMilestone", [id, fields]);
    this.current = this.entityFixture.update.sdkOutput;
    return {};
  }

  async updateIssue(id: string, fields: unknown) {
    this.record("updateIssue", [id, fields]);
    this.current = this.entityFixture.update.sdkOutput;
    return {};
  }
}

// ---------------------------------------------------------------------------
// TestScopeLinearSdkOperations -- the exact marker config/commands.toml and
// internal/command/linear_sync.go's MustDeclareNodeScope registration name
// for the "linear-sdk-operations" scope.
// ---------------------------------------------------------------------------

test("TestScopeLinearSdkOperations", async (t) => {
  const fixture = loadFixture();
  assert.strictEqual(fixture.schemaVersion, 1, "expected hierarchy-operations.json schemaVersion 1");

  const expectedEntities: EntityKind[] = ["project", "project_milestone", "parent_issue", "requirement_issue", "task_subissue"];
  assert.deepStrictEqual(
    fixture.entities.map((entry) => entry.entity),
    expectedEntities,
    "expected exactly one fixture entry per entity kind, in the canonical hierarchy order",
  );

  for (const entityFixture of fixture.entities) {
    await t.test(`${entityFixture.entity}: create, update, and read back through the fake SDK`, async () => {
      const fakeClient = new FakeLinearClient(entityFixture);
      const adapter = new LinearSdkAdapter(fakeClient as unknown as LinearClient);

      const createOperation = {
        entity: entityFixture.entity,
        action: "create",
        fields: entityFixture.create.input,
      } as unknown as Operation;
      const createResult = (await adapter.execute(createOperation)) as MutationResult;
      assert.deepStrictEqual(createResult.record, entityFixture.create.record, "create readback record must match the fixture exactly");

      const updateOperation = {
        entity: entityFixture.entity,
        action: "update",
        linearUUID: entityFixture.create.linearUUID,
        fields: entityFixture.update.input,
      } as unknown as Operation;
      const updateResult = (await adapter.execute(updateOperation)) as MutationResult;
      assert.deepStrictEqual(updateResult.record, entityFixture.update.record, "update readback record must match the fixture exactly");

      const readOperation = {
        entity: entityFixture.entity,
        action: "read",
        linearUUID: entityFixture.create.linearUUID,
      } as unknown as Operation;
      const readResult = (await adapter.execute(readOperation)) as ReadResult;
      assert.ok(readResult.found, `expected ${entityFixture.entity} to read back found=true after create+update`);
      if (readResult.found) {
        assert.deepStrictEqual(readResult.record, entityFixture.update.record, "explicit read must observe the post-update state");
      }

      assert.deepStrictEqual(
        fakeClient.calls,
        entityFixture.calls,
        `${entityFixture.entity}: exact SDK method/input transcript must match the fixture (create+readback, update+readback, read)`,
      );
    });
  }
});

// ---------------------------------------------------------------------------
// Supplementary coverage: cli.ts's strict NDJSON contract and its "no HTTP
// server, no reconciliation policy" structural boundary. Node's test
// runner discovers every top-level test() in this file regardless of name,
// so these run alongside TestScopeLinearSdkOperations without a second
// scope registration.
// ---------------------------------------------------------------------------

test("cli.ts runCLI decodes strict NDJSON, executes one operation per line, and rejects a malformed line closed", async () => {
  const fixture = loadFixture();
  const projectFixture = fixture.entities[0];
  if (!projectFixture) {
    throw new Error("expected at least one fixture entity");
  }

  const fakeClient = new FakeLinearClient(projectFixture);
  const adapter = new LinearSdkAdapter(fakeClient as unknown as LinearClient);

  const createOperation = {
    entity: projectFixture.entity,
    action: "create",
    fields: projectFixture.create.input,
  };
  const ndjsonLine = JSON.stringify(createOperation) + "\n";

  const written: string[] = [];
  const output: CliOutput = { write: (chunk: string) => written.push(chunk) };
  async function* singleLineInput(): AsyncGenerator<string> {
    yield ndjsonLine;
  }

  const processed = await runCLI(singleLineInput(), output, adapter);
  assert.strictEqual(processed, 1, "expected runCLI to process exactly one NDJSON operation");
  assert.strictEqual(written.length, 1, "expected runCLI to write exactly one NDJSON result line");

  const decodedResult = JSON.parse(written[0] as string) as MutationResult;
  assert.deepStrictEqual(decodedResult.record, projectFixture.create.record, "runCLI's output line must match the direct adapter.execute result");

  const badClient = new FakeLinearClient(projectFixture);
  const badAdapter = new LinearSdkAdapter(badClient as unknown as LinearClient);
  async function* malformedLineInput(): AsyncGenerator<string> {
    yield "not json at all\n";
  }
  await assert.rejects(
    () => runCLI(malformedLineInput(), { write: () => undefined }, badAdapter),
    (error: unknown) => error instanceof Error && error.message.startsWith("LINEAR_CLI_LINE_1:"),
    "expected runCLI to reject a malformed NDJSON line closed, naming the exact line number",
  );
});

test("cli.ts contains no HTTP server or listener surface", () => {
  const cliSource = readFileSync(process.cwd() + "/src/cli.ts", "utf8");
  for (const forbidden of ["node:http", "node:https", "node:net", "createServer", "listen(", "express", "fastify"]) {
    assert.ok(!cliSource.includes(forbidden), `cli.ts must never reference ${JSON.stringify(forbidden)} (no HTTP server/listener surface)`);
  }
});

test("protocol.ts strict decoding still rejects an unrecognized entity (regression guard for the fake-SDK contract above)", () => {
  assert.throws(
    () => decodeOperation({ entity: "not_a_real_entity", action: "read", linearUUID: "x" }),
    (error: unknown) => error instanceof ProtocolDecodeError && error.message.startsWith("LINEAR_PROTOCOL_UNKNOWN_ENTITY"),
  );
});
