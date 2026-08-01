// ResizeHandle is the thin draggable strip a resizable panel places on its
// own resizing edge -- purely presentational (useResizablePanel owns the
// actual size/drag math); this just wires that hook's pointerDown/
// isResizing back into a hit target big enough to grab reliably. Double-
// click resets to the panel's own default size (resetSize), the same
// convention native OS split-view dividers use.
import type { PointerEvent as ReactPointerEvent } from "react";

import styles from "./ResizeHandle.module.css";

interface ResizeHandleProps {
  onPointerDown: (event: ReactPointerEvent) => void;
  onDoubleClick: () => void;
  isResizing: boolean;
  label: string;
  /** Which edge of the panel's own position:relative wrapper this handle
   * sits on -- "end" (the panel's trailing edge: right for a horizontal
   * panel, bottom for a vertical one) or "start" (its leading edge: left
   * or top). Mirrors useResizablePanel's own `edge` option so a panel only
   * ever passes one value to both. */
  edge: "start" | "end";
  /** "horizontal" (default): a vertical col-resize strip on the panel's
   * left/right edge, for width-resizable panels. "vertical": a horizontal
   * row-resize strip on the panel's top/bottom edge, for height-resizable
   * panels (e.g. Desk's per-universe rows). Mirrors useResizablePanel's
   * own `axis` option. */
  axis?: "horizontal" | "vertical";
}

export default function ResizeHandle({ onPointerDown, onDoubleClick, isResizing, label, edge, axis = "horizontal" }: ResizeHandleProps) {
  const axisClass = axis === "horizontal" ? styles.axisHorizontal : styles.axisVertical;
  const edgeClass =
    axis === "horizontal"
      ? edge === "end"
        ? styles.handleRight
        : styles.handleLeft
      : edge === "end"
        ? styles.handleBottom
        : styles.handleTop;
  const className = isResizing
    ? `${styles.handle} ${axisClass} ${edgeClass} ${styles.handleActive}`
    : `${styles.handle} ${axisClass} ${edgeClass}`;
  return (
    <div
      role="separator"
      aria-orientation={axis === "horizontal" ? "vertical" : "horizontal"}
      aria-label={label}
      className={className}
      onPointerDown={onPointerDown}
      onDoubleClick={onDoubleClick}
    />
  );
}
