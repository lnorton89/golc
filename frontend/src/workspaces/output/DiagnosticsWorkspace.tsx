// DiagnosticsWorkspace is the Output group's integrity-check workspace
// (application-shell-navigation.md): the same combined file-level (PRAGMA
// integrity_check) + structural (validate()) health check "show diagnose"
// already runs, wired directly against ShowService.Diagnose
// (internal/wails/svc_show.go) -- the identical binding OverviewWorkspace's
// own inline Diagnose action uses. This replaces the ComingSoon stub that
// pointed operators at "golc show diagnose" from the command line. Unlike
// Overview's inline chip (a secondary action alongside pool/deployment
// browsing), this workspace's whole job is the diagnostic report, so it
// runs automatically on mount rather than waiting for a manual click.
import { useCallback, useEffect, useState } from "react";

import {
  diagnoseShow,
  errorMessage,
  offlineDiagnosticReport,
  type DiagnosticReportView,
} from "../../lib/wailsBridge";
import Toolbar from "../../components/primitives/Toolbar/Toolbar";
import Panel from "../../components/primitives/Panel/Panel";
import PanelHeader from "../../components/primitives/PanelHeader/PanelHeader";
import Button from "../../components/primitives/Button/Button";
import Chip from "../../components/primitives/Chip/Chip";
import styles from "./DiagnosticsWorkspace.module.css";

export default function DiagnosticsWorkspace() {
  const [report, setReport] = useState<DiagnosticReportView>(offlineDiagnosticReport());
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async (): Promise<void> => {
    try {
      const next = await diagnoseShow();
      setReport(next);
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

  const handleRerun = async () => {
    setLoading(true);
    await refresh();
  };

  return (
    <div className={styles.workspace}>
      <Toolbar title="Diagnostics" />
      <div className={styles.canvas}>
        {loading ? (
          <p className={styles.loading}>Running diagnostics…</p>
        ) : (
          <>
            {error ? <p className={styles.errorText}>{error}</p> : null}
            <Panel>
              <PanelHeader
                label="Integrity Check"
                action={
                  <Button variant="secondary" onClick={() => void handleRerun()}>
                    Re-run
                  </Button>
                }
              />
              <div className={styles.summary}>
                <Chip tone={report.structuralOk ? "frame-lock" : "revoked"}>
                  {report.structuralOk ? "Healthy" : "Issues found"}
                </Chip>
                {report.migrationRequired ? <Chip tone="armed">Migration required</Chip> : null}
                <span className={styles.meta}>
                  Schema {report.schemaVersion} · Revision {report.revision}
                </span>
              </div>

              {report.structuralError ? (
                <p className={styles.structuralError}>{report.structuralError}</p>
              ) : null}

              {report.migrationRequired ? (
                <p className={styles.migrationNote}>
                  Run <code>golc show open --show &lt;path&gt; --confirm-migration</code> from the command line to
                  migrate -- a verified backup is made first.
                </p>
              ) : null}

              {report.fileLevelIssues.length === 0 ? (
                <p className={styles.emptyState}>No file-level integrity issues found.</p>
              ) : (
                <ul className={styles.issueList} aria-label="File-level issues">
                  {report.fileLevelIssues.map((issue, index) => (
                    <li key={index}>{issue}</li>
                  ))}
                </ul>
              )}
            </Panel>
          </>
        )}
      </div>
    </div>
  );
}
