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
// This workspace does not bind "show open"/a file-picker: the desktop app
// resolves exactly one show path at startup (cmd/golc-desktop/main.go's
// GOLC_DESKTOP_SHOW), so there is no "open a different show" flow to wire
// yet. A migration-required note (surfaced via Diagnose, same as
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
import Toolbar from "../../components/primitives/Toolbar/Toolbar";
import Panel from "../../components/primitives/Panel/Panel";
import PanelHeader from "../../components/primitives/PanelHeader/PanelHeader";
import Button from "../../components/primitives/Button/Button";
import ScrollRegion from "../../components/primitives/ScrollRegion/ScrollRegion";
import EmptyState from "../../components/primitives/EmptyState/EmptyState";
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
    <div className={styles.workspace}>
      <Toolbar title="Save & Recovery" icon={SaveIcon} />
      <div className={styles.canvas}>
        {loading ? (
          <p className={styles.loading}>Loading save & recovery…</p>
        ) : (
          <>
            {error ? <p className={styles.errorText}>{error}</p> : null}
            {migrationRequired ? (
              <p className={styles.migrationNote}>
                This show needs a schema migration before it can be edited further. Run{" "}
                <code>golc show open --show &lt;path&gt; --confirm-migration</code> from the command line -- a
                verified backup is made first.
              </p>
            ) : null}

            <div className={styles.layout}>
              <Panel>
                <PanelHeader label="Save" icon={SaveIcon} />
                <div className={styles.saveRow}>
                  <Button variant="primary" icon={SaveIcon} disabled={saving} onClick={() => void handleSave()}>
                    {saving ? "Saving…" : "Save"}
                  </Button>
                  <input
                    className={styles.destInput}
                    type="text"
                    value={destPath}
                    placeholder="Save a copy to path…"
                    aria-label="Save As destination path"
                    onChange={(event) => setDestPath(event.target.value)}
                    onKeyDown={(event) => {
                      if (event.key === "Enter") {
                        void handleSaveAs();
                      }
                    }}
                  />
                  <Button variant="secondary" icon={Copy} disabled={savingAs} onClick={() => void handleSaveAs()}>
                    {savingAs ? "Saving…" : "Save As"}
                  </Button>
                </div>
              </Panel>

              <Panel>
                <PanelHeader
                  label={`Recovery Points (${points.length})`}
                  icon={History}
                  action={
                    points.length > 0 ? (
                      <Button
                        variant="destructive"
                        icon={Trash2}
                        disabled={discarding}
                        onClick={() => void handleDiscardAll()}
                      >
                        {discarding ? "Discarding…" : "Discard All"}
                      </Button>
                    ) : undefined
                  }
                />
                <ScrollRegion>
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
    </div>
  );
}
