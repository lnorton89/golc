// OverviewWorkspace.test.tsx exercises the workspace end-to-end against a
// mocked window.go.wails.ShowService, the same direct-window-object
// convention every other Wails-bridge test in this codebase uses (see
// wailsBridge.ts's own doc comment; mirrors ScenesLooksWorkspace.test.tsx's
// identical shape).
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";

import OverviewWorkspace from "./OverviewWorkspace";

function inspectView(overrides: Partial<Record<string, unknown>> = {}) {
  return {
    showPath: "C:\\shows\\demo.golc",
    schemaVersion: 3,
    revision: 7,
    pools: [{ id: "pool-1", name: "Wash", requiredCapabilities: [], memberCount: 2 }],
    deployments: [{ id: "deploy-1", name: "Main Rig", active: true, instanceCount: 2 }],
    ...overrides,
  };
}

function diagnosticReport(overrides: Partial<Record<string, unknown>> = {}) {
  return {
    fileLevelIssues: [],
    structuralOk: true,
    migrationRequired: false,
    schemaVersion: 3,
    revision: 7,
    ...overrides,
  };
}

describe("OverviewWorkspace", () => {
  beforeEach(() => {
    vi.stubGlobal("go", {
      wails: {
        ShowService: {
          Inspect: vi.fn().mockResolvedValue(inspectView()),
          Diagnose: vi.fn().mockResolvedValue(diagnosticReport()),
        },
      },
    });
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it("loads and displays the show's identity, pools, and deployments", async () => {
    render(<OverviewWorkspace />);
    await waitFor(() => expect(screen.getByText("C:\\shows\\demo.golc")).toBeInTheDocument());
    expect(screen.getByText("Schema 3 · Revision 7")).toBeInTheDocument();
    expect(screen.getByText("Wash")).toBeInTheDocument();
    expect(screen.getByText("Main Rig")).toBeInTheDocument();
    expect(screen.getByText("Active")).toBeInTheDocument();
  });

  it("shows empty states when there are no pools or deployments yet", async () => {
    const svc = (window as unknown as { go: { wails: { ShowService: Record<string, ReturnType<typeof vi.fn>> } } })
      .go.wails.ShowService;
    svc.Inspect.mockResolvedValue(inspectView({ pools: [], deployments: [] }));

    render(<OverviewWorkspace />);
    await waitFor(() => expect(screen.getByText("No fixture pools yet.")).toBeInTheDocument());
    expect(screen.getByText("No deployments yet.")).toBeInTheDocument();
  });

  it("runs Diagnose and renders a healthy result", async () => {
    render(<OverviewWorkspace />);
    await waitFor(() => expect(screen.getByText("C:\\shows\\demo.golc")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: "Diagnose" }));
    await waitFor(() => expect(screen.getByText("Healthy")).toBeInTheDocument());
  });

  it("surfaces structural issues and a migration-required note", async () => {
    const svc = (window as unknown as { go: { wails: { ShowService: Record<string, ReturnType<typeof vi.fn>> } } })
      .go.wails.ShowService;
    svc.Diagnose.mockResolvedValue(
      diagnosticReport({
        structuralOk: false,
        structuralError: "GOLC_SHOW_MIGRATION_REQUIRED: schema_version 2 requires migration to 3",
        migrationRequired: true,
      }),
    );

    render(<OverviewWorkspace />);
    await waitFor(() => expect(screen.getByText("C:\\shows\\demo.golc")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: "Diagnose" }));
    await waitFor(() => expect(screen.getByText("Issues found")).toBeInTheDocument());
    expect(screen.getByText("Migration required")).toBeInTheDocument();
    expect(screen.getByText(/GOLC_SHOW_MIGRATION_REQUIRED/)).toBeInTheDocument();
  });
});
