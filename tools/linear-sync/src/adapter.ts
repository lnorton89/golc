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
  type MutationOutcome,
  type NormalizedRecord,
  type Operation,
  type OperationResult,
  type ProjectFields,
  type ProjectMilestoneFields,
  type ReadResult,
  type Snapshot,
} from "./protocol.js";
import { fetchAllPages, type FetchPage } from "./pagination.js";
import {
  describeDiagnostics,
  normalizeGraphQLResult,
  normalizeRateLimit,
  type RawGraphQLResponse,
  type RawRateLimitSignal,
  type RequestContext,
} from "./errors.js";
import { safeError } from "./redact.js";

/**
 * LinearEntityHandle is the narrow subset of a Linear SDK model
 * (Project | ProjectMilestone | Issue) this adapter reads from: an
 * immutable id, either a "name" (Project/ProjectMilestone) or "title"
 * (Issue), an optional description, and a required updatedAt. No other
 * SDK model field crosses this boundary. Exported so pagination-driven
 * connection reads (captureSnapshot below, and its tests) can construct
 * and normalize the exact same shape a single-entity read produces.
 */
export interface LinearEntityHandle {
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

/**
 * ConnectionQuery describes one intended Linear Relay connection to
 * exhaust during a snapshot capture: which entity kind its records
 * normalize as, a diagnostic label for anomaly reporting, and the paged
 * fetcher itself (a real @linear/sdk connection walk, or a fixture-driven
 * fake in tests). captureSnapshot never issues a GraphQL call directly --
 * every connection read routes through fetchAllPages (pagination.ts; Plan
 * 01-14 key_links "all connection reads" -> fetchAllPages).
 */
export interface ConnectionQuery<TEntity extends EntityKind = EntityKind> {
  entity: TEntity;
  label: string;
  fetchPage: FetchPage<LinearEntityHandle>;
  /**
   * probeGraphQLResult is an optional preflight raw-response probe
   * captureSnapshot inspects through errors.ts's normalizeGraphQLResult
   * before exhausting this connection's pages (Plan 01-26; CONTEXT
   * D-21/T-01-40): a real adapter supplies the connection's most recently
   * observed raw HTTP-200 GraphQL envelope (data/errors together); a
   * fixture-driven fake in tests supplies the exact
   * data-plus-errors.json scenario. A connection that omits this (every
   * existing Plan 01-14 caller/test) behaves exactly as before --
   * captureSnapshot never invents a probe.
   */
  probeGraphQLResult?: () => Promise<RawGraphQLResponse>;
  /**
   * probeRateLimit is the parallel preflight for rate-limit evidence,
   * normalized through errors.ts's normalizeRateLimit and blocking the
   * whole snapshot with status "rate_limited" the same way a
   * probeGraphQLResult data-plus-errors probe blocks it with "partial".
   * Returning undefined (or omitting this field) means no rate-limit
   * signal was observed for this connection.
   */
  probeRateLimit?: () => Promise<RawRateLimitSignal | undefined>;
}

/**
 * captureSnapshot exhausts every one of the given connections through
 * fetchAllPages and normalizes every node it reads (CONTEXT D-21;
 * T-01-39/T-01-40). Before paginating a connection, its optional
 * probeGraphQLResult/probeRateLimit are inspected first: a non-empty
 * GraphQL errors array (even alongside populated data, and even on
 * HTTP 200) or an observed rate-limit signal each block the whole
 * snapshot immediately -- exactly like a cursor anomaly already does --
 * discarding every record already read (including from connections that
 * already completed), so a hidden later page or partial error can never
 * spoof absence or uniqueness for an identity decision. Multiple matching
 * records across connections are preserved individually here, never
 * deduplicated or collapsed: whether zero, one, or more than one record
 * carries a given GOLC identity marker is a decision
 * internal/trace/reconcile (Go) makes, not this transport.
 */
export async function captureSnapshot(connections: readonly ConnectionQuery[]): Promise<Snapshot> {
  const records: NormalizedRecord[] = [];
  for (const connection of connections) {
    if (connection.probeGraphQLResult) {
      const response = await connection.probeGraphQLResult();
      const outcome = normalizeGraphQLResult(response, { operation: connection.label });
      if (outcome.kind === "partial") {
        return {
          status: "partial",
          reason: describeDiagnostics(connection.label, outcome.diagnostics),
          records: [],
        };
      }
    }
    if (connection.probeRateLimit) {
      const signal = await connection.probeRateLimit();
      if (signal !== undefined) {
        const diagnostic = normalizeRateLimit(signal, { operation: connection.label });
        return {
          status: "rate_limited",
          reason: describeDiagnostics(connection.label, [diagnostic]),
          records: [],
        };
      }
    }

    const result = await fetchAllPages<LinearEntityHandle>(connection.fetchPage);
    if (!result.complete) {
      return {
        status: "cursor_anomaly",
        reason: `${connection.label}: ${result.reason} (${result.code})`,
        records: [],
      };
    }
    for (const node of result.nodes) {
      records.push(normalize(connection.entity, node));
    }
  }
  return { status: "complete", records };
}

async function readOperation(client: LinearClient, entity: EntityKind, linearUUID: string): Promise<ReadResult> {
  const handle = await readByEntity(client, entity, linearUUID);
  if (!handle) {
    return { found: false };
  }
  return { found: true, record: normalize(entity, handle) };
}

/**
 * confirmReadback re-reads the exact object a mutation attempt just
 * targeted (Plan 01-27; CONTEXT D-20/D-21; T-01-40/T-01-41): both
 * createOperation and updateOperation call this instead of trusting the
 * mutation payload echo (mutations require readback). A missing record or
 * any exception the readback call itself throws (a timeout, an aborted
 * request) never propagates raw -- it becomes a typed "unknown"
 * MutationOutcome via redact.ts's safeError, immediately, with no retry.
 */
async function confirmReadback(client: LinearClient, entity: EntityKind, linearUUID: string, context: RequestContext): Promise<MutationOutcome> {
  let handle: LinearEntityHandle | undefined;
  try {
    handle = await readByEntity(client, entity, linearUUID);
  } catch (error) {
    return { status: "unknown", diagnostic: safeError(error, context) };
  }
  if (!handle) {
    return { status: "unknown", diagnostic: safeError(new Error("mutation did not read back"), context) };
  }
  return { status: "confirmed", record: normalize(entity, handle) };
}

/**
 * createOperation performs exactly one create mutation attempt (CONTEXT
 * D-21; T-01-40): the SDK call and its mandatory readback are the only two
 * remote calls ever made -- any exception from either (a partial GraphQL
 * error, a timeout, an aborted request, or any other SDK failure) returns a
 * typed "unknown" MutationOutcome immediately via redact.ts's safeError,
 * with zero automatic retry. Go retains sole authority for discovering the
 * true remote postcondition of an "unknown" create (identity-marker
 * discovery, internal/trace/apply/engine.go's applyUnlinkedOperation).
 */
async function createOperation(
  client: LinearClient,
  entity: EntityKind,
  fields: ProjectFields | ProjectMilestoneFields | IssueFields,
): Promise<MutationOutcome> {
  const context: RequestContext = { operation: `${entity} create` };
  let createdId: string;
  try {
    switch (entity) {
      case "project": {
        const payload = await client.createProject(fields as ProjectFields);
        const project = await payload.project;
        if (!project) {
          throw new Error("createProject returned no project");
        }
        createdId = project.id;
        break;
      }
      case "project_milestone": {
        const payload = await client.createProjectMilestone(fields as ProjectMilestoneFields);
        const milestone = await payload.projectMilestone;
        if (!milestone) {
          throw new Error("createProjectMilestone returned no milestone");
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
          throw new Error("createIssue returned no issue");
        }
        createdId = issue.id;
        break;
      }
      default:
        return assertExhaustive(entity, "LINEAR_ADAPTER_UNKNOWN_ENTITY");
    }
  } catch (error) {
    return { status: "unknown", diagnostic: safeError(error, context) };
  }

  return confirmReadback(client, entity, createdId, context);
}

/**
 * updateOperation performs exactly one update mutation attempt, mirroring
 * createOperation's single-attempt/typed-unknown-outcome discipline (CONTEXT
 * D-21; T-01-40): the update call and its mandatory readback are the only
 * two remote calls ever made, and any exception from either returns a typed
 * "unknown" MutationOutcome immediately via redact.ts's safeError, with
 * zero automatic retry.
 */
async function updateOperation(
  client: LinearClient,
  entity: EntityKind,
  linearUUID: string,
  fields: Partial<ProjectFields> | Partial<ProjectMilestoneFields> | Partial<IssueFields>,
): Promise<MutationOutcome> {
  const context: RequestContext = { operation: `${entity} update` };
  try {
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
  } catch (error) {
    return { status: "unknown", diagnostic: safeError(error, context) };
  }

  return confirmReadback(client, entity, linearUUID, context);
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
