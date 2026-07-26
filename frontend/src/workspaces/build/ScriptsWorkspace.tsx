// ScriptsWorkspace is Build -> Scripts (08-04-PLAN.md Task 2, SCRP-01/
// D-16): the D-16 script library view, the create/edit/save/delete round
// trip, and (via useInspectorSlot) the selected script's capability-profile
// summary in the contextual inspector. Owns every ScriptService call and
// all script state, following ScenesLooksWorkspace.tsx's exact load/
// refresh/error and selection-validity-repair pattern (08-PATTERNS.md) --
// the correct structural template per 08-UI-SPEC.md's correction of
// RESEARCH.md (FixtureLibraryWorkspace.tsx is a bare ComingSoon stub, not a
// library pattern).
//
// This plan's editing surface is a plain bounded <textarea> styled to the
// Technical readout row (see the <textarea> element below for the D-15/
// 08-11 handoff note). Run/Debug/Validate/Stop Script (08-10/08-11) extend
// this same Toolbar action slot and this same editor region in place --
// this file is written so that extension needs no rewrite.
import { useCallback, useEffect, useState } from "react";

import {
  assertOk,
  createScript,
  deleteScript,
  errorMessage,
  getScript,
  listScripts,
  saveScriptSource,
  type ScriptSummaryView,
} from "../../lib/wailsBridge";

import Toolbar from "../../components/primitives/Toolbar/Toolbar";
import ScrollRegion from "../../components/primitives/ScrollRegion/ScrollRegion";
import ListRow from "../../components/primitives/ListRow/ListRow";
import Chip, { type ChipTone } from "../../components/primitives/Chip/Chip";
import Button from "../../components/primitives/Button/Button";
import { useInspectorSlot } from "../../shell/InspectorSlot";
import styles from "./ScriptsWorkspace.module.css";

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

  const selectedScript = scripts.find((script) => script.name === selectedName) ?? null;

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

                  {/* Plain bounded textarea -- this plan's editing surface
                      only. 08-11 replaces this element in place with the
                      Monaco editor (D-15's live TypeScript language service,
                      autocomplete, and the D-01 breakpoint gutter); do not
                      mistake this <textarea> for a finished editor. */}
                  <textarea
                    className={styles.editor}
                    value={source}
                    spellCheck={false}
                    wrap="off"
                    disabled={sourceLoading}
                    aria-label={`${selectedScript.name} source`}
                    onChange={(event) => setSource(event.target.value)}
                  />
                </>
              ) : (
                <p className={styles.emptySelection}>Select a script to view and edit its source.</p>
              )}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
