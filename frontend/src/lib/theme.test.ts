import { afterEach, beforeEach, describe, expect, it } from "vitest";

import { applyTheme, getStoredTheme, setStoredTheme } from "./theme";

describe("theme", () => {
  beforeEach(() => {
    window.localStorage.clear();
    document.documentElement.removeAttribute("data-theme");
  });

  afterEach(() => {
    window.localStorage.clear();
    document.documentElement.removeAttribute("data-theme");
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
});
