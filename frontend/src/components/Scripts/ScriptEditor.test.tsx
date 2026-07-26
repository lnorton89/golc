// ScriptEditor.test.tsx (08-11-PLAN.md Task 3; extended by 08-12-PLAN.md
// Task 1, D-01): Monaco cannot instantiate under jsdom (there is no real
// canvas/text-measurement layer, jsdom does not implement Worker at all,
// and the real monaco-editor package touches
// `document.queryCommandSupported` at its own module-evaluation time -- see
// ScriptEditor.tsx's own doc comments), so this file vi.mock()s
// "monaco-editor" with a fake exposing editor.create/createModel/
// defineTheme/setTheme/MouseTargetType and the TypeScript-defaults surface,
// then asserts this component's <behavior> bullets against the fake's
// recorded calls (options passed, extra lib registered once per distinct
// value, dispose called on unmount, theme switched on a media-query
// change). This is the only tractable way to test Monaco here -- a later
// reader should not try to "fix" this into a real Monaco instantiation.
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
//
// 08-12-PLAN.md Task 1 extends the fake's editor.create() return value with
// onMouseDown (recording listeners, exposing __emitMouseDown to simulate a
// glyph-margin/line-number/text-area mouse-down) and
// createDecorationsCollection (recording every .set()/.clear() call on a
// fake IEditorDecorationsCollection) -- ScriptEditor.tsx creates exactly
// two collections per mount, in order: breakpoints first, then the
// current-execution-line highlight (see decorationsCollectionAt below).
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";

// MOCK_MOUSE_TARGET_TYPE mirrors the real monaco.editor.MouseTargetType
// enum's relevant members (GUTTER_GLYPH_MARGIN, GUTTER_LINE_NUMBERS,
// TEXTAREA) -- exact numeric values don't matter, only that
// ScriptEditor.tsx and this test file agree on the same object. Declared
// via vi.hoisted() (not a plain top-level const) because vi.mock's factory
// below is hoisted above this test file's own static
// `import * as monacoMock from "monaco-editor"` -- a plain const here would
// still be in its temporal dead zone when the factory first runs.
const { MOCK_MOUSE_TARGET_TYPE } = vi.hoisted(() => ({
  MOCK_MOUSE_TARGET_TYPE: {
    TEXTAREA: 1,
    GUTTER_GLYPH_MARGIN: 2,
    GUTTER_LINE_NUMBERS: 3,
  } as const,
}));

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

  // createFakeDecorationsCollection mirrors Monaco's own
  // IEditorDecorationsCollection contract closely enough to assert
  // ScriptEditor.tsx's own decoration-replacement behavior: .set() records
  // the full replacement list (never an append), .clear() records an empty
  // list.
  function createFakeDecorationsCollection() {
    let current: Array<Record<string, unknown>> = [];
    const set = vi.fn((decorations: Array<Record<string, unknown>>) => {
      current = decorations;
      return decorations.map((_, index) => `dec-${index}`);
    });
    const clear = vi.fn(() => {
      current = [];
    });
    return {
      set,
      clear,
      get length() {
        return current.length;
      },
      __current: () => current,
    };
  }

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

    const mouseDownListeners: Array<(event: unknown) => void> = [];
    const onMouseDown = vi.fn((listener: (event: unknown) => void) => {
      mouseDownListeners.push(listener);
      return {
        dispose: vi.fn(() => {
          const index = mouseDownListeners.indexOf(listener);
          if (index >= 0) mouseDownListeners.splice(index, 1);
        }),
      };
    });

    const decorationsCollections: Array<ReturnType<typeof createFakeDecorationsCollection>> = [];
    const createDecorationsCollection = vi.fn(
      (initial?: Array<Record<string, unknown>>) => {
        const collection = createFakeDecorationsCollection();
        if (initial && initial.length > 0) collection.set(initial);
        decorationsCollections.push(collection);
        return collection;
      },
    );

    return {
      dispose: vi.fn(),
      updateOptions: vi.fn(),
      getModel: () => model,
      onMouseDown,
      createDecorationsCollection,
      __options: options,
      __textarea: textarea,
      __decorationsCollections: decorationsCollections,
      __emitMouseDown: (event: unknown) => {
        mouseDownListeners.slice().forEach((listener) => listener(event));
      },
    };
  });

  const defineTheme = vi.fn();
  const setTheme = vi.fn();
  const addExtraLib = vi.fn(() => ({ dispose: vi.fn() }));
  const setCompilerOptions = vi.fn();

  return {
    editor: {
      create,
      createModel,
      defineTheme,
      setTheme,
      MouseTargetType: MOCK_MOUSE_TARGET_TYPE,
    },
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

// createdEditorAt returns the fake editor object returned by the Nth
// (0-indexed) editor.create() call in the current test.
// __decorationsCollections[0] is always the breakpoint collection and [1]
// is always the current-execution-line collection -- ScriptEditor.tsx's
// mount effect creates them in that fixed order (08-12-PLAN.md Task 1).
function createdEditorAt(index: number) {
  return (monacoMock.editor.create as ReturnType<typeof vi.fn>).mock.results[index].value;
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

  // --- 08-12-PLAN.md Task 1: glyph-margin breakpoint gutter and the
  // current-execution-line highlight (D-01) ---

  it("calls onToggleBreakpoint exactly once for a glyph-margin mouse-down on line N, and not for the line-number column or text area", async () => {
    const handleToggle = vi.fn();
    render(
      <ScriptEditor
        value=""
        onChange={() => {}}
        sdkTypeDefinitions=""
        breakpointLines={[]}
        onToggleBreakpoint={handleToggle}
      />,
    );
    await waitForMount();
    const editor = createdEditorAt(0);

    editor.__emitMouseDown({
      target: { type: MOCK_MOUSE_TARGET_TYPE.GUTTER_GLYPH_MARGIN, position: { lineNumber: 5 } },
    });
    expect(handleToggle).toHaveBeenCalledTimes(1);
    expect(handleToggle).toHaveBeenCalledWith(5);

    editor.__emitMouseDown({
      target: { type: MOCK_MOUSE_TARGET_TYPE.GUTTER_LINE_NUMBERS, position: { lineNumber: 5 } },
    });
    editor.__emitMouseDown({ target: { type: MOCK_MOUSE_TARGET_TYPE.TEXTAREA, position: null } });

    // Neither the line-number column nor the text area ever call
    // onToggleBreakpoint -- still exactly the one call from the real
    // glyph-margin mouse-down above.
    expect(handleToggle).toHaveBeenCalledTimes(1);
  });

  it("renders no glyph decorations when breakpointLines is empty", async () => {
    render(<ScriptEditor value="" onChange={() => {}} sdkTypeDefinitions="" breakpointLines={[]} />);
    await waitForMount();
    const editor = createdEditorAt(0);
    const breakpointCollection = editor.__decorationsCollections[0];

    expect(breakpointCollection.__current()).toEqual([]);
  });

  it("gives one breakpoint and three breakpoints the identical glyph decoration class, with no count-dependent variant", async () => {
    const { rerender } = render(
      <ScriptEditor value="" onChange={() => {}} sdkTypeDefinitions="" breakpointLines={[3]} />,
    );
    await waitForMount();
    const editor = createdEditorAt(0);
    const breakpointCollection = editor.__decorationsCollections[0];
    await waitFor(() => expect(breakpointCollection.__current()).toHaveLength(1));

    const oneBreakpointClass = (
      breakpointCollection.__current()[0].options as Record<string, unknown>
    ).glyphMarginClassName;
    expect(oneBreakpointClass).toBeTruthy();

    rerender(<ScriptEditor value="" onChange={() => {}} sdkTypeDefinitions="" breakpointLines={[3, 7, 12]} />);
    await waitFor(() => expect(breakpointCollection.__current()).toHaveLength(3));

    const classes = breakpointCollection
      .__current()
      .map((decoration: Record<string, unknown>) => (decoration.options as Record<string, unknown>).glyphMarginClassName);
    expect(new Set(classes).size).toBe(1);
    expect(classes[0]).toBe(oneBreakpointClass);
  });

  it("replaces the breakpoint decoration set rather than accumulating, across three successive breakpointLines changes", async () => {
    const { rerender } = render(
      <ScriptEditor value="" onChange={() => {}} sdkTypeDefinitions="" breakpointLines={[1]} />,
    );
    await waitForMount();
    const editor = createdEditorAt(0);
    const breakpointCollection = editor.__decorationsCollections[0];
    await waitFor(() => expect(breakpointCollection.__current()).toHaveLength(1));

    rerender(<ScriptEditor value="" onChange={() => {}} sdkTypeDefinitions="" breakpointLines={[1, 2]} />);
    await waitFor(() => expect(breakpointCollection.__current()).toHaveLength(2));

    rerender(<ScriptEditor value="" onChange={() => {}} sdkTypeDefinitions="" breakpointLines={[5]} />);
    await waitFor(() => expect(breakpointCollection.__current()).toHaveLength(1));

    // The final decoration set names only line 5 -- line 1's earlier
    // decoration was replaced, not left behind alongside the new one.
    const lines = breakpointCollection
      .__current()
      .map((decoration: Record<string, unknown>) => (decoration.range as Record<string, unknown>).startLineNumber);
    expect(lines).toEqual([5]);
    // Three distinct breakpointLines values -> exactly three .set() calls,
    // each carrying the FULL replacement list -- never an append.
    expect(breakpointCollection.set).toHaveBeenCalledTimes(3);
  });

  it("applies an armed current-execution-line decoration for currentExecutionLine, and removes it when null", async () => {
    const { rerender } = render(
      <ScriptEditor value="" onChange={() => {}} sdkTypeDefinitions="" currentExecutionLine={7} />,
    );
    await waitForMount();
    const editor = createdEditorAt(0);
    const currentLineCollection = editor.__decorationsCollections[1];
    await waitFor(() => expect(currentLineCollection.__current()).toHaveLength(1));

    const decoration = currentLineCollection.__current()[0] as Record<string, unknown>;
    expect((decoration.range as Record<string, unknown>).startLineNumber).toBe(7);
    expect((decoration.options as Record<string, unknown>).className).toBeTruthy();

    rerender(<ScriptEditor value="" onChange={() => {}} sdkTypeDefinitions="" currentExecutionLine={null} />);
    await waitFor(() => expect(currentLineCollection.__current()).toEqual([]));
  });

  it("clears both decoration collections on unmount, along with the existing editor disposal", async () => {
    const { unmount } = render(
      <ScriptEditor
        value=""
        onChange={() => {}}
        sdkTypeDefinitions=""
        breakpointLines={[2]}
        currentExecutionLine={4}
      />,
    );
    await waitForMount();
    const editor = createdEditorAt(0);
    const breakpointCollection = editor.__decorationsCollections[0];
    const currentLineCollection = editor.__decorationsCollections[1];
    await waitFor(() => expect(breakpointCollection.__current()).toHaveLength(1));

    unmount();

    expect(breakpointCollection.clear).toHaveBeenCalledTimes(1);
    expect(currentLineCollection.clear).toHaveBeenCalledTimes(1);
    expect(editor.dispose).toHaveBeenCalledTimes(1);
  });
});
