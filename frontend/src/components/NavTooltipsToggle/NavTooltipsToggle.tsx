// NavTooltipsToggle is the global on/off switch for suppressible hover
// text (lib/navTooltips.ts) -- CommandRail's nav destination buttons show
// their own description on hover/focus (HoverTooltip's `suppressible`
// option), which some operators find intrusive once they already know the
// rail by heart. This does not affect InfoTooltip's own "i" icons -- those
// exist solely to be hovered for more detail and are never suppressed.
//
// Rendered in GlobalFrame.tsx immediately after MidiLearnToggle. Unlike
// MidiLearnToggle (a raw <button>, registered DS005 exception, because it
// needs a soft-disabled contract no shared primitive supports), this
// toggle has no disabled state at all, so it composes the shared Button
// primitive directly -- same pattern SafetyCluster.tsx already uses for
// its own data-active-styled toggle controls.
import { MessageCircle } from "lucide-react";

import Button from "../primitives/Button/Button";
import { useNavTooltipsEnabled } from "../../hooks/useNavTooltipsEnabled";
import { setStoredNavTooltipsEnabled } from "../../lib/navTooltips";
import styles from "./NavTooltipsToggle.module.css";

export default function NavTooltipsToggle() {
  const enabled = useNavTooltipsEnabled();

  return (
    <Button
      type="button"
      variant="secondary"
      size="compact"
      className={styles.toggle}
      leadingIcon={MessageCircle}
      data-active={enabled ? true : undefined}
      aria-pressed={enabled}
      aria-label={enabled ? "Turn off navigation hover text" : "Turn on navigation hover text"}
      title={
        enabled
          ? "Navigation hover text is on -- click to stop menu items from showing a description on hover"
          : "Navigation hover text is off -- click to show a description when hovering menu items"
      }
      onClick={() => setStoredNavTooltipsEnabled(!enabled)}
    >
      <span className={styles.label}>Nav Hints</span>
    </Button>
  );
}
