// Exercises the event-payload validation through the real subscription
// exports (onAppLog / onStatusUpdate / onMidiFeedback / onScriptEvent)
// against a stubbed window.runtime, rather than calling the schemas
// directly. Parsing the right payload with the right schema is half the
// contract; the other half is that each subscription is actually wired to
// its own schema, and only an end-to-end assertion catches a copy-paste
// that validates "app:log" against the status snapshot.
import { afterEach, describe, expect, it, vi } from "vitest";

import { onAppLog, onMidiFeedback, onScriptEvent, onStatusUpdate } from "./wailsBridge";

type Handler = (...data: unknown[]) => void;

/** installRuntime stubs window.runtime.EventsOn and hands back a way to
 * fire a payload at whichever event name was subscribed. */
function installRuntime() {
  const handlers = new Map<string, Handler>();
  (window as unknown as { runtime: unknown }).runtime = {
    EventsOn: (name: string, handler: Handler) => {
      handlers.set(name, handler);
      return () => handlers.delete(name);
    },
  };
  return {
    emit(name: string, payload: unknown) {
      const handler = handlers.get(name);
      if (!handler) throw new Error(`no subscriber for ${name}`);
      handler(payload);
    },
  };
}

const validSnapshot = {
  reachable: true,
  active: false,
  bpm: 120,
  barIndex: 0,
  beatFraction: 0,
  enabledLayers: [],
  controllingSource: "live",
  outputState: "live",
};

const validAppLog = { seq: 1, level: "info" };

afterEach(() => {
  delete (window as unknown as { runtime?: unknown }).runtime;
  vi.restoreAllMocks();
});

describe("Wails event payload validation", () => {
  it("passes a valid payload through to the subscriber", () => {
    const runtime = installRuntime();
    const received: unknown[] = [];
    onStatusUpdate((snapshot) => received.push(snapshot));

    runtime.emit("status:update", validSnapshot);

    expect(received).toEqual([validSnapshot]);
  });

  it("drops a payload whose required field has the wrong type, and says which stream", () => {
    const runtime = installRuntime();
    const errors: string[] = [];
    vi.spyOn(console, "error").mockImplementation((...args: unknown[]) => {
      errors.push(String(args[0]));
    });
    const received: unknown[] = [];
    onStatusUpdate((snapshot) => received.push(snapshot));

    // bpm arrives as a string -- exactly what a Go-side type change would
    // look like on the wire, and precisely what the old `as StatusSnapshot`
    // cast waved through.
    runtime.emit("status:update", { ...validSnapshot, bpm: "120" });

    expect(received).toEqual([]);
    expect(errors).toHaveLength(1);
    expect(errors[0]).toContain("GOLC_WAILS_EVENT_PAYLOAD_INVALID");
    expect(errors[0]).toContain("status:update");
  });

  it("drops a payload missing a required field", () => {
    const runtime = installRuntime();
    vi.spyOn(console, "error").mockImplementation(() => {});
    const received: unknown[] = [];
    onAppLog((event) => received.push(event));

    const { level: _dropped, ...withoutLevel } = validAppLog;
    runtime.emit("app:log", withoutLevel);

    expect(received).toEqual([]);
  });

  it("accepts an unknown extra field, so an additive Go change stays compatible", () => {
    const runtime = installRuntime();
    const errorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    const received: { seq: number }[] = [];
    onAppLog((event) => received.push(event));

    runtime.emit("app:log", { ...validAppLog, somethingGoAddedLater: "value" });

    expect(errorSpy).not.toHaveBeenCalled();
    expect(received).toHaveLength(1);
    expect(received[0].seq).toBe(1);
    // Stripped rather than carried through, so the parsed value matches the
    // declared interface exactly.
    expect(received[0]).not.toHaveProperty("somethingGoAddedLater");
  });

  it("accepts a payload with its optional fields absent", () => {
    const runtime = installRuntime();
    const errorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    const received: unknown[] = [];
    onScriptEvent((event) => received.push(event));

    runtime.emit("script:event", { seq: 7, kind: "script.log" });

    expect(errorSpy).not.toHaveBeenCalled();
    expect(received).toEqual([{ seq: 7, kind: "script.log" }]);
  });

  it("ignores an absent payload without logging", () => {
    const runtime = installRuntime();
    const errorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    const received: unknown[] = [];
    onAppLog((event) => received.push(event));

    runtime.emit("app:log", undefined);

    expect(errorSpy).not.toHaveBeenCalled();
    expect(received).toEqual([]);
  });

  it("validates each subscription against its own schema", () => {
    const runtime = installRuntime();
    const errors: string[] = [];
    vi.spyOn(console, "error").mockImplementation((...args: unknown[]) => {
      errors.push(String(args[0]));
    });
    const received: unknown[] = [];
    onMidiFeedback((feedback) => received.push(feedback));

    // A well-formed status snapshot is NOT a valid midi:feedback payload.
    // If every subscription shared one schema, this would pass unnoticed.
    runtime.emit("midi:feedback", validSnapshot);

    expect(received).toEqual([]);
    expect(errors[0]).toContain("midi:feedback");
  });

  it("rejects a value outside a closed union", () => {
    const runtime = installRuntime();
    vi.spyOn(console, "error").mockImplementation(() => {});
    const received: unknown[] = [];
    onMidiFeedback((feedback) => received.push(feedback));

    runtime.emit("midi:feedback", {
      scope: "somethingElse",
      surfaceName: "Main",
      mappingId: "m1",
      kind: "cc",
      armed: true,
      appValue: 0,
      physical: 0,
    });

    expect(received).toEqual([]);
  });
});
