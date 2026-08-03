import { createRef } from "react";
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

  it("renders no info tooltip when none is given", () => {
    const { container } = render(<Toolbar title="Art-Net" />);
    expect(container.querySelectorAll("button").length).toBe(0);
  });

  it("renders an info tooltip trigger when info is given, without changing the heading's accessible name", () => {
    render(<Toolbar title="Art-Net" info="Configures the Art-Net output path." />);
    expect(screen.getByRole("heading", { name: "Art-Net" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "How Art-Net works" })).toBeInTheDocument();
  });

  it("forwards native props and ref while exposing compact toolbar density", () => {
    const ref = createRef<HTMLDivElement>();
    render(<Toolbar ref={ref} title="A very long workspace title that must remain bounded" density="compact" aria-label="Workspace toolbar" />);

    const toolbar = screen.getByLabelText("Workspace toolbar");
    expect(ref.current).toBe(toolbar);
    expect(toolbar).toHaveAttribute("data-density", "compact");
  });
});
