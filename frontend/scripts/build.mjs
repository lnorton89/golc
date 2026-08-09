#!/usr/bin/env node
// build.mjs runs the frontend's production build: type-check, unit tests,
// and the Vite bundle. `tsc --noEmit` and `vitest run` never depend on
// each other's output, so this runs them concurrently instead of the
// sequential `tsc --noEmit && vitest run && vite build` chain -- cutting
// this step to roughly max(tsc, vitest) instead of their sum, before
// `vite build` (which does need both to have already passed) starts.
// Invokes the checked-in node_modules entrypoints directly, matching
// internal/command/designsystem.go's own direct-binary style rather than
// shelling through `npm run`/`npx`.
//
// This is frontend:build (wails.json) -- the production/CI gate. `wails
// dev`'s own startup compile step uses the separate, test-free
// build-dev.mjs (frontend:dev:build) instead: see that file for why.
import { run, runConcurrently } from "./run-concurrently.mjs";

async function main() {
  await runConcurrently([
    ["tsc --noEmit", ["node_modules/typescript/bin/tsc", "--noEmit"]],
    ["vitest run", ["node_modules/vitest/vitest.mjs", "run"]],
  ]);
  await run("vite build", ["node_modules/vite/bin/vite.js", "build"]).done;
}

main().catch((error) => {
  console.error(error.message);
  process.exit(1);
});
