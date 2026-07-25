import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";

import Toolbar from "./Toolbar";

describe("Toolbar", () => {
  afterEach(() => cleanup());

  it("renders the title as a heading", () => {
    render(<Toolbar title="Scenes & Looks" />);
    expect(screen.getByRole("heading", { name: "Scenes & Looks" })).toBeInTheDocument();
  });

  it("renders an action element when provided", () => {
    render(<Toolbar title="Patch & Pools" action={<button type="button">Add</button>} />);
    expect(screen.getByRole("button", { name: "Add" })).toBeInTheDocument();
  });

  it("renders no action when none is given", () => {
    const { container } = render(<Toolbar title="Art-Net" />);
    expect(container.querySelectorAll("button").length).toBe(0);
  });
});
