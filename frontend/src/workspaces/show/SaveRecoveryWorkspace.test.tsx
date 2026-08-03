// SaveRecoveryWorkspace.test.tsx exercises the workspace end-to-end against
// a mocked window.go.wails.ShowService, the same direct-window-object
// convention every other Wails-bridge test in this codebase uses (see
// wailsBridge.ts's own doc comment; mirrors ScenesLooksWorkspace.test.tsx's
// identical shape).
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";

import SaveRecoveryWorkspace from "./SaveRecoveryWorkspace";

function ok(stdout = "") {
  return { exitCode: 0, stdout, stderr: "" };
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

describe("SaveRecoveryWorkspace", () => {
  beforeEach(() => {
    vi.stubGlobal("go", {
      wails: {
        ShowService: {
          Save: vi.fn().mockResolvedValue(ok()),
          SaveAs: vi.fn().mockResolvedValue(ok()),
          Diagnose: vi.fn().mockResolvedValue(diagnosticReport()),
          DetectRecoveryPoints: vi.fn().mockResolvedValue([]),
          AcceptRecoveryPoint: vi.fn().mockResolvedValue(ok()),
          DiscardRecoveryPoints: vi.fn().mockResolvedValue(ok()),
        },
      },
    });
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it("shows an empty state when no recovery points are offered", async () => {
    render(<SaveRecoveryWorkspace />);
    await waitFor(() =>
      expect(
        screen.getByText("No interrupted-session recovery points are currently offered."),
      ).toBeInTheDocument(),
    );
    expect(screen.getByRole("region", { name: "Save & Recovery workspace" })).toBeInTheDocument();
  });

  it("saves the working show via the Save button", async () => {
    const svc = (window as unknown as { go: { wails: { ShowService: Record<string, ReturnType<typeof vi.fn>> } } })
      .go.wails.ShowService;
    render(<SaveRecoveryWorkspace />);
    await waitFor(() => expect(screen.getByRole("button", { name: "Save" })).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(svc.Save).toHaveBeenCalled());
  });

  it("saves a copy via Save As with the entered destination path", async () => {
    const svc = (window as unknown as { go: { wails: { ShowService: Record<string, ReturnType<typeof vi.fn>> } } })
      .go.wails.ShowService;
    render(<SaveRecoveryWorkspace />);
    await waitFor(() => expect(screen.getByLabelText("Save As destination path")).toBeInTheDocument());

    fireEvent.change(screen.getByLabelText("Save As destination path"), {
      target: { value: "C:\\shows\\copy.golc" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save As" }));

    await waitFor(() => expect(svc.SaveAs).toHaveBeenCalledWith("C:\\shows\\copy.golc"));
  });

  it("lists offered recovery points and accepts one", async () => {
    const svc = (window as unknown as { go: { wails: { ShowService: Record<string, ReturnType<typeof vi.fn>> } } })
      .go.wails.ShowService;
    svc.DetectRecoveryPoints.mockResolvedValue([
      { id: 1, createdAt: "2026-07-25T00:00:01Z", revision: 5 },
    ]);

    render(<SaveRecoveryWorkspace />);
    await waitFor(() => expect(screen.getByText("Revision 5")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: "Accept" }));
    await waitFor(() => expect(svc.AcceptRecoveryPoint).toHaveBeenCalledWith(1));
  });

  it("discards every offered recovery point via Discard All", async () => {
    const svc = (window as unknown as { go: { wails: { ShowService: Record<string, ReturnType<typeof vi.fn>> } } })
      .go.wails.ShowService;
    svc.DetectRecoveryPoints.mockResolvedValue([
      { id: 1, createdAt: "2026-07-25T00:00:01Z", revision: 5 },
    ]);

    render(<SaveRecoveryWorkspace />);
    await waitFor(() => expect(screen.getByRole("button", { name: "Discard All" })).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: "Discard All" }));
    await waitFor(() => expect(svc.DiscardRecoveryPoints).toHaveBeenCalled());
  });

  it("shows a migration-required note pointing at the CLI confirm flag", async () => {
    const svc = (window as unknown as { go: { wails: { ShowService: Record<string, ReturnType<typeof vi.fn>> } } })
      .go.wails.ShowService;
    svc.Diagnose.mockResolvedValue(diagnosticReport({ migrationRequired: true }));

    render(<SaveRecoveryWorkspace />);
    await waitFor(() => expect(screen.getByText(/confirm-migration/)).toBeInTheDocument());
  });
});
