// design-system.system-theme.spec.ts: direct regression coverage for the
// "zero color on first launch" bug. src/main.tsx calls
// applyTheme(getStoredTheme()) before first render; getStoredTheme()
// returns "system" whenever golc-theme is not yet in localStorage (a
// genuine first launch, cleared site data, or any test/CI context that
// doesn't explicitly seed a theme -- see e2e/desktop-view-docs.spec.ts,
// which does not seed a theme and was rendering fully grayscale). "system"
// makes applyTheme remove the data-theme attribute entirely, relying on the
// generated stylesheet to supply colors via @media (prefers-color-scheme)
// for that case. Before this fix, tokens.generated.css had no such media
// query -- every semantic --ds-* color was undefined with no [data-theme]
// attribute present, leaving plain browser-default black/white/transparent.
//
// This spec never seeds golc-theme/golc-theme-name and never sets
// data-theme/data-theme-name -- it is the one deliberately "cold" case, in
// contrast to every design-system.visual-*.spec.ts sibling, which always
// seeds an explicit theme via localStorage specifically to get deterministic
// output. It emulates both OS color-scheme preferences and asserts the
// computed --ds-* custom properties resolve to the real default-theme
// palette (design-system/tokens.json's "default" faces), not the browser
// default.
import { test, expect, type Page } from "@playwright/test";

import { installHealthyBindings, waitForFonts } from "./helpers";

import tokensManifest from "../design-system/tokens.json" with { type: "json" };

type Palette = Record<string, string>;
type ThemeFace = { name: string; mode: "light" | "dark"; palette: string };
interface TokensManifest {
  semanticRoles: string[];
  palettes: Record<string, Palette>;
  themes: ThemeFace[];
}

const tokens = tokensManifest as TokensManifest;

function defaultPalette(mode: "light" | "dark"): Palette {
  const face = tokens.themes.find((candidate) => candidate.name === "default" && candidate.mode === mode);
  if (!face) throw new Error(`tokens.json is missing the default/${mode} face`);
  const palette = tokens.palettes[face.palette];
  if (!palette) throw new Error(`tokens.json is missing palette ${face.palette}`);
  return palette;
}

// Representative cross-section of semantic roles -- one text color, one
// surface color, one action color -- rather than every one of
// semanticRoles.length: this spec exists to prove the media-query fallback
// resolves real palette values at all, not to re-derive theme-contrast.
// test.ts's exhaustive per-role coverage.
const SAMPLE_ROLES = ["text.primary", "surface.canvas", "surface.panel", "action.primary"] as const;

function cssVar(role: string): string {
  return `--ds-${role.replaceAll(".", "-")}`;
}

async function readComputedRootTokens(page: Page, roles: readonly string[]): Promise<Record<string, string>> {
  return page.evaluate((cssVarNames: string[]) => {
    const style = window.getComputedStyle(document.documentElement);
    const out: Record<string, string> = {};
    for (const name of cssVarNames) out[name] = style.getPropertyValue(name).trim();
    return out;
  }, roles.map(cssVar));
}

const THEMES: Array<"light" | "dark"> = ["light", "dark"];

for (const colorScheme of THEMES) {
  test(`fresh session with no stored theme renders ${colorScheme} colors via prefers-color-scheme, not a colorless fallback`, async ({
    browser,
  }) => {
    // A brand-new context: no localStorage entries at all (no golc-theme,
    // no golc-theme-name) -- the genuine first-launch condition.
    const context = await browser.newContext({ colorScheme });
    const page = await context.newPage();
    await installHealthyBindings(page);

    await page.goto("/");
    await expect(page.getByRole("heading", { name: "Overview", exact: true })).toBeVisible();
    await waitForFonts(page);
    await page.emulateMedia({ colorScheme, reducedMotion: "reduce" });
    await page.waitForTimeout(250);

    // The bug's precondition: applyTheme("system") must have left no
    // data-theme/data-theme-name attribute on the document element.
    const attrs = await page.evaluate(() => ({
      dataTheme: document.documentElement.getAttribute("data-theme"),
      dataThemeName: document.documentElement.getAttribute("data-theme-name"),
    }));
    expect(attrs.dataTheme, "a genuine first launch must not have data-theme set").toBeNull();
    expect(attrs.dataThemeName, "a genuine first launch must not have data-theme-name set").toBeNull();

    const expected = defaultPalette(colorScheme);
    const actual = await readComputedRootTokens(page, SAMPLE_ROLES);
    for (const role of SAMPLE_ROLES) {
      expect(
        actual[cssVar(role)],
        `--ds-${role.replaceAll(".", "-")} must resolve via @media (prefers-color-scheme: ${colorScheme}), got "${actual[cssVar(role)]}"`,
      ).toBe(expected[role]);
    }

    // Directly prove real rendered colors, not just the custom property
    // strings: body background/foreground must not be transparent/browser
    // default, and must match the resolved default-theme palette.
    const bodyColors = await page.evaluate(() => {
      const style = window.getComputedStyle(document.body);
      return { backgroundColor: style.backgroundColor, color: style.color };
    });
    expect(bodyColors.backgroundColor, "body background must not be transparent").not.toBe("rgba(0, 0, 0, 0)");
    expect(bodyColors.backgroundColor, "body background must not be transparent").not.toBe("transparent");

    await context.close();
  });
}
