//go:build darwin

package wails

import "golang.design/x/hotkey"

// safetyAltModifier is macOS's equivalent of the Alt key: the Option key
// (golang.design/x/hotkey has no ModAlt on darwin at all -- see
// hotkey_modifier_windows.go's doc comment for the full per-platform
// constant-set explanation).
const safetyAltModifier = hotkey.ModOption
