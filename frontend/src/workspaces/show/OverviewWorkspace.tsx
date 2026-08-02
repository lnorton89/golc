// OverviewWorkspace is the Show group's landing workspace (application-
// shell-navigation.md): show identity (path, schema version, revision), a
// Diagnose action surfacing the same combined file-level + structural
// health check "show diagnose" runs, and the same pool/deployment summary
// "show inspect" already prints -- wired directly against ShowService
// (internal/wails/svc_show.go). This replaces the ComingSoon stub that
// pointed operators at "golc show open"/"golc show inspect" from the
// command line; there is no separate pool/deployment mutation path here,
// Build workspace already owns that (ScenesLooksWorkspace/PatchPoolsWorkspace).
//
// 09-03-PLAN.md (FDUI-03, D-08/D-10) extends the load callback to also
// read ListProgramming and, once both reads succeed, evaluate the
// auto-launch condition: a non-empty showPath (never the offline/no-
// bridge fallback, whose showPath is always "") AND zero pools AND zero
// deployments AND zero scenes. requestAutoLaunch() is itself a no-op
// after its first process-lifetime call (GuidedFirstShowContext.tsx), so
// a later remount with content already added never re-triggers it. The
// "Start Guide" action is the deliberate manual entry point for anyone
// else (D-10) -- no auto-launch banner, the guide simply takes over the
// canvas.
import { useCallback, useEffect, useState } from "react";
import { LayoutDashboard, Activity, Package, Boxes, Compass } from "lucide-react";

import {
  diagnoseShow,
  errorMessage,
  inspectShow,
  listProgramming,
  offlineShowInspectView,
  type DiagnosticReportView,
  type ShowInspectView,
} from "../../lib/wailsBridge";
import Toolbar from "../../components/primitives/Toolbar/Toolbar";
import { HOW_IT_WORKS_BY_ID } from "../../shell/navigation";
import Panel from "../../components/primitives/Panel/Panel";
import PanelHeader from "../../components/primitives/PanelHeader/PanelHeader";
import Button from "../../components/primitives/Button/Button";
import Chip from "../../components/primitives/Chip/Chip";
import ListRow from "../../components/primitives/ListRow/ListRow";
import ScrollRegion from "../../components/primitives/ScrollRegion/ScrollRegion";
import EmptyState from "../../components/primitives/EmptyState/EmptyState";
import { useGuidedFirstShow } from "./GuidedFirstShow/GuidedFirstShowContext";
import styles from "./OverviewWorkspace.module.css";

export default function OverviewWorkspace() {
  const [view, setView] = useState<ShowInspectView>(offlineShowInspectView());
  const [report, setReport] = useState<DiagnosticReportView | null>(null);
  const [loading, setLoading] = useState(true);
  const [diagnosing, setDiagnosing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const { startGuide, requestAutoLaunch, navigateTo } = useGuidedFirstShow();

  const refresh = useCallback(async (): Promise<void> => {
    try {
      const [next, programming] = await Promise.all([inspectShow(), listProgramming()]);
      setView(next);
      setError(null);

      const isGenuinelyEmpty =
        next.showPath !== "" &&
        next.pools.length === 0 &&
        next.deployments.length === 0 &&
        programming.scenes.length === 0;
      if (isGenuinelyEmpty) {
        requestAutoLaunch();
      }
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setLoading(false);
    }
  }, [requestAutoLaunch]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const handleDiagnose = async () => {
    setDiagnosing(true);
    try {
      const next = await diagnoseShow();
      setReport(next);
      setError(null);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setDiagnosing(false);
    }
  };

  return (
    <div className={styles.workspace}>
      <Toolbar title="Overview" icon={LayoutDashboard} info={HOW_IT_WORKS_BY_ID["show-overview"]} />
      <div className={styles.canvas}>
        {loading ? (
          <p className={styles.loading}>Loading show overview…</p>
        ) : (
          <>
            {error ? <p className={styles.errorText}>{error}</p> : null}
            <div className={styles.layout}>
              <Panel className={styles.identityPanel}>
                <PanelHeader
                  label="Show"
                  info="Shows the open show's file path, schema/revision, and structural health, with a Diagnose action that re-runs the same integrity check without leaving Overview."
                  action={
                    <Button
                      variant="secondary"
                      icon={Activity}
                      disabled={diagnosing}
                      onClick={() => void handleDiagnose()}
                    >
                      {diagnosing ? "Diagnosing…" : "Diagnose"}
                    </Button>
                  }
                />
                <div className={styles.identity}>
                  <span className={styles.path} title={view.showPath}>
                    {view.showPath || "(unsaved show)"}
                  </span>
                  <span className={styles.meta}>
                    Schema {view.schemaVersion} · Revision {view.revision}
                  </span>
                  {report ? (
                    <Chip tone={report.structuralOk ? "frame-lock" : "revoked"}>
                      {report.structuralOk ? "Healthy" : "Issues found"}
                    </Chip>
                  ) : null}
                  {report?.migrationRequired ? <Chip tone="armed">Migration required</Chip> : null}
                </div>
                {report && !report.structuralOk && report.structuralError ? (
                  <p className={styles.errorText}>{report.structuralError}</p>
                ) : null}
                {report && report.fileLevelIssues.length > 0 ? (
                  <ul className={styles.issueList} aria-label="File-level issues">
                    {report.fileLevelIssues.map((issue, index) => (
                      <li key={index}>{issue}</li>
                    ))}
                  </ul>
                ) : null}
              </Panel>

              <Panel>
                <PanelHeader
                  label={`Pools (${view.pools.length})`}
                  info="Lists every fixture pool in the show and how many instances belong to each; selecting one jumps to Patch & Pools."
                />
                <ScrollRegion>
                  {view.pools.length === 0 ? (
                    <EmptyState icon={Package}>No fixture pools yet.</EmptyState>
                  ) : (
                    <ul className={styles.list} aria-label="Pool list">
                      {view.pools.map((pool) => (
                        <li key={pool.id}>
                          <ListRow
                            label={pool.name}
                            icon={Package}
                            meta={`${pool.memberCount} member${pool.memberCount === 1 ? "" : "s"}`}
                            onSelect={() => navigateTo("build-patch-pools")}
                          />
                        </li>
                      ))}
                    </ul>
                  )}
                </ScrollRegion>
              </Panel>

              <Panel>
                <PanelHeader
                  label={`Deployments (${view.deployments.length})`}
                  info="Lists every deployment (an active grouping of patched instances) in the show; selecting one jumps to Patch & Pools."
                />
                <ScrollRegion>
                  {view.deployments.length === 0 ? (
                    <EmptyState icon={Boxes}>No deployments yet.</EmptyState>
                  ) : (
                    <ul className={styles.list} aria-label="Deployment list">
                      {view.deployments.map((deployment) => (
                        <li key={deployment.id}>
                          <ListRow
                            label={deployment.name}
                            icon={Boxes}
                            meta={
                              deployment.active
                                ? "Active"
                                : `${deployment.instanceCount} instance${deployment.instanceCount === 1 ? "" : "s"}`
                            }
                            onSelect={() => navigateTo("build-patch-pools")}
                          />
                        </li>
                      ))}
                    </ul>
                  )}
                </ScrollRegion>
              </Panel>
            </div>

            {/* Start Guide (D-10): the deliberate manual entry point into
                Guided First Show for a show that already has content --
                separated from the rest of the surface by the 48px
                major-section-break token, per 09-UI-SPEC.md's Spacing
                Scale. No auto-launch banner anywhere here; a genuinely
                empty show (see refresh() above) takes over the canvas
                immediately instead. */}
            <div className={styles.guideCta}>
              <Button variant="primary" icon={Compass} onClick={startGuide}>
                Start Guide
              </Button>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
