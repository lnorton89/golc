// useResizablePanel.test.ts covers the drag lifecycle and persistence
// contract directly (the hook previously had no unit suite of its own --
// only the Playwright e2e resize spec exercised it). Written for the
// 2026-08-10 review pass findings: per-frame localStorage writes, a drag
// that never ends when the pointer is released outside the window, and a
// resize started by a non-primary mouse button.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, renderHook } from "@testing-library/react";

import { useResizablePanel } from "./useResizablePanel";

const OPTIONS = {
  min: 100,
  max: 400,
  defaultSize: 200,
  storageKey: "golc.test.panelWidth",
  edge: "end" as const,
};

/** pointerDownEvent fakes the React synthetic pointer event the handle
 * passes in, including the currentTarget the hook captures the pointer on. */
function pointerDownEvent(overrides: Partial<{ button: number; clientX: number }> = {}) {
  const element = {
    setPointerCapture: vi.fn(),
    releasePointerCapture: vi.fn(),
  };
  return {
    event: {
      button: overrides.button ?? 0,
      clientX: overrides.clientX ?? 0,
      clientY: 0,
      pointerId: 1,
      currentTarget: element,
      preventDefault: vi.fn(),
    } as unknown as React.PointerEvent,
    element,
  };
}

function movePointer(clientX: number) {
  act(() => {
    window.dispatchEvent(new MouseEvent("pointermove", { clientX }) as unknown as PointerEvent);
  });
}

describe("useResizablePanel", () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  afterEach(() => {
    cleanup();
    window.localStorage.clear();
    vi.restoreAllMocks();
  });

  it("tracks the pointer and clamps to [min, max]", () => {
    const { result } = renderHook(() => useResizablePanel(OPTIONS));
    const { event } = pointerDownEvent({ clientX: 0 });

    act(() => result.current.handlePointerDown(event));
    movePointer(50);
    expect(result.current.size).toBe(250);

    movePointer(9999);
    expect(result.current.size).toBe(400);
    movePointer(-9999);
    expect(result.current.size).toBe(100);
  });

  it("writes localStorage once at the end of a drag, not on every pointermove frame", () => {
    const setItem = vi.spyOn(Storage.prototype, "setItem");
    const { result } = renderHook(() => useResizablePanel(OPTIONS));
    const { event } = pointerDownEvent({ clientX: 0 });

    const writesAfterMount = setItem.mock.calls.filter(([key]) => key === OPTIONS.storageKey).length;

    act(() => result.current.handlePointerDown(event));
    for (let x = 1; x <= 60; x += 1) {
      movePointer(x);
    }
    expect(setItem.mock.calls.filter(([key]) => key === OPTIONS.storageKey).length).toBe(writesAfterMount);

    act(() => {
      window.dispatchEvent(new Event("pointerup"));
    });

    expect(setItem.mock.calls.filter(([key]) => key === OPTIONS.storageKey).length).toBe(writesAfterMount + 1);
    expect(window.localStorage.getItem(OPTIONS.storageKey)).toBe("260");
  });

  it("persists an imperative (non-drag) size change immediately", () => {
    const { result } = renderHook(() => useResizablePanel(OPTIONS));
    act(() => result.current.setSize(321));
    expect(window.localStorage.getItem(OPTIONS.storageKey)).toBe("321");
  });

  it("ignores a non-primary button press, so right/middle-click never starts a resize", () => {
    const { result } = renderHook(() => useResizablePanel(OPTIONS));
    const right = pointerDownEvent({ button: 2, clientX: 0 });

    act(() => result.current.handlePointerDown(right.event));

    expect(result.current.isResizing).toBe(false);
    // preventDefault must NOT have run either -- suppressing it was what
    // hid the context menu that would otherwise explain the behaviour.
    expect(right.event.preventDefault).not.toHaveBeenCalled();
    movePointer(50);
    expect(result.current.size).toBe(OPTIONS.defaultSize);
  });

  it("captures the pointer so a release outside the window still ends the drag", () => {
    const { result } = renderHook(() => useResizablePanel(OPTIONS));
    const { event, element } = pointerDownEvent({ clientX: 0 });

    act(() => result.current.handlePointerDown(event));
    expect(element.setPointerCapture).toHaveBeenCalledWith(1);

    act(() => {
      window.dispatchEvent(new Event("pointerup"));
    });
    expect(result.current.isResizing).toBe(false);
    expect(element.releasePointerCapture).toHaveBeenCalledWith(1);
  });

  it("ends a drag on lostpointercapture and on window blur", () => {
    const { result } = renderHook(() => useResizablePanel(OPTIONS));

    act(() => result.current.handlePointerDown(pointerDownEvent().event));
    expect(result.current.isResizing).toBe(true);
    act(() => {
      window.dispatchEvent(new Event("lostpointercapture"));
    });
    expect(result.current.isResizing).toBe(false);

    act(() => result.current.handlePointerDown(pointerDownEvent().event));
    expect(result.current.isResizing).toBe(true);
    act(() => {
      window.dispatchEvent(new Event("blur"));
    });
    expect(result.current.isResizing).toBe(false);
  });

  it("does not resume resizing after a drag ended, even as the pointer keeps moving", () => {
    const { result } = renderHook(() => useResizablePanel(OPTIONS));

    act(() => result.current.handlePointerDown(pointerDownEvent({ clientX: 0 }).event));
    movePointer(30);
    expect(result.current.size).toBe(230);

    act(() => {
      window.dispatchEvent(new Event("pointerup"));
    });
    movePointer(300);
    expect(result.current.size).toBe(230);
  });
});
