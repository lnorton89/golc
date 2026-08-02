// theme.ts is a pure client-side preference: which mode (light/dark/system)
// and which palette (default/gruvbox/tokyo) index.css's `[data-theme]` /
// `[data-theme-name]` overrides resolve to. It never round-trips through a
// Wails binding (nothing on the Go side owns or needs to know the operator's
// display preference), so it deliberately lives outside wailsBridge.ts and
// useGolcStore (store.ts's own doc comment: "cache of Go-pushed snapshots,
// never authoritative" -- a local UI preference isn't Go-pushed data at
// all). Mode and palette are independent axes: mode picks light vs. dark
// values, palette picks which set of values (index.css defines one per
// palette per mode).
const STORAGE_KEY = "golc-theme";
const THEME_NAME_STORAGE_KEY = "golc-theme-name";

export type ThemePreference = "light" | "dark" | "system";
export type ThemeName =
  | "default"
  | "gruvbox"
  | "tokyo"
  | "dracula"
  | "nord"
  | "catppuccin"
  | "solarized"
  | "one-dark"
  | "rose-pine"
  | "everforest"
  | "rainbow"
  | "acid";

const THEME_NAMES: readonly ThemeName[] = [
  "default",
  "gruvbox",
  "tokyo",
  "dracula",
  "nord",
  "catppuccin",
  "solarized",
  "one-dark",
  "rose-pine",
  "everforest",
  "rainbow",
  "acid",
];

export function getStoredTheme(): ThemePreference {
  const stored = window.localStorage.getItem(STORAGE_KEY);
  return stored === "light" || stored === "dark" ? stored : "system";
}

export function applyTheme(theme: ThemePreference): void {
  const root = document.documentElement;
  if (theme === "system") {
    root.removeAttribute("data-theme");
  } else {
    root.setAttribute("data-theme", theme);
  }
}

export function setStoredTheme(theme: ThemePreference): void {
  if (theme === "system") {
    window.localStorage.removeItem(STORAGE_KEY);
  } else {
    window.localStorage.setItem(STORAGE_KEY, theme);
  }
  applyTheme(theme);
}

export function getStoredThemeName(): ThemeName {
  const stored = window.localStorage.getItem(THEME_NAME_STORAGE_KEY);
  return (THEME_NAMES as string[]).includes(stored ?? "") ? (stored as ThemeName) : "default";
}

export function applyThemeName(name: ThemeName): void {
  const root = document.documentElement;
  if (name === "default") {
    root.removeAttribute("data-theme-name");
  } else {
    root.setAttribute("data-theme-name", name);
  }
}

export function setStoredThemeName(name: ThemeName): void {
  if (name === "default") {
    window.localStorage.removeItem(THEME_NAME_STORAGE_KEY);
  } else {
    window.localStorage.setItem(THEME_NAME_STORAGE_KEY, name);
  }
  applyThemeName(name);
}
