import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";

import LayerRow from "./LayerRow";

const looks = [
  { id: "look-1", name: "Warm Wash" },
  { id: "look-2", name: "Cool Wash" },
];

describe("LayerRow", () => {
  afterEach(() => cleanup());

  it("renders the layer's human label and reflects enabled state via aria-pressed", () => {
    render(
      <ul>
        <LayerRow kind="base_look" enabled looks={looks} refId="look-1" onToggle={() => {}} onSelectLook={() => {}} />
      </ul>,
    );
    expect(screen.getByRole("button", { name: "Base Look" })).toHaveAttribute("aria-pressed", "true");
  });

  it("calls onToggle when the toggle button is clicked", () => {
    const onToggle = vi.fn();
    render(
      <ul>
        <LayerRow
          kind="color_theme"
          enabled={false}
          looks={looks}
          refId=""
          onToggle={onToggle}
          onSelectLook={() => {}}
        />
      </ul>,
    );
    fireEvent.click(screen.getByRole("button", { name: "Color Theme" }));
    expect(onToggle).toHaveBeenCalledTimes(1);
  });

  it("lists every look as a select option and calls onSelectLook with the chosen id", () => {
    const onSelectLook = vi.fn();
    render(
      <ul>
        <LayerRow
          kind="chase"
          enabled
          looks={looks}
          refId="look-1"
          onToggle={() => {}}
          onSelectLook={onSelectLook}
        />
      </ul>,
    );
    const select = screen.getByLabelText("Chase look");
    expect(screen.getByRole("option", { name: "Warm Wash" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "Cool Wash" })).toBeInTheDocument();
    fireEvent.change(select, { target: { value: "look-2" } });
    expect(onSelectLook).toHaveBeenCalledWith("look-2");
  });

  it("shows a 'No looks available' placeholder when the look list is empty", () => {
    render(
      <ul>
        <LayerRow kind="motion" enabled={false} looks={[]} refId="" onToggle={() => {}} onSelectLook={() => {}} />
      </ul>,
    );
    expect(screen.getByRole("option", { name: "No looks available" })).toBeInTheDocument();
  });
});
