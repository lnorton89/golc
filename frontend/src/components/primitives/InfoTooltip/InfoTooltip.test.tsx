import { afterEach, describe, expect, it } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

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

  // Driven with userEvent rather than fireEvent.focus/blur: Base UI closes
  // on `focusout`, which bubbles, while fireEvent.blur dispatches the
  // non-bubbling `blur` and so never reached it -- the tooltip stayed open
  // and the assertion failed for a reason that had nothing to do with the
  // component. user.tab() moves real focus and fires the real event pair.
  it("shows the tooltip on keyboard focus and hides it when focus leaves", async () => {
    const user = userEvent.setup();
    render(<InfoTooltip label="How Overview works" text="Reads the currently open show's state." />);
    const trigger = screen.getByRole("button", { name: "How Overview works" });

    await user.tab();
    expect(trigger).toHaveFocus();

    const tooltip = await screen.findByRole("tooltip");
    // The association Base UI does NOT provide on its own (its tooltip
    // parts set no ARIA at all) -- HoverTooltip wires the id and the
    // open-scoped aria-describedby by hand, and this is what proves it.
    expect(trigger).toHaveAttribute("aria-describedby", tooltip.id);
    expect(tooltip.id).not.toBe("");
    // No native `title` attribute: it would fire the browser's own
    // unstyled tooltip on top of this component's styled one.
    expect(trigger).not.toHaveAttribute("title");

    await user.tab();
    expect(trigger).not.toHaveFocus();
    await waitFor(() => expect(screen.queryByRole("tooltip")).not.toBeInTheDocument());
    expect(trigger).not.toHaveAttribute("aria-describedby");
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
