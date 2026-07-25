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
	"strings"
	"testing"

	"github.com/lnorton89/golc/internal/command"
	"github.com/lnorton89/golc/internal/show"
)

func TestAPIKeyCreateStoresOnlyHashAndPrefix(t *testing.T) {
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}
	root := t.TempDir()
	showPath := "show.golc"

	create := registry.Execute(command.Request{Root: root, Args: []string{
		"api-key", "create", "--scope", "playback,admin", "--expires", "720h", "--show", showPath, "--json",
	}})
	if create.ExitCode != 0 {
		t.Fatalf("api-key create failed: exit=%d stderr=%s", create.ExitCode, create.Stderr)
	}

	var created struct {
		ID       string   `json:"id"`
		Prefix   string   `json:"prefix"`
		Scopes   []string `json:"scopes"`
		RawToken string   `json:"raw_token"`
	}
	if err := json.Unmarshal(create.Stdout, &created); err != nil {
		t.Fatalf("unmarshal api-key create output: %v", err)
	}
	if created.RawToken == "" {
		t.Fatalf("expected api-key create to print the raw token exactly once, got empty raw_token")
	}
	if len(created.Scopes) != 2 || created.Scopes[0] != "playback" || created.Scopes[1] != "admin" {
		t.Fatalf("expected scopes [playback admin], got %v", created.Scopes)
	}

	// The stored row never contains the raw token -- only a hash.
	record, found, err := show.LookupAPIKeyByPrefix(root, showPath, created.Prefix)
	if err != nil {
		t.Fatalf("LookupAPIKeyByPrefix: %v", err)
	}
	if !found {
		t.Fatalf("expected the created key to be findable by prefix")
	}
	if record.Hash == created.RawToken {
		t.Fatalf("expected the stored hash to differ from the raw token")
	}
	if !show.CompareAPIKeyHash(created.RawToken, record.Hash) {
		t.Fatalf("expected the stored hash to match the raw token via CompareAPIKeyHash")
	}

	// "api-key list" never prints the raw key or hash.
	list := registry.Execute(command.Request{Root: root, Args: []string{"api-key", "list", "--show", showPath, "--json"}})
	if list.ExitCode != 0 {
		t.Fatalf("api-key list failed: exit=%d stderr=%s", list.ExitCode, list.Stderr)
	}
	if strings.Contains(string(list.Stdout), created.RawToken) {
		t.Fatalf("expected api-key list output to never contain the raw token, got: %s", list.Stdout)
	}
	if strings.Contains(string(list.Stdout), record.Hash) {
		t.Fatalf("expected api-key list output to never contain the stored hash, got: %s", list.Stdout)
	}

	var listed struct {
		Keys []struct {
			ID     string   `json:"id"`
			Prefix string   `json:"prefix"`
			Scopes []string `json:"scopes"`
		} `json:"keys"`
	}
	if err := json.Unmarshal(list.Stdout, &listed); err != nil {
		t.Fatalf("unmarshal api-key list output: %v", err)
	}
	if len(listed.Keys) != 1 || listed.Keys[0].ID != created.ID {
		t.Fatalf("expected exactly one listed key matching %q, got %+v", created.ID, listed.Keys)
	}
}

func TestAPIKeyRevokeMarksRevoked(t *testing.T) {
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}
	root := t.TempDir()
	showPath := "show.golc"

	create := registry.Execute(command.Request{Root: root, Args: []string{
		"api-key", "create", "--scope", "authoring", "--expires", "1h", "--show", showPath, "--json",
	}})
	if create.ExitCode != 0 {
		t.Fatalf("api-key create failed: exit=%d stderr=%s", create.ExitCode, create.Stderr)
	}
	var created struct {
		ID     string `json:"id"`
		Prefix string `json:"prefix"`
	}
	if err := json.Unmarshal(create.Stdout, &created); err != nil {
		t.Fatalf("unmarshal api-key create output: %v", err)
	}

	revoke := registry.Execute(command.Request{Root: root, Args: []string{"api-key", "revoke", "--id", created.ID, "--show", showPath}})
	if revoke.ExitCode != 0 {
		t.Fatalf("api-key revoke failed: exit=%d stderr=%s", revoke.ExitCode, revoke.Stderr)
	}

	record, found, err := show.LookupAPIKeyByPrefix(root, showPath, created.Prefix)
	if err != nil {
		t.Fatalf("LookupAPIKeyByPrefix: %v", err)
	}
	if !found {
		t.Fatalf("expected the revoked key to still be findable by prefix")
	}
	if record.RevokedAt.IsZero() {
		t.Fatalf("expected RevokedAt to be set after api-key revoke")
	}

	// Revoking an already-revoked (or unknown) id fails cleanly, never a
	// silent no-op.
	again := registry.Execute(command.Request{Root: root, Args: []string{"api-key", "revoke", "--id", created.ID, "--show", showPath}})
	if again.ExitCode == 0 || !strings.Contains(string(again.Stderr), "GOLC_APIKEY_NOT_FOUND") {
		t.Fatalf("expected GOLC_APIKEY_NOT_FOUND revoking an already-revoked key, got exit=%d stderr=%s", again.ExitCode, again.Stderr)
	}
}

func TestAPIKeyCreateUsageErrors(t *testing.T) {
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}
	root := t.TempDir()
	showPath := "show.golc"

	missingScope := registry.Execute(command.Request{Root: root, Args: []string{"api-key", "create", "--expires", "1h", "--show", showPath}})
	if missingScope.ExitCode != 2 {
		t.Fatalf("expected ExitCode 2 for missing --scope, got %d stderr=%s", missingScope.ExitCode, missingScope.Stderr)
	}

	invalidScope := registry.Execute(command.Request{Root: root, Args: []string{"api-key", "create", "--scope", "bogus", "--expires", "1h", "--show", showPath}})
	if invalidScope.ExitCode != 2 || !strings.Contains(string(invalidScope.Stderr), "GOLC_APIKEY_SCOPE_INVALID") {
		t.Fatalf("expected GOLC_APIKEY_SCOPE_INVALID for an invalid scope, got exit=%d stderr=%s", invalidScope.ExitCode, invalidScope.Stderr)
	}

	missingExpires := registry.Execute(command.Request{Root: root, Args: []string{"api-key", "create", "--scope", "playback", "--show", showPath}})
	if missingExpires.ExitCode != 2 {
		t.Fatalf("expected ExitCode 2 for missing --expires, got %d stderr=%s", missingExpires.ExitCode, missingExpires.Stderr)
	}
}
