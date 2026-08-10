import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  HOTKEY_ACTIONS,
  NAV_ACTIONS,
  beginHotkeyCapture,
  findChordConflict,
  findHotkeyConflict,
  formatChordLabel,
  formatHotkeyLabel,
  getStoredHotkeys,
  getStoredNavHotkeys,
  isHotkeyCaptureActive,
  isModifierKeyEvent,
  normalizeHotkeyChord,
  normalizeHotkeyEvent,
  onHotkeysChanged,
  resetAllHotkeys,
  resetHotkeyBinding,
  resetNavHotkeyBinding,
  setHotkeyBinding,
  setNavHotkeyBinding,
} from "./hotkeys";

describe("hotkeys", () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  afterEach(() => {
    window.localStorage.clear();
  });

  it("defaults every action to its declared defaultKey", () => {
    const bindings = getStoredHotkeys();
    for (const action of HOTKEY_ACTIONS) {
      expect(bindings[action.id]).toBe(action.defaultKey);
    }
  });

  it("persists a rebind and merges it over the defaults on the next read", () => {
    setHotkeyBinding("toggleBaseLook", "z");
    const bindings = getStoredHotkeys();
    expect(bindings.toggleBaseLook).toBe("z");
    expect(bindings.toggleColorTheme).toBe("w");
  });

  it("resets a single binding back to its default", () => {
    setHotkeyBinding("evaluate", "x");
    resetHotkeyBinding("evaluate");
    expect(getStoredHotkeys().evaluate).toBe("Enter");
  });

  it("resets every binding back to defaults", () => {
    setHotkeyBinding("evaluate", "x");
    setHotkeyBinding("tapTempo", "t");
    resetAllHotkeys();
    const bindings = getStoredHotkeys();
    expect(bindings.evaluate).toBe("Enter");
    expect(bindings.tapTempo).toBe("Space");
  });

  it("falls back to defaults when localStorage holds malformed JSON", () => {
    window.localStorage.setItem("golc-hotkeys", "{not json");
    const bindings = getStoredHotkeys();
    expect(bindings.evaluate).toBe("Enter");
  });

  it("notifies subscribers on a change and unsubscribes cleanly", () => {
    const listener = vi.fn();
    const unsubscribe = onHotkeysChanged(listener);

    setHotkeyBinding("bpmUp", "u");
    expect(listener).toHaveBeenCalledTimes(1);

    unsubscribe();
    setHotkeyBinding("bpmUp", "i");
    expect(listener).toHaveBeenCalledTimes(1);
  });

  it("normalizes Space via event.code, letters lowercase, and named keys as-is", () => {
    expect(normalizeHotkeyEvent(new KeyboardEvent("keydown", { code: "Space" }))).toBe("Space");
    expect(normalizeHotkeyEvent(new KeyboardEvent("keydown", { key: "Q" }))).toBe("q");
    expect(normalizeHotkeyEvent(new KeyboardEvent("keydown", { key: "ArrowUp" }))).toBe("ArrowUp");
    expect(normalizeHotkeyEvent(new KeyboardEvent("keydown", { key: "Enter" }))).toBe("Enter");
  });

  it("formats known named keys as glyphs and single characters uppercase", () => {
    expect(formatHotkeyLabel("ArrowUp")).toBe("↑");
    expect(formatHotkeyLabel("ArrowDown")).toBe("↓");
    expect(formatHotkeyLabel("Space")).toBe("Space");
    expect(formatHotkeyLabel("q")).toBe("Q");
    expect(formatHotkeyLabel("Enter")).toBe("Enter");
  });

  it("flags a digit 1-9 as a scene-switch conflict", () => {
    const bindings = getStoredHotkeys();
    expect(findHotkeyConflict(bindings, "evaluate", "5")).toBe("scene-switch");
  });

  it("flags the shell's own reserved help-overlay keys", () => {
    const bindings = getStoredHotkeys();
    // '?' and Escape are hard-coded in useGlobalKeyboardWorkflow, not
    // HOTKEY_ACTIONS entries, so the bindings-vs-bindings check can't see
    // them -- binding a playback action to '?' used to save cleanly and
    // then fire alongside the help overlay on every press.
    expect(findHotkeyConflict(bindings, "evaluate", "?")).toBe("shell-reserved");
    expect(findHotkeyConflict(bindings, "evaluate", "Escape")).toBe("shell-reserved");
  });

  it("tracks rebind-capture suppression with nesting and idempotent release", () => {
    expect(isHotkeyCaptureActive()).toBe(false);
    const releaseOuter = beginHotkeyCapture();
    const releaseInner = beginHotkeyCapture();
    expect(isHotkeyCaptureActive()).toBe(true);

    releaseInner();
    releaseInner();
    expect(isHotkeyCaptureActive()).toBe(true);

    releaseOuter();
    expect(isHotkeyCaptureActive()).toBe(false);
  });

  it("flags a key already bound to another action, but not itself", () => {
    const bindings = getStoredHotkeys();
    expect(findHotkeyConflict(bindings, "toggleColorTheme", "q")).toBe("toggleBaseLook");
    expect(findHotkeyConflict(bindings, "toggleBaseLook", "q")).toBeNull();
  });

  it("reports whether an event.key is a bare modifier press", () => {
    expect(isModifierKeyEvent(new KeyboardEvent("keydown", { key: "Control" }))).toBe(true);
    expect(isModifierKeyEvent(new KeyboardEvent("keydown", { key: "Alt" }))).toBe(true);
    expect(isModifierKeyEvent(new KeyboardEvent("keydown", { key: "Shift" }))).toBe(true);
    expect(isModifierKeyEvent(new KeyboardEvent("keydown", { key: "Meta" }))).toBe(true);
    expect(isModifierKeyEvent(new KeyboardEvent("keydown", { key: "k" }))).toBe(false);
  });

  it("defaults every nav action to its declared defaultChord", () => {
    const bindings = getStoredNavHotkeys();
    for (const action of NAV_ACTIONS) {
      expect(bindings[action.id]).toBe(action.defaultChord);
    }
  });

  it("persists a nav rebind independently of the playback bindings", () => {
    setNavHotkeyBinding("openQuickSwitcher", "Ctrl|Shift|p");
    expect(getStoredNavHotkeys().openQuickSwitcher).toBe("Ctrl|Shift|p");
    expect(getStoredHotkeys().evaluate).toBe("Enter");
  });

  it("resets a single nav binding, and resetAllHotkeys clears both namespaces", () => {
    setNavHotkeyBinding("openSettings", "Alt|s");
    resetNavHotkeyBinding("openSettings");
    expect(getStoredNavHotkeys().openSettings).toBe("Ctrl|,");

    setNavHotkeyBinding("openSettings", "Alt|s");
    setHotkeyBinding("evaluate", "x");
    resetAllHotkeys();
    expect(getStoredNavHotkeys().openSettings).toBe("Ctrl|,");
    expect(getStoredHotkeys().evaluate).toBe("Enter");
  });

  it("normalizes a chord as Ctrl/Alt/Shift (fixed order) plus the base key, and returns null for a bare modifier press", () => {
    expect(normalizeHotkeyChord(new KeyboardEvent("keydown", { key: "k", ctrlKey: true }))).toBe("Ctrl|k");
    expect(
      normalizeHotkeyChord(new KeyboardEvent("keydown", { key: "ArrowUp", ctrlKey: true, altKey: true })),
    ).toBe("Ctrl|Alt|ArrowUp");
    expect(normalizeHotkeyChord(new KeyboardEvent("keydown", { key: "Control", ctrlKey: true }))).toBeNull();
  });

  it("formats a chord's modifiers plus the trailing key via formatHotkeyLabel", () => {
    expect(formatChordLabel("Ctrl|k")).toBe("Ctrl + K");
    expect(formatChordLabel("Ctrl|Alt|ArrowUp")).toBe("Ctrl + Alt + ↑");
    expect(formatChordLabel("Alt|ArrowDown")).toBe("Alt + ↓");
  });

  it("flags a chord already bound to another nav action, but not itself", () => {
    const bindings = getStoredNavHotkeys();
    expect(findChordConflict(bindings, "openSettings", "Ctrl|k")).toBe("openQuickSwitcher");
    expect(findChordConflict(bindings, "openQuickSwitcher", "Ctrl|k")).toBeNull();
  });
});
