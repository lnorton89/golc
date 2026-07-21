// redact.ts is this workspace's canary-scan and safe-uncertain-outcome
// contract (Plan 01-27; CONTEXT D-19/D-20; T-01-41) -- the TypeScript-side
// sibling of internal/security/redact.go. adapter.ts's createOperation/
// updateOperation route every raw create/update failure (a network
// timeout, an aborted request, a partial GraphQL error mid-mutation, or any
// other exception the official @linear/sdk or this adapter's own mandatory
// readback throws) through safeError before it can ever reach a returned
// MutationOutcome (protocol.ts), cli.ts's NDJSON output line, or process
// stderr. safeError never reads a raw exception's message, stack, cause
// content, or any attached property (a client instance, environment map,
// request body, or header set): only its constructor-assigned `.name` -- a
// small, fixed, safe classification the JS runtime or SDK itself assigns,
// never attacker- or credential-influenced free text -- ever informs the
// returned diagnostic's `code`, which is always one of exactly three fixed
// string constants. scanCanary/scanCanaryAll are this module's own offline
// proof mechanism, byte-for-byte matching internal/security/redact.go's
// CanaryToken and forbidden-pattern list, so redact.test.ts (and
// mutation.test.ts) can plant a unique fake secret inside a hostile raw
// error and prove it never survives safeError's normalization.

import type { RequestContext } from "./errors.js";
import type { TransportDiagnostic } from "./protocol.js";

/**
 * CANARY_TOKEN mirrors internal/security/redact.go's CanaryToken exactly
 * (byte-for-byte identical value) -- a unique, obviously-fake secret-shaped
 * byte sequence this module's own test suite plants into a controlled raw
 * error to prove scanCanary actually detects leaked secret material, never
 * only observing already-clean output. It must never appear in any real
 * committed or generated repository artifact.
 */
export const CANARY_TOKEN = "GOLC_FAKE_SECRET_CANARY_4f9c2e6b1a7d3f809c21";

/**
 * FORBIDDEN_PATTERNS mirrors internal/security/redact.go's
 * forbiddenPatterns exactly: the exact CANARY_TOKEN plus the fixed set of
 * common secret-shaped substrings (a raw Linear credential declaration, a
 * bearer auth header, and API-key prefixes) that must never appear in
 * emitted diagnostics or committed/generated artifacts, independent of
 * whether CANARY_TOKEN itself was ever planted.
 */
const FORBIDDEN_PATTERNS: readonly string[] = [CANARY_TOKEN, "LINEAR_API_KEY=", "Bearer ", "sk-", "lin_api_"];

/**
 * scanCanary reports the first forbidden token found in data, or undefined
 * when data is clean -- the exact TypeScript-side mirror of
 * internal/security/redact.go's ScanCanary. It operates on the full text of
 * whatever surface a caller passes (a rendered diagnostic, an NDJSON output
 * line, a committed fixture's own JSON), never a special-cased subset.
 */
export function scanCanary(data: string): string | undefined {
  for (const token of FORBIDDEN_PATTERNS) {
    if (data.includes(token)) {
      return token;
    }
  }
  return undefined;
}

/**
 * CanaryViolation names one output surface that leaked a forbidden token,
 * mirroring internal/security/redact.go's CanaryViolation.
 */
export interface CanaryViolation {
  source: string;
  token: string;
}

/**
 * scanCanaryAll scans every named source in sources and returns every
 * violation, sorted by source name for deterministic reporting -- the exact
 * TypeScript-side mirror of internal/security/redact.go's ScanCanaryAll. An
 * empty array means every source is clean.
 */
export function scanCanaryAll(sources: Readonly<Record<string, string>>): CanaryViolation[] {
  const names = Object.keys(sources).sort();
  const violations: CanaryViolation[] = [];
  for (const name of names) {
    const token = scanCanary(sources[name] ?? "");
    if (token !== undefined) {
      violations.push({ source: name, token });
    }
  }
  return violations;
}

// ---------------------------------------------------------------------------
// safeError -- the sole producer of an "unknown" MutationOutcome's
// diagnostic (adapter.ts, Plan 01-27).
// ---------------------------------------------------------------------------

/**
 * MUTATION_FAILURE_CODES is the exhaustive, fixed set of safe diagnostic
 * codes safeError may ever assign -- never a raw exception's own message or
 * any other free-text content (CONTEXT D-20's "secret values must never
 * appear in previews, logs, errors" extended here to every mutation
 * failure, not only credentials).
 */
const MUTATION_FAILURE_CODES = {
  timeout: "LINEAR_MUTATION_TIMEOUT",
  network: "LINEAR_MUTATION_NETWORK_ERROR",
  unknown: "LINEAR_MUTATION_UNCERTAIN",
} as const;

/**
 * hasStructuredCause reports whether error carries an object-shaped
 * `.cause` property (the shape Node's own fetch-style network failures
 * use) -- inspected only for its type, never its content, so this
 * classification step itself can never read or forward a raw value.
 */
function hasStructuredCause(error: Error): boolean {
  const cause = (error as { cause?: unknown }).cause;
  return typeof cause === "object" && cause !== null;
}

/**
 * classifyMutationFailure maps a raw, untrusted create/update failure to
 * one of MUTATION_FAILURE_CODES's three safe codes. It inspects only an
 * Error's constructor-assigned `.name` ("AbortError"/"TimeoutError" for a
 * cancelled/timed-out request, "TypeError" with a structured `.cause` for a
 * lower-level network failure) and never reads `.message`, `.stack`, or any
 * other property -- a hostile or malformed raw error can never smuggle
 * arbitrary content into the returned code by construction, since the
 * return value is always one of exactly three fixed string constants.
 */
function classifyMutationFailure(error: unknown): string {
  if (error instanceof Error) {
    if (error.name === "AbortError" || error.name === "TimeoutError") {
      return MUTATION_FAILURE_CODES.timeout;
    }
    if (error.name === "TypeError" && hasStructuredCause(error)) {
      return MUTATION_FAILURE_CODES.network;
    }
  }
  return MUTATION_FAILURE_CODES.unknown;
}

/**
 * safeError converts one raw, untrusted create/update failure (any
 * exception thrown by the official @linear/sdk's mutation call, or by this
 * adapter's own mandatory readback immediately following it) into the
 * exact allowlisted TransportDiagnostic surface protocol.ts declares:
 * operation/endpoint come only from the caller's own RequestContext (never
 * the error itself), and code is always one of classifyMutationFailure's
 * three fixed constants. No raw exception message, stack trace, GraphQL
 * query/variables, header value, credential, client instance, environment
 * map, or request body ever reaches the returned diagnostic -- this is the
 * sole producer of an "unknown" MutationOutcome's diagnostic (adapter.ts).
 */
export function safeError(error: unknown, context: RequestContext): TransportDiagnostic {
  const diagnostic: TransportDiagnostic = { operation: context.operation, code: classifyMutationFailure(error) };
  if (context.endpoint !== undefined) {
    diagnostic.endpoint = context.endpoint;
  }
  return diagnostic;
}
