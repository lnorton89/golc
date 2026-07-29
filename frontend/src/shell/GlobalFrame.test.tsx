import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";

import GlobalFrame from "./GlobalFrame";
import { PlaybackSnapshotProvider } from "./PlaybackSnapshotContext";

describe("GlobalFrame", () => {
  afterEach(() => cleanup());

  it("composes the live status bar and tempo controls together", () => {
    render(
      <PlaybackSnapshotProvider>
        <GlobalFrame />
      </PlaybackSnapshotProvider>,
    );

    expect(screen.getByLabelText("Live status bar")).toBeInTheDocument();
    expect(screen.getByLabelText("Tempo controls")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "0 BPM" })).toBeInTheDocument();
  });
});
