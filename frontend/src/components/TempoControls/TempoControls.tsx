// TempoControls is the shell's single, authoritative BPM control (persistent
// GlobalFrame) -- BPM used to render twice in the header (LiveStatusBar's
// read-only field alongside this component's own input+readout), a visibly
// redundant triple-display of the same number. LiveStatusBar's BPM field was
// removed; this is now the only place BPM appears, shown as one clickable
// value that becomes an editable input on click and commits on Enter/blur
// (no separate "Set" button) -- matching WORKFLOW-MAP.md's "BPM + bar/beat"
// persistent-transport contract without duplicating the number itself.
//
// 13-15 design-system migration: the display/Tap controls now compose the
// shared `Button` primitive, and the editing-mode input+nudge-spinner pair
// now composes the shared `NumberStepper` primitive (extended with an
// optional fractional `step` and onBlur/onKeyDown pass-throughs for this
// exact click-to-edit/commit-on-Enter contract) instead of a hand-rolled
// native input+button pair -- the click-to-edit toggle, the exact 0.1 BPM
// nudge amount, and every dispatch handler are unchanged.
import { useCallback, useEffect, useRef, useState } from "react";
import { Gauge, Hand } from "lucide-react";

import { usePlaybackSnapshot } from "../../shell/PlaybackSnapshotContext";
import { dispatch } from "../../lib/playbackDispatch";
import Button from "../primitives/Button/Button";
import NumberStepper from "../primitives/NumberStepper/NumberStepper";
import styles from "./TempoControls.module.css";

// BPM_STEP is the fixed nudge amount for both the input's own `step="0.1"`
// (NumberStepper's own native input) and the custom spinner buttons that
// replace the native, unstyleable up/down arrows -- both must nudge by the
// exact same increment.
const BPM_STEP = 0.1;

export default function TempoControls() {
  const { state, refreshState } = usePlaybackSnapshot();
  const [editing, setEditing] = useState(false);
  const [bpmInput, setBpmInput] = useState("");
  const inputRef = useRef<HTMLInputElement | null>(null);

  const bpm = state?.bpm ?? 0;

  useEffect(() => {
    if (editing) {
      inputRef.current?.focus();
      inputRef.current?.select();
    }
  }, [editing]);

  const commit = useCallback(async () => {
    const parsed = Number(bpmInput);
    if (Number.isFinite(parsed) && parsed > 0) {
      await dispatch.setBPM(parsed);
      await refreshState();
    }
    setEditing(false);
  }, [bpmInput, refreshState]);

  const startEditing = useCallback(() => {
    setBpmInput(String(bpm));
    setEditing(true);
  }, [bpm]);

  const handleTap = useCallback(async () => {
    await dispatch.recordTap();
    await refreshState();
  }, [refreshState]);

  return (
    <div className={styles.tempo} aria-label="Tempo controls">
      {editing ? (
        <NumberStepper
          ref={inputRef}
          label="BPM"
          value={bpmInput}
          onChange={setBpmInput}
          min={1}
          step={BPM_STEP}
          onBlur={() => void commit()}
          onKeyDown={(event) => {
            if (event.key === "Enter") {
              event.preventDefault();
              void commit();
            } else if (event.key === "Escape") {
              event.preventDefault();
              setEditing(false);
            }
          }}
        />
      ) : (
        <Button type="button" variant="secondary" size="compact" leadingIcon={Gauge} onClick={startEditing}>
          {bpm} BPM
        </Button>
      )}
      <Button type="button" variant="secondary" size="compact" leadingIcon={Hand} aria-label="Tap" onClick={() => void handleTap()}>
        <span className={styles.tapLabel}>Tap</span>
      </Button>
    </div>
  );
}
