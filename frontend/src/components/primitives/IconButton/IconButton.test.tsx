import { createRef } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { Save } from "lucide-react";

import IconButton from "./IconButton";

describe("IconButton", () => {
  afterEach(() => cleanup());

  it("exposes its required accessible label and forwards the button ref", () => {
    const ref = createRef<HTMLButtonElement>();
    render(<IconButton ref={ref} label="Save show" icon={Save} />);

    const button = screen.getByRole("button", { name: "Save show" });
    expect(ref.current).toBe(button);
    expect(button).toHaveAttribute("type", "button");
  });

  it("prevents duplicate dispatch while loading", () => {
    const onClick = vi.fn();
    render(<IconButton label="Save show" icon={Save} loading onClick={onClick} />);

    const button = screen.getByRole("button", { name: "Save show" });
    fireEvent.click(button);
    expect(button).toBeDisabled();
    expect(button).toHaveAttribute("aria-busy", "true");
    expect(onClick).not.toHaveBeenCalled();
  });

  it("rejects an empty accessible label", () => {
    expect(() => render(<IconButton label="" icon={Save} />)).toThrow("accessible label");
  });

  it("defaults disabled to the native attribute", () => {
    render(<IconButton label="Save show" icon={Save} disabled />);
    const button = screen.getByRole("button", { name: "Save show" });
    expect(button).toBeDisabled();
    expect(button).not.toHaveAttribute("aria-disabled");
  });

  it("soft-disables without the native attribute, staying hoverable/focusable", () => {
    const onClick = vi.fn();
    render(<IconButton label="Release" icon={Save} disabled disabledBehavior="soft" onClick={onClick} />);
    const button = screen.getByRole("button", { name: "Release" });
    expect(button).not.toBeDisabled();
    expect(button).toHaveAttribute("aria-disabled", "true");
    button.focus();
    expect(button).toHaveFocus();
  });

  it("defaults to the target size and accepts compact for dense rows", () => {
    const { rerender } = render(<IconButton label="Save show" icon={Save} />);
    expect(screen.getByRole("button", { name: "Save show" }).className).toMatch(/target/);

    rerender(<IconButton label="Save show" icon={Save} size="compact" />);
    expect(screen.getByRole("button", { name: "Save show" }).className).toMatch(/compact/);
  });
});
