// TitleBarControlButton is the shared minimize/maximize/close control,
// extracted to its own file (CommandRailGroupToggle.tsx precedent, see
// design-system/exceptions.json) so TitleBar.tsx itself carries no raw
// <button> tag -- three raw <button> elements in one file always produce
// byte-identical DS005 diagnostic values ("button"), which the checker's
// exception mechanism can only resolve to exactly one diagnostic per
// rule+path. This one file, used three times, is the only DS005 finding.
import type { ComponentType } from "react";

interface TitleBarControlButtonProps {
  icon: ComponentType<{ size?: number }>;
  size: number;
  label: string;
  className: string;
  onClick: () => void;
}

export default function TitleBarControlButton({ icon: Icon, size, label, className, onClick }: TitleBarControlButtonProps) {
  return (
    <button type="button" className={className} aria-label={label} onClick={onClick}>
      <Icon size={size} />
    </button>
  );
}
