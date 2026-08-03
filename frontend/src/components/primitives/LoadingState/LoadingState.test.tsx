import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";

import LoadingState from "./LoadingState";

describe("LoadingState", () => {
  afterEach(() => cleanup());

  it.each(["inline", "panel", "list-row"] as const)("announces %s loading without inventing application truth", (variant) => {
    render(<LoadingState variant={variant} label="Loading fixtures…" />);

    const status = screen.getByRole("status", { name: "Loading fixtures…" });
    expect(status).toHaveAttribute("aria-busy", "true");
    expect(status).toHaveAttribute("aria-live", "polite");
    expect(screen.getByText("Loading fixtures…")).toBeInTheDocument();
  });

  it("retains caller-owned stable content while its local region is busy", () => {
    render(
      <LoadingState label="Refreshing fixture list…">
        <p>Existing fixture patch</p>
      </LoadingState>,
    );

    expect(screen.getByText("Existing fixture patch")).toBeInTheDocument();
    expect(screen.getByRole("status", { name: "Refreshing fixture list…" })).toBeInTheDocument();
  });
});
