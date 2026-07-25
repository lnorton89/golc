import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";

import OverviewWorkspace from "./OverviewWorkspace";

describe("OverviewWorkspace", () => {
  afterEach(() => cleanup());

  it("renders as a Coming Soon stub pointing at the CLI equivalent", () => {
    render(<OverviewWorkspace />);
    expect(screen.getByText("Overview")).toBeInTheDocument();
    expect(screen.getByText(/golc show open/)).toBeInTheDocument();
  });
});
