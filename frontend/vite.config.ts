import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

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
  // Vitest config lives here (not a separate vitest.config.ts) so it
  // always shares this project's real Vite config (aliases, plugins) --
  // the smoke test must exercise the exact same module graph the actual
  // build produces, including the go:embed'd output path above.
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: ["./src/test/setup.ts"],
  },
});
