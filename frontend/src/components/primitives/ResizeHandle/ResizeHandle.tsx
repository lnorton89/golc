// ResizeHandle is the thin draggable strip a resizable panel places on its
// own resizing edge -- purely presentational (useResizablePanel owns the
// actual size/drag math); this just wires that hook's pointerDown/
// isResizing back into a hit target big enough to grab reliably. Double-
// click resets to the panel's own default size (resetSize), the same
// convention native OS split-view dividers use.
import type { HTMLAttributes, KeyboardEvent, PointerEvent as ReactPointerEvent } from "react";

import styles from "./ResizeHandle.module.css";

interface ResizeHandleProps extends Omit<HTMLAttributes<HTMLDivElement>, "onDoubleClick" | "onPointerDown"> {
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
  /** Current panel geometry and its limits. Supply all four value props to
   * expose a keyboard-operable separator; legacy pointer-only handles remain
   * supported while their owners migrate. */
  value?: number;
  min?: number;
  max?: number;
  step?: number;
  onValueChange?: (value: number) => void;
}

function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
}

export default function ResizeHandle({ onPointerDown, onDoubleClick, isResizing, label, edge, axis = "horizontal", value, min, max, step = 1, onValueChange, onKeyDown, className, ...rest }: ResizeHandleProps) {
  const axisClass = axis === "horizontal" ? styles.axisHorizontal : styles.axisVertical;
  const edgeClass =
    axis === "horizontal"
      ? edge === "end"
        ? styles.handleRight
        : styles.handleLeft
      : edge === "end"
        ? styles.handleBottom
        : styles.handleTop;
  const handleClassName = isResizing
    ? `${styles.handle} ${axisClass} ${edgeClass} ${styles.handleActive}`
    : `${styles.handle} ${axisClass} ${edgeClass}`;
  const bounds =
    typeof value === "number" && Number.isFinite(value) && typeof min === "number" && Number.isFinite(min) && typeof max === "number" && Number.isFinite(max) && min <= max
      ? { min, max, currentValue: clamp(value, min, max) }
      : undefined;
  const hasValueContract = bounds !== undefined && onValueChange !== undefined;
  const resolvedStep = Number.isFinite(step) && step > 0 ? step : 1;

  const handleKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (!hasValueContract || bounds === undefined || onValueChange === undefined) {
      onKeyDown?.(event);
      return;
    }

    let nextValue: number | undefined;
    if (event.key === "Home") nextValue = bounds.min;
    if (event.key === "End") nextValue = bounds.max;
    if (event.key === "ArrowRight" || (axis === "vertical" && event.key === "ArrowUp")) nextValue = bounds.currentValue + resolvedStep;
    if (event.key === "ArrowLeft" || (axis === "vertical" && event.key === "ArrowDown")) nextValue = bounds.currentValue - resolvedStep;
    if (nextValue === undefined) {
      onKeyDown?.(event);
      return;
    }

    event.preventDefault();
    onValueChange(clamp(nextValue, bounds.min, bounds.max));
  };
  return (
    <div
      {...rest}
      role="separator"
      aria-orientation={axis === "horizontal" ? "vertical" : "horizontal"}
      aria-label={label}
      aria-valuemin={bounds?.min}
      aria-valuemax={bounds?.max}
      aria-valuenow={bounds?.currentValue}
      className={[handleClassName, className].filter(Boolean).join(" ")}
      onPointerDown={onPointerDown}
      onDoubleClick={onDoubleClick}
      onKeyDown={handleKeyDown}
      tabIndex={hasValueContract ? 0 : undefined}
    />
  );
}
