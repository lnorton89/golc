import { createRef } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import Dialog from "./Dialog";

afterEach(() => cleanup());

describe("Dialog", () => {
  it("wires its title and description and focuses the supplied safe action", async () => {
    const safeActionRef = createRef<HTMLButtonElement>();

    render(
      <Dialog open title="Review changes" description="Nothing has been saved yet." initialFocusRef={safeActionRef} onClose={vi.fn()}>
        <button ref={safeActionRef}>Keep editing</button>
        <button>Save changes</button>
      </Dialog>,
    );

    expect(screen.getByRole("dialog", { name: "Review changes" })).toHaveAttribute("aria-modal", "true");
    expect(screen.getByText("Nothing has been saved yet.")).toHaveAttribute("id");
    // Base UI resolves initialFocus asynchronously, not synchronously on mount.
    await waitFor(() => expect(safeActionRef.current).toHaveFocus());
  });

  it("traps Tab navigation and closes on Escape or a backdrop click when allowed", async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    render(
      <Dialog open title="Review changes" onClose={onClose}>
        <button>First action</button>
        <button>Last action</button>
      </Dialog>,
    );

    const first = screen.getByRole("button", { name: "First action" });
    const last = screen.getByRole("button", { name: "Last action" });
    // Base UI resolves its own default initial focus asynchronously; wait
    // for that to settle before manually moving focus, or the async effect
    // can steal focus back after this test's own last.focus() call.
    await waitFor(() => expect(document.activeElement).not.toBe(document.body));
    last.focus();
    // Base UI traps focus with sentinel guard elements around the popup
    // rather than intercepting a synthetic Tab keydown directly, so this
    // needs a real Tab keypress (user-event) to exercise it, not
    // fireEvent.keyDown. Tabbing off the last focusable element first lands
    // on the trailing guard span; the guard's own focusin handler then
    // redirects back to the first focusable element on the next tick.
    await user.tab();
    await waitFor(() => expect(first).toHaveFocus());

    fireEvent.keyDown(screen.getByRole("dialog"), { key: "Escape" });
    // Base UI's outside-press dismissal listens for a click, not a bare
    // mousedown/pointerdown.
    fireEvent.click(screen.getByTestId("dialog-backdrop"));

    expect(onClose).toHaveBeenCalledTimes(2);
  });

  it("does not close on Escape or backdrop click when both are disabled", () => {
    const onClose = vi.fn();
    render(
      <Dialog open title="Review changes" onClose={onClose} closeOnEscape={false} closeOnBackdrop={false}>
        <button>Keep editing</button>
      </Dialog>,
    );

    fireEvent.keyDown(screen.getByRole("dialog"), { key: "Escape" });
    fireEvent.click(screen.getByTestId("dialog-backdrop"));

    expect(onClose).not.toHaveBeenCalled();
  });

  it("restores focus to the invoking control when it closes", async () => {
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

    await waitFor(() => expect(trigger).toHaveFocus());
    trigger.remove();
  });
});
