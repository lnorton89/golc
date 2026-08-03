// monacoTheme.ts (08-11-PLAN.md Task 2, D-15): builds the two Paper/Ink
// Monaco editor theme definitions -- golc-paper-ink-light/golc-paper-ink-
// dark -- mapping Monaco's token and UI colours onto GOLC's own semantic
// design-system tokens (08-UI-SPEC.md "Monaco chrome reconciliation"), and
// exposes resolveThemeName so ScriptEditor.tsx (Task 3) can switch the
// active theme on the same prefers-color-scheme signal the rest of the
// shell keys off -- Monaco does not follow the media query on its own.
//
// 13-27-PLAN.md Task 2 (D-04, the must_haves key_link from this file to
// design-system/tokens.generated.ts) re-anchors this mapping onto the
// generated semantic token names below (MONACO_TOKEN_SOURCES /
// MONACO_TOKEN_CSS_VARIABLES) in place of the earlier hand-copied "verbatim
// from index.css" comment -- frontend/src/index.css's own :root block that
// comment referred to no longer exists (Plan 13-08 removed it). Monaco's
// theme API still requires literal hex constants for BOTH registered
// themes up front, and a `:root[data-theme="light"|"dark"]` custom
// property (design-system/tokens.generated.css) can only ever be resolved
// live for whichever ONE mode is currently applied to the real <html>
// element -- so LIGHT_COLORS/DARK_COLORS below stay pinned literal copies
// rather than a runtime getComputedStyle read (this is Monaco's own
// documented "specialized surfaces" carve-out per 13-UI-SPEC.md: it may own
// its own editor-specific color mapping, but must still be derived FROM
// semantic --ds-* tokens, never a blanket exemption). Each entry is
// hand-verified equal to its named token's own light/dark resolution in
// the generated stylesheet's un-suffixed ("default" theme) block;
// monacoTheme.test.ts asserts MONACO_TOKEN_CSS_VARIABLES against the real
// generated semanticTokenCSSVariables mapping, so a renamed or removed
// token is a real test failure here, not a silently stale comment (the
// exact drift this migration exists to fix).
//
// This file imports only monaco-editor's TYPES (`import type`), never its
// runtime module: it builds plain theme-definition data objects, and
// ScriptEditor.tsx is the one place that actually calls
// monaco.editor.defineTheme with them. Keeping this file free of a runtime
// monaco-editor import means monacoTheme.test.ts can exercise these pure
// functions directly under jsdom with no mock required -- unlike
// ScriptEditor.test.tsx, which must vi.mock("monaco-editor") because
// Monaco itself cannot instantiate under jsdom.
import type { editor } from "monaco-editor";

import { semanticTokenCSSVariables, type SemanticTokenName } from "../../design-system/tokens.generated";

/** MONACO_TOKEN_SOURCES documents the exact --ds-* semantic token each
 * Monaco theme role (the GolcThemeColors field of the same name) is pinned
 * to. See this file's own header comment for why the literal hex values
 * below can't be read live via getComputedStyle for both themes at once. */
const MONACO_TOKEN_SOURCES = {
  panel: "surface.control",
  text: "text.secondary",
  muted: "text.muted",
  line: "border.default",
  accent: "action.primary",
  statusRevoked: "status.revoked",
  statusArmed: "status.armed",
} as const satisfies Record<string, SemanticTokenName>;

/** MONACO_TOKEN_CSS_VARIABLES derives each Monaco role's real --ds-*
 * custom-property name from the generated tokens.generated.ts mapping (not
 * a second hand-copied string) -- exported so monacoTheme.test.ts can
 * assert it stays byte-identical to the live semanticTokenCSSVariables
 * table, catching a renamed/removed token as a real test failure. */
export const MONACO_TOKEN_CSS_VARIABLES: Record<keyof typeof MONACO_TOKEN_SOURCES, string> = Object.fromEntries(
  (Object.entries(MONACO_TOKEN_SOURCES) as Array<[keyof typeof MONACO_TOKEN_SOURCES, SemanticTokenName]>).map(
    ([role, tokenName]) => [role, semanticTokenCSSVariables[tokenName]],
  ),
) as Record<keyof typeof MONACO_TOKEN_SOURCES, string>;

// LIGHT_COLORS/DARK_COLORS: literal hex values, hand-verified equal to each
// MONACO_TOKEN_SOURCES entry's own resolution in
// design-system/tokens.generated.css's `:root[data-theme="light"|"dark"]`
// block (the "default" theme, no `data-theme-name` qualifier).
const LIGHT_COLORS = {
  panel: "#f4f1eb", // --ds-surface-control (light)
  text: "#4a4941", // --ds-text-secondary (light)
  muted: "#8a887f", // --ds-text-muted (light)
  line: "#d2ccc0", // --ds-border-default (light)
  accent: "#1b44d9", // --ds-action-primary (fixed across light/dark)
  statusRevoked: "#e23a2e", // --ds-status-revoked (fixed across light/dark)
  statusArmed: "#c8a24b", // --ds-status-armed (fixed across light/dark)
};

// tokens.generated.css's dark block only re-declares --ds-surface-*/
// --ds-text-*/--ds-border-* -- --ds-status-* stays fixed across modes
// (mirrors every status token's own "fixed across themes" convention), so
// only panel/text/muted/line differ here.
const DARK_COLORS = {
  ...LIGHT_COLORS,
  panel: "#1e2027", // --ds-surface-control (dark)
  text: "#b7b5ac", // --ds-text-secondary (dark)
  muted: "#87857d", // --ds-text-muted (dark)
  line: "#2e3038", // --ds-border-default (dark)
};

type GolcThemeColors = typeof LIGHT_COLORS;

// alpha appends a two-digit hex alpha suffix to a "#rrggbb" colour, for the
// translucent line-highlight/selection/debug-line overlays below -- Monaco
// renders these as overlays on top of editor.background, so a fully opaque
// fill would hide the text underneath entirely.
function alpha(hex: string, twoDigitHexAlpha: string): string {
  return `${hex}${twoDigitHexAlpha}`;
}

// themeFromColors builds one monaco.editor.IStandaloneThemeData from a
// GolcThemeColors set: base surface (--ds-surface-control), foreground
// (--ds-text-secondary), comments (--ds-text-muted), selection/active-line
// (--ds-action-primary), gutter/rulers (--ds-border-default), error
// squiggle (--ds-status-revoked), and the debug current-line highlight
// (--ds-status-armed, registered here for 08-12's Task to consume once it
// lands) -- see MONACO_TOKEN_SOURCES above for the exact per-field mapping.
function themeFromColors(base: editor.BuiltinTheme, colors: GolcThemeColors): editor.IStandaloneThemeData {
  return {
    base,
    inherit: true,
    rules: [{ token: "comment", foreground: colors.muted.replace("#", "") }],
    colors: {
      "editor.background": colors.panel,
      "editor.foreground": colors.text,
      "editorLineNumber.foreground": colors.muted,
      "editorLineNumber.activeForeground": colors.text,
      "editor.lineHighlightBackground": alpha(colors.accent, "14"),
      "editor.lineHighlightBorder": "#00000000",
      "editor.selectionBackground": alpha(colors.accent, "40"),
      "editorGutter.background": colors.line,
      "editorRuler.foreground": colors.line,
      "editorError.foreground": colors.statusRevoked,
      "editorError.border": colors.statusRevoked,
      "editorErrorWidget.border": colors.statusRevoked,
      // Debug current-line highlight (08-12): --ds-status-armed.
      "editor.stackFrameHighlightBackground": alpha(colors.statusArmed, "33"),
    },
  };
}

/** buildGolcMonacoThemes returns the two Paper/Ink Monaco theme
 * definitions (light/dark), ready to pass to monaco.editor.defineTheme
 * as "golc-paper-ink-light"/"golc-paper-ink-dark" respectively. */
export function buildGolcMonacoThemes(): {
  light: editor.IStandaloneThemeData;
  dark: editor.IStandaloneThemeData;
} {
  return {
    light: themeFromColors("vs", LIGHT_COLORS),
    dark: themeFromColors("vs-dark", DARK_COLORS),
  };
}

/** GOLC_PAPER_INK_LIGHT_THEME/GOLC_PAPER_INK_DARK_THEME are the exact
 * theme names ScriptEditor.tsx registers via monaco.editor.defineTheme and
 * resolveThemeName below resolves to. */
export const GOLC_PAPER_INK_LIGHT_THEME = "golc-paper-ink-light";
export const GOLC_PAPER_INK_DARK_THEME = "golc-paper-ink-dark";

/** resolveThemeName maps the prefers-color-scheme boolean onto the
 * registered Monaco theme name -- the same signal the rest of the shell
 * keys off, so Monaco's active theme always agrees with it (Monaco does
 * not follow the media query on its own; ScriptEditor.tsx attaches the
 * matchMedia listener and calls this). */
export function resolveThemeName(prefersDark: boolean): string {
  return prefersDark ? GOLC_PAPER_INK_DARK_THEME : GOLC_PAPER_INK_LIGHT_THEME;
}
