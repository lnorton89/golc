// ScriptDebugPanel.tsx is the live debug/log panel (08-10-PLAN.md Task 3,
// D-03/D-04/D-05/D-08/D-12/D-13; extended by 08-12-PLAN.md Task 2, D-01):
// a presentational, append-only rendering of one script run's live log
// lines and per-call SDK outcomes, its current status chip, the four
// paused-run step controls, and (once a run ends) the "Stopped: {reason}"
// banner that survives until the caller explicitly dismisses or relaunches
// it. ScriptsWorkspace.tsx (08-04-PLAN.md, extended by this plan) owns
// every subscription/accumulation decision -- this component only renders
// the materials it is handed (events/status/pausedLine/terminalReason/
// stackFrames), and never resets or clears anything on its own;
// onDismiss/onRunAgain/onContinue/onStepOver/onStepInto/onStepOut/
// onSelectFrame are all the caller's own state transitions -- clicking a
// step control never optimistically clears the paused state here (T-08-53:
// paused state is derived only from the backend's own next event, which
// ScriptsWorkspace feeds back in as a fresh pausedLine prop).
//
// There is no automatic re-run anywhere in this file: no retry timer, no
// reconnect-and-relaunch, no effect that launches on mount (D-13). The
// only way a new run starts is the caller's own onRunAgain handler, wired
// to re-opening the launch dialog -- never a direct relaunch from here.
import { useState } from "react";
import { Play, ChevronsRight, ArrowDownToLine, ArrowUpToLine, X, RotateCcw, ChevronDown, ChevronUp } from "lucide-react";

import type { ScriptEventView } from "../../lib/wailsBridge";
import { Button, Chip, ScrollRegion } from "../../design-system";
import type { ChipTone } from "../primitives/Chip/Chip";
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
  /** pausedLine (08-12-PLAN.md Task 2, D-01): the paused run's current
   * author-coordinate line, derived by ScriptsWorkspace.tsx from the same
   * live script.status events this panel's own log stream renders --
   * ScriptsWorkspace is the single derivation point so this value and
   * ScriptEditor's own `currentExecutionLine` prop can never independently
   * drift apart (see this file's own header comment / key_links). null
   * whenever status is not "paused". */
  pausedLine: number | null;
  terminalReason?: string;
  stackFrames: string[];
  onDismiss: () => void;
  onRunAgain: () => void;
  /** onContinue/onStepOver/onStepInto/onStepOut (08-12-PLAN.md Task 2,
   * D-01): the four step controls, rendered only while status === "paused"
   * (never during a plain Run, never with no active debug run). Each
   * calls its corresponding backend route via ScriptsWorkspace.tsx; this
   * component never clears pausedLine itself on click. */
  onContinue: () => void;
  onStepOver: () => void;
  onStepInto: () => void;
  onStepOut: () => void;
  /** onSelectFrame (08-12-PLAN.md Task 2, D-03): called with a clicked
   * stack-trace frame's author-coordinate line -- ScriptsWorkspace.tsx
   * reveals it in ScriptEditor via the same currentExecutionLine
   * mechanism Task 1 already built for the paused-line highlight. Not
   * called for a frame whose text carries no parseable line number (e.g.
   * a shim-marker frame). */
  onSelectFrame: (line: number) => void;
}

// STACK_FRAME_LINE_PATTERN extracts the author-coordinate line number from
// one stack-trace frame's formatted text -- both debugbridge.go's
// `"    at %s (%s:%d:%d)"` exception frames and Deno's own raw captured
// "at Func (file:///...:LINE:COL)" stderr text end in the same trailing
// ":LINE:COL)" shape, so one pattern covers both sources.
const STACK_FRAME_LINE_PATTERN = /:(\d+):\d+\)\s*$/;

// stackFrameLine parses STACK_FRAME_LINE_PATTERN out of one frame's text,
// returning null (never a wrong guess) when the frame's shape doesn't
// carry a recognizable trailing line/column -- a click on such a frame is
// then a deliberate no-op rather than jumping to a fabricated line.
function stackFrameLine(frame: string): number | null {
  const match = STACK_FRAME_LINE_PATTERN.exec(frame);
  if (!match) return null;
  const line = Number(match[1]);
  return Number.isFinite(line) && line > 0 ? line : null;
}

const EMPTY_PLACEHOLDER =
  "Run or Debug this script to see live logs, diagnostics, and command outcomes here.";
const HOST_UNREACHABLE_MESSAGE =
  "Can't reach the script host. GOLC will try to reconnect automatically.";
const STOPPING_MESSAGE = "Stopping — finishing in-flight commands…";
const RESTART_DISCLAIMER =
  "This script won't restart automatically — run it again when you're ready";

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

interface TerminationDescription {
  sentence: string;
  isCrash: boolean;
  /** false when the run ended cleanly and `sentence` is merely its final
   * output -- the banner must not report that as having been stopped. */
  isTermination: boolean;
}

// describeTermination translates a terminal event's machine-readable
// Reason into the exact Copywriting Contract sentence for a deadline/
// rate/memory/scope termination (D-08) or a crash (D-03). The Go host
// (internal/script) publishes GOLC_SCRIPT_MEMORY_EXCEEDED for a memory-
// ceiling kill -- produced by both a proactive Job Object usage monitor
// (memorywatch.go) and a post-exit classifier (classifyMemoryExhaustion,
// capability.go), which emit identical text, so which one actually fired
// is never observable here. The CPU cap deliberately has no termination
// code of its own: it throttles rather than kills, so a CPU-bound run
// always ends on its deadline instead (08-RESEARCH.md Pitfall 3). An
// explicit user Stop (its own GOLC_SCRIPT_STOPPED_BY_USER message text)
// and any other unrecognized termination cause both fall back to a
// generic "Terminated: {reason}" rendering rather than guessing a limit
// name that was never reported.
function describeTermination(status: ScriptPanelStatus, reason: string): TerminationDescription {
  // A SUCCEEDED run routinely carries a non-empty Reason:
  // internal/script/session.go fills outcome.Reason from the captured
  // stderr tail regardless of status, so a script that merely ends with a
  // console.error and exits cleanly used to fall all the way through the
  // four GOLC_SCRIPT_* matchers to "Terminated: done" -- while the status
  // chip next to it simultaneously read "Succeeded". Nothing about a
  // clean exit is a termination cause, so this is framed as what it
  // actually is: the run's final output.
  if (status === "succeeded") {
    const summary = reason.split("\n")[0]?.trim() || reason;
    return { sentence: `Finished with output: ${summary}`, isCrash: false, isTermination: false };
  }

  if (status === "failed") {
    const summary = reason.split("\n")[0]?.trim() || reason;
    return { sentence: `Script crashed: ${summary}`, isCrash: true, isTermination: true };
  }

  const deadlineMatch = /GOLC_SCRIPT_DEADLINE_EXCEEDED: run exceeded its (\S+) deadline/.exec(reason);
  if (deadlineMatch) {
    return {
      sentence: `Terminated: deadline exceeded (${deadlineMatch[1]}). Increase the limit in this script's profile if this is expected.`,
      isCrash: false,
      isTermination: true,
    };
  }

  const rateMatch = /GOLC_SCRIPT_RATE_EXCEEDED: run exceeded its (\d+) call\/sec rate limit/.exec(reason);
  if (rateMatch) {
    return {
      sentence: `Terminated: rate limit exceeded (${rateMatch[1]} calls/sec). Increase the limit in this script's profile if this is expected.`,
      isCrash: false,
      isTermination: true,
    };
  }

  const memoryMatch = /GOLC_SCRIPT_MEMORY_EXCEEDED: run exceeded its (\d+) MB memory limit/.exec(reason);
  if (memoryMatch) {
    return {
      sentence: `Terminated: memory limit exceeded (${memoryMatch[1]} MB). Increase the limit in this script's profile if this is expected.`,
      isCrash: false,
      isTermination: true,
    };
  }

  const scopeMatch =
    /GOLC_SCRIPT_SCOPE_DENIED: method "([^"]+)" requires scope "[^"]+", profile carries "([^"]+)"/.exec(reason);
  if (scopeMatch) {
    return {
      sentence: `Terminated: this script tried to call ${scopeMatch[1]} outside its assigned ${scopeMatch[2]} capability.`,
      isCrash: false,
      isTermination: true,
    };
  }

  const summary = reason.split("\n")[0]?.trim() || reason;
  return { sentence: `Terminated: ${summary}`, isCrash: false, isTermination: true };
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
      <li className={event.ok ? styles.outcomeOk : styles.outcomeFailed}>
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
  // script.status entries drive the status chip (statusChip above, fed by
  // ScriptsWorkspace.tsx's own pausedLine derivation) rather than their own
  // stream row, so the same GOLC_SCRIPT_DEBUG_* text is never rendered
  // twice.
  return null;
}

export default function ScriptDebugPanel({
  events,
  status,
  pausedLine,
  terminalReason,
  stackFrames,
  onDismiss,
  onRunAgain,
  onContinue,
  onStepOver,
  onStepInto,
  onStepOut,
  onSelectFrame,
}: ScriptDebugPanelProps) {
  const [traceExpanded, setTraceExpanded] = useState(false);

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
        {/* Step controls (D-01, 08-12-PLAN.md Task 2): rendered ONLY while
            status === "paused" -- absent with no active debug run, and
            absent during a plain Run even while it's running. Each click
            calls its own prop callback exactly once; this component never
            clears pausedLine or the chip itself -- the caller's next
            script.status/script.terminal event, fed back in as a fresh
            prop, is the only thing that ever changes what's rendered here
            (T-08-53). */}
        {status === "paused" ? (
          <div className={styles.stepControls}>
            <Button variant="secondary" icon={Play} onClick={onContinue}>
              Continue
            </Button>
            <Button variant="secondary" icon={ChevronsRight} onClick={onStepOver}>
              Step Over
            </Button>
            <Button variant="secondary" icon={ArrowDownToLine} onClick={onStepInto}>
              Step Into
            </Button>
            <Button variant="secondary" icon={ArrowUpToLine} onClick={onStepOut}>
              Step Out
            </Button>
          </div>
        ) : null}
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
              <Button
                variant="secondary"
                size="compact"
                leadingIcon={traceExpanded ? ChevronUp : ChevronDown}
                onClick={() => setTraceExpanded((current) => !current)}
                aria-expanded={traceExpanded}
              >
                {traceExpanded ? "Hide stack trace" : "Show stack trace"}
              </Button>
              {traceExpanded ? (
                <ScrollRegion className={styles.traceBody}>
                  {/* Each frame is its own keyboard-focusable control
                      (D-03, 08-12-PLAN.md Task 2): clicking (or activating
                      via keyboard) calls onSelectFrame with the frame's
                      parsed author-coordinate line, a no-op for a frame
                      whose text carries no parseable line (never a wrong
                      guess). */}
                  <ul className={styles.traceList} aria-label="Stack trace">
                    {stackFrames.map((frame, index) => {
                      const line = stackFrameLine(frame);
                      return (
                        <li key={index}>
                          <button
                            type="button"
                            className={styles.traceFrame}
                            onClick={() => {
                              if (line !== null) onSelectFrame(line);
                            }}
                          >
                            {frame}
                          </button>
                        </li>
                      );
                    })}
                  </ul>
                </ScrollRegion>
              ) : null}
            </div>
          ) : null}
        </div>
      ) : null}

      {isTerminal ? (
        <div className={styles.banner}>
          <p className={styles.bannerHeading}>
            {termination && termination.isTermination
              ? `Stopped: ${termination.sentence.replace(/^Terminated: |^Script crashed: /, "")}`
              : status === "succeeded"
                ? "Finished"
                : `Stopped: ${status}`}
          </p>
          <p className={styles.bannerBody}>{RESTART_DISCLAIMER}</p>
          <div className={styles.bannerActions}>
            <Button variant="secondary" icon={X} onClick={onDismiss}>
              Dismiss
            </Button>
            <Button variant="primary" icon={RotateCcw} onClick={onRunAgain}>
              Run Again
            </Button>
          </div>
        </div>
      ) : null}
    </div>
  );
}
