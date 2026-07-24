//go:build windows

package wails

import "golang.design/x/hotkey"

// safetyAltModifier is the platform's equivalent of the Alt key in the
// three D-16 safety-cluster shortcuts' fixed Ctrl+Alt+Shift combination.
// golang.design/x/hotkey declares an entirely distinct Modifier constant
// set per platform (windows: ModAlt/ModCtrl/ModShift/ModWin; darwin:
// ModCtrl/ModShift/ModOption/ModCmd; X11: ModCtrl/ModShift/Mod1..Mod5),
// so hotkey.go cannot reference hotkey.ModAlt directly without breaking
// every non-Windows build (observed live: cross-platform-mage.yml run
// 30077193060 failed compiling internal/wails on macos-latest with
// "undefined: hotkey.ModAlt" -- X11 would fail identically).
const safetyAltModifier = hotkey.ModAlt
