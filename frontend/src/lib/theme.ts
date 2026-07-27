// theme.ts is a pure client-side preference: which mode
// (light/dark/system) index.css's `[data-theme]` overrides resolve to. It
// never round-trips through a Wails binding (nothing on the Go side owns
// or needs to know the operator's display preference), so it deliberately
// lives outside wailsBridge.ts and useGolcStore (store.ts's own doc
// comment: "cache of Go-pushed snapshots, never authoritative" -- a local
// UI preference isn't Go-pushed data at all).
const STORAGE_KEY = "golc-theme";

export type ThemePreference = "light" | "dark" | "system";

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
