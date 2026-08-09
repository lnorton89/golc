import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";

import ConfirmDialog from "./ConfirmDialog";

afterEach(() => cleanup());

describe("ConfirmDialog", () => {
  it("focuses the safe cancellation action before a confirmation", async () => {
    render(
      <ConfirmDialog
        open
        title="Remove mapping?"
        message="This mapping will be removed. This cannot be undone."
        cancelLabel="Keep mapping"
        confirmLabel="Remove mapping"
        onCancel={vi.fn()}
        onConfirm={vi.fn()}
      />,
    );

    expect(screen.getByRole("dialog", { name: "Remove mapping?" })).toBeInTheDocument();
    // Base UI resolves initialFocus asynchronously (after its own opening
    // effect/animation-frame settles), not synchronously on mount.
    await waitFor(() => expect(screen.getByRole("button", { name: "Keep mapping" })).toHaveFocus());
  });

  it("uses alertdialog semantics and a destructive confirmation action only when destructive", () => {
    const onConfirm = vi.fn();
    render(
      <ConfirmDialog
        open
        destructive
        title="Remove mapping?"
        message="This mapping will be removed. This cannot be undone."
        onCancel={vi.fn()}
        onConfirm={onConfirm}
      />,
    );

    const dialog = screen.getByRole("alertdialog", { name: "Remove mapping?" });
    fireEvent.click(screen.getByRole("button", { name: "Confirm" }));

    expect(dialog).toHaveAttribute("aria-modal", "true");
    expect(onConfirm).toHaveBeenCalledOnce();
  });

  it("allows alertdialog semantics without the destructive visual variant via the role prop", () => {
    const onConfirm = vi.fn();
    render(
      <ConfirmDialog
        open
        role="alertdialog"
        title="Leave the guide?"
        message="Your progress is kept, and you can resume from Overview later."
        confirmLabel="Leave Guide"
        cancelLabel="Stay in Guide"
        onCancel={vi.fn()}
        onConfirm={onConfirm}
      />,
    );

    expect(screen.getByRole("alertdialog", { name: "Leave the guide?" })).toBeInTheDocument();
    const confirmButton = screen.getByRole("button", { name: "Leave Guide" });
    expect(confirmButton.className).not.toMatch(/destructive/i);
  });

  it("keeps dismissal policy explicit without taking ownership of safety commands", () => {
    const onCancel = vi.fn();
    render(
      <ConfirmDialog
        open
        title="Revoke Automation?"
        message="The safety command remains controlled by its own hold-to-confirm action."
        onCancel={onCancel}
        onConfirm={vi.fn()}
        closeOnBackdrop={false}
        closeOnEscape={false}
      />,
    );

    fireEvent.keyDown(screen.getByRole("dialog"), { key: "Escape" });
    fireEvent.click(screen.getByRole("dialog"));

    expect(onCancel).not.toHaveBeenCalled();
  });
});
