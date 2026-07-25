// TempoControls is the shell's single, authoritative BPM control (persistent
// GlobalFrame) -- BPM used to render twice in the header (LiveStatusBar's
// read-only field alongside this component's own input+readout), a visibly
// redundant triple-display of the same number. LiveStatusBar's BPM field was
// removed; this is now the only place BPM appears, shown as one clickable
// value that becomes an editable input on click and commits on Enter/blur
// (no separate "Set" button) -- matching WORKFLOW-MAP.md's "BPM + bar/beat"
// persistent-transport contract without duplicating the number itself.
import { useCallback, useEffect, useRef, useState } from "react";

import { usePlaybackSnapshot } from "../../shell/PlaybackSnapshotContext";
import { dispatch } from "../../lib/playbackDispatch";
import styles from "./TempoControls.module.css";

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
        <input
          ref={inputRef}
          className={styles.bpmInput}
          type="number"
          min={1}
          step="0.1"
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
      ) : (
        <button type="button" className={styles.bpmDisplay} onClick={startEditing}>
          {bpm} BPM
        </button>
      )}
      <button type="button" className={styles.button} onClick={() => void handleTap()}>
        Tap
      </button>
    </div>
  );
}
