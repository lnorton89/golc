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
import { offlineStatusSnapshot } from "../../lib/wailsBridge";

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
    // The status snapshot is a module-singleton store; a test that drives
    // outputState must not leak it into the next one.
    useGolcStore.getState().setStatus(offlineStatusSnapshot());
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

  // --- 2026-08-10 review pass regressions ------------------------------

  /** progressFill reads the determinate hold wash's own transform. */
  function progressFill(button: HTMLElement): string {
    const fill = button.querySelector('[aria-hidden="true"]') as HTMLElement | null;
    return fill?.style.transform ?? "";
  }

  it("drains the hold-progress fill after a completed hold instead of leaving it at 100%", () => {
    stubSafetyService();
    render(<SafetyCluster />);
    const button = screen.getByRole("button", { name: "Blackout" });

    fireEvent.pointerDown(button);
    act(() => {
      vi.advanceTimersByTime(HOLD_DURATION_MS);
    });
    expect(progressFill(button)).toBe("scaleX(1)");

    // Releasing after a completed hold must not re-fire onComplete (the
    // `completed` latch), but it must still clear the wash -- a safety
    // control that already fired used to keep reading as mid-press until
    // the next hold began.
    fireEvent.pointerUp(button);
    expect(progressFill(button)).toBe("scaleX(0)");
  });

  it("sends the toggle argument matching each control's own label when only one of the two is engaged", async () => {
    const { Blackout, StopReleaseAll } = stubSafetyService();
    render(<SafetyCluster />);

    // Engage Blackout only.
    fireEvent.pointerDown(screen.getByRole("button", { name: "Blackout" }));
    act(() => {
      vi.advanceTimersByTime(HOLD_DURATION_MS);
    });
    expect(Blackout).toHaveBeenCalledWith(true);

    // The daemon reports the combined descriptor, which cannot say which
    // control caused it.
    act(() => {
      useGolcStore.setState((state) => ({ status: { ...state.status, outputState: "blackout" } }));
    });

    expect(screen.getByRole("button", { name: "Release Blackout" })).toBeInTheDocument();
    // Stop/Release-All was never engaged, so it must NOT claim the
    // "Release" state and must not send safetyStopReleaseAll(false).
    const stopButton = screen.getByRole("button", { name: "Stop / Release All" });

    fireEvent.pointerDown(stopButton);
    act(() => {
      vi.advanceTimersByTime(HOLD_DURATION_MS);
    });
    expect(StopReleaseAll).toHaveBeenCalledWith(true);
    expect(StopReleaseAll).not.toHaveBeenCalledWith(false);
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
