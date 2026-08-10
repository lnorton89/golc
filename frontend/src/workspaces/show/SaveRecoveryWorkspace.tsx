// SaveRecoveryWorkspace is the Show group's save/recovery workspace
// (application-shell-navigation.md): Save/Save As drive the same "show
// save"/"show save-as" routes the CLI already implements and tests, and
// the recovery-point list surfaces exactly what "golc show open"'s own
// offer/accept/discard flow does -- wired directly against ShowService
// (internal/wails/svc_show.go) rather than a second recovery
// implementation. This replaces the ComingSoon stub that pointed operators
// at "golc show save"/"golc show save-as"/the recovery-offer flow on
// "golc show open" from the command line.
//
// Opening, creating, and switching shows lives in the Shows workspace
// (ShowsWorkspace.tsx, 09-02-PLAN.md, FDUI-02) -- this workspace remains
// scoped to save / save-as / recovery points for the currently open show. A
// migration-required note (surfaced via Diagnose, same as
// OverviewWorkspace) points an operator at the CLI's own
// "--confirm-migration" flag rather than adding a second on-screen
// migration trigger -- migration is a rare, one-time, high-consequence
// action (a verified backup, then an atomic schema rewrite) this round
// keeps as an explicit, deliberate command-line step.
import { useCallback, useEffect, useState } from "react";
import { Save as SaveIcon, Copy, History, Check, Trash2 } from "lucide-react";

import {
  acceptRecoveryPoint,
  assertOk,
  detectRecoveryPoints,
  diagnoseShow,
  discardRecoveryPoints,
  errorMessage,
  saveShow,
  saveShowAs,
  type RecoveryPointView,
} from "../../lib/wailsBridge";
import { HOW_IT_WORKS_BY_ID } from "../../shell/navigation";
import { Button, ConfirmDialog, EmptyState, ErrorState, Field, FormActions, LoadingState, Panel, PanelHeader, ScrollRegion, WorkspaceFrame } from "../../design-system";
import styles from "./SaveRecoveryWorkspace.module.css";

export default function SaveRecoveryWorkspace() {
  const [points, setPoints] = useState<RecoveryPointView[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [migrationRequired, setMigrationRequired] = useState(false);

  const [saving, setSaving] = useState(false);
  const [destPath, setDestPath] = useState("");
  const [savingAs, setSavingAs] = useState(false);
  const [acceptingId, setAcceptingId] = useState<number | null>(null);
  const [discarding, setDiscarding] = useState(false);
  // Discard All permanently deletes every crash-recovery snapshot for the
  // open show, and it sits directly beside the per-row "Accept" button on
  // a list the operator is scanning -- a misclick destroyed the only copy
  // of unsaved work with no confirmation of any kind. ConfirmDialog is the
  // design system's public confirmation contract (AppShell.tsx uses it for
  // the leave-the-guide prompt).
  const [confirmingDiscardAll, setConfirmingDiscardAll] = useState(false);

  const refresh = useCallback(async (): Promise<void> => {
    try {
      const [nextPoints, report] = await Promise.all([detectRecoveryPoints(), diagnoseShow()]);
      setPoints(nextPoints);
      setMigrationRequired(report.migrationRequired);
      setError(null);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const handleSave = async () => {
    setSaving(true);
    try {
      const result = await saveShow();
      assertOk(result, "Save");
      await refresh();
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setSaving(false);
    }
  };

  const handleSaveAs = async () => {
    const trimmed = destPath.trim();
    if (trimmed === "") {
      return;
    }
    setSavingAs(true);
    try {
      const result = await saveShowAs(trimmed);
      assertOk(result, "SaveAs");
      setDestPath("");
      setError(null);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setSavingAs(false);
    }
  };

  const handleAccept = async (id: number) => {
    setAcceptingId(id);
    try {
      const result = await acceptRecoveryPoint(id);
      assertOk(result, "AcceptRecoveryPoint");
      await refresh();
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setAcceptingId(null);
    }
  };

  const handleDiscardAll = async () => {
    setConfirmingDiscardAll(false);
    setDiscarding(true);
    try {
      const result = await discardRecoveryPoints();
      assertOk(result, "DiscardRecoveryPoints");
      await refresh();
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setDiscarding(false);
    }
  };

  return (
    <WorkspaceFrame
      title="Save & Recovery"
      info={HOW_IT_WORKS_BY_ID["show-save-recovery"]}
    >
      <div className={styles.canvas}>
        {loading ? (
          <LoadingState label="Loading save & recovery…" variant="panel" />
        ) : (
          <>
            {error ? <ErrorState heading="Save & Recovery unavailable" message={error} /> : null}
            {migrationRequired ? (
              <p className={styles.migrationNote}>
                This show needs a schema migration before it can be edited further. Run{" "}
                <code>golc show open --show &lt;path&gt; --confirm-migration</code> from the command line -- a
                verified backup is made first.
              </p>
            ) : null}

            <div className={styles.layout}>
              <Panel>
                <PanelHeader
                  label="Save"
                  icon={SaveIcon}
                  info="Saves the open show to its own file, or saves a copy to a different path."
                />
                <FormActions>
                  <Button variant="primary" icon={SaveIcon} disabled={saving} onClick={() => void handleSave()}>
                    {saving ? "Saving…" : "Save"}
                  </Button>
                  <div className={styles.destPathField}>
                    <Field
                      label="Save As destination path"
                      hideLabel
                      value={destPath}
                      placeholder="Save a copy to path…"
                      onChange={(event) => setDestPath(event.target.value)}
                      onKeyDown={(event) => {
                        if (event.key === "Enter") {
                          void handleSaveAs();
                        }
                      }}
                    />
                  </div>
                  <Button variant="secondary" icon={Copy} disabled={savingAs} onClick={() => void handleSaveAs()}>
                    {savingAs ? "Saving…" : "Save As"}
                  </Button>
                </FormActions>
              </Panel>

              <Panel>
                <PanelHeader
                  label={`Recovery Points (${points.length})`}
                  icon={History}
                  info="Lists crash/recovery snapshots the desktop app retained for this show, so you can restore from one instead of losing unsaved work."
                  action={
                    points.length > 0 ? (
                      <Button
                        variant="destructive"
                        icon={Trash2}
                        disabled={discarding}
                        onClick={() => setConfirmingDiscardAll(true)}
                      >
                        {discarding ? "Discarding…" : "Discard All"}
                      </Button>
                    ) : undefined
                  }
                />
                <ScrollRegion className={styles.recoveryScroll}>
                  {points.length === 0 ? (
                    <EmptyState icon={History}>
                      No interrupted-session recovery points are currently offered.
                    </EmptyState>
                  ) : (
                    <ul className={styles.list} aria-label="Recovery point list">
                      {points.map((point) => (
                        <li key={point.id} className={styles.pointRow}>
                          <div className={styles.pointMeta}>
                            <span className={styles.pointRevision}>Revision {point.revision}</span>
                            <span className={styles.pointCreatedAt}>{point.createdAt}</span>
                          </div>
                          <Button
                            variant="primary"
                            icon={Check}
                            disabled={acceptingId !== null}
                            onClick={() => void handleAccept(point.id)}
                          >
                            {acceptingId === point.id ? "Accepting…" : "Accept"}
                          </Button>
                        </li>
                      ))}
                    </ul>
                  )}
                </ScrollRegion>
              </Panel>
            </div>
          </>
        )}
      </div>

      <ConfirmDialog
        open={confirmingDiscardAll}
        title="Discard all recovery points?"
        message={`This permanently deletes ${points.length} crash-recovery snapshot${
          points.length === 1 ? "" : "s"
        } for this show. Anything not already saved into the show file is lost, and this can't be undone.`}
        confirmLabel="Discard All"
        cancelLabel="Keep Them"
        destructive
        onConfirm={() => void handleDiscardAll()}
        onCancel={() => setConfirmingDiscardAll(false)}
      />
    </WorkspaceFrame>
  );
}
