// ScriptEditor.tsx (08-11-PLAN.md Task 3, D-15): a real Monaco editor
// running the TypeScript language service against the generated GOLC SDK
// -- live type-checking and autocomplete from a bare `golc.` with nothing
// imported, not syntax highlighting with validate-on-demand. Replaces
// ScriptsWorkspace.tsx's 08-04 placeholder <textarea> in place; the
// D-01/D-02 breakpoint gutter (glyph-margin click handling) is scaffolded
// by this task's `glyphMargin: true` option but its click wiring lands in a
// later plan.
//
// A controlled component: `value`/`onChange` mirror the plain <textarea>
// contract it replaces. The Monaco model is created once at mount (in an
// async effect -- see below) and kept in sync with an externally-driven
// `value` change (e.g. selecting a different script) via a dedicated
// effect that only calls model.setValue() when the incoming value
// actually differs from the model's own current value -- never on every
// render, and never in response to the model's own onDidChangeContent
// firing from the user's own typing (that would reset the cursor position
// on every keystroke).
//
// value/readOnly/sdkTypeDefinitions are also mirrored into refs
// (valueRef/readOnlyRef/sdkTypeDefinitionsRef), updated unconditionally on
// every render. The async mount effect below reads those refs -- not its
// own closured props -- at the moment Monaco actually finishes loading,
// because mounting resolves on a later microtask/task than the render
// that scheduled it: by the time it completes, a prop this component
// received at mount time may already be stale (e.g. ScriptsWorkspace's
// GetScript/getSdkTypeDefinitions calls can resolve before or after
// Monaco's own dynamic import does, in either order). Reading the refs at
// completion time -- rather than relying on a `[..., ready]`-dependent
// effect to "catch up" after the fact -- avoids a real race: a `ready`
// dependency fires the sync effect the instant `ready` commits, which is
// not guaranteed to be the same commit as a `value` update that happened
// to land first, so the effect could read a still-stale `value` closure
// and stomp the model with it immediately after the model was already
// correct.
//
// "monaco-editor" is imported DYNAMICALLY (never a static top-level
// `import * as monaco from "monaco-editor"`) and wrapped in try/catch: the
// real package touches browser-only APIs (e.g.
// `document.queryCommandSupported`) at its own module-evaluation time,
// which jsdom does not implement -- a static import would crash every
// jsdom-based test that renders this component's tree, including
// app-wide smoke/navigation tests that never mount Scripts and have no
// reason to know they need to vi.mock("monaco-editor"). A dynamic import
// confines that failure to a caught rejection scoped to this component
// alone, matching wailsBridge.ts's own "degrade gracefully, never throw"
// contract at the editor layer. ScriptEditor.test.tsx and
// ScriptsWorkspace.test.tsx vi.mock("monaco-editor") to exercise the real
// mount path against a fake; every other test that merely renders through
// this component sees the caught-failure fallback below instead.
import { useEffect, useRef, useState } from "react";
import type * as Monaco from "monaco-editor";

import {
  buildGolcMonacoThemes,
  GOLC_PAPER_INK_DARK_THEME,
  GOLC_PAPER_INK_LIGHT_THEME,
  resolveThemeName,
} from "./monacoTheme";
import styles from "./ScriptEditor.module.css";

// SDK_EXTRA_LIB_PATH is the extra-lib's virtual filename Monaco's
// TypeScript language service uses to key its own registration -- never
// read from or written to, just a stable identity for
// addExtraLib/updating the same lib on a later sdkTypeDefinitions change.
const SDK_EXTRA_LIB_PATH = "file:///golc-sdk.d.ts";

const READY_PLACEHOLDER = "Loading editor…";
const LOAD_FAILED_MESSAGE = "The script editor failed to load. Reload GOLC to try again.";

// environmentConfigured guards configureMonacoEnvironment against
// reconfiguring self.MonacoEnvironment on every ScriptEditor mount --
// module-level because the worker loader is a process-wide singleton, not
// a per-editor-instance concern.
let environmentConfigured = false;

// configureMonacoEnvironment wires self.MonacoEnvironment.getWorker to
// Vite's native `?worker` import suffix (monaco-editor@0.55.1's own worker
// entry files -- editor.worker.js, language/typescript/ts.worker.js -- are
// plain ES modules Vite can bundle this way with no dedicated Monaco Vite
// plugin required), so the TypeScript language-service worker loads both
// under `vite dev` and under the go:embed'd production build (vite.config.ts's
// `worker: { format: "es" }` covers the build-time half of this).
//
// The worker modules are only ever dynamically imported when `Worker` is a
// defined global -- true in the real Wails webview, false under Vitest's
// jsdom test environment (jsdom does not implement `Worker` at all, and
// Vite's `?worker` wrapper module extends the global `Worker` class at its
// own module-evaluation time). Guarding on `typeof Worker` keeps this
// codepath from ever running under jsdom.
async function configureMonacoEnvironment(): Promise<void> {
  if (environmentConfigured) return;
  environmentConfigured = true;
  if (typeof Worker === "undefined") return;

  const [{ default: EditorWorker }, { default: TsWorker }] = await Promise.all([
    import("monaco-editor/esm/vs/editor/editor.worker?worker"),
    import("monaco-editor/esm/vs/language/typescript/ts.worker?worker"),
  ]);

  self.MonacoEnvironment = {
    getWorker(_workerId: string, label: string) {
      if (label === "typescript" || label === "javascript") {
        return new TsWorker();
      }
      return new EditorWorker();
    },
  };
}

interface ScriptEditorProps {
  value: string;
  onChange: (value: string) => void;
  readOnly?: boolean;
  sdkTypeDefinitions: string;
  ariaLabel?: string;
}

export default function ScriptEditor({ value, onChange, readOnly, sdkTypeDefinitions, ariaLabel }: ScriptEditorProps) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const monacoRef = useRef<typeof Monaco | null>(null);
  const editorRef = useRef<Monaco.editor.IStandaloneCodeEditor | null>(null);
  const modelRef = useRef<Monaco.editor.ITextModel | null>(null);
  const onChangeRef = useRef(onChange);
  const valueRef = useRef(value);
  const readOnlyRef = useRef(readOnly);
  const sdkTypeDefinitionsRef = useRef(sdkTypeDefinitions);
  const extraLibRef = useRef<Monaco.IDisposable | null>(null);
  const lastRegisteredTypesRef = useRef<string | null>(null);
  const [ready, setReady] = useState(false);
  const [loadFailed, setLoadFailed] = useState(false);

  onChangeRef.current = onChange;
  valueRef.current = value;
  readOnlyRef.current = readOnly;
  sdkTypeDefinitionsRef.current = sdkTypeDefinitions;

  // Mount effect: dynamically imports monaco-editor, creates the model +
  // editor exactly once (using the *ref* snapshots above, not this
  // effect's own closured props -- see this file's header comment),
  // registers both Paper/Ink themes, and attaches the prefers-color-scheme
  // listener (Monaco does not follow the media query on its own). Cleanup
  // disposes everything so unmounting (or React StrictMode's dev-only
  // double-invoke) never leaks a second live instance.
  useEffect(() => {
    let cancelled = false;
    let cleanupMounted: (() => void) | undefined;

    void (async () => {
      let monaco: typeof Monaco;
      try {
        monaco = await import("monaco-editor");
      } catch {
        if (!cancelled) setLoadFailed(true);
        return;
      }
      if (cancelled || !containerRef.current) return;
      monacoRef.current = monaco;

      void configureMonacoEnvironment();

      const themes = buildGolcMonacoThemes();
      monaco.editor.defineTheme(GOLC_PAPER_INK_LIGHT_THEME, themes.light);
      monaco.editor.defineTheme(GOLC_PAPER_INK_DARK_THEME, themes.dark);

      const model = monaco.editor.createModel(valueRef.current, "typescript");
      modelRef.current = model;

      // monaco-editor@0.55.1 deprecated `monaco.languages.typescript`
      // (typed as `{ deprecated: true }`) in favour of a new top-level
      // `typescript` namespace exported directly off "monaco-editor" --
      // verified against this pinned version's own
      // esm/vs/editor/editor.main.d.ts, which exports
      // `monaco_contribution as typescript` at the module's root rather
      // than nesting it under `languages`.
      monaco.typescript.typescriptDefaults.setCompilerOptions({
        target: monaco.typescript.ScriptTarget.ESNext,
        module: monaco.typescript.ModuleKind.None,
        moduleResolution: monaco.typescript.ModuleResolutionKind.NodeJs,
        noEmit: true,
        allowNonTsExtensions: true,
        lib: ["es2022"],
      });

      const prefersDarkQuery = window.matchMedia?.("(prefers-color-scheme: dark)");
      const initialPrefersDark = prefersDarkQuery?.matches ?? false;

      const editor = monaco.editor.create(containerRef.current, {
        model,
        theme: resolveThemeName(initialPrefersDark),
        fontFamily: "JetBrains Mono",
        fontSize: 14,
        lineHeight: 21,
        minimap: { enabled: false },
        glyphMargin: true,
        lineNumbers: "on",
        readOnly: readOnlyRef.current ?? false,
        automaticLayout: true,
        scrollBeyondLastLine: false,
        ariaLabel,
      });
      editorRef.current = editor;

      const changeSubscription = model.onDidChangeContent(() => {
        onChangeRef.current(model.getValue());
      });

      const handleSchemeChange = (event: MediaQueryListEvent) => {
        monaco.editor.setTheme(resolveThemeName(event.matches));
      };
      prefersDarkQuery?.addEventListener("change", handleSchemeChange);

      // Registers the extra lib using the freshest known
      // sdkTypeDefinitions (via the ref, not this effect's own closured
      // prop -- see this file's header comment): ScriptsWorkspace fetches
      // it in parallel with Monaco's own dynamic import, so a real value
      // may already be available by the time mounting finishes.
      lastRegisteredTypesRef.current = sdkTypeDefinitionsRef.current;
      extraLibRef.current = sdkTypeDefinitionsRef.current
        ? monaco.typescript.typescriptDefaults.addExtraLib(sdkTypeDefinitionsRef.current, SDK_EXTRA_LIB_PATH)
        : null;

      setReady(true);

      cleanupMounted = () => {
        prefersDarkQuery?.removeEventListener("change", handleSchemeChange);
        changeSubscription.dispose();
        extraLibRef.current?.dispose();
        extraLibRef.current = null;
        lastRegisteredTypesRef.current = null;
        editorRef.current?.dispose();
        editorRef.current = null;
        modelRef.current?.dispose();
        modelRef.current = null;
        monacoRef.current = null;
      };
    })();

    return () => {
      cancelled = true;
      cleanupMounted?.();
      setReady(false);
    };
    // Mount/unmount only -- value/readOnly/ariaLabel/sdkTypeDefinitions
    // changes are handled by the dedicated sync effects below (reading the
    // ref snapshots at mount-completion time above), not by recreating the
    // editor.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Keeps the model in sync with an externally-driven `value` change (e.g.
  // selecting a different script) -- only calls setValue when the incoming
  // value actually differs from the model's own current value, so the
  // user's own typing (which already updated the model directly) never
  // triggers a redundant setValue that would reset the cursor position.
  // Deliberately depends only on `value` (not `ready`): the mount effect
  // above already applies the freshest value once at completion via
  // valueRef, so this effect only needs to react to a *subsequent* prop
  // change -- adding `ready` here would let this effect's own `value`
  // closure race the mount effect's `ready` commit (see this file's
  // header comment for the exact race that caused).
  useEffect(() => {
    const model = modelRef.current;
    if (!model) return;
    if (model.getValue() !== value) {
      model.setValue(value);
    }
  }, [value]);

  // Registers the fetched golc.d.ts content as Monaco's TypeScript extra
  // lib exactly once per distinct sdkTypeDefinitions value (never once per
  // keystroke -- this effect only depends on sdkTypeDefinitions, not
  // value): disposes the previously registered lib (if any) before adding
  // the new one, so a later re-fetch never accumulates stale duplicate
  // libs. The mount effect above already registers the freshest value once
  // at completion (via sdkTypeDefinitionsRef), so this effect's own no-op
  // guard prevents a redundant duplicate registration immediately after
  // mount; it only fires again for a genuinely later change.
  useEffect(() => {
    const monaco = monacoRef.current;
    if (!monaco) return;
    if (lastRegisteredTypesRef.current === sdkTypeDefinitions) return;
    lastRegisteredTypesRef.current = sdkTypeDefinitions;

    extraLibRef.current?.dispose();
    extraLibRef.current = sdkTypeDefinitions
      ? monaco.typescript.typescriptDefaults.addExtraLib(sdkTypeDefinitions, SDK_EXTRA_LIB_PATH)
      : null;
  }, [sdkTypeDefinitions]);

  // Keeps the live editor's read-only state in sync with the readOnly prop
  // (e.g. while the workspace is still loading a script's source). The
  // mount effect above already applies the freshest readOnly once at
  // completion (via readOnlyRef); this effect only fires again for a
  // genuinely later change.
  useEffect(() => {
    editorRef.current?.updateOptions({ readOnly: readOnly ?? false });
  }, [readOnly]);

  return (
    <div className={styles.wrapper}>
      {loadFailed ? (
        <div className={styles.placeholder} role="status">
          {LOAD_FAILED_MESSAGE}
        </div>
      ) : !ready ? (
        <div className={styles.placeholder} role="status">
          {READY_PLACEHOLDER}
        </div>
      ) : null}
      <div ref={containerRef} className={styles.container} aria-hidden={!ready} />
    </div>
  );
}
