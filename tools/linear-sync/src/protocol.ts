// protocol.ts is the discriminated Linear operation contract this isolated
// workspace exposes (CONTEXT D-11; Plan 01-13). It defines the complete,
// exhaustive set of Linear operations the TypeScript SDK transport
// supports and the normalized result shapes each operation produces. Go
// retains every reconciliation policy decision (three-way diff, identity
// discovery, ordering, hashing -- internal/trace/reconcile, D-13/D-17);
// this module owns only the transport vocabulary between GOLC's Go plan
// engine and the official Linear SDK. Unknown operation/kind/result shapes
// fail strict decoding rather than falling through to a best-effort call.

/**
 * EntityKind enumerates the exact five Linear entity roles this workspace
 * maps (CONTEXT canonical hierarchy / AGENTS.md "Linear from Day One"): the
 * delivery Project, a Project Milestone, a plan's parent Issue, a REQ-*
 * requirement Issue, and an executable-task sub-Issue.
 */
export type EntityKind =
  | "project"
  | "project_milestone"
  | "parent_issue"
  | "requirement_issue"
  | "task_subissue";

/**
 * OperationAction enumerates the exact three actions this contract
 * supports for every entity kind: a read-back query, a create mutation,
 * and an update mutation. There is no delete action -- local absence never
 * mirrors as a remote delete (CONTEXT D-15); archive/unlink stay explicit
 * reviewed operations owned by internal/trace/reconcile, not this
 * transport contract.
 */
export type OperationAction = "read" | "create" | "update";

export const ENTITY_KINDS: readonly EntityKind[] = [
  "project",
  "project_milestone",
  "parent_issue",
  "requirement_issue",
  "task_subissue",
];

export const OPERATION_ACTIONS: readonly OperationAction[] = ["read", "create", "update"];

// ---------------------------------------------------------------------------
// Field shapes
//
// These field sets stay deliberately narrow: only the repository-owned
// fields this workspace ever writes (CONTEXT D-11 authority split --
// status/assignee/priority/estimate/completed_at are Linear-operational
// fields this adapter never writes).
// ---------------------------------------------------------------------------

export interface ProjectFields {
  name: string;
  teamIds: string[];
  description?: string;
  targetDate?: string;
}

export interface ProjectMilestoneFields {
  name: string;
  description?: string;
  targetDate?: string;
  projectId: string;
}

export interface IssueFields {
  title: string;
  description?: string;
  teamId: string;
  projectId?: string;
  projectMilestoneId?: string;
  parentId?: string;
  labelIds?: string[];
}

/** EntityFieldsFor maps an EntityKind to its exact field shape. */
export type EntityFieldsFor<TEntity extends EntityKind> = TEntity extends "project"
  ? ProjectFields
  : TEntity extends "project_milestone"
    ? ProjectMilestoneFields
    : IssueFields;

// ---------------------------------------------------------------------------
// Read operations
// ---------------------------------------------------------------------------

export interface ReadOperation<TEntity extends EntityKind = EntityKind> {
  entity: TEntity;
  action: "read";
  linearUUID: string;
}

export type ReadProjectOperation = ReadOperation<"project">;
export type ReadProjectMilestoneOperation = ReadOperation<"project_milestone">;
export type ReadParentIssueOperation = ReadOperation<"parent_issue">;
export type ReadRequirementIssueOperation = ReadOperation<"requirement_issue">;
export type ReadTaskSubIssueOperation = ReadOperation<"task_subissue">;

// ---------------------------------------------------------------------------
// Create operations
// ---------------------------------------------------------------------------

export interface CreateOperation<TEntity extends EntityKind = EntityKind> {
  entity: TEntity;
  action: "create";
  fields: EntityFieldsFor<TEntity>;
}

export type CreateProjectOperation = CreateOperation<"project">;
export type CreateProjectMilestoneOperation = CreateOperation<"project_milestone">;
export type CreateParentIssueOperation = CreateOperation<"parent_issue">;
export type CreateRequirementIssueOperation = CreateOperation<"requirement_issue">;
export type CreateTaskSubIssueOperation = CreateOperation<"task_subissue">;

// ---------------------------------------------------------------------------
// Update operations. A mutation only ever targets an already-linked,
// immutable Linear UUID (CONTEXT D-14): there is no update-by-marker or
// update-by-title form anywhere in this contract.
// ---------------------------------------------------------------------------

export interface UpdateOperation<TEntity extends EntityKind = EntityKind> {
  entity: TEntity;
  action: "update";
  linearUUID: string;
  fields: Partial<EntityFieldsFor<TEntity>>;
}

export type UpdateProjectOperation = UpdateOperation<"project">;
export type UpdateProjectMilestoneOperation = UpdateOperation<"project_milestone">;
export type UpdateParentIssueOperation = UpdateOperation<"parent_issue">;
export type UpdateRequirementIssueOperation = UpdateOperation<"requirement_issue">;
export type UpdateTaskSubIssueOperation = UpdateOperation<"task_subissue">;

/**
 * Operation is the exhaustive discriminated union of every request shape
 * this transport accepts: exactly one of {read, create, update} for each
 * of the five entity kinds (15 explicit variants). There is no generic or
 * catch-all variant -- an operation that does not match one of these exact
 * shapes fails strict decoding (decodeOperation below) rather than
 * silently reaching a best-effort GraphQL call.
 */
export type Operation =
  | ReadProjectOperation
  | ReadProjectMilestoneOperation
  | ReadParentIssueOperation
  | ReadRequirementIssueOperation
  | ReadTaskSubIssueOperation
  | CreateProjectOperation
  | CreateProjectMilestoneOperation
  | CreateParentIssueOperation
  | CreateRequirementIssueOperation
  | CreateTaskSubIssueOperation
  | UpdateProjectOperation
  | UpdateProjectMilestoneOperation
  | UpdateParentIssueOperation
  | UpdateRequirementIssueOperation
  | UpdateTaskSubIssueOperation;

// ---------------------------------------------------------------------------
// Results
// ---------------------------------------------------------------------------

/**
 * NormalizedRecord is the transport-neutral shape one Linear read or
 * mutation readback produces. Field names match
 * internal/trace/transport.RemoteRecord (Go, Plan 01-23) exactly --
 * linearUUID/linearType/title/description/fields/updatedAt -- so a future
 * real adapter can serialize straight into that Snapshot contract without
 * a second field-mapping layer.
 */
export interface NormalizedRecord {
  linearUUID: string;
  linearType: EntityKind;
  title: string;
  description: string;
  fields: Record<string, string>;
  updatedAt: string;
}

/**
 * ReadResult is the normalized outcome of a "read" operation: either the
 * record was found, or it was not. There is no ambiguous or partial read
 * result at this layer -- CONTEXT D-21 completeness diagnostics
 * (incomplete/partial/cursor_anomaly/ambiguous/rate_limited) live one
 * layer up, at the Snapshot/Transport boundary internal/trace/transport
 * (Go, Plan 01-23) already owns.
 */
export type ReadResult = { found: true; record: NormalizedRecord } | { found: false };

/**
 * MutationResult is the normalized outcome of a "create" or "update"
 * operation. It always carries a readback record: every write this
 * adapter performs is confirmed by re-reading the exact object the
 * mutation targeted, never trusting the mutation payload echo alone
 * (mutations require readback).
 */
export interface MutationResult {
  record: NormalizedRecord;
}

/**
 * OperationResult maps an Operation's action to its exact result shape:
 * "read" always produces a ReadResult; "create"/"update" always produce a
 * MutationResult with a mandatory readback.
 */
export type OperationResult<TOperation extends Operation> = TOperation["action"] extends "read"
  ? ReadResult
  : MutationResult;

// ---------------------------------------------------------------------------
// Snapshot (exhaustive connection capture)
// ---------------------------------------------------------------------------

/**
 * SnapshotStatus mirrors internal/trace/transport.SnapshotStatus (Go)
 * exactly -- CONTEXT D-21's complete/incomplete/partial/cursor_anomaly/
 * ambiguous/rate_limited vocabulary. Plan 01-14's pagination-driven
 * captureSnapshot (adapter.ts) only ever produces "complete" or
 * "cursor_anomaly"; "partial" and "rate_limited" are produced starting
 * with Plan 01-26's GraphQL/rate error normalization (errors.ts), layered
 * on top of this same Snapshot shape without changing it here.
 * "incomplete" and "ambiguous" are reserved for future producers and are
 * declared now only to keep this vocabulary exhaustive and Go-matching.
 */
export type SnapshotStatus =
  | "complete"
  | "incomplete"
  | "partial"
  | "cursor_anomaly"
  | "ambiguous"
  | "rate_limited";

/**
 * Snapshot is the transport-neutral complete-capture outcome this
 * workspace reports, matching internal/trace/transport.Snapshot (Go)
 * field names exactly. Only status "complete" may ever feed a
 * reconciliation preview (CONTEXT D-21); every other status is a
 * diagnostic that must never reach an identity or create/preview decision
 * (T-01-39) -- records is always empty whenever status is not "complete",
 * so an incomplete or anomalous capture can never expose a partial record
 * set for identity use.
 */
export interface Snapshot {
  status: SnapshotStatus;
  reason?: string;
  records: NormalizedRecord[];
}

// ---------------------------------------------------------------------------
// Transport diagnostics (Plan 01-26 -- GraphQL/rate normalization)
// ---------------------------------------------------------------------------

/**
 * TransportDiagnostic is the exact allowlisted metadata surface a partial
 * GraphQL error or rate-limit signal may ever expose (CONTEXT D-20/D-21;
 * T-01-40): path/code/operation/request/endpoint/complexity/reset only.
 * No raw error message, GraphQL query text, variables, header value, or
 * credential may ever appear on this type -- errors.ts's
 * normalizeGraphQLResult/normalizeRateLimit are the sole producers of this
 * shape, and adapter.ts's captureSnapshot is the sole place it is folded
 * into a Snapshot's "partial"/"rate_limited" reason text (CONTEXT D-20's
 * "secret values must never appear in previews, logs, errors" extended
 * here to every transport diagnostic, not only credentials).
 */
export interface TransportDiagnostic {
  operation: string;
  path?: string;
  code?: string;
  request?: string;
  endpoint?: string;
  complexity?: number;
  reset?: string;
}

// ---------------------------------------------------------------------------
// Strict decoding
// ---------------------------------------------------------------------------

/**
 * ProtocolDecodeError is thrown by decodeOperation/decodeNormalizedRecord
 * when an input does not match one of this contract's exact discriminated
 * shapes. Its message names a stable code so callers can distinguish an
 * unknown entity, an unknown action, or a malformed field set.
 */
export class ProtocolDecodeError extends Error {
  constructor(code: string, detail: string) {
    super(`${code}: ${detail}`);
    this.name = "ProtocolDecodeError";
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isNonEmptyString(value: unknown): value is string {
  return typeof value === "string" && value.length > 0;
}

function isEntityKind(value: unknown): value is EntityKind {
  return typeof value === "string" && (ENTITY_KINDS as readonly string[]).includes(value);
}

function isOperationAction(value: unknown): value is OperationAction {
  return typeof value === "string" && (OPERATION_ACTIONS as readonly string[]).includes(value);
}

/**
 * decodeOperation strictly decodes an unknown value into an Operation. An
 * unrecognized "entity", an unrecognized "action", or a field set that
 * does not match the exact shape that (entity, action) pair requires all
 * throw a ProtocolDecodeError rather than returning a best-effort partial
 * Operation. This is this contract's core safety property: unknown
 * operation/kind/result shapes fail strict decoding.
 */
export function decodeOperation(value: unknown): Operation {
  if (!isRecord(value)) {
    throw new ProtocolDecodeError("LINEAR_PROTOCOL_OPERATION_SHAPE", "operation is not an object");
  }
  const { entity, action } = value;
  if (!isEntityKind(entity)) {
    throw new ProtocolDecodeError(
      "LINEAR_PROTOCOL_UNKNOWN_ENTITY",
      `entity ${JSON.stringify(entity)} is not one of ${ENTITY_KINDS.join(", ")}`,
    );
  }
  if (!isOperationAction(action)) {
    throw new ProtocolDecodeError(
      "LINEAR_PROTOCOL_UNKNOWN_ACTION",
      `action ${JSON.stringify(action)} is not one of ${OPERATION_ACTIONS.join(", ")}`,
    );
  }

  if (action === "read") {
    if (!isNonEmptyString(value.linearUUID)) {
      throw new ProtocolDecodeError("LINEAR_PROTOCOL_READ_SHAPE", "read operation requires a non-empty linearUUID");
    }
    return { entity, action: "read", linearUUID: value.linearUUID } as Operation;
  }

  if (action === "create") {
    if (!isRecord(value.fields)) {
      throw new ProtocolDecodeError("LINEAR_PROTOCOL_CREATE_SHAPE", "create operation requires a fields object");
    }
    return { entity, action: "create", fields: value.fields } as unknown as Operation;
  }

  // action === "update" (exhaustive: OperationAction has exactly three members)
  if (!isNonEmptyString(value.linearUUID)) {
    throw new ProtocolDecodeError("LINEAR_PROTOCOL_UPDATE_SHAPE", "update operation requires a non-empty linearUUID");
  }
  if (!isRecord(value.fields)) {
    throw new ProtocolDecodeError("LINEAR_PROTOCOL_UPDATE_SHAPE", "update operation requires a fields object");
  }
  return { entity, action: "update", linearUUID: value.linearUUID, fields: value.fields } as unknown as Operation;
}

/**
 * decodeNormalizedRecord strictly decodes an unknown value into a
 * NormalizedRecord, matching the same exhaustive-shape discipline
 * decodeOperation applies to requests: every field is required and typed
 * exactly, and any field.value entry that is not a string fails closed.
 */
export function decodeNormalizedRecord(value: unknown): NormalizedRecord {
  if (!isRecord(value)) {
    throw new ProtocolDecodeError("LINEAR_PROTOCOL_RECORD_SHAPE", "record is not an object");
  }
  const { linearUUID, linearType, title, description, fields, updatedAt } = value;
  if (!isNonEmptyString(linearUUID)) {
    throw new ProtocolDecodeError("LINEAR_PROTOCOL_RECORD_SHAPE", "record requires a non-empty linearUUID");
  }
  if (!isEntityKind(linearType)) {
    throw new ProtocolDecodeError(
      "LINEAR_PROTOCOL_UNKNOWN_ENTITY",
      `record linearType ${JSON.stringify(linearType)} is not one of ${ENTITY_KINDS.join(", ")}`,
    );
  }
  if (typeof title !== "string") {
    throw new ProtocolDecodeError("LINEAR_PROTOCOL_RECORD_SHAPE", "record requires a string title");
  }
  if (typeof description !== "string") {
    throw new ProtocolDecodeError("LINEAR_PROTOCOL_RECORD_SHAPE", "record requires a string description");
  }
  if (!isRecord(fields)) {
    throw new ProtocolDecodeError("LINEAR_PROTOCOL_RECORD_SHAPE", "record requires a fields object");
  }
  const normalizedFields: Record<string, string> = {};
  for (const [key, fieldValue] of Object.entries(fields)) {
    if (typeof fieldValue !== "string") {
      throw new ProtocolDecodeError("LINEAR_PROTOCOL_RECORD_SHAPE", `record field ${JSON.stringify(key)} is not a string`);
    }
    normalizedFields[key] = fieldValue;
  }
  if (typeof updatedAt !== "string") {
    throw new ProtocolDecodeError("LINEAR_PROTOCOL_RECORD_SHAPE", "record requires a string updatedAt");
  }
  return { linearUUID, linearType, title, description, fields: normalizedFields, updatedAt };
}
