import { createRef } from "react";
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";

import PanelHeader from "./PanelHeader";

describe("PanelHeader", () => {
  afterEach(() => cleanup());

  it("renders the label", () => {
    render(<PanelHeader label="Looks" />);
    expect(screen.getByText("Looks")).toBeInTheDocument();
  });

  it("does not render an action slot when none is given", () => {
    const { container } = render(<PanelHeader label="Looks" />);
    expect(container.querySelectorAll("button").length).toBe(0);
  });

  it("renders a provided action element", () => {
    render(<PanelHeader label="Looks" action={<button type="button">+ Theme</button>} />);
    expect(screen.getByRole("button", { name: "+ Theme" })).toBeInTheDocument();
  });

  it("forwards native div props and ref while keeping title, metadata, and actions bounded", () => {
    const ref = createRef<HTMLDivElement>();
    render(
      <PanelHeader
        ref={ref}
        label="An intentionally long panel title that remains readable"
        metadata="12 fixtures"
        density="compact"
        aria-label="Fixture pool details"
        action={<button type="button">Edit</button>}
      />,
    );

    const header = screen.getByLabelText("Fixture pool details");
    expect(ref.current).toBe(header);
    expect(header).toHaveAttribute("data-density", "compact");
    expect(screen.getByText("12 fixtures")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Edit" })).toBeInTheDocument();
  });
});
