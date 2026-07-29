import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render } from "@testing-library/react";

import ContextualInspector from "./ContextualInspector";

describe("ContextualInspector", () => {
  afterEach(() => cleanup());

  it("hands its DOM node up via onContainerReady on mount", () => {
    const onContainerReady = vi.fn();
    render(<ContextualInspector onContainerReady={onContainerReady} onHasContentChange={vi.fn()} />);
    expect(onContainerReady).toHaveBeenCalledTimes(1);
    expect(onContainerReady.mock.calls[0][0]).toBeInstanceOf(HTMLElement);
  });

  it("calls onContainerReady with null on unmount (portal target released)", () => {
    const onContainerReady = vi.fn();
    const { unmount } = render(<ContextualInspector onContainerReady={onContainerReady} onHasContentChange={vi.fn()} />);
    onContainerReady.mockClear();
    unmount();
    expect(onContainerReady).toHaveBeenCalledWith(null);
  });
});
