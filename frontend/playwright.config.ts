import { defineConfig, devices } from "@playwright/test";
import { existsSync, readFileSync } from "node:fs";
import path from "node:path";

// screenshot-tolerance.json (frontend/design-system/) is Plan 13-17's
// single calibrated global diff tolerance -- "one reviewed threshold"
// (13-17-PLAN.md's key_links), computed from three repeated bounded
// Windows captures rather than an invented or per-test default. Every
// design-system visual spec (Waves 9-13) reads it back out implicitly by
// simply calling expect(page).toHaveScreenshot() with no per-call
// threshold options, inheriting the default wired below. Until Plan
// 13-17's calibration task runs at least once, the file does not exist
// yet, so this falls back to maxDiffPixelRatio: 0 (pixel-exact) -- a
// strict, fail-loud default is safer than silently guessing a lenient
// number nobody has reviewed.
const TOLERANCE_PATH = path.join(import.meta.dirname, "design-system", "screenshot-tolerance.json");

interface ScreenshotTolerance {
  maxDiffPixelRatio: number;
  pixelThreshold: number;
}

function loadScreenshotTolerance(): ScreenshotTolerance {
  if (!existsSync(TOLERANCE_PATH)) {
    return { maxDiffPixelRatio: 0, pixelThreshold: 0.2 };
  }
  const parsed = JSON.parse(readFileSync(TOLERANCE_PATH, "utf-8")) as Partial<ScreenshotTolerance>;
  return {
    maxDiffPixelRatio: parsed.maxDiffPixelRatio ?? 0,
    pixelThreshold: parsed.pixelThreshold ?? 0.2,
  };
}

const screenshotTolerance = loadScreenshotTolerance();

// Real-browser layout coverage: Vitest's jsdom environment never runs an
// actual layout engine (getBoundingClientRect is always zeroed), so a
// responsive bug -- a button's label wrapping and spilling out of its own
// fixed-height box, a crowded toolbar row overflowing past the window
// edge -- is invisible to every existing frontend/src/**/*.test.tsx file
// no matter how it's written. Playwright drives a real Chromium (the
// same engine WebView2 embeds), so e2e/responsive.spec.ts can assert
// actual on-screen geometry at real window widths instead.
//
// Deliberately outside frontend's own `npm test`/`npm run build` chain
// (package.json) and never wired into internal/command/build.go's
// pinned-toolchain pipeline: those need to stay fast and network-free
// (GOFLAGS=-mod=readonly, GOPROXY=off) for every `mage Build`, while a
// real browser suite needs its own downloaded Chromium binary
// (`npx playwright install chromium`, ~300MB) and takes real seconds per
// test. Run explicitly via `npm run test:e2e`.
export default defineConfig({
  testDir: "./e2e",
  fullyParallel: true,
  reporter: "list",
  // Default Playwright naming interpolates {arg}-{projectName}-{platform}{ext}
  // -- this repo only ever runs a single "chromium" project, so the
  // projectName segment is pure noise on every committed baseline filename.
  // Dropping it keeps snapshot filenames platform-qualified (still catches a
  // genuine cross-OS rendering difference) without a redundant, never-varying
  // "-chromium" segment.
  snapshotPathTemplate: "{testDir}/{testFileDir}/{testFileName}-snapshots/{arg}{-snapshotSuffix}{ext}",
  use: {
    baseURL: "http://localhost:4790",
  },
  expect: {
    toHaveScreenshot: {
      maxDiffPixelRatio: screenshotTolerance.maxDiffPixelRatio,
      threshold: screenshotTolerance.pixelThreshold,
      animations: "disabled",
      caret: "hide",
      scale: "css",
      stylePath: path.join(import.meta.dirname, "e2e", "screenshot.css"),
    },
  },
  webServer: {
    command: "npm run dev -- --port 4790 --strictPort",
    port: 4790,
    reuseExistingServer: !process.env.CI,
    timeout: 30_000,
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
});
