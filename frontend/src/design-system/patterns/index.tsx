import type { ReactNode } from "react";

import Button from "../../components/primitives/Button/Button";
import Chip, { type ChipTone } from "../../components/primitives/Chip/Chip";
import EmptyState from "../../components/primitives/EmptyState/EmptyState";
import ErrorState from "../../components/primitives/ErrorState/ErrorState";
import ListRow from "../../components/primitives/ListRow/ListRow";
import LoadingState from "../../components/primitives/LoadingState/LoadingState";
import Panel from "../../components/primitives/Panel/Panel";
import Toolbar from "../../components/primitives/Toolbar/Toolbar";
import styles from "./patterns.module.css";

export function WorkspaceFrame({ title, action, children }: { title: string; action?: ReactNode; children: ReactNode }) {
  return <section className={styles.workspaceFrame} aria-label={`${title} workspace`}><Toolbar title={title} action={action} />{children}</section>;
}

export function SplitPane({ primary, secondary }: { primary: ReactNode; secondary: ReactNode }) {
  return <div className={styles.splitPane}><div>{primary}</div><aside>{secondary}</aside></div>;
}

export type DataListProps = { label: string; items: ReadonlyArray<{ id: string; label: string; meta?: ReactNode; selected?: boolean; disabled?: boolean }>; busy?: boolean; error?: string; emptyHeading?: string; emptyBody?: string; onSelect?: (id: string) => void };
export function DataList({ label, items, busy = false, error, emptyHeading = "Nothing here yet", emptyBody = "Create an item to begin.", onSelect }: DataListProps) {
  if (busy) return <LoadingState label={`${label} is loading`} variant="panel" />;
  if (error) return <ErrorState heading={`${label} unavailable`} message={error} variant="panel" />;
  if (items.length === 0) return <EmptyState heading={emptyHeading} body={emptyBody} />;
  return <div className={styles.dataList} aria-label={label}>{items.map((item) => <ListRow key={item.id} label={item.label} meta={item.meta} selected={item.selected} disabled={item.disabled} onSelect={onSelect ? () => onSelect(item.id) : undefined} />)}</div>;
}

export function FormActions({ children }: { children: ReactNode }) { return <div className={styles.formActions}>{children}</div>; }

export function ImpactReview({ summary, impacts, status = "armed", children }: { summary: string; impacts: ReadonlyArray<string>; status?: ChipTone; children?: ReactNode }) {
  return <Panel variant="warning"><div className={styles.impactReview}><Chip tone={status}>Review required</Chip><p className={styles.impactSummary}>{summary}</p><ul>{impacts.map((impact) => <li key={impact}>{impact}</li>)}</ul>{children}</div></Panel>;
}

export function GuidedFlow({ steps }: { steps: ReadonlyArray<{ title: string; description: string; state?: "current" | "complete" | "upcoming" }> }) {
  return <ol className={styles.guidedFlow}>{steps.map((step, index) => <li className={styles.guidedStep} key={step.title}><span className={styles.guidedNumber} aria-label={`Step ${index + 1}`}>{index + 1}</span><div><strong>{step.title}</strong><p>{step.description}</p>{step.state === "current" ? <Chip tone="armed">Current</Chip> : null}</div></li>)}</ol>;
}

export function SceneStack({ scenes }: { scenes: ReadonlyArray<{ id: string; name: string; status: ChipTone }> }) {
  return <div className={styles.sceneStack} aria-label="Scene stack">{scenes.map((scene) => <ListRow key={scene.id} label={scene.name} meta={<Chip tone={scene.status}>{scene.status}</Chip>} />)}</div>;
}

export function LauncherMasters({ masters }: { masters: ReadonlyArray<{ id: string; name: string; value: string; disabled?: boolean }> }) {
  return <div className={styles.masterList} aria-label="Launcher masters">{masters.map((master) => <div className={styles.masterRow} key={master.id}><span>{master.name}</span><Button size="compact" disabled={master.disabled}>{master.value}</Button></div>)}</div>;
}

export function MidiPickup({ value, target, pickedUp }: { value: number; target: number; pickedUp: boolean }) {
  return <div className={styles.midiPickup} role="status">MIDI {pickedUp ? "picked up" : "waiting"}: control {value}, target {target}.</div>;
}

export function SafetyAction({ label, description, disabled = false, onInvoke }: { label: string; description: string; disabled?: boolean; onInvoke?: () => void }) {
  return <div className={styles.safetyAction}><Button variant="destructive" size="target" disabled={disabled} onClick={onInvoke}>{label}</Button><span className={styles.safetyDescription}>{description}</span></div>;
}
