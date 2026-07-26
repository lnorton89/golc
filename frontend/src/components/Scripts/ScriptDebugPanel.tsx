// ScriptDebugPanel.tsx is the live debug/log panel (08-10-PLAN.md Task 3,
// D-03/D-04/D-05/D-08/D-12/D-13): a presentational, append-only rendering
// of one script run's live log lines and per-call SDK outcomes, its
// current status chip, and (once a run ends) the "Stopped: {reason}"
// banner that survives until the caller explicitly dismisses or relaunches
// it. ScriptsWorkspace.tsx (08-04-PLAN.md, extended by this plan) owns
// every subscription/accumulation decision -- this component only renders
// the materials (events/status/terminalReason/stackFrames) it is handed,
// and never resets or clears anything on its own; onDismiss/onRunAgain are
// the caller's own state transitions.
//
// There is no automatic re-run anywhere in this file: no retry timer, no
// reconnect-and-relaunch, no effect that launches on mount (D-13). The
// only way a new run starts is the caller's own onRunAgain handler, wired
// to re-opening the launch dialog -- never a direct relaunch from here.
import { useMemo, useState } from "react";

import type { ScriptEventView } from "../../lib/wailsBridge";
import Chip, { type ChipTone } from "../primitives/Chip/Chip";
import ScrollRegion from "../primitives/ScrollRegion/ScrollRegion";
import Button from "../primitives/Button/Button";
import styles from "./ScriptDebugPanel.module.css";

export type ScriptPanelStatus =
  | "idle"
  | "running"
  | "paused"
  | "stopping"
  | "succeeded"
  | "failed"
  | "terminated"
  | "offline";

interface ScriptDebugPanelProps {
  events: ScriptEventView[];
  status: ScriptPanelStatus;
  terminalReason?: string;
  stackFrames: string[];
  onDismiss: () => void;
  onRunAgain: () => void;
}

const EMPTY_PLACEHOLDER =
  "Run or Debug this script to see live logs, diagnostics, and command outcomes here.";
const HOST_UNREACHABLE_MESSAGE =
  "Can't reach the script host. GOLC will try to reconnect automatically.";
const STOPPING_MESSAGE = "Stopping — finishing in-flight commands…";
const RESTART_DISCLAIMER =
  "This script won't restart automatically — run it again when you're ready";

const PAUSED_LINE_PATTERN = /GOLC_SCRIPT_DEBUG_PAUSED:\s*line=(\d+)/;

function statusChip(status: ScriptPanelStatus, pausedLine: number | null): { tone: ChipTone; label: string } | null {
  switch (status) {
    case "running":
      return { tone: "live", label: "Running" };
    case "paused":
      return {
        tone: "armed",
        label: pausedLine !== null ? `Paused at breakpoint — line ${pausedLine}` : "Paused at breakpoint",
      };
    case "stopping":
      return { tone: "armed", label: "Stopping…" };
    case "succeeded":
      return { tone: "neutral", label: "Succeeded" };
    case "failed":
      return { tone: "revoked", label: "Crashed" };
    case "terminated":
      return { tone: "revoked", label: "Terminated" };
    case "offline":
      return { tone: "offline", label: "Offline" };
    default:
      return null;
  }
}

// mostRecentPausedLine scans events (arrival order) from the end for the
// most recent script.status event whose Reason carries D-01's
// GOLC_SCRIPT_DEBUG_PAUSED marker, returning the paused author-coordinate
// line number it names -- null once a later GOLC_SCRIPT_DEBUG_RESUMED
// status is seen first, or when no pause has been reported at all.
function mostRecentPausedLine(events: ScriptEventView[]): number | null {
  for (let i = events.length - 1; i >= 0; i -= 1) {
    const event = events[i];
    if (event.kind !== "script.status" || !event.reason) continue;
    const match = PAUSED_LINE_PATTERN.exec(event.reason);
    if (match) return Number(match[1]);
    if (event.reason.startsWith("GOLC_SCRIPT_DEBUG_RESUMED")) return null;
  }
  return null;
}

interface TerminationDescription {
  sentence: string;
  isCrash: boolean;
}

// describeTermination translates a terminal event's machine-readable
// Reason into the exact Copywriting Contract sentence for a deadline/rate/
// scope termination (D-08) or a crash (D-03). internal/script currently
// has no structured signal for a memory/CPU resource-limit kill (the
// Windows Job Object terminates the process directly, with no
// TerminationReason recorded) or for an explicit user Stop beyond its own
// GOLC_SCRIPT_STOPPED_BY_USER message text -- both, and any other
// unrecognized termination cause, fall back to a generic
// "Terminated: {reason}" rendering rather than guessing a limit name that
// was never reported.
function describeTermination(status: ScriptPanelStatus, reason: string): TerminationDescription {
  if (status === "failed") {
    const summary = reason.split("\n")[0]?.trim() || reason;
    return { sentence: `Script crashed: ${summary}`, isCrash: true };
  }

  const deadlineMatch = /GOLC_SCRIPT_DEADLINE_EXCEEDED: run exceeded its (\S+) deadline/.exec(reason);
  if (deadlineMatch) {
    return {
      sentence: `Terminated: deadline exceeded (${deadlineMatch[1]}). Increase the limit in this script's profile if this is expected.`,
      isCrash: false,
    };
  }

  const rateMatch = /GOLC_SCRIPT_RATE_EXCEEDED: run exceeded its (\d+) call\/sec rate limit/.exec(reason);
  if (rateMatch) {
    return {
      sentence: `Terminated: rate limit exceeded (${rateMatch[1]} calls/sec). Increase the limit in this script's profile if this is expected.`,
      isCrash: false,
    };
  }

  const scopeMatch =
    /GOLC_SCRIPT_SCOPE_DENIED: method "([^"]+)" requires scope "[^"]+", profile carries "([^"]+)"/.exec(reason);
  if (scopeMatch) {
    return {
      sentence: `Terminated: this script tried to call ${scopeMatch[1]} outside its assigned ${scopeMatch[2]} capability.`,
      isCrash: false,
    };
  }

  const summary = reason.split("\n")[0]?.trim() || reason;
  return { sentence: `Terminated: ${summary}`, isCrash: false };
}

function formatTimestamp(at?: string): string {
  if (!at) return "";
  const parsed = new Date(at);
  if (Number.isNaN(parsed.getTime())) return "";
  return parsed.toLocaleTimeString([], { hour12: false });
}

function LogRow({ event }: { event: ScriptEventView }) {
  if (event.kind === "script.gap") {
    return (
      <li className={styles.resyncRow}>
        {`Resyncing — some events may have been missed${event.gapCount ? ` (${event.gapCount} dropped)` : ""}.`}
      </li>
    );
  }
  if (event.kind === "script.outcome") {
    const text = event.ok
      ? `${event.method}(...) → OK (${event.durationMs ?? 0}ms)`
      : `${event.method}(...) → ERROR: ${event.message ?? ""}`;
    return (
      <li className={event.ok ? styles.outcomeOk : styles.outcomeError}>
        <span className={styles.timestamp}>{formatTimestamp(event.at)}</span>
        <span className={styles.rowText}>{text}</span>
      </li>
    );
  }
  if (event.kind === "script.log") {
    return (
      <li className={styles.logRow}>
        <span className={styles.timestamp}>{formatTimestamp(event.at)}</span>
        {event.source ? <span className={styles.source}>{event.source}</span> : null}
        <span className={styles.rowText}>{event.message}</span>
      </li>
    );
  }
  // script.status entries drive the status chip (statusChip/
  // mostRecentPausedLine above) rather than their own stream row, so the
  // same GOLC_SCRIPT_DEBUG_* text is never rendered twice.
  return null;
}

export default function ScriptDebugPanel({
  events,
  status,
  terminalReason,
  stackFrames,
  onDismiss,
  onRunAgain,
}: ScriptDebugPanelProps) {
  const [traceExpanded, setTraceExpanded] = useState(false);

  const pausedLine = useMemo(() => mostRecentPausedLine(events), [events]);
  const chip = statusChip(status, pausedLine);
  const isTerminal = status === "succeeded" || status === "failed" || status === "terminated";
  const termination = isTerminal && terminalReason ? describeTermination(status, terminalReason) : null;

  if (status === "idle" && events.length === 0) {
    return (
      <div className={styles.panel}>
        <p className={styles.placeholder}>{EMPTY_PLACEHOLDER}</p>
      </div>
    );
  }

  const streamRows = events.filter((event) => event.kind !== "script.status");

  return (
    <div className={styles.panel}>
      <div className={styles.statusRow}>
        {chip ? <Chip tone={chip.tone}>{chip.label}</Chip> : null}
        {status === "offline" ? <span className={styles.offlineText}>{HOST_UNREACHABLE_MESSAGE}</span> : null}
        {status === "stopping" ? <span className={styles.stoppingText}>{STOPPING_MESSAGE}</span> : null}
      </div>

      <ScrollRegion className={styles.stream}>
        <ul className={styles.streamList} aria-label="Script event log">
          {streamRows.map((event, index) => (
            <LogRow key={`${event.seq}-${index}`} event={event} />
          ))}
        </ul>
      </ScrollRegion>

      {termination ? (
        <div className={styles.terminationDetail}>
          <p className={styles.terminationSentence}>{termination.sentence}</p>
          {termination.isCrash && stackFrames.length > 0 ? (
            <div className={styles.trace}>
              <button
                type="button"
                className={styles.traceToggle}
                onClick={() => setTraceExpanded((current) => !current)}
                aria-expanded={traceExpanded}
              >
                {traceExpanded ? "Hide stack trace" : "Show stack trace"}
              </button>
              {traceExpanded ? (
                <ScrollRegion className={styles.traceBody}>
                  <pre className={styles.tracePre}>{stackFrames.join("\n")}</pre>
                </ScrollRegion>
              ) : null}
            </div>
          ) : null}
        </div>
      ) : null}

      {isTerminal ? (
        <div className={styles.banner}>
          <p className={styles.bannerHeading}>
            {`Stopped: ${termination ? termination.sentence.replace(/^Terminated: |^Script crashed: /, "") : status}`}
          </p>
          <p className={styles.bannerBody}>{RESTART_DISCLAIMER}</p>
          <div className={styles.bannerActions}>
            <Button variant="secondary" onClick={onDismiss}>
              Dismiss
            </Button>
            <Button variant="primary" onClick={onRunAgain}>
              Run Again
            </Button>
          </div>
        </div>
      ) : null}
    </div>
  );
}
