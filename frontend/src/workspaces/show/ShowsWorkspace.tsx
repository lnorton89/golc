// ShowsWorkspace is the Show group's open/new/switch workspace (09-02-
// PLAN.md, FDUI-02): displays the current show path and offers
// "Open Show…"/"New Show…" actions that pick a *.golc path via a native OS
// file picker and then perform a supervised self-relaunch of
// golc-desktop.exe bound to that path (D-05) -- never an in-process show
// swap. "New Show…" uses the identical mechanism pointed at a path that
// does not exist yet: there is no separate new-show setup flow and no
// second backend concept (D-06). This closes the gap
// SaveRecoveryWorkspace.tsx previously documented: that workspace no longer
// claims there is no way to open a different show from the desktop app.
import { useCallback, useEffect, useState } from "react";
import { FolderOpen, FilePlus } from "lucide-react";

import {
  errorMessage,
  inspectShow,
  pickNewShowPath,
  pickShowPath,
  relaunchWithShow,
} from "../../lib/wailsBridge";
import { HOW_IT_WORKS_BY_ID } from "../../shell/navigation";
import { Button, ErrorState, LoadingState, Panel, PanelHeader, WorkspaceFrame } from "../../design-system";
import styles from "./ShowsWorkspace.module.css";

export default function ShowsWorkspace() {
  const [showPath, setShowPath] = useState("");
  const [loading, setLoading] = useState(true);
  const [switching, setSwitching] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async (): Promise<void> => {
    try {
      const view = await inspectShow();
      setShowPath(view.showPath);
      setError(null);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  // switchTo saves the working show, starts a replacement process bound to
  // path, and quits this process -- purely client-side "switching" state
  // tied to the call promise, never a subscribed runtime event: the
  // process is about to exit and no backend progress signal exists. On a
  // successful (zero exit code) relaunch, switching deliberately stays
  // true -- the transient below is the last thing this workspace ever
  // renders before this process exits.
  const switchTo = async (path: string): Promise<void> => {
    setSwitching(true);
    setError(null);
    try {
      const result = await relaunchWithShow(path);
      if (result.exitCode !== 0) {
        setError("Couldn't switch to this show. GOLC is still running the previous show.");
        setSwitching(false);
      }
    } catch (err) {
      setError(errorMessage(err));
      setSwitching(false);
    }
  };

  const handleOpen = async (): Promise<void> => {
    const path = await pickShowPath();
    if (path === "") {
      // Cancelling the picker is a silent no-op -- no state change, no
      // error, no relaunch call.
      return;
    }
    await switchTo(path);
  };

  const handleNew = async (): Promise<void> => {
    const path = await pickNewShowPath();
    if (path === "") {
      return;
    }
    await switchTo(path);
  };

  return (
    <WorkspaceFrame
      title="Shows"
      info={HOW_IT_WORKS_BY_ID["show-shows"]}
    >
      <div className={styles.canvas}>
        {loading ? (
          <LoadingState label="Loading show…" variant="panel" />
        ) : (
          <>
            {error ? <ErrorState heading="Show unavailable" message={error} /> : null}
            {switching ? (
              <p className={styles.switchingText}>Switching shows — GOLC will reload in a moment…</p>
            ) : null}
            <div className={styles.layout}>
              <Panel>
                <PanelHeader
                  label="Current Show"
                  icon={FolderOpen}
                  info="Shows the path of whichever show file is open, or lets you open or create one if none is."
                />
                <div className={styles.currentShow}>
                  {showPath ? (
                    <span className={styles.path} title={showPath}>
                      {showPath}
                    </span>
                  ) : (
                    <p className={styles.noShowCopy}>Choose a show file to open, or create a new one.</p>
                  )}
                </div>
                <div className={styles.actionRow}>
                  <Button
                    variant="primary"
                    icon={FolderOpen}
                    disabled={switching}
                    onClick={() => void handleOpen()}
                  >
                    Open Show…
                  </Button>
                  <Button
                    variant="primary"
                    icon={FilePlus}
                    disabled={switching}
                    onClick={() => void handleNew()}
                  >
                    New Show…
                  </Button>
                </div>
              </Panel>
            </div>
          </>
        )}
      </div>
    </WorkspaceFrame>
  );
}
