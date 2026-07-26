// ScriptsWorkspace is Build -> Scripts (08-04-PLAN.md Task 2, SCRP-01/
// D-16, extended by 08-10-PLAN.md Task 3 for SCRP-04/SCRP-05): the D-16
// script library view, the create/edit/save/delete round trip, the Run/
// Debug/Validate/Stop Script toolbar actions, the Run/Debug launch dialog,
// and (via useInspectorSlot) the selected script's capability-profile
// summary in the contextual inspector. Owns every ScriptService call and
// all script state, following ScenesLooksWorkspace.tsx's exact load/
// refresh/error and selection-validity-repair pattern (08-PATTERNS.md) --
// the correct structural template per 08-UI-SPEC.md's correction of
// RESEARCH.md (FixtureLibraryWorkspace.tsx is a bare ComingSoon stub, not a
// library pattern).
//
// The editing surface is a real Monaco instance (ScriptEditor.tsx, 08-11-
// PLAN.md Task 3, D-15) running the TypeScript language service against
// the generated GOLC SDK -- replacing 08-04's plain bounded plaintext
// input element in place. The D-01 breakpoint gutter (DebugScript's
// breakpointLines argument) has
// no UI source yet -- this plan still calls DebugScript with an empty
// breakpoint list, matching "no --breakpoint flags launches in Debug mode
// with no breakpoints and immediately resumes".
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
  stopScript,
  validateScript,
  type ScriptEventView,
  type ScriptSummaryView,
  type ScriptValidationView,
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
// Task 3): events in arrival order (log/outcome/status/gap), the current
// sub-state derived from the most recent script.status event, and the
// frozen terminal status/reason once a run ends -- D-12: terminal never
// clears itself, only handleDismissPanel does.
interface ScriptPanelState {
  events: ScriptEventView[];
  runId: string | null;
  liveStatus: "idle" | "running" | "paused" | "stopping";
  terminal: { status: string; reason: string } | null;
}

const IDLE_PANEL_STATE: ScriptPanelState = { events: [], runId: null, liveStatus: "idle", terminal: null };

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
    return { events, runId, liveStatus: "idle", terminal: { status: event.status ?? "", reason: event.reason ?? "" } };
  }

  if (event.kind === "script.status") {
    const liveStatus: ScriptPanelState["liveStatus"] = event.reason?.startsWith("GOLC_SCRIPT_DEBUG_PAUSED")
      ? "paused"
      : "running";
    return { events, runId, liveStatus, terminal: isNewRun ? null : state.terminal };
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
  // a stale "N error(s)" summary for the wrong source.
  useEffect(() => {
    setValidation(null);
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
    const launch = mode === "debug" ? debugScript(name, []) : runScript(name);
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
  // on a script.terminal event), never by a timer.
  const handleStop = () => {
    if (!selectedName) return;
    const name = selectedName;
    setPanelStateByScript((current) => {
      const previous = current[name] ?? IDLE_PANEL_STATE;
      return { ...current, [name]: { ...previous, liveStatus: "stopping" } };
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
  const handleDismissPanel = () => {
    if (!selectedName) return;
    const name = selectedName;
    setPanelStateByScript((current) => ({ ...current, [name]: IDLE_PANEL_STATE }));
  };

  // handleRunAgain is ScriptDebugPanel's onRunAgain: re-opens the launch
  // dialog rather than relaunching directly, so the (possibly still-
  // editable) profile is reviewed again before every run (D-07) -- there
  // is no direct-relaunch path anywhere in this file (D-13).
  const handleRunAgain = () => setDialogMode("run");

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
                      the generated GOLC SDK, themed to Paper/Ink. */}
                  <ScriptEditor
                    value={source}
                    onChange={setSource}
                    readOnly={sourceLoading}
                    sdkTypeDefinitions={sdkTypeDefinitions}
                    ariaLabel={`${selectedScript.name} source`}
                  />

                  <ScriptDebugPanel
                    events={panelState.events}
                    status={panelStatus}
                    terminalReason={panelState.terminal?.reason}
                    stackFrames={panelState.terminal ? deriveStackFramesFromReason(panelState.terminal.reason) : []}
                    onDismiss={handleDismissPanel}
                    onRunAgain={handleRunAgain}
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
