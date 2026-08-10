// Toast is the app's transient-confirmation surface: the affordance for
// "that worked" / "that didn't" feedback which needs to be noticed but
// must not take focus, block the operator, or occupy permanent layout.
// Before this primitive there was no notification mechanism at all -- a
// successful action either re-rendered some list and said nothing, or
// reported failure only as inline text inside the panel that owned it.
//
// Built on @base-ui/react/toast rather than a standalone toast library:
// Base UI already backs Dialog/Popover/Tooltip here, so its portal,
// focus, and dismissal semantics are the ones the rest of this design
// system already agrees with. A second library would introduce a parallel
// portal/stacking model next to the one Dialog already established.
//
// PLACEMENT IS A SAFETY CONSTRAINT, NOT A STYLE CHOICE. GlobalFrame's
// fixed header owns LiveStatusBar and SafetyCluster (Blackout / Revoke
// Automation), and D-13 requires that no overlay can ever intercept those
// controls. The viewport is therefore anchored bottom-right -- structurally
// unable to cover the header -- and additionally carries
// `pointer-events: none`, with pointer events re-enabled only on the toast
// cards themselves. Even a toast that visually overlapped a control could
// not swallow the click that dismisses a blackout.
//
// The emit side is `useToast()` below, exported from this module rather
// than the design-system barrel -- the same shape HoverTooltip.tsx already
// uses for the non-inventory half of a primitive, and required by the barrel's
// contract test, which asserts the barrel's runtime exports match the
// component inventory one-for-one.
import { Toast as BaseToast } from "@base-ui/react/toast";
import { X } from "lucide-react";
import type { ReactNode } from "react";

import styles from "./Toast.module.css";

/** TOAST_LIMIT caps how many cards stack at once. A lighting operator mid-
 * show should never have to read a wall of them: past three, the oldest is
 * dropped rather than growing the stack up toward the safety header. */
const TOAST_LIMIT = 3;

/** TOAST_TIMEOUT_MS is the auto-dismiss delay. Deliberately longer than a
 * web app's usual ~4s: this is a desktop console whose operator is often
 * looking at the rig rather than the screen when an action completes. */
const TOAST_TIMEOUT_MS = 6000;

/** ToastTone maps onto the app's existing status vocabulary (Chip's own
 * tones) rather than inventing a second success/danger palette -- "added
 * to library" is the same green as a "Valid" chip, and a failure the same
 * red as "Invalid". */
export type ToastTone = "neutral" | "success" | "error";

const TONE_CLASS: Record<ToastTone, string> = {
  neutral: styles.neutral,
  success: styles.success,
  error: styles.error,
};

export interface ToastProps {
  children: ReactNode;
}

export default function Toast({ children }: ToastProps) {
  return (
    <BaseToast.Provider limit={TOAST_LIMIT} timeout={TOAST_TIMEOUT_MS}>
      {children}
      <BaseToast.Portal>
        <BaseToast.Viewport className={styles.viewport}>
          <ToastList />
        </BaseToast.Viewport>
      </BaseToast.Portal>
    </BaseToast.Provider>
  );
}

function ToastList() {
  const { toasts } = BaseToast.useToastManager();

  return (
    <>
      {toasts.map((toast) => {
        const tone = (toast.data as { tone?: ToastTone } | undefined)?.tone ?? "neutral";
        return (
          <BaseToast.Root key={toast.id} toast={toast} className={`${styles.toast} ${TONE_CLASS[tone]}`}>
            <BaseToast.Content className={styles.content}>
              <BaseToast.Title className={styles.title} />
              {toast.description ? <BaseToast.Description className={styles.description} /> : null}
            </BaseToast.Content>
            <BaseToast.Close className={styles.close} aria-label="Dismiss notification">
              <X className={styles.closeIcon} aria-hidden="true" />
            </BaseToast.Close>
          </BaseToast.Root>
        );
      })}
    </>
  );
}

export interface ToastEmitter {
  success: (title: string, description?: string) => void;
  error: (title: string, description?: string) => void;
  show: (title: string, description?: string) => void;
}

/** useToast is the emit half of this primitive. It deliberately exposes
 * three intent-named calls rather than Base UI's raw `add({...})`, so a
 * call site states what happened and never picks a colour itself.
 *
 * Must be called beneath a mounted <Toast> (AppShell mounts exactly one). */
export function useToast(): ToastEmitter {
  const manager = BaseToast.useToastManager();

  return {
    success: (title, description) => manager.add({ title, description, data: { tone: "success" } }),
    // Failures hold longer than the shared default: an operator who missed
    // why something did not happen has no other record of it once the card
    // is gone.
    error: (title, description) =>
      manager.add({ title, description, data: { tone: "error" }, timeout: TOAST_TIMEOUT_MS * 2 }),
    show: (title, description) => manager.add({ title, description, data: { tone: "neutral" } }),
  };
}
