// Covers AppLogPanel's windowed render path, which the other tests in this
// directory never reach: they use a handful of rows and so take the plain
// below-threshold branch by design.
//
// This file is separate because exercising the windowed path requires
// lying to jsdom about layout, and that lie must not leak into tests that
// assert real behaviour. jsdom implements no layout at all -- every
// getBoundingClientRect is zeroed and there is no ResizeObserver -- so
// @tanstack/react-virtual measures a zero-height viewport and renders zero
// rows. The shims below give it exactly enough geometry to make a decision:
// a 600px scroll viewport and 22px rows.
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { cleanup, render, screen, within } from "@testing-library/react";

import AppLogPanel from "./AppLogPanel";
import type { AppLogView } from "../../lib/wailsBridge";

const VIEWPORT_HEIGHT = 600;
const ROW_HEIGHT = 22;

let originalResizeObserver: typeof globalThis.ResizeObserver | undefined;
const patchedDescriptors: [string, PropertyDescriptor | undefined][] = [];

function definePrototypeValue(property: string, descriptor: PropertyDescriptor) {
  patchedDescriptors.push([property, Object.getOwnPropertyDescriptor(HTMLElement.prototype, property)]);
  Object.defineProperty(HTMLElement.prototype, property, descriptor);
}

beforeEach(() => {
  originalResizeObserver = globalThis.ResizeObserver;
  // Minimal stub: react-virtual only needs the callback to fire once with
  // the observed element so it re-reads the element's own dimensions.
  globalThis.ResizeObserver = class {
    constructor(private readonly callback: ResizeObserverCallback) {}
    observe(target: Element) {
      this.callback(
        [{ target, contentRect: { width: 800, height: VIEWPORT_HEIGHT } } as unknown as ResizeObserverEntry],
        this as unknown as ResizeObserver,
      );
    }
    unobserve() {}
    disconnect() {}
  } as unknown as typeof globalThis.ResizeObserver;

  // A list item is one 22px line; anything else is the scroll viewport.
  definePrototypeValue("offsetHeight", {
    configurable: true,
    get(this: HTMLElement) {
      return this.tagName === "LI" ? ROW_HEIGHT : VIEWPORT_HEIGHT;
    },
  });
  definePrototypeValue("offsetWidth", { configurable: true, get: () => 800 });
  definePrototypeValue("getBoundingClientRect", {
    configurable: true,
    value(this: HTMLElement) {
      const height = this.tagName === "LI" ? ROW_HEIGHT : VIEWPORT_HEIGHT;
      return { x: 0, y: 0, top: 0, left: 0, right: 800, bottom: height, width: 800, height, toJSON: () => ({}) };
    },
  });
});

afterEach(() => {
  cleanup();
  if (originalResizeObserver) globalThis.ResizeObserver = originalResizeObserver;
  else delete (globalThis as { ResizeObserver?: unknown }).ResizeObserver;
  for (const [property, descriptor] of patchedDescriptors.reverse()) {
    if (descriptor) Object.defineProperty(HTMLElement.prototype, property, descriptor);
    else delete (HTMLElement.prototype as unknown as Record<string, unknown>)[property];
  }
  patchedDescriptors.length = 0;
});

function lines(count: number): AppLogView[] {
  return Array.from({ length: count }, (_, index) => ({
    seq: index + 1,
    level: "info",
    source: "daemon",
    message: `line ${index + 1}`,
    at: "2026-08-09T12:00:00Z",
  }));
}

function renderedRowCount(): number {
  return within(screen.getByRole("list", { name: "Application log" })).getAllByRole("listitem").length;
}

describe("AppLogPanel windowed rendering", () => {
  it("renders every row when the stream is small enough not to need windowing", () => {
    render(<AppLogPanel events={lines(10)} onClear={() => {}} />);
    expect(renderedRowCount()).toBe(10);
  });

  it("mounts only a window of rows for a full 500-line buffer", () => {
    render(<AppLogPanel events={lines(500)} onClear={() => {}} />);

    const rendered = renderedRowCount();
    // The point of the exercise: far fewer <li> than lines in the store.
    expect(rendered).toBeLessThan(500);
    // ...but enough to fill a 600px viewport of 22px rows, plus overscan,
    // so the panel is never blank or short.
    expect(rendered).toBeGreaterThanOrEqual(VIEWPORT_HEIGHT / ROW_HEIGHT);
  });

  it("still reports the full stream height, so the scrollbar reflects every line", () => {
    render(<AppLogPanel events={lines(500)} onClear={() => {}} />);

    const list = screen.getByRole("list", { name: "Application log" });
    // 500 rows x 22px. Exact value depends on measurement, so assert it is
    // in the right order of magnitude rather than to the pixel -- a
    // viewport-sized height here would mean the scrollbar only covered the
    // rendered window.
    expect(Number.parseInt(list.style.height, 10)).toBeGreaterThan(500 * ROW_HEIGHT * 0.5);
  });

  it("keeps the earliest rows out of the DOM while showing the first window", () => {
    render(<AppLogPanel events={lines(500)} onClear={() => {}} />);

    expect(screen.getByText("line 1")).toBeInTheDocument();
    // A line far past the viewport must not be mounted at all.
    expect(screen.queryByText("line 499")).not.toBeInTheDocument();
  });
});
