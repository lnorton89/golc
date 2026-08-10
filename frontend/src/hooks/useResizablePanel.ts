// useResizablePanel drives a single draggable panel dimension: pointer-drag
// delta (clamped to [min, max]) plus a localStorage-persisted last size, so
// a user's chosen rail/sidebar/inspector width (or a vertically-resizable
// panel's height) survives a reload. Every resizable panel in the shell
// (CommandRail, ContextualInspector, workspace sidebars, Desk's per-
// universe rows) shares this one hook rather than each hand-rolling its own
// pointer-event bookkeeping.
import { useCallback, useEffect, useRef, useState, type PointerEvent as ReactPointerEvent } from "react";

interface UseResizablePanelOptions {
  min: number;
  max: number;
  defaultSize: number;
  /** localStorage key this panel's chosen size is persisted under. */
  storageKey: string;
  /** Which side of the panel the drag handle sits on: "end" for a handle
   * on the panel's trailing edge -- right for a horizontal panel (dragging
   * right grows it, e.g. a left-hand rail/sidebar), bottom for a vertical
   * one (dragging down grows it) -- or "start" for a handle on its leading
   * edge (dragging left/up grows it, e.g. a right-hand inspector). */
  edge: "start" | "end";
  /** "horizontal" (default) drags along X and produces a width; "vertical"
   * drags along Y and produces a height. Same clamp/persist/edge semantics
   * either way -- only which pointer coordinate is read changes. */
  axis?: "horizontal" | "vertical";
}

function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
}

function readStoredSize(storageKey: string, min: number, max: number, fallback: number): number {
  if (typeof window === "undefined") return fallback;
  const raw = window.localStorage.getItem(storageKey);
  const parsed = raw === null ? NaN : Number(raw);
  return Number.isFinite(parsed) ? clamp(parsed, min, max) : fallback;
}

export function useResizablePanel({ min, max, defaultSize, storageKey, edge, axis = "horizontal" }: UseResizablePanelOptions) {
  const [size, setSize] = useState(() => readStoredSize(storageKey, min, max, defaultSize));
  const [isResizing, setIsResizing] = useState(false);
  // Plain ref, not state: the drag anchor only needs to be read inside the
  // pointermove listener below, and re-rendering on every recorded anchor
  // would be pure waste.
  const dragStart = useRef<{ pointer: number; startSize: number } | null>(null);
  // The element that captured the pointer for this drag, so the capture
  // can be released again when the drag ends.
  const captureTarget = useRef<{ element: Element; pointerId: number } | null>(null);

  const handlePointerDown = useCallback(
    (event: ReactPointerEvent) => {
      // Primary button only. Without this, a right- or middle-click on the
      // handle started a resize -- and the preventDefault() below then
      // suppressed the context menu that would otherwise have explained
      // what was happening.
      if (event.button !== 0) {
        return;
      }
      event.preventDefault();
      // Pointer capture keeps the drag's own pointerup addressed to this
      // handle even when the pointer is released outside the webview.
      // Without it, releasing past the window edge delivered no pointerup
      // anywhere, isResizing stayed true, and the panel silently resumed
      // resizing the moment the cursor re-entered.
      const element = event.currentTarget;
      try {
        element.setPointerCapture?.(event.pointerId);
        captureTarget.current = { element, pointerId: event.pointerId };
      } catch {
        // Capture is a best-effort improvement, never a precondition: the
        // window-level listeners below still terminate the drag normally.
        captureTarget.current = null;
      }
      const pointer = axis === "horizontal" ? event.clientX : event.clientY;
      dragStart.current = { pointer, startSize: size };
      setIsResizing(true);
    },
    [size, axis],
  );

  useEffect(() => {
    if (!isResizing) return;

    const handlePointerMove = (event: PointerEvent) => {
      if (!dragStart.current) return;
      const pointer = axis === "horizontal" ? event.clientX : event.clientY;
      const delta = pointer - dragStart.current.pointer;
      const signedDelta = edge === "end" ? delta : -delta;
      setSize(clamp(dragStart.current.startSize + signedDelta, min, max));
    };
    const stopResizing = () => {
      const capture = captureTarget.current;
      if (capture) {
        try {
          capture.element.releasePointerCapture?.(capture.pointerId);
        } catch {
          // Already released (the browser does this itself on pointerup).
        }
        captureTarget.current = null;
      }
      dragStart.current = null;
      setIsResizing(false);
    };

    // Listening on window (not the handle element) is what lets the drag
    // keep tracking the pointer even once it leaves the handle's own thin
    // hit area mid-drag -- a real user's cursor overshoots a 6px strip
    // constantly.
    window.addEventListener("pointermove", handlePointerMove);
    window.addEventListener("pointerup", stopResizing);
    window.addEventListener("pointercancel", stopResizing);
    window.addEventListener("lostpointercapture", stopResizing);
    // A drag cannot meaningfully continue once the window loses focus, and
    // this is the last backstop for a release the webview never sees.
    window.addEventListener("blur", stopResizing);
    return () => {
      window.removeEventListener("pointermove", handlePointerMove);
      window.removeEventListener("pointerup", stopResizing);
      window.removeEventListener("pointercancel", stopResizing);
      window.removeEventListener("lostpointercapture", stopResizing);
      window.removeEventListener("blur", stopResizing);
    };
  }, [isResizing, edge, axis, min, max]);

  // Persist on drag END, not on every pointermove frame. `size` is set
  // from handlePointerMove on every pointer event, so keying this effect
  // on `size` alone performed hundreds of synchronous localStorage.setItem
  // calls per one-second drag -- and Desk instantiates two of these hooks
  // per UniverseRow, so dragging one row's handle ran that while every
  // other row's effect was live too. Skipping while isResizing is
  // behaviourally identical for the user-visible contract: the final value
  // is written the moment the drag ends (isResizing flips, this re-runs),
  // and non-drag changes (resetSize, setClampedSize) still persist
  // immediately.
  useEffect(() => {
    if (isResizing) return;
    window.localStorage.setItem(storageKey, String(size));
  }, [storageKey, size, isResizing]);

  const resetSize = useCallback(() => setSize(defaultSize), [defaultSize]);

  // setClampedSize is the imperative escape hatch drag alone doesn't cover
  // -- e.g. a row of "Compact / Normal / Large" preset buttons jumping
  // straight to a specific value rather than the user dragging there by
  // hand. Clamps exactly like the drag path so a preset can never punch
  // past this panel's own min/max.
  const setClampedSize = useCallback((next: number) => setSize(clamp(next, min, max)), [min, max]);

  return { size, isResizing, handlePointerDown, resetSize, setSize: setClampedSize };
}
