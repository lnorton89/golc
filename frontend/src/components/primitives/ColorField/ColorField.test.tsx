import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import ColorField from "./ColorField";
import type { RgbColor } from "./ColorField";

afterEach(() => cleanup());

// react-colorful's own Interactive component reads event.pageX/pageY, which
// jsdom's MouseEvent constructor silently drops (they are not part of the
// standard MouseEventInit dictionary jsdom implements) -- react-colorful's
// own test suite works around this with a FakeMouseEvent that force-assigns
// them as plain own properties after construction, which is the approach
// followed here. getBoundingClientRect is also mocked to a fixed square:
// jsdom never runs layout, so the real implementation always returns all
// zeros, which would make every drag compute as "top-left corner" no matter
// where the fake pointer lands.
class FakeMouseEvent extends MouseEvent {
  constructor(type: string, options: MouseEventInit & { pageX?: number; pageY?: number }) {
    const { pageX, pageY, ...init } = options;
    super(type, init);
    // jsdom's MouseEvent exposes pageX/pageY as read-only getters (unlike
    // the "not implemented at all" assumption react-colorful's own
    // upstream test helper made) -- Object.assign's plain `this.pageX = x`
    // throws against a getter-only accessor, so the property must be
    // redefined outright to make it a real, settable own property.
    Object.defineProperty(this, "pageX", { value: pageX ?? 0, configurable: true });
    Object.defineProperty(this, "pageY", { value: pageY ?? 0, configurable: true });
  }
}

function mockSquareLayout() {
  const original = HTMLElement.prototype.getBoundingClientRect;
  HTMLElement.prototype.getBoundingClientRect = () =>
    ({ width: 200, height: 200, top: 0, left: 0, right: 200, bottom: 200, x: 0, y: 0, toJSON: () => {} }) as DOMRect;
  return () => {
    HTMLElement.prototype.getBoundingClientRect = original;
  };
}

function renderColorField(value: RgbColor, options: { disabled?: boolean; hideLabel?: boolean } = {}) {
  const onValueChange = vi.fn();
  render(
    <ColorField
      label="Red channel"
      value={value}
      onValueChange={onValueChange}
      disabled={options.disabled}
      hideLabel={options.hideLabel}
    />,
  );
  return { onValueChange };
}

describe("ColorField", () => {
  it("renders with its label and a swatch showing the current color", () => {
    renderColorField({ r: 10, g: 20, b: 30 });

    expect(screen.getByText("Red channel")).toBeInTheDocument();
    const swatch = screen.getByRole("button", { name: "Red channel swatch" });
    expect(swatch).toHaveStyle({ backgroundColor: "rgb(10, 20, 30)" });
  });

  it("hides the visible label and swaps in the accessible name on the swatch when hideLabel is set", () => {
    renderColorField({ r: 10, g: 20, b: 30 }, { hideLabel: true });

    expect(screen.queryByText("Red channel")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Red channel" })).toBeInTheDocument();
  });

  it("opens the picker popover when the swatch is clicked, with the R/G/B fields inside", async () => {
    const user = userEvent.setup();
    renderColorField({ r: 10, g: 20, b: 30 });

    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Red channel swatch" }));

    const popup = await screen.findByRole("dialog", { name: "Red channel color picker" });
    expect(popup).toBeInTheDocument();
    expect(screen.getByLabelText("Red channel red channel")).toHaveValue(10);
    expect(screen.getByLabelText("Red channel green channel")).toHaveValue(20);
    expect(screen.getByLabelText("Red channel blue channel")).toHaveValue(30);
  });

  it("dragging react-colorful's own saturation picker updates value via onValueChange", async () => {
    const user = userEvent.setup();
    const restoreLayout = mockSquareLayout();
    try {
      const { onValueChange } = renderColorField({ r: 120, g: 120, b: 120 });

      await user.click(screen.getByRole("button", { name: "Red channel swatch" }));
      await screen.findByRole("dialog", { name: "Red channel color picker" });

      // Documented by react-colorful as the aria-label its Saturation area's
      // Interactive element always carries (_autodocs/architecture.md),
      // independent of react-colorful's own internal class-name nesting.
      const saturation = document.querySelector('[aria-label="Color"]');
      expect(saturation).not.toBeNull();

      fireEvent(saturation as Element, new FakeMouseEvent("mousedown", { bubbles: true, pageX: 150, pageY: 40 }));

      expect(onValueChange).toHaveBeenCalled();
      const [next] = onValueChange.mock.calls.at(-1) as [RgbColor];
      for (const channel of [next.r, next.g, next.b]) {
        expect(channel).toBeGreaterThanOrEqual(0);
        expect(channel).toBeLessThanOrEqual(255);
        expect(Number.isInteger(channel)).toBe(true);
      }
      // Dragging off the initial gray toward the picker's top-right corner
      // must actually move the value -- otherwise this assertion would pass
      // even if the drag handler were silently wired to a no-op.
      expect(next).not.toEqual({ r: 120, g: 120, b: 120 });
    } finally {
      restoreLayout();
    }
  });

  it("typing into a numeric channel field calls onValueChange with only that channel updated", async () => {
    const { onValueChange } = renderColorField({ r: 10, g: 20, b: 30 });

    await userEvent.setup().click(screen.getByRole("button", { name: "Red channel swatch" }));
    await screen.findByRole("dialog", { name: "Red channel color picker" });

    fireEvent.change(screen.getByLabelText("Red channel green channel"), { target: { value: "77" } });
    expect(onValueChange).toHaveBeenLastCalledWith({ r: 10, g: 77, b: 30 });
  });

  it("clamps typed channel values to the 0-255 DMX range", async () => {
    const { onValueChange } = renderColorField({ r: 10, g: 20, b: 30 });

    await userEvent.setup().click(screen.getByRole("button", { name: "Red channel swatch" }));
    await screen.findByRole("dialog", { name: "Red channel color picker" });

    const redField = screen.getByLabelText("Red channel red channel");

    fireEvent.change(redField, { target: { value: "999" } });
    expect(onValueChange).toHaveBeenLastCalledWith({ r: 255, g: 20, b: 30 });

    fireEvent.change(redField, { target: { value: "-40" } });
    expect(onValueChange).toHaveBeenLastCalledWith({ r: 0, g: 20, b: 30 });

    fireEvent.change(redField, { target: { value: "" } });
    expect(onValueChange).toHaveBeenLastCalledWith({ r: 0, g: 20, b: 30 });
  });

  it("does not open the popover when disabled", async () => {
    const user = userEvent.setup();
    renderColorField({ r: 10, g: 20, b: 30 }, { disabled: true });

    await user.click(screen.getByRole("button", { name: "Red channel swatch" }));

    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });
});
