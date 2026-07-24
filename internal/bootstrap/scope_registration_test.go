package bootstrap_test

import "github.com/lnorton89/golc/internal/command"

// Bootstrap is a delivery dependency, so its internal tests cannot import
// command without closing bootstrap[test] -> command -> delivery ->
// bootstrap. Keep the exact quick-scope declarations external.
var _ = command.MustDeclareScope(command.ScopeRegistration{
	Scope:   "bootstrap-archive",
	Summary: "Official-source policy, archive verification, and atomic promotion tests.",
})

var _ = command.MustDeclareScope(command.ScopeRegistration{
	Scope:   "bootstrap-cache",
	Summary: "Project-local cache layout, offline environment, and directory warming tests.",
})

var _ = command.MustDeclareScope(command.ScopeRegistration{
	Scope:   "bootstrap-engine",
	Summary: "Platform-aware, offline Go bootstrap engine orchestration tests.",
})

var _ = command.MustDeclareScope(command.ScopeRegistration{
	Scope:   "bootstrap-linear-sync",
	Summary: "Pinned Node, exact-lock npm ci, and TypeScript output bootstrap tests.",
})
