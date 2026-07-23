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
// enforcement.
//
// All Go-bound calls go through window.go.wails.SurfaceService (Wails v2's
// runtime-injected bridge for the internal/wails.SurfaceService struct);
// this file owns every SurfaceService call in the component tree --
// SurfaceList.tsx and AssignmentToggle.tsx are purely presentational and
// receive data/callbacks as props. 06-07-PLAN.md's SurfaceService returns
// camelCase JSON (internal/wails.Result convention), matched by the
// TypeScript shapes below field-for-field.
//
// This Wave 3 plan replaces this file's contents; App.tsx's mount point for
// <OperatorSurface /> is never changed.

import { useCallback, useEffect, useState } from "react";

import { useGolcStore } from "../../store/store";
import AssignmentToggle from "./AssignmentToggle";
import SurfaceList from "./SurfaceList";
import styles from "./OperatorSurface.module.css";

// ---------------------------------------------------------------------------
// Types (mirror internal/wails/svc_surface.go's JSON shapes field-for-field)
// ---------------------------------------------------------------------------

export type ControlKind = "scene" | "layer" | "master" | "safety";

export interface ControlRefInput {
  kind: ControlKind;
  scene?: string;
  layerKind?: string;
  masterKind?: "grand" | "group";
  group?: string;
  safety?: string;
}

export interface ControlRefView extends ControlRefInput {
  label: string;
  assigned: boolean;
}

export interface SurfaceSummary {
  id: string;
  name: string;
  sceneCount: number;
  layerCount: number;
  masterCount: number;
  safetyCount: number;
  assignedCount: number;
  midiMappingCount: number;
}

export interface SurfaceDetail {
  id: string;
  name: string;
  controls: ControlRefView[];
  midiMappingCount: number;
}

interface GoResult {
  exitCode: number;
  stdout: string;
  stderr: string;
}

declare global {
  interface Window {
    go?: {
      wails?: {
        SurfaceService?: {
          CreateSurface(name: string): Promise<GoResult>;
          ListSurfaces(): Promise<SurfaceSummary[]>;
          AssignItem(surfaceName: string, controlRef: ControlRefInput): Promise<GoResult>;
          UnassignItem(surfaceName: string, controlRef: ControlRefInput): Promise<GoResult>;
          ShowSurface(surfaceName: string): Promise<SurfaceDetail>;
          RemoveSurface(surfaceName: string): Promise<GoResult>;
          AuthorizeControl(surfaceName: string, controlRef: ControlRefInput): Promise<GoResult>;
        };
      };
    };
  }
}

function surfaceService() {
  const service = window.go?.wails?.SurfaceService;
  if (!service) {
    throw new Error(
      "GOLC_WAILS_BINDING_UNAVAILABLE: SurfaceService is not available on window.go.wails -- this component must run inside the golc-desktop Wails webview.",
    );
  }
  return service;
}

function assertOk(result: GoResult, action: string): void {
  if (result.exitCode !== 0) {
    throw new Error(result.stderr || `${action} failed (exit ${result.exitCode})`);
  }
}

// selector strips ControlRefView's extra label/assigned fields before
// sending a control reference back to a binding that only accepts the bare
// ControlRefInput selector shape.
function selector(controlRef: ControlRefInput): ControlRefInput {
  const { kind, scene, layerKind, masterKind, group, safety } = controlRef;
  return { kind, scene, layerKind, masterKind, group, safety };
}

function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}

function controlKey(control: ControlRefView): string {
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

  const [surfaces, setSurfaces] = useState<SurfaceSummary[]>([]);
  const [selectedName, setSelectedName] = useState<string | null>(null);
  const [controls, setControls] = useState<ControlRefView[]>([]);
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

  const handleSelect = (name: string) => {
    setSelectedName(name);
  };

  const handleCreate = async (name: string) => {
    try {
      const result = await surfaceService().CreateSurface(name);
      assertOk(result, "CreateSurface");
      await refreshSurfaces();
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
    } catch (err) {
      setError(errorMessage(err));
    }
  };

  const handleToggle = async (control: ControlRefView) => {
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
    <section className={styles.panel} aria-label="Operator surfaces" aria-busy={loading}>
      {loading ? (
        <div className={styles.skeleton}>Loading operator surfaces…</div>
      ) : (
        <>
          <SurfaceList
            surfaces={surfaces}
            selectedName={selectedName}
            onSelect={handleSelect}
            onCreate={handleCreate}
            onRemove={handleRemove}
          />

          {error && <p className={styles.errorText}>{error}</p>}

          {selectedName && (
            <div className={styles.detailPanel}>
              <div className={styles.detailHeader}>
                <h3 className={styles.detailTitle} title={selectedName}>
                  {selectedName}
                </h3>
                <button
                  type="button"
                  className={styles.modeButton}
                  onClick={() => setMode((current) => (current === "author" ? "operate" : "author"))}
                >
                  {mode === "author" ? "Preview as Operator" : "Back to Authoring"}
                </button>
              </div>

              {detailLoading ? (
                <div className={styles.skeleton}>Loading assignments…</div>
              ) : (
                <ul className={styles.controlList} aria-label={`${selectedName} controls`}>
                  {controls.map((control) => {
                    const key = controlKey(control);
                    if (mode === "author") {
                      return (
                        <li key={key} className={styles.controlRow}>
                          <AssignmentToggle
                            label={control.label}
                            assigned={control.assigned}
                            onToggle={() => handleToggle(control)}
                          />
                        </li>
                      );
                    }
                    return (
                      <li
                        key={key}
                        className={`${styles.controlRow} ${
                          control.assigned ? styles.controlAssigned : styles.controlLocked
                        }`}
                        aria-disabled={!control.assigned}
                        title={control.label}
                      >
                        <span className={styles.controlLabel}>{control.label}</span>
                        <span className={styles.controlState}>
                          {control.assigned ? "Available" : "Locked"}
                        </span>
                      </li>
                    );
                  })}
                </ul>
              )}
            </div>
          )}
        </>
      )}
    </section>
  );
}
