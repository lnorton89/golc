// ScriptsWorkspace is Build -> Scripts (08-04-PLAN.md Task 2, SCRP-01/
// D-16, extended by 08-10-PLAN.md Task 3 for SCRP-04/SCRP-05 and by
// 08-12-PLAN.md Task 2 for D-01's full breakpoint/step debugger UI): the
// D-16 script library view, the create/edit/save/delete round trip, the
// Run/Debug/Validate/Stop Script toolbar actions, the Run/Debug launch
// dialog, and (via useInspectorSlot) the selected script's
// capability-profile summary in the contextual inspector. Owns every
// ScriptService call and all script state, following
// ScenesLooksWorkspace.tsx's exact load/refresh/error and
// selection-validity-repair pattern (08-PATTERNS.md) -- the correct
// structural template per 08-UI-SPEC.md's correction of RESEARCH.md
// (FixtureLibraryWorkspace.tsx is a bare ComingSoon stub, not a library
// pattern).
//
// The editing surface is a real Monaco instance (ScriptEditor.tsx, 08-11-
// PLAN.md Task 3, D-15) running the TypeScript language service against
// the generated GOLC SDK -- replacing 08-04's plain bounded plaintext
// input element in place. 08-12-PLAN.md Task 1 gave ScriptEditor a real
// glyph-margin breakpoint gutter and a currentExecutionLine highlight;
// this file (Task 2) is the one place that holds the gutter's own
// breakpointLines state, sends it verbatim as DebugScript's own
// breakpointLines argument on launch, derives pausedLine from the live
// script.status stream (reduceScriptEvent, this file's own single
// derivation point per the plan's key_links), and feeds it to both
// ScriptEditor and ScriptDebugPanel so the two surfaces can never
// independently drift apart.
//
// Run/Debug/Stop Script are deliberately the same 32px Button height as
// every other secondary/destructive action in the app -- NOT Phase 6's
// 64px hold-to-confirm safety-cluster treatment. D-10 requires per-script
// Stop to read as a normal, lightweight, single-script-scoped action,
// visually distinct from the global Revoke Automation control; reusing the
// existing Button primitive's own default scale is itself the mechanism
// (see 08-UI-SPEC.md's Spacing Scale "Run / Debug / Stop / Validate
// buttons" row).
//
// There is no automatic re-run anywhere in this file: no retry timer, no
// reconnect-and-relaunch, no effect that launches a script on mount or on
// selection change (D-13). The only way a new run starts is an explicit
// user click on Run/Debug, or "Run Again" re-opening the launch dialog
// (never relaunching directly) so the profile is always reviewed (D-07).
import { useCallback, useEffect, useState, type CSSProperties } from "react";
import { FileCode2, Plus, X, Check, Save, Trash2, ShieldCheck, Play, Bug, Square } from "lucide-react";
import { AnimatePresence, motion } from "motion/react";
// BaseToolbar (aliased -- this file already imports the design-system's own
// unrelated `Toolbar` primitive, a workspace title bar, not a button
// group; see this file's own toolbarActions doc comment below for why the
// two are false cognates that happen to share a name): Base UI's Toolbar
// gives the Run/Debug/Delete/Validate/Stop Script action row real
// roving-tabindex arrow-key navigation between its buttons, the same
// composable-primitive approach Menu.tsx/Dialog.tsx already use in this
// codebase (`render={<Button .../>}` merges Base UI's own computed props
// onto the given Button element rather than wrapping it in a new one).
import { Toolbar as BaseToolbar } from "@base-ui/react/toolbar";

import {
  assertOk,
  continueScript,
  createScript,
  debugScript,
  deleteScript,
  errorMessage,
  getScript,
  getSdkTypeDefinitions,
  isScriptServiceAvailable,
  listScripts,
  onScriptEvent,
  runScript,
  saveScriptSource,
  setScriptProfile,
  stepIntoScript,
  stepOutScript,
  stepOverScript,
  stopScript,
  validateScript,
  type ScriptEventView,
  type ScriptSummaryView,
  type ScriptValidationView,
  type WailsResult,
} from "../../lib/wailsBridge";

import { HOW_IT_WORKS_BY_ID } from "../../shell/navigation";
import { motionTransition } from "../../design-system/motion";
import { Button, Chip, EmptyState, Field, FormActions, ListRow, ResizeHandle, ScrollRegion, Toolbar } from "../../design-system";
import type { ChipTone } from "../../components/primitives/Chip/Chip";
import ScriptEditor from "../../components/Scripts/ScriptEditor";
import ScriptRunDialog, { type ScriptDialogProfile as ScriptProfileFields, type ScriptLaunchMode } from "../../components/Scripts/ScriptRunDialog";
import ScriptDebugPanel, { type ScriptPanelStatus } from "../../components/Scripts/ScriptDebugPanel";
import { useInspectorSlot } from "../../shell/InspectorSlot";
import { useResizablePanel } from "../../hooks/useResizablePanel";
import styles from "./ScriptsWorkspace.module.css";

const rowExitTransition = motionTransition("settle");

// ScriptPanelState is one script's accumulated live-run view (08-10-PLAN.md
// Task 3, extended by 08-12-PLAN.md Task 2 for D-01): events in arrival
// order (log/outcome/status/gap), the current sub-state derived from the
// most recent script.status event, the paused run's current
// author-coordinate line (null whenever liveStatus isn't "paused"), and
// the frozen terminal status/reason once a run ends -- D-12: terminal
// never clears itself, only handleDismissPanel does.
//
// pausedLine is this workspace's single derivation point for "where is the
// paused run currently stopped" (08-12-PLAN.md Task 2's key_links: the
// same value feeds both ScriptEditor's currentExecutionLine highlight and
// ScriptDebugPanel's paused chip/step-control gate, so the two surfaces
// can never independently drift apart).
interface ScriptPanelState {
  events: ScriptEventView[];
  runId: string | null;
  liveStatus: "idle" | "running" | "paused" | "stopping";
  pausedLine: number | null;
  terminal: { status: string; reason: string } | null;
}

const IDLE_PANEL_STATE: ScriptPanelState = {
  events: [],
  runId: null,
  liveStatus: "idle",
  pausedLine: null,
  terminal: null,
};

// PAUSED_LINE_PATTERN mirrors ScriptDebugPanel.tsx's own (removed, 08-12-
// PLAN.md Task 2) internal derivation exactly: a script.status event whose
// Reason carries D-01's GOLC_SCRIPT_DEBUG_PAUSED marker names the paused
// author-coordinate line as "line=<N>".
const PAUSED_LINE_PATTERN = /GOLC_SCRIPT_DEBUG_PAUSED:\s*line=(\d+)/;

// reduceScriptEvent folds one live script.log/script.outcome/script.status/
// script.terminal/script.gap event into a script's own accumulated panel
// state. A script.status/script.terminal event carrying a runId different
// from the one already tracked starts a fresh accumulated list (D-13: two
// runs never share state) -- this is always safe even across a dismissed
// or still-frozen terminal state, because the only way a new runId can
// ever arrive is an explicit new Run/Debug launch the user just triggered.
function reduceScriptEvent(state: ScriptPanelState, event: ScriptEventView): ScriptPanelState {
  if (event.kind === "script.gap") {
    return { ...state, events: [...state.events, event] };
  }

  const isNewRun = event.runId !== undefined && event.runId !== "" && event.runId !== state.runId;
  const events = isNewRun ? [event] : [...state.events, event];
  const runId = event.runId || state.runId;

  if (event.kind === "script.terminal") {
    // A terminal event always clears pausedLine (D-01/D-12): the paused
    // highlight/controls never survive past the run they belonged to,
    // even if the run happened to terminate while still paused.
    return {
      events,
      runId,
      liveStatus: "idle",
      pausedLine: null,
      terminal: { status: event.status ?? "", reason: event.reason ?? "" },
    };
  }

  if (event.kind === "script.status") {
    const pausedMatch = event.reason ? PAUSED_LINE_PATTERN.exec(event.reason) : null;
    const liveStatus: ScriptPanelState["liveStatus"] = pausedMatch ? "paused" : "running";
    return {
      events,
      runId,
      liveStatus,
      pausedLine: pausedMatch ? Number(pausedMatch[1]) : null,
      terminal: isNewRun ? null : state.terminal,
    };
  }

  // script.log / script.outcome
  return { ...state, events, runId };
}

// deriveStackFramesFromReason mirrors internal/wails.deriveStackFrames
// exactly (svc_script.go, 08-10-PLAN.md Task 1): a terminal event's Reason
// carries a crash's full captured text as one multi-line string -- its
// first line is the crash summary rendered separately by
// ScriptDebugPanel's own "Script crashed: {summary}" line, and every
// remaining non-blank line is one D-03 expandable trace entry. Reused here
// (TypeScript-side) because ScriptEventView (the live "script:event" push
// payload) carries no separately structured stackFrames field of its own
// -- unlike RunScript/DebugScript's own ScriptRunOutcomeView return value,
// which is only available once the ENTIRE run finishes, well after the
// terminal event this function actually derives from has already arrived
// live.
function deriveStackFramesFromReason(reason: string): string[] {
  const lines = reason.split("\n");
  if (lines.length <= 1) return [];
  return lines
    .slice(1)
    .map((line) => line.trim())
    .filter((line) => line !== "");
}

// HOST_UNREACHABLE_MESSAGE is the UI-SPEC's exact "script host unreachable"
// copy (Copywriting Contract), rendered inline whenever ScriptService is
// not bound (isScriptServiceAvailable) -- both a missing bridge (jsdom, a
// plain browser preview) and a rejected ListScripts call resolve to this same message,
// alongside the D-16 empty state (listScripts/never throws, see
// wailsBridge.ts's doc comment).
const HOST_UNREACHABLE_MESSAGE = "Can't reach the script host. GOLC will try to reconnect automatically.";

// chipToneForStatus maps a ScriptRunStatus onto the existing six-colour
// status vocabulary (08-UI-SPEC.md's Status Vocabulary table): running ->
// live, failed/terminated -> revoked, never_run -> offline, succeeded (and
// any unrecognized value) -> neutral. No new state colours are introduced.
function chipToneForStatus(status: string): ChipTone {
  switch (status) {
    case "running":
      return "live";
    case "failed":
    case "terminated":
      return "revoked";
    case "never_run":
      return "offline";
    default:
      return "neutral";
  }
}

// statusLabel renders a ScriptRunStatus as human-readable text for the
// library row's chip.
function statusLabel(status: string): string {
  switch (status) {
    case "running":
      return "Running";
    case "succeeded":
      return "Succeeded";
    case "failed":
      return "Failed";
    case "terminated":
      return "Terminated";
    case "never_run":
      return "Never run";
    default:
      return status;
  }
}

export default function ScriptsWorkspace() {
  const [scripts, setScripts] = useState<ScriptSummaryView[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selectedName, setSelectedName] = useState<string | null>(null);
  const [source, setSource] = useState("");
  const [sourceLoading, setSourceLoading] = useState(false);
  const [creating, setCreating] = useState(false);
  const [newName, setNewName] = useState("");
  const [confirmingDelete, setConfirmingDelete] = useState(false);
  const [dialogMode, setDialogMode] = useState<ScriptLaunchMode | null>(null);
  const [panelStateByScript, setPanelStateByScript] = useState<Record<string, ScriptPanelState>>({});
  const [validation, setValidation] = useState<ScriptValidationView | null>(null);
  const [validating, setValidating] = useState(false);
  const [sdkTypeDefinitions, setSdkTypeDefinitions] = useState("");
  // breakpointLines (08-12-PLAN.md Task 1/Task 2, D-01): the gutter's own
  // breakpoint set, held here (not inside ScriptPanelState) because it's
  // authored UI state tied to the currently open script, not something any
  // live script.status event ever reports back -- ScriptEditor's own
  // glyph-margin click calls handleToggleBreakpoint below.
  const [breakpointLines, setBreakpointLines] = useState<number[]>([]);
  // selectedFrameLine (08-12-PLAN.md Task 2, D-03): the line a user last
  // clicked in an expanded crash stack trace, reusing ScriptEditor's
  // existing currentExecutionLine highlight mechanism (Task 1) to reveal
  // it -- panelState.pausedLine always takes priority when a run is
  // actually paused (see currentExecutionLine below), so the two never
  // fight over the same highlight.
  const [selectedFrameLine, setSelectedFrameLine] = useState<number | null>(null);
  const libraryPanel = useResizablePanel({
    min: 180,
    max: 440,
    defaultSize: 240,
    storageKey: "golc.scriptsLibraryWidth",
    edge: "end",
  });

  // Fetches golc.d.ts once for the component's whole lifetime (D-15):
  // ScriptEditor registers it as Monaco's TypeScript extra lib, giving
  // live autocomplete/diagnostics against the real generated GOLC SDK.
  useEffect(() => {
    void (async () => {
      setSdkTypeDefinitions(await getSdkTypeDefinitions());
    })();
  }, []);

  const refresh = useCallback(async (): Promise<void> => {
    try {
      const next = await listScripts();
      setScripts(next);
      setError(isScriptServiceAvailable() ? null : HOST_UNREACHABLE_MESSAGE);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  // Live D-04/D-05 event subscription (08-08-PLAN.md Task 3's
  // "script:event" push): one subscription for the component's whole
  // lifetime, folding every event into its own script's accumulated panel
  // state (keyed by ScriptEventView.scriptName) via reduceScriptEvent, so
  // switching the selected script never drops another script's still-
  // running or still-frozen-terminal history. Unsubscribes on unmount.
  useEffect(() => {
    const unsubscribe = onScriptEvent((event) => {
      const scriptName = event.scriptName;
      if (!scriptName) return;
      setPanelStateByScript((current) => {
        const previous = current[scriptName] ?? IDLE_PANEL_STATE;
        return { ...current, [scriptName]: reduceScriptEvent(previous, event) };
      });
    });
    return unsubscribe;
  }, []);

  // A prior validation result only ever applies to the script it was run
  // against -- selecting a different script clears it rather than showing
  // a stale "N error(s)" summary for the wrong source. Breakpoints (D-01)
  // and a clicked crash frame (D-03) are equally script-specific UI state
  // -- switching scripts must never show one script's gutter breakpoints
  // or highlighted frame line on a different script's editor.
  useEffect(() => {
    setValidation(null);
    setBreakpointLines([]);
    setSelectedFrameLine(null);
  }, [selectedName]);

  // Selection-validity-repair effect (ScenesLooksWorkspace.tsx's identical
  // discipline): drop a selection that no longer exists (e.g. a CLI-side
  // delete outside this session) and default to the first script once data
  // loads.
  useEffect(() => {
    if (selectedName && scripts.some((script) => script.name === selectedName)) {
      return;
    }
    setSelectedName(scripts[0]?.name ?? null);
  }, [scripts, selectedName]);

  // Loads the selected script's full source into the editor whenever
  // selection changes.
  useEffect(() => {
    if (!selectedName) {
      setSource("");
      return;
    }
    let cancelled = false;
    setSourceLoading(true);
    void (async () => {
      try {
        const detail = await getScript(selectedName);
        if (!cancelled) {
          setSource(detail.source);
        }
      } catch (err) {
        if (!cancelled) {
          setError(errorMessage(err));
        }
      } finally {
        if (!cancelled) {
          setSourceLoading(false);
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [selectedName]);

  const handleCreate = () => {
    const trimmed = newName.trim();
    if (trimmed === "") {
      return;
    }
    void (async () => {
      try {
        const result = await createScript(trimmed);
        assertOk(result, "CreateScript");
        setNewName("");
        setCreating(false);
        await refresh();
        setSelectedName(trimmed);
      } catch (err) {
        setError(errorMessage(err));
      }
    })();
  };

  const handleSave = () => {
    if (!selectedName) {
      return;
    }
    void (async () => {
      try {
        const result = await saveScriptSource(selectedName, source);
        assertOk(result, "SaveScriptSource");
        await refresh();
      } catch (err) {
        setError(errorMessage(err));
      }
    })();
  };

  const handleDelete = () => {
    if (!selectedName) {
      return;
    }
    void (async () => {
      try {
        const result = await deleteScript(selectedName);
        assertOk(result, "DeleteScript");
        setConfirmingDelete(false);
        setSelectedName(null);
        await refresh();
      } catch (err) {
        setError(errorMessage(err));
      }
    })();
  };

  // handleDialogSubmit is ScriptRunDialog's onSubmit (08-10-PLAN.md Task 2/
  // Task 3): persist the edited profile (D-07: an edited profile becomes
  // the new saved default) via SetScriptProfile, then close the dialog and
  // fire RunScript/DebugScript. The launch call is deliberately NOT
  // awaited here: internal/wails.ScriptService.RunScript/DebugScript is a
  // full blocking Wails round trip that only resolves once the ENTIRE run
  // finishes (svc_script.go's decodeRunOutcome), so awaiting it here would
  // keep this dialog open and busy for the run's whole duration -- the
  // opposite of letting the user watch it live in ScriptDebugPanel below.
  // The dialog closes as soon as the profile save succeeds (this plan's
  // own flagged "spawn succeeds or fails" backstop truth, approximated
  // this way since the backend has no earlier "spawn started" signal to
  // await); the live onScriptEvent stream is the actual source of truth
  // for progress from this point on. The launch promise is still handled
  // here so a pre-flight failure (e.g. GOLC_SCRIPT_NOT_FOUND, unlikely but
  // possible if the script was deleted out from under this call) is never
  // silently lost -- it surfaces as a synthetic terminal state exactly
  // like a real crash would.
  const handleDialogSubmit = async (profile: ScriptProfileFields, mode: ScriptLaunchMode): Promise<void> => {
    if (!selectedName) return;
    const name = selectedName;

    const saveResult = await setScriptProfile(
      name,
      profile.scope,
      profile.preset,
      profile.deadlineSeconds,
      profile.ratePerSecond,
      profile.memoryLimitMB,
      profile.cpuCapPercent,
    );
    assertOk(saveResult, "SetScriptProfile");
    await refresh();

    setDialogMode(null);
    setSelectedFrameLine(null);
    // debugScript's breakpointLines argument is this workspace's own
    // gutter state (D-01) -- the author's exact line-coordinate set
    // currently marked in ScriptEditor's glyph margin, sent verbatim; the
    // backend (internal/command/scriptdebug.go) is the validation
    // authority and applies the shim-offset correction, never this client.
    const launch = mode === "debug" ? debugScript(name, breakpointLines) : runScript(name);
    void launch.catch((err) => {
      setPanelStateByScript((current) => {
        const previous = current[name] ?? IDLE_PANEL_STATE;
        return {
          ...current,
          [name]: { ...previous, liveStatus: "idle", terminal: { status: "failed", reason: errorMessage(err) } },
        };
      });
    });
  };

  // handleStop calls StopScript immediately, with no confirmation gesture
  // (D-10): a single click terminates exactly the selected script's own
  // run, never Phase 6's global Revoke Automation. The transient
  // "Stopping — finishing in-flight commands…" copy (D-11) is set here,
  // client-side, the instant the click happens -- it is cleared the moment
  // the run's own guaranteed terminal event arrives over onScriptEvent
  // (reduceScriptEvent always transitions liveStatus away from "stopping"
  // on a script.terminal event), never by a timer. Also clears pausedLine
  // immediately (08-12-PLAN.md Task 2): stopping a paused debug run clears
  // the execution-line highlight right away rather than waiting for the
  // guaranteed terminal event, which the plan's own must_haves calls out
  // as this exact single-click Stop Script action's expected behavior.
  const handleStop = () => {
    if (!selectedName) return;
    const name = selectedName;
    setPanelStateByScript((current) => {
      const previous = current[name] ?? IDLE_PANEL_STATE;
      return { ...current, [name]: { ...previous, liveStatus: "stopping", pausedLine: null } };
    });
    void (async () => {
      try {
        const result = await stopScript(name);
        assertOk(result, "StopScript");
      } catch (err) {
        setError(errorMessage(err));
      }
    })();
  };

  // handleValidate runs SCRP-01/SCRP-03's validate verb (never executes
  // the script); Run/Debug stay disabled below while the result carries
  // any diagnostic, until a later successful validation clears it.
  const handleValidate = () => {
    if (!selectedName) return;
    const name = selectedName;
    setValidating(true);
    void (async () => {
      try {
        setValidation(await validateScript(name));
      } catch (err) {
        setError(errorMessage(err));
      } finally {
        setValidating(false);
      }
    })();
  };

  // handleDismissPanel is ScriptDebugPanel's onDismiss: the only thing
  // that clears a frozen terminal state and its accumulated events back to
  // the pre-first-run placeholder (D-12 -- never a timer, never automatic).
  // Also clears any clicked crash-frame highlight (D-03) -- dismissing the
  // panel is the same "back to a clean idle state" action for both.
  const handleDismissPanel = () => {
    if (!selectedName) return;
    const name = selectedName;
    setSelectedFrameLine(null);
    setPanelStateByScript((current) => ({ ...current, [name]: IDLE_PANEL_STATE }));
  };

  // handleRunAgain is ScriptDebugPanel's onRunAgain: re-opens the launch
  // dialog rather than relaunching directly, so the (possibly still-
  // editable) profile is reviewed again before every run (D-07) -- there
  // is no direct-relaunch path anywhere in this file (D-13).
  const handleRunAgain = () => setDialogMode("run");

  // handleToggleBreakpoint is ScriptEditor's onToggleBreakpoint (D-01,
  // 08-12-PLAN.md Task 1/Task 2): a glyph-margin click on a line already in
  // the set removes it, otherwise adds it (kept sorted so the gutter's own
  // decoration order is stable/predictable, though ScriptEditor itself
  // doesn't depend on ordering).
  const handleToggleBreakpoint = useCallback((line: number) => {
    setBreakpointLines((current) =>
      current.includes(line)
        ? current.filter((existing) => existing !== line)
        : [...current, line].sort((a, b) => a - b),
    );
  }, []);

  // runDebugControl (D-01, 08-12-PLAN.md Task 2) wraps one of the four
  // step-control bridge calls identically to every other bridge-call
  // handler in this file: await, assertOk, surface a failure inline via
  // setError. It never touches panelStateByScript itself -- the backend's
  // own next script.status event (folded in by reduceScriptEvent, via the
  // existing onScriptEvent subscription above) is what actually advances
  // pausedLine/liveStatus. No optimistic local state (T-08-53): a click
  // that fails leaves the user paused with an inline error rather than in
  // a UI state the process isn't actually in.
  const runDebugControl = (action: () => Promise<WailsResult>, label: string) => {
    void (async () => {
      try {
        const result = await action();
        assertOk(result, label);
      } catch (err) {
        setError(errorMessage(err));
      }
    })();
  };

  const handleContinue = () => runDebugControl(continueScript, "ContinueScript");
  const handleStepOver = () => runDebugControl(stepOverScript, "StepOverScript");
  const handleStepInto = () => runDebugControl(stepIntoScript, "StepIntoScript");
  const handleStepOut = () => runDebugControl(stepOutScript, "StepOutScript");

  // handleSelectFrame is ScriptDebugPanel's onSelectFrame (D-03,
  // 08-12-PLAN.md Task 2): reveals a clicked crash-frame's line by reusing
  // ScriptEditor's existing currentExecutionLine highlight mechanism
  // (Task 1) -- see selectedFrameLine's own declaration above for why this
  // needed no new ScriptEditor API.
  const handleSelectFrame = (line: number) => setSelectedFrameLine(line);

  const selectedScript = scripts.find((script) => script.name === selectedName) ?? null;

  const bridgeMissing = !isScriptServiceAvailable();
  const panelState = selectedName ? (panelStateByScript[selectedName] ?? IDLE_PANEL_STATE) : IDLE_PANEL_STATE;
  const panelStatus: ScriptPanelStatus = bridgeMissing
    ? "offline"
    : panelState.terminal
      ? ((panelState.terminal.status || "terminated") as ScriptPanelStatus)
      : panelState.liveStatus;
  const isRunActive = panelStatus === "running" || panelStatus === "paused" || panelStatus === "stopping";
  const validationBlocksLaunch = validation !== null && !validation.valid;

  const inspectorPortal = useInspectorSlot(
    selectedScript ? (
      <div className={styles.inspector}>
        <span className={styles.inspectorLabel}>Capability scope</span>
        <span className={styles.inspectorValue}>{selectedScript.scope}</span>
        <span className={styles.inspectorLabel}>Resource limits</span>
        <span className={styles.inspectorValue}>{selectedScript.preset}</span>
      </div>
    ) : (
      <p className={styles.inspectorPlaceholder}>Select a script to see its capability profile.</p>
    ),
  );

  const newScriptForm = creating ? (
    <div className={styles.createForm}>
      <Field
        label="New script name"
        type="text"
        value={newName}
        placeholder="Script name"
        onChange={(event) => setNewName(event.target.value)}
        onKeyDown={(event) => {
          if (event.key === "Enter") {
            handleCreate();
          }
        }}
      />
      <FormActions>
        <Button variant="primary" icon={Check} onClick={handleCreate}>
          Create
        </Button>
      </FormActions>
    </div>
  ) : null;

  // toolbarActions is a genuine Base UI Toolbar use case (unlike this
  // file's earlier-considered, declined conversion of the design-system's
  // own `Toolbar` primitive, a workspace title bar): seven related actions
  // in one row benefit from real roving-tabindex Left/Right arrow-key
  // navigation between them, the way a rich-text editor's button row does.
  //
  // Each Button stays the actual event-handling/label/variant/icon owner --
  // Toolbar.Button's `render` prop merges Base UI's own computed props
  // (composite tabIndex, the onClick disabled-guard, roving-tabindex
  // key handling) onto the given Button element rather than replacing it,
  // confirmed via this session's Base UI docs lookup (context7 /mui/base-ui)
  // against Menu.tsx/Dialog.tsx's identical established pattern.
  //
  // focusableWhenDisabled={false} on every Toolbar.Button (Base UI's own
  // default inside a composite Toolbar is `true`, which keeps a disabled
  // item focusable by swapping to `aria-disabled` instead of the native
  // `disabled` attribute): Button.module.css's disabled styling
  // (opacity/cursor/hover/active) is written entirely against the native
  // `:disabled` pseudo-class, with every hover/active rule gated
  // `:not(:disabled)` -- `aria-disabled` alone would leave a "disabled"
  // Save/Run/etc. looking fully interactive (full opacity, hover states
  // still firing) while Base UI silently no-ops its click. Forcing the
  // native-disabled branch keeps Button's existing CSS contract intact
  // without touching Button.module.css (a primitive shared by every other
  // Button call site in the app) -- the one accepted trade-off is that
  // Left/Right arrow navigation skips a disabled item entirely rather than
  // landing focus on it to explain why it's disabled, an acceptable cost on
  // this non-live-critical authoring/debugging surface.
  const toolbarActions = (
    <BaseToolbar.Root className={styles.toolbarActions}>
      <BaseToolbar.Button
        focusableWhenDisabled={false}
        render={
          <Button variant="secondary" icon={creating ? X : Plus} onClick={() => setCreating((current) => !current)}>
            {creating ? "Cancel" : "New Script"}
          </Button>
        }
      />
      <BaseToolbar.Button
        disabled={!selectedName}
        focusableWhenDisabled={false}
        render={
          <Button variant="primary" icon={Save} onClick={handleSave}>
            Save
          </Button>
        }
      />
      <BaseToolbar.Button
        disabled={!selectedName}
        focusableWhenDisabled={false}
        render={
          <Button variant="destructive" icon={Trash2} onClick={() => setConfirmingDelete(true)}>
            Delete Script
          </Button>
        }
      />
      <BaseToolbar.Button
        disabled={!selectedName || validating}
        focusableWhenDisabled={false}
        render={
          <Button variant="secondary" icon={ShieldCheck} onClick={handleValidate}>
            Validate
          </Button>
        }
      />
      {/* Run/Debug/Stop Script (D-10): standard 32px Button height, not
          Phase 6's 64px safety-cluster treatment -- see this file's own
          top-of-file doc comment. */}
      <BaseToolbar.Button
        disabled={!selectedName || isRunActive || validationBlocksLaunch}
        focusableWhenDisabled={false}
        render={
          <Button variant="primary" icon={Play} onClick={() => setDialogMode("run")}>
            Run
          </Button>
        }
      />
      <BaseToolbar.Button
        disabled={!selectedName || isRunActive || validationBlocksLaunch}
        focusableWhenDisabled={false}
        render={
          <Button variant="secondary" icon={Bug} onClick={() => setDialogMode("debug")}>
            Debug
          </Button>
        }
      />
      <BaseToolbar.Button
        disabled={!selectedName || !isRunActive}
        focusableWhenDisabled={false}
        render={
          <Button variant="destructive" icon={Square} onClick={handleStop}>
            Stop Script
          </Button>
        }
      />
    </BaseToolbar.Root>
  );

  return (
    <div className={styles.workspace}>
      {inspectorPortal}
      <Toolbar title="Scripts" icon={FileCode2} info={HOW_IT_WORKS_BY_ID["build-scripts"]} action={toolbarActions} />
      <div className={styles.canvas}>
        {error ? <p className={styles.feedback}>{error}</p> : null}

        {!loading && scripts.length === 0 ? (
          <EmptyState
            icon={FileCode2}
            heading="No scripts yet"
            body="Create a script to automate GOLC through the typed SDK. Scripts run in an isolated process and can't touch playback or Art-Net directly."
            action={
              creating ? (
                newScriptForm
              ) : (
                <Button variant="primary" icon={Plus} onClick={() => setCreating(true)}>
                  New Script
                </Button>
              )
            }
          />
        ) : (
          <div className={styles.layout} style={{ "--ds-scriptslist-width": `${libraryPanel.size}px` } as CSSProperties}>
            <div className={styles.library}>
              {newScriptForm}
              <ResizeHandle
                edge="end"
                label="Resize script list"
                isResizing={libraryPanel.isResizing}
                onPointerDown={libraryPanel.handlePointerDown}
                onDoubleClick={libraryPanel.resetSize}
              />
              <ScrollRegion>
                {loading ? (
                  <ul className={styles.list} aria-label="Script list">
                    <li className={styles.pendingRow}>Loading scripts…</li>
                  </ul>
                ) : (
                  <ul className={styles.list} aria-label="Script list">
                    <AnimatePresence initial={false}>
                      {scripts.map((script) => (
                        <motion.li
                          key={script.id}
                          style={{ overflow: "hidden" }}
                          initial={false}
                          exit={{ opacity: 0, height: 0 }}
                          transition={rowExitTransition}
                        >
                          <ListRow
                            label={script.name}
                            icon={FileCode2}
                            meta={
                              <span className={styles.rowMeta}>
                                <Chip tone={chipToneForStatus(script.lastRunStatus)}>
                                  {statusLabel(script.lastRunStatus)}
                                </Chip>
                                <span className={styles.scopeLabel}>{script.scope}</span>
                              </span>
                            }
                            selected={script.name === selectedName}
                            onSelect={() => setSelectedName(script.name)}
                          />
                        </motion.li>
                      ))}
                    </AnimatePresence>
                  </ul>
                )}
              </ScrollRegion>
            </div>

            <div className={styles.editorColumn}>
              {selectedScript ? (
                <>
                  <div className={styles.editorHeader}>
                    <span className={styles.editorTitle} title={selectedScript.name}>
                      {selectedScript.name}
                    </span>
                  </div>

                  {confirmingDelete ? (
                    <div className={styles.deleteConfirm}>
                      <p className={styles.deleteConfirmText}>
                        {`Delete Script: This permanently removes ${selectedScript.name} and its saved capability profile from this show. This can't be undone.`}
                      </p>
                      <div className={styles.deleteConfirmActions}>
                        <Button variant="destructive" icon={Trash2} onClick={handleDelete}>
                          Delete Script
                        </Button>
                        <Button variant="secondary" icon={X} onClick={() => setConfirmingDelete(false)}>
                          Cancel
                        </Button>
                      </div>
                    </div>
                  ) : null}

                  {validation && !validation.valid ? (
                    <p className={styles.validationFeedback}>
                      {`This script has ${validation.diagnostics.length} error(s). Fix them before running.`}
                    </p>
                  ) : null}

                  {/* Real Monaco instance (D-15, 08-11-PLAN.md Task 3):
                      live TypeScript type-checking and autocomplete against
                      the generated GOLC SDK, themed to Paper/Ink.
                      breakpointLines/onToggleBreakpoint (D-01, Task 1) wire
                      the gutter; currentExecutionLine prefers the paused
                      run's own live line over a clicked crash-frame line
                      (D-03, Task 2) -- see selectedFrameLine's own
                      declaration above for why the two never fight over
                      the same highlight. */}
                  <ScriptEditor
                    value={source}
                    onChange={setSource}
                    readOnly={sourceLoading}
                    sdkTypeDefinitions={sdkTypeDefinitions}
                    ariaLabel={`${selectedScript.name} source`}
                    breakpointLines={breakpointLines}
                    onToggleBreakpoint={handleToggleBreakpoint}
                    currentExecutionLine={panelState.pausedLine ?? selectedFrameLine}
                  />

                  <ScriptDebugPanel
                    events={panelState.events}
                    status={panelStatus}
                    pausedLine={panelState.pausedLine}
                    terminalReason={panelState.terminal?.reason}
                    stackFrames={panelState.terminal ? deriveStackFramesFromReason(panelState.terminal.reason) : []}
                    onDismiss={handleDismissPanel}
                    onRunAgain={handleRunAgain}
                    onContinue={handleContinue}
                    onStepOver={handleStepOver}
                    onStepInto={handleStepInto}
                    onStepOut={handleStepOut}
                    onSelectFrame={handleSelectFrame}
                  />
                </>
              ) : (
                <p className={styles.noSelectionMessage}>Select a script to view and edit its source.</p>
              )}
            </div>
          </div>
        )}
      </div>

      {dialogMode && selectedScript ? (
        <ScriptRunDialog
          mode={dialogMode}
          scriptName={selectedScript.name}
          profile={{
            scope: selectedScript.scope,
            preset: selectedScript.preset,
            deadlineSeconds: selectedScript.deadlineSeconds,
            ratePerSecond: selectedScript.ratePerSecond,
            memoryLimitMB: selectedScript.memoryLimitMB,
            cpuCapPercent: selectedScript.cpuCapPercent,
          }}
          onSubmit={handleDialogSubmit}
          onCancel={() => setDialogMode(null)}
        />
      ) : null}
    </div>
  );
}
