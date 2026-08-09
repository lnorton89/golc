import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { configDefaults } from "vitest/config";

// Wails embeds this project's compiled output directly
// (cmd/golc-desktop/main.go's `//go:embed all:frontend/dist`) -- no Wails
// v2 dynamic AssetsHandler tricks (.planning/research/STACK.md's own
// guidance). Go's `//go:embed` cannot reference a directory outside the
// embedding file's own package tree (no ".." in embed patterns), while
// this repo's convention keeps frontend/ source at the project root, a
// sibling of cmd/golc-desktop/ rather than nested under it (cmd/golc-
// project/main.go's own "sibling cmd/ target" precedent). outDir is
// therefore redirected to land the build output directly inside
// cmd/golc-desktop's own package directory, where the embed directive can
// see it; frontend/dist itself is never produced. Both are already
// covered by .gitignore's generic "dist/" rule.
export default defineConfig({
  plugins: [react()],
  build: {
    outDir: "../cmd/golc-desktop/frontend/dist",
    emptyOutDir: true,
  },
  // Monaco (08-11-PLAN.md Task 2, D-15) ships its editor/TypeScript
  // language-service workers as separate scripts loaded via Vite's native
  // `?worker` import suffix (ScriptEditor.tsx, Task 3) rather than a
  // dedicated Monaco Vite plugin -- monaco-editor@0.55.1's own worker
  // entry files (editor.worker.js, language/typescript/ts.worker.js) are
  // plain ES modules, so `worker: { format: "es" }` is required: Vite's
  // default IIFE/UMD worker format cannot bundle Monaco's own internal
  // `import`/`export` statements. Without this, `vite build` still
  // succeeds but the emitted worker chunk throws at runtime under the
  // go:embed'd production build the same way it would under `vite dev`.
  worker: {
    format: "es",
  },
  // Vitest config lives here (not a separate vitest.config.ts) so it
  // always shares this project's real Vite config (aliases, plugins) --
  // the smoke test must exercise the exact same module graph the actual
  // build produces, including the go:embed'd output path above.
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: ["./src/test/setup.ts"],
    // e2e/ holds Playwright specs (playwright.config.ts, `npm run
    // test:e2e`), not Vitest ones -- excluded here so `npm test`/`vitest
    // run` doesn't try to load @playwright/test's `test.describe` through
    // Vitest's own test runner (a hard error: "Playwright Test did not
    // expect test.describe() to be called here").
    exclude: [...configDefaults.exclude, "e2e/**"],
    // Perf (vitest.dev/guide/improving-performance): `threads` pool is
    // faster than the `forks` default on larger suites and this project has
    // no native Node addons to lose compatibility with. fsModuleCache
    // persists transformed-module output to disk across runs, which matters
    // a lot for this specific workflow -- `npx vitest run` gets re-invoked
    // repeatedly against the same large module graph (Monaco, Tiptap, Base
    // UI, ...) during iterative development. Deliberately NOT setting
    // `isolate: false`: several test files (MidiPanel, MidiLearnToggle,
    // Desk, LiveStatusBar, OperatorSurface.activeSurface) mutate the shared
    // useGolcStore singleton via setState/getState directly, and nothing
    // resets it to a clean baseline between files -- disabling isolation
    // would let one file's leftover store state (e.g. midiLearnMode: true)
    // leak into the next file sharing the same worker, trading speed for
    // real order-dependent flakiness risk in a safety-relevant app.
    pool: "threads",
    experimental: { fsModuleCache: true },
  },
});
