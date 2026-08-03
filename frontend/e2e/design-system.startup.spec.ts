// design-system.startup.spec.ts: Plan 13-30 Task 1's backstop for
// UI-SPEC's "loading | Theme/font initialization" row -- "Visual tests must
// prove no theme flash or unreadable fallback at app start in light and
// dark modes." A single post-settle screenshot cannot prove this (it only
// shows the frame chosen after the fact, typically the already-settled
// one). This spec instead reads back startupProbe.ts's continuous
// per-animation-frame sample sequence -- recorded from before `main.tsx`
// ever runs through the font/theme settle window -- and asserts every
// single recorded frame, not just the last one, is readable, on-theme, and
// safety-complete.
import { test, expect } from "@playwright/test";
import { mkdirSync, writeFileSync } from "node:fs";
import path from "node:path";

import { assertNoRuntimeIssues, expectSafetyClusterAvailable, installHealthyBindings, waitForFonts } from "./helpers";
import {
  contrastRatio,
  getBuildSha,
  installStartupProbe,
  isTransparent,
  readStartupProbe,
  stopStartupProbe,
  WCAG_AA_NORMAL_TEXT_MINIMUM,
  type StartupSample,
  type StartupTheme,
} from "./fixtures/startupProbe";

const EVIDENCE_PATH = path.join(
  import.meta.dirname,
  "..",
  "..",
  ".planning",
  "phases",
  "13-unified-ui-design-system-and-automated-enforcement",
  "evidence",
  "startup-theme-font.json",
);

const THEMES: StartupTheme[] = ["light", "dark"];
const VIEWPORT = { width: 1280, height: 720 } as const;

interface ContrastFinding {
  frameIndex: number;
  elapsedMs: number;
  ratio: number | null;
  meetsFloor: boolean;
}

interface ThemeEvidence {
  theme: StartupTheme;
  preSettleSampleCountAtFirstRead: number;
  fontsReadyAtMs: number | null;
  totalSamples: number;
  mountedSamples: number;
  themeSequence: Array<{ frameIndex: number; elapsedMs: number; documentThemeAttr: string | null; rootMounted: boolean }>;
  fontSequence: Array<{
    frameIndex: number;
    elapsedMs: number;
    fontsStatus: string;
    fontFamily: string;
    rootMounted: boolean;
  }>;
  contrastFindings: ContrastFinding[];
  assertions: {
    neverTransparentBackground: boolean;
    neverWrongThemeFrame: boolean;
    neverMissingSafetyLabelsWhenPresent: boolean;
    allSampledFramesMeetContrastFloor: boolean;
    finalFontFamilyIncludesArchivo: boolean;
    finalMonoFontAvailable: boolean;
    finalThemeAttrMatchesPersistedTheme: boolean;
  };
}

function isWrongThemeFrame(sample: StartupSample, theme: StartupTheme): boolean {
  const opposite: StartupTheme = theme === "light" ? "dark" : "light";
  return sample.documentThemeAttr === opposite;
}

test(
  "startup theme/font pre-settle backstop: no unreadable frame in light or dark persisted themes",
  async ({ browser }, testInfo) => {
    test.setTimeout(120_000);

    const context = await browser.newContext();
    const themeEvidence: ThemeEvidence[] = [];

    try {
      for (const theme of THEMES) {
        const page = await context.newPage();
        await installHealthyBindings(page);
        await installStartupProbe(page, theme);
        await page.setViewportSize(VIEWPORT);

        await page.goto("/");
        await expect(page.getByRole("heading", { name: "Overview", exact: true })).toBeVisible();

        // Read the probe BEFORE calling the font/theme settle helper below --
        // this is the literal "pre-settle sample window" the plan's own
        // <action> requires: do not call a settle helper until this read has
        // already happened.
        const preSettleSnapshot = await readStartupProbe(page);
        expect(
          preSettleSnapshot.samples.length,
          `${theme}: instrumentation must have recorded at least one sample before waitForFonts is ever called`,
        ).toBeGreaterThan(0);

        await waitForFonts(page);
        await page.emulateMedia({ reducedMotion: "reduce" });
        await page.waitForTimeout(250);
        await assertNoRuntimeIssues(page);
        await expectSafetyClusterAvailable(page);

        const finalFontFamily = await page.evaluate(
          () => window.getComputedStyle(document.body).fontFamily,
        );
        const monoAvailable = await page.evaluate(() => document.fonts.check('14px "JetBrains Mono"'));
        const finalThemeAttr = await page.evaluate(() => document.documentElement.getAttribute("data-theme"));

        await stopStartupProbe(page);
        const finalSnapshot = await readStartupProbe(page);
        await page.close();

        const samples = finalSnapshot.samples;
        // Only samples where React has actually mounted content under #root
        // are eligible for readability/theme/safety-label violations -- the
        // genuinely blank pre-mount window (dev-server module fetch/parse,
        // no text content yet) is an ordinary loading state, not a "theme
        // flash", and is never flagged. See StartupSample.rootMounted's own
        // doc comment.
        const mountedSamples = samples.filter((sample) => sample.rootMounted);
        expect(
          mountedSamples.length,
          `${theme}: at least one sampled frame must show mounted content to evaluate`,
        ).toBeGreaterThan(0);

        const transparentSamples = mountedSamples.filter((sample) => isTransparent(sample.backgroundColor));
        const wrongThemeSamples = mountedSamples.filter((sample) => isWrongThemeFrame(sample, theme));
        const missingSafetyLabelSamples = mountedSamples.filter(
          (sample) => sample.safetyClusterPresent && (!sample.safetyClusterVisible || sample.safetyLabels.length !== 3),
        );

        const contrastFindings: ContrastFinding[] = mountedSamples.map((sample) => {
          const ratio = isTransparent(sample.backgroundColor) || isTransparent(sample.color)
            ? null
            : contrastRatio(sample.color, sample.backgroundColor);
          return {
            frameIndex: sample.frameIndex,
            elapsedMs: Math.round(sample.elapsedMs * 100) / 100,
            ratio: ratio === null ? null : Math.round(ratio * 100) / 100,
            meetsFloor: ratio === null ? true : ratio >= WCAG_AA_NORMAL_TEXT_MINIMUM,
          };
        });
        const contrastFailures = contrastFindings.filter((finding) => !finding.meetsFloor);

        const assertions = {
          neverTransparentBackground: transparentSamples.length === 0,
          neverWrongThemeFrame: wrongThemeSamples.length === 0,
          neverMissingSafetyLabelsWhenPresent: missingSafetyLabelSamples.length === 0,
          allSampledFramesMeetContrastFloor: contrastFailures.length === 0,
          finalFontFamilyIncludesArchivo: finalFontFamily.toLowerCase().includes("archivo"),
          finalMonoFontAvailable: monoAvailable,
          finalThemeAttrMatchesPersistedTheme: finalThemeAttr === theme,
        };

        themeEvidence.push({
          theme,
          preSettleSampleCountAtFirstRead: preSettleSnapshot.samples.length,
          fontsReadyAtMs: finalSnapshot.fontsReadyAtMs,
          totalSamples: samples.length,
          mountedSamples: mountedSamples.length,
          themeSequence: samples.map((sample) => ({
            frameIndex: sample.frameIndex,
            elapsedMs: Math.round(sample.elapsedMs * 100) / 100,
            documentThemeAttr: sample.documentThemeAttr,
            rootMounted: sample.rootMounted,
          })),
          fontSequence: samples.map((sample) => ({
            frameIndex: sample.frameIndex,
            elapsedMs: Math.round(sample.elapsedMs * 100) / 100,
            fontsStatus: sample.fontsStatus,
            fontFamily: sample.fontFamily,
            rootMounted: sample.rootMounted,
          })),
          contrastFindings,
          assertions,
        });

        // Fail loudly per-theme with a specific reason rather than only at
        // the very end -- easier to diagnose which theme and which check
        // regressed.
        expect(transparentSamples, `${theme}: no sampled frame may render a transparent background`).toEqual([]);
        expect(wrongThemeSamples, `${theme}: no sampled frame may show the opposite persisted theme`).toEqual([]);
        expect(
          missingSafetyLabelSamples,
          `${theme}: once the safety cluster is present it must always be visible with exactly 3 labeled controls`,
        ).toEqual([]);
        expect(
          contrastFailures,
          `${theme}: every sampled foreground/background pair must meet the WCAG AA ${WCAG_AA_NORMAL_TEXT_MINIMUM}:1 floor`,
        ).toEqual([]);
        expect(assertions.finalFontFamilyIncludesArchivo, `${theme}: settled body font must be Archivo`).toBe(true);
        expect(assertions.finalMonoFontAvailable, `${theme}: JetBrains Mono must be available after fonts settle`).toBe(
          true,
        );
        expect(
          assertions.finalThemeAttrMatchesPersistedTheme,
          `${theme}: the settled data-theme attribute must match the persisted preference`,
        ).toBe(true);
      }
    } finally {
      await context.close();
    }

    const evidence = {
      schemaVersion: 1,
      capturedAt: new Date().toISOString(),
      buildSha: getBuildSha(),
      environment: {
        platform: process.platform,
        browserName: browser.browserType().name(),
        browserVersion: browser.version(),
        viewport: VIEWPORT,
      },
      wcagAaNormalTextMinimum: WCAG_AA_NORMAL_TEXT_MINIMUM,
      themes: themeEvidence,
    };

    mkdirSync(path.dirname(EVIDENCE_PATH), { recursive: true });
    writeFileSync(EVIDENCE_PATH, `${JSON.stringify(evidence, null, 2)}\n`);

    void testInfo;
  },
);
