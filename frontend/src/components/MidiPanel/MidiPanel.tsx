// MidiPanel.tsx is the feature region for generic MIDI Note/CC learn and
// soft-takeover feedback (06-08-PLAN.md Task 3, PLAY-04/05 D-05..D-12).
// It composes a surface selector (mappings are per-surface, D-07),
// MidiLearn.tsx's per-control Learn affordance for every control
// currently assigned to the selected surface (D-08 -- the assignment set
// itself, read via the existing SurfaceService.ShowSurface, never a
// separate MIDI-mappable list), the MIDI mapping list (06-UI-SPEC.md
// empty/populated states: control name, Note/CC/channel, Remove
// affordance), and SoftTakeoverSlider.tsx for every continuous CC/fader
// mapping (D-09/D-10/D-11) -- Note/button mappings render only the armed
// chip (D-12: no takeover slider).
//
// All Go-bound calls go through window.go.wails.MidiService and
// window.go.wails.SurfaceService (Wails v2's runtime-injected bridge);
// this file owns every such call in the component tree -- MidiLearn.tsx
// and SoftTakeoverSlider.tsx are purely presentational, receiving
// data/callbacks as props (mirrors OperatorSurface.tsx's own
// composition). SetActiveSurface is called whenever the selected surface
// changes so the Go host's live dispatch loop (svc_midi.go's
// dispatchLoop) arbitrates incoming MIDI against the surface currently
// being viewed.
//
// This Wave 4 plan replaces this file's contents; App.tsx's mount point
// for <MidiPanel /> is never changed.

import { useCallback, useEffect, useState } from "react";

import { useGolcStore } from "../../store/store";
import { onMidiFeedback, type MidiFeedback } from "../../lib/wailsBridge";
import MidiLearn from "./MidiLearn";
import SoftTakeoverSlider from "./SoftTakeoverSlider";
import styles from "./MidiPanel.module.css";

// ---------------------------------------------------------------------------
// Types (mirror internal/wails/svc_surface.go's and svc_midi.go's JSON
// shapes field-for-field)
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

interface ControlRefView extends ControlRefInput {
  label: string;
  assigned: boolean;
}

interface SurfaceSummary {
  id: string;
  name: string;
}

interface SurfaceDetail {
  controls: ControlRefView[];
}

export type MidiMessageKind = "note" | "control_change";

export interface MidiMappingView {
  id: string;
  channel: number;
  kind: MidiMessageKind;
  number: number;
  target: ControlRefInput;
  label: string;
}

interface GoResult {
  exitCode: number;
  stdout: string;
  stderr: string;
}

interface SurfaceServiceBinding {
  ListSurfaces(): Promise<SurfaceSummary[]>;
  ShowSurface(surfaceName: string): Promise<SurfaceDetail>;
}

interface MidiServiceBinding {
  RemoveMapping(surfaceName: string, mappingId: string): Promise<GoResult>;
  ListMappings(surfaceName: string): Promise<MidiMappingView[]>;
  SetActiveSurface(surfaceName: string): Promise<GoResult>;
}

// The `Window.go.wails` global shape itself is declared once, centrally,
// in src/lib/wailsBridge.ts -- cast through that shared shape locally,
// mirroring OperatorSurface.tsx's own surfaceService() pattern.
function surfaceService(): SurfaceServiceBinding | undefined {
  return window.go?.wails?.SurfaceService as unknown as SurfaceServiceBinding | undefined;
}

function midiService(): MidiServiceBinding | undefined {
  return window.go?.wails?.MidiService as unknown as MidiServiceBinding | undefined;
}

function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}

// selector strips ControlRefView's extra label/assigned fields before
// sending a control reference to a binding that only accepts the bare
// ControlRefInput selector shape (mirrors OperatorSurface.tsx's identical
// helper).
function selector(control: ControlRefInput): ControlRefInput {
  const { kind, scene, layerKind, masterKind, group, safety } = control;
  return { kind, scene, layerKind, masterKind, group, safety };
}

function controlKey(control: ControlRefInput): string {
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
      return JSON.stringify(control);
  }
}

function mappingTechnical(mapping: MidiMappingView): string {
  const kindLabel = mapping.kind === "note" ? "Note" : "CC";
  return `${kindLabel} ${mapping.number} · ch ${mapping.channel}`;
}

export default function MidiPanel() {
  const connectionStatus = useGolcStore((state) => state.connectionStatus);
  const daemonLoading = connectionStatus === "connecting";
  const surfaceListVersion = useGolcStore((state) => state.surfaceListVersion);

  const [surfaces, setSurfaces] = useState<SurfaceSummary[]>([]);
  const [selectedSurface, setSelectedSurface] = useState<string | null>(null);
  const [assignedControls, setAssignedControls] = useState<ControlRefView[]>([]);
  const [mappings, setMappings] = useState<MidiMappingView[]>([]);
  const [feedbackByMappingId, setFeedbackByMappingId] = useState<
    Record<string, MidiFeedback>
  >({});
  const [listLoading, setListLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refreshSurfaces = useCallback(async (): Promise<void> => {
    const svc = surfaceService();
    if (!svc) {
      setListLoading(false);
      return;
    }
    try {
      const result = await svc.ListSurfaces();
      setSurfaces(result);
      setError(null);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setListLoading(false);
    }
  }, []);

  const refreshSurfaceDetail = useCallback(async (name: string): Promise<void> => {
    const surfSvc = surfaceService();
    const midiSvc = midiService();
    if (!surfSvc || !midiSvc) {
      return;
    }
    try {
      const [detail, mappingRows] = await Promise.all([
        surfSvc.ShowSurface(name),
        midiSvc.ListMappings(name),
      ]);
      setAssignedControls(detail.controls.filter((control) => control.assigned));
      setMappings(mappingRows);
      setError(null);
    } catch (err) {
      setError(errorMessage(err));
      setAssignedControls([]);
      setMappings([]);
    }
  }, []);

  useEffect(() => {
    void refreshSurfaces();
    // surfaceListVersion is OperatorSurface.tsx's create/remove invalidation
    // signal (store.ts) -- App.tsx mounts both components permanently side
    // by side, so this list must re-fetch whenever the other one changes it,
    // not just once on mount.
  }, [refreshSurfaces, surfaceListVersion]);

  useEffect(() => {
    if (!selectedSurface) {
      setAssignedControls([]);
      setMappings([]);
      return;
    }
    void midiService()?.SetActiveSurface(selectedSurface);
    void refreshSurfaceDetail(selectedSurface);
  }, [selectedSurface, refreshSurfaceDetail]);

  useEffect(() => {
    return onMidiFeedback((feedback) => {
      if (feedback.surfaceName !== selectedSurface) {
        return;
      }
      setFeedbackByMappingId((prev) => ({ ...prev, [feedback.mappingId]: feedback }));
    });
  }, [selectedSurface]);

  const handleLearned = () => {
    if (selectedSurface) {
      void refreshSurfaceDetail(selectedSurface);
    }
  };

  const handleRemove = async (mapping: MidiMappingView) => {
    const svc = midiService();
    if (!svc || !selectedSurface) {
      return;
    }
    // 06-UI-SPEC.md Destructive confirmation -- Remove MIDI Mapping.
    const confirmed = window.confirm(
      `Remove Mapping: This unassigns ${mappingTechnical(mapping)} from ${mapping.label} on ${selectedSurface}.`,
    );
    if (!confirmed) {
      return;
    }
    try {
      const result = await svc.RemoveMapping(selectedSurface, mapping.id);
      if (result.exitCode !== 0) {
        throw new Error(result.stderr || "RemoveMapping failed");
      }
      await refreshSurfaceDetail(selectedSurface);
    } catch (err) {
      setError(errorMessage(err));
    }
  };

  const loading = daemonLoading || listLoading;

  return (
    <section className={styles.panel} aria-label="MIDI mappings" aria-busy={loading}>
      {loading ? (
        <div className={styles.skeleton}>Loading MIDI mappings…</div>
      ) : (
        <>
          <label className={styles.surfaceSelectRow}>
            <span className={styles.surfaceSelectLabel}>Operator surface</span>
            <select
              className={styles.surfaceSelect}
              value={selectedSurface ?? ""}
              onChange={(event) => setSelectedSurface(event.target.value || null)}
              aria-label="Select operator surface for MIDI mappings"
            >
              <option value="">Select a surface…</option>
              {surfaces.map((surface) => (
                <option key={surface.id} value={surface.name}>
                  {surface.name}
                </option>
              ))}
            </select>
          </label>

          {error && <p className={styles.errorText}>{error}</p>}

          {selectedSurface && (
            <>
              <div>
                <h3 className={styles.sectionHeading}>Assigned controls</h3>
                {assignedControls.length === 0 ? (
                  <p className={styles.emptyBody}>
                    No controls are assigned to this surface yet — assign one from the
                    Operator Surfaces view first.
                  </p>
                ) : (
                  <ul
                    className={styles.controlList}
                    aria-label={`${selectedSurface} learnable controls`}
                  >
                    {assignedControls.map((control) => (
                      <li key={controlKey(control)} className={styles.controlRow}>
                        <span className={styles.controlLabel}>{control.label}</span>
                        <MidiLearn
                          surfaceName={selectedSurface}
                          controlRef={selector(control)}
                          controlLabel={control.label}
                          onLearned={handleLearned}
                        />
                      </li>
                    ))}
                  </ul>
                )}
              </div>

              <div className={styles.mappingSection}>
                <h3 className={styles.sectionHeading}>MIDI mappings</h3>
                {mappings.length === 0 ? (
                  <div className={styles.emptyState}>
                    <p className={styles.emptyHeading}>No MIDI mappings yet</p>
                    <p className={styles.emptyBody}>
                      Click Learn on any assigned control, then move or press the
                      matching hardware control.
                    </p>
                  </div>
                ) : (
                  <ul
                    className={styles.mappingList}
                    aria-label={`${selectedSurface} MIDI mappings`}
                  >
                    {mappings.map((mapping) => {
                      const feedback = feedbackByMappingId[mapping.id];
                      return (
                        <li key={mapping.id} className={styles.mappingRow}>
                          <div className={styles.mappingInfo}>
                            <span className={styles.mappingLabel} title={mapping.label}>
                              {mapping.label}
                            </span>
                            <span className={styles.mappingTechnical}>
                              {mappingTechnical(mapping)}
                            </span>
                          </div>

                          {mapping.kind === "control_change" ? (
                            <SoftTakeoverSlider feedback={feedback} />
                          ) : (
                            <span
                              className={`${styles.armedChip} ${
                                feedback?.armed ? styles.armedChipOn : styles.armedChipOff
                              }`}
                            >
                              {feedback?.armed ? "Armed" : "Not armed"}
                            </span>
                          )}

                          <button
                            type="button"
                            className={styles.removeButton}
                            onClick={() => handleRemove(mapping)}
                            aria-label={`Remove mapping from ${mapping.label}`}
                          >
                            Remove
                          </button>
                        </li>
                      );
                    })}
                  </ul>
                )}
              </div>
            </>
          )}
        </>
      )}
    </section>
  );
}
