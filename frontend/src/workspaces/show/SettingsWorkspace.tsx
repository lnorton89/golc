// SettingsWorkspace is the Show group's application-preferences workspace.
// Currently holds only Appearance (light/dark/system theme) -- a pure
// client-side preference (lib/theme.ts), never a Go-bound show-state call,
// so unlike OverviewWorkspace/SaveRecoveryWorkspace it does not go through
// wailsBridge.ts at all.
import { useState } from "react";
import {
  Palette,
  Sun,
  Moon,
  Monitor,
  Info,
  Keyboard,
  ChevronRight,
  ChevronDown,
  ExternalLink,
} from "lucide-react";

import {
  getStoredTheme,
  getStoredThemeName,
  setStoredTheme,
  setStoredThemeName,
  type ThemeName,
  type ThemePreference,
} from "../../lib/theme";
import { openExternalURL } from "../../lib/wailsBridge";
import { HOW_IT_WORKS_BY_ID } from "../../shell/navigation";
import { Button, Panel, PanelHeader, RadioGroup, type RadioGroupOption, ScrollRegion, ToggleGroup, type ToggleGroupOption, WorkspaceFrame } from "../../design-system";
import HotkeySettings from "../../components/HotkeySettings/HotkeySettings";
import styles from "./SettingsWorkspace.module.css";

// THEME_OPTIONS backs the Mode row's ToggleGroup -- value is ThemePreference
// narrowed back from ToggleGroup's own plain-string onValueChange in
// handleSelect below (same cast pattern FixtureLibraryWorkspace's own
// source toggle uses), since the shared primitive's contract can't carry a
// caller-specific literal union.
const THEME_OPTIONS: ReadonlyArray<ToggleGroupOption> = [
  { value: "system", label: "Match System", icon: Monitor },
  { value: "light", label: "Light", icon: Sun },
  { value: "dark", label: "Dark", icon: Moon },
];

// Swatch is a decorative preview only (this palette's accent color in its
// dark variant, design-system/tokens.json's own `action.primary` for each
// theme's "-dark" face) -- it does not track the active light/dark mode,
// since the point is to let the operator tell the palettes apart at a
// glance regardless of which mode they're currently in. Hardcoded rather
// than read from the generated CSS: tokens.generated.ts exports role/theme
// *names*, not resolved color values, and this preview is deliberately
// independent of whichever theme is currently applied to :root.
const THEME_NAME_OPTIONS: ReadonlyArray<RadioGroupOption> = [
  { value: "default", label: "Default", swatch: "#1b44d9" },
  { value: "gruvbox", label: "Gruvbox", swatch: "#fe8019" },
  { value: "tokyo", label: "Tokyo Night", swatch: "#7aa2f7" },
  { value: "dracula", label: "Dracula", swatch: "#bd93f9" },
  { value: "nord", label: "Nord", swatch: "#88c0d0" },
  { value: "catppuccin", label: "Catppuccin", swatch: "#cba6f7" },
  { value: "solarized", label: "Solarized", swatch: "#268bd2" },
  { value: "one-dark", label: "One Dark", swatch: "#61afef" },
  { value: "rose-pine", label: "Rosé Pine", swatch: "#c4a7e7" },
  { value: "everforest", label: "Everforest", swatch: "#a7c080" },
  { value: "rainbow", label: "Rainbow", swatch: "#ff2d95" },
  { value: "acid", label: "Acid", swatch: "#c4fd3f" },
];

interface Credit {
  name: string;
  version: string;
  license: string;
  /** What the library itself is/does, in general -- its own README's
   * one-liner, not GOLC-specific. */
  description: string;
  /** The library's own longer self-description (its README/repo "About"
   * blurb, paraphrased but faithful to their wording) -- shown on row
   * expand, alongside `usage` below. */
  longDescription: string;
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
    longDescription:
      "Lets you build desktop applications using Go and web technologies -- wrapping Go code and a web frontend into a single native binary, so Go developers can ship polished desktop apps without a separate server.",
    usage: "Desktop shell: window, native dialogs, OS bridge",
    url: "https://pkg.go.dev/github.com/wailsapp/wails/v2",
  },
  {
    name: "chi",
    version: "5.3.1",
    license: "MIT",
    description: "Lightweight HTTP router",
    longDescription:
      "A lightweight, idiomatic, and composable router for building Go HTTP services, built on Go's standard context package to handle request cancellation and scoped values across handler chains. The core router is under 1,000 lines.",
    usage: "Routes the external /v1 control API",
    url: "https://pkg.go.dev/github.com/go-chi/chi/v5",
  },
  {
    name: "huma",
    version: "2.39.0",
    license: "MIT",
    description: "REST API framework with OpenAPI generation",
    longDescription:
      "A modern, simple, fast, and flexible micro framework for building HTTP REST/RPC APIs in Go, backed by OpenAPI 3.1 and JSON Schema, designed for incremental adoption with automatically generated docs.",
    usage: "The /v1 API's request handling and OpenAPI contract",
    url: "https://pkg.go.dev/github.com/danielgtaylor/huma/v2",
  },
  {
    name: "gomidi/midi",
    version: "2.3.24",
    license: "MIT",
    description: "MIDI I/O and driver bindings",
    longDescription:
      "Helps with reading and writing MIDI messages in Go -- constructing, categorizing, and sending/receiving MIDI data through auto-registering, cross-platform drivers, plus an smf subpackage for Standard MIDI Files.",
    usage: "MIDI Note/CC learn and controller I/O",
    url: "https://pkg.go.dev/gitlab.com/gomidi/midi/v2",
  },
  {
    name: "go-winio",
    version: "0.6.2",
    license: "MIT",
    description: "Windows named pipes and other Win32 I/O primitives",
    longDescription:
      "A Microsoft library of utilities for efficiently performing Win32 IO operations in Go, focused on named pipes and other file handles. It uses IO completion ports so blocking operations don't tie up system threads.",
    usage: "Named-pipe IPC to the supervised Art-Net daemon",
    url: "https://pkg.go.dev/github.com/Microsoft/go-winio",
  },
  {
    name: "cdp",
    version: "0.35.0",
    license: "MIT",
    description: "Chrome DevTools Protocol client",
    longDescription:
      "Type-safe, auto-generated Go bindings for the Chrome DevTools Protocol, built for Chrome/Chromium but compatible with any target implementing the protocol. High-level browser automation is explicitly not a goal -- the focus is a better CDP developer experience.",
    usage: "Drives the Scripts workspace's step debugger",
    url: "https://pkg.go.dev/github.com/mafredri/cdp",
  },
  {
    name: "hotkey",
    version: "0.6.1",
    license: "MIT",
    description: "Cross-platform global hotkey registration",
    longDescription:
      "A cross-platform Go package providing the basic facility to register a system-level global hotkey shortcut, notifying an application when a user triggers it even without window focus. Supports macOS, Linux (X11), and Windows.",
    usage: "Registers the OS-level safety-cluster hotkeys",
    url: "https://pkg.go.dev/golang.design/x/hotkey",
  },
  {
    name: "jsonschema",
    version: "0.14.0",
    license: "MIT",
    description: "Go struct to JSON Schema reflection",
    longDescription:
      "Generates JSON Schemas from Go types via reflection, supporting interfaces, maps, slices, and validation keywords like minLength/pattern/format plus string and numeric enums, targeting JSON Schema Draft 2020-12.",
    usage: "Reflects Go types into the /v1 API's OpenAPI schema",
    url: "https://pkg.go.dev/github.com/invopop/jsonschema",
  },
  {
    name: "toml",
    version: "1.6.0",
    license: "MIT",
    description: "TOML encoding and decoding",
    longDescription:
      "TOML stands for Tom's Obvious, Minimal Language; this package provides a reflection interface similar to the standard library's json and xml packages for encoding and decoding TOML.",
    usage: "Reads and writes GOLC's TOML config files",
    url: "https://pkg.go.dev/github.com/BurntSushi/toml",
  },
  {
    name: "uuid",
    version: "1.6.0",
    license: "BSD-3-Clause",
    description: "UUID generation and parsing",
    longDescription:
      "Generates and inspects UUIDs based on RFC 9562 and DCE 1.1 (Authentication and Security Services), representing UUIDs as 16-byte arrays rather than byte slices.",
    usage: "IDs for pools, deployments, scenes, surfaces",
    url: "https://pkg.go.dev/github.com/google/uuid",
  },
  {
    name: "golang.org/x/sys, x/time",
    version: "0.46.0, 0.15.0",
    license: "BSD-3-Clause",
    description: "Extended Go standard library packages",
    longDescription:
      "x/sys is a Go sub-repository of supplemental packages for low-level OS interaction -- unix and windows primitives, CPU feature detection, and Windows registry/service access. x/time is its companion for supplementary time packages, currently shipping the rate limiter package.",
    usage: "Per-key API rate limiting; Windows registry/pipe access",
    url: "https://pkg.go.dev/golang.org/x",
  },
  {
    name: "modernc.org/sqlite",
    version: "1.54.0",
    license: "BSD-3-Clause",
    description: "cgo-free SQLite driver",
    longDescription:
      "A CGo-free, pure-Go port of SQLite -- an in-process implementation of a self-contained, serverless, zero-configuration, transactional SQL database engine that needs no C compiler or cgo toolchain to build or run.",
    usage: "Durable show storage and recovery points",
    url: "https://pkg.go.dev/modernc.org/sqlite",
  },
  {
    name: "go-yaml",
    version: "4.0.0-rc.6",
    license: "Apache-2.0",
    description: "YAML encoding and decoding",
    longDescription:
      "Enables Go programs to comfortably encode and decode YAML values. It originated at Canonical as part of the Juju project and is a pure Go implementation built on the design of the libyaml C library.",
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
    longDescription:
      "A JavaScript library for building user interfaces. It's declarative, component-based, and \"Learn Once, Write Anywhere\" -- React makes no assumptions about the rest of your technology stack.",
    usage: "Renders every workspace and shell surface",
    url: "https://www.npmjs.com/package/react",
  },
  {
    name: "Zustand",
    version: "5.0.14",
    license: "MIT",
    description: "Client-side state management",
    longDescription:
      "A small, fast, and scalable \"barebones\" state-management solution for React using simplified Flux principles, with a comfortable hooks-based API that isn't boilerplate-heavy or opinionated.",
    usage: "Caches Go-pushed live status (playback, safety)",
    url: "https://www.npmjs.com/package/zustand",
  },
  {
    name: "Monaco Editor",
    version: "0.55.1",
    license: "MIT",
    description: "Code editor component",
    longDescription:
      "The code editor that powers VS Code, packaged as a browser-based editor component. It's built directly from VS Code's source, so it shares much of VS Code's editing feature set.",
    usage: "Powers the Scripts workspace's code editor",
    url: "https://www.npmjs.com/package/monaco-editor",
  },
  {
    name: "Base UI",
    version: "1.7.0",
    license: "MIT",
    description: "Headless React component primitives",
    longDescription:
      "A library of headless (\"unstyled\") React components and low-level hooks, built by the Radix and Floating UI teams, giving complete control over an app's own CSS and accessibility features.",
    usage: "Unstyled behavior underneath every design-system primitive (Dialog, Select, Menu, Slider, etc.)",
    url: "https://www.npmjs.com/package/@base-ui/react",
  },
  {
    name: "dnd kit",
    version: "6.3.1, 10.0.0",
    license: "MIT",
    description: "Drag-and-drop toolkit",
    longDescription:
      "A lightweight, modular, performant, accessible, and extensible React library for building drag-and-drop interfaces, with a sortable preset built on top of its core sensors and collision detection.",
    usage: "Drag-to-reorder for the Scene Stack's scene list",
    url: "https://www.npmjs.com/package/@dnd-kit/core",
  },
  {
    name: "Tiptap",
    version: "3.29.2",
    license: "MIT",
    description: "Headless rich-text editor framework",
    longDescription:
      "A headless, framework-agnostic rich-text editor built on ProseMirror, providing a starter kit of common extensions (bold, lists, etc.) plus additional task-list/task-item extensions for structured checklists.",
    usage: "Powers the Notes workspace's rich-text editor",
    url: "https://www.npmjs.com/package/@tiptap/core",
  },
  {
    name: "Motion",
    version: "13.0.0",
    license: "MIT",
    description: "Animation library",
    longDescription:
      "An animation library for JavaScript and React (the successor to Framer Motion), combining a hardware-accelerated engine with a small bundle size for spring physics, gestures, and layout animations.",
    usage: "Shared motion tokens and transitions across the shell and workspaces",
    url: "https://www.npmjs.com/package/motion",
  },
  {
    name: "react-colorful",
    version: "5.8.0",
    license: "MIT",
    description: "Color picker component",
    longDescription:
      "A tiny, dependency-free color picker component for React and Preact apps -- fast, well-tested, and mobile-friendly, supporting RGB(A), HSL(A), HSV(A), and HEX(A) formats.",
    usage: "The color-swatch field's RGB picker",
    url: "https://www.npmjs.com/package/react-colorful",
  },
  {
    name: "Lucide",
    version: "1.27.0",
    license: "ISC",
    description: "Icon set",
    longDescription:
      "An open-source icon library providing 1,600+ vector icons for digital and non-digital projects, billed as a \"beautiful & consistent icon toolkit made by the community.\" It started as a community-driven fork of Feather Icons.",
    usage: "Icon set used across the whole shell",
    url: "https://www.npmjs.com/package/lucide-react",
  },
  {
    name: "Archivo",
    version: "5.3.0",
    license: "OFL-1.1",
    description: "Interface typeface",
    longDescription:
      "A grotesque sans-serif typeface family with roots in late-19th-century American type design, originally created for highlights and headlines and optimized for print and digital use across 200+ languages. It became a variable font in 2021.",
    usage: "Interface typeface used throughout the shell",
    url: "https://www.npmjs.com/package/@fontsource/archivo",
  },
  {
    name: "JetBrains Mono",
    version: "5.3.0",
    license: "OFL-1.1",
    description: "Monospace typeface",
    longDescription:
      "A free and open-source typeface engineered specifically for programming: it maximizes lowercase letter height while keeping standard character width, includes code-specific ligatures, and ships in multiple weights with matching italics.",
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
      <Button
        variant="secondary"
        className={styles.creditButton}
        aria-expanded={expanded}
        onClick={onToggle}
      >
        <ChevronIcon size={14} className={styles.creditChevron} aria-hidden="true" />
        <span className={styles.creditName}>{credit.name}</span>
        <span className={styles.creditDescription}>{credit.description}</span>
        <span className={styles.creditMeta}>
          <span className={styles.creditLicense}>{credit.license}</span>
          <span className={styles.creditSep} aria-hidden="true">
            ·
          </span>
          <span className={styles.creditVersion}>v{credit.version}</span>
        </span>
      </Button>

      {expanded ? (
        <div className={styles.creditDetail}>
          <p className={styles.creditLongDescription}>{credit.longDescription}</p>
          <p className={styles.creditUsage}>
            <span className={styles.creditUsageLabel}>In GOLC:</span> {credit.usage}
          </p>
          <Button variant="secondary" icon={ExternalLink} onClick={() => void openExternalURL(credit.url)}>
            View project
          </Button>
        </div>
      ) : null}
    </li>
  );
}

export default function SettingsWorkspace() {
  const [theme, setTheme] = useState<ThemePreference>(() => getStoredTheme());
  const [themeName, setThemeName] = useState<ThemeName>(() => getStoredThemeName());
  const [expandedCredits, setExpandedCredits] = useState<Set<string>>(() => new Set());
  const [creditsExpanded, setCreditsExpanded] = useState(false);

  const handleSelect = (next: ThemePreference) => {
    setStoredTheme(next);
    setTheme(next);
  };

  const handleSelectThemeName = (next: ThemeName) => {
    setStoredThemeName(next);
    setThemeName(next);
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
    <WorkspaceFrame
      title="Settings"
      info={HOW_IT_WORKS_BY_ID["show-settings"]}
    >
      <ScrollRegion className={styles.canvas}>
        <div className={styles.layout}>
          <Panel className={styles.appearancePanel}>
            <PanelHeader
              label="Appearance"
              icon={Palette}
              info="Switches the desktop app between light, dark, and system mode, and picks the color palette."
            />
            <span className={styles.groupLabel}>Mode</span>
            <div className={styles.themeRow}>
              <ToggleGroup
                aria-label="Mode"
                options={THEME_OPTIONS}
                value={theme}
                onValueChange={(next) => handleSelect(next as ThemePreference)}
              />
            </div>
            <span className={styles.groupLabel}>Theme</span>
            <div className={styles.themeRow}>
              <RadioGroup
                label="Theme"
                hideLabel
                layout="wrap"
                options={THEME_NAME_OPTIONS}
                value={themeName}
                onValueChange={(next) => handleSelectThemeName(next as ThemeName)}
              />
            </div>
          </Panel>

          <Panel className={styles.hotkeysPanel}>
            <PanelHeader label="Hotkeys" icon={Keyboard} info="Lists every keyboard shortcut currently bound in the app." />
            <HotkeySettings />
          </Panel>

          <Panel className={styles.aboutPanel}>
            <PanelHeader
              label="About"
              icon={Info}
              info="Shows the GOLC version, the Go toolchain it's built with, a short description of the app, and license credits for the open-source libraries it depends on."
            />
            <div className={styles.about}>
              <div className={styles.aboutHeading}>
                <span className={styles.aboutTitle}>GOLC — Go Lighting Control</span>
                <span className={styles.aboutMeta}>v1.0 · Go 1.26.5 · github.com/lnorton89/golc</span>
              </div>
              <p className={styles.aboutDescription}>
                A modern lighting-control application for club/DJ operators running small live
                shows, built in Go with a Wails desktop interface.
              </p>

              <Button
                variant="secondary"
                aria-expanded={creditsExpanded}
                aria-controls="settings-about-credits"
                onClick={() => setCreditsExpanded((current) => !current)}
              >
                {creditsExpanded ? (
                  <ChevronDown size={14} className={styles.creditsToggleChevron} aria-hidden="true" />
                ) : (
                  <ChevronRight size={14} className={styles.creditsToggleChevron} aria-hidden="true" />
                )}
                Open Source Libraries
              </Button>

              {creditsExpanded ? (
                <div className={styles.creditsColumns} id="settings-about-credits">
                  <div className={styles.creditsColumn}>
                    <span className={styles.creditsLabel}>Backend</span>
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
                    <span className={styles.creditsLabel}>Frontend</span>
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
              ) : null}
            </div>
          </Panel>
        </div>
      </ScrollRegion>
    </WorkspaceFrame>
  );
}
