// SettingsWorkspace is the Show group's application-preferences workspace.
// Currently holds only Appearance (light/dark/system theme) -- a pure
// client-side preference (lib/theme.ts), never a Go-bound show-state call,
// so unlike OverviewWorkspace/SaveRecoveryWorkspace it does not go through
// wailsBridge.ts at all.
import { useState } from "react";
import { Settings as SettingsIcon, Palette, Sun, Moon, Monitor, type LucideIcon } from "lucide-react";

import { getStoredTheme, setStoredTheme, type ThemePreference } from "../../lib/theme";
import Toolbar from "../../components/primitives/Toolbar/Toolbar";
import Panel from "../../components/primitives/Panel/Panel";
import PanelHeader from "../../components/primitives/PanelHeader/PanelHeader";
import Button from "../../components/primitives/Button/Button";
import styles from "./SettingsWorkspace.module.css";

const THEME_OPTIONS: Array<{ id: ThemePreference; label: string; icon: LucideIcon }> = [
  { id: "system", label: "Match System", icon: Monitor },
  { id: "light", label: "Light", icon: Sun },
  { id: "dark", label: "Dark", icon: Moon },
];

export default function SettingsWorkspace() {
  const [theme, setTheme] = useState<ThemePreference>(() => getStoredTheme());

  const handleSelect = (next: ThemePreference) => {
    setStoredTheme(next);
    setTheme(next);
  };

  return (
    <div className={styles.workspace}>
      <Toolbar title="Settings" icon={SettingsIcon} />
      <div className={styles.canvas}>
        <div className={styles.layout}>
          <Panel>
            <PanelHeader label="Appearance" icon={Palette} />
            <div className={styles.themeRow} role="group" aria-label="Theme">
              {THEME_OPTIONS.map((option) => (
                <Button
                  key={option.id}
                  variant={theme === option.id ? "primary" : "secondary"}
                  icon={option.icon}
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
