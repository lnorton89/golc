import { useId, useState } from "react";
import type { ReactNode } from "react";

import styles from "./Tabs.module.css";

export interface TabItem {
  id: string;
  label: string;
  panel: ReactNode;
  disabled?: boolean;
}

interface TabsProps {
  "aria-label": string;
  tabs: readonly TabItem[];
  value?: string;
  defaultValue?: string;
  onValueChange?: (value: string) => void;
}

function firstEnabledTab(tabs: readonly TabItem[]): TabItem | undefined {
  return tabs.find((tab) => !tab.disabled);
}

function enabledTabAt(tabs: readonly TabItem[], startIndex: number, direction: 1 | -1): TabItem | undefined {
  const enabledTabs = tabs.filter((tab) => !tab.disabled);
  if (enabledTabs.length === 0) {
    return undefined;
  }

  for (let offset = 1; offset <= tabs.length; offset += 1) {
    const index = (startIndex + direction * offset + tabs.length) % tabs.length;
    const candidate = tabs[index];
    if (!candidate.disabled) {
      return candidate;
    }
  }

  return undefined;
}

export default function Tabs({
  "aria-label": ariaLabel,
  tabs,
  value,
  defaultValue,
  onValueChange,
}: TabsProps) {
  const initialTab = tabs.find((tab) => tab.id === defaultValue && !tab.disabled) ?? firstEnabledTab(tabs);
  const [uncontrolledValue, setUncontrolledValue] = useState(initialTab?.id);
  const generatedId = useId().replace(/:/g, "");
  const selectedValue = value ?? uncontrolledValue;
  const selectedTab = tabs.find((tab) => tab.id === selectedValue && !tab.disabled) ?? firstEnabledTab(tabs);

  const selectTab = (tab: TabItem) => {
    if (tab.disabled) {
      return;
    }
    if (value === undefined) {
      setUncontrolledValue(tab.id);
    }
    onValueChange?.(tab.id);
  };

  const moveFocus = (destination: TabItem | undefined) => {
    if (!destination) {
      return;
    }
    selectTab(destination);
    const destinationIndex = tabs.findIndex((tab) => tab.id === destination.id);
    const destinationButton = document.getElementById(`${generatedId}-tab-${destinationIndex}`);
    destinationButton?.focus();
  };

  const handleKeyDown = (event: React.KeyboardEvent<HTMLButtonElement>, index: number) => {
    let destination: TabItem | undefined;
    switch (event.key) {
      case "ArrowRight":
      case "ArrowDown":
        destination = enabledTabAt(tabs, index, 1);
        break;
      case "ArrowLeft":
      case "ArrowUp":
        destination = enabledTabAt(tabs, index, -1);
        break;
      case "Home":
        destination = firstEnabledTab(tabs);
        break;
      case "End":
        destination = [...tabs].reverse().find((tab) => !tab.disabled);
        break;
      case "Enter":
      case " ":
        event.preventDefault();
        const tab = tabs[index];
        selectTab(tab);
        return;
      default:
        return;
    }

    event.preventDefault();
    moveFocus(destination);
  };

  if (!selectedTab) {
    return null;
  }

  const selectedIndex = tabs.findIndex((tab) => tab.id === selectedTab.id);
  const selectedTabId = `${generatedId}-tab-${selectedIndex}`;
  const selectedPanelId = `${generatedId}-panel-${selectedIndex}`;

  return (
    <div className={styles.tabs}>
      <div className={styles.list} role="tablist" aria-label={ariaLabel}>
        {tabs.map((tab, index) => {
          const selected = tab.id === selectedTab.id;
          return (
            <button
              key={tab.id}
              id={`${generatedId}-tab-${index}`}
              type="button"
              className={styles.tab}
              role="tab"
              aria-selected={selected}
              aria-controls={`${generatedId}-panel-${index}`}
              tabIndex={selected ? 0 : -1}
              disabled={tab.disabled}
              onClick={() => selectTab(tab)}
              onKeyDown={(event) => handleKeyDown(event, index)}
            >
              {tab.label}
            </button>
          );
        })}
      </div>
      <div id={selectedPanelId} className={styles.panel} role="tabpanel" aria-labelledby={selectedTabId}>
        {selectedTab.panel}
      </div>
    </div>
  );
}
