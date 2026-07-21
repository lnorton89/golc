package command

import "testing"

// TestScopeBuildArgs is the exact quick-test marker for scope
// "build-args" (test --quick --scope build-args). It exercises only the
// pure argument-parsing/lookup logic build.go's "--scope" extension adds
// (Plan 01-13) — no archive download, module fetch, tool install, or Node
// toolchain resolution ever happens here, so the registered scope exits 0
// offline.
func TestScopeBuildArgs(t *testing.T) {
	t.Run("no arguments means the bare full build", func(t *testing.T) {
		scope, err := parseBuildArgs(nil)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if scope != "" {
			t.Fatalf("expected an empty scope, got %q", scope)
		}
	})

	t.Run("--scope <name> selects a named scope", func(t *testing.T) {
		scope, err := parseBuildArgs([]string{"--scope", "linear-sdk"})
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if scope != "linear-sdk" {
			t.Fatalf("expected scope %q, got %q", "linear-sdk", scope)
		}
	})

	t.Run("--scope=<name> selects a named scope", func(t *testing.T) {
		scope, err := parseBuildArgs([]string{"--scope=linear-sdk"})
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if scope != "linear-sdk" {
			t.Fatalf("expected scope %q, got %q", "linear-sdk", scope)
		}
	})

	t.Run("--scope without a value is rejected", func(t *testing.T) {
		if _, err := parseBuildArgs([]string{"--scope"}); err == nil {
			t.Fatal("expected an error for a bare --scope")
		}
	})

	t.Run("--scope with an empty value is rejected", func(t *testing.T) {
		if _, err := parseBuildArgs([]string{"--scope", ""}); err == nil {
			t.Fatal("expected an error for --scope with an empty value")
		}
		if _, err := parseBuildArgs([]string{"--scope="}); err == nil {
			t.Fatal("expected an error for --scope= with an empty value")
		}
	})

	t.Run("an unsupported argument is rejected", func(t *testing.T) {
		if _, err := parseBuildArgs([]string{"--bogus"}); err == nil {
			t.Fatal("expected an error for an unsupported argument")
		}
	})

	t.Run("linear-sdk build scope self-registers with the documented directory", func(t *testing.T) {
		registration, found := lookupNodeBuildScope("linear-sdk")
		if !found {
			t.Fatal("expected the linear-sdk build scope to be registered")
		}
		if registration.Dir != "tools/linear-sync" {
			t.Fatalf("expected Dir %q, got %q", "tools/linear-sync", registration.Dir)
		}
	})

	t.Run("an unknown build scope is not registered", func(t *testing.T) {
		if _, found := lookupNodeBuildScope("does-not-exist"); found {
			t.Fatal("expected an unregistered scope name to be absent")
		}
	})

	t.Run("linear-sdk-operations test scope self-registers with a non-empty command", func(t *testing.T) {
		registration, found := lookupNodeScope("linear-sdk-operations")
		if !found {
			t.Fatal("expected the linear-sdk-operations quick-test scope to be registered")
		}
		if registration.Dir != "tools/linear-sync" {
			t.Fatalf("expected Dir %q, got %q", "tools/linear-sync", registration.Dir)
		}
		if len(registration.Command) == 0 {
			t.Fatal("expected a non-empty registered Command")
		}
	})
}
