// MidiLearn.tsx is the per-control Learn affordance (D-05): a small
// "Learn" button rendered next to each control currently assigned to the
// active operator surface (D-08 -- MidiPanel.tsx only ever renders this
// component for an already-assigned control, never a separate fixed
// "MIDI-mappable" list). Clicking Learn calls the bound
// MidiService.StartLearn and switches to the "Listening for MIDI input…"
// loading state with a Cancel affordance (06-UI-SPEC.md Copywriting
// Contract) while the call is in flight; on success it calls onLearned so
// MidiPanel.tsx can refresh the mapping list. StartLearn's own Stderr
// diagnostic drives which error copy renders: GOLC_MIDI_MAPPING_CONFLICT
// carries the exact UI-SPEC mapping-conflict sentence embedded by
// internal/wails/svc_midi.go (this component strips the diagnostic
// prefix and renders the remainder verbatim); GOLC_MIDI_LEARN_TIMEOUT
// (minted by internal/midi/learn.go's CaptureCandidate, 06-03) maps to
// the UI-SPEC timeout copy client-side, since that diagnostic's own
// message text isn't phrased as user-facing copy.

import { useState } from "react";

import styles from "./MidiPanel.module.css";
import type { ControlRefInput } from "./MidiPanel";

interface GoResult {
  exitCode: number;
  stdout: string;
  stderr: string;
}

interface MidiServiceBinding {
  StartLearn(surfaceName: string, controlRef: ControlRefInput): Promise<GoResult>;
  CancelLearn(): Promise<GoResult>;
}

// The `Window.go.wails` global shape itself is declared once, centrally,
// in src/lib/wailsBridge.ts -- cast through that shared shape locally
// here, mirroring OperatorSurface.tsx's own surfaceService() pattern,
// rather than re-declaring `declare global` (which would collide under
// TypeScript's declaration-merging rules for the same inline-typed `go`
// property).
function midiService(): MidiServiceBinding | undefined {
  const service = window.go?.wails?.MidiService;
  return service as unknown as MidiServiceBinding | undefined;
}

type LearnStatus = "idle" | "listening" | "conflict" | "timeout" | "error";

const CONFLICT_PREFIX = "GOLC_MIDI_MAPPING_CONFLICT:";
const TIMEOUT_MARKER = "GOLC_MIDI_LEARN_TIMEOUT";
// 06-UI-SPEC.md Copywriting Contract -- Error state, MIDI Learn timeout.
const TIMEOUT_COPY = "No MIDI input received. Try again.";
const BRIDGE_UNAVAILABLE_COPY =
  "GOLC_WAILS_BRIDGE_UNAVAILABLE: not running inside the GOLC desktop shell";

function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}

interface MidiLearnProps {
  surfaceName: string;
  controlRef: ControlRefInput;
  controlLabel: string;
  onLearned: () => void;
}

export default function MidiLearn({
  surfaceName,
  controlRef,
  controlLabel,
  onLearned,
}: MidiLearnProps) {
  const [status, setStatus] = useState<LearnStatus>("idle");
  const [message, setMessage] = useState<string | null>(null);

  const handleLearn = async () => {
    const svc = midiService();
    if (!svc) {
      setStatus("error");
      setMessage(BRIDGE_UNAVAILABLE_COPY);
      return;
    }
    setStatus("listening");
    setMessage(null);
    try {
      const result = await svc.StartLearn(surfaceName, controlRef);
      if (result.exitCode === 0) {
        setStatus("idle");
        setMessage(null);
        onLearned();
        return;
      }
      if (result.stderr.includes(CONFLICT_PREFIX)) {
        setStatus("conflict");
        setMessage(result.stderr.replace(CONFLICT_PREFIX, "").trim());
        return;
      }
      if (result.stderr.includes(TIMEOUT_MARKER)) {
        setStatus("timeout");
        setMessage(TIMEOUT_COPY);
        return;
      }
      setStatus("error");
      setMessage(result.stderr.trim() || "Learn failed");
    } catch (err) {
      setStatus("error");
      setMessage(errorMessage(err));
    }
  };

  const handleCancel = async () => {
    const svc = midiService();
    if (svc) {
      try {
        await svc.CancelLearn();
      } catch {
        // CancelLearn failing (e.g. the session already finished on its
        // own) is not itself an error worth surfacing -- the button
        // simply returns to idle either way.
      }
    }
    setStatus("idle");
    setMessage(null);
  };

  if (status === "listening") {
    return (
      <div className={styles.learnListening} role="status" aria-live="polite">
        <span>Listening for MIDI input…</span>
        <button type="button" className={styles.cancelButton} onClick={handleCancel}>
          Cancel
        </button>
      </div>
    );
  }

  return (
    <div className={styles.learnControl}>
      <button
        type="button"
        className={styles.learnButton}
        onClick={handleLearn}
        aria-label={`Learn MIDI mapping for ${controlLabel}`}
      >
        Learn
      </button>
      {message && (status === "conflict" || status === "timeout" || status === "error") && (
        <p className={styles.learnError} role="alert">
          {message}
        </p>
      )}
    </div>
  );
}
