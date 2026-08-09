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
import { spawn } from "node:child_process";

function run(label, args) {
  const child = spawn(process.execPath, args, { stdio: "inherit" });
  const done = new Promise((resolve, reject) => {
    child.on("error", reject);
    child.on("exit", (code, signal) => {
      if (signal) {
        reject(new Error(`${label} killed by signal ${signal}`));
      } else if (code !== 0) {
        reject(new Error(`${label} exited with code ${code}`));
      } else {
        resolve();
      }
    });
  });
  return { child, done };
}

// Runs every task concurrently; if any fails, kills the rest rather than
// leaving them running to completion for no reason (e.g. a fast tsc
// failure would otherwise leave a full vitest run finishing in the
// background after this script has already reported failure and exited).
async function runConcurrently(tasks) {
  const started = tasks.map(([label, args]) => run(label, args));
  try {
    await Promise.all(started.map((task) => task.done));
  } catch (error) {
    for (const { child } of started) {
      if (child.exitCode === null && child.signalCode === null) {
        child.kill();
      }
    }
    throw error;
  }
}

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
