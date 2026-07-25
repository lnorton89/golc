import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";

import CommandRail from "./CommandRail";
import { NAV_GROUPS, type DestinationId } from "./navigation";

describe("CommandRail", () => {
  afterEach(() => cleanup());

  it("renders every group label and every destination button from navigation.ts", () => {
    render(<CommandRail active="show-overview" onSelect={() => {}} />);
    for (const group of NAV_GROUPS) {
      expect(screen.getByText(group.label)).toBeInTheDocument();
      for (const destination of group.destinations) {
        expect(screen.getByRole("button", { name: destination.label })).toBeInTheDocument();
      }
    }
  });

  it("marks the active destination with aria-current", () => {
    render(<CommandRail active="build-scenes-looks" onSelect={() => {}} />);
    expect(screen.getByRole("button", { name: "Scenes & Looks" })).toHaveAttribute("aria-current", "page");
    expect(screen.getByRole("button", { name: "Art-Net" })).not.toHaveAttribute("aria-current");
  });

  it("calls onSelect with the clicked destination's id", () => {
    const onSelect = vi.fn();
    render(<CommandRail active="show-overview" onSelect={onSelect} />);
    fireEvent.click(screen.getByRole("button", { name: "Art-Net" }));
    const expectedId: DestinationId = "output-artnet";
    expect(onSelect).toHaveBeenCalledWith(expectedId);
  });
});
