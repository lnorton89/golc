import { themeNames, type DesignSystemThemeName } from "../design-system/tokens.generated";

// Theme selection is a client-side preference. The generated design-system
// manifest owns every selectable face and the CSS semantics it resolves to;
// this module only persists the two selection axes for startup.
const STORAGE_KEY = "golc-theme";
const THEME_NAME_STORAGE_KEY = "golc-theme-name";

export type ThemePreference = "light" | "dark" | "system";
export type ThemeName = DesignSystemThemeName;
export const supportedThemeNames = themeNames;

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
  return stored !== null && supportedThemeNames.includes(stored as ThemeName) ? stored as ThemeName : "default";
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
