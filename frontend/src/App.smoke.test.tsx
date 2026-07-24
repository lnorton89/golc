// App.smoke.test.tsx is a build-time gate, not a feature test: it mounts
// the real <App/> component tree in jsdom and fails if anything throws or
// logs a console error/warning during import or render.
//
// This exists because tsc --noEmit and `vite build` both only catch
// type/syntax errors -- neither one ever actually EXECUTES the app, so a
// runtime-only bug (e.g. a circular ES module import leaving a top-level
// const still undefined at the exact moment a sibling module dereferences
// it) compiles and bundles cleanly and then crashes the instant a real
// browser/webview loads it. That exact bug shipped through Phase 6 Waves
// 3-4 undetected because every visual PLAY-01/02 check was deferred to
// end-of-phase UAT, and nothing exercised the actual bundle before then.
// `npm run build` now runs this FIRST -- the desktop app cannot build if
// mounting the app logs a console error or throws.
//
// window.go (the Wails runtime bridge) does not exist in jsdom, which is
// intentional: every component already has a documented "bridge
// unavailable" degraded-render path (see wailsBridge.ts) for exactly this
// case, so this smoke test doubly verifies that degraded path never
// itself throws or logs an error.
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render } from "@testing-library/react";

import App from "./App";

describe("App smoke test", () => {
  afterEach(() => {
    cleanup();
  });

  it("mounts without throwing or logging a console error", () => {
    const errors: unknown[][] = [];
    const errorSpy = vi.spyOn(console, "error").mockImplementation((...args: unknown[]) => {
      errors.push(args);
    });

    const windowErrors: unknown[] = [];
    const onWindowError = (event: ErrorEvent) => windowErrors.push(event.error ?? event.message);
    const onUnhandledRejection = (event: PromiseRejectionEvent) => windowErrors.push(event.reason);
    window.addEventListener("error", onWindowError);
    window.addEventListener("unhandledrejection", onUnhandledRejection);

    try {
      expect(() => render(<App />)).not.toThrow();
    } finally {
      window.removeEventListener("error", onWindowError);
      window.removeEventListener("unhandledrejection", onUnhandledRejection);
      errorSpy.mockRestore();
    }

    expect(errors).toEqual([]);
    expect(windowErrors).toEqual([]);
  });
});
