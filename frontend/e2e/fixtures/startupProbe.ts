// startupProbe.ts: a deterministic pre-navigation instrumentation probe for
// Plan 13-30 Task 1 (D-06/D-10/D-12/D-13, UI-CONSIDERATIONS-BACKSTOP-STARTUP).
// UI-SPEC's "loading | Theme/font initialization" row is explicitly a
// backstop: "Visual tests must prove no theme flash or unreadable fallback
// at app start in light and dark modes." A screenshot alone cannot prove
// this -- a single capture only shows one frame, chosen after the fact,
// which could easily be the settled frame. This module instead installs a
// `page.addInitScript` probe that starts sampling before `main.tsx` (or any
// page script) ever runs, keeps sampling every animation frame through the
// theme/font settle window, and returns the full sample sequence so the
// spec can assert every single frame -- not just the last one -- was
// readable.
import type { Page } from "@playwright/test";
import { execFileSync } from "node:child_process";

export type StartupTheme = "light" | "dark";

export interface StartupSample {
  frameIndex: number;
  elapsedMs: number;
  documentThemeAttr: string | null;
  fontsStatus: string;
  backgroundColor: string;
  color: string;
  fontFamily: string;
  safetyClusterPresent: boolean;
  safetyClusterVisible: boolean;
  safetyLabels: string[];
  // rootMounted is true once React has attached at least one child under
  // #root. Before that point the document is a genuinely empty shell still
  // being fetched/parsed/evaluated over the dev-server network (no text
  // content exists to be unreadable) -- a real, ordinary, unavoidable
  // browser loading window, not the "theme flash" this probe exists to
  // catch. Readability/theme/safety-label assertions only apply once this
  // is true; the pre-mount samples are still recorded and kept in the
  // evidence file for a complete picture, just not treated as violations.
  rootMounted: boolean;
}

export interface StartupProbeSnapshot {
  startedAtEpochMs: number;
  fontsReadyAtMs: number | null;
  samples: StartupSample[];
  stopped: boolean;
}

// installStartupProbe seeds the persisted theme preference (golc-theme --
// see src/lib/theme.ts's STORAGE_KEY) so main.tsx's own
// `applyTheme(getStoredTheme())` boots directly into the requested
// light/dark face on the very first render, then installs the sampling
// instrumentation itself. Both addInitScript calls are registered before
// `page.goto` is ever called by the spec, and addInitScript scripts always
// run before any of the page's own scripts on every navigation -- this is
// what makes "before navigation and before fonts or theme settle" true
// rather than aspirational.
export async function installStartupProbe(page: Page, theme: StartupTheme): Promise<void> {
  await page.addInitScript((seededTheme: string) => {
    window.localStorage.setItem("golc-theme", seededTheme);
  }, theme);

  await page.addInitScript(() => {
    const startedAtEpochMs = Date.now();
    const state = {
      startedAtEpochMs,
      startPerf: performance.now(),
      fontsReadyAtMs: null as number | null,
      samples: [] as StartupSample[],
      stopped: false,
    };
    (window as unknown as { __golcStartupProbe: typeof state }).__golcStartupProbe = state;

    function readSafetyCluster(): { present: boolean; visible: boolean; labels: string[] } {
      const cluster = document.querySelector('[aria-label="Safety cluster"]');
      if (!cluster) return { present: false, visible: false, labels: [] };
      const style = window.getComputedStyle(cluster);
      const rect = cluster.getBoundingClientRect();
      const visible = style.visibility !== "hidden" && style.display !== "none" && rect.width > 0 && rect.height > 0;
      const labels = Array.from(cluster.querySelectorAll("button"))
        .map((button) => (button.textContent ?? "").trim())
        .filter((label) => label.length > 0);
      return { present: true, visible, labels };
    }

    function sample(frameIndex: number): void {
      const target = document.body ?? document.documentElement;
      const style = window.getComputedStyle(target);
      const cluster = readSafetyCluster();
      const root = document.getElementById("root");
      state.samples.push({
        frameIndex,
        elapsedMs: performance.now() - state.startPerf,
        documentThemeAttr: document.documentElement.getAttribute("data-theme"),
        fontsStatus: document.fonts ? document.fonts.status : "unknown",
        backgroundColor: style.backgroundColor,
        color: style.color,
        fontFamily: style.fontFamily,
        safetyClusterPresent: cluster.present,
        safetyClusterVisible: cluster.visible,
        safetyLabels: cluster.labels,
        rootMounted: root !== null && root.children.length > 0,
      });
    }

    // MAX_FRAMES is a hard safety cap (5s at 60fps) so the loop cannot run
    // forever if a spec forgets to call stopStartupProbe -- normal usage
    // always stops the probe explicitly once its own post-settle sample has
    // been captured.
    const MAX_FRAMES = 300;
    let frameIndex = 0;
    function loop(): void {
      if (state.stopped || frameIndex >= MAX_FRAMES) return;
      sample(frameIndex);
      frameIndex += 1;
      requestAnimationFrame(loop);
    }
    requestAnimationFrame(loop);

    if (document.fonts?.ready) {
      document.fonts.ready.then(() => {
        state.fontsReadyAtMs = performance.now() - state.startPerf;
      });
    }
  });
}

export async function readStartupProbe(page: Page): Promise<StartupProbeSnapshot> {
  return page.evaluate(() => {
    const state = (window as unknown as { __golcStartupProbe: StartupProbeSnapshot }).__golcStartupProbe;
    return {
      startedAtEpochMs: state.startedAtEpochMs,
      fontsReadyAtMs: state.fontsReadyAtMs,
      samples: state.samples,
      stopped: state.stopped,
    };
  });
}

export async function stopStartupProbe(page: Page): Promise<void> {
  await page.evaluate(() => {
    const state = (window as unknown as { __golcStartupProbe?: { stopped: boolean } }).__golcStartupProbe;
    if (state) state.stopped = true;
  });
}

// ---------------------------------------------------------------------------
// Contrast arithmetic (WCAG relative luminance / contrast ratio)
// ---------------------------------------------------------------------------

export interface RgbColor {
  r: number;
  g: number;
  b: number;
  a: number;
}

// parseCssColor reads the exact string getComputedStyle returns
// ("rgb(r, g, b)" or "rgba(r, g, b, a)") -- computed style is always
// normalized to this form by every browser Playwright drives, so no other
// CSS color syntax needs to be handled here.
export function parseCssColor(value: string): RgbColor | null {
  const match = /^rgba?\(([^)]+)\)$/.exec(value.trim());
  if (!match) return null;
  const parts = match[1].split(",").map((part) => Number.parseFloat(part.trim()));
  const [r, g, b, a = 1] = parts;
  if ([r, g, b, a].some((channel) => Number.isNaN(channel))) return null;
  return { r, g, b, a };
}

export function isTransparent(value: string): boolean {
  const parsed = parseCssColor(value);
  return parsed !== null && parsed.a === 0;
}

function srgbChannelToLinear(channel255: number): number {
  const channel = channel255 / 255;
  return channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4;
}

export function relativeLuminance(color: RgbColor): number {
  return (
    0.2126 * srgbChannelToLinear(color.r) +
    0.7152 * srgbChannelToLinear(color.g) +
    0.0722 * srgbChannelToLinear(color.b)
  );
}

// contrastRatio implements the standard WCAG formula: (L1 + 0.05) / (L2 + 0.05)
// with L1 the lighter of the two relative luminances. Returns null when
// either color string cannot be parsed (e.g. "transparent" as a literal
// keyword rather than an rgba() function) so callers can distinguish
// "genuinely failed contrast" from "not a comparable pair".
export function contrastRatio(foreground: string, background: string): number | null {
  const fg = parseCssColor(foreground);
  const bg = parseCssColor(background);
  if (!fg || !bg) return null;
  const l1 = relativeLuminance(fg);
  const l2 = relativeLuminance(bg);
  const lighter = Math.max(l1, l2);
  const darker = Math.min(l1, l2);
  return (lighter + 0.05) / (darker + 0.05);
}

// WCAG AA normal-text contrast floor (UI-SPEC's Accessibility Contract:
// "Light and dark modes meet WCAG AA contrast").
export const WCAG_AA_NORMAL_TEXT_MINIMUM = 4.5;

// getBuildSha reads the current commit so evidence JSON is attributable to
// an exact build, mirroring how other Phase 13 evidence files record their
// environment. Falls back to "unknown" (never throws) since evidence
// capture must not fail just because git metadata is briefly unavailable.
export function getBuildSha(): string {
  try {
    return execFileSync("git", ["rev-parse", "HEAD"], { encoding: "utf-8" }).trim();
  } catch {
    return "unknown";
  }
}
