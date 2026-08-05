// Nav hover-text is a client-side preference, same storage shape as
// lib/theme.ts -- but unlike theme (a DOM attribute CSS reacts to on its
// own), suppressing it is a JS-level decision inside useTooltip.tsx's own
// open/close logic, so every mounted instance (CommandRail alone mounts
// 15) needs to react the instant the header toggle flips, not just the
// next remount. The listener set below is what makes that instant --
// see hooks/useNavTooltipsEnabled.ts, the useSyncExternalStore consumer.
const STORAGE_KEY = "golc-nav-tooltips-enabled";
const listeners = new Set<() => void>();

export function getStoredNavTooltipsEnabled(): boolean {
  return window.localStorage.getItem(STORAGE_KEY) !== "false";
}

export function setStoredNavTooltipsEnabled(enabled: boolean): void {
  if (enabled) {
    window.localStorage.removeItem(STORAGE_KEY);
  } else {
    window.localStorage.setItem(STORAGE_KEY, "false");
  }
  listeners.forEach((listener) => listener());
}

export function subscribeNavTooltipsEnabled(listener: () => void): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}
