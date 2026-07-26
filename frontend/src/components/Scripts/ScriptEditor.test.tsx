// ScriptEditor.test.tsx (08-11-PLAN.md Task 3): Monaco cannot instantiate
// under jsdom (there is no real canvas/text-measurement layer, jsdom does
// not implement Worker at all, and the real monaco-editor package touches
// `document.queryCommandSupported` at its own module-evaluation time -- see
// ScriptEditor.tsx's own doc comments), so this file vi.mock()s
// "monaco-editor" with a fake exposing editor.create/createModel/
// defineTheme/setTheme and the TypeScript-defaults surface, then asserts
// this component's <behavior> bullets against the fake's recorded calls
// (options passed, extra lib registered once per distinct value, dispose
// called on unmount, theme switched on a media-query change). This is the
// only tractable way to test Monaco here -- a later reader should not try
// to "fix" this into a real Monaco instantiation.
//
// ScriptEditor.tsx dynamically imports "monaco-editor" inside its mount
// effect (rather than a static top-level import), so mounting resolves on
// a microtask -- every test below awaits monacoMock.editor.create having
// been called before asserting against it.
//
// The fake's editor.create() renders a real <textarea> into the given
// container (carrying the passed ariaLabel), wired so typing into it calls
// the fake model's setValue -- this both proves ScriptEditor's own
// onDidChangeContent wiring and lets ScriptsWorkspace.test.tsx interact
// with the mounted (fake) editor exactly like it did with 08-04's plain
// <textarea>.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";

vi.mock("monaco-editor", () => {
  function createFakeModel(initialValue: string) {
    let value = initialValue;
    const listeners: Array<() => void> = [];
    return {
      getValue: () => value,
      setValue: (next: string) => {
        if (next === value) return;
        value = next;
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
    textarea.value = model.getValue();
    textarea.addEventListener("change", () => {
      model.setValue(textarea.value);
    });
    container.appendChild(textarea);

    return {
      dispose: vi.fn(),
      updateOptions: vi.fn(),
      getModel: () => model,
      __options: options,
      __textarea: textarea,
    };
  });

  const defineTheme = vi.fn();
  const setTheme = vi.fn();
  const addExtraLib = vi.fn(() => ({ dispose: vi.fn() }));
  const setCompilerOptions = vi.fn();

  return {
    editor: { create, createModel, defineTheme, setTheme },
    // monaco-editor@0.55.1's real TypeScript language-service surface is
    // the top-level `typescript` namespace (see ScriptEditor.tsx's own
    // doc comment) -- `monaco.languages.typescript` is deprecated in this
    // pinned version, so the fake mirrors the real shape exactly.
    typescript: {
      typescriptDefaults: { addExtraLib, setCompilerOptions },
      ScriptTarget: { ESNext: 99 },
      ModuleKind: { None: 0 },
      ModuleResolutionKind: { NodeJs: 2 },
    },
  };
});

import * as monacoMock from "monaco-editor";
import ScriptEditor from "./ScriptEditor";

// mediaQueryListeners lets tests simulate a prefers-color-scheme change
// without a real matchMedia implementation (jsdom does not implement it).
let mediaQueryListeners: Array<(event: { matches: boolean }) => void>;

function stubMatchMedia(initialMatches: boolean) {
  mediaQueryListeners = [];
  const mql = {
    matches: initialMatches,
    addEventListener: vi.fn((_event: string, listener: (event: { matches: boolean }) => void) => {
      mediaQueryListeners.push(listener);
    }),
    removeEventListener: vi.fn((_event: string, listener: (event: { matches: boolean }) => void) => {
      mediaQueryListeners = mediaQueryListeners.filter((registered) => registered !== listener);
    }),
  };
  vi.stubGlobal("matchMedia", vi.fn().mockReturnValue(mql));
  return {
    emitSchemeChange: (matches: boolean) => {
      mediaQueryListeners.forEach((listener) => listener({ matches }));
    },
  };
}

// waitForMount waits for ScriptEditor's dynamic import("monaco-editor") to
// resolve and editor.create() to have actually run.
async function waitForMount(): Promise<void> {
  await waitFor(() => expect(monacoMock.editor.create).toHaveBeenCalled());
}

describe("ScriptEditor", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    stubMatchMedia(false);
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it("mounts Monaco with the reconciled Paper/Ink editor options", async () => {
    render(<ScriptEditor value="" onChange={() => {}} sdkTypeDefinitions="" ariaLabel="Test source" />);
    await waitForMount();

    expect(monacoMock.editor.create).toHaveBeenCalledTimes(1);
    const options = (monacoMock.editor.create as ReturnType<typeof vi.fn>).mock.calls[0][1];
    expect(options.fontFamily).toBe("JetBrains Mono");
    expect(options.fontSize).toBe(14);
    expect(options.lineHeight).toBe(21);
    expect(options.minimap).toEqual({ enabled: false });
    expect(options.glyphMargin).toBe(true);
    expect(options.lineNumbers).toBe("on");
  });

  it("registers the two Paper/Ink themes on mount", async () => {
    render(<ScriptEditor value="" onChange={() => {}} sdkTypeDefinitions="" />);
    await waitForMount();

    expect(monacoMock.editor.defineTheme).toHaveBeenCalledWith("golc-paper-ink-light", expect.any(Object));
    expect(monacoMock.editor.defineTheme).toHaveBeenCalledWith("golc-paper-ink-dark", expect.any(Object));
  });

  it("registers the SDK extra lib exactly once per distinct sdkTypeDefinitions value, not once per keystroke", async () => {
    const { rerender } = render(
      <ScriptEditor value="const x = 1;" onChange={() => {}} sdkTypeDefinitions="declare namespace golc {}" />,
    );
    await waitForMount();
    await waitFor(() => expect(monacoMock.typescript.typescriptDefaults.addExtraLib).toHaveBeenCalledTimes(1));

    // A value (content) change alone must not re-register the extra lib.
    rerender(<ScriptEditor value="const x = 2;" onChange={() => {}} sdkTypeDefinitions="declare namespace golc {}" />);
    expect(monacoMock.typescript.typescriptDefaults.addExtraLib).toHaveBeenCalledTimes(1);

    // A distinct sdkTypeDefinitions value registers a new extra lib.
    rerender(<ScriptEditor value="const x = 2;" onChange={() => {}} sdkTypeDefinitions="declare namespace golc { function ping(): void; }" />);
    await waitFor(() => expect(monacoMock.typescript.typescriptDefaults.addExtraLib).toHaveBeenCalledTimes(2));
  });

  it("calls onChange with the new value when the model's content changes", async () => {
    const handleChange = vi.fn();
    render(<ScriptEditor value="" onChange={handleChange} sdkTypeDefinitions="" ariaLabel="Chase Cycler source" />);
    await waitForMount();

    fireEvent.change(await screen.findByLabelText("Chase Cycler source"), {
      target: { value: "export function run() {}\n" },
    });

    expect(handleChange).toHaveBeenCalledWith("export function run() {}\n");
  });

  it("switches the active Monaco theme on a prefers-color-scheme change without remounting the editor", async () => {
    render(<ScriptEditor value="" onChange={() => {}} sdkTypeDefinitions="" />);
    await waitForMount();
    expect(monacoMock.editor.create).toHaveBeenCalledTimes(1);

    mediaQueryListeners.forEach((listener) => listener({ matches: true }));

    expect(monacoMock.editor.setTheme).toHaveBeenCalledWith("golc-paper-ink-dark");
    // Still exactly one editor instance -- the theme change never
    // remounted Monaco.
    expect(monacoMock.editor.create).toHaveBeenCalledTimes(1);
  });

  it("disposes the editor and model on unmount, and does not leak a second instance across StrictMode-style double mount/unmount", async () => {
    const { unmount } = render(<ScriptEditor value="" onChange={() => {}} sdkTypeDefinitions="" />);
    await waitForMount();

    const created = (monacoMock.editor.create as ReturnType<typeof vi.fn>).mock.results[0].value;
    const model = (monacoMock.editor.createModel as ReturnType<typeof vi.fn>).mock.results[0].value;

    unmount();

    expect(created.dispose).toHaveBeenCalledTimes(1);
    expect(model.dispose).toHaveBeenCalledTimes(1);

    // A second mount/unmount cycle creates and disposes its own instance,
    // never accumulating a leaked second live editor.
    const second = render(<ScriptEditor value="" onChange={() => {}} sdkTypeDefinitions="" />);
    await waitFor(() => expect(monacoMock.editor.create).toHaveBeenCalledTimes(2));
    second.unmount();
    const secondCreated = (monacoMock.editor.create as ReturnType<typeof vi.fn>).mock.results[1].value;
    expect(secondCreated.dispose).toHaveBeenCalledTimes(1);
  });
});
