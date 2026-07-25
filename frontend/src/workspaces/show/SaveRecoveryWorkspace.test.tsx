import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";

import SaveRecoveryWorkspace from "./SaveRecoveryWorkspace";

describe("SaveRecoveryWorkspace", () => {
  afterEach(() => cleanup());

  it("renders as a Coming Soon stub pointing at the CLI equivalent", () => {
    render(<SaveRecoveryWorkspace />);
    expect(screen.getByText("Save & Recovery")).toBeInTheDocument();
    expect(screen.getByText(/golc show save/)).toBeInTheDocument();
  });
});
