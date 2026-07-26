// monacoTheme.test.ts (08-11-PLAN.md Task 2) exercises buildGolcMonacoThemes/
// resolveThemeName's pure data-shaping behaviour directly -- no
// vi.mock("monaco-editor") is needed here because monacoTheme.ts itself
// only imports monaco-editor's TYPES, never its runtime module (see this
// file's own header comment).
import { describe, expect, it } from "vitest";

import { buildGolcMonacoThemes, GOLC_PAPER_INK_DARK_THEME, GOLC_PAPER_INK_LIGHT_THEME, resolveThemeName } from "./monacoTheme";

// Colour values copied verbatim from frontend/src/index.css, exactly as
// monacoTheme.ts's own LIGHT_COLORS/DARK_COLORS constants are -- kept
// independently here (not imported) so this test actually catches a
// mismatch between the two rather than trivially agreeing with itself.
const INDEX_CSS_LIGHT_PANEL = "#f4f1eb";
const INDEX_CSS_DARK_PANEL = "#1e2027";
const INDEX_CSS_LIGHT_TEXT = "#4a4941";
const INDEX_CSS_DARK_TEXT = "#b7b5ac";
const INDEX_CSS_ACCENT = "#1b44d9";
const INDEX_CSS_LIGHT_LINE = "#d2ccc0";
const INDEX_CSS_DARK_LINE = "#2e3038";

describe("buildGolcMonacoThemes", () => {
  it("returns a light and dark theme definition each keyed by the exact index.css --panel background", () => {
    const themes = buildGolcMonacoThemes();

    expect(themes.light.colors["editor.background"]).toBe(INDEX_CSS_LIGHT_PANEL);
    expect(themes.dark.colors["editor.background"]).toBe(INDEX_CSS_DARK_PANEL);
  });

  it("maps foreground, selection, and gutter/rulers onto the exact index.css --text/--accent/--line values, per colour scheme", () => {
    const themes = buildGolcMonacoThemes();

    expect(themes.light.colors["editor.foreground"]).toBe(INDEX_CSS_LIGHT_TEXT);
    expect(themes.dark.colors["editor.foreground"]).toBe(INDEX_CSS_DARK_TEXT);

    expect(themes.light.colors["editor.selectionBackground"]).toContain(INDEX_CSS_ACCENT);
    expect(themes.dark.colors["editor.selectionBackground"]).toContain(INDEX_CSS_ACCENT);
    expect(themes.light.colors["editor.lineHighlightBackground"]).toContain(INDEX_CSS_ACCENT);
    expect(themes.dark.colors["editor.lineHighlightBackground"]).toContain(INDEX_CSS_ACCENT);

    expect(themes.light.colors["editorGutter.background"]).toBe(INDEX_CSS_LIGHT_LINE);
    expect(themes.dark.colors["editorGutter.background"]).toBe(INDEX_CSS_DARK_LINE);
  });

  it("uses a distinct Monaco base theme (vs for light, vs-dark for dark)", () => {
    const themes = buildGolcMonacoThemes();

    expect(themes.light.base).toBe("vs");
    expect(themes.dark.base).toBe("vs-dark");
  });
});

describe("resolveThemeName", () => {
  it("returns golc-paper-ink-dark when prefersDark is true", () => {
    expect(resolveThemeName(true)).toBe(GOLC_PAPER_INK_DARK_THEME);
  });

  it("returns golc-paper-ink-light when prefersDark is false", () => {
    expect(resolveThemeName(false)).toBe(GOLC_PAPER_INK_LIGHT_THEME);
  });
});
