// ScriptsWorkspace.test.tsx exercises the workspace end-to-end against a
// mocked window.go.wails.ScriptService, the same direct-window-object
// convention every other Wails-bridge test in this codebase uses (see
// wailsBridge.ts's own doc comment and ScenesLooksWorkspace.test.tsx).
//
// Monaco cannot instantiate under jsdom (ScriptEditor.tsx's own
// configureMonacoEnvironment doc comment), so this file vi.mock()s
// "monaco-editor" with the same fake ScriptEditor.test.tsx uses -- its
// editor.create() renders a real <textarea> (carrying the passed
// ariaLabel) bound two-way to the fake model, so every existing test below
// that queries `${name} source` and fires change events keeps working
// exactly as it did against 08-04's plain <textarea>, without knowing
// Monaco is now the thing rendering it.
import { afterEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";

vi.mock("monaco-editor", () => {
  function createFakeModel(initialValue: string) {
    let value = initialValue;
    const listeners: Array<() => void> = [];
    const boundElements = new Set<HTMLTextAreaElement>();
    return {
      getValue: () => value,
      setValue: (next: string) => {
        if (next === value) return;
        value = next;
        boundElements.forEach((el) => {
          el.value = next;
        });
        listeners.slice().forEach((listener) => listener());
      },
      onDidChangeContent: (listener: () => void) => {
        listeners.push(listener);
        return {
          dispose: vi.fn(() => {
            const index = listeners.indexOf(listener);
            if (index >= 0) listeners.splice(index, 1);
          }),
        };
      },
      __bindElement: (el: HTMLTextAreaElement) => {
        boundElements.add(el);
        el.value = value;
      },
      dispose: vi.fn(),
    };
  }

  const createModel = vi.fn((value: string) => createFakeModel(value));

  const create = vi.fn((container: HTMLElement, options: Record<string, unknown>) => {
    const textarea = document.createElement("textarea");
    if (typeof options.ariaLabel === "string") {
      textarea.setAttribute("aria-label", options.ariaLabel);
    }
    const model = options.model as ReturnType<typeof createFakeModel>;
    model.__bindElement(textarea);
    textarea.addEventListener("change", () => {
      model.setValue(textarea.value);
    });
    container.appendChild(textarea);

    return { dispose: vi.fn(), updateOptions: vi.fn(), getModel: () => model };
  });

  return {
    editor: {
      create,
      createModel,
      defineTheme: vi.fn(),
      setTheme: vi.fn(),
    },
    // monaco-editor@0.55.1's real TypeScript language-service surface is
    // the top-level `typescript` namespace, not `languages.typescript`
    // (deprecated in this pinned version) -- see ScriptEditor.tsx's own
    // doc comment.
    typescript: {
      typescriptDefaults: {
        addExtraLib: vi.fn(() => ({ dispose: vi.fn() })),
        setCompilerOptions: vi.fn(),
      },
      ScriptTarget: { ESNext: 99 },
      ModuleKind: { None: 0 },
      ModuleResolutionKind: { NodeJs: 2 },
    },
  };
});

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

function runOutcome(overrides: Partial<Record<string, unknown>> = {}) {
  return {
    runId: "run-1",
    status: "succeeded",
    reason: "",
    logs: [],
    outcomes: [],
    stackFrames: [],
    ...overrides,
  };
}

function stubScriptService(overrides: Partial<Record<string, ReturnType<typeof vi.fn>>> = {}) {
  const svc = {
    ListScripts: vi.fn().mockResolvedValue([]),
    GetScript: vi.fn().mockResolvedValue({ ...summary(), source: "" }),
    CreateScript: vi.fn().mockResolvedValue(ok()),
    SaveScriptSource: vi.fn().mockResolvedValue(ok()),
    DeleteScript: vi.fn().mockResolvedValue(ok()),
    SetScriptProfile: vi.fn().mockResolvedValue(ok()),
    RunScript: vi.fn().mockResolvedValue(runOutcome()),
    DebugScript: vi.fn().mockResolvedValue(runOutcome()),
    StopScript: vi.fn().mockResolvedValue(ok()),
    ValidateScript: vi.fn().mockResolvedValue({ valid: true, diagnostics: [] }),
    GetSDKTypeDefinitions: vi.fn().mockResolvedValue("declare namespace golc {}\n"),
    ...overrides,
  };
  vi.stubGlobal("go", { wails: { ScriptService: svc } });
  return svc;
}

// stubRuntimeEvents mocks window.runtime.EventsOn so a test can simulate a
// live "script:event" push (08-10-PLAN.md Task 3) without a real Wails
// webview host, mirroring wailsBridge.ts's own onStatusUpdate/
// onScriptEvent undefined-runtime-degrades-gracefully contract in reverse
// (here it IS defined, so onScriptEvent actually subscribes). Returns
// emitScriptEvent, which invokes every currently-registered "script:event"
// listener with one event payload.
function stubRuntimeEvents() {
  const listeners: Array<(...data: unknown[]) => void> = [];
  const runtime = {
    EventsOn: vi.fn((eventName: string, callback: (...data: unknown[]) => void) => {
      if (eventName === "script:event") {
        listeners.push(callback);
      }
      return vi.fn();
    }),
  };
  vi.stubGlobal("runtime", runtime);
  return {
    emitScriptEvent: (event: Partial<Record<string, unknown>>) => {
      listeners.forEach((callback) => callback(event));
    },
  };
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

    // Wrapped in `await act(async () => ...)` (not a plain fireEvent.change):
    // ScriptEditor's onChange fires from the fake Monaco model's own
    // "content changed" listener -- a native DOM event outside React's
    // synthetic event system, exactly like the real Monaco model's
    // onDidChangeContent -- so the resulting setSource update is scheduled
    // on a microtask under React 18+'s automatic batching, which a
    // synchronous fireEvent.change does not wait for before the next
    // synchronous fireEvent.click reads (possibly stale) state.
    await act(async () => {
      fireEvent.change(screen.getByLabelText("Chase Cycler source"), {
        target: { value: "export function run() {}\n" },
      });
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

  // --- 08-10-PLAN.md Task 3: toolbar actions, launch dialog, debug panel --

  it("renders Run/Debug/Validate/Stop Script, with Stop Script disabled until a run is active", async () => {
    stubScriptService({ ListScripts: vi.fn().mockResolvedValue([summary({ name: "Chase Cycler" })]) });
    stubRuntimeEvents();

    render(<ScriptsWorkspace />);
    await waitFor(() => expect(within(scriptList()).getByText("Chase Cycler")).toBeInTheDocument());
    await waitFor(() => expect(screen.getByRole("button", { name: "Run" })).not.toBeDisabled());

    expect(screen.getByRole("button", { name: "Debug" })).not.toBeDisabled();
    expect(screen.getByRole("button", { name: "Validate" })).not.toBeDisabled();
    expect(screen.getByRole("button", { name: "Stop Script" })).toBeDisabled();
  });

  it("clicking Run opens the launch dialog in run mode; clicking Debug opens it in debug mode", async () => {
    stubScriptService({ ListScripts: vi.fn().mockResolvedValue([summary({ name: "Chase Cycler" })]) });
    stubRuntimeEvents();

    render(<ScriptsWorkspace />);
    await waitFor(() => expect(within(scriptList()).getByText("Chase Cycler")).toBeInTheDocument());
    // Wait for the selection-validity-repair effect's auto-selection to
    // actually land (Run only enables once selectedName is set) --
    // otherwise this click can race ahead of that effect and land on a
    // still-disabled button.
    await waitFor(() => expect(screen.getByRole("button", { name: "Run" })).not.toBeDisabled());

    fireEvent.click(screen.getByRole("button", { name: "Run" }));
    expect(screen.getByText("Run Chase Cycler")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));

    fireEvent.click(screen.getByRole("button", { name: "Debug" }));
    expect(screen.getByText("Debug Chase Cycler")).toBeInTheDocument();
  });

  it("submitting the launch dialog saves the profile then launches, closing the dialog", async () => {
    const svc = stubScriptService({ ListScripts: vi.fn().mockResolvedValue([summary({ name: "Chase Cycler" })]) });
    stubRuntimeEvents();

    render(<ScriptsWorkspace />);
    await waitFor(() => expect(within(scriptList()).getByText("Chase Cycler")).toBeInTheDocument());
    // Wait for the selection-validity-repair effect's auto-selection to
    // actually land (Run only enables once selectedName is set) --
    // otherwise this click can race ahead of that effect and land on a
    // still-disabled button.
    await waitFor(() => expect(screen.getByRole("button", { name: "Run" })).not.toBeDisabled());

    fireEvent.click(screen.getByRole("button", { name: "Run" }));
    await waitFor(() => expect(screen.getByText("Run Chase Cycler")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: /^Run Chase Cycler$/ }));

    await waitFor(() => expect(svc.SetScriptProfile).toHaveBeenCalled());
    await waitFor(() => expect(svc.RunScript).toHaveBeenCalledWith("Chase Cycler"));
    await waitFor(() => expect(screen.queryByText("Run Chase Cycler")).not.toBeInTheDocument());
  });

  it("streams live script:event log lines into the debug panel", async () => {
    stubScriptService({ ListScripts: vi.fn().mockResolvedValue([summary({ name: "Chase Cycler" })]) });
    const { emitScriptEvent } = stubRuntimeEvents();

    render(<ScriptsWorkspace />);
    await waitFor(() => expect(within(scriptList()).getByText("Chase Cycler")).toBeInTheDocument());
    // Wait for the selection-validity-repair effect's auto-selection to
    // actually land (mirrors the same wait every other test below that
    // depends on a script being selected already uses) -- the debug
    // panel's placeholder only renders once selectedScript is set, and
    // ScriptEditor's own async Monaco mount (dynamic import) now shares
    // the same effect-flush window, so this can no longer be asserted
    // synchronously right after the list itself renders.
    await waitFor(() =>
      expect(
        screen.getByText("Run or Debug this script to see live logs, diagnostics, and command outcomes here."),
      ).toBeInTheDocument(),
    );

    emitScriptEvent({
      seq: 1,
      kind: "script.status",
      runId: "run-1",
      scriptName: "Chase Cycler",
      status: "running",
      reason: "",
    });
    emitScriptEvent({
      seq: 2,
      kind: "script.log",
      runId: "run-1",
      scriptName: "Chase Cycler",
      level: "info",
      message: "hello from script",
      source: "stdout",
    });

    await waitFor(() => expect(screen.getByText("hello from script")).toBeInTheDocument());
    expect(screen.getByText("Running")).toBeInTheDocument();
  });

  it("Stop Script calls stopScript immediately with no confirmation, while a run is active", async () => {
    const svc = stubScriptService({ ListScripts: vi.fn().mockResolvedValue([summary({ name: "Chase Cycler" })]) });
    const { emitScriptEvent } = stubRuntimeEvents();

    render(<ScriptsWorkspace />);
    await waitFor(() => expect(within(scriptList()).getByText("Chase Cycler")).toBeInTheDocument());

    emitScriptEvent({
      seq: 1,
      kind: "script.status",
      runId: "run-1",
      scriptName: "Chase Cycler",
      status: "running",
      reason: "",
    });
    await waitFor(() => expect(screen.getByRole("button", { name: "Stop Script" })).not.toBeDisabled());

    fireEvent.click(screen.getByRole("button", { name: "Stop Script" }));

    // No confirmation dialog appears -- StopScript is called immediately.
    await waitFor(() => expect(svc.StopScript).toHaveBeenCalledWith("Chase Cycler"));
    expect(screen.getByText("Stopping — finishing in-flight commands…")).toBeInTheDocument();
  });

  it("Validate renders the error summary and disables Run/Debug until a later validation passes", async () => {
    const svc = stubScriptService({
      ListScripts: vi.fn().mockResolvedValue([summary({ name: "Chase Cycler" })]),
      ValidateScript: vi
        .fn()
        .mockResolvedValueOnce({
          valid: false,
          diagnostics: [
            { code: "GOLC_SCRIPT_IMPORT_FORBIDDEN", message: "no imports", line: 1, column: 1, severity: "error" },
          ],
        })
        .mockResolvedValueOnce({ valid: true, diagnostics: [] }),
    });
    stubRuntimeEvents();

    render(<ScriptsWorkspace />);
    await waitFor(() => expect(within(scriptList()).getByText("Chase Cycler")).toBeInTheDocument());
    await waitFor(() => expect(screen.getByRole("button", { name: "Validate" })).not.toBeDisabled());

    fireEvent.click(screen.getByRole("button", { name: "Validate" }));

    await waitFor(() => expect(svc.ValidateScript).toHaveBeenCalledWith("Chase Cycler"));
    await waitFor(() =>
      expect(screen.getByText("This script has 1 error(s). Fix them before running.")).toBeInTheDocument(),
    );
    expect(screen.getByRole("button", { name: "Run" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Debug" })).toBeDisabled();

    fireEvent.click(screen.getByRole("button", { name: "Validate" }));
    await waitFor(() =>
      expect(screen.queryByText("This script has 1 error(s). Fix them before running.")).not.toBeInTheDocument(),
    );
    expect(screen.getByRole("button", { name: "Run" })).not.toBeDisabled();
  });

  it("Run Again re-opens the launch dialog after a terminal event, and Dismiss clears the banner", async () => {
    stubScriptService({ ListScripts: vi.fn().mockResolvedValue([summary({ name: "Chase Cycler" })]) });
    const { emitScriptEvent } = stubRuntimeEvents();

    render(<ScriptsWorkspace />);
    await waitFor(() => expect(within(scriptList()).getByText("Chase Cycler")).toBeInTheDocument());

    emitScriptEvent({
      seq: 1,
      kind: "script.status",
      runId: "run-1",
      scriptName: "Chase Cycler",
      status: "running",
      reason: "",
    });
    emitScriptEvent({
      seq: 2,
      kind: "script.terminal",
      runId: "run-1",
      scriptName: "Chase Cycler",
      status: "terminated",
      reason: 'GOLC_SCRIPT_STOPPED_BY_USER: script "Chase Cycler" was stopped by user request',
    });

    await waitFor(() => expect(screen.getByText(/^Stopped:/)).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: "Run Again" }));
    expect(screen.getByText("Run Chase Cycler")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));

    fireEvent.click(screen.getByRole("button", { name: "Dismiss" }));
    await waitFor(() =>
      expect(
        screen.getByText("Run or Debug this script to see live logs, diagnostics, and command outcomes here."),
      ).toBeInTheDocument(),
    );
  });
});
