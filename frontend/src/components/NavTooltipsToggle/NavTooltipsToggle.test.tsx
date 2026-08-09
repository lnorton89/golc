import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";

import NavTooltipsToggle from "./NavTooltipsToggle";
import { getStoredNavTooltipsEnabled } from "../../lib/navTooltips";

describe("NavTooltipsToggle", () => {
  beforeEach(() => window.localStorage.clear());
  afterEach(() => {
    cleanup();
    window.localStorage.clear();
  });

  it("renders as on by default", () => {
    render(<NavTooltipsToggle />);
    const toggle = screen.getByRole("button", { name: "Turn off navigation hover text" });
    expect(toggle).toHaveAttribute("aria-pressed", "true");
    expect(toggle).toHaveAttribute("data-active", "true");
  });

  it("turns nav tooltips off and back on when clicked", () => {
    render(<NavTooltipsToggle />);

    fireEvent.click(screen.getByRole("button", { name: "Turn off navigation hover text" }));
    expect(getStoredNavTooltipsEnabled()).toBe(false);
    const off = screen.getByRole("button", { name: "Turn on navigation hover text" });
    expect(off).toHaveAttribute("aria-pressed", "false");
    expect(off).not.toHaveAttribute("data-active");

    fireEvent.click(off);
    expect(getStoredNavTooltipsEnabled()).toBe(true);
    expect(screen.getByRole("button", { name: "Turn off navigation hover text" })).toBeInTheDocument();
  });
});
