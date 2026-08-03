// SafetyCluster.test.tsx is the focused regression suite for Plan 13-15's
// design-system migration of the hold-to-confirm safety controls (D-13/
// D-14). Mocks window.go.wails.SafetyService directly, mirroring every
// other Wails-bridge test in this codebase (see Desk.test.tsx and
// wailsBridge.ts's own doc comment). Uses fake timers to drive the shared
// useHoldToConfirm state machine deterministically through every required
// scenario: threshold-minus-one, exact threshold, continued hold beyond
// threshold, duplicate completion events, every cancellation path (early
// release, pointercancel, focus loss, window blur, Escape), and a
// successful retry after a cancelled hold.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";

import SafetyCluster from "./SafetyCluster";
import { useGolcStore } from "../../store/store";

const HOLD_DURATION_MS = 750;

function ok(stdout = "") {
  return { exitCode: 0, stdout, stderr: "" };
}

function stubSafetyService() {
  const Blackout = vi.fn().mockResolvedValue(ok());
  const RevokeAutomation = vi.fn().mockResolvedValue(ok());
  const StopReleaseAll = vi.fn().mockResolvedValue(ok());
  vi.stubGlobal("go", { wails: { SafetyService: { Blackout, RevokeAutomation, StopReleaseAll } } });
  return { Blackout, RevokeAutomation, StopReleaseAll };
}

describe("SafetyCluster", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
    vi.useRealTimers();
    useGolcStore.getState().setConnectionStatus("connecting");
  });

  it("renders all three persistent, independently labeled safety controls", () => {
    stubSafetyService();
    render(<SafetyCluster />);
    const cluster = screen.getByLabelText("Safety cluster");
    const controls = cluster.querySelectorAll("button");
    expect(controls).toHaveLength(3);
    expect(screen.getByRole("button", { name: "Blackout" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "Automation" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "Stop / Release All" })).toBeEnabled();
  });

  it("does not dispatch at threshold-minus-one but dispatches exactly once at the exact threshold", () => {
    const { Blackout } = stubSafetyService();
    render(<SafetyCluster />);
    const button = screen.getByRole("button", { name: "Blackout" });

    fireEvent.pointerDown(button);
    act(() => {
      vi.advanceTimersByTime(HOLD_DURATION_MS - 1);
    });
    expect(Blackout).not.toHaveBeenCalled();

    act(() => {
      vi.advanceTimersByTime(1);
    });
    expect(Blackout).toHaveBeenCalledTimes(1);
    expect(Blackout).toHaveBeenCalledWith(true);
  });

  it("never dispatches again on a continued hold past threshold", () => {
    const { Blackout } = stubSafetyService();
    render(<SafetyCluster />);
    const button = screen.getByRole("button", { name: "Blackout" });

    fireEvent.pointerDown(button);
    act(() => {
      vi.advanceTimersByTime(HOLD_DURATION_MS);
    });
    expect(Blackout).toHaveBeenCalledTimes(1);

    act(() => {
      vi.advanceTimersByTime(2000);
    });
    expect(Blackout).toHaveBeenCalledTimes(1);
  });

  it("ignores duplicate terminal events (pointerup then pointercancel) arriving after completion", () => {
    const { Blackout } = stubSafetyService();
    render(<SafetyCluster />);
    const button = screen.getByRole("button", { name: "Blackout" });

    fireEvent.pointerDown(button);
    act(() => {
      vi.advanceTimersByTime(HOLD_DURATION_MS);
    });
    expect(Blackout).toHaveBeenCalledTimes(1);

    fireEvent.pointerUp(button);
    fireEvent.pointerCancel(button);
    expect(Blackout).toHaveBeenCalledTimes(1);
  });

  it("cancels without dispatching on early pointerup release, and a later fresh hold completes normally", () => {
    const { Blackout } = stubSafetyService();
    render(<SafetyCluster />);
    const button = screen.getByRole("button", { name: "Blackout" });

    fireEvent.pointerDown(button);
    act(() => {
      vi.advanceTimersByTime(HOLD_DURATION_MS - 1);
    });
    fireEvent.pointerUp(button);
    act(() => {
      vi.advanceTimersByTime(HOLD_DURATION_MS);
    });
    expect(Blackout).not.toHaveBeenCalled();

    fireEvent.pointerDown(button);
    act(() => {
      vi.advanceTimersByTime(HOLD_DURATION_MS);
    });
    expect(Blackout).toHaveBeenCalledTimes(1);
  });

  it("cancels without dispatching on pointerleave before threshold", () => {
    const { Blackout } = stubSafetyService();
    render(<SafetyCluster />);
    const button = screen.getByRole("button", { name: "Blackout" });

    fireEvent.pointerDown(button);
    act(() => {
      vi.advanceTimersByTime(HOLD_DURATION_MS - 1);
    });
    fireEvent.pointerLeave(button);
    act(() => {
      vi.advanceTimersByTime(HOLD_DURATION_MS);
    });
    expect(Blackout).not.toHaveBeenCalled();
  });

  it("cancels without dispatching on pointercancel before threshold", () => {
    const { Blackout } = stubSafetyService();
    render(<SafetyCluster />);
    const button = screen.getByRole("button", { name: "Blackout" });

    fireEvent.pointerDown(button);
    act(() => {
      vi.advanceTimersByTime(HOLD_DURATION_MS - 1);
    });
    fireEvent.pointerCancel(button);
    act(() => {
      vi.advanceTimersByTime(HOLD_DURATION_MS);
    });
    expect(Blackout).not.toHaveBeenCalled();
  });

  it("cancels without dispatching on element focus loss (blur) before threshold", () => {
    const { Blackout } = stubSafetyService();
    render(<SafetyCluster />);
    const button = screen.getByRole("button", { name: "Blackout" });

    fireEvent.pointerDown(button);
    act(() => {
      vi.advanceTimersByTime(HOLD_DURATION_MS - 1);
    });
    fireEvent.blur(button);
    act(() => {
      vi.advanceTimersByTime(HOLD_DURATION_MS);
    });
    expect(Blackout).not.toHaveBeenCalled();
  });

  it("cancels without dispatching on window blur before threshold", () => {
    const { Blackout } = stubSafetyService();
    render(<SafetyCluster />);
    const button = screen.getByRole("button", { name: "Blackout" });

    fireEvent.pointerDown(button);
    act(() => {
      vi.advanceTimersByTime(HOLD_DURATION_MS - 1);
      window.dispatchEvent(new Event("blur"));
    });
    act(() => {
      vi.advanceTimersByTime(HOLD_DURATION_MS);
    });
    expect(Blackout).not.toHaveBeenCalled();
  });

  it("cancels without dispatching on Escape while holding", () => {
    const { Blackout } = stubSafetyService();
    render(<SafetyCluster />);
    const button = screen.getByRole("button", { name: "Blackout" });

    fireEvent.pointerDown(button);
    act(() => {
      vi.advanceTimersByTime(HOLD_DURATION_MS - 1);
      fireEvent.keyDown(document, { key: "Escape" });
    });
    act(() => {
      vi.advanceTimersByTime(HOLD_DURATION_MS);
    });
    expect(Blackout).not.toHaveBeenCalled();
  });

  it("supports keyboard activation via Space, ignoring repeat keydown events, and cancels on early keyup", () => {
    const { RevokeAutomation } = stubSafetyService();
    render(<SafetyCluster />);
    const button = screen.getByRole("button", { name: "Automation" });

    fireEvent.keyDown(button, { key: " " });
    fireEvent.keyDown(button, { key: " ", repeat: true });
    fireEvent.keyDown(button, { key: " ", repeat: true });
    act(() => {
      vi.advanceTimersByTime(HOLD_DURATION_MS - 1);
    });
    fireEvent.keyUp(button, { key: " " });
    act(() => {
      vi.advanceTimersByTime(HOLD_DURATION_MS);
    });
    expect(RevokeAutomation).not.toHaveBeenCalled();

    fireEvent.keyDown(button, { key: "Enter" });
    act(() => {
      vi.advanceTimersByTime(HOLD_DURATION_MS);
    });
    expect(RevokeAutomation).toHaveBeenCalledTimes(1);
    expect(RevokeAutomation).toHaveBeenCalledWith(true);
  });

  it("keeps each control's busy/error state independent -- a failing action never blocks its siblings", async () => {
    const Blackout = vi.fn().mockResolvedValue({ exitCode: 1, stdout: "", stderr: "daemon unreachable" });
    const RevokeAutomation = vi.fn().mockResolvedValue(ok());
    const StopReleaseAll = vi.fn().mockResolvedValue(ok());
    vi.stubGlobal("go", { wails: { SafetyService: { Blackout, RevokeAutomation, StopReleaseAll } } });

    render(<SafetyCluster />);
    const blackoutButton = screen.getByRole("button", { name: "Blackout" });
    const revokeButton = screen.getByRole("button", { name: "Automation" });

    fireEvent.pointerDown(blackoutButton);
    await act(async () => {
      vi.advanceTimersByTime(HOLD_DURATION_MS);
      await Promise.resolve();
    });
    expect(Blackout).toHaveBeenCalledTimes(1);
    expect(blackoutButton).toHaveAttribute("data-error", "true");
    expect(blackoutButton).toBeEnabled();

    // The sibling Automation control must remain fully independent and
    // reachable -- neither disabled nor otherwise affected by Blackout's
    // own failure.
    expect(revokeButton).toBeEnabled();
    fireEvent.pointerDown(revokeButton);
    act(() => {
      vi.advanceTimersByTime(HOLD_DURATION_MS);
    });
    expect(RevokeAutomation).toHaveBeenCalledTimes(1);
  });
});
