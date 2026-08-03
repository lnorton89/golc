import { afterEach, beforeEach, describe, expect, it } from "vitest";

import { themeNames } from "../design-system/tokens.generated";
import {
  applyTheme,
  applyThemeName,
  getStoredTheme,
  getStoredThemeName,
  setStoredTheme,
  setStoredThemeName,
  supportedThemeNames,
} from "./theme";

describe("theme", () => {
  beforeEach(() => {
    window.localStorage.clear();
    document.documentElement.removeAttribute("data-theme");
    document.documentElement.removeAttribute("data-theme-name");
  });

  afterEach(() => {
    window.localStorage.clear();
    document.documentElement.removeAttribute("data-theme");
    document.documentElement.removeAttribute("data-theme-name");
  });

  it("defaults to system when nothing is stored", () => {
    expect(getStoredTheme()).toBe("system");
  });

  it("ignores a corrupt/unknown stored value and falls back to system", () => {
    window.localStorage.setItem("golc-theme", "solarized");
    expect(getStoredTheme()).toBe("system");
  });

  it("persists an explicit light/dark choice and reflects it on re-read", () => {
    setStoredTheme("dark");
    expect(getStoredTheme()).toBe("dark");
    expect(document.documentElement.getAttribute("data-theme")).toBe("dark");

    setStoredTheme("light");
    expect(getStoredTheme()).toBe("light");
    expect(document.documentElement.getAttribute("data-theme")).toBe("light");
  });

  it("clears the stored preference and the data-theme attribute when set back to system", () => {
    setStoredTheme("dark");
    setStoredTheme("system");
    expect(getStoredTheme()).toBe("system");
    expect(document.documentElement.hasAttribute("data-theme")).toBe(false);
  });

  it("applyTheme alone does not persist -- a later getStoredTheme call is unaffected", () => {
    applyTheme("dark");
    expect(document.documentElement.getAttribute("data-theme")).toBe("dark");
    expect(getStoredTheme()).toBe("system");
  });

  it("defaults to the default palette when nothing is stored", () => {
    expect(getStoredThemeName()).toBe("default");
  });

  it("uses the generated exhaustive theme list as its palette authority", () => {
    expect(supportedThemeNames).toEqual(themeNames);
  });

  it("ignores a corrupt/unknown stored palette and falls back to default", () => {
    window.localStorage.setItem("golc-theme-name", "commodore64");
    expect(getStoredThemeName()).toBe("default");
  });

  it("persists an explicit palette choice and reflects it on re-read", () => {
    setStoredThemeName("gruvbox");
    expect(getStoredThemeName()).toBe("gruvbox");
    expect(document.documentElement.getAttribute("data-theme-name")).toBe("gruvbox");

    setStoredThemeName("tokyo");
    expect(getStoredThemeName()).toBe("tokyo");
    expect(document.documentElement.getAttribute("data-theme-name")).toBe("tokyo");
  });

  it("accepts every added palette id", () => {
    const names: Array<Parameters<typeof setStoredThemeName>[0]> = [
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
    for (const name of names) {
      setStoredThemeName(name);
      expect(getStoredThemeName()).toBe(name);
      expect(document.documentElement.getAttribute("data-theme-name")).toBe(name);
    }
  });

  it("clears the stored palette and the data-theme-name attribute when set back to default", () => {
    setStoredThemeName("gruvbox");
    setStoredThemeName("default");
    expect(getStoredThemeName()).toBe("default");
    expect(document.documentElement.hasAttribute("data-theme-name")).toBe(false);
  });

  it("applyThemeName alone does not persist -- a later getStoredThemeName call is unaffected", () => {
    applyThemeName("tokyo");
    expect(document.documentElement.getAttribute("data-theme-name")).toBe("tokyo");
    expect(getStoredThemeName()).toBe("default");
  });
});
