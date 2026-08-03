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
//
// This workspace also renders the "Application Log" panel (AppLogPanel.tsx),
// reading accumulated app-wide log lines from the shared store's `appLog`
// slice (useGolcStore, store.ts) rather than subscribing to the Go host's
// "app:log" push itself: AppLogStream.tsx (shell/, mounted unconditionally
// inside GlobalFrame) is that push's sole subscriber/writer, started as
// soon as the app shell mounts -- well before an operator has necessarily
// navigated here. Most "app:log" lines fire during App.OnStartup, within
// the first moments the window opens; a subscription scoped to this
// workspace's own mount lifetime (as it originally was) would miss every
// line pushed before the first visit, since Wails' EventsEmit is fire-and-
// forget and never replayed to a late listener -- see AppLogStream.tsx's
// own doc comment.
import { useCallback, useEffect, useState } from "react";
import { Activity, RefreshCw, ListChecks } from "lucide-react";

import { diagnoseShow, errorMessage, offlineDiagnosticReport, type DiagnosticReportView } from "../../lib/wailsBridge";
import { useGolcStore } from "../../store/store";
import Toolbar from "../../components/primitives/Toolbar/Toolbar";
import { HOW_IT_WORKS_BY_ID } from "../../shell/navigation";
import Panel from "../../components/primitives/Panel/Panel";
import PanelHeader from "../../components/primitives/PanelHeader/PanelHeader";
import Button from "../../components/primitives/Button/Button";
import Chip from "../../components/primitives/Chip/Chip";
import EmptyState from "../../components/primitives/EmptyState/EmptyState";
import ErrorState from "../../components/primitives/ErrorState/ErrorState";
import LoadingState from "../../components/primitives/LoadingState/LoadingState";
import AppLogPanel from "../../components/Diagnostics/AppLogPanel";
import styles from "./DiagnosticsWorkspace.module.css";

export default function DiagnosticsWorkspace() {
  const [report, setReport] = useState<DiagnosticReportView>(offlineDiagnosticReport());
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const logEvents = useGolcStore((state) => state.appLog);
  const clearAppLog = useGolcStore((state) => state.clearAppLog);

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
      <Toolbar title="Diagnostics" icon={Activity} info={HOW_IT_WORKS_BY_ID["output-diagnostics"]} />
      <div className={styles.canvas}>
        {loading ? (
          <LoadingState label="Running diagnostics…" variant="panel" />
        ) : (
          <>
            {error ? <ErrorState heading="Diagnostics unavailable" message={error} variant="inline" /> : null}
            <Panel>
              <PanelHeader
                label="Integrity Check"
                icon={ListChecks}
                info="Runs the same combined file-level and structural health check 'show diagnose' runs from the command line, and lets you re-run it on demand."
                action={
                  <Button variant="secondary" icon={RefreshCw} onClick={() => void handleRerun()}>
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
                <p className={styles.structuralDetail}>{report.structuralError}</p>
              ) : null}

              {report.migrationRequired ? (
                <p className={styles.migrationNote}>
                  Run <code>golc show open --show &lt;path&gt; --confirm-migration</code> from the command line to
                  migrate -- a verified backup is made first.
                </p>
              ) : null}

              {report.fileLevelIssues.length === 0 ? (
                <EmptyState>No file-level integrity issues found.</EmptyState>
              ) : (
                <ul className={styles.issueList} aria-label="File-level issues">
                  {report.fileLevelIssues.map((issue, index) => (
                    <li key={index}>{issue}</li>
                  ))}
                </ul>
              )}
            </Panel>

            <AppLogPanel events={logEvents} onClear={clearAppLog} />
          </>
        )}
      </div>
    </div>
  );
}
