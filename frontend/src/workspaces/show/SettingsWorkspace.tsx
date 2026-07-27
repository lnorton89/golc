// SettingsWorkspace is the Show group's application-preferences workspace.
// Currently holds only Appearance (light/dark/system theme) -- a pure
// client-side preference (lib/theme.ts), never a Go-bound show-state call,
// so unlike OverviewWorkspace/SaveRecoveryWorkspace it does not go through
// wailsBridge.ts at all.
import { useState } from "react";

import { getStoredTheme, setStoredTheme, type ThemePreference } from "../../lib/theme";
import Toolbar from "../../components/primitives/Toolbar/Toolbar";
import Panel from "../../components/primitives/Panel/Panel";
import PanelHeader from "../../components/primitives/PanelHeader/PanelHeader";
import Button from "../../components/primitives/Button/Button";
import styles from "./SettingsWorkspace.module.css";

const THEME_OPTIONS: Array<{ id: ThemePreference; label: string }> = [
  { id: "system", label: "Match System" },
  { id: "light", label: "Light" },
  { id: "dark", label: "Dark" },
];

export default function SettingsWorkspace() {
  const [theme, setTheme] = useState<ThemePreference>(() => getStoredTheme());

  const handleSelect = (next: ThemePreference) => {
    setStoredTheme(next);
    setTheme(next);
  };

  return (
    <div className={styles.workspace}>
      <Toolbar title="Settings" />
      <div className={styles.canvas}>
        <div className={styles.layout}>
          <Panel>
            <PanelHeader label="Appearance" />
            <div className={styles.themeRow} role="group" aria-label="Theme">
              {THEME_OPTIONS.map((option) => (
                <Button
                  key={option.id}
                  variant={theme === option.id ? "primary" : "secondary"}
                  aria-pressed={theme === option.id}
                  onClick={() => handleSelect(option.id)}
                >
                  {option.label}
                </Button>
              ))}
            </div>
          </Panel>
        </div>
      </div>
    </div>
  );
}
