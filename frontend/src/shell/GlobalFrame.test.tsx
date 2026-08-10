import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "../test/renderWithQuery";

import GlobalFrame from "./GlobalFrame";
import { PlaybackSnapshotProvider } from "./PlaybackSnapshotContext";

describe("GlobalFrame", () => {
  afterEach(() => cleanup());

  it("composes the live status bar and tempo controls together", () => {
    render(
      <PlaybackSnapshotProvider>
        <GlobalFrame activeDestination="show-overview" />
      </PlaybackSnapshotProvider>,
    );

    expect(screen.getByLabelText("Live status bar")).toBeInTheDocument();
    expect(screen.getByLabelText("Tempo controls")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "0 BPM" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Turn off navigation hover text" })).toBeInTheDocument();
  });
});
