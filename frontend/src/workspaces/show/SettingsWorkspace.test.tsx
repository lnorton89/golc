import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";

import SettingsWorkspace from "./SettingsWorkspace";

describe("SettingsWorkspace", () => {
  beforeEach(() => {
    window.localStorage.clear();
    document.documentElement.removeAttribute("data-theme");
  });

  afterEach(() => {
    cleanup();
    window.localStorage.clear();
    document.documentElement.removeAttribute("data-theme");
  });

  it("defaults to Match System selected", () => {
    render(<SettingsWorkspace />);
    expect(screen.getByRole("region", { name: "Settings workspace" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Match System" })).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByRole("button", { name: "Light" })).toHaveAttribute("aria-pressed", "false");
    expect(screen.getByRole("button", { name: "Dark" })).toHaveAttribute("aria-pressed", "false");
  });

  it("selecting Dark applies data-theme and persists across remount", () => {
    render(<SettingsWorkspace />);
    fireEvent.click(screen.getByRole("button", { name: "Dark" }));

    expect(screen.getByRole("button", { name: "Dark" })).toHaveAttribute("aria-pressed", "true");
    expect(document.documentElement.getAttribute("data-theme")).toBe("dark");

    cleanup();
    render(<SettingsWorkspace />);
    expect(screen.getByRole("button", { name: "Dark" })).toHaveAttribute("aria-pressed", "true");
  });

  it("selecting Light then Match System clears the data-theme attribute", () => {
    render(<SettingsWorkspace />);
    fireEvent.click(screen.getByRole("button", { name: "Light" }));
    expect(document.documentElement.getAttribute("data-theme")).toBe("light");

    fireEvent.click(screen.getByRole("button", { name: "Match System" }));
    expect(document.documentElement.hasAttribute("data-theme")).toBe(false);
  });
});
