import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";

import ErrorState from "./ErrorState";

describe("ErrorState", () => {
  afterEach(() => cleanup());

  it.each(["inline", "panel"] as const)("renders an accessible %s error with non-color signal", (variant) => {
    render(
      <ErrorState
        variant={variant}
        heading="Couldn't load fixtures"
        message="The current fixture patch is unchanged. Try loading fixtures again."
      />,
    );

    expect(screen.getByRole("alert")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Couldn't load fixtures" })).toBeInTheDocument();
    expect(screen.getByText("The current fixture patch is unchanged. Try loading fixtures again.")).toBeInTheDocument();
    expect(screen.getByTestId("error-icon")).toHaveAttribute("aria-hidden", "true");
  });

  it("keeps a precisely named retry action available", () => {
    const onRetry = vi.fn();
    render(
      <ErrorState
        heading="Couldn't load fixtures"
        message="The current fixture patch is unchanged. Try loading fixtures again."
        retryLabel="Load fixtures again"
        onRetry={onRetry}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Load fixtures again" }));
    expect(onRetry).toHaveBeenCalledOnce();
  });

  it("exposes optional diagnostic detail without making it the recovery path", () => {
    render(
      <ErrorState
        heading="Couldn't load fixtures"
        message="The current fixture patch is unchanged. Try loading fixtures again."
        diagnostic="GOLC_FIXTURE_READ_FAILED"
      />,
    );

    expect(screen.getByText("Technical details")).toBeInTheDocument();
    expect(screen.getByText("GOLC_FIXTURE_READ_FAILED")).toBeInTheDocument();
  });
});
