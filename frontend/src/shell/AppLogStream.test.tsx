// AppLogStream.test.tsx proves this non-visual component is actually the
// store's "app:log" writer: mounting it subscribes to window.runtime's
// "app:log" EventsOn, each pushed AppLogView lands in useGolcStore's
// `appLog` slice, and unmounting unsubscribes (mirrors
// ScriptsWorkspace.test.tsx's stubRuntimeEvents convention for simulating
// a live push without a real Wails webview host). Also covers the
// regression this component exists to fix: App.RecentAppLogs() (the
// backlog fetch) seeds the store with lines that fired before this
// component's own live subscription registered, and a line present in
// both the backlog and the live stream is never duplicated.
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, waitFor } from "@testing-library/react";

import AppLogStream from "./AppLogStream";
import { useGolcStore } from "../store/store";

function stubAppBinding(recentAppLogs: unknown[]) {
  vi.stubGlobal("go", {
    wails: {
      App: {
        RecentAppLogs: vi.fn().mockResolvedValue(recentAppLogs),
      },
    },
  });
}

function stubRuntimeEvents() {
  const listeners: Array<(...data: unknown[]) => void> = [];
  const unsubscribeSpy = vi.fn();
  const runtime = {
    EventsOn: vi.fn((eventName: string, callback: (...data: unknown[]) => void) => {
      if (eventName === "app:log") {
        listeners.push(callback);
      }
      return unsubscribeSpy;
    }),
  };
  vi.stubGlobal("runtime", runtime);
  return {
    emitAppLog: (event: Partial<Record<string, unknown>>) => {
      listeners.forEach((callback) => callback(event));
    },
    unsubscribeSpy,
  };
}

describe("AppLogStream", () => {
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
    useGolcStore.getState().clearAppLog();
  });

  it("writes each pushed app:log event into the store's appLog slice", () => {
    const { emitAppLog } = stubRuntimeEvents();
    render(<AppLogStream />);

    emitAppLog({ seq: 1, level: "info", source: "daemon", message: "daemon reachable" });
    emitAppLog({ seq: 2, level: "warn", source: "hotkeys", message: "control=blackout error=taken" });

    expect(useGolcStore.getState().appLog).toEqual([
      { seq: 1, level: "info", source: "daemon", message: "daemon reachable" },
      { seq: 2, level: "warn", source: "hotkeys", message: "control=blackout error=taken" },
    ]);
  });

  it("unsubscribes on unmount", () => {
    const { unsubscribeSpy } = stubRuntimeEvents();
    const { unmount } = render(<AppLogStream />);

    expect(unsubscribeSpy).not.toHaveBeenCalled();
    unmount();
    expect(unsubscribeSpy).toHaveBeenCalledTimes(1);
  });

  it("renders nothing", () => {
    stubRuntimeEvents();
    const { container } = render(<AppLogStream />);
    expect(container).toBeEmptyDOMElement();
  });

  it("seeds the store from App.RecentAppLogs on mount -- the backlog fetch this component exists to run", async () => {
    stubRuntimeEvents();
    stubAppBinding([
      { seq: 1, level: "info", source: "daemon", message: "GOLC_WAILS_DAEMON_REACHABLE: daemon already reachable" },
      { seq: 2, level: "warn", source: "hotkeys", message: "control=blackout error=taken" },
    ]);

    render(<AppLogStream />);

    await waitFor(() =>
      expect(useGolcStore.getState().appLog).toEqual([
        { seq: 1, level: "info", source: "daemon", message: "GOLC_WAILS_DAEMON_REACHABLE: daemon already reachable" },
        { seq: 2, level: "warn", source: "hotkeys", message: "control=blackout error=taken" },
      ]),
    );
  });

  it("never duplicates a line present in both the live stream and the backlog fetch", async () => {
    const { emitAppLog } = stubRuntimeEvents();
    stubAppBinding([{ seq: 1, level: "info", source: "daemon", message: "daemon reachable" }]);

    render(<AppLogStream />);
    // Simulates a line arriving live in the brief window before the
    // backlog fetch's Promise resolves -- both carry the same seq.
    emitAppLog({ seq: 1, level: "info", source: "daemon", message: "daemon reachable" });

    await waitFor(() => expect(useGolcStore.getState().appLog).toHaveLength(1));
    expect(useGolcStore.getState().appLog).toEqual([
      { seq: 1, level: "info", source: "daemon", message: "daemon reachable" },
    ]);
  });
});
