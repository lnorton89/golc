import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { getStoredNavTooltipsEnabled, setStoredNavTooltipsEnabled, subscribeNavTooltipsEnabled } from "./navTooltips";

describe("navTooltips", () => {
  beforeEach(() => window.localStorage.clear());
  afterEach(() => window.localStorage.clear());

  it("defaults to enabled when nothing is stored", () => {
    expect(getStoredNavTooltipsEnabled()).toBe(true);
  });

  it("persists an explicit off choice and reflects it on re-read", () => {
    setStoredNavTooltipsEnabled(false);
    expect(getStoredNavTooltipsEnabled()).toBe(false);
  });

  it("clears storage (rather than writing an explicit true) when turned back on", () => {
    setStoredNavTooltipsEnabled(false);
    setStoredNavTooltipsEnabled(true);
    expect(getStoredNavTooltipsEnabled()).toBe(true);
    expect(window.localStorage.getItem("golc-nav-tooltips-enabled")).toBeNull();
  });

  it("notifies subscribers when the preference changes", () => {
    const listener = vi.fn();
    const unsubscribe = subscribeNavTooltipsEnabled(listener);

    setStoredNavTooltipsEnabled(false);
    expect(listener).toHaveBeenCalledTimes(1);

    unsubscribe();
    setStoredNavTooltipsEnabled(true);
    expect(listener).toHaveBeenCalledTimes(1);
  });
});
