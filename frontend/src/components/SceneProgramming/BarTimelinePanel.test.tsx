import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";

import BarTimelinePanel from "./BarTimelinePanel";
import { dispatch } from "../../lib/playbackDispatch";

vi.mock("../../lib/playbackDispatch", async () => {
  const actual = await vi.importActual<typeof import("../../lib/playbackDispatch")>("../../lib/playbackDispatch");
  return { ...actual, dispatch: { ...actual.dispatch, evaluate: vi.fn() } };
});

describe("BarTimelinePanel", () => {
  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it("shows the active scene name appended to the Evaluate label", () => {
    render(<BarTimelinePanel activeSceneName="Alpha" />);
    expect(screen.getByText("Evaluate — Alpha")).toBeInTheDocument();
  });

  it("shows just 'Evaluate' when there is no active scene", () => {
    render(<BarTimelinePanel activeSceneName={null} />);
    expect(screen.getAllByText("Evaluate")).toHaveLength(2);
  });

  it("evaluates at the entered bar position and renders the returned stdout", async () => {
    vi.mocked(dispatch.evaluate).mockResolvedValue({ exitCode: 0, stdout: "bar 2.50 preview", stderr: "" });

    render(<BarTimelinePanel activeSceneName="Alpha" />);
    fireEvent.change(screen.getByLabelText("Evaluate position (bar.beatfraction)"), { target: { value: "2.5" } });
    fireEvent.click(screen.getByRole("button", { name: "Evaluate" }));

    expect(dispatch.evaluate).toHaveBeenCalledWith(2.5);
    await waitFor(() => expect(screen.getByText("bar 2.50 preview")).toBeInTheDocument());
  });

  it("falls back to stderr when stdout is empty", async () => {
    vi.mocked(dispatch.evaluate).mockResolvedValue({ exitCode: 1, stdout: "", stderr: "evaluate failed" });

    render(<BarTimelinePanel activeSceneName="Alpha" />);
    fireEvent.click(screen.getByRole("button", { name: "Evaluate" }));

    await waitFor(() => expect(screen.getByText("evaluate failed")).toBeInTheDocument());
  });
});
