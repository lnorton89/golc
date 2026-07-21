// linear_sync.go self-registers the isolated tools/linear-sync Node
// workspace's exact build and quick-test scopes (CONTEXT D-01/D-03; Plan
// 01-13) through the same MustDeclareRoute/MustDeclareScope
// self-registration contract Plan 17 established in
// internal/command/router.go: internal/command/test.go's own doc comment
// names tools/linear-sync as the intended first consumer of
// MustDeclareNodeScope, and build.go's MustDeclareNodeBuildScope (Plan
// 01-13) is the parallel build-side registration. config/commands.toml
// documents these exact two scope names/directory as the single
// authoritative declaration (D-05); this file only wires the
// already-established build/test dispatch contracts to that documented
// pair. It never redeclares the scope name, Node version, or workspace
// directory anywhere else, and it never runs npm/tsc itself -- both
// dispatchers (build.go/test.go) resolve the pinned project-local Node
// toolchain at request time.
package command

import "os"

// linearSyncWorkspaceDir is the single repository-relative directory both
// registrations below point at, matching tools/linear-sync's own
// package.json/tsconfig.json location exactly.
const linearSyncWorkspaceDir = "tools/linear-sync"

// linear-sdk is the exact build scope config/commands.toml documents for
// tools/linear-sync: `build --scope linear-sdk` compiles protocol.ts/
// adapter.ts with the pinned project-local TypeScript compiler.
var _ = MustDeclareNodeBuildScope(NodeBuildScopeRegistration{
	Scope: "linear-sdk",
	Dir:   linearSyncWorkspaceDir,
})

// linear-sdk-operations is the exact quick-test scope config/commands.toml
// documents for tools/linear-sync: `test --quick --scope
// linear-sdk-operations` runs the workspace's Node test suite (Plan
// 01-25's test/operations.test.ts, compiled to dist/test/*.test.js).
// Command resolves the pinned project-local Node executable through
// GOLC_PROJECT_ROOT, the exact environment variable golc.ps1 exports
// before delegating every non-bootstrap route to the compiled CLI
// (cmd/golc-project/main.go); this is the same officially-supported-
// via-the-shim precedent main.go's own root resolution already relies on.
var _ = MustDeclareNodeScope(NodeScopeRegistration{
	Scope:   "linear-sdk-operations",
	Dir:     linearSyncWorkspaceDir,
	Marker:  "TestScopeLinearSdkOperations",
	Command: linearSyncNodeTestCommand(),
})

// linearSyncNodeTestGlob is a glob pattern, not a bare directory: Node's
// own --test path resolution (confirmed empirically against the pinned
// Node 24.18.0 build) fails a bare directory argument with
// MODULE_NOT_FOUND rather than auto-discovering test files inside it, but
// natively resolves a glob pattern itself (no shell expansion required,
// so this is safe to pass through exec.Command's argv unquoted). "**"
// covers any future nested test file without editing this dispatcher
// again.
const linearSyncNodeTestGlob = "dist/test/**/*.test.js"

// linearSyncNodeTestCommand resolves the exact `node --test
// dist/test/**/*.test.js` invocation this scope's registered Command
// runs. MustDeclareNodeScope requires a non-empty Command at package-init
// time (before any Request.Root is known), so this falls back to
// GOLC_PROJECT_ROOT -- the same environment variable golc.ps1 exports
// before delegating to the compiled CLI -- and, failing that, the
// process's own working directory. If the pinned Node toolchain has not
// been provisioned yet (bootstrap --include linear-sync has not run),
// this still returns a non-empty placeholder Command rather than
// panicking: MustDeclareNodeScope would otherwise crash every route in
// the CLI at startup over one unbootstrapped scope. Invoking this exact
// scope before bootstrap then fails closed with an ordinary "executable
// file not found" exec error instead.
func linearSyncNodeTestCommand() []string {
	root := os.Getenv("GOLC_PROJECT_ROOT")
	if root == "" {
		if cwd, err := os.Getwd(); err == nil {
			root = cwd
		}
	}
	if nodeExecutable, err := resolvePinnedNodeExecutable(root); err == nil {
		return []string{nodeExecutable, "--test", linearSyncNodeTestGlob}
	}
	return []string{"golc-linear-sync-node-not-bootstrapped", "--test", linearSyncNodeTestGlob}
}
