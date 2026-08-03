import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";

import ConfirmDialog from "./ConfirmDialog";

afterEach(() => cleanup());

describe("ConfirmDialog", () => {
  it("focuses the safe cancellation action before a confirmation", () => {
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
    expect(screen.getByRole("button", { name: "Keep mapping" })).toHaveFocus();
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
