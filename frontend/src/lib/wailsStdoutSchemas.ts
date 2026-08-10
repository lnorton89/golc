// wailsStdoutSchemas.ts validates the JSON payloads that arrive as a
// *string* in WailsResult.stdout, rather than as a typed binding return
// value.
//
// wailsEventSchemas.ts already covers the push side (runtime.EventsOn
// payloads). This module covers the other hole the type system cannot see
// into: a handful of routes answer with a raw canonical encoding in
// stdout, and every call site parsed it with `JSON.parse(result.stdout) as
// <Shape>`. A cast is not a check. A patch impact plan missing `plan_id`
// sailed through into `applyPatch(undefined as unknown as string)`, and one
// missing `proposed_universe`/`proposed_address` rendered as the literal
// string "Universe undefined, Address undefined" on the review screen the
// operator is supposed to approve before committing a structural edit.
//
// Same two strictness choices wailsEventSchemas.ts documents and for the
// same reasons: unknown keys are STRIPPED (Go adding a field is routine and
// backward-compatible), and optional fields stay optional (Go's `omitempty`
// means they genuinely are absent on the wire).
//
// Where the interfaces used to be duplicated per component -- FixturePatch
// and ProjectFixtures each declared their own slightly different ImpactPlan
// -- the schema is now the single declaration and the type is inferred from
// it, so a schema and its type cannot drift apart at all.
import { z } from "zod";

import type { PlaybackStateSummary } from "./playbackDispatch";

/** ImpactOperation mirrors internal/pool/impact.go's snake_case json tags.
 * AddPoolMemberPreview/RemovePoolMemberPreview return the plan's raw
 * canonical encoding verbatim, never re-cased through the camelCase
 * convention the rest of the bridge uses. */
export const impactOperationSchema = z.object({
  dependent_kind: z.string(),
  dependent_ref: z.string(),
  dependent_id: z.string(),
  action: z.string(),
  pool_member_index: z.number().optional(),
  pool_member_id: z.string().optional(),
  proposed_universe: z.number().optional(),
  proposed_address: z.number().optional(),
  status: z.string(),
});

const impactDiagnosticSchema = z.object({
  code: z.string(),
  message: z.string(),
});

export const impactPlanSchema = z.object({
  schema_version: z.number(),
  pool_id: z.string(),
  add: z
    .array(
      z.object({
        fixture_stable_key: z.string(),
        fixture_content_hash: z.string(),
        mode: z.string(),
      }),
    )
    .optional(),
  remove: z.array(z.string()).optional(),
  propagate: z.string().optional(),
  expected_revision: z.number().optional(),
  // impact.go's Operations field carries no `omitempty` and is left as a
  // nil slice when no deployment references the pool yet, which
  // encoding/json marshals as JSON null -- not []. Nullable, and every
  // read of it still goes through `?? []`.
  operations: z.array(impactOperationSchema).nullable(),
  warnings: z.array(impactDiagnosticSchema).optional(),
  errors: z.array(impactDiagnosticSchema).optional(),
  // The one field an unvalidated cast most needed to guarantee: this is
  // what gets handed to applyPatch to commit the edit.
  plan_id: z.string().min(1),
});

export type ImpactOperation = z.infer<typeof impactOperationSchema>;
export type ImpactPlan = z.infer<typeof impactPlanSchema>;

const layerSummarySchema = z.object({
  kind: z.enum(["base_look", "color_theme", "chase", "motion"]),
  enabled: z.boolean(),
  ref: z.string().optional(),
});

const sceneSummarySchema = z.object({
  name: z.string(),
  active: z.boolean(),
  barsPerLoop: z.number(),
  layers: z.array(layerSummarySchema),
});

export const playbackStateSummarySchema = z.object({
  bpm: z.number(),
  scenes: z.array(sceneSummarySchema),
}) satisfies z.ZodType<PlaybackStateSummary>;

/** parseStdoutJson validates one stdout-carried payload, THROWING a named
 * diagnostic when it does not match.
 *
 * Throwing (rather than returning undefined, as parseEventPayload does) is
 * deliberate and matches how these call sites already work: each one runs
 * inside a try/catch that renders the caught message in its own error
 * banner, so a malformed plan now fails loudly at the boundary instead of
 * travelling onward as a plausible-looking object. The diagnostic is
 * prefixed like every other GOLC_* code so it reads the same as a real
 * backend rejection in that banner. */
export function parseStdoutJson<T>(schema: z.ZodType<T>, label: string, stdout: string): T {
  let raw: unknown;
  try {
    raw = JSON.parse(stdout);
  } catch {
    throw new Error(`GOLC_WAILS_STDOUT_INVALID: ${label} did not return valid JSON.`);
  }

  const result = schema.safeParse(raw);
  if (!result.success) {
    throw new Error(
      `GOLC_WAILS_STDOUT_INVALID: ${label} did not match the expected shape. ${z.prettifyError(result.error)}`,
    );
  }
  return result.data;
}

/** parseStdoutJsonOrUndefined is the non-throwing variant for the one
 * caller whose documented contract is "answers undefined for every failure
 * mode" (playbackDispatch.getState -- usePlaybackStateSnapshot turns that
 * into "keep the last known state" rather than blanking the readout). The
 * console.error keeps the drop visible instead of silent, exactly as
 * parseEventPayload does. */
export function parseStdoutJsonOrUndefined<T>(
  schema: z.ZodType<T>,
  label: string,
  stdout: string,
): T | undefined {
  try {
    return parseStdoutJson(schema, label, stdout);
  } catch (err) {
    console.error(err instanceof Error ? err.message : String(err));
    return undefined;
  }
}
