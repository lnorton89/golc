import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";

import FixtureLibraryWorkspace from "./FixtureLibraryWorkspace";

describe("FixtureLibraryWorkspace", () => {
  afterEach(() => cleanup());

  it("renders as a Coming Soon stub pointing at the CLI equivalent", () => {
    render(<FixtureLibraryWorkspace />);
    expect(screen.getByText("Fixture Library")).toBeInTheDocument();
    expect(screen.getByText(/golc fixture/)).toBeInTheDocument();
  });
});
