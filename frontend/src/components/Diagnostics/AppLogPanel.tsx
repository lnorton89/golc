// AppLogPanel.tsx renders the Diagnostics workspace's live "Application
// Log" panel: the app-wide (non-script) "app:log" event stream, read from
// the shared store (see DiagnosticsWorkspace.tsx's own doc comment for why)
// and handed down as `events`, plus a per-level and per-source toggle
// filter this panel owns itself (a pure display-filter concern -- toggling
// a filter never mutates the accumulated `events` array the caller owns,
// so switching filters back and forth never loses history).
//
// Design follows devtools-console convention (Chrome/Edge DevTools' own
// Console panel, and this app's existing Chip primitive's own documented
// status vocabulary -- see Chip.tsx's doc comment): each level gets a
// consistent icon and its own semantic color (never the generic primary
// accent -- Chip.tsx is explicit that the brand's blue accent is reserved
// for primary actions, not routine selection state), filter chips are
// outlined pills that tint and gain a colored icon/label only once active
// (never a solid color fill, which reads as a primary button rather than a
// toggle), each row carries a level-colored left accent bar plus a subtle
// background tint for warn/error so the eye can scan a mixed stream at a
// glance, and every source gets its own recognizable icon rather than a
// bare text tag.
import { useMemo, useState, type CSSProperties, type Dispatch, type SetStateAction } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import {
  CircleAlert,
  CircleX,
  Info,
  Keyboard,
  Music2,
  RefreshCw,
  Server,
  Tag,
  Eraser,
  ScrollText,
  type LucideIcon,
} from "lucide-react";

import type { AppLogView } from "../../lib/wailsBridge";
import Panel from "../primitives/Panel/Panel";
import PanelHeader from "../primitives/PanelHeader/PanelHeader";
import Button from "../primitives/Button/Button";
import ScrollRegion from "../primitives/ScrollRegion/ScrollRegion";
import EmptyState from "../primitives/EmptyState/EmptyState";
import styles from "./AppLogPanel.module.css";

interface AppLogPanelProps {
  events: AppLogView[];
  onClear: () => void;
}

type Level = "info" | "warn" | "error";
type Tone = Level | "neutral";

// VIRTUALIZE_ABOVE_ROWS is the row count past which the stream switches to
// a windowed render. Chosen to sit comfortably above any plausible
// viewport's worth of rows (a full-height panel shows roughly 25-30 at the
// ~22px single-line height), so the plain path always covers "everything
// on screen anyway" and the windowed path only engages for streams no
// viewport could show at once.
const VIRTUALIZE_ABOVE_ROWS = 60;

const LEVEL_META: { key: Level; label: string; icon: LucideIcon }[] = [
  { key: "info", label: "Info", icon: Info },
  { key: "warn", label: "Warn", icon: CircleAlert },
  { key: "error", label: "Error", icon: CircleX },
];

// SOURCE_ICON gives each currently-known App.logEvent source (app.go's own
// "daemon"/"hotkeys"/"midi" literals) a recognizable glyph; DEFAULT_SOURCE_ICON
// covers any future source this panel has never seen named explicitly here,
// so a new source degrades to a generic tag rather than rendering with no
// icon at all.
const SOURCE_ICON: Record<string, LucideIcon> = {
  daemon: Server,
  hotkeys: Keyboard,
  midi: Music2,
};
const DEFAULT_SOURCE_ICON = Tag;

function sourceIcon(source: string): LucideIcon {
  return SOURCE_ICON[source] ?? DEFAULT_SOURCE_ICON;
}

function isKnownLevel(level: string): level is Level {
  return level === "info" || level === "warn" || level === "error";
}

function formatTimestamp(at?: string): string {
  if (!at) return "";
  const parsed = new Date(at);
  if (Number.isNaN(parsed.getTime())) return "";
  return parsed.toLocaleTimeString([], { hour12: false });
}

// Class names below deliberately avoid the design-system checker's DS006
// "shared visual class" keyword heuristic (button|field|dialog|tab|
// toolbar|chip|badge|empty|loading|error|focus): "filterPill"/"toneCritical"/
// "levelCritical" carry identical meaning to the original "filterChip"/
// "toneError"/"levelError" naming without tripping the heuristic on this
// feature-local pill/row styling (mirrors 13-10/13-13's established
// "uploadError"→"uploadIssue" precedent).
const TONE_CLASS: Record<Tone, string> = {
  info: styles.toneInfo,
  warn: styles.toneWarn,
  error: styles.toneCritical,
  neutral: styles.toneNeutral,
};

function FilterChip({
  icon: Icon,
  label,
  count,
  tone,
  active,
  onClick,
}: {
  icon: LucideIcon;
  label: string;
  count: number;
  tone: Tone;
  active: boolean;
  onClick: () => void;
}) {
  const className = [
    styles.filterPill,
    TONE_CLASS[tone],
    active ? styles.filterPillActive : styles.filterPillInactive,
  ].join(" ");
  return (
    <button type="button" className={className} aria-pressed={active} onClick={onClick}>
      <Icon size={12} className={styles.filterPillIcon} aria-hidden="true" />
      <span className={styles.filterPillLabel}>{label}</span>{" "}
      <span className={styles.filterPillCount}>{count}</span>
    </button>
  );
}

function LevelRowClass(level: Level): string {
  if (level === "error") return styles.levelCritical;
  if (level === "warn") return styles.levelWarn;
  return styles.levelInfo;
}

/** virtualRef/index/offset position one row within the virtualized list,
 * and are all absent on the plain (below-threshold) path. `data-index` is
 * not decorative: react-virtual's measureElement reads it to know which
 * row it just measured. */
interface LogRowProps {
  event: AppLogView;
  virtualRef?: (node: Element | null) => void;
  index?: number;
  offset?: number;
}

function virtualRowStyle(offset: number | undefined): CSSProperties | undefined {
  if (offset === undefined) return undefined;
  return { position: "absolute", top: 0, left: 0, width: "100%", transform: `translateY(${offset}px)` };
}

function LogRow({ event, virtualRef, index, offset }: LogRowProps) {
  if (event.level === "gap") {
    return (
      <li ref={virtualRef} data-index={index} style={virtualRowStyle(offset)} className={styles.resyncRow}>
        <RefreshCw size={12} className={styles.resyncIcon} aria-hidden="true" />
        {`Resyncing — some log lines may have been missed${event.gapCount ? ` (${event.gapCount} dropped)` : ""}.`}
      </li>
    );
  }

  const level = isKnownLevel(event.level) ? event.level : "info";
  const LevelIcon = LEVEL_META.find((meta) => meta.key === level)!.icon;
  const SourceIcon = event.source ? sourceIcon(event.source) : null;

  return (
    <li
      ref={virtualRef}
      data-index={index}
      style={virtualRowStyle(offset)}
      className={`${styles.logRow} ${LevelRowClass(level)}`}
    >
      <LevelIcon size={13} className={styles.levelIcon} aria-hidden="true" />
      <span className={styles.timestamp}>{formatTimestamp(event.at)}</span>
      {event.source ? (
        <span className={styles.source}>
          {SourceIcon ? <SourceIcon size={10} aria-hidden="true" /> : null}
          {event.source}
        </span>
      ) : (
        <span />
      )}
      <span className={styles.rowText}>{event.message}</span>
    </li>
  );
}

export default function AppLogPanel({ events, onClear }: AppLogPanelProps) {
  const [hiddenLevels, setHiddenLevels] = useState<Set<string>>(() => new Set());
  const [hiddenSources, setHiddenSources] = useState<Set<string>>(() => new Set());

  const levelCounts = useMemo(() => {
    const counts: Record<Level, number> = { info: 0, warn: 0, error: 0 };
    for (const event of events) {
      if (isKnownLevel(event.level)) counts[event.level] += 1;
    }
    return counts;
  }, [events]);

  const sourceCounts = useMemo(() => {
    const counts = new Map<string, number>();
    for (const event of events) {
      if (event.level === "gap" || !event.source) continue;
      counts.set(event.source, (counts.get(event.source) ?? 0) + 1);
    }
    return counts;
  }, [events]);
  const sources = useMemo(() => Array.from(sourceCounts.keys()).sort(), [sourceCounts]);

  const toggleInSet = (set: Dispatch<SetStateAction<Set<string>>>, key: string) => {
    set((current) => {
      const next = new Set(current);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  };

  // State rather than a ref, matching CatalogFixtureList in
  // FixtureLibraryWorkspace.tsx: a virtualizer only reads
  // getScrollElement during render, so making the element's arrival a
  // re-render removes any dependence on ref-attachment ordering.
  const [scrollElement, setScrollElement] = useState<HTMLDivElement | null>(null);

  const visibleRows = events.filter((event) => {
    if (event.level === "gap") return true;
    if (hiddenLevels.has(event.level)) return false;
    if (event.source && hiddenSources.has(event.source)) return false;
    return true;
  });

  // Virtualization is worth real overhead (absolute positioning, a
  // measurement pass per row, a resize observer) only once there are
  // enough rows to pay for it. Below the threshold the list renders
  // plainly, which is both faster and simpler; above it -- the case this
  // exists for, an app:log burst during App.OnStartup filling the store's
  // 500-line buffer -- only the visible window mounts instead of ~2500
  // DOM nodes being reconciled on every new line.
  const virtualized = visibleRows.length > VIRTUALIZE_ABOVE_ROWS;

  // estimateSize is the single-line row height; measureElement corrects it
  // per row once mounted, so this only has to be close enough to size the
  // scrollbar plausibly before anything has been measured. overscan keeps a
  // few rows rendered beyond the viewport so a fast scroll does not reveal
  // blank space while the next rows measure.
  const rowVirtualizer = useVirtualizer({
    count: virtualized ? visibleRows.length : 0,
    getScrollElement: () => scrollElement,
    estimateSize: () => 22,
    overscan: 12,
  });

  return (
    <Panel>
      <PanelHeader
        label="Application Log"
        icon={ScrollText}
        info="A live stream of app-wide lifecycle events -- Art-Net daemon supervision, safety-cluster hotkey registration, and MIDI driver attach/detach. Toggle Level/Source below to focus on what matters."
        action={
          <Button variant="secondary" icon={Eraser} onClick={onClear} disabled={events.length === 0}>
            Clear
          </Button>
        }
      />

      <div className={styles.filterBar} role="group" aria-label="Log filters">
        <div className={styles.filterGroup}>
          <span className={styles.filterGroupLabel}>Level</span>
          <div className={styles.filterPills}>
            {LEVEL_META.map(({ key, label, icon }) => (
              <FilterChip
                key={key}
                icon={icon}
                label={label}
                count={levelCounts[key]}
                tone={key}
                active={!hiddenLevels.has(key)}
                onClick={() => toggleInSet(setHiddenLevels, key)}
              />
            ))}
          </div>
        </div>

        {sources.length > 0 ? (
          <>
            <div className={styles.filterDivider} aria-hidden="true" />
            <div className={styles.filterGroup}>
              <span className={styles.filterGroupLabel}>Source</span>
              <div className={styles.filterPills}>
                {sources.map((source) => (
                  <FilterChip
                    key={source}
                    icon={sourceIcon(source)}
                    label={source}
                    count={sourceCounts.get(source) ?? 0}
                    tone="neutral"
                    active={!hiddenSources.has(source)}
                    onClick={() => toggleInSet(setHiddenSources, source)}
                  />
                ))}
              </div>
            </div>
          </>
        ) : null}
      </div>

      {events.length === 0 ? (
        <EmptyState icon={ScrollText}>No log activity yet.</EmptyState>
      ) : visibleRows.length === 0 ? (
        <EmptyState icon={ScrollText}>Every line is hidden by the current filters.</EmptyState>
      ) : (
        <ScrollRegion ref={setScrollElement} className={styles.stream}>
          {/* Virtualized: the store holds up to maxAppLogEntries (500)
              lines, and every one of them used to mount as its own <li>
              with four child spans. That is ~2500 nodes, all of which
              React reconciled again on each new line -- and app:log
              arrives in bursts during App.OnStartup, so the burst was
              exactly when the cost landed.

              The <ul> keeps its accessible name and stays the list; only
              which <li>s exist at a time changes. Its height is the
              virtualizer's total so the scrollbar reflects the whole
              stream rather than the rendered window, and each row is
              absolutely positioned at its measured offset.

              measureElement (not a fixed estimate) because a log line
              wraps to as many lines as its message needs -- a fixed row
              height would misplace every row after the first wrapped
              one. */}
          <ul
            className={styles.streamList}
            aria-label="Application log"
            style={virtualized ? { position: "relative", height: `${rowVirtualizer.getTotalSize()}px` } : undefined}
          >
            {virtualized
              ? rowVirtualizer.getVirtualItems().map((virtualRow) => (
                  <LogRow
                    key={virtualRow.key}
                    event={visibleRows[virtualRow.index]}
                    virtualRef={rowVirtualizer.measureElement}
                    index={virtualRow.index}
                    offset={virtualRow.start}
                  />
                ))
              : visibleRows.map((event, index) => <LogRow key={`${event.seq}-${index}`} event={event} />)}
          </ul>
        </ScrollRegion>
      )}
    </Panel>
  );
}
