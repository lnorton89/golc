import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";

import ScenePad from "./ScenePad";

describe("ScenePad", () => {
  afterEach(() => cleanup());

  it("renders the scene name and calls onLaunch when clicked", () => {
    const onLaunch = vi.fn();
    render(<ScenePad name="Alpha" live={false} locked={false} onLaunch={onLaunch} />);
    fireEvent.click(screen.getByRole("button", { name: "Alpha" }));
    expect(onLaunch).toHaveBeenCalledTimes(1);
  });

  it("shows a LIVE tag and aria-current when live", () => {
    render(<ScenePad name="Alpha" live locked={false} onLaunch={() => {}} />);
    const pad = screen.getByRole("button", { name: "AlphaLIVE" });
    expect(pad).toHaveAttribute("aria-current", "true");
  });

  it("renders as locked, disabled, and never dispatches onLaunch when clicked", () => {
    const onLaunch = vi.fn();
    render(<ScenePad name="Beta" live={false} locked onLaunch={onLaunch} />);
    const pad = screen.getByRole("button", { name: "BetaLocked" });
    expect(pad).toBeDisabled();
    expect(pad).toHaveAttribute("aria-disabled", "true");
    fireEvent.click(pad);
    expect(onLaunch).not.toHaveBeenCalled();
  });
});
