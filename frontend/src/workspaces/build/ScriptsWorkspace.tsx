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
import { useCallback, useEffect, useState } from "react";

import {
  assertOk,
  continueScript,
  createScript,
  debugScript,
  deleteScript,
  errorMessage,
  getScript,
  getSdkTypeDefinitions,
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

import Toolbar from "../../components/primitives/Toolbar/Toolbar";
import ScrollRegion from "../../components/primitives/ScrollRegion/ScrollRegion";
import ListRow from "../../components/primitives/ListRow/ListRow";
import Chip, { type ChipTone } from "../../components/primitives/Chip/Chip";
import Button from "../../components/primitives/Button/Button";
import ScriptEditor from "../../components/Scripts/ScriptEditor";
import ScriptRunDialog, { type ScriptDialogProfile as ScriptProfileFields, type ScriptLaunchMode } from "../../components/Scripts/ScriptRunDialog";
import ScriptDebugPanel, { type ScriptPanelStatus } from "../../components/Scripts/ScriptDebugPanel";
import { useInspectorSlot } from "../../shell/InspectorSlot";
import styles from "./ScriptsWorkspace.module.css";

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
// not bound -- both a missing window.go bridge (jsdom, a plain browser
// preview) and a rejected ListScripts call resolve to this same message,
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
      const bridgeMissing = typeof window === "undefined" || !window.go?.wails?.ScriptService;
      setError(bridgeMissing ? HOST_UNREACHABLE_MESSAGE : null);
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

  const bridgeMissing = typeof window === "undefined" || !window.go?.wails?.ScriptService;
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
      <p className={styles.inspectorEmpty}>Select a script to see its capability profile.</p>
    ),
  );

  const newScriptForm = creating ? (
    <div className={styles.createForm}>
      <input
        className={styles.createInput}
        type="text"
        value={newName}
        placeholder="Script name"
        aria-label="New script name"
        onChange={(event) => setNewName(event.target.value)}
        onKeyDown={(event) => {
          if (event.key === "Enter") {
            handleCreate();
          }
        }}
      />
      <Button variant="primary" onClick={handleCreate}>
        Create
      </Button>
    </div>
  ) : null;

  const toolbarActions = (
    <div className={styles.toolbarActions}>
      <Button variant="secondary" onClick={() => setCreating((current) => !current)}>
        {creating ? "Cancel" : "New Script"}
      </Button>
      <Button variant="primary" onClick={handleSave} disabled={!selectedName}>
        Save
      </Button>
      <Button variant="destructive" onClick={() => setConfirmingDelete(true)} disabled={!selectedName}>
        Delete Script
      </Button>
      <Button variant="secondary" onClick={handleValidate} disabled={!selectedName || validating}>
        Validate
      </Button>
      {/* Run/Debug/Stop Script (D-10): standard 32px Button height, not
          Phase 6's 64px safety-cluster treatment -- see this file's own
          top-of-file doc comment. */}
      <Button
        variant="primary"
        onClick={() => setDialogMode("run")}
        disabled={!selectedName || isRunActive || validationBlocksLaunch}
      >
        Run
      </Button>
      <Button
        variant="secondary"
        onClick={() => setDialogMode("debug")}
        disabled={!selectedName || isRunActive || validationBlocksLaunch}
      >
        Debug
      </Button>
      <Button variant="destructive" onClick={handleStop} disabled={!selectedName || !isRunActive}>
        Stop Script
      </Button>
    </div>
  );

  return (
    <div className={styles.workspace}>
      {inspectorPortal}
      <Toolbar title="Scripts" action={toolbarActions} />
      <div className={styles.canvas}>
        {error ? <p className={styles.errorText}>{error}</p> : null}

        {!loading && scripts.length === 0 ? (
          <div className={styles.emptyState}>
            <h3 className={styles.emptyHeading}>No scripts yet</h3>
            <p className={styles.emptyBody}>
              {"Create a script to automate GOLC through the typed SDK. Scripts run in an isolated process and can't touch playback or Art-Net directly."}
            </p>
            {creating ? null : (
              <Button variant="primary" onClick={() => setCreating(true)}>
                New Script
              </Button>
            )}
            {newScriptForm}
          </div>
        ) : (
          <div className={styles.layout}>
            <div className={styles.library}>
              {newScriptForm}
              <ScrollRegion>
                {loading ? (
                  <ul className={styles.list} aria-label="Script list">
                    <li className={styles.loadingRow}>Loading scripts…</li>
                  </ul>
                ) : (
                  <ul className={styles.list} aria-label="Script list">
                    {scripts.map((script) => (
                      <li key={script.id}>
                        <ListRow
                          label={script.name}
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
                      </li>
                    ))}
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
                        <Button variant="destructive" onClick={handleDelete}>
                          Delete Script
                        </Button>
                        <Button variant="secondary" onClick={() => setConfirmingDelete(false)}>
                          Cancel
                        </Button>
                      </div>
                    </div>
                  ) : null}

                  {validation && !validation.valid ? (
                    <p className={styles.validationError}>
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
                <p className={styles.emptySelection}>Select a script to view and edit its source.</p>
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
