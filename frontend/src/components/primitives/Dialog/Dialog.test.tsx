import { createRef } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";

import Dialog from "./Dialog";

afterEach(() => cleanup());

describe("Dialog", () => {
  it("wires its title and description and focuses the supplied safe action", () => {
    const safeActionRef = createRef<HTMLButtonElement>();

    render(
      <Dialog open title="Review changes" description="Nothing has been saved yet." initialFocusRef={safeActionRef} onClose={vi.fn()}>
        <button ref={safeActionRef}>Keep editing</button>
        <button>Save changes</button>
      </Dialog>,
    );

    expect(screen.getByRole("dialog", { name: "Review changes" })).toHaveAttribute("aria-modal", "true");
    expect(screen.getByText("Nothing has been saved yet.")).toHaveAttribute("id");
    expect(safeActionRef.current).toHaveFocus();
  });

  it("contains Tab navigation and closes on Escape or a backdrop click when allowed", () => {
    const onClose = vi.fn();
    render(
      <Dialog open title="Review changes" onClose={onClose}>
        <button>First action</button>
        <button>Last action</button>
      </Dialog>,
    );

    const dialog = screen.getByRole("dialog");
    const first = screen.getByRole("button", { name: "First action" });
    const last = screen.getByRole("button", { name: "Last action" });
    last.focus();
    fireEvent.keyDown(dialog, { key: "Tab" });
    expect(first).toHaveFocus();
    fireEvent.keyDown(dialog, { key: "Escape" });
    fireEvent.mouseDown(screen.getByTestId("dialog-backdrop"));

    expect(onClose).toHaveBeenCalledTimes(2);
  });

  it("restores focus to the invoking control when it closes", () => {
    const trigger = document.createElement("button");
    trigger.textContent = "Open review";
    document.body.append(trigger);
    trigger.focus();
    const onClose = vi.fn();
    const { rerender } = render(
      <Dialog open title="Review changes" onClose={onClose}>
        <button>Keep editing</button>
      </Dialog>,
    );

    rerender(
      <Dialog open={false} title="Review changes" onClose={onClose}>
        <button>Keep editing</button>
      </Dialog>,
    );

    expect(trigger).toHaveFocus();
    trigger.remove();
  });
});
