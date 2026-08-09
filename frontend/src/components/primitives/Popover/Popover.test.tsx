import { useState } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import Popover from "./Popover";

afterEach(() => cleanup());

describe("Popover", () => {
  it("is closed until the trigger is clicked, then opens and renders arbitrary content", async () => {
    const user = userEvent.setup();
    render(
      <Popover trigger={<button>Open settings</button>} aria-label="Quick settings">
        <label>
          Brightness
          <input type="range" />
        </label>
      </Popover>,
    );

    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Open settings" }));

    const popup = await screen.findByRole("dialog", { name: "Quick settings" });
    expect(popup).toBeInTheDocument();
    expect(screen.getByLabelText("Brightness")).toBeInTheDocument();
  });

  it("closes on Escape", async () => {
    const user = userEvent.setup();
    render(
      <Popover trigger={<button>Open settings</button>} aria-label="Quick settings">
        <p>Content</p>
      </Popover>,
    );

    await user.click(screen.getByRole("button", { name: "Open settings" }));
    await screen.findByRole("dialog", { name: "Quick settings" });

    fireEvent.keyDown(screen.getByRole("dialog"), { key: "Escape" });

    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  });

  it("closes on an outside click", async () => {
    const user = userEvent.setup();
    render(
      <div>
        <button>Outside</button>
        <Popover trigger={<button>Open settings</button>} aria-label="Quick settings">
          <p>Content</p>
        </Popover>
      </div>,
    );

    await user.click(screen.getByRole("button", { name: "Open settings" }));
    await screen.findByRole("dialog", { name: "Quick settings" });

    // Base UI's outside-press dismissal listens for a real click (confirmed
    // on Dialog's own outside-press dismissal), not a bare
    // mousedown/pointerdown -- user.click drives a real click.
    await user.click(screen.getByRole("button", { name: "Outside" }));

    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  });

  it("supports controlled open/onOpenChange", async () => {
    const user = userEvent.setup();
    const onOpenChange = vi.fn();

    function Controlled() {
      const [open, setOpen] = useState(false);
      return (
        <Popover
          trigger={<button>Open settings</button>}
          aria-label="Quick settings"
          open={open}
          onOpenChange={(next) => {
            onOpenChange(next);
            setOpen(next);
          }}
        >
          <p>Content</p>
        </Popover>
      );
    }

    render(<Controlled />);

    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Open settings" }));
    await screen.findByRole("dialog", { name: "Quick settings" });
    expect(onOpenChange).toHaveBeenCalledWith(true);

    fireEvent.keyDown(screen.getByRole("dialog"), { key: "Escape" });
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("stays closed when a controlled `open` prop is not flipped by the caller", async () => {
    const user = userEvent.setup();
    const onOpenChange = vi.fn();

    render(
      <Popover trigger={<button>Open settings</button>} aria-label="Quick settings" open={false} onOpenChange={onOpenChange}>
        <p>Content</p>
      </Popover>,
    );

    await user.click(screen.getByRole("button", { name: "Open settings" }));

    expect(onOpenChange).toHaveBeenCalledWith(true);
    // The caller never re-rendered with open=true, so the popover itself
    // must not have opened on its own -- controlled mode means the caller
    // owns the state entirely.
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("manages its own open state when uncontrolled (no `open` prop)", async () => {
    const user = userEvent.setup();
    render(
      <Popover trigger={<button>Open settings</button>} aria-label="Quick settings">
        <p>Content</p>
      </Popover>,
    );

    await user.click(screen.getByRole("button", { name: "Open settings" }));
    await screen.findByRole("dialog", { name: "Quick settings" });

    await user.click(screen.getByRole("button", { name: "Open settings" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  });
});
