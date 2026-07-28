// ShowsWorkspace.test.tsx exercises the workspace end-to-end against a
// mocked window.go.wails.ShowService.Inspect (current show path) and
// window.go.wails.App (PickShowPath/PickNewShowPath/RelaunchWithShow) --
// the same direct-window-object convention SaveRecoveryWorkspace.test.tsx
// uses (see wailsBridge.ts's own doc comment).
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";

import ShowsWorkspace from "./ShowsWorkspace";

function ok(stdout = "") {
  return { exitCode: 0, stdout, stderr: "" };
}

function failed(stderr = "boom") {
  return { exitCode: 1, stdout: "", stderr };
}

function showInspectView(overrides: Partial<Record<string, unknown>> = {}) {
  return {
    showPath: "",
    schemaVersion: 3,
    revision: 1,
    pools: [],
    deployments: [],
    ...overrides,
  };
}

type ShowServiceMock = { Inspect: ReturnType<typeof vi.fn> };
type AppMock = {
  PickShowPath: ReturnType<typeof vi.fn>;
  PickNewShowPath: ReturnType<typeof vi.fn>;
  RelaunchWithShow: ReturnType<typeof vi.fn>;
};

function showService(): ShowServiceMock {
  return (window as unknown as { go: { wails: { ShowService: ShowServiceMock } } }).go.wails.ShowService;
}

function appBinding(): AppMock {
  return (window as unknown as { go: { wails: { App: AppMock } } }).go.wails.App;
}

describe("ShowsWorkspace", () => {
  beforeEach(() => {
    vi.stubGlobal("go", {
      wails: {
        ShowService: {
          Inspect: vi.fn().mockResolvedValue(showInspectView()),
        },
        App: {
          PickShowPath: vi.fn().mockResolvedValue(""),
          PickNewShowPath: vi.fn().mockResolvedValue(""),
          RelaunchWithShow: vi.fn().mockResolvedValue(ok()),
        },
      },
    });
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it("renders the current show path with a full-value tooltip", async () => {
    const longPath = "C:\\shows\\a-very-long-nested-directory-structure\\my-club-show.golc";
    showService().Inspect.mockResolvedValue(showInspectView({ showPath: longPath }));

    render(<ShowsWorkspace />);

    await waitFor(() => expect(screen.getByText(longPath)).toBeInTheDocument());
    expect(screen.getByText(longPath)).toHaveAttribute("title", longPath);
  });

  it("renders the empty state when no show path is resolved", async () => {
    render(<ShowsWorkspace />);
    await waitFor(() =>
      expect(screen.getByText("Choose a show file to open, or create a new one.")).toBeInTheDocument(),
    );
  });

  it("renders the switching transient and disables both actions", async () => {
    const app = appBinding();
    app.PickShowPath.mockResolvedValue("C:\\shows\\other.golc");
    // A promise that never settles -- the process is exiting, no
    // resolution is ever expected in the running app.
    app.RelaunchWithShow.mockReturnValue(new Promise(() => {}));

    render(<ShowsWorkspace />);
    await waitFor(() => expect(screen.getByRole("button", { name: "Open Show…" })).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: "Open Show…" }));

    await waitFor(() =>
      expect(screen.getByText("Switching shows — GOLC will reload in a moment…")).toBeInTheDocument(),
    );
    expect(screen.getByRole("button", { name: "Open Show…" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "New Show…" })).toBeDisabled();
  });

  it("renders the relaunch failure copy", async () => {
    const app = appBinding();
    app.PickShowPath.mockResolvedValue("C:\\shows\\other.golc");
    app.RelaunchWithShow.mockResolvedValue(failed("GOLC_WAILS_RELAUNCH_SPAWN_FAILED: boom"));

    render(<ShowsWorkspace />);
    await waitFor(() => expect(screen.getByRole("button", { name: "Open Show…" })).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: "Open Show…" }));

    await waitFor(() =>
      expect(
        screen.getByText("Couldn't switch to this show. GOLC is still running the previous show."),
      ).toBeInTheDocument(),
    );
    expect(screen.getByRole("button", { name: "Open Show…" })).not.toBeDisabled();
    expect(screen.getByRole("button", { name: "New Show…" })).not.toBeDisabled();
  });

  it("cancelling the picker is a no-op", async () => {
    const app = appBinding();
    app.PickShowPath.mockResolvedValue("");

    render(<ShowsWorkspace />);
    await waitFor(() => expect(screen.getByRole("button", { name: "Open Show…" })).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: "Open Show…" }));

    await waitFor(() => expect(app.PickShowPath).toHaveBeenCalled());
    expect(app.RelaunchWithShow).not.toHaveBeenCalled();
    expect(screen.queryByText("Switching shows — GOLC will reload in a moment…")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Open Show…" })).not.toBeDisabled();
  });
});
