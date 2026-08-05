import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { Compass } from "lucide-react";

import NavDestinationButton from "./NavDestinationButton";
import { setStoredNavTooltipsEnabled } from "../lib/navTooltips";
import type { NavDestination } from "./navigation";

const destination: NavDestination = {
  id: "show-overview",
  label: "Overview",
  howItWorks: "Reads the currently open show's state.",
};

describe("NavDestinationButton", () => {
  beforeEach(() => window.localStorage.clear());
  afterEach(() => {
    cleanup();
    window.localStorage.clear();
  });

  it("shows its howItWorks text on hover when nav tooltips are enabled (the default)", () => {
    render(<NavDestinationButton destination={destination} icon={Compass} isActive={false} onSelect={() => {}} />);
    fireEvent.mouseEnter(screen.getByRole("button", { name: "Overview" }));
    expect(screen.getByRole("tooltip")).toHaveTextContent("Reads the currently open show's state.");
  });

  it("stays silent on hover once NavTooltipsToggle has turned nav tooltips off", () => {
    setStoredNavTooltipsEnabled(false);
    render(<NavDestinationButton destination={destination} icon={Compass} isActive={false} onSelect={() => {}} />);
    fireEvent.mouseEnter(screen.getByRole("button", { name: "Overview" }));
    expect(screen.queryByRole("tooltip")).not.toBeInTheDocument();
  });
});
