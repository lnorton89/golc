// InspectorSlot.test.tsx guards the portal mechanism itself, since this
// exact area previously caused a real infinite-render-loop bug (an earlier
// state-based design). The regression to prevent: re-rendering the
// publishing component must never cause a second, cascading render of an
// ancestor.
import { useState } from "react";
import { afterEach, describe, expect, it } from "vitest";
import { act, cleanup, render, screen } from "@testing-library/react";

import { InspectorPortalProvider, useInspectorSlot } from "./InspectorSlot";

function Publisher({ text }: { text: string }) {
  const portal = useInspectorSlot(<span data-testid="published">{text}</span>);
  return <div data-testid="publisher-root">{portal}</div>;
}

describe("useInspectorSlot", () => {
  afterEach(() => cleanup());

  it("returns null when there is no container (no provider, or container not ready yet)", () => {
    render(<Publisher text="orphan" />);
    expect(screen.queryByTestId("published")).not.toBeInTheDocument();
  });

  it("portals its content into the provided container element", () => {
    const container = document.createElement("div");
    render(
      <InspectorPortalProvider container={container}>
        <Publisher text="hello" />
      </InspectorPortalProvider>,
    );
    expect(container.textContent).toBe("hello");
  });

  it("re-rendering the publishing component does not throw or infinitely loop", () => {
    const container = document.createElement("div");

    function Wrapper() {
      const [text, setText] = useState("v1");
      return (
        <div>
          <button type="button" onClick={() => setText((current) => `${current}-updated`)}>
            update
          </button>
          <InspectorPortalProvider container={container}>
            <Publisher text={text} />
          </InspectorPortalProvider>
        </div>
      );
    }

    render(<Wrapper />);
    expect(container.textContent).toBe("v1");

    act(() => {
      screen.getByRole("button", { name: "update" }).click();
    });

    expect(container.textContent).toBe("v1-updated");
  });
});
