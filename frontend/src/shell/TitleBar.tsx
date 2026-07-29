// TitleBar is this app's self-drawn window chrome, needed because
// cmd/golc-desktop/main.go now runs Frameless: true on every platform: a
// friend running Linux with no desktop environment got no titlebar at all
// (nothing there draws decorations for an undecorated-by-default window
// manager, or for none at all), so instead of leaning on the OS/WM to draw
// one, this app draws its own, identically, on Windows/Linux/macOS.
//
// The drag region is plain Wails convention: any element with the inline
// CSS custom property --wails-draggable: drag becomes a native window-drag
// handle, and the button cluster overrides it back to no-drag so the
// buttons stay clickable. See https://wails.io/docs/guides/frameless.
import { useEffect, useState, type CSSProperties } from "react";
import { Minus, Square, Copy, X } from "lucide-react";

import {
  windowMinimise,
  windowToggleMaximise,
  windowIsMaximised,
  windowClose,
  inspectShow,
} from "../lib/wailsBridge";
import appIcon from "../assets/app-icon.png";
import styles from "./TitleBar.module.css";

const dragStyle = { "--wails-draggable": "drag" } as CSSProperties;
const noDragStyle = { "--wails-draggable": "no-drag" } as CSSProperties;

/** projectNameFromPath strips a show file's directory and .golc extension
 * (e.g. "C:\shows\Fall Tour.golc" -> "Fall Tour"), tolerating both Windows
 * and POSIX separators since showPath comes straight from the Go host,
 * whichever platform it's running on. */
function projectNameFromPath(showPath: string): string {
  const base = showPath.split(/[\\/]/).pop() ?? "";
  return base.replace(/\.golc$/i, "");
}

export default function TitleBar() {
  const [maximised, setMaximised] = useState(false);
  // "(unsaved show)" mirrors OverviewWorkspace.tsx's identical fallback for
  // an unsaved show (offlineShowInspectView's showPath is always "") or a
  // missing bridge -- never a blank center label.
  const [projectName, setProjectName] = useState("(unsaved show)");

  useEffect(() => {
    windowIsMaximised().then(setMaximised);
    inspectShow().then((view) => {
      const name = projectNameFromPath(view.showPath);
      if (name) setProjectName(name);
    });
  }, []);

  const toggleMaximise = () => {
    windowToggleMaximise();
    setMaximised((current) => !current);
  };

  return (
    <div className={styles.titleBar} style={dragStyle} onDoubleClick={toggleMaximise}>
      <div className={styles.brand}>
        <img src={appIcon} alt="" className={styles.brandIcon} />
        <span className={styles.brandLabel}>GOLC</span>
      </div>
      <span className={styles.projectName}>{projectName}</span>
      <div className={styles.controls} style={noDragStyle} onDoubleClick={(event) => event.stopPropagation()}>
        <button
          type="button"
          className={styles.controlButton}
          aria-label="Minimize"
          onClick={windowMinimise}
        >
          <Minus size={14} />
        </button>
        <button
          type="button"
          className={styles.controlButton}
          aria-label={maximised ? "Restore" : "Maximize"}
          onClick={toggleMaximise}
        >
          {maximised ? <Copy size={12} /> : <Square size={12} />}
        </button>
        <button
          type="button"
          className={`${styles.controlButton} ${styles.closeButton}`}
          aria-label="Close"
          onClick={windowClose}
        >
          <X size={14} />
        </button>
      </div>
    </div>
  );
}
