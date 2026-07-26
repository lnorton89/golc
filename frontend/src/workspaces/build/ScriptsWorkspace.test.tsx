// ScriptsWorkspace.test.tsx exercises the workspace end-to-end against a
// mocked window.go.wails.ScriptService, the same direct-window-object
// convention every other Wails-bridge test in this codebase uses (see
// wailsBridge.ts's own doc comment and ScenesLooksWorkspace.test.tsx).
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";

import ScriptsWorkspace from "./ScriptsWorkspace";

function summary(overrides: Partial<Record<string, unknown>> = {}) {
  return {
    id: "script-1",
    name: "Chase Cycler",
    lastRunStatus: "never_run",
    scope: "playback",
    preset: "quick-action",
    deadlineSeconds: 30,
    ratePerSecond: 20,
    memoryLimitMB: 256,
    cpuCapPercent: 25,
    ...overrides,
  };
}

function ok(stdout = "") {
  return { exitCode: 0, stdout, stderr: "" };
}

function fail(stderr: string) {
  return { exitCode: 1, stdout: "", stderr };
}

function stubScriptService(overrides: Partial<Record<string, ReturnType<typeof vi.fn>>> = {}) {
  const svc = {
    ListScripts: vi.fn().mockResolvedValue([]),
    GetScript: vi.fn().mockResolvedValue({ ...summary(), source: "" }),
    CreateScript: vi.fn().mockResolvedValue(ok()),
    SaveScriptSource: vi.fn().mockResolvedValue(ok()),
    DeleteScript: vi.fn().mockResolvedValue(ok()),
    SetScriptProfile: vi.fn().mockResolvedValue(ok()),
    ...overrides,
  };
  vi.stubGlobal("go", { wails: { ScriptService: svc } });
  return svc;
}

// scriptList returns the D-16 library list's <ul aria-label="Script list">
// container, so row-scoped queries never collide with the editor Toolbar
// title rendering the same selected script's name.
function scriptList(): HTMLElement {
  return screen.getByRole("list", { name: "Script list" });
}

describe("ScriptsWorkspace", () => {
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it("shows every script's name, last-run status chip, and capability-scope summary", async () => {
    stubScriptService({
      ListScripts: vi.fn().mockResolvedValue([
        summary({ id: "script-1", name: "Chase Cycler", lastRunStatus: "running", scope: "playback" }),
        summary({ id: "script-2", name: "Blackout Fade", lastRunStatus: "failed", scope: "authoring" }),
      ]),
    });

    render(<ScriptsWorkspace />);

    await waitFor(() => expect(within(scriptList()).getByText("Chase Cycler")).toBeInTheDocument());
    const list = scriptList();
    expect(within(list).getByText("Blackout Fade")).toBeInTheDocument();
    expect(within(list).getByText("Running")).toBeInTheDocument();
    expect(within(list).getByText("Failed")).toBeInTheDocument();
    expect(within(list).getByText("playback")).toBeInTheDocument();
    expect(within(list).getByText("authoring")).toBeInTheDocument();
  });

  it("shows the empty state heading, body copy, and a New Script control when there are no scripts", async () => {
    stubScriptService();

    render(<ScriptsWorkspace />);

    await waitFor(() => expect(screen.getByText("No scripts yet")).toBeInTheDocument());
    expect(
      screen.getByText(
        "Create a script to automate GOLC through the typed SDK. Scripts run in an isolated process and can't touch playback or Art-Net directly.",
      ),
    ).toBeInTheDocument();
    expect(screen.getAllByRole("button", { name: "New Script" }).length).toBeGreaterThan(0);
  });

  it("creates a script from the New Script form and lists it", async () => {
    const svc = stubScriptService();
    svc.ListScripts
      .mockResolvedValueOnce([])
      .mockResolvedValueOnce([summary({ name: "Chase Cycler" })]);

    render(<ScriptsWorkspace />);
    await waitFor(() => expect(screen.getByText("No scripts yet")).toBeInTheDocument());

    fireEvent.click(screen.getAllByRole("button", { name: "New Script" })[0]);
    fireEvent.change(screen.getByLabelText("New script name"), { target: { value: "Chase Cycler" } });
    fireEvent.click(screen.getByRole("button", { name: "Create" }));

    await waitFor(() => expect(svc.CreateScript).toHaveBeenCalledWith("Chase Cycler"));
    await waitFor(() => expect(svc.ListScripts).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(within(scriptList()).getByText("Chase Cycler")).toBeInTheDocument());
  });

  it("selects a script, loads its source into the editor via GetScript", async () => {
    const svc = stubScriptService({
      ListScripts: vi.fn().mockResolvedValue([summary({ name: "Chase Cycler" })]),
      GetScript: vi.fn().mockResolvedValue({ ...summary({ name: "Chase Cycler" }), source: "export const x = 1;" }),
    });

    render(<ScriptsWorkspace />);
    await waitFor(() => expect(within(scriptList()).getByText("Chase Cycler")).toBeInTheDocument());

    fireEvent.click(within(scriptList()).getByText("Chase Cycler"));

    await waitFor(() => expect(svc.GetScript).toHaveBeenCalledWith("Chase Cycler"));
    await waitFor(() =>
      expect(screen.getByLabelText("Chase Cycler source")).toHaveValue("export const x = 1;"),
    );
  });

  it("edits the textarea and saves via SaveScriptSource, then refreshes", async () => {
    const svc = stubScriptService({
      ListScripts: vi.fn().mockResolvedValue([summary({ name: "Chase Cycler" })]),
      GetScript: vi.fn().mockResolvedValue({ ...summary({ name: "Chase Cycler" }), source: "" }),
    });

    render(<ScriptsWorkspace />);
    await waitFor(() => expect(within(scriptList()).getByText("Chase Cycler")).toBeInTheDocument());
    fireEvent.click(within(scriptList()).getByText("Chase Cycler"));
    await waitFor(() => expect(screen.getByLabelText("Chase Cycler source")).toBeInTheDocument());

    fireEvent.change(screen.getByLabelText("Chase Cycler source"), {
      target: { value: "export function run() {}\n" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() =>
      expect(svc.SaveScriptSource).toHaveBeenCalledWith("Chase Cycler", "export function run() {}\n"),
    );
    await waitFor(() => expect(svc.ListScripts).toHaveBeenCalledTimes(2));
  });

  it("shows the Delete Script confirmation naming the script; confirming calls DeleteScript, cancelling does not", async () => {
    const svc = stubScriptService({
      ListScripts: vi.fn().mockResolvedValue([summary({ name: "Chase Cycler" })]),
      GetScript: vi.fn().mockResolvedValue({ ...summary({ name: "Chase Cycler" }), source: "" }),
    });

    render(<ScriptsWorkspace />);
    await waitFor(() => expect(within(scriptList()).getByText("Chase Cycler")).toBeInTheDocument());
    fireEvent.click(within(scriptList()).getByText("Chase Cycler"));
    await waitFor(() => expect(screen.getByLabelText("Chase Cycler source")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: "Delete Script" }));

    const confirmCopy = await screen.findByText(
      "Delete Script: This permanently removes Chase Cycler and its saved capability profile from this show. This can't be undone.",
    );
    const confirmContainer = confirmCopy.closest("div") as HTMLElement;

    // Cancelling must not call DeleteScript.
    fireEvent.click(within(confirmContainer).getByRole("button", { name: "Cancel" }));
    expect(svc.DeleteScript).not.toHaveBeenCalled();

    // Re-open and confirm.
    fireEvent.click(screen.getByRole("button", { name: "Delete Script" }));
    const confirmCopyAgain = await screen.findByText(
      "Delete Script: This permanently removes Chase Cycler and its saved capability profile from this show. This can't be undone.",
    );
    const confirmContainerAgain = confirmCopyAgain.closest("div") as HTMLElement;
    fireEvent.click(within(confirmContainerAgain).getByRole("button", { name: "Delete Script" }));

    await waitFor(() => expect(svc.DeleteScript).toHaveBeenCalledWith("Chase Cycler"));
  });

  it("renders the empty state and an inline error rather than throwing when window.go is undefined", async () => {
    // No stubScriptService() call -- window.go stays undefined for this test.
    render(<ScriptsWorkspace />);

    await waitFor(() => expect(screen.getByText("No scripts yet")).toBeInTheDocument());
    expect(
      screen.getByText("Can't reach the script host. GOLC will try to reconnect automatically."),
    ).toBeInTheDocument();
  });

  it("gives a long script name a title attribute carrying the full name", async () => {
    const longName = "A".repeat(120);
    stubScriptService({
      ListScripts: vi.fn().mockResolvedValue([summary({ name: longName })]),
    });

    render(<ScriptsWorkspace />);
    await waitFor(() => expect(within(scriptList()).getByText(longName)).toBeInTheDocument());

    const row = within(scriptList()).getByText(longName).closest("[title]");
    expect(row).not.toBeNull();
    expect(row).toHaveAttribute("title", longName);
  });

  it("surfaces CreateScript's GOLC_SCRIPT_NAME_DUPLICATE error inline", async () => {
    const svc = stubScriptService({
      ListScripts: vi.fn().mockResolvedValue([summary({ name: "Chase Cycler" })]),
      CreateScript: vi.fn().mockResolvedValue(fail("GOLC_SCRIPT_NAME_DUPLICATE: a script named \"Chase Cycler\" already exists")),
    });

    render(<ScriptsWorkspace />);
    await waitFor(() => expect(within(scriptList()).getByText("Chase Cycler")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: "New Script" }));
    fireEvent.change(screen.getByLabelText("New script name"), { target: { value: "Chase Cycler" } });
    fireEvent.click(screen.getByRole("button", { name: "Create" }));

    await waitFor(() => expect(svc.CreateScript).toHaveBeenCalledWith("Chase Cycler"));
    await waitFor(() => expect(screen.getByText(/GOLC_SCRIPT_NAME_DUPLICATE/)).toBeInTheDocument());
  });
});
