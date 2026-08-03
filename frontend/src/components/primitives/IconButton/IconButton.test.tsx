import { createRef } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
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
});
