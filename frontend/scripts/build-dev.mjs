#!/usr/bin/env node
// build-dev.mjs is wails.json's frontend:dev:build: the command `wails
// dev` runs once at startup ("Compiling frontend: ...") before handing
// off to the live Vite dev server (frontend:dev:watcher's `npm run dev`,
// frontend:dev:serverUrl: "auto"). Without this override, Wails falls
// back to frontend:build -- the full production build, including the
// entire ~1,200-test Vitest suite -- on every single `mage dev` launch,
// even though nothing about starting a dev session needs the test suite
// to have passed: a real bug still shows up immediately in Vite's own
// in-browser error overlay once the app loads, exactly like any other
// Vite-based dev workflow. This runs only a fast type-check and the
// bundle, dropping the part that made "start a dev session" pay for a
// full CI-grade test run every time.
import { run, runConcurrently } from "./run-concurrently.mjs";

async function main() {
  await runConcurrently([
    ["tsc --noEmit", ["node_modules/typescript/bin/tsc", "--noEmit"]],
    ["vite build", ["node_modules/vite/bin/vite.js", "build"]],
  ]);
}

main().catch((error) => {
  console.error(error.message);
  process.exit(1);
});
