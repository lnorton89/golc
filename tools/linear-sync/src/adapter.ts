// adapter.ts maps the discriminated Operation contract (protocol.ts) onto
// the official @linear/sdk LinearClient (CONTEXT D-01/D-03; Plan 01-13
// key_links). It owns transport only: field mapping, request dispatch, and
// mandatory create/update readback. It never makes a reconciliation
// decision -- identity discovery, three-way conflict detection, ordering,
// and plan construction all stay in Go (internal/trace/reconcile,
// CONTEXT D-11/D-13/D-17). This module is the official-SDK mapping without
// reconciliation policy.
import { LinearClient } from "@linear/sdk";

import {
  ProtocolDecodeError,
  type EntityKind,
  type IssueFields,
  type MutationResult,
  type NormalizedRecord,
  type Operation,
  type OperationResult,
  type ProjectFields,
  type ProjectMilestoneFields,
  type ReadResult,
} from "./protocol.js";

/**
 * LinearEntityHandle is the narrow subset of a Linear SDK model
 * (Project | ProjectMilestone | Issue) this adapter reads from: an
 * immutable id, either a "name" (Project/ProjectMilestone) or "title"
 * (Issue), an optional description, and a required updatedAt. No other
 * SDK model field crosses this boundary.
 */
interface LinearEntityHandle {
  id: string;
  name?: string;
  title?: string;
  description?: string | null;
  updatedAt: Date;
}

/**
 * assertExhaustive is called from the default branch of every switch over
 * EntityKind. If a sixth entity kind is ever added to protocol.ts without
 * updating this file, the `never` parameter type makes that omission a
 * compile-time error here, not a silent runtime fallthrough.
 */
function assertExhaustive(value: never, code: string): never {
  throw new ProtocolDecodeError(code, `unhandled entity ${JSON.stringify(value)}`);
}

/**
 * readByEntity dispatches a read to the exact LinearClient accessor for
 * entity: project()/projectMilestone() for those two kinds, issue() for
 * all three Issue-shaped kinds (parent/requirement/task). It never issues
 * a generic/untyped GraphQL call.
 */
async function readByEntity(
  client: LinearClient,
  entity: EntityKind,
  linearUUID: string,
): Promise<LinearEntityHandle | undefined> {
  switch (entity) {
    case "project":
      return (await client.project(linearUUID)) as unknown as LinearEntityHandle;
    case "project_milestone":
      return (await client.projectMilestone(linearUUID)) as unknown as LinearEntityHandle;
    case "parent_issue":
    case "requirement_issue":
    case "task_subissue":
      return (await client.issue(linearUUID)) as unknown as LinearEntityHandle;
    default:
      return assertExhaustive(entity, "LINEAR_ADAPTER_UNKNOWN_ENTITY");
  }
}

/**
 * normalize converts one LinearClient entity handle into the
 * transport-neutral NormalizedRecord shape protocol.ts declares, matching
 * internal/trace/transport.RemoteRecord's field names exactly (Go, Plan
 * 01-23) so a future real adapter can hand this straight to that Snapshot
 * contract without a second field-mapping layer.
 */
function normalize(entity: EntityKind, handle: LinearEntityHandle): NormalizedRecord {
  return {
    linearUUID: handle.id,
    linearType: entity,
    title: handle.name ?? handle.title ?? "",
    description: handle.description ?? "",
    fields: {},
    updatedAt: handle.updatedAt.toISOString(),
  };
}

async function readOperation(client: LinearClient, entity: EntityKind, linearUUID: string): Promise<ReadResult> {
  const handle = await readByEntity(client, entity, linearUUID);
  if (!handle) {
    return { found: false };
  }
  return { found: true, record: normalize(entity, handle) };
}

/**
 * readBackOrFail re-reads the exact object a mutation just targeted and
 * fails closed if it does not read back. Both createOperation and
 * updateOperation call this instead of trusting the mutation payload echo
 * (mutations require readback).
 */
async function readBackOrFail(client: LinearClient, entity: EntityKind, linearUUID: string, code: string): Promise<MutationResult> {
  const handle = await readByEntity(client, entity, linearUUID);
  if (!handle) {
    throw new ProtocolDecodeError(code, `${entity} ${linearUUID} did not read back after mutation`);
  }
  return { record: normalize(entity, handle) };
}

async function createOperation(
  client: LinearClient,
  entity: EntityKind,
  fields: ProjectFields | ProjectMilestoneFields | IssueFields,
): Promise<MutationResult> {
  let createdId: string;
  switch (entity) {
    case "project": {
      const payload = await client.createProject(fields as ProjectFields);
      const project = await payload.project;
      if (!project) {
        throw new ProtocolDecodeError("LINEAR_ADAPTER_CREATE_FAILED", "createProject returned no project");
      }
      createdId = project.id;
      break;
    }
    case "project_milestone": {
      const payload = await client.createProjectMilestone(fields as ProjectMilestoneFields);
      const milestone = await payload.projectMilestone;
      if (!milestone) {
        throw new ProtocolDecodeError("LINEAR_ADAPTER_CREATE_FAILED", "createProjectMilestone returned no milestone");
      }
      createdId = milestone.id;
      break;
    }
    case "parent_issue":
    case "requirement_issue":
    case "task_subissue": {
      const payload = await client.createIssue(fields as IssueFields);
      const issue = await payload.issue;
      if (!issue) {
        throw new ProtocolDecodeError("LINEAR_ADAPTER_CREATE_FAILED", "createIssue returned no issue");
      }
      createdId = issue.id;
      break;
    }
    default:
      return assertExhaustive(entity, "LINEAR_ADAPTER_UNKNOWN_ENTITY");
  }

  return readBackOrFail(client, entity, createdId, "LINEAR_ADAPTER_READBACK_FAILED");
}

async function updateOperation(
  client: LinearClient,
  entity: EntityKind,
  linearUUID: string,
  fields: Partial<ProjectFields> | Partial<ProjectMilestoneFields> | Partial<IssueFields>,
): Promise<MutationResult> {
  switch (entity) {
    case "project":
      await client.updateProject(linearUUID, fields as Partial<ProjectFields>);
      break;
    case "project_milestone":
      await client.updateProjectMilestone(linearUUID, fields as Partial<ProjectMilestoneFields>);
      break;
    case "parent_issue":
    case "requirement_issue":
    case "task_subissue":
      await client.updateIssue(linearUUID, fields as Partial<IssueFields>);
      break;
    default:
      return assertExhaustive(entity, "LINEAR_ADAPTER_UNKNOWN_ENTITY");
  }

  return readBackOrFail(client, entity, linearUUID, "LINEAR_ADAPTER_READBACK_FAILED");
}

/**
 * LinearSdkAdapter is the thin transport this workspace exposes: it holds
 * one authenticated LinearClient and executes exactly one strictly-decoded
 * Operation at a time. It never batches, never infers identity, and never
 * decides what to do about a conflict -- all of that stays in Go
 * (internal/trace/reconcile).
 */
export class LinearSdkAdapter {
  private readonly client: LinearClient;

  constructor(client: LinearClient) {
    this.client = client;
  }

  /**
   * execute dispatches one Operation to the matching LinearClient call and
   * returns its normalized result. Every branch below is exhaustive over
   * OperationAction; an operation with an action outside {read, create,
   * update} was already rejected by protocol.ts's decodeOperation before
   * it could reach this method.
   */
  async execute<TOperation extends Operation>(operation: TOperation): Promise<OperationResult<TOperation>> {
    if (operation.action === "read") {
      return (await readOperation(this.client, operation.entity, operation.linearUUID)) as OperationResult<TOperation>;
    }
    if (operation.action === "create") {
      return (await createOperation(this.client, operation.entity, operation.fields)) as OperationResult<TOperation>;
    }
    return (await updateOperation(
      this.client,
      operation.entity,
      operation.linearUUID,
      operation.fields,
    )) as OperationResult<TOperation>;
  }
}

/**
 * createLinearSdkAdapter constructs a LinearSdkAdapter from an API key,
 * matching D-19/D-20: the key itself is never logged, echoed, or persisted
 * by this module -- it is handed directly to the official LinearClient
 * constructor and never inspected again.
 */
export function createLinearSdkAdapter(apiKey: string): LinearSdkAdapter {
  return new LinearSdkAdapter(new LinearClient({ apiKey }));
}
