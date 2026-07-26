// monacoTheme.ts (08-11-PLAN.md Task 2, D-15): builds the two Paper/Ink
// Monaco editor theme definitions -- golc-paper-ink-light/golc-paper-ink-
// dark -- mapping Monaco's token and UI colours onto the exact
// frontend/src/index.css custom-property values (08-UI-SPEC.md "Monaco
// chrome reconciliation"), and exposes resolveThemeName so ScriptEditor.tsx
// (Task 3) can switch the active theme on the same prefers-color-scheme
// signal index.css keys off -- Monaco does not follow the media query on
// its own.
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

// Colour values copied verbatim from frontend/src/index.css's :root block
// (light) and its `@media (prefers-color-scheme: dark)` override block
// (dark) -- never re-derived or approximated, so a future brand-token edit
// to index.css is the only place this mapping can ever drift from.
const LIGHT_COLORS = {
  panel: "#f4f1eb",
  text: "#4a4941",
  muted: "#8a887f",
  line: "#d2ccc0",
  accent: "#1b44d9",
  statusRevoked: "#e23a2e",
  statusArmed: "#c8a24b",
};

// index.css's dark media query only re-declares --page/--panel/--ink/
// --text/--text2/--muted/--line/--accent -- it never re-declares
// --status-revoked/--status-armed, so both stay fixed at their :root
// values in dark mode too (mirrors index.css's own --status-blackout-text
// doc comment: every status-* token is fixed across themes).
const DARK_COLORS = {
  ...LIGHT_COLORS,
  panel: "#1e2027",
  text: "#b7b5ac",
  muted: "#87857d",
  line: "#2e3038",
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
// GolcThemeColors set: base surface (--panel), foreground (--text),
// comments (--muted), selection/active-line (--accent), gutter/rulers
// (--line), error squiggle (--status-revoked), and the debug current-line
// highlight (--status-armed, registered here for 08-12's Task to consume
// once it lands).
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
      // Debug current-line highlight (08-12): --status-armed.
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
 * registered Monaco theme name -- the exact signal index.css's own dark
 * media query keys off, so Monaco's active theme always agrees with the
 * rest of the shell (Monaco does not follow the media query on its own;
 * ScriptEditor.tsx attaches the matchMedia listener and calls this). */
export function resolveThemeName(prefersDark: boolean): string {
  return prefersDark ? GOLC_PAPER_INK_DARK_THEME : GOLC_PAPER_INK_LIGHT_THEME;
}
