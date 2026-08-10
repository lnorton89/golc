// BarTimelinePanel is the Scene Stack's bottom evaluation panel
// (programming-scene-authoring.md: "a lower timeline/evaluation panel
// shows bar-relative layer behavior and the evaluated position").
// Absorbs PlaybackControls.tsx's old Transport/Evaluate control (shell
// restructure plan Step 6) -- "Evaluation at a bar position previews
// through shared Go commands" is documented as a *programming*-workspace
// concern, not a performance one.
import { useState } from "react";
import { Zap } from "lucide-react";

import { Button, ErrorState, Field, Panel } from "../../design-system";
import { useLatestRequest } from "../../hooks/useLatestRequest";
import { dispatch } from "../../lib/playbackDispatch";
import { errorMessage } from "../../lib/wailsBridge";
import styles from "./BarTimelinePanel.module.css";

interface BarTimelinePanelProps {
  activeSceneName: string | null;
}

export default function BarTimelinePanel({ activeSceneName }: BarTimelinePanelProps) {
  const [evaluateAt, setEvaluateAt] = useState("0");
  const [previewOutput, setPreviewOutput] = useState("");
  const [evaluating, setEvaluating] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const beginLatest = useLatestRequest();

  // Latest-wins (see useLatestRequest): evaluating bar 4, then changing
  // the field to 8 and evaluating again, used to show bar 4's output under
  // a field reading 8 whenever the first dispatch resolved last. The
  // button is also disabled while a call is outstanding, and a thrown
  // dispatch now clears the previous output instead of leaving it on
  // screen as if it were current.
  const handleEvaluate = async () => {
    const parsed = Number(evaluateAt);
    if (!Number.isFinite(parsed)) {
      return;
    }
    const isCurrent = beginLatest();
    setEvaluating(true);
    try {
      const result = await dispatch.evaluate(parsed);
      if (!isCurrent()) {
        return;
      }
      setPreviewOutput(result?.stdout || result?.stderr || "");
      setError(null);
    } catch (err) {
      if (!isCurrent()) {
        return;
      }
      setPreviewOutput("");
      setError(errorMessage(err));
    } finally {
      if (isCurrent()) {
        setEvaluating(false);
      }
    }
  };

  return (
    <Panel className={styles.panel} aria-label="Bar timeline preview">
      <span className={styles.label}>
        Evaluate{activeSceneName ? ` — ${activeSceneName}` : ""}
      </span>
      <div className={styles.row}>
        <Field
          label="Evaluate position (bar.beatfraction)"
          type="number"
          step="0.01"
          value={evaluateAt}
          onChange={(event) => setEvaluateAt(event.target.value)}
        />
        <Button variant="primary" icon={Zap} loading={evaluating} disabled={evaluating} onClick={() => void handleEvaluate()}>
          {evaluating ? "Evaluating…" : "Evaluate"}
        </Button>
      </div>
      {error ? <ErrorState heading="Evaluate failed" message={error} /> : null}
      {previewOutput ? <pre className={styles.output}>{previewOutput}</pre> : null}
    </Panel>
  );
}
