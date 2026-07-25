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
});
