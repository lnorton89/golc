// ComingSoon is the shared stub for nav destinations with no frontend/
// Wails binding yet. Calm tone (design-system EmptyState defaults), not an
// error/warning color -- this is expected, not broken.
//
// Composed from the same WorkspaceFrame + bounded ScrollRegion + EmptyState
// contract every migrated *Workspace.tsx now uses (see SettingsWorkspace.tsx,
// NotesWorkspace.tsx, etc.) rather than a bespoke Toolbar/Panel pairing, so a
// future destination that graduates from this stub inherits the same shell
// shape its replacement workspace will already be using.
import { EmptyState, ScrollRegion, WorkspaceFrame } from "../design-system";
import styles from "./ComingSoon.module.css";

interface ComingSoonProps {
  title: string;
  description: string;
  cliHint: string;
}

export default function ComingSoon({ title, description, cliHint }: ComingSoonProps) {
  return (
    <WorkspaceFrame title={title}>
      <ScrollRegion className={styles.canvas}>
        <EmptyState heading={description} body={cliHint} />
      </ScrollRegion>
    </WorkspaceFrame>
  );
}
