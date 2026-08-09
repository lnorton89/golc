import { Trash2 } from "lucide-react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import IconButton from "../IconButton/IconButton";
import Menu, { type MenuItem } from "./Menu";

afterEach(() => cleanup());

function renderMenu(items: readonly MenuItem[], ariaLabel = "Row actions") {
  return render(
    <Menu
      trigger={<IconButton icon={Trash2} label="Open row actions" />}
      items={items}
      aria-label={ariaLabel}
    />,
  );
}

describe("Menu", () => {
  it("opens on trigger click and lists every item", async () => {
    const user = userEvent.setup();
    renderMenu([
      { id: "rename", label: "Rename", onSelect: vi.fn() },
      { id: "duplicate", label: "Duplicate", onSelect: vi.fn() },
    ]);

    expect(screen.queryByRole("menu")).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Open row actions" }));

    const menu = await screen.findByRole("menu", { name: "Row actions" });
    expect(menu).toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: "Rename" })).toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: "Duplicate" })).toBeInTheDocument();
  });

  it("opens on Enter when the trigger is focused", async () => {
    const user = userEvent.setup();
    renderMenu([{ id: "rename", label: "Rename", onSelect: vi.fn() }]);

    const trigger = screen.getByRole("button", { name: "Open row actions" });
    trigger.focus();
    await waitFor(() => expect(trigger).toHaveFocus());
    await user.keyboard("{Enter}");

    expect(await screen.findByRole("menu")).toBeInTheDocument();
  });

  it("navigates with ArrowDown/ArrowUp and selects the highlighted item with Enter", async () => {
    const user = userEvent.setup();
    const onSelectRename = vi.fn();
    const onSelectDuplicate = vi.fn();
    renderMenu([
      { id: "rename", label: "Rename", onSelect: onSelectRename },
      { id: "duplicate", label: "Duplicate", onSelect: onSelectDuplicate },
    ]);

    await user.click(screen.getByRole("button", { name: "Open row actions" }));
    await screen.findByRole("menu");

    // Base UI's Menu handles arrow-key composite-list navigation through its
    // own internal keyboard handling, which needs a real keypress
    // (user-event) to exercise -- not a synthetic fireEvent.keyDown.
    await user.keyboard("{ArrowDown}");
    await waitFor(() => expect(screen.getByRole("menuitem", { name: "Rename" })).toHaveFocus());

    await user.keyboard("{ArrowDown}");
    await waitFor(() => expect(screen.getByRole("menuitem", { name: "Duplicate" })).toHaveFocus());

    await user.keyboard("{ArrowUp}");
    await waitFor(() => expect(screen.getByRole("menuitem", { name: "Rename" })).toHaveFocus());

    await user.keyboard("{Enter}");
    await waitFor(() => expect(onSelectRename).toHaveBeenCalledTimes(1));
    expect(onSelectDuplicate).not.toHaveBeenCalled();
    await waitFor(() => expect(screen.queryByRole("menu")).not.toBeInTheDocument());
  });

  it("closes on Escape without selecting anything", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    renderMenu([{ id: "rename", label: "Rename", onSelect }]);

    await user.click(screen.getByRole("button", { name: "Open row actions" }));
    await screen.findByRole("menu");

    await user.keyboard("{Escape}");
    await waitFor(() => expect(screen.queryByRole("menu")).not.toBeInTheDocument());
    expect(onSelect).not.toHaveBeenCalled();
  });

  it("marks a disabled item inert and never fires its onSelect from keyboard or pointer activation", async () => {
    const user = userEvent.setup();
    const onSelectRename = vi.fn();
    const onSelectArchive = vi.fn();
    renderMenu([
      { id: "rename", label: "Rename", onSelect: onSelectRename },
      { id: "archive", label: "Archive", onSelect: onSelectArchive, disabled: true },
    ]);

    await user.click(screen.getByRole("button", { name: "Open row actions" }));
    await screen.findByRole("menu");

    const archiveItem = screen.getByRole("menuitem", { name: "Archive" });
    // Base UI marks a disabled Item via aria-disabled/data-disabled, not the
    // native `disabled` attribute -- it stays in the composite list's
    // roving-tabindex navigation (arrow keys can still land focus on it,
    // unlike a fully removed/skipped item) but ignores activation.
    expect(archiveItem).toHaveAttribute("aria-disabled", "true");

    await user.keyboard("{ArrowDown}");
    await waitFor(() => expect(screen.getByRole("menuitem", { name: "Rename" })).toHaveFocus());
    await user.keyboard("{ArrowDown}");
    await waitFor(() => expect(archiveItem).toHaveFocus());

    await user.keyboard("{Enter}");
    expect(onSelectArchive).not.toHaveBeenCalled();
    // A disabled item ignoring Enter should also leave the menu open.
    expect(screen.getByRole("menu")).toBeInTheDocument();

    await user.click(archiveItem);
    expect(onSelectArchive).not.toHaveBeenCalled();
    expect(onSelectRename).not.toHaveBeenCalled();
  });

  it("clicking an item calls its onSelect and closes the menu", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    renderMenu([
      { id: "rename", label: "Rename", onSelect: vi.fn() },
      { id: "delete", label: "Delete", onSelect, destructive: true },
    ]);

    await user.click(screen.getByRole("button", { name: "Open row actions" }));
    await screen.findByRole("menu");

    await user.click(screen.getByRole("menuitem", { name: "Delete" }));

    expect(onSelect).toHaveBeenCalledTimes(1);
    await waitFor(() => expect(screen.queryByRole("menu")).not.toBeInTheDocument());
  });

  it("renders a destructive item with the destructive visual marker", async () => {
    const user = userEvent.setup();
    renderMenu([
      { id: "rename", label: "Rename", onSelect: vi.fn() },
      { id: "delete", label: "Delete", onSelect: vi.fn(), destructive: true },
    ]);

    await user.click(screen.getByRole("button", { name: "Open row actions" }));
    await screen.findByRole("menu");

    expect(screen.getByRole("menuitem", { name: "Delete" })).toHaveAttribute("data-destructive", "true");
    expect(screen.getByRole("menuitem", { name: "Rename" })).not.toHaveAttribute("data-destructive");
  });
});
