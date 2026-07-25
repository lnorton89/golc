// AppShell.navigation.test.tsx is the direct regression guard for the
// shell restructure itself (the flat unnavigated App.tsx stack -> grouped
// Show/Build/Operate/Output shell): clicks through all 8 real nav
// destinations plus the 4 stub destinations and asserts each mounts
// without throwing or logging a console error/warning, matching
// App.smoke.test.tsx's own "mounts cleanly with no window.go bridge"
// convention (jsdom has no window.go, so every workspace's degraded-
// render path is exercised here too).
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";

import AppShell from "./AppShell";
import { NAV_GROUPS } from "./navigation";

describe("AppShell navigation", () => {
  afterEach(() => {
    cleanup();
  });

  const allDestinationLabels = NAV_GROUPS.flatMap((group) => group.destinations.map((destination) => destination.label));

  it.each(allDestinationLabels)("mounts %s without throwing or logging a console error", (label) => {
    const errors: unknown[][] = [];
    const errorSpy = vi.spyOn(console, "error").mockImplementation((...args: unknown[]) => {
      errors.push(args);
    });
    const warnings: unknown[][] = [];
    const warnSpy = vi.spyOn(console, "warn").mockImplementation((...args: unknown[]) => {
      warnings.push(args);
    });

    const windowErrors: unknown[] = [];
    const onWindowError = (event: ErrorEvent) => windowErrors.push(event.error ?? event.message);
    const onUnhandledRejection = (event: PromiseRejectionEvent) => windowErrors.push(event.reason);
    window.addEventListener("error", onWindowError);
    window.addEventListener("unhandledrejection", onUnhandledRejection);

    try {
      render(<AppShell />);
      const navButton = screen.getByRole("button", { name: label });
      expect(() => fireEvent.click(navButton)).not.toThrow();
      // Every destination renders a workspace heading matching its own nav
      // label -- confirms navigation actually replaced the workspace,
      // not just that the click handler ran without throwing.
      expect(screen.getByRole("heading", { name: label })).toBeInTheDocument();
    } finally {
      window.removeEventListener("error", onWindowError);
      window.removeEventListener("unhandledrejection", onUnhandledRejection);
      errorSpy.mockRestore();
      warnSpy.mockRestore();
    }

    expect(errors).toEqual([]);
    expect(warnings).toEqual([]);
    expect(windowErrors).toEqual([]);
  });
});
