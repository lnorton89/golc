// ScriptRunDialog is the Run/Debug launch dialog (08-10-PLAN.md Task 2,
// D-07/D-09): a user reviews and edits the exact capability scope,
// resource-limit preset, and (under Advanced) the four raw numeric limits
// a run will use before launch. Follows HelpOverlay.tsx's exact backdrop +
// dialog pattern (08-UI-SPEC.md's explicit correction of 06-UI-SPEC.md's
// stale Radix reference: backdrop click and Escape close the dialog,
// focus moves onto it on open, and it carries the standard dialog/modal
// ARIA attributes) -- no Radix, no icon library, plain HTML controls
// under CSS Modules.
//
// This component only ever calls one callback, onSubmit(profile, mode):
// the caller (ScriptsWorkspace.tsx, 08-10-PLAN.md Task 3) is responsible
// for both persisting the edited profile via SetScriptProfile (D-07: an
// edited profile becomes the new saved default) and then launching via
// RunScript/DebugScript, awaiting both before resolving -- "Submitting
// calls the profile-save callback with the edited values and then the
// launch callback" is satisfied by the caller's own onSubmit
// implementation, not by two separate props here.
import { useEffect, useRef, useState, type FormEvent } from "react";
import { X, Play, Bug } from "lucide-react";

import { errorMessage } from "../../lib/wailsBridge";
import Field from "../primitives/Field/Field";
import Button from "../primitives/Button/Button";
import styles from "./ScriptRunDialog.module.css";

export type ScriptLaunchMode = "run" | "debug";

/** ScriptDialogProfile is the dialog's controlled capability/resource-
 * limit profile shape -- mirrors internal/wails.ScriptSummaryView's own
 * scope/preset/deadlineSeconds/ratePerSecond/memoryLimitMB/cpuCapPercent
 * fields exactly (field-for-field, same names), so a caller can pass a
 * ScriptSummaryView/ScriptDetailView straight through as the initial
 * profile. */
export interface ScriptDialogProfile {
  scope: string;
  preset: string;
  deadlineSeconds: number;
  ratePerSecond: number;
  memoryLimitMB: number;
  cpuCapPercent: number;
}

interface ScriptRunDialogProps {
  mode: ScriptLaunchMode;
  scriptName: string;
  profile: ScriptDialogProfile;
  onSubmit: (profile: ScriptDialogProfile, mode: ScriptLaunchMode) => Promise<void>;
  onCancel: () => void;
}

// D-06: scope reuses Phase 7's coarse API-key domain scopes verbatim
// (show.APIKeyScope's closed set) -- the UI-SPEC's exact Copywriting
// Contract labels over the backend's own kebab/lower-case wire values.
const SCOPE_OPTIONS: Array<{ value: string; label: string }> = [
  { value: "playback", label: "Playback" },
  { value: "authoring", label: "Authoring" },
  { value: "admin", label: "Admin" },
];

// D-09: named presets plus the "advanced" escape hatch -- values match
// internal/show/scripts.go's ResourcePreset closed set exactly
// (quick-action/long-running-automation/advanced).
const PRESET_OPTIONS: Array<{ value: string; label: string }> = [
  { value: "quick-action", label: "Quick action" },
  { value: "long-running-automation", label: "Long-running automation" },
  { value: "advanced", label: "Advanced (custom)" },
];

const ADVANCED_PRESET_VALUE = "advanced";

export default function ScriptRunDialog({ mode, scriptName, profile, onSubmit, onCancel }: ScriptRunDialogProps) {
  const dialogRef = useRef<HTMLDivElement>(null);

  const [scope, setScope] = useState(profile.scope);
  const [preset, setPreset] = useState(profile.preset);
  const [deadlineSeconds, setDeadlineSeconds] = useState(profile.deadlineSeconds);
  const [ratePerSecond, setRatePerSecond] = useState(profile.ratePerSecond);
  const [memoryLimitMB, setMemoryLimitMB] = useState(profile.memoryLimitMB);
  const [cpuCapPercent, setCpuCapPercent] = useState(profile.cpuCapPercent);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Focus moves into the dialog on open (08-UI-SPEC.md's Modal/dialog
  // pattern) -- mirrors HelpOverlay.tsx's closeButtonRef.current?.focus()
  // exactly, focusing the dialog surface itself (tabIndex={-1} below)
  // rather than a specific control, since this dialog has several equally
  // reasonable first-focus candidates.
  useEffect(() => {
    dialogRef.current?.focus();
  }, []);

  // Escape closes without launching -- disabled while a submit is in
  // flight, mirroring the Cancel button and backdrop click below: "the
  // dialog does not close" while submitting (08-10-PLAN.md Task 2's own
  // <behavior> bullet).
  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape" && !submitting) {
        onCancel();
      }
    };
    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [onCancel, submitting]);

  // The dialog title/CTA text always uses the mode's own static label;
  // aria-label (below) stays fixed to this same static label so a screen
  // reader's announced accessible name never churns between "Run" and
  // "Launching…" mid-submit -- only the button's visible text switches.
  const title = mode === "run" ? `Run ${scriptName}` : `Debug ${scriptName}`;
  const submitLabel = mode === "run" ? "Run" : "Start Debugging";
  const submitAriaLabel = `${submitLabel} ${scriptName}`;

  const handleBackdropClick = () => {
    if (!submitting) {
      onCancel();
    }
  };

  const handleSubmit = (event: FormEvent) => {
    event.preventDefault();
    setSubmitting(true);
    setError(null);
    void (async () => {
      try {
        await onSubmit({ scope, preset, deadlineSeconds, ratePerSecond, memoryLimitMB, cpuCapPercent }, mode);
      } catch (err) {
        setError(errorMessage(err));
        setSubmitting(false);
      }
    })();
  };

  return (
    <div className={styles.backdrop} onClick={handleBackdropClick}>
      <div
        ref={dialogRef}
        className={styles.dialog}
        role="dialog"
        aria-modal="true"
        aria-label={title}
        tabIndex={-1}
        onClick={(event) => event.stopPropagation()}
      >
        <form className={styles.form} onSubmit={handleSubmit}>
          <div className={styles.header}>
            <span className={styles.title}>
              {mode === "run" ? <Play size={16} aria-hidden="true" /> : <Bug size={16} aria-hidden="true" />}
              {title}
            </span>
          </div>

          <div className={styles.body}>
            <label className={styles.selectField}>
              <span className={styles.selectLabel}>Capability scope</span>
              <select
                className={styles.select}
                value={scope}
                disabled={submitting}
                onChange={(event) => setScope(event.target.value)}
              >
                {SCOPE_OPTIONS.map((option) => (
                  <option key={option.value} value={option.value}>
                    {option.label}
                  </option>
                ))}
              </select>
            </label>

            <label className={styles.selectField}>
              <span className={styles.selectLabel}>Resource limits</span>
              <select
                className={styles.select}
                value={preset}
                disabled={submitting}
                onChange={(event) => setPreset(event.target.value)}
              >
                {PRESET_OPTIONS.map((option) => (
                  <option key={option.value} value={option.value}>
                    {option.label}
                  </option>
                ))}
              </select>
            </label>

            {preset === ADVANCED_PRESET_VALUE ? (
              <div className={styles.advancedGrid}>
                <Field
                  label="Deadline (seconds)"
                  type="number"
                  min={1}
                  max={86400}
                  step={1}
                  value={deadlineSeconds}
                  disabled={submitting}
                  onChange={(event) => setDeadlineSeconds(Number(event.target.value))}
                />
                <Field
                  label="Rate limit (calls/sec)"
                  type="number"
                  min={1}
                  max={1000}
                  step={1}
                  value={ratePerSecond}
                  disabled={submitting}
                  onChange={(event) => setRatePerSecond(Number(event.target.value))}
                />
                <Field
                  label="Memory limit (MB)"
                  type="number"
                  min={1}
                  max={8192}
                  step={1}
                  value={memoryLimitMB}
                  disabled={submitting}
                  onChange={(event) => setMemoryLimitMB(Number(event.target.value))}
                />
                <Field
                  label="CPU cap (%)"
                  type="number"
                  min={1}
                  max={100}
                  step={1}
                  value={cpuCapPercent}
                  disabled={submitting}
                  onChange={(event) => setCpuCapPercent(Number(event.target.value))}
                />
              </div>
            ) : null}

            {error ? <p className={styles.errorText}>{error}</p> : null}
          </div>

          <div className={styles.actions}>
            <Button type="button" variant="secondary" icon={X} onClick={onCancel} disabled={submitting}>
              Cancel
            </Button>
            <Button
              type="submit"
              variant="primary"
              icon={mode === "run" ? Play : Bug}
              disabled={submitting}
              aria-label={submitAriaLabel}
            >
              {submitting ? "Launching…" : submitLabel}
            </Button>
          </div>
        </form>
      </div>
    </div>
  );
}
