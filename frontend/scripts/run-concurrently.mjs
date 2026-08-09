// run-concurrently.mjs is the shared process-spawning helper for
// scripts/build.mjs (production) and scripts/build-dev.mjs (wails dev's
// frontend:dev:build) -- both need "run N independent steps at once,
// kill the rest if one fails" and neither should drift from the other's
// definition of that.
import { spawn } from "node:child_process";

export function run(label, args) {
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
export async function runConcurrently(tasks) {
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
