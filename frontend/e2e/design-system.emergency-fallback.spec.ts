// design-system.emergency-fallback.spec.ts: Plan 13-30 Task 2's backstop
// for UI-SPEC's "error | Primitive render failure / error boundary" row --
// "Token-independent emergency fallback remains readable before theme CSS
// is available and exposes a safe recovery action." Blocks the real served
// token stylesheet (see emergencyFallback.ts's own doc comment for why
// that is `/src/index.css`, not a separate tokens.generated.css request),
// forces a deterministic render-time exception via the `?e2e=
// emergency-fallback` fixture route, then asserts ErrorBoundary's fallback
// renders with its exact registered literal colors (not a var(--ds-*)
// value that quietly resolved to nothing), meets WCAG AA contrast, stays
// inside the viewport at both acceptance widths, and exposes a keyboard-
// reachable, activatable Reload action.
import { test, expect } from "@playwright/test";
import { mkdirSync, writeFileSync } from "node:fs";
import path from "node:path";

import { contrastRatio, getBuildSha } from "./fixtures/startupProbe";
import {
  blockGeneratedTokenStylesheet,
  EMERGENCY_FALLBACK_VIEWPORTS,
  EXPECTED_BODY_COLOR,
  EXPECTED_RELOAD_COLOR,
  EXPECTED_SCREEN_BACKGROUND,
  EXPECTED_SCREEN_COLOR,
  EXPECTED_TITLE_COLOR,
  gotoEmergencyFallback,
  type BlockedTokenStylesheetResult,
  type EmergencyFallbackViewportName,
} from "./fixtures/emergencyFallback";

const EVIDENCE_PATH = path.join(
  import.meta.dirname,
  "..",
  "..",
  ".planning",
  "phases",
  "13-unified-ui-design-system-and-automated-enforcement",
  "evidence",
  "error-boundary-fallback.json",
);

const WCAG_AA_NORMAL_TEXT_MINIMUM = 4.5;

interface ViewportEvidence {
  name: EmergencyFallbackViewportName;
  width: number;
  height: number;
  blockedStylesheet: BlockedTokenStylesheetResult;
  computedStyles: {
    screenBackground: string;
    screenColor: string;
    titleColor: string;
    bodyColor: string;
    reloadColor: string;
    sampleTokenAfterBlock: string;
  };
  contrast: {
    screenBackgroundVsScreenColor: number | null;
    screenBackgroundVsTitleColor: number | null;
    screenBackgroundVsBodyColor: number | null;
    screenBackgroundVsReloadColor: number | null;
    minimumRequired: number;
  };
  focus: {
    reachableViaKeyboardTab: boolean;
    outlineStyle: string;
    outlineWidthPx: number;
  };
  activation: {
    reloadedAfterActivation: boolean;
    fallbackReappearedAfterReload: boolean;
  };
  viewportContainment: {
    screenWithinViewport: boolean;
    documentHasNoHorizontalOverflow: boolean;
  };
  assertions: {
    exactRegisteredLiteralsUsed: boolean;
    tokensGenuinelyUnavailable: boolean;
    meetsContrastFloor: boolean;
    keyboardOperable: boolean;
    staysInViewport: boolean;
    safetyReachabilityCopyPresent: boolean;
  };
}

test(
  "emergency fallback is readable, operable, and token-independent before theme CSS",
  async ({ browser }, testInfo) => {
    test.setTimeout(120_000);

    const context = await browser.newContext();
    const viewportEvidence: ViewportEvidence[] = [];

    try {
      for (const viewport of EMERGENCY_FALLBACK_VIEWPORTS) {
        const page = await context.newPage();
        const blockedStylesheet = await blockGeneratedTokenStylesheet(page);
        await page.setViewportSize({ width: viewport.width, height: viewport.height });

        await gotoEmergencyFallback(page);

        const alert = page.getByRole("alert");
        await expect(alert).toBeVisible();
        await expect(page.getByRole("heading", { name: "GOLC hit an unexpected error" })).toBeVisible();
        const safetyReachabilityCopy = page.getByText(
          /Playback, Art-Net output, and safety controls run independently of this window/i,
        );
        await expect(safetyReachabilityCopy).toBeVisible();
        const safetyReachabilityCopyPresent = (await safetyReachabilityCopy.count()) > 0;

        const reloadButton = page.getByRole("button", { name: "Reload" });
        await expect(reloadButton).toBeVisible();
        await expect(reloadButton).toBeEnabled();

        // ---------------------------------------------------------------
        // Computed styles: exact registered literals, and proof tokens are
        // genuinely gone (not merely "we didn't check").
        // ---------------------------------------------------------------
        const computedStyles = await page.evaluate(() => {
          const screen = document.querySelector('[role="alert"]') as HTMLElement;
          const title = screen.querySelector("h1") as HTMLElement;
          const body = screen.querySelector("p") as HTMLElement;
          const reload = screen.querySelector("button") as HTMLElement;
          return {
            screenBackground: window.getComputedStyle(screen).backgroundColor,
            screenColor: window.getComputedStyle(screen).color,
            titleColor: window.getComputedStyle(title).color,
            bodyColor: window.getComputedStyle(body).color,
            reloadColor: window.getComputedStyle(reload).color,
            sampleTokenAfterBlock: window.getComputedStyle(document.documentElement).getPropertyValue("--ds-focus-color"),
          };
        });

        // ---------------------------------------------------------------
        // Contrast arithmetic (WCAG AA normal-text floor, UI-SPEC's
        // Accessibility Contract).
        // ---------------------------------------------------------------
        const contrast = {
          screenBackgroundVsScreenColor: contrastRatio(computedStyles.screenColor, computedStyles.screenBackground),
          screenBackgroundVsTitleColor: contrastRatio(computedStyles.titleColor, computedStyles.screenBackground),
          screenBackgroundVsBodyColor: contrastRatio(computedStyles.bodyColor, computedStyles.screenBackground),
          screenBackgroundVsReloadColor: contrastRatio(computedStyles.reloadColor, computedStyles.screenBackground),
          minimumRequired: WCAG_AA_NORMAL_TEXT_MINIMUM,
        };
        const contrastValues = [
          contrast.screenBackgroundVsScreenColor,
          contrast.screenBackgroundVsTitleColor,
          contrast.screenBackgroundVsBodyColor,
          contrast.screenBackgroundVsReloadColor,
        ];

        // ---------------------------------------------------------------
        // Keyboard reachability + focus-visible. ErrorBoundary's plain
        // <button> deliberately carries no custom :focus-visible rule (see
        // its own doc comment on avoiding shared primitives) -- it relies
        // on the browser's native focus outline, which is itself
        // token-independent by construction. Tab order here is body -> the
        // scrollable stack-trace <pre> (browsers make an overflowing
        // scroll region keyboard-focusable so it can be scrolled with
        // arrow keys -- real, expected accessibility behavior, not a
        // detour) -> the Reload button. Loop with a bounded attempt count
        // rather than assume a fixed number of presses.
        // ---------------------------------------------------------------
        const MAX_TAB_ATTEMPTS = 5;
        let reachedReloadViaTab = false;
        for (let attempt = 0; attempt < MAX_TAB_ATTEMPTS; attempt += 1) {
          await page.keyboard.press("Tab");
          if (await reloadButton.evaluate((element) => element === document.activeElement)) {
            reachedReloadViaTab = true;
            break;
          }
        }
        expect(reachedReloadViaTab, `${viewport.name}: Reload must be reachable via repeated Tab presses`).toBe(true);
        await expect(reloadButton).toBeFocused();
        const focusStyles = await reloadButton.evaluate((element) => {
          const style = window.getComputedStyle(element);
          return { outlineStyle: style.outlineStyle, outlineWidth: style.outlineWidth };
        });
        const outlineWidthPx = Number.parseFloat(focusStyles.outlineWidth) || 0;
        const reachableViaKeyboardTab =
          reachedReloadViaTab && focusStyles.outlineStyle !== "none" && outlineWidthPx > 0;

        // ---------------------------------------------------------------
        // Activation: Enter on the focused Reload button triggers a real
        // reload. The fixture route deterministically re-throws, so the
        // fallback reappearing after the reload proves activation actually
        // fired a recovery attempt rather than merely looking clickable.
        // ---------------------------------------------------------------
        await page.keyboard.press("Enter");
        await page.waitForLoadState("load");
        const reappearedAlert = page.getByRole("alert");
        const fallbackReappearedAfterReload = await reappearedAlert
          .waitFor({ state: "visible", timeout: 10_000 })
          .then(() => true)
          .catch(() => false);

        // ---------------------------------------------------------------
        // Viewport containment (900x720 compact acceptance, 1280x720
        // normal acceptance). Only measured once the fallback is confirmed
        // to have reappeared -- if it never did, containment is vacuously
        // failing rather than crashing on a null element.
        // ---------------------------------------------------------------
        const containment = fallbackReappearedAfterReload
          ? await page.evaluate(
              (vp) => {
                const screen = document.querySelector('[role="alert"]') as HTMLElement;
                const rect = screen.getBoundingClientRect();
                const doc = document.documentElement;
                return {
                  withinViewport:
                    rect.left >= -1 && rect.top >= -1 && rect.right <= vp.width + 1 && rect.bottom <= vp.height + 1,
                  noHorizontalOverflow: doc.scrollWidth <= doc.clientWidth + 1,
                };
              },
              { width: viewport.width, height: viewport.height },
            )
          : { withinViewport: false, noHorizontalOverflow: false };

        await page.close();

        const meetsContrastFloor = contrastValues.every(
          (value) => value !== null && value >= WCAG_AA_NORMAL_TEXT_MINIMUM,
        );
        const exactRegisteredLiteralsUsed =
          computedStyles.screenBackground === EXPECTED_SCREEN_BACKGROUND &&
          computedStyles.screenColor === EXPECTED_SCREEN_COLOR &&
          computedStyles.titleColor === EXPECTED_TITLE_COLOR &&
          computedStyles.bodyColor === EXPECTED_BODY_COLOR &&
          computedStyles.reloadColor === EXPECTED_RELOAD_COLOR;
        const tokensGenuinelyUnavailable = computedStyles.sampleTokenAfterBlock.trim() === "";

        viewportEvidence.push({
          name: viewport.name,
          width: viewport.width,
          height: viewport.height,
          blockedStylesheet,
          computedStyles,
          contrast,
          focus: {
            reachableViaKeyboardTab,
            outlineStyle: focusStyles.outlineStyle,
            outlineWidthPx,
          },
          activation: {
            reloadedAfterActivation: true,
            fallbackReappearedAfterReload,
          },
          viewportContainment: {
            screenWithinViewport: containment.withinViewport,
            documentHasNoHorizontalOverflow: containment.noHorizontalOverflow,
          },
          assertions: {
            exactRegisteredLiteralsUsed,
            tokensGenuinelyUnavailable,
            meetsContrastFloor,
            keyboardOperable: reachableViaKeyboardTab && fallbackReappearedAfterReload,
            staysInViewport: containment.withinViewport && containment.noHorizontalOverflow,
            safetyReachabilityCopyPresent,
          },
        });

        // Fail loudly per-viewport with a specific reason.
        expect(tokensGenuinelyUnavailable, `${viewport.name}: --ds-* custom properties must be genuinely blocked`).toBe(
          true,
        );
        expect(
          exactRegisteredLiteralsUsed,
          `${viewport.name}: fallback must render its exact registered token-independent literals`,
        ).toBe(true);
        expect(meetsContrastFloor, `${viewport.name}: every fallback text pair must meet WCAG AA ${WCAG_AA_NORMAL_TEXT_MINIMUM}:1`).toBe(
          true,
        );
        expect(reachableViaKeyboardTab, `${viewport.name}: Reload must be keyboard-reachable with a visible focus ring`).toBe(
          true,
        );
        expect(
          fallbackReappearedAfterReload,
          `${viewport.name}: activating Reload must trigger a real reload that recovers into the same fallback`,
        ).toBe(true);
        expect(containment.withinViewport, `${viewport.name}: fallback must stay inside the viewport`).toBe(true);
        expect(containment.noHorizontalOverflow, `${viewport.name}: document must not scroll horizontally`).toBe(true);
        expect(safetyReachabilityCopyPresent, `${viewport.name}: safety-reachability copy must remain visible`).toBe(
          true,
        );
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
      },
      wcagAaNormalTextMinimum: WCAG_AA_NORMAL_TEXT_MINIMUM,
      viewports: viewportEvidence,
    };

    mkdirSync(path.dirname(EVIDENCE_PATH), { recursive: true });
    writeFileSync(EVIDENCE_PATH, `${JSON.stringify(evidence, null, 2)}\n`);

    void testInfo;
  },
);
