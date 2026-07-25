// DiagnosticsWorkspace.test.tsx exercises the workspace end-to-end against
// a mocked window.go.wails.ShowService, the same direct-window-object
// convention every other Wails-bridge test in this codebase uses (see
// wailsBridge.ts's own doc comment; mirrors OverviewWorkspace.test.tsx's
// identical shape).
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";

import DiagnosticsWorkspace from "./DiagnosticsWorkspace";

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

describe("DiagnosticsWorkspace", () => {
  beforeEach(() => {
    vi.stubGlobal("go", {
      wails: {
        ShowService: {
          Diagnose: vi.fn().mockResolvedValue(diagnosticReport()),
        },
      },
    });
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it("runs diagnostics automatically on mount and shows a healthy result", async () => {
    render(<DiagnosticsWorkspace />);
    await waitFor(() => expect(screen.getByText("Healthy")).toBeInTheDocument());
    expect(screen.getByText("Schema 3 · Revision 7")).toBeInTheDocument();
    expect(screen.getByText("No file-level integrity issues found.")).toBeInTheDocument();
  });

  it("surfaces file-level issues, a structural error, and a migration-required note", async () => {
    const svc = (window as unknown as { go: { wails: { ShowService: Record<string, ReturnType<typeof vi.fn>> } } })
      .go.wails.ShowService;
    svc.Diagnose.mockResolvedValue(
      diagnosticReport({
        fileLevelIssues: ["*** in database main ***\nPage 12 is never used"],
        structuralOk: false,
        structuralError: "GOLC_SHOW_MIGRATION_REQUIRED: schema_version 2 requires migration to 3",
        migrationRequired: true,
      }),
    );

    render(<DiagnosticsWorkspace />);
    await waitFor(() => expect(screen.getByText("Issues found")).toBeInTheDocument());
    expect(screen.getByText("Migration required")).toBeInTheDocument();
    expect(screen.getByText(/GOLC_SHOW_MIGRATION_REQUIRED/)).toBeInTheDocument();
    expect(screen.getByText(/Page 12 is never used/)).toBeInTheDocument();
    expect(screen.getByText(/confirm-migration/)).toBeInTheDocument();
  });

  it("re-runs diagnostics via the Re-run button", async () => {
    const svc = (window as unknown as { go: { wails: { ShowService: Record<string, ReturnType<typeof vi.fn>> } } })
      .go.wails.ShowService;
    render(<DiagnosticsWorkspace />);
    await waitFor(() => expect(screen.getByText("Healthy")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: "Re-run" }));
    await waitFor(() => expect(svc.Diagnose).toHaveBeenCalledTimes(2));
  });
});
