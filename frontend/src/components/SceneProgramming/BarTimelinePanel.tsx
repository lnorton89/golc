// BarTimelinePanel is the Scene Stack's bottom evaluation panel
// (programming-scene-authoring.md: "a lower timeline/evaluation panel
// shows bar-relative layer behavior and the evaluated position").
// Absorbs PlaybackControls.tsx's old Transport/Evaluate control (shell
// restructure plan Step 6) -- "Evaluation at a bar position previews
// through shared Go commands" is documented as a *programming*-workspace
// concern, not a performance one.
import { useState } from "react";

import Button from "../primitives/Button/Button";
import { dispatch } from "../../lib/playbackDispatch";
import styles from "./BarTimelinePanel.module.css";

interface BarTimelinePanelProps {
  activeSceneName: string | null;
}

export default function BarTimelinePanel({ activeSceneName }: BarTimelinePanelProps) {
  const [evaluateAt, setEvaluateAt] = useState("0");
  const [previewOutput, setPreviewOutput] = useState("");

  const handleEvaluate = async () => {
    const parsed = Number(evaluateAt);
    if (!Number.isFinite(parsed)) {
      return;
    }
    const result = await dispatch.evaluate(parsed);
    setPreviewOutput(result?.stdout || result?.stderr || "");
  };

  return (
    <div className={styles.panel} aria-label="Bar timeline preview">
      <span className={styles.label}>
        Evaluate{activeSceneName ? ` — ${activeSceneName}` : ""}
      </span>
      <div className={styles.row}>
        <input
          className={styles.input}
          type="number"
          step="0.01"
          aria-label="Evaluate position (bar.beatfraction)"
          value={evaluateAt}
          onChange={(event) => setEvaluateAt(event.target.value)}
        />
        <Button variant="primary" onClick={() => void handleEvaluate()}>
          Evaluate
        </Button>
      </div>
      {previewOutput ? <pre className={styles.output}>{previewOutput}</pre> : null}
    </div>
  );
}
