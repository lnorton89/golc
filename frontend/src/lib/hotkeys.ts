// hotkeys.ts is the single source of truth for the app's rebindable
// playback hotkeys (Settings > Hotkeys). Both useKeyboardWorkflow.ts's
// keydown handler and KeyboardShortcuts.tsx's reference panel resolve a
// shortcut's effective key through here, so a custom binding can never
// drift between "what fires" and "what's documented" -- the same
// single-source-of-truth contract useKeyboardWorkflow.ts's own doc comment
// describes for PLAYBACK_SHORTCUTS, now backed by a persisted override
// instead of a fixed constant.
//
// Bindings are a pure client-side preference (same rationale as
// lib/theme.ts): persisted to localStorage, never round-tripped through
// wailsBridge.ts -- nothing on the Go side needs to know the operator's
// key choices.
//
// The digit scene-switch shortcuts (1-9) are intentionally NOT part of
// this rebindable set: they're an ordinal mapping over however many
// scenes the open show has, not a single action with a single key, so
// "rebinding" one digit doesn't have a coherent meaning here. They stay
// fixed (SCENE_SWITCH_SHORTCUT below) and digits are reserved so a custom
// binding can never shadow them.

const STORAGE_KEY = "golc-hotkeys";
const CHANGE_EVENT = "golc-hotkeys-changed";

export type HotkeyActionId =
  | "toggleBaseLook"
  | "toggleColorTheme"
  | "toggleChase"
  | "toggleMotion"
  | "tapTempo"
  | "bpmUp"
  | "bpmDown"
  | "evaluate";

export interface HotkeyActionDef {
  id: HotkeyActionId;
  category: string;
  description: string;
  /** Canonical binding string in the same shape normalizeHotkeyEvent
   * produces -- lowercase single characters, or the named key ("Space",
   * "ArrowUp", "Enter", ...) for everything else. */
  defaultKey: string;
}

export const HOTKEY_ACTIONS: HotkeyActionDef[] = [
  { id: "toggleBaseLook", category: "Layers", description: "Toggle Base Look on the active scene", defaultKey: "q" },
  { id: "toggleColorTheme", category: "Layers", description: "Toggle Color Theme on the active scene", defaultKey: "w" },
  { id: "toggleChase", category: "Layers", description: "Toggle Chase on the active scene", defaultKey: "e" },
  { id: "toggleMotion", category: "Layers", description: "Toggle Motion on the active scene", defaultKey: "r" },
  { id: "tapTempo", category: "Tempo", description: "Tap tempo (accumulates with prior taps within 2s)", defaultKey: "Space" },
  { id: "bpmUp", category: "Tempo", description: "Nudge BPM up by 1", defaultKey: "ArrowUp" },
  { id: "bpmDown", category: "Tempo", description: "Nudge BPM down by 1", defaultKey: "ArrowDown" },
  { id: "evaluate", category: "Transport", description: "Evaluate/preview the active scene at bar 0", defaultKey: "Enter" },
];

/** The fixed, non-rebindable scene-switch shortcut -- shown alongside the
 * rebindable HOTKEY_ACTIONS in both reference panels but never editable. */
export const SCENE_SWITCH_SHORTCUT = {
  category: "Scenes",
  keys: "1 – 9",
  description: "Switch to the Nth scene in the current show",
};

export type HotkeyBindings = Record<HotkeyActionId, string>;

function defaultBindings(): HotkeyBindings {
  const out = {} as HotkeyBindings;
  for (const action of HOTKEY_ACTIONS) {
    out[action.id] = action.defaultKey;
  }
  return out;
}

function persist(bindings: HotkeyBindings): void {
  window.localStorage.setItem(STORAGE_KEY, JSON.stringify(bindings));
  window.dispatchEvent(new Event(CHANGE_EVENT));
}

export function getStoredHotkeys(): HotkeyBindings {
  const defaults = defaultBindings();
  const raw = window.localStorage.getItem(STORAGE_KEY);
  if (!raw) {
    return defaults;
  }
  try {
    const parsed = JSON.parse(raw) as Partial<Record<string, unknown>>;
    const merged = { ...defaults };
    for (const action of HOTKEY_ACTIONS) {
      const value = parsed[action.id];
      if (typeof value === "string" && value.length > 0) {
        merged[action.id] = value;
      }
    }
    return merged;
  } catch {
    return defaults;
  }
}

export function setHotkeyBinding(id: HotkeyActionId, key: string): void {
  persist({ ...getStoredHotkeys(), [id]: key });
}

export function resetHotkeyBinding(id: HotkeyActionId): void {
  const action = HOTKEY_ACTIONS.find((entry) => entry.id === id);
  if (!action) {
    return;
  }
  persist({ ...getStoredHotkeys(), [id]: action.defaultKey });
}

/** resetAllHotkeys clears both the playback (bare-key) and navigation
 * (chorded) binding stores -- "Reset all to defaults" in Settings >
 * Hotkeys means all of it, not just one namespace. */
export function resetAllHotkeys(): void {
  window.localStorage.removeItem(STORAGE_KEY);
  window.localStorage.removeItem(NAV_STORAGE_KEY);
  window.dispatchEvent(new Event(CHANGE_EVENT));
}

/** onHotkeysChanged notifies listeners after any set/reset in this window
 * (CHANGE_EVENT) and after a binding change made in another window/tab
 * ("storage") -- returns the unsubscribe function. */
export function onHotkeysChanged(listener: () => void): () => void {
  window.addEventListener(CHANGE_EVENT, listener);
  window.addEventListener("storage", listener);
  return () => {
    window.removeEventListener(CHANGE_EVENT, listener);
    window.removeEventListener("storage", listener);
  };
}

/** normalizeHotkeyEvent extracts the canonical binding string both
 * useKeyboardWorkflow.ts's matcher and the Settings rebind capture use --
 * Space is matched on event.code (layout-independent), everything else on
 * event.key so a custom binding can use any printable character or named
 * key the browser reports. */
export function normalizeHotkeyEvent(event: KeyboardEvent): string {
  if (event.code === "Space") {
    return "Space";
  }
  if (event.key.length === 1) {
    return event.key.toLowerCase();
  }
  return event.key;
}

const KEY_LABELS: Record<string, string> = {
  ArrowUp: "↑",
  ArrowDown: "↓",
  ArrowLeft: "←",
  ArrowRight: "→",
  " ": "Space",
};

/** formatHotkeyLabel renders a canonical binding string for display --
 * used by both the Settings key button and the read-only reference
 * panels, so a custom binding always displays identically everywhere. */
export function formatHotkeyLabel(key: string): string {
  if (key in KEY_LABELS) {
    return KEY_LABELS[key];
  }
  return key.length === 1 ? key.toUpperCase() : key;
}

/** findHotkeyConflict reports why `key` cannot be bound to `id`: another
 * rebindable action already using it, or the fixed 1-9 scene-switch range.
 * Returns null when the key is free. */
export function findHotkeyConflict(
  bindings: HotkeyBindings,
  id: HotkeyActionId,
  key: string,
): HotkeyActionId | "scene-switch" | null {
  if (/^[1-9]$/.test(key)) {
    return "scene-switch";
  }
  const clash = HOTKEY_ACTIONS.find((action) => action.id !== id && bindings[action.id] === key);
  return clash ? clash.id : null;
}

const MODIFIER_KEY_NAMES = new Set(["Control", "Alt", "Shift", "Meta"]);

/** isModifierKeyEvent reports whether `event.key` is itself a modifier
 * (Ctrl/Alt/Shift/Meta) rather than a real key -- both capture flows below
 * use this to keep waiting instead of recording the modifier press as if
 * it were the binding. */
export function isModifierKeyEvent(event: KeyboardEvent): boolean {
  return MODIFIER_KEY_NAMES.has(event.key);
}

// --- Navigation (chorded) shortcuts -----------------------------------
//
// NAV_ACTIONS covers useGlobalKeyboardWorkflow.ts's menu-navigation
// shortcuts (Discord's Alt+Up/Down channel-nav and Ctrl+Alt+Up/Down
// server-nav conventions, plus a Ctrl+K quick switcher and a Ctrl+, jump-
// to-Settings). Every one of these needs a modifier chord, which is a
// structurally different binding shape than HOTKEY_ACTIONS' bare single
// key -- normalizeHotkeyChord/formatChordLabel/findChordConflict below are
// the chord equivalents of normalizeHotkeyEvent/formatHotkeyLabel/
// findHotkeyConflict above, and NAV_ACTIONS gets its own localStorage key
// (NAV_STORAGE_KEY) and get/set/reset trio so a chord accidentally never
// collides with a bare-key playback binding's storage. Both namespaces
// still share CHANGE_EVENT, so one onHotkeysChanged subscription sees
// changes to either.
//
// The digit scene-switch shortcuts (1-9) stay fixed for the same reason
// HOTKEY_ACTIONS' own doc comment gives: an ordinal range over however
// many scenes the show has isn't a single action with a single binding.

const NAV_STORAGE_KEY = "golc-nav-hotkeys";

export type NavActionId =
  | "navPrevInGroup"
  | "navNextInGroup"
  | "navPrevGroup"
  | "navNextGroup"
  | "openQuickSwitcher"
  | "openSettings";

export interface NavActionDef {
  id: NavActionId;
  category: string;
  description: string;
  /** Canonical chord string in the same shape normalizeHotkeyChord
   * produces: modifier names (Ctrl/Alt/Shift, in that fixed order) joined
   * with the normalizeHotkeyEvent-shaped base key by "|". */
  defaultChord: string;
}

export const NAV_ACTIONS: NavActionDef[] = [
  {
    id: "navPrevInGroup",
    category: "Navigation",
    description: "Move to the previous destination in this nav group",
    defaultChord: "Alt|ArrowUp",
  },
  {
    id: "navNextInGroup",
    category: "Navigation",
    description: "Move to the next destination in this nav group",
    defaultChord: "Alt|ArrowDown",
  },
  {
    id: "navPrevGroup",
    category: "Navigation",
    description: "Jump to the previous nav group",
    defaultChord: "Ctrl|Alt|ArrowUp",
  },
  {
    id: "navNextGroup",
    category: "Navigation",
    description: "Jump to the next nav group",
    defaultChord: "Ctrl|Alt|ArrowDown",
  },
  {
    id: "openQuickSwitcher",
    category: "Navigation",
    description: "Open the quick switcher to jump to any workspace",
    defaultChord: "Ctrl|k",
  },
  {
    id: "openSettings",
    category: "Navigation",
    description: "Jump straight to Settings",
    defaultChord: "Ctrl|,",
  },
];

export type NavHotkeyBindings = Record<NavActionId, string>;

function defaultNavBindings(): NavHotkeyBindings {
  const out = {} as NavHotkeyBindings;
  for (const action of NAV_ACTIONS) {
    out[action.id] = action.defaultChord;
  }
  return out;
}

function persistNav(bindings: NavHotkeyBindings): void {
  window.localStorage.setItem(NAV_STORAGE_KEY, JSON.stringify(bindings));
  window.dispatchEvent(new Event(CHANGE_EVENT));
}

export function getStoredNavHotkeys(): NavHotkeyBindings {
  const defaults = defaultNavBindings();
  const raw = window.localStorage.getItem(NAV_STORAGE_KEY);
  if (!raw) {
    return defaults;
  }
  try {
    const parsed = JSON.parse(raw) as Partial<Record<string, unknown>>;
    const merged = { ...defaults };
    for (const action of NAV_ACTIONS) {
      const value = parsed[action.id];
      if (typeof value === "string" && value.length > 0) {
        merged[action.id] = value;
      }
    }
    return merged;
  } catch {
    return defaults;
  }
}

export function setNavHotkeyBinding(id: NavActionId, chord: string): void {
  persistNav({ ...getStoredNavHotkeys(), [id]: chord });
}

export function resetNavHotkeyBinding(id: NavActionId): void {
  const action = NAV_ACTIONS.find((entry) => entry.id === id);
  if (!action) {
    return;
  }
  persistNav({ ...getStoredNavHotkeys(), [id]: action.defaultChord });
}

/** normalizeHotkeyChord extracts the canonical chord string for a
 * modifier-bearing shortcut: "Ctrl"/"Alt"/"Shift" (in that fixed order,
 * regardless of physical press order) plus the normalizeHotkeyEvent-shaped
 * base key, joined by "|" (not "+" -- so a chord bound to the literal "+"
 * key can never collide with the join separator itself). Returns null
 * while only a modifier is held (event.key itself is Ctrl/Alt/Shift/Meta)
 * -- the caller should keep waiting for the following keydown rather than
 * treat the bare modifier press as a complete chord. */
export function normalizeHotkeyChord(event: KeyboardEvent): string | null {
  if (isModifierKeyEvent(event)) {
    return null;
  }
  const parts: string[] = [];
  if (event.ctrlKey) parts.push("Ctrl");
  if (event.altKey) parts.push("Alt");
  if (event.shiftKey) parts.push("Shift");
  parts.push(normalizeHotkeyEvent(event));
  return parts.join("|");
}

/** formatChordLabel renders a canonical chord string for display, reusing
 * formatHotkeyLabel for the trailing base-key segment (so ArrowUp/Space/a
 * single letter render identically to the bare-key playback shortcuts). */
export function formatChordLabel(chord: string): string {
  const segments = chord.split("|");
  const keySegment = segments[segments.length - 1];
  const modifierSegments = segments.slice(0, -1);
  return [...modifierSegments, formatHotkeyLabel(keySegment)].join(" + ");
}

/** findChordConflict reports which other nav action already owns `chord`,
 * or null when it's free. Chords always carry a modifier (enforced by the
 * Settings capture UI, not here), so unlike findHotkeyConflict there's no
 * separate reserved-range check: a modifier chord can never collide with
 * the bare 1-9 scene-switch keys or a bare-key playback binding --
 * useKeyboardWorkflow.ts's own matcher ignores any event carrying a
 * modifier. */
export function findChordConflict(bindings: NavHotkeyBindings, id: NavActionId, chord: string): NavActionId | null {
  const clash = NAV_ACTIONS.find((action) => action.id !== id && bindings[action.id] === chord);
  return clash ? clash.id : null;
}
