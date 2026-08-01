import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";

import QuickSwitcher from "./QuickSwitcher";

describe("QuickSwitcher", () => {
  afterEach(() => {
    cleanup();
  });

  it("renders nothing when closed", () => {
    render(<QuickSwitcher open={false} onClose={vi.fn()} onNavigate={vi.fn()} />);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("lists every workspace when open with an empty query, focused for typing", () => {
    render(<QuickSwitcher open onClose={vi.fn()} onNavigate={vi.fn()} />);
    expect(screen.getByLabelText("Jump to a workspace")).toHaveFocus();
    expect(screen.getByRole("option", { name: /Overview/ })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: /Art-Net/ })).toBeInTheDocument();
  });

  it("filters results as the query changes", () => {
    render(<QuickSwitcher open onClose={vi.fn()} onNavigate={vi.fn()} />);
    fireEvent.change(screen.getByLabelText("Jump to a workspace"), { target: { value: "midi" } });

    expect(screen.getByRole("option", { name: /MIDI Mapping/ })).toBeInTheDocument();
    expect(screen.queryByRole("option", { name: /Overview/ })).not.toBeInTheDocument();
  });

  it("shows an empty state when nothing matches", () => {
    render(<QuickSwitcher open onClose={vi.fn()} onNavigate={vi.fn()} />);
    fireEvent.change(screen.getByLabelText("Jump to a workspace"), { target: { value: "zzzzz" } });
    expect(screen.getByText("No matching workspace")).toBeInTheDocument();
  });

  it("navigates to the first result and closes on Enter", () => {
    const onNavigate = vi.fn();
    const onClose = vi.fn();
    render(<QuickSwitcher open onClose={onClose} onNavigate={onNavigate} />);

    fireEvent.change(screen.getByLabelText("Jump to a workspace"), { target: { value: "art-net" } });
    fireEvent.keyDown(screen.getByLabelText("Jump to a workspace"), { key: "Enter" });

    expect(onNavigate).toHaveBeenCalledWith("output-artnet");
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("moves the selection with ArrowDown/ArrowUp before committing on Enter", () => {
    const onNavigate = vi.fn();
    render(<QuickSwitcher open onClose={vi.fn()} onNavigate={onNavigate} />);
    const input = screen.getByLabelText("Jump to a workspace");

    fireEvent.change(input, { target: { value: "show" } });
    fireEvent.keyDown(input, { key: "ArrowDown" });
    fireEvent.keyDown(input, { key: "Enter" });

    expect(onNavigate).toHaveBeenCalledWith("show-shows");
  });

  it("closes on Escape without navigating", () => {
    const onNavigate = vi.fn();
    const onClose = vi.fn();
    render(<QuickSwitcher open onClose={onClose} onNavigate={onNavigate} />);

    fireEvent.keyDown(screen.getByLabelText("Jump to a workspace"), { key: "Escape" });

    expect(onClose).toHaveBeenCalledTimes(1);
    expect(onNavigate).not.toHaveBeenCalled();
  });

  it("clicking a result navigates and closes", () => {
    const onNavigate = vi.fn();
    const onClose = vi.fn();
    render(<QuickSwitcher open onClose={onClose} onNavigate={onNavigate} />);

    fireEvent.click(screen.getByRole("option", { name: /Diagnostics/ }));

    expect(onNavigate).toHaveBeenCalledWith("output-diagnostics");
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("clicking the backdrop closes without navigating", () => {
    const onNavigate = vi.fn();
    const onClose = vi.fn();
    const { container } = render(<QuickSwitcher open onClose={onClose} onNavigate={onNavigate} />);

    fireEvent.click(container.firstChild as Element);

    expect(onClose).toHaveBeenCalledTimes(1);
    expect(onNavigate).not.toHaveBeenCalled();
  });
});
