// DiagnosticsWorkspace.test.tsx exercises the workspace end-to-end against
// a mocked window.go.wails.ShowService, the same direct-window-object
// convention every other Wails-bridge test in this codebase uses (see
// wailsBridge.ts's own doc comment; mirrors OverviewWorkspace.test.tsx's
// identical shape).
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";

import DiagnosticsWorkspace from "./DiagnosticsWorkspace";
import { useGolcStore } from "../../store/store";

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
    // The `appLog` slice lives in the module-singleton zustand store
    // (AppLogStream.tsx is its real writer, mounted unconditionally
    // outside this workspace -- see DiagnosticsWorkspace.tsx's own doc
    // comment) rather than this component's own state, so it survives
    // across tests in this file unless explicitly reset here.
    useGolcStore.getState().clearAppLog();
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

  it("renders app:log lines already present in the store before this workspace ever mounted", async () => {
    // Simulates the real startup sequence: AppLogStream.tsx (mounted
    // unconditionally in GlobalFrame, not here) writes to the store as
    // soon as App.OnStartup's lines arrive -- often before an operator
    // has navigated to Diagnostics at all. Pre-seeding the store, then
    // mounting this workspace afterward, is what would have caught the
    // "nothing appears in the log" regression a workspace-scoped
    // subscription produced.
    act(() => {
      useGolcStore.getState().appendAppLog({
        seq: 1,
        level: "info",
        source: "daemon",
        message: "GOLC_WAILS_DAEMON_REACHABLE: daemon already reachable",
        at: "2026-07-25T12:00:00Z",
      });
    });

    render(<DiagnosticsWorkspace />);
    await waitFor(() => expect(screen.getByText("Healthy")).toBeInTheDocument());

    expect(
      within(screen.getByRole("list", { name: "Application log" })).getByText(
        "GOLC_WAILS_DAEMON_REACHABLE: daemon already reachable",
      ),
    ).toBeInTheDocument();
  });

  it("accumulates further app:log lines appended to the store while mounted, and Clear empties the store", async () => {
    render(<DiagnosticsWorkspace />);
    await waitFor(() => expect(screen.getByText("Healthy")).toBeInTheDocument());

    act(() => {
      useGolcStore.getState().appendAppLog({
        seq: 1,
        level: "warn",
        source: "midi",
        message: "GOLC_WAILS_MIDI_DRIVER_UNAVAILABLE: no ports available",
        at: "2026-07-25T12:00:00Z",
      });
    });

    await waitFor(() =>
      expect(
        within(screen.getByRole("list", { name: "Application log" })).getByText(
          "GOLC_WAILS_MIDI_DRIVER_UNAVAILABLE: no ports available",
        ),
      ).toBeInTheDocument(),
    );

    fireEvent.click(screen.getByRole("button", { name: "Clear" }));
    expect(screen.queryByRole("list", { name: "Application log" })).not.toBeInTheDocument();
    expect(screen.getByText("No log activity yet.")).toBeInTheDocument();
    expect(useGolcStore.getState().appLog).toEqual([]);
  });
});
