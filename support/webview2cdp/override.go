// Package webview2cdp contains the pure configuration seam used by the
// packaged WebView2 dialog proof's version-locked Go build overlay.
package webview2cdp

import (
	"strconv"
	"strings"
)

const (
	PortEnvironment     = "GOLC_DIALOG_PROOF_CDP_PORT"
	UserDataEnvironment = "GOLC_DIALOG_PROOF_USER_DATA_FOLDER"
)

// Resolve returns the WebView2 environment inputs for the packaged proof.
// Without a valid proof port it preserves Wails' values exactly.
func Resolve(getenv func(string) string, browserArguments, userDataFolder string) (string, string) {
	rawPort := strings.TrimSpace(getenv(PortEnvironment))
	port, err := strconv.ParseUint(rawPort, 10, 16)
	if err != nil || port < 1024 {
		return browserArguments, userDataFolder
	}

	remoteDebugging := "--remote-debugging-port=" + strconv.FormatUint(port, 10)
	if strings.TrimSpace(browserArguments) == "" {
		browserArguments = remoteDebugging
	} else {
		browserArguments += " " + remoteDebugging
	}
	if proofUserData := strings.TrimSpace(getenv(UserDataEnvironment)); proofUserData != "" {
		userDataFolder = proofUserData
	}
	return browserArguments, userDataFolder
}
