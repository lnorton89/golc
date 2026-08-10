// ScriptDebugPanel.test.tsx covers 08-10-PLAN.md Task 3's every
// <behavior> bullet for the live debug/log panel: the pre-first-run
// placeholder, live log/outcome append-in-order rendering, every status
// chip mapping, the exact D-08 deadline/rate/scope termination sentences,
// the D-03 crash-plus-expandable-trace rendering, the D-12/D-13 persistent
// "Stopped: {reason}" banner with Dismiss/Run Again, the D-11 stopping
// transient copy, and a gap event's visible resync notice.
//
// Extended by 08-12-PLAN.md Task 2 (D-01): pausedLine is now an explicit
// prop (ScriptsWorkspace.tsx derives it from the same live events, per
// this component's own header comment) rather than something this
// component scans `events` for itself, plus the four step controls'
// visibility/click wiring and each clickable stack-trace frame's
// onSelectFrame call.
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";

import ScriptDebugPanel, { type ScriptPanelStatus } from "./ScriptDebugPanel";
import type { ScriptEventView } from "../../lib/wailsBridge";

function logEvent(overrides: Partial<ScriptEventView> = {}): ScriptEventView {
  return {
    seq: 1,
    kind: "script.log",
    runId: "run-1",
    scriptName: "Chase Cycler",
    at: "2026-07-25T12:00:00Z",
    level: "info",
    message: "hello",
    source: "stdout",
    ...overrides,
  };
}

function outcomeEvent(overrides: Partial<ScriptEventView> = {}): ScriptEventView {
  return {
    seq: 2,
    kind: "script.outcome",
    runId: "run-1",
    scriptName: "Chase Cycler",
    at: "2026-07-25T12:00:01Z",
    method: "golc.scene.activate",
    durationMs: 12,
    ok: true,
    ...overrides,
  };
}

function baseProps(overrides: Partial<Parameters<typeof ScriptDebugPanel>[0]> = {}) {
  return {
    events: [] as ScriptEventView[],
    status: "idle" as ScriptPanelStatus,
    pausedLine: null as number | null,
    terminalReason: undefined,
    stackFrames: [] as string[],
    onDismiss: vi.fn(),
    onRunAgain: vi.fn(),
    onContinue: vi.fn(),
    onStepOver: vi.fn(),
    onStepInto: vi.fn(),
    onStepOut: vi.fn(),
    onSelectFrame: vi.fn(),
    ...overrides,
  };
}

describe("ScriptDebugPanel", () => {
  afterEach(() => {
    cleanup();
  });

  it("renders the pre-first-run placeholder when idle with no events", () => {
    render(<ScriptDebugPanel {...baseProps()} />);
    expect(
      screen.getByText("Run or Debug this script to see live logs, diagnostics, and command outcomes here."),
    ).toBeInTheDocument();
  });

  it("appends log events in arrival order, each with a timestamp and source location", () => {
    render(
      <ScriptDebugPanel
        {...baseProps({
          status: "running",
          events: [
            logEvent({ seq: 1, message: "first", source: "stdout" }),
            logEvent({ seq: 2, message: "second", source: "stderr" }),
          ],
        })}
      />,
    );

    const list = screen.getByRole("list", { name: "Script event log" });
    const rows = list.querySelectorAll("li");
    expect(rows).toHaveLength(2);
    expect(rows[0]).toHaveTextContent("first");
    expect(rows[0]).toHaveTextContent("stdout");
    expect(rows[1]).toHaveTextContent("second");
    expect(rows[1]).toHaveTextContent("stderr");
  });

  it("renders a successful command outcome as '{method}(...) -> OK ({duration}ms)'", () => {
    render(
      <ScriptDebugPanel
        {...baseProps({ status: "running", events: [outcomeEvent({ method: "golc.scene.activate", durationMs: 42, ok: true })] })}
      />,
    );
    expect(screen.getByText("golc.scene.activate(...) → OK (42ms)")).toBeInTheDocument();
  });

  it("renders a failed command outcome as '{method}(...) -> ERROR: {message}'", () => {
    render(
      <ScriptDebugPanel
        {...baseProps({
          status: "running",
          events: [outcomeEvent({ method: "golc.scene.activate", ok: false, message: "GOLC_SCENE_NOT_FOUND" })],
        })}
      />,
    );
    expect(screen.getByText("golc.scene.activate(...) → ERROR: GOLC_SCENE_NOT_FOUND")).toBeInTheDocument();
  });

  it("shows a 'Running' chip while a script is running", () => {
    render(<ScriptDebugPanel {...baseProps({ status: "running", events: [logEvent()] })} />);
    expect(screen.getByText("Running")).toBeInTheDocument();
  });

  it("shows a 'Paused at breakpoint — line {N}' chip from the pausedLine prop", () => {
    render(
      <ScriptDebugPanel
        {...baseProps({
          status: "paused",
          pausedLine: 12,
          events: [
            logEvent({
              seq: 3,
              kind: "script.status",
              status: "running",
              reason: "GOLC_SCRIPT_DEBUG_PAUSED: line=12, reason=other",
            }),
          ],
        })}
      />,
    );
    expect(screen.getByText("Paused at breakpoint — line 12")).toBeInTheDocument();
  });

  it("shows the host-unreachable copy and an offline chip when the bridge is unavailable", () => {
    render(<ScriptDebugPanel {...baseProps({ status: "offline" })} />);
    expect(screen.getByText("Offline")).toBeInTheDocument();
    expect(
      screen.getByText("Can't reach the script host. GOLC will try to reconnect automatically."),
    ).toBeInTheDocument();
  });

  it("shows the transient 'Stopping — finishing in-flight commands…' copy while stopping", () => {
    render(<ScriptDebugPanel {...baseProps({ status: "stopping", events: [logEvent()] })} />);
    expect(screen.getByText("Stopping — finishing in-flight commands…")).toBeInTheDocument();
  });

  it("renders the exact deadline-termination sentence (D-08)", () => {
    render(
      <ScriptDebugPanel
        {...baseProps({
          status: "terminated",
          terminalReason: "GOLC_SCRIPT_DEADLINE_EXCEEDED: run exceeded its 30s deadline (elapsed 31s)",
        })}
      />,
    );
    expect(
      screen.getByText(
        "Terminated: deadline exceeded (30s). Increase the limit in this script's profile if this is expected.",
      ),
    ).toBeInTheDocument();
  });

  it("renders the exact rate-limit-termination sentence (D-08)", () => {
    render(
      <ScriptDebugPanel
        {...baseProps({
          status: "terminated",
          terminalReason: "GOLC_SCRIPT_RATE_EXCEEDED: run exceeded its 20 call/sec rate limit",
        })}
      />,
    );
    expect(
      screen.getByText(
        "Terminated: rate limit exceeded (20 calls/sec). Increase the limit in this script's profile if this is expected.",
      ),
    ).toBeInTheDocument();
  });

  it("renders the exact memory-limit-termination sentence (D-08)", () => {
    render(
      <ScriptDebugPanel
        {...baseProps({
          status: "terminated",
          terminalReason: "GOLC_SCRIPT_MEMORY_EXCEEDED: run exceeded its 64 MB memory limit",
        })}
      />,
    );
    expect(
      screen.getByText(
        "Terminated: memory limit exceeded (64 MB). Increase the limit in this script's profile if this is expected.",
      ),
    ).toBeInTheDocument();
  });

  it("renders the memory-limit-termination sentence with a parsed (not hardcoded) megabyte figure", () => {
    render(
      <ScriptDebugPanel
        {...baseProps({
          status: "terminated",
          terminalReason: "GOLC_SCRIPT_MEMORY_EXCEEDED: run exceeded its 256 MB memory limit",
        })}
      />,
    );
    expect(
      screen.getByText(
        "Terminated: memory limit exceeded (256 MB). Increase the limit in this script's profile if this is expected.",
      ),
    ).toBeInTheDocument();
  });

  it("renders the memory-limit-termination sentence in the stopped banner form", () => {
    render(
      <ScriptDebugPanel
        {...baseProps({
          status: "terminated",
          terminalReason: "GOLC_SCRIPT_MEMORY_EXCEEDED: run exceeded its 64 MB memory limit",
        })}
      />,
    );
    expect(
      screen.getByText(
        "Stopped: memory limit exceeded (64 MB). Increase the limit in this script's profile if this is expected.",
      ),
    ).toBeInTheDocument();
  });

  // 2026-08-10 review pass: internal/script/session.go fills
  // outcome.Reason from the captured stderr tail regardless of status, so
  // a script that merely ends with a console.error and exits cleanly fell
  // through every GOLC_SCRIPT_* matcher to "Terminated: done" -- while the
  // status chip beside it read "Succeeded".
  it("reports a clean exit that carried stderr output as finished, not as terminated", () => {
    render(<ScriptDebugPanel {...baseProps({ status: "succeeded", terminalReason: "done" })} />);

    expect(screen.getByText("Finished with output: done")).toBeInTheDocument();
    expect(screen.queryByText(/^Terminated:/)).not.toBeInTheDocument();
    expect(screen.queryByText(/^Stopped:/)).not.toBeInTheDocument();
    expect(screen.getByText("Finished")).toBeInTheDocument();
  });

  it("renders the exact capability-scope-violation sentence", () => {
    render(
      <ScriptDebugPanel
        {...baseProps({
          status: "terminated",
          terminalReason:
            'GOLC_SCRIPT_SCOPE_DENIED: method "scene create" requires scope "authoring", profile carries "playback"',
        })}
      />,
    );
    expect(
      screen.getByText("Terminated: this script tried to call scene create outside its assigned playback capability."),
    ).toBeInTheDocument();
  });

  it("renders 'Script crashed: {summary}' with a collapsed-by-default expandable stack trace", () => {
    render(
      <ScriptDebugPanel
        {...baseProps({
          status: "failed",
          terminalReason: "Uncaught Error: deliberate failure",
          stackFrames: ["at run (Broken:1:7)", "at eventLoopTick (ext:core)"],
        })}
      />,
    );

    expect(screen.getByText("Script crashed: Uncaught Error: deliberate failure")).toBeInTheDocument();
    expect(screen.queryByText(/at run \(Broken:1:7\)/)).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Show stack trace" }));
    expect(screen.getByText(/at run \(Broken:1:7\)/)).toBeInTheDocument();
  });

  it("keeps the terminal state visible with a 'Stopped: {reason}' banner, Dismiss and Run Again, and the no-auto-restart copy", () => {
    const onDismiss = vi.fn();
    const onRunAgain = vi.fn();
    render(
      <ScriptDebugPanel
        {...baseProps({
          status: "terminated",
          terminalReason: "GOLC_SCRIPT_STOPPED_BY_USER: script \"Chase Cycler\" was stopped by user request",
          onDismiss,
          onRunAgain,
        })}
      />,
    );

    expect(screen.getByText(/^Stopped:/)).toBeInTheDocument();
    expect(
      screen.getByText("This script won't restart automatically — run it again when you're ready"),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Run Again" }));
    expect(onRunAgain).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByRole("button", { name: "Dismiss" }));
    expect(onDismiss).toHaveBeenCalledTimes(1);
  });

  it("keeps rendering the same terminal banner across re-renders until the caller (via onDismiss) clears its own state", () => {
    const props = baseProps({ status: "terminated" as ScriptPanelStatus, terminalReason: "GOLC_SCRIPT_STOPPED_BY_USER: stopped" });
    const { rerender } = render(<ScriptDebugPanel {...props} />);
    expect(screen.getByText(/^Stopped:/)).toBeInTheDocument();

    // Re-rendering with the identical props (simulating an unrelated parent
    // re-render) must not clear the banner -- only the caller's own
    // onDismiss-triggered prop change does that.
    rerender(<ScriptDebugPanel {...props} />);
    expect(screen.getByText(/^Stopped:/)).toBeInTheDocument();
  });

  it("renders a visible resync notice for a gap event rather than silently omitting lines", () => {
    render(
      <ScriptDebugPanel
        {...baseProps({
          status: "running",
          events: [
            logEvent({ seq: 1, message: "before" }),
            { seq: 2, kind: "script.gap", gapCount: 5 } as ScriptEventView,
            logEvent({ seq: 3, message: "after" }),
          ],
        })}
      />,
    );
    expect(screen.getByText(/Resyncing — some events may have been missed \(5 dropped\)\./)).toBeInTheDocument();
  });

  // --- 08-12-PLAN.md Task 2: step controls and clickable stack-trace
  // frames (D-01) ---

  it("renders no step control with no active debug run", () => {
    render(<ScriptDebugPanel {...baseProps({ status: "idle" })} />);
    expect(screen.queryByRole("button", { name: "Continue" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Step Over" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Step Into" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Step Out" })).not.toBeInTheDocument();
  });

  it("renders no step control during a plain Run, even while the script is running", () => {
    render(<ScriptDebugPanel {...baseProps({ status: "running", events: [logEvent()] })} />);
    expect(screen.queryByRole("button", { name: "Continue" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Step Over" })).not.toBeInTheDocument();
  });

  it("renders exactly Continue/Step Over/Step Into/Step Out while paused, and each calls its own callback exactly once", () => {
    const onContinue = vi.fn();
    const onStepOver = vi.fn();
    const onStepInto = vi.fn();
    const onStepOut = vi.fn();
    render(
      <ScriptDebugPanel
        {...baseProps({ status: "paused", pausedLine: 9, onContinue, onStepOver, onStepInto, onStepOut })}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Continue" }));
    fireEvent.click(screen.getByRole("button", { name: "Step Over" }));
    fireEvent.click(screen.getByRole("button", { name: "Step Into" }));
    fireEvent.click(screen.getByRole("button", { name: "Step Out" }));

    expect(onContinue).toHaveBeenCalledTimes(1);
    expect(onStepOver).toHaveBeenCalledTimes(1);
    expect(onStepInto).toHaveBeenCalledTimes(1);
    expect(onStepOut).toHaveBeenCalledTimes(1);
  });

  it("does not optimistically clear the paused chip or step controls on a step-control click -- only a prop change does", () => {
    const props = baseProps({ status: "paused" as ScriptPanelStatus, pausedLine: 9 });
    const { rerender } = render(<ScriptDebugPanel {...props} />);
    expect(screen.getByText("Paused at breakpoint — line 9")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Step Over" }));

    // The click alone (with no prop change) leaves everything exactly as
    // it was -- this component never clears pausedLine or the controls
    // itself (T-08-53).
    expect(screen.getByText("Paused at breakpoint — line 9")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Continue" })).toBeInTheDocument();

    // Only a genuinely new prop (the caller's own resumed/terminal event)
    // clears them.
    rerender(<ScriptDebugPanel {...props} status="running" pausedLine={null} />);
    expect(screen.queryByText(/^Paused at breakpoint/)).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Continue" })).not.toBeInTheDocument();
  });

  it("calls onSelectFrame with a clicked stack-trace frame's parsed line number", () => {
    const onSelectFrame = vi.fn();
    render(
      <ScriptDebugPanel
        {...baseProps({
          status: "failed",
          terminalReason: "Uncaught Error: deliberate failure",
          stackFrames: ["at run (Broken:1:7)", "at eventLoopTick (ext:core)"],
          onSelectFrame,
        })}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Show stack trace" }));
    fireEvent.click(screen.getByRole("button", { name: /at run \(Broken:1:7\)/ }));

    expect(onSelectFrame).toHaveBeenCalledTimes(1);
    expect(onSelectFrame).toHaveBeenCalledWith(1);
  });

  it("does not call onSelectFrame for a frame with no parseable line number", () => {
    const onSelectFrame = vi.fn();
    render(
      <ScriptDebugPanel
        {...baseProps({
          status: "failed",
          terminalReason: "Uncaught Error: deliberate failure",
          stackFrames: ["<anonymous>: GOLC_SCRIPT_SDK_SHIM_ERROR"],
          onSelectFrame,
        })}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Show stack trace" }));
    fireEvent.click(screen.getByRole("button", { name: /GOLC_SCRIPT_SDK_SHIM_ERROR/ }));

    expect(onSelectFrame).not.toHaveBeenCalled();
  });
});
