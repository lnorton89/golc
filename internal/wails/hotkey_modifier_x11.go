//go:build linux || openbsd

package wails

import "golang.design/x/hotkey"

// safetyAltModifier is X11's equivalent of the Alt key: Mod1, the
// conventional X11 modifier mask Alt is bound to (golang.design/x/hotkey
// has no ModAlt on linux/openbsd at all -- see
// hotkey_modifier_windows.go's doc comment for the full per-platform
// constant-set explanation). Matches this exact build constraint against
// golang.design/x/hotkey's own hotkey_x11.go, which is what actually
// defines Mod1 -- CGO_ENABLED=0 falls back to hotkey_nocgo.go instead,
// which defines no Modifier constants at all, a pre-existing constraint
// of the upstream package this file does not attempt to work around.
const safetyAltModifier = hotkey.Mod1
