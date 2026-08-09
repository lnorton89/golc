import { Tabs as BaseTabs } from "@base-ui/react/tabs";
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

export default function Tabs({
  "aria-label": ariaLabel,
  tabs,
  value,
  defaultValue,
  onValueChange,
}: TabsProps) {
  return (
    <BaseTabs.Root
      className={styles.tabs}
      value={value}
      defaultValue={defaultValue}
      onValueChange={(next) => onValueChange?.(next as string)}
    >
      <BaseTabs.List className={styles.list} aria-label={ariaLabel} activateOnFocus>
        {tabs.map((tab) => (
          <BaseTabs.Tab key={tab.id} className={styles.tab} value={tab.id} disabled={tab.disabled}>
            {tab.label}
          </BaseTabs.Tab>
        ))}
      </BaseTabs.List>
      {tabs.map((tab) => (
        <BaseTabs.Panel key={tab.id} className={styles.panel} value={tab.id}>
          {tab.panel}
        </BaseTabs.Panel>
      ))}
    </BaseTabs.Root>
  );
}
