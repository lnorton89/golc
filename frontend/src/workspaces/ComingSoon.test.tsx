import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";

import ComingSoon from "./ComingSoon";

describe("ComingSoon", () => {
  afterEach(() => cleanup());

  it("renders the title, description, and CLI hint it is given", () => {
    render(<ComingSoon title="Overview" description="Not wired in yet." cliHint="Use golc show open." />);
    expect(screen.getByText("Overview")).toBeInTheDocument();
    expect(screen.getByText("Not wired in yet.")).toBeInTheDocument();
    expect(screen.getByText("Use golc show open.")).toBeInTheDocument();
  });

  it("labels the workspace as a named, bounded region", () => {
    render(<ComingSoon title="Overview" description="Not wired in yet." cliHint="Use golc show open." />);
    expect(screen.getByRole("region", { name: "Overview workspace" })).toBeInTheDocument();
  });
});
