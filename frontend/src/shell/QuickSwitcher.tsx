// QuickSwitcher is the Ctrl+K "jump to any workspace" search overlay
// (lib/hotkeys.ts's NAVIGATION_SHORTCUTS), mirroring Discord's own Quick
// Switcher -- opened/closed by useGlobalKeyboardWorkflow.ts's window-level
// Ctrl+K listener. Filtering and arrow-key/Enter selection are handled
// locally on the search input rather than a second window listener, so
// they naturally respect normal input focus semantics and never race the
// window-level Ctrl+K toggle.
import { useEffect, useMemo, useRef, useState } from "react";
import { Search } from "lucide-react";

import { NAV_GROUPS, type DestinationId } from "./navigation";
import { DESTINATION_ICONS } from "./destinationIcons";
import styles from "./QuickSwitcher.module.css";

interface QuickSwitcherResult {
  id: DestinationId;
  label: string;
  groupLabel: string;
}

const RESULTS: QuickSwitcherResult[] = NAV_GROUPS.flatMap((group) =>
  group.destinations.map((destination) => ({
    id: destination.id,
    label: destination.label,
    groupLabel: group.label,
  })),
);

interface QuickSwitcherProps {
  open: boolean;
  onClose: () => void;
  onNavigate: (id: DestinationId) => void;
}

export default function QuickSwitcher({ open, onClose, onNavigate }: QuickSwitcherProps) {
  const [query, setQuery] = useState("");
  const [selectedIndex, setSelectedIndex] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);

  const results = useMemo(() => {
    const needle = query.trim().toLowerCase();
    if (!needle) {
      return RESULTS;
    }
    return RESULTS.filter(
      (result) => result.label.toLowerCase().includes(needle) || result.groupLabel.toLowerCase().includes(needle),
    );
  }, [query]);

  useEffect(() => {
    if (open) {
      setQuery("");
      setSelectedIndex(0);
      inputRef.current?.focus();
    }
  }, [open]);

  useEffect(() => {
    setSelectedIndex(0);
  }, [results.length]);

  if (!open) {
    return null;
  }

  const commit = (id: DestinationId) => {
    onNavigate(id);
    onClose();
  };

  return (
    <div className={styles.backdrop} onClick={onClose}>
      <div
        className={styles.dialog}
        role="dialog"
        aria-modal="true"
        aria-label="Quick switcher"
        onClick={(event) => event.stopPropagation()}
      >
        <div className={styles.searchRow}>
          <Search size={14} className={styles.searchIcon} aria-hidden="true" />
          <input
            ref={inputRef}
            type="text"
            className={styles.input}
            placeholder="Jump to a workspace…"
            aria-label="Jump to a workspace"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === "Escape") {
                event.preventDefault();
                onClose();
                return;
              }
              if (event.key === "ArrowDown") {
                event.preventDefault();
                setSelectedIndex((current) => (results.length === 0 ? 0 : (current + 1) % results.length));
                return;
              }
              if (event.key === "ArrowUp") {
                event.preventDefault();
                setSelectedIndex((current) =>
                  results.length === 0 ? 0 : (current - 1 + results.length) % results.length,
                );
                return;
              }
              if (event.key === "Enter") {
                event.preventDefault();
                const selected = results[selectedIndex];
                if (selected) {
                  commit(selected.id);
                }
              }
            }}
          />
        </div>
        <ul className={styles.results} role="listbox" aria-label="Workspaces">
          {results.length === 0 ? (
            <li className={styles.empty}>No matching workspace</li>
          ) : (
            results.map((result, index) => {
              const Icon = DESTINATION_ICONS[result.id];
              const isSelected = index === selectedIndex;
              return (
                <li key={result.id}>
                  <button
                    type="button"
                    role="option"
                    aria-selected={isSelected}
                    className={isSelected ? `${styles.result} ${styles.resultSelected}` : styles.result}
                    onMouseEnter={() => setSelectedIndex(index)}
                    onClick={() => commit(result.id)}
                  >
                    <Icon size={15} className={styles.resultIcon} aria-hidden="true" />
                    <span className={styles.resultLabel}>{result.label}</span>
                    <span className={styles.resultGroup}>{result.groupLabel}</span>
                  </button>
                </li>
              );
            })
          )}
        </ul>
      </div>
    </div>
  );
}
