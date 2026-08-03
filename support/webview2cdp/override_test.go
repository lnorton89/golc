package webview2cdp

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolvePreservesProductionInputsWithoutValidProofPort(t *testing.T) {
	for _, port := range []string{"", "invalid", "1023", "65536"} {
		t.Run(port, func(t *testing.T) {
			values := map[string]string{
				PortEnvironment:     port,
				UserDataEnvironment: `C:\proof-data`,
			}
			arguments, data := Resolve(func(name string) string { return values[name] }, "--disable-gpu", `C:\wails-data`)
			require.Equal(t, "--disable-gpu", arguments)
			require.Equal(t, `C:\wails-data`, data)
		})
	}
}

func TestResolveAddsValidatedProofInputs(t *testing.T) {
	values := map[string]string{
		PortEnvironment:     "19226",
		UserDataEnvironment: `C:\proof-data`,
	}
	getenv := func(name string) string { return values[name] }

	arguments, data := Resolve(getenv, "--disable-features=msSmartScreenProtection", `C:\wails-data`)
	require.Equal(t, "--disable-features=msSmartScreenProtection --remote-debugging-port=19226", arguments)
	require.Equal(t, `C:\proof-data`, data)

	arguments, data = Resolve(getenv, "", `C:\wails-data`)
	require.Equal(t, "--remote-debugging-port=19226", arguments)
	require.Equal(t, `C:\proof-data`, data)
}
