// TempoControls is the shell's single, authoritative BPM control (persistent
// GlobalFrame) -- BPM used to render twice in the header (LiveStatusBar's
// read-only field alongside this component's own input+readout), a visibly
// redundant triple-display of the same number. LiveStatusBar's BPM field was
// removed; this is now the only place BPM appears, shown as one clickable
// value that becomes an editable input on click and commits on Enter/blur
// (no separate "Set" button) -- matching WORKFLOW-MAP.md's "BPM + bar/beat"
// persistent-transport contract without duplicating the number itself.
import { useCallback, useEffect, useRef, useState } from "react";
import { Gauge, Hand, ChevronUp, ChevronDown } from "lucide-react";

import { usePlaybackSnapshot } from "../../shell/PlaybackSnapshotContext";
import { dispatch } from "../../lib/playbackDispatch";
import styles from "./TempoControls.module.css";

// BPM_STEP mirrors the input's own `step="0.1"` -- the custom spinner
// buttons below (which replace the native, unstyleable up/down arrows on
// a `type="number"` input) must nudge by the exact same increment.
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

  // step nudges the uncommitted bpmInput by +/-BPM_STEP, clamped to the
  // input's own min={1} -- mirrors exactly what the native spinner arrows
  // this replaces already did (change the value, never auto-commit; only
  // Enter/blur calls commit()).
  const step = useCallback((delta: number) => {
    setBpmInput((current) => {
      const parsed = Number(current) || 0;
      const next = Math.max(1, Math.round((parsed + delta) * 10) / 10);
      return String(next);
    });
  }, []);

  const handleTap = useCallback(async () => {
    await dispatch.recordTap();
    await refreshState();
  }, [refreshState]);

  return (
    <div className={styles.tempo} aria-label="Tempo controls">
      {editing ? (
        <span className={styles.bpmInputWrap}>
          <input
            ref={inputRef}
            className={styles.bpmInput}
            type="number"
            min={1}
            step={BPM_STEP}
            aria-label="BPM"
            value={bpmInput}
            onChange={(event) => setBpmInput(event.target.value)}
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
          <span className={styles.bpmSpinner}>
            {/* tabIndex={-1} + onMouseDown preventDefault: a spinner click
                must nudge the value without ever stealing focus from the
                input -- stealing focus would blur it and fire commit()
                (ending edit mode) on every single click. */}
            <button
              type="button"
              className={styles.bpmSpinnerButton}
              tabIndex={-1}
              aria-label="Increase BPM"
              onMouseDown={(event) => event.preventDefault()}
              onClick={() => step(BPM_STEP)}
            >
              <ChevronUp size={10} aria-hidden="true" />
            </button>
            <button
              type="button"
              className={styles.bpmSpinnerButton}
              tabIndex={-1}
              aria-label="Decrease BPM"
              onMouseDown={(event) => event.preventDefault()}
              onClick={() => step(-BPM_STEP)}
            >
              <ChevronDown size={10} aria-hidden="true" />
            </button>
          </span>
        </span>
      ) : (
        <button type="button" className={styles.bpmDisplay} onClick={startEditing}>
          <Gauge size={13} className={styles.bpmIcon} aria-hidden="true" />
          {bpm} BPM
        </button>
      )}
      <button type="button" className={styles.button} onClick={() => void handleTap()}>
        <Hand size={13} aria-hidden="true" />
        Tap
      </button>
    </div>
  );
}
