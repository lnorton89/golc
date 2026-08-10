import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { StrictMode } from "react";

import SceneList, { reorderSceneNames } from "./SceneList";
import type { ProgSceneView } from "../../lib/wailsBridge";

const scenes: ProgSceneView[] = [
  { name: "Alpha", active: true, barsPerLoop: 4, layers: [] },
  { name: "Beta", active: false, barsPerLoop: 8, layers: [] },
];

const threeScenes: ProgSceneView[] = [
  { name: "Alpha", active: true, barsPerLoop: 4, layers: [] },
  { name: "Beta", active: false, barsPerLoop: 8, layers: [] },
  { name: "Gamma", active: false, barsPerLoop: 2, layers: [] },
];

const noop = () => {};
// onReorder reports whether the server accepted the new order (SceneList
// rolls its optimistic local order back on false); acceptReorder is the
// "server said yes" stub every test that isn't specifically about
// rejection uses.
const acceptReorder = () => true;

// visibleSceneOrder reads each row's "<name> actions" menu-trigger button
// (already used by the existing rename/delete tests above) in DOM order --
// a stable way to read back which scene renders in which position without
// depending on SceneList.module.css's own private class names.
function visibleSceneOrder(): string[] {
  return screen.getAllByRole("button", { name: /actions$/ }).map((button) => {
    const label = button.getAttribute("aria-label") ?? "";
    return label.replace(/ actions$/, "");
  });
}

// pressKey fires a keydown and then lets dnd-kit's own rAF-scheduled rect
// measurement/collision-detection effects (useLayoutEffect-driven, but
// deferred a tick behind the synthetic event itself) flush before the next
// key fires -- without this, ArrowDown/the final Space run against
// not-yet-measured droppable rects and no reorder happens at all.
async function pressKey(target: HTMLElement, code: string): Promise<void> {
  await act(async () => {
    fireEvent.keyDown(target, { code });
    await new Promise((resolve) => requestAnimationFrame(resolve));
  });
}

describe("SceneList", () => {
  afterEach(() => cleanup());

  // dnd-kit's collision detection (closestCenter) and its keyboard
  // coordinate getter both need real, distinct element rects to tell rows
  // apart -- jsdom never computes layout, so every getBoundingClientRect()
  // call returns an all-zero rect by default, which makes every row look
  // like it occupies the exact same position. Stubbing a stacked rect per
  // <li> (ordered by its position among its siblings) gives dnd-kit enough
  // to work with for both the drag-end and keyboard-reorder tests below.
  beforeEach(() => {
    vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockImplementation(function (this: HTMLElement) {
      if (this.tagName === "LI") {
        const siblings = Array.from(this.parentElement?.children ?? []);
        const index = siblings.indexOf(this);
        const rectTop = index * 44;
        return {
          width: 260,
          height: 44,
          top: rectTop,
          left: 0,
          right: 260,
          bottom: rectTop + 44,
          x: 0,
          y: rectTop,
          toJSON: () => ({}),
        } as DOMRect;
      }
      return { width: 0, height: 0, top: 0, left: 0, right: 0, bottom: 0, x: 0, y: 0, toJSON: () => ({}) } as DOMRect;
    });
  });

  it("shows an empty state when there are no scenes", () => {
    render(
      <SceneList scenes={[]} selectedName={null} onSelect={noop} onCreate={noop} onRename={noop} onDelete={noop} onReorder={acceptReorder} />,
    );
    expect(screen.getByText("No scenes yet — create one above.")).toBeInTheDocument();
  });

  it("renders each scene with LIVE or bar-count meta and marks the selected one", () => {
    render(
      <SceneList scenes={scenes} selectedName="Alpha" onSelect={noop} onCreate={noop} onRename={noop} onDelete={noop} onReorder={acceptReorder} />,
    );
    expect(screen.getByRole("button", { name: "AlphaLIVE" })).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByRole("button", { name: "Beta8bar" })).toHaveAttribute("aria-pressed", "false");
  });

  it("calls onSelect with the clicked scene's name", () => {
    const onSelect = vi.fn();
    render(
      <SceneList
        scenes={scenes}
        selectedName="Alpha"
        onSelect={onSelect}
        onCreate={noop}
        onRename={noop}
        onDelete={noop}
        onReorder={acceptReorder}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Beta8bar" }));
    expect(onSelect).toHaveBeenCalledWith("Beta");
  });

  it("toggles the create form open and closed via the New/Cancel button", () => {
    render(
      <SceneList scenes={[]} selectedName={null} onSelect={noop} onCreate={noop} onRename={noop} onDelete={noop} onReorder={acceptReorder} />,
    );
    expect(screen.queryByLabelText("New scene name")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "New" }));
    expect(screen.getByLabelText("New scene name")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(screen.queryByLabelText("New scene name")).not.toBeInTheDocument();
  });

  it("calls onCreate with the trimmed name and parsed bar count, then closes the form", () => {
    const onCreate = vi.fn();
    render(
      <SceneList scenes={[]} selectedName={null} onSelect={noop} onCreate={onCreate} onRename={noop} onDelete={noop} onReorder={acceptReorder} />,
    );

    fireEvent.click(screen.getByRole("button", { name: "New" }));
    fireEvent.change(screen.getByLabelText("New scene name"), { target: { value: "  Gamma  " } });
    fireEvent.change(screen.getByLabelText("Bars per loop"), { target: { value: "16" } });
    fireEvent.click(screen.getByRole("button", { name: "Create" }));

    expect(onCreate).toHaveBeenCalledWith("Gamma", 16);
    expect(screen.queryByLabelText("New scene name")).not.toBeInTheDocument();
  });

  it("does not call onCreate when the name is blank", () => {
    const onCreate = vi.fn();
    render(
      <SceneList scenes={[]} selectedName={null} onSelect={noop} onCreate={onCreate} onRename={noop} onDelete={noop} onReorder={acceptReorder} />,
    );

    fireEvent.click(screen.getByRole("button", { name: "New" }));
    fireEvent.click(screen.getByRole("button", { name: "Create" }));

    expect(onCreate).not.toHaveBeenCalled();
  });

  it("renames a scene via the row actions menu's inline rename control", async () => {
    const user = userEvent.setup();
    const onRename = vi.fn();
    render(
      <SceneList
        scenes={scenes}
        selectedName="Alpha"
        onSelect={noop}
        onCreate={noop}
        onRename={onRename}
        onDelete={noop}
        onReorder={acceptReorder}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Alpha actions" }));
    await user.click(await screen.findByRole("menuitem", { name: "Rename" }));

    fireEvent.change(screen.getByLabelText("Scene name"), { target: { value: "Alpha Renamed" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    expect(onRename).toHaveBeenCalledWith("Alpha", "Alpha Renamed");
  });

  it("deletes a scene via the row actions menu after confirming", async () => {
    const user = userEvent.setup();
    const onDelete = vi.fn();
    render(
      <SceneList
        scenes={scenes}
        selectedName="Alpha"
        onSelect={noop}
        onCreate={noop}
        onRename={noop}
        onDelete={onDelete}
        onReorder={acceptReorder}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Alpha actions" }));
    await user.click(await screen.findByRole("menuitem", { name: "Delete" }));

    // The confirmation is the design system's ConfirmDialog now, not the
    // native window.confirm the app used to block the JS thread on.
    await user.click(await screen.findByRole("button", { name: "Delete Scene" }));

    expect(onDelete).toHaveBeenCalledWith("Alpha");
  });

  it("does not delete a scene when the confirmation is dismissed", async () => {
    const user = userEvent.setup();
    const onDelete = vi.fn();
    render(
      <SceneList
        scenes={scenes}
        selectedName="Alpha"
        onSelect={noop}
        onCreate={noop}
        onRename={noop}
        onDelete={onDelete}
        onReorder={acceptReorder}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Alpha actions" }));
    await user.click(await screen.findByRole("menuitem", { name: "Delete" }));

    expect(onDelete).not.toHaveBeenCalled();
    vi.restoreAllMocks();
  });

  describe("drag-to-reorder", () => {
    it("exposes a keyboard-focusable, labeled drag handle per scene row (accessible, not pointer-only)", () => {
      render(
        <SceneList scenes={scenes} selectedName="Alpha" onSelect={noop} onCreate={noop} onRename={noop} onDelete={noop} onReorder={acceptReorder} />,
      );
      expect(screen.getByRole("button", { name: "Reorder Alpha" })).toBeInTheDocument();
      expect(screen.getByRole("button", { name: "Reorder Beta" })).toBeInTheDocument();
    });

    it("reorders the rendered rows via keyboard: focus the handle, pick up with Space, move with ArrowDown, drop with Space", async () => {
      render(
        <SceneList
          scenes={threeScenes}
          selectedName="Alpha"
          onSelect={noop}
          onCreate={noop}
          onRename={noop}
          onDelete={noop}
          onReorder={acceptReorder}
        />,
      );
      expect(visibleSceneOrder()).toEqual(["Alpha", "Beta", "Gamma"]);

      const handle = screen.getByRole("button", { name: "Reorder Alpha" });
      handle.focus();
      await pressKey(handle, "Space");
      await pressKey(handle, "ArrowDown");
      await pressKey(handle, "Space");

      expect(visibleSceneOrder()).toEqual(["Beta", "Alpha", "Gamma"]);
    });

    it("calls onReorder with the new name order once a drag-drop actually moves a row", async () => {
      const onReorder = vi.fn().mockReturnValue(true);
      render(
        <SceneList
          scenes={threeScenes}
          selectedName="Alpha"
          onSelect={noop}
          onCreate={noop}
          onRename={noop}
          onDelete={noop}
          onReorder={onReorder}
        />,
      );

      const handle = screen.getByRole("button", { name: "Reorder Alpha" });
      handle.focus();
      await pressKey(handle, "Space");
      await pressKey(handle, "ArrowDown");
      await pressKey(handle, "Space");

      expect(onReorder).toHaveBeenCalledTimes(1);
      expect(onReorder).toHaveBeenCalledWith(["Beta", "Alpha", "Gamma"]);
    });

    // 2026-08-10 review pass: onReorder was called from INSIDE the
    // setOrder updater, and main.tsx wraps the app in React.StrictMode,
    // which deliberately double-invokes updaters in development -- so
    // every drag issued two ReorderScenes calls against the Go host.
    it("calls onReorder exactly once per drag even when React double-invokes state updaters", async () => {
      const onReorder = vi.fn().mockReturnValue(true);
      render(
        <StrictMode>
          <SceneList
            scenes={threeScenes}
            selectedName="Alpha"
            onSelect={noop}
            onCreate={noop}
            onRename={noop}
            onDelete={noop}
            onReorder={onReorder}
          />
        </StrictMode>,
      );

      const handle = screen.getByRole("button", { name: "Reorder Alpha" });
      handle.focus();
      await pressKey(handle, "Space");
      await pressKey(handle, "ArrowDown");
      await pressKey(handle, "Space");

      expect(onReorder).toHaveBeenCalledTimes(1);
    });

    it("rolls the optimistic order back when the server rejects the reorder", async () => {
      const onReorder = vi.fn().mockResolvedValue(false);
      render(
        <SceneList
          scenes={threeScenes}
          selectedName="Alpha"
          onSelect={noop}
          onCreate={noop}
          onRename={noop}
          onDelete={noop}
          onReorder={onReorder}
        />,
      );

      const handle = screen.getByRole("button", { name: "Reorder Alpha" });
      handle.focus();
      await pressKey(handle, "Space");
      await pressKey(handle, "ArrowDown");
      await pressKey(handle, "Space");

      expect(onReorder).toHaveBeenCalledWith(["Beta", "Alpha", "Gamma"]);
      // The reset effect can't catch this on its own: it only fires when
      // the scene NAME SET changes, which a failed reorder never does, so
      // the wrong order used to stick for the rest of the session.
      await waitFor(() => expect(visibleSceneOrder()).toEqual(["Alpha", "Beta", "Gamma"]));
    });

    it("keeps the new order when the server accepts it", async () => {
      const onReorder = vi.fn().mockResolvedValue(true);
      render(
        <SceneList
          scenes={threeScenes}
          selectedName="Alpha"
          onSelect={noop}
          onCreate={noop}
          onRename={noop}
          onDelete={noop}
          onReorder={onReorder}
        />,
      );

      const handle = screen.getByRole("button", { name: "Reorder Alpha" });
      handle.focus();
      await pressKey(handle, "Space");
      await pressKey(handle, "ArrowDown");
      await pressKey(handle, "Space");

      await waitFor(() => expect(onReorder).toHaveBeenCalled());
      expect(visibleSceneOrder()).toEqual(["Beta", "Alpha", "Gamma"]);
    });

    it("cancels a keyboard reorder with Escape, leaving the rendered order unchanged and never calling onReorder", async () => {
      const onReorder = vi.fn().mockReturnValue(true);
      render(
        <SceneList
          scenes={threeScenes}
          selectedName="Alpha"
          onSelect={noop}
          onCreate={noop}
          onRename={noop}
          onDelete={noop}
          onReorder={onReorder}
        />,
      );

      const handle = screen.getByRole("button", { name: "Reorder Alpha" });
      handle.focus();
      await pressKey(handle, "Space");
      await pressKey(handle, "ArrowDown");
      await pressKey(handle, "Escape");

      expect(visibleSceneOrder()).toEqual(["Alpha", "Beta", "Gamma"]);
      expect(onReorder).not.toHaveBeenCalled();
    });

    it("preserves the locally-reordered rows across an incidental scenes-prop refresh that doesn't change which scenes exist", async () => {
      const { rerender } = render(
        <SceneList
          scenes={threeScenes}
          selectedName="Alpha"
          onSelect={noop}
          onCreate={noop}
          onRename={noop}
          onDelete={noop}
          onReorder={acceptReorder}
        />,
      );

      const handle = screen.getByRole("button", { name: "Reorder Alpha" });
      handle.focus();
      await pressKey(handle, "Space");
      await pressKey(handle, "ArrowDown");
      await pressKey(handle, "Space");
      expect(visibleSceneOrder()).toEqual(["Beta", "Alpha", "Gamma"]);

      // A brand-new array reference, same three scene names -- exactly
      // what ScenesLooksWorkspace.tsx's refresh() produces after an
      // unrelated mutation elsewhere in the workspace (e.g. creating a
      // theme). The local drag order must survive this.
      rerender(
        <SceneList
          scenes={threeScenes.map((scene) => ({ ...scene }))}
          selectedName="Alpha"
          onSelect={noop}
          onCreate={noop}
          onRename={noop}
          onDelete={noop}
          onReorder={acceptReorder}
        />,
      );
      expect(visibleSceneOrder()).toEqual(["Beta", "Alpha", "Gamma"]);
    });

    it("resets the local order back to the server-provided order when a scene is actually added or removed", async () => {
      const { rerender } = render(
        <SceneList
          scenes={threeScenes}
          selectedName="Alpha"
          onSelect={noop}
          onCreate={noop}
          onRename={noop}
          onDelete={noop}
          onReorder={acceptReorder}
        />,
      );

      const handle = screen.getByRole("button", { name: "Reorder Alpha" });
      handle.focus();
      await pressKey(handle, "Space");
      await pressKey(handle, "ArrowDown");
      await pressKey(handle, "Space");
      expect(visibleSceneOrder()).toEqual(["Beta", "Alpha", "Gamma"]);

      // Real structural change: Gamma is gone, a fourth scene showed up.
      const nextScenes: ProgSceneView[] = [
        threeScenes[0],
        threeScenes[1],
        { name: "Delta", active: false, barsPerLoop: 4, layers: [] },
      ];
      rerender(
        <SceneList
          scenes={nextScenes}
          selectedName="Alpha"
          onSelect={noop}
          onCreate={noop}
          onRename={noop}
          onDelete={noop}
          onReorder={acceptReorder}
        />,
      );
      // Gamma's row plays its own exit animation (AnimatePresence) before
      // actually leaving the DOM, so the removal isn't synchronous with the
      // rerender -- Delta (newly added) shows up immediately, Gamma lingers
      // briefly mid-animation.
      await waitFor(() => expect(visibleSceneOrder()).toEqual(["Alpha", "Beta", "Delta"]));
    });
  });
});

describe("reorderSceneNames (pure arrayMove-based reorder logic)", () => {
  // dnd-kit pointer-drag interaction is notoriously fiddly to simulate
  // faithfully in jsdom; the underlying reorder math is extracted from the
  // component specifically so it can be verified directly and
  // unambiguously here, independent of any DnD simulation at all.
  const order = ["Alpha", "Beta", "Gamma"];

  it("moves the active name to the position of the over name", () => {
    expect(reorderSceneNames(order, "Alpha", "Gamma")).toEqual(["Beta", "Gamma", "Alpha"]);
    expect(reorderSceneNames(order, "Gamma", "Alpha")).toEqual(["Gamma", "Alpha", "Beta"]);
  });

  it("is a no-op when active and over are the same name", () => {
    expect(reorderSceneNames(order, "Beta", "Beta")).toEqual(order);
  });

  it("is a no-op when either name is not present in the order", () => {
    expect(reorderSceneNames(order, "Missing", "Beta")).toEqual(order);
    expect(reorderSceneNames(order, "Alpha", "Missing")).toEqual(order);
  });
});
