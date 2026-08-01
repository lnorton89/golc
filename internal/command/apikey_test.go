// apikey_test.go pins the "api-key create"/"api-key list"/"api-key
// revoke" route contract (07-04-PLAN.md Task 1, RED state) before
// internal/command/apikey.go exists: the raw token is printed exactly
// once at create time and is never present in "api-key list" output or
// in the stored row's hash; revoking a key marks it invalid immediately
// and revoking an unknown/already-revoked id fails cleanly. It follows
// pooldeploy_test.go's exact route-invocation convention.
package command_test

import (
	"encoding/json"
	"testing"

	"github.com/lnorton89/golc/internal/command"
	"github.com/lnorton89/golc/internal/show"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyCreateStoresOnlyHashAndPrefix(t *testing.T) {
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err)
	root := t.TempDir()
	showPath := "show.golc"

	create := registry.Execute(command.Request{Root: root, Args: []string{
		"api-key", "create", "--scope", "playback,admin", "--expires", "720h", "--show", showPath, "--json",
	}})
	require.Equal(t, 0, create.ExitCode, "api-key create failed: exit=%d stderr=%s", create.ExitCode, create.Stderr)

	var created struct {
		ID       string   `json:"id"`
		Prefix   string   `json:"prefix"`
		Scopes   []string `json:"scopes"`
		RawToken string   `json:"raw_token"`
	}
	if err := json.Unmarshal(create.Stdout, &created); err != nil {
		require.NoError(t, err)
	}
	require.NotEqual(t, "", created.RawToken, "expected api-key create to print the raw token exactly once, got empty raw_token")
	require.Len(t, created.Scopes, 2)
	require.Equal(t, "playback", created.Scopes[0])
	require.Equal(t, "admin", created.Scopes[1], "expected scopes [playback admin], got %v", created.Scopes)

	// The stored row never contains the raw token -- only a hash.
	record, found, err := show.LookupAPIKeyByPrefix(root, showPath, created.Prefix)
	require.NoError(t, err)
	require.True(t, found, "expected the created key to be findable by prefix")
	require.NotEqual(t, created.RawToken, record.Hash, "expected the stored hash to differ from the raw token")
	require.True(t, show.CompareAPIKeyHash(created.RawToken, record.Hash), "expected the stored hash to match the raw token via CompareAPIKeyHash")

	// "api-key list" never prints the raw key or hash.
	list := registry.Execute(command.Request{Root: root, Args: []string{"api-key", "list", "--show", showPath, "--json"}})
	require.Equal(t, 0, list.ExitCode, "api-key list failed: exit=%d stderr=%s", list.ExitCode, list.Stderr)
	require.NotContains(t, string(list.Stdout), created.RawToken, "expected api-key list output to never contain the raw token, got: %s", list.Stdout)
	require.NotContains(t, string(list.Stdout), record.Hash, "expected api-key list output to never contain the stored hash, got: %s", list.Stdout)

	var listed struct {
		Keys []struct {
			ID     string   `json:"id"`
			Prefix string   `json:"prefix"`
			Scopes []string `json:"scopes"`
		} `json:"keys"`
	}
	if err := json.Unmarshal(list.Stdout, &listed); err != nil {
		require.NoError(t, err)
	}
	require.Len(t, listed.Keys, 1)
	require.Equal(t, created.ID, listed.Keys[0].ID, "expected exactly one listed key matching %q, got %+v", created.ID, listed.Keys)
}

func TestAPIKeyRevokeMarksRevoked(t *testing.T) {
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err)
	root := t.TempDir()
	showPath := "show.golc"

	create := registry.Execute(command.Request{Root: root, Args: []string{
		"api-key", "create", "--scope", "authoring", "--expires", "1h", "--show", showPath, "--json",
	}})
	require.Equal(t, 0, create.ExitCode, "api-key create failed: exit=%d stderr=%s", create.ExitCode, create.Stderr)
	var created struct {
		ID     string `json:"id"`
		Prefix string `json:"prefix"`
	}
	if err := json.Unmarshal(create.Stdout, &created); err != nil {
		require.NoError(t, err)
	}

	revoke := registry.Execute(command.Request{Root: root, Args: []string{"api-key", "revoke", "--id", created.ID, "--show", showPath}})
	require.Equal(t, 0, revoke.ExitCode, "api-key revoke failed: exit=%d stderr=%s", revoke.ExitCode, revoke.Stderr)

	record, found, err := show.LookupAPIKeyByPrefix(root, showPath, created.Prefix)
	require.NoError(t, err)
	require.True(t, found, "expected the revoked key to still be findable by prefix")
	require.False(t, record.RevokedAt.IsZero(), "expected RevokedAt to be set after api-key revoke")

	// Revoking an already-revoked (or unknown) id fails cleanly, never a
	// silent no-op.
	again := registry.Execute(command.Request{Root: root, Args: []string{"api-key", "revoke", "--id", created.ID, "--show", showPath}})
	require.NotEqual(t, 0, again.ExitCode)
	require.Contains(t, string(again.Stderr), "GOLC_APIKEY_NOT_FOUND", "expected GOLC_APIKEY_NOT_FOUND revoking an already-revoked key, got exit=%d stderr=%s", again.ExitCode, again.Stderr)
}

func TestAPIKeyCreateUsageErrors(t *testing.T) {
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err)
	root := t.TempDir()
	showPath := "show.golc"

	missingScope := registry.Execute(command.Request{Root: root, Args: []string{"api-key", "create", "--expires", "1h", "--show", showPath}})
	require.Equal(t, 2, missingScope.ExitCode, "expected ExitCode 2 for missing --scope, got %d stderr=%s", missingScope.ExitCode, missingScope.Stderr)

	invalidScope := registry.Execute(command.Request{Root: root, Args: []string{"api-key", "create", "--scope", "bogus", "--expires", "1h", "--show", showPath}})
	require.Equal(t, 2, invalidScope.ExitCode)
	require.Contains(t, string(invalidScope.Stderr), "GOLC_APIKEY_SCOPE_INVALID", "expected GOLC_APIKEY_SCOPE_INVALID for an invalid scope, got exit=%d stderr=%s", invalidScope.ExitCode, invalidScope.Stderr)

	missingExpires := registry.Execute(command.Request{Root: root, Args: []string{"api-key", "create", "--scope", "playback", "--show", showPath}})
	require.Equal(t, 2, missingExpires.ExitCode, "expected ExitCode 2 for missing --expires, got %d stderr=%s", missingExpires.ExitCode, missingExpires.Stderr)
}
