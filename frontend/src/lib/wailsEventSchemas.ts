// wailsEventSchemas.ts validates the payloads the Go host pushes over
// runtime.EventsOn before wailsBridge.ts hands them to a subscriber.
//
// Every other call in wailsBridge.ts is a request/response the caller
// initiated and whose shape TypeScript checked at the call site. The four
// event subscriptions are different: their payloads arrive as `unknown[]`
// from outside the type system entirely, and were previously asserted with
// a bare `data[0] as StatusSnapshot | undefined`. A cast is not a check --
// if internal/wails/events.go renamed a field or changed a type, the cast
// still succeeded and the wrong shape travelled onward until something far
// away dereferenced a property that was not there. The crash then surfaced
// in a component with no obvious connection to the event that caused it.
//
// Parsing at the boundary converts that class of failure into a named,
// located error at the moment the bad payload arrives -- which is exactly
// what App.smoke.test.tsx's console-error build gate exists to catch.
//
// Two deliberate choices about strictness:
//
//   - Unknown keys are STRIPPED, not rejected. Go adding a field to an
//     event struct is a routine, backward-compatible change and must not
//     break a running frontend. Only a missing or wrongly-typed field that
//     this side actually reads is a real contract break.
//
//   - Optional fields stay optional, mirroring the interfaces exactly. Go's
//     `omitempty` tags mean these genuinely are absent on the wire rather
//     than present-and-empty, so requiring them would reject valid traffic.
//
// The `satisfies z.ZodType<...>` on each schema is load-bearing: it makes
// tsc fail if a schema and the interface it validates ever drift apart, so
// the two cannot silently disagree about what a payload looks like.
//
// The type import is deliberately `import type`. It is erased at compile
// time, so wailsBridge.ts importing this module at runtime while this
// module names its types creates no import cycle -- the exact class of bug
// App.smoke.test.tsx was added to catch.
import { z } from "zod";

import type { AppLogView, MidiFeedback, ScriptEventView, StatusSnapshot } from "./wailsBridge";

export const statusSnapshotSchema = z.object({
  reachable: z.boolean(),
  active: z.boolean(),
  sceneId: z.string().optional(),
  sceneName: z.string().optional(),
  bpm: z.number(),
  barIndex: z.number(),
  beatFraction: z.number(),
  enabledLayers: z.array(z.string()),
  controllingSource: z.string(),
  outputState: z.string(),
}) satisfies z.ZodType<StatusSnapshot>;

export const midiFeedbackSchema = z.object({
  scope: z.enum(["surface", "desk"]),
  surfaceName: z.string(),
  mappingId: z.string(),
  kind: z.string(),
  armed: z.boolean(),
  appValue: z.number(),
  physical: z.number(),
}) satisfies z.ZodType<MidiFeedback>;

export const scriptEventSchema = z.object({
  seq: z.number(),
  kind: z.string(),
  runId: z.string().optional(),
  scriptName: z.string().optional(),
  at: z.string().optional(),
  level: z.string().optional(),
  message: z.string().optional(),
  source: z.string().optional(),
  method: z.string().optional(),
  route: z.string().optional(),
  durationMs: z.number().optional(),
  ok: z.boolean().optional(),
  code: z.string().optional(),
  status: z.string().optional(),
  reason: z.string().optional(),
  gapCount: z.number().optional(),
}) satisfies z.ZodType<ScriptEventView>;

export const appLogSchema = z.object({
  seq: z.number(),
  level: z.string(),
  source: z.string().optional(),
  message: z.string().optional(),
  at: z.string().optional(),
  gapCount: z.number().optional(),
}) satisfies z.ZodType<AppLogView>;

/** parseEventPayload validates one pushed event payload.
 *
 * Returns the parsed value, or undefined when the payload is absent or
 * fails validation -- never throws. That is not incidental: these run
 * inside a Wails event callback, where a thrown error has no caller to
 * catch it and would surface as an unhandled exception in the webview
 * rather than as anything actionable. Dropping the event keeps the
 * subscriber's own contract intact (it is only ever handed a valid
 * payload), and the console.error is what makes the drop visible instead
 * of silent.
 *
 * The event name is included in the message because all four
 * subscriptions funnel through here -- without it, a diagnosis starts by
 * working out which stream misbehaved. */
export function parseEventPayload<T>(schema: z.ZodType<T>, eventName: string, payload: unknown): T | undefined {
  if (payload === undefined || payload === null) return undefined;

  const result = schema.safeParse(payload);
  if (result.success) return result.data;

  console.error(
    `GOLC_WAILS_EVENT_PAYLOAD_INVALID: "${eventName}" payload did not match the expected shape -- dropping it. ` +
      `This means internal/wails/events.go and wailsEventSchemas.ts disagree. ${z.prettifyError(result.error)}`,
  );
  return undefined;
}
