// OperatorSurface.tsx is the feature region for the operator-surface
// builder (06-07-PLAN.md Task 2, PLAY-03 D-01..D-04). It composes
// SurfaceList.tsx (multiple named surfaces, D-02) with a per-control detail
// list toggled between two modes for the one selected surface:
//
//   - "author" mode renders AssignmentToggle.tsx in place on every
//     scene/layer/master/safety control -- the in-place "add to this
//     operator surface" checkbox (D-01), individual-item only, never a
//     bulk/category control (D-03).
//   - "operate" mode is the visible-but-locked renderer (D-04): every
//     control always renders, assigned or not -- assigned is full opacity
//     with the Signal Blue selection indicator, unassigned is reduced
//     opacity and non-interactive. Never hidden.
//
// The lock is enforced server-side by internal/wails/svc_surface.go's
// AuthorizeControl/command.Authorize (D-04/ASVS V4) -- this component's own
// dimmed/disabled rendering is a UI affordance only, never the actual
// enforcement. Entering "operate" mode for a surface also calls
// SafetyService/PlaybackService's own SetActiveSurface (CR-01 fix,
// wailsBridge.ts's setSafetyActiveSurface/setPlaybackActiveSurface) so the
// real dispatch paths those two services own are scoped to this surface's
// assignments for as long as the preview is active -- leaving "operate"
// mode (or switching/deselecting the surface) clears both back to
// unrestricted/author-mode dispatch.
//
// All Go-bound calls go through wailsBridge.ts's requireSurfaceService
// accessor (that module owns every read of Wails v2's runtime-injected
// bridge for the internal/wails.SurfaceService struct);
// this file owns every SurfaceService call in the component tree --
// SurfaceList.tsx and AssignmentToggle.tsx are purely presentational and
// receive data/callbacks as props. 06-07-PLAN.md's SurfaceService returns
// camelCase JSON (internal/wails.Result convention), matched by the
// TypeScript shapes below field-for-field.
//
// This Wave 3 plan replaces this file's contents; App.tsx's mount point for
// <OperatorSurface /> is never changed.

import { useCallback, useEffect, useState } from "react";
import { Eye, ArrowLeft } from "lucide-react";

import { useGolcStore } from "../../store/store";
import {
  assertOk,
  requireSurfaceService,
  setPlaybackActiveSurface,
  setSafetyActiveSurface,
  type SurfaceControlRefInput,
  type SurfaceControlRefView,
  type SurfaceSummary,
} from "../../lib/wailsBridge";
import { Button, ErrorState, LoadingState, Panel, ScrollRegion } from "../../design-system";
import AssignmentToggle from "./AssignmentToggle";
import SurfaceList from "./SurfaceList";
import Launcher from "./Launcher";
import styles from "./OperatorSurface.module.css";

// ---------------------------------------------------------------------------
// Types (mirror internal/wails/svc_surface.go's JSON shapes field-for-field)
// ---------------------------------------------------------------------------

// ControlKind/ControlRefInput/ControlRefView/SurfaceSummary/SurfaceDetail
// are re-exported from wailsBridge.ts's canonical declarations rather than
// re-declared here: they mirror internal/wails/svc_surface.go's JSON
// shapes, which that module owns. The re-export preserves this file as the
// import site Launcher.tsx, SurfaceList.tsx and DeskOperatorFixture.tsx
// already use, under the names they already import.
export type {
  SurfaceControlKind as ControlKind,
  SurfaceControlRefInput as ControlRefInput,
  SurfaceControlRefView as ControlRefView,
  SurfaceSummary,
  SurfaceDetail,
} from "../../lib/wailsBridge";

// requireSurfaceService is wailsBridge.ts's throwing accessor -- this
// component is the one caller that treats an absent binding as a
// programming error rather than a degraded state, and that behaviour is
// preserved verbatim. The binding's return types are now declared
// precisely at the source (the bridge owns the wire shapes), so the
// `as unknown as <local shape>` narrowing cast this file used to need is
// gone entirely.
const surfaceService = requireSurfaceService;

// selector strips SurfaceControlRefView's extra label/assigned fields before
// sending a control reference back to a binding that only accepts the bare
// SurfaceControlRefInput selector shape.
function selector(controlRef: SurfaceControlRefInput): SurfaceControlRefInput {
  const { kind, scene, layerKind, masterKind, group, safety } = controlRef;
  return { kind, scene, layerKind, masterKind, group, safety };
}

function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}

function controlKey(control: SurfaceControlRefView): string {
  switch (control.kind) {
    case "scene":
      return `scene:${control.scene ?? ""}`;
    case "layer":
      return `layer:${control.scene ?? ""}:${control.layerKind ?? ""}`;
    case "master":
      return control.masterKind === "grand" ? "master:grand" : `master:group:${control.group ?? ""}`;
    case "safety":
      return `safety:${control.safety ?? ""}`;
    default:
      return control.label;
  }
}

type ViewMode = "author" | "operate";

export default function OperatorSurface() {
  const connectionStatus = useGolcStore((state) => state.connectionStatus);
  const daemonLoading = connectionStatus === "connecting";
  const bumpSurfaceListVersion = useGolcStore((state) => state.bumpSurfaceListVersion);

  const [surfaces, setSurfaces] = useState<SurfaceSummary[]>([]);
  const [selectedName, setSelectedName] = useState<string | null>(null);
  const [controls, setControls] = useState<SurfaceControlRefView[]>([]);
  const [mode, setMode] = useState<ViewMode>("author");
  const [listLoading, setListLoading] = useState(true);
  const [detailLoading, setDetailLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const refreshSurfaces = useCallback(async (): Promise<SurfaceSummary[]> => {
    try {
      const result = await surfaceService().ListSurfaces();
      setSurfaces(result);
      setError(null);
      return result;
    } catch (err) {
      setError(errorMessage(err));
      return [];
    } finally {
      setListLoading(false);
    }
  }, []);

  const refreshDetail = useCallback(async (name: string): Promise<void> => {
    setDetailLoading(true);
    try {
      const detail = await surfaceService().ShowSurface(name);
      setControls(detail.controls);
      setError(null);
    } catch (err) {
      setError(errorMessage(err));
      setControls([]);
    } finally {
      setDetailLoading(false);
    }
  }, []);

  useEffect(() => {
    void refreshSurfaces();
  }, [refreshSurfaces]);

  useEffect(() => {
    if (selectedName) {
      void refreshDetail(selectedName);
    } else {
      setControls([]);
    }
  }, [selectedName, refreshDetail]);

  // CR-01 fix: while previewing this surface in "operate" mode, scope the
  // real SafetyService/PlaybackService dispatch paths to it server-side
  // (SetActiveSurface) so the D-04 lock this component's own rendering
  // implies is actually enforced, not just displayed. Any change that ends
  // the preview -- switching modes, switching/deselecting the surface, or
  // unmounting entirely -- clears both back to unrestricted/author-mode
  // dispatch via this same effect's cleanup.
  useEffect(() => {
    if (mode !== "operate" || !selectedName) {
      return;
    }
    void setSafetyActiveSurface(selectedName);
    void setPlaybackActiveSurface(selectedName);
    return () => {
      void setSafetyActiveSurface("");
      void setPlaybackActiveSurface("");
    };
  }, [mode, selectedName]);

  const handleSelect = (name: string) => {
    setSelectedName(name);
  };

  const handleCreate = async (name: string) => {
    try {
      const result = await surfaceService().CreateSurface(name);
      assertOk(result, "CreateSurface");
      await refreshSurfaces();
      bumpSurfaceListVersion();
      setSelectedName(name);
    } catch (err) {
      setError(errorMessage(err));
    }
  };

  const handleRemove = async (name: string) => {
    try {
      const result = await surfaceService().RemoveSurface(name);
      assertOk(result, "RemoveSurface");
      if (selectedName === name) {
        setSelectedName(null);
      }
      await refreshSurfaces();
      bumpSurfaceListVersion();
    } catch (err) {
      setError(errorMessage(err));
    }
  };

  const handleToggle = async (control: SurfaceControlRefView) => {
    if (!selectedName) {
      return;
    }
    try {
      const result = control.assigned
        ? await surfaceService().UnassignItem(selectedName, selector(control))
        : await surfaceService().AssignItem(selectedName, selector(control));
      assertOk(result, control.assigned ? "UnassignItem" : "AssignItem");
      await refreshDetail(selectedName);
      await refreshSurfaces();
    } catch (err) {
      setError(errorMessage(err));
    }
  };

  const loading = daemonLoading || listLoading;

  return (
    <Panel className={styles.surfacePanel} aria-label="Operator surfaces" aria-busy={loading}>
      {loading ? (
        <LoadingState label="Loading operator surfaces…" variant="panel" />
      ) : (
        <>
          <SurfaceList
            surfaces={surfaces}
            selectedName={selectedName}
            onSelect={handleSelect}
            onCreate={handleCreate}
            onRemove={handleRemove}
          />

          {error && <ErrorState heading="Operator surfaces unavailable" message={error} />}

          {selectedName && (
            <div className={styles.detailPanel}>
              <div className={styles.detailHeader}>
                <h3 className={styles.detailTitle} title={selectedName}>
                  {selectedName}
                </h3>
                <Button
                  variant="secondary"
                  leadingIcon={mode === "author" ? Eye : ArrowLeft}
                  onClick={() => setMode((current) => (current === "author" ? "operate" : "author"))}
                >
                  {mode === "author" ? "Preview as Operator" : "Back to Authoring"}
                </Button>
              </div>

              {detailLoading ? (
                <LoadingState label="Loading assignments…" variant="panel" />
              ) : mode === "author" ? (
                <ScrollRegion aria-label={`${selectedName} controls`}>
                  <ul className={styles.controlList}>
                    {controls.map((control) => (
                      <li key={controlKey(control)} className={styles.controlRow}>
                        <AssignmentToggle
                          label={control.label}
                          assigned={control.assigned}
                          onToggle={() => handleToggle(control)}
                        />
                      </li>
                    ))}
                  </ul>
                </ScrollRegion>
              ) : (
                <Launcher controls={controls} />
              )}
            </div>
          )}
        </>
      )}
    </Panel>
  );
}
