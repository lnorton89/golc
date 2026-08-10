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
// All Go-bound calls go through wailsBridge.ts's getMidiService and
// getSurfaceService accessors (that module owns every read of Wails v2's
// runtime-injected bridge);
// this file owns every such call in the component tree -- MidiLearn.tsx
// and SoftTakeoverSlider.tsx are purely presentational, receiving
// data/callbacks as props (mirrors OperatorSurface.tsx's own
// composition). SetActiveSurface is called whenever the selected surface
// changes so the Go host's live dispatch loop (svc_midi.go's
// dispatchLoop) arbitrates incoming MIDI against the surface currently
// being viewed.
//
// Phase 13 Plan 28 migrated this component onto shared design-system
// primitives (Panel/PanelHeader/Field/ListRow/EmptyState/ErrorState/
// LoadingState/Chip/IconButton) -- every Wails call, state transition, and
// dispatch path above is unchanged.
//
// A later revision replaced the operator-surface picker's hand-rolled
// unstyled <select> with the shared Select primitive (Base UI-backed) --
// a small, fully-known, fixed list of surfaces is exactly Select's
// intended fit, not Combobox's (no typing-to-filter value here).

import { useCallback, useEffect, useState } from "react";
import { Music2, Trash2 } from "lucide-react";
import { AnimatePresence, motion } from "motion/react";

import { useGolcStore } from "../../store/store";
import { useLatestRequest } from "../../hooks/useLatestRequest";
import {
  errorMessage,
  getMidiService,
  getSurfaceService,
  onMidiFeedback,
  type MidiFeedback,
  type MidiMappingView,
  type MidiMessageKind,
  type SurfaceControlRefInput,
  type SurfaceControlRefView,
  type SurfaceSummary,
} from "../../lib/wailsBridge";
import { motionTransition } from "../../design-system/motion";
import { Chip, EmptyState, ErrorState, IconButton, ListRow, LoadingState, Panel, PanelHeader, Select } from "../../design-system";
import MidiLearn from "./MidiLearn";
import SoftTakeoverSlider from "./SoftTakeoverSlider";
import DeskMappingsSection from "./DeskMappingsSection";
import styles from "./MidiPanel.module.css";

const rowExitTransition = motionTransition("settle");

// ---------------------------------------------------------------------------
// Types (mirror internal/wails/svc_surface.go's and svc_midi.go's JSON
// shapes field-for-field)
// ---------------------------------------------------------------------------

// ControlKind/ControlRefInput/MidiMessageKind/MidiMappingView are
// re-exported from wailsBridge.ts's canonical declarations rather than
// re-declared here: they mirror the Go host's wire shapes, which that
// module owns. The re-export keeps this file the import site MidiLearn.tsx
// and SoftTakeoverSlider.tsx already use.
export type {
  SurfaceControlKind as ControlKind,
  SurfaceControlRefInput as ControlRefInput,
  MidiMessageKind,
  MidiMappingView,
} from "../../lib/wailsBridge";

// Both bindings come from wailsBridge.ts, the one module that reads
// window.go. errorMessage is imported from there too rather than
// re-declared -- this file previously carried a fourth identical copy of
// it (the bridge's own doc comment already flags that duplication).
const surfaceService = getSurfaceService;
const midiService = getMidiService;

// selector strips SurfaceControlRefView's extra label/assigned fields before
// sending a control reference to a binding that only accepts the bare
// ControlRefInput selector shape (mirrors OperatorSurface.tsx's identical
// helper).
function selector(control: SurfaceControlRefInput): SurfaceControlRefInput {
  const { kind, scene, layerKind, masterKind, group, safety } = control;
  return { kind, scene, layerKind, masterKind, group, safety };
}

function controlKey(control: SurfaceControlRefInput): string {
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

// mappingTechnical's parameter is the narrow shape both MidiMappingView and
// DeskMappingsSection.tsx's own DeskMidiMappingView satisfy structurally --
// there is only one Note/CC/channel formatting implementation, reused by
// both the surface mapping list and the Desk mapping list below.
function mappingTechnical(mapping: { kind: MidiMessageKind; number: number; channel: number }): string {
  const kindLabel = mapping.kind === "note" ? "Note" : "CC";
  return `${kindLabel} ${mapping.number} · ch ${mapping.channel}`;
}

export default function MidiPanel() {
  const connectionStatus = useGolcStore((state) => state.connectionStatus);
  const daemonLoading = connectionStatus === "connecting";
  const surfaceListVersion = useGolcStore((state) => state.surfaceListVersion);

  const [surfaces, setSurfaces] = useState<SurfaceSummary[]>([]);
  const [selectedSurface, setSelectedSurface] = useState<string | null>(null);
  const [assignedControls, setAssignedControls] = useState<SurfaceControlRefView[]>([]);
  const [mappings, setMappings] = useState<MidiMappingView[]>([]);
  const [feedbackByMappingId, setFeedbackByMappingId] = useState<
    Record<string, MidiFeedback>
  >({});
  const [listLoading, setListLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const beginLatestDetail = useLatestRequest();

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

  // Generation-guarded for the same reason OperatorSurface's own detail
  // read is: selecting surface A then surface B inside A's round trip
  // used to leave B's header over A's assigned controls and mappings when
  // A resolved last, and the follow-on StartLearn(B, controlRefFromA)
  // then came back as an authorization error on a control the user could
  // see listed. See useLatestRequest.
  const refreshSurfaceDetail = useCallback(
    async (name: string): Promise<void> => {
      const surfSvc = surfaceService();
      const midiSvc = midiService();
      if (!surfSvc || !midiSvc) {
        return;
      }
      const isCurrent = beginLatestDetail();
      try {
        const [detail, mappingRows] = await Promise.all([
          surfSvc.ShowSurface(name),
          midiSvc.ListMappings(name),
        ]);
        if (!isCurrent()) {
          return;
        }
        setAssignedControls(detail.controls.filter((control) => control.assigned));
        setMappings(mappingRows);
        setError(null);
      } catch (err) {
        if (!isCurrent()) {
          return;
        }
        setError(errorMessage(err));
        setAssignedControls([]);
        setMappings([]);
      }
    },
    [beginLatestDetail],
  );

  useEffect(() => {
    void refreshSurfaces();
    // surfaceListVersion is OperatorSurface.tsx's create/remove invalidation
    // signal (store.ts) -- App.tsx mounts both components permanently side
    // by side, so this list must re-fetch whenever the other one changes it,
    // not just once on mount.
  }, [refreshSurfaces, surfaceListVersion]);

  // Re-fetching the list is not enough on its own: deleting the selected
  // surface from the Operator Surfaces view (mounted side by side with
  // this one) left `selectedSurface` pointing at a name with no matching
  // option, so the Select rendered a dangling value and
  // refreshSurfaceDetail kept failing with GOLC_OPERATORSURFACE_NOT_FOUND
  // under a still-mounted controls/mappings section. Deselect instead, so
  // the panel collapses back to its own empty state.
  useEffect(() => {
    if (listLoading || !selectedSurface) {
      return;
    }
    if (!surfaces.some((surface) => surface.name === selectedSurface)) {
      setSelectedSurface(null);
    }
  }, [surfaces, selectedSurface, listLoading]);

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
    <Panel aria-label="MIDI mappings" aria-busy={loading}>
      {loading ? (
        <LoadingState label="Loading MIDI mappings" variant="panel" />
      ) : (
        <>
          <Select
            label="Operator surface"
            options={surfaces.map((surface) => ({ value: surface.name, label: surface.name }))}
            value={selectedSurface ?? undefined}
            placeholder="Select a surface…"
            onValueChange={(next) => setSelectedSurface(next || null)}
          />

          {error && <ErrorState heading="MIDI mappings unavailable" message={error} variant="inline" />}

          <DeskMappingsSection />

          {selectedSurface && (
            <>
              <div className={styles.assignedSection}>
                <PanelHeader
                  label="Assigned controls"
                  info="Lists the show controls assigned to the selected operator surface that can be mapped to a physical MIDI control."
                />
                {assignedControls.length === 0 ? (
                  <EmptyState
                    heading="No controls assigned"
                    body="No controls are assigned to this surface yet — assign one from the Operator Surfaces view first."
                  />
                ) : (
                  <ul
                    className={styles.controlList}
                    aria-label={`${selectedSurface} learnable controls`}
                  >
                    {assignedControls.map((control) => (
                      <li key={controlKey(control)}>
                        <ListRow
                          label={control.label}
                          actions={
                            <MidiLearn
                              surfaceName={selectedSurface}
                              controlRef={selector(control)}
                              controlLabel={control.label}
                              onLearned={handleLearned}
                            />
                          }
                        />
                      </li>
                    ))}
                  </ul>
                )}
              </div>

              <div className={styles.mappingSection}>
                <PanelHeader
                  label="MIDI mappings"
                  info="Lists every control currently mapped to an incoming MIDI message, and its live soft-takeover state."
                />
                {mappings.length === 0 ? (
                  <EmptyState
                    icon={Music2}
                    heading="No MIDI mappings yet"
                    body="Click Learn on any assigned control, then move or press the matching hardware control."
                  />
                ) : (
                  <ul
                    className={styles.mappingList}
                    aria-label={`${selectedSurface} MIDI mappings`}
                  >
                    <AnimatePresence initial={false}>
                      {mappings.map((mapping) => {
                        const feedback = feedbackByMappingId[mapping.id];
                        return (
                          <motion.li
                            key={mapping.id}
                            style={{ overflow: "hidden" }}
                            initial={false}
                            exit={{ opacity: 0, height: 0 }}
                            transition={rowExitTransition}
                          >
                            <ListRow
                              label={mapping.label}
                              meta={mappingTechnical(mapping)}
                              actions={
                                <>
                                  {mapping.kind === "control_change" ? (
                                    <SoftTakeoverSlider feedback={feedback} />
                                  ) : (
                                    <Chip tone={feedback?.armed ? "armed" : "neutral"}>
                                      {feedback?.armed ? "Armed" : "Not armed"}
                                    </Chip>
                                  )}
                                  <IconButton
                                    icon={Trash2}
                                    label={`Remove mapping from ${mapping.label}`}
                                    variant="destructive"
                                    size="compact"
                                    onClick={() => handleRemove(mapping)}
                                  />
                                </>
                              }
                            />
                          </motion.li>
                        );
                      })}
                    </AnimatePresence>
                  </ul>
                )}
              </div>
            </>
          )}
        </>
      )}
    </Panel>
  );
}
