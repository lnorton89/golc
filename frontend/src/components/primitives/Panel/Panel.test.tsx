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
});
