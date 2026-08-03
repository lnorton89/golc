import { createRef } from "react";
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";

import Panel from "./Panel";

describe("Panel", () => {
  afterEach(() => cleanup());

  it("renders its children inside a section", () => {
    render(<Panel>hello</Panel>);
    const section = screen.getByText("hello");
    expect(section.closest("section")).toBeInTheDocument();
  });

  it("merges a custom className with its own", () => {
    render(<Panel className="extra">content</Panel>);
    const section = screen.getByText("content").closest("section");
    expect(section?.className).toContain("extra");
  });

  it("forwards arbitrary HTML attributes (e.g. aria-label, aria-busy)", () => {
    render(
      <Panel aria-label="Test panel" aria-busy="true">
        content
      </Panel>,
    );
    const section = screen.getByLabelText("Test panel");
    expect(section).toHaveAttribute("aria-busy", "true");
  });

  it("forwards its section ref and exposes its closed surface variants", () => {
    const ref = createRef<HTMLElement>();
    render(
      <Panel ref={ref} variant="warning" density="compact" aria-label="Patch warning">
        Long fixture pool name
      </Panel>,
    );

    const panel = screen.getByLabelText("Patch warning");
    expect(ref.current).toBe(panel);
    expect(panel).toHaveAttribute("data-variant", "warning");
    expect(panel).toHaveAttribute("data-density", "compact");
  });
});
