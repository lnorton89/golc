// SettingsWorkspace is the Show group's application-preferences workspace.
// Currently holds only Appearance (light/dark/system theme) -- a pure
// client-side preference (lib/theme.ts), never a Go-bound show-state call,
// so unlike OverviewWorkspace/SaveRecoveryWorkspace it does not go through
// wailsBridge.ts at all.
import { useState } from "react";
import {
  Settings as SettingsIcon,
  Palette,
  Sun,
  Moon,
  Monitor,
  Info,
  Keyboard,
  ChevronRight,
  ChevronDown,
  ExternalLink,
  type LucideIcon,
} from "lucide-react";

import { getStoredTheme, setStoredTheme, type ThemePreference } from "../../lib/theme";
import { openExternalURL } from "../../lib/wailsBridge";
import Toolbar from "../../components/primitives/Toolbar/Toolbar";
import Panel from "../../components/primitives/Panel/Panel";
import PanelHeader from "../../components/primitives/PanelHeader/PanelHeader";
import Button from "../../components/primitives/Button/Button";
import ScrollRegion from "../../components/primitives/ScrollRegion/ScrollRegion";
import HotkeySettings from "../../components/HotkeySettings/HotkeySettings";
import styles from "./SettingsWorkspace.module.css";

const THEME_OPTIONS: Array<{ id: ThemePreference; label: string; icon: LucideIcon }> = [
  { id: "system", label: "Match System", icon: Monitor },
  { id: "light", label: "Light", icon: Sun },
  { id: "dark", label: "Dark", icon: Moon },
];

interface Credit {
  name: string;
  version: string;
  license: string;
  /** What the library itself is/does, in general -- its own README's
   * one-liner, not GOLC-specific. */
  description: string;
  /** How GOLC itself uses this dependency -- verified against actual
   * import sites (e.g. `grep -rl` for the module path under internal/,
   * or the frontend equivalent) rather than assumed from the package's
   * own README. */
  usage: string;
  url: string;
}

// Direct dependencies actually compiled into the golc-desktop binary/bundle
// (verified via `go list -deps ./cmd/golc-desktop/...` for the Go side --
// excludes e.g. modelcontextprotocol/go-sdk, which cmd/golc-project pulls
// in but golc-desktop never imports) and the frontend's npm runtime deps.
// Transitive/indirect packages are intentionally omitted -- this credits
// what the project chose to depend on, not the full module graph. Each
// url is that package's pkg.go.dev/npmjs.com page rather than a hand-picked
// repo link -- both resolve deterministically from the module/package name,
// so there is no risk of pointing at the wrong fork or a stale URL.
const BACKEND_CREDITS: Credit[] = [
  {
    name: "Wails v2",
    version: "2.13.0",
    license: "MIT",
    description: "Go + web-frontend desktop application framework",
    usage: "Desktop shell: window, native dialogs, OS bridge",
    url: "https://pkg.go.dev/github.com/wailsapp/wails/v2",
  },
  {
    name: "chi",
    version: "5.3.1",
    license: "MIT",
    description: "Lightweight HTTP router",
    usage: "Routes the external /v1 control API",
    url: "https://pkg.go.dev/github.com/go-chi/chi/v5",
  },
  {
    name: "huma",
    version: "2.39.0",
    license: "MIT",
    description: "REST API framework with OpenAPI generation",
    usage: "The /v1 API's request handling and OpenAPI contract",
    url: "https://pkg.go.dev/github.com/danielgtaylor/huma/v2",
  },
  {
    name: "gomidi/midi",
    version: "2.3.24",
    license: "MIT",
    description: "MIDI I/O and driver bindings",
    usage: "MIDI Note/CC learn and controller I/O",
    url: "https://pkg.go.dev/gitlab.com/gomidi/midi/v2",
  },
  {
    name: "go-winio",
    version: "0.6.2",
    license: "MIT",
    description: "Windows named pipes and other Win32 I/O primitives",
    usage: "Named-pipe IPC to the supervised Art-Net daemon",
    url: "https://pkg.go.dev/github.com/Microsoft/go-winio",
  },
  {
    name: "cdp",
    version: "0.35.0",
    license: "MIT",
    description: "Chrome DevTools Protocol client",
    usage: "Drives the Scripts workspace's step debugger",
    url: "https://pkg.go.dev/github.com/mafredri/cdp",
  },
  {
    name: "hotkey",
    version: "0.6.1",
    license: "MIT",
    description: "Cross-platform global hotkey registration",
    usage: "Registers the OS-level safety-cluster hotkeys",
    url: "https://pkg.go.dev/golang.design/x/hotkey",
  },
  {
    name: "jsonschema",
    version: "0.14.0",
    license: "MIT",
    description: "Go struct to JSON Schema reflection",
    usage: "Reflects Go types into the /v1 API's OpenAPI schema",
    url: "https://pkg.go.dev/github.com/invopop/jsonschema",
  },
  {
    name: "toml",
    version: "1.6.0",
    license: "MIT",
    description: "TOML encoding and decoding",
    usage: "Reads and writes GOLC's TOML config files",
    url: "https://pkg.go.dev/github.com/BurntSushi/toml",
  },
  {
    name: "uuid",
    version: "1.6.0",
    license: "BSD-3-Clause",
    description: "UUID generation and parsing",
    usage: "IDs for pools, deployments, scenes, surfaces",
    url: "https://pkg.go.dev/github.com/google/uuid",
  },
  {
    name: "golang.org/x/sys, x/time",
    version: "0.46.0, 0.15.0",
    license: "BSD-3-Clause",
    description: "Extended Go standard library packages",
    usage: "Per-key API rate limiting; Windows registry/pipe access",
    url: "https://pkg.go.dev/golang.org/x",
  },
  {
    name: "modernc.org/sqlite",
    version: "1.54.0",
    license: "BSD-3-Clause",
    description: "cgo-free SQLite driver",
    usage: "Durable show storage and recovery points",
    url: "https://pkg.go.dev/modernc.org/sqlite",
  },
  {
    name: "go-yaml",
    version: "4.0.0-rc.6",
    license: "Apache-2.0",
    description: "YAML encoding and decoding",
    usage: "Decodes hand-authored and OFL fixture YAML files",
    url: "https://pkg.go.dev/go.yaml.in/yaml/v4",
  },
];

const FRONTEND_CREDITS: Credit[] = [
  {
    name: "React",
    version: "19.2.7",
    license: "MIT",
    description: "UI component library",
    usage: "Renders every workspace and shell surface",
    url: "https://www.npmjs.com/package/react",
  },
  {
    name: "Zustand",
    version: "5.0.14",
    license: "MIT",
    description: "Client-side state management",
    usage: "Caches Go-pushed live status (playback, safety)",
    url: "https://www.npmjs.com/package/zustand",
  },
  {
    name: "Monaco Editor",
    version: "0.55.1",
    license: "MIT",
    description: "Code editor component",
    usage: "Powers the Scripts workspace's code editor",
    url: "https://www.npmjs.com/package/monaco-editor",
  },
  {
    name: "Lucide",
    version: "1.27.0",
    license: "ISC",
    description: "Icon set",
    usage: "Icon set used across the whole shell",
    url: "https://www.npmjs.com/package/lucide-react",
  },
  {
    name: "Archivo",
    version: "5.3.0",
    license: "OFL-1.1",
    description: "Interface typeface",
    usage: "Interface typeface used throughout the shell",
    url: "https://www.npmjs.com/package/@fontsource/archivo",
  },
  {
    name: "JetBrains Mono",
    version: "5.3.0",
    license: "OFL-1.1",
    description: "Monospace typeface",
    usage: "Monospace typeface for IDs, code, and meta text",
    url: "https://www.npmjs.com/package/@fontsource/jetbrains-mono",
  },
];

interface CreditRowProps {
  credit: Credit;
  expanded: boolean;
  onToggle: () => void;
}

function CreditRow({ credit, expanded, onToggle }: CreditRowProps) {
  const ChevronIcon = expanded ? ChevronDown : ChevronRight;

  return (
    <li>
      <button
        type="button"
        className={styles.creditRow}
        aria-expanded={expanded}
        onClick={onToggle}
      >
        <ChevronIcon size={14} className={styles.creditChevron} aria-hidden="true" />
        <span className={styles.creditName}>{credit.name}</span>
        <span className={styles.creditDescription}>{credit.description}</span>
        <span className={styles.creditMeta}>
          {credit.license} · v{credit.version}
        </span>
      </button>

      {expanded ? (
        <div className={styles.creditDetail}>
          <p className={styles.creditUsage}>{credit.usage}</p>
          <button
            type="button"
            className={styles.creditLink}
            onClick={() => void openExternalURL(credit.url)}
          >
            View project
            <ExternalLink size={12} aria-hidden="true" />
          </button>
        </div>
      ) : null}
    </li>
  );
}

export default function SettingsWorkspace() {
  const [theme, setTheme] = useState<ThemePreference>(() => getStoredTheme());
  const [expandedCredits, setExpandedCredits] = useState<Set<string>>(() => new Set());

  const handleSelect = (next: ThemePreference) => {
    setStoredTheme(next);
    setTheme(next);
  };

  const toggleCredit = (key: string) => {
    setExpandedCredits((current) => {
      const next = new Set(current);
      if (next.has(key)) {
        next.delete(key);
      } else {
        next.add(key);
      }
      return next;
    });
  };

  return (
    <div className={styles.workspace}>
      <Toolbar title="Settings" icon={SettingsIcon} />
      <ScrollRegion className={styles.canvas}>
        <div className={styles.layout}>
          <Panel className={styles.appearancePanel}>
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

          <Panel className={styles.hotkeysPanel}>
            <PanelHeader label="Hotkeys" icon={Keyboard} />
            <HotkeySettings />
          </Panel>

          <Panel className={styles.aboutPanel}>
            <PanelHeader label="About" icon={Info} />
            <div className={styles.about}>
              <div className={styles.aboutHeading}>
                <span className={styles.aboutTitle}>GOLC — Go Lighting Control</span>
                <span className={styles.aboutMeta}>v1.0 · github.com/lnorton89/golc</span>
              </div>

              <div className={styles.creditsColumns}>
                <div className={styles.creditsColumn}>
                  <span className={styles.creditsLabel}>Open Source — Backend</span>
                  <ul className={styles.creditsList} aria-label="Backend open source credits">
                    {BACKEND_CREDITS.map((credit) => {
                      const key = `backend-${credit.name}`;
                      return (
                        <CreditRow
                          key={key}
                          credit={credit}
                          expanded={expandedCredits.has(key)}
                          onToggle={() => toggleCredit(key)}
                        />
                      );
                    })}
                  </ul>
                </div>

                <div className={styles.creditsColumn}>
                  <span className={styles.creditsLabel}>Open Source — Frontend</span>
                  <ul className={styles.creditsList} aria-label="Frontend open source credits">
                    {FRONTEND_CREDITS.map((credit) => {
                      const key = `frontend-${credit.name}`;
                      return (
                        <CreditRow
                          key={key}
                          credit={credit}
                          expanded={expandedCredits.has(key)}
                          onToggle={() => toggleCredit(key)}
                        />
                      );
                    })}
                  </ul>
                </div>
              </div>
            </div>
          </Panel>
        </div>
      </ScrollRegion>
    </div>
  );
}
