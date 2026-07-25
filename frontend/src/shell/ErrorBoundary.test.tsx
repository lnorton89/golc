import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";

import ErrorBoundary from "./ErrorBoundary";

function Bomb(): never {
  throw new Error("boom");
}

describe("ErrorBoundary", () => {
  afterEach(() => cleanup());

  it("renders children normally when nothing throws", () => {
    render(
      <ErrorBoundary>
        <p>All good</p>
      </ErrorBoundary>,
    );
    expect(screen.getByText("All good")).toBeInTheDocument();
  });

  it("renders a visible fallback instead of unmounting when a child throws", () => {
    const errorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    try {
      render(
        <ErrorBoundary>
          <Bomb />
        </ErrorBoundary>,
      );
    } finally {
      errorSpy.mockRestore();
    }

    expect(screen.getByRole("alert")).toBeInTheDocument();
    expect(screen.getByText("GOLC hit an unexpected error")).toBeInTheDocument();
    expect(screen.getByText(/boom/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Reload" })).toBeInTheDocument();
  });

  it("reloads the page when the Reload button is clicked", () => {
    const errorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    const reloadSpy = vi.fn();
    const originalLocation = window.location;
    Object.defineProperty(window, "location", {
      configurable: true,
      value: { ...originalLocation, reload: reloadSpy },
    });

    try {
      render(
        <ErrorBoundary>
          <Bomb />
        </ErrorBoundary>,
      );
      fireEvent.click(screen.getByRole("button", { name: "Reload" }));
      expect(reloadSpy).toHaveBeenCalled();
    } finally {
      Object.defineProperty(window, "location", { configurable: true, value: originalLocation });
      errorSpy.mockRestore();
    }
  });
});
