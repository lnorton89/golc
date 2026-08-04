// emergencyFallback.ts: Plan 13-30 Task 2's fixture helper for
// UI-CONSIDERATIONS-BACKSTOP-ERROR ("Token-independent emergency fallback
// remains readable before theme CSS is available and exposes a safe
// recovery action"). ErrorBoundary.tsx's own doc comment explains why its
// seven color/background declarations are bare hex/rgb literals (never
// var(--ds-*)) rather than tokens: the fallback must render correctly even
// if tokens.generated.css itself never loaded, which is exactly what this
// module proves in a real browser rather than merely asserting on source
// code.
//
// In Vite's dev server, index.css's own `@import "./design-system/
// tokens.generated.css"` is inlined server-side into ONE served CSS
// module -- the browser never makes a separate network request for
// tokens.generated.css to intercept. blockGeneratedTokenStylesheet
// therefore intercepts the real served request (`/src/index.css`, the
// module main.tsx actually imports) and rewrites its body, stripping every
// `--ds-*` custom-property *declaration* so every var(--ds-*) reference
// anywhere in the app becomes invalid at computed-value time -- the
// authentic effect of "the generated theme stylesheet is unavailable"
// without breaking ES module resolution (which page.route's abort() would
// do, crashing module evaluation before React ever mounts and making the
// whole scenario untestable).
import type { Page } from "@playwright/test";

export interface BlockedTokenStylesheetResult {
  pathname: string | null;
  originalLength: number;
  strippedLength: number;
  declarationsStripped: number;
}

const TOKEN_DECLARATION_PATTERN = /--ds-[a-zA-Z0-9-]+\s*:\s*[^;]+;/g;

export async function blockGeneratedTokenStylesheet(page: Page): Promise<BlockedTokenStylesheetResult> {
  const result: BlockedTokenStylesheetResult = {
    pathname: null,
    originalLength: 0,
    strippedLength: 0,
    declarationsStripped: 0,
  };

  await page.route("**/*", async (route) => {
    const url = route.request().url();
    let pathname = "";
    try {
      pathname = new URL(url).pathname;
    } catch {
      pathname = "";
    }

    if (pathname.endsWith("/src/index.css")) {
      const response = await route.fetch();
      const original = await response.text();
      const matches = original.match(TOKEN_DECLARATION_PATTERN) ?? [];
      const stripped = original.replace(TOKEN_DECLARATION_PATTERN, "");
      result.pathname = pathname;
      result.originalLength = original.length;
      result.strippedLength = stripped.length;
      result.declarationsStripped = matches.length;
      await route.fulfill({ response, body: stripped });
      return;
    }

    await route.continue();
  });

  return result;
}

export async function gotoEmergencyFallback(page: Page): Promise<void> {
  await page.goto("/?e2e=emergency-fallback");
  await page.waitForSelector('[role="alert"]', { state: "visible" });
}

export const EMERGENCY_FALLBACK_VIEWPORTS = [
  { name: "compact", width: 900, height: 720 },
  { name: "normal", width: 1280, height: 720 },
] as const;

export type EmergencyFallbackViewportName = (typeof EMERGENCY_FALLBACK_VIEWPORTS)[number]["name"];

// EXPECTED_* mirror ErrorBoundary.module.css's registered
// design-system/exceptions.json literals exactly -- the
// spec asserts computed style equals these precise values even with tokens
// blocked, proving the fallback never silently degraded to an unreadable
// default because a var(--ds-*) reference resolved to nothing.
//
// Title/reload use #e54e43 / rgb(229, 78, 67) rather than the canonical
// #e23a2e / rgb(226, 58, 46) revoked red: this spec's own contrast
// measurement found the canonical value only reaches 4.27:1 against this
// screen's #131419 background (below the WCAG AA 4.5:1 normal-text floor).
// This is a minimal, hue-preserving lightening of ErrorBoundary's own
// independent literal (never the shared --ds-status-revoked token used
// elsewhere in the app) that clears the floor -- see
// ErrorBoundary.module.css's own comment on both declarations.
export const EXPECTED_SCREEN_BACKGROUND = "rgb(19, 20, 25)"; // #131419
export const EXPECTED_SCREEN_COLOR = "rgb(228, 224, 216)"; // #e4e0d8
export const EXPECTED_TITLE_COLOR = "rgb(229, 78, 67)"; // #e54e43
export const EXPECTED_BODY_COLOR = "rgb(138, 136, 127)"; // #8a887f
export const EXPECTED_RELOAD_COLOR = "rgb(229, 78, 67)"; // rgb(229, 78, 67) literal in .reload
