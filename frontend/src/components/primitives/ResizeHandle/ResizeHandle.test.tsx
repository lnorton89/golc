import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";

import ResizeHandle from "./ResizeHandle";

describe("ResizeHandle", () => {
  afterEach(() => cleanup());

  it("exposes typed bounds and current geometry to assistive technology", () => {
    render(
      <ResizeHandle
        edge="end"
        label="Resize navigation rail"
        isResizing={false}
        min={160}
        max={360}
        onDoubleClick={vi.fn()}
        onPointerDown={vi.fn()}
        onValueChange={vi.fn()}
        value={186}
      />,
    );

    const separator = screen.getByRole("separator", { name: "Resize navigation rail" });
    expect(separator).toHaveAttribute("aria-valuemin", "160");
    expect(separator).toHaveAttribute("aria-valuemax", "360");
    expect(separator).toHaveAttribute("aria-valuenow", "186");
    expect(separator).toHaveAttribute("tabindex", "0");
  });

  it("changes the value with bounded keyboard increments", () => {
    const onValueChange = vi.fn();
    render(
      <ResizeHandle
        edge="end"
        label="Resize navigation rail"
        isResizing={false}
        min={160}
        max={360}
        onDoubleClick={vi.fn()}
        onPointerDown={vi.fn()}
        onValueChange={onValueChange}
        step={8}
        value={186}
      />,
    );

    const separator = screen.getByRole("separator", { name: "Resize navigation rail" });
    fireEvent.keyDown(separator, { key: "ArrowRight" });
    fireEvent.keyDown(separator, { key: "Home" });
    fireEvent.keyDown(separator, { key: "End" });

    expect(onValueChange).toHaveBeenNthCalledWith(1, 194);
    expect(onValueChange).toHaveBeenNthCalledWith(2, 160);
    expect(onValueChange).toHaveBeenNthCalledWith(3, 360);
  });
});
