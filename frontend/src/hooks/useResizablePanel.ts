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

  const handlePointerDown = useCallback(
    (event: ReactPointerEvent) => {
      event.preventDefault();
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
    return () => {
      window.removeEventListener("pointermove", handlePointerMove);
      window.removeEventListener("pointerup", stopResizing);
      window.removeEventListener("pointercancel", stopResizing);
    };
  }, [isResizing, edge, axis, min, max]);

  useEffect(() => {
    window.localStorage.setItem(storageKey, String(size));
  }, [storageKey, size]);

  const resetSize = useCallback(() => setSize(defaultSize), [defaultSize]);

  return { size, isResizing, handlePointerDown, resetSize };
}
