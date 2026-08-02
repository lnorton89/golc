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
import desktopViews from "./desktopViews.json";

describe("AppShell navigation", () => {
  afterEach(() => {
    cleanup();
  });

  const allDestinationLabels = NAV_GROUPS.flatMap((group) => group.destinations.map((destination) => destination.label));

  it("projects the complete, unique schema-v1 catalog into navigation", () => {
    const catalogGroups = desktopViews.groups as ReadonlyArray<{
      label: string;
      views: ReadonlyArray<{ id: string; slug: string; navLabel: string }>;
    }>;
    expect(desktopViews.schemaVersion).toBe(1);
    expect(catalogGroups.map((group) => group.label)).toEqual(["Show", "Build", "Operate", "Perform", "Output"]);

    const catalogViews = catalogGroups.flatMap((group) => group.views);
    expect(catalogViews).toHaveLength(15);
    expect(new Set(catalogViews.map((view) => view.id)).size).toBe(catalogViews.length);
    expect(new Set(catalogViews.map((view) => view.slug)).size).toBe(catalogViews.length);
    expect(
      NAV_GROUPS.flatMap((group) =>
        group.destinations.map((destination) => ({
          group: group.label,
          id: destination.id,
          label: destination.label,
        })),
      ),
    ).toEqual(
      catalogGroups.flatMap((group) =>
        group.views.map((view) => ({
          group: group.label,
          id: view.id,
          label: view.navLabel,
        })),
      ),
    );
  });

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
