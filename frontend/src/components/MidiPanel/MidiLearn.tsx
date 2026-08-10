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
//
// Phase 13 Plan 28 migrated the idle Learn affordance and Cancel action
// onto the shared Button primitive (its own min 44px target-size variant
// already covers the 06-UI-SPEC.md touch-target requirement this file
// used to hand-roll); the "Listening…" pill and its own inline issue
// message remain local, since neither matches an existing shared
// primitive shape closely enough to reuse one.

import { useEffect, useRef, useState } from "react";
import { Radio, X } from "lucide-react";

import { getMidiService } from "../../lib/wailsBridge";
import { useLatestRequest } from "../../hooks/useLatestRequest";
import { Button } from "../../design-system";
import styles from "./MidiPanel.module.css";
import type { ControlRefInput } from "./MidiPanel";

// The MidiService binding and its wire types are declared once, centrally,
// in src/lib/wailsBridge.ts, which owns every window.go read; this file
// imports the typed accessor and uses only the two methods it needs
// (StartLearn/CancelLearn) rather than re-declaring a narrower local copy
// of the binding that could drift from the Go source.
const midiService = getMidiService;

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
  // Each Learn press is its own generation. CancelLearn closes
  // session.cancel on the Go side, which unblocks the still-pending
  // StartLearn so it returns GOLC_MIDI_LEARN_TIMEOUT (svc_midi.go) a
  // moment later -- without this guard that late resolution walked
  // straight into the timeout branch and put "No MIDI input received.
  // Try again." under the button for a learn the operator deliberately
  // aborted. (Desk.tsx already carries the equivalent guard for its own
  // capture flow via capturingKeyRef.)
  const beginLatestLearn = useLatestRequest();
  const learnButtonRef = useRef<HTMLButtonElement | null>(null);
  const cancelButtonRef = useRef<HTMLButtonElement | null>(null);
  // Only move focus for a learn the operator actually started from the
  // keyboard/this control -- never yank it from wherever they went next.
  const ownsFocusRef = useRef(false);

  // Pressing Learn swaps this component's entire subtree: the focused
  // Learn button unmounts and a role="status" region plus a Cancel button
  // takes its place. With no handoff, focus fell to <body>, so a keyboard
  // operator could not reach Cancel without tabbing from the top of the
  // document -- and when the learn resolved, focus was not returned to the
  // remounted Learn button either.
  useEffect(() => {
    if (!ownsFocusRef.current) {
      return;
    }
    if (status === "listening") {
      cancelButtonRef.current?.focus();
      return;
    }
    // Back to the idle subtree. Restore only if focus is currently
    // orphaned on <body> (i.e. it was ours and the element went away).
    if (document.activeElement === document.body || document.activeElement === null) {
      learnButtonRef.current?.focus();
    }
    ownsFocusRef.current = false;
  }, [status]);

  const handleLearn = async () => {
    const svc = midiService();
    if (!svc) {
      setStatus("error");
      setMessage(BRIDGE_UNAVAILABLE_COPY);
      return;
    }
    ownsFocusRef.current = document.activeElement === learnButtonRef.current;
    const isCurrent = beginLatestLearn();
    setStatus("listening");
    setMessage(null);
    try {
      const result = await svc.StartLearn(surfaceName, controlRef);
      if (!isCurrent()) {
        return;
      }
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
      if (!isCurrent()) {
        return;
      }
      setStatus("error");
      setMessage(errorMessage(err));
    }
  };

  const handleCancel = async () => {
    // Claim a fresh generation and drop the predicate on the floor: that
    // is exactly "invalidate whatever StartLearn is still in flight", so
    // its imminent GOLC_MIDI_LEARN_TIMEOUT resolution can no longer write
    // any status.
    beginLatestLearn();
    setStatus("idle");
    setMessage(null);
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
  };

  if (status === "listening") {
    return (
      <div className={styles.learnListening} role="status" aria-live="polite">
        <span>Listening for MIDI input…</span>
        <Button
          ref={cancelButtonRef}
          variant="secondary"
          size="compact"
          leadingIcon={X}
          onClick={() => void handleCancel()}
        >
          Cancel
        </Button>
      </div>
    );
  }

  return (
    <div className={styles.learnControl}>
      <Button
        ref={learnButtonRef}
        variant="secondary"
        size="target"
        leadingIcon={Radio}
        onClick={() => void handleLearn()}
        aria-label={`Learn MIDI mapping for ${controlLabel}`}
      >
        Learn
      </Button>
      {message && (status === "conflict" || status === "timeout" || status === "error") && (
        <p className={styles.learnIssue} role="alert">
          {message}
        </p>
      )}
    </div>
  );
}
