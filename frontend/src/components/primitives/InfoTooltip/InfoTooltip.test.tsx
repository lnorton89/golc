import { afterEach, describe, expect, it } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";

import InfoTooltip from "./InfoTooltip";

describe("InfoTooltip", () => {
  afterEach(() => cleanup());

  it("renders only the trigger until hovered or focused", () => {
    render(<InfoTooltip label="How Overview works" text="Reads the currently open show's state." />);
    expect(screen.getByRole("button", { name: "How Overview works" })).toBeInTheDocument();
    expect(screen.queryByRole("tooltip")).not.toBeInTheDocument();
  });

  it("shows the tooltip text on hover and hides it again on mouse leave", () => {
    render(<InfoTooltip label="How Overview works" text="Reads the currently open show's state." />);
    const trigger = screen.getByRole("button", { name: "How Overview works" });

    fireEvent.mouseEnter(trigger);
    expect(screen.getByRole("tooltip")).toHaveTextContent("Reads the currently open show's state.");

    fireEvent.mouseLeave(trigger);
    expect(screen.queryByRole("tooltip")).not.toBeInTheDocument();
  });

  it("shows the tooltip on keyboard focus and hides it on blur", () => {
    render(<InfoTooltip label="How Overview works" text="Reads the currently open show's state." />);
    const trigger = screen.getByRole("button", { name: "How Overview works" });

    fireEvent.focus(trigger);
    const tooltip = screen.getByRole("tooltip");
    expect(tooltip).toBeInTheDocument();
    expect(trigger).toHaveAttribute("aria-describedby", tooltip.id);
    // No native `title` attribute: it would fire the browser's own
    // unstyled tooltip on top of this component's styled one.
    expect(trigger).not.toHaveAttribute("title");

    fireEvent.blur(trigger);
    expect(screen.queryByRole("tooltip")).not.toBeInTheDocument();
  });

  it("portals the tooltip onto document.body rather than nesting it under the trigger", () => {
    const { container } = render(
      <InfoTooltip label="How Overview works" text="Reads the currently open show's state." />,
    );
    fireEvent.mouseEnter(screen.getByRole("button", { name: "How Overview works" }));

    const tooltip = screen.getByRole("tooltip");
    expect(container.contains(tooltip)).toBe(false);
    expect(document.body.contains(tooltip)).toBe(true);
  });
});
