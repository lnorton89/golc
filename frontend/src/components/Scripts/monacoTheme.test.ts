// monacoTheme.test.ts (08-11-PLAN.md Task 2, re-anchored by 13-27-PLAN.md
// Task 2 onto the generated design-system tokens) exercises
// buildGolcMonacoThemes/resolveThemeName's pure data-shaping behaviour
// directly -- no vi.mock("monaco-editor") is needed here because
// monacoTheme.ts itself only imports monaco-editor's TYPES, never its
// runtime module (see this file's own header comment).
import { describe, expect, it } from "vitest";

import { semanticTokenCSSVariables } from "../../design-system/tokens.generated";
import { buildGolcMonacoThemes, GOLC_PAPER_INK_DARK_THEME, GOLC_PAPER_INK_LIGHT_THEME, MONACO_TOKEN_CSS_VARIABLES, resolveThemeName } from "./monacoTheme";

// Colour values hand-verified against
// design-system/tokens.generated.css's `:root[data-theme="light"|"dark"]`
// block (the "default" theme) -- kept independently here (not imported)
// so this test actually catches a mismatch between the two rather than
// trivially agreeing with itself.
const GENERATED_LIGHT_PANEL = "#f4f1eb"; // --ds-surface-control (light)
const GENERATED_DARK_PANEL = "#1e2027"; // --ds-surface-control (dark)
const GENERATED_LIGHT_TEXT = "#4a4941"; // --ds-text-secondary (light)
const GENERATED_DARK_TEXT = "#b7b5ac"; // --ds-text-secondary (dark)
const GENERATED_ACCENT = "#1b44d9"; // --ds-action-primary
const GENERATED_LIGHT_LINE = "#d2ccc0"; // --ds-border-default (light)
const GENERATED_DARK_LINE = "#2e3038"; // --ds-border-default (dark)

describe("buildGolcMonacoThemes", () => {
  it("returns a light and dark theme definition each keyed by the exact --ds-surface-control background", () => {
    const themes = buildGolcMonacoThemes();

    expect(themes.light.colors["editor.background"]).toBe(GENERATED_LIGHT_PANEL);
    expect(themes.dark.colors["editor.background"]).toBe(GENERATED_DARK_PANEL);
  });

  it("maps foreground, selection, and gutter/rulers onto the exact --ds-text-secondary/--ds-action-primary/--ds-border-default values, per colour scheme", () => {
    const themes = buildGolcMonacoThemes();

    expect(themes.light.colors["editor.foreground"]).toBe(GENERATED_LIGHT_TEXT);
    expect(themes.dark.colors["editor.foreground"]).toBe(GENERATED_DARK_TEXT);

    expect(themes.light.colors["editor.selectionBackground"]).toContain(GENERATED_ACCENT);
    expect(themes.dark.colors["editor.selectionBackground"]).toContain(GENERATED_ACCENT);
    expect(themes.light.colors["editor.lineHighlightBackground"]).toContain(GENERATED_ACCENT);
    expect(themes.dark.colors["editor.lineHighlightBackground"]).toContain(GENERATED_ACCENT);

    expect(themes.light.colors["editorGutter.background"]).toBe(GENERATED_LIGHT_LINE);
    expect(themes.dark.colors["editorGutter.background"]).toBe(GENERATED_DARK_LINE);
  });

  it("uses a distinct Monaco base theme (vs for light, vs-dark for dark)", () => {
    const themes = buildGolcMonacoThemes();

    expect(themes.light.base).toBe("vs");
    expect(themes.dark.base).toBe("vs-dark");
  });
});

describe("MONACO_TOKEN_CSS_VARIABLES", () => {
  it("resolves every Monaco theme role to a real, currently-declared --ds-* semantic token (13-27-PLAN.md D-04 key_link)", () => {
    expect(MONACO_TOKEN_CSS_VARIABLES.panel).toBe(semanticTokenCSSVariables["surface.control"]);
    expect(MONACO_TOKEN_CSS_VARIABLES.text).toBe(semanticTokenCSSVariables["text.secondary"]);
    expect(MONACO_TOKEN_CSS_VARIABLES.muted).toBe(semanticTokenCSSVariables["text.muted"]);
    expect(MONACO_TOKEN_CSS_VARIABLES.line).toBe(semanticTokenCSSVariables["border.default"]);
    expect(MONACO_TOKEN_CSS_VARIABLES.accent).toBe(semanticTokenCSSVariables["action.primary"]);
    expect(MONACO_TOKEN_CSS_VARIABLES.statusRevoked).toBe(semanticTokenCSSVariables["status.revoked"]);
    expect(MONACO_TOKEN_CSS_VARIABLES.statusArmed).toBe(semanticTokenCSSVariables["status.armed"]);
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
