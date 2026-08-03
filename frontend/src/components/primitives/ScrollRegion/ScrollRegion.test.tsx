import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";

import ScrollRegion from "./ScrollRegion";

describe("ScrollRegion", () => {
  afterEach(() => cleanup());

  it("renders its children", () => {
    render(<ScrollRegion>content</ScrollRegion>);
    expect(screen.getByText("content")).toBeInTheDocument();
  });

  it("merges a custom className with its own", () => {
    render(<ScrollRegion className="custom">content</ScrollRegion>);
    const div = screen.getByText("content");
    expect(div.className).toContain("custom");
  });

  it("keeps scrolling bounded to the requested axis and makes named regions reachable", () => {
    render(
      <ScrollRegion aria-label="Fixture list" direction="vertical">
        content
      </ScrollRegion>,
    );

    const region = screen.getByRole("region", { name: "Fixture list" });
    expect(region).toHaveAttribute("tabindex", "0");
    expect(region).toHaveAttribute("data-scroll-direction", "vertical");
  });
});
