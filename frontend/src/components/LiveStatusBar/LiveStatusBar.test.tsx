// LiveStatusBar.test.tsx covers the field set this bar renders -- added
// alongside the fix that removed its BPM field (TempoControls in
// GlobalFrame is now the header's single, non-duplicated BPM display; see
// TempoControls.tsx's doc comment). Confirms BPM never reappears here.
import { afterEach, describe, expect, it } from "vitest";
import { act, cleanup, render, screen } from "@testing-library/react";

import LiveStatusBar from "./LiveStatusBar";
import { useGolcStore } from "../../store/store";

describe("LiveStatusBar", () => {
  afterEach(() => {
    cleanup();
    useGolcStore.getState().setConnectionStatus("connecting");
  });

  it("renders Scene, Layers, and Bar fields, but never a BPM field", () => {
    render(<LiveStatusBar />);
    expect(screen.getByText("Scene")).toBeInTheDocument();
    expect(screen.getByText("Layers")).toBeInTheDocument();
    expect(screen.getByText("Bar")).toBeInTheDocument();
    expect(screen.queryByText("BPM")).not.toBeInTheDocument();
  });

  it("still omits BPM once an active scene snapshot is applied", () => {
    act(() => {
      useGolcStore.getState().setStatus({
        reachable: true,
        active: true,
        sceneName: "Alpha",
        bpm: 128,
        barIndex: 0,
        beatFraction: 0,
        enabledLayers: ["base_look"],
        controllingSource: "live",
        outputState: "live",
      });
    });

    render(<LiveStatusBar />);
    expect(screen.getByText("Alpha")).toBeInTheDocument();
    expect(screen.queryByText("BPM")).not.toBeInTheDocument();
    expect(screen.queryByText("128")).not.toBeInTheDocument();
  });
});
