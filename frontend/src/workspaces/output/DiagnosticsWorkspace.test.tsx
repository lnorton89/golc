import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";

import DiagnosticsWorkspace from "./DiagnosticsWorkspace";

describe("DiagnosticsWorkspace", () => {
  afterEach(() => cleanup());

  it("renders as a Coming Soon stub pointing at the CLI equivalent", () => {
    render(<DiagnosticsWorkspace />);
    expect(screen.getByText("Diagnostics")).toBeInTheDocument();
    expect(screen.getByText(/golc show diagnose/)).toBeInTheDocument();
  });
});
