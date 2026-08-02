// AppLogPanel.test.tsx covers this panel's own <behavior> contract:
// rendering every accumulated app:log line in order, the empty-events and
// all-filtered-out empty states, a gap event's visible resync notice, the
// per-level and per-source toggle filters actually hiding/showing rows, and
// Clear invoking its own callback (never mutating `events` itself -- the
// caller, DiagnosticsWorkspace.tsx, owns that state).
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";

import AppLogPanel from "./AppLogPanel";
import type { AppLogView } from "../../lib/wailsBridge";

function line(overrides: Partial<AppLogView> = {}): AppLogView {
  return {
    seq: 1,
    level: "info",
    source: "daemon",
    message: "GOLC_WAILS_DAEMON_REACHABLE: daemon already reachable",
    at: "2026-07-25T12:00:00Z",
    ...overrides,
  };
}

function logList(): HTMLElement {
  return screen.getByRole("list", { name: "Application log" });
}

describe("AppLogPanel", () => {
  afterEach(() => {
    cleanup();
  });

  it("shows an empty state when there is no log activity yet", () => {
    render(<AppLogPanel events={[]} onClear={vi.fn()} />);
    expect(screen.getByText("No log activity yet.")).toBeInTheDocument();
  });

  it("renders every accumulated line, with its source and message", () => {
    render(
      <AppLogPanel
        events={[
          line({ seq: 1, level: "info", source: "daemon", message: "daemon reachable" }),
          line({ seq: 2, level: "warn", source: "hotkeys", message: "control=blackout error=taken" }),
          line({ seq: 3, level: "error", source: "midi", message: "no ports available" }),
        ]}
        onClear={vi.fn()}
      />,
    );

    const list = logList();
    expect(within(list).getByText("daemon reachable")).toBeInTheDocument();
    expect(within(list).getByText("control=blackout error=taken")).toBeInTheDocument();
    expect(within(list).getByText("no ports available")).toBeInTheDocument();
    expect(within(list).getByText("daemon")).toBeInTheDocument();
    expect(within(list).getByText("hotkeys")).toBeInTheDocument();
    expect(within(list).getByText("midi")).toBeInTheDocument();
  });

  it("renders a gap line as a visible resync notice carrying the dropped count", () => {
    render(<AppLogPanel events={[{ seq: 0, level: "gap", gapCount: 4 }]} onClear={vi.fn()} />);
    expect(screen.getByText(/Resyncing.*\(4 dropped\)/)).toBeInTheDocument();
  });

  it("hides lines whose level is toggled off, and restores them when toggled back on", () => {
    render(
      <AppLogPanel
        events={[
          line({ seq: 1, level: "info", message: "info line" }),
          line({ seq: 2, level: "warn", message: "warn line" }),
        ]}
        onClear={vi.fn()}
      />,
    );

    expect(within(logList()).getByText("warn line")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /^Warn/ }));
    expect(within(logList()).queryByText("warn line")).not.toBeInTheDocument();
    expect(within(logList()).getByText("info line")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /^Warn/ }));
    expect(within(logList()).getByText("warn line")).toBeInTheDocument();
  });

  it("hides lines whose source is toggled off", () => {
    render(
      <AppLogPanel
        events={[
          line({ seq: 1, source: "daemon", message: "daemon line" }),
          line({ seq: 2, source: "midi", message: "midi line" }),
        ]}
        onClear={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: /^midi/ }));
    expect(within(logList()).queryByText("midi line")).not.toBeInTheDocument();
    expect(within(logList()).getByText("daemon line")).toBeInTheDocument();
  });

  it("shows a per-level and per-source count badge reflecting the full (unfiltered) event set", () => {
    render(
      <AppLogPanel
        events={[
          line({ seq: 1, level: "warn", source: "midi" }),
          line({ seq: 2, level: "warn", source: "midi" }),
          line({ seq: 3, level: "error", source: "daemon" }),
        ]}
        onClear={vi.fn()}
      />,
    );

    expect(screen.getByRole("button", { name: "Warn 2" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Error 1" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Info 0" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "midi 2" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "daemon 1" })).toBeInTheDocument();
  });

  it("shows an explicit message when every line is hidden by the current filters", () => {
    render(<AppLogPanel events={[line({ level: "warn" })]} onClear={vi.fn()} />);
    fireEvent.click(screen.getByRole("button", { name: /^Warn/ }));
    expect(screen.getByText("Every line is hidden by the current filters.")).toBeInTheDocument();
  });

  it("calls onClear when Clear is clicked, and disables Clear when there is nothing to clear", () => {
    const onClear = vi.fn();
    const { rerender } = render(<AppLogPanel events={[line()]} onClear={onClear} />);

    fireEvent.click(screen.getByRole("button", { name: "Clear" }));
    expect(onClear).toHaveBeenCalledTimes(1);

    rerender(<AppLogPanel events={[]} onClear={onClear} />);
    expect(screen.getByRole("button", { name: "Clear" })).toBeDisabled();
  });
});
