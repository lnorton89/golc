import { defineConfig, devices } from "@playwright/test";

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
  use: {
    baseURL: "http://localhost:4790",
  },
  webServer: {
    command: "npm run dev -- --port 4790 --strictPort",
    port: 4790,
    reuseExistingServer: !process.env.CI,
    timeout: 30_000,
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
});
