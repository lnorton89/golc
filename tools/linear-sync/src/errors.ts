// errors.ts normalizes partial GraphQL errors and rate-limit signals into
// the exact allowlisted diagnostic surface protocol.ts's TransportDiagnostic
// declares (CONTEXT D-20/D-21; T-01-40): path/code/operation/request/
// endpoint/complexity/reset only -- never a raw error message, GraphQL
// query, variables, header value, or credential. It also owns the one
// retry policy this workspace ever applies to a failed request: bounded
// retry is permitted only for a "read" operation observing a transient
// 5xx-class transport failure; every rate-limited or partial-data-plus-
// errors diagnosis observed on a "create"/"update" mutation returns a
// typed stop-write decision instead -- mutations never flow through
// generic retry (CONTEXT D-21 "rate limits stop writes"). This module
// makes no network call itself: every input is a plain, already-received
// response/signal shape supplied by the caller (adapter.ts for a real
// transport, or a fixture-driven fake transcript in tests), so both
// normalization and the retry policy are provable entirely offline
// (tests/fixtures/linear/data-plus-errors.json,
// tests/fixtures/linear/rate-limited.json).

import type { OperationAction, TransportDiagnostic } from "./protocol.js";

// ---------------------------------------------------------------------------
// Raw, untrusted input shapes -- exactly what a real GraphQL/HTTP transport
// may hand this module. Every field is optional/unknown-typed: a hostile
// or malformed transport must never crash normalization, only fail closed.
// ---------------------------------------------------------------------------

/** RawGraphQLError is one untrusted GraphQL error entry as received on the
 * wire (Linear's GraphQL error shape: message, path, extensions). */
export interface RawGraphQLError {
  message?: unknown;
  path?: unknown;
  extensions?: {
    code?: unknown;
    complexity?: unknown;
    [key: string]: unknown;
  };
  [key: string]: unknown;
}

/**
 * RawGraphQLResponse is the exact untrusted HTTP-200 GraphQL envelope
 * shape this module inspects: data and errors are read together -- a
 * non-empty errors array blocks the read even when data is also present
 * (CONTEXT D-21 "HTTP-200 data-plus-errors is incomplete/blocked").
 */
export interface RawGraphQLResponse {
  data?: unknown;
  errors?: RawGraphQLError[];
}

/**
 * RawRateLimitSignal is the untrusted rate-limit evidence a real
 * transport observes (GraphQL error extensions, or response headers,
 * already collapsed to one plain object by the caller before this module
 * ever sees it). Only requestId/endpoint/complexity/reset ever leave
 * normalizeRateLimit; every other field is ignored.
 */
export interface RawRateLimitSignal {
  requestId?: unknown;
  endpoint?: unknown;
  complexity?: unknown;
  reset?: unknown;
  [key: string]: unknown;
}

/**
 * RequestContext names the operation this normalization is diagnosing (a
 * ConnectionQuery label or a protocol.ts Operation description) and the
 * safe display endpoint. Never the request's query text or variables.
 */
export interface RequestContext {
  operation: string;
  endpoint?: string;
}

// ---------------------------------------------------------------------------
// Safe coercion helpers -- every allowlisted field is coerced to a string
// or number and dropped (never thrown) if the raw value is not safely
// representable, so a hostile/malformed raw error can never smuggle an
// object, function, or oversized payload into a diagnostic.
// ---------------------------------------------------------------------------

function safeString(value: unknown): string | undefined {
  return typeof value === "string" && value.length > 0 ? value : undefined;
}

function safeNumber(value: unknown): number | undefined {
  return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}

function safePath(value: unknown): string | undefined {
  if (!Array.isArray(value)) {
    return undefined;
  }
  const segments = value.filter(
    (segment): segment is string | number => typeof segment === "string" || typeof segment === "number",
  );
  if (segments.length === 0) {
    return undefined;
  }
  return segments.join(".");
}

// ---------------------------------------------------------------------------
// Normalization
// ---------------------------------------------------------------------------

/**
 * normalizeGraphQLError converts one untrusted RawGraphQLError into the
 * exact allowlisted TransportDiagnostic surface: path/code come from the
 * error itself; operation/endpoint come from the caller's RequestContext,
 * never from the error's own free-text message.
 */
export function normalizeGraphQLError(error: RawGraphQLError, context: RequestContext): TransportDiagnostic {
  const diagnostic: TransportDiagnostic = { operation: context.operation };
  const path = safePath(error.path);
  if (path !== undefined) {
    diagnostic.path = path;
  }
  const code = safeString(error.extensions?.code);
  if (code !== undefined) {
    diagnostic.code = code;
  }
  if (context.endpoint !== undefined) {
    diagnostic.endpoint = context.endpoint;
  }
  const complexity = safeNumber(error.extensions?.complexity);
  if (complexity !== undefined) {
    diagnostic.complexity = complexity;
  }
  return diagnostic;
}

/**
 * normalizeRateLimit converts one untrusted RawRateLimitSignal into the
 * exact allowlisted TransportDiagnostic surface.
 */
export function normalizeRateLimit(signal: RawRateLimitSignal, context: RequestContext): TransportDiagnostic {
  const diagnostic: TransportDiagnostic = { operation: context.operation };
  const request = safeString(signal.requestId);
  if (request !== undefined) {
    diagnostic.request = request;
  }
  const endpoint = safeString(signal.endpoint) ?? context.endpoint;
  if (endpoint !== undefined) {
    diagnostic.endpoint = endpoint;
  }
  const complexity = safeNumber(signal.complexity);
  if (complexity !== undefined) {
    diagnostic.complexity = complexity;
  }
  const reset = safeString(signal.reset);
  if (reset !== undefined) {
    diagnostic.reset = reset;
  }
  return diagnostic;
}

/**
 * GraphQLResultOutcome is the result of inspecting one raw GraphQL
 * response's data and errors together. "clean" only when errors is absent
 * or empty; any non-empty errors array (even alongside a populated data
 * object, and even though the HTTP transport reported 200) produces
 * "partial" with one TransportDiagnostic per error -- CONTEXT D-21's core
 * safety property this module exists to enforce.
 */
export type GraphQLResultOutcome = { kind: "clean" } | { kind: "partial"; diagnostics: TransportDiagnostic[] };

/**
 * normalizeGraphQLResult is this module's sole entrypoint for the
 * data-plus-errors check (adapter.ts's "data/errors/rate normalization"
 * key_link): it inspects response.data and response.errors together and
 * blocks -- returning "partial" -- the instant errors is a non-empty
 * array, regardless of whether data is also populated and regardless of
 * the transport-level HTTP status. It never inspects an HTTP status code
 * itself -- a real transport's 5xx handling is a distinct, retry-eligible
 * path (decideRetry below), not a data-plus-errors diagnosis.
 */
export function normalizeGraphQLResult(response: RawGraphQLResponse, context: RequestContext): GraphQLResultOutcome {
  const errors = Array.isArray(response.errors) ? response.errors : [];
  if (errors.length === 0) {
    return { kind: "clean" };
  }
  return {
    kind: "partial",
    diagnostics: errors.map((error) => normalizeGraphQLError(error, context)),
  };
}

/**
 * describeDiagnostics renders a stable, allowlist-only summary string for
 * a Snapshot's "reason" field (protocol.ts's Snapshot already matches
 * internal/trace/transport.Snapshot's exact shape and carries no
 * dedicated diagnostics array, so every diagnostic detail this module
 * produces must be safely embedded into that one reason string --
 * matching adapter.ts's existing cursor_anomaly reason-string precedent).
 */
export function describeDiagnostics(label: string, diagnostics: readonly TransportDiagnostic[]): string {
  const parts = diagnostics.map((diagnostic) => {
    const segments = [diagnostic.operation];
    if (diagnostic.path !== undefined) segments.push(`path=${diagnostic.path}`);
    if (diagnostic.code !== undefined) segments.push(`code=${diagnostic.code}`);
    if (diagnostic.request !== undefined) segments.push(`request=${diagnostic.request}`);
    if (diagnostic.endpoint !== undefined) segments.push(`endpoint=${diagnostic.endpoint}`);
    if (diagnostic.complexity !== undefined) segments.push(`complexity=${diagnostic.complexity}`);
    if (diagnostic.reset !== undefined) segments.push(`reset=${diagnostic.reset}`);
    return segments.join(" ");
  });
  return `${label}: ${parts.join("; ")}`;
}

// ---------------------------------------------------------------------------
// Retry policy
// ---------------------------------------------------------------------------

/**
 * FailureKind enumerates the exact reasons a request attempt failed, as
 * already classified by the caller: "server_error" is a transient
 * 5xx-class transport failure; "partial" and "rate_limited" are the two
 * D-21 diagnoses normalizeGraphQLResult/normalizeRateLimit above produce.
 * There is no generic/unknown failure kind -- an unrecognized failure
 * never becomes retry-eligible by falling through a default case.
 */
export type FailureKind = "server_error" | "partial" | "rate_limited";

export interface RetryContext {
  action: OperationAction;
  kind: FailureKind;
  attempt: number;
  maxAttempts: number;
  diagnostic: TransportDiagnostic;
}

/**
 * DEFAULT_MAX_RETRY_ATTEMPTS bounds every retry-eligible read: a small,
 * fixed ceiling (CONTEXT D-21 "reads/5xx retry only within fixed
 * bounds"), never unbounded or exponential-forever.
 */
export const DEFAULT_MAX_RETRY_ATTEMPTS = 3;

/**
 * RetryDecision is this module's exhaustive retry outcome:
 * - "retry": bounded-eligible -- only ever a "read" operation observing a
 *   "server_error" failure below maxAttempts.
 * - "stop_write": a "create"/"update" mutation observed a "partial" or
 *   "rate_limited" diagnosis -- the mutation is blocked outright and never
 *   retried (CONTEXT D-21 "rate limits stop writes"; T-01-40).
 * - "stop": every other case (a read that exhausted its bound, a read
 *   observing a non-retryable diagnosis, or a mutation observing a
 *   server_error) -- fails closed without ever retrying.
 */
export type RetryDecision =
  | { outcome: "retry"; nextAttempt: number }
  | { outcome: "stop_write"; reason: string; diagnostic: TransportDiagnostic }
  | { outcome: "stop"; reason: string; diagnostic: TransportDiagnostic };

/**
 * decideRetry is the one retry policy this workspace ever applies
 * (CONTEXT D-21; T-01-40): a "create"/"update" mutation never retries at
 * all -- observing "partial" or "rate_limited" returns "stop_write"
 * immediately (mutations never flow through generic retry); observing
 * "server_error" falls through to the generic "stop" outcome below rather
 * than a bounded retry, because retrying a mutation risks a duplicate
 * write. Only a "read" observing a "server_error" failure may retry, and
 * only within maxAttempts.
 */
export function decideRetry(context: RetryContext): RetryDecision {
  const { action, kind, attempt, maxAttempts, diagnostic } = context;

  if (action !== "read" && (kind === "partial" || kind === "rate_limited")) {
    return {
      outcome: "stop_write",
      reason: `${action} mutation observed a ${kind} diagnosis for operation ${diagnostic.operation}; mutations never retry`,
      diagnostic,
    };
  }

  if (action === "read" && kind === "server_error" && attempt < maxAttempts) {
    return { outcome: "retry", nextAttempt: attempt + 1 };
  }

  return {
    outcome: "stop",
    reason: `${action} ${kind} is not retry-eligible (attempt ${attempt} of ${maxAttempts})`,
    diagnostic,
  };
}
